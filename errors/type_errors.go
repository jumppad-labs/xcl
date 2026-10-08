package errors

import "fmt"

// TypeNameClashError is returned when a block type, with its subtype where it
// has one, is provided twice: by two plugins in one registry, by plugins in
// two registries, or by a plugin and a type declared with WithType, or a
// builtin. A plugin's clash is reported when plugins load, whatever order the
// registrations were made in. There is no precedence between providers.
type TypeNameClashError struct {
	// Name is the clashing type, as its type and subtype joined by a dot, i.e.
	// "resource.postgres", or its type alone when it has no subtype
	Name string

	// Provider is what provides the name a second time: a plugin's name, or
	// "type <Go type>" for a declared type
	Provider string

	// Registry is the registry Provider came from, empty for a declared type
	Registry string

	// Existing is what already provides the name: "builtin", a plugin's name,
	// or "type <Go type>" for a declared type
	Existing string

	// ExistingRegistry is the registry Existing came from, empty for a
	// builtin or a declared type
	ExistingRegistry string
}

func (e *TypeNameClashError) Error() string {
	existing := describeProvider(e.Existing, e.ExistingRegistry)

	if e.Provider == "" {
		return fmt.Sprintf("type %q is already provided by %s", e.Name, existing)
	}

	return fmt.Sprintf("type %q is provided by both %s and %s", e.Name, existing, describeProvider(e.Provider, e.Registry))
}

// describeProvider names a provider, with its registry when it has one
func describeProvider(provider, registry string) string {
	if registry == "" {
		return provider
	}

	return fmt.Sprintf("%s (registry %s)", provider, registry)
}

// TypeFormError is returned when a type keyword is registered in the other
// form from the one it already has: with a subtype when it is already used
// without one, or without one when it already takes one. A keyword is used in
// one form only, so an address such as server.big.web can always be read by
// position. "resource" always takes a subtype.
type TypeFormError struct {
	// Type is the type keyword
	Type string
	// TakesSubtype is the form the keyword already has
	TakesSubtype bool
}

func (e *TypeFormError) Error() string {
	if e.TakesSubtype {
		return fmt.Sprintf("type %q takes a subtype, i.e. '%s \"<subtype>\" \"<name>\" {}', so it must be registered with one", e.Type, e.Type)
	}

	return fmt.Sprintf("type %q is declared without a subtype, i.e. '%s \"<name>\" {}', so it can not be registered with one", e.Type, e.Type)
}
