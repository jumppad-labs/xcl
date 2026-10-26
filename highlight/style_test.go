package highlight_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

func TestParseStyleReadsBold(t *testing.T) {
	require.Equal(t, highlight.Style{Bold: true}, highlight.ParseStyle("bold"))
}

func TestParseStyleReadsItalic(t *testing.T) {
	require.Equal(t, highlight.Style{Italic: true}, highlight.ParseStyle("italic"))
}

func TestParseStyleReadsUnderline(t *testing.T) {
	require.Equal(t, highlight.Style{Underline: true}, highlight.ParseStyle("underline"))
}

func TestParseStyleReadsDim(t *testing.T) {
	require.Equal(t, highlight.Style{Dim: true}, highlight.ParseStyle("dim"))
}

func TestParseStyleReadsAHexForeground(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "#ff8800"}, highlight.ParseStyle("#ff8800"))
}

func TestParseStyleStoresAHexForegroundInLowerCase(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "#abcdef"}, highlight.ParseStyle("#ABCDEF"))
}

func TestParseStyleReadsAHexBackground(t *testing.T) {
	require.Equal(t, highlight.Style{Background: "#102030"}, highlight.ParseStyle("bg:#102030"))
}

func TestParseStyleStoresAHexBackgroundInLowerCase(t *testing.T) {
	require.Equal(t, highlight.Style{Background: "#a0b0c0"}, highlight.ParseStyle("bg:#A0B0C0"))
}

func TestParseStyleReadsBlack(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "black"}, highlight.ParseStyle("black"))
}

func TestParseStyleReadsRed(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "red"}, highlight.ParseStyle("red"))
}

func TestParseStyleReadsGreen(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "green"}, highlight.ParseStyle("green"))
}

func TestParseStyleReadsYellow(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "yellow"}, highlight.ParseStyle("yellow"))
}

func TestParseStyleReadsBlue(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "blue"}, highlight.ParseStyle("blue"))
}

func TestParseStyleReadsMagenta(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "magenta"}, highlight.ParseStyle("magenta"))
}

func TestParseStyleReadsCyan(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "cyan"}, highlight.ParseStyle("cyan"))
}

func TestParseStyleReadsWhite(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "white"}, highlight.ParseStyle("white"))
}

func TestParseStyleReadsABrightColour(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "bright-red"}, highlight.ParseStyle("bright-red"))
}

func TestParseStyleReadsBrightBlack(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "bright-black"}, highlight.ParseStyle("bright-black"))
}

func TestParseStyleReadsEveryWordTogether(t *testing.T) {
	style := highlight.ParseStyle("bold italic underline dim #112233 bg:#445566")

	require.Equal(t, highlight.Style{
		Bold:       true,
		Italic:     true,
		Underline:  true,
		Dim:        true,
		Foreground: "#112233",
		Background: "#445566",
	}, style)
}

func TestParseStyleLetsALaterForegroundWin(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "#ff8800"}, highlight.ParseStyle("red #ff8800"))
}

func TestParseStyleLetsALaterNamedForegroundWin(t *testing.T) {
	require.Equal(t, highlight.Style{Foreground: "blue"}, highlight.ParseStyle("#ff8800 blue"))
}

func TestParseStyleLetsALaterBackgroundWin(t *testing.T) {
	require.Equal(t, highlight.Style{Background: "#222222"}, highlight.ParseStyle("bg:#111111 bg:#222222"))
}

func TestParseStyleIgnoresExtraSpaces(t *testing.T) {
	require.Equal(t, highlight.Style{Bold: true, Foreground: "red"}, highlight.ParseStyle("  bold   red  "))
}

func TestParseStyleOfNothingIsTheZeroStyle(t *testing.T) {
	require.True(t, highlight.ParseStyle("").IsZero())
}

func TestParseStyleSkipsAnUnknownWord(t *testing.T) {
	require.Equal(t, highlight.Style{Bold: true}, highlight.ParseStyle("bold purple"))
}

func TestParseStyleDoesNotAcceptAShortHexColour(t *testing.T) {
	require.Equal(t, highlight.Style{}, highlight.ParseStyle("#fff"))
}

func TestParseStyleDoesNotAcceptAHexColourWithBadDigits(t *testing.T) {
	require.Equal(t, highlight.Style{}, highlight.ParseStyle("#gg0000"))
}

func TestParseStyleDoesNotAcceptANamedBackground(t *testing.T) {
	require.Equal(t, highlight.Style{}, highlight.ParseStyle("bg:red"))
}

func TestParseStyleDoesNotAcceptAColourWithoutAHash(t *testing.T) {
	require.Equal(t, highlight.Style{}, highlight.ParseStyle("ff8800"))
}

func TestParseStyleIsCaseSensitiveForWords(t *testing.T) {
	require.Equal(t, highlight.Style{}, highlight.ParseStyle("Bold RED"))
}

func TestStyleIsZeroForTheZeroStyle(t *testing.T) {
	require.True(t, highlight.Style{}.IsZero())
}

func TestStyleIsNotZeroWhenItSetsSomething(t *testing.T) {
	require.False(t, highlight.Style{Dim: true}.IsZero())
}

func TestColourHexOfRed(t *testing.T) {
	require.Equal(t, "#cd3131", highlight.Colour("red").Hex())
}

func TestColourHexOfBrightRed(t *testing.T) {
	require.Equal(t, "#f14c4c", highlight.Colour("bright-red").Hex())
}

func TestColourHexOfBrightBlack(t *testing.T) {
	require.Equal(t, "#666666", highlight.Colour("bright-black").Hex())
}

func TestColourHexOfAHexColourIsTheColour(t *testing.T) {
	require.Equal(t, "#ff8800", highlight.Colour("#ff8800").Hex())
}

func TestColourHexOfNoColourIsEmpty(t *testing.T) {
	require.Equal(t, "", highlight.Colour("").Hex())
}
