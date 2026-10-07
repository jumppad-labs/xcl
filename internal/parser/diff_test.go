package parser

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	diffBaseConfig             = "../test_fixtures/config/diff/base/main.xcl"
	diffChangedAttributeConfig = "../test_fixtures/config/diff/changed_attribute/main.xcl"
	diffAddedConfig            = "../test_fixtures/config/diff/added/main.xcl"
	diffRemovedConfig          = "../test_fixtures/config/diff/removed/main.xcl"
	diffMixedConfig            = "../test_fixtures/config/diff/mixed/main.xcl"
	diffWithBuiltinsConfig     = "../test_fixtures/config/diff/with_builtins/main.xcl"

	diffNetworkAppID       = "resource.network.app"
	diffNetworkBackendID   = "resource.network.backend"
	diffContainerAPIID     = "resource.container.api"
	diffContainerWebID     = "resource.container.web"
	diffContainerWorkerID  = "resource.container.worker"
	diffBuiltinVariableID  = "variable.subnet"
	diffBuiltinOutputID    = "output.app_name"
	diffBuiltinModuleID    = "module.shared"
	diffDisabledNetworkID  = "resource.network.off"
	diffRegisteredDBID     = "resource.database.main"
	diffBaseProviderCount  = 3
	diffInjectedReadFailed = "network API unavailable"
)

// runDiff builds a fresh Parser from the harness and diffs the config at path
// against the state the harness store last saved, requiring it to succeed
func (h *lifecycleHarness) runDiff(t *testing.T, path string) *diff.Diff {
	t.Helper()

	p := h.newParser(t, nil)

	result, err := p.Diff(context.Background(), diff.Options{}, path)
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

// registerDatabaseType registers the config-only database type into the
// harness registry, alongside the TestPlugin's provider types
func (h *lifecycleHarness) registerDatabaseType(t *testing.T) {
	t.Helper()

	err := h.registry.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)
	require.NoError(t, err)
}

// diffAddresses returns the address of every resource the diff lists, in the
// order it lists them
func diffAddresses(result *diff.Diff) []string {
	addresses := []string{}
	for _, resource := range result.Resources {
		addresses = append(addresses, resource.Address)
	}

	return addresses
}

// diffActionFor returns the action the diff lists for the address, failing
// the test when the address is not listed
func diffActionFor(t *testing.T, result *diff.Diff, address string) diff.Action {
	t.Helper()

	for _, resource := range result.Resources {
		if resource.Address == address {
			return resource.Action
		}
	}

	require.FailNow(t, fmt.Sprintf("expected %s in the diff, listed: %v", address, diffAddresses(result)))
	return ""
}

// sortedCopy returns the strings sorted, leaving the given slice as it is
func sortedCopy(values []string) []string {
	copied := append([]string{}, values...)
	sort.Strings(copied)

	return copied
}

func TestDiffMakesNoCreateUpdateOrDestroyCalls(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)
	h.plugin.ResetCalls()

	// mixed adds, removes and edits resources
	result := h.runDiff(t, diffMixedConfig)
	require.NotZero(t, result.Changed())

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())
}

func TestDiffReadsExactlyEverySavedConfiguredResourceThatDidNotFail(t *testing.T) {
	h := setupLifecycle(t)

	// web fails to create, network and api are saved as created
	h.plugin.SetCreateError(diffContainerWebID, fmt.Errorf("container API unavailable"))
	h.applyAndSaveExpectingFailure(t, diffBaseConfig)
	require.Equal(t, types.StatusFailed, resourceStatus(t, h.loadSaved(t), diffContainerWebID))

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	// added keeps every saved resource and adds worker, which is not saved
	h.runDiff(t, diffAddedConfig)

	require.Equal(t,
		sorted(diffNetworkAppID, diffContainerAPIID),
		sortedCopy(h.plugin.GetReadResources()),
	)
}

func TestDiffReadsNoResourceRemovedFromTheConfiguration(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)
	h.plugin.ResetCalls()

	h.runDiff(t, diffRemovedConfig)

	require.Equal(t,
		sorted(diffNetworkAppID, diffContainerAPIID),
		sortedCopy(h.plugin.GetReadResources()),
	)
}

func TestDiffReportsNewResourceAsCreate(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffAddedConfig)

	require.Equal(t, []string{diffContainerWorkerID}, diffAddresses(result))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffContainerWorkerID))
	require.Equal(t, diff.Summary{Create: 1, Unchanged: 3}, result.Summary)
}

func TestDiffReportsRemovedResourceAsDelete(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffRemovedConfig)

	require.Equal(t, []diff.Resource{
		{Address: diffContainerWebID, Action: diff.ActionDelete},
	}, result.Resources)
	require.Equal(t, diff.Summary{Delete: 1, Unchanged: 2}, result.Summary)
}

func TestDiffReportsEditedAttributeAsUpdate(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffChangedAttributeConfig)

	require.Len(t, result.Resources, 1)
	require.Equal(t, diffContainerAPIID, result.Resources[0].Address)
	require.Equal(t, diff.ActionUpdate, result.Resources[0].Action)
}

