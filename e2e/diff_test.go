package e2e_test

// The diff tests prove that Config.Diff predicts Config.Apply. Each scenario
// works on a copy of a testdata configuration in a temporary directory, so it
// can edit the files, and keeps its state in a temporary directory of its own.
// A scenario runs Diff, checks that the diff left the state file byte for byte
// as it was and reported no create, update or destroy step, then runs Apply
// and checks the diff against what the apply's lifecycle events show it did.
//
// The diff never under-reports and may over-report. A resource the diff
// reports as an update has every computed value unknown, so each resource
// referencing one is reported as an update with that value unknown, though
// the apply may find the value unchanged and leave it alone. The contract
// between the two is therefore:
//
//   - an apply never changes a resource the diff did not list: every resource
//     the apply creates, updates, replaces or deletes is in the diff with that
//     same action
//   - every resource the diff lists that the apply leaves alone was listed with
//     at least one change that is unknown
//
// A failed resource is produced through the plugin configuration itself: a
// postgres location holding a space makes the in-process provider's connect
// fail, which leaves the resource saved as failed. Only the scenario's copy of
// the configuration is edited, the fixtures stay as they are for every other
// test.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// diffPluginDatabasePassword is the default of the plugin configuration's
// db_password variable, the password of postgres.main and postgres.replica
const diffPluginDatabasePassword = "pg-s3cret-example"

// diffPluginAnalyticsPassword is the password of the analytics module's
// database, written directly in the module
const diffPluginAnalyticsPassword = "pg-an4lytics-example"

// diffPluginRotatedPassword is the database password the leak tests change
// db_password to
const diffPluginRotatedPassword = "pg-r0tated-n3w-s3cret"

// diffFailingLocation is a postgres location the in-process provider can not
// connect to, so the create of a postgres block using it fails
const diffFailingLocation = "bad host"

// diffActionSets is what an apply changed, or what a diff said it would
// change: the sorted addresses of the resources for each action
type diffActionSets struct {
	Create  []string
	Update  []string
	Replace []string
	Delete  []string
}

// diffScenario is one configuration copied to a temporary directory, applied
// and diffed by one Config with its state in a temporary directory, and every
// event the Config reports
type diffScenario struct {
	t         *testing.T
	configDir string
	stateDir  string
	recorder  *testutil.EventRecorder
	config    *xcl.Config
}

// newPluginDiffScenario copies the plugin configuration to a temporary
// directory and returns a scenario for it, with both plugins registered and
// state encrypted with testStateKey
func newPluginDiffScenario(t *testing.T) *diffScenario {
	t.Helper()

	scenario := &diffScenario{
		t:         t,
		configDir: diffCopyConfiguration(t, pluginConfigDir),
		stateDir:  t.TempDir(),
		recorder:  &testutil.EventRecorder{},
	}

	scenario.config = newPluginConfig(t, registry.NewPluginRegistry(), scenario.recorder.Record, scenario.stateDir, testStateKey)

	return scenario
}

// newKubeDiffScenario copies the kube configuration to a temporary directory
// and returns a scenario for it, with the kube block types registered and
// state encrypted with testStateKey
func newKubeDiffScenario(t *testing.T) *diffScenario {
	t.Helper()

	t.Setenv("DB_PASSWORD", testPassword)

	scenario := &diffScenario{
		t:         t,
		configDir: diffCopyConfiguration(t, kubeConfigDir),
		stateDir:  t.TempDir(),
		recorder:  &testutil.EventRecorder{},
	}

	scenario.config = newKubeConfig(t, registry.NewPluginRegistry(), scenario.recorder.Record, scenario.stateDir, testStateKey)

	return scenario
}

// diffCopyConfiguration copies the configuration directory source, with its
// modules, to a new temporary directory and returns that directory
func diffCopyConfiguration(t *testing.T, source string) string {
	t.Helper()

	destination := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.CopyFS(destination, os.DirFS(source)))

	return destination
}

