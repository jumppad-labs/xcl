---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Research: 20261006112108-17623cda-encoder-syntax-highlighting

## Alternatives considered and rejected

- **Regular-expression, line-by-line highlighter (today's prettylog approach).** `example/prettylog/highlight.go:11-23` matches block headers, attributes and value tokens with regexes, line at a time. Rejected: the spec's Technical Approach mandates the HCL scanner, and regexes cannot handle multi-line strings, heredocs, block comments or nested interpolation correctly.
- **Highlight from `hclwrite` tokens before formatting.** `encode.go:226-229` builds an `hclwrite.File` and `file.Bytes()` runs the formatter, which changes spacing. Tokens taken before formatting would not line up with the final bytes. Rejected in favour of lexing the final bytes with `hclsyntax.LexConfig` (`internal/xcl/hclsyntax/public.go:159`), which gives byte ranges into exactly the text returned.
- **Running the TextMate grammar itself in Go (an Oniguruma/TextMate engine port).** Would give exact parity with `xcl-vscode:syntaxes/xcl.tmLanguage.json` but needs a regex engine with lookbehind (Go's RE2 has none) or a cgo/WASM dependency. Rejected: violates "prefer standard library" (`conventions/dependencies.md`) and the scanner steer.
- **Using lipgloss/termenv/fatih-color for ANSI output.** `go.mod` already lists `fatih/color` and `muesli/termenv`, but charmbracelet is banned from the library (a rule kept by review) and the escape codes needed (SGR 30-37/90-97, 38;2;r;g;b, 1/3/4/9, 0) are trivial. Rejected: write SGR codes with the standard library; no new dependency.
- **Full TextMate scope-stack matching (descendant selectors, exclusions).** The spec fixes the renderer contract as `(scope, text)` pairs, i.e. one scope per token, so a stack is not available. Rejected; descendant selectors match on their last element (see assumptions).
- **Placing the package under `/pkg`.** `conventions/project-structure.md` lists `/pkg`, but the module's public packages are all top-level (`mask/`, `events/`, `state/`, `types/`), and the masking plan (20261003153421-9fa72edd-masking) read the convention as "/internal is private" and added `mask` at the top level. Rejected for consistency; new package is top-level `highlight`.
- **A test globbing every `.xcl` in the repo for the "text unchanged" criterion.** Violates `conventions/testing-and-mocking.md` ("never put tests ... that reach into another directory (globbing, parsing or building sources elsewhere)"); `static_examples_test.go` was deleted for this reason. Rejected; in-package corpus + fuzz test, with the whole-repo sweep as a manual check.

## Chosen approach — evidence

- `encode.go:27-39` — options are a closed `encodeOptions` struct set by `EncodeOption` funcs; a `Highlight(renderer)` option slots in the same way without changing any existing call.
- `encode.go:180-230` — `encodeEntity` is the single path for both `EncodeEntity` and `EncodeSavedEntity`, returning `file.Bytes()`; highlighting applied after that covers every encoder output, including `ShowReferences` text (`encode.go:281`).
- `internal/xcl/hclsyntax/public.go:159` — `LexConfig(src, filename, start) (Tokens, hcl.Diagnostics)`; `internal/xcl/hclsyntax/token.go:16-20` tokens carry `Bytes` and `Range` (byte offsets), so gaps (whitespace) can be filled from the source, guaranteeing the text is unchanged by construction.
- `internal/xcl/hclsyntax/token.go:34-108` — token types needed: `TokenIdent`, `TokenOQuote/CQuote`, `TokenQuotedLit` (may hold escapes), `TokenStringLit`, `TokenNumberLit`, `TokenComment`, `TokenOHeredoc/CHeredoc`, `TokenTemplateInterp/Control/SeqEnd`, `TokenDot`, operators, `TokenInvalid`.
- `xcl-vscode:syntaxes/xcl.tmLanguage.json` — the scope set to match: `comment.line.double-slash.xcl`, `comment.line.number-sign.xcl`, `comment.block.xcl`, `storage.type.xcl`, `entity.name.type.xcl`, `entity.name.tag.xcl`, `entity.name.function.block.xcl`, `variable.other.property.xcl`, `constant.language.xcl`, `constant.numeric.xcl`, `support.function.builtin.xcl`, `entity.name.function.xcl`, `support.class.reference.xcl`, `punctuation.accessor.xcl`, `variable.other.member.xcl`, `keyword.operator.xcl`, `string.quoted.double.xcl`, `constant.character.escape.xcl`, `meta.interpolation.xcl`, `punctuation.section.interpolation.begin.xcl`/`.end.xcl`, `string.unquoted.heredoc.xcl`, `keyword.operator.heredoc.xcl`, `keyword.control.heredoc.xcl`, `meta.interpolation.template.xcl`.
- `xcl-vscode:test/fixtures/sample.xcl` + `test/tokenize.test.js` — the grammar's own fixture and the token/scope pairs it asserts; a model for the Go parity fixture.
- `conventions/testing-and-mocking.md` — forbids tests that inspect repository files (imports, `go.mod`, source scans), so the charmbracelet boundary for the new `highlight` package is checked in review, not by a test.
- `example/prettylog/prettylog.go:73-104` — the only place examples print configuration; it decides colour via `lipgloss.NewRenderer(w)` and calls `EncodeSavedEntity(reg, e.Data, xcl.IncludeComputed())`. The migration swaps its highlighter for `xcl.Highlight(...)`, keeping the terminal decision in prettylog.
- `example/prettylog/highlight.go:42-53` — current 16-colour palette (magenta bold block type, cyan type label, green name label, blue attribute, yellow string, bright magenta constant, cyan reference root, bright-black italic comment): basis for the default theme.
- `errors/encode_errors.go`, `plugins/errors.go` — sentinel-and-detail pattern; `config.go:16` re-export pattern.
- `xcl-website:src/pages/configuration-text.mdx` — the encoding page; sections at lines 23-180, a new "Highlighting" section fits before "For reading, not reprocessing" (line 180).

## Files examined

- `encode.go:1-402` — options, `EncodeEntity`, `EncodeSavedEntity`, single `encodeEntity` path, `ShowReferences` rewriting.
- `internal/xcl/hclsyntax/public.go:159` — `LexConfig` entry point.
- `internal/xcl/hclsyntax/token.go:16-108` — token struct and types.
- `example/prettylog/highlight.go` — regex highlighter to delete.
- `example/prettylog/highlight_test.go` — tests to delete/replace; `ansiEscapes` strip regex `\x1b\[[0-9;]*m`.
- `example/prettylog/prettylog.go:56-122` — handler wiring, colour decided per writer, `indent` applied after encoding.
- `example/prettylog/prettylog_test.go:234-447` — handler tests with real apply; coloured output assertions to adjust.
- `errors/` — home for shared error types (`encode_errors.go` etc.).
- `go.mod` — Go 1.25; no new dependency needed.
- `xcl-vscode:syntaxes/xcl.tmLanguage.json` — grammar (working tree has uncommitted edits; this is the version read).
- `xcl-vscode:test/fixtures/sample.xcl`, `test/tokenize.test.js` — grammar fixture and expectations.
- `xcl-website:src/pages/configuration-text.mdx` — encoding docs page.

## External references

- VS Code Color Theme reference (code.visualstudio.com/api/extension-guides/color-theme) — `tokenColors` entries with `scope` (string, comma-separated string, or array) and `settings.foreground` / `settings.fontStyle`; themes are JSONC.
- TextMate scope selector / theme matching rules (macromates.com/manual/en/scope_selectors) — prefix matching on dot segments; more specific (longer) selector wins; VS Code resolves foreground and fontStyle independently.
- ECMA-48 SGR codes — 30-37/90-97 foreground, 38;2;r;g;b truecolour, 1 bold, 3 italic, 4 underline, 9 strikethrough, 0 reset.

## Prior plans / specs consulted

- `20261003153421-9fa72edd-masking` (plan) — precedent for a new top-level public package (`mask`) with errors in `errors` re-exported from `xcl`; read "/pkg" convention as "/internal is private".
- `20261006071142-506b8289-e2e-suite-and-real-world-examples` (spec, sibling in epic) — makes prettylog self-contained (own module, builds alone against a published xcl) and explicitly leaves highlighting to this spec. Both touch `example/prettylog/`.

## Open assumptions

- The xcl-vscode grammar in the working tree (with uncommitted edits) is the reference scope set. If those edits are discarded or change, the parity fixture must follow.
- `hclsyntax.LexConfig` returns usable tokens with byte ranges even for text with lex errors (e.g. `TokenInvalid`); gap filling keeps text unchanged regardless.
- The sibling e2e spec may land first and move prettylog into its own module; the migration task applies wherever prettylog lives then. If prettylog then pins a published xcl without `highlight`, the migration needs a release or a replace directive — STOP and ask if so.

## Drafting assumptions

### Chosen direction: post-pass highlighter over the encoder's final text (architecture)
- **Decision**: a new top-level `highlight` package lexes the formatted encoder output with `hclsyntax.LexConfig`, labels tokens with the xcl-vscode TextMate scopes, and passes every byte (unlabelled text with scope "") to a one-method `Renderer`; `xcl.Highlight(r)` enables it; `NewANSIRenderer` with `WithTheme`/`WithThemeFile` options reads VS Code themes; default theme uses 16 colours only.
- **Key design decisions**: label only the innermost scope (the spec's `(scope, text)` contract); unlabelled text goes through the renderer so escaping formats work; multi-line tokens are styled per line; foreground and fontStyle resolved independently; errors in `errors` re-exported from `xcl`.
- **Rejected**: regex highlighter, TextMate engine port, hclwrite pre-format tokens (see research.md).

### New public package is top-level `highlight` (discovery)
- **Decision**: put the tokeniser, `Renderer` interface, theme loading and ANSI renderer in a new top-level package `github.com/jumppad-labs/xcl/highlight`.
- **Rationale**: every public package in the module is top-level (`mask`, `events`, `state`, `types`); the masking plan read the `/pkg` line of the project-structure convention as "/internal is private" and placed `mask` at the top level.
- **Rejected**: `pkg/highlight` (inconsistent with every existing public package); putting it in the root `xcl` package (bloats root, and a renderer author should not need the whole config API).

### "Text unchanged" across the whole repo is a manual check (discovery)
- **Decision**: automate text preservation inside the `highlight` package (its own testdata corpus covering every scope, plus a Go fuzz test), and verify "every configuration in the repository's examples and test fixtures" as a manual check.
- **Rationale**: the testing convention forbids tests that glob or parse sources in other directories; gap-filling from token byte ranges makes preservation hold by construction.
- **Rejected**: a test walking `internal/test_fixtures` and `example/` from the highlight package (breaks the convention).

### Plan against the xcl-vscode working tree grammar (discovery)
- **Decision**: the scope set is the one in the working-tree `syntaxes/xcl.tmLanguage.json`, which has uncommitted edits.
- **Rationale**: it is the newest grammar on disk; the spec names "the same set the xcl-vscode grammar assigns".
- **Rejected**: the last committed grammar (likely to be superseded by the pending edits).

### Descendant selectors match on their last element (architecture)
- **Decision**: a theme selector with spaces (`meta.interpolation string`) is matched using its last element only; exclusion selectors (`-`) are ignored along with the part they exclude.
- **Rationale**: the renderer sees one scope per token, so there is no stack to check parents against; taking the last element keeps the theme's intended colour for the token itself.
- **Rejected**: dropping such rules entirely (loses colour many themes rely on); passing a full scope stack (contradicts the spec's `(scope, text)` renderer shape).

### Theme JSON accepts comments and trailing commas; `include` is not followed (architecture)
- **Decision**: the theme reader strips `//` and `/* */` comments and trailing commas outside strings before decoding with `encoding/json`; a theme's `include` key and a string-valued `tokenColors` (a path to a .tmTheme) are rejected as invalid.
- **Rationale**: published VS Code themes are commonly JSONC; following includes needs a file system root and is beyond "accepts an editor colour theme".
- **Rejected**: strict JSON only (rejects many real themes); a JSONC dependency (stdlib preference).

### Scope-less theme entries and editor colours are ignored (architecture)
- **Decision**: `tokenColors` entries with no `scope` and the theme's `colors` block (e.g. `editor.foreground`) set nothing.
- **Rationale**: the spec requires unmatched tokens to keep the terminal's default colour.
- **Rejected**: using the theme's global foreground for unmatched text.

### Multi-line tokens are styled per line (architecture)
- **Decision**: the ANSI renderer closes the style before each newline in a token and reopens it after.
- **Rationale**: prettylog indents encoder output line by line after encoding; per-line styling keeps indentation uncoloured and stops colour leaking into following terminal lines.
- **Rejected**: one escape span across newlines.

### Default theme carries over prettylog's palette (architecture)
- **Decision**: `storage.type` magenta bold, `entity.name.type` cyan, `entity.name.tag` green, `variable.other.property` blue, `string` yellow, `constant` bright magenta, `support.class.reference` cyan, `comment` bright black italic; functions, operators and punctuation default colour.
- **Rationale**: keeps the logging example's look; all codes in the 16-colour set.
- **Rejected**: a new palette (no requirement asks for one).

### Errors in the `errors` package, re-exported (architecture)
- **Decision**: `ErrInvalidTheme` and `InvalidThemeError` go in `errors/highlight_errors.go` and are re-exported from `xcl`.
- **Rationale**: the shared-errors convention and the precedent of `NotEncodableError`.
- **Rejected**: defining them in `highlight` (breaks the convention).

### Conventions selected (architecture)
- **Decision**: the conventions applied are testing and mocking, shared test helpers, shared errors, dependencies, code style, project structure and never modifying dependencies. The ones dropped are database, development standards (structured logging: nothing here logs), patterns and architecture beyond dependency injection, test state from a real apply, and graph ordering.
- **Rationale**: these are the only ones this feature touches.
- **Rejected**: listing every convention.

### Shared ANSI strip helper lands with the first task (tasks)
- **Decision**: `testutil.StripANSI` is added to `internal/testutil` in the first `highlight` task and used by `highlight` tests; prettylog keeps its own local regex.
- **Rationale**: shared-test-helpers convention for helpers used by more than one package; prettylog is becoming a self-contained module (sibling spec) and cannot import `internal/`.
- **Rejected**: copying the regex into every package.

### prettylog's colour decision stays with lipgloss (tasks)
- **Decision**: prettylog decides colour with `lipgloss.NewRenderer(w).ColorProfile() != termenv.Ascii` and passes `xcl.Highlight` only when colour is on; tests force colour with `CLICOLOR_FORCE=1`.
- **Rationale**: the library must not detect terminals (constraint), and the example already makes this decision today with lipgloss.
- **Rejected**: making the library or the ANSI renderer decide.

### No changelog task (tasks)
- **Decision**: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented. The plan's "Changelog input" notes say what that entry should cover for this spec.
- **Rationale**: specs in the epic touch different code and must not collide on `CHANGELOG.md` or be forced to run in sequence.

### Website docs task depends on the API tasks (tasks)
- **Decision**: the xcl-website task depends on the ANSI renderer and encode option tasks so the page uses final names.
- **Rationale**: the docs must match the real API.
- **Rejected**: drafting the docs in parallel from the plan.

## Rehydration cues

- `spektacular spec file read 20261006112108-17623cda-encoder-syntax-highlighting`
- `spektacular knowledge always-applied --tier repo --filter xclconfig`
- Re-read `encode.go`, `example/prettylog/{prettylog,highlight}.go`.
- Re-read `/home/nicj/code/github.com/jumppad-labs/xcl-vscode/syntaxes/xcl.tmLanguage.json` and `test/fixtures/sample.xcl`.
- Re-read `/home/nicj/code/github.com/jumppad-labs/xcl-website/src/pages/configuration-text.mdx`.
