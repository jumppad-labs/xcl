package registry

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// PluginRegistry manages all resource types (builtin, registered and plugin-based) and can create resource instances.
//
// Registering a plugin, a plugin path or a discovery directory only records
// it. Plugins are discovered, started and checked by Load, once per registry,
// which Config calls at the start of the first Validate, Apply or Destroy. A
// registry may be shared by several Configs and is safe for concurrent use.
type PluginRegistry struct {
	// mu guards typeInfo, pluginHosts and the recorded plugins
	mu sync.RWMutex

	// typeInfo is the single place the details of every non plugin type live:
	// its name, its declaration form, whether it is a builtin, and the Go value
	// instances are built from. Plugin provided types are not here because they
	// have no Go type; they are read from their host
	typeInfo    map[string]types.TypeInfo
	pluginHosts []plugins.PluginHost

	// recorded by the Register and Discover methods, loaded by Load
	pending          []plugins.Plugin
	pendingPaths     []string
	discoveryDirs    []string
	discoveryPattern string

	// loadMu makes Load run once, its result is kept in loaded and loadErr
	loadMu  sync.Mutex
	loaded  atomic.Bool
	loadErr error

	// active is the emitter of the operation currently using the registry,
	// set by Activate, and loading the emitter of the operation running Load.
	// Plugin log messages written outside a provider call go to loading
	// while plugins load, otherwise to active, and are dropped when neither
	// is set.
	active  atomic.Pointer[events.Emit]
	loading atomic.Pointer[events.Emit]
}

// NewPluginRegistry creates a new plugin registry with builtin types
func NewPluginRegistry() *PluginRegistry {
	typeInfo := map[string]types.TypeInfo{}
	for name, prototype := range resources.DefaultResources() {
		typeInfo[name] = types.TypeInfo{Name: name, Builtin: true, Prototype: prototype}
	}

	return &PluginRegistry{
		typeInfo:    typeInfo,
		pluginHosts: []plugins.PluginHost{},
	}
}

// RegisterType registers a plain Go type as a configuration block type,
// without a plugin or provider. Blocks of the type are decoded into new
// instances of the Go type, take part in references and state, and never
// trigger a provider call.
//
// resource must be a pointer to a struct that embeds types.ResourceBase.
// Registering a name that is already provided by a builtin, a registered type
// or a loaded plugin returns a *TypeNameClashError, and leaves the registry
// unchanged. A name also provided by a plugin that has not loaded yet is
// accepted here, and the clash is reported when plugins load.
func (r *PluginRegistry) RegisterType(name string, resource any) error {
	return r.registerType(name, resource, false)
}

// registerType registers resource under name, bare says it is declared by its
// own keyword
func (r *PluginRegistry) registerType(name string, resource any, bare bool) error {
	value := reflect.ValueOf(resource)
	if resource == nil || value.Kind() != reflect.Ptr || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase", name)
	}

	if _, err := types.GetMeta(resource); err != nil {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase: %w", name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.checkTypeName(name); err != nil {
		return err
	}

	r.typeInfo[name] = types.TypeInfo{Name: name, Prototype: resource, Bare: bare}

	return nil
}

// RegisterBareType registers resource under name in the bare declaration form,
// declared by its own keyword with a single label, i.e. container "nics".
// A type is registered under one form or the other and never both, so a type
// registered here is addressed <name>.<resource> rather than
// resource.<name>.<resource>.
//
// It shares RegisterType's validation and clash rules.
func (r *PluginRegistry) RegisterBareType(name string, resource any) error {
	return r.registerType(name, resource, true)
}

// Type returns the details held for name, from any source this registry draws
// on. A plugin provided type is reported with a nil Prototype, because it
// exists host side only as a schema.
func (r *PluginRegistry) Type(name string) (types.TypeInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.typeInfo[name]; ok {
		return info, true
	}

	for _, host := range r.pluginHosts {
		for _, t := range host.GetTypes() {
			if t.Type == types.TypeResource && t.SubType == name {
				return types.TypeInfo{Name: name}, true
			}
		}
	}

	return types.TypeInfo{}, false
}

