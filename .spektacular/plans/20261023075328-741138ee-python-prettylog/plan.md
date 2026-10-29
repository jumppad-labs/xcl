---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Plan: 20261023075328-741138ee-python-prettylog

<!-- Metadata -->
<!-- Created: 2026-10-23T09:30:36Z -->
<!-- Commit: xcl ae3b7b6, xcl-python 1aee25c, xcl-website f82b715 -->
<!-- Branch: f-python-prettylogger (xcl), main (xcl-python), main (xcl-website) -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git, jumppad-labs/xcl-python, jumppad-labs/xcl-website -->

## Overview

Python programs that use xcl get a ready-made pretty logger, `xcl.prettylog`, modelled on the one Go's examples use and set up in one line on either client. Each event appears as a readable, coloured line. Each created resource's configuration, including values the provider filled in, appears beneath its creation line as it happens, and a configuration mistake appears with its report and the faulty source lines. Until now a Python program had to write its own event handling and still could not show what each resource became.

The logger takes its colours, formats and reports from the package's printing support. Asyncio handlers now receive events in order, as Go's and the synchronous client's do. A new parity stage holds the Python output to Go's real example on every shared fixture. Python application authors, and anyone watching a Python program apply configuration, benefit.

## Conventions

- **Testing & Mocking: never use table-driven tests** — each level read from `XCL_LOG_LEVEL`, each event shape, each colour branch and each parity fixture gets its own test function; no `pytest.mark.parametrize` tables in xcl-python, no table-driven Go test in the parity program.
- **Testing & Mocking: never mix positive and negative tests; favour verbosity over abstraction** — "configuration shown" and "nothing shown" (above info, no data, builtins), a report above info,, colour on a terminal and plain elsewhere, and accepted and refused arguments are separate test functions, each spelling out its events and the expected lines.
- **Testing & Mocking: locate tests with the source they test; never write tests that inspect repository files** — unit tests in `xcl-python:tests/unit/test_prettylog.py`, engine-backed tests in `tests/integration/`, the Go parity additions in `bindings/libxcl/cmd/parity/main_test.go`; README, PARITY.md and the website are checked by review only.
- **Testing & Mocking: use testify `require`** — applies to the Go parity program's new tests; the Python tests keep the suite's plain `assert`.
- **Dependencies: prefer the standard library; document reasoning for third-party dependencies; pin versions** — the pretty logger uses only the standard library and the package's own printing support; the parity program's new import of `example/prettylog` brings charmbracelet/log v0.4.2, lipgloss v1.1.0 and termenv v0.16.0 into `bindings/libxcl/go.mod`, pinned at the versions `example/prettylog/go.mod` already pins, with a comment recording why.
- **Development standards: structured logs** — when routed through a logger, each record carries the event's fields as structured attributes under Go's names (the logging bridge's extras) alongside the pretty text.
- **Go code style (gofmt/vet, descriptive names, explicit error handling)** — applies to the parity program's new stage and its normaliser.

Deliberately dropped: shared errors package (no new Go error type), shared test helpers in `internal/testutil` (the only Go change is inside the parity program, which uses no shared helper), project structure (no new Go package), config parser options builder, hclsyntax references, graph-parent ordering, real-apply test state (no state fixture is written), provider Changed/Update testing, never-modify-dependencies (no dependency is edited), database, and patterns & architecture (no services).

## Architecture & Design Decisions

The pretty logger is a new public module of the Python package, `xcl.prettylog` (`xcl-python:xcl/prettylog.py`), a port of Go's `example/prettylog` built on the package's printing support (plan `20261023075302-bad71237-printing-and-highlighting-python`) and its logging bridge. It offers `handler(file=None, level=logging.INFO, *, logger=None, formatter=None)`, which returns an ordinary `on_event` callable, `level_from_env()`, which reads `XCL_LOG_LEVEL`, and the constant `LEVEL_ENV`. Setup is one line on either client: `xcl.Config(on_event=xcl.prettylog.handler(sys.stderr), event_data=xcl.EventDataLevel.PROCESSED)`, and the same with `xcl.aio.Config`. The handler applies Go's logging bridge rules for level and message (shared with `xcl.logging_handler`, not copied) and drops events below its level. It then writes one line in the layout Go's example writes through charmbracelet/log: Kitchen time, a four-letter level, the message, then `source`, `operation`, `phase`, `resource`, `type`, `file`, `duration`, `error` and the sorted details as `key=value`, with charmbracelet's quoting and multi-line value rules. At info and below, beneath a `create success` line it writes the created resource's configuration, with computed values, indented two spaces. Beneath an error whose `xcl.Error` carries a report it writes that report, indented the same way. Everything lands in xcl-python except one Go change: a parity stage in xcl's `bindings/libxcl/cmd/parity` that runs Go's real example for the success metric. Documentation lands in the xcl-python README and PARITY.md and in xcl-website's events page.

Colour and text come from the printing support, never from copies (spec constraint; design `printing-and-highlighting.md`, source `design`). Each styled piece of a line is one `Kind.TEXT` token, formatted with a one-entry theme `{Kind.TEXT: "<style>"}` by the chosen formatter. The style strings are Go's choices written in the design's syntax: `bold magenta` for the source value, `bold cyan` for the resource, `bold yellow` for the operation, plain `magenta`, `cyan` and `yellow` for their keys, `dim` for other keys and `=`, and bold blue, cyan, yellow and red for the four levels. The formatter is `highlight.for_stream(file)`, so a terminal with `NO_COLOR` unset gets `TERMINAL` and anything else gets `PLAIN`. With a logger it is `PLAIN`, unless the caller passes `formatter=`. Configuration is the `saved_entity_tokens(event.data, include_computed=True)` of the configuration that delivered the event, formatted with `DEFAULT_THEME`. Reports are `xcl.format_error(err, formatter=…, theme=DEFAULT_THEME, width=80)`. A report is shown once per problem. A `parse` or resource error event shows its own report, and the operation's closing error event shows its report only when it carries more problems than have already been shown in that operation, so an unsupported block is not reported twice. A `NotEncodableError` (variables, outputs, modules) is skipped silently. Any other failure to render becomes one `WARN unable to show configuration resource=… error=…` line, as in Go, and the handler never raises into the operation; write errors on the stream are swallowed, as Go's bridge discards them. When a `create success` carries no data and the configuration that delivered it was created with `EventDataLevel.NONE`, the handler writes once, for its lifetime, an info line saying to create the configuration with `event_data=xcl.EventDataLevel.PROCESSED`. Above info it writes event lines and configuration-mistake reports only, with no configuration, render warnings or guidance. This differs from Go's example, which writes no reports at any level and returns its bare bridge above info (confirmed by the user; see below). A line and the block beneath it are written in one locked write. With `logger=` they travel as one `LogRecord` whose message is that same text, at the event's level and time, carrying the bridge's structured fields, so a `%(message)s` handler prints exactly what the stream would.

Two small changes to the package's internals make this work on both clients without waiting inside the handler (spec constraint; spec `20261023074627-cc23fc08-encode-saved-entity-during-operation`). First, an `Event` keeps a private reference to the configuration that delivered it, set where `_classes` is set today (`xcl/_poll.py` `_dispatch`, `xcl/aio.py` `_handle_event`). It takes no part in fields, equality or repr, so the handler needs nothing from the application, as Go's event carries its decoder. Both configuration flavours gain a private blocking `_saved_entity_tokens_now`. It sends `EncodeSavedEntity` straight through the client from whatever thread the handler runs on, never through the asyncio client's single-worker poll executor, so a render never queues behind a waiting `Poll`. The transport matches responses by id, so this is safe. Second, `xcl.aio.Config` delivers events to `on_event` one at a time and in order, through one consumer per operation, instead of one concurrent task per event. Plain handlers still run on the default executor and `async def` handlers on the loop, and state and function tasks stay concurrent. Without this, the asynchronous run cannot give the same lines in the same order. It also fixes `logging_handler` ordering under asyncio, and it matches how Go and the synchronous client deliver events.

**Decisions the user confirmed during epic planning.** Two readings of the spec were put to the user and confirmed. They are recorded decisions, not drafting assumptions. (1) *The render inside the handler is allowed.* The constraint "must not wait on the operation, or on anything else slower than writing to its stream" forbids waiting on the operation, which deadlocks the loop driving it. The one `EncodeSavedEntity` render per created resource, made inside the handler, is allowed. (2) *Reports show at every level.* "Quiet at higher levels … event lines only, with no configuration" means no resource configuration blocks. A configuration mistake's report is still written beneath its error line at every level, with the same once-per-problem rule, so the unconditional "Configuration mistakes shown with their source" requirement holds. Above info the handler writes event lines and reports only: no configuration, no render warnings and no guidance. This differs from Go's example, which returns its bare logging bridge above info and never writes reports.

This direction beats a pretty `logging.Formatter` over the bridge's records (users would have to configure their handler before a line matched, and the configuration block has no natural home on a record), an `async def` handler serialised by a lock (two factories, and awaiting the asyncio lookup waits on the operation's `Poll`), and hand-written escape codes (forbidden by the spec's constraint). The success metric is proven against Go's real output. A `pretty_log` stage in xcl's parity program applies each fixture whose apply succeeds through `example/prettylog.Handler`, and in `xcl-python:tests/parity/run_python.py` through `xcl.prettylog.handler`. Both use processed event data and a plain buffer at info. Timestamps and durations are replaced, colour is stripped, each event line is grouped with the indented block beneath it, and the groups are sorted, so concurrent resources cannot reorder them. The existing comparison then fails at the first difference. Importing the example adds charmbracelet/log, lipgloss and termenv to `bindings/libxcl/go.mod`, a module nothing imports as a library, and that reasoning is recorded there (`conventions/dependencies.md`). The Python package gains no dependency. Tests follow `conventions/testing-and-mocking.md`: one behaviour per test, no parametrised tables, positive and negative cases apart, and no test reading the README, PARITY.md or the website. See `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Pretty logger (xcl-python, new public module `xcl.prettylog`)**: owns the handler factory, the level read from `XCL_LOG_LEVEL`, and everything a pretty run writes. That covers the level filter, the event line in Go's example layout, the configuration beneath a creation line, the report beneath a configuration mistake, the render warning and the once-per-handler guidance. It owns the choice of style for each part of a line, not how a style becomes colour. It asks the formatters to render every styled piece, the delivering configuration for each created resource's labelled text, and the error-printing functions for reports. Its output goes to the stream given or, as records, to a logger given. It is an ordinary `on_event` handler, so both configuration flavours accept it unchanged.
- **Line layout (xcl-python, new, private to the pretty logger)**: turns one event, its level and its message into the text Go's example writes through charmbracelet/log. That means the Kitchen-format time, the four-letter level, the message, the attributes in Go's order, and Go's formatting of values: numbers, booleans, missing values, lists, maps and durations. It also applies charmbracelet's quoting, escaping and multi-line value layout. It holds the parity contract with Go's lines, so it is kept apart from the handler's decisions and tested on its own.
- **Logging bridge (xcl-python, changed slightly; added by the printing plan)**: still owns Go's rules for an event's level, message and record fields. Those rules become shared private helpers that the pretty logger calls, rather than copies, so the two cannot drift. The bridge's own behaviour is unchanged.
- **Printing support (xcl-python, unchanged here; added by the printing plan)**: the formatters, default theme, colour chooser, error report printing and a configuration's labelled text for a saved record. The pretty logger consumes them as they are and adds nothing to them.
- **Events (xcl-python, changed)**: an event delivered by a configuration also remembers that configuration privately, the way it already remembers the configuration's classes, so a handler can have the saved record rendered without the application passing the configuration in. An event built by hand has none. The reference takes no part in an event's fields, equality or representation.
- **Configurations, synchronous and asyncio (xcl-python, changed)**: both remember the event data level they were created with, and both offer a private render of a saved record's labelled text that blocks on its own thread and goes straight to the engine. The asyncio flavour's render bypasses its single-worker poll executor, so a render from a handler never queues behind a waiting poll. Both attach themselves to every event they deliver.
- **Asyncio event delivery (xcl-python, changed)**: the asyncio configuration delivers events to its handler one at a time, in the order the engine sent them, through one consumer per operation. It still runs plain handlers on the default executor and coroutine handlers on the loop, keeps state and function tasks concurrent, drains the consumer before the operation returns, and cancels it with the rest when the operation is cancelled. A handler's exception is still dropped, as today.
- **Package exports (xcl-python, changed)**: `xcl.prettylog` is importable as an attribute of `xcl`, as `xcl.mask` is.
- **Go parity program (xcl, changed)**: gains a stage that applies each fixture whose apply succeeds through Go's example pretty logger at info, with processed event data, into a plain buffer. It records the normalised event groups. Its module records the example and its terminal-styling libraries as dependencies, with the reason.
- **Python parity program and comparison (xcl-python, changed)**: the Python parity program records the same stage through `xcl.prettylog`, normalised the same way. The existing comparison then holds Python's pretty output to Go's on every shared fixture.
- **Test suite (xcl-python, changed)**: unit tests cover the line layout, level parsing, every handler decision and asyncio ordering against the fake transport. Integration tests cover each acceptance criterion against the real engine and the recording fixture plugin, on both flavours, including a pseudo-terminal where the platform has one.
- **Package documentation (xcl-python, changed)**: the README gains a pretty-logging section with the one-line setup for both flavours, the logger form, the environment variable and a sample of the output. It notes that asyncio now delivers events in order. The parity list pairs Go's example with `xcl.prettylog`.
- **Website events page (xcl-website, changed)**: the pretty-receiver section's Python tabs show `xcl.prettylog` instead of a hand-written receiver, with a sample of its output. The page is edited in place.

## Data Structures & Interfaces

**`xcl.prettylog` (new public module)**: the handler factory, the environment helper and the variable's name, after Go's `prettylog.Handler`, `LevelFromEnv` and `LevelEnv`. Levels are the standard library's `logging` integers.

```python
LEVEL_ENV: Final[str] = "XCL_LOG_LEVEL"

def level_from_env() -> int: ...
    # logging.DEBUG / INFO / WARNING / ERROR for "debug" / "info" / "warn" / "error" in any case; INFO otherwise

def handler(
    file: TextIO | None = None,           # None: sys.stderr as it is when handler() is called
    level: int = logging.INFO,            # a logging level int; anything else raises TypeError
    *,
    logger: logging.Logger | None = None, # route through this logger instead of a stream; with file -> ValueError
    formatter: highlight.Formatter | None = None,  # None: highlight.for_stream(file), or PLAIN with a logger
) -> Callable[[xcl.Event], None]: ...
```

The returned handler is an ordinary `on_event` callable for `xcl.Config` and `xcl.aio.Config`. It returns nothing and never raises into the operation. One handler may serve several configurations. Its once-only guidance and its per-operation report count belong to the handler.

**The text contract (what a run writes)**: every event at or above the level is one line in charmbracelet/log's text layout, with Go's `SlogHandler` attributes:

```text
3:04PM INFO create success source=core operation=create phase=success resource=resource.recording.one type=recording.one file=/work/main.xcl duration=1.2ms
  resource "recording" "one" {
    log    = "/work/log.jsonl"
    output = "one-a"
    value  = "a"
  }
3:04PM ERRO parse error source=core operation=parse phase=error file=/work/main.xcl error="…"
  Error: Unsupported block type
  …
3:04PM WARN unable to show configuration resource=resource.cache.main error="…"
3:04PM INFO configuration not shown: events carry no resource data hint="create the configuration with event_data=xcl.EventDataLevel.PROCESSED"
```

- The time is the event's time in local time, in Go's `time.Kitchen` form. The level is `DEBU`, `INFO`, `WARN` or `ERRO`. The message is the logging bridge's message.
- Attributes are `source`, `operation`, `phase`, `resource`, `type` and `file` when set, `duration` when non-zero, `error` when set, then the details in sorted key order (a log event's `level` and `message` excluded). Each value is written as Go writes it: quoted and escaped when it holds a quote, `=`, a space or a non-printable character; `""` when empty; and in charmbracelet's indented `│` form when it spans lines.
- A block (configuration or report) is the formatter's text with every non-empty line indented two spaces, written directly after its line.
- With a terminal formatter, each styled piece is wrapped in its style's codes by the formatter. Styles: the four levels `bold blue`, `bold cyan`, `bold yellow`, `bold red`; keys `dim`, except `source` (`magenta`), `resource` (`cyan`) and `operation` (`yellow`); `=` and the `│` gutter `dim`; values plain, except `source` (`bold magenta`), `resource` (`bold cyan`) and `operation` (`bold yellow`).

**Logger form (contract with the application's handlers)**: one `LogRecord` per written group, on the given logger.

- `levelno` is the group's level: the event's level for an event line and its block, `WARNING` for a render warning, `INFO` for the guidance.
- `msg` is exactly the text the stream form writes, without the final newline, and `args` is `()`.
- `created` is the event's time.
- Its attributes are the logging bridge's extras (`source`, `operation`, `phase`, `resource`, `type`, `file`, `duration`, `error`, details, `xcl_event`).
- The logger's own level and filters apply after the handler's level.

**Private additions (package-internal contracts)**:

```python
class Event:                      # xcl.events
    _config = None                # the Config or aio.Config that delivered it; not a field, not compared, not in repr

class Config:                     # xcl.config, and likewise xcl.aio.Config
    _event_data: EventDataLevel   # the level it was created with
    def _saved_entity_tokens_now(self, data: bytes, *, include_computed=False, include_empty=False,
                                 reveal_sensitive=False, show_references=False) -> list[Token]: ...
        # blocking, on the calling thread, straight through the client; safe from an on_event handler

# xcl._logbridge (from the printing plan) exposes the rules it already applies, unchanged:
def event_level(event: Event) -> int: ...
def event_message(event: Event) -> str: ...
def event_extra(event: Event) -> dict[str, Any]: ...
def event_record(logger: logging.Logger, event: Event, level: int, message: str) -> logging.LogRecord: ...
```

**Asyncio delivery contract (changed behaviour, no new type)**: `xcl.aio.Config` calls `on_event` once per event, in the order the engine sent them. It never starts one call before the previous one returns, or before an `async def` handler's coroutine finishes. The operation returns only after every event has been handled. A handler's exception is dropped, as before.

**Parity record (Go and Python programs, compared key by key)**: a new key `pretty_log`.

- For a fixture whose apply succeeds, it holds the list of groups the pretty logger wrote for a fresh apply at info, with processed event data, into a plain buffer.
- Each group is an event line with the lines beneath it, joined by `\n`, with ANSI codes removed, the leading time replaced by `<time>`, any `duration=<value>` replaced by `duration=<duration>`, and run paths replaced as the existing keys replace them. The groups are sorted.
- For any other fixture, it is `null`.

## Implementation Detail

**A port with a contract, split into a pure layout and a thin handler.** The pretty logger is two layers. Below is a pure line layout. It takes an event, a level, a message and a function that styles a piece of text, and returns the line, with no I/O and no engine. It is a line-for-line port of charmbracelet/log's text formatter and Go's value formatting, so a reader can hold the Go and Python side by side, and it is tested with literal events and expected lines. Above is the handler. It decides whether to write, what goes beneath a line, and where the text goes. Each decision mirrors a branch of Go's example, plus the three behaviours the spec adds: reports beneath configuration mistakes, the once-only guidance and logger routing. This follows how the printing plan ports Go's formatters, and keeps the parity-sensitive code separate from the policy code.

**Styling goes through the printing support's formatter seam, one piece at a time.** The layout never writes escape codes. It asks for each piece to be styled, and the handler answers by formatting a single plain-text token with a one-entry theme through the chosen formatter. So the terminal-or-plain decision is the formatter's, made once from the stream, and the codes are exactly what the printing support writes for configuration and reports. Configuration blocks and reports are formatted in one call each with the default theme, then indented. The terminal formatter resets before every newline, so indenting coloured text is safe, as Go's example relies on.

**Shared rules, not copies.** The logging bridge's level, message and record-building rules are lifted into small private helpers in the bridge's module. The bridge keeps its behaviour, and the pretty logger calls the same helpers. In logger form the pretty logger builds its record the way the bridge does, with the pretty text as the message, so routing, filtering and structured fields behave as they do for the bridge.

**The event knows who delivered it.** The package already attaches the delivering configuration's classes to every event it builds, privately and outside equality. This plan attaches the configuration itself in the same two places, the synchronous dispatch and the asyncio event handling. A handler can then reach the engine for that configuration without a closure over a variable that does not exist yet when the handler is created. Both configuration flavours gain one private blocking render that goes straight to the client. The synchronous flavour's public token method can delegate to it. The asyncio flavour keeps its awaitable public methods, and uses the blocking one only from handler threads.

**Asyncio events become a queue with one consumer.** Today each event becomes its own asyncio task, so plain handlers run concurrently on the default executor and can finish in any order. The driver now puts each event on a per-operation queue that one consumer task drains in order. It awaits each handler call, an executor call for a plain handler or the coroutine for an `async def` one, before taking the next. State and function tasks keep their own concurrent tasks. At the end the driver closes the queue and waits for the consumer. On cancellation the consumer is cancelled with the pending tasks, as those are today. This is the one change of behaviour outside the new module, and it brings asyncio in line with the synchronous client and Go.

**Thread safety and never failing the operation.** The handler holds a lock around each write, the guidance flag and the report count, so one handler shared by several configurations or threads never interleaves a line with another's block. Rendering happens before the lock is taken, because the lock guards writing only. A render failure becomes a warning group. A write failure on a stream is swallowed. With a logger, the standard `handleError` applies.

**Parity follows the existing harness.** The Go and Python parity programs each gain one stage computed the same way, normalised by the same rules, under one record key, so the existing comparison test reports any difference by its JSON path. This is the same pattern the printing plan used for its printing and logging stages. The Go stage imports the example package rather than reimplementing it, so the comparison is against what Go users actually see.

**What stays the same.** `xcl.logging_handler`, the printing functions, the formatters, `Event`'s public fields and every configuration method's signature are unchanged. The synchronous driver changes only to attach the configuration to events. Go's example pretty logger is not modified (spec Non-Goal).

## Dependencies

- **Design documents this plan was built on:** `printing-and-highlighting.md` from the `design` design source. It settles that colours, formats and reports come from tokens, themes as style strings and formatters, that colour is chosen by `for_stream` (terminal with `NO_COLOR` unset, else plain), and the shape of the Python print API, `logging_handler` and the default theme. The pretty logger consumes all of these without changing them.
- **Plan `20261023075302-bad71237-printing-and-highlighting-python` (must land first).** It provides `xcl.highlight` (`Kind`, `Token`, `DEFAULT_THEME`, `PLAIN`, `TERMINAL`, `Formatter`, `for_stream`), `xcl.format_error`, `err.tokens`, `Config.saved_entity_tokens` on both flavours and `xcl.logging_handler`. This plan lifts the bridge's level, message and record rules into shared private helpers without changing its behaviour. It also adds the parity `log` and `printed_entities` stages and the `unsupported_block` fixture, which this plan's parity stage sits beside. This plan does not depend on its open question about whether a multi-problem `str(err)` is one line or several: either is written with charmbracelet's multi-line rule, as Go writes it.
- **Plan `20261023074627-cc23fc08-encode-saved-entity-during-operation` (must be in the bundled engine first).** The engine renders a saved record during an operation, so the handler can render each created resource from its `create success` event while the apply runs. No package code from it is needed.
- **Plan `20261023074223-e06f835f-event-data-level-enum` (must land first).** It provides `xcl.EventDataLevel`, which the guidance line names and the tests and docs pass as `event_data`, and `py.typed`, which makes this module's annotations part of users' type checking. It also edits `xcl/config.py`, `xcl/aio.py`, the README and PARITY.md; whichever lands second rebases.
- **Plan `20261023074926-eaa51527-printing-and-highlighting-core` (xcl, must land first).** It switches Go's `example/prettylog` to `xcl.PrintEntity` and keeps its lines and colours. The Go parity stage runs that example as it is. Nothing in it needs to change.
- **Go's `example/prettylog` and `events.SlogHandler` (xcl, reference and parity input, unchanged).** They are the look and layout Python reproduces, and the Go side of the parity comparison. Changing the example is a spec Non-Goal.
- **charmbracelet/log v0.4.2, lipgloss v1.1.0, termenv v0.16.0 (Go, already pinned by `example/prettylog`).** Their text layout is the reference for the line format. They become dependencies of the `bindings/libxcl` module only because the parity program imports the example. They are pinned at the example's versions and are not part of the shared library's build.
- **xcl's parity program (`bindings/libxcl/cmd/parity`, changed by this plan).** Its new stage must be in the xcl release that xcl-python pins, because xcl-python's CI builds the parity program from that checkout. If the printing release has already been cut when this lands, a further xcl release is needed before xcl-python ships this plan.
- **An xcl release containing the plans above and this plan's parity stage (must exist before xcl-python ships).** xcl-python bundles only an engine built from a released xcl tag pinned in `xcl-ref.txt`. During development the pin points at the xcl branch; before merge it names the release.
- **xcl-python's test fixtures: the recording plugin (`xcl-plugin-recording`, built from xcl's `bindings/libxcl/testdata`) and the fake transport.** The plugin provides a resource whose `output` the provider computes on create, used for the "value the provider filled in" criteria. The fake transport drives the unit tests. Unchanged.
- **Python standard library (3.11+).** It provides `logging`, `os`, `sys`, `io`, `threading`, `asyncio`, `datetime` and `decimal` (Go-style float formatting), plus `pty` in tests (POSIX only, so those tests skip on Windows). No runtime or test dependency is added, as the spec requires.
- **xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).** Its events page is edited in place after the core and Python printing plans' edits to the same page. The website change goes out after the xcl-python release it documents.
- **xcl-python release flow (existing).** The PR is labelled `minor`, then auto-tag, the release workflow and the automated engine-bump PR run. The change is additive apart from asyncio's ordered event delivery, which the README notes and a person adds to the GitHub release notes by hand. No workflow change.

## Testing Approach

Testing follows the xcl-python suite's layout and the project's testing conventions. Each test function covers one behaviour, with no `pytest.mark.parametrize` tables and positive and negative cases in separate functions. Unit tests run against hand-built events and the fake transport, integration tests against the real engine with the recording fixture plugin, and the Go parity additions use testify `require` beside the parity program. No test reads the README, PARITY.md or the website.

**Unit tests: the line layout (the parity-sensitive core, most of the coverage).** Literal events in, literal lines out, one rule per test:
- the Kitchen time from a fixed UTC event time, converted to local time, including midnight and noon;
- each of `DEBU`, `INFO`, `WARN` and `ERRO`;
- an empty message left out;
- the attribute order, with each optional attribute absent when unset;
- details sorted, and a log event's `level` and `message` left out;
- value formatting for strings, booleans, integers, integral and fractional floats, exponent floats, `None`, lists, nested maps and durations (each format as Go prints it);
- quoting of a value holding a space, `=` or a quote, and escaping of control characters;
- `""` for an empty value;
- a multi-line value in charmbracelet's indented form, both last and followed by further attributes.

With a recording styler, each piece is styled with the style the design gives it, the `source`, `operation` and `resource` keys and values with theirs. The expected lines are Go's outputs for the same inputs, produced once by running charmbracelet/log and pasted with a comment naming its version, as the printing plan does for word wrapping.

**Unit tests: handler decisions (fake transport and hand-built events).**
- Level filtering at each level.
- `level_from_env` for "debug", "warn", "error", "INFO", unset and "loud", one test each.
- Refusal of a non-integer level, a boolean level, and `file` and `logger` together, one test each.
- The configuration block after a `create success` delivered by a configuration, indented, and nothing for other events.
- Nothing for a hand-built event without data.
- One warning for a hand-built event with data and no configuration, with later events still written.
- Silence for `NotEncodableError`.
- One warning per render failure, with the run completing.
- The guidance written once, and only when the delivering configuration has no event data.
- Above info: no configuration, render warning or guidance beneath any line, while a configuration mistake's report still appears beneath its error line, in a separate test.
- The report beneath a `parse error`, not repeated beneath the closing `apply error`, and shown on the closing event when nothing earlier showed it.
- Records in logger form: the text, the level, the time and the structured fields.
- A failing stream not raising.

The plumbing gets its own tests:
- an event delivered by each flavour carries its configuration, and an event's equality and repr ignore it;
- each flavour's private render sends a token-asking saved-record request on the calling thread;
- the asyncio configuration delivers several events to a slow plain handler, and to an `async def` handler that awaits, strictly in order and one at a time;
- an asyncio operation returns only after its last event is handled.

**Integration tests (real engine, both flavours where the call differs): the acceptance criteria.** One configuration creates one `recording` resource, whose `output` the provider fills in.
- Applying it with `handler(buf)` and no level writes one line per event at info or above, each starting with a time and a level and carrying the message, source, operation, phase and resource ("Setup and event lines").
- In the same run, the indented configuration directly after the creation line holds `output`, before the next event's line ("Configuration beneath creation").
- The asyncio run gives the same lines once timestamps and durations are replaced ("Asynchronous client").
- A run through an application logger whose handler formats `%(message)s` writes the same lines ("Through an existing logger").
- At warn, the run writes no configuration and no info lines ("Warn level is terse").
- At warn, applying a configuration with an unsupported block still writes the report beneath the error line (the user's confirmed reading of "Quiet at higher levels").
- At debug, it adds the core's debug events ("Debug level").
- A configuration with an unsupported block writes the error line followed by a report with the summary, the file and line, and the numbered source lines ("Configuration mistake").
- Writing to a pseudo-terminal gives colour codes, with the source, operation and resource values in their styles' codes. A file gives none, and a pseudo-terminal with `NO_COLOR` set gives none. These are three tests, and the terminal ones skip where the platform has no `pty` ("Colour on a terminal only").
- Two resources with event data off give event lines, exactly one guidance line and no configuration ("Guidance without resource data").
- A configuration with a variable, an output and a module gives no warning and no configuration for them ("Unrenderable items", first half). The second half, two resources whose rendering fails giving one warning each while the run completes, is covered by the fake-transport unit test, because a real engine renders every resource it created.

**Contract test: parity with Go (the success metric).** Both parity programs gain a `pretty_log` stage that applies each fixture whose apply succeeds through the pretty logger. Go runs its real `example/prettylog` and Python runs `xcl.prettylog`, both at info, with processed event data, into a plain buffer. Each side normalises the same way: timestamps and durations replaced, ANSI codes stripped, run paths replaced, and each line grouped with the block beneath it, the groups sorted. The existing comparison fails at the first differing group. The Go program's own tests check that the stage is present, normalised, deterministic across runs and `null` for a failing fixture.

**Success metrics.**
- "The same apply run through Go's example and through the Python pretty logger gives the same event lines and configuration, apart from timestamps and the event-attribute order": **Behavioural test**. This is the `pretty_log` parity stage above, run on every shared fixture whose apply succeeds. Python writes attributes in Go's order, so the comparison does not reorder them.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: the README shows the one-line setup for both flavours, the logger form, `XCL_LOG_LEVEL`, the event data the configuration needs and a sample of the output, and notes that asyncio delivers events in order ("Docs").
- **Manual — captured in the implementation test plan**: the website's events page shows `xcl.prettylog`'s one-line setup and a sample of its output in its Python tabs, no hand-written Python receiver remains, existing URLs still work, and the site builds ("Docs").
- **Manual — captured in the implementation test plan (descoped release, done by a person after the merge)**: the xcl release xcl-python pins carries the `pretty_log` parity stage, the printing work and saved-record rendering during an operation; xcl-python pins it, CI passes and the next minor version is published with the `minor` label; the published GitHub release notes carry a line saying the asyncio client now delivers events to `on_event` one at a time and in order, so a handler that relied on running concurrently may slow an operation; the website change goes out after the release.
- **Manual — captured in the implementation test plan**: in a real terminal, a pretty run of the README's example looks right, with coloured levels, the source, operation and resource highlighted, and the configuration and a report coloured by the default theme. With `NO_COLOR` set, or redirected to a file, it is plain.

**Deliberate gaps.** There are no tests of the formatters, the report layout or saved-record rendering themselves. Those belong to the printing and engine plans and arrive here through their public functions. The line layout is compared with Go through pasted expected strings and through the live parity stage, not by running charmbracelet/log from Python tests. Error fixtures are left out of the parity stage, because Go's example writes no report and no guidance. The stage runs at info on fixtures whose apply succeeds, where no error event and no guidance occur. Reports shown at every level are therefore a Python-only difference that the comparison never sees, and it is tested by the Python unit and integration tests instead.

## Milestones & Tasks

### Milestone 1: Asyncio handlers receive events in order, and a handler can have any event's resource rendered

**What changes**: A program using the asyncio client now receives its events one at a time and in the order xcl sent them, as the synchronous client and Go already deliver them. A slow or awaiting handler no longer sees later events first. Logging through the standard-logging bridge under asyncio therefore writes records in order. Behind that, every event remembers, privately, the configuration that delivered it. Each configuration can render a saved record from inside a handler without waiting on the running operation, and the bridge's level and message rules become shareable. None of this adds public API. It is the groundwork the pretty logger needs to work identically on both clients, and it is worth landing on its own because the ordering fix stands by itself.

**Validation point**: The unit suite passes. Asyncio events reach a slow plain handler and an awaiting `async def` handler strictly in order. Events from both flavours carry their configuration without it affecting equality. Each flavour's private render sends a labelled-text saved-record request from the calling thread. The bridge's existing tests pass unchanged. The integration suites pass against the engine built from the xcl branch.

#### - [x] Task: Deliver asyncio events in order
**Id:** 319db6ce-633f-4b6d-a454-4c292d0493e8
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Change the asyncio configuration so its event handler is called once per event, in the order the engine sent them, never starting one call before the previous one has finished. Today every event becomes its own concurrent task, so plain handlers can run side by side and finish in any order. State-store and function tasks stay concurrent, the operation still returns only after every event has been handled, cancelling still stops everything, and a handler's exception is still dropped as before.

*Technical detail:* [context.md#task-deliver-asyncio-events-in-order](./context.md#task-deliver-asyncio-events-in-order)

**Acceptance criteria**:
- [x] A plain handler that is slow for the first event still receives every event of an asyncio operation in the order they were sent, one at a time
- [x] An `async def` handler that awaits between events receives them in order, one at a time
- [x] An asyncio operation returns only after its last event has been handled
- [x] Cancelling an asyncio operation while events are being handled still stops it and lets the cancellation through
- [x] A handler that raises does not stop later events or fail the operation

#### - [x] Task: Let a handler reach the configuration that delivered its event
**Id:** 82bca570-5792-419f-95dc-7f49a6a3882b
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Make every event delivered by a configuration remember that configuration privately, as it already remembers the configuration's classes, so a handler can have the event's saved record rendered without the application passing the configuration in. Give both configuration flavours a private, blocking render of a saved record's labelled text that goes straight to the engine from whatever thread calls it, and have them remember the event data level they were created with. Lift the logging bridge's level, message and record rules into shared private helpers, leaving its behaviour unchanged.

*Technical detail:* [context.md#task-let-a-handler-reach-the-configuration-that-delivered-its-event](./context.md#task-let-a-handler-reach-the-configuration-that-delivered-its-event)

**Acceptance criteria**:
- [x] An event delivered by a synchronous or an asyncio configuration refers to that configuration, and an event built by hand refers to none
- [x] Two events with the same fields are equal and print the same whichever configuration delivered them
- [x] Each configuration flavour can render a saved record's labelled text with a blocking call from any thread, asking the engine for labelled text with the chosen encode choices
- [x] Each configuration reports the event data level it was created with
- [x] The logging bridge writes exactly the records it wrote before

### Milestone 2: One line turns on pretty logging, with created resources and configuration mistakes shown as they happen

**What changes**: A Python program turns on a readable, coloured log of everything xcl does with one line, `on_event=xcl.prettylog.handler(sys.stderr)`, on either client, at the level it chooses or the level named by `XCL_LOG_LEVEL`. Each event is one line in the layout Go's examples show, with the time, level and message, and the source, operation, phase, resource and details as key=value, the source, operation and resource highlighted. Each created resource's configuration, including values the provider filled in, appears indented beneath its creation line as it happens. A configuration mistake shows its report, with the faulty source lines, beneath the error line. Colour appears only on a terminal without `NO_COLOR`. A run that asks only for warnings stays terse, writing event lines and configuration-mistake reports only. A program that already uses standard logging can send the same lines through its own logger instead. A program that forgot to turn on resource data is told once how to.

**Validation point**: The unit tests of the line layout and every handler decision pass. The integration tests for each acceptance criterion pass on both clients against the real engine: setup and event lines, configuration beneath creation, the asyncio run, the logger route, warn and debug levels, the configuration mistake, colour on a pseudo-terminal only, the guidance, and silent builtins.

#### - [x] Task: Lay out event lines as Go's example writes them
**Id:** 4d9d42b7-48c0-4d0f-822d-8bb39ae3638e
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Add the private layout that turns an event, its level and its message into the exact line Go's example writes through charmbracelet/log: the Kitchen-format time, the four-letter level, the message, then the attributes in Go's order with Go's value formatting, quoting and multi-line layout. Each piece is styled through a function the caller supplies, so the layout itself writes no escape codes. It is tested against lines produced by Go for the same inputs.

*Technical detail:* [context.md#task-lay-out-event-lines-as-gos-example-writes-them](./context.md#task-lay-out-event-lines-as-gos-example-writes-them)

**Acceptance criteria**:
- [x] For the same event, level and message, the plain line equals the line Go's example writes, apart from the time zone of the timestamp
- [x] Attributes appear in Go's order, unset ones are left out, and details are sorted with a log event's level and message left out
- [x] Numbers, booleans, missing values, lists, maps and durations are written as Go writes them
- [x] Values with spaces, `=`, quotes or control characters are quoted and escaped, empty values are `""`, and multi-line values use charmbracelet's indented form
- [x] Each level, key, separator and value is styled with the style the plan gives it, and the source, operation and resource keys and values with theirs

#### - [x] Task: Add the pretty logger
**Id:** 11f281eb-15c4-4d3c-9ae5-b331a3072311
**Repo:** xcl-python
**Depends on:**
- 319db6ce-633f-4b6d-a454-4c292d0493e8 — Deliver asyncio events in order
- 82bca570-5792-419f-95dc-7f49a6a3882b — Let a handler reach the configuration that delivered its event
- 4d9d42b7-48c0-4d0f-822d-8bb39ae3638e — Lay out event lines as Go's example writes them
**Execution:** agent

Add the public `xcl.prettylog` module with the handler factory, the level helper that reads `XCL_LOG_LEVEL`, and the variable's name. The handler writes each event at or above its level as one line. At info and below, it writes each created resource's configuration with computed values beneath its creation line, a configuration mistake's report beneath its error line, one warning per resource that cannot be shown, and the once-only guidance when events carry no resource data. It writes to a stream, coloured only for a terminal without `NO_COLOR`, or as records through an application's logger, and it never raises into the operation.

*Technical detail:* [context.md#task-add-the-pretty-logger](./context.md#task-add-the-pretty-logger)

**Acceptance criteria**:
- [x] `XCL_LOG_LEVEL` set to "debug", "warn", "error", "INFO", unset and "loud" gives debug, warn, error, info, info and info
- [x] Events below the chosen level write nothing, and above info only event lines and configuration-mistake reports are written, with no configuration, warnings or guidance
- [x] At warn, an unsupported block's report still appears beneath its error line
- [x] A creation event delivered by a configuration is followed by that resource's configuration, indented, before anything else is written
- [x] Variables, outputs and modules show nothing and warn nothing, and each resource that cannot be shown writes one warning while later events still appear
- [x] With resource data off, two created resources produce exactly one line explaining how to turn it on
- [x] An unsupported block's report appears once, beneath the error line about it
- [x] Routed through a logger, each event becomes one record whose message is the same text, at the event's level and time, with its fields attached
- [x] A non-integer level, or a stream and a logger together, is refused

#### - [x] Task: Prove the pretty logger's behaviour against the real engine
**Id:** 7caa2aa7-1371-4f66-8a4b-fa5b0ca29442
**Repo:** xcl-python
**Depends on:**
- 11f281eb-15c4-4d3c-9ae5-b331a3072311 — Add the pretty logger
**Execution:** agent

Add integration tests that drive real applies through the pretty logger, on the synchronous and asyncio clients, with the recording fixture plugin whose provider fills in a value. They cover every acceptance criterion a real run can show: the event lines, the configuration beneath creation, the asyncio run, the logger route, the warn and debug levels, the configuration mistake, colour on a pseudo-terminal only, the guidance with resource data off, and silent builtins.

*Technical detail:* [context.md#task-prove-the-pretty-loggers-behaviour-against-the-real-engine](./context.md#task-prove-the-pretty-loggers-behaviour-against-the-real-engine)

**Acceptance criteria**:
- [x] Applying a configuration that creates one resource, with the pretty logger set up in one line on a captured stream and no level chosen, writes one line per event at info or above, each with a time, level, message, source, operation, phase and resource
- [x] The lines directly after the creation line hold that resource's configuration, indented, including the value the provider filled in, before the next event's line
- [x] The same run on the asyncio client gives the same lines apart from timestamps and durations
- [x] The same run through an application's own logger writes the same lines through that logger's handlers
- [x] At warn the run writes no configuration and no info lines, and at debug it adds debug events
- [x] At warn, a configuration with an unsupported block still writes the report beneath the error line
- [x] A configuration with an unsupported block writes the error line followed by the report with its summary, file and line, and faulty source lines
- [x] A pseudo-terminal gets colour codes, with the source, operation and resource values in their colours, while a file, or a pseudo-terminal with `NO_COLOR` set, gets none
- [x] With resource data off and two created resources, the run writes exactly one guidance line and no configuration, and a configuration with a variable, an output and a module writes no warning for them

### Milestone 3: Python's pretty output is proven to match Go's example

**What changes**: The Go and Python parity programs now both apply every shared fixture through their pretty logger, Go's example and `xcl.prettylog`, and the comparison fails the build on any difference in the event lines or the configuration beneath them, apart from timestamps and durations. Users get a guarantee that the Python pretty logger shows what Go's does. The work adds no new behaviour.

**Validation point**: The Go parity program's tests pass with the new stage, which is deterministic across runs. The Python parity suite passes on every fixture against the parity program built from the xcl branch.

#### - [x] Task: Record the pretty log in the Go parity program
**Id:** 327664ad-088a-4e77-b581-968677276eb1
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Extend xcl's Go parity program with a stage that applies each fixture whose apply succeeds, in a fresh state directory, through Go's example pretty logger at info with processed event data into a plain buffer. It records the normalised groups of event lines with the configuration beneath them. The program's module gains the example and its terminal-styling libraries as dependencies, pinned at the example's versions, with the reason written beside them. This needs the printing-core plan on the branch, and it must merge before the xcl release that xcl-python pins.

*Technical detail:* [context.md#task-record-the-pretty-log-in-the-go-parity-program](./context.md#task-record-the-pretty-log-in-the-go-parity-program)

**Acceptance criteria**:
- [x] The record of every fixture whose apply succeeds holds its pretty log, with times, durations, colour and run paths normalised and each line grouped with the configuration beneath it
- [x] A fixture whose apply fails records no pretty log
- [x] The record is still the same on every run of the same fixture

#### - [x] Task: Compare the pretty log with Go in the Python parity suite
**Id:** b031762f-27d9-4464-9095-9f0ac310e976
**Repo:** xcl-python
**Depends on:**
- 327664ad-088a-4e77-b581-968677276eb1 — Record the pretty log in the Go parity program
- 11f281eb-15c4-4d3c-9ae5-b331a3072311 — Add the pretty logger
**Execution:** agent

Extend the Python parity program to record the same stage through `xcl.prettylog`, normalised by the same rules, so the existing comparison holds Python's pretty output to Go's on every shared fixture.

*Technical detail:* [context.md#task-compare-the-pretty-log-with-go-in-the-python-parity-suite](./context.md#task-compare-the-pretty-log-with-go-in-the-python-parity-suite)

**Acceptance criteria**:
- [x] For every shared fixture whose apply succeeds, the Python pretty logger writes the same event lines and configuration as Go's example, apart from timestamps and durations
- [x] For a fixture whose apply fails, both sides record no pretty log

### Milestone 4: The README, the parity list and xcl.dev show the pretty logger, and it ships

**What changes**: Readers of the package README and xcl.dev's events page find the one-line setup for both clients, the logger route, the environment variable, the resource data it needs and a sample of its output. The website's hand-written Python receiver is replaced by the real module. The README notes that asyncio now delivers events in order. The package is released as a minor version, bundling the xcl release that carries the parity stage and the work this plan builds on.

**Validation point**: The manual reviews of the README, the parity list and the website pass, and the site builds. xcl-python's main branch pins the xcl release, its checks pass, and the minor version is published.

#### - [x] Task: Document the pretty logger in the README and parity list
**Id:** 276623de-9820-4e70-93a2-a126ed229564
**Repo:** xcl-python
**Depends on:**
- 11f281eb-15c4-4d3c-9ae5-b331a3072311 — Add the pretty logger
**Execution:** agent

Add a pretty-logging section to the README. It shows the one-line setup for both clients, the level and `XCL_LOG_LEVEL`, the processed event data it needs, routing through an application's logger and a sample of the output. It also says that asyncio handlers now receive events in order. Pair Go's example pretty logger with `xcl.prettylog` in the parity list.

*Technical detail:* [context.md#task-document-the-pretty-logger-in-the-readme-and-parity-list](./context.md#task-document-the-pretty-logger-in-the-readme-and-parity-list)

**Acceptance criteria**:
- [x] The README shows the one-line setup for both clients, the level and its environment variable, the event data needed, the logger route and a sample of the output
- [x] The README says that asyncio event handlers receive events one at a time, in order
- [x] The parity list pairs Go's example pretty logger and its level helper with their Python forms

#### - [x] Task: Document the pretty logger on the website's events page
**Id:** c1d31990-9eb3-4df1-868c-c46723e9d83c
**Repo:** xcl-website
**Depends on:**
- 11f281eb-15c4-4d3c-9ae5-b331a3072311 — Add the pretty logger
**Execution:** agent

Replace the Python tabs of the events page's pretty-receiver section, which today show a hand-written receiver with its own escape codes, with `xcl.prettylog`'s one-line setup on both clients and a sample of its output. Reword the surrounding sentences that say the Python tab is written by hand. The page is edited in place, on top of the printing plans' edits to the same page, with nothing renamed or removed.

*Technical detail:* [context.md#task-document-the-pretty-logger-on-the-websites-events-page](./context.md#task-document-the-pretty-logger-on-the-websites-events-page)

**Acceptance criteria**:
- [x] The events page's pretty-receiver Python tabs show `xcl.prettylog`'s one-line setup and a sample of its output
- [x] No hand-written Python receiver or escape-code table remains on the page
- [x] Every existing page URL still works and the site builds

**Descoped requirements**:
- Release the parity stage and the package with the pretty logger (task id 3b46bc75-709e-48fb-a005-4d3a08ecc801) — descoped: done by a person after the merge. Releasing and labelled merges are outside the repo and the user decided during epic implementation that human release tasks are follow-up work. What the person must do: (1) make sure the xcl release xcl-python will bundle contains the parity program's `pretty_log` stage, the printing core, engine and parity work, and saved-record rendering during an operation, cutting a new xcl release if the printing release already went out without the stage; (2) pin that release in xcl-python's `xcl-ref.txt` (on this work's branch or on the automated engine-bump pull request), confirm CI including the parity suite passes, and merge with the `minor` label so the next minor version is published; (3) once the release is published, add a line to its generated GitHub release notes saying that the asyncio client now delivers events to `on_event` one at a time and in order, so a handler that relied on running concurrently may slow an operation; (4) let the website change go out after the release. Checked by: the published release's notes contain the asyncio ordering line, the published version's `xcl.engine_version()` reports the pinned xcl release, `import xcl.prettylog` works from the PyPI install, and the website shows the new tabs.

## Open Questions

- **Do Go's example and the Python pretty logger see the same events for a parity fixture?** Both run the same engine code, so a successful apply should emit the same events, and the stage sorts line groups so that concurrent resources cannot reorder them. Only running the fixtures shows three things: whether the engine path adds or drops an event compared with Go in process, whether a plugin's info messages carry values that vary between runs, and whether a detail value Go holds as a non-JSON type prints differently once it crosses the engine as JSON. This depends on the parity fixtures, the engine's event delivery and the printing plan's `log` stage, which asks the same question. If the groups differ in which events appear, or in a value's formatting that the layout cannot match, the implementer must STOP and ask the user. They must not filter lines or drop the stage.
- **Does the landed printing support match the shapes recorded in its plan?** This plan calls `saved_entity_tokens`, `format_error`, `err.tokens`, `for_stream`, `TERMINAL`, `PLAIN`, `DEFAULT_THEME` and the logging bridge's module as the printing plan describes them, and none of that code exists yet. Where a name or signature differs, the implementer follows the landed code. If a capability this plan needs is missing, for example the terminal formatter does not write a one-token stream as that style's codes and a reset, the implementer must STOP and ask the user.

Everything else was settled during planning and is recorded in the drafting assumptions.

## Out of Scope

- **Changing Go's example pretty logger** (spec Non-Goal). `example/prettylog` is read and run by the parity program but not modified, so it keeps writing no report beneath errors at any level and no guidance line. Those are Python additions the spec asks for, and the user confirmed that Python shows reports at every level. The parity stage leaves failing fixtures out for that reason.
- **Structured or JSON log output** (spec Non-Goal). `xcl.logging_handler` from the printing plan already serves it. The pretty logger's logger form attaches the same fields to its records but makes no JSON of its own.
- **Log file handling such as rotation** (spec Non-Goal). A user who wants files passes a logger whose handlers rotate them.
- **Configurable styles or themes for event lines.** The line styles are fixed to Go's choices, and configuration and reports use the default theme. A `formatter=` override is offered, but there is no theme override.
- **Configuration, render warnings or guidance above info.** Above info the pretty logger writes event lines and configuration-mistake reports only.
- **Configuration beneath update or read events.** Only creations show configuration, as in Go's example.
- **Nanosecond durations.** Python events hold durations at microsecond resolution, so a `duration=` value can differ from Go's in its last digits. Durations are timing values and are not compared.
- **A TypeScript pretty logger.** There is no TypeScript package yet.
- **A changelog entry in xcl.** The only xcl change is the parity program's test stage, and xcl-python keeps no changelog file. The release is a `minor` with no breaking change to note, apart from asyncio's ordered event delivery, which the README describes.
- **New website pages or navigation.** Only the events page's existing pretty-receiver section and delivery guarantee change.
- **Fixing type errors that `py.typed` may surface in unrelated annotations.** This plan keeps its own module's annotations accurate.

## Changelog

### Plan change: release task descoped (2026-10-26)

- **What changed**: the human task "Release the parity stage and the package with the pretty logger" (id 3b46bc75-709e-48fb-a005-4d3a08ecc801) was taken out of the `## Milestones & Tasks` checklist and recorded under `**Descoped requirements**:` as done by a person after the merge, with the steps the person must take, including the release-notes line about asyncio's in-order event delivery. A matching manual check was added to the Testing Approach so the implementation test plan carries it.
- **Why**: the user decided for this epic that human release tasks are follow-up work; an agent never releases, pushes, opens or merges pull requests, or edits GitHub releases.

### 2026-10-26 — Task: Deliver asyncio events in order

**What was done**: `xcl.aio.Config._drive` now puts each event on a per-operation `asyncio.Queue` drained by one consumer task (`_deliver_events`), so `on_event` is called once per event, in order, each call finished before the next starts. The consumer sits in the pending set, so the operation awaits it after a `None` sentinel and cancels it with the other tasks; handler exceptions are swallowed in the consumer as `gather(return_exceptions=True)` swallowed them before. State and function tasks are unchanged. Module and class docstrings describe the new delivery.

**Deviations**: None. `_dispatch` takes the queue as a third argument and builds the `Event` itself; `_handle_event` was replaced by `_deliver_events`. Added two tests beyond the plan's list (coroutine handler one at a time; coroutine handler exception).

**Files changed**:
- `xcl-python: xcl/aio.py`
- `xcl-python: tests/unit/test_aio_events.py`

**Discoveries**: the fake transport's start-operation message is `core_pb2.StartOperationResponse`; a cancellable fake needs its own `Poll` that returns empty responses until `Cancel` is called. The xcl-python knowledge gotcha `aio-events-out-of-order.md` is now out of date (events are ordered); `aio-state-tasks-run-beside-handlers.md` still holds (state tasks still run beside the handler).

### 2026-10-26 — Task: Let a handler reach the configuration that delivered its event

**What was done**: Every event a configuration delivers now carries it privately as `Event._config` (set in `_poll._dispatch` through a new `Handlers.config`, and in the asyncio `_dispatch`); it is not a dataclass field, so equality and repr ignore it. Both flavours store `_event_data` and offer a private blocking `_saved_entity_tokens_now`, sharing a module-level `_saved_entity_tokens` that calls `EncodeSavedEntity` straight through the client on the calling thread (the asyncio flavour never goes through its poll executor). The sync `saved_entity_tokens` delegates to it. The logging bridge's private rules became shared helpers `event_level`, `event_message`, `event_extra` and a new `event_record`, with `logging_handler` unchanged in behaviour.

**Deviations**: The bridge already had private `_level`, `_message`, `_extra` and `_set_time` (the plan expected to extract them); they were renamed to the plan's names and `event_record` was added. `Event._config` is declared with a type comment (`_config = None  # type: Any`), because an annotation would make it a dataclass field.

**Files changed**:
- `xcl-python: xcl/events.py`
- `xcl-python: xcl/_poll.py`
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/aio.py`
- `xcl-python: xcl/_logbridge.py`
- `xcl-python: tests/unit/test_events.py`
- `xcl-python: tests/unit/test_config.py`

**Discoveries**: A private attribute on a dataclass that must stay out of fields, equality and repr needs no annotation; typing it with a `# type: Any` comment keeps mypy happy without turning it into a field.

### 2026-10-26 — Task: Lay out event lines as Go's example writes them

**What was done**: New private `xcl/_prettyline.py` ports charmbracelet/log v0.4.2's text layout with `events.SlogHandler`'s attribute rules: `format_line(event, level, message, style)` and `format_record(time, level, message, attrs, style)` (for the warning and guidance lines), `kitchen`, `level_label`, `attributes`, and Go's value formatting (`go_value`, `go_float` for `strconv` 'g' shortest, `go_duration` for `time.Duration.String`), with charmbracelet's quoting, escaping and multi-line layout. Styling goes through a caller-supplied `style(text, style)` with `STYLES`. `tests/unit/test_prettyline.py` holds 62 one-rule tests whose expected lines were produced by running Go.

**Deviations**: Added `format_record` (the plan described "a helper that takes explicit attributes") and a `plain` styler default. The Go program printing the expected lines was run from a scratch directory outside the repos and is not kept.

**Files changed**:
- `xcl-python: xcl/_prettyline.py`
- `xcl-python: tests/unit/test_prettyline.py`

**Discoveries**: charmbracelet drops an empty message entirely (no double space). A multi-line value writes `\n  key=\n` then `  │ ` lines; only the last line takes the key's value style, it ends in a line break only when more attributes follow, and the next attribute then starts with a space at the start of a line. A value ending in a line break gives a blank line before the final break. Lists and maps are quoted because Go's `[a 1]` and `map[a:x]` contain spaces. Go's `unicode.IsPrint` equals Python's `str.isprintable()` plus the ASCII space.

### 2026-10-26 — Task: Add the pretty logger

**What was done**: New public module `xcl.prettylog` (`LEVEL_ENV`, `level_from_env()`, `handler(file=None, level=logging.INFO, *, logger=None, formatter=None)`), exported as `xcl.prettylog`. The handler writes each event at or above its level as Go's example line (through `_prettyline`, each styled piece formatted as one `Kind.TEXT` token with a one-entry theme by the chosen formatter, `for_stream(file)` by default, `PLAIN` with a logger). At info and below it renders each `create success` from the delivering configuration's `_saved_entity_tokens_now(event.data, include_computed=True)`, indented beneath the line; `NotEncodableError` is silent, any other failure is one `WARN unable to show configuration` line; with `EventDataLevel.NONE` one guidance line is written once per handler. At every level a configuration mistake's report (`format_error`, default theme, width 80) is written beneath its error line, once per problem. With `logger=`, each group is one record built by the bridge's `event_record`. Unit tests (75, written by a test sub-agent) in `tests/unit/test_prettylog.py`; `tests/unit/test_docstrings.py` walks the new module.

**Deviations**: (1) The shown-problem count is kept per configuration (by `id(event._config)`), cleared at that configuration's operation start, rather than one count per handler, because the test author showed that two configurations sharing a handler could hide or repeat each other's reports. (2) In logger mode the handler returns early when the logger is not enabled for the event's level, so no render, warning or guidance is produced for a line the logger would drop. (3) `_indent` mirrors Go's example exactly (always ends in one line break) rather than giving `""` for empty text. (4) The render warning and guidance lines carry the event's time.

**Files changed**:
- `xcl-python: xcl/prettylog.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_prettylog.py`
- `xcl-python: tests/unit/test_docstrings.py`

**Discoveries**: Against the real engine an unsupported nested block in a plugin resource is reported as a `validate error` event carrying the resource id (not a `parse error`), then the closing `apply error`; both count as "a block's own error" vs "closing" under the once-per-problem rule. Variables and outputs get `create success` events, and with `EventDataLevel.NONE` the first one triggers the guidance line.

### 2026-10-26 — Task: Prove the pretty logger's behaviour against the real engine

**What was done**: Engine-backed tests for every acceptance criterion a real run can show, with the recording fixture plugin: event lines with one-line setup, the configuration (with the provider's `output = "one-a"`) directly beneath the creation line, the logger route equal to the stream after normalising times and durations, warn terse, warn still showing the unsupported block's report, debug adding `decided every entity`, the configuration mistake's report (summary, `on <file> line 7`, `     7 |   nope {`) written once, colour on a pseudo-terminal only (and none with `NO_COLOR` or to a file), one guidance line with data off, and silent builtins (variable, output, module). Two asyncio tests in `test_aio.py`: same normalised lines as the sync run, and the block after the creation line.

**Deviations**: The sync integration file is `tests/integration/test_prettylog_engine.py`, not `test_prettylog.py`: pytest refuses two test modules with the same basename (`tests/unit/test_prettylog.py`) in this suite's rootdir-less layout. The unsupported-block configuration uses the recording plugin type rather than a Python-registered type; the engine reports it as a `validate error` on the resource, then the closing `apply error`.

**Files changed**:
- `xcl-python: tests/integration/test_prettylog_engine.py`
- `xcl-python: tests/integration/test_aio.py`

**Discoveries**: xcl-python's tests directories are not packages, so a test module name must be unique across `tests/unit` and `tests/integration`. A pty in tests needs a reader thread draining the controller end, and reading it after the terminal end closes raises `OSError` (EIO) on Linux, which ends the read.

### 2026-10-26 — Task: Record the pretty log in the Go parity program

**What was done**: xcl's parity program gains a `pretty_log` record key: for a fixture whose apply succeeds it applies the fixture again in `<tmp>/pretty-state` through Go's real `example/prettylog.Handler` at info with `xcl.WithEventData(xcl.EventDataProcessed)` into a buffer, and records the sorted groups (an event line joined with the indented lines beneath it), with SGR codes stripped, the Kitchen time replaced by `<time>`, `duration=` values by `<duration>` and run paths by `n.text`; it is `null` when the apply fails. `newConfig` takes extra `xcl.ConfigOption`s. `bindings/libxcl/go.mod` requires `example/prettylog` (replaced to `../../example/prettylog`) with a comment giving the reason; `go mod tidy` brought charmbracelet/log v0.4.2, lipgloss v1.1.0 and termenv v0.16.0 in as indirect requirements at the example's versions. The parity README documents the key, and the Go tests cover the stage (configuration under create success, null for the failing fixture, no times/durations/colour, sorted, no temp paths) and the grouping normaliser.

**Deviations**: The plan's `os.Setenv("CLICOLOR_FORCE", "")` was not added: the buffer is not a terminal, so lipgloss writes plain text, and colour codes are stripped regardless. The existing `TestParityInvalidFixtureReportsEveryProblem` key list gained `pretty_log`. Added a `TestParityPrettyLogGroupsAreSorted` test and three normaliser unit tests. No xcl CHANGELOG section, as planned.

**Files changed**:
- `xcl: bindings/libxcl/cmd/parity/main.go`
- `xcl: bindings/libxcl/cmd/parity/main_test.go`
- `xcl: bindings/libxcl/go.mod`
- `xcl: bindings/libxcl/go.sum`
- `xcl: bindings/libxcl/testdata/parity/README.md`

**Discoveries**: Go's example renders the configuration from `e.Entity()` with `xcl.PrintEntity`, and its output for the recording plugin includes the `# set by the provider` comment after a computed field. At info, a successful plugin fixture's pretty log holds apply, load, parse and create events only; debug-level plugin events (which carry pids and socket paths) never reach it.

### 2026-10-26 — Task: Compare the pretty log with Go in the Python parity suite

**What was done**: The Python parity program records `pretty_log` the way the Go program does: for a fixture whose apply succeeds it applies the fixture again in `<tmp>/pretty-state` through `xcl.prettylog.handler(buf, logging.INFO)` with `event_data=xcl.EventDataLevel.PROCESSED`, and `Normaliser.pretty_groups` strips colour, replaces the leading time and durations, replaces run paths and sorts the groups; it is `None` when the apply fails. `new_config` passes extra `xcl.Config` options through. The existing per-fixture comparisons in `test_parity.py` now compare the pretty log on every shared fixture, and all pass: the Python pretty logger writes the same event lines and configuration as Go's example. Three normaliser unit tests mirror the Go ones.

**Deviations**: Added `tests/parity/test_normaliser.py` tests for the grouping (not in the plan's list). The pretty-log configuration is closed after its apply.

**Files changed**:
- `xcl-python: tests/parity/run_python.py`
- `xcl-python: tests/parity/test_normaliser.py`

**Discoveries**: The parity suite runs the Go parity binary from xcl-python's `build/plugins`, so after changing the Go program `make plugins XCL_SRC=<xcl checkout>` must run again, or the comparison silently uses the old Go record and fails on the new key.

### 2026-10-26 — Task: Document the pretty logger in the README and parity list

**What was done**: The README gains a "Pretty logging" section: the one-line setup on `xcl.Config` and on `xcl.aio.Config`, a sample of the output with a create-success line and its configuration, the level and `XCL_LOG_LEVEL`, the processed event data it needs and the guidance line, reports beneath configuration mistakes at every level, terse output above info, colour only on a terminal without `NO_COLOR`, and the logger route with `%(message)s`. The Asyncio section now says state-store calls and functions run as concurrent asyncio tasks while events reach `on_event` one at a time, in order. PARITY.md pairs `prettylog.Handler`, `LevelFromEnv`, `LevelEnv` and `slog.Level` with their Python forms and notes that Go's is an example while Python's is part of the package.

**Deviations**: The section sits under "## Logging" as "### Pretty logging" rather than as a "####" subsection of "Variables, state and events", next to the logging bridge it builds on. The PARITY.md pairing is a table after the Logging table rather than rows under "Configuration options".

**Files changed**:
- `xcl-python: README.md`
- `xcl-python: PARITY.md`

**Discoveries**: None.

### 2026-10-26 — Task: Document the pretty logger on the website's events page

**What was done**: On xcl.dev's events page, the pretty-receiver section's Python tab now shows `xcl.prettylog`'s setup with processed event data and a sample of its output (a create-success line with its configuration), in place of the hand-written receiver with its own ANSI table; the intro says Python ships the same receiver as part of the package. A sentence says `xcl.aio.Config` takes the same handler and `logger=` routes the lines through standard logging; another describes the Python receiver's rendering from the saved record, its colour choice, its reports beneath configuration mistakes and its guidance line. The one-line Python tab uses `xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env())`. The "Ordered, one at a time" guarantee now says both Python clients deliver events the same way. The site builds (`npm ci`, `npm run build`, 23 pages) and the events page's internal links resolve.

**Deviations**: The Python output sample sits inside the first Python tab rather than after the Go sample output.

**Files changed**:
- `xcl-website: src/pages/events.mdx`

**Discoveries**: None.
