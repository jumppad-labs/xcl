package parser

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
)

const (
	twoDependenciesReplacedID = "resource.network.replaced"
	twoDependenciesUpdatedID  = "resource.network.updated"
	twoDependenciesSameID     = "resource.network.same"
	twoDependenciesUserID     = "resource.container.user"

	computedRefNetworkID = "resource.network.one"
)

// applyTwoDependencies applies and saves the two_dependencies configuration
// with the TestPlugin, then clears the plugin's calls
func applyTwoDependencies(t *testing.T) *lifecycleHarness {
	t.Helper()

	h := setupLifecycle(t)
	h.applyAndSave(t, twoDependenciesConfig)
	h.plugin.ResetCalls()

	return h
}

// applyComputedRef applies and saves the computed_ref configuration, whose
// container names its network block after the network's provider id, then
// clears the plugin's calls
func applyComputedRef(t *testing.T) *lifecycleHarness {
	t.Helper()

	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleComputedRefConfig)
	h.plugin.ResetCalls()

	return h
}

// runDecide runs the decide pass over the configuration at path against the
// state the harness store last saved, requiring it to succeed, and returns its
// decision record
func (h *lifecycleHarness) runDecide(t *testing.T, path string) *diffRecorder {
	t.Helper()

	p := h.newParser(t, nil)

	current, previous, err := p.parseAndValidate(path)
	require.NoError(t, err)

	record, err := p.decide(context.Background(), events.OperationDiff, diff.Options{}, current, previous)
	require.NoError(t, err)
	require.NotNil(t, record)

	return record
}

// diffExpectingFailure diffs the configuration at path against the state the
// harness store last saved, requiring it to fail with no result, and returns
// the error
func (h *lifecycleHarness) diffExpectingFailure(t *testing.T, path string) error {
	t.Helper()

	p := h.newParser(t, nil)

	result, err := p.Diff(context.Background(), diff.Options{}, path)
	require.Error(t, err)
	require.Nil(t, result)

	return err
}

// readCallFor returns the only Read the plugin received for the resource,
// failing the test when it received none or more than one
func readCallFor(t *testing.T, plugin *TestPlugin, id string) ReadCall {
	t.Helper()

	found := []ReadCall{}
	for _, call := range plugin.GetReadCalls() {
		if call.ID == id {
			found = append(found, call)
		}
	}

	require.Len(t, found, 1, "expected exactly one read of %s", id)
	return found[0]
}

// containerNetworkNames decodes a serialized container and returns the name of
// each of its network blocks, in order
func containerNetworkNames(t *testing.T, data []byte) []string {
	t.Helper()

	container := structs.Container{}
	err := json.Unmarshal(data, &container)
	require.NoError(t, err, "unable to decode container: %s", string(data))

	names := []string{}
	for _, network := range container.Networks {
		names = append(names, network.Name)
	}

	return names
}

func TestDecideTellsResourceAboutReplacedAndUpdatedDependencies(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUpdatedID, entity.Update)

	h.runDiff(t, twoDependenciesConfig)

	require.Equal(t, []entity.DependencyChange{
		{Address: twoDependenciesReplacedID, Change: entity.Replace},
		{Address: twoDependenciesUpdatedID, Change: entity.Update},
	}, h.plugin.GetChangedDependencies(twoDependenciesUserID))
}

func TestDecideDoesNotTellResourceAboutUnchangedDependency(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)

	h.runDiff(t, twoDependenciesConfig)

	// updated and same are both left unchanged, so only replaced is told
	require.Equal(t, []entity.DependencyChange{
		{Address: twoDependenciesReplacedID, Change: entity.Replace},
	}, h.plugin.GetChangedDependencies(twoDependenciesUserID))
}

func TestDecideTellsResourceNothingWhenNoDependencyChanges(t *testing.T) {
	h := applyTwoDependencies(t)

	h.runDiff(t, twoDependenciesConfig)

	require.Contains(t, h.plugin.GetChangedCalls(), twoDependenciesUserID+"->"+twoDependenciesUserID)
	require.Nil(t, h.plugin.GetChangedDependencies(twoDependenciesUserID))
}

