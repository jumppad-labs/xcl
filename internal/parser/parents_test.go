package parser

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	lifecycleModuleReferenceConfig = "../test_fixtures/config/lifecycle/module_reference/main.xcl"

	moduleReferenceModuleID    = "module.networks"
	moduleReferenceOneID       = "module.networks.resource.network.one"
	moduleReferenceTwoID       = "module.networks.resource.network.two"
	moduleReferenceConsumerID  = "resource.network.consumer"
	moduleReferenceAloneID     = "resource.network.alone"
	dependentIndependentVarID  = "variable.independent_subnet"
	dependentFirstNameOutputID = "output.first_name"
	registeredSharedModuleID   = "module.shared"
	registeredModuleOutputID   = "module.shared.output.location"
)

// savedIDs returns the ID of every entity in entities, in state order.
func savedIDs(t *testing.T, entities []any) []string {
	t.Helper()

	ids := []string{}
	for _, resource := range entities {
		meta, err := types.GetMeta(resource)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	return ids
}

// The destroy graph built from a saved state makes every resource a child of
// the resources it depends on, and a resource with no dependencies a child of
// none of them.
func TestSavedStateOrdersEachResourceAfterItsDependencies(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleDependentConfig)

	require.Empty(t, savedGraphParents(t, h.newParser(t, nil), dependentFirstID))
	require.Equal(t, []string{dependentFirstID}, savedGraphParents(t, h.newParser(t, nil), dependentSecondID))
	require.Equal(t, []string{dependentSecondID}, savedGraphParents(t, h.newParser(t, nil), dependentThirdID))
	require.Equal(t, []string{dependentIndependentVarID}, savedGraphParents(t, h.newParser(t, nil), dependentIndependentID))
	require.Equal(t, []string{dependentFirstID}, savedGraphParents(t, h.newParser(t, nil), dependentFirstNameOutputID))
	require.Empty(t, savedGraphParents(t, h.newParser(t, nil), dependentIndependentVarID))
}

// The destroy graph built from a saved state makes a resource inside a module
// a child of that module, alongside anything else it depends on.
func TestSavedStateOrdersModuleBeforeItsResources(t *testing.T) {
	h := setupRegisteredTypes(t)

	h.applyAndSave(t, registeredBasicConfig)

	require.Equal(
		t,
		[]string{registeredSharedModuleID},
		savedGraphParents(t, h.newParser(t, nil), registeredModuleDatabaseID),
	)
	require.Equal(
		t,
		[]string{registeredSharedModuleID, registeredModuleDatabaseID},
		savedGraphParents(t, h.newParser(t, nil), registeredModuleOutputID),
	)
}

// The destroy graph built from a saved state makes a resource whose depends_on
// names a whole module a child of every resource in that module, while a
// resource that depends on nothing is a child of none of them.
func TestSavedStateOrdersEveryModuleResourceBeforeModuleWideDependent(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleModuleReferenceConfig)

	require.Equal(
		t,
		[]string{moduleReferenceOneID, moduleReferenceTwoID},
		savedGraphParents(t, h.newParser(t, nil), moduleReferenceConsumerID),
	)
	require.Equal(t, []string{moduleReferenceModuleID}, savedGraphParents(t, h.newParser(t, nil), moduleReferenceOneID))
	require.Equal(t, []string{moduleReferenceModuleID}, savedGraphParents(t, h.newParser(t, nil), moduleReferenceTwoID))
	require.Empty(t, savedGraphParents(t, h.newParser(t, nil), moduleReferenceAloneID))
}

// Re-applying an unchanged configuration makes no provider destroy call and
// saves the same resources, ordered the same way in the destroy graph, as the
// first apply.
func TestReapplyWithoutChangesDestroysNothingAndKeepsOrder(t *testing.T) {
	h := setupLifecycle(t)

	expectedIDs := []string{
		dependentIndependentVarID,
		dependentFirstID,
		dependentSecondID,
		dependentThirdID,
		dependentIndependentID,
		dependentFirstNameOutputID,
	}

	h.applyAndSave(t, lifecycleDependentConfig)
	first := h.loadSaved(t)

	firstParents := map[string][]string{}
	for _, id := range expectedIDs {
		firstParents[id] = savedGraphParents(t, h.newParser(t, nil), id)
	}

	h.plugin.ResetCalls()

	h.applyAndSave(t, lifecycleDependentConfig)
	second := h.loadSaved(t)

	for _, call := range h.plugin.GetCalls() {
		require.False(t, strings.HasPrefix(call, "destroy "), "unexpected provider call %q", call)
	}
	require.Empty(t, h.plugin.GetDestroyedResources())

	require.ElementsMatch(t, expectedIDs, savedIDs(t, first))
	require.ElementsMatch(t, expectedIDs, savedIDs(t, second))

	require.Equal(t, firstParents[dependentFirstID], savedGraphParents(t, h.newParser(t, nil), dependentFirstID))
	require.Equal(t, firstParents[dependentSecondID], savedGraphParents(t, h.newParser(t, nil), dependentSecondID))
	require.Equal(t, firstParents[dependentThirdID], savedGraphParents(t, h.newParser(t, nil), dependentThirdID))
	require.Equal(t, firstParents[dependentIndependentID], savedGraphParents(t, h.newParser(t, nil), dependentIndependentID))
	require.Equal(t, firstParents[dependentIndependentVarID], savedGraphParents(t, h.newParser(t, nil), dependentIndependentVarID))
	require.Equal(t, firstParents[dependentFirstNameOutputID], savedGraphParents(t, h.newParser(t, nil), dependentFirstNameOutputID))

	require.Equal(t, []string{dependentSecondID}, savedGraphParents(t, h.newParser(t, nil), dependentThirdID))
}

// A saved state records no parents of its own, the destroy order is worked
// out from each entity's links when the state is destroyed.
func TestSavedStateHasNoParentsKey(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleDependentConfig)

	loaded, err := h.store.Load()
	require.NoError(t, err)
	require.NotEmpty(t, loaded)

	for _, record := range loaded {
		encoded, err := json.Marshal(record)
		require.NoError(t, err)

		var decoded struct {
			Meta map[string]json.RawMessage `json:"meta"`
		}
		err = json.Unmarshal(encoded, &decoded)
		require.NoError(t, err)

		require.NotNil(t, decoded.Meta, "record has no meta: %s", encoded)
		require.NotContains(t, decoded.Meta, "parents", "record saved a parents key: %s", encoded)
	}
}
