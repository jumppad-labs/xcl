---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Plan: 20261008194959-b128e508-update-property-changes

<!-- Metadata -->
<!-- Created: 2026-10-08T20:27:21Z -->
<!-- Commit: 9a0c46ae709b6ab02f632d7d443a12349e88bba3 -->
<!-- Branch: f-diff -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

When a plugin decides a resource can be changed in place, xcl today hands its update only the new settings, so the plugin can't tell what changed or what the values used to be. This plan tells plugins exactly which settings changed, with their previous and new values, when deciding and when updating. The update is also told which dependencies are being updated or replaced. It does this with the same comparison the plan already uses, so a plugin is told exactly what the plan shows, with real values for sensitive settings and for values worked out during the apply.

The Docker plugin example shows it working end to end:
- it hot swaps a container's networks in place;
- it keeps the container through a network rebuild;
- it rebuilds the container only when its init script's producer is rebuilt.

Plugin authors can write precise in-place updates without extra calls to rediscover state, and people applying configuration get fewer needless rebuilds.

## Conventions

- **Go code style (gofmt, vet, `any`, descriptive names)** — applies to all new and changed Go code: `entity.PropertyChange`, the path helpers, the converters, the core change plumbing and the Docker provider changes.
- **Public library packages live at the module's top level** — `PropertyChange` and the moved `Path` go in the existing top-level `entity` package, with `diff` aliasing `Path`. Nothing goes under `/pkg`.
- **Testing & mocking: testify `require`, Mockery, no table-driven tests, positive and negative cases in separate functions, tests next to their source, no tests that inspect repository files** — applies to every task's tests. The `ProviderAdapter` mock and the example's Docker client mock are regenerated with Mockery. Documentation and the changelog are checked by review, never by tests.
- **Generate test state with a real apply** — every core test of what `Changed` and `Update` are told, and every example scenario, gets its saved state from a first apply (`applyAndSave`, `applyExampleDir`), never from a hand-written state file. This is also a spec constraint.
- **Shared test helpers live in `internal/testutil`** — any helper that turns out to be needed by more than one package goes there. The planned helpers (the recording provider's log reader, and the core test plugin's getters) are each used by one package, so they stay local.
- **Assert ordering on graph parents, not provider call order** — this applies only if a test checks that the network's destroy (force-detach) comes before the container's update. Those resources are linked, so call order is valid there. No other ordering assertions are added.
- **Never modify dependency packages** — the proto, gRPC stubs and mocks are regenerated with the pinned generators (protoc plugins at the versions in the generated headers, mockery v3.8.0). The Docker SDK is used as is.
- **Example modules cannot import xcl's internal packages (gotcha)** — the Docker example uses only `entity`, `plugins` and other public packages. Its tests keep their own helpers.
- **A custom MarshalJSON changes what state and plugins see (gotcha)** — `PropertyChange`'s masking `MarshalJSON` must never be used to carry changes to plugins. The gRPC converters encode `Before`/`After` values themselves through `wire.Marshal`/`json.Marshal` of the raw value, not by marshalling the `PropertyChange`.
- **registered.Secret / the sensitive Credential fixture (learning)** — the sensitive-change tests use the existing sensitive fixtures (`structs.Credential` in the TestPlugin, `registered.Secret` where a registered type fits) rather than a new type.
- **Include proper logging with structured logs** — any new debug logging of the change count goes through the core logger as key/value pairs, and logs only paths and counts, never values.
- **Entity is the shared vocabulary (glossary)** — doc comments in `entity` speak of entities and settings. `PropertyChange` describes a changed setting of a provider-backed resource.
- Not applied: shared errors package (no new sentinel is needed), database and external services, and patterns-and-architecture's HTTP and graceful-shutdown points (no services are involved).

## Architecture & Design Decisions

The change extends what plugins are told and leaves alone what the core decides and the order it acts in. The design `replacement-and-dependency-changes.md` (source `design`) still governs the unchanged/update/replace answer, the direct-only dependency list, and destroy-then-create. The work spans both registered repositories, as follows.

**The vocabulary (xclconfig, `entity/`).** The public, stdlib-only `entity` package gains the type that describes one changed setting, beside `Change` and `DependencyChange`, as the spec's constraints require.
- `PropertyChange{Path, Before, After, Unknown, Sensitive}`.
- Its location is structured. `diff.Path`, `Step` and `StepKind` (`diff/path.go`, stdlib only) move into `entity`, and `diff` keeps type aliases, so the plan's JSON and Go API don't change.
- Matching is a method call, not text parsing: `Path.Equal`, `Path.Within(prefix)`, and `PropertyChange.At(path)` / `PropertyChange.Within(path)`. For example, `change.Within(entity.Path{}.Attribute("network"))`.
- Values are plain JSON values (string, float64, bool, nil, `[]any`, `map[string]any`) on every path, so built-in and external plugins see identical values.
- A sensitive change carries its real values, which the spec requires. `String`, `Format`, `LogValue` and `MarshalJSON` hide them, so a stray `%v` or log line can't leak them.

**The contract (xclconfig, `plugins/`).** Both calls take the new list, and `Update` also takes the dependency list:
- `ResourceProvider[T].Changed(ctx, old, new T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)`
- `Update(ctx, resource T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (T, error)`

The same two lists thread through every layer that carries the calls, as the dependency list did in the previous change:
- `ProviderAdapter` and `TypedProviderAdapter`;
- `PluginEntityProvider`, `PluginHost` and `PluginBase`;
- the direct host and `sourcedAdapter`;
- the gRPC wrapper, server and resource adapter;
- the testing helpers and the regenerated adapter mock.

The proto carries the lists on the wire:
- a `PathStep` message (kind, attribute, index, key) and a `PropertyChange` message, whose before and after values are JSON bytes, empty meaning absent;
- `ChangedRequest.changes = 6`;
- `UpdateRequest.changes = 4` and `UpdateRequest.dependencies = 5`.

Converters live beside the existing ones in `plugins/change_proto.go`. `DefaultChanged` keeps its comparison and ignores both lists. The spec allows breaking the interface, so there's no compatibility layer. The contract change lands first, with the core passing empty lists, so the repository builds at every step. This follows the sequencing the spec's risk note asks for.

**The core computes changes once per pass, with the plan's own comparison (xclconfig, `internal/parser`).** The spec says to reuse the plan's comparison, and the success metric requires the plugin's set of paths to equal the plan's, so the core computes both from one comparison.

