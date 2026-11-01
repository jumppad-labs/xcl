// Package registry defines where a Config gets its block types, functions
// and plugins from. A Registry declares plain Go types, which blocks are only
// decoded into, offers Go functions that configuration can call, and provides
// plugins, and a Plugin starts itself and returns the host xcl talks to it
// through. A Config is given registries with xcl.WithRegistry and uses every
// one, in the order given.
//
// The package ships the local registry, NewLocal, which holds Go types
// declared with RegisterType, functions registered with RegisterFunction,
// plugins compiled into the program, external plugins run from a command and
// directories of plugin binaries. It also ships the GitHub registry,
// NewGitHub, which installs plugins from GitHub releases into a local cache,
// verifies them and starts them as plugin binaries. Third parties write
// registries of their own against the same interfaces, starting compiled-in
// plugins with InProcess.
package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/plugins"
)

// Registry provides types, functions and plugins to a Config. A Config uses
// every registry given to it with xcl.WithRegistry, in that order.
type Registry interface {
	// Name identifies the registry in events and errors, i.e. "local" or
	// "registry.xcl.dev"
	Name() string

	// Types returns the plain Go types this registry declares. It is read
	// once, by NewConfig. A registry with no Go types returns nil.
	Types() []Type

	// Functions returns the Go functions this registry offers to
	// configuration. It is read once, by NewConfig, which reports a function
	// named like a builtin or like another application function. A registry
	// with no functions returns nil.
	Functions() []Function

	// Plugins returns the plugins this registry provides, fetching them first
	// if it needs to. It is called once per Config, when plugins load.
	Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error)
}

// Type is a plain Go type declared as a block type. It has no provider: it
// is only decoded into.
type Type struct {
	// Type is the block type, i.e. "deployment" or "resource"
	Type string

	// Subtype is the subtype, empty for a type declared without one
	Subtype string

	// Prototype is a pointer to a plain struct, the type's Go type; a struct
	// embedding types.ResourceBase is refused when the type is declared
	Prototype any
}

// Function is a Go func a registry offers to configuration under Name, so
// that name = slugify("My App") calls it. Func takes string, bool and number
// parameters, or slices and string-keyed maps of those, and returns a result
// of such a type and an error, i.e. func(title string) (string, error). It is
// called while configuration is read, so an entity only ever holds its
// result.
type Function struct {
	// Name is the bare name configuration calls the function by, i.e.
	// "slugify". Names are not namespaced, so one that a builtin or another
	// function already uses is reported by NewConfig
	Name string

	// Func is the Go func
	Func any
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

// executablePlugin returns a Plugin for the program at path, started with
// args. It is started as a separate process over gRPC. A path that does not
// exist is not checked here, it fails when the plugin is started.
func executablePlugin(path string, args ...string) Plugin {
	return executable{path: path, args: args}
}

// executable starts a program as a plugin process over gRPC
type executable struct {
	path string
	args []string
}

// Name returns the binary's file name, without a Windows .exe extension, for
// a plugin started without arguments, and otherwise the whole command line,
// since the program alone, such as python3, says nothing about the plugin.
// The name only labels the plugin in load events and errors.
func (p executable) Name() string {
	if len(p.args) == 0 {
		return plugins.PluginBinaryName(p.path)
	}

	return strings.Join(append([]string{p.path}, p.args...), " ")
}

// Start runs the binary with its arguments and connects to it. The host it returns can stop and
// restart the process, which a Config does around each operation.
func (p executable) Start(emit events.Emit) (plugins.PluginHost, error) {
	host := plugins.NewGRPCPluginHost(emit, nil)

	if err := host.Start(p.path, p.args...); err != nil {
		return nil, fmt.Errorf("unable to start plugin binary %s: %w", p.path, err)
	}

	return host, nil
}