// Types returns the details of every type this registry knows, from all of its
// sources.
func (r *PluginRegistry) Types() []types.TypeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]types.TypeInfo, 0, len(r.typeInfo))
	for _, info := range r.typeInfo {
		all = append(all, info)
	}

	for _, host := range r.pluginHosts {
		for _, t := range host.GetTypes() {
			if t.Type != types.TypeResource {
				continue
			}

			if _, ok := r.typeInfo[t.SubType]; ok {
				continue
			}

			all = append(all, types.TypeInfo{Name: t.SubType})
		}
	}

	return all
}

// KnownType returns true when name is a type this registry knows from any of
// its sources: a builtin, a type registered with RegisterType or
// RegisterBareType, or one provided by a loaded plugin.
//
// It is deliberately separate from IsRegisteredType, which answers the much
// narrower question of whether a name came from RegisterType alone. The
// lifecycle depends on that narrow meaning to decide what reaches a provider,
// so widening it would silently change provider routing.
func (r *PluginRegistry) KnownType(name string) bool {
	_, ok := r.Type(name)
	return ok
}

// IsRegisteredType returns true when name was registered with RegisterType.
// Builtin and plugin types are not registered types.
func (r *PluginRegistry) IsRegisteredType(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.typeInfo[name]
	return ok && !info.Builtin
}

// CreateResource creates a new resource instance of the specified type and name
// It tries builtin types, then registered types, then falls back to plugin types
// Returns any to accommodate builtin, registered and schema-generated resources
func (r *PluginRegistry) CreateResource(resourceType, resourceName string) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.typeInfo[resourceType]
	if !ok || info.Prototype == nil {
		// no Go type here, so it is a plugin type or nothing at all
		return r.createResourceFromPlugins(resourceType, resourceName)
	}

	resource := reflect.New(reflect.TypeOf(info.Prototype).Elem()).Interface()

	meta, err := types.GetMeta(resource)
	if err != nil {
		return nil, fmt.Errorf("type %q does not embed types.ResourceBase: %w", resourceType, err)
	}

	// the declaration form decides the axes: a bare or builtin type is its own
	// kind and carries no variety, a kind led one is a resource of that variety
	meta.Name = resourceName
	meta.Type = types.TypeResource
	meta.Subtype = info.Name
	if info.Bare || info.Builtin {
		meta.Type = info.Name
		meta.Subtype = ""
	}
	meta.Properties = make(map[string]any)

	return resource, nil
}

// checkTypeName returns a *TypeNameClashError when name is already provided
// by a builtin, a registered type or a loaded plugin, mu must be held
func (r *PluginRegistry) checkTypeName(name string) error {
	if info, ok := r.typeInfo[name]; ok {
		existing := "registered type"
		if info.Builtin {
			existing = "builtin"
		}

		return &TypeNameClashError{Name: name, Existing: existing}
	}

	for _, host := range r.pluginHosts {
		for _, t := range host.GetTypes() {
			if t.Type == types.TypeResource && t.SubType == name {
				return &TypeNameClashError{Name: name, Existing: "plugin"}
			}
		}
	}

	return nil
}

// checkHostTypes checks every resource type a new plugin host provides
// against the names already in the registry, and against the other types the
// same host provides. It returns every clash joined together. mu must be
// held.
func (r *PluginRegistry) checkHostTypes(host plugins.PluginHost) error {
	var clashes []error
	seen := map[string]bool{}

	for _, t := range host.GetTypes() {
		if t.Type != "resource" {
			continue
		}

		if seen[t.SubType] {
			clashes = append(clashes, &TypeNameClashError{Name: t.SubType, Existing: "the same plugin"})
			continue
		}
		seen[t.SubType] = true

		if err := r.checkTypeName(t.SubType); err != nil {
			clashes = append(clashes, err)
		}
	}

	return errors.Join(clashes...)
}

