package mask_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/mask"
)

// passthrough is a reversible masker that writes the value unchanged. It has
// a name of its own, so it stands in for any masker other than the one an
// envelope names.
type passthrough struct{}

func (passthrough) Name() string { return "passthrough" }

func (passthrough) Mask(value json.RawMessage) (json.RawMessage, error) { return value, nil }

func (passthrough) Unmask(masked json.RawMessage) (json.RawMessage, error) { return masked, nil }

// Unmask, accepted.

func TestUnmaskOpensAReversibleEnvelope(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`{"password":"s3cr3t"}`), masker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, masker)
	require.NoError(t, err)
	require.JSONEq(t, `{"password":"s3cr3t"}`, string(value))
}

// Unmask, rejected.

func TestUnmaskRejectsAOneWayEnvelope(t *testing.T) {
	masker, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, masker)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrUnrecoverable))
	require.Nil(t, value)
}

func TestUnmaskRejectsAnAESEnvelopeOpenedWithADifferentlyNamedMasker(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, passthrough{})
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrUnrecoverable))
	require.Nil(t, value)
}

func TestUnmaskRejectsAnHMACEnvelopeOpenedWithAES(t *testing.T) {
	hmacMasker, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	aesMasker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), hmacMasker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, aesMasker)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrUnrecoverable))
	require.Nil(t, value)
}

func TestUnmaskRejectsAnEnvelopeOpenedWithTheWrongKey(t *testing.T) {
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

func TestUnmaskRejectsDataThatIsNotAnEnvelope(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	value, err := mask.Unmask(json.RawMessage(`{"password":"s3cr3t"}`), masker)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrUnrecoverable))
	require.Nil(t, value)
}

func TestUnmaskRejectsANilMasker(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrUnrecoverable))
	require.Nil(t, value)
}

// IsMasked.

func TestIsMaskedIsFalseForAPlainObject(t *testing.T) {
	_, ok := mask.IsMasked(json.RawMessage(`{"a":1}`))

	require.False(t, ok)
}

func TestIsMaskedIsFalseForAnObjectWithAnExtraKey(t *testing.T) {
	_, ok := mask.IsMasked(json.RawMessage(`{"xcl_masked":"redact","value":"(sensitive)","extra":true}`))

	require.False(t, ok)
}

func TestIsMaskedIsFalseForANonObject(t *testing.T) {
	_, ok := mask.IsMasked(json.RawMessage(`"xcl_masked"`))

	require.False(t, ok)
}

func TestIsMaskedIsTrueForAWellFormedEnvelope(t *testing.T) {
	masked, ok := mask.IsMasked(json.RawMessage(`{"xcl_masked":"redact","value":"(sensitive)"}`))

	require.True(t, ok)
	require.Equal(t, "redact", masked.By)
	require.JSONEq(t, `"(sensitive)"`, string(masked.Value))
}

// IsMaskedObject.

func TestIsMaskedObjectIsFalseForAPlainObject(t *testing.T) {
	_, ok := mask.IsMaskedObject(map[string]any{"a": float64(1)})

	require.False(t, ok)
}

func TestIsMaskedObjectIsFalseForAnObjectWithAnExtraKey(t *testing.T) {
	_, ok := mask.IsMaskedObject(map[string]any{
		"xcl_masked": "redact",
		"value":      "(sensitive)",
		"extra":      true,
	})

	require.False(t, ok)
}

func TestIsMaskedObjectIsFalseWhenTheMaskerIsNotAString(t *testing.T) {
	_, ok := mask.IsMaskedObject(map[string]any{"xcl_masked": float64(1)})

	require.False(t, ok)
}

func TestIsMaskedObjectIsTrueForAWellFormedEnvelope(t *testing.T) {
	masked, ok := mask.IsMaskedObject(map[string]any{
		"xcl_masked": "redact",
		"value":      "(sensitive)",
	})

	require.True(t, ok)
	require.Equal(t, "redact", masked.By)
	require.JSONEq(t, `"(sensitive)"`, string(masked.Value))
}

// Every envelope names the masker that produced it.

func TestAES256GCMEnvelopeNamesItsMasker(t *testing.T) {
	masker, err := mask.EncryptAES256GCM(aesKey)
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	masked, ok := mask.IsMasked(envelope)
	require.True(t, ok)
	require.Equal(t, mask.AES256GCMName, masked.By)
}

func TestHMACSHA256EnvelopeNamesItsMasker(t *testing.T) {
	masker, err := mask.HashHMACSHA256([]byte("correlation-key"))
	require.NoError(t, err)

	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), masker)
	require.NoError(t, err)

	masked, ok := mask.IsMasked(envelope)
	require.True(t, ok)
	require.Equal(t, mask.HMACSHA256Name, masked.By)
}

func TestOmitEnvelopeNamesItsMasker(t *testing.T) {
	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), mask.Omit())
	require.NoError(t, err)

	masked, ok := mask.IsMasked(envelope)
	require.True(t, ok)
	require.Equal(t, mask.OmitName, masked.By)
}

func TestRedactEnvelopeNamesItsMasker(t *testing.T) {
	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), mask.Redact())
	require.NoError(t, err)

	masked, ok := mask.IsMasked(envelope)
	require.True(t, ok)
	require.Equal(t, mask.RedactName, masked.By)
}
