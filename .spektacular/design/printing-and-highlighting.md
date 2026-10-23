---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
specs:
    - 20261023074926-eaa51527-printing-and-highlighting-core
    - 20261023075136-42c51a7d-printing-tokens-over-the-engine
    - 20261023075302-bad71237-printing-and-highlighting-python
    - 20261023075328-741138ee-python-prettylog
---

# Printing and highlighting: tokens from the core, formatting in the language

## Problem

Getting readable output out of xcl is harder than it should be, and only Go can do it at all.

- Errors print as one line. Since v0.8.0 removed `ParserError`, printing an error no longer shows the source. The report exists (`xcl.DiagnosticPieces`) but is reached through `errors.As` → `Diagnostics()` → `DiagnosticPieces` → `Render`, and shows only the faulty line. Python cannot produce it at all: the engine sends the source, `xcl.Problem` drops it.
- The highlight API is shaped around VS Code, not Go. Styling means TextMate scope strings (`entity.name.tag.xcl`) and raw escape codes; a custom theme is a VS Code JSON blob that replaces the default instead of extending it; `Render` returns `[]byte` rather than writing to an `io.Writer`, so every call is `out.Write(r.Render(highlight.Pieces(text)))`; five types (`Renderer`, `RendererFunc`, `PieceFunc`, `ANSIRenderer`, `ANSIOption`) cover one idea; and a per-piece function cannot produce SVG or anything else that needs the whole document.
- Python gets finished ANSI bytes or nothing. `encode_entity(highlight=True)` returns terminal codes; there is no diff or report rendering and no way to style anything.

## The shape

Three things, kept apart, after chroma (`alecthomas/chroma`):

- **Tokens**: what the text is, a `Kind` and the text.
- **Theme**: how each kind looks, plain data, styles written as strings like `"bold #ff8800"`.
- **Formatter**: turns tokens into output on a writer. It sees the whole stream, so terminal, HTML, SVG or anything else fits one interface.

On top: print functions that do the common case in one call, and hooks into each language's own error printing and loggers so the common case needs no xcl code at all.

## What lives where

The rule from the libxcl design holds: whatever must behave identically and depends on the parser stays in the core; everything a user styles, writes or extends is native.

| Concern | Core (Go engine, over CoreService) | Native (Go, Python, TypeScript) |
|---|---|---|
| Lexing configuration text into tokens | ✔ the HCL lexer, never reimplemented | |
| Encoding an entity as text | ✔ returns text and tokens | |
| Laying out an error report | ✔ once, attached to every `Error` | wrapping the detail to a width |
| Laying out a diff | ✔ returns tokens | |
| Source context, 2 lines either side | ✔ captured at parse time | |
| Converting a VS Code theme to a `Theme` | ✔ `ConvertTheme`, needs the TextMate selector rules | `load_vscode_theme` calls it |
| `Kind` enum and its parent rule | numbering defined in the proto | enum mirrored by hand, tested against the proto |
| `Token`, `Theme`, style parsing, default theme | | ✔ golden-tested for parity |
| `Formatter` interface; Plain, Terminal, HTML, HTMLInline, SVG; CSS from a theme | | ✔ golden-tested for parity |
| Print functions, colour choice, logger hooks | | ✔ idiomatic per language |

In Go "core" and "native" are the same process: Go calls the core functions directly, no protocol.

## Token kinds

Kinds are numbered in blocks of 100. A kind's **parent** is `kind / 100 * 100`, so a theme that styles `Comment` covers every comment. Resolution is: the kind, then its parent, then `Text`. Because the parent is arithmetic, every language computes it identically, and an unknown kind from a newer engine still falls back to its parent.

| Parent | Kinds |
|---|---|
| `Text` 0 | unlabelled text |
| `Block` 100 | `BlockType` 101, `NestedBlock` 102 |
| `Label` 200 | `TypeLabel` 201, `NameLabel` 202 |
| `Attribute` 300 | |
| `String` 400 | `StringEscape` 401, `Heredoc` 402, `HeredocMarker` 403, `HeredocOperator` 404 |
| `Number` 500 | |
| `Constant` 600 | `true`, `false`, `null` |
| `Function` 700 | `BuiltinFunction` 701 |
| `Reference` 800 | `Member` 801 |
| `Operator` 900 | |
| `Punctuation` 1000 | `InterpolationBegin` 1001, `InterpolationEnd` 1002, `TemplateDirective` 1003, `Accessor` 1004 |
| `Comment` 1100 | `LineComment` 1101, `HashComment` 1102, `BlockComment` 1103 |
| `Diff` 1200 | `Inserted` 1201, `Deleted` 1202, `Changed` 1203 |
| `Problem` 1300 | `Error` 1301, `Warning` 1302 |
| `Report` 1400 | `Summary` 1401, `Location` 1402, `Gutter` 1403, `Detail` 1404 |

