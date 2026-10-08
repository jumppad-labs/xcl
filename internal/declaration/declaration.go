// Package declaration checks the shape of a plain Go type declared as a block
// type. It is shared by the registries a type is declared on and the catalog
// it is registered in, so both report a malformed declaration alike.
package declaration

import (
	"fmt"
	"reflect"

	"github.com/jumppad-labs/xcl/types"
)

// Validate checks the shape of a type declaration on its own, without any
// other declaration: the type must be named, take at most one subtype that
// is not empty, and prototype must be a non-nil pointer to a struct that
// embeds types.ResourceBase. The checks that need every declaration, such as
// duplicates, are made when the type is registered in a catalog.
func Validate(prototype any, name ...string) error {
	if len(name) == 0 || name[0] == "" {
		return fmt.Errorf("an entity type must be named")
	}

	if len(name) > 2 {
		return fmt.Errorf("type %q takes at most one subtype, got %d", name[0], len(name)-1)
	}

	entityType := name[0]

	sub := ""
	if len(name) == 2 {
		sub = name[1]
		if sub == "" {
			return fmt.Errorf("type %q was given an empty subtype, leave it out to register the type without one", entityType)
		}
	}

	key := types.TypeKey(entityType, sub)

	value := reflect.ValueOf(prototype)
	if prototype == nil || value.Kind() != reflect.Ptr || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase", key)
	}

	if _, err := types.GetMeta(prototype); err != nil {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase: %w", key, err)
	}

	return nil
}
