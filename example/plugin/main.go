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
// Build the external plugin and run the example from this directory with
// `make run`, see the Makefile for the other targets. The configuration
// directory and the external plugin binary can be passed as arguments:
// `go run . <config dir> <external plugin binary>`, they default to ./config
// and ./build/external.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/internal"
	"github.com/jumppad-labs/xcl/example/plugin/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

func main() {
	dir := "./config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	externalPlugin := "./build/external"
	if len(os.Args) > 2 {
		externalPlugin = os.Args[2]
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	_, err = run(os.Stdout, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv()), dir, externalPlugin, filepath.Join(stateDir, "state.json"))
	os.RemoveAll(stateDir)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the in-process ExamplePlugin and
// the external plugin binary at externalPlugin registered, keeping the state
// in a file at statePath. It writes the resources and query results to out,
// then destroys everything through the providers and returns the resources
// that were applied. Every event xcl produces, including the plugins' log
// messages, goes to handler, a nil handler leaves xcl silent.
func run(out io.Writer, handler xcl.EventHandler, dir string, externalPlugin string, statePath string) ([]any, error) {
	r := registry.NewPluginRegistry()

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

	// Keep the state in a file, Destroy works from it alone
	store, err := state.NewFileStateStore(statePath, r)
	if err != nil {
		return nil, err
	}

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(r),
		xcl.WithStateStore(store),
		xcl.WithEventHandler(handler),
	)

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

	// A value the configuration publishes is read by its address like anything
	// else, and comes back as the value itself rather than the declaration
	// that produced it. Nothing here needs to know how outputs are stored
	webDatabase, err := xcl.Find[string](c, "output.web_database")
	if err != nil {
		return nil, err
	}

	moduleLocation, err := xcl.Find[string](c, "module.analytics.output.location")
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Published")
	fmt.Fprintf(out, "  output.web_database=%q\n", *webDatabase)
	fmt.Fprintf(out, "  module.analytics.output.location=%q\n", *moduleLocation)

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
