// Package highlight turns labelled text into output: plain text, terminal
// colour, HTML or SVG.
//
// Three things are kept apart. Tokens say what text is: a Kind, such as
// BlockType, Attribute or LineComment, and the text. A Theme says how each
// kind looks, as short style strings such as "bold #ff8800". A Formatter
// writes tokens to an io.Writer, given all of them at once and a theme:
//
//	tokens := highlight.Tokenize(text)
//	highlight.Terminal.Format(os.Stdout, highlight.DefaultTheme, tokens)
//
// Most programs never call a formatter themselves: xcl.PrintError,
// xcl.PrintEntity and xcl.PrintDiff print an error, a resource or a change
// summary in one call, taking the formatter and theme as options.
//
// Tokenize labels xcl configuration text with the kinds of text the
// xcl-vscode grammar recognises. A change summary labels its own output, with
// Inserted, Deleted and Changed for what changes, and an error report labels
// its severity, summary, location, gutter and detail with the Report kinds.
// Text between tokens, and text nothing labels, is a Text token, so the
// tokens hold every byte exactly once and in order, and formatting can add to
// the text but never change it.
//
// Kinds are numbered in blocks of 100 and each falls back to its category,
// its Parent, so a theme that styles Comment styles every kind of comment
// until a theme styles LineComment itself. DefaultTheme is xcl's own look;
// With extends a theme, Validate reports style words it does not understand,
// and LoadVSCodeTheme loads a VS Code colour theme.
//
// Plain, Terminal, HTML, HTMLInline and SVG are the built-in formats; any
// other format is a Formatter of its own, and FormatterFunc turns a function
// into one. The HTML formatter writes classes named after the kinds, and
// WriteCSS generates a stylesheet for them from a theme.
//
// The package never checks whether output is a terminal: whether to colour is
// the caller's decision. For makes the usual choice when asked.
package highlight