*Decide pass:*
- `refresh` builds the configured copy before calling `Changed` (`lifecycle.go:528` < `:577`). Right there, the core runs `resourceChanges(saved, configured, body, unknownPaths)` once, with real values.
- From that one result it derives two lists:
  - the plugin list: normalised to JSON values, with unknowns flagged and their new value absent;
  - the plan list: the same `diff.Change`s, with sensitive values blanked unless `RevealSensitive` is set.
- The plugin list goes to `Changed`. The decision record keeps the plan list for `diff.Resource.Changes`, so the plan no longer recomputes it in `recordPendingResource`.

*Act pass:*
- `update` recomputes the list with real values: the saved copy from `l.previous` against the entity decoded with no unknowns (`callbacks.go:141`). That decode already holds the results of dependencies created or changed earlier in the same apply.
- It adds back any path that was unknown at decide time, so the set still matches the plan.
- It passes the list, together with the dependency list already recorded in the decision (`decision.dependencies`), to `Update`.

Nothing else moves. Decisions are still all made before anything acts, destroys still come first, and the plugin still decides the outcome. Neither list is ever put into an event, the plan or a log: event `Data` stays the masked resource, and `logDecisions` logs no values. The core's recording test plugin captures the lists from both calls so core tests can assert them.

**The Docker example hot swaps networks (xclconfig, `example/plugin`).** Driven by the spec's example requirements:
- **Docker client.** The client interface gains `NetworkDisconnect`, and its mock is regenerated.
- **Network destroy.** The network's `Destroy` inspects the network and force-disconnects every attached container before removing it, as the spec's technical approach prefers.
- **Container `Changed`.** It answers `Replace` for a change to image, command, environment or the new `init_script`, and for any replaced dependency that is not a `docker.network`, which in practice is the init-script producer. It answers `Update` for any other change or changing dependency.
- **Container `Update`.** It works only from what it is told, as the success metric requires:
  - It rebuilds the previous attachments by applying each network change's `Before` value to the current list.
  - It disconnects the networks that went away (a not-found error is ignored, since a rebuilt or removed network has already detached it).
  - It connects the new ones.
  - It reconnects any attachment whose network dependency is replaced, matched by the address's last segment, which is the Docker network name.
  - It then reads the new IP address, which is an output, not previous state.
- **Init script.** A second template, `template "init"`, renders the script with a new optional template `mode` so it is executable. The container mounts it read-only at `/docker-entrypoint.d/90-xcl-init.sh`, which the nginx image runs at start.
- **Scenarios.** The scenarios are a network swap, a subnet rebuild, an init-script destination change and a network removal, plus the dangling-reference rejection. Each is a config variant written at test time with `writeChangedConfig`, and each runs against real Docker.

**Documentation (xclconfig `docs/`, `README.md`, `CHANGELOG.md`, `plugins/example/README.md`; xcl-website `src/pages/replacement.mdx`, `src/pages/examples/plugins.mdx`).** The docs show the new signatures and what `Changed` and `Update` are told, using the network hot swap and the init-script rebuild as worked examples. This includes replacing the website's current claim that the Docker `Update` does nothing.

**Rejected directions.** These are recorded in `research.md#alternatives-considered-and-rejected`:
- passing `diff.Change` to plugins, or having `entity` import `diff`;
- a text path;
- comparing against the post-Read bytes;
- carrying the decide-time list into `Update`;
- raw revealed values with no masking guard;
- structpb values;
- reconciling against Docker;
- rebuilding the container on any template update.

## Component Breakdown

- **Change vocabulary (changed, public `entity` package).** It already holds `Change` and `DependencyChange`, and it now also owns:
  - `PropertyChange`: one changed setting, with its structured location, its previous and new values, and flags for "not yet known" and "sensitive".
  - The structured location `Path` (with `Step` and `StepKind`), moved here from `diff`, plus its builders, `String`, `Equal` and `Within`.
  - `PropertyChange.At` and `PropertyChange.Within`.

  `PropertyChange` masks its own values when printed, logged or marshalled to JSON. The package still imports only the standard library, so the plugin contract, the parser and the diff renderer can all share it.
- **Plan diff types (changed, public `diff` package).** `Path`, `Step`, `StepKind` and the builders become aliases of the `entity` types. `diff.Change` and `diff.Resource` are otherwise unchanged and remain the plan's rendering model. Neither the plan's JSON nor `Render`'s output changes.
- **Provider contract (changed, `plugins`).**
  - `ResourceProvider[T].Changed` gains the property changes.
  - `Update` gains the property changes and the dependency list.
  - The doc comments say what each list holds at each call: at decide time, unknown values are flagged; at update time, every value is real.
  - `DefaultChanged` takes and ignores both lists.
- **Adapter and host chain (changed, `plugins`).** `ProviderAdapter`, `TypedProviderAdapter`, `PluginEntityProvider`, `PluginHost`, `PluginBase`, the direct host and its sourced adapter, and the gRPC resource adapter carry both lists unchanged from the core to the provider. The plugin-testing helpers and the generated adapter mock follow.
- **gRPC protocol (changed, `plugins/plugin.proto`, the generated `plugins/proto`, the gRPC wrapper and server, and the change converters).**
  - New `StepKind`, `PathStep` and `PropertyChange` messages.
  - `ChangedRequest` gains the property changes.
  - `UpdateRequest` gains the property changes and the dependencies.

  The converters encode each value as JSON bytes, with empty bytes meaning absent, and decode it back to the same plain values an in-process plugin receives. External and built-in plugins are therefore told the same things.
- **Change computation (changed, `internal/parser`, the existing `resourceChanges`).** It stays the single comparison of a saved copy against a configured copy. It is run with real values, and two small projections are added on top:
  - a plan projection, which blanks sensitive values unless reveal is on;
  - a plugin projection, which converts the result to `entity.PropertyChange`s with JSON-normalised values and appends any decide-time unknown path the comparison no longer reports.

  Both projections come from one result, so the plan and the plugin always list the same settings.
- **Decide pass and refresh (changed, `internal/parser`).**
  - `refresh` computes the change list from the saved copy against the configured copy, before calling Read and `Changed`, and passes the plugin projection to `Changed`.
  - The decision record keeps the plan projection, which `diff.Resource.Changes` then uses instead of a second computation.
  - Outcomes, the unknown floor, dependency lists and ordering are unchanged.
