package highlight_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// cssRule is one rule of a stylesheet: its selector and its declarations,
// each written "name: value"
type cssRule struct {
	selector     string
	declarations []string
}

// parseCSS reads a stylesheet of plain rules, selector { name: value; ... },
// failing the test unless every rule has a selector, balanced braces and
// declarations that each have a name and a value
func parseCSS(t *testing.T, css string) []cssRule {
	t.Helper()

	require.Equal(t, strings.Count(css, "{"), strings.Count(css, "}"), "unbalanced braces in %q", css)

	var rules []cssRule
	for _, written := range strings.Split(css, "}") {
		written = strings.TrimSpace(written)
		if written == "" {
			continue
		}

		require.Equal(t, 1, strings.Count(written, "{"), "rule %q does not have one {", written)

		selector, body, _ := strings.Cut(written, "{")
		selector = strings.TrimSpace(selector)
		require.NotEmpty(t, selector, "rule %q has no selector", written)

		body = strings.TrimSpace(body)
		require.True(t, strings.HasSuffix(body, ";"), "rule %q does not end its last declaration with ;", written)

		var declarations []string
		for _, declaration := range strings.Split(body, ";") {
			declaration = strings.TrimSpace(declaration)
			if declaration == "" {
				continue
			}

			name, value, found := strings.Cut(declaration, ":")
			require.True(t, found, "declaration %q in rule %q has no :", declaration, written)
			require.NotEmpty(t, strings.TrimSpace(name), "declaration %q in rule %q has no name", declaration, written)
			require.NotEmpty(t, strings.TrimSpace(value), "declaration %q in rule %q has no value", declaration, written)
			require.NotContains(t, strings.TrimSpace(name), " ", "declaration %q in rule %q has a bad name", declaration, written)

			declarations = append(declarations, declaration)
		}

		require.NotEmpty(t, declarations, "rule %q has no declarations", written)

		rules = append(rules, cssRule{selector: selector, declarations: declarations})
	}

	return rules
}

// cssOf writes the stylesheet of theme
func cssOf(t *testing.T, theme highlight.Theme) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, highlight.WriteCSS(&out, theme))

	return out.String()
}

// selectorIndex returns the position of the rule for selector, or -1
func selectorIndex(rules []cssRule, selector string) int {
	for i, rule := range rules {
		if rule.selector == selector {
			return i
		}
	}

	return -1
}

func TestWriteCSSOfTheDefaultThemeIsValidCSS(t *testing.T) {
	rules := parseCSS(t, cssOf(t, highlight.DefaultTheme))

	require.NotEmpty(t, rules)
}

func TestWriteCSSScopesEveryRuleToTheContainer(t *testing.T) {
	theme := highlight.DefaultTheme.With(highlight.Theme{highlight.Text: "#eeeeee"})

	rules := parseCSS(t, cssOf(t, theme))

	for _, rule := range rules {
		require.True(t, rule.selector == ".xcl" || strings.HasPrefix(rule.selector, ".xcl ."), "selector %q is not scoped to .xcl", rule.selector)
	}
}

func TestWriteCSSOfALoadedVSCodeThemeIsValidCSSScopedToTheContainer(t *testing.T) {
	rules := parseCSS(t, cssOf(t, loadReferenceTheme(t)))

	require.NotEmpty(t, rules)
	for _, rule := range rules {
		require.True(t, strings.HasPrefix(rule.selector, ".xcl"), "selector %q is not scoped to .xcl", rule.selector)
	}
}

func TestWriteCSSOfTheDefaultTheme(t *testing.T) {
	css := cssOf(t, highlight.DefaultTheme)

	require.Equal(t, ""+
		".xcl .xcl-block { color: #bc3fbc; font-weight: bold; }\n"+
		".xcl .xcl-label { color: #0dbc79; }\n"+
		".xcl .xcl-type-label { color: #11a8cd; }\n"+
		".xcl .xcl-attribute { color: #2472c8; }\n"+
		".xcl .xcl-string { color: #e5e510; }\n"+
		".xcl .xcl-string-escape { color: #d670d6; }\n"+
		".xcl .xcl-number { color: #d670d6; }\n"+
		".xcl .xcl-constant { color: #d670d6; }\n"+
		".xcl .xcl-reference { color: #11a8cd; }\n"+
		".xcl .xcl-comment { color: #666666; font-style: italic; }\n"+
		".xcl .xcl-inserted { color: #0dbc79; }\n"+
		".xcl .xcl-deleted { color: #cd3131; }\n"+
		".xcl .xcl-changed { color: #e5e510; }\n"+
		".xcl .xcl-error { color: #cd3131; font-weight: bold; }\n"+
		".xcl .xcl-warning { color: #e5e510; font-weight: bold; }\n"+
		".xcl .xcl-summary { font-weight: bold; }\n"+
		".xcl .xcl-gutter { opacity: 0.5; }\n",
		css)
}

