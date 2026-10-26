package highlight_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
)

// loadVSCodeTheme loads a VS Code colour theme from source, failing the test
// when it is rejected
func loadVSCodeTheme(t *testing.T, source string) highlight.Theme {
	t.Helper()

	loaded, err := highlight.LoadVSCodeTheme(strings.NewReader(source))
	require.NoError(t, err)

	return loaded
}

// loadReferenceTheme loads testdata/reference-theme.json
func loadReferenceTheme(t *testing.T) highlight.Theme {
	t.Helper()

	file, err := os.Open("testdata/reference-theme.json")
	require.NoError(t, err)
	defer file.Close()

	loaded, err := highlight.LoadVSCodeTheme(file)
	require.NoError(t, err)

	return loaded
}

// requireInvalidTheme asserts err is an InvalidThemeError for a theme read
// from a reader whose reason mentions want
func requireInvalidTheme(t *testing.T, err error, want string) {
	t.Helper()

	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrInvalidTheme))

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, "theme", detail.Source)
	require.Contains(t, detail.Reason, want)
}

// failingReader is an io.Reader whose every read fails with err
type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestLoadVSCodeThemeStylesEveryKindTheReferenceThemeStyles(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.Equal(t, highlight.Theme{
		highlight.BlockType:    "bold #c586c0",
		highlight.NestedBlock:  "#4ec9b0",
		highlight.Label:        "#4ec9b0",
		highlight.TypeLabel:    "#4ec9b0",
		highlight.NameLabel:    "underline #569cd6",
		highlight.Attribute:    "#9cdcfe",
		highlight.String:       "#ce9178",
		highlight.StringEscape: "#b5cea8",
		highlight.Heredoc:      "#ce9178",
		highlight.Number:       "#b5cea8",
		highlight.Constant:     "#b5cea8",
		highlight.Function:     "#4ec9b0",
		highlight.Reference:    "#4fc1ff",
		highlight.Comment:      "italic #6a9955",
		highlight.LineComment:  "italic #6a9955",
		highlight.HashComment:  "italic #6a9955",
		highlight.BlockComment: "italic #6a9955",
		highlight.Gutter:       "italic #6a9955",
	}, loaded)
}

func TestLoadVSCodeThemeReadsCommentsAndTrailingCommas(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.Equal(t, "bold #c586c0", loaded[highlight.BlockType])
}

func TestLoadVSCodeThemeLeavesOutKindsTheThemeDoesNotStyle(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.NotContains(t, loaded, highlight.Operator)
	require.NotContains(t, loaded, highlight.Text)
	require.NotContains(t, loaded, highlight.Block)
}

func TestLoadVSCodeThemeHoldsOnlyTheKindsItStyles(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Equal(t, highlight.Theme{
		highlight.String:  "#123456",
		highlight.Heredoc: "#123456",
	}, loaded)
}

func TestLoadVSCodeThemeColoursPropertiesAsAttributes(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "variable.other.property", "settings": { "foreground": "#ff8800" } }
		]
	}`)

	require.Equal(t, "#ff8800", loaded[highlight.Attribute])
}

func TestLoadVSCodeThemeColoursAttributeNamesInTheTerminal(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "variable.other.property", "settings": { "foreground": "#ff8800" } }
		]
	}`)

	output := format(t, highlight.Terminal, loaded, highlight.Tokenize([]byte("a = 1")))

	require.Equal(t, "\x1b[38;2;255;136;0ma\x1b[0m = 1", output)
}

func TestLoadVSCodeThemeSpecificSelectorBeatsAGeneralOne(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.Equal(t, "underline #569cd6", loaded[highlight.NameLabel])
}

func TestLoadVSCodeThemeGeneralSelectorStylesAKindWithoutASpecificRule(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.Equal(t, "#4ec9b0", loaded[highlight.TypeLabel])
}

func TestLoadVSCodeThemeSpecificSelectorBeatsAGeneralOneWrittenAfterIt(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "string.quoted", "settings": { "foreground": "#111111" } },
			{ "scope": "string", "settings": { "foreground": "#222222" } }
		]
	}`)

	require.Equal(t, "#111111", loaded[highlight.String])
	require.Equal(t, "#222222", loaded[highlight.Heredoc])
}

func TestLoadVSCodeThemeResolvesColourAndFontStyleSeparately(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "storage", "settings": { "foreground": "#111111", "fontStyle": "bold" } },
			{ "scope": "storage.type", "settings": { "foreground": "#222222" } }
		]
	}`)

	require.Equal(t, "bold #222222", loaded[highlight.BlockType])
}

