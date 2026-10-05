---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Context: 20261003153421-6ec0eab3-module-boundary-and-output-entities

## Current State Analysis

- `xclconfig:internal/resources/output.go:10-17` — `Output` is declared in an internal package, so applications cannot name it.
- `xclconfig:query.go:46-51` — `Find` unwraps an output to its bare `Value`; `query.go:272-274` — `FindByType(..., "output")` is refused with `ErrNotTypeable` pointing at `Outputs()`.
- `xclconfig:config.go:218-241` — `Outputs()` returns every published value keyed by address.
- `xclconfig:internal/parser/validate.go:96-125` — stage 2 checks only that references resolve somewhere; nothing restricts what inside a module a reference may name.
- `xclconfig:internal/parser/references.go:46-56` — the module-scoped key is string-joined (`module.a.module.b...`), so a module cannot re-export its child's output (`gotchas/nested-module-keys-use-append-parent-module.md`); the DAG (`internal/parser/util.go:536,552`) and the evaluation context (`internal/parser/context.go:56`) already use `AppendParentModule`.
- `xclconfig:internal/parser/parser.go:1102-1120` — user `depends_on` entries are appended to `Meta.Links`, so stage 2 sees them.
- No existing fixture or example configuration references a module internal other than an output.
- `xclconfig:example/plugin/main.go:170-188`, `README.md:435-452`, `knowledge architecture/ux-flow.md` — all show `Find[string]` on an output address.

## Per-Task Technical Notes

### Task: Move the output type into the public types package

Requirement coverage: "Outputs are found as entities" (prerequisite: applications must be able to name the type). Spec Technical Approach: "Move the output entity type out of the internal package". Repo: `xclconfig`.

**File changes**:
- `types/output.go` (new) — declare `type Output struct { ResourceBase \`xcl:",remain"\`; CtyValue cty.Value \`xcl:"value,optional"\`; Value any \`json:"value"\`; Description string \`xcl:"description,optional" json:"description,omitempty"\` }`, importing `github.com/jumppad-labs/xcl/internal/cty`. Copy the field order, tags and doc comments exactly from `internal/resources/output.go:10-17`. Add a doc comment saying it is the entity an `output` block declares, that `Value` holds the published value as plain Go, and that `CtyValue` is used by the evaluator. Do not add a custom `MarshalJSON` (`gotchas/custom-marshaljson-changes-internal-hops.md`).
- `internal/resources/output.go:1-17` — remove the `Output` struct and keep `const TypeOutput = "output"`, so the file holds only the constant. `internal/resources/fqrn.go:83`, `encode.go:126`, `query.go:273` and the parser keep using `resources.TypeOutput`.
- `internal/resources/default.go:9` — `"output": &types.Output{}`.
- `internal/resources/default_test.go:22` — `reflect.TypeOf(&types.Output{})`.
- `internal/parser/callbacks.go:191`, `internal/parser/util.go:278`, `internal/parser/context.go:86`, `internal/parser/parser.go:879` — change the type assertions `*resources.Output` to `*types.Output`. `types` is already imported in each, so verify the imports.
- `internal/parser/properties.go:151` — `reflect.TypeOf(types.Output{})`.
- `config.go:229` — `e.(*types.Output)`.
- `query.go:49` — `entity.(*types.Output)`. The special case itself is removed by the next task, so this line only needs to compile here.
- Tests to retarget, type name only: `internal/parser/parse_test.go:181-443` (every `findResource[resources.Output]`), `internal/parser/registered_types_test.go:600-601`, `internal/parser/properties_test.go:186,195`, `internal/savedentity/savedentity_test.go:231`, `query_test.go:243,247`, `query_all_test.go:245`, `query_by_type_test.go:176`. The root-package query tests are rewritten by the next task, so only make them compile here.
- Run `grep -rn "resources\.Output\b" --include='*.go' .`, excluding `internal/xcl` and `internal/cty`. It must return nothing afterwards.

