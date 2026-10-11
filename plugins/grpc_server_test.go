package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/proto"
)

// newChangedTestServer returns a GRPCServer serving a changedTestPlugin backed
// by provider, connected to a fake host callback client so no broker is needed
func newChangedTestServer(t *testing.T, provider *changedRecordingProvider) *GRPCServer {
	plugin := &changedTestPlugin{provider: provider}
	require.NoError(t, plugin.Init(logger.Nop(), emptyState{}))

	return &GRPCServer{plugin: plugin, callbackClient: &fakeHostCallbackClient{}}
}

func TestGRPCServerChangedReturnsPluginAnswer(t *testing.T) {
	provider := &changedRecordingProvider{changedResult: entity.Replace}
	server := newChangedTestServer(t, provider)

	resp, err := server.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		OldEntityData: []byte(`{"name":"web"}`),
		NewEntityData: []byte(`{"name":"web"}`),
	})
	require.NoError(t, err)

	require.Empty(t, resp.Error)
	require.Equal(t, proto.Change_CHANGE_REPLACE, resp.Change)
}

func TestGRPCServerChangedReturnsPluginError(t *testing.T) {
	provider := &changedRecordingProvider{changedError: errors.New("unable to compare resources")}
	server := newChangedTestServer(t, provider)

	resp, err := server.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		OldEntityData: []byte(`{"name":"web"}`),
		NewEntityData: []byte(`{"name":"web"}`),
	})
	require.NoError(t, err)

	require.Equal(t, "unable to compare resources", resp.Error)
}

func TestGRPCServerChangedPassesDependenciesToPlugin(t *testing.T) {
	provider := &changedRecordingProvider{changedResult: entity.Update}
	server := newChangedTestServer(t, provider)

	_, err := server.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		OldEntityData: []byte(`{"name":"web"}`),
		NewEntityData: []byte(`{"name":"web"}`),
		Dependencies: []*proto.DependencyChange{
			{Address: "resource.network.main", Change: proto.Change_CHANGE_REPLACE},
			{Address: "resource.volume.data", Change: proto.Change_CHANGE_UPDATE},
		},
	})
	require.NoError(t, err)

	expected := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
		{Address: "resource.volume.data", Change: entity.Update},
	}
	require.Equal(t, expected, provider.changedDependencies)
}

func TestGRPCServerChangedRejectsUnknownDependencyChange(t *testing.T) {
	provider := &changedRecordingProvider{changedResult: entity.Update}
	server := newChangedTestServer(t, provider)

	resp, err := server.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		OldEntityData: []byte(`{"name":"web"}`),
		NewEntityData: []byte(`{"name":"web"}`),
		Dependencies: []*proto.DependencyChange{
			{Address: "resource.network.main", Change: proto.Change(99)},
		},
	})
	require.NoError(t, err)

	require.Contains(t, resp.Error, "dependency resource.network.main")
	require.False(t, provider.changedCalled)
}

func TestGRPCServerChangedPassesDecodedChangesToPlugin(t *testing.T) {
	provider := &changedRecordingProvider{changedResult: entity.Update}
	server := newChangedTestServer(t, provider)

	_, err := server.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		OldEntityData: []byte(`{"name":"web","count":1}`),
		NewEntityData: []byte(`{"name":"web","count":2}`),
		Changes: []*proto.PropertyChange{
			{
				Path:   []*proto.PathStep{{Kind: proto.StepKind_STEP_KIND_ATTRIBUTE, Attribute: "count"}},
				Before: []byte(`1`),
				After:  []byte(`2`),
			},
			{
				Path: []*proto.PathStep{
					{Kind: proto.StepKind_STEP_KIND_ATTRIBUTE, Attribute: "env"},
					{Kind: proto.StepKind_STEP_KIND_KEY, Key: "TOKEN"},
				},
				Before:    []byte(`"old-secret"`),
				After:     []byte(`"new-secret"`),
				Sensitive: true,
			},
			{
				Path: []*proto.PathStep{
					{Kind: proto.StepKind_STEP_KIND_ATTRIBUTE, Attribute: "ports"},
					{Kind: proto.StepKind_STEP_KIND_INDEX, Index: 1},
				},
				Before:  []byte(`443`),
				Unknown: true,
			},
		},
	})
	require.NoError(t, err)

	expected := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("count"), Before: float64(1), After: float64(2)},
		{Path: entity.Path{}.Attribute("env").Key("TOKEN"), Before: "old-secret", After: "new-secret", Sensitive: true},
		{Path: entity.Path{}.Attribute("ports").Index(1), Before: float64(443), Unknown: true},
	}
	require.True(t, provider.changedCalled)
	require.Equal(t, expected, provider.changedChanges)
}

