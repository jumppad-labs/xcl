package plugins

import (
	"context"
	"errors"
	"github.com/jumppad-labs/xcl/logger"

	"github.com/jumppad-labs/xcl/internal/schema"
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

	// every lifecycle method takes the provider call's context, which
	// carries the call's logger, see logger.Logger
	Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error
	Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)
	Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error
	Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error)
	Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)
	Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error)
}

// RegisterResourceProvider registers a typed resource provider with the plugin.
// This creates a typed adapter and registers it with the plugin.
//
// logger is plugin scoped, it is for messages the provider writes outside a
// provider call, such as in Init. During a call use Logger(ctx), which is
// bound to the resource and step being worked on.
func RegisterResourceProvider[T any](p *PluginBase, logger logger.Logger, state State, typeName, subTypeName string, resourceInstance T, provider ResourceProvider[T]) error {
	// Create a typed adapter for the provider, named after the block type so
	// the provider's logs are tagged with it
	adapter := NewTypedProviderAdapter(provider, resourceInstance)
	adapter.name = subTypeName

	// Initialize the adapter with state, functions (can be nil), and logger
	err := adapter.Init(state, nil, logger)
	if err != nil {
		return err
	}

	return p.RegisterType(typeName, subTypeName, resourceInstance, adapter)
}

// Plugin is a private interface that defines the contract between HCLConfig
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

// lifecycle methods

// Validate validates the given entity data.
func (p *PluginBase) Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	if entityType == "" || entitySubType == "" {
		return errors.New("entityType and entitySubType cannot be empty")
	}

	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Validate(ctx, entityData)
}

// Create creates a new entity.
func (p *PluginBase) Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return nil, errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Create(ctx, entityData)
}

// Destroy deletes an existing entity.
func (p *PluginBase) Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Destroy(ctx, entityData, false)
}

// Read reports the real entity, given its saved and configured copies.
func (p *PluginBase) Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return nil, errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Read(ctx, oldEntityData, newEntityData)
}

// Update updates an existing entity.
func (p *PluginBase) Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return nil, errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Update(ctx, entityData)
}

// Changed checks if the entity has changed by comparing old and new.
func (p *PluginBase) Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return false, errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Changed(ctx, oldEntityData, newEntityData)
}
