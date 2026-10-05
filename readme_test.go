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

func TestReadmeDocumentsSensitiveValues(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "## Sensitive values")
	require.Contains(t, text, "types.Sensitive[string]")
	require.Contains(t, text, "types.NewSensitive(")
}

func TestReadmeDocumentsRevealingASensitiveValue(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, ".Reveal()")
}

func TestReadmeDocumentsTheSensitiveToPlainValidationError(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "is not declared sensitive and cannot be")
}

func TestReadmeDocumentsTheSensitiveToPlainTypeError(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "xcl.ErrTypeMismatch")
	require.Contains(t, text, "is sensitive,")
}

func TestReadmeDocumentsTheMarkerInTheApplicationsOwnOutput(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "### Your own JSON and templates")
	require.Contains(t, text, `"password":"(sensitive)"`)
}

func TestReadmeDocumentsTheRevealSensitiveOption(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "xcl.RevealSensitive()")
	require.Contains(t, text, "logger.WithRevealSensitive(true)")
}

func TestReadmeNoLongerSaysSecretsAreShown(t *testing.T) {
	text := readme(t)

	require.NotContains(t, text, "anything secret is shown too")
}

func TestPluginGuideWarnsThatRevealedValuesAreUnprotected(t *testing.T) {
	data, err := os.ReadFile("docs/plugin-developer-guide.md")
	require.NoError(t, err)

	guide := string(data)
	require.Contains(t, guide, "## Sensitive fields")
	require.Contains(t, guide, "it is no longer\nprotected and must not be logged or otherwise emitted")
}

func TestChangelogRecordsSensitiveValues(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	require.Contains(t, string(data), "## 20261003134528-327e0657-references-and-secrets")
}

func TestChangelogListsTheStateStoreBreakingChange(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	require.Contains(t, string(data), "`StateStore.Save` receives each entity as a `json.RawMessage`")
}

func TestChangelogNoLongerSaysSecretsAreShown(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	require.NotContains(t, string(data), "so a password or other secret is shown too")
}

func TestReadmeDocumentsTheReferencesOption(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "**Showing references as written.**")
	require.Contains(t, text, "xcl.EncodeEntity(app, xcl.ShowReferences())")
	require.Contains(t, text, "  db_location = resource.postgres.main.location\n")
}

func TestReadmeQualifiesThatReferencesAreResolvedByDefault(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "By default references come\nout as the literal values they resolved to, unless you ask for\n`ShowReferences`.")
	require.NotContains(t, text, "**This text is for reading, not for reprocessing.** References come out as the")
}

func TestStateGuideDocumentsSavedReferences(t *testing.T) {
	data, err := os.ReadFile("docs/state.md")
	require.NoError(t, err)

	require.Contains(t, string(data), "Each saved record also carries `meta.references`")
}

func TestChangelogRecordsTheReferencesOption(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	changelog := string(data)
	require.Contains(t, changelog, "## 20261003153421-bf87d907-references-as-written")
	require.Contains(t, changelog, "`xcl.ShowReferences()`")
}

func TestChangelogSaysTheReferencesOptionBreaksNothing(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	changelog := string(data)
	start := strings.Index(changelog, "## 20261003153421-bf87d907-references-as-written")
	require.GreaterOrEqual(t, start, 0)

	end := strings.Index(changelog[start+3:], "\n## ")
	require.Greater(t, end, 0)

	entry := changelog[start : start+3+end]
	require.Contains(t, entry, "There are no breaking changes.")
	require.NotContains(t, entry, "**Breaking:**")
}

func TestReadmeDocumentsWrittenDependsOnIsShown(t *testing.T) {
	text := readme(t)

	require.Contains(t, text, "A `depends_on` list is written exactly as you wrote it, and\nleft out when you wrote none")
	require.Contains(t, text, "the dependencies xcl works out from references\nare never added to it, though they still order creation and destruction.")
}

func TestReadmeNoLongerSaysDependsOnIsNeverWritten(t *testing.T) {
	text := readme(t)

	require.NotContains(t, text, "and neither is `depends_on`")
}

func TestChangelogRecordsUserDependsOn(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)

	changelog := string(data)
	start := strings.Index(changelog, "## 20261003153421-c283547c-user-depends-on")
	require.GreaterOrEqual(t, start, 0)

	end := strings.Index(changelog[start+3:], "\n## ")
	require.Greater(t, end, 0)

	entry := changelog[start : start+3+end]
	require.Contains(t, entry, "**Breaking:**")
	require.Contains(t, entry, "`types.AppendUniqueLink`")
	require.Contains(t, entry, "`meta.parents`")
	require.Contains(t, entry, "`types.Meta.Parents`")
	require.Contains(t, entry, "Create and destroy order both come from each entity's `Meta.Links`")
}
