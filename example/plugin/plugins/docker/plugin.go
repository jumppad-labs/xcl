// Package docker is the Docker plugin, which provides the block types
// docker "network" and docker "container" and creates them as real Docker
// networks and containers.
//
// Plugin is importable: a host registers it in-process with
// registry.Local.RegisterPlugin(&docker.Plugin{}), and cmd/docker serves the
// same type as a separate program. The block types live in entities, the
// providers in providers and the Docker client layer in client. A program
// that only reads the blocks it applied imports entities alone.
package docker

import (
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers"
	dockerclient "github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/docker"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/providers"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// Plugin provides the block types docker "network" and docker "container".
// cmd/docker serves it as an external plugin.
type Plugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*Plugin)(nil)

// Init builds the Docker client from the environment, and the container task
// layer over it once, then registers both block types, each with a provider
// sharing that task layer. Building them does not contact Docker, the
// providers do when they are called.
//
// logger is plugin scoped, for messages written outside a provider call like
// this one. xcl adds provider=<block subtype> to what each provider's Init
// logs. During a call the providers log through plugins.Logger(ctx) instead,
// which xcl binds to the resource and step being worked on.
func (p *Plugin) Init(logger logger.Logger, state plugins.State) error {
	client, err := dockerclient.New()
	if err != nil {
		return err
	}

	tasks := containers.New(client)

	logger.Debug("registering block types", "block_types", "docker.network, docker.container")

	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"docker",
		"network",
		&entities.Network{},
		providers.NewNetworkProvider(tasks),
	)
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"docker",
		"container",
		&entities.Container{},
		providers.NewContainerProvider(tasks),
	)
}
