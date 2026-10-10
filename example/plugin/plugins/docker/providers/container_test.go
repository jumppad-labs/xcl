package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers/mocks"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/types"
)

func testContainer() *entities.Container {
	return &entities.Container{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				ID:   "docker.container.web",
				Name: "web",
				Type: "docker",
			},
		},
		Image: "nginx:1.27",
		Networks: []entities.NetworkAttachment{
			{Name: "app", Aliases: []string{"web.local"}},
		},
	}
}

// expectImagePresent makes the image task succeed for the test image
func expectImagePresent(tasks *mocks.MockTasks) {
	tasks.EXPECT().
		EnsureImage(mock.Anything, "nginx:1.27").
		Return(nil).
		Once()
}

// expectContainerCreated makes CreateContainer succeed with the ID "ctr-123"
func expectContainerCreated(tasks *mocks.MockTasks) {
	tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Return("ctr-123", nil).
		Once()
}

// expectContainerStarted makes StartContainer succeed for "ctr-123"
func expectContainerStarted(tasks *mocks.MockTasks) {
	tasks.EXPECT().
		StartContainer(mock.Anything, "ctr-123").
		Return(nil).
		Once()
}

// expectContainerInspected makes ContainerAddresses report the address
// 10.42.0.5 on the network "app"
func expectContainerInspected(tasks *mocks.MockTasks) {
	tasks.EXPECT().
		ContainerAddresses(mock.Anything, "ctr-123").
		Return(map[string]string{"app": "10.42.0.5"}, nil).
		Once()
}

// The image lookup and pull themselves now live in client/containers, whose
// tests cover pulling a missing image and skipping the pull for a present
// one. The provider only asks for the image to be ensured.

func TestContainerCreatePullsAMissingImage(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		EnsureImage(mock.Anything, "nginx:1.27").
		Return(nil).
		Once()
	expectContainerCreated(tasks)
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
}

func TestContainerCreateSkipsThePullForAPresentImage(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	ensure := tasks.EXPECT().
		EnsureImage(mock.Anything, "nginx:1.27").
		Return(nil).
		Once()
	create := tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Return("ctr-123", nil).
		Once()
	mock.InOrder(ensure, create)
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
}

func TestContainerCreateUsesTheBlocksNameImageAndLabels(t *testing.T) {
	var gotSpec containers.ContainerSpec

	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Run(func(_ context.Context, spec containers.ContainerSpec) {
			gotSpec = spec
		}).
		Return("ctr-123", nil).
		Once()
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.Command = []string{"nginx", "-g", "daemon off;"}
	c.Environment = map[string]string{
		"ZONE":  "eu",
		"APP":   "web",
		"DEBUG": "true",
	}

	_, err := provider.Create(context.Background(), c)

	// sorting the environment into KEY=VALUE now lives in client/containers
	require.NoError(t, err)
	require.Equal(t, containers.ContainerSpec{
		Name:    "web",
		Image:   "nginx:1.27",
		Command: []string{"nginx", "-g", "daemon off;"},
		Environment: map[string]string{
			"ZONE":  "eu",
			"APP":   "web",
			"DEBUG": "true",
		},
		Labels: map[string]string{
			"created_by": "xcl-example-plugin",
			"xcl_id":     "docker.container.web",
		},
		Network: &containers.Attachment{Name: "app", Aliases: []string{"web.local"}},
	}, gotSpec)
}

func TestContainerCreateAttachesTheFirstNetworkWithAliases(t *testing.T) {
	var gotSpec containers.ContainerSpec

	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Run(func(_ context.Context, spec containers.ContainerSpec) {
			gotSpec = spec
		}).
		Return("ctr-123", nil).
		Once()
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
	require.Equal(t, &containers.Attachment{Name: "app", Aliases: []string{"web.local"}}, gotSpec.Network)
}

