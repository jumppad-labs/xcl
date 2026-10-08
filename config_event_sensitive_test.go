package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

const eventSensitiveSecretID = "resource.secret.literal"

// applySensitiveFixtureWithEventData applies the sensitive fixture with an
// event handler recording every event at the given event data level, and
// returns the recorder and the configuration that was applied
func applySensitiveFixtureWithEventData(t *testing.T, level EventDataLevel) (*eventRecorder, *Config) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c, err := NewConfig(
		WithType(&registered.Secret{}, "resource", registered.TypeSecret),
		WithType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer),
		WithStateStore(store),
		WithEventHandler(recorder.Record),
		WithEventData(level),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive/basic/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return recorder, c
}

func TestRawEventDataShowsOnlyTheSensitiveMarker(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataRaw)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestProcessedEventDataShowsOnlyTheSensitiveMarker(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestProcessedEventDataShowsOnlyTheMarkerForAReferencedSensitiveValue(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, "resource.secret_consumer.reference", "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestEncodeSavedEntityOfProcessedEventDataWritesTheMarker(t *testing.T) {
	recorder, c := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	out, err := c.EncodeSavedEntity(succeeded.Data)
	require.NoError(t, err)

	require.Contains(t, string(out), `password = "(sensitive)"`)
	require.NotContains(t, string(out), "from-literal")
}

func TestEncodeSavedEntityOfProcessedEventDataWritesTheMarkerWhenRevealing(t *testing.T) {
	recorder, c := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	out, err := c.EncodeSavedEntity(succeeded.Data, RevealSensitive())
	require.NoError(t, err)

	require.Contains(t, string(out), `password = "(sensitive)"`)
	require.NotContains(t, string(out), "from-literal")
}
