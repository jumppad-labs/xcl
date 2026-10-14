package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunLabelsPrintsSize(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"labels", "bug", "minor"}, &stdout, &stderr)

	require.Equal(t, 0, status)
	require.Equal(t, "minor\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunLabelsWithNoReleaseLabelFails(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"labels", "bug"}, &stdout, &stderr)

	require.Equal(t, 1, status)
	require.Empty(t, stdout.String())
	require.Equal(t, "no release label: add exactly one of major, minor, patch, no-release\n", stderr.String())
}

func TestRunLabelsWithTwoReleaseLabelsFails(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"labels", "major", "patch"}, &stdout, &stderr)

	require.Equal(t, 1, status)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "more than one release label: major, patch")
}

func TestRunNextPrintsNextTag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"next", "--size", "minor", "v0.7.0", "v0.6.2", "docs-1"}, &stdout, &stderr)

	require.Equal(t, 0, status)
	require.Equal(t, "v0.8.0\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunNextNoReleasePrintsNothing(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"next", "--size", "no-release", "v0.7.0"}, &stdout, &stderr)

	require.Equal(t, 0, status)
	require.Empty(t, stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunNextWithInvalidSizeIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"next", "--size", "huge", "v0.7.0"}, &stdout, &stderr)

	require.Equal(t, 2, status)
	require.Empty(t, stdout.String())
	require.Equal(t, "--size \"huge\" is not a release size\n", stderr.String())
}

func TestRunNextWithoutVersionTagsFails(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"next", "--size", "patch", "docs-1"}, &stdout, &stderr)

	require.Equal(t, 1, status)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "no version tags")
}

func TestRunBumpPrintsSize(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"bump", "v1.2.3", "v2.0.0"}, &stdout, &stderr)

	require.Equal(t, 0, status)
	require.Equal(t, "major\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunBumpWithWrongArgumentCountIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"bump", "v1.2.3"}, &stdout, &stderr)

	require.Equal(t, 2, status)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "usage:")
}

func TestRunWithNoArgumentsIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{}, &stdout, &stderr)

	require.Equal(t, 2, status)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "usage:")
}

func TestRunWithUnknownSubcommandIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	status := run([]string{"publish"}, &stdout, &stderr)

	require.Equal(t, 2, status)
	require.Contains(t, stderr.String(), "usage:")
}
