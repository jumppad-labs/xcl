---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Context: 20261023074926-eaa51527-printing-and-highlighting-core

## Current State Analysis

- Highlighting is scope-based and VS Code-shaped. `highlight/tokenize.go:111` `Pieces(src) []Piece` labels configuration text with TextMate scope strings (`highlight/highlight.go:9-117`). `Renderer`/`RendererFunc`/`PieceFunc`/`Text` (`highlight/highlight.go:119-153`) and `ANSIRenderer`/`NewANSI`/`WithTheme`/`WithThemeFile` (`highlight/ansi.go`) return `[]byte`. Themes are VS Code JSON matched by TextMate rules (`highlight/theme.go:42-130`) and replace, rather than extend, a 16-colour default (`highlight/theme.go:124-150`).
- Error reports: `xcl.DiagnosticPieces(diagnostics, width)` (`diagnostics.go:36`) lays out severity, summary, location, numbered source and wrapped detail, reached only through `errors.As` → `ConfigError.Diagnostics()` → `DiagnosticPieces` → `Render`. `Diagnostic.Error()` (`errors/diagnostic.go:136`) and `ConfigError.Error()` (`errors/config_error.go:44`) are one line or one line each. Neither implements `fmt.Formatter`.
- Source context: `errors.NewDiagnostic` (`errors/diagnostic.go:70-89`) captures only the lines the subject and context cover (`linesOf`, :93-117), from the source bytes the parser already holds.
- Change summaries: `diff.(*Diff).Pieces()` (`diff/render.go:89`) labels markers with `markup.*.diff` scopes and lexes headers and values with `highlight.Pieces`.
- Callers of the old API: `coreservice/lookups.go:190-197` (`highlight=true` → `highlight.ANSI`); examples `example/plugin/inspect.go`, `example/plugin/plan.go`, `example/prettylog/prettylog.go` (lipgloss decides colour); tests in the root package, `diff/`, `internal/parser/`, `viper/`, `highlight/`.
- The examples print errors as `error: %s` (`example/configonly/main.go:44`, `example/plugin/main.go:113`, `example/remoteplugin/main.go:94`).
- Import graph constraint: `highlight` imports `errors` (for `InvalidThemeError`). `errors` imports only `internal/xcl` and `go-wordwrap` (`conventions/shared-errors-package.md`).
- Verified by a probe during planning: `Config.Validate` returns `*errors.ConfigError` unwrapped, and an unsupported nested block yields `Unsupported block type` with `Entity` and `Field` set (`variable.name`, `timeout[0]`).
- No zap dependency (`go.mod`). go-wordwrap v1.0.1 and go-textseg v15 are present. The latest tag is v0.11.1.

## Per-Task Technical Notes

### Task: Lay out error reports in one private package

Requirement attribution: "One-call error printing with context" (layout), "Every problem in a wrapped error is printed" (walk), "Detail wraps to a width" (Wrap). Repo xcl.

File changes:
- `internal/report/report.go` (new): `Part` (Severity, Summary, Location, Gutter, Source, Detail, Plain), `Segment{Part, Text, Warning, Fault [2]int, HasFault}`, `Problem{Warning bool; Summary, Detail, Entity, Field string; Subject *hcl.Range; File string; Start int; Lines []string; HasSource bool}`, and `Layout(p Problem) []Segment`. The wording is taken verbatim from `diagnostics.go:67-102`:
  - Severity "Error"/"Warning", then Summary `": "+summary`, then Plain "\n".
  - With a subject: Plain "\n", then Location `fmt.Sprintf("  on %s line %d", file, line)` plus `", in "+entity` and `", field "+field`, plus ":", then Plain "\n".
  - Without source: Plain "  (source code not available)\n". Otherwise, per line: Gutter `fmt.Sprintf("  %4d | ", n)`, then Source (the whole line, with Fault from `fault()`), then Plain "\n".
  - With detail: Plain "\n", then Detail (unwrapped), then Plain "\n".
  - Finally Plain "\n".
  Plain text must stay byte-identical to today, so the location's trailing "\n" moves into a Plain segment.
- `internal/report/report.go`: move `fault`, `byteAt` (textseg grapheme columns) from `diagnostics.go:142-183` here unchanged.
- `internal/report/wrap.go` (new): `Wrap(detail string, width int) string` = `wordwrap.WrapString(detail, uint(width))` when width > 0, else unchanged. This is the logic from `diagnostics.go:91-96`.
- `internal/report/walk.go` (new): `Walk(err error, isProblem func(error) bool, visit func(leaf error, problem bool))`. Depth-first, in order. It stops at a leaf: an error with no Unwrap, or one the caller's `isProblem` predicate says is a diagnostic. Callers pass `func(e error) bool { _, ok := e.(*xclerrors.Diagnostic); return ok }`, so the package never imports `errors`. Recurse into `Unwrap() []error` children, and into `Unwrap() error` only when the subtree contains a problem; otherwise the wrapper itself is a leaf, so its message, wrapping context included, is kept. Add a helper `Contains(err, isProblem) bool`.
- `internal/report/*_test.go` (new): one test each for:
  - an error layout
  - a warning
  - entity only, and entity plus field
  - no subject
  - no source
  - multi-line fault
  - empty fault, shown as one character
  - grapheme columns
  - Wrap at 40, and at 0
  - Walk through `fmt.Errorf("%w")`, through `errors.Join`, through a struct with `Unwrap() []error`, and not into a problem's cause
  - Walk on an error with no problem, visited as one leaf
  Port the expectations from `diagnostics_test.go:53-182`.
- `diagnostics.go:36-215`: rebuild `DiagnosticPieces` on `report.Layout` and keep its signature. Lex Source segments with `highlight.Pieces` plus `overlay`, and map the other parts to today's scopes (Severity → ScopeError/ScopeWarning, everything else ""). Apply `report.Wrap` to the Detail segment with `width`. `diagnostics_test.go` must pass unchanged. This file is deleted later, by "Add one-call printing".

Complexity: Medium
Token estimate: ~30k tokens
Agent strategy: Single agent, sequential execution: layout first, then the DiagnosticPieces rebase, then tests.

### Task: Capture two lines of context around each fault

Requirement attribution: "Surrounding lines are restored". Constraint: use source already read. Repo xcl.

