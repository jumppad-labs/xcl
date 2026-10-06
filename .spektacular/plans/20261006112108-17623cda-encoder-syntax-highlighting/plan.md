---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Plan: 20261006112108-17623cda-encoder-syntax-highlighting

<!-- Metadata -->
<!-- Created: 2026-10-06T11:39:38Z -->
<!-- Commit: 8271816 -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

xcl can turn an entity back into configuration text, but only as plain text, so an application that wants it coloured has to write its own highlighter, as the logging example does today. This plan adds a new public `highlight` package. It labels configuration text with the same TextMate scope names the xcl VS Code extension uses and passes every piece to a pluggable renderer. It ships a terminal renderer that uses the terminal's 16 colours by default or any VS Code colour theme, and an `xcl.Highlight` encode option to switch it on. Application developers get configuration in their terminal output that looks the way it does in the editor, can produce any other format with one small interface, and the logging example drops its own highlighter.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, never mix positive and negative cases, tests live with the code they test** — every new test in `highlight`, `encode` and `prettylog` follows it. The rule against tests reaching into other directories is why the sweep of every configuration in the repository is a manual check, and the rule against tests that inspect repository files (imports, `go.mod`, source scans) is why the library's dependency boundary is checked in review rather than by a test.
- **Shared test helpers live in `internal/testutil`** — the ANSI strip helper is used by `highlight`, the root package and prettylog. prettylog is becoming a separate module, so it keeps its own copy, and only `highlight` and the root package share the helper in `internal/testutil`.
- **Shared error types live in the `errors` package, sentinel-and-detail, re-exported from `xcl`** — `ErrInvalidTheme` and `InvalidThemeError`, with a pointer receiver that always wraps the cause.
- **Dependencies: prefer the standard library** — the ANSI codes and the theme JSON reader are written with the standard library, and the module gains no dependency.
- **Code style: small, focused interfaces, and `any` over `interface{}`** — `Renderer` has a single method.
- **Project structure: `/internal` is private** — the tokeniser's helpers stay unexported inside `highlight`, and the public surface is the `Renderer` interface, `RendererFunc`, `Text`, the ANSI renderer and its options.
- **HCL lives in `internal/xcl` (MPL); never modify dependencies** — `highlight` only imports `hclsyntax` and changes none of its files, so no MPL header or `UPSTREAM.md` entry is needed.

## Architecture & Design Decisions

**Highlighting is a post-pass over the encoder's finished text, done by a new public `highlight` package.** `encodeEntity` (`encode.go:180`) is the single path behind both `EncodeEntity` and `EncodeSavedEntity`, and it ends by returning the formatted `file.Bytes()`. A new option, `xcl.Highlight(renderer highlight.Renderer)`, joins `IncludeComputed`, `IncludeEmpty`, `RevealSensitive` and `ShowReferences` in the closed `encodeOptions` struct. When it is set, `encodeEntity` passes the finished text through `highlight.Text(text, renderer)` before returning it. Without it nothing changes, so existing calls and their output stay byte-for-byte the same. Because the pass runs on the final text, it covers everything the encoder writes, including the expressions `ShowReferences` puts back. The library never looks at where the text is going; whether to colour is the caller's decision. The package sits at the top level beside `mask`, `events` and `state`, following the module's layout for public packages (`conventions/project-structure.md`, read as in the masking plan: `/internal` is private). It depends only on the standard library, `internal/xcl/hclsyntax` and the `errors` package, so the library gains no charmbracelet or other new module dependency; review confirms this.

**Tokens come from the HCL scanner, labelled to match the xcl-vscode grammar.** `highlight.Text` lexes the text with `hclsyntax.LexConfig` (`internal/xcl/hclsyntax/public.go:159`). It walks the tokens in byte order and labels each one with the TextMate scope that `xcl-vscode/syntaxes/xcl.tmLanguage.json` gives the same text, using a small amount of look-around in place of the grammar's regular expressions. Some examples:
- An identifier at the start of a line, followed by labels and `{`, is `storage.type.xcl`. With two labels, the first is `entity.name.type.xcl` and the second `entity.name.tag.xcl`.
- An identifier followed directly by `{` is `entity.name.function.block.xcl`.
- An identifier followed by `=` is `variable.other.property.xcl`. One followed by `(` is `support.function.builtin.xcl` when it is in the grammar's built-in list, and `entity.name.function.xcl` otherwise.
- The first identifier of a dotted chain is `support.class.reference.xcl`, each `.` is `punctuation.accessor.xcl` and each later part is `variable.other.member.xcl`.
- Quoted strings are `string.quoted.double.xcl`, with escapes split out as `constant.character.escape.xcl` and `${`/`}` as the interpolation punctuation scopes.
- A heredoc's opener is split into `keyword.operator.heredoc.xcl` and `keyword.control.heredoc.xcl`, and its body is `string.unquoted.heredoc.xcl`, with `#{{ … }}` template markers picked out as the grammar does.
- Comments are split by their prefix.

Any text between tokens, such as whitespace, and any token the grammar leaves unlabelled, such as braces and commas, is passed through with the empty scope. Every byte of the input therefore reaches the renderer exactly once and in order, so the text cannot change by construction. The scope strings keep the grammar's `.xcl` suffix, so they are exactly the names the editor shows.

