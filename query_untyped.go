package xcl

import (
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// FindEntitiesByType returns every entity of the type the segments name,
// exactly as FindByType does, but without a Go type to convert to: each
// entity is returned as the Config stores it. It is for a host that has no Go
// type for what it is asking about, such as a language binding driving the
// core, or a program reading a plugin's entities generically.
//
// The segments follow the rules of FindByType, with the same errors: the
// first names a type and the second, where the type takes one, its subtype;
// at least one segment is required; a leading segment that is not a type
// returns an error wrapping ErrUnknownType, and segments that span more than
// one type, as a type that takes a subtype does on its own, return one
// wrapping ErrNotTypeable. A well formed query that matches nothing returns an
// empty result and a nil error.
func (c *Config) FindEntitiesByType(path ...string) ([]any, error) {
	return c.entitiesByType(path)
}

// FindOneEntity returns the single entity of the type the segments name,
// exactly as FindOne does, but without a Go type to convert to: the entity is
// returned as the Config stores it. The segments and their errors are those
// of FindEntitiesByType. Finding none returns an error wrapping ErrNotFound,
// and finding more than one returns one wrapping ErrNotUnique, whose detail
// reports how many were found.
func (c *Config) FindOneEntity(path ...string) (any, error) {
	found, err := c.entitiesByType(path)
	if err != nil {
		return nil, err
	}

	one, ok, err := oneOf(found, path)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, &xclerrors.NotFoundError{Address: strings.Join(path, ".")}
	}

	return one, nil
}
