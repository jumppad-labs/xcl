package e2e_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
)

// pluginEventEncodedCreates applies the plugin configuration and returns the
// configuration text of every entity a create success event carries, keyed by
// resource id, encoded from the event's data the way an event receiver
// renders it: with the registry that typed it and the values the providers
// filled in. A variable, output or module is never written as configuration,
// so those are skipped
func pluginEventEncodedCreates(t *testing.T) map[string]string {
	t.Helper()

	r := registry.NewPluginRegistry()
	recorder := &testutil.EventRecorder{}
	c := newPluginConfig(t, r, recorder.Record, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))

	encoded := map[string]string{}
	for _, e := range recorder.Events() {
		if e.Operation != events.OperationCreate || e.Phase != events.PhaseSuccess || len(e.Data) == 0 {
			continue
		}

		text, err := xcl.EncodeSavedEntity(r, e.Data, xcl.IncludeComputed())
		if errors.Is(err, xcl.ErrNotEncodable) {
			continue
		}
		require.NoError(t, err, "encoding the data of %s", e.ResourceID)

		encoded[e.ResourceID] = string(text)
	}

	require.NotEmpty(t, encoded, "no create success event carried encodable data, so the check proves nothing")

	return encoded
}

// pluginEventAllEncoded joins the configuration text of every created entity
func pluginEventAllEncoded(encoded map[string]string) string {
	all := []string{}
	for _, text := range encoded {
		all = append(all, text)
	}

	return strings.Join(all, "\n")
}

// TestEncodeSavedEntityWritesPluginConfigurationWithComputedValues asserts
// the event data of a resource a provider created encodes back to its
// configuration, holding the values that were configured, the nested block,
// and the connection string the provider filled in
func TestEncodeSavedEntityWritesPluginConfigurationWithComputedValues(t *testing.T) {
	encoded := pluginEventEncodedCreates(t)

	postgresMain, ok := encoded["resource.postgres.main"]
	require.True(t, ok, "the create success of resource.postgres.main carried no encodable data")

	require.Contains(t, postgresMain, `resource "postgres" "main" {`)

	// the formatter aligns the equals signs to the longest name in the block,
	// so the gap before one is matched rather than written out
	require.Regexp(t, `port\s+= 5432`, postgresMain)
	require.Contains(t, postgresMain, "timeouts {")
	require.Regexp(t, `connection_string\s+=\s+"postgres://admin@localhost:5432/main"`, postgresMain)
}

// TestEncodeSavedEntityWritesEveryCreatedPluginResource asserts every
// resource the providers create has its configuration encoded from its event
// data, including the one declared inside a module
func TestEncodeSavedEntityWritesEveryCreatedPluginResource(t *testing.T) {
	all := pluginEventAllEncoded(pluginEventEncodedCreates(t))

	require.Contains(t, all, `resource "postgres" "main" {`)
	require.Contains(t, all, `resource "postgres" "replica" {`)
	require.Contains(t, all, `resource "postgres" "analytics" {`)
	require.Contains(t, all, `resource "redis" "cache" {`)
	require.Contains(t, all, `resource "app" "web" {`)
	require.Contains(t, all, `resource "ingress" "web" {`)
}

// TestEncodeSavedEntityMasksPluginPasswords asserts the configuration
// encoded from the event data of the created databases shows the sensitive
// marker in place of either password
func TestEncodeSavedEntityMasksPluginPasswords(t *testing.T) {
	all := pluginEventAllEncoded(pluginEventEncodedCreates(t))

	require.NotContains(t, all, pluginStateVariablePassword)
	require.NotContains(t, all, pluginStateModulePassword)
	require.Contains(t, all, types.SensitiveMarker)
}
