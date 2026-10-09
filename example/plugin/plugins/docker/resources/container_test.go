package resources

import (
	"context"
	"errors"
	"io"
	"os"
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

	"github.com/jumppad-labs/xcl/entity"
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

func TestContainerCreateMountsTheInitScriptReadOnlyByItsAbsolutePath(t *testing.T) {
	var gotHostConfig *container.HostConfig

	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *v1.Platform, _ string) {
			gotHostConfig = hostConfig
		}).
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}
	c := testContainer()
	c.InitScript = "build/rendered/init.sh"

	_, err := provider.Create(context.Background(), c)
	require.NoError(t, err)

	workingDir, err := os.Getwd()
	require.NoError(t, err)

	expectedBind := workingDir + "/build/rendered/init.sh:/docker-entrypoint.d/90-xcl-init.sh:ro"
	require.Equal(t, &container.HostConfig{Binds: []string{expectedBind}}, gotHostConfig)
}

func TestContainerCreateWithoutAnInitScriptPassesAnEmptyHostConfig(t *testing.T) {
	var gotHostConfig *container.HostConfig

	client := mocks.NewMockDocker(t)
	expectImagePresent(client)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *v1.Platform, _ string) {
			gotHostConfig = hostConfig
		}).
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()
	expectContainerStarted(client)
	expectContainerInspected(client)

	provider := &containerProvider{client: client}

	_, err := provider.Create(context.Background(), testContainer())
	require.NoError(t, err)

	require.Equal(t, &container.HostConfig{}, gotHostConfig)
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

// networkPath returns the path of a container's network block at index
func networkPath(index int) entity.Path {
	return entity.Path{}.Attribute("network").Index(index)
}

// expectContainerInspectedOnBackend makes ContainerInspect report the address
// 10.43.0.7 on the network "backend"
func expectContainerInspectedOnBackend(client *mocks.MockDocker) {
	client.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"backend": {IPAddress: "10.43.0.7"},
				},
			},
		}, nil).
		Once()
}

// savedContainer returns the test container as Update receives it, created
// with the Docker ID "ctr-123" and the address 10.42.0.5
func savedContainer() *Container {
	c := testContainer()
	c.DockerID = "ctr-123"
	c.IPAddress = "10.42.0.5"
	return c
}

func TestContainerChangedReplacesOnImageChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	new := testContainer()
	new.Image = "nginx:1.28"
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.27", After: "nginx:1.28"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestContainerChangedReplacesOnCommandChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.Command = []string{"nginx", "-g", "daemon off;"}
	new := testContainer()
	new.Command = []string{"nginx", "-g", "daemon on;"}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("command").Index(2), Before: "daemon off;", After: "daemon on;"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestContainerChangedReplacesOnEnvironmentChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.Environment = map[string]string{"MODE": "dev"}
	new := testContainer()
	new.Environment = map[string]string{"MODE": "prod"}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("environment").Key("MODE"), Before: "dev", After: "prod"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestContainerChangedUpdatesOnNetworkNameChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	new := testContainer()
	new.Networks = []NetworkAttachment{
		{Name: "backend", Aliases: []string{"web.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("name"), Before: "app", After: "backend"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestContainerChangedUpdatesOnNetworkAliasesChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	new := testContainer()
	new.Networks = []NetworkAttachment{
		{Name: "app", Aliases: []string{"www.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("aliases").Index(0), Before: "web.local", After: "www.local"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestContainerChangedUpdatesWhenNetworkIsReplaced(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	new := testContainer()
	dependencies := []entity.DependencyChange{
		{Address: "docker.network.app", Change: entity.Replace},
	}

	change, err := p.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestContainerChangedUpdatesWhenNetworkIsOnlyUpdated(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	new := testContainer()
	dependencies := []entity.DependencyChange{
		{Address: "docker.network.app", Change: entity.Update},
	}

	change, err := p.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestContainerChangedReplacesWhenANonNetworkDependencyIsReplaced(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	new := testContainer()
	dependencies := []entity.DependencyChange{
		{Address: "template.init", Change: entity.Replace},
	}

	change, err := p.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestContainerChangedReplacesOnInitScriptChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.InitScript = "build/rendered/init.sh"
	new := testContainer()
	new.InitScript = "build/rendered/start.sh"
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("init_script"), Before: "build/rendered/init.sh", After: "build/rendered/start.sh"},
	}

	change, err := p.Changed(context.Background(), old, new, changes, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestContainerChangedReportsNoChangeWhenTheInitScriptTemplateIsOnlyUpdated(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.InitScript = "build/rendered/init.sh"
	new := testContainer()
	new.InitScript = "build/rendered/init.sh"
	dependencies := []entity.DependencyChange{
		{Address: "template.init", Change: entity.Update},
	}

	// the template rendered new content to the same path, the mounted file
	// changes under the running container, so nothing needs doing
	change, err := p.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

func TestContainerChangedUpdatesWhenTheAppNetworkDependencyIsUpdated(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.InitScript = "build/rendered/init.sh"
	new := testContainer()
	new.InitScript = "build/rendered/init.sh"
	dependencies := []entity.DependencyChange{
		{Address: "docker.network.app", Change: entity.Update},
		{Address: "template.init", Change: entity.Update},
	}

	change, err := p.Changed(context.Background(), old, new, nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestContainerChangedReportsNoChangeForIdenticalContainer(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.Command = []string{"nginx"}
	old.Environment = map[string]string{"MODE": "dev"}
	new := testContainer()
	new.Command = []string{"nginx"}
	new.Environment = map[string]string{"MODE": "dev"}

	// nothing reported, so the decision is left to DefaultChanged
	change, err := p.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

func TestContainerChangedTreatsEmptyAndNilCommandAsEqual(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockDocker(t))

	old := testContainer()
	old.Command = nil
	new := testContainer()
	new.Command = []string{}

	change, err := p.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

// The Update tests use a strict mock: any Docker call without an expectation
// fails the test. Update is only permitted to disconnect, connect and, once
// the container has moved, inspect it for its new address. No test expects a
// ContainerInspect, NetworkInspect or ContainerList before a move, so Update
// works only from the changes and dependencies it is told about, never from
// the container's previous state in Docker.

func TestContainerUpdateMovesTheContainerToTheRenamedNetwork(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(nil).
		Once()
	dockerClient.EXPECT().
		NetworkConnect(mock.Anything, "backend", "ctr-123", &network.EndpointSettings{Aliases: []string{"web.local"}}).
		Return(nil).
		Once()
	expectContainerInspectedOnBackend(dockerClient)

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = []NetworkAttachment{
		{Name: "backend", Aliases: []string{"web.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("name"), Before: "app", After: "backend"},
	}

	got, err := p.Update(context.Background(), c, changes, nil)
	require.NoError(t, err)
	require.Equal(t, "10.43.0.7", got.IPAddress)
}

func TestContainerUpdateReattachesTheContainerToAReplacedNetwork(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkConnect(mock.Anything, "app", "ctr-123", &network.EndpointSettings{Aliases: []string{"web.local"}}).
		Return(nil).
		Once()
	dockerClient.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"app": {IPAddress: "10.42.1.9"},
				},
			},
		}, nil).
		Once()

	p := NewContainerProvider(dockerClient)

	dependencies := []entity.DependencyChange{
		{Address: "docker.network.app", Change: entity.Replace},
	}

	got, err := p.Update(context.Background(), savedContainer(), nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, "10.42.1.9", got.IPAddress)
}

func TestContainerUpdateDetachesTheContainerFromItsRemovedOnlyNetwork(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(nil).
		Once()
	dockerClient.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{},
			},
		}, nil).
		Once()

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = nil
	changes := []entity.PropertyChange{
		{Path: networkPath(0), Before: map[string]any{"name": "app"}, After: nil},
	}

	got, err := p.Update(context.Background(), c, changes, nil)
	require.NoError(t, err)
	require.Empty(t, got.IPAddress)
}

func TestContainerUpdateConnectsOnlyAnAddedNetwork(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkConnect(mock.Anything, "backend", "ctr-123", &network.EndpointSettings{Aliases: []string{"api"}}).
		Return(nil).
		Once()
	expectContainerInspected(dockerClient)

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = []NetworkAttachment{
		{Name: "app", Aliases: []string{"web.local"}},
		{Name: "backend", Aliases: []string{"api"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(1), Before: nil, After: map[string]any{"name": "backend", "aliases": []any{"api"}}},
	}

	got, err := p.Update(context.Background(), c, changes, nil)
	require.NoError(t, err)
	require.Equal(t, "10.42.0.5", got.IPAddress)
}

func TestContainerUpdateReconnectsWithTheNewAliases(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	disconnect := dockerClient.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(nil).
		Once()
	dockerClient.EXPECT().
		NetworkConnect(mock.Anything, "app", "ctr-123", &network.EndpointSettings{Aliases: []string{"web.local", "www.local"}}).
		Return(nil).
		Once().
		NotBefore(disconnect)
	expectContainerInspected(dockerClient)

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = []NetworkAttachment{
		{Name: "app", Aliases: []string{"web.local", "www.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("aliases").Index(1), Before: nil, After: "www.local"},
	}

	got, err := p.Update(context.Background(), c, changes, nil)
	require.NoError(t, err)
	require.Equal(t, "10.42.0.5", got.IPAddress)
}

func TestContainerUpdateIgnoresANetworkThatIsAlreadyGoneOnDisconnect(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(errdefs.NotFound(errors.New("no such network"))).
		Once()
	dockerClient.EXPECT().
		NetworkConnect(mock.Anything, "backend", "ctr-123", &network.EndpointSettings{Aliases: []string{"web.local"}}).
		Return(nil).
		Once()
	expectContainerInspectedOnBackend(dockerClient)

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = []NetworkAttachment{
		{Name: "backend", Aliases: []string{"web.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("name"), Before: "app", After: "backend"},
	}

	got, err := p.Update(context.Background(), c, changes, nil)
	require.NoError(t, err)
	require.Equal(t, "10.43.0.7", got.IPAddress)
}

func TestContainerUpdateReturnsDisconnectErrors(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(errors.New("daemon unavailable")).
		Once()

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = []NetworkAttachment{
		{Name: "backend", Aliases: []string{"web.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("name"), Before: "app", After: "backend"},
	}

	_, err := p.Update(context.Background(), c, changes, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "unable to disconnect container web from network app")
	require.ErrorContains(t, err, "daemon unavailable")
}

func TestContainerUpdateReturnsConnectErrors(t *testing.T) {
	dockerClient := mocks.NewMockDocker(t)
	dockerClient.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(nil).
		Once()
	dockerClient.EXPECT().
		NetworkConnect(mock.Anything, "backend", "ctr-123", mock.Anything).
		Return(errors.New("network backend not found")).
		Once()

	p := NewContainerProvider(dockerClient)

	c := savedContainer()
	c.Networks = []NetworkAttachment{
		{Name: "backend", Aliases: []string{"web.local"}},
	}
	changes := []entity.PropertyChange{
		{Path: networkPath(0).Attribute("name"), Before: "app", After: "backend"},
	}

	_, err := p.Update(context.Background(), c, changes, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "unable to connect container web to network backend")
}

func TestContainerUpdateWithNothingToldMakesNoDockerCalls(t *testing.T) {
	// a strict mock with no expectations: any Docker call fails the test
	p := NewContainerProvider(mocks.NewMockDocker(t))

	got, err := p.Update(context.Background(), savedContainer(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, "ctr-123", got.DockerID)
	require.Equal(t, "10.42.0.5", got.IPAddress)
}
