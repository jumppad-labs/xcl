package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser/mocks"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	lifecycleDisabledConfig = "../test_fixtures/config/lifecycle/disabled/disabled.xcl"

	disabledOnID  = "resource.network.on"
	disabledOffID = "resource.network.off"

	dependentVariableID = "variable.independent_subnet"
	dependentOutputID   = "output.first_name"

	lifecycleModuleInternalReferenceConfig = "../test_fixtures/config/lifecycle/module_internal_reference/main.xcl"

	moduleInternalReferenceOneID = "module.inner.resource.network.one"
	moduleInternalReferenceTwoID = "module.inner.resource.network.two"
)

// dependentProviderIDs are the resources of the dependent fixture a provider
// creates and destroys, the variable and output are builtin types.
var dependentProviderIDs = []string{
	dependentFirstID,
	dependentSecondID,
	dependentThirdID,
	dependentIndependentID,
}

// dependentParents is, for every resource of the dependent fixture, the
// resources it depends on. None of them may be destroyed while it remains.
var dependentParents = map[string][]string{
	dependentFirstID:       {},
	dependentSecondID:      {dependentFirstID},
	dependentThirdID:       {dependentSecondID},
	dependentIndependentID: {dependentVariableID},
	dependentOutputID:      {dependentFirstID},
	dependentVariableID:    {},
}

// destroyAll loads the state the harness store last saved and destroys it with
// a fresh Parser, just as a separate destroy run would. emit may be nil.
func destroyAll(t *testing.T, h *lifecycleHarness, emit events.Emit) (*State, error) {
	t.Helper()

	p := h.newParser(t, emit)

	loaded, err := h.store.Load()
	require.NoError(t, err)

	return p.Destroy(context.Background(), loaded)
}

// destroyCalls returns the IDs the provider was asked to destroy, in order.
func destroyCalls(p *TestPlugin) []string {
	ids := []string{}
	for _, call := range p.GetCalls() {
		id, found := strings.CutPrefix(call, "destroy ")
		if found {
			ids = append(ids, id)
		}
	}

	return ids
}

// stateIDs returns the sorted IDs of every entity in entities.
func stateIDs(t *testing.T, entities []any) []string {
	t.Helper()

	ids := []string{}
	for _, r := range entities {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)

	return ids
}

// snapshotIDs returns the sorted IDs of every resource in a serialized state.
func snapshotIDs(t *testing.T, snapshot []byte) []string {
	t.Helper()

	resources := []struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}{}

	err := json.Unmarshal(snapshot, &resources)
	require.NoError(t, err)

	ids := []string{}
	for _, r := range resources {
		ids = append(ids, r.Meta.ID)
	}

	sort.Strings(ids)

	return ids
}

// sorted returns a sorted copy of ids.
func sorted(ids ...string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)

	return out
}

// recordingStore is a state store that saves through another store and keeps
// a copy of every state it was asked to save.
type recordingStore struct {
	store state.StateStore

	mu        sync.Mutex
	snapshots [][]byte
}

func (s *recordingStore) Load() ([]any, error) {
	return s.store.Load()
}

func (s *recordingStore) Save(entities []any) error {
	snapshot, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.snapshots = append(s.snapshots, snapshot)
	s.mu.Unlock()

	return s.store.Save(entities)
}

func (s *recordingStore) Exists() bool {
	return s.store.Exists()
}

func (s *recordingStore) Clear() error {
	return s.store.Clear()
}

func (s *recordingStore) all() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([][]byte{}, s.snapshots...)
}

// destroyRecordingSaves destroys the saved state of the harness with a parser
// whose store records every save, and returns the recorded snapshots.
func destroyRecordingSaves(t *testing.T, h *lifecycleHarness) [][]byte {
	t.Helper()

	recording := &recordingStore{store: h.store}

	options := testOptions(t)
	options.Catalog = h.registry
	options.StateStore = recording

	p := NewParser(options)

	loaded, err := h.store.Load()
	require.NoError(t, err)

	remaining, err := p.Destroy(context.Background(), loaded)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	return recording.all()
}

