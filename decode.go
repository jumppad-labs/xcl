package xcl

import (
	"reflect"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// Decode fills target from the entities the configuration declares, in one
// call, so an application can gather its configuration into a struct of its
// own rather than asking for each type separately.
//
// target must be a non-nil pointer to a struct. Each of its exported fields is
// matched by its type alone; no tags are read and field names play no part:
//
//   - A []*T field, where T is a registered type, receives every entity of T,
//     exactly as All returns them: in declaration order, disabled entities
//     included, and as the configuration's own instances rather than copies.
//     None declared gives an empty, non-nil slice.
//   - A *T field, where T is a registered type, receives the one entity of T.
//     None declared sets it to nil. More than one makes the call fail with the
//     error FindOne returns for the same type, matching ErrNotUnique.
//   - Every other field, including a []*T or *T whose T the registry cannot
//     reach, such as an unregistered or plugin provided type, keeps its value.
//     Nested structs are not entered.
//
// A target that is not a non-nil pointer to a struct returns an error wrapping
// ErrInvalidDecodeTarget. On any error no field is assigned, so target is
// never left partly filled. Before Apply the call succeeds, leaving every
// slice empty and every pointer nil. c must not be nil.
//
// Decode is not generic, so unlike the generic lookups its method form is
// available on every supported Go version.
func Decode(c *Config, target any) error {
	return decode(c, target)
}

// Decode fills target from the entities the configuration declares. See the
// package level Decode for the full contract.
func (c *Config) Decode(target any) error {
	return decode(c, target)
}

// pendingField is a field resolved by decode and the value it will be set to
// once every field has resolved.
type pendingField struct {
	field reflect.Value
	value reflect.Value
}

// decode is the implementation behind both spellings of Decode. It never scans
// the configuration or converts an entity itself: each field goes through the
// same type path and kind scan as All, so the two cannot disagree.
//
// Every field is resolved before any is assigned, which is what keeps a failed
// call from leaving the target half filled.
func decode(c *Config, target any) error {
	v := reflect.ValueOf(target)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return &xclerrors.InvalidDecodeTargetError{
			Type: reflect.TypeOf(target),
			Nil:  v.IsValid() && v.Kind() == reflect.Pointer && v.IsNil(),
		}
	}

	s := v.Elem()
	pending := []pendingField{}

	for i := range s.NumField() {
		if !s.Type().Field(i).IsExported() {
			continue
		}

		field := s.Field(i)
		if !field.CanSet() {
			continue
		}

		fieldType := field.Type()

		switch {
		case isCollectionField(fieldType):
			want := fieldType.Elem().Elem()

			// a type the registry cannot reach is not configuration, it is
			// the application's own and is left alone
			path, err := c.typePath(want)
			if err != nil {
				continue
			}

			found, err := c.entitiesOf(want, path...)
			if err != nil {
				return err
			}

			collection := reflect.MakeSlice(fieldType, 0, len(found))
			for _, e := range found {
				collection = reflect.Append(collection, reflect.ValueOf(e))
			}

			pending = append(pending, pendingField{field: field, value: collection})

		case isSingleField(fieldType):
			want := fieldType.Elem()

			path, err := c.typePath(want)
			if err != nil {
				continue
			}

			found, err := c.entitiesOf(want, path...)
			if err != nil {
				return err
			}

			one, ok, err := oneOf(found, path)
			if err != nil {
				return err
			}

			value := reflect.Zero(fieldType)
			if ok {
				value = reflect.ValueOf(one)
			}

			pending = append(pending, pendingField{field: field, value: value})
		}
	}

	for _, p := range pending {
		p.field.Set(p.value)
	}

	return nil
}

// isCollectionField reports whether a field has the []*T shape, T a struct.
func isCollectionField(t reflect.Type) bool {
	return t.Kind() == reflect.Slice &&
		t.Elem().Kind() == reflect.Pointer &&
		t.Elem().Elem().Kind() == reflect.Struct
}

// isSingleField reports whether a field has the *T shape, T a struct.
func isSingleField(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct
}
