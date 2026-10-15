# XCL Configuration Language

[![Go Reference](https://pkg.go.dev/badge/github.com/jumppad-labs/xcl.svg)](https://pkg.go.dev/github.com/jumppad-labs/xcl)

XCL is a configuration language for describing things and how they relate to each other. It builds on the readability
of HCL (blocks, attributes and expressions) and adds what modern configuration needs:

- **References**: any value in a block can come from another block, such as a database's address or a generated
  password, even one that only exists once that block has been created. XCL works out the value for you.
- **Graph based processing**: those references form a directed acyclic graph, so every block is processed in
  dependency order and a value is always set before anything reads it.
- **Plugins**: block types, and the providers that create, read, update and destroy them, come from plugins run
  in-process or as separate programs, and installed from local paths or signed releases.

It also has variables, outputs, modules, functions and state, so a configuration can be planned, applied, changed and
destroyed. Functions come built in, from the application embedding XCL, or from plugins, and are all called the same
way from configuration.

XCL starts from HCL's syntax but does not aim to be compatible with Terraform configuration or other HCL dialects.
Where XCL can be clearer or do more, it will diverge.

This repository is the Go implementation: you define block types as Go structs, without needing to know the HCL
library underneath. Implementations for Python, JavaScript and other languages are planned.

## Block types

Every block type is registered with a type and, optionally, a subtype. They become the block's labels, followed by its
name, and the address other blocks use to refer to it:

```hcl
database "postgres" "main" {
  location = "localhost"
  port     = 5432
  username = "admin"
  password = env("DB_PASSWORD")
}

app "web" {
  database_location = database.postgres.main.location
  connection_string = database.postgres.main.connection_string
}
```

- `database "postgres" "main"` has the type `database`, the subtype `postgres` and the name `main`. Other blocks refer
  to it as `database.postgres.main`.
- `app "web"` has the type `app`, no subtype, and the name `web`. Other blocks refer to it as `app.web`.

A subtype groups related block types under one type: `database "postgres"` and `database "mysql"` can be different Go
types with different attributes, provided by different plugins.

## How configuration is processed

Processing happens in two steps.

#### Step 1: build the graph

XCL reads every file and finds the references between blocks. `app.web` reads two values from
`database.postgres.main`, so it depends on it:

```
app.web
└── database.postgres.main
```

References and `depends_on` together form a directed acyclic graph. A cycle is reported as an error.

#### Step 2: walk the graph

The graph is walked in dependency order. `database.postgres.main` is processed first: its provider creates the
database and sets `connection_string`, a computed value only the provider knows. `app.web` is processed next, with
its references filled in from the database's values. Blocks with no dependency between them are processed at the same
time, and destroying runs in reverse order.

## Quick start

### Configuration only

An application that only reads its configuration declares each Go type, on a
local registry, under the block type name it is written with, applies the
configuration and reads it back. No plugin and no provider is needed. A
block type is a plain struct of the application's own:

```go
type PostgreSQL struct {
	Engine string     `xcl:"engine,label"` // the block's labels, filled by xcl:
	Name   string     `xcl:"name,label"`   // database "postgres" "main"
	Port   int        `xcl:"port,optional"`
	Meta   types.Meta // the block's id, file and line, filled by xcl
}

local := registry.NewLocal()
registry.Register[resources.Deployment](local, "deployment") // deployment "api" {}
registry.Register[resources.Service](local, "service")       // service "api" {}
registry.Register[PostgreSQL](local, "database", "postgres") // database "postgres" "main" {}

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

A field tagged as a label is filled from the block's labels and can be read
in expressions like an attribute (`database.postgres.main.name`), and a field
called `Meta`, a `types.Meta`, receives the block's metadata, its disabled
flag and dependencies among it (`Meta.Disabled`, `Meta.DependsOn`).
Configuration sets `disabled` and `depends_on` on any block; a type declares
no fields by those names. A plugin's types are written the same way. A type
does not embed `types.ResourceBase`: xcl stores each type as one built from
it that does. On Go 1.27 the
declaration is also a method, `local.Register[PostgreSQL]("database", "postgres")`,
and `RegisterType(&PostgreSQL{}, ...)` remains for a type only known at run
time.

A plain type that cannot read a name, having neither label fields nor a `Meta`
field, is a singleton: its block is written without labels,
`runner { server = "..." }`, it is addressed by its type alone, as
`runner.server`, and a second `runner` block in the same module is an error. A
singleton takes no subtype.

To read configuration through [Viper](https://github.com/spf13/viper), the
`github.com/jumppad-labs/xcl/viper` package returns a Viper that reads `.xcl`
files into the same plain types, as keys such as `database.postgres.main.port`
and, for a singleton, `runner.server`. `xclviper.Read` reads a file or a whole
directory of them:

```go
v := xclviper.New(
	xclviper.Type[Runner]("runner"),
	xclviper.Type[PostgreSQL]("database", "postgres"),
)
err := xclviper.Read(v, "./config")
```

**No state.** Without `WithStatePath` or `WithStateStore` nothing is
persisted: the applied configuration is held in memory only. This is the
supported mode for configuration-only use. Give `WithStatePath` a directory to
keep state between runs, so a later run can plan, update and destroy what an
earlier one applied.

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
platform, see [Installing plugins from GitHub](https://xcl.dev/github-registry/):

```go
gh := registry.NewGitHub(registry.GitHubTrustedKeys(publicKey))
gh.RegisterPlugin("jumppad-labs/xcl-plugin-docker", "v0.2.0")
```

**Writing a plugin.** Start from the
[plugin template](https://github.com/jumppad-labs/xcl-plugin-template), a
GitHub template repository with one working entity type that runs both
in-process and as a separate program, tests against strict doubles and
publishes signed releases the GitHub registry installs. It follows the
standard [plugin layout](docs/plugin-layout.md) every xcl plugin uses.

**Registration problems.** Registration returns no errors. Problems are
reported in one of three places:

- A mistake in a single call, wrong on every run, panics straight away
  naming the type: an empty name, more than one subtype or an empty subtype,
  a prototype that is not a pointer to a struct, a plain struct xcl cannot lay
  out (such as one declaring an attribute called `meta`, or a `Meta` field
  that is not a `types.Meta`), or a nil plugin or registry.
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

## Documentation

The documentation lives at **[xcl.dev](https://xcl.dev)**:

- [Language reference](https://xcl.dev/language/): files, entities, attributes and blocks, references, expressions, variables, modules and functions, with Go and Python types
- [Registries](https://xcl.dev/registries/): declaring Go types and registering plugins
- [Installing plugins from GitHub](https://xcl.dev/github-registry/) and the
  [plugin template](https://xcl.dev/plugin-template/)
- [Unchanged, update or replace](https://xcl.dev/replacement/): how providers decide what a change does
- [Diffs](https://xcl.dev/diff/): what an apply would change, before it runs
- [Events and logging](https://xcl.dev/events/) and [plugin logging](https://xcl.dev/plugin-logging/)
- [Sensitive values](https://xcl.dev/sensitive-values/) and [state masking](https://xcl.dev/state-masking/)
- [Configuration text](https://xcl.dev/configuration-text/): turning entities back into XCL

## Examples

Each example under [`example`](./example) is a Go module of its own, and runs with `make run`:

- [Configuration only](https://xcl.dev/examples/configuration-only/) ([`example/configonly`](./example/configonly)):
  configuration read into Go types, with no plugins.
- [Plugins and the lifecycle](https://xcl.dev/examples/plugins/) ([`example/plugin`](./example/plugin)): an
  external Docker plugin and an in-process template plugin that create, update and destroy real things.
- [`example/remoteplugin`](./example/remoteplugin): the Docker plugin installed from its signed GitHub release.

`make test-examples` builds, vets and tests every example.

## Other languages

The engine is available to Python as the `xcl-lang` package, `pip install xcl-lang`, imported as `xcl`,
from [jumppad-labs/xcl-python](https://github.com/jumppad-labs/xcl-python):
the same engine, bundled in the wheel as the `xcl-core` program and run as a
subprocess on every platform, Alpine included, with no shared library to
load, and a synchronous and an asyncio API. Go users are unaffected: the Go
library needs no cgo, and nothing in it changes for the binding.

## Contributing

[`docs/`](./docs) holds notes on how xcl is built inside: the parser and entity lifecycle, state, modules and
the plugin architecture.

## License

Apache 2.0, see [LICENSE](./LICENSE). The HCL library forked under `internal/xcl` keeps its MPL-2.0 license, see
[NOTICE](./NOTICE).
