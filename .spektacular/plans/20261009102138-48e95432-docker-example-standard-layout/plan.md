---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Plan: 20261009102138-48e95432-docker-example-standard-layout

<!-- Metadata -->
<!-- Created: 2026-10-09T13:42:20Z -->
<!-- Commit: a000ffa -->
<!-- Branch: f-plugin-registries -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

This plan rebuilds the plugin example's Docker plugin to the standard xcl plugin layout. The plugin becomes a module of its own, with its entity types, providers, backend client layer, entry point, sample and end-to-end test where the layout design puts them. Every file in the plugin example lists its public surface first. It solves two problems. The example application pulls in the Docker providers and SDK just to read entity types from state, and the example teaches a different shape from the one the coming plugin template uses. Plugin authors get an example that matches the standard and the template, and applications reading a plugin's state stay light. The example's behaviour does not change, and the README, guides and documentation site are updated to the new locations.

## Conventions

- **Testing & Mocking: testify `require`, Mockery, no table-driven tests, positive and negative cases in separate functions, tests located with their source** — every moved and new test (provider, client, ping, e2e) is written this way, and each test file sits in the directory of the code it tests.
- **Never write tests that inspect repository files or enforce code rules by parsing source** — the "application build pulls in no Docker libraries" and "file order" checks stay manual (reviewing the application's package dependency list, and code review), never a test that reads `go.mod` or greps imports; docs are checked by review and the site build.
- **Code style (gofmt, go vet, semantic import grouping, small focused interfaces, `any`)** — the new `containers.Tasks` interface is kept to the calls the providers make; imports grouped stdlib / third-party / xcl.
- **Dependencies: pin versions in go.mod, prefer the standard library** — the new plugin `go.mod` pins the same Docker SDK and testify versions the example uses today; the application's engine check uses only the standard library instead of the SDK.
- **NEVER modify dependency packages** — the module split is done with `replace` directives to local paths only; nothing under the module cache is touched.
- **Changelog: one top `CHANGELOG.md` entry per spec, headed with the spec name, prose then a **Breaking:** list** — written in the README and guides task (settled across the epic's plans).
- **Example modules cannot import xcl's internal packages (gotcha)** — the new plugin module uses only public xcl packages and keeps its own small test helpers.

## Architecture & Design Decisions

The Docker plugin at `example/plugin/plugins/docker` (repo **xcl**) is rebuilt in place to the standard plugin layout in the design `plugin-layout.md` (source `design`), which is the settled shape this plan builds on. It becomes a Go module of its own, `github.com/jumppad-labs/xcl/example/plugin/plugins/docker`, with `replace github.com/jumppad-labs/xcl => ../../../..` (the per-example module pattern the knowledge entry *example modules cannot import internal* describes). Its parts land where the design puts them: `plugin.go` (package `docker`, the importable `Plugin` type) and `cmd/docker/main.go` (serves it as a separate program); `entities/network.go` and `entities/container.go` (block types only, `NetworkAttachment` nested with the container, importing nothing but `xcl/types`); `providers/network.go`, `providers/container.go`, and the shared helpers `providers/attachments.go` and `providers/labels.go`; `client/docker/docker.go` (today's narrow Docker SDK interface with `New` and `Ping`, moved unchanged) and `client/containers/containers.go` (a task layer over it with its own types); `mocks/` beside each client package; `examples/basic/main.xcl`; `e2e/`; and `.mockery.yml`, `Makefile`, `README.md`, `go.mod` at the plugin root. The example application keeps its place at `example/plugin`, requires the plugin module through `replace => ./plugins/docker`, and imports only `.../plugins/docker/entities`.

Two decisions go beyond moving files, and both come from the design rather than from a new idea. First, the design makes `client/` the only place backend libraries are imported, and today's providers import Docker SDK types and `dockerclient.IsErrNotFound`. So the SDK translation the providers do now (building create options, bind mounts, `KEY=VALUE` environment, pulling a missing image, reading the first network's address, recognising "not found") moves into `client/containers`. That layer's `Tasks` interface speaks in the plugin's own types and returns a wrapped `containers.ErrNotFound`. Providers depend only on `containers.Tasks`. The Docker calls made, their order and their error messages stay exactly as they are, so behaviour does not change: "only layout and ordering" is read as no observable change. Second, the application's check for a running Docker engine before `apply`, `plan` and `destroy` can no longer go through the plugin's client without pulling the SDK into the application binary. It is reimplemented with the standard library: `GET /_ping` on `DOCKER_HOST` (unix or tcp, default `unix:///var/run/docker.sock`), with the same 5-second timeout and the same "no Docker engine reachable" message. A scheme it cannot dial skips the check, and the plugin then reports the engine error itself.

Every source file in the plugin example follows the design's file order. Exported types, vars and consts come first. In provider files, the provider type, its interface assertion and its constructor follow, then `Init, Create, Read, Changed, Update, Destroy, Functions` in that order. Unexported helpers and unexported vars/consts come last. This covers the Docker plugin, the in-process template plugin (reordered only, not restructured) and the application's own files. The network's `Changed` is made explicit in the design's form. A single `replaceSettings` list (`subnet`) is matched with `change.Within`, a non-empty `changes` answers update, and otherwise the decision defers to `DefaultChanged`. Because `DefaultChanged` already answers update whenever settings differ (`plugins/changed.go:33-49`), the outcomes do not change. The container's `Changed`/`Update` logic is untouched apart from its placement.

Tests move with their code and keep their assertions. The provider unit tests use strict Mockery doubles of `containers.Tasks`. The SDK-level expectations they assert today (labels, IPAM subnet, read-only bind, sorted disconnects, pull-when-missing) move to `client/containers` tests against doubles of `client/docker.Docker`. Real-engine tests stay beside the providers and still skip without an engine. The new `e2e/` builds `cmd/docker`, applies `examples/basic`, checks that `Config.Diff` reports `Changed() == 0` and destroys. The application's scenario and unit tests change only their import paths and the plugin build command. The **xcl-website** repo's plugin example page (and the other pages that quote Docker plugin files) and the xcl README and `docs/` guides are updated to the new paths and code. CI, the root e2e example runner and `.gitignore` learn about the nested module. Rejected alternatives (one module, providers keeping SDK types, one combined client package, app importing the plugin client, dropping or relocating the engine check) are recorded with evidence in `research.md#alternatives-considered-and-rejected`.

Conventions that shape this: test style (testify `require`, Mockery, no table-driven tests, positive and negative cases apart, tests beside their source) drives the test moves and the new client tests. The rule that no test inspects repository files means the "application build has no Docker libraries" check is a manual check of the application's package dependency list, not a test. Never modifying dependencies and pinning versions keep the Mockery pin (`v3.8.0`) and the module requirements as they are.

## Component Breakdown

- **Docker plugin module (new module boundary, existing code).** It owns everything the Docker plugin is: its block types, providers, backend layer, entry points, samples, tests and build. It becomes a Go module of its own, so an application can depend on part of it, the entity types, without the rest. The example application depends on it through a local module replacement.
- **Plugin type (changed).** It owns building the backend layer once and registering `docker "network"` and `docker "container"`, each with a provider built through its constructor. It moves from a `main` package into the module's importable root package, so the separate-program entry point and any in-process host register the same type. It builds the Docker SDK client and the container task layer once, without contacting Docker, and shares one task layer between both providers.
- **Separate-program entry point (moved).** It owns serving the plugin type over xcl's plugin protocol. It is a thin `main` that calls `plugins.Serve` with the plugin type. The example application's build and tests compile it to the `docker-plugin` binary as today.
- **Entity types (new package, existing types).** It owns the `Network`, `Container` and nested `NetworkAttachment` block types: fields, xcl/json tags and doc comments, and no behaviour. It depends only on xcl's `types`. The example application's `status` command and its tests read state through it alone.
- **Providers (moved and reordered).** It owns the network and container resource providers, their explicit `Changed` decisions, their `Update`s worked from the reported changes and dependencies, and the shared helpers they need (previous network attachments, network-address recognition, labels). Each provider is built from the container task layer it is given. Providers no longer import any Docker library. Their methods follow the lifecycle order.
- **Docker SDK client (moved, unchanged).** It owns the narrow interface over the Docker Engine SDK calls the plugin makes, the real SDK client that satisfies it, `New` and `Ping`, and its generated double. Only the container task layer, and the tests that inspect real Docker objects, use it.
- **Container task layer (new).** It owns translating the plugin's intent into Docker SDK calls: creating, inspecting and removing networks, connecting and disconnecting containers, pulling an image Docker lacks, creating, starting, stopping and removing containers with labels, environment and the read-only init-script mount, reading a container's address on its first network, and turning Docker's "not found" into its own sentinel. It speaks in its own data types, is built on the Docker SDK client interface, and has its own generated double for the provider tests. It is justified by the design's rule that backend libraries are imported only under `client/`.
- **Plugin samples and end-to-end tests (new).** It owns one sample configuration and an end-to-end test that builds the plugin binary, applies the sample through a host using a local registry, confirms a plan of the same configuration reports no changes, and destroys it. The test skips when no Docker engine is reachable.
- **Plugin build and generation (new for the plugin, moved from the app).** It owns the plugin's Makefile (build, test, generate), its Mockery configuration for both client packages, its README, and its `go.mod`. The example application's Makefile and Mockery configuration stop generating plugin doubles and build the plugin binary from the plugin module.
- **Example application (changed).** It owns the `xcl-docker` commands. It now imports only the plugin's entity types. Its pre-flight Docker engine check is its own small standard-library ping that keeps today's message and timeout. Its tests build the plugin from the plugin module and import the entity types. Its files and the in-process template plugin's files are reordered to the file order, with nothing restructured.
- **Repository wiring (changed).** CI's example build loop, the root end-to-end runner of example test suites and `.gitignore` gain the plugin module, so it is built, vetted and tested like the other examples.
- **Documentation (changed, two repos).** The xcl README and `docs/` guides, and the xcl-website plugin example page plus the other site pages that quote Docker plugin files, show the new file locations and the moved code. The site still builds.

## Data Structures & Interfaces

No serialization boundary changes. The block types, their xcl and json tags, the state they produce and the plugin protocol all stay exactly as they are. The contract doesn't change. The new or moved contracts are all inside the example.

**Entity types (`entities` package, moved unchanged).** `Network`, `Container` and `NetworkAttachment` keep every field, tag and doc comment. Only their package changes, from `resources` to `entities`:

```go
package entities // imports only github.com/jumppad-labs/xcl/types

type Network struct {
    types.ResourceBase `xcl:",remain"`
    Subnet   string `xcl:"subnet,optional" json:"subnet,omitempty"`
    DockerID string `xcl:"docker_id,optional,computed" json:"docker_id,omitempty"`
}

type Container struct { /* Image, Command, Environment, InitScript, Networks []NetworkAttachment, DockerID, IPAddress — unchanged */ }
type NetworkAttachment struct { Name string; Aliases []string }
```

**Plugin type (module root package `docker`, moved).** `type Plugin struct{ plugins.PluginBase }` with `Init(logger.Logger, plugins.State) error`. Registration names, `docker "network"` and `docker "container"`, are unchanged.

**Docker SDK client (`client/docker`, moved unchanged).** `type Docker interface { … }` keeps the same thirteen SDK-signature methods. It keeps `New() (Docker, error)` and `Ping(ctx) error`. The real `*dockerclient.Client` satisfies it.

**Container task layer (`client/containers`, new).** This is the only contract providers hold. It uses its own types and never imports `entities` or `providers`:

```go
package containers

var ErrNotFound = errors.New("docker object not found") // matched with errors.Is; wraps Docker's not-found errors

type Attachment struct { Name string; Aliases []string }

type NetworkSpec struct { Subnet string; Labels map[string]string }

type ContainerSpec struct {
    Name, Image  string
    Command      []string
    Environment  map[string]string // sent as sorted KEY=VALUE
    Labels       map[string]string
    InitScript   string            // absolute host path, mounted read-only; "" for none
    Network      *Attachment       // the network the container is created on; nil for none
}

type Tasks interface {
    CreateNetwork(ctx context.Context, name string, spec NetworkSpec) (string, error)
    NetworkContainers(ctx context.Context, networkID string) ([]string, error) // sorted container IDs
    RemoveNetwork(ctx context.Context, networkID string) error
    ConnectNetwork(ctx context.Context, network, containerID string, aliases []string) error
    DisconnectNetwork(ctx context.Context, network, containerID string) error   // always forced
    EnsureImage(ctx context.Context, ref string) error                          // pull only when missing
    CreateContainer(ctx context.Context, spec ContainerSpec) (string, error)
    StartContainer(ctx context.Context, containerID string) error
    StopContainer(ctx context.Context, containerID string) error
    RemoveContainer(ctx context.Context, containerID string) error              // always forced
    ContainerAddresses(ctx context.Context, containerID string) (map[string]string, error) // network name → IP
}

func New(docker docker.Docker) Tasks
```

**Provider constructors (changed parameter type).** `NewNetworkProvider(tasks containers.Tasks) plugins.ResourceProvider[*entities.Network]` and `NewContainerProvider(tasks containers.Tasks) plugins.ResourceProvider[*entities.Container]`. Providers keep their current error messages and log lines, and wrap task errors exactly as they wrap SDK errors today.

**Labels (`providers` package, unchanged).** The exported `LabelCreatedBy`, `CreatedByValue` and `LabelXCLID` constants stay with the providers, which apply them.

**Application engine check (new, package `main`).** `pingDocker(ctx context.Context) error` sends `GET /_ping` to the engine `DOCKER_HOST` names (`unix://` or `tcp://`; default `unix:///var/run/docker.sock`) within 5 seconds. Any failure returns an error starting `no Docker engine reachable`. A scheme it cannot dial returns nil.

## Implementation Detail

**A new module boundary inside an example.** Until now each example under `example/` has been exactly one Go module. This plan introduces the first nested one: the Docker plugin is its own module inside the plugin example's directory. The parent module drops the plugin's directory from its own package tree and depends on the plugin through a local `replace`. The same pattern the examples already use to point at the local xcl is applied one level down. A developer working on the plugin works inside it and uses its own test and generate targets, as they would in any plugin made from the coming template. A developer working on the application builds the plugin binary from the plugin module, which the application's Makefile and test setup do for them. CI and the root end-to-end runner treat the plugin module as one more example module.

**Splitting one package three ways.** The single `resources` package, which holds types, providers and helpers, becomes `entities` (data only), `providers` (behaviour) and, under `client/`, an SDK layer and a task layer. Import direction is strictly one way: `providers` → `entities` and `client/containers`; `client/containers` → `client/docker` → Docker SDK; the plugin type → all of them. Nothing points back. A reader can open `entities` and see the whole configuration surface with no Docker code in view. They can open a provider and read the decision and update logic in plugin terms, `ConnectNetwork` rather than `network.EndpointSettings`, and find every Docker SDK detail in one task-layer file. This follows the design's layering exactly, and it is the shape the plugin template will copy.

**A task layer as the seam for tests.** Provider unit tests change their double from the SDK interface to the task interface. They then read as the design asks: given these changes, `Update` makes only these task calls. The SDK-shape assertions that are valuable on their own (labels on every object, IPAM only when a subnet is set, read-only bind at the entrypoint path, sorted environment, pull only when missing, forced disconnect and remove, not-found translation) move to task-layer tests written against the SDK double. That double is generated, as today. Each moved test keeps its name, intent and positive/negative split. Only the layer it targets changes. Real-engine tests stay beside the providers. They build the real task layer over the real SDK client and skip without an engine.

**File order as a visible convention.** Every Go file reads top-down as public surface, then private detail. Exported types, vars and consts come first. In a provider file, the provider struct, its interface assertion, its constructor and the seven lifecycle methods follow in lifecycle order. Unexported helpers come next, and unexported vars and consts close the file. Today helpers sit next to the method that first uses them. After the change a reader always finds `Destroy` after `Update`, and finds helpers at the bottom. The rule is applied to the Docker plugin, the in-process template plugin and the application's files alike. Nothing is renamed and no logic moves between functions, except the network's `Changed`, which takes the design's explicit form with the same outcomes.

**The application stays light.** The application imports the plugin's entity types and nothing else from the plugin. Its one other need from Docker, a pre-flight "is an engine there?" check, is met by a tiny standard-library ping that knows only enough of `DOCKER_HOST` to dial a unix or tcp engine. This is a new, deliberately small piece of application code. It is not shared with the plugin, because sharing it would recreate the dependency the spec removes.

## Dependencies

- **Design `plugin-layout.md` from the `design` design source**: the settled plugin layout, what each part holds, file order, the explicit `Changed`, `Update` from what it is told, and the test shape. This plan implements it for the Docker plugin. No changes needed.
- **xcl public packages (`plugins`, `types`, `entity`, `logger`, `registry`, `diff`, root `xcl`)**: the plugin contract and host APIs the plugin and its e2e use. They are used as they are today; the contract doesn't change.
- **Docker Engine SDK (`github.com/docker/docker` v28.5.2+incompatible) and `github.com/opencontainers/image-spec` v1.1.1**: the backend library. After this plan they are imported only by the plugin's `client/` packages, plus the app's tests that inspect real objects. The versions stay the same, now required by the plugin module.
- **testify v1.12.1 and Mockery v3.8.0 (run without a local install, at a pinned version)**: test assertions and generated strict doubles for both client packages. The same pinned versions as today.
- **Example `prettylog` module**: still used by the application only, through its existing local replace. No changes.
- **Go toolchain minimum 1.25.0**: the new module declares the same `go` directive as the other examples and is built on the minimum toolchain in CI.
- **A reachable Docker engine (external service, optional)**: needed only by the real-engine tests, the plugin's e2e and the app's scenario tests, which all skip without one. Nothing new.
- **xcl-website repo (Astro 5 / MDX)**: hosts the plugin example page and other pages quoting Docker plugin files. Edited by this plan and verified with its existing build.
- **Upstream specs/plans**: none. This spec has no dependencies in the epic and can start immediately. Its sibling `20261009092551-82db0140-plugin-template` builds the same layout and writes the standard layout into the guides. This plan leaves that write-up to it, and both specs edit `docs/plugin-developer-guide.md`, so whichever lands second rebases its doc edits.
- **Design documents this plan was built on**: `plugin-layout.md` from the `design` design source, the binding layout above.

## Testing Approach

The rebuild keeps behaviour, so the main safety net is the suite that already exists. The application's scenario tests cover network swap, subnet rebuild, init-script rebuild and content edit, network removal and the dangling reference. The application's unit and smoke tests and the providers' unit and real-engine tests round it out. All of them must pass with their assertions untouched. Only their import paths and the plugin build command change. Every test follows the project rules: testify `require`, Mockery doubles, no table-driven tests, positive and negative cases in separate functions, and each test beside the code it tests.

**Unit tests (most coverage).**
- **Providers**: the tests move with the providers and switch their strict double from the SDK interface to the container task interface. They keep guaranteeing the `Changed` rules (replace for a change within image, command, environment or init script, or when a non-network dependency is replaced; update for network changes; no change when nothing differs) and the network's subnet replace. They also keep guaranteeing that `Update` makes only the task calls the reported changes and dependencies require, and none when it is told nothing. Because the double is strict, an unexpected call to rediscover previous state fails the test.
- **Container task layer (new)**: tests against a strict double of the SDK interface. They guarantee the Docker calls are exactly the ones the providers made before: labels on every object, IPAM only when a subnet is set, a bridge attachable network, a read-only bind of the init script at the entrypoint path, sorted `KEY=VALUE` environment, a pull only when the image is missing with its progress drained, forced disconnects and removes, sorted container IDs from a network inspect, and Docker not-found errors matched by `errors.Is(err, containers.ErrNotFound)`. Positive and negative cases are separate functions. Every SDK-shape assertion removed from the provider tests reappears here, so coverage is moved, not lost.
- **Docker SDK client**: the existing `New` and `Ping` tests move with it.
- **Application engine check (new)**: a ping that succeeds against a stub engine listening on a temporary unix socket; failure, with the "no Docker engine reachable" message, when nothing listens; and nil for a scheme it cannot dial.

**Real-engine integration tests.** These stay beside the providers. They build the real task layer over the real SDK client and skip when no engine is reachable, as today.

**End-to-end (new, plugin module).** The plugin's `e2e/` builds the plugin binary, applies the sample configuration through a host with a local registry, checks that a plan of the same configuration reports zero changes, and destroys it. It skips without a Docker engine. The root end-to-end runner gains an entry that runs the plugin module's whole suite, as it already does for each example module.

**Success metrics.**
- **One shape everywhere** (the example and the docs never disagree with the layout design): **Manual — captured in the implementation test plan**. A reviewer walks the design's directory tree and "what each part holds" against the rebuilt plugin, and checks every doc path quoted in the README, guides and site pages against the tree. The project's rules forbid tests that read docs or walk source.
- **Example applications stay light** (the app builds without the Docker libraries): **Manual — captured in the implementation test plan**. The application's package dependency list is checked for the absence of any Docker SDK package. A test that inspects imports or `go.mod` is forbidden by the project's conventions.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: file-order review. Every source file in the plugin example lists exported types and methods before unexported helpers, and provider methods follow `Init, Create, Read, Changed, Update, Destroy, Functions`.
- **Manual — captured in the implementation test plan**: the xcl-website site builds, and the plugin example page (and the other pages quoting Docker plugin files) show only paths that exist in the rebuilt example.
- **Manual — captured in the implementation test plan**: with a Docker engine, the plugin example's run, swap, replace, rebuild-init and remove-network targets behave as before, and the plugin module's build, test and generate targets work.

**Deliberate gaps.** No new tests guard documentation text, file order or import rules. The conventions forbid them, and review covers them. The in-process template plugin's behaviour is not retested beyond its existing tests, since it is only reordered.

## Milestones & Tasks

### Milestone 1: The plugin example's application reads state with only the Docker plugin's entity types

**What changes**: The Docker plugin becomes a module of its own, laid out in the standard shape: entity types in their own package, providers in theirs, and every Docker library call behind the plugin's own client layer. It has an importable plugin type and a separate entry point that serves it. The plugin example's application now depends only on the entity types to read what it applied. It checks for a running Docker engine with its own small check, so building the application no longer brings in the Docker libraries. Everything the example does, applying, planning, showing status, inspecting and destroying across all its scenarios, behaves exactly as before. CI and the repository's end-to-end runner build and test the new module alongside the other examples.

**Validation point**: The plugin module's unit tests pass, and so does the application's whole suite, scenario tests included, with their assertions unchanged. The application's package dependency list contains no Docker SDK package, and the CI example loop builds and vets the plugin module on the minimum Go version.

#### - [ ] Task: Create the plugin module with its entity types and Docker SDK client
**Id:** 0f1bffe1-cc6d-4c3a-a390-b5af705028cf
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Give the Docker plugin its own Go module and move the parts that need nothing new. The block types go into an `entities` package that imports only xcl's types. The narrow Docker SDK interface, with `New` and `Ping`, goes into its own client package with its generated double. Add the plugin's Mockery configuration and a Makefile with build, test and generate targets, so the module builds and regenerates its doubles on its own.

*Technical detail:* [context.md#task-create-the-plugin-module-with-its-entity-types-and-docker-sdk-client](./context.md#task-create-the-plugin-module-with-its-entity-types-and-docker-sdk-client)

**Acceptance criteria**:
- [ ] The plugin directory is a Go module of its own that points at the local xcl.
- [ ] The network, container and nested network-attachment block types live in a package named `entities`, with the same fields, tags and doc comments, and that package depends on nothing but xcl's types.
- [ ] The Docker SDK interface, its real client and its generated double live in the plugin's client area, and their existing tests pass there.
- [ ] Regenerating the doubles from the plugin directory reproduces the committed ones.

#### - [ ] Task: Add the container task layer over the Docker SDK client
**Id:** 0c49ee6a-7499-4e09-aeb0-c656ae0f9894
**Repo:** xcl
**Depends on:**
- 0f1bffe1-cc6d-4c3a-a390-b5af705028cf — Create the plugin module with its entity types and Docker SDK client
**Execution:** agent

Add the plugin's container task layer. It is the one place the plugin turns its intent into Docker SDK calls: networks, image pulls, containers, attachments and addresses, using its own data types and a "not found" sentinel. It is built on the SDK interface and tested against that interface's strict double. Its tests take over every Docker-call assertion the provider tests make today, so the exact calls the plugin sends Docker are pinned in one place.

*Technical detail:* [context.md#task-add-the-container-task-layer-over-the-docker-sdk-client](./context.md#task-add-the-container-task-layer-over-the-docker-sdk-client)

**Acceptance criteria**:
- [ ] The task layer makes exactly the Docker calls the providers made before, with the same options, labels, mounts, environment ordering and forced removals.
- [ ] Docker's "not found" errors come back recognisable as the task layer's own not-found sentinel, and other errors come back unchanged.
- [ ] The task layer has its own generated strict double for callers' tests.
- [ ] The task layer depends on neither the entity types nor the providers.

#### - [ ] Task: Move the providers onto the task layer
**Id:** 2e3b4e15-469c-4b06-9c39-8f1e81f55c87
**Repo:** xcl
**Depends on:**
- 0c49ee6a-7499-4e09-aeb0-c656ae0f9894 — Add the container task layer over the Docker SDK client
**Execution:** agent

Move the network and container providers, and their shared attachment and label helpers, into a `providers` package. Each provider is built from the task layer instead of the SDK client, so no provider imports a Docker library. Their decision and update logic, error messages and log lines are unchanged. Their unit tests move with them and use the task layer's strict double. Their real-engine tests also move with them and still skip without Docker.

*Technical detail:* [context.md#task-move-the-providers-onto-the-task-layer](./context.md#task-move-the-providers-onto-the-task-layer)

**Acceptance criteria**:
- [ ] No provider source file imports a Docker library.
- [ ] Every existing provider unit test still exists under the same name and asserts the same decision or the same set of backend actions, now as task calls.
- [ ] The real-engine provider tests pass against a Docker engine and skip without one.

#### - [ ] Task: Make the plugin type importable and serve it from its own entry point
**Id:** df9ea2cc-60d5-431f-923f-8b1bf865253a
**Repo:** xcl
**Depends on:**
- 2e3b4e15-469c-4b06-9c39-8f1e81f55c87 — Move the providers onto the task layer
**Execution:** agent

Turn the plugin type into the module's importable root package. It builds the SDK client and one shared task layer without contacting Docker, then registers both block types with their providers. Add a separate-program entry point that serves it. Remove the old combined `resources` package, the old client package and the old `main` package, so only the standard layout remains.

*Technical detail:* [context.md#task-make-the-plugin-type-importable-and-serve-it-from-its-own-entry-point](./context.md#task-make-the-plugin-type-importable-and-serve-it-from-its-own-entry-point)

**Acceptance criteria**:
- [ ] Another program can import the plugin type from the module's root package.
- [ ] The separate-program entry point builds to a plugin binary that the example application loads as before.
- [ ] Nothing remains in the plugin directory outside the locations the layout design names.

#### - [ ] Task: Point the example application at the entity types only
**Id:** f33f267a-2c4c-409c-993c-12b7a451fbd5
**Repo:** xcl
**Depends on:**
- df9ea2cc-60d5-431f-923f-8b1bf865253a — Make the plugin type importable and serve it from its own entry point
**Execution:** agent

Make the example application depend on the plugin module and import only its entity types. Replace its use of the plugin's client for the "is Docker running" check with a small standard-library check that keeps today's message and timeout. Update the application's tests to the new import paths and to build the plugin binary from the plugin module, with no change to what they assert. Drop the plugin-double generation from the application's Mockery configuration and Makefile.

*Technical detail:* [context.md#task-point-the-example-application-at-the-entity-types-only](./context.md#task-point-the-example-application-at-the-entity-types-only)

**Acceptance criteria**:
- [ ] The application's own code imports nothing from the plugin except the entity types, and its build includes no Docker library.
- [ ] Applying, planning or destroying with no Docker engine fails with the same "no Docker engine reachable" message as before.
- [ ] The application's scenario, unit and smoke tests pass with their assertions unchanged.
- [ ] The new engine check has its own tests for success, failure and an unsupported address.

#### - [ ] Task: Build and test the plugin module in CI and the end-to-end runner
**Id:** bb58e37d-cab2-4074-9c77-d13dee346a94
**Repo:** xcl
**Depends on:**
- df9ea2cc-60d5-431f-923f-8b1bf865253a — Make the plugin type importable and serve it from its own entry point
**Execution:** agent

Add the plugin module to CI's minimum-Go build-and-vet loop over the examples and to the root end-to-end suite that runs each example module's tests. Ignore its build output in git. Then the new module is checked exactly like the existing examples.

*Technical detail:* [context.md#task-build-and-test-the-plugin-module-in-ci-and-the-end-to-end-runner](./context.md#task-build-and-test-the-plugin-module-in-ci-and-the-end-to-end-runner)

**Acceptance criteria**:
- [ ] CI builds and vets the plugin module on the minimum supported Go alongside the other examples.
- [ ] The root end-to-end suite runs the plugin module's tests and fails naming it when they fail.
- [ ] Building the plugin leaves no untracked output for git to pick up.

### Milestone 2: The Docker plugin reads, decides and is tested the way the standard describes

**What changes**: Every source file in the plugin example now lists its public surface first and its private helpers last, and provider methods follow one lifecycle order. That covers the Docker plugin, the in-process template plugin and the application. The network's change decision takes the standard's explicit form, with its replace-only settings listed in one place and the same outcomes as before. The plugin gains what the standard expects a plugin to carry: a sample configuration, an end-to-end test that applies it, confirms nothing changes on the next plan and destroys it, and a README explaining how to build, test, regenerate doubles and register it. This is mostly internal shape, and it is worth its own milestone because it is what makes the example a faithful copy of the standard the plugin template follows.

**Validation point**: All unit tests still pass. The plugin's end-to-end test passes against a Docker engine and skips without one. A file-by-file read confirms the order, and the plugin directory holds every location the layout design names.

#### - [ ] Task: Put every plugin example file in the standard order
**Id:** 6333ec79-2f31-4bac-91d5-9422aaff8eda
**Repo:** xcl
**Depends on:**
- f33f267a-2c4c-409c-993c-12b7a451fbd5 — Point the example application at the entity types only
**Execution:** agent

Reorder every Go source file in the plugin example: the Docker plugin, the in-process template plugin and the application. Exported types, vars and consts come first. Provider files follow with the provider type, its assertion and constructor, and then the lifecycle methods in order. Unexported helpers come next, and unexported vars and consts last. Make the network's change decision explicit in the standard's form, with its replace-only settings in one list. Nothing else about the logic changes.

*Technical detail:* [context.md#task-put-every-plugin-example-file-in-the-standard-order](./context.md#task-put-every-plugin-example-file-in-the-standard-order)

**Acceptance criteria**:
- [ ] In every source file of the plugin example, exported types and methods appear before any unexported helper.
- [ ] Every provider lists its methods in the order Init, Create, Read, Changed, Update, Destroy, Functions.
- [ ] The network answers replace for a subnet change, update for any other setting change and no change otherwise, as before, with its replace-only settings listed in one place.
- [ ] All existing tests pass unchanged.

#### - [ ] Task: Add a sample configuration and end-to-end test to the plugin
**Id:** 44018226-dd72-48f1-b1f1-6e7960a89380
**Repo:** xcl
**Depends on:**
- df9ea2cc-60d5-431f-923f-8b1bf865253a — Make the plugin type importable and serve it from its own entry point
**Execution:** agent

Give the plugin a sample configuration of a network and a container attached to it, and an end-to-end test. The test builds the plugin binary, applies the sample through a host, confirms the next plan reports no changes and destroys everything. It skips when no Docker engine is reachable, and its Docker object names don't clash with the application's tests running alongside.

*Technical detail:* [context.md#task-add-a-sample-configuration-and-end-to-end-test-to-the-plugin](./context.md#task-add-a-sample-configuration-and-end-to-end-test-to-the-plugin)

**Acceptance criteria**:
- [ ] With a Docker engine, the end-to-end test applies the sample, sees no changes on the next plan and leaves nothing behind after destroying.
- [ ] Without a Docker engine, the end-to-end test skips rather than fails.
- [ ] The sample and the application's configurations can be applied at the same time without name clashes.

#### - [ ] Task: Write a README for the plugin
**Id:** e3de9ca1-a14d-40d4-8596-fc8e734fed15
**Repo:** xcl
**Depends on:**
- 44018226-dd72-48f1-b1f1-6e7960a89380 — Add a sample configuration and end-to-end test to the plugin
**Execution:** agent

Write a README for the plugin. It explains what the plugin provides, how its parts are laid out, how to build, test and regenerate its doubles with one command each, and how to register it in-process or as a separate program. It points at the layout design as the shared reference.

*Technical detail:* [context.md#task-write-a-readme-for-the-plugin](./context.md#task-write-a-readme-for-the-plugin)

**Acceptance criteria**:
- [ ] The README names each part of the plugin and where it lives, matching the directory.
- [ ] Every command the README shows works from the plugin directory.
- [ ] The README shows both ways to register the plugin.

### Milestone 3: The documentation shows the example's new layout

**What changes**: The repository README, the plugin guides and the documentation site's plugin example page, along with the other site pages that quote the Docker plugin, point at the plugin's new file locations and show its code as it now reads. That includes how the application imports only the entity types and where the generated doubles live. Readers following the docs land on files that exist.

**Validation point**: Every Docker-plugin path quoted in the README, the guides and the site pages exists in the rebuilt example, and the documentation site builds.

#### - [ ] Task: Update the README and guides to the new layout
**Id:** 697bd2b2-b9ee-4aaa-bdd9-89cf567e1021
**Repo:** xcl
**Depends on:**
- 6333ec79-2f31-4bac-91d5-9422aaff8eda — Put every plugin example file in the standard order
- e3de9ca1-a14d-40d4-8596-fc8e734fed15 — Write a README for the plugin
**Execution:** agent

Update the repository README and the plugin guides wherever they link to or quote the Docker plugin. Point them at the new entity, provider, client and entry-point locations, show the moved code as it now reads, and say that the application imports only the entity types. The standard layout write-up itself belongs to the plugin template spec and is not added here. Add this spec's changelog entry at the top of `CHANGELOG.md`.

*Technical detail:* [context.md#task-update-the-readme-and-guides-to-the-new-layout](./context.md#task-update-the-readme-and-guides-to-the-new-layout)

**Acceptance criteria**:
- [ ] Every Docker-plugin link and quoted file in the README and guides resolves to a file in the rebuilt example.
- [ ] Code quoted from the plugin matches the rebuilt source.
- [ ] The README describes the application reading state through the entity types alone.
- [ ] `CHANGELOG.md` has one top entry headed `## 20261009102138-48e95432-docker-example-standard-layout`, prose first, then a **Breaking:** list naming the Docker plugin's move to its own module and import path and the removal of its old `resources/`, `client/` and `main.go`.

#### - [ ] Task: Update the documentation site pages to the new layout
**Id:** 19027dee-481d-42d8-aa1e-d5a84a190374
**Repo:** xcl-website
**Depends on:**
- 6333ec79-2f31-4bac-91d5-9422aaff8eda — Put every plugin example file in the standard order
- e3de9ca1-a14d-40d4-8596-fc8e734fed15 — Write a README for the plugin
**Execution:** agent

Update the site's plugin example page, and the replacement and plugin-logging pages that quote the Docker plugin, to the new file locations and the code as it now reads. That includes the task layer, the generated doubles' new home, the separate entry point and the application importing only entity types. Then confirm the site still builds.

*Technical detail:* [context.md#task-update-the-documentation-site-pages-to-the-new-layout](./context.md#task-update-the-documentation-site-pages-to-the-new-layout)

**Acceptance criteria**:
- [ ] Every code block title and path on the site that names a Docker plugin file names one that exists in the rebuilt example.
- [ ] Quoted plugin code matches the rebuilt source.
- [ ] The site builds without errors.

## Open Questions

None that can't be settled now. Every open choice (module boundary, client layering, the engine check, test inputs, docs scope) was decided during planning and recorded as a drafting assumption.

One implementation-time condition is worth stating. The plan reads "the rebuild changes only layout and ordering" as "no observable behaviour change". It moves the Docker SDK translation into a new `client/containers` task layer on that basis. If porting a provider shows that some Docker call, its order or an error message cannot be kept identical behind the task layer, the implementer must STOP and ask the user rather than accept the difference.

## Out of Scope

- **Restructuring the other examples.** The person example plugin, the end-to-end test fixtures, `example/configonly` and `example/prettylog` keep their current shape (spec Non-Goals).
- **Restructuring the in-process template plugin into the layout.** It is only reordered to the file order. It stays a single package compiled into the application.
- **Porting jumppad's resources to xcl plugins.** That is later work (spec Non-Goals).
- **Writing the standard plugin layout into the guides, and the plugin template itself.** Both belong to spec `20261009092551-82db0140-plugin-template` in the same epic.
- **Installing the Docker plugin from a remote registry or releasing it.** The example keeps loading its locally built binary through the local registry. GitHub release installs are spec `20261009102148-7d0b205b-github-releases-registry`.
- **Any change to the plugin contract, the block types' settings or the plugin's behaviour.** This includes making `Read` look up the real Docker objects, which the design describes but which would change behaviour; `Read` keeps copying saved IDs as today.
- **Running the plugin's e2e against other runtimes such as Podman.** The design allows it, but the example targets Docker only.
- **Behaviour jumppad's engine provides outside any resource** (implicit image cache, registry merging). The design leaves this out of the layout.
