package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// eventRecorder records every event passed to its handler, it adds the root
// package's queries to the shared recorder
type eventRecorder struct {
	testutil.EventRecorder
}

// reset discards every event recorded so far, so a test sees only what
// follows. It must only be called while nothing is delivering events, such
// as after Apply has returned
func (r *eventRecorder) reset() {
	r.EventRecorder = testutil.EventRecorder{}
}

// find returns the events for the resource id, operation and phase
func (r *eventRecorder) find(id, operation, phase string) []Event {
	found := []Event{}
	for _, e := range r.Events() {
		if e.ResourceID == id && e.Operation == operation && e.Phase == phase {
			found = append(found, e)
		}
	}

	return found
}

// applyQueryFixtureWithEvents applies the query fixture, three registered
// database blocks and two plugin network blocks, with an event handler
func applyQueryFixtureWithEvents(t *testing.T) *eventRecorder {
	t.Helper()

	return applyQueryFixtureWithEventData(t, EventDataNone)
}

// applyQueryFixtureWithEventData is applyQueryFixtureWithEvents for a test
// that asserts on Event.Data, which carries nothing unless a level asks for it
func applyQueryFixtureWithEventData(t *testing.T, level EventDataLevel) *eventRecorder {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	local := registry.NewLocal()

	local.RegisterPlugin(&parser.TestPlugin{})

	recorder := &eventRecorder{}

	c, err := NewConfig(
		WithType(&registered.Database{}, "resource", registered.TypeDatabase),
		WithRegistry(local),
		WithEventHandler(recorder.Record),
		WithEventData(level),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return recorder
}

// TestApplyCallsEventHandlerWhenPluginResourceIsCreated asserts the handler
// receives the start and success of a plugin resource's create, with the
// serialized resource
func TestApplyCallsEventHandlerWhenPluginResourceIsCreated(t *testing.T) {
	recorder := applyQueryFixtureWithEventData(t, EventDataRaw)

	started := recorder.find("resource.network.frontend", "create", "start")
	require.Len(t, started, 1)
	require.Equal(t, "network.frontend", started[0].ResourceType)
	require.NotEmpty(t, started[0].Data)

	succeeded := recorder.find("resource.network.frontend", "create", "success")
	require.Len(t, succeeded, 1)
	require.NoError(t, succeeded[0].Error)
	require.NotEmpty(t, succeeded[0].Data)
}

// TestApplyCallsEventHandlerWhenRegisteredTypeIsApplied asserts the handler
// receives the success of a registered type, which has no provider and so no
// start event. It carries no data because no event does at the default level,
// see TestRegisteredTypeCarriesDataAtRaw for what it carries when asked
func TestApplyCallsEventHandlerWhenRegisteredTypeIsApplied(t *testing.T) {
	recorder := applyQueryFixtureWithEvents(t)

	require.Empty(t, recorder.find("resource.database.primary", "create", "start"))

	succeeded := recorder.find("resource.database.primary", "create", "success")
	require.Len(t, succeeded, 1)
	require.Equal(t, "database.primary", succeeded[0].ResourceType)
	require.Nil(t, succeeded[0].Data)
}

// TestApplyCallsEventHandlerWhenResourceIsParsed asserts the handler receives
// a parse event for each resource, with the file it was parsed from
func TestApplyCallsEventHandlerWhenResourceIsParsed(t *testing.T) {
	recorder := applyQueryFixtureWithEvents(t)

	file, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	parsed := recorder.find("resource.network.frontend", "parse", "success")
	require.Len(t, parsed, 1)
	require.Equal(t, "network.frontend", parsed[0].ResourceType)
	require.Equal(t, file, parsed[0].File)
}

// TestValidateCallsEventHandlerWhenResourceIsParsed asserts Validate passes
// the parse events to the handler between the validate operation's start and
// success, after the TestPlugin's load start and success as this is the first
// operation, and nothing else as Validate creates nothing
func TestValidateCallsEventHandlerWhenResourceIsParsed(t *testing.T) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	local := registry.NewLocal()

	local.RegisterPlugin(&parser.TestPlugin{})

	recorder := &eventRecorder{}

	c, err := NewConfig(
		WithType(&registered.Database{}, "resource", registered.TypeDatabase),
		WithRegistry(local),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)

	file, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Validate(file)
	require.NoError(t, err)

	// the plugin load and the parse events are wrapped by the validate
	// operation's start and success
	recorded := recorder.Events()
	require.Len(t, recorded, 9)

	first := recorded[0]
	require.Equal(t, "validate", first.Operation)
	require.Equal(t, "start", first.Phase)

	require.Equal(t, "load", recorded[1].Operation)
	require.Equal(t, "start", recorded[1].Phase)
	require.Equal(t, "load", recorded[2].Operation)
	require.Equal(t, "success", recorded[2].Phase)

	last := recorded[8]
	require.Equal(t, "validate", last.Operation)
	require.Equal(t, "success", last.Phase)

	for _, e := range recorded[3:8] {
		require.Equal(t, "parse", e.Operation)
		require.Equal(t, "success", e.Phase)
		require.Equal(t, file, e.File)
	}
}
