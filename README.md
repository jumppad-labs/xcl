# HCL Configuration Parser

[![Go Reference](https://pkg.go.dev/badge/github.com/jumppad-labs/xcl.svg)](https://pkg.go.dev/github.com/jumppad-labs/xcl)

This package allows you to process configuration files written using the HashiCorp Configuration Language (HCL).
It has full resource linking where a parameter in one configuration stanza can reference a parameter in another stanza.
Variable support, and Modules allowing configuration to be loaded from local or remote sources.

The project aims to provide a simple API allowing you to define resources as Go structs without needing to fully understand
the HashiCorp HCL2 library. 

HCLConfig has a full AcyclicGraph that allows you to process configuration with strict dependencies. This ensures
that a parameter from one configuration has been set before the value is interpolated in a dependent resource.

Parsing is a two step approach, first the parser reads the HCL configuration from the supplied files, at this stage a 
graph is computed based on any references inside the configuration. For example given the following two resources.

```javascript
resource "postgres" "mydb_2" {
  location = "localhost"
  port = 5432
  name = "mydatabase"

  username = "db2"
  password = resource.postgres.mydb_1.password
}

resource "postgres" "mydb_1" {
  location = "localhost"
  port = 5432
  name = "mydatabase"
  
  username = "db1"
  password = random_password()
}
```

#### Step 1:
When the first pass of the parser runs it will read `mydb_2` before `mydb_1`, marshaling each resource into a struct and
calling the optional `Parse` method on that struct. At this point none of the interpolated properties like 
`resource.postgres.mydb_1.password` have a value as it is assumed that the referenced resources does not yet exist. At
this point the parser replaces the interpolated value with a default value for the field.  

#### Step 2:
After resources have been processed from the HCL configuration a graph of dependent resources
is calculated. Given the previous example where resource `mydb_2` references a property from 
`mydb_1`, the resultant graph would look like the following.

```
| -- resource.postgres.mydb_2
     |  -- resource.postgres.mydb_1
```

This graph is then walked, as each resource is processed, any referenced properties are resolved and assigned
to the struct. For example, when `resource.postgres.mydb_2` is processed the `password` field that contains
a reference to `resource.postgres.mydb_1` will be assigned the actual value from the linked resource.

The optional `Process` method on the struct is also called, where a resource may contain computed fields the
user can implement these computations in `Process` as this will make their value available to the next
node in graph.


## Example

The [`example`](./example) directory holds three self-contained programs, each
with its own configuration and Go types.

### Configuration only

[`example/configonly`](./example/configonly) uses XCL for what it is most
often needed for: parsing a configuration into Go objects. The block types
([`configonly/resources`](./example/configonly/resources)) are plain Go types
registered on the plugin registry, with no plugin and no provider.

Its configuration ([`configonly/config`](./example/configonly/config)) is a
small Kubernetes-like deployment, chosen because that shape needs everything
a configuration language is asked for:

- **blocks nested inside blocks** — `resources` inside `container`, holding
  `limits` and `requests` of its own
- **repeated blocks** — two `container` blocks, each with its own `port` and
  `env` blocks, decoded into a Go slice; a block that appears at most once is
  a pointer, and is `nil` when it is left out
- **links between resources** — the `service` names the `deployment` by id
  and reads its target port out of it
  (`resource.deployment.api.container[0].port[0].container_port`), the
  `ingress` names the `service` the same way, and the container's environment
  is read out of a `config_map`'s map attribute
  (`resource.config_map.api.data.db_host`). Kubernetes matches a service to
  its pods with a label selector because a manifest can not point at another
  object; a reference does it directly.

```hcl
resource "deployment" "api" {
  replicas = variable.replicas

  container {
    name  = "api"
    image = "ghcr.io/example/api:${variable.image_tag}"

    port {
      name           = "http"
      container_port = 8080
    }

    env {
      name  = "DB_HOST"
      value = resource.config_map.api.data.db_host
    }

    resources {
      limits {
        cpu    = "500m"
        memory = "512Mi"
      }
    }
  }
}
```

After `Apply`, the program gathers every block it reads into one struct of its
own with a single call, rather than looking each type up separately (see
[Filling a struct of your own](#filling-a-struct-of-your-own)):

```go
type appConfig struct {
	ConfigMaps  []*resources.ConfigMap
	Deployments []*resources.Deployment
	Service     *resources.Service
	Ingress     *resources.Ingress
}

var cfg appConfig
err := c.Decode(&cfg)
```

### Plugins

[`example/plugin`](./example/plugin) applies its configuration
([`plugin/config`](./example/plugin/config)) through two plugins, each
providing two block types with a provider of its own. `ExamplePlugin`
(`internal/`) is an in-process plugin that provides `postgres` and `redis`,
filling in a computed `connection_string` on both. `external` (`external/`)
is an external plugin, compiled to its own binary and called over gRPC, that
provides `app` and `ingress`. A plugin registers each block type with its own
call to `plugins.RegisterResourceProvider` in `Init`.

Computed values cross both ways: `app` reads the `connection_string` the
in-process `redis` provider computed, and `ingress` reads the `url` the
external `app` provider computed, so a value moves between two plugins and
between two types of the same plugin. The configuration also uses a module,
[`plugin/config/modules/db`](./example/plugin/config/modules/db).

Their providers log from each lifecycle method with
`plugins.Logger(ctx).Info("created database", "connection_string", ...)`,
passing no resource details. xcl binds that logger to the resource, its type,
the file it was declared in and the step, and names the plugin as the
message's source, so the in-process and the external plugin read the same in
the output:
`INFO created database source=ExamplePlugin operation=create phase=log resource=resource.postgres.main ...`.

### Application configuration

[`example/appconfig`](./example/appconfig) is the shape configuration most
often takes outside infrastructure: one application configuration file, of the
kind usually written as JSON, read into a tree of Go structs. Its
configuration is a single file,
[`appconfig/config/app.xcl`](./example/appconfig/config/app.xcl), and one
block type ([`appconfig/resources`](./example/appconfig/resources)) is
registered for it, with no plugin and no provider.

Every shape a JSON document is built from has an equivalent: an object is a
nested block held in a pointer (`server`, and `tls` and `client_auth` nested
below it), an array of objects is a repeated block held in a slice (`service`,
each holding repeated `route` blocks of its own), an array of values is a list
attribute (`ciphers`), and an object whose keys the Go type does not know is a
map attribute (`labels`, `options`, `headers`). Nesting goes four blocks deep
on two different paths.

```hcl
resource "application" "api" {
  name        = "checkout-api"
  environment = variable.environment

  server {
    host = "0.0.0.0"
    port = 8443

    tls {
      enabled   = true
      cert_file = "/etc/certs/api.pem"

      client_auth {
        mode    = "require_and_verify"
        ca_file = "/etc/certs/ca.pem"
      }
    }
  }

  service {
    name = "payments"
    url  = "https://payments.internal"

    route {
      path    = "/v1/charges"
      methods = ["POST"]

      rate_limit {
        requests_per_second = 50
      }
    }
  }
}
```

The program prints the parsed configuration as a tree, then prints the same
resource as JSON, which is the document this configuration replaces.

### Running them

Every example keeps its state in a file, applies the configuration and prints
the resources it parsed. The plugin example then `Destroy`s everything through
the providers and prints what is left under `## Destroyed`. Each sends
everything xcl reports, lifecycle events, plugin log messages and errors, to
the shared [`example/prettylog`](./example/prettylog) receiver, set up in one
line, which writes styled lines to standard error. It shows info and above;
set `XCL_LOG_LEVEL=debug` to see plugin loading and `Init` messages too. The
program's own report goes to standard output.

Run any of them from its directory with `make run`. For `plugin` this builds
the external plugin into `build/` first, and it has the extra Makefile targets
`build` and `clean`; every example has `run` and `test`. The tests for all
three run as part of `go test ./...`, and the plugin tests build the external
plugin themselves.

Block types are defined as Go structs that embed `types.ResourceBase` and map
configuration to fields with `xcl` tags.

```go
// PostgreSQL defines the block type `postgres`
type PostgreSQL struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Port     int    `xcl:"port" json:"port"`
	Username string `xcl:"username" json:"username"`
	Password string `xcl:"password" json:"password"`

	// Fields that are of `struct` type must be marked as a `block`, and be a
	// pointer
	Timeouts *Timeouts `xcl:"timeouts,block" json:"timeouts,omitempty"`

	// A computed field is set by a provider, never by configuration, and must
	// be optional
	ConnectionString string `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`
}
```

### Configuration only types

When a block only holds configuration and nothing needs to be created, read or
destroyed, register its Go type on the plugin registry with `RegisterType`.
No plugin or provider is needed.

Everything a configuration declares is an entity, and an entity has a type
and an optional subtype. The type is the keyword a block leads with, and the
subtype, when there is one, is its first label:

```go
r := registry.NewPluginRegistry()

// a type with a subtype, declared: server "big" "web" {}, addressed server.big.web
err := r.RegisterType(&Server{}, "server", "big")

// a type without one, declared: cache "main" {}, addressed cache.main
err = r.RegisterType(&Cache{}, "cache")

// resource is a type like any other: resource "postgres" "main" {}
err = r.RegisterType(&PostgreSQL{}, "resource", "postgres")

c, err := xcl.NewConfig(xcl.WithPluginRegistry(r))
err = c.Apply("./config")
```

Registered blocks are decoded into your own Go type, take part in references
(`server.big.web.location`, `cache.main.location`) and dependency ordering,
work in modules and when disabled, and are saved to state. They are never
passed to a provider. `RegisterType` takes a pointer to a struct that embeds
`types.ResourceBase`.

A type keyword takes a subtype for every registration or for none, so an
address can always be read by position. `resource` always takes one.
Registering `server` without a subtype after registering it with one, or the
other way round, fails with a `*registry.TypeFormError`.

Every type and subtype must be unique across builtin blocks (`variable`,
`output`, `module`, `root`), registered types and plugin types.
`server` and `resource "server"` are different types and do not clash.
Registering a type and subtype a builtin or another registered type already
has fails straight away with a `*registry.TypeNameClashError` that names it,
i.e. `resource.postgres`. A clash with a type a plugin provides is reported
when the plugins load, see below.

### Registering plugins

The registry needs no logger. Registering a plugin only records it:

```go
r := registry.NewPluginRegistry()

err := r.RegisterPlugin(&MyPlugin{})                  // an in-process plugin
err = r.RegisterPluginWithPath("./bin/xcl-plugin-x") // an external plugin binary
r.DiscoverPlugins([]string{"~/.xcl/plugins"}, "")    // directories to search
```

Plugins load when they are first needed, at the start of the first
`Validate`, `Apply` or `Destroy`, and only once per registry however many
Configs share it. Discovery, loading and the rejection of a discovered binary
that is not a plugin are reported as `discover` and `load` events. A
registered plugin that fails to load, such as a path that does not exist,
fails that first operation with an error matching `xcl.ErrPluginLoad`, whose
`*xcl.PluginLoadError` detail names the plugin. A plugin type whose name
clashes with a known type fails it with a `*registry.TypeNameClashError`.

### Querying a configuration

Ask the configuration for what you want, as your own Go type, in a single call.

```go
// one entity, by its address
db, err := xcl.Find[PostgreSQL](c, "resource.postgres.main")

// every resource "postgres" block
databases, err := xcl.FindByType[PostgreSQL](c, "resource", "postgres")

// the one you expect there to be exactly one of
ingress, err := xcl.FindOne[Ingress](c, "resource", "ingress")

// every entity of a Go type, without naming it as a string
all, err := xcl.All[PostgreSQL](c)
```

The segments given to `FindByType` and `FindOne` are the leading segments of an
address, matched in order from the left: segment one is the type, segment two
the subtype where the type takes one. `xcl.FindByType[Server](c, "server",
"big")` returns every `server "big"` entity. A block declared without a
subtype, `container "nics"`, is a different type from
`resource "container" "nics"` and is reached as
`xcl.FindByType[Container](c, "container")`.

Everything a configuration declares can be enumerated without naming a type,
and an entity from that enumeration converts in one call:

```go
for _, entity := range c.Entities() {
    // ...
}

container, err := xcl.As[Container](entity)
```

A registered type is returned as the value held in state, so changing it
changes state. A plugin type is returned as a copy.

#### Filling a struct of your own

An application usually wants its configuration gathered into one structure it
can pass around. `Decode` fills a struct of your own from the entities a
configuration declares, in one call, so reading a new block type needs only a
new field. Given this configuration, with `Server` and `Mount` registered as
`server` and `mount`:

```hcl
server "vault_current" {
  location = "http://localhost:8200"
  type     = "current"
}

server "vault_arc" {
  location = "http://localhost:9100"
  type     = "arc"
}

mount "secrets_classic_v1" {
  path = "secrets/kv1"
  type = "kv1"

  classic {
    server     = server.vault_current
    mount_path = "v1/secrets"
  }
}
```

one call fills the application's struct:

```go
type Config struct {
	Servers     []*Server // every server block
	MountPoints []*Mount  // every mount block
	Name        string    // the application's own, left as it is
}

var cfg Config
if err := c.Decode(&cfg); err != nil {
	return err
}

// cfg.Servers holds vault_current then vault_arc, cfg.MountPoints holds
// secrets_classic_v1, its classic block holding the referenced server
```

Each exported field is matched by its type alone. No struct tags are read, and
field names play no part:

- A `[]*T` field, where `T` is a registered type, receives every entity of
  `T`, exactly as `xcl.All[T]` returns them: in the order they were written,
  disabled entities included, and as the configuration's own instances rather
  than copies. None declared gives an empty slice.
- A `*T` field, where `T` is a registered type, receives the one entity of
  `T`, and is set to `nil` when none is declared. When more than one is
  declared the call fails with the error `xcl.FindOne` returns for the same
  type, matching `xcl.ErrNotUnique`. A `Primary *Server` field would do that
  against the configuration above, which declares two servers.
- Every other field keeps its value: plain values, structs, and a `[]*T` or
  `*T` whose `T` is not registered or is provided by a plugin, which has no Go
  type to match. Nested structs are not entered.

A target that is not a non-nil pointer to a struct returns an error matching
`xcl.ErrInvalidDecodeTarget`, whose `*xcl.InvalidDecodeTargetError` detail
names what was passed. On any error no field is assigned, so the struct is
never left partly filled. Before `Apply` the call succeeds, leaving every
slice empty and every pointer `nil`.

#### Reading values a configuration publishes

An `output` is an entity like any other. Looking one up by its address returns
a `types.Output`, and the value it publishes is on its `Value` field.

```go
// output "web_database" { ... } at the root
out, err := xcl.Find[types.Output](c, "output.web_database")
url := out.Value // the published value

// an output published by a module
location, err := xcl.Find[types.Output](c, "module.analytics.output.location")
fmt.Println(location.Value)

// every declared output, at any module depth, as entities
outputs, err := xcl.FindByType[types.Output](c, "output")
outputs, err = xcl.All[types.Output](c)

// or every published value at once, keyed by address
published := c.Outputs()
```

An output needs its complete address, including the module it belongs to. A
bare name is not an address and is not inferred. Asking for an output as any
type other than `types.Output`, such as `xcl.Find[string]`, returns an error
matching `ErrTypeMismatch`; read `.Value`, or use `c.Outputs()`.

#### When a lookup cannot be answered

An empty result and an unanswerable question are never the same thing. A
well-formed query that matches nothing returns an empty result and a nil error;
a query that cannot be answered returns an error you can match by identity.

```go
db, err := xcl.Find[PostgreSQL](c, "resource.postgres.main")
switch {
case errors.Is(err, xcl.ErrNotFound):
    // nothing is declared at that address
case errors.Is(err, xcl.ErrTypeMismatch):
    // something is, but it is not a PostgreSQL
}

// the detail behind a failure is recoverable
_, err = xcl.FindOne[PostgreSQL](c, "resource", "postgres")
var many *xcl.NotUniqueError
if errors.As(err, &many) {
    fmt.Printf("expected one, found %d\n", many.Count)
}
```

The full vocabulary is `ErrNotFound`, `ErrUnknownType`, `ErrNotTypeable`,
`ErrNotRegistered`, `ErrTypeMismatch`, `ErrNotAnEntity` and `ErrNotUnique`.
`ErrNotFound` and `ErrNotUnique` are ordinary outcomes to handle; the other
five mean the question itself had no answer. `Decode` adds one more,
`ErrInvalidDecodeTarget` (`*xcl.InvalidDecodeTargetError`), for a target it
cannot fill. Errors returned from `Apply` and
`Destroy` are matchable the same way, so `errors.Is` reaches a failure nested
inside them without unwrapping anything by hand.

#### Two spellings, one implementation

Every lookup above is also a method on the configuration — `c.Find[T](addr)`,
`c.FindByType[T](...)`, `c.FindOne[T](...)`, `c.All[T]()`. The method form is
the idiomatic one and is what this documentation presents as the destination.

Generic methods arrived in Go 1.27, and this project supports Go 1.25.0, so the
method form is excluded below 1.27 by a build constraint while the package
level functions compile everywhere. **That is why every runnable example in
this repository uses the function form**: what a reader copies works on the
oldest supported toolchain. The two spellings delegate to one implementation
and cannot differ; tests assert they return equal values and equal errors.

`As` is a function in both worlds, because it takes no configuration for a
method to hang off.

`Decode` is the exception to the rule. It is not generic, so `c.Decode(&cfg)`
and `xcl.Decode(c, &cfg)` are both available on Go 1.25, and the examples use
the method form for it. The two spellings delegate to one implementation, as
the lookups do.

Lookups are linear scans over the configuration's entities. There is no index,
which is adequate at one configuration's scale; see `docs/state.md`.

### State and Destroy

Keep state between runs with a `StateStore`. `Apply` loads the saved state,
applies the configuration and saves the result. `Destroy` needs no
configuration: it destroys everything in the saved state, dependents before
what they depend on.

```go
c, err := xcl.NewConfig(
	xcl.WithPluginRegistry(r),
	// keeps state in ./.xcl/state.json, creating the directory and file if needed
	xcl.WithStatePath("./.xcl"),
)

err = c.Apply("./config")

// later, remove everything that was applied
err = c.Destroy()
```

The state is saved after each resource is destroyed, so an interrupted
`Destroy` picks up where it stopped. A resource whose destroy fails stays in
the state, with everything it depends on, and is named in the error; running
`Destroy` again retries it. Registered and builtin types never reach a
provider, they are just removed from the state.

A block removed from the configuration is destroyed on the next `Apply`,
before anything is created or changed. Applying a configuration with no
blocks fails with `xcl.ErrEmptyConfiguration` and changes nothing, use
`Destroy` to remove everything.

Register every type and plugin before the first operation: saved resources
of a type the registry does not know fail the load with
`state.UnknownTypesError` rather than being dropped. Config loads the plugins
before it loads state; code that loads a state store directly should call the
registry's `Load` first.

### Events and logging

xcl writes no output of its own. Everything it and its plugins report, each
step of the resource lifecycle, plugin loading, warnings, errors and the log
messages plugins write, is one stream of events delivered to the receiver set
with `WithEventHandler`. With no receiver xcl is silent.

The quickest way to see them is the shipped adapter to the standard
library's `log/slog`, set up in one line. It writes log messages at their own
level, lifecycle and loading events at info, failures at error, and filters
by the slog handler's level:

```go
c, err := xcl.NewConfig(
	xcl.WithPluginRegistry(r),
	xcl.WithEventHandler(events.SlogHandler(slog.Default())),
)
```

Every event is an `xcl.Event` (the same type as `events.Event`), one flat
shape for all of them:

| Field | Meaning |
|---|---|
| `Time` | when it happened |
| `Source` | `core` for xcl itself, otherwise the plugin's name |
| `Operation` | `validate`, `apply`, `destroy`, `parse`, `create`, `read`, `changed`, `update`, `discover`, `load`, or `events` for a blocked announcement |
| `Phase` | `start`, `success`, `error`, `log` for a log message, `blocked` |
| `ResourceType`, `ResourceID`, `File` | the resource the event is about and the file it was declared in |
| `Duration` | how long a step took, or how long emitting was blocked |
| `Error` | the failure, for the error phase |
| `Data` | the serialized resource, carried only when you ask for it with `WithEventData`, see below |
| `Meta` | details; a log message's level and text are under the reserved keys `events.KeyLevel` (`level`) and `events.KeyMessage` (`message`), which caller details never overwrite |

#### Resource data on events

Events carry no resource data by default. A resource's configuration and state
are not something to push through every receiver by accident, so you ask for
them:

```go
c, err := xcl.NewConfig(
	xcl.WithEventHandler(handler),
	xcl.WithEventData(xcl.EventDataProcessed),
)
```

| Level | What `Data` carries |
|---|---|
| `EventDataNone` | nothing, on any event. The default |
| `EventDataRaw` | on every lifecycle event, the resource as it was before the provider was called |
| `EventDataProcessed` | on a success event, the resource as xcl records it in state, including the values the provider filled in and the status it ended with. Other phases carry the same as `EventDataRaw` |

`EventDataProcessed` is byte for byte what state stores, so it goes straight to
`EncodeSavedEntity` below. This is how the examples show each resource as it is
created.

If you read `Event.Data` today, add `xcl.WithEventData(xcl.EventDataRaw)` to
keep what you had.

Each `Validate`, `Apply` and `Destroy` starts with its own `start` event and
ends with a `success` or an `error` carrying the error it returns. In between
come the `parse` events, and for each resource a `start` before each provider
call, then any log messages the provider writes during that call, with the
call's step as their operation, then a `success` or an `error`. Every failure
that is returned is also emitted as an error event.

Delivery guarantees:

- The receiver is called one event at a time, in the order the events were
  emitted, never concurrently.
- Emitting never waits for the receiver. Undelivered events are held in a
  bounded buffer, `WithEventBufferSize` (default
  `xcl.DefaultEventBufferSize`, 1024). When it is full, emitting waits
  rather than dropping an event, and once it can continue the receiver is
  sent one `blocked` event saying how long it waited.
- Every event of a call has been delivered before the call returns, whether
  it succeeds or fails.
- A panic in the receiver is not recovered. xcl starts no new provider call,
  lets the calls in progress finish and saves state, then the panic continues
  from `Validate`, `Apply` or `Destroy` with its original value and stack.
- xcl never changes the standard library's global logger.

One known exception to silence comes from go-plugin, which xcl uses to run
external plugins: when an external plugin is stopped within milliseconds of
starting, go-plugin writes `[ERR] plugin: plugin acceptAndServe error: broker
closed` to standard error through the standard library logger. It is outside
xcl's control.

The [`example/prettylog`](./example/prettylog) receiver shows the adapter in
use with a styled terminal handler.

Plugin authors log from a provider with the logger in the call's context,
which xcl has already bound to the resource and the step, so they pass none
of it themselves. It works the same in an in-process and an external plugin:

```go
func (p *provider) Create(ctx context.Context, db *PostgreSQL) (*PostgreSQL, error) {
	plugins.Logger(ctx).Info("created database", "remote_id", id)
	return db, nil
}
```

See [docs/plugins.md](./docs/plugins.md) for more.

### Converting to configuration text

`EncodeEntity` turns one entity back into configuration text in xcl's own
syntax, ready to print or write to a `.xcl` file:

```go
db, err := xcl.Find[*Postgres](c, "resource.postgres.main")
if err != nil {
	return err
}

text, err := xcl.EncodeEntity(db)
if err != nil {
	return err
}

fmt.Println(string(text))
```

```hcl
resource "postgres" "main" {
  location = "localhost"
  port     = 5432

  timeouts {
    connection = 10
    keep_alive = 60
  }
}
```

`EncodeSavedEntity` does the same from an entity's saved data, in the form
state stores it and events carry it at `EventDataProcessed`. It needs the
registry, which is what types the record, and loads its plugins if they are not
loaded already:

```go
text, err := xcl.EncodeSavedEntity(registry, event.Data)
```

Both write exactly one block, so convert several entities by calling once for
each.

**What is left out.** The text shows what a person wrote. xcl's own bookkeeping
is not written, and neither is `depends_on`, which by the time a configuration
is parsed holds the references xcl resolved as well as anything you wrote.
Values a provider filled in are left out too. Ask for them with
`IncludeComputed`, and each one is marked so a reader can tell it apart:

```go
text, err := xcl.EncodeEntity(db, xcl.IncludeComputed())
```

```hcl
resource "postgres" "main" {
  location          = "localhost"
  port              = 5432
  connection_string = "postgres://admin@localhost:5432/main" # set by the provider
}
```

**This text is for reading, not for reprocessing.** References come out as the
literal values they resolved to, comments and layout from the original file are
not kept, and output including provider-filled values does not validate, since
xcl refuses a configuration that sets them. A sensitive value is written as
`"(sensitive)"`; see [Sensitive values](#sensitive-values) for showing the
real value.

**When it fails** no text is returned, and the error says why:

| Error | Means |
|---|---|
| `ErrUnregisteredType` | the saved data names a type the registry does not know. `UnregisteredTypeError` names it |
| `ErrInvalidSavedData` | the data is not one saved entity record |
| `ErrNotEncodable` | the value is not an entity, or it is a `variable`, `output` or `module`, which are never written as configuration |

```go
if errors.Is(err, xcl.ErrUnregisteredType) {
	// register the type or its plugin
}
```

## Sensitive values

Passwords, tokens and other secrets are declared sensitive in the Go type that
holds them. xcl then shows them only as the fixed marker `(sensitive)` in its
logs, events, errors, configuration text and printed resources, and in your own
`fmt`, `log/slog` and `encoding/json` output. State keeps the real value, and
your code reaches it only by asking for it.

### Declaring a sensitive field

Give the field the type `types.Sensitive[T]`. Configuration sets it exactly as
it would set a plain field:

```go
type Postgres struct {
	types.ResourceBase `xcl:",remain"`

	Username string                  `xcl:"username" json:"username"`
	Password types.Sensitive[string] `xcl:"password" json:"password"`
}
```

```hcl
resource "postgres" "main" {
  username = "admin"
  password = env("DB_PASSWORD")
}
```

The same declaration works on a plugin's types. In tests, build one with
`types.NewSensitive("hunter2")`.

### Reading the real value

`.Reveal()` is the only way to the real value. Call it where the value is
used, such as when opening a connection, and do not print or log what it
returns: a revealed value is an ordinary value and is no longer protected.

```go
db, err := xcl.Find[Postgres](c, "resource.postgres.main")
if err != nil {
	return err
}

conn, err := sql.Open("postgres", dsn(db.Username, db.Password.Reveal()))
```

Sensitivity follows the value through the configuration. A value referenced
from a sensitive field, interpolated into a string, passed through a function,
passed into a module, or published by an output, is sensitive too. An output
holds its sensitive parts as `types.Sensitive` values, so
`xcl.Find[types.Output](c, "output.db").Value` may be a map whose `password`
entry is a `types.Sensitive[string]`.

### When a sensitive value meets a plain field

Assigning a sensitive value, or one derived from it, to a field that is not
declared sensitive fails validation, so `Validate` and `Apply` refuse the
configuration before anything is created:

```text
field "note" of resource.audit.main is not declared sensitive and cannot be
assigned a sensitive value
```

In Go, converting or looking up an entity into a type whose matching field is
plain fails with `xcl.ErrTypeMismatch`, and the `*xcl.TypeMismatchError`
detail names the field. There is no option to unwrap automatically; declare
the field `types.Sensitive[T]` in your type too.

```go
_, err := xcl.Find[PlainPostgres](c, "resource.postgres.main")
// entity "resource.postgres.main" field "password" is sensitive,
// main.PlainPostgres declares it as a plain value
```

### Your own JSON and templates

A sensitive value formats as `(sensitive)` everywhere Go formats it:
`fmt.Sprintf("%v")`, `%+v` and every other verb, `log/slog`,
`encoding/json` and `text/template`. So `json.Marshal(db)` writes
`"password":"(sensitive)"`. To put the real value in your own output, unwrap
it with `.Reveal()` first, knowing it is then unprotected.

### Showing real values

xcl's own output can show the real value when you ask for it explicitly:

```go
// configuration text with the real value instead of "(sensitive)"
text, err := xcl.EncodeEntity(db, xcl.RevealSensitive())

// the resource printer, in every format
printer := logger.NewResourcePrinter(logger.WithRevealSensitive(true))
```

A value read back from event data has no real value to show, so it is written
as the marker even when revealing.

## Struct Tags

To create types that can be converted from HCL your top level resource needs to embed the
following type into your structs.

``types.ResourceBase `xcl:",remain"` ``

The struct tag `` `xcl:",remain"` ``, must be included with this type as it tells the HCL parser
to unfold the default properties such as `disabled` and `depends_on` from your custom type.

### Basic Attributes

If you add the field `` Location string `xcl:"location"` `` to your type this will mean that 
the hcl attribute `location` will be parsed into this Field. This creates a required
attribute for HCL, not providing the `location` attribute on the hcl representing 
the `PostgresSQL` struct will result in a parser error.

```go
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location"`
}
```

### Optional Attributes
To create optional attribute you can add the `optional` keyword to the struct tag
the previous example has been modified to make `location` optional.

```go
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location,optional"`
}
```

### Mandatory Blocks

To define child blocks in your configuration you can specify a field that contains
another struct. In the following example the `Timeouts` field specifies that
the `Config` must be specified with a mandatory child stanza `timeouts`.

To configure a block the `block` struct tag is used after the hcl attribute
name.

```
`xcl:"timeouts,block"`
```

This can be seen in the following code sample.

```go
type Config struct {
	types.ResourceBase `xcl:",remain"`

	DBConnectionString string `xcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts Timeouts `xcl:"timeouts,block"`
}
```

This would be configured using the following HCL.

```javascript
resource "config" "myconfig" {
  db_connection_string = "abc"
  timeouts {
    tls_handshake = 10
  }
}
```

The `Timeout` type used by the field `Timeout` does not need to embed `ResourceBase`
as it is not a top level resource but all other struct tags that define blocks and
optional parameters are required.

### Optional Blocks

To make child blocks optional you simply need to change the Field type to a reference

```go
type Config struct {
	types.ResourceBase `xcl:",remain"`

	DBConnectionString string `xcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts *Timeouts `xcl:"timeouts,block"`
}
```

`timeouts` is now optional and will not result in a parser error if not 
specified.

```javascript
resource "config" "myconfig" {
  db_connection_string = "abc"
}
```

### Multiple Blocks

To allow a block to be used 0 or more times you can define the Field as a
slice.

```go
type Config struct {
	types.ResourceBase `xcl:",remain"`

	DBConnectionString string `xcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts []Timeouts `xcl:"timeouts,block"`
}
```

`timeouts` can now be specified multiple times

```javascript
resource "config" "myconfig" {
  db_connection_string = "abc"
  
  timeouts {
    tls_handshake = 10
  }
  
  timeouts {
    tls_handshake = 10
  }
}
```

Note: when parsing the configuration the order of the `Timeouts` field will correspond 
to the order of the `timeouts` blocks as defined in the `config`.

## References to other resources

A resource can reference other resources that can be set through interpolation.

The following structs define a `config` resource, and a `postgres_sql`
resource.

```go
type Config struct {
	types.ResourceBase `xcl:",remain"`

  // Other structs can be referenced by defining the type
  // to the other struct, the referenced type must implemented types.ResourceBase
	MainDBConnection PostgreSQL `xcl:"main_db_connection"`
  
  // It is also possible to reference arrays of structs 
	OtherDBConnections []PostgreSQL `xcl:"other_db_connections"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts []Timeouts `xcl:"timeouts,block"`
}

type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location,optional"`
}
```

These are represented as HCL using the following syntax, note: rather than
referencing an individual attribute from the `postgres_sql` resource the entire
struct is referenced. When the parser processes the `config` resource and the
references are resolved the value of the referenced resources are copied to
the `config`.

```javascript
resource "postgres_sql" "main" {
  location = "main.mydomain.com"
}

resource "postgres_sql" "other_1" {
  location = "1.mydomain.com"
}

resource "postgres_sql" "other_2" {
  location = "2.mydomain.com"
}

resource "config" "default" {
  main_db_connection = resource.postgres_sql.main
  other_db_connections = [
    resource.postgres_sql.other_1
    resource.postgres_sql.other_2
  ]
}
```

You could then access the properties of the referenced `PostgreSQL` structs 
in the normal go way.

```go
  conf, err := xcl.Find[Config](c, "resource.config.default")
  if err != nil {
    return err
  }

  fmt.Println("loc main", conf.MainDBConnection.Location)
  fmt.Println("loc other 1", conf.OtherDBConnections[0].Location)
  fmt.Println("loc other 2", conf.OtherDBConnections[1].Location)
```

### Defining shared fields for resources
It is common that you might have two resources that are similar but have some 
differences. For example, you might have two `database`, `postgres` and `mysql`
that share some common fields like `location` and `port` but have some differences
that are specific to the implementation.

To enable code reuse you can define a `shared` struct that contains the common fields
and then embed this struct into the `postgres` and `mysql` structs.

To enable this you define a common type that embed the `ResourceBase` type and
then you can embed this type into the `postgres` and `mysql` types.
Note: you must use the `xcl:",remain"` tag to ensure that the fields from the shared
type.

```go
type DB struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location,optional"`
	Port     int    `xcl:"port,optional"`
}

type PostgreSQL struct {
  DB `xcl:",remain"`

  MaxLocks int `xcl:"max_locks"`
}

type MySQL struct {
  DB `xcl:",remain"`

  CacheSize int `xcl:"cache_size"`
}
```

## Variables

Variables allow dynamic values to be set in your configuration, they are defined
using the `variable` resource stanza.

```javascript
variable "username" {
  default = "root"
}

variable "connection_string" {
  default = "root:password@localhost"
}
```

Setting a default value for a variable will enable it to be used within
resources.

```javascript
resource "config" "myconfig1" {
  db_connection_string = variable.connection_string 
}

resource "config" "myconfig2" {
  db_connection_string = "${variable.username}:password@localhost"
}
```

Variables can also be overridden by setting the corresponding environment
variable. For example to set the variable `username`, you prefix the environment
variable with `HCL_VAR_`, so to set username you could do set the following:

```shell
export HCL_VAR_username="nic"
```

The prefix for environment variables can be changed in the `ParserOptions`.

Note: variables can contain interpolated references for other resources as
the are not parsed by the graph and are parsed before any other resource.

For computed local variables use `local` resources.

## Local

Local resources allow you to create, temporary computed variables that can 
be used within your config. For example, if you wanted to compute a value
that was based on the attribute of another resource you could use a `local`.

```javascript
resource "config" "myconfig1" {
  db_connection_string = variable.connection_string 
}

local "conn" {
  value = resource.config.myconfig1.db_connection_string == "abc" ? "localhost" : resource.config.myconfig1.db_connection_string
}

resource "config" "myconfig2" {
  db_connection_string = local.conn
}
```

Unlike variables `local` variables are part of the graph and can contain references
to other resources.

## Modules

HCLConfig supports modular configuration that enables you to group your configuration or encapsulate certain
functionality into modules.

A module is a default type, however you still need to create the go structs that 
define the resources included in your module. The following example shows how you can
use the module that is defined in
[./example/plugin/config/modules/db/db.xcl](./example/plugin/config/modules/db/db.xcl)

Any sub folder can be a module, to create a module all that is needed is one or more `.hcl` files
that contain your custom resources.

```javascript
// modules can also use 
module "mymodule_1" {
  source = "../example/modules/db"

  variables = {
    db_username = "root"
    db_password = "password"
  }
}
```

Modules can also be imported from remote sources such as a GitHub repository, to version
a module the SHA of the commit can be used.

```javascript
module "mymodule_1" {
  source = "github.com/jumppad-labs/xcl?ref=9173050/example/modules//db"

  variables = {
    db_username = variable.db_username
    # the module assigns it to a field declared types.Sensitive[string]
    db_password = env("DB_PASSWORD")
  }
}
```

### Inputs

To enable dynamic module use, `variables` and `outputs` can be used to define
the interface for your module. Variables can be define inside the module and
the value set explicitly using the `variables` block as shown in the previous
example.

### Outputs

To return a value from a module you can define an `output`, the `db` module
defines the output `connection_string`. 

```javascript
output "connection_string" {
  value = resource.postgres.mydb.connection_string
}
```

To read this value you can use the interpolation syntax `module.mymodule_1.output.name`
The following example shows how an output from one module can be used as an
input to another module. Because HCLConfig understands the links between resources
the resources in `my_other_module` will only be processed after the resources
in `mymodule_1`.

```javascript
module "mymodule_1" {
  source = "../example/modules/db"

  variables = {
    db_username = "root"
    db_password = "password"
  }
}

module "my_other_module" {
  source = "../example/modules/app"

  variables = {
    db_connection_string = module.mymodule_1.output.connection_string
  }
}
```

Outputs can also contain complex types like lists ...

```javascript
output "connection_string_list" {
  value = [
    resource.postgres.mydb1.connection_string,
    resource.postgres.mydb2.connection_string
  ]
}
```

and maps ...

```javascript
output "connection_string_map" {
  value = {
    connection1 = resource.postgres.mydb1.connection_string
    connection2 = resource.postgres.mydb2.connection_string
  }
}
```

It is possible to consume these values like so ...

```javascript
output "connection_string_list_1" {
  value = output.connection_string_list.0
}

output "connection_string_map_1" {
  value = output.connection_string_map.connection1
}
```

### The module boundary

A module's outputs are the only way to reach inside it. From its parent you can
reference a module's outputs, `module.<name>.output.<name>`, and the module
itself, as in `depends_on = ["module.mymodule_1"]`, but nothing else. A
reference to a module's resources, variables or nested modules, or to anything
inside a module nested in it, fails validation with an error naming the
reference:

```
resource 'output.x' refers to 'module.a.resource.postgres.db.connection_string', which is inside module 'a'; only a module's outputs can be referenced from outside it
```

The boundary holds at every level of nesting: a parent reaches only its direct
children's outputs. To expose a value from a module nested further down, each
module in between re-exports it as one of its own outputs. Here module `a` uses
module `b` and re-exports `b`'s output, so the root can read it:

```javascript
// b/b.xcl
output "value" {
  value = "from-b"
}

// a/a.xcl
module "b" {
  source = "./b"
}

output "from_b" {
  value = module.b.output.value
}

// main.xcl
module "a" {
  source = "./a"
}

output "deep" {
  value = module.a.output.from_b // module.a.b.output.value would be rejected
}
```

The boundary applies to references written in configuration. Application code
can still look up any entity inside a module by its full address with `Find`.

## Functions

HCLConfig supports functions that can be used inside your configuration


```javascript
postgres "mydb" {
  location = "localhost"
  port = 5432
  name = "mydatabase"

  username = var.db_username

  // functions can be used inside the configuration,
  // functions are evaluated when the configuration is parsed 
  password = env("DB_PASSWORD")
}
```
### Default functions

For convenience HCLConfig has the following default functions:

#### len(type)

Returns the length of a string or collection

```javascript
mytype "test" {
  collection = ["one", "two"]
  string = "mystring"
}

myothertype "test" {
  // Value = 2
  collection_length = len(resource.mytype.test.collection)

  // Value = 8
  string_length = len(resource.mytype.test.string)
}
```

#### env(name)

Returns the value of a system environment variable

```javascript
mytype "test" {
  // returns the value of the system environment variable $GOPATH
  gopath = env("GOPATH")
}
```

#### home()

Returns the location of the users home directory

```javascript
mytype "test" {
  // returns the value of the system home directory
  home_folder = home()
}
```

#### file(path)

Returns the contents of a file at the given path.

```javascript

# given the file "./myfile.txt" with the contents "foo bar"

mytype "test" {
  // my_file = "foobar"
  my_file = file("./myfile.txt")
}
```

#### template_file(path, variables)

Returns the rendered contents of a template file at the given path with the given input variables.

Templates can leverage the Handlebars templating language, more details on Handlebars
can be found at the following link:

[https://handlebarsjs.com/](https://handlebarsjs.com/)

```javascript
#given a file "./mytemplate.tmpl" with the contents "hello {{name}}"

mytype "test" {
  // my_file = "foobar"
  my_file = template_file("./mytemplate.tmpl", {
    name = "world"
  })
}
```

##### Template Helpers

The template_file function provides helpers that can be used inside your 
templates as shown in the example below.

```javascript
resource "template" "consul_config" {

  source = <<-EOF

  file_content = "{{ file "./myfile.txt" }}"
  quote = {{quote something}} 
  trim = {{quote (trim with_whitespace)}}

  EOF

  destination = "./consul_config/consul.hcl"
}
```

###### quote [string]

Returns the original string wrapped in quotations, quote can be used with 
the Go template pipe modifier.

```go
// given the string abc

quote "abc" // would return the value "abc"
```

###### trim [string]

Removes whitespace such as carrige returns and spaces from the begining and 
the end of the string, can be used with the Go template pipe modifier.

```go
// given the string abc

trim " abc " // would return the value "abc"
```

#### dir()

Returns the absolute path of the directory containing the current resource

```javascript
mytype "test" {
  resource_folder = dir()
}
```

#### trim(string)

Returns the given string with leading and trailing whitespace removed
of the given string

```javascript
mytype "test" {
  // trimmed = "abc 123"
  trimmed = trim("  abc  123   ")
}
```

#### element(list | map, int | string)
Returns a value from a map or list by the given index. 

```javascript
variable "property" {
  default = "name"
}

mytype "test1" {
  // trimmed = "abc 123"
  item {
    name = "nic"
  }
  
  item {
    name = "eric"
  }
}

mytype "test2" {
  item {
    name = element(resource.mytype.test1.0, variable property)
  }
}
```

### Custom Functions

In addition to the default functions it is possible to register custom functions.

For example, given a requirement to have a function that returns a random number in a set
range you could write a go function that looks like the following. Note: only a single
return type can be consumed by the HCL parser and assigned to the resource value.

```go
func RandRange(min, max int) int {
	return rand.Intn((max-min)+1) + min
}
```

This could then be referenced in the following config

```javascript
postgres "mydb" {
  location = "localhost"

  // custom function to return a random number between 5000 and 6000
  port = rand(5000,6000)
  
  name = "mydatabase"
}
```

You set up the parser as normal

```go
p := NewParser(DefaultOptions())
p.RegisterType(&structs.Postgres{}, "resource", "postgres")
```

However, in order to use the custom function before parsing you register it with the 
`RegisterFunction` method as shown below.

```go
p.RegisterFunction("rand", RandRange)
```

At present only the following simple types are supported for custom functions

* string
* uint
* uint32
* uint64
* int
* int32
* int64
* float32
* float64

#### Errors in custom functions
To signify that an error occurred in a custom function and to halt parsing of the
config your function can optionally return a tuple of (type, error). For example
to add error handling to the random function you could write it as shown below.

```go
func RandRange(min, max int) (int, error) {
  if min >= max {
    return -1, fmt.Errorf("minimum value '%d' must be smaller than the maximum value '%d')
  }

	return rand.Intn((max-min)+1) + min
}
```

## Lifecycle Callbacks

HCLConfig provides three hooks that can be used when parsing configuration.

* Resource `Processable` interface
* Parser Callback
* Config Process Callback

### Resource Processable interface

The resource `Processable` interface can be added to your resources by adding
a an optional method with the following singature.

```go
Process() error
```

For example, the `PostgresSQL` resource implement the `Processable` interface
to compute the value of the attribute `connection_string`.

```go
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location"`
	Port     int    `xcl:"port"`
	DBName   string `xcl:"name"`
	Username string `xcl:"username"`
	Password string `xcl:"password"`

	// ConnectionString is a computed field and must be marked optional
	ConnectionString string `xcl:"connection_string,optional"`
}

// Process is called using an order calculated from the dependency graph
// this is where you can set any computed fields
func (t *PostgreSQL) Process() error {
	t.ConnectionString = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", t.Username, t.Password, t.Location, t.Port, t.DBName)
	return nil
}
```

`Process` is called in strict order depending on the dependencies for your resources.

For example, given the following custom resources

```go
// Config defines the type `config`
type Config struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	ID string `xcl:"id"`

	DBConnectionString string `xcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts *Timeouts `xcl:"timeouts,block"`
}

func (t *Config) Process() error {
	// override default values
	if t.Timeouts.TLSHandshake == 0 {
		t.Timeouts.TLSHandshake = 5
	}

	return nil
}

// PostgreSQL defines the Resource `postgres`
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location"`
	Port     int    `xcl:"port"`
	DBName   string `xcl:"name"`
	Username string `xcl:"username"`
	Password string `xcl:"password"`

	// ConnectionString is a computed field and must be marked optional
	ConnectionString string `xcl:"connection_string,optional"`
}

