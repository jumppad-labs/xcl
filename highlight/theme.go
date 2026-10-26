package highlight

import (
	"errors"
	"maps"
	"slices"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// Theme says how each kind of text looks: a style string for each kind it
// styles, such as "bold #ff8800". A kind it does not style takes its
// category's style, and a category it does not style takes Text's, so a
// short theme styles everything:
//
//	theme := highlight.DefaultTheme.With(highlight.Theme{
//		highlight.Attribute: "#ff8800",
//		highlight.Comment:   "italic #6a9955",
//	})
//
// A theme is plain data: build one as a map literal, extend one with With,
// or load one from a VS Code colour theme with LoadVSCodeTheme.
type Theme map[Kind]string

// DefaultTheme is xcl's own look. It uses only the terminal's 16 named
// colours, so in a terminal it follows the user's own palette, and leaves
// Text unstyled, so text keeps the destination's own colours.
var DefaultTheme = Theme{
	Block:        "bold magenta",
	Label:        "green",
	TypeLabel:    "cyan",
	Attribute:    "blue",
	String:       "yellow",
	StringEscape: "bright-magenta",
	Number:       "bright-magenta",
	Constant:     "bright-magenta",
	Reference:    "cyan",
	Comment:      "italic bright-black",
	Inserted:     "green",
	Deleted:      "red",
	Changed:      "yellow",
	Error:        "bold red",
	Warning:      "bold yellow",
	Summary:      "bold",
	Gutter:       "dim",
}

// With returns a new theme holding t's styles with overrides' in their place,
// for the kinds overrides styles. Neither t nor overrides is changed.
func (t Theme) With(overrides Theme) Theme {
	extended := make(Theme, len(t)+len(overrides))
	maps.Copy(extended, t)
	maps.Copy(extended, overrides)

	return extended
}

// Style returns how kind looks: its own style when the theme gives it one,
// else its category's, else Text's. A style is taken whole, never merged
// with its category's. Words a style does not understand are skipped.
func (t Theme) Style(kind Kind) Style {
	if written := t[kind]; written != "" {
		return ParseStyle(written)
	}

	if written := t[kind.Parent()]; written != "" {
		return ParseStyle(written)
	}

	return ParseStyle(t[Text])
}

// Validate reports every word of the theme's styles that a style does not
// understand, as an InvalidStyleError for each, joined, in the order of the
// kinds and then of the words. It returns nil for a theme it finds nothing
// wrong with. Printing with a theme that does not validate still works: the
// words are skipped.
func (t Theme) Validate() error {
	var problems []error

	for _, kind := range t.kindsInOrder() {
		written := t[kind]
		for _, word := range unknownWords(written) {
			problems = append(problems, &xclerrors.InvalidStyleError{Kind: kind.String(), Style: written, Word: word})
		}
	}

	return errors.Join(problems...)
}

// kindsInOrder returns the kinds the theme styles, in numeric order
func (t Theme) kindsInOrder() []Kind {
	ordered := make([]Kind, 0, len(t))
	for kind := range t {
		ordered = append(ordered, kind)
	}

	slices.Sort(ordered)

	return ordered
}
