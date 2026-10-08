// Command xcl-docker shows xcl used to build a command line tool, with two
// plugins that do real work. Their providers take part in the lifecycle,
// creating real things on apply and removing them on destroy:
//
//   - The Docker plugin (./plugins/docker) is an external plugin, a standalone
//     binary that xcl starts as a separate process and calls over gRPC. It
//     provides docker "network" and docker "container", and creates real
//     Docker networks and containers.
//   - The template plugin (./plugins/template) is an in-process plugin,
//     compiled into this program. It provides template, a block type with no
//     subtype, and renders a Handlebars template to a file.
//
// It has five commands, each a separate run of the program sharing the state
// saved in a directory, ./.xcl-docker by default:
//
//	xcl-docker apply [flags] <path>       apply the configuration at path
//	xcl-docker plan [flags] <path>        print what applying path would change
//	xcl-docker status [flags]             print what the saved state holds as a tree
//	xcl-docker inspect [flags] <address>  print the resource at address as configuration
//	xcl-docker destroy [flags]            remove everything in the saved state
//
// The flags are --state <dir>, the directory the state is kept in, and
// --plugin <path>, the Docker plugin binary, docker-plugin next to the
// xcl-docker executable by default.
//
// The configuration in ./config is an example to apply. The template reads the
// container's address, which the Docker plugin computes when it creates the
// container, so a value crosses from one plugin to the other.
//
// apply, plan and destroy need a Docker engine, reached through DOCKER_HOST or the
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
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/template"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/registry"
)

// defaultStateDir is where the commands keep the state between runs, unless
// --state names another directory
const defaultStateDir = "./.xcl-docker"

// dockerPluginName is the file name of the Docker plugin binary, which
// xcl-docker looks for next to its own executable unless --plugin names it
const dockerPluginName = "docker-plugin"

const usage = `usage: xcl-docker <command> [flags]

commands:
  apply [flags] <path>       apply the configuration at path
  plan [flags] <path>        print what applying the configuration at path would change
  status [flags]             print what the saved state holds as a tree
  inspect [flags] <address>  print the resource at address as configuration
  destroy [flags]            remove everything in the saved state

flags, given before the path or address:
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

		err = applyCommand(stderr, flags.Arg(0), *dockerPlugin, *stateDir)
	case "plan":
		if flags.NArg() != 1 {
			fmt.Fprint(stderr, "plan needs the path of the configuration to compare\n\n"+usage)
			return 2
		}

		err = planCommand(stdout, flags.Arg(0), *dockerPlugin, *stateDir)
	case "status":
		err = statusCommand(stdout, *dockerPlugin, *stateDir)
	case "inspect":
		if flags.NArg() != 1 {
			fmt.Fprint(stderr, "inspect needs the address of the resource to inspect\n\n"+usage)
			return 2
		}

		err = inspectCommand(stdout, flags.Arg(0), *dockerPlugin, *stateDir)
	case "destroy":
		err = destroyCommand(stderr, *dockerPlugin, *stateDir)
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

// applyCommand applies the configuration at configDir. It prints nothing of
// its own, the events written to stderr end with the apply's success or
// error. The state is saved in stateDir, so status and destroy see it.
func applyCommand(stderr io.Writer, configDir, dockerPlugin, stateDir string) error {
	// The Docker plugin creates real containers, check an engine answers
	// before applying anything
	if err := client.Ping(context.Background()); err != nil {
		return err
	}

	c, err := newConfig(eventHandler(stderr), dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	return apply(c, configDir)
}

// planCommand prints what applying the configuration at configDir would change
// in the state saved in stateDir, as a diff, without changing anything. Each
// resource in the state is read through its provider, as apply reads it, so a
// Docker engine must answer. It prints only the diff, so xcl is given no
// event handler and stays silent.
func planCommand(stdout io.Writer, configDir, dockerPlugin, stateDir string) error {
	if err := client.Ping(context.Background()); err != nil {
		return err
	}

	c, err := newConfig(nil, dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	return plan(stdout, c, configDir)
}

// statusCommand prints what the state saved in stateDir holds as a tree. It
// reads the state alone, needing neither the configuration nor a Docker
// engine. It prints only the tree, so xcl is given no event handler and stays
// silent.
func statusCommand(stdout io.Writer, dockerPlugin, stateDir string) error {
	c, err := newConfig(nil, dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	if err := load(c); err != nil {
		return err
	}

	return status(stdout, c)
}

// inspectCommand prints the resource at address in the state saved in
// stateDir as configuration text. Like status it reads the state alone and
// prints only the text, so xcl is given no event handler and stays silent.
func inspectCommand(stdout io.Writer, address, dockerPlugin, stateDir string) error {
	c, err := newConfig(nil, dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	if err := load(c); err != nil {
		return err
	}

	return inspect(stdout, c, address)
}

// destroyCommand removes everything in the state saved in stateDir, working
// from the state alone. Like apply it prints nothing of its own, the events
// written to stderr end with the destroy's success or error.
func destroyCommand(stderr io.Writer, dockerPlugin, stateDir string) error {
	if err := client.Ping(context.Background()); err != nil {
		return err
	}

	c, err := newConfig(eventHandler(stderr), dockerPlugin, stateDir)
	if err != nil {
		return err
	}

	return destroy(c)
}

// eventHandler returns the receiver every command gives xcl, which writes
// each event to out at the level XCL_LOG_LEVEL names
func eventHandler(out io.Writer) xcl.EventHandler {
	return prettylog.Handler(out, prettylog.LevelFromEnv())
}

// newConfig adds a local registry holding the in-process template plugin and
// the Docker plugin binary at dockerPlugin, and returns a Config keeping its
// state in stateDir. Every event xcl produces, including the plugins' log
// messages, goes to handler, a nil handler leaves xcl silent.
func newConfig(handler xcl.EventHandler, dockerPlugin, stateDir string) (*xcl.Config, error) {
	// The local registry holds plugins the application supplies itself: the
	// in-process plugin, which provides every block type it registers in
	// Init, and the external plugin binary. Registering only records a
	// plugin, each is loaded, and the binary started, by the first operation
	// that needs plugins.
	local := registry.NewLocal()
	local.RegisterPlugin(&template.TemplatePlugin{})
	local.RegisterExternalPlugin(dockerPlugin)

	return xcl.NewConfig(
		xcl.WithRegistry(local),
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

// destroy removes everything c applied, dependents before what they depend
// on, working only from the saved state
func destroy(c *xcl.Config) error {
	return explainPluginLoad(c.Destroy())
}
