---
created_date: "2026-10-26"
document_status: draft
project: xclconfig
spec: 20261023074926-eaa51527-printing-and-highlighting-core
plan: 20261023074926-eaa51527-printing-and-highlighting-core
---

# Printing errors, resources and change summaries in one call

Go programs now print any error, resource or change summary readably in one call. Output can be plain text, terminal colour, HTML a website restyles, inline-styled HTML or SVG, styled by short themes that extend the default.

A configuration problem prints as a report showing its location and the numbered lines around the fault. The two surrounding lines on each side are restored. Structured loggers such as zap record that report automatically.

The previous highlighting and report rendering API is removed, and each removed function has a one-line replacement in the release notes.

> Derived from project xcl (file), spec/plan 20261023074926-eaa51527-printing-and-highlighting-core. See the project-level record for the full feature.

## What changed in this repo

- **Printing.**
  - `print.go` adds `PrintError`, `PrintEntity` and `PrintDiff` with the `WithFormatter`, `WithTheme` and `WithWidth` options. `EncodeOption` is also a `PrintOption`.
  - `diagnostics.go` replaces `DiagnosticPieces` with `ErrorTokens`, which walks the whole error tree.
  - `encode.go` adds `EntityTokens`. `config.go` re-exports `ErrInvalidStyle` and `InvalidStyleError`.
- **Errors.**
  - `errors.NewDiagnostic` captures up to two context lines either side of the fault.
  - `errors/format.go` implements `fmt.Formatter` on `*Diagnostic` and `*ConfigError`.
  - `errors/highlight_errors.go` adds `ErrInvalidStyle` and `InvalidStyleError`.
- **Report layout.** The new private `internal/report` package holds the single report layout, detail wrapping and the error-tree walk.
- **`highlight`, rebuilt.**
  - `kind.go`, `tokenize.go` (`Token`, `Tokenize`), `style.go`, `theme.go`, `vscode.go` (`LoadVSCodeTheme`).
  - `formatter.go`, `terminal.go`, `html.go`, `svg.go`, `css.go` (`WriteCSS`), `for.go`, and a new `doc.go`.
  - `highlight.go` and `ansi.go` are deleted.
- **`diff`.** `(*Diff).Tokens()` replaces `Pieces()`. The text is unchanged.
- **Core service.** `highlight=true` formats through `highlight.Terminal`.
- **Examples.**
  - The plugin example's `inspect` and `plan`, and `prettylog`, print through `PrintEntity` and `PrintDiff`.
  - `configonly`, `plugin` and `remoteplugin` print errors through `PrintError`. Each gains a `testdata/broken` configuration and a test that runs the program against it.
- **Reference outputs.** `highlight/testdata/reference/` holds the inputs, the per-format outputs, `default.css`, `classes.txt` and a README. `highlight/reference_test.go` checks them byte for byte, and `-update` regenerates them.
- **Captured output.** `testdata/print/` holds the plain output captured before the refactor. New tests hold `PrintEntity` and `PrintDiff` to it.
- **Tests.** The parser, viper and root tests were moved to the new API.
- **Dependencies.** `go.uber.org/zap` v1.27.0 is a test-only dependency. It is used only to prove that `errorVerbose` carries the report.
- **Docs.** `CHANGELOG.md`, `docs/overview.md`, `docs/README.md` and `README.md` are updated.

## Why

The core library is where lexing, report layout and the reference formatters live. Every other xcl language and the website build on this repo's API and on its published reference outputs.

The old API printed errors as one line and was shaped around VS Code scopes and `[]byte` renderers. It could not restyle output for a website or produce whole documents.
