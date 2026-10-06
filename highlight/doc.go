// Package highlight labels xcl configuration text for display.
//
// Text splits configuration into pieces and labels each one with the
// TextMate scope the xcl-vscode grammar gives the same text, such as
// storage.type.xcl for a block type or variable.other.property.xcl for an
// attribute name. Every piece is handed to a Renderer, which decides how it
// is written: wrapped in terminal colour codes, in markup, or in any other
// form. Text between tokens, and tokens the grammar leaves unlabelled, reach
// the renderer too, with the empty scope, so a renderer sees every byte of
// the input exactly once and in order. Joining the pieces unchanged gives
// back the input, so highlighting never changes the text itself.
//
// NewANSIRenderer returns the built-in terminal renderer. The package never
// checks whether output is a terminal: whether to colour is the caller's
// decision.
package highlight
