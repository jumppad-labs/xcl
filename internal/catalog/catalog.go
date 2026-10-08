// Package catalog is the private catalog of block types and the plugin loader
// owned by one Config. It holds the builtin types, the Go types declared for
// the Config, and the registries its plugins come from, and it loads those
// plugins, checks the types they provide and starts and stops their processes
// around each operation.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
)

// Catalog manages all block types (builtin, registered and plugin-based) and
// can create entity instances.
//
// Adding a registry only records it. Its plugins are fetched, started and
// checked by Load, once per catalog, which Config calls, through Use, at the
// start of the first Validate, Apply, Destroy or Load. An external plugin's
// process then runs only while an operation uses it, Use stops it after each
// operation and starts it again for the next. A catalog is safe for
// concurrent use.
type Catalog struct {
	// mu guards typeInfo, hosts and registries
	mu sync.RWMutex

	// typeInfo is the single place the details of every non plugin type live,
	// keyed by types.TypeKey: its type and subtype, whether it is a builtin,
	// and the Go value instances are built from. Plugin provided types are not
	// here because they have no Go type; they are read from their host
	typeInfo map[string]types.TypeInfo
	hosts    []loadedHost

	// registries are the registries plugins are loaded from, in the order
	// they were added
	registries []registry.Registry

	// loadMu makes Load run once, its result is kept in loaded and loadErr
	loadMu  sync.Mutex
	loaded  atomic.Bool
	loadErr error

	// active is the emitter of the operation currently using the catalog,
	// set by Activate, and loading the emitter of the operation running Load.
	// Plugin log messages written outside a provider call go to loading
	// while plugins load, otherwise to active, and are dropped when neither
	// is set.
	active  atomic.Pointer[events.Emit]
	loading atomic.Pointer[events.Emit]

	// usersMu guards users, the number of operations using the plugins, set
	// by Use. External plugin processes run while it is above zero.
	usersMu sync.Mutex
	users   int
}

// loadedHost is the host of a loaded plugin, with the plugin's name and the
// name of the registry it came from
type loadedHost struct {
	host     plugins.PluginHost
	plugin   string
	registry string
}

// restartable is a plugin host whose process Use stops when no operation is
// using it and starts again for the next, an external plugin's host
type restartable interface {
	Restart() error
	Path() string
}

// New creates a new catalog holding the builtin types
func New() *Catalog {
	typeInfo := map[string]types.TypeInfo{}
	for name, prototype := range resources.DefaultResources() {
		typeInfo[name] = types.TypeInfo{Type: name, Builtin: true, Prototype: prototype}
	}

	return &Catalog{
		typeInfo: typeInfo,
	}
}

// RegisterType registers a plain Go type as an entity type, without a plugin
// or provider. Blocks of the type are decoded into new instances of the Go
// type, take part in references and state, and never trigger a provider call.
//
// name is the entity's type followed by an optional subtype. The type is the
// keyword its declarations lead with, and a subtype, when given, is the first
// label:
//
//	c.RegisterType(&Server{}, "server")                 // server "web" {}
//	c.RegisterType(&Server{}, "server", "big")          // server "big" "web" {}
//	c.RegisterType(&Postgres{}, "resource", "postgres") // resource "postgres" "main" {}
//
// prototype must be a pointer to a struct that embeds types.ResourceBase. A
// type keyword takes a subtype for every registration or for none, so an
// address can be read by position; "resource" always takes one.
//
// Registration is a programmer's declaration, so a mistake panics, naming the
// type: a missing or malformed name, a prototype of the wrong kind, a type and
// subtype that a builtin or a registered type already provides, or a type
// keyword in the other form from the one it already has. A type also provided
// by a plugin is accepted here, and the clash is reported when plugins load.
func (c *Catalog) RegisterType(prototype any, name ...string) {
	if err := c.registerType(prototype, name...); err != nil {
		panic(fmt.Sprintf("xcl: %s", err))
	}
}

