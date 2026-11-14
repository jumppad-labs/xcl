---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Plan: 20261110083940-53be9068-typescript-printing

<!-- Metadata -->
<!-- Created: 2026-11-10T09:16:42+00:00 -->
<!-- Commit: 788959a6ecad23b6d101f55f74fb793d6ca9cbad -->
<!-- Branch: f-python-plugin -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Node programs using `xcl-lang` get the same readable output Go and Python have: a configuration mistake shows the lines in question wherever it ends up, entities and change summaries print in one call, output can be plain, coloured, HTML or an image, and a ready-made pretty logger shows each event and each created entity as it happens. Today a TypeScript program would see a mistake as one line, could not style xcl's output, and would have to write its own event handling. It serves developers building tools and services on the TypeScript package, and the people who read what those tools print.

## Conventions

- **Testing and mocking: never table-driven, positive and negative cases in separate tests, verbose over abstract** — every format, style word and pretty-logger behaviour gets its own test in the TypeScript suites.
- **Never write tests that inspect repository files** — README, PARITY.md and doc comments go to manual checks. Reading xcl's reference outputs is comparison against a published contract, and resolving `xcl-lang/xcl.css` exercises the package's export map, so neither is a repository-inspection test.
- **Reference outputs in `highlight/testdata/reference` are a cross-language contract** — the five formats and the stylesheet are checked against them byte for byte, and this package never regenerates them.
- **Breaking changes are allowed until xcl is stable** — the core plan's error and diff types are extended in place with no compatibility paths.
- **Prefer the standard library, document third-party dependencies, pin versions** — no new runtime dependency; pino and winston are pinned dev dependencies used only to prove logger integration.
- **Write a release plan once released work is merged** — this spec ships with the package's first release, so the implement run covers it in that plan.
- **Spec worktrees belong to the agent** — implementation is delivered to the branches of `xcl-typescript` and `xcl`.

## Architecture & Design Decisions

The work is built to `printing-and-highlighting.md`, in the shape it fixes: the engine labels the text, the package holds kinds, themes, formats and printing, and nothing lexes configuration or lays out a report in TypeScript. All of it lives in `jumppad-labs/xcl-typescript` on top of the core SDK plan, except one correction to the design document in `xcl`. `src/highlight/` is a port of `xcl-python`'s `highlight` package, held byte for byte to `xcl:highlight/testdata/reference`, so it depends on nothing above it. `src/print.ts` adds the one-call functions. `src/errors.ts`, `src/diff.ts` and `src/config.ts` gain the labelled text. `src/events.ts` gains `toLogger`, and `src/prettylog.ts` with `src/internal/prettyline.ts` is the pretty logger, shaped after `xcl-python`'s. Exports are flat from `xcl-lang` as the design's examples are, with `events` and `prettylog` as namespaces, because the design writes `events.toLogger` and `prettylog.handler` reads better than a bare `handler`.

An error's report arrives with the error, so printing one is synchronous and needs no engine. That report has three routes out, and they are deliberately different. `printError` and `formatError` choose any format. `console.error` goes through `util.inspect.custom`, which renders the report in terminal colour when Node's `options.colors` is on and plain otherwise. An uncaught error cannot use that route: Node's fatal path calls `inspect` with `customInspect: false`, read from source on Node 22 and 24 and observed on Node 26. So the plain report is also carried in the error's `stack`, between the first line and the frames, and the package adds no process listener, since a listener is a process-wide side effect. An uncaught report is therefore always plain, and the spec says so. `report` is an own enumerable accessor rather than a data property, so JSON loggers get it while Node's uncaught output shows `[Getter]` instead of a second, escaped copy. `message` stays one line: a multi-problem `ConfigError` arrives with one line per problem, so `fromProto` joins them with `; `.

The pretty logger is an `onEvent` handler that returns a Promise. The core plan's queue awaits each handler before the next, so the configuration beneath a creation line is on the stream before the next event's line, with no ordering code here. It renders the saved record on the event through `EncodeSavedEntity`, which the engine allows during an operation and which never looks up a live entity. Each event's line and everything beneath it go out in one `write`, which answers the spec's reordering risk: chunks on one stream keep their order, and a report can never separate from its line. It never awaits `drain`, so a slow stream cannot hold up an operation. The rule that each problem is reported once, beneath the first line about it, comes from `xcl-python`'s `_report_wanted`, counted per configuration because one handler may serve several. Colour, formats and reports all come from `src/highlight/` and `formatError`, as the spec requires.

Testing follows the three layers of the core plan. The formats and stylesheet are checked against the reference files with no engine, printing and the pretty logger against the fake transport, and real behaviour against a built engine, including an uncaught-error program in a fresh process on each of Node 22, 24 and 26. The parity runner gains the printing keys the core plan left out, so printed text, log records and pretty-log output are compared with Go's for every fixture. pino and winston are dev dependencies only, used to prove the structured-log and logger-bridge criteria. The alternatives weighed and rejected are in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

**New, in xcl-typescript**