func TestContainerCreateMountsTheInitScriptReadOnlyByItsAbsolutePath(t *testing.T) {
	var gotSpec containers.ContainerSpec

	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Run(func(_ context.Context, spec containers.ContainerSpec) {
			gotSpec = spec
		}).
		Return("ctr-123", nil).
		Once()
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.InitScript = "build/rendered/init.sh"

	_, err := provider.Create(context.Background(), c)
	require.NoError(t, err)

	workingDir, err := os.Getwd()
	require.NoError(t, err)

	// the read-only bind of this path into the image now lives in
	// client/containers, the provider passes the script's absolute path
	require.Equal(t, filepath.Join(workingDir, "build/rendered/init.sh"), gotSpec.InitScript)
}

func TestContainerCreateWithoutAnInitScriptPassesAnEmptyHostConfig(t *testing.T) {
	var gotSpec containers.ContainerSpec

	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Run(func(_ context.Context, spec containers.ContainerSpec) {
			gotSpec = spec
		}).
		Return("ctr-123", nil).
		Once()
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}

	_, err := provider.Create(context.Background(), testContainer())
	require.NoError(t, err)

	// client/containers turns an empty init script into an empty host config
	require.Empty(t, gotSpec.InitScript)
}

func TestContainerCreateConnectsFurtherNetworks(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	expectContainerCreated(tasks)
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "backend", "ctr-123", []string{"api"}).
		Return(nil).
		Once()
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "metrics", "ctr-123", []string(nil)).
		Return(nil).
		Once()
	expectContainerStarted(tasks)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.Networks = append(c.Networks,
		entities.NetworkAttachment{Name: "backend", Aliases: []string{"api"}},
		entities.NetworkAttachment{Name: "metrics"},
	)

	_, err := provider.Create(context.Background(), c)

	require.NoError(t, err)
}

func TestContainerCreateConnectsFurtherNetworksBeforeStarting(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	expectContainerCreated(tasks)
	connect := tasks.EXPECT().
		ConnectNetwork(mock.Anything, "backend", "ctr-123", mock.Anything).
		Return(nil).
		Once()
	start := tasks.EXPECT().
		StartContainer(mock.Anything, "ctr-123").
		Return(nil).
		Once()
	mock.InOrder(connect, start)
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.Networks = append(c.Networks, entities.NetworkAttachment{Name: "backend"})

	_, err := provider.Create(context.Background(), c)

	require.NoError(t, err)
}

func TestContainerCreateStartsTheContainer(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	expectContainerCreated(tasks)
	tasks.EXPECT().
		StartContainer(mock.Anything, "ctr-123").
		Return(nil).
		Once()
	expectContainerInspected(tasks)

	provider := &containerProvider{tasks: tasks}

	_, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
}

func TestContainerCreateFillsTheDockerIDAndAddress(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	expectContainerCreated(tasks)
	expectContainerStarted(tasks)
	tasks.EXPECT().
		ContainerAddresses(mock.Anything, "ctr-123").
		Return(map[string]string{
			"bridge": "172.17.0.2",
			"app":    "10.42.0.5",
		}, nil).
		Once()

	provider := &containerProvider{tasks: tasks}

	created, err := provider.Create(context.Background(), testContainer())

	require.NoError(t, err)
	require.Equal(t, "ctr-123", created.DockerID)
	require.Equal(t, "10.42.0.5", created.IPAddress)
}

func TestContainerCreateReturnsPullErrors(t *testing.T) {
	dockerErr := errors.New("pull access denied")

	// the task layer names the image in its pull error
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		EnsureImage(mock.Anything, "nginx:1.27").
		Return(fmt.Errorf("unable to pull image nginx:1.27: %w", dockerErr)).
		Once()

	provider := &containerProvider{tasks: tasks}

	created, err := provider.Create(context.Background(), testContainer())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "nginx:1.27")
	tasks.AssertNotCalled(t, "CreateContainer", mock.Anything, mock.Anything)
}

