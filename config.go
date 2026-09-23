package xcl

import (
	"context"
	"errors"
	"fmt"
	"time"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/eventstream"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// ErrEmptyConfiguration is returned by Apply when the configuration declares no
// blocks. Nothing is destroyed, created, changed or saved: use Destroy to
// remove everything. Check for it with errors.Is.
var ErrEmptyConfiguration = parser.ErrEmptyConfiguration

// The errors a lookup against a configuration can return. Each is matched by
// identity with errors.Is, and each is wrapped by the detail type below it,
// recovered with errors.As.
//
// They are re-exported from the project's errors package so that matching one
// needs no import named errors beside the standard library's.
//
// ErrNotFound and ErrNotUnique are ordinary outcomes to handle. The other five
// mean the question itself had no answer.
var (
	// ErrNotFound means no entity is declared at the address given. It is
	// distinct from plugins.ErrNotFound, which means the real infrastructure
	// behind a declared entity has gone missing.
	ErrNotFound = xclerrors.ErrNotFound

	// ErrUnknownType means the leading segment of a query is not a known kind.
	ErrUnknownType = xclerrors.ErrUnknownType

	// ErrNotTypeable means the query matches entities of more than one Go
	// type, so the result cannot be typed.
	ErrNotTypeable = xclerrors.ErrNotTypeable

	// ErrNotRegistered means a Go type has no registered name to derive an
	// address from, which is always so for a plugin provided type.
	ErrNotRegistered = xclerrors.ErrNotRegistered

	// ErrTypeMismatch means an entity is not of the Go type asked for.
	ErrTypeMismatch = xclerrors.ErrTypeMismatch

	// ErrNotAnEntity means a Go type is a block nested inside another
	// declaration and so has no address of its own.
	ErrNotAnEntity = xclerrors.ErrNotAnEntity

	// ErrNotUnique means a query expecting exactly one entity matched more
	// than one. The detail reports how many.
	ErrNotUnique = xclerrors.ErrNotUnique
)

// ErrPluginLoad is returned by the first Validate, Apply or Destroy when a
// registered plugin cannot be loaded, plugins load when first needed rather
// than when registered. Check for it with errors.Is, the PluginLoadError
// detail names the plugin.
var ErrPluginLoad = xclerrors.ErrPluginLoad

// The errors below name every way turning an entity, or an entity's saved
// data, into configuration text can fail. Check for them with errors.Is, and
// recover the detail carried alongside each with errors.As.
var (
	// ErrUnregisteredType means saved data names a type the registry cannot
	// create. The UnregisteredTypeError detail names the type. A plugin's
	// types resolve only once the registry has loaded.
	ErrUnregisteredType = xclerrors.ErrUnregisteredType

	// ErrInvalidSavedData means the data given is not one saved entity
	// record. The InvalidSavedDataError detail names the record where it was
	// readable enough to name itself.
	ErrInvalidSavedData = xclerrors.ErrInvalidSavedData

	// ErrNotEncodable means a value cannot be written as configuration,
	// because it is not an entity, it is a builtin such as a variable, output
	// or module, or it holds a value that has no configuration form.
	ErrNotEncodable = xclerrors.ErrNotEncodable
)

// The detail carried by each of the errors above, recovered with errors.As.
// These are aliases, so a caller names them here without importing a second
// package called errors.
type (
	NotFoundError      = xclerrors.NotFoundError
	UnknownTypeError   = xclerrors.UnknownTypeError
	NotTypeableError   = xclerrors.NotTypeableError
	NotRegisteredError = xclerrors.NotRegisteredError
	TypeMismatchError  = xclerrors.TypeMismatchError
	NotAnEntityError   = xclerrors.NotAnEntityError
	NotUniqueError     = xclerrors.NotUniqueError
	PluginLoadError    = xclerrors.PluginLoadError

	UnregisteredTypeError = xclerrors.UnregisteredTypeError
	InvalidSavedDataError = xclerrors.InvalidSavedDataError
	NotEncodableError     = xclerrors.NotEncodableError
)

// Config defines the stack config
// It orchestrates high-level operations (Apply, Validate, Destroy)
// and manages the current state
type Config struct {
	entities        []any                    // what the configuration declares (private)
	pluginRegistry  *registry.PluginRegistry // Config owns plugins
	stateStore      state.StateStore         // Persistence for state
	variables       map[string]any           // Variables for HCL parsing
	eventHandler    EventHandler             // Receives every event of Validate, Apply and Destroy
	eventBufferSize int                      // Undelivered events held before emitting waits, 0 is the default
	eventData       events.DataLevel         // What resource data events carry, none by default

	addresses *resources.AddressParser // resolves addresses against the known types
}

// addressParser returns a parser that resolves addresses against the types the
// registry knows. A module relative address cannot be split without them: in
// module.a.b.c, "b" is a module name unless something is registered under it.
//
// The parser is not kept until the registry's plugins have loaded, the types
// they provide are not known before then.
func (c *Config) addressParser() *resources.AddressParser {
	if c.addresses != nil {
		return c.addresses
	}

	var known []types.TypeInfo
	if c.pluginRegistry != nil {
		known = c.pluginRegistry.Types()
	}

	addresses := resources.NewAddressParser(known)
	if c.pluginRegistry == nil || c.pluginRegistry.Loaded() {
		c.addresses = addresses
	}

	return addresses
}

// NewConfig creates a new Config with functional options
// If no options are provided, creates a minimal config with only the builtin
// resource types and no state
func NewConfig(opts ...ConfigOption) *Config {
	c := &Config{
		entities:  []any{},
		variables: map[string]any{},
	}

	// Apply all options
	for _, opt := range opts {
		opt(c)
	}

	if c.pluginRegistry == nil {
		c.pluginRegistry = registry.NewPluginRegistry()
	}

	return c
}

// GetResources returns all resources in current state
func (c *Config) Entities() []any {
	return c.entities
}

// FindResource finds a resource in current state by FQRN path
func (c *Config) FindResource(path string) (any, error) {
	fqrn, err := c.addressParser().Parse(path)
	if err != nil {
		return nil, err
	}

	entity, found := resources.Match(c.Entities(), fqrn)
	if !found {
		return nil, &xclerrors.NotFoundError{Address: path}
	}

	return entity, nil
}

// ResourceCount returns number of resources in current state
func (c *Config) EntityCount() int {
	return len(c.entities)
}

// GetResources returns every entity the configuration declares.
//
// Deprecated: use Entities. The configuration declares more than resources —
// variables, published values and modules are entities too — and the name said
// otherwise. This returns exactly what Entities returns.
func (c *Config) GetResources() []any {
	return c.Entities()
}

// ResourceCount returns the number of entities the configuration declares.
//
// Deprecated: use EntityCount. It counts every kind of declaration, not only
// resources. This returns exactly what EntityCount returns.
func (c *Config) ResourceCount() int {
	return c.EntityCount()
}

// Outputs returns every value the configuration publishes, keyed by address.
//
// There is one entry per published declaration, and each value is the resolved
// value rather than the declaration that produced it, exactly as Find returns
// it for the same address. It is the call FindByType names when asked for
// published values, which it cannot type because they span whatever the
// configuration publishes.
func (c *Config) Outputs() map[string]any {
	published := map[string]any{}

	for _, e := range c.Entities() {
		output, ok := e.(*resources.Output)
		if !ok {
			continue
		}

		meta, err := types.GetMeta(output)
		if err != nil {
			continue
		}

		published[meta.ID] = output.Value
	}

	return published
}

// Validate parses config from the given paths and reports whether it is valid,
// without executing plugins and without creating, changing or removing anything.
// A nil error means the configuration is valid; otherwise the returned error
// collects every problem found.
// Validates:
//   - HCL syntax and schema
//   - DAG has no cycles
//   - Resource dependencies are valid
func (c *Config) Validate(paths ...string) error {
	return c.run(events.OperationValidate, func(ctx context.Context, emit events.Emit) error {
		if len(paths) == 0 {
			return fmt.Errorf("at least one path is required")
		}

		// Create parser with StateStore
		p := parser.NewParser(&parser.ParserOptions{
			EventData:      c.eventData,
			StateStore:     c.stateStore,
			PluginRegistry: c.pluginRegistry,
			Variables:      convertVariablesToStringMap(c.variables),
			Emit:           emit,
		})

		// Validate without resolving: no decode, no DAG walk, no plugins
		return p.Validate(ctx, paths...)
	})
}

// Apply parses config from paths, loads existing state, and applies changes.
// Resources that were applied before but are no longer in the configuration
// are destroyed first, children before parents, before anything is created or
// changed. Plugins are then executed for each resource in dependency order and
// the resulting state is saved. When a provider call fails, the progress made
// so far is saved before the error is returned, so the next apply resumes from
// it. A removed resource whose destroy fails stops the apply before anything
// is created or changed, and is kept as destroy_failed for the next apply to
// retry first.
// Nothing is saved when the configuration does not parse or validate, or when
// it declares no blocks, which returns ErrEmptyConfiguration: use Destroy to
// remove everything.
func (c *Config) Apply(paths ...string) error {
	return c.run(events.OperationApply, func(ctx context.Context, emit events.Emit) error {
		if len(paths) == 0 {
			return fmt.Errorf("at least one path is required")
		}

		// Create parser with StateStore
		p := parser.NewParser(&parser.ParserOptions{
			EventData:      c.eventData,
			StateStore:     c.stateStore,
			PluginRegistry: c.pluginRegistry,
			Variables:      convertVariablesToStringMap(c.variables),
			Emit:           emit,
		})

		// Parser manages State independently (loads from store, parses,
		// returns new state). A failed or cancelled apply returns the
		// progress it made along with the error, it is saved here so a
		// cancelled apply still saves before the runner lets a receiver
		// panic continue
		newState, err := p.Apply(ctx, paths...)
		if newState == nil {
			return err
		}

		// Adopt what the parse produced
		c.entities = newState.GetResources()

		// Save to store
		if c.stateStore != nil {
			if saveErr := c.stateStore.Save(c.entities); saveErr != nil {
				return errors.Join(err, fmt.Errorf("failed to save state: %w", saveErr))
			}
		}

		return err
	})
}

// Destroy removes every resource in the saved state, or in the in-memory state
// when no state store is configured. It needs no configuration: resources are
// destroyed children first, in the reverse of the order they were created in,
// using the parents each resource recorded when it was applied. Variables,
// outputs, modules, disabled blocks and registered types never reach a
// provider.
//
// The state is saved after each resource, so an interrupted destroy resumes
// from where it stopped. A resource that fails to be destroyed stays in the
// state as destroy_failed, together with everything it depends on, and is
// named in the returned error; calling Destroy again retries it. When nothing
// has been saved Destroy succeeds and writes nothing.
func (c *Config) Destroy() error {
	return c.run(events.OperationDestroy, func(ctx context.Context, emit events.Emit) error {
		saved := c.entities

		if c.stateStore != nil {
			if !c.stateStore.Exists() {
				return nil
			}

			loaded, err := c.stateStore.Load()
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			saved = loaded
		}

		if len(saved) == 0 {
			return nil
		}

		// Create parser with StateStore, destroy saves through it after
		// every resource
		p := parser.NewParser(&parser.ParserOptions{
			EventData:      c.eventData,
			StateStore:     c.stateStore,
			PluginRegistry: c.pluginRegistry,
			Emit:           emit,
		})

		remaining, err := p.Destroy(ctx, saved)
		if remaining != nil {
			c.entities = remaining.GetResources()
		}

		return err
	})
}

// convertVariablesToStringMap converts map[string]any to map[string]string
// This is needed for parser compatibility
func convertVariablesToStringMap(vars map[string]any) map[string]string {
	result := make(map[string]string)
	for k, v := range vars {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// run runs the work of one Validate, Apply or Destroy and delivers its events.
//
// Without an event handler the work runs directly with a nil emit, so xcl is
// silent and nothing extra is started. With one, the work runs on a worker
// goroutine and emits into a bounded stream, while the calling goroutine
// drains the stream into the handler, one event at a time. run returns once
// the work has finished and every event it emitted has been delivered. The
// operation's start event comes first and its success or error event, which
// carries the error the work returned, comes last.
//
// A panic in the handler happens on the calling goroutine. The deferred
// shutdown deliberately does not call recover: it cancels the operation
// context so no new provider call starts, discards the stream so blocked
// emitters are released, and waits for the worker, which lets calls in
// progress finish and saves state. The panic then continues unchanged, with
// the handler's own value and stack, from the calling method. Recovering and
// panicking again would replace the handler's stack with this one.
//
// A panic raised by a provider on a worker goroutine is not handled here, it
// behaves as it does without a handler.
func (c *Config) run(operation string, work func(ctx context.Context, emit events.Emit) error) error {
	work = c.withPlugins(work)

	if c.eventHandler == nil {
		return work(context.Background(), nil)
	}

	size := c.eventBufferSize
	if size == 0 {
		size = DefaultEventBufferSize
	}

	stream := eventstream.New(size)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := time.Now()
	stream.Emit(Event{Operation: operation, Phase: events.PhaseStart})

	var err error
	done := make(chan struct{})

	go func() {
		defer close(done)

		err = work(ctx, stream.Emit)

		finished := Event{Operation: operation, Phase: events.PhaseSuccess, Duration: time.Since(started)}
		if err != nil {
			finished.Phase = events.PhaseError
			finished.Error = err
		}

		stream.Emit(finished)
	}()

	drained := false
	defer func() {
		if drained {
			return
		}

		// the handler panicked, stop starting provider calls and wait for
		// the work to finish and save state, the panic continues afterwards
		cancel()
		stream.Discard()
		<-done
	}()

	stream.Drain(c.eventHandler, done)
	drained = true

	return err
}

// withPlugins returns work preceded by loading the registry's plugins, so a
// plugin is started on the first operation that needs it and a plugin that
// fails to load fails that operation. While work runs, the messages plugins
// write outside a provider call go to emit.
func (c *Config) withPlugins(work func(ctx context.Context, emit events.Emit) error) func(ctx context.Context, emit events.Emit) error {
	return func(ctx context.Context, emit events.Emit) error {
		deactivate := c.pluginRegistry.Activate(emit)
		defer deactivate()

		if err := c.pluginRegistry.Load(emit); err != nil {
			return err
		}

		return work(ctx, emit)
	}
}