File changes:
- `errors/diagnostic.go:93-117` (`linesOf`): after computing `first`/`last` for the covered range (keeping the `r.End.Column <= 1` rule at :100-102), widen with `first = max(1, first-2)` and `last = min(len(lines), last+2)`. When `src` ends with "\n", `strings.Split` yields a final empty element, so take `len(lines)` without that trailing empty element: a fault on the last real line must not show an empty "line N+1". Return `SourceLines{Start: first, ...}`. `NewDiagnostic` (:70-89) is unchanged; it still reads only from `sources`.
- `errors/diagnostic.go:19-49`: update the `Diagnostic`/`Source` doc comment to say up to two lines either side of the fault, clipped at the file's ends.
- `errors/diagnostic_test.go:67-124`: update expectations. :76 now expects `Start: 1` with the three lines of `reportSource`, and :89-109 change similarly. Add separate tests for: middle of a file (±2), first line of a three-line file (lines 1–3), last line (that line and the two before), a file ending without a newline, a file ending with a newline (no phantom empty line), and a multi-line fault (±2 around the span).
- `internal/parser/validate_test.go:758-760`: `Start` moves from 5 to 3, and `Lines[0]` no longer holds "colour". Assert that `Lines` contains the line with "colour" at index `5-Start`.
- `internal/parser/parse_test.go:966-968`: `Start` 7 becomes 5. Assert accordingly.
- `viper/xclviper_test.go:154` and `viper/xclviper_directory_test.go:147`: report expectations gain the context lines. Adjust the expected strings; the call sites themselves move in a later task.
- `diagnostics_test.go`: expected reports gain the surrounding lines of `reportSource` (lines 1–3 for a fault on line 2). Update the expected strings.

Complexity: Low
Token estimate: ~15k tokens
Agent strategy: Single agent, sequential execution.

### Task: Print the full report through detailed error formatting

Requirement attribution: "Ordinary messages stay one line", "Reports in structured logs". Repo xcl.

File changes:
- `errors/format.go` (new): `func (d *Diagnostic) Format(s fmt.State, verb rune)` and `func (p *ConfigError) Format(s fmt.State, verb rune)`.
  - `'v'` with `s.Flag('+')` → the plain report.
  - `'v'` and `'s'` → `Error()`.
  - `'q'` → `strconv.Quote(Error())`.
  - Any other verb → `%!<verb>(…)`, as fmt does.
  The plain report is: `report.Walk` over the receiver, then for each problem leaf `report.Layout(problemOf(d))` with each Detail segment passed through `report.Wrap(…, 80)`, all segments' text concatenated; for each non-problem leaf `leaf.Error()+"\n"`. When there is no problem in the tree at all, the result is `Error()+"\n"`. `problemOf(d *Diagnostic) report.Problem` is an unexported converter: Warning=`d.IsWarning()`, Summary, Detail, Entity, Field, Subject, and the Source fields when `d.Source != nil`. The `errors` import list grows only by `internal/report` (`conventions/shared-errors-package.md`).
- `errors/diagnostic.go:130-135`: doc for `Error()` notes that `%+v` gives the report and works only on the unwrapped error. `PrintError` is the dependable route.
- `errors/format_test.go` (new):
  - `%v` gives one line
  - `%s` gives one line
  - `%q` gives the quoted line
  - `%+v` on a Diagnostic gives the report with context
  - `%+v` on a ConfigError holding two diagnostics plus a plain error gives two reports and the message line
  - `%+v` wraps detail at 80
- `errors/format_zap_test.go` (new, `package errors_test`): build a `zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(cfg), zapcore.AddSync(&buf), zap.DebugLevel))`, log `zap.Error(cfgErr)` where `cfgErr` is a `*ConfigError` holding a Diagnostic made by `NewDiagnostic`, unmarshal the JSON, and require `errorVerbose` to equal `fmt.Sprintf("%+v", cfgErr)` and to contain the gutter line. The root-level AC with a real Apply is covered with zap in "Add one-call printing".
- `go.mod`, `go.sum`: add `go.uber.org/zap v1.27.0` (test-only). `go mod tidy` places it in the first require block.

Complexity: Medium
Token estimate: ~20k tokens
Agent strategy: Single agent, sequential execution.

### Task: Capture today's plain output of resources and change summaries

Requirement attribution: AC "Resource and change summary" (match the current release). Repo xcl.

File changes:
- `testdata/print/` (new, root package test data). Generate it with a throwaway program or test run against the current code, before any change: `entity_<name>.txt` holding `EncodeEntity(entity, IncludeComputed())` output for 3–4 fixed entities, and `diff_<name>.txt` holding `highlight.Text.Render(d.Pieces())` for fixed diffs.
  - Entities: reuse the registered fixture types from root tests (see `encode_test.go`, `encode_highlight_test.go:44-160` and `internal/testutil/entities.go`). Cover a resource with nested blocks and lists, a resource with computed fields and `ShowReferences`, and a sensitive field.
  - Diffs: reuse the literal in `diff/example_test.go:10-35`, plus a no-change diff and a replace-with-dependency-reason diff from `diff/render_test.go`.
- `print_fixtures_test.go` (new, root `package xcl`): declares the fixed entities and diffs as test helpers (`printFixtureEntities()`, `printFixtureDiffs()`), so the comparison tests in "Add one-call printing" use exactly the inputs that were captured. Include a test that the captured files equal today's `EncodeEntity` and `highlight.Text.Render(d.Pieces())` output. That test is rewritten to `PrintEntity`/`PrintDiff` later.

Complexity: Low
Token estimate: ~12k tokens
Agent strategy: Single agent, sequential execution. This must run before any other Milestone 2 task changes code.

### Task: Rebuild highlighting around kinds, themes and formatters

Requirement attribution: "Choice of output format", "Websites restyle HTML output", "Stylesheets from themes", "Themes extend the default", "Themes can be checked", "Editor themes can be imported", "Custom output formats", "Colour chosen from the destination on request", "Labelled text for extenders" (Tokenize). Constraints: no chroma, no cgo, no new dependency; lexing stays in the core. Repo xcl.

This task deletes the old `highlight` API, so packages outside `highlight` (diff, root, coreservice, internal/parser tests, viper, examples) stop compiling until "Label change summaries with kinds", "Add one-call printing" and "Move every remaining caller" land. Verify this task with `go test ./highlight/... ./errors/...` only. The whole module is green again at the end of "Move every remaining caller".

