---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Plan: 20261003153421-c283547c-user-depends-on

<!-- Metadata -->
<!-- Created: 2026-10-05T10:19:04Z -->
<!-- Commit: d554c1d -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

xcl copies the dependencies it works out from references into the `depends_on` list a user writes, so after parsing the two are mixed and the author's own list cannot be shown faithfully. This plan moves create and destroy ordering onto the entity's links, which already hold every dependency: create builds its graph from them, and destroy builds the same graph with the same builder from the saved links and walks it backwards, so the separately recorded `meta.parents` is no longer needed and is removed. It then stops altering the user's list, so `depends_on` holds exactly what was written after parsing, applying and reloading, and configuration text shows it as written. Configuration authors see their own list in state, events and configuration text, while ordering still honours every dependency, and the README, the docs, the site's configuration-text guide and the changelog explain the change.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, never mix positive and negative cases, favour verbosity** — the graph tests, the destroy-order tests, the parse/apply/reload tests and the encoder tests are each separate named functions, with "list shown" and "no list written" in separate tests.
- **Generate test state with a real apply, not a hand-written state file** — the reload and destroy-order tests apply a fixture and read back what state wrote; destroy-from-saved-state tests load that state with a fresh parser.
- **New or removed `types.Meta` fields must be mirrored in the golden schema** (`gotchas/meta-field-golden-schema.md`) — removing `Meta.Parents` removes it from both places `Meta` appears in `internal/schema/test_fixtures/embedded.go`.
- **Code style: standard Go conventions, `any` over `interface{}`, descriptive names** — applies to the new `types.AppendUniqueLink` and the graph change.
- **Shared public types live in `types`** (`architecture/shared-public-types-live-in-types.md`) — the link helper stays in `types/resource_helpers.go`, where parser, root package and plugins can all reach it.
- **Nested module keys are built with `FQRN.AppendParentModule`** (`gotchas/nested-module-keys-use-append-parent-module.md`) — the graph keeps resolving each link with `AppendParentModule` when it switches from `DependsOn` to `Links`, for create and, through the same builder, for destroy from saved state.

## Architecture & Design Decisions

All Go work lands in the `xclconfig` repo: the dependency graph builder used for both create and destroy (`internal/parser/dag.go`, `internal/parser/util.go`), the destroyer and its two callers (`internal/parser/destroy.go`, `internal/parser/parser.go` `Apply` and `Destroy`), link discovery in the parser (`internal/parser/parser.go`), the entity types and helpers in `types/resource.go` and `types/resource_helpers.go`, the golden schema fixture (`internal/schema/test_fixtures/embedded.go`), the configuration-text encoder (`encode.go`), and the README, CHANGELOG, `docs/state.md`, `docs/overview.md`, `docs/parser-lifecycle.md`, `docs/plugin-developer-guide.md` and their content tests. The site's configuration-text guide in `xcl-website`, created by `20261003153421-bf87d907-references-as-written`, gains a section on written dependency lists. This plan lands after that spec, the module-boundary spec and the references-and-secrets spec, which all touch the same files.

**Ordering moves to `Meta.Links` first, then the mirror is removed.** Today the parser and the graph builder both copy every link into `DependsOn` through `types.AppendUniqueDependency` (`types/resource_helpers.go:74-111`, `internal/parser/dag.go:62-72`), and the create graph reads `DependsOn` back (`internal/parser/util.go:506-520`). Following the spec's technical direction, the first change makes `getResourceDependencies` read `Meta.Links` instead, with no other change, and gives that its own tests for create order. Only then is the mirroring removed: `AppendUniqueDependency` is replaced by `types.AppendUniqueLink`, which touches `Meta.Links` alone, and the mirror loop in `buildCreateDAG` is deleted. `Meta.Links` keeps holding every dependency, both the references xcl works out and the entries the user wrote in `depends_on`. That is deliberate: validation's undefined-reference and property stages, the evaluation context and the module boundary check from `20261003153421-6ec0eab3-module-boundary-and-output-entities` all read `Links`, so a user-written `depends_on` entry stays boundary-checked with no change to any of them.