func TestDestroyCallsProvidersChildrenFirst(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	requireBefore(t, dependentThirdID, dependentSecondID, calls)
	requireBefore(t, dependentSecondID, dependentFirstID, calls)
	require.Contains(t, calls, dependentIndependentID)
}

func TestDestroyRemovesEveryResourceFromSavedState(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	require.Equal(t, 6, len(h.loadSaved(t)))

	remaining, err := destroyAll(t, h, nil)
	require.NoError(t, err)
	require.NotNil(t, remaining)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Equal(t, 0, len(h.loadSaved(t)))
}

func TestDestroyCallsEachProviderResourceExactlyOnce(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	require.Len(t, calls, 4)
	require.ElementsMatch(t, dependentProviderIDs, calls)
}

func TestDestroyFailureLeavesParentsAndDestroysUnrelated(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	remaining, err := destroyAll(t, h, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), dependentSecondID)
	require.Contains(t, err.Error(), "boom")

	calls := destroyCalls(h.plugin)
	require.Contains(t, calls, dependentThirdID)
	require.Contains(t, calls, dependentIndependentID)
	require.Contains(t, calls, dependentSecondID)
	require.NotContains(t, calls, dependentFirstID)

	require.Equal(t, sorted(dependentSecondID, dependentFirstID), stateIDs(t, remaining.GetResources()))

	saved := h.loadSaved(t)
	require.Equal(t, sorted(dependentSecondID, dependentFirstID), stateIDs(t, saved))
	require.Equal(t, types.StatusDestroyFailed, resourceStatus(t, saved, dependentSecondID))
	require.Equal(t, types.StatusCreated, resourceStatus(t, saved, dependentFirstID))
}

func TestDestroyRetriesFailedResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	_, err := destroyAll(t, h, nil)
	require.Error(t, err)

	h.plugin.ClearErrors()
	h.plugin.ResetCalls()

	remaining, err := destroyAll(t, h, nil)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Equal(t, []string{dependentSecondID, dependentFirstID}, destroyCalls(h.plugin))
	require.Equal(t, 0, len(h.loadSaved(t)))
}

func TestDestroySavesStateAfterEachResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)

	// one save for each of the six resources
	require.Len(t, snapshots, 6)

	previous := sorted(
		dependentFirstID,
		dependentSecondID,
		dependentThirdID,
		dependentIndependentID,
		dependentOutputID,
		dependentVariableID,
	)

	for i, snapshot := range snapshots {
		current := snapshotIDs(t, snapshot)

		// each save holds exactly one resource fewer than the one before
		require.Len(t, current, len(previous)-1, "snapshot %d: %v", i, current)
		for _, id := range current {
			require.Contains(t, previous, id, "snapshot %d holds %s, destroyed earlier", i, id)
		}

		// a resource that remains still has every resource it depends on
		for _, id := range current {
			for _, parent := range dependentParents[id] {
				require.Contains(t, current, parent, "snapshot %d holds %s but not its parent %s", i, id, parent)
			}
		}

		previous = current
	}

	require.Empty(t, previous)
}

func TestDestroyFirstSaveRemovesAResourceNothingDependsOn(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)
	require.NotEmpty(t, snapshots)

	all := sorted(
		dependentFirstID,
		dependentSecondID,
		dependentThirdID,
		dependentIndependentID,
		dependentOutputID,
		dependentVariableID,
	)

	first := snapshotIDs(t, snapshots[0])

	removed := []string{}
	for _, id := range all {
		if !contains(first, id) {
			removed = append(removed, id)
		}
	}

	require.Len(t, removed, 1)
	require.Contains(t, []string{dependentThirdID, dependentIndependentID, dependentOutputID}, removed[0])
}

func TestInterruptedDestroyResumesFromSavedState(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)
	require.Len(t, snapshots, 6)

	// restart the destroy from every point the first run saved, as if it had
	// been interrupted straight after that save
	for i, snapshot := range snapshots {
		err := os.WriteFile(h.statePath, snapshot, 0644)
		require.NoError(t, err)

		remainingProviderIDs := []string{}
		for _, id := range snapshotIDs(t, snapshot) {
			if contains(dependentProviderIDs, id) {
				remainingProviderIDs = append(remainingProviderIDs, id)
			}
		}

		h.plugin.ResetCalls()

		remaining, err := destroyAll(t, h, nil)
		require.NoError(t, err, "resuming from snapshot %d", i)
		require.Equal(t, 0, remaining.ResourceCount())

		require.ElementsMatch(t, remainingProviderIDs, destroyCalls(h.plugin), "resuming from snapshot %d", i)
		require.Equal(t, 0, len(h.loadSaved(t)))
	}
}

