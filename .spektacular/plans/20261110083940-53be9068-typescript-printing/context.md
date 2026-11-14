---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Context: 20261110083940-53be9068-typescript-printing

## Current State Analysis

`xcl-typescript` holds only LICENSE, a two-line README and its `.spektacular` footprint, so every file here builds on the core SDK plan `20261031080136-264e0eab-typescript-core-sdk`, which must land first and whose `src/errors.ts`, `src/diff.ts`, `src/config.ts`, `src/events.ts`, `test/parity/` and `package.json` this plan extends. The behaviour to match is `xcl-python`'s at engine pin `v0.13.1`: `xcl/highlight/`, `xcl/_print.py`, `xcl/_render.py`, `xcl/_wrap.py`, `xcl/_logbridge.py`, `xcl/prettylog.py`, `xcl/_prettyline.py`. The engine already sends tokens on `Error.report`, `EncodeEntityResponse`, `EncodeSavedEntityResponse` and the diff response, and answers `ConvertTheme` (`xcl:proto/xcl/core/v1/core.proto`), so nothing changes in `xcl` except one design document.

Requirements by repo. Everything in the spec is carried out in `xcl-typescript`: formats and themes in `src/highlight/`, one-call printing in `src/print.ts`, errors in `src/errors.ts`, the logger bridge in `src/events.ts` and `src/internal/logbridge.ts`, the pretty logger in `src/prettylog.ts` and `src/internal/prettyline.ts`, documentation in `README.md` and `PARITY.md`. The one exception is the design correction, in `xcl`. `xcl-python` and `xcl` code are read only.

## Per-Task Technical Notes

### Task: Add kinds, styles and themes

**File changes**:
- `xcl-typescript:src/highlight/kinds.ts` (new): numeric `enum Kind`, `Token`, `parentOf` (`Math.floor(kind / 100) * 100`), `cssClassOf`, kebab case from the enum's reverse mapping, `tokensFromProto`, from `xcl-python:xcl/highlight/_kinds.py:11-156`.
- `xcl-typescript:src/highlight/style.ts` (new): `Style`, `parseStyle`, `hexColour`, palette and `sgr`, from `xcl-python:xcl/highlight/_style.py:1-154`, with the same SGR order.
- `xcl-typescript:src/highlight/theme.ts` (new): `Theme`, frozen `defaultTheme`, `styleFor`, from `xcl-python:xcl/highlight/_theme.py:21-64`. It imports nothing from `errors.ts`, so errors can import the formats without a cycle.
- `xcl-typescript:src/highlight/validate.ts` (new): `validateTheme`, walking kinds in ascending order then words.
- `xcl-typescript:src/errors.ts`: `InvalidStyleError`, `words`, code `invalid_style`, a one-line message joined with `; ` (`xcl-python:xcl/_errors.py:428-446` is multi-line and is not copied there).
- `xcl-typescript:src/index.ts`: exports.
- `xcl-typescript:test/unit/highlight-kinds.test.ts`, `test/unit/highlight-style.test.ts`, `test/unit/highlight-theme.test.ts` (new). The kind test walks the generated `TokenKind`, numbers and upper-snake names.

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Add the built-in formats

**File changes**:
- `xcl-typescript:src/highlight/formatters.ts` (new): `Writer`, `Formatter`, `plain`, `terminal`, `html`, `htmlInline`, `svg`, `cssDeclarations`, from `xcl-python:xcl/highlight/_formatters.py:28-269`. SVG columns count code points with `[...text].length`, as `xcl:highlight/svg.go:33` counts runes.
- `xcl-typescript:src/index.ts`: exports.
- `xcl-typescript:test/unit/highlight-reference.test.ts` (new): one test per input and format, as `xcl-python:tests/unit/test_highlight_reference.py`, reading `highlight/testdata/reference` through the core plan's xcl-source helper, with CRLF turned back to LF.
- `xcl-typescript:test/unit/highlight-formatters.test.ts` (new): reset before line breaks, an empty token, an unknown kind, non-ASCII SVG widths, `'` escaped as `&#39;`.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Generate and ship the stylesheet

