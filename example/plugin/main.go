// Command xcl-docker shows xcl used to build a command line tool, with two
// plugins that do real work. Their providers take part in the lifecycle,
// creating real things on apply and removing them on destroy:
//
//   - The Docker plugin (./docker) is an external plugin, a standalone binary
//     that xcl starts as a separate process and calls over gRPC. It provides
//     docker "network" and docker "container", and creates real Docker
//     networks and containers.
//   - The template plugin (./template) is an in-process plugin, compiled into
//     this program. It provides template, a block type with no subtype, and
//     renders a Handlebars template to a file.
//
// It has three commands, each a separate run of the program sharing the state
// saved in a directory, ./.xcl-docker by default:
//
//	xcl-docker apply [flags] <path>   apply the configuration at path, print what it created
//	xcl-docker status [flags]         print what the saved state holds
//	xcl-docker destroy [flags]        remove everything in the saved state
//
// The flags are --state <dir>, the directory the state is kept in, and
// --plugin <path>, the Docker plugin binary, docker-plugin next to the
// xcl-docker executable by default.
//
// The configuration in ./config is an example to apply. The template reads the
// container's address, which the Docker plugin computes when it creates the
// container, so a value crosses from one plugin to the other.
//
// apply and destroy need a Docker engine, reached through DOCKER_HOST or the
// default socket. `make build` builds xcl-docker and the Docker plugin side by
// side into ./build, where xcl-docker finds the plugin. `make run` applies the
// example configuration, prints the status and destroys it again.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/docker/client"
	"github.com/jumppad-labs/xcl/example/plugin/docker/resources"
	"github.com/jumppad-labs/xcl/example/plugin/template"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// defaultStateDir is where the commands keep the state between runs, unless
// --state names another directory
const defaultStateDir = "./.xcl-docker"

// dockerPluginName is the file name of the Docker plugin binary, which
// xcl-docker looks for next to its own executable unless --plugin names it
const dockerPluginName = "docker-plugin"

