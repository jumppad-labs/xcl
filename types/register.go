package types

import (
	"fmt"
	"reflect"
)

type ErrTypeNotRegistered struct {
	Type string
}

func (e *ErrTypeNotRegistered) Error() string {
	return fmt.Sprintf("type %s, not registered", e.Type)
}

func NewTypeNotRegisteredError(t string) *ErrTypeNotRegistered {
	return &ErrTypeNotRegistered{Type: t}
}

type RegisteredTypes map[string]any

// CreateResource creates a new instance of a resource from one of the registered types.
func (r RegisteredTypes) CreateResource(resourceType, resourceName string) (any, error) {
	// check that the type exists
	if t, ok := r[resourceType]; ok {
		ptr := reflect.New(reflect.TypeOf(t).Elem())

		res := ptr.Interface()

		// resourceType is the type here, i.e. one of the builtins, which
		// carry no subtype. Callers holding a subtype set both themselves
		meta, _ := GetMeta(res)
		meta.Name = resourceName
		meta.Type = resourceType
		meta.Subtype = ""
		meta.Properties = make(map[string]any)

		return res, nil
	}

	return nil, fmt.Errorf("unable to create resource: %s", NewTypeNotRegisteredError(resourceType))
}

// TypeInfo is everything the system knows about a declarable type, held in one
// place so that registration, address parsing and Go type lookup all read the
// same record rather than each keeping their own.
//
// An entity has a type and an optional subtype. The type is the keyword a
// declaration leads with; a type that takes a subtype is declared with it as
// the first label, i.e. resource "container" "nics" or server "big" "web",
// and one that does not carries only the name, i.e. cache "main". A type
// keyword takes a subtype for every declaration or for none, never both, so
// an address can always be read by position.
type TypeInfo struct {
	// Type is the keyword a declaration leads with: "resource", "server",
	// "variable"
	Type string

	// Subtype is the first label of a declaration whose type takes one, i.e.
	// the "container" in resource "container" "nics". It is empty for a type
	// declared with only a name
	Subtype string

	// Builtin is true for the types xcl interprets itself: variable, output,
	// module and root. They carry no subtype
	Builtin bool

	// Prototype is the Go value new instances are built from. It is nil for a
	// plugin provided type, which exists host side only as a schema and so has
	// no Go type to reflect against
	Prototype any
}

// AddressPath returns the address segments this type is reached by:
// {"resource", "container"} or {"server", "big"} for a type with a subtype,
// and {"cache"} or {"variable"} for one without.
func (t TypeInfo) AddressPath() []string {
	if t.Subtype != "" {
		return []string{t.Type, t.Subtype}
	}

	return []string{t.Type}
}

// Key returns the address path joined by dots, i.e. "resource.container" or
// "cache", which names the type uniquely.
func (t TypeInfo) Key() string {
	return TypeKey(t.Type, t.Subtype)
}

// TypeKey returns the name of the type entityType with subtype, i.e.
// "resource.container", or entityType alone when there is no subtype.
func TypeKey(entityType, subtype string) string {
	if subtype == "" {
		return entityType
	}

	return entityType + "." + subtype
}
