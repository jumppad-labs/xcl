---
created_date: "2026-10-07"
---

# Planning summary: 20261007105731-2388b579-diff

## Decisions
None.

## Order added for shared files
None added.

## 20261007105731-2388b579-diff
### Approach
`Config.Diff(paths, ...diff.Option)` runs the existing apply walk in a diff mode. A new `Parser.Diff` reuses `parseAndValidate`, the empty-configuration check, `removedResources` (for deletes) and `Parser.walk`. Apply's read path becomes a shared `refresh` step (Read, then Changed), so apply and diff decide changes the same way. A recorder guarded by a mutex tracks pending resources (to be created or replaced), unknown paths and the result. The computed fields of pending resources become `cty.Unknown` through an optional hook in context building. A diff decoder records the unknown paths, swaps in placeholders and decodes a copy of the body. A new comparator in `internal/parser` produces the field-level changes. The result types go in a new public `diff` package.

Three alternatives were rejected:
- A dry-run provider adapter: it would still fire events, save state and give no unknowns.
- A standalone comparator: a separate code path, which the spec forbids.
- Decoding unknowns directly: gocty can't hold them.

### Milestones and tasks
- **M1, code built on xcl can work with diff results:** Add the diff result types package
- **M2, a diff reports which resources an apply would create, update, replace or delete:** Share the read-and-compare step between apply and diff; Add the diff walk to the parser; Add Config.Diff as a public operation
- **M3, a diff shows exactly which values would change, with secrets hidden:** Compute field-level changes between saved and configured resources; Report value changes in diff results
- **M4, a diff marks values known only after apply and matches what apply does:** Mark values known only after apply; Prove diff matches apply end to end

All 8 tasks are agent tasks in repo `xclconfig`.

### Tasks a person must do
None.

### Out of scope
- Rendering, Highlight and the website docs (spec cf3b66d8)
- Diff output in the `example/` programs
- A CLI
- Saving a diff to apply later
- Matching list elements by identity
- Showing the saved values of a deleted resource
- The VS Code extension
- Predicting what Update would compute
- Reading diff JSON back into Go types

### Drafting assumptions
- Options: `Option func(*Options)`, `Options{RevealSensitive bool}`, `RevealSensitive()`, `NewOptions(...)`. The changed count is a method, `Diff.Changed() int`.
- A create lists each non-computed top-level field that is set in the configuration or is non-zero, so defaults show.
- An unknown entry on update or replace keeps the saved value as `before`.
- A whole added or created value that holds a sensitive value is split until the sensitive value stands alone.
- A nil list or map equals an empty one. `cty.Value` fields are compared as single values.
- `Path` can be written to JSON but not read back.
- A `disabled` expression that evaluates to unknown fails the diff with the existing decode error.
- On success the core writes one debug log, "diff complete", with the counts.
- The diff recorder is its own component, separate from `applyProgress`.
- Unknowns come last (M4). Until then, a reference to a computed value of a resource being created reads as that value's empty state before create.
- Diff fixtures go under `internal/test_fixtures/config/diff/`, and test state comes from real applies.

Open for implementation:
- Does gohcl accept every placeholder value? If it would take a change to the HCL fork, stop and ask.
- Can the e2e plugin produce a failed resource using public packages only? If not, replace is covered only by the root-package tests.

### Project-wide rules
- A new public top-level package `github.com/jumppad-labs/xcl/diff`, importing neither the root package nor `internal/parser`
- A new constant `events.OperationDiff = "diff"`
- Public names, doc comments and errors say "diff", never "plan"
- CHANGELOG.md is left to the implement workflow's changelog step
- Test state comes from real applies with `parser.TestPlugin`; tests sit next to the code they test

### Manual checks
- A person checks that every public name, doc comment and error message uses "diff" and never "plan".
- A person checks the `diff` package's godoc against the design (actions, path forms, which JSON fields appear).

