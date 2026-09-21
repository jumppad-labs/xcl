package errors

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// The sentinels below name every way a lookup against a parsed configuration
// can fail. Each is matched by identity with errors.Is, and each is wrapped by
// a detail type carrying the specifics, recovered with errors.As.
//
// They are declared here, rather than in the package that returns them,
// because the state layer raises one of them too and the public package
// already imports state. A neutral home both can depend on is what lets one
// not-found concept exist at all. The public package re-exports them, so a
// caller matches xcl.ErrNotFound while keeping the standard library's errors
// package unaliased in their own code.
//
// ErrNotFound and ErrNotUnique are ordinary outcomes a caller handles. The
// other five mean the question itself had no answer, which is a caller bug.
var (
	// ErrNotFound is returned when no entity is declared at the address given.
	// It is distinct from plugins.ErrNotFound, which means the real
	// infrastructure behind a declared entity has gone missing.
	ErrNotFound = errors.New("no entity found at address")

	// ErrUnknownType is returned when the leading segment of a query does not
	// name a kind the configuration knows.
	ErrUnknownType = errors.New("unknown type")

	// ErrNotTypeable is returned when the segments given match entities of
	// more than one Go type, so the result cannot be typed. Querying the
	// resource kind alone does this, as does querying published values.
	ErrNotTypeable = errors.New("query spans more than one type")

	// ErrNotRegistered is returned when a Go type has no registered name to
	// derive an address from. A plugin provides a schema and no Go type, so a
	// plugin backed type is always reported this way.
	ErrNotRegistered = errors.New("type is not registered")

	// ErrTypeMismatch is returned when an entity is not of the Go type asked
	// for. Conversion refuses rather than returning a value with some of its
	// fields unset.
	ErrTypeMismatch = errors.New("entity is not of the requested type")

	// ErrNotAnEntity is returned when a Go type carries no addressable
	// identity, because it is a block nested inside another declaration rather
	// than a declaration of its own.
	ErrNotAnEntity = errors.New("type is not an entity")

	// ErrNotUnique is returned when a query expecting exactly one entity
	// matched more than one. The detail reports how many.
	ErrNotUnique = errors.New("more than one entity matched")
)

// NotFoundError reports the address that matched nothing.
type NotFoundError struct {
	Address string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no entity found at address %q", e.Address)
}

func (e *NotFoundError) Unwrap() error { return ErrNotFound }

// UnknownTypeError reports the segments queried and the one that named nothing.
type UnknownTypeError struct {
	Segments []string
	Name     string
}

func (e *UnknownTypeError) Error() string {
	return fmt.Sprintf("%q is not a known type, in query %q", e.Name, strings.Join(e.Segments, "."))
}

func (e *UnknownTypeError) Unwrap() error { return ErrUnknownType }

// NotTypeableError reports the segments queried and names the call that does
// return what was asked for, where there is one.
type NotTypeableError struct {
	Segments []string
	Use      string
}

func (e *NotTypeableError) Error() string {
	msg := fmt.Sprintf("query %q matches entities of more than one type", strings.Join(e.Segments, "."))
	if e.Use != "" {
		msg = fmt.Sprintf("%s, use %s instead", msg, e.Use)
	}

	return msg
}

func (e *NotTypeableError) Unwrap() error { return ErrNotTypeable }

// NotRegisteredError reports the Go type that has no registered name, and
// names the query form that works for it.
type NotRegisteredError struct {
	Type reflect.Type
	Use  string
}

func (e *NotRegisteredError) Error() string {
	msg := fmt.Sprintf("type %s is not registered", typeName(e.Type))
	if e.Use != "" {
		msg = fmt.Sprintf("%s, use %s instead", msg, e.Use)
	}

	return msg
}

func (e *NotRegisteredError) Unwrap() error { return ErrNotRegistered }

// TypeMismatchError reports the entity asked for, the Go type requested, and
// the type the entity actually is.
type TypeMismatchError struct {
	Address string
	Want    reflect.Type
	Got     string
}

func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf("entity %q is %s, not %s", e.Address, e.Got, typeName(e.Want))
}

func (e *TypeMismatchError) Unwrap() error { return ErrTypeMismatch }

// NotAnEntityError reports a Go type that has no address of its own, and the
// declaration it is reached through.
type NotAnEntityError struct {
	Type           reflect.Type
	ReachedThrough string
}

func (e *NotAnEntityError) Error() string {
	msg := fmt.Sprintf("type %s is not an entity, it has no address of its own", typeName(e.Type))
	if e.ReachedThrough != "" {
		msg = fmt.Sprintf("%s, reach it through the %s that contains it", msg, e.ReachedThrough)
	}

	return msg
}

func (e *NotAnEntityError) Unwrap() error { return ErrNotAnEntity }

// NotUniqueError reports the query and how many entities matched it.
type NotUniqueError struct {
	Segments []string
	Count    int
}

func (e *NotUniqueError) Error() string {
	return fmt.Sprintf("query %q matched %d entities, expected exactly one", strings.Join(e.Segments, "."), e.Count)
}

func (e *NotUniqueError) Unwrap() error { return ErrNotUnique }

// typeName renders a Go type for an error message, tolerating a nil type and
// the anonymous structs that reflection built plugin resources are.
func typeName(t reflect.Type) string {
	if t == nil {
		return "unknown type"
	}

	if name := t.String(); name != "" {
		return name
	}

	return "anonymous type"
}