func TestGRPCServerChangedRejectsUnknownStepKind(t *testing.T) {
	provider := &changedRecordingProvider{changedResult: entity.Update}
	server := newChangedTestServer(t, provider)

	resp, err := server.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		OldEntityData: []byte(`{"name":"web"}`),
		NewEntityData: []byte(`{"name":"web"}`),
		Changes: []*proto.PropertyChange{
			{Path: []*proto.PathStep{{Kind: proto.StepKind(99), Attribute: "name"}}},
		},
	})
	require.NoError(t, err)

	require.Contains(t, resp.Error, "unknown step kind 99")
	require.False(t, provider.changedCalled)
}

func TestGRPCServerUpdatePassesDecodedChangesAndDependenciesToPlugin(t *testing.T) {
	provider := &changedRecordingProvider{}
	server := newChangedTestServer(t, provider)

	resp, err := server.Update(context.Background(), &proto.UpdateRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		EntityData:    []byte(`{"name":"web","count":2}`),
		Changes: []*proto.PropertyChange{
			{
				Path:   []*proto.PathStep{{Kind: proto.StepKind_STEP_KIND_ATTRIBUTE, Attribute: "count"}},
				Before: []byte(`1`),
				After:  []byte(`2`),
			},
			{
				Path:      []*proto.PathStep{{Kind: proto.StepKind_STEP_KIND_ATTRIBUTE, Attribute: "password"}},
				Before:    []byte(`"old-secret"`),
				After:     []byte(`"new-secret"`),
				Sensitive: true,
			},
		},
		Dependencies: []*proto.DependencyChange{
			{Address: "resource.network.main", Change: proto.Change_CHANGE_REPLACE},
		},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Error)

	expectedChanges := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("count"), Before: float64(1), After: float64(2)},
		{Path: entity.Path{}.Attribute("password"), Before: "old-secret", After: "new-secret", Sensitive: true},
	}
	expectedDependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
	}
	require.True(t, provider.updateCalled)
	require.Equal(t, expectedChanges, provider.updateChanges)
	require.Equal(t, expectedDependencies, provider.updateDependencies)
}

func TestGRPCServerUpdateRejectsUnknownStepKind(t *testing.T) {
	provider := &changedRecordingProvider{}
	server := newChangedTestServer(t, provider)

	resp, err := server.Update(context.Background(), &proto.UpdateRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		EntityData:    []byte(`{"name":"web"}`),
		Changes: []*proto.PropertyChange{
			{Path: []*proto.PathStep{{Kind: proto.StepKind(99), Attribute: "name"}}},
		},
	})
	require.NoError(t, err)

	require.Contains(t, resp.Error, "unknown step kind 99")
	require.False(t, provider.updateCalled)
}

func TestGRPCServerUpdateRejectsUnknownDependencyChange(t *testing.T) {
	provider := &changedRecordingProvider{}
	server := newChangedTestServer(t, provider)

	resp, err := server.Update(context.Background(), &proto.UpdateRequest{
		EntityType:    "resource",
		EntitySubType: "test",
		EntityData:    []byte(`{"name":"web"}`),
		Dependencies: []*proto.DependencyChange{
			{Address: "resource.network.main", Change: proto.Change(99)},
		},
	})
	require.NoError(t, err)

	require.Contains(t, resp.Error, "dependency resource.network.main")
	require.False(t, provider.updateCalled)
}

// loopbackServiceClient is a gRPC client that hands Changed and Update
// requests straight to a GRPCServer, standing in for the connection to a
// plugin run as a separate program. The embedded interface satisfies the
// remaining methods, which must not be called by these tests.
type loopbackServiceClient struct {
	proto.PluginServiceClient

	server *GRPCServer
}

