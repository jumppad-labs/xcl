// Command plugin shows xcl used with two plugins that do real work. Their
// providers take part in the lifecycle, creating real things on apply and
// removing them on destroy:
//
//   - The Docker plugin (./docker) is an external plugin, served by its own
//     binary (./cmd/docker-plugin) that xcl starts as a separate process and
//     calls over gRPC. It provides docker "network" and docker "container",
//     and creates real Docker networks and containers.
//   - The template plugin (./template) is an in-process plugin, compiled into
//     this program. It provides template, a block type with no subtype, and
//     renders a Handlebars template to a file.
//
// The configuration it applies is in ./config. The template reads the
// container's address, which the Docker plugin computes when it creates the
// container, so a value crosses from one plugin to the other.
//
// It needs a Docker engine, reached through DOCKER_HOST or the default socket.
// Build the Docker plugin and run the example from this directory with
// `make run`, see the Makefile for the other targets. The configuration
// directory and the Docker plugin binary can be passed as arguments:
// `go run . <config dir> <docker plugin binary>`, they default to ./config
// and ./build/docker-plugin.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/docker"
	"github.com/jumppad-labs/xcl/example/plugin/template"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

func main() {
	configDir := "./config"
	if len(os.Args) > 1 {
		configDir = os.Args[1]
	}

	dockerPlugin := "./build/docker-plugin"
	if len(os.Args) > 2 {
		dockerPlugin = os.Args[2]
	}

	// The Docker plugin creates real containers, check an engine answers
	// before applying anything
	if err := docker.Ping(context.Background()); err != nil {
		exit(err)
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example-plugin")
	if err != nil {
		exit(err)
	}

	// the registry is built here so that the event receiver can share it: it
	// is what types the entity an event carries, which is how the receiver
	// shows each resource's configuration as it is created
	r := registry.NewPluginRegistry()

	c, err := apply(r, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r), configDir, dockerPlugin, stateDir)
	if err == nil {
		err = report(os.Stdout, c)
	}

	// Destroy whatever was applied, even when applying or reporting failed
	// part way, so the example never leaves containers behind
	if c != nil && c.EntityCount() > 0 {
		err = errors.Join(err, destroy(os.Stdout, c))
	}

	// The Docker plugin runs as a separate process, stop it when done
	for _, host := range r.GetPluginHosts() {
		host.Stop()
	}

	os.RemoveAll(stateDir)

	if err != nil {
		exit(err)
	}
}

// exit reports err and ends the program with a failure
func exit(err error) {
	fmt.Fprintf(os.Stderr, "error: %s\n", err)
	os.Exit(1)
}

// apply registers the in-process template plugin and the Docker plugin binary
// at dockerPlugin with r, then applies the configuration in configDir,
// keeping the state in a file in stateDir. Every event xcl produces, including
// the plugins' log messages, goes to handler, a nil handler leaves xcl silent.
//
// It returns the Config whenever one was created, even when applying failed,
// so the caller can destroy what was applied.
func apply(r *registry.PluginRegistry, handler xcl.EventHandler, configDir, dockerPlugin, stateDir string) (*xcl.Config, error) {
	// Register the in-process plugin, which provides every block type it
	// registers in Init. Registering only records the plugin, it is loaded
	// by the first Apply.
	if err := r.RegisterPlugin(&template.TemplatePlugin{}); err != nil {
		return nil, err
	}

	// Register the external plugin binary, it is started by the first Apply
	if err := r.RegisterPluginWithPath(dockerPlugin); err != nil {
		return nil, err
	}

	c, err := xcl.NewConfig(
		xcl.WithPluginRegistry(r),
		// Keep the state in a file, Destroy works from it alone
		xcl.WithStatePath(stateDir),
		xcl.WithEventHandler(handler),
		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
		// configuration text
		xcl.WithEventData(xcl.EventDataProcessed),
	)
	if err != nil {
		return nil, err
	}

	if err := c.Apply(configDir); err != nil {
		// a plugin that fails to load is reported by the first operation,
		// the likeliest cause is a Docker plugin that was not built
		if errors.Is(err, xcl.ErrPluginLoad) {
			return c, fmt.Errorf("%w, build it with `make build` in example/plugin", err)
		}

		return c, err
	}

	return c, nil
}

// report writes the networks, containers and rendered templates c applied to
// out
func report(out io.Writer, c *xcl.Config) error {
	// Plugin types are held as types generated from the plugin's schema, the
	// lookup copies them into the plugin's Go type
	networks, err := xcl.FindByType[docker.Network](c, "docker", "network")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Networks")
	for _, n := range networks {
		fmt.Fprintf(out, "  %s name=%s subnet=%s docker_id=%s\n", n.Meta.ID, n.Meta.Name, n.Subnet, n.DockerID)
	}

	containers, err := xcl.FindByType[docker.Container](c, "docker", "container")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Containers")
	for _, ctr := range containers {
		fmt.Fprintf(out, "  %s image=%s ip_address=%s docker_id=%s\n", ctr.Meta.ID, ctr.Image, ctr.IPAddress, ctr.DockerID)
	}

	// template has no subtype, so its address has one segment before the name
	templates, err := xcl.FindByType[template.Template](c, "template")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Templates")
	for _, t := range templates {
		rendered, err := os.ReadFile(t.Destination)
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "  %s destination=%s\n", t.Meta.ID, t.Destination)
		fmt.Fprintf(out, "%s", rendered)
	}

	return nil
}

// destroy removes everything c applied, dependents before what they depend
// on, working only from the saved state, and writes what remains to out
func destroy(out io.Writer, c *xcl.Config) error {
	if err := c.Destroy(); err != nil {
		return err
	}

	fmt.Fprintln(out, "## Destroyed")
	fmt.Fprintf(out, "  %d resources remaining\n", c.EntityCount())

	return nil
}
