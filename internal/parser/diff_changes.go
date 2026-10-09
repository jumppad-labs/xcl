package parser

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/cty"
	ctyjson "github.com/jumppad-labs/xcl/internal/cty/json"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/types"
)

// resourceChanges returns the changes to a resource's configured values that
// an apply taking action would make, one per changed field in field
// declaration order. saved is the copy the last apply saved and configured is
// the resource as configured, body is the configuration block it was decoded
// from and unknown are the paths of configured values only known once an
// apply has run.
//
//   - create lists each configured top-level field with only its new value,
//     a field is configured when the body sets it or it holds a value after
//     decoding, such as a default
//   - update and replace compare the saved copy with the configured copy:
//     leaves by value, nested blocks by descending, lists by position and
//     maps by key, an added element or key is one entry with only its new
//     value and a removed one has only its old value
//   - delete lists nothing
//
// Computed fields are never compared, they are owned by the provider. A
// change to a sensitive value is marked sensitive and carries neither value
// unless reveal is set. A value at an unknown path is marked unknown and has
// no new value. An added or created whole value that holds a sensitive or
// unknown value is split into its parts until that value stands alone, so
// every other value stays concrete and no secret is hidden inside a whole
// value. Values are plain Go values keyed by configuration names.
func resourceChanges(action diff.Action, saved, configured any, body *hclsyntax.Body, unknown []diff.Path, reveal bool) []diff.Change {
	c := &changeCollector{unknown: unknown, reveal: reveal}

	switch action {
	case diff.ActionCreate:
		c.created(reflect.ValueOf(configured), body)
	case diff.ActionUpdate, diff.ActionReplace:
		c.compare(diff.Path{}, reflect.ValueOf(saved), reflect.ValueOf(configured))
	}

	return c.changes
}

// changeCollector gathers the changes of one resource
type changeCollector struct {
	unknown []diff.Path
	reveal  bool
	changes []diff.Change
}

// created lists the configured top-level fields of a resource being created
func (c *changeCollector) created(configured reflect.Value, body *hclsyntax.Body) {
	configured = indirect(configured)
	if !configured.IsValid() || configured.Kind() != reflect.Struct {
		return
	}

	for _, f := range structFields(configured.Type()) {
		if isComputed(f.field) {
			continue
		}

		path := diff.Path{}.Attribute(f.name)
		value := configured.FieldByIndex(f.index)

		if !c.unknownAt(path) && !c.unknownUnder(path) && !setInBody(body, f.name) && value.IsZero() {
			continue
		}

		c.whole(path, value, true)
	}
}

// compare records the differences between the saved and configured values at
// path
func (c *changeCollector) compare(path diff.Path, saved, configured reflect.Value) {
	if c.unknownAt(path) {
		c.unknownChange(path, saved)
		return
	}

	saved = indirect(saved)
	configured = indirect(configured)

	switch {
	case !saved.IsValid() && !configured.IsValid():
		return
	case !saved.IsValid():
		c.whole(path, configured, true)
		return
	case !configured.IsValid():
		c.whole(path, saved, false)
		return
	}

	switch {
	case isSensitiveType(configured.Type()):
		c.compareSensitive(path, saved, configured)

	case configured.Type() == ctyValueType:
		c.compareCty(path, saved, configured)

	case configured.Kind() == reflect.Struct:
		for _, f := range structFields(configured.Type()) {
			if isComputed(f.field) {
				continue
			}

			c.compare(path.Attribute(f.name), saved.FieldByIndex(f.index), configured.FieldByIndex(f.index))
		}

	case configured.Kind() == reflect.Slice || configured.Kind() == reflect.Array:
		c.compareList(path, saved, configured)

	case configured.Kind() == reflect.Map:
		c.compareMap(path, saved, configured)

	default:
		if !reflect.DeepEqual(saved.Interface(), configured.Interface()) {
			c.changes = append(c.changes, diff.Change{
				Path:   path,
				Before: plainValue(saved, c.reveal),
				After:  plainValue(configured, c.reveal),
			})
		}
	}
}

// compareList compares two lists position by position
func (c *changeCollector) compareList(path diff.Path, saved, configured reflect.Value) {
	common := min(saved.Len(), configured.Len())

	for i := 0; i < common; i++ {
		c.compare(path.Index(i), saved.Index(i), configured.Index(i))
	}

	for i := common; i < configured.Len(); i++ {
		c.whole(path.Index(i), configured.Index(i), true)
	}

	for i := common; i < saved.Len(); i++ {
		c.whole(path.Index(i), saved.Index(i), false)
	}
}

