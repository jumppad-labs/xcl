package diff

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatValueWritesNilAsNull(t *testing.T) {
	require.Equal(t, "null", formatValue(nil))
}

func TestFormatValueQuotesString(t *testing.T) {
	require.Equal(t, `"nginx"`, formatValue("nginx"))
}

func TestFormatValueEscapesQuoteInString(t *testing.T) {
	require.Equal(t, `"say \"hi\""`, formatValue(`say "hi"`))
}

func TestFormatValueEscapesTemplateSequenceInString(t *testing.T) {
	require.Equal(t, `"$${name}"`, formatValue("${name}"))
}

func TestFormatValueWritesInt(t *testing.T) {
	require.Equal(t, "8080", formatValue(8080))
}

func TestFormatValueWritesNegativeInt(t *testing.T) {
	require.Equal(t, "-3", formatValue(int64(-3)))
}

func TestFormatValueWritesUint(t *testing.T) {
	require.Equal(t, "42", formatValue(uint16(42)))
}

func TestFormatValueWritesWholeFloatWithoutFraction(t *testing.T) {
	require.Equal(t, "8080", formatValue(float64(8080)))
}

func TestFormatValueWritesFractionalFloat(t *testing.T) {
	require.Equal(t, "1.5", formatValue(1.5))
}

func TestFormatValueWritesFloat32InShortestForm(t *testing.T) {
	require.Equal(t, "0.1", formatValue(float32(0.1)))
}

func TestFormatValueWritesJSONNumberAsItsText(t *testing.T) {
	require.Equal(t, "12.50", formatValue(json.Number("12.50")))
}

func TestFormatValueWritesTrue(t *testing.T) {
	require.Equal(t, "true", formatValue(true))
}

func TestFormatValueWritesFalse(t *testing.T) {
	require.Equal(t, "false", formatValue(false))
}

func TestFormatValueWritesSliceAsList(t *testing.T) {
	require.Equal(t, `["a", "b"]`, formatValue([]any{"a", "b"}))
}

func TestFormatValueWritesEmptySliceAsEmptyList(t *testing.T) {
	require.Equal(t, "[]", formatValue([]any{}))
}

func TestFormatValueWritesMapWithSortedKeys(t *testing.T) {
	value := map[string]any{"local": 8443, "host": 443}

	require.Equal(t, "{ host = 443, local = 8443 }", formatValue(value))
}

func TestFormatValueQuotesNonIdentifierMapKey(t *testing.T) {
	value := map[string]any{"my-key.x": "v"}

	require.Equal(t, `{ "my-key.x" = "v" }`, formatValue(value))
}

func TestFormatValueWritesEmptyMapAsEmptyObject(t *testing.T) {
	require.Equal(t, "{}", formatValue(map[string]any{}))
}

func TestFormatValueDereferencesPointer(t *testing.T) {
	name := "web"

	require.Equal(t, `"web"`, formatValue(&name))
}

func TestFormatValueWritesNilPointerAsNull(t *testing.T) {
	var name *string

	require.Equal(t, "null", formatValue(name))
}

func TestFormatValueWritesNestedValuesOnOneLine(t *testing.T) {
	value := map[string]any{
		"ports": []any{
			map[string]any{"host": float64(80)},
			map[string]any{"host": float64(443)},
		},
		"name": "web",
	}

	require.Equal(t, `{ name = "web", ports = [{ host = 80 }, { host = 443 }] }`, formatValue(value))
}
