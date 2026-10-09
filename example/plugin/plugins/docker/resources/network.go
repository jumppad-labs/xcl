package resources

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Network defines the block type docker "network", a Docker bridge network
// named after the block
type Network struct {
	types.ResourceBase `xcl:",remain"`

	// Subnet is the network's address range, i.e. 10.42.0.0/24. When it is
	// not set Docker picks one.
	Subnet string `xcl:"subnet,optional" json:"subnet,omitempty"`

	// DockerID is computed, the provider sets it to the ID Docker gives the
	// network when it is created
	DockerID string `xcl:"docker_id,optional,computed" json:"docker_id,omitempty"`
}

// networkProvider creates and removes Docker networks for docker "network"
// blocks. It holds the Docker client it was given, the real SDK client when
// the plugin runs and a mock in the unit tests.
type networkProvider struct {
	plugins.DefaultChanged[*Network]

	client client.Docker
}

var _ plugins.ResourceProvider[*Network] = (*networkProvider)(nil)

// NewNetworkProvider returns the provider for docker "network" blocks, which
// creates and removes networks through dockerClient
func NewNetworkProvider(dockerClient client.Docker) plugins.ResourceProvider[*Network] {
	return &networkProvider{client: dockerClient}
}

// Init logs that the provider is ready, it needs nothing else
func (p *networkProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	log.Debug("provider ready")
	return nil
}

// Create asks Docker for a labelled bridge network named after the block, and
// records the ID Docker gives it
func (p *networkProvider) Create(ctx context.Context, n *Network) (*Network, error) {
	options := network.CreateOptions{
		Driver:     "bridge",
		Attachable: true,
		Labels:     labels(n.Meta),
	}

	if n.Subnet != "" {
		options.IPAM = &network.IPAM{
			Driver: "default",
			Config: []network.IPAMConfig{{Subnet: n.Subnet}},
		}
	}

	resp, err := p.client.NetworkCreate(ctx, n.Meta.Name, options)
	if err != nil {
		return nil, fmt.Errorf("unable to create network %s: %w", n.Meta.Name, err)
	}

	n.DockerID = resp.ID
	plugins.Logger(ctx).Info("created network", "name", n.Meta.Name, "id", n.DockerID)

	return n, nil
}

// Destroy removes the network, a network that is already gone counts as
// removed. Every container still attached is force-disconnected first, so
// rebuilding a network never fails on its active endpoints; the container's
// own Update attaches it again to the network that replaces this one.
func (p *networkProvider) Destroy(ctx context.Context, n *Network, force bool) error {
	inspect, err := p.client.NetworkInspect(ctx, n.DockerID, network.InspectOptions{})
	if dockerclient.IsErrNotFound(err) {
		plugins.Logger(ctx).Info("network already removed", "name", n.Meta.Name, "id", n.DockerID)
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to inspect network %s: %w", n.Meta.Name, err)
	}

	// sorted, so the containers are always detached in the same order
	attached := make([]string, 0, len(inspect.Containers))
	for containerID := range inspect.Containers {
		attached = append(attached, containerID)
	}
	sort.Strings(attached)

	for _, containerID := range attached {
		err := p.client.NetworkDisconnect(ctx, n.DockerID, containerID, true)
		if err != nil && !dockerclient.IsErrNotFound(err) {
			return fmt.Errorf("unable to disconnect container %s from network %s: %w", containerID, n.Meta.Name, err)
		}

		plugins.Logger(ctx).Info("disconnected container from network", "name", n.Meta.Name, "container", containerID)
	}

	err = p.client.NetworkRemove(ctx, n.DockerID)
	if err != nil && !dockerclient.IsErrNotFound(err) {
		return fmt.Errorf("unable to remove network %s: %w", n.Meta.Name, err)
	}

	plugins.Logger(ctx).Info("destroyed network", "name", n.Meta.Name, "id", n.DockerID)

	return nil
}

// Read returns the configured network with the Docker ID saved when it was
// created, the example does not detect changes made outside xcl
func (p *networkProvider) Read(ctx context.Context, old *Network, new *Network) (*Network, error) {
	new.DockerID = old.DockerID
	return new, nil
}

// Changed answers replace when the subnet changes: Docker can not move a
// network to a new address range, the network has to be removed and created
// again. Any other change is left to DefaultChanged.
func (p *networkProvider) Changed(ctx context.Context, old *Network, new *Network, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	if old.Subnet != new.Subnet {
		return entity.Replace, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new, changes, dependencies)
}

// Update returns the network unchanged. Every setting Docker can not change in
// place answers replace in Changed, so Update is only reached for changes that
// need no Docker call, such as xcl's own metadata.
func (p *networkProvider) Update(ctx context.Context, n *Network, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Network, error) {
	return n, nil
}

// Functions returns nil, the provider offers no functions
func (p *networkProvider) Functions() plugins.ProviderFunctions {
	return nil
}