**A renderer is one small interface, and the terminal renderer is built from a VS Code theme.**
- `highlight.Renderer` has one method, `Render(scope, text string) string`. A `RendererFunc` adapter covers one-off renderers. Unlabelled text goes through the renderer too, so a renderer for a format that needs escaping, such as HTML, sees every byte (`conventions/code-style.md`: small, focused interfaces).
- `highlight.NewANSIRenderer(options ...ANSIOption) (*ANSIRenderer, error)` takes `highlight.WithTheme(io.Reader)` or `highlight.WithThemeFile(path)`. Each reads a VS Code colour theme: JSON with comments and trailing commas allowed, `tokenColors` entries whose `scope` is a string, a comma-separated string or an array, and `settings.foreground` / `settings.fontStyle`.
- Theme matching follows TextMate: a selector matches a scope when it equals the scope or is a dot-segment prefix of it, and the selector with the most segments wins. Ties go to the later rule, as in VS Code. Foreground and font style are resolved independently, each from the most specific rule that sets it.
- A theme colour is written as 24-bit SGR (`38;2;r;g;b`), and bold, italic, underline and strikethrough as SGR 1, 3, 4 and 9. A scope that no rule matches, and unlabelled text, are written with no codes at all, which leaves the terminal's default colour.
- With no theme, a built-in default maps scopes to the standard 16 foreground codes (30–37, 90–97) only, carrying over the palette prettylog uses today.
- A token spanning lines, such as a heredoc or a block comment, is styled one line at a time, with each newline left outside the escape codes. A caller can then indent or prefix lines, as prettylog does, without colour leaking.
- An unreadable or invalid theme fails the constructor with `xcl.ErrInvalidTheme`, a sentinel with an `InvalidThemeError` detail type in the `errors` package, re-exported from `xcl` (`conventions/shared-errors-package.md`). A theme is never silently replaced by the default.

**The logging example and the docs adopt it.** `example/prettylog` deletes `highlight.go` and `highlight_test.go`. It keeps deciding colour for its writer as it does today, through lipgloss's renderer, and adds `xcl.Highlight(defaultRenderer)` to its `EncodeSavedEntity` call only when that writer shows colour. The xcl-website page `src/pages/configuration-text.mdx` gains a "Highlighting" section showing how to enable highlighting, pass a theme and write a renderer.

This direction beats a regular-expression highlighter, a TextMate engine port, and taking tokens from `hclwrite` before formatting. Regular expressions cannot follow multi-line or nested constructs. A TextMate port needs lookbehind or cgo, which Go's standard library does not provide. Formatting changes byte positions, so tokens taken before it no longer match the final text. Lexing the final bytes uses scanner code xcl already owns and needs no new dependency (`conventions/dependencies.md`). The cost is that the labelling rules restate the grammar in Go, so a parity fixture pins them to the grammar. See `research.md#alternatives-considered-and-rejected` for the evidence.

## Component Breakdown

- **Tokeniser (new, inside the public `highlight` package, unexported).** It turns configuration text into an ordered sequence of `(scope, text)` pieces covering every byte of the input. It lexes with the HCL scanner already in xcl, then labels each token with the scope the xcl-vscode grammar gives the same text. It knows nothing about entities, rendering or terminals. `highlight.Text` is its only caller.
- **`highlight.Text` and the `Renderer` interface (new, public).** `Text` runs the tokeniser and hands each piece to a `Renderer`, then joins the results. `Renderer` is what a developer implements to produce any output format, and `RendererFunc` adapts a plain function. This is the single seam between labelled tokens and output. The encoder option and any application that already holds configuration text both use it.
- **Theme (new, inside `highlight`).** It reads a VS Code colour theme, resolves `tokenColors` rules into selectors with a foreground and a font style, and answers "what style does this scope get" by TextMate prefix and specificity rules. A theme that cannot be read or is invalid is rejected with a typed error. The built-in default theme is expressed in the same rule form, using 16-colour palette indices. Only the ANSI renderer uses it.
- **ANSI renderer (new, public in `highlight`).** It implements `Renderer` by wrapping each labelled token in SGR escape codes taken from its theme. Theme colours are written as 24-bit codes and the default theme's colours as standard 16-colour codes. Unmatched and unlabelled text gets no codes, and multi-line tokens are styled one line at a time. Construction is where theme errors surface, through functional options for a theme from a reader or from a file.
- **Encoder options (changed, root `xcl` package).** A new `Highlight(renderer)` option joins the existing closed option set. The shared encode path applies `highlight.Text` to its finished text only when the option is given, so `EncodeEntity` and `EncodeSavedEntity` gain highlighting identically, and all existing output is unchanged.
- **Errors package (changed).** It gains the invalid-theme sentinel and detail type, re-exported from the root package beside the other encode errors. Its thin import list is unchanged.
- **Logging example, prettylog (changed).** Its own highlighter is removed. It keeps deciding per writer whether colour is shown, and when it is, it asks the encoder for highlighting with the built-in ANSI renderer. It owns no tokenising or styling of configuration text any more.
- **Library dependency boundary (unchanged rule, checked in review).** No non-example package may depend on charmbracelet or bring in a new module. The new `highlight` package keeps to that, and review confirms it; no test inspects imports or `go.mod`.
- **Documentation site encoding page (changed, xcl-website).** It gains a section on turning highlighting on, passing a theme and writing a custom renderer.
- **Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.**

## Data Structures & Interfaces

**`highlight.Renderer` (new, public interface).** It is what a developer implements to produce an output format. `Text` calls it once for every piece of the input, in order. `scope` is the TextMate scope the xcl-vscode grammar gives the piece, such as `variable.other.property.xcl`, or `""` for text the grammar leaves unlabelled, such as whitespace, braces and commas. The returned strings are concatenated as they are.

```go
package highlight

type Renderer interface {
    Render(scope, text string) string
}

// RendererFunc lets a plain function act as a Renderer
type RendererFunc func(scope, text string) string

// Text returns text with every piece passed through renderer. Joining the
// pieces with no renderer changes gives back text exactly.
func Text(text []byte, renderer Renderer) []byte
```

