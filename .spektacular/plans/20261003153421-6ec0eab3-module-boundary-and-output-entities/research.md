---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Research: 20261003153421-6ec0eab3-module-boundary-and-output-entities

## Alternatives considered and rejected

- **Enforce the boundary in the evaluation context instead of validation** (stop `buildContextForResource` from exposing module internals, `xclconfig:internal/parser/context.go:44-160`). Rejected: the spec's constraint says the boundary is enforced by validating references; a context-only fix would surface as a late evaluation failure (unknown value) rather than a validation error naming the reference, and Apply already refuses to walk when validation fails (`internal/parser/parser.go:536`).
- **Make `output` a non-builtin / plugin-style type to enforce the boundary by type rules.** Rejected by the spec's constraint: `output` must remain a builtin entity type.
- **Keep `resources.Output` and add a type alias `types.Output = resources.Output`.** Rejected: leaves two names for one type and the public name would still be declared in an internal package; `architecture/shared-public-types-live-in-types.md` says a public type both the parser and applications need lives in `types`. The type moves outright and every internal site switches to `types.Output` (~15 sites, listed below).
- **Put `Output` in the root `xcl` package.** Rejected: root imports `internal/parser`, which needs the type — import cycle (`architecture/shared-public-types-live-in-types.md`).
- **Return the value via a generic `OutputValue[T]` helper / keep `Find[string]` working for outputs alongside entities.** Rejected: the spec's Technical Approach says drop the lookup special case so `Find` returns the entity; `Outputs()` already covers "all values in one call".
- **Check the boundary by inspecting the resolved working-set key** (count module segments of the resolved key relative to the referrer). Rejected in favour of checking the reference as written: the written reference's FQRN `Module` part is exactly the path into child modules, relative to the scope it resolves in, so the rule is a pure function of the parsed reference (no dependence on which scope matched).

## Chosen approach — evidence

- `xclconfig:query.go:46-51` — the only lookup special case: `find` unwraps `*resources.Output` to `output.Value`. Removing it makes `Find[types.Output]` return the entity.
- `xclconfig:query.go:272-274` — `typeable` rejects `"output"` with `NotTypeableError{Use:"Outputs"}`; removing the case lets `FindByType[types.Output](c,"output")` work (output is exactly one Go type, like variable/module/root at `query.go:276-278`).
- `xclconfig:plugins/registry/plugin_registry.go:57-61,737-765` — builtins are registered with `Builtin:true` and a `Prototype`, and `TypePath` reflects over prototypes; `type_path_test.go:48` proves a builtin (`Variable`) resolves to `{"variable"}`. So once `DefaultResources` holds `&types.Output{}`, `All[types.Output]` and `Decode` `[]*types.Output` / `*types.Output` fields work with no new code (`decode.go:80-130` goes through `typePath` + `entitiesOf`).
- `xclconfig:config.go:218-241` — `Outputs()` already returns every published value keyed by address; keep it, switching its type assertion to `*types.Output`.
- `xclconfig:internal/resources/output.go:1-17` — the struct to move; `internal/resources/default.go:9` registers it.
- `xclconfig:internal/parser/references.go:36-69` — `resolveReference` builds the module-scoped key by string join `"module." + fromModule + "." + base`, so a reference written inside module `a` to `module.b.output.y` becomes `module.a.module.b.output.y` and never resolves — re-export fails validation today (`gotchas/nested-module-keys-use-append-parent-module.md`). Fix with `fqrn.AppendParentModule(fromModule)` (`internal/resources/fqrn.go:235-251`), matching `context.go:56` and `util.go:536,552`.
- `xclconfig:internal/parser/validate.go:96-125` — `validateReferences` (stage 2) iterates every entity's `meta.Links`; the boundary check belongs here as a second problem kind in the same stage, so Apply and Validate both get it (`parser.go:536`).
- `xclconfig:internal/parser/parser.go:1102-1120` — user `depends_on` entries are parsed and appended to `Meta.Links`, so the boundary check also covers `depends_on` (e.g. `depends_on = ["module.a.resource.x.y"]` is rejected, `["module.a"]` stays valid).
- `xclconfig:internal/resources/fqrn.go:118-215` — address parsing: `module.a.output.x` → `{Module:"a",Type:"output"}`; `module.a` → `{Module:"",Type:"module",Resource:"a"}`; `module.a.b` → `{Module:"a",Type:"module",Resource:"b"}`; `module.a.b.output.x` → `{Module:"a.b",Type:"output"}`. Boundary rule as written: `Module == ""` OR (`Module` has no `.` AND `Type == "output"`).
- `xclconfig:internal/parser/util.go:506-575` (`getResourceDependencies`) and `context.go:56,120-160` already resolve module-relative links via `AppendParentModule`, and key the context `module` namespace by the written `fqdn.Module`; with the boundary in place that is always a single child-module name, so nested re-export evaluates correctly once validation lets it through.
- Fixtures: no existing configuration (`internal/test_fixtures/config/**`, `example/**/config`) references a module internal other than an output — grep of `module\.<x>\.(resource|variable|module|local)` in `.xcl/.hcl` returns nothing; `depends_on = ["module.networks"]` and `["module.consul_1"]` reference the module itself and stay valid.

