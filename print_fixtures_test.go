package xcl

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
)

// printFixtureDirectory holds the plain output captured from the encoder and
// the change-summary renderer as they were before printing was rebuilt, one
// file per fixture. The tests compare printing with them, so the rebuild
// cannot change the text unnoticed. They are test input: never regenerate
// them from the code under test.
const printFixtureDirectory = "testdata/print"

// printEntityFixture is a resource printed with computed values included,
// and the options it is printed with besides
type printEntityFixture struct {
	name    string
	entity  any
	options []EncodeOption
}

// printDiffFixture is a change summary
type printDiffFixture struct {
	name string
	diff *diff.Diff
}

// printFixtureEntities returns the fixed resources the captured output was
// written for: nested blocks, repeated blocks and lists, a computed value,
// references with a written dependency, and a sensitive value
func printFixtureEntities(t *testing.T) []printEntityFixture {
	t.Helper()

	c, _ := applyEncodeFixture(t)
	sensitive, _ := applyEncodeSensitiveConfig(t, encodeSensitiveSecretConfig)

	return []printEntityFixture{
		{name: "database", entity: encodeEntityByID(t, c, encodeDatabaseID)},
		{name: "network", entity: encodeEntityByID(t, c, encodeNetworkID)},
		{name: "container", entity: encodeEntityByID(t, c, encodeContainerID)},
		{name: "references", entity: encodeEntityByID(t, c, encodeDependentContainerID), options: []EncodeOption{ShowReferences()}},
		{name: "sensitive", entity: encodeEntityByID(t, sensitive, "resource.secret.login")},
	}
}

// printFixtureDiffs returns the fixed change summaries the captured output
// was written for: every resource action, every replace reason, dependency
// reasons, and every kind of change
func printFixtureDiffs() []printDiffFixture {
	return []printDiffFixture{
		{name: "example", diff: printExampleDiff()},
		{name: "reasons", diff: printReasonsDiff()},
		{name: "changes", diff: printChangesDiff()},
		{name: "nochange", diff: &diff.Diff{Summary: diff.Summary{Unchanged: 3}}},
	}
}

// printExampleDiff is the change summary of the diff package's example: one
// resource of each action, a nested path, an added element, a masked
// sensitive value and an unknown value
func printExampleDiff() *diff.Diff {
	return &diff.Diff{
		Summary: diff.Summary{Create: 1, Update: 2, Replace: 1, Delete: 1, Unchanged: 4},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("ports").Index(0).Attribute("host"), Before: float64(8080), After: float64(9090)},
					{Path: diff.Path{}.Attribute("ports").Index(2), After: map[string]any{"host": float64(443), "local": float64(8443)}},
					{Path: diff.Path{}.Attribute("env").Key("DB_PASSWORD"), Sensitive: true},
				},
			},
			{Address: "resource.container.cache", Action: diff.ActionUpdate},
			{
				Address: "resource.container.web",
				Action:  diff.ActionCreate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("image"), After: "nginx"},
					{Path: diff.Path{}.Attribute("db_host"), Unknown: true},
				},
			},
			{Address: "resource.network.app", Action: diff.ActionReplace, Reason: diff.ReplaceFailed},
			{Address: "resource.postgres.old", Action: diff.ActionDelete},
		},
	}
}

// printReasonsDiff gives every reason a resource is replaced or updated
func printReasonsDiff() *diff.Diff {
	return &diff.Diff{
		Summary: diff.Summary{Update: 2, Replace: 4},
		Resources: []diff.Resource{
			{Address: "resource.container.web", Action: diff.ActionReplace, Reason: diff.ReplaceDependency, ReplacedDeps: []string{"resource.network.app"}},
			{Address: "resource.container.worker", Action: diff.ActionReplace, Reason: diff.ReplaceDependency},
			{Address: "resource.container.proxy", Action: diff.ActionReplace, Reason: diff.ReplaceProvider},
			{Address: "resource.container.batch", Action: diff.ActionReplace},
			{
				Address: "resource.container.front",
				Action:  diff.ActionUpdate,
				Dependencies: []diff.Dependency{
					{Address: "resource.network.a", Action: diff.ActionReplace},
					{Address: "resource.network.b", Action: diff.ActionReplace},
					{Address: "resource.volume.data", Action: diff.ActionUpdate},
				},
			},
			{Address: "resource.container.back", Action: diff.ActionUpdate, Dependencies: []diff.Dependency{{Address: "resource.volume.data", Action: diff.ActionUpdate}}},
		},
	}
}

