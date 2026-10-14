package release

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersionReadsPlainVersion(t *testing.T) {
	version, err := ParseVersion("v1.2.3")

	require.NoError(t, err)
	require.Equal(t, Version{Major: 1, Minor: 2, Patch: 3}, version)
}

func TestParseVersionRejectsPreRelease(t *testing.T) {
	_, err := ParseVersion("v0.9.0-rc1")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotAVersion))
}

func TestParseVersionRejectsMissingPrefix(t *testing.T) {
	_, err := ParseVersion("0.9.0")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotAVersion))
}

func TestParseVersionRejectsTwoPartVersion(t *testing.T) {
	_, err := ParseVersion("v1.2")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotAVersion))
}

func TestParseVersionRejectsLeadingZero(t *testing.T) {
	_, err := ParseVersion("v1.02.3")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotAVersion))
}

func TestVersionStringWritesTag(t *testing.T) {
	version := Version{Major: 0, Minor: 7, Patch: 1}

	require.Equal(t, "v0.7.1", version.String())
}

func TestLatestIgnoresTagsThatAreNotPlainVersions(t *testing.T) {
	latest, found := Latest([]string{"v0.8.0", "v0.9.0-rc1", "docs-1", "0.9.0", "v1.2"})

	require.True(t, found)
	require.Equal(t, Version{Major: 0, Minor: 8, Patch: 0}, latest)
}

func TestLatestComparesMinorNumbersNumerically(t *testing.T) {
	latest, found := Latest([]string{"v0.10.0", "v0.9.0"})

	require.True(t, found)
	require.Equal(t, Version{Major: 0, Minor: 10, Patch: 0}, latest)
}

func TestLatestFindsHighestVersionRegardlessOfOrder(t *testing.T) {
	latest, found := Latest([]string{"v0.2.0", "v1.0.1", "v0.9.9", "v1.0.0"})

	require.True(t, found)
	require.Equal(t, Version{Major: 1, Minor: 0, Patch: 1}, latest)
}

func TestLatestReportsNotFoundWhenNoTagIsAVersion(t *testing.T) {
	_, found := Latest([]string{"docs-1", "v0.9.0-rc1"})

	require.False(t, found)
}

func TestNextPatchFromV070GivesV071(t *testing.T) {
	next, ok, err := Next([]string{"v0.7.0"}, SizePatch)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v0.7.1", next.String())
}

func TestNextMinorFromV070GivesV080(t *testing.T) {
	next, ok, err := Next([]string{"v0.7.0"}, SizeMinor)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v0.8.0", next.String())
}

func TestNextMajorFromV070GivesV100(t *testing.T) {
	next, ok, err := Next([]string{"v0.7.0"}, SizeMajor)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v1.0.0", next.String())
}

func TestNextMajorAbove1AdvancesMajorNumber(t *testing.T) {
	next, ok, err := Next([]string{"v1.2.3"}, SizeMajor)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v2.0.0", next.String())
}

func TestNextMinorResetsPatch(t *testing.T) {
	next, ok, err := Next([]string{"v1.2.3"}, SizeMinor)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v1.3.0", next.String())
}

func TestNextCountsFromHighestVersionIgnoringOtherTags(t *testing.T) {
	next, ok, err := Next([]string{"v0.9.0", "v0.10.0-rc1", "v0.10.0", "docs-1"}, SizePatch)

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "v0.10.1", next.String())
}

func TestNextNoReleaseGivesNoVersion(t *testing.T) {
	_, ok, err := Next([]string{"v0.7.0"}, SizeNoRelease)

	require.NoError(t, err)
	require.False(t, ok)
}

func TestNextNoReleaseGivesNoVersionEvenWithoutVersionTags(t *testing.T) {
	_, ok, err := Next([]string{}, SizeNoRelease)

	require.NoError(t, err)
	require.False(t, ok)
}

func TestNextErrorsWhenNoTagIsAVersion(t *testing.T) {
	_, ok, err := Next([]string{"docs-1", "v0.9.0-rc1"}, SizePatch)

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoVersionTags))
	require.False(t, ok)
}

func TestNextErrorsWhenThereAreNoTags(t *testing.T) {
	_, _, err := Next(nil, SizeMinor)

	require.True(t, errors.Is(err, ErrNoVersionTags))
}

func TestNextErrorsForUnknownSize(t *testing.T) {
	_, ok, err := Next([]string{"v0.7.0"}, Size("huge"))

	require.Error(t, err)
	require.False(t, ok)
}

func TestBumpReportsMajor(t *testing.T) {
	size, err := Bump("v0.7.3", "v1.0.0")

	require.NoError(t, err)
	require.Equal(t, SizeMajor, size)
}

func TestBumpReportsMinor(t *testing.T) {
	size, err := Bump("v1.2.3", "v1.3.0")

	require.NoError(t, err)
	require.Equal(t, SizeMinor, size)
}

func TestBumpReportsPatch(t *testing.T) {
	size, err := Bump("v1.2.3", "v1.2.4")

	require.NoError(t, err)
	require.Equal(t, SizePatch, size)
}

func TestBumpErrorsForNonVersionFromTag(t *testing.T) {
	_, err := Bump("docs-1", "v1.2.4")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotAVersion))
}

func TestBumpErrorsForNonVersionToTag(t *testing.T) {
	_, err := Bump("v1.2.3", "v1.3.0-rc1")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotAVersion))
}

func TestBumpErrorsWhenToIsEarlierThanFrom(t *testing.T) {
	_, err := Bump("v1.2.3", "v1.2.2")

	require.Error(t, err)
	require.Equal(t, "v1.2.2 is not later than v1.2.3", err.Error())
}

func TestBumpErrorsWhenToEqualsFrom(t *testing.T) {
	_, err := Bump("v1.2.3", "v1.2.3")

	require.Error(t, err)
	require.Contains(t, err.Error(), "is not later than")
}