func TestInterruptedDestroyAfterChildDestroyedNeverDestroysItAgain(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	snapshots := destroyRecordingSaves(t, h)

	// find the first save that no longer holds third, the rest of the chain is
	// still there
	resumeFrom := -1
	for i, snapshot := range snapshots {
		if !contains(snapshotIDs(t, snapshot), dependentThirdID) {
			resumeFrom = i
			break
		}
	}
	require.NotEqual(t, -1, resumeFrom)

	err := os.WriteFile(h.statePath, snapshots[resumeFrom], 0644)
	require.NoError(t, err)
	h.plugin.ResetCalls()

	_, err = destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	require.NotContains(t, calls, dependentThirdID)
	requireBefore(t, dependentSecondID, dependentFirstID, calls)
}

func TestDestroyNeverCallsProviderForBuiltinAndRegisteredBlocks(t *testing.T) {
	h := setupRegisteredTypes(t)
	h.applyAndSave(t, registeredBasicConfig)

	collector := &eventCollector{}

	options := testOptions(t)
	options.Catalog = h.registry
	options.StateStore = h.store
	options.Emit = collector.collect
	// no expectations, any provider lookup fails the test
	options.ProviderResolver = mocks.NewMockProviderResolver(t)

	p := NewParser(options)

	loaded, err := h.store.Load()
	require.NoError(t, err)
	require.Len(t, loaded, 7)

	remaining, err := p.Destroy(context.Background(), loaded)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.Empty(t, saved)

	events := collector.all()
	for _, id := range []string{
		registeredVariableID,
		registeredDatabaseID,
		"module.shared",
		registeredModuleDatabaseID,
		"module.shared.output.location",
		registeredAppID,
		registeredConsumerID,
	} {
		require.Equal(t, []string{"destroy success"}, eventsFor(events, id), id)
	}
}

func TestDestroyNeverCallsProviderForDisabledRegisteredBlock(t *testing.T) {
	h := setupRegisteredTypes(t)
	h.applyAndSave(t, registeredDisabledConfig)

	collector := &eventCollector{}

	options := testOptions(t)
	options.Catalog = h.registry
	options.StateStore = h.store
	options.Emit = collector.collect
	// no expectations, any provider lookup fails the test
	options.ProviderResolver = mocks.NewMockProviderResolver(t)

	p := NewParser(options)

	loaded := loadTyped(t, h.store, h.registry)
	require.Equal(t, []string{registeredDisabledID}, stateIDs(t, loaded))

	remaining, err := p.Destroy(context.Background(), loaded)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	saved, err := h.store.Load()
	require.NoError(t, err)
	require.Empty(t, saved)

	require.Equal(t, []string{"destroy success"}, eventsFor(collector.all(), registeredDisabledID))
}

func TestDestroyNeverCallsProviderForDisabledProviderBlock(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDisabledConfig)
	require.Equal(t, sorted(disabledOnID, disabledOffID), stateIDs(t, h.loadSaved(t)))
	h.plugin.ResetCalls()

	collector := &eventCollector{}

	remaining, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Equal(t, []string{disabledOnID}, destroyCalls(h.plugin))
	require.Equal(t, []string{"destroy success"}, eventsFor(collector.all(), disabledOffID))
	require.Equal(t, 0, len(h.loadSaved(t)))
}

func TestDestroyOrphansNothingWhenAResourceFails(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	created := h.plugin.GetCreatedResources()
	require.ElementsMatch(t, dependentProviderIDs, created)

	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	_, err := destroyAll(t, h, nil)
	require.Error(t, err)

	destroyedByProvider := []string{}
	for _, id := range destroyCalls(h.plugin) {
		if id != dependentSecondID {
			destroyedByProvider = append(destroyedByProvider, id)
		}
	}

	saved := stateIDs(t, h.loadSaved(t))

	// every resource a provider created is either gone or still saved
	for _, id := range created {
		if contains(destroyedByProvider, id) {
			require.NotContains(t, saved, id)
			continue
		}

		require.Contains(t, saved, id, "%s was created but is neither destroyed nor saved", id)
	}
}

