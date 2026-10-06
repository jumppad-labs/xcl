package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

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

	client Client
}

var _ plugins.ResourceProvider[*Network] = (*networkProvider)(nil)

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
// removed
func (p *networkProvider) Destroy(ctx context.Context, n *Network, force bool) error {
	err := p.client.NetworkRemove(ctx, n.DockerID)
	if err != nil && !client.IsErrNotFound(err) {
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

// Update returns the network unchanged, the example does not change networks
// in place
func (p *networkProvider) Update(ctx context.Context, n *Network) (*Network, error) {
	return n, nil
}

// Functions returns nil, the provider offers no functions
func (p *networkProvider) Functions() plugins.ProviderFunctions {
	return nil
}
