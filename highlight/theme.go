package highlight

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// colour is a foreground colour. A theme's colours are 24-bit, the default
// theme's are one of the terminal's 16 palette colours, given as the SGR
// foreground code: 30 to 37, or 90 to 97 for the bright colours.
type colour struct {
	basic      int
	r, g, b    uint8
	truecolour bool
}

// fontStyle is the set of styles a rule turns on. A rule that sets an empty
// font style turns every style off.
type fontStyle struct {
	bold, italic, underline, strikethrough bool
}

// rule is one selector of a theme's tokenColors entry, with what it sets
type rule struct {
	selector  string     // e.g. "string.quoted", the last element of a descendant selector
	segments  int        // dot segments in selector, its specificity
	order     int        // position in the theme, later wins a tie
	colour    *colour    // nil when the rule sets no foreground
	fontStyle *fontStyle // nil when the rule sets no fontStyle; an empty style resets
}

// style is what a scope resolves to: a colour and a font style, each nil
// when no rule sets it
type style struct {
	colour    *colour
	fontStyle *fontStyle
}

// theme answers what style a scope gets, by TextMate's rules: a selector
// matches a scope it equals or is a dot-segment prefix of, the selector with
// the most segments wins, and the later rule wins a tie. The colour and the
// font style are resolved separately, each from the most specific rule that
// sets it. Answers are cached, so a scope is matched only once.
type theme struct {
	rules []rule

	mutex sync.Mutex
	cache map[string]style
}

// newTheme returns a theme of rules
func newTheme(rules []rule) *theme {
	return &theme{rules: rules, cache: map[string]style{}}
}

// style returns the style scope gets
func (t *theme) style(scope string) style {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if resolved, ok := t.cache[scope]; ok {
		return resolved
	}

	var resolved style
	var colourRule, fontStyleRule *rule

	for i := range t.rules {
		candidate := &t.rules[i]
		if !selectorMatches(candidate.selector, scope) {
			continue
		}

		if candidate.colour != nil && outranks(candidate, colourRule) {
			colourRule = candidate
			resolved.colour = candidate.colour
		}

		if candidate.fontStyle != nil && outranks(candidate, fontStyleRule) {
			fontStyleRule = candidate
			resolved.fontStyle = candidate.fontStyle
		}
	}

	t.cache[scope] = resolved

	return resolved
}

// selectorMatches reports whether selector equals scope or is a dot-segment
// prefix of it, so "string" matches "string.quoted.double.xcl" but not
// "strings"
func selectorMatches(selector, scope string) bool {
	return scope == selector || strings.HasPrefix(scope, selector+".")
}

// outranks reports whether candidate is more specific than best, or as
// specific and later in the theme
func outranks(candidate, best *rule) bool {
	if best == nil {
		return true
	}

	if candidate.segments != best.segments {
		return candidate.segments > best.segments
	}

	return candidate.order > best.order
}

// defaultTheme is the theme used when none is given. It uses only the
// terminal's 16 palette colours, so the result follows the user's own
// terminal palette, and scopes it has no rule for keep the default colour.
// Besides configuration text it colours diff output: inserted green, deleted
// red and changed yellow.
func defaultTheme() *theme {
	basic := func(selector string, code int, style *fontStyle, order int) rule {
		return rule{
			selector:  selector,
			segments:  strings.Count(selector, ".") + 1,
			order:     order,
			colour:    &colour{basic: code},
			fontStyle: style,
		}
	}

	return newTheme([]rule{
		basic("storage.type", 35, &fontStyle{bold: true}, 0),
		basic("entity.name.type", 36, nil, 1),
		basic("entity.name.tag", 32, nil, 2),
		basic("variable.other.property", 34, nil, 3),
		basic("string", 33, nil, 4),
		basic("constant", 95, nil, 5),
		basic("support.class.reference", 36, nil, 6),
		basic("comment", 90, &fontStyle{italic: true}, 7),
		basic("markup.inserted", 32, nil, 8),
		basic("markup.deleted", 31, nil, 9),
		basic("markup.changed", 33, nil, 10),
	})
}

// themeFile is the part of a VS Code colour theme that is read
type themeFile struct {
	Include     json.RawMessage `json:"include"`
	TokenColors json.RawMessage `json:"tokenColors"`
}

// tokenColor is one entry of a theme's tokenColors
type tokenColor struct {
	Scope    json.RawMessage `json:"scope"`
	Settings struct {
		Foreground *string `json:"foreground"`
		FontStyle  *string `json:"fontStyle"`
	} `json:"settings"`
}