File changes:
- `highlight/kind.go` (new):
  - `type Kind int` and every constant from the design table: Text 0; Block 100, BlockType 101, NestedBlock 102; Label 200, TypeLabel 201, NameLabel 202; Attribute 300; String 400, StringEscape 401, Heredoc 402, HeredocMarker 403, HeredocOperator 404; Number 500; Constant 600; Function 700, BuiltinFunction 701; Reference 800, Member 801; Operator 900; Punctuation 1000, InterpolationBegin 1001, InterpolationEnd 1002, TemplateDirective 1003, Accessor 1004; Comment 1100, LineComment 1101, HashComment 1102, BlockComment 1103; Diff 1200, Inserted 1201, Deleted 1202, Changed 1203; Problem 1300, Error 1301, Warning 1302; Report 1400, Summary 1401, Location 1402, Gutter 1403, Detail 1404.
  - `Parent()` = `k/100*100`.
  - `String()` is the name, or `Kind(n)` for an unknown kind.
  - `Class()` is `"xcl-"` plus the kebab-cased name (BlockType → `xcl-block-type`, StringEscape → `xcl-string-escape`, HeredocMarker → `xcl-heredoc-marker`). An unknown kind gives its parent's class, or "" when the parent is unknown too.
  - `Kinds() []Kind` lists every known kind in numeric order (used by CSS, classes.txt and the VS Code import).
  - `Scope()` uses the grammar scopes from `highlight/highlight.go:9-117` for the kinds the tokenizer emits (BlockType storage.type.xcl, TypeLabel entity.name.type.xcl, NameLabel entity.name.tag.xcl, NestedBlock entity.name.function.block.xcl, Attribute variable.other.property.xcl, String string.quoted.double.xcl, StringEscape constant.character.escape.xcl, InterpolationBegin/End punctuation.section.interpolation.begin/end.xcl, Heredoc string.unquoted.heredoc.xcl, HeredocOperator keyword.operator.heredoc.xcl, HeredocMarker keyword.control.heredoc.xcl, TemplateDirective meta.interpolation.template.xcl, Number constant.numeric.xcl, Constant constant.language.xcl, BuiltinFunction support.function.builtin.xcl, Function entity.name.function.xcl, Reference support.class.reference.xcl, Accessor punctuation.accessor.xcl, Member variable.other.member.xcl, Operator keyword.operator.xcl, LineComment comment.line.double-slash.xcl, HashComment comment.line.number-sign.xcl, BlockComment comment.block.xcl, Inserted markup.inserted.diff, Deleted markup.deleted.diff, Changed markup.changed.diff, Error invalid.illegal.diagnostic, Warning invalid.deprecated.diagnostic). Parents and report kinds get: Text "", Block meta.block.xcl, Label entity.name.xcl, Punctuation punctuation.xcl, Comment comment.xcl, Diff markup.diff, Problem invalid.diagnostic, Report meta.report.xcl, Summary markup.heading.report.xcl, Location meta.report.location.xcl, Gutter comment.line.gutter.report.xcl, Detail meta.report.detail.xcl. An unknown kind gives its parent's scope.
- `highlight/tokenize.go:1-786`: rename `Piece` → `Token{Kind, Text}` and `Pieces` → `Tokenize`. `emit(scope string, …)` becomes `emit(kind Kind, …)`, and `span.scope` becomes `span.kind`. Replace each `Scope*` use (:275-765) with the matching Kind. `ScopeTemplateInterpolation` → `TemplateDirective`. Unlabelled is `Text`. Behaviour is otherwise untouched (byte coverage, merge neighbours, no empty tokens).
- `highlight/highlight.go`: delete it (Scope* constants, Renderer, RendererFunc, PieceFunc, Text).
- `highlight/ansi.go`: delete it (ANSIRenderer, ANSIOption, WithTheme, WithThemeFile, ANSI, NewANSI). Its `sgrParameters` logic is reused in `terminal.go`.
- `highlight/style.go` (new):
  - `Style`, `Colour`, `ParseStyle`.
  - Words: `bold`, `italic`, `underline`, `dim`, `#rrggbb` (exactly 6 hex digits, case-insensitive, stored lower-case), `bg:#rrggbb`, and the named colours black, red, green, yellow, blue, magenta, cyan, white and `bright-` plus each. A later word of the same slot wins. Unknown words are skipped.
  - `Colour.Hex()` maps named colours through the palette black #000000, red #cd3131, green #0dbc79, yellow #e5e510, blue #2472c8, magenta #bc3fbc, cyan #11a8cd, white #e5e5e5, bright-black #666666, bright-red #f14c4c, bright-green #23d18b, bright-yellow #f5f543, bright-blue #3b8eea, bright-magenta #d670d6, bright-cyan #29b8db, bright-white #ffffff. Hex passes through.
  - `unknownWords(s string) []string` is used by Validate.
- `highlight/theme.go` (rewritten):
  - `type Theme map[Kind]string`.
  - `With` copies, then overrides.
  - `Style(k)`: if `t[k]` is non-empty, parse it; else if `t[k.Parent()]` is non-empty, parse that; else parse `t[Text]`.
  - `Validate`: walk kinds in numeric order and words in order, building `&xclerrors.InvalidStyleError{Kind: k.String(), Style: s, Word: w}` for each unknown word, and return `errors.Join(...)` or nil.
  - `DefaultTheme = Theme{Block: "bold magenta", Label: "green", TypeLabel: "cyan", Attribute: "blue", String: "yellow", StringEscape: "bright-magenta", Number: "bright-magenta", Constant: "bright-magenta", Reference: "cyan", Comment: "italic bright-black", Inserted: "green", Deleted: "red", Changed: "yellow", Error: "bold red", Warning: "bold yellow", Summary: "bold", Gutter: "dim"}`. It keeps `\x1b[1;35m` for block types and `\x1b[34m` for attributes, as `example/prettylog/prettylog_test.go:494-497` asserts.
