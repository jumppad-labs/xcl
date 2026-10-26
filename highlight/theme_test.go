package highlight_test

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
)

// commentsSource holds one comment of each kind, a line each
const commentsSource = "// a\n# b\n/* c */\n"

// terminalOf formats xcl text with Terminal and theme
func terminalOf(t *testing.T, theme highlight.Theme, text string) string {
	t.Helper()

	return format(t, highlight.Terminal, theme, highlight.Tokenize([]byte(text)))
}

func TestThemeStyleIsTheKindsOwnStyle(t *testing.T) {
	theme := highlight.Theme{highlight.LineComment: "bold red"}

	require.Equal(t, highlight.Style{Bold: true, Foreground: "red"}, theme.Style(highlight.LineComment))
}

func TestThemeStyleFallsBackToTheParent(t *testing.T) {
	theme := highlight.Theme{highlight.Comment: "italic bright-black"}

	require.Equal(t, highlight.Style{Italic: true, Foreground: "bright-black"}, theme.Style(highlight.LineComment))
}

func TestThemeStyleFallsBackToTextWhenTheParentIsUnstyled(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "#eeeeee"}

	require.Equal(t, highlight.Style{Foreground: "#eeeeee"}, theme.Style(highlight.LineComment))
}

func TestThemeStyleOfACategoryFallsBackToText(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "#eeeeee"}

	require.Equal(t, highlight.Style{Foreground: "#eeeeee"}, theme.Style(highlight.Attribute))
}

func TestThemeStyleOfAnUnstyledKindIsTheZeroStyle(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "blue"}

	require.True(t, theme.Style(highlight.Operator).IsZero())
}

func TestThemeStyleOfAnUnknownKindFallsBackToItsParent(t *testing.T) {
	theme := highlight.Theme{highlight.Comment: "italic"}

	require.Equal(t, highlight.Style{Italic: true}, theme.Style(highlight.Kind(1105)))
}

func TestThemeStyleReplacesTheParentStyleWhole(t *testing.T) {
	theme := highlight.Theme{
		highlight.Comment:     "italic bright-black",
		highlight.LineComment: "#ff8800",
	}

	require.Equal(t, highlight.Style{Foreground: "#ff8800"}, theme.Style(highlight.LineComment))
}

func TestThemeStyleSkipsUnknownWords(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bold purple"}

	require.Equal(t, highlight.Style{Bold: true}, theme.Style(highlight.Attribute))
}

func TestThemeStylingOnlyCommentsColoursEveryKindOfComment(t *testing.T) {
	theme := highlight.Theme{highlight.Comment: "italic red"}

	output := terminalOf(t, theme, commentsSource)

	require.Equal(t, "\x1b[3;31m// a\x1b[0m\n\x1b[3;31m# b\x1b[0m\n\x1b[3;31m/* c */\x1b[0m\n", output)
}

func TestThemeStylingLineCommentsGivesThemTheirOwnStyle(t *testing.T) {
	theme := highlight.Theme{
		highlight.Comment:     "italic red",
		highlight.LineComment: "bold green",
	}

	output := terminalOf(t, theme, commentsSource)

	require.Equal(t, "\x1b[1;32m// a\x1b[0m\n\x1b[3;31m# b\x1b[0m\n\x1b[3;31m/* c */\x1b[0m\n", output)
}

func TestDefaultThemeColoursBlockTypesBoldMagenta(t *testing.T) {
	output := terminalOf(t, highlight.DefaultTheme, `resource "network" "main" {}`)

	require.Contains(t, output, "\x1b[1;35mresource\x1b[0m")
}

func TestDefaultThemeColoursAttributesBlue(t *testing.T) {
	output := terminalOf(t, highlight.DefaultTheme, "a = 1\n")

	require.Contains(t, output, "\x1b[34ma\x1b[0m")
}

func TestDefaultThemeLeavesOperatorsUnstyled(t *testing.T) {
	require.True(t, highlight.DefaultTheme.Style(highlight.Operator).IsZero())
}

func TestDefaultThemeLeavesTextUnstyled(t *testing.T) {
	require.True(t, highlight.DefaultTheme.Style(highlight.Text).IsZero())
}

func TestDefaultThemeColoursCommentsItalicGrey(t *testing.T) {
	require.Equal(t, highlight.Style{Italic: true, Foreground: "bright-black"}, highlight.DefaultTheme.Style(highlight.HashComment))
}

