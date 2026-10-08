---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Research: 20261007111826-cf3b66d8-diff-rendering-and-docs

## Alternatives considered and rejected

- **Format values with `hclwrite.TokensForValue` on cty values (via ctyjson implied types)** — rejected for collections: hclwrite writes objects and lists across several lines (`internal/xcl/hclwrite`, as used by `encode.go:222-260`), while the design's rendering is one line per change with inline values (`ports[2] = { host = 443, local = 8443 }`, `config-diff.md` § Rendering). hclwrite is kept only for quoting strings, so `${`/`%{` and escapes come out as valid HCL.
- **Colour the diff with a new, diff-specific colour mechanism (own SGR codes or a second renderer interface)** — rejected: the spec's Technical Approach says to reuse the highlight renderer and themes, and the design says lines are coloured "through the same `highlight.Renderer` and themes the encoder uses" (`highlight/highlight.go:99-112` Renderer is `Render(scope, text string) string`). Colouring is therefore expressed as scopes the renderer resolves through the theme.
- **Use invented scopes such as `diff.added`** — rejected: VS Code themes colour diffs through the TextMate `markup.inserted`, `markup.deleted` and `markup.changed` scopes (the built-in diff grammar labels them `markup.inserted.diff`, etc.), and the theme matcher resolves by dot-segment prefix (`highlight/theme.go:101-105`), so `markup.inserted.diff` picks up a theme's `markup.inserted` rule. Invented names would match no real theme, leaving every theme plain.
- **Add `Type`/`Subtype`/`Name` fields to `diff.Resource` so the header need not be derived from the address** — rejected: the design fixes the result types and JSON (`config-diff.md` § Types, § JSON format) and the dependency plan implemented exactly those; adding fields would contradict the design.
- **Parse the header with `resources.ParseFQRN`** — rejected as the sole rule: it only knows the structural keywords (`internal/resources/fqrn.go:75-82`), so a provider-backed entity with a custom keyword inside a module (`module.a.server.big.web`) is read as a module address (`fqrn.go:131-143`), and the diff package has no registry at render time. A small positional reader in the diff package is used instead (see Open assumptions).
- **Check the docs example against the renderer with a test that reads the website page** — rejected: the testing convention forbids tests that inspect repository files (`conventions/testing-and-mocking.md`). The check is a runnable Go `Example` whose `// Output:` is the rendered design example, plus a manual review that the page's block matches it.

## Chosen approach — evidence

