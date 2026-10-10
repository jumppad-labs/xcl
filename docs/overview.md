# Overview

xclconfig parses HCL configuration describing entities, resolves them into a
dependency graph, and — for each entity — calls out to a *provider* (a
plugin) to actually create/update/destroy whatever the entity represents.
It is architecturally close to Terraform: HCL in, provider RPCs out, state
persisted in between runs.

## The three-layer split

```
Config            (repo root, package xcl)
  owns: Catalog, StateStore, the entities currently declared
  entry point: NewConfig(opts...) (*Config, error), then Apply()/Validate()/Destroy()

Parser            (internal/parser)
  does one Apply() call: load previous state -> parse HCL -> destroy
  removed entities -> build DAG -> walk DAG, decoding each entity and
  calling its provider
  or one Destroy() call: walk the saved state children first, calling
  each entity's provider

Catalog           (internal/catalog)
  built by NewConfig from the registries added with WithRegistry,
  reading the Go types each declares; loads the registries' plugins on
  the first operation, aggregates PluginHosts, answers "what Go type is
  entity X" and "what ProviderAdapter handles entity X"
```

`Config` is the only piece meant to be constructed directly by a library
user. `Parser` is constructed fresh, internally, on every
`Apply`/`Validate`/`Destroy` call — it is not held onto between calls.
The `Catalog` and the `StateStore` are the two pieces of long-lived state
`Config` owns and passes into each new `Parser` via `ParserOptions`. The
catalog is private; applications reach it only through `WithRegistry`. The
public [`registry`](../registry) package defines where Go types and plugins
come from: the `Registry` (`Name()`, `Types()`, `Plugins(ctx, emit)`), `Type`
and `Plugin` types, and the local registry, `registry.NewLocal()`.

## Entry point

```go
local := registry.NewLocal()
local.RegisterType(&Server{}, "server")   // a plain Go type, no plugin
local.RegisterPlugin(&MyPlugin{})         // an in-process plugin

cfg, err := xcl.NewConfig(
    xcl.WithRegistry(local),              // Go types and plugins
    xcl.WithStateStore(stateStore),       // or xcl.WithStatePath("./.xcl"), or neither to persist nothing
    xcl.WithVariables(vars),
)

err := cfg.Validate("./infra")         // checks only, acts on nothing
err := cfg.Apply("./infra")            // parses + executes provider lifecycle
err := cfg.Destroy()                   // destroys everything in the saved state
```

