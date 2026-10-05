package types

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jumppad-labs/xcl/internal/cty"
)

// Output is the entity an output block declares: a value a configuration or
// module publishes.
//
// Value holds the published value as plain Go, which is what applications
// read. CtyValue holds the evaluated value and is used by the evaluator.
//
// The parts of Value that are sensitive are held as Sensitive values: a
// sensitive string, number or bool as Sensitive[string], Sensitive[float64]
// or Sensitive[bool], and a sensitive object or list as
// Sensitive[map[string]any] or Sensitive[[]any]. SensitivePaths records where
// they are, so reading an output back from JSON wraps them again.
type Output struct {
	ResourceBase `xcl:",remain"`

	CtyValue    cty.Value `xcl:"value,optional"` // value of the output
	Value       any       `json:"value"`
	Description string    `xcl:"description,optional" json:"description,omitempty"` // description for the output

	// SensitivePaths holds the path, inside Value, of each part that is
	// sensitive. An empty path means the whole value is sensitive. Map keys
	// and list indices are written as strings.
	SensitivePaths [][]string `json:"sensitive_paths,omitempty"`
}

// outputJSON has Output's fields without its methods, so UnmarshalJSON can
// decode into it without recursing.
type outputJSON Output

// Format writes the output as fmt would, except that an evaluated value
// carrying a sensitive part is shown as SensitiveMarker. CtyValue holds the
// real value for the evaluator, and fmt would otherwise reach it by
// reflection.
func (o Output) Format(state fmt.State, verb rune) {
	shown := outputJSON(o)
	if o.CtyValue != cty.NilVal && o.CtyValue.ContainsMarked() {
		shown.CtyValue = cty.StringVal(SensitiveMarker)
	}

	fmt.Fprintf(state, fmt.FormatString(state, verb), shown)
}

// UnmarshalJSON reads an output and wraps the parts of Value named by
// SensitivePaths as Sensitive values again.
func (o *Output) UnmarshalJSON(data []byte) error {
	decoded := outputJSON{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*o = Output(decoded)
	o.WrapSensitivePaths()

	return nil
}

// WrapSensitivePaths wraps each part of Value named by SensitivePaths as a
// Sensitive value, leaving parts that already are one as they are.
func (o *Output) WrapSensitivePaths() {
	for _, path := range o.SensitivePaths {
		o.Value = wrapSensitiveAt(o.Value, path)
	}
}

// wrapSensitiveAt returns value with the part at path wrapped as a Sensitive
// value. A path that does not lead anywhere leaves value unchanged.
func wrapSensitiveAt(value any, path []string) any {
	if len(path) == 0 {
		return wrapSensitive(value)
	}

	switch container := value.(type) {
	case map[string]any:
		element, ok := container[path[0]]
		if !ok {
			return value
		}

		container[path[0]] = wrapSensitiveAt(element, path[1:])

	case []any:
		index, err := strconv.Atoi(path[0])
		if err != nil || index < 0 || index >= len(container) {
			return value
		}

		container[index] = wrapSensitiveAt(container[index], path[1:])
	}

	return value
}

// wrapSensitive wraps a plain Go value read from JSON or converted from cty
// as the Sensitive type for its kind. The value is passed through JSON, so the
// bare marker, as event data carries it, gives a redacted value.
func wrapSensitive(value any) any {
	switch value.(type) {
	case nil, SensitiveValue:
		return value
	}

	data, err := json.Marshal(value)
	if err != nil {
		return value
	}

	switch value.(type) {
	case string:
		return unmarshalSensitive[string](data, value)
	case float64:
		return unmarshalSensitive[float64](data, value)
	case bool:
		return unmarshalSensitive[bool](data, value)
	case map[string]any:
		return unmarshalSensitive[map[string]any](data, value)
	case []any:
		return unmarshalSensitive[[]any](data, value)
	}

	return value
}

func unmarshalSensitive[T any](data []byte, fallback any) any {
	sensitive := Sensitive[T]{}
	if err := json.Unmarshal(data, &sensitive); err != nil {
		return fallback
	}

	return sensitive
}
