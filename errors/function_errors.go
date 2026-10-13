package errors

import "fmt"

// FunctionNameClashError is returned when a function name is offered twice:
// by an application function and a builtin, by two application functions, in
// one registry or across two, by two plugins, or by a plugin and an
// application function. Function names are bare, so there is no namespace to
// tell them apart and no precedence between providers. NewConfig returns it
// for an application function that clashes with a builtin or another
// application function. The first operation that loads plugins returns it for
// any clash involving a plugin.
type FunctionNameClashError struct {
	// Name is the clashing function name, i.e. "upper"
	Name string

	// Provider is what provides the name a second time: a plugin's name, or
	// "function <Go type>" for an application function
	Provider string

	// Registry is the registry Provider came from
	Registry string

	// Existing is what already provides the name: "builtin", a plugin's
	// name, or "function <Go type>" for an application function
	Existing string

	// ExistingRegistry is the registry Existing came from, empty for a
	// builtin
	ExistingRegistry string
}

func (e *FunctionNameClashError) Error() string {
	existing := describeProvider(e.Existing, e.ExistingRegistry)

	if e.Provider == "" {
		return fmt.Sprintf("function %q is already provided by %s", e.Name, existing)
	}

	return fmt.Sprintf("function %q is provided by both %s and %s", e.Name, existing, describeProvider(e.Provider, e.Registry))
}

// InvalidFunctionError is returned by NewConfig when an application function
// cannot be called from configuration: it is not a Go func, it is variadic,
// it does not return a result and an error, or a parameter or its result has
// a type configuration cannot carry. Err names the problem.
type InvalidFunctionError struct {
	// Name is the name the function was registered under
	Name string

	// Registry is the registry the function came from
	Registry string

	// Err is the problem with the function's signature
	Err error
}

func (e *InvalidFunctionError) Error() string {
	if e.Registry == "" {
		return fmt.Sprintf("function %q cannot be called from configuration: %s", e.Name, e.Err)
	}

	return fmt.Sprintf("function %q (registry %s) cannot be called from configuration: %s", e.Name, e.Registry, e.Err)
}

func (e *InvalidFunctionError) Unwrap() error {
	return e.Err
}
