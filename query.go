package xcl

import (
	"fmt"
	"reflect"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/types"
)

// Find returns the entity declared at address, as T.
//
// The address is complete: resource.container.mine, variable.region,
// module.db.output.connection. A bare name is not an address and is not
// inferred. A module relative address, one that is not normalised, and one
// carrying a trailing attribute suffix all resolve to the same entity.
//
// An address that matches nothing returns an error wrapping ErrNotFound. An
// entity that is not a T returns one wrapping ErrTypeMismatch, and never a
// value with some of its fields unset.
//
// Where the address names a value the configuration publishes, the resolved
// value is returned rather than the declaration that produced it, so a caller
// does not need to know such values are held differently in order to ask for
// one.
func Find[T any](c *Config, address string) (*T, error) {
	return find[T](c, address)
}

// find is the implementation behind both spellings of the address lookup. It
// is written once and takes the configuration as an ordinary parameter, so
// neither the function nor the method form carries any logic of its own.
func find[T any](c *Config, address string) (*T, error) {
	fqrn, err := c.addressParser().Parse(address)
	if err != nil {
		return nil, err
	}

	entity, found := resources.Match(c.Entities(), fqrn)
	if !found {
		return nil, &xclerrors.NotFoundError{Address: address}
	}

	// a published value is asked for by address like anything else, and
	// yields the value rather than the declaration that produced it
	if output, ok := entity.(*resources.Output); ok {
		return As[T](output.Value)
	}

	return As[T](entity)
}

// As converts an entity, typically one taken from an untyped enumeration, into
// a caller's Go type.
//
// An entity that already is a *T is returned as it stands, so the caller holds
// the value in the configuration rather than a copy of it. Anything else is
// copied into a new T, which is how a plugin provided entity built from a
// schema is reached.
//
// Where the entity's own Go type says it cannot be a T, the conversion is
// refused. The copy underneath is a JSON round trip that drops unknown fields
// and zeroes absent ones, so without that refusal converting one entity into
// an unrelated type would quietly yield a value with most of its fields unset.
func As[T any](entity any) (*T, error) {
	if entity == nil {
		return nil, &xclerrors.TypeMismatchError{Want: reflect.TypeFor[T](), Got: "nothing"}
	}

	if typed, ok := entity.(*T); ok {
		return typed, nil
	}

	if err := convertible[T](entity); err != nil {
		return nil, err
	}

	typed := new(T)
	if err := schema.UnmarshalUntyped(entity, typed); err != nil {
		// the copy is the last word on whether the entity can be a T, so a
		// failure here is a type mismatch like any other and is reported as
		// one. The underlying error is kept for a caller that wants the
		// detail, but it is not what they have to match on
		return nil, fmt.Errorf("%w: %w", &xclerrors.TypeMismatchError{
			Address: entityAddress(entity),
			Want:    reflect.TypeFor[T](),
			Got:     describeType(entity),
		}, err)
	}

	return typed, nil
}

// convertible refuses a conversion the entity's own Go type rules out.
//
// A named struct is its own type and nothing else: a Database is not a
// Container, whatever their fields look like. An entity built by reflection
// from a plugin schema is an anonymous struct with no name to compare, so it
// is left to the copy, which is the only way to reach those types at all.
func convertible[T any](entity any) error {
	want := reflect.TypeFor[T]()

	et := reflect.TypeOf(entity)
	for et != nil && et.Kind() == reflect.Ptr {
		et = et.Elem()
	}

	if et == nil || et.Kind() != reflect.Struct || et.Name() == "" {
		// nothing nameable to compare, so let the copy decide
		return nil
	}

	if want.Kind() == reflect.Struct && want.Name() != "" && et != want {
		return &xclerrors.TypeMismatchError{
			Address: entityAddress(entity),
			Want:    want,
			Got:     et.String(),
		}
	}

	return nil
}

// entityAddress returns the address of an entity, or empty for a value that is
// not one, such as a published value.
func entityAddress(entity any) string {
	meta, err := types.GetMeta(entity)
	if err != nil {
		return ""
	}

	return meta.ID
}

// describeType names what an entity actually is, for an error a caller reads.
func describeType(entity any) string {
	t := reflect.TypeOf(entity)
	if t == nil {
		return "nothing"
	}

	return t.String()
}

// FindByType returns every entity whose address begins with the segments
// given, as T.
//
// The segments are matched positionally from the root, exactly as they appear
// in an address: FindByType[Container](c, "resource", "container") for an
// entity declared resource "container" "nics", and
// FindByType[Container](c, "container") for one declared container "nics".
// Segment one is always the kind, so a variety on its own matches nothing and
// says so rather than matching in the wrong position.
//
// At least one segment is required; the no-segment form is All.
//
// A well formed query that matches nothing returns an empty result and a nil
// error. A query that cannot be answered returns an error: one wrapping
// ErrUnknownType where the leading segment is not a kind, and one wrapping
// ErrNotTypeable where the segments span more than one Go type, as the
// resource kind alone does.
func FindByType[T any](c *Config, path ...string) ([]*T, error) {
	return findByType[T](c, path...)
}