**Scope names (new, public constants).** Exported string constants name every scope the tokeniser emits, so renderers and tests do not repeat string literals. They are the grammar's full names, `.xcl` suffix included: `ScopeBlockType = "storage.type.xcl"`, `ScopeTypeLabel = "entity.name.type.xcl"`, `ScopeNameLabel = "entity.name.tag.xcl"`, `ScopeNestedBlock = "entity.name.function.block.xcl"`, `ScopeAttribute = "variable.other.property.xcl"`, `ScopeString`, `ScopeEscape`, `ScopeInterpolationBegin`/`End`, `ScopeHeredoc`, `ScopeHeredocOperator`, `ScopeHeredocMarker`, `ScopeTemplateInterpolation`, `ScopeNumber`, `ScopeLanguageConstant`, `ScopeBuiltinFunction`, `ScopeFunction`, `ScopeReference`, `ScopeAccessor`, `ScopeMember`, `ScopeOperator`, `ScopeLineComment` (`//`), `ScopeHashComment` (`#`) and `ScopeBlockComment`.

**`highlight.ANSIRenderer` and its options (new, public).** This is the built-in terminal renderer. Its constructor is the only place a theme is read, so a bad theme fails there and never during rendering.

```go
type ANSIRenderer struct { /* resolved theme, unexported */ }

type ANSIOption func(*ansiOptions)

func WithTheme(theme io.Reader) ANSIOption   // a VS Code colour theme, JSON or JSONC
func WithThemeFile(path string) ANSIOption   // the same, read from a file

func NewANSIRenderer(options ...ANSIOption) (*ANSIRenderer, error)
func (r *ANSIRenderer) Render(scope, text string) string
```

**Theme (new, unexported).** Theme JSON is decoded into a list of rules. Each rule holds one selector, an optional colour and an optional font style. The default theme is a hard-coded rule list using 16-colour indices.

```go
type rule struct {
    selector  string     // e.g. "string.quoted", the last element of a descendant selector
    segments  int        // dot segments in selector, its specificity
    order     int        // position in the theme, later wins a tie
    colour    *colour    // nil when the rule sets no foreground
    fontStyle *fontStyle // nil when the rule sets no fontStyle; an empty style resets
}
type colour struct { basic int; r, g, b uint8; truecolour bool }   // basic: SGR 30–37 / 90–97
type fontStyle struct { bold, italic, underline, strikethrough bool }
```

The serialization boundary is the VS Code theme file. Only `tokenColors[].scope` and `tokenColors[].settings.{foreground,fontStyle}` are read. A `scope` may be a string, a comma-separated string or an array of strings. Colours are written `#RGB`, `#RRGGBB` or `#RRGGBBAA`, with alpha ignored. A font style is a space-separated list of `bold`, `italic`, `underline` and `strikethrough`, or empty.

**Encode option (new, root package).**

```go
func Highlight(renderer highlight.Renderer) EncodeOption   // sets encodeOptions.renderer
```

A nil renderer is treated as no highlighting.

**Errors (new, `errors` package, re-exported from `xcl`).**

```go
var ErrInvalidTheme = errors.New("invalid theme")   // matched with errors.Is

type InvalidThemeError struct {
    Source string // the file path, or "theme" for a reader
    Reason string // what was wrong, e.g. `tokenColors[3].settings.foreground "#zz0000" is not a colour`
    Err    error  // the underlying read or JSON error, if any
}
func (e *InvalidThemeError) Error() string
func (e *InvalidThemeError) Unwrap() []error   // ErrInvalidTheme and Err
```

**Unchanged contracts.** `EncodeEntity`, `EncodeSavedEntity` and the existing options keep their signatures and output.

## Implementation Detail

**A new leaf package, following the shape of `mask`.** `highlight` is a small public package that depends only on the standard library, the HCL scanner already in xcl and the `errors` package. Its public surface is the `Renderer` interface, `RendererFunc`, `Text`, the scope constants, the ANSI renderer and the renderer's options. Everything else stays unexported: the tokeniser, theme parsing and rule matching. A developer reading the package finds three concerns, each in its own file: tokenising (lexing and labelling), the theme (reading and matching) and the ANSI renderer (styling). Each has its own tests beside it.

**The tokeniser restates the grammar in Go, as a token walk with look-around.** It is a new pattern for the codebase. It lexes once, then makes a single pass over the token list, looking a few tokens ahead or behind to decide a token's scope. Examples:
- An identifier followed by `=` is an attribute.
- An identifier at the start of a line, followed by quoted labels and `{`, is a block type.
- An identifier followed by `.` and another identifier is the root of a reference.

Each scope decision is a small, named function, documented with the grammar rule it mirrors, so a change to the grammar maps to one place in Go. Pieces inside strings and heredocs need finer splitting than the scanner gives: escape sequences in a quoted literal, the operator and marker of a heredoc opener, and `#{{ … }}` template markers in a heredoc body. These are cut out of the token's bytes with small scanners, not regular expressions over whole lines. The walk keeps a byte cursor and always emits the gap between the cursor and the next token as unlabelled text. This invariant is what guarantees highlighting never changes the text, and the fuzz test checks it.

**The theme is resolved once, at construction.** The ANSI renderer resolves its theme into a rule list when it is built. Each `Render` call then finds the best rule for its scope, separately for colour and for font style, and caches the result per scope. The default theme uses the same rule form with basic palette indices, so the same matching code serves both. Malformed input is reported with the path into the theme, for example `tokenColors[3].settings.foreground`, so a developer can find the bad entry. The errors follow the existing sentinel-and-detail pattern.

**The encoder change follows the existing option pattern.** `Highlight` is one more functional option on the closed `encodeOptions` struct. The shared encode path applies it as the last step, after formatting, so the option interacts with no other option. `ShowReferences` rewrites expressions before formatting, and highlighting then sees the final text like any other.

**The example change is a deletion plus one option.** prettylog loses its regex highlighter and its tests. Its handler keeps the colour decision it makes today for its writer, through lipgloss's renderer, and passes `xcl.Highlight` with the default ANSI renderer only when colour is on. Its existing coloured-output test changes to assert that the configuration lines carry SGR codes and that stripping them gives the plain encoding. prettylog never shows a theme and no example needs one, so the example uses only the default theme.