func TestContainerCreateReturnsCreateErrors(t *testing.T) {
	dockerErr := errors.New("name already in use")

	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	tasks.EXPECT().
		CreateContainer(mock.Anything, mock.Anything).
		Return("", dockerErr).
		Once()

	provider := &containerProvider{tasks: tasks}

	created, err := provider.Create(context.Background(), testContainer())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "web")
	tasks.AssertNotCalled(t, "RemoveContainer", mock.Anything, mock.Anything)
}

func TestContainerCreateRemovesTheContainerWhenStartFails(t *testing.T) {
	dockerErr := errors.New("port is already allocated")

	tasks := mocks.NewMockTasks(t)
	expectImagePresent(tasks)
	expectContainerCreated(tasks)
	tasks.EXPECT().
		StartContainer(mock.Anything, "ctr-123").
		Return(dockerErr).
		Once()
	tasks.EXPECT().
		RemoveContainer(mock.Anything, "ctr-123").
		Return(nil).
		Once()

	provider := &containerProvider{tasks: tasks}

	created, err := provider.Create(context.Background(), testContainer())

	require.Nil(t, created)
	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "web")
}

func TestContainerDestroyStopsAndRemovesTheContainer(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	stop := tasks.EXPECT().
		StopContainer(mock.Anything, "ctr-123").
		Return(nil).
		Once()
	remove := tasks.EXPECT().
		RemoveContainer(mock.Anything, "ctr-123").
		Return(nil).
		Once()
	mock.InOrder(stop, remove)

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.DockerID = "ctr-123"

	err := provider.Destroy(context.Background(), c, false)

	require.NoError(t, err)
}

func TestContainerDestroySucceedsWhenTheContainerIsGone(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		StopContainer(mock.Anything, "ctr-123").
		Return(fmt.Errorf("no such container: %w", containers.ErrNotFound)).
		Once()

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.DockerID = "ctr-123"

	err := provider.Destroy(context.Background(), c, false)

	require.NoError(t, err)
	tasks.AssertNotCalled(t, "RemoveContainer", mock.Anything, mock.Anything)
}

func TestContainerDestroyReturnsDockerErrors(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")

	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		StopContainer(mock.Anything, "ctr-123").
		Return(dockerErr).
		Once()

	provider := &containerProvider{tasks: tasks}
	c := testContainer()
	c.DockerID = "ctr-123"

	err := provider.Destroy(context.Background(), c, false)

	require.ErrorIs(t, err, dockerErr)
	require.ErrorContains(t, err, "web")
}

