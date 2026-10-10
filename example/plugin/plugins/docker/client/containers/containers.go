// Package containers is the Docker plugin's container task layer, the one
// place the plugin turns what its providers want done into Docker Engine SDK
// calls: creating and removing networks, pulling images, creating, starting
// and removing containers, and moving containers between networks.
//
// It speaks in its own data types, so the providers never import a Docker
// library. It is built on the narrow SDK interface in client/docker, the real
// SDK client when the plugin runs and a mock in this package's unit tests.
// Docker's "not found" errors come back matching ErrNotFound.
package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/docker"
	"github.com/jumppad-labs/xcl/plugins"
)

// ErrNotFound is matched, with errors.Is, by the error a task returns when
// Docker reports that the network, container or image it names does not
// exist. The error keeps Docker's own message.
var ErrNotFound = errors.New("docker object not found")

// Attachment is a network a container joins, with the extra names it is
// known by there
type Attachment struct {
	// Name is the Docker name of the network
	Name string

	// Aliases are extra names the container is known by on the network
	Aliases []string
}

// NetworkSpec describes a network to create
type NetworkSpec struct {
	// Subnet is the network's address range, Docker picks one when it is ""
	Subnet string

	// Labels are put on the network
	Labels map[string]string
}

// ContainerSpec describes a container to create
type ContainerSpec struct {
	// Name is the container's Docker name
	Name string

	// Image is the image the container runs
	Image string

	// Command overrides the image's command
	Command []string

	// Environment holds the container's environment variables, sent to
	// Docker as KEY=VALUE sorted by key
	Environment map[string]string

	// Labels are put on the container
	Labels map[string]string

	// InitScript is the absolute host path of a script mounted read-only into
	// the nginx image's entrypoint directory, "" for none
	InitScript string

	// Network is the network the container is created on, nil for none
	Network *Attachment
}

// Tasks are the Docker tasks the plugin's providers perform
type Tasks interface {
	// CreateNetwork creates a labelled, attachable bridge network and
	// returns its Docker ID
	CreateNetwork(ctx context.Context, name string, spec NetworkSpec) (string, error)

	// NetworkContainers returns the IDs of the containers attached to the
	// network, sorted
	NetworkContainers(ctx context.Context, networkID string) ([]string, error)

	// RemoveNetwork removes the network
	RemoveNetwork(ctx context.Context, networkID string) error

	// ConnectNetwork attaches the container to the network with aliases
	ConnectNetwork(ctx context.Context, network, containerID string, aliases []string) error

	// DisconnectNetwork force-detaches the container from the network
	DisconnectNetwork(ctx context.Context, network, containerID string) error

	// EnsureImage pulls the image unless Docker already has it
	EnsureImage(ctx context.Context, ref string) error

	// CreateContainer creates the container and returns its Docker ID
	CreateContainer(ctx context.Context, spec ContainerSpec) (string, error)

	// StartContainer starts the container
	StartContainer(ctx context.Context, containerID string) error

	// StopContainer stops the container
	StopContainer(ctx context.Context, containerID string) error

	// RemoveContainer force-removes the container
	RemoveContainer(ctx context.Context, containerID string) error

	// ContainerAddresses returns the container's address on each network it
	// is attached to, keyed by network name
	ContainerAddresses(ctx context.Context, containerID string) (map[string]string, error)
}

// tasks performs the container tasks through a Docker SDK client, the real
// SDK client when the plugin runs and a mock in the unit tests
type tasks struct {
	client docker.Docker
}

var _ Tasks = (*tasks)(nil)

// New returns the container tasks performed through client
func New(client docker.Docker) Tasks {
	return &tasks{client: client}
}

// CreateNetwork asks Docker for a labelled bridge network that containers can
// attach to, with an address range only when spec names one
func (t *tasks) CreateNetwork(ctx context.Context, name string, spec NetworkSpec) (string, error) {
	options := network.CreateOptions{
		Driver:     "bridge",
		Attachable: true,
		Labels:     spec.Labels,
	}

	if spec.Subnet != "" {
		options.IPAM = &network.IPAM{
			Driver: "default",
			Config: []network.IPAMConfig{{Subnet: spec.Subnet}},
		}
	}

	resp, err := t.client.NetworkCreate(ctx, name, options)
	if err != nil {
		return "", notFound(err)
	}

	return resp.ID, nil
}

// NetworkContainers inspects the network and returns the IDs of the
// containers attached to it, sorted so callers always see the same order
func (t *tasks) NetworkContainers(ctx context.Context, networkID string) ([]string, error) {
	inspect, err := t.client.NetworkInspect(ctx, networkID, network.InspectOptions{})
	if err != nil {
		return nil, notFound(err)
	}

	attached := make([]string, 0, len(inspect.Containers))
	for containerID := range inspect.Containers {
		attached = append(attached, containerID)
	}
	sort.Strings(attached)

	return attached, nil
}

// RemoveNetwork removes the network
func (t *tasks) RemoveNetwork(ctx context.Context, networkID string) error {
	return notFound(t.client.NetworkRemove(ctx, networkID))
}

