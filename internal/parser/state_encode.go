package parser

import (
	"encoding/json"
	"fmt"

	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/types"
)

// EncodeForState encodes each entity as the JSON a state store saves, with
// every sensitive value written as its real value. A store receives these raw
// messages rather than the typed entities, so a store that marshals what it is
// given with encoding/json keeps real values without reaching xcl's internal
// encoder. These are the points the state save sites share.
func EncodeForState(entities []any) ([]any, error) {
	if entities == nil {
		return nil, nil
	}

	encoded := make([]any, 0, len(entities))
	for _, entity := range entities {
		data, err := wire.Marshal(entity)
		if err != nil {
			id := ""
			if meta, metaErr := types.GetMeta(entity); metaErr == nil {
				id = meta.ID
			}

			return nil, fmt.Errorf("unable to encode %s for state: %w", id, err)
		}

		encoded = append(encoded, json.RawMessage(data))
	}

	return encoded, nil
}