func TestDestroyNeverDestroysParentOfFailedChild(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	_, err := destroyAll(t, h, nil)
	require.Error(t, err)

	calls := destroyCalls(h.plugin)
	saved := h.loadSaved(t)

	failed := 0
	for _, r := range saved {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		if meta.Status != types.StatusDestroyFailed {
			continue
		}

		failed++
		for _, parent := range savedGraphParents(t, h.newParser(t, nil), meta.ID) {
			require.NotContains(t, calls, parent, "%s was destroyed after its child %s failed", parent, meta.ID)
		}
	}

	require.Equal(t, 1, failed)
}

func TestDestroyWithEmptyStateCallsNoProvider(t *testing.T) {
	h := setupLifecycle(t)
	p := h.newParser(t, nil)

	remaining, err := p.Destroy(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, remaining)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Empty(t, h.plugin.GetCalls())
}

func TestDestroyWithNilStateCallsNoProvider(t *testing.T) {
	h := setupLifecycle(t)
	p := h.newParser(t, nil)

	remaining, err := p.Destroy(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, remaining)
	require.Equal(t, 0, remaining.ResourceCount())

	require.Empty(t, h.plugin.GetCalls())
}

func TestDestroyFiresStartThenSuccessForProviderResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)

	require.Equal(t, []string{"destroy start", "destroy success"}, eventsFor(collector.all(), dependentFirstID))
}

func TestDestroyFiresStartThenErrorForFailingResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.SetDestroyError(dependentSecondID, fmt.Errorf("boom"))

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.Error(t, err)

	events := collector.all()
	require.Equal(t, []string{"destroy start", "destroy error"}, eventsFor(events, dependentSecondID))
	require.Empty(t, eventsFor(events, dependentFirstID))
}

func TestDestroyFiresOnlySuccessForVariable(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)

	events := collector.all()
	require.Equal(t, []string{"destroy success"}, eventsFor(events, dependentVariableID))
	require.Equal(t, []string{"destroy success"}, eventsFor(events, dependentOutputID))
}

// contains reports whether ids holds id.
func contains(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}

	return false
}

// loadSavedState loads the entities the harness store last saved into a fresh
// working state, and returns it with the address parser of a fresh parser,
// just as Parser.Destroy prepares them.
func loadSavedState(t *testing.T, h *lifecycleHarness) (*State, *resources.AddressParser) {
	t.Helper()

	p := h.newParser(t, nil)

	err := p.loadPlugins()
	require.NoError(t, err)

	working := NewState()
	for _, entity := range h.loadSaved(t) {
		err := working.AppendResource(entity)
		require.NoError(t, err)
	}

	return working, p.addressParser()
}

// eventIndex returns the position in recorded of the event with the given
// resource ID, operation and phase, failing the test when there is none.
func eventIndex(t *testing.T, recorded []events.Event, resourceID string, operation string, phase string) int {
	t.Helper()

	for i, event := range recorded {
		if event.ResourceID == resourceID && event.Operation == operation && event.Phase == phase {
			return i
		}
	}

	require.Failf(t, "event not found", "no %s %s event for %s", operation, phase, resourceID)

	return -1
}

// Destroying from saved state destroys an entity before the resource it names
// in depends_on.
func TestDestroyFromSavedStateDestroysBeforeWrittenDependency(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	requireBefore(t, writtenAndReferencedAID, writtenAndReferencedBID, calls)
}

// Destroying from saved state destroys an entity before the resource it
// references.
func TestDestroyFromSavedStateDestroysBeforeReferencedDependency(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	requireBefore(t, writtenAndReferencedAID, writtenAndReferencedCID, calls)
}

