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
	return highlight.RendererFunc(func(scope, text string) string {
		if scope == "" {
			return text
		}

		return "[" + scope + "]" + text + "[/" + scope + "]"
	})
}

// stripEncodeHighlightMarkers returns text with every marker
// encodeMarkerRenderer wrote removed
func stripEncodeHighlightMarkers(text string) string {
	return encodeHighlightMarkers.ReplaceAllString(text, "")
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

func TestEncodeEntityWithHighlightPassesEveryTokenToRenderer(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database, Highlight(encodeMarkerRenderer()))
	require.NoError(t, err)

	text := string(out)

	require.Contains(t, text, `[storage.type.xcl]resource[/storage.type.xcl]`)
	require.Contains(t, text, `[entity.name.type.xcl]"database"[/entity.name.type.xcl]`)
	require.Contains(t, text, `[entity.name.tag.xcl]"main"[/entity.name.tag.xcl]`)
	require.Contains(t, text, `[variable.other.property.xcl]location[/variable.other.property.xcl]`)
	require.Contains(t, text, `[string.quoted.double.xcl]"us-east"[/string.quoted.double.xcl]`)
	require.Contains(t, text, `[constant.numeric.xcl]5432[/constant.numeric.xcl]`)
	require.Contains(t, text, `[entity.name.function.block.xcl]timeouts[/entity.name.function.block.xcl]`)
}

func TestEncodeEntityWithHighlightStripsToPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	require.NoError(t, err)

	highlighted, err := EncodeEntity(container, Highlight(encodeMarkerRenderer()))
	require.NoError(t, err)

	require.NotEqual(t, string(plain), string(highlighted))
	require.Equal(t, string(plain), stripEncodeHighlightMarkers(string(highlighted)))
}

func TestEncodeSavedEntityWithHighlightStripsToPlainText(t *testing.T) {
	c, statePath := applyEncodeFixture(t)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	plain, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)

	highlighted, err := c.EncodeSavedEntity(record, Highlight(encodeMarkerRenderer()))
	require.NoError(t, err)

	require.NotEqual(t, string(plain), string(highlighted))
	require.Equal(t, string(plain), stripEncodeHighlightMarkers(string(highlighted)))
}

func TestEncodeEntityWithANSIHighlightStripsToPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	plain, err := EncodeEntity(container)
	require.NoError(t, err)

	highlighted, err := EncodeEntity(container, Highlight(renderer))
	require.NoError(t, err)

	require.Contains(t, string(highlighted), "\x1b[")
	require.Equal(t, string(plain), testutil.StripANSI(string(highlighted)))
}

func TestEncodeEntityWithHighlightAndShowReferencesLabelsReferenceRoots(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database, ShowReferences(), Highlight(encodeMarkerRenderer()))
	require.NoError(t, err)

	// location = variable.region, as the fixture wrote it
	expected := `[support.class.reference.xcl]variable[/support.class.reference.xcl]` +
		`[punctuation.accessor.xcl].[/punctuation.accessor.xcl]` +
		`[variable.other.member.xcl]region[/variable.other.member.xcl]`

	require.Contains(t, string(out), expected)
}

func TestEncodeEntityWithHighlightAndShowReferencesLabelsNestedReferenceRoots(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container, ShowReferences(), Highlight(encodeMarkerRenderer()))
	require.NoError(t, err)

	// name = resource.network.main.meta.name, as the fixture wrote it in each
	// network block
	expected := `[support.class.reference.xcl]resource[/support.class.reference.xcl]` +
		`[punctuation.accessor.xcl].[/punctuation.accessor.xcl]` +
		`[variable.other.member.xcl]network[/variable.other.member.xcl]`

	require.Contains(t, string(out), expected)
}

func TestEncodeEntityWithNilRendererMatchesPlainText(t *testing.T) {
	c, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	plain, err := EncodeEntity(container)
	require.NoError(t, err)

	withNil, err := EncodeEntity(container, Highlight(nil))
	require.NoError(t, err)

	require.Equal(t, string(plain), string(withNil))
}

func TestEncodeSavedEntityWithNilRendererMatchesPlainText(t *testing.T) {
	c, statePath := applyEncodeFixture(t)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	plain, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)

	withNil, err := c.EncodeSavedEntity(record, Highlight(nil))
	require.NoError(t, err)

	require.Equal(t, string(plain), string(withNil))
}