[`config.go:27`](../config.go#L27) `NewConfig` applies functional options
([`options.go`](../options.go)) onto a `Config` holding no entities yet. An
option can fail, `WithStatePath` does when it cannot create the state
directory, and the first failure is returned from `NewConfig`.
Registration returns no error. A mistake in a single call, such as an empty
type name or a nil registry, panics. Declared Go types that clash, the same
block type declared twice in one registry or across two, or a builtin name,
are a `*xcl.TypeNameClashError` returned by `NewConfig`, and a keyword used in
both forms a `*xcl.TypeFormError`. A plugin that fails to load
(`*xcl.PluginLoadError`) or a block type a plugin provides that something else
already provides (`*xcl.TypeNameClashError`) is returned by the first
operation.
With no options, you get a config that parses and validates HCL but never
touches a real provider or disk — useful for testing.

## What `Apply` actually does

[`config.go:110`](../config.go#L110):

1. Construct a `parser.Parser` for this call only, handing it `Config`'s
   `StateStore`, `Catalog`, and variables via `ParserOptions`.
2. Call `p.Apply(paths...)`, see
   [Parser & Entity Lifecycle](parser-lifecycle.md).
3. Adopt the entities the parse produced as the configuration's own.
4. If a `StateStore` is configured, `Save` the new state.
5. Return the error from `p.Apply`, if any.

Before anything is created or changed, `p.Apply` rejects a configuration
with no blocks (`xcl.ErrEmptyConfiguration`: use `Destroy` to remove
everything), then decides what happens to every entity without changing
anything: each provider-backed entity is created, left alone, updated or
replaced, as its provider's `Changed` answers. Only then does it destroy the
entities being replaced and those in the previous state that are no longer
in the configuration, children first, saving the state after each one, and
then create and update in dependency order.

State is saved even when the apply failed. When a provider call fails,
`p.Apply` returns the state the walk reached along with the error: reached
entities with their new status, the failing entity as `failed`, and the
previous entry of entities that were not reached.
When destroying a replaced or removed entity fails, nothing is created or changed, and
the state returned is the previous state minus what was destroyed, with the
failures as `destroy_failed`; the next apply retries them first.
`Config.Apply` saves that state and then returns the error. Only when
`p.Apply` returns no state at all (the configuration didn't parse or
validate, declared no blocks, the dependency graph couldn't be built, or
deciding failed) is nothing saved. See
[State & Persistence](state.md#state-saved-after-a-failed-apply).

`Config.Validate` ([`config.go:77`](../config.go#L77)) answers only whether a
configuration is valid, returning `error` alone. It calls `p.Validate(paths...)`,
which parses every file and then runs validation to completion — **without**
decoding bodies, walking the DAG or reaching a provider. A nil error means the
configuration is valid; otherwise the returned `*errors.ConfigError` collects
every problem found.

Validation runs three stages in order — structure, then references, then
properties — and each gathers all of its own findings before the next is
considered. A later stage is skipped when an earlier one found anything, because
checking properties on a reference that resolves nowhere would only report
consequences of a problem already reported.

## What `Destroy` does

[`config.go:155`](../config.go#L155) needs no configuration:

1. Load the saved state from the `StateStore` (or use the in-memory state
   when there is none). Nothing saved, or an empty state, returns nil and
   writes nothing; a load error is returned as `failed to load state: ...`.
2. Construct a `parser.Parser` and call `p.Destroy(saved)`, which destroys
   every entity children first, from the same dependency graph as create,
   built from the links each entity saved. Unrelated entities are destroyed
   in parallel. Variables, outputs, modules, registered types and disabled
   blocks never reach a provider.
3. The state is saved after every entity, so an interrupted destroy
   resumes from it. An entity whose destroy fails stays as
   `destroy_failed`, together with everything it depends on, and is named in
   the returned error; calling `Destroy` again retries it.
4. Adopt what is left as `c.currentState`.

See [Parser & Entity Lifecycle](parser-lifecycle.md#destroy) and
[State & Persistence](state.md#state-during-a-destroy).

## Entity metadata convention

Every entity type — builtin (`resources.Module`, `resources.Output`, ...)
or plugin-defined — embeds [`types.ResourceBase`](../types/resource.go#L58),
which in turn embeds [`types.Meta`](../types/resource.go#L5):

```go
type ResourceBase struct {
    DependsOn []string `xcl:"depends_on,optional"` // exactly what the user wrote
    Disabled  bool     `xcl:"disabled,optional"`
    Meta      Meta     `xcl:"meta,optional"`
}

type Meta struct {
    ID, Name, Type, Module, File string
    Line, Column                 int
    Properties                   map[string]any
    Links                        []string // every dependency, orders create and destroy
    Status                       string   // see below
}
```

`Status` is set by xcl, never by providers, and is one of `created`,
`updated`, `failed`, `destroyed` or `destroy_failed`
([`types/status.go`](../types/status.go)). The status saved by the last apply
decides what the next apply does with the entity: `created` and `updated`
entities are read and then left alone, updated or replaced as their
provider's `Changed` answers, `failed` and `destroy_failed` entities are
replaced: destroyed and created again. `destroyed` is
never saved: a destroyed entity leaves the state. See
[State & Persistence](state.md#entity-statuses).

`types.GetMeta(resource any) (*Meta, error)` ([`types/resource_helpers.go`](../types/resource_helpers.go))
is the canonical way engine code reads this off an arbitrary entity value —
it walks embedded fields by reflection, so it works uniformly whether the
entity is a compiled-in Go struct or one dynamically built by
`schema.CreateInstanceFromSchema` (see [Plugin Architecture](plugins.md)).
This is what lets `internal/parser` and `internal/catalog` operate on
entities as `any` without knowing their concrete type.

## Where to go next

- Writing or hosting a provider: [Plugin Architecture](plugins.md)
- How HCL becomes provider calls, in what order: [Parser & Entity
  Lifecycle](parser-lifecycle.md)
- What gets persisted between runs: [State & Persistence](state.md)
- How `module` blocks are resolved: [Module System](modules.md)
