---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Test plan: 20261023074926-eaa51527-printing-and-highlighting-core

Both success metrics are covered by automated behavioural tests:

- *Showing a configuration problem with its source takes one call in Go*: `TestPrintErrorReportsAnUnsupportedBlockWithItsSourceInOneCall` in `xcl/print_error_test.go`.
- *No example program prints a configuration error as a single line*: the broken-configuration tests in `xcl/example/configonly/smoke_test.go`, `xcl/example/plugin/smoke_test.go` (Docker-gated) and `xcl/example/remoteplugin/main_test.go`.

What follows are the four manual reviews the plan lists. Paths are relative to the xcl repository (`xcl/`) or the website repository (`xcl-website/`) on the merged branch.

## 1. Release notes list a replacement for every removed function

- **Who / when:** the reviewer of the merge, before the next release is tagged.
- **Where:** `xcl/CHANGELOG.md`, the top section `## 20261023074926-eaa51527-printing-and-highlighting-core`, its `**Breaking:**` list.
- **What to check:** there is a one-line replacement for each of these: `highlight.Piece`, `highlight.Pieces`, `highlight.Renderer`, `highlight.RendererFunc`, `highlight.PieceFunc`, `highlight.Text` (the renderer), `highlight.ANSI`, `highlight.ANSIRenderer`, `highlight.NewANSI`, `highlight.ANSIOption`, `highlight.WithTheme(io.Reader)`, `highlight.WithThemeFile`, the `highlight.Scope*` constants, `xcl.DiagnosticPieces` and `diff.(*Diff).Pieces`. Each replacement names a function that exists, which you can confirm with `go doc github.com/jumppad-labs/xcl/highlight` and `go doc github.com/jumppad-labs/xcl`.
- **Pass:** every removed name has a correct replacement line, and no replacement names a function that does not exist.

## 2. The website documents the new printing and builds

- **Who / when:** the website maintainer, before the website change is published.
- **How:**
  ```shell
  cd xcl-website
  npm ci
  npm run build
  npx astro check
  npm run preview
  ```
- **Where to look:**
  - `/configuration-text/#printing-and-highlighting`: its subsections are Output formats, Themes, With an editor theme, On a website (the class table and `WriteCSS`) and Writing a format.
  - `/problems/#showing-problems-to-a-person`: wrapped errors, colour, `%+v` and "In logs".
  - `/diff/`: printing, Colour and writing your own format.
  - `/events/`: Errors in logs, and the prettylog description.
  - `/sensitive-values/`.
  - `/examples/viper/`: When the file is wrong.
  - `/examples/plugins/`: the main, plan and inspect listings, and the broken-configuration report.
- **What to check:**
  - Printing errors, resources and change summaries, output formats, themes, class names, stylesheet generation, custom formats and logging integration each have a Go example.
  - A search of the site for `Pieces`, `PieceFunc`, `Renderer`, `NewANSI`, `highlight.ANSI`, `WithThemeFile`, `Scope` and `DiagnosticPieces` finds no xcl API documented.
  - Every page in the navigation opens.
  - The `#printing-and-highlighting` and `#showing-problems-to-a-person` links land on their headings.
- **Pass:** the build and `astro check` report no errors, every topic has its Go example, no removed function is documented, and no existing URL is broken.

## 3. The reference outputs look right

- **Who / when:** the reviewer of the merge, and again whenever `go test ./highlight/ -run TestReference -update` changes the files.
- **How:**
  1. In `xcl/highlight/testdata/reference/`, open `report.svg`, `entity.svg` and `diff.svg` in a browser.
  2. Make a local page that links `default.css` and pastes in the contents of `report.html`, `entity.html` and `diff.html`, and open it.
  3. Open `report.inline.html`, `entity.inline.html` and `diff.inline.html` directly.
  4. In a real terminal, run `cat report.terminal.txt entity.terminal.txt diff.terminal.txt`.
  5. Build any example, for instance `cd xcl/example/configonly && go build -o /tmp/configonly .`, and run `/tmp/configonly testdata/broken`. Run it once in a terminal, once with `NO_COLOR=1`, and once piped through `| cat`.
- **What to check:**
  - The SVGs show every line, uncut, in the default colours: magenta bold block types, blue attributes, yellow strings, italic grey comments, and a bold red `Error` with the fault marked.
  - The HTML with the stylesheet and the inline HTML look the same as each other and match the SVGs.
  - Terminal colours never run past the end of a line.
  - `highlight.For` gives colour on a terminal and plain text with `NO_COLOR` set or when piped.
- **Pass:** every rendering is readable and coloured as described, and the `NO_COLOR` and piped runs contain no escape codes.

## 4. Each example shows a readable, coloured report for a broken configuration

- **Who / when:** the reviewer of the merge, in a real terminal.
- **How:**
  - `cd xcl/example/configonly && go run . ./testdata/broken`
  - `cd xcl/example/plugin && make build && ./build/xcl-docker plan --state "$(mktemp -d)" ./testdata/broken`. This needs a Docker engine.
  - `cd xcl/example/remoteplugin`: point it at a configuration with a mistake, such as `./testdata/broken`, with `go run . apply --state "$(mktemp -d)" ./testdata/broken`. This needs network access to the published release key and the plugin release.
- **What to check:**
  - stderr shows `Error: …` in bold red and the location line naming the file, line, block and field.
  - The numbered source lines show two lines of context either side, with the fault in red, and the detail follows.
  - None of them prints a single `error: …` line for the configuration problem.
  - An application error, such as `go run . ./testdata/unknown_service` in `configonly`, still prints as one line.
- **Pass:** all three examples print the report, coloured on a terminal, and plain when piped.
