package xcl

import (
	"regexp"
	"testing"

	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// encodeHighlightMarkers matches the open and close markers
// encodeMarkerRenderer writes around each labelled piece
var encodeHighlightMarkers = regexp.MustCompile(`\[/?[a-z.\-]+\]`)

// encodeMarkerRenderer wraps each labelled piece as [scope]text[/scope] and
// passes unlabelled text through unchanged, so a test can read which scope
// each piece was given
func encodeMarkerRenderer() highlight.Renderer {
	return highlight.PieceFunc(func(p highlight.Piece) string {
		if p.Scope == "" {
			return p.Text
		}

		return "[" + p.Scope + "]" + p.Text + "[/" + p.Scope + "]"
	})
}

// stripEncodeHighlightMarkers returns text with every marker
// encodeMarkerRenderer wrote removed
func stripEncodeHighlightMarkers(text string) string {
	return encodeHighlightMarkers.ReplaceAllString(text, "")
}

// rendered returns encoded configuration text rendered through renderer
func rendered(t *testing.T, text []byte, err error, renderer highlight.Renderer) string {
	t.Helper()

	require.NoError(t, err)

	return string(renderer.Render(highlight.Pieces(text)))
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

func TestEncodedEntityPiecesLabelEveryToken(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database)
	text := rendered(t, out, err, encodeMarkerRenderer())

	require.Contains(t, text, `[storage.type.xcl]resource[/storage.type.xcl]`)
	require.Contains(t, text, `[entity.name.type.xcl]"database"[/entity.name.type.xcl]`)
	require.Contains(t, text, `[entity.name.tag.xcl]"main"[/entity.name.tag.xcl]`)
	require.Contains(t, text, `[variable.other.property.xcl]location[/variable.other.property.xcl]`)
	require.Contains(t, text, `[string.quoted.double.xcl]"us-east"[/string.quoted.double.xcl]`)
	require.Contains(t, text, `[constant.numeric.xcl]5432[/constant.numeric.xcl]`)
	require.Contains(t, text, `[entity.name.function.block.xcl]timeouts[/entity.name.function.block.xcl]`)
}

func TestEncodedEntityPiecesStripToPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	highlighted := rendered(t, plain, err, encodeMarkerRenderer())

	require.NotEqual(t, string(plain), highlighted)
	require.Equal(t, string(plain), stripEncodeHighlightMarkers(highlighted))
}

func TestEncodedSavedEntityPiecesStripToPlainText(t *testing.T) {
	c, statePath := applyEncodeFixture(t)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	plain, err := c.EncodeSavedEntity(record)
	highlighted := rendered(t, plain, err, encodeMarkerRenderer())

	require.NotEqual(t, string(plain), highlighted)
	require.Equal(t, string(plain), stripEncodeHighlightMarkers(highlighted))
}

func TestEncodedEntityRenderedAsANSIStripsToPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	highlighted := rendered(t, plain, err, highlight.ANSI)

	require.Contains(t, highlighted, "\x1b[")
	require.Equal(t, string(plain), testutil.StripANSI(highlighted))
}

func TestEncodedEntityRenderedAsTextIsThePlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)

	require.Equal(t, string(plain), rendered(t, plain, err, highlight.Text))
}

func TestEncodedEntityWithShowReferencesLabelsReferenceRoots(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database, ShowReferences())
	text := rendered(t, out, err, encodeMarkerRenderer())

	// location = variable.region, as the fixture wrote it
	expected := `[support.class.reference.xcl]variable[/support.class.reference.xcl]` +
		`[punctuation.accessor.xcl].[/punctuation.accessor.xcl]` +
		`[variable.other.member.xcl]region[/variable.other.member.xcl]`

	require.Contains(t, text, expected)
}

func TestEncodedEntityWithShowReferencesLabelsNestedReferenceRoots(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container, ShowReferences())
	text := rendered(t, out, err, encodeMarkerRenderer())

	// name = resource.network.main.meta.name, as the fixture wrote it in each
	// network block
	expected := `[support.class.reference.xcl]resource[/support.class.reference.xcl]` +
		`[punctuation.accessor.xcl].[/punctuation.accessor.xcl]` +
		`[variable.other.member.xcl]network[/variable.other.member.xcl]`

	require.Contains(t, text, expected)
}