**File changes**:
- `xcl-typescript:src/highlight/css.ts` (new): `css`, from `xcl-python:xcl/highlight/_css.py:11-61`, kinds in ascending numeric order.
- `xcl-typescript:scripts/write-css.mjs` (new): after `tsc`, imports `dist/index.js` and writes `dist/xcl.css`.
- `xcl-typescript:package.json`: `exports` gains `"./xcl.css": "./dist/xcl.css"`; `build` runs the script.
- `xcl-typescript:Makefile`: `test` and `test-unit` build first, so the shipped file exists.
- `xcl-typescript:src/index.ts`: export.
- `xcl-typescript:test/unit/highlight-css.test.ts` (new): against `default.css` and `classes.txt`, one kind changed, and the file reached with `import.meta.resolve("xcl-lang/xcl.css")`.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Choose a format from the stream and load VS Code themes

**File changes**:
- `xcl-typescript:src/highlight/stream.ts` (new): `forStream` on `stream.isTTY === true` and `NO_COLOR` unset or empty, from `xcl-python:xcl/highlight/_stream.py:10-31`.
- `xcl-typescript:src/highlight/vscode.ts` (new): `loadVSCodeTheme`, `ConvertTheme` at handle 0 through the core plan's shared client, as `xcl-python:xcl/highlight/_vscode.py:10-41`. A path is read with `fs/promises`. No configuration exists to close, so the engine stays running but `unref`ed and never holds the process open.
- `xcl-typescript:src/index.ts`: exports.
- `xcl-typescript:test/unit/highlight-stream.test.ts`, `test/integration/vscode-theme.test.ts` (new).

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Carry the report on errors

**File changes**:
- `xcl-typescript:src/internal/wrap.ts` (new): port of `xcl-python:xcl/_wrap.py:1-78`, iterating code points, with the same explicit space set.
- `xcl-typescript:src/internal/render.ts` (new): `wrapped`, `render`, `plainReport`, from `xcl-python:xcl/_render.py:14-34`.
- `xcl-typescript:src/errors.ts`: in `fromProto`, tokens from `Error.report`, `message` lines joined with `; ` (`xcl:errors/config_error.go:42-50`). `attachReport(err, tokens)` defines `tokens` non-enumerable and `report` as an own enumerable accessor, and rewrites `stack` as `Name: message`, a line break, the plain report and the frames from the first `\n    at`; with no frames the report goes at the end. `[util.inspect.custom](depth, options)` returns `this` when there is no report, otherwise the stack with the report rendered by `terminal` if `options.colors`. A comment at the accessor says why it is one.
- `xcl-typescript:test/unit/errors-report.test.ts`, `test/unit/wrap.test.ts` (new): one behaviour each; wrap cases from `xcl-python:tests/unit/test_wrap.py`.

**Complexity**: High
**Token estimate**: ~22k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Print errors, entities and change summaries

**File changes**:
- `xcl-typescript:src/print.ts` (new): `printError`, `formatError`, `printDiff`, from `xcl-python:xcl/_print.py:16-102`. The text of a non-error is `String(value)`, falling back to `util.inspect` if that throws. `printDiff` throws `TypeError` for a diff with no tokens. `process.stderr` and `process.stdout` are read at call time.
- `xcl-typescript:src/diff.ts`: `tokens`.
- `xcl-typescript:src/config.ts`: `diff` passes `diff_tokens`; `printEntity`, `entityTokens` with `tokens: true`.
- `xcl-typescript:src/index.ts`: exports.
- `xcl-typescript:test/unit/print.test.ts` (new): from `xcl-python:tests/unit/test_print.py`, against the fake transport.
- `xcl-typescript:test/integration/print.test.ts` (new): an entity under the four encoding choices and a change summary against the Go text, and labelled text joining to plain text.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Test printed and uncaught errors against a real engine

