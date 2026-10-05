package types

import (
	"github.com/jumppad-labs/xcl/internal/cty"
)

// Output is the entity an output block declares: a value a configuration or
// module publishes.
//
// Value holds the published value as plain Go, which is what applications
// read. CtyValue holds the evaluated value and is used by the evaluator.
type Output struct {
	ResourceBase `xcl:",remain"`

	CtyValue    cty.Value `xcl:"value,optional"` // value of the output
	Value       any       `json:"value"`
	Description string    `xcl:"description,optional" json:"description,omitempty"` // description for the output
}
