package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/jumppad-labs/xcl/internal/cty/gocty"
)

// SensitiveMarker is the fixed text every display of a sensitive value
// shows in place of the real value.
const SensitiveMarker = "(sensitive)"

// quotedSensitiveMarker is SensitiveMarker as a JSON string.
var quotedSensitiveMarker = []byte(`"` + SensitiveMarker + `"`)

// Sensitive holds a value that must not be shown. A type author declares a
// field sensitive by giving it this type:
//
//	Password types.Sensitive[string] `xcl:"password" json:"password"`
//
// Printing it with fmt, logging it with log/slog, or marshalling it as text
// or JSON yields SensitiveMarker. Only Reveal returns the real value, and
// once revealed the value is an ordinary value that is no longer protected.
//
// State keeps the real value: xcl writes it through its own internal
// encoder, and UnmarshalJSON reads the real value back.
type Sensitive[T any] struct {
	value T

	// redacted is set when the value was read back from the marker, such as
	// from event data, and so holds no real value.
	redacted bool
}

// NewSensitive wraps value as a sensitive value.
func NewSensitive[T any](value T) Sensitive[T] {
	return Sensitive[T]{value: value}
}

// Reveal returns the real value. It is the only way to the real value; a
// revealed value is no longer protected and must not be logged or emitted.
//
// A value read back from the marker holds no real value, and Reveal returns
// the zero value of T.
func (s Sensitive[T]) Reveal() T {
	return s.value
}

// String returns SensitiveMarker.
func (s Sensitive[T]) String() string {
	return SensitiveMarker
}

// GoString returns SensitiveMarker, so %#v does not show the real value.
func (s Sensitive[T]) GoString() string {
	return SensitiveMarker
}

// Format writes SensitiveMarker for every verb, honouring only the width.
func (s Sensitive[T]) Format(state fmt.State, verb rune) {
	text := SensitiveMarker
	if width, ok := state.Width(); ok && width > len(text) {
		padding := string(bytes.Repeat([]byte(" "), width-len(text)))
		if state.Flag('-') {
			text = text + padding
		} else {
			text = padding + text
		}
	}

	fmt.Fprint(state, text)
}

// LogValue returns SensitiveMarker as a slog string value.
func (s Sensitive[T]) LogValue() slog.Value {
	return slog.StringValue(SensitiveMarker)
}

// MarshalText returns SensitiveMarker.
func (s Sensitive[T]) MarshalText() ([]byte, error) {
	return []byte(SensitiveMarker), nil
}

// MarshalJSON returns SensitiveMarker as a JSON string. xcl writes the real
// value to state and to plugins through its own internal encoder.
func (s Sensitive[T]) MarshalJSON() ([]byte, error) {
	return append([]byte(nil), quotedSensitiveMarker...), nil
}

// UnmarshalJSON reads the real value. Reading the bare marker gives a
// redacted value, which holds no real value and still shows the marker.
func (s *Sensitive[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), quotedSensitiveMarker) {
		var zero T
		s.value = zero
		s.redacted = true

		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	s.value = value
	s.redacted = false

	return nil
}

// RevealAny returns the real value as any. It lets xcl's own encoders reveal
// a sensitive value without knowing T; like Reveal, it is explicit.
func (s Sensitive[T]) RevealAny() any {
	return s.value
}

func (s Sensitive[T]) sensitive() {}

func (s Sensitive[T]) isRedacted() bool {
	return s.redacted
}

// setAny stores value, which must be a T, as the real value.
func (s *Sensitive[T]) setAny(value any) error {
	typed, ok := value.(T)
	if !ok {
		var zero T
		return fmt.Errorf("cannot store a %T in a sensitive %T", value, zero)
	}

	s.value = typed
	s.redacted = false

	return nil
}

// SensitiveValue is implemented by every Sensitive[T]. It lets xcl recognise
// a sensitive value of any T. It is sealed: only Sensitive implements it.
type SensitiveValue interface {
	// RevealAny returns the real value as any.
	RevealAny() any

	sensitive()
}

// IsRedacted reports whether value was read back from the marker, such as
// from event data, and so holds no real value to reveal.
func IsRedacted(value SensitiveValue) bool {
	redactable, ok := value.(interface{ isRedacted() bool })
	if !ok {
		return false
	}

	return redactable.isRedacted()
}

// sensitiveMark is the type of SensitiveMark. It is unexported so no other
// package can make a value equal to it.
type sensitiveMark struct{}

// SensitiveMark is the cty mark sensitivity travels as through configuration.
var SensitiveMark any = sensitiveMark{}

// redactedMark is the type of RedactedMark.
type redactedMark struct{}

// RedactedMark is the cty mark carried, beside SensitiveMark, by a sensitive
// value read back from the marker, which has no real value to reveal.
var RedactedMark any = redactedMark{}

// sensitiveSetter is implemented by a pointer to every Sensitive[T].
type sensitiveSetter interface {
	setAny(value any) error
}

var (
	sensitiveValueType  = reflect.TypeOf((*SensitiveValue)(nil)).Elem()
	sensitiveSetterType = reflect.TypeOf((*sensitiveSetter)(nil)).Elem()
)

// init registers Sensitive with the cty conversion library, so a sensitive
// field converts as its inner type carrying SensitiveMark, and a cty value
// decoded into a sensitive field is unmarked and wrapped.
func init() {
	gocty.RegisterWrapper(gocty.Wrapper{
		Inner: func(t reflect.Type) (reflect.Type, bool) {
			if t.Kind() != reflect.Struct || !t.Implements(sensitiveValueType) || !reflect.PointerTo(t).Implements(sensitiveSetterType) {
				return nil, false
			}

			return t.Field(0).Type, true
		},
		Unwrap: func(wrapper reflect.Value) reflect.Value {
			return reflect.ValueOf(wrapper.Interface().(SensitiveValue).RevealAny())
		},
		Wrap: func(target reflect.Value, inner reflect.Value) {
			// inner always has the wrapper's inner type, so setAny cannot fail.
			_ = target.Addr().Interface().(sensitiveSetter).setAny(inner.Interface())
		},
		Mark: SensitiveMark,
		ExtraMarks: func(wrapper reflect.Value) []any {
			if IsRedacted(wrapper.Interface().(SensitiveValue)) {
				return []any{RedactedMark}
			}

			return nil
		},
	})
}
