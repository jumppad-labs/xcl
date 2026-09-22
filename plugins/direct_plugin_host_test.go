package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
)

// loggingProvider is a fake provider that logs through the call's logger in
// Create, and through the plugin scoped logger in Init
type loggingProvider struct {
	DefaultChanged[*testResource]
}

func (p *loggingProvider) Init(state State, functions ProviderFunctions, log logger.Logger) error {
	log.Debug("provider initialised")
	return nil
}

func (p *loggingProvider) Create(ctx context.Context, resource *testResource) (*testResource, error) {
	Logger(ctx).Info("creating test resource", "name", resource.Name)
	return resource, nil
}

func (p *loggingProvider) Destroy(ctx context.Context, resource *testResource, force bool) error {
	return nil
}

func (p *loggingProvider) Read(ctx context.Context, old *testResource, new *testResource) (*testResource, error) {
	return new, nil
}

func (p *loggingProvider) Update(ctx context.Context, resource *testResource) (*testResource, error) {
	return resource, nil
}

func (p *loggingProvider) Functions() ProviderFunctions {
	return nil
}

// LoggingTestPlugin is an in-process plugin that registers the loggingProvider
// for the resource types test and other
type LoggingTestPlugin struct {
	PluginBase
}

func (p *LoggingTestPlugin) Init(log logger.Logger, state State) error {
	p.SetState(state)

	if err := RegisterResourceProvider(&p.PluginBase, log, state, "resource", "test", &testResource{}, &loggingProvider{}); err != nil {
		return err
	}

	return RegisterResourceProvider(&p.PluginBase, log, state, "resource", "other", &testResource{}, &loggingProvider{})
}

// eventsWithPhase returns the events in recorded with the given phase
func eventsWithPhase(recorded []events.Event, phase string) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Phase == phase {
			found = append(found, e)
		}
	}

	return found
}

// adapterFor returns the adapter the host gives for the resource subType
func adapterFor(t *testing.T, host *DirectPluginHost, subType string) ProviderAdapter {
	t.Helper()

	for _, registered := range host.GetTypes() {
		if registered.Type == "resource" && registered.SubType == subType {
			return registered.Adapter
		}
	}

	require.FailNow(t, "no adapter registered", "resource type %s", subType)
	return nil
}

// loadEventsIn returns the events in recorded whose operation is load and
// that are not log messages, the start, success and error events of a load
func loadEventsIn(recorded []events.Event) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Operation == events.OperationLoad && e.Phase != events.PhaseLog {
			found = append(found, e)
		}
	}

	return found
}

func TestNewDirectPluginHostEmitsNoLoadEvents(t *testing.T) {
	recorder := &eventRecorder{}

	_, err := NewDirectPluginHost(recorder.emit, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	require.Empty(t, loadEventsIn(recorder.recorded()), "the registry reports loads, not the host")
}

func TestNewDirectPluginHostWithANilEmitDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
		require.NoError(t, err)
	})
}

func TestNewDirectPluginHostInitLoggerNamesThePluginAsSource(t *testing.T) {
	recorder := &eventRecorder{}

	_, err := NewDirectPluginHost(recorder.emit, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	logs := eventsWithPhase(recorder.recorded(), events.PhaseLog)
	require.Len(t, logs, 2)
	require.Equal(t, "LoggingTestPlugin", logs[0].Source)
	require.Equal(t, events.OperationLoad, logs[0].Operation)
	require.Equal(t, map[string]any{
		"level":    "debug",
		"message":  "provider initialised",
		"provider": "test",
	}, logs[0].Meta)
	require.Equal(t, "other", logs[1].Meta["provider"])
}

func TestDirectPluginHostAdapterNamesThePluginAsSourceOfProviderLogs(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	recorder := &eventRecorder{}
	callLogger := logger.New(recorder.emit, events.Event{
		Source:     events.SourceCore,
		Operation:  events.OperationCreate,
		ResourceID: "resource.test.web",
	})
	ctx := WithLogger(context.Background(), callLogger)

	_, err = adapterFor(t, host, "test").Create(ctx, []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, "LoggingTestPlugin", recorded[0].Source)
	require.Equal(t, "resource.test.web", recorded[0].ResourceID)
	require.Equal(t, events.OperationCreate, recorded[0].Operation)
	require.Equal(t, events.PhaseLog, recorded[0].Phase)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "creating test resource",
		"name":    "web",
	}, recorded[0].Meta)
}

func TestDirectPluginHostAdapterDoesNotChangeTheCallersLogger(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	recorder := &eventRecorder{}
	ctx := WithLogger(context.Background(), logger.New(recorder.emit, events.Event{Source: events.SourceCore}))

	_, err = adapterFor(t, host, "test").Create(ctx, []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	Logger(ctx).Info("after the call")

	recorded := recorder.recorded()
	require.Len(t, recorded, 2)
	require.Equal(t, events.SourceCore, recorded[1].Source)
}

func TestDirectPluginHostCreateNamesThePluginAsSourceOfProviderLogs(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	recorder := &eventRecorder{}
	callLogger := logger.New(recorder.emit, events.Event{Source: events.SourceCore, ResourceID: "resource.test.web"})
	ctx := WithLogger(context.Background(), callLogger)

	_, err = host.Create(ctx, "resource", "test", []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, "LoggingTestPlugin", recorded[0].Source)
	require.Equal(t, "resource.test.web", recorded[0].ResourceID)
}

func TestDirectPluginHostAdapterWithoutABoundLoggerEmitsNothing(t *testing.T) {
	recorder := &eventRecorder{}
	host, err := NewDirectPluginHost(recorder.emit, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)
	beforeCall := len(recorder.recorded())

	_, err = adapterFor(t, host, "test").Create(context.Background(), []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	require.Len(t, recorder.recorded(), beforeCall)
}
