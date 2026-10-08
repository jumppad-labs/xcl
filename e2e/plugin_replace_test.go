package e2e_test

// The person replace tests run the example person plugin, from
// plugins/example, both in-process and as the external binary TestMain
// builds, and require that renaming a person, which the person provider
// answers with replace, is planned and applied the same way by both.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
)

// personConfiguration declares two people, the tests rename the first
const personConfiguration = `resource "person" "renamed" {
  first_name = "Ada"
  last_name  = "Lovelace"
  age        = 36
}

resource "person" "kept" {
  first_name = "Grace"
  last_name  = "Hopper"
}
`

// personInProcessPlugin registers the example person provider in this
// process, the same way the example plugin's binary does
type personInProcessPlugin struct {
	plugins.PluginBase
}

// Init registers the person block type with the example provider
func (p *personInProcessPlugin) Init(log logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		log,
		state,
		"resource",
		"person",
		&person.Person{},
		&person.ExampleProvider{},
	)
}

// newPersonScenario writes personConfiguration to a temporary directory and
// returns a scenario for it, with the person plugin registered in local and
// state encrypted with testStateKey
func newPersonScenario(t *testing.T, local *registry.Local) *diffScenario {
	t.Helper()

	configDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "main.xcl"), []byte(personConfiguration), 0644))

	scenario := &diffScenario{
		t:         t,
		configDir: configDir,
		stateDir:  t.TempDir(),
		recorder:  &testutil.EventRecorder{},
	}

	scenario.config = newConfig(t, scenario.recorder.Record, scenario.stateDir, testStateKey, xcl.WithRegistry(local))

	return scenario
}

// newInProcessPersonScenario returns a person scenario with the person plugin
// running in-process
func newInProcessPersonScenario(t *testing.T) *diffScenario {
	t.Helper()

	local := registry.NewLocal()
	local.RegisterPlugin(&personInProcessPlugin{})

	return newPersonScenario(t, local)
}

// newExternalPersonScenario returns a person scenario with the person plugin
// running as the external binary TestMain builds
func newExternalPersonScenario(t *testing.T) *diffScenario {
	t.Helper()

	local := registry.NewLocal()
	local.RegisterExternalPlugin(externalPersonPlugin)

	return newPersonScenario(t, local)
}

// renamePerson applies the scenario's configuration, then renames the first
// person, which the person provider answers with replace
func renamePerson(s *diffScenario) {
	s.t.Helper()

	s.apply()
	s.edit("main.xcl", `first_name = "Ada"`, `first_name = "Augusta"`)
}

// personDiffJSON renames the first person and returns the diff of the
// rename marshalled to JSON
func personDiffJSON(t *testing.T, s *diffScenario) string {
	t.Helper()

	renamePerson(s)

	found, _ := s.diff()

	marshalled, err := json.Marshal(found)
	require.NoError(t, err)

	return string(marshalled)
}

// personApplyOperations renames the first person, applies the rename and
// returns the lifecycle operations the apply reported for each resource
func personApplyOperations(t *testing.T, s *diffScenario) map[string][]string {
	t.Helper()

	renamePerson(s)

	return testutil.ResourceOperations(s.apply())
}

// personApplyValues renames the first person, applies the rename and returns
// every person the scenario's Config then holds, keyed by address, with the
// metadata left out so only the values are compared
func personApplyValues(t *testing.T, s *diffScenario) map[string]person.Person {
	t.Helper()

	renamePerson(s)
	s.apply()

	values := map[string]person.Person{}
	for _, id := range []string{"resource.person.renamed", "resource.person.kept"} {
		found, err := xcl.Find[person.Person](s.config, id)
		require.NoError(t, err)

		found.ResourceBase = types.ResourceBase{}
		values[id] = *found
	}

	return values
}

func TestPersonNameChangePlansReplace(t *testing.T) {
	scenario := newInProcessPersonScenario(t)
	renamePerson(scenario)

	found, _ := scenario.diff()

	expected := diffActionSets{
		Create:  []string{},
		Update:  []string{},
		Replace: []string{"resource.person.renamed"},
		Delete:  []string{},
	}
	require.Equal(t, expected, diffSets(found))
}

func TestPersonNameChangePlansTheSameInProcessAndExternal(t *testing.T) {
	inProcess := personDiffJSON(t, newInProcessPersonScenario(t))
	external := personDiffJSON(t, newExternalPersonScenario(t))

	require.JSONEq(t, inProcess, external)
}

func TestPersonNameChangeAppliesTheSameInProcessAndExternal(t *testing.T) {
	inProcess := personApplyOperations(t, newInProcessPersonScenario(t))
	external := personApplyOperations(t, newExternalPersonScenario(t))

	expected := map[string][]string{
		"resource.person.renamed": {"destroy", "create"},
	}
	require.Equal(t, expected, inProcess)
	require.Equal(t, inProcess, external)
}

func TestPersonNameChangeLeavesTheSameValuesInProcessAndExternal(t *testing.T) {
	inProcess := personApplyValues(t, newInProcessPersonScenario(t))
	external := personApplyValues(t, newExternalPersonScenario(t))

	expected := map[string]person.Person{
		"resource.person.renamed": {
			FirstName: "Augusta",
			LastName:  "Lovelace",
			Age:       36,
			PersonID:  "person-augusta-lovelace",
		},
		"resource.person.kept": {
			FirstName: "Grace",
			LastName:  "Hopper",
			PersonID:  "person-grace-hopper",
		},
	}
	require.Equal(t, expected, inProcess)
	require.Equal(t, inProcess, external)
}

func TestPersonNameChangeDiffPredictsApply(t *testing.T) {
	scenario := newExternalPersonScenario(t)
	renamePerson(scenario)

	found, _ := scenario.diff()
	applied := scenario.apply()

	requireDiffEqualsApply(t, found, applySets(applied))
}