**File changes**:
- `xcl-typescript:package.json`: `pino`, `winston` in `devDependencies`, pinned exactly.
- `xcl-typescript:test/integration/fixtures/unsupported-block/main.xcl` (new): nine lines with the block on line 7, as in `xcl-python:tests/integration/test_prettylog_engine.py`.
- `xcl-typescript:test/integration/scripts/uncaught-throw.ts`, `test/integration/scripts/uncaught-await.ts` (new): programs with no printing code, compiled by `tsconfig.test.json`.
- `xcl-typescript:test/integration/error-report.test.ts` (new): the report compared with the expected text written out in the test (the parity suite's `unsupported_block` fixture holds the same path to Go's record), `console.error` in a fresh process, and the two uncaught programs through `test/helpers/script.ts` with `process.execPath`, so CI's Node 22, 24 and 26 matrix covers each.
- `xcl-typescript:test/integration/json-logs.test.ts` (new): pino's standard serializer and winston's `format.errors()` to in-memory destinations. If an own enumerable accessor is not carried, STOP and ask.

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Correct the printing design document

**File changes**:
- `xcl:.spektacular/design/printing-and-highlighting.md`: edited only through the `spek-design` skill or `spektacular design` commands, in the "Errors and loggers" TypeScript section: the first paragraph, and removal of "To be confirmed while implementing".

**Complexity**: Low
**Token estimate**: ~3k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Send events to an existing logger

**File changes**:
- `xcl-typescript:src/internal/logbridge.ts` (new): `eventLevel`, `eventMessage`, `eventFields`, from `xcl-python:xcl/_logbridge.py:830-874` and `xcl:events/slog.go:41-101`, with no reserved-name handling and no event object.
- `xcl-typescript:src/events.ts`: `toLogger`, default `fields-first`.
- `xcl-typescript:src/index.ts`: `export * as events`, alongside the existing direct exports.
- `xcl-typescript:test/unit/logbridge.test.ts` (new): one test per phase and level, each argument order, a fake logger.
- `xcl-typescript:test/integration/logger-bridge.test.ts` (new): pino and winston, one record per event.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Lay out event lines

**File changes**:
- `xcl-typescript:src/internal/prettyline.ts` (new): `formatLine`, `formatRecord`, `kitchen`, `levelLabel`, `STYLES`, `goValue`, `goFloat`, `goDuration`, quoting and escaping, from `xcl-python:xcl/_prettyline.py:1-300`. Printability is `/[\p{L}\p{M}\p{N}\p{P}\p{S}]/u` or the ASCII space. A number that `Number.isInteger` accepts is written as an integer, since JavaScript cannot tell `2` from `2.0`.
- `xcl-typescript:test/unit/prettyline.test.ts` (new): from `xcl-python:tests/unit/test_prettyline.py`.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Add the pretty logger

**File changes**:
- `xcl-typescript:src/prettylog.ts` (new): `handler`, `levelFromEnv`, `LEVEL_ENV`, from `xcl-python:xcl/prettylog.py:340-550`. The problems shown are counted in a `WeakMap` by configuration; `_report_wanted`'s rule is unchanged. Guidance reads `create the configuration with eventData: EventDataLevel.Processed`. Everything is in a `try`/`catch`.
- `xcl-typescript:src/internal/event-link.ts` (new): a `WeakMap` from event to `{ eventData, savedEntityTokens }`.
- `xcl-typescript:src/config.ts`: sets the link when delivering an event; internal `savedEntityTokens`, with `tokens: true` and `includeComputed: true`, as `xcl-python:xcl/aio.py:400-425`.
- `xcl-typescript:src/index.ts`: `export * as prettylog`.
- `xcl-typescript:test/unit/prettylog.test.ts` (new): from `xcl-python:tests/unit/test_prettylog.py`, against the fake transport, including a scripted render failure.

**Note**: Check the core plan's `Event` for the unit of `duration`. `goDuration` needs whatever it provides, and must not lose precision the parity normalisation relies on.

