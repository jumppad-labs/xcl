package containers

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

	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/docker/mocks"
)

// pullProgress is an image pull's progress stream that records how far it was
// read and whether it was closed
type pullProgress struct {
	reader *strings.Reader
	closed bool
}

func newPullProgress(content string) *pullProgress {
	return &pullProgress{reader: strings.NewReader(content)}
}

func (p *pullProgress) Read(buffer []byte) (int, error) {
	return p.reader.Read(buffer)
}

func (p *pullProgress) Close() error {
	p.closed = true
	return nil
}

// failingPullProgress is a pull progress stream that fails when read
type failingPullProgress struct{}

func (failingPullProgress) Read([]byte) (int, error) {
	return 0, errors.New("stream reset")
}

func (failingPullProgress) Close() error {
	return nil
}

func imageListOptions(ref string) image.ListOptions {
	return image.ListOptions{Filters: filters.NewArgs(filters.Arg("reference", ref))}
}

// CreateNetwork

func TestCreateNetworkSendsALabelledAttachableBridgeWithoutIPAM(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(mock.Anything, "app", network.CreateOptions{
			Driver:     "bridge",
			Attachable: true,
			Labels:     map[string]string{"xcl_id": "docker.network.app"},
		}).
		Return(network.CreateResponse{ID: "net-123"}, nil).
		Once()

	id, err := New(client).CreateNetwork(context.Background(), "app", NetworkSpec{
		Labels: map[string]string{"xcl_id": "docker.network.app"},
	})

	require.NoError(t, err)
	require.Equal(t, "net-123", id)
}

func TestCreateNetworkSendsTheSubnetAsDefaultIPAM(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(mock.Anything, "app", network.CreateOptions{
			Driver:     "bridge",
			Attachable: true,
			Labels:     map[string]string{"xcl_id": "docker.network.app"},
			IPAM: &network.IPAM{
				Driver: "default",
				Config: []network.IPAMConfig{{Subnet: "10.5.0.0/16"}},
			},
		}).
		Return(network.CreateResponse{ID: "net-123"}, nil).
		Once()

	id, err := New(client).CreateNetwork(context.Background(), "app", NetworkSpec{
		Subnet: "10.5.0.0/16",
		Labels: map[string]string{"xcl_id": "docker.network.app"},
	})

	require.NoError(t, err)
	require.Equal(t, "net-123", id)
}

func TestCreateNetworkReturnsDockersError(t *testing.T) {
	dockerErr := errors.New("pool overlaps with other one on this address space")
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkCreate(mock.Anything, "app", mock.Anything).
		Return(network.CreateResponse{}, dockerErr).
		Once()

	id, err := New(client).CreateNetwork(context.Background(), "app", NetworkSpec{})

	require.Same(t, dockerErr, err)
	require.Empty(t, id)
}

// NetworkContainers

func TestNetworkContainersReturnsTheAttachedContainerIDsSorted(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(mock.Anything, "net-123", network.InspectOptions{}).
		Return(network.Inspect{
			Containers: map[string]network.EndpointResource{
				"ctr-c": {},
				"ctr-a": {},
				"ctr-b": {},
			},
		}, nil).
		Once()

	attached, err := New(client).NetworkContainers(context.Background(), "net-123")

	require.NoError(t, err)
	require.Equal(t, []string{"ctr-a", "ctr-b", "ctr-c"}, attached)
}

func TestNetworkContainersReturnsDockersError(t *testing.T) {
	dockerErr := errors.New("daemon unavailable")
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(mock.Anything, "net-123", network.InspectOptions{}).
		Return(network.Inspect{}, dockerErr).
		Once()

	attached, err := New(client).NetworkContainers(context.Background(), "net-123")

	require.Same(t, dockerErr, err)
	require.NotErrorIs(t, err, ErrNotFound)
	require.Nil(t, attached)
}

func TestNetworkContainersReportsAMissingNetworkAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such network"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkInspect(mock.Anything, "net-123", network.InspectOptions{}).
		Return(network.Inspect{}, dockerErr).
		Once()

	_, err := New(client).NetworkContainers(context.Background(), "net-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, err, dockerErr)
	require.Equal(t, dockerErr.Error(), err.Error())
}

