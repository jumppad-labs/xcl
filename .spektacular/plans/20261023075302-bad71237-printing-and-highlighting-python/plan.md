---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Plan: 20261023075302-bad71237-printing-and-highlighting-python

<!-- Metadata -->
<!-- Created: 2026-10-23T09:05:25Z -->
<!-- Commit: xcl ae3b7b6, xcl-python 1aee25c, xcl-website f82b715 -->
<!-- Branch: f-python-prettylogger (xcl), main (xcl-python), main (xcl-website) -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git, jumppad-labs/xcl-python, jumppad-labs/xcl-website -->

## Overview

Python programs that use xcl gain the same readable output as Go. Any xcl error prints in one call as Go's report: severity, location, numbered source lines with the fault marked, and wrapped detail. The report also appears on its own in an uncaught error's traceback and in logged exceptions, while `str(err)` stays one line. Resources, saved records and change summaries print in one call. Output can be plain text, terminal colour, HTML a website restyles, inline-styled HTML or SVG, styled by short themes that extend the default or come from a VS Code theme, or by a format the user writes. Events reach Python's standard logging in one line.

This replaces one-line errors, a terminal-only `highlight=` keyword (removed) and hand-written logging handlers. Python application authors, website maintainers and the downstream pretty logger benefit. Every format is held byte for byte to the reference outputs xcl publishes, and the reports, resources, diffs and event logs are compared with Go's on shared fixtures.

## Conventions

- **Testing & Mocking: never use table-driven tests** — every format × reference input, every refused option and every logging level gets its own test function, with no `pytest.mark.parametrize` table; the xcl-python suite follows xcl's rules, as the earlier Python plans did.
- **Testing & Mocking: never mix positive and negative tests; favour verbosity over abstraction** — valid and invalid themes, the colour and plain branches of `for_stream`, and accepted and removed keywords are separate functions, each spelling out its input and assertion.
- **Testing & Mocking: locate tests with the source they test; never write tests that inspect repository files** — unit tests sit in `tests/unit/` and engine-backed tests in `tests/integration/`, the Go parity additions in `cmd/parity/main_test.go`; reading the published reference outputs is golden test input, while README, PARITY.md, the website and release notes are checked by review only.
- **Testing & Mocking: use testify `require`** — applies to the Go parity program's new tests in xcl; the Python tests keep the suite's plain `assert`.
- **Dependencies: prefer the standard library; document every third-party dependency** — the formatters, CSS, SVG, wrapping, notes and logging use only the standard library, and no runtime or test dependency is added beyond protobuf.
- **Development standards: structured logs** — the logging bridge writes each event's fields as structured `LogRecord` attributes under Go's attribute names, so JSON and key-value handlers receive them as fields.
- **Go code style (gofmt/vet, descriptive names, explicit error handling)** — applies to the few new stages of the Go parity program.

Deliberately dropped: shared errors package, shared test helpers in `internal/testutil` and project structure (Go-library layout rules; the only Go change is inside the parity program, which adds no error type and no shared helper), config parser options builder, hclsyntax references, graph-parent ordering, real-apply test state (no state fixtures are hand-written here), provider Changed/Update testing, never-modify-dependencies (no dependency is edited; go-wordwrap is ported, not patched), database and patterns & architecture (no services).

## Architecture & Design Decisions

