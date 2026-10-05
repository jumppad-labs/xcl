---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Plan: 20261003153421-bf87d907-references-as-written

<!-- Metadata -->
<!-- Created: 2026-10-05T10:11:19Z -->
<!-- Commit: d554c1d -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Configuration text that xcl produces shows every reference as the value it resolved to, so a reader cannot see which entity a field pointed at. This plan records, at parse time, the exact text a user wrote for every attribute holding a reference, and keeps it in each entity's saved bookkeeping. A new `xcl.ShowReferences()` option then writes those attributes as written, from a live entity or its saved data with byte-identical results, while resolved values stay the default and sensitive values keep the marker. Developers embedding xcl can show their users where values came from, and the README and a new configuration-text guide on the site explain how.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, positive and negative cases in separate functions, favour verbosity** — each reference shape (bare, template, nested block, sensitive bare, sensitive template) and each default-versus-option case is its own named test.
- **Generate test state with a real apply, not a hand-written state file** — the live-versus-saved equality tests apply a fixture and read the record state wrote; no state file is hand-written.
- **Code style: standard Go conventions, `any` over `interface{}`, descriptive names** — applies to the new option, the recorder in the parser and the replacement walk in `encode.go`.
- **NEVER modify dependency packages; `internal/xcl` is an in-repo MPL copy** — the reason the replacement is done in the root wrapper on the `hclwrite` tree and the copied `gohcl` encoder is not changed.
- **Shared public types live in `types`** (`architecture/shared-public-types-live-in-types.md`) — the recorded references are a field of `types.Meta`, which the parser writes and the root package reads.
- **Shared errors live in the `errors` package** — considered and not needed: the option adds no failure mode, and a recorded text that cannot be re-lexed falls back to the resolved value rather than erroring.

## Architecture & Design Decisions

All Go work lands in the `xclconfig` repo: the parser (`internal/parser/parser.go`), the shared bookkeeping type (`types/resource.go`) and the configuration-text encoder (`encode.go`), with the README, CHANGELOG and their content tests. The documentation site's new configuration-text guide and its navigation entry land in `xcl-website`. This plan lands after `20261003134528-327e0657-references-and-secrets`, whose sensitive marker, `RevealSensitive()` option and wire-encoded state it relies on.

**The text a user wrote is captured at parse time and kept in the entity's bookkeeping.** The source text of an expression exists only while a file is being parsed (`gotchas/expression-text-lost-at-parse.md`), so the parser passes the file's bytes down from `parseResourcesInFile` to `parseResource` and `parseModule`. While it already walks every attribute and nested block to find links (`internal/parser/parser.go:1130-1235`), it records, for each attribute that holds at least one reference by the same rule that fills `Meta.Links` (`processExpr` with `isReferenceRoot`), the exact bytes written after the `=`. Each is keyed by the attribute's path, with nested blocks numbered by their position among blocks of the same type: `location`, `network[1].name`. The map is a new `types.Meta.References` field, serialised as `references` and left out when empty. It carries only a `json` tag, so it never reaches cty or the configuration (`gotchas/xcl-tags-gate-what-reaches-cty.md`). Because `Meta` is already saved in state, already restored by `replaceValues`, ignored by change detection (`plugins/changed.go:12`) and passed to plugins as the real Go type, the live entity, its saved record and event data at `EventDataProcessed` all carry the same map with no new code on those paths. This is the spec's technical direction: the stored format gains one optional field. State written before this change loads unchanged and simply has no references to show.

