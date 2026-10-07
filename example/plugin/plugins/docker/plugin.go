package main

import (
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// Plugin provides the block types docker "network" and docker "container".
// main serves it as an external plugin.
type Plugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*Plugin)(nil)

// Init builds the Docker client from the environment and registers both block
// types, each with a provider holding that client. Building the client does
// not contact Docker, the providers do when they are called.
//
// logger is plugin scoped, for messages written outside a provider call like
// this one. xcl adds provider=<block subtype> to what each provider's Init
// logs. During a call the providers log through plugins.Logger(ctx) instead,
// which xcl binds to the resource and step being worked on.
func (p *Plugin) Init(logger logger.Logger, state plugins.State) error {
	dockerClient, err := client.New()
	if err != nil {
		return err
	}

	logger.Debug("registering block types", "block_types", "docker.network, docker.container")

	err = plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"docker",
		"network",
		&resources.Network{},
		resources.NewNetworkProvider(dockerClient),
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
		&resources.Container{},
		resources.NewContainerProvider(dockerClient),
	)
}
