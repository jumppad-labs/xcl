---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Plan: 20261007111826-cf3b66d8-diff-rendering-and-docs

<!-- Metadata -->
<!-- Created: 2026-10-07T16:25:51Z -->
<!-- Commit: 3ffcf638b2dc89800370e9a88ed5489418cb512c -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

This plan adds `diff.Render`, which turns the result of `Config.Diff` into readable text in the style of a git diff — each resource and value marked as added, changed, replaced or removed, values known only after apply and sensitive values shown by placeholder, and a summary line of what an apply would do — and `diff.Highlight`, which colours that text through the same highlight renderers and VS Code themes xcl already uses for configuration text. It solves the gap between a diff result that code can inspect and one a person can review at a glance, and a new guide on the xcl documentation site explains how to run a diff, what each kind of change means and how to read the output. Developers building tools on xcl, and the people who review changes before applying them, benefit: they can show a clear, optionally coloured summary of pending changes without exposing secrets.

## Conventions

- **Go code style (gofmt, vet, `any`, descriptive names, stdlib-first import grouping)** — all new code in `diff/` and `highlight/`; the value formatter takes `any`.
- **Public library packages live at the module's top level** — `Render` and `Highlight` go in the existing top-level `diff` package, the new scope constants in the top-level `highlight` package.
- **Public types shared with the parser live outside the root package; `diff` imports neither the root package nor `internal/parser`** — Render must keep that rule, so it imports only `highlight`, `internal/xcl` helpers and the standard library.
- **Prefer the standard library; document any new dependency** — no new third-party dependency; `strconv`, `strings`, `sort`, `reflect`, `bytes` plus in-repo packages.
- **Testing: testify `require`, no table-driven tests, positive and negative cases in separate functions, tests next to the code they test** — `diff/render_test.go`, `diff/render_value_test.go`, `diff/example_test.go`, and default-theme tests in `highlight/theme_test.go`.
- **Never write tests that inspect repository files** — the website page is checked by human review against `ExampleRender`'s output; no test reads the page.
- **Never modify dependency packages; `internal/xcl` is an in-repo MPL fork** — `hclwrite`/`hclsyntax` are used as they are, unchanged.
- **Entity is the shared vocabulary (glossary)** — internal doc comments speak of entities; the rendered output and page keep the design's wording ("resource") because only provider-backed resources appear in a diff.

Deliberately dropped: database, HTTP, logging, graceful-shutdown, test-state-from-real-apply and graph-ordering conventions — Render is a pure formatting function over an in-memory result, with no I/O, state, providers or DAG. The shared-errors convention does not apply: Render returns no errors.

## Architecture & Design Decisions

`diff.Render(d *Diff, options ...RenderOption) []byte` and `diff.Highlight(renderer highlight.Renderer) RenderOption` are added to the existing public `diff` package (xclconfig repo, `diff/render.go`, `diff/render_value.go`), exactly as the referenced design `config-diff.md` fixes them. Render is a pure function of the result: it walks `d.Resources` in the result's order and writes, for each one, a comment line saying what will happen, a block header marked with the action (`+`, `-`, `~`, `-/+`), one line per change with `=` aligned within the block, and a closing brace; it ends with one summary line built from `d.Summary` (`Diff: 2 to create, 1 to update, 1 to replace, 1 to delete, 3 unchanged.`), which is the only line when `d.Changed()` is zero. Unknown values render as `(known after apply)`; a sensitive change renders as `(sensitive value)` unless the result carries its values, which it does only when the diff ran with `diff.RevealSensitive()`, so Render needs no reveal switch of its own and can never show what the result does not hold. The package keeps its import rule from the dependency plan — never the root package or `internal/parser` — and gains only `highlight`, `internal/xcl/hclwrite`/`hclsyntax` and the standard library. Every public name, doc comment and the page say "diff", never "plan".

