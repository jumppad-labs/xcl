package testutil

import (
	"encoding/json"
	"testing"

	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// EntityByID returns the entity in entities whose meta records the given id.
// Storage answers no questions about addresses, so a test holding saved
// entities scans them and compares the id each entity already records
func EntityByID(entities []any, id string) (any, error) {
	for _, e := range entities {
		meta, err := types.GetMeta(e)
		if err != nil {
			continue
		}

		if meta.ID == id {
			return e, nil
		}
	}

	return nil, state.ResourceNotFoundError{Resource: id}
}

// SavedID returns the address a raw saved record carries, which is how a
// record is matched to the entity it was written from
func SavedID(t testing.TB, record json.RawMessage) string {
	t.Helper()

	var envelope struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}

	require.NoError(t, json.Unmarshal(record, &envelope))
	require.NotEmpty(t, envelope.Meta.ID)

	return envelope.Meta.ID
}
