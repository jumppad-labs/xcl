package xcl

import (
	"reflect"

	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/types"
)

// sensitiveValueType is the interface every types.Sensitive[T] implements,
// which is how the fill recognises a sensitive value of any T
var sensitiveValueType = reflect.TypeFor[types.SensitiveValue]()

// bookkeepingTypes are the types of the fields xcl keeps about an entity
// rather than values a person configured, which the fill never walks
var bookkeepingTypes = []reflect.Type{
	reflect.TypeFor[types.ResourceBase](),
	reflect.TypeFor[types.Meta](),
}

// EncodeEntityRecord returns the configuration text for an entity's JSON
// record, in the shape a lookup returns it, as encoding/json writes an entity
// such as one returned by Find, FindByType or Entities, and possibly edited
// since. The text is identical to what EncodeEntity returns for the same
// entity with the same options.
//
// A lookup's record holds types.SensitiveMarker in place of every sensitive
// value, set or not, and leaves out every list or map that is empty. So what
// the record lost is taken from this Config's own copy of the entity its
// meta.id names, and the text is what EncodeEntity writes for that entity:
// a sensitive value still holding the marker is written as the marker when
// it is set, as its configured value with RevealSensitive, and not at all
// when it was never set, and an empty list or map the record leaves out is
// written as the entity holds it. A sensitive value the record holds for
// real, such as one edited since the lookup or set in a record built by
// hand, is shown as it is, as the marker unless RevealSensitive is given. A
// record whose meta.id is missing or names no entity of this Config is not
// filled, and its marker values stay the marker.
//
// The record's type is resolved through the Config's own types and plugins,
// and plugins are loaded if no operation has yet, as for EncodeSavedEntity.
//
// It fails with ErrUnregisteredType when the Config does not know the type
// the record names, with ErrInvalidSavedData when the data is not one entity
// record, with ErrPluginLoad when a plugin fails to load, and with
// ErrNotEncodable on the same terms as EncodeEntity. A failure returns no
// text.
func (c *Config) EncodeEntityRecord(data []byte, options ...EncodeOption) ([]byte, error) {
	opts := encodeOptions{}
	for _, option := range options {
		option(&opts)
	}

	// a plugin's types are resolvable only after the plugins have loaded,
	// which is once only and cached, as for EncodeSavedEntity
	done, err := c.catalog.Use(nil)
	if err != nil {
		return nil, err
	}
	defer done()

	entity, err := savedentity.Decode(c.catalog, data, savedentity.ReadOptions{ForDisplay: true})
	if err != nil {
		return nil, err
	}

	// a lookup hands out only the marker, whether or not a value is set, so
	// what the value is can come only from the entity this Config holds. The
	// encoder still writes the marker for it unless reveal is asked for
	fillRedacted(entity, c.storedFor(entity))

	return encodeEntity(entity, opts)
}

// storedFor returns this Config's own copy of the entity a record names by
// its meta.id, or nil when the record has no id or the id names nothing
func (c *Config) storedFor(entity any) any {
	meta, err := types.GetMeta(entity)
	if err != nil || meta.ID == "" {
		return nil
	}

	stored, err := c.FindResource(meta.ID)
	if err != nil {
		return nil
	}

	return stored
}

// fillRedacted copies into decoded what a lookup's JSON loses of stored:
// each sensitive value decoded holds only as the marker, and each empty list
// or map, which encoding/json leaves out of the JSON so decoded holds none.
// The two are walked side by side, and wherever their types differ, or
// stored has nothing at that place, decoded is left as it is: the fill never
// adds elements, never replaces a value that is not redacted, and never
// replaces a list or map decoded holds.
func fillRedacted(decoded, stored any) {
	if decoded == nil || stored == nil {
		return
	}

	fillValue(reflect.ValueOf(decoded), reflect.ValueOf(stored))
}

// fillValue is one step of fillRedacted's walk
func fillValue(dst, src reflect.Value) {
	if !dst.IsValid() || !src.IsValid() || dst.Type() != src.Type() {
		return
	}

	if dst.Type().Implements(sensitiveValueType) {
		if !dst.CanSet() || !dst.CanInterface() || !src.CanInterface() {
			return
		}

		if types.IsRedacted(dst.Interface().(types.SensitiveValue)) && !types.IsRedacted(src.Interface().(types.SensitiveValue)) {
			dst.Set(src)
		}

		return
	}

	switch dst.Kind() {
	case reflect.Pointer:
		if dst.IsNil() || src.IsNil() {
			return
		}

		fillValue(dst.Elem(), src.Elem())

	case reflect.Interface:
		if dst.IsNil() || src.IsNil() || !dst.CanSet() {
			return
		}

		// an interface's element cannot be set in place, so fill a copy and
		// put it back
		element := reflect.New(dst.Elem().Type()).Elem()
		element.Set(dst.Elem())
		fillValue(element, src.Elem())
		dst.Set(element)

	case reflect.Struct:
		for i := range dst.NumField() {
			field := dst.Type().Field(i)
			if !field.IsExported() || isBookkeeping(field.Type) {
				continue
			}

			fillValue(dst.Field(i), src.Field(i))
		}

	case reflect.Slice, reflect.Array:
		if dst.Kind() == reflect.Slice && dst.IsNil() && !src.IsNil() && src.Len() == 0 && dst.CanSet() {
			dst.Set(reflect.MakeSlice(dst.Type(), 0, 0))
			return
		}

		for i := range min(dst.Len(), src.Len()) {
			fillValue(dst.Index(i), src.Index(i))
		}

	case reflect.Map:
		if dst.IsNil() && !src.IsNil() && src.Len() == 0 && dst.CanSet() {
			dst.Set(reflect.MakeMap(dst.Type()))
			return
		}

		if dst.IsNil() || src.IsNil() {
			return
		}

		// a map's element cannot be set in place either
		for _, key := range dst.MapKeys() {
			stored := src.MapIndex(key)
			if !stored.IsValid() {
				continue
			}

			element := reflect.New(dst.Type().Elem()).Elem()
			element.Set(dst.MapIndex(key))
			fillValue(element, stored)
			dst.SetMapIndex(key, element)
		}
	}
}

// isBookkeeping reports whether a field holds what xcl records about an
// entity rather than a configured value
func isBookkeeping(fieldType reflect.Type) bool {
	for _, bookkeeping := range bookkeepingTypes {
		if fieldType == bookkeeping {
			return true
		}
	}

	return false
}
