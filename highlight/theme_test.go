package highlight

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// These tests are internal: there is no public way to load a theme until the
// ANSI renderer exists, so they drive parseTheme and theme.style directly.

// mustParseTheme parses source as a theme read from a reader, failing the
// test when it is rejected
func mustParseTheme(t *testing.T, source string) *theme {
	t.Helper()

	parsed, err := parseTheme("theme", strings.NewReader(source))
	require.NoError(t, err)

	return parsed
}

// loadReferenceTheme parses testdata/reference-theme.json
func loadReferenceTheme(t *testing.T) *theme {
	t.Helper()

	file, err := os.Open("testdata/reference-theme.json")
	require.NoError(t, err)
	defer file.Close()

	parsed, err := parseTheme("testdata/reference-theme.json", file)
	require.NoError(t, err)

	return parsed
}

// requireInvalidTheme asserts err is an InvalidThemeError for source whose
// reason mentions want
func requireInvalidTheme(t *testing.T, err error, source, want string) {
	t.Helper()

	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrInvalidTheme))

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, source, detail.Source)
	require.Contains(t, detail.Reason, want)
}

// failingReader is an io.Reader whose every read fails with err
type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestThemePrefixSelectorMatchesDeeperScope(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#ce9178" } }
		]
	}`)

	resolved := parsed.style("string.quoted.double.xcl")

	require.Equal(t, &colour{truecolour: true, r: 0xce, g: 0x91, b: 0x78}, resolved.colour)
}

func TestThemeSelectorMatchesEqualScope(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string.quoted", "settings": { "foreground": "#ce9178" } }
		]
	}`)

	resolved := parsed.style("string.quoted")

	require.Equal(t, &colour{truecolour: true, r: 0xce, g: 0x91, b: 0x78}, resolved.colour)
}

func TestThemePrefixSelectorDoesNotMatchPartialSegment(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#ce9178" } }
		]
	}`)

	resolved := parsed.style("strings.x")

	require.Nil(t, resolved.colour)
	require.Nil(t, resolved.fontStyle)
}

func TestThemeMoreSpecificSelectorOverridesGeneral(t *testing.T) {
	parsed := loadReferenceTheme(t)

	resolved := parsed.style("entity.name.tag.xcl")

	require.Equal(t, &colour{truecolour: true, r: 0x56, g: 0x9c, b: 0xd6}, resolved.colour)
	require.Equal(t, &fontStyle{underline: true}, resolved.fontStyle)
}

func TestThemeGeneralSelectorColoursScopeWithoutSpecificRule(t *testing.T) {
	parsed := loadReferenceTheme(t)

	resolved := parsed.style("entity.name.type.xcl")

	require.Equal(t, &colour{truecolour: true, r: 0x4e, g: 0xc9, b: 0xb0}, resolved.colour)
	require.Nil(t, resolved.fontStyle)
}

func TestThemeMoreSpecificSelectorOverridesGeneralWrittenAfterIt(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string.quoted", "settings": { "foreground": "#111111" } },
			{ "scope": "string", "settings": { "foreground": "#222222" } }
		]
	}`)

	resolved := parsed.style("string.quoted.double")

	require.Equal(t, &colour{truecolour: true, r: 0x11, g: 0x11, b: 0x11}, resolved.colour)
}

func TestThemeResolvesColourAndFontStyleIndependently(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "storage", "settings": { "foreground": "#111111", "fontStyle": "bold" } },
			{ "scope": "storage.type", "settings": { "foreground": "#222222" } }
		]
	}`)

	resolved := parsed.style("storage.type.xcl")

	require.Equal(t, &colour{truecolour: true, r: 0x22, g: 0x22, b: 0x22}, resolved.colour)
	require.Equal(t, &fontStyle{bold: true}, resolved.fontStyle)
}

func TestThemeLaterRuleWinsTie(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "comment.line", "settings": { "foreground": "#111111", "fontStyle": "bold" } },
			{ "scope": "comment.line", "settings": { "foreground": "#222222", "fontStyle": "italic" } }
		]
	}`)

	resolved := parsed.style("comment.line.double-slash")

	require.Equal(t, &colour{truecolour: true, r: 0x22, g: 0x22, b: 0x22}, resolved.colour)
	require.Equal(t, &fontStyle{italic: true}, resolved.fontStyle)
}

func TestThemeReadsCommaSeparatedScopes(t *testing.T) {
	parsed := loadReferenceTheme(t)

	numeric := parsed.style("constant.numeric.xcl")
	language := parsed.style("constant.language.xcl")
	escape := parsed.style("constant.character.escape.xcl")

	expected := &colour{truecolour: true, r: 0xb5, g: 0xce, b: 0xa8}
	require.Equal(t, expected, numeric.colour)
	require.Equal(t, expected, language.colour)
	require.Equal(t, expected, escape.colour)
}

