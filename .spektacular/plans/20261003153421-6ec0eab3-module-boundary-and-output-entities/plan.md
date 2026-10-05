---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Plan: 20261003153421-6ec0eab3-module-boundary-and-output-entities

<!-- Metadata -->
<!-- Created: 2026-10-05T09:45:06Z -->
<!-- Commit: d554c1d -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

A module's outputs become the only way to reach inside it from configuration, at every level of nesting: validation rejects any reference into a module other than to a direct child's output, and a module can re-export a nested module's value as its own output. In the Go API, outputs become public `types.Output` entities that `Find`, `FindByType`, `All` and `Decode` return like any other entity, with the published value on `.Value`, while `Outputs()` still returns every value keyed by address. Module authors control exactly what they expose, and application code handles outputs consistently; the library docs, the documentation site, the plugin example and the `architecture/ux-flow.md` knowledge entry are brought into line.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, never mix positive and negative cases in one test, favour verbosity** — every new boundary, re-export and output-entity test is a separate named function, with an accepted reference and a rejected one each in its own test.
- **Code style: standard Go conventions, `any` over `interface{}`, descriptive names** — applies to the new `types/output.go`, the boundary check and the reference-resolution fix.
- **Project structure: `/internal` is private and `/pkg`/public packages are for library code** — this is why the output type leaves `internal/resources` for the public `types` package. The `architecture/shared-public-types-live-in-types.md` knowledge entry is cited alongside it.
- **Generate test state with a real apply, not a hand-written state file** — the end-to-end re-export and output-entity tests run a real `Apply` against fixtures and never hand-write state.
- **Shared errors live in the `errors` package** — considered and deliberately not used. The boundary problem is reported as a `ParserError` collected into the existing `ConfigError`, as every other validation problem is, so no new sentinel is introduced.

## Architecture & Design Decisions

The work has two independent halves, both in the `xclconfig` repo, plus documentation in `xclconfig` and `xcl-website`.

**The module boundary is a validation rule.** Stage 2 of validation (`validateReferences`, `internal/parser/validate.go:96-125`) gains a boundary check that runs over every entity's `Meta.Links`, which hold both interpolated references and user-written `depends_on` entries (`internal/parser/parser.go:1102-1120`). The check is a pure function of the reference as written. The module part of a parsed address is the path into child modules, relative to the scope the reference is made from. A reference is within the boundary when that module part is empty, or when it is a single child-module name and the target is an `output`. So `module.a.output.x` and `module.a` are allowed. `module.a.resource.container.c`, `module.a.variable.v`, `module.a.b` and `module.a.b.output.x` are rejected, each with a problem that names the referring entity and the reference. A crossing reference is reported once, as a boundary problem, and not also as "not defined". Because `output` stays a builtin entity type and nothing changes in how modules are evaluated, this satisfies the spec's constraint that the boundary is enforced by validating references. Apply already runs the same validation before walking (`internal/parser/parser.go:536`), so both `Validate` and `Apply` refuse a crossing reference. The rule applies to configuration only. The Go lookups (`Find`, `FindResource`, `Entities`) still reach a module's internals by full address, because application code is not configuration.

**Re-export needs a resolution fix.** A module that re-exports its child's output (`output "x" { value = module.b.output.y }` inside module `a`) fails validation today. `resolveReference` (`internal/parser/references.go:46-56`) builds the scoped key by string concatenation, which gives `module.a.module.b.output.y` (`gotchas/nested-module-keys-use-append-parent-module.md`). The scoped key is rebuilt with `FQRN.AppendParentModule`, which gives `module.a.b.output.y`. The DAG and the evaluation context already build their keys this way (`internal/parser/util.go:536,552`, `internal/parser/context.go:56`). The context keys its `module` namespace by the module name as written, and with the boundary in place that is always a single direct child. So a nested re-export evaluates correctly, and an end-to-end test across three levels proves it.

