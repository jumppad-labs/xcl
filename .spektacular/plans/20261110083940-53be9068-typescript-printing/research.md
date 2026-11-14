---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Research: 20261110083940-53be9068-typescript-printing

## Alternatives considered and rejected

- **`util.inspect.custom` alone for the uncaught report.** What the design and the first spec draft named. Node's fatal-exception path calls `inspect` with `customInspect: false`, so the method is never called. Read in `lib/internal/errors.js` `fatalExceptionStackEnhancers.afterInspector` on v22.x and v24.x; observed by running v26.13.0 for a sync throw, a rejected promise and a top-level await.
- **`process.on('uncaughtExceptionMonitor')` at import.** Fires for all three routes on Node 26 and could colour the report. Rejected by the user's decision through the orchestrator: installing a listener is a process-wide side effect, the same reason the core spec refuses a SIGINT listener. It would also print when a program's own handler recovers.
- **Colouring the text in `stack` at construction, based on `process.stderr.isTTY`.** `stack` is fixed when the error is built, so escape codes would reach JSON logs and files, against the rule that the package never emits colour unasked.
- **A data property for `report`.** Node's uncaught output then prints it a second time as an escaped, quoted, concatenated string (seen on Node 26). An own enumerable accessor prints as `[Getter]` and still appears in `JSON.stringify` and `Object.assign`.
- **Rendering built-in formats in the engine.** Rejected by the design ("Built-in formatters are native"): custom and built-in formats share one interface and themes never cross the wire.
- **Stream-level queueing or awaiting `drain` in the pretty logger.** Would make an operation wait on a slow stream, against the spec. One `write` per group keeps a line and what is beneath it together, and chunks on one stream keep their order.
- **A bare `String(value)` in `printError`.** Throws for `Object.create(null)` and similar; the spec says printing must not throw. Falls back to `util.inspect`.
- **Porting `textwrap`-like wrapping or JS `.length` for widths.** `xcl-python:xcl/_wrap.py:1-10` records that go-wordwrap must be ported exactly; columns and SVG widths count code points (`xcl:highlight/svg.go:33` uses `utf8.RuneCountInString`), not UTF-16 units.
- **A separate entry point for printing or the pretty logger.** The spec requires the pretty logger in `xcl-lang`, and `typescript-sdk.md` has examples importing names from `xcl-lang` directly.

## Chosen approach — evidence

- `printing-and-highlighting.md` fixes the TypeScript surface: `printError`, `formatError`, `cfg.printEntity`, `cfg.entityTokens`, `printDiff`, `err.tokens`, `diff.tokens`, `Formatter`, `Writer`, `forStream`, `css`, `events.toLogger`.
- The report travels on every `Error` (`xcl:proto/xcl/core/v1/core.proto:79-86`), so printing an error needs no engine call.
- `xcl-python:xcl/highlight/*` is a line-by-line port target for kinds, styles, themes, formats and stylesheet; `xcl-python:tests/unit/test_highlight_reference.py` shows the byte-for-byte method against `xcl:highlight/testdata/reference`.
- `xcl-python:xcl/prettylog.py:406-539` and `xcl/_prettyline.py` are the pretty logger model, including the report-once rule of `_report_wanted`.
- `xcl-python:xcl/_logbridge.py:830-874` and `xcl:events/slog.go:41-101` give levels, messages and field order for the logger bridge.
- Node 26 run: a report placed between the first line and the frames prints intact on an uncaught throw and a top-level await; on a terminal only frames and properties get colour, on a pipe nothing does.
- `xcl:errors/config_error.go:42-50`: a `ConfigError`'s text joins its errors with line breaks, so `message` must be made one line in the SDK; `typescript-sdk.md` already says `message` is one line.
- `xcl-python:tests/parity/run_python.py:434-470`: the parity record reports diagnostics by their structured fields, so a one-line `message` does not change it.
- `xcl:events/events.go:12-17`: `level` and `message` are reserved for log events, and the bridge leaves them out of the fields there, so they cannot collide with pino's or winston's own keys.

## Files examined