// parseTheme reads a VS Code colour theme, JSON or JSON with comments and
// trailing commas, from r. source names the theme in errors: its file path,
// or "theme" for a reader. A theme that cannot be read or is invalid returns
// an InvalidThemeError.
func parseTheme(source string, r io.Reader) (*theme, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, invalidTheme(source, "it cannot be read", err)
	}

	var file themeFile
	if err := json.Unmarshal(stripJSONC(src), &file); err != nil {
		return nil, invalidTheme(source, "it is not valid JSON", err)
	}

	// following an include needs to know where to look, which a reader
	// cannot say, so a theme has to stand on its own
	if file.Include != nil {
		return nil, invalidTheme(source, "include is not supported, the theme must hold all its tokenColors", nil)
	}

	tokenColors := bytes.TrimSpace(file.TokenColors)
	if len(tokenColors) == 0 || string(tokenColors) == "null" {
		return newTheme(nil), nil
	}

	if tokenColors[0] == '"' {
		return nil, invalidTheme(source, "tokenColors names a .tmTheme file, which is not supported", nil)
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(tokenColors, &entries); err != nil {
		return nil, invalidTheme(source, "tokenColors is not a list", nil)
	}

	var rules []rule
	for i, raw := range entries {
		entryRules, err := parseTokenColor(source, i, raw, len(rules))
		if err != nil {
			return nil, err
		}

		rules = append(rules, entryRules...)
	}

	return newTheme(rules), nil
}

// parseTokenColor returns the rules of tokenColors entry i, one for each of
// its selectors, numbered from order. An entry with no scope sets nothing.
func parseTokenColor(source string, i int, raw json.RawMessage, order int) ([]rule, error) {
	var entry tokenColor
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, invalidTheme(source, fmt.Sprintf("tokenColors[%d] is not a valid entry", i), err)
	}

	selectors, err := parseScope(entry.Scope)
	if err != nil {
		return nil, invalidTheme(source, fmt.Sprintf("tokenColors[%d].scope %s", i, err), nil)
	}

	var foreground *colour
	if entry.Settings.Foreground != nil {
		parsed, ok := parseColour(*entry.Settings.Foreground)
		if !ok {
			return nil, invalidTheme(source, fmt.Sprintf("tokenColors[%d].settings.foreground %q is not a colour", i, *entry.Settings.Foreground), nil)
		}
		foreground = &parsed
	}

	var style *fontStyle
	if entry.Settings.FontStyle != nil {
		parsed, err := parseFontStyle(*entry.Settings.FontStyle)
		if err != nil {
			return nil, invalidTheme(source, fmt.Sprintf("tokenColors[%d].settings.fontStyle %s", i, err), nil)
		}
		style = &parsed
	}

	rules := make([]rule, 0, len(selectors))
	for _, selector := range selectors {
		rules = append(rules, rule{
			selector:  selector,
			segments:  strings.Count(selector, ".") + 1,
			order:     order + len(rules),
			colour:    foreground,
			fontStyle: style,
		})
	}

	return rules, nil
}

// parseScope returns the selectors of an entry's scope, written as a string,
// a comma-separated string or a list of strings. Each token carries one
// scope, so a descendant selector such as "meta.interpolation string" is
// matched on its last element, and an exclusion such as "string - comment"
// is dropped along with what it excludes.
func parseScope(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var written []string

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		written = strings.Split(single, ",")
	} else if err := json.Unmarshal(raw, &written); err != nil {
		return nil, fmt.Errorf("is not a string or a list of strings")
	}

	var selectors []string
	for _, item := range written {
		for _, part := range strings.Split(item, ",") {
			if selector := lastElement(part); selector != "" {
				selectors = append(selectors, selector)
			}
		}
	}

	return selectors, nil
}

// lastElement returns the scope a selector applies to: its last element,
// once any exclusion is removed
func lastElement(selector string) string {
	selector = strings.TrimSpace(selector)
	if strings.HasPrefix(selector, "-") {
		return ""
	}

	if before, _, found := strings.Cut(selector, " -"); found {
		selector = before
	}

	fields := strings.Fields(selector)
	if len(fields) == 0 {
		return ""
	}

	return fields[len(fields)-1]
}

// parseColour reads a colour written #RGB, #RGBA, #RRGGBB or #RRGGBBAA. The
// alpha is ignored, a terminal has no use for it.
func parseColour(written string) (colour, bool) {
	if !strings.HasPrefix(written, "#") {
		return colour{}, false
	}

	digits := written[1:]
	for i := 0; i < len(digits); i++ {
		if !isHexDigit(digits[i]) {
			return colour{}, false
		}
	}

	switch len(digits) {
	case 3, 4:
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	case 6, 8:
		digits = digits[:6]
	default:
		return colour{}, false
	}

	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return colour{}, false
	}

	return colour{
		truecolour: true,
		r:          uint8(value >> 16),
		g:          uint8(value >> 8),
		b:          uint8(value),
	}, true
}

// parseFontStyle reads a space-separated list of bold, italic, underline
// and strikethrough. An empty list is an explicit reset.
func parseFontStyle(written string) (fontStyle, error) {
	var style fontStyle

	for _, word := range strings.Fields(written) {
		switch word {
		case "bold":
			style.bold = true
		case "italic":
			style.italic = true
		case "underline":
			style.underline = true
		case "strikethrough":
			style.strikethrough = true
		default:
			return fontStyle{}, fmt.Errorf("%q is not a font style", word)
		}
	}

	return style, nil
}

// invalidTheme returns the error for a theme that cannot be used
func invalidTheme(source, reason string, err error) error {
	return &xclerrors.InvalidThemeError{Source: source, Reason: reason, Err: err}
}
