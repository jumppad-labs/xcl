package providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// networkProvider creates and removes Docker networks for docker "network"
// blocks. It holds the container tasks it was given, the real task layer
// when the plugin runs and a mock in the unit tests.
type networkProvider struct {
	plugins.DefaultChanged[*entities.Network]

	tasks containers.Tasks
}

var _ plugins.ResourceProvider[*entities.Network] = (*networkProvider)(nil)

// NewNetworkProvider returns the provider for docker "network" blocks, which
// creates and removes networks through tasks
func NewNetworkProvider(tasks containers.Tasks) plugins.ResourceProvider[*entities.Network] {
	return &networkProvider{tasks: tasks}
}

// Init logs that the provider is ready, it needs nothing else
func (p *networkProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	log.Debug("provider ready")
	return nil
}

// Create asks Docker for a labelled bridge network named after the block, and
// records the ID Docker gives it
func (p *networkProvider) Create(ctx context.Context, n *entities.Network) (*entities.Network, error) {
	id, err := p.tasks.CreateNetwork(ctx, n.Meta.Name, containers.NetworkSpec{
		Subnet: n.Subnet,
		Labels: labels(n.Meta),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create network %s: %w", n.Meta.Name, err)
	}

	n.DockerID = id
	plugins.Logger(ctx).Info("created network", "name", n.Meta.Name, "id", n.DockerID)

	return n, nil
}

// Read returns the configured network with the Docker ID saved when it was
// created, the example does not detect changes made outside xcl
func (p *networkProvider) Read(ctx context.Context, old *entities.Network, new *entities.Network) (*entities.Network, error) {
	new.DockerID = old.DockerID
	return new, nil
}

// Changed decides from what it is told. It answers replace when the subnet
// changed: Docker can not move a network to a new address range, the network
// has to be removed and created again. Any other change to its settings
// answers update, which needs no Docker call. When nothing changed the
// decision is left to DefaultChanged.
func (p *networkProvider) Changed(ctx context.Context, old *entities.Network, new *entities.Network, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	for _, change := range changes {
		for _, setting := range networkReplaceSettings {
			if change.Within(setting) {
				return entity.Replace, nil
			}
		}
	}

	if len(changes) > 0 {
		return entity.Update, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new, changes, dependencies)
}

// Update returns the network unchanged. Every setting Docker can not change in
// place answers replace in Changed, so Update is only reached for changes that
// need no Docker call, such as xcl's own metadata.
func (p *networkProvider) Update(ctx context.Context, n *entities.Network, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*entities.Network, error) {
	return n, nil
}

// Destroy removes the network, a network that is already gone counts as
// removed. Every container still attached is force-disconnected first, so
// rebuilding a network never fails on its active endpoints; the container's
// own Update attaches it again to the network that replaces this one.
func (p *networkProvider) Destroy(ctx context.Context, n *entities.Network, force bool) error {
	// sorted, so the containers are always detached in the same order
	attached, err := p.tasks.NetworkContainers(ctx, n.DockerID)
	if errors.Is(err, containers.ErrNotFound) {
		plugins.Logger(ctx).Info("network already removed", "name", n.Meta.Name, "id", n.DockerID)
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to inspect network %s: %w", n.Meta.Name, err)
	}

	for _, containerID := range attached {
		err := p.tasks.DisconnectNetwork(ctx, n.DockerID, containerID)
		if err != nil && !errors.Is(err, containers.ErrNotFound) {
			return fmt.Errorf("unable to disconnect container %s from network %s: %w", containerID, n.Meta.Name, err)
		}

		plugins.Logger(ctx).Info("disconnected container from network", "name", n.Meta.Name, "container", containerID)
	}

	err = p.tasks.RemoveNetwork(ctx, n.DockerID)
	if err != nil && !errors.Is(err, containers.ErrNotFound) {
		return fmt.Errorf("unable to remove network %s: %w", n.Meta.Name, err)
	}

	plugins.Logger(ctx).Info("destroyed network", "name", n.Meta.Name, "id", n.DockerID)

	return nil
}

// Functions returns nil, the provider offers no functions
func (p *networkProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// networkReplaceSettings are the settings Docker fixes when it creates a
// network, a change to any of them needs a new network
var networkReplaceSettings = []entity.Path{
	entity.Path{}.Attribute("subnet"),
}