- `highlight/vscode.go` (new, from `highlight/theme.go:18-373`): keep `rule`, `selectorMatches`, `outranks`, the resolver (`theme.style`), `parseTheme`, `parseTokenColor`, `parseScope`, `lastElement`, `parseColour` and `parseFontStyle` (strikethrough still parses but is dropped from the output), all unexported. `LoadVSCodeTheme(r io.Reader) (Theme, error)` parses with source "theme", then for each `Kinds()` entry with a non-empty `Scope()`, resolves the style and, if a colour or font style is set, writes `"bold italic underline #rrggbb"` (only the parts present, in that order). The old default 16-colour `defaultTheme()` (:124-150) is deleted. Errors keep the `InvalidThemeError` reasons.
- `highlight/jsonc.go`: unchanged.
- `highlight/formatter.go` (new): `Formatter`, `FormatterFunc` (its `Format` calls the function), `Plain` (writes each text and ignores the theme). Formatters write through a single `bufio`-free `io.Writer` and return the first write error.
- `highlight/terminal.go` (new): `Terminal`. For each token, `p := sgr(theme.Style(kind))`. If `p` is "", write the text. Otherwise split on "\n" and write each non-empty line as `"\x1b["+p+"m"+line+"\x1b[0m"`, rejoined with "\n". Parameter order: 1 bold, 2 dim, 3 italic, 4 underline, then the foreground (named 30–37/90–97, hex `38;2;r;g;b`), then the background (`48;2;r;g;b`).
- `highlight/html.go` (new):
  - `HTML`: writes `<pre class="xcl">`. A Text token (or a kind whose `Class()` is "") is written as escaped text. Any other token is written as `<span class="P C">` (P is the parent class, present only when `kind.Parent() != kind` and the parent is known) followed by the escaped text and `</span>`. Then `</pre>`, with no trailing newline.
  - `HTMLInline`: `<pre>` gets a `style="…"` attribute only when `theme.Style(Text)` sets something. A token whose resolved style is empty, or a Text token, is written plain. Otherwise `<span style="…">`. The CSS declaration order is color, background-color, font-weight:bold, font-style:italic, text-decoration:underline, opacity:0.5, joined with "; ".
  - Escaping for both: & → &amp;, < → &lt;, > → &gt;, " → &quot;, ' → &#39;.
- `highlight/svg.go` (new): `SVG`.
  - Split the joined token stream into lines on "\n", dropping one trailing empty line. Columns are counted in runes.
  - Write `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 W H" font-family="monospace" font-size="10"` + (` fill="#hex"` when Text has a foreground) + `>` and "\n", where W = 16 + 6×maxColumns and H = 16 + 14×lines.
  - Write `<rect width="W" height="H" fill="#bg"/>` and "\n" when Text has `bg:`.
  - For each line i, write `<text x="8" y="(19+14i)" xml:space="preserve">` … `</text>` and "\n". Inside it, a token piece with a non-empty resolved style is `<tspan` plus ` fill="#hex"`, ` font-weight="bold"`, ` font-style="italic"`, ` text-decoration="underline"` and ` opacity="0.5"` in that order, then `>`, the escaped text and `</tspan>`. Otherwise the escaped text alone. A token background is ignored inside text.
  - Finish with `</svg>` and "\n".
- `highlight/css.go` (new): `WriteCSS(w, theme)`.
  - If `theme[Text]` is non-empty, write `.xcl { <decls> }` and "\n" first.
  - Then for each kind in `Kinds()` order (parents before children) with a non-empty `theme[k]`, write `.xcl .<k.Class()> { <decls> }` and "\n". The decls are the declarations of `ParseStyle(theme[k])`, then resets for each property set by the reference style that this style does not set: `color: inherit`, `background-color: transparent`, `font-weight: normal`, `font-style: normal`, `text-decoration: none`, `opacity: 1`. The reference style is `theme[k.Parent()]` for a child, else `theme[Text]`.
  - Colours always go through `Colour.Hex()`. `dim` becomes `opacity: 0.5`.
- `highlight/for.go` (new): `For(w io.Writer) Formatter`. Return Plain unless `w` is an `*os.File` whose `Stat()` mode has `os.ModeCharDevice`, and `os.Getenv("NO_COLOR") == ""` (any non-empty value disables colour, per no-color.org). Then return Terminal.
- `highlight/doc.go`: rewrite for tokens, themes and formatters, with the "colour is the caller's choice" paragraph kept.
- `errors/highlight_errors.go`: add `ErrInvalidStyle` and `InvalidStyleError{Kind, Style, Word}` (pointer receiver; `Error()` is `style %q for %s: unknown word %q`; `Unwrap()` returns ErrInvalidStyle). Update the ErrInvalidTheme doc to name `highlight.LoadVSCodeTheme` instead of `NewANSI`. Add `errors/highlight_errors_test.go` cases.
- Tests (all `package highlight_test`, one behaviour each, no tables):
  - `highlight/tokenize_test.go`: replace the `markers` PieceFunc (:13-20) with a test-local `highlight.FormatterFunc` that writes `[scope]text[/scope]` using `token.Kind.Scope()` (Text written raw). Keep every expected string. These are the xcl-vscode parity tests.
  - `highlight/highlight_test.go`: becomes Plain and well-formedness tests over `Tokenize` (`joined`, `requireWellFormed` kept).
  - `highlight/kind_test.go`: Parent, String, Class, Scope, unknown kinds.
  - `highlight/style_test.go`: each word, last wins, unknown skipped, Hex.
  - `highlight/theme_test.go` (rewritten): Style fallback; the comment-only theme colours line, hash and block comments; LineComment's own style; unmentioned kinds keep the default; With does not mutate; Validate reports a word and matches `ErrInvalidStyle`; Validate on DefaultTheme is nil; formatting with an invalid word succeeds.
  - `highlight/vscode_test.go`: port the parse and invalid-theme cases from the old `theme_test.go`/`ansi_test.go`. Properties `#ff8800` → `Theme[Attribute] == "#ff8800"`. Use `testdata/reference-theme.json`.
  - `highlight/terminal_test.go`: codes present, reset before each newline, `testutil.StripANSI` equals Plain, a multi-line token is coloured per line.
  - `highlight/html_test.go`: container class, parent and kind classes, no span for Text, escaping of `<&"'>`, no `style=`; inline: styles, no `class=`, no span for an unstyled kind.
  - `highlight/svg_test.go`: `xml.NewDecoder` reads to EOF without error; the `<text>` count equals the lines; the viewBox width is at least 16 + 6×longest.
  - `highlight/css_test.go`: a test-local parser splits on `}` and checks each rule is `selector { decl; … }` with the selector starting `.xcl`; parent before child; a diff between DefaultTheme CSS and `DefaultTheme.With(Theme{Attribute: "#ff8800"})` CSS is exactly the attribute line.
  - `highlight/for_test.go`: a bytes.Buffer gives Plain; a temp regular file gives Plain; `/dev/null` (char device; skip if absent) gives Terminal; `/dev/null` with `t.Setenv("NO_COLOR","1")` gives Plain.
  - `highlight/ansi_test.go`: delete it, its cases ported above.
  - `highlight/jsonc_test.go`: unchanged.