- **Decision record (changed, `internal/parser` diff recorder).** Each decision also holds the plan's change list. The decide-time unknown paths it already keeps are read back by the act pass. Dependency lists stay as recorded.
- **Act-pass update (changed, `internal/parser` lifecycle `update`).** It recomputes the plugin change list from the saved copy against the entity decoded with real values, and passes it to `Update` with the recorded dependency list. Create, replace, keep and destroy are untouched.
- **Recording test plugin (changed, `internal/parser` `TestPlugin`).** It records, per entity, the property changes and dependencies from the last `Changed` call and from the last `Update` call, and resets them with the other call records. Every core test of the new information uses it.
- **Recording provider for in-process vs external parity (new test fixture, e2e).** It is a small provider compiled into the in-process e2e fixture and into the external e2e fixture binary. It writes what `Changed` and `Update` were told to a file named in its configuration, so a test can apply the same edit through both and compare the records. It's new because no existing fixture can report its call arguments across a process boundary.
- **In-repo providers (changed).**
  - The person example, the e2e fixtures, the subtypeless fixture, the prettylog fixtures and every test fake take the new parameters.
  - Those that override `Changed` also gain a test showing they receive the changes.
- **Docker example client (changed, `example/plugin` client interface and mock).** Gains `NetworkDisconnect`. The mock is regenerated.
- **Docker network provider (changed).** `Destroy` force-disconnects every attached container, found by inspecting the network, before removing the network, so a rebuild no longer fails on active endpoints. `Changed` still replaces on a subnet change.
- **Docker container provider (changed).**
  - Gains `init_script`, bind-mounted read-only into nginx's entrypoint directory at create.
  - `Changed` replaces on image, command, environment or init-script changes, or on a replaced dependency that is not a network. Otherwise it updates.
  - `Update` works only from the changes and dependencies it is told:
    - it rebuilds the previous attachments from the network changes' previous values;
    - it disconnects the networks that went away, ignoring not-found errors;
    - it connects the new ones;
    - it reconnects attachments whose network dependency was replaced;
    - it refreshes the IP address output.
- **Template provider (changed, `example/plugin`).** Gains an optional `mode` for the rendered file, so the init script can be executable. Its replace rule (on `destination`) is unchanged.
- **Plugin example configuration and scenarios (changed).** The main configuration gains `template "init"` and the container's `init_script`. The test-time variants cover:
  - a network swap;
  - a network removal with every reference removed;
  - a dangling reference;
  - an init-script destination change.

  The existing subnet configuration now expects the container to be updated rather than replaced. The Makefile gains targets that walk through the new scenarios.
- **Documentation (changed).**
  - xclconfig: the plugin developer guide, the plugins and parser-lifecycle guides, the docs index, the README, the person example README and the changelog.
  - xcl-website: the "Unchanged, update or replace" page and the plugin example page.
  - The docs cover the new signatures and the two lists, with the network hot swap and the init-script rebuild as worked examples, and correct the claim that Docker `Update` does nothing.

## Data Structures & Interfaces

**Public, package `github.com/jumppad-labs/xcl/entity` (extended).** `Path` moves here from `diff` unchanged in shape. `PropertyChange` is new. The package stays stdlib-only.

```go
package entity

// StepKind, Step and Path: moved from diff, same fields, builders and String
type StepKind int // StepAttribute, StepIndex, StepKey
type Step struct {
	Kind      StepKind
	Attribute string
	Index     int
	Key       string
}
type Path []Step

func (p Path) Attribute(name string) Path
func (p Path) Index(i int) Path
func (p Path) Key(k string) Path
func (p Path) String() string               // network[0].name, environment["LOG_LEVEL"]
func (p Path) MarshalJSON() ([]byte, error) // the String form, as today
func (p Path) Equal(other Path) bool        // same steps
func (p Path) Within(prefix Path) bool      // equal to prefix, or below it

// PropertyChange is one setting of a resource that differs between its last
// apply and the new configuration
type PropertyChange struct {
	Path      Path // where the setting is, in configuration names
	Before    any  // previous value; nil when the setting was added
	After     any  // new value; nil when removed, or when Unknown
	Unknown   bool // the new value is only known once the apply runs (Changed only)
	Sensitive bool // Before and After are real, sensitive values
}

func (c PropertyChange) At(path Path) bool     // c.Path.Equal(path)
func (c PropertyChange) Within(path Path) bool // c.Path.Within(path)

// Masking guards: when Sensitive, these show "(sensitive)" for both values
func (c PropertyChange) String() string
func (c PropertyChange) Format(f fmt.State, verb rune)
func (c PropertyChange) LogValue() slog.Value
func (c PropertyChange) MarshalJSON() ([]byte, error)
```

`Before` and `After` always hold plain JSON values (string, float64, bool, nil, `[]any`, `map[string]any`), whether the plugin runs in-process or externally. An `Update` call never sees `Unknown` set.

**Public, package `github.com/jumppad-labs/xcl/diff` (aliases only).** `type Path = entity.Path`, `type Step = entity.Step`, `type StepKind = entity.StepKind`, and the step-kind constants re-declared from `entity`. `diff.Change`, `diff.Resource`, the plan JSON and `Render` are unchanged.

**Public, package `github.com/jumppad-labs/xcl/plugins` (breaking).**

```go
type ResourceProvider[T any] interface {
	// ... Create, Destroy, Read unchanged
	Update(ctx context.Context, resource T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (T, error)
	Changed(ctx context.Context, old, new T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)
}

func (DefaultChanged[T]) Changed(ctx context.Context, old, new T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)

// ProviderAdapter
Update(ctx context.Context, entityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) ([]byte, error)
Changed(ctx context.Context, oldEntityData, newEntityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)

// PluginEntityProvider / PluginHost
Update(ctx context.Context, entityType, entitySubType string, entityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) ([]byte, error)
Changed(ctx context.Context, entityType, entitySubType string, oldEntityData, newEntityData []byte, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)
```

**gRPC protocol, `plugins/plugin.proto` (serialization boundary, breaking).**

