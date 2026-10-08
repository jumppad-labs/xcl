package parser

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// changingCalls returns the create, update and destroy calls in calls, in
// order
func changingCalls(calls []string) []string {
	changing := []string{}
	for _, call := range calls {
		if strings.HasPrefix(call, "create ") || strings.HasPrefix(call, "update ") || strings.HasPrefix(call, "destroy ") {
			changing = append(changing, call)
		}
	}

	return changing
}

// replaceCallsFor returns the destroy and create calls in calls made for any
// of the resources with the given IDs, in order
func replaceCallsFor(calls []string, resourceIDs ...string) []string {
	matching := []string{}
	for _, call := range calls {
		for _, id := range resourceIDs {
			if call == "destroy "+id || call == "create "+id {
				matching = append(matching, call)
			}
		}
	}

	return matching
}

func TestApplyReplacesResourceWhenProviderAnswersReplace(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetChangedResult(lifecycleNetworkID, entity.Replace)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{
		"destroy " + lifecycleNetworkID,
		"create " + lifecycleNetworkID,
	}, changingCalls(callsFor(h.plugin.GetCalls(), lifecycleNetworkID)))
	require.Equal(t, types.StatusCreated, networkStatus(t, st.GetResources()))
}

func TestApplyUpdatesResourceInPlaceWhenProviderAnswersUpdate(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()
	h.plugin.SetChangedResult(lifecycleNetworkID, entity.Update)

	st := h.applyAndSave(t, lifecycleOriginalConfig)

	require.Equal(t, []string{
		"update " + lifecycleNetworkID,
	}, changingCalls(callsFor(h.plugin.GetCalls(), lifecycleNetworkID)))
	require.Empty(t, h.plugin.GetDestroyedResources())
	require.Equal(t, types.StatusUpdated, networkStatus(t, st.GetResources()))
}

func TestApplyDestroysDependentsBeforeDependenciesWhenReplacing(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	// container second is attached to network first, both are replaced
	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetChangedResult(dependentSecondID, entity.Replace)

	h.applyAndSave(t, lifecycleDependentConfig)

	// the order is asserted only because second references first
	require.Equal(t, []string{
		"destroy " + dependentSecondID,
		"destroy " + dependentFirstID,
		"create " + dependentFirstID,
		"create " + dependentSecondID,
	}, replaceCallsFor(h.plugin.GetCalls(), dependentFirstID, dependentSecondID))
}

func TestApplyDecidesEverythingBeforeActing(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetChangedResult(dependentIndependentID, entity.Update)

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Apply(context.Background(), lifecycleDependentConfig)
	require.NoError(t, err)

	lastDecision := -1
	firstAction := -1
	for i, event := range collector.all() {
		switch event.Operation {
		case events.OperationRead, events.OperationChanged:
			lastDecision = i
		case events.OperationDestroy, events.OperationCreate, events.OperationUpdate:
			if firstAction == -1 {
				firstAction = i
			}
		}
	}

	require.NotEqual(t, -1, lastDecision, "no read or changed event was emitted")
	require.NotEqual(t, -1, firstAction, "no destroy, create or update event was emitted")
	require.Less(t, lastDecision, firstAction)
}

func TestApplyMakesNoProviderChangeWhenADecisionFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetChangedResult(dependentIndependentID, entity.Update)
	h.plugin.SetChangedError(dependentThirdID, fmt.Errorf("container runtime unavailable"))

	p := h.newParser(t, nil)

	st, err := p.Apply(context.Background(), lifecycleDependentConfig)
	require.Error(t, err)
	require.Nil(t, st)

	require.Empty(t, changingCalls(h.plugin.GetCalls()))
}

func TestApplyLeavesSavedStateUnchangedWhenADecisionFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	before, err := os.ReadFile(h.statePath)
	require.NoError(t, err)

	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetChangedError(dependentThirdID, fmt.Errorf("container runtime unavailable"))

	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)

	after, err := os.ReadFile(h.statePath)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func TestApplySavesFailedWhenCreatingAReplacementFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	h.plugin.SetChangedResult(lifecycleNetworkID, entity.Replace)
	h.plugin.SetCreateError(lifecycleNetworkID, fmt.Errorf("network API unavailable"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)
	require.Contains(t, err.Error(), "create failed for "+lifecycleNetworkID)

	require.Equal(t, types.StatusFailed, networkStatus(t, h.loadSaved(t)))
}

