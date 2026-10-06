package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// configDir is the configuration this example applies
const configDir = "./config"

// testStateKey is the 32 byte key the tests encrypt the sensitive values in
// state with, fixed so every test run encrypts the same way
var testStateKey = []byte("0123456789abcdef0123456789abcdef")

// externalPlugin is the external plugin binary, built once for the package's
// tests by TestMain
var externalPlugin string

// TestMain builds the external plugin into a temporary directory, so the
// tests never depend on a binary built by hand
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "xcl-example-plugin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create build directory: %s\n", err)
		os.Exit(1)
	}

	externalPlugin = filepath.Join(dir, "external")

	build := exec.Command("go", "build", "-o", externalPlugin, "./external")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to build the external plugin: %s\n%s", err, output)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

// declaredResourceIDs is every resource the example configuration declares,
// sorted
var declaredResourceIDs = []string{
	"module.analytics",
	"module.analytics.output.location",
	"module.analytics.resource.postgres.analytics",
	"module.analytics.variable.db_username",
	"output.web_database",
	"resource.app.web",
	"resource.ingress.web",
	"resource.postgres.main",
	"resource.postgres.replica",
	"resource.redis.cache",
	"variable.db_password",
	"variable.db_username",
}

func TestPluginExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, t.TempDir(), testStateKey)
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

func TestPluginExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), "./does-not-exist", externalPlugin, t.TempDir(), testStateKey)
	require.Error(t, err)
}

// TestPluginExampleFailsWithoutExternalPlugin asserts the example tells the
// reader how to build the external plugin when it is missing
func TestPluginExampleFailsWithoutExternalPlugin(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, "./does-not-exist", t.TempDir(), testStateKey)
	require.Error(t, err)
	require.Contains(t, err.Error(), ", build it with `make build` in example/plugin")
}

func TestPluginExamplePrintsNoResourcesRemaining(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, t.TempDir(), testStateKey)
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Destroyed\n  0 resources remaining\n")
}

// TestPluginExamplePrintsPublishedTotal asserts every published value is
// reachable at once, the total counting the module's output alongside the
// root's
func TestPluginExamplePrintsPublishedTotal(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, t.TempDir(), testStateKey)
	require.NoError(t, err)

	require.Contains(t, out.String(), "  2 published in total\n")
}

// TestPluginExamplePrintsNoSecret asserts neither the password configured
// through the variable's default nor the literal one in the module appears
// in the example's report or the events rendered by the pretty printer
func TestPluginExamplePrintsNoSecret(t *testing.T) {
	r := registry.NewPluginRegistry()
	out := &bytes.Buffer{}
	rendered := &bytes.Buffer{}

	_, err := run(out, prettylog.Handler(rendered, slog.LevelDebug, r), r, configDir, externalPlugin, t.TempDir(), testStateKey)
	require.NoError(t, err)

	for _, written := range []string{out.String(), rendered.String()} {
		require.NotContains(t, written, variablePassword)
		require.NotContains(t, written, modulePassword)
	}
}

// The passwords the example configuration holds: the default of the
// db_password variable and the literal one in the analytics module
const (
	variablePassword = "pg-s3cret-example"
	modulePassword   = "pg-an4lytics-example"
)
