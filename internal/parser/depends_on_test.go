package parser

import (
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// writtenAndReferencedOffID is the disabled entity in the written and
// referenced fixture, it names b in a written depends_on.
const writtenAndReferencedOffID = "resource.network.off"

// parseWrittenAndReferenced parses the written and referenced fixture without
// applying it and returns the parsed state.
func parseWrittenAndReferenced(t *testing.T) *State {
	t.Helper()

	p, _ := setupParser(t)

	st, _, err := p.parseAndValidate(lifecycleWrittenAndReferencedConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	return st
}

// requireDependsOn returns the dependency list the entity holds.
func requireDependsOn(t *testing.T, entity any) []string {
	t.Helper()

	dependsOn, err := types.GetDependencies(entity)
	require.NoError(t, err)

	return dependsOn
}

// requireLinks returns the links recorded in the entity's meta.
func requireLinks(t *testing.T, entity any) []string {
	t.Helper()

	meta, err := types.GetMeta(entity)
	require.NoError(t, err)

	return meta.Links
}

// requireSavedEntity returns the entity with the given ID from what the
// harness store last saved.
func requireSavedEntity(t *testing.T, h *lifecycleHarness, id string) any {
	t.Helper()

	entity, err := findByID(h.loadSaved(t), id)
	require.NoError(t, err)

	return entity
}

// After parsing, an entity's dependency list holds exactly what was written in
// its depends_on, and not the resource it references.
func TestParseKeepsDependsOnAsWritten(t *testing.T) {
	st := parseWrittenAndReferenced(t)
	a := requireEntity(t, st, writtenAndReferencedAID)

	require.Equal(t, []string{"resource.network.b"}, requireDependsOn(t, a))
}

// After parsing, an entity's links hold both the resource it names in
// depends_on and the attribute of the resource it references.
func TestParseRecordsWrittenAndReferencedDependenciesAsLinks(t *testing.T) {
	st := parseWrittenAndReferenced(t)
	a := requireEntity(t, st, writtenAndReferencedAID)

	links := requireLinks(t, a)
	require.Len(t, links, 2)
	require.Contains(t, links, "resource.network.b")
	require.Contains(t, links, "resource.network.c.subnet")
}

// After an apply, the live entity's dependency list still holds exactly what
// was written in its depends_on.
func TestApplyKeepsDependsOnAsWritten(t *testing.T) {
	h := setupLifecycle(t)
	st := h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	a := requireEntity(t, st, writtenAndReferencedAID)

	require.Equal(t, []string{"resource.network.b"}, requireDependsOn(t, a))
}

// After the applied state is saved and loaded back, the entity's dependency
// list still holds exactly what was written in its depends_on.
func TestReloadKeepsDependsOnAsWritten(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	a := requireSavedEntity(t, h, writtenAndReferencedAID)

	require.Equal(t, []string{"resource.network.b"}, requireDependsOn(t, a))
}

// An entity with no depends_on has an empty dependency list after parsing.
func TestParseLeavesDependsOnEmptyWhenNoneWritten(t *testing.T) {
	st := parseWrittenAndReferenced(t)
	alone := requireEntity(t, st, writtenAndReferencedAloneID)

	require.Empty(t, requireDependsOn(t, alone))
}

// An entity with no depends_on has an empty dependency list after an apply.
func TestApplyLeavesDependsOnEmptyWhenNoneWritten(t *testing.T) {
	h := setupLifecycle(t)
	st := h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	alone := requireEntity(t, st, writtenAndReferencedAloneID)

	require.Empty(t, requireDependsOn(t, alone))
}

// An entity with no depends_on has an empty dependency list after the applied
// state is saved and loaded back.
func TestReloadLeavesDependsOnEmptyWhenNoneWritten(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	alone := requireSavedEntity(t, h, writtenAndReferencedAloneID)

	require.Empty(t, requireDependsOn(t, alone))
}

// A disabled entity keeps the dependency list it wrote after parsing. Whether
// it is disabled is only decoded by the walk, so parsing alone cannot show it.
func TestParseKeepsDependsOnAsWrittenForDisabledEntity(t *testing.T) {
	st := parseWrittenAndReferenced(t)
	off := requireEntity(t, st, writtenAndReferencedOffID)

	require.Equal(t, []string{"resource.network.b"}, requireDependsOn(t, off))
}

// A disabled entity keeps the dependency list it wrote in the saved state.
func TestReloadKeepsDependsOnAsWrittenForDisabledEntity(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	off := requireSavedEntity(t, h, writtenAndReferencedOffID)

	disabled, err := types.GetDisabled(off)
	require.NoError(t, err)
	require.True(t, disabled)

	require.Equal(t, []string{"resource.network.b"}, requireDependsOn(t, off))
}

// With the dependency list holding only what was written, the destroy graph
// built from saved state still makes both the resource an entity names in
// depends_on and the resource it references its parents.
func TestSavedGraphParentsAreWrittenAndReferencedDependencies(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)

	require.Equal(
		t,
		[]string{writtenAndReferencedBID, writtenAndReferencedCID},
		savedGraphParents(t, h.newParser(t, nil), writtenAndReferencedAID),
	)
}
