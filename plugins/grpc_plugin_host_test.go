package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/plugins/proto"
)

// fakeReadServiceClient is a fake gRPC client that returns a fixed response
// from Read. The embedded interface satisfies the remaining methods, which
// must not be called by these tests.
type fakeReadServiceClient struct {
	proto.PluginServiceClient

	readResponse *proto.ReadResponse
}

func (c *fakeReadServiceClient) Read(ctx context.Context, in *proto.ReadRequest, opts ...grpc.CallOption) (*proto.ReadResponse, error) {
	return c.readResponse, nil
}

func TestGRPCPluginWrapperReadPreservesErrorMessageContainingPercent(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{Error: "disk 100% full"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	result, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.EqualError(t, err, "disk 100% full")
	require.Nil(t, result)
}

func TestGRPCPluginWrapperReadErrorWithoutNotFoundIsNotErrNotFound(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{Error: "disk 100% full"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	_, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNotFound)
}

func TestGRPCPluginWrapperReadNotFoundIsErrNotFound(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{NotFound: true, Error: "resource not found"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	result, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorContains(t, err, "resource not found")
	require.Nil(t, result)
}

func TestGRPCPluginWrapperReadReturnsEntityData(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{EntityData: []byte(`{"name":"test"}`)},
	}
	wrapper := &grpcPluginWrapper{client: client}

	result, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.NoError(t, err)
	require.Equal(t, []byte(`{"name":"test"}`), result)
}

// fakeChangedServiceClient is a fake gRPC client that records the requests
// sent to Changed and Update and returns fixed responses. The embedded
// interface satisfies the remaining methods, which must not be called by these
// tests.
type fakeChangedServiceClient struct {
	proto.PluginServiceClient

	changedRequest  *proto.ChangedRequest
	changedResponse *proto.ChangedResponse

	updateRequest  *proto.UpdateRequest
	updateResponse *proto.UpdateResponse
}

func (c *fakeChangedServiceClient) Update(ctx context.Context, in *proto.UpdateRequest, opts ...grpc.CallOption) (*proto.UpdateResponse, error) {
	c.updateRequest = in
	return c.updateResponse, nil
}

func (c *fakeChangedServiceClient) Changed(ctx context.Context, in *proto.ChangedRequest, opts ...grpc.CallOption) (*proto.ChangedResponse, error) {
	c.changedRequest = in
	return c.changedResponse, nil
}

func TestGRPCPluginWrapperChangedSendsDependencies(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change_CHANGE_NO_CHANGE},
	}
	wrapper := &grpcPluginWrapper{client: client}

	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
		{Address: "resource.volume.data", Change: entity.Update},
	}

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{"a":1}`), []byte(`{"a":2}`), nil, dependencies)
	require.NoError(t, err)

	require.NotNil(t, client.changedRequest)
	require.Equal(t, "resource", client.changedRequest.EntityType)
	require.Equal(t, "test", client.changedRequest.EntitySubType)
	require.Equal(t, []byte(`{"a":1}`), client.changedRequest.OldEntityData)
	require.Equal(t, []byte(`{"a":2}`), client.changedRequest.NewEntityData)
	require.Len(t, client.changedRequest.Dependencies, 2)
	require.Equal(t, "resource.network.main", client.changedRequest.Dependencies[0].Address)
	require.Equal(t, proto.Change_CHANGE_REPLACE, client.changedRequest.Dependencies[0].Change)
	require.Equal(t, "resource.volume.data", client.changedRequest.Dependencies[1].Address)
	require.Equal(t, proto.Change_CHANGE_UPDATE, client.changedRequest.Dependencies[1].Change)
}

func TestGRPCPluginWrapperChangedReturnsReplace(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change_CHANGE_REPLACE},
	}
	wrapper := &grpcPluginWrapper{client: client}

	change, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil, nil)

	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestGRPCPluginWrapperChangedReturnsError(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Error: "unable to compare resources"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	change, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil, nil)

	require.EqualError(t, err, "unable to compare resources")
	require.Equal(t, entity.NoChange, change)
}

func TestGRPCPluginWrapperChangedRejectsUnknownChange(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change(99)},
	}
	wrapper := &grpcPluginWrapper{client: client}

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil, nil)

	require.ErrorContains(t, err, "unknown change 99")
}

func TestGRPCPluginWrapperChangedSendsChanges(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change_CHANGE_UPDATE},
	}
	wrapper := &grpcPluginWrapper{client: client}

	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("env").Key("LOG_LEVEL"), Before: "info", After: "debug"},
		{Path: entity.Path{}.Attribute("password"), Before: "old-secret", After: "new-secret", Sensitive: true},
		{Path: entity.Path{}.Attribute("ports").Index(0), Before: float64(80), Unknown: true},
	}

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), changes, nil)
	require.NoError(t, err)

	require.NotNil(t, client.changedRequest)
	require.Len(t, client.changedRequest.Changes, 3)

	first := client.changedRequest.Changes[0]
	require.Len(t, first.Path, 2)
	require.Equal(t, proto.StepKind_STEP_KIND_ATTRIBUTE, first.Path[0].Kind)
	require.Equal(t, "env", first.Path[0].Attribute)
	require.Equal(t, proto.StepKind_STEP_KIND_KEY, first.Path[1].Kind)
	require.Equal(t, "LOG_LEVEL", first.Path[1].Key)
	require.Equal(t, []byte(`"info"`), first.Before)
	require.Equal(t, []byte(`"debug"`), first.After)
	require.False(t, first.Sensitive)
	require.False(t, first.Unknown)

	second := client.changedRequest.Changes[1]
	require.True(t, second.Sensitive)
	require.Equal(t, []byte(`"old-secret"`), second.Before)
	require.Equal(t, []byte(`"new-secret"`), second.After)

	third := client.changedRequest.Changes[2]
	require.Len(t, third.Path, 2)
	require.Equal(t, proto.StepKind_STEP_KIND_INDEX, third.Path[1].Kind)
	require.Equal(t, int64(0), third.Path[1].Index)
	require.True(t, third.Unknown)
	require.Equal(t, []byte(`80`), third.Before)
	require.Empty(t, third.After)
}

func TestGRPCPluginWrapperChangedSendsNoChangesForNil(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change_CHANGE_NO_CHANGE},
	}
	wrapper := &grpcPluginWrapper{client: client}

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil, nil)
	require.NoError(t, err)

	require.NotNil(t, client.changedRequest)
	require.Empty(t, client.changedRequest.Changes)
}

func TestGRPCPluginWrapperChangedRejectsUnknownStepKind(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change_CHANGE_NO_CHANGE},
	}
	wrapper := &grpcPluginWrapper{client: client}

	changes := []entity.PropertyChange{
		{Path: entity.Path{{Kind: entity.StepKind(99), Attribute: "image"}}, After: "nginx:2.0"},
	}

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), changes, nil)

	require.ErrorContains(t, err, "unknown step kind 99")
	require.Nil(t, client.changedRequest)
}

func TestGRPCPluginWrapperUpdateSendsChangesAndDependencies(t *testing.T) {
	client := &fakeChangedServiceClient{
		updateResponse: &proto.UpdateResponse{UpdatedEntityData: []byte(`{"name":"web"}`)},
	}
	wrapper := &grpcPluginWrapper{client: client}

	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.0", After: "nginx:2.0"},
		{Path: entity.Path{}.Attribute("token"), Before: "a", After: "b", Sensitive: true},
	}
	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Replace},
		{Address: "resource.volume.data", Change: entity.Update},
	}

	data, err := wrapper.Update(context.Background(), "resource", "test", []byte(`{"name":"web"}`), changes, dependencies)
	require.NoError(t, err)
	require.Equal(t, []byte(`{"name":"web"}`), data)

	require.NotNil(t, client.updateRequest)
	require.Equal(t, "resource", client.updateRequest.EntityType)
	require.Equal(t, "test", client.updateRequest.EntitySubType)
	require.Equal(t, []byte(`{"name":"web"}`), client.updateRequest.EntityData)

	require.Len(t, client.updateRequest.Changes, 2)
	require.Len(t, client.updateRequest.Changes[0].Path, 1)
	require.Equal(t, "image", client.updateRequest.Changes[0].Path[0].Attribute)
	require.Equal(t, []byte(`"nginx:1.0"`), client.updateRequest.Changes[0].Before)
	require.Equal(t, []byte(`"nginx:2.0"`), client.updateRequest.Changes[0].After)
	require.True(t, client.updateRequest.Changes[1].Sensitive)
	require.Equal(t, []byte(`"a"`), client.updateRequest.Changes[1].Before)
	require.Equal(t, []byte(`"b"`), client.updateRequest.Changes[1].After)

	require.Len(t, client.updateRequest.Dependencies, 2)
	require.Equal(t, "resource.network.main", client.updateRequest.Dependencies[0].Address)
	require.Equal(t, proto.Change_CHANGE_REPLACE, client.updateRequest.Dependencies[0].Change)
	require.Equal(t, "resource.volume.data", client.updateRequest.Dependencies[1].Address)
	require.Equal(t, proto.Change_CHANGE_UPDATE, client.updateRequest.Dependencies[1].Change)
}

func TestGRPCPluginWrapperUpdateRejectsUnknownStepKind(t *testing.T) {
	client := &fakeChangedServiceClient{
		updateResponse: &proto.UpdateResponse{},
	}
	wrapper := &grpcPluginWrapper{client: client}

	changes := []entity.PropertyChange{
		{Path: entity.Path{{Kind: entity.StepKind(99), Attribute: "image"}}, After: "nginx:2.0"},
	}

	_, err := wrapper.Update(context.Background(), "resource", "test", []byte(`{}`), changes, nil)

	require.ErrorContains(t, err, "unknown step kind 99")
	require.Nil(t, client.updateRequest)
}

func TestGRPCPluginWrapperUpdateRejectsUnknownDependencyChange(t *testing.T) {
	client := &fakeChangedServiceClient{
		updateResponse: &proto.UpdateResponse{},
	}
	wrapper := &grpcPluginWrapper{client: client}

	dependencies := []entity.DependencyChange{
		{Address: "resource.network.main", Change: entity.Change(99)},
	}

	_, err := wrapper.Update(context.Background(), "resource", "test", []byte(`{}`), nil, dependencies)

	require.ErrorContains(t, err, "dependency resource.network.main")
	require.Nil(t, client.updateRequest)
}