// RemoveNetwork

func TestRemoveNetworkRemovesTheNetwork(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkRemove(mock.Anything, "net-123").
		Return(nil).
		Once()

	err := New(client).RemoveNetwork(context.Background(), "net-123")

	require.NoError(t, err)
}

func TestRemoveNetworkReportsAMissingNetworkAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such network"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkRemove(mock.Anything, "net-123").
		Return(dockerErr).
		Once()

	err := New(client).RemoveNetwork(context.Background(), "net-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, dockerErr.Error(), err.Error())
}

func TestRemoveNetworkReturnsOtherErrorsUnchanged(t *testing.T) {
	dockerErr := errors.New("network has active endpoints")
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkRemove(mock.Anything, "net-123").
		Return(dockerErr).
		Once()

	err := New(client).RemoveNetwork(context.Background(), "net-123")

	require.Same(t, dockerErr, err)
	require.NotErrorIs(t, err, ErrNotFound)
}

// DisconnectNetwork

func TestDisconnectNetworkForceDetachesTheContainer(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(nil).
		Once()

	err := New(client).DisconnectNetwork(context.Background(), "app", "ctr-123")

	require.NoError(t, err)
}

func TestDisconnectNetworkReportsAMissingContainerAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such container"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(dockerErr).
		Once()

	err := New(client).DisconnectNetwork(context.Background(), "app", "ctr-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, dockerErr.Error(), err.Error())
}

func TestDisconnectNetworkReportsAContainerNotOnTheNetworkAsNotFound(t *testing.T) {
	dockerErr := errdefs.Forbidden(errors.New("container ctr-123 is not connected to the network app"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(dockerErr).
		Once()

	err := New(client).DisconnectNetwork(context.Background(), "app", "ctr-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, dockerErr.Error(), err.Error())
}

func TestDisconnectNetworkReturnsOtherErrors(t *testing.T) {
	dockerErr := errdefs.Forbidden(errors.New("operation not permitted"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkDisconnect(mock.Anything, "app", "ctr-123", true).
		Return(dockerErr).
		Once()

	err := New(client).DisconnectNetwork(context.Background(), "app", "ctr-123")

	require.ErrorIs(t, err, dockerErr)
	require.NotErrorIs(t, err, ErrNotFound)
}

// ConnectNetwork

func TestConnectNetworkAttachesTheContainerWithAliases(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkConnect(mock.Anything, "app", "ctr-123", &network.EndpointSettings{
			Aliases: []string{"web.local", "web"},
		}).
		Return(nil).
		Once()

	err := New(client).ConnectNetwork(context.Background(), "app", "ctr-123", []string{"web.local", "web"})

	require.NoError(t, err)
}

func TestConnectNetworkReportsAMissingNetworkAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such network"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		NetworkConnect(mock.Anything, "app", "ctr-123", mock.Anything).
		Return(dockerErr).
		Once()

	err := New(client).ConnectNetwork(context.Background(), "app", "ctr-123", nil)

	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, dockerErr.Error(), err.Error())
}

// EnsureImage

func TestEnsureImageSkipsThePullForAPresentImage(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, imageListOptions("nginx:1.27")).
		Return([]image.Summary{{ID: "sha256:abc"}}, nil).
		Once()

	err := New(client).EnsureImage(context.Background(), "nginx:1.27")

	require.NoError(t, err)
	client.AssertNotCalled(t, "ImagePull", mock.Anything, mock.Anything, mock.Anything)
}

func TestEnsureImagePullsAMissingImageAndDrainsItsProgress(t *testing.T) {
	progress := newPullProgress(`{"status":"Pulling from library/nginx"}` + "\n" + `{"status":"Download complete"}`)
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, imageListOptions("nginx:1.27")).
		Return([]image.Summary{}, nil).
		Once()
	client.EXPECT().
		ImagePull(mock.Anything, "nginx:1.27", image.PullOptions{}).
		Return(progress, nil).
		Once()

	err := New(client).EnsureImage(context.Background(), "nginx:1.27")

	require.NoError(t, err)
	require.Zero(t, progress.reader.Len(), "expected the pull progress to be read to the end")
	require.True(t, progress.closed, "expected the pull progress to be closed")
}

func TestEnsureImageReturnsAnImageListError(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, imageListOptions("nginx:1.27")).
		Return(nil, errors.New("daemon unavailable")).
		Once()

	err := New(client).EnsureImage(context.Background(), "nginx:1.27")

	require.EqualError(t, err, "unable to list images: daemon unavailable")
}

