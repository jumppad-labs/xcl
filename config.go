package xcl

import (
	"errors"
	"fmt"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/logger"
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
)

// Config defines the stack config
// It orchestrates high-level operations (Apply, Validate, Destroy)
// and manages the current state
type Config struct {
	entities       []any                    // what the configuration declares (private)
	pluginRegistry *registry.PluginRegistry // Config owns plugins
	stateStore     state.StateStore         // Persistence for state
	variables      map[string]any           // Variables for HCL parsing
	eventHandler   EventHandler             // Called for every lifecycle event during Apply and Destroy

	addresses *resources.AddressParser // resolves addresses against the known types
}

// addressParser returns a parser that resolves addresses against the types the
// registry knows. A module relative address cannot be split without them: in
// module.a.b.c, "b" is a module name unless something is registered under it.
func (c *Config) addressParser() *resources.AddressParser {
	if c.addresses == nil {
		var known []types.TypeInfo
		if c.pluginRegistry != nil {
			known = c.pluginRegistry.Types()
		}

		c.addresses = resources.NewAddressParser(known)
	}

	return c.addresses
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
		c.pluginRegistry = registry.NewPluginRegistry(logger.NewStdOutLogger())
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
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	// Create parser with StateStore
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		Variables:      convertVariablesToStringMap(c.variables),
		OnParserEvent:  parserEventHandler(c.eventHandler),
	})

	// Validate without resolving: no decode, no DAG walk, no plugins
	if err := p.Validate(paths...); err != nil {
		return err
	}

	return nil
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
	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	// Create parser with StateStore
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		Variables:      convertVariablesToStringMap(c.variables),
		OnParserEvent:  parserEventHandler(c.eventHandler),
	})

	// Parser manages State independently (loads from store, parses, returns new state)
	// A failed apply returns the progress it made along with the error
	newState, err := p.Apply(paths...)
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

	// Create parser with StateStore, destroy saves through it after every resource
	p := parser.NewParser(&parser.ParserOptions{
		StateStore:     c.stateStore,
		PluginRegistry: c.pluginRegistry,
		OnParserEvent:  parserEventHandler(c.eventHandler),
	})

	remaining, err := p.Destroy(saved)
	if remaining != nil {
		c.entities = remaining.GetResources()
	}

	return err
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