// registerType is RegisterType returning the problem instead of panicking
// ValidateDeclaration checks the shape of a type declaration on its own,
// without the catalog: the type must be named, take at most one subtype that
// is not empty, and prototype must be a non-nil pointer to a struct that
// embeds types.ResourceBase. The checks that need every declaration, such as
// duplicates, are made when the type is registered.
func ValidateDeclaration(prototype any, name ...string) error {
	if len(name) == 0 || name[0] == "" {
		return fmt.Errorf("an entity type must be named")
	}

	if len(name) > 2 {
		return fmt.Errorf("type %q takes at most one subtype, got %d", name[0], len(name)-1)
	}

	entityType := name[0]

	sub := ""
	if len(name) == 2 {
		sub = name[1]
		if sub == "" {
			return fmt.Errorf("type %q was given an empty subtype, leave it out to register the type without one", entityType)
		}
	}

	key := types.TypeKey(entityType, sub)

	value := reflect.ValueOf(prototype)
	if prototype == nil || value.Kind() != reflect.Ptr || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase", key)
	}

	if _, err := types.GetMeta(prototype); err != nil {
		return fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase: %w", key, err)
	}

	return nil
}

func (c *Catalog) registerType(prototype any, name ...string) error {
	if err := ValidateDeclaration(prototype, name...); err != nil {
		return err
	}

	entityType := name[0]

	sub := ""
	if len(name) == 2 {
		sub = name[1]
	}

	key := types.TypeKey(entityType, sub)

	c.mu.Lock()
	defer c.mu.Unlock()

	if clash := c.checkDeclared(entityType, sub); clash != nil {
		clash.Provider = describeGoType(prototype)
		return clash
	}

	if err := c.checkForm(entityType, sub); err != nil {
		return err
	}

	c.typeInfo[key] = types.TypeInfo{Type: entityType, Subtype: sub, Prototype: prototype}

	return nil
}

