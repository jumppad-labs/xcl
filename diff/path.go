package diff

import (
	"encoding/json"
	"strconv"
	"strings"
)

// StepKind is the kind of a Path step.
type StepKind int

const (
	// StepAttribute names a field or nested block, such as image.
	StepAttribute StepKind = iota

	// StepIndex is a position in a list, such as [0].
	StepIndex

	// StepKey is a key in a map, such as ["LOG_LEVEL"].
	StepKey
)

// Step is one segment of a Path. Kind says which of Attribute, Index or Key
// holds the segment.
type Step struct {
	Kind      StepKind
	Attribute string
	Index     int
	Key       string
}

// Path locates a value within a resource, as a list of steps rather than a
// string, so a consumer can rebuild a tree without parsing strings apart.
type Path []Step

// Attribute returns a copy of the path extended with a field or nested block
// name.
func (p Path) Attribute(name string) Path {
	return p.with(Step{Kind: StepAttribute, Attribute: name})
}

// Index returns a copy of the path extended with a list position.
func (p Path) Index(index int) Path {
	return p.with(Step{Kind: StepIndex, Index: index})
}

// Key returns a copy of the path extended with a map key.
func (p Path) Key(key string) Path {
	return p.with(Step{Kind: StepKey, Key: key})
}

// with returns a new path holding p followed by step, never sharing p's
// backing array
func (p Path) with(step Step) Path {
	extended := make(Path, len(p), len(p)+1)
	copy(extended, p)
	return append(extended, step)
}

// String returns the path in its written form: attributes joined by dots, a
// list position in brackets and a map key quoted in brackets, such as image,
// ports[0].host or env["LOG_LEVEL"].
func (p Path) String() string {
	var builder strings.Builder

	for i, step := range p {
		switch step.Kind {
		case StepIndex:
			builder.WriteString("[")
			builder.WriteString(strconv.Itoa(step.Index))
			builder.WriteString("]")
		case StepKey:
			builder.WriteString("[")
			builder.WriteString(strconv.Quote(step.Key))
			builder.WriteString("]")
		default:
			if i > 0 {
				builder.WriteString(".")
			}
			builder.WriteString(step.Attribute)
		}
	}

	return builder.String()
}

// MarshalJSON encodes the path as its written form, see String.
func (p Path) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}
