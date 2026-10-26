package highlight

import (
	"io"
	"strings"
)

// HTML writes the tokens as one preformatted block for a web page,
// <pre class="xcl">, with each token that is not Text in a span carrying its
// kind's class and, when the kind belongs to a category, the category's class
// first, such as <span class="xcl-comment xcl-line-comment">. Text is written
// without a span, and every token's text is escaped. The theme is ignored: a
// website styles the classes with its own stylesheet, or one WriteCSS
// generates from a theme. The class names are a public contract, the same in
// every language xcl is used from.
var HTML Formatter = FormatterFunc(func(w io.Writer, _ Theme, tokens []Token) error {
	out := &writer{w: w}

	out.write(`<pre class="xcl">`)

	for _, token := range tokens {
		classes := classesOf(token.Kind)
		if classes == "" {
			out.write(escapeHTML(token.Text))
			continue
		}

		out.write(`<span class="` + classes + `">` + escapeHTML(token.Text) + `</span>`)
	}

	out.write(`</pre>`)

	return out.err
})

// HTMLInline writes the same markup as HTML, but styled from the theme with
// style attributes instead of classes, for places a stylesheet cannot reach,
// such as an email. A token that the theme styles with nothing, and every
// Text token, is written without a span; the block itself carries Text's
// style, when the theme gives it one.
var HTMLInline Formatter = FormatterFunc(func(w io.Writer, theme Theme, tokens []Token) error {
	out := &writer{w: w}

	if declarations := cssDeclarations(theme.Style(Text)); len(declarations) > 0 {
		out.write(`<pre style="` + escapeHTML(strings.Join(declarations, "; ")) + `">`)
	} else {
		out.write(`<pre>`)
	}

	for _, token := range tokens {
		declarations := cssDeclarations(theme.Style(token.Kind))
		if token.Kind == Text || len(declarations) == 0 {
			out.write(escapeHTML(token.Text))
			continue
		}

		out.write(`<span style="` + escapeHTML(strings.Join(declarations, "; ")) + `">` + escapeHTML(token.Text) + `</span>`)
	}

	out.write(`</pre>`)

	return out.err
})

// classesOf returns the classes of a kind's span: its category's class, when
// it belongs to one, then its own. Text, and a kind with no class, has none.
func classesOf(kind Kind) string {
	if kind == Text {
		return ""
	}

	class := kind.Class()
	if class == "" {
		return ""
	}

	parent := kind.Parent()
	if _, known := kinds[kind]; !known || parent == kind {
		return class
	}

	if parentClass := parent.Class(); parentClass != "" {
		return parentClass + " " + class
	}

	return class
}

// cssDeclarations returns the CSS declarations of a style, such as
// "color: #cd3131", in the order colour, background colour, bold, italic,
// underline and dim
func cssDeclarations(s Style) []string {
	var declarations []string

	if s.Foreground != "" {
		declarations = append(declarations, "color: "+s.Foreground.Hex())
	}
	if s.Background != "" {
		declarations = append(declarations, "background-color: "+s.Background.Hex())
	}
	if s.Bold {
		declarations = append(declarations, "font-weight: bold")
	}
	if s.Italic {
		declarations = append(declarations, "font-style: italic")
	}
	if s.Underline {
		declarations = append(declarations, "text-decoration: underline")
	}
	if s.Dim {
		declarations = append(declarations, "opacity: 0.5")
	}

	return declarations
}

// htmlEscaper escapes text for HTML and XML, in content and in attributes
var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

// escapeHTML escapes text for HTML and XML
func escapeHTML(text string) string {
	return htmlEscaper.Replace(text)
}
