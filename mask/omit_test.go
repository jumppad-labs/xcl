package mask_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/mask"
)

func TestOmitMaskLeavesNoValue(t *testing.T) {
	output, err := mask.Omit().Mask(json.RawMessage(`"s3cr3t"`))

	require.NoError(t, err)
	require.Nil(t, output)
}

func TestOmitEnvelopeNamesTheMaskerAndHoldsNoValue(t *testing.T) {
	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), mask.Omit())

	require.NoError(t, err)
	require.Equal(t, `{"xcl_masked":"omit"}`, string(envelope))
}