**Outputs become public entities.** `resources.Output` moves to `types.Output` with identical fields and tags, so state, saved-entity decoding and plugins see the same JSON. The move follows `architecture/shared-public-types-live-in-types.md`: the parser needs the type and the root package imports the parser, so the type cannot live in the root package. `resources.TypeOutput` stays as the keyword constant, and every internal use is retargeted. The query layer drops its two output special cases: `find` no longer unwraps `output.Value` (`query.go:46-51`), and `typeable` no longer refuses `"output"` (`query.go:272-274`). An output is then exactly one Go type, like `variable` and `module`. Builtins are registered with a prototype that `PluginRegistry.TypePath` reflects over (`plugins/registry/plugin_registry.go:57-61,737-765`), so `Find[types.Output]`, `FindByType[types.Output](c, "output")`, `All[types.Output]` and `Decode` into `[]*types.Output` and `*types.Output` all work through the existing scan, with no new code paths. `Config.Outputs()` stays as the single call for every published value keyed by address. These are breaking changes: `Find[string]` on an output address now fails with `ErrTypeMismatch`, and configurations that reach into a module other than through an output now fail validation. Both are listed under **Breaking** in the changelog.

**Documentation follows the established pattern.** The README section on published values, `docs/modules.md`, the CHANGELOG and the README/CHANGELOG content tests (`readme_test.go`) are updated in `xclconfig`. The plugin example reads outputs as entities. In `xcl-website`, the Modules feature card and the plugin example page are updated. The knowledge entry `architecture/ux-flow.md` is rewritten through the `spek-knowledge` skill to show `Find[types.Output]` with `.Value` and a note on `Outputs()`, as the user decided. The other options, enforcing the boundary in the evaluation context, a type alias, a root-package type and checking resolved keys, are recorded with evidence in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Output entity type (moved, public)**: the declaration of a published value. It holds the entity's base fields, its description, its evaluated value as a cty value for the evaluator, and its published value as a plain Go value. It moves from the internal resources package to the public `types` package so applications can name it, with fields and serialised form unchanged. The builtin type registry registers it as the `output` builtin. The parser, the evaluator, saved-entity decoding and the query layer all refer to it by its new name.
- **Reference resolver (changed)**: answers whether a reference resolves, and to which working-set entry. It builds module-scoped keys through the address type's own parent-module composition rather than string joining, so a reference written inside a module to that module's child module's output resolves at any depth. The DAG builder and the evaluation context already compose keys this way, and the resolver now does too.
- **Module boundary check (new, inside validation stage 2)**: judges every reference an entity holds, interpolated or written in `depends_on`, against the boundary rule as written. The module path must be empty, or a single child module whose output is the target. A crossing reference becomes a positioned validation problem that names the referring entity and the reference, and is not also reported as undefined. It sits beside the existing undefined-reference check, so `Validate` and `Apply` both enforce it and the later property stage is skipped when it fires.
- **Typed query layer (changed)**: `Find`, `FindByType`, `FindOne`, `All` and `Decode` no longer treat outputs specially. An output address yields the output entity, and `output` is a typeable kind that maps to exactly one Go type. Listing and decoding outputs therefore reuse the existing scan and conversion. `Config.Outputs()` is unchanged in shape and still returns every published value keyed by address.
- **Bundled examples (changed)**: the plugin example reads its root and module outputs as entities and prints their `.Value`. The configurations of the configuration-only, application and plugin examples already follow the boundary and are kept that way.
- **Library documentation (changed)**: the README's published-values section, the modules guide, the changelog and the README/changelog content tests. Together they describe the output-only boundary with a re-export example, and reading outputs as entities.
- **Documentation site (changed, `xcl-website`)**: the Modules feature card on the home page and the plugin example page describe the boundary and show an output read as an entity.
- **Knowledge entry `architecture/ux-flow.md` (changed, through `spek-knowledge`)**: its published-values passage shows `Find[types.Output]` with `.Value`, and notes `Outputs()` for all values keyed by address.

## Data Structures & Interfaces

**`types.Output` (moved, now public).** This is the declaration of a published value, which applications can now name. Its shape and serialised form are the same as the internal type it replaces, so saved state and plugin payloads do not change.

