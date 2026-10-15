// Package plainplugin is a test plugin whose types are plain structs, with
// no types.ResourceBase: gadget, named by its label field, hub, a singleton
// with neither label fields nor a Meta field, and layout, whose columns and
// rows hold each other. Its providers fill
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
	plugins.Logger(ctx).Info("created gadget", "handle", g.Handle)
	return g, nil
}

func (p *gadgetProvider) Destroy(ctx context.Context, g *Gadget, force bool) error {
	plugins.Logger(ctx).Info("destroyed gadget", "handle", g.Handle)
	return nil
}

// Read reports a gadget configured with the size Lost as gone, so it is
// created again
func (p *gadgetProvider) Read(ctx context.Context, old *Gadget, new *Gadget) (*Gadget, error) {
	if new.Size == Lost {
		return nil, plugins.ErrNotFound
	}

	new.Described = old.Described
	return new, nil
}

func (p *gadgetProvider) Update(ctx context.Context, g *Gadget, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Gadget, error) {
	g.Described = describe(g)
	plugins.Logger(ctx).Info("updated gadget", "handle", g.Handle, "changes", len(changes))
	return g, nil
}

// Lost is the size of a gadget Read reports as gone
const Lost = 99

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

// Gizmo is the block type gizmo "small", written gizmo "small" "<label>" {},
// so it takes two label fields, the subtype and the label. It holds a secret
// and nested parts the provider gives serials to
type Gizmo struct {
	Size   string                  `xcl:"size,label" json:"size"`
	Handle string                  `xcl:"handle,label" json:"handle"`
	Secret types.Sensitive[string] `xcl:"secret" json:"secret"`
	Parts  []Part                  `xcl:"part,block" json:"part,omitempty"`

	// SecretLength is computed: the length of the secret the provider read
	SecretLength int `xcl:"secret_length,optional,computed" json:"secret_length,omitempty"`
}

// Part is a nested part of a gizmo, paired by its name
type Part struct {
	Name string `xcl:"name,key" json:"name"`

	// Serial is computed: the provider gives each part one at create
	Serial string `xcl:"serial,optional,computed" json:"serial,omitempty"`
}

type gizmoProvider struct {
	plugins.DefaultChanged[*Gizmo]
}

func (p *gizmoProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *gizmoProvider) Create(ctx context.Context, g *Gizmo) (*Gizmo, error) {
	g.SecretLength = len(g.Secret.Reveal())
	for i := range g.Parts {
		g.Parts[i].Serial = g.Size + "-" + g.Handle + "-" + g.Parts[i].Name
	}

	return g, nil
}

func (p *gizmoProvider) Destroy(ctx context.Context, g *Gizmo, force bool) error {
	return nil
}

func (p *gizmoProvider) Read(ctx context.Context, old *Gizmo, new *Gizmo) (*Gizmo, error) {
	return new, nil
}

func (p *gizmoProvider) Update(ctx context.Context, g *Gizmo, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Gizmo, error) {
	g.SecretLength = len(g.Secret.Reveal())
	return g, nil
}

func (p *gizmoProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// Layout is the block type layout, written layout "<label>" {}: columns
// holding rows holding columns, as deep as configuration nests them
type Layout struct {
	Handle  string   `xcl:"handle,label" json:"handle"`
	Columns []Column `xcl:"column,block" json:"column,omitempty"`

	// Counted is computed: the provider counts the columns it received, at
	// every depth
	Counted int `xcl:"counted,optional,computed" json:"counted,omitempty"`
}

// Column is a column of a layout, which may hold rows
type Column struct {
	Width string `xcl:"width,optional" json:"width,omitempty"`
	Rows  []Row  `xcl:"row,block" json:"row,omitempty"`
}

// Row is a row of a column, which may hold columns
type Row struct {
	Height  string   `xcl:"height,optional" json:"height,omitempty"`
	Columns []Column `xcl:"column,block" json:"column,omitempty"`
}

// countColumns counts columns and every column within them
func countColumns(columns []Column) int {
	count := len(columns)
	for _, column := range columns {
		for _, row := range column.Rows {
			count += countColumns(row.Columns)
		}
	}

	return count
}

type layoutProvider struct {
	plugins.DefaultChanged[*Layout]
}

func (p *layoutProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *layoutProvider) Create(ctx context.Context, l *Layout) (*Layout, error) {
	l.Counted = countColumns(l.Columns)
	return l, nil
}

func (p *layoutProvider) Destroy(ctx context.Context, l *Layout, force bool) error {
	return nil
}

func (p *layoutProvider) Read(ctx context.Context, old *Layout, new *Layout) (*Layout, error) {
	new.Counted = old.Counted
	return new, nil
}

func (p *layoutProvider) Update(ctx context.Context, l *Layout, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Layout, error) {
	l.Counted = countColumns(l.Columns)
	return l, nil
}

func (p *layoutProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// Plugin registers gadget, hub, gizmo and layout
type Plugin struct {
	plugins.PluginBase
}

func (p *Plugin) Init(log logger.Logger, state plugins.State) error {
	if err := plugins.RegisterResourceProvider(&p.PluginBase, log, state, "gadget", "", &Gadget{}, &gadgetProvider{}); err != nil {
		return err
	}

	if err := plugins.RegisterResourceProvider(&p.PluginBase, log, state, "gizmo", "small", &Gizmo{}, &gizmoProvider{}); err != nil {
		return err
	}

	if err := plugins.RegisterResourceProvider(&p.PluginBase, log, state, "layout", "", &Layout{}, &layoutProvider{}); err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "hub", "", &Hub{}, &hubProvider{})
}
