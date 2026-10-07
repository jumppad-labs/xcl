package resources

// The tests in this file run the providers against a real Docker engine. Each
// one skips when no engine is reachable, so the suite still passes on a
// machine without Docker. Every object a test creates has a unique name and
// is removed by name when the test ends, even when the test fails.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/example/plugin/docker/client"
	"github.com/jumppad-labs/xcl/types"
)

// testImage is the image the container tests run
const testImage = "nginx:1.27-alpine"

// requireDocker skips the test when no Docker engine is reachable
func requireDocker(t *testing.T) {
	t.Helper()

	if err := client.Ping(context.Background()); err != nil {
		t.Skip(err.Error()) // Ping already says no Docker engine is reachable
	}
}

// newDockerClient returns a real Docker client, closed when the test ends
func newDockerClient(t *testing.T) client.Docker {
	t.Helper()

	c, err := client.New()
	require.NoError(t, err)

	if closer, ok := c.(interface{ Close() error }); ok {
		t.Cleanup(func() { closer.Close() })
	}

	return c
}

// uniqueName returns a Docker object name no other test run uses
func uniqueName(t *testing.T) string {
	t.Helper()

	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	require.NoError(t, err)

	return "xcl-example-test-" + hex.EncodeToString(suffix)
}

// removeNetworkOnCleanup removes the network called name when the test ends,
// a network that is already gone is ignored
func removeNetworkOnCleanup(t *testing.T, c client.Docker, name string) {
	t.Helper()

	t.Cleanup(func() {
		err := c.NetworkRemove(context.Background(), name)
		if err != nil && !dockerclient.IsErrNotFound(err) {
			t.Errorf("unable to remove test network %s: %s", name, err)
		}
	})
}

// removeContainerOnCleanup removes the container called name when the test
// ends, a container that is already gone is ignored
func removeContainerOnCleanup(t *testing.T, c client.Docker, name string) {
	t.Helper()

	t.Cleanup(func() {
		err := c.ContainerRemove(context.Background(), name, container.RemoveOptions{Force: true})
		if err != nil && !dockerclient.IsErrNotFound(err) {
			t.Errorf("unable to remove test container %s: %s", name, err)
		}
	})
}

func integrationNetwork(name string) *Network {
	return &Network{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "docker.network." + name,
				Name: name,
				Type: "docker",
			},
		},
	}
}

func integrationContainer(name string, networkName string) *Container {
	return &Container{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "docker.container." + name,
				Name: name,
				Type: "docker",
			},
		},
		Image: testImage,
		Networks: []NetworkAttachment{
			{Name: networkName},
		},
	}
}

func TestNetworkCreateMakesANetworkVisibleInDocker(t *testing.T) {
	requireDocker(t)

	c := newDockerClient(t)
	name := uniqueName(t)
	removeNetworkOnCleanup(t, c, name)

	provider := &networkProvider{client: c}

	created, err := provider.Create(context.Background(), integrationNetwork(name))
	require.NoError(t, err)
	require.NotEmpty(t, created.DockerID)

	inspect, err := c.NetworkInspect(context.Background(), name, network.InspectOptions{})
	require.NoError(t, err)
	require.Equal(t, name, inspect.Name)
	require.Equal(t, "xcl-example-plugin", inspect.Labels["created_by"])
	require.Equal(t, "docker.network."+name, inspect.Labels["xcl_id"])
}

func TestNetworkDestroyRemovesItFromDocker(t *testing.T) {
	requireDocker(t)

	c := newDockerClient(t)
	name := uniqueName(t)
	removeNetworkOnCleanup(t, c, name)

	provider := &networkProvider{client: c}

	created, err := provider.Create(context.Background(), integrationNetwork(name))
	require.NoError(t, err)

	err = provider.Destroy(context.Background(), created, false)
	require.NoError(t, err)

	_, err = c.NetworkInspect(context.Background(), name, network.InspectOptions{})
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected not found, got: %s", err)
}

func TestContainerCreateRunsAContainerAttachedToTheNetwork(t *testing.T) {
	requireDocker(t)

	c := newDockerClient(t)
	networkName := uniqueName(t)
	containerName := uniqueName(t)

	// cleanups run last registered first, so the container goes before the
	// network it is attached to
	removeNetworkOnCleanup(t, c, networkName)
	removeContainerOnCleanup(t, c, containerName)

	networks := &networkProvider{client: c}
	_, err := networks.Create(context.Background(), integrationNetwork(networkName))
	require.NoError(t, err)

	containers := &containerProvider{client: c}
	created, err := containers.Create(context.Background(), integrationContainer(containerName, networkName))
	require.NoError(t, err)
	require.NotEmpty(t, created.DockerID)
	require.NotEmpty(t, created.IPAddress)

	inspect, err := c.ContainerInspect(context.Background(), containerName)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
	require.NotNil(t, inspect.NetworkSettings)
	require.Contains(t, inspect.NetworkSettings.Networks, networkName)
}

func TestContainerDestroyRemovesItFromDocker(t *testing.T) {
	requireDocker(t)

	c := newDockerClient(t)
	networkName := uniqueName(t)
	containerName := uniqueName(t)

	removeNetworkOnCleanup(t, c, networkName)
	removeContainerOnCleanup(t, c, containerName)

	networks := &networkProvider{client: c}
	_, err := networks.Create(context.Background(), integrationNetwork(networkName))
	require.NoError(t, err)

	containers := &containerProvider{client: c}
	created, err := containers.Create(context.Background(), integrationContainer(containerName, networkName))
	require.NoError(t, err)

	err = containers.Destroy(context.Background(), created, false)
	require.NoError(t, err)

	_, err = c.ContainerInspect(context.Background(), containerName)
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected not found, got: %s", err)
}
