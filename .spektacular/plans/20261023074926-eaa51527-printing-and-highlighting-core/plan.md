---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Plan: 20261023074926-eaa51527-printing-and-highlighting-core

<!-- Metadata -->
<!-- Created: 2026-10-23T08:27:41Z -->
<!-- Commit: ae3b7b638d0a1717609dd147d9806ab34f3a63ef -->
<!-- Branch: f-python-prettylogger -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

xcl's Go library gains one-call printing of any error, resource or change summary to a writer, as plain text, terminal colour, class-marked HTML a website can restyle, inline-styled HTML or SVG. It is styled by short, extendable themes (or an imported VS Code theme) and restores the two lines of source either side of a configuration fault. It replaces a multi-step, editor-shaped highlighting API that printed errors as one line and could not restyle output or produce whole documents. The old API is removed, and the examples, release notes and website are moved over. Go application authors and website maintainers benefit directly, and every other xcl language benefits from the reference outputs this plan publishes, which their printing is checked against.

## Conventions

- **Shared error types live in `errors`; keep its imports thin** — it forces the token-free `internal/report` split and keeps `InvalidThemeError` and any theme-validation error in `errors`, re-exported from the root.
- **Testing & Mocking (testify `require`, no table tests, positive and negative tests apart, tests beside their source, no tests that inspect repository files)** — every formatter, theme, print function and example gets one-behaviour tests in its own directory; the reference-output test lives in `highlight`; the changelog, docs and website are checked by review, not by tests.
- **Dependencies (prefer the standard library, document and pin any third-party dependency)** — the formatters, CSS, SVG and `For` use only the standard library; the one new module (zap, test-only) is pinned and justified.
- **Code style (gofmt/vet, `any`, descriptive names, standard-library idioms)** — the new API follows `go/scanner.PrintError`, the `io.Writer`-first signatures and the functional-option style already used by `EncodeOption`.
- **Project structure (public packages at module top level, private code in `/internal`)** — `highlight` stays public at the top level; the report layout goes in `internal/report`.
- **Shared test helpers live in `internal/testutil`; only `_test.go` files import it; it must not import the root package** — any helper shared by the root and `highlight` tests (such as stripping SGR codes) goes there; per-package helpers stay local.
- **Example modules cannot import internal packages** (gotcha) — the examples use only `xcl`, `highlight` and their own helpers to print reports.
- **HCL halts on a malformed block header** (gotcha) — the two-problem fixture uses two recoverable faults so two reports really appear.
- **Removing a website page breaks its URL** (xcl-website gotcha) — the docs are rewritten inside existing pages; none are renamed or removed.

Deliberately dropped: database/external services, patterns & architecture (no services or handlers here), config parser options builder, hclsyntax.Variables references, graph-parent ordering, real-apply test state, provider Changed/Update testing — none bear on printing.

## Architecture & Design Decisions

The work follows the design `design/printing-and-highlighting.md` (source `design`) as written: tokens, themes and formatters stay apart. Print functions do the common case in one call, and the error types hook into Go's own `%+v`. Nearly all of it lands in the xcl repo (`/home/nicj/code/github.com/jumppad-labs/xcl`). The `highlight` package is rebuilt around `Kind` (numbered in blocks of 100, with `Parent()` by arithmetic and `Scope()` for TextMate), `Token{Kind, Text}`, `Tokenize`, `Theme` (a `map[Kind]string` of style strings with `With`, `Validate` and `Style` resolution: the kind, then its parent, then `Text`), `DefaultTheme`, the `Formatter`/`FormatterFunc` interface, five built-in formatters (`Plain`, `Terminal`, `HTML`, `HTMLInline`, `SVG`), `WriteCSS`, `For`, and `LoadVSCodeTheme`. The last reuses today's TextMate selector matcher and JSONC stripper to turn each kind's scope into a style string. The root `xcl` package gains `PrintError`, `PrintEntity`, `PrintDiff`, the `PrintOption`s `WithFormatter`, `WithTheme` and `WithWidth`, and the extender functions `ErrorTokens` and `EntityTokens`. `diff.(*Diff).Tokens` replaces `Pieces`. The whole previous API (`Piece`, `Pieces`, `Renderer`, `RendererFunc`, `PieceFunc`, `Text`, `ANSI`, `ANSIRenderer`, `NewANSI`, `ANSIOption`, `WithTheme(io.Reader)`, `WithThemeFile`, the `Scope*` constants, `xcl.DiagnosticPieces`, `diff.(*Diff).Pieces`) is deleted, not deprecated. Each removal gets a one-line replacement in the `CHANGELOG.md` Breaking list. No chroma, no cgo, no new runtime dependency (`conventions/dependencies.md`).

The one structural decision the design leaves open is where the error report is laid out. The design asks for `*errors.Diagnostic` and `*errors.ConfigError` to implement `fmt.Formatter`, with `%+v` printing the plain report. But `highlight` imports `errors` for `InvalidThemeError`, and the knowledge entry `conventions/shared-errors-package.md` keeps that error in `errors` and keeps `errors`' imports thin. So `errors` cannot reach the lexer through `highlight`. The report is therefore laid out once, in a new token-free package `internal/report`. It produces segments (severity, summary, location, gutter, one source line with its fault's byte range, detail, plain text) from a diagnostic's fields, and also holds the error-tree walk. `errors` uses it to write the plain report for `%+v`, joining the segments' text and wrapping the detail at 80 columns. The root package's `ErrorTokens` uses the same segments: it lexes each source line with `highlight.Tokenize`, overlays `Error` or `Warning` on the fault, and labels the rest with the new `Report` kinds. `%+v` and plain `PrintError` therefore come from one layout and agree by construction. Detail is never wrapped in the layout or in `ErrorTokens`; it is wrapped when printing (`WithWidth`, default 80, 0 never), so the unwrapped report stays available to the engine spec. Restoring context is a change to `errors.NewDiagnostic`'s `linesOf`: it widens the captured range by two lines either side, clipped to the file, from the source bytes already in memory.

