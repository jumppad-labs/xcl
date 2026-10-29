---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Research: 20261023074926-eaa51527-printing-and-highlighting-core

## Alternatives considered and rejected

### Option A: `errors` imports `highlight` so `Diagnostic` lays its report out as tokens itself

**Rejected**: `highlight` already imports `errors` for `InvalidThemeError` (`highlight/theme.go:12`, `highlight/ansi.go:9`), so this is an import cycle. Breaking it by moving theme errors into `highlight` contradicts the knowledge entry `conventions/shared-errors-package.md` ("shared error types live in `errors`", "keep that import list thin").

### Option B: Kinds and tokenizer in an internal package, with `highlight` re-exporting them through type aliases

**Rejected**: Every public type, constant and method would appear in godoc as `= internal/...` aliases, and the design names `highlight.Kind`, `highlight.Token` and `highlight.Tokenize` as the public home.

### Option C: A hook variable in `errors` that the root package sets in `init()`, so `%+v` can call the root report code

**Rejected**: It is hidden global coupling, and a `Diagnostic` used without importing the root package would silently print one line.

### Option D: A second plain-text report layout in `errors`, separate from the token layout in the root package

**Rejected**: Two layouts drift apart. The AC that `%+v` equals plain `PrintError` would catch the drift, but would not prevent it.

### Option E: Reference inputs as configuration text that every language lexes itself

**Rejected**: The Python formatter parity check would need the engine. Token JSON lets a language test its formatters with no engine at all. Lexing parity is the engine spec's job.

### Option F: Golden tests in the root package that write into `highlight/testdata`

**Rejected**: under `conventions/testing-and-mocking.md` ("never put tests ... that reach into another directory"). The reference test lives in `highlight` as an external `highlight_test` package. It imports `xcl` and `diff` to build its inputs, which is exercising code, not reading sources.

### Option G: chroma for lexing, styles or formatters

**Rejected**: by the spec constraint and the design decision "chroma is not used directly". Only its shape is borrowed.

### Option H: `highlight.For` for every example's colour decision

**Rejected**: for the examples that already use lipgloss. `example/prettylog/prettylog_test.go:484` forces colour through `CLICOLOR_FORCE` into a buffer, which `For` (an `*os.File` char-device check) would always answer with Plain. Those examples keep their lipgloss decision and pass `highlight.Terminal` or `highlight.Plain` to the new print functions.

### Option I: zap test in an example module

**Rejected**: Tests sit with the code they test (`conventions/testing-and-mocking.md`), and the code is `errors`' `fmt.Formatter`.

## Chosen approach — evidence

