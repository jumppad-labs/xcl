package diff_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
)

func TestNewOptionsDefaultsRevealSensitiveToFalse(t *testing.T) {
	options := diff.NewOptions()

	require.False(t, options.RevealSensitive)
}

func TestNewOptionsWithRevealSensitiveSetsRevealSensitive(t *testing.T) {
	options := diff.NewOptions(diff.RevealSensitive())

	require.True(t, options.RevealSensitive)
}

func TestNewOptionsIgnoresNilOption(t *testing.T) {
	options := diff.NewOptions(nil)

	require.False(t, options.RevealSensitive)
}

func TestNewOptionsIgnoresNilOptionAlongsideRevealSensitive(t *testing.T) {
	options := diff.NewOptions(nil, diff.RevealSensitive(), nil)

	require.True(t, options.RevealSensitive)
}
