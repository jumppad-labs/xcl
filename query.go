package xcl

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/declaration"
	"github.com/jumppad-labs/xcl/internal/plaintype"
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
// An output address returns the types.Output entity like any other entity,
// and its Value holds the published value. Asking for an output as any other
// type, a plain string for instance, returns an error wrapping
// ErrTypeMismatch; Config.Outputs returns every published value at once.
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

	return As[T](entity)
}

// As converts an entity, typically one taken from an untyped enumeration, into
// a caller's Go type.
//
// An entity of a declared or plugin type is handed back as a copy of its
// plain struct, labels and Meta filled; changing the copy leaves the
// configuration as it is, as only configuration and a provider's results set
// an entity. An entity that already is a *T, such as a builtin types.Output,
// is returned as it stands.
//
// Where the entity's own Go type says it cannot be a T, the conversion is
// refused. The copy underneath is a JSON round trip that drops unknown fields
// and zeroes absent ones, so without that refusal converting one entity into
// an unrelated type would quietly yield a value with most of its fields unset.
func As[T any](entity any) (*T, error) {
	typed, err := asType(entity, reflect.TypeFor[T]())
	if err != nil {
		return nil, err
	}

	return typed.(*T), nil
}

// asType is the conversion behind As, written over a runtime type so a caller
// that only learns the type it wants by reflection converts an entity by
// exactly the same rule. On success the result is always a pointer to want.
func asType(entity any, want reflect.Type) (any, error) {
	if entity == nil {
		return nil, &xclerrors.TypeMismatchError{Want: want, Got: "nothing"}
	}

	if reflect.TypeOf(entity) == reflect.PointerTo(want) {
		return entity, nil
	}

	// an entity of a plain type is held as the type built from it, and is
	// handed back as the plain type it was declared with, labels included;
	// like any registered type it converts into nothing else. A type built
	// from a schema, a plugin's or another language's, is unnamed and has no
	// Go type of its own here, so it converts as any entity does, below
	if built, ok := plaintype.Lookup(reflect.TypeOf(entity)); ok && built.Plain.Name() != "" {
		if built.Plain != want {
			return nil, &xclerrors.TypeMismatchError{
				Address: entityAddress(entity),
				Want:    want,
				Got:     built.Plain.String(),
			}
		}

		return built.ToPlain(entity)
	}

	if err := convertibleTo(entity, want); err != nil {
		return nil, err
	}

	// a sensitive value is never copied into a plain field, there is no way
	// to unwrap one automatically
	if field, blocked := sensitiveFieldBlocked(reflect.TypeOf(entity), want); blocked {
		return nil, &xclerrors.TypeMismatchError{
			Address: entityAddress(entity),
			Want:    want,
			Got:     describeType(entity),
			Field:   field,
		}
	}

	// a plugin's entity asked for as a plain struct, its plugin's own type,
	// is copied into the type built from it and handed back as the plain
	// struct, so its labels and Meta are filled as they are for any plain
	// type
	if built, ok := plainFor(entity, want); ok {
		stored := built.New()
		if err := schema.UnmarshalUntyped(entity, stored); err != nil {
			return nil, fmt.Errorf("%w: %w", &xclerrors.TypeMismatchError{
				Address: entityAddress(entity),
				Want:    want,
				Got:     describeType(entity),
			}, err)
		}

		// the copy is as the entity's JSON has it, metadata included
		return built.ToPlain(stored)
	}

	typed := reflect.New(want).Interface()
	if err := schema.UnmarshalUntyped(entity, typed); err != nil {
		// the copy is the last word on whether the entity can be a T, so a
		// failure here is a type mismatch like any other and is reported as
		// one. The underlying error is kept for a caller that wants the
		// detail, but it is not what they have to match on
		return nil, fmt.Errorf("%w: %w", &xclerrors.TypeMismatchError{
			Address: entityAddress(entity),
			Want:    want,
			Got:     describeType(entity),
		}, err)
	}

	return typed, nil
}

