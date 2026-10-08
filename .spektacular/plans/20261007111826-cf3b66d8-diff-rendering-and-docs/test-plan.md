---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Test plan: 20261007111826-cf3b66d8-diff-rendering-and-docs

Automated coverage, not repeated here: the rendering rules (`diff/render_test.go`, `diff/render_value_test.go`, `diff/render_header_test.go`), the checked `ExampleRender` output (`diff/example_test.go`), the colour scopes (`highlight/theme_test.go`, `highlight/ansi_test.go`), and the "no sensitive value in an unrevealed rendering" metric (`sensitive_leak_test.go`: `TestLeakDiffRendering*` and `TestLeakDiffHighlightedRendering*`).

## 1. Success metric: the page's example matches the renderer line for line

- **What to measure**: the `text` block under "Reading the rendered output" in `xcl-website: src/pages/diff.mdx` against the `// Output:` block of `ExampleRender` in `xclconfig: diff/example_test.go`.
- **How**:
  1. From the xclconfig root, run `go test -run ExampleRender ./diff/` and confirm it passes. That proves the `// Output:` block is what `diff.Render` produces.
  2. Open both files and compare the two blocks line by line, including leading spaces (two spaces before `#`, `~`, `+` and `-`, none before `-/+`) and the blank lines between blocks. A mechanical check: extract the `// Output:` lines with their `// ` prefix stripped, extract the page's first ```` ```text ```` block in that section, and `diff` the two.
- **Expected result**: zero differing lines. Any difference fails.
- **Who / when**: the reviewer of any change to `diff/render.go`, `diff/example_test.go` or the page, before merging.

## 2. Review: the diff guide reads correctly and the site builds

- **What to look at**: the `/diff/` page (`xcl-website: src/pages/diff.mdx`), the Guides menu (`src/components/Nav.astro`), and the "What xcl shows, and what it keeps" list on `/sensitive-values/`.
- **How**: in the xcl-website root, run `make install` (or `npm ci`), then `make build` and `make check`, then `make dev` and open `http://localhost:4321/diff/`.
- **Pass when**:
  - `make build` lists `/diff/index.html` and `make check` reports 0 errors.
  - The page explains running a diff, the meaning of create, update, replace, delete and known after apply, sensitive values and how to reveal them, and colour.
  - The Guides menu shows "Diffs" and it opens the page.
  - The sensitive-values list has a "Diffs" bullet linking to `/diff/`.
  - The page does not describe a provider method for narrowing which computed values an update changes, since none exists yet.

## 3. Review: highlighted rendering in a real terminal

- **What to look at**: `diff.Render(d, diff.Highlight(renderer))` printed to a colour terminal.
- **How**: write a small Go program (or a scratch test run with `-v`) that builds the design example `Diff` (copy it from `ExampleRender`) and prints it twice:
  1. with `highlight.NewANSIRenderer()`;
  2. with `highlight.NewANSIRenderer(highlight.WithThemeFile("<a VS Code theme JSON that defines markup.inserted, markup.deleted and markup.changed>"))`, such as a theme from a VS Code theme extension's `themes/` directory.
- **Pass when**:
  - With the default theme, the `+` markers and paths are green, `-` red, and `~` and `-/+` yellow. Comment lines are grey italic, and values and headers are coloured as configuration.
  - With the VS Code theme, the markers take that theme's diff colours.
  - Added, removed and changed lines are easy to tell apart in both cases.
  - Colour does not run past the end of a line.
- **Who / when**: a maintainer, once before release.

## 4. Review: "diff", never "plan"

- **What to look at**: the new public names and doc comments in `xclconfig: diff/render.go` (`Render`, `RenderOption`, `Highlight`), `xclconfig: highlight/highlight.go` (`ScopeInserted`, `ScopeDeleted`, `ScopeChanged`) and `highlight/doc.go`, and the page `xcl-website: src/pages/diff.mdx`.
- **How**: read each file. As a quick aid, run `grep -in plan diff/render*.go highlight/highlight.go highlight/doc.go` in xclconfig and `grep -in plan src/pages/diff.mdx` in xcl-website.
- **Pass when**: the feature is always called a diff, and no name, doc comment or page text calls it a plan.
- **Who / when**: the reviewer, before merging.
