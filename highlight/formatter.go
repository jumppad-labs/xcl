package highlight

import (
	"io"
)

// Formatter turns labelled text into output on a writer: plain text, terminal
// colour, HTML, SVG or any other format. It is given every token at once, in
// order, so it can reason across them, such as by line or for a whole
// document, and the theme to style them with, which it may ignore. The tokens
// hold every byte of the text, Text tokens included, so a format that needs
// escaping sees all of it. Format returns the first error writing to w.
type Formatter interface {
	Format(w io.Writer, theme Theme, tokens []Token) error
}

// FormatterFunc lets a function act as a Formatter
type FormatterFunc func(w io.Writer, theme Theme, tokens []Token) error

// Format calls f
func (f FormatterFunc) Format(w io.Writer, theme Theme, tokens []Token) error {
	return f(w, theme, tokens)
}

// Plain writes the text of the tokens with nothing added, ignoring the theme
var Plain Formatter = FormatterFunc(func(w io.Writer, _ Theme, tokens []Token) error {
	out := &writer{w: w}
	for _, token := range tokens {
		out.write(token.Text)
	}

	return out.err
})

// writer writes strings to w, keeping the first error and writing nothing
// after it, so a formatter checks for an error once, at the end
type writer struct {
	w   io.Writer
	err error
}

// write writes text unless an earlier write failed
func (o *writer) write(text string) {
	if o.err != nil || text == "" {
		return
	}

	_, o.err = io.WriteString(o.w, text)
}