The renderer is written once, as a line builder that sends every piece of output through a single *styler*: with no `Highlight` option the styler returns text unchanged, so the output is plain and has no escape codes; with one, it calls the caller's `highlight.Renderer`. This follows the spec's Technical Approach and the design ("through the same `highlight.Renderer` and themes the encoder uses"): markers and paths are passed with the action's scope, block headers and values are passed through `highlight.Text` so they are coloured as HCL, comment lines use the hash-comment scope, and `=`, `->`, braces, placeholders and the summary are passed unlabelled. The action scopes are the TextMate diff scopes real VS Code themes already colour — `markup.inserted.diff`, `markup.deleted.diff`, `markup.changed.diff` (replace uses changed) — exported from `highlight` as `ScopeInserted`, `ScopeDeleted`, `ScopeChanged` beside the grammar scopes, so a custom renderer can match them; the theme matcher's prefix rule (`highlight/theme.go:101-105`) means a theme's `markup.inserted` rule applies, and a theme without one leaves them plain, as the design requires. The default theme gains basic-colour rules for them (green, red, yellow) so the default terminal renderer colours a diff out of the box. Because both paths share one builder, the coloured output with its codes removed is byte-for-byte the plain output — the same guarantee the encoder's highlighting gives.

Two pieces of formatting are small, dedicated helpers. Values (`Before`/`After` are plain Go values per the dependency plan) are written on one line as HCL literals — quoted and escaped strings (via `hclwrite`, so `${` stays literal), shortest-form numbers of any numeric kind, `true`/`false`, `null`, `[a, b]`, and `{ key = value }` with sorted keys quoted when they are not identifiers — matching the design's `{ host = 443, local = 8443 }`. Block headers are derived from `Resource.Address` by a positional reader (`resource.container.api` → `resource "container" "api"`, `server.web` → `server "web"`), with any module prefix shown only in the comment line, since the design's types carry no separate type or name fields and Render has no registry. Alignment pads each path to the block's longest path plus one space, the design's stated rule and the encoder's own layout (the design's first example block carries two extra spaces; see the assumption log).

Documentation lands in the xcl-website repo: a new guide page `src/pages/diff.mdx` (running a diff, what create, update, replace, delete and known-after-apply mean, sensitive values, reading and colouring the rendered output), a Guides nav entry in `src/components/Nav.astro`, and a "Diffs" bullet in the sensitive-values page's list of outputs. The page's rendered example is the output of a runnable Go `ExampleRender` in `diff/example_test.go` built from the design's example result, so the success metric "the page's example matches the renderer line for line" is checked by running code plus one human comparison, never by a test that reads the page (`conventions/testing-and-mocking.md`). This beats rendering plain text and then highlighting it whole (the HCL tokenizer would mislabel markers, `->` and placeholders and could not tell actions apart) and building a separate line model with two printers (twice the code for the same output); see `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Diff renderer (new, `diff` package)** — owns `Render` and the `Highlight` render option: turns a `Diff` into the design's git-diff-style text — a comment line, a marked block header, one aligned line per change and a closing brace for each resource in the result's order, then the summary line. It decides each line's marker from the resource's action and the change's values, writes the unknown and sensitive placeholders, and sends every piece through the styler. It reads the result only; it never changes it and never consults anything outside it.
- **Styler (new, internal to the renderer)** — the single seam between plain and coloured output: identity when no renderer is given, otherwise a thin adapter over the caller's `highlight.Renderer` that colours markers and paths with the action's scope, comment lines with the comment scope, and block headers and values as HCL through the highlight package's text highlighter. Because every piece goes through it, coloured output without its codes equals the plain output.
- **Value formatter (new, internal to the renderer)** — writes a change's plain Go value (string, any numeric kind, bool, nil, list, map) as a one-line HCL literal, with HCL string escaping and sorted, quoted-when-needed map keys. Used for before and after values; never sees a placeholder.
- **Header reader (new, internal to the renderer)** — derives a block header (`resource "container" "api"`) from a resource's address, setting a module prefix aside so it appears only in the comment line.
- **Highlight package (changed)** — gains three exported scope constants for diff output (inserted, deleted, changed, named after the TextMate diff scopes real themes colour) and default-theme rules for them in the terminal's basic green, red and yellow. The renderer interface, the ANSI renderer and theme loading are reused unchanged, so any VS Code theme that colours diffs colours xcl diffs the same way.
- **Runnable example (new, `diff` package tests)** — renders the design's example result in plain text as a Go example with checked output; it is the single source the documentation page's example is copied from.
- **Diff guide page (new, xcl-website)** — explains running `Config.Diff` and revealing sensitive values, what create, update, replace, delete and known-after-apply mean, reading the rendered output line by line with the example from the runnable example, colouring it with a highlight renderer or theme, and the JSON form; linked from the Guides navigation.
- **Sensitive values page (changed, xcl-website)** — its list of what xcl shows gains diffs: a changed sensitive value is reported and rendered without its values unless revealing is asked for.

