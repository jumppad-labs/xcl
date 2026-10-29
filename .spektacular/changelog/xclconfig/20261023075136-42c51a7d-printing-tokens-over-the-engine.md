---
created_date: "2026-10-26"
document_status: draft
project: xclconfig
spec: 20261023075136-42c51a7d-printing-tokens-over-the-engine
plan: 20261023075136-42c51a7d-printing-tokens-over-the-engine
---

# Labelled output over the core service

Programs that use xcl from another language get the same readable output as Go from the engine. A configuration problem arrives with its full report, including the numbered source lines with the fault marked, ready to print without asking the engine again. Rendered resources and change summaries can come as labelled text a language styles its own way, and a VS Code colour theme can be converted without creating a configuration. The engine no longer sends terminal colour codes: an older caller that still asks for colour gets plain text.

> Derived from project xcl (/home/nicj/code/github.com/jumppad-labs/xcl), spec/plan 20261023075136-42c51a7d-printing-tokens-over-the-engine. See the project-level record for the full feature.

## What changed in this repo

- **Protocol (`proto/xcl/core/v1/core.proto`, regenerated into `coreservice/proto`)**:
  - New `TokenKind` enum, numbered as Go's `highlight.Kind`, and a new `Token` message.
  - `Error.report = 5`.
  - `bool tokens = 4` on both encode requests and `repeated Token tokens = 2` on both responses.
  - `OperationFinished.diff_tokens = 4`.
  - New handle-0 `ConvertTheme` call with its messages.
  - `highlight = 2` on both encode requests reserved by number and name.
- **Core service (`coreservice/`)**:
  - `toProto` attaches the labelled report to every error that holds a diagnostic, at every level of causes, through a single `tokensToProto` converter (`tokens.go`).
  - The encode handlers return labelled text on request, and the legacy colouring is deleted.
  - The operation runner derives `diff_json` and `diff_tokens` from the one diff.
  - `ConvertTheme` (`theme.go`) wraps `highlight.LoadVSCodeTheme`.
  - The new `invalid_theme` error code.
- **Contract check**:
  - `proto/buf.yaml` keeps buf's FILE rules with `FIELD_NO_DELETE` replaced by `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`.
  - The comments in `buf.yaml`, `.github/workflows/go.yml` and the proto header say a field is removed only by reserving its number and name.
- **Docs**:
  - `CHANGELOG.md` section with a Breaking list.
  - `bindings/libxcl/xcl.h` lists `ConvertTheme` among the handle-0 methods.
  - The other-language design `libxcl.md` is revised.
- **Tests**: kind parity with Go in both directions, the token converter, reports on unit errors and through a failed apply, its error event, causes and provider failures, and a host printing a report with the engine closed. They also cover labelled encode text for saved, looked-up and edited records, raw old-caller bytes getting plain text, diff tokens equal to Go's `PrintDiff`, and `ConvertTheme`.

## Breaking

- `highlight` (field 2) on `EncodeSavedEntityRequest` and `EncodeEntityRequest` is reserved. A request that still sets it gets plain text: set `tokens` and format the tokens.
- The generated Go `corepb.EncodeSavedEntityRequest.Highlight`, `corepb.EncodeEntityRequest.Highlight` and their `GetHighlight` methods are removed.

## Why

xcl's engine is how every other language runs xcl's Go core, and it sent either one-line errors or finished terminal colour codes. Lexing and laying out reports, resources and diffs stay in this repo's core, and the engine now sends their labelled form. Each language binding, starting with Python, can then print exactly what Go prints and style it its own way.

## Release note for maintainers

Cut the release carrying the encode-saved-entity-during-operation change before merging this to `main`. xcl-python's tests that expect colour from `highlight=True` fail against an engine built from this change until the Python printing spec regenerates its stubs.
