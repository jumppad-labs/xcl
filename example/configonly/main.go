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
	"fmt"
	"os"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/configonly/resources"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/kr/pretty"
)

type appConfig struct {
	Deployments []*resources.Deployment
	Services    []*resources.Service
	Ingresses   []*resources.Ingress
}

func main() {
	dir := "./config"

	// the registry is built here so that the event receiver can share it: it
	// is what types the entity an event carries, which is how the receiver
	// shows each resource's configuration as it is created
	r := registry.NewPluginRegistry()
	r.RegisterType(&resources.ConfigMap{}, "config_map")
	r.RegisterType(&resources.Secret{}, "secret")
	r.RegisterType(&resources.Deployment{}, "deployment")
	r.RegisterType(&resources.Service{}, "service")
	r.RegisterType(&resources.Ingress{}, "ingress")

	c, err := xcl.NewConfig(xcl.WithPluginRegistry(r))
	if err != nil {
		fmt.Printf("Error creating config: %s", err)
		os.Exit(1)
	}

	if err := c.Apply(dir); err != nil {
		fmt.Printf("Error parsing config: %s", err)
		os.Exit(1)
	}

	// gather the configuration into the application's own struct, Decode is
	// an ordinary method so this works on every supported Go version
	var cfg appConfig
	if err := c.Decode(&cfg); err != nil {
		fmt.Printf("Error decoding config: %s", err)
	}

	// write object to the output
	fmt.Println("")
	fmt.Println("-- Config Loaded ---------------")
	pretty.Println(cfg)
}