// edit replaces the one occurrence of old in the scenario's copy of the file
// at name, relative to the configuration directory, with replacement
func (s *diffScenario) edit(name, old, replacement string) {
	s.t.Helper()

	path := filepath.Join(s.configDir, name)

	content, err := os.ReadFile(path)
	require.NoError(s.t, err)
	require.Equal(s.t, 1, strings.Count(string(content), old), "expected %q exactly once in %s", old, name)

	edited := strings.Replace(string(content), old, replacement, 1)
	require.NoError(s.t, os.WriteFile(path, []byte(edited), 0644))
}

// stateFile returns the bytes of the scenario's state file
func (s *diffScenario) stateFile() []byte {
	s.t.Helper()

	content, err := os.ReadFile(filepath.Join(s.stateDir, state.StateFileName))
	require.NoError(s.t, err)

	return content
}

// apply applies the scenario's configuration, requires it to succeed and
// returns the events it reported
func (s *diffScenario) apply() []xcl.Event {
	s.t.Helper()

	start := len(s.recorder.Events())
	require.NoError(s.t, s.config.Apply(s.configDir))

	return s.recorder.Events()[start:]
}

// applyFails applies the scenario's configuration and requires it to fail
func (s *diffScenario) applyFails() {
	s.t.Helper()

	require.Error(s.t, s.config.Apply(s.configDir))
}

// diff diffs the scenario's configuration and returns the result with the
// events the diff reported. It requires that the diff left the state file
// byte for byte as it was and reported no create, update or destroy step,
// because a diff never changes anything.
func (s *diffScenario) diff(options ...diff.Option) (*diff.Diff, []xcl.Event) {
	s.t.Helper()

	before := s.stateFile()
	start := len(s.recorder.Events())

	result, err := s.config.Diff([]string{s.configDir}, options...)
	require.NoError(s.t, err)

	reported := s.recorder.Events()[start:]

	require.Equal(s.t, string(before), string(s.stateFile()), "the diff changed the state file")

	for _, e := range reported {
		require.NotEqual(s.t, events.OperationCreate, e.Operation, "the diff reported a create event for %q", e.ResourceID)
		require.NotEqual(s.t, events.OperationUpdate, e.Operation, "the diff reported an update event for %q", e.ResourceID)
		require.NotEqual(s.t, events.OperationDestroy, e.Operation, "the diff reported a destroy event for %q", e.ResourceID)
	}

	return result, reported
}

// diffSets returns the sorted addresses for each action the diff found
func diffSets(found *diff.Diff) diffActionSets {
	sets := diffActionSets{Create: []string{}, Update: []string{}, Replace: []string{}, Delete: []string{}}

	for _, r := range found.Resources {
		switch r.Action {
		case diff.ActionCreate:
			sets.Create = append(sets.Create, r.Address)
		case diff.ActionUpdate:
			sets.Update = append(sets.Update, r.Address)
		case diff.ActionReplace:
			sets.Replace = append(sets.Replace, r.Address)
		case diff.ActionDelete:
			sets.Delete = append(sets.Delete, r.Address)
		}
	}

	sort.Strings(sets.Create)
	sort.Strings(sets.Update)
	sort.Strings(sets.Replace)
	sort.Strings(sets.Delete)

	return sets
}

// applySets returns the sorted addresses for each action the apply that
// reported recorded took, read from its lifecycle events. Only resources
// handed to a provider count: a provider call reports a start event, while
// variables, outputs, modules and registered types report a create success
// without one and are left out. A resource destroyed and then created was
// replaced, one only created was created, one only destroyed was deleted and
// one updated was updated.
func applySets(recorded []xcl.Event) diffActionSets {
	provided := map[string]bool{}
	created := map[string]bool{}
	updated := map[string]bool{}
	destroyed := map[string]bool{}

	for _, e := range recorded {
		if e.ResourceID == "" {
			continue
		}

		if e.Phase == events.PhaseStart {
			provided[e.ResourceID] = true
		}

		if e.Phase != events.PhaseSuccess {
			continue
		}

		switch e.Operation {
		case events.OperationCreate:
			created[e.ResourceID] = true
		case events.OperationUpdate:
			updated[e.ResourceID] = true
		case events.OperationDestroy:
			destroyed[e.ResourceID] = true
		}
	}

	sets := diffActionSets{Create: []string{}, Update: []string{}, Replace: []string{}, Delete: []string{}}

	for id := range provided {
		switch {
		case created[id] && destroyed[id]:
			sets.Replace = append(sets.Replace, id)
		case created[id]:
			sets.Create = append(sets.Create, id)
		case destroyed[id]:
			sets.Delete = append(sets.Delete, id)
		case updated[id]:
			sets.Update = append(sets.Update, id)
		}
	}

	sort.Strings(sets.Create)
	sort.Strings(sets.Update)
	sort.Strings(sets.Replace)
	sort.Strings(sets.Delete)

	return sets
}

