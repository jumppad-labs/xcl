---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Printing tokens over the engine

## What was built

The xcl engine now sends readable output to programs in other languages as labelled text: the plain text split into pieces, each tagged with what it is. Every change is in the xcl repository.

- **Reports on errors.** Every error the core service sends that holds a configuration problem now carries `report`. This is Go's full report as `xcl.PrintError` prints it: the severity and summary, the location, the numbered source lines with the fault marked, and the detail, as labelled tokens.
  - The report is laid out once, in the one place every outgoing error is converted, with the same function `PrintError` uses. Its plain text therefore equals Go's plain report, and it is sent unwrapped.
  - Each cause that holds a problem carries its own report. So does a provider failure reported at its block, and so do errors carried by events while an operation runs.
  - Errors with no configuration problem, such as `not_found`, carry no report and keep their message.
- **The kind vocabulary on the wire.** A new `TokenKind` enum mirrors Go's `highlight.Kind` number for number, named `TOKEN_KIND_` plus the upper-snake Go name. A new `Token { kind, text }` message carries each piece.
  - A descriptor test holds the enum to `highlight.Kinds()` in both directions.
  - A category is `kind / 100 * 100`, and the enum is open, so an unknown kind keeps its number and prints in its category's style.
- **Labelled resources and diffs.**
  - `EncodeSavedEntity` and `EncodeEntity` take `tokens = 4` and return the rendered text labelled by the core's lexer.
  - A finished diff carries `OperationFinished.diff_tokens` beside `diff_json`. The operation runner now hands back the diff itself, so both are derived from one value.
- **Theme conversion.** A new handle-0 `ConvertTheme` call converts a VS Code colour theme into a style string per kind number through the core's TextMate rules. An unusable theme fails with the new stable code `invalid_theme`, with `source` and `reason` details.
- **The colour request is retired.** The engine no longer colours text. The `highlight` field of both encode requests is reserved by number and name, and an old caller that still sets it gets plain text.
  - `proto/buf.yaml` keeps buf's FILE rules but swaps `FIELD_NO_DELETE` for the `_UNLESS_NUMBER_RESERVED`/`_UNLESS_NAME_RESERVED` pair, so a field can be removed only by reserving both.
- **Documentation.**
  - The protocol's comments describe all of the above.
  - The other-language design `design:libxcl.md` gained the report, the labelled text (with a new "Labelled text" section), `ConvertTheme`, `diff_tokens` and the buf rule.
  - `CHANGELOG.md` has this change's section with a Breaking list.

## Why it matters

Programs written in languages other than Go could not show xcl's errors with their source, could not colour resources or change summaries their own way, and could not use an editor theme. The engine sent them either one line or finished terminal colour codes. Now each language can print the same readable output as Go from what the engine already returned, with zero extra engine calls, styled as its users choose. Binding authors, starting with the Python package (spec `20261023075302-bad71237-printing-and-highlighting-python`), build their printing on this.

## Deviations from the plan

- The success-metric test proves "no further engine call" by closing the service before printing from the received tokens, rather than by counting calls through a wrapped test client.
- `encodeAllowed`, which the plan said to leave alone, no longer existed. The merged sibling spec `20261023074627-cc23fc08-encode-saved-entity-during-operation` had replaced it, and the existing guards were left untouched.
- `bindings/libxcl/xcl.h`'s comment listing handle-0 methods now names `ConvertTheme`. This one-line doc fix was outside the plan's file list.
- The design's "Keeping the contract stable" bullet was updated to state the new buf rule, beyond the plan's list of design edits. Under the orchestrated run, the orchestrator stored the revised design through `spektacular design author`.
- A few tests were added beyond the plan's lists:
  - token count parity
  - an unknown kind through `tokensToProto`
  - a wrapped diagnostic's report
  - each apply cause's report
  - a provider cause's report
  - only styled kinds in `ConvertTheme`
  - the `ErrInvalidTheme` sentinel mapping
  - validate, destroy and load carrying no diff tokens
- The goal block for the `ConvertTheme` task was handed to its reviewer without the `coreservice/service.go` scope line. The verdicts are per criterion and cite `service.go`, so the check stands.

## Follow-up for a person

The xcl release carrying `20261023074627-cc23fc08-encode-saved-entity-during-operation` must be cut before this change merges to xcl `main`. Reserving `highlight` makes xcl-python's colour tests fail on its automatic engine bump until the Python printing spec lands. buf breaking was checked locally with buf 1.27.2; CI's 1.50.0 confirms it on the pull request. Both checks are in the test plan.