**Notes**: The plugin host's `reflect.StructOf` type map (`gotchas/plugin-types-rebuilt-with-structof.md`) is not involved, because outputs are builtins and are never plugin types. `types` must not import `internal/resources` or `internal/parser`, which would create a cycle. Only `internal/cty` is added.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution. Move the type, then run `go build ./... && go test ./...`.

### Task: Return outputs as entities from every lookup

Requirements: "Outputs are found as entities" and "All published values remain available together". Repo: `xclconfig`.

**File changes**:
- `query.go:16-30` — rewrite the `Find` doc comment. Drop the paragraph "Where the address names a value the configuration publishes, the resolved value is returned…". Say instead that an output address returns the `types.Output` entity, whose `Value` holds the published value.
- `query.go:46-51` — delete the `if output, ok := entity.(*types.Output)` special case, so `find` always returns `As[T](entity)`.
- `query.go:258-275` — in `typeable`, remove `case resources.TypeOutput:` returning `NotTypeableError{Use: "Outputs"}`, and add `resources.TypeOutput` to the `case resources.TypeVariable, resources.TypeModule, resources.TypeRoot:` list ("each is exactly one Go type"). Update the doc comment at `query.go:258-266`, removing the sentence about published values never being typeable.
- `config.go:218-224` — reword the `Outputs()` doc comment. It returns every published value keyed by address, the same value as `Find[types.Output](c, addr).Value`. Remove the sentence saying it is the call `FindByType` names for published values. The body is unchanged apart from the earlier retarget.
- `errors/query_errors.go:34-37,86-95` — check the `ErrNotTypeable`/`NotTypeableError` doc comments for any mention of outputs or "Outputs", and remove it if present.
- `decode.go` — no logic change. `typePath` resolves `types.Output` through the builtin prototype (`plugins/registry/plugin_registry.go:57-61,737-765`). If the `Decode` doc comment (`decode.go:10-36`) lists builtins, add outputs.
- New fixture `internal/test_fixtures/config/output_entities/main.xcl`. It declares a root `output "greeting" { value = "hello" }` and `module "inner" { source = "./module" }`. Add `internal/test_fixtures/config/output_entities/module/inner.xcl` with `output "location" { value = "eu-west" }`. Use a separate fixture so the counts in the existing `setupFindConfig` tests (`registered/basic`) do not change.
- `query_test.go:222-260` — replace `TestFindReturnsThePublishedValueRatherThanItsDeclaration`, `TestFindDoesNotReturnThePublishedValuesDeclaration` and `TestFindResolvesAPublishedValueDeclaredInsideAModule` with separate tests. `TestFindReturnsARootOutputAsAnEntity` asserts `*types.Output`, `Meta.ID == "output.greeting"` and `Value == "hello"`. `TestFindReturnsAModuleOutputAsAnEntity` asserts `module.inner.output.location` and `Value == "eu-west"`. `TestFindRefusesAnOutputAsAPlainValue` asserts that `Find[string]` gives `ErrTypeMismatch`. Each is its own function, with positive and negative tests kept apart. Each applies the fixture with a real `Apply` (`conventions/test-state-from-real-apply.md`), following the `setupFindConfig` helper pattern in `query_setup_test.go`.
- `query_by_type_test.go:174-183` — replace `TestFindByTypeRejectsPublishedValuesAndNamesTheCallThatReturnsThem` with `TestFindByTypeReturnsEveryOutputAsAnEntity`, which expects 2 entities with both addresses, in declaration order.
- `query_all_test.go:225-250` — update the block comment. Keep `TestOutputsReturnsEveryPublishedValueKeyedByAddress`. Rewrite `TestOutputsReturnsTheResolvedValueRatherThanItsDeclaration` so it compares against `Find[types.Output](...).Value`. Add `TestAllReturnsEveryOutputAsAnEntity`, and `TestOutputsHasOneEntryPerDeclaredOutput` on the new fixture (2 entries).
- `decode_test.go` — add `TestDecodeFillsAnOutputsField`, with a struct holding `Outputs []*types.Output` that is filled with both outputs.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Single agent. Change the code first, then write the tests, then run `go test ./...`.

