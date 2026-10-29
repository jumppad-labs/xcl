---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Research: 20261023075328-741138ee-python-prettylog

## Alternatives considered and rejected

### Option A: Pretty formatting as a `logging.Formatter` applied to the logging bridge's records (`xcl.logging_handler` + `xcl.prettylog.Formatter`)

Records would keep Go's bare message and the user would attach the formatter to their own handler to get pretty lines.

**Rejected**: The spec's "route the pretty logger's output through a logger of their own" and the acceptance criterion "writes the same lines through that logger's handlers" mean the pretty text itself travels through the logger; a formatter-only design makes the user configure their handler before any line matches, and the configuration block would have to be smuggled onto records for the formatter to find. Evidence: spec Requirements "Works with existing logging", AC "Through an existing logger".

### Option B: A hand-written ANSI table of escape codes in `xcl.prettylog` (as the website's current Python tab does, `xcl-website:src/pages/events.mdx:358-394`)

**Rejected**: The spec's constraints require colours, formats and reports to come from the package's printing support, not from the pretty logger's own copies. Colours are written as style strings and rendered by `xcl.highlight` formatters.

### Option C: Building one token stream per event line with private, out-of-range `Kind` numbers and a private theme

It would render a whole line in one formatter call.

**Rejected**: It invents kinds outside the protocol's numbering (`design:printing-and-highlighting.md` "Token kinds", kinds mirrored from the proto) and leaks them to any custom formatter. Each styled piece is instead formatted as one `Kind.TEXT` token with a one-entry theme, which uses only the public formatter seam.

### Option D: Leaving `xcl.aio.Config` event dispatch as it is (one asyncio task per event; plain handlers run concurrently on the default executor, `xcl-python:xcl/aio.py:158-171,302-310`)

**Rejected**: Events then reach a handler out of order and concurrently, so the asynchronous run cannot "produce the same lines apart from timestamps" (spec AC "Asynchronous client"), and a configuration block could land after the next event's line. The same flaw already affects `logging_handler` under asyncio.

### Option E: An `async def` pretty handler for asyncio serialised with an `asyncio.Lock`

**Rejected**: One callable cannot be a plain handler for `xcl.Config` (whose driver ignores a returned coroutine, `xcl-python:xcl/_poll.py:197-201`) and a coroutine function for `xcl.aio.Config` on Python 3.11 (`inspect.iscoroutinefunction` sees only real `async def` functions), so it would force two factories; and awaiting `aio.Config.print_saved_entity` from a handler queues behind the poll executor's in-flight `Poll` (single worker, `xcl-python:xcl/aio.py:66,112-114,220-222`), i.e. waits on the operation, which the spec's constraint forbids.

### Option F: Buffering configuration until the operation ends, or rendering on a background thread

**Rejected**: The spec requires the configuration "as soon as the creation is reported" and "before the next event's line"; this is why the sibling spec `20261023074627-cc23fc08-encode-saved-entity-during-operation` allows `EncodeSavedEntity` mid-operation.

### Option G: Rendering from the event's hydrated entity with `encode_entity` / `print_entity`

**Rejected**: `EncodeEntity` stays refused during an operation (`xcl:coreservice/lookups.go`, plan cc23fc08), and the spec's Technical Approach says to render from the event's saved record.

### Option H: Comparing Python with Go through literal expected strings only

**Rejected**: For the success metric, as the printing plan rejected it: literals agree with Go only at authoring time. The Go parity program runs Go's real `example/prettylog` instead.

### Option I: A separate Go command in the `example/prettylog` module for the comparison

**Rejected**: It would duplicate the parity program's fixture setup (`xcl:bindings/libxcl/cmd/parity/main.go:159-210`), and xcl-python's Makefile already builds the parity program from the xcl checkout (`xcl-python:Makefile:41-45`).

### Option J: Writing a report beneath every error event that carries one