// plainFor builds want, a plain struct, as a block type of entity's shape,
// one label with a subtype's two, and reports false for a want that embeds
// types.ResourceBase or cannot be built so
func plainFor(entity any, want reflect.Type) (*plaintype.Built, bool) {
	if want.Kind() != reflect.Struct || !declaration.IsPlain(reflect.New(want).Interface()) {
		return nil, false
	}

	meta, err := types.GetMeta(entity)
	if err != nil {
		return nil, false
	}

	labels := 1
	if meta.Subtype != "" {
		labels = 2
	}

	built, err := plaintype.Build(want, labels)
	if err != nil {
		return nil, false
	}

	return built, true
}

// convertibleTo refuses a conversion the entity's own Go type rules out.
//
// A named struct is its own type and nothing else: a Database is not a
// Container, whatever their fields look like. An entity built by reflection
// from a plugin schema is an anonymous struct with no name to compare, so it
// is left to the copy, which is the only way to reach those types at all.
func convertibleTo(entity any, want reflect.Type) error {
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
// Segment one is always the type, so a subtype on its own matches nothing and
// says so rather than matching in the wrong position.
//
// At least one segment is required; the no-segment form is All.
//
// A well formed query that matches nothing returns an empty result and a nil
// error. A query that cannot be answered returns an error: one wrapping
// ErrUnknownType where the leading segment is not a type, and one wrapping
// ErrNotTypeable where the segments span more than one Go type, as a type
// that takes a subtype does when it is given alone.
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
	found, err := c.entitiesOf(reflect.TypeFor[T](), path...)
	if err != nil {
		return nil, err
	}

	typed := make([]*T, 0, len(found))
	for _, e := range found {
		typed = append(typed, e.(*T))
	}

	return typed, nil
}

// entitiesOf is the kind scan behind FindByType, written over a runtime type
// so every caller, typed or reflective, matches entities by the same rule.
// Each result is a pointer to want, and nothing matching gives an empty,
// non-nil result.
func (c *Config) entitiesOf(want reflect.Type, path ...string) ([]any, error) {
	if err := c.addressableType(want, path...); err != nil {
		return nil, err
	}

	stored, err := c.entitiesByType(path)
	if err != nil {
		return nil, err
	}

	found := make([]any, 0, len(stored))

	for _, e := range stored {
		typed, err := asType(e, want)
		if err != nil {
			return nil, err
		}

		found = append(found, typed)
	}

	return found, nil
}

// entitiesByType is the type and subtype scan shared by the typed and the
// untyped lookups: the same segment rules, the same errors, and each entity
// returned as the Config stores it. Nothing matching gives an empty, non-nil
// result.
func (c *Config) entitiesByType(path []string) ([]any, error) {
	if len(path) == 0 {
		return nil, &xclerrors.NotTypeableError{Segments: path, Use: "All"}
	}

	if err := c.typeable(path); err != nil {
		return nil, err
	}

	found := []any{}

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

		found = append(found, e)
	}

	return found, nil
}

func findOne[T any](c *Config, path ...string) (*T, error) {
	found, err := c.entitiesOf(reflect.TypeFor[T](), path...)
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

	return one.(*T), nil
}

// oneOf picks the single entity out of a scan's results, reporting whether
// there was one at all. More than one is an error naming the query and how
// many matched, the same error whoever asked.
func oneOf(found []any, path []string) (any, bool, error) {
	switch len(found) {
	case 0:
		return nil, false, nil
	case 1:
		return found[0], true, nil
	default:
		return nil, false, &xclerrors.NotUniqueError{Segments: path, Count: len(found)}
	}
}

