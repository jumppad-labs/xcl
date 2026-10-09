package prettylog_test

import (
	"context"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// typeDatabase is the subtype Database is registered under, it is declared
// resource "database" "<name>" {}
const typeDatabase = "database"

// typeCache is the type Cache is registered as, without a subtype, so it
// leads its own declaration with only a name, cache "<name>" {}
const typeCache = "cache"

// typeNetwork is the subtype the fixture plugin provides Network under
const typeNetwork = "network"

// typeContainer is the subtype the fixture plugin provides Container under
const typeContainer = "container"

// Database is a registered type with a nested block and a computed field,
// registered without a plugin so nothing ever sets the computed field
type Database struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Port     int    `xcl:"port" json:"port"`

	Timeouts *Timeouts `xcl:"timeouts,block" json:"timeouts,omitempty"`

	ConnectionString string `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`
}

// Timeouts is the nested timeouts block of a Database
type Timeouts struct {
	Connect int `xcl:"connect" json:"connect"`
	Read    int `xcl:"read,optional" json:"read,omitempty"`
}

// Cache is registered as the type "cache" without a subtype, it is addressed
// cache.<name>
type Cache struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
}

// Network is provided by fixturePlugin, whose provider fills in the computed
// ProviderID on create
type Network struct {
	types.ResourceBase `xcl:",remain"`

	Subnet string `xcl:"subnet" json:"subnet"`

	ProviderID string `xcl:"provider_id,optional,computed" json:"provider_id,omitempty"`
}

// Container is provided by fixturePlugin, it has a repeated network block
type Container struct {
	types.ResourceBase `xcl:",remain"`

	Command  []string            `xcl:"command,optional" json:"command,omitempty"`
	Networks []NetworkAttachment `xcl:"network,block" json:"networks,omitempty"`
}

// NetworkAttachment is one repeated network block of a Container, the
// provider fills in the computed AssignedAddress on create
type NetworkAttachment struct {
	Name      string `xcl:"name" json:"name"`
	IPAddress string `xcl:"ip_address,optional" json:"ip_address,omitempty"`

	AssignedAddress string `xcl:"assigned_address,optional,computed" json:"assigned_address,omitempty"`
}

// fixturePlugin is an in-process plugin providing the network and container
// types the encode fixture declares
type fixturePlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*fixturePlugin)(nil)

// Init registers the network and container types with their providers
func (p *fixturePlugin) Init(logger logger.Logger, state plugins.State) error {
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		typeNetwork,
		&Network{},
		&networkProvider{},
	)
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		typeContainer,
		&Container{},
		&containerProvider{},
	)
}

// networkProvider sets the network's ProviderID to "id-<name>" on create
type networkProvider struct {
	plugins.DefaultChanged[*Network]
}

var _ plugins.ResourceProvider[*Network] = (*networkProvider)(nil)

func (p *networkProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *networkProvider) Create(ctx context.Context, network *Network) (*Network, error) {
	network.ProviderID = "id-" + network.Meta.Name

	return network, nil
}

func (p *networkProvider) Read(ctx context.Context, old *Network, new *Network) (*Network, error) {
	return new, nil
}

func (p *networkProvider) Update(ctx context.Context, network *Network, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Network, error) {
	return network, nil
}

func (p *networkProvider) Destroy(ctx context.Context, network *Network, force bool) error {
	return nil
}

func (p *networkProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// containerProvider sets each network attachment's AssignedAddress to
// "assigned-<name>" on create
type containerProvider struct {
	plugins.DefaultChanged[*Container]
}

var _ plugins.ResourceProvider[*Container] = (*containerProvider)(nil)

func (p *containerProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *containerProvider) Create(ctx context.Context, container *Container) (*Container, error) {
	for i := range container.Networks {
		container.Networks[i].AssignedAddress = "assigned-" + container.Networks[i].Name
	}

	return container, nil
}

func (p *containerProvider) Read(ctx context.Context, old *Container, new *Container) (*Container, error) {
	return new, nil
}

func (p *containerProvider) Update(ctx context.Context, container *Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Container, error) {
	return container, nil
}

func (p *containerProvider) Destroy(ctx context.Context, container *Container, force bool) error {
	return nil
}

func (p *containerProvider) Functions() plugins.ProviderFunctions {
	return nil
}
