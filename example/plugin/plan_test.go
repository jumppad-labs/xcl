package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeChangedConfig writes a copy of the example's configuration to a
// temporary directory with the template's source changed, and returns the
// directory
func writeChangedConfig(t *testing.T) string {
	t.Helper()

	original, err := os.ReadFile("./config/main.xcl")
	require.NoError(t, err)

	changed := strings.Replace(string(original), "Welcome to the", "Hello from the", 1)
	require.NotEqual(t, string(original), changed, "the template source was not found to change")

	dir := t.TempDir()
	err = os.WriteFile(filepath.Join(dir, "main.xcl"), []byte(changed), 0o644)
	require.NoError(t, err)

	return dir
}

func TestPlanWithTheAppliedConfigurationReportsNoChanges(t *testing.T) {
	stateDir := t.TempDir()
	applyExampleWithState(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, c, "./config")
	require.NoError(t, err)

	require.Equal(t, "Diff: no changes, 3 unchanged.\n", out.String())
}

func TestPlanReportsAChangedTemplateAsAnUpdate(t *testing.T) {
	stateDir := t.TempDir()
	applyExampleWithState(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, c, writeChangedConfig(t))
	require.NoError(t, err)

	// out is not a terminal, so the diff is plain
	printed := out.String()
	require.Contains(t, printed, "# template.welcome will be updated")
	require.Contains(t, printed, `~ template "welcome" {`)
	require.Contains(t, printed, `"Welcome to the {{network}} network.`)
	require.Contains(t, printed, `-> "Hello from the {{network}} network.`)
	require.Contains(t, printed, "Diff: 0 to create, 1 to update, 0 to replace, 0 to delete, 2 unchanged.")
}

func TestPlanLeavesTheSavedStateUnchanged(t *testing.T) {
	stateDir := t.TempDir()
	applyExampleWithState(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	err = plan(io.Discard, c, writeChangedConfig(t))
	require.NoError(t, err)

	// applying the original configuration again changes nothing, so the plan
	// saved nothing of the changed one
	again, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, again, "./config")
	require.NoError(t, err)

	require.Equal(t, "Diff: no changes, 3 unchanged.\n", out.String())
}

func TestPlanOfSubnetChangeReplacesNetworkAndContainer(t *testing.T) {
	stateDir := t.TempDir()
	applyExampleWithState(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, c, "./config-subnet")
	require.NoError(t, err)

	// out is not a terminal, so the diff is plain
	printed := out.String()
	require.Contains(t, printed, "# docker.network.app will be replaced, it cannot be updated in place")
	require.Contains(t, printed, `-/+ docker "network" "app" {`)
	require.Contains(t, printed, `~ subnet = "10.42.0.0/24" -> "10.42.0.0/23"`)
	require.Contains(t, printed, "# docker.container.web will be replaced because docker.network.app is replaced")
	require.Contains(t, printed, `-/+ docker "container" "web"`)
	require.Contains(t, printed, "# template.welcome will be updated")
	require.Contains(t, printed, `~ template "welcome" {`)
	require.Contains(t, printed, "Diff: 0 to create, 1 to update, 2 to replace, 0 to delete, 0 unchanged.")
}

func TestRunPlanWithoutAPathFails(t *testing.T) {
	stderr := &bytes.Buffer{}

	code := run([]string{"plan"}, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "plan needs the path of the configuration to compare")
}
