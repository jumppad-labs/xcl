---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Context: 20261007111826-cf3b66d8-diff-rendering-and-docs

## Current State Analysis

- The upstream plan `20261007105731-2388b579-diff` (final, not yet implemented) adds the public `diff` package (`Diff`, `Summary`, `Resource`, `Change`, `Action`, `Path`, `Diff.Changed`, `Options`, `RevealSensitive`) and `Config.Diff`; it deliberately leaves rendering and the website page to this plan. The `diff/` directory does not exist yet in the working tree.
- The `highlight` package (`highlight/highlight.go:1-128`, `highlight/ansi.go:1-152`, `highlight/theme.go:1-365`) labels configuration text with xcl-vscode grammar scopes and hands each piece to a `Renderer` (`Render(scope, text string) string`); `ANSIRenderer` writes SGR codes from a theme, leaves unlabelled or unstyled pieces plain, and styles multi-line pieces line by line. `defaultTheme()` (`highlight/theme.go:118-140`) has eight basic-colour rules and none for `markup.*`.
- The encoder's `Highlight(renderer) EncodeOption` (`encode.go:106-120`) passes finished text through `highlight.Text` (`encode.go:255`); its tests use a `[scope]text[/scope]` marker renderer and assert plain output has no `\x1b` (`encode_highlight_test.go:12-56`).
- Entity addresses (the diff's `Resource.Address`) are `FQRN.String()` forms: `[module.<m1>[.<m2>…].]type[.subtype].name` (`internal/resources/fqrn.go:266-291`); provider-backed types can use custom keywords and may have no subtype (`config_plugin_subtypeless_test.go`, `types/register_test.go:19`).
- `internal/xcl/hclsyntax/public.go:199` `ValidIdentifier` and `internal/xcl/hclwrite/generate.go:27` `TokensForValue` exist for map-key and string-literal formatting.
- The sensitive-leak suite (`sensitive_leak_test.go:202-420`) holds one test per output path (events, slog, fmt, encoder); the upstream plan adds diff result cases there.
- xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`): Astro 5 + MDX + expressive-code; guide pages under `src/pages/*.mdx` follow `configuration-text.mdx`'s structure; the Guides menu is a static list in `src/components/Nav.astro:18-26`; `src/pages/sensitive-values.mdx:133-147` lists every output's handling of secrets; no redirects exist (gotcha `no-redirects-on-page-removal`), irrelevant here since no page is removed.

## Per-Task Technical Notes

### Task: Render a diff as plain text

Requirements carried: Readable rendering; Sensitive values stay hidden in the rendering. Repo: xclconfig, root `/home/nicj/code/github.com/jumppad-labs/xcl`. Requires the upstream plan's `diff` package (task "Add the diff result types package", id 620e6cd3-…) to have landed.

**File changes**:
- `diff/render.go` (new) — `type renderOptions struct{ renderer highlight.Renderer }`, `type RenderOption func(*renderOptions)`, `func Render(d *Diff, options ...RenderOption) []byte`. Build with a `strings.Builder` (or `bytes.Buffer`) and a `styler` value with methods used for every piece (`marker(action/kind, text)`, `path(kind, text)`, `comment(text)`, `hcl(text)`, `plain(text)`); in this task the styler is the identity (no renderer field yet used). For each resource in `d.Resources` order: comment `  # <address> <phrase>` where phrase is create "will be created", update with changes "will be updated", update without changes "changed outside xcl and will be updated", replace "will be replaced, its last apply failed", delete "will be deleted"; header `<mark> <header> {` with mark `"  +"`, `"  -"`, `"  ~"`, `"-/+"`; with no changes the header ends ` {}` and no body/closing line; otherwise each change line is 6 spaces + change marker + space + path padded with spaces to `longest+1` (longest `Path.String()` length in this block, measured in runes) + `= ` + value text, then `    }`. One blank line after each block. Summary from `d.Summary`: when `d.Changed() == 0` → `Diff: no changes, <unchanged> unchanged.`, otherwise `Diff: <c> to create, <u> to update, <r> to replace, <d> to delete, <n> unchanged.`; output ends with `\n`. Nil `d` → treated as `&Diff{}`.
  Change line rules (`c := change`): `masked := c.Sensitive && c.Before == nil && c.After == nil`; masked → marker `+` in a create resource else `~`, value `(sensitive value)`; `c.Unknown` → if `c.Before != nil` marker `~` and `<before> -> (known after apply)` else marker `+` and `(known after apply)`; `Before == nil && After != nil` → `+`, `<after>`; `Before != nil && After == nil` → `-`, `<before>`; both → `~`, `<before> -> <after>`. A revealed sensitive change (values present) follows the value rules. The marker set for the line's path colour is the marker's kind (inserted/deleted/changed).
  Doc comments: describe output in diff terms; state that a sensitive value is shown only when the result carries it, i.e. when the diff ran with `RevealSensitive()`.
- `diff/render_value.go` (new) — `func formatValue(value any) string` one-line HCL literal: nil → `null`; string → `hclwrite.TokensForValue(cty.StringVal(s)).Bytes()` (escapes `${`/`%{`, quotes, `\n`); bool → `true`/`false`; signed/unsigned ints via `strconv.FormatInt/FormatUint`; float32/64 → `strconv.FormatFloat(v, 'f', -1, 64)`; `json.Number` → its string; slices/arrays (via `reflect`) → `[a, b]`, empty `[]`; maps (via `reflect`, any key kind formatted with `fmt.Sprint`) → `{ k = v, k2 = v2 }` with keys sorted, key bare when `hclsyntax.ValidIdentifier(key)` else quoted like a string, empty `{}`; pointers dereferenced (nil → `null`); anything else → quoted `fmt.Sprint(value)`. Imports: `github.com/jumppad-labs/xcl/internal/cty`, `github.com/jumppad-labs/xcl/internal/xcl/hclwrite`, `.../hclsyntax` — both exist: `internal/xcl/hclsyntax/public.go:199` `ValidIdentifier`, `internal/xcl/hclwrite/generate.go:27` `TokensForValue`.
- `diff/render_header.go` (new) — `func blockHeader(address string) string`: split on `.`; if `segments[0] == "module"`: body starts at the first index ≥ 2 whose segment is `resource`, else the remaining segments after `segments[1]`, taking the last 3 when there are 3 or more and the last 2 otherwise; no module → whole address. Body of 3 → `type "subtype" "name"`, 2 → `type "name"`, anything else → the address quoted as a single label after the first segment (defensive). Labels quoted with `strconv.Quote`.
- `diff/render_test.go` (new, package `diff_test` or `diff`) — one behaviour per test, `require`: create block with `+` lines and comment; delete header `  - resource "postgres" "old" {}`; replace header `-/+ ... {}` and comment; update with changes `~ path = 8080 -> 9090`; update without changes comment "changed outside xcl"; added element `+ ports[2] = { host = 443, local = 8443 }`; removed element `- ...`; unknown in create `+ db_host = (known after apply)`; unknown with before `~ x = "a" -> (known after apply)`; masked sensitive `~ env["DB_PASSWORD"] = (sensitive value)` and output contains neither secret used to build a revealed counterpart; revealed sensitive shows both values; alignment computed per block independently; order follows `Resources`; summary five numbers for 2/1/1/1/3; no-changes output is exactly `Diff: no changes, 3 unchanged.\n`; nil diff renders `Diff: no changes, 0 unchanged.\n`; output has no `\x1b`; golden test of the design's example `Diff` (build it literally from `config-diff.md` § JSON format) equals the design's text with the `api` block aligned at longest+1.
- `diff/render_value_test.go`, `diff/render_header_test.go` (new) — formatter and header reader cases listed in the Testing Approach, one per test.
- `diff/example_test.go` (new, package `diff_test`) — `func ExampleRender()` builds the design's example result, `fmt.Print(string(diff.Render(d)))`, with `// Output:` holding the rendered text (this is the text the website page copies).

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: 2-3 parallel agents for independent changes (value formatter + tests; header reader + tests; then renderer, example and golden test sequentially on top).

### Task: Prove renderings of a real diff keep secrets hidden

Requirements carried: Sensitive values stay hidden in the rendering (success metric 2). Repo: xclconfig. Requires the upstream plan's `Config.Diff` and its sensitive diff fixture (task "Report value changes in diff results", id 541a9fbb-…).

**File changes**:
- `sensitive_leak_test.go:351-420` (beside the `TestLeakEncode…` cases) — add `TestLeakDiffRenderingOfChangedCredential` and `TestLeakDiffRenderingOfChangedCredentialRevealed` (separate functions: negative and positive). Reuse the upstream plan's credential diff fixture (`internal/test_fixtures/config/diff/credential/main.xcl`, `password = variable.db_password` with a sensitive variable) and its helpers: apply once with one password, change the variable value (by editing a copied fixture or the variable default per the upstream helper), run `c.Diff(paths)`, then `diff.Render(result)` and `diff.Render(result, diff.Highlight(renderer))` once the colour task lands (until then plain only; the colour task adds the highlighted assertion). Assert with distinctive secrets (per gotcha `event-data-includes-temp-paths`) that neither old nor new secret appears and `(sensitive value)` does; revealed variant (`c.Diff(paths, diff.RevealSensitive())`) contains both.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Add diff colours to the highlight package

Requirements carried: Optional colour (scope plumbing). Repo: xclconfig.

**File changes**:
- `highlight/highlight.go:8-97` — after the grammar scope block, add a second `const` group with a doc comment saying these are not grammar scopes but the TextMate diff scopes xcl's diff renderer labels output with, matching how VS Code themes colour diffs: `ScopeInserted = "markup.inserted.diff"`, `ScopeDeleted = "markup.deleted.diff"`, `ScopeChanged = "markup.changed.diff"`, each with a one-line comment.
- `highlight/doc.go:1-16` — one sentence noting the package also names the scopes used for diff output.
- `highlight/theme.go:118-140` — `defaultTheme()` gains `basic("markup.inserted", 32, nil, 8)`, `basic("markup.deleted", 31, nil, 9)`, `basic("markup.changed", 33, nil, 10)`; update the function's doc comment to mention diff colours.
- `highlight/theme_test.go:387-430` — add `TestDefaultThemeColoursInsertedGreen`, `TestDefaultThemeColoursDeletedRed`, `TestDefaultThemeColoursChangedYellow` (resolve `ScopeInserted` etc. via `parsed.style`); existing `TestDefaultThemeUsesOnlyBasicColours` keeps passing.
- `highlight/ansi_test.go` — add a test that a theme read with `WithTheme` containing `{"scope":"markup.inserted","settings":{"foreground":"#00ff00"}}` colours `ScopeInserted` text with 24-bit codes, and one that the reference theme (no markup rules) leaves `ScopeDeleted` text uncoloured; existing sample-rendering tests (configuration text) unchanged.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Colour the rendered diff on request

Requirements carried: Optional colour; Sensitive values stay hidden in the rendering (coloured path). Repo: xclconfig.

**File changes**:
- `diff/render.go` — add `func Highlight(renderer highlight.Renderer) RenderOption` (doc modelled on `encode.go:106-120`: xcl never checks for a terminal; nil renderer renders plain). The styler, when `opts.renderer != nil`: marker and path → `renderer.Render(scopeFor(kind), text)` with inserted/deleted/changed (`-/+` → changed); comment line text (without leading indentation) → `renderer.Render(highlight.ScopeHashComment, text)`; header text (`resource "container" "api" {`/`{}`) and formatted values → `string(highlight.Text([]byte(text), renderer))`; `=`, `->`, padding, closing brace, placeholders and summary → `renderer.Render("", text)` so escaping renderers see every byte. Indentation stays outside any styled piece.
- `diff/render_test.go` — add: plain output contains no `\x1b`; with a marker renderer (copy the `[scope]text[/scope]` technique from `encode_highlight_test.go:12-32`, kept local to the diff tests) each marker/path piece carries the expected scope per action, comment lines carry `ScopeHashComment`, values carry grammar scopes (e.g. `ScopeNumber`, `ScopeString`); with `highlight.NewANSIRenderer()` output contains `\x1b[`; stripping SGR codes (`\x1b\[[0-9;]*m`) from that output equals `Render(d)`; with a theme lacking markup rules markers are not wrapped; `Highlight(nil)` equals plain; masked sensitive change in coloured output contains neither secret.
- `sensitive_leak_test.go` — extend the two tests from "Prove renderings of a real diff keep secrets hidden" to also render with `diff.Highlight(renderer)` and assert the same.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Write the diff guide on the documentation site

Requirements carried: Documentation. Repo: xcl-website, root `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

**File changes**:
- `xcl-website:src/pages/diff.mdx` (new) — follow `xcl-website:src/pages/configuration-text.mdx:1-15` frontmatter and imports (`layout: ../layouts/Shell.astro`, `title: "Diffs - xcl"`, description, `Hero`, `Prose`, `CtaBanner`, `Button`). Sections: what a diff is (what an apply would do, nothing created, changed, destroyed or saved; providers' read and change checks show drift); running one (`d, err := c.Diff(paths)`, `d.Changed()`, `d.Summary`, iterating `d.Resources`/`Changes`, `json.Marshal(d)`); the four actions and when each happens (table from `config-diff.md` § Actions) and unchanged resources counted not listed; known after apply; sensitive values (`(sensitive value)`, `diff.RevealSensitive()` and that the result then holds the secrets); reading the rendered output (`diff.Render(d)`, a ```text block copied verbatim from `diff/example_test.go`'s `// Output:`, then a marker table `+`, `-`, `~`, `-/+`, and the summary forms including `Diff: no changes, 9 unchanged.`); colour (`diff.Render(d, diff.Highlight(renderer))` with `highlight.NewANSIRenderer()` and a theme file, the three scopes for custom renderers, link to `/configuration-text/#highlighting`). Closing `CtaBanner` linking to `/sensitive-values/` and `/configuration-text/`. Use "diff" throughout, never "plan".
- `xcl-website:src/components/Nav.astro:18-26` — add `{ label: "Diffs", href: "/diff/" }` to the Guides children (after "Configuration text").
- `xcl-website:src/pages/sensitive-values.mdx:133-147` — add a bullet: **Diffs** from `Config.Diff` report that a sensitive value changed without either value, and `diff.Render` shows `(sensitive value)`; pass `diff.RevealSensitive()` to include the values.
- Verify with the site's `make build` (and `make check`).

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

## Testing Strategy

Per task:

- **Render a diff as plain text** — unit tests in `diff/render_test.go` (one rule per test: each action's comment, header and body; value markers; unknown and sensitive placeholders; per-block alignment; order; summary forms; nil diff; no escape codes; design-example golden), `diff/render_value_test.go` (formatter cases), `diff/render_header_test.go` (address forms), and `ExampleRender` in `diff/example_test.go` whose checked output is the website's example.
- **Prove renderings of a real diff keep secrets hidden** — two root-package tests in `sensitive_leak_test.go` through `Config.Diff` + `diff.Render`: masked (no secret, placeholder present) and revealed (both values present).
- **Add diff colours to the highlight package** — default-theme tests for the three new rules (basic green/red/yellow; still only basic colours), an ANSI test that a theme's `markup.inserted` rule colours `ScopeInserted`, and one that a theme without markup rules leaves `ScopeDeleted` plain; existing configuration-text rendering tests unchanged.
- **Colour the rendered diff on request** — `diff/render_test.go` additions: plain has no `\x1b`; marker-renderer scope assertions per piece; ANSI output contains codes and equals plain once stripped; theme without diff rules leaves markers plain; `Highlight(nil)` equals plain; masked secret absent from coloured output; leak-suite tests extended to highlighted renderings.
- **Write the diff guide on the documentation site** — no automated tests (no test may read the page); the site's build and type-check, and the manual checks in the test plan: page example identical to `ExampleRender`'s output, page content review, terminology review.

Success metrics and manual reviews (carried to the implementation test plan):
- Page example matches the renderer line for line — behavioural (`ExampleRender`) plus **Manual — captured in the implementation test plan** comparison of the page block.
- No sensitive value in any unrevealed rendering — behavioural (renderer unit tests, leak-suite tests).
- **Manual — captured in the implementation test plan**: page content review and site build; terminal colour check with default and a VS Code theme; "diff, never plan" terminology review.

## Project References

- Spec `20261007111826-cf3b66d8-diff-rendering-and-docs` (`spektacular spec file read …`).
- Design `config-diff.md` from the `design` source — § Rendering is binding.
- Upstream plan `20261007105731-2388b579-diff` (plan, context, research).
- Knowledge: `xclconfig` conventions (code style, project structure, testing and mocking, shared test helpers, never modify dependencies, dependencies), glossary `entity`; gotchas `sensitive-values-leak-through-fmt`, `event-data-includes-temp-paths`; `xcl-website` gotcha `no-redirects-on-page-removal`.
- Repo roots: xclconfig `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

"Render a diff as plain text" is Medium (~40k); every other task is Low (10-20k). Tasks "Render a diff as plain text" and "Add diff colours to the highlight package" have no dependency on each other and can run in parallel.

## Migration Notes

None. Additions only: new functions in the `diff` package, three new constants and three default-theme rules in `highlight`. The default-theme additions change no existing output, because the xcl-vscode grammar never emits `markup.*` scopes.

## Performance Considerations

Render is linear in the number of changes; values are formatted once each and highlighting runs only on short header and value strings. No performance requirement applies.