**Code-structure UX for a renderer author.** Writing a renderer means implementing one method that receives every byte with a label. The documentation shows a short marker-wrapping renderer and how to compare scopes against the exported constants or by prefix.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **`internal/xcl/hclsyntax` (internal, the HCL scanner).** `LexConfig` supplies the tokens and their byte ranges. It is used as it is, with no changes, so no MPL header or `UPSTREAM.md` entry is needed.
- **`errors` package (internal shared errors).** It gains the invalid-theme sentinel and detail type. Its import list is unchanged.
- **Root `xcl` encoder.** The single encode path gains the `Highlight` option. No existing option or output changes.
- **Go standard library only.** `encoding/json`, `io`, `os`, `strings` and `strconv` are enough for theme reading and SGR output. No new module enters `go.mod`, so the "no charmbracelet in the library" rule holds, as review confirms.
- **xcl-vscode grammar (`syntaxes/xcl.tmLanguage.json`, read only).** It is the reference for the scope names and the rules for assigning them. Nothing in that repo changes. The working tree holds uncommitted grammar edits, and this plan follows the working-tree version.
- **xcl-website (`src/pages/configuration-text.mdx`).** The encoding page gains a highlighting section. The site uses Astro MDX with expressive-code, and needs no new component.
- **charmbracelet lipgloss (examples only, existing).** prettylog keeps using it to decide whether its writer shows colour. It is no longer used to style configuration text.
- **Sibling spec `20261006071142-506b8289-e2e-suite-and-real-world-examples` (planning dependency, not ordered).** It makes prettylog a self-contained module that builds against a published xcl. The epic does not order the two specs. If it lands first, the prettylog migration in this plan must work in prettylog's own module. That needs either a published xcl version that includes `highlight` or the local `replace` that module uses during development, so the implementer should check which one prettylog uses at that point.
- **Prior plan `20261003153421-9fa72edd-masking` (precedent only).** It set the pattern this plan follows for a top-level public package, with errors in the `errors` package re-exported from `xcl`. Nothing needs to land first.

## Testing Approach

**Unit tests carry most of the weight, and they sit in the `highlight` package.** The tokeniser, the theme and the ANSI renderer are pure functions of their input, so almost every acceptance criterion can be asserted directly, without I/O or a real apply. Tests follow the project conventions: testify `require`, one behaviour per test function, no table-driven tests, positive and negative cases in separate functions, and each test file beside the code it tests.

- **Tokeniser parity with the editor grammar.** A fixture in the package's own testdata is modelled on the xcl-vscode grammar fixture and holds at least one token of every scope the grammar assigns. For each scope, a named test asserts that a representative piece of text is labelled with the grammar's scope name, `.xcl` suffix included. This covers block type, both labels, nested block, attribute, each comment form, string, escape, interpolation punctuation, heredoc operator, marker and body, template interpolation, number, language constant, built-in and other functions, reference root, accessor, member and operator. A reference test asserts that, in text written by `ShowReferences`, the first segment of each reference is labelled `support.class.reference.xcl`.
- **Text is never changed.** One test asserts that joining every piece through an identity renderer returns the input byte for byte, for the fixture and for real encoder output. A Go fuzz test, seeded from the package's testdata, asserts the same invariant for arbitrary input, including text that does not lex cleanly. This is the load-bearing guarantee behind "highlighting never changes the text".
- **Custom renderer.** A test renderer wraps each piece in `[scope]…[/scope]` markers. A test asserts that every labelled token appears wrapped with its expected label, and that unlabelled text reaches the renderer with the empty scope.
- **Theme matching.** Separate tests cover the matching rules:
  - A prefix selector matches a deeper scope.
  - A more specific selector overrides a general one, for colour and for font style independently.
  - The later rule wins a tie.
  - Comma-separated and array scopes both work.
  - A descendant selector matches on its last element.
  - Each of bold, italic, underline and strikethrough is written.
  - A JSONC theme with comments and trailing commas loads.
- **ANSI output.**
  - With no theme, a test asserts that every SGR code in the output is a 16-colour foreground code (30–37, 90–97) or a style or reset code, and never `38;5` or `38;2`.
  - With a reference theme in testdata, tests assert that each kind of token gets the theme's 24-bit colour and font style.
  - A theme without a rule for some kind of token leaves those tokens with no codes, and unlabelled text never gets codes.
  - A multi-line token puts no code around its newlines.
- **Invalid themes.** Each failure is its own negative test: an unreadable file, malformed JSON, a malformed colour, an unknown font style word, a `tokenColors` that is not a list, and an `include` key. Each asserts `errors.Is(err, xcl.ErrInvalidTheme)` and an `*InvalidThemeError` carrying the reason. The `errors` package gets the usual detail-type tests for its new type.

**Integration through the encoder (root package).** The tests reuse the existing encode fixture and a real apply, as the encode tests do today:
- Without `Highlight`, output is byte-identical to today's. This is guarded by the existing encode tests passing unchanged, plus one explicit test.
- With `Highlight` and the custom renderer, both `EncodeEntity` and `EncodeSavedEntity` produce wrapped output, and stripping the markers gives the plain text.
- With `Highlight` plus `ShowReferences`, the reference roots are labelled.
- A nil renderer behaves as no option.

**Regression guards.** prettylog's handler test changes from asserting its own styles to asserting that the configuration lines carry SGR codes when colour is forced on and that stripping them gives the plain encoding. Its plain-writer test confirms that no codes are written when the writer has no colour.

**Deliberate gaps.**
- No test drives the real VS Code tokeniser from Go. Parity is pinned by hand-written expectations taken from the grammar. Re-checking against the editor is a manual review, below.
- No test in this module globs configurations from other directories, because the testing convention forbids it. The whole-repository sweep is a manual check instead.
- No test inspects the library's imports or `go.mod`, because the testing convention forbids tests that inspect repository files. That the library gains no charmbracelet or other new module dependency is checked in review.
- The docs page has no automated test.

