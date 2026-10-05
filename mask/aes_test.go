package mask_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/mask"
)

// aesKey and otherAESKey are two distinct 32 byte keys, kept by hand so a
// failure points straight at the key that was used.
var (
	aesKey      = []byte("0123456789abcdef0123456789abcdef")
	otherAESKey = []byte("fedcba9876543210fedcba9876543210")
)

func TestAES256GCMOutputDiffersFromInput(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	input := json.RawMessage(`"s3cr3t"`)

	output, err := masker.Mask(input)
	require.NoError(t, err)

	require.NotEqual(t, string(input), string(output))
	require.False(t, bytes.Contains(output, []byte("s3cr3t")))
}

func TestAES256GCMEnvelopeRoundTripsWithTheSameKey(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, masker)
	require.NoError(t, err)
	require.JSONEq(t, `"s3cr3t"`, string(value))
}

func TestAES256GCMReversibleUnmaskRoundTripsWithTheSameKey(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	reversible, ok := masker.(mask.Reversible)
	require.True(t, ok)

	masked, err := reversible.Mask(json.RawMessage(`{"user":"admin","port":5432}`))
	require.NoError(t, err)

	value, err := reversible.Unmask(masked)
	require.NoError(t, err)
	require.JSONEq(t, `{"user":"admin","port":5432}`, string(value))
}

func TestAES256GCMFailsToOpenWithADifferentKey(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	other, err := mask.EncryptAES256GCM(otherAESKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, other)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrUnrecoverable))
	require.Nil(t, value)
}

func TestAES256GCMMaskingOneValueTwiceGivesDifferentOutput(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	first, err := masker.Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	second, err := masker.Mask(json.RawMessage(`"s3cr3t"`))
	require.NoError(t, err)

	require.NotEqual(t, string(first), string(second))
}

func TestAES256GCMRejectsA16ByteKey(t *testing.T) {
	masker, err := mask.EncryptAES256GCM([]byte("0123456789abcdef"))

	require.Error(t, err)
	require.Nil(t, masker)
}

func TestAES256GCMCopiesTheKey(t *testing.T) {
	key := append([]byte(nil), aesKey...)

	masker, err := mask.EncryptAES256GCM(key)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	for index := range key {
		key[index] = 0
	}

	value, err := mask.Unmask(envelope, masker)
	require.NoError(t, err)
	require.JSONEq(t, `"s3cr3t"`, string(value))
}