- `xcl-typescript:` — LICENSE, README and `.spektacular` only; every source file comes from the core SDK plan.
- `xcl-python:xcl/_print.py:1-102` — print and format pair, non-xcl errors as message plus newline, diff without tokens raises.
- `xcl-python:xcl/_render.py:14-34` — wrap only Detail tokens; note text is plain, 80 columns, trailing newlines removed.
- `xcl-python:xcl/_wrap.py:1-78` — port of go-wordwrap, explicit Unicode space set, no-break space never breaks.
- `xcl-python:xcl/highlight/_kinds.py:11-156` — Kind numbers, parent arithmetic, class names, `Kind(1199)` naming.
- `xcl-python:xcl/highlight/_style.py:1-154` — style words, palette, SGR order.
- `xcl-python:xcl/highlight/_theme.py:21-41` — default theme and fallback; a style is taken whole.
- `xcl-python:xcl/highlight/_formatters.py:28-269` — the five formats, escaping, SVG geometry.
- `xcl-python:xcl/highlight/_css.py:11-61` — rule order and resets.
- `xcl-python:xcl/highlight/_stream.py:10-31` — NO_COLOR empty counts as unset.
- `xcl-python:xcl/highlight/_vscode.py:10-41` — ConvertTheme at handle 0.
- `xcl-python:xcl/_errors.py:428-446` — `InvalidStyleError` words; its message is multi-line and must not be copied.
- `xcl-python:xcl/events.py:44-105` — event fields and the private configuration reference.
- `xcl-python:xcl/aio.py:400-425` — saved-record render during an operation.
- `xcl-python:tests/parity/run_python.py:169-241,320-336,518-531,564-598` — stages and normalisation for the printing keys.
- `xcl-python:tests/integration/test_prettylog_engine.py:1-70` — scenarios, event-line regex.
- `xcl-python:tests/unit/test_print.py` — behaviours to carry over.
- `xcl:highlight/testdata/reference/README.md` — inputs and outputs of the contract.
- `xcl:bindings/libxcl/testdata/parity` — fixtures including `invalid` and `unsupported_block`.
- `xcl:coreservice/errors.go:43` — `invalid_theme` is an engine code, so the core plan's classes already cover it.

## External references

- Node.js `lib/internal/errors.js` on branches v22.x and v24.x — `customInspect: false` in the fatal path. **Node 22 and 24 were read from source only**: downloading those binaries to run the probe was refused by the permission system. Only v26.13.0 was run. The implementation's uncaught-report test runs on all three.
- Node.js `util.inspect.custom` — returning the object itself falls through to default formatting (to be confirmed by a unit test).
- pino `err` serializer and winston `format.errors()` — both read own enumerable properties, from memory of their source; installing them to check was refused. Confirmed in implementation with the dev dependencies.
- no-color.org — the NO_COLOR convention.

## Prior plans / specs consulted

- `20261031080136-264e0eab-typescript-core-sdk` plan, context — layout, Transport, Client, poll queue, Event, parity runner, test helpers, and the printing keys left for this spec.
- Knowledge: `xcl` `architecture/reference-outputs-are-a-contract.md`, `gotchas/style-strings-cannot-reset.md`, `conventions/testing-and-mocking.md`; `xcl-python` `gotchas/error-event-report-once.md`, `gotchas/aio-events-out-of-order.md`.

## Open assumptions

- pino and winston carry an own enumerable accessor named `report` with no configuration. If not, STOP and ask: the fallback is a data property, which prints a second copy in uncaught output.
- Node 22 and 24 print the report held in `stack` as Node 26 does. If the test fails there, STOP and ask.
- A fixture's non-diagnostic engine message holds no line break. If one does, the parity runner collapses Go's record the same way, in the test code only.
- Returning the error itself from the custom inspect method gives Node's default output.

## Drafting assumptions

### Uncaught report is carried in `err.stack` as plain text (discovery)
- **Decision**: the plain report sits in the error's `stack`, between the first line and the frames, with no process listener. `console.error` keeps `util.inspect.custom` with Node's colour option. An uncaught report is always plain.
- **Rationale**: Node's fatal path calls `inspect` with `customInspect: false`. Chosen by the orchestrator on the user's recorded decision against process-wide listeners; the spec's Constraints and Technical Approach were amended to match.
- **Rejected**: an `uncaughtExceptionMonitor` listener at import, a side effect on the whole process; leaving uncaught errors to Node's default, which shows only an escaped string.

### `report` is an own enumerable accessor (discovery)
- **Decision**: `report` is defined with a getter, `problems` stays a data property as in the core plan.
- **Rationale**: a data property prints the report a second time, escaped, in uncaught output. An accessor still reaches JSON and `Object.assign`.
- **Rejected**: a data property. pino and winston are confirmed in implementation.