**Success metrics.**
- *"Every place xcl's examples show configuration in a terminal uses the library's highlighting, with no highlighting code of their own."* Partly a behavioural test: prettylog's handler test asserts that the coloured configuration comes from the encoder, and the prettylog package no longer contains a highlighter. Confirming that no other example prints configuration with its own code, including any example rewritten by sibling specs: **Manual — captured in the implementation test plan**.

**Manual reviews and checks.**
- Run highlighting over every `.xcl` configuration in the repository's examples and test fixtures, and confirm that stripping SGR codes gives the uncoloured text byte for byte: **Manual — captured in the implementation test plan**.
- Open the parity fixture in VS Code with the xcl extension, use "Inspect Editor Tokens and Scopes" on a sample of tokens, and confirm that the scopes match the labels `highlight` gives: **Manual — captured in the implementation test plan**.
- Run the logging example in a real terminal and check that the configuration is coloured the way the editor shows it, both in the default 16-colour theme and with a dark editor theme, and that no colour leaks past the configuration: **Manual — captured in the implementation test plan**.
- Review the new "Highlighting" section on the website's encoding page in a local build, and check that it shows enabling highlighting, passing a theme and writing a renderer: **Manual — captured in the implementation test plan**.
- Review the change and confirm the library gains no charmbracelet or other new module dependency: **Manual — captured in the implementation test plan**.

## Milestones & Tasks

### Milestone 1: Configuration text can be labelled and rendered in any format
**What changes**: Developers get a new `highlight` package that splits xcl configuration text into tokens. It labels each token with the same scope names the xcl VS Code extension uses and passes every piece to a renderer they write. They can turn configuration into marked-up text, HTML or any other format with one small interface, and the text itself never changes. This milestone adds no colours and does not change the encoder.

**Validation point**: Tests show that a fixture holding every scope the grammar assigns is labelled exactly as the grammar labels it. A marker-wrapping custom renderer wraps every token with its label. The fuzz test confirms that joining the pieces always gives back the input.

#### - [x] Task: Add the highlight package with the tokeniser and renderer interface
**Id:** 90d79027-5621-4838-873c-331c3f9a6268
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Create the public `highlight` package. Its tokeniser lexes configuration text with xcl's HCL scanner and labels every token with the scope the xcl-vscode grammar gives it. `Text` hands every piece of the input, labelled or not, to a `Renderer` in order. This is the core every later task builds on: one interface for any output format, and a guarantee that the text is never changed.

