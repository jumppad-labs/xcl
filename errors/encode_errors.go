package errors

import (
	"errors"
	"fmt"
)

// The sentinels below name every way turning an entity, or an entity's saved
// data, into configuration text can fail. Each is matched by identity with
// errors.Is, and each is wrapped by a detail type carrying the specifics,
// recovered with errors.As.
//
// They are declared here, rather than in the package that returns them,
// because the root package raises them and the shared reader of saved records
// that the state layer also calls raises two of them. A neutral home both can
// depend on is what lets one concept exist at all. The public package
// re-exports them, so a caller matches xcl.ErrUnregisteredType while keeping
// the standard library's errors package unaliased in their own code.
var (
	// ErrUnregisteredType is returned when saved data names a type the
	// registry cannot create. Register the type or its plugin, and remember
	// that a plugin's types resolve only once the registry has loaded.
	ErrUnregisteredType = errors.New("type is not known to the registry")

	// ErrInvalidSavedData is returned when data is not one saved entity
	// record: it is not valid JSON, or it carries no readable meta, type or
	// name, or it does not fit the type it names.
	ErrInvalidSavedData = errors.New("data is not a saved entity")

	// ErrNotEncodable is returned when a value cannot be written as
	// configuration: it is not an entity, it is a builtin such as a variable,
	// output or module, or it holds a value that has no configuration form.
	ErrNotEncodable = errors.New("entity cannot be encoded as configuration")
)

// UnregisteredTypeError reports the type the registry does not know.
type UnregisteredTypeError struct {
	Type string
}

func (e *UnregisteredTypeError) Error() string {
	return fmt.Sprintf("type %q is not known to the registry", e.Type)
}

func (e *UnregisteredTypeError) Unwrap() error { return ErrUnregisteredType }

// InvalidSavedDataError reports the record that could not be read and why. ID
// is the record's own id where the record was readable enough to name itself,
// and empty otherwise.
type InvalidSavedDataError struct {
	ID  string
	Err error
}

func (e *InvalidSavedDataError) Error() string {
	subject := "data is not a saved entity"
	if e.ID != "" {
		subject = fmt.Sprintf("record %q is not a saved entity", e.ID)
	}

	if e.Err != nil {
		return fmt.Sprintf("%s: %s", subject, e.Err)
	}

	return subject
}

// Unwrap returns ErrInvalidSavedData and the reason the record could not be
// read, so both answer errors.Is
func (e *InvalidSavedDataError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrInvalidSavedData}
	}

	return []error{ErrInvalidSavedData, e.Err}
}

// NotEncodableError reports what could not be written as configuration and
// why. What names the value, such as its address or its Go type, and Reason
// says what stopped it.
type NotEncodableError struct {
	What   string
	Reason string
	Err    error
}

func (e *NotEncodableError) Error() string {
	msg := fmt.Sprintf("%s cannot be encoded as configuration", e.What)
	if e.Reason != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.Reason)
	}

	if e.Err != nil {
		msg = fmt.Sprintf("%s: %s", msg, e.Err)
	}

	return msg
}

// Unwrap returns ErrNotEncodable and the reason it could not be written, so
// both answer errors.Is
func (e *NotEncodableError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrNotEncodable}
	}

	return []error{ErrNotEncodable, e.Err}
}