## Data Structures & Interfaces

**Public, package `github.com/jumppad-labs/xcl/diff`** — exactly the signatures the referenced design fixes, plus the option type they need:

```go
// RenderOption configures Render. Construct one with Highlight.
type RenderOption func(*renderOptions)

// Highlight colours the rendered text through renderer, the same
// highlight.Renderer the encoder uses. A nil renderer renders plain text.
func Highlight(renderer highlight.Renderer) RenderOption

// Render writes d as git-diff-style text, ending with the summary line and a
// newline. Without Highlight the text is plain, with no escape codes.
func Render(d *Diff, options ...RenderOption) []byte
```

`renderOptions` is unexported (it holds only the renderer), so the set of render options stays closed to the package's functions, as the encoder does for its options. `Render` takes `*Diff` as produced by `Config.Diff`; a nil `*Diff` renders as a diff with nothing to change. The `Diff`, `Resource`, `Change`, `Summary` and `Path` types from the dependency plan are read, not changed.

**Public, package `github.com/jumppad-labs/xcl/highlight`** — three new scope constants, a group separate from the grammar scopes:

```go
const (
	ScopeInserted = "markup.inserted.diff" // created resources, added fields and elements
	ScopeDeleted  = "markup.deleted.diff"  // deleted resources, removed fields and elements
	ScopeChanged  = "markup.changed.diff"  // updated and replaced resources, changed fields
)
```

They are the scopes Render passes to a `highlight.Renderer` for markers and paths; the existing `ScopeHashComment` is passed for comment lines, and headers and values arrive with the grammar scopes `highlight.Text` assigns. `Renderer`, `RendererFunc`, `Text`, `ANSIRenderer` and theme loading are unchanged.

**Output contract** — the rendered text is the boundary documented on the website, fixed by the design:

```
  # <address> will be created | will be updated | changed outside xcl and will be updated
  #           | will be replaced, its last apply failed | will be deleted
<m> <type> ["<subtype>"] "<name>" {        m: "  +" | "  -" | "  ~" | "-/+"
      <+|-|~> <path padded to longest+1>= <value> | <before> -> <after>
                                             | (known after apply) | (sensitive value)
    }                                       (or "{}" on the header when there are no changes)
<blank line between blocks and before the summary>
Diff: <c> to create, <u> to update, <r> to replace, <d> to delete, <n> unchanged.
Diff: no changes, <n> unchanged.          (only line when nothing changes)
```

No serialization boundary changes: the `Diff` JSON, saved state, plugin wire format and events are untouched.

## Implementation Detail

**A pure renderer beside the result types.** Rendering is added to the `diff` package as a pure function of the result, not to the root package or the parser. A reader opening the package finds the data types, the options and the renderer side by side, and can render any `Diff` — one returned by `Config.Diff`, one built by hand in a test, or one decoded by an application's own code — without touching xcl's internals. Nothing in the renderer performs I/O, returns an error or reads global state.

**One builder, one styler.** The renderer is a small line builder: per resource it writes the comment line, the header, the change lines and the closing brace; then the summary. Every piece of text it writes goes through a single styler, which is the only place that knows whether colour was asked for. That keeps the formatting rules in one place and makes "plain unless asked" structural rather than a set of `if colour` branches: the plain path is the coloured path with an identity styler. It follows the pattern the encoder already set — format first, colour through `highlight` as a separate, optional concern — and the same promise holds: removing the codes from coloured output gives back the plain output exactly.

**Colour by scope, not by code.** The renderer never writes an escape code itself. It labels pieces with scopes and lets the caller's `highlight.Renderer` decide what each scope looks like, exactly as the encoder does. Action colours use TextMate's diff scopes, which real VS Code themes already style, and which the `highlight` package now exports beside its grammar scopes, so a custom renderer (HTML, markup, a different terminal palette) can treat diff output the same way it treats configuration text. The default terminal theme gains three basic-colour rules; nothing else in `highlight` changes, and encoder output is unaffected because the grammar never produces those scopes.

