package xcl

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// diffFixture is everything a diff test needs: a Config wired to a real file
// state store, the TestPlugin that records every provider call, the store and
// its file, the directory holding the configuration and the recorder
// receiving every event
type diffFixture struct {
	config    *Config
	plugin    *parser.TestPlugin
	store     *state.FileStateStore
	statePath string
	configDir string
	recorder  *eventRecorder
}

// setupDiffConfig builds a Config with the TestPlugin registered, a file
// state store and an event recorder. The configuration lives in a temporary
// directory that starts empty, useDiffConfiguration copies a fixture into it
// so a test can apply one configuration and diff another.
func setupDiffConfig(t *testing.T) *diffFixture {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	local := registry.NewLocal()

	testPlugin := &parser.TestPlugin{}
	local.RegisterPlugin(testPlugin)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	recorder := &eventRecorder{}

	c, err := NewConfig(
		WithRegistry(local),
		WithStateStore(store),
		WithEventHandler(recorder.Record),
		// the diff tests assert on what events carry, which is nothing
		// unless a level asks for it
		WithEventData(EventDataRaw),
	)
	require.NoError(t, err)

	return &diffFixture{
		config:    c,
		plugin:    testPlugin,
		store:     store,
		statePath: store.Path(),
		configDir: t.TempDir(),
		recorder:  recorder,
	}
}

// useDiffConfiguration replaces the fixture's configuration with the
// main.xcl of the named directory under internal/test_fixtures/config/diff
func useDiffConfiguration(t *testing.T, f *diffFixture, name string) {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join("./internal/test_fixtures/config/diff", name, "main.xcl"))
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(f.configDir, "main.xcl"), contents, 0644)
	require.NoError(t, err)
}

// applyDiffConfiguration applies the named configuration, then clears the
// plugin's recorded calls and the recorder's events so a test sees only what
// the diff does
func applyDiffConfiguration(t *testing.T, f *diffFixture, name string) {
	t.Helper()

	useDiffConfiguration(t, f, name)

	err := f.config.Apply(f.configDir)
	require.NoError(t, err)

	f.plugin.ResetCalls()
	f.recorder.reset()
}

// resourceActions returns "<address> <action>" for every resource a diff
// lists, in the order it lists them
func resourceActions(result *diff.Diff) []string {
	found := []string{}
	for _, r := range result.Resources {
		found = append(found, r.Address+" "+string(r.Action))
	}

	return found
}

// coreDiffLog returns the core log events of the diff operation with the
// given message
func coreDiffLog(r *eventRecorder, message string) []Event {
	found := []Event{}
	for _, e := range r.Events() {
		if e.Phase != events.PhaseLog || e.Source != events.SourceCore || e.Operation != events.OperationDiff {
			continue
		}

		if e.Meta[events.KeyMessage] == message {
			found = append(found, e)
		}
	}

	return found
}

func TestDiffOfAppliedConfigurationReportsNoChanges(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Empty(t, result.Resources)
	require.Equal(t, 0, result.Summary.Create)
	require.Equal(t, 0, result.Summary.Update)
	require.Equal(t, 0, result.Summary.Replace)
	require.Equal(t, 0, result.Summary.Delete)

	// base declares three provider resources: network.app, container.api
	// and container.web
	require.Equal(t, 3, result.Summary.Unchanged)
	require.Equal(t, 0, result.Changed())
}

func TestDiffLeavesStateFileUntouched(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	before, err := os.ReadFile(f.statePath)
	require.NoError(t, err)

	useDiffConfiguration(t, f, "mixed")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)
	require.NotZero(t, result.Changed(), "the diff must report changes for this test to mean anything")

	after, err := os.ReadFile(f.statePath)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func TestDiffLeavesEntitiesUntouched(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	before := f.config.Entities()
	beforeCopy := append([]any{}, before...)
	beforeCount := f.config.EntityCount()

	useDiffConfiguration(t, f, "mixed")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)
	require.NotZero(t, result.Changed(), "the diff must report changes for this test to mean anything")

	after := f.config.Entities()
	require.Equal(t, beforeCount, f.config.EntityCount())
	require.Len(t, after, len(beforeCopy))

	// the very same entities, in the same order, with the same contents
	for i := range beforeCopy {
		require.Same(t, beforeCopy[i], after[i])
	}
	require.Equal(t, beforeCopy, after)

	_, err = f.config.FindResource("resource.container.web")
	require.NoError(t, err, "web is removed by the diffed configuration but must still be an entity")

	_, err = f.config.FindResource("resource.container.worker")
	require.Error(t, err, "worker is only added by the diffed configuration and must not be an entity")
}

