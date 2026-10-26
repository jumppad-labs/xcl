package highlight

import (
	"io"
	"strconv"
	"strings"
)

// Terminal writes the tokens for a terminal: each in the SGR escape codes of
// its style from the theme, the 16 named colours as the terminal's own
// palette and hex colours as 24-bit colour. Text styled with nothing is
// written with no codes at all. A token that spans lines, such as a heredoc
// or a block comment, is styled one line at a time, with a reset before every
// line break, so indenting or prefixing a line never bleeds colour.
//
// Terminal always writes colour: whether to use it is the caller's choice,
// which For makes the usual way.
var Terminal Formatter = FormatterFunc(func(w io.Writer, theme Theme, tokens []Token) error {
	out := &writer{w: w}

	for _, token := range tokens {
		parameters := sgr(theme.Style(token.Kind))
		if parameters == "" {
			out.write(token.Text)
			continue
		}

		lines := strings.Split(token.Text, "\n")
		for i, line := range lines {
			if i > 0 {
				out.write("\n")
			}

			if line != "" {
				out.write("\x1b[" + parameters + "m" + line + "\x1b[0m")
			}
		}
	}

	return out.err
})

// sgr returns the SGR parameters of a style, such as "1;35": its attributes
// in the order bold, dim, italic, underline, then its foreground and its
// background colour. A style that sets nothing gives "".
func sgr(s Style) string {
	var parameters []string

	if s.Bold {
		parameters = append(parameters, "1")
	}
	if s.Dim {
		parameters = append(parameters, "2")
	}
	if s.Italic {
		parameters = append(parameters, "3")
	}
	if s.Underline {
		parameters = append(parameters, "4")
	}

	if s.Foreground != "" {
		if named, ok := namedColours[s.Foreground]; ok {
			parameters = append(parameters, strconv.Itoa(named.code))
		} else {
			parameters = append(parameters, trueColour("38", s.Foreground))
		}
	}

	if s.Background != "" {
		if named, ok := namedColours[s.Background]; ok {
			parameters = append(parameters, strconv.Itoa(named.code+10))
		} else {
			parameters = append(parameters, trueColour("48", s.Background))
		}
	}

	return strings.Join(parameters, ";")
}

// trueColour returns the SGR parameters setting a 24-bit colour, with prefix
// 38 for the foreground or 48 for the background
func trueColour(prefix string, c Colour) string {
	r, g, b := c.rgb()

	return prefix + ";2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b)
}
