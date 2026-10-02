package plugins

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
)

// DirectPluginHost provides direct method calls to in-process plugins without gRPC overhead.
// This is used for testing and embedded plugins where the plugin runs in the same process.
type DirectPluginHost struct {
	plugin Plugin
	name   string
	state  State

	// types are the plugin's types with adapters that name the plugin as
	// the source of every log event its providers write during a call
	types []RegisteredType
}

// NewDirectPluginHost creates a new direct plugin host for in-process plugins.
// The plugin is initialised with a plugin scoped logger that emits to emit,
// naming the plugin's Go type as the Source, i.e. ExamplePlugin, see
// PluginName. A nil emit is silent.
func NewDirectPluginHost(emit events.Emit, state State, plugin Plugin) (*DirectPluginHost, error) {
	name := PluginName(plugin)
	log := logger.New(emit, events.Event{Source: name, Operation: events.OperationLoad})

	// Initialize the plugin with logger and state
	if err := plugin.Init(log, state); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}

	host := &DirectPluginHost{
		plugin: plugin,
		name:   name,
		state:  state,
		types:  sourcedTypes(plugin.GetTypes(), name),
	}

	return host, nil
}

// ResourceTypeNames returns the comma separated block types of the resource
// types in registered, i.e. "postgres, app"
func ResourceTypeNames(registered []RegisteredType) string {
	names := []string{}
	for _, t := range registered {
		if t.Type == "resource" {
			names = append(names, t.SubType)
		}
	}

	return strings.Join(names, ", ")
}

// PluginName returns the name an in-process plugin is known by, the name of
// its Go type, i.e. ExamplePlugin for an *ExamplePlugin. It is the Source of
// the plugin's log events.
func PluginName(plugin Plugin) string {
	t := reflect.TypeOf(plugin)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t.Name()
}

// sourcedTypes returns a copy of registered whose adapters name source as the
// Source of the logger each provider call receives
func sourcedTypes(registered []RegisteredType, source string) []RegisteredType {
	sourced := make([]RegisteredType, len(registered))
	for i, t := range registered {
		sourced[i] = t
		if t.Adapter != nil {
			sourced[i].Adapter = &sourcedAdapter{next: t.Adapter, source: source}
		}
	}

	return sourced
}

// sourcedAdapter names the plugin as the source of the logger in the context
// of every provider call, before passing the call on
type sourcedAdapter struct {
	next   ProviderAdapter
	source string
}

// Ensure sourcedAdapter implements ProviderAdapter
var _ ProviderAdapter = (*sourcedAdapter)(nil)

// withSource returns ctx with its logger naming the plugin as the source
func (a *sourcedAdapter) withSource(ctx context.Context) context.Context {
	return WithLogger(ctx, logger.WithSource(Logger(ctx), a.source))
}

func (a *sourcedAdapter) Init(state State, functions ProviderFunctions, log logger.Logger) error {
	return a.next.Init(state, functions, log)
}

func (a *sourcedAdapter) Validate(ctx context.Context, entityData []byte) error {
	return a.next.Validate(a.withSource(ctx), entityData)
}

func (a *sourcedAdapter) Create(ctx context.Context, entityData []byte) ([]byte, error) {
	return a.next.Create(a.withSource(ctx), entityData)
}

func (a *sourcedAdapter) Destroy(ctx context.Context, entityData []byte, force bool) error {
	return a.next.Destroy(a.withSource(ctx), entityData, force)
}

func (a *sourcedAdapter) Read(ctx context.Context, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	return a.next.Read(a.withSource(ctx), oldEntityData, newEntityData)
}

func (a *sourcedAdapter) Update(ctx context.Context, entityData []byte) ([]byte, error) {
	return a.next.Update(a.withSource(ctx), entityData)
}

func (a *sourcedAdapter) Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error) {
	return a.next.Changed(a.withSource(ctx), oldEntityData, newEntityData)
}

// GetTypes returns the types handled by the plugin
func (h *DirectPluginHost) GetTypes() []RegisteredType {
	return h.types
}

// Validate validates the given entity data
func (h *DirectPluginHost) Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	return h.plugin.Validate(h.withSource(ctx), entityType, entitySubType, entityData)
}

// Create creates a new entity
func (h *DirectPluginHost) Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	return h.plugin.Create(h.withSource(ctx), entityType, entitySubType, entityData)
}

// Destroy deletes an existing entity
func (h *DirectPluginHost) Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	return h.plugin.Destroy(h.withSource(ctx), entityType, entitySubType, entityData)
}

// Read reports the real entity, given its saved and configured copies
func (h *DirectPluginHost) Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	return h.plugin.Read(h.withSource(ctx), entityType, entitySubType, oldEntityData, newEntityData)
}

// Update updates an existing entity
func (h *DirectPluginHost) Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	return h.plugin.Update(h.withSource(ctx), entityType, entitySubType, entityData)
}

// Changed checks if the entity has changed by comparing old and new
func (h *DirectPluginHost) Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error) {
	return h.plugin.Changed(h.withSource(ctx), entityType, entitySubType, oldEntityData, newEntityData)
}

// withSource returns ctx with its logger naming the plugin as the source
func (h *DirectPluginHost) withSource(ctx context.Context) context.Context {
	return WithLogger(ctx, logger.WithSource(Logger(ctx), h.name))
}

// Stop is a no-op for direct plugins as there's nothing to clean up
func (h *DirectPluginHost) Stop() {
	// No cleanup needed for in-process plugins
}

// Ensure DirectPluginHost implements PluginHost interface
var _ PluginHost = (*DirectPluginHost)(nil)