### Task: Read outputs as entities in the plugin example

Requirement: "Bundled examples follow the module boundary" (outputs read as entities). Repo: `xclconfig`.

**File changes**:
- `example/plugin/main.go:170-188` — replace `xcl.Find[string](c, "output.web_database")` and `xcl.Find[string](c, "module.analytics.output.location")` with `xcl.Find[types.Output](...)`, and print `webDatabase.Value` and `moduleLocation.Value` with `%q`. The value is `any` holding a string, so `%q` prints the same quoted text. Confirm this against `example/plugin/main_test.go:840-841`. Update the comment at `:170-172`: outputs are entities like everything else, and their published value is on `.Value`. Keep the `len(c.Outputs())` line and its comment. Import `github.com/jumppad-labs/xcl/types` and follow the existing import grouping.
- `example/plugin/main_test.go:840-841` — the expected lines should stay identical. Adjust them only if the formatting genuinely changes.
- `example/plugin/config/main.xcl:44-56` and `example/plugin/config/modules/db/db.xcl` — already output-only. No change.
- `static_examples_test.go` — check that no AST guard forbids importing `types` from an example main. None is expected (`:99-160` guard handlers, logger and registry).

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential. Run `go test ./example/plugin/...`, and the root `static_examples_test.go`.

### Task: Resolve re-exported outputs of nested modules

Requirement: "The boundary holds at every level of nesting" (re-export must work). Repo: `xclconfig`.

**File changes**:
- `internal/parser/references.go:46-56` — replace `scoped := "module." + fromModule + "." + base` with a key built from `fqrn.AppendParentModule(fromModule)`, then `.StringWithoutAttribute()` (`internal/resources/fqrn.go:235-251,296-330`), following `internal/parser/context.go:56` and `internal/parser/util.go:536,552`. Keep the global-scope fallback at `:58-65` unchanged. Fix the empty `import ()` at `:3` if gofmt/vet complains. Update the doc comment to say that scoped keys are composed with `AppendParentModule` so nested module references resolve (`gotchas/nested-module-keys-use-append-parent-module.md`).
- New fixture `internal/test_fixtures/config/module_reexport/main.xcl`, holding `module "a" { source = "./a" }` and `output "deep" { value = module.a.output.from_b }`. Add `.../module_reexport/a/a.xcl`, holding `module "b" { source = "./b" }` (path relative to `a`; check how relative sources resolve in `internal/test_fixtures/config/disabled/modules/resources.xcl` and its `sub-modules`) and `output "from_b" { value = module.b.output.value }`. Add `.../module_reexport/a/b/b.xcl`, holding `output "value" { value = "from-b" }`. The next task reuses this fixture.
- `internal/parser/references_test.go` — add `TestResolveReferenceResolvesAChildModuleOutputFromInsideAModule`, in which `resolveReference("module.b.output.from_b"... )` with `fromModule "a"` gives found and key `module.a.b.output.value` (match the actual names). Add `TestResolveReferenceStillResolvesUnqualifiedReferencesInsideAModule`, a regression test with the same shape as `:91-100` that passes unchanged. Use the `parseFixture` helper (`references_test.go`).

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential. Run `go test ./internal/parser/...`.

### Task: Reject references that cross a module boundary

Requirements: "Only outputs are reachable from outside a module" and "The boundary holds at every level of nesting". Spec constraint: `output` stays a builtin, and the boundary is enforced by validating references. Repo: `xclconfig`.

