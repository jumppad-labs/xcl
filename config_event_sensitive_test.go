package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

const eventSensitiveSecretID = "resource.secret.literal"

// applySensitiveFixtureWithEventData applies the sensitive fixture with an
// event handler recording every event at the given event data level, and
// returns the recorder and the registry the configuration was applied with
func applySensitiveFixtureWithEventData(t *testing.T, level EventDataLevel) (*eventRecorder, *registry.PluginRegistry) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()

	err := reg.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret)
	require.NoError(t, err)

	err = reg.RegisterType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer)
	require.NoError(t, err)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c, err := NewConfig(
		WithPluginRegistry(reg),
		WithStateStore(store),
		WithEventHandler(recorder.handle),
		WithEventData(level),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive/basic/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return recorder, reg
}

func TestRawEventDataShowsOnlyTheSensitiveMarker(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataRaw)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":"(sensitive)"`)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestProcessedEventDataShowsOnlyTheSensitiveMarker(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":"(sensitive)"`)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestProcessedEventDataShowsOnlyTheMarkerForAReferencedSensitiveValue(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, "resource.secret_consumer.reference", "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":"(sensitive)"`)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestEncodeSavedEntityOfProcessedEventDataWritesTheMarker(t *testing.T) {
	recorder, reg := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	out, err := EncodeSavedEntity(reg, succeeded.Data)
	require.NoError(t, err)

	require.Contains(t, string(out), `password = "(sensitive)"`)
	require.NotContains(t, string(out), "from-literal")
}

func TestEncodeSavedEntityOfProcessedEventDataWritesTheMarkerWhenRevealing(t *testing.T) {
	recorder, reg := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	out, err := EncodeSavedEntity(reg, succeeded.Data, RevealSensitive())
	require.NoError(t, err)

	require.Contains(t, string(out), `password = "(sensitive)"`)
	require.NotContains(t, string(out), "from-literal")
}