// ConnectNetwork attaches the container to the network with aliases
func (t *tasks) ConnectNetwork(ctx context.Context, networkName, containerID string, aliases []string) error {
	return notFound(t.client.NetworkConnect(ctx, networkName, containerID, &network.EndpointSettings{Aliases: aliases}))
}

// DisconnectNetwork force-detaches the container from the network. A container
// that is not attached to the network, as when destroying the network already
// detached it, is reported as ErrNotFound, like a missing network or container.
// Docker answers that with a forbidden error, and Podman with not found.
func (t *tasks) DisconnectNetwork(ctx context.Context, networkName, containerID string) error {
	err := t.client.NetworkDisconnect(ctx, networkName, containerID, true)
	if err != nil && strings.Contains(err.Error(), "is not connected to") {
		return &notFoundError{err: err}
	}

	return notFound(err)
}

// EnsureImage pulls ref unless Docker already has it, reading the pull's
// progress to the end, since the pull runs while its progress is read
func (t *tasks) EnsureImage(ctx context.Context, ref string) error {
	images, err := t.client.ImageList(ctx, image.ListOptions{Filters: filters.NewArgs(filters.Arg("reference", ref))})
	if err != nil {
		return fmt.Errorf("unable to list images: %w", notFound(err))
	}

	if len(images) > 0 {
		return nil
	}

	plugins.Logger(ctx).Info("pulling image", "image", ref)

	progress, err := t.client.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("unable to pull image %s: %w", ref, notFound(err))
	}
	defer progress.Close()

	// the pull runs while its progress is read, it is done when the stream ends
	_, err = io.Copy(io.Discard, progress)
	if err != nil {
		return fmt.Errorf("unable to pull image %s: %w", ref, err)
	}

	return nil
}

// CreateContainer creates the container on its network, with its environment
// sorted and its init script mounted read-only at initScriptPath
func (t *tasks) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	config := &container.Config{
		Image:  spec.Image,
		Cmd:    spec.Command,
		Env:    environment(spec.Environment),
		Labels: spec.Labels,
	}

	networking := &network.NetworkingConfig{}
	if spec.Network != nil {
		networking.EndpointsConfig = map[string]*network.EndpointSettings{
			spec.Network.Name: {Aliases: spec.Network.Aliases},
		}
	}

	hostConfig := &container.HostConfig{}
	if spec.InitScript != "" {
		hostConfig.Binds = []string{spec.InitScript + ":" + initScriptPath + ":ro"}
	}

	resp, err := t.client.ContainerCreate(ctx, config, hostConfig, networking, nil, spec.Name)
	if err != nil {
		return "", notFound(err)
	}

	return resp.ID, nil
}

// StartContainer starts the container
func (t *tasks) StartContainer(ctx context.Context, containerID string) error {
	return notFound(t.client.ContainerStart(ctx, containerID, container.StartOptions{}))
}

// StopContainer stops the container
func (t *tasks) StopContainer(ctx context.Context, containerID string) error {
	return notFound(t.client.ContainerStop(ctx, containerID, container.StopOptions{}))
}

// RemoveContainer force-removes the container, stopping it when it runs
func (t *tasks) RemoveContainer(ctx context.Context, containerID string) error {
	return notFound(t.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}))
}

// ContainerAddresses inspects the container and returns its address on each
// network it is attached to, keyed by network name, empty when Docker reports
// no network settings
func (t *tasks) ContainerAddresses(ctx context.Context, containerID string) (map[string]string, error) {
	inspect, err := t.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, notFound(err)
	}

	addresses := map[string]string{}
	if inspect.NetworkSettings == nil {
		return addresses, nil
	}

	for name, endpoint := range inspect.NetworkSettings.Networks {
		if endpoint != nil {
			addresses[name] = endpoint.IPAddress
		}
	}

	return addresses, nil
}

// notFound marks err as ErrNotFound when Docker reports that the object it
// names does not exist, and returns any other error, or nil, unchanged
func notFound(err error) error {
	if err != nil && dockerclient.IsErrNotFound(err) {
		return &notFoundError{err: err}
	}

	return err
}

// notFoundError is a Docker "not found" error that also matches ErrNotFound.
// Its message is Docker's own, so the errors the providers wrap around it read
// exactly as they did when they wrapped Docker's error directly.
type notFoundError struct {
	err error
}

// Error returns Docker's message
func (e *notFoundError) Error() string {
	return e.err.Error()
}

// Unwrap returns ErrNotFound and Docker's error, so errors.Is matches both
func (e *notFoundError) Unwrap() []error {
	return []error{ErrNotFound, e.err}
}

// environment returns the variables as KEY=VALUE, sorted so the container is
// always created the same way
func environment(variables map[string]string) []string {
	env := []string{}
	for key, value := range variables {
		env = append(env, key+"="+value)
	}

	sort.Strings(env)
	return env
}

// initScriptPath is where the init script is mounted. The nginx image's
// entrypoint runs every executable script in /docker-entrypoint.d before it
// starts nginx.
const initScriptPath = "/docker-entrypoint.d/90-xcl-init.sh"
