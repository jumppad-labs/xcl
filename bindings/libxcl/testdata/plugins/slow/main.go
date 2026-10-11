// Command slow is a fixture plugin for the language bindings' tests: it
// provides resource "slow" "<name>" {}, whose create takes delay_ms to
// complete, so a test can show that other threads, coroutines or requests
// keep running while an apply waits on a provider. It is built as an
// external plugin, xcl-plugin-slow, and loaded from a path.
package main

import (
	"context"
	"time"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Slow is the block type resource "slow" "<name>" {}
type Slow struct {
	types.ResourceBase `xcl:",remain"`

	// DelayMs is how long create, update and destroy take, in milliseconds
	DelayMs int `xcl:"delay_ms,optional" json:"delay_ms,omitempty" default:"500"`

	// Value is a plain setting, changing it answers update
	Value string `xcl:"value,optional" json:"value,omitempty"`

	// Done is computed by create, so an entity read back shows it ran
	Done bool `xcl:"done,optional,computed" json:"done,omitempty"`
}

type slowProvider struct {
	plugins.DefaultChanged[*Slow]
}

var _ plugins.ResourceProvider[*Slow] = (*slowProvider)(nil)

func (p *slowProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *slowProvider) Create(ctx context.Context, s *Slow) (*Slow, error) {
	wait(ctx, s.DelayMs)
	s.Done = true

	return s, nil
}

func (p *slowProvider) Read(ctx context.Context, old *Slow, new *Slow) (*Slow, error) {
	new.Done = old.Done

	return new, nil
}

func (p *slowProvider) Update(ctx context.Context, s *Slow, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Slow, error) {
	wait(ctx, s.DelayMs)
	s.Done = true

	return s, nil
}

func (p *slowProvider) Destroy(ctx context.Context, s *Slow, force bool) error {
	wait(ctx, s.DelayMs)

	return nil
}

func (p *slowProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// wait sleeps for ms milliseconds, or until ctx ends
func wait(ctx context.Context, ms int) {
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
	case <-ctx.Done():
	}
}

// SlowPlugin provides the slow block type
type SlowPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*SlowPlugin)(nil)

func (p *SlowPlugin) Init(log logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "resource", "slow", &Slow{}, &slowProvider{})
}

func main() {
	plugins.Serve(&SlowPlugin{})
}
