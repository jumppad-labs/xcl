---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Printing and highlighting for Python

Python programs that use xcl now get the same readable output as Go. An xcl error holding a configuration problem prints, in one call, as the report Go prints: severity and summary, the file, line, block and field, the numbered source lines around the fault, and the detail wrapped at 80 columns. The report also appears on its own after an uncaught error's traceback and in exceptions logged with their traceback, while `str(err)` stays the engine's message. Resources, saved records and change summaries print in one call. Output can be plain text, terminal colour, class-marked HTML a website restyles, inline-styled HTML or SVG, styled by short themes that extend the default or come from a VS Code theme, or by a format the user writes. Events reach Python's standard logging in one line, at the levels and with the messages Go's logging bridge uses.

## Why it matters

Before this change a Python program saw a configuration mistake as one line with no hint of where it was, could not style xcl's output, and could not render it for a web page or an image; only a terminal-colour flag on the encode calls existed. Python application authors, website maintainers and the downstream Python pretty logger now get Go's output through the same engine, and every format is held byte for byte to Go's: the built-in formats to the reference outputs xcl publishes, and the printed reports, resources, change summaries and event logs to Go's on every shared parity fixture.

## What was built

**xcl-python**
- Protocol stubs regenerated from the engine that sends labelled text (token kinds, `Error.report`, encode `tokens`, `OperationFinished.diff_tokens`, `ConvertTheme`). `highlight=` is removed from `encode_entity` and `encode_saved_entity` on both configuration flavours; passing it raises `TypeError`.
- A public `xcl.highlight` package: `Kind` (mirroring the protocol's numbering, checked against it both ways), `Token`, style strings, `Theme`, an immutable `DEFAULT_THEME` extended with `|`, `style_for`, `validate` (raising the new `xcl.InvalidStyleError`), the formatters `PLAIN`, `TERMINAL`, `HTML`, `HTML_INLINE` and `SVG` behind a `Formatter` protocol, `css`, `for_stream`, `load_vscode_theme` (converted by the engine; the new `xcl.InvalidThemeError`) and `LogFormatter`.
- Every `xcl.Error` keeps the engine's report as `err.tokens` and carries the plain report as an exception note. `xcl.print_error` and `xcl.format_error` print it with a format, a theme and a detail width, wrapping detail with an exact port of Go's wrapping library.
- `print_entity`, `print_saved_entity`, `entity_tokens` and `saved_entity_tokens` on `xcl.Config` and `xcl.aio.Config`; `Diff.tokens` and `xcl.print_diff`.
- `xcl.logging_handler`, an `on_event` handler mirroring Go's `events.SlogHandler`.
- The Python parity program records the printed reports, entities, change summaries and event log; the suite holds them to Go's on every fixture, including a new unsupported-block fixture.
- README and PARITY.md document printing, formats, themes, stylesheets, custom formats, logging and the migration from `highlight=`.

**xcl**
- The Go parity program (`bindings/libxcl/cmd/parity`) records `printed_validate`, `printed_apply`, `printed_diff_before`, `printed_diff_changed`, `printed_entities` and `log` (events through `events.SlogHandler`), with a new `unsupported_block` fixture whose nested block opens on line 7, documented in the fixture README.

**xcl-website**
- The configuration-text, problems, diff and events pages and the plugins and configuration-only examples show Python printing, formats, themes, stylesheet generation, custom formats and logging, and no longer say any of it is unavailable from Python.

## Deviations from the plan

- The human release task (releasing xcl with the parity change, pinning it in `xcl-ref.txt`, publishing the `minor` xcl-python release, adding the release-notes line naming the replacement for `highlight=`, and publishing the website after it) was descoped to a person after the merge, as decided for the epic; it is listed in the test plan. `xcl-ref.txt` stays at `v0.11.1`; the work was tested against an engine, stubs and parity program built from the xcl branch.
- The Go and Python parity records also hold `printed_diff_before`, so every fixture carries a printed change summary.
- `Theme` is typed as a union of kind-keyed mappings and `DEFAULT_THEME` as a `MappingProxyType`, so the documented theme idioms pass mypy under `py.typed`; mypy tests hold them.
- The logging bridge follows Go exactly for a non-string `level` or `message` detail (info, empty message) and skips a detail whose name collides with an attribute already set.
- The website's problems page gained an "In a traceback" subsection; no page was added, renamed or removed.