**Rejected**: An unsupported block produces a `parse error` event carrying that one diagnostic (`xcl:internal/parser/parser.go:860-940`, `emitParse` in `internal/parser/events.go:160-183`) and then the operation's closing `apply error` event carrying the whole `ConfigError` (`xcl:config.go:655-668`), so the same report would appear twice.

### Option K: A `xcl.prettylog.Level` StrEnum for the level

**Rejected**: The Go type is the standard library's `slog.Level`; its Python counterpart is the standard library's `logging` level integer, which `logging.Logger.setLevel` and every handler already use. The epic's "top-level StrEnum named after the Go type" pattern covers xcl's own option types (`xcl.EventDataLevel`).

## Chosen approach — evidence

- Go reference: `xcl:example/prettylog/prettylog.go:37-178` — `Handler(w, level)` wraps `events.SlogHandler` around charmbracelet/log (`TimeFormat: time.Kitchen`, `ReportTimestamp: true`), returns the bare delegate above info, writes configuration only on `create`/`success`, warns `"unable to show configuration"` with `resource` and `error`, skips `ErrNotEncodable` silently, indents non-empty lines by two spaces, styles keys/values of `source` (5), `resource` (6), `operation` (3); `LevelFromEnv` reads `XCL_LOG_LEVEL` lower-cased: debug/warn/error, else info.
- Go tests to mirror: `xcl:example/prettylog/prettylog_test.go:84-544` — level-from-env per value, one line per event, debug only at debug (`DEBU`), configuration lines after create success with `provider_id`, nothing extra without data, nothing above info, builtins silent, a hand-built event with data warns once and later events still write, forced colour `\x1b[1;35m` block type and `\x1b[34m` attribute.
- charmbracelet/log v0.4.2 text format (`~/go/pkg/mod/github.com/charmbracelet/log@v0.4.2/text.go:178-272`, `styles.go:47-82`, `logger.go:84-140`, `logger_121.go:34-50`): `<time> <LEVL> <message> key=value...\n`; level `DEBU`/`INFO`/`WARN`/`ERRO` (MaxWidth 4), bold, colours 63/86/192/204; keys and `=` faint; empty message omitted; empty value `""`; a value with `"`, `=`, a space or a non-printable rune is quoted and escaped; a multi-line value is written as `\n  key=\n  │ line` lines; values are `fmt.Sprintf("%+v", slog.Value)`.
- Go value formatting checked by running Go (2026-10-23): `100.0`→`100`, `1e6`→`1e+06`, `123456.0`→`123456`, `1234567.0`→`1.234567e+06`, `0.0001`→`0.0001`, `1e-05`, `nil`→`<nil>`, `[a 1]`, `map[a:x b:1]`, durations `5ms`, `1.5ms`, `1m1s`; `time.Kitchen` of 00:04 → `12:04AM`.
- Go attribute order: `xcl:events/slog.go:20-120` — `source`, `operation`, `phase`, `resource`, `type`, `file` (when set), `duration` (non-zero), `error` (`Error()`), then Meta keys sorted, a log event's `level`/`message` excluded; levels: log→own, error phase→error, blocked→warn, else info; message `"<operation> <phase>"` or the log message.
- Event data and back-reference: `xcl-python:xcl/events.py:16-90` (`Event._classes` set by the delivering configuration and hidden from fields/equality), `xcl-python:xcl/_poll.py:62-70,190-201` (`Handlers`, `_dispatch`), `xcl-python:xcl/aio.py:166-171` (`_handle_event`) — the place to also attach the delivering configuration.
- Transport is concurrent: `xcl-python:xcl/_transport/subprocess.py:1-30,297-310` (responses matched by id, a waiting `Poll` never holds up another thread's call), so a render from a handler thread does not wait on `Poll`.
- Error events: `xcl:internal/parser/events.go:160-198` (`emitParse` per block/diagnostic, `emitOperationError`), `xcl:config.go:640-668` (closing `<operation> error` event carries the operation's error). `xcl-python:xcl/_errors.py:206-219` (`ConfigError.problems`, one per positioned cause).
- Printing support to build on (plan `20261023075302-bad71237-printing-and-highlighting-python`): `xcl.highlight.Kind`, `Token`, `DEFAULT_THEME`, `TERMINAL`, `PLAIN`, `for_stream`, `Formatter`; `xcl.format_error(err, formatter=, theme=, width=)`; `err.tokens`; `Config.saved_entity_tokens` / `print_saved_entity`; `xcl.logging_handler` (Go's level/message/attribute rules, record built with `makeRecord`, event time as `created`, `xcl_event` attached).
- Engine allows `EncodeSavedEntity` mid-operation (plan `20261023074627-cc23fc08-encode-saved-entity-during-operation`); `EncodeEntity` stays refused.
- Event data levels (plan `20261023074223-e06f835f-event-data-level-enum`): `xcl.EventDataLevel.NONE/RAW/PROCESSED`, only members accepted, `TypeError` otherwise; `py.typed` shipped.
- Fixtures: `xcl:bindings/libxcl/testdata/plugins/recording/main.go:23-110` (`resource "recording"` whose `output` is computed by create, external plugin `xcl-plugin-recording`); core emits a debug log event during apply (`xcl:internal/parser/parser.go:605`, `"decided every entity"`).
- Parity harness: `xcl:bindings/libxcl/cmd/parity/main.go:1-200` (stages, `newConfig`, normaliser), `xcl-python:tests/parity/run_python.py:1-135`, `tests/parity/test_parity.py` (first differing JSON path), `xcl-python:Makefile:41-45` (builds parity from `XCL_SRC`).
- Website target: `xcl-website:src/pages/events.mdx:332-443` ("A pretty receiver": Go tab, a hand-written Python tab with its own ANSI table, the one-line setup tabs already naming `prettylog.handler(out, prettylog.level_from_env())`, sample output).

### Decisions the user confirmed during epic planning

- **Render inside the handler: allowed.** The constraint "must not wait on the operation, or on anything else slower than writing to its stream" is read as forbidding waiting on the operation, which deadlocks the loop driving it. The one `EncodeSavedEntity` render per created resource, inside the handler on its own thread, is allowed. Evidence: the spec requires configuration before the next event's line, rendered from the saved record, and spec cc23fc08's constraint defines the forbidden waiting as waiting on the operation.
- **Reports at every level.** "Quiet at higher levels … event lines only, with no configuration" means no resource configuration blocks. Configuration-mistake reports are written beneath their error line at every level, under the once-per-problem rule: parse and resource error events, and the closing operation error event only for problems not already shown. The guidance line and render warnings appear only at info and below. This differs from Go's example, which returns its bare bridge above info and writes no reports at any level. The Go parity stage runs at info on fixtures whose apply succeeds, so the difference never reaches the comparison.

## Files examined

- `xcl:example/prettylog/prettylog.go:1-178` — Go reference pretty logger (see evidence).
- `xcl:example/prettylog/prettylog_test.go:1-544` — behaviours and the forced-colour assertions.
- `xcl:example/prettylog/go.mod:6-21,60` — charmbracelet/log v0.4.2, lipgloss v1.1.0, termenv v0.16.0; replace of xcl to `../..`.
- `xcl:example/prettylog/testdata/{cache,encode}/main.xcl` — fixtures with a bare cache, a variable, a plugin type with a computed value.
- `xcl:example/{configonly,plugin,remoteplugin}/main.go` — every example sets it up in one line with `LevelFromEnv`.
- `xcl:events/slog.go:1-120` — SlogHandler level/message/attribute rules.
- `xcl:config.go:375-470,630-690` — Validate/Apply run through `c.run`; closing event with the operation's error.
- `xcl:internal/parser/events.go:150-198, parser.go:860-940` — parse error events per diagnostic.
- `xcl:internal/parser/parser.go:605` — a core debug event during apply.
- `xcl:bindings/libxcl/go.mod` — its own module, replace of xcl to `../..`; no charmbracelet today.
- `xcl:bindings/libxcl/cmd/parity/main.go:1-200` — parity stages and fixture setup.
- `xcl:bindings/libxcl/testdata/parity/` — fixtures (annotations, basic_types, encode, functions, invalid, outputs, plugin_entities, singletons, variables; the printing plan adds unsupported_block).
- `xcl:bindings/libxcl/testdata/plugins/recording/main.go:1-140` — computed `output`.
- `~/go/pkg/mod/github.com/charmbracelet/log@v0.4.2/{text.go,styles.go,logger.go,logger_121.go}` — exact text format.
- `xcl-python:xcl/events.py:1-90` — `Event`, `_classes`, `data`, `duration` (microsecond timedelta), `meta`.
- `xcl-python:xcl/config.py:1-319` — sync `Config`, `event_data` handling, encode calls, `_encode_options`.
- `xcl-python:xcl/aio.py:1-349` — per-event asyncio tasks, `_call_maybe_async` runs plain handlers on the default executor, single-worker poll executor.
- `xcl-python:xcl/_poll.py:1-212` — `Handlers`, `drive`, `_dispatch` (sync handler exceptions propagate).
- `xcl-python:xcl/_client.py, xcl/_transport/subprocess.py:1-60,164-310` — thread-safe concurrent calls.
- `xcl-python:xcl/_errors.py:20-60,175-240,440-471` — `Error`, `ConfigError.problems`, `from_proto`.
- `xcl-python:xcl/__init__.py:1-149` — exports; `mask` imported as a submodule.
- `xcl-python:tests/unit/test_events.py:1-230` — FakeTransport-driven event delivery tests for both flavours.
- `xcl-python:tests/fakes.py:1-60` — scripted transport.
- `xcl-python:tests/integration/conftest.py:1-60` — `recording_plugin`, `slow_plugin` fixtures, `make plugins`.
- `xcl-python:tests/parity/run_python.py:1-135, test_parity.py:1-60` — Python parity side.
- `xcl-python:README.md:221-306` — events and encoding sections; PARITY.md:1-94 — tables.
- `xcl-python:.github/workflows/test.yml:57-91` — Linux, macOS and Windows matrix; pyproject.toml:11 — Python >= 3.11.
- `xcl-website:src/pages/events.mdx:20-90,320-443` — logging section and "A pretty receiver".

## External references

- charmbracelet/log v0.4.2 source (module cache) — the exact line layout, quoting and level labels Go's example writes; Python reproduces it so lines compare equal.
- Go `fmt` `%v` for float64 and `time.Duration.String` (verified by running Go) — how attribute values print.
- Python `logging` (`Logger.makeRecord`, `Logger.handle`, `isEnabledFor`) — logger routing.
- NO_COLOR convention (no-color.org) — honoured through `xcl.highlight.for_stream`.
- Python `pty` module (POSIX only) — pseudo-terminal tests; skipped on Windows.

## Prior plans / specs consulted

- Plan `20261023075302-bad71237-printing-and-highlighting-python` (final) — provides every printing API used here; explicitly leaves the pretty logger and the website's pretty-receiver Python tab to this spec; adds parity `log` and `printed_entities` stages and an `unsupported_block` fixture; no xcl CHANGELOG section for a parity-program-only change.
- Plan `20261023074627-cc23fc08-encode-saved-entity-during-operation` (final) — `EncodeSavedEntity` allowed during an operation; Python handlers reach the configuration through a closure in its tests; notes the asyncio client's lookups queue on the single-worker executor between polls.
- Plan `20261023074223-e06f835f-event-data-level-enum` (final) — `xcl.EventDataLevel`, option pattern, `py.typed`.
- Plan `20261023074926-eaa51527-printing-and-highlighting-core` — Go's prettylog switches to `xcl.PrintEntity` with its own lipgloss colour decision; default theme keeps `\x1b[1;35m`/`\x1b[34m`.
- Spec `20261023074627-cc23fc08-encode-saved-entity-during-operation` — its constraint explains the meaning of "must not wait inside an event handler": waiting there for the operation deadlocks it.
- Design `printing-and-highlighting.md` (source `design`) — tokens/theme/formatter split, style strings, `for_stream`, Python print API, `logging_handler`.

## Open assumptions

- The printing plan's `saved_entity_tokens`, `format_error`, `err.tokens`, `TERMINAL`/`PLAIN`, `for_stream` and `logging_handler` land with the shapes recorded in its plan. If a name or signature differs, the implementer follows the landed code; if a capability is missing, STOP and ask.
- The printing plan's logging bridge exposes (or can expose without behaviour change) its level, message and record-building rules as private helpers the pretty logger can share. If it cannot without changing its behaviour, STOP and ask.
- The TERMINAL formatter writes a token with an empty style as plain text, and a one-token stream with a `{Kind.TEXT: style}` theme as that style's SGR codes followed by a reset. If not, STOP and ask.
- Every `create success` event carries data at `EventDataLevel.PROCESSED`, so the guidance line can be keyed on the delivering configuration's level being `NONE`.
- The engine emits the same events, in the same per-resource order, through Go in-process and through the Python engine, as the printing plan's `log` stage assumes. Concurrent resources may interleave differently between runs, so the parity stage compares event groups as a sorted list. If the groups differ beyond that, STOP and ask.
- Meta values decoded from JSON print as Go prints the in-process values for strings, booleans, integers and lists; an integral float Go holds as a float (for example `1e6`) prints as an integer in Python. The parity fixtures carry no such value; if one appears, STOP and ask.
- `str(err)` of an event's error equals Go's `Error()` (printing plan); multi-line values are written with charmbracelet's multi-line rule either way, so the open question on multi-problem `str(err)` does not change this plan.

## Drafting assumptions

### Chosen direction: a stdlib port of Go's example built on the printing support (architecture)
- **Decision**: a new public module `xcl.prettylog` with `handler(file=None, level=logging.INFO, *, logger=None, formatter=None)`, `level_from_env()` and `LEVEL_ENV`. It formats lines as charmbracelet/log does, styles each piece with style strings through an `xcl.highlight` formatter, renders configuration from the delivering configuration's `saved_entity_tokens` and reports with `xcl.format_error`, and shares the logging bridge's level and message rules. Key design decisions: the event carries a private back-reference to its configuration; a private blocking render bypasses the asyncio poll executor; `xcl.aio.Config` delivers events in order through one consumer; a `pretty_log` parity stage compares against Go's real example.
- **Rationale**: it meets every requirement with one setup line for both clients, uses only the standard library and the printing support, and proves the success metric against Go's actual output.
- **Rejected**: a pretty `logging.Formatter` over bridge records; an `async def` handler with a lock; hand-written ANSI codes; literal-only parity tests (see research.md alternatives).

### Line format reproduces charmbracelet/log's text format (discovery)
- **Decision**: Python writes `<h:mmAM> <LEVL> <message> key=value...` exactly as charmbracelet/log v0.4.2 does, with Go's SlogHandler attribute order, quoting, multi-line value layout and value formatting, the time in local time.
- **Rationale**: the success metric compares Go's lines with Python's, apart from timestamps and attribute order.
- **Rejected**: Python's `logging` default layout (different lines).

### asyncio delivers events in order (architecture)
- **Decision**: `xcl.aio.Config` delivers events to `on_event` one at a time, in arrival order, through one consumer task per operation; plain handlers still run on the default executor and `async def` handlers on the loop; handler exceptions are still swallowed as today; state and function tasks stay concurrent.
- **Rationale**: the asynchronous acceptance criterion needs the same lines in the same order; Go and the synchronous client already deliver events sequentially; it also fixes `logging_handler` ordering under asyncio.
- **Rejected**: leaving per-event concurrent tasks (lines out of order); an `async def` pretty handler with a lock (needs two factories on Python 3.11, and the asyncio lookup waits on `Poll`).

### Level is a standard logging integer (architecture)
- **Decision**: `level` takes a `logging` level `int` (default `logging.INFO`); anything else, including `bool` and level names, is refused with `TypeError`; `level_from_env()` returns `logging.DEBUG`, `INFO`, `WARNING` or `ERROR`.
- **Rationale**: Go's type is the standard library's `slog.Level`, whose Python counterpart is `logging`'s integer levels; refusing other types by type follows the epic's option pattern.
- **Rejected**: an `xcl.prettylog.Level` StrEnum (duplicates the standard library); accepting level names (a second spelling).

### Colours as named style strings (architecture)
- **Decision**: level labels use `bold blue`, `bold cyan`, `bold yellow`, `bold red` for DEBU, INFO, WARN, ERRO; source, resource and operation use magenta, cyan and yellow (bold values, plain keys); other keys and `=` are `dim`.
- **Rationale**: the design's style syntax has 16 named colours and 24-bit hex but no 256-colour index; named colours render on every terminal and stay close to charmbracelet's 63/86/192/204; the source, resource and operation colours are exactly Go's lipgloss basic colours 5, 6 and 3.
- **Rejected**: 24-bit hex approximations of the 256-colour indices (mis-rendered on terminals without true colour).

### Reports shown once per problem (architecture)
- **Decision**: a report goes beneath a `parse` or resource error event whose error carries one; the operation's closing error event (no resource, not `parse`) shows its report only when its problem count exceeds the problems already shown since the previous closing event; the count resets at each closing event.
- **Rationale**: an unsupported block emits a `parse error` with its diagnostic and then `apply error` with the whole `ConfigError`; printing both duplicates the report, while problems never reported earlier still appear.
- **Rejected**: a report beneath every error event (duplicates); only beneath the closing event (separates the report from the line about its block).

### Guidance keyed on the delivering configuration's event data level (architecture)
- **Decision**: one info line, at most once per handler: `configuration not shown: events carry no resource data hint="create the configuration with event_data=xcl.EventDataLevel.PROCESSED"`, written when a `create success` has no data and the configuration that delivered it was created with `EventDataLevel.NONE`. A hand-built event with no data writes nothing extra, as in Go.
- **Rationale**: precise, never misfires when data is on, and matches Go's "nothing extra without data" for events no configuration delivered. Info, not warn, because it is advice, not a failure.
- **Rejected**: triggering on any data-less create success (misleads if one ever arrives with data on); warn level.

### Logger routing carries the pretty text (architecture)
- **Decision**: with `logger=`, each event becomes one `LogRecord` at the event's level and time, whose message is the exact text the stream would get (line plus its block), carrying the logging bridge's structured extras; render warnings and the guidance are records of their own. `file` and `logger` together raise `ValueError`. The default formatter with a logger is `PLAIN`; `formatter=` overrides it in either mode.
- **Rationale**: the acceptance criterion asks for the same lines through the application's handlers; one record per event keeps a block with its line through filters and threads; structured fields keep JSON handlers useful.
- **Rejected**: records with Go's bare message (lines differ); a separate record for the block (can be filtered apart from its line).

### The handler never raises into the operation (architecture)
- **Decision**: render failures become one warning line; stream write errors are swallowed; with a logger, `logging`'s own `handleError` applies.
- **Rationale**: an exception from a synchronous `on_event` propagates out of the operation loop; Go's bridge discards write errors because a receiver must not affect processing.
- **Rejected**: letting write errors propagate.

### Success metric through a new parity stage, no xcl changelog section (architecture)
- **Decision**: add a `pretty_log` stage to xcl's parity program (importing `example/prettylog`) and to `run_python.py`, recorded for fixtures whose apply succeeds (`null` otherwise), normalised (timestamps and durations replaced, ANSI stripped, run paths replaced, line+block groups sorted). charmbracelet/log, lipgloss and termenv join `bindings/libxcl/go.mod` at `example/prettylog`'s pinned versions, with a comment. No xcl CHANGELOG section, because the only xcl change is the parity program, as the printing plan did for its parity stages.
- **Rationale**: proves "same lines and configuration as Go" against Go's real output on every shared fixture; Go's example writes no report or guidance, so failing fixtures are out of the comparison; Python's reports at every level (a user-confirmed difference from Go) therefore never reach it.
- **Rejected**: a Go command inside `example/prettylog` (duplicates fixture setup); literal expectations (agreement only at authoring time).

### Conventions selected (architecture)
- **Decision**: apply testing & mocking (no tables, positive/negative apart, tests beside code, no repo-file tests, testify in Go), dependencies, structured logs, Go code style; drop the Go-library layout, errors, test-helper, parser, graph, state-fixture, provider-testing, dependency-editing, database and services conventions.
- **Rationale**: the work is a Python module plus one Go test-tool stage; only those conventions bear on it.
- **Rejected**: listing every loaded convention.

### Durations compared like timestamps (testing_approach)
- **Decision**: wherever two runs are compared (the asyncio criterion, the parity stage), `duration=` values are replaced along with timestamps.
- **Rationale**: a duration is a timing measurement that differs on every run, just as the time does; the spec's "apart from timestamps" cannot otherwise hold for any two real runs.
- **Rejected**: comparing durations (never equal); dropping the attribute from the output (differs from Go).

### Unrenderable resources tested through the fake transport (testing_approach)
- **Decision**: the "two resources whose configuration cannot be rendered produce one warning line each, and the run completes" half of the criterion is a unit test driving a full `Config.apply` over the fake transport whose `EncodeSavedEntity` fails twice; the variables/outputs/modules half runs against the real engine.
- **Rationale**: a real engine renders every resource it has just created, so no honest real-engine configuration makes rendering fail.
- **Rejected**: corrupting event data or registries in a real run (tests a contrived path).

### Go expected lines pasted for the layout unit tests (testing_approach)
- **Decision**: the line-layout unit tests compare with expected lines produced once by running charmbracelet/log v0.4.2 and pasted as literals with a comment, in addition to the live parity stage.
- **Rationale**: same practice as the printing plan's word-wrap port; covers rules (quoting, multi-line values, floats) the parity fixtures never exercise.
- **Rejected**: running Go from Python unit tests (slow, needs Go).

### Task split: integration tests as their own task (tasks)
- **Decision**: the handler task carries its unit tests; the real-engine acceptance tests are a separate task that depends on it.
- **Rationale**: the integration tests need the engine and fixture plugins built from the xcl branch and cover both clients; keeping them apart keeps each task reviewable.
- **Rejected**: one large handler-plus-integration task.

### Website delivery-guarantees sentence (tasks)
- **Decision**: the website task also adds one sentence to the events page's "Ordered, one at a time" guarantee saying both Python clients deliver events the same way.
- **Rationale**: the asyncio change makes that true, and the page already states the guarantee for Go.
- **Rejected**: leaving the guarantee Go-only (a reader would assume asyncio differs).

### Release is a human task that may need a new xcl release (tasks)
- **Decision**: one `human` task checks the pinned xcl release carries the parity stage (cutting a new xcl release if the printing release went out without it), pins it, and merges with the `minor` label.
- **Rationale**: releasing and labelled merges are outside the repo; xcl-python CI builds the parity program from the pinned xcl checkout.
- **Rejected**: an agent task for the release.

## Rehydration cues

- `spektacular spec file read 20261023075328-741138ee-python-prettylog`
- `spektacular design read --data '{"source":"design","path":"printing-and-highlighting.md"}'`
- `spektacular plan file read 20261023075302-bad71237-printing-and-highlighting-python plan` (and `context`)
- `spektacular plan file read 20261023074627-cc23fc08-encode-saved-entity-during-operation plan`
- `spektacular plan file read 20261023074223-e06f835f-event-data-level-enum plan`
- Read `xcl:example/prettylog/prettylog.go`, `xcl:example/prettylog/prettylog_test.go`, `xcl:events/slog.go`.
- Read `~/go/pkg/mod/github.com/charmbracelet/log@v0.4.2/text.go` and `styles.go`.
- Read `xcl-python:xcl/aio.py`, `xcl/_poll.py`, `xcl/events.py`, `xcl/config.py`.
- Read `xcl-website:src/pages/events.mdx:332-443`.
