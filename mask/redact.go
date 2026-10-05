package mask

import (
	"encoding/json"

	"github.com/jumppad-labs/xcl/types"
)

// RedactName is the name the redact masker writes into what it masks.
const RedactName = "redact"

// Redact returns a one-way masker that replaces every value with the fixed
// marker types.SensitiveMarker. It is the masker event data uses unless
// another is chosen.
func Redact() Masker {
	return redact{}
}

type redact struct{}

func (redact) Name() string { return RedactName }

func (redact) Mask(json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(types.SensitiveMarker)
}
