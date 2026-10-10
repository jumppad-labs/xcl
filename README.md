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


## Quick start

### Configuration only

An application that only reads its configuration declares each Go type, on a
local registry, under the block type name it is written with, applies the
configuration and reads it back. No plugin and no provider is needed:

```go
local := registry.NewLocal()
local.RegisterType(&resources.Deployment{}, "deployment") // deployment "api" {}
local.RegisterType(&resources.Service{}, "service")       // service "api" {}
local.RegisterType(&PostgreSQL{}, "resource", "postgres") // resource "postgres" "main" {}

c, err := xcl.NewConfig(xcl.WithRegistry(local))
if err != nil {
	return err
}

if err := c.Apply("./config"); err != nil {
	return err
}

var cfg appConfig
err = c.Decode(&cfg)
```

**No state.** Without `WithStatePath` or `WithStateStore` nothing is
persisted: the applied configuration is held in memory only. This is the
supported mode for configuration-only use, see
[State and Destroy](#state-and-destroy) to keep state between runs.

### With plugins

Plugins come from registries, the same place Go types are declared. The local
registry holds plugins compiled into the program, plugin binaries named by path, and directories searched for
plugin binaries. Registering only records a plugin, nothing is started until
the first operation needs it:

```go
local := registry.NewLocal()
local.RegisterPlugin(&template.TemplatePlugin{})        // an in-process plugin
local.RegisterExternalPlugin("./bin/xcl-plugin-docker") // an external plugin binary
local.RegisterPluginDirectory("~/.xcl/plugins")         // a directory to search

c, err := xcl.NewConfig(
	xcl.WithRegistry(local),
	xcl.WithStatePath("./.xcl"),
)
```

`WithRegistry` may be given more than once. Registries load in the order they
were given, and the plugins of each in the order they were registered.
One registry can hold Go types and plugins together. `WithRegistry` is the
one way a `Config` gets anything beyond the builtins, and the registry the one
place types and plugins are registered.

A plugin published as GitHub releases installs with one line from the GitHub
registry, which downloads, verifies and caches the build for the current
platform, see [Installing plugins from GitHub](#installing-plugins-from-github):

```go
gh := registry.NewGitHub()
gh.RegisterPlugin("jumppad-labs/xcl-plugin-docker", "v1.2.0")
```

**Writing a plugin.** Start from the
[plugin template](https://github.com/jumppad-labs/xcl-plugin-template), a
GitHub template repository with one working resource that runs both
in-process and as a separate program, tests against strict doubles and
publishes signed releases the GitHub registry installs. It follows the
standard [plugin layout](docs/plugin-layout.md) every xcl plugin uses.

**Registration problems.** Registration returns no errors. Problems are
reported in one of three places:

- A mistake in a single call, wrong on every run, panics straight away
  naming the type: an empty name, more than one subtype or an empty subtype,
  a prototype that is not a pointer to a struct embedding
  `types.ResourceBase`, or a nil plugin or registry.
- Declared Go types that do not fit together are an error returned by
  `NewConfig`: the same block type declared twice, in one registry or across
  two, or with a builtin name, is a `*xcl.TypeNameClashError` naming both Go
  types and their registries, and a type keyword used both with and without a
  subtype is a `*xcl.TypeFormError`.
- A problem with the environment is returned by the first `Validate`,
  `Apply`, `Destroy`, `Diff` or `Load`: any missing or failing plugin,
  including one found in a plugin directory, as a `*xcl.PluginLoadError`
  (matching `xcl.ErrPluginLoad`), and a block type a plugin provides that a
  declared type or another plugin already provides as a
  `*xcl.TypeNameClashError` naming both providers and their registries.

## Example

The [`example`](./example) directory holds two self-contained programs, each
a Go module of its own with its own configuration and Go types.

### Configuration only

[`example/configonly`](./example/configonly) uses XCL for what it is most
often needed for: an application reading its configuration into its own Go
types and acting on it. The block types
([`configonly/resources`](./example/configonly/resources)) are plain Go types
declared on a local registry with `RegisterType`, with no plugin and no
provider.

Its configuration ([`configonly/config`](./example/configonly/config)) is a
small Kubernetes-like deployment split across three files, `deployment.xcl`,
`ingress.xcl` and `secret.xcl`, declaring a config map, a secret, a
deployment, a service and an ingress. That shape needs everything a
configuration language is asked for:

- **blocks nested inside blocks** — `resources` inside `container`, holding
  `limits` and `requests` of its own
- **repeated blocks** — two `container` blocks, each with its own `port` and
  `env` blocks, decoded into a Go slice; a block that appears at most once is
  a pointer, and is `nil` when it is left out
- **links between resources** — the `service` names the `deployment` by id
  and reads its target port out of it
  (`deployment.api.container[0].port[0].container_port`), the `ingress` names
  the `service` the same way, and the container's environment is read out of
  a `config_map`'s map attribute (`config_map.api.data.db_host`). Kubernetes
  matches a service to its pods with a label selector because a manifest can
  not point at another object; a reference does it directly.

```hcl
deployment "api" {
  replicas = variable.replicas

  # container is a repeated block, this deployment has two of them
  container {
    name  = "api"
    image = "ghcr.io/example/api:${variable.image_tag}"

    # port is repeated in turn, a block nested inside a block
    port {
      name           = "http"
      container_port = 8080
    }
```

The program loads the configuration with `loadConfig`, which declares the
block types, applies the configuration and gathers every block it reads into
one struct of its own with a single call, rather than looking each type up
separately (see [Filling a struct of your own](#filling-a-struct-of-your-own)):

```go
type appConfig struct {
	Deployments []*resources.Deployment
	Services    []*resources.Service
	Ingresses   []*resources.Ingress
}
```

```go
	var cfg appConfig
	if err := c.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("reading configuration from %s: %w", dir, err)
	}
```

Then `ingressRoutes` follows the links: for every path of every ingress it
looks up the service the rule names, the deployment that service names, and
the container port that listens on the service's target port. It prints one
line per route, and a link that can not be followed is an error naming the
ingress and path:

```
api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)
```

Its tests are the ones an application author writes for configuration-driven
code: `ingressRoutes` is checked against test configurations under
[`configonly/testdata`](./example/configonly/testdata), loaded through the same
`loadConfig`, and a smoke test builds and runs the program. The program keeps
no state: without a state option xcl holds the applied configuration in
memory only, which is all a program that only reads its configuration needs.

### Plugins

[`example/plugin`](./example/plugin) applies its configuration
([`plugin/config`](./example/plugin/config)) through two plugins that do real
work, each laid out the way a plugin author would lay out their own project:

- The Docker plugin
  ([`plugin/plugins/docker`](./example/plugin/plugins/docker)) is an external
  plugin and a Go module of its own, laid out in the standard plugin layout
  (see its [README](./example/plugin/plugins/docker/README.md)). Its
  importable `Plugin` type is in the module's root package, and
  [`plugin/plugins/docker/cmd/docker`](./example/plugin/plugins/docker/cmd/docker)
  serves it as a standalone program that xcl starts as a separate process and
  calls over gRPC. It provides `docker "network"` and `docker "container"`,
  whose block types are in
  [`plugin/plugins/docker/entities`](./example/plugin/plugins/docker/entities)
  and whose providers are in
  [`plugin/plugins/docker/providers`](./example/plugin/plugins/docker/providers),
  and creates real Docker networks and containers. The Docker libraries are
  imported only under `client/`: a narrow `Docker` interface over the Docker
  SDK in
  [`plugin/plugins/docker/client/docker`](./example/plugin/plugins/docker/client/docker),
  and the container task layer the providers use in
  [`plugin/plugins/docker/client/containers`](./example/plugin/plugins/docker/client/containers),
  each with a Mockery mock beside it in `mocks/` that the unit tests use. The
  plugin also carries a sample configuration
  ([`examples/basic`](./example/plugin/plugins/docker/examples/basic)) and
  end-to-end tests ([`e2e`](./example/plugin/plugins/docker/e2e)). The
  application reads what it applied through the entity types alone, so its
  build includes neither the providers nor the Docker libraries.
- The template plugin
  ([`plugin/plugins/template`](./example/plugin/plugins/template)) is an
  in-process plugin, compiled into the program. It provides `template`, a
  block type with no subtype written `template "welcome" {}`, and renders a
  Handlebars template to a file, with the permissions in its optional `mode`
  (an octal string, `"0644"` when unset).

A plugin registers each block type with its own call to
`plugins.RegisterResourceProvider` in `Init`, passing an empty subtype for a
type like `template`. The container's `ip_address` is computed by the Docker
plugin when it creates the container, and the template's `variables` read it,
so a value moves from the external plugin to the in-process one. Values move
the other way too: the `init` template renders a script with `mode = "0755"`,
and the container's `init_script` reads its destination and mounts the file
read-only at `/docker-entrypoint.d/90-xcl-init.sh`, where nginx runs it when
the container starts.

The container's provider decides and acts from what xcl tells it, the
settings that changed and the dependencies the same apply updates or
replaces. Its `Changed` answers replace for a change within `image`,
`command`, `environment` or `init_script`, which Docker fixes when it creates
a container, or when a dependency that is not a Docker network is replaced,
such as the `init` template. A change to its networks, or a network that is
updated or replaced, answers update, and a dependency that is only updated,
such as the `init` template rendering new content, leaves it alone. Its
`Update` hot swaps the running container's networks: it rebuilds the previous
attachments by putting back each network change's `Before` value,
disconnects the networks that went away or whose aliases changed, connects
the new ones, reconnects those whose network was replaced, and reads the
container's new address. The network's `Destroy` force-disconnects every
container still attached before it removes the network, so a network can be
rebuilt under a running container.

Their providers log from each lifecycle method with
`plugins.Logger(ctx).Info("created network", "name", n.Meta.Name, "id", n.DockerID)`,
passing no resource details. xcl binds that logger to the resource, its type,
the file it was declared in and the step, and names the plugin as the
message's source, so the in-process and the external plugin read the same in
the output:
`INFO created network source=docker-plugin operation=create phase=log resource=docker.network.app ...`.

Running it needs a Docker engine, reached through `DOCKER_HOST` or the default
socket. Its tests do not: the provider unit tests use the mock or a temporary
directory, and the tests that need a real engine skip when none answers.

### Running them

Every example applies the configuration and prints what it read. The
configuration-only example keeps no state; the plugin example is a command line tool, `xcl-docker`, with
`apply <path>`, `plan <path>`, `status`, `inspect <address>` and `destroy`
commands, each a separate run sharing the state saved in `./.xcl-docker`.
`apply` and `destroy` print nothing of their own. `plan` compares the saved
state with the configuration at path using `Diff`, changes nothing, and prints
what an apply would do with `diff.Render`, highlighted on a terminal. `status` reads the state back with
`Load` and prints it as a tree drawn with
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and `inspect` prints
one resource as highlighted configuration text with `EncodeEntity`. Each
example sends everything xcl reports, lifecycle events, plugin log messages
and errors, to the shared [`example/prettylog`](./example/prettylog)
receiver, set up in one line, which writes styled lines to standard error. It
shows info and above; set `XCL_LOG_LEVEL=debug` to see plugin loading and
`Init` messages too. The program's own output goes to standard output. The
plugin example's `plan`, `status` and `inspect` are the exception: they
print only their output, so they give xcl no receiver.

Run any of them from its directory with `make run`. For `plugin` this builds
`xcl-docker` and the Docker plugin side by side into `build/`, then runs
`apply ./config`, `status` and `destroy` in turn, and it has the extra Makefile
targets `build`, `replace`, `swap`, `rebuild-init`, `remove-network`,
`generate` and `clean`; every example has `run` and `test`. The plugin tests
build both binaries themselves.

`./config` is the plugin example's whole configuration and applies on its
own. Next to it, [`plugin/config-subnet`](./example/plugin/config-subnet) is
the same configuration with the network's address range widened from
`10.42.0.0/24` to `10.42.0.0/23`. Docker cannot move a network to a new range
in place, so the network's provider answers replace when its `subnet`
changes. The container is not replaced with it: it is told the network is
replaced and answers update, and its `Update` reattaches the running
container to the new network, keeping its ID. `make replace` applies
`./config`, then plans and applies `./config-subnet`, prints the state and
destroys everything. The plan shows the replacement as `-/+` and the
container's update with the dependency behind it:

```
  # docker.container.web will be updated because docker.network.app is replaced
  ~ docker "container" "web" {}

  # docker.network.app will be replaced, it cannot be updated in place
-/+ docker "network" "app" {
      ~ subnet = "10.42.0.0/24" -> "10.42.0.0/23"
    }

  # template.welcome will be updated
  ~ template "welcome" {
      ~ variables["address"] = "10.42.0.2" -> (known after apply)
    }

Diff: 0 to create, 2 to update, 1 to replace, 0 to delete, 1 unchanged.
```

The container's configuration is unchanged; it is updated only because its
network is replaced. The template reads the container's address, which is
only known once the container is on the new network, so it is updated; the
`init` template, which reads only the network's name, is unchanged. Applying
`./config-subnet` destroys the old network, which detaches the container,
creates the network on `10.42.0.0/23`, connects the same container to it and
updates the template with the container's new address.

Three more variants each have a Makefile target that applies `./config`,
then plans and applies the variant, prints the state and destroys
everything:

- `make swap` applies [`plugin/config-swap`](./example/plugin/config-swap),
  which adds a `backend` network and moves the container to it. The container
  is updated in place, disconnected from `app` and connected to `backend`
  with the same ID: `Diff: 1 to create, 2 to update, 0 to replace, 0 to
  delete, 2 unchanged.`
- `make rebuild-init` applies [`plugin/config-init`](./example/plugin/config-init),
  which moves the init script's destination. The template cannot move its
  file in place, so it is replaced, and the container that mounts the script
  is replaced because it is: `Diff: 0 to create, 1 to update, 2 to replace,
  0 to delete, 1 unchanged.` Editing only the script's content, as
  [`plugin/config-init-content`](./example/plugin/config-init-content) does,
  renders the file again and leaves the container alone.
- `make remove-network` applies
  [`plugin/config-remove`](./example/plugin/config-remove), which removes the
  network and every reference to it. The network is deleted, detaching the
  container, which is updated in place and keeps running with no network:
  `Diff: 0 to create, 3 to update, 0 to replace, 1 to delete, 0 unchanged.`
  Removing only the network block and leaving references to it, as
  [`plugin/config-dangling`](./example/plugin/config-dangling) does, is
  rejected by validation before anything changes.

Each example is a Go module of its own, pointed at this checkout with a
`replace` directive, so it can be copied out of the repository: drop the
`replace`, require a published xcl version and it builds on its own. Run an
example's tests with `go test ./...` (or `make test`) in its directory. xcl's
own `go test ./...` runs every example's tests too, through the end-to-end
suite in [`e2e/`](./e2e), which also holds xcl's end-to-end tests of the
library itself and a [coverage map](./e2e/COVERAGE.md) of what they cover.

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
destroyed, declare its Go type on a registry with `RegisterType` and give the
registry to the `Config`. No plugin or provider is needed.

Everything a configuration declares is an entity, and an entity has a type
and an optional subtype. The type is the keyword a block leads with, and the
subtype, when there is one, is its first label:

```go
local := registry.NewLocal()

// a type with a subtype, declared: server "big" "web" {}, addressed server.big.web
local.RegisterType(&Server{}, "server", "big")

// a type without one, declared: cache "main" {}, addressed cache.main
local.RegisterType(&Cache{}, "cache")

// resource is a type like any other: resource "postgres" "main" {}
local.RegisterType(&PostgreSQL{}, "resource", "postgres")

c, err := xcl.NewConfig(xcl.WithRegistry(local))
if err != nil {
	return err
}
err = c.Apply("./config")
```

Declared blocks are decoded into your own Go type, take part in references
(`server.big.web.location`, `cache.main.location`) and dependency ordering,
work in modules and when disabled, and are saved to state when there is a
state store. They are never passed to a provider. `RegisterType` takes a
pointer to a struct that embeds `types.ResourceBase`.

A type keyword takes a subtype for every declaration or for none, so an
address can always be read by position. `resource` always takes one.
Declaring `server` without a subtype after declaring it with one, or the
other way round, is a `*xcl.TypeFormError` returned by `NewConfig`; a plugin
type that does so fails the first operation with a `*xcl.TypeFormError`.

Every type and subtype must be unique across builtin blocks (`variable`,
`output`, `module`, `root`), declared types and plugin types.
`server` and `resource "server"` are different types and do not clash.
Declaring a type and subtype a builtin or another declared type already has,
in the same registry or another, is a `*xcl.TypeNameClashError` returned by
`NewConfig`, naming it, i.e. `resource.postgres`, and both Go types and their
registries. A clash with a type a plugin provides is reported when the plugins
load, see below.

### Registering plugins

Plugins come only from registries, added with `xcl.WithRegistry`. The local
registry, from the [`registry`](./registry) package, needs no logger, and
registering a plugin only records it:

```go
local := registry.NewLocal()
local.RegisterPlugin(&MyPlugin{})                  // an in-process plugin
local.RegisterExternalPlugin("./bin/xcl-plugin-x") // an external plugin binary
local.RegisterPluginDirectory("~/.xcl/plugins")    // a directory to search

c, err := xcl.NewConfig(xcl.WithRegistry(local))
```

A directory is searched for executables named `xcl-plugin-*`; give another
pattern when the registry is created, `registry.NewLocal(registry.PluginPattern("acme-plugin-*"))`.
A directory that does not exist provides no plugins. `WithRegistry` may be
given more than once: registries load in the order they were given, and each
registry's plugins in the order they were registered. The `registry` package
also defines the `Registry` and `Plugin` interfaces, with `registry.InProcess`
and `registry.Executable` to start a plugin, for writing a registry of your
own.

Plugins load when they are first needed, at the start of the first
`Validate`, `Apply`, `Destroy`, `Diff` or `Load`, and only once per `Config`.
Directory searches and loading are reported as `discover` and `load` events;
a `load` event's `Meta` names the `plugin`, its `registry` and, on success,
the `block_types` it provides, and a `discover` event's names the `dirs`
searched, the `registry` and, on success, the `count` found. Any plugin that
fails to load, a path that does not exist or a discovered binary that is not
a plugin alike, fails that first operation with a `*xcl.PluginLoadError`
naming the `Plugin` and its `Registry`, which matches `xcl.ErrPluginLoad`.

A block type provided twice anywhere, by two plugins in one registry, by
plugins in two registries, or by a plugin and a declared type or a builtin,
fails the load with a `*xcl.TypeNameClashError`. There is no precedence: the
error names the `Name`, the `Provider` and its `Registry`, and what already
provides it, `Existing` and `ExistingRegistry`, whatever order they were
registered in.

An external plugin's process runs only while an operation is using it: xcl
starts it for each `Validate`, `Apply`, `Destroy` or `Load` and stops it when
the operation is done, so a program never stops a plugin itself and nothing
is left running between operations. The process starts afresh each time, so
the plugin's `Init` runs again and it keeps nothing in memory from one
operation to the next. In-process plugins load once and stay loaded.

### Installing plugins from GitHub

A plugin published as GitHub releases, laid out as the plugin template
publishes them, installs with one line. The GitHub registry downloads the release's build
for the platform the program runs on, checks it against the release's
checksums, keeps it in a cache and starts it as an external plugin:

```go
gh := registry.NewGitHub()
gh.RegisterPlugin("jumppad-labs/xcl-plugin-docker", "v1.2.0")

c, err := xcl.NewConfig(xcl.WithRegistry(local), xcl.WithRegistry(gh))
```

The plugin is named after the repository, `xcl-plugin-docker`, and is
installed when plugins load, not when it is registered. A GitHub registry
wraps a local one, so `RegisterType` works on it too, and it can be given
beside a local registry or any other.

- **Pin an exact version.** The version is one exact release tag,
  `v<major>.<minor>.<patch>` with an optional pre-release suffix such as
  `v1.2.0-rc.1`. An empty version, a range such as `~1.2`, `latest`, a tag
  without its `v` or a repository that is not `owner/repo` is a programmer
  error and panics at `RegisterPlugin`, naming the plugin, before anything is
  downloaded. Draft releases are never installed.
- **Trust signing keys.** `registry.GitHubTrustedKeys(armored...)` takes
  ASCII-armoured OpenPGP public keys. When any are given, a plugin is used
  only when its release's checksums file is signed by one of them; a release
  signed by another key, or not signed, is refused. Without keys, signatures
  are not checked and unsigned releases install. A key that cannot be read
  fails the load against the `github.com` registry.
- **Private repositories.** Requests carry a GitHub token when there is one:
  `registry.GitHubToken(token)`, otherwise `GITHUB_TOKEN`, then `GH_TOKEN`.
  Public repositories need none. Without a token, a private repository's
  release is reported as not found, saying it may need a token.
- **The cache.** Plugins are kept under `~/.xcl/cache/plugins`, as
  `github.com/<owner>/<repo>/<version>/<os>_<arch>/`, or under the directory
  given with `registry.GitHubCacheDir(dir)`. Each entry holds the release's
  archive, its checksums file, its signature when it has one, and the
  extracted binary. A version already in the cache is used without contacting
  GitHub, so applies work offline, and it is verified again, against the
  checksums and the application's current trusted keys, every time the plugin
  is started. An entry is written in one step, so a failed or interrupted
  download leaves nothing usable. Old versions are never removed; delete them
  by hand.

A plugin that cannot be installed fails the first operation with a
`*xcl.PluginLoadError` naming the plugin and the `github.com` registry. It
wraps a `*xcl.PluginInstallError` naming the `Repository`, `Version` and
`Platform`, which matches `xcl.ErrPluginNotFound` when the release, or its
build for this platform, does not exist (or the repository is private and no
token was given), and `xcl.ErrPluginVerification` when the release has no
checksums file, a file does not match its checksum, or a signature is missing
or untrusted. The release layout the registry installs, archive, checksums
and signature names, is described in [docs/plugins.md](./docs/plugins.md).

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

Keep state between runs with a `StateStore`. Without `WithStatePath` or
`WithStateStore` nothing is persisted, which is the supported mode for
configuration-only use. `Apply` loads the saved state,
applies the configuration and saves the result. `Destroy` needs no
configuration: it destroys everything in the saved state, dependents before
what they depend on. `Load` reads the saved state back without changing
anything.

```go
c, err := xcl.NewConfig(
	xcl.WithRegistry(local),
	// keeps state in ./.xcl/state.json, creating the directory and file if needed
	xcl.WithStatePath("./.xcl"),
)

err = c.Apply("./config")

// later, in another run, read back what was applied without applying it
err = c.Load()

// or remove everything that was applied
err = c.Destroy()
```

`Load` reads the saved state into the `Config`, so `Find`, `FindByType` and
the other lookups answer from what was last saved. Like `Destroy` it needs no
configuration, and it calls no provider. It loads the plugins first, as a
saved plugin type is read back through its plugin's schema, and reports
itself with `load_state` events. A program that applies in one run and
reports in another, a `status` command, calls it before looking anything up.

The state is saved after each resource is destroyed, so an interrupted
`Destroy` picks up where it stopped. A resource whose destroy fails stays in
the state, with everything it depends on, and is named in the error; running
`Destroy` again retries it. Registered and builtin types never reach a
provider, they are just removed from the state.

A block removed from the configuration is destroyed on the next `Apply`,
before anything is created or changed. Applying a configuration with no
blocks fails with `xcl.ErrEmptyConfiguration` and changes nothing, use
`Destroy` to remove everything.

Declare every type and register every plugin when the `Config` is created:
saved resources of a type the `Config` does not know fail the load with
`state.UnknownTypesError` rather than being dropped. Config loads the plugins
before it loads state.

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
	xcl.WithRegistry(local),
	xcl.WithEventHandler(events.SlogHandler(slog.Default())),
)
```

Every event is an `xcl.Event` (the same type as `events.Event`), one flat
shape for all of them:

| Field | Meaning |
|---|---|
| `Time` | when it happened |
| `Source` | `core` for xcl itself, otherwise the plugin's name |
| `Operation` | `validate`, `apply`, `destroy`, `load_state`, `parse`, `create`, `read`, `changed`, `update`, `discover`, `load`, or `events` for a blocked announcement |
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

`EventDataProcessed` is the record state stores, so it goes straight to
`EncodeSavedEntity` below. This is how the examples show each resource as it is
created. Sensitive values in `Data` are masked, by default as
`{"xcl_masked":"redact","value":"(sensitive)"}`, so processed data matches
the state record only where state and events mask alike; see
[Masking sensitive values in events](#masking-sensitive-values-in-events).

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
state stores it and events carry it at `EventDataProcessed`. It is a method on
the `Config`, whose declared types and plugins are what type the record, and
loads its plugins if they are not loaded already:

```go
text, err := c.EncodeSavedEntity(event.Data)
```

Inside an event handler, `event.Entity()` returns the event's `Data` as the
registered Go type, with every sensitive value masked, and `nil, nil` when the
event carries no data. The shipped receiver in
[`example/prettylog`](./example/prettylog) uses it to write each resource as
configuration text, and is set up with `prettylog.Handler(os.Stderr, level)`.

Both write exactly one block, so convert several entities by calling once for
each.

**What is left out.** The text shows what a person wrote. xcl's own bookkeeping
is not written. A `depends_on` list is written exactly as you wrote it, and
left out when you wrote none: the dependencies xcl works out from references
are never added to it, though they still order creation and destruction.
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

**Showing references as written.** By default a field that referred to another
entity shows the value it resolved to. Ask for `ShowReferences` and each such
field is written exactly as the user wrote it instead, whether a bare reference,
a template or a nested block. Given this configuration:

```hcl
resource "app" "web" {
  db_location = resource.postgres.main.location
  url         = "https://${resource.postgres.main.location}/app"
}
```

```go
text, err := xcl.EncodeEntity(app, xcl.ShowReferences())
```

the text shows each reference as it was written:

```hcl
resource "app" "web" {
  db_location = resource.postgres.main.location
  url         = "https://${resource.postgres.main.location}/app"
}
```

Without the option the same entity is written with its resolved values:

```hcl
resource "app" "web" {
  db_location = "localhost"
  url         = "https://localhost/app"
}
```

It works the same from saved data,
`c.EncodeSavedEntity(record, xcl.ShowReferences())`, and the two
give byte-identical text, since the references are kept in each entity's saved
record. It combines with `IncludeComputed`. A sensitive field shows its
reference only when it was written as a single bare reference, such as
`password = variable.db_password`; written any other way it keeps
`"(sensitive)"` unless `RevealSensitive` is also given. State saved by an
earlier version of xcl holds no references, so its text shows resolved values
until the configuration is applied again.

**This text is for reading, not for reprocessing.** By default references come
out as the literal values they resolved to, unless you ask for
`ShowReferences`. Comments and layout from the original file are not kept, and output including provider-filled values does not validate, since
xcl refuses a configuration that sets them. A sensitive value is written as
`"(sensitive)"`; see [Sensitive values](#sensitive-values) for showing the
real value.

**When it fails** no text is returned, and the error says why:

| Error | Means |
|---|---|
| `ErrUnregisteredType` | the saved data names a type the `Config` does not know. `UnregisteredTypeError` names it |
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
`fmt`, `log/slog` and `encoding/json` output. State keeps the real value,
encrypted when you give it a key, and your code reaches it only by asking for
it.

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

### Encrypting sensitive values in state

State must hold the real value of every sensitive field, so a later run can
read it back. Without a state masker it holds them in plain text. Give xcl a
key and it encrypts each sensitive value before it is saved, and decrypts it
when state is loaded:

```go
// a 32 byte key, kept wherever the application keeps its secrets and never
// committed, for example decoded from an environment variable
key, err := base64.StdEncoding.DecodeString(os.Getenv("XCL_STATE_KEY"))

stateMask, err := mask.EncryptAES256GCM(key)

c, err := xcl.NewConfig(
	xcl.WithStatePath("./state"),
	xcl.WithStateMask(stateMask),
)
```

Only the sensitive values are encrypted, everything else in state stays
readable. Each one is written as an envelope naming the masker that produced
it:

```json
"password": {"xcl_masked": "aes-256-gcm", "value": "o8Rk1x...base64..."}
```

The state masker must be able to recover what it masks, so it must implement
`mask.Reversible`. `mask.EncryptAES256GCM` does. Giving a one-way masker, such
as `mask.HashHMACSHA256`, `mask.Omit` or `mask.Redact`, fails `NewConfig` with
`xcl.ErrMaskNotReversible`:

```text
the state masker must be reversible: "hmac-sha256" cannot recover the values it masks
```

Loading state holding a value that cannot be opened fails with
`xcl.ErrUnrecoverable`, naming the entity and the masker, rather than losing
the value: state encrypted under a different key, state encrypted while no
masker is configured now, or state masked by a different masker. The
`*xcl.UnrecoverableError` detail carries the entity's `ID`, the `MaskedBy`
masker and the reason.

State written in plain text still loads once a masker is added, and the next
save encrypts it. There is no key rotation: changing the key makes existing
encrypted state unreadable, so keep the key for as long as the state exists.

### The plaintext state warning

When no state masker is configured and an `Apply` or `Destroy` writes a
sensitive value to a state store, xcl emits one warning for that operation, a
warn-level log event from `core`:

```text
sensitive values are stored unencrypted in state; use xcl.WithStateMask to encrypt them
```

A configuration with no sensitive values, a configuration with no state store,
and one with a state masker emit no such warning.

### Masking sensitive values in events

Resource data on events, see [Resource data on events](#resource-data-on-events),
writes each sensitive value through the event masker. The default is
`mask.Redact()`, which shows the marker inside an envelope naming it:

```json
"password": {"xcl_masked": "redact", "value": "(sensitive)"}
```

Choose another masker with `xcl.WithEventMask(...)`. A keyed hash lets a
receiver tell whether two events carry the same value without ever seeing it:

```go
eventMask, err := mask.HashHMACSHA256(correlationKey)

c, err := xcl.NewConfig(
	xcl.WithEventHandler(handler),
	xcl.WithEventData(xcl.EventDataProcessed),
	xcl.WithEventMask(eventMask),
)
```

When every receiver is trusted with secrets, turn masking off with
`xcl.WithNoEventMask()`, and event data carries the real values. When both
options are given, the last one wins.

Event masking changes only `Event.Data`. Errors and log details always show
`(sensitive)`, whatever the event masker, because the sensitive type formats
itself as the marker. `EncodeSavedEntity` shows the marker for any masked
value, from state or from events, never ciphertext or a hash.

### Built-in maskers

| Masker | Name | Reversible | What it writes |
|---|---|---|---|
| `mask.EncryptAES256GCM(key)` | `aes-256-gcm` | yes | AES-256-GCM ciphertext under a 32 byte key, base64, with a fresh nonce each time |
| `mask.HashHMACSHA256(key)` | `hmac-sha256` | no | the hex HMAC-SHA256 of the value under a non-empty key, the same for the same value and key |
| `mask.Omit()` | `omit` | no | no value at all, only the envelope naming the masker |
| `mask.Redact()` | `redact` | no | the marker `(sensitive)` |

Only a reversible masker can be used for state. Any masker can be used for
events.

### Writing your own masker

A masker is any type implementing `mask.Masker`. `Mask` receives the JSON of
the real value and returns the JSON to write as the envelope's `value`, or nil
to write none:

```go
type upperMasker struct{}

func (upperMasker) Name() string { return "upper" }

func (upperMasker) Mask(value json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(strings.ToUpper(string(value)))
}
```

To use it for state it must also implement `mask.Reversible`, adding
`Unmask(masked json.RawMessage) (json.RawMessage, error)`, which returns the
original JSON or an error when the value does not open.

A receiver opens a masked value in event data with `mask.Unmask(data, m)`,
given the envelope's JSON and a reversible masker with the same name. Data
produced by a one-way masker, by a different masker, or under a different key
fails with `xcl.ErrUnrecoverable` and returns no value, never a wrong one.
`mask.IsMasked` reports whether some JSON is an envelope.

The key `xcl_masked` is reserved for envelopes: an object holding it, with at
most a `value` beside it, is always read as masked data.

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
use a module in the directory `../example/modules/db`, a `db` module that
declares a database from the variables `db_username` and `db_password` and
publishes its `connection_string` as an output.

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

