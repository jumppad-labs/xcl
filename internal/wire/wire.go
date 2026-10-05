// Package wire serialises values as JSON with the real value of every
// types.Sensitive they hold.
//
// Sensitive values marshal to types.SensitiveMarker through encoding/json, so
// anything shown to someone, such as event data or an application's own
// json.Marshal, never carries a secret. The hops that must keep the truth use
// this package instead, and are the only allowed callers:
//
//   - state save (internal/parser state helper),
//   - provider calls in both directions (internal/parser lifecycle and
//     callbacks, plugins adapter, plugins testing helpers),
//   - change detection and the configured-value check (plugins),
//   - the host state callback (plugins),
//   - query conversion (internal/schema),
//   - the resource printer when revealing is asked for (logger).
//
// A caller may also pass a masker through Encode, which then writes each
// sensitive value as that masker's envelope instead of its real value. The
// masker is chosen per call, never through shared state:
//
//   - provider calls, change detection and conversions pass no masker, so
//     they keep real values,
//   - the state save passes the configured state masker,
//   - event data passes the configured event masker.
//
// Apart from writing sensitive values revealed, the output follows
// encoding/json exactly: tag names, omitempty, omitzero, "-", the string
// option, embedded struct flattening with Go's dominance rules, and existing
// json.Marshaler and encoding.TextMarshaler implementations. A value whose
// type cannot hold a sensitive value is handed to encoding/json whole, so its
// bytes are identical.
package wire

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/types"
)

var (
	sensitiveValueType = reflect.TypeOf((*types.SensitiveValue)(nil)).Elem()
	jsonMarshalerType  = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshalerType  = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// Options choose how Encode writes sensitive values.
type Options struct {
	// Mask, when set, writes each sensitive value as this masker's envelope.
	// Nil writes real values.
	Mask mask.Masker
}

// Result is what Encode wrote.
type Result struct {
	// Data is the JSON encoding.
	Data []byte

	// Sensitive reports whether at least one sensitive value was written,
	// masked or not.
	Sensitive bool
}

// Encode returns the JSON encoding of value, with every sensitive value
// written as its real value, or as the options' masker's envelope when one is
// given. It also reports whether any sensitive value was written.
func Encode(value any, options Options) (Result, error) {
	buffer := &bytes.Buffer{}
	state := &encoder{options: options}
	if err := state.encode(buffer, reflect.ValueOf(value)); err != nil {
		return Result{}, err
	}

	return Result{Data: buffer.Bytes(), Sensitive: state.sensitive}, nil
}

// Marshal returns the JSON encoding of value, with every sensitive value
// written as its real value. It is Encode with no masker.
func Marshal(value any) ([]byte, error) {
	result, err := Encode(value, Options{})
	if err != nil {
		return nil, err
	}

	return result.Data, nil
}

// encoder carries one Encode call's options, and what it has seen, through
// the recursive walk.
type encoder struct {
	options   Options
	sensitive bool
}

// MarshalIndent is like Marshal but applies json.Indent to the output, as
// json.MarshalIndent does.
func MarshalIndent(value any, prefix, indent string) ([]byte, error) {
	compact, err := Marshal(value)
	if err != nil {
		return nil, err
	}

	indented := &bytes.Buffer{}
	if err := json.Indent(indented, compact, prefix, indent); err != nil {
		return nil, err
	}

	return indented.Bytes(), nil
}

func (e *encoder) encode(buffer *bytes.Buffer, value reflect.Value) error {
	if !value.IsValid() {
		buffer.WriteString("null")
		return nil
	}

	valueType := value.Type()

	if valueType.Kind() != reflect.Interface && valueType.Implements(sensitiveValueType) {
		return e.encodeSensitive(buffer, value)
	}

	if !mayHoldSensitive(valueType) || isMarshaler(value) {
		return delegate(buffer, value)
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			buffer.WriteString("null")
			return nil
		}

		return e.encode(buffer, value.Elem())

	case reflect.Struct:
		return e.encodeStruct(buffer, value)

	case reflect.Map:
		return e.encodeMap(buffer, value)

	case reflect.Slice:
		if value.IsNil() {
			buffer.WriteString("null")
			return nil
		}

		return e.encodeArray(buffer, value)

	case reflect.Array:
		return e.encodeArray(buffer, value)
	}

	return delegate(buffer, value)
}