const usage = `usage: xcl-docker <command> [flags]

commands:
  apply [flags] <path>  apply the configuration at path and print what it created
  status [flags]        print what the saved state holds
  destroy [flags]       remove everything in the saved state

flags, given before the path:
  --state <dir>     directory the state is kept in (default ./.xcl-docker)
  --plugin <path>   Docker plugin binary (default docker-plugin next to xcl-docker)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs the command args names, writing its results to stdout and its
// events and errors to stderr, and returns the program's exit code
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	command := args[0]

	flags := flag.NewFlagSet("xcl-docker "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }

	stateDir := flags.String("state", defaultStateDir, "directory the state is kept in")
	dockerPlugin := flags.String("plugin", defaultDockerPlugin(), "Docker plugin binary")

	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}

	var err error

	switch command {
	case "apply":
		if flags.NArg() != 1 {
			fmt.Fprint(stderr, "apply needs the path of the configuration to apply\n\n"+usage)
			return 2
		}

		err = applyCommand(stdout, stderr, flags.Arg(0), *dockerPlugin, *stateDir)
	case "status":
		err = statusCommand(stdout, stderr, *dockerPlugin, *stateDir)
	case "destroy":
		err = destroyCommand(stdout, stderr, *dockerPlugin, *stateDir)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usage)
		return 2
	}

	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}

	return 0
}

// defaultDockerPlugin returns the path of the Docker plugin binary next to
// this program's executable, where `make build` puts it
func defaultDockerPlugin() string {
	executable, err := os.Executable()
	if err != nil {
		return dockerPluginName
	}

	return filepath.Join(filepath.Dir(executable), dockerPluginName)
}

// applyCommand applies the configuration at configDir and prints what it
// created. The state is saved in stateDir, so status and destroy see it.
func applyCommand(stdout, stderr io.Writer, configDir, dockerPlugin, stateDir string) error {
	// The Docker plugin creates real containers, check an engine answers
	// before applying anything
	if err := client.Ping(context.Background()); err != nil {
		return err
	}

	// the registry is built here so that the event receiver can share it: it
	// is what types the entity an event carries, which is how the receiver
	// shows each resource's configuration as it is created
	r := registry.NewPluginRegistry()

	c, err := newConfig(r, eventHandler(stderr, r), dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	if err := apply(c, configDir); err != nil {
		return err
	}

	return report(stdout, c)
}

// statusCommand prints what the state saved in stateDir holds. It reads the
// state alone, needing neither the configuration nor a Docker engine.
func statusCommand(stdout, stderr io.Writer, dockerPlugin, stateDir string) error {
	r := registry.NewPluginRegistry()

	c, err := newConfig(r, eventHandler(stderr, r), dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	if err := load(c); err != nil {
		return err
	}

	return report(stdout, c)
}

// destroyCommand removes everything in the state saved in stateDir, working
// from the state alone
func destroyCommand(stdout, stderr io.Writer, dockerPlugin, stateDir string) error {
	if err := client.Ping(context.Background()); err != nil {
		return err
	}

	r := registry.NewPluginRegistry()

	c, err := newConfig(r, eventHandler(stderr, r), dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	return destroy(stdout, c)
}

// eventHandler returns the receiver every command gives xcl, which writes
// each event to out at the level XCL_LOG_LEVEL names
func eventHandler(out io.Writer, r *registry.PluginRegistry) xcl.EventHandler {
	return prettylog.Handler(out, prettylog.LevelFromEnv(), r)
}

// newConfig registers the in-process template plugin and the Docker plugin
// binary at dockerPlugin with r, and returns a Config keeping its state in
// stateDir. Every event xcl produces, including the plugins' log messages,
// goes to handler, a nil handler leaves xcl silent.
func newConfig(r *registry.PluginRegistry, handler xcl.EventHandler, dockerPlugin, stateDir string) (*xcl.Config, error) {
	// Register the in-process plugin, which provides every block type it
	// registers in Init. Registering only records the plugin, it is loaded
	// by the first operation.
	if err := r.RegisterPlugin(&template.TemplatePlugin{}); err != nil {
		return nil, err
	}

	// Register the external plugin binary, it is started by the first
	// operation
	if err := r.RegisterPluginWithPath(dockerPlugin); err != nil {
		return nil, err
	}

	return xcl.NewConfig(
		xcl.WithPluginRegistry(r),
		// Keep the state in a file, so each command, a separate run of the
		// program, works from what the last one saved
		xcl.WithStatePath(stateDir),
		xcl.WithEventHandler(handler),
		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
		// configuration text
		xcl.WithEventData(xcl.EventDataProcessed),
	)
}

// apply applies the configuration in configDir. What it applied is saved,
// even when applying fails part way, so destroy can remove it.
func apply(c *xcl.Config, configDir string) error {
	return explainPluginLoad(c.Apply(configDir))
}

// load reads the saved state into c, without applying anything
func load(c *xcl.Config) error {
	return explainPluginLoad(c.Load())
}

// explainPluginLoad adds how to fix it to err when a plugin failed to load,
// the likeliest cause is a Docker plugin that was not built
func explainPluginLoad(err error) error {
	if errors.Is(err, xcl.ErrPluginLoad) {
		return fmt.Errorf("%w, build it with `make build` in example/plugin", err)
	}

	return err
}

// report writes the networks, containers and rendered templates c applied to
// out
func report(out io.Writer, c *xcl.Config) error {
	// Plugin types are held as types generated from the plugin's schema, the
	// lookup copies them into the plugin's Go type
	networks, err := xcl.FindByType[resources.Network](c, "docker", "network")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## Networks")
	for _, n := range networks {
		fmt.Fprintf(out, "  %s name=%s subnet=%s docker_id=%s\n", n.Meta.ID, n.Meta.Name, n.Subnet, n.DockerID)
	}

	containers, err := xcl.FindByType[resources.Container](c, "docker", "container")
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
	if err := explainPluginLoad(c.Destroy()); err != nil {
		return err
	}

	fmt.Fprintln(out, "## Destroyed")
	fmt.Fprintf(out, "  %d resources remaining\n", c.EntityCount())

	return nil
}
