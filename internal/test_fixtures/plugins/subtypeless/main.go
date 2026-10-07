// Package main is an external test plugin that provides one block type with no
// subtype, widget, written widget "<name>" {}. A widget has a repeated nested
// part block, and its provider copies the names of the parts it receives into
// the computed field part_names, which proves the nested values crossed the
// plugin process boundary.
package main

import (
	"context"
	"strings"

	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Widget is the block type widget, which has no subtype
type Widget struct {
	types.ResourceBase `xcl:",remain"`

	Size  int    `xcl:"size,optional" json:"size,omitempty"`
	Parts []Part `xcl:"part,block" json:"part,omitempty"`

	// PartNames is computed, the provider joins the names of the parts it
	// received with commas
	PartNames string `xcl:"part_names,optional,computed" json:"part_names,omitempty"`
}

// Part is the nested part block of a widget
type Part struct {
	Name string `xcl:"name" json:"name"`
}

// widgetProvider creates and destroys widgets, it holds nothing
type widgetProvider struct {
	plugins.DefaultChanged[*Widget]
}

func (p *widgetProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	log.Debug("provider ready")
	return nil
}

func (p *widgetProvider) Create(ctx context.Context, w *Widget) (*Widget, error) {
	names := []string{}
	for _, part := range w.Parts {
		names = append(names, part.Name)
	}

	w.PartNames = strings.Join(names, ",")
	plugins.Logger(ctx).Info("created widget", "parts", w.PartNames)

	return w, nil
}

func (p *widgetProvider) Destroy(ctx context.Context, w *Widget, force bool) error {
	plugins.Logger(ctx).Info("destroyed widget")
	return nil
}

func (p *widgetProvider) Read(ctx context.Context, old *Widget, new *Widget) (*Widget, error) {
	return new, nil
}

func (p *widgetProvider) Update(ctx context.Context, w *Widget) (*Widget, error) {
	return w, nil
}

func (p *widgetProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// WidgetPlugin registers widget with no subtype
type WidgetPlugin struct {
	plugins.PluginBase
}

func (p *WidgetPlugin) Init(log logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "widget", "", &Widget{}, &widgetProvider{})
}

func main() {
	plugins.Serve(&WidgetPlugin{})
}