func TestEnsureImageReturnsAnImagePullError(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, imageListOptions("nginx:1.27")).
		Return([]image.Summary{}, nil).
		Once()
	client.EXPECT().
		ImagePull(mock.Anything, "nginx:1.27", image.PullOptions{}).
		Return(nil, errors.New("manifest unknown")).
		Once()

	err := New(client).EnsureImage(context.Background(), "nginx:1.27")

	require.EqualError(t, err, "unable to pull image nginx:1.27: manifest unknown")
}

func TestEnsureImageReportsAMissingImageAsNotFound(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, imageListOptions("nginx:1.27")).
		Return([]image.Summary{}, nil).
		Once()
	client.EXPECT().
		ImagePull(mock.Anything, "nginx:1.27", image.PullOptions{}).
		Return(nil, errdefs.NotFound(errors.New("pull access denied for nginx"))).
		Once()

	err := New(client).EnsureImage(context.Background(), "nginx:1.27")

	require.ErrorIs(t, err, ErrNotFound)
	require.EqualError(t, err, "unable to pull image nginx:1.27: pull access denied for nginx")
}

func TestEnsureImageReturnsAPullProgressError(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ImageList(mock.Anything, imageListOptions("nginx:1.27")).
		Return([]image.Summary{}, nil).
		Once()
	client.EXPECT().
		ImagePull(mock.Anything, "nginx:1.27", image.PullOptions{}).
		Return(io.ReadCloser(failingPullProgress{}), nil).
		Once()

	err := New(client).EnsureImage(context.Background(), "nginx:1.27")

	require.EqualError(t, err, "unable to pull image nginx:1.27: stream reset")
}

// CreateContainer

func TestCreateContainerSendsTheNameImageCommandLabelsAndSortedEnvironment(t *testing.T) {
	var gotConfig *container.Config

	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, &container.HostConfig{}, &network.NetworkingConfig{}, (*v1.Platform)(nil), "web").
		Run(func(_ context.Context, config *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *v1.Platform, _ string) {
			gotConfig = config
		}).
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()

	id, err := New(client).CreateContainer(context.Background(), ContainerSpec{
		Name:    "web",
		Image:   "nginx:1.27",
		Command: []string{"nginx", "-g", "daemon off;"},
		Environment: map[string]string{
			"ZONE":  "eu",
			"APP":   "web",
			"DEBUG": "true",
		},
		Labels: map[string]string{"xcl_id": "docker.container.web"},
	})

	require.NoError(t, err)
	require.Equal(t, "ctr-123", id)
	require.Equal(t, &container.Config{
		Image:  "nginx:1.27",
		Cmd:    []string{"nginx", "-g", "daemon off;"},
		Env:    []string{"APP=web", "DEBUG=true", "ZONE=eu"},
		Labels: map[string]string{"xcl_id": "docker.container.web"},
	}, gotConfig)
}

func TestCreateContainerAttachesTheNetworkWithAliases(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"app": {Aliases: []string{"web.local"}},
			},
		}, (*v1.Platform)(nil), "web").
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()

	_, err := New(client).CreateContainer(context.Background(), ContainerSpec{
		Name:    "web",
		Image:   "nginx:1.27",
		Network: &Attachment{Name: "app", Aliases: []string{"web.local"}},
	})

	require.NoError(t, err)
}