**Formatting helpers kept local and small.** The one-line HCL value formatter and the address-to-header reader are unexported helpers of the renderer, each unit-tested directly. The value formatter reuses the in-repo HCL writer only for string quoting so escapes match what the encoder writes; collections are written inline to keep one line per change. Neither helper becomes a shared utility: no other code needs them yet.

**Documentation built from running code.** The rendered example on the website is copied from a runnable Go example whose output Go's test runner checks, built from the design's example result. A change to the renderer that alters the output fails that example, prompting the page update; the page itself is reviewed, never read by a test, per the testing convention. The new guide page follows the site's existing guide structure (hero, prose sections, closing call-to-action) and is added to the Guides navigation like every other guide.

**Patterns followed.** Functional options with an unexported options struct (as the encoder), `highlight.Renderer` for every colour decision, scope constants exported from `highlight`, tests next to the code with testify `require` and one behaviour per test, and the marker-renderer technique from the encoder's highlighting tests to assert which scope each piece received.

## Dependencies

- **Plan `20261007105731-2388b579-diff` (upstream, must land first)** — provides the public `diff` package (`Diff`, `Summary`, `Resource`, `Change`, `Action`, `Path`, `Diff.Changed`, `RevealSensitive`) and `Config.Diff`. This plan adds to that package and reads its types unchanged; implementation cannot start until at least its "Add the diff result types package" task has landed, and the website page's `Config.Diff` usage assumes the whole plan has.
- **`highlight` package** — `Renderer`, `RendererFunc`, `Text`, `ANSIRenderer`, theme matching. Changed only by three new exported scope constants and three default-theme rules.
- **`internal/xcl/hclwrite` and `internal/xcl/hclsyntax` (in-repo HCL fork)** — string literal quoting and identifier validation for the value formatter. Used as is; no change.
- **xcl-website (Astro 5, MDX, expressive-code)** — hosts the new guide page and nav entry; built and type-checked with the site's existing tooling. No new site dependency.
- **External libraries** — none added; standard library only (`bytes`, `reflect`, `sort`, `strconv`, `strings`).

Design documents this plan was built on:
- `config-diff.md` from the `design` source — the settled `Render`/`Highlight` signatures, markers, comment texts, placeholders, summary line forms and colour behaviour this plan implements. Binding; the only divergence is the alignment of its first example block (two extra spaces), which this plan resolves by the design's own stated rule.

## Testing Approach

Tests follow the project's conventions: testify `require`, one behaviour per test function, no table-driven tests, positive and negative cases in separate functions, and tests next to the code they test. No test reads the website page or any other repository file.

**Unit tests — renderer.** The renderer gets the densest coverage because it carries every rule of the design's output format. Results are built by hand in Go, so each test isolates one rule: each action's comment line, header marker and body; an update with no changes saying the resource changed outside xcl; delete and replace with no changes rendering an empty-bodied header; added, removed and changed values with their markers and `before -> after`; known-after-apply with and without a prior value; a masked sensitive change containing neither value and showing `(sensitive value)`; a revealed sensitive change showing both values; `=` aligned within each block independently; resources in the result's order; the summary line with all five numbers; the summary as the only line when nothing changes; and a nil result. A golden test renders the design's example result and compares it with the expected text in full.

