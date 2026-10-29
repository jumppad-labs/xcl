---
tags: [highlight, golden, parity, testing, python]
---

# Reference outputs in highlight/testdata/reference are a cross-language contract

`highlight/testdata/reference/` holds three labelled token streams as JSON
(`report`, `entity`, `diff`), each one's exact output in every built-in format
(Plain, Terminal, HTML, HTMLInline, SVG) with `DefaultTheme`, the stylesheet
`WriteCSS(DefaultTheme)` writes, and `classes.txt` (every HTML class name).
Other languages' formatters (Python first) are checked byte for byte against
these files with no engine involved, so they are a public contract, not
ordinary golden data.

- Any change to a formatter's output, the default theme, the style resolution
  or a kind's class name changes these files. Regenerate them deliberately with
  `go test ./highlight/ -run TestReference -update` and review the diff as an
  API change; a sibling language must follow it.
- The inputs are built from real code in `highlight/reference_test.go`
  (`xcl.ErrorTokens` over `errors.NewDiagnostic` on fixed in-memory source,
  `highlight.Tokenize` of `inputs/entity.xcl`, a literal `diff.Diff`), never
  from an applied configuration, so no file path leaks into the outputs.
- The token-input tests come first in the file: Go runs a file's tests in
  source order, so on an `-update` run the JSON is rewritten before the format
  tests read it back. Keep that order.