// typeable reports whether the segments given pin exactly one Go type, which
// is what makes a result typeable at all.
//
// Segment one names a type: one of the builtin keywords, "resource", or a type
// registered under its own keyword. A type that takes a subtype spans every
// subtype declared under it, so it needs a second segment to pin a Go type.
func (c *Config) typeable(path []string) error {
	entityType := path[0]

	switch entityType {
	case resources.TypeVariable, resources.TypeOutput, resources.TypeModule, resources.TypeRoot:
		// each is exactly one Go type
		return nil
	}

	if c.catalog == nil {
		return &xclerrors.UnknownTypeError{Segments: path, Name: entityType}
	}

	takes, known := c.catalog.TakesSubtype(entityType)
	if !known {
		return &xclerrors.UnknownTypeError{Segments: path, Name: entityType}
	}

	if !takes {
		return nil
	}

	if len(path) < 2 {
		return &xclerrors.NotTypeableError{Segments: path, Use: fmt.Sprintf("FindByType with the subtype, i.e. (%q, \"<subtype>\")", entityType)}
	}

	// the subtype must be one that can actually be declared under the type,
	// naming any other asks a question no configuration could answer
	if !c.catalog.KnownType(entityType, path[1]) {
		return &xclerrors.UnknownTypeError{Segments: path, Name: path[1]}
	}

	return nil
}

// addressableType refuses a query for a type that has no address of its own,
// because it is a block nested inside another declaration rather than a
// declaration in its own right. Answering such a query with an empty result
// would read as "you declared none of those", which is not what is true.
//
// A type embedding types.ResourceBase is an entity type wherever it is used.
// A plain struct is one only when it is registered as a block type, or when
// path names a type a plugin provides, whose plain struct this process may
// never have registered: the same struct may just as well be a nested block
// of another.
func (c *Config) addressableType(want reflect.Type, path ...string) error {
	if _, err := types.GetMeta(reflect.New(want).Interface()); err == nil {
		return nil
	}

	if c.catalog != nil {
		if _, registered := c.catalog.TypePath(want); registered {
			return nil
		}

		if len(path) > 0 {
			subtype := ""
			if len(path) > 1 {
				subtype = path[1]
			}

			// a plugin's type is known with no Go type behind it
			if info, known := c.catalog.Type(path[0], subtype); known && info.Prototype == nil {
				return nil
			}
		}
	}

	return &xclerrors.NotAnEntityError{
		Type:           want,
		ReachedThrough: "declaration",
	}
}

// All returns every entity of the Go type T, without naming it as a string.
//
// The addressing is worked out from how T was registered, so All[Container]
// returns what FindByType[Container]("resource", "container") returns for a
// type registered as ("resource", "container"), and what
// FindByType[Container]("container") returns for one registered as
// ("container") alone.
//
// A plugin provides a schema and no Go type, so a plugin backed entity cannot
// be matched this way. Asking for one returns an error wrapping
// ErrNotRegistered, naming the kind lookup as the form that does work.
func All[T any](c *Config) ([]*T, error) {
	return all[T](c)
}

func all[T any](c *Config) ([]*T, error) {
	path, err := c.typePath(reflect.TypeFor[T]())
	if err != nil {
		return nil, err
	}

	// deliberately goes through the kind lookup rather than scanning again, so
	// the two cannot disagree about what a type matches
	return findByType[T](c, path...)
}

// typePath works out the address segments a Go type is declared under from
// how it was registered, refusing a type with no address of its own and one
// the registry cannot reach, such as a plugin provided type.
func (c *Config) typePath(want reflect.Type) ([]string, error) {
	// with no path to go by, a plain struct nothing registered may be a
	// plugin's type as much as a nested block, so both are named
	if err := c.addressableType(want); err != nil {
		var notEntity *xclerrors.NotAnEntityError
		if errors.As(err, &notEntity) {
			return nil, &xclerrors.NotRegisteredError{Type: want, Use: "FindByType with the type and subtype", MayBeNested: true}
		}

		return nil, err
	}

	if c.catalog == nil {
		return nil, &xclerrors.NotRegisteredError{Type: want, Use: "FindByType with the type and subtype"}
	}

	path, ok := c.catalog.TypePath(want)
	if !ok {
		return nil, &xclerrors.NotRegisteredError{Type: want, Use: "FindByType with the type and subtype"}
	}

	return path, nil
}
