package xcl

import (
	"bytes"
	"io"
	"regexp"
	"testing"

	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// encodeHighlightMarkers matches the open and close markers
// encodeMarkerFormatter writes around each labelled token
var encodeHighlightMarkers = regexp.MustCompile(`\[/?[a-z.\-]+\]`)

// encodeMarkerFormatter wraps each labelled token as [scope]text[/scope],
// naming its kind's TextMate scope, and passes Text tokens through unchanged,
// so a test can read which kind each token was given
var encodeMarkerFormatter = highlight.FormatterFunc(func(w io.Writer, _ highlight.Theme, tokens []highlight.Token) error {
	for _, token := range tokens {
		text := token.Text
		if token.Kind != highlight.Text {
			scope := token.Kind.Scope()
			text = "[" + scope + "]" + token.Text + "[/" + scope + "]"
		}

		if _, err := io.WriteString(w, text); err != nil {
			return err
		}
	}

	return nil
})

// stripEncodeHighlightMarkers returns text with every marker
// encodeMarkerFormatter wrote removed
func stripEncodeHighlightMarkers(text string) string {
	return encodeHighlightMarkers.ReplaceAllString(text, "")
}

// printed returns entity printed with options
func printed(t *testing.T, entity any, options ...PrintOption) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintEntity(&out, entity, options...))

	return out.String()
}

// formatted returns encoded configuration text labelled by highlight.Tokenize
// and written through formatter
func formatted(t *testing.T, text []byte, err error, formatter highlight.Formatter) string {
	t.Helper()

	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, formatter.Format(&out, highlight.DefaultTheme, highlight.Tokenize(text)))

	return out.String()
}

func TestEncodeEntityWithoutHighlightHasNoColourCodes(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database)
	require.NoError(t, err)

	expected := `resource "database" "main" {
  location = "us-east"
  port     = 5432

  timeouts {
    connect = 30
    read    = 60
  }
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), "\x1b")
}

func TestPrintedEntityLabelsEveryToken(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	text := printed(t, database, WithFormatter(encodeMarkerFormatter))

	require.Contains(t, text, `[storage.type.xcl]resource[/storage.type.xcl]`)
	require.Contains(t, text, `[entity.name.type.xcl]"database"[/entity.name.type.xcl]`)
	require.Contains(t, text, `[entity.name.tag.xcl]"main"[/entity.name.tag.xcl]`)
	require.Contains(t, text, `[variable.other.property.xcl]location[/variable.other.property.xcl]`)
	require.Contains(t, text, `[string.quoted.double.xcl]"us-east"[/string.quoted.double.xcl]`)
	require.Contains(t, text, `[constant.numeric.xcl]5432[/constant.numeric.xcl]`)
	require.Contains(t, text, `[entity.name.function.block.xcl]timeouts[/entity.name.function.block.xcl]`)
}

func TestPrintedEntityMarkersStripToPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	require.NoError(t, err)
	highlighted := printed(t, container, WithFormatter(encodeMarkerFormatter))

	require.NotEqual(t, string(plain), highlighted)
	require.Equal(t, string(plain), stripEncodeHighlightMarkers(highlighted))
}

func TestFormattedSavedEntityMarkersStripToPlainText(t *testing.T) {
	c, statePath := applyEncodeFixture(t)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	plain, err := c.EncodeSavedEntity(record)
	highlighted := formatted(t, plain, err, encodeMarkerFormatter)

	require.NotEqual(t, string(plain), highlighted)
	require.Equal(t, string(plain), stripEncodeHighlightMarkers(highlighted))
}

func TestEncodedEntityPrintedForATerminalStripsToPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	require.NoError(t, err)
	highlighted := printed(t, container, WithFormatter(highlight.Terminal))

	require.Contains(t, highlighted, "\x1b[")
	require.Equal(t, string(plain), testutil.StripANSI(highlighted))
}

func TestEncodedEntityPrintedAsPlainTextIsTheEncodedText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	require.NoError(t, err)

	require.Equal(t, string(plain), printed(t, container, WithFormatter(highlight.Plain)))
}

func TestPrintedEntityWithShowReferencesLabelsReferenceRoots(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	text := printed(t, database, ShowReferences(), WithFormatter(encodeMarkerFormatter))

	// location = variable.region, as the fixture wrote it
	expected := `[support.class.reference.xcl]variable[/support.class.reference.xcl]` +
		`[punctuation.accessor.xcl].[/punctuation.accessor.xcl]` +
		`[variable.other.member.xcl]region[/variable.other.member.xcl]`

	require.Contains(t, text, expected)
}

func TestPrintedEntityWithShowReferencesLabelsNestedReferenceRoots(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	text := printed(t, container, ShowReferences(), WithFormatter(encodeMarkerFormatter))

	// name = resource.network.main.meta.name, as the fixture wrote it in each
	// network block
	expected := `[support.class.reference.xcl]resource[/support.class.reference.xcl]` +
		`[punctuation.accessor.xcl].[/punctuation.accessor.xcl]` +
		`[variable.other.member.xcl]network[/variable.other.member.xcl]`

	require.Contains(t, text, expected)
}