// encodeSensitive writes the real value of a sensitive value, or its
// envelope when a masker is given. A value already redacted is written as the
// marker, or the marker masked.
func (e *encoder) encodeSensitive(buffer *bytes.Buffer, value reflect.Value) error {
	e.sensitive = true

	sensitive := value.Interface().(types.SensitiveValue)

	if e.options.Mask == nil {
		if types.IsRedacted(sensitive) {
			return delegate(buffer, value)
		}

		return e.encode(buffer, reflect.ValueOf(sensitive.RevealAny()))
	}

	plain := &bytes.Buffer{}
	if types.IsRedacted(sensitive) {
		if err := delegate(plain, value); err != nil {
			return err
		}
	} else {
		// the real value is what the masker receives, so it is written with
		// no masker of its own
		inner := &encoder{}
		if err := inner.encode(plain, reflect.ValueOf(sensitive.RevealAny())); err != nil {
			return err
		}
	}

	envelope, err := mask.Envelope(plain.Bytes(), e.options.Mask)
	if err != nil {
		return err
	}

	buffer.Write(envelope)

	return nil
}

// delegate writes value with encoding/json. An addressable value is passed
// by pointer so pointer-receiver marshalers run, as they do in encoding/json.
func delegate(buffer *bytes.Buffer, value reflect.Value) error {
	var target any
	if value.CanAddr() {
		target = value.Addr().Interface()
	} else {
		target = value.Interface()
	}

	data, err := json.Marshal(target)
	if err != nil {
		return err
	}

	buffer.Write(data)

	return nil
}

// isMarshaler reports whether encoding/json would hand value to its own
// MarshalJSON or MarshalText.
func isMarshaler(value reflect.Value) bool {
	valueType := value.Type()
	if valueType.Kind() == reflect.Interface {
		return false
	}

	if valueType.Implements(jsonMarshalerType) || valueType.Implements(textMarshalerType) {
		return true
	}

	if valueType.Kind() != reflect.Pointer && value.CanAddr() {
		pointerType := reflect.PointerTo(valueType)
		return pointerType.Implements(jsonMarshalerType) || pointerType.Implements(textMarshalerType)
	}

	return false
}

func (e *encoder) encodeStruct(buffer *bytes.Buffer, value reflect.Value) error {
	buffer.WriteByte('{')

	first := true
	for _, field := range cachedTypeFields(value.Type()) {
		fieldValue, ok := fieldByIndex(value, field.index)
		if !ok {
			continue
		}

		if field.omitEmpty && isEmptyValue(fieldValue) {
			continue
		}

		if field.omitZero && isZeroValue(fieldValue) {
			continue
		}

		if !first {
			buffer.WriteByte(',')
		}
		first = false

		buffer.Write(field.encodedName)
		buffer.WriteByte(':')

		if field.quoted {
			if err := e.encodeQuoted(buffer, fieldValue); err != nil {
				return err
			}

			continue
		}

		if err := e.encode(buffer, fieldValue); err != nil {
			return err
		}
	}

	buffer.WriteByte('}')

	return nil
}

// encodeQuoted writes a field carrying the string option, which encoding/json
// writes as a JSON string holding the JSON encoding of the value.
func (e *encoder) encodeQuoted(buffer *bytes.Buffer, value reflect.Value) error {
	inner := &bytes.Buffer{}
	if err := e.encode(inner, value); err != nil {
		return err
	}

	if inner.String() == "null" {
		buffer.WriteString("null")
		return nil
	}

	quoted, err := json.Marshal(inner.String())
	if err != nil {
		return err
	}

	buffer.Write(quoted)

	return nil
}

// fieldByIndex walks an index sequence, reporting false when it passes
// through a nil embedded pointer.
func fieldByIndex(value reflect.Value, index []int) (reflect.Value, bool) {
	for depth, fieldIndex := range index {
		if depth > 0 && value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return reflect.Value{}, false
			}

			value = value.Elem()
		}

		value = value.Field(fieldIndex)
	}

	return value, true
}