```proto
enum StepKind {
  STEP_KIND_ATTRIBUTE = 0;
  STEP_KIND_INDEX     = 1;
  STEP_KIND_KEY       = 2;
}

message PathStep {
  StepKind kind     = 1;
  string attribute  = 2;
  int64  index      = 3;
  string key        = 4;
}

message PropertyChange {
  repeated PathStep path = 1;
  bytes before    = 2; // JSON; empty = absent
  bytes after     = 3; // JSON; empty = absent
  bool  unknown   = 4;
  bool  sensitive = 5;
}

message ChangedRequest {
  // fields 1-5 unchanged
  repeated PropertyChange changes = 6;
}

message UpdateRequest {
  string entity_type     = 1;
  string entity_sub_type = 2;
  bytes  entity_data     = 3;
  repeated PropertyChange   changes      = 4;
  repeated DependencyChange dependencies = 5;
}
```

The converters encode each value's raw JSON themselves. They never marshal the `PropertyChange`, whose `MarshalJSON` masks sensitive values.

**Internal, package `internal/parser` (contracts between parts of the walk; names are indicative).**

```go
// the decision record gains the plan's change list
type decision struct {
	// ... action, reason, replacedDeps, dependencies, read as today
	changes []diff.Change // plan projection, masked per diff.Options
}

// one real-valued comparison, two projections
func planChanges(revealed []diff.Change, reveal bool) []diff.Change
func pluginChanges(revealed []diff.Change, unknown []diff.Path) []entity.PropertyChange
```

`refresh` gains `changes []entity.PropertyChange` as an input and passes it to `Changed`. The act pass's `update` passes `pluginChanges(...)` and `dec.dependencies` to `Update`.

**Test plugin (`internal/parser` `TestPlugin`).** It gains per-ID records of the last call's lists:
- `ChangedChanges`, read with `GetChangedChanges(id)`;
- `UpdateChanges`, read with `GetUpdateChanges(id)`;
- `UpdateDependencies`, read with `GetUpdateDependencies(id)`.

`ResetCalls` clears all three.

**Docker example (`example/plugin`).**
- The `client.Docker` interface gains `NetworkDisconnect(ctx, networkID, containerID string, force bool) error`.
- `Container` gains `InitScript string` (`xcl:"init_script,optional"`).
- `Template` gains `Mode string` (`xcl:"mode,optional"`; an octal file mode, defaulting to `0644`).

**Unchanged.** The saved-state format, event payloads (`Event.Data` stays the masked resource, with no changes), the plan JSON, and `Create`, `Read` and `Destroy`.

## Implementation Detail

**One comparison, two views.** The main new pattern in the core is computing a resource's changes once, with real values, and projecting the result twice.
- **The plan view** blanks sensitive values unless reveal is on. It is exactly what the plan showed before.
- **The plugin view** converts each change to `entity.PropertyChange`, with JSON-normalised real values.

Today the comparison runs only after `Changed` has answered and only for resources that will change. It moves earlier, to the point in `refresh` where the configured copy is built, and runs for every saved resource that reaches `Changed`. The decision record then carries the plan view, so recording a pending resource stops recomputing it. A reader sees one place where "what changed" is worked out, feeding the provider, the plan and, at act time, `Update`. That shared source is what keeps the plan and the plugin from disagreeing.

**The act pass repeats the comparison rather than carrying it.** `Update` must never see an unknown, and some values only become real once earlier resources in the same apply have run. So the act pass compares the saved copy against the freshly decoded entity, using the same function and the same plugin projection. Decide-time unknown paths are added back so the set stays aligned with the plan. The decide pass and the act pass each have one obvious line where changes are computed, and they mirror each other.

**The contract change follows the dependency-list precedent exactly.** The previous change threaded `[]entity.DependencyChange` from the core through every adapter, host, gRPC wrapper and server, the proto and the mocks. This change threads two lists along the same route, so a reader who has seen one parameter follow that path will recognise the other. The proto converters stay in the one file that already converts the change enum and dependencies.

The work is sequenced to keep the repository building:
1. Move the path type and add the change type. The `diff` aliases make this invisible to callers.
2. Break the signatures everywhere at once, with the core passing empty lists.
3. Fill the lists in the core.
4. Migrate the example behaviour.
5. Write the documentation.

**Self-masking value type.** `PropertyChange` is the first public type in `entity` that can hold a secret. Following the stdlib `slog.LogValuer` and `fmt.Formatter` idioms, it hides its values whenever it is printed, logged or marshalled, and plugin code reads the real values through the fields. This deliberately differs from `types.Sensitive`, which wraps the value itself; `entity` cannot import `types`, and plugins need plain values to compare. Because of the custom-MarshalJSON gotcha, the gRPC converters encode the fields themselves and never marshal the struct.

**Path matching moves to the public type.** The parser already compares paths with private helpers (path equality, "unknown under"). Those become `Path.Equal` and `Path.Within` on the public `entity.Path`, and the parser's helpers delegate to them. That gives plugin authors and the core one definition of "at" and "inside".

**The Docker example becomes a real in-place updater.** The container's `Update` stops being a no-op. It becomes a small reconciler over what it is told, not over what Docker reports:
- it works out the previous attachments by applying each network change's previous value to the configured list;
- it disconnects and connects the difference;
- it reconnects anything whose network dependency was rebuilt.

The network's `Destroy` takes on detaching, so rebuilding a network never fails on attached containers. The init script follows nginx's own entrypoint convention, so no command override is needed. Example tests keep their shape: config variants written at test time, applied in sequence against one state directory, then checked through the Docker API and a follow-up plan that must report no changes.

## Dependencies

- **Design documents this plan was built on:**
  - `replacement-and-dependency-changes.md` from the `design` source: the settled unchanged/update/replace answer, the direct-only dependency list and the destroy-then-create order. This plan extends what providers are told and leaves those rules as they are.
  - The spec carries no formal design references. It names this document in its constraints, so the plan treats it as binding.
- **Shipped spec `20261008132354-4538504f-replacement-deps` (merged into `f-diff`):**
  - Provides the `entity` package, the decide-then-act passes, the decision record with recorded dependency lists, and the `Changed` dependency list on every layer and on the wire.
  - It has already landed, and this plan builds directly on it.
- **Shipped diff specs (`20261007105731-2388b579-diff`, `20261007111826-cf3b66d8-diff-rendering-and-docs`):**
  - Provide `resourceChanges`, `diff.Change`, `diff.Path` and the plan rendering this plan reuses.
  - `diff.Path` moves into `entity` behind aliases. No behaviour changes.