**Unit tests — colour.** Plain output contains no escape codes. With `Highlight` and a marker renderer (the technique the encoder's highlighting tests use) the tests assert which scope each piece received: inserted, deleted and changed scopes on markers and paths per action, the comment scope on comment lines, grammar scopes on headers and values. With the built-in terminal renderer the output contains escape codes, and stripping them gives back exactly the plain output. A theme with no diff rules leaves markers uncoloured. Default-theme tests assert the three new rules resolve to basic green, red and yellow and the theme still uses only the 16 basic colours.

**Unit tests — helpers.** The value formatter: strings quoted and escaped as HCL (including a literal `${`), integer and fractional numbers of several Go kinds, bools, nil, empty and non-empty lists and maps, sorted keys, non-identifier keys quoted, nested values on one line. The header reader: root `resource` addresses, subtype-less and custom-keyword addresses, and module-prefixed addresses.

**Runnable example.** A Go example renders the design's example result and its checked output is the text the website page shows; Go's test runner fails if the renderer's output drifts from it.

**Integration test — real diff, rendered.** Through the public `Config.Diff` on the dependency plan's sensitive-credential fixture, rendering the result plain and highlighted contains no secret without `diff.RevealSensitive()`, and contains both values with it. This extends the existing sensitive-leak coverage to renderings.

Success metrics:
- *The rendered example on the documentation page matches, line for line, what the renderer produces for the same diff result* — **Behavioural test** for the renderer side: the runnable example pins the exact rendered text of the design's example result, so any output change fails the test run. **Manual — captured in the implementation test plan**: a person checks that the page's example block is identical, line for line, to the example's checked output (no test may read the page).
- *No sensitive value appears in any rendering produced without an explicit request to reveal it* — **Behavioural test**: the renderer unit tests and the `Config.Diff` integration test assert that plain and highlighted renderings of a result with a changed sensitive value contain neither the old nor the new secret unless the diff was run with `diff.RevealSensitive()`.

Manual reviews:
- **Manual — captured in the implementation test plan**: review the new website page reads correctly — it explains running a diff, create, update, replace, delete and known-after-apply, sensitive values and colour — and that the site builds and type-checks with the page and nav entry.
- **Manual — captured in the implementation test plan**: view a highlighted rendering in a real terminal with the default theme and with a VS Code theme that colours diffs, and confirm added, removed and changed lines read clearly.
- **Manual — captured in the implementation test plan**: review that every new public name, doc comment and the page say "diff" and never "plan".

Deliberate gaps: no tests of the `example/` programs (non-goal); no performance tests (no metric asks for one); no tests of the website build itself beyond the manual build check.

## Milestones & Tasks

### Milestone 1: A diff can be read as a git-diff-style summary

**What changes**: Applications can turn a diff result into readable text in one call. Each changed resource is shown as a block with a comment saying what will happen, marked as created, updated, replaced or deleted, with one line per changed value showing what is added, removed or changed from old to new. Values known only after apply are shown as such, a changed sensitive value is shown as changed without either value unless the diff revealed it, and a summary line ends the output with the numbers to create, update, replace and delete and the number unchanged. The text is plain, ready to print or log.

**Validation point**: The renderer's tests pass for every action, value marker, placeholder and summary form; rendering the design's example result produces the design's text (with alignment by its stated rule); the runnable example's checked output matches; and a real `Config.Diff` with a changed secret renders without the secret unless revealed.

#### - [x] Task: Render a diff as plain text
**Id:** d0553186-6e90-437e-bd57-0e7ebdbf205b
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Adds `Render` to the public `diff` package, turning a diff result into the git-diff-style text the referenced design fixes: a comment line, a marked header, one aligned line per changed value and a closing brace for each changed resource, then the summary line. Values are written as one-line configuration literals, values known only after apply and masked sensitive values get their placeholders, and nothing is coloured yet. A runnable example renders the design's example result so its output can be copied into the documentation.

*Technical detail:* [context.md#task-render-a-diff-as-plain-text](./context.md#task-render-a-diff-as-plain-text)

**Acceptance criteria**:
- [x] Created resources and values are marked as added, deleted ones as removed, replaced ones as replaced, and updated ones as changed with the old and new values shown together.
- [x] Each resource has a comment line saying what will happen, including a resource that changed outside xcl, and resources appear in the result's order.
- [x] A value known only after apply is shown as known after apply rather than as a concrete value.
- [x] A changed sensitive value is shown as changed with neither its old nor its new value, unless the result carries the values because the diff revealed them, in which case both appear.
- [x] The `=` signs line up within each resource's block.
- [x] The output ends with a summary line giving the numbers to create, update, replace and delete and the number unchanged; when nothing changes, the summary line is the only output.
- [x] The output contains no colour codes.
- [x] The design's example result renders as the design's example text, and the runnable example shows that text.
- [x] Public names and doc comments say "diff", never "plan".

#### - [x] Task: Prove renderings of a real diff keep secrets hidden
**Id:** 1cf90c27-512d-4b35-bd61-56d393331a2f
**Repo:** xclconfig
**Depends on:**
- d0553186-6e90-437e-bd57-0e7ebdbf205b — Render a diff as plain text
**Execution:** agent

Extends the sensitive-leak checks to rendered diffs: a real configuration whose sensitive value has changed is diffed through `Config.Diff` and rendered, and the rendering must not contain the secret unless the diff was asked to reveal it. This turns the spec's "no sensitive value in any rendering" metric into a test over the whole path, not only hand-built results.

*Technical detail:* [context.md#task-prove-renderings-of-a-real-diff-keep-secrets-hidden](./context.md#task-prove-renderings-of-a-real-diff-keep-secrets-hidden)

**Acceptance criteria**:
- [x] Rendering a real diff with a changed sensitive value contains neither the old nor the new secret, and shows that a sensitive value changed.
- [x] Rendering the same diff run with sensitive values revealed contains both values.

### Milestone 2: The diff can be shown in colour

**What changes**: Applications can ask for the rendered diff in colour, using the same highlight renderers and VS Code themes they already use for configuration text. Added lines are coloured as insertions, removed ones as deletions, changed ones as changes, and values are coloured as configuration. The built-in terminal renderer colours a diff out of the box, a VS Code theme that colours diffs colours xcl diffs the same way, and without the option the output stays plain.

**Validation point**: Plain output has no escape codes; highlighted output gives each piece the expected scope, contains escape codes with the terminal renderer, and equals the plain output once the codes are removed; the default theme's diff colours resolve and it still uses only the 16 basic colours.

#### - [x] Task: Add diff colours to the highlight package
**Id:** f04d43d7-13ce-4e5d-a9ba-97e3662c8b91
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Gives the highlight package names for the three kinds of diff line — inserted, deleted and changed — using the scopes VS Code themes already colour for diffs, and adds basic green, red and yellow for them to the built-in terminal theme. Custom renderers can match the new names, and highlighting of configuration text is unaffected.

*Technical detail:* [context.md#task-add-diff-colours-to-the-highlight-package](./context.md#task-add-diff-colours-to-the-highlight-package)

**Acceptance criteria**:
- [x] The inserted, deleted and changed scopes are exported and documented as the scopes diff output uses.
- [x] The built-in terminal theme colours inserted text green, deleted text red and changed text yellow, and still uses only the terminal's 16 standard colours.
- [x] A VS Code theme's diff colours apply to the new scopes.
- [x] Highlighted configuration text is unchanged.

#### - [x] Task: Colour the rendered diff on request
**Id:** c49e9113-5be2-4515-bc6f-98ff2345b56d
**Repo:** xclconfig
**Depends on:**
- d0553186-6e90-437e-bd57-0e7ebdbf205b — Render a diff as plain text
- f04d43d7-13ce-4e5d-a9ba-97e3662c8b91 — Add diff colours to the highlight package
**Execution:** agent

Adds the `Highlight` render option, so a caller can pass the same highlight renderer it uses for configuration text and get a coloured diff: added, removed and changed markers and paths in the theme's diff colours, comment lines as comments, and headers and values coloured as configuration. Without the option, output stays exactly as it was.

*Technical detail:* [context.md#task-colour-the-rendered-diff-on-request](./context.md#task-colour-the-rendered-diff-on-request)

**Acceptance criteria**:
- [x] Without the option, rendered output contains no colour codes; with the built-in terminal renderer, it contains colour codes.
- [x] Added, removed and changed lines receive the inserted, deleted and changed colours; replaced resources use the changed colour; values and headers are coloured as configuration.
- [x] Removing the colour codes from coloured output gives back exactly the plain output.
- [x] A theme with no diff colours leaves markers and paths in the terminal's default colour.
- [x] Sensitive values stay hidden in coloured output exactly as in plain output.

### Milestone 3: The documentation explains how to run and read a diff

**What changes**: The xcl documentation site gains a guide that shows how to run a diff, explains what create, update, replace, delete and known after apply mean, how sensitive values are hidden and revealed, and walks through an example of the rendered output and how to colour it. The guide is in the site's Guides navigation, and the sensitive values guide lists diffs among the outputs that hide secrets.

**Validation point**: The site builds and type-checks with the new page and nav entry, and the page's rendered example is identical to the renderer's checked example output.

#### - [x] Task: Write the diff guide on the documentation site
**Id:** 3aa01462-ab79-47ae-ae96-4100636fe75d
**Repo:** xcl-website
**Depends on:**
- d0553186-6e90-437e-bd57-0e7ebdbf205b — Render a diff as plain text
- c49e9113-5be2-4515-bc6f-98ff2345b56d — Colour the rendered diff on request
**Execution:** agent

Adds a guide page to the xcl site that shows how to run a diff and render it, explains what create, update, replace, delete and known after apply mean, how sensitive values are hidden and revealed, and walks through the rendered output using the renderer's own example, then shows how to colour it and where the JSON form fits. Links it from the Guides navigation, and adds diffs to the sensitive values guide's list of outputs that hide secrets.

*Technical detail:* [context.md#task-write-the-diff-guide-on-the-documentation-site](./context.md#task-write-the-diff-guide-on-the-documentation-site)

**Acceptance criteria**:
- [x] The site has a diff page that explains running a diff, the meaning of create, update, replace, delete and known after apply, and shows an example of the rendered output.
- [x] The page's rendered example is identical, line for line, to the renderer's example output.
- [x] The page explains how sensitive values appear and how to reveal them, and how to colour the output.
- [x] The page is reachable from the Guides navigation, and the sensitive values guide mentions diffs.
- [x] The page uses the word "diff" throughout and never "plan" for this feature.
- [x] The site builds with the new page.

## Open Questions

- **Do the upstream plan's `Before`/`After` values hold only plain Go values as planned?** It depends on how the upstream comparator converts `cty.Value` fields and numbers once implemented (e.g. `float64`, `json.Number` or `*big.Float`). The value formatter handles every Go numeric kind and `json.Number` by reflection and falls back to a quoted string; if the upstream implementation ever puts a `cty.Value`, a `types.Sensitive` wrapper or another non-plain type into a change, STOP and ask the user rather than teaching Render to unwrap it, since that would mean the result itself breaks its documented contract.

## Out of Scope

- Producing the diff itself (`Config.Diff`, the result types, JSON) — plan `20261007105731-2388b579-diff`, which this plan builds on.
- Adding diff output to the `example/` programs — left for later (spec non-goal).
- A command-line interface; xcl remains a library (spec non-goal).
- Showing the saved values of a resource that would be deleted; a delete renders as an empty-bodied header (spec non-goal, design).
- Changes to the VS Code extension; the diff scopes are ones its themes already colour (spec non-goal).
- Detecting whether output is a terminal; whether to colour stays the caller's decision, as for configuration text.
- A README section on diffs; the spec asks for the documentation site only.
- Multi-line or wrapped value rendering; every change is one line, however long its value.
- Rendering formats other than text (HTML, Markdown); a custom `highlight.Renderer` can style the text, but there is no separate format.
- Correcting the alignment in the design document's first example block — noted for the user's review, not changed by this plan.
- CHANGELOG.md entries — left to the implement workflow's changelog step.

## Changelog

### 2026-10-07 — Task: Render a diff as plain text

**What was done**: Added `diff.Render` and the `RenderOption` type. Render writes a `Diff` as git-diff-style text: a comment line, a marked header, aligned change lines and a closing brace for each resource, then the summary line. Every piece goes through an internal styler, which is the identity for now. Added two unexported helpers, the one-line HCL value formatter `formatValue` and the address-to-header reader `blockHeader`. Added unit tests, a golden test of the design example, and `ExampleRender`, whose checked output is the text the website copies.

**Deviations**: Implementation started before the dependency `20261007105731-2388b579-diff` was recorded as implemented: the store reported it as "planned, not started". The user approved overriding it through the orchestrator because the record was stale. That spec is implemented and its code is merged into this branch (316bb5f), but its finished plan and changelog are not committed yet. Otherwise: the design example's `api` block is aligned at longest+1, as the plan decided, not with the design's two extra spaces. gofmt strips leading spaces inside doc-comment code blocks, so the example in Render's doc comment is not indented like the real output.

**Files changed**:
- `xclconfig: diff/render.go`
- `xclconfig: diff/render_value.go`
- `xclconfig: diff/render_header.go`
- `xclconfig: diff/render_test.go`
- `xclconfig: diff/render_value_test.go`
- `xclconfig: diff/render_header_test.go`
- `xclconfig: diff/example_test.go`

**Discoveries**: The upstream comparator turns values into plain Go values through ctyjson, so every number arrives as a float64. `strconv.FormatFloat(v, 'f', -1, 64)` writes them in their shortest form (8080, not 8080.0). Per the revised design, dependents of an updated resource arrive as `update` changes with `Before` set and `Unknown: true`, and they render as `~ path = <before> -> (known after apply)`.

### 2026-10-07 — Task: Prove renderings of a real diff keep secrets hidden

**What was done**: Added two root-package leak tests. Each one diffs a real configuration whose sensitive password changed, through `Config.Diff`, and renders the result. The rendering without reveal contains neither password and shows `~ password = (sensitive value)`. The rendering of the diff run with `diff.RevealSensitive()` shows `"<before>" -> "<after>"`.

**Deviations**: The tests reuse the upstream `diffChangedCredential` helper and its `credential/before` and `credential/after` fixtures from `config_diff_test.go`, which the plan's context pointed to only loosely. The highlighted assertion is added by the colour task, as planned.

**Files changed**:
- `xclconfig: sensitive_leak_test.go`

**Discoveries**: None.

### 2026-10-07 — Task: Add diff colours to the highlight package

**What was done**: Exported the TextMate diff scopes `ScopeInserted` (`markup.inserted.diff`), `ScopeDeleted` and `ScopeChanged` from `highlight` as a separate, documented constant group. Added basic green, red and yellow rules for `markup.inserted`, `markup.deleted` and `markup.changed` to the default theme. Updated the package doc. Tests cover the default-theme colours, a VS Code theme's `markup.inserted` rule applying to `ScopeInserted`, and a theme without markup rules leaving `ScopeDeleted` plain.

**Deviations**: None.

**Files changed**:
- `xclconfig: highlight/highlight.go`
- `xclconfig: highlight/theme.go`
- `xclconfig: highlight/doc.go`
- `xclconfig: highlight/theme_test.go`
- `xclconfig: highlight/ansi_test.go`

**Discoveries**: None.

### 2026-10-07 — Task: Colour the rendered diff on request

**What was done**: Added `diff.Highlight(renderer)`. The styler now passes markers and paths to the renderer with `ScopeInserted`, `ScopeDeleted` or `ScopeChanged` (replace uses changed), comment lines with `ScopeHashComment`, headers and values through `highlight.Text`, and every other piece with the empty scope. Indentation and newlines stay outside every styled piece. Tests assert the scope of each piece with a marker renderer. They also check that ANSI output contains codes and equals the plain output once the codes are stripped, that a theme without markup rules leaves markers plain, and that `Highlight(nil)` gives plain output. Two new leak tests cover the highlighted rendering of a real changed credential, with and without reveal.

**Deviations**: The header markers and the closing brace now carry their indentation outside the styled piece: `headerMarker` returns `+`, `-`, `~` or `-/+`, and the padding is written separately. The plain output is unchanged.

**Files changed**:
- `xclconfig: diff/render.go`
- `xclconfig: diff/render_test.go`
- `xclconfig: sensitive_leak_test.go`

**Discoveries**: `highlight.Text` labels an object key named `local` as `storage.type.xcl`, the block-type keyword scope, while `host` gets `variable.other.property.xcl`. So `{ host = 443, local = 8443 }` colours `local` as a keyword. This is pre-existing tokenizer behaviour, probably shared with the encoder's highlighting, and was not changed here.

### 2026-10-07 — Task: Write the diff guide on the documentation site

**What was done**: Added the `/diff/` guide page. It covers what a diff is, running `Config.Diff` and reading the result, the four actions, known-after-apply, sensitive values and `diff.RevealSensitive()`, reading the rendered output with the example and a marker table, the summary forms, colour with `diff.Highlight` and the three scopes, and the JSON form. Also added a "Diffs" entry to the Guides nav and a "Diffs" bullet to the sensitive-values page's list of outputs. The page's example block was compared mechanically with `ExampleRender`'s `// Output:` and is identical. `npm run build` and `astro check` pass, with 0 errors and 0 warnings.

**Deviations**: Following the revised design, the page explains that a diff assumes updating a resource can change the values its provider fills in, so resources that refer to them are reported as updates with those values known after apply. It does not mention a provider method for narrowing this, since none exists yet. The JSON example on the page lists a single created resource, with a matching summary, instead of reproducing the design's full example.

**Files changed**:
- `xcl-website: src/pages/diff.mdx`
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/sensitive-values.mdx`

**Discoveries**: None.
