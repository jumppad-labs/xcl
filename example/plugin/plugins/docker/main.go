// Command docker is the Docker plugin, an external xcl plugin that creates
// real Docker networks and containers. It provides the block types
// docker "network" and docker "container", defined in ./resources. xcl starts
// this binary as a separate process and talks to it over gRPC.
//
// Build it from the example/plugin directory with `make build`, which runs
// `go build -o build/docker-plugin ./plugins/docker`.
package main

import (
	"github.com/jumppad-labs/xcl/plugins"
)

// main serves the plugin to the host that started this process
func main() {
	plugins.Serve(&Plugin{})
}
