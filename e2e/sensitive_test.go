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
	"github.com/jumppad-labs/xcl/e2e/fixtures/kube"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// sensitivePlaintextStateWarning is the warning xcl emits when it writes
// sensitive values to state in plain text
const sensitivePlaintextStateWarning = "sensitive values are stored unencrypted in state; use xcl.WithStateMask to encrypt them"

// sensitiveSavedRecords reads the records the state file in stateDir holds
func sensitiveSavedRecords(t *testing.T, stateDir string) []json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(stateDir, state.StateFileName))
	require.NoError(t, err)

	records := []json.RawMessage{}
	require.NoError(t, json.Unmarshal(data, &records))

	return records
}

// sensitiveApplyAndDestroy applies the kube configuration and then destroys
// everything it applied, recording every event either reports. stateKey
// encrypts the sensitive values in state, nil leaves them in plain text
func sensitiveApplyAndDestroy(t *testing.T, stateKey []byte) []xcl.Event {
	t.Helper()

	recorder := &testutil.EventRecorder{}
	c := newKubeConfig(t, registry.NewPluginRegistry(), recorder.Record, t.TempDir(), stateKey)

	require.NoError(t, c.Apply(kubeConfigDir))
	require.NoError(t, c.Destroy())

	return recorder.Events()
}

// TestSavedEntityEncodesAsAppliedEntity asserts the configuration text of a
// saved record is identical to the text of the entity it was written from,
// for every record the apply saved. A variable or output is never written as
// configuration, so those records are skipped
func TestSavedEntityEncodesAsAppliedEntity(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	r := registry.NewPluginRegistry()
	stateDir := t.TempDir()
	c := newKubeConfig(t, r, nil, stateDir, testStateKey)

	require.NoError(t, c.Apply(kubeConfigDir))

	records := sensitiveSavedRecords(t, stateDir)
	require.NotEmpty(t, records)

	compared := 0

	for _, record := range records {
		fromState, err := xcl.EncodeSavedEntity(r, record)
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

// TestNoPlainStateWarningWithKey asserts that with a state mask xcl reports no
// warning that state holds sensitive values in plain text, though the
// configuration holds a secret
func TestNoPlainStateWarningWithKey(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	recorded := sensitiveApplyAndDestroy(t, testStateKey)
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.NotEqual(t, sensitivePlaintextStateWarning, e.Meta[events.KeyMessage], "unexpected plain text warning: %+v", e)
	}
}

// TestPlainStateWarningWithoutKey asserts that without a state mask xcl warns,
// once, that the secret is stored in plain text
func TestPlainStateWarningWithoutKey(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	recorded := sensitiveApplyAndDestroy(t, nil)

	warnings := []xcl.Event{}
	for _, e := range recorded {
		if e.Meta[events.KeyMessage] == sensitivePlaintextStateWarning && e.Operation == events.OperationApply {
			warnings = append(warnings, e)
		}
	}

	require.Len(t, warnings, 1)
	require.Equal(t, events.SourceCore, warnings[0].Source)
	require.Equal(t, events.PhaseLog, warnings[0].Phase)
	require.Equal(t, events.LevelWarn, warnings[0].Meta[events.KeyLevel])
}

// TestEnvFunctionReadsSensitiveValue asserts the secret's data is read with
// env when the configuration is parsed, and that Reveal returns the real value
func TestEnvFunctionReadsSensitiveValue(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	c := newKubeConfig(t, registry.NewPluginRegistry(), nil, t.TempDir(), testStateKey)
	require.NoError(t, c.Apply(kubeConfigDir))

	entity, err := testutil.EntityByID(c.Entities(), "secret.db")
	require.NoError(t, err)

	secret, ok := entity.(*kube.Secret)
	require.True(t, ok)

	require.Equal(t, testPassword, secret.Data.Reveal()["password"])
}

// TestNestedBlockReferencesSecretByNameAndKey asserts the container reads the
// password through a secret_key_ref, so the deployment holds only the secret's
// name and the key, never the password
func TestNestedBlockReferencesSecretByNameAndKey(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	c := newKubeConfig(t, registry.NewPluginRegistry(), nil, t.TempDir(), testStateKey)
	require.NoError(t, c.Apply(kubeConfigDir))

	entity, err := testutil.EntityByID(c.Entities(), "deployment.api")
	require.NoError(t, err)

	d, ok := entity.(*kube.Deployment)
	require.True(t, ok)

	env := d.Containers[0].Env[2]
	require.Equal(t, "DB_PASSWORD", env.Name)
	require.Empty(t, env.Value)
	require.NotNil(t, env.ValueFrom)
	require.NotNil(t, env.ValueFrom.SecretKeyRef)
	require.Equal(t, "db", env.ValueFrom.SecretKeyRef.Name)
	require.Equal(t, "password", env.ValueFrom.SecretKeyRef.Key)
}

// TestStateHoldsNoSecretWithKey asserts that with a state key the password is
// encrypted in the state the apply saved, the sensitive values written as
// masked envelopes
func TestStateHoldsNoSecretWithKey(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	stateDir := t.TempDir()
	c := newKubeConfig(t, registry.NewPluginRegistry(), nil, stateDir, testStateKey)
	require.NoError(t, c.Apply(kubeConfigDir))

	records := sensitiveSavedRecords(t, stateDir)
	require.NotEmpty(t, records, "the apply saved no state, so the check proves nothing")

	all := ""
	for _, record := range records {
		all += string(record)
	}

	require.NotContains(t, all, testPassword)
	require.Contains(t, all, "xcl_masked")
}

// TestEventsHoldNoSecret asserts no event an apply and destroy report carries
// the password, in its data or anywhere else
func TestEventsHoldNoSecret(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	recorded := sensitiveApplyAndDestroy(t, testStateKey)
	require.NotEmpty(t, recorded)

	withData := 0
	for _, e := range recorded {
		require.NotContains(t, string(e.Data), testPassword, "event data holds the password: %s %s %s", e.Operation, e.Phase, e.ResourceID)
		require.NotContains(t, fmt.Sprintf("%+v", e), testPassword, "event holds the password: %s %s %s", e.Operation, e.Phase, e.ResourceID)

		if len(e.Data) > 0 {
			withData++
		}
	}

	require.NotZero(t, withData, "no event carried data, so the check proves nothing")
}

// TestStandardStreamsHoldNoSecret asserts xcl itself writes nothing holding
// the password to the process's standard streams while it applies and
// destroys a configuration with a secret
func TestStandardStreamsHoldNoSecret(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	recorder := &testutil.EventRecorder{}
	c := newKubeConfig(t, registry.NewPluginRegistry(), recorder.Record, t.TempDir(), testStateKey)

	var applyErr, destroyErr error
	captured := testutil.CaptureStandardStreams(t, func() {
		applyErr = c.Apply(kubeConfigDir)
		destroyErr = c.Destroy()
	})

	require.NoError(t, applyErr)
	require.NoError(t, destroyErr)
	require.NotContains(t, captured.Stdout, testPassword)
	require.NotContains(t, captured.Stderr, testPassword)
}
