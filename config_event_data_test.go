package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// applyEncodeFixtureWithEventData applies the encode fixture with a file state
// store and an event handler recording every event, at the given event data
// level. It returns the recorder, the path the state was written to, the
// registry the configuration was applied with and the applied configuration,
// so a test can compare what an event carried against both the saved record
// and the live entity for the same resource
func applyEncodeFixtureWithEventData(t *testing.T, level EventDataLevel) (*eventRecorder, string, *registry.PluginRegistry, *Config) {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := encodeRegistry(t)

	statePath := filepath.Join(t.TempDir(), "state.json")

	store, err := state.NewFileStateStore(statePath, pr)
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(store),
		WithEventHandler(recorder.handle),
		WithEventData(level),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/encode/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return recorder, statePath, pr, c
}

// applyEncodeFixtureWithDefaultEventData applies the encode fixture with an
// event handler and no WithEventData at all, which is how an application that
// has never heard of event data levels configures xcl
func applyEncodeFixtureWithDefaultEventData(t *testing.T) *eventRecorder {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := encodeRegistry(t)

	statePath := filepath.Join(t.TempDir(), "state.json")

	store, err := state.NewFileStateStore(statePath, pr)
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(store),
		WithEventHandler(recorder.handle),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/encode/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return recorder
}

// eventDataNetwork is the part of a serialized network resource these tests
// assert on, both fields are pointers so an absent field can be told apart
// from one that was written empty
type eventDataNetwork struct {
	ProviderID *string `json:"provider_id"`
	Meta       struct {
		Status *string `json:"status"`
	} `json:"meta"`
}

// eventDataSingleEvent returns the one event the recorder holds for the
// resource, operation and phase
func eventDataSingleEvent(t *testing.T, recorder *eventRecorder, id, operation, phase string) Event {
	t.Helper()

	found := recorder.find(id, operation, phase)
	require.Len(t, found, 1, "expected exactly one %s %s event for %s", operation, phase, id)

	return found[0]
}

// eventDataNetworkFromEvent unmarshals the resource an event carried
func eventDataNetworkFromEvent(t *testing.T, e Event) eventDataNetwork {
	t.Helper()

	require.NotEmpty(t, e.Data, "expected the event to carry the resource")

	network := eventDataNetwork{}

	err := json.Unmarshal(e.Data, &network)
	require.NoError(t, err)

	return network
}

// TestEventsCarryNoDataByDefault asserts a configuration that never asks for
// event data receives none, neither for a plugin type nor for a registered
// type handled without a provider
func TestEventsCarryNoDataByDefault(t *testing.T) {
	recorder := applyEncodeFixtureWithDefaultEventData(t)

	network := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "success")
	require.Empty(t, network.Data)

	database := eventDataSingleEvent(t, recorder, encodeDatabaseID, "create", "success")
	require.Empty(t, database.Data)
}

// TestRawEventDataIsPreCallResource asserts the raw level carries the resource
// as it was before the provider ran, on both the start and the success of a
// create, so the success reports the configuration rather than the result
func TestRawEventDataIsPreCallResource(t *testing.T) {
	recorder, _, _, _ := applyEncodeFixtureWithEventData(t, EventDataRaw)

	started := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "start")
	require.NotEmpty(t, started.Data)

	succeeded := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	network := eventDataNetworkFromEvent(t, succeeded)

	// the provider sets provider_id during the call, so the resource as it was
	// before the call has none
	require.Nil(t, network.ProviderID)
}

// TestProcessedCreateSuccessCarriesProviderFilledValue asserts the processed
// level carries the result of the call, the value the provider filled in
func TestProcessedCreateSuccessCarriesProviderFilledValue(t *testing.T) {
	recorder, _, _, _ := applyEncodeFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "success")

	network := eventDataNetworkFromEvent(t, succeeded)

	require.NotNil(t, network.ProviderID)
	require.Equal(t, "id-main", *network.ProviderID)
}

// TestProcessedCreateSuccessHasCreatedStatus asserts the success event is
// emitted after the resource's status was set, so the data it carries is the
// resource as state records it rather than a resource still mid flight
func TestProcessedCreateSuccessHasCreatedStatus(t *testing.T) {
	recorder, _, _, _ := applyEncodeFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "success")

	network := eventDataNetworkFromEvent(t, succeeded)

	require.NotNil(t, network.Meta.Status)
	require.Equal(t, "created", *network.Meta.Status)
}

// TestProcessedCreateSuccessMatchesStateRecord asserts the processed data is
// the same document state saved for the resource. The state file is written
// indented so the bytes differ in whitespace, the documents do not
func TestProcessedCreateSuccessMatchesStateRecord(t *testing.T) {
	recorder, statePath, _, _ := applyEncodeFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	record := encodeSavedRecordByID(t, statePath, encodeNetworkID)

	require.JSONEq(t, string(record), string(succeeded.Data))
}

// TestRegisteredTypeCarriesDataAtRaw asserts a registered type handled without
// a provider carries its resource at the raw level, it was never serialized
// for a provider call so the resource itself is the raw data
func TestRegisteredTypeCarriesDataAtRaw(t *testing.T) {
	recorder, _, _, _ := applyEncodeFixtureWithEventData(t, EventDataRaw)

	succeeded := eventDataSingleEvent(t, recorder, encodeDatabaseID, "create", "success")

	require.NotEmpty(t, succeeded.Data)
}

// TestRegisteredTypeCarriesDataAtProcessed asserts the same registered type
// carries its resource at the processed level
func TestRegisteredTypeCarriesDataAtProcessed(t *testing.T) {
	recorder, _, _, _ := applyEncodeFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, encodeDatabaseID, "create", "success")

	require.NotEmpty(t, succeeded.Data)
}

// TestProcessedDataConvertsLikeTheEntity asserts the processed data is the
// form EncodeSavedEntity reads, and that converting it gives the same
// configuration text as converting the live entity. This is what ties the
// event data level back to the encoding helpers
func TestProcessedDataConvertsLikeTheEntity(t *testing.T) {
	recorder, _, reg, c := applyEncodeFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, encodeNetworkID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	fromEvent, err := EncodeSavedEntity(reg, succeeded.Data)
	require.NoError(t, err)

	network := encodeEntityByID(t, c, encodeNetworkID)

	fromEntity, err := EncodeEntity(network)
	require.NoError(t, err)

	require.Equal(t, fromEntity, fromEvent)
}
