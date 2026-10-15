// Package main is an external test plugin that offers the same function name
// as the functions fixture plugin, greet, from the provider of a different
// block type, salute, so a test can load both and see the clash reported.
package main

import (
	"context"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// Salute is the block type salute, which has no subtype
type Salute struct {
	Name string `xcl:"name,label" json:"name"`

	Target string `xcl:"target" json:"target"`
}

// saluteProvider offers greet, the same name the functions fixture offers
type saluteProvider struct {
	plugins.DefaultChanged[*Salute]

	functions plugins.ProviderFunctions
}

func newSaluteProvider() (*saluteProvider, error) {
	greet, err := plugins.NewFunction(func(name string) (string, error) {
		return "salute " + name, nil
	})
	if err != nil {
		return nil, err
	}

	return &saluteProvider{
		functions: plugins.NewFunctions(map[string]plugins.ProviderFunction{"greet": greet}),
	}, nil
}

func (p *saluteProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *saluteProvider) Create(ctx context.Context, s *Salute) (*Salute, error) {
	return s, nil
}

func (p *saluteProvider) Destroy(ctx context.Context, s *Salute, force bool) error {
	return nil
}

func (p *saluteProvider) Read(ctx context.Context, old *Salute, new *Salute) (*Salute, error) {
	return new, nil
}

func (p *saluteProvider) Update(ctx context.Context, s *Salute, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Salute, error) {
	return s, nil
}

func (p *saluteProvider) Functions() plugins.ProviderFunctions {
	return p.functions
}

// SalutePlugin registers salute with no subtype
type SalutePlugin struct {
	plugins.PluginBase
}

func (p *SalutePlugin) Init(log logger.Logger, state plugins.State) error {
	provider, err := newSaluteProvider()
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "salute", "", &Salute{}, provider)
}

func main() {
	plugins.Serve(&SalutePlugin{})
}
