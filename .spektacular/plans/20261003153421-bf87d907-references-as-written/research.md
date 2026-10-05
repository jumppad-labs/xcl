---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Research: 20261003153421-bf87d907-references-as-written

## Alternatives considered and rejected

### Rebuild expressions from `Meta.Links` at encode time

Derive the reference text from the addresses parsing already keeps.

**Rejected**: links hold only flattened addresses with no field and no surrounding text (`internal/parser/parser.go:1084-1127`, `gotchas/expression-text-lost-at-parse.md`). A template such as `"${resource.b.one.y}-x"` cannot be recovered.

### Keep the parsed `hclsyntax.Body` and read the expression from it when encoding

Hold on to the parsed body and slice the expression out when text is requested.

**Rejected**: the body exists only in the parser's working set (`internal/parser/parser.go:59`, `p.parsedResources.store`). It is not in state, so `EncodeSavedEntity` could never match `EncodeEntity`, which the spec requires byte for byte.

### Record the expression's own range (`attr.Expr.Range()`)

Take the source text from the expression node's range.

**Rejected**: some hclsyntax nodes' ranges do not cover everything written after `=` (for example wrapping parentheses). The bytes between `attr.EqualsRange.End` and `attr.SrcRange.End` are exactly what the author wrote, so that slice is used instead.

### A path-keyed raw-token hook inside the copied `gohcl` encoder

Add an `EncodeOptions` callback in `internal/xcl/gohcl/encode.go:294-333` that writes raw tokens for a path.

**Rejected**: it would keep attribute order for a referenced field whose resolved value is left out, but adds a second xcl-motivated extension to the MPL fork for something the root wrapper can do on the `hclwrite` tree it already owns, as `trimBookkeeping` does (`encode.go:169-186`). `gotchas/xcl-tags-gate-what-reaches-cty.md` records that shaping is the root wrapper's job.

### Store the references outside `Meta`

A new top-level `ResourceBase` field holding the map.

**Rejected**: an `xcl`-tagged field would reach cty and configuration; a `json`-only one works, but `Meta` is already the home of xcl's own bookkeeping (`types/resource.go:3-60`), is already trimmed from text, ignored by change detection (`plugins/changed.go:12`) and passed to plugins as the real Go type (`gotchas/plugin-types-rebuilt-with-structof.md`).

### Always keep the sensitive marker for a sensitive field

Never replace a marker-holding attribute, whatever the expression.

**Rejected**: safe, but hides the most useful reference of all (`password = variable.db_password`), and a bare reference holds no secret. Chosen instead: a sensitive field shows its reference only when the expression is a single bare reference, or when `RevealSensitive()` is also given.

## Chosen approach — evidence

- `internal/parser/parser.go:556-560` — `parseResourcesInFile` holds the parsed file `f`, whose `f.Bytes` is the only place the source text exists (`gotchas/expression-text-lost-at-parse.md`).
- `internal/parser/parser.go:612,637,703` — every non-module entity is parsed by `parseResource(file, b, module)`, called only from `parseResourcesInFile`; `parser.go:1071` recurses into module files through the same function, so every file's bytes are reachable.
- `internal/parser/parser.go:1130-1235` — `getDependentResources` and `getDependentResourcesFromBlock` already walk every attribute and nested block, already count nested blocks per type (`blockIndex`), and already call `processExpr(a.Expr, p.isReferenceRoot)` (`internal/parser/exp.go:26`), which decides what counts as a reference.
- `types/resource.go:44-60` — `Meta.Links`, `Parents`, `Status` are `json`-only fields: in state, invisible to cty and to configuration (`gotchas/xcl-tags-gate-what-reaches-cty.md`).
- `internal/parser/lifecycle.go:317-338,414` — `replaceValues` restores the configured `Meta`; a provider result is unmarshalled over the entity, which leaves a map field untouched when the result omits it.
- `plugins/changed.go:12` — `meta` is ignored by default change detection, so a new `Meta` field triggers no update.
- `internal/savedentity/savedentity.go:28-88` — saved records decode through plain JSON into the typed entity, so a new `Meta` field round-trips with no decoder change.
- `encode.go:116-171` — `encodeEntity` is the one path both public entry points share; post-processing the `hclwrite` body there gives identical text from both.
- `internal/xcl/hclwrite/ast_body.go:52-77,148` — `Body.Blocks()`, `GetAttribute` and `SetAttributeRaw` let the wrapper find an attribute by nested-block index and replace its tokens.
- `encode_test.go:33-127`, `internal/test_fixtures/config/encode/main.xcl` — the fixture already holds references (`location = variable.region`, `network { name = resource.network.main.meta.name }` twice), and the helpers reach both the live entity and the saved record.