- **Kinds and tokens.** The `Kind` enum mirrored by hand from the protocol, the parent arithmetic, class names and the `Token` type. Everything else in printing depends on it, and a kind a newer engine sends still falls back to its category.
- **Styles and themes.** Reads style strings, holds the fixed colour palette, the default theme, the resolution from a kind to its category and then plain text, and theme validation. Used by the formats and the stylesheet.
- **Built-in formats.** Plain, terminal, HTML with classes, HTML with inline styles and SVG, each a function of the `Formatter` type, so a user's own format is accepted anywhere one of these is.
- **Stylesheet generator.** Turns a theme into CSS for the HTML classes. A build step uses it to write the `xcl.css` the package ships.
- **Format chooser.** `forStream`: terminal colour for a terminal with `NO_COLOR` unset, plain otherwise. The only code that looks at a stream to decide on colour.
- **VS Code theme loader.** Sends a theme file to the engine for conversion and returns a `Theme`. The only printing feature that needs the engine and no configuration.
- **Detail wrapping.** A port of Go's word wrap, applied only to detail tokens.
- **Print functions.** `printError`, `formatError` and `printDiff`: one call for the common case, on top of the formats and wrapping.
- **Logger bridge.** `events.toLogger`: event to level, message and fields as Go's bridge writes them, for loggers taking fields first or message first. Its rules are shared with the pretty logger.
- **Pretty line layout.** One event as the text line Go's example writes: time, level, message, attributes and Go's value formatting. Pure, with styling supplied by its caller, so it is tested alone.
- **Pretty logger.** `prettylog.handler` and `prettylog.levelFromEnv`: decides what to write for each event, including configuration beneath a creation, a report beneath a mistake, one-time guidance and render warnings, and writes each group at once to a stream or a logger.

**Changed, in xcl-typescript**

- **Errors.** `XclError` gains `tokens`, the enumerable `report` and the report in its `stack`, and the custom inspect method that colours the report for the console. `fromProto` reads the report and makes `message` one line. A new `InvalidStyleError` is added.
- **Diff.** Carries the change summary's labelled text from the engine.
- **Config.** Gains `printEntity` and `entityTokens`, and an internal way to render a saved record as labelled text for the pretty logger.
- **Event.** Remembers, privately, the configuration that delivered it and that configuration's event data level, so the pretty logger needs no configuration passed in.
- **Parity runner.** Adds the printing keys, and the comparison stops leaving them out.
- **Package manifest and build.** Exports `xcl.css`, writes it during the build, and adds pino and winston as dev dependencies.
- **Package documentation.** README and parity record gain printing, and every new export has a doc comment.

**Changed, in the design source**

- **`printing-and-highlighting.md`.** The TypeScript section is corrected to say that an uncaught error's report comes from `stack`, as plain text.

## Data Structures & Interfaces

The shapes the design fixes are used as written: `Writer`, `Formatter`, `Theme`, `PrintOptions`, `printError`, `formatError`, `cfg.printEntity`, `cfg.entityTokens`, `printDiff`, `err.tokens`, `diff.tokens`, `forStream`, `css` and `events.toLogger`. This section gives those contracts and what the plan adds to them.

**Labelled text and formats.** The contract every format, custom or built in, is held to.

```ts
enum Kind { Text = 0, Block = 100, BlockType = 101, /* ... the design's table */ Detail = 1404 }

interface Token { readonly kind: number; readonly text: string }  // a Kind, or a number this package does not know

interface Writer { write(chunk: string): unknown }
type Theme = Readonly<Record<number, string>>;
type Formatter = (out: Writer, theme: Theme, tokens: readonly Token[]) => void;

const plain: Formatter, terminal: Formatter, html: Formatter, htmlInline: Formatter, svg: Formatter;
const defaultTheme: Theme;   // frozen; extended with spread
```

**Styles and themes.** What a theme's strings mean, and how they are checked.

```ts
interface Style { bold: boolean; italic: boolean; underline: boolean; dim: boolean; foreground: string; background: string }

function parseStyle(style: string): Style;
function styleFor(theme: Theme, kind: number): Style;
function validateTheme(theme: Theme): void;       // throws InvalidStyleError listing every unknown word
function css(theme: Theme): string;
function forStream(stream: object): Formatter;
function loadVSCodeTheme(source: string | Uint8Array): Promise<Theme>;
```

**Printing.**

```ts
interface FormatOptions { formatter?: Formatter; theme?: Theme; width?: number | null }  // width 80; 0 or null never wraps
interface PrintOptions extends FormatOptions { out?: Writer }

function printError(err: unknown, options?: PrintOptions): void;    // out is process.stderr
function formatError(err: unknown, options?: FormatOptions): string;
function printDiff(diff: Diff, options?: Omit<PrintOptions, "width">): void;  // out is process.stdout

// on Config
printEntity(entity: object, options?: EncodeOptions & Omit<PrintOptions, "width">): Promise<void>;
entityTokens(entity: object, options?: EncodeOptions): Promise<Token[]>;
```

**Errors.** Added to the core plan's `XclError`.

```ts
class XclError extends Error {
  readonly tokens: readonly Token[];  // non-enumerable; empty for an error holding no report
  readonly report?: string;           // own, enumerable accessor; present only with a report
}
class InvalidStyleError extends XclError {
  readonly words: ReadonlyArray<{ kind: number; style: string; word: string }>;
}
```

The `stack` of an error with a report is its first line, the plain report at 80 columns, then the frames.

**Diff.** `Diff` gains `readonly tokens: readonly Token[]`, filled from the engine's `diff_tokens`.

**Logger bridge.**

```ts
interface LevelLogger {
  debug(...args: unknown[]): unknown;
  info(...args: unknown[]): unknown;
  warn(...args: unknown[]): unknown;
  error(...args: unknown[]): unknown;
}
function toLogger(logger: LevelLogger, options?: { order?: "fields-first" | "message-first" }): (event: Event) => void;
```

**Pretty logger.**

```ts
type LogLevel = "debug" | "info" | "warn" | "error";

interface PrettyLogOptions {
  out?: Writer;                                 // process.stderr; give out or logger, not both
  level?: LogLevel;                             // "info"
  logger?: LevelLogger;
  order?: "fields-first" | "message-first";     // with logger
  formatter?: Formatter;                        // forStream(out); plain through a logger
}
function handler(options?: PrettyLogOptions): (event: Event) => Promise<void>;
function levelFromEnv(): LogLevel;
const LEVEL_ENV: "XCL_LOG_LEVEL";
```