- **Shipped masking and sensitive work (`20261003153421-9fa72edd-masking` and earlier):**
  - Provides `types.Sensitive`, `internal/wire` (real values for provider calls) and the event and state masks.
  - Unchanged; the new lists stay out of events and logs.
- **`internal/wire`:** encodes values with real sensitive content for provider calls and the gRPC converters. Unchanged.
- **Protocol Buffers and gRPC toolchain:**
  - `protoc` with `protoc-gen-go` v1.36.11 and `protoc-gen-go-grpc` v1.5.1, matching the generated headers, run through the repository's existing proto generation target.
  - Runtime `google.golang.org/protobuf` v1.36.6 and `grpc` v1.73.0, pinned in `go.mod`, with no version change.
  - Generated code is regenerated, never hand-edited.
- **Mockery v3.8.0:** regenerates the `ProviderAdapter` mock (root mockery config) and the example's Docker client mock (the example's own mockery config).
- **Docker Engine Go SDK `github.com/docker/docker` v28.5.2 (example only):**
  - Provides `NetworkDisconnect` and `NetworkInspect`.
  - It is already a dependency, so no version change is needed.
- **Docker engine (example tests only):** a running engine for the example's scenario tests. Those tests skip when it is absent, as today.
- **`nginx:1.27-alpine` image (example only):** its entrypoint runs executable scripts in `/docker-entrypoint.d/`, which the init script relies on. The image is already used.
- **xcl-website (Astro 5, MDX):** hosts the documentation pages updated by this plan. Its build and check verify them. There are no new packages.
- **Standard library `log/slog` and `fmt`:** `PropertyChange` implements `slog.LogValuer` and `fmt.Formatter` to mask sensitive values. No third-party dependency is added.
- **Nothing must land before this plan starts.** GitHub issue #8 (plugin scaffold) is a non-goal and is independent.

## Testing Approach

**Kinds of tests.**
- **Unit tests** cover:
  - the new `entity` vocabulary: the path helpers, `At`/`Within`, and the masking forms;
  - the two projections of the shared comparison;
  - the proto converters;
  - each provider that overrides `Changed`.
- **Core integration tests** drive real applies through the parser with the recording `TestPlugin`. They assert what `Changed` and `Update` were told, getting saved state only from earlier real applies.
- **Contract tests** check the two hosts against each other: the same edit is applied through the in-process and external e2e fixtures with the recording provider, and the told lists are compared.
- **End-to-end example tests** run the Docker example against a real engine. Each scenario is a config variant applied over a first apply's state, then checked through the Docker API and a follow-up plan.
- **Regression tests:** the full existing suite, every example's tests and the external test plugins must build and pass after the signature break. The tests that encode the old replace-on-network behaviour are rewritten, not deleted.

**Where coverage is heaviest, and why.** The core's change plumbing gets the most tests, because it carries the spec's correctness promises:
- the right settings with the right previous and new values;
- unknown values flagged only when deciding;
- real values at update time, including those from dependencies applied earlier;
- sensitive values real for plugins and hidden everywhere else.

The Docker example gets scenario tests, because its behaviour is the spec's user-visible proof.

**Load-bearing assertions.**
- **Edited setting.** When a setting is edited and the plugin answers update, `Update` is told exactly that setting, its location, its previous value and its new value, and nothing unchanged.
- **Dependency only.** When only a dependency is replaced and the plugin answers update, `Update` is told no setting changes and is told the replaced dependency, exactly as `Changed` was.
- **Decide.** `Changed` is told the edited setting with its previous and new values.
- **Unknown values.** A value taken from a dependency the same apply replaces is flagged not-yet-known when deciding. At update time, a value taken from a dependency created or changed earlier in the apply holds its real value.
- **Sensitive values.** A sensitive setting's change reaches the plugin with real values. The plan, the event stream and the captured logs show it hidden, and printing or logging the change hides it.
- **Matching.** A change at the first network's name matches that path and the `network` setting, and does not match `image`. These are separate positive and negative tests.
- **Parity.** In-process and external plugins are told identical change and dependency lists, when deciding and when updating.
- **Docker scenarios.**
  - A network swap keeps the container's ID and moves it to the new network.
  - A subnet rebuild keeps the container's ID and reattaches it.
  - An init-script producer replacement gives a new container ID, and the plan names the reason.
  - Removing a network and every reference leaves the container with no network.
  - A dangling reference fails validation and changes nothing.
  - Every scenario ends with a plan that reports no changes.

**Fit with existing conventions.**
- Tests use testify `require`, one behaviour per function, positive and negative cases apart, no table-driven tests, and live next to the code they test.
- Mocks are regenerated with Mockery.
- Shared helpers needed by more than one package go in `internal/testutil`.
- The example's tests use only public packages.

**Deliberate gaps.**
- No test reads the documentation or changelog; documentation is checked by review.
- No test of the generated proto code itself; the converters and the parity test cover the wire.
- The Docker scenarios don't run without an engine and skip as today. The core and contract tests carry the guarantees there.

**Success metrics.**
- **The container updates in place using only what it is told** — **behavioural test**. The container's unit tests drive `Update` against a strict Docker client mock that permits only disconnect, connect and the post-reconnect inspect for the new address; any other call, such as an inspect to discover previous state, fails the test. The real-Docker scenarios confirm the same container ID survives a network change.
- **Applies rebuild only what genuinely needs it** — **behavioural test**. The example scenarios assert that:
  - an address-range change replaces only the network and updates the container (same ID);
  - a network swap updates the container (same ID) and replaces nothing;
  - an init-script producer replacement replaces the container (new ID);
  - each scenario's plan names the replace and update counts.
- **Real Docker state matches the configuration, and the next plan reports no changes** — **behavioural test**. Each example scenario checks the network, container and attachments through the Docker API, then runs a plan and asserts "no changes".
- **The settings a plugin is told match the plan's** — **behavioural test**. Core tests run a diff and an apply on the same edit and assert that the paths in the plan equal the paths told to `Changed` and to `Update`. They cover a plain edit, a sensitive edit (values hidden in the plan, real for the plugin) and a not-yet-known value (unknown in the plan and at decide time, real at update time).

**Manual reviews.**
- **Manual — captured in the implementation test plan:** run the example's Makefile walkthroughs (network swap, subnet rebuild, init-script rebuild, network removal) against a local Docker engine, and confirm the plan and status output read correctly to a person.
- **Manual — captured in the implementation test plan:** review the updated guides and the two website pages for accuracy against the new signatures and the example code, and confirm the website builds and passes its check.

