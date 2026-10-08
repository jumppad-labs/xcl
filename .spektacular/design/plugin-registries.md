---
created_date: "2026-10-08"
document_status: draft
spec: 20261008071608-eb05cae0-config-and-plugin-registries
specs:
    - 20261008071608-eb05cae0-config-and-plugin-registries
---

# Config and plugin registry API

## Goal

Make the common xcl programs short and obvious:
- **Config only:** read `.xcl` files into your own Go types.
- **Plugins:** apply configuration through plugins.

Today both need a hand-built `PluginRegistry`, an error check after every registration call, and state setup even when nothing should be persisted. This design removes all three. It makes **registries** the one way a `Config` gets anything beyond the builtins: plain Go types and plugins alike. That way, types and plugins can come from more than one place: from disk (local), or fetched from a server (remote).

The API may break. The aim is the nicest syntax, not compatibility.

## What the result looks like

Config only, with no plugins and no state:

```go
local := registry.NewLocal()
local.RegisterType(&resources.Deployment{}, "deployment")
local.RegisterType(&resources.Service{}, "service")
local.RegisterType(&resources.Ingress{}, "ingress")

c, err := xcl.NewConfig(xcl.WithRegistry(local))
if err != nil {
    return err
}

if err := c.Apply("./config"); err != nil {
    return err
}
```

With plugins from a local registry and a remote registry:

```go
local := registry.NewLocal()
local.RegisterPlugin(&template.TemplatePlugin{})
local.RegisterExternalPlugin("./bin/xcl-plugin-docker")
local.RegisterPluginDirectory("~/.xcl/plugins")

remote := registry.NewRemote("https://registry.xcl.dev")
remote.RegisterPlugin("postgres", "1.2.3")

c, err := xcl.NewConfig(
    xcl.WithRegistry(local),
    xcl.WithRegistry(remote),
    xcl.WithStatePath("./.xcl"),
    xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv())),
    xcl.WithEventData(xcl.EventDataProcessed),
)
```

There is one way in, `xcl.WithRegistry`, and one place things are registered: the registry. That way it's always clear where a type or plugin comes from.

## Registration never returns an error

Registering something only records it. Every `Register*` method returns nothing. Problems are reported in one of three places, depending on when they can first be seen:

- **Mistakes in a single call panic at that call.** These fail on every run until the code is fixed:
  - an empty type name, more than one subtype, or an empty subtype
  - a type that isn't a pointer to a struct embedding `types.ResourceBase`
  - a nil in-process plugin or a nil registry
  - an empty remote name or version

  This matches `gob.Register` and `http.ServeMux.Handle`, and the stack trace points at the bad line.
- **Type clashes are returned by `NewConfig`.** Declared Go types are known without loading anything, so `NewConfig` reads every registry's types and returns an error when they don't fit together:
  - the same block type declared twice, in one registry or across two
  - a type keyword used both with and without a subtype
  - a builtin name

  A clash between registries comes from how they are combined, not from one bad line, so it is an error rather than a panic.
- **Environment errors are returned when plugins load.** These depend on the machine or network: a binary that is missing or won't start, a remote plugin that can't be fetched, or a block type a plugin provides that something else already provides. Plugins load once per `Config`, on the first `Validate`, `Apply`, `Destroy`, `Diff` or `Load`.
  - **Any** plugin that fails to start fails the load, including one found by directory discovery. The error is a `*PluginLoadError` naming the plugin and its registry.
  - A clash involving a plugin is a `*TypeNameClashError` naming both providers and their registries.

Whatever order registration and loading happen in, each problem is reported in the same place.

## Config options

| Option | Purpose |
|---|---|
| `WithRegistry(r registry.Registry)` | Adds a registry. Can be given any number of times. Registries are read, and their plugins load, in the order given. |
| `WithStatePath`, `WithStateStore`, `WithStateMask` | Unchanged. Without either of the first two, nothing is persisted. This is the supported mode for config-only use, and the docs say so. |
| `WithEventHandler`, `WithEventData`, `WithEventMask`, ... | Unchanged. |

`WithPluginRegistry` is removed, and there is no `WithType`. Options can be given in any order: `NewConfig` collects them all, then builds its type catalog from the registries.

## Registries

A registry provides plain Go types, plugins, or both:

```go
package registry

// Registry provides types and plugins to a Config. A Config uses every
// registry given to it with xcl.WithRegistry, in that order.
type Registry interface {
    // Name identifies the registry in events and errors, i.e. "local" or
    // "registry.xcl.dev"
    Name() string

    // Types returns the plain Go types this registry declares. It is read
    // once, by NewConfig. A registry with no Go types returns nil.
    Types() []Type

    // Plugins returns the plugins this registry provides, fetching them first
    // if it needs to. It is called once per Config, when plugins load.
    Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error)
}

// Type is a plain Go type declared as a block type. It has no provider: it
// is only decoded into.
type Type struct {
    Type      string // the block type, i.e. "deployment" or "resource"
    Subtype   string // the subtype, empty for a type declared without one
    Prototype any    // pointer to a struct embedding types.ResourceBase
}

// Plugin is one plugin a registry provides, ready to start
type Plugin interface {
    // Name identifies the plugin in events and errors
    Name() string

    // Start runs the plugin and returns the host xcl talks to it through
    Start(emit events.Emit) (plugins.PluginHost, error)
}
```

