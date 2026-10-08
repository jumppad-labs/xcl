---
created_date: "2026-10-08"
document_status: draft
---

# Plan: 20261008132354-4538504f-replacement-deps

<!-- Metadata -->
<!-- Created: 2026-10-08T17:00:09Z -->
<!-- Commit: ea66b0981b518e39e287ddeb41797563bb1fa683 -->
<!-- Branch: f-diff -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Today, when a plugin reports that a resource changed, the change is always applied in place. Two problems follow:
- A change that can't be applied in place, such as moving the plugin example's Docker network to a new address range, is recorded in state while the real network stays as it was.
- The container attached to that network is never told.

This plan lets a plugin answer that a resource is unchanged, needs an in-place update, or must be replaced. It also tells each resource which of its dependencies the same apply will update or replace, so the resource can decide its own outcome. Apply then decides every outcome before it acts, destroys replaced and removed resources dependents-first, and only then creates and updates in dependency order. Plans run the same decision pass and say why each replacement happens.

Plugin authors get a way to express changes their resources can't take in place, and anyone applying a configuration gets real infrastructure that matches what they wrote.

## Conventions

- **Go code style (gofmt, vet, `any`, descriptive names)** — applies to all new and changed Go code: `plugins.Change`, the decision record, and the provider migrations.
- **Public library packages live at the module's top level** — `Change` and `DependencyChange` go in the public `plugins` package, and the replacement reason goes in the public `diff` package. Nothing goes under `/pkg` or the root package.
- **Testing & mocking: testify `require`, Mockery, no table-driven tests, positive and negative cases in separate functions, tests next to their source, no tests that inspect repository files** — every task's tests. The provider replace tests live beside each provider. Documentation changes are reviewed by hand, never asserted by tests.
- **Generate test state with a real apply** — replacement, failed-replacement and retry tests produce their saved state by applying with `parser.TestPlugin` (using `SetCreateError`/`SetDestroyError` for failures), never from a hand-written state file.
- **Assert ordering on graph parents, not provider call order** — the destroy-before-create test asserts call order only between the linked network and container. Any other ordering checks assert on graph parents.
- **Shared test helpers live in `internal/testutil`** — a helper needed by both root and e2e tests (for example, classifying lifecycle events into plan-like sets) goes there; helpers used by a single package stay local.
- **Never modify dependency packages** — the proto and mocks are regenerated with pinned generator versions, and no module-cache code is touched.
- **Shared error types live in the `errors` package** — if the decide pass needs a new sentinel (for example, a decision failure that callers match), it goes in `github.com/jumppad-labs/xcl/errors` with the sentinel-and-detail pattern.
- **Include proper logging with structured logs** — the decide and act phases log their counts and the reason for each replacement at debug level through the core logger, as key/value pairs.
- **Example modules cannot import xcl's internal packages (gotcha)** — the example plugin's new tests and its subnet configuration use only public packages.
- **Code that saves state itself must use `parser.EncodeForState` (gotcha)** — the destroy phase for replaced resources saves through the destroyer's existing `save`, which already uses it.
- **Entity is the shared vocabulary (glossary)** — internal code and doc comments speak of entities. `DependencyChange` lists only provider-backed resources, so its doc comment says "resource".
- Not applied: database and external services (no database is involved), and patterns-and-architecture's HTTP handler and graceful-shutdown points (no services are involved).
## Architecture & Design Decisions

The change is built on the design `replacement-and-dependency-changes.md` (source `design`), which settles the shape of the provider answer, the dependency list and the order of work. Spread across the work's two repositories, it looks like this.

**The contract (xclconfig, `plugins/`).** The public `plugins` package gains:
- `type Change int` with `NoChange`, `Update` and `Replace`, and a `String` method;
- `DependencyChange{Address, Change}`;
- the new `ResourceProvider[T].Changed(ctx, old, new T, dependencies []DependencyChange) (Change, error)`.

Every layer that carries the call passes `dependencies` in and `Change` out: `ProviderAdapter`, `TypedProviderAdapter`, the `Plugin` and `PluginHost` interfaces, `PluginBase`, the direct host and `sourcedAdapter`, the gRPC wrapper, server and resource adapter, and `plugins/plugin.proto`. The proto gains a `Change` enum, a `DependencyChange` message and `repeated DependencyChange dependencies = 5` on `ChangedRequest`. `ChangedResponse`'s `bool changed = 1` becomes `Change change = 3`, with field 1 reserved.

`DefaultChanged` keeps its comparison and now returns `Update` or `NoChange`, ignoring dependencies. Because nearly every provider in the repository embeds it, the signature change moves them all at once. The types live in `plugins` rather than reusing `diff.Action`, because the provider vocabulary has no create or delete and `plugins` must not depend on the renderer (see `research.md#alternatives-considered-and-rejected`). Breaking the interface and the protocol is allowed by the spec, so there is no compatibility shim.

**Apply becomes decide-then-act, and the decide pass is the existing diff walk (xclconfig, `internal/parser`).**
- **Decide.** `Parser.Diff` already walks the graph without acting. It reads and runs `Changed` through the shared `refresh`, and marks pending entities so their computed values become unknown to dependents. It is promoted into the single decide pass that both `Parser.Diff` and `Parser.Apply` run. This is what keeps a plan from disagreeing with the apply that follows.
- **The decision record.** The diff recorder grows into a per-entity decision record holding the outcome, the dependency changes it was told, the reason, and the decide-pass read copy. Because the DAG walk finishes every parent before its children, a resource's `dependencies` is built from its parents' recorded decisions. It contains only `Update` and `Replace` entries.
- **Which resources count as dependencies.** They are resolved from `Meta.Links`, looking through outputs, variables, modules and registered config-only types to the provider-backed resources behind them, as the user decided.
- **Outcomes without a provider call.** A resource not in state is a create. One saved `failed` or `destroy_failed` is a replace, as today.
- **Unknown inputs.** A resource whose configuration holds unknowns (its dependency will be created, updated or replaced) is still read and asked `Changed`, with each unknown replaced by its saved value. Its outcome is never lower than `Update`, because its inputs will change; the provider may still raise it to `Replace`. Without this floor, a template whose input address will change could answer "unchanged" and never re-render.
- **Decide failures.** If any Read or `Changed` fails, the apply fails before anything is destroyed, created or updated, and the previous state is returned untouched.
- **Act, step 1: destroy.** The existing `destroyer` handles everything being replaced together with everything removed, building its reverse graph from the saved links of those targets only. That gives dependents-first destroys and per-resource state saves. A destroy failure marks the resource `destroy_failed` and stops the apply, just as a failed removal does today.
- **Act, step 2: create and update.** The ordinary apply walk then runs in dependency order, with real values. The lifecycle no longer reads or compares; it follows the decision:
  - create for new and replaced resources;
  - `Update` for updated ones, with the fresh configuration and the computed values from the decide pass's Read;
  - for unchanged ones, the decide-pass copy is kept with its status.
