---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
spec: 20261009092551-82db0140-plugin-template
specs:
    - 20261009092551-82db0140-plugin-template
    - 20261009102138-48e95432-docker-example-standard-layout
---

# Standard xcl plugin layout

Every xcl plugin, whether it runs inside an application or as a separate program, uses one layout. A plugin groups the resource types of one domain, such as containers, Kubernetes or templates. The plugin template, the plugin examples and the documentation all follow this layout, and it is the shape jumppad's resources take when they are ported to xcl plugins.

## Directory tree

```
<plugin>/
  plugin.go                    the plugin type: Init builds the clients once and registers each type
  cmd/<plugin>/main.go         serves the plugin as a separate program
  entities/
    <resource>.go              one block type per file, with its nested block types
  providers/
    <resource>.go              one provider per resource type
    <resource>_<concern>.go    further files for a large provider (images, scaling, ...)
    <resource>_test.go         strict-double unit tests; real-backend tests skip without one
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

The plugin type lives in the module's root package, so it can be imported. An application registers it in-process from there, and `cmd/<plugin>/main.go` serves the same type as a separate program.

## What each part holds

- **`entities/`** holds block types only: structs, their xcl tags and doc comments, and no behaviour.
  - An entity may embed or reference other block types of the same plugin. For example, a container embeds its own image and network blocks.
  - A plugin doesn't import another plugin's entities. It defines its own block types, even where they look alike, so plugins don't depend on each other.
  - A resource that needs another plugin's values takes them through an xcl reference in the configuration, such as a cluster's address.
  - Entities never import providers, clients or backend libraries. An application that reads a plugin's resources from state needs only `entities`.
  - Computed values are carried between applies by xcl itself, so an entity never loads state.
- **`providers/`** holds the resource providers.
  - **Files:** one file per resource type, with a large provider split into `<resource>_<concern>.go` files. Helpers shared across the plugin's providers go in files named for their concern.
  - **Sharing:** one provider may serve several closely related types, such as a container and a sidecar.
  - **Config-only types:** a type with no backend behaviour, such as a child block type or a pure grouping, is registered as config-only and has no provider.
  - **Construction:** each provider is built by a constructor that takes the clients it uses, so tests build it the same way, with doubles.
- **`client/`** is the plugin's own layer over its backends, and the only place backend libraries are imported.
  - Each backend gets its own package, and may be a narrow interface over an SDK or a domain task layer built on one; for example, container tasks over a Docker interface.
  - A client package may build on another of the plugin's client packages, or on an external client package the author chooses to depend on. It never depends on providers or entities, and any data shapes it needs are its own types.
  - Interfaces are kept to the calls their callers make, so tests substitute strict doubles.
  - A plugin with no backend to wrap, such as one that only writes local files, may leave `client/` out.
- **`plugin.go`** builds each client once, without contacting the backend, and passes it to the provider constructors. A client shared by several types is shared, not rebuilt.

## File order

Every source file keeps its public surface together at the top, followed by its private parts:

1. Exported types, with their exported vars and consts.
2. In a provider file, the provider type, its interface assertion and its constructor, then the exported methods in lifecycle order: `Init`, `Create`, `Read`, `Changed`, `Update`, `Destroy`, `Functions`.
3. Unexported helpers, then unexported vars and consts.

## Deciding and updating

- **`Changed` is always explicit.**
  - It answers replace when a setting that can't change in place has changed, matched with `change.Within(entity.Path{}.Attribute("<setting>"))`, or when a dependency the resource is built on is replaced.
  - Otherwise it answers update when any setting changed, and defers to `DefaultChanged` when nothing did.
  - The settings that need a replace are listed in one place.
  - A provider never diffs the resource itself in another method; the decision lives in `Changed`.
- **`Update` works only from what it is told.**
  - It acts on the `changes` and `dependencies` it is given, and works out previous values from each change's `Before`.
  - It never calls the backend to rediscover previous state. It may read back outputs, such as an assigned address, after acting.
  - Values a later decision needs, such as an image ID or a file checksum, are kept as computed values on the entity.
- **`Read` reports the real resource.** It replaces jumppad's `Lookup` of backend IDs: anything a caller needs to know about the real resource, such as its backend ID, is a computed value `Read` fills in.

## Tests

- **Unit tests** sit beside each provider. They build it through its constructor with strict Mockery doubles, so an unexpected backend call fails the test. They cover the `Changed` rules, and show that `Update` makes only the calls the reported changes require.
- **Client packages** have their own tests against doubles of the layer beneath them.
- **Real-backend tests** skip when no backend is available.
- **`e2e/`** builds the plugin, applies sample configurations, confirms the next plan reports no changes, and destroys them. It can run against more than one runtime, such as Docker and Podman.
- **Test style:** testify `require`, no table-driven tests, positive and negative cases in separate functions.

## Not covered by this layout

- Behaviour jumppad's engine provides outside any resource, such as its implicit image cache and registry merging. A port to xcl decides these separately.