// createResourceFromPlugins attempts to create a resource using registered
// plugins, mu must be held
func (r *PluginRegistry) createResourceFromPlugins(resourceType, resourceName string) (any, error) {
	// Create type mapping for proper type creation
	typeMapping := map[string]reflect.Type{
		"types.Meta":         reflect.TypeOf(types.Meta{}),
		"types.ResourceBase": reflect.TypeOf(types.ResourceBase{}),
		"cty.Value":          reflect.TypeOf(cty.Value{}), // Treat cty.Value as interface{}
	}

	// Iterate through all plugin hosts
	for _, host := range r.pluginHosts {
		pluginTypes := host.GetTypes()

		// Look for a matching type
		for _, t := range pluginTypes {
			if t.Type == "resource" && t.SubType == resourceType {
				// Create a resource instance from the schema
				rawResource, err := schema.CreateInstanceFromSchema(t.Schema, typeMapping)
				if err != nil {
					return nil, fmt.Errorf("failed to create resource from schema for type %s: %w", resourceType, err)
				}

				meta, err := types.GetMeta(rawResource)
				if err != nil {
					panic(fmt.Sprintf("resource does not have ResourceBase embedded: %T", rawResource))
				}

				// the plugin wire already carries the two axes separately,
				// so take both from the registered type rather than folding
				// the variety into the kind
				meta.Name = resourceName
				meta.Type = t.Type
				meta.Subtype = t.SubType
				meta.Properties = make(map[string]any)

				return rawResource, nil
			}
		}
	}

	// No plugin found for this resource type
	return nil, fmt.Errorf("resource type %s not found in any registered plugin", resourceType)
}

// GetProvider finds the provider adapter for a given resource
// Returns nil if the resource type is a builtin type (not handled by plugins)
func (r *PluginRegistry) GetProvider(resource any) plugins.ProviderAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, err := types.GetMeta(resource)
	if err != nil {
		return nil // Skip resources without ResourceBase
	}
	// Only resource kind entities reach a plugin, the single label builtins
	// are handled by xcl itself
	if meta.Type != types.TypeResource {
		return nil
	}

	resourceType := meta.Subtype

	// Search through plugin hosts
	for _, host := range r.pluginHosts {
		pluginTypes := host.GetTypes()

		for _, t := range pluginTypes {
			if t.Type == "resource" && t.SubType == resourceType {
				// Found the matching plugin type
				return t.Adapter
			}
		}
	}

	// No plugin found for this resource type
	return nil
}

// RegisterPlugin records an in-process plugin, it is initialised when plugins
// load, see Load
func (r *PluginRegistry) RegisterPlugin(plugin plugins.Plugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.pending = append(r.pending, plugin)

	return nil
}

// RegisterPluginWithPath records an external plugin binary, it is started
// when plugins load, see Load. A path that does not exist is not an error
// here, the first Validate, Apply or Destroy fails with an ErrPluginLoad
// naming it.
func (r *PluginRegistry) RegisterPluginWithPath(pluginPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pendingPaths = append(r.pendingPaths, pluginPath)

	return nil
}

// DiscoverPlugins records directories to search for plugin binaries whose
// file names match pattern, "xcl-plugin-*" when empty. They are searched and
// the plugins found are started when plugins load, see Load. A later call
// adds its directories and replaces the pattern.
func (r *PluginRegistry) DiscoverPlugins(directories []string, pattern string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.discoveryDirs = append(r.discoveryDirs, directories...)
	r.discoveryPattern = pattern
}

// Loaded returns true once Load has run, whether or not it succeeded
func (r *PluginRegistry) Loaded() bool {
	return r.loaded.Load()
}

// Activate routes the log messages plugins write outside a provider call,
// such as go-plugin's own messages, to emit, and returns a function that
// stops it. When operations overlap, the one activated last receives them.
func (r *PluginRegistry) Activate(emit events.Emit) (deactivate func()) {
	if emit == nil {
		return func() {}
	}

	current := &emit
	r.active.Store(current)

	return func() {
		r.active.CompareAndSwap(current, nil)
	}
}

// pluginEmit is the emitter plugin hosts are given for the messages plugins
// write outside a provider call, it forwards to the loading operation while
// plugins load, then to the active operation, and drops them when there is
// neither
func (r *PluginRegistry) pluginEmit(e events.Event) {
	if emit := r.loading.Load(); emit != nil {
		(*emit)(e)
		return
	}

	if emit := r.active.Load(); emit != nil {
		(*emit)(e)
	}
}