- **What goes.** The in-walk `rebuild` disappears. Failed resources follow the same destroy-then-create road as provider-decided replacements, which is what the spec's "reuse the existing replacement path" asks for. A create failure saves `failed`, so the next apply replaces the resource again.

**Plans show the reason (xclconfig, `diff/`).** `diff.Resource` gains a reason for a replacement:
- the last apply failed;
- the provider decided;
- a dependency is replaced, with the replaced dependency addresses listed.

`Render`'s comment line above the header names the cause, for example `# docker.container.web will be replaced because docker.network.app is replaced`. This keeps the existing layout rather than the trailing comment in the design sketch; the user chose it. Unknown values keep rendering as `(known after apply)` through the existing pending/unknown hooks, which already cover updated as well as replaced dependencies.

**Plugins, example and docs.**
- **Example plugin (xclconfig, `example/plugin`).**
  - The Docker network answers `Replace` when `subnet` changes.
  - The container answers `Replace` for changes to image, command, environment or networks, and whenever a dependency is replaced.
  - The template answers `Update` for source and variables, and `Replace` for destination.
  - `alt.xcl` moves to `example/plugin/config-subnet/` so `./config` applies on its own again.
- **Person plugin.** It answers `Replace` for `first_name`/`last_name`, which its ID derives from. It is the fixture run both in-process and external to prove the two behave the same.
- **Test plugin.** `TestPlugin` records the dependencies it is told and can be set to answer any `Change`.
- **Documentation (xclconfig `docs/`, `README.md`, `CHANGELOG.md`; xcl-website `src/pages/`).** It explains unchanged, update and replace for plugin authors, on a new website guide page linked from the Guides nav. The plugin example and diff pages show a `-/+` replacement with its reason.

**Delivery order.** The work is sequenced so the repository builds at every step:
1. Change the contract everywhere, with the core mapping `Replace` onto the current rebuild and passing no dependencies yet.
2. Introduce the decide/act split and dependency lists.
3. Add the reason and rendering.
4. Migrate the example plugins.
5. Write the documentation.

Rejected directions are recorded in `research.md#alternatives-considered-and-rejected`:
- extending the in-walk `rebuild`, which destroys in create order;
- a second decision path, which could disagree with the plan;
- an acting phase that runs as serial lists instead of the DAG walker;
- tag-driven replacement;
- reusing `diff.Action`.

**Conventions that drive specific choices.**
- Tests use testify `require`, one behaviour per function, no table-driven tests, and sit next to the code they test.
- Saved state for tests comes from real applies with `TestPlugin`.
- Ordering is asserted on graph parents, or on call order only for linked resources (the network and its container).
- Mocks are regenerated with Mockery.
- The examples use only public packages.

## Component Breakdown

- **Change vocabulary (new, public `plugins` package).** Owns `Change` (`NoChange`, `Update`, `Replace`, with a `String` method) and `DependencyChange`. It is the shared language of the provider contract, the adapters, the gRPC protocol and the core's decision record. It has no dependencies beyond the standard library.
- **Provider contract (changed, `plugins`).** `ResourceProvider[T].Changed` takes the dependency list and returns a `Change`. The doc comments on `Changed` and `Update` describe what the core does with each answer.
- **`DefaultChanged` (changed, `plugins`).** Keeps its comparison of configured values (ignoring meta, `depends_on` and `disabled`) and answers `Update` or `NoChange`. It ignores dependencies, so a provider that has to react to a replaced dependency overrides `Changed` itself. Every in-repo provider that embeds it migrates with it.
- **Adapter and host chain (changed, `plugins`).** `ProviderAdapter`, `TypedProviderAdapter`, the `Plugin` and `PluginHost` interfaces, `PluginBase`, the direct host and its sourced adapter, and the gRPC resource adapter pass `dependencies` in and the `Change` out, unchanged. The plugin-testing helpers and the generated adapter mock follow suit.
- **gRPC protocol (changed, `plugins/plugin.proto` and the generated `plugins/proto`, plus `grpcPluginWrapper` and `GRPCServer`).** Carries the `Change` enum and the repeated dependency list across the process boundary. The wrapper and server convert between the proto and Go types, so external and in-process plugins see identical values.
- **Decision record (new, `internal/parser`, grown from the diff recorder).** A concurrency-safe store of each entity's decision for one operation:
  - the outcome: create, update, replace, delete or unchanged;
  - the dependency changes the entity was told;
  - the reason for a replacement (last apply failed, provider, or replaced dependencies);
  - the decide-pass read copy;
  - the pending and unknown-path information the diff recorder already keeps.

  It answers "what are this entity's changing dependencies", produces the `diff.Diff` for a plan, and gives the act pass the lists of entities to destroy and how to treat each one.
- **Dependency resolver (new, `internal/parser`).** For one entity, resolves its links to the provider-backed resources it reaches, looking through outputs, variables, modules and registered config-only types. Using the decision record, it returns the `[]plugins.DependencyChange` for those that will update or replace.
- **Decide pass (changed: today's diff walk and the lifecycle's diff step).** Runs in dependency order for both plan and apply and never acts. For each entity it:
  - assigns create or replace from saved status as today;
  - otherwise calls Read, then `Changed` with the resolved dependencies, through the shared refresh;
  - for an entity with unknown inputs, substitutes the saved values before Read and floors the outcome at `Update`;
  - marks entities that will be created, updated or replaced as pending, so their computed values are unknown to dependents.

  Any provider error stops the pass.
