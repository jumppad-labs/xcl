package highlight

import (
	"io"
	"os"
	"strconv"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// ANSIRenderer is the built-in terminal renderer. It wraps each labelled
// piece in SGR escape codes taken from its theme: 24-bit colour for a VS Code
// theme's colours, the standard 16 foreground colours for the default theme,
// and bold, italic, underline and strikethrough as the theme sets them.
// Unlabelled text, and pieces no rule in the theme matches, are written with
// no codes at all, so they keep the terminal's default colour. A piece that
// spans lines, such as a heredoc or a block comment, is styled one line at a
// time with each newline left outside the codes, so a caller can indent or
// prefix lines without colour leaking.
//
// Create one with NewANSIRenderer and pass it to xcl.Highlight or Text. It is
// safe for concurrent use.
type ANSIRenderer struct {
	theme *theme
}

// ansiOptions holds what the ANSIOption functions configure
type ansiOptions struct {
	source string
	reader io.Reader
	path   string
}

// ANSIOption configures NewANSIRenderer. Construct one with WithTheme or
// WithThemeFile.
type ANSIOption func(*ansiOptions)

// WithTheme colours text with the VS Code colour theme read from theme. JSON
// with comments and trailing commas is accepted, as VS Code accepts it. Only
// the theme's tokenColors are used.
func WithTheme(theme io.Reader) ANSIOption {
	return func(o *ansiOptions) {
		o.source = "theme"
		o.reader = theme
		o.path = ""
	}
}

// WithThemeFile colours text with the VS Code colour theme in the file at
// path, read when the renderer is created
func WithThemeFile(path string) ANSIOption {
	return func(o *ansiOptions) {
		o.source = path
		o.reader = nil
		o.path = path
	}
}

// NewANSIRenderer returns a terminal renderer. With no option it uses the
// default theme, which uses only the terminal's 16 standard colours so the
// result follows the user's own palette. Given a theme, it colours text the
// way the editor colours it with that theme.
//
// A theme that cannot be read or is invalid returns an error matching
// xcl.ErrInvalidTheme, with an InvalidThemeError detail saying what was
// wrong. A bad theme is never replaced with the default colours.
func NewANSIRenderer(options ...ANSIOption) (*ANSIRenderer, error) {
	var opts ansiOptions
	for _, option := range options {
		option(&opts)
	}

	switch {
	case opts.path != "":
		file, err := os.Open(opts.path)
		if err != nil {
			return nil, &xclerrors.InvalidThemeError{Source: opts.path, Reason: "it cannot be read", Err: err}
		}
		defer file.Close()

		theme, err := parseTheme(opts.source, file)
		if err != nil {
			return nil, err
		}

		return &ANSIRenderer{theme: theme}, nil
	case opts.reader != nil:
		theme, err := parseTheme(opts.source, opts.reader)
		if err != nil {
			return nil, err
		}

		return &ANSIRenderer{theme: theme}, nil
	}

	return &ANSIRenderer{theme: defaultTheme()}, nil
}

// Render returns text wrapped in the SGR codes of the style scope gets, or
// text unchanged when scope is empty or the theme gives it no style
func (r *ANSIRenderer) Render(scope, text string) string {
	if scope == "" {
		return text
	}

	parameters := sgrParameters(r.theme.style(scope))
	if parameters == "" {
		return text
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "\x1b[" + parameters + "m" + line + "\x1b[0m"
		}
	}

	return strings.Join(lines, "\n")
}

// sgrParameters returns the SGR parameters for a style, such as "1;35", or
// "" for a style that sets nothing
func sgrParameters(s style) string {
	var parameters []string

	if s.fontStyle != nil {
		if s.fontStyle.bold {
			parameters = append(parameters, "1")
		}
		if s.fontStyle.italic {
			parameters = append(parameters, "3")
		}
		if s.fontStyle.underline {
			parameters = append(parameters, "4")
		}
		if s.fontStyle.strikethrough {
			parameters = append(parameters, "9")
		}
	}

	if c := s.colour; c != nil {
		if c.truecolour {
			parameters = append(parameters, "38", "2",
				strconv.Itoa(int(c.r)), strconv.Itoa(int(c.g)), strconv.Itoa(int(c.b)))
		} else {
			parameters = append(parameters, strconv.Itoa(c.basic))
		}
	}

	return strings.Join(parameters, ";")
}
