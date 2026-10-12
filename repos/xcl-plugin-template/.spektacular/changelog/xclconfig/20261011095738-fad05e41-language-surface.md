---
created_date: "2026-10-11"
document_status: draft
project: xclconfig
spec: 20261011095738-fad05e41-language-surface
plan: 20261011095738-fad05e41-language-surface
---

# The template offers an example function

The template now builds against xcl v0.3.0 and shows how a plugin offers a function to configuration. The note provider offers `note_filename(title)`, which turns a title into a safe `.txt` file name, and the sample configuration names its note with it: `name = note_filename("Greeting")`. The README's new "Add a function" section walks an author through adding a function of their own, and the tests show the function working in process and as a separate plugin program.

> Derived from project xclconfig (file), spec/plan 20261011095738-fad05e41-language-surface. See the project-level record for the full feature.

## What changed in this repo

- `go.mod`/`go.sum`: `github.com/jumppad-labs/xcl` moved from v0.1.1 to v0.3.0 (pulling in `github.com/ProtonMail/go-crypto` and `github.com/cloudflare/circl` as indirect dependencies).
- `providers/note.go`: `NewNoteProvider` builds the provider's functions once with `plugins.NewFunctions` and `plugins.NewFunction` through a `mustFunction` helper; `Functions` returns them and stays the last method; `noteFilename` sits with the unexported helpers and refuses a title with no letters or digits.
- `examples/basic/main.xcl`: the note's `name` comes from `note_filename("Greeting")`, so the file is still `greeting.txt`.
- `README.md`: the "Add a function" section, between "Add your own resource" and "Checks".
- `providers/note_test.go`: the function is tested through `Functions()`; `e2e/inprocess_test.go` and `e2e/external_test.go` assert the applied note holds the function's result and that the function's error reaches `Apply`; `e2e/changes_test.go` gained the `untitledNote` config.

## Why

xcl v0.3.0 lets a plugin's providers offer functions that configuration calls by name. The template is the worked example of the standard plugin layout, so it has to show the feature in the place the layout puts it, `Functions` last in the provider, and give an author a README path to follow alone.
