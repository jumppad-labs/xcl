package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/proto"
)

// testPluginName is the name of the external plugin in these tests, the
// Source its log messages are re-sourced to
const testPluginName = "xcl-plugin-person"

// fakeCreateServiceClient is a fake gRPC client that records the context of
// each Create call, and runs during, if set, while the call is in progress.
// The embedded interface satisfies the remaining methods, which must not be
// called by these tests.
type fakeCreateServiceClient struct {
	proto.PluginServiceClient

	contexts []context.Context
	during   func(ctx context.Context)
}

func (c *fakeCreateServiceClient) Create(ctx context.Context, in *proto.CreateRequest, opts ...grpc.CallOption) (*proto.CreateResponse, error) {
	c.contexts = append(c.contexts, ctx)

	if c.during != nil {
		c.during(ctx)
	}

	return &proto.CreateResponse{MutatedEntityData: in.EntityData}, nil
}

// fakeHostCallbackClient is a fake host callback client that records every
// log request sent to it. The embedded interface satisfies the remaining
// methods, which must not be called by these tests.
type fakeHostCallbackClient struct {
	proto.HostCallbackServiceClient

	requests []*proto.LogRequest
}

func (c *fakeHostCallbackClient) Info(ctx context.Context, in *proto.LogRequest, opts ...grpc.CallOption) (*proto.LogResponse, error) {
	c.requests = append(c.requests, in)
	return &proto.LogResponse{}, nil
}

// createCallLogger returns a logger bound the way xcl binds the create of
// resource.person.john, emitting to recorder
func createCallLogger(recorder *eventRecorder) logger.Logger {
	return logger.New(recorder.emit, events.Event{
		Source:       events.SourceCore,
		Operation:    events.OperationCreate,
		ResourceType: "person.john",
		ResourceID:   "resource.person.john",
		File:         "/config/main.xcl",
	})
}

// callIDOf returns the call ID the wrapper sent as outgoing metadata in ctx
func callIDOf(t *testing.T, ctx context.Context) string {
	t.Helper()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok, "the call should carry outgoing metadata")

	ids := md.Get(callIDKey)
	require.Len(t, ids, 1)

	return ids[0]
}

func TestGRPCPluginWrapperSendsCallIDAsOutgoingMetadata(t *testing.T) {
	client := &fakeCreateServiceClient{}
	wrapper := &grpcPluginWrapper{client: client, name: testPluginName, calls: newCallLoggers()}

	_, err := wrapper.Create(context.Background(), "resource", "person", []byte(`{}`))
	require.NoError(t, err)

	require.Len(t, client.contexts, 1)
	require.NotEmpty(t, callIDOf(t, client.contexts[0]))
}

func TestGRPCPluginWrapperGivesEachCallANewCallID(t *testing.T) {
	client := &fakeCreateServiceClient{}
	wrapper := &grpcPluginWrapper{client: client, name: testPluginName, calls: newCallLoggers()}

	_, err := wrapper.Create(context.Background(), "resource", "person", []byte(`{}`))
	require.NoError(t, err)

	_, err = wrapper.Create(context.Background(), "resource", "person", []byte(`{}`))
	require.NoError(t, err)

	require.Len(t, client.contexts, 2)
	require.NotEqual(t, callIDOf(t, client.contexts[0]), callIDOf(t, client.contexts[1]))
}

func TestGRPCPluginWrapperWithoutCallLoggersSendsNoCallID(t *testing.T) {
	client := &fakeCreateServiceClient{}
	wrapper := &grpcPluginWrapper{client: client, name: testPluginName}

	_, err := wrapper.Create(context.Background(), "resource", "person", []byte(`{}`))
	require.NoError(t, err)

	require.Len(t, client.contexts, 1)

	md, _ := metadata.FromOutgoingContext(client.contexts[0])
	require.Empty(t, md.Get(callIDKey))
}

// TestGRPCPluginWrapperRegistersCallLoggerSourcedToPlugin asserts that while
// a call is in progress its ID resolves to the ctx logger, keeping the call's
// resource and step, with the plugin's name as its Source
func TestGRPCPluginWrapperRegistersCallLoggerSourcedToPlugin(t *testing.T) {
	recorder := &eventRecorder{}
	calls := newCallLoggers()

	client := &fakeCreateServiceClient{}
	client.during = func(ctx context.Context) {
		l, ok := calls.lookup(callIDOf(t, ctx))
		require.True(t, ok, "the call's logger should be registered during the call")

		l.Info("Creating person", "name", "John Doe")
	}

	wrapper := &grpcPluginWrapper{client: client, name: testPluginName, calls: calls}
	ctx := WithLogger(context.Background(), createCallLogger(recorder))

	_, err := wrapper.Create(ctx, "resource", "person", []byte(`{}`))
	require.NoError(t, err)

	logged := recorder.recorded()
	require.Len(t, logged, 1)
	require.Equal(t, testPluginName, logged[0].Source)
	require.Equal(t, events.OperationCreate, logged[0].Operation)
	require.Equal(t, events.PhaseLog, logged[0].Phase)
	require.Equal(t, "resource.person.john", logged[0].ResourceID)
	require.Equal(t, "person.john", logged[0].ResourceType)
	require.Equal(t, "/config/main.xcl", logged[0].File)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "Creating person",
		"name":    "John Doe",
	}, logged[0].Meta)
}

