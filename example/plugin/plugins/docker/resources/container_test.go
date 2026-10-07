package resources

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/mocks"
	"github.com/jumppad-labs/xcl/types"
)

func testContainer() *Container {
	return &Container{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "docker.container.web",
				Name: "web",
				Type: "docker",
			},
		},
		Image: "nginx:1.27",
		Networks: []NetworkAttachment{
			{Name: "app", Aliases: []string{"web.local"}},
		},
	}
}

// emptyPullProgress returns a pull progress stream that ends straight away
func emptyPullProgress() io.ReadCloser {
	return io.NopCloser(strings.NewReader(""))
}

// expectImagePresent makes Docker report that it already has the image
func expectImagePresent(client *mocks.MockDocker) {
	client.EXPECT().
		ImageList(mock.Anything, mock.Anything).
		Return([]image.Summary{{ID: "sha256:abc"}}, nil).
		Once()
}

// expectContainerCreated makes ContainerCreate succeed with the ID "ctr-123"
func expectContainerCreated(client *mocks.MockDocker) {
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()
}

// expectContainerStarted makes ContainerStart succeed
func expectContainerStarted(client *mocks.MockDocker) {
	client.EXPECT().
		ContainerStart(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
}

// expectContainerInspected makes ContainerInspect report the address
// 10.42.0.5 on the network "app"
func expectContainerInspected(client *mocks.MockDocker) {
	client.EXPECT().
		ContainerInspect(mock.Anything, mock.Anything).
		Return(container.InspectResponse{
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"app": {IPAddress: "10.42.0.5"},
				},
			},
		}, nil).
		Once()
}

func TestContainerCreatePullsAMissingImage(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, image.ListOptions{
			Filters: filters.NewArgs(filters.Arg("reference", "nginx:1.27")),
		}).
		Return([]image.Summary{}, nil).
		Once()
	client.EXPECT().
		ImagePull(mock.Anything, "nginx:1.27", image.PullOptions{}).
		Return(emptyPullProgress(), nil).
		Once()
	expectContainerCreated(client)
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
}

func TestContainerCreateSkipsThePullForAPresentImage(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, image.ListOptions{
			Filters: filters.NewArgs(filters.Arg("reference", "nginx:1.27")),
		}).
		Return([]image.Summary{{ID: "sha256:abc"}}, nil).
		Once()
	expectContainerCreated(client)
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
	client.AssertNotCalled(t, "ImagePull", mock.Anything, mock.Anything, mock.Anything)
}

func TestContainerCreateUsesTheBlocksNameImageAndLabels(t *testing.T) {
	var gotConfig *container.Config
	var gotName string

	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, &container.HostConfig{}, mock.Anything, (*v1.Platform)(nil), "web").
		Run(func(_ context.Context, config *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *v1.Platform, name string) {
			gotConfig = config
			gotName = name
		}).
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}
	c := testContainer()
	c.Command = []string{"nginx", "-g", "daemon off;"}
	c.Environment = map[string]string{
		"ZONE":  "eu",
		"APP":   "web",
		"DEBUG": "true",
	}

	_, err := provider.Create(context.Background(), c)

	require.NoError(t, err)
	require.Equal(t, "web", gotName)
	require.Equal(t, &container.Config{
		Image: "nginx:1.27",
		Cmd:   []string{"nginx", "-g", "daemon off;"},
		Env:   []string{"APP=web", "DEBUG=true", "ZONE=eu"},
		Labels: map[string]string{
			"created_by": "xcl-example-plugin",
			"xcl_id":     "docker.container.web",
		},
	}, gotConfig)
}

func TestContainerCreateAttachesTheFirstNetworkWithAliases(t *testing.T) {
	var gotNetworking *network.NetworkingConfig

	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *container.Config, _ *container.HostConfig, networking *network.NetworkingConfig, _ *v1.Platform, _ string) {
			gotNetworking = networking
		}).
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
	require.Equal(t, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"app": {Aliases: []string{"web.local"}},
		},
	}, gotNetworking)
}

func TestContainerCreateConnectsFurtherNetworks(t *testing.T) {
	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	expectContainerCreated(client)
	client.EXPECT().
		NetworkConnect(mock.Anything, "backend", "ctr-123", &network.EndpointSettings{Aliases: []string{"api"}}).
		Return(nil).
		Once()
	client.EXPECT().
		NetworkConnect(mock.Anything, "metrics", "ctr-123", &network.EndpointSettings{}).
		Return(nil).
		Once()
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}
	c := testContainer()
	c.Networks = append(c.Networks,
		NetworkAttachment{Name: "backend", Aliases: []string{"api"}},
		NetworkAttachment{Name: "metrics"},
	)

	_, err := provider.Create(context.Background(), c)

	require.NoError(t, err)
}