*Technical detail:* [context.md#task-add-the-highlight-package-with-the-tokeniser-and-renderer-interface](./context.md#task-add-the-highlight-package-with-the-tokeniser-and-renderer-interface)

**Acceptance criteria**:
- [x] Every scope the xcl-vscode grammar assigns has a test showing the tokeniser gives the same text the same scope name, `.xcl` suffix included.
- [x] In text that shows references as written, the first segment of each reference is labelled as a reference root.
- [x] A test renderer that wraps each token in markers naming its label gets every token wrapped with the expected label, and unlabelled text arrives with an empty label.
- [x] Joining the pieces with an identity renderer gives back the input byte for byte, for the fixtures and under fuzzing, including input that does not lex cleanly.
- [x] Review confirms the package pulls in no charmbracelet library and no new module.

### Milestone 2: Configuration can be coloured for the terminal, by default or with an editor theme
**What changes**: Developers can create a built-in terminal renderer. With no theme it uses only the terminal's 16 standard colours, so it follows the user's own palette. Given a VS Code colour theme, it colours tokens exactly as the editor would, with bold, italic and underline, and leaves tokens the theme does not cover in the terminal's default colour. A theme that cannot be read or is invalid fails when the renderer is created, with an `xcl.ErrInvalidTheme` error explaining what is wrong.

**Validation point**: Tests show that default output uses only 16-colour codes and that each token gets the reference theme's colour and style, with a more specific rule overriding a general one. Unmatched tokens carry no codes. Each kind of bad theme returns `xcl.ErrInvalidTheme`.

#### - [x] Task: Add the invalid-theme error
**Id:** c63cf7d1-d229-412d-b2a8-76c6a76a0502
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Add the sentinel error and its detail type for a theme that cannot be read or is invalid. Following the shared-errors convention, they live in the `errors` package and are re-exported from `xcl`. Callers can then match `xcl.ErrInvalidTheme` and read what was wrong and where.

*Technical detail:* [context.md#task-add-the-invalid-theme-error](./context.md#task-add-the-invalid-theme-error)

**Acceptance criteria**:
- [x] A theme error matches `xcl.ErrInvalidTheme`, and also matches the underlying read or parse error when there is one.
- [x] The error message names the theme's source and the reason it is invalid.
- [x] Review confirms the `errors` package imports nothing new.

#### - [x] Task: Read VS Code colour themes and match scopes against them
**Id:** 5fded1d4-fd87-4b6c-8681-9310847a9b79
**Repo:** xclconfig
**Depends on:**
- c63cf7d1-d229-412d-b2a8-76c6a76a0502 — Add the invalid-theme error
**Execution:** agent

Teach the `highlight` package to read a VS Code colour theme, comments and trailing commas included, into rules of selector, colour and font style. Resolve a token's style by TextMate rules: prefix selectors, with the most specific winning, and colour and font style resolved separately. The built-in default theme is a set of 16-colour rules in the same form. A bad theme is reported as `xcl.ErrInvalidTheme`, never replaced with defaults.

*Technical detail:* [context.md#task-read-vs-code-colour-themes-and-match-scopes-against-them](./context.md#task-read-vs-code-colour-themes-and-match-scopes-against-them)

**Acceptance criteria**:
- [x] A theme selector colours every scope it is a dot-segment prefix of, and a more specific selector overrides a general one.
- [x] Colour and font style are each taken from the most specific rule that sets them, and the later rule wins a tie.
- [x] Scopes written as a string, a comma-separated string or an array all work, and a theme with comments and trailing commas loads.
- [x] A scope no rule matches gets no style.
- [x] An unreadable file, malformed JSON, a malformed colour, an unknown font style, a malformed `tokenColors` and an `include` each fail with `xcl.ErrInvalidTheme`.

#### - [x] Task: Add the ANSI terminal renderer
**Id:** 60e83712-3914-47f9-b90c-ef7ffc1f6c95
**Repo:** xclconfig
**Depends on:**
- 90d79027-5621-4838-873c-331c3f9a6268 — Add the highlight package with the tokeniser and renderer interface
- 5fded1d4-fd87-4b6c-8681-9310847a9b79 — Read VS Code colour themes and match scopes against them
**Execution:** agent

Add the built-in terminal renderer, created with an optional theme from a reader or a file. It writes theme colours as 24-bit codes, the default theme as standard 16-colour codes, and bold, italic, underline and strikethrough as the theme sets them. Unmatched and unlabelled text gets no codes, and a token spanning several lines is styled one line at a time, so colour never leaks across lines.

*Technical detail:* [context.md#task-add-the-ansi-terminal-renderer](./context.md#task-add-the-ansi-terminal-renderer)

**Acceptance criteria**:
- [x] With no theme, output uses only the standard 16 foreground colours, with no 256-colour or 24-bit codes.
- [x] With the reference theme, every kind of token gets the theme's colour and font style.
- [x] Tokens a theme has no rule for, and text between tokens, are written with no colour codes.
- [x] Creating the renderer from an unreadable or invalid theme returns an error, and never a renderer with default colours.
- [x] Stripping the colour codes from rendered output gives back the original text.

### Milestone 3: The encoder highlights on request, and the logging example uses it
**What changes**: `EncodeEntity` and `EncodeSavedEntity` accept a new `xcl.Highlight(renderer)` option and return highlighted text, including text that shows references as written. Without the option the output is exactly as before. The logging example drops its own highlighter and colours the configuration it prints through the library. The documentation website's encoding page explains how to turn highlighting on, use a theme and write a renderer.

**Validation point**: Encoder tests show that highlighted output from both entry points strips back to the plain output, that references are coloured, and that output without the option is unchanged. prettylog's tests pass with no highlighting code left in the example. Review confirms the library gained no charmbracelet or other new module dependency, and the website page builds with the new section.

#### - [x] Task: Add the Highlight encode option
**Id:** d0a8df74-2634-4a2a-aaf8-4ae3e7b36d03
**Repo:** xclconfig
**Depends on:**
- 90d79027-5621-4838-873c-331c3f9a6268 — Add the highlight package with the tokeniser and renderer interface
**Execution:** agent

Add `xcl.Highlight(renderer)` to the encoder's options. When it is given, the shared encode path passes its finished text through the renderer. `EncodeEntity` and `EncodeSavedEntity` both gain highlighting, including for text that shows references as written. Without the option, nothing about encoding changes.

*Technical detail:* [context.md#task-add-the-highlight-encode-option](./context.md#task-add-the-highlight-encode-option)

**Acceptance criteria**:
- [x] Encoding without the option gives exactly today's text, with no colour codes.
- [x] Both encode functions return highlighted text when given a renderer, and removing the highlighting gives exactly the plain text.
- [x] With references shown as written, the first segment of each reference is highlighted as a reference.
- [x] A nil renderer behaves as if the option were not given.

#### - [x] Task: Switch the logging example to library highlighting
**Id:** 6302ae6c-f533-4fe8-8ee9-4026893c7eda
**Repo:** xclconfig
**Depends on:**
- 60e83712-3914-47f9-b90c-ef7ffc1f6c95 — Add the ANSI terminal renderer
- d0a8df74-2634-4a2a-aaf8-4ae3e7b36d03 — Add the Highlight encode option
**Execution:** agent

Remove prettylog's own regex highlighter and its tests, and have the handler ask the encoder for highlighting with the built-in terminal renderer whenever its writer shows colour. The example still decides for itself whether colour is shown, so output to a file or buffer stays plain, but it no longer contains any code that colours configuration.

*Technical detail:* [context.md#task-switch-the-logging-example-to-library-highlighting](./context.md#task-switch-the-logging-example-to-library-highlighting)

**Acceptance criteria**:
- [x] The logging example contains no highlighting code of its own.
- [x] When colour is on, the configuration it prints is coloured by the library, and stripping the colour gives the plain configuration.
- [x] When the writer does not show colour, the configuration is printed plain.
- [x] Review confirms the library still depends on no charmbracelet package.

#### - [x] Task: Document highlighting on the website encoding page
**Id:** d16236dc-043f-4404-9852-132f325ca8db
**Repo:** xcl-website
**Depends on:**
- 60e83712-3914-47f9-b90c-ef7ffc1f6c95 — Add the ANSI terminal renderer
- d0a8df74-2634-4a2a-aaf8-4ae3e7b36d03 — Add the Highlight encode option
**Execution:** agent

Add a "Highlighting" section to the configuration-text page. It shows how to turn highlighting on with the built-in terminal renderer, how to pass a VS Code theme and handle a bad one, and how to write a custom renderer from the scope names. It also says that colouring is the caller's decision, since xcl never checks for a terminal.

*Technical detail:* [context.md#task-document-highlighting-on-the-website-encoding-page](./context.md#task-document-highlighting-on-the-website-encoding-page)

**Acceptance criteria**:
- [x] The encoding page has a section showing how to enable highlighting, pass a theme and write a renderer.
- [x] The examples on the page match the library's real API names and signatures.
- [x] The site builds with the new section.

## Open Questions

- **Which module will prettylog be in when the migration task runs?** This depends on whether the sibling spec `20261006071142-506b8289-e2e-suite-and-real-world-examples` has landed and made prettylog self-contained. If prettylog is still inside the main module, migrate it in place. If it has its own module with a development `replace` pointing at the local xcl, migrate it there. If it pins a published xcl version that does not contain `highlight`, STOP and ask the user whether to release first or add a `replace`.
- **Does the scanner tokenise any grammar construct in a way that makes exact scope parity impossible?** This only shows up when the labelling functions are written against real tokens. The grammar is line-based regex, and examples are a block header whose `{` is on the next line, or a keyword used as an attribute name. If a construct cannot be labelled as the grammar labels it, STOP and ask the user: either accept the difference and document it in the parity tests, or change the grammar in xcl-vscode.

## Out of Scope

- **A built-in HTML renderer.** HTML is left to the presentation layer, such as the website's highlighter, which uses the editor grammar (spec Non-Goals). The `Renderer` interface makes one easy to write, and the docs show how.
- **Sharing a tokeniser with the planned `xcl fmt` tool.** This is deferred to when that tool is specified (issue #5, spec Non-Goals). The tokeniser stays unexported inside `highlight`, so nothing is committed to yet.
- **Terminal detection.** The library never checks whether output is a terminal (spec constraint). Applications such as prettylog decide.
- **256-colour output and colour-depth downsampling.** Theme colours are always written as 24-bit codes and the default theme as 16-colour codes. Converting a theme to 256 or 16 colours for limited terminals is not done.
- **Full TextMate scope-stack matching.** Each token carries one scope, as the spec's `(scope, text)` renderer contract fixes. Parent and descendant selectors match on their last element, and exclusion selectors are ignored.
- **Theme `include` chains and `.tmTheme` (plist) themes.** Only a self-contained VS Code JSON or JSONC colour theme is read. One that relies on `include` or points `tokenColors` at a `.tmTheme` file is reported as invalid.
- **Shipping named themes.** Apart from the 16-colour default, xcl bundles no editor themes. Callers supply their own theme file.
- **Changes to the xcl-vscode grammar or extension.** The grammar is the reference only. Any parity gap found during implementation is raised as an open question, not fixed there as part of this plan.
- **Highlighting outside the encoder in other examples.** Examples rewritten by the sibling specs that later print configuration are expected to use `xcl.Highlight`. That rewrite belongs to those specs.

## Changelog input

Notes for the epic-level changelog entry, written once after every spec in the epic is implemented:
- New public `highlight` package: `Renderer`, `RendererFunc`, `Text` and `Scope*` constants matching the xcl-vscode grammar scopes.
- New `xcl.Highlight(renderer)` encode option for `EncodeEntity` and `EncodeSavedEntity`; without it, encode output is unchanged.
- New `highlight.NewANSIRenderer` with `WithTheme` and `WithThemeFile` (16-colour default theme, 24-bit VS Code theme colours, font styles), plus `xcl.ErrInvalidTheme` / `InvalidThemeError` for unreadable or invalid themes.
- The prettylog example now uses the library's highlighting instead of its own regex highlighter.
- Breaking changes: none.

## Changelog

### 2026-10-06 — Task: Add the highlight package with the tokeniser and renderer interface

**What was done**: Added the public `highlight` package: `Renderer`, `RendererFunc`, `Text` and the 24 `Scope*` constants, plus an unexported tokeniser that lexes with `hclsyntax.LexConfig` and labels tokens with the xcl-vscode grammar's scopes using a frame stack (quote, heredoc, interpolation, directive) and byte spans for block labels and `%{ }` directives. Gaps between tokens are emitted from a byte cursor, so the text is unchanged by construction. Added parity fixtures, per-scope tests, identity/nil tests, a fuzz test, and `internal/testutil.StripANSI` for later tasks.

**Deviations**:
- Grammar reference is the xcl-vscode working-tree grammar (uncommitted edits: any identifier can be a labelled block type or a reference root), as the plan specified.
- Adjacent pieces with the same scope are merged into one renderer call (a label and its quotes arrive as one piece).
- Identifiers and whitespace inside `${ }` that no expression rule matches are passed with scope "" rather than `meta.interpolation.xcl` (the plan defines no constant for it).
- The keyword rule is applied as the grammar writes it: `resource|module|variable|output|local` not followed by `.` is `storage.type.xcl` anywhere outside strings, and a single label after a keyword with no `{` on the line is `entity.name.type.xcl` (the grammar's capture 2).
- `$${` in a quoted string is treated as text; the grammar would start an interpolation there. Not replicated.

**Files changed**:
- `xclconfig: highlight/doc.go`
- `xclconfig: highlight/highlight.go`
- `xclconfig: highlight/tokenize.go`
- `xclconfig: highlight/highlight_test.go`
- `xclconfig: highlight/tokenize_test.go`
- `xclconfig: highlight/testdata/sample.xcl`
- `xclconfig: highlight/testdata/references.xcl`
- `xclconfig: internal/testutil/ansi.go`

**Discoveries**: The HCL scanner includes a line comment's trailing newline in the comment token, and emits a heredoc's opener including its newline (`<<-EOF\n`) and its closer including leading indentation (`  EOF`), so these are split by hand. Tokens are never separated by newlines (newlines are tokens), so "next token" lookahead equals the grammar's `(?=\s*x)`.

### 2026-10-06 — Task: Add the invalid-theme error

**What was done**: Added `ErrInvalidTheme` and the `InvalidThemeError{Source, Reason, Err}` detail type (pointer receivers, `Unwrap() []error` returning the sentinel and the cause) to the `errors` package, and re-exported `xcl.ErrInvalidTheme` and the `xcl.InvalidThemeError` alias from `config.go` beside the encode errors.

**Deviations**: The plan named only the sentinel re-export; the detail type is also aliased from `xcl`, following the existing pattern for every other detail type, so callers can `errors.As` without importing the `errors` package.

**Files changed**:
- `xclconfig: errors/highlight_errors.go`
- `xclconfig: errors/highlight_errors_test.go`
- `xclconfig: config.go`
- `xclconfig: highlight_errors_test.go`

**Discoveries**: None.

### 2026-10-06 — Task: Read VS Code colour themes and match scopes against them

**What was done**: Added `stripJSONC` (comments and trailing commas removed outside strings) and the unexported theme: `parseTheme` reads a VS Code colour theme's `tokenColors` into rules (one per selector), and `theme.style` resolves a scope's colour and font style independently by dot-segment prefix matching, most segments winning and the later rule winning a tie, cached per scope behind a mutex. `defaultTheme` carries over prettylog's 16-colour palette as basic rules. Every failure is an `*InvalidThemeError` naming the path into the theme. Added `testdata/reference-theme.json` and the theme and JSONC tests.

**Deviations**:
- `#RGBA` colours are accepted as well as `#RGB`, `#RRGGBB` and `#RRGGBBAA` (VS Code accepts all four); alpha is ignored.
- A theme with no `tokenColors` (or `null`) loads and styles nothing, rather than failing.
- An entry that is not an object, or whose scope is neither a string nor a list of strings, is rejected as invalid.

**Files changed**:
- `xclconfig: highlight/jsonc.go`
- `xclconfig: highlight/theme.go`
- `xclconfig: highlight/testdata/reference-theme.json`
- `xclconfig: highlight/theme_test.go`
- `xclconfig: highlight/jsonc_test.go`

**Discoveries**: None.

### 2026-10-06 — Task: Add the ANSI terminal renderer

**What was done**: Added the public `ANSIRenderer` with `NewANSIRenderer`, `WithTheme(io.Reader)` and `WithThemeFile(path)`. With no theme it uses the 16-colour default theme; with a theme it writes `38;2;r;g;b` colours. Font styles are written as SGR 1/3/4/9 before the colour. Unlabelled and unmatched pieces are returned unchanged, and every non-empty line of a piece is wrapped separately so newlines stay outside escape codes. A file that cannot be opened, or any theme error, fails the constructor with `*InvalidThemeError` and a nil renderer.

**Deviations**: `ansi.go` also imports `io`, `os` and the `errors` package (the plan said only `strconv` and `strings`), needed for the options and the file-open error. The default theme leaves heredoc operators and markers, functions, members and operators in the terminal's default colour, as the palette carried over from prettylog has no rule for them.

**Files changed**:
- `xclconfig: highlight/ansi.go`
- `xclconfig: highlight/ansi_test.go`

**Discoveries**: None.

### 2026-10-06 — Task: Add the Highlight encode option

**What was done**: Added `xcl.Highlight(renderer highlight.Renderer)` to the closed `encodeOptions` set. `encodeEntity`, the single path behind `EncodeEntity` and `EncodeSavedEntity`, now passes the formatted `file.Bytes()` through `highlight.Text` as its last step when a renderer is set; a nil renderer, or no option, leaves the output exactly as before. Option and `EncodeEntity` doc comments name the new option. Added encoder integration tests.

**Deviations**: None.

**Files changed**:
- `xclconfig: encode.go`
- `xclconfig: encode_highlight_test.go`

**Discoveries**: None.

### 2026-10-06 — Task: Switch the logging example to library highlighting

**What was done**: Deleted prettylog's regex highlighter (`highlight.go`, `highlight_test.go`). `Handler` now decides colour for its writer with `lipgloss.NewRenderer(w).ColorProfile() != termenv.Ascii` and, when colour is on, adds `xcl.Highlight` with the default `highlight.NewANSIRenderer()` to the `EncodeSavedEntity` options; `writeConfiguration` takes the options and indents the (possibly coloured) text. Updated the package and `Handler` doc comments, replacing the stale `ExamplePlugin` log line with the template plugin's. Added forced-colour and plain-writer handler tests.

**Deviations**: prettylog had already become its own module (sibling e2e spec) with a development `replace` to the local xcl, so the migration was made there and needed no release; the plan's open question resolved without a stop.

**Files changed**:
- `xclconfig: example/prettylog/highlight.go` (deleted)
- `xclconfig: example/prettylog/highlight_test.go` (deleted)
- `xclconfig: example/prettylog/prettylog.go`
- `xclconfig: example/prettylog/prettylog_test.go`

**Discoveries**: lipgloss's renderer resolves colour through termenv's `EnvColorProfile`, which turns colour on for any writer when `CLICOLOR_FORCE` is set (and not `0`), and treats an empty `NO_COLOR` as unset; tests force colour with `t.Setenv("CLICOLOR_FORCE", "1")`.

### 2026-10-06 — Task: Document highlighting on the website encoding page

**What was done**: Added a "Highlighting" section to `src/pages/configuration-text.mdx`, before "For reading, not reprocessing", covering turning highlighting on with `highlight.NewANSIRenderer` and `xcl.Highlight`, the caller's own terminal decision, using a VS Code theme with `WithThemeFile`/`WithTheme` and handling `xcl.ErrInvalidTheme`, and writing a renderer with `highlight.RendererFunc`, the scope names and the `highlight.Scope*` constants. The page intro mentions colouring. Also fixed the stale `prettylog.Handler` snippet on `src/pages/events.mdx` (it showed a two-argument signature) and noted that the example colours configuration through `xcl.Highlight`.

**Deviations**: The `events.mdx` fix is outside this task's listed files; it was reported by an earlier spec as stale and falls to this spec because this spec changes prettylog's `Handler`.

**Files changed**:
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/pages/events.mdx`

**Discoveries**: The website worktree has no `node_modules`; the build was run against the main checkout's installed modules through a temporary symlink, removed afterwards.
