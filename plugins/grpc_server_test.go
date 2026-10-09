package plugins

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"

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
