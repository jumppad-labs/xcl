// Command configonly shows XCL used for configuration only: parsing a
// configuration into Go objects. The block types are plain Go types
// registered on the plugin registry, there is no plugin and no provider.
// Blocks are decoded into the Go types, references between them are resolved,
// and no provider is ever called.
//
// The configuration it parses (./config) is a small Kubernetes-like
// deployment, and the Go types it parses into are in ./resources. Between
// them they show blocks nested inside blocks, blocks that repeat into a
// slice, and resources linked to each other by reference.
//
// After the configuration is applied, one Decode call gathers every block the
// program reads into a struct of its own, appConfig, rather than looking each
// type up separately.
//
// The state is kept in a file, and after the resources are printed everything
// is destroyed again, which for these types only clears them from the state.
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
	"github.com/kr/pretty"
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

	// Encrypt the sensitive values in state, see newStateKey
	stateKey, err := newStateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// the registry is built here so that the event receiver can share it: it
	// is what types the entity an event carries, which is how the receiver
	// shows each resource's configuration as it is created
	r := registry.NewPluginRegistry()
	eh := prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r)

	// parse the config
	entities, err := run(eh, r, dir, stateDir, stateKey)

	pretty.Print(entities)

	// cleanup
	os.RemoveAll(stateDir)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

}

// appConfig is the application's view of the configuration. Decode fills it
// in one call: a slice field receives every block of its registered type, in
// the order the blocks were written, and a pointer field receives the one
// block of its type. Reading a new block type needs only a new field.
type appConfig struct {
	ConfigMaps  []*resources.ConfigMap
	Deployments []*resources.Deployment
	Service     *resources.Service
	Ingress     *resources.Ingress
}

// run applies the configuration in dir with the example types registered,
// keeping the state in a file in stateDir, writes the resources and query
// results to out, then destroys everything and returns the resources that were
// applied. Every event xcl produces goes to handler, a nil handler leaves xcl
// silent. The sensitive values in state are encrypted with stateKey, or
// written in plain text, with a warning, when it is nil.
func run(handler xcl.EventHandler, r *registry.PluginRegistry, dir string, stateDir string, stateKey []byte) ([]any, error) {

	// Register each Go type under the block type name used in configuration.
	// A registered type needs nothing else: no plugin, no provider, no schema
	// to write by hand.
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

	// a masker allows sensitive values in the state and events to be
	// encrypted
	masker, err := mask.EncryptAES256GCM(stateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid state key: %w", err)
	}

	options := []xcl.ConfigOption{
		xcl.WithPluginRegistry(r),

		// Keep the state in a file, Destroy works from it alone
		xcl.WithStatePath(stateDir),

		// Using an optional state mask ensures that all types.Sensitive values are stored
		// in the state as encrypted values
		xcl.WithStateMask(masker),

		// Every action for a resources lifecycle is broadcast as an event
		// rather than injecting a logger you inject an event handler
		// which can log values or push them to Prometheus,etc
		xcl.WithEventHandler(handler),

		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
		// configuration text. Passwords are redacted in it.
		xcl.WithEventData(xcl.EventDataProcessed),
	}

	// with a key, the passwords are encrypted in the state file
	c, err := xcl.NewConfig(options...)
	if err != nil {
		return nil, err
	}

	if err := c.Apply(dir); err != nil {
		return nil, err
	}

	applied := c.Entities()

	return applied, nil
}
