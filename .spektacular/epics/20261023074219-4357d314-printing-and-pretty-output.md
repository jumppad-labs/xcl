---
created_date: "2026-10-23"
document_status: draft
specs:
    - name: 20261023074223-e06f835f-event-data-level-enum
      depends_on: []
    - name: 20261023074627-cc23fc08-encode-saved-entity-during-operation
      depends_on:
        - 20261023074223-e06f835f-event-data-level-enum
    - name: 20261023074926-eaa51527-printing-and-highlighting-core
      depends_on:
        - 20261023074223-e06f835f-event-data-level-enum
        - 20261023074627-cc23fc08-encode-saved-entity-during-operation
    - name: 20261023075136-42c51a7d-printing-tokens-over-the-engine
      depends_on:
        - 20261023074926-eaa51527-printing-and-highlighting-core
    - name: 20261023075302-bad71237-printing-and-highlighting-python
      depends_on:
        - 20261023074926-eaa51527-printing-and-highlighting-core
        - 20261023075136-42c51a7d-printing-tokens-over-the-engine
    - name: 20261023075328-741138ee-python-prettylog
      depends_on:
        - 20261023074223-e06f835f-event-data-level-enum
        - 20261023074627-cc23fc08-encode-saved-entity-during-operation
        - 20261023075302-bad71237-printing-and-highlighting-python
---


## Overview

Reading xcl's output is harder than it should be, and in Python much of it is not possible at all. Since v0.8.0 a configuration error prints as one line with no source. Highlighting needs TextMate scope strings, raw escape codes and a VS Code theme blob. Python gets finished ANSI bytes or nothing. A Python program cannot show what each resource became as an apply runs, because encoding is refused while an operation is in progress.

This epic makes xcl's output readable by default and easy to restyle in every language. The design is fixed by `design/printing-and-highlighting.md`: the core lexes and lays out tokens, each language formats them through themes and formatters, errors carry their report, and the usual loggers need little or no adapter. Around it sit three smaller changes that a pretty Python logger needs: event data levels as an enum, encoding a saved record while an operation runs, and the logger itself. Documentation for each change ships with it.

## Specs

| # | Spec | Scope |
|---|---|---|
| 1 | 20261023074223-e06f835f-event-data-level-enum | Python event data levels become a top-level enum, accepted only as the enum. |
| 2 | 20261023074627-cc23fc08-encode-saved-entity-during-operation | Saved records can be rendered while an operation runs, so Python can show resources as they are created. |
| 3 | 20261023074926-eaa51527-printing-and-highlighting-core | Go: one-call printing of errors (with source context), resources and diffs; tokens, themes and formatters (plain, terminal, HTML classes, HTML inline, SVG). |
| 4 | 20261023075136-42c51a7d-printing-tokens-over-the-engine | The engine sends other languages labelled reports on every error, labelled resource and diff text, and converted editor themes. |
| 5 | 20261023075302-bad71237-printing-and-highlighting-python | Python: the same printing, formats and themes as Go, reports in tracebacks and logs, logging bridge. |
| 6 | 20261023075328-741138ee-python-prettylog | Python pretty logger modelled on Go's example, showing resources as they are created. |