// Process is called using an order calculated from the dependency graph
// this is where you can set any computed fields
func (t *PostgreSQL) Process() error {
	t.ConnectionString = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", t.Username, t.Password, t.Location, t.Port, t.DBName)
	return nil
}
```

And the following configuration that uses these resources

```javascript
resource "config" "myconfig" {
  // resource.postgres.mydb.connection_string will be available after the `Process` has
  // been called on the `postgres` resource. HCLConfig understands dependency and will
  // call Process in a strict order
  db_connection_string = resource.postgres.mydb.connection_string
}

resource "postgres" "mydb" {
  location = "localhost"
  port     = 5432
  name     = "mydatabase"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password
}
```

Because you are referencing the attribute `resource.postgres.mydb.connection_string`
to set a value in the `config` resource. `Process` for the `PostgreSQL` type will be called
before `Process` for the `Config` type. This allows you to perform any computations or validations
needed to calculate `connection_string` before `config` attempts to consume the value.

Returning an `error` from `Process` will immediately exit the `ParseFile` or `ParseDirectory`
method.

### Parser Callback

Rather than implementing individual resource functions you may prefer to leverage the global
callback that can be set on the `ParserOptions`.

```go
o := xcl.DefaultOptions()

// set the callback that will be executed when a resource has been created
// this function can be used to execute any external work required for the
// resource.
o.ParseCallback = func(r types.Resource) error {
	fmt.Printf(
    "resource '%s' named '%s' has been parsed from the file: %s\n", 
    r.Metadata().Type, 
    r.Metadata().Name, 
    r.Metadata().File,
  )

  // cast the Resource into a concrete type
  switch r.Metadata().Type {
    case "config":
      myconfig := r.(*Config)
      fmt.Println(myconfig.DBConnectionString)
  }

	return nil
}
```

The `ParseCallback` function is executed `after` the `Processable` interface
and respects the same call order that is implemented for `Processable`.

### Config `Process` function

A final callback is available using the `Process(wf ProcessCallback, reverse bool) error`
function that is available on the `xcl.Config` type.

`Process` builds a Directed Acyclic Graph for your configuration based on
the dependency and calls the provided `ProcessCallback` for each resource 
in the graph.

```go
nc, _ := p.ParseFile("./config.hcl")