func TestDefaultThemeUsesOnlyNamedColours(t *testing.T) {
	for kind, written := range highlight.DefaultTheme {
		style := highlight.ParseStyle(written)

		require.NotContains(t, string(style.Foreground), "#", "kind %s", kind)
		require.Empty(t, style.Background, "kind %s", kind)
	}
}

func TestThemeWithKeepsTheDefaultStyleOfKindsNotMentioned(t *testing.T) {
	theme := highlight.DefaultTheme.With(highlight.Theme{highlight.Attribute: "#ff8800"})

	require.Equal(t, highlight.Style{Bold: true, Foreground: "magenta"}, theme.Style(highlight.BlockType))
	require.Equal(t, highlight.Style{Italic: true, Foreground: "bright-black"}, theme.Style(highlight.LineComment))
}

func TestThemeWithReplacesTheStyleOfKindsMentioned(t *testing.T) {
	theme := highlight.DefaultTheme.With(highlight.Theme{highlight.Attribute: "#ff8800"})

	require.Equal(t, highlight.Style{Foreground: "#ff8800"}, theme.Style(highlight.Attribute))
}

func TestThemeWithDoesNotChangeItsReceiver(t *testing.T) {
	base := highlight.Theme{highlight.Attribute: "blue"}

	base.With(highlight.Theme{highlight.Attribute: "red", highlight.Comment: "italic"})

	require.Equal(t, highlight.Theme{highlight.Attribute: "blue"}, base)
}

func TestThemeWithDoesNotChangeItsArgument(t *testing.T) {
	overrides := highlight.Theme{highlight.Attribute: "red"}

	extended := highlight.Theme{highlight.Comment: "italic"}.With(overrides)
	extended[highlight.Number] = "green"

	require.Equal(t, highlight.Theme{highlight.Attribute: "red"}, overrides)
}

func TestThemeWithReturnsANewTheme(t *testing.T) {
	base := highlight.Theme{highlight.Attribute: "blue"}

	extended := base.With(highlight.Theme{})
	extended[highlight.Attribute] = "red"

	require.Equal(t, "blue", base[highlight.Attribute])
}

func TestThemeValidateReportsAnUnknownWord(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bold purple"}

	err := theme.Validate()

	require.True(t, errors.Is(err, xclerrors.ErrInvalidStyle))
}

func TestThemeValidateDetailsTheUnknownWord(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bold purple"}

	err := theme.Validate()

	var detail *xclerrors.InvalidStyleError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, "Attribute", detail.Kind)
	require.Equal(t, "bold purple", detail.Style)
	require.Equal(t, "purple", detail.Word)
}

func TestThemeValidateReportsEachUnknownWord(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "purple bold mauve"}

	err := theme.Validate()

	require.Error(t, err)
	require.Equal(t,
		"style \"purple bold mauve\" for Attribute: unknown word \"purple\"\n"+
			"style \"purple bold mauve\" for Attribute: unknown word \"mauve\"",
		err.Error())

	joined, ok := err.(interface{ Unwrap() []error })
	require.True(t, ok)
	require.Len(t, joined.Unwrap(), 2)
}

func TestThemeValidateReportsKindsInNumericOrder(t *testing.T) {
	theme := highlight.Theme{
		highlight.Comment:   "wavy",
		highlight.BlockType: "#fff",
	}

	err := theme.Validate()

	require.Equal(t,
		"style \"#fff\" for BlockType: unknown word \"#fff\"\n"+
			"style \"wavy\" for Comment: unknown word \"wavy\"",
		err.Error())
}

func TestThemeValidateOfTheDefaultThemeIsNil(t *testing.T) {
	require.NoError(t, highlight.DefaultTheme.Validate())
}

func TestThemeValidateOfAnEmptyThemeIsNil(t *testing.T) {
	require.NoError(t, highlight.Theme{}.Validate())
}

func TestThemeValidateOfAThemeOfKnownWordsIsNil(t *testing.T) {
	theme := highlight.Theme{
		highlight.Text:      "bg:#000000 #ffffff",
		highlight.Attribute: "bold italic underline dim bright-cyan",
	}

	require.NoError(t, theme.Validate())
}

func TestTerminalWithAnUnknownStyleWordSucceeds(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bold purple"}

	err := highlight.Terminal.Format(io.Discard, theme, highlight.Tokenize([]byte("a = 1\n")))

	require.NoError(t, err)
}

func TestTerminalWithAnUnknownStyleWordAppliesTheKnownWords(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bold purple"}

	output := terminalOf(t, theme, "a = 1\n")

	require.Equal(t, "\x1b[1ma\x1b[0m = 1\n", output)
}