// Load discovers, starts and checks every recorded plugin. It runs once per
// registry however many Configs share it, later calls return the first
// call's result, including a failure. What happens is reported to emit:
// discover events for the directory search, then for each plugin a load
// start, and a load success or error event. A nil emit is silent.
//
// A registered plugin that fails to start, or any plugin type whose name
// clashes with a known type, fails the load. The error for a plugin that
// fails to start is a *PluginLoadError, matching ErrPluginLoad, naming it. A
// discovered plugin that fails to start is rejected, reported with a load
// error event and skipped, unless every discovered plugin fails.
func (r *PluginRegistry) Load(emit events.Emit) error {
	r.loadMu.Lock()
	defer r.loadMu.Unlock()

	if r.loaded.Load() {
		return r.loadErr
	}

	if emit != nil {
		r.loading.Store(&emit)
		defer r.loading.Store(nil)
	}

	r.loadErr = r.load(emit)
	r.loaded.Store(true)

	return r.loadErr
}

// load loads every recorded plugin, it is called once by Load
func (r *PluginRegistry) load(emit events.Emit) error {
	r.mu.RLock()
	pending := append([]plugins.Plugin{}, r.pending...)
	paths := append([]string{}, r.pendingPaths...)
	dirs := append([]string{}, r.discoveryDirs...)
	pattern := r.discoveryPattern
	r.mu.RUnlock()

	discovered, err := r.discover(emit, dirs, pattern)
	if err != nil {
		return err
	}

	for _, plugin := range pending {
		name := plugins.PluginName(plugin)
		emitLoad(emit, name, events.PhaseStart, nil, nil)

		host, err := plugins.NewDirectPluginHost(r.pluginEmit, nil, plugin)
		if err != nil {
			err = &xclerrors.PluginLoadError{Plugin: name, Err: err}
			emitLoad(emit, name, events.PhaseError, err, nil)
			return err
		}

		if err := r.addHost(host); err != nil {
			emitLoad(emit, name, events.PhaseError, err, nil)
			return err
		}

		emitLoad(emit, name, events.PhaseSuccess, nil, host.GetTypes())
	}

	for _, path := range paths {
		name := plugins.PluginBinaryName(path)
		emitLoad(emit, name, events.PhaseStart, nil, nil)

		host, err := r.startExternal(path)
		if err != nil {
			err = &xclerrors.PluginLoadError{Plugin: path, Err: err}
			emitLoad(emit, name, events.PhaseError, err, nil)
			return err
		}

		if err := r.addHost(host); err != nil {
			host.Stop()
			err = fmt.Errorf("plugin %s: %w", path, err)
			emitLoad(emit, name, events.PhaseError, err, nil)
			return err
		}

		emitLoad(emit, name, events.PhaseSuccess, nil, host.GetTypes())
	}

	return r.loadDiscovered(emit, discovered)
}

// discover searches dirs for plugin binaries, reporting the search as
// discover events
func (r *PluginRegistry) discover(emit events.Emit, dirs []string, pattern string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	emitEvent(emit, events.Event{Operation: events.OperationDiscover, Phase: events.PhaseStart, Meta: map[string]any{"dirs": dirs}})

	found, err := NewPluginDiscovery(dirs, pattern, emit).DiscoverPlugins()
	if err != nil {
		err = fmt.Errorf("plugin discovery failed: %w", err)
		emitEvent(emit, events.Event{Operation: events.OperationDiscover, Phase: events.PhaseError, Error: err, Meta: map[string]any{"dirs": dirs}})
		return nil, err
	}

	emitEvent(emit, events.Event{Operation: events.OperationDiscover, Phase: events.PhaseSuccess, Meta: map[string]any{"dirs": dirs, "count": len(found)}})

	return found, nil
}

// loadDiscovered starts the discovered plugin binaries. One that fails to
// start is rejected and skipped, the load fails only when every one fails, or
// when any provides a type whose name clashes.
func (r *PluginRegistry) loadDiscovered(emit events.Emit, discovered []string) error {
	var loadErrors []string
	var clashErrors []error
	successCount := 0

	for _, path := range discovered {
		name := plugins.PluginBinaryName(path)
		emitLoad(emit, name, events.PhaseStart, nil, nil)

		host, err := r.startExternal(path)
		if err != nil {
			err = fmt.Errorf("failed to start external plugin %s: %w", path, err)
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", path, err))
			emitRejected(emit, name, path, err)
			continue
		}

		if err := r.addHost(host); err != nil {
			host.Stop()
			err = fmt.Errorf("plugin %s: %w", path, err)
			clashErrors = append(clashErrors, err)
			emitRejected(emit, name, path, err)
			continue
		}

		successCount++
		emitLoad(emit, name, events.PhaseSuccess, nil, host.GetTypes())
	}

	// Type name clashes always fail the load, even when other plugins loaded
	if len(clashErrors) > 0 {
		return errors.Join(clashErrors...)
	}

	// Only fail when every discovered plugin failed to load
	if len(loadErrors) > 0 && successCount == 0 {
		return fmt.Errorf("all plugin loads failed: %s", strings.Join(loadErrors, "; "))
	}

	return nil
}

