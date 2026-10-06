// Command configonly shows XCL used for configuration only: an application
// reads its configuration into its own Go types and acts on it. The block
// types are plain Go types registered on the plugin registry, there is no
// plugin and no provider.
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
// The state is kept, encrypted, in a temporary directory that is removed when
// the program ends.
//
// Run it from this directory with `make run`, see the Makefile for the other
// targets. The configuration directory can be passed as an argument:
// `go run . <config dir>`, it defaults to ./config.
package main

import (
	"crypto/rand"
	"fmt"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/configonly/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// newStateKey returns a random 32 byte key to encrypt the sensitive values in
// state with. The state only lives as long as one run of the example, so a
// fresh key each run is enough. A real application keeps its state, so it
// loads a stable key from a secret store instead, a key it loses is state it
// can no longer read.
func newStateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("unable to generate the state key: %w", err)
	}

	return key, nil
}

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
// one line per route. It is separate from main only so the temporary state
// directory is removed before main exits, os.Exit skips deferred calls.
func run(dir string) error {
	// Encrypt the sensitive values in state, see newStateKey
	stateKey, err := newStateKey()
	if err != nil {
		return err
	}

	masker, err := mask.EncryptAES256GCM(stateKey)
	if err != nil {
		return fmt.Errorf("invalid state key: %w", err)
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stateDir)

	// the registry is built here so that the event receiver can share it: it
	// is what types the entity an event carries, which is how the receiver
	// shows each resource's configuration as it is created
	r := registry.NewPluginRegistry()

	cfg, err := loadConfig(dir, r,
		// Keep the state in a file
		xcl.WithStatePath(stateDir),
		// Encrypt sensitive values in the state, without a mask they are
		// written in plain text and xcl warns about it
		xcl.WithStateMask(masker),
		// Every event xcl produces goes to the pretty receiver on stderr, so
		// stdout holds only the routes
		xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r)),
		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
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

// loadConfig registers the example's block types on r, applies the
// configuration in dir with options added to the registry, and gathers it into
// an appConfig.
func loadConfig(dir string, r *registry.PluginRegistry, options ...xcl.ConfigOption) (*appConfig, error) {
	// Register each Go type under the block type name used in configuration.
	// A registered type needs nothing else: no plugin, no provider, no schema
	// to write by hand. Every type the configuration declares must be
	// registered, even the ones the program does not read.
	if err := r.RegisterType(&resources.ConfigMap{}, "config_map"); err != nil {
		return nil, err
	}

	if err := r.RegisterType(&resources.Secret{}, "secret"); err != nil {
		return nil, err
	}

	if err := r.RegisterType(&resources.Deployment{}, "deployment"); err != nil {
		return nil, err
	}

	if err := r.RegisterType(&resources.Service{}, "service"); err != nil {
		return nil, err
	}

	if err := r.RegisterType(&resources.Ingress{}, "ingress"); err != nil {
		return nil, err
	}

	c, err := xcl.NewConfig(append([]xcl.ConfigOption{xcl.WithPluginRegistry(r)}, options...)...)
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