func TestLoadVSCodeThemeLaterRuleWinsATie(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "comment.line", "settings": { "foreground": "#111111", "fontStyle": "bold" } },
			{ "scope": "comment.line", "settings": { "foreground": "#222222", "fontStyle": "italic" } }
		]
	}`)

	require.Equal(t, "italic #222222", loaded[highlight.LineComment])
}

func TestLoadVSCodeThemeReadsCommaSeparatedScopes(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.Equal(t, "#b5cea8", loaded[highlight.Number])
	require.Equal(t, "#b5cea8", loaded[highlight.Constant])
	require.Equal(t, "#b5cea8", loaded[highlight.StringEscape])
}

func TestLoadVSCodeThemeReadsAListOfScopes(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.Equal(t, "#ce9178", loaded[highlight.String])
	require.Equal(t, "#ce9178", loaded[highlight.Heredoc])
}

func TestLoadVSCodeThemeReadsAListOfCommaSeparatedScopes(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": ["string, comment"], "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Equal(t, "#123456", loaded[highlight.String])
	require.Equal(t, "#123456", loaded[highlight.LineComment])
}

func TestLoadVSCodeThemeMatchesADescendantSelectorOnItsLastElement(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "meta.interpolation string", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Equal(t, "#123456", loaded[highlight.String])
}

func TestLoadVSCodeThemeDoesNotMatchADescendantSelectorOnItsAncestor(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "meta.interpolation string", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.NotContains(t, loaded, highlight.TemplateDirective)
}

func TestLoadVSCodeThemeKeepsTheSelectorBeforeAnExclusion(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "string - comment", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Equal(t, "#123456", loaded[highlight.String])
}

func TestLoadVSCodeThemeDropsAnExclusion(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "string - comment", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.NotContains(t, loaded, highlight.LineComment)
}

func TestLoadVSCodeThemeDropsASelectorStartingWithAnExclusion(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "-comment", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Empty(t, loaded)
}

func TestLoadVSCodeThemeDoesNotMatchAPartialSegment(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "constant.num", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.NotContains(t, loaded, highlight.Number)
}

func TestLoadVSCodeThemeReadsAShortColour(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "constant.numeric", "settings": { "foreground": "#abc" } }
		]
	}`)

	require.Equal(t, "#aabbcc", loaded[highlight.Number])
}

func TestLoadVSCodeThemeReadsAShortColourWithAlpha(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "constant.numeric", "settings": { "foreground": "#abcd" } }
		]
	}`)

	require.Equal(t, "#aabbcc", loaded[highlight.Number])
}

func TestLoadVSCodeThemeReadsAColourWithAlphaInLowerCase(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "constant.numeric", "settings": { "foreground": "#1a2B3c80" } }
		]
	}`)

	require.Equal(t, "#1a2b3c", loaded[highlight.Number])
}

func TestLoadVSCodeThemeReadsBoldItalicAndUnderline(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "constant.numeric", "settings": { "fontStyle": "underline italic bold" } }
		]
	}`)

	require.Equal(t, "bold italic underline", loaded[highlight.Number])
}

func TestLoadVSCodeThemeLeavesOutStrikethrough(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "constant.numeric", "settings": { "foreground": "#123456", "fontStyle": "strikethrough bold" } }
		]
	}`)

	require.Equal(t, "bold #123456", loaded[highlight.Number])
}

func TestLoadVSCodeThemeEmptyFontStyleResetsTheGeneralFontStyle(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "comment", "settings": { "foreground": "#111111", "fontStyle": "italic bold" } },
			{ "scope": "comment.line", "settings": { "fontStyle": "" } }
		]
	}`)

	require.Equal(t, "#111111", loaded[highlight.LineComment])
	require.Equal(t, "bold italic #111111", loaded[highlight.BlockComment])
}

func TestLoadVSCodeThemeIgnoresAnEntryWithoutAScope(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "settings": { "foreground": "#ffffff", "background": "#000000" } },
			{ "scope": "constant.numeric", "settings": { "foreground": "#123456" } }
		]
	}`)

	require.Equal(t, highlight.Theme{highlight.Number: "#123456"}, loaded)
}

func TestLoadVSCodeThemeWithoutTokenColorsStylesNothing(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{ "name": "empty", "colors": { "editor.foreground": "#ffffff" } }`)

	require.Empty(t, loaded)
}

func TestLoadVSCodeThemeWithNullTokenColorsStylesNothing(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{ "tokenColors": null }`)

	require.Empty(t, loaded)
}

func TestLoadVSCodeThemeGivesAThemeThatValidates(t *testing.T) {
	loaded := loadReferenceTheme(t)

	require.NoError(t, loaded.Validate())
}

