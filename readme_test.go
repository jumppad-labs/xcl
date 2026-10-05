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

func TestReadmeDocumentsDecode(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "#### Filling a struct of your own")
	require.Contains(t, text, ".Decode(&")
	require.Contains(t, text, "ErrInvalidDecodeTarget")
}

// Decode is not generic, so unlike the query functions its method form works
// on every supported Go version
func TestReadmeSaysDecodeWorksAsAMethodOnEveryVersion(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "`Decode` is the exception to the rule")
}

func TestChangelogRecordsDecode(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	entry := string(data)
	require.Contains(t, entry, "## 20261003081552-e1e07cbe-config-decode")
	require.Contains(t, entry, "xcl.Decode(")
	require.Contains(t, entry, "ErrInvalidDecodeTarget")
}

func TestReadmeDocumentsReadingOutputsAsEntities(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "#### Reading values a configuration publishes")
	require.Contains(t, text, `xcl.Find[types.Output](c, "output.web_database")`)
	require.Contains(t, text, "out.Value")
	require.Contains(t, text, `xcl.FindByType[types.Output](c, "output")`)
	require.Contains(t, text, "c.Outputs()")
}

func TestReadmeNoLongerReadsAnOutputAsAPlainValue(t *testing.T) {
	text := readme(t)

	require.NotContains(t, text, `xcl.Find[string](c, "output.`)
	require.NotContains(t, text, `xcl.Find[string](c, "module.analytics.output.`)
}

func TestReadmeDocumentsTheModuleBoundary(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "### The module boundary")
	require.Contains(t, text, "only a module's outputs can be referenced from outside it")
	require.Contains(t, text, "value = module.b.output.value")
	require.Contains(t, text, "value = module.a.output.from_b")
}

func TestModulesGuideDocumentsTheModuleBoundary(t *testing.T) {
	data, err := os.ReadFile("docs/modules.md")
	require.NoError(t, err)

	guide := string(data)
	require.Contains(t, guide, "### The module boundary")
	require.Contains(t, guide, "A module's outputs are the only way to reach inside it")
	require.Contains(t, guide, "value = module.b.output.value")
}

func TestChangelogRecordsTheModuleBoundaryAndOutputEntities(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	entry := string(data)
	require.Contains(t, entry, "## 20261003153421-6ec0eab3-module-boundary-and-output-entities")
	require.Contains(t, entry, "xcl.Find[types.Output](")
	require.Contains(t, entry, "now fail validation")
	require.Contains(t, entry, "ErrTypeMismatch")
}