- **Refresh (changed, `internal/parser`).** The single Read + `Changed` step. It takes the dependency list, returns the provider's `Change`, and keeps the read copy for the act pass.
- **Act pass (changed: `Parser.Apply` and the lifecycle's apply step).** Runs only after a successful decide pass:
  1. hands every replaced and removed entity to the destroyer;
  2. runs the apply walk in dependency order, where the lifecycle follows each entity's decision (create, `Update` with fresh configuration plus carried computed values, or keep the decide copy) instead of reading and comparing.

  The in-walk rebuild is removed.
- **Destroyer (reused, `internal/parser`).** Unchanged mechanics: a reverse graph over the saved links of its targets, a destroy per entity, and a state save after each one. Its targets now include replaced entities as well as removed ones.
- **Diff result and renderer (changed, public `diff` package).** `Resource` gains the replacement reason and the replaced dependency addresses. `Render`'s comment line names the cause ("its last apply failed", "it cannot be updated in place", or "because <address> is replaced"). The JSON form carries the same fields.
- **Recording test plugin (changed, `internal/parser` `TestPlugin`).** Can be set to answer any `Change` per entity, and records the dependency list each `Changed` call received. It is used by every core replacement test.
- **In-repo providers (changed).**
  - The Docker network and container providers and the template provider in the plugin example, and the person provider in the plugin SDK example, override `Changed` with their replace rules.
  - The e2e fixtures, prettylog fixtures, subtypeless fixture and test fakes migrate through `DefaultChanged`.
  - The no-op Docker `Update`s are kept only for changes that can be made in place.
- **Plugin example configurations (changed, `example/plugin`).** The main configuration stands alone again. A new subnet-change configuration in its own directory drives the example's replacement tests and the documentation walkthrough.
- **Documentation (changed: xclconfig guides, README and changelog; xcl-website pages and nav).**
  - A plugin-author explanation of unchanged, update and replace, with the dependency list, as a new website guide page linked from the Guides menu.
  - The diff guide's replace semantics and sample output.
  - The plugin example page's subnet walkthrough.
  - The core guides that show the `Changed` signature.

## Data Structures & Interfaces

**Public, package `github.com/jumppad-labs/xcl/plugins`.** These are the design's shapes, unchanged.

```go
// Change is what applying a new configuration needs for one resource
type Change int

const (
	NoChange Change = iota // leave the resource as it is
	Update                 // call Update in place
	Replace                // Destroy the resource, then Create it again
)

func (c Change) String() string // "no change", "update", "replace"

// DependencyChange is a resource this one depends on that the same apply
// will update or replace
type DependencyChange struct {
	Address string // e.g. "docker.network.app"
	Change  Change // Update or Replace, never NoChange
}

type ResourceProvider[T any] interface {
	// ... Create, Destroy, Read, Update unchanged
	Changed(ctx context.Context, old, new T, dependencies []DependencyChange) (Change, error)
}

func (DefaultChanged[T]) Changed(ctx context.Context, old, new T, dependencies []DependencyChange) (Change, error)
```

Every byte-level layer changes the same way, gaining `dependencies` and returning `Change`:

```go
// ProviderAdapter
Changed(ctx context.Context, oldEntityData, newEntityData []byte, dependencies []DependencyChange) (Change, error)

// Plugin / PluginHost
Changed(ctx context.Context, entityType, entitySubType string, oldEntityData, newEntityData []byte, dependencies []DependencyChange) (Change, error)
```

**gRPC protocol (`plugins/plugin.proto`).** This is a serialization boundary, and the change breaks compatibility (allowed by the spec).

```proto
enum Change {
  CHANGE_NO_CHANGE = 0;
  CHANGE_UPDATE    = 1;
  CHANGE_REPLACE   = 2;
}

message DependencyChange {
  string address = 1;
  Change change  = 2;
}

message ChangedRequest {
  string entity_type      = 1;
  string entity_sub_type  = 2;
  bytes  old_entity_data  = 3;
  bytes  new_entity_data  = 4;
  repeated DependencyChange dependencies = 5;
}

message ChangedResponse {
  reserved 1;               // was: bool changed
  string error  = 2;
  Change change = 3;
}
```

**Public, package `github.com/jumppad-labs/xcl/diff`.** `Resource` gains the reason for a replacement. Both new fields are empty for every other action, and both are carried in the JSON form.

```go
// ReplaceReason says why a resource will be replaced
type ReplaceReason string

const (
	ReplaceFailed     ReplaceReason = "failed"     // its last apply failed
	ReplaceProvider   ReplaceReason = "provider"   // its provider cannot update it in place
	ReplaceDependency ReplaceReason = "dependency" // a resource it depends on is replaced
)

type Resource struct {
	Address      string        `json:"address"`
	Action       Action        `json:"action"`
	Reason       ReplaceReason `json:"reason,omitempty"`
	ReplacedDeps []string      `json:"replaced_dependencies,omitempty"` // sorted; set with ReplaceDependency
	Changes      []Change      `json:"changes,omitempty"`
}
```

`ActionReplace`'s doc comment widens to "destroyed and created again". `Render`'s comment line for a replacement reads, by reason:
- `ReplaceFailed`: `will be replaced, its last apply failed`
- `ReplaceProvider`: `will be replaced, it cannot be updated in place`
- `ReplaceDependency`: `will be replaced because a, b are replaced`

**Internal, package `internal/parser`.** These are contracts between the parts of the walk; the names are indicative.

```go
// decision is one entity's outcome from the decide pass
type decision struct {
	action       diff.Action               // create, update, replace, delete; "" = unchanged
	reason       diff.ReplaceReason
	replacedDeps []string
	dependencies []plugins.DependencyChange // what Changed was told
	read         []byte                     // decide-pass read copy (wire JSON)
}

// decisions is the decision record: the diffRecorder grown to hold a
// decision per entity ID alongside its pending/unknown bookkeeping
type decisions struct { /* mutex, map[id]decision, pending, unknown, unchanged count */ }
func (d *decisions) decide(id string, dec decision)
func (d *decisions) lookup(id string) (decision, bool)
func (d *decisions) toDestroy(previous *State) []any // replaced + removed saved entities
func (d *decisions) result() *diff.Diff              // plan; sorted, with summary

// dependencyChanges resolves an entity's links to provider-backed resources
// (looking through outputs, variables, modules, config-only types) and
// returns those the record says will update or replace
func dependencyChanges(entity any, state *State, record *decisions) []plugins.DependencyChange

// refreshOutcome gains the provider's answer
type refreshOutcome int // refreshNotFound, refreshChanged(update), refreshReplace, refreshUnchanged
func (l *resourceLifecycle) refresh(r, old any, adapter plugins.ProviderAdapter, deps []plugins.DependencyChange) (refreshOutcome, refreshed, error)
```

The walk keeps its two modes, but the meaning changes. `walkDecide` (formerly `walkDiff`) records decisions and never acts. `walkApply` is now the act walk, and it requires a completed decision record.

**Test plugin (`internal/parser` `TestPlugin`).** `ChangedResults` becomes `map[string]plugins.Change`, and `SetChangedResult(id string, change plugins.Change)`. A new `ChangedDependencies map[string][]plugins.DependencyChange` is read with `GetChangedDependencies(id)` and holds the list from the last `Changed` call for each ID.

**Unchanged.** The saved-state format (statuses, `Meta.Links`), event payloads and operations (a replace is still `destroy` followed by `create` for one ID), and the `Create`, `Read`, `Update` and `Destroy` signatures.

## Implementation Detail

**Two passes, one decision.** The main change in the parser is that `Apply` stops being one walk that reads and acts as it goes. It becomes "decide, then act", and `Diff` becomes "decide, then report".

A reader sees three thin entry points over one sequence: parse and validate, run the decide walk, then hand the decision record to a reporter (diff) or an actor (apply). The walk's mode no longer means "diff or apply". It means "decide or act", and the branches on mode stay where they are today: which lifecycle step runs, how a body is decoded, and which events fire.

The decide pass emits the read and changed lifecycle events. The act pass emits destroy, create and update. Each resource's event sequence is unchanged. Across resources, every read and changed event now comes before the first destroy, create or update, which is exactly the "nothing changes until every decision is made" guarantee, and it can be seen in the event stream.

**The diff recorder grows up rather than being replaced.** The recorder already holds the cross-resource knowledge the decide pass needs: pending entities and unknown paths. It gains one decision per entity and becomes the decision record. The existing pattern carries over unchanged: a mutex-guarded store written from concurrent walk callbacks, relying on the DAG guarantee that a parent is recorded before its children run.

Dependency lists are derived from the record at the moment a resource is decided. No second graph traversal is needed and no new ordering guarantee is introduced.

**Refresh stays the single provider-facing decision step.** It already isolates Read + `Changed` behind a small outcome type. The outcome grows a replace case, and refresh gains the dependency list as an input. Apply's old "refresh, then update" collapses into the decide pass. The act pass never calls Read or `Changed`.

This is the refactor that lets the in-walk rebuild go away. Failed-status replacements and provider-decided replacements become the same record entry, a replace, and are executed by the same destroy phase.

**The destroyer is reused as the destroy phase.** Today apply already runs the destroyer over removed resources before walking. The destroy phase simply widens its target list to include replaced resources. The destroyer's reverse graph, per-resource state saves and `destroy_failed` marking are inherited unchanged. Its existing rule that a failed destroy blocks its own dependencies' destroys still holds.

A replaced resource destroyed successfully is dropped from working state, then recreated by the act walk like any new resource, so its fresh saved copy has no stale computed values.

**Unknown inputs get saved values plus a floor.** The diff decoder already records unknown paths and substitutes placeholders. For the copy handed to a provider in the decide pass, the placeholders are replaced by the saved values at those paths. The provider then sees a meaningful "what you had, plus what's known to change", together with the dependency list that tells it why.

The core then raises a NoChange answer to Update for such a resource. This is the one place the core overrides a provider, and it never overrides towards Replace, so "the plugin decides whether to replace" holds.

**Contract migration is mechanical and lands first.** The `Change` vocabulary and the new signatures go through every layer in one change, with the core temporarily treating `Replace` as today's rebuild and passing no dependencies. Because nearly every provider embeds `DefaultChanged`, most of the migration is that one method. Hand edits are confined to the recording test plugin, the gRPC conversion, the generated code (regenerated, never hand-edited) and the callers that asserted booleans.

The proto is regenerated with generator versions pinned to those already recorded in the generated files.

**Providers express replace rules as small, readable overrides.** The example providers each gain a `Changed` method that reads as a list of rules: "if subnet differs, replace", "if any dependency is replaced, replace", "otherwise defer to `DefaultChanged`". This is the pattern the documentation teaches plugin authors.

Each rule gets its own unit test next to the provider, with no table-driven tests. Mocked Docker clients are untouched, because `Changed` needs no client.

**Renderer change is a phrase, not a layout change.** `diff.Resource` carries the reason. The renderer's existing per-action phrase function branches on it, so the output layout, markers and colours are unchanged. That keeps the user's choice of the comment-line-above layout and every existing render test except the replace phrase.

**Patterns followed and introduced.**
- Followed:
  - the walk-mode pattern and the recorder pattern from the diff work;
  - the destroyer for ordered destroys;
  - `callProvider` for provider calls and their events;
  - real-apply test state with `TestPlugin`;
  - e2e plan-versus-apply comparison through public packages;
  - examples using only public packages.
- Introduced:
  - the decision record as the contract between the two passes;
  - provider-side `Changed` overrides that read the dependency list.

## Dependencies

**Code this plan builds on**
- **`internal/parser` diff walk, recorder, refresh and decoder.** These become the decide pass and the decision record: the diff recorder is extended, refresh gains dependencies and a replace outcome, and the walk's modes change meaning. They are the core of the change.
- **`internal/parser` destroyer and destroy graph builder.** Reused as the act pass's destroy phase with a wider target list. No change to their mechanics.
- **`internal/parser` apply walk, lifecycle and progress.** The lifecycle's apply step follows decisions instead of reading and comparing, and the in-walk rebuild is removed. Progress and partial-state building are unchanged.
- **`internal/dag` walker.** Its dependency-order guarantee, concurrency and upstream-failure skipping are relied on as they are. No change.
- **`plugins` package: provider contract, `DefaultChanged`, adapters, direct host and gRPC host/server.** Changed to carry `Change` and the dependency list. This change must land first, in one step, so the repository builds.
- **`plugins/plugin.proto` and the generated `plugins/proto`.** Gain an enum, a message and new fields, and are regenerated with the generator versions already recorded in the generated files (protoc-gen-go v1.36.11, protoc-gen-go-grpc v1.5.1).
- **`diff` package.** `Resource` gains the reason fields, and `Render` phrases the reason. Everything else is unchanged.
- **`types` (statuses, `Meta.Links`), `events`, `logger`, `errors`, `state`.** Used as they are. No new statuses or event operations.
- **`internal/parser` `TestPlugin`.** Its `Changed` knobs change type, and it gains dependency recording.
- **`.mockery.yml` / the generated `plugins/mocks`.** The adapter mock is regenerated with Mockery v3.8.0, the version the plugin example already pins.
- **Example modules (`example/plugin`, `example/prettylog`, `example/configonly`) and the plugin SDK example (`plugins/example`).** They depend on the local xcl through `replace` and migrate in the same change as the contract. `example/plugin`'s Docker client and its mock are unchanged.

**External libraries**
- No new libraries. The existing pinned `google.golang.org/protobuf` and `google.golang.org/grpc` runtime versions are unchanged; only the generated code is regenerated.

**Repositories**
- **xclconfig** (`/home/nicj/code/github.com/jumppad-labs/xcl`) — all code, tests, examples, core docs and the changelog.
- **xcl-website** (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) — the new plugin-author guide page, the Guides nav entry, and updates to the diff and plugin-example pages. It is built with its existing Astro build.

**Upstream specs and plans**
- Nothing has to land first. This plan builds on the shipped diff work (plan `20261007105731-2388b579-diff`) and diff rendering (`20261007111826-cf3b66d8-diff-rendering-and-docs`), and on the registries work on the current `f-diff` branch, all of which are already in the tree.
- `example/plugin/config/alt.xcl` (commit 8f6f931) is the user's subnet configuration. This plan moves it rather than recreating it.

**Design documents this plan was built on**
- `replacement-and-dependency-changes.md` from the `design` source — the settled shape: the `Change`/`DependencyChange` contract, what a resource is told about its dependencies, decide-then-act, destroy-then-create order, and `-/+` plans showing the reason. It is binding. The one presentation detail that differs from the sketch (reason on the comment line above the header) was the user's decision during planning.
## Testing Approach

Tests follow the project's conventions throughout:
- testify `require`, one behaviour per test function, no table-driven tests;
- positive and negative cases in separate functions, with tests next to the code they test;
- saved state produced by real applies with the recording `TestPlugin` (a failing create gives `failed`, a failing destroy gives `destroy_failed`), never hand-written state files;
- ordering asserted on graph parents, with call order compared only between linked resources;
- no test that reads documentation, source or CI files.

The existing lifecycle, diff, removal and destroy suites are the regression guard. Each phase lands with them passing. Tests whose expectations the design deliberately changes are updated in the same task, with the reason given in the task: event interleaving across resources, a resource with unknown inputs now being read, and a decide-pass Read failure no longer marking the resource failed.

**Unit tests — contract layers (`plugins`).**
- `Change.String`.
- `DefaultChanged` answers `Update` or `NoChange` for each existing comparison case, and ignores dependencies.
- The typed adapter decodes both copies and passes dependencies through.
- The direct host passes dependencies and the answer through.
- The gRPC wrapper sends the dependency list and maps each proto `Change` back to the Go value, tested with a fake service client that captures the request.
- The gRPC server maps the plugin's answer and error into the response.

**Unit tests — providers.** Each in-repo provider with replace rules gets one test per rule, beside the provider:
- network `subnet` → Replace;
- container `image`, `command`, `environment` and `network` blocks → Replace each, and a replaced dependency → Replace;
- container with only an updated dependency → its own default answer;
- template `destination` → Replace, while `source` and `variables` → Update;
- person `first_name` and `last_name` → Replace, and other fields → Update.

There are also negative cases: an identical configuration gives NoChange, and a dependency that is only updated does not force a replace. These carry the "every setting it cannot change in place answers replace" success metric.

**Unit tests — decision record and dependency resolver (`internal/parser`).**
- The dependency list contains only Update and Replace dependencies.
- It lists direct references only, and looks through outputs, variables, modules and config-only types to the provider-backed resources behind them.
- The destroy target list is replaced plus removed.
- The diff result carries reason and replaced dependencies, sorted.

**Integration tests — parser apply and diff with `TestPlugin`.** These are the densest coverage, because the decide/act split carries the design. One test per acceptance criterion:
- A provider answering Replace is planned as replace and applied as destroy then create.
- An Update answer is planned and applied as an in-place update.
- A resource referencing one replaced and one updated dependency is told exactly those two with their outcomes, and is not told about an unchanged one.
- A dependent answering NoChange to a replaced dependency is neither updated nor replaced.
- With a linked network and container both replaced, the calls run container destroy, network destroy, network create, container create.
- A `Changed` error on one resource fails the apply with no create, update or destroy call, and the state is unchanged.
- A failing create of a replaced resource saves it as failed, and the next diff lists it as replace again.
- A failing destroy of a replaced resource saves `destroy_failed` and stops the apply.
- Every read and changed event precedes every destroy, create and update event.
- A resource with unknown inputs is read with saved values, and is planned and applied at least as an update.
- Failed-status replacement still works through the new destroy phase.

**Unit and integration tests — rendering (`diff`).** Each reason renders its phrase on the comment line with the `-/+` marker. The dependency reason names the dependency. The JSON form includes `reason` and `replaced_dependencies` only for replacements. Existing render tests keep their output except the replace phrase.

**Root-package tests.** `Config.Diff` and `Config.Apply` report and perform a provider-decided replacement end to end through public options, including the lifecycle event order.

**End-to-end tests (`e2e`, public packages only).**
- The plan-versus-apply helper becomes an exact comparison for the replacement scenarios.
- External and in-process providers are exercised with the same configuration change, and must produce the same diff and the same apply outcome. The person plugin from the SDK example runs it both ways, and the e2e fixtures cover the mixed graph.

**Example tests (`example/plugin`, Docker-gated as today).**
- `./config` applies alone and the existing tests pass.
- Applying `./config` then `./config-subnet` leaves Docker with the network on the new subnet, a new container ID attached to it, and the template rendered with the new container's address.
- The plan for the subnet configuration renders the network `-/+` and the container `-/+` "because docker.network.app is replaced", with the template updated.
- A plan after the apply reports no changes.

Tests that need no Docker (rendering, provider rules with the mocked client) run everywhere.

Deliberate gaps:
- There is no new test of the gRPC server in isolation beyond the mapping test, because the external-versus-in-process e2e test exercises the full protocol.
- The website and guides are not tested by code (convention).

**Success metrics**
- *After the subnet configuration is applied, the Docker network's address range matches the configuration and the next plan reports no changes* — **Behavioural test**: a Docker-gated example test applies `./config` then `./config-subnet`, inspects the real network's IPAM subnet and requires it to equal the configured value, then runs a plan of `./config-subnet` and requires "no changes". The test skips without Docker, so a run on a machine with Docker is also part of the manual walkthrough below.
- *For each plugin, every setting it cannot change in place has a test showing it answers "replace"* — **Behavioural test**: the per-provider unit tests above, one per setting: Docker network subnet; container image, command, environment, networks and replaced dependency; template destination; person first and last name. The e2e fixture providers have no in-place limits, which is recorded as a deliberate decision.
- *The plan shown before an apply lists exactly the resources the apply then destroys, creates or updates* — **Behavioural test**: the parser integration tests and the e2e plan-versus-apply comparison require, for create, update, replace, delete, dependent-replace and failed-replace scenarios, that the set of addresses per action in the diff equals the set derived from the apply's lifecycle events (destroy+create = replace), with nothing extra and nothing missing.

**Manual reviews**
- **Manual — captured in the implementation test plan**: run the plugin example's documented walkthrough against real Docker (`apply ./config`, `plan ./config-subnet`, `apply ./config-subnet`, `plan ./config-subnet`), and check that the output shown in the docs matches what the program prints, including the `-/+` lines and reason.
- **Manual — captured in the implementation test plan**: review the new website guide on unchanged/update/replace and the updated diff and plugin-example pages for accuracy against the shipped behaviour, check the page is reachable from the Guides menu, and check the site builds and `astro check` passes.
- **Manual — captured in the implementation test plan**: review the core guides (plugin developer guide, plugins, parser lifecycle, state), README and CHANGELOG entry for the new `Changed` signature, the breaking-change notes, and the decide-then-act description.

## Milestones & Tasks

### Milestone 1: Plugins answer unchanged, update or replace
**What changes**: When xcl asks a plugin whether a resource has changed, the plugin now answers that it is unchanged, needs updating in place, or must be replaced, rather than just yes or no. A "replace" answer already destroys the resource and creates it again, using the same path that failed resources take today. Every plugin in the repository, including the examples, the test plugins and plugins that run as separate programs, moves to the new answer in the same step, so the repository builds and every test passes throughout. Plugin authors see the new contract straight away. Dependents are not yet told anything about their dependencies.

**Validation point**: The full test suite, every example's tests and the external test plugins build and pass. A provider answering "replace" through both an in-process and an external plugin causes a destroy followed by a create.

#### - [ ] Task: Change contract through every plugin layer
**Id:** 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Introduces the `Change` answer (unchanged, update, replace) and the dependency list into the public plugin contract. Carries them through every layer between a provider and the core: the typed adapter, the in-process host, the gRPC protocol for external plugins, and the testing helpers. `DefaultChanged` and the recording test plugin are migrated, along with every caller that asserted a yes/no answer, so the whole repository and its examples keep building. The core honours a "replace" answer by destroying and recreating the resource on the existing failed-resource path, and passes no dependencies yet.

*Technical detail:* [context.md#task-change-contract-through-every-plugin-layer](./context.md#task-change-contract-through-every-plugin-layer)

**Acceptance criteria**:
- [ ] A provider can answer unchanged, update or replace, and the answer arrives unchanged at the core through both in-process and external plugins
- [ ] A dependency list given to an external plugin's change check arrives at the provider with the same addresses and outcomes
- [ ] `DefaultChanged` answers update when configured values differ and unchanged otherwise
- [ ] A "replace" answer during apply destroys the resource and creates it again
- [ ] The root module, every example module and the external test plugins build, and all their tests pass

### Milestone 2: Applies decide everything first, tell dependents, and replace in a safe order
**What changes**: Applying now decides the outcome of every resource before it touches any of them. Each resource's plugin is told which of the resources it references will be updated or replaced, and decides its own outcome from that. Applying then destroys everything being replaced or removed, dependents first, and only then creates and updates in dependency order. A failure while deciding changes nothing. A failed replacement is recorded as failed and is retried next time. Plans use the same decision pass, so a plan lists exactly what the apply then does.

**Validation point**: Behavioural tests show the following:
- dependents are told exactly their changing dependencies;
- a dependent may stay unchanged;
- with a network and its container both replaced, the container is destroyed, then the network, then the network is created, then the container;
- a failing decision causes no provider action;
- a failed replacement is planned as a replacement again;
- plan and apply agree in the e2e scenarios.

#### - [ ] Task: Decision record and dependency resolver
**Id:** 56c89a24-1d35-469b-ac6f-f429f2de1ddb
**Repo:** xclconfig
**Depends on:**
- 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7 — Change contract through every plugin layer
**Execution:** agent

Grows the diff recorder into a decision record that holds each entity's decided outcome, the dependencies it was told about, the reason for a replacement, and the copy read while deciding. Adds the resolver that turns an entity's references into the list of provider-backed resources it depends on that will update or replace, looking through outputs, variables, modules and config-only types. These are the building blocks the decide and act passes share.

*Technical detail:* [context.md#task-decision-record-and-dependency-resolver](./context.md#task-decision-record-and-dependency-resolver)

**Acceptance criteria**:
- [ ] A resource's dependency list contains only the provider-backed resources it references that will update or replace, each with its outcome
- [ ] A dependency reached through a module output or variable is listed as the provider-backed resource behind it
- [ ] The record lists every replaced and removed resource as the set to destroy
- [ ] Existing diff results are unchanged

#### - [ ] Task: Decide pass tells each resource about its dependencies
**Id:** a4febb1d-d732-4579-b6fd-651d403d21e0
**Repo:** xclconfig
**Depends on:**
- 56c89a24-1d35-469b-ac6f-f429f2de1ddb — Decision record and dependency resolver
**Execution:** agent

Turns the diff walk into the single decide pass shared by plans and applies. Every saved resource is read and asked whether it changed, together with the decisions already made for its dependencies, and its answer is recorded. A resource whose inputs are not yet known is read with its saved values and planned as at least an update. Any failure while deciding stops the operation before anything is touched. The recording test plugin learns to record the dependencies it is told.

*Technical detail:* [context.md#task-decide-pass-tells-each-resource-about-its-dependencies](./context.md#task-decide-pass-tells-each-resource-about-its-dependencies)

**Acceptance criteria**:
- [ ] A resource referencing a replaced dependency and an updated dependency is told about exactly those two, with their outcomes, and not about an unchanged one
- [ ] A plugin's replace answer is planned as a replacement and its update answer as an update
- [ ] A dependent whose plugin answers unchanged to a replaced dependency is planned as unchanged
- [ ] A resource whose inputs will only be known after the apply is planned as at least an update
- [ ] A failing change check fails the plan or apply without any create, update or destroy

#### - [ ] Task: Act pass destroys first, then creates and updates
**Id:** ad317131-f5a2-4b3b-a5eb-e3bbd5230d0c
**Repo:** xclconfig
**Depends on:**
- a4febb1d-d732-4579-b6fd-651d403d21e0 — Decide pass tells each resource about its dependencies
**Execution:** agent

Makes apply run the decide pass first and then act on it. Every resource being replaced or removed is destroyed together, dependents first. The apply walk then creates new and replaced resources, updates updated ones, and leaves unchanged ones alone, in dependency order. The old in-walk rebuild goes away, so resources whose last apply failed are replaced the same way. A failed destroy or create is recorded so that the next apply replaces the resource again.

*Technical detail:* [context.md#task-act-pass-destroys-first-then-creates-and-updates](./context.md#task-act-pass-destroys-first-then-creates-and-updates)

**Acceptance criteria**:
- [ ] With a network and the container on it both replaced, the container is destroyed, then the network, then the network is created, then the container
- [ ] Every read and change check in an apply happens before its first destroy, create or update
- [ ] An update answer updates the resource in place without destroying it
- [ ] When creating a replaced resource fails, it is saved as failed and the next plan lists it as a replacement again
- [ ] When destroying a replaced resource fails, it is saved as failed to destroy and the apply stops
- [ ] Resources whose last apply failed are still replaced on the next apply

#### - [ ] Task: Person plugin replaces on a name change
**Id:** d6ddcc23-ebee-4c18-afbe-4c2723905f5b
**Repo:** xclconfig
**Depends on:**
- 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7 — Change contract through every plugin layer
**Execution:** agent

The person provider in the plugin SDK example derives its ID from the first and last name, so it cannot rename a person in place. It now answers replace when either name changes and update for its other fields. This provider runs both in-process and as an external program, which makes it the fixture for proving that the two behave the same.

*Technical detail:* [context.md#task-person-plugin-replaces-on-a-name-change](./context.md#task-person-plugin-replaces-on-a-name-change)

**Acceptance criteria**:
- [ ] Changing a person's first or last name is answered as replace, through both the in-process and external plugin
- [ ] Changing any other field is answered as update, and an identical person as unchanged

#### - [ ] Task: Plans agree with applies for built-in and external plugins
**Id:** 960a7608-e9ce-4fdc-bcc6-c33a389a2d85
**Repo:** xclconfig
**Depends on:**
- ad317131-f5a2-4b3b-a5eb-e3bbd5230d0c — Act pass destroys first, then creates and updates
- d6ddcc23-ebee-4c18-afbe-4c2723905f5b — Person plugin replaces on a name change
**Execution:** agent

Proves end to end, through public packages only, that a plan lists exactly what the following apply destroys, creates and updates, including replacements decided by a plugin and replacements caused by a dependency. The same change is applied once through a built-in plugin and once through the same plugin run as a separate program, and both must give the same plan and the same outcome. Root-package tests check that the public diff and apply methods expose the replacement.

*Technical detail:* [context.md#task-plans-agree-with-applies-for-built-in-and-external-plugins](./context.md#task-plans-agree-with-applies-for-built-in-and-external-plugins)

**Acceptance criteria**:
- [ ] For every e2e scenario, the resources a plan lists per action are exactly those the following apply acts on, with nothing extra and nothing missing
- [ ] The same change through an in-process and an external plugin produces the same plan and the same apply outcome
- [ ] The public diff reports a plugin-decided replacement, and the public apply performs it

### Milestone 3: Plans say why a resource is replaced
**What changes**: A plan or diff now says why each replacement happens: its last apply failed, its plugin cannot update it in place, or a resource it depends on is being replaced, naming that resource. For example: `# docker.container.web will be replaced because docker.network.app is replaced`. The same reason is in the diff's Go types and JSON, so tools built on xcl can show or act on it. Values that a replaced or updated dependency only learns after the apply are shown as "(known after apply)".

**Validation point**: Render and JSON tests pass for each reason, and a parser diff of a dependent replacement names the dependency.

#### - [ ] Task: Diff carries and renders the replacement reason
**Id:** 6fa14271-8711-4d1d-8d7d-801eec77c80a
**Repo:** xclconfig
**Depends on:**
- a4febb1d-d732-4579-b6fd-651d403d21e0 — Decide pass tells each resource about its dependencies
**Execution:** agent

Adds the reason for a replacement to the diff result, along with the replaced dependencies behind it, so both code and the JSON form can say why. The rendered plan names the cause on the comment line above the resource, for example "will be replaced because docker.network.app is replaced". The layout and markers stay as they are.

*Technical detail:* [context.md#task-diff-carries-and-renders-the-replacement-reason](./context.md#task-diff-carries-and-renders-the-replacement-reason)

**Acceptance criteria**:
- [ ] A replacement caused by a replaced dependency renders "will be replaced because <dependency> is replaced", naming every replaced dependency
- [ ] A replacement decided by the plugin itself renders "will be replaced, it cannot be updated in place"
- [ ] A replacement of a resource whose last apply failed still renders "will be replaced, its last apply failed"
- [ ] The JSON form of a replacement includes its reason and replaced dependencies, and other actions include neither
- [ ] Values a replaced or updated dependency only learns after the apply render as "(known after apply)"

### Milestone 4: The plugin example rebuilds its network, and the docs explain replacement
**What changes**: In the plugin example:
- changing the network's address range replaces the Docker network and the container attached to it, and updates the template that reads them, so real Docker matches the configuration;
- the main configuration applies on its own again;
- the address-range change ships as a separate configuration that the documentation uses to show a plan and an apply with replacements.

The project guides and the documentation site explain how plugin authors decide unchanged, update or replace from what they are told about their dependencies, and show how plans display replacements and their reasons.

**Validation point**:
- On a machine with Docker, the example's tests pass and show the new subnet, a new container and a re-rendered template, followed by a plan with no changes.
- Without Docker, the provider rule tests pass.
- The website builds with the new guide reachable from the Guides menu.

#### - [ ] Task: Docker and template providers decide replacements
**Id:** be24536e-59ff-4655-8b6a-6c1be1a5261b
**Repo:** xclconfig
**Depends on:**
- 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7 — Change contract through every plugin layer
**Execution:** agent

Gives the example's providers real change rules:
- The Docker network answers replace when its address range changes.
- The container answers replace when its image, command, environment or networks change, or when a resource it depends on is replaced.
- The template answers replace when its destination changes, and update for its source and variables.

Each rule has its own test, so every setting a provider cannot change in place is shown to answer replace.

*Technical detail:* [context.md#task-docker-and-template-providers-decide-replacements](./context.md#task-docker-and-template-providers-decide-replacements)

**Acceptance criteria**:
- [ ] A network with a different address range is answered as replace
- [ ] A container with a different image, command, environment or network is answered as replace, as is a container whose network is replaced
- [ ] A container whose dependencies are only updated, with an identical configuration, is answered as unchanged
- [ ] A template with a different destination is answered as replace, and one with different source or variables as update

#### - [ ] Task: Plugin example ships its address-range change configuration
**Id:** 863d7ec9-41fe-4245-8ef7-a3f40e2004a3
**Repo:** xclconfig
**Depends on:**
- be24536e-59ff-4655-8b6a-6c1be1a5261b — Docker and template providers decide replacements
- ad317131-f5a2-4b3b-a5eb-e3bbd5230d0c — Act pass destroys first, then creates and updates
- 6fa14271-8711-4d1d-8d7d-801eec77c80a — Diff carries and renders the replacement reason
**Execution:** agent

Moves the second configuration, which changes the network's address range, out of the main configuration directory into its own directory beside it, so the main configuration applies on its own again. The example's tests then show the change end to end against real Docker: the plan marks the network and container as replaced and gives the reason, the apply rebuilds the network on the new range with a new container and a re-rendered template, and a following plan shows no changes.

*Technical detail:* [context.md#task-plugin-example-ships-its-address-range-change-configuration](./context.md#task-plugin-example-ships-its-address-range-change-configuration)

**Acceptance criteria**:
- [ ] Applying the main configuration on its own creates the network, container and template, and the example's existing tests pass
- [ ] The plan for the address-range configuration shows the network replaced, the container replaced because of the network, and the template updated
- [ ] After applying the address-range configuration, the Docker network has the new range, a new container is attached to it, and the template contains the new container's address
- [ ] A plan straight after that apply reports no changes

#### - [ ] Task: Core guides, README and changelog explain replacement
**Id:** cdd6c390-c856-443e-8257-f3354a8358ef
**Repo:** xclconfig
**Depends on:**
- 863d7ec9-41fe-4245-8ef7-a3f40e2004a3 — Plugin example ships its address-range change configuration
**Execution:** agent

Updates the project's own guides for plugin authors and maintainers to the new contract. They explain the unchanged/update/replace answer and the dependency list, describe how apply now decides everything and then destroys before creating, and show a plan with a replacement and its reason. The changelog gains an entry for the change that lists the breaking changes to the plugin interface, the protocol and saved state.

*Technical detail:* [context.md#task-core-guides-readme-and-changelog-explain-replacement](./context.md#task-core-guides-readme-and-changelog-explain-replacement)

**Acceptance criteria**:
- [ ] The plugin developer guide explains how to decide unchanged, update or replace using the dependency list, with an example override
- [ ] The lifecycle and state guides describe decide-then-act and the destroy-first order
- [ ] The README's plugin example section shows the address-range plan with its replacements
- [ ] The changelog has an entry for this change naming every breaking change

#### - [ ] Task: Website guide on unchanged, update or replace
**Id:** bc5085bc-0568-4d88-8d95-eb23ae9d3971
**Repo:** xcl-website
**Depends on:**
- 863d7ec9-41fe-4245-8ef7-a3f40e2004a3 — Plugin example ships its address-range change configuration
**Execution:** agent

Adds a guide page to the documentation site for plugin authors. It explains how `Changed` answers unchanged, update or replace, what a resource is told about its dependencies, and how apply decides everything before destroying and then creating. The page includes the Docker network and container rules as a worked example, and is linked from the site's Guides menu.

*Technical detail:* [context.md#task-website-guide-on-unchanged-update-or-replace](./context.md#task-website-guide-on-unchanged-update-or-replace)

**Acceptance criteria**:
- [ ] The site has a guide page explaining unchanged, update and replace for plugin authors, reachable from the Guides menu
- [ ] The page shows a provider override that reads its dependency list
- [ ] The site builds

#### - [ ] Task: Website diff and plugin example pages show replacements
**Id:** fee32990-a561-4ded-b71e-6f69966adab9
**Repo:** xcl-website
**Depends on:**
- 863d7ec9-41fe-4245-8ef7-a3f40e2004a3 — Plugin example ships its address-range change configuration
**Execution:** agent

Updates the diff guide so that "replace" covers all three reasons, with a rendered sample that shows a replacement and its reason. Updates the plugin example page to walk through the address-range change configuration: the plan with the network and container replaced, then the apply that rebuilds them. The existing walkthrough misleadingly showed an image change as an update, so it is corrected too.

*Technical detail:* [context.md#task-website-diff-and-plugin-example-pages-show-replacements](./context.md#task-website-diff-and-plugin-example-pages-show-replacements)

**Acceptance criteria**:
- [ ] The diff page explains each reason a resource is replaced and shows a rendered replacement naming its dependency
- [ ] The plugin example page shows the address-range plan and apply with the network and container replaced
- [ ] No page still shows a container image change as an in-place update
- [ ] The site builds

## Open Questions

- **Does substituting saved values at unknown paths ever produce a copy that a provider's Read rejects?** This depends on whether any in-repo provider validates a cross-field combination that a mix of saved and new values could break. Only exercising Read on the decide pass with real fixtures will show it. If a Read fails only because of the substitution, the implementer should STOP and ask the user, rather than skipping Read for such resources, since that would bring back today's behaviour that the plan deliberately drops.
- **Does Docker refuse to remove the old network while the old container is still attached, under the real destroy order?** The plan destroys the container first, so this should not happen. It depends on Docker's teardown timing after `ContainerRemove{Force}`, which only a real run shows. If the gated example test fails on network removal because of a lingering endpoint, the implementer should STOP and ask before adding retries or waits to the provider.

No other questions remain open. Every other decision is recorded in the assumption log.

## Out of Scope

- **Replacing after a failed update.** A provider cannot turn a failed in-place `Update` into a replacement; a failed update fails as it does today. This is a spec Non-Goal and listed as "Open" in the design, with no follow-up spec yet.
- **Create-before-destroy replacement.** A replacement always destroys first, so a resource that needs zero downtime is not served by this plan. This is a spec Constraint and design "Open" item, with no follow-up spec yet.
- **Cascading deletes.** Removing a resource from the configuration does not change how its dependents are decided; validation already rejects a configured resource that references a removed one. Spec Non-Goal.
- **Core-inferred replacement.** There are no struct tags, configuration markers or rules for xcl to infer replacement itself; only the provider's `Changed` decides. This was rejected during spec work.
- **Compatibility with older plugins or saved state.** There is no shim for the old yes/no `Changed` answer, no support for external plugins built against the old protocol, and no migration of state saved by the previous version. This is allowed by the spec's Constraints, and the CHANGELOG entry calls it out.
- **Plugin registration returning errors.** Whether `PluginBase.RegisterType` and `RegisterResourceProvider` should stop returning errors is a separate open question raised during spec work, outside this change.
- **The editor extension (xcl-vscode).** No configuration syntax changes, so the extension is untouched. Spec Non-Goal.
- **Drift detection in the Docker example's `Read`.** The network and container `Read` still copy only computed IDs and do not inspect Docker for out-of-band changes. The success metric's no-drift check compares state and configuration after an apply, which this plan covers; detecting manual Docker edits is not part of it.
- **Replacing the remaining no-op Docker `Update`s with real in-place updates.** Every container and network setting the example models is now a replacement. Any genuinely in-place Docker change (for example labels) is left as future example work.
