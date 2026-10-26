package highlight

import (
	"strings"
)

// Style is how a kind of text looks, parsed from a style string.
type Style struct {
	Bold      bool
	Italic    bool
	Underline bool
	Dim       bool

	// Foreground and Background are the colours of the text and behind it,
	// or "" to keep the destination's own
	Foreground Colour
	Background Colour
}

// IsZero reports whether the style sets nothing, so text written with it
// looks as it would unstyled
func (s Style) IsZero() bool {
	return s == Style{}
}

// Colour is a colour of a style: one of the 16 named terminal colours, such
// as "red" or "bright-red", a 24-bit colour written "#rrggbb" in lower case,
// or "" for none.
type Colour string

// namedColours are the 16 named colours, each with the SGR foreground code a
// terminal shows it with and the hex colour it is written as elsewhere, such
// as in HTML, SVG and CSS. The hex colours are a fixed palette, the same in
// every language, since the terminal's bright-red has no CSS name.
var namedColours = map[Colour]struct {
	code int
	hex  string
}{
	"black":          {30, "#000000"},
	"red":            {31, "#cd3131"},
	"green":          {32, "#0dbc79"},
	"yellow":         {33, "#e5e510"},
	"blue":           {34, "#2472c8"},
	"magenta":        {35, "#bc3fbc"},
	"cyan":           {36, "#11a8cd"},
	"white":          {37, "#e5e5e5"},
	"bright-black":   {90, "#666666"},
	"bright-red":     {91, "#f14c4c"},
	"bright-green":   {92, "#23d18b"},
	"bright-yellow":  {93, "#f5f543"},
	"bright-blue":    {94, "#3b8eea"},
	"bright-magenta": {95, "#d670d6"},
	"bright-cyan":    {96, "#29b8db"},
	"bright-white":   {97, "#ffffff"},
}

// Hex returns the colour as "#rrggbb": a named colour through the fixed
// palette, a hex colour as it is, and "" for no colour
func (c Colour) Hex() string {
	if named, ok := namedColours[c]; ok {
		return named.hex
	}

	return string(c)
}

// rgb returns the red, green and blue parts of the colour's hex form
func (c Colour) rgb() (int, int, int) {
	hex := c.Hex()
	if len(hex) != 7 {
		return 0, 0, 0
	}

	return hexByte(hex[1:3]), hexByte(hex[3:5]), hexByte(hex[5:7])
}

// ParseStyle reads a style string: words separated by spaces, each one of
// bold, italic, underline, dim, a foreground colour written #rrggbb or as one
// of the named colours black, red, green, yellow, blue, magenta, cyan and
// white and their bright- forms, such as bright-red, or a background colour
// written bg:#rrggbb. A later word for the same thing wins. A word that is
// none of these is skipped: parsing never fails. Theme.Validate reports the
// words skipped.
func ParseStyle(s string) Style {
	var style Style

	for _, word := range strings.Fields(s) {
		applyWord(&style, word)
	}

	return style
}

// applyWord sets what word sets on style, and reports whether the word is
// one a style understands
func applyWord(style *Style, word string) bool {
	switch word {
	case "bold":
		style.Bold = true
	case "italic":
		style.Italic = true
	case "underline":
		style.Underline = true
	case "dim":
		style.Dim = true
	default:
		if background, ok := strings.CutPrefix(word, "bg:"); ok {
			colour, ok := hexColour(background)
			if !ok {
				return false
			}

			style.Background = colour
			return true
		}

		if _, ok := namedColours[Colour(word)]; ok {
			style.Foreground = Colour(word)
			return true
		}

		colour, ok := hexColour(word)
		if !ok {
			return false
		}

		style.Foreground = colour
	}

	return true
}

// unknownWords returns the words of style string s that a style does not
// understand, in order
func unknownWords(s string) []string {
	var unknown []string
	for _, word := range strings.Fields(s) {
		var style Style
		if !applyWord(&style, word) {
			unknown = append(unknown, word)
		}
	}

	return unknown
}

// hexColour reads a colour written #rrggbb, in either case, as the lower case
// Colour
func hexColour(word string) (Colour, bool) {
	if len(word) != 7 || word[0] != '#' {
		return "", false
	}

	for i := 1; i < len(word); i++ {
		if !isHexDigit(word[i]) {
			return "", false
		}
	}

	return Colour(strings.ToLower(word)), true
}

// hexByte reads two hex digits
func hexByte(digits string) int {
	value := 0
	for i := 0; i < len(digits); i++ {
		value *= 16

		switch b := digits[i]; {
		case b >= '0' && b <= '9':
			value += int(b - '0')
		case b >= 'a' && b <= 'f':
			value += int(b-'a') + 10
		case b >= 'A' && b <= 'F':
			value += int(b-'A') + 10
		}
	}

	return value
}