func (e *encoder) encodeMap(buffer *bytes.Buffer, value reflect.Value) error {
	if value.IsNil() {
		buffer.WriteString("null")
		return nil
	}

	type entry struct {
		key   string
		value reflect.Value
	}

	entries := make([]entry, 0, value.Len())
	iterator := value.MapRange()
	for iterator.Next() {
		key, err := mapKey(iterator.Key())
		if err != nil {
			return err
		}

		entries = append(entries, entry{key: key, value: iterator.Value()})
	}

	slices.SortFunc(entries, func(left, right entry) int {
		return strings.Compare(left.key, right.key)
	})

	buffer.WriteByte('{')
	for i, mapEntry := range entries {
		if i > 0 {
			buffer.WriteByte(',')
		}

		encodedKey, err := json.Marshal(mapEntry.key)
		if err != nil {
			return err
		}

		buffer.Write(encodedKey)
		buffer.WriteByte(':')

		if err := e.encode(buffer, mapEntry.value); err != nil {
			return err
		}
	}
	buffer.WriteByte('}')

	return nil
}

// mapKey resolves a map key to the string encoding/json uses for it.
func mapKey(key reflect.Value) (string, error) {
	if key.Kind() == reflect.String {
		return key.String(), nil
	}

	if textMarshaler, ok := key.Interface().(encoding.TextMarshaler); ok {
		if key.Kind() == reflect.Pointer && key.IsNil() {
			return "", nil
		}

		text, err := textMarshaler.MarshalText()
		return string(text), err
	}

	switch key.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprint(key.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fmt.Sprint(key.Uint()), nil
	}

	return "", fmt.Errorf("json: unsupported map key type %s", key.Type())
}

func (e *encoder) encodeArray(buffer *bytes.Buffer, value reflect.Value) error {
	buffer.WriteByte('[')
	for i := 0; i < value.Len(); i++ {
		if i > 0 {
			buffer.WriteByte(',')
		}

		if err := e.encode(buffer, value.Index(i)); err != nil {
			return err
		}
	}
	buffer.WriteByte(']')

	return nil
}

func isEmptyValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Interface, reflect.Pointer:
		return value.IsZero()
	}

	return false
}

func isZeroValue(value reflect.Value) bool {
	if zeroer, ok := value.Interface().(interface{ IsZero() bool }); ok {
		if value.Kind() == reflect.Pointer && value.IsNil() {
			return true
		}

		return zeroer.IsZero()
	}

	if value.CanAddr() {
		if zeroer, ok := value.Addr().Interface().(interface{ IsZero() bool }); ok {
			return zeroer.IsZero()
		}
	}

	return value.IsZero()
}

// mayHoldSensitive reports whether a value of the given type can hold a
// sensitive value. Any interface may, since it can hold anything. Answers are
// cached per type.
func mayHoldSensitive(valueType reflect.Type) bool {
	if cached, ok := holdsSensitiveCache.Load(valueType); ok {
		return cached.(bool)
	}

	result := reachesSensitive(valueType, map[reflect.Type]bool{})
	holdsSensitiveCache.Store(valueType, result)

	return result
}

var holdsSensitiveCache sync.Map

// reachesSensitive explores every type reachable from valueType once, and
// reports whether any of them is sensitive or an interface.
func reachesSensitive(valueType reflect.Type, visited map[reflect.Type]bool) bool {
	if visited[valueType] {
		return false
	}
	visited[valueType] = true

	if valueType.Kind() == reflect.Interface {
		return true
	}

	if valueType.Implements(sensitiveValueType) {
		return true
	}

	switch valueType.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return reachesSensitive(valueType.Elem(), visited)

	case reflect.Struct:
		for _, field := range cachedTypeFields(valueType) {
			if reachesSensitive(field.fieldType, visited) {
				return true
			}
		}
	}

	return false
}

// field is a struct field encoding/json writes.
type field struct {
	name        string
	encodedName []byte
	tagged      bool
	index       []int
	fieldType   reflect.Type
	omitEmpty   bool
	omitZero    bool
	quoted      bool
}

var typeFieldsCache sync.Map

