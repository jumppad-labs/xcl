package e2e_test

// The plugin changes tests run the recorder provider, from fixtures/recorder,
// both in-process and as the external binary TestMain builds, apply the same
// edit through each and require that both were told exactly the same
// property changes and dependency changes, when deciding what changed and
// when updating.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/recorder"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
)

// recorderConfiguration declares two recorders, a reads b's computed output,
// %s is the path of the log both recorders append to
const recorderConfiguration = `resource "recorder" "b" {
  replace_key = "k1"
  log         = "%s"
}

resource "recorder" "a" {
  value       = "first"
  secret      = "first-secret"
  input       = resource.recorder.b.output
  replace_key = "a1"
  log         = "%s"
}
`

// recorderInProcessPlugin registers the recorder provider in this process,
// the same way the external fixture binary does
type recorderInProcessPlugin struct {
	plugins.PluginBase
}

// Init registers the recorder block type with the recorder provider
func (p *recorderInProcessPlugin) Init(log logger.Logger, state plugins.State) error {
	return recorder.Register(&p.PluginBase, log, state)
}

// recorderScenario is a diffScenario for recorderConfiguration with the path
// of the log the recorders append to
type recorderScenario struct {
	*diffScenario
	logPath string
}

// newRecorderScenario writes recorderConfiguration to a temporary directory
// and returns a scenario for it, with the recorder plugin registered in local,
// the log in its own temporary directory and state encrypted with
// testStateKey
func newRecorderScenario(t *testing.T, local *registry.Local) *recorderScenario {
	t.Helper()

	logPath := filepath.Join(t.TempDir(), "log.jsonl")

	configDir := t.TempDir()
	configuration := fmt.Sprintf(recorderConfiguration, logPath, logPath)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "main.xcl"), []byte(configuration), 0644))

	scenario := &diffScenario{
		t:         t,
		configDir: configDir,
		stateDir:  t.TempDir(),
		recorder:  &testutil.EventRecorder{},
	}

	scenario.config = newConfig(t, scenario.recorder.Record, scenario.stateDir, testStateKey, xcl.WithRegistry(local))

	return &recorderScenario{diffScenario: scenario, logPath: logPath}
}

// newInProcessRecorderScenario returns a recorder scenario with the recorder
// plugin running in-process
func newInProcessRecorderScenario(t *testing.T) *recorderScenario {
	t.Helper()

	local := registry.NewLocal()
	local.RegisterPlugin(&recorderInProcessPlugin{})

	return newRecorderScenario(t, local)
}

// newExternalRecorderScenario returns a recorder scenario with the recorder
// plugin running as the external binary TestMain builds
func newExternalRecorderScenario(t *testing.T) *recorderScenario {
	t.Helper()

	local := registry.NewLocal()
	local.RegisterExternalPlugin(externalPlugin)

	return newRecorderScenario(t, local)
}

// applyRecorderEdit applies the scenario's configuration, then edits a's
// plain value, a's sensitive secret and b's replace_key, which the recorder
// answers with replace, and applies again. It returns every record the
// recorders appended to the log across both applies.
func applyRecorderEdit(s *recorderScenario) []recorder.Record {
	s.t.Helper()

	s.apply()

	s.edit("main.xcl", `value       = "first"`, `value       = "second"`)
	s.edit("main.xcl", `secret      = "first-secret"`, `secret      = "second-secret"`)
	s.edit("main.xcl", `replace_key = "k1"`, `replace_key = "k2"`)

	s.apply()

	return readRecorderLog(s.t, s.logPath)
}

// readRecorderLog returns every record in the recorder log at path, in the
// order they were appended
func readRecorderLog(t *testing.T, path string) []recorder.Record {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	records := []recorder.Record{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		record := recorder.Record{}
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))

		records = append(records, record)
	}
	require.NoError(t, scanner.Err())

	return records
}

// recordsForCall returns the records of the given call, "changed" or
// "update", keyed by resource id. It requires there is at most one record
// per resource for the call.
func recordsForCall(t *testing.T, records []recorder.Record, call string) map[string]recorder.Record {
	t.Helper()

	found := map[string]recorder.Record{}
	for _, record := range records {
		if record.Call != call {
			continue
		}

		_, duplicate := found[record.ID]
		require.False(t, duplicate, "more than one %s record for %s", call, record.ID)

		found[record.ID] = record
	}

	return found
}

