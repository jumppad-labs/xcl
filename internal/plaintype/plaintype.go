// Package plaintype lets a plain Go struct be a block type.
//
// The engine reaches an entity's name, dependencies and disabled flag through
// an embedded types.ResourceBase. A plain struct has none, so Build makes the
// type the engine stores instead: types.ResourceBase first, as a hand-written
// type embeds it, then the plain struct's fields, then a marker naming the
// plain type. Everything that handles entities sees an ordinary block type,
// as it does for a type built from a plugin's schema. ToPlain turns a stored
// entity back into the plain struct for the application.
//
// A plain struct's fields are copied as they are, tags included, except:
//
//   - unexported fields are left out, since nothing could decode into them;
//   - an embedded struct tagged `xcl:",remain"` has its fields lifted, so a
//     struct composed of others is laid out as it reads;
//   - fields tagged as labels are filled from the block's labels, in order,
//     rather than decoded as attributes;
//   - fields tagged `disabled` or `depends_on` mirror the values every block
//     has, filled from types.ResourceBase rather than decoded twice.
package plaintype

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/jumppad-labs/xcl/internal/xcl/tags"
	"github.com/jumppad-labs/xcl/types"
)

// MarkerField is the name of the field that names the plain type in a built
// one. It is a zero length array of the plain type: it takes no space, has no
// xcl tag so configuration never sees it, and is left out of JSON.
//
// It also keeps built types apart. reflect.StructOf returns the same type for
// the same fields, so two plain types with identical fields would otherwise
// be built into one engine type, and an entity of either would convert into
// both.
const MarkerField = "XclPlain"

// resourceBaseField is the types.ResourceBase every built type starts with,
// tagged as a hand-written type tags it.
var resourceBaseField = reflect.StructField{
	Name:      "ResourceBase",
	Type:      reflect.TypeOf(types.ResourceBase{}),
	Anonymous: true,
	Tag:       `xcl:",remain"`,
}

// The configuration names types.ResourceBase gives every block.
const (
	disabledName  = "disabled"
	dependsOnName = "depends_on"
	metaName      = "meta"
)

// Built is a plain type and the type the engine stores in its place.
type Built struct {
	// Plain is the application's struct type
	Plain reflect.Type

	// Engine is the struct type entities of the plain type are stored as
	Engine reflect.Type

	// copied maps each field of Engine that came from Plain to its index
	// path in Plain
	copied []copiedField

	// labels are the index paths of Plain's label fields, in order
	labels [][]int

	// disabled and dependsOn are the index paths of Plain's mirror fields,
	// nil when it has none
	disabled  []int
	dependsOn []int
}

type copiedField struct {
	engine int
	plain  []int
}

// Build makes the engine type for plain, a struct type that does not embed
// types.ResourceBase. labels is how many labels a block of the type has: one
// for a type declared without a subtype, two for one declared with one.
//
// The error names the field at fault: a reserved name, a label count that does
// not match, a mirror field of the wrong type, an embedded field that is not a
// remain struct, or a second field with the same Go name once embedded fields
// are lifted.
func Build(plain reflect.Type, labels int) (*Built, error) {
	if plain.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", plain)
	}

	b := &Built{Plain: plain}

	fields := []reflect.StructField{resourceBaseField}
	seen := map[string]bool{}

	var collect func(t reflect.Type, path []int) error
	collect = func(t reflect.Type, path []int) error {
		for i := range t.NumField() {
			field := t.Field(i)
			index := append(append([]int{}, path...), i)

			tag, tagged := field.Tag.Lookup(tags.Name)
			var parsed tags.Field
			if tagged && tag != "" {
				var err error
				parsed, err = tags.Parse(tag)
				if err != nil {
					return fmt.Errorf("field %s of %s: %w", field.Name, plain, err)
				}
			}

			if field.Anonymous {
				if parsed.Kind != tags.KindRemain || parsed.Name != "" {
					return fmt.Errorf("field %s of %s: an embedded field must be a struct tagged `xcl:\",remain\"`", field.Name, plain)
				}

				if field.Type.Kind() != reflect.Struct {
					return fmt.Errorf("field %s of %s: an embedded field must be a struct, not %s", field.Name, plain, field.Type)
				}

				if err := collect(field.Type, index); err != nil {
					return err
				}

				continue
			}

			if !field.IsExported() {
				continue
			}

			if field.Name == resourceBaseField.Name || field.Name == MarkerField {
				return fmt.Errorf("field %s of %s: the Go name %s is used by xcl", field.Name, plain, field.Name)
			}

			if tagged && tag != "" {
				switch {
				case parsed.Kind == tags.KindLabel:
					if field.Type.Kind() != reflect.String {
						return fmt.Errorf("field %s of %s: a label is a string, not %s", field.Name, plain, field.Type)
					}

					b.labels = append(b.labels, index)
					continue

				case parsed.Kind == tags.KindRemain || parsed.Kind == tags.KindBody:
					return fmt.Errorf("field %s of %s: a block type's remain and body are xcl's, a plain type cannot take them", field.Name, plain)

				case parsed.Name == metaName:
					return fmt.Errorf("field %s of %s: %q is held by every block and cannot be declared", field.Name, plain, metaName)

				case parsed.Name == disabledName:
					if field.Type.Kind() != reflect.Bool {
						return fmt.Errorf("field %s of %s: %q mirrors every block's disabled flag and must be a bool, not %s", field.Name, plain, disabledName, field.Type)
					}

					b.disabled = index
					continue

				case parsed.Name == dependsOnName:
					if field.Type != reflect.TypeOf([]string{}) {
						return fmt.Errorf("field %s of %s: %q mirrors every block's dependencies and must be a []string, not %s", field.Name, plain, dependsOnName, field.Type)
					}

					b.dependsOn = index
					continue
				}
			}

			if seen[field.Name] {
				return fmt.Errorf("field %s of %s: another field has the same Go name", field.Name, plain)
			}
			seen[field.Name] = true

			b.copied = append(b.copied, copiedField{engine: len(fields), plain: index})
			fields = append(fields, reflect.StructField{
				Name: field.Name,
				Type: field.Type,
				Tag:  field.Tag,
			})
		}

		return nil
	}

	if err := collect(plain, nil); err != nil {
		return nil, err
	}

	if len(b.labels) != 0 && len(b.labels) != labels {
		return nil, fmt.Errorf("%s has %d label fields, a block of its type has %d labels", plain, len(b.labels), labels)
	}

	fields = append(fields, reflect.StructField{
		Name: MarkerField,
		Type: reflect.ArrayOf(0, plain),
		Tag:  `json:"-"`,
	})

	b.Engine = reflect.StructOf(fields)
	built.Store(b.Engine, b)
	plains.Store(plain, true)

	return b, nil
}