## 20261007111826-cf3b66d8-diff-rendering-and-docs
### Approach
`diff.Render(d *Diff, options ...RenderOption) []byte` and `diff.Highlight(renderer highlight.Renderer) RenderOption` go into the `diff` package, with the signatures `config-diff.md` gives. Render builds the output line by line and passes every piece through one styler. Without `Highlight` the output is plain text. With `Highlight`, each piece goes to the caller's renderer, and coloured output with its codes removed is byte for byte the plain output. Colours use the TextMate scopes `markup.inserted.diff`, `markup.deleted.diff` and `markup.changed.diff`. `highlight` exports them as `ScopeInserted`, `ScopeDeleted` and `ScopeChanged`, and the default theme gets green, red and yellow rules for them. Headers and values are highlighted as HCL. The docs page's example is copied from a runnable `ExampleRender`.

Two alternatives were rejected:
- Highlighting the whole plain output: the tokenizer would mislabel markers.
- A separate line model with two printers: it would duplicate the formatting logic.

### Milestones and tasks
- **M1, a diff can be read as a git-diff-style summary:** Render a diff as plain text (xclconfig); Prove renderings of a real diff keep secrets hidden (xclconfig)
- **M2, the diff can be shown in colour:** Add diff colours to the highlight package (xclconfig); Colour the rendered diff on request (xclconfig)
- **M3, the documentation explains how to run and read a diff:** Write the diff guide on the documentation site (xcl-website: `src/pages/diff.mdx`, a Guides nav entry, a "Diffs" bullet on the sensitive-values page)

All five tasks are agent tasks. The first plain-text rendering task and the colours task can run in parallel. This plan needs plan 2388b579 implemented first.

### Tasks a person must do
None.

### Out of scope
- Producing the diff itself (plan 2388b579)
- Diff output in the `example/` programs
- A CLI
- Saved values of deleted resources
- Changes to the VS Code extension
- Terminal detection
- A README section
- Multi-line or wrapped values
- Output formats other than text
- Correcting the design document's example
- CHANGELOG entries

### Drafting assumptions
- A single line builder with a pluggable styler.
- `=` alignment pads each path to the longest path in its block plus one space. The design's `api` example pads by three and is probably wrong; check it at review.
- Headers come from the address with a positional reader, and module prefixes appear only in the comment line. The design's types have no type or name fields, and Render has no type registry. The only address it reads wrongly is a custom type with no subtype inside a nested module.
- Values are written on one line as HCL literals: hclwrite string escaping, numbers in shortest form, `null`, `[a, b]`, and `{ k = v }` with keys sorted.
- Markers:
  - An added value: `+ path = after`
  - A removed value: `- path = before`
  - A changed value: `~ path = before -> after`
  - An unknown value: `~ path = before -> (known after apply)`
  - A masked sensitive value: `+` in a create, `~` otherwise
- Sensitive values appear only after `RevealSensitive()`.
- There is a blank line between blocks and before the summary, and the output ends with a newline. A nil `*Diff` renders as an empty diff. The "no changes" summary is used when `Changed()` is 0.
- On a coloured change line, the marker and path take the action's colour and the value is coloured as HCL.

### Project-wide rules
- The `diff` package must not import the root package or `internal/parser`. This plan adds only `highlight`, `internal/xcl`, `internal/cty` and the standard library.
- Public names, docs and the page say "diff", never "plan".
- Tests sit next to the code they test, and no test reads website or docs files. The docs example is pinned by `ExampleRender`.
- Sensitive-leak coverage lives in `sensitive_leak_test.go`. Rendering cases go beside the diff result cases.
- `highlight` exports `ScopeInserted`, `ScopeDeleted` and `ScopeChanged` (`markup.*.diff`), and the default theme colours them.
- New website guide pages are added to the static Guides list in `Nav.astro`.
- CHANGELOG.md is left to the implement workflow's changelog step.

### Manual checks
- The website page's example is identical, line for line, to `ExampleRender`'s checked output.
- The page explains running a diff, create, update, replace, delete, known after apply, sensitive values and colour. The site builds and type-checks with the new page and nav entry.
- A coloured rendering, viewed in a real terminal with the default theme and with a VS Code theme that colours diffs, shows added, removed and changed lines clearly.
- Every new public name, doc comment and the page say "diff" and never "plan".
