---
created_date: "2026-10-06"
document_status: draft
project: xclconfig
spec: 20261006112108-17623cda-encoder-syntax-highlighting
plan: 20261006112108-17623cda-encoder-syntax-highlighting
---

# Encoder syntax highlighting

Configuration text written by xcl can now be coloured. Ask for it when encoding an entity or its saved data and every token is labelled with the same name the xcl VS Code extension uses, then rendered by a built-in terminal renderer, by any VS Code colour theme, or by a renderer you write for any other format. Output is unchanged unless you ask, and highlighting never alters the text itself. The logging example now uses this instead of its own highlighter.

> Derived from project xcl (jumppad-labs/xcl), spec/plan 20261006112108-17623cda-encoder-syntax-highlighting. See the project-level record for the full feature.

## What changed

- New public package `highlight`: `Renderer`, `RendererFunc`, `Text` and `Scope*` constants matching the xcl-vscode grammar scopes; an unexported tokeniser built on the HCL scanner.
- New `highlight.NewANSIRenderer` with `WithTheme` and `WithThemeFile`: a 16-colour default theme, 24-bit colours and font styles from VS Code themes, no codes for unmatched or unlabelled text, multi-line tokens styled per line.
- New `xcl.Highlight(renderer)` encode option for `EncodeEntity` and `EncodeSavedEntity`; without it output is unchanged.
- New `xcl.ErrInvalidTheme` and `xcl.InvalidThemeError` for themes that cannot be read or are invalid.
- `example/prettylog` no longer has its own highlighter; it passes `xcl.Highlight` when its writer shows colour.
- Test helper `internal/testutil.StripANSI`.
- No new module dependencies; the library still does not depend on charmbracelet. Breaking changes: none.

## Why

Applications had to write their own highlighter to show configuration in colour. Putting it in the library gives every application the editor's look from one option, keeps the colour decision with the caller, and lets the example drop its own code.
