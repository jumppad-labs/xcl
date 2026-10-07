package plugins

import (
	"github.com/hashicorp/go-plugin"
)

// pluginName is the name the plugin is served under and the host dispenses
// it by, the two must match
const pluginName = "plugin"

// Serve serves p as an external xcl plugin to the host that started this
// process, and returns once the host stops it. It is all a plugin binary's
// main needs to call:
//
//	func main() {
//		plugins.Serve(&MyPlugin{})
//	}
//
// The handshake with the host and the gRPC transport are xcl's to manage, so a
// plugin never imports go-plugin itself.
func Serve(p Plugin) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			pluginName: &GRPCPlugin{Impl: p},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