**File changes**:
- `internal/parser/references.go` — add a pure function `crossesModuleBoundary(fqrn *resources.FQRN) bool` (or `withinModuleBoundary`). It returns false when `fqrn.Module == ""`. It returns true when `fqrn.Module` contains `"."`, because the reference reaches a grandchild. Otherwise it returns `fqrn.Type != resources.TypeOutput`. Document the rule: a reference may reach into a module only to that module's output, and only into a direct child. The module path is relative to the scope the reference is written in.
- `internal/parser/validate.go:96-125` (`validateReferences`) — for each `link`, parse it with `p.addressParser().Parse(link)`. If parsing fails, fall through to the existing undefined check. If it crosses the boundary, append `errors.NewParserError(meta.File, meta.Line, meta.Column, fmt.Sprintf("resource '%s' refers to '%s', which is inside module '%s'; only a module's outputs can be referenced from outside it", meta.ID, link, fqrn.Module))` and `continue`, so the link is not also reported as undefined. Otherwise run the existing resolution check. Update the stage doc comment and the stage-2 comment in `validate()` (`:39-41`) to mention the boundary.
- `internal/parser/validate.go:59-90` (`validateProperties`) — no change needed, because stage 3 is skipped when stage 2 reports problems.
- New fixtures under `internal/test_fixtures/config/module_boundary/`:
  - `internal_resource/main.xcl` plus `module/m.xcl`. The root `output "x" { value = module.m.resource.container.c.name }`, or a registered type available in parser tests (see `internal/test_fixtures/plugin/structs`), with the module declaring that resource and an `output "name"`.
  - `through_output/main.xcl`, the same module with root `output "x" { value = module.m.output.name }`.
  - `internal_variable/main.xcl`, with a root reference to `module.m.variable.v`.
  - `depends_on_internal/main.xcl`, with `depends_on = ["module.m.resource.container.c"]`.
  - `depends_on_module/main.xcl`, with `depends_on = ["module.m"]`.
  - Nesting reuses `internal/test_fixtures/config/module_reexport` from the previous task. Add `module_boundary/grandchild_output/main.xcl`, which references `module.a.b.output.value` from the root with the same `a/` and `b/` structure (copy it, or point `source` at `../../module_reexport/a`), and `module_boundary/grandchild_module/main.xcl`, which references `module.a.b` in `depends_on`.
- `internal/parser/validate_test.go` — add one function per case, following existing validate tests (look for helpers that run validation on a fixture and collect `ConfigError.Errors`):
  - `TestValidateRejectsAReferenceToAModuleResource`, which asserts exactly one problem whose text contains the reference and "only a module's outputs", and no "not defined" problem.
  - `TestValidateAcceptsAReferenceToAModuleOutput`.
  - `TestValidateRejectsAReferenceToAModuleVariable`.
  - `TestValidateRejectsADependsOnNamingAModuleInternal`.
  - `TestValidateAcceptsADependsOnNamingAChildModule`.
  - `TestValidateRejectsAReferenceToAGrandchildModuleOutput`.
  - `TestValidateRejectsAReferenceToAGrandchildModule`.
  - `TestValidateAcceptsAReExportedGrandchildOutput`.
- `internal/parser/references_test.go` — unit tests for the predicate, one function per shape: root reference, child output, child output with attribute, child module itself (accepted), child resource, child variable, child nested module, grandchild output (rejected).
- `config_validate_test.go` — add `TestApplyRefusesAReferenceThatCrossesAModuleBoundary`. It asserts that `Apply` returns a `*errors.ConfigError` and that the state store holds nothing.
- `config_test.go` (or a new `config_module_boundary_test.go` in package `xcl`) — add `TestApplyResolvesAReExportedGrandchildOutput`. It runs a real `Apply` on `module_reexport`, then `Find[types.Output](c, "output.deep")`, and `.Value == "from-b"`. This depends on `types.Output` from the move task, which is on a separate branch of the graph. If that task has not landed, assert through `c.Outputs()["output.deep"]` instead, which works either way. Prefer `Outputs()` to keep this task independent.
- Run the whole suite. Any existing fixture or example that crosses the boundary will now fail. Research found none (`grep -rnE 'module\.[a-z_0-9]+\.(resource|variable|module|local)'` over `.xcl`/`.hcl` is empty). If one appears, convert it to read through an output; do not weaken the rule.

