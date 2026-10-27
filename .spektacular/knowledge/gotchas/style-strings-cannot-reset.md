---
tags: [highlight, vscode, themes]
---

# A style string cannot say "explicitly unstyled"

xcl themes are `map[Kind]string` of style strings, and a kind with no (or an
empty) style falls back to its category, then to `Text`. There is no word for
"nothing, on purpose", so a style can never switch off what its category sets
except by setting something else.

Consequence for `highlight.LoadVSCodeTheme`: a VS Code rule that resets
`fontStyle` to `""` without also setting a colour converts to an empty style,
which `LoadVSCodeTheme` leaves out, so the kind inherits its category's style
(for example a line comment stays italic from `Comment`) where VS Code would
show it plain. A reset that comes with a colour converts correctly. A
`strikethrough` font style has no style word either and is dropped. Fixing
either needs a new style word, which is a change to the design's style syntax
shared by every language, not a Go-only change.