`PrintError` walks the whole error tree through `Unwrap() error` and `Unwrap() []error`. Each `*Diagnostic` becomes a report, a non-diagnostic leaf next to diagnostics becomes its one-line message, and an error holding no diagnostic is printed as `err.Error()` and a newline, as `go/scanner.PrintError` does. `EncodeOption` gains an unexported method that makes it a `PrintOption`, so `PrintEntity(w, entity, xcl.IncludeComputed(), xcl.WithFormatter(...))` mixes both. Colour is never decided by the library: `highlight.For(w)` is opt-in (an `*os.File` that is a character device, and `NO_COLOR` unset), and examples that already decide colour through lipgloss keep doing so (`example/prettylog/prettylog_test.go:484` forces colour into a buffer). The HTML formatter writes the class contract (`<pre class="xcl">`, `xcl-<kebab kind>` plus the parent class, no span for `Text`). The HTML formatter, `HTMLInline`, `SVG`, `Terminal` and `WriteCSS` are held byte for byte to reference outputs published in `highlight/testdata/reference/`. These are three token-stream inputs as JSON (an error report, a resource, a change summary), each one's output for every format, the default-theme stylesheet and the class list. Other languages check their formatters against them without the engine. `coreservice`'s legacy `highlight=true` switches to the `Terminal` formatter so the engine keeps working until the sibling engine spec reserves the field. `events.SlogHandler` is untouched; zap picks the report up through `errorVerbose` once the error types implement `fmt.Formatter`.