- The report layout today is a single function over `Diagnostic` fields (`diagnostics.go:36-215`). It needs a lexer only to label source lines (`diagnostics.go:123`). Everything else (severity, summary, location, gutter, detail) is plain string building. So a token-free layout, with segments for the source lines and their fault byte range, can live in an internal package that `errors` imports (internal/xcl + textseg + wordwrap, all existing deps: `go.mod:8,20`). The root package then turns the segments into tokens by lexing each source line, as it does now (`diagnostics.go:117-135`, `overlay` at :190).
- `errors` already imports `go-wordwrap` (knowledge `shared-errors-package.md`). Wrapping at print time uses the same `wordwrap.WrapString` the layout uses today (`diagnostics.go:91-96`).
- `Diagnostic` is built from source already in memory: `errors.NewDiagnostic(d, sources)` → `linesOf(src, covered)` (`errors/diagnostic.go:72-117`). Widening `first`/`last` by two lines, clipped to `[1, len(lines)]`, restores context without touching disk.
- `ConfigError.AppendError` flattens `Diagnostics` (`errors/config_error.go:21-31`), but also appends non-diagnostic errors (`internal/parser/destroy.go:104`). `Unwrap() []error` exists on both `ConfigError` and `Diagnostics` (`errors/config_error.go:37`, `errors/diagnostic.go:213`). A tree walk over `Unwrap() error` and `Unwrap() []error` finds every diagnostic through `%w` wraps and `errors.Join`.
- `EncodeOption` is `func(*encodeOptions)` (`encode.go:40`). A method on that func type can satisfy an unexported-method `PrintOption` interface, so encode and print options mix in one call, as the design asks.
- The tokenizer already guarantees full byte coverage, no empty pieces, and different neighbouring scopes (`highlight/tokenize.go:100-118`). Switching the label from scope string to `Kind` keeps those guarantees. Every scope constant (`highlight/highlight.go:9-117`) maps 1:1 to a design kind, with `ScopeTemplateInterpolation` → `TemplateDirective` (1003). So `Kind.Scope()` reproduces every string the parity tests assert (`highlight/tokenize_test.go`, 58 tests).
- The VS Code theme matcher (`highlight/theme.go:42-130`, selector prefix, specificity, later wins) and the JSONC stripper (`highlight/jsonc.go`) are kept. Conversion resolves each `Kind.Scope()` through them into a style string.
- The diff layout labels with `add(scope, text)` (`diff/render.go:112-150`, `scopeFor` at :353). Swapping scope strings for kinds is mechanical. `Diff.Pieces` becomes `Diff.Tokens`.
- `coreservice/lookups.go:190-197` colours encode output with `highlight.ANSI`, and three tests assert on it (`coreservice/lookups_test.go:167,424,436`). It must move to `highlight.Terminal` so `highlight=true` keeps working until the engine spec reserves the field.
- The prettylog test asserts the SGR bytes `\x1b[1;35m` for block type and `\x1b[34m` for attribute (`example/prettylog/prettylog_test.go:494-497`). The default theme keeps `bold magenta` and `blue` for these, and the Terminal formatter writes attributes before colours.
- zap's `zap.Error` adds `<key>Verbose` from `fmt.Sprintf("%+v", err)` for any error implementing `fmt.Formatter` (zap `zapcore/error.go`, `encodeError`). Implementing `Format` on `*ConfigError` and `*Diagnostic` is enough.
- Changelog convention: one `## <spec name>` section at the top of `CHANGELOG.md`, prose, then a `**Breaking:**` list (`CHANGELOG.md:1-20`). Release notes live there.

## Files examined

