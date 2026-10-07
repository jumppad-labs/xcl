package diff

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
)

// formatValue writes a change's value on one line as a configuration
// literal: a quoted string with configuration escapes, a number in its
// shortest form, true or false, null for nil, a list as [a, b] and a map as
// { key = value } with its keys sorted. A value of any other kind is written
// as a quoted string of its default format.
func formatValue(value any) string {
	if number, ok := value.(json.Number); ok {
		return number.String()
	}

	return formatReflected(reflect.ValueOf(value))
}

// formatReflected writes value as formatValue does
func formatReflected(value reflect.Value) string {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return "null"
		}
		value = value.Elem()
	}

	if !value.IsValid() {
		return "null"
	}

	if value.CanInterface() {
		if number, ok := value.Interface().(json.Number); ok {
			return number.String()
		}
	}

	switch value.Kind() {
	case reflect.String:
		return formatString(value.String())

	case reflect.Bool:
		return strconv.FormatBool(value.Bool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(value.Uint(), 10)

	case reflect.Float32:
		return strconv.FormatFloat(value.Float(), 'f', -1, 32)

	case reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64)

	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return "null"
		}

		elements := make([]string, value.Len())
		for i := range elements {
			elements[i] = formatReflected(value.Index(i))
		}

		return "[" + strings.Join(elements, ", ") + "]"

	case reflect.Map:
		if value.IsNil() {
			return "null"
		}

		return formatMap(value)
	}

	if value.CanInterface() {
		return formatString(fmt.Sprint(value.Interface()))
	}

	return formatString(value.String())
}

// formatMap writes a map as { key = value }, its keys sorted, written bare
// when they are identifiers and quoted otherwise
func formatMap(value reflect.Value) string {
	if value.Len() == 0 {
		return "{}"
	}

	type entry struct {
		key   string
		value reflect.Value
	}

	entries := make([]entry, 0, value.Len())
	iter := value.MapRange()
	for iter.Next() {
		entries = append(entries, entry{key: mapKey(iter.Key()), value: iter.Value()})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	pairs := make([]string, len(entries))
	for i, e := range entries {
		key := e.key
		if !hclsyntax.ValidIdentifier(key) {
			key = formatString(key)
		}

		pairs[i] = key + " = " + formatReflected(e.value)
	}

	return "{ " + strings.Join(pairs, ", ") + " }"
}

// mapKey returns the string form of a map key
func mapKey(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return key.String()
	}

	return fmt.Sprint(key.Interface())
}

// formatString writes text as a quoted configuration string, escaping
// quotes, control characters and template sequences such as ${
func formatString(text string) string {
	return string(hclwrite.TokensForValue(cty.StringVal(text)).Bytes())
}