// The destroy graph takes an entity's dependencies from the links it saved, so
// clearing its written dependency list does not change its parents.
func TestDestroyFromSavedStateIgnoresWrittenDependsOnList(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)

	working, addresses := loadSavedState(t, h)
	a := requireEntity(t, working, writtenAndReferencedAID)

	err := types.SetDependencies(a, nil)
	require.NoError(t, err)

	graph, err := buildDestroyDAG(working, addresses, working.GetResources())
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{writtenAndReferencedBID, writtenAndReferencedCID},
		graphParentIDs(t, graph, a),
	)
}

// Destroying from saved state destroys an entity that depends on a whole module
// before every resource in that module.
func TestDestroyFromSavedStateDestroysModuleWideDependentFirst(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleModuleReferenceConfig)

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)

	recorded := collector.all()
	consumer := eventIndex(t, recorded, moduleReferenceConsumerID, events.OperationDestroy, events.PhaseSuccess)
	one := eventIndex(t, recorded, moduleReferenceOneID, events.OperationDestroy, events.PhaseSuccess)
	two := eventIndex(t, recorded, moduleReferenceTwoID, events.OperationDestroy, events.PhaseSuccess)

	require.Less(t, consumer, one)
	require.Less(t, consumer, two)
}

// Destroying from saved state destroys the resources in a module before the
// module itself.
func TestDestroyFromSavedStateDestroysModuleResourcesBeforeModule(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleModuleReferenceConfig)

	collector := &eventCollector{}

	_, err := destroyAll(t, h, collector.collect)
	require.NoError(t, err)

	recorded := collector.all()
	module := eventIndex(t, recorded, moduleReferenceModuleID, events.OperationDestroy, events.PhaseSuccess)
	one := eventIndex(t, recorded, moduleReferenceOneID, events.OperationDestroy, events.PhaseSuccess)
	two := eventIndex(t, recorded, moduleReferenceTwoID, events.OperationDestroy, events.PhaseSuccess)

	require.Less(t, one, module)
	require.Less(t, two, module)
}

// Destroying from saved state resolves a reference written relative to the
// module it sits in, and destroys the referencing resource first.
func TestDestroyFromSavedStateResolvesModuleRelativeLinks(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleModuleInternalReferenceConfig)
	h.plugin.ResetCalls()

	_, err := destroyAll(t, h, nil)
	require.NoError(t, err)

	calls := destroyCalls(h.plugin)
	requireBefore(t, moduleInternalReferenceTwoID, moduleInternalReferenceOneID, calls)
}

// The module-relative link a resource inside a module saved resolves from
// saved state alone, so the resource it references is one of its parents in
// the destroy graph alongside the module.
func TestDestroyGraphResolvesModuleRelativeLinksFromSavedState(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleModuleInternalReferenceConfig)

	working, addresses := loadSavedState(t, h)
	two := requireEntity(t, working, moduleInternalReferenceTwoID)

	graph, err := buildDestroyDAG(working, addresses, working.GetResources())
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{"module.inner", moduleInternalReferenceOneID},
		graphParentIDs(t, graph, two),
	)
}

// The destroy graph orders only the entities being destroyed, a dependency
// that stays in the state is not one of an entity's parents.
func TestDestroyGraphIgnoresDependenciesOutsideTheSet(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)

	working, addresses := loadSavedState(t, h)
	a := requireEntity(t, working, writtenAndReferencedAID)
	c := requireEntity(t, working, writtenAndReferencedCID)

	graph, err := buildDestroyDAG(working, addresses, []any{a, c})
	require.NoError(t, err)

	require.Equal(t, []string{writtenAndReferencedCID}, graphParentIDs(t, graph, a))
}

// The destroy graph builds when the module a resource sits in is no longer in
// the state, and the resource then hangs off the root.
func TestDestroyGraphIgnoresMissingParentModule(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleModuleReferenceConfig)

	working, addresses := loadSavedState(t, h)
	module := requireEntity(t, working, moduleReferenceModuleID)
	one := requireEntity(t, working, moduleReferenceOneID)

	err := working.RemoveResource(module)
	require.NoError(t, err)

	targets := append([]any{}, working.GetResources()...)

	graph, err := buildDestroyDAG(working, addresses, targets)
	require.NoError(t, err)

	require.Empty(t, graphParentIDs(t, graph, one))
	require.True(t, graphHasRootParent(graph, one))
}
