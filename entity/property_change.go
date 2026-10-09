package entity

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// sensitiveMarker is shown in place of a sensitive change's values whenever
// the change is printed, logged or encoded. It matches types.SensitiveMarker,
// which this package cannot import.
const sensitiveMarker = "(sensitive)"

// unknownMarker is shown in place of a new value that is only known once the
// apply runs.
const unknownMarker = "(known after apply)"

// PropertyChange is one setting of a provider-backed resource that differs
// between its last apply and the new configuration.
//
// Before and After hold plain JSON values: string, float64, bool, nil,
// []any or map[string]any, the same whether the plugin runs in the program or
// as a separate one.
//
// A sensitive change holds its real values in Before and After, so a plugin
// can act on them. Printing, logging or encoding the change to JSON shows
// "(sensitive)" in their place, so a stray %v or log line does not leak them.
type PropertyChange struct {
	// Path is where the setting is, in configuration names, for example
	// network[0].name.
	Path Path
	// Before is the previous value, nil when the setting was added.
	Before any
	// After is the new value, nil when the setting was removed or when
	// Unknown is set.
	After any
	// Unknown is set when the new value is only known once the apply runs. It
	// is only ever set for Changed, never for Update.
	Unknown bool
	// Sensitive is set when the setting is sensitive. Before and After still
	// hold the real values.
	Sensitive bool
}

// At returns true when the change is exactly at path.
func (c PropertyChange) At(path Path) bool {
	return c.Path.Equal(path)
}

// Within returns true when the change is at path or anywhere inside it, so a
// change at network[0].name is within network.
func (c PropertyChange) Within(path Path) bool {
	return c.Path.Within(path)
}

// String returns the change in its written form, such as
// network[0].name: "app" -> "backend". A sensitive change shows
// "(sensitive)" for both values.
func (c PropertyChange) String() string {
	before, after := c.shownValues()
	return fmt.Sprintf("%s: %s -> %s", c.Path, displayValue(before), displayValue(after))
}

// Format writes String for every verb, so %v, %+v and %#v never show a
// sensitive change's real values.
func (c PropertyChange) Format(state fmt.State, verb rune) {
	fmt.Fprint(state, c.String())
}

// LogValue returns the change as a group of path, before, after, unknown and
// sensitive, with a sensitive change's values replaced by "(sensitive)".
func (c PropertyChange) LogValue() slog.Value {
	before, after := c.shownValues()

	return slog.GroupValue(
		slog.String("path", c.Path.String()),
		slog.Any("before", before),
		slog.Any("after", after),
		slog.Bool("unknown", c.Unknown),
		slog.Bool("sensitive", c.Sensitive),
	)
}

// MarshalJSON encodes the change as an object of path, before, after, unknown
// and sensitive, with a sensitive change's values replaced by "(sensitive)".
// It is for display only; xcl never uses it to carry changes to plugins.
func (c PropertyChange) MarshalJSON() ([]byte, error) {
	before, after := c.shownValues()

	return json.Marshal(struct {
		Path      Path `json:"path"`
		Before    any  `json:"before"`
		After     any  `json:"after"`
		Unknown   bool `json:"unknown"`
		Sensitive bool `json:"sensitive"`
	}{
		Path:      c.Path,
		Before:    before,
		After:     after,
		Unknown:   c.Unknown,
		Sensitive: c.Sensitive,
	})
}

// shownValues returns the values safe to show: the marker for both values of
// a sensitive change, otherwise the real values
func (c PropertyChange) shownValues() (any, any) {
	if c.Sensitive {
		return sensitiveMarker, sensitiveMarker
	}

	return c.Before, c.After
}

// displayValue returns a value as JSON text for String; the sensitive marker
// is shown bare
func displayValue(value any) string {
	if text, ok := value.(string); ok && text == sensitiveMarker {
		return text
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}

	return string(encoded)
}