Each kind has a `Scope()` returning its TextMate scope (`Attribute` → `variable.other.property.xcl`). It exists only for loading VS Code themes and for the xcl-vscode parity tests; nothing else in the API uses scopes.

The `Report` kinds are new. Today the summary, location, line-number gutter and detail are unlabelled, so HTML or SVG cannot style them. The faulty span is `Error` or `Warning`, overlaid on the source tokens.

## Styles and themes

A style is a string of words, identical in every language and in JSON:

```
bold italic underline dim    #rrggbb  bg:#rrggbb    red green yellow blue magenta cyan white black  bright-red ...
```

Named colours map to the 16 basic terminal colours, hex to 24-bit. An unknown word is reported by `Validate` and skipped by formatters, which never fail on a style.

A theme maps kinds to styles. Each language extends the default with its own map idiom:

```go
theme := highlight.DefaultTheme.With(highlight.Theme{
	highlight.Attribute: "#ff8800",
	highlight.String:    "italic #00ff88",
})
```

```python
theme = highlight.DEFAULT_THEME | {Kind.ATTRIBUTE: "#ff8800", Kind.STRING: "italic #00ff88"}
```

```ts
const theme = { ...defaultTheme, [Kind.Attribute]: "#ff8800", [Kind.String]: "italic #00ff88" };
```

## Formatters

```go
type Formatter interface {
	Format(w io.Writer, theme Theme, tokens []Token) error
}

type FormatterFunc func(w io.Writer, theme Theme, tokens []Token) error
```

```python
class Formatter(Protocol):
    def format(self, out: TextIO, theme: Theme, tokens: Sequence[Token]) -> None: ...
```

```ts
export interface Writer { write(chunk: string): unknown }   // process.stdout, a file stream, a string builder
export type Formatter = (out: Writer, theme: Theme, tokens: readonly Token[]) => void;
```

Built-in, with identical output in every language, held to shared golden files:

- **Plain**: the text, the theme ignored.
- **Terminal**: SGR codes, reset before every newline so indenting or prefixing a line never bleeds colour.
- **HTML**: `<pre class="xcl">` with a `<span class="…">` per token, text escaped. The theme is ignored: a website styles the classes with its own stylesheet. See [HTML on a website](#html-on-a-website).
- **HTMLInline**: the same markup with `style="…"` from the theme instead of classes, for places a stylesheet cannot reach, such as email or a pasted snippet.
- **SVG**: monospace, one `<text xml:space="preserve">` per line with a `<tspan fill="…">` per token, `viewBox` sized from the longest line and the line count, background from `Text`'s `bg:`. An SVG is usually used as a standalone image, where page CSS does not apply, so it carries its colours itself.

### HTML on a website

The HTML formatter writes classes, so a website owns how xcl output looks. Class names are a public contract, held to the golden files like the formatters' output:

- The wrapper is `<pre class="xcl">`.
- Each token's span carries its kind's class and, when the kind has a parent, the parent's class too, so a stylesheet can target a whole category or one kind: `<span class="xcl-comment xcl-line-comment">`, `<span class="xcl-attribute">`. Class names are the kind names in kebab case with an `xcl-` prefix.
- `Text` tokens are written without a span.

A bad configuration rendered on a page:

```html
<pre class="xcl"><span class="xcl-problem xcl-error">Error</span><span class="xcl-report xcl-summary">: Unsupported block type</span>

<span class="xcl-report xcl-location">  on config.xcl line 7, in database.postgres.main, field timeout[0]:</span>
<span class="xcl-report xcl-gutter">     6 | </span>
<span class="xcl-report xcl-gutter">     7 | </span>  <span class="xcl-problem xcl-error">timeout</span> <span class="xcl-label xcl-name-label">"http"</span> {
<span class="xcl-report xcl-gutter">     8 | </span>    <span class="xcl-attribute">vaulue</span> <span class="xcl-operator">=</span> <span class="xcl-number">5000</span>
…</pre>
```

styled by the site's own CSS:

```css
.xcl              { background: var(--surface); color: var(--text); }
.xcl .xcl-error   { color: var(--danger); text-decoration: wavy underline; }
.xcl .xcl-gutter  { color: var(--muted); user-select: none; }
.xcl .xcl-string  { color: var(--accent); }
```

A site that wants xcl's own colours, or a custom theme, as a starting point generates a stylesheet from a theme rather than writing one by hand:

```go
highlight.WriteCSS(w, highlight.DefaultTheme) // or any Theme
```

```python
highlight.css(highlight.DEFAULT_THEME) -> str
```

```ts
css(defaultTheme): string
```

The stylesheet scopes every rule under `.xcl`, writes parent rules before child rules so a kind's own style wins, maps named colours to a fixed hex palette (the terminal's `bright-red` has no CSS name) and `dim` to `opacity`. The same stylesheet ships as `xcl.css` in the TypeScript package for sites that only want to link it.