- highlight/doc.go:1-26 — package doc describes Pieces/Renderer and is to be rewritten
- highlight/highlight.go:9-153 — Scope* constants, Renderer, RendererFunc, PieceFunc, Text: all removed
- highlight/ansi.go:1-167 — ANSIRenderer, ANSIOption, WithTheme, WithThemeFile, NewANSI, sgrParameters: removed (the SGR logic is reused in Terminal)
- highlight/theme.go:1-373 — TextMate rule matching, default theme (16 colours), parseTheme/parseColour/parseFontStyle: rework into LoadVSCodeTheme
- highlight/jsonc.go:1-85 — JSONC stripping: kept
- highlight/tokenize.go:1-786 — Piece and Pieces over hclsyntax.LexConfig: becomes Token, Tokenize, with Kind labels
- highlight/tokenize_test.go — 58 scope-assertion tests: the xcl-vscode parity tests, kept by asserting through Kind.Scope()
- highlight/ansi_test.go (38 tests), theme_test.go (46), highlight_test.go (10), jsonc_test.go (8) — rewritten for the new API
- highlight/testdata/{sample.xcl,references.xcl,reference-theme.json} — existing fixtures, reusable
- diagnostics.go:1-215 — DiagnosticPieces report layout: removed, logic moves into internal/report plus root ErrorTokens
- diagnostics_test.go:53-188 — 16 layout tests, ported to ErrorTokens/PrintError
- errors/diagnostic.go:1-238 — Diagnostic, SourceLines, NewDiagnostic, linesOf, Error(): gains context lines and Format
- errors/config_error.go:1-50 — ConfigError: gains Format
- errors/highlight_errors.go:1-50 — ErrInvalidTheme/InvalidThemeError: kept, doc mentions NewANSI and must be updated
- errors/diagnostic_test.go, internal/parser/validate_test.go, internal/parser/parse_test.go, viper/xclviper_test.go — assert Source lines, and must expect the ±2 context lines
- encode.go:28-130 — EncodeOption func type, EncodeEntity doc that mentions highlight.ANSI
- encode_highlight_test.go — 8 tests over Pieces/ANSI, rewritten over EntityTokens/PrintEntity
- config.go:114-117,184 — re-exports ErrInvalidTheme/InvalidThemeError, doc mentions NewANSI
- diff/render.go:1-361 — Pieces layout with scopes, becomes Tokens with kinds
- diff/pieces_test.go, diff/render_test.go, diff/example_test.go (ExampleDiff_Pieces) — renamed/ported
- config_singleton_lifecycle_test.go:69, sensitive_leak_test.go:635-672, internal/parser/decide_test.go:376, internal/parser/diff_unknown_test.go:583,609, viper/xclviper_test.go:154, viper/xclviper_directory_test.go:147 — callers of the removed API in tests
- coreservice/lookups.go:134,151,190-197 — highlight=true uses highlight.ANSI
- example/configonly/main.go:44, example/plugin/main.go (`error: %s`), example/remoteplugin/main.go:94 — print errors as one line
- example/plugin/inspect.go, example/plugin/plan.go — lipgloss colour decision + highlight.Renderer
- example/prettylog/prettylog.go:83-133 — lipgloss decision, renders highlight.Pieces with ANSI, indents afterwards
- example/*/smoke_test.go — assert `error:` on stderr for a bad configuration
- docs/overview.md:128-130, docs/README.md:53 — describe DiagnosticPieces and Renderer
- CHANGELOG.md:1-20 — changelog section format
- go.mod:1-24 — no zap, go-wordwrap and textseg already present
- xcl-vscode:test/tokenize.test.js — the extension's own grammar tests, independent of xcl's Go code
- xcl-website:src/pages/configuration-text.mdx:326-479 — Highlighting section (Pieces, ANSI, NewANSI, WithThemeFile, Renderer, PieceFunc, Scope*)
- xcl-website:src/pages/problems.mdx:103-144 — "Showing problems to a person" with DiagnosticPieces, plus a CtaBanner
- xcl-website:src/pages/diff.mdx:27-70,195-310 — d.Pieces(), highlight.Text, Colour section with NewANSI and Scope* diff scopes
- xcl-website:src/pages/events.mdx:27-84,332-443 — SlogHandler, prettylog description mentioning highlight.Pieces/ANSI
- xcl-website:src/pages/examples/viper.mdx:185-233 — DiagnosticPieces + highlight.Text/ANSI
- xcl-website:src/pages/examples/plugins.mdx:1264-1322,1420-1484 — plan/inspect with Pieces/ANSI
- xcl-website:src/pages/sensitive-values.mdx:198-229 — mentions Diff.Pieces
- xcl-website:src/components/Nav.astro — hard-coded nav; Guides holds Configuration text, Problems, Diffs
- xcl-website:src/components/CodeTabs.astro — Go/Python slot tabs
- xcl-website:src/styles/global.css — site stylesheet (Tailwind v4 tokens, expressive-code rules)
- xcl-python:Makefile:5, .github/workflows/test.yml:75-81 — Python CI checks out xcl at a ref (XCL_SRC=../xcl), so reference files in the xcl repo are reachable by path

## External references

- design `design/printing-and-highlighting.md` (source `design`) — binding: kinds, style syntax, formatter shape, print API, removals, class names
- go/scanner.PrintError — print every error in a list to a writer, other errors as Error()
- CUE cuelang.org/go/cue/errors Print/Details — writer-based detailed error printing
- HCL hcl.NewDiagnosticTextWriter — colour as an explicit caller choice, width-wrapped detail
- alecthomas/chroma — tokens/style/formatter separation and style-string syntax
- go.uber.org/zap zapcore encodeError — `errorVerbose` from `%+v` for fmt.Formatter errors
- no-color.org — NO_COLOR convention, used by highlight.For
- ECMA-48 SGR codes — terminal attributes, 30–37/90–97 basic colours, 38;2 and 48;2 24-bit colours

## Prior plans / specs consulted

- spec 20261023075136-42c51a7d-printing-tokens-over-the-engine (sibling) — the engine/proto work (report on every Error, tokens over the wire, ConvertTheme RPC, reserving `highlight`) is out of scope here. This plan must leave Go functions it can call: ErrorTokens, EntityTokens, Diff.Tokens, LoadVSCodeTheme.
- spec 20261023075302-bad71237-printing-and-highlighting-python (sibling) — Python's formats are checked against "the shared reference outputs", so reference inputs must be token JSON and outputs byte-exact files in the xcl repo.
- knowledge xcl:conventions/shared-errors-package.md — drives the internal/report split
- knowledge xcl:conventions/testing-and-mocking.md — no table tests, positive and negative tests apart, tests sit with their code, no repo-file-inspecting tests
- knowledge xcl:gotchas/example-modules-cannot-import-internal.md — examples use only public packages
- knowledge xcl:gotchas/hcl-halts-on-malformed-block-header.md — the two-problem fixture must use recoverable faults (two resource blocks missing name labels), not two bad headers
- knowledge xcl:learnings/releases-tagged-by-hand.md — check tags. The latest is v0.11.1, and cutting a release is not part of this plan.
- knowledge xcl-website:gotchas/no-redirects-on-page-removal.md — prefer adding sections to existing pages over renaming or removing them
- knowledge xcl-website:gotchas/guides-dropdown-label-opens-first-child.md — leave the Guides order alone

## Open assumptions

- Verified during planning, so not open: `Config.Validate` returns `*errors.ConfigError` unwrapped for configuration problems. An unsupported nested block inside an entity yields `Summary "Unsupported block type"`, `Entity "variable.name"` and `Field "timeout[0]"`. This was checked by a probe program against the current code, and `config_validate_test.go:110` type-asserts the same thing.
- The current HEAD encoder and diff renderer output equals the "current release" (v0.11.1) output for the captured fixtures. The capture is taken from HEAD before any change.
- The xcl-python test suite does not assert exact ANSI bytes from `encode_entity(highlight=True)` that the new default theme would change. If it does, the engine-bump PR in xcl-python surfaces it, and the Python spec owns that path.

## Drafting assumptions

### Chosen direction: the design as written, with the report laid out in internal/report (architecture)
- **Decision**: Build the design's tokens/themes/formatters/print API in xcl. Lay out the error report once, as token-free segments in `internal/report`, shared by `errors`' `%+v` and the root's `ErrorTokens`. Wrap detail only when printing. Restore context in `errors.NewDiagnostic`. Hold the formatters to token-JSON reference outputs in `highlight/testdata/reference/`.
- **Key design decisions**: (1) Breaking the errors/highlight cycle with internal/report rather than moving the theme errors. (2) `PrintError` walks the full error tree and prints non-diagnostic leaves as single lines. (3) `ErrorTokens` returns a message token for an error with no diagnostics, so `PrintError` is exactly "format ErrorTokens". (4) Examples keep their own colour decision. (5) coreservice `highlight=true` maps to Terminal until the engine spec reserves it.
- **Rejected**: internal type aliases; an init() hook; a duplicate plain layout (see research.md alternatives).

### Conventions selected (architecture)
- **Decision**: Apply shared-errors-package, testing-and-mocking, dependencies, code-style, project-structure, shared-test-helpers, and the gotchas example-modules-cannot-import-internal, hcl-halts-on-malformed-block-header and website no-redirects. Drop the rest as irrelevant.
- **Rationale**: Only these bear on a printing and highlighting library change with docs.
- **Rejected**: Listing every always-applied convention (noise).

### Report layout lives in a token-free internal package (discovery)
- **Decision**: Lay out the report once, in a new `internal/report` package, as segments: severity, summary, location, gutter, a source line with its fault byte range, detail, and text. `errors` imports it to write the plain report for `%+v`. The root package turns the segments into `highlight.Token`s by lexing each source line.
- **Rationale**: `highlight` imports `errors`, so `errors` cannot import `highlight` without a cycle. The `shared-errors-package` knowledge entry keeps theme errors in `errors` and its import list thin. This split keeps one layout and breaks no rule.
- **Rejected**: `errors` → `highlight` (cycle, and moving theme errors out contradicts knowledge); internal aliases (poor godoc); an init() hook (hidden coupling); a duplicate plain layout (drift).

### Reference outputs are token JSON inputs plus byte-exact outputs under highlight/testdata/reference (discovery)
- **Decision**: Publish `highlight/testdata/reference/` holding three token-stream inputs (an error report, a resource, a change summary) as JSON, each one's output for every built-in format with the default theme, the CSS from the default theme, and `classes.txt`. An external `highlight_test` test builds the inputs from real code, compares them, and checks every formatter byte for byte. A `-update` flag rewrites the files.
- **Rationale**: Other languages can check their formatters with no engine, Python CI already checks xcl out at a ref, and the test stays in the directory whose files it owns.
- **Rejected**: configuration-text inputs (each language would need the lexer); root-package golden tests (reach into another directory).

### Examples keep their own colour decision (discovery)
- **Decision**: Examples that decide colour with lipgloss keep doing so, choosing `highlight.Terminal` or `highlight.Plain` for `xcl.PrintError`, `PrintEntity` and `PrintDiff`. Examples with no decision of their own use `highlight.For(os.Stderr)`.
- **Rationale**: The prettylog tests force colour into a buffer through `CLICOLOR_FORCE`, which `highlight.For` cannot see, and the logger and the configuration should agree on colour.
- **Rejected**: `highlight.For` everywhere (breaks the forced-colour tests and disagrees with charmlog).

### coreservice highlight=true moves to the Terminal formatter (discovery)
- **Decision**: `coreservice` keeps honouring `highlight=true` by formatting `Tokenize(text)` with `highlight.Terminal` and the default theme.
- **Rationale**: Removing `highlight.ANSI` must not break the engine. Reserving the field and treating it as plain belongs to the sibling engine spec.
- **Rejected**: Reserving the field here (it is that spec's scope).

### zap is a test-only dependency of the root module (discovery)
- **Decision**: Add `go.uber.org/zap` (pinned) for one test, in the `errors` package's external tests. It checks that `zap.Error(err)` carries the plain report in `errorVerbose`.
- **Rationale**: The AC names zap, and tests sit with the code they test. The constraint forbids new dependencies *for highlighting*, and this dependency is test-only, for logging. Go module graph pruning keeps it off importers' builds.
- **Rejected**: A zap test in an example module (it would test root code away from that code).

### Theme validation errors use a new ErrInvalidStyle/InvalidStyleError in errors (data_structures)
- **Decision**: `Theme.Validate` returns `errors.Join` of one `*errors.InvalidStyleError{Kind, Style, Word}` per unknown word. Each wraps the new sentinel `ErrInvalidStyle`, and both are re-exported from the root.
- **Rationale**: This follows `shared-errors-package` (sentinel plus pointer-receiver detail in `errors`). Kind is carried as its name, so `errors` does not import `highlight`.
- **Rejected**: Reusing `ErrInvalidTheme` (a VS Code theme file problem, not a style word); plain `fmt.Errorf` (unmatchable).

### Style resolution is whole-style, and the default theme styles parents (data_structures)
- **Decision**: A kind's style replaces its parent's entirely, with no per-attribute merge. `DefaultTheme` sets parents (Block, Label, Attribute, String, Number, Constant, Reference, Comment, Inserted/Deleted/Changed, Error/Warning, Summary, Gutter) and only those children that must differ (TypeLabel, StringEscape). Overriding `Comment` therefore recolours every comment, as the AC requires.
- **Rationale**: The design says "the kind, then its parent, then Text". The comment AC needs the default to leave comment children unset.
- **Rejected**: Per-attribute merge (cannot express "not bold" in a child).

### Style vocabulary is exactly the design's (data_structures)
- **Decision**: The style words are `bold italic underline dim`, `#rrggbb`, `bg:#rrggbb`, the eight named colours and their `bright-` forms. Today's `strikethrough` (VS Code fontStyle) is dropped on import, and `#rgb` is not accepted.
- **Rationale**: Every language must parse identically. The design fixes the syntax.
- **Rejected**: Adding `strikethrough` or short hex (outside the design's syntax, and would need a design change).

### Exported Style/Colour/ParseStyle and Theme.Style for custom formatters (data_structures)
- **Decision**: Export the parsed `Style` and `Colour` types, `ParseStyle` and `Theme.Style(kind)`, so user formatters resolve styles exactly as the built-ins do.
- **Rationale**: The spec requires custom formats that are usable anywhere. Without resolution helpers, every custom formatter would reimplement parsing.
- **Rejected**: Keeping parsing private (custom formatters would diverge).

### LoadVSCodeTheme returns only what the VS Code theme styles (data_structures)
- **Decision**: `LoadVSCodeTheme(r)` returns a `Theme` holding the kinds whose scope a rule matches, as `"[bold] [italic] [underline] #rrggbb"`. Callers wanting xcl's defaults for the rest use `DefaultTheme.With(loaded)`.
- **Rationale**: A converted theme is plain data and composes with `With`. The engine spec's `ConvertTheme` returns the same map.
- **Rejected**: Auto-merging with the default (hides what the editor theme did, and mixes 16-colour and 24-bit palettes silently).

### Unknown kinds render through their parent (data_structures)
- **Decision**: For a kind not in the table, `String()` is `Kind(<n>)`, `Class()` is its parent's class (or "" when the parent is unknown too, written like Text), and `Scope()` is its parent's.
- **Rationale**: The design's forward-compatibility rule ("an unknown kind still falls back to its parent").
- **Rejected**: Panicking or inventing class names.

### Golden files are regenerated with a -update test flag (implementation_detail)
- **Decision**: The reference-output test takes a standard `-update` flag that rewrites `highlight/testdata/reference/`. Without it, the test compares.
- **Rationale**: This is the idiomatic Go golden pattern. The repo has no existing golden-update convention to follow.
- **Rejected**: A separate generator command (more surface for one test's data).

### zap pinned at v1.27.0 (dependencies)
- **Decision**: Pin `go.uber.org/zap v1.27.0` as a test-only requirement.
- **Rationale**: It is a stable, widely used release well over two weeks old with the `errorVerbose` behaviour. The implementer may pick the newest patch at implementation time if `go mod tidy` prefers it.
- **Rejected**: An unpinned or `latest` version (breaks the pinning convention).

### CSS validity checked with a test-local tokenizer (testing_approach)
- **Decision**: The "CSS parser accepts it" AC is checked by a small standard-library rule parser in the `highlight` tests: balanced braces, `selector { prop: value; }` rules, every selector starting `.xcl`. It does not use a third-party CSS library.
- **Rationale**: Prefer the standard library, and no new dependency for highlighting.
- **Rejected**: Adding a CSS parser module (a new dependency for one assertion).

### For's terminal case tested only where a character device is available (testing_approach)
- **Decision**: The Terminal branch of `highlight.For` is tested by opening `/dev/tty` (or `/dev/null`, which is a char device on Unix) and skips where none exists. The NO_COLOR and non-file branches are always tested.
- **Rationale**: CI has no TTY. `/dev/null` is a character device, so it exercises the `ModeCharDevice` check deterministically on Unix.
- **Rejected**: Faking `os.File` (cannot be done), or a pty dependency.

### Old API removed together with the new API landing (milestones)
- **Decision**: Milestone 2 introduces the new `highlight`/print API and deletes the old one in the same milestone, migrating every in-repo caller, including the examples' resource and diff printing, so they compile. Milestone 4 then switches the examples' error printing to reports.
- **Rationale**: The new `highlight.Text` (a Kind) and `xcl.WithTheme` clash with old names, and the spec forbids deprecation, so the two APIs cannot coexist across milestones.
- **Rejected**: A temporary token-returning `DiagnosticPieces` (churn for one milestone).

### Milestone 2 tasks pass through a non-compiling module (tasks)
- **Decision**: "Rebuild highlighting" deletes the old API, so packages outside `highlight`/`errors` do not compile until "Move every remaining caller" lands. Each task verifies its own packages, and the milestone's validation is the full suite.
- **Rationale**: The design's `highlight.Text` Kind clashes with the old `highlight.Text` renderer, and the spec forbids deprecation shims. Doing all of Milestone 2 as one task would be too large for one agent.
- **Rejected**: Temporary compatibility shims (churn, and an API removal the spec says is not deprecation); one giant task.

### Walk keeps a non-problem wrapper's own message (tasks)
- **Decision**: `report.Walk` descends through `Unwrap() error` only when the subtree holds a diagnostic. A wrapper around plain errors stays one leaf with its full message, and a wrapper around diagnostics contributes only the reports (its prefix is dropped).
- **Rationale**: This matches `go/scanner.PrintError` printing list entries, and the AC that a wrapped two-problem error prints both reports. Plain errors keep their context.
- **Rejected**: Printing wrapper prefixes above reports (the design and spec ask for reports only).

### configonly and remoteplugin use highlight.For for errors; plugin uses its lipgloss check (tasks)
- **Decision**: Examples with an existing lipgloss decision (plugin, prettylog) reuse it. Others use `highlight.For(os.Stderr)`.
- **Rationale**: Each example stays consistent with how it already colours output.
- **Rejected**: Adding lipgloss to examples that do not use it.

### Website Python tabs left to the Python spec (tasks)
- **Decision**: Website edits in this plan rewrite the Go content and leave existing Python statements about unavailable features in place.
- **Rationale**: The sibling Python spec explicitly owns removing "the report is unavailable from Python" and adding Python examples. Editing them here would collide.
- **Rejected**: Writing Python examples now (that API does not exist yet).

### Reference report input built from NewDiagnostic, not an Apply (tasks)
- **Decision**: The reference report tokens come from `xcl.ErrorTokens` over diagnostics made with `errors.NewDiagnostic` on fixed source, and the entity input from `Tokenize` of a fixture file.
- **Rationale**: These are deterministic and need no config, registry or plugins, so the reference test stays small and stable.
- **Rejected**: Applying a config (filesystem paths would leak into the reference output).

## Rehydration cues

- `spektacular spec file read 20261023074926-eaa51527-printing-and-highlighting-core`
- `spektacular design read --data '{"source":"design","path":"printing-and-highlighting.md"}'`
- `spektacular knowledge read --data '{"tier":"repo","name":"xcl","path":"conventions/shared-errors-package.md"}'`
- Re-read: /home/nicj/code/github.com/jumppad-labs/xcl/diagnostics.go, errors/diagnostic.go, highlight/{tokenize,theme,ansi,highlight}.go, diff/render.go, coreservice/lookups.go:185-197, example/prettylog/prettylog.go
- `grep -rn 'highlight\.\|DiagnosticPieces\|\.Pieces()' --include='*.go' /home/nicj/code/github.com/jumppad-labs/xcl` to find every caller of the removed API
- Website pages: /home/nicj/code/github.com/jumppad-labs/xcl-website/src/pages/{configuration-text,problems,diff,events,sensitive-values}.mdx, examples/{viper,plugins}.mdx
