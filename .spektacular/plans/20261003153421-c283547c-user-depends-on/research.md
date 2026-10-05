---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Research: 20261003153421-c283547c-user-depends-on

## Alternatives considered and rejected

- **Split `Meta.Links` into references only and have the graph read `Links` plus `DependsOn`.** Rejected: the module-boundary plan (`20261003153421-6ec0eab3`) judges every entry of `Meta.Links`, interpolated or written in `depends_on`, in validation stage 2 (`internal/parser/validate.go:96-125`), and the undefined-reference and property checks (`validate.go:60-90,116`) and the evaluation context (`internal/parser/context.go:44`) all read `Links`. Taking `depends_on` out of `Links` would need every one of those readers to learn a second source, and risks a user-written entry escaping the boundary check the user decided must apply to it.
- **Keep mirroring and remember which `DependsOn` entries were written, filtering at encode time.** Rejected: the list in state, in plugin calls and in event data would still hold worked-out entries, so the spec's "after parsing, applying and reloading" criterion fails everywhere except configuration text.
- **Leave `DependsOn` empty at parse time and rely on the walk's `gohcl.DecodeBody` (`internal/parser/callbacks.go:124`) to fill it.** Rejected: decode never runs for disabled entities (`callbacks.go:114-117`), so a disabled entity would lose its written list, and the list would be wrong between parse and walk ("after parsing" in the acceptance criteria).
- **Keep `types.AppendUniqueDependency` with new semantics (Links only).** Rejected in favour of a correctly named `types.AppendUniqueLink`: a public function named for dependencies that no longer touches `DependsOn` would mislead plugin authors. Recorded as a drafting assumption and a breaking change.
- **Store the canonical form (`fqdn.String()`, `parser.go:1118`) in `DependsOn`.** Rejected: the spec says the list holds exactly what the user wrote; the walk's decode already writes the raw strings, so parse time must match it.
- **Keep destroy on the recorded `Meta.Parents` (this plan's previous approach).** The plan first kept destroy reading the parent IDs the create graph records in `Meta.Parents` (`internal/parser/dag.go:83-104`, `buildDestroyDAG` at `dag.go:121-168`), on the grounds that destroy would follow the create graph's switch to links automatically. Superseded by the user's decision ("on the depends_on. you can use this for destroy too, there is a way to walk a graph backwards"): destroy builds the same graph from `Meta.Links` with the create graph's builder and walks it with `Reverse`, and `Meta.Parents` is removed, leaving one dependency source and one builder. Cost accepted: a breaking change to the saved-state format (`parents` was released in the `Config.Destroy` changelog entry).
- **Build the destroy graph from the targets alone (`rp` = targets).** Rejected in favour of resolving against the destroyer's whole working state and keeping only edges between targets: the edges are the same for dependencies in the set, but resolving against the full state lets the parent-module lookup and module-wide expansion see entities that are not being destroyed, and it matches what the create graph does. Both callers already hold the full state (`parser.go:266-282`, `parser.go:355-382`).
- **A separate destroy builder that mirrors `getResourceDependencies`.** Rejected: two builders would drift; the create graph's body is extracted into one builder that takes the node set, and create and destroy wrap it.
- **Fail the destroy when an entity's parent module is missing from state (as create does).** Rejected for destroy only: a destroy must never refuse to start over an incomplete state, and recorded parents missing from state were ignored the same way (`dag.go:151-155`). Create keeps the error.

## Chosen approach — evidence

- `internal/parser/util.go:506-520` — `getResourceDependencies` reads `types.GetDependencies` (the `DependsOn` field) to build the create graph; this is the single reader that must move to `Meta.Links`.
- `internal/parser/dag.go:62-72` — `buildCreateDAG` mirrors every `Links` entry into `DependsOn` via `AppendUniqueDependency` just before reading it; removed once the graph reads `Links`.
- `types/resource_helpers.go:74-111` — `AppendUniqueDependency` appends to `Links` and copies into `DependsOn` ("for backwards compatibility").
- `internal/parser/parser.go:1085-1124` — `getUniqueResourceLinks` adds references and parsed `depends_on` entries (canonical `fqdn.String()`) into both via `AppendUniqueDependency`; this is where `DependsOn` is set to the written strings and `Links` alone gains entries.
- `internal/parser/callbacks.go:124` — walk decode overwrites `DependsOn` with the written strings when the attribute is present; consistent with the new parse-time value.
- `internal/parser/dag.go:115-168` and `internal/parser/destroy.go:52-74` — destroy ordering reads `Meta.Parents`, which `buildCreateDAG` fills from the resolved deps (`dag.go:83-104`), and walks with `dag.Walker{Reverse: true}`; this is the second record of dependencies the user chose to remove. `buildDestroyDAG` already has the create graph's shape (parent → child edges, root for parentless nodes, outside-the-set parents ignored), so swapping its edge source for `getResourceDependencies` over the working state keeps the walk unchanged.
- `internal/parser/parser.go:266-287` (Apply's removal: `working` = previous state, targets = `removedResources`) and `parser.go:352-386` (`Destroy`: `working` = every decoded saved record, targets = all) — both callers hold the full state the builder resolves against; neither passes an address parser yet (`p.addressParser()`, `parser.go:163-174`).
- `types/resource.go:47-49` — `Links` is saved (`json:"links,omitempty"`) and has been since before `Parents` existed (`git log -S`), and `Module` is saved too; `internal/parser/destroy_test.go:39-48,474` and `parents_test.go:58-91` show variables, outputs, modules and disabled entities are all in saved state. So the builder works from saved state without the configuration.
- `internal/savedentity/savedentity.go:33,75` — plain `json.Unmarshal`, which ignores unknown keys; records saved with `parents` still decode after the field is removed.
- `internal/dag/walk.go:26-36`, `internal/dag/graph.go:171` — `Walker.Reverse` and `UpEdges` exist; no change to `internal/dag`.
- `encode.go:160-175` — `trimBookkeeping` removes `depends_on` unconditionally; becomes conditional on an empty list. `internal/xcl/gohcl/encode.go:565-583` `resolveValue` skips nil slices.
- `internal/parser/registered_types_test.go:439` — already asserts that a reloaded consumer with `depends_on = ["resource.app.web"]` and a reference to the same entity holds `[app ID]`; the pattern for the new reload test.
- `internal/parser/parents_test.go` — real-apply tests of `Meta.Parents`, including `depends_on` on a whole module (`lifecycle/module_reference`), the regression guard for the graph change; rewritten to read parents from the graph built from saved state once `Parents` is removed. Other readers: `destroy_test.go:543`, `removal_test.go:224`, `registered_types_test.go:751-753`; golden schema `internal/schema/test_fixtures/embedded.go:84-86,189-191`.

## Files examined

- `types/resource.go:49-81` — `Meta.Links`, `Meta.Parents` (removed by this plan), `ResourceBase.DependsOn` with `xcl:"depends_on,optional" json:"depends_on,omitempty"`.
- `internal/parser/dag.go:1-168` — `DoYouLikeDags`, `buildCreateDAG`, `buildDestroyDAG`.
- `internal/parser/destroy.go:1-141` — `destroyer`, `destroy(targets)`, reverse walk, save path (shared with the references-and-secrets save helper).
- `internal/parser/parser.go:225-410` — `Apply` removal phase, `Destroy`, `removedResources`.
- `internal/parser/util.go:621-694` — `findByAddress`, `findRelative`, `findModule`, `findByID`.
- `internal/parser/destroy_test.go`, `removal_test.go`, `config_destroy_test.go`, `registered_types_test.go:740-756` — destroy-order and parents assertions.
- `internal/savedentity/savedentity.go`, `decode_all_test.go` — decoding saved records; test style for record literals.
- `internal/schema/test_fixtures/embedded.go:75-95,180-195` — golden `Meta` schema.
- `internal/test_fixtures/config/lifecycle/module_reference/**` — module-wide `depends_on` fixture; its module's resources do not reference each other, so a new fixture covers module-relative links.
- `types/resource_helpers.go:55-131` — `GetDependencies`, `AppendUniqueDependency`, `SetDependencies`.
- `types/resource_helpers_test.go:109-225` — helper tests; `AppendUniqueDependency` test at 142.
- `internal/parser/dag.go:42-113` — create graph; mirror loop at 62-72.
- `internal/parser/util.go:506-579` — `getResourceDependencies`, module expansion and parent module edge.
- `internal/parser/parser.go:883-897` — commented-out `setDependsOn` TODO to delete.
- `internal/parser/parser.go:1085-1205` — link discovery and cycle check (reads `depMeta.Links`, unaffected).
- `internal/parser/context.go:44-60` — evaluation context reads `Links`.
- `internal/parser/validate.go:60-125` — property and reference stages read `Links`.
- `internal/parser/callbacks.go:100-130` — disabled skip and `DecodeBody`.
- `internal/parser/parents_test.go:1-120` — parents tests and fixtures.
- `internal/parser/registered_types_test.go:420-440` — reload assertion on `DependsOn`.
- `internal/test_fixtures/config/registered/basic/main.xcl:30-34`, `internal/test_fixtures/config/lifecycle/module_reference/main.xcl` — fixtures using `depends_on`.
- `config_test.go:40-100` — uses `AppendUniqueDependency` to build links for Config tests; switch to `AppendUniqueLink`.
- `encode.go:160-175`, `encode_test.go:238-270` — `TestEncodeEntityOmitsDependsOn`; inverted by this spec.
- `logger/pretty_printer.go:302,588` — prints `DependsOn` as "Dependencies" and `Links` separately; now shows only written entries, no change needed.
- `plugins/changed.go:12` — change detection ignores `depends_on`; unchanged.
- `README.md:680-705`, `CHANGELOG.md:1-30`, `readme_test.go` — configuration-text docs say `depends_on` is never written; content tests pattern.
- `docs/overview.md:105-108,125-140`, `docs/state.md:21-29`, `docs/parser-lifecycle.md:28-31,148-175`, `docs/plugin-developer-guide.md:394-395` — dependency and destroy-order descriptions; all that mention `meta.parents` / `Meta.Parents` are rewritten; parser-lifecycle also needs the graph source corrected. `README.md:510-540` says only "dependents before what they depend on" (no change for destroy). `CHANGELOG.md:57` — the released `Config.Destroy` entry that introduced `meta.parents`.
- `xcl-website:src/pages/index.mdx:134-137` — Dependency graph feature card (accurate, unchanged); `xcl-website:src/pages/examples/plugins.mdx:72-76` — depends_on example (unchanged). The configuration-text guide does not exist yet; references-as-written creates it.

## External references

- None. All behaviour is in-repo.

## Prior plans / specs consulted

- `20261003153421-6ec0eab3-module-boundary-and-output-entities` (plan) — boundary check runs over `Meta.Links` including `depends_on`; this plan must keep `depends_on` in `Links`. `FQRN.AppendParentModule` for module keys (already used in `getResourceDependencies`).
- `20261003134528-327e0657-references-and-secrets` (plan) — wire-encoded state and sensitive marker; no overlap with `depends_on` beyond touching `encode.go`.
- `20261003153421-bf87d907-references-as-written` (plan) — adds `Meta.References`, `ShowReferences()`, the site's `/configuration-text/` guide; leaves `depends_on` trimmed and unrecorded, explicitly handing it to this spec. Its replacement step runs after `trimBookkeeping`.
- Knowledge `learnings/depends-on-mirrors-links.md` — describes today's mirroring and says to re-check once this spec lands.

## Open assumptions

- The walk's decode of `depends_on` produces exactly the strings written (string list literal, no interpolation), equal to what parse time records. If it differs (e.g. a normalised form), STOP and ask.
- `gohcl` writes a non-empty `DependsOn` as `depends_on = ["..."]` on the encoded block; the encoder test proves it.
- No caller outside the repo relies on `DependsOn` holding worked-out dependencies; treated as a breaking change in the changelog.
- No caller outside the repo needs resolved parent IDs from saved state that `Meta.Links` cannot give; the removal of `Meta.Parents` / `meta.parents` is a breaking change in the changelog.
- The references-and-secrets plan's wire-encoded state keeps `meta.links` and `meta.module` as plain strings; destroy from saved state relies on it. If it does not, STOP and ask.

## Drafting assumptions

### Chosen direction: graph reads Meta.Links, DependsOn holds only written entries (architecture)
- **Decision**: switch `getResourceDependencies` to `Meta.Links` (with its own create/destroy tests), then remove mirroring by replacing `AppendUniqueDependency` with `AppendUniqueLink`; parser sets `DependsOn` to the written strings; encoder trims `depends_on` only when empty. `depends_on` entries stay in `Links`.
- **Rationale**: one reader to change; every other `Links` reader, including the module boundary check, is untouched; follows the spec's technical direction and the depends-on-mirrors-links knowledge entry.
- **Rejected**: splitting Links (many readers, boundary risk); encode-time filtering (state and plugins still polluted); decode-only (disabled entities lose the list).

### Destroy builds the create graph from links and walks it backwards; Meta.Parents removed (architecture, user decision)
- **Decision**: extract `buildCreateDAG`'s body into one builder `buildDependencyGraph(rp, addresses, nodes, rootName, requireParentModule)`; create = all entities, destroy = targets resolved against the destroyer's whole working state with only edges between targets kept, walked with `Reverse` as today. Remove `Meta.Parents` (field, `parents` JSON key, recording loop, golden fixture, docs) and rewrite the tests that read it to read the graph built from saved state.
- **Rationale**: the user's decision — one dependency source (`Meta.Links`) and one builder for create and destroy. Saved state already holds links, module and every entity kind the builder needs.
- **Rejected**: keeping destroy on recorded `Meta.Parents` (previous approach); a separate destroy builder (drift); resolving against targets only (loses the parent-module and module-wide lookups outside the set).

### Destroy ignores a missing parent module (discovery)
- **Decision**: `getResourceDependencies` gains `requireParentModule`; create passes true (error as today), destroy passes false (the edge is skipped).
- **Rationale**: a destroy must not refuse to start over an incomplete state; recorded parents missing from state were ignored the same way.
- **Rejected**: failing the destroy (could leave state undestroyable); always ignoring it (would weaken create's existing check).

### Order of change: create switch, destroy switch, Parents removal, then the DependsOn mirror (sequencing)
- **Decision**: three Milestone 1 tasks — create graph reads links; destroy uses the shared builder while `Parents` is still recorded (existing destroy tests pass unchanged); then `Parents` is removed and its tests rewritten. The `DependsOn` mirror goes in Milestone 2 as before. The `docs/` pages describing `meta.parents` are updated in the removal task so they never describe a field that no longer exists; README/CHANGELOG stay in the Milestone 3 docs task.
- **Rationale**: keeps the plan's principle of switching the reader before removing the second record, so each step is guarded by unchanged existing tests.
- **Rejected**: switching destroy and removing `Parents` in one step (the existing destroy tests would change at the same time as the code they guard).

### meta.parents is a Breaking changelog item (discovery)
- **Decision**: list the removal of `types.Meta.Parents` and of `meta.parents` from saved state under **Breaking:** in this spec's CHANGELOG entry.
- **Rationale**: epic decision — Breaking lists name only changes against the last release; `parents` was released (`CHANGELOG.md:57`).
- **Rejected**: omitting it as internal (it is a public field and part of the saved format).

### Old state carrying parents (discovery)
- **Decision**: no migration; the key is ignored on load and destroy orders from `links`, which every version has saved. A record with `parents` but no usable links is destroyed without that ordering.
- **Rationale**: the spec's Non-Goals rule out backwards compatibility; in practice all real state has links.
- **Rejected**: reading `parents` as a fallback (keeps the second source alive).

### Conventions selected (architecture)
- **Decision**: testing, real-apply state, code style, shared types in `types`, AppendParentModule, golden schema for `Meta` fields. Dropped database, dependencies, logging, project structure, patterns, never-modify-dependencies and shared-errors (no new errors, no dependency or `internal/xcl` change).
- **Rationale**: only these bear on the touched code.
- **Rejected**: listing all conventions.

### Rename AppendUniqueDependency to AppendUniqueLink (discovery)
- **Decision**: replace the public `types.AppendUniqueDependency` with `types.AppendUniqueLink`, which only appends to `Meta.Links`; list the removal under Breaking.
- **Rationale**: a function named for dependencies that no longer touches `DependsOn` would mislead plugin authors.
- **Rejected**: keeping the old name with new semantics (misleading); keeping both (dead API).

### depends_on stays in Meta.Links (discovery)
- **Decision**: user-written `depends_on` entries keep being added to `Meta.Links`, so the graph, validation and the module boundary check all keep seeing them; `DependsOn` holds only the written list.
- **Rationale**: the module-boundary plan's check runs over `Links` and relies on `depends_on` being there.
- **Rejected**: splitting Links into references only.

### DependsOn set at parse time to the strings as written (discovery)
- **Decision**: the parser sets `DependsOn` to the written strings (after validating each parses as an address), not the canonical form.
- **Rationale**: matches the walk's decode and covers disabled entities, which are never decoded.
- **Rejected**: relying on the walk decode only.

## Rehydration cues

- `spektacular spec file read 20261003153421-c283547c-user-depends-on`
- `spektacular plan file read 20261003153421-6ec0eab3-module-boundary-and-output-entities plan` and the same for `20261003153421-bf87d907-references-as-written`.
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"learnings/depends-on-mirrors-links.md"}'`
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"gotchas/meta-field-golden-schema.md"}'`
- Re-read `internal/parser/dag.go`, `internal/parser/destroy.go`, `internal/parser/parser.go:225-410`, `internal/parser/util.go:506-579`, `internal/parser/parser.go:1085-1124`, `types/resource_helpers.go`, `encode.go:160-175`.
