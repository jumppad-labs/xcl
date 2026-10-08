---
created_date: "2026-10-07"
document_status: draft
project: xclconfig
spec: 20261007111826-cf3b66d8-diff-rendering-and-docs
plan: 20261007111826-cf3b66d8-diff-rendering-and-docs
---

# Render diffs as readable, optionally coloured text

You can now turn the result of `Config.Diff` into a readable summary in the style of a git diff. Each resource an apply would create, update, replace or delete is shown with its changed values, and a summary line ends the output. Values only known after apply and changed secrets are shown by placeholder, and secrets appear only when the diff was asked to reveal them. Colour is available on request, using the same highlight renderers and VS Code themes as configuration text.

> Derived from project xcl, spec/plan 20261007111826-cf3b66d8-diff-rendering-and-docs. See the project-level record for the full feature.

## What changed in this repo

- `diff.Render(d, ...diff.RenderOption) []byte` and the `diff.RenderOption` type (`diff/render.go`). They are backed by a one-line value formatter (`diff/render_value.go`) and an address-to-header reader (`diff/render_header.go`). A runnable `ExampleRender` (`diff/example_test.go`) pins the exact output.
- `diff.Highlight(renderer highlight.Renderer)` colours the output: markers and paths in the diff colours, comments as comments, headers and values as configuration. Plain output is unchanged, and stripping the codes gives it back exactly.
- `highlight.ScopeInserted`, `ScopeDeleted` and `ScopeChanged` (`markup.inserted.diff`, `markup.deleted.diff`, `markup.changed.diff`) are new exported scopes. The default terminal theme colours them green, red and yellow.
- Leak tests (`sensitive_leak_test.go`) render a real changed credential plain and highlighted, and assert that no secret appears unless revealed.

## Why

People reviewing a change need to see what an apply would do at a glance. Giving the diff a readable, theme-aware rendering lets applications show pending changes clearly without exposing sensitive values.

## Notes

- Every block aligns its `=` signs at the longest path plus one space.
- An object key named `local` is coloured as a keyword by the existing highlighter. This is a pre-existing tokenizer quirk, not changed here.
