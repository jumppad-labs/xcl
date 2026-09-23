package xcl

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// readme returns the README's text, so the tests below can check that what it
// documents is what the package actually offers
func readme(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile("README.md")
	require.NoError(t, err)

	return string(data)
}

func TestReadmeDocumentsConvertingAnEntity(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "### Converting to configuration text")
	require.Contains(t, text, "xcl.EncodeEntity(")
}

func TestReadmeDocumentsConvertingSavedData(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "xcl.EncodeSavedEntity(")
}

func TestReadmeDocumentsTheComputedOption(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "xcl.IncludeComputed()")
}

func TestReadmeDocumentsEveryEventDataLevel(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "xcl.WithEventData(")
	require.Contains(t, text, "EventDataNone")
	require.Contains(t, text, "EventDataRaw")
	require.Contains(t, text, "EventDataProcessed")
}

// the README used to describe Config.ToJSON and Parser.UnmarshalJSON, neither
// of which exists
func TestReadmeDescribesNoSerializationFunctionsThatDoNotExist(t *testing.T) {
	text := readme(t)

	require.NotContains(t, text, "ToJSON")
	require.NotContains(t, text, "UnmarshalJSON")
}

func TestChangelogRecordsTheConversionFunctions(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	entry := string(data)
	require.Contains(t, entry, "xcl.EncodeEntity(")
	require.Contains(t, entry, "xcl.EncodeSavedEntity(")
	require.Contains(t, entry, "xcl.WithEventData(")
	require.Contains(t, entry, "ErrNotEncodable")
}

// the event data change is breaking, an application reading Event.Data has to
// opt in to keep it
func TestChangelogRecordsTheEventDataBreakingChange(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	entry := string(data)
	breaking := entry[strings.Index(entry, "**Breaking:**"):]

	require.Contains(t, breaking, "Event.Data")
}