```go
package types

// Output is a value a configuration or module publishes
type Output struct {
    ResourceBase `xcl:",remain"`

    CtyValue    cty.Value // evaluated value, used by the evaluator
    Value       any       `json:"value"`                 // the published value, as plain Go
    Description string    `xcl:"description,optional" json:"description,omitempty"`
}
```

The `output` keyword constant stays where it is. The builtin registry registers `&types.Output{}` under `output`.

**Public query contract (changed behaviour, unchanged signatures).**

```go
xcl.Find[types.Output](c, "module.a.output.x")   // *types.Output; .Value holds the published value
xcl.FindByType[types.Output](c, "output")        // []*types.Output, every declared output at any module depth
xcl.All[types.Output](c)                          // same as above
xcl.Decode(c, &struct{ Outputs []*types.Output }{}) // filled with the same entities
c.Outputs()                                       // map[string]any, address -> published value (unchanged)
```

`Find[string]` on an output address now returns an error matching `ErrTypeMismatch`, where it used to return the value. `FindByType(..., "output")` no longer returns `ErrNotTypeable`.

**Validation problem (new message, existing type).** A boundary violation is an `*errors.ParserError` placed at the referring entity and collected into the existing `*errors.ConfigError`. It names the referring entity and the reference, and says that only a module's outputs can be referenced from outside it. No new error type or sentinel is added.

**Internal resolver contract (changed).** `resolveReference(reference, fromModule)` keeps its signature and result. Its module-scoped key is now composed with `FQRN.AppendParentModule`, so for `fromModule = "a"` the reference `module.b.output.y` resolves to the key `module.a.b.output.y`.

## Implementation Detail

**One new pattern: a boundary rule judged on the reference as written.** Validation's reference stage already walks every entity's links in a stable order. It gains a small, pure predicate that classifies a parsed reference as within or across the module boundary. The predicate looks only at the reference's module path and target kind, never at the working set, so it is unit-testable without parsing a configuration and gives the same answer whichever scope the reference later resolves in. The stage checks the boundary first and only then asks whether the reference resolves, so each bad reference produces exactly one problem. Readers of the validation code will find the boundary rule next to the "not defined" rule, worded the same way and placed at the same position: the referring entity's block.

**Existing patterns followed, not new ones.**
- Module-scoped keys are composed with the address type's parent-module helper everywhere. The resolver joins the DAG builder and the evaluation context, which already do this. This removes the last string-joined module key in the parser.
- An output becomes an ordinary builtin entity, like `variable` and `module`. The query layer loses its only per-kind special cases, so finding, listing, `All` and `Decode` treat every builtin uniformly. No new query code is written: removing the special cases is what makes outputs listable and decodable.
- Public types that the parser also needs live in `types`, so the output type takes the same place `ResourceBase` and `Meta` already have.

**Code-shape change: a type move with a mechanical retarget.** The internal `Output` struct disappears from the internal resources package and appears in `types` unchanged. Every internal reference to it is renamed: parser, evaluator, property checks, saved-entity decoding and the builtin registry, plus their tests. The `output` keyword constant does not move. The move is behaviour-neutral by construction, because the fields and tags are byte-identical. The existing parser and state tests prove it before any query behaviour changes.

