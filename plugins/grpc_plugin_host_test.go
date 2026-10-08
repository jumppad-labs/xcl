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

// fakeChangedServiceClient is a fake gRPC client that records the request sent
// to Changed and returns a fixed response. The embedded interface satisfies
// the remaining methods, which must not be called by these tests.
type fakeChangedServiceClient struct {
	proto.PluginServiceClient

	changedRequest  *proto.ChangedRequest
	changedResponse *proto.ChangedResponse
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

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{"a":1}`), []byte(`{"a":2}`), dependencies)
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

	change, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil)

	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestGRPCPluginWrapperChangedReturnsError(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Error: "unable to compare resources"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	change, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil)

	require.EqualError(t, err, "unable to compare resources")
	require.Equal(t, entity.NoChange, change)
}

func TestGRPCPluginWrapperChangedRejectsUnknownChange(t *testing.T) {
	client := &fakeChangedServiceClient{
		changedResponse: &proto.ChangedResponse{Change: proto.Change(99)},
	}
	wrapper := &grpcPluginWrapper{client: client}

	_, err := wrapper.Changed(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`), nil)

	require.ErrorContains(t, err, "unknown change 99")
}