**Complexity**: Medium
**Token estimate**: ~45k tokens
**Agent strategy**: 2 parallel agents. One writes the fixtures and the parser tests. The other writes the predicate, the validation change and the root-package Apply tests. Integrate them sequentially and run `go test ./...`.

### Task: Document the module boundary and output entities in the library

Requirement: "Module boundary and outputs are documented" (library part). Repo: `xclconfig`.

**File changes**:
- `README.md:435-452` (`#### Reading values a configuration publishes`) — rewrite it. An `output` is an entity like any other. `xcl.Find[types.Output](c, "output.web_database")` returns it, and `out.Value` holds the published value. A module output works the same way (`module.analytics.output.location`). `xcl.FindByType[types.Output](c, "output")` or `xcl.All[types.Output](c)` lists them, and `c.Outputs()` returns all values keyed by address. Remove "comes back as the value itself rather than the declaration". Keep the note that a complete address is needed.
- `README.md:478` — the error vocabulary is unchanged. Check that nothing near it says outputs are not typeable.
- `README.md:1105-1140` (`### Outputs` under `## Modules`) — add a `### The module boundary` subsection (or a paragraph in Outputs). Only a module's outputs can be referenced from its parent: a reference to anything else inside a module, or into a module nested inside it, fails validation with an error naming the reference. Include a worked re-export example, with a module `a` that uses module `b` and re-exports `b`'s output as `output "from_b" { value = module.b.output.value }`, and the root reading `module.a.output.from_b`. Note that `depends_on = ["module.a"]` is still allowed.
- `docs/modules.md:55-57` — extend the "Cross-module references" paragraph with the same boundary rule. Say where it is enforced (validation stage 2, before any walk) and add a short re-export example.
- `CHANGELOG.md` (top) — add `## 20261003153421-6ec0eab3-module-boundary-and-output-entities`, following the existing entry style (`CHANGELOG.md:1-20`). Explain both behaviours, then a `**Breaking:**` list. First, `Find[string]` (or any non-`types.Output` type) on an output address now fails with `ErrTypeMismatch`: use `Find[types.Output]` and `.Value`, or `c.Outputs()`. Second, configurations that reference a module's resources, variables or nested modules, or a grandchild module's outputs, now fail validation: re-export through outputs. Also mention that `FindByType(..., "output")` no longer returns `ErrNotTypeable`.
- `readme_test.go` — add `TestReadmeDocumentsReadingOutputsAsEntities`, which requires `xcl.Find[types.Output](` and `.Value` and requires that the text does not contain `xcl.Find[string](c, "output.`. Add `TestReadmeDocumentsTheModuleBoundary`, which requires the boundary subsection heading and a re-export snippet. Add `TestChangelogRecordsTheModuleBoundaryAndOutputEntities`, which requires the entry heading and `types.Output`, following `readme_test.go:59-110`. Add a docs/modules.md content test if a similar pattern exists; otherwise the README tests suffice.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential. Run `go test -run 'Readme|Changelog' .`.

### Task: Document the module boundary and output entities on the site

Requirement: "Module boundary and outputs are documented" (site part). Repo: `xcl-website`.

**File changes**:
- `xcl-website:src/pages/index.mdx:146-150` (Modules `FeatureCard`) — say that only a module's outputs are reachable from its parent, read with `module.<name>.output.<name>`, and that a nested module's value is exposed by re-exporting it as an output. Keep "Modules can nest."
- `xcl-website:src/pages/index.mdx:140-144` (Variables and outputs card) — add that outputs are entities in Go: `Find[types.Output]` and `.Value`.
- `xcl-website:src/pages/examples/plugins.mdx:88-125` — where the module and its output are shown, add a short prose note and a snippet on the boundary and on re-exporting. Wherever the page shows or describes the program reading outputs (around `:410-450`, where the program output is listed), show `xcl.Find[types.Output](c, "output.web_database")` and `.Value`, matching `xclconfig:example/plugin/main.go` after the plugin example task. The printed output lines (`:445-446`) are unchanged.
- Gate: `npm ci`, then `make check` in `xcl-website`. `make check` prompts interactively without `node_modules`. Note that a `node_modules` directory already exists in this checkout.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

