package xcl

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// The plain output of each fixed resource is compared with the output
// captured before printing was rebuilt in print_fixtures_test.go.

// printVSCodeTheme is a VS Code colour theme colouring attribute names, the
// scope variable.other.property, #ff8800
const printVSCodeTheme = `{
	// a theme of one colour
	"name": "orange properties",
	"tokenColors": [
		{
			"scope": "variable.other.property",
			"settings": { "foreground": "#ff8800" },
		},
	],
}`

// svgViewBox matches the view box of an SVG image, capturing its width
var svgViewBox = regexp.MustCompile(`viewBox="0 0 ([0-9]+) [0-9]+"`)

// printedEntity returns entity printed with options
func printedEntity(t *testing.T, entity any, options ...PrintOption) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintEntity(&out, entity, options...))

	return out.String()
}

// longestLine returns the number of characters in the longest line of text
func longestLine(text string) int {
	longest := 0
	for _, line := range strings.Split(text, "\n") {
		longest = max(longest, utf8.RuneCountInString(line))
	}

	return longest
}

func TestEntityTokensJoinToThePlainPrintedEntity(t *testing.T) {
	fixture := printFixtureEntity(t, "container")

	tokens, err := EntityTokens(fixture.entity, IncludeComputed())
	require.NoError(t, err)

	require.Equal(t, printedEntity(t, fixture.entity, IncludeComputed()), joinedTokenText(tokens))
}

func TestPrintEntityTakesEncodeAndPrintOptionsInOneCall(t *testing.T) {
	fixture := printFixtureEntity(t, "container")

	out := printedEntity(t, fixture.entity, IncludeComputed(), WithFormatter(highlight.Terminal))

	require.Contains(t, out, "\x1b[")
	require.Contains(t, testutil.StripANSI(out), `assigned_address = "assigned-main" # set by the provider`)
}

func TestPrintEntityAsSVGIsWellFormedXML(t *testing.T) {
	fixture := printFixtureEntity(t, "database")

	out := printedEntity(t, fixture.entity, WithFormatter(highlight.SVG))

	decoder := xml.NewDecoder(strings.NewReader(out))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
}

func TestPrintEntityAsSVGWritesOneTextElementPerLine(t *testing.T) {
	fixture := printFixtureEntity(t, "database")
	plain := printedEntity(t, fixture.entity)

	out := printedEntity(t, fixture.entity, WithFormatter(highlight.SVG))

	require.Equal(t, 9, strings.Count(plain, "\n"))
	require.Equal(t, 9, strings.Count(out, "<text "))
}

func TestPrintEntityAsSVGFitsTheLongestLineInItsViewBox(t *testing.T) {
	fixture := printFixtureEntity(t, "database")
	plain := printedEntity(t, fixture.entity)

	out := printedEntity(t, fixture.entity, WithFormatter(highlight.SVG))

	match := svgViewBox.FindStringSubmatch(out)
	require.NotNil(t, match, "the image has no view box")
	width, err := strconv.Atoi(match[1])
	require.NoError(t, err)

	require.GreaterOrEqual(t, width, 16+6*longestLine(plain))
}

func TestPrintEntityWithAVSCodeThemeColoursAttributeNamesAsTheThemeDoes(t *testing.T) {
	theme, err := highlight.LoadVSCodeTheme(strings.NewReader(printVSCodeTheme))
	require.NoError(t, err)
	fixture := printFixtureEntity(t, "database")

	out := printedEntity(t, fixture.entity, WithFormatter(highlight.Terminal), WithTheme(theme))

	require.Contains(t, out, "\x1b[38;2;255;136;0mlocation\x1b[0m")
	require.Contains(t, out, "\x1b[38;2;255;136;0mport\x1b[0m")
}

func TestPrintEntityOfAValueThatIsNotAnEntityFailsAsNotEncodable(t *testing.T) {
	var out bytes.Buffer

	err := PrintEntity(&out, 42)

	require.ErrorIs(t, err, ErrNotEncodable)
}

func TestPrintEntityOfAValueThatIsNotAnEntityWritesNothing(t *testing.T) {
	var out bytes.Buffer

	_ = PrintEntity(&out, 42, WithFormatter(highlight.HTML))

	require.Empty(t, out.String())
}
