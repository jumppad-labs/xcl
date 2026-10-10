// Command docker serves the Docker plugin, an external xcl plugin that creates
// real Docker networks and containers. It provides the block types
// docker "network" and docker "container", defined in ../../entities. xcl
// starts this binary as a separate process and talks to it over gRPC.
//
// Build it with `make build` in the plugin directory, which runs
// `go build -o build/docker-plugin ./cmd/docker`, or with `make build` in the
// example/plugin directory, which builds it next to xcl-docker.
package main

import (
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker"
	"github.com/jumppad-labs/xcl/plugins"
)

// main serves the plugin to the host that started this process
func main() {
	plugins.Serve(&docker.Plugin{})
}