**Showing references is one option on the encoder, applied in the root wrapper after the resolved text is built.** `xcl.ShowReferences()` sits beside `IncludeComputed()` and `RevealSensitive()`. Without it nothing changes, so resolved values remain the default. With it, `encodeEntity` (`encode.go:116`) first writes the resolved text exactly as today, then walks the entity's `References`. It finds each attribute in the `hclwrite` body by its path and replaces that attribute's tokens with the recorded text, re-lexed through `hclwrite` so the formatter lays it out. Only an attribute the resolved text already writes is replaced. A referenced field that is left out, such as `disabled` when false, stays out, so the option never adds a line the resolved text would not have. Both `EncodeEntity` and `EncodeSavedEntity` already share `encodeEntity`, so their text is byte-identical with and without the option. The work sits in the root wrapper, beside `trimBookkeeping`, and not in the copied `gohcl` encoder. That keeps xcl-specific shaping out of the MPL-licensed fork (`conventions/never-modify-dependencies.md`) and leaves `internal/xcl` and its `UPSTREAM.md` untouched.

**Sensitive values keep the marker.** A field whose resolved text shows the sensitive marker `"(sensitive)"` is replaced with its reference only when the expression is a single bare reference, such as `password = variable.db_password`. An address is not a value, so it discloses nothing. Any other expression, such as a template that may embed a literal, keeps the marker. When `RevealSensitive()` is also given, references are shown for every field, since the caller has asked for real values. This keeps the project-wide rule that configuration text writes sensitive values as the marker. The text remains for reading and is not meant to be reprocessed (spec Non-Goals). Rejected directions are recorded with evidence in `research.md#alternatives-considered-and-rejected`: rebuilding from `Meta.Links`, keeping the parsed body, a hook inside `gohcl`, a field outside `Meta`, and always masking sensitive references.

## Component Breakdown