// printChangesDiff gives every kind of change to a value: changed, added,
// removed, becoming unknown, masked and revealed sensitive values
func printChangesDiff() *diff.Diff {
	return &diff.Diff{
		Summary: diff.Summary{Create: 1, Update: 1},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("image"), Before: "nginx:1", After: "nginx:2"},
					{Path: diff.Path{}.Attribute("ports").Index(1), Before: map[string]any{"host": float64(80)}},
					{Path: diff.Path{}.Attribute("ports").Index(2), After: map[string]any{"host": float64(443), "local": float64(8443)}},
					{Path: diff.Path{}.Attribute("address"), Before: "10.0.0.1", Unknown: true},
					{Path: diff.Path{}.Attribute("env").Key("API_TOKEN"), Sensitive: true},
					{Path: diff.Path{}.Attribute("env").Key("DB_PASSWORD"), Before: "hunter2-old-secret", After: "correct-horse-new-secret", Sensitive: true},
					{Path: diff.Path{}.Attribute("enabled"), Before: false, After: true},
				},
			},
			{
				Address: "resource.secret.login",
				Action:  diff.ActionCreate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("username"), After: "admin"},
					{Path: diff.Path{}.Attribute("password"), Sensitive: true},
					{Path: diff.Path{}.Attribute("tags"), After: []any{"web", "prod"}},
				},
			},
		},
	}
}

// readPrintFixture returns the captured output in file
func readPrintFixture(t *testing.T, file string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(printFixtureDirectory, file))
	require.NoError(t, err)

	return string(data)
}

// printFixtureEntity returns the fixed resource called name
func printFixtureEntity(t *testing.T, name string) printEntityFixture {
	t.Helper()

	for _, fixture := range printFixtureEntities(t) {
		if fixture.name == name {
			return fixture
		}
	}

	require.Failf(t, "no print fixture", "there is no fixed resource called %q", name)

	return printEntityFixture{}
}

// printFixtureDiff returns the fixed change summary called name
func printFixtureDiff(t *testing.T, name string) printDiffFixture {
	t.Helper()

	for _, fixture := range printFixtureDiffs() {
		if fixture.name == name {
			return fixture
		}
	}

	require.Failf(t, "no print fixture", "there is no fixed change summary called %q", name)

	return printDiffFixture{}
}

// requireEntityMatchesCapture prints the fixed resource called name as plain
// text with computed values included, and requires it equal the output
// captured before printing was rebuilt
func requireEntityMatchesCapture(t *testing.T, name string) {
	t.Helper()

	fixture := printFixtureEntity(t, name)
	options := []PrintOption{IncludeComputed()}
	for _, option := range fixture.options {
		options = append(options, option)
	}

	var out bytes.Buffer
	require.NoError(t, PrintEntity(&out, fixture.entity, options...))

	require.Equal(t, readPrintFixture(t, "entity_"+name+".txt"), out.String())
}

// requireDiffMatchesCapture prints the fixed change summary called name as
// plain text, and requires it equal the output captured before printing was
// rebuilt
func requireDiffMatchesCapture(t *testing.T, name string) {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintDiff(&out, printFixtureDiff(t, name).diff))

	require.Equal(t, readPrintFixture(t, "diff_"+name+".txt"), out.String())
}

func TestPrintFixtureDatabaseMatchesTheCapturedOutput(t *testing.T) {
	requireEntityMatchesCapture(t, "database")
}

func TestPrintFixtureNetworkMatchesTheCapturedOutput(t *testing.T) {
	requireEntityMatchesCapture(t, "network")
}

func TestPrintFixtureContainerMatchesTheCapturedOutput(t *testing.T) {
	requireEntityMatchesCapture(t, "container")
}

func TestPrintFixtureReferencesMatchesTheCapturedOutput(t *testing.T) {
	requireEntityMatchesCapture(t, "references")
}

func TestPrintFixtureSensitiveMatchesTheCapturedOutput(t *testing.T) {
	requireEntityMatchesCapture(t, "sensitive")
}

func TestPrintFixtureExampleDiffMatchesTheCapturedOutput(t *testing.T) {
	requireDiffMatchesCapture(t, "example")
}

func TestPrintFixtureReasonsDiffMatchesTheCapturedOutput(t *testing.T) {
	requireDiffMatchesCapture(t, "reasons")
}

func TestPrintFixtureChangesDiffMatchesTheCapturedOutput(t *testing.T) {
	requireDiffMatchesCapture(t, "changes")
}

func TestPrintFixtureNoChangeDiffMatchesTheCapturedOutput(t *testing.T) {
	requireDiffMatchesCapture(t, "nochange")
}
