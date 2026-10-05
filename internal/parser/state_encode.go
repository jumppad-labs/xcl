package parser

import (
	"encoding/json"
	"fmt"

	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/types"
)

// PlaintextStateWarning is the message of the warning Apply and Destroy emit,
// once per operation, when they write a sensitive value to a state store in
// plain text because no state masker is configured.
const PlaintextStateWarning = "sensitive values are stored unencrypted in state; use xcl.WithStateMask to encrypt them"

// EncodeForState encodes each entity as the JSON a state store saves. Every
// sensitive value is written as stateMask's envelope, or as its real value
// when stateMask is nil. A store receives these raw messages rather than the
// typed entities, so a store that marshals what it is given with
// encoding/json keeps what was encoded here without reaching xcl's internal
// encoder. These are the points the state save sites share.
//
// unmaskedSensitive reports whether any sensitive value was written in plain
// text, which is so when there is no state masker and at least one entity
// holds a sensitive value.
func EncodeForState(entities []any, stateMask mask.Masker) (records []any, unmaskedSensitive bool, err error) {
	if entities == nil {
		return nil, false, nil
	}

	encoded := make([]any, 0, len(entities))
	for _, entity := range entities {
		result, err := wire.Encode(entity, wire.Options{Mask: stateMask})
		if err != nil {
			id := ""
			if meta, metaErr := types.GetMeta(entity); metaErr == nil {
				id = meta.ID
			}

			return nil, false, fmt.Errorf("unable to encode %s for state: %w", id, err)
		}

		if result.Sensitive && stateMask == nil {
			unmaskedSensitive = true
		}

		encoded = append(encoded, json.RawMessage(result.Data))
	}

	return encoded, unmaskedSensitive, nil
}