func TestDiffReportsDriftedNetworkAsUpdate(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	f.plugin.SetReadObserved("resource.network.app", "changed outside xcl")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	var app *diff.Resource
	for i := range result.Resources {
		if result.Resources[i].Address == "resource.network.app" {
			app = &result.Resources[i]
		}
	}

	require.NotNil(t, app, "resource.network.app is not listed, got: %v", resourceActions(result))
	require.Equal(t, diff.ActionUpdate, app.Action)
	require.Equal(t, []string{"resource.network.app update"}, resourceActions(result))
	require.Equal(t, 1, result.Summary.Update)
	require.Equal(t, 2, result.Summary.Unchanged)
	require.Equal(t, 0, result.Summary.Create)
	require.Equal(t, 0, result.Summary.Replace)
	require.Equal(t, 0, result.Summary.Delete)
}

func TestDiffReportsEveryKindOfChange(t *testing.T) {
	f := setupDiffConfig(t)
	useDiffConfiguration(t, f, "replace_base")

	// legacy fails to create, so the apply saves it as failed
	f.plugin.SetCreateError("resource.network.legacy", errors.New("boom"))

	err := f.config.Apply(f.configDir)
	require.Error(t, err)

	f.plugin.ClearErrors()
	f.plugin.ResetCalls()
	f.recorder.reset()

	useDiffConfiguration(t, f, "replace_mixed")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Equal(t, 2, result.Summary.Create)
	require.Equal(t, 1, result.Summary.Update)
	require.Equal(t, 1, result.Summary.Replace)
	require.Equal(t, 1, result.Summary.Delete)

	// network.app and container.web are left as they are
	require.Equal(t, 2, result.Summary.Unchanged)
	require.Equal(t, 5, result.Changed())

	require.Equal(t, []string{
		"resource.container.api update",
		"resource.container.old delete",
		"resource.container.worker create",
		"resource.network.backend create",
		"resource.network.legacy replace",
	}, resourceActions(result))
}

func TestDiffListsOnlyTheChangedResource(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "changed_attribute")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Len(t, result.Resources, 1)
	require.Equal(t, "resource.container.api", result.Resources[0].Address)
	require.Equal(t, diff.ActionUpdate, result.Resources[0].Action)

	require.Equal(t, 1, result.Summary.Update)
	require.Equal(t, 2, result.Summary.Unchanged)
}

func TestDiffResultEnumeratesAddressAndAction(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "mixed")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	// what an application would do with the result: walk it
	addresses := []string{}
	actions := []diff.Action{}
	for _, r := range result.Resources {
		addresses = append(addresses, r.Address)
		actions = append(actions, r.Action)
	}

	require.Equal(t, []string{
		"resource.container.api",
		"resource.container.web",
		"resource.container.worker",
		"resource.network.backend",
	}, addresses)

	require.Equal(t, []diff.Action{
		diff.ActionUpdate,
		diff.ActionDelete,
		diff.ActionCreate,
		diff.ActionCreate,
	}, actions)

	require.Equal(t, 1, result.Summary.Unchanged)
}

func TestDiffRunsAsDiffOperation(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "changed_attribute")

	_, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	recorded := f.recorder.Events()
	require.NotEmpty(t, recorded)

	first := recorded[0]
	require.Equal(t, events.OperationDiff, first.Operation)
	require.Equal(t, events.PhaseStart, first.Phase)

	last := recorded[len(recorded)-1]
	require.Equal(t, events.OperationDiff, last.Operation)
	require.Equal(t, events.PhaseSuccess, last.Phase)
}

func TestDiffEmitsReadAndChangedEvents(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "changed_attribute")

	_, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.NotEmpty(t, f.recorder.find("resource.container.api", events.OperationRead, events.PhaseSuccess))
	require.NotEmpty(t, f.recorder.find("resource.container.api", events.OperationChanged, events.PhaseSuccess))
	require.NotEmpty(t, f.recorder.find("resource.network.app", events.OperationRead, events.PhaseSuccess))
}

func TestDiffEmitsNoCreateUpdateOrDestroyEvents(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "mixed")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)
	require.NotZero(t, result.Changed(), "the diff must report changes for this test to mean anything")

	for _, e := range f.recorder.Events() {
		require.NotEqual(t, events.OperationCreate, e.Operation, "unexpected create event: %+v", e)
		require.NotEqual(t, events.OperationUpdate, e.Operation, "unexpected update event: %+v", e)
		require.NotEqual(t, events.OperationDestroy, e.Operation, "unexpected destroy event: %+v", e)
	}

	require.Empty(t, f.plugin.GetCreatedResources())
	require.Empty(t, f.plugin.GetUpdatedResources())
	require.Empty(t, f.plugin.GetDestroyedResources())
}

