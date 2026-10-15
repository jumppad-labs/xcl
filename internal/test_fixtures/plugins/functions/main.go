// Package main is an external test plugin that offers a function to
// configuration. It provides the block type greeting, written
// greeting "<name>" {}, whose provider offers greet(name) returning
// "hello <name>", and whose Create copies the target it received into the
// computed field received, which proves the value a function produced
// reached the plugin as a value.
package main

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// Greeting is the block type greeting, which has no subtype
type Greeting struct {
	Name string `xcl:"name,label" json:"name"`

	// Target is the value to greet, configuration usually sets it from
	// greet(...)
	Target string `xcl:"target" json:"target"`

	// Received is computed, the provider copies the target it was given
	Received string `xcl:"received,optional,computed" json:"received,omitempty"`
}

// greetingProvider creates greetings and offers the greet function
type greetingProvider struct {
	plugins.DefaultChanged[*Greeting]

	functions plugins.ProviderFunctions
}

func newGreetingProvider() (*greetingProvider, error) {
	greet, err := plugins.NewFunction(func(name string) (string, error) {
		if name == "" {
			return "", fmt.Errorf("a name is required")
		}

		return "hello " + name, nil
	})
	if err != nil {
		return nil, err
	}

	return &greetingProvider{
		functions: plugins.NewFunctions(map[string]plugins.ProviderFunction{"greet": greet}),
	}, nil
}

func (p *greetingProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *greetingProvider) Create(ctx context.Context, g *Greeting) (*Greeting, error) {
	g.Received = g.Target
	plugins.Logger(ctx).Info("created greeting", "target", g.Target)

	return g, nil
}

func (p *greetingProvider) Destroy(ctx context.Context, g *Greeting, force bool) error {
	return nil
}

func (p *greetingProvider) Read(ctx context.Context, old *Greeting, new *Greeting) (*Greeting, error) {
	return new, nil
}

func (p *greetingProvider) Update(ctx context.Context, g *Greeting, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Greeting, error) {
	g.Received = g.Target
	return g, nil
}

func (p *greetingProvider) Functions() plugins.ProviderFunctions {
	return p.functions
}

// GreetingPlugin registers greeting with no subtype
type GreetingPlugin struct {
	plugins.PluginBase
}

func (p *GreetingPlugin) Init(log logger.Logger, state plugins.State) error {
	provider, err := newGreetingProvider()
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "greeting", "", &Greeting{}, provider)
}

func main() {
	plugins.Serve(&GreetingPlugin{})
}
