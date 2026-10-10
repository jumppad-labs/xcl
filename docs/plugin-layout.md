# Plugin Layout

Every xcl plugin, whether it runs inside an application or as a separate
program, uses one layout. A plugin groups the entity types of one domain,
such as containers, Kubernetes or templates. The
[plugin template](https://github.com/jumppad-labs/xcl-plugin-template) is the
worked example of this layout, the
[Docker plugin example](../example/plugin/plugins/docker) follows it, and it
is the shape jumppad's resources take when they are ported to xcl plugins.

This guide is about where things go. For what each provider method must do,
see the [Plugin Developer Guide](plugin-developer-guide.md); for how plugins
are hosted and registered, see [Plugin Architecture](plugins.md).

## Directory tree

```
<plugin>/
  plugin.go                    the plugin type: Init builds the clients once and registers each type
  cmd/<plugin>/main.go         serves the plugin as a separate program
  entities/
    <entity>.go                one block type per file, with its nested block types
  providers/
    <entity>.go                one provider per entity type
    <entity>_<concern>.go      further files for a large provider (images, scaling, ...)
    <entity>_test.go           strict-double unit tests; real-backend tests skip without one
    <concern>.go               helpers shared by the plugin's providers, named for what they do
  client/
    <backend>/                 one package per backend or task layer
      <backend>.go             narrow interface plus its real implementation
      mocks/                   Mockery-generated doubles
  examples/<name>/main.xcl     sample configurations
  e2e/                         end-to-end tests: apply, plan with no changes, destroy
  .mockery.yml
  Makefile                     build, test, generate
  README.md
  go.mod
```

The plugin type lives in the module's root package, so it can be imported.
An application registers it in-process from there:

```go
local := registry.NewLocal()
local.RegisterPlugin(&notes.Plugin{})
```

and `cmd/<plugin>/main.go` serves the same type as a separate program:

```go
func main() {
	plugins.Serve(&notes.Plugin{})
}
```

which an application registers by its binary:

```go
local.RegisterExternalPlugin("./build/xcl-plugin-template")
```

In the template, the plugin is `notes`, so the tree is `plugin.go`,
`cmd/notes/main.go`, `entities/note.go`, `providers/note.go` with
`providers/note_test.go`, `client/files/files.go` with `client/files/mocks/`,
`examples/basic/main.xcl` and `e2e/`.

## What each part holds

### `entities/`

Block types only: structs, their xcl tags and doc comments, and no behaviour.

- An entity may embed or reference other block types of the same plugin. For
  example, a container embeds its own image and network blocks.
- A plugin doesn't import another plugin's entities. It defines its own block
  types, even where they look alike, so plugins don't depend on each other.
- An entity that needs another plugin's values takes them through an xcl
  reference in the configuration, such as a cluster's address.
- Entities never import providers, clients or backend libraries. An
  application that reads a plugin's entities from state needs only
  `entities`, for example `xcl.FindByType[entities.Note](c, "notes", "note")`.
  The template's `e2e/stateonly` test builds with nothing but xcl and
  `entities` to show it.
- Computed values are carried between applies by xcl itself, so an entity
  never loads state.

### `providers/`

The entity providers.

- **Files:** one file per entity type, with a large provider split into
  `<entity>_<concern>.go` files. Helpers shared across the plugin's
  providers go in files named for their concern.
- **Sharing:** one provider may serve several closely related types, such as
  a container and a sidecar.
- **Config-only types:** a type with no backend behaviour, such as a child
  block type or a pure grouping, is registered as config-only and has no
  provider.
- **Construction:** each provider is built by a constructor that takes the
  clients it uses, so tests build it the same way, with doubles.

### `client/`

The plugin's own layer over its backends, and the only place backend
libraries are imported.

- Each backend gets its own package. It may be a narrow interface over an
  SDK, or a domain task layer built on one, such as container tasks over a
  Docker interface.
- A client package may build on another of the plugin's client packages, or
  on an external client package the author chooses to depend on. It never
  depends on providers or entities, and any data shapes it needs are its own
  types.
- Interfaces are kept to the calls their callers make, so tests substitute
  strict doubles.
- A plugin with no backend to wrap, such as one that only writes local
  files, may leave `client/` out. The template keeps one, `client/files`, so
  it shows the shape a real backend takes.

### `plugin.go`

Builds each client once, without contacting the backend, and passes it to the
provider constructors. A client shared by several types is shared, not
rebuilt.

### `cmd/<plugin>/`, `examples/` and `e2e/`

`cmd/<plugin>/main.go` does nothing but serve the plugin type, so in-process
and separate-program use run the same code. `examples/<name>/main.xcl` holds
sample configurations, and `e2e/` applies them.

## File order

Every source file keeps its public surface together at the top, followed by
its private parts:

1. Exported types, with their exported vars and consts.
2. In a provider file, the provider type, its interface assertion and its
   constructor, then the exported methods in lifecycle order: `Init`,
   `Create`, `Read`, `Changed`, `Update`, `Destroy`, `Functions`.
3. Unexported helpers, then unexported vars and consts.

## Deciding and updating

### `Changed` is always explicit

- It answers replace when a setting that can't change in place has changed,
  matched with `change.Within(entity.Path{}.Attribute("<setting>"))`, or when
  a dependency the entity is built on is replaced.
- Otherwise it answers update when any setting changed, and defers to
  `DefaultChanged` when nothing did.
- The settings that need a replace are listed in one place. In the template
  that is `ReplaceSettings` at the top of `providers/note.go`:

  ```go
  var ReplaceSettings = []entity.Path{
  	entity.Path{}.Attribute("directory"),
  	entity.Path{}.Attribute("name"),
  }
  ```

- A provider never diffs the entity itself in another method; the decision
  lives in `Changed`.

### `Update` works only from what it is told

- It acts on the `changes` and `dependencies` it is given, and works out
  previous values from each change's `Before`.
- It never calls the backend to rediscover previous state. It may read back
  outputs, such as an assigned address, after acting.
- Values a later decision needs, such as an image ID or a file checksum, are
  kept as computed values on the entity.

In the template, a `content` change rewrites the note's file and keeps its new
checksum, and a `mode` change on its own only changes the file's mode.

### `Read` reports the real object

It replaces jumppad's `Lookup` of backend IDs: anything a caller needs to know
about the real object, such as its backend ID, is a computed value `Read`
fills in. When the object is gone, `Read` returns `plugins.ErrNotFound`.

## Tests

- **Unit tests** sit beside each provider. They build it through its
  constructor with strict Mockery doubles, so an unexpected backend call
  fails the test. They cover the `Changed` rules, and show that `Update`
  makes only the calls the reported changes require.
- **Client packages** have their own tests against doubles of the layer
  beneath them, or, for the template's local files, against a temporary
  directory.
- **Real-backend tests** skip when no backend is available.
- **`e2e/`** builds the plugin, applies sample configurations, confirms the
  next plan reports no changes, and destroys them. It can run against more
  than one runtime, such as Docker and Podman.
- **Test style:** testify `require`, no table-driven tests, positive and
  negative cases in separate functions.

## Start from the template

The quickest way to a plugin in this layout is the template. Click **Use this
template** on
[jumppad-labs/xcl-plugin-template](https://github.com/jumppad-labs/xcl-plugin-template),
or:

```shell
gh repo create <you>/xcl-plugin-<name> --template jumppad-labs/xcl-plugin-template --public --clone
```

Unchanged, it builds, tests and applies its sample with nothing else running.
Its README walks through making it yours, with one command each for building
(`make build`), testing (`make test`), regenerating the doubles
(`make generate`), running it in-process (`make inprocess`) and as a separate
program (`make external`), and adding an entity type of your own.

## Releasing

The template's release automation publishes the layout the GitHub registry
installs. Pushing a version tag (`make release VERSION=v0.1.0`) builds the
plugin for linux, darwin and windows on amd64 and arm64 with `make dist`, and
publishes a GitHub release holding:

- `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` on windows), with the binary
  at the archive root;
- `<name>_<version>_checksums.txt`, in `sha256sum` format;
- `<name>_<version>_checksums.txt.sig`, an armoured detached OpenPGP
  signature over the checksums file, made with the key in the repository's
  secrets.

`<name>` is the repository name and `<version>` the tag without its leading
`v`. An application installs the release with the GitHub registry, see
[Installing plugins from GitHub](https://xcl.dev/github-registry/).

## Not covered by this layout

- Behaviour jumppad's engine provides outside any resource, such as its
  implicit image cache and registry merging. A port to xcl decides these
  separately.
