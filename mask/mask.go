// Package mask turns sensitive values into masked data, and back again where
// that is possible.
//
// A Masker receives the JSON encoding of a sensitive value's real value and
// returns the JSON to write in its place. xcl writes the result in an
// envelope, in exactly the position the real value would occupy:
//
//	{"xcl_masked":"<masker name>","value":<masker output>}
//
// The envelope names the masker that produced it, so a reader can tell
// whether the value can be recovered without knowing the Go type it belongs
// to. The value is absent when the masker leaves none, as Omit does. The key
// xcl_masked is reserved for the envelope: an object holding it, with at most
// a value beside it, is always read as masked data.
//
// A masker that can open what it produced implements Reversible. Only a
// reversible masker can be configured for state, since state must be read
// back to the real value. Unmask opens one envelope, and reports anything it
// cannot open faithfully as xclerrors.ErrUnrecoverable rather than returning a
// wrong value.
//
// The built-in maskers are EncryptAES256GCM, which is reversible,
// HashHMACSHA256, Omit and Redact, which are one way. Application code can
// supply its own by implementing Masker, and Reversible where the masker can
// open what it masks.
package mask

import (
	"bytes"
	"encoding/json"
	"fmt"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// EnvelopeKey is the reserved key naming the masker in masked data.
const EnvelopeKey = "xcl_masked"

// Masker masks a sensitive value. Name identifies the masker, and is written
// into everything it masks. Mask receives the JSON encoding of the real value
// and returns the JSON to write as the envelope's value, or nil to write no
// value.
type Masker interface {
	Name() string
	Mask(value json.RawMessage) (json.RawMessage, error)
}

// Reversible is a masker that can open what it produced. Unmask receives the
// envelope's value and returns the JSON of the real value, or an error when
// the value does not open, for example under a different key.
type Reversible interface {
	Masker
	Unmask(masked json.RawMessage) (json.RawMessage, error)
}

// Masked is the envelope every masked value is written in, in place of the
// real value. By is the producing masker's Name, and Value its output, absent
// where the masker leaves no value.
type Masked struct {
	By    string          `json:"xcl_masked"`
	Value json.RawMessage `json:"value,omitempty"`
}

// Envelope masks the JSON of a real value with m and returns the envelope to
// write in its place.
func Envelope(value json.RawMessage, m Masker) (json.RawMessage, error) {
	output, err := m.Mask(value)
	if err != nil {
		return nil, fmt.Errorf("masker %q failed: %w", m.Name(), err)
	}

	if len(output) > 0 && !json.Valid(output) {
		return nil, fmt.Errorf("masker %q returned invalid JSON", m.Name())
	}

	return json.Marshal(Masked{By: m.Name(), Value: output})
}

// IsMasked reports whether data is an envelope, and returns it when it is. An
// envelope is a JSON object whose keys are xcl_masked, holding a non-empty
// string, and optionally value.
func IsMasked(data json.RawMessage) (Masked, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Masked{}, false
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return Masked{}, false
	}

	return fromFields(fields)
}

// IsMaskedObject reports whether a JSON object already decoded into a map is
// an envelope, and returns it when it is. It is IsMasked for a reader that
// holds a decoded record rather than its bytes.
func IsMaskedObject(object map[string]any) (Masked, bool) {
	by, ok := object[EnvelopeKey].(string)
	if !ok || by == "" {
		return Masked{}, false
	}

	for key := range object {
		if key != EnvelopeKey && key != "value" {
			return Masked{}, false
		}
	}

	masked := Masked{By: by}
	if value, ok := object["value"]; ok {
		encoded, err := json.Marshal(value)
		if err != nil {
			return Masked{}, false
		}

		masked.Value = encoded
	}

	return masked, true
}

func fromFields(fields map[string]json.RawMessage) (Masked, bool) {
	rawBy, ok := fields[EnvelopeKey]
	if !ok {
		return Masked{}, false
	}

	for key := range fields {
		if key != EnvelopeKey && key != "value" {
			return Masked{}, false
		}
	}

	var by string
	if err := json.Unmarshal(rawBy, &by); err != nil || by == "" {
		return Masked{}, false
	}

	return Masked{By: by, Value: fields["value"]}, true
}

// Unmask opens one envelope with m and returns the JSON of the real value.
//
// It fails with *xclerrors.UnrecoverableError, which answers errors.Is with
// xclerrors.ErrUnrecoverable, when data is not an envelope, when m is nil or
// not Reversible, when the envelope names a different masker, or when m cannot
// open the value. It never returns a wrong value.
func Unmask(data json.RawMessage, m Masker) (json.RawMessage, error) {
	masked, ok := IsMasked(data)
	if !ok {
		return nil, &xclerrors.UnrecoverableError{Reason: "the data is not masked data"}
	}

	return Open(masked, m)
}

// Open opens an envelope already read with IsMasked or IsMaskedObject, with
// the same rules as Unmask.
func Open(masked Masked, m Masker) (json.RawMessage, error) {
	if m == nil {
		return nil, &xclerrors.UnrecoverableError{MaskedBy: masked.By, Reason: "no masker was given to open it"}
	}

	if masked.By != m.Name() {
		return nil, &xclerrors.UnrecoverableError{
			MaskedBy: masked.By,
			Reason:   fmt.Sprintf("it was masked by a different masker than %q", m.Name()),
		}
	}

	reversible, ok := m.(Reversible)
	if !ok {
		return nil, &xclerrors.UnrecoverableError{MaskedBy: masked.By, Reason: "the masker is one way"}
	}

	if len(masked.Value) == 0 {
		return nil, &xclerrors.UnrecoverableError{MaskedBy: masked.By, Reason: "the masked data holds no value"}
	}

	value, err := reversible.Unmask(masked.Value)
	if err != nil {
		return nil, &xclerrors.UnrecoverableError{MaskedBy: masked.By, Reason: "it does not open", Err: err}
	}

	if !json.Valid(value) {
		return nil, &xclerrors.UnrecoverableError{MaskedBy: masked.By, Reason: "it opened to invalid JSON"}
	}

	return value, nil
}