// AddRegistry adds r to the registries plugins are loaded from. Registries
// load in the order they were added. A nil r is a programmer error and
// panics.
func (c *Catalog) AddRegistry(r registry.Registry) {
	if r == nil {
		panic("xcl: registry must not be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.registries = append(c.registries, r)
}

// Type returns the details held for the type entityType with subtype, from
// any source this catalog draws on. A plugin provided type is reported with
// a nil Prototype, because it exists host side only as a schema.
func (c *Catalog) Type(entityType, subtype string) (types.TypeInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.typeOf(entityType, subtype)
}

// typeOf is Type with mu held
func (c *Catalog) typeOf(entityType, subtype string) (types.TypeInfo, bool) {
	if info, ok := c.typeInfo[types.TypeKey(entityType, subtype)]; ok {
		return info, true
	}

	for _, loaded := range c.hosts {
		for _, t := range loaded.host.GetTypes() {
			if t.Type == entityType && t.SubType == subtype {
				return types.TypeInfo{Type: t.Type, Subtype: t.SubType}, true
			}
		}
	}

	return types.TypeInfo{}, false
}

// TakesSubtype reports whether declarations of the type keyword entityType
// carry a subtype as their first label, and whether the keyword is known at
// all. "resource" always takes a subtype, whether or not anything has been
// registered under it yet.
func (c *Catalog) TakesSubtype(entityType string) (takes bool, known bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.takesSubtype(entityType)
}

// takesSubtype is TakesSubtype with mu held
func (c *Catalog) takesSubtype(entityType string) (bool, bool) {
	if entityType == types.TypeResource {
		return true, true
	}

	for _, info := range c.typeInfo {
		if info.Type == entityType {
			return info.Subtype != "", true
		}
	}

	for _, loaded := range c.hosts {
		for _, t := range loaded.host.GetTypes() {
			if t.Type == entityType {
				return t.SubType != "", true
			}
		}
	}

	return false, false
}

// Types returns the details of every type this catalog knows, from all of its
// sources.
func (c *Catalog) Types() []types.TypeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	all := make([]types.TypeInfo, 0, len(c.typeInfo))
	for _, info := range c.typeInfo {
		all = append(all, info)
	}

	for _, loaded := range c.hosts {
		for _, t := range loaded.host.GetTypes() {
			if _, ok := c.typeInfo[types.TypeKey(t.Type, t.SubType)]; ok {
				continue
			}

			all = append(all, types.TypeInfo{Type: t.Type, Subtype: t.SubType})
		}
	}

	return all
}

// KnownType returns true when the type entityType with subtype is one this
// catalog knows from any of its sources: a builtin, a type registered with
// RegisterType, or one provided by a loaded plugin.
//
// It is deliberately separate from IsRegisteredType, which answers the much
// narrower question of whether a type came from RegisterType alone. The
// lifecycle depends on that narrow meaning to decide what reaches a provider,
// so widening it would silently change provider routing.
func (c *Catalog) KnownType(entityType, subtype string) bool {
	_, ok := c.Type(entityType, subtype)
	return ok
}

// IsRegisteredType returns true when the type entityType with subtype was
// registered with RegisterType. Builtin and plugin types are not registered
// types.
func (c *Catalog) IsRegisteredType(entityType, subtype string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info, ok := c.typeInfo[types.TypeKey(entityType, subtype)]
	return ok && !info.Builtin
}

// CreateEntity creates a new instance of the type entityType with subtype,
// named name. It tries builtin and registered types, then falls back to plugin
// types. It returns any to accommodate builtin, registered and
// schema-generated entities.
func (c *Catalog) CreateEntity(entityType, subtype, name string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info, ok := c.typeInfo[types.TypeKey(entityType, subtype)]
	if !ok || info.Prototype == nil {
		// no Go type here, so it is a plugin type or nothing at all
		return c.createEntityFromPlugins(entityType, subtype, name)
	}

	entity := reflect.New(reflect.TypeOf(info.Prototype).Elem()).Interface()

	meta, err := types.GetMeta(entity)
	if err != nil {
		return nil, fmt.Errorf("type %q does not embed types.ResourceBase: %w", info.Key(), err)
	}

	meta.Name = name
	meta.Type = info.Type
	meta.Subtype = info.Subtype
	meta.Properties = make(map[string]any)

	return entity, nil
}

// checkDeclared returns a *TypeNameClashError, with Provider left for the
// caller to fill in, when the type entityType with subtype is already
// provided by a builtin or a registered type. mu must be held
func (c *Catalog) checkDeclared(entityType, subtype string) *xclerrors.TypeNameClashError {
	key := types.TypeKey(entityType, subtype)

	// a builtin keyword is xcl's own, with or without a subtype
	if info, ok := c.typeInfo[entityType]; ok && info.Builtin {
		return &xclerrors.TypeNameClashError{Name: key, Existing: "builtin"}
	}

	if info, ok := c.typeInfo[key]; ok {
		existing := describeGoType(info.Prototype)
		if info.Builtin {
			existing = "builtin"
		}

		return &xclerrors.TypeNameClashError{Name: key, Existing: existing}
	}

	return nil
}

// checkForm returns a *TypeFormError when entityType is already used in the
// other form from the one subtype gives it. mu must be held
func (c *Catalog) checkForm(entityType, subtype string) error {
	if takes, known := c.takesSubtype(entityType); known && takes != (subtype != "") {
		return &xclerrors.TypeFormError{Type: entityType, TakesSubtype: takes}
	}

	return nil
}

// checkHostTypes checks every type a new plugin host provides against the
// types already in the catalog, including those of plugins already loaded,
// and against the other types the same host provides, and checks that the
// host can rebuild each one from its schema, which fails for a field of an
// unsupported types.Sensitive instantiation. plugin and registry name the
// plugin the host belongs to. It returns every problem joined together. mu
// must be held.
func (c *Catalog) checkHostTypes(host plugins.PluginHost, plugin, registryName string) error {
	var problems []error
	seen := map[string]bool{}

	for _, t := range host.GetTypes() {
		key := types.TypeKey(t.Type, t.SubType)

		if seen[key] {
			problems = append(problems, &xclerrors.TypeNameClashError{
				Name:             key,
				Provider:         plugin,
				Registry:         registryName,
				Existing:         plugin,
				ExistingRegistry: registryName,
			})
			continue
		}
		seen[key] = true

		if clash := c.checkDeclared(t.Type, t.SubType); clash != nil {
			clash.Provider = plugin
			clash.Registry = registryName
			problems = append(problems, clash)
		} else if clash := c.checkLoaded(t.Type, t.SubType); clash != nil {
			clash.Provider = plugin
			clash.Registry = registryName
			problems = append(problems, clash)
		} else if err := c.checkForm(t.Type, t.SubType); err != nil {
			problems = append(problems, err)
		}

		if _, err := schema.CreateInstanceFromSchema(t.Schema, schema.KnownTypes()); err != nil {
			problems = append(problems, fmt.Errorf("plugin type %s cannot be rebuilt from its schema: %w", key, err))
		}
	}

	return errors.Join(problems...)
}

// checkLoaded returns a *TypeNameClashError, with Provider left for the
// caller to fill in, when the type entityType with subtype is already
// provided by a loaded plugin. mu must be held
func (c *Catalog) checkLoaded(entityType, subtype string) *xclerrors.TypeNameClashError {
	for _, loaded := range c.hosts {
		for _, t := range loaded.host.GetTypes() {
			if t.Type == entityType && t.SubType == subtype {
				return &xclerrors.TypeNameClashError{
					Name:             types.TypeKey(entityType, subtype),
					Existing:         loaded.plugin,
					ExistingRegistry: loaded.registry,
				}
			}
		}
	}

	return nil
}

// describeGoType names a declared type by its Go type, i.e.
// "type *main.Server", as a TypeNameClashError describes it
func describeGoType(prototype any) string {
	return fmt.Sprintf("type %s", reflect.TypeOf(prototype).String())
}

// createEntityFromPlugins attempts to create an entity using loaded plugins,
// mu must be held
func (c *Catalog) createEntityFromPlugins(entityType, subtype, name string) (any, error) {
	// the Go types the host rebuilds plugin types with
	typeMapping := schema.KnownTypes()

	key := types.TypeKey(entityType, subtype)

	for _, loaded := range c.hosts {
		for _, t := range loaded.host.GetTypes() {
			if t.Type == entityType && t.SubType == subtype {
				// Create an instance from the schema
				rawEntity, err := schema.CreateInstanceFromSchema(t.Schema, typeMapping)
				if err != nil {
					return nil, fmt.Errorf("failed to create entity from schema for type %s: %w", key, err)
				}

				meta, err := types.GetMeta(rawEntity)
				if err != nil {
					panic(fmt.Sprintf("entity does not have ResourceBase embedded: %T", rawEntity))
				}

				meta.Name = name
				meta.Type = t.Type
				meta.Subtype = t.SubType
				meta.Properties = make(map[string]any)

				return rawEntity, nil
			}
		}
	}

	// No plugin found for this type
	return nil, fmt.Errorf("type %s not found in any registered plugin", key)
}

// GetProvider finds the provider adapter for a given entity
// Returns nil if the entity is not handled by a plugin
func (c *Catalog) GetProvider(entity any) plugins.ProviderAdapter {
	c.mu.RLock()
	defer c.mu.RUnlock()

	meta, err := types.GetMeta(entity)
	if err != nil {
		return nil // Skip entities without ResourceBase
	}

	return c.pluginAdapter(meta)
}

// pluginAdapter returns the adapter of the plugin type that matches meta's
// type and subtype, or nil when no plugin provides it. mu must be held
func (c *Catalog) pluginAdapter(meta *types.Meta) plugins.ProviderAdapter {
	// the builtins are handled by xcl itself, never by a plugin
	if info, ok := c.typeInfo[meta.Type]; ok && info.Builtin {
		return nil
	}

	for _, loaded := range c.hosts {
		for _, t := range loaded.host.GetTypes() {
			if t.Type == meta.Type && t.SubType == meta.Subtype {
				return t.Adapter
			}
		}
	}

	return nil
}

// Loaded returns true once Load has run, whether or not it succeeded
func (c *Catalog) Loaded() bool {
	return c.loaded.Load()
}

// Activate routes the log messages plugins write outside a provider call,
// such as go-plugin's own messages, to emit, and returns a function that
// stops it. When operations overlap, the one activated last receives them.
func (c *Catalog) Activate(emit events.Emit) (deactivate func()) {
	if emit == nil {
		return func() {}
	}

	current := &emit
	c.active.Store(current)

	return func() {
		c.active.CompareAndSwap(current, nil)
	}
}

// pluginEmit is the emitter plugin hosts are given for the messages plugins
// write outside a provider call, it forwards to the loading operation while
// plugins load, then to the active operation, and drops them when there is
// neither
func (c *Catalog) pluginEmit(e events.Event) {
	if emit := c.loading.Load(); emit != nil {
		(*emit)(e)
		return
	}

	if emit := c.active.Load(); emit != nil {
		(*emit)(e)
	}
}

// Use makes the plugins ready for an operation and returns a function the
// operation calls when it is done. The first Use loads them, as Load does.
// External plugins run as processes only while an operation uses them: Use
// starts any that were stopped, and done stops them once the last operation
// using the catalog is done. Nothing is left running between operations, so
// a program never stops a plugin itself. Config calls Use around every
// Validate, Apply, Destroy and Load.
//
// A plugin that fails to load or start again fails Use with an error matching
// ErrPluginLoad, and done is then nil.
func (c *Catalog) Use(emit events.Emit) (done func(), err error) {
	c.usersMu.Lock()
	defer c.usersMu.Unlock()

	if err := c.Load(emit); err != nil {
		// a load that fails part way may have started some plugins
		if c.users == 0 {
			c.stopPlugins()
		}

		return nil, err
	}

	if c.users == 0 {
		if err := c.restartPlugins(); err != nil {
			c.stopPlugins()
			return nil, err
		}
	}

	c.users++

	var once sync.Once

	return func() {
		once.Do(c.release)
	}, nil
}

// release ends one operation's use of the plugins, stopping the external
// plugin processes when it was the last
func (c *Catalog) release() {
	c.usersMu.Lock()
	defer c.usersMu.Unlock()

	c.users--
	if c.users == 0 {
		c.stopPlugins()
	}
}

// restartPlugins starts every external plugin process that is not running
func (c *Catalog) restartPlugins() error {
	for _, loaded := range c.loadedHosts() {
		external, ok := loaded.host.(restartable)
		if !ok {
			continue
		}

		if err := external.Restart(); err != nil {
			return &xclerrors.PluginLoadError{
				Plugin:   loaded.plugin,
				Registry: loaded.registry,
				Err:      fmt.Errorf("unable to restart plugin binary %s: %w", external.Path(), err),
			}
		}
	}

	return nil
}

// stopPlugins stops every external plugin process, the in-process plugins
// have nothing to stop
func (c *Catalog) stopPlugins() {
	for _, loaded := range c.loadedHosts() {
		if _, ok := loaded.host.(restartable); ok {
			loaded.host.Stop()
		}
	}
}

// Load fetches, starts and checks the plugins of every registry, in the
// order the registries were added and, within a registry, the order it
// returns them. It runs once per catalog, later calls return the first
// call's result, including a failure. What happens is reported to emit:
// whatever a registry reports while providing its plugins, such as discover
// events, then for each plugin a load start, and a load success or error
// event naming the plugin and its registry. A nil emit is silent.
//
// The first problem fails the load: a registry that cannot provide its
// plugins, a plugin that fails to start, both a *PluginLoadError matching
// ErrPluginLoad, or a plugin type that clashes with a known type, a
// *TypeNameClashError or *TypeFormError. Plugins loaded before the failure
// stay in the catalog, so Use can stop them.
func (c *Catalog) Load(emit events.Emit) error {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()

	if c.loaded.Load() {
		return c.loadErr
	}

	if emit != nil {
		c.loading.Store(&emit)
		defer c.loading.Store(nil)
	}

	c.loadErr = c.load(emit)
	c.loaded.Store(true)

	return c.loadErr
}

// load loads the plugins of every registry, it is called once by Load
func (c *Catalog) load(emit events.Emit) error {
	c.mu.RLock()
	registries := append([]registry.Registry{}, c.registries...)
	c.mu.RUnlock()

	for _, r := range registries {
		registryName := r.Name()

		provided, err := r.Plugins(context.Background(), emit)
		if err != nil {
			err = &xclerrors.PluginLoadError{Registry: registryName, Err: err}
			emitEvent(emit, events.Event{
				Operation: events.OperationLoad,
				Phase:     events.PhaseError,
				Error:     err,
				Meta:      map[string]any{"registry": registryName},
			})

			return err
		}

		for _, plugin := range provided {
			name := plugin.Name()
			emitLoad(emit, name, registryName, events.PhaseStart, nil, nil)

			host, err := plugin.Start(c.pluginEmit)
			if err != nil {
				err = &xclerrors.PluginLoadError{Plugin: name, Registry: registryName, Err: err}
				emitLoad(emit, name, registryName, events.PhaseError, err, nil)
				return err
			}

			if err := c.addHost(host, name, registryName); err != nil {
				host.Stop()
				emitLoad(emit, name, registryName, events.PhaseError, err, nil)
				return err
			}

			emitLoad(emit, name, registryName, events.PhaseSuccess, nil, host.GetTypes())
		}
	}

	return nil
}

// addHost adds a loaded plugin's host, rejecting it when any of its types
// clash with a known type name
func (c *Catalog) addHost(host plugins.PluginHost, plugin, registryName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkHostTypes(host, plugin, registryName); err != nil {
		return err
	}

	c.hosts = append(c.hosts, loadedHost{host: host, plugin: plugin, registry: registryName})

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

// emitLoad emits a load event for the plugin called name from the registry
// called registryName, a success names the block types it provides
func emitLoad(emit events.Emit, name, registryName, phase string, err error, registered []plugins.RegisteredType) {
	meta := map[string]any{"plugin": name, "registry": registryName}
	if phase == events.PhaseSuccess {
		meta["block_types"] = plugins.ResourceTypeNames(registered)
	}

	emitEvent(emit, events.Event{Operation: events.OperationLoad, Phase: phase, Error: err, Meta: meta})
}

// GetPluginHosts returns the hosts of the plugins that have loaded, it is
// empty until Load has run
func (c *Catalog) GetPluginHosts() []plugins.PluginHost {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hosts := make([]plugins.PluginHost, 0, len(c.hosts))
	for _, loaded := range c.hosts {
		hosts = append(hosts, loaded.host)
	}

	return hosts
}

// loadedHosts returns a copy of the loaded hosts with their plugin and
// registry names
func (c *Catalog) loadedHosts() []loadedHost {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return append([]loadedHost{}, c.hosts...)
}

// GetProviderForResource gets the provider for any entity that embeds ResourceBase
func (c *Catalog) GetProviderForResource(entity any) plugins.ProviderAdapter {
	c.mu.RLock()
	defer c.mu.RUnlock()

	meta, err := types.GetMeta(entity)
	if err != nil {
		panic(fmt.Sprintf("entity does not have ResourceBase embedded: %T", entity))
	}

	return c.pluginAdapter(meta)
}

// TypePath returns the address segments a registered Go type is reached by:
// its type and subtype, i.e. {"resource", "container"} or {"server", "big"},
// or its type alone, i.e. {"cache"}, when it was registered without a subtype.
//
// It reports false for a type it cannot reach. A plugin provides a schema and
// no Go type, so a plugin backed type has nothing to reflect against and is
// always reported this way; the caller should fall back to naming the kind.
func (c *Catalog) TypePath(t reflect.Type) ([]string, bool) {
	if t == nil {
		return nil, false
	}

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, info := range c.typeInfo {
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
