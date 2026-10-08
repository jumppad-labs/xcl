package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/declaration"
	"github.com/jumppad-labs/xcl/plugins"
)

// LocalName is the Name of every local registry
const LocalName = "local"

// DefaultPluginPattern is the file name pattern a local registry searches
// plugin directories for when no PluginPattern is given
const DefaultPluginPattern = "xcl-plugin-*"

// Local is a registry of types and plugins found on this machine: plain Go
// types, plugins compiled into the program, plugin binaries named by path, and
// directories searched for plugin binaries. Registering only records, nothing
// is started or searched until a Config loads its plugins, and its plugins
// load in the order they were registered.
type Local struct {
	mu      sync.Mutex
	pattern string
	types   []Type
	entries []localEntry
}

// localEntry is one registration, either a plugin or a directory to search
type localEntry struct {
	plugin    Plugin
	directory string
}

// LocalOption configures a local registry when it is created
type LocalOption func(*Local)

// PluginPattern sets the file name pattern, as filepath.Match takes it, that
// the registry searches plugin directories for. It is fixed when the registry
// is created, so a later registration cannot change what an earlier one
// matches. An empty pattern keeps DefaultPluginPattern.
func PluginPattern(pattern string) LocalOption {
	return func(l *Local) {
		if pattern != "" {
			l.pattern = pattern
		}
	}
}

// NewLocal returns an empty local registry
//
//	local := registry.NewLocal()
//	local.RegisterType(&resources.Deployment{}, "deployment")
//	local.RegisterPlugin(&template.TemplatePlugin{})
//	local.RegisterExternalPlugin("./bin/xcl-plugin-docker")
//	local.RegisterPluginDirectory("~/.xcl/plugins")
func NewLocal(options ...LocalOption) *Local {
	l := &Local{pattern: DefaultPluginPattern}

	for _, option := range options {
		option(l)
	}

	return l
}

// Name returns LocalName
func (l *Local) Name() string {
	return LocalName
}

// RegisterType declares prototype, a plain Go type, as a block type. name is
// the block type and an optional subtype:
//
//	local.RegisterType(&Database{}, "database")             // database "main" {}
//	local.RegisterType(&Database{}, "resource", "database") // resource "database" "main" {}
//
// prototype is a pointer to a struct that embeds types.ResourceBase. A
// declared type has no provider, it is only decoded into.
//
// A declaration that is wrong on every run is a programmer error and panics
// here, naming the type: an empty name, more than one subtype, an empty
// subtype, or a prototype that is not a pointer to a struct embedding
// types.ResourceBase. A declaration that clashes with another one, in this
// registry or another, or with a builtin, is returned by xcl.NewConfig. A
// plugin providing the same block type is reported when plugins load.
func (l *Local) RegisterType(prototype any, name ...string) {
	if err := declaration.Validate(prototype, name...); err != nil {
		panic(fmt.Sprintf("xcl: registry %s: %s", LocalName, err))
	}

	declared := Type{Type: name[0], Prototype: prototype}
	if len(name) == 2 {
		declared.Subtype = name[1]
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.types = append(l.types, declared)
}

// Types returns the Go types declared with RegisterType, in the order they
// were registered, or nil when none were
func (l *Local) Types() []Type {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.types) == 0 {
		return nil
	}

	return append([]Type{}, l.types...)
}

// RegisterPlugin registers p, a plugin compiled into the program. A nil p is
// a programmer error and panics.
func (l *Local) RegisterPlugin(p plugins.Plugin) {
	if p == nil {
		panic(fmt.Sprintf("xcl: registry %s: in-process plugin must not be nil", LocalName))
	}

	l.add(localEntry{plugin: InProcess(p)})
}

// RegisterExternalPlugin registers the plugin binary at path. A path that
// does not exist is reported when plugins load, not here.
func (l *Local) RegisterExternalPlugin(path string) {
	l.add(localEntry{plugin: Executable(path)})
}

// RegisterPluginDirectory registers dir to be searched, when plugins load,
// for executable files whose names match the registry's pattern. A leading ~/
// is the home directory and environment variables are expanded. A directory
// that does not exist provides no plugins and is not an error.
func (l *Local) RegisterPluginDirectory(dir string) {
	l.add(localEntry{directory: dir})
}

// add records an entry, in registration order
func (l *Local) add(entry localEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)
}

// Plugins returns every registered plugin in registration order, with each
// directory's plugin binaries at the point the directory was registered. Each
// directory search is reported as discover events to emit. It fails when a
// directory cannot be searched.
func (l *Local) Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error) {
	l.mu.Lock()
	entries := append([]localEntry{}, l.entries...)
	l.mu.Unlock()

	var found []Plugin

	for _, entry := range entries {
		if entry.plugin != nil {
			found = append(found, entry.plugin)
			continue
		}

		paths, err := l.discover(emit, entry.directory)
		if err != nil {
			return nil, err
		}

		for _, path := range paths {
			found = append(found, Executable(path))
		}
	}

	return found, nil
}

// discover searches one directory for plugin binaries, reporting the search
// as discover events
func (l *Local) discover(emit events.Emit, dir string) ([]string, error) {
	dirs := expandPluginDirectories([]string{dir})

	emitEvent(emit, events.Event{
		Operation: events.OperationDiscover,
		Phase:     events.PhaseStart,
		Meta:      map[string]any{"dirs": dirs, "registry": LocalName},
	})

	found, err := newPluginDiscovery(dirs, l.pattern, emit).discoverPlugins()
	if err != nil {
		err = fmt.Errorf("plugin discovery failed: %w", err)

		emitEvent(emit, events.Event{
			Operation: events.OperationDiscover,
			Phase:     events.PhaseError,
			Error:     err,
			Meta:      map[string]any{"dirs": dirs, "registry": LocalName},
		})

		return nil, err
	}

	emitEvent(emit, events.Event{
		Operation: events.OperationDiscover,
		Phase:     events.PhaseSuccess,
		Meta:      map[string]any{"dirs": dirs, "registry": LocalName, "count": len(found)},
	})

	return found, nil
}

// emitEvent emits e from xcl itself, a nil emit is silent
func emitEvent(emit events.Emit, e events.Event) {
	if emit == nil {
		return
	}

	e.Source = events.SourceCore
	emit(e)
}