- `highlight/highlight.go:99-128` — `Renderer` interface, `RendererFunc`, `Text(text, renderer)` (nil renderer returns text as is): values can be highlighted as HCL by `highlight.Text(valueBytes, renderer)` and individual pieces (markers, paths, comments) coloured by `renderer.Render(scope, text)`.
- `highlight/ansi.go:95-115` — `ANSIRenderer.Render` writes no codes for an empty scope or a scope the theme leaves unstyled: exactly the design's "themes that do not define those colours fall back to plain text".
- `highlight/theme.go:118-140` — `defaultTheme()` uses 16 basic colours and has no `markup` rules; adding `markup.inserted` (32 green), `markup.deleted` (31 red), `markup.changed` (33 yellow) keeps `TestDefaultThemeUsesOnlyBasicColours` (`highlight/theme_test.go:387-398`) passing.
- `highlight/highlight.go:8-97` — scope constants are exported strings documented as xcl-vscode grammar names; new diff scopes are a separate, documented group.
- `encode.go:106-120,255` — the encoder's `Highlight(renderer) EncodeOption` passes finished text through `highlight.Text`; `Render`'s `Highlight(renderer) RenderOption` mirrors that name and doc style.
- `encode_highlight_test.go:12-56` — test pattern: a marker renderer (`[scope]text[/scope]`) to assert which scope each piece got, and `require.NotContains(out, "\x1b")` for plain output.
- `encode_highlight_test.go:40-52` and encoder output — `=` aligned one space after the longest name in a block (gofmt style).
- Dependency plan `20261007105731-2388b579-diff` (final): `diff` package types; `Before`/`After` hold plain Go values (strings, any numeric kind, bools, `[]any`, `map[string]any`, nil); sensitive entries carry no values unless revealed, and keep `Sensitive: true` when revealed; `Resources` sorted by address; `Diff.Changed()` nil-safe.
- `internal/resources/fqrn.go:266-291` — entity IDs (the diff's `Address`) are `module.<m1>.<m2>.` + `type[.subtype].name`.
- `xcl-website:src/pages/configuration-text.mdx` — page pattern: frontmatter (layout Shell, title "… - xcl", description), `Hero`, `Prose`, `CtaBanner` with `Button`s; fenced `go`/`hcl`/`text` blocks rendered by expressive-code.
- `xcl-website:src/components/Nav.astro:8-27` — Guides dropdown lists every guide page; a new page needs an entry there.
- `xcl-website:src/pages/sensitive-values.mdx:133-147` — "What xcl shows, and what it keeps" lists every output that handles sensitive values; diffs belong in it.

## Files examined

- `highlight/doc.go:1-16` — package doc: renderer decides output; package never checks for a terminal.
- `highlight/highlight.go:1-128` — scope constants, `Renderer`, `RendererFunc`, `Text`.
- `highlight/ansi.go:1-152` — `ANSIRenderer`, options, per-line SGR wrapping, unstyled scopes plain.
- `highlight/theme.go:1-365` — TextMate prefix matching, `defaultTheme()` rules, theme parsing.
- `highlight/theme_test.go:387-430` — default-theme tests (basic colours only; operator unstyled).
- `highlight/ansi_test.go:25-60` — test helpers for default and reference themes.
- `highlight/tokenize.go:1-120` — tokenizer labels any input; invalid text passes through unlabelled.
- `encode.go:1-120,222-260` — encoder options, `Highlight`, hclwrite use for blocks and strings.
- `encode_highlight_test.go:1-60` — marker-renderer test pattern, plain-output assertion.
- `internal/resources/fqrn.go:1-330` — address forms, module prefix, positional parsing, custom keywords.
- `config_plugin_subtypeless_test.go:1-40` — provider-backed types can be subtype-less with custom keywords.
- `types/register_test.go:13-19` — custom keyword with subtype (`server "big"`).
- `xcl-website:astro.config.mjs`, `ec.config.mjs` — Astro 5 + MDX + expressive-code (github-dark), no redirects.
- `xcl-website:src/pages/configuration-text.mdx` — guide page structure and highlighting section to link to.
- `xcl-website:src/pages/sensitive-values.mdx:133-147` — sensitive outputs list.
- `xcl-website:src/components/Nav.astro:8-27` — nav entries.
- `xcl-website:Makefile` — `make build`, `make check` (astro check).

## External references

- TextMate scope naming conventions (`markup.inserted`, `markup.deleted`, `markup.changed`) and VS Code's built-in diff grammar (`markup.inserted.diff`, …) — why these scopes pick up real themes' diff colours.
- Go `testing` package runnable examples (`func ExampleX` with `// Output:`) — lets the docs example be checked by running code, not by reading files.

## Prior plans / specs consulted

- `20261007105731-2388b579-diff` plan, context, research (final) — defines the `diff` package this plan extends, value shapes in `Before`/`After`, sensitive and unknown semantics, sorting; its Out of Scope assigns `Render`, `Highlight` and the website page to this spec.
- Design `config-diff.md` (source `design`) — binding: `Render`/`Highlight` signatures, markers, comment texts, placeholders, summary line forms, colour behaviour.
- Spec `20261006112108-17623cda-encoder-syntax-highlighting` (implemented, per git log) — origin of the highlight package and the encoder `Highlight` option reused here.

## Open assumptions

- The dependency plan's `diff` package exists with the planned types before this plan is implemented; if its shapes differ (e.g. `Before`/`After` can hold types other than plain Go values), STOP and ask.
- A sensitive change's values are shown exactly when the result carries them (`Before` or `After` non-nil), which only happens when the diff ran with `RevealSensitive()`; Render takes no reveal option of its own.
- Header derivation from the address: outside a module, `type.subtype.name` → `type "subtype" "name"`, `type.name` → `type "name"`; inside a module, the body starts at a `resource` segment if present, else 2 trailing segments after the first module name → `type "name"`, 3 or more → the last three. The only misread case is a subtype-less custom type inside a nested module; the comment line always shows the exact address.
- `markup.*` scopes do not collide with any scope the xcl-vscode grammar emits, so encoder output is unaffected by the default-theme additions.

## Drafting assumptions

### Chosen direction: single line builder with a pluggable styler (architecture)
- **Decision**: implement Render as one line builder whose every piece goes through a styler that is identity without `Highlight` and the caller's `highlight.Renderer` with it; action colours via exported `markup.*.diff` scopes; values and headers via `highlight.Text`.
- **Rationale**: one code path guarantees plain output equals coloured output with codes stripped, reuses the renderer and themes as the spec and design require, and keeps Render pure and small.
- **Rejected**: render plain then `highlight.Text` the whole output (tokenizer mislabels markers and cannot tell actions apart); a line model with separate plain and colour printers (duplicated logic).

### Conventions selected (architecture)
- **Decision**: apply code style, top-level public packages, diff-package import rule, stdlib-first, testing conventions, no repo-file tests, no dependency edits, entity glossary; drop database, HTTP, logging, shutdown, state, DAG and shared-errors conventions.
- **Rationale**: Render is pure formatting with no I/O, errors or state.
- **Rejected**: listing every convention (noise).

### Alignment of `=` within a block (discovery)
- **Decision**: pad every path in a resource block to the longest path in that block plus one space, so `=` sits one column after the longest path (as in the design's `web` block and the encoder's own output).
- **Rationale**: the design states the rule "`=` signs aligned within the block"; its `api` example block pads to longest+3 while its `web` block pads to longest+1, so the example is self-inconsistent and the stated rule plus the encoder's gofmt style decide it.
- **Rejected**: longest+3 (matches one design block only and no other output in xcl). The design's `api` example should be corrected to match; flagged for review.

### Diff colours through `markup.*` scopes (discovery)
- **Decision**: colour markers, paths and header markers through the TextMate scopes `markup.inserted.diff`, `markup.deleted.diff`, `markup.changed.diff`, exported as `highlight.ScopeInserted`, `ScopeDeleted`, `ScopeChanged`; replace uses the changed scope; the default theme gains green/red/yellow basic-colour rules for them.
- **Rationale**: the design names the theme's "inserted", "deleted" and "changed" colours, which VS Code themes define as `markup.inserted`/`deleted`/`changed`; without default rules the default renderer would leave every marker plain.
- **Rejected**: invented scope names (no theme defines them); leaving the default theme without diff colours (default colour output would show no diff colours).

### Header derived from the address (discovery)
- **Decision**: Render derives the block header (`resource "container" "api"`) from `Resource.Address` with a positional reader in the `diff` package; module prefixes appear only in the comment line.
- **Rationale**: the design fixes the types (no type/subtype/name fields) and Render has no registry; the reader is exact for every root-level entity and every `resource` entity in a module.
- **Rejected**: new fields on `diff.Resource` (contradicts the design's types); `resources.ParseFQRN` alone (misreads custom keyword types in modules).

### Value formatting (discovery)
- **Decision**: values render on one line as HCL literals: strings quoted with HCL escaping, numbers in shortest form, bools, `null` for nil, lists `[a, b]`, maps/objects `{ key = value, … }` with keys sorted and quoted when not identifiers, empty `[]`/`{}`; other Go kinds via reflection, falling back to a quoted `fmt` form.
- **Rationale**: matches the design example `{ host = 443, local = 8443 }` and keeps each change on one line.
- **Rejected**: multi-line hclwrite output (breaks one-line-per-change).

### Markers for change entries (discovery)
- **Decision**: `+ path = after` when only `after` (or unknown with no `before`) is present; `- path = before` when only `before`; `~ path = before -> after` when both; unknown with `before` shows `~ path = before -> (known after apply)`; a masked sensitive entry is `+` in a create and `~` otherwise, showing `(sensitive value)`; a revealed sensitive entry follows the value rules.
- **Rationale**: the design's marker table and example; a masked entry carries no values to decide add/remove.
- **Rejected**: `- path = before -> null` (Terraform style, not in the design).

### Output framing (discovery)
- **Decision**: one blank line between resource blocks and before the summary; output ends with a newline; a nil `*Diff` renders as an empty diff; "no changes" form used when `Changed() == 0`, numbers taken from `Summary`.
- **Rationale**: matches the design example; the summary must show the result's numbers.
- **Rejected**: recomputing counts from `Resources` (could disagree with the result's summary).

### Documentation scope (discovery)
- **Decision**: a new website guide page `/diff/` plus a Guides nav entry, a "Diffs" bullet on the sensitive-values page, and no README change.
- **Rationale**: the spec names the documentation site; the sensitive-values page lists every output that handles secrets, so leaving diffs out would make it wrong.
- **Rejected**: README section (not required by the spec); features grid card on the home page (not required).

### What "coloured lines" covers (verification)
- **Decision**: on a change line the marker and the path take the action's colour, while the value is coloured as HCL; on a header the marker takes the action's colour and the block text is coloured as HCL.
- **Rationale**: the design says both that added/removed/changed lines use the theme's diff colours and that values are highlighted as HCL; colouring marker and path by action and values as HCL satisfies both.
- **Rejected**: colouring the whole line in the action colour (would override the HCL value highlighting the design also asks for).

## Rehydration cues

- `spektacular spec file read 20261007111826-cf3b66d8-diff-rendering-and-docs`
- `spektacular design read --data '{"source":"design","path":"config-diff.md"}'` (§ Rendering)
- `spektacular plan file read 20261007105731-2388b579-diff plan` (and `context`)
- Re-read `highlight/highlight.go`, `highlight/ansi.go`, `highlight/theme.go:118-140`, `encode.go:106-120`, `encode_highlight_test.go:1-60`.
- Re-read `xcl-website:src/pages/configuration-text.mdx`, `src/components/Nav.astro`, `src/pages/sensitive-values.mdx:133-147`.