func TestDiffAfterFailedReplacementListsReplaceAgain(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleOriginalConfig)

	h.plugin.SetChangedResult(lifecycleNetworkID, entity.Replace)
	h.plugin.SetCreateError(lifecycleNetworkID, fmt.Errorf("network API unavailable"))
	h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)

	// the provider no longer asks for a replace, the saved failed status
	// alone makes the next plan replace the network
	h.plugin.ClearErrors()
	h.plugin.SetChangedResult(lifecycleNetworkID, entity.NoChange)

	result := h.runDiff(t, lifecycleOriginalConfig)

	require.Equal(t, diff.ActionReplace, diffActionFor(t, result, lifecycleNetworkID))
}

func TestApplySavesDestroyFailedWhenDestroyingAReplacementFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleOriginalConfig)

	h.plugin.SetChangedResult(lifecycleNetworkID, entity.Replace)
	h.plugin.SetDestroyError(lifecycleNetworkID, fmt.Errorf("network is still in use"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)
	require.Contains(t, err.Error(), "destroy failed for "+lifecycleNetworkID)

	require.Equal(t, types.StatusDestroyFailed, networkStatus(t, h.loadSaved(t)))
}

func TestApplyStopsWhenDestroyingAReplacementFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	// independent is replaced as well, but no create may follow a failed
	// destroy
	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetChangedResult(dependentIndependentID, entity.Replace)
	h.plugin.SetDestroyError(dependentFirstID, fmt.Errorf("network is still in use"))

	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
}

func TestApplyDoesNotReplaceDependentWhenItsPluginAnswersUnchanged(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	// second references only first's meta.name, so first's replacement leaves
	// none of second's inputs unknown, and the provider is left to answer
	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetChangedResult(dependentSecondID, entity.NoChange)

	h.applyAndSave(t, lifecycleDependentConfig)

	require.Equal(t, []string{
		"destroy " + dependentFirstID,
		"create " + dependentFirstID,
	}, changingCalls(callsFor(h.plugin.GetCalls(), dependentFirstID)))
	require.Empty(t, changingCalls(callsFor(h.plugin.GetCalls(), dependentSecondID)))
}

func TestApplyTellsDependentAboutTheReplacedResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)

	h.applyAndSave(t, lifecycleDependentConfig)

	dependencies := h.plugin.GetChangedDependencies(dependentSecondID)
	require.Len(t, dependencies, 1)
	require.Equal(t, entity.Replace, dependencies[0].Change)
}

func TestApplyReplacesFailedResourceThroughDestroyPhase(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	// apply 2: first is replaced and its create fails, so it is saved failed
	h.plugin.SetChangedResult(dependentFirstID, entity.Replace)
	h.plugin.SetCreateError(dependentFirstID, fmt.Errorf("network API unavailable"))
	h.applyAndSaveExpectingFailure(t, lifecycleDependentConfig)
	require.Equal(t, types.StatusFailed, resourceStatus(t, h.loadSaved(t), dependentFirstID))

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()
	h.plugin.SetChangedResult(dependentFirstID, entity.NoChange)
	h.plugin.SetChangedResult(dependentIndependentID, entity.Update)

	// apply 3: the failed first is destroyed in the destroy phase, before any
	// create or update of any resource, then created
	h.applyAndSave(t, lifecycleDependentConfig)

	changing := changingCalls(h.plugin.GetCalls())
	require.NotEmpty(t, changing)
	require.Equal(t, "destroy "+dependentFirstID, changing[0])
	require.Contains(t, changing, "create "+dependentFirstID)
	require.Contains(t, changing, "update "+dependentIndependentID)
	require.Equal(t, types.StatusCreated, resourceStatus(t, h.loadSaved(t), dependentFirstID))
}
