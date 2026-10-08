package e2e_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// The passwords the plugin configuration holds: the default of the
// db_password variable and the literal one in the analytics module
const (
	pluginStateVariablePassword = "pg-s3cret-example"
	pluginStateModulePassword   = "pg-an4lytics-example"
)

// pluginStateSavedRecords reads the records the state file in stateDir holds
func pluginStateSavedRecords(t *testing.T, stateDir string) []json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(stateDir, state.StateFileName))
	require.NoError(t, err)

	records := []json.RawMessage{}
	require.NoError(t, json.Unmarshal(data, &records))

	return records
}

// pluginStateFiles reads every file in the state directory
func pluginStateFiles(t *testing.T, stateDir string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(stateDir)
	require.NoError(t, err)

	files := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(stateDir, entry.Name()))
		require.NoError(t, err)

		files[entry.Name()] = string(data)
	}

	return files
}

// TestEncodeSavedEntityWritesExternalPluginTypes asserts a type provided by
// the external plugin binary, which xcl holds as a type generated from the
// plugin's schema, is encoded from its event data as configuration just as an
// in-process type is
func TestEncodeSavedEntityWritesExternalPluginTypes(t *testing.T) {
	encoded := pluginEventEncodedCreates(t)

	require.Contains(t, encoded["resource.app.web"], `resource "app" "web" {`)
	require.Contains(t, encoded["resource.ingress.web"], `resource "ingress" "web" {`)
}

// TestPluginSavedEntityEncodesAsAppliedEntity asserts the configuration text
// of a saved record is identical to the text of the entity it was written
// from, for every record the apply saved through the providers. A variable,
// output or module is never written as configuration, so those records are
// skipped
func TestPluginSavedEntityEncodesAsAppliedEntity(t *testing.T) {
	stateDir := t.TempDir()
	c := newPluginConfig(t, nil, stateDir, testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))

	records := pluginStateSavedRecords(t, stateDir)
	require.NotEmpty(t, records)

	compared := 0

	for _, record := range records {
		fromState, err := c.EncodeSavedEntity(record)
		if errors.Is(err, xcl.ErrNotEncodable) {
			continue
		}
		require.NoError(t, err)

		id := testutil.SavedID(t, record)

		entity, err := testutil.EntityByID(c.Entities(), id)
		require.NoError(t, err)

		fromEntity, err := xcl.EncodeEntity(entity)
		require.NoError(t, err)

		require.Equal(t, string(fromEntity), string(fromState), "the saved record and the entity disagree for %s", id)

		compared++
	}

	require.NotZero(t, compared, "no record was encodable, so the comparison proves nothing")
}

// TestPluginEntitiesEncodeOrAreRefusedAsNotEncodable asserts every entity an
// apply with plugins holds either converts to configuration text or is
// refused as not encodable, which is what a variable, output or module is. No
// other failure is allowed
func TestPluginEntitiesEncodeOrAreRefusedAsNotEncodable(t *testing.T) {
	c := newPluginConfig(t, nil, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))

	applied := c.Entities()
	require.NotEmpty(t, applied)

	converted := 0

	for _, entity := range applied {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		text, err := xcl.EncodeEntity(entity)
		if err != nil {
			require.ErrorIs(t, err, xcl.ErrNotEncodable, "%s failed for another reason", meta.ID)
			require.Empty(t, text, "%s returned text with its failure", meta.ID)
			continue
		}

		require.NotEmpty(t, text, "%s converted to nothing", meta.ID)

		converted++
	}

	require.NotZero(t, converted, "nothing converted, so the check proves nothing")
}

// TestPluginStateHoldsNoSecretWithKey asserts that with a state key neither
// password is in any file of the state directory once the apply has saved
// it, the sensitive values written as masked envelopes
func TestPluginStateHoldsNoSecretWithKey(t *testing.T) {
	stateDir := t.TempDir()
	c := newPluginConfig(t, nil, stateDir, testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))

	files := pluginStateFiles(t, stateDir)
	require.NotEmpty(t, files, "the apply left no state, so the check proves nothing")

	all := ""
	for name, data := range files {
		require.NotContains(t, data, pluginStateVariablePassword, "%s holds the variable's password", name)
		require.NotContains(t, data, pluginStateModulePassword, "%s holds the module's password", name)

		all += data
	}

	require.Contains(t, all, "xcl_masked")
}

// TestPluginEventsHoldNoSecret asserts no event an apply and destroy with
// plugins report carries either password, neither in its data nor anywhere
// else in the event
func TestPluginEventsHoldNoSecret(t *testing.T) {
	recorded := pluginEventLifecycle(t)
	require.NotEmpty(t, recorded)

	withData := 0
	for _, e := range recorded {
		require.NotContains(t, string(e.Data), pluginStateVariablePassword, "event data holds the variable's password: %s %s %s", e.Operation, e.Phase, e.ResourceID)
		require.NotContains(t, string(e.Data), pluginStateModulePassword, "event data holds the module's password: %s %s %s", e.Operation, e.Phase, e.ResourceID)

		formatted := fmt.Sprintf("%+v", e)
		require.NotContains(t, formatted, pluginStateVariablePassword, "event holds the variable's password: %s %s %s", e.Operation, e.Phase, e.ResourceID)
		require.NotContains(t, formatted, pluginStateModulePassword, "event holds the module's password: %s %s %s", e.Operation, e.Phase, e.ResourceID)

		if len(e.Data) > 0 {
			withData++
		}
	}

	require.NotZero(t, withData, "no event carried data, so the check proves nothing")
}

// TestPluginPlainStateWarningWithoutKey asserts an apply and destroy with
// plugins and no state key report the plain text warning from core once for
// the apply and once for the destroy
func TestPluginPlainStateWarningWithoutKey(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	c := newPluginConfig(t, recorder.Record, t.TempDir(), nil)

	require.NoError(t, c.Apply(pluginConfigDir))
	require.NoError(t, c.Destroy())

	applyWarnings := []xcl.Event{}
	destroyWarnings := []xcl.Event{}
	for _, e := range recorder.Events() {
		if e.Meta[events.KeyMessage] != sensitivePlaintextStateWarning {
			continue
		}

		require.Equal(t, events.SourceCore, e.Source)
		require.Equal(t, events.PhaseLog, e.Phase)
		require.Equal(t, events.LevelWarn, e.Meta[events.KeyLevel])

		switch e.Operation {
		case events.OperationApply:
			applyWarnings = append(applyWarnings, e)
		case events.OperationDestroy:
			destroyWarnings = append(destroyWarnings, e)
		default:
			require.Failf(t, "unexpected warning", "the plain text warning was reported for %s", e.Operation)
		}
	}

	require.Len(t, applyWarnings, 1)
	require.Len(t, destroyWarnings, 1)
}
