package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/mocks"
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
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{ID: "net-123"}, nil).
		Once()
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

func TestNetworkDestroyWithNothingAttachedInspectsThenRemoves(t *testing.T) {
	var calls []string

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Run(func(_ context.Context, _ string, _ network.InspectOptions) {
			calls = append(calls, "inspect")
		}).
		Return(network.Inspect{ID: "net-123", Containers: map[string]network.EndpointResource{}}, nil).
		Once()
	client.EXPECT().
		NetworkRemove(context.Background(), "net-123").
		Run(func(_ context.Context, _ string) {
			calls = append(calls, "remove")
		}).
		Return(nil).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
	require.Equal(t, []string{"inspect", "remove"}, calls)
}

func TestNetworkDestroyForceDisconnectsAttachedContainersInSortedOrderBeforeRemoving(t *testing.T) {
	var calls []string

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Run(func(_ context.Context, _ string, _ network.InspectOptions) {
			calls = append(calls, "inspect")
		}).
		Return(network.Inspect{
			ID: "net-123",
			Containers: map[string]network.EndpointResource{
				"container-b": {Name: "web"},
				"container-a": {Name: "api"},
			},
		}, nil).
		Once()
	client.EXPECT().
		NetworkDisconnect(context.Background(), "net-123", "container-a", true).
		Run(func(_ context.Context, _ string, containerID string, _ bool) {
			calls = append(calls, "disconnect "+containerID)
		}).
		Return(nil).
		Once()
	client.EXPECT().
		NetworkDisconnect(context.Background(), "net-123", "container-b", true).
		Run(func(_ context.Context, _ string, containerID string, _ bool) {
			calls = append(calls, "disconnect "+containerID)
		}).
		Return(nil).
		Once()
	client.EXPECT().
		NetworkRemove(context.Background(), "net-123").
		Run(func(_ context.Context, _ string) {
			calls = append(calls, "remove")
		}).
		Return(nil).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
	require.Equal(t, []string{
		"inspect",
		"disconnect container-a",
		"disconnect container-b",
		"remove",
	}, calls)
}

func TestNetworkDestroySucceedsWithoutRemovingWhenInspectFindsTheNetworkGone(t *testing.T) {
	// the strict mock fails the test if NetworkRemove is called
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{}, errdefs.NotFound(errors.New("no such network"))).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroySucceedsWhenTheNetworkIsGoneByRemoveTime(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{ID: "net-123"}, nil).
		Once()
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

func TestNetworkDestroyIgnoresAContainerAlreadyGoneOnDisconnectAndStillRemoves(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{
			ID: "net-123",
			Containers: map[string]network.EndpointResource{
				"container-a": {Name: "api"},
			},
		}, nil).
		Once()
	client.EXPECT().
		NetworkDisconnect(context.Background(), "net-123", "container-a", true).
		Return(errdefs.NotFound(errors.New("no such container"))).
		Once()
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

func TestNetworkDestroyReturnsInspectErrors(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	// the strict mock fails the test if NetworkRemove is called
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{}, dockerErr).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
}

func TestNetworkDestroyReturnsDisconnectErrorsWithoutRemoving(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	// the strict mock fails the test if NetworkRemove is called
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{
			ID: "net-123",
			Containers: map[string]network.EndpointResource{
				"container-a": {Name: "api"},
			},
		}, nil).
		Once()
	client.EXPECT().
		NetworkDisconnect(context.Background(), "net-123", "container-a", true).
		Return(dockerErr).
		Once()

	provider := &networkProvider{client: client}
	n := testNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
	require.ErrorContains(t, err, "container-a")
}

func TestNetworkDestroyReturnsRemoveErrors(t *testing.T) {
	dockerErr := errors.New("network has active endpoints")

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(context.Background(), "net-123", network.InspectOptions{}).
		Return(network.Inspect{ID: "net-123"}, nil).
		Once()
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

func TestNetworkChangedReplacesOnSubnetChange(t *testing.T) {
	p := NewNetworkProvider(mocks.NewMockDocker(t))

	old := testNetwork()
	old.Subnet = "10.42.0.0/24"
	new := testNetwork()
	new.Subnet = "10.43.0.0/24"

	change, err := p.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestNetworkChangedReportsNoChangeForIdenticalNetwork(t *testing.T) {
	p := NewNetworkProvider(mocks.NewMockDocker(t))

	old := testNetwork()
	old.Subnet = "10.42.0.0/24"
	new := testNetwork()
	new.Subnet = "10.42.0.0/24"

	change, err := p.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}