func TestDiffReportsProviderReplaceAsReplace(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	result := h.runDiff(t, twoDependenciesConfig)

	require.Equal(t, []string{twoDependenciesUserID}, diffAddresses(result))
	require.Equal(t, diff.ActionReplace, diffActionFor(t, result, twoDependenciesUserID))
	require.Equal(t, diff.Summary{Replace: 1, Unchanged: 3}, result.Summary)
}

func TestDiffReportsProviderUpdateAsUpdate(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Update)

	result := h.runDiff(t, twoDependenciesConfig)

	require.Equal(t, []string{twoDependenciesUserID}, diffAddresses(result))
	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, twoDependenciesUserID))
	require.Equal(t, diff.Summary{Update: 1, Unchanged: 3}, result.Summary)
}

func TestDecideRecordsProviderAsReasonWhenNoDependencyIsReplaced(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesUpdatedID, entity.Update)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	record := h.runDecide(t, twoDependenciesConfig)

	user, ok := record.lookup(twoDependenciesUserID)
	require.True(t, ok)
	require.Equal(t, diff.ActionReplace, user.action)
	require.Equal(t, diff.ReplaceProvider, user.reason)
	require.Empty(t, user.replacedDeps)
}

func TestDecideRecordsReplacedDependenciesAsReason(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUpdatedID, entity.Update)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	record := h.runDecide(t, twoDependenciesConfig)

	// only the replaced dependency is a reason, the updated one is not
	user, ok := record.lookup(twoDependenciesUserID)
	require.True(t, ok)
	require.Equal(t, diff.ActionReplace, user.action)
	require.Equal(t, diff.ReplaceDependency, user.reason)
	require.Equal(t, []string{twoDependenciesReplacedID}, user.replacedDeps)
}

func TestDiffKeepsDependentUnchangedWhenItsPluginSaysSo(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.NoChange)

	result := h.runDiff(t, twoDependenciesConfig)

	// user references the network's meta.name, a configured value that stays
	// known while the network is pending, so nothing floors user to an update
	require.Equal(t, []string{twoDependenciesReplacedID}, diffAddresses(result))
	require.Equal(t, diff.Summary{Replace: 1, Unchanged: 3}, result.Summary)
}

func TestDiffFloorsResourceWithUnknownInputsAtUpdate(t *testing.T) {
	h := applyComputedRef(t)
	h.plugin.SetChangedResult(computedRefNetworkID, entity.Replace)
	h.plugin.SetChangedResult(computedRefUserID, entity.NoChange)

	result := h.runDiff(t, lifecycleComputedRefConfig)

	// user names its network after the replaced network's provider id, only
	// known once the apply has run, so its unchanged answer is not trusted
	require.Equal(t, diff.ActionReplace, diffActionFor(t, result, computedRefNetworkID))
	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, computedRefUserID))
	require.Equal(t, diff.Summary{Replace: 1, Update: 1}, result.Summary)
}

func TestDiffKeepsProviderReplaceOfResourceWithUnknownInputs(t *testing.T) {
	h := applyComputedRef(t)
	h.plugin.SetChangedResult(computedRefNetworkID, entity.Replace)
	h.plugin.SetChangedResult(computedRefUserID, entity.Replace)

	result := h.runDiff(t, lifecycleComputedRefConfig)

	require.Equal(t, diff.ActionReplace, diffActionFor(t, result, computedRefUserID))
	require.Equal(t, diff.Summary{Replace: 2}, result.Summary)
}

func TestDiffReadsResourceWithUnknownInputsUsingSavedValues(t *testing.T) {
	h := applyComputedRef(t)
	h.plugin.SetChangedResult(computedRefNetworkID, entity.Replace)

	h.runDiff(t, lifecycleComputedRefConfig)

	// decide pass reads every saved resource; unknowns carry saved values
	read := readCallFor(t, h.plugin, computedRefUserID)
	require.Equal(t, []string{"id-one"}, containerNetworkNames(t, read.New))
}

