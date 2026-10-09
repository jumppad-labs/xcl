// Package entity holds the shared vocabulary for what happens to an entity,
// anything an xcl configuration declares, when a new configuration is applied.
//
// A provider answers Changed with a Change: NoChange, Update or Replace. It is
// also told, as a list of DependencyChange values, which of the resources it
// directly depends on the same apply will update or replace, so it can decide
// its own outcome from theirs.
//
// A PropertyChange describes one setting of a provider-backed resource that
// differs from its last apply: where it is, as a Path, and its previous and
// new values. Path.Within and PropertyChange.Within match a change against a
// setting without parsing text.
//
// The package imports only the standard library, so the plugin contract, the
// parser and any other package can share it without an import cycle.
package entity