// compareMap compares two maps key by key
func (c *changeCollector) compareMap(path diff.Path, saved, configured reflect.Value) {
	for _, key := range sortedKeys(saved, configured) {
		savedElement := mapIndex(saved, key)
		configuredElement := mapIndex(configured, key)
		elementPath := path.Key(mapKeyString(key))

		switch {
		case !savedElement.IsValid():
			c.whole(elementPath, configuredElement, true)
		case !configuredElement.IsValid():
			c.whole(elementPath, savedElement, false)
		default:
			c.compare(elementPath, savedElement, configuredElement)
		}
	}
}

// compareSensitive compares two sensitive values, a change is recorded
// without either value unless they are revealed
func (c *changeCollector) compareSensitive(path diff.Path, saved, configured reflect.Value) {
	savedValue := saved.Interface().(types.SensitiveValue).RevealAny()
	configuredValue := configured.Interface().(types.SensitiveValue).RevealAny()

	if reflect.DeepEqual(savedValue, configuredValue) {
		return
	}

	change := diff.Change{Path: path, Sensitive: true}
	if c.reveal {
		change.Before = plainValue(saved, true)
		change.After = plainValue(configured, true)
	}

	c.changes = append(c.changes, change)
}

// compareCty compares two cty values as single leaves
func (c *changeCollector) compareCty(path diff.Path, saved, configured reflect.Value) {
	savedValue := saved.Interface().(cty.Value)
	configuredValue := configured.Interface().(cty.Value)

	if savedValue.RawEquals(configuredValue) {
		return
	}

	if containsSensitive(saved) || containsSensitive(configured) {
		change := diff.Change{Path: path, Sensitive: true}
		if c.reveal {
			change.Before = plainValue(saved, true)
			change.After = plainValue(configured, true)
		}

		c.changes = append(c.changes, change)
		return
	}

	c.changes = append(c.changes, diff.Change{
		Path:   path,
		Before: plainValue(saved, c.reveal),
		After:  plainValue(configured, c.reveal),
	})
}

// whole records an added or removed value as a single change, split into its
// parts when it holds a sensitive or unknown value so that value stands alone
func (c *changeCollector) whole(path diff.Path, value reflect.Value, added bool) {
	if added && c.unknownAt(path) {
		c.unknownChange(path, reflect.Value{})
		return
	}

	value = indirect(value)

	split := containsSensitive(value) || (added && c.unknownUnder(path))
	if !split || !value.IsValid() {
		change := diff.Change{Path: path}
		if added {
			change.After = plainValue(value, c.reveal)
		} else {
			change.Before = plainValue(value, c.reveal)
		}

		c.changes = append(c.changes, change)
		return
	}

	switch {
	case isSensitiveType(value.Type()) || value.Type() == ctyValueType:
		change := diff.Change{Path: path, Sensitive: true}
		if c.reveal {
			if added {
				change.After = plainValue(value, true)
			} else {
				change.Before = plainValue(value, true)
			}
		}

		c.changes = append(c.changes, change)

	case value.Kind() == reflect.Struct:
		for _, f := range structFields(value.Type()) {
			if isComputed(f.field) {
				continue
			}

			fieldPath := path.Attribute(f.name)
			field := value.FieldByIndex(f.index)

			// a part with no value is left out, unless it is the one that
			// is unknown
			if field.IsZero() && !(added && (c.unknownAt(fieldPath) || c.unknownUnder(fieldPath))) {
				continue
			}

			c.whole(fieldPath, field, added)
		}

	case value.Kind() == reflect.Slice || value.Kind() == reflect.Array:
		for i := 0; i < value.Len(); i++ {
			c.whole(path.Index(i), value.Index(i), added)
		}

	case value.Kind() == reflect.Map:
		for _, key := range sortedKeys(value, reflect.Value{}) {
			c.whole(path.Key(mapKeyString(key)), value.MapIndex(key), added)
		}
	}
}

// unknownChange records a value at an unknown path, with the saved value
// when there is one
func (c *changeCollector) unknownChange(path diff.Path, saved reflect.Value) {
	change := diff.Change{Path: path, Unknown: true}

	saved = indirect(saved)
	if saved.IsValid() && !saved.IsZero() {
		if containsSensitive(saved) {
			change.Sensitive = true
			if c.reveal {
				change.Before = plainValue(saved, true)
			}
		} else {
			change.Before = plainValue(saved, c.reveal)
		}
	}

	c.changes = append(c.changes, change)
}

// unknownAt returns true when path is an unknown path
func (c *changeCollector) unknownAt(path diff.Path) bool {
	for _, u := range c.unknown {
		if u.Equal(path) {
			return true
		}
	}

	return false
}

