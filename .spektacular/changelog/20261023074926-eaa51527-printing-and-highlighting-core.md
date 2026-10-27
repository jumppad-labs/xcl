---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Printing and highlighting core

## What was built

xcl's Go library now prints any error, resource or change summary readably in one call. Output can be plain text, terminal colour, class-marked HTML a website restyles, inline-styled HTML or SVG, styled by short, extendable themes.

- **One-call printing (xcl).** `xcl.PrintError`, `xcl.PrintEntity` and `xcl.PrintDiff` write to any `io.Writer`, configured by `WithFormatter`, `WithTheme` and `WithWidth`. Every `EncodeOption` is also a `PrintOption`.
  - `PrintError` finds every configuration problem inside an error, however it was wrapped, and prints each one as a report: severity and summary, location (file, line, block, field), numbered source lines with the fault marked, and the detail wrapped at 80 columns. Any other error prints as its message.
  - Extenders get the labelled text of each output from `xcl.ErrorTokens`, `xcl.EntityTokens`, `diff.(*Diff).Tokens` and `highlight.Tokenize`.
- **Context restored (xcl).** A `Diagnostic` keeps up to two lines either side of the fault, clipped at the file's ends, from the source already in memory.
  - The core service's error details carry the wider source too.
  - `*Diagnostic` and `*ConfigError` implement `fmt.Formatter`. `%+v` gives the plain report, which zap records in `errorVerbose`. `%v` and `Error()` stay one line.
- **Single report layout (xcl).** The report's wording lives in one private package, `internal/report`. The `errors` package uses it for `%+v`, and the root package labels the same segments, so the two outputs always agree. This also avoids the `errors`↔`highlight` import cycle.
- **Rebuilt `highlight` package (xcl):**
  - Numbered `Kind`s that fall back to their category, with `Class()` and `Scope()`.
  - `Token`, and `Tokenize` labelling text exactly as before (the grammar-parity tests are unchanged).
  - `Theme` as a map of style strings, with `With`, `Validate` (`ErrInvalidStyle`) and `DefaultTheme`.
  - `LoadVSCodeTheme`.
  - The `Formatter` interface with `Plain`, `Terminal`, `HTML`, `HTMLInline` and `SVG`.
  - `WriteCSS`, and `For` (colour on request, honouring `NO_COLOR`).
  - The previous API was removed outright: `Piece`, `Pieces`, `Renderer`, `RendererFunc`, `PieceFunc`, the `Text` renderer, `ANSI`, `ANSIRenderer`, `NewANSI`, `ANSIOption`, `WithTheme`, `WithThemeFile`, the `Scope*` constants, `xcl.DiagnosticPieces` and `diff.(*Diff).Pieces`. Every caller in the repository was moved over.
- **Reference outputs (xcl).** `highlight/testdata/reference/` publishes three token-stream inputs, each one's exact output in every built-in format, the default stylesheet and the class list. The Go formatters are tested against these files byte for byte, and other languages check their own formatters against the same files.
- **Core service and examples (xcl):**
  - The core service's legacy `highlight=true` formats through `Terminal`.
  - The examples print resources and change summaries through `PrintEntity` and `PrintDiff`.
  - The examples print errors through `PrintError`, so a broken configuration shows a report. Each example has a test that runs it against a broken configuration.
- **Release notes and docs (xcl).** `CHANGELOG.md` gives a one-line replacement for every removed function. `docs/` and `README.md` describe the new printing.
- **Website (xcl-website).** Seven existing pages were rewritten in place, with nothing renamed or removed. They document printing errors, resources and change summaries, output formats, themes, the HTML class names, stylesheet generation, custom formats and logging integration, each with a Go example.

## Why it matters

Showing a configuration problem with its source used to take several little-known steps and printed only the faulty line. It now takes one call and shows the surrounding lines again. The examples demonstrate this: none prints a configuration error as a single line.

Output is no longer shaped around one editor's conventions. Websites restyle it with their own CSS, a theme changes one kind of text in one line, and whole documents such as SVG images can be produced.

The published reference outputs let every other xcl language match Go's printing exactly, without running the engine.

## Deviations from the plan

- `report.Walk` descends into an `Unwrap() []error` collection only when a problem lies beneath it, the same rule as for single wraps.
- `coreservice/errors_test.go` also asserted the captured source lines and was updated.
- Two root test files the plan gave to "Move every remaining caller" were ported earlier, in "Add one-call printing", so the root tests could run there.
- The plugin example's broken-configuration smoke test runs `plan`, which needs Docker. The remoteplugin test uses the existing fake GitHub release.
- The examples no longer prefix a non-configuration error with `error:`. This is recorded in the Breaking list.
- On the website, Python tabs keep their current form and say richer printing in Python is coming. The site stylesheet was not changed.
- The pages describe logger behaviour more precisely than the plan did:
  - slog's text handler formats an error value with `%+v`.
  - zap's `errorVerbose` holds the report only for the unwrapped error.
  - `%v` on a `ConfigError` gives one line per problem.
- Codebase research for the two High tasks was done in the main context. Their tests were written by sub-agents.
