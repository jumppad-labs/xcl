package highlight_test

import (
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// viewBox captures the width and height of an SVG view box at the origin
var viewBox = regexp.MustCompile(`viewBox="0 0 ([0-9]+) ([0-9]+)"`)

// svgOf formats xcl text as SVG with theme
func svgOf(t *testing.T, theme highlight.Theme, text string) string {
	t.Helper()

	return format(t, highlight.SVG, theme, highlight.Tokenize([]byte(text)))
}

// viewBoxOf returns the width and height of the SVG's view box
func viewBoxOf(t *testing.T, output string) (int, int) {
	t.Helper()

	match := viewBox.FindStringSubmatch(output)
	require.NotNil(t, match, "no view box in %q", output)

	width, err := strconv.Atoi(match[1])
	require.NoError(t, err)

	height, err := strconv.Atoi(match[2])
	require.NoError(t, err)

	return width, height
}

// requireWellFormedXML fails unless an XML decoder reads all of output
// without an error
func requireWellFormedXML(t *testing.T, output string) {
	t.Helper()

	decoder := xml.NewDecoder(strings.NewReader(output))
	for {
		_, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return
		}

		require.NoError(t, err)
	}
}

func TestSVGOfTheSampleIsWellFormedXML(t *testing.T) {
	output := format(t, highlight.SVG, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	requireWellFormedXML(t, output)
}

func TestSVGWithEveryStyleIsWellFormedXML(t *testing.T) {
	theme := highlight.DefaultTheme.With(highlight.Theme{
		highlight.Text:      "bg:#000000 #ffffff",
		highlight.Attribute: "bold italic underline dim #ff8800",
	})

	output := format(t, highlight.SVG, theme, highlight.Tokenize(readSample(t)))

	requireWellFormedXML(t, output)
}

func TestSVGIsAnSVGDocument(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, "a = 1\n")

	require.True(t, strings.HasPrefix(output, `<svg xmlns="http://www.w3.org/2000/svg"`), output)
	require.True(t, strings.HasSuffix(output, "</svg>\n"), output)
}

func TestSVGWritesOneTextElementForEachLineOfTheSample(t *testing.T) {
	input := readSample(t)

	output := format(t, highlight.SVG, highlight.DefaultTheme, highlight.Tokenize(input))

	lines := strings.Count(strings.TrimSuffix(string(input), "\n"), "\n") + 1
	require.Equal(t, lines, strings.Count(output, "<text "))
}

func TestSVGWritesOneTextElementForEachLine(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, "a = 1\n\nb = 2")

	require.Equal(t, 3, strings.Count(output, "<text "))
}

func TestSVGDoesNotAddALineForATrailingNewline(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, "a = 1\nb = 2\n")

	require.Equal(t, 2, strings.Count(output, "<text "))
}

func TestSVGViewBoxFitsTheLongestLineAndEveryLine(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, "a = 1\nlonger_name = 22\n")

	width, height := viewBoxOf(t, output)

	// 16 of margin and 6 for each of the 16 characters of the longest line
	require.GreaterOrEqual(t, width, 16+6*16)
	// 16 of margin and 14 for each of the 2 lines
	require.Equal(t, 16+14*2, height)
}

func TestSVGViewBoxIsExactlyWideEnoughForTheLongestLine(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, "a = 1\nlonger_name = 22\n")

	require.Contains(t, output, `viewBox="0 0 112 44"`)
}

func TestSVGViewBoxCountsCharactersRatherThanBytes(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, `x = "é"`)

	width, _ := viewBoxOf(t, output)

	require.Equal(t, 16+6*7, width)
}

func TestSVGViewBoxFitsTheSample(t *testing.T) {
	input := readSample(t)

	output := format(t, highlight.SVG, highlight.DefaultTheme, highlight.Tokenize(input))

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	longest := 0
	for _, line := range lines {
		longest = max(longest, utf8.RuneCountInString(line))
	}

	width, height := viewBoxOf(t, output)
	require.GreaterOrEqual(t, width, 16+6*longest)
	require.Equal(t, 16+14*len(lines), height)
}

func TestSVGWritesAStyledTokenInATspan(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "#ff8800"}

	output := svgOf(t, theme, "a = 1\n")

	require.Contains(t, output, `<tspan fill="#ff8800">a</tspan>`)
}

func TestSVGWritesANamedColourAsHex(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "red"}

	output := svgOf(t, theme, "a = 1\n")

	require.Contains(t, output, `<tspan fill="#cd3131">a</tspan>`)
}

func TestSVGWritesAttributesOfATspan(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "dim underline italic bold #ff8800"}

	output := svgOf(t, theme, "a = 1\n")

	require.Contains(t, output, `<tspan fill="#ff8800" font-weight="bold" font-style="italic" text-decoration="underline" opacity="0.5">a</tspan>`)
}

func TestSVGWritesAnUnstyledTokenWithoutATspan(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "#ff8800"}

	output := svgOf(t, theme, "a = 1\n")

	require.Equal(t, 1, strings.Count(output, "<tspan"))
}

func TestSVGWritesTextForegroundOnTheImage(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "#eeeeee"}
	tokens := []highlight.Token{{Kind: highlight.Text, Text: "{ }\n"}}

	output := format(t, highlight.SVG, theme, tokens)

	require.Contains(t, output, `font-size="10" fill="#eeeeee">`)
	require.NotContains(t, output, "<tspan")
}

func TestSVGFillsTheImageWithTextBackground(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "bg:#101010"}

	output := svgOf(t, theme, "a = 1\n")

	require.Contains(t, output, `<rect width="46" height="30" fill="#101010"/>`)
}

func TestSVGHasNoBackgroundWhenTextHasNone(t *testing.T) {
	output := svgOf(t, highlight.DefaultTheme, "a = 1\n")

	require.NotContains(t, output, "<rect")
}

func TestSVGEscapesText(t *testing.T) {
	tokens := []highlight.Token{{Kind: highlight.Text, Text: "a < b & c"}}

	output := format(t, highlight.SVG, highlight.DefaultTheme, tokens)

	require.Contains(t, output, "a &lt; b &amp; c")
	requireWellFormedXML(t, output)
}

func TestSVGEscapesTheTextOfATspan(t *testing.T) {
	theme := highlight.Theme{highlight.String: "yellow"}

	output := svgOf(t, theme, `x = "<a & b>"`)

	require.Contains(t, output, `<tspan fill="#e5e510">&quot;&lt;a &amp; b&gt;&quot;</tspan>`)
}

func TestSVGReturnsTheWriterError(t *testing.T) {
	err := highlight.SVG.Format(failingWriter{}, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	require.ErrorIs(t, err, errWriteFailed)
}
