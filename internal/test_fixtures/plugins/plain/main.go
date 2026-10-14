// Package main serves plainplugin as a plugin binary, so its plain types cross
// the plugin process boundary.
package main

import (
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugins/plain/plainplugin"
	"github.com/jumppad-labs/xcl/plugins"
)

func main() {
	plugins.Serve(&plainplugin.Plugin{})
}