func TestLoadVSCodeThemeExtendsTheDefaultTheme(t *testing.T) {
	loaded := loadVSCodeTheme(t, `{
		"tokenColors": [
			{ "scope": "variable.other.property", "settings": { "foreground": "#ff8800" } }
		]
	}`)

	theme := highlight.DefaultTheme.With(loaded)

	require.Equal(t, "#ff8800", theme[highlight.Attribute])
	require.Equal(t, "bold magenta", theme[highlight.Block])
}

func TestLoadVSCodeThemeColoursTheSampleInTheTerminal(t *testing.T) {
	loaded := loadReferenceTheme(t)

	output := format(t, highlight.Terminal, loaded, highlight.Tokenize(readSample(t)))

	require.Contains(t, output, "\x1b[1;38;2;197;134;192mresource\x1b[0m ")
	require.Contains(t, output, "\x1b[38;2;78;201;176m\"network\"\x1b[0m")
	require.Contains(t, output, "\x1b[4;38;2;86;156;214m\"onprem\"\x1b[0m")
	require.Contains(t, output, "\x1b[38;2;156;220;254msubnet\x1b[0m = \x1b[38;2;206;145;120m\"10.6.0.0/16\"\x1b[0m")
	require.Contains(t, output, "\x1b[3;38;2;106;153;85m# hash comment\x1b[0m\n")
	require.Contains(t, output, "\x1b[38;2;79;193;255mdeployment\x1b[0m.")
}

func TestLoadVSCodeThemeRejectsAnUnreadableReader(t *testing.T) {
	readErr := errors.New("disk on fire")

	_, err := highlight.LoadVSCodeTheme(failingReader{err: readErr})

	requireInvalidTheme(t, err, "cannot be read")
	require.True(t, errors.Is(err, readErr))
}

func TestLoadVSCodeThemeRejectsInvalidJSON(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{ "tokenColors": [ `))

	requireInvalidTheme(t, err, "not valid JSON")

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.NotNil(t, detail.Err)
}

func TestLoadVSCodeThemeRejectsABadColour(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#zz0000" } }
		]
	}`))

	requireInvalidTheme(t, err, `tokenColors[0].settings.foreground "#zz0000" is not a colour`)
}

func TestLoadVSCodeThemeRejectsAColourWithoutAHash(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "ff0000" } }
		]
	}`))

	requireInvalidTheme(t, err, `tokenColors[0].settings.foreground "ff0000" is not a colour`)
}

func TestLoadVSCodeThemeRejectsAColourOfTheWrongLength(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#12345" } }
		]
	}`))

	requireInvalidTheme(t, err, `tokenColors[0].settings.foreground "#12345" is not a colour`)
}

func TestLoadVSCodeThemeRejectsABadFontStyle(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{
		"tokenColors": [
			{ "scope": "string", "settings": { "foreground": "#ffffff" } },
			{ "scope": "comment", "settings": { "fontStyle": "italic wavy" } }
		]
	}`))

	requireInvalidTheme(t, err, `tokenColors[1].settings.fontStyle "wavy" is not a font style`)
}

func TestLoadVSCodeThemeRejectsTokenColorsThatIsNotAList(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{ "tokenColors": { "scope": "string" } }`))

	requireInvalidTheme(t, err, "tokenColors is not a list")
}

func TestLoadVSCodeThemeRejectsTokenColorsAsAString(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{ "tokenColors": "./dark.tmTheme" }`))

	requireInvalidTheme(t, err, ".tmTheme")
}

func TestLoadVSCodeThemeRejectsInclude(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{
		"include": "./base.json",
		"tokenColors": []
	}`))

	requireInvalidTheme(t, err, "include is not supported")
}

func TestLoadVSCodeThemeRejectsAScopeThatIsNotAString(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{
		"tokenColors": [
			{ "scope": 42, "settings": { "foreground": "#ffffff" } }
		]
	}`))

	requireInvalidTheme(t, err, "tokenColors[0].scope is not a string or a list of strings")
}

func TestLoadVSCodeThemeRejectsAnEntryThatIsNotAnObject(t *testing.T) {
	_, err := highlight.LoadVSCodeTheme(strings.NewReader(`{ "tokenColors": [ "string" ] }`))

	requireInvalidTheme(t, err, "tokenColors[0] is not a valid entry")
}

func TestLoadVSCodeThemeReturnsNoThemeWhenItFails(t *testing.T) {
	loaded, err := highlight.LoadVSCodeTheme(strings.NewReader("{"))

	require.Error(t, err)
	require.Nil(t, loaded)
}