func TestContainerReadKeepsTheDockerIDAndAddressFromTheSavedContainer(t *testing.T) {
	provider := &containerProvider{tasks: mocks.NewMockTasks(t)}

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

// expectContainerInspectedOnBackend makes ContainerAddresses report the
// address 10.43.0.7 on the network "backend"
func expectContainerInspectedOnBackend(tasks *mocks.MockTasks) {
	tasks.EXPECT().
		ContainerAddresses(mock.Anything, "ctr-123").
		Return(map[string]string{"backend": "10.43.0.7"}, nil).
		Once()
}

// savedContainer returns the test container as Update receives it, created
// with the Docker ID "ctr-123" and the address 10.42.0.5
func savedContainer() *entities.Container {
	c := testContainer()
	c.DockerID = "ctr-123"
	c.IPAddress = "10.42.0.5"
	return c
}

func TestContainerChangedReplacesOnImageChange(t *testing.T) {
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

	old := testContainer()
	new := testContainer()
	new.Networks = []entities.NetworkAttachment{
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
	p := NewContainerProvider(mocks.NewMockTasks(t))

	old := testContainer()
	new := testContainer()
	new.Networks = []entities.NetworkAttachment{
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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

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
	p := NewContainerProvider(mocks.NewMockTasks(t))

	old := testContainer()
	old.Command = nil
	new := testContainer()
	new.Command = []string{}

	change, err := p.Changed(context.Background(), old, new, nil, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

// The Update tests use a strict mock: any task call without an expectation
// fails the test. Update is only permitted to disconnect, connect and, once
// the container has moved, read its addresses. No test expects a
// ContainerAddresses or NetworkContainers call before a move, so Update
// works only from the changes and dependencies it is told about, never from
// the container's previous state in Docker.

func TestContainerUpdateMovesTheContainerToTheRenamedNetwork(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		DisconnectNetwork(mock.Anything, "app", "ctr-123").
		Return(nil).
		Once()
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "backend", "ctr-123", []string{"web.local"}).
		Return(nil).
		Once()
	expectContainerInspectedOnBackend(tasks)

	p := NewContainerProvider(tasks)

	c := savedContainer()
	c.Networks = []entities.NetworkAttachment{
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
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "app", "ctr-123", []string{"web.local"}).
		Return(nil).
		Once()
	tasks.EXPECT().
		ContainerAddresses(mock.Anything, "ctr-123").
		Return(map[string]string{"app": "10.42.1.9"}, nil).
		Once()

	p := NewContainerProvider(tasks)

	dependencies := []entity.DependencyChange{
		{Address: "docker.network.app", Change: entity.Replace},
	}

	got, err := p.Update(context.Background(), savedContainer(), nil, dependencies)
	require.NoError(t, err)
	require.Equal(t, "10.42.1.9", got.IPAddress)
}

func TestContainerUpdateDetachesTheContainerFromItsRemovedOnlyNetwork(t *testing.T) {
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		DisconnectNetwork(mock.Anything, "app", "ctr-123").
		Return(nil).
		Once()
	tasks.EXPECT().
		ContainerAddresses(mock.Anything, "ctr-123").
		Return(map[string]string{}, nil).
		Once()

	p := NewContainerProvider(tasks)

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
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "backend", "ctr-123", []string{"api"}).
		Return(nil).
		Once()
	expectContainerInspected(tasks)

	p := NewContainerProvider(tasks)

	c := savedContainer()
	c.Networks = []entities.NetworkAttachment{
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
	tasks := mocks.NewMockTasks(t)
	disconnect := tasks.EXPECT().
		DisconnectNetwork(mock.Anything, "app", "ctr-123").
		Return(nil).
		Once()
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "app", "ctr-123", []string{"web.local", "www.local"}).
		Return(nil).
		Once().
		NotBefore(disconnect)
	expectContainerInspected(tasks)

	p := NewContainerProvider(tasks)

	c := savedContainer()
	c.Networks = []entities.NetworkAttachment{
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
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		DisconnectNetwork(mock.Anything, "app", "ctr-123").
		Return(fmt.Errorf("no such network: %w", containers.ErrNotFound)).
		Once()
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "backend", "ctr-123", []string{"web.local"}).
		Return(nil).
		Once()
	expectContainerInspectedOnBackend(tasks)

	p := NewContainerProvider(tasks)

	c := savedContainer()
	c.Networks = []entities.NetworkAttachment{
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
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		DisconnectNetwork(mock.Anything, "app", "ctr-123").
		Return(errors.New("daemon unavailable")).
		Once()

	p := NewContainerProvider(tasks)

	c := savedContainer()
	c.Networks = []entities.NetworkAttachment{
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
	tasks := mocks.NewMockTasks(t)
	tasks.EXPECT().
		DisconnectNetwork(mock.Anything, "app", "ctr-123").
		Return(nil).
		Once()
	tasks.EXPECT().
		ConnectNetwork(mock.Anything, "backend", "ctr-123", mock.Anything).
		Return(errors.New("network backend not found")).
		Once()

	p := NewContainerProvider(tasks)

	c := savedContainer()
	c.Networks = []entities.NetworkAttachment{
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
	// a strict mock with no expectations: any task call fails the test
	p := NewContainerProvider(mocks.NewMockTasks(t))

	got, err := p.Update(context.Background(), savedContainer(), nil, nil)
	require.NoError(t, err)
	require.Equal(t, "ctr-123", got.DockerID)
	require.Equal(t, "10.42.0.5", got.IPAddress)
}