func TestCreateContainerMountsTheInitScriptReadOnly(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, &container.HostConfig{
			Binds: []string{"/tmp/xcl/init.sh:/docker-entrypoint.d/90-xcl-init.sh:ro"},
		}, mock.Anything, (*v1.Platform)(nil), "web").
		Return(container.CreateResponse{ID: "ctr-123"}, nil).
		Once()

	_, err := New(client).CreateContainer(context.Background(), ContainerSpec{
		Name:       "web",
		Image:      "nginx:1.27",
		InitScript: "/tmp/xcl/init.sh",
	})

	require.NoError(t, err)
}

func TestCreateContainerReturnsDockersError(t *testing.T) {
	dockerErr := errors.New("conflict: container name in use")
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "web").
		Return(container.CreateResponse{}, dockerErr).
		Once()

	id, err := New(client).CreateContainer(context.Background(), ContainerSpec{Name: "web", Image: "nginx:1.27"})

	require.Same(t, dockerErr, err)
	require.Empty(t, id)
}

// StartContainer, StopContainer, RemoveContainer

func TestStartContainerStartsTheContainer(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStart(mock.Anything, "ctr-123", container.StartOptions{}).
		Return(nil).
		Once()

	err := New(client).StartContainer(context.Background(), "ctr-123")

	require.NoError(t, err)
}

func TestStartContainerReturnsDockersError(t *testing.T) {
	dockerErr := errors.New("port is already allocated")
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStart(mock.Anything, "ctr-123", container.StartOptions{}).
		Return(dockerErr).
		Once()

	err := New(client).StartContainer(context.Background(), "ctr-123")

	require.Same(t, dockerErr, err)
}

func TestStopContainerStopsTheContainer(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStop(mock.Anything, "ctr-123", container.StopOptions{}).
		Return(nil).
		Once()

	err := New(client).StopContainer(context.Background(), "ctr-123")

	require.NoError(t, err)
}

func TestStopContainerReportsAMissingContainerAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such container"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerStop(mock.Anything, "ctr-123", container.StopOptions{}).
		Return(dockerErr).
		Once()

	err := New(client).StopContainer(context.Background(), "ctr-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, dockerErr.Error(), err.Error())
}

func TestRemoveContainerForceRemovesTheContainer(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerRemove(mock.Anything, "ctr-123", container.RemoveOptions{Force: true}).
		Return(nil).
		Once()

	err := New(client).RemoveContainer(context.Background(), "ctr-123")

	require.NoError(t, err)
}

func TestRemoveContainerReportsAMissingContainerAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such container"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerRemove(mock.Anything, "ctr-123", container.RemoveOptions{Force: true}).
		Return(dockerErr).
		Once()

	err := New(client).RemoveContainer(context.Background(), "ctr-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, err, dockerErr)
	require.Equal(t, dockerErr.Error(), err.Error())
}

// ContainerAddresses

func TestContainerAddressesReturnsTheAddressOnEachNetwork(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{
					"app":     {IPAddress: "10.5.0.2"},
					"backend": {IPAddress: "10.6.0.2"},
					"gone":    nil,
				},
			},
		}, nil).
		Once()

	addresses, err := New(client).ContainerAddresses(context.Background(), "ctr-123")

	require.NoError(t, err)
	require.Equal(t, map[string]string{"app": "10.5.0.2", "backend": "10.6.0.2"}, addresses)
}

func TestContainerAddressesIsEmptyWithoutNetworkSettings(t *testing.T) {
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{}, nil).
		Once()

	addresses, err := New(client).ContainerAddresses(context.Background(), "ctr-123")

	require.NoError(t, err)
	require.NotNil(t, addresses)
	require.Empty(t, addresses)
}

func TestContainerAddressesReportsAMissingContainerAsNotFound(t *testing.T) {
	dockerErr := errdefs.NotFound(errors.New("no such container"))
	client := mocks.NewMockDocker(t)
	client.EXPECT().
		ContainerInspect(mock.Anything, "ctr-123").
		Return(container.InspectResponse{}, dockerErr).
		Once()

	addresses, err := New(client).ContainerAddresses(context.Background(), "ctr-123")

	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, dockerErr.Error(), err.Error())
	require.Nil(t, addresses)
}