Outside the xcl repo, the xcl-website repo (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) has its existing pages rewritten in place, with no pages removed or renamed (`gotchas/no-redirects-on-page-removal.md`): `configuration-text.mdx`, `problems.mdx`, `diff.mdx`, `events.mdx`, `sensitive-values.mdx`, `examples/viper.mdx` and `examples/plugins.mdx`. They cover printing, formats, themes, class names, stylesheet generation, custom formats and logging, with Go examples. xcl-vscode is not changed; its grammar stays the source of the scopes that `Kind.Scope()` reproduces, and xcl's own `highlight/tokenize_test.go` parity assertions keep their exact scope strings by labelling through `Kind.Scope()`. This direction beats moving the tokenizer behind internal type aliases (public docs would show internal aliases), an `init()` hook from the root package (hidden coupling), and a second plain-text layout in `errors` (drift). The evidence is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Token kinds (`highlight`, new)**: owns `Kind`, the numbered kinds from the design, each kind's parent by arithmetic, its kebab-case class name and its TextMate scope. Every other component labels text with these kinds. The scope mapping exists only for the VS Code importer and the xcl-vscode parity tests.
- **Tokenizer (`highlight`, changed)**: today's lexer-driven labeller of configuration text, unchanged in behaviour, emitting `Token{Kind, Text}` instead of scope-labelled pieces through `Tokenize`. The report builder, the diff layout, `EntityTokens` and extenders all lex configuration text through it, and nothing else lexes.
- **Themes and styles (`highlight`, new)**: owns `Theme` as plain data (kind → style string), style-string parsing into attributes and colours, the kind → parent → `Text` resolution, `With` for extending, `Validate` for reporting unknown words, and `DefaultTheme`, which keeps today's 16-colour look. Formatters and the CSS writer consume resolved styles. Parsing never fails, so formatters never fail on a style.
- **VS Code theme importer (`highlight`, reworked from today's theme loader)**: owns reading a VS Code colour theme (JSONC, TextMate selector specificity) and converting it to a `Theme`, by resolving each kind's scope. It returns the existing `InvalidThemeError` for an unusable theme. The sibling engine spec reuses it for its theme-conversion call.
- **Formatters (`highlight`, new)**: the `Formatter` interface and `FormatterFunc`, plus the built-in `Plain`, `Terminal`, `HTML`, `HTMLInline` and `SVG`. Each is given the whole token stream, a theme and a writer. `Terminal` writes SGR codes with a reset before every newline. `HTML` writes the documented class contract and ignores the theme. `HTMLInline` writes the same markup with inline styles. `SVG` writes a standalone, self-coloured document. User-written formatters plug in at the same seam.
- **Stylesheet writer and colour chooser (`highlight`, new)**: `WriteCSS` turns any theme into a `.xcl`-scoped stylesheet, parents before children, with named colours mapped to a fixed hex palette. `For` picks `Terminal` or `Plain` from a destination on request. Both are standard library only.
- **Report layout (`internal/report`, new)**: owns laying a diagnostic out as token-free segments (severity, summary, location, gutter, source line with fault range, detail, text), wrapping detail text, and walking an error tree for diagnostics and other leaf errors. It is the single source of the report's wording. Both the error types and the root report builder use it, which breaks the `errors` ↔ `highlight` cycle.
- **Error types (`errors`, changed)**: `Diagnostic` captures two lines of context either side of the fault from source already in memory. `Diagnostic` and `ConfigError` implement `fmt.Formatter`: one line for `%v`/`%s`, the plain report wrapped at 80 for `%+v`, built from the report layout. That is what structured loggers such as zap pick up. Theme errors stay here.
- **Report builder (`xcl` root, replaces `DiagnosticPieces`)**: `ErrorTokens` turns the report layout's segments into tokens. It lexes source lines through the tokenizer, overlays `Error`/`Warning` on the fault, and labels summary, location, gutter and detail with the `Report` kinds. Errors without diagnostics become their message.
- **Print API (`xcl` root, new)**: `PrintError`, `PrintEntity`, `PrintDiff` and `EntityTokens`, configured by `PrintOption`s (`WithFormatter`, `WithTheme`, `WithWidth`), with `EncodeOption` also acting as a `PrintOption`. It owns the defaults (Plain, the default theme, width 80) and print-time detail wrapping. It composes the report builder, the encoder, the diff layout and a formatter.
- **Diff layout (`diff`, changed)**: lays a change summary out as tokens through `Tokens()` instead of `Pieces()`, with markers as `Inserted`/`Deleted`/`Changed`, comments as `HashComment`, and headers and values lexed by the tokenizer. The text is unchanged.
- **Encoder (`xcl` root, unchanged in behaviour)**: still produces configuration text. `EntityTokens` and `PrintEntity` lex its output. Its doc stops pointing at the removed renderer.
- **Core service colouring (`coreservice`, changed)**: the legacy `highlight=true` on the two encode calls formats through the `Terminal` formatter and default theme instead of the removed ANSI renderer. The protocol is untouched, because changing it belongs to the sibling engine spec.
- **Reference outputs (`highlight` test data, new)**: the published, byte-exact contract for every built-in format, the default stylesheet and the HTML class list, over three fixed token-stream inputs (an error report, a resource, a change summary). The Go formatters are tested against it here, and other languages in their own work.
- **Examples (`example/*`, changed)**: the configuration-only, plugin and remote-plugin programs print errors through `PrintError`, so a bad configuration shows a report. The plugin `inspect`/`plan` commands and the shared `prettylog` receiver print resources and change summaries through `PrintEntity`/`PrintDiff`, keeping their own colour decision.
- **Release notes and repo docs (`CHANGELOG.md`, `docs/`, changed)**: the changelog entry describes the new printing and lists a one-line replacement for every removed function. The repo's overview and package table describe the new `highlight` and print API.
- **Website (xcl-website, changed)**: the existing guide and example pages document printing errors, resources and change summaries, output formats, themes, class names and stylesheet generation, custom formats and logging integration with Go examples, and stop documenting removed functions.

## Data Structures & Interfaces

All public shapes come from the design and are repeated here as the contracts between components. Per-field detail (default styles, the colour palette, escaping, SVG geometry, scope strings) is in the plan's context document.

**`highlight.Kind` and `highlight.Token`**: the label vocabulary, numbered exactly as the design's table so the engine spec's proto enum and other languages agree. An unknown kind still resolves through its parent.

```go
type Kind int
const ( Text Kind = 0; Block = 100; BlockType = 101; NestedBlock = 102; /* … the design's table … */
        Report = 1400; Summary = 1401; Location = 1402; Gutter = 1403; Detail = 1404 )
func (k Kind) Parent() Kind   // k / 100 * 100
func (k Kind) String() string // "BlockType"
func (k Kind) Class() string  // "xcl-block-type"
func (k Kind) Scope() string  // "storage.type.xcl"; used only by LoadVSCodeTheme and the parity tests

type Token struct { Kind Kind; Text string }
func Tokenize(src []byte) []Token // replaces Pieces; joined text == src, no empty tokens, neighbours differ
```

**`highlight.Theme` and `highlight.Style`**: a theme is plain data, kind to style string. A style is resolved whole: the kind's own style, else its parent's, else `Text`'s. Parsing never fails: unknown words are skipped, and only `Validate` reports them.

```go
type Theme map[Kind]string
var DefaultTheme Theme
func (t Theme) With(overrides Theme) Theme // new map, overrides win
func (t Theme) Validate() error            // nil, or errors.Join of *errors.InvalidStyleError, one per unknown word
func (t Theme) Style(k Kind) Style         // resolved and parsed

type Style struct { Bold, Italic, Underline, Dim bool; Foreground, Background Colour }
type Colour string                         // "", a named colour ("red", "bright-red") or "#rrggbb"
func (c Colour) Hex() string               // named colours through the fixed palette
func ParseStyle(s string) Style
func LoadVSCodeTheme(r io.Reader) (Theme, error) // *errors.InvalidThemeError (matches ErrInvalidTheme) when unusable
```

**`highlight.Formatter`**: the one output seam, given the whole stream. Built-ins and user formatters are interchangeable.

```go
type Formatter interface { Format(w io.Writer, theme Theme, tokens []Token) error }
type FormatterFunc func(w io.Writer, theme Theme, tokens []Token) error
var Plain, Terminal, HTML, HTMLInline, SVG Formatter
func WriteCSS(w io.Writer, theme Theme) error
func For(w io.Writer) Formatter // Terminal for an *os.File char device with NO_COLOR unset, else Plain
```

**Root print API (`xcl`)**: one call per output. `PrintOption` is an interface with an unexported method, so `EncodeOption` (an existing func type) can satisfy it by gaining that method.

```go
type PrintOption interface{ applyPrint(*printOptions) }
func WithFormatter(f highlight.Formatter) PrintOption // default highlight.Plain
func WithTheme(t highlight.Theme) PrintOption         // default highlight.DefaultTheme
func WithWidth(columns int) PrintOption               // detail wrap width; default 80, 0 never wraps
func (o EncodeOption) applyPrint(*printOptions)

func PrintError(w io.Writer, err error, options ...PrintOption) error
func PrintEntity(w io.Writer, entity any, options ...PrintOption) error
func PrintDiff(w io.Writer, d *diff.Diff, options ...PrintOption) error
func ErrorTokens(err error) []highlight.Token // unwrapped report; a message token for an error with no diagnostics; nil for nil
func EntityTokens(entity any, options ...EncodeOption) ([]highlight.Token, error)
var ErrInvalidStyle = xclerrors.ErrInvalidStyle; type InvalidStyleError = xclerrors.InvalidStyleError
```

**`diff.(*Diff).Tokens() []highlight.Token`**: replaces `Pieces()`. It gives the same text, labelled `Inserted`/`Deleted`/`Changed`, `HashComment`, and lexed configuration.

**`errors` changes**: `Diagnostic.Source` now holds up to two extra lines either side of the fault, with `SourceLines.Start` moving accordingly. `*Diagnostic` and `*ConfigError` implement `fmt.Formatter` (`%v`/`%s` give `Error()`, `%+v` gives the plain report wrapped at 80, `%q` gives the quoted one line). A new sentinel and detail type follow the package's convention:

```go
var ErrInvalidStyle = errors.New("invalid style")
type InvalidStyleError struct { Kind, Style, Word string } // Kind as its name, keeping errors free of highlight
func (e *InvalidStyleError) Unwrap() error // ErrInvalidStyle
```

**`internal/report` (private contract between `errors` and the root)**: the single report layout. It knows nothing of tokens.

```go
type Part int // Severity, Summary, Location, Gutter, Source, Detail, Plain
type Segment struct {
	Part       Part
	Text       string
	Warning    bool     // Severity and fault: warning rather than error
	Fault      [2]int   // Source only: byte range of the fault within Text
	HasFault   bool
}
type Problem struct { Warning bool; Summary, Detail, Entity, Field string; Subject *hcl.Range;
                      File string; Start int; Lines []string; HasSource bool }
func Layout(p Problem) []Segment            // one diagnostic, unwrapped, ends with its blank line
func Wrap(detail string, width int) string  // go-wordwrap; width 0 returns detail unchanged
func Walk(err error, isProblem func(error) bool, visit func(leaf error, problem bool)) // in order; never into a problem's Cause
func Contains(err error, isProblem func(error) bool) bool
```

**Serialization boundary: reference outputs**: `highlight/testdata/reference/` holds `<input>.tokens.json` (a JSON array of `{"kind": <int>, "text": "<string>"}`) for the inputs `report`, `entity` and `diff`. For each input it holds `<input>.plain.txt`, `<input>.terminal.txt`, `<input>.html`, `<input>.inline.html` and `<input>.svg`, all produced with `DefaultTheme`. It also holds `default.css` (from `WriteCSS(DefaultTheme)`) and `classes.txt` (the container class and every kind's class, one per line in kind order). These files are a public cross-language contract.

## Implementation Detail

**Capture before changing.** The first piece of work records the current plain output of the encoder and the change-summary renderer for a fixed set of resources and change summaries, written out as test data. The "matches the current release" acceptance tests compare against those captured bytes later, so the refactor cannot quietly change text. The captured files are produced once, from the code as it stands, before anything else in this plan lands.

**One label vocabulary, swapped in place.** The tokenizer, the diff layout and the report keep their current algorithms. Each spot that today emits a scope string now emits a `Kind`, and the "join neighbours with the same label" invariant carries over unchanged. A reader of the tokenizer sees the same frame-stack walk with kind constants where scope constants were. TextMate scope strings shrink to a single per-kind mapping, used only by the VS Code importer and by the parity tests. Those tests keep their exact expected strings by formatting tokens through a test-only formatter that writes `[scope]text[/scope]` from `Kind.Scope()`.

**Themes become data, rendering becomes formatters.** Today's `theme` struct (TextMate rules and a cache) stops being the runtime theme and becomes the internal engine of `LoadVSCodeTheme`. It resolves each kind's scope once and emits style strings. At runtime a theme is a map, and resolution is two map lookups and a fallback to `Text`. Formatters follow one shape: walk the stream once, resolve each token's style, write. `Terminal` keeps today's per-line reset discipline. `HTML` never looks at the theme. `HTMLInline` and `SVG` turn resolved styles into attributes. `WriteCSS` walks the theme's own entries in kind order, so a change to one kind changes one rule. Colour rendering (named → SGR code, named → palette hex) lives in one small table shared by `Terminal`, `HTMLInline`, `SVG` and `WriteCSS`.

**A private layout package between `errors` and the root.** The report's wording moves out of the root into `internal/report`, a new module boundary inside xcl. It is token-free and imported by both `errors` (for `%+v`) and the root (for tokens). This follows the existing pattern of `errors` as neutral ground: `internal/report` imports nothing from xcl but `internal/xcl` and existing libraries, so it never creates a cycle. The root's report builder becomes a thin adapter that maps segments to kinds and lexes source lines. Wrapping is a print-time step applied to `Detail` tokens (`PrintError`) or `Detail` segments (`%+v`), using the same function, so the two always agree.

**Printing follows `go/scanner.PrintError` and functional options.** The print functions take the writer first, then the subject, then options. They build tokens and hand them to the chosen formatter, so every output path is "tokens, then formatter", with no special cases per format. `EncodeOption`'s new method lets one variadic list carry both kinds of option, the same functional-option pattern already used for encoding and configuration. The error-tree walk is the one new traversal. It uses `Unwrap() error` and `Unwrap() []error`, as `errors.Is`/`As` do, and never descends into a diagnostic's `Cause`.

**Reference outputs as the cross-language contract.** A single external test in `highlight` builds the three reference inputs from real code: an error report from applying a bad fixture, a resource from encoding a fixture entity, and a change summary from a fixed diff. It compares them to the stored token JSON, then formats the stored tokens with every built-in formatter and compares byte for byte. An `-update` test flag (the standard Go golden-file idiom, new to this repo) rewrites the files. A person reviews the resulting diff, as with any golden change.

**Breaking removal, done in one sweep.** The old API is deleted outright. Every in-repo caller (tests, examples, `coreservice`, viper tests, parser tests) is moved to the new API in the same change, so the module never builds against a half-removed API. Docs inside the repo, the changelog's Breaking list and the website are updated to match. No compatibility shims remain.

## Dependencies

- **Design documents this plan was built on:** `printing-and-highlighting.md` from the `design` design source. It is the settled split between core and language, the kind numbering, the style syntax, the formatter shape, the print API, the HTML class contract and the removals that this plan implements.
- **`internal/xcl/hclsyntax` (in-repo HCL)**: provides `LexConfig`, which the tokenizer already uses. No change needed.
- **`github.com/mitchellh/go-wordwrap` v1.0.1 (existing)**: wraps detail text, now used from `internal/report`. No change needed.
- **`github.com/apparentlymart/go-textseg/v15` (existing)**: grapheme-cluster column counting for placing the fault, moving with the layout into `internal/report`. No change needed.
- **Go standard library**: `io`, `os` (`ModeCharDevice` for `For`), `fmt.Formatter`, `errors`, `encoding/json` (VS Code themes, reference token JSON), `strconv`, `strings`, `unicode/utf8`. No third-party highlighting dependency, as the constraints require.
- **`go.uber.org/zap` v1.27.0 (new, test-only, pinned)**: used only by the test proving that a configuration error logged with `zap.Error` carries the plain report in `errorVerbose`. It is not imported by any non-test code. The reason is recorded beside it in `go.mod` review notes and the changelog.
- **`github.com/stretchr/testify` (existing)**: `require` for every new test.
- **Example modules' existing `charmbracelet/lipgloss` and `muesli/termenv`**: they keep making the examples' colour decision. No change needed.
- **`events.SlogHandler` (existing)**: deliberately unchanged; logging integration comes through `fmt.Formatter` on the error types.
- **`coreservice` (existing)**: must move off the removed ANSI renderer in this plan, but its protocol is untouched.
- **Sibling spec `20261023075136-42c51a7d-printing-tokens-over-the-engine` (downstream)**: depends on this plan. It needs `ErrorTokens`, `EntityTokens`, `diff.(*Diff).Tokens`, `highlight.Kind` and `highlight.LoadVSCodeTheme` to exist, and it reserves the proto's `highlight` field. Nothing from it must land first.
- **Sibling spec `20261023075302-bad71237-printing-and-highlighting-python` (downstream)**: consumes the reference outputs under `highlight/testdata/reference/` through the xcl checkout its CI already makes. Nothing from it must land first.
- **xcl-website repo (`/home/nicj/code/github.com/jumppad-labs/xcl-website`)**: existing guide and example pages are edited. Docs should land after the Go API is final, so examples match.
- **xcl-vscode repo (`/home/nicj/code/github.com/jumppad-labs/xcl-vscode`)**: its grammar is the reference for the scopes `Kind.Scope()` returns. It is not changed.
- **Prior plans consulted**: `20261006112108-17623cda-encoder-syntax-highlighting` and `20261007111826-cf3b66d8-diff-rendering-and-docs` introduced the API being replaced. No dependency, history only.
- **No upstream work must land before this plan starts.**

## Testing Approach

Testing follows the repo's conventions: testify `require`, one behaviour per test, positive and negative cases in separate functions, no table-driven tests, and tests beside the code they test. Documentation, the changelog and the website are checked by review only. No test reads repository files to assert on prose, imports or removed identifiers. Golden data under `testdata/` is test input, not prose, and is used as such.

**Unit tests (most of the coverage).**
- `highlight`: kind arithmetic, names, classes and scopes (including unknown kinds); style parsing of every word, with unknown words skipped; whole-style resolution through parent and `Text`; `With` not mutating its receiver; `Validate` naming each unknown word and matching `ErrInvalidStyle`.
- `highlight` formatters, one behaviour per test:
  - `Plain` ignores the theme.
  - `Terminal` writes SGR codes, resets before every newline, and strips back to the plain text.
  - `HTML` writes the container class, kind plus parent classes, no span for `Text`, escaping, and no `style=`.
  - `HTMLInline` writes styles and no `class=`.
  - `SVG` is accepted by `encoding/xml`, with one `<text>` per line and a view-box width that fits the longest line.
  - `WriteCSS` output parses as rules all scoped under `.xcl`, parents come before children, and a theme changing only `Attribute` changes only that rule. "A CSS parser accepts it" is checked with a small standard-library rule tokenizer in the test, not a new dependency.
  - `For` returns Plain for a non-file, a regular file, and a terminal with `NO_COLOR` set, and Terminal for a character device. The device case uses `/dev/tty` or a pty only when available, and skips otherwise.
- `LoadVSCodeTheme`: properties coloured orange colour attribute names orange when a resource is printed. The existing invalid-theme cases keep their errors.
- `internal/report`: layout wording, location with and without entity and field, the "source code not available" path, multi-line and empty faults, grapheme columns, wrapping at a width and at 0, and the walk through `%w` wraps, `errors.Join` and `ConfigError`, never into a `Cause`.
- `errors`: context capture of ±2 lines, clipped at line 1 and at the last line; `%v`/`%s` give one line; `%+v` gives the report.

**Regression tests.**
- The existing 58 tokenizer scope assertions (the xcl-vscode parity tests) keep their exact expected strings, now produced through `Kind.Scope()`.
- `PrintEntity` (with computed values) and `PrintDiff` as plain text must equal output captured from the current encoder and diff renderer before the refactor.
- Every existing test of the removed API is ported to its replacement rather than deleted, so coverage of the layout, the diff and the encoder is kept.

**Integration tests (root package).**
- Applying a fixture with an unsupported block on line 7 and calling `PrintError` gives the full report: first line, location line, numbered lines 5–9, detail.
- A two-problem fixture built from recoverable faults (per the HCL-halts gotcha), wrapped by the application with `%w`, prints both reports in order.
- A non-configuration error prints as its message and a newline.
- `%+v` equals plain `PrintError`.
- Detail wrapping holds at the default 80, at 40, and with wrapping off.
- Terminal output for the bad configuration resets before newlines and strips to plain.
- HTML output for the same error marks the fault with `xcl-problem xcl-error`.
- A user `FormatterFunc` wrapping each token in brackets receives the whole report and writes to the given writer.
- Labelled text (`ErrorTokens`, `EntityTokens`, `Diff.Tokens`) joins to the plain output.
- Logging the bad configuration's error with `zap.Error` puts the full report in `errorVerbose`.

**Contract tests (reference outputs).** One external `highlight_test` test builds the three reference inputs from real code. It checks them against the stored token JSON, then checks every built-in formatter's output, the default stylesheet and the class list byte for byte against `highlight/testdata/reference/`. This is the contract other languages are held to. Regenerating it is deliberate (`-update`), and a person reviews the diff.

**End-to-end (examples).** Each example that prints errors gets a smoke-test case that runs the real binary against a configuration with a mistake and asserts that stderr carries a report with the numbered faulty line. The prettylog and plugin `inspect`/`plan` tests keep asserting forced-colour output.

**Success metrics.**
- *Showing a configuration problem with its source takes one call in Go* — **Behavioural test**: the root integration test makes exactly one `PrintError` call on the error `Apply` returned and asserts that the output holds the summary, location and numbered source lines.
- *No example program prints a configuration error as a single line* — **Behavioural test**: each example's smoke test with a broken configuration asserts that stderr holds the report's gutter and faulty line, not a one-line `error:` message.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: the `CHANGELOG.md` entry lists a correct one-line replacement for every removed function (`Piece`, `Pieces`, `Renderer`, `RendererFunc`, `PieceFunc`, `Text`, `ANSI`, `ANSIRenderer`, `NewANSI`, `ANSIOption`, `WithTheme(io.Reader)`, `WithThemeFile`, the `Scope*` constants, `DiagnosticPieces`, `Diff.Pieces`).
- **Manual — captured in the implementation test plan**: the xcl website pages cover printing errors, resources and change summaries, output formats, themes, class names, stylesheet generation, custom formats and logging, each with a Go example, and no page still documents a removed function. The site builds.
- **Manual — captured in the implementation test plan**: the reference HTML (with the generated stylesheet), inline HTML and SVG look right in a browser, and Terminal output looks right in a real terminal, including with `NO_COLOR` set through `highlight.For`.
- **Manual — captured in the implementation test plan**: running each example against a broken configuration in a real terminal shows a readable, coloured report.

**Deliberate gaps.** There is no test for the protocol or other languages; those belong to the sibling specs. There is no test for `SlogHandler` changes, because there are none. Website rendering is reviewed by hand, because the site has no test suite.

## Milestones & Tasks

### Milestone 1: Configuration errors show their surrounding lines and print in full through `%+v`

**What changes**: A configuration problem now carries up to two lines of source before and after the lines at fault, taken from the text already read when parsing, so a report shows the fault in context again. Formatting a configuration error with `%+v` now writes the full plain report: severity and summary, location, numbered source lines, wrapped detail. `%v` and `Error()` stay one line. As a result, a structured logger that records an error's detailed formatting, such as zap, carries the full report with no adapter. The report's wording now lives in one private layout that every later printing path reuses. Its text is unchanged apart from the restored lines.

**Validation point**: The `errors` and `internal/report` tests pass: context clipped at the file's edges, `%+v` gives the report, `%v` gives one line, and the zap test finds the report in `errorVerbose`. The existing report tests pass with the extra context lines.

#### - [ ] Task: Lay out error reports in one private package
**Id:** 1a65fb65-b0e7-4064-8aa0-cb3e9b4ee239
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Move the wording of an error report (severity and summary, location, numbered source lines with the fault marked, detail) out of the root package into a new private layout package. The package knows nothing about colour or tokens, so both the error types and the root package can use it without an import cycle. It also gains the detail wrapping and the walk that finds every problem inside an error. The existing report function is rebuilt on top of it, so its output stays exactly as it is today.

*Technical detail:* [context.md#task-lay-out-error-reports-in-one-private-package](./context.md#task-lay-out-error-reports-in-one-private-package)

**Acceptance criteria**:
- [ ] There is one place that decides a report's wording, and the existing report output is unchanged
- [ ] Detail can be wrapped at any width, or left whole when the width is 0
- [ ] Every problem inside an error is found, including through application wrapping and joined errors, without descending into a problem's own cause

#### - [ ] Task: Capture two lines of context around each fault
**Id:** c5eb1740-6882-418f-a342-d9b05297d8e4
**Repo:** xcl
**Depends on:** none
**Execution:** agent

When a configuration problem is recorded, keep up to two lines before and after the lines at fault, clipped at the start and end of the file. They are taken from the source already read for parsing, never from disk. Tests that assert on the captured lines are updated to the wider range.

*Technical detail:* [context.md#task-capture-two-lines-of-context-around-each-fault](./context.md#task-capture-two-lines-of-context-around-each-fault)

**Acceptance criteria**:
- [ ] A fault in the middle of a file captures the two lines before and the two after it
- [ ] A fault on line 1 of a three-line file captures lines 1 to 3
- [ ] A fault on the last line captures that line and the two before it
- [ ] No file is read again to capture context

#### - [ ] Task: Print the full report through detailed error formatting
**Id:** 560866fc-d07d-4dd7-bb93-fa32b9b8fa26
**Repo:** xcl
**Depends on:**
- 1a65fb65-b0e7-4064-8aa0-cb3e9b4ee239 — Lay out error reports in one private package
- c5eb1740-6882-418f-a342-d9b05297d8e4 — Capture two lines of context around each fault
**Execution:** agent

Make a single configuration problem, and the collected configuration error, answer Go's detailed formatting with the full plain report, wrapped at 80 columns, while their ordinary message stays one line. Loggers that record an error's detailed form, such as zap, then carry the report with no adapter. A test proves this with zap, added as a test-only dependency.

*Technical detail:* [context.md#task-print-the-full-report-through-detailed-error-formatting](./context.md#task-print-the-full-report-through-detailed-error-formatting)

**Acceptance criteria**:
- [ ] Formatting a configuration error in its ordinary form gives a single line
- [ ] Asking for its detailed form gives the full report with the surrounding lines, detail wrapped at 80 columns
- [ ] Logging the error with zap writes the full plain report in the entry's detailed error field
- [ ] zap is used only by tests, at a pinned version

### Milestone 2: Errors, resources and change summaries print in one call, as plain text, terminal colour, HTML or SVG, styled by themes

**What changes**: Go programs print any error, resource or change summary in one call (`PrintError`, `PrintEntity`, `PrintDiff`) to any writer, choosing a format (Plain, Terminal, class-marked HTML, inline-styled HTML, SVG, or their own formatter), a theme and a detail width. Themes are short style strings that extend the default kind by kind, with categories covering their kinds. They can be validated, turned into a website stylesheet, or imported from a VS Code colour theme. `highlight.For` picks colour from the destination on request. Extenders get the labelled text of any report, resource, change summary or configuration text. The previous highlighting, report and change-summary rendering API is removed in the same change, and every caller in the repository, the core service included, moves to the new one. Plain output of resources and change summaries is byte-identical to before.

**Validation point**: The whole root module's test suite passes, including the ported tokenizer parity assertions with unchanged scope strings, the "matches captured output" regression tests, and the new print, format, theme and labelled-text tests. Nothing in the module refers to a removed identifier, which the build itself proves.

#### - [ ] Task: Capture today's plain output of resources and change summaries
**Id:** c944aff2-99d6-443f-9a69-bc5bf771153b
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Before anything about printing changes, record the exact plain text the current encoder writes for a fixed set of resources (with computed values) and the current change-summary renderer writes for a fixed set of change summaries. Store it as test data. Later tests compare the new printing against these bytes, so the refactor cannot alter the text unnoticed.

*Technical detail:* [context.md#task-capture-todays-plain-output-of-resources-and-change-summaries](./context.md#task-capture-todays-plain-output-of-resources-and-change-summaries)

**Acceptance criteria**:
- [ ] The captured output comes from the code as it stands before this plan's printing changes
- [ ] The fixed inputs cover nested blocks, references, sensitive values, every kind of change and every resource action

#### - [ ] Task: Rebuild highlighting around kinds, themes and formatters
**Id:** 668c52a0-5ffc-46ca-a0ab-801363fc69c1
**Repo:** xcl
**Depends on:**
- c944aff2-99d6-443f-9a69-bc5bf771153b — Capture today's plain output of resources and change summaries
**Execution:** agent

Replace the highlighting package's scope-labelled pieces and renderers with the design's model. Text is labelled with numbered kinds that fall back to their category. Themes are short style strings that extend the default kind by kind and can be validated. Formatters write plain text, terminal colour, class-marked HTML, inline-styled HTML or SVG to any writer, from the whole labelled text at once. The package also generates website stylesheets from a theme, picks colour from a destination on request, and imports VS Code colour themes. The old highlighting API is deleted, and the tokenizer's grammar-parity tests keep their exact expected scopes.

*Technical detail:* [context.md#task-rebuild-highlighting-around-kinds-themes-and-formatters](./context.md#task-rebuild-highlighting-around-kinds-themes-and-formatters)

**Acceptance criteria**:
- [ ] Configuration text is labelled exactly as before, and the existing grammar-parity assertions pass with unchanged scope names
- [ ] A theme that styles only comments colours every kind of comment, and styling line comments as well gives them their own style
- [ ] An unknown style word is reported by validation and ignored when printing
- [ ] Terminal output resets colour before every newline and, with codes removed, equals the plain output
- [ ] HTML output uses only the documented class names, escapes text and has no inline styles; inline-styled HTML has styles and no classes
- [ ] SVG output is a well-formed document with one text line per source line and a view box wide enough for the longest line
- [ ] A stylesheet generated from any theme is valid CSS scoped to the container class, and changing only attributes changes only the attribute rule
- [ ] A VS Code theme that colours properties orange colours attribute names orange
- [ ] Colour chosen from a destination is terminal colour for a terminal and plain text for a file or when NO_COLOR is set

#### - [ ] Task: Label change summaries with kinds
**Id:** dd13916e-b0ee-4851-a1a7-670f3cc2f616
**Repo:** xcl
**Depends on:**
- 668c52a0-5ffc-46ca-a0ab-801363fc69c1 — Rebuild highlighting around kinds, themes and formatters
**Execution:** agent

Switch the change-summary layout from scope-labelled pieces to labelled text with kinds, exposed as the summary's tokens. Additions, deletions and changes are marked with the diff kinds, comment lines as comments, and headers and values as configuration. The text itself does not change.

*Technical detail:* [context.md#task-label-change-summaries-with-kinds](./context.md#task-label-change-summaries-with-kinds)

**Acceptance criteria**:
- [ ] A change summary's labelled text joins back to exactly the text it produced before
- [ ] Created, deleted and changed parts carry the inserted, deleted and changed kinds

#### - [ ] Task: Add one-call printing of errors, resources and change summaries
**Id:** bf7098c4-edcc-45e6-84c6-db052d6ea591
**Repo:** xcl
**Depends on:**
- 1a65fb65-b0e7-4064-8aa0-cb3e9b4ee239 — Lay out error reports in one private package
- 560866fc-d07d-4dd7-bb93-fa32b9b8fa26 — Print the full report through detailed error formatting
- 668c52a0-5ffc-46ca-a0ab-801363fc69c1 — Rebuild highlighting around kinds, themes and formatters
- dd13916e-b0ee-4851-a1a7-670f3cc2f616 — Label change summaries with kinds
**Execution:** agent

Give the root package one call each to print an error, a resource or a change summary to any writer, with a choice of format, theme and detail width. Resources also accept the existing choices about which values to include. Extenders can obtain the labelled text of an error report or a resource. The old report function is removed, and its tests move to the new functions. Error printing finds every configuration problem in an error, however it was wrapped, and prints anything else as its message.

*Technical detail:* [context.md#task-add-one-call-printing-of-errors-resources-and-change-summaries](./context.md#task-add-one-call-printing-of-errors-resources-and-change-summaries)

**Acceptance criteria**:
- [ ] Printing the error from applying a configuration with an unsupported block on line 7 writes a report starting "Error: Unsupported block type", naming the file, line 7, the block and the field, showing numbered lines 5 to 9, and ending with the detail
- [ ] A configuration with two problems, wrapped by the application, prints both reports in order
- [ ] An error without a configuration problem prints as exactly its message and a newline
- [ ] The detailed formatting of the configuration error equals printing it as plain text
- [ ] Detail is wrapped at 80 columns by default, at the chosen width, or not at all
- [ ] Plain output of the fixed resources and change summaries equals the output captured before the change
- [ ] Terminal, HTML and inline-HTML printing of the error behave as the formats promise, with the fault marked as an error
- [ ] A user-written format receives the whole report at once and writes to the given destination
- [ ] The labelled text of an error report, a resource and a change summary joins to its plain output

#### - [ ] Task: Move every remaining caller to the new printing
**Id:** 97a596f0-3dbc-428b-8c81-8c64907e1038
**Repo:** xcl
**Depends on:**
- bf7098c4-edcc-45e6-84c6-db052d6ea591 — Add one-call printing of errors, resources and change summaries
**Execution:** agent

Update everything else in the repository that used the removed API. The core service keeps colouring encoded text on request through the terminal format. Parser, viper and configuration tests are ported. The example programs' resource and change-summary output goes through the new print functions while keeping their own colour decision. After this task the whole repository, examples included, builds and passes with no reference to a removed function.

*Technical detail:* [context.md#task-move-every-remaining-caller-to-the-new-printing](./context.md#task-move-every-remaining-caller-to-the-new-printing)

**Acceptance criteria**:
- [ ] The core service still returns coloured text when asked for highlighting, and plain text otherwise
- [ ] The examples' inspect, plan and pretty-log output is unchanged in plain text and still coloured when colour is forced
- [ ] No code or test in the repository uses a removed highlighting, report or change-summary rendering function

### Milestone 3: Reference outputs published for every language's printing

**What changes**: The repository publishes a fixed set of labelled inputs (an error report, a resource, a change summary), each one's exact output in every built-in format, the stylesheet generated from the default theme, and the list of HTML class names. The Go formatters are held to these byte for byte. Other languages' printing (Python first) can then be checked against the same files without running the engine, and any change to a format or a class name becomes a visible, reviewed diff of a public contract.

**Validation point**: The reference-output test passes without `-update`, and regenerating with `-update` leaves the tree unchanged.

#### - [ ] Task: Publish reference outputs for every built-in format
**Id:** 60674e3d-c8e7-4fa8-a829-c0e0a9baa721
**Repo:** xcl
**Depends on:**
- bf7098c4-edcc-45e6-84c6-db052d6ea591 — Add one-call printing of errors, resources and change summaries
- 97a596f0-3dbc-428b-8c81-8c64907e1038 — Move every remaining caller to the new printing
**Execution:** agent

Add the reference set other languages are checked against: labelled inputs for an error report, a resource and a change summary, each one's exact output in every built-in format with the default theme, the stylesheet generated from the default theme, and the list of every HTML class name. A test builds the inputs from real code and holds the Go formats to the files byte for byte. A flag regenerates them for deliberate, reviewed changes.

*Technical detail:* [context.md#task-publish-reference-outputs-for-every-built-in-format](./context.md#task-publish-reference-outputs-for-every-built-in-format)

**Acceptance criteria**:
- [ ] The repository holds labelled inputs, outputs for every built-in format, the default stylesheet and the class-name list
- [ ] The Go formats' output matches every reference file byte for byte
- [ ] Regenerating the files from unchanged code leaves them unchanged

### Milestone 4: Examples print reports, and the release notes and website describe the new printing

**What changes**: Every example program prints errors through the new error printing, so running one against a broken configuration shows a report with the faulty source lines instead of a one-line message, coloured where the example already colours output. The changelog describes the new printing and gives a one-line replacement for every removed function. The repository's own docs and the xcl website document printing errors, resources and change summaries, output formats, themes, class names and stylesheet generation, custom formats and logging integration, with Go examples, and no page documents a removed function any more.

**Validation point**: Each example's smoke test against a broken configuration finds a report on stderr. The website builds. The manual reviews of the changelog and the website pass.

#### - [ ] Task: Make the example programs print errors as reports
**Id:** 16cab7d3-7891-450c-9136-b14922ecdd54
**Repo:** xcl
**Depends on:**
- 97a596f0-3dbc-428b-8c81-8c64907e1038 — Move every remaining caller to the new printing
**Execution:** agent

Change every example program that reports an error to print it through the new error printing, coloured where the example colours its other output. A broken configuration then shows a report with the faulty lines instead of a one-line message. Each example gets a smoke test that runs the real program against a configuration with a mistake.

*Technical detail:* [context.md#task-make-the-example-programs-print-errors-as-reports](./context.md#task-make-the-example-programs-print-errors-as-reports)

**Acceptance criteria**:
- [ ] Running each example against a configuration with a mistake prints a report with the numbered faulty source lines
- [ ] No example prints a configuration error as a single line
- [ ] Application errors that are not configuration problems still print as one line

#### - [ ] Task: Write the release notes and repository docs
**Id:** 0ea6110b-3a4b-41aa-9093-b31c673ae5e3
**Repo:** xcl
**Depends on:**
- bf7098c4-edcc-45e6-84c6-db052d6ea591 — Add one-call printing of errors, resources and change summaries
- 60674e3d-c8e7-4fa8-a829-c0e0a9baa721 — Publish reference outputs for every built-in format
- 16cab7d3-7891-450c-9136-b14922ecdd54 — Make the example programs print errors as reports
**Execution:** agent

Add the changelog entry describing the new printing, themes, formats, reference outputs and restored context. Its Breaking list gives a one-line replacement for every removed function. Update the repository's overview and package table, and the package docs, to describe the new printing instead of the removed API.

*Technical detail:* [context.md#task-write-the-release-notes-and-repository-docs](./context.md#task-write-the-release-notes-and-repository-docs)

**Acceptance criteria**:
- [ ] The changelog lists a one-line replacement for each removed highlighting, report and change-summary rendering function
- [ ] The repository docs describe the new printing and no longer mention a removed function

#### - [ ] Task: Document the new printing on the website
**Id:** a3b4a3e9-2874-4159-b8a9-83464bd0fb8b
**Repo:** xcl-website
**Depends on:**
- bf7098c4-edcc-45e6-84c6-db052d6ea591 — Add one-call printing of errors, resources and change summaries
- 60674e3d-c8e7-4fa8-a829-c0e0a9baa721 — Publish reference outputs for every built-in format
**Execution:** agent

Rewrite the website's highlighting, problems, diff, events and example pages to document the new printing with Go examples: printing errors, resources and change summaries, output formats, themes, the HTML class names and stylesheet generation, custom formats, and logging integration. Every mention of a removed function goes. Pages are edited in place, never renamed or removed, so no URL breaks.

*Technical detail:* [context.md#task-document-the-new-printing-on-the-website](./context.md#task-document-the-new-printing-on-the-website)

**Acceptance criteria**:
- [ ] The site has sections on printing errors, resources and change summaries, output formats, themes, class names, stylesheet generation, custom formats and logging integration, each with a Go example
- [ ] No page documents a removed function
- [ ] Every existing page URL still works and the site builds

## Open Questions

None. The two uncertainties found during research were settled during planning with a probe against the current code. Validation returns the collected configuration error unwrapped, so zap sees its detailed formatting. An unsupported nested block reports its entity and field, so the acceptance fixture's location line is achievable. Every other choice is decided and recorded in the drafting assumptions.

## Out of Scope

- **Python printing.** The Python tokens, themes, formatters, `print_error`/`format_error`, exception notes and logging handler are not part of this plan. They are the sibling spec `20261023075302-bad71237-printing-and-highlighting-python`, which consumes the reference outputs published here. The website's Python tabs and the "not available from Python" statements stay as they are for that spec to rewrite.
- **TypeScript.** The design defines it, but no TypeScript package exists yet. That includes the `xcl.css` file shipped in that package.
- **Labelled text and theme conversion over the engine.** The proto changes (a report on every `Error`, tokens on encode and diff responses, the `ConvertTheme` call) and reserving the legacy `highlight` field are not part of this plan. They are the sibling spec `20261023075136-42c51a7d-printing-tokens-over-the-engine`. Here the core service only keeps `highlight=true` working through the Terminal formatter.
- **Structured slog fields for configuration problems.** Errors do not implement `slog.LogValuer`, per the design's decision. slog logs the one-line message.
- **Changing what a report says.** Beyond restoring the surrounding lines and labelling its parts, the report's wording, layout and blank-line separation are unchanged.
- **Changes to `events.SlogHandler`.** It stays as it is. Logging integration comes from `fmt.Formatter` on the error types.
- **Changes to the xcl-vscode extension.** Its grammar remains the reference the scopes follow. Its own tests are not touched.
- **Cutting a release.** Tagging the next pre-1.0 minor version that carries this breaking change is not part of this plan. Releases follow the repo's release process.
- **A Python website redesign or new website pages and navigation.** The docs are rewritten inside existing pages only.
- **Removing the protocol's legacy `highlight` encode option, or Python's `highlight=` parameter.** Those belong to the engine and Python specs respectively.
