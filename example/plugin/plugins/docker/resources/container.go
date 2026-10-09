package resources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"github.com/jumppad-labs/xcl/entity"
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

	// InitScript is the path of a script the container runs when it starts,
	// mounted read-only into the nginx image's entrypoint directory. It is
	// usually a template's destination, so replacing that template replaces
	// the container.
	InitScript string `xcl:"init_script,optional" json:"init_script,omitempty"`

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

	hostConfig, err := initScriptMount(c.InitScript)
	if err != nil {
		return nil, fmt.Errorf("unable to mount the init script of container %s: %w", c.Meta.Name, err)
	}

	resp, err := p.client.ContainerCreate(ctx, config, hostConfig, networking, nil, c.Meta.Name)
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

// initScriptPath is where the init script is mounted. The nginx image's
// entrypoint runs every executable script in /docker-entrypoint.d before it
// starts nginx.
const initScriptPath = "/docker-entrypoint.d/90-xcl-init.sh"

// initScriptMount returns the host configuration that mounts script
// read-only at initScriptPath, none when there is no script
func initScriptMount(script string) (*container.HostConfig, error) {
	if script == "" {
		return &container.HostConfig{}, nil
	}

	path, err := filepath.Abs(script)
	if err != nil {
		return nil, err
	}

	return &container.HostConfig{Binds: []string{path + ":" + initScriptPath + ":ro"}}, nil
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

// replaceSettings are the settings Docker fixes when it creates a container,
// a change to any of them needs a new container
var replaceSettings = []entity.Path{
	entity.Path{}.Attribute("image"),
	entity.Path{}.Attribute("command"),
	entity.Path{}.Attribute("environment"),
	entity.Path{}.Attribute("init_script"),
}

// Changed decides from what it is told. It answers replace when the image,
// command, environment or init script changed, since Docker fixes them when
// it creates a container, or when a dependency other than a Docker network is
// replaced, such as the template that renders the init script. A change to
// the container's networks, or a network that is updated or replaced, is
// handled in place by Update, which moves the running container between
// networks. Any other change to its settings answers update. A dependency
// that is only updated, such as the init script's template rendering new
// content, leaves the container alone: the decision is left to
// DefaultChanged.
func (p *containerProvider) Changed(ctx context.Context, old *Container, new *Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	for _, change := range changes {
		for _, setting := range replaceSettings {
			if change.Within(setting) {
				return entity.Replace, nil
			}
		}
	}

	for _, dependency := range dependencies {
		if dependency.Change == entity.Replace && !isNetworkAddress(dependency.Address) {
			return entity.Replace, nil
		}
	}

	if len(changes) > 0 {
		return entity.Update, nil
	}

	for _, dependency := range dependencies {
		if isNetworkAddress(dependency.Address) {
			return entity.Update, nil
		}
	}

	return p.DefaultChanged.Changed(ctx, old, new, changes, dependencies)
}

// Update moves the running container between networks, working only from what
// it is told, never from what Docker reports:
//
//   - the previous attachments are the configured ones with each network
//     change's previous value put back
//   - networks that went away, or whose aliases changed, are disconnected
//   - networks that are new, or whose aliases changed, are connected
//   - networks that were replaced are connected again, since destroying a
//     network detaches every container
//
// When it moved the container it reads the container's new address, an
// output, not previous state.
func (p *containerProvider) Update(ctx context.Context, c *Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Container, error) {
	previous, err := previousAttachments(c.Networks, changes)
	if err != nil {
		return nil, fmt.Errorf("unable to work out the previous networks of container %s: %w", c.Meta.Name, err)
	}

	moved := false

	for _, attachment := range previous {
		current, ok := findAttachment(c.Networks, attachment.Name)
		if ok && equalStrings(current.Aliases, attachment.Aliases) {
			continue
		}

		// a network that was removed or rebuilt has already detached it
		err := p.client.NetworkDisconnect(ctx, attachment.Name, c.DockerID, true)
		if err != nil && !dockerclient.IsErrNotFound(err) {
			return nil, fmt.Errorf("unable to disconnect container %s from network %s: %w", c.Meta.Name, attachment.Name, err)
		}

		plugins.Logger(ctx).Info("disconnected container from network", "name", c.Meta.Name, "network", attachment.Name)
		moved = true
	}

	connected := map[string]bool{}
	for _, attachment := range c.Networks {
		before, ok := findAttachment(previous, attachment.Name)
		if ok && equalStrings(before.Aliases, attachment.Aliases) {
			continue
		}

		if err := p.connect(ctx, c, attachment); err != nil {
			return nil, err
		}

		connected[attachment.Name] = true
		moved = true
	}

	for _, dependency := range dependencies {
		if dependency.Change != entity.Replace || !isNetworkAddress(dependency.Address) {
			continue
		}

		attachment, ok := findAttachment(c.Networks, networkName(dependency.Address))
		if !ok || connected[attachment.Name] {
			continue
		}

		if err := p.connect(ctx, c, attachment); err != nil {
			return nil, err
		}

		connected[attachment.Name] = true
		moved = true
	}

	if !moved {
		return c, nil
	}

	inspect, err := p.client.ContainerInspect(ctx, c.DockerID)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect container %s: %w", c.Meta.Name, err)
	}

	c.IPAddress = firstAddress(c, inspect)

	return c, nil
}

// connect attaches the container to a network with the attachment's aliases
func (p *containerProvider) connect(ctx context.Context, c *Container, attachment NetworkAttachment) error {
	err := p.client.NetworkConnect(ctx, attachment.Name, c.DockerID, &network.EndpointSettings{Aliases: attachment.Aliases})
	if err != nil {
		return fmt.Errorf("unable to connect container %s to network %s: %w", c.Meta.Name, attachment.Name, err)
	}

	plugins.Logger(ctx).Info("connected container to network", "name", c.Meta.Name, "network", attachment.Name)

	return nil
}

// Functions returns nil, the provider offers no functions
func (p *containerProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// equalStrings reports whether two lists hold the same strings in the same
// order, a nil list equals an empty one
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
