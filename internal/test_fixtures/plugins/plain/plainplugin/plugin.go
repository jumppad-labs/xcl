// Package plainplugin is a test plugin whose types are plain structs, with
// no types.ResourceBase: gadget, named by its label field, and hub, a
// singleton with neither label fields nor a Meta field. Its providers fill
// computed fields from what they receive, which proves the plain values
// crossed into the provider and back. It is used in process and, through
// the main package beside it, as a plugin binary.
package plainplugin

import (
	"context"
	"strconv"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Gadget is the block type gadget, written gadget "<label>" {}. It reads its
// metadata, which xcl fills and the provider never changes
type Gadget struct {
	Handle string `xcl:"handle,label" json:"handle"`
	Size   int    `xcl:"size,optional" json:"size,omitempty"`
	Meta   types.Meta

	// Described is computed: the provider writes the label and the size it
	// received
	Described string `xcl:"described,optional,computed" json:"described,omitempty"`
}

// Hub is the singleton hub, written hub {}
type Hub struct {
	Address string `xcl:"address" json:"address"`

	// Dialled is computed: the provider copies the address it received
	Dialled string `xcl:"dialled,optional,computed" json:"dialled,omitempty"`
}

type gadgetProvider struct {
	plugins.DefaultChanged[*Gadget]
}

func (p *gadgetProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *gadgetProvider) Create(ctx context.Context, g *Gadget) (*Gadget, error) {
	g.Described = describe(g)
	return g, nil
}

func (p *gadgetProvider) Destroy(ctx context.Context, g *Gadget, force bool) error {
	plugins.Logger(ctx).Info("destroyed gadget", "handle", g.Handle)
	return nil
}

func (p *gadgetProvider) Read(ctx context.Context, old *Gadget, new *Gadget) (*Gadget, error) {
	new.Described = old.Described
	return new, nil
}

func (p *gadgetProvider) Update(ctx context.Context, g *Gadget, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Gadget, error) {
	g.Described = describe(g)
	return g, nil
}

func (p *gadgetProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func describe(g *Gadget) string {
	if g.Size == 0 {
		return g.Handle
	}

	return g.Handle + " of size " + strconv.Itoa(g.Size)
}

type hubProvider struct {
	plugins.DefaultChanged[*Hub]
}

func (p *hubProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *hubProvider) Create(ctx context.Context, h *Hub) (*Hub, error) {
	h.Dialled = h.Address
	return h, nil
}

func (p *hubProvider) Destroy(ctx context.Context, h *Hub, force bool) error {
	plugins.Logger(ctx).Info("destroyed hub", "address", h.Address)
	return nil
}

func (p *hubProvider) Read(ctx context.Context, old *Hub, new *Hub) (*Hub, error) {
	new.Dialled = old.Dialled
	return new, nil
}

func (p *hubProvider) Update(ctx context.Context, h *Hub, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Hub, error) {
	h.Dialled = h.Address
	return h, nil
}

func (p *hubProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// Plugin registers gadget and hub
type Plugin struct {
	plugins.PluginBase
}

func (p *Plugin) Init(log logger.Logger, state plugins.State) error {
	if err := plugins.RegisterResourceProvider(&p.PluginBase, log, state, "gadget", "", &Gadget{}, &gadgetProvider{}); err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "hub", "", &Hub{}, &hubProvider{})
}