func TestDiffLogsDiffComplete(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "changed_attribute")

	_, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	logs := coreDiffLog(f.recorder, "diff complete")
	require.Len(t, logs, 1)

	complete := logs[0]
	require.Equal(t, events.LevelDebug, complete.Meta[events.KeyLevel])
	require.Equal(t, 0, complete.Meta["create"])
	require.Equal(t, 1, complete.Meta["update"])
	require.Equal(t, 0, complete.Meta["replace"])
	require.Equal(t, 0, complete.Meta["delete"])
	require.Equal(t, 2, complete.Meta["unchanged"])
}

func TestDiffDeliversProviderLogsWrittenDuringRead(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	f.plugin.SetLogOnRead(parser.LogMessage{Level: "info", Message: "reading the real resource"})

	_, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	found := []Event{}
	for _, e := range f.recorder.Events() {
		if e.Phase == events.PhaseLog && e.Meta[events.KeyMessage] == "reading the real resource" {
			found = append(found, e)
		}
	}

	require.NotEmpty(t, found, "no provider log was delivered during the diff")
	for _, e := range found {
		require.Equal(t, events.OperationRead, e.Operation)
		require.NotEqual(t, events.SourceCore, e.Source)
	}
}

func TestDiffFailsWithoutPaths(t *testing.T) {
	f := setupDiffConfig(t)

	result, err := f.config.Diff([]string{})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestDiffFailsForInvalidConfiguration(t *testing.T) {
	f := setupDiffConfig(t)

	result, err := f.config.Diff([]string{"./internal/test_fixtures/config/invalid/no_name.xcl"})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestDiffFailsForEmptyConfiguration(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	result, err := f.config.Diff([]string{"./internal/test_fixtures/config/empty"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEmptyConfiguration), "unexpected error: %v", err)
	require.Nil(t, result)
}

func TestDiffFailsWhenSavedStateCannotBeLoaded(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	err := os.WriteFile(f.statePath, []byte("this is not a state file"), 0644)
	require.NoError(t, err)

	result, err := f.config.Diff([]string{f.configDir})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to load previous state")
	require.Nil(t, result)

	require.Empty(t, f.plugin.GetCalls())
}

// diffResourceFor returns the resource the diff lists for the address,
// failing the test when the address is not listed
func diffResourceFor(t *testing.T, result *diff.Diff, address string) diff.Resource {
	t.Helper()

	for _, r := range result.Resources {
		if r.Address == address {
			return r
		}
	}

	require.FailNow(t, "resource not listed", "expected %s in the diff, got: %v", address, resourceActions(result))
	return diff.Resource{}
}

// changePaths returns the path string of every change, in the order they are
// listed
func changePaths(changes []diff.Change) []string {
	paths := []string{}
	for _, c := range changes {
		paths = append(paths, c.Path.String())
	}

	return paths
}

func TestDiffListsConfiguredValuesOfAnAddedResource(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "added")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Equal(t, []string{"resource.container.worker create"}, resourceActions(result))

	worker := diffResourceFor(t, result, "resource.container.worker")
	require.Equal(t, []string{"default", "network"}, changePaths(worker.Changes))

	// default is not set in the configuration, it holds the field's default
	defaultValue := worker.Changes[0]
	require.Nil(t, defaultValue.Before)
	require.Equal(t, "hello world", defaultValue.After)
	require.False(t, defaultValue.Sensitive)
	require.False(t, defaultValue.Unknown)

	// the network block is listed whole with every field that is not
	// computed, its name resolved from the applied app network
	network := worker.Changes[1]
	require.Nil(t, network.Before)
	require.Equal(t, []any{
		map[string]any{
			"id":         0,
			"name":       "app",
			"ip_address": "",
			"aliases":    nil,
		},
	}, network.After)
	require.False(t, network.Sensitive)
	require.False(t, network.Unknown)
}

func TestDiffListsExactlyTheOneChangedAttribute(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "changed_attribute")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Equal(t, []string{"resource.container.api update"}, resourceActions(result))

	api := diffResourceFor(t, result, "resource.container.api")
	require.Len(t, api.Changes, 1)

	port := api.Changes[0]
	require.Equal(t, `env["PORT"]`, port.Path.String())
	require.Equal(t, "8080", port.Before)
	require.Equal(t, "9090", port.After)
	require.False(t, port.Sensitive)
	require.False(t, port.Unknown)
}

func TestDiffReportsDriftOnlyAsUpdateWithNoChanges(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	// the real network changed outside xcl, its configuration did not
	f.plugin.SetReadObserved("resource.network.app", "changed outside xcl")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Equal(t, []string{"resource.network.app update"}, resourceActions(result))

	app := diffResourceFor(t, result, "resource.network.app")
	require.Empty(t, app.Changes)
}