**Complexity**: High
**Token estimate**: ~35k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Test the pretty logger against a real engine

**File changes**:
- `xcl-typescript:test/integration/prettylog.test.ts` (new): scenarios from `xcl-python:tests/integration/test_prettylog_engine.py`, with the `recording` fixture plugin and a fake terminal stream with `isTTY: true`. An event line matches `^\d{1,2}:\d{2}(AM|PM) (DEBU|INFO|WARN|ERRO) `.
- `xcl-typescript:test/integration/event-loop.test.ts`: a pretty-logger case with `perf_hooks.monitorEventLoopDelay` and `xcl-plugin-slow`.

**Complexity**: Medium
**Token estimate**: ~22k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Compare printing with Go on the parity fixtures

**File changes**:
- `xcl-typescript:test/parity/run.ts`: the printing stages, as `xcl-python:tests/parity/run_python.py:169-201,219-241,320-336`, a recording logger through `events.toLogger`, and `Normaliser` gains `printedError`, `printedDiff`, `log` and `prettyGroups` (`:518-598`).
- `xcl-typescript:test/parity/parity.test.ts`: the comparison no longer leaves out the printing keys.
- `xcl-typescript:test/parity/normaliser.test.ts`: the new normalisers.

**Note**: If a fixture's non-diagnostic error message has a line break, collapse Go's record the same way, in the runner only.

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Document printing and the pretty logger

**File changes**:
- `xcl-typescript:README.md`: printing, logging and pretty logger sections.
- `xcl-typescript:PARITY.md`: fill the printing rows the core plan marked for this spec, from `xcl-python:PARITY.md:58-103`.
- `xcl-typescript:src/highlight/`, `xcl-typescript:src/print.ts`, `xcl-typescript:src/prettylog.ts`, `xcl-typescript:src/events.ts`, `xcl-typescript:src/errors.ts`, `xcl-typescript:src/config.ts`: doc comments on every new export.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution

## Testing Strategy

Per-task detail: each task's tests are named in its file changes above. Reference, unit and parity tests need an xcl checkout through the core plan's helper; real-engine suites need `make engine plugins` and `XCL_CORE_PATH`. `XCL_TEST_STRICT=1`, as CI always sets, fails instead of skipping when something is missing. The uncaught-report tests start a fresh process with `process.execPath`, so the Node version under test is the one CI's matrix selects.

Tests follow the core plan's layers and the project's rules: Node's built-in runner compiled with `tsc`, one behaviour per test, no table-driven tests, positive and negative cases in separate tests, verbose over abstract.

**Reference tests, no engine.** Each of plain, terminal, HTML, inline HTML and SVG is checked against xcl's published reference output for each of the report, the entity and the change summary, byte for byte, with one test per pair. The stylesheet is checked against `default.css` and the class names against `classes.txt`. This is where most of the risk lies, so it is what the contract covers.

**Unit tests, no engine.** They guarantee that:

- kinds match the protocol's numbers and names, and an unknown kind falls back to its category;
- style words parse, a later word wins, and an unknown word is skipped by formats and reported by validation;
- a theme changing one kind changes only that kind, in output and in the stylesheet;
- wrapping matches Go at the default width, a chosen width and with no wrapping, including a word longer than the width and non-ASCII text;
- `forStream` picks by terminal and `NO_COLOR`, and printing with no format writes no colour on a terminal;
- a plain error and a non-error value print without throwing, including a value whose string conversion throws;
- `formatError` returns exactly what `printError` writes;
- a user's format receives the whole text in one call;
- an error's message has no line break, and an error with no report inspects as Node's default;
- the custom inspect method colours the report only when Node's option asks;
- the logger bridge gives Go's level and message for each phase, in both argument orders;
- the line layout writes Go's values, durations, quoting and multi-line values;
- the pretty logger writes configuration beneath a creation, a report beneath a mistake once per problem, guidance once, one warning per failed render, nothing for variables, outputs and modules, nothing but events above info, and swallows a failing stream.

