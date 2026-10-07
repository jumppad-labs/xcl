package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/example/plugin/docker/client/mocks"
	"github.com/jumppad-labs/xcl/types"
)

func testNetwork() *Network {
	return &Network{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "docker.network.app",
				Name: "app",
				Type: "docker",
			},
		},
	}
}

func TestNetworkCreateAsksDockerForALabelledBridgeNetwork(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(context.Background(), "app", network.CreateOptions{
			Driver:     "bridge",
			Attachable: true,
			Labels: map[string]string{
				"created_by": "xcl-example-plugin",
				"xcl_id":     "docker.network.app",
			},
		}).
		Return(network.CreateResponse{ID: "net-123"}, nil).
		Once()

	provider := &networkProvider{client: client}

	_, err := provider.Create(context.Background(), testNetwork())

	require.NoError(t, err)
}

func TestNetworkCreateSetsTheSubnetWhenGiven(t *testing.T) {
	var got network.CreateOptions

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(context.Background(), "app", network.CreateOptions{
			Driver:     "bridge",
			Attachable: true,
			Labels: map[string]string{
				"created_by": "xcl-example-plugin",
				"xcl_id":     "docker.network.app",
			},
			IPAM: &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{{Subnet: "10.42.0.0/24"}},
			},
		}).
		Run(func(_ context.Context, _ string, options network.CreateOptions) {
			got = options
		}).
		Return(network.CreateResponse{ID: "net-123"}, nil).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.Subnet = "10.42.0.0/24"

	_, err := provider.Create(context.Background(), n)

	require.NoError(t, err)
	require.NotNil(t, got.IPAM)
	require.Equal(t, "10.42.0.0/24", got.IPAM.Config[0].Subnet)
}

func TestNetworkCreateFillsTheDockerID(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(context.Background(), "app", network.CreateOptions{
			Driver:     "bridge",
			Attachable: true,
			Labels: map[string]string{
				"created_by": "xcl-example-plugin",
				"xcl_id":     "docker.network.app",
			},
		}).
		Return(network.CreateResponse{ID: "net-123"}, nil).
		Once()

	provider := &networkProvider{client: client}

	created, err := provider.Create(context.Background(), testNetwork())

	require.NoError(t, err)
	require.Equal(t, "net-123", created.DockerID)
}

func TestNetworkCreateReturnsDockerErrors(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(context.Background(), "app", network.CreateOptions{
			Driver:     "bridge",
			Attachable: true,
			Labels: map[string]string{
				"created_by": "xcl-example-plugin",
				"xcl_id":     "docker.network.app",
			},
		}).
		Return(network.CreateResponse{}, dockerErr).
		Once()

	provider := &networkProvider{client: client}

	created, err := provider.Create(context.Background(), testNetwork())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
}

func TestNetworkDestroyRemovesTheNetworkByID(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkRemove(context.Background(), "net-123").
		Return(nil).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroySucceedsWhenTheNetworkIsGone(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkRemove(context.Background(), "net-123").
		Return(errdefs.NotFound(errors.New("no such network"))).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroyReturnsDockerErrors(t *testing.T) {
	dockerErr := errors.New("network has active endpoints")

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkRemove(context.Background(), "net-123").
		Return(dockerErr).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
}

func TestNetworkReadKeepsTheDockerIDFromTheSavedNetwork(t *testing.T) {
	provider := &networkProvider{client: mocks.NewMockDocker(t)}

	saved := testNetwork()
	saved.DockerID = "net-123"
	configured := testNetwork()

	got, err := provider.Read(context.Background(), saved, configured)

	require.NoError(t, err)
	require.Equal(t, "net-123", got.DockerID)
}