// requireDiffPredictsApply requires the contract between a diff and the apply
// that follows it: every resource the apply created, updated, replaced or
// deleted is in the diff with that same action, and every resource the diff
// lists that the apply left alone was listed with at least one unknown change
func requireDiffPredictsApply(t *testing.T, found *diff.Diff, applied diffActionSets) {
	t.Helper()

	predicted := diffSets(found)

	require.Subset(t, predicted.Create, applied.Create, "the apply created a resource the diff did not list as a create")
	require.Subset(t, predicted.Update, applied.Update, "the apply updated a resource the diff did not list as an update")
	require.Subset(t, predicted.Replace, applied.Replace, "the apply replaced a resource the diff did not list as a replace")
	require.Subset(t, predicted.Delete, applied.Delete, "the apply deleted a resource the diff did not list as a delete")

	changed := map[string]bool{}
	for _, addresses := range [][]string{applied.Create, applied.Update, applied.Replace, applied.Delete} {
		for _, address := range addresses {
			changed[address] = true
		}
	}

	for _, r := range found.Resources {
		if changed[r.Address] {
			continue
		}

		require.NotEmpty(t, diffUnknownPaths(r), "the diff lists %s as %s but the apply left it alone and none of its changes is unknown", r.Address, r.Action)
	}
}

// diffResource returns the resource at address in found, failing the test
// when the diff does not list it
func diffResource(t *testing.T, found *diff.Diff, address string) diff.Resource {
	t.Helper()

	for _, r := range found.Resources {
		if r.Address == address {
			return r
		}
	}

	require.Failf(t, "resource not in diff", "the diff does not list %s", address)
	return diff.Resource{}
}

// diffUnknownPaths returns the sorted paths of the changes to r that are
// unknown until an apply has run
func diffUnknownPaths(r diff.Resource) []string {
	paths := []string{}
	for _, change := range r.Changes {
		if change.Unknown {
			paths = append(paths, change.Path.String())
		}
	}

	sort.Strings(paths)
	return paths
}

// diffEventText returns everything an event could show a reader: the event
// formatted with its field names, its data as text and its error
func diffEventText(e xcl.Event) string {
	text := fmt.Sprintf("%+v\n%s", e, string(e.Data))
	if e.Error != nil {
		text += "\n" + e.Error.Error()
	}

	return text
}

// rotatePassword changes the plugin configuration's db_password default to
// diffPluginRotatedPassword
func (s *diffScenario) rotatePassword() {
	s.t.Helper()

	s.edit("main.xcl", `default = "`+diffPluginDatabasePassword+`"`, `default = "`+diffPluginRotatedPassword+`"`)
}

