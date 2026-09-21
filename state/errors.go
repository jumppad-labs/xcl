package state

import (
	"fmt"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// ResourceNotFoundError is returned when a resource cannot be found
type ResourceNotFoundError struct {
	Resource string
}

func (r ResourceNotFoundError) Error() string {
	return "resource not found: " + r.Resource
}

// Is reports this as the lookup surface's not-found condition, so a failure to
// find a declared entity reads as one condition wherever it was raised. The
// receiver stays a value because the existing construction sites and the
// errors.As callers in the parser depend on it.
//
// It stays distinct from plugins.ErrNotFound, which means the real
// infrastructure behind a declared entity has gone missing.
func (r ResourceNotFoundError) Is(target error) bool {
	return target == xclerrors.ErrNotFound
}

// ResourceExistsError is returned when trying to add a duplicate resource
type ResourceExistsError struct {
	Name string
}

func (r ResourceExistsError) Error() string {
	return "resource already exists: " + r.Name
}

// UnknownTypesError is returned when saved state holds records that cannot be
// loaded: a resource whose type is not registered, or a record whose metadata
// cannot be read at all. Loading fails rather than returning the records it
// could read, because a partial configuration would be written back over the
// file on the next save and lose the rest.
type UnknownTypesError struct {
	// Types is the sorted, unique list of what could not be loaded. A record
	// names the type it asked for where that is what failed, and otherwise
	// names itself by its id, or by its position in the file as "entry N"
	Types []string
}

func (u UnknownTypesError) Error() string {
	return fmt.Sprintf(
		"saved state could not be loaded: %s (a bare name is a type that is not registered, so register its type or plugin; an id or an entry position is a record that cannot be read at all)",
		strings.Join(u.Types, ", "),
	)
}
