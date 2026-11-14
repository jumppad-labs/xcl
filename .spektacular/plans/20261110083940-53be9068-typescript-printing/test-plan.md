---
created_date: "2026-11-11"
document_status: final
closed_date: "2026-11-11"
---

# Test plan: 20261110083940-53be9068-typescript-printing

These are the checks the automated suites cannot make. Everything else, including the uncaught report on Node 22, 24 and 26, is covered by `npm test` in `xcl-typescript` with `XCL_TEST_STRICT=1`, as CI runs it.

Setup for the checks that run code: in a checkout of `jumppad-labs/xcl-typescript` with an `xcl` checkout beside it, run `npm ci`, then `make engine plugins` and `npm run build`, and `export XCL_CORE_PATH=$PWD/build/engine/xcl-core`. Some examples below use `build/plugins/xcl-plugin-recording` and the fixture `test/integration/fixtures/unsupported-block/main.xcl`.

## Success metric: no colour codes in output the user did not ask to colour

- **What to measure**: open issues on `jumppad-labs/xcl-typescript` reporting escape codes (`\x1b[`) in output where the user chose no colour format and no format from the stream.
- **How**: after the release that ships printing, search the issue tracker for "colour", "color", "escape", "ANSI" and `\x1b` once a week for the first month, then at each release. The behaviour itself is covered by tests: printing with no format on a terminal, a captured stream, `NO_COLOR`, and an uncaught error all write no escape codes.
- **Expected result**: zero open issues of this kind. Any such report is a bug against the rule that colour is chosen only by the user, by `forStream` or by turning on the pretty logger.
- **Who / when**: the package maintainer, from the first release that includes printing.

## Review: README coverage

- **What to look at**: `README.md` in `xcl-typescript`, the sections "Printing errors", "Printing entities and change summaries", "Formats and themes", "Stylesheets", "Custom formats", "Logging" and "Pretty logger".
- **What to look for**: each of error printing, how reports appear on uncaught and logged errors (`console.error`, uncaught, pino and winston), output formats, themes (including `validateTheme` and `loadVSCodeTheme`), stylesheet generation and the shipped `xcl-lang/xcl.css`, custom formats, logger integration (`events.toLogger`) and the pretty logger is explained with a TypeScript example. Every example compiles: paste each into a scratch `.ts` file importing from `xcl-lang` and run `npx tsc --noEmit`.
- **Passing**: every topic has prose and a TypeScript example, and no example has a type error.
- **Who / when**: a reviewer other than the implementer, before release.

## Review: parity record coverage

- **What to look at**: the "Printing", "Logging" and "Not applicable" sections of `PARITY.md` in `xcl-typescript`, beside the Go API: `print.go` and `diagnostics.go` at the root of `xcl`, `highlight/*.go`, `events/slog.go` and `example/prettylog/prettylog.go`.
- **What to look for**: every exported Go printing, highlighting and logging identifier has a row giving its TypeScript form, or a not-applicable row with a reason.
- **Passing**: no exported Go item in those files is missing from the record.
- **Who / when**: a reviewer, before release, and again whenever the bundled xcl version changes.

## Review: doc comments on new exports

- **What to look at**: `src/index.ts`, and for each printing and logging name it exports, its declaration in `src/highlight/*.ts`, `src/print.ts`, `src/prettylog.ts`, `src/events.ts`, `src/errors.ts` (`InvalidStyleError`, `XclError.tokens`, `XclError.report`), `src/config.ts` (`printEntity`, `entityTokens`) and `src/diff.ts` (`Diff.tokens`).
- **What to look for**: a `/** ... */` comment above each that says what it is and how it is used. The comment should show up as hover text in an editor on `dist/index.d.ts`.
- **Passing**: every new public export has a doc comment.
- **Who / when**: a reviewer, before release.

## Review: colour in a real terminal

- **What to look at**: three runs in a real terminal (for example a recent macOS Terminal or iTerm2, GNOME Terminal and Windows Terminal), with `NO_COLOR` unset:
  1. A pretty-logged apply: a script calling `createConfig({ registries: [local], statePath: "/tmp/pl-state", eventData: EventDataLevel.Processed, onEvent: prettylog.handler() })`, with `local.registerExternalPlugin("build/plugins/xcl-plugin-recording")`, then `apply` on a file holding `resource "recording" "one" { log = "/tmp/pl.jsonl" value = "a" }`.
  2. `console.error(err)` for the `ConfigError` from applying `test/integration/fixtures/unsupported-block/main.xcl` with the same registry.
  3. The same apply left uncaught: `await config.apply("test/integration/fixtures/unsupported-block/main.xcl")` at the top level, with no `try`.
- **What to look for**: in run 1, the level labels, source, operation and entity are coloured, and the configuration beneath the creation line is highlighted and readable on both a dark and a light background. In run 2, the report's `Error`, summary and gutter are coloured. In run 3, the report appears between the first line and the stack frames as plain text, with no escape codes visible.
- **Passing**: runs 1 and 2 are coloured and readable, and run 3 shows the plain report and exits with status 1.
- **Who / when**: a reviewer, before release, on at least one terminal per operating system the CI matrix covers.

## Review: a site linking the shipped stylesheet

- **What to look at**: an HTML page that links `node_modules/xcl-lang/dist/xcl.css` (the file `xcl-lang/xcl.css` resolves to) and holds the output of `formatError(err, { formatter: html })` for the unsupported-block error, and of `config.printEntity(entity, { formatter: html, out })` for an entity.
- **What to look for**: block types bold magenta, attributes blue, strings yellow, comments italic grey, the faulty span bold red and the gutter dimmed, as the terminal shows them.
- **Passing**: the page renders xcl's colours in a current browser, with no unstyled spans.
- **Who / when**: a reviewer, before release.