xcl provides the `Plugin` implementations a registry needs:
- `registry.InProcess(p plugins.Plugin)`: a plugin compiled into the program, started with today's direct host.
- `registry.Executable(path string)`: a plugin binary, started as a process over gRPC with today's gRPC host.

Because a plugin starts itself, a later `registry.Connect(address)` can reach a plugin **already running somewhere else** without any change to the core or to `Registry`. Running remote plugins isn't built now, but the interface is shaped so it can be added. Third parties can write registries of their own against the same interface.

### Local registry

```go
local := registry.NewLocal()                      // Name() == "local"
local.RegisterType(&resources.Deployment{}, "deployment")
local.RegisterType(&Database{}, "resource", "database")
local.RegisterPlugin(&template.TemplatePlugin{})  // in-process
local.RegisterExternalPlugin("./bin/xcl-plugin-docker")
local.RegisterPluginDirectory("./plugins")        // every file matching the pattern

local := registry.NewLocal(registry.PluginPattern("acme-plugin-*"))  // default "xcl-plugin-*"
```

- `RegisterType(prototype, name...)` declares a plain Go type. `name` is the block type and an optional subtype. A malformed declaration panics at the call; a clash with another declaration is returned by `NewConfig`.
- `RegisterExternalPlugin` replaces `RegisterPluginWithPath`.
- `RegisterPluginDirectory` replaces `DiscoverPlugins`. The search pattern is set once, when the registry is created, so a later call can't change what an earlier one matches.
- Plugins load in the order they were registered.
- Any plugin that fails to start fails the load, whether it was registered explicitly or found in a directory.

### Remote registry

```go
remote := registry.NewRemote("https://registry.xcl.dev")  // Name() == "registry.xcl.dev"
remote.RegisterPlugin("postgres", "1.2.3")
remote.RegisterPlugin("docker", "0.9.0")
```

- A plugin is named by its name and an exact version. The plugin itself defines which types and subtypes it provides, so registration doesn't name types. Its `Types()` returns nil.
- When plugins load, the remote registry fetches each registered plugin's binary, caches it, and returns an `Executable` for the cached file. From then on it's started exactly like a local binary.
- Its `RegisterPlugin` uses the same verb as the local one, which takes a `plugins.Plugin`. Only the arguments differ, because what identifies a plugin differs.

**The remote registry is designed here so the UX is settled, but it isn't built now.** Its protocol, cache location, version ranges and verification of downloads are left until it is.

## Clashes

A block type provided twice is an error, wherever the two come from. There is no precedence between registries: their order only decides the order types are read, plugins load and events are emitted.

- **Two declared Go types**, in one registry or across two, are a `*TypeNameClashError` returned by `NewConfig`, naming both Go types and their registries. A declared type whose keyword is used in the other form is a `*TypeFormError`, and a declared type with a builtin name is a `*TypeNameClashError` naming `builtin`.
- **A plugin and a declared type, or two plugins**, are a `*TypeNameClashError` returned by the first operation that loads plugins, naming both providers and their registries.

## Events without a registry

`Event` gains a method that returns the Go value the event's `Data` holds, decoded through the emitting `Config`'s type catalog:

```go
// Entity returns the entity the event's Data holds, as its registered Go type,
// with sensitive values shown as the mask marker. It returns nil when the
// event carries no data.
func (e Event) Entity() (any, error)
```

The `Config` attaches the decoder in one place, its `run` wrapper, so no emitter changes. The value is decoded on demand from `Data`, so a buffered handler gets its own copy and never a live entity, and a handler that doesn't call it pays nothing. The decoder field on `events.Event` is excluded from JSON.

`prettylog.Handler(w, level)` loses its registry argument and encodes `e.Entity()` with `xcl.EncodeEntity`.

## What becomes internal

`plugins/registry.PluginRegistry` is the type catalog plus the plugin loader. It moves to `internal/catalog`, and each `Config` owns its own. Nothing in the public API takes it. The public functions that do today change as follows:

| Today | After |
|---|---|
| `WithPluginRegistry(r)` | Removed, use `WithRegistry` |
| `PluginRegistry.RegisterType(...)` | `registry.Local.RegisterType(...)` |
| `EncodeSavedEntity(reg, data, ...)` | `(*Config).EncodeSavedEntity(data, ...)`, for tools that read saved data with a `Config` that knows the types |
| `prettylog.Handler(w, level, reg)` | `prettylog.Handler(w, level)` |
| `registry.CastResourceTo[T](reg, entity)` | Dropped along with the catalog |

The new public package `github.com/jumppad-labs/xcl/registry` holds `Registry`, `Type`, `Plugin`, `InProcess`, `Executable`, `NewLocal`, `NewRemote` and their options. It sits at the top level of the module, under a name that no longer collides with the old package.

## Examples

Both examples are rewritten to the new API:
- `example/configonly` declares its types on one local registry and keeps no state.
- `example/plugin` uses one local registry for its plugins.

Neither builds a catalog, checks a registration error or passes a registry to `prettylog`.

## Open

- **The remote registry's protocol, cache, version ranges and download verification.** All deferred until the remote registry is built.
- **Fetching only what's used.** A remote plugin could be fetched only when the configuration uses one of its types. That needs the registry to publish an index of the types each plugin provides. It can be added without changing `RegisterPlugin`.
- **Connecting to running plugins** (`registry.Connect`). Wanted later, not built now.
