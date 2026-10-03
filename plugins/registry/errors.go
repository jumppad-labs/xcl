package registry

import "fmt"

// TypeNameClashError is returned when a type and subtype are already provided
// by a builtin, a registered type or a plugin. It is returned whichever way the
// clashing name reached the registry: RegisterType, or a plugin registered
// with RegisterPlugin, RegisterPluginWithPath or DiscoverPlugins, whose clash
// is reported when plugins load.
type TypeNameClashError struct {
	// Name is the clashing type, as its type and subtype joined by a dot, i.e.
	// "resource.postgres", or its type alone when it has no subtype
	Name string
	// Existing describes what already provides the name, i.e. "builtin",
	// "registered type" or "plugin"
	Existing string
}

func (e *TypeNameClashError) Error() string {
	return fmt.Sprintf("type %q is already provided by %s", e.Name, e.Existing)
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