nc.Process(func(r types.Resource) error {
	fmt.Println("  ", r.Metadata().ID)
	return nil
}, false)
```

**Note**  
While you can mutate the values of the `Resource` passed to the
ProcessCallback, it will not update any resources that reference this attribute.

When ParseFile resolves interpolated values it `copies` the value to the destination
resource. Given the earlier example mutating the `ConnectionString` field on the 
`postgres` resource would not update the `config` resource even though the  `ProcessCallback`
will be called with the `PostgreSQL` type before `Config`.

### Walking dependencies in reverse

To reverse the order of resources that are provided to the `ProcessCallback` you can
set the second process method attribute to `true`.

```go
nc, _ := p.ParseFile("./config.hcl")

nc.Process(func(r types.Resource) error {
	fmt.Println("  ", r.Metadata().ID)
	return nil
}, true)
```

`ProcessCallback` will be called first for resources lowest down in the dependency
graph `children` before the resources they depend on.

An ideal use for this method is to clean up any operations that may have been created
with the `Processable` interface on your resource or the `ParseCallback`.

## Serialization

xcl saves state itself, through the state store given to `WithStateStore`, see
[docs/state.md](./docs/state.md).

To turn a single entity back into configuration text, see
[Converting to configuration text](#converting-to-configuration-text) above.

