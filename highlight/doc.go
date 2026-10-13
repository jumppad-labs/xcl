// Package highlight turns labelled text into output, plain or coloured.
//
// Producing output and styling it are two steps. A producer lays its output
// out as Pieces: runs of text, each labelled with a TextMate scope. A Renderer
// then turns the pieces into bytes, given all of them at once:
//
//	text, _ := xcl.EncodeEntity(entity)
//	os.Stdout.Write(highlight.ANSI.Render(highlight.Pieces(text)))
//
//	os.Stdout.Write(highlight.Text.Render(changes.Pieces()))
//
// Pieces labels xcl configuration text with the scopes the xcl-vscode grammar
// gives it, such as storage.type.xcl for a block type or
// variable.other.property.xcl for an attribute name. A diff labels its own
// output, with ScopeInserted, ScopeDeleted and ScopeChanged for what changes
// and the configuration scopes for headers and values. Text between tokens,
// and text nothing labels, is a piece too, with the empty scope, so the pieces
// hold every byte exactly once and in order, and rendering can add to the
// text but never change it.
//
// Text renders plain text, ANSI renders for a terminal with the default theme,
// and NewANSI makes a terminal renderer with a theme of your own. Any other
// format is a Renderer of its own; RendererFunc and PieceFunc turn a function
// into one. The package never checks whether output is a terminal: whether to
// colour is the caller's decision.
package highlight