func TestDiffOfUnappliedPluginConfigurationPredictsApply(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	found, _ := scenario.diff()
	applied := scenario.apply()

	expected := diffActionSets{
		Create: []string{
			"module.analytics.resource.postgres.analytics",
			"resource.app.web",
			"resource.ingress.web",
			"resource.postgres.main",
			"resource.postgres.replica",
			"resource.redis.cache",
		},
		Update:  []string{},
		Replace: []string{},
		Delete:  []string{},
	}

	require.Equal(t, expected, diffSets(found))
	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfUnappliedPluginConfigurationReportsComputedValuesUnknown(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	found, _ := scenario.diff()

	app := diffResource(t, found, "resource.app.web")
	require.Equal(t, []string{"cache_connection_string", "connection_string"}, diffUnknownPaths(app))

	ingress := diffResource(t, found, "resource.ingress.web")
	require.Equal(t, []string{"app_url"}, diffUnknownPaths(ingress))
}

func TestDiffOfUnchangedPluginConfigurationReportsNoChanges(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	found, _ := scenario.diff()
	applied := scenario.apply()

	require.Equal(t, 0, found.Changed())
	require.Empty(t, found.Resources)
	require.Equal(t, 6, found.Summary.Unchanged)
	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfEditedRedisPortPredictsApply(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", "port     = 6379", "port     = 6380")

	found, _ := scenario.diff()
	applied := scenario.apply()

	expected := diffActionSets{
		Create:  []string{},
		Update:  []string{"resource.app.web", "resource.ingress.web", "resource.redis.cache"},
		Replace: []string{},
		Delete:  []string{},
	}
	require.Equal(t, expected, diffSets(found))

	app := diffResource(t, found, "resource.app.web")
	require.Equal(t, []string{"cache_connection_string"}, diffUnknownPaths(app))

	ingress := diffResource(t, found, "resource.ingress.web")
	require.Equal(t, []string{"app_url"}, diffUnknownPaths(ingress))

	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfEditedReplicaLocationPredictsApply(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", `location = "replica.localhost"`, `location = "replica2.localhost"`)

	found, _ := scenario.diff()
	applied := scenario.apply()

	expected := diffActionSets{
		Create:  []string{},
		Update:  []string{"resource.postgres.replica"},
		Replace: []string{},
		Delete:  []string{},
	}

	require.Equal(t, expected, diffSets(found))
	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfRemovedIngressPredictsApply(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.edit("main.xcl", `resource "ingress" "web" {
  hostname = "example.com"
  app_url  = resource.app.web.url
}`, "")

	found, _ := scenario.diff()
	applied := scenario.apply()

	expected := diffActionSets{
		Create:  []string{},
		Update:  []string{},
		Replace: []string{},
		Delete:  []string{"resource.ingress.web"},
	}

	require.Equal(t, expected, diffSets(found))
	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfFailedReplicaPredictsReplaceByApply(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	scenario.edit("main.xcl", `location = "replica.localhost"`, `location = "`+diffFailingLocation+`"`)
	scenario.applyFails()
	scenario.edit("main.xcl", `location = "`+diffFailingLocation+`"`, `location = "replica.localhost"`)

	found, _ := scenario.diff()
	applied := scenario.apply()

	require.Equal(t, []string{"resource.postgres.replica"}, diffSets(found).Replace)
	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfChangedPasswordPredictsApply(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.rotatePassword()

	found, _ := scenario.diff()
	applied := scenario.apply()

	expected := diffActionSets{
		Create:  []string{},
		Update:  []string{"resource.app.web", "resource.ingress.web", "resource.postgres.main", "resource.postgres.replica"},
		Replace: []string{},
		Delete:  []string{},
	}
	require.Equal(t, expected, diffSets(found))

	app := diffResource(t, found, "resource.app.web")
	require.Equal(t, []string{"connection_string"}, diffUnknownPaths(app))

	ingress := diffResource(t, found, "resource.ingress.web")
	require.Equal(t, []string{"app_url"}, diffUnknownPaths(ingress))

	requireDiffPredictsApply(t, found, applySets(applied))
}

func TestDiffOfChangedPasswordReportsSensitivePasswordChange(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()

	scenario.rotatePassword()

	found, _ := scenario.diff()

	for _, address := range []string{"resource.postgres.main", "resource.postgres.replica"} {
		r := diffResource(t, found, address)
		require.Equal(t, diff.ActionUpdate, r.Action, address)

		expected := []diff.Change{
			{Path: diff.Path{}.Attribute("password"), Sensitive: true},
		}
		require.Equal(t, expected, r.Changes, address)
	}
}

func TestDiffOfKubeConfigurationReportsNothing(t *testing.T) {
	scenario := newKubeDiffScenario(t)

	found, _ := scenario.diff()

	require.Empty(t, found.Resources)
	require.Equal(t, diff.Summary{}, found.Summary)
}

func TestApplyOfKubeConfigurationHandsNothingToAProvider(t *testing.T) {
	scenario := newKubeDiffScenario(t)

	applied := scenario.apply()

	expected := diffActionSets{Create: []string{}, Update: []string{}, Replace: []string{}, Delete: []string{}}
	require.Equal(t, expected, applySets(applied))

	for _, e := range testutil.EventsWithPhase(applied, events.PhaseStart) {
		require.Empty(t, e.ResourceID, "a provider call was started for %s", e.ResourceID)
	}
}

func TestDiffOfUnappliedPluginConfigurationMarshalsNoPassword(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	found, _ := scenario.diff()

	marshalled, err := json.Marshal(found)
	require.NoError(t, err)

	require.NotContains(t, string(marshalled), diffPluginDatabasePassword)
	require.NotContains(t, string(marshalled), diffPluginAnalyticsPassword)
}

func TestDiffOfChangedPasswordMarshalsNoPassword(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()
	scenario.rotatePassword()

	found, _ := scenario.diff()

	marshalled, err := json.Marshal(found)
	require.NoError(t, err)

	require.NotContains(t, string(marshalled), diffPluginDatabasePassword)
	require.NotContains(t, string(marshalled), diffPluginRotatedPassword)
	require.NotContains(t, string(marshalled), diffPluginAnalyticsPassword)
}

func TestDiffOfUnappliedPluginConfigurationFormatsNoPasswordWithV(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	found, _ := scenario.diff()

	formatted := fmt.Sprintf("%v %v", found, *found)

	require.NotContains(t, formatted, diffPluginDatabasePassword)
	require.NotContains(t, formatted, diffPluginAnalyticsPassword)
}

func TestDiffOfChangedPasswordFormatsNoPasswordWithV(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()
	scenario.rotatePassword()

	found, _ := scenario.diff()

	formatted := fmt.Sprintf("%v %v", found, *found)

	require.NotContains(t, formatted, diffPluginDatabasePassword)
	require.NotContains(t, formatted, diffPluginRotatedPassword)
	require.NotContains(t, formatted, diffPluginAnalyticsPassword)
}

func TestDiffOfUnappliedPluginConfigurationFormatsNoPasswordWithPlusV(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	found, _ := scenario.diff()

	formatted := fmt.Sprintf("%+v %+v", found, *found)

	require.NotContains(t, formatted, diffPluginDatabasePassword)
	require.NotContains(t, formatted, diffPluginAnalyticsPassword)
}

func TestDiffOfChangedPasswordFormatsNoPasswordWithPlusV(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()
	scenario.rotatePassword()

	found, _ := scenario.diff()

	formatted := fmt.Sprintf("%+v %+v", found, *found)

	require.NotContains(t, formatted, diffPluginDatabasePassword)
	require.NotContains(t, formatted, diffPluginRotatedPassword)
	require.NotContains(t, formatted, diffPluginAnalyticsPassword)
}

func TestDiffOfUnappliedPluginConfigurationReportsNoPasswordInEvents(t *testing.T) {
	scenario := newPluginDiffScenario(t)

	_, reported := scenario.diff()

	require.NotEmpty(t, reported)
	for _, e := range reported {
		text := diffEventText(e)

		require.NotContains(t, text, diffPluginDatabasePassword)
		require.NotContains(t, text, diffPluginAnalyticsPassword)
	}
}

func TestDiffOfChangedPasswordReportsNoPasswordInEvents(t *testing.T) {
	scenario := newPluginDiffScenario(t)
	scenario.apply()
	scenario.rotatePassword()

	_, reported := scenario.diff()

	require.NotEmpty(t, testutil.EventsWithPhase(reported, events.PhaseSuccess))
	for _, e := range reported {
		text := diffEventText(e)

		require.NotContains(t, text, diffPluginDatabasePassword)
		require.NotContains(t, text, diffPluginRotatedPassword)
		require.NotContains(t, text, diffPluginAnalyticsPassword)
	}
}
