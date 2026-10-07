package resources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Container defines the block type docker "container", a Docker container
// named after the block
type Container struct {
	types.ResourceBase `xcl:",remain"`

	// Image is the image the container runs, it is pulled when Docker does
	// not have it
	Image string `xcl:"image" json:"image"`

	// Command overrides the image's command
	Command []string `xcl:"command,optional" json:"command,omitempty"`

	// Environment holds the container's environment variables
	Environment map[string]string `xcl:"environment,optional" json:"environment,omitempty"`

	// Networks are the networks the container joins, one nested network block
	// each. The container is created on the first and connected to the rest.
	Networks []NetworkAttachment `xcl:"network,block" json:"network,omitempty"`

	// DockerID is computed, the ID Docker gives the container
	DockerID string `xcl:"docker_id,optional,computed" json:"docker_id,omitempty"`

	// IPAddress is computed, the container's address on its first network
	IPAddress string `xcl:"ip_address,optional,computed" json:"ip_address,omitempty"`
}

// NetworkAttachment is a nested network block of a docker "container" block
type NetworkAttachment struct {
	// Name is the Docker name of the network, i.e. docker.network.app.meta.name
	Name string `xcl:"name" json:"name"`

	// Aliases are extra names the container is known by on the network
	Aliases []string `xcl:"aliases,optional" json:"aliases,omitempty"`
}

// containerProvider creates and removes Docker containers for
// docker "container" blocks. It holds the Docker client it was given, the
// real SDK client when the plugin runs and a mock in the unit tests.
type containerProvider struct {
	plugins.DefaultChanged[*Container]

	client client.Docker
}

var _ plugins.ResourceProvider[*Container] = (*containerProvider)(nil)

// NewContainerProvider returns the provider for docker "container" blocks,
// which creates and removes containers through dockerClient
func NewContainerProvider(dockerClient client.Docker) plugins.ResourceProvider[*Container] {
	return &containerProvider{client: dockerClient}
}

// Init logs that the provider is ready, it needs nothing else
func (p *containerProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	log.Debug("provider ready")
	return nil
}

// Create pulls the image when Docker does not have it, creates the container
// on its first network, connects the rest, starts it and records its ID and
// its address on the first network. When a step after the container was
// created fails, the container is removed again.
func (p *containerProvider) Create(ctx context.Context, c *Container) (*Container, error) {
	err := p.pullImage(ctx, c.Image)
	if err != nil {
		return nil, err
	}

	config := &container.Config{
		Image:  c.Image,
		Cmd:    c.Command,
		Env:    environment(c.Environment),
		Labels: labels(c.Meta),
	}

	networking := &network.NetworkingConfig{}
	if len(c.Networks) > 0 {
		first := c.Networks[0]
		networking.EndpointsConfig = map[string]*network.EndpointSettings{
			first.Name: {Aliases: first.Aliases},
		}
	}

	resp, err := p.client.ContainerCreate(ctx, config, &container.HostConfig{}, networking, nil, c.Meta.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to create container %s: %w", c.Meta.Name, err)
	}

	address, err := p.startContainer(ctx, c, resp.ID)
	if err != nil {
		removeErr := p.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, errors.Join(err, removeErr)
	}

	c.DockerID = resp.ID
	c.IPAddress = address
	plugins.Logger(ctx).Info("created container", "name", c.Meta.Name, "image", c.Image, "ip_address", c.IPAddress)

	return c, nil
}

// pullImage pulls ref unless Docker already has it
func (p *containerProvider) pullImage(ctx context.Context, ref string) error {
	images, err := p.client.ImageList(ctx, image.ListOptions{Filters: filters.NewArgs(filters.Arg("reference", ref))})
	if err != nil {
		return fmt.Errorf("unable to list images: %w", err)
	}

	if len(images) > 0 {
		return nil
	}

	plugins.Logger(ctx).Info("pulling image", "image", ref)

	progress, err := p.client.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("unable to pull image %s: %w", ref, err)
	}
	defer progress.Close()

	// the pull runs while its progress is read, it is done when the stream ends
	_, err = io.Copy(io.Discard, progress)
	if err != nil {
		return fmt.Errorf("unable to pull image %s: %w", ref, err)
	}

	return nil
}

// startContainer connects the created container to its further networks,
// starts it and returns its address on its first network
func (p *containerProvider) startContainer(ctx context.Context, c *Container, id string) (string, error) {
	for _, attachment := range c.Networks[min(1, len(c.Networks)):] {
		err := p.client.NetworkConnect(ctx, attachment.Name, id, &network.EndpointSettings{Aliases: attachment.Aliases})
		if err != nil {
			return "", fmt.Errorf("unable to connect container %s to network %s: %w", c.Meta.Name, attachment.Name, err)
		}
	}

	err := p.client.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to start container %s: %w", c.Meta.Name, err)
	}

	inspect, err := p.client.ContainerInspect(ctx, id)
	if err != nil {
		return "", fmt.Errorf("unable to inspect container %s: %w", c.Meta.Name, err)
	}

	return firstAddress(c, inspect), nil
}

// firstAddress returns the container's address on its first network, or on
// the engine's default network when it names none
func firstAddress(c *Container, inspect container.InspectResponse) string {
	if inspect.NetworkSettings == nil {
		return ""
	}

	if len(c.Networks) > 0 {
		if endpoint := inspect.NetworkSettings.Networks[c.Networks[0].Name]; endpoint != nil {
			return endpoint.IPAddress
		}

		return ""
	}

	// a container with no network block is only on the default network
	for _, endpoint := range inspect.NetworkSettings.Networks {
		if endpoint != nil {
			return endpoint.IPAddress
		}
	}

	return ""
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

// Destroy stops and removes the container, a container that is already gone
// counts as removed
func (p *containerProvider) Destroy(ctx context.Context, c *Container, force bool) error {
	err := p.client.ContainerStop(ctx, c.DockerID, container.StopOptions{})
	if dockerclient.IsErrNotFound(err) {
		plugins.Logger(ctx).Info("destroyed container", "name", c.Meta.Name)
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to stop container %s: %w", c.Meta.Name, err)
	}

	err = p.client.ContainerRemove(ctx, c.DockerID, container.RemoveOptions{Force: true})
	if err != nil && !dockerclient.IsErrNotFound(err) {
		return fmt.Errorf("unable to remove container %s: %w", c.Meta.Name, err)
	}

	plugins.Logger(ctx).Info("destroyed container", "name", c.Meta.Name)

	return nil
}

// Read returns the configured container with the Docker ID and address saved
// when it was created, the example does not detect changes made outside xcl
func (p *containerProvider) Read(ctx context.Context, old *Container, new *Container) (*Container, error) {
	new.DockerID = old.DockerID
	new.IPAddress = old.IPAddress
	return new, nil
}

// Update returns the container unchanged, the example does not change
// containers in place
func (p *containerProvider) Update(ctx context.Context, c *Container) (*Container, error) {
	return c, nil
}

// Functions returns nil, the provider offers no functions
func (p *containerProvider) Functions() plugins.ProviderFunctions {
	return nil
}
