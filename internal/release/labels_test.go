package release

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSizeFromLabelsReturnsMajorForMajorLabel(t *testing.T) {
	size, err := SizeFromLabels([]string{"major"})

	require.NoError(t, err)
	require.Equal(t, SizeMajor, size)
}

func TestSizeFromLabelsReturnsMinorForMinorLabel(t *testing.T) {
	size, err := SizeFromLabels([]string{"minor"})

	require.NoError(t, err)
	require.Equal(t, SizeMinor, size)
}

func TestSizeFromLabelsReturnsPatchForPatchLabel(t *testing.T) {
	size, err := SizeFromLabels([]string{"patch"})

	require.NoError(t, err)
	require.Equal(t, SizePatch, size)
}

func TestSizeFromLabelsReturnsNoReleaseForNoReleaseLabel(t *testing.T) {
	size, err := SizeFromLabels([]string{"no-release"})

	require.NoError(t, err)
	require.Equal(t, SizeNoRelease, size)
}

func TestSizeFromLabelsIgnoresUnrelatedLabelsAlongsideOneReleaseLabel(t *testing.T) {
	size, err := SizeFromLabels([]string{"documentation", "minor", "enhancement"})

	require.NoError(t, err)
	require.Equal(t, SizeMinor, size)
}

func TestSizeFromLabelsRejectsNoLabels(t *testing.T) {
	_, err := SizeFromLabels([]string{})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoReleaseLabel))
	require.Equal(t, "no release label: add exactly one of major, minor, patch, no-release", err.Error())
}

func TestSizeFromLabelsRejectsOnlyUnrelatedLabels(t *testing.T) {
	_, err := SizeFromLabels([]string{"bug", "documentation"})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoReleaseLabel))

	var labelError *LabelError
	require.True(t, errors.As(err, &labelError))
	require.Empty(t, labelError.Found)
	require.Contains(t, err.Error(), "no release label")
}

func TestSizeFromLabelsRejectsTwoReleaseLabels(t *testing.T) {
	_, err := SizeFromLabels([]string{"patch", "major"})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSeveralReleaseLabels))

	var labelError *LabelError
	require.True(t, errors.As(err, &labelError))
	require.Equal(t, []Size{SizeMajor, SizePatch}, labelError.Found)
	require.Equal(t, "more than one release label: major, patch; keep exactly one of major, minor, patch, no-release", err.Error())
}

func TestSizeFromLabelsTwoReleaseLabelsIsNotNoReleaseLabel(t *testing.T) {
	_, err := SizeFromLabels([]string{"minor", "no-release"})

	require.Error(t, err)
	require.False(t, errors.Is(err, ErrNoReleaseLabel))
}
