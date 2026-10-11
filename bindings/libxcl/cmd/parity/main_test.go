package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// binDir holds the parity binary and the recording plugin, both built once
// per test binary by TestMain. It doubles as the -plugins directory because
// the parity program looks the plugin up by name under it.
var binDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "xcl-parity-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "parity test: create build directory:", err)
		os.Exit(1)
	}

	binDir = dir

	packageDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "parity test: working directory:", err)
		os.Exit(1)
	}

	if err := build(packageDir, filepath.Join(binDir, "parity"), "."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.RemoveAll(binDir)
		os.Exit(1)
	}

	if err := build(packageDir, filepath.Join(binDir, recordingPluginName), "../../testdata/plugins/recording"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.RemoveAll(binDir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(binDir)
	os.Exit(code)
}

// build runs go build for pkg, resolved from dir, writing the binary to out
func build(dir, out, pkg string) error {
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("parity test: go build %s: %w\n%s", pkg, err, output)
	}

	return nil
}

// fixtureDir returns the absolute path of a parity fixture
func fixtureDir(t *testing.T, name string) string {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "parity", name))
	require.NoError(t, err)

	return dir
}

// runParityRaw runs the parity binary on a fixture and returns the record it
// wrote, as written
func runParityRaw(t *testing.T, fixture string) []byte {
	t.Helper()

	out := filepath.Join(t.TempDir(), "record.json")

	cmd := exec.Command(filepath.Join(binDir, "parity"), "-fixture", fixtureDir(t, fixture), "-plugins", binDir, "-out", out)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "parity failed:\n%s", output)

	raw, err := os.ReadFile(out)
	require.NoError(t, err)

	return raw
}

// runParity runs the parity binary on a fixture and decodes the record
func runParity(t *testing.T, fixture string) map[string]any {
	t.Helper()

	var record map[string]any
	require.NoError(t, json.Unmarshal(runParityRaw(t, fixture), &record))

	return record
}

// entity finds the entity with the given meta id in a rendered entity list
func entity(t *testing.T, entities any, id string) map[string]any {
	t.Helper()

	list, ok := entities.([]any)
	require.True(t, ok, "entities is %T, not a list", entities)

	for _, e := range list {
		if entityID(e) == id {
			return e.(map[string]any)
		}
	}

	require.Failf(t, "entity missing", "no entity with id %q in %v", id, entities)

	return nil
}

func keysOf(m map[string]any) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

func TestParityRecordIsDeterministic(t *testing.T) {
	first := runParity(t, "basic_types")
	second := runParity(t, "basic_types")

	require.Equal(t, first, second)
}

func TestParityRecordIsDeterministicForPluginEntities(t *testing.T) {
	first := runParity(t, "plugin_entities")
	second := runParity(t, "plugin_entities")

	require.Equal(t, first, second)
}

func TestParityRecordCoversEveryStage(t *testing.T) {
	record := runParity(t, "annotations")

	require.ElementsMatch(t, []string{
		"validate",
		"diff_before",
		"apply",
		"entities",
		"outputs",
		"state",
		"diff_changed",
		"load",
		"destroy",
		"after_destroy",
	}, keysOf(record))

	require.Equal(t, true, record["validate"].(map[string]any)["ok"])
	require.Equal(t, true, record["apply"].(map[string]any)["ok"])

	entities := record["entities"].([]any)
	require.Len(t, entities, 1)

	web := entity(t, entities, "app.web")
	require.Equal(t, "nginx", web["image"])
	require.Equal(t, float64(2), web["replicas"])
	require.Equal(t, "eu", web["region"])
	require.Equal(t, "(sensitive)", web["password"])

	listener := web["listener"].(map[string]any)
	require.ElementsMatch(t, []string{"http", "https"}, keysOf(listener))

	require.Len(t, web["volume"].([]any), 2)

	state := record["state"].([]any)
	require.Len(t, state, 1)
	require.Equal(t, "s3cret", entity(t, state, "app.web")["password"])

	load := record["load"].(map[string]any)
	require.Equal(t, true, load["load"].(map[string]any)["ok"])
	require.Equal(t, record["entities"], load["entities"])

	require.Equal(t, true, record["destroy"].(map[string]any)["ok"])

	afterDestroy := record["after_destroy"].(map[string]any)
	require.Empty(t, afterDestroy["entities"])
	require.Empty(t, afterDestroy["state"])
}

