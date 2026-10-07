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

func TestDiffNeverReadsResourceReferencingComputedValueOfDriftedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Empty(t, callsFor(h.plugin.GetCalls(), diffUpdateRefUserID))
	require.NotContains(t, h.plugin.GetReadResources(), diffUpdateRefUserID)
	require.NotContains(t, h.plugin.GetChangedCalls(), diffUpdateRefUserID+"->"+diffUpdateRefUserID)
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

func TestDiffNeverReadsResourceReceivingPropagatedUnknown(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)

	require.Empty(t, callsFor(h.plugin.GetCalls(), diffUpdateRefChainedID))
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

func TestDiffNeverReadsResourceReferencingComputedValueOfEditedResource(t *testing.T) {
	h := applyUpdateRefBefore(t)

	h.runDiff(t, diffUpdateRefEditedConfig)

	require.Empty(t, callsFor(h.plugin.GetCalls(), diffUpdateRefUserID))
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

func TestApplyAfterDiffOfDriftedResourceUpdatesOnlyTheDriftedResource(t *testing.T) {
	h := applyUpdateRefBeforeWithDrift(t)

	h.runDiff(t, diffUpdateRefBeforeConfig)
	h.plugin.ResetCalls()

	st := h.applyAndSave(t, diffUpdateRefBeforeConfig)

	// apply re-reads user and asks Changed, the diff's unknown does not force
	// an update: the provider id is unchanged by the update
	require.Equal(t, []string{diffUpdateRefAppID}, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())
	require.Equal(t, []string{
		"read " + diffUpdateRefUserID,
		"changed " + diffUpdateRefUserID,
	}, callsFor(h.plugin.GetCalls(), diffUpdateRefUserID))

	user := findResource[structs.Container](t, st.GetResources(), diffUpdateRefUserID)
	require.Len(t, user.Networks, 1)
	require.Equal(t, "id-app", user.Networks[0].Name)
	require.Equal(t, types.StatusCreated, user.Meta.Status)
}
