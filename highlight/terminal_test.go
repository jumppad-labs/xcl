package highlight_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// sgrSequence matches an SGR escape sequence, capturing its parameters
var sgrSequence = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// terminalOne formats a single token with Terminal and theme
func terminalOne(t *testing.T, theme highlight.Theme, kind highlight.Kind, text string) string {
	t.Helper()

	return format(t, highlight.Terminal, theme, []highlight.Token{{Kind: kind, Text: text}})
}

// requireNoNewlineInsideColour fails when a line break is written while a
// colour is on: after an SGR sequence other than the reset and before the
// next reset
func requireNoNewlineInsideColour(t *testing.T, output string) {
	t.Helper()

	coloured := false
	position := 0
	for _, match := range sgrSequence.FindAllStringSubmatchIndex(output, -1) {
		between := output[position:match[0]]
		if coloured {
			require.NotContains(t, between, "\n", "line break inside a colour before offset %d", match[0])
		}

		coloured = output[match[2]:match[3]] != "0"
		position = match[1]
	}

	require.False(t, coloured, "output ends with a colour on")
}

func TestTerminalColoursTheSample(t *testing.T) {
	output := format(t, highlight.Terminal, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	require.Contains(t, output, "\x1b[")
}

func TestTerminalOutputWithCodesRemovedIsThePlainOutput(t *testing.T) {
	tokens := highlight.Tokenize(readSample(t))

	terminal := format(t, highlight.Terminal, highlight.DefaultTheme, tokens)
	plain := format(t, highlight.Plain, highlight.DefaultTheme, tokens)

	require.Equal(t, plain, testutil.StripANSI(terminal))
}

func TestTerminalOutputWithCodesRemovedIsTheSampleInput(t *testing.T) {
	input := readSample(t)

	output := format(t, highlight.Terminal, highlight.DefaultTheme, highlight.Tokenize(input))

	require.Equal(t, string(input), testutil.StripANSI(output))
}

func TestTerminalResetsColourBeforeEveryNewline(t *testing.T) {
	output := format(t, highlight.Terminal, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	requireNoNewlineInsideColour(t, output)
}

func TestTerminalResetsColourBeforeEveryNewlineWhenTextIsStyled(t *testing.T) {
	theme := highlight.DefaultTheme.With(highlight.Theme{highlight.Text: "#eeeeee"})

	output := format(t, highlight.Terminal, theme, highlight.Tokenize(readSample(t)))

	requireNoNewlineInsideColour(t, output)
}

func TestTerminalEndsEveryColouredLineWithAReset(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "dim"}

	output := format(t, highlight.Terminal, theme, highlight.Tokenize(readSample(t)))

	for i, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if strings.Contains(line, "\x1b[") {
			require.True(t, strings.HasSuffix(line, "\x1b[0m"), "line %d %q does not end with a reset", i, line)
		}
	}
}

func TestTerminalColoursAMultiLineBlockCommentPerLine(t *testing.T) {
	theme := highlight.Theme{highlight.Comment: "italic"}

	output := terminalOne(t, theme, highlight.BlockComment, "/* first\n\n   last */")

	require.Equal(t, "\x1b[3m/* first\x1b[0m\n\n\x1b[3m   last */\x1b[0m", output)
}

func TestTerminalColoursAHeredocPerLine(t *testing.T) {
	theme := highlight.Theme{highlight.Heredoc: "yellow"}

	output := format(t, highlight.Terminal, theme, highlight.Tokenize([]byte("x = <<EOF\none\ntwo\nEOF\n")))

	require.Contains(t, output, "\x1b[33mone\x1b[0m\n\x1b[33mtwo\x1b[0m\n")
}

func TestTerminalWritesAttributesInOrderThenTheColour(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "underline italic dim bold red"}

	output := terminalOne(t, theme, highlight.Attribute, "a")

	require.Equal(t, "\x1b[1;2;3;4;31ma\x1b[0m", output)
}

func TestTerminalWritesANamedColourAsItsCode(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "blue"}

	output := terminalOne(t, theme, highlight.Attribute, "a")

	require.Equal(t, "\x1b[34ma\x1b[0m", output)
}

func TestTerminalWritesABrightColourAsItsCode(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bright-red"}

	output := terminalOne(t, theme, highlight.Attribute, "a")

	require.Equal(t, "\x1b[91ma\x1b[0m", output)
}

func TestTerminalWritesAHexForegroundAs24BitColour(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "#ff8800"}

	output := terminalOne(t, theme, highlight.Attribute, "a")

	require.Equal(t, "\x1b[38;2;255;136;0ma\x1b[0m", output)
}

func TestTerminalWritesAHexBackgroundAs24BitColour(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bg:#102030"}

	output := terminalOne(t, theme, highlight.Attribute, "a")

	require.Equal(t, "\x1b[48;2;16;32;48ma\x1b[0m", output)
}

func TestTerminalWritesTheForegroundBeforeTheBackground(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bg:#102030 bold #ff8800"}

	output := terminalOne(t, theme, highlight.Attribute, "a")

	require.Equal(t, "\x1b[1;38;2;255;136;0;48;2;16;32;48ma\x1b[0m", output)
}

func TestTerminalWritesAnUnstyledKindWithNoCodes(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "blue"}

	output := terminalOne(t, theme, highlight.Operator, "=")

	require.Equal(t, "=", output)
}

func TestTerminalWritesUnstyledTextWithNoCodes(t *testing.T) {
	output := terminalOne(t, highlight.DefaultTheme, highlight.Text, " {\n}\n")

	require.Equal(t, " {\n}\n", output)
}

func TestTerminalStylesTextWhenTheThemeStylesIt(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "dim"}

	output := terminalOne(t, theme, highlight.Text, " {\n}")

	require.Equal(t, "\x1b[2m {\x1b[0m\n\x1b[2m}\x1b[0m", output)
}

func TestTerminalStylesAKindByItsCategory(t *testing.T) {
	output := terminalOne(t, highlight.DefaultTheme, highlight.BlockType, "resource")

	require.Equal(t, "\x1b[1;35mresource\x1b[0m", output)
}

func TestTerminalWritesNoTokensAsNothing(t *testing.T) {
	output := format(t, highlight.Terminal, highlight.DefaultTheme, nil)

	require.Empty(t, output)
}

func TestTerminalReturnsTheWriterError(t *testing.T) {
	tokens := highlight.Tokenize(readSample(t))

	err := highlight.Terminal.Format(failingWriter{}, highlight.DefaultTheme, tokens)

	require.ErrorIs(t, err, errWriteFailed)
}

func TestTerminalUsesOnlyNamedColourCodesWithTheDefaultTheme(t *testing.T) {
	output := format(t, highlight.Terminal, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	for _, match := range sgrSequence.FindAllStringSubmatch(output, -1) {
		require.NotContains(t, match[1], "38;2", "24-bit colour in %q", match[0])
		require.NotContains(t, match[1], "48;2", "24-bit colour in %q", match[0])
	}
}
