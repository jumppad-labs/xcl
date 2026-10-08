package plugins

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

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
