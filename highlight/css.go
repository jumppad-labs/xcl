package highlight

import (
	"io"
	"strings"
)

// WriteCSS writes a stylesheet styling the classes the HTML formatter writes
// as theme styles their kinds, as a starting point for a website's own
// styles. Every rule is scoped under the .xcl container: a rule for Text's
// style on .xcl itself, then one for each kind the theme styles, categories
// before the kinds in them, so a kind's own rule wins over its category's.
// Named colours are written through the fixed palette as hex, and dim as
// opacity. A kind's rule resets what its category's rule sets and its own
// style does not, since a kind's style replaces its category's whole.
func WriteCSS(w io.Writer, theme Theme) error {
	out := &writer{w: w}

	if declarations := cssDeclarations(ParseStyle(theme[Text])); len(declarations) > 0 {
		out.write(".xcl { " + strings.Join(declarations, "; ") + "; }\n")
	}

	for _, kind := range orderedKinds {
		written := theme[kind]
		if kind == Text || written == "" {
			continue
		}

		reference := theme[Text]
		if parent := kind.Parent(); parent != kind {
			reference = theme[parent]
		}

		style := ParseStyle(written)
		declarations := append(cssDeclarations(style), cssResets(style, ParseStyle(reference))...)
		if len(declarations) == 0 {
			continue
		}

		out.write(".xcl ." + kind.Class() + " { " + strings.Join(declarations, "; ") + "; }\n")
	}

	return out.err
}

// cssResets returns the declarations putting back what inherited sets and
// style does not
func cssResets(style, inherited Style) []string {
	var resets []string

	if inherited.Foreground != "" && style.Foreground == "" {
		resets = append(resets, "color: inherit")
	}
	if inherited.Background != "" && style.Background == "" {
		resets = append(resets, "background-color: transparent")
	}
	if inherited.Bold && !style.Bold {
		resets = append(resets, "font-weight: normal")
	}
	if inherited.Italic && !style.Italic {
		resets = append(resets, "font-style: normal")
	}
	if inherited.Underline && !style.Underline {
		resets = append(resets, "text-decoration: none")
	}
	if inherited.Dim && !style.Dim {
		resets = append(resets, "opacity: 1")
	}

	return resets
}