// FindOne returns the single entity matching the segments given.
//
// It is for the entity a configuration is expected to declare exactly one of,
// and returns it directly rather than a collection to index into. Finding none
// returns an error wrapping ErrNotFound, and finding more than one returns one
// wrapping ErrNotUnique, whose detail reports how many were found.
func FindOne[T any](c *Config, path ...string) (*T, error) {
	return findOne[T](c, path...)
}

func findByType[T any](c *Config, path ...string) ([]*T, error) {
	if err := addressable[T](); err != nil {
		return nil, err
	}

	if len(path) == 0 {
		return nil, &xclerrors.NotTypeableError{Segments: path, Use: "All"}
	}

	if err := c.typeable(path); err != nil {
		return nil, err
	}

	found := []*T{}

	for _, e := range c.Entities() {
		meta, err := types.GetMeta(e)
		if err != nil {
			continue
		}

		if meta.Type != path[0] {
			continue
		}

		if len(path) > 1 && meta.Subtype != path[1] {
			continue
		}

		typed, err := As[T](e)
		if err != nil {
			return nil, err
		}

		found = append(found, typed)
	}

	return found, nil
}

func findOne[T any](c *Config, path ...string) (*T, error) {
	found, err := findByType[T](c, path...)
	if err != nil {
		return nil, err
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return nil, &xclerrors.NotFoundError{Address: strings.Join(path, ".")}
	default:
		return nil, &xclerrors.NotUniqueError{Segments: path, Count: len(found)}
	}
}

// typeable reports whether the segments given pin exactly one Go type, which
// is what makes a result typeable at all.
//
// Segment one names a kind: one of the stanza keywords, or a type declared by
// its own keyword. The resource kind spans every variety declared under it, so
// it needs a second segment to pin a type, and published values span whatever
// the configuration publishes, so they are never typeable and point at the
// call that does return them.
func (c *Config) typeable(path []string) error {
	kind := path[0]

	switch kind {
	case types.TypeResource:
		if len(path) < 2 {
			return &xclerrors.NotTypeableError{Segments: path, Use: "FindByType with the variety, i.e. (\"resource\", \"container\")"}
		}

		// the variety must be a type that can actually be declared under the
		// resource keyword. A type declared by its own keyword, or a builtin
		// kind, never can be, so naming one here asks a question no
		// configuration could answer
		info, ok := c.typeInfo(path[1])
		if !ok || info.Bare || info.Builtin {
			return &xclerrors.UnknownTypeError{Segments: path, Name: path[1]}
		}

		return nil

	case resources.TypeOutput:
		return &xclerrors.NotTypeableError{Segments: path, Use: "Outputs"}

	case resources.TypeVariable, resources.TypeModule, resources.TypeRoot:
		// each is exactly one Go type
		return nil
	}

	// anything else is only a kind when it is declared by its own keyword
	if info, ok := c.typeInfo(kind); ok && info.Bare {
		return nil
	}

	return &xclerrors.UnknownTypeError{Segments: path, Name: kind}
}

func (c *Config) typeInfo(name string) (types.TypeInfo, bool) {
	if c.pluginRegistry == nil {
		return types.TypeInfo{}, false
	}

	return c.pluginRegistry.Type(name)
}

// addressable refuses a query for a type that has no address of its own,
// because it is a block nested inside another declaration rather than a
// declaration in its own right. Answering such a query with an empty result
// would read as "you declared none of those", which is not what is true.
func addressable[T any]() error {
	var zero T
	if _, err := types.GetMeta(&zero); err != nil {
		return &xclerrors.NotAnEntityError{
			Type:           reflect.TypeFor[T](),
			ReachedThrough: "declaration",
		}
	}

	return nil
}

// All returns every entity of the Go type T, without naming it as a string.
//
// The addressing is worked out from how T was registered, so All[Container]
// returns what FindByType[Container]("resource", "container") returns for a
// type declared with the resource keyword, and what
// FindByType[Container]("container") returns for one declared by its own.
//
// A plugin provides a schema and no Go type, so a plugin backed entity cannot
// be matched this way. Asking for one returns an error wrapping
// ErrNotRegistered, naming the kind lookup as the form that does work.
func All[T any](c *Config) ([]*T, error) {
	return all[T](c)
}

func all[T any](c *Config) ([]*T, error) {
	if err := addressable[T](); err != nil {
		return nil, err
	}

	if c.pluginRegistry == nil {
		return nil, &xclerrors.NotRegisteredError{Type: reflect.TypeFor[T](), Use: "FindByType with the kind and variety"}
	}

	path, ok := c.pluginRegistry.TypePath(reflect.TypeFor[T]())
	if !ok {
		return nil, &xclerrors.NotRegisteredError{Type: reflect.TypeFor[T](), Use: "FindByType with the kind and variety"}
	}

	// deliberately goes through the kind lookup rather than scanning again, so
	// the two cannot disagree about what a type matches
	return findByType[T](c, path...)
}
