package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordAppendsOneLinePerCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recording.log")

	require.NoError(t, record(path, "create", "resource.recording.x"))
	require.NoError(t, record(path, "create", "resource.recording.x"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	require.Len(t, lines, 2)

	for _, line := range lines {
		var r Record
		require.NoError(t, json.Unmarshal([]byte(line), &r), "line %q is not JSON", line)

		require.Equal(t, "create", r.Step)
		require.Equal(t, "resource.recording.x", r.Address)
		require.False(t, r.Time.IsZero())
	}
}

func TestRecordFailsWhenLogDirectoryMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "recording.log")

	err := record(path, "create", "resource.recording.x")

	require.Error(t, err)
	require.Contains(t, err.Error(), path)
	require.NoFileExists(t, path)
}