**Destroy builds the same graph from the same links, and walks it backwards; `Meta.Parents` is removed.** Today the create graph records each resource's resolved parent IDs in `Meta.Parents` (`internal/parser/dag.go:83-104`, saved under the `parents` JSON key, `types/resource.go:51-56`), and `buildDestroyDAG` (`dag.go:121-168`) rebuilds edges from those IDs for `dag.Walker{Reverse: true}` (`internal/parser/destroy.go:52-74`). That is a second record of the same dependencies. By the user's decision, destroy instead uses the create graph's builder. `buildCreateDAG`'s body becomes one shared builder, `buildDependencyGraph(rp, addresses, nodes, root, requireParentModule)`: every entity in `nodes` is a vertex; its dependencies are resolved by `getResourceDependencies` from its `Meta.Links` (module-relative through `AppendParentModule`, a module-wide entry expanded to the module's entities, plus the module the entity sits in) against every entity `rp` holds; an edge is added from each resolved dependency whose ID is one of `nodes`, and a node with none hangs off the root. Create calls it with the whole current state as both `rp` and `nodes`, so it behaves exactly as today. Destroy calls it with the destroyer's working state as `rp` and the targets as `nodes`, and keeps walking it with `Reverse: true`. Saved state already holds everything this needs without the configuration: `Meta.Links` and `Meta.Module` are saved on every entity, and variables, outputs, modules and disabled entities are saved alongside resources (the existing destroy tests rely on it). Building over the full working state and keeping only edges between targets reproduces today's rule that parents outside the set are ignored, which Apply's removal phase relies on, while still letting each link resolve against entities that are not being destroyed. The one difference between the two uses is the parent-module lookup: create keeps failing when the module an entity sits in is missing, as today, while destroy ignores it like any other dependency not in state, because a destroy must never refuse to start over an incomplete state (recorded parents missing from state were ignored the same way). Once destroy reads links, `Meta.Parents` has no reader and is removed — the field, its `parents` JSON key, its golden-schema entries, the recording loop in the create graph and the docs that describe it. This is breaking for the saved-state format and for code reading `Meta.Parents`, and is listed under **Breaking**. State saved earlier still loads, because a saved record's unknown `parents` key is ignored by `encoding/json`, and since `links` has always been saved it is now ordered too, including state saved before `parents` existed.

**`DependsOn` holds exactly what the user wrote, from parse time on.** The parser still validates each `depends_on` entry as an address and adds its canonical form to `Links`, but it now sets `DependsOn` itself to the strings as written. The walk's decode writes the same strings again for enabled entities (`internal/parser/callbacks.go:124`), and a disabled entity, which is never decoded, keeps the list from parse time. Because `DependsOn` already travels through state, plugin calls and event data as the `depends_on` JSON key, "parsed, applied and reloaded" all hold the written list with no change to `state`, `plugins` or `internal/savedentity`. Change detection keeps ignoring `depends_on` (`plugins/changed.go:12`). State written by earlier versions loads, but may hold a polluted list until the configuration is applied again; the spec's Non-Goals do not ask for more.

**Configuration text shows a written list and never a worked-out one.** `trimBookkeeping` (`encode.go:160-175`) removes `depends_on` only when the entity's `DependsOn` is empty. Since the list now holds only what the user wrote, there is nothing worked out to hide, and `EncodeEntity` and `EncodeSavedEntity` stay byte-identical because both read the same field. `ShowReferences()` from the references-as-written plan is unaffected: `depends_on` is a list of strings, not a reference expression, so it is never recorded in `Meta.References`. The removal of `AppendUniqueDependency`, the narrower meaning of `DependsOn` and the removal of `Meta.Parents` are breaking for plugin and application code, and are listed under **Breaking** in the changelog. Rejected directions — splitting `Links`, filtering at encode time, relying on the walk's decode alone, keeping the old helper name, storing the canonical form, and keeping destroy on recorded `Meta.Parents` (the plan's previous approach) — are recorded with evidence in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Dependency graph builder (changed)**: one builder now orders both creation and destruction. It takes every dependency from the entity's links, which hold both the references xcl worked out and the entries the user wrote, and no longer reads the user's dependency list. It no longer writes anything into that list, and no longer records parents on the entity. It builds over a chosen set of entities while resolving links against everything the state holds, so it serves a whole configuration and a subset being destroyed alike. Module expansion, parent-module edges and the module-relative resolution of each link are unchanged.
- **Destroyer (changed)**: builds its graph with the shared builder from the saved links, over the working state, keeping only edges between the entities being destroyed, and walks it backwards as before. It needs the address parser, which `Apply` and `Destroy` hand it. Failure handling, saving after each entity and events are unchanged.
- **Entity metadata (changed, `types`)**: the recorded parents field is removed, so saved state no longer carries `parents`.
- **Link discovery in the parser (changed)**: still finds references in every attribute and nested block and still adds each written `depends_on` entry, validated as an address, to the entity's links. It now also sets the entity's dependency list to the `depends_on` strings exactly as written, and adds nothing else to it. The walk's decode writes the same strings later for enabled entities, so the two agree.
- **Entity link helper (`types`, changed)**: the helper that appended to both the links and the dependency list is replaced by one that appends to the links alone. The getter and setter for the dependency list stay as they are. The parser, the graph and test setup use the new helper.
- **Validation and module boundary check (unchanged)**: keep reading the entity's links. Because user-written `depends_on` entries stay in the links, they remain subject to the undefined-reference check and the module boundary check.
- **Configuration-text encoder (changed, root package)**: writes `depends_on` when the entity's dependency list is non-empty, and leaves it out otherwise. Live and saved entities give identical text, since both carry the same list. `ShowReferences()` is unaffected, because `depends_on` is not a reference expression.
- **Library documentation (changed)**: the state, overview, parser lifecycle and plugin developer guides stop describing recorded parents and say that create and destroy order both come from the entity's links, destroy walking the same graph backwards. The README's configuration-text section, the changelog and the README/CHANGELOG content tests say that a written dependency list is kept and shown as written, that worked-out dependencies are not added to it, that ordering uses both, and that saved state no longer carries `parents`.
- **Documentation site (changed, `xcl-website`)**: the configuration-text guide gains a short section on written dependency lists with an example.
- **Knowledge entry `learnings/depends-on-mirrors-links.md` (changed)**: rewritten through `spek-knowledge` to describe the new split, since it asks to be re-checked once this spec lands.

## Data Structures & Interfaces

No new types are introduced. Two existing contracts change meaning, one public helper is replaced, and one public field is removed.

**`types.ResourceBase.DependsOn` (meaning narrowed, shape unchanged).** It holds exactly the strings the user wrote in `depends_on`, in the order written, and nothing else. It is empty (nil) when the user wrote no `depends_on`. Its tags, `xcl:"depends_on,optional" json:"depends_on,omitempty"`, are unchanged, so state, plugin calls and event data carry it under the same key.

**`types.Meta.Links` (meaning unchanged, now the sole ordering source).** Every dependency of the entity: each reference xcl found in its attributes and nested blocks, and each `depends_on` entry in its canonical address form, without duplicates. The dependency graph for both create and destroy, validation (including the module boundary check) and the evaluation context read it. Entries stay module-relative, as today; the graph resolves them with the entity's `Meta.Module`, which saved state also holds.

**`types.Meta.Parents` (removed).** The field and its `json:"parents,omitempty"` key are deleted; nothing replaces them, because destroy resolves the same dependencies from `Meta.Links`. Saved state written by this version has no `parents`; state written earlier still loads, its `parents` key ignored.

**Graph builder (internal, `internal/parser/dag.go`).** One builder for both directions; create and destroy wrap it.

```go
// buildDependencyGraph builds the graph that orders nodes. Each node's
// dependencies are resolved from its Meta.Links, and the module it sits in,
// against every entity rp holds. An edge runs from each dependency that is
// itself one of nodes to the node; a node with none hangs off the root.
// When requireParentModule is false a missing parent module is ignored.
func buildDependencyGraph(rp ResourceProvider, addresses *resources.AddressParser, nodes []any, rootName string, requireParentModule bool) (*dagpkg.AcyclicGraph, error)

func buildCreateDAG(rp ResourceProvider, addresses *resources.AddressParser) (*dagpkg.AcyclicGraph, error) // rp, rp.GetResources(), "root", true
func buildDestroyDAG(rp ResourceProvider, addresses *resources.AddressParser, toDestroy []any) (*dagpkg.AcyclicGraph, error) // rp, toDestroy, "destroy_root", false
```

```go
type ResourceBase struct {
    // DependsOn holds the dependencies the user wrote in depends_on, exactly
    // as written. Dependencies xcl works out from references are never added;
    // the full set used for ordering is Meta.Links.
    DependsOn []string `xcl:"depends_on,optional" json:"depends_on,omitempty"`
    // ...Disabled, Meta unchanged
}
```

**`types.AppendUniqueLink` (new, public) replaces `types.AppendUniqueDependency` (removed).** It appends a dependency to `Meta.Links` when it is not already there, and does not touch `DependsOn`. `GetDependencies` and `SetDependencies` keep their signatures and read and write `DependsOn`.

```go
// AppendUniqueLink adds link to the entity's Meta.Links when it is not
// already present
func AppendUniqueLink(resource any, link string) error
```

**Configuration text (changed).** A block whose entity has a non-empty `DependsOn` includes `depends_on = ["…", …]` as written; one without writes no `depends_on`. `EncodeEntity` and `EncodeSavedEntity` keep their signatures and options.

```hcl
resource "app" "a" {
  depends_on = ["resource.app.b"]

  x = "resolved value of resource.app.c"
}
```

**Saved record format (breaking).** `meta.parents` is no longer written. `depends_on` now carries only the written list; state saved by an earlier version may carry worked-out entries until the configuration is applied again, and its `parents` key is ignored on load.

## Implementation Detail

**No new patterns; one responsibility is untangled.** Today one field does two jobs: it is the list a user writes and the list the graph orders by, and a helper keeps them in step by copying. After this change each job has one home. The entity's links are the ordering source and the input to every validation rule, and the dependency list is configuration the user owns, treated like any other written attribute. A developer reading the graph builder sees it take dependencies from the same links that validation and the evaluation context already read, so the three agree by construction. The same untangling applies to destroy: the dependencies were held twice, as links and as recorded parents, and now they are held once, with one builder turning them into a graph that create walks forwards and destroy walks backwards.

**Order of change matters.** The create graph is switched to the links first, while the mirror and the recorded parents still exist. At that point the sources hold the same entries, so the existing lifecycle, parents and destroy tests must pass unchanged, and new graph-level tests pin down create order for a dependency that exists only as a written entry and one that exists only as a reference. Destroy is then switched to the shared builder while `Meta.Parents` is still recorded, so the existing destroy and removal tests, which still read it, must pass unchanged, and new tests pin down destroy order from loaded saved state. Only then is `Meta.Parents` removed and the tests that read it rewritten to read the graph instead. Last, the `DependsOn` mirror is removed, so a regression in ordering cannot hide behind the change in what the dependency list holds.

**Existing patterns followed.**
- Module-relative resolution of each dependency keeps using the address type's parent-module composition, as the graph already does, now for destroy too.
- Destroy keeps the existing shape: the graph runs parent to child and is walked with the walker's `Reverse` option, followed by `TransitiveReduction` and `Validate` as today. Only where its edges come from changes.
- Membership of the destroy set is decided by entity ID, as `buildDestroyDAG` does today, so a dependency outside the set never becomes an edge.
- The encoder keeps shaping the written tree after `gohcl` writes it, in the same trim step that already drops `meta` and a false `disabled`. `depends_on` joins `disabled` as an attribute written only when it carries something.
- The parser keeps validating every written entry as an address before recording it, so a malformed entry fails at parse time exactly as before.
- Dead code is removed: the commented-out `setDependsOn` block that a TODO left in the block parser describes the work this plan does.

**Public surface UX.** A configuration author sees their `depends_on` list come back exactly as they wrote it in configuration text, saved state and event data, and sees no list when they wrote none. An application or plugin author who read `DependsOn` to learn every dependency, or `Meta.Parents` to learn resolved parents, now reads `Meta.Links` for that, and the replaced link helper is the one rename they must make. Saved state no longer carries `meta.parents`. Ordering behaves exactly as before for state written by this version, and state written before `parents` existed, which was destroyed in no particular order, is now destroyed children first too.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **`20261003153421-6ec0eab3-module-boundary-and-output-entities` (planned; must land first).** Adds the module boundary check over `Meta.Links` in validation stage 2 and the `FQRN.AppendParentModule` fix in reference resolution. This plan keeps user-written `depends_on` entries in `Meta.Links`, so they remain boundary-checked; no change to that work.
- **`20261003134528-327e0657-references-and-secrets` (planned; must land first).** Wire-encoded state and the sensitive marker in `encode.go`, and a shared state-encoding helper called from the destroyer's save (`internal/parser/destroy.go:125-140`). This plan's encoder change sits in the same trim step and does not touch sensitive handling; its destroyer change touches only graph building (`destroy.go:47-56`) and the struct's fields, not the save path. Saved records keep `links` and `module` in plain form, which destroy now relies on.
- **`20261003153421-bf87d907-references-as-written` (planned; must land first).** Adds `Meta.References`, `xcl.ShowReferences()` and creates the site's configuration-text guide (`/configuration-text/`). This plan extends that guide and the README section it edits, and relies on its decision that `depends_on` is never recorded as a reference.
- **`internal/parser` (graph builder, destroyer, link discovery).** Changed: one builder reads `Meta.Links` for create and destroy and records no parents; the destroyer builds its graph with it over the working state and gains the address parser; the parser sets `DependsOn` to the written list and stops mirroring.
- **`internal/dag`.** Unchanged; destroy keeps using `Walker{Reverse: true}`.
- **`types` package (`resource_helpers.go`, `resource.go`).** Changed: `AppendUniqueLink` replaces `AppendUniqueDependency`; `Meta.Parents` is removed; the `DependsOn` and `Links` doc comments are updated.
- **`internal/schema/test_fixtures/embedded.go`.** Changed: the `Parents` entry is removed from both places `Meta` appears.
- **Root package (`encode.go`).** Changed: `depends_on` is written when non-empty.
- **`state`, `plugins`, `internal/savedentity`, `internal/xcl`.** Unchanged. They carry `depends_on` and `links` through their existing JSON paths, and `encoding/json` ignores the `parents` key in older records; `internal/xcl` and its `UPSTREAM.md` are untouched.
- **`xcl-website` repo.** Hosts the configuration-text guide; its gate is its own build and `astro check` after `npm ci`.
- **Knowledge base (`learnings/depends-on-mirrors-links.md`).** Rewritten through the `spek-knowledge` skill once the code lands.
- **External libraries: none added.** Tests use testify `require`, already a dependency.

## Testing Approach

All tests follow the project's conventions: testify `require`, no table-driven tests, each accepted case and each rejected or "nothing written" case in its own function, and every test needing state gets it from a real `Apply` against a fixture, never from a hand-written state file.

**Graph tests (the load-bearing change).** The spec asks for the graph change to have its own tests, because it orders both create and destroy. A fixture holds an entity A that depends on B only through its written `depends_on` and on C only through a reference, plus an unrelated entity. Graph-level tests assert that A's parents in the create graph (its up-edges, root excluded) are exactly B and C. With A's dependency list emptied by hand before the graph is built, the same edges still appear, which proves the graph no longer reads the list. Lifecycle tests apply the fixture and assert that the provider is asked to create B and C before A. These run after the graph switch and again after the mirror is removed.

**Destroy from links (the user's decision).** Destroy-graph tests build the destroy graph from state loaded back from the store by a fresh parser, never from the live entities, so they prove the order comes from saved links alone: A's parents in the destroy graph are exactly B and C, A's dependency list is emptied by hand first in a separate test, and a subset that leaves out B keeps C as A's only parent. Lifecycle tests destroy the applied fixture from loaded state and assert the provider destroys A before B and C, one test for B (only in `depends_on`) and one for C (only a reference). Further tests destroy the `module_reference` fixture from loaded state and assert, from destroy events, that the consumer whose `depends_on` names the whole module goes before both of the module's resources, and that each resource inside the module goes before the module itself. A new fixture whose module holds one resource referencing another by its module-relative address proves, from loaded state, that the referencing resource is destroyed first, so module-relative links resolve from saved state. The existing destroy and removal tests (children first, never a parent of a failed child, removal of a child before its removed parent) must pass unchanged after destroy is switched, while `Meta.Parents` still exists.

**Parents removal.** Every test that read `Meta.Parents` is rewritten to read the graph instead, through a test helper that builds the graph with the shared builder over loaded saved state and returns a node's parent IDs. The parents tests keep their intent — each saved resource's dependencies, the module as parent of its resources, a module-wide `depends_on` expanding to every resource in the module, a reapply leaving them unchanged — now asserted on the graph built from saved state. The two "never destroys the parent of a failed child" tests and the subtype test take parents from the same helper. A separate test asserts saved state has no `parents` key, and a `savedentity` decode test (which, like the existing ones there, decodes a record literal rather than a state file) asserts a record still carrying `parents` decodes with the key ignored. The golden schema test passes with `Parents` removed from the fixture.

**Parse, apply and reload tests.** For the same fixture, A's dependency list holds exactly `["<B as written>"]`: after parsing, after applying, and in the entity loaded back from state, each in its own test. A disabled entity with a written `depends_on` keeps its list after parsing and in saved state. An entity with no `depends_on` has an empty list at each point, even though it holds references. A's links still hold both B and C.

**Boundary check regression.** A `depends_on` entry naming a module's internal resource is still rejected by validation, and one naming the module itself still validates. This proves user-written entries remain subject to the module boundary check now that they are no longer mirrored.

**Helper tests.** The new link helper adds a link once, ignores a duplicate, and leaves the dependency list untouched. The tests for the old helper are replaced.

**Encoder tests (acceptance criterion "output shows only written dependencies").** For applied entity A, the configuration text contains `depends_on = ["<B>"]` and does not name C in a dependency list. An entity with no written list has no `depends_on` in its text. The text from the live entity and from its saved record is byte-identical. The existing test that asserted `depends_on` is never written is replaced by these two. The existing test that bookkeeping inside an object attribute is left out stays unchanged.

**Documentation.** README and CHANGELOG content tests are extended so they fail if the README stops saying that a written `depends_on` list is shown as written and that worked-out dependencies are not added to it, or if the changelog entry for this spec, with its breaking changes (including the removal of `meta.parents`), is removed. The docs under `docs/` have no content tests; a grep for `Parents`/`meta.parents` across `docs/` and Go sources must come back empty. The site has no content tests; its build and `astro check` are the automated gate.

**Regression.** The full suite must pass, including the parser, lifecycle, parents, destroy, removal, config-destroy, registered-type, schema, encoder, plugin and example tests. Example expected output changes only where an example entity writes `depends_on` and its configuration text is printed.

**Success metrics.** The spec defines none, so there are none to verify.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: read the configuration-text guide on the `xcl-website` site in a browser and check that its new section says a written `depends_on` list is preserved and shown as written, that worked-out dependencies are not added to it, and that its example is correct.

**Deliberate gaps.** External plugin round-trips of `depends_on` are not tested separately: the field travels through the same JSON path as before, which existing plugin tests cover. State saved by earlier versions is not tested for a clean dependency list, nor for destroy order from a record that holds `parents` but unusable links, since the spec's Non-Goals drop backwards compatibility.

## Milestones & Tasks

### Milestone 1: Create and destroy order rests on every dependency, from one source

**What changes**: Entities are created and destroyed in the same order as before. The ordering stops reading the user's dependency list and takes every dependency, written or worked out from references, from the one record validation already uses. Destroy stops relying on a separately recorded list of parents: it builds the same dependency graph as create, with the same builder, from what saved state already holds, and walks it backwards. The one visible change is that saved state no longer carries `meta.parents`, and state saved before that field existed is now destroyed children first instead of in no particular order. This earns its own milestone because it is the risky half of the spec. New tests pin down create and destroy order for a dependency that exists only as a written entry and one that exists only as a reference, and destroy order from loaded saved state across modules. That makes it safe to stop altering the user's list in the next milestone.

**Validation point**: The full suite passes, with only the tests that read `Meta.Parents` rewritten to read the graph, and new tests prove that an entity depending on one entity through `depends_on` and another through a reference is created after both and, from loaded saved state, destroyed before both, even when its written list is empty; that a module-wide `depends_on` and resources inside a module destroy in the right order from saved state alone; and that saved state has no `parents`.

#### - [x] Task: Order creation from links
**Id:** d35dcc10-232f-484e-ad4e-c9becbed8a8f
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

The create graph stops reading the user's dependency list and takes every dependency from the entity's links, which already hold both references and written `depends_on` entries. New tests fix create order for an entity that depends on one entity only through `depends_on` and on another only through a reference. Destroy is untouched in this task, and nothing visible changes yet.

*Technical detail:* [context.md#task-order-creation-from-links](./context.md#task-order-creation-from-links)

**Acceptance criteria**:
- [x] An entity that depends on B only through its written list and on C only through a reference is created after B and C.
- [x] Its parents in the create graph are exactly B and C, even when its written dependency list is empty when the graph is built.
- [x] A `depends_on` naming a whole module still makes every resource in that module a parent.
- [x] Every existing test passes unchanged.

#### - [x] Task: Order destruction from links with the create graph's builder
**Id:** b3e94632-958e-4a2a-8309-5510e37f7827
**Repo:** xclconfig
**Depends on:**
- d35dcc10-232f-484e-ad4e-c9becbed8a8f — Order creation from links
**Execution:** agent

The create graph's builder becomes the one builder for both directions. Destroy builds its graph with it from the saved links of the entities being destroyed, resolving them against the whole saved state, keeps only the dependencies that are themselves being destroyed, and walks it backwards as today. Both `Destroy` from saved state and the removal phase of `Apply` use it. Recorded parents are still written but no longer read, so the existing destroy and removal tests pass unchanged.

*Technical detail:* [context.md#task-order-destruction-from-links-with-the-create-graphs-builder](./context.md#task-order-destruction-from-links-with-the-create-graphs-builder)

**Acceptance criteria**:
- [x] Destroying from loaded saved state destroys an entity before B, which it names only in `depends_on`, and before C, which it only references, even when its written dependency list is empty.
- [x] Destroying from loaded saved state destroys an entity whose `depends_on` names a whole module before every resource in that module, and each resource in a module before the module.
- [x] A resource that references another inside the same module by its module-relative address is destroyed first, from saved state alone.
- [x] When only some entities are destroyed, dependencies outside that set are ignored, and a missing parent module does not stop the destroy.
- [x] Every existing destroy and removal test passes unchanged.

#### - [x] Task: Stop recording parents in saved state
**Id:** 5078beb1-14d6-4eb4-a2d9-503409e28785
**Repo:** xclconfig
**Depends on:**
- b3e94632-958e-4a2a-8309-5510e37f7827 — Order destruction from links with the create graph's builder
**Execution:** agent

With nothing reading it, the recorded parents field is removed from entity metadata, from what the create graph writes, from saved state and from the golden schema. Tests that read it are rewritten to read the graph built from saved state, keeping what each one proves. The internal docs that describe recorded parents say instead that destroy builds the same graph from links and walks it backwards. This is a breaking change to the saved-state format and the public metadata type.

*Technical detail:* [context.md#task-stop-recording-parents-in-saved-state](./context.md#task-stop-recording-parents-in-saved-state)

**Acceptance criteria**:
- [x] Entity metadata has no parents field and saved state carries no `parents` key.
- [x] The parents, destroy, removal and subtype tests keep their intent, reading each entity's parents from the graph built from saved state.
- [x] A saved record that still carries a `parents` key decodes, the key ignored.
- [x] The state, overview, parser lifecycle and plugin developer guides no longer mention recorded parents and describe destroy as the create graph walked backwards.
- [x] The full suite passes, including the golden schema test.

### Milestone 2: A written `depends_on` list is kept and shown exactly as written

**What changes**: A `depends_on` list holds exactly what the author wrote after parsing, after applying and when loaded back from saved state. Dependencies xcl works out from references are no longer added to it, so state, plugin calls and event data carry the author's list as written. Configuration text now includes `depends_on` when the author wrote one, exactly as written, and leaves it out when they did not. Ordering still honours both kinds of dependency, and a written entry that reaches into a module is still rejected by the module boundary check. Plugin and application code that read the list to learn every dependency now reads the entity's links. The public helper that appended to both is replaced by one that appends to the links only.

**Validation point**: The full suite passes. New tests prove that the list is unchanged by parsing, applying and reloading; that configuration text shows a written list and no list when none was written, identically for live and saved entities; and that boundary-checking of written entries still holds.

#### - [x] Task: Keep the written dependency list as written
**Id:** d9669e0e-376d-455a-9177-8ea10ee3e1ea
**Repo:** xclconfig
**Depends on:**
- 5078beb1-14d6-4eb4-a2d9-503409e28785 — Stop recording parents in saved state
**Execution:** agent

The parser and the graph stop copying worked-out dependencies into the user's `depends_on` list. The parser sets the list to the strings exactly as written, and keeps adding every dependency to the entity's links so ordering and validation, including the module boundary check, still see written entries. The public helper that appended to both is replaced by one that appends to the links only. This is a breaking change for code that read the list to learn every dependency.

*Technical detail:* [context.md#task-keep-the-written-dependency-list-as-written](./context.md#task-keep-the-written-dependency-list-as-written)

**Acceptance criteria**:
- [x] An entity whose `depends_on` names B and whose fields reference C has a dependency list holding only B, as written, after parsing, after applying and when loaded back from state.
- [x] A disabled entity keeps its written list, and an entity with no `depends_on` has an empty list even when it holds references.
- [x] Creation and destruction still honour both B and C.
- [x] A `depends_on` entry reaching inside a module is still rejected by validation, and one naming the module itself still validates.
- [x] The new link helper adds a link once and leaves the dependency list untouched.

#### - [x] Task: Show the written dependency list in configuration text
**Id:** d8b008f2-de08-45d7-9321-6501b8716092
**Repo:** xclconfig
**Depends on:**
- d9669e0e-376d-455a-9177-8ea10ee3e1ea — Keep the written dependency list as written
**Execution:** agent

Configuration text now writes `depends_on` when the author wrote one, exactly as written, and leaves it out otherwise. Since the list no longer holds anything xcl worked out, it never shows a dependency the author did not write. Text from a live entity and from its saved data stays identical.

*Technical detail:* [context.md#task-show-the-written-dependency-list-in-configuration-text](./context.md#task-show-the-written-dependency-list-in-configuration-text)

**Acceptance criteria**:
- [x] The text for an entity that wrote `depends_on` naming B and references C contains a dependency list naming only B.
- [x] The text for an entity with no written dependency list contains no dependency list.
- [x] Text from the live entity and from its saved data is byte-identical.
- [x] Bookkeeping inside an attribute holding a whole object is still left out.

### Milestone 3: The documentation explains written dependency lists

**What changes**: A developer reading the README, the parser lifecycle guide or the site's configuration-text guide learns that a written `depends_on` list is preserved exactly, is shown in configuration text, and never gains the dependencies xcl works out, which still order creation and destruction. The changelog records the change and its breaking parts, including the removal of `meta.parents` from saved state. The project's knowledge entry on how the dependency list relates to links is brought into line.

**Validation point**: The README and CHANGELOG content tests pass, and they fail if the new text is removed. The site builds and type-checks. The knowledge entry describes the new split.

#### - [x] Task: Document written dependency lists in the library
**Id:** 38a71b63-f8dc-4298-bf7f-81b5bba61fe4
**Repo:** xclconfig
**Depends on:**
- d8b008f2-de08-45d7-9321-6501b8716092 — Show the written dependency list in configuration text
**Execution:** agent

The README's configuration-text section stops saying `depends_on` is never written, and says instead that a written list is preserved exactly and shown, while dependencies xcl works out are not added to it. The parser lifecycle guide says create ordering comes from the entity's links and that a written list is no longer altered. The changelog gets an entry for this spec listing the breaking changes, including that saved state no longer carries `meta.parents` and that destroy order now comes from links. Content tests guard the new text.

*Technical detail:* [context.md#task-document-written-dependency-lists-in-the-library](./context.md#task-document-written-dependency-lists-in-the-library)

**Acceptance criteria**:
- [x] The README states that a written dependency list is preserved exactly and shown in configuration text, and that worked-out dependencies are not added to it.
- [x] The README no longer says `depends_on` is never written.
- [x] The changelog has an entry for this work listing the narrowed dependency list, the replaced helper and the removal of `meta.parents` / `types.Meta.Parents` as breaking changes, and saying destroy order now comes from links.
- [x] The content tests fail if the new README text or the changelog entry is removed.

#### - [x] Task: Describe written dependency lists in the site's configuration-text guide
**Id:** 12852342-b3ac-4574-8a5a-82e61696e2c4
**Repo:** xcl-website
**Depends on:**
- 38a71b63-f8dc-4298-bf7f-81b5bba61fe4 — Document written dependency lists in the library
**Execution:** agent

The site's configuration-text guide gains a short section saying that a written `depends_on` list is preserved exactly and shown in configuration text, and that dependencies xcl works out are never added to it, with an example. The wording follows the README.

*Technical detail:* [context.md#task-describe-written-dependency-lists-in-the-sites-configuration-text-guide](./context.md#task-describe-written-dependency-lists-in-the-sites-configuration-text-guide)

**Acceptance criteria**:
- [x] The configuration-text guide states that a written dependency list is preserved exactly and shown, and that worked-out dependencies are not added to it, with an example.
- [x] The site builds and type-checks.

#### - [x] Task: Bring the dependency-list knowledge entry into line
**Id:** 361e4f02-4221-4eef-b5d9-99420e1ceb18
**Repo:** xclconfig
**Depends on:**
- d9669e0e-376d-455a-9177-8ea10ee3e1ea — Keep the written dependency list as written
**Execution:** agent

The project's knowledge entry on how the dependency list relates to links still describes the old copying behaviour and asks to be re-checked once this spec lands. It is rewritten through the knowledge skill to say that the list holds only what the user wrote and that ordering, for both create and destroy, and validation read the links.

*Technical detail:* [context.md#task-bring-the-dependency-list-knowledge-entry-into-line](./context.md#task-bring-the-dependency-list-knowledge-entry-into-line)

**Acceptance criteria**:
- [x] The knowledge entry describes the dependency list as holding only what the user wrote, and the links as the source for create and destroy ordering and for validation.
- [x] The entry no longer asks to be re-checked once this spec lands.

## Open Questions

- **Does the walk's decode of `depends_on` write exactly the strings the parser recorded?** It depends on how the copied `gohcl` decoder converts a list of string literals, which only the reload tests will show. The expected answer is yes, since the existing registered-type reload test already sees the written form. If decode writes a different form (for example a normalised address), STOP and ask the user before changing the decoder, because that touches `internal/xcl`.

## Out of Scope

- **Backwards compatibility with existing state files** (spec Non-Goals). State saved by earlier versions still loads, but its `depends_on` may hold worked-out entries until the configuration is applied again; nothing cleans it up. Its `parents` key is ignored and never read: destroy orders from its `links`, and a record whose links cannot be resolved (for example hand-edited state holding `parents` but no `links`) is destroyed without that ordering. No migration from `parents` to links is provided.
- **Changing how dependencies are worked out or ordered.** Which references count as dependencies, module expansion and parent-module edges stay as they are; only where the graph reads them from changes, and destroy reads them from the same place as create.
- **Changes to the graph walker (`internal/dag`).** Destroy keeps using the existing `Reverse` walk.
- **Changing destroy's failure, save or event behaviour.** Only where its edges come from changes.
- **Interpolation in `depends_on`.** It stays a list of address strings.
- **Showing worked-out dependencies anywhere new.** The resource printer keeps printing the written list and the links separately; configuration text never shows worked-out dependencies.
- **Recording `depends_on` as a reference for `ShowReferences()`.** It is not a reference expression; `20261003153421-bf87d907-references-as-written` keeps it out of `Meta.References`.
- **Changes to `internal/xcl` (the copied HCL).** None are needed, so `UPSTREAM.md` is untouched.

## Changelog

### 2026-10-05 — Task: Order creation from links

**What was done**: `getResourceDependencies` now reads every dependency from the entity's `Meta.Links` instead of its `DependsOn` list, so the create graph no longer depends on the user's written list. New graph-level and lifecycle tests pin create order for a dependency that exists only in `depends_on` and one that exists only as a reference, plus module-wide `depends_on`.

**Deviations**: The graph tests build the create graph over the state returned by a real `Apply` (which implements `ResourceProvider`) rather than a parse-only result. The apply-order test lives in `dag_test.go` next to the new fixture constants instead of `lifecycle_test.go`.

**Files changed**:
- `xclconfig: internal/parser/util.go`
- `xclconfig: internal/parser/dag_test.go`
- `xclconfig: internal/test_fixtures/config/lifecycle/written_and_referenced/main.xcl`

**Discoveries**: Until the mirror loop in `buildCreateDAG` is removed (task "Keep the written dependency list as written"), emptying `DependsOn` before building the graph is refilled by the mirror, so the "reads links" test only becomes discriminating after that task.

### 2026-10-05 — Task: Order destruction from links with the create graph's builder

**What was done**: The create graph's body became one builder, `buildDependencyGraph(rp, addresses, nodes, rootName, requireParentModule)`, which resolves each node's `Meta.Links` (and its parent module) against everything `rp` holds and keeps only edges between nodes. `buildCreateDAG` and `buildDestroyDAG` wrap it; the destroyer gained an `addresses` field set by `Apply`'s removal phase and `Destroy`, and builds its graph over its whole working state before walking it in reverse. `getResourceDependencies` gained `requireParentModule`, false for destroy so a missing parent module is ignored.

**Deviations**: `buildCreateDAG` keeps the `Links`→`DependsOn` mirror and the `Meta.Parents` recording as a prelude before calling the shared builder (rather than inside it), so the builder itself is already in its final form. An extra graph-level test (`TestDestroyGraphResolvesModuleRelativeLinksFromSavedState`) was added because the call-order test for module-relative links could pass by chance.

**Files changed**:
- `xclconfig: internal/parser/dag.go`
- `xclconfig: internal/parser/util.go`
- `xclconfig: internal/parser/destroy.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/destroy_test.go`
- `xclconfig: internal/test_fixtures/config/lifecycle/module_internal_reference/main.xcl`
- `xclconfig: internal/test_fixtures/config/lifecycle/module_internal_reference/module/networks.xcl`

**Discoveries**: Provider call-order assertions between independent resources can pass by luck, because the walker runs unrelated vertices concurrently; graph-level parent assertions are the deterministic check. Builtin modules do emit destroy success events, so module ordering can be asserted from events.

### 2026-10-05 — Task: Stop recording parents in saved state

**What was done**: `types.Meta.Parents` and its `parents` JSON key were removed, along with the recording loop in `buildCreateDAG` and both golden-schema entries. The `Links` doc comment now says it holds every dependency and orders create and destroy. Tests that read `Meta.Parents` now read the destroy graph built from saved state through a new `savedGraphParents` helper, keeping their asserted IDs; new tests check saved state has no `parents` key and a legacy record carrying `parents` still decodes. The state, overview, parser lifecycle and plugin developer guides describe destroy as the create graph built from saved links and walked backwards.

**Deviations**: `savedGraphParents` takes a `*Parser` (using its store and registry) instead of a harness, so one helper serves the lifecycle and registered-types harnesses. `docs/state.md` keeps one mention of `meta.parents` to say saved state no longer carries it and older state holding it still loads.

**Files changed**:
- `xclconfig: types/resource.go`
- `xclconfig: internal/parser/dag.go`
- `xclconfig: internal/schema/test_fixtures/embedded.go`
- `xclconfig: internal/parser/dag_test.go`
- `xclconfig: internal/parser/parents_test.go`
- `xclconfig: internal/parser/destroy_test.go`
- `xclconfig: internal/parser/removal_test.go`
- `xclconfig: internal/parser/registered_types_test.go`
- `xclconfig: internal/savedentity/savedentity_test.go`
- `xclconfig: docs/state.md`
- `xclconfig: docs/overview.md`
- `xclconfig: docs/parser-lifecycle.md`
- `xclconfig: docs/plugin-developer-guide.md`

**Discoveries**: The test sub-agent saw one unexplained full-suite `FAIL` (output truncated) that did not recur in six later full runs; possibly a pre-existing intermittent test. The knowledge entry `gotchas/xcl-tags-gate-what-reaches-cty.md` lists `Parents` among `Meta`'s json-only fields and is now slightly out of date.

### 2026-10-05 — Task: Keep the written dependency list as written

**What was done**: `types.AppendUniqueDependency` was replaced by `types.AppendUniqueLink`, which appends to `Meta.Links` only. The parser's link discovery adds references and canonical `depends_on` entries to the links, and sets `DependsOn` once to the `depends_on` strings exactly as written; the create graph's `Links`→`DependsOn` mirror and the dead `setDependsOn` TODO block were deleted. New tests prove the list holds only the written entry after parse, apply and reload, stays empty when none is written, survives on a disabled entity, and that create and destroy still order by both written and referenced dependencies.

**Deviations**: The module-boundary `depends_on` tests from the earlier spec (`TestValidateRejectsADependsOnNamingAModuleInternal`, `TestValidateAcceptsADependsOnNamingAChildModule`) already cover the boundary regression and pass unchanged, so none were duplicated. A disabled entity `off` was added to the `written_and_referenced` fixture.

**Files changed**:
- `xclconfig: types/resource_helpers.go`
- `xclconfig: types/resource.go`
- `xclconfig: types/resource_helpers_test.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/dag.go`
- `xclconfig: internal/parser/depends_on_test.go`
- `xclconfig: internal/test_fixtures/config/lifecycle/written_and_referenced/main.xcl`
- `xclconfig: config_test.go`

**Discoveries**: The plan's open question is resolved: the walk's decode writes the same strings the parser recorded. References are stored in `Meta.Links` as full attribute paths (e.g. `resource.network.c.subnet`), not entity addresses. A disabled entity's `disabled` flag is only decoded during the walk, so at parse time `GetDisabled` reports false.

### 2026-10-05 — Task: Show the written dependency list in configuration text

**What was done**: `trimBookkeeping` now removes `depends_on` only when the entity's dependency list is empty, so configuration text shows a written list exactly as written and nothing xcl worked out. The encode fixture gained `resource.container.api`, which writes `depends_on` naming the database and references the network; new tests check its text shows only the database in `depends_on`, that an entity without a written list has none, and that live and saved text are byte-identical. `TestEncodeEntityOmitsDependsOn` was replaced.

**Deviations**: None. No example expected output needed changing; the only example writing `depends_on` does not assert on configuration text.

**Files changed**:
- `xclconfig: encode.go`
- `xclconfig: encode_test.go`
- `xclconfig: internal/test_fixtures/config/encode/main.xcl`

**Discoveries**: None.

### 2026-10-05 — Task: Document written dependency lists in the library

**What was done**: The README's configuration-text "What is left out" paragraph now says a `depends_on` list is written exactly as written, left out when none was written, and never gains the dependencies xcl works out, which still order creation and destruction. The parser lifecycle guide says every graph edge comes from `Meta.Links` and `DependsOn` is never altered; the overview's `ResourceBase` sketch notes `DependsOn` is what the user wrote. CHANGELOG gained a top entry for this spec with a `**Breaking:**` list (narrowed `DependsOn`, `AppendUniqueLink`, removal of `types.Meta.Parents` / `meta.parents`, `depends_on` in configuration text). Content tests guard the README text and the changelog entry.

**Deviations**: None.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: CHANGELOG.md`
- `xclconfig: docs/parser-lifecycle.md`
- `xclconfig: docs/overview.md`
- `xclconfig: readme_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Document the module boundary and output entities on the site

**What was done**: On the home page, the Modules card now says outputs are the only way into a module, that anything else is rejected by validation, and that a nested module's value is exposed by re-exporting it. The Variables and outputs card says an output is an entity in Go, read with `Find[types.Output]` and `.Value`. The plugin example page gains a snippet of the program reading its outputs as `types.Output` entities (matching `example/plugin/main.go`), and a "What to notice" bullet on the module boundary with a re-export example. The printed output lines are unchanged.

**Deviations**: None. The site has no content tests. It was verified with `npm ci`, `make check` (0 errors, 0 warnings) and `npm run build` (6 pages built).

**Files changed**:
- `xcl-website: src/pages/index.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`

**Discoveries**: The site worktree had no `node_modules`, so `npm ci` had to run before `make check`.

### 2026-10-05 — Task: Bring the dependency-list knowledge entry into line

**What was done**: The knowledge entry `learnings/depends-on-mirrors-links.md` was rewritten through `spektacular knowledge write` and retitled "Meta.Links orders; DependsOn is as written". It says `DependsOn` holds only what the user wrote, set at parse time; `Meta.Links` holds every dependency and is what create and destroy ordering (one builder, destroy from saved links walked in reverse, no `Meta.Parents`), validation, the module boundary check and the evaluation context read; and `AppendUniqueLink` replaced `AppendUniqueDependency`. The "re-check once this lands" line is gone.

**Deviations**: The entry keeps its path (`learnings/depends-on-mirrors-links.md`) and only its title changed, so nothing that points at it breaks. Being an orchestrated run with a planned task, the write was made without a separate propose-then-confirm round.

**Files changed**:
- `xclconfig: .spektacular/knowledge/learnings/depends-on-mirrors-links.md`

**Discoveries**: `gotchas/xcl-tags-gate-what-reaches-cty.md` still lists `Parents` among `Meta`'s json-only fields and could be updated.
