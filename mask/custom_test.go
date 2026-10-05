package mask_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/mask"
)

// constantMasker is application code's own one-way masker: it writes the same
// value in place of everything it masks.
type constantMasker struct{}

func (constantMasker) Name() string { return "constant" }

func (constantMasker) Mask(json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`"hidden"`), nil
}

// reversingMasker is application code's own reversible masker: it reverses
// the bytes of a value, and reverses them back to open it.
type reversingMasker struct{}

func (reversingMasker) Name() string { return "reversing" }

func (reversingMasker) Mask(value json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(reverse(string(value)))
}

func (reversingMasker) Unmask(masked json.RawMessage) (json.RawMessage, error) {
	var reversed string
	if err := json.Unmarshal(masked, &reversed); err != nil {
		return nil, err
	}

	return json.RawMessage(reverse(reversed)), nil
}

func reverse(text string) string {
	runes := []rune(text)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}

	return string(runes)
}

var (
	_ mask.Masker     = constantMasker{}
	_ mask.Reversible = reversingMasker{}
)

func TestCustomMaskerOutputIsWrittenInTheEnvelope(t *testing.T) {
	envelope, err := mask.Envelope(json.RawMessage(`"s3cr3t"`), constantMasker{})

	require.NoError(t, err)
	require.Equal(t, `{"xcl_masked":"constant","value":"hidden"}`, string(envelope))
}

func TestCustomReversibleMaskerRoundTripsThroughEnvelopeAndUnmask(t *testing.T) {
	envelope, err := mask.Envelope(json.RawMessage(`{"password":"s3cr3t"}`), reversingMasker{})
	require.NoError(t, err)

	value, err := mask.Unmask(envelope, reversingMasker{})
	require.NoError(t, err)
	require.JSONEq(t, `{"password":"s3cr3t"}`, string(value))
}