func TestDiffReportsDriftAsUpdate(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	// the real network changed outside xcl
	h.plugin.SetReadObserved(diffNetworkAppID, "drifted")

	result := h.runDiff(t, diffBaseConfig)

	require.Len(t, result.Resources, 1)
	require.Equal(t, diffNetworkAppID, result.Resources[0].Address)
	require.Equal(t, diff.ActionUpdate, result.Resources[0].Action)
	require.Equal(t, diff.Summary{Update: 1, Unchanged: 2}, result.Summary)
}

func TestDiffReportsResourceTheProviderNoLongerFindsAsCreate(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	h.plugin.SetReadNotFound(diffContainerWebID)

	result := h.runDiff(t, diffBaseConfig)

	require.Equal(t, []string{diffContainerWebID}, diffAddresses(result))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffContainerWebID))
	require.Equal(t, diff.Summary{Create: 1, Unchanged: 2}, result.Summary)
}

func TestDiffReportsFailedResourceAsReplace(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	result := h.runDiff(t, lifecycleOriginalConfig)

	require.Equal(t, []diff.Resource{
		{Address: lifecycleNetworkID, Action: diff.ActionReplace},
	}, result.Resources)
	require.Equal(t, diff.Summary{Replace: 1}, result.Summary)
}

func TestDiffNeverReadsFailedResource(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkCreate(t, h)

	h.runDiff(t, lifecycleOriginalConfig)

	require.Empty(t, h.plugin.GetReadResources())
	require.Empty(t, h.plugin.GetChangedCalls())
	require.Empty(t, h.plugin.GetCalls())
}

// failNetworkRebuild leaves the single network saved as destroy_failed: its
// create fails, then the next apply's rebuild fails to destroy it. The
// plugin's errors and calls are cleared afterwards.
func failNetworkRebuild(t *testing.T, h *lifecycleHarness) {
	t.Helper()

	failNetworkCreate(t, h)

	h.plugin.SetDestroyError(lifecycleNetworkID, fmt.Errorf("network is still in use"))

	_, err := h.applyAndSaveExpectingFailure(t, lifecycleOriginalConfig)
	require.Contains(t, err.Error(), "destroy failed for "+lifecycleNetworkID)
	require.Equal(t, types.StatusDestroyFailed, networkStatus(t, h.loadSaved(t)))

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()
}

func TestDiffReportsDestroyFailedResourceAsReplace(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkRebuild(t, h)

	result := h.runDiff(t, lifecycleOriginalConfig)

	require.Equal(t, []diff.Resource{
		{Address: lifecycleNetworkID, Action: diff.ActionReplace},
	}, result.Resources)
	require.Equal(t, diff.Summary{Replace: 1}, result.Summary)
}

func TestDiffNeverReadsDestroyFailedResource(t *testing.T) {
	h := setupLifecycle(t)
	failNetworkRebuild(t, h)

	h.runDiff(t, lifecycleOriginalConfig)

	require.Empty(t, h.plugin.GetReadResources())
	require.Empty(t, h.plugin.GetChangedCalls())
	require.Empty(t, h.plugin.GetCalls())
}

func TestDiffOfIdenticalConfigurationListsNothing(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffBaseConfig)

	require.Empty(t, result.Resources)
	require.Equal(t, diff.Summary{Unchanged: diffBaseProviderCount}, result.Summary)
	require.Zero(t, result.Changed())
}

func TestDiffListsOnlyTheChangedResourceAndCountsTheRestUnchanged(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffChangedAttributeConfig)

	require.Equal(t, []string{diffContainerAPIID}, diffAddresses(result))
	require.Equal(t, 2, result.Summary.Unchanged)
	require.Equal(t, diff.Summary{Update: 1, Unchanged: 2}, result.Summary)
}

func TestDiffListsResourcesSortedByAddress(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffMixedConfig)

	// the deleted web container is recorded before the walk starts, it is
	// still listed in address order
	require.Equal(t, []string{
		diffContainerAPIID,
		diffContainerWebID,
		diffContainerWorkerID,
		diffNetworkBackendID,
	}, diffAddresses(result))
}

func TestDiffOfMixedChangesReportsEachActionAndCountsThem(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	result := h.runDiff(t, diffMixedConfig)

	require.Equal(t, diff.ActionUpdate, diffActionFor(t, result, diffContainerAPIID))
	require.Equal(t, diff.ActionDelete, diffActionFor(t, result, diffContainerWebID))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffContainerWorkerID))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffNetworkBackendID))

	require.Equal(t, diff.Summary{
		Create:    2,
		Update:    1,
		Replace:   0,
		Delete:    1,
		Unchanged: 1,
	}, result.Summary)
	require.Equal(t, 4, result.Changed())
}

func TestDiffNeitherListsNorCountsBlocksWithoutAProviderWhenUnchanged(t *testing.T) {
	h := setupLifecycle(t)
	h.registerDatabaseType(t)
	h.applyAndSave(t, diffWithBuiltinsConfig)

	result := h.runDiff(t, diffWithBuiltinsConfig)

	// only the app network is provider-backed and it is unchanged
	require.Empty(t, result.Resources)
	require.Equal(t, diff.Summary{Unchanged: 1}, result.Summary)
}