func TestGRPCPluginWrapperForgetsCallLoggerAfterCallReturns(t *testing.T) {
	recorder := &eventRecorder{}
	calls := newCallLoggers()
	client := &fakeCreateServiceClient{}
	wrapper := &grpcPluginWrapper{client: client, name: testPluginName, calls: calls}
	ctx := WithLogger(context.Background(), createCallLogger(recorder))

	_, err := wrapper.Create(ctx, "resource", "person", []byte(`{}`))
	require.NoError(t, err)

	require.Len(t, client.contexts, 1)

	_, ok := calls.lookup(callIDOf(t, client.contexts[0]))
	require.False(t, ok, "the call's logger should be removed once the call returns")
}

func TestGRPCLoggerSendsItsCallIDWithEveryMessage(t *testing.T) {
	client := &fakeHostCallbackClient{}
	l := &GRPCLogger{client: client, callID: "host-7"}

	l.Info("Creating person", "name", "John Doe")

	require.Len(t, client.requests, 1)
	require.Equal(t, "host-7", client.requests[0].CallId)
	require.Equal(t, "Creating person", client.requests[0].Message)
	require.Equal(t, []string{"name", "John Doe"}, client.requests[0].Args)
}

// setupCallbackServer returns a host callback server whose plugin scoped
// logger emits to pluginRecorder, with one call in progress whose logger
// emits to callRecorder, and the ID of that call
func setupCallbackServer(pluginRecorder, callRecorder *eventRecorder) (*GRPCHostCallbackServer, string) {
	pluginLogger := logger.New(pluginRecorder.emit, events.Event{Source: testPluginName, Operation: events.OperationLoad})

	server := NewGRPCHostCallbackServer(pluginLogger, nil)
	server.calls = newCallLoggers()

	callID, _ := server.calls.register(logger.WithSource(createCallLogger(callRecorder), testPluginName))

	return server, callID
}

func TestGRPCHostCallbackServerWritesKnownCallIDToTheCallsLogger(t *testing.T) {
	pluginRecorder := &eventRecorder{}
	callRecorder := &eventRecorder{}
	server, callID := setupCallbackServer(pluginRecorder, callRecorder)

	_, err := server.Info(context.Background(), &proto.LogRequest{
		Message: "Creating person",
		Args:    []string{"name", "John Doe"},
		CallId:  callID,
	})
	require.NoError(t, err)

	require.Empty(t, pluginRecorder.recorded())

	logged := callRecorder.recorded()
	require.Len(t, logged, 1)
	require.Equal(t, testPluginName, logged[0].Source)
	require.Equal(t, events.OperationCreate, logged[0].Operation)
	require.Equal(t, "resource.person.john", logged[0].ResourceID)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "Creating person",
		"name":    "John Doe",
	}, logged[0].Meta)
}

func TestGRPCHostCallbackServerWritesUnknownCallIDToThePluginLogger(t *testing.T) {
	pluginRecorder := &eventRecorder{}
	callRecorder := &eventRecorder{}
	server, _ := setupCallbackServer(pluginRecorder, callRecorder)

	_, err := server.Warn(context.Background(), &proto.LogRequest{
		Message: "late message",
		CallId:  "no-such-call",
	})
	require.NoError(t, err)

	require.Empty(t, callRecorder.recorded())

	logged := pluginRecorder.recorded()
	require.Len(t, logged, 1)
	require.Equal(t, testPluginName, logged[0].Source)
	require.Equal(t, events.OperationLoad, logged[0].Operation)
	require.Empty(t, logged[0].ResourceID)
	require.Equal(t, events.LevelWarn, logged[0].Meta[events.KeyLevel])
	require.Equal(t, "late message", logged[0].Meta[events.KeyMessage])
}

func TestGRPCHostCallbackServerWritesEmptyCallIDToThePluginLogger(t *testing.T) {
	pluginRecorder := &eventRecorder{}
	callRecorder := &eventRecorder{}
	server, _ := setupCallbackServer(pluginRecorder, callRecorder)

	_, err := server.Debug(context.Background(), &proto.LogRequest{
		Message: "plugin ready",
		Args:    []string{"types", "1"},
	})
	require.NoError(t, err)

	require.Empty(t, callRecorder.recorded())

	logged := pluginRecorder.recorded()
	require.Len(t, logged, 1)
	require.Equal(t, testPluginName, logged[0].Source)
	require.Equal(t, events.OperationLoad, logged[0].Operation)
	require.Equal(t, map[string]any{
		"level":   "debug",
		"message": "plugin ready",
		"types":   1,
	}, logged[0].Meta)
}

// TestGRPCHostCallbackServerWritesReturnedCallIDToThePluginLogger asserts a
// message arriving after its call has returned falls back to the plugin
// scoped logger
func TestGRPCHostCallbackServerWritesReturnedCallIDToThePluginLogger(t *testing.T) {
	pluginRecorder := &eventRecorder{}
	callRecorder := &eventRecorder{}

	pluginLogger := logger.New(pluginRecorder.emit, events.Event{Source: testPluginName, Operation: events.OperationLoad})
	server := NewGRPCHostCallbackServer(pluginLogger, nil)
	server.calls = newCallLoggers()

	callID, done := server.calls.register(createCallLogger(callRecorder))
	done()

	_, err := server.Error(context.Background(), &proto.LogRequest{Message: "after return", CallId: callID})
	require.NoError(t, err)

	require.Empty(t, callRecorder.recorded())
	require.Len(t, pluginRecorder.recorded(), 1)
}

func TestCallIDFromContextReadsIncomingMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(callIDKey, "host-42"))

	require.Equal(t, "host-42", callIDFromContext(ctx))
}

func TestCallIDFromContextIsEmptyWithoutMetadata(t *testing.T) {
	require.Equal(t, "", callIDFromContext(context.Background()))
}

func TestCallIDFromContextIsEmptyWhenMetadataHasNoCallID(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("other", "value"))

	require.Equal(t, "", callIDFromContext(ctx))
}