// plains holds every plain type Build has built, so a value of one can be
// recognised for what it is where the engine type is not to hand.
var plains sync.Map

// IsBuilt reports whether plain, or the type a pointer to it points at, has
// been built as a plain block type: a value of it is a copy of an entity,
// not the entity.
func IsBuilt(plain reflect.Type) bool {
	for plain.Kind() == reflect.Pointer {
		plain = plain.Elem()
	}

	_, ok := plains.Load(plain)
	return ok
}

// built holds every Built by its engine type, so an entity can be turned back
// into its plain type where nothing else knows how it was built, such as a
// query given an entity and a Go type and no configuration. Building is
// deterministic, so the same plain type always yields an equivalent Built and
// storing it again is harmless.
var built sync.Map

// Lookup returns how engine, or the type a pointer to it points at, was
// built, and false for a type Build did not make.
func Lookup(engine reflect.Type) (*Built, bool) {
	for engine.Kind() == reflect.Pointer {
		engine = engine.Elem()
	}

	b, ok := built.Load(engine)
	if !ok {
		return nil, false
	}

	return b.(*Built), true
}

// New returns a pointer to a new zero value of the engine type.
func (b *Built) New() any {
	return reflect.New(b.Engine).Interface()
}

// PlainOf returns the plain type an engine type was built from, and false for
// a type that was not built by Build.
func PlainOf(engine reflect.Type) (reflect.Type, bool) {
	for engine.Kind() == reflect.Pointer {
		engine = engine.Elem()
	}

	if engine.Kind() != reflect.Struct {
		return nil, false
	}

	marker, ok := engine.FieldByName(MarkerField)
	if !ok || marker.Type.Kind() != reflect.Array || marker.Type.Len() != 0 {
		return nil, false
	}

	return marker.Type.Elem(), true
}

// ToPlain copies an entity of the engine type into a new value of the plain
// type and returns a pointer to it. Labels are filled from the entity's
// metadata, and the mirror fields from its types.ResourceBase.
//
// The result is a copy: changing it does not change the stored entity.
func (b *Built) ToPlain(entity any) (any, error) {
	source := reflect.ValueOf(entity)
	if source.Kind() == reflect.Pointer {
		source = source.Elem()
	}

	if source.Type() != b.Engine {
		return nil, fmt.Errorf("%s is not an entity of %s", source.Type(), b.Plain)
	}

	target := reflect.New(b.Plain).Elem()

	for _, f := range b.copied {
		target.FieldByIndex(f.plain).Set(source.Field(f.engine))
	}

	base := source.Field(0).Interface().(types.ResourceBase)

	if len(b.labels) == 2 {
		target.FieldByIndex(b.labels[0]).SetString(base.Meta.Subtype)
		target.FieldByIndex(b.labels[1]).SetString(base.Meta.Name)
	} else if len(b.labels) == 1 {
		target.FieldByIndex(b.labels[0]).SetString(base.Meta.Name)
	}

	if b.disabled != nil {
		target.FieldByIndex(b.disabled).SetBool(base.Disabled)
	}

	if b.dependsOn != nil && base.DependsOn != nil {
		target.FieldByIndex(b.dependsOn).Set(reflect.ValueOf(append([]string{}, base.DependsOn...)))
	}

	return target.Addr().Interface(), nil
}
