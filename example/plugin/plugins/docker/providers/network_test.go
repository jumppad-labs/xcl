package providers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers/mocks"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/types"
)

func networkTestNetwork() *entities.Network {
	return &entities.Network{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "docker.network.app",
				Name: "app",
				Type: "docker",
			},
		},
	}
}

func networkTestLabels() map[string]string {
	return map[string]string{
		"created_by": "xcl-example-plugin",
		"xcl_id":     "docker.network.app",
	}
}

func networkTestNotFound(message string) error {
	return fmt.Errorf("%s: %w", message, containers.ErrNotFound)
}

func TestNetworkCreateAsksDockerForALabelledBridgeNetwork(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		CreateNetwork(context.Background(), "app", containers.NetworkSpec{
			Labels: networkTestLabels(),
		}).
		Return("net-123", nil).
		Once()

	provider := &networkProvider{tasks: tasks}

	_, err := provider.Create(context.Background(), networkTestNetwork())

	require.NoError(t, err)
}

func TestNetworkCreateSetsTheSubnetWhenGiven(t *testing.T) {
	var got containers.NetworkSpec

	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		CreateNetwork(context.Background(), "app", containers.NetworkSpec{
			Subnet: "10.42.0.0/24",
			Labels: networkTestLabels(),
		}).
		Run(func(_ context.Context, _ string, spec containers.NetworkSpec) {
			got = spec
		}).
		Return("net-123", nil).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.Subnet = "10.42.0.0/24"

	_, err := provider.Create(context.Background(), n)

	require.NoError(t, err)
	require.Equal(t, "10.42.0.0/24", got.Subnet)
}

func TestNetworkCreateFillsTheDockerID(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		CreateNetwork(context.Background(), "app", containers.NetworkSpec{
			Labels: networkTestLabels(),
		}).
		Return("net-123", nil).
		Once()

	provider := &networkProvider{tasks: tasks}

	created, err := provider.Create(context.Background(), networkTestNetwork())

	require.NoError(t, err)
	require.Equal(t, "net-123", created.DockerID)
}

func TestNetworkCreateReturnsDockerErrors(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		CreateNetwork(context.Background(), "app", containers.NetworkSpec{
			Labels: networkTestLabels(),
		}).
		Return("", dockerErr).
		Once()

	provider := &networkProvider{tasks: tasks}

	created, err := provider.Create(context.Background(), networkTestNetwork())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
}

func TestNetworkDestroyRemovesTheNetworkByID(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return([]string{}, nil).
		Once()
	tasks.EXPECT().
		RemoveNetwork(context.Background(), "net-123").
		Return(nil).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroyWithNothingAttachedInspectsThenRemoves(t *testing.T) {
	var calls []string

	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Run(func(_ context.Context, _ string) {
			calls = append(calls, "inspect")
		}).
		Return([]string{}, nil).
		Once()
	tasks.EXPECT().
		RemoveNetwork(context.Background(), "net-123").
		Run(func(_ context.Context, _ string) {
			calls = append(calls, "remove")
		}).
		Return(nil).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
	require.Equal(t, []string{"inspect", "remove"}, calls)
}

func TestNetworkDestroyForceDisconnectsAttachedContainersInSortedOrderBeforeRemoving(t *testing.T) {
	var calls []string

	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Run(func(_ context.Context, _ string) {
			calls = append(calls, "inspect")
		}).
		Return([]string{"container-a", "container-b"}, nil).
		Once()
	tasks.EXPECT().
		DisconnectNetwork(context.Background(), "net-123", "container-a").
		Run(func(_ context.Context, _ string, containerID string) {
			calls = append(calls, "disconnect "+containerID)
		}).
		Return(nil).
		Once()
	tasks.EXPECT().
		DisconnectNetwork(context.Background(), "net-123", "container-b").
		Run(func(_ context.Context, _ string, containerID string) {
			calls = append(calls, "disconnect "+containerID)
		}).
		Return(nil).
		Once()
	tasks.EXPECT().
		RemoveNetwork(context.Background(), "net-123").
		Run(func(_ context.Context, _ string) {
			calls = append(calls, "remove")
		}).
		Return(nil).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
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
	// the strict mock fails the test if RemoveNetwork is called
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return(nil, networkTestNotFound("no such network")).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroySucceedsWhenTheNetworkIsGoneByRemoveTime(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return([]string{}, nil).
		Once()
	tasks.EXPECT().
		RemoveNetwork(context.Background(), "net-123").
		Return(networkTestNotFound("no such network")).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroyIgnoresAContainerAlreadyGoneOnDisconnectAndStillRemoves(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return([]string{"container-a"}, nil).
		Once()
	tasks.EXPECT().
		DisconnectNetwork(context.Background(), "net-123", "container-a").
		Return(networkTestNotFound("no such container")).
		Once()
	tasks.EXPECT().
		RemoveNetwork(context.Background(), "net-123").
		Return(nil).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.NoError(t, err)
}

func TestNetworkDestroyReturnsInspectErrors(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	// the strict mock fails the test if RemoveNetwork is called
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return(nil, dockerErr).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
}

func TestNetworkDestroyReturnsDisconnectErrorsWithoutRemoving(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	// the strict mock fails the test if RemoveNetwork is called
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return([]string{"container-a"}, nil).
		Once()
	tasks.EXPECT().
		DisconnectNetwork(context.Background(), "net-123", "container-a").
		Return(dockerErr).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
	require.ErrorContains(t, err, "container-a")
}

func TestNetworkDestroyReturnsRemoveErrors(t *testing.T) {
	dockerErr := errors.New("network has active endpoints")

	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		NetworkContainers(context.Background(), "net-123").
		Return([]string{}, nil).
		Once()
	tasks.EXPECT().
		RemoveNetwork(context.Background(), "net-123").
		Return(dockerErr).
		Once()

	provider := &networkProvider{tasks: tasks}
	n := networkTestNetwork()
	n.DockerID = "net-123"

	err := provider.Destroy(context.Background(), n, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "app")
}

func TestNetworkReadKeepsTheDockerIDFromTheSavedNetwork(t *testing.T) {
	provider := &networkProvider{tasks: mocks.NewMockTasks(t)}

	saved := networkTestNetwork()
	saved.DockerID = "net-123"
	configured := networkTestNetwork()

	got, err := provider.Read(context.Background(), saved, configured)

	require.NoError(t, err)
	require.Equal(t, "net-123", got.DockerID)
}

func TestNetworkChangedReplacesOnSubnetChange(t *testing.T) {
	p := NewNetworkProvider(mocks.NewMockTasks(t))

	old := networkTestNetwork()
	old.Subnet = "10.42.0.0/24"
	new := networkTestNetwork()
	new.Subnet = "10.43.0.0/24"

	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("subnet"), Before: "10.42.0.0/24", After: "10.43.0.0/24"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestNetworkChangedUpdatesWhenANonReplaceSettingChanges(t *testing.T) {
	p := NewNetworkProvider(mocks.NewMockTasks(t))

	old := networkTestNetwork()
	new := networkTestNetwork()
	new.DependsOn = []string{"docker.network.other"}

	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("depends_on"), Before: nil, After: []any{"docker.network.other"}},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestNetworkChangedReportsNoChangeForIdenticalNetwork(t *testing.T) {
	p := NewNetworkProvider(mocks.NewMockTasks(t))

	old := networkTestNetwork()
	old.Subnet = "10.42.0.0/24"
	new := networkTestNetwork()
	new.Subnet = "10.42.0.0/24"

	change, err := p.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}