In Go the HTML is already escaped, so it goes into a page as `template.HTML`:

```go
var report strings.Builder
xcl.PrintError(&report, err, xcl.WithFormatter(highlight.HTML))
page.Execute(w, map[string]any{"Report": template.HTML(report.String())})
```

### Choosing colour

Colour is the caller's decision; xcl never checks for a terminal on its own. One opt-in helper makes the usual choice, Terminal for a terminal with `NO_COLOR` unset and Plain otherwise: `highlight.For(os.Stderr)` in Go (`os.File` and `ModeCharDevice`, standard library only), `highlight.for_stream(sys.stderr)` in Python, `forStream(process.stderr)` in TypeScript.

## Printing

### Go

```go
// package xcl
func PrintError(w io.Writer, err error, options ...PrintOption) error
func PrintEntity(w io.Writer, entity any, options ...PrintOption) error
func PrintDiff(w io.Writer, d *diff.Diff, options ...PrintOption) error

func WithFormatter(f highlight.Formatter) PrintOption // default highlight.Plain
func WithTheme(t highlight.Theme) PrintOption         // default highlight.DefaultTheme
func WithWidth(columns int) PrintOption               // wraps report details; default 80, 0 never wraps

// for extenders
func ErrorTokens(err error) []highlight.Token
func EntityTokens(entity any, options ...EncodeOption) ([]highlight.Token, error)
func (d *Diff) Tokens() []highlight.Token             // package diff
```

`PrintError` takes any error: it finds every diagnostic in the chain with `errors.As`, so wrapping with `%w` does not hide them, and prints any other error as `err.Error()`, as `go/scanner.PrintError` does.

`EncodeOption` gains a method that makes it a `PrintOption`, so encode and print options mix in one call:

```go
xcl.PrintEntity(os.Stdout, db, xcl.IncludeComputed(), xcl.WithFormatter(highlight.For(os.Stdout)))
```

`*errors.ConfigError` and `*errors.Diagnostic` implement `fmt.Formatter`: `%v` and `%s` print the one line, `%+v` the plain report. This works only on the unwrapped error and is documented so; `PrintError` is the dependable route.

### Python

```python
xcl.print_error(err, file=sys.stderr, *, formatter=highlight.PLAIN, theme=highlight.DEFAULT_THEME, width=80)
xcl.format_error(err, *, formatter=..., theme=..., width=80) -> str
cfg.print_entity(entity, file=sys.stdout, *, include_computed=False, ..., formatter=..., theme=...)
cfg.entity_tokens(entity, *, include_computed=False, ...) -> list[Token]
xcl.print_diff(diff, file=sys.stdout, *, formatter=..., theme=...)
err.tokens   # list[Token], the core's report; empty for an error without diagnostics
diff.tokens
```

The `print_`/`format_` pairs follow `traceback.print_exception` and `traceback.format_exception`. `str(err)` stays one line.

### TypeScript

```ts
printError(err: unknown, options?: PrintOptions): void        // out defaults to process.stderr
formatError(err: unknown, options?: FormatOptions): string
await cfg.printEntity(entity, { out, formatter, theme, includeComputed: true });
await cfg.entityTokens(entity, { includeComputed: true }); // Token[]
printDiff(diff, options?);

interface PrintOptions { out?: Writer; formatter?: Formatter; theme?: Theme; width?: number }
err.tokens; diff.tokens;
```

## Errors and loggers: the common case needs nothing

### Go

`log/slog` is the hub: zap (`zapslog`), zerolog (`slog-zerolog`), logrus and charmbracelet/log all accept or provide an `slog.Handler`, so the existing `events.SlogHandler(logger)` remains the one bridge, unchanged.

zap adds an `errorVerbose` field from `%+v` for any error implementing `fmt.Formatter`, so `zap.Error(err)` carries the plain report with no adapter.

Pretty terminal output is `xcl.PrintError(os.Stderr, err, xcl.WithFormatter(highlight.For(os.Stderr)))`, and an event handler that wants reports under its log lines does the same with `e.Error`.

### Python

Every `xcl.Error` that carries a report gets it added as an exception note (`add_note`, Python 3.11+). An uncaught error's traceback, `logging.exception`, `exc_info=True`, `rich` tracebacks and loguru all print notes after the message, so a bad configuration shows its source with no code at all. That is what `ParserError` used to give Go.