**Integration tests, real engine.** A real apply drives what a program observes: a report byte for byte equal to Go's for an unsupported block on line 7; `console.error` and an uncaught error each showing the report, the second in a fresh Node process checking standard error and a failure status, for a throw and for a rejected top-level await; JSON from pino's standard serializer and winston's error format holding each problem's fields and the plain report with no configuration; events through a pino and a winston logger, one record per event; and every pretty-logger criterion on a captured stream, a fake terminal stream and through a logger. The uncaught and console tests run on Node 22, 24 and 26 through the existing CI matrix. This is required: the uncaught route was run on Node 26 only during planning, and read from source for 22 and 24.

**Parity tests.** The parity runner adds the printing keys, so each fixture's printed reports, printed entities, printed change summaries, logged events and pretty-log groups are compared with Go's program.

**An operation is not slowed.** Node's event-loop delay monitor is read during a slow apply with the pretty logger on, and its 99th percentile asserted under 50 ms.

**Deliberate gaps.** No test reads the README, the parity record, doc comments or workflow files, as the project's rules require. Terminal rendering by a real terminal is not tested: colour is asserted by the escape codes written to a fake terminal stream. Windows consoles are covered only as far as the CI matrix runs the same assertions.

**Success metrics.**

- A TypeScript developer can read a configuration mistake's file, line and faulty source from an uncaught error with no printing code of their own: behavioural test. A program with no printing code lets the error go uncaught in a fresh process, and its standard error is asserted to hold the file, the line and the faulty source, on each supported Node version.
- No open issue reports colour codes in output the user did not ask to colour: **Manual — captured in the implementation test plan** (issue tracker watched after release). The behaviour itself is covered by the tests that printing with no format, a captured stream, and `NO_COLOR` write no escape codes.

**Manual reviews.**

- **Manual — captured in the implementation test plan**: the README covers error printing, how reports appear on uncaught and logged errors, output formats, themes, stylesheet generation, custom formats, logger integration and the pretty logger, with TypeScript examples.
- **Manual — captured in the implementation test plan**: the parity record covers the same topics, with each Go printing and logging item mapped to its TypeScript form.
- **Manual — captured in the implementation test plan**: every new public export has a doc comment.
- **Manual — captured in the implementation test plan**: a real terminal shows a pretty-logged apply and a `console.error` report in readable colour, and an uncaught error's report as plain text.
- **Manual — captured in the implementation test plan**: a site linking the shipped `xcl.css` and showing HTML output renders xcl's colours.

## Project References

- Spec `20261110083940-53be9068-typescript-printing`, epic `20261031080131-8c2a58ec-typescript-sdk`; plan `20261031080136-264e0eab-typescript-core-sdk`.
- Designs from source `design`: `printing-and-highlighting.md`, `typescript-sdk.md`.
- Knowledge, store xcl: `architecture/reference-outputs-are-a-contract.md`, `gotchas/style-strings-cannot-reset.md`, `conventions/testing-and-mocking.md`, `conventions/no-compatibility-before-stable.md`.
- Knowledge, store xcl-python: `gotchas/error-event-report-once.md`, `gotchas/aio-events-out-of-order.md`.
- Repo roots: `xcl-typescript` at /home/nicj/code/github.com/jumppad-labs/xcl-typescript, `xcl` at /home/nicj/code/github.com/jumppad-labs/xcl, `xcl-python` at /home/nicj/code/github.com/jumppad-labs/xcl-python (read only, the model).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

None. These are additions to a package not yet released. `XclError`'s `stack` changes for errors holding a report, which nothing depends on yet.

## Performance Considerations

With the pretty logger on, event-loop delay must stay under 50 ms at the 99th percentile during an apply, asserted by a test. The logger renders one saved record per created entity, a single engine round trip of tens of microseconds, and never awaits the stream. Printing an error needs no engine call.