func TestWriteCSSPutsACategoryRuleBeforeItsChildRule(t *testing.T) {
	rules := parseCSS(t, cssOf(t, highlight.DefaultTheme))

	category := selectorIndex(rules, ".xcl .xcl-string")
	child := selectorIndex(rules, ".xcl .xcl-string-escape")

	require.NotEqual(t, -1, category)
	require.NotEqual(t, -1, child)
	require.Less(t, category, child)
}

func TestWriteCSSChangingOnlyAttributesChangesOnlyTheAttributeRule(t *testing.T) {
	before := strings.Split(cssOf(t, highlight.DefaultTheme), "\n")
	after := strings.Split(cssOf(t, highlight.DefaultTheme.With(highlight.Theme{highlight.Attribute: "#ff8800"})), "\n")

	require.Len(t, after, len(before))

	var changed []string
	for i := range before {
		if before[i] != after[i] {
			changed = append(changed, after[i])
		}
	}

	require.Equal(t, []string{".xcl .xcl-attribute { color: #ff8800; }"}, changed)
}

func TestWriteCSSWritesNamedColoursAsHex(t *testing.T) {
	css := cssOf(t, highlight.Theme{highlight.Attribute: "bright-red"})

	require.Equal(t, ".xcl .xcl-attribute { color: #f14c4c; }\n", css)
}

func TestWriteCSSWritesDimAsOpacity(t *testing.T) {
	css := cssOf(t, highlight.Theme{highlight.Gutter: "dim"})

	require.Equal(t, ".xcl .xcl-gutter { opacity: 0.5; }\n", css)
}

func TestWriteCSSWritesEveryStyleInOrder(t *testing.T) {
	css := cssOf(t, highlight.Theme{highlight.Attribute: "dim underline italic bold bg:#102030 #ff8800"})

	require.Equal(t,
		".xcl .xcl-attribute { color: #ff8800; background-color: #102030; font-weight: bold; font-style: italic; text-decoration: underline; opacity: 0.5; }\n",
		css)
}

func TestWriteCSSResetsTheBoldAChildDropsFromItsCategory(t *testing.T) {
	css := cssOf(t, highlight.Theme{
		highlight.Block:     "bold magenta",
		highlight.BlockType: "cyan",
	})

	require.Contains(t, css, ".xcl .xcl-block-type { color: #11a8cd; font-weight: normal; }\n")
}

func TestWriteCSSResetsEverythingAChildDropsFromItsCategory(t *testing.T) {
	css := cssOf(t, highlight.Theme{
		highlight.Comment:     "bold italic underline dim bg:#101010 #202020",
		highlight.LineComment: "#ff8800",
	})

	require.Contains(t, css,
		".xcl .xcl-line-comment { color: #ff8800; background-color: transparent; font-weight: normal; font-style: normal; text-decoration: none; opacity: 1; }\n")
}

func TestWriteCSSResetsTheColourAChildDropsFromItsCategory(t *testing.T) {
	css := cssOf(t, highlight.Theme{
		highlight.Comment:     "italic #202020",
		highlight.LineComment: "italic",
	})

	require.Contains(t, css, ".xcl .xcl-line-comment { font-style: italic; color: inherit; }\n")
}

func TestWriteCSSWritesTextStyleOnTheContainer(t *testing.T) {
	css := cssOf(t, highlight.Theme{highlight.Text: "#eeeeee bg:#111111"})

	require.Equal(t, ".xcl { color: #eeeeee; background-color: #111111; }\n", css)
}

func TestWriteCSSWritesTheContainerRuleFirst(t *testing.T) {
	rules := parseCSS(t, cssOf(t, highlight.DefaultTheme.With(highlight.Theme{highlight.Text: "#eeeeee"})))

	require.Equal(t, ".xcl", rules[0].selector)
}

func TestWriteCSSWritesNoContainerRuleWhenTextIsUnstyled(t *testing.T) {
	rules := parseCSS(t, cssOf(t, highlight.DefaultTheme))

	require.Equal(t, -1, selectorIndex(rules, ".xcl"))
}

func TestWriteCSSOfAnEmptyThemeIsEmpty(t *testing.T) {
	require.Empty(t, cssOf(t, highlight.Theme{}))
}

func TestWriteCSSSkipsAStyleOfOnlyUnknownWords(t *testing.T) {
	css := cssOf(t, highlight.Theme{highlight.Attribute: "purple"})

	require.Empty(t, css)
}

func TestWriteCSSReturnsTheWriterError(t *testing.T) {
	err := highlight.WriteCSS(failingWriter{}, highlight.DefaultTheme)

	require.ErrorIs(t, err, errWriteFailed)
}
