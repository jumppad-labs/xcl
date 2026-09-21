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

		// resourceType is the stanza kind here, i.e. one of the single label
		// builtins. Callers holding a variety set the two axes themselves
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
type TypeInfo struct {
	// Name is the keyword a declaration leads with for a bare or builtin type,
	// and the variety for a kind led one: "container", "postgres", "variable"
	Name string

	// Bare is true when the type is declared by its own keyword with a single
	// label, i.e. container "nics", and addressed container.nics rather than
	// resource.container.nics. A type is registered under one form, never both
	Bare bool

	// Builtin is true for the types xcl interprets itself: variable, output,
	// module and root. They carry no variety
	Builtin bool

	// Prototype is the Go value new instances are built from. It is nil for a
	// plugin provided type, which exists host side only as a schema and so has
	// no Go type to reflect against
	Prototype any
}

// AddressPath returns the address segments this type is reached by:
// {"resource", "container"} for a kind led type, {"container"} for a bare one
// and {"variable"} for a builtin. Never both forms.
func (t TypeInfo) AddressPath() []string {
	if t.Bare || t.Builtin {
		return []string{t.Name}
	}

	return []string{TypeResource, t.Name}
}
