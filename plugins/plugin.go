package plugins

import (
	"context"
	"errors"
	"fmt"

	"github.com/jumppad-labs/xcl/entity"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/types"
)

/*
// last element in labels array is the name

lab {}
lab "0" {}
resource lab 0 {}

lab.0.meta.id
lab.meta.id
resource.lab.meta.id

container "blah" {
	plugin = var.cloud
}

resource "container" "blah" {
}

container "gcp" "blah" {
}
*/

// RegisteredType represents a registered resource type with its metadata
type RegisteredType struct {
	// The top level type name, i.e. resource
	Type string
	// the optional sub type, i.e. k8s_config
	SubType string
	// The json schema for the type
	Schema []byte
	// The provider adapter that handles this type
	Adapter ProviderAdapter
}

type PluginEntityProvider interface {
	// GetTypes returns the types handled by the plugin.
	GetTypes() []RegisteredType

	// GetFunctions returns the functions the plugin's providers offer to
	// configuration, sorted by name.
	GetFunctions() []RegisteredFunction

	// CallFunction calls the function called name with args, a JSON array
	// holding one value per parameter, and returns its result as JSON. The
	// function's own error is returned with its message.
	CallFunction(ctx context.Context, name string, args []byte) ([]byte, error)

	// every lifecycle method takes the provider call's context, which
	// carries the call's logger, see logger.Logger
	Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error
	Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)
	Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error
	Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error)
	Update(ctx context.Context, entityType, entitySubType string, entityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) ([]byte, error)
	Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)
}

// RegisterResourceProvider registers a typed resource provider with the plugin.
// This creates a typed adapter and registers it with the plugin.
//
// subTypeName may be empty, in which case the block type has no subtype and
// is written <type> "<name>" {} and addressed <type>.<name>. A keyword cannot
// be registered both with and without a subtype.
//
// logger is plugin scoped, it is for messages the provider writes outside a
// provider call, such as in Init. During a call use Logger(ctx), which is
// bound to the resource and step being worked on.
func RegisterResourceProvider[T any](p *PluginBase, logger logger.Logger, state State, typeName, subTypeName string, resourceInstance T, provider ResourceProvider[T]) error {
	// Create a typed adapter for the provider, named after the block type so
	// the provider's logs are tagged with it. A type registered without a
	// subtype, written <type> "<name>" {}, is named after its type.
	adapter := NewTypedProviderAdapter(provider, resourceInstance)
	adapter.name = subTypeName
	if subTypeName == "" {
		adapter.name = typeName
	}

	// Initialize the adapter with state, functions (can be nil), and logger
	err := adapter.Init(state, nil, logger)
	if err != nil {
		return err
	}

	if err := p.RegisterType(typeName, subTypeName, resourceInstance, adapter); err != nil {
		return err
	}

	// the provider's functions are offered to configuration under their
	// bare names; two providers of one plugin cannot offer the same name
	return p.registerProviderFunctions(adapter.name, provider.Functions())
}

// Plugin is a private interface that defines the contract between XCL
// and the providers. Init is called once, when the plugin loads. The logger it
// is given is plugin scoped, for messages written outside a provider call;
// during a call use Logger(ctx).
type Plugin interface {
	Init(logger.Logger, State) error
	SetState(state State)
	PluginEntityProvider
}

type PluginBase struct {
	state           State // state functions passed to the plugin via Init
	registeredTypes []RegisteredType

	// functions are the functions the plugin's providers offer, keyed by
	// name, with the provider each came from
	functions map[string]pluginFunction
}

// pluginFunction is a function a provider of the plugin offers, with the
// name of the provider, for reporting a name two providers both offer
type pluginFunction struct {
	fn       ProviderFunction
	provider string
}

// SetState sets the state for the plugin base
func (p *PluginBase) SetState(state State) {
	p.state = state
}

// RegisterType registers a type with the plugin using type-safe parameters.
// This method works with any type that has embedded ResourceBase.
func (p *PluginBase) RegisterType(typeName, subTypeName string, t any, prov ProviderAdapter) error {
	entitySchema, err := schema.GenerateSchemaFromInstance(t, 10)
	if err != nil {
		return err
	}

	if p.registeredTypes == nil {
		p.registeredTypes = []RegisteredType{}
	}

	p.registeredTypes = append(p.registeredTypes, RegisteredType{
		Type:    typeName,
		SubType: subTypeName,
		Schema:  entitySchema,
		Adapter: prov,
	})

	return nil
}