func TestDiffNeitherListsNorCountsNewBlocksWithoutAProvider(t *testing.T) {
	h := setupLifecycle(t)
	h.registerDatabaseType(t)

	// nothing has been applied, every block is new
	result := h.runDiff(t, diffWithBuiltinsConfig)

	require.Equal(t, []string{diffNetworkAppID}, diffAddresses(result))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffNetworkAppID))
	require.Equal(t, diff.Summary{Create: 1}, result.Summary)

	addresses := diffAddresses(result)
	require.NotContains(t, addresses, diffBuiltinVariableID)
	require.NotContains(t, addresses, diffBuiltinOutputID)
	require.NotContains(t, addresses, diffBuiltinModuleID)
	require.NotContains(t, addresses, diffDisabledNetworkID)
	require.NotContains(t, addresses, diffRegisteredDBID)
}

func TestDiffNeverListsRemovedBlocksWithoutAProviderAsDelete(t *testing.T) {
	h := setupLifecycle(t)
	h.registerDatabaseType(t)
	h.applyAndSave(t, diffWithBuiltinsConfig)

	saved := stateIDs(t, h.loadSaved(t))
	require.Contains(t, saved, diffBuiltinVariableID)
	require.Contains(t, saved, diffBuiltinOutputID)
	require.Contains(t, saved, diffDisabledNetworkID)
	require.Contains(t, saved, diffRegisteredDBID)

	// base keeps the app network with the same subnet, adds the api and web
	// containers and drops every block without a provider
	result := h.runDiff(t, diffBaseConfig)

	require.Equal(t, []string{diffContainerAPIID, diffContainerWebID}, diffAddresses(result))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffContainerAPIID))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffContainerWebID))
	require.Equal(t, diff.Summary{Create: 2, Unchanged: 1}, result.Summary)
}

func TestDiffFailsWhenAReadFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	injected := stderrors.New(diffInjectedReadFailed)
	h.plugin.SetReadError(diffContainerAPIID, injected)

	p := h.newParser(t, nil)

	result, err := p.Diff(context.Background(), diff.Options{}, diffBaseConfig)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), diffContainerAPIID)
	require.True(t, stderrors.Is(err, injected), "error does not wrap the injected error: %v", err)
}

func TestDiffFailsWhenChangeDetectionFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	injected := stderrors.New("change detection unavailable")
	h.plugin.SetChangedError(diffNetworkAppID, injected)

	p := h.newParser(t, nil)

	result, err := p.Diff(context.Background(), diff.Options{}, diffBaseConfig)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), diffNetworkAppID)
	require.True(t, stderrors.Is(err, injected), "error does not wrap the injected error: %v", err)
}

func TestDiffEmitsReadAndChangedEventsForASavedResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err := p.Diff(context.Background(), diff.Options{}, diffChangedAttributeConfig)
	require.NoError(t, err)

	require.Equal(t, []string{
		"read start",
		"read success",
		"changed start",
		"changed success",
	}, eventsFor(collector.all(), diffContainerAPIID))
}

func TestDiffEmitsNoCreateUpdateOrDestroyEvents(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	// mixed adds, removes and edits resources
	_, err := p.Diff(context.Background(), diff.Options{}, diffMixedConfig)
	require.NoError(t, err)

	recorded := collector.all()
	require.NotEmpty(t, recorded)

	for _, event := range recorded {
		require.NotEqual(t, events.OperationCreate, event.Operation, "unexpected event %s %s for %s", event.Operation, event.Phase, event.ResourceID)
		require.NotEqual(t, events.OperationUpdate, event.Operation, "unexpected event %s %s for %s", event.Operation, event.Phase, event.ResourceID)
		require.NotEqual(t, events.OperationDestroy, event.Operation, "unexpected event %s %s for %s", event.Operation, event.Phase, event.ResourceID)
	}
}

func TestDiffLeavesTheStateFileUntouched(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)

	before, err := os.ReadFile(h.statePath)
	require.NoError(t, err)

	result := h.runDiff(t, diffMixedConfig)
	require.NotZero(t, result.Changed())

	after, err := os.ReadFile(h.statePath)
	require.NoError(t, err)

	require.Equal(t, string(before), string(after))
}

func TestDiffRequiresAPath(t *testing.T) {
	h := setupLifecycle(t)
	p := h.newParser(t, nil)

	result, err := p.Diff(context.Background(), diff.Options{})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestDiffRejectsEmptyConfiguration(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffBaseConfig)
	h.plugin.ResetCalls()

	p := h.newParser(t, nil)

	result, err := p.Diff(context.Background(), diff.Options{}, emptyConfigDir)
	require.Error(t, err)
	require.True(t, stderrors.Is(err, ErrEmptyConfiguration), "unexpected error: %v", err)
	require.Nil(t, result)
	require.Empty(t, h.plugin.GetCalls())
}
