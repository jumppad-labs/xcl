---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Encoder syntax highlighting

## What was built

xcl can now colour the configuration text it writes. A new public `highlight` package splits configuration text into pieces with xcl's own HCL scanner and labels each one with the TextMate scope the xcl VS Code extension's grammar gives the same text (`storage.type.xcl`, `variable.other.property.xcl`, `support.class.reference.xcl` and so on, all exported as `highlight.Scope*` constants). Every piece, labelled or not, is handed in order to a one-method `Renderer` (`Render(scope, text string) string`, with a `RendererFunc` adapter), so any output format can be produced and the text itself never changes. `highlight.Text` highlights any configuration text.

The package ships a terminal renderer, `highlight.NewANSIRenderer`. With no theme it uses only the terminal's 16 standard colours; with `WithTheme(io.Reader)` or `WithThemeFile(path)` it reads a VS Code colour theme (JSON with comments and trailing commas) and colours tokens as the editor would, in 24-bit colour with bold, italic, underline and strikethrough. Theme matching follows TextMate: dot-segment prefix selectors, most specific wins, later rule wins a tie, colour and font style resolved independently. Unmatched and unlabelled text gets no codes, and multi-line tokens are styled one line at a time. A theme that cannot be read or is invalid fails the constructor with the new `xcl.ErrInvalidTheme` (detail `xcl.InvalidThemeError`, naming the theme and the path to the bad entry).

`xcl.EncodeEntity` and `xcl.EncodeSavedEntity` accept a new `xcl.Highlight(renderer)` option that runs the finished text through the renderer, including text written with `ShowReferences`. Without it, or with a nil renderer, output is byte-for-byte as before.

The prettylog example dropped its own regex highlighter and asks the encoder for highlighting whenever its writer shows colour. The website's configuration-text page has a new "Highlighting" section, and the events page's prettylog snippet was brought up to date.

## Why it matters

Before this, an application wanting coloured configuration had to write its own highlighter, as the logging example did. Configuration in an application's output now looks the way it does in the editor, from the same token names and theme format, and developers can plug in any other output format with one small interface. The library still never checks for a terminal and gains no new module dependency.

## Deviations from the plan

- Scopes follow the xcl-vscode working-tree grammar (uncommitted edits making any identifier a labelled block type or reference root), as planned.
- Adjacent pieces with the same scope reach the renderer as one piece. Identifiers and whitespace inside `${ }` that no expression rule matches are passed with an empty scope rather than `meta.interpolation.xcl`. `$${` in a quoted string is treated as text.
- `#RGBA` theme colours are accepted too; a theme without `tokenColors` loads and styles nothing; malformed entries and scopes are rejected.
- `xcl.InvalidThemeError` is aliased from the root package as well as the sentinel.
- prettylog had already become its own module (with a local `replace`), so it was migrated there; its stale `ExamplePlugin` doc line was replaced.
- The website's `events.mdx` prettylog snippet, reported stale by an earlier spec, was fixed here.