## Files examined

- `encode.go:1-196` — public `EncodeEntity`/`EncodeSavedEntity`, `encodeOptions`, `IncludeComputed`, shared `encodeEntity`, `trimBookkeeping`.
- `internal/xcl/gohcl/encode.go:17-30,294-333,493-563` — `EncodeOptions`, attribute writing, nested blocks written one per slice element in order.
- `internal/xcl/hclwrite/ast_body.go:52-242` — body API for attributes and blocks.
- `internal/parser/parser.go:556-655` — file parse and dispatch.
- `internal/parser/parser.go:703-905` — `parseResource`, sets `Meta` and links.
- `internal/parser/parser.go:1081-1235` — link discovery over attributes and nested blocks.
- `internal/parser/exp.go:26,149` — `processExpr`, reference extraction.
- `internal/parser/lifecycle.go:160-425` — read, replaceValues, callProvider.
- `plugins/changed.go:1-60` — default change detection ignores `meta`.
- `internal/savedentity/savedentity.go` — saved record decoding.
- `types/resource.go`, `types/resource_helpers.go:74-110` — `Meta`, `AppendUniqueDependency`.
- `encode_test.go`, `internal/test_fixtures/config/encode/main.xcl` — encode test conventions and fixture.
- `readme_test.go` — README/CHANGELOG content tests.
- `README.md:653-720` — "Converting to configuration text" section, including "References come out as the literal values they resolved to".
- `CHANGELOG.md:1-30` — entry format.
- `docs/state.md:120-140` — `EncodeSavedEntity` from saved records.
- `xcl-website:src/components/Nav.astro:5-25` — nav groups; no configuration-text page exists.
- `xcl-website:src/pages/events.mdx` — page structure to follow (Hero, Prose).
- `xcl-website:src/pages/examples/configuration-only.mdx:255-265` — only existing mention of configuration text on the site.

## External references

- HCL `hclsyntax.Attribute` (`NameRange`, `EqualsRange`, `SrcRange`) — in-repo copy `internal/xcl/hclsyntax`; why: the equals-to-end slice is the text as written.
- HCL `hclwrite.ParseConfig` and `Expression.BuildTokens` — in-repo copy; why: re-lexes the recorded text into tokens for `SetAttributeRaw` so the formatter can lay it out.

## Prior plans / specs consulted

- `20261003134528-327e0657-references-and-secrets` (plan) — `types.Sensitive[T]`, `SensitiveMarker`, `RevealSensitive()` beside `IncludeComputed()`, `gohcl.EncodeOptions.ReplaceMarked`, `internal/wire` at the state save sites. Requires this spec to keep writing sensitive values as the marker. This plan lands after it.
- `20261003153421-6ec0eab3-module-boundary-and-output-entities` (plan) — outputs become `types.Output`; module boundary over `Meta.Links` as written; `FQRN.AppendParentModule`. No overlap beyond: references are recorded as written, never re-scoped, and `Meta.Links` is unchanged.
- `20261003153421-c283547c-user-depends-on` (spec) — owns `depends_on` in configuration text and also names the site's configuration-text guide. This plan creates that guide; that spec extends it.
- `20260922132517-hcl-encoding-helpers` (via CHANGELOG) — configuration text is for reading, not reprocessing.

## Open assumptions

- A nested block's position among blocks of the same type in the source equals its position in the decoded Go slice and in the text `gohcl` writes. Decoding fills slices in source order and the encoder writes one block per element in order (`internal/xcl/gohcl/encode.go:525-535`). If a test shows otherwise, STOP and ask.
- A provider result never deletes `meta.references`: `json.Unmarshal` leaves a map untouched when the key is absent. Holds for in-process and external plugins.
- The equals-to-end byte slice re-lexes as a valid single expression in `hclwrite`, including templates, heredocs and multi-line object constructors.

## Drafting assumptions

### Record references in Meta (discovery)
- **Decision**: each entity records, at parse time, the text written after `=` for every attribute that holds a reference, keyed by the attribute's path (`name`, `network[1].name`), in a new `types.Meta.References map[string]string` serialised as `references`, omitted when empty.
- **Rationale**: `Meta` is already xcl's bookkeeping, already in state, trimmed from text, ignored by change detection and carried to plugins as the real type.
- **Rejected**: a new `ResourceBase` field; keeping the parsed body; rebuilding from `Meta.Links`.

