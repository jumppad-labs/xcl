// Command docker-plugin serves the Docker plugin, ../../docker, as an
// external xcl plugin. xcl starts this binary as a separate process and talks
// to it over gRPC.
//
// Build it from the example/plugin directory with `make build`, which runs
// `go build -o build/docker-plugin ./cmd/docker-plugin`.
package main

import (
	"github.com/hashicorp/go-plugin"

	"github.com/jumppad-labs/xcl/example/plugin/docker"
	"github.com/jumppad-labs/xcl/plugins"
)

// main serves the plugin to the host that started this process
func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"plugin": &plugins.GRPCPlugin{
				Impl: &docker.Plugin{},
			},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