func TestDiffFailsWhenChangedFails(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)

	injected := stderrors.New("change check unavailable")
	h.plugin.SetChangedError(twoDependenciesUserID, injected)

	err := h.diffExpectingFailure(t, twoDependenciesConfig)

	require.Contains(t, err.Error(), twoDependenciesUserID)
	require.True(t, stderrors.Is(err, injected), "error does not wrap the injected error: %v", err)
}

func TestDiffMakesNoCreateUpdateOrDestroyCallsWhenChangedFails(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedError(twoDependenciesUserID, stderrors.New("change check unavailable"))

	h.diffExpectingFailure(t, twoDependenciesConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())
}

func TestDiffFailsWhenReadFails(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)

	injected := stderrors.New("read unavailable")
	h.plugin.SetReadError(twoDependenciesUserID, injected)

	err := h.diffExpectingFailure(t, twoDependenciesConfig)

	require.Contains(t, err.Error(), twoDependenciesUserID)
	require.True(t, stderrors.Is(err, injected), "error does not wrap the injected error: %v", err)
}

func TestDiffMakesNoCreateUpdateOrDestroyCallsWhenReadFails(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetReadError(twoDependenciesUserID, stderrors.New("read unavailable"))

	h.diffExpectingFailure(t, twoDependenciesConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())
}

func TestDiffNamesReplacedDependencyAsReason(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	result := h.runDiff(t, twoDependenciesConfig)

	user := diffResourceAt(t, result, twoDependenciesUserID)
	require.Equal(t, diff.ActionReplace, user.Action)
	require.Equal(t, diff.ReplaceDependency, user.Reason)
	require.Equal(t, []string{twoDependenciesReplacedID}, user.ReplacedDeps)
}

func TestDiffNamesProviderAsReasonOfReplacedDependency(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	result := h.runDiff(t, twoDependenciesConfig)

	// the replaced network depends on nothing, so its own plugin decided it
	network := diffResourceAt(t, result, twoDependenciesReplacedID)
	require.Equal(t, diff.ActionReplace, network.Action)
	require.Equal(t, diff.ReplaceProvider, network.Reason)
	require.Empty(t, network.ReplacedDeps)
}

func TestDiffNamesProviderAsReasonWhenNoDependencyIsReplaced(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesUpdatedID, entity.Update)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	result := h.runDiff(t, twoDependenciesConfig)

	// an updated dependency is never a reason for a replacement
	user := diffResourceAt(t, result, twoDependenciesUserID)
	require.Equal(t, diff.ActionReplace, user.Action)
	require.Equal(t, diff.ReplaceProvider, user.Reason)
	require.Empty(t, user.ReplacedDeps)
}

func TestDiffMarksFailedResourceReplaceAsFailed(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	result := h.runDiff(t, lifecycleOriginalConfig)

	network := diffResourceAt(t, result, lifecycleNetworkID)
	require.Equal(t, diff.ActionReplace, network.Action)
	require.Equal(t, diff.ReplaceFailed, network.Reason)
	require.Empty(t, network.ReplacedDeps)
}

func TestDiffGivesUpdatedResourceNoReplaceReason(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Update)

	result := h.runDiff(t, twoDependenciesConfig)

	user := diffResourceAt(t, result, twoDependenciesUserID)
	require.Equal(t, diff.ActionUpdate, user.Action)
	require.Empty(t, user.Reason)
	require.Empty(t, user.ReplacedDeps)
}

func TestDiffRendersReplacedDependencyAsReason(t *testing.T) {
	h := applyTwoDependencies(t)
	h.plugin.SetChangedResult(twoDependenciesReplacedID, entity.Replace)
	h.plugin.SetChangedResult(twoDependenciesUserID, entity.Replace)

	result := h.runDiff(t, twoDependenciesConfig)

	output := string(diff.Render(result))
	require.Contains(t, output, "  # "+twoDependenciesUserID+" will be replaced because "+twoDependenciesReplacedID+" is replaced\n-/+ resource \"container\" \"user\" {")
}
