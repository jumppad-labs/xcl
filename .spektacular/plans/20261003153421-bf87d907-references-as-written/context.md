---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Context: 20261003153421-bf87d907-references-as-written

## Current State Analysis

- `encode.go:112-160` — `encodeEntity` is the single path behind `EncodeEntity` and `EncodeSavedEntity`; it writes resolved values through `gohcl.EncodeBody`, then `trimBookkeeping` removes `meta`, `depends_on` and a false `disabled`.
- `internal/parser/parser.go:556-655` — `parseResourcesInFile` holds `f.Bytes`, the only place the source text exists; the block parsers do not receive it (`gotchas/expression-text-lost-at-parse.md`).
- `internal/parser/parser.go:1084-1235` — link discovery walks every attribute and nested block, counting nested blocks per type, and keeps only addresses in `Meta.Links`.
- `types/resource.go:3-60` — `Meta` holds `json`-only bookkeeping (`Links`, `Parents`, `Status`) that state saves and cty never sees.
- `plugins/changed.go:12` — default change detection ignores `meta`.
- `internal/parser/lifecycle.go:317-338,414` — `replaceValues` restores the configured `Meta`; provider results are unmarshalled over the entity.
- `README.md:653-720` — documents configuration text and says references come out as resolved values.
- `xcl-website` has no configuration-text page; Guides menu in `xcl-website:src/components/Nav.astro:18-24`.
- After `20261003134528-327e0657-references-and-secrets` lands: `RevealSensitive()`, `types.SensitiveMarker`, `ReplaceMarked` in the encoder, wire-encoded state.

## Per-Task Technical Notes

### Task: Record references as written while parsing

Requirement → repo: "References survive saving" (recording half) → `xclconfig`.