// unknownUnder returns true when an unknown path lies inside path
func (c *changeCollector) unknownUnder(path diff.Path) bool {
	for _, u := range c.unknown {
		if len(u) > len(path) && u.Within(path) {
			return true
		}
	}

	return false
}

// setInBody returns true when the body sets the attribute or holds a block
// with the given name
func setInBody(body *hclsyntax.Body, name string) bool {
	if body == nil {
		return false
	}

	if _, ok := body.Attributes[name]; ok {
		return true
	}

	for _, block := range body.Blocks {
		if block.Type == name {
			return true
		}
	}

	return false
}

// indirect follows pointers and interfaces, it returns the zero Value for nil
func indirect(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}

		value = value.Elem()
	}

	return value
}

// containsSensitive returns true when the value is, or holds anywhere inside
// it, a sensitive value: a types.Sensitive leaf or a marked cty value
func containsSensitive(value reflect.Value) bool {
	value = indirect(value)
	if !value.IsValid() {
		return false
	}

	switch {
	case isSensitiveType(value.Type()):
		return true

	case value.Type() == ctyValueType:
		return value.Interface().(cty.Value).ContainsMarked()

	case value.Kind() == reflect.Struct:
		for _, f := range structFields(value.Type()) {
			if !isComputed(f.field) && containsSensitive(value.FieldByIndex(f.index)) {
				return true
			}
		}

	case value.Kind() == reflect.Slice || value.Kind() == reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if containsSensitive(value.Index(i)) {
				return true
			}
		}

	case value.Kind() == reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if containsSensitive(iter.Value()) {
				return true
			}
		}
	}

	return false
}

// plainValue converts a value to plain Go values keyed by configuration
// names: a struct becomes a map of its non-computed fields, a list a []any
// and a map a map[string]any. A sensitive value is converted only when reveal
// is set, otherwise it is nil.
func plainValue(value reflect.Value, reveal bool) any {
	value = indirect(value)
	if !value.IsValid() {
		return nil
	}

	switch {
	case isSensitiveType(value.Type()):
		if !reveal {
			return nil
		}

		return plainValue(reflect.ValueOf(value.Interface().(types.SensitiveValue).RevealAny()), reveal)

	case value.Type() == ctyValueType:
		return plainCtyValue(value.Interface().(cty.Value), reveal)

	case value.Kind() == reflect.Struct:
		fields := map[string]any{}
		for _, f := range structFields(value.Type()) {
			if isComputed(f.field) {
				continue
			}

			fields[f.name] = plainValue(value.FieldByIndex(f.index), reveal)
		}

		return fields

	case value.Kind() == reflect.Slice || value.Kind() == reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return nil
		}

		elements := make([]any, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			elements = append(elements, plainValue(value.Index(i), reveal))
		}

		return elements

	case value.Kind() == reflect.Map:
		if value.IsNil() {
			return nil
		}

		elements := map[string]any{}
		iter := value.MapRange()
		for iter.Next() {
			elements[mapKeyString(iter.Key())] = plainValue(iter.Value(), reveal)
		}

		return elements
	}

	return value.Interface()
}

// plainCtyValue converts a cty value to plain Go values through its JSON
// form. A value holding a sensitive mark is converted only when reveal is
// set, an unknown or unconvertible value is nil.
func plainCtyValue(value cty.Value, reveal bool) any {
	if value.ContainsMarked() {
		if !reveal {
			return nil
		}

		value, _ = value.UnmarkDeep()
	}

	if value.IsNull() || !value.IsWhollyKnown() {
		return nil
	}

	data, err := ctyjson.Marshal(value, value.Type())
	if err != nil {
		return nil
	}

	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		return nil
	}

	return plain
}

// sortedKeys returns the keys of both maps, without repeats, sorted by their
// string form
func sortedKeys(a, b reflect.Value) []reflect.Value {
	seen := map[string]bool{}
	keys := []reflect.Value{}

	for _, m := range []reflect.Value{a, b} {
		if !m.IsValid() || m.IsNil() {
			continue
		}

		for _, key := range m.MapKeys() {
			name := mapKeyString(key)
			if seen[name] {
				continue
			}

			seen[name] = true
			keys = append(keys, key)
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return mapKeyString(keys[i]) < mapKeyString(keys[j])
	})

	return keys
}

// mapIndex returns the element of m at key, or the zero Value when m holds no
// such key
func mapIndex(m reflect.Value, key reflect.Value) reflect.Value {
	if !m.IsValid() || m.IsNil() {
		return reflect.Value{}
	}

	return m.MapIndex(key)
}

// mapKeyString returns the string form of a map key
func mapKeyString(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return key.String()
	}

	return fmt.Sprint(key.Interface())
}
