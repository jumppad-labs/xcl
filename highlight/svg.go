package highlight

import (
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The geometry of SVG output, in user units: a monospace font 10 high, a
// character 6 wide, a line 14 high and a margin of 8 all round
const (
	svgMargin     = 8
	svgCharWidth  = 6
	svgLineHeight = 14
	svgBaseline   = 11
)

// SVG writes the tokens as a standalone SVG image: monospace text, one
// <text xml:space="preserve"> element per line with a <tspan> for each styled
// token, its colours and attributes from the theme. The view box fits the
// longest line and every line. Text's foreground colours the whole image,
// and Text's background, when the theme gives one, fills it; a token's own
// background is not drawn. An image is usually shown where no stylesheet
// applies, so it carries its colours itself.
var SVG Formatter = FormatterFunc(func(w io.Writer, theme Theme, tokens []Token) error {
	lines := svgLines(tokens)

	columns := 0
	for _, line := range lines {
		width := 0
		for _, piece := range line {
			width += utf8.RuneCountInString(piece.Text)
		}

		columns = max(columns, width)
	}

	width := strconv.Itoa(2*svgMargin + svgCharWidth*columns)
	height := strconv.Itoa(2*svgMargin + svgLineHeight*len(lines))

	out := &writer{w: w}

	text := theme.Style(Text)

	out.write(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + width + ` ` + height + `" font-family="monospace" font-size="10"`)
	if text.Foreground != "" {
		out.write(` fill="` + text.Foreground.Hex() + `"`)
	}
	out.write(">\n")

	if text.Background != "" {
		out.write(`<rect width="` + width + `" height="` + height + `" fill="` + text.Background.Hex() + `"/>` + "\n")
	}

	for i, line := range lines {
		y := strconv.Itoa(svgMargin + svgBaseline + svgLineHeight*i)
		out.write(`<text x="` + strconv.Itoa(svgMargin) + `" y="` + y + `" xml:space="preserve">`)

		for _, piece := range line {
			attributes := svgAttributes(theme.Style(piece.Kind))
			if piece.Kind == Text || attributes == "" {
				out.write(escapeHTML(piece.Text))
				continue
			}

			out.write(`<tspan` + attributes + `>` + escapeHTML(piece.Text) + `</tspan>`)
		}

		out.write("</text>\n")
	}

	out.write("</svg>\n")

	return out.err
})

// svgLines splits the tokens into lines, each the pieces of the tokens on it,
// without the line breaks. A final line break ends the last line rather than
// starting an empty one.
func svgLines(tokens []Token) [][]Token {
	lines := [][]Token{nil}

	for _, token := range tokens {
		for i, part := range strings.Split(token.Text, "\n") {
			if i > 0 {
				lines = append(lines, nil)
			}

			if part != "" {
				last := len(lines) - 1
				lines[last] = append(lines[last], Token{Kind: token.Kind, Text: part})
			}
		}
	}

	if len(lines) > 1 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// svgAttributes returns the attributes of a tspan with a style, each with a
// leading space, in the order fill, bold, italic, underline and dim, or ""
// for a style SVG text shows nothing of
func svgAttributes(s Style) string {
	var attributes strings.Builder

	if s.Foreground != "" {
		attributes.WriteString(` fill="` + s.Foreground.Hex() + `"`)
	}
	if s.Bold {
		attributes.WriteString(` font-weight="bold"`)
	}
	if s.Italic {
		attributes.WriteString(` font-style="italic"`)
	}
	if s.Underline {
		attributes.WriteString(` text-decoration="underline"`)
	}
	if s.Dim {
		attributes.WriteString(` opacity="0.5"`)
	}

	return attributes.String()
}