**Public surface UX.** Application code reads an output the way it reads any entity: `Find[types.Output]`, then `.Value`. It lists outputs with `FindByType[types.Output](c, "output")` or `All[types.Output]`, and still gets every value in one call through `Outputs()`. Configuration authors who reach into a module get a validation error that names the reference and says only outputs are reachable. To expose a nested value they re-export it as an output of each module in between, and the documentation shows that pattern.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **`internal/resources` address model (`FQRN`, `AddressParser`, `AppendParentModule`)** — provides parsing of references into module path, type and name, and module-relative key composition. Used as is, with no changes.
- **`internal/parser` validation stages and reference resolver** — the host for the boundary check. The resolver is changed to compose keys with `AppendParentModule`.
- **`plugins/registry` builtin registration and `TypePath`** — makes `types.Output` reachable by `All` and `Decode` through its builtin prototype. No changes beyond the retargeted builtin set it reads from `internal/resources`.
- **`types` package** — receives the output type. It may import `internal/cty`, which is in the same module, so no new external dependency is introduced.
- **`errors` package (`ParserError`, `ConfigError`)** — carries the new validation problem. Unchanged.
- **`xcl-website` repo** — hosts the home page and the plugin example page that are updated. Its snippets are copied text with no code dependency on `xclconfig`, so the site change can land independently. Its gate is its own build and type-check, after installing its packages.
- **Knowledge entries** — `architecture/shared-public-types-live-in-types.md` decides where the output type lives. `gotchas/nested-module-keys-use-append-parent-module.md` identifies the resolver fix. `architecture/ux-flow.md` is rewritten by this plan, with the wording the user approved.
- **Upstream specs and plans** — none. This spec has no dependencies in the epic `20261003134528-327e0657-references-and-secrets`, and the sensitive-values spec depends on it. The later user-depends-on spec changes how `depends_on` relates to `Meta.Links`. It must keep user-written `depends_on` entries subject to this boundary check.
- **External libraries** — none added. Tests use testify `require`, which is already a dependency.

## Testing Approach

All tests follow the project's conventions. They use testify `require`, there are no table-driven tests, every accepted case and every rejected case has its own test function, and test state comes from a real `Apply` rather than a hand-written state file.

**Unit tests (parser).** The boundary predicate is covered reference shape by reference shape. A root-level reference is accepted. So is a direct child's output, a direct child's output with a trailing attribute, and a direct child module itself. A direct child's resource, variable or nested module is rejected, and so is a grandchild's output. The resolver gets a regression test proving that a reference written inside a module to its own child's output resolves to the correctly composed nested key. Validation-level tests parse small fixtures and assert three things. A configuration referencing a module internal fails with one problem naming that reference, and is not also reported as undefined. The same configuration referencing the module's output instead validates. A `depends_on` entry naming a module internal is rejected, while `depends_on` naming the module itself still validates.

**Nesting, end to end.** The spec's nesting criterion runs on a three-level fixture: root uses module A, and A uses module B. A root reference to anything inside B, including B's outputs, fails validation. A root reference to an output of A that re-exports B's output validates, and after `Apply` it resolves to B's value. This is the load-bearing guarantee that the boundary holds at every depth and that re-export works.

**Query-layer tests (root package).** These replace the tests that asserted the old value-returning behaviour. `Find[types.Output]` on a root output and on a module output returns the entity, with `.Value` holding the published value. `Find[string]` on an output address fails with `ErrTypeMismatch`. `FindByType[types.Output](c, "output")` and `All[types.Output]` return every declared output as entities. `Decode` fills a `[]*types.Output` field with the same entities. `Outputs()` returns one entry per declared output, keyed by address, each holding that output's value.

**Behaviour-neutral move.** The type move is proven by the existing parser, saved-entity and state tests, which are retargeted to `types.Output` and must pass unchanged in assertion.

**Examples and documentation.** The existing example tests run each bundled example's `Validate` and `Apply` and check its printed output, so they cover "examples run under the module boundary". The plugin example's expected output is updated wherever reading outputs as entities changes what it prints. README and CHANGELOG content tests are extended so they fail if the published-values section stops showing `Find[types.Output]` and `.Value`, if the module-boundary and re-export documentation is removed, or if the changelog entry is missing. The site has no content tests. Its build and type-check are the automated gate.

**Success metrics.** The spec defines none, so there are none to verify.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: read the updated `xcl-website` home page Modules card and plugin example page in a browser, and check that they describe the output-only boundary with a re-export example and show an output read as an entity.

**Deliberate gaps.** Go lookups of module internals by full address are not restricted, so no test asserts a restriction there. Existing tests continue to cover those lookups. The evaluation context is not changed, so it gets no new tests beyond the end-to-end nesting test.

## Milestones & Tasks

### Milestone 1: Applications read outputs as entities

**What changes**: Application code can name the output type, `types.Output`, and works with outputs the same way as every other entity. Looking up an output by its address returns the output entity, and the published value is on its `.Value`. Listing outputs by type, `All`, and `Decode` into an outputs field all return every declared output as entities. `Outputs()` still returns every published value in one call, keyed by address. The plugin example reads its root and module outputs this way. Asking for an output as a plain value, as in `Find[string]`, now fails with a type mismatch, and that is the breaking part of this milestone.