- **Entity bookkeeping (`types.Meta`, changed)**: gains the record of references as written, a map from each referenced attribute's path to the text the user wrote after its `=`. It owns nothing else. Because it is part of `Meta`, it travels everywhere `Meta` already does (state, provider calls, event data, saved-entity decoding) and is ignored by change detection and by the configuration text itself.
- **Reference recorder (new, inside the parser's link discovery)**: while the parser walks each block's attributes and nested blocks to find links, it also writes the source text of every attribute that holds a reference into the entity's bookkeeping, keyed by its path. It uses the same rule as the links to decide what is a reference, and it needs the file's bytes, which the file parser now hands down to the block parsers. It runs for every parsed entity, at any module depth, and records the text exactly as written, never re-scoped to the module.
- **Configuration-text encoder (changed, root package)**: gains the `ShowReferences()` option. After writing the resolved text and trimming bookkeeping, and only when the option is given, it replaces each recorded attribute's value with the recorded text. It applies the sensitive rule: a marker-holding attribute takes a bare reference only, unless real values were asked for. Both public entry points share this one path, so live and saved text stay identical. The default output is unchanged.
- **Library documentation (changed)**: the README's configuration-text section describes the option with an example, and its "for reading" note says references are shown as resolved values unless asked for. The changelog entry and the README/CHANGELOG content tests guard the new text.
- **Documentation site (new page, `xcl-website`)**: a configuration-text guide, linked from the navigation, covers converting an entity and its saved data, provider-filled values, the sensitive marker and showing references as written, with an example. Later specs in the epic extend the same page.

## Data Structures & Interfaces

**`types.Meta.References` (new field, public).** The text the user wrote for each attribute that holds a reference, keyed by the attribute's path. A top-level attribute is keyed by its configuration name. An attribute inside a nested block is keyed by the block's type and its position among blocks of that type, then the name, as in `network[1].name`. The value is everything written after the `=`, as written: `resource.b.one.y`, `"${resource.b.one.y}-x"`, or a multi-line object. It carries only a `json` tag, so it never reaches cty or configuration text. It is omitted from JSON when empty.

```go
type Meta struct {
    // ...existing fields...

    // References holds, for each attribute whose value refers to another
    // entity, the expression exactly as the user wrote it, keyed by the
    // attribute's path, i.e. "location" or "network[1].name"
    // this is an internal property that can not be set with hcl
    References map[string]string `json:"references,omitempty"`
}
```

**Saved record format (changed, additive).** Every saved entity record, and event data at both levels, gains `meta.references` when the entity holds a reference. Records written before this change have no such key and load as before. This is the only stored-format change, and state from earlier versions still loads (spec Non-Goals do not require it to, but it does).

```json
{ "meta": { "id": "resource.app.one", "references": { "x": "resource.b.one.y" }, ... }, "x": "resolved" }
```

**`xcl.ShowReferences() EncodeOption` (new, public).** It asks `EncodeEntity` and `EncodeSavedEntity` to write each referenced attribute as the user wrote it instead of its resolved value. It combines freely with `IncludeComputed()` and `RevealSensitive()`.

```go
func ShowReferences() EncodeOption

text, err := xcl.EncodeEntity(app, xcl.ShowReferences())
text, err := xcl.EncodeSavedEntity(registry, record, xcl.ShowReferences())
```

```hcl
resource "app" "one" {
  x = resource.b.one.y
}
```

**Unchanged contracts.** `EncodeEntity` and `EncodeSavedEntity` keep their signatures, errors and default output. `Meta.Links`, `DependsOn` and the plugin wire contract are unchanged apart from the extra optional `meta` key.

## Implementation Detail

**New pattern: recording what was written, not only what it meant.** Until now, parsing kept only the resolved meaning of an expression, the addresses in `Meta.Links`. A reader will now find a second, parallel record built in the same walk: the parser's link discovery records each reference-holding attribute's source text against its path as it visits it. The two records come from one walk and one rule for what a reference is, so they cannot disagree about which attributes hold references. The block parsers take the file's bytes as a new argument. That is the one plumbing change, and the only way to reach the source text.

**Existing patterns followed.**
- The encoder's options remain closed functional options on an unexported struct: `ShowReferences()` joins `IncludeComputed()` and `RevealSensitive()`.
- The root wrapper shapes the `hclwrite` tree after `gohcl` has written it, just as `trimBookkeeping` already removes `meta` and `depends_on`. The replacement step runs after the trim, so it never resurrects a trimmed attribute.
- Bookkeeping lives in `Meta` with a `json`-only tag, beside `Links`, `Parents` and `Status`.

**Code-shape change.** In `encode.go`, `encodeEntity` gains one step after `trimBookkeeping`, which runs only under the option. It reads the entity's `Meta.References` in sorted path order, so the output is deterministic. For each path it walks the body to the attribute: it descends into the n-th block of each named type, then takes the attribute by name. If the attribute is missing, it skips the path. It re-lexes the recorded text into tokens and swaps them in. The sensitive check reads the attribute's current tokens for the marker, and asks whether the recorded text parses to a single bare reference. In the parser, `getUniqueResourceLinks` and its helpers carry a path prefix and the file bytes, and write into `Meta.References` as they go. The link logic itself is not changed.

**Public surface UX.** A developer who wants to see where values came from adds `xcl.ShowReferences()` to the call they already make, live or saved, and gets the same text from either. Nothing else in their code changes, and without the option they get exactly what they got before. A configuration author sees no difference at all, because what they write is what is shown. A secret held in a sensitive field still shows as `(sensitive)`, unless its value came from a single bare reference, in which case that address is shown.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **`20261003134528-327e0657-references-and-secrets` (planned; must land first).** Provides `types.SensitiveMarker`, `xcl.RevealSensitive()`, the `ReplaceMarked` handling in the encoder and wire-encoded state at the two save sites. This plan's sensitive rule reads the marker that spec writes, and its option sits beside `RevealSensitive()`. No changes to that work.
- **`20261003153421-6ec0eab3-module-boundary-and-output-entities` (planned; lands before this one).** Outputs become `types.Output` and the module boundary is judged on `Meta.Links` as written. This plan leaves `Meta.Links` and outputs untouched. Outputs, variables and modules remain unencodable, so the recorded references on them are never written as text.
- **`20261003153421-c283547c-user-depends-on` (later spec in the epic).** It owns `depends_on` in configuration text and extends the configuration-text guide this plan creates. `depends_on` stays trimmed here and is never recorded as a reference.
- **`internal/parser` link discovery.** Hosts the reference recorder. It gains the file bytes and a path prefix; the link rule is unchanged.
- **`types` package (`Meta`).** Gains the `References` field. No other change.
- **`internal/xcl/hclwrite`, `hclsyntax` (in-repo MPL copy).** Used as is to find attributes and blocks, to re-lex recorded text and to recognise a bare reference. No changes, so `internal/xcl/UPSTREAM.md` is untouched.
- **`internal/savedentity`, `state`, `plugins`.** Unchanged. They carry the new `Meta` field through their existing JSON paths, and default change detection already ignores `meta`.
- **`xcl-website` repo.** Hosts the new configuration-text guide and its navigation entry. Snippets are copied text with no code dependency, so it lands after the library docs. Its gate is its own build and `astro check` after `npm ci`.
- **External libraries: none added.** Tests use testify `require`, already a dependency.

## Testing Approach

All tests follow the project's conventions. They use testify `require`, there are no table-driven tests, each accepted case and each rejected or default case has its own test function, and every test that needs state gets it from a real `Apply` against a fixture, never from a hand-written state file.

**Unit tests (parser): what is recorded.** Small fixtures are parsed, and the tests assert on each entity's recorded references. A bare reference is recorded as written. So are a template mixing a reference with literal text, a function call taking a reference, a multi-line object holding a reference, and a parenthesised expression. An attribute inside a repeated nested block is keyed by its block position. A reference written inside a module is recorded exactly as written, not re-scoped. A literal attribute, or one using only a function with literal arguments, is not recorded, and an entity with no reference has no references at all.

**Encoder tests (root package): the spec's acceptance criteria.** These run on an applied fixture holding a field written as a reference to another entity (`x = resource.b.one.y`), a template, a nested-block reference and a sensitive field.
- With `ShowReferences()`, the text contains each referenced field exactly as written and not the resolved value. Literal fields and nested-block references are covered too.
- Without the option, the same entity's text shows the resolved values, byte-identical to what the encoder produced before this change. The existing encoder tests, which already cover references in the encode fixture, must pass unchanged.
- After apply, the text from the live entity and from its saved record are byte-identical, both with and without the option, in separate tests.
- With `IncludeComputed()` combined, references are still shown and computed values still carry their comment.
- Repeated calls with the option are byte-identical, and the output is stable under the formatter.

**Sensitive interplay.** A sensitive field written as a bare reference shows the reference with `ShowReferences()`. A sensitive field written as a template keeps `"(sensitive)"` with `ShowReferences()`, and shows its template with `ShowReferences()` and `RevealSensitive()` together. Each case is its own test, and no test secret appears in any text produced without the reveal option.

**Regression.** The existing parser, state, plugin, event, encode and query tests must pass unchanged. That proves the new bookkeeping field changes nothing for change detection, provider calls or default output. One test confirms that state saved without `meta.references`, as an earlier version wrote it, still loads and encodes with resolved values.

**Documentation.** README and CHANGELOG content tests are extended so they fail if the README stops documenting `xcl.ShowReferences()` with an example, if the "for reading" note stops mentioning the option, or if the changelog entry for this spec is removed. The site has no content tests. Its build and `astro check` are the automated gate.

**Success metrics.** The spec defines none, so there are none to verify.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: read the new configuration-text guide on the `xcl-website` site in a browser, check it is reachable from the navigation, and check that it describes requesting references with an example showing a reference as the user wrote it.

**Deliberate gaps.** External plugin round-trips of `meta.references` are not tested separately. The field travels in `Meta`, which the existing plugin tests already prove survives provider calls in both directions, and the live-versus-saved test runs through a plugin-provided type. Reprocessing the text is a spec non-goal, so no test parses the output back.

## Milestones & Tasks

### Milestone 1: Configuration text can show references as the user wrote them

**What changes**: A developer converting an entity to configuration text can pass `xcl.ShowReferences()` and see each field that referred to another entity written exactly as the user wrote it, such as `x = resource.b.one.y` or a template mixing a reference with text, instead of the value it resolved to. This works the same from a live entity and from its saved data, and the two give byte-identical text. Without the option, the text is exactly what it was before. A secret held in a sensitive field still shows as `(sensitive)`, unless it came from a single bare reference, in which case that address is shown. Saved state now records, for each entity, the references the user wrote. State written by an earlier version still loads, and simply shows resolved values.

**Validation point**: The full test suite passes unchanged. New parser tests prove what is recorded for each expression shape. New encoder tests prove three things: references are shown with the option and resolved values without it; live and saved text are byte-identical with and without it; and sensitive fields follow the marker rule.

#### - [x] Task: Record references as written while parsing
**Id:** 6a8abd6c-73c5-4bb3-92f5-b0bb1ae98fb9
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

While the parser finds the links between entities, it also records the exact text the user wrote for every attribute that refers to another entity. The text is keyed by the attribute's path, including attributes inside repeated nested blocks. The record lives in each entity's bookkeeping, so it is saved in state, carried to plugins and events, and read back from saved data without any change to those paths. Nothing visible changes yet: configuration text, change detection and validation behave exactly as before.

*Technical detail:* [context.md#task-record-references-as-written-while-parsing](./context.md#task-record-references-as-written-while-parsing)

**Acceptance criteria**:
- [x] After parsing, an entity with a field written as a reference records that field's text exactly as written, including templates, function calls, multi-line values and references inside nested blocks.
- [x] Fields with no reference are not recorded, and an entity without references records nothing.
- [x] A reference written inside a module is recorded as written, not rewritten to the module's scope.
- [x] After apply, the saved record of the entity holds the same references as the live entity.
- [x] Every existing test passes unchanged, and state written before this change still loads.

#### - [x] Task: Show references in configuration text on request
**Id:** 9730d38c-61b6-42d2-86b8-add18fd3e98e
**Repo:** xclconfig
**Depends on:**
- 6a8abd6c-73c5-4bb3-92f5-b0bb1ae98fb9 — Record references as written while parsing
**Execution:** agent

A new option, `xcl.ShowReferences()`, makes configuration text show each referenced field as the user wrote it instead of its resolved value. It works the same for a live entity and for its saved data. Without the option the text is unchanged. A sensitive field still shows the marker unless its value came from a single bare reference, or real values were also asked for.

*Technical detail:* [context.md#task-show-references-in-configuration-text-on-request](./context.md#task-show-references-in-configuration-text-on-request)

**Acceptance criteria**:
- [x] With the option, an entity whose field is written as `x = resource.b.one.y` produces text containing `x = resource.b.one.y`, not the resolved value.
- [x] Without the option, the same entity's text shows the resolved value, exactly as before this change.
- [x] After apply, the text from the live entity and from its saved data are byte-identical, both with and without the option.
- [x] A sensitive field written as a bare reference shows the reference, while one written any other way shows `(sensitive)` unless real values are also requested.
- [x] The text is the same on every call and works together with the option for provider-filled values.

### Milestone 2: The documentation explains how to show references

**What changes**: A developer reading the README or the documentation site learns how to ask for references in configuration text, and sees an example of a reference written as the user wrote it. The README's note that text shows resolved values now says so only for the default. The site gains a configuration-text guide, reachable from the navigation, that brings converting entities and saved data, provider-filled values, the sensitive marker and showing references together on one page. The changelog records the new option and the new field in saved state.

**Validation point**: The README and CHANGELOG content tests pass, and they fail if the new text is removed. The site builds and type-checks, and the new guide is reachable from the navigation.

#### - [x] Task: Document showing references in the library
**Id:** dc1e3875-837b-4e20-b8d8-461596e8839d
**Repo:** xclconfig
**Depends on:**
- 9730d38c-61b6-42d2-86b8-add18fd3e98e — Show references in configuration text on request
**Execution:** agent

The README's configuration-text section explains how to ask for references and shows an example of a reference written as the user wrote it. Its note that text shows resolved values now describes that as the default. The changelog gains an entry for this spec describing the option and the new field in saved state. The README and changelog content tests guard the new text.

*Technical detail:* [context.md#task-document-showing-references-in-the-library](./context.md#task-document-showing-references-in-the-library)

**Acceptance criteria**:
- [x] The README describes requesting references, with an example showing a reference exactly as written.
- [x] The README no longer says, without qualification, that references always come out as resolved values.
- [x] The changelog has an entry for this work naming the option and saying there are no breaking changes.
- [x] The content tests fail if the new README text or the changelog entry is removed.

#### - [x] Task: Add the configuration-text guide to the site
**Id:** 762508e1-3e8b-46dc-a306-aac8e9559d0e
**Repo:** xcl-website
**Depends on:**
- dc1e3875-837b-4e20-b8d8-461596e8839d — Document showing references in the library
**Execution:** agent

The documentation site gains a configuration-text guide, linked from the navigation's Guides menu. It explains turning an entity and its saved data back into configuration text, showing provider-filled values, how sensitive values appear, and how to show references as the user wrote them, with an example. The wording follows the library documentation so the two do not drift.

*Technical detail:* [context.md#task-add-the-configuration-text-guide-to-the-site](./context.md#task-add-the-configuration-text-guide-to-the-site)

**Acceptance criteria**:
- [x] The site has a configuration-text guide reachable from the navigation.
- [x] The guide describes requesting references, with an example showing a reference written as the user wrote it, next to the default resolved form.
- [x] The site builds and type-checks.

## Open Questions

- **Does the sensitive marker always show in an attribute's written tokens when its value is sensitive?** This depends on how the secrets plan's `ReplaceMarked` handling writes a marked value nested inside an object or list attribute, which only exists once that plan lands. The sensitive rule in this plan reads the attribute's tokens for the quoted marker. If a sensitive part can be written without the marker appearing in the attribute's tokens, switch the check to the entity's Go field, a `types.SensitiveValue` at or below the attribute's path, keeping the same rule. STOP and ask the user only if neither check can tell a sensitive attribute apart, because then the rule cannot be kept without always masking references.

## Out of Scope

- **Making configuration text reprocessable.** The text remains for display and reading, as settled in spec `20260922132517-hcl-encoding-helpers` (spec Non-Goals). Nothing checks that text with references parses back.
- **Backwards compatibility for references in older state.** Records saved before this change carry no references, so they show resolved values even with the option, until the configuration is applied again. They still load (spec Non-Goals do not require it).
- **`depends_on` in configuration text.** It stays trimmed, and it is never recorded as a reference. The user-depends-on spec, `20261003153421-c283547c-user-depends-on`, owns showing a written dependency list.
- **Showing references for variables, outputs and modules.** These are not encodable as configuration text. Their references are recorded like any entity's, but are never written.
- **Restoring comments or original layout.** Only the expression after `=` is reproduced, and the formatter lays it out. Comments outside it and the original spacing are not kept.
- **Showing references in the resource printer or event data.** Only configuration text gains the option. Event data and state carry the recorded map as part of `meta`, but nothing displays it.

## Changelog

### 2026-10-05 — Task: Record references as written while parsing

**What was done**: `types.Meta` gained `References map[string]string` (json `references,omitempty`, no `xcl` tag). The parser now passes each file's bytes from `parseResourcesInFile` through `parseResource`/`parseModule` to `getUniqueResourceLinks`, and the link walk records, via a new `recordWrittenText` helper, the text after `=` for every attribute `processExpr` finds a reference in, keyed by `name` or `type[n].name` (recursively for deeper blocks). The map is set on `Meta` only when non-empty.

**Deviations**: Adding a field to `Meta` changed the schema snapshot in `internal/schema/test_fixtures/embedded.go`; both `Meta` occurrences gained the `References` entry. That existing test expectation had to change, which the plan did not anticipate. Tests for the module case use a dedicated module fixture rather than the shared `single` module.

**Files changed**:
- `xclconfig: types/resource.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/exp.go`
- `xclconfig: internal/schema/test_fixtures/embedded.go`
- `xclconfig: internal/parser/references_written_test.go`
- `xclconfig: internal/test_fixtures/config/references/main.xcl`
- `xclconfig: internal/test_fixtures/config/references/module/main.xcl`
- `xclconfig: config_test.go`

**Discoveries**: `processExpr` only recognises templates, function calls, scope traversals and object constructors; tuples, conditionals and unary expressions (e.g. `[resource.x.y]`, `!variable.enabled`) produce no link and are therefore not recorded either. Any new `Meta` field changes the plugin schema snapshot in `internal/schema/test_fixtures/embedded.go`.

### 2026-10-05 — Task: Show references in configuration text on request

**What was done**: Added `xcl.ShowReferences()` beside `IncludeComputed()` and `RevealSensitive()`. Under the option, `encodeEntity` runs `showReferences` after `trimBookkeeping`: it walks `Meta.References` in sorted path order, finds each attribute in the `hclwrite` body (descending into the n-th nested block of each type), skips paths the text does not hold, and replaces the attribute's tokens with the recorded text re-lexed through `hclwrite.ParseConfig`. A marker-holding attribute is replaced only for a single bare reference unless `RevealSensitive()` is also given.

**Deviations**: The plan's open question (does the marker always show in the attribute's tokens?) was settled by checking for the quoted marker anywhere in the attribute's tokens, which covers markers nested inside objects and lists; the fallback to the Go field was not needed. `TestStateWithoutReferencesStillLoads` was extended to also encode with `ShowReferences()`. A few extra tests were added beyond the plan's list (nested-block default, template formatter stability, saved-entity sensitive template).

**Files changed**:
- `xclconfig: encode.go`
- `xclconfig: encode_references_test.go`
- `xclconfig: config_test.go`

**Discoveries**: `hclwrite.Body.SetAttributeRaw` replaces only the expression, so a line comment such as the computed-value comment survives the replacement. `disabled = variable.off` is recorded as a reference, yet stays trimmed under the option because replacement runs after `trimBookkeeping` and only touches attributes still present.

### 2026-10-05 — Task: Document showing references in the library

**What was done**: The README's "Converting to configuration text" section gained a "Showing references as written" paragraph with a configuration, the `xcl.ShowReferences()` call, the text it produces and the default resolved form, plus the saved-data, `IncludeComputed`, sensitive and older-state notes. The "for reading" note now says references are resolved by default unless `ShowReferences` is asked for. `CHANGELOG.md` has a new top entry for this spec ending "There are no breaking changes.", and `docs/state.md` notes that saved records carry `meta.references`.

**Deviations**: Content tests go beyond the plan's two: they also guard the reworded "for reading" note, the `docs/state.md` sentence and the absence of a `**Breaking:**` list in this entry.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: CHANGELOG.md`
- `xclconfig: docs/state.md`
- `xclconfig: readme_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Add the configuration-text guide to the site

**What was done**: Added `src/pages/configuration-text.mdx`, a guide in the existing Hero/Prose/CtaBanner shape. It covers converting an entity, converting saved data, provider-filled values, sensitive values, showing references as written (with the configuration, the `ShowReferences()` call, its output and the default resolved form), and the "for reading, not reprocessing" note. Its wording mirrors the README. It is linked as "Configuration text" in the navigation's Guides menu.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/components/Nav.astro`

**Discoveries**: None. `npm ci`, `astro check` (0 errors, 0 warnings) and `npm run build` pass, and the built home page links the new guide.
