package e2e_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/types"
)

// encodeCreatedConfiguration applies the kube configuration and returns the
// configuration text of every entity a create success event carries, encoded
// from the event's data the way an event receiver renders it: with the
// registry that typed it and the values the provider filled in. A variable or
// output is never written as configuration, so those are skipped
func encodeCreatedConfiguration(t *testing.T) string {
	t.Helper()

	recorder := &testutil.EventRecorder{}
	c := newKubeConfig(t, recorder.Record, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(kubeConfigDir))

	encoded := []string{}
	for _, e := range recorder.Events() {
		if e.Operation != events.OperationCreate || e.Phase != events.PhaseSuccess || len(e.Data) == 0 {
			continue
		}

		text, err := c.EncodeSavedEntity(e.Data, xcl.IncludeComputed())
		if errors.Is(err, xcl.ErrNotEncodable) {
			continue
		}
		require.NoError(t, err, "encoding the data of %s", e.ResourceID)

		encoded = append(encoded, string(text))
	}

	require.NotEmpty(t, encoded, "no create success event carried encodable data, so the check proves nothing")

	return strings.Join(encoded, "\n")
}

// TestEncodeSavedEntityWritesCreatedConfiguration asserts every registered
// type the apply creates is encoded from its event data back to
// configuration, nested blocks and all
func TestEncodeSavedEntityWritesCreatedConfiguration(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	encoded := encodeCreatedConfiguration(t)

	require.Contains(t, encoded, `config_map "api" {`)
	require.Contains(t, encoded, `deployment "api" {`)
	require.Contains(t, encoded, `service "api" {`)
	require.Contains(t, encoded, `ingress "api" {`)

	// the nested blocks of the deployment are written too, and the formatter
	// aligns the equals signs, so the gap before one is matched rather than
	// written out
	require.Contains(t, encoded, "container {")
	require.Regexp(t, `container_port\s+= 8080`, encoded)
	require.Regexp(t, `target_port\s+= 8080`, encoded)
}

// TestEncodeSavedEntityMasksSecret asserts the configuration encoded from the
// event data of a created secret shows the sensitive marker in place of the
// password
func TestEncodeSavedEntityMasksSecret(t *testing.T) {
	t.Setenv("DB_PASSWORD", testPassword)

	encoded := encodeCreatedConfiguration(t)

	require.Contains(t, encoded, `secret "db" {`)
	require.NotContains(t, encoded, testPassword)
	require.Contains(t, encoded, types.SensitiveMarker)
}
