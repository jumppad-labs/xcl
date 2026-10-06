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
	"io"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/configonly/resources"
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

	_, err = run(os.Stdout, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r), r, dir, stateDir, stateKey)
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
func run(out io.Writer, handler xcl.EventHandler, r *registry.PluginRegistry, dir string, stateDir string, stateKey []byte) ([]any, error) {

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

	options := []xcl.ConfigOption{
		xcl.WithPluginRegistry(r),
		// Keep the state in a file, Destroy works from it alone
		xcl.WithStatePath(stateDir),
		xcl.WithEventHandler(handler),
		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
		// configuration text
		// By default sensitive fields are redacted in the output
		xcl.WithEventData(xcl.EventDataProcessed),
	}

	// Encrypt sensitive values in the state when there is a key, without one
	// they are written in plain text and xcl warns about it
	if stateKey != nil {
		masker, err := mask.EncryptAES256GCM(stateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid state key: %w", err)
		}

		options = append(options, xcl.WithStateMask(masker))
	}

	c, err := xcl.NewConfig(options...)
	if err != nil {
		return nil, err
	}

	if err := c.Apply(dir); err != nil {
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

	// gather the configuration into the application's own struct, Decode is
	// an ordinary method so this works on every supported Go version
	var cfg appConfig
	if err := c.Decode(&cfg); err != nil {
		return nil, err
	}

	printDeployments(out, cfg.Deployments)

	if err := printRouting(out, cfg.Service, cfg.Ingress); err != nil {
		return nil, err
	}

	applied := append([]any{}, c.Entities()...)

	// Destroy everything that was applied, dependents before what they depend
	// on, working only from the saved state. For these types that means
	// clearing them from state, since none of them has a provider
	if err := c.Destroy(); err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "## Destroyed")
	fmt.Fprintf(out, "  %d resources remaining\n", c.EntityCount())

	return applied, nil
}

// printDeployments writes each deployment and walks the blocks nested inside
// it, the containers and, for each of those, its ports, environment and
// resource limits. Registered types come back as the Go type that was
// registered, so the nested blocks are ordinary Go structs and slices.
func printDeployments(out io.Writer, deployments []*resources.Deployment) {
	fmt.Fprintln(out, "## Deployments")
	for _, d := range deployments {
		fmt.Fprintf(out, "  %s replicas=%d\n", d.Meta.ID, d.Replicas)

		for _, container := range d.Containers {
			fmt.Fprintf(out, "    container %s image=%s\n", container.Name, container.Image)

			for _, port := range container.Ports {
				fmt.Fprintf(out, "      port %s container_port=%d\n", port.Name, port.ContainerPort)
			}

			// env values were read from the config map, the reference is
			// resolved by the time the resource is returned
			for _, env := range container.Env {
				fmt.Fprintf(out, "      env %s=%s\n", env.Name, env.Value)
			}

			// A block that appears once is a pointer, nil when the
			// configuration leaves it out
			if container.Resources != nil {
				fmt.Fprintf(out, "      limits cpu=%s memory=%s\n", container.Resources.Limits.CPU, container.Resources.Limits.Memory)
				fmt.Fprintf(out, "      requests cpu=%s memory=%s\n", container.Resources.Requests.CPU, container.Resources.Requests.Memory)
			}

			for _, mount := range container.VolumeMounts {
				fmt.Fprintf(out, "      volume_mount %s path=%s\n", mount.Name, mount.Path)
			}
		}

		for _, volume := range d.Volumes {
			fmt.Fprintf(out, "    volume %s config_map=%s\n", volume.Name, volume.ConfigMap)
		}
	}
}

// printRouting writes the two resources that are linked to the deployment,
// their values were read from the blocks they reference rather than repeated
// in the configuration. The configuration declares one of each, a missing one
// is reported rather than printed
func printRouting(out io.Writer, service *resources.Service, ingress *resources.Ingress) error {
	if service == nil {
		return fmt.Errorf("the configuration declares no service")
	}

	if ingress == nil {
		return fmt.Errorf("the configuration declares no ingress")
	}

	fmt.Fprintln(out, "## Service")
	fmt.Fprintf(out, "  %s deployment=%s port=%d target_port=%d\n", service.Meta.ID, service.Deployment, service.Port, service.TargetPort)

	fmt.Fprintln(out, "## Ingress")
	fmt.Fprintf(out, "  %s host=%s\n", ingress.Meta.ID, ingress.Host)

	for _, rule := range ingress.Rules {
		fmt.Fprintf(out, "    rule path=%s service=%s port=%d\n", rule.Path, rule.Service, rule.Port)
	}

	return nil
}
