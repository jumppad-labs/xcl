package mask_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/mask"
)

func TestHMACSHA256SameInputAndKeyGiveTheSameOutput(t *testing.T) {
	first, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	second, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	firstOutput, err := first.Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	secondOutput, err := second.Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	require.Equal(t, string(firstOutput), string(secondOutput))
}

func TestHMACSHA256DifferentKeyGivesDifferentOutput(t *testing.T) {
	first, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	second, err := mask.HashHMACSHA256([]byte("another-key"))
	require.NoError(t, err)

	firstOutput, err := first.Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	secondOutput, err := second.Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	require.NotEqual(t, string(firstOutput), string(secondOutput))
}

func TestHMACSHA256RejectsAnEmptyKey(t *testing.T) {
	masker, err := mask.HashHMACSHA256([]byte{})

	require.Error(t, err)
	require.Nil(t, masker)
}

func TestHMACSHA256IsNotReversible(t *testing.T) {
	masker, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	_, ok := masker.(mask.Reversible)
	require.False(t, ok)
}
