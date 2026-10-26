package main

import (
	"io"

	"github.com/jumppad-labs/xcl"
)

// plan writes to out what applying the configuration in configDir would
// change in the state c holds: each resource it would create, update,
// replace or delete, then a summary line. Nothing is created, changed or
// saved. The diff is coloured when out is a terminal.
func plan(out io.Writer, c *xcl.Config, configDir string) error {
	changes, err := c.Diff([]string{configDir})
	if err != nil {
		return explainPluginLoad(err)
	}

	// as with inspect, colour is decided for out itself, so a plan written to
	// a file or a pipe is plain text
	return xcl.PrintDiff(out, changes, xcl.WithFormatter(formatterFor(out)))
}
