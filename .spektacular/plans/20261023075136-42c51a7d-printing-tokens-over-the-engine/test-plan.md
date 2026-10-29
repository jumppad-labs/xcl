---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Test plan: 20261023075136-42c51a7d-printing-tokens-over-the-engine

The automated tests cover the success metric's engine half. That is `TestHostPrintsReportWithoutAnotherCall` in `coreservice/operations_test.go`: it closes the service once the finished task arrives, then prints the report from the received tokens alone. The metric's Python half ("a Python program prints a configuration problem ... with zero extra engine calls") is checked by the sibling spec `20261023075302-bad71237-printing-and-highlighting-python`, which depends on this one. No success metric needs a manual procedure here.

The plan lists the manual reviews below. A person also has to honour one release constraint.

## Manual review 1: buf lint and buf breaking against main

- **What**: the protocol only grows. The retired `highlight` field passes the breaking check only because its number and its name are both reserved.
- **How**, from the xcl repository root on the branch carrying this change, before or with the pull request:
  - `make buf-lint` (`buf lint`)
  - `make buf-breaking` (`buf breaking --against '.git#branch=main'`)
  - On the pull request, check that CI's "Lint the protos" and "Check the protos for breaking changes" steps in `.github/workflows/go.yml` pass. CI pins buf 1.50.0; this change was checked locally with buf 1.27.2.
  - Optional: to confirm the rule still bites, delete `reserved "highlight";` (or `reserved 2;`) from `EncodeSavedEntityRequest` in `proto/xcl/core/v1/core.proto`, run `make buf-breaking` and expect exit 100 ("deleted without reserving the name/number"). Then revert.
- **Pass**: both commands exit 0 and both CI steps are green. The optional probe fails as described.
- **If CI's buf 1.50.0 still reports the reserved deletion as breaking**: stop and raise it. Do not loosen the rules further. The plan's research marks this as an open assumption.
- **Who / when**: the reviewer of the pull request, before merge.

## Manual review 2: protocol comments

- **What**: the protocol's own comments in `proto/xcl/core/v1/core.proto` describe the new output.
- **Look at**:
  - `Error.report`: Go's `PrintError` report, labelled, laid out once, unwrapped, present only for configuration problems (on causes and event errors too).
  - The `TokenKind` enum comment: blocks of 100, category `kind / 100 * 100`, numbers are Go's `highlight.Kind`, names are `TOKEN_KIND_` plus the upper-snake Go name.
  - `Token`: the texts joined are the plain text.
  - Both encode requests: `reserved 2; reserved "highlight";` with the comment that an old colour request gets plain text, and `tokens = 4`.
  - Both encode responses: `tokens = 2`.
  - `OperationFinished.diff_tokens`.
  - The `ConvertTheme` rpc comment (handle 0, TextMate rules, `invalid_theme`) and `ConvertThemeRequest`/`ConvertThemeResponse`.
  - The header contract comment: retired fields reserve their number and name.
- **Pass**: every item above is documented as listed, and nothing still describes the engine colouring text.
- **Who / when**: the reviewer of the pull request, before merge.

## Manual review 3: the other-language design

- **What**: `design:libxcl.md` (source `design`, read it with `spektacular design read --data '{"source":"design","path":"libxcl.md"}'`) describes the report on errors, labelled text and theme conversion in the same way as the protocol.
- **Look at**:
  - "Error model": the `report` field, the paragraph on when it is present and that it is unwrapped, and the `invalid_theme` row in the code table.
  - "The CoreService": `ConvertTheme` under Library, handle 0.
  - "Polling": `diff_tokens = 4` on `OperationFinished`.
  - "Lookups": the reserved `highlight` field, `tokens`, and the closing paragraph (no engine colouring; old colour requests get plain text).
  - "Labelled text": `TokenKind`, `Token`, categories, open enum, `ConvertTheme`.
  - "Keeping the contract stable": the reserved-removal buf rule.
- **Pass**: each section matches the protocol comments in review 2, and the `TokenKind` values match `core.proto` exactly.
- **Who / when**: the reviewer of the pull request, before merge.

## Manual review 4: the changelog section

- **What**: `CHANGELOG.md` has the section `## 20261023075136-42c51a7d-printing-tokens-over-the-engine` at the top.
- **Look at**: the prose describes what other languages now receive:
  - `report` on errors
  - `Token`/`TokenKind`
  - `tokens` on both encode calls
  - `diff_tokens`
  - `ConvertTheme` and `invalid_theme`
  - the buf rule

  The `**Breaking:**` list names three things: the reserved `highlight` field 2 on both encode requests, the plain text old colour requests now get, and the removed generated `corepb` `Highlight`/`GetHighlight` accessors.
- **Pass**: all of the above is present and accurate against the code.
- **Who / when**: the reviewer of the pull request, before merge.

## Release constraint (done by a person, not by this run)

- **What**: the xcl release carrying `20261023074627-cc23fc08-encode-saved-entity-during-operation` must be cut before this change merges to xcl `main`.
- **Why**: reserving `highlight` makes the engine answer old colour requests with plain text. xcl-python's tests that expect colour from `highlight=True` then fail on its automatic engine bump. They keep failing until the Python printing spec `20261023075302-bad71237-printing-and-highlighting-python` regenerates the stubs and drops `highlight=`.
- **How**: before merging this branch's change to xcl `main`, confirm that the encode-saved-entity release is tagged and published. After merging, expect xcl-python's engine bump to fail its colour tests until the Python printing spec lands.
- **Pass**: the earlier release exists before the merge, and the Python spec is scheduled to land with or right after the engine bump.
- **Who / when**: the maintainer cutting releases, before the merge to xcl `main`.