## Files examined

- `xclconfig:query.go:16-51` — Find doc + output special case to remove.
- `xclconfig:query.go:258-300` — `typeable` output rejection to remove; doc comment mentions published values.
- `xclconfig:decode.go:1-150` — Decode goes through `typePath`/`entitiesOf`; no change needed beyond docs.
- `xclconfig:config.go:177-241` — `Entities`, `FindResource`, `Outputs()` (type assertion on `*resources.Output`).
- `xclconfig:encode.go:126` — output is NotEncodable; uses `resources.TypeOutput` constant only.
- `xclconfig:internal/resources/output.go` — Output struct (ResourceBase, CtyValue cty.Value, Value any, Description).
- `xclconfig:internal/resources/default.go:9` — builtin registration `"output": &Output{}`.
- `xclconfig:internal/resources/fqrn.go:58-95,235-251,266-330` — structural keywords, AppendParentModule, String forms, Match.
- `xclconfig:internal/parser/references.go` — resolveReference (string-join bug).
- `xclconfig:internal/parser/validate.go` — stages; validateReferences message format `resource '%s' refers to '%s', which is not defined anywhere in the configuration`.
- `xclconfig:internal/parser/context.go:44-200` — module namespace built from written fqdn.Module.
- `xclconfig:internal/parser/util.go:278,506-575,620-660` — Output use; dependency resolution.
- `xclconfig:internal/parser/callbacks.go:191`, `parser.go:879`, `properties.go:151`, `context.go:86` — `*resources.Output` usages to retarget.
- `xclconfig:internal/parser/references_test.go:80-130` — resolveReference tests (style model for new tests).
- `xclconfig:internal/parser/parse_test.go:181-443`, `registered_types_test.go:600`, `properties_test.go:186,195`, `internal/savedentity/savedentity_test.go:231`, `internal/resources/default_test.go:22` — test usages of `resources.Output`.
- `xclconfig:query_test.go:225-260`, `query_all_test.go:229-250`, `query_by_type_test.go:174-183` — tests asserting the old value-returning behaviour / NotTypeable for outputs; must be rewritten.
- `xclconfig:plugins/registry/plugin_registry.go:57-61,737-765`, `plugins/registry/type_path_test.go:48` — builtin TypePath.
- `xclconfig:types/resource.go`, `types/register.go` — package `types`; Output joins it (types may import `internal/cty`).
- `xclconfig:example/plugin/main.go:150-190` — uses `xcl.Find[string]` for `output.web_database` and `module.analytics.output.location`; prints `len(c.Outputs())`.
- `xclconfig:example/plugin/config/main.xcl:44-80`, `example/plugin/config/modules/db/db.xcl` — module + output usage, already output-only.
- `xclconfig:example/configonly/config/main.xcl:116`, `example/appconfig/config/app.xcl:159` — declare outputs; their main.go do not read outputs.
- `xclconfig:README.md:437-452` — "Reading values a configuration publishes" documents `Find[string]`.
- `xclconfig:docs/modules.md:55-57` — cross-module references note.
- `xclconfig:readme_test.go`, `CHANGELOG.md` — README/CHANGELOG content tests; changelog entry per spec at top, `**Breaking:**` list.
- `xcl-website:src/pages/index.mdx:140-150` — "Variables and outputs" and "Modules" feature cards.
- `xcl-website:src/pages/examples/plugins.mdx:88-125,410-450` — plugin example page mirroring example/plugin (config + program output).

## External references

- None needed; behaviour is entirely in-repo.

## Prior plans / specs consulted

- `20261003081552-e1e07cbe-config-decode` plan — historical: established the doc pattern (README section + `readme_test.go` content tests + CHANGELOG top entry with content test + `xcl-website` example page), and that the website build needs `npm ci` before `make check`.
- Epic `20261003134528-327e0657-references-and-secrets` — this spec has no dependencies and is depended on by the sensitive-values spec; later specs (user-depends-on) change how `DependsOn`/`Links` relate (`learnings/depends-on-mirrors-links.md`).

## Open assumptions

- The Go lookup API (`Find`, `FindResource`, `FindByType`, `Entities`) can still reach entities inside modules by full address; the boundary applies only to references written in configuration (spec says "rejected when the configuration is validated"). If wrong, STOP.
- A module's references that fall back to the global (root) scope (`references.go` "global scope second") keep working; the boundary is checked on the reference as written, relative to the scope it resolves in.
- Referencing a direct child module itself (`module.a`, typically in `depends_on`) stays valid; only addresses inside the module are restricted.
- `types.Output` keeps the same fields and tags as `resources.Output` (including exported `CtyValue` of type `internal/cty.Value`), so state JSON and saved-entity decoding are unchanged.
- Nested re-export evaluates correctly at walk time once validation resolves it (context and DAG already use `AppendParentModule`); verified by an end-to-end test, if it fails the fix is in scope.

## Drafting assumptions