**File changes**:
- `types/resource.go:44-60` — add `References map[string]string \`json:"references,omitempty"\`` to `Meta`, after `Links`, with a comment in the existing style ("this is an internal property that can not be set with hcl"). No `xcl` tag, so it never reaches cty (`gotchas/xcl-tags-gate-what-reaches-cty.md`).
- `internal/parser/parser.go:556-655` — `parseResourcesInFile` passes `f.Bytes` to `parseResource` (calls at `:612`, `:637`) and `parseModule` (`:607`).
- `internal/parser/parser.go:703` — `parseResource(file, b, moduleName)` gains `src []byte`, passed on to `getUniqueResourceLinks` (`:863`).
- `internal/parser/parser.go:913,981` — `parseModule` gains `src []byte`, passed on to `getUniqueResourceLinks`.
- `internal/parser/parser.go:1084-1127` — `getUniqueResourceLinks(resource, b, src)` passes `src` to `getDependentResources`.
- `internal/parser/parser.go:1130-1208` — `getDependentResources`: for each attribute, when `processExpr` returns at least one reference, record the attribute under its name. For each nested block, the existing `blockIndex` counter gives `<type>[<n>]`, so pass the prefix `<type>[<n>].` to `getDependentResourcesFromBlock`. Note: the current code increments `blockIndex` before use, so the first block is `0`; keep that. Skip `depends_on` (string literals, never references, but skip explicitly for clarity).
- `internal/parser/parser.go:1211-1235` — `getDependentResourcesFromBlock(b, prefix, src)` records attributes as `prefix + name`, and recurses with a per-type counter for its own nested blocks. Return the recorded map alongside the references, or write it through a small helper on the entity's `Meta`, whichever keeps the cycle check at `:1170-1205` untouched.
- New helper (in `internal/parser/parser.go` or `internal/parser/exp.go`): `writtenText(attr *hclsyntax.Attribute, src []byte) string` returns `strings.TrimSpace(string(src[attr.EqualsRange.End.Byte:attr.SrcRange.End.Byte]))`. Guard against `src == nil` or out-of-range offsets by recording nothing.
- New fixture `internal/test_fixtures/config/references/main.xcl` — registered types: a bare reference (`location = variable.region`), a template (`location = "${variable.region}-a"`), a function call with a reference, a multi-line object holding a reference if a registered type has a map/object field (otherwise use the plugin `container` type's attributes), a repeated nested block with references (`network { name = resource.network.main.meta.name }` ×2), and a literal-only entity. Plus a module whose resource writes a reference, to check it is not re-scoped.
- `internal/parser/parse_test.go` (or a new `internal/parser/references_written_test.go`) — one test per shape: `TestParseRecordsBareReferenceAsWritten`, `TestParseRecordsTemplateAsWritten`, `TestParseRecordsFunctionCallAsWritten`, `TestParseRecordsMultiLineValueAsWritten`, `TestParseRecordsNestedBlockReferenceByPosition`, `TestParseRecordsModuleReferenceUnscoped`, `TestParseRecordsNothingForLiteralFields`, `TestParseRecordsNothingForEntityWithoutReferences`.
- `state/file_state_store_apply_test.go` or `config_test.go` — `TestApplySavesReferencesInState`: real apply, the saved record's `meta.references` equals the live entity's.
- `config_test.go` — `TestStateWithoutReferencesStillLoads`: run a real apply, then remove `meta.references` from the state file it wrote (this test is about the state format, the exception in `conventions/test-state-from-real-apply.md`), reload with a second `Config`, and check that `EncodeSavedEntity` with `ShowReferences()` succeeds and shows resolved values.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Single agent, sequential: thread `src` through, add recording, then fixture and tests.

### Task: Show references in configuration text on request

Requirement → repo: "References can be shown as written", "Resolved values remain the default", "References survive saving" (text half) → `xclconfig`.

**File changes**:
- `encode.go:18-39` — add `showReferences bool` to `encodeOptions`; add `func ShowReferences() EncodeOption` beside `IncludeComputed()` and `RevealSensitive()` (from the secrets plan), with a doc comment: writes each field that referred to another entity as the user wrote it; the text is still for reading; a sensitive field shows its reference only when written as a single bare reference, unless `RevealSensitive` is also given. Update the `EncodeOption` comment ("Construct one with IncludeComputed, RevealSensitive or ShowReferences").
- `encode.go:41-60` — `EncodeEntity` doc: "Values are written as the literals they resolved to … unless ShowReferences is given".
- `encode.go:112-160` — in `encodeEntity`, after `trimBookkeeping(entity, block.Body())` (`:155`), call `showReferences(meta, block.Body(), opts)` when `opts.showReferences`.
- `encode.go` (new func) — `showReferences(meta *types.Meta, body *hclwrite.Body, opts encodeOptions)`: iterate `meta.References` keys sorted (`slices.Sorted(maps.Keys(...))`); for each path, split into segments (`name` or `type[n]`), descend through `body.Blocks()` filtered by `Type()` and index; take `GetAttribute(last)`; skip when nil. Parse the text with `hclwrite.ParseConfig([]byte("v = "+text+"\n"), "", hcl.InitialPos)`; skip on diagnostics; take `GetAttribute("v").Expr().BuildTokens(nil)`; `body.SetAttributeRaw(name, tokens)`. Sensitive rule: when `!opts.revealSensitive` and the attribute's current tokens (`attr.Expr().BuildTokens(nil).Bytes()`) contain `strconv.Quote(types.SensitiveMarker)`, replace only if `hclsyntax.ParseExpression([]byte(text), "", hcl.InitialPos)` yields `*hclsyntax.ScopeTraversalExpr`.
- `encode.go:160-171` — the formatter run by `file.Bytes()` lays the raw tokens out; no change.
- `encode_test.go` — new tests on an applied references fixture (reuse the parser task's fixture under `internal/test_fixtures/config/references/`, or extend a copy of the encode registry helper): `TestEncodeEntityShowsReferencesWhenAsked`, `TestEncodeEntityShowsTemplateAsWritten`, `TestEncodeEntityShowsNestedBlockReferencesAsWritten`, `TestEncodeEntityShowsResolvedValuesByDefault`, `TestEncodeSavedEntityMatchesEncodeEntityWithReferences`, `TestEncodeSavedEntityMatchesEncodeEntityWithoutReferences`, `TestEncodeEntityShowsReferencesWithComputed`, `TestEncodeEntityWithReferencesIsDeterministic`, `TestEncodeEntityWithReferencesIsFormatterStable`, `TestEncodeEntityKeepsTrimmedDisabledWithReferences`.
- `encode_test.go` (sensitive) — needs a registered type with a `types.Sensitive[string]` field; add one to `internal/test_fixtures/registered/types.go` if the secrets plan did not: `TestEncodeEntityShowsBareReferenceForSensitiveField`, `TestEncodeEntityKeepsMarkerForSensitiveTemplate`, `TestEncodeEntityShowsSensitiveTemplateWhenRevealed`. Assert the test secret never appears without `RevealSensitive()`.
- Existing `encode_test.go:130-440` — must pass unchanged.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential: option and walk, then tests.

### Task: Document showing references in the library

Requirement → repo: "References output is documented" (library half) → `xclconfig`.

**File changes**:
- `README.md:653-720` — in "### Converting to configuration text", add a paragraph and example: `text, err := xcl.EncodeEntity(app, xcl.ShowReferences())` with an `hcl` block showing `x = resource.b.one.y` next to the default resolved form; mention it works the same for `EncodeSavedEntity`, combines with `IncludeComputed()`, and the sensitive rule. Reword `:714-718` so "References come out as the literal values they resolved to" applies by default, "unless you ask for `ShowReferences`". Note that state saved by an earlier version shows resolved values until applied again.
- `CHANGELOG.md:1` — new top entry `## 20261003153421-bf87d907-references-as-written`: the option, live and saved text identical, the `meta.references` field in saved state, the sensitive rule, and "There are no breaking changes." (no `**Breaking:**` list, since nothing breaks).
- `readme_test.go` — `TestReadmeDocumentsTheReferencesOption` (contains `xcl.ShowReferences()` and an example line `= resource.`), `TestChangelogRecordsTheReferencesOption` (the entry heading `## 20261003153421-bf87d907-references-as-written` and `xcl.ShowReferences()`).
- `docs/state.md:120-140` — one sentence: saved records carry `meta.references`, which `EncodeSavedEntity` uses under `ShowReferences`.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

### Task: Add the configuration-text guide to the site

Requirement → repo: "References output is documented" (site half) → `xcl-website`.

**File changes**:
- `xcl-website:src/pages/configuration-text.mdx` (new) — follow `xcl-website:src/pages/events.mdx` (Shell layout, `Hero`, `Prose`, `CtaBanner`). Sections: converting an entity (`EncodeEntity`), converting saved data (`EncodeSavedEntity`, with `EventDataProcessed`), provider-filled values (`IncludeComputed()`), sensitive values (`(sensitive)`, `RevealSensitive()`), showing references as written (`ShowReferences()`, with an example beside the default resolved text), and "for reading, not reprocessing". Wording follows the README section.
- `xcl-website:src/components/Nav.astro:19-23` — add `{ label: "Configuration text", href: "/configuration-text/" }` to the Guides children.
- Gate: `npm ci`, the site build and `astro check`.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

## Testing Strategy

- **Record references as written while parsing**: parser unit tests, one per expression shape and one per non-recorded case; one real-apply test that the saved record carries the same references; one state-format test that state without the key still loads.
- **Show references in configuration text on request**: encoder tests on an applied fixture covering the spec's acceptance criteria (shown with the option, resolved by default, live and saved byte-identical with and without), combination with `IncludeComputed()`, determinism and formatter stability, and the three sensitive cases. Existing encode tests unchanged.
- **Document showing references in the library**: README and CHANGELOG content tests in `readme_test.go`.
- **Add the configuration-text guide to the site**: site build and `astro check` after `npm ci`; manual browser review captured in the implementation test plan.
- Conventions: testify `require`, no table-driven tests, positive and negative cases in separate functions, state from a real apply.
- No success metrics in the spec.

## Project References

- Spec: `20261003153421-bf87d907-references-as-written`.
- Plans built on: `20261003134528-327e0657-references-and-secrets`, `20261003153421-6ec0eab3-module-boundary-and-output-entities`.
- Later spec extending this work: `20261003153421-c283547c-user-depends-on` (depends_on in text, extends the site guide).
- Knowledge: `gotchas/expression-text-lost-at-parse.md`, `gotchas/xcl-tags-gate-what-reaches-cty.md`, `gotchas/plugin-types-rebuilt-with-structof.md`, `learnings/state-save-and-load-points.md`, `architecture/shared-public-types-live-in-types.md`, `conventions/never-modify-dependencies.md`, `conventions/test-state-from-real-apply.md`, `conventions/testing-and-mocking.md`.
- Design documents: none.
- Repo roots: `xclconfig` at `/home/nicj/code/github.com/jumppad-labs/xcl`; `xcl-website` at `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

All four tasks run as single agents in sequence: the encoder task needs the recorder, and the docs follow the code.

## Migration Notes

No migration. Saved records gain an optional `meta.references` key. Records saved earlier load unchanged and show resolved values until the configuration is applied again.

## Performance Considerations

Parsing slices one byte range per reference-holding attribute; encoding re-lexes one short expression per recorded reference, only under the option. Both are negligible next to parsing and applying.
