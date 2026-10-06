---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Test plan: encoder syntax highlighting

Manual checks for what the automated suite does not cover. Everything else (scope parity per grammar rule, marker renderer, identity and fuzz preservation, theme matching and errors, ANSI output, encoder integration, prettylog colour on and off) is covered by `go test ./highlight/... ./errors/... .` in xclconfig and `go test ./...` in `example/prettylog`.

All paths are relative to the xclconfig repository root unless a repository is named.

## 1. Text unchanged across every configuration in the repository

**What**: For every `.xcl` file in the examples and test fixtures, highlighting and then stripping the SGR codes gives the file back byte for byte (spec acceptance criterion "Text unchanged").

**How**: From the xclconfig root, create a throwaway program (do not commit it):

```
mkdir -p .tmp/hlsweep && cat > .tmp/hlsweep/main.go <<'EOF'
package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/jumppad-labs/xcl/highlight"
)

func main() {
	sgr := regexp.MustCompile("\x1b\\[[0-9;]*m")
	renderer, err := highlight.NewANSIRenderer()
	if err != nil {
		panic(err)
	}
	failed := 0
	for _, path := range os.Args[1:] {
		src, _ := os.ReadFile(path)
		out := sgr.ReplaceAll(highlight.Text(src, renderer), nil)
		if string(out) != string(src) {
			fmt.Println("CHANGED", path)
			failed++
		}
	}
	fmt.Printf("%d files, %d changed\n", len(os.Args)-1, failed)
}
EOF
go run ./.tmp/hlsweep $(git ls-files '*.xcl' '*.hcl')
rm -r .tmp/hlsweep
```

Repeat once with `highlight.NewANSIRenderer(highlight.WithThemeFile("highlight/testdata/reference-theme.json"))` in place of the default renderer.

**Expected**: the last line reads `N files, 0 changed`, and no `CHANGED` line is printed, for both renderers.

**Who / when**: the implementer or reviewer, before the epic is released.

## 2. Scope parity spot-check in VS Code

**What**: the scopes `highlight` gives match what the editor shows.

**How**: Open `highlight/testdata/sample.xcl` in VS Code with the xcl extension installed from the xcl-vscode working tree (its grammar has uncommitted edits, which this implementation follows). Run "Developer: Inspect Editor Tokens and Scopes" on at least: `resource` (block type), `"network"`, `"onprem"`, `deployment`, `container`, `default`, `2048`, `false`, `len`, `my_function`, the `resource` of `resource.template.other.append_file`, a `.` and member after it, `<<-`, `EOF`, `#{{`, `Vars`, `\t` inside `escaped`, `&&`, and each comment form. Compare the innermost `.xcl` scope the editor shows with the scope in `highlight/tokenize_test.go` for the same text.

**Expected**: every sampled token has the same innermost scope. Known, accepted differences: the editor gives `meta.interpolation.xcl` to identifiers and whitespace inside `${ }` that no other rule matches (e.g. `interpolated`), which `highlight` passes with the empty scope; the editor gives a heredoc closer's leading whitespace the marker scope, which `highlight` leaves unlabelled.

**Who / when**: a reviewer with VS Code, before release.

## 3. Logging example in a real terminal

**What**: configuration printed by prettylog is coloured the way the editor shows it, with no colour leaking past it.

**How**: Run the plugin example in a colour terminal (see `example/plugin/Makefile` or `example/plugin/README` for the Docker prerequisites), e.g. from `example/plugin`: `go run . ` with Docker available. Watch the `create success` lines.

**Expected**:
- The configuration under each `create success` line is coloured with the default 16-colour palette: block type magenta bold, type label cyan, name label green, attribute names blue, strings yellow, numbers and `true`/`false`/`null` bright magenta, reference roots cyan, comments (`# set by the provider`) grey italic. Operators, braces and members stay in the default colour.
- The indentation before each line is uncoloured and the line after the configuration starts in the default colour.
- Re-run with output redirected to a file (`go run . 2> out.log`): `out.log` contains no `\x1b[` sequences.
- Optionally, in a scratch program, encode the same entity with `xcl.Highlight` and `highlight.NewANSIRenderer(highlight.WithThemeFile(<a dark VS Code theme JSON, e.g. Dark+ from the VS Code repo>))` and confirm the colours match the editor with that theme.

**Who / when**: a reviewer on a machine with Docker, before release.

## 4. Website "Highlighting" section review

**What**: the new section reads well and is accurate.

**How**: In the xcl-website repository run `npm install` (if needed) and `npm run dev`, then open `/configuration-text/#highlighting`. Also open `/events/` and check the "A pretty receiver" snippet.

**Expected**: The section shows enabling highlighting with `highlight.NewANSIRenderer()` and `xcl.Highlight`, the caller's own terminal decision, passing a theme with `highlight.WithThemeFile` and handling `xcl.ErrInvalidTheme`, and writing a renderer with `highlight.RendererFunc`. Code blocks render with syntax colouring, the heading anchors work, and the events page snippet shows `Handler(w io.Writer, level slog.Level, reg *registry.PluginRegistry)` and links to the highlighting section.

**Who / when**: the docs owner, before the website is published.

## 5. No other example prints configuration with its own highlighter (success metric)

**What**: "Every place xcl's examples show configuration in a terminal uses the library's highlighting, with no highlighting code of their own."

**How**: From the xclconfig root run `grep -rn "EncodeEntity\|EncodeSavedEntity" example --include='*.go' | grep -v _test.go` and `grep -rln "lipgloss\|regexp" example --include='*.go' | grep -v _test.go`. For every call site found, check that terminal output of configuration passes `xcl.Highlight` (or deliberately prints plain text), and that no file colours configuration text itself.

**Expected**: the only encoder call printing to a terminal with colour is `example/prettylog/prettylog.go`, which uses `xcl.Highlight`; `lipgloss` appears only for log key styles and the colour decision in that file. At implementation time (2026-10-06) that was the only call site; `example/configonly` and `example/plugin` do not encode configuration. The sweep in check 1 was dry-run then and reported `164 files, 0 changed` with the default renderer.

**Who / when**: a reviewer, before release, and again after any sibling spec rewrites an example.

## 6. Library dependency boundary

**What**: the library gains no charmbracelet or other new module dependency.

**How**: From the xclconfig root: `go list -deps ./... | grep -i charm`, `grep -i charm go.mod go.sum`, and `git diff <base>..HEAD -- go.mod go.sum` for the epic's merge base.

**Expected**: both greps print nothing, and `go.mod`/`go.sum` are unchanged by this spec. `highlight` imports only the standard library, `internal/xcl`, `internal/xcl/hclsyntax` and the `errors` package (`go list -f '{{join .Imports "\n"}}' ./highlight`).

**Who / when**: the reviewer of the merge.