### Task: Bring the user-flow knowledge entry into line with output entities

Settled user decision: the spec contradicted `architecture/ux-flow.md`, and the user chose option A, rewriting the entry with the approved wording. Repo: `xclconfig` (its knowledge store).

**File changes**:
- Knowledge entry `architecture/ux-flow.md` (tier `repo`, store `xclconfig`). Invoke the `spek-knowledge` skill to update it. Never edit the store file with file tools, and never build its path by hand. In section "3. Query the Configuration", replace the paragraph "Values the configuration publishes are read the same way, and come back as the value rather than the declaration that produced it:" and its code block (`url, err := xcl.Find[string](config, "output.web_database")` / `published := config.Outputs()`) with outputs read as entities. For example:

  ```go
  out, err := xcl.Find[types.Output](config, "output.web_database")
  url := out.Value // the published value
  ```

  Follow it with a note that `config.Outputs()` returns every published value in one call, keyed by address. In the Architecture diagram's public-API box, the `Entities/EntityCount/Outputs` line stays. Change nothing else in the entry.
- Read the entry back through `spektacular knowledge read` to confirm the change.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, through the `spek-knowledge` skill.

## Testing Strategy

- **Move the output type into the public types package** — behaviour-neutral; the existing parser, saved-entity and state tests, retargeted by name, are the proof.
- **Return outputs as entities from every lookup** — root-package tests on a new `output_entities` fixture via real Apply: Find returns entities (root and module), `Find[string]` gives `ErrTypeMismatch`, FindByType/All/Decode return every output, `Outputs()` keyed by address. One function per case, positive and negative separated.
- **Read outputs as entities in the plugin example** — existing example tests; printed lines unchanged.
- **Resolve re-exported outputs of nested modules** — resolver unit tests for the nested key and a regression for unqualified references.
- **Reject references that cross a module boundary** — predicate unit tests per reference shape; validation tests per fixture (internal resource, variable, depends_on internal, depends_on module, grandchild output, grandchild module, re-exported grandchild); Apply refuses a crossing configuration; end-to-end re-export resolves to the grandchild's value after Apply.
- **Document … in the library** — README/CHANGELOG content tests in `readme_test.go`.
- **Document … on the site** — site build and type-check (`npm ci`, `make check`); browser read-through is a manual item in the implementation test plan.
- **Bring the user-flow knowledge entry into line** — read back through `spektacular knowledge read`.
- The spec has no success metrics.

## Project References

- Spec: `20261003153421-6ec0eab3-module-boundary-and-output-entities` (no design references).
- Epic: `20261003134528-327e0657-references-and-secrets` — this spec has no dependencies; the sensitive-values spec depends on it; `20261003153421-c283547c-user-depends-on` later changes DependsOn/Links.
- Knowledge: `architecture/ux-flow.md` (rewritten by this plan, user decision option A), `architecture/shared-public-types-live-in-types.md`, `architecture/config-is-the-public-query-surface.md`, `gotchas/nested-module-keys-use-append-parent-module.md`, `gotchas/custom-marshaljson-changes-internal-hops.md`, `learnings/depends-on-mirrors-links.md`; conventions listed in plan.md.
- Repo roots: `xclconfig` → `/home/nicj/code/github.com/jumppad-labs/xcl`; `xcl-website` → `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Milestones 1 and 2 are independent and can be implemented in parallel; Milestone 3 follows both.

## Migration Notes

- Applications calling `Find[string]` (or any non-`types.Output` type) on an output address must switch to `Find[types.Output](...).Value`, or read `c.Outputs()[address]`.
- Configurations that reference a module's resources, variables or nested modules, or a grandchild's outputs, must re-export the value as an output of each module in between and reference that output.
- Saved state is unchanged: the output type's serialised form is identical, so no state migration is needed.

## Performance Considerations

- The boundary check is a constant-time inspection of each already-parsed reference during validation; negligible.
