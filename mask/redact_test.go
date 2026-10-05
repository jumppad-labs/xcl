package mask_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/types"
)

func TestRedactMaskGivesTheSensitiveMarker(t *testing.T) {
	output, err := mask.Redact().Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	var marker string
	require.NoError(t, json.Unmarshal(output, &marker))
	require.Equal(t, types.SensitiveMarker, marker)
	require.Equal(t, "(sensitive)", marker)
}

func TestRedactEnvelopeHoldsTheSensitiveMarker(t *testing.T) {
	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), mask.Redact())

	require.NoError(t, err)
	require.Equal(t, `{"xcl_masked":"redact","value":"(sensitive)"}`, string(envelope))
}