// startExternal starts the external plugin binary at path
func (r *PluginRegistry) startExternal(path string) (*plugins.GRPCPluginHost, error) {
	host := plugins.NewGRPCPluginHost(r.pluginEmit, nil)

	if err := host.Start(path); err != nil {
		return nil, err
	}

	return host, nil
}

// addHost adds a loaded plugin's host, rejecting it when any of its types
// clash with a known type name
func (r *PluginRegistry) addHost(host plugins.PluginHost) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.checkHostTypes(host); err != nil {
		return err
	}

	r.pluginHosts = append(r.pluginHosts, host)

	return nil
}

// emitEvent emits e from xcl itself, a nil emit is silent
func emitEvent(emit events.Emit, e events.Event) {
	if emit == nil {
		return
	}

	e.Source = events.SourceCore
	emit(e)
}

// emitLoad emits a load event for the plugin called name, a success names the
// block types it provides
func emitLoad(emit events.Emit, name, phase string, err error, registered []plugins.RegisteredType) {
	meta := map[string]any{"plugin": name}
	if phase == events.PhaseSuccess {
		meta["block_types"] = plugins.ResourceTypeNames(registered)
	}

	emitEvent(emit, events.Event{Operation: events.OperationLoad, Phase: phase, Error: err, Meta: meta})
}

// emitRejected emits a load error for a discovered plugin that was skipped
func emitRejected(emit events.Emit, name, path string, err error) {
	emitEvent(emit, events.Event{
		Operation: events.OperationLoad,
		Phase:     events.PhaseError,
		Error:     err,
		Meta:      map[string]any{"plugin": name, "path": path, "rejected": true},
	})
}

// GetPluginHosts returns the hosts of the plugins that have loaded, it is
// empty until Load has run
func (r *PluginRegistry) GetPluginHosts() []plugins.PluginHost {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return append([]plugins.PluginHost{}, r.pluginHosts...)
}

// TODO: Rebuild CastResource without GenericResource

// CastResourceTo attempts to cast a resource to a specific concrete type
// This is a type-safe way to get strongly-typed resources from the registry
func CastResourceTo[T any](registry *PluginRegistry, resource any) (T, error) {
	var zero T

	// Try direct type assertion
	if concrete, ok := resource.(T); ok {
		return concrete, nil
	}

	return zero, fmt.Errorf("resource cannot be cast to requested type")
}

// GetProviderForResource gets the provider for any resource that embeds ResourceBase
func (r *PluginRegistry) GetProviderForResource(resource any) plugins.ProviderAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, err := types.GetMeta(resource)
	if err != nil {
		panic(fmt.Sprintf("resource does not have ResourceBase embedded: %T", resource))
	}

	// plugins are registered against the variety, not the stanza kind
	resourceType := meta.Subtype

	// Search through plugin hosts
	for _, host := range r.pluginHosts {
		pluginTypes := host.GetTypes()

		for _, t := range pluginTypes {
			if t.Type == "resource" && t.SubType == resourceType {
				// Found the matching plugin type
				return t.Adapter
			}
		}
	}

	return nil
}

// TypePath returns the address segments a registered Go type is reached by:
// {"resource", "container"} for a type declared with the resource keyword, and
// {"container"} for one declared by its own keyword. A type is registered
// under one form or the other, so it is never both.
//
// It reports false for a type it cannot reach. A plugin provides a schema and
// no Go type, so a plugin backed type has nothing to reflect against and is
// always reported this way; the caller should fall back to naming the kind.
func (r *PluginRegistry) TypePath(t reflect.Type) ([]string, bool) {
	if t == nil {
		return nil, false
	}

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, info := range r.typeInfo {
		if info.Prototype == nil {
			continue
		}

		pt := reflect.TypeOf(info.Prototype)
		for pt.Kind() == reflect.Ptr {
			pt = pt.Elem()
		}

		if pt == t {
			return info.AddressPath(), true
		}
	}

	return nil, false
}