func (p *PluginBase) getRegisteredType(entityType, entitySubType string) *RegisteredType {
	for i := range p.registeredTypes {
		if p.registeredTypes[i].Type == entityType && p.registeredTypes[i].SubType == entitySubType {
			return &p.registeredTypes[i]
		}
	}
	return nil
}

// GetTypes returns all registered types
func (p *PluginBase) GetTypes() []RegisteredType {
	return p.registeredTypes
}

// registerProviderFunctions records the functions a provider offers, named
// provider, so the plugin can list and call them. A nil fns offers none. A
// name another provider of the plugin already offers is returned as a
// *FunctionNameClashError naming both providers.
func (p *PluginBase) registerProviderFunctions(provider string, fns ProviderFunctions) error {
	if fns == nil {
		return nil
	}

	offered, err := fns.List()
	if err != nil {
		return fmt.Errorf("provider %s: listing its functions: %w", provider, err)
	}

	if p.functions == nil {
		p.functions = map[string]pluginFunction{}
	}

	for _, name := range sortedFunctionNames(offered) {
		if existing, taken := p.functions[name]; taken {
			return &xclerrors.FunctionNameClashError{
				Name:     name,
				Provider: "provider " + provider,
				Existing: "provider " + existing.provider,
			}
		}

		p.functions[name] = pluginFunction{fn: offered[name], provider: provider}
	}

	return nil
}

// GetFunctions returns the functions the plugin's providers offer, sorted by
// name
func (p *PluginBase) GetFunctions() []RegisteredFunction {
	registered := make([]RegisteredFunction, 0, len(p.functions))
	for _, name := range sortedFunctionNames(p.functions) {
		registered = append(registered, RegisteredFunction{
			Name:      name,
			Signature: p.functions[name].fn.Signature(),
		})
	}

	return registered
}

// CallFunction calls the function called name with args, a JSON array
// holding one value per parameter, and returns its result as JSON
func (p *PluginBase) CallFunction(ctx context.Context, name string, args []byte) ([]byte, error) {
	fn, ok := p.functions[name]
	if !ok {
		return nil, fmt.Errorf("function %q is not offered by this plugin", name)
	}

	return fn.fn.Call(ctx, args)
}

// lifecycle methods

// Validate validates the given entity data.
func (p *PluginBase) Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	// the subtype is optional, an entity type declared without one has none
	if entityType == "" {
		return errors.New("entityType cannot be empty")
	}

	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + types.TypeKey(entityType, entitySubType))
	}

	return rt.Adapter.Validate(ctx, entityData)
}

// Create creates a new entity.
func (p *PluginBase) Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return nil, errors.New("no registered type found for " + types.TypeKey(entityType, entitySubType))
	}

	return rt.Adapter.Create(ctx, entityData)
}

// Destroy deletes an existing entity.
func (p *PluginBase) Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + types.TypeKey(entityType, entitySubType))
	}

	return rt.Adapter.Destroy(ctx, entityData, false)
}

// Read reports the real entity, given its saved and configured copies.
func (p *PluginBase) Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return nil, errors.New("no registered type found for " + types.TypeKey(entityType, entitySubType))
	}

	return rt.Adapter.Read(ctx, oldEntityData, newEntityData)
}

// Update updates an existing entity.
func (p *PluginBase) Update(ctx context.Context, entityType, entitySubType string, entityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) ([]byte, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return nil, errors.New("no registered type found for " + types.TypeKey(entityType, entitySubType))
	}

	return rt.Adapter.Update(ctx, entityData, changes, dependencies)
}

// Changed decides what applying new needs for the entity saved as old, given
// the dependencies the same apply will update or replace.
func (p *PluginBase) Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return entity.NoChange, errors.New("no registered type found for " + types.TypeKey(entityType, entitySubType))
	}

	return rt.Adapter.Changed(ctx, oldEntityData, newEntityData, changes, dependencies)
}