## Milestones & Tasks

### Milestone 1: Plugins have a place to be told what changed
**What changes**: The plugin contract changes shape in one step, so plugin authors can start writing against it.
- **The new type.** There is a public type for a changed setting. It has a structured location that can be matched against a setting without parsing text, and it hides sensitive values when printed or logged.
- **The new signatures.** A plugin's change decision and its in-place update both take a list of changed settings, and the update also takes the list of changing dependencies.
- **Every plugin migrates at once.** Every plugin in the repository moves to the new signatures in the same step, including the examples, the test plugins, and plugins that run as separate programs. The protocol for separate programs carries both lists.
- **The lists are still empty.** The core passes empty lists for now, so nothing behaves differently yet.
- **The plan is unaffected.** Plans look exactly as before, though the plan's own path type now comes from the shared vocabulary.

**Validation point**:
- The full test suite, every example's tests and the external test plugins build and pass.
- The path and change helpers have their own tests.
- A list of changes and dependencies given to an external plugin arrives at its provider identical to what an in-process provider receives.

#### - [ ] Task: Changed setting type and structured path
**Id:** f4bc13f8-81c5-4fbc-86d7-27680f4eda5b
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Adds the public type that describes one changed setting, beside the existing outcome and dependency types, and moves the plan's structured path type into the same shared package. The plan's diff package keeps working unchanged through aliases. Plugin authors get simple ways to check whether a change is at a setting or anywhere inside it. A sensitive change hides its values whenever it is printed, logged or turned into JSON.