// recordedChangeAt returns the change in record at path, requiring there is
// exactly one
func recordedChangeAt(t *testing.T, record recorder.Record, path string) recorder.RecordedChange {
	t.Helper()

	matches := []recorder.RecordedChange{}
	for _, change := range record.Changes {
		if change.Path == path {
			matches = append(matches, change)
		}
	}

	require.Len(t, matches, 1, "expected exactly one change at %s in %+v", path, record.Changes)

	return matches[0]
}

func TestRecorderEditIsToldTheSameChangesWhenDecidingInProcessAndExternal(t *testing.T) {
	inProcess := recordsForCall(t, applyRecorderEdit(newInProcessRecorderScenario(t)), "changed")
	external := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "changed")

	require.NotEmpty(t, inProcess)
	require.Contains(t, inProcess, "resource.recorder.a")
	require.Contains(t, inProcess, "resource.recorder.b")
	require.Equal(t, inProcess, external)
}

func TestRecorderEditIsToldTheSameChangesWhenUpdatingInProcessAndExternal(t *testing.T) {
	inProcess := recordsForCall(t, applyRecorderEdit(newInProcessRecorderScenario(t)), "update")
	external := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "update")

	require.NotEmpty(t, inProcess)
	require.Contains(t, inProcess, "resource.recorder.a")
	require.Equal(t, inProcess, external)
}

func TestRecorderEditIsToldThePlainValueChangeWhenDeciding(t *testing.T) {
	changed := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "changed")

	expected := recorder.RecordedChange{
		Path:   "value",
		Before: "first",
		After:  "second",
	}
	require.Equal(t, expected, recordedChangeAt(t, changed["resource.recorder.a"], "value"))
}

func TestRecorderEditIsToldThePlainValueChangeWhenUpdating(t *testing.T) {
	updated := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "update")

	expected := recorder.RecordedChange{
		Path:   "value",
		Before: "first",
		After:  "second",
	}
	require.Equal(t, expected, recordedChangeAt(t, updated["resource.recorder.a"], "value"))
}

func TestRecorderEditIsToldTheRealSensitiveChangeWhenDeciding(t *testing.T) {
	changed := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "changed")

	expected := recorder.RecordedChange{
		Path:      "secret",
		Before:    "first-secret",
		After:     "second-secret",
		Sensitive: true,
	}
	require.Equal(t, expected, recordedChangeAt(t, changed["resource.recorder.a"], "secret"))
}

func TestRecorderEditIsToldTheRealSensitiveChangeWhenUpdating(t *testing.T) {
	updated := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "update")

	expected := recorder.RecordedChange{
		Path:      "secret",
		Before:    "first-secret",
		After:     "second-secret",
		Sensitive: true,
	}
	require.Equal(t, expected, recordedChangeAt(t, updated["resource.recorder.a"], "secret"))
}

func TestRecorderEditIsToldTheReplacedDependencyValueIsUnknownWhenDeciding(t *testing.T) {
	changed := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "changed")

	expected := recorder.RecordedChange{
		Path:    "input",
		Before:  "b-k1",
		After:   nil,
		Unknown: true,
	}
	require.Equal(t, expected, recordedChangeAt(t, changed["resource.recorder.a"], "input"))
}

func TestRecorderEditIsToldTheReplacedDependencyValueWhenUpdating(t *testing.T) {
	updated := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "update")

	expected := recorder.RecordedChange{
		Path:   "input",
		Before: "b-k1",
		After:  "b-k2",
	}
	require.Equal(t, expected, recordedChangeAt(t, updated["resource.recorder.a"], "input"))
}

func TestRecorderEditIsToldTheDependencyWasReplacedWhenDeciding(t *testing.T) {
	changed := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "changed")

	expected := []recorder.RecordedDependency{
		{Address: "resource.recorder.b", Change: "replace"},
	}
	require.Equal(t, expected, changed["resource.recorder.a"].Dependencies)
}

func TestRecorderEditIsToldTheDependencyWasReplacedWhenUpdating(t *testing.T) {
	updated := recordsForCall(t, applyRecorderEdit(newExternalRecorderScenario(t)), "update")

	expected := []recorder.RecordedDependency{
		{Address: "resource.recorder.b", Change: "replace"},
	}
	require.Equal(t, expected, updated["resource.recorder.a"].Dependencies)
}
