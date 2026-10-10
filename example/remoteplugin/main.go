// Command xcl-docker shows xcl used to build a command line tool, with two
// plugins that do real work. Their providers take part in the lifecycle,
// creating real things on apply and removing them on destroy:
//
//   - The Docker plugin (./plugins/docker) is an external plugin, a module of
//     its own laid out in the standard plugin layout. Its entry point,
//     ./plugins/docker/cmd/docker, is a standalone binary that xcl starts as
//     a separate process and calls over gRPC. It provides docker "network"
//     and docker "container", and creates real Docker networks and
//     containers. This program imports only its entity types,
//     ./plugins/docker/entities, to read the blocks it applied.
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
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/registry"
)

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

		err = applyCommand(stderr, flags.Arg(0), *stateDir)
	case "destroy":
		err = destroyCommand(stderr, *stateDir)
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

func destroyCommand(stderr io.Writer, stateDir string) error {
	c, err := newConfig(eventHandler(stderr), stateDir)
	if err != nil {
		return err
	}

	return c.Destroy()
}

func applyCommand(stderr io.Writer, config, stateDir string) error {
	c, err := newConfig(eventHandler(stderr), stateDir)
	if err != nil {
		return err
	}

	return c.Apply(config)
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
func newConfig(handler xcl.EventHandler, stateDir string) (*xcl.Config, error) {
	releaseKey, err := fetchKey(releaseKeyURL)
	if err != nil {
		return nil, err
	}

	github := registry.NewGitHub(
		registry.GitHubCacheDir("./.xcl-cache"),
		registry.GitHubTrustedKeys(releaseKey),
	)

	github.RegisterPlugin("jumppad-labs/xcl-plugin-docker", "v0.2.0")

	return xcl.NewConfig(
		xcl.WithRegistry(github),
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

// releaseKeyURL is where the public key that signs jumppad-labs plugin
// releases is published
const releaseKeyURL = "https://xcl.dev/keys/jumppad-labs-releases.asc"

// fetchKey downloads the ASCII-armoured public key at url, which the GitHub
// registry checks each plugin release's signature against
func fetchKey(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	response, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching release key %s: %w", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching release key %s: %s", url, response.Status)
	}

	key, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading release key %s: %w", url, err)
	}

	return string(key), nil
}

// defaultStateDir is where the commands keep the state between runs, unless
// --state names another directory
const defaultStateDir = "./.xcl-docker"

const usage = `usage: xcl-docker <command> [flags]

commands:
  apply <path>       apply the configuration at path
  destroy            remove everything in the saved state
`