*Technical detail:* [context.md#task-changed-setting-type-and-structured-path](./context.md#task-changed-setting-type-and-structured-path)

**Acceptance criteria**:
- [ ] A change at the first network's name is reported as at that name and inside the networks setting
- [ ] The same change is reported as not inside the image setting
- [ ] Printing, logging or encoding a sensitive change shows it as hidden, while its fields still hold the real values
- [ ] Plans render and encode exactly as before, and every existing diff test passes

#### - [ ] Task: Change contract through every plugin layer
**Id:** d098cb41-9d56-41ac-9b32-2089eb4756c2
**Repo:** xclconfig
**Depends on:**
- f4bc13f8-81c5-4fbc-86d7-27680f4eda5b — Changed setting type and structured path
**Execution:** agent

Changes the plugin contract so a provider's change decision receives the changed settings, and its in-place update receives the changed settings and the changing dependencies. Carries both lists through every layer between the core and a provider, including the protocol for plugins that run as separate programs, the testing helpers and the generated mock. Every plugin in the repository moves to the new signatures in the same step, so everything keeps building. For now, the core passes empty lists.

*Technical detail:* [context.md#task-change-contract-through-every-plugin-layer](./context.md#task-change-contract-through-every-plugin-layer)

**Acceptance criteria**:
- [ ] Changes and dependencies given to a plugin run as a separate program reach its provider with the same locations, values, flags and outcomes an in-process provider receives, for both the decision and the update
- [ ] Sensitive values cross to a separate program with their real values
- [ ] Values not yet known cross to a separate program still marked as not yet known
- [ ] The root module, every example module and the external test plugins build, and all their tests pass

### Milestone 2: Plugins are told exactly what changed, when deciding and when updating
**What changes**: The core now fills both lists.
- **When deciding.** A plugin asked whether a resource changed is told every setting that differs from the last apply, with its previous and new value. A value that will only be known once the apply runs is marked as not yet known.
- **When updating.** A plugin updating a resource in place is told the same settings with their real values, including values produced earlier in the same apply. It is also told which dependencies are being updated or replaced, exactly as when deciding.
- **Sensitive settings.** Plugins get real values for sensitive settings, while plans, events and logs keep hiding them.
- **Agreement with the plan.** The settings a plugin is told about are exactly the ones the plan shows.
- **Separate programs.** Plugins that run as separate programs are told the same as built-in ones.

**Validation point**: Core tests show each of the following:
- an edited setting reaches both calls with its location and values, and no unchanged setting does;
- an update with only a replaced dependency is told no setting changes and is told the dependency;
- a not-yet-known value is flagged when deciding and real when updating;
- sensitive values are real for the plugin and hidden in the plan, the events and the logs;
- the plan's paths equal the told paths.

A parity test shows an in-process and an external plugin told identical lists for the same edit.

#### - [ ] Task: One comparison feeds the plan and the plugins
**Id:** a47a3ab3-2835-403c-9b9d-59492709aa42
**Repo:** xclconfig
**Depends on:**
- d098cb41-9d56-41ac-9b32-2089eb4756c2 — Change contract through every plugin layer
**Execution:** agent

Turns the plan's existing comparison of a resource's last-applied settings against its new configuration into a single real-valued result with two views. The plan view keeps sensitive values hidden unless they are revealed, so plans stay as they are. The plugin view holds real values in plain form, the same whether the plugin runs in-process or separately. Because both views come from one result, the plan and the plugins can never list different settings.

*Technical detail:* [context.md#task-one-comparison-feeds-the-plan-and-the-plugins](./context.md#task-one-comparison-feeds-the-plan-and-the-plugins)

**Acceptance criteria**:
- [ ] The plan view of a sensitive change hides its values unless reveal is on, and the plugin view holds the real values
- [ ] The plugin view lists exactly the same settings as the plan view
- [ ] A value not yet known is marked as such in both views, with no made-up new value
- [ ] A setting that was not yet known when deciding is still listed at update time, with its real value

#### - [ ] Task: Change decisions are told the changed settings
**Id:** acf2194a-a313-427a-a7df-5ce52d3068bd
**Repo:** xclconfig
**Depends on:**
- a47a3ab3-2835-403c-9b9d-59492709aa42 — One comparison feeds the plan and the plugins
**Execution:** agent

While deciding, the core computes each saved resource's changed settings before asking its plugin, and passes them in. The decision record keeps the plan's view of the same changes, so the plan no longer works them out a second time. The recording test plugin learns to record what each decision was told.

*Technical detail:* [context.md#task-change-decisions-are-told-the-changed-settings](./context.md#task-change-decisions-are-told-the-changed-settings)

**Acceptance criteria**:
- [ ] When a resource's setting is edited, its plugin is told that setting with its previous and new values when deciding
- [ ] When a resource takes a value from a dependency the same apply replaces, the decision marks the new value as not yet known
- [ ] An unchanged resource's decision is told no changed settings
- [ ] Every plan shows the same changes as before

#### - [ ] Task: Updates are told the changed settings and dependencies
**Id:** b9d62183-7e90-4813-bb5a-528487d616de
**Repo:** xclconfig
**Depends on:**
- acf2194a-a313-427a-a7df-5ce52d3068bd — Change decisions are told the changed settings
**Execution:** agent

When the apply acts, the core works the changed settings out again from the real values, including values produced by resources created or changed earlier in the same apply. It passes them to the update along with the dependency list recorded while deciding. The recording test plugin records what each update was told. Tests confirm that the settings an update is told about match what the plan showed, and that sensitive values stay out of plans, events and logs.

*Technical detail:* [context.md#task-updates-are-told-the-changed-settings-and-dependencies](./context.md#task-updates-are-told-the-changed-settings-and-dependencies)

**Acceptance criteria**:
- [ ] When a setting is edited and the plugin answers update, the update is told exactly that setting, its location, its previous value and its new value, and no unchanged setting
- [ ] When only a dependency is replaced and the plugin answers update, the update is told no setting changes and is told the dependency is replaced
- [ ] A value from a dependency created or changed earlier in the same apply reaches the update as its real value
- [ ] A sensitive setting's change reaches the update with real values, while the plan, the event stream and the logs show it hidden
- [ ] For the same edit, the settings in the plan, in the decision and in the update are the same

#### - [ ] Task: Built-in and external plugins are told the same
**Id:** 4f6f38cd-b346-4550-bbe0-ece0afab2c9b
**Repo:** xclconfig
**Depends on:**
- b9d62183-7e90-4813-bb5a-528487d616de — Updates are told the changed settings and dependencies
**Execution:** agent

Adds a small recording provider to the end-to-end fixtures, compiled both into the in-process fixture plugin and into the external fixture program. It records what each decision and update was told. A test applies the same edit through each and compares the records, proving that plugins running as separate programs are told exactly what built-in ones are.

*Technical detail:* [context.md#task-built-in-and-external-plugins-are-told-the-same](./context.md#task-built-in-and-external-plugins-are-told-the-same)

**Acceptance criteria**:
- [ ] The same edit applied through a built-in plugin and through the same plugin run as a separate program results in identical changes and dependencies told when deciding
- [ ] The same is true when updating
- [ ] The comparison covers a plain setting, a sensitive setting and a value from a replaced dependency

### Milestone 3: The plugin example hot swaps networks and rebuilds only what it must
**What changes**: The Docker example behaves the way the spec describes:
- changing a container's network moves the running container to the new network without rebuilding it;
- rebuilding a network after an address-range change detaches the container first and reattaches the same container afterwards;
- the container gains an init script rendered by a second template, and moving that template so it has to be rebuilt rebuilds the container;
- removing a network and every reference to it deletes the network and leaves the container running with no network;
- removing the network while the container still refers to it is rejected before anything changes.

The example's update works only from what it is told, with no extra calls to discover the previous state.

**Validation point**: Against a real Docker engine, the example's scenario tests confirm each behaviour through the Docker API:
- the same or a new container ID, as expected;
- network attachments and the address range match the configuration;
- the next plan reports no changes.

The container's unit tests show its update makes only disconnect, connect and address-read calls. Without an engine, these scenario tests skip, as today.

#### - [ ] Task: Network rebuild detaches its containers
**Id:** 9c71fe48-5948-46a5-a51f-a1bc776da01d
**Repo:** xclconfig
**Depends on:**
- d098cb41-9d56-41ac-9b32-2089eb4756c2 — Change contract through every plugin layer
**Execution:** agent

Gives the example's Docker client the ability to disconnect a container from a network, and makes the network's destroy force-disconnect every container still attached before removing the network. Rebuilding a network, for example after an address-range change, then no longer fails because a container is attached.

*Technical detail:* [context.md#task-network-rebuild-detaches-its-containers](./context.md#task-network-rebuild-detaches-its-containers)

**Acceptance criteria**:
- [ ] Destroying a network with containers attached disconnects each one and then removes the network
- [ ] Destroying a network with nothing attached removes it as before
- [ ] Destroying a network that is already gone still succeeds

#### - [ ] Task: Container hot swaps its networks
**Id:** fff86a62-13ec-4f89-a1c8-4d1a9c57961a
**Repo:** xclconfig
**Depends on:**
- 9c71fe48-5948-46a5-a51f-a1bc776da01d — Network rebuild detaches its containers
- b9d62183-7e90-4813-bb5a-528487d616de — Updates are told the changed settings and dependencies
**Execution:** agent

Changes the example container's decision so that network changes, and a network being updated or rebuilt, update the container in place, while image, command and environment changes still rebuild it. Its update then works only from what it is told:
- it detaches the networks that went away;
- it attaches the new ones;
- it reattaches networks that were rebuilt;
- it reads the container's new address.

*Technical detail:* [context.md#task-container-hot-swaps-its-networks](./context.md#task-container-hot-swaps-its-networks)

**Acceptance criteria**:
- [ ] Changing a container's network answers update, and the update detaches the old network and attaches the new one
- [ ] A rebuilt network answers update, and the update reattaches the container to it
- [ ] Removing a container's only network detaches it and leaves the container with no network
- [ ] Changing the image, command or environment still answers replace
- [ ] The update makes no call to discover the container's previous state

#### - [ ] Task: Container rebuilds when its init script is rebuilt
**Id:** 42b0206c-863b-4a24-9c8f-a4c352bf74ef
**Repo:** xclconfig
**Depends on:**
- fff86a62-13ec-4f89-a1c8-4d1a9c57961a — Container hot swaps its networks
**Execution:** agent

Gives the example's container an init script, rendered by a second template in the example configuration and run by the container at start. The template gains an optional file mode so the script can be executable. When the template producing the init script is replaced, the container is replaced too, and the plan says why.

*Technical detail:* [context.md#task-container-rebuilds-when-its-init-script-is-rebuilt](./context.md#task-container-rebuilds-when-its-init-script-is-rebuilt)

**Acceptance criteria**:
- [ ] The example configuration renders an executable init script, and the container runs it at start
- [ ] A replaced init-script template makes the container answer replace
- [ ] Editing the init script's content without moving it updates the template and leaves the container alone
- [ ] A template without a mode is written as before

#### - [ ] Task: Example scenarios prove hot swap and rebuilds
**Id:** df727eec-77d7-4fcb-8ee3-dfd16cb113ee
**Repo:** xclconfig
**Depends on:**
- 42b0206c-863b-4a24-9c8f-a4c352bf74ef — Container rebuilds when its init script is rebuilt
**Execution:** agent

Rewrites and extends the example's end-to-end scenarios against real Docker. The scenarios are a network swap, an address-range rebuild, an init-script rebuild, removing a network with its references, and removing it while a reference remains. The subnet scenario's expectations move from rebuilding the container to updating it. Makefile walkthroughs are added for the new scenarios. Each scenario checks Docker directly and ends with a plan that reports no changes.

*Technical detail:* [context.md#task-example-scenarios-prove-hot-swap-and-rebuilds](./context.md#task-example-scenarios-prove-hot-swap-and-rebuilds)

**Acceptance criteria**:
- [ ] Changing the container's network from one network to another keeps the same container, attached to the new network and no longer to the old one
- [ ] The address-range change rebuilds the network and keeps the same container attached to it, and the plan shows the container updated rather than replaced
- [ ] Moving the init-script template shows the container replaced because that template is replaced, and Docker then shows a new container
- [ ] Removing the network and every reference deletes the network and leaves the container running with no network
- [ ] Removing the network while the container still references it fails validation, and nothing is created, changed or destroyed
- [ ] After each scenario that applies, the next plan reports no changes

### Milestone 4: Documentation shows plugin authors how to use the changes
**What changes**: The project's guides, the README, the changelog and the documentation site describe the new signatures and what `Changed` and `Update` are told. They use the network hot swap and the init-script rebuild as worked examples. The site no longer claims the Docker example's update does nothing.

**Validation point**: The guides and website pages match the code, as checked by review, and the documentation site builds and passes its check.

#### - [ ] Task: Guides, README and changelog describe the changes
**Id:** bf756947-f47e-4639-b46c-489a982083f3
**Repo:** xclconfig
**Depends on:**
- df727eec-77d7-4fcb-8ee3-dfd16cb113ee — Example scenarios prove hot swap and rebuilds
**Execution:** agent

Updates the project's own guides, README, the example plugin's README and the changelog. They show the new signatures, what a decision and an update are told, how to match a change to a setting, and how sensitive and not-yet-known values appear. The network hot swap and the init-script rebuild are the worked examples. The changelog lists the breaking changes to the contract and the protocol.

*Technical detail:* [context.md#task-guides-readme-and-changelog-describe-the-changes](./context.md#task-guides-readme-and-changelog-describe-the-changes)

**Acceptance criteria**:
- [ ] Every signature shown in the guides and README matches the code
- [ ] The plugin developer guide explains the changed settings and the dependency list for both calls, with the network hot swap and the init-script rebuild as examples
- [ ] The changelog has an entry for this change, with its breaking changes listed

#### - [ ] Task: Website shows the changed settings and the hot swap
**Id:** 3d5ec8b7-1ffe-4c3c-9259-becf7e3f9a67
**Repo:** xcl-website
**Depends on:**
- df727eec-77d7-4fcb-8ee3-dfd16cb113ee — Example scenarios prove hot swap and rebuilds
**Execution:** agent

Updates the documentation site's "Unchanged, update or replace" page and the plugin example page. They show what decisions and updates are told, with the network hot swap and the init-script rebuild as worked examples. The new walkthroughs are added, and the claim that the Docker example's update does nothing is replaced.

*Technical detail:* [context.md#task-website-shows-the-changed-settings-and-the-hot-swap](./context.md#task-website-shows-the-changed-settings-and-the-hot-swap)

**Acceptance criteria**:
- [ ] Both pages show the new signatures, matching the code
- [ ] The pages walk through the network hot swap and the init-script rebuild using the example's real code
- [ ] No page says the Docker example's update does nothing
- [ ] The documentation site builds and passes its check

## Open Questions

- **Whether blanking sensitive values per change reproduces today's plan output exactly for nested sensitive values.** The plan view is derived from one real-valued comparison by blanking `Before`/`After` on changes marked sensitive. That matches today only if the comparison already splits a sensitive leaf into its own change wherever it sits inside a larger added or removed value, which `whole()` and `containsSensitive` are believed to do. This depends on the comparison's behaviour for sensitive values nested in lists, maps and blocks, and it only shows up when the existing sensitive diff tests run against the new projection. If any existing plan test changes output, STOP and ask the user rather than altering the expected plan.
- **Whether the act-time comparison ever reports a path the plan did not.** The act pass recomputes changes from real values. A value the plan saw as known could, in principle, decode differently at act time, for example a function result that differs between the two parses. That would add a path to `Update` that the plan never showed, which the success metric forbids. This can only be observed by running the core tests across the existing fixtures. If it happens, STOP and ask the user whether to restrict the `Update` list to the plan's paths or to treat it as a bug in the reparse.

## Out of Scope

- **A plugin scaffold with an explicit change decision.** It is tracked separately in GitHub issue #8, probably as a GitHub template.
- **A shared default change decision that replaces on replaced dependencies.** No new built-in decision is added, and `DefaultChanged` ignores both new lists. Scaffolding an explicit one is issue #8.
- **Reconciling against the real resource or saved state.** Providers are not expected to look up previous values from Docker or from saved state, because the change list replaces that need. The Docker example reads only its new address after reconnecting.
- **Cascade delete.** Removing a resource never deletes configured resources that reference it. A dangling reference stays a validation error.
- **Changes to decide-then-act.** The ordering of decisions, destroys, creates and updates, and the plugin's ownership of the update-or-replace choice, are unchanged. So is everything else the design `replacement-and-dependency-changes.md` settles. Its open items, such as `Replace` from a failed `Update` and create-before-destroy replacement, stay open.
- **Telling `Create`, `Read` or `Destroy` about changes.** Only `Changed` and `Update` gain the lists.
- **Drift as a change.** The lists compare the last apply with the new configuration, exactly as the plan does. Differences that only `Read` observes are not listed.
- **Configuration language changes.** None. The example's new `init_script` and template `mode` are ordinary provider attributes.
- **A compatibility layer for plugins built against the current contract.** The contract and protocol break, as the spec allows. External plugins must be rebuilt.
- **The editor extension (xcl-vscode).** It needs no change.