func TestParityRecordHasNoTempOrFixturePaths(t *testing.T) {
	raw := runParityRaw(t, "plugin_entities")

	// the encoder escapes < and > in the file, so the placeholders are
	// checked on the decoded record and the real paths on the raw bytes,
	// where a slash is never escaped
	var record map[string]any
	require.NoError(t, json.Unmarshal(raw, &record))

	first := entity(t, record["entities"], "resource.recording.first")
	require.Equal(t, "<tmp>/recording.log", first["log"])
	require.Equal(t, "<fixture>/main.xcl", first["meta"].(map[string]any)["file"])

	require.False(t, bytes.Contains(raw, []byte(os.TempDir())), "record leaks the temporary directory:\n%s", raw)
	require.False(t, bytes.Contains(raw, []byte(fixtureDir(t, "plugin_entities"))), "record leaks the fixture directory:\n%s", raw)
	require.False(t, bytes.Contains(raw, []byte(binDir)), "record leaks the plugins directory:\n%s", raw)
}

func TestParityInvalidFixtureReportsEveryProblem(t *testing.T) {
	record := runParity(t, "invalid")

	require.ElementsMatch(t, []string{
		"validate",
		"diff_before",
		"apply",
		"entities",
		"outputs",
		"state",
	}, keysOf(record))

	validate := record["validate"].(map[string]any)
	require.Equal(t, false, validate["ok"])

	problems := validate["problems"].([]any)
	require.Len(t, problems, 2)

	lines := []float64{}
	summaries := []string{}
	for _, p := range problems {
		problem := p.(map[string]any)
		require.Equal(t, "config", problem["code"])
		require.Equal(t, "<fixture>/main.xcl", problem["file"])
		require.NotZero(t, problem["column"])

		lines = append(lines, problem["line"].(float64))
		summaries = append(summaries, problem["summary"].(string))
	}

	require.ElementsMatch(t, []float64{6, 11}, lines)

	joined := strings.Join(summaries, "\n")
	require.Contains(t, joined, "endpoint")
	require.Contains(t, joined, "computed")
	require.Contains(t, joined, "colour")

	require.Equal(t, false, record["apply"].(map[string]any)["ok"])
	require.Empty(t, record["entities"])
	require.Empty(t, record["state"])
}

func TestParityPluginEntitiesDiffReportsUpdate(t *testing.T) {
	record := runParity(t, "plugin_entities")

	diff := record["diff_changed"].(map[string]any)
	resources := diff["resources"].([]any)

	var first map[string]any
	for _, r := range resources {
		resource := r.(map[string]any)
		if resource["address"] == "resource.recording.first" {
			first = resource
		}
	}

	require.NotNil(t, first, "no diff entry for resource.recording.first in %v", resources)
	require.Equal(t, "update", first["action"])

	changes := first["changes"].([]any)
	require.Len(t, changes, 1)

	change := changes[0].(map[string]any)
	require.Equal(t, "value", change["path"])
	require.Equal(t, "one", change["before"])
	require.Equal(t, "changed", change["after"])
}

func TestParityFunctionsFixtureSlugifies(t *testing.T) {
	record := runParity(t, "functions")

	a := entity(t, record["entities"], "app.a")
	require.Equal(t, "my-app", a["name"])

	b := entity(t, record["entities"], "app.b")
	require.Equal(t, "my-app", b["name"])
	require.Equal(t, "(sensitive)", b["token"])

	stateB := entity(t, record["state"], "app.b")
	require.Equal(t, "hunter-two", stateB["token"])
}

func TestParityVariablesFixtureUsesPassedVariables(t *testing.T) {
	record := runParity(t, "variables")

	web := entity(t, record["entities"], "app.web")
	require.Equal(t, "us", web["region"])
	require.Equal(t, float64(2), web["replicas"])
}

func TestParityOutputsFixtureRecordsOutputs(t *testing.T) {
	record := runParity(t, "outputs")

	outputs := record["outputs"].(map[string]any)
	require.Equal(t, "eu", outputs["output.region"])
	require.Equal(t, []any{float64(80), float64(443)}, outputs["output.ports"])

	summary := outputs["output.summary"].(map[string]any)
	require.Equal(t, float64(2), summary["count"])
	require.Equal(t, "web", summary["name"])
}

func TestParityMissingFixtureFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	cmd := exec.Command(filepath.Join(binDir, "parity"), "-fixture", missing, "-plugins", binDir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.NotEqual(t, 0, exitErr.ExitCode())
	require.Contains(t, stderr.String(), "main.xcl")
}