Complexity: High
Token estimate: ~90k tokens
Agent strategy: Parallel analysis, sequential integration. One agent ports the tokenizer, kinds and parity tests. A second writes style, theme, VS Code import and the errors additions. A third writes the formatters, CSS and For against the agreed `Kind`/`Theme`/`Style` API. Integrate and run the highlight tests last.

### Task: Label change summaries with kinds

Requirement attribution: "One-call printing of resources and change summaries" (layout labels), "Labelled text for extenders". Repo xcl.

File changes:
- `diff/render.go:36-150`: rename `Pieces()` → `Tokens() []highlight.Token`, and the doc comment accordingly (mentioning `xcl.PrintDiff` for printing). `layout.pieces` → `tokens`; `add(scope string, …)` → `add(kind highlight.Kind, …)`; `plain` uses `highlight.Text`; `hcl` (:137-142) iterates `highlight.Tokenize`. :146 `ScopeHashComment` → `highlight.HashComment`.
- `diff/render.go:353-361` (`scopeFor`) → `kindFor` returning `highlight.Inserted`/`Deleted`/`Changed`.
- `diff/pieces_test.go` → rename to `diff/tokens_test.go` and assert Kinds instead of scopes.
- `diff/render_test.go`: replace `highlight.Text.Render(d.Pieces())` with a local `plain(d)` helper that joins `d.Tokens()` texts. Expected strings are unchanged.
- `diff/example_test.go:10-62`: rename to `ExampleDiff_Tokens`, printing the joined token text. The Output block is unchanged.

Complexity: Low
Token estimate: ~15k tokens
Agent strategy: Single agent, sequential execution.

### Task: Add one-call printing of errors, resources and change summaries

Requirement attribution: "One-call error printing with context", "Every problem in a wrapped error is printed", "Detail wraps to a width", "One-call printing of resources and change summaries", "Labelled text for extenders", "Custom output formats" (accepted by print functions). Repo xcl.

File changes:
- `print.go` (new):
  - `printOptions{formatter highlight.Formatter; theme highlight.Theme; width int; encode []EncodeOption}`, defaulting to Plain, DefaultTheme and 80.
  - `PrintOption` interface with `applyPrint(*printOptions)`, and the unexported `printOptionFunc`.
  - `WithFormatter`, `WithTheme` and `WithWidth` (a negative width is treated as 0).
  - `func (o EncodeOption) applyPrint(p *printOptions) { p.encode = append(p.encode, o) }`.
  - `PrintError(w, err, options...)`: if `err == nil`, write nothing and return nil. Otherwise take `tokens := ErrorTokens(err)`, rewrite each `highlight.Detail` token's text with `report.Wrap(text, width)`, and call `formatter.Format(w, theme, tokens)`.
  - `PrintEntity(w, entity, options...)`: `EntityTokens(entity, p.encode...)`, then Format.
  - `PrintDiff(w, d, options...)`: `d.Tokens()` (a nil d is handled by Tokens), then Format.
  - Godoc examples follow the design (`highlight.For(os.Stderr)`, `template.HTML`).