**Validation point**: The full test suite passes, including the retargeted parser, state and saved-entity tests. New query tests prove that finding, listing, `All` and `Decode` return output entities and that `Outputs()` returns values keyed by address. The plugin example's tests pass with its outputs read as entities.

#### - [ ] Task: Move the output type into the public types package
**Id:** 1bee269b-f277-452b-be57-44c2c927a1d8
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

The output declaration type moves from the internal resources package to the public `types` package as `types.Output`, so applications can name it. Its fields, tags and serialised form stay exactly the same. Every internal user of the type is retargeted: the builtin registration, the parser, the evaluator, property checks, saved-entity decoding and their tests. Nothing changes in behaviour, and the existing tests prove that.

*Technical detail:* [context.md#task-move-the-output-type-into-the-public-types-package](./context.md#task-move-the-output-type-into-the-public-types-package)

**Acceptance criteria**:
- [ ] Applications can refer to `types.Output`, and the internal resources package no longer declares an output type.
- [ ] Saved state and plugin payloads for outputs are byte-for-byte what they were before the move.
- [ ] Every existing parser, state and saved-entity test passes with its assertions unchanged apart from the type name.

#### - [ ] Task: Return outputs as entities from every lookup
**Id:** ac841670-bdfa-4261-8268-c6b6bd23eb9e
**Repo:** xclconfig
**Depends on:**
- 1bee269b-f277-452b-be57-44c2c927a1d8 — Move the output type into the public types package
**Execution:** agent

The query layer stops treating outputs specially. Looking up an output by address returns the output entity, and the published value is on its `.Value`. Outputs become a listable kind, so `FindByType`, `All` and `Decode` return every declared output as entities. `Outputs()` keeps returning every published value keyed by address. The tests that asserted the old value-returning behaviour are replaced with tests for the new behaviour.

*Technical detail:* [context.md#task-return-outputs-as-entities-from-every-lookup](./context.md#task-return-outputs-as-entities-from-every-lookup)

**Acceptance criteria**:
- [ ] Looking up a root output or a module output by address returns the output entity, and its value field holds the published value.
- [ ] Asking for an output as a plain value fails with a type-mismatch error rather than returning the value.
- [ ] Listing outputs by type, `All` and `Decode` each return every declared output, at any module depth, as entities.
- [ ] `Outputs()` returns one entry per declared output, keyed by address, each holding that output's value.

#### - [ ] Task: Read outputs as entities in the plugin example
**Id:** b989237b-cb9c-4d3b-9d86-5abefdfd2154
**Repo:** xclconfig
**Depends on:**
- ac841670-bdfa-4261-8268-c6b6bd23eb9e — Return outputs as entities from every lookup
**Execution:** agent

The plugin example reads its root output and its module output as `types.Output` entities and prints their values, so it keeps working after lookups return entities. Its comments explain that outputs are entities like everything else and that `Outputs()` gives every value at once. What the example prints stays the same.

*Technical detail:* [context.md#task-read-outputs-as-entities-in-the-plugin-example](./context.md#task-read-outputs-as-entities-in-the-plugin-example)

**Acceptance criteria**:
- [ ] The plugin example looks up its outputs as entities and prints the same published values as before.
- [ ] The plugin example's tests pass.

### Milestone 2: Configurations reach into a module only through its outputs

**What changes**: A configuration that refers to anything inside a module other than one of its outputs fails validation, with an error naming the reference. This covers a module's resources, variables and nested modules, and `depends_on` entries too. Both `Validate` and `Apply` refuse it before anything is created. The rule holds at every depth: a parent reaches only its direct children's outputs. A module can now re-export a value from its own child module as one of its outputs, which failed validation before. So a root configuration can read a deeply nested value whenever each module in between re-exports it. Every bundled example still validates and runs.

**Validation point**: New parser tests prove that each reference shape is accepted or rejected as the rule says, and that rejected references are reported once with the reference named. The three-level nesting test shows that a root reference into the grandchild module is rejected. It also shows that a root reference to a re-exported output validates and, after `Apply`, resolves to the grandchild's value. All example tests pass.

#### - [ ] Task: Resolve re-exported outputs of nested modules
**Id:** 95adecca-887d-4b97-ad57-c5d7d85363f1
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Validation cannot currently resolve a reference, written inside a module, to that module's own child module's output. So a module cannot re-export a value from a module it uses. The resolver is changed to build module-scoped addresses the same way the dependency graph and the evaluator already do, so these references resolve at any depth.

*Technical detail:* [context.md#task-resolve-re-exported-outputs-of-nested-modules](./context.md#task-resolve-re-exported-outputs-of-nested-modules)

**Acceptance criteria**:
- [ ] A reference written inside a module to its child module's output resolves during validation.
- [ ] Every reference that resolved before still resolves to the same entity.

#### - [ ] Task: Reject references that cross a module boundary
**Id:** 27249cc8-0b54-4654-b714-f73b5d3d88ce
**Repo:** xclconfig
**Depends on:**
- 95adecca-887d-4b97-ad57-c5d7d85363f1 — Resolve re-exported outputs of nested modules
**Execution:** agent

Validation gains a rule: a reference may reach into a module only to one of that module's own outputs, and only into a direct child module. A reference to a module's resource, variable or nested module, or to anything in a grandchild module, is reported as a validation problem that names the reference. This applies to `depends_on` entries too. Referring to a child module itself, as in `depends_on = ["module.x"]`, is still allowed. A three-level test configuration proves that a value can be reached from the root only by re-exporting it through each module in between.

*Technical detail:* [context.md#task-reject-references-that-cross-a-module-boundary](./context.md#task-reject-references-that-cross-a-module-boundary)

**Acceptance criteria**:
- [ ] A configuration that refers to anything inside a module other than one of its outputs fails validation, with an error naming the reference, and the same reference is not also reported as undefined.
- [ ] The same configuration referring to the module's output instead validates.
- [ ] With root using module A and A using module B, a root reference to anything inside B, including B's outputs, fails validation.
- [ ] With the same nesting, a root reference to an output of A that re-exports B's output validates, and after apply it holds B's value.
- [ ] `Apply` refuses a configuration that crosses a module boundary before anything is created.
- [ ] Every bundled example and every existing test configuration still validates.

### Milestone 3: Documentation describes the module boundary and output entities

**What changes**: A developer reading the README, the modules guide or the documentation site learns three things: only a module's outputs can be referenced from its parent, how to re-export a nested module's value, and how to read an output as an entity with `Find[types.Output]` and `.Value`, with `Outputs()` returning all values at once. The changelog records both breaking changes. The project's knowledge entry on the user flow is brought into line with outputs as entities.

**Validation point**: The README and CHANGELOG content tests pass, and they fail if the new sections are removed. The website builds and type-checks. The `architecture/ux-flow.md` knowledge entry shows `Find[types.Output]` with `.Value` and the note on `Outputs()`.

#### - [ ] Task: Document the module boundary and output entities in the library
**Id:** f0bb4c15-f1a7-4170-a129-0cd59a72daba
**Repo:** xclconfig
**Depends on:**
- b989237b-cb9c-4d3b-9d86-5abefdfd2154 — Read outputs as entities in the plugin example
- 27249cc8-0b54-4654-b714-f73b5d3d88ce — Reject references that cross a module boundary
**Execution:** agent

The README's published-values section is rewritten to read outputs as entities with `Find[types.Output]` and `.Value`, and to list them with `FindByType`, keeping `Outputs()` for all values at once. The README's Modules section and the modules guide gain a short explanation of the boundary: only a module's outputs can be referenced from its parent. They include a worked re-export example through a nested module. The changelog gets an entry for this spec that lists both breaking changes. The README and changelog content tests are extended to guard the new text.

*Technical detail:* [context.md#task-document-the-module-boundary-and-output-entities-in-the-library](./context.md#task-document-the-module-boundary-and-output-entities-in-the-library)

**Acceptance criteria**:
- [ ] The README shows reading an output as an entity and its value, and no longer shows reading an output as a plain value.
- [ ] The README and the modules guide each state that only a module's outputs can be referenced from its parent, with a re-export example.
- [ ] The changelog has an entry for this work that lists the two breaking changes.
- [ ] The content tests fail if any of these sections or the changelog entry is removed.

#### - [ ] Task: Document the module boundary and output entities on the site
**Id:** 13e60e8c-75ee-4fce-8914-78029607e0ce
**Repo:** xcl-website
**Depends on:**
- f0bb4c15-f1a7-4170-a129-0cd59a72daba — Document the module boundary and output entities in the library
**Execution:** agent

The documentation site's Modules feature card says that only a module's outputs are reachable from its parent, and that a nested value is exposed by re-exporting it. The plugin example page shows a re-export example and the example program reading its outputs as entities, matching the library's example. The wording follows the library documentation, so the two do not drift.

*Technical detail:* [context.md#task-document-the-module-boundary-and-output-entities-on-the-site](./context.md#task-document-the-module-boundary-and-output-entities-on-the-site)

**Acceptance criteria**:
- [ ] The site describes the output-only module boundary with a re-export example.
- [ ] The site shows an output being read as an entity with its value.
- [ ] The site builds and type-checks.

#### - [ ] Task: Bring the user-flow knowledge entry into line with output entities
**Id:** 162163be-487d-4c55-a7e5-12f7ad9341b5
**Repo:** xclconfig
**Depends on:**
- ac841670-bdfa-4261-8268-c6b6bd23eb9e — Return outputs as entities from every lookup
**Execution:** agent

The knowledge entry `architecture/ux-flow.md` still says that looking up an output returns its bare value. The user decided it should be rewritten. Its published-values passage now shows `Find[types.Output]` with a `.Value` example, plus a note that `Outputs()` returns every value keyed by address. The edit goes through the `spek-knowledge` skill, never by editing the store file directly.

*Technical detail:* [context.md#task-bring-the-user-flow-knowledge-entry-into-line-with-output-entities](./context.md#task-bring-the-user-flow-knowledge-entry-into-line-with-output-entities)

**Acceptance criteria**:
- [ ] The user-flow knowledge entry shows an output read with `Find[types.Output]` and its `.Value`, and no longer shows `Find[string]` for an output.
- [ ] The entry notes that `Outputs()` returns every published value keyed by address.

## Open Questions

- **Does a nested re-export evaluate correctly at walk time once validation lets it through?** The dependency graph and the evaluation context already compose module keys correctly, so the expected answer is yes. Only the end-to-end Apply test in the boundary task can confirm it. If the value comes back null or unknown, the fix is in scope: make the context's `module` namespace resolve the child's output, keeping the boundary rule as it is. STOP and ask the user only if the fix would mean weakening the boundary or changing how modules are evaluated beyond that.

## Out of Scope

- **Jumppad's own `output` entity.** Jumppad, a downstream tool built on xcl, uses `output` to mean an environment-variable export and implements it as its own type. This plan does not touch it (spec Non-Goals).
- **Restricting Go lookups of module internals.** `Find`, `FindResource`, `FindByType`, `Entities` and `Decode` still reach entities inside modules by their full address. The boundary applies only to references written in configuration.
- **Changing how a module refers outward.** A reference made inside a module still falls back to the root scope as it does today. This plan restricts only references that reach *into* a module.
- **Tightening the evaluation context.** The `module` namespace built for evaluation is not changed. Validation is what enforces the boundary.
- **A matchable error sentinel for boundary violations.** Violations are reported as validation problems like every other reference problem. No `errors.Is` sentinel is added.
- **How `depends_on` relates to `Meta.Links`.** The user-depends-on spec in the same epic (`20261003153421-c283547c-user-depends-on`) owns this. It must keep user-written `depends_on` entries subject to this boundary check.
- **Rewording the validation-stage description in `architecture/ux-flow.md`.** Only the published-values passage is rewritten, with the wording the user approved.
