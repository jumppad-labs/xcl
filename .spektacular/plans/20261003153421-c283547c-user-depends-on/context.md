---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Context: 20261003153421-c283547c-user-depends-on

## Current State Analysis

- `types/resource_helpers.go:74-111` — `AppendUniqueDependency` appends to `Meta.Links` and mirrors into `ResourceBase.DependsOn`.
- `internal/parser/parser.go:1085-1124` — link discovery adds references and canonical `depends_on` entries through that helper, so `DependsOn` holds both after parsing.
- `internal/parser/dag.go:62-72` — `buildCreateDAG` mirrors `Links` into `DependsOn` again; `internal/parser/util.go:506-520` then builds edges from `DependsOn`.
- `internal/parser/callbacks.go:124` — the walk's decode overwrites `DependsOn` with the written strings for enabled entities only; disabled entities keep the polluted list.
- Validation (`internal/parser/validate.go:60-125`), the evaluation context (`internal/parser/context.go:44`) and the cycle check (`internal/parser/parser.go:1183`) already read `Meta.Links`; the module-boundary plan adds its check there too.
- `encode.go:160-175` — `trimBookkeeping` always drops `depends_on` because it cannot tell written from worked-out entries.
- Destroy order reads `Meta.Parents` (`internal/parser/dag.go:115-168` `buildDestroyDAG(toDestroy)`, called at `internal/parser/destroy.go:52` and walked with `dag.Walker{Reverse: true}` at `destroy.go:71-74`), filled by the create graph at `dag.go:83-104` and saved as `parents` (`types/resource.go:51-56`). By the user's decision this goes: destroy builds the create graph's graph from `Meta.Links` and walks it in reverse, and `Meta.Parents` is removed.
- The destroyer is constructed at `internal/parser/parser.go:275-282` (Apply's removal phase: `working` is a copy of the previous state, targets are `removedResources`) and `parser.go:371-382` (`Destroy`: `working` holds every saved record decoded by `savedentity.DecodeAll` after `loadPlugins`, targets are all of it). Neither passes the address parser today; `p.addressParser()` (`parser.go:163-174`) builds one from the plugin registry's types.
- Saved state holds every entity, including variables, outputs, modules and disabled entities (`internal/parser/destroy_test.go:39-48,474`, `parents_test.go:58-91`), each with `Meta.Links` (`json:"links"`) and `Meta.Module`. `savedentity` decodes with `json.Unmarshal` (`internal/savedentity/savedentity.go:33,75`), which ignores unknown keys, so records saved with `parents` still load.
- `getResourceDependencies` (`internal/parser/util.go:506-579`) resolves each entry against `rp` with `findModule`/`findByAddress` (`util.go:621-673`), ignoring unresolved entries (nil), and fails when the entity's own module is missing (`util.go:562-576`).
- `DoYouLikeDags(rp, addresses, destroy)` (`dag.go:32-40`) is the exported entry; its destroy branch calls `buildDestroyDAG(rp.GetResources())`.
- Tests reading `Meta.Parents`: `internal/parser/parents_test.go:42-130`, `destroy_test.go:543`, `removal_test.go:224`, `registered_types_test.go:751-753`; golden schema `internal/schema/test_fixtures/embedded.go:84-86,189-191`. Docs: `docs/state.md:21-29`, `docs/overview.md:105-108,137`, `docs/parser-lifecycle.md:28-31,148-153,174-175`, `docs/plugin-developer-guide.md:394-395`; README (`README.md:510-540`) says only "dependents before what they depend on" and needs no change for this.

## Per-Task Technical Notes

### Task: Order creation from links

Requirement → repo: "Ordering still honours every dependency" → `xclconfig`.

**File changes**:
- `internal/parser/util.go:506-520` — `getResourceDependencies` iterates `resourceMeta.Links` instead of `types.GetDependencies(resource)`; drop the `GetDependencies` call and its error. Keep `addresses.Parse`, the `TypeModule` branch with `findModule` and the resource branch with `findByAddress`, both resolving through `fqdn.AppendParentModule(resourceMeta.Module)` (gotcha `nested-module-keys-use-append-parent-module.md`). Links carry attribute paths (`resource.c.one.x`); `findByAddress` already resolves those today because the mirrored `DependsOn` held the same strings. Parent-module edge at `util.go:562-576` unchanged in this task.
- `internal/parser/dag.go:62-72` — leave the mirror loop in place in this task (removed in "Keep the written dependency list as written"), so both sources agree and existing tests pass unchanged. `Meta.Parents` recording and `buildDestroyDAG` untouched here.
- `internal/test_fixtures/config/lifecycle/written_and_referenced/main.xcl` (new) — `resource "network" "b"`, `resource "network" "c"` with an attribute, `resource "network" "a" { depends_on = ["resource.network.b"]  subnet = resource.network.c.subnet }`, plus `resource "network" "alone"`. Use the lifecycle harness's network type (`internal/parser/lifecycle_test.go:21-66`).
- `internal/parser/dag_test.go` (new) — test helper `graphParentIDs(t, graph, entity) []string`: sorted IDs of the vertex's up-edges (`graph.UpEdges(v)`), the root excluded. Graph-level tests on a parsed fixture: A's parents in `buildCreateDAG` are exactly B and C; with `types.SetDependencies(a, nil)` before building, still B and C (proves the graph reads `Links`). Separate test: `alone` has no parents and hangs off root.
- `internal/parser/lifecycle_test.go` — one test asserting provider create calls for B and C precede A (`h.plugin.GetCalls()` order).

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential: switch the reader, run the existing suite, then add fixture and tests.

### Task: Order destruction from links with the create graph's builder

Requirement → repo: "Ordering still honours every dependency" (destroy half; user decision: one dependency source and one builder) → `xclconfig`.

**File changes**:
- `internal/parser/dag.go:42-113` — extract the body of `buildCreateDAG` into `buildDependencyGraph(rp ResourceProvider, addresses *resources.AddressParser, nodes []any, rootName string, requireParentModule bool) (*dagpkg.AcyclicGraph, error)`:
  - return an empty graph when `nodes` is empty (as `buildDestroyDAG` does today); otherwise add a root vertex named `rootName` and every node;
  - index nodes by `Meta.ID` (`map[string]any`); for each node call `getResourceDependencies(rp, addresses, node, meta, requireParentModule)`; for each non-nil dep whose `Meta.ID` is in the index, `graph.Connect(BasicEdge(index[id], node))`; a node with no such edge connects to the root;
  - keep the `Links`→`DependsOn` mirror loop and the `Meta.Parents` recording in this task, only for the create call (guard by `requireParentModule` or keep them in `buildCreateDAG` before calling the builder), so the existing tests reading `Parents` keep passing. Both go in later tasks.
  - `buildCreateDAG(rp, addresses)` = `buildDependencyGraph(rp, addresses, rp.GetResources(), "root", true)`; output identical to today since every dep is in the node set.
  - `buildDestroyDAG(rp ResourceProvider, addresses *resources.AddressParser, toDestroy []any)` = `buildDependencyGraph(rp, addresses, toDestroy, "destroy_root", false)`. Rewrite its doc comment: same graph as create, from `Meta.Links` resolved against all of `rp`, edges only between entities in `toDestroy`, walked with `Reverse`.
  - `DoYouLikeDags` (`dag.go:32-40`) destroy branch: `buildDestroyDAG(rp, addresses, rp.GetResources())`. Leave the function name and its comment as they are (the comment forbids renaming it).
- `internal/parser/util.go:506-579` — `getResourceDependencies` gains `requireParentModule bool`; when the parent-module lookup (`util.go:562-576`) fails and it is false, skip the edge instead of returning the error. Reason: a destroy must not refuse to start over an incomplete state; recorded parents missing from state were ignored the same way.
- `internal/parser/destroy.go:22-56` — add `addresses *resources.AddressParser` to `destroyer`; `destroy(targets)` calls `buildDestroyDAG(d.working, d.addresses, targets)`. The graph is built before the walk, so the walk's concurrent `RemoveResource` on `d.working` does not race it. Rest of the method (TransitiveReduction, Validate, `Walker{Reverse: true}`) unchanged; update the doc comments that say parents are read from recorded parents.
- `internal/parser/parser.go:275-282` and `:371-382` — set `addresses: p.addressParser()` in both `destroyer` literals. In `Destroy` this comes after `loadPlugins`, so plugin types are known. Update the `Destroy` doc comment (`parser.go:331-337`): order is "the reverse of their create order, built from the links each resource saved".
- `internal/test_fixtures/config/lifecycle/module_internal_reference/main.xcl` + `module/main.xcl` (new) — a module whose `resource "network" "two"` sets `subnet = resource.network.one.subnet` (module-relative), plus `resource "network" "one"`.
- `internal/parser/destroy_test.go` — new functions, each loading state with `h.store.Load()` and a fresh parser (`destroyAll`):
  - `TestDestroyFromSavedStateDestroysBeforeWrittenDependency` (A before B, `written_and_referenced`);
  - `TestDestroyFromSavedStateDestroysBeforeReferencedDependency` (A before C);
  - `TestDestroyFromSavedStateIgnoresWrittenDependsOnList` — build `buildDestroyDAG` over the loaded, decoded state with A's `DependsOn` set to nil: A's parents still B and C (via `graphParentIDs`);
  - `TestDestroyFromSavedStateDestroysModuleWideDependentFirst` (`module_reference`: consumer's destroy success event precedes those of `module.networks.resource.network.one` and `.two`, use the existing event collector);
  - `TestDestroyFromSavedStateDestroysModuleResourcesBeforeModule` (one and two before `module.networks`, from events);
  - `TestDestroyFromSavedStateResolvesModuleRelativeLinks` (`module_internal_reference`: two before one);
  - `TestDestroyGraphIgnoresDependenciesOutsideTheSet` — `buildDestroyDAG` over loaded state with targets {A, C}: A's only parent is C;
  - `TestDestroyGraphIgnoresMissingParentModule` — loaded `module_reference` state with `module.networks` removed from the working state: building succeeds and `one` hangs off the root.
- Existing `destroy_test.go`, `removal_test.go`, `config_destroy_test.go` tests unchanged and must pass (they still read the recorded `Parents` in this task).

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential: extract the builder, switch destroy, run the existing suite, then add fixtures and tests.

### Task: Stop recording parents in saved state

Requirement → repo: supporting (user decision: one dependency source; removes the second record) → `xclconfig`.

**File changes**:
- `types/resource.go:51-56` — delete `Parents` and its comment. Update the `Links` comment: every dependency (references and written `depends_on`), module-relative; what create and destroy ordering, validation and the evaluation context read.
- `internal/parser/dag.go` — delete the `parents` slice, `slices.Sort` and `resourceMeta.Parents = parents` (`dag.go:83-104`); drop the `slices` import if unused.
- `internal/schema/test_fixtures/embedded.go:84-86,189-191` — remove the `Parents` entry in both places (gotcha `meta-field-golden-schema.md`).
- `internal/parser/dag_test.go` — add a shared helper `savedGraphParents(t, h, id) []string`: load the store, decode with `savedentity.DecodeAll` (as `Parser.Destroy` does), build `buildDestroyDAG` over all of it, return `graphParentIDs` for `id`. Used by the rewrites below.
- `internal/parser/parents_test.go:42-130` — rewrite each test to the helper, keeping its intent and asserted IDs, and rename away from "Records": `TestSavedStateOrdersEachResourceAfterItsDependencies`, `TestSavedStateOrdersModuleBeforeItsResources`, `TestSavedStateOrdersEveryModuleResourceBeforeModuleWideDependent`, `TestReapplyWithoutChangesDestroysNothingAndKeepsOrder` (parents from the first and second saved state equal). Add `TestSavedStateHasNoParentsKey` (raw saved records, no `"parents"` in any `meta`).
- `internal/parser/destroy_test.go:543` and `internal/parser/removal_test.go:224` — take the failed resource's parents from the helper over the state loaded after the failure (the failed resource and its parents remain in it) instead of `meta.Parents`.
- `internal/parser/registered_types_test.go:751-753` — read parents from the create graph (`DoYouLikeDags(st, p.addressParser(), false)` + `graphParentIDs`) or the helper; rename the test away from "Records".
- `internal/savedentity/savedentity_test.go` — `TestDecodeIgnoresLegacyParentsKey`: a record literal with `"parents": ["x"]` in `meta` decodes without error.
- `docs/state.md:21-29` — replace the `meta.parents` paragraph: each saved resource keeps `meta.links` and `meta.module`; `Destroy` resolves them against the saved state with the create graph's builder and walks the graph backwards, so it needs no configuration; `parents` is no longer written, and older state carrying it still loads.
- `docs/overview.md:105-108` — destroy order: "children first, from the same dependency graph as create, built from the links each resource saved"; `:137` — delete the `Parents` line from the `Meta` sketch; update `Links` comment ("every dependency, orders create and destroy").
- `docs/parser-lifecycle.md:28-31` — drop the "recorded in `Meta.Parents`" sentence; `:148-153` — `buildDestroyDAG` uses the create graph's builder over the working state, resolving each target's `Meta.Links` and module, edges only between targets, missing parent module ignored; `:174-175` — replace the "saved before `Meta.Parents`" note with: state saved by earlier versions is ordered from its links; its `parents` is ignored.
- `docs/plugin-developer-guide.md:394-395` — "children first, the create order reversed" (no `meta.parents`).
- Grep gate: no `Parents`/`meta.parents` left in Go sources (outside `internal/xcl`) or `docs/`; `config_destroy_test.go:219`'s comment "parents: ..." may stay (plain English).

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential: remove the field and recording, fix compile errors in tests via the helper, golden fixture, then docs.

### Task: Keep the written dependency list as written

Requirement → repo: "User-written dependencies are preserved" → `xclconfig`.

**File changes**:
- `types/resource_helpers.go:74-111` — replace `AppendUniqueDependency` with `AppendUniqueLink(resource any, link string) error`: get `Meta`, return when present, append to `Meta.Links`. No `DependsOn` access. Doc comment per Data Structures.
- `types/resource.go:79-80` (lines shift once `Parents` is gone) — `DependsOn` doc comment: holds exactly what the user wrote; full set is `Meta.Links`. The `Links` comment was already rewritten in "Stop recording parents in saved state"; check it still reads right.
- `internal/parser/dag.go` — delete the `Links`→`DependsOn` mirror loop (originally `dag.go:62-72`, kept on the create path by "Order destruction from links with the create graph's builder"), and the `errors`/`fmt` imports if now unused.
- `internal/parser/parser.go:1085-1124` — `getUniqueResourceLinks`: references via `types.AppendUniqueLink`; for `depends_on`, keep `p.addressParser().Parse(d.AsString())` validation, add `fqdn.String()` with `AppendUniqueLink`, collect `d.AsString()` into a `written []string` and call `types.SetDependencies(resource, written)` once. Leave `DependsOn` untouched when the attribute is absent.
- `internal/parser/parser.go:883-897` — delete the commented-out `setDependsOn` TODO block.
- `config_test.go:52-93` — switch `types.AppendUniqueDependency` to `types.AppendUniqueLink`.
- `types/resource_helpers_test.go:140-150` — replace the `AppendUniqueDependency` test with `TestAppendUniqueLinkAddsOnce` and `TestAppendUniqueLinkLeavesDependsOnUntouched`.
- `internal/parser/parse_test.go` or a new `internal/parser/depends_on_test.go` — using the `written_and_referenced` fixture: after parse A's `DependsOn` is `["resource.network.b"]` and `Links` contains B and C; after apply (live entity) the same; after `loadSaved` the same; `alone` has empty `DependsOn`. A disabled entity with `depends_on` (add `resource "network" "off" { disabled = true  depends_on = ["resource.network.b"] }` to the fixture or a sibling fixture) keeps its list after parse and in saved state. Each a separate test.
- `internal/parser/validate_test.go` — confirm the module-boundary plan's tests (`depends_on` naming a module internal rejected; naming the module itself validates) still pass; add them here if that plan placed them elsewhere and they do not exercise `depends_on` after this change.
- `internal/parser/registered_types_test.go:439` — existing assertion stays and passes.
- Re-run Milestone 1 graph and destroy-from-saved-state tests: must pass with the mirror gone, so destroy order for B (only in `depends_on`) and C (only a reference) is proven to come from links.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential: helper, parser, graph, then tests.

### Task: Show the written dependency list in configuration text

Requirement → repo: "User-written dependencies appear in output" → `xclconfig`.

**File changes**:
- `encode.go:160-175` — `trimBookkeeping`: remove `depends_on` only when `types.GetDependencies(entity)` returns an empty list (or an error); rewrite the doc comment to say a written list is shown as written and nothing xcl works out is in it. The references-as-written replacement step runs after the trim and never touches `depends_on`.
- `internal/xcl/gohcl/encode.go:376` — no change; the object-attribute path still skips the embedded base. `internal/xcl/UPSTREAM.md` untouched.
- `encode_test.go:238-249` — replace `TestEncodeEntityOmitsDependsOn` with `TestEncodeEntityWritesWrittenDependsOn` (fixture entity with `depends_on` naming B and a reference to C: text contains `depends_on = ["<B>"]`, and the dependency list does not name C) and `TestEncodeEntityOmitsDependsOnWhenNoneWritten`. Add `TestEncodeSavedEntityMatchesLiveWithDependsOn` (byte-identical). Extend the encode fixture (used by `applyEncodeFixture`, `encode_test.go:34`) with such an entity, or add a small one; keep `TestEncodeEntityOmitsBookkeepingInsideObjectAttribute` (`encode_test.go:251-270`) unchanged.
- `example/**` expected output — update only where a printed entity writes `depends_on` (e.g. `example/plugin/config/main.xcl`).

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Document written dependency lists in the library

Requirement → repo: "Dependency behaviour is documented" (library half) → `xclconfig`.

**File changes**:
- `README.md:694-700` — "What is left out" paragraph: drop "neither is `depends_on`…"; add that a `depends_on` list is written exactly as the author wrote it, and that dependencies xcl works out from references are never added to it, though they still order creation and destruction. Keep the references-as-written plan's wording on `ShowReferences()` intact.
- `docs/parser-lifecycle.md:28-31` — the DAG is built from `Meta.Links`, which hold references and written `depends_on`; `DependsOn` keeps only what was written. (The `Meta.Parents` wording here and in the destroy section was already replaced by "Stop recording parents in saved state".)
- `docs/overview.md:125-130` — add a one-line comment on `DependsOn` holding the written list.
- `CHANGELOG.md:1` — new top entry `## 20261003153421-c283547c-user-depends-on`: what changes, then `**Breaking:**` list: `DependsOn` holds only the written list (read `Meta.Links` for every dependency); `types.AppendUniqueDependency` is replaced by `types.AppendUniqueLink`; configuration text now includes `depends_on` when written; `types.Meta.Parents` is removed and saved state no longer carries `meta.parents` — destroy builds the create graph from each saved resource's `meta.links` and walks it backwards (code reading `Meta.Parents` reads `Meta.Links`; older state carrying `parents` still loads, the key ignored). The prose part says destroy order now comes from the same links as create, so state saved before `parents` existed is destroyed children first too. `parents` was released (CHANGELOG `Config.Destroy` entry), so it belongs in Breaking; the epic's other unreleased changes do not.
- `readme_test.go` — add `TestReadmeDocumentsWrittenDependsOnIsShown` and `TestChangelogRecordsUserDependsOn` (heading present, `**Breaking:**`, `AppendUniqueLink`, `meta.parents`), following `readme_test.go:22-110`. If an existing test asserts the "neither is `depends_on`" text, update it.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

### Task: Describe written dependency lists in the site's configuration-text guide

Requirement → repo: "Dependency behaviour is documented" (site half) → `xcl-website`.

**File changes**:
- `xcl-website:src/pages/configuration-text.mdx` — created by `20261003153421-bf87d907-references-as-written`. Add a "Dependency lists" section: a written `depends_on` is preserved exactly and shown; dependencies worked out from references are not added but still order creation and destruction; example HCL showing a block with `depends_on = ["resource.app.b"]` and a resolved reference field. Wording follows the README.
- `xcl-website:src/pages/examples/plugins.mdx:72-76`, `xcl-website:src/pages/index.mdx:134-137` — no change (still accurate).
- Gate: `npm ci`, the site build and `astro check`.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential.

### Task: Bring the dependency-list knowledge entry into line

Requirement → repo: supporting (keeps the knowledge base current) → `xclconfig` knowledge store.

**File changes**:
- `.spektacular/knowledge/learnings/depends-on-mirrors-links.md` — rewrite only through the `spek-knowledge` skill (`spektacular knowledge` CLI), never with file tools. New content: `DependsOn` holds exactly what the user wrote (set at parse time, re-decoded at walk for enabled entities); `Meta.Links` holds references plus canonical `depends_on` entries and is what the dependency graph for both create and destroy (destroy builds it from saved state and walks it in reverse; there is no `Meta.Parents`), validation, the module boundary check and the evaluation context read; `types.AppendUniqueLink` replaces `AppendUniqueDependency`. Retitle accordingly (e.g. "Links order, DependsOn is as written"); drop the "re-check once that lands" line. The skill's own propose-then-confirm flow applies.

**Complexity**: Low
**Token estimate**: ~4k tokens
**Agent strategy**: Single agent.

## Testing Strategy

- **Order creation from links**: graph-level tests on a parsed fixture (parents B and C; same with the written list emptied; unrelated entity off root) and a provider create-order test, each its own function. Existing lifecycle, parents and destroy tests unchanged.
- **Order destruction from links with the create graph's builder**: destroy from state loaded by a fresh parser — A before B (only `depends_on`), A before C (only a reference), same parents with A's written list emptied, module-wide `depends_on` dependent before the module's resources, module resources before the module, module-relative link inside a module; destroy graph ignores dependencies outside the set and a missing parent module. Existing destroy, removal and config-destroy tests unchanged.
- **Stop recording parents in saved state**: parents, destroy, removal and subtype tests rewritten to read parents from the graph built from saved state, intent and asserted IDs kept; saved state has no `parents` key; a legacy record with `parents` decodes; golden schema test passes; grep gate on `Parents` in Go sources and `docs/`.
- **Keep the written dependency list as written**: parse, apply and reload tests on the list (A holds only B; disabled entity keeps its list; entity without `depends_on` empty); links still hold B and C; helper tests for `AppendUniqueLink`; module-boundary `depends_on` tests still pass; graph tests re-run with the mirror gone.
- **Show the written dependency list in configuration text**: encoder tests on an applied fixture (written list shown, none written when absent, live and saved byte-identical); object-attribute bookkeeping test unchanged.
- **Document written dependency lists in the library**: README and CHANGELOG content tests in `readme_test.go`.
- **Describe written dependency lists in the site's configuration-text guide**: site build and `astro check` after `npm ci`; manual browser review captured in the implementation test plan.
- **Bring the dependency-list knowledge entry into line**: read the entry back through `spektacular knowledge read`.
- Conventions: testify `require`, no table-driven tests, positive and negative cases in separate functions, state from a real apply. No success metrics in the spec.

## Project References

- Spec: `20261003153421-c283547c-user-depends-on` (epic `20261003134528-327e0657-references-and-secrets`). No design documents referenced.
- Plans built on: `20261003153421-6ec0eab3-module-boundary-and-output-entities`, `20261003134528-327e0657-references-and-secrets`, `20261003153421-bf87d907-references-as-written`.
- Knowledge: `learnings/depends-on-mirrors-links.md`, `gotchas/meta-field-golden-schema.md`, `gotchas/nested-module-keys-use-append-parent-module.md`, `architecture/shared-public-types-live-in-types.md`, `conventions/test-state-from-real-apply.md`, `conventions/testing-and-mocking.md`.
- Repo roots: `xclconfig` → `/home/nicj/code/github.com/jumppad-labs/xcl`; `xcl-website` → `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

- Plugin and application code: replace `types.AppendUniqueDependency` with `types.AppendUniqueLink`; read `Meta.Links` instead of `DependsOn` or `Meta.Parents` to learn every dependency (links are unresolved, module-relative addresses; there is no longer a field holding resolved parent IDs).
- Saved state no longer carries `meta.parents`. Records saved earlier still decode (the key is ignored) and are destroyed in order from their `links`, which every version has saved; no migration step.
- Saved state from earlier versions loads; its `depends_on` may hold worked-out entries until the configuration is applied again (spec Non-Goals).

## Performance Considerations

None significant. The create graph reads a slice of the same size as before, and one mirror pass per entity is removed. Destroy now resolves each target's links with the same linear-scan lookups the create graph already does for the whole configuration (`findByAddress`/`findModule`, `util.go:621-673`), instead of matching recorded IDs; this is the cost every apply already pays, once per destroy.