func TestThemeReadsScopeArray(t *testing.T) {
	parsed := loadReferenceTheme(t)

	quoted := parsed.style("string.quoted.double.xcl")
	heredoc := parsed.style("string.unquoted.heredoc.xcl")

	expected := &colour{truecolour: true, r: 0xce, g: 0x91, b: 0x78}
	require.Equal(t, expected, quoted.colour)
	require.Equal(t, expected, heredoc.colour)
}

func TestThemeReadsScopeArrayWithCommaSeparatedItems(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": ["string, comment"], "settings": { "foreground": "#123456" } }
		]
	}`)

	resolved := parsed.style("comment.line")

	require.Equal(t, &colour{truecolour: true, r: 0x12, g: 0x34, b: 0x56}, resolved.colour)
}

func TestThemeMatchesDescendantSelectorOnLastElement(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "meta.interpolation string", "settings": { "foreground": "#123456" } }
		]
	}`)

	resolved := parsed.style("string.quoted.double")

	require.Equal(t, &colour{truecolour: true, r: 0x12, g: 0x34, b: 0x56}, resolved.colour)
}

func TestThemeDoesNotMatchDescendantSelectorOnAncestor(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "meta.interpolation string", "settings": { "foreground": "#123456" } }
		]
	}`)

	resolved := parsed.style("meta.interpolation.xcl")

	require.Nil(t, resolved.colour)
}

func TestThemeKeepsSelectorBeforeExclusion(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string - comment", "settings": { "foreground": "#123456" } }
		]
	}`)

	resolved := parsed.style("string.quoted")

	require.Equal(t, &colour{truecolour: true, r: 0x12, g: 0x34, b: 0x56}, resolved.colour)
}

func TestThemeDoesNotColourExcludedScope(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string - comment", "settings": { "foreground": "#123456" } }
		]
	}`)

	resolved := parsed.style("comment.line")

	require.Nil(t, resolved.colour)
}

func TestThemeDropsSelectorStartingWithExclusion(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "-comment", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Empty(t, parsed.rules)
	require.Nil(t, parsed.style("comment.line").colour)
}

func TestThemeLoadsCommentsAndTrailingCommas(t *testing.T) {
	parsed := loadReferenceTheme(t)

	resolved := parsed.style("storage.type.xcl")

	require.Equal(t, &colour{truecolour: true, r: 0xc5, g: 0x86, b: 0xc0}, resolved.colour)
	require.Equal(t, &fontStyle{bold: true}, resolved.fontStyle)
}

func TestThemeLoadsEveryReferenceRule(t *testing.T) {
	parsed := loadReferenceTheme(t)

	// one rule per selector: 1+1+1+1+2+3+1+1
	require.Len(t, parsed.rules, 11)
}

func TestThemeReadsShortColour(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#abc" } }
		]
	}`)

	resolved := parsed.style("string")

	require.Equal(t, &colour{truecolour: true, r: 0xaa, g: 0xbb, b: 0xcc}, resolved.colour)
}

func TestThemeReadsShortColourWithAlpha(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#abcd" } }
		]
	}`)

	resolved := parsed.style("string")

	require.Equal(t, &colour{truecolour: true, r: 0xaa, g: 0xbb, b: 0xcc}, resolved.colour)
}

func TestThemeReadsColourWithAlpha(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#1a2B3c80" } }
		]
	}`)

	resolved := parsed.style("string")

	require.Equal(t, &colour{truecolour: true, r: 0x1a, g: 0x2b, b: 0x3c}, resolved.colour)
}

func TestThemeReadsEveryFontStyle(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "markup", "settings": { "fontStyle": "bold italic underline strikethrough" } }
		]
	}`)

	resolved := parsed.style("markup.heading")

	require.Nil(t, resolved.colour)
	require.Equal(t, &fontStyle{bold: true, italic: true, underline: true, strikethrough: true}, resolved.fontStyle)
}

func TestThemeEmptyFontStyleResets(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "comment", "settings": { "fontStyle": "italic bold" } },
			{ "scope": "comment.block.documentation", "settings": { "fontStyle": "" } }
		]
	}`)

	resolved := parsed.style("comment.block.documentation.xcl")

	require.Equal(t, &fontStyle{}, resolved.fontStyle)
}

func TestThemeLeavesUnmatchedScopeUnstyled(t *testing.T) {
	parsed := loadReferenceTheme(t)

	resolved := parsed.style("keyword.operator.xcl")

	require.Nil(t, resolved.colour)
	require.Nil(t, resolved.fontStyle)
}

func TestThemeIgnoresEntryWithoutScope(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "settings": { "foreground": "#ffffff", "background": "#000000" } },
			{ "scope": "string", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Len(t, parsed.rules, 1)
	require.Nil(t, parsed.style("comment").colour)
	require.Equal(t, &colour{truecolour: true, r: 0x12, g: 0x34, b: 0x56}, parsed.style("string").colour)
}

func TestThemeWithoutTokenColorsStylesNothing(t *testing.T) {
	parsed := mustParseTheme(t, `{ "name": "empty", "colors": { "editor.foreground": "#ffffff" } }`)

	resolved := parsed.style("storage.type.xcl")

	require.Empty(t, parsed.rules)
	require.Nil(t, resolved.colour)
	require.Nil(t, resolved.fontStyle)
}