func TestContainerCreateConnectsFurtherNetworksBeforeStarting(t *testing.T) {
	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	expectContainerCreated(client)
	connect := client.EXPECT().
		NetworkConnect(mock.Anything, "backend", "ctr-123", mock.Anything).
		Return(nil).
		Once()
	start := client.EXPECT().
		ContainerStart(mock.Anything, "ctr-123", mock.Anything).
		Return(nil).
		Once()
	mock.InOrder(connect, start)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}
	c := testContainer()
	c.Networks = append(c.Networks, NetworkAttachment{Name: "backend"})

	_, err := provider.Create(context.Background(), c)

	require.NoError(t, err)
}

func TestContainerCreateStartsTheContainer(t *testing.T) {
	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	expectContainerCreated(client)
	client.EXPECT().
		ContainerStart(mock.Anything, "ctr-123", container.StartOptions{}).
		Return(nil).
		Once()
	expectContainerInspected(client)

	provider := &containerProvider{client: client}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
}

func TestContainerCreateFillsTheDockerIDAndAddress(t *testing.T) {
	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	expectContainerCreated(client)
	expectContainerStarted(client)
	client.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"bridge": {IPAddress: "172.17.0.2"},
					"app":    {IPAddress: "10.42.0.5"},
				},
			},
		}, nil).
		Once()

	provider := &containerProvider{client: client}

	created, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
	require.Equal(t, "ctr-123", created.DockerID)
	require.Equal(t, "10.42.0.5", created.IPAddress)
}

func TestContainerCreateReturnsPullErrors(t *testing.T) {
	dockerErr := errors.New("pull access denied")

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, mock.Anything).
		Return([]image.Summary{}, nil).
		Once()
	client.EXPECT().
		ImagePull(mock.Anything, "nginx:1.27", mock.Anything).
		Return(nil, dockerErr).
		Once()

	provider := &containerProvider{client: client}

	created, err := provider.Create(context.Background(), testContainer())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "nginx:1.27")
	client.AssertNotCalled(t, "ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestContainerCreateReturnsCreateErrors(t *testing.T) {
	dockerErr := errors.New("name already in use")

	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.CreateResponse{}, dockerErr).
		Once()

	provider := &containerProvider{client: client}

	created, err := provider.Create(context.Background(), testContainer())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "web")
	client.AssertNotCalled(t, "ContainerRemove", mock.Anything, mock.Anything, mock.Anything)
}

func TestContainerCreateRemovesTheContainerWhenStartFails(t *testing.T) {
	dockerErr := errors.New("port is already allocated")

	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	expectContainerCreated(client)
	client.EXPECT().
		ContainerStart(mock.Anything, "ctr-123", mock.Anything).
		Return(dockerErr).
		Once()
	client.EXPECT().
		ContainerRemove(mock.Anything, "ctr-123", container.RemoveOptions{Force: true}).
		Return(nil).
		Once()

	provider := &containerProvider{client: client}

	created, err := provider.Create(context.Background(), testContainer())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "web")
}

func TestContainerDestroyStopsAndRemovesTheContainer(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStop(mock.Anything, "ctr-123", container.StopOptions{}).
		Return(nil).
		Once()
	client.EXPECT().
		ContainerRemove(mock.Anything, "ctr-123", container.RemoveOptions{Force: true}).
		Return(nil).
		Once()

	provider := &containerProvider{client: client}
	c := testContainer()
	c.DockerID = "ctr-123"

	err := provider.Destroy(context.Background(), c, false)

	require.NoError(t, err)
}

func TestContainerDestroySucceedsWhenTheContainerIsGone(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStop(mock.Anything, "ctr-123", mock.Anything).
		Return(errdefs.NotFound(errors.New("no such container"))).
		Once()

	provider := &containerProvider{client: client}
	c := testContainer()
	c.DockerID = "ctr-123"

	err := provider.Destroy(context.Background(), c, false)

	require.NoError(t, err)
	client.AssertNotCalled(t, "ContainerRemove", mock.Anything, mock.Anything, mock.Anything)
}

func TestContainerDestroyReturnsDockerErrors(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStop(mock.Anything, "ctr-123", mock.Anything).
		Return(dockerErr).
		Once()

	provider := &containerProvider{client: client}
	c := testContainer()
	c.DockerID = "ctr-123"

	err := provider.Destroy(context.Background(), c, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "web")
}

func TestContainerReadKeepsTheDockerIDAndAddressFromTheSavedContainer(t *testing.T) {
	provider := &containerProvider{client: mocks.NewMockDocker(t)}

	saved := testContainer()
	saved.DockerID = "ctr-123"
	saved.IPAddress = "10.42.0.5"
	configured := testContainer()

	got, err := provider.Read(context.Background(), saved, configured)

	require.NoError(t, err)
	require.Equal(t, "ctr-123", got.DockerID)
	require.Equal(t, "10.42.0.5", got.IPAddress)
}