func cachedTypeFields(structType reflect.Type) []field {
	if cached, ok := typeFieldsCache.Load(structType); ok {
		return cached.([]field)
	}

	fields := typeFields(structType)
	typeFieldsCache.Store(structType, fields)

	return fields
}

// typeFields returns the fields encoding/json writes for a struct type, in
// the order it writes them. It follows encoding/json's typeFields: embedded
// structs are flattened breadth first, and a name present at several depths
// is resolved by Go's dominance rules.
func typeFields(structType reflect.Type) []field {
	current := []field{}
	next := []field{{fieldType: structType}}

	var count map[reflect.Type]int
	nextCount := map[reflect.Type]int{}

	visited := map[reflect.Type]bool{}

	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, parent := range current {
			if visited[parent.fieldType] {
				continue
			}
			visited[parent.fieldType] = true

			for i := 0; i < parent.fieldType.NumField(); i++ {
				structField := parent.fieldType.Field(i)

				if structField.Anonymous {
					embedded := structField.Type
					if embedded.Kind() == reflect.Pointer {
						embedded = embedded.Elem()
					}

					if !structField.IsExported() && embedded.Kind() != reflect.Struct {
						continue
					}
				} else if !structField.IsExported() {
					continue
				}

				tag := structField.Tag.Get("json")
				if tag == "-" {
					continue
				}

				name, options, _ := strings.Cut(tag, ",")
				if !isValidTag(name) {
					name = ""
				}

				index := make([]int, len(parent.index)+1)
				copy(index, parent.index)
				index[len(parent.index)] = i

				fieldType := structField.Type
				if fieldType.Name() == "" && fieldType.Kind() == reflect.Pointer {
					fieldType = fieldType.Elem()
				}

				quoted := false
				if hasOption(options, "string") {
					switch fieldType.Kind() {
					case reflect.Bool,
						reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
						reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
						reflect.Float32, reflect.Float64,
						reflect.String:
						quoted = true
					}
				}

				if name != "" || !structField.Anonymous || fieldType.Kind() != reflect.Struct {
					tagged := name != ""
					if name == "" {
						name = structField.Name
					}

					encodedName, _ := json.Marshal(name)

					fields = append(fields, field{
						name:        name,
						encodedName: encodedName,
						tagged:      tagged,
						index:       index,
						fieldType:   fieldType,
						omitEmpty:   hasOption(options, "omitempty"),
						omitZero:    hasOption(options, "omitzero"),
						quoted:      quoted,
					})

					if count[parent.fieldType] > 1 {
						// Two copies at the same depth annihilate each other
						// in the dominance step below.
						fields = append(fields, fields[len(fields)-1])
					}

					continue
				}

				nextCount[fieldType]++
				if nextCount[fieldType] == 1 {
					next = append(next, field{name: fieldType.Name(), index: index, fieldType: fieldType})
				}
			}
		}
	}

	slices.SortFunc(fields, func(left, right field) int {
		if c := strings.Compare(left.name, right.name); c != 0 {
			return c
		}

		if c := len(left.index) - len(right.index); c != 0 {
			return c
		}

		if left.tagged != right.tagged {
			if left.tagged {
				return -1
			}

			return 1
		}

		return slices.Compare(left.index, right.index)
	})

	dominant := fields[:0:0]
	for start := 0; start < len(fields); {
		end := start + 1
		for end < len(fields) && fields[end].name == fields[start].name {
			end++
		}

		group := fields[start:end]
		if len(group) == 1 {
			dominant = append(dominant, group[0])
		} else if len(group[0].index) != len(group[1].index) || group[0].tagged != group[1].tagged {
			dominant = append(dominant, group[0])
		}

		start = end
	}

	slices.SortFunc(dominant, func(left, right field) int {
		return slices.Compare(left.index, right.index)
	})

	return dominant
}

func hasOption(options, option string) bool {
	for options != "" {
		var current string
		current, options, _ = strings.Cut(options, ",")
		if current == option {
			return true
		}
	}

	return false
}

// isValidTag follows encoding/json's rule for a usable tag name.
func isValidTag(name string) bool {
	if name == "" {
		return false
	}

	for _, character := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", character):
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9':
		case character > 127:
		default:
			return false
		}
	}

	return true
}