func TestDiffResultEnumeratesEveryChangeFromGoValues(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "base")

	useDiffConfiguration(t, f, "mixed")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	// what an application would do with the result: walk it, reading only
	// Go values, and write one line per resource and per change
	lines := []string{}
	for _, r := range result.Resources {
		lines = append(lines, r.Address+" "+string(r.Action))

		for _, c := range r.Changes {
			lines = append(lines, "  "+c.Path.String()+" before="+jsonText(t, c.Before)+" after="+jsonText(t, c.After))
		}
	}

	require.Equal(t, []string{
		"resource.container.api update",
		`  env["PORT"] before="8080" after="9090"`,
		"resource.container.web delete",
		"resource.container.worker create",
		`  default before=null after="hello world"`,
		`  network before=null after=[{"aliases":null,"id":0,"ip_address":"","name":"backend"}]`,
		"resource.network.backend create",
		`  subnet before=null after="10.1.0.0/16"`,
	}, lines)
}

// jsonText returns the JSON form of a change value, so a test can compare
// nested values as one readable string
func jsonText(t *testing.T, value any) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	return string(encoded)
}

// diffCredentialBeforePassword and diffCredentialAfterPassword are the
// db_password values of the credential/before and credential/after
// fixtures. Neither may appear in a diff that was not asked to reveal them.
const (
	diffCredentialBeforePassword = "diff-before-pw-3a9c"
	diffCredentialAfterPassword  = "diff-after-pw-8e1f"
)

// diffChangedCredential applies credential/before, switches to
// credential/after, which changes the sensitive password, and returns the
// diff run with the given options
func diffChangedCredential(t *testing.T, options ...diff.Option) *diff.Diff {
	t.Helper()

	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "credential/before")

	useDiffConfiguration(t, f, "credential/after")

	result, err := f.config.Diff([]string{f.configDir}, options...)
	require.NoError(t, err)

	return result
}

func TestDiffHidesBothValuesOfAChangedSensitiveValue(t *testing.T) {
	result := diffChangedCredential(t)

	require.Equal(t, []string{"resource.credential.db update"}, resourceActions(result))

	credential := diffResourceFor(t, result, "resource.credential.db")
	require.Len(t, credential.Changes, 1)

	password := credential.Changes[0]
	require.Equal(t, "password", password.Path.String())
	require.True(t, password.Sensitive)
	require.Nil(t, password.Before)
	require.Nil(t, password.After)
}

func TestDiffMarshalledToJSONHoldsNeitherPassword(t *testing.T) {
	result := diffChangedCredential(t)
	require.Equal(t, 1, result.Summary.Update, "the diff must report the password change for this test to mean anything")

	encoded, err := json.Marshal(result)
	require.NoError(t, err)

	require.NotContains(t, string(encoded), diffCredentialBeforePassword)
	require.NotContains(t, string(encoded), diffCredentialAfterPassword)
	require.Contains(t, string(encoded), `"sensitive":true`)
}

func TestDiffRevealsBothValuesOfAChangedSensitiveValueWhenAsked(t *testing.T) {
	result := diffChangedCredential(t, diff.RevealSensitive())

	require.Equal(t, []string{"resource.credential.db update"}, resourceActions(result))

	credential := diffResourceFor(t, result, "resource.credential.db")
	require.Len(t, credential.Changes, 1)

	password := credential.Changes[0]
	require.Equal(t, "password", password.Path.String())
	require.True(t, password.Sensitive, "a revealed value is still marked sensitive")
	require.Equal(t, diffCredentialBeforePassword, password.Before)
	require.Equal(t, diffCredentialAfterPassword, password.After)
}

func TestDiffMarksUnknownValuesAndDoesNotReadTheirResource(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "unknown_ref/before")

	// user's network block is now named after network two's provider id,
	// which only exists once network two is created
	useDiffConfiguration(t, f, "unknown_ref/after")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Equal(t, []string{
		"resource.container.user update",
		"resource.network.two create",
	}, resourceActions(result))

	user := diffResourceFor(t, result, "resource.container.user")
	require.Equal(t, []diff.Change{
		{
			Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before:  "fixed",
			Unknown: true,
		},
	}, user.Changes)

	require.NotContains(t, f.plugin.GetReadResources(), "resource.container.user")
	require.Equal(t, []string{"resource.network.one"}, f.plugin.GetReadResources())
}

func TestDiffMarshalledToJSONMarksUnknownValueWithoutAnAfter(t *testing.T) {
	f := setupDiffConfig(t)
	applyDiffConfiguration(t, f, "unknown_ref/before")
	useDiffConfiguration(t, f, "unknown_ref/after")

	result, err := f.config.Diff([]string{f.configDir})
	require.NoError(t, err)

	user := diffResourceFor(t, result, "resource.container.user")
	encoded, err := json.Marshal(user.Changes)
	require.NoError(t, err)

	require.JSONEq(t, `[{"path":"network[0].name","before":"fixed","unknown":true}]`, string(encoded))
}