The work is built to the design `design/printing-and-highlighting.md` (source `design`) as written: the engine lexes and lays out, and Python formats the labelled text (tokens) it receives. Almost all of it lands in xcl-python (`/home/nicj/code/github.com/jumppad-labs/xcl-python`). A new public package, `xcl.highlight` (`xcl/highlight/__init__.py` re-exporting small private modules), holds `Kind` (an `IntEnum` mirroring the protocol's `TokenKind`, category by `kind // 100 * 100`), `Token`, style parsing, theme resolution, `DEFAULT_THEME` (an immutable mapping that extends with `|`), `validate`, the five built-in formatters `PLAIN`, `TERMINAL`, `HTML`, `HTML_INLINE` and `SVG` behind the design's `Formatter` protocol, `css`, `for_stream`, `load_vscode_theme` and `LogFormatter`. The top-level package gains `print_error`, `format_error`, `print_diff` and `logging_handler`, the exceptions `InvalidStyleError` and `InvalidThemeError`, `err.tokens` and `diff.tokens`. Both configuration flavours gain `print_entity` and `entity_tokens`, and also `print_saved_entity` and `saved_entity_tokens`: the engine offers labelled text on both encode calls, the removed `highlight=` on `encode_saved_entity` needs a replacement, and the downstream pretty-logger spec renders created resources from saved records. `highlight=` is deleted from `encode_entity` and `encode_saved_entity` in both flavours, and the protocol stubs in `xcl/_core` are regenerated from an xcl checkout carrying the engine plan, in the same change, because the regenerated stubs no longer accept the field. No runtime dependency is added beyond protobuf (`conventions/dependencies.md`).

Every engine error already becomes a Python exception in one function, `_errors.from_proto`, used by calls, finished operations and event errors. That function now converts the error's `report` into `err.tokens` and, when there is a report, adds the plain report as an exception note (`add_note`, Python 3.11, the package minimum). So an uncaught error's traceback, `logging.exception`, `exc_info=True`, rich and loguru show the source with no code at all, and `str(err)` keeps the engine's message. The note is the report formatted with `PLAIN` at width 80, trailing blank lines removed. `print_error`/`format_error` follow `traceback.print_exception`/`format_exception`: an `xcl.Error` with tokens is formatted with the chosen formatter, theme and width; any other exception prints as its string and a newline, as Go prints `err.Error()`. Detail wrapping is applied at print time to `DETAIL` tokens only, by an exact port of `mitchellh/go-wordwrap` v1.0.1, the function Go's printing uses, so plain Python output equals Go's byte for byte. Colour is never chosen by the package: every default is `PLAIN`, and `for_stream` (TTY and `NO_COLOR` unset gives `TERMINAL`) is opt-in. `load_vscode_theme` sends the file to the engine's configuration-free `ConvertTheme` and returns only the kinds the theme styles, as Go's `LoadVSCodeTheme` does; an unusable theme raises `xcl.InvalidThemeError`, registered for the engine's new `invalid_theme` code. Kinds from a newer engine stay plain integers and fall back to their category everywhere.

**Recorded decision: `str(err)` matches Go (confirmed by the user during epic planning).** The spec criterion "Ordinary messages stay one line" is read as "the report never goes into `str(err)`". `xcl.Error.__str__` stays the engine's message, which is Go's `Error()` text: one line for a single problem, and one line per problem for a configuration error holding several. The report reaches people only through the exception note, `err.tokens` and the print functions. The user confirmed this reading during epic planning, so it is a recorded decision for this plan, not a drafting assumption, and implementation must not collapse a multi-problem message into one line.

Logging mirrors Go. `xcl.logging_handler(logger)` returns an `on_event` callable that writes each event as a `LogRecord` with Go's `SlogHandler` rules: the logger's level checked first, the event's own level and message for log events, info for lifecycle, discovery and load events as `"<operation> <phase>"`, error for the error phase and warn for blocked; the record carries the event's time and, as `extra`, Go's attribute names (`source`, `operation`, `phase`, `resource`, `type`, `file`, `duration`, `error`) followed by its details, plus the `Event` itself as `xcl_event`. A detail whose name would overwrite a `LogRecord` attribute is reachable only through `xcl_event`, because logging refuses to overwrite one. structlog and loguru take these records through their documented stdlib integration. `xcl.highlight.LogFormatter` is a `logging.Formatter` whose `formatException` formats the traceback without the plain report note and appends the report in the chosen formatter, so a coloured console handler is one line.

Parity with Go is proven against Go's real output, not against literals. Built-in formats are held byte for byte to the reference files the core plan publishes in xcl's `highlight/testdata/reference/`, read through the test suite's existing `xcl_src` fixture (strict in CI). The kind enum is held to the protocol's descriptor in both directions. The report, resource and change-summary text and the logging bridge are compared with Go through the existing Go/Python parity harness. Its Go side (`xcl:bindings/libxcl/cmd/parity`, the only xcl change) and its Python side (`xcl-python:tests/parity/run_python.py`) both gain printed-error, printed-entity, printed-diff and event-log stages, with a new fixture whose configuration has an unsupported block on line 7. Tests follow the project's testing conventions (`conventions/testing-and-mocking.md`): one behaviour per test, no parametrised tables, positive and negative cases apart, and no test reading README, PARITY.md or website pages. Documentation lands in xcl-python's README (with a migration note for the removed `highlight=`) and PARITY.md, and in the Python tabs and "not yet available from Python" statements of xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`), edited in place with no page removed or renamed (`gotchas/no-redirects-on-page-removal.md`). The change ships as a `minor` xcl-python release, with a line added by hand to its generated release notes naming the replacement for `highlight=`. This beats a flat single module (a 1,000-line file mixing formatting, logging and engine calls), lazily computed notes (a report missing from tracebacks of errors never touched by xcl code) and literal "same as Go" expectations (agreement only at authoring time); see `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Protocol messages (xcl-python, regenerated)**: the generated CoreService message classes the package builds and reads. They are regenerated from the engine protocol that carries the token kinds, labelled text, the report on errors, labelled diff text and theme conversion, and they no longer carry the terminal-colour request field. Every other component reads labelled text from them.
- **Token kinds and labelled text (xcl-python, new, in `xcl.highlight`)**: owns `Kind`, the numbered kinds mirrored by hand from the protocol, each kind's category by arithmetic and its HTML class name, and `Token`, one labelled piece of text. It turns protocol tokens into Python tokens, keeping a kind a newer engine sends but this package does not know as its number. Every formatter, the error and diff types and the print functions use it.
- **Styles and themes (xcl-python, new, in `xcl.highlight`)**: owns parsing style strings ("bold #ff8800", named colours, `bg:`), the fixed colour palette, resolving a kind's style through its category and then plain text, the immutable default theme that a user extends with the mapping union operator, and theme validation, which names every unknown word while printing simply skips it. Formatters and the stylesheet generator consume resolved styles.
- **Formatters (xcl-python, new, in `xcl.highlight`)**: owns the `Formatter` protocol and the built-in plain, terminal, class-marked HTML, inline-styled HTML and SVG formats. Each is given the whole labelled text, a theme and a stream, and writes byte for byte what Go's matching formatter writes. A user's own formatter plugs in at the same seam and is accepted everywhere a built-in one is.
- **Stylesheet generator and colour chooser (xcl-python, new, in `xcl.highlight`)**: the stylesheet generator turns any theme into a `.xcl`-scoped stylesheet identical to Go's. The colour chooser picks the terminal format for a terminal stream with `NO_COLOR` unset and plain text otherwise, and is used only when a caller asks for it.
- **VS Code theme import (xcl-python, new, in `xcl.highlight`)**: reads a VS Code colour theme from a path or bytes and asks the engine to convert it, with no configuration needed, so the TextMate matching stays in the core. It returns styles keyed by kind and raises the theme error the engine reports for an unusable theme.
- **Errors (xcl-python, changed)**: the engine-error conversion that every call, finished operation and event already goes through now also keeps the error's report as its labelled text and adds the plain report, wrapped at 80 columns, as an exception note, so tracebacks and logged exceptions show it with no code. An error's string stays the engine's message. Two exceptions join the hierarchy: one for an unknown style word in a theme, raised by the package itself, and one registered for the engine's new theme error code.
- **Diffs (xcl-python, changed)**: a diff returned by an operation now also holds its change summary as labelled text, taken from the finished operation alongside the structured diff it already reads.
- **Detail wrapping (xcl-python, new, private)**: wraps a report's detail text to a width exactly as Go's wrapping library does, or not at all. The print functions and the exception note use it, and it touches only detail tokens.
- **Print functions (xcl-python, new, top level)**: own one-call printing of an error, to a stream or as a string, and of a change summary, with a formatter, a theme and, for errors, a detail width, all defaulting to plain text, the default theme and 80 columns. They compose the error and diff labelled text, detail wrapping and a formatter, and print any other exception as its message.
- **Configurations, synchronous and asyncio (xcl-python, changed)**: the two configuration flavours lose the terminal-colour keyword on their encode calls and gain printing and labelled text for an entity and for a saved record. These ask the engine for labelled text with the existing encode choices and hand it to a formatter. The asyncio flavour awaits the engine call and shares the request building with the synchronous one, as it does today.
- **Logging bridge (xcl-python, new, top level)**: turns each event into a standard logging record at Go's level and with Go's message, with the event's fields as record attributes and the event itself attached, honouring the logger's level first. It is an ordinary `on_event` handler, so it works with both configuration flavours and reaches structlog and loguru through their stdlib integration.
- **Log formatter (xcl-python, new, in `xcl.highlight`)**: a standard logging formatter that, for a logged xcl error, writes the traceback without the plain report note and then the report in the chosen format. It uses the print functions and the formatters.
- **Parity harness (xcl and xcl-python, changed)**: the Go parity program and its Python counterpart each record, for every shared fixture, the printed report of a failed validate and apply, the printed text of every created entity with computed values, the printed change summary, and the level and message of every event written through each side's logging bridge. The comparison test then fails at the first difference. A new fixture holds a configuration with an unsupported block on line 7. This is the only xcl change in the plan.
- **Test suite (xcl-python, changed)**: the colour tests and the tests of the removed keyword go; unit tests cover kinds, styles, themes, formatters, wrapping, notes, printing and the logging pieces against the fake transport; reference tests hold every built-in format, the stylesheet and the class names to xcl's published files; integration tests cover tracebacks, logged errors, entity and saved-record printing, diffs, VS Code themes and the logging bridge against the real engine.
- **Package documentation and release notes (xcl-python, changed)**: the README documents printing, notes on unhandled and logged errors, formats, themes, stylesheets, custom formats and logging, with a migration note for the removed keyword. The parity list gains the printing and logging rows. The GitHub release notes of the shipping version get a line, added by a person, naming the replacement.
- **Website (xcl-website, changed)**: the Python tabs of the configuration-text, problems, diff and events pages and of the plugins and configuration-only examples show Python printing, formats, themes, stylesheet generation, custom formats and logging. Every statement that the report, rendered diffs, editor themes or custom renderers are unavailable from Python goes. Pages are edited in place.
- **Engine and Go library (xcl, unchanged here)**: the report layout, lexing, encoding, diff layout, theme conversion and reference outputs come from the core and engine plans. This plan consumes them and adds nothing to the engine.

## Data Structures & Interfaces

The public shapes follow the design's Python section, extended only where noted. Byte-level formatter rules, the palette and the record attribute list are in the plan's context document.

**`xcl.highlight.Kind` and `Token`**: the label vocabulary, hand-mirrored from the protocol's `TokenKind` (same numbers; member names are the protocol names without `TOKEN_KIND_`). A kind a newer engine sends that this package does not know stays a plain `int` and resolves through its category.

```python
class Kind(enum.IntEnum):
    TEXT = 0; BLOCK = 100; BLOCK_TYPE = 101; NESTED_BLOCK = 102; ...   # the design's table
    REPORT = 1400; SUMMARY = 1401; LOCATION = 1402; GUTTER = 1403; DETAIL = 1404
    @property
    def parent(self) -> int: ...        # self // 100 * 100
    @property
    def css_class(self) -> str: ...     # "xcl-block-type"

def parent_of(kind: int) -> int: ...    # any int, known or not
def css_class_of(kind: int) -> str: ... # a known kind's class, else its category's, else ""

@dataclasses.dataclass(frozen=True)
class Token:
    kind: int          # a Kind member when known
    text: str
```

**Styles and themes**: a theme is plain data, kind to style string; the default is immutable and extends with `|`, as the design shows. Resolution is the kind's style, else its category's, else `TEXT`'s.

```python
Theme = Mapping[int, str]
DEFAULT_THEME: Theme                    # types.MappingProxyType; DEFAULT_THEME | {...} -> dict

@dataclasses.dataclass(frozen=True)
class Style:
    bold: bool = False; italic: bool = False; underline: bool = False; dim: bool = False
    foreground: str = ""                # "", a named colour such as "bright-red", or "#rrggbb"
    background: str = ""                # "" or "#rrggbb"

def parse_style(style: str) -> Style: ...           # never fails; unknown words skipped
def style_for(theme: Theme, kind: int) -> Style: ...
def hex_colour(colour: str) -> str: ...             # named colours through the fixed palette
def validate(theme: Theme) -> None: ...             # raises xcl.InvalidStyleError naming every unknown word
```

**`Formatter` and the built-ins**: the one output seam, given the whole labelled text.

```python
class Formatter(Protocol):
    def format(self, out: TextIO, theme: Theme, tokens: Sequence[Token]) -> None: ...

PLAIN: Formatter; TERMINAL: Formatter; HTML: Formatter; HTML_INLINE: Formatter; SVG: Formatter
def css(theme: Theme) -> str: ...
def for_stream(stream: object) -> Formatter: ...    # TERMINAL for a TTY with NO_COLOR unset, else PLAIN
def load_vscode_theme(source: str | os.PathLike[str] | bytes) -> dict[int, str]: ...  # path, or the file's bytes

class LogFormatter(logging.Formatter):
    def __init__(self, fmt=None, datefmt=None, style="%", validate=True, *, defaults=None,
                 formatter: Formatter = PLAIN, theme: Theme = DEFAULT_THEME, width: int | None = 80) -> None: ...
```

**Print functions (top level)**: `print_`/`format_` pairs after `traceback.print_exception`/`format_exception`. `file=None` means `sys.stderr` for errors and `sys.stdout` for diffs and entities, looked up at call time. A width of `0` or `None` never wraps.

```python
def print_error(err: BaseException, file: TextIO | None = None, *, formatter: Formatter = PLAIN,
                theme: Theme = DEFAULT_THEME, width: int | None = 80) -> None: ...
def format_error(err: BaseException, *, formatter: Formatter = PLAIN,
                 theme: Theme = DEFAULT_THEME, width: int | None = 80) -> str: ...
def print_diff(diff: Diff, file: TextIO | None = None, *, formatter: Formatter = PLAIN,
               theme: Theme = DEFAULT_THEME) -> None: ...       # a diff with no labelled text raises ValueError
def logging_handler(logger: logging.Logger | None = None) -> Callable[[Event], None]: ...  # default logging.getLogger("xcl")
```

**Configurations (both flavours; `xcl.aio.Config` methods are `async`)**: the encode calls lose `highlight`; printing and labelled text are added for an entity (the design) and a saved record (this plan's addition). The encode keywords are the existing four.

```python
def encode_entity(self, entity, *, include_computed=False, include_empty=False,
                  reveal_sensitive=False, show_references=False) -> str            # highlight= removed
def encode_saved_entity(self, data: bytes, *, ...same four...) -> str               # highlight= removed
def entity_tokens(self, entity, *, ...same four...) -> list[Token]
def print_entity(self, entity, file: TextIO | None = None, *, ...same four...,
                 formatter: Formatter = PLAIN, theme: Theme = DEFAULT_THEME) -> None
def saved_entity_tokens(self, data: bytes, *, ...same four...) -> list[Token]
def print_saved_entity(self, data: bytes, file: TextIO | None = None, *, ...same four...,
                       formatter: Formatter = PLAIN, theme: Theme = DEFAULT_THEME) -> None
```

**Errors and diffs (changed)**: `Error` gains labelled text and a note. `Diff` gains labelled text. Two exceptions are added and exported.

```python
class Error(Exception):
    tokens: list[Token]                 # the engine's report; empty for an error without one
    def __init__(self, message, *, code=None, details=None, causes=(), tokens=()) -> None: ...
                                        # tokens present -> add_note(plain report at width 80, trailing newlines removed)

class InvalidStyleError(Error):         # raised by the package; code "invalid_style"
    words: tuple[tuple[int, str, str], ...]   # (kind, style, unknown word), in kind order

class InvalidThemeError(Error):         # engine code "invalid_theme"; details "source", "reason"
    code = "invalid_theme"

@dataclasses.dataclass(frozen=True)
class Diff:
    ...                                 # summary, resources, raw: unchanged
    tokens: tuple[Token, ...] = ()      # compare=False, repr=False; from OperationFinished.diff_tokens
```

**Logging record (contract with formatters and structlog/loguru)**: level and message as Go's `SlogHandler`. `extra` holds `source`, `operation`, `phase`, `resource`, `type`, `file` (each only when set), `duration` (a `timedelta`, when non-zero), `error` (the `xcl.Error`, when set), then each `meta` detail by name except a log event's `level` and `message` and any name a `LogRecord` already uses, plus `xcl_event` (the `xcl.Event`). `created` is the event's time.

**Serialization boundaries**:
- Engine → Python: `Error.report`, `EncodeEntityResponse.tokens`/`EncodeSavedEntityResponse.tokens` (asked for with `tokens=True`), `OperationFinished.diff_tokens`, `ConvertThemeResponse.styles` (`map<int32,string>`). The requests no longer have a `highlight` field.
- Reference files (read-only, from the xcl checkout): `highlight/testdata/reference/<report|entity|diff>.tokens.json` is a JSON array of `{"kind": <int>, "text": "<string>"}`, with the expected `.plain.txt`, `.terminal.txt`, `.html`, `.inline.html`, `.svg`, plus `default.css` and `classes.txt`.
- Parity record (Go and Python programs, compared key by key): new keys `printed_validate` and `printed_apply` (plain report of the failure at width 80, or `""`), `printed_entities` (address → plain configuration text with computed values, for every entity that is not a variable, output or module), `printed_diff_changed` (plain change summary, when the fixture has a changed configuration), and `log` (each event written through the logging bridge as `{"level": "debug|info|warn|error", "message", "resource", "operation"}`, sorted), with run paths replaced as the existing keys are.

## Implementation Detail

**A new public subpackage, built as a port with a contract.** `xcl.highlight` is the package's first public subpackage made of several private modules: kinds, styles and themes, formatters, the stylesheet, the colour chooser, the VS Code import and the log formatter, re-exported from one `__init__`. Each formatter is a line-for-line port of the matching Go formatter from the core plan, so a reader can hold the Go and Python files side by side: walk the tokens once, resolve each token's style, write. One small colour table (named colour to SGR code and to palette hex) is shared by the terminal, inline-HTML, SVG and stylesheet code, as in Go. The port is held to Go by the reference files rather than by code review alone: any byte that differs fails a named test for that input and format.

**The engine-error conversion grows, it is not duplicated.** The package already turns every engine error into its exception in one function. That function now also converts the report into tokens and, when there is one, adds the plain report as a note. Calls, finished operations and event errors pick it up with no change of their own. The exception's constructor takes the tokens, so an `xcl.Error` built by hand with tokens behaves the same. Formatting the note needs the plain formatter and the detail wrapper, which depend only on the kinds, so the error module imports them without an import cycle. The richer printing functions sit above both and import them.

**Printing is "tokens, then formatter", with one print-time step.** Every print path, whether error, entity, saved record or diff, obtains tokens (from the error, the diff, or an encode call with `tokens=True`), then hands them to the chosen formatter with the theme and the stream. The only transformation in Python is wrapping `DETAIL` tokens to the width, which happens in the print functions and the note, never in the tokens an extender reads. The `print_`/`format_` pair and the `file=None` default copy the standard library's `traceback` functions, so the API reads like Python, not like Go.

**Encode calls are extended in place, sharing one request builder.** The four encode choices are already built in one helper shared by both flavours. The new tokens and print methods reuse it and add the `tokens` flag, and the asyncio methods await the same call through the executor as their siblings do. Removing `highlight=` is a deletion from both signatures. Python itself then reports an unexpected keyword. No shim or `RemovedNameError` is added, because the spec asks for removal, not deprecation.

**Logging follows Go's bridge rule for rule.** The logging bridge is a closure returned by a factory, like the user's own `on_event` handlers. It checks `logger.isEnabledFor` first, as Go checks `Enabled`, builds the record with `makeRecord` so the event's time and the attributes can be set, and hands it to `logger.handle`. The log formatter subclasses `logging.Formatter` and overrides only `formatException`. It formats the traceback from a `TracebackException` whose notes have the plain report removed, then appends the formatted report, so it composes with any `fmt`, `datefmt` and style the user already passes.

**Colour stays a caller's decision.** Nothing in the package probes a stream unless `for_stream` is called. Every default is `PLAIN`, including the note and `LogFormatter`.

**Parity tests reuse existing harnesses.** The reference-output tests read files from the xcl checkout through the suite's existing `xcl_src` fixture, so they fail in strict CI rather than skip. The kind test walks the regenerated protocol enum descriptor, as earlier Python enums are checked against the wire. The "same as Go" criteria extend the existing Go and Python parity programs with new record stages that both sides compute the same way, so the existing comparison test finds any difference by its JSON path. The only Go change is in that parity program: new stages and their tests, plus one new fixture directory and a line in the fixture README.

**What stays the same.** The structured `Diff`, `Problem`, `ConfigError.problems`, `Event`, the operation loop and the transport are untouched apart from the new fields. Every existing lookup and encode call keeps its signature, minus `highlight=`.

## Dependencies

- **Design documents this plan was built on:** `printing-and-highlighting.md` from the `design` design source. It is the settled split between core and language, the kind numbering, the style syntax, the formatter protocol, the Python print API, exception notes, `logging_handler` and `LogFormatter`, the HTML class contract and the removal of `highlight=`, which this plan implements for Python.
- **Plan `20261023074926-eaa51527-printing-and-highlighting-core` (must land in xcl first).** It provides the Go formatter rules Python ports, `DefaultTheme`, Go's `PrintError`/`PrintEntity`/`PrintDiff` (used by the Go parity program), and the reference outputs under xcl's `highlight/testdata/reference/` that Python's formats are held to. Nothing in it needs to change.
- **Plan `20261023075136-42c51a7d-printing-tokens-over-the-engine` (must land in xcl first).** It provides the protocol this package regenerates its stubs from: `TokenKind`, `Token`, `Error.report`, the `tokens` flag and response field on both encode calls, `OperationFinished.diff_tokens`, `ConvertTheme` and the `invalid_theme` code, with `highlight = 2` reserved. It warns that bumping that engine into xcl-python fails the colour tests until this plan lands, which this plan resolves by landing the stub regeneration and the removal together. Nothing in it needs to change.
- **An xcl release containing both plans above (must exist before xcl-python ships).** The package bundles only an engine built from a released xcl tag pinned in `xcl-ref.txt`. During development the pin points temporarily at the xcl branch, as in plan `20261023074627-cc23fc08-encode-saved-entity-during-operation`. Before merge it names the release.
- **xcl's parity program (`bindings/libxcl/cmd/parity`, changed by this plan).** Its new printing and logging stages must be in the xcl release that xcl-python pins, because xcl-python's CI builds and runs it from that checkout. It merges into xcl before, or with, that release.
- **Plan `20261023074223-e06f835f-event-data-level-enum` (independent, overlapping).** It sets the option pattern (a top-level `StrEnum`, refused by type with `TypeError`) and ships `xcl/py.typed`, which makes this plan's public annotations part of users' type checking. It also edits `xcl/config.py`, `xcl/aio.py`, `README.md`, `PARITY.md` and the website's configuration-only and plugins example pages. Neither must land first; whichever lands second rebases.
- **Plan `20261023074627-cc23fc08-encode-saved-entity-during-operation` (independent, overlapping).** It makes `EncodeSavedEntity` allowed during an operation, so `print_saved_entity` and `saved_entity_tokens` work from an event handler once both have landed. It edits the encode docstrings, the README encoding section and the sync and asyncio integration suites. Neither must land first; whichever lands second rebases.
- **Downstream: spec `20261023075328-741138ee-python-prettylog`.** It depends on this plan, using `print_saved_entity`, `print_error`, `for_stream`, `DEFAULT_THEME` and `logging_handler` rather than its own copies.
- **`protobuf` (existing runtime dependency, `>=5.26`).** It carries the new messages. No version change.
- **`protoc` (existing development tool, run through the Makefile's protocol target; CI pins 28.x).** It regenerates `xcl/_core` from the xcl checkout. No change.
- **Python standard library (3.11+).** It provides `enum.IntEnum`, `dataclasses`, `types.MappingProxyType`, `BaseException.add_note`, `traceback.TracebackException`, `logging` and `os`. No new runtime or test dependency is added, as the spec's constraint requires.
- **`mitchellh/go-wordwrap` v1.0.1 (reference only, not a dependency).** Its `WrapString` is the algorithm Python ports for detail wrapping, so wrapped reports match Go's.
- **Go's `events.SlogHandler` (reference only, xcl, unchanged).** It provides the levels, messages and attribute names the logging bridge mirrors, and the Go parity program writes events through it.
- **pytest (existing test dependency).** No change.
- **xcl-website repo (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).** Its pages are edited after the core plan's rewrite of the same pages has landed, since that rewrite renames the highlighting sections and leaves the Python tabs for this plan. The website change goes out after the xcl-python release it documents.
- **xcl-python release flow (existing).** The release label on the PR (`minor` for this breaking pre-1.0 change), auto-tag, the release workflow and the automated engine-bump PR. A person edits the published release notes. No workflow change.

## Testing Approach

Testing follows the xcl-python suite's layout and the project's testing conventions: one behaviour per test function, no `pytest.mark.parametrize` tables, positive and negative cases in separate functions, unit tests against the fake transport, integration tests against the real engine, and the Go parity additions with testify `require` beside the parity program. No test reads the README, PARITY.md, website pages or release notes. The reference files and parity fixtures are test input, not prose.

**Contract tests: reference outputs (the load-bearing parity guarantee for formats).** For each of the three published inputs (report, entity, diff), each built-in format (plain, terminal, HTML, inline HTML, SVG) applied to the published tokens with the default theme must equal the published output byte for byte. That is one test function per input and format. The stylesheet generated from the default theme must equal the published `default.css`, and the container class plus every known kind's class must equal `classes.txt`. These tests read the files from the xcl checkout through the existing fixture, so a missing checkout fails CI rather than skipping. They carry the spec's "Formats match the reference outputs" and "Stylesheet" criteria and the constraint that Python's formats are identical to Go's.

**Contract test: kinds against the protocol.** Every `Kind` member has a protocol `TokenKind` value with the same number and the `TOKEN_KIND_` name, and every protocol value is a `Kind` member, checked in both directions over the generated descriptor.

**Parity tests against Go (the load-bearing guarantee for "the same as Go").** The Go and Python parity programs both record, for every shared fixture, the printed plain report of a failed validate and apply, the printed text of every created entity with computed values, the printed change summary, and each event's level, message, resource and operation as written through each side's logging bridge. The existing comparison fails at the first difference, so the following must equal Go's output for the same configuration:
- the report of the new fixture with an unsupported block on line 7 ("A bad configuration prints as a report");
- the resources and change summaries ("Resource and change summary");
- the event log ("Events to logging", levels and messages).
The Go program's own tests check that the new stages are present and normalised.

**Unit tests (most of the coverage, no engine).**
- Kinds: category arithmetic, class names and the fallback for an unknown kind number (category class and style, for example 1199 printing in the comment style).
- Styles: every style word and named colour, last word wins, unknown words skipped, palette hex.
- Themes: resolution through the category and then text. A theme of the default plus a comment style colours line, hash and block comments with it, and adding a line-comment style overrides only line comments ("Theme extension and fallback"). `DEFAULT_THEME` cannot be mutated, and `|` gives a new mapping. Validation names each unknown word and raises `InvalidStyleError`, while printing with that theme succeeds and ignores the word ("Invalid style words"); these are separate tests. Validating the default theme passes.
- Formatters: each built-in's behaviour beyond the references, such as terminal reset before every newline and HTML escaping.
- Detail wrapping: the port of Go's wrapper against expected strings produced by Go for runs of spaces, existing newlines, a word longer than the width, tabs and a no-break space.
- Printing from hand-built tokens: a detail longer than 80 characters gives no detail line over 80 by default, none over 40 at width 40, and one line with wrapping off ("Detail wrapping"); a non-xcl exception prints as its message and a newline.
- Errors: `from_proto` keeps the report as tokens on the error and on each cause, adds the plain report as a note only when there is one, and `str(err)` is the engine's message with no report in it: one line for the single-problem error ("One-line string form"), and Go's `Error()` text, one line per problem, for a multi-problem error (the recorded decision). A diff from a finished operation keeps its labelled text.
- Colour choice: a TTY-like stream gives terminal colour, a file or `StringIO` gives plain, and a TTY with `NO_COLOR` set gives plain ("Colour chosen from the stream"). Each is a separate test, and a real pseudo-terminal is used where the platform has one.
- Encode calls: the print and tokens methods send `tokens=True` with the four choices, for both flavours.
- Removed keyword: passing `highlight=` to `encode_entity` or `encode_saved_entity`, in either flavour, fails with `TypeError` as an unexpected keyword ("Old option gone"), one test each.
- Logging bridge from hand-built events: Go's level for each phase and log level, its message, the record attributes and `xcl_event`, the event's time, a reserved-name detail skipped, and nothing written below the logger's level.
- Log formatter: a logged `xcl.Error` with a terminal formatter writes the traceback, then the report with colour codes, and the plain note does not appear twice ("Coloured log reports"). A non-xcl exception is formatted as `logging.Formatter` does.
- Public docstrings: the existing walk covers `xcl.highlight` and the new top-level names.

**Integration tests (real engine, both flavours where the call differs).**
- Applying the unsupported-block configuration and printing the raised error writes the full report, and `format_error` returns the same text ("As a string").
- A script run in a subprocess whose apply raises that error, uncaught, prints a traceback followed by the plain report ("Unhandled error").
- `logging.exception` on the caught error writes the plain report after the traceback ("Logged with traceback").
- Printing a created resource with computed values, from a lookup and from its saved record, and printing a diff, write text whose labelled form joins to the plain output, and each labelled piece carries a kind ("Labelled text"). These run for `xcl.Config` and `xcl.aio.Config`.
- A user-written formatter passed to `print_error`, `print_entity` and `print_diff` receives the whole labelled text in one call each time and writes to the given stream ("Custom format").
- A VS Code theme colouring properties orange, loaded through the engine, colours attribute names orange when a resource is printed in the terminal format ("VS Code theme"). An unusable theme raises `InvalidThemeError`, in a separate test.
- `logging_handler` as `on_event` during an apply writes records whose resource and operation attributes are set ("Events to logging"; the comparison with Go is the parity test above).

**Regression.** The colour tests and the request-field tests of the removed keyword are deleted, not ported, because their behaviour is gone. Every other existing unit, integration and parity test passes against the regenerated stubs and the new engine.

**Success metrics.** The spec defines no post-delivery success metric. Correctness is covered by the acceptance criteria, each mapped above.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: the README and PARITY.md describe error printing, how reports appear on unhandled and logged errors, output formats, themes, stylesheet generation, custom formats and logging integration with Python examples, and the README's migration note names the replacement for `highlight=` ("Docs").
- **Manual — captured in the implementation test plan**: the website's configuration-text, problems, diff and events pages and the plugins and configuration-only examples show Python examples of the same topics, no page says the report, rendered diffs, editor themes or custom formats are unavailable from Python, and the site builds ("Docs").
- **Manual — captured in the implementation test plan**: the GitHub release notes of the shipping xcl-python version contain a line naming the replacement for the removed `highlight=` option ("Old option gone, notes present").
- **Manual — captured in the implementation test plan (descoped release task, done by a person after the merge)**: the xcl release pinned by xcl-python contains the printing core, the engine labelled text and the parity program's new stages; xcl-python's main branch pins that release, its checks pass, and the next minor version is published with the release-notes line above; the website change goes out after it.
- **Manual — captured in the implementation test plan**: a report printed with `for_stream(sys.stderr)` looks right in a real terminal, and plain with `NO_COLOR` set. The HTML (with `css(DEFAULT_THEME)`), inline HTML and SVG of a report look right in a browser.
- **Manual — captured in the implementation test plan**: an uncaught configuration error shows its report in a rich traceback (`rich.traceback.install()`) and through loguru's `logger.exception`, and `logging_handler` records reach structlog through `ProcessorFormatter` with their attributes. These libraries are not dependencies, so this is checked by hand.

**Deliberate gaps.** There is no Python test of the engine's report layout or lexing: those belong to the core and engine plans and arrive here as tokens. No Python test exercises every unknown-kind HTML class combination, since the references hold no unknown kind; the fallback is covered by the category style and class test. Wrapping is compared with Go through expected strings Go produced, not live, because the parity fixtures have no detail over 80 columns.

## Milestones & Tasks

### Milestone 1: Python styles and formats xcl's labelled text exactly as Go does, and the old colour keyword is gone

**What changes**: The package speaks the new engine protocol. The terminal-colour keyword is removed from both encode calls, and passing it now fails as an unexpected argument. A new `xcl.highlight` module gives Python programs the labelled-text vocabulary (kinds, with categories a theme can style at once), themes written as the same style strings Go uses that extend the default kind by kind and can be validated, the five built-in formats (plain text, terminal colour, class-marked HTML, inline-styled HTML and SVG), a stylesheet generated from any theme, and an opt-in colour choice made from the stream. Given the same labelled text and theme, every format writes exactly what Go writes, held to the reference files xcl publishes. A user can also write a format of their own against the same protocol.

**Validation point**: The unit tests pass against regenerated stubs: kinds match the protocol both ways, every built-in format, the stylesheet and the class names match xcl's reference files byte for byte, theme fallback and validation behave as specified, and the removed keyword is refused for both flavours. The integration suites pass against an engine built from the xcl branch carrying the core and engine plans.

#### - [x] Task: Regenerate the protocol stubs and remove the colour keyword
**Id:** e8d33eb5-0d7a-400c-83fe-2377c14a2ed5
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Point the package at an xcl checkout that carries the core and engine printing plans, regenerate the protocol message classes from it, and remove the `highlight` keyword from `encode_entity` and `encode_saved_entity` in both configuration flavours, in the same change, because the regenerated messages no longer accept the field. The tests that expected terminal colour from the engine, or checked the field on the request, are deleted. New tests show that passing the keyword now fails as an unexpected argument. During development the engine pin points at the xcl branch; the release task moves it to the released tag.

*Technical detail:* [context.md#task-regenerate-the-protocol-stubs-and-remove-the-colour-keyword](./context.md#task-regenerate-the-protocol-stubs-and-remove-the-colour-keyword)

**Acceptance criteria**:
- [x] The package's protocol messages include the token kinds, labelled text, the report on errors, labelled diff text and theme conversion, and no longer have a colour field on the encode requests
- [x] Passing `highlight=` to `encode_entity` or `encode_saved_entity`, on `xcl.Config` or `xcl.aio.Config`, fails as an unexpected keyword argument
- [x] No code, test or docstring in the package still passes or describes the removed keyword, and the existing unit and integration suites pass against an engine built from the xcl branch

#### - [x] Task: Add token kinds and themes
**Id:** 62592911-467b-4495-afe3-a989b32d809e
**Repo:** xcl-python
**Depends on:**
- e8d33eb5-0d7a-400c-83fe-2377c14a2ed5 — Regenerate the protocol stubs and remove the colour keyword
**Execution:** agent

Create the public `xcl.highlight` package with the labelled-text vocabulary: the kinds, numbered as the protocol numbers them and checked against it in both directions, each with its category and HTML class name, and the labelled-text token. Add style strings written as Go writes them, resolution of a kind's style through its category and then plain text, the default theme that a user extends with `|`, and validation that names every unknown style word through a new `xcl.InvalidStyleError`. A kind a newer engine sends that the package does not know keeps its number and falls back to its category.

*Technical detail:* [context.md#task-add-token-kinds-and-themes](./context.md#task-add-token-kinds-and-themes)

**Acceptance criteria**:
- [x] Every kind matches the protocol's kind with the same number and name, and the protocol has no kind the package lacks
- [x] A theme made from the default plus a comment style styles line, hash and block comments with it, and adding a line-comment style changes line comments only
- [x] Validating a theme with an unknown style word reports that word, and validating the default theme reports nothing
- [x] The default theme cannot be changed in place, and extending it gives a new theme
- [x] An unknown kind number resolves to its category's style and class

#### - [x] Task: Add the built-in formats and the stylesheet
**Id:** 4f94762e-6184-473d-aaf5-7cf9976dd5bb
**Repo:** xcl-python
**Depends on:**
- 62592911-467b-4495-afe3-a989b32d809e — Add token kinds and themes
**Execution:** agent

Add the formatter protocol and the five built-in formats (plain text, terminal colour, class-marked HTML, inline-styled HTML and SVG), each a port of Go's, plus the stylesheet generated from a theme and the opt-in colour choice made from a stream. Every format, the default stylesheet and the class list are tested byte for byte against the reference files xcl publishes, read from the xcl checkout the tests already use. Printing with a theme that holds an unknown style word ignores the word.

*Technical detail:* [context.md#task-add-the-built-in-formats-and-the-stylesheet](./context.md#task-add-the-built-in-formats-and-the-stylesheet)

**Acceptance criteria**:
- [x] Every built-in format applied to each published reference input writes the published output byte for byte
- [x] The stylesheet generated from the default theme equals the published stylesheet, and the HTML class names equal the published list
- [x] Formatting with a theme that contains an unknown style word succeeds and ignores the word
- [x] Asking for a format chosen from a terminal stream gives terminal colour; from a file, or with `NO_COLOR` set, it gives plain text
- [x] A format written by a user against the formatter protocol is accepted where a built-in one is

### Milestone 2: Errors, resources and change summaries print in one call, and reports appear on unhandled and logged errors

**What changes**: A Python program prints any xcl error to a stream, or gets it as a string, in one call: a configuration problem shows the same report Go prints, with severity, summary, location, numbered source lines with the fault marked and detail wrapped at 80 columns, another width, or not at all. An uncaught configuration error, or one logged with its traceback, now shows that report after the traceback with no code at all, while `str(err)` stays the one-line message. Resources, from a lookup or a saved record, and change summaries print in one call with the existing encode choices and any format and theme. Their labelled text, and an error's, is available to extenders. A VS Code colour theme can be loaded and used as a theme.

**Validation point**: The unit and integration suites pass for both configuration flavours. That includes the unsupported-block report printed and formatted, the traceback and logged-exception reports, detail wrapping at 80, 40 and off, labelled text joining to plain output, a custom format receiving the whole text for errors, resources and diffs, and the orange-properties VS Code theme colouring attribute names.

#### - [x] Task: Keep reports on errors and print them in one call
**Id:** 48f95336-5609-483b-b6b5-61630b7417c9
**Repo:** xcl-python
**Depends on:**
- 4f94762e-6184-473d-aaf5-7cf9976dd5bb — Add the built-in formats and the stylesheet
**Execution:** agent

Make every xcl error keep the engine's report as labelled text and carry the plain report, wrapped at 80 columns, as an exception note. Tracebacks and logged exceptions then show it with no code, while `str(err)` stays the one-line message. Add `print_error` and `format_error`, shaped like the standard library's traceback pair, with a format, a theme and a detail width. Detail is wrapped by a port of Go's wrapping function, so plain output matches Go's byte for byte. Any other exception prints as its message.

*Technical detail:* [context.md#task-keep-reports-on-errors-and-print-them-in-one-call](./context.md#task-keep-reports-on-errors-and-print-them-in-one-call)

**Acceptance criteria**:
- [x] Applying a configuration with an unsupported block on line 7 and printing the raised error writes the full report: summary, location naming line 7, numbered source lines with the fault, and detail
- [x] Getting that error's report as a string returns the same text the print writes
- [x] Running a script whose apply raises that error, uncaught, prints a traceback followed by the plain report
- [x] Logging the caught error with its traceback through standard logging writes the plain report after the traceback
- [x] Converting the error to a string gives a single line
- [x] A detail longer than 80 characters prints with no detail line over 80 columns by default, none over 40 at width 40, and on one line with wrapping off
- [x] The error's labelled text joins to its plain report, and each piece carries a kind

#### - [x] Task: Print resources and change summaries in one call
**Id:** 340791f4-6541-43b8-8a71-24acd8fbfb32
**Repo:** xcl-python
**Depends on:**
- 48f95336-5609-483b-b6b5-61630b7417c9 — Keep reports on errors and print them in one call
**Execution:** agent

Give both configuration flavours one call each to print an entity and a saved record as configuration text, and to get their labelled text, with the existing encode choices and any format and theme. Keep a diff's change summary as labelled text when an operation returns it, and add `print_diff`. A user's own format receives the whole labelled text at once for errors, entities and diffs.

*Technical detail:* [context.md#task-print-resources-and-change-summaries-in-one-call](./context.md#task-print-resources-and-change-summaries-in-one-call)

**Acceptance criteria**:
- [x] Printing a created resource with computed values, from a lookup and from its saved record, writes its configuration text, on both configuration flavours
- [x] Printing a diff writes its change summary
- [x] The labelled text of a resource and of a change summary each joins to the matching plain output, and each piece carries a kind
- [x] A user-written format passed to error, resource and change-summary printing receives the whole labelled text in one call each time and writes to the given stream
- [x] Printing a diff that carries no labelled text fails with a message saying where printable diffs come from

#### - [x] Task: Load VS Code themes through the engine
**Id:** d072bb20-0524-4ea4-b342-01e8b2af6ced
**Repo:** xcl-python
**Depends on:**
- 340791f4-6541-43b8-8a71-24acd8fbfb32 — Print resources and change summaries in one call
**Execution:** agent

Add `load_vscode_theme`, which sends a VS Code colour theme, from a path or its bytes, to the engine's configuration-free theme conversion and returns the styles keyed by kind, as Go's loader does. An unusable theme raises a new `xcl.InvalidThemeError`, registered for the engine's theme error code, carrying the engine's reason.

*Technical detail:* [context.md#task-load-vs-code-themes-through-the-engine](./context.md#task-load-vs-code-themes-through-the-engine)

**Acceptance criteria**:
- [x] Loading a VS Code theme that colours properties orange and printing a resource with it in terminal colour colours attribute names orange
- [x] Loading works with no configuration created
- [x] An unusable theme raises the theme error with the engine's reason

### Milestone 3: Events reach standard logging, logged errors render in colour, and Python's output is proven identical to Go's

**What changes**: One line, `on_event=xcl.logging_handler(log)`, sends xcl's events to Python's standard logging at the levels and with the messages Go's logging bridge uses, with each event's resource, operation and other fields on the record for formatters, structlog and loguru. A logging handler given `xcl.highlight.LogFormatter` renders a logged xcl error's report in the format of the user's choice, coloured on a terminal. Go's and Python's parity programs now also compare printed reports, printed resources, printed change summaries and the event log for every shared fixture, so any difference from Go fails the build. This includes a configuration with an unsupported block on line 7.

**Validation point**: The logging unit and integration tests pass. The parity suite passes with its new stages on every fixture, including the new unsupported-block fixture, against the xcl parity program built from the branch. The Go parity program's own tests pass.

#### - [x] Task: Send events to standard logging
**Id:** 58c66252-582e-4816-9771-df57b1534a87
**Repo:** xcl-python
**Depends on:**
- e8d33eb5-0d7a-400c-83fe-2377c14a2ed5 — Regenerate the protocol stubs and remove the colour keyword
**Execution:** agent

Add `xcl.logging_handler`, which returns an event handler that writes each event to a standard logger at the level and with the message Go's logging bridge uses. Each record carries the event's time, its fields under Go's attribute names and the event itself, and the logger's level is checked first. It works with both configuration flavours and needs no adapter for structlog or loguru.

*Technical detail:* [context.md#task-send-events-to-standard-logging](./context.md#task-send-events-to-standard-logging)

**Acceptance criteria**:
- [x] Log events are written at their own level with their own message, lifecycle events at info as "operation phase", error events at error and blocked events at warn
- [x] Each record carries the event's resource, operation and other fields, and the event itself
- [x] Nothing is written for an event below the logger's level
- [x] Applying a configuration with the bridge as the event handler writes records for the apply's events

#### - [x] Task: Render logged errors in a chosen format
**Id:** 99b65f1b-6fea-407f-b820-095d71b47da5
**Repo:** xcl-python
**Depends on:**
- 48f95336-5609-483b-b6b5-61630b7417c9 — Keep reports on errors and print them in one call
**Execution:** agent

Add `xcl.highlight.LogFormatter`, a standard logging formatter that, for a logged xcl error, writes the traceback without the plain report note and then the report in the format the user chose. A coloured console handler becomes one line. Other exceptions are formatted as the standard formatter does, and the formatter takes the standard formatter's options.

*Technical detail:* [context.md#task-render-logged-errors-in-a-chosen-format](./context.md#task-render-logged-errors-in-a-chosen-format)

**Acceptance criteria**:
- [x] A logging handler given the formatter with terminal colour, logging a caught xcl error with its traceback, writes the report with terminal colour codes
- [x] The report appears once, not also as the plain note
- [x] A logged exception that is not an xcl error is written exactly as the standard formatter writes it

#### - [x] Task: Record printing and logging in the Go parity program
**Id:** e542e38c-a05b-4fc2-be18-28f631966e21
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Extend xcl's Go parity program so its record also holds the plain printed report of a failed validate and apply, the printed text of every created entity with computed values, the printed change summary, and the level, message, resource and operation of each event written through Go's logging bridge. Add a fixture whose configuration has an unsupported block on line 7, and document the new record sections in the fixture README. This requires the printing-core plan to have landed, and it must merge before the xcl release that xcl-python pins.

*Technical detail:* [context.md#task-record-printing-and-logging-in-the-go-parity-program](./context.md#task-record-printing-and-logging-in-the-go-parity-program)

**Acceptance criteria**:
- [x] The record of every fixture holds the printed report, entities, change summary and event log, with run paths replaced
- [x] The new fixture's printed report shows the unsupported block on line 7 with its numbered source lines
- [x] The record is still the same on every run of the same fixture

#### - [x] Task: Compare printing and logging with Go in the Python parity suite
**Id:** 589b2736-a0cc-4e6b-874d-eded19ac61d4
**Repo:** xcl-python
**Depends on:**
- e542e38c-a05b-4fc2-be18-28f631966e21 — Record printing and logging in the Go parity program
- 340791f4-6541-43b8-8a71-24acd8fbfb32 — Print resources and change summaries in one call
- 58c66252-582e-4816-9771-df57b1534a87 — Send events to standard logging
**Execution:** agent

Extend the Python parity program to record the same new sections through the package's own printing and logging bridge, and add a parity test for the new fixture. The existing comparison then holds Python's printed reports, resources, change summaries and event log to Go's on every fixture.

*Technical detail:* [context.md#task-compare-printing-and-logging-with-go-in-the-python-parity-suite](./context.md#task-compare-printing-and-logging-with-go-in-the-python-parity-suite)

**Acceptance criteria**:
- [x] A Python program that applies the configuration with an unsupported block on line 7 and prints the raised error as plain text writes the same report Go writes, byte for byte
- [x] Printing every created resource with computed values, and every change summary, writes the same text as Go for every fixture
- [x] The logging bridge writes records whose levels and messages match Go's logging bridge for the same apply, with the resource and operation on each

### Milestone 4: The package docs, the parity list, the website and the release describe Python printing

**What changes**: Readers of the package README, the parity list and xcl.dev learn how to print errors, how reports appear on unhandled and logged errors, the output formats, themes, stylesheet generation, custom formats and logging integration, with Python examples. The website no longer says the report, rendered diffs, editor themes or custom formats are unavailable from Python. The change ships as a minor xcl-python release bundling the xcl release that carries the engine work. Its release notes name the replacement for the removed colour keyword.

**Validation point**: The manual reviews of the README, PARITY.md and the website pass, and the site builds. The xcl-python release bundling the xcl release is published, and its notes carry the replacement line.

#### - [x] Task: Document Python printing in the README and parity list
**Id:** fa37b310-5b2c-4cdb-9047-190aaf302d66
**Repo:** xcl-python
**Depends on:**
- 340791f4-6541-43b8-8a71-24acd8fbfb32 — Print resources and change summaries in one call
- d072bb20-0524-4ea4-b342-01e8b2af6ced — Load VS Code themes through the engine
- 58c66252-582e-4816-9771-df57b1534a87 — Send events to standard logging
- 99b65f1b-6fea-407f-b820-095d71b47da5 — Render logged errors in a chosen format
**Execution:** agent

Rewrite the README's encoding and error sections to document printing errors, how reports appear on unhandled and logged errors, resources and change summaries, output formats, themes and validation, VS Code themes, stylesheet generation, custom formats, colour choice and logging integration, with Python examples. Add a migration note giving the replacement for the removed `highlight=` keyword. Update the parity list's encode rows and add printing and logging rows that pair each Go function with its Python form.

*Technical detail:* [context.md#task-document-python-printing-in-the-readme-and-parity-list](./context.md#task-document-python-printing-in-the-readme-and-parity-list)

**Acceptance criteria**:
- [x] The README shows Python examples of error printing, reports on unhandled and logged errors, output formats, themes, stylesheet generation, custom formats and logging
- [x] The README has a migration note naming the replacement for the removed `highlight=` keyword
- [x] The parity list pairs Go's printing, theme, format and logging functions with their Python forms, and no longer lists `highlight=`

#### - [x] Task: Document Python printing on the website
**Id:** 3ecb953b-80ea-414f-b7bc-fd2c57fc47d5
**Repo:** xcl-website
**Depends on:**
- 340791f4-6541-43b8-8a71-24acd8fbfb32 — Print resources and change summaries in one call
- d072bb20-0524-4ea4-b342-01e8b2af6ced — Load VS Code themes through the engine
- 58c66252-582e-4816-9771-df57b1534a87 — Send events to standard logging
- 99b65f1b-6fea-407f-b820-095d71b47da5 — Render logged errors in a chosen format
**Execution:** agent

Rewrite the Python tabs of the website's configuration-text, problems, diff and events pages, and of the plugins and configuration-only examples, to show Python printing, formats, themes, stylesheet generation, custom formats and logging. Remove every statement that the report, rendered diffs, editor themes or custom renderers are unavailable from Python. Pages are edited in place on top of the printing-core plan's rewrite of the same pages, with none renamed or removed.

*Technical detail:* [context.md#task-document-python-printing-on-the-website](./context.md#task-document-python-printing-on-the-website)

**Acceptance criteria**:
- [x] The problems and configuration-text pages show Python examples of error printing, reports on unhandled and logged errors, output formats, themes, stylesheet generation, custom formats and logging
- [x] No page says the report, rendered diffs, editor themes or custom formats are unavailable from Python
- [x] Every existing page URL still works and the site builds

**Descoped requirements**:
- Release the engine and the package with the printing work (task ab1f8a85-7805-4263-9f9b-2b18ea1e6a27) — descoped: done by a person after the merge. Merging pull requests with release labels, publishing releases and editing GitHub release notes happen outside the repos and only once this spec's branches are merged. What the person must do: (1) in xcl, make sure the core plan, the engine plan and this spec's parity-program change ("Record printing and logging in the Go parity program") are merged and released through the release label on their pull requests (a separate `patch` release if the parity change merges after the other two are released); (2) in xcl-python, set `xcl-ref.txt` to that release tag, either on this work's branch or by pushing this work onto the automated `engine/<tag>` pull request (which otherwise fails its checks, see xcl-python `gotchas/ci-regenerates-stubs.md`), closing the other; (3) merge the xcl-python pull request with the `minor` label so the next minor version is published; (4) edit that GitHub release's generated notes and add: "Breaking: `highlight=` was removed from `encode_entity` and `encode_saved_entity`; use `cfg.print_entity(entity, formatter=xcl.highlight.TERMINAL)` (or `print_saved_entity`) to print coloured configuration text, or `entity_tokens` for labelled text."; (5) merge the website pull request after the package release is on PyPI. Checked by `xcl.engine_version()` of the released package reporting the xcl release tag, the release page showing the added line, and the website showing the new Python tabs. Covers the spec's "Replacement for the previous option is documented" (release-notes half) and the release-notes half of "Old option gone, notes present", and the constraint that the bundled engine is an xcl release.

## Open Questions

- **Are the Go and Python event logs for a parity fixture identical and stable?** Both sides run the same engine code, so the same apply should emit the same events, and the Go parity program's existing determinism test reruns each fixture. But only running the fixtures shows whether a plugin's debug messages carry timings or paths that vary, or whether the engine path adds or drops an event. It depends on the parity fixtures and the engine plan's event delivery. If the log stage is not deterministic, or differs only there, the implementer must STOP and ask the user, not filter entries or drop the stage.
- **Do the landed Go sources match the formatter rules recorded from the core plan?** The plan's per-task notes repeat the core plan's byte rules, but the authority is the landed Go code and its reference files, which do not exist yet. It depends on the core plan as implemented. Where the two differ, the implementer follows the Go source and the reference files. If a reference file is missing, renamed or shaped differently (for example, the token JSON is not a list of `{"kind", "text"}`), STOP and ask.

Everything else was settled during planning and is recorded in the drafting assumptions.

## Out of Scope

- **The Python pretty logger** (spec Non-Goal). The ready-made coloured event logger, `XCL_LOG_LEVEL` and resources printed beneath their creation events belong to spec `20261023075328-741138ee-python-prettylog`, which builds on this plan's printing, `print_saved_entity` and `logging_handler`. The website's pretty-receiver Python tab is left for it.
- **Dedicated adapters for structlog or loguru** (spec Non-Goal). They receive `logging_handler`'s records and exception notes through their own stdlib integration. That is checked by hand, not shipped as code.
- **A TypeScript package** (spec Non-Goal), including the design's `xcl.css` file shipped with it.
- **Any change to the engine, its protocol or the Go library.** Lexing, report layout, labelled text, theme conversion and the reference outputs come from plans `20261023074926-eaa51527-printing-and-highlighting-core` and `20261023075136-42c51a7d-printing-tokens-over-the-engine`. The only xcl change here is the parity program's new stages and fixture.
- **Lexing configuration text in Python** (spec constraint). There is no `highlight.tokenize` and no `Kind.scope()` in Python: labelled text comes only from the engine, and VS Code themes are converted by it.
- **Changing what `str(err)` or `err.message` say.** They stay the engine's message, Go's `Error()`, per the recorded decision the user confirmed during epic planning. The report reaches people through notes and the print functions.
- **A deprecation period for `highlight=`** (spec constraint). The keyword is removed outright.
- **Structured problem fields on log records.** `logging_handler` mirrors Go's `SlogHandler`, which writes the error as one line. The design's decision not to implement `slog.LogValuer` in Go applies to Python too.
- **Changing the release workflows.** The breaking-change line is added by hand to this release's generated notes. xcl-python keeps no changelog file.
- **New website pages or navigation.** Python examples go into the existing pages' Python tabs only.
- **Fixing type errors that `py.typed` may surface in unrelated annotations.** This plan keeps its own public annotations accurate and adds no type-checker test.

## Changelog

### Plan change — 2026-10-26: release task descoped to follow-up work

**Change**: Took the human task "Release the engine and the package with the printing work" (ab1f8a85) out of the `## Milestones & Tasks` checklist and listed it under **Descoped requirements** as done by a person after the merge, with what the person must do. Added the release checks to the manual reviews in `## Testing Approach` so the implementation test plan carries them.
**Why**: The user decided for this epic that human release tasks are follow-up work: the workflow has no step to finish a human task, and the releases can only happen after the spec's branches are merged.
**Effect**: The release-notes line naming the replacement for `highlight=`, the released-engine pin in `xcl-ref.txt` and the published minor version are met by the person's follow-up, not by this run. During this run the Python suites run against an engine, plugins and parity program built from this spec's xcl worktree.

### 2026-10-26 — Task: Regenerate the protocol stubs and remove the colour keyword

**What was done**: Regenerated `xcl/_core/core_pb2.py`/`.pyi` from the spec's xcl worktree, so the package's messages carry `TokenKind`, `Token`, `Error.report`, the encode `tokens` flag and response field, `OperationFinished.diff_tokens` and `ConvertTheme`, with no `highlight` field. Removed `highlight=` from `encode_entity` and `encode_saved_entity` on `xcl.Config` and `xcl.aio.Config` (signatures, requests, docstrings), deleted the colour tests, renamed the record tests, and added four tests showing the keyword is refused as an unexpected argument.

**Deviations**: `xcl-ref.txt` stays at `v0.11.1`, as in plan `20261023074627-cc23fc08-encode-saved-entity-during-operation`: the development pin to an xcl branch only serves CI, this run never pushes, and the xcl branch is local. The suites ran against an engine, plugins and stubs built from the xcl worktree (`make protos engine plugins XCL_SRC=<xcl worktree>`); the descoped release task sets the pin to the release. The README example and paragraph and the PARITY.md encode rows lost `highlight` now (rather than in the documentation task), so no document describes the removed keyword in the meantime. The unused `ANSI`/`strip_colour` helpers in `tests/integration/test_sync.py` went with the colour tests.

**Files changed**:
- `xcl-python: xcl/_core/core_pb2.py`
- `xcl-python: xcl/_core/core_pb2.pyi`
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/aio.py`
- `xcl-python: tests/unit/test_config.py`
- `xcl-python: tests/integration/test_sync.py`
- `xcl-python: tests/integration/test_aio.py`
- `xcl-python: README.md`
- `xcl-python: PARITY.md`

**Discoveries**: Local `protoc` is 36.1 and the committed stubs were already generated with it (`Protobuf Python Version: 7.36.1`), so regenerating locally adds no version churn. With the regenerated stubs and the worktree engine the whole suite passes (538 passed, 1 skipped).

### 2026-10-26 — Task: Add token kinds and themes

**What was done**: Added the public `xcl.highlight` package with `Kind` (an `IntEnum` of the protocol's 41 kinds, with `parent` and `css_class`), `parent_of`, `css_class_of`, the frozen `Token`, private protocol converters that keep an unknown kind as its number, `Style`/`parse_style`/`hex_colour` (a port of Go's `style.go`, plus the private SGR helper the terminal format will use), and `Theme`, the immutable `DEFAULT_THEME`, `style_for` and `validate`. Added `xcl.InvalidStyleError` (code `invalid_style`, `words` as `(kind, style, word)` tuples, one message line per word in Go's `style "<style>" for <Kind>: unknown word "<word>"` form) and exported it and `xcl.highlight` from the top level.

**Deviations**: The kind helper `kinds()` and a public `known()` were not added: iterating `Kind` already gives the kinds in numeric order, and `_known` stays private. The docstring walk now skips module constants (such as `DEFAULT_THEME`, a `MappingProxyType` that cannot carry a docstring) in the per-module loop, the same rule its `xcl.__all__` loop already applies. Messages name kinds as Go does (`LineComment`, `Kind(1199)`), not by the Python member name, so they match Go's `InvalidStyleError`.

**Files changed**:
- `xcl-python: xcl/highlight/__init__.py`
- `xcl-python: xcl/highlight/_kinds.py`
- `xcl-python: xcl/highlight/_style.py`
- `xcl-python: xcl/highlight/_theme.py`
- `xcl-python: xcl/_errors.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_docstrings.py`
- `xcl-python: tests/unit/test_highlight_kinds.py`
- `xcl-python: tests/unit/test_highlight_style.py`
- `xcl-python: tests/unit/test_highlight_theme.py`

**Discoveries**: `validate` imports `InvalidStyleError` inside the function and `InvalidStyleError` imports the kind names inside its constructor, so `xcl._errors` and `xcl.highlight` never import each other at module load; the later error-notes task must keep `xcl._errors` free of a module-level import of `xcl.highlight` modules that import `xcl._errors` back. Go's `Theme.Validate` reports an empty style string as nothing, and so does the port.

### 2026-10-26 — Task: Add the built-in formats and the stylesheet

**What was done**: Added the `Formatter` protocol and the built-in formats `PLAIN`, `TERMINAL`, `HTML`, `HTML_INLINE` and `SVG` (`xcl/highlight/_formatters.py`), each a line-for-line port of the landed Go formatter; `css(theme)` (`_css.py`), a port of `WriteCSS`; and the opt-in `for_stream` (`_stream.py`). All 15 reference outputs, `default.css` and `classes.txt` match byte for byte. Tests: 15 reference tests plus stylesheet and class-list tests, formatter behaviour tests (escaping, per-line reset, unknown kind 1199, unknown style word, a user formatter typed as `Formatter`), CSS rule tests and eight `for_stream` tests including a real pty.

**Deviations**: The landed Go code matched the plan's byte rules, with two details the notes did not spell out and the port follows: a stylesheet rule ends `; }` (`.xcl .x { a; b; }`), and a named background colour (only reachable programmatically, since `bg:` takes hex) uses SGR code+10. `Theme` became `Union[Mapping[int, str], Mapping[Kind, str]]` and `DEFAULT_THEME` is annotated `types.MappingProxyType[int, str]`, because `Mapping[int, str]` rejected a `dict[Kind, str]` variable and `Mapping` has no `|` for mypy; two mypy tests in `tests/unit/test_typing.py` hold the documented theme idioms (`DEFAULT_THEME | {...}`, a dict of `Kind` keys, a dict of numbers, an annotated `Theme`) and flag a non-string style. The tests were written by a sub-agent as the plan's High complexity requires.

**Files changed**:
- `xcl-python: xcl/highlight/_formatters.py`
- `xcl-python: xcl/highlight/_css.py`
- `xcl-python: xcl/highlight/_stream.py`
- `xcl-python: xcl/highlight/_theme.py`
- `xcl-python: xcl/highlight/__init__.py`
- `xcl-python: tests/unit/test_highlight_reference.py`
- `xcl-python: tests/unit/test_highlight_formatters.py`
- `xcl-python: tests/unit/test_highlight_css.py`
- `xcl-python: tests/unit/test_highlight_stream.py`
- `xcl-python: tests/unit/test_typing.py`

**Discoveries**: Recorded as knowledge entry `gotchas/theme-typing-mapping-keys.md` (repo tier, xcl-python store): `Mapping` key invariance and the missing `Mapping.__or__` make the natural `Theme = Mapping[int, str]` reject documented usage under `py.typed`. Formatter instances are module constants, which the docstring walk now skips like other constants; their private classes carry the docstrings.

### 2026-10-26 — Task: Keep reports on errors and print them in one call

**What was done**: Every `xcl.Error` now keeps the engine's report as `err.tokens` (converted in `from_proto` for the error and each cause) and, when it has one, carries the plain report wrapped at 80 columns as an exception note, so tracebacks and `logging.exception` show it; `str(err)` stays the engine's message. Added `xcl._wrap.wrap_detail` (an exact port of go-wordwrap v1.0.1 `WrapString`, checked against strings Go produced), `xcl._render` (detail-only wrapping, render, plain report) and `xcl.print_error`/`xcl.format_error`, shaped like `traceback.print_exception`/`format_exception`. Tests (written by a sub-agent, as the High complexity requires): wrap cases from Go, error tokens and notes, print/format on hand-built reports, and real-engine tests of the line-7 unsupported block (printed, as a string, one-line `str`, logged, uncaught in a subprocess, tokens joining to the plain report) for both flavours.

**Deviations**: None in behaviour. `Error.__init__` imports `plain_report` only when tokens are given and `from_proto` imports the token converter inside the function, so `xcl._errors` still imports no `xcl.highlight` module at load. The report fixture is a ten-line configuration (`sync_app "web"`, blank line, `sync_app "api"` with a nested `timeout` block opening on line 7) rather than nine lines; the engine shows lines 5 to 9 around the fault.

**Files changed**:
- `xcl-python: xcl/_wrap.py`
- `xcl-python: xcl/_render.py`
- `xcl-python: xcl/_print.py`
- `xcl-python: xcl/_errors.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_wrap.py`
- `xcl-python: tests/unit/test_print.py`
- `xcl-python: tests/unit/test_errors.py`
- `xcl-python: tests/integration/test_sync.py`
- `xcl-python: tests/integration/test_aio.py`

**Discoveries**: The engine's unsupported-block `ConfigError` carries the report on the top-level error and on its one cause, so both get a note; a traceback shows the top-level note once because causes are not chained as `__cause__`. Go's `unicode.IsSpace` differs from Python's `str.isspace` (Python also counts U+001C–U+001F), so the wrapper spells out the White_Space set. A strict local run of the whole suite hits GitHub's anonymous API rate limit in the plugin-release tests; `GITHUB_TOKEN=$(gh auth token)` avoids it (the registry client reads `GITHUB_TOKEN`).

### 2026-10-26 — Task: Print resources and change summaries in one call

**What was done**: Both configuration flavours gained `entity_tokens`, `print_entity`, `saved_entity_tokens` and `print_saved_entity`, which ask the engine for labelled text (`tokens=True`) with the four existing encode choices and hand the whole token list to the chosen formatter (`sys.stdout` by default). The encode requests are built by shared `_encode_entity_request`/`_encode_saved_entity_request` helpers in `xcl/config.py`, imported by `xcl/aio.py`. `Diff` gained `tokens` (excluded from equality and repr), filled from `OperationFinished.diff_tokens` in `Operation.finish`, and `xcl.print_diff` prints it, raising `ValueError` naming `Config.diff` for a diff without labelled text. Unit tests on the fake transport (requests carry `tokens`, output, stdout default, custom format, diff tokens kept for both flavours) and real-engine tests with the recording plugin (computed text from a lookup and a saved record, token joins, the git-style change summary, a recording custom format for error, entity and diff, and asyncio twins).

**Deviations**: The finished-diff unit tests live in `tests/unit/test_config.py` (driving `Config.diff` over the fake transport) rather than a new `tests/unit/test_poll.py`, which the plan allowed as an alternative. Added `test_encode_saved_entity_does_not_ask_for_tokens` beside the planned `encode_entity` one.

**Files changed**:
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/aio.py`
- `xcl-python: xcl/diff.py`
- `xcl-python: xcl/_poll.py`
- `xcl-python: xcl/_print.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_config.py`
- `xcl-python: tests/unit/test_print.py`
- `xcl-python: tests/integration/test_sync.py`
- `xcl-python: tests/integration/test_aio.py`

**Discoveries**: A diff over config-only (Python-registered) types reports no changes ("Diff: no changes, 0 unchanged."), as Go does: only provider-backed resources appear in a change summary, so diff printing tests need a plugin such as the recording fixture.

### 2026-10-26 — Task: Load VS Code themes through the engine

**What was done**: Added `xcl.highlight.load_vscode_theme(source)`, which reads a VS Code colour theme from a path or bytes, sends it to the engine's configuration-free `ConvertTheme` on handle 0 and returns only the styled kinds (as `Kind` members, an unknown number kept as an `int`), as Go's `LoadVSCodeTheme` does. Added `xcl.InvalidThemeError` (code `invalid_theme`, registered for the engine's code, `details["source"]`/`["reason"]`). Unit tests on the fake transport (handle 0, no `CreateConfig`, path and bytes, kind mapping, error class) and real-engine tests (the orange-properties theme colours `image` with `\x1b[38;2;255;136;0m` under `TERMINAL`, only styled kinds returned, loading in a fresh process with no configuration, an unusable colour raising `InvalidThemeError` with the engine's reason).

**Deviations**: None. `xcl/config.py` (named in the task's scope) needed no change.

**Files changed**:
- `xcl-python: xcl/highlight/_vscode.py`
- `xcl-python: xcl/highlight/__init__.py`
- `xcl-python: xcl/_errors.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_highlight_vscode.py`
- `xcl-python: tests/unit/test_errors.py`
- `xcl-python: tests/integration/test_sync.py`

**Discoveries**: `load_vscode_theme` imports the client inside the function, so `import xcl.highlight` still starts nothing; unit tests replace `xcl._client.default_client` with `monkeypatch` to route the call to the fake transport.

### 2026-10-26 — Task: Send events to standard logging

**What was done**: Added `xcl.logging_handler(logger=None)` (`xcl/_logbridge.py`), an `on_event` handler that mirrors Go's `events.SlogHandler`: the logger's level is checked first; log events are written at their own level (debug/info/warn/error, anything else info) with their own message, other events as `"<operation> <phase>"` at info, error for phase `error` and warning for `blocked`. Records carry the event's time and, as attributes under Go's names, `source`, `operation`, `phase`, `resource`, `type`, `file`, `duration` (a `timedelta`), `error` (the `xcl.Error`), the sorted event details, and `xcl_event`. Unit tests per level and phase, attributes, ordering, reserved names, time and the default logger; integration tests on a real apply for both flavours.

**Deviations**: Following Go exactly, a log event whose `level` or `message` detail is not a string is written at info with an empty message (Go's type assertion), rather than `str(...)` as the plan's note suggested. An event detail that collides with an attribute already set (such as a detail named `resource`) is skipped, keeping Go's first attribute, since a record cannot hold two values for one name. The record's `relativeCreated` is moved by the same offset as `created`, which needs no private `logging` state and works on every supported Python.

**Files changed**:
- `xcl-python: xcl/_logbridge.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_logbridge.py`
- `xcl-python: tests/integration/test_sync.py`
- `xcl-python: tests/integration/test_aio.py`

**Discoveries**: None beyond the existing `gotchas/aio-events-out-of-order.md`: under `xcl.aio.Config` the bridge's records can arrive out of order, so the asyncio integration test checks membership, not order.

### 2026-10-26 — Task: Render logged errors in a chosen format

**What was done**: Added `xcl.highlight.LogFormatter`, a `logging.Formatter` taking the standard options (`fmt`, `datefmt`, `style`, `validate`, `defaults`) plus `formatter`, `theme` and `width`. Its `formatException` writes, for an `xcl.Error` carrying a report, the traceback from a `TracebackException` whose notes drop the plain report note (user notes stay), then the report from `format_error` in the chosen format; any other exception goes to the standard `formatException`. Unit tests (coloured report, report once, traceback first, plain by default, user notes kept, other exceptions and report-less xcl errors as standard, `fmt` and `{`-style honoured, detail width) and a real-engine test with the line-7 report.

**Deviations**: None.

**Files changed**:
- `xcl-python: xcl/highlight/_logging.py`
- `xcl-python: xcl/highlight/__init__.py`
- `xcl-python: tests/unit/test_highlight_logging.py`
- `xcl-python: tests/integration/test_sync.py`

**Discoveries**: An unsupported-block error's one-line message itself contains the detail sentence (`...: Unsupported block type; Blocks of type "timeout" are not expected here. (...)`), so a test counting the report's appearances must count the report's own lines (such as the detail at the start of a line), not the sentence anywhere.

### 2026-10-26 — Task: Record printing and logging in the Go parity program

**What was done**: xcl's Go parity program (`bindings/libxcl/cmd/parity`) now records `printed_validate` and `printed_apply` (the plain report `xcl.PrintError` prints, or `""`), `printed_diff_before` and, with a changed configuration, `printed_diff_changed` (the summary `xcl.PrintDiff` prints, or the outcome), `printed_entities` (`xcl.PrintEntity` with `IncludeComputed()` for every non-builtin entity), and `log` (every event up to the load written through `events.SlogHandler` into a recording `slog.Handler`, as `{level, message, resource, operation}`, sorted), all with run paths replaced. `newConfig` takes an optional event handler. A new `unsupported_block` fixture (an `app` with a nested `timeout` block opening on line 7) and README sections document the stages. Go tests (testify `require`) cover the new keys, the exact line-7 report, printed entities and diffs, the log entries and order, path normalisation and determinism of the new fixture.

**Deviations**: Added `printed_diff_before` (not in the plan): the goal check read "the record of every fixture holds the ... change summary" literally, and `printed_diff_changed` exists only for fixtures with a changed configuration, so the diff before the apply is printed for every fixture as well (a failed diff records its outcome). The Python parity task must record the same key. The parity program's own record is built only by `go test ./cmd/parity/`; xcl-python's `build/plugins/parity` is rebuilt in the next task.

**Files changed**:
- `xcl: bindings/libxcl/cmd/parity/main.go`
- `xcl: bindings/libxcl/cmd/parity/main_test.go`
- `xcl: bindings/libxcl/testdata/parity/unsupported_block/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/unsupported_block/types.json`
- `xcl: bindings/libxcl/testdata/parity/README.md`

**Discoveries**: The event log is deterministic across runs (five repeated runs of each fixture agree) once sorted: plugin debug messages (`starting plugin`, `plugin address`, `waiting for RPC address`, ...) carry no timings or paths in their message. The unsupported-block fixture's validate, diff and apply all fail; its log shows `validate error` events for `app.api` from the parse stage beside the top-level `validate error`.

### 2026-10-26 — Task: Compare printing and logging with Go in the Python parity suite

**What was done**: The Python parity program (`tests/parity/run_python.py`) now records the same new keys as Go's through the package's own API: `printed_validate`/`printed_apply` (`xcl.format_error`), `printed_diff_before`/`printed_diff_changed` (`xcl.print_diff`), `printed_entities` (`Config.print_entity(..., include_computed=True)`) and `log` (every event through `xcl.logging_handler` into a recording handler, sorted as Go sorts it). `new_config` takes an `on_event`. `tests/parity/types.py` gained the `unsupported_block` fixture's type, with its `types.json` check, and `test_parity.py` a comparison test for the new fixture plus explicit checks of the line-7 report and of the printing and log keys for `plugin_entities`. The rebuilt Go parity program and the Python program agree on every fixture, logs included.

**Deviations**: Python also records `printed_diff_before`, matching the Go key added during the previous task's goal check. The plan's open question about the log stage did not arise: Go's and Python's event logs agree on every fixture with no filtering.

**Files changed**:
- `xcl-python: tests/parity/run_python.py`
- `xcl-python: tests/parity/types.py`
- `xcl-python: tests/parity/test_types.py`
- `xcl-python: tests/parity/test_parity.py`

**Discoveries**: The goal reviewer's reply must be bare JSON for `implement goal record`; a reply with prose around the JSON is refused and the reviewer is asked again.

### 2026-10-26 — Task: Document Python printing in the README and parity list

**What was done**: The README gained a "Printing and highlighting" section (printing errors with an example report, reports on unhandled and logged errors through exception notes, `str(err)` and `err.tokens`; printing resources, saved records and change summaries; the five output formats and `for_stream`; themes, `validate` and `load_vscode_theme`; the HTML classes and `css`; writing a format), a "Logging" section (`logging_handler` and `LogFormatter`), a "Migrating from highlight=" note naming `print_entity(..., formatter=xcl.highlight.TERMINAL)`, `print_saved_entity` and `entity_tokens`, a pointer from the encoding section and a line in "Errors". PARITY.md gained the encode-row additions (`entity_tokens`, `print_entity`, the saved-record pair), "Printing" and "Logging" tables pairing each Go function with its Python form, and "Not applicable" rows for `highlight.Tokenize` and `Kind.Scope()`.

**Deviations**: The `highlight=` example and parity cells had already been removed in the first task. The new sections sit before "Asyncio" as top-level sections rather than directly after the encoding subsection, following the README's heading levels.

**Files changed**:
- `xcl-python: README.md`
- `xcl-python: PARITY.md`

**Discoveries**: None.

### 2026-10-26 — Task: Document Python printing on the website

**What was done**: The Python tabs of xcl.dev's configuration-text, problems, diff and events pages and of the plugins and configuration-only examples now show Python printing: `cfg.print_entity`/`print_saved_entity`, the five formats and `for_stream`, themes with `DEFAULT_THEME | {...}` and `validate`, `load_vscode_theme`, `css`, HTML in a template, a custom format class with `style_for`, `entity_tokens`; `xcl.print_error`/`format_error` with `width=`, colour and themes, reports on uncaught errors and `logging.exception` through exception notes, `LogFormatter`; `xcl.print_diff` with colour, themes, HTML and `diff.tokens`; `xcl.logging_handler` in place of the hand-written logging handler; the `run`, `plan` and `inspect` example listings printing through the package. Every "not yet available from Python"/"is coming" statement about printing and every `highlight=` mention is gone. Edited by three sub-agents in parallel, one per page group, as the plan suggested.

**Deviations**: problems.mdx gained one subsection, "In a traceback", inside the existing page (no page added, renamed or removed; navigation unchanged). The configuration-only example now shows its Go program's `main`, which prints errors with `xcl.PrintError`, beside a Python `main` using `xcl.print_error`. The HTML-in-a-template Python examples use Jinja's `markupsafe.Markup` as the counterpart of Go's `template.HTML`. The events page's per-problem receiver tells a single problem's error event from the final one by `event.error.tokens and not event.error.causes`, checked against the engine.

**Files changed**:
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/pages/problems.mdx`
- `xcl-website: src/pages/diff.mdx`
- `xcl-website: src/pages/events.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`
- `xcl-website: src/pages/examples/configuration-only.mdx`

**Discoveries**: Python tracebacks name the error class by its private module path (`xcl._errors.ConfigError`), which the website's sample outputs show as Python prints it. The site builds from a worktree after `npm ci` (23 pages).