**Boundaries.**

- Tokens: the protocol's `Token` messages, read in `Error.report`, `EncodeEntityResponse.tokens`, `EncodeSavedEntityResponse.tokens` and the diff response's `diff_tokens`. Nothing changes in the protocol.
- `ConvertTheme`: theme bytes in, `map<int32, string>` out, at handle 0.
- Logger fields carry Go's names: `source`, `operation`, `phase`, `resource`, `type`, `file`, `duration`, `error`, then the event's details in key order. The event itself is never a field.
- Parity record: the printing keys `printed_validate`, `printed_apply`, `printed_entities`, `printed_diff_before`, `printed_diff_changed`, `log` and `pretty_log`, in the form Go's parity program writes them.
- Package path: `xcl-lang/xcl.css`.

## Implementation Detail

A reader who knows `xcl-python`'s printing can read this package's: the module boundaries, the rules and the order of the formats follow it closely, because the output must be Go's byte for byte. The patterns below are where Node gives a different answer.

**A report has three routes, each suited to who is writing.** Asked for directly, a report is rendered in the chosen format. On the console it is rendered by the custom inspect method, using the colour option Node passes. An uncaught error is printed by Node from `stack`, so the plain report is stitched in once, when the error is built from its protocol message, and never recomputed. Everything that needs the plain form reads the same text.

**Properties chosen for who reads them.** `message` is one line for logs and comparisons. `tokens` is non-enumerable because nothing but printing wants it. `report` is an enumerable accessor so a JSON logger copies it while Node's uncaught output does not print it twice. The choice is explained in a comment at the definition, since it is not obvious from the code why one property is a getter.

**Formats are plain functions.** A format is a function, so a user's format and a built-in one are the same kind of value, and there is no class to extend. Each format writes small chunks to a `Writer`, and the string forms collect into a small in-memory writer.

**Counting as Go counts.** Anything Go measures in runes, such as wrap widths and SVG columns, is measured in code points. Iterating a string with `for...of` gives this; `.length` is never used for a column count.

**One pure line layout.** The event line is computed by a pure function taking a styling callback, so its Go-formatted values and charmbracelet quoting are tested with no stream and no engine, and the layout never contains an escape code of its own.

**One write per group.** The pretty logger assembles a line and everything beneath it into a string and writes it once. Rendering happens first, and nothing waits on the stream. Its handler catches everything, since a receiver must never affect the operation.

**Private links stay off the public shape.** An event's link back to its configuration is held in a module-private `WeakMap` shared with the configuration, rather than a property on the event, so it cannot appear in a log or a JSON dump of an event.

**The stylesheet is generated, not committed.** The build writes `xcl.css` from the default theme, so the shipped file cannot differ from what `css` returns, and a test reaches it through the package's own export path.

**Tests mirror the layers.** Formats and themes against reference files, printing and the pretty logger against the fake transport, and real-engine suites for what a program can observe. Uncaught behaviour is tested in a fresh Node process, as it can only be seen from outside. One behaviour per test, no table-driven tests, positive and negative cases apart, with each format and each reference output getting its own test.

## Dependencies

Design documents this plan was built on:

- `printing-and-highlighting.md` from the `design` design source: the TypeScript surface, the split between core and language, style syntax and logger integration. Its TypeScript section needs one correction, for the uncaught report. The spec holds no formal design reference, but its Constraints bind the work to this document.
- `typescript-sdk.md` from the `design` design source: package layout and names. No changes.

Planning:

- Spec and plan `20261031080136-264e0eab-typescript-core-sdk`: must land first. This plan extends its errors, diff, config, events, parity runner and package manifest, and uses its transport, client, test harness and CI matrix.

Runtime:

- `xcl-core` at the pinned tag: already sends tokens on errors, encodes and diffs, and answers `ConvertTheme`, so no engine change and no protocol change.
- `@bufbuild/protobuf`: unchanged and still the only runtime dependency.
- Node 22 or later: `util.inspect.custom`, `Error.stack` and `import.meta.resolve`.

Development and tests:

- `pino` and `winston`, pinned exactly as dev dependencies: prove the structured-log and logger-bridge criteria. They must never become runtime dependencies.
- xcl's `highlight/testdata/reference` and `bindings/libxcl/testdata/parity`, read from the xcl checkout: the byte-for-byte contract. No changes.
- `cmd/parity` in xcl and the `recording` and `slow` fixture plugins: the Go record and the real-engine scenarios. No changes.

Model only, not changed:

- `xcl-python`'s `highlight`, `_print`, `_wrap`, `_render`, `_logbridge`, `prettylog` and `_prettyline`.

## Testing Approach

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

## Milestones & Tasks

### Milestone 1: Output can be plain, coloured, HTML or an image, styled by themes

**What changes**: A program can take labelled text and write it in any of the five formats, or a format of its own, with output identical to Go's. It can extend the default theme, check a theme for unknown style words, load a VS Code colour theme, generate a stylesheet and link the one the package ships, and ask for a format chosen from its stream. Nothing prints an error or entity in one call yet.

**Validation point**: Every format and the stylesheet match xcl's reference outputs byte for byte, and the shipped `xcl.css` resolves through the package's export path.

#### - [x] Task: Add kinds, styles and themes
**Id:** 8def2786-366e-4864-9f3a-0f3a9a00c4cf
**Repo:** xcl-typescript
**Depends on:** none
**Execution:** agent

Add the `Kind` enum, tokens, style strings, the fixed colour palette and the default theme, with the same fallback from a kind to its category and then to plain text as Go. Validating a theme reports every style word it does not recognise, and a theme is extended with an ordinary object spread.

