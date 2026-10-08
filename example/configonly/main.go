// Command configonly shows XCL used for configuration only: an application
// reads its configuration into its own Go types and acts on it. The block
// types are plain Go types declared with xcl.WithType, there is no plugin, no
// provider and no registry.
//
// The configuration it reads (./config) is a small Kubernetes-like
// deployment, and the Go types it reads it into are in ./resources. Between
// them they show blocks nested inside blocks, blocks that repeat into a
// slice, and resources linked to each other by reference.
//
// The program does three things: loadConfig applies the configuration and
// gathers it into the application's own struct, appConfig, with one Decode
// call; ingressRoutes follows the links from each ingress path to the service,
// deployment, container and port its traffic reaches; and main prints one
// line per route.
//
// The example keeps no state: without a state option xcl holds the applied
// configuration in memory only, which is all a program that only reads its
// configuration needs.
//
// Run it from this directory with `make run`, see the Makefile for the other
// targets. The configuration directory can be passed as an argument:
// `go run . <config dir>`, it defaults to ./config.
package main

import (
	"fmt"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/configonly/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
)

func main() {
	dir := "./config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	if err := run(dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run loads the configuration in dir, works out its ingress routes and prints
// one line per route.
func run(dir string) error {
	cfg, err := loadConfig(dir,
		// Every event xcl produces goes to the pretty receiver on stderr, so
		// stdout holds only the routes
		xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv())),
		// events carry nothing by default, this asks for each resource as it
		// is processed, which is what the receiver turns back into
		// configuration text. Sensitive fields are redacted in events.
		xcl.WithEventData(xcl.EventDataProcessed),
	)
	if err != nil {
		return err
	}

	routes, err := ingressRoutes(cfg)
	if err != nil {
		return err
	}

	for _, route := range routes {
		fmt.Println(route)
	}

	return nil
}

// appConfig is the application's view of the configuration, filled by one
// Decode call. A slice field receives every block of its registered type, in
// the order the blocks were written, so the configuration can declare as many
// of each as it needs. It holds only what the program reads, reading a new
// block type needs only a new field.
type appConfig struct {
	Deployments []*resources.Deployment
	Services    []*resources.Service
	Ingresses   []*resources.Ingress
}

// loadConfig declares the example's block types, applies the configuration in
// dir with the caller's options, and gathers it into an appConfig.
func loadConfig(dir string, options ...xcl.ConfigOption) (*appConfig, error) {
	// Declare each Go type under the block type name used in configuration.
	// A declared type needs nothing else: no plugin, no provider, no schema
	// to write by hand. Every type the configuration declares must be
	// declared here, even the ones the program does not read.
	types := []xcl.ConfigOption{
		xcl.WithType(&resources.ConfigMap{}, "config_map"),
		xcl.WithType(&resources.Secret{}, "secret"),
		xcl.WithType(&resources.Deployment{}, "deployment"),
		xcl.WithType(&resources.Service{}, "service"),
		xcl.WithType(&resources.Ingress{}, "ingress"),
	}

	c, err := xcl.NewConfig(append(types, options...)...)
	if err != nil {
		return nil, err
	}

	if err := c.Apply(dir); err != nil {
		return nil, fmt.Errorf("loading configuration from %s: %w", dir, err)
	}

	// gather the configuration into the application's own struct, Decode is
	// an ordinary method so this works on every supported Go version
	var cfg appConfig
	if err := c.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("reading configuration from %s: %w", dir, err)
	}

	return &cfg, nil
}
