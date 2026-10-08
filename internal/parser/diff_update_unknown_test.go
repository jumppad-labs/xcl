package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
)

const (
	diffUpdateRefBeforeConfig = "../test_fixtures/config/diff/update_ref/before/main.xcl"
	diffUpdateRefEditedConfig = "../test_fixtures/config/diff/update_ref/edited/main.xcl"
	diffUpdateRefAppID        = "resource.network.app"
	diffUpdateRefUserID       = "resource.container.user"
	diffUpdateRefPlainID      = "resource.container.plain"
	diffUpdateRefNamedID      = "resource.container.named"
	diffUpdateRefChainedID    = "resource.container.chained"
	diffUpdateRefDrift        = "drifted"
)

// applyUpdateRefBefore applies and saves the before configuration, then
// clears the plugin's calls
func applyUpdateRefBefore(t *testing.T) *lifecycleHarness {
	t.Helper()

	h := setupLifecycle(t)
	h.applyAndSave(t, diffUpdateRefBeforeConfig)
	h.plugin.ResetCalls()

	return h
}

// applyUpdateRefBeforeWithDrift applies and saves the before configuration,
// then makes network app's provider report drift on every later read
func applyUpdateRefBeforeWithDrift(t *testing.T) *lifecycleHarness {
	t.Helper()

	h := applyUpdateRefBefore(t)
	h.plugin.SetReadObserved(diffUpdateRefAppID, diffUpdateRefDrift)

	return h
}

func TestDiffReportsDriftedResourceAsUpdate(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, diffUpdateRefAppID))
}

func TestDiffReportsResourceReferencingComputedValueOfDriftedResourceAsUpdate(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	user := diffResourceAt(t, result, diffUpdateRefUserID)
	require.Equal(t, diff.ActionUpdate, user.Action)
	require.Equal(t, []diff.Change{
		{
			Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before:  "id-app",
			After:   nil,
			Unknown: true,
		},
	}, user.Changes)
}

func TestDiffReadsResourceReferencingComputedValueOfDriftedResourceWithSavedValues(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	// decide pass reads every saved resource; unknowns carry saved values
	read := readCallFor(t, h.plugin, diffUpdateRefUserID)
	require.Equal(t, []string{"id-app"}, containerNetworkNames(t, read.New))
	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, diffUpdateRefUserID))
}

func TestDiffDoesNotListResourceReferencingConfiguredValueOfDriftedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	require.NotContains(t, diffAddresses(result), diffUpdateRefPlainID)
}

func TestDiffReadsResourceReferencingConfiguredValueOfDriftedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Equal(t, []string{
		"read " + diffUpdateRefPlainID,
		"changed " + diffUpdateRefPlainID,
	}, callsFor(h.plugin.GetCalls(), diffUpdateRefPlainID))
}

func TestDiffDoesNotListResourceReferencingMetaNameOfDriftedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	require.NotContains(t, diffAddresses(result), diffUpdateRefNamedID)
}

func TestDiffReadsResourceReferencingMetaNameOfDriftedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Equal(t, []string{
		"read " + diffUpdateRefNamedID,
		"changed " + diffUpdateRefNamedID,
	}, callsFor(h.plugin.GetCalls(), diffUpdateRefNamedID))
}

func TestDiffPropagatesUnknownOfUpdatedResourceToItsDependents(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	// user is updated only because it holds an unknown, which makes its own
	// computed assigned address unknown to chained
	chained := diffResourceAt(t, result, diffUpdateRefChainedID)
	require.Equal(t, diff.ActionUpdate, chained.Action)
	require.Equal(t, []diff.Change{
		{
			Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before:  "assigned-id-app",
			After:   nil,
			Unknown: true,
		},
	}, chained.Changes)
}

func TestDiffReadsResourceReceivingPropagatedUnknownWithSavedValues(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	// decide pass reads every saved resource; unknowns carry saved values
	read := readCallFor(t, h.plugin, diffUpdateRefChainedID)
	require.Equal(t, []string{"assigned-id-app"}, containerNetworkNames(t, read.New))
	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, diffUpdateRefChainedID))
}