*Technical detail:* [context.md#task-add-kinds,-styles-and-themes](./context.md#task-add-kinds,-styles-and-themes)

**Acceptance criteria**:
- [x] Every kind has the protocol's number and name, and a kind this package does not know falls back to its category
- [x] A kind given no style takes its category's style, then plain text's
- [x] A theme giving a new style for line comments alone changes only line comments
- [x] Validating a theme holding the word `blinky` reports that word
- [x] An unknown word is skipped rather than failing

#### - [x] Task: Add the built-in formats
**Id:** 4b256612-342a-4e90-a626-40c4455ece0c
**Repo:** xcl-typescript
**Depends on:**
- 8def2786-366e-4864-9f3a-0f3a9a00c4cf — Add kinds, styles and themes
**Execution:** agent

Add plain text, terminal colour, HTML with classes, HTML with inline styles and SVG, each a function that receives the whole labelled text at once. A user's own format is the same kind of function. Each is held to xcl's published reference outputs.

*Technical detail:* [context.md#task-add-the-built-in-formats](./context.md#task-add-the-built-in-formats)

**Acceptance criteria**:
- [x] Each of the five formats is byte-for-byte equal to Go's for the report, the entity and the change summary
- [x] HTML carries Go's class names
- [x] Terminal output resets before every line break
- [x] SVG widths count characters as Go counts them, including non-ASCII text

#### - [x] Task: Generate and ship the stylesheet
**Id:** 978f288c-2905-4e00-b8bc-54c254884d51
**Repo:** xcl-typescript
**Depends on:**
- 4b256612-342a-4e90-a626-40c4455ece0c — Add the built-in formats
**Execution:** agent

Add `css`, which generates a stylesheet from a theme, and have the build write the default stylesheet into the package, exported at a published path so a site can link it.

*Technical detail:* [context.md#task-generate-and-ship-the-stylesheet](./context.md#task-generate-and-ship-the-stylesheet)

**Acceptance criteria**:
- [x] The stylesheet from the default theme is byte-for-byte equal to Go's
- [x] A theme changing one kind's style differs from the default only in that kind's rule
- [x] `xcl-lang/xcl.css` resolves through the package's exports and holds the same text

#### - [x] Task: Choose a format from the stream and load VS Code themes
**Id:** d5433aa6-08ce-410d-92ca-1002b09519a2
**Repo:** xcl-typescript
**Depends on:**
- 4b256612-342a-4e90-a626-40c4455ece0c — Add the built-in formats
**Execution:** agent

Add `forStream`, which picks terminal colour for a terminal unless `NO_COLOR` is set and plain text otherwise, and `loadVSCodeTheme`, which has the engine convert a colour theme into a theme.

*Technical detail:* [context.md#task-choose-a-format-from-the-stream-and-load-vs-code-themes](./context.md#task-choose-a-format-from-the-stream-and-load-vs-code-themes)

**Acceptance criteria**:
- [x] A terminal stream gets terminal colour, a terminal stream with `NO_COLOR` set gets plain text, and a non-terminal stream gets plain text
- [x] Loading a VS Code colour theme gives a theme that renders a keyword in the file's keyword colour
- [x] A file that is not a colour theme throws the invalid-theme error

### Milestone 2: Errors, entities and change summaries print in one call

**What changes**: A configuration mistake shows the lines in question wherever it ends up: printed in one call, as a string, passed to `console.error`, left uncaught, or logged as JSON through pino or winston with no setup. Entities and change summaries print in one call each, with the same formatting choices. An error's message stays one line, and detail wraps at 80 columns unless asked otherwise.

**Validation point**: A real bad configuration prints Go's report byte for byte, an uncaught one shows the report on Node 22, 24 and 26, JSON logs carry each problem, and the design document describes what was built.

#### - [x] Task: Carry the report on errors
**Id:** 2e5b235a-3ab4-4ef9-81d9-d1284e23ce49
**Repo:** xcl-typescript
**Depends on:**
- 4b256612-342a-4e90-a626-40c4455ece0c — Add the built-in formats
**Execution:** agent

Give `XclError` the engine's report as labelled text and as plain text, and show it on the console with colour following Node's option. Put the plain report in the error's `stack`, between the first line and the frames, because Node's uncaught-error printing does not use custom inspection. Keep `message` to one line, and add wrapping of report detail to a width, as Go wraps it.

*Technical detail:* [context.md#task-carry-the-report-on-errors](./context.md#task-carry-the-report-on-errors)

**Acceptance criteria**:
- [x] An error's `message` holds no line break, including one with several problems
- [x] `console.error` shows the report, in colour only when Node's option asks for it
- [x] An error's `stack` holds the plain report between its first line and its frames
- [x] `report` and `problems` are enumerable, and `report` is present only when there is a report
- [x] An error with no report inspects as Node's default does
- [x] Detail wraps as Go wraps it at 80, at a chosen width and with no wrapping

#### - [x] Task: Print errors, entities and change summaries
**Id:** 7ce78661-4aff-42fd-974b-88ca23de6cb3
**Repo:** xcl-typescript
**Depends on:**
- 2e5b235a-3ab4-4ef9-81d9-d1284e23ce49 — Carry the report on errors
**Execution:** agent

Add `printError`, `formatError`, `printDiff`, `cfg.printEntity` and `cfg.entityTokens`, and give a diff its labelled text. Every option has a plain default, so nothing is coloured unless asked. Anything that is not an xcl error still prints, without throwing.

*Technical detail:* [context.md#task-print-errors,-entities-and-change-summaries](./context.md#task-print-errors,-entities-and-change-summaries)

**Acceptance criteria**:
- [x] Printing an xcl error writes its report in the chosen format, and getting it as a string returns exactly what printing writes
- [x] A plain error prints its message, and any other value prints its text form, without throwing
- [x] Printing with no format chosen writes no colour on a terminal
- [x] Printing an entity gives Go's text under the four encoding choices, and a change summary gives Go's text
- [x] A user's own format receives the whole labelled text in one call for errors, entities and change summaries
- [x] Each of the three labelled texts concatenates to the plain text of that output

#### - [x] Task: Test printed and uncaught errors against a real engine
**Id:** be7b8438-8077-435c-832f-a0ad86115573
**Repo:** xcl-typescript
**Depends on:**
- 7ce78661-4aff-42fd-974b-88ca23de6cb3 — Print errors, entities and change summaries
**Execution:** agent

Add real-engine tests for a configuration with an unsupported block: its report, how it appears on the console and when uncaught on every supported Node version, and what pino and winston log with no configuration. This is the first time the uncaught route runs on Node 22 and 24, and where pino and winston carrying `problems` and `report` is confirmed.

*Technical detail:* [context.md#task-test-printed-and-uncaught-errors-against-a-real-engine](./context.md#task-test-printed-and-uncaught-errors-against-a-real-engine)

**Acceptance criteria**:
- [x] The report for an unsupported block on line 7 equals Go's byte for byte
- [x] A program that lets the error go uncaught writes the report to standard error and exits with a failure status, on Node 22, 24 and 26
- [x] A rejected top-level await shows the report the same way
- [x] JSON from pino's standard error serializer and from winston's error format holds each problem's file, line, column, summary, entity and field, and the plain report, with no other configuration
- [x] pino and winston appear only as development dependencies

#### - [x] Task: Correct the printing design document
**Id:** 14cc6528-63d2-43d8-b93b-965a21064bf1
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Change the TypeScript section of `printing-and-highlighting.md` to say that `console.error` uses custom inspection, while an uncaught error's report comes from the error's `stack` as plain text, with no process listener. The sentence leaving this to be confirmed is removed.

*Technical detail:* [context.md#task-correct-the-printing-design-document](./context.md#task-correct-the-printing-design-document)

**Acceptance criteria**:
- [x] The design's TypeScript section describes the console route and the uncaught route as built
- [x] The design no longer says that an uncaught error goes through custom inspection, or leaves it to be confirmed

### Milestone 3: A one-line logger shows what xcl is doing

**What changes**: A program sends xcl's events to its own logger in one line, or turns on the pretty logger in one line: each event a readable line, each created entity's configuration beneath its line as it happens, and each configuration mistake's report beneath its error. It follows the terminal for colour and can read its level from `XCL_LOG_LEVEL`, and it never holds up an operation.

**Validation point**: The pretty-logger scenarios pass against a real engine at each level, and event-loop delay stays under the target during a slow apply with it on.

#### - [x] Task: Send events to an existing logger
**Id:** f5073bc8-600a-4daf-8352-c9d448708a4b
**Repo:** xcl-typescript
**Depends on:**
- be7b8438-8077-435c-832f-a0ad86115573 — Test printed and uncaught errors against a real engine
**Execution:** agent

Add `events.toLogger`, which writes each event to any logger with the four level methods, at the levels and with the messages Go's logging bridge uses. Loggers taking fields then message, and message then fields, are both supported.

*Technical detail:* [context.md#task-send-events-to-an-existing-logger](./context.md#task-send-events-to-an-existing-logger)

**Acceptance criteria**:
- [x] A log event is written at its own level with its own message, and any other event as `<operation> <phase>`, at error for `error`, warn for `blocked` and info otherwise
- [x] Each record's fields hold the event's details under Go's names, and never the event object itself
- [x] With a pino and a winston logger set up in one line, an apply produces one record per event

#### - [x] Task: Lay out event lines
**Id:** fe387bfd-f1aa-418f-8364-371c2ba23ad1
**Repo:** xcl-typescript
**Depends on:**
- 8def2786-366e-4864-9f3a-0f3a9a00c4cf — Add kinds, styles and themes
**Execution:** agent

Add the pure function that writes one event as Go's example writes it: time, level, message and attributes, with Go's value formatting and quoting. The caller supplies the styling, so the layout never contains an escape code.

*Technical detail:* [context.md#task-lay-out-event-lines](./context.md#task-lay-out-event-lines)

**Acceptance criteria**:
- [x] A line has the time, level label, message and attributes in Go's order
- [x] Numbers, durations, lists and maps are written as Go writes them
- [x] A value needing quotes is quoted and escaped as Go's example does, and a multi-line value is indented
- [x] Source, operation and entity are styled, and the other parts as Go's example styles them

#### - [x] Task: Add the pretty logger
**Id:** 8d60434e-bf52-46d4-b4e4-2a9e170ece23
**Repo:** xcl-typescript
**Depends on:**
- 7ce78661-4aff-42fd-974b-88ca23de6cb3 — Print errors, entities and change summaries
- f5073bc8-600a-4daf-8352-c9d448708a4b — Send events to an existing logger
- fe387bfd-f1aa-418f-8364-371c2ba23ad1 — Lay out event lines
- d5433aa6-08ce-410d-92ca-1002b09519a2 — Choose a format from the stream and load VS Code themes
**Execution:** agent

Add `prettylog.handler` and `prettylog.levelFromEnv`. Each event at or above the level is a readable line, with a created entity's configuration or a mistake's report beneath it, written at once to a stream or through a logger. A failed render warns and carries on, and nothing it does can hold up or fail an operation.

*Technical detail:* [context.md#task-add-the-pretty-logger](./context.md#task-add-the-pretty-logger)

**Acceptance criteria**:
- [x] Configuration appears beneath a creation line at info and below, and none at higher levels
- [x] A configuration mistake's report appears beneath its error line at every level, once per problem
- [x] With no entity data on events, the guidance is written exactly once per logger
- [x] A render failure writes one warning line and the following entities are still shown
- [x] Variables, outputs and modules show nothing and write no warning
- [x] `XCL_LOG_LEVEL` is read only when asked for, in any letter case, and an unknown word means info
- [x] Colour follows the stream, and a failing stream never affects the operation

#### - [x] Task: Test the pretty logger against a real engine
**Id:** 7befb73c-c2c2-4f50-b398-61e73afbe116
**Repo:** xcl-typescript
**Depends on:**
- 8d60434e-bf52-46d4-b4e4-2a9e170ece23 — Add the pretty logger
**Execution:** agent

Add real-engine tests of every pretty-logger scenario in the spec: one-line setup, each level, a configuration mistake, colour, routing through a logger, and an event-loop delay check during a slow apply.

*Technical detail:* [context.md#task-test-the-pretty-logger-against-a-real-engine](./context.md#task-test-the-pretty-logger-against-a-real-engine)

**Acceptance criteria**:
- [x] A run creating one entity writes one line per event at info or above, with the configuration, including a value a provider filled in, before the next event's line
- [x] The same run through an application's logger writes the same lines
- [x] At warn there is no configuration and no info line, and at debug debug events appear
- [x] An unsupported block writes the error line followed by the report
- [x] A terminal stream has colour codes, and `NO_COLOR` or a captured stream has none
- [x] Two entities with no entity data write the guidance once and no configuration or warning
- [x] A configuration with a variable, an output and a module writes no configuration and no warning for them
- [x] During a slow apply with the pretty logger on, a timer's 99th-percentile delay stays under 50 ms

### Milestone 4: Printing is proven against Go and documented

**What changes**: Every parity fixture's printed reports, entities, change summaries, logged events and pretty-log output are compared with Go's, so a difference fails the suite. The README and parity record cover printing, and every new export has a doc comment.

**Validation point**: The parity suite passes with no printing keys left out.

#### - [x] Task: Compare printing with Go on the parity fixtures
**Id:** ee37549d-2913-49d6-bec0-2feb2817b377
**Repo:** xcl-typescript
**Depends on:**
- 7ce78661-4aff-42fd-974b-88ca23de6cb3 — Print errors, entities and change summaries
- 8d60434e-bf52-46d4-b4e4-2a9e170ece23 — Add the pretty logger
**Execution:** agent

Add the printing keys the core plan left out to the parity runner, and stop leaving them out of the comparison. A difference from Go in a report, printed entity, change summary, logged event or pretty-log line now fails the suite.

*Technical detail:* [context.md#task-compare-printing-with-go-on-the-parity-fixtures](./context.md#task-compare-printing-with-go-on-the-parity-fixtures)

**Acceptance criteria**:
- [x] For every fixture, the Node and Go records are equal with no keys left out
- [x] A difference names the fixture and the first path that differs

#### - [x] Task: Document printing and the pretty logger
**Id:** e046e730-d0cb-4488-8ddb-a8a4ce5e6ed3
**Repo:** xcl-typescript
**Depends on:**
- 8d60434e-bf52-46d4-b4e4-2a9e170ece23 — Add the pretty logger
- 978f288c-2905-4e00-b8bc-54c254884d51 — Generate and ship the stylesheet
**Execution:** agent

Extend the README and the parity record to cover error printing, how reports appear on uncaught and logged errors, output formats, themes, stylesheet generation, custom formats, logger integration and the pretty logger, with TypeScript examples, and give every new public export a doc comment.

*Technical detail:* [context.md#task-document-printing-and-the-pretty-logger](./context.md#task-document-printing-and-the-pretty-logger)

**Acceptance criteria**:
- [x] The README covers every topic named above, with TypeScript examples
- [x] The parity record maps each Go printing and logging item to its TypeScript form
- [x] Every new public export has a doc comment

## Open Questions

- **Do Node 22 and 24 print a report held in `stack` as Node 26 does?** Node's fatal path was read from source for 22 and 24 and run only on 26, because those binaries could not be fetched while planning. It depends on running the uncaught-report test on each version in CI. If it fails on either, STOP and ask the user: the spec rules out a process listener.
- **Do pino's standard serializer and winston's `format.errors()` carry an own enumerable accessor?** Node's `JSON.stringify` and `Object.assign` do, but the two loggers could not be installed while planning. It depends on the tests with the dev dependencies. If either drops `report`, STOP and ask the user: a data property works but prints a second, escaped copy of the report in uncaught output.

## Out of Scope

- The TypeScript SDK reference and guide on xcl.dev, and TypeScript tabs on the site: the epic's `typescript-sdk-reference` spec.
- Using the formatters in a browser: the package runs on Node.
- New output formats or themes beyond those Go and Python have.
- Any change to what the engine's labelled text contains, or to the protocol.
- Any change to the Go library or the Python package; in xcl only the design document changes.
- Colour in Node's uncaught-error output: a report there is plain text, by decision.
- A process listener of any kind, including `uncaughtExceptionMonitor`.
- `printError` following an error's `cause` chain.
- Public `printSavedEntity` and `savedEntityTokens`, which Go has no form of and the design does not list.
- A `LogFormatter` equivalent: Node's loggers have no shared formatter class, and `toLogger` plus the error's enumerable fields cover it.

## Changelog

### 2026-11-10 — Task: Add kinds, styles and themes

**What was done**: Added the `Kind` enum, `Token`, style strings and the colour palette, the frozen `defaultTheme`, and `validateTheme` with `InvalidStyleError`, in `src/highlight/`, ported from the Python package and exported from `xcl-lang`.

**Deviations**: None

**Files changed**:
- `xcl-typescript: src/highlight/kinds.ts`
- `xcl-typescript: src/highlight/style.ts`
- `xcl-typescript: src/highlight/theme.ts`
- `xcl-typescript: src/highlight/validate.ts`
- `xcl-typescript: src/errors.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/highlight-kinds.test.ts`
- `xcl-typescript: test/unit/highlight-style.test.ts`
- `xcl-typescript: test/unit/highlight-theme.test.ts`

**Discoveries**: `theme.ts` must not import `errors.ts`, so `errors.ts` can import the formats without a cycle; `validate.ts` is the one module of `highlight/` that reaches for the errors.

### 2026-11-10 — Task: Add the built-in formats

**What was done**: Added plain, terminal, HTML with classes, inline-styled HTML and SVG as functions of the `Formatter` type, each held byte for byte to xcl's reference outputs for the report, the entity and the change summary.

**Deviations**: The core plan's xcl-source helper did not exist, so `test/helpers/xcl-source.ts` was added to find the reference outputs through `XCL_SRC` and the sibling checkout, as the parity runner finds its fixtures.

**Files changed**:
- `xcl-typescript: src/highlight/formatters.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/helpers/xcl-source.ts`
- `xcl-typescript: test/unit/highlight-reference.test.ts`
- `xcl-typescript: test/unit/highlight-formatters.test.ts`

**Discoveries**: None

### 2026-11-10 — Task: Generate and ship the stylesheet

**What was done**: Added `css`, which generates a stylesheet from a theme, and a build step that writes `dist/xcl.css` from the compiled package, exported as `xcl-lang/xcl.css`.

**Deviations**: `npm run test:unit` builds the package itself, so `make test-unit` and a bare `npm run test:unit` both find the shipped file; the Makefile changed only by a comment. The plan had the Makefile's targets build first.

**Files changed**:
- `xcl-typescript: src/highlight/css.ts`
- `xcl-typescript: scripts/write-css.mjs`
- `xcl-typescript: package.json`
- `xcl-typescript: Makefile`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/highlight-css.test.ts`

**Discoveries**: A package can resolve its own name through its `exports` map, so `import.meta.resolve("xcl-lang/xcl.css")` works from the package's own tests with no install step.

### 2026-11-10 — Task: Choose a format from the stream and load VS Code themes

**What was done**: Added `forStream`, which picks terminal colour for a terminal unless `NO_COLOR` is set and plain text otherwise, and `loadVSCodeTheme`, which has the engine convert a theme at handle 0 through the shared client.

**Deviations**: The shared client was private to `config.ts`, so `sharedEngineClient` was added there for the loader to use.

**Files changed**:
- `xcl-typescript: src/highlight/stream.ts`
- `xcl-typescript: src/highlight/vscode.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/highlight-stream.test.ts`
- `xcl-typescript: test/integration/vscode-theme.test.ts`

**Discoveries**: No kind is literally a keyword. A VS Code `keyword` rule reaches `Kind.Operator` through its `keyword.operator.xcl` scope, and `Kind.BlockType` is `storage.type.xcl`.

### 2026-11-10 — Task: Carry the report on errors

**What was done**: `XclError` gained `tokens`, the own enumerable accessor `report`, the plain report in `stack` and a custom inspect method that colours the report only when Node asks. `fromProto` makes `message` one line, and `src/internal/wrap.ts` and `render.ts` wrap detail as Go does.

**Deviations**: None

**Files changed**:
- `xcl-typescript: src/errors.ts`
- `xcl-typescript: src/internal/wrap.ts`
- `xcl-typescript: src/internal/render.ts`
- `xcl-typescript: test/unit/errors-report.test.ts`
- `xcl-typescript: test/unit/wrap.test.ts`

**Discoveries**: `oneLine` trims each line, so a multi-line engine message reads as `a; b`. The custom inspect looks for the report only after the first line, because the one-line message can hold the same text as a short report.

### 2026-11-10 — Task: Print errors, entities and change summaries

**What was done**: Added `printError`, `formatError`, `printDiff`, `cfg.printEntity` and `cfg.entityTokens`, and gave a diff its labelled text.

**Deviations**: A diff's tokens are read in `src/internal/poll.ts`, where the finished task is, rather than in `src/config.ts`.

**Files changed**:
- `xcl-typescript: src/print.ts`
- `xcl-typescript: src/diff.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/internal/poll.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/print.test.ts`
- `xcl-typescript: test/integration/print.test.ts`

**Discoveries**: None

### 2026-11-10 — Task: Test printed and uncaught errors against a real engine

**What was done**: Added real-engine tests for an unsupported block: the report byte for byte, `console.error` and an uncaught error in a fresh process for a throw and a rejected top-level await, and pino and winston JSON logs. pino and winston are pinned dev dependencies.

**Deviations**: None. Both open questions resolved in favour of the plan: the uncaught route prints the report in `stack` on Node 22, 24 and 26 (run locally on 26 and in Podman on `node:22-slim` and `node:24-slim`), and pino's serializer and winston's `format.errors()` both carry the own enumerable `report` accessor.

**Files changed**:
- `xcl-typescript: package.json`
- `xcl-typescript: package-lock.json`
- `xcl-typescript: test/integration/fixtures/unsupported-block/main.xcl`
- `xcl-typescript: test/integration/scripts/uncaught-throw.ts`
- `xcl-typescript: test/integration/scripts/uncaught-await.ts`
- `xcl-typescript: test/integration/error-report.test.ts`
- `xcl-typescript: test/integration/json-logs.test.ts`

**Discoveries**: Node 22's test runner uses the TAP reporter when output is piped, so a grep for `✔` needs `--test-reporter=spec`. An engine and plugins built on the host run unchanged in the `node:*-slim` containers, which is a quick way to try another Node version.

### 2026-11-11 — Task: Correct the printing design document

**What was done**: The TypeScript part of "Errors and loggers" in `printing-and-highlighting.md` now says that `console.error` uses custom inspection with Node's colour option, while an uncaught error's report comes from `stack` as plain text, with no process listener. The sentence leaving it to be confirmed is gone.

**Deviations**: This run does not write design documents, so the orchestrator applied the wording this run proposed, after the user approved it.

**Amendments**: design `printing-and-highlighting.md` (source `design`), section "Errors and loggers", TypeScript, first paragraph: corrected to the console route and the uncaught route as built. The orchestrator recorded it with `spektacular spec amend` and added the design reference to the spec.

**Files changed**:
- `xcl: .spektacular/design/printing-and-highlighting.md`

**Discoveries**: None

### 2026-11-11 — Task: Send events to an existing logger

**What was done**: Added `events.toLogger`, which writes each event to any logger with the four level methods at Go's bridge's level and message, fields first by default or message first. The bridge's rules live in `src/internal/logbridge.ts`, ready for the pretty logger.

**Deviations**: A detail whose key matches one of Go's named fields does not overwrite that field. The plan said only that no reserved-name handling is done.

**Files changed**:
- `xcl-typescript: src/internal/logbridge.ts`
- `xcl-typescript: src/events.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/logbridge.test.ts`
- `xcl-typescript: test/integration/logger-bridge.test.ts`

**Discoveries**: A real apply delivers plugin log events (phase `log`, such as "starting plugin") alongside lifecycle events, and an entity's `resourceType` is `slow.one` (type and name), not `slow`.

### 2026-11-11 — Task: Lay out event lines

**What was done**: Added `src/internal/prettyline.ts`, a pure port of the line layout of Go's example pretty logger: kitchen time, level label, message and attributes, with Go's value formatting, durations, quoting, escaping and multi-line indentation. Styling comes from a callback, so the layout writes no escape code of its own.

**Deviations**: A whole number is written as an integer only within the safe integer range. Beyond it, such as `1.5e300`, it is written in Go's float form rather than as a long run of digits. The plan said every number `Number.isInteger` accepts. An event's error is written as its one-line `message`, so a multi-line Go error text reads as one line joined with `; `, where Go's example would indent it over several lines.

**Files changed**:
- `xcl-typescript: src/internal/prettyline.ts`
- `xcl-typescript: test/unit/prettyline.test.ts`

**Discoveries**: Each test file runs in its own process under `node --test`, so setting `process.env.TZ = "UTC"` at the top of one file pins its kitchen times without touching the others.

### 2026-11-11 — Task: Add the pretty logger

**What was done**: Added `prettylog.handler` and `prettylog.levelFromEnv`, shaped after xcl-python's pretty logger. A created entity's configuration or a mistake's report is written beneath its line in one write, through a stream or a logger, with guidance once and one warning per failed render. Each event links privately, through a WeakMap, to the configuration delivering it, which renders saved records with their computed values.

**Deviations**: The link is set inside `ConfigImpl`, which wraps the configuration's `onEvent`, and the saved-record render is a private method of the configuration. Bad options throw `InvalidArgumentError`, as `createConfig` does for its own.

**Files changed**:
- `xcl-typescript: src/prettylog.ts`
- `xcl-typescript: src/internal/event-link.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/prettylog.test.ts`

**Discoveries**: A variable's creation event carries no saved record, so with event data off the guidance can be written at the variable's creation rather than at the first resource's.

### 2026-11-11 — Task: Test the pretty logger against a real engine

**What was done**: Added real-engine tests of every pretty-logger scenario with the recording fixture plugin, covering one-line setup, the configuration with a provider's value before the next line, the same lines through a logger, warn and debug, `XCL_LOG_LEVEL`, an unsupported block, colour on a fake terminal, `NO_COLOR`, guidance once, and variables, outputs and modules. Also added an event-loop delay check during a slow apply with the pretty logger on.

**Deviations**: None

**Files changed**:
- `xcl-typescript: test/integration/prettylog.test.ts`
- `xcl-typescript: test/integration/event-loop.test.ts`

**Discoveries**: None

### 2026-11-11 — Task: Compare printing with Go on the parity fixtures

**What was done**: The parity runner now writes the printing keys Go's program writes: printed validate and apply reports, printed entities, printed change summaries, every event through `events.toLogger`, and pretty-log groups for a fixture whose apply succeeds. The comparison no longer leaves any key out. Every fixture's records are equal to Go's, with and without typed lookups.

**Deviations**: None. No fixture's error message needed collapsing in the runner: the printed records already agree, because an error with a report prints its report.

**Files changed**:
- `xcl-typescript: test/parity/run.ts`
- `xcl-typescript: test/parity/parity.test.ts`
- `xcl-typescript: test/parity/normaliser.test.ts`

**Discoveries**: None

### 2026-11-11 — Task: Document printing and the pretty logger

**What was done**: The README gained sections on printing errors (including uncaught and logged errors), entities and change summaries, formats and themes, stylesheets, custom formats, logging and the pretty logger, each with TypeScript examples. The parity record's printing and logging tables now map every Go printing and logging export to its TypeScript form, or say why there is none. Every new public export has a doc comment.

**Deviations**: The first goal check found Go's `Kinds()`, `Kind.String()`, `Kind.Scope()`, `Style.IsZero()`, `Colour` and `Colour.Hex()` unmapped. They are now in the parity record, the last two not-applicable items with their reasons.

**Files changed**:
- `xcl-typescript: README.md`
- `xcl-typescript: PARITY.md`
- `xcl-typescript: src/prettylog.ts`

**Discoveries**: None
