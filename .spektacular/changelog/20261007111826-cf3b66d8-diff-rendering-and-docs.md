---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Diff rendering and documentation

## What was built

- **Readable diffs (xclconfig).** `diff.Render(d, ...diff.RenderOption) []byte` turns the result of `Config.Diff` into text in the style of a git diff. For each resource an apply would change, it writes a comment saying what would happen and a block header marked `+`, `-`, `~` or `-/+`. Inside the block, each changed value gets its own line marked as added, removed or changed (`before -> after`), with the `=` signs aligned. The output ends with a summary line: `Diff: 1 to create, 2 to update, 1 to replace, 1 to delete, 4 unchanged.`, or `Diff: no changes, N unchanged.` when nothing would change. Values only known after apply render as `(known after apply)`, also as `"old" -> (known after apply)` for dependents of an updated resource. Changed sensitive values render as `(sensitive value)` unless the diff was run with `diff.RevealSensitive()`. Values are written as one-line configuration literals. A runnable `ExampleRender` pins the exact output of the design's example.
- **Colour on request (xclconfig).** `diff.Highlight(renderer)` colours the rendering through the same `highlight.Renderer` and VS Code themes the encoder uses. Markers and paths take the new exported scopes `highlight.ScopeInserted`, `ScopeDeleted` and `ScopeChanged`: the TextMate `markup.*.diff` scopes that real themes already colour. Comments are coloured as comments, and headers and values as configuration. The default terminal theme gains green, red and yellow for the three scopes. Without the option the output stays plain, and stripping the colour codes gives back the plain text exactly.
- **Leak coverage (xclconfig).** The sensitive-leak suite now renders a real `Config.Diff` of a changed credential, plain and highlighted. It asserts that neither password appears unless the diff revealed them.
- **Documentation (xcl-website).** A new `/diff/` guide covers:
  - running a diff;
  - the four actions;
  - known after apply, including why dependents of an updated resource show it;
  - sensitive values;
  - reading the rendered output, with the example copied verbatim from `ExampleRender`;
  - colour;
  - the JSON form.

  The guide is in the Guides menu, and the sensitive-values guide now lists diffs among the outputs that hide secrets.

## Why it matters

People reviewing a change want to see at a glance what an apply would do, not read raw data. Applications built on xcl can now print a clear, optionally coloured summary of pending changes without exposing secrets, and the documentation explains how to run a diff and read its output.

## Deviations from the plan

- Implementation started before the dependency `20261007105731-2388b579-diff` was recorded as implemented: the store reported it as "planned, not started". The user approved overriding it because the record was stale. That spec is implemented and its code is merged into this branch (316bb5f), but its finished plan and changelog were not committed when this run started.
- The design's example aligns its first block with two extra spaces. The rendering aligns every block at the longest path plus one space, as the plan decided, so the documented example follows that rule.
- Header markers and closing braces carry their indentation outside the styled pieces. Only colour output is affected, and the plain text is unchanged.
- Following the revised design, the guide explains that a diff assumes an updated resource's provider-filled values can change. It documents no way of narrowing this, because none exists yet.
- The leak tests reuse the upstream `diffChangedCredential` helper and its fixtures.

## Known follow-ups

- `highlight.Text` labels an object key named `local` with the block-type keyword scope, so in `{ host = 443, local = 8443 }` the key `local` is coloured as a keyword. This is pre-existing tokenizer behaviour and was left unchanged.