### Chosen direction: validation-stage boundary + public types.Output (architecture)
- **Decision**: Enforce the module boundary as a second check in validation stage 2 over `Meta.Links`, judged on the reference as written; fix `resolveReference` with `AppendParentModule` so re-exports resolve; move `Output` to `types.Output` and remove the two output special cases in `query.go`, letting `Find`/`FindByType`/`All`/`Decode` return output entities through the existing scan; keep `Config.Outputs()` for values keyed by address.
- **Rationale**: Matches the spec's constraint and technical approach exactly, reuses the existing validation and query machinery, and needs no new code paths for listing or decoding.
- **Rejected**: Context-level enforcement (late, unnamed errors); alias/root-package type (two names / import cycle); resolved-key boundary check (more coupling). See research.md alternatives.

### No new error sentinel for boundary violations (architecture)
- **Decision**: Boundary problems are `ParserError`s collected into `ConfigError` like every other validation problem; the message names the referring entity, the reference and that only a module's outputs can be referenced from outside it.
- **Rationale**: Consistent with stage 2's existing "not defined" problem; the spec asks only for an error naming the reference.
- **Rejected**: A new `ErrModuleBoundary` sentinel — no consumer needs to match it.

### ux-flow.md rewrite limited to the approved wording (architecture)
- **Decision**: The knowledge-entry task rewrites only the published-values passage of `architecture/ux-flow.md` (Find[types.Output] + .Value + Outputs() note), as approved; it does not add boundary wording to the validation-stage description.
- **Rationale**: The user approved specific wording; the stage-2 text ("reaches into modules") stays accurate since references still reach modules through outputs.
- **Rejected**: Broader rewrite without approval.

### Conventions selection (architecture)
- **Decision**: Applied testing, code style, project structure and real-apply test state; deliberately did not apply shared-errors (no new error type), database, logging, dependencies, patterns (no services/handlers touched).
- **Rationale**: Only these bear on the touched surfaces.
- **Rejected**: Listing all conventions.

### Boundary applies to configuration references only (discovery)
- **Decision**: The Go lookup API (`Find`, `FindResource`, `FindByType`, `Entities`, `Decode`) keeps reaching entities inside modules by full address; only references written in configuration are restricted.
- **Rationale**: The spec states the rejection happens "when the configuration is validated"; application code is not configuration, and existing tests/examples look up module internals by address.
- **Rejected**: Restricting Go lookups too — not asked for, and would break every module-internal lookup.

### Boundary checked on the reference as written (discovery)
- **Decision**: A reference is within the boundary when its written module path is empty, or is a single child-module name and the target is an `output`. Referencing a direct child module itself (`module.a`, e.g. in `depends_on`) stays valid. The global (root-scope) fallback for references made from inside a module is unchanged.
- **Rationale**: The written FQRN's module part is exactly the path into child modules relative to the scope the reference resolves in, so the rule is simple and independent of which scope matched; keeps existing behaviour that is not in scope.
- **Rejected**: Inspecting the resolved key; removing the global fallback (out of scope).

### A boundary violation is reported instead of "not defined" (discovery)
- **Decision**: The boundary check runs before resolution; a reference that crosses the boundary is reported as a boundary problem whether or not its target exists, and is not also reported as undefined.
- **Rationale**: The reference is invalid either way; the boundary message is the more useful one and avoids two problems for one reference.
- **Rejected**: Reporting both.

### Output moves outright, no alias (discovery)
- **Decision**: `resources.Output` becomes `types.Output` with identical fields and tags; every internal use is retargeted; `resources.TypeOutput` stays as the keyword constant.
- **Rationale**: `architecture/shared-public-types-live-in-types.md`; one name per type.
- **Rejected**: Type alias in `internal/resources`.

### Separate fixture for output-entity query tests (tasks)
- **Decision**: New `output_entities` fixture (root + module output) rather than adding a root output to `registered/basic`.
- **Rationale**: Adding to the shared fixture would change entity counts asserted across many existing query tests.
- **Rejected**: Editing `registered/basic`.

### Boundary task independent of the type move (tasks)
- **Decision**: The end-to-end re-export Apply test asserts through `c.Outputs()` so Milestone 2 does not depend on Milestone 1's tasks.
- **Rationale**: Keeps milestones independently deliverable and lets the two halves run in parallel.
- **Rejected**: Making the boundary task depend on the type move.

### Plugin example output unchanged (tasks)
- **Decision**: The plugin example prints `.Value` with `%q`, keeping the printed lines identical.
- **Rationale**: Avoids churn in example tests and the website's copied program output.
- **Rejected**: Changing the printed format.

## Rehydration cues

- `spektacular spec file read 20261003153421-6ec0eab3-module-boundary-and-output-entities`
- `spektacular knowledge read` for `architecture/ux-flow.md`, `architecture/shared-public-types-live-in-types.md`, `gotchas/nested-module-keys-use-append-parent-module.md`, `architecture/config-is-the-public-query-surface.md`
- Re-read `query.go:16-60,255-300`, `internal/parser/references.go`, `internal/parser/validate.go:90-130`, `internal/resources/fqrn.go:118-251`, `config.go:218-241`.
- `grep -rn "resources\.Output" --include='*.go' .` (excluding internal/xcl, internal/cty) for the retarget list.