func TestDiffOfDriftedResourceListsEveryAffectedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Equal(t, sorted(
		diffUpdateRefAppID,
		diffUpdateRefChainedID,
		diffUpdateRefUserID,
	), sortedCopy(diffAddresses(result)))
	require.Equal(t, diff.Summary{Update: 3, Unchanged: 2}, result.Summary)
}

func TestDiffOfDriftedResourceMakesNoCreateUpdateOrDestroyCalls(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())
}

func TestDiffReportsResourceReferencingComputedValueOfEditedResourceAsUpdate(t *testing.T) {
	h := applyUpdateRefBefore(t)

	result := h.runDiff(t, diffUpdateRefEditedConfig)

	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, diffUpdateRefAppID))

	user := diffResourceAt(t, result, diffUpdateRefUserID)
	require.Equal(t, diff.ActionUpdate, user.Action)
	require.Equal(t, []diff.Change{
		{
			Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before:  "id-app",
			After:   nil,
			Unknown: true,
		},
	}, user.Changes)
}

func TestDiffReadsResourceReferencingComputedValueOfEditedResourceWithSavedValues(t *testing.T) {
	h := applyUpdateRefBefore(t)

	result := h.runDiff(t, diffUpdateRefEditedConfig)

	// decide pass reads every saved resource; unknowns carry saved values
	read := readCallFor(t, h.plugin, diffUpdateRefUserID)
	require.Equal(t, []string{"id-app"}, containerNetworkNames(t, read.New))
	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, diffUpdateRefUserID))
}

func TestDiffPassesEditedConfiguredValueAsKnown(t *testing.T) {
	h := applyUpdateRefBefore(t)

	result := h.runDiff(t, diffUpdateRefEditedConfig)

	// subnet is configured, so plain sees its new value rather than an
	// unknown
	plain := diffResourceAt(t, result, diffUpdateRefPlainID)
	require.Equal(t, diff.ActionUpdate, plain.Action)
	require.Equal(t, []diff.Change{
		{
			Path:   diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before: "10.0.0.0/16",
			After:  "10.9.0.0/16",
		},
	}, plain.Changes)
}

func TestDiffOfUnchangedResourceListsNoDependents(t *testing.T) {
	h := applyUpdateRefBefore(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Empty(t, result.Resources)
	require.Equal(t, diff.Summary{Unchanged: 5}, result.Summary)
}

func TestDiffOfUnchangedResourceReadsEveryDependent(t *testing.T) {
	h := applyUpdateRefBefore(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Equal(t, sorted(
		diffUpdateRefAppID,
		diffUpdateRefChainedID,
		diffUpdateRefNamedID,
		diffUpdateRefPlainID,
		diffUpdateRefUserID,
	), sortedCopy(h.plugin.GetReadResources()))
}

// The decide pass floors a dependent whose inputs reference a computed value
// of an updated resource to an update, so the apply now updates exactly what
// the diff listed, where it used to update only the drifted resource
func TestApplyAfterDiffOfDriftedResourceUpdatesWhatTheDiffListed(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)
	h.plugin.ResetCalls()

	listedForUpdate := []string{}
	for _, resource := range result.Resources {
		if resource.Action == diff.ActionUpdate {
			listedForUpdate = append(listedForUpdate, resource.Address)
		}
	}
	require.Equal(t, sorted(diffUpdateRefAppID, diffUpdateRefChainedID, diffUpdateRefUserID), sortedCopy(listedForUpdate))

	st := h.applyAndSave(t, diffUpdateRefBeforeConfig)

	require.Equal(t, sortedCopy(listedForUpdate), sortedCopy(h.plugin.GetUpdatedResources()))
	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())

	// the update of app left its provider id as it was, so user still
	// resolves to it
	user := findResource[structs.Container](t, st.GetResources(), diffUpdateRefUserID)
	require.Len(t, user.Networks, 1)
	require.Equal(t, "id-app", user.Networks[0].Name)
	require.Equal(t, types.StatusUpdated, user.Meta.Status)
}
