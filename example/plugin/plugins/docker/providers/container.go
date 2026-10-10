package providers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// containerProvider creates and removes Docker containers for
// docker "container" blocks. It holds the container tasks it was given, the
// real task layer when the plugin runs and a mock in the unit tests.
type containerProvider struct {
	plugins.DefaultChanged[*entities.Container]

	tasks containers.Tasks
}

var _ plugins.ResourceProvider[*entities.Container] = (*containerProvider)(nil)

// NewContainerProvider returns the provider for docker "container" blocks,
// which creates and removes containers through tasks
func NewContainerProvider(tasks containers.Tasks) plugins.ResourceProvider[*entities.Container] {
	return &containerProvider{tasks: tasks}
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
func (p *containerProvider) Create(ctx context.Context, c *entities.Container) (*entities.Container, error) {
	err := p.tasks.EnsureImage(ctx, c.Image)
	if err != nil {
		return nil, err
	}

	spec := containers.ContainerSpec{
		Name:        c.Meta.Name,
		Image:       c.Image,
		Command:     c.Command,
		Environment: c.Environment,
		Labels:      labels(c.Meta),
	}

	if len(c.Networks) > 0 {
		first := c.Networks[0]
		spec.Network = &containers.Attachment{Name: first.Name, Aliases: first.Aliases}
	}

	if c.InitScript != "" {
		path, err := filepath.Abs(c.InitScript)
		if err != nil {
			return nil, fmt.Errorf("unable to mount the init script of container %s: %w", c.Meta.Name, err)
		}

		spec.InitScript = path
	}

	id, err := p.tasks.CreateContainer(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("unable to create container %s: %w", c.Meta.Name, err)
	}

	address, err := p.startContainer(ctx, c, id)
	if err != nil {
		removeErr := p.tasks.RemoveContainer(ctx, id)
		return nil, errors.Join(err, removeErr)
	}

	c.DockerID = id
	c.IPAddress = address
	plugins.Logger(ctx).Info("created container", "name", c.Meta.Name, "image", c.Image, "ip_address", c.IPAddress)

	return c, nil
}

// Read returns the configured container with the Docker ID and address saved
// when it was created, the example does not detect changes made outside xcl
func (p *containerProvider) Read(ctx context.Context, old *entities.Container, new *entities.Container) (*entities.Container, error) {
	new.DockerID = old.DockerID
	new.IPAddress = old.IPAddress
	return new, nil
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
func (p *containerProvider) Changed(ctx context.Context, old *entities.Container, new *entities.Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
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
func (p *containerProvider) Update(ctx context.Context, c *entities.Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*entities.Container, error) {
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
		err := p.tasks.DisconnectNetwork(ctx, attachment.Name, c.DockerID)
		if err != nil && !errors.Is(err, containers.ErrNotFound) {
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

	addresses, err := p.tasks.ContainerAddresses(ctx, c.DockerID)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect container %s: %w", c.Meta.Name, err)
	}

	c.IPAddress = firstAddress(c, addresses)

	return c, nil
}

// Destroy stops and removes the container, a container that is already gone
// counts as removed
func (p *containerProvider) Destroy(ctx context.Context, c *entities.Container, force bool) error {
	err := p.tasks.StopContainer(ctx, c.DockerID)
	if errors.Is(err, containers.ErrNotFound) {
		plugins.Logger(ctx).Info("destroyed container", "name", c.Meta.Name)
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to stop container %s: %w", c.Meta.Name, err)
	}

	err = p.tasks.RemoveContainer(ctx, c.DockerID)
	if err != nil && !errors.Is(err, containers.ErrNotFound) {
		return fmt.Errorf("unable to remove container %s: %w", c.Meta.Name, err)
	}

	plugins.Logger(ctx).Info("destroyed container", "name", c.Meta.Name)

	return nil
}

// Functions returns nil, the provider offers no functions
func (p *containerProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// startContainer connects the created container to its further networks,
// starts it and returns its address on its first network
func (p *containerProvider) startContainer(ctx context.Context, c *entities.Container, id string) (string, error) {
	for _, attachment := range c.Networks[min(1, len(c.Networks)):] {
		err := p.tasks.ConnectNetwork(ctx, attachment.Name, id, attachment.Aliases)
		if err != nil {
			return "", fmt.Errorf("unable to connect container %s to network %s: %w", c.Meta.Name, attachment.Name, err)
		}
	}

	err := p.tasks.StartContainer(ctx, id)
	if err != nil {
		return "", fmt.Errorf("unable to start container %s: %w", c.Meta.Name, err)
	}

	addresses, err := p.tasks.ContainerAddresses(ctx, id)
	if err != nil {
		return "", fmt.Errorf("unable to inspect container %s: %w", c.Meta.Name, err)
	}

	return firstAddress(c, addresses), nil
}

// connect attaches the container to a network with the attachment's aliases
func (p *containerProvider) connect(ctx context.Context, c *entities.Container, attachment entities.NetworkAttachment) error {
	err := p.tasks.ConnectNetwork(ctx, attachment.Name, c.DockerID, attachment.Aliases)
	if err != nil {
		return fmt.Errorf("unable to connect container %s to network %s: %w", c.Meta.Name, attachment.Name, err)
	}

	plugins.Logger(ctx).Info("connected container to network", "name", c.Meta.Name, "network", attachment.Name)

	return nil
}

// firstAddress returns the container's address on its first network, or on
// the engine's default network when it names none, from the container's
// addresses keyed by network name
func firstAddress(c *entities.Container, addresses map[string]string) string {
	if len(c.Networks) > 0 {
		return addresses[c.Networks[0].Name]
	}

	// a container with no network block is only on the default network
	for _, address := range addresses {
		return address
	}

	return ""
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

// replaceSettings are the settings Docker fixes when it creates a container,
// a change to any of them needs a new container
var replaceSettings = []entity.Path{
	entity.Path{}.Attribute("image"),
	entity.Path{}.Attribute("command"),
	entity.Path{}.Attribute("environment"),
	entity.Path{}.Attribute("init_script"),
}