### Multi-line engine messages are collapsed to one line (discovery)
- **Decision**: `fromProto` joins a message's lines with `; `. The package's own `InvalidStyleError` does the same.
- **Rationale**: Go's `ConfigError` text has one line per problem and the spec requires a one-line `message`.
- **Rejected**: keeping only the first line, which loses problems from a plain log.

### `printError` does not walk `cause` (discovery)
- **Decision**: only an `XclError` itself prints a report; anything else prints its message or text form.
- **Rationale**: the spec's acceptance criteria and Python's behaviour; the TypeScript design does not ask for a chain walk.
- **Rejected**: following `cause`, as Go follows `%w`.

### Chosen direction: port the Python formatters, async pretty logger on the event queue (architecture)
- **Decision**: `src/highlight/` ports `xcl-python`'s package; the pretty logger is an async `onEvent` handler rendering saved records and writing each group in one `write`.
- **Rationale**: the design fixes the split and the spec names Python as the model. The core plan's queue already orders handlers.
- **Rejected**: buffering lines until a render finishes, which adds ordering code the queue makes unnecessary.

### Flat exports with `events` and `prettylog` namespaces (architecture)
- **Decision**: `plain`, `terminal`, `html`, `htmlInline`, `svg`, `Kind`, `Token`, `Theme`, `defaultTheme`, `css`, `forStream`, `validateTheme`, `loadVSCodeTheme`, `parseStyle`, `printError`, `formatError`, `printDiff` are top-level; `events.toLogger` and `prettylog.handler` are namespaced.
- **Rationale**: the design's TypeScript examples use bare names and `events.toLogger`.
- **Rejected**: a `highlight` namespace, which the design's TypeScript examples do not use.

### Pretty logger takes one options object (architecture)
- **Decision**: `prettylog.handler({ out, level, logger, order, formatter })`, levels as `"debug" | "info" | "warn" | "error"`.
- **Rationale**: the design has no TypeScript section for it, and an options object matches `PrintOptions`.
- **Rejected**: Python's positional `file, level`.

### Conventions selected (architecture)
- **Decision**: testing rules, no repository-inspection tests, reference outputs as contract, no compatibility before stable, dependencies, release plan, worktrees.
- **Rationale**: those are the ones this work touches.
- **Rejected**: the Go-only, database, state-fixture and DAG conventions, which do not bear on a TypeScript printing library.

### Saved-record printing stays internal (data_structures)
- **Decision**: no public `printSavedEntity` or `savedEntityTokens`; the pretty logger uses an internal render.
- **Rationale**: the spec and the design's TypeScript section list only entities, errors and diffs, and Go has no such forms.
- **Rejected**: copying Python's two extra methods.

### Theme and logger names (data_structures)
- **Decision**: `validateTheme`, `styleFor`, `parseStyle`, `loadVSCodeTheme`, `LevelLogger`, order values `fields-first` and `message-first`.
- **Rationale**: the design names only `message-first`; the rest follow camelCase forms of Python's and Go's names.
- **Rejected**: a method on a theme object, since a theme is plain data.

### JavaScript numbers print as integers when whole (tasks)
- **Decision**: in event lines a whole number is written as an integer.
- **Rationale**: JavaScript has one number type, and Python's integer path is the common case.
- **Rejected**: Go's float form for every number, which would write `1e+06` for one million.

### Report on `stack` is built once in `fromProto` (tasks)
- **Decision**: `attachReport` runs after the error is built, so the first line carries the subclass name and one-line message.
- **Rationale**: the name is set after `super`, and V8 captures the stack header at construction.
- **Rejected**: stitching inside the constructor.

## Rehydration cues

- `spektacular spec file read 20261110083940-53be9068-typescript-printing`
- `spektacular plan file read 20261031080136-264e0eab-typescript-core-sdk plan` and `context`
- `spektacular design read --data '{"source":"design","path":"printing-and-highlighting.md"}'` and `typescript-sdk.md`
- Read `xcl-python:xcl/highlight/`, `xcl/_print.py`, `xcl/prettylog.py`, `xcl/_prettyline.py`, `xcl/_logbridge.py`, `xcl/_wrap.py`
- Read `xcl:highlight/testdata/reference/README.md`
