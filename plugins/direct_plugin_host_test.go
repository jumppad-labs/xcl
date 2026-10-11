package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
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

func (p *loggingProvider) Update(ctx context.Context, resource *testResource, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*testResource, error) {
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
	recorder := &testutil.EventRecorder{}

	_, err := NewDirectPluginHost(recorder.Record, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	require.Empty(t, loadEventsIn(recorder.Events()), "the registry reports loads, not the host")
}

func TestNewDirectPluginHostWithANilEmitDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
		require.NoError(t, err)
	})
}

func TestNewDirectPluginHostInitLoggerNamesThePluginAsSource(t *testing.T) {
	recorder := &testutil.EventRecorder{}

	_, err := NewDirectPluginHost(recorder.Record, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	logs := testutil.EventsWithPhase(recorder.Events(), events.PhaseLog)
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

	recorder := &testutil.EventRecorder{}
	callLogger := logger.New(recorder.Record, events.Event{
		Source:     events.SourceCore,
		Operation:  events.OperationCreate,
		ResourceID: "resource.test.web",
	})
	ctx := WithLogger(context.Background(), callLogger)

	_, err = adapterFor(t, host, "test").Create(ctx, []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	recorded := recorder.Events()
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

	recorder := &testutil.EventRecorder{}
	ctx := WithLogger(context.Background(), logger.New(recorder.Record, events.Event{Source: events.SourceCore}))

	_, err = adapterFor(t, host, "test").Create(ctx, []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	Logger(ctx).Info("after the call")

	recorded := recorder.Events()
	require.Len(t, recorded, 2)
	require.Equal(t, events.SourceCore, recorded[1].Source)
}

func TestDirectPluginHostCreateNamesThePluginAsSourceOfProviderLogs(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	callLogger := logger.New(recorder.Record, events.Event{Source: events.SourceCore, ResourceID: "resource.test.web"})
	ctx := WithLogger(context.Background(), callLogger)

	_, err = host.Create(ctx, "resource", "test", []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	recorded := recorder.Events()
	require.Len(t, recorded, 1)
	require.Equal(t, "LoggingTestPlugin", recorded[0].Source)
	require.Equal(t, "resource.test.web", recorded[0].ResourceID)
}

func TestDirectPluginHostAdapterWithoutABoundLoggerEmitsNothing(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	host, err := NewDirectPluginHost(recorder.Record, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)
	beforeCall := len(recorder.Events())

	_, err = adapterFor(t, host, "test").Create(context.Background(), []byte(`{"name":"web","count":1}`))
	require.NoError(t, err)

	require.Len(t, recorder.Events(), beforeCall)
}

// loggingFunction is a ProviderFunction that logs through the call's logger,
// so a test can see which source the host names for a function call. A
// function built with NewFunction never sees the call's context, so one is
// declared by hand
type loggingFunction struct{}

func (loggingFunction) Signature() FunctionSignature {
	return FunctionSignature{
		Params: []FunctionParam{{Name: "arg0", Type: json.RawMessage(`"string"`)}},
		Return: json.RawMessage(`"string"`),
	}
}

func (loggingFunction) Call(ctx context.Context, args []byte) ([]byte, error) {
	Logger(ctx).Info("function called", "args", string(args))
	return json.Marshal("logged")
}

// FunctionTestPlugin is an in-process plugin whose providers offer greet,
// which prefixes its argument with hello, fail, which always returns the
// error boom, and shout, which logs through the call's logger
type FunctionTestPlugin struct {
	PluginBase
}

func (p *FunctionTestPlugin) Init(log logger.Logger, state State) error {
	p.SetState(state)

	greet, err := NewFunction(func(name string) (string, error) {
		return "hello " + name, nil
	})
	if err != nil {
		return err
	}

	fail, err := NewFunction(func(name string) (string, error) {
		return "", errors.New("boom")
	})
	if err != nil {
		return err
	}

	greeter := &functionProvider{fns: map[string]ProviderFunction{"greet": greet, "shout": loggingFunction{}}}
	if err := RegisterResourceProvider(&p.PluginBase, log, state, "resource", "greeter", &testResource{}, greeter); err != nil {
		return err
	}

	failer := &functionProvider{fns: map[string]ProviderFunction{"fail": fail}}
	return RegisterResourceProvider(&p.PluginBase, log, state, "resource", "failer", &testResource{}, failer)
}

func TestDirectHostListsFunctions(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &FunctionTestPlugin{})
	require.NoError(t, err)

	registered := host.GetFunctions()
	require.Len(t, registered, 3)
	require.Equal(t, "fail", registered[0].Name)
	require.Equal(t, "greet", registered[1].Name)
	require.Equal(t, "shout", registered[2].Name)

	greet := registered[1].Signature
	require.Len(t, greet.Params, 1)
	require.Equal(t, `"string"`, string(greet.Params[0].Type))
	require.Equal(t, `"string"`, string(greet.Return))
}

func TestDirectHostCallsFunction(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &FunctionTestPlugin{})
	require.NoError(t, err)

	result, err := host.CallFunction(context.Background(), "greet", []byte(`["Nic"]`))
	require.NoError(t, err)

	var greeting string
	require.NoError(t, json.Unmarshal(result, &greeting))
	require.Equal(t, "hello Nic", greeting)
}

func TestDirectHostReturnsFunctionError(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &FunctionTestPlugin{})
	require.NoError(t, err)

	_, err = host.CallFunction(context.Background(), "fail", []byte(`["Nic"]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestDirectHostCallFunctionNamesThePluginAsLoggerSource(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &FunctionTestPlugin{})
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	callLogger := logger.New(recorder.Record, events.Event{Source: events.SourceCore, Operation: events.OperationLoad})
	ctx := WithLogger(context.Background(), callLogger)

	result, err := host.CallFunction(ctx, "shout", []byte(`["Nic"]`))
	require.NoError(t, err)
	require.JSONEq(t, `"logged"`, string(result))

	recorded := recorder.Events()
	require.Len(t, recorded, 1)
	require.Equal(t, "FunctionTestPlugin", recorded[0].Source)
	require.Equal(t, events.OperationLoad, recorded[0].Operation)
	require.Equal(t, events.PhaseLog, recorded[0].Phase)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "function called",
		"args":    `["Nic"]`,
	}, recorded[0].Meta)
}

func TestDirectHostWithPluginOfferingNoFunctionsListsNone(t *testing.T) {
	host, err := NewDirectPluginHost(nil, emptyState{}, &LoggingTestPlugin{})
	require.NoError(t, err)

	require.Empty(t, host.GetFunctions())
}