func TestThemeWithNullTokenColorsStylesNothing(t *testing.T) {
	parsed := mustParseTheme(t, `{ "tokenColors": null }`)

	require.Empty(t, parsed.rules)
	require.Nil(t, parsed.style("string").colour)
}

func TestThemeCachesResolvedStyle(t *testing.T) {
	parsed := mustParseTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#123456" } }
		]
	}`)

	first := parsed.style("string.quoted")
	second := parsed.style("string.quoted")

	require.Same(t, first.colour, second.colour)
	require.Contains(t, parsed.cache, "string.quoted")
}

func TestDefaultThemeUsesOnlyBasicColours(t *testing.T) {
	parsed := defaultTheme()

	require.NotEmpty(t, parsed.rules)
	for _, candidate := range parsed.rules {
		require.NotNil(t, candidate.colour, "rule %s", candidate.selector)
		require.False(t, candidate.colour.truecolour, "rule %s", candidate.selector)

		code := candidate.colour.basic
		isPalette := (code >= 30 && code <= 37) || (code >= 90 && code <= 97)
		require.True(t, isPalette, "rule %s has code %d", candidate.selector, code)
	}
}

func TestDefaultThemeColoursBlockTypeMagentaBold(t *testing.T) {
	parsed := defaultTheme()

	resolved := parsed.style("storage.type.xcl")

	require.Equal(t, &colour{basic: 35}, resolved.colour)
	require.Equal(t, &fontStyle{bold: true}, resolved.fontStyle)
}

func TestDefaultThemeColoursCommentGreyItalic(t *testing.T) {
	parsed := defaultTheme()

	resolved := parsed.style("comment.line.number-sign.xcl")

	require.Equal(t, &colour{basic: 90}, resolved.colour)
	require.Equal(t, &fontStyle{italic: true}, resolved.fontStyle)
}

func TestDefaultThemeLeavesOperatorUnstyled(t *testing.T) {
	parsed := defaultTheme()

	resolved := parsed.style("keyword.operator.xcl")

	require.Nil(t, resolved.colour)
	require.Nil(t, resolved.fontStyle)
}

func TestParseThemeRejectsUnreadableReader(t *testing.T) {
	readErr := errors.New("disk on fire")

	_, err := parseTheme("theme", failingReader{err: readErr})

	requireInvalidTheme(t, err, "theme", "cannot be read")
	require.True(t, errors.Is(err, readErr))
}

func TestParseThemeRejectsMalformedJSON(t *testing.T) {
	_, err := parseTheme("themes/broken.json", strings.NewReader(`{ "tokenColors": [ `))

	requireInvalidTheme(t, err, "themes/broken.json", "not valid JSON")

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.NotNil(t, detail.Err)
}

func TestParseThemeRejectsMalformedColour(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#zz0000" } }
		]
	}`))

	requireInvalidTheme(t, err, "theme", `tokenColors[0].settings.foreground "#zz0000" is not a colour`)
}

func TestParseThemeRejectsColourWithoutHash(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "ff0000" } }
		]
	}`))

	requireInvalidTheme(t, err, "theme", `tokenColors[0].settings.foreground "ff0000" is not a colour`)
}

func TestParseThemeRejectsColourOfWrongLength(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#12345" } }
		]
	}`))

	requireInvalidTheme(t, err, "theme", `tokenColors[0].settings.foreground "#12345" is not a colour`)
}

func TestParseThemeRejectsUnknownFontStyle(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#ffffff" } },
			{ "scope": "comment", "settings": { "fontStyle": "italic wavy" } }
		]
	}`))

	requireInvalidTheme(t, err, "theme", `tokenColors[1].settings.fontStyle "wavy" is not a font style`)
}

func TestParseThemeRejectsTokenColorsThatIsNotAList(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{ "tokenColors": { "scope": "string" } }`))

	requireInvalidTheme(t, err, "theme", "tokenColors is not a list")
}

func TestParseThemeRejectsTokenColorsPath(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{ "tokenColors": "./dark.tmTheme" }`))

	requireInvalidTheme(t, err, "theme", ".tmTheme")
}

func TestParseThemeRejectsInclude(t *testing.T) {
	_, err := parseTheme("themes/child.json", strings.NewReader(`{
		"include": "./base.json",
		"tokenColors": []
	}`))

	requireInvalidTheme(t, err, "themes/child.json", "include is not supported")
}

func TestParseThemeRejectsScopeThatIsNotAString(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{
		"tokenColors": [
			{ "scope": 42, "settings": { "foreground": "#ffffff" } }
		]
	}`))

	requireInvalidTheme(t, err, "theme", "tokenColors[0].scope is not a string or a list of strings")
}

func TestParseThemeRejectsEntryThatIsNotAnObject(t *testing.T) {
	_, err := parseTheme("theme", strings.NewReader(`{ "tokenColors": [ "string" ] }`))

	requireInvalidTheme(t, err, "theme", "tokenColors[0] is not a valid entry")
}
