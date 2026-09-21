package plugins

import "errors"

// ErrNotFound is returned by a provider's Read when the real resource no longer
// exists. xcl creates the resource again when it sees this error.
//
// Check for it with errors.Is. It keeps its identity when the provider runs in
// a separate plugin process.
//
// This means the real infrastructure is gone, not that the configuration does
// not declare the entity. It is deliberately distinct from the lookup
// surface's xcl.ErrNotFound, which means no entity is declared at an address:
// same words, different meaning, and neither matches the other.
var ErrNotFound = errors.New("resource not found")
