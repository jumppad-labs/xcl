package plugins

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/internal/plaintype"
	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/logger"
)

// ProviderAdapter defines a common interface for all providers regardless of their concrete type.
//
// This adapter pattern exists to bridge the gap between the plugin system's dynamic nature
// and Go's static type system. The plugin system must work with:
//   - String-based type identification ("resource.container", "resource.network", etc.)
//   - Raw []byte data from HCL configuration files
//   - Runtime provider lookup and invocation
//
// However, ResourceProvider[T] implementations work with strongly-typed resources and
// provide compile-time type safety. The ProviderAdapter interface allows the plugin
// system to work uniformly with all provider types without knowing their concrete types.
//
// Without this adapter, the plugin system would need to use reflection or type assertions
// to convert between []byte and concrete types, making the code complex and error-prone.
type ProviderAdapter interface {
	Init(state State, functions ProviderFunctions, logger logger.Logger) error
	Validate(ctx context.Context, entityData []byte) error
	Create(ctx context.Context, entityData []byte) ([]byte, error)
	Destroy(ctx context.Context, entityData []byte, force bool) error
	Read(ctx context.Context, oldEntityData []byte, newEntityData []byte) ([]byte, error)
	Update(ctx context.Context, entityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) ([]byte, error)
	Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)
}

// TypedProviderAdapter wraps a ResourceProvider[T] to implement ProviderAdapter.
//
// This adapter serves as a type-safe bridge between the generic plugin system and
// strongly-typed resource providers. It performs the following key functions:
//
//  1. Type Conversion: Converts []byte configuration data to concrete Go types (T)
//     using the stored schema and unmarshaling logic.
//
//  2. Method Translation: Translates ProviderAdapter method calls to ResourceProvider[T]
//     method calls, handling the type conversion seamlessly.
//
//  3. Error Handling: Provides consistent error handling and validation across all
//     provider types without requiring each provider to handle []byte parsing.
//
//  4. State Management: Manages provider initialization and state consistently across
//     all provider implementations.
//
// Why not use ResourceProvider[T] directly in the plugin system?
//   - The plugin system works with runtime type resolution (string -> provider lookup)
//   - Configuration data comes as []byte from HCL files, not as typed structs
//   - Each ResourceProvider[T] has a different concrete type T, making uniform handling impossible
//   - This adapter provides a uniform interface while preserving type safety within providers
//
// Example flow:
//
//	Plugin receives: ("resource", "container", []byte{...})
//	Adapter unmarshals: []byte -> *ContainerResource
//	Provider receives: ResourceProvider[*ContainerResource].Create(ctx, *ContainerResource)
type TypedProviderAdapter[T any] struct {
	provider     ResourceProvider[T]
	concreteType T
	state        State
	functions    ProviderFunctions

	// name is the block type the provider handles, when set the plugin
	// scoped logger passed to the provider adds the detail provider=<name>
	name string

	// built is the engine type a plain T is stored as, nil when T embeds
	// types.ResourceBase and is stored as itself
	built *plaintype.Built
}

// decode turns an entity's JSON into the T the provider works with. For a
// plain T it also returns the entity as the engine stores it, which encode
// writes the provider's result back into, so what T cannot hold, such as its
// metadata, is kept. Without data the resource is the zero value of T when
// optional, and an error otherwise.
func (a *TypedProviderAdapter[T]) decode(data []byte, optional bool) (T, any, error) {
	var resource T
	read := data != nil || !optional

	if a.built == nil {
		if read {
			if err := json.Unmarshal(data, &resource); err != nil {
				return resource, nil, err
			}
		}

		return resource, nil, nil
	}

	// without data the provider is given the zero value, nil for a pointer,
	// as it is for any T; what it returns is written into a new entity
	stored := a.built.New()
	if !read {
		return resource, stored, nil
	}

	if err := json.Unmarshal(data, stored); err != nil {
		return resource, nil, err
	}

	plain, err := a.built.ToPlain(stored)
	if err != nil {
		return resource, nil, err
	}

	// ToPlain returns a pointer to the plain struct, which is T itself when
	// the provider works with pointers
	if converted, ok := plain.(T); ok {
		return converted, stored, nil
	}

	return reflect.ValueOf(plain).Elem().Interface().(T), stored, nil
}

// encode turns the provider's result into the entity's JSON. stored is what
// decode returned with the resource the provider was given.
func (a *TypedProviderAdapter[T]) encode(resource T, stored any) ([]byte, error) {
	if a.built == nil {
		return wire.Marshal(resource)
	}

	if err := a.built.FromPlain(stored, resource); err != nil {
		return nil, err
	}

	return wire.Marshal(stored)
}

// NewTypedProviderAdapter creates a new adapter for a typed provider
func NewTypedProviderAdapter[T any](provider ResourceProvider[T], concreteType T) *TypedProviderAdapter[T] {
	return &TypedProviderAdapter[T]{
		provider:     provider,
		concreteType: concreteType,
	}
}

// Init initializes the provider with the plugin scoped logger, for messages
// written outside a provider call. When the adapter was registered for a
// block type, that logger adds the detail provider=<block type>.
func (a *TypedProviderAdapter[T]) Init(state State, functions ProviderFunctions, log logger.Logger) error {
	if a.name != "" {
		log = logger.WithTag(log, "provider", a.name)
	}

	a.state = state
	a.functions = functions
	return a.provider.Init(state, functions, log)
}

// Validate checks that the entity's JSON decodes into T.
func (a *TypedProviderAdapter[T]) Validate(ctx context.Context, entityData []byte) error {
	_, _, err := a.decode(entityData, false)
	return err
}

func (a *TypedProviderAdapter[T]) Create(ctx context.Context, entityData []byte) ([]byte, error) {
	resource, stored, err := a.decode(entityData, false)
	if err != nil {
		return nil, err
	}

	// the provider was initialized during registration
	created, err := a.provider.Create(ctx, resource)
	if err != nil {
		return nil, err
	}

	return a.encode(created, stored)
}

func (a *TypedProviderAdapter[T]) Destroy(ctx context.Context, entityData []byte, force bool) error {
	resource, _, err := a.decode(entityData, false)
	if err != nil {
		return err
	}

	return a.provider.Destroy(ctx, resource, force)
}

func (a *TypedProviderAdapter[T]) Read(ctx context.Context, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	// when no data is provided the resource is the zero value of T
	oldResource, _, err := a.decode(oldEntityData, true)
	if err != nil {
		return nil, err
	}

	newResource, stored, err := a.decode(newEntityData, true)
	if err != nil {
		return nil, err
	}

	// the error is returned unwrapped so that callers can check for
	// ErrNotFound
	read, err := a.provider.Read(ctx, oldResource, newResource)
	if err != nil {
		return nil, err
	}

	return a.encode(read, stored)
}

func (a *TypedProviderAdapter[T]) Update(ctx context.Context, entityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) ([]byte, error) {
	resource, stored, err := a.decode(entityData, false)
	if err != nil {
		return nil, err
	}

	updated, err := a.provider.Update(ctx, resource, changes, dependencies)
	if err != nil {
		return nil, err
	}

	return a.encode(updated, stored)
}

func (a *TypedProviderAdapter[T]) Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	oldResource, _, err := a.decode(oldEntityData, false)
	if err != nil {
		return entity.NoChange, err
	}

	newResource, _, err := a.decode(newEntityData, false)
	if err != nil {
		return entity.NoChange, err
	}

	// Call the provider's Changed method with both resources and the
	// dependencies the same apply will update or replace
	return a.provider.Changed(ctx, oldResource, newResource, changes, dependencies)
}
