package main

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
)

// configDir is the configuration this example parses
const configDir = "./config"

// testStateKey is the 32 byte key the tests encrypt the sensitive values in
// state with, fixed so every test run encrypts the same way
var testStateKey = []byte("0123456789abcdef0123456789abcdef")

// declaredResourceIDs is every resource the configuration declares, sorted
var declaredResourceIDs = []string{
	"config_map.api",
	"deployment.api",
	"ingress.api",
	"output.api_url",
	"secret.db",
	"service.api",
	"variable.image_tag",
	"variable.replicas",
}

func TestConfigOnlyExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, t.TempDir(), testStateKey)
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

// TestConfigOnlyExamplePrintsNestedBlocks asserts the printed deployment
// walks the blocks nested inside it
func TestConfigOnlyExamplePrintsNestedBlocks(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, t.TempDir(), testStateKey)
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Deployments\n")
	require.Contains(t, out.String(), "  deployment.api replicas=3\n")
	require.Contains(t, out.String(), "    container api image=ghcr.io/example/api:1.2.0\n")
	require.Contains(t, out.String(), "      port http container_port=8080\n")
	require.Contains(t, out.String(), "      env DB_HOST=postgres.default.svc\n")
	require.Contains(t, out.String(), "      limits cpu=500m memory=512Mi\n")
	require.Contains(t, out.String(), "      requests cpu=100m memory=128Mi\n")
	require.Contains(t, out.String(), "      volume_mount config path=/etc/api\n")
	require.Contains(t, out.String(), "    volume config config_map=config_map.api\n")
	require.Contains(t, out.String(), "    container proxy image=ghcr.io/example/proxy:0.4.1\n")
}

// TestConfigOnlyExamplePrintsLinkedResources asserts the printed service and
// ingress hold the values they read from the blocks they reference
func TestConfigOnlyExamplePrintsLinkedResources(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, t.TempDir(), testStateKey)
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Service\n")
	require.Contains(t, out.String(), "  service.api deployment=deployment.api port=80 target_port=8080\n")
	require.Contains(t, out.String(), "## Ingress\n")
	require.Contains(t, out.String(), "  ingress.api host=api.example.com\n")
	require.Contains(t, out.String(), "    rule path=/ service=service.api port=80\n")
}

func TestConfigOnlyExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), "./does-not-exist", t.TempDir(), testStateKey)
	require.Error(t, err)
}

func TestConfigOnlyExamplePrintsNoResourcesRemaining(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, t.TempDir(), testStateKey)
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Destroyed\n  0 resources remaining\n")
}

// testPassword is the database password the tests set through DB_PASSWORD
const testPassword = "configonly-s3cret-4b7d2e"

// TestConfigOnlyExamplePrintsNoSecret asserts the password appears in nothing
// the example writes: not the report and not the rendered events
func TestConfigOnlyExamplePrintsNoSecret(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	r := registry.NewPluginRegistry()
	out := &bytes.Buffer{}
	rendered := &bytes.Buffer{}

	_, err := run(out, prettylog.Handler(rendered, slog.LevelDebug, r), r, configDir, t.TempDir(), testStateKey)

	require.NoError(t, err)
	require.NotContains(t, out.String(), testPassword)
	require.NotContains(t, rendered.String(), testPassword)
	require.Contains(t, rendered.String(), types.SensitiveMarker)
}