- `diagnostics.go`: replace the whole file.
  - `ErrorTokens(err error) []highlight.Token`. If `err == nil`, return nil. If `!report.Contains(err, isDiagnostic)`, return `[{Text, err.Error()+"\n"}]`. Otherwise call `report.Walk`: a diagnostic leaf contributes its laid-out tokens; any other leaf contributes `{Text, leaf.Error()+"\n"}`.
  - Diagnostic tokens come from `report.Layout(problemOf(d))`, mapped: Severity → Error/Warning; Summary → Summary; Location → Location; Gutter → Gutter; Source → `highlight.Tokenize(line)` with `overlay` (kept from today's :190-215, over Tokens) of Error/Warning on the Fault range; Detail → Detail; Plain → Text. Merge same-kind neighbours, as `report.add` did (:54-65).
  - `problemOf` duplicates the `errors` converter. That is deliberate: the internal/report split forbids sharing it, and the `%+v` equality test guards against drift.
  - Delete `DiagnosticPieces` and the `report` type.
- `encode.go:104-131`: doc for `EncodeEntity` points at `PrintEntity`/`EntityTokens` instead of `highlight.ANSI`. Add `EntityTokens(entity any, options ...EncodeOption) ([]highlight.Token, error)` = `EncodeEntity` then `highlight.Tokenize`, returning the error unchanged.
- `config.go:114-117`: ErrInvalidTheme doc names `highlight.LoadVSCodeTheme`. Add `ErrInvalidStyle = xclerrors.ErrInvalidStyle` beside it. At :184 add `InvalidStyleError = xclerrors.InvalidStyleError` to the alias block. `highlight_errors_test.go` gains re-export tests.
- `diagnostics_test.go`: port all 16 tests (:53-188) to `ErrorTokens`/`PrintError`. Use a test-local marker `FormatterFunc` for label assertions (`[Kind]text[/Kind]`), and plain `PrintError` with `WithWidth(0)` for the layout strings. They now include context lines.
- `print_error_test.go` (new):
  - unsupported block type on line 7: write a fixture under `testdata/print/bad/main.xcl`, nine or more lines, with an unsupported nested block inside a resource on line 7, as in the design sample; `NewConfig`, then `Validate`/`Apply` on it, then `PrintError` into a buffer. Assert the first line, a location line naming the file, line 7, the entity and the field, gutters 5–9, and that the last non-empty line is the detail.
  - first-line fault in a three-line file (lines 1–3)
  - last-line fault
  - two problems wrapped with `fmt.Errorf("loading: %w", err)`, using two resource blocks missing name labels per the HCL-halts gotcha; both reports in order
  - non-configuration error, written exactly as `"boom\n"`
  - `fmt.Sprintf("%+v", cfgErr)` equals plain `PrintError` output
  - `fmt.Sprintf("%v")` holds no newline
  - detail over 80 characters, with no detail line over 80 by default
  - the same with no line over 40 at `WithWidth(40)`
  - the same on a single line at `WithWidth(0)`
  - Terminal: codes present, `\x1b[0m\n` before each coloured line's newline, `StripANSI` equals plain
  - HTML: a single `<pre class="xcl">`, the fault in `<span class="xcl-problem xcl-error">`, no `style=`, configuration text escaped (the fixture holds `<`)
  - HTMLInline: `style=` present, no `class=`
  - custom `FormatterFunc` wrapping each token in `[` `]`: called once with all tokens, output in the given buffer
  - `ErrorTokens` joined equals plain
  - zap: log the error returned by `Apply` with `zap.Error`; `errorVerbose` holds the plain report. Validate and Apply return the `*ConfigError` unwrapped, which was verified during planning.
- `print_entity_test.go` (new):
  - each fixture from `print_fixtures_test.go` printed with `IncludeComputed()` equals the captured `testdata/print/entity_*.txt`
  - `EntityTokens` joined equals plain
  - encode and print options mix in one call
  - SVG on a resource: `xml` decode ok, one `<text>` per line, viewBox width fits the longest line
  - the VS Code theme AC: properties orange gives the attribute name in `38;2;255;136;0` under Terminal
  - an unencodable entity returns `ErrNotEncodable` and writes nothing
- `print_diff_test.go` (new): each fixture diff equals the captured `testdata/print/diff_*.txt`; `Diff.Tokens` joined equals plain; a nil diff prints the no-change summary.
- `print_fixtures_test.go`: rewrite the capture-equality test to use `PrintEntity`/`PrintDiff`.
- `encode_highlight_test.go`: port the 8 tests (:44-160) to `EntityTokens`/`PrintEntity` with Terminal/Plain.

Complexity: High
Token estimate: ~70k tokens
Agent strategy: Parallel analysis, sequential integration. One agent writes print.go, ErrorTokens and the error tests. A second writes EntityTokens, the entity and diff tests and the encode test port. Integrate, then run root tests.

### Task: Move every remaining caller to the new printing

Requirement attribution: constraint "previous API must be removed"; keeps the engine working. Repo xcl.

File changes:
- `coreservice/lookups.go:189-197` (`colour`): on `true`, format `highlight.Tokenize(text)` with `highlight.Terminal` and `highlight.DefaultTheme` into a `bytes.Buffer`. Update the comment to say the field is legacy. :134 and :151 are unchanged. `coreservice/lookups_test.go:167,424,436` should pass. Adjust only if they assert specific SGR bytes that the new theme changed.
- `config_singleton_lifecycle_test.go:69`, `sensitive_leak_test.go:635-672` (NewANSI → `highlight.Terminal`), `internal/parser/decide_test.go:376`, `internal/parser/diff_unknown_test.go:583,609`: replace `highlight.Text.Render(x.Pieces())` with a joined-`Tokens()` helper, or `xcl.PrintDiff` where the package allows. `internal/parser` cannot import the root package, so join `Tokens()` there.
- `viper/xclviper_test.go:154`, `viper/xclviper_directory_test.go:147`: replace `DiagnosticPieces` with `xcl.PrintError(&buf, d, xcl.WithWidth(0))` (d is a `*Diagnostic`).
- `example/plugin/inspect.go:30-41`: `formatter := highlight.Plain`; if lipgloss reports a colour profile, use `highlight.Terminal`. Then `xcl.PrintEntity(out, entity, xcl.IncludeComputed(), xcl.ShowReferences(), xcl.WithFormatter(formatter))`.
- `example/plugin/plan.go:22-30`: likewise with `xcl.PrintDiff(out, changes, xcl.WithFormatter(formatter))`.
- `example/prettylog/prettylog.go:40-133`: the lipgloss decision picks a `highlight.Formatter`. `writeConfiguration` prints into a `strings.Builder` with `xcl.PrintEntity(&b, entity, xcl.IncludeComputed(), xcl.WithFormatter(f))`, then `indent` as now. The `ErrNotEncodable` handling is unchanged. Update the doc comment (:40-48) to describe `PrintEntity` and `highlight.Terminal`.
- `example/prettylog/prettylog_test.go:484-497`: should pass unchanged (`1;35`, `34`, `33`). If a default-theme difference breaks an assertion, fix the theme, not the test.
- `example/plugin/*_test.go`, `example/remoteplugin/*`: compile fixes only.
- Each example `go.mod`: no change (`replace ../..`).

Complexity: Medium
Token estimate: ~30k tokens
Agent strategy: 2–3 parallel agents: one for coreservice and the root/parser/viper tests, one for the examples. Then run the root suite plus each example module's tests.

### Task: Publish reference outputs for every built-in format

Requirement attribution: "Reference outputs for every language". Repo xcl.

File changes:
- `highlight/testdata/reference/` (new), all generated by the test with `-update`:
  - `report.tokens.json`, `entity.tokens.json`, `diff.tokens.json`: a JSON array of `{"kind":<int>,"text":"…"}`, written with `json.MarshalIndent(v, "", "  ")` plus "\n"
  - for each input: `.plain.txt`, `.terminal.txt`, `.html`, `.inline.html`, `.svg`
  - `default.css`
  - `classes.txt`: `xcl` then every known kind's `Class()` except Text's, in `Kinds()` order, one per line, each followed by "\n"
  - `README.md`: a short description of the file set, the JSON shape and how to regenerate, for other languages' maintainers. This is a data README, not project docs, and no test reads it.
- `highlight/testdata/reference/inputs/entity.xcl` (new): a resource with nested blocks, a heredoc, a reference, a function call, comments of all three kinds, a number, a bool, an escape and a `<` in a string (to exercise escaping).
- `highlight/reference_test.go` (new, `package highlight_test`):
  - `var update = flag.Bool("update", false, "rewrite reference outputs")`.
  - Build the inputs:
    - report: `xcl.ErrorTokens(xclerrors.Diagnostics{...})` from `xclerrors.NewDiagnostic` over a fixed source with an error and a warning, entity and field set, and detail longer than 80 characters (unwrapped in the tokens)
    - entity: `highlight.Tokenize(inputs/entity.xcl)`
    - diff: `(&diff.Diff{…literal from diff/example_test.go…}).Tokens()`
  - One test per input and format, plus one each for CSS and classes, each comparing bytes (`require.Equal(t, string(want), string(got))`), or writing when `*update` is set.
  - A test that the stored token JSON equals the freshly built tokens.
  Importing `xcl` and `diff` from an external test package is a normal import, not a cycle.

Complexity: Medium
Token estimate: ~25k tokens
Agent strategy: Single agent, sequential execution.

### Task: Make the example programs print errors as reports

Requirement attribution: "Examples use the new printing" (errors), success metric "No example program prints a configuration error as a single line". Repo xcl.

File changes:
- `example/configonly/main.go:43-46`: replace `fmt.Fprintf(os.Stderr, "error: %s\n", err)` with `xcl.PrintError(os.Stderr, err, xcl.WithFormatter(highlight.For(os.Stderr)))`. This example has no lipgloss decision of its own, and prettylog's decision applies only to events. Update the package doc (:12-16).
- `example/configonly/testdata/broken/main.xcl` (new): the routes config with an unsupported attribute in a deployment.
- `example/configonly/smoke_test.go:60-67` area: add a smoke test running the binary on `testdata/broken`. Assert a non-zero exit and stderr holding `Error: Unsupported argument` and a gutter line `|` with the faulty line. The existing `unknown_service` test keeps asserting a one-line application error. Rename its expectation if the `error:` prefix goes: it becomes the bare message, since PrintError prints non-configuration errors as `err.Error()`.
- `example/plugin/main.go:112-114`: `xcl.PrintError(stderr, err, xcl.WithFormatter(formatterFor(stderr)))`, where `formatterFor` is a new small helper in the example using the existing lipgloss check (shared with `inspect.go`/`plan.go`).
- `example/plugin/smoke_test.go:95-112`: add a broken-configuration case (new `example/plugin/testdata/broken/`, an unsupported block type) asserting the report. It runs `plan` or `apply`; `plan` needs Docker, so prefer a path that fails at parse before contacting Docker. If none exists, assert through `run()` in `main_test.go` with a buffer.
- `example/remoteplugin/main.go:93-95`: same change with lipgloss or `highlight.For`. Use `highlight.For` if no lipgloss is in its go.mod. Add `example/remoteplugin/testdata/broken/` and a test in `example/remoteplugin/main_test.go` calling `run()` with a buffer and asserting the report.
- Expectations on the `error:` prefix in existing tests are updated to the new output.

Complexity: Medium
Token estimate: ~25k tokens
Agent strategy: 2 parallel agents: configonly plus remoteplugin, and plugin. Run each example module's tests.

### Task: Write the release notes and repository docs

Requirement attribution: "Replacements for the previous API are documented". Repo xcl.

File changes:
- `CHANGELOG.md:1` (new top section `## 20261023074926-eaa51527-printing-and-highlighting-core`): prose covering the print functions, options, formats, themes and style strings, Validate, WriteCSS, For, LoadVSCodeTheme, the labelled-text functions, the HTML class contract, the reference outputs, restored context, `%+v` and zap, coreservice `highlight=true` now formatted by Terminal, and zap as a test-only dependency (with the reason). Then `**Breaking:**` with one line per removal:
  - `highlight.Piece` → `highlight.Token`
  - `highlight.Pieces(src)` → `highlight.Tokenize(src)`
  - `highlight.Renderer`/`RendererFunc`/`PieceFunc` → `highlight.Formatter`/`highlight.FormatterFunc`
  - `highlight.Text.Render(p)` → `highlight.Plain.Format(w, theme, tokens)`, or `xcl.PrintEntity`/`PrintDiff`/`PrintError`
  - `highlight.ANSI` and `ANSIRenderer` → `highlight.Terminal`
  - `highlight.NewANSI(WithTheme(r))` → `highlight.LoadVSCodeTheme(r)` with `xcl.WithTheme`
  - `WithThemeFile(path)` → `os.Open` + `LoadVSCodeTheme`
  - `ANSIOption` → `xcl.PrintOption`
  - `Scope*` → `Kind` constants with `Kind.Scope()`
  - `xcl.DiagnosticPieces(d, w)` → `xcl.ErrorTokens(err)` or `xcl.PrintError(w, err, xcl.WithWidth(n))`
  - `diff.(*Diff).Pieces()` → `(*Diff).Tokens()` or `xcl.PrintDiff`
  Plus behaviour changes: `Diagnostic.Source` now holds context lines, and the default terminal theme was adjusted.
- `docs/overview.md:120-131`: describe the context lines, `%+v`, `PrintError`/`ErrorTokens`, and the `internal/report` layout.
- `docs/README.md:53`: package table row for `highlight/` (kinds, tokens, themes, formatters, CSS, VS Code import). Add a row for `internal/report` if internal packages are listed there.
- `README.md`: if it shows error handling, mention `PrintError`. Otherwise no change.

Complexity: Low
Token estimate: ~12k tokens
Agent strategy: Single agent, sequential execution. No tests read these files (`conventions/testing-and-mocking.md`).

### Task: Document the new printing on the website

Requirement attribution: "Documentation describes the new printing". Repo xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).

