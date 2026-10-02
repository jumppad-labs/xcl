# Plugin Architecture

A provider (plugin) is the thing that actually creates, updates, and
destroys whatever a resource represents (a container, a network, a cloud
resource, ...). This page covers how a provider is authored, how it's
exposed to the engine, and the two ways it can be hosted.

## The chain of interfaces

There are three distinct interfaces in play, each solving a different
problem. Understanding why there are three (not one) is the key to this
package:

```
ResourceProvider[T]   -- what a plugin author writes: typed, one Go struct T
        |             (plugins/provider.go)
        v
ProviderAdapter       -- uniform, untyped ([]byte in/out) contract
        |             (plugins/adapter.go)
        v
PluginHost            -- adds GetTypes()/Stop(), abstracts in-process vs gRPC
                      (plugins/plugin_host.go)
```

### 1. `ResourceProvider[T]` — what you write

A plugin author writes one of these per resource type, with `T` being their
own concrete Go struct (e.g. `*ContainerResource`):

```go
type ResourceProvider[T any] interface {
    Init(state State, functions ProviderFunctions, logger logger.Logger) error
    Create(ctx context.Context, resource T) (T, error)
    Destroy(ctx context.Context, resource T, force bool) error
    Read(ctx context.Context, old T, new T) (T, error)
    Update(ctx context.Context, resource T) (T, error)
    Changed(ctx context.Context, old T, new T) (bool, error)
    Functions() ProviderFunctions
}
```

This is the ergonomic, type-safe surface — no manual JSON marshaling, no
`any`. What each method receives, may change and returns, and when xcl calls
it, is in the [Plugin Developer Guide](plugin-developer-guide.md). In short:

- `Init` gets a plugin scoped logger, for messages written outside a
  provider call. During a call, log through `plugins.Logger(ctx)`, which xcl
  binds to the resource and step (see [Plugin logging](#plugin-logging)).
- `Read(ctx, old, new)` reports the real resource. `old` is the copy saved by
  the last apply, `new` is the configured copy; `Read` fills `new` in and
  returns it. It is only called for resources in the previous state.
- `Read` returns `plugins.ErrNotFound`
  ([`plugins/errors.go`](../plugins/errors.go)) when the real resource no
  longer exists, and xcl creates it again. xcl checks for it with
  `errors.Is`, so it can be wrapped.
- `Changed(ctx, old, new)` compares the saved copy with what `Read` returned.
  Embed `plugins.DefaultChanged[T]`
  ([`plugins/changed.go`](../plugins/changed.go)) to get a comparison of the
  JSON form of both copies that ignores `meta`, `depends_on` and `disabled`;
  define `Changed` on the provider to override it.

### 2. `ProviderAdapter` — the uniform contract

The rest of the engine (the parser's DAG walk, the plugin registry) can't
work with a different generic type per resource — it needs one interface it
can call regardless of which plugin or resource type it's dealing with:

```go
// plugins/adapter.go
type ProviderAdapter interface {
    Init(state State, functions ProviderFunctions, logger logger.Logger) error
    Validate(ctx context.Context, entityData []byte) error
    Create(ctx context.Context, entityData []byte) ([]byte, error)
    Destroy(ctx context.Context, entityData []byte, force bool) error
    Read(ctx context.Context, oldEntityData []byte, newEntityData []byte) ([]byte, error)
    Update(ctx context.Context, entityData []byte) ([]byte, error)
    Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error)
}
```

Everything is `[]byte` (JSON) in and out. This is the interface
`internal/parser/lifecycle.go` actually calls during the DAG walk (see
[Parser & Resource Lifecycle](parser-lifecycle.md)) — it never knows or
cares whether the concrete implementation is local or remote.

`TypedProviderAdapter[T]` ([`plugins/adapter.go`](../plugins/adapter.go))
is the bridge between the two: it wraps a `ResourceProvider[T]`, and each
method does `json.Unmarshal([]byte) -> T`, calls the typed provider, then
`json.Marshal(T) -> []byte`. A plugin author never constructs this
directly — `RegisterResourceProvider` does it for you (see below).
`TypedProviderAdapter.Read` returns the provider's error unwrapped, so
`ErrNotFound` reaches the parser intact.

A second implementation, `GRPCResourceProviderAdapter`
([`plugins/grpc_resource_adapter.go`](../plugins/grpc_resource_adapter.go)),
satisfies the same interface but forwards each call over gRPC to a plugin
running in a separate process. From the parser's point of view these two
are indistinguishable — same interface, same call sites.

### 3. `PluginHost` — in-process vs. out-of-process

```go
// plugins/plugin_host.go
type PluginHost interface {
    GetTypes() []RegisteredType
    Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error
    Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)
    Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error
    Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error)
    Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)
    Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error)
    Stop()
}
```

Every lifecycle method takes the provider call's context, which carries the
call's logger; `PluginEntityProvider`
([`plugins/plugin.go`](../plugins/plugin.go)), the lifecycle half of a
`Plugin`, has the same signatures.

Two implementations:

- **`DirectPluginHost`** ([`plugins/direct_plugin_host.go`](../plugins/direct_plugin_host.go)) —
  wraps a `Plugin` living in the same process. `NewDirectPluginHost(emit,
  state, plugin)` calls the plugin's `Init` with a plugin scoped logger that
  emits to `emit`. Every method is a direct passthrough call, after naming
  the plugin as the source of the call's logger. Used for embedded/in-process
  plugins and in tests (see `internal/parser/test_plugin.go`).
- **`GRPCPluginHost`** ([`plugins/grpc_plugin_host.go`](../plugins/grpc_plugin_host.go)) —
  created with `NewGRPCPluginHost(emit, state)`, `Start(path)` starts a
  plugin binary as a subprocess and talks to it over gRPC
  (`plugins/grpc_server.go` is what runs *inside* the plugin process).
  `GetTypes()` calls the remote `GetTypes` RPC once, then builds and caches
  one `GRPCResourceProviderAdapter` per returned type (`h.cachedTypes`,
  `h.typesCached`) — so the gRPC round-trip for type discovery happens
  once per host, not once per lifecycle call.

Error values don't survive gRPC: the plugin process sends an error back as a
string. So that `ErrNotFound` still means "not found" out of process,
`ReadResponse` in [`plugins/plugin.proto`](../plugins/plugin.proto) has a
`not_found` field. The plugin side (`GRPCServer.Read`) sets it when the
adapter's error `errors.Is` `ErrNotFound`, and the host side
(`grpcPluginWrapper.Read`) turns it back into an error wrapping
`ErrNotFound`. Every other error arrives at the host as a plain error with
the provider's message.

## Registering a provider

A plugin embeds `PluginBase` ([`plugins/plugin.go`](../plugins/plugin.go))
and, per resource type, calls:

```go
func RegisterResourceProvider[T any](
    p *PluginBase, logger logger.Logger, state State,
    typeName, subTypeName string,
    resourceInstance T, provider ResourceProvider[T],
) error
```

This does three things:

1. Wraps `provider` in a `TypedProviderAdapter[T]` and calls `Init` on it,
   passing on `logger`, the plugin scoped logger the plugin's own `Init` was
   given.
2. Generates a JSON schema from `resourceInstance` via
   `schema.GenerateSchemaFromInstance` (reflects over the struct — see
   `internal/schema/serialize.go`).
3. Appends a `RegisteredType{Type, SubType, Schema, Adapter}` to
   `PluginBase.registeredTypes`.

A plugin provides as many block types as it likes: call
`RegisterResourceProvider` once per type in `Init`, each with its own
provider. The adapter adds the detail `provider=<block type>` to the plugin
scoped logger it passes to each provider's `Init`, so the providers of one
plugin are told apart in what they log outside a call. (It is added with
`logger.WithTag`, which only tags xcl's own event logger, so it applies to
in-process plugins; inside an external plugin process the plugin scoped logger
sends to the host over gRPC and is passed on unchanged.) During a call the
event names the resource and its type instead. Both plugins in
[`example/plugin`](../example/plugin) provide two types this way, the
in-process one `postgres` and `redis`, the external one `app` and `ingress`.

`PluginBase.GetTypes()` just returns that slice — it's what both
`DirectPluginHost.GetTypes()` (directly) and `GRPCPluginHost.GetTypes()`
(via a `GetTypes` RPC call to the plugin process) expose upward.

The out-of-process case is why the schema exists at all: the host process
doesn't have `T` compiled in, so it can't decode `entityData` itself. When a
resource of a given type needs to be *instantiated* from HCL (not just
passed through as `[]byte`), the host reconstructs a Go type dynamically
from the schema via `schema.CreateInstanceFromSchema` — see
[`plugins/registry/plugin_registry.go:createResourceFromPlugins`](../plugins/registry/plugin_registry.go).

## `PluginRegistry` — tying it together

[`plugins/registry/plugin_registry.go`](../plugins/registry/plugin_registry.go)
holds a `[]plugins.PluginHost` plus the compiled-in builtin types.
`NewPluginRegistry()` takes no logger, and registering only records:
`RegisterPlugin` for in-process plugins, `RegisterPluginWithPath` for a gRPC
plugin binary, and `DiscoverPlugins(dirs, pattern)` for directories to search
for binaries named like `pattern` (`xcl-plugin-*` when empty).

Nothing is started until `Load(emit)`, which `Config` calls at the start of
the first `Validate`, `Apply` or `Destroy` (and the parser calls again, which
is free). `Load` runs once per registry however many `Config`s share it, and
later calls return the first call's result, failure included. It reports what
it does to `emit`:

- a `discover` start, `discover` log events for the binaries found, then a
  success with `Meta` `dirs` and `count` (or an error), when there are
  directories to search;
- for each plugin, a `load` start, then a `load` success whose `Meta` names
  the `plugin` and its `block_types`, or a `load` error.

A registered plugin that fails to start fails the load, and so the
operation, with a `*xcl.PluginLoadError` (from
[`errors/plugin_load_error.go`](../errors/plugin_load_error.go), re-exported
from the root package) that names the plugin and matches `xcl.ErrPluginLoad`
with `errors.Is`. A discovered binary that fails to start is rejected, with a
`load` error event whose `Meta` has `rejected=true`, and skipped; discovery
fails only when every discovered plugin fails. `Loaded()` reports whether
`Load` has run.

`Activate(emit)` routes the messages plugins write outside a provider call,
such as `Init` messages and go-plugin's, to the operation in progress; `Config`
activates each operation before loading. While plugins load, those messages
go to the loading operation.

Its two jobs, used from two different places in the parser:

- **`CreateResource(resourceType, resourceName) (any, error)`** — instantiate
  a new (empty) resource instance for an HCL block. Tries builtins first,
  then [configuration-only types](#configuration-only-types) (a real
  instance of the registered Go type), then walks plugin hosts' `GetTypes()`
  looking for a schema match, and builds a dynamic instance via
  `schema.CreateInstanceFromSchema` if found. Used while *parsing* HCL,
  before any dependency graph exists.
- **`GetProviderForResource(resource any) plugins.ProviderAdapter`** —
  given an already-decoded resource, find the `ProviderAdapter` that
  handles its type (matches `types.GetMeta(resource).Type` against each
  host's `GetTypes()`). Used during the DAG walk to actually invoke
  lifecycle methods (see [Parser & Resource Lifecycle](parser-lifecycle.md)).

These are two different methods on the same struct because they solve two
different problems (build a Go value vs. look up an RPC target) — code that
only needs the second one can depend on the narrower
`parser.ProviderResolver` interface instead of the concrete
`*PluginRegistry` (see the "testing" note in [Parser & Resource
Lifecycle](parser-lifecycle.md)).

## Plugin logging

Plugins never write output. Everything a plugin logs becomes an
`events.Event` with the phase `log` on the same stream as xcl's own events,
delivered to the application's receiver (see
[Parser & Resource Lifecycle](parser-lifecycle.md#events-parseroptionsemit)).
The level and text are in `Meta` under `events.KeyLevel` (`level`) and
`events.KeyMessage` (`message`), and the key/value arguments of the call are
the other `Meta` details, under their own names. The `logger` package holds
the `Logger` interface (`Debug`/`Info`/`Warn`/`Error(msg, args ...any)`) and
its one implementation, `logger.New(emit, base)`, which turns each call into
a copy of `base` with the message in `Meta`. `logger.Nop()` emits nothing.

### Logging during a provider call

A provider logs through the logger in the call's context:

```go
func (p *postgresProvider) Create(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
    db.ConnectionString = connectionString(db)
    plugins.Logger(ctx).Info("created database", "connection_string", db.ConnectionString)
    return db, nil
}
```

The parser puts a logger in the context before each call with
`plugins.WithLogger` (see
[The operation context](parser-lifecycle.md#the-operation-context)), bound to
the resource (`ResourceID`, `ResourceType`, `File`) and the step (`Operation`
is `create`, `read`, `changed`, `update` or `destroy`). The provider passes no
resource details, and the message arrives between the call's `start` and its
`success` or `error`. Outside a provider call, `plugins.Logger(ctx)` returns a
logger that emits nothing.

The event's `Source` is the plugin's name: `core` is xcl itself. The host
sets it with `logger.WithSource` before the call reaches the plugin:

- in-process, `DirectPluginHost` re-sources the call's logger with the Go
  type name of the plugin, `plugins.PluginName` (`ExamplePlugin`);
- external, `GRPCPluginHost` uses the binary's file name,
  `plugins.PluginBinaryName` (`xcl-plugin-person`, without `.exe`).

Loggers are not kept on the provider: providers are called concurrently for
different resources, so each call carries its own.

### The plugin scoped logger

`Plugin.Init(logger, state)` and each provider's `Init` get a plugin scoped
logger, for messages written outside a provider call. Its events have the
plugin's name as `Source` and `load` as `Operation`, and no resource. The
registry forwards them to the operation loading the plugins, or later to the
active operation (see `Activate` above); with neither, they are dropped.

### Across the process boundary

An external plugin's messages cross gRPC through the host callback service
([`plugins/grpc_host_callback.go`](../plugins/grpc_host_callback.go)):

- **Call IDs.** Before each call, the host registers the call's logger,
  sourced to the plugin, under a new ID and sends the ID as the `xcl-call-id`
  gRPC metadata ([`plugins/grpc_calls.go`](../plugins/grpc_calls.go)). Inside
  the plugin, `GRPCServer` puts a `GRPCLogger` carrying that ID into the
  call's context, so `plugins.Logger(ctx)` works the same as in-process. Each
  message is sent as a `LogRequest` with the ID in its `call_id` field, and
  the host writes it to that call's logger, so it gets the call's resource,
  step and receiver. A message with no ID, or one for a call that has
  already returned, goes to the plugin scoped logger.
- **Values as text.** `LogRequest.args` is a list of strings, so detail values
  cross the boundary as text. The host converts them back to integers,
  floats and booleans where they parse as one, so they may not keep their
  original types.
- **The plugin scoped logger inside the plugin** is an asynchronous logger:
  `Init` runs before the host has connected, so each message is sent from its
  own goroutine once the connection is available. Such messages may arrive
  out of order, and are dropped when the host can't be reached. A message
  that fails to send is always dropped; logging never changes the outcome of
  a call.

### go-plugin's messages

go-plugin starts external plugins and logs how it starts and talks to the
plugin process, and passes on anything the process writes to stderr.
`GRPCPluginHost` gives it an adapter
([`plugins/hclog_adapter.go`](../plugins/hclog_adapter.go)) that turns all of
that into log events from the plugin scoped logger, with the detail
`component=go-plugin` so a receiver can tell them apart from the plugin's own.
They are routed like any other message outside a call, to the operation
loading plugins or the active one. go-plugin's info and debug messages are
passed on at debug and its trace messages are dropped; warnings and errors
keep their level. The `received EOF, stopping recv loop` debug message is
dropped as well: it reports the plugin's stdio stream ending, which happens
every time the plugin process stops, but carries an `err=` field that reads
like a failure.

One message escapes this. When an external plugin is stopped within
milliseconds of starting, go-plugin v1.6.3 writes
`[ERR] plugin: plugin acceptAndServe error: broker closed` to standard error
through the standard library's global logger, which xcl does not change. It
is outside xcl's control and is accepted as a known exception.

### What is no longer logged

There is no framework log message around provider calls: the lifecycle
`start`, `success` and `error` events describe each call, and a plugin
loading is a `load` event rather than a log message.

## Configuration-only types

Not every block type needs a plugin. `PluginRegistry.RegisterType(name,
&MyType{})` registers a plain Go type (a pointer to a struct embedding
`types.ResourceBase`) under a block type name, with no plugin and no
provider:

```go
r := registry.NewPluginRegistry()
err := r.RegisterType("postgres", &PostgreSQL{})
```

`CreateResource` builds registered types with `reflect.New`, like builtins, so
blocks decode into the developer's own type and state reload returns that
type. The parser learns about them through the one-method
`parser.TypeRegistry` interface (`IsRegisteredType`), which `*PluginRegistry`
satisfies. The lifecycle and the destroy walk treat a registered type like a
builtin: it gets a success event and no provider is ever called for it. On
apply its status is left unchanged; on destroy it is removed from the state.

Type names are unique across the registry, and a clash is a
`*registry.TypeNameClashError` naming the type. When it is found depends on
when the name arrives:

- `RegisterType` checks immediately, against builtins, registered types and
  the types of plugins that have already loaded, and leaves the registry
  unchanged on a clash.
- Plugin types are checked when plugins load, against all of those and the
  other types the same plugin provides. A clashing plugin is stopped and not
  added, and the load fails, and with it the operation, even when other
  discovered plugins loaded. A registered type that shares a name with a
  plugin that has not loaded yet is accepted by `RegisterType` and reported
  here.

[`example/configonly`](../example/configonly) parses a Kubernetes-like
configuration into registered types this way, with no plugin at all.
[`example/plugin`](../example/plugin) is the other half of the picture, four
block types provided by two plugins instead.

## Mocks

Both `plugins.ProviderAdapter` and `plugins.State` have generated
`testify`/`mockery` mocks under [`plugins/mocks/`](../plugins/mocks/)
(config: [`.mockery.yml`](../.mockery.yml)). `internal/parser.ProviderResolver`
(a narrow interface covering just `GetProviderForResource`) has its own
mock under [`internal/parser/mocks/`](../internal/parser/mocks/). Together
these let tests exercise the DAG-walk/lifecycle logic without a real plugin
process or a hand-written fake plugin.