func (c *loopbackServiceClient) Changed(ctx context.Context, in *proto.ChangedRequest, opts ...grpc.CallOption) (*proto.ChangedResponse, error) {
	return c.server.Changed(ctx, in)
}

func (c *loopbackServiceClient) Update(ctx context.Context, in *proto.UpdateRequest, opts ...grpc.CallOption) (*proto.UpdateResponse, error) {
	return c.server.Update(ctx, in)
}

func TestGRPCChangedDeliversTheSameChangesAndDependenciesAsInProcess(t *testing.T) {
	provider := &changedRecordingProvider{changedResult: entity.Replace}
	server := newChangedTestServer(t, provider)
	wrapper := &grpcPluginWrapper{client: &loopbackServiceClient{server: server}}

	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.0", After: "nginx:2.0"},
		{Path: entity.Path{}.Attribute("env").Key("TOKEN"), Before: "old-secret", After: "new-secret", Sensitive: true},
		{Path: entity.Path{}.Attribute("network").Index(0).Attribute("ip"), Before: "10.0.0.1", Unknown: true},
		{Path: entity.Path{}.Attribute("label"), After: "web"},
	}
	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
	}

	change, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{"name":"web"}`), []byte(`{"name":"web"}`), changes, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)

	require.Equal(t, changes, provider.changedChanges)
	require.Equal(t, dependencies, provider.changedDependencies)
}

func TestGRPCUpdateDeliversTheSameChangesAndDependenciesAsInProcess(t *testing.T) {
	provider := &changedRecordingProvider{}
	server := newChangedTestServer(t, provider)
	wrapper := &grpcPluginWrapper{client: &loopbackServiceClient{server: server}}

	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.0", After: "nginx:2.0"},
		{Path: entity.Path{}.Attribute("env").Key("TOKEN"), Before: "old-secret", After: "new-secret", Sensitive: true},
		{Path: entity.Path{}.Attribute("network").Index(0).Attribute("ip"), Before: "10.0.0.1", Unknown: true},
		{Path: entity.Path{}.Attribute("label"), Before: "web"},
	}
	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
		{Address: "resource.volume.data", Change: entity.Update},
	}

	_, err := wrapper.Update(context.Background(), "resource", "test", []byte(`{"name":"web"}`), changes, dependencies)
	require.NoError(t, err)

	require.True(t, provider.updateCalled)
	require.Equal(t, changes, provider.updateChanges)
	require.Equal(t, dependencies, provider.updateDependencies)
}

// The function RPCs. The host lists and calls a plugin's functions over gRPC
// exactly as it does in process, and a plugin built before the RPCs existed,
// which answers Unimplemented, offers no functions.

// newFunctionTestServer returns a GRPCServer serving a FunctionTestPlugin,
// whose providers offer greet, fail and shout, connected to a fake host
// callback client so no broker is needed
func newFunctionTestServer(t *testing.T) *GRPCServer {
	t.Helper()

	plugin := &FunctionTestPlugin{}
	require.NoError(t, plugin.Init(logger.Nop(), emptyState{}))

	return &GRPCServer{plugin: plugin, callbackClient: &fakeHostCallbackClient{}}
}

func (c *loopbackServiceClient) GetFunctions(ctx context.Context, in *proto.GetFunctionsRequest, opts ...grpc.CallOption) (*proto.GetFunctionsResponse, error) {
	return c.server.GetFunctions(ctx, in)
}

func (c *loopbackServiceClient) CallFunction(ctx context.Context, in *proto.CallFunctionRequest, opts ...grpc.CallOption) (*proto.CallFunctionResponse, error) {
	return c.server.CallFunction(ctx, in)
}

// unimplementedFunctionsClient stands in for the connection to a plugin
// built before the function RPCs existed: both answer Unimplemented, as a
// gRPC server does for a method it does not know
type unimplementedFunctionsClient struct {
	proto.PluginServiceClient
}

func (c *unimplementedFunctionsClient) GetFunctions(ctx context.Context, in *proto.GetFunctionsRequest, opts ...grpc.CallOption) (*proto.GetFunctionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unknown method GetFunctions")
}

func (c *unimplementedFunctionsClient) CallFunction(ctx context.Context, in *proto.CallFunctionRequest, opts ...grpc.CallOption) (*proto.CallFunctionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unknown method CallFunction")
}

// countingFunctionsClient counts the GetFunctions requests it passes on to
// the loopback client it wraps
type countingFunctionsClient struct {
	*loopbackServiceClient

	getFunctionsCalls int
}

func (c *countingFunctionsClient) GetFunctions(ctx context.Context, in *proto.GetFunctionsRequest, opts ...grpc.CallOption) (*proto.GetFunctionsResponse, error) {
	c.getFunctionsCalls++
	return c.loopbackServiceClient.GetFunctions(ctx, in, opts...)
}

func TestGRPCHostListsFunctionsLikeDirectHost(t *testing.T) {
	server := newFunctionTestServer(t)
	wrapper := &grpcPluginWrapper{client: &loopbackServiceClient{server: server}}

	direct, err := NewDirectPluginHost(nil, emptyState{}, &FunctionTestPlugin{})
	require.NoError(t, err)

	overGRPC := wrapper.GetFunctions()
	inProcess := direct.GetFunctions()

	require.Len(t, overGRPC, 3)
	require.Equal(t, "fail", overGRPC[0].Name)
	require.Equal(t, "greet", overGRPC[1].Name)
	require.Equal(t, "shout", overGRPC[2].Name)
	require.Equal(t, inProcess, overGRPC)
}

func TestGRPCHostCallsFunction(t *testing.T) {
	server := newFunctionTestServer(t)
	wrapper := &grpcPluginWrapper{client: &loopbackServiceClient{server: server}}

	result, err := wrapper.CallFunction(context.Background(), "greet", []byte(`["Nic"]`))
	require.NoError(t, err)
	require.JSONEq(t, `"hello Nic"`, string(result))
}

func TestGRPCHostReturnsFunctionError(t *testing.T) {
	server := newFunctionTestServer(t)
	wrapper := &grpcPluginWrapper{client: &loopbackServiceClient{server: server}}

	result, err := wrapper.CallFunction(context.Background(), "fail", []byte(`["Nic"]`))
	require.Nil(t, result)
	require.EqualError(t, err, "boom")
}

func TestGRPCHostTreatsUnimplementedAsNoFunctions(t *testing.T) {
	wrapper := &grpcPluginWrapper{client: &unimplementedFunctionsClient{}}

	require.Nil(t, wrapper.GetFunctions())
}

func TestGRPCHostReportsUnimplementedCallAsNotOffered(t *testing.T) {
	wrapper := &grpcPluginWrapper{client: &unimplementedFunctionsClient{}}

	result, err := wrapper.CallFunction(context.Background(), "greet", []byte(`["Nic"]`))
	require.Nil(t, result)
	require.EqualError(t, err, `function "greet" is not offered by this plugin`)
}

func TestGRPCPluginHostCachesFunctions(t *testing.T) {
	server := newFunctionTestServer(t)
	client := &countingFunctionsClient{loopbackServiceClient: &loopbackServiceClient{server: server}}
	host := &GRPCPluginHost{plugin: &grpcPluginWrapper{client: client}}

	first := host.GetFunctions()
	second := host.GetFunctions()

	require.Equal(t, 1, client.getFunctionsCalls)
	require.Len(t, first, 3)
	require.Equal(t, first, second)
}

func TestGRPCServerGetFunctionsMarshalsSignatures(t *testing.T) {
	server := newFunctionTestServer(t)

	resp, err := server.GetFunctions(context.Background(), &proto.GetFunctionsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Functions, 3)

	require.Equal(t, "greet", resp.Functions[1].Name)

	var signature FunctionSignature
	require.NoError(t, json.Unmarshal(resp.Functions[1].Signature, &signature))

	require.Equal(t, server.plugin.GetFunctions()[1].Signature, signature)
	require.Len(t, signature.Params, 1)
	require.Equal(t, "arg0", signature.Params[0].Name)
	require.JSONEq(t, `"string"`, string(signature.Params[0].Type))
	require.JSONEq(t, `"string"`, string(signature.Return))
	require.Nil(t, signature.Variadic)
}