File changes:
- `xcl-website:src/pages/configuration-text.mdx:326-479`: replace "## Highlighting" with "## Printing and highlighting".
  - `xcl.PrintEntity` with encode and print options.
  - "### Output formats": Plain, Terminal, HTML, HTMLInline and SVG, with `highlight.For`.
  - "### Themes": style-string syntax, `DefaultTheme.With`, category fallback, `Validate`.
  - "### With an editor theme": `LoadVSCodeTheme`.
  - "### On a website": the class contract, a table of every class name (from `classes.txt`), `WriteCSS`, and `template.HTML`.
  - "### Writing a format": `FormatterFunc`, `Theme.Style`, `EntityTokens`/`Tokenize`.
  The Python tabs keep the current Python form (`encode_entity(highlight=True)`) and say richer printing in Python is coming. The Python spec rewrites them. Drop the statements at :423 and :470 only where the Go text changed.
- `xcl-website:src/pages/problems.mdx:103-144`: "## Showing problems to a person" becomes `xcl.PrintError` (one call, context lines, wrapped errors, width), `%+v`, and "### In logs" (zap's `errorVerbose`, slog logs the one line). Keep the Python statement at :132-134 as is (the Python spec owns it). Update the CtaBanner text (:138-144) to formats and themes.
- `xcl-website:src/pages/diff.mdx:27-70,195-310`: `highlight.Text.Render(d.Pieces())` becomes `xcl.PrintDiff`. "## Colour" uses `WithFormatter(highlight.Terminal)` and the Inserted/Deleted/Changed kinds and classes. `d.Tokens()` for extenders.
- `xcl-website:src/pages/events.mdx:400-409`: the prettylog description uses `PrintEntity` and `highlight.Terminal`. Add a short "Errors in logs" note near :27-84 (zap `errorVerbose`; `SlogHandler` unchanged).
- `xcl-website:src/pages/sensitive-values.mdx:198-229`: `Diff.Pieces` becomes `PrintDiff`/`Tokens`.
- `xcl-website:src/pages/examples/viper.mdx:185-233`: `DiagnosticPieces` becomes `xcl.PrintError`.
- `xcl-website:src/pages/examples/plugins.mdx:1264-1322,1420-1484`: plan and inspect code listings match the updated example sources (`PrintDiff`/`PrintEntity` with the lipgloss-chosen formatter). Main's error printing uses `PrintError`.
- `xcl-website:src/styles/global.css`: no change required. Optionally add a `.xcl` block styling the reference classes with site tokens, used by one rendered HTML example on configuration-text.mdx. If added, keep it scoped under `.xcl`.
- No page is renamed or removed, and nav (`src/components/Nav.astro`) is unchanged (`gotchas/no-redirects-on-page-removal.md`, `gotchas/guides-dropdown-label-opens-first-child.md`).

Complexity: Medium
Token estimate: ~40k tokens
Agent strategy: 2–3 parallel agents: configuration-text; problems, events and viper; diff, sensitive-values and plugins. Then `npm run build` and `make check` in the website repo.

## Testing Strategy

Per task, the tests land beside the code they test (testify `require`, one behaviour per test, no tables, positive and negative tests apart):

- **Lay out error reports in one private package**: `internal/report` unit tests for layout wording, wrapping and the walk. The existing `diagnostics_test.go` must stay green over the rebased `DiagnosticPieces`.
- **Capture two lines of context around each fault**: `errors/diagnostic_test.go` edge cases (middle, first line, last line, trailing newline, multi-line fault). Parser, viper and diagnostics expectations are updated.
- **Print the full report through detailed error formatting**: `errors` tests for `%v`/`%s`/`%q`/`%+v`, and the zap `errorVerbose` test.
- **Capture today's plain output of resources and change summaries**: one test asserting the captured files equal current output (the regression anchor).
- **Rebuild highlighting around kinds, themes and formatters**: the `highlight` unit tests listed in the task notes. The 58 tokenizer parity assertions keep their scope strings.
- **Label change summaries with kinds**: diff token tests. Render tests and the example Output are unchanged in text.
- **Add one-call printing of errors, resources and change summaries**: root integration tests for every acceptance criterion on errors, resources, diffs, formats, custom formats, labelled text, and zap with a real `Validate`.
- **Move every remaining caller to the new printing**: the existing coreservice, parser, viper and example tests pass after porting.
- **Publish reference outputs for every built-in format**: the byte-for-byte reference contract test, with `-update`.
- **Make the example programs print errors as reports**: smoke and run tests per example against a broken configuration.
- **Write the release notes and repository docs**, **Document the new printing on the website**: no automated tests by convention. Manual review is captured in the implementation test plan.

Plan-level strategy:

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

## Project References

- Spec: `20261023074926-eaa51527-printing-and-highlighting-core`, epic `20261023074219-4357d314-printing-and-pretty-output`.
- Design: `printing-and-highlighting.md` from the `design` design source.
- Sibling specs: `20261023075136-42c51a7d-printing-tokens-over-the-engine` (engine/proto), `20261023075302-bad71237-printing-and-highlighting-python` (Python, consumes the reference outputs).
- Knowledge: xcl `conventions/shared-errors-package.md`, `conventions/testing-and-mocking.md`, `conventions/dependencies.md`, `conventions/code-style.md`, `conventions/project-structure.md`, `conventions/shared-test-helpers.md`, `gotchas/example-modules-cannot-import-internal.md`, `gotchas/hcl-halts-on-malformed-block-header.md`, `learnings/releases-tagged-by-hand.md`; xcl-website `gotchas/no-redirects-on-page-removal.md`, `gotchas/guides-dropdown-label-opens-first-child.md`.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`; xcl-vscode `/home/nicj/code/github.com/jumppad-labs/xcl-vscode` (reference only).
- Requirement → repo and files:
  - One-call error printing, surrounding lines, wrapped errors, one-line messages, detail width, labelled text, custom formats, structured logs → xcl: `internal/report/`, `errors/`, `diagnostics.go`, `print.go`, `encode.go`.
  - Formats, class names, stylesheets, themes, validation, VS Code import, colour choice → xcl: `highlight/`.
  - Change summaries → xcl: `diff/render.go`, `print.go`.
  - Reference outputs → xcl: `highlight/testdata/reference/`, `highlight/reference_test.go`.
  - Release notes → xcl: `CHANGELOG.md`, `docs/`.
  - Examples → xcl: `example/`.
  - Documentation → xcl-website: `src/pages/`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The two High tasks ("Rebuild highlighting", "Add one-call printing") should split along the agent lines in their notes and integrate sequentially. Run package-scoped tests while the module is mid-migration in Milestone 2.

## Migration Notes

- This is a breaking pre-1.0 change. Every removed function has a one-line replacement in the changelog's Breaking list (see "Write the release notes and repository docs"). There are no shims.
- `Diagnostic.Source` now includes up to two context lines either side, so `SourceLines.Start` can be smaller than the fault's line. Code that assumed `Lines[0]` is the fault line must use `Subject.Start.Line - Start`.
- The default terminal theme is expressed as kinds. Block types, labels, attributes, strings, numbers, references, comments, diffs and problems keep their 16-colour look. Nested blocks, members, heredoc markers and report summary and gutter gain styles through category fallback.
- The core service's `highlight=true` is still honoured, through the Terminal formatter, until the engine spec reserves it.
- Milestone 2 passes through a state where the module does not compile, between "Rebuild highlighting" and "Move every remaining caller". Land those tasks together on one branch.

## Performance Considerations

- Theme resolution is two map lookups per token. There is no per-scope rule matching at print time; TextMate matching runs only in `LoadVSCodeTheme`.
- Formatters stream to the writer in a single pass over the tokens. SVG needs one extra pass to measure the longest line and count lines.
- Context capture adds at most four lines per diagnostic from bytes already in memory. There is no file I/O.