`xcl.logging_handler(logger=logging.getLogger("xcl"))` returns an `on_event` callable that writes each event as a `LogRecord`, at the same levels and messages as Go's `SlogHandler`, with the event's fields in `extra`. structlog and loguru receive stdlib records through their documented stdlib integration, so they need no adapter of their own.

`xcl.highlight.LogFormatter` is a `logging.Formatter` whose `formatException` renders an `xcl.Error` with a chosen formatter, making a coloured console handler one line:

```python
handler.setFormatter(xcl.highlight.LogFormatter(formatter=highlight.for_stream(sys.stderr)))
```

### TypeScript

`XclError` implements `util.inspect.custom`, so `console.error(err)` and an uncaught error print the report. Colour follows the `colors` option Node's console already sets for its stream, so Node decides colour as it does for everything else. To be confirmed while implementing: that the supported Node versions print uncaught errors through `inspect`.

`XclError` has enumerable `problems` (file, line, column, summary, entity, field) and `report` (plain text). pino's standard `err` serializer and winston's `format.errors()` include enumerable properties, so JSON logs carry structured problems with no configuration.

`events.toLogger(logger)` adapts any logger with `debug`/`info`/`warn`/`error` methods, pino-style `(fields, message)` by default and winston-style `(message, fields)` with `{ order: "message-first" }`: `onEvent: events.toLogger(pino())`.

## Protocol

```protobuf
enum TokenKind { TOKEN_KIND_TEXT = 0; TOKEN_KIND_BLOCK = 100; TOKEN_KIND_BLOCK_TYPE = 101; /* … the table above … */ }
message Token { TokenKind kind = 1; string text = 2; }

message Error { /* code, message, details, causes */ repeated Token report = 5; }

message EncodeEntityRequest  { bytes entity = 1; reserved 2; EncodeOptions options = 3; bool tokens = 4; }
message EncodeEntityResponse { bytes text = 1; repeated Token tokens = 2; }
// EncodeSavedEntityRequest and EncodeSavedEntityResponse change the same way

// the response carrying diff_json gains: repeated Token diff_tokens

message ConvertThemeRequest  { bytes vscode_theme_json = 1; }       // handle 0, no configuration needed
message ConvertThemeResponse { map<int32, string> styles = 1; }     // TokenKind → style string
```

`report` is laid out once, when the error is converted: the diagnostic's summary, location, source context and detail, unwrapped. Because every error carries it, a binding prints a report with no engine call, including from an event handler while an operation runs.

The old `highlight` field is reserved, never reused, as `buf breaking` requires.

## Source context

`errors.SourceLines` captures up to 2 lines before and after the lines the fault covers, clipped at the file's ends. The subject range still marks the fault exactly, so the report numbers every line and overlays `Error` on the faulty span alone. This restores the context `ParserError` showed, from source already in memory, never re-reading the file.

## Removed

xcl is pre-1.0, so the old API is removed rather than deprecated: `highlight.Piece`, `Pieces`, `Renderer`, `RendererFunc`, `PieceFunc`, `Text`, `ANSI`, `ANSIRenderer`, `NewANSI`, `WithTheme(io.Reader)`, `WithThemeFile`, the exported `Scope*` constants (now `Kind.Scope()`), `xcl.DiagnosticPieces`, `diff.(*Diff).Pieces`, and Python's `highlight=` parameter on `encode_entity` and `encode_saved_entity`. Each has a one-line replacement in the migration notes. `highlight.Tokenize(src []byte) []Token` replaces `Pieces` for extenders who tokenize configuration text themselves.

## Decisions

- **Built-in formatters are native, not rendered by the core.** Custom and built-in formatters share one interface, themes never cross the wire, and the formatters are small. Parity comes from shared golden files, not one implementation.
- **The report travels on every `Error`, not through a separate call.** No round trip, no configuration handle needed, and it works mid-operation.
- **Styles are strings.** One syntax across Go, Python, TypeScript and JSON theme files, proven by chroma; a typed struct differs per language.
- **Two-level kinds, the parent by arithmetic.** Short themes, identical fallback everywhere, no lookup table.
- **HTML writes classes; inline styles are the exception.** A website must be able to restyle xcl output to match itself, which inline styles prevent. Inline styles remain as `HTMLInline` for email and other places without a stylesheet. Class names follow the kinds, parent and child both, so CSS can be as broad or as narrow as the site wants.
- **Go errors do not implement `slog.LogValuer`, for now.** Doing so would let slog's JSON handlers log each problem as structured fields, as pino and winston will in TypeScript, but it makes text handlers noisy and nothing asks for it yet. slog logs the one-line `Error()`; it can be added later without breaking anything.
- **chroma is not used directly.** It would make chroma a dependency of xcl's core and give one language two lexers; its shape is borrowed instead.
