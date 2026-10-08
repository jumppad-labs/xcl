package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
)

const (
	twoDependenciesConfig = "../test_fixtures/config/lifecycle/two_dependencies"
	dependentConfig       = "../test_fixtures/config/lifecycle/dependent"
	throughModuleConfig   = "../test_fixtures/config/lifecycle/through_module"
)

// parseForDependencies parses the configuration at path without walking it,
// so every entity carries the links it was parsed with, and returns the
// parsed state together with the parser it was parsed by
func parseForDependencies(t *testing.T, path string) (*State, *Parser) {
	t.Helper()

	p, _ := setupParser(t)

	current, _, err := p.parseAndValidate(path)
	require.NoError(t, err)

	return current, p
}

func TestDependencyChangesListsUpdatedAndReplacedDependencies(t *testing.T) {
	current, p := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()
	record.decide("resource.network.replaced", decision{action: diff.ActionReplace, reason: diff.ReplaceProvider})
	record.decide("resource.network.updated", decision{action: diff.ActionUpdate})
	record.decide("resource.network.same", decision{})

	user := requireEntity(t, current, "resource.container.user")

	changes, err := dependencyChanges(user, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Equal(t, []entity.DependencyChange{
		{Address: "resource.network.replaced", Change: entity.Replace},
		{Address: "resource.network.updated", Change: entity.Update},
	}, changes)
}

func TestDependencyChangesOmitsUnchangedDependencies(t *testing.T) {
	current, p := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()
	record.decide("resource.network.replaced", decision{})
	record.decide("resource.network.updated", decision{})
	record.decide("resource.network.same", decision{})

	user := requireEntity(t, current, "resource.container.user")

	changes, err := dependencyChanges(user, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Nil(t, changes)
}

func TestDependencyChangesOmitsCreatedDependencies(t *testing.T) {
	current, p := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()
	record.decide("resource.network.replaced", decision{action: diff.ActionCreate})
	record.decide("resource.network.updated", decision{action: diff.ActionCreate})
	record.decide("resource.network.same", decision{action: diff.ActionCreate})

	user := requireEntity(t, current, "resource.container.user")

	changes, err := dependencyChanges(user, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Nil(t, changes)
}

func TestDependencyChangesOmitsUndecidedDependencies(t *testing.T) {
	current, p := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()

	user := requireEntity(t, current, "resource.container.user")

	changes, err := dependencyChanges(user, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Nil(t, changes)
}

func TestDependencyChangesListsOnlyDirectDependencies(t *testing.T) {
	current, p := parseForDependencies(t, dependentConfig)

	record := newDiffRecorder()
	record.decide("resource.network.first", decision{action: diff.ActionReplace, reason: diff.ReplaceProvider})
	record.decide("resource.container.second", decision{})

	third := requireEntity(t, current, "resource.container.third")

	changes, err := dependencyChanges(third, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Nil(t, changes)
}

func TestDependencyChangesListsADirectDependencyOfAChain(t *testing.T) {
	current, p := parseForDependencies(t, dependentConfig)

	record := newDiffRecorder()
	record.decide("resource.network.first", decision{action: diff.ActionReplace, reason: diff.ReplaceProvider})
	record.decide("resource.container.second", decision{action: diff.ActionReplace, reason: diff.ReplaceDependency, replacedDeps: []string{"resource.network.first"}})

	third := requireEntity(t, current, "resource.container.third")

	changes, err := dependencyChanges(third, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Equal(t, []entity.DependencyChange{
		{Address: "resource.container.second", Change: entity.Replace},
	}, changes)
}

func TestDependencyChangesLooksThroughModuleOutputs(t *testing.T) {
	current, p := parseForDependencies(t, throughModuleConfig)

	record := newDiffRecorder()
	record.decide("module.net.resource.network.inner", decision{action: diff.ActionReplace, reason: diff.ReplaceProvider})
	record.decide("resource.network.root", decision{})

	fromOutput := requireEntity(t, current, "resource.container.from_output")

	changes, err := dependencyChanges(fromOutput, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Equal(t, []entity.DependencyChange{
		{Address: "module.net.resource.network.inner", Change: entity.Replace},
	}, changes)
}

func TestDependencyChangesLooksThroughVariables(t *testing.T) {
	current, p := parseForDependencies(t, throughModuleConfig)

	record := newDiffRecorder()
	record.decide("resource.network.root", decision{action: diff.ActionUpdate})
	record.decide("module.net.resource.network.inner", decision{})

	fromVariable := requireEntity(t, current, "module.app.resource.container.from_variable")

	changes, err := dependencyChanges(fromVariable, current, p.addressParser(), p.typeRegistry, record)
	require.NoError(t, err)

	require.Equal(t, []entity.DependencyChange{
		{Address: "resource.network.root", Change: entity.Update},
	}, changes)
}
