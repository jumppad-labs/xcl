// Package registry defines where a Config gets its plugins from. A Registry
// provides plugins, and a Plugin starts itself and returns the host xcl talks
// to it through. A Config is given registries with xcl.WithRegistry and uses
// every one, in the order given.
//
// The package ships the local registry, NewLocal, which holds plugins
// compiled into the program, plugin binaries and directories of plugin
// binaries, and the two ways of starting a plugin it needs: InProcess and
// Executable. Third parties write registries and plugin starters of their own
// against the same interfaces.
package registry

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/plugins"
)

// Registry provides plugins to a Config. A Config uses every registry given to
// it with xcl.WithRegistry, in that order.
type Registry interface {
	// Name identifies the registry in events and errors, i.e. "local" or
	// "registry.xcl.dev"
	Name() string

	// Plugins returns the plugins this registry provides, fetching them first
	// if it needs to. It is called once per Config, when plugins load.
	Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error)
}

// Plugin is one plugin a registry provides, ready to start
type Plugin interface {
	// Name identifies the plugin in events and errors
	Name() string

	// Start runs the plugin and returns the host xcl talks to it through
	Start(emit events.Emit) (plugins.PluginHost, error)
}

// InProcess returns a Plugin for p, a plugin compiled into the program. It is
// started with the direct host, in the same process, and is known by the
// name of its Go type. A nil p is a programmer error and panics.
func InProcess(p plugins.Plugin) Plugin {
	if p == nil {
		panic("xcl: registry: in-process plugin must not be nil")
	}

	return inProcess{plugin: p}
}

// inProcess starts a plugin compiled into the program
type inProcess struct {
	plugin plugins.Plugin
}

// Name returns the name of the plugin's Go type
func (p inProcess) Name() string {
	return plugins.PluginName(p.plugin)
}

// Start creates the plugin's direct host, the plugin's log messages go to emit
func (p inProcess) Start(emit events.Emit) (plugins.PluginHost, error) {
	return plugins.NewDirectPluginHost(emit, nil, p.plugin)
}

// Executable returns a Plugin for the plugin binary at path. It is started as
// a separate process over gRPC, and is known by the binary's file name. A
// path that does not exist is not checked here, it fails when the plugin is
// started.
func Executable(path string) Plugin {
	return executable{path: path}
}

// executable starts a plugin binary as a process over gRPC
type executable struct {
	path string
}

// Name returns the binary's file name, without a Windows .exe extension
func (p executable) Name() string {
	return plugins.PluginBinaryName(p.path)
}

// Start runs the binary and connects to it. The host it returns can stop and
// restart the process, which a Config does around each operation.
func (p executable) Start(emit events.Emit) (plugins.PluginHost, error) {
	host := plugins.NewGRPCPluginHost(emit, nil)

	if err := host.Start(p.path); err != nil {
		return nil, fmt.Errorf("unable to start plugin binary %s: %w", p.path, err)
	}

	return host, nil
}
