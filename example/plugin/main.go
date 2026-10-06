// Command plugin shows XCL used with plugins. The block types are provided
// by two plugins whose providers take part in the lifecycle, creating the
// resources on apply and destroying them at the end. Each plugin provides two
// block types, registered with a provider of their own:
//
//   - ExamplePlugin (./internal) is an in-process plugin, compiled into this
//     program. It provides postgres and redis, filling in the computed
//     connection_string on both that the configuration only example leaves
//     empty.
//   - external (./external) is an external plugin, compiled to its own binary
//     that xcl starts as a separate process and calls over gRPC. It provides
//     app and ingress, and fills in the computed url on app that ingress
//     reads.
//
// The Go types are in ./resources and the configuration it applies is in
// ./config.
//
// The passwords are encrypted in state with a key generated at start up,
// see newStateKey.
//
// Build the external plugin and run the example from this directory with
// `make run`, see the Makefile for the other targets. The configuration
// directory and the external plugin binary can be passed as arguments:
// `go run . <config dir> <external plugin binary>`, they default to ./config
// and ./build/external.
package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/internal"
	"github.com/jumppad-labs/xcl/example/plugin/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
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

// stateMaskOptions returns the option that encrypts state with stateKey, or
// none when there is no key
func stateMaskOptions(stateKey []byte) ([]xcl.ConfigOption, error) {
	if stateKey == nil {
		return nil, nil
	}

	masker, err := mask.EncryptAES256GCM(stateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid state key: %w", err)
	}

	return []xcl.ConfigOption{xcl.WithStateMask(masker)}, nil
}

func main() {
	dir := "./config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	externalPlugin := "./build/external"
	if len(os.Args) > 2 {
		externalPlugin = os.Args[2]
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

	_, err = run(os.Stdout, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r), r, dir, externalPlugin, stateDir, stateKey)
	os.RemoveAll(stateDir)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the in-process ExamplePlugin and
// the external plugin binary at externalPlugin registered, keeping the state
// in a file in stateDir. It writes the resources and query results to out,
// then destroys everything through the providers and returns the resources
// that were applied. Every event xcl produces, including the plugins' log
// messages, goes to handler, a nil handler leaves xcl silent. The sensitive
// values in state are encrypted with stateKey, or written in plain text, with
// a warning, when it is nil.
func run(out io.Writer, handler xcl.EventHandler, r *registry.PluginRegistry, dir string, externalPlugin string, stateDir string, stateKey []byte) ([]any, error) {
	// The external plugin runs as a separate process, stop it when done
	defer func() {
		for _, host := range r.GetPluginHosts() {
			host.Stop()
		}
	}()

	// Register the in-process plugin, which provides every block type it
	// registers in Init. Registering only records the plugin, it is loaded
	// by the first Apply.
	if err := r.RegisterPlugin(&internal.ExamplePlugin{}); err != nil {
		return nil, err
	}

	// Register the external plugin binary, it is started by the first Apply
	if err := r.RegisterPluginWithPath(externalPlugin); err != nil {
		return nil, err
	}

	masking, err := stateMaskOptions(stateKey)
	if err != nil {
		return nil, err
	}

	options := []xcl.ConfigOption{
		xcl.WithPluginRegistry(r),
		// Keep the state in a file, Destroy works from it alone
		xcl.WithStatePath(stateDir),
		xcl.WithEventHandler(handler),
		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
		// configuration text. Passwords are redacted in it.
		xcl.WithEventData(xcl.EventDataProcessed),
	}

	// with a key, the passwords are encrypted in the state file
	c, err := xcl.NewConfig(append(options, masking...)...)
	if err != nil {
		return nil, err
	}

	if err := c.Apply(dir); err != nil {
		// a plugin that fails to load is reported by the first operation,
		// the likeliest cause is an external plugin that was not built
		if errors.Is(err, xcl.ErrPluginLoad) {
			return nil, fmt.Errorf("%w, build it with `make build` in example/plugin", err)
		}

		return nil, err
	}

	fmt.Fprintln(out, "## Resources")
	for _, res := range c.Entities() {
		meta, err := types.GetMeta(res)
		if err != nil {
			return nil, err
		}

		fmt.Fprintf(out, "  %s\n", meta.ID)
	}

	// Plugin types are held as types generated from the plugin's schema, the
	// lookup copies them into the Go type
	databases, err := xcl.FindByType[resources.PostgreSQL](c, "resource", "postgres")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Databases")
	for _, db := range databases {
		fmt.Fprintf(out, "  %s location=%s port=%d connection_string=%q\n", db.Meta.ID, db.Location, db.Port, db.ConnectionString)
	}

	caches, err := xcl.FindByType[resources.Redis](c, "resource", "redis")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Caches")
	for _, cache := range caches {
		fmt.Fprintf(out, "  %s location=%s port=%d connection_string=%q\n", cache.Meta.ID, cache.Location, cache.Port, cache.ConnectionString)
	}

	app, err := xcl.Find[resources.App](c, "resource.app.web")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## App")
	fmt.Fprintf(out, "  %s database_location=%s database_user=%s analytics_location=%s connection_string=%q cache_connection_string=%q url=%q\n",
		app.Meta.ID, app.DatabaseLocation, app.DatabaseUser, app.AnalyticsLocation, app.ConnectionString, app.CacheConnectionString, app.URL)

	ingress, err := xcl.Find[resources.Ingress](c, "resource.ingress.web")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Ingress")
	fmt.Fprintf(out, "  %s hostname=%s app_url=%q\n", ingress.Meta.ID, ingress.Hostname, ingress.AppURL)

	// An output is an entity like everything else: it is found by its address
	// as a types.Output, and the value it publishes is on its Value field
	webDatabase, err := xcl.Find[types.Output](c, "output.web_database")
	if err != nil {
		return nil, err
	}

	moduleLocation, err := xcl.Find[types.Output](c, "module.analytics.output.location")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Published")
	fmt.Fprintf(out, "  output.web_database=%q\n", webDatabase.Value)
	fmt.Fprintf(out, "  module.analytics.output.location=%q\n", moduleLocation.Value)

	// or every published value at once, keyed by address
	fmt.Fprintf(out, "  %d published in total\n", len(c.Outputs()))

	applied := append([]any{}, c.Entities()...)

	// Destroy everything that was applied, dependents before what they depend
	// on, working only from the saved state
	if err := c.Destroy(); err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Destroyed")
	fmt.Fprintf(out, "  %d resources remaining\n", c.EntityCount())

	return applied, nil
}