### Site configuration-text guide is created by this plan (discovery)
- **Decision**: the site has no configuration-text guide, so this plan adds one (`src/pages/configuration-text.mdx`, linked from the nav) covering converting an entity and saved data, provider-filled values, the sensitive marker and showing references. The user-depends-on spec extends it.
- **Rationale**: the spec's documentation requirement names the guide; it does not exist and nothing earlier in the epic creates it.
- **Rejected**: adding the section to an example page, which the spec does not name.

### Chosen direction: parse-time capture into Meta, root-wrapper replacement (architecture)
- **Decision**: capture the text after `=` for reference-holding attributes at parse time into `Meta.References`; `xcl.ShowReferences()` replaces those attributes' tokens in the `hclwrite` body inside `encodeEntity`, after `trimBookkeeping`.
- **Rationale**: follows the spec's technical direction; one shared path makes live and saved text identical; no change to the MPL fork.
- **Rejected**: a `gohcl` hook (fork change), keeping parsed bodies (not in state), rebuilding from links (lossy).

### Option name ShowReferences (architecture)
- **Decision**: the option is `xcl.ShowReferences() EncodeOption`.
- **Rationale**: reads as the spec's own wording ("references shown") and sits beside `IncludeComputed()` and `RevealSensitive()`.
- **Rejected**: `IncludeReferences()` (nothing is added, values are replaced), `WithReferences()` (the encoder options do not use the `With` prefix).

### Replace only attributes the resolved text writes (architecture)
- **Decision**: a referenced field that the resolved text leaves out (nil value, `disabled = false`) stays out with the option.
- **Rationale**: keeps the option a pure substitution and keeps `trimBookkeeping`'s rules intact.
- **Rejected**: appending missing attributes at the end of the block, which would reorder fields and contradict the disabled trimming rule.

### Sensitive fields show only a bare reference unless revealing (architecture)
- **Decision**: without `RevealSensitive()`, an attribute whose resolved text shows the marker is replaced only when its expression is a single bare reference; otherwise the marker stays. With `RevealSensitive()` every reference is shown.
- **Rationale**: keeps the project-wide rule that configuration text writes sensitive values as the marker, while showing the useful `password = variable.db_password`.
- **Rejected**: always keeping the marker (hides the most useful reference), always showing references (a template could embed a literal secret).

### Text as written is the bytes after the equals sign (architecture)
- **Decision**: record `src[attr.EqualsRange.End.Byte:attr.SrcRange.End.Byte]`, trimmed of surrounding whitespace.
- **Rationale**: exactly what the user wrote, including parentheses and templates, independent of how hclsyntax ranges each node.
- **Rejected**: `attr.Expr.Range()`, which may not cover everything written.

### Unparseable recorded text falls back to the resolved value (architecture)
- **Decision**: if recorded text cannot be re-lexed as an expression, the attribute keeps its resolved value; no error.
- **Rationale**: text the parser accepted always re-lexes, so this is a guard, not a feature; failing the whole conversion would be worse for a display-only function.
- **Rejected**: returning `ErrNotEncodable`.

### Conventions selected (architecture)
- **Decision**: testing, real-apply state, code style, never-modify-dependencies, shared-public-types; shared-errors noted as not needed; database, project-structure, patterns, logging conventions dropped as not bearing on this work.
- **Rationale**: only these shape choices in this plan.
- **Rejected**: listing every convention.

### Deterministic replacement order (implementation detail)
- **Decision**: replacements are applied in sorted path order.
- **Rationale**: map iteration is random; the existing test `TestEncodeEntityIsDeterministic` requires stable output.
- **Rejected**: source order (would need an ordered structure in `Meta` for no visible gain, since each replacement is in place).

### Task split (tasks)
- **Decision**: four tasks: recording (parser), the option (encoder), library docs, site guide.
- **Rationale**: recording is independently verifiable through state; the site is a separate repo.
- **Rejected**: one combined Go task (too large to review), a separate fixture task.

### No breaking changes in the changelog (tasks)
- **Decision**: the changelog entry says "There are no breaking changes" and has no `**Breaking:**` list.
- **Rationale**: the option is additive, the default output is unchanged, and the new state key is optional.
- **Rejected**: listing the new `meta.references` key as breaking, which no reader of earlier state is affected by.

## Rehydration cues

- `spektacular spec file read 20261003153421-bf87d907-references-as-written`
- `spektacular plan file read 20261003134528-327e0657-references-and-secrets plan` (sensitive marker, `RevealSensitive`, wire encoder).
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"gotchas/expression-text-lost-at-parse.md"}'`
- Read `encode.go`, `internal/parser/parser.go:556-905,1081-1235`, `encode_test.go:1-160`.
