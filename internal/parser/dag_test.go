package parser

import (
	"slices"
	"testing"

	dagpkg "github.com/jumppad-labs/xcl/internal/dag"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	lifecycleWrittenAndReferencedConfig = "../test_fixtures/config/lifecycle/written_and_referenced/main.xcl"

	writtenAndReferencedAID     = "resource.network.a"
	writtenAndReferencedBID     = "resource.network.b"
	writtenAndReferencedCID     = "resource.network.c"
	writtenAndReferencedAloneID = "resource.network.alone"
)

// isRootVertex reports whether vertex is the root the create or destroy graph
// hangs resources with no dependencies off.
func isRootVertex(vertex any) bool {
	_, isRoot := vertex.(*resources.Root)
	if isRoot {
		return true
	}

	meta, err := types.GetMeta(vertex)
	if err != nil {
		return false
	}

	return meta.Type == resources.TypeRoot
}

// graphParentIDs returns the sorted IDs of the vertices with an edge into
// entity in graph, leaving out the root.
func graphParentIDs(t *testing.T, graph *dagpkg.AcyclicGraph, entity any) []string {
	t.Helper()

	ids := []string{}
	for _, vertex := range graph.UpEdges(entity).List() {
		if isRootVertex(vertex) {
			continue
		}

		meta, err := types.GetMeta(vertex)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	slices.Sort(ids)

	return ids
}

// graphHasRootParent reports whether the root has an edge into entity in graph.
func graphHasRootParent(graph *dagpkg.AcyclicGraph, entity any) bool {
	for _, vertex := range graph.UpEdges(entity).List() {
		if isRootVertex(vertex) {
			return true
		}
	}

	return false
}

// applyForGraph applies the config at path and returns the applied state and
// the parser that applied it, ready to build a graph from.
func applyForGraph(t *testing.T, path string) (*State, *Parser) {
	t.Helper()

	h := setupLifecycle(t)
	p := h.newParser(t, nil)

	st, err := p.Apply(t.Context(), path)
	require.NoError(t, err)
	require.NotNil(t, st)

	return st, p
}

// requireEntity returns the entity with the given ID in st.
func requireEntity(t *testing.T, st *State, id string) any {
	t.Helper()

	entity, err := findByID(st.GetResources(), id)
	require.NoError(t, err)

	return entity
}

// savedGraphParents loads the state p's store last saved, types it with p's
// registry just as Parser.Destroy does, builds the destroy graph over every
// saved entity and returns the sorted IDs of the parents of the entity with the
// given id, leaving out the root. p is only used for its store, registry and
// address parser, build it from the harness the state was saved with.
func savedGraphParents(t *testing.T, p *Parser, id string) []string {
	t.Helper()

	err := p.loadPlugins()
	require.NoError(t, err)

	loaded, err := p.stateStore.Load()
	require.NoError(t, err)

	saved, err := savedentity.DecodeAll(p.pluginRegistry, loaded)
	require.NoError(t, err)

	working := NewState()
	for _, entity := range saved {
		err := working.AppendResource(entity)
		require.NoError(t, err)
	}

	graph, err := buildDestroyDAG(working, p.addressParser(), working.GetResources())
	require.NoError(t, err)

	return graphParentIDs(t, graph, requireEntity(t, working, id))
}

// An entity's parents in the create graph are both the resource it names in
// depends_on and the resource it references.
func TestCreateGraphParentsOfEntityAreWrittenAndReferencedDependencies(t *testing.T) {
	st, p := applyForGraph(t, lifecycleWrittenAndReferencedConfig)
	a := requireEntity(t, st, writtenAndReferencedAID)

	graph, err := buildCreateDAG(st, p.addressParser())
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{writtenAndReferencedBID, writtenAndReferencedCID},
		graphParentIDs(t, graph, a),
	)
}

// The create graph takes an entity's dependencies from its links, so clearing
// the written dependency list does not change its parents.
func TestCreateGraphReadsLinksNotWrittenDependencyList(t *testing.T) {
	st, p := applyForGraph(t, lifecycleWrittenAndReferencedConfig)
	a := requireEntity(t, st, writtenAndReferencedAID)

	err := types.SetDependencies(a, nil)
	require.NoError(t, err)

	graph, err := buildCreateDAG(st, p.addressParser())
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{writtenAndReferencedBID, writtenAndReferencedCID},
		graphParentIDs(t, graph, a),
	)
}

// An entity with no dependencies has no parent in the create graph other than
// the root.
func TestCreateGraphHangsEntityWithNoDependenciesOffRoot(t *testing.T) {
	st, p := applyForGraph(t, lifecycleWrittenAndReferencedConfig)
	alone := requireEntity(t, st, writtenAndReferencedAloneID)

	graph, err := buildCreateDAG(st, p.addressParser())
	require.NoError(t, err)

	require.Empty(t, graphParentIDs(t, graph, alone))
	require.True(t, graphHasRootParent(graph, alone))
}

// A depends_on naming a whole module makes every resource in that module a
// parent in the create graph.
func TestCreateGraphModuleWideDependsOnMakesEveryModuleResourceAParent(t *testing.T) {
	st, p := applyForGraph(t, lifecycleModuleReferenceConfig)
	consumer := requireEntity(t, st, moduleReferenceConsumerID)

	graph, err := buildCreateDAG(st, p.addressParser())
	require.NoError(t, err)

	require.Equal(
		t,
		[]string{moduleReferenceOneID, moduleReferenceTwoID},
		graphParentIDs(t, graph, consumer),
	)
}

// An apply creates the resource an entity names in depends_on and the resource
// it references before it creates the entity itself.
func TestApplyCreatesWrittenAndReferencedDependenciesFirst(t *testing.T) {
	h := setupLifecycle(t)

	h.applyAndSave(t, lifecycleWrittenAndReferencedConfig)
	calls := h.plugin.GetCalls()

	createA := slices.Index(calls, "create "+writtenAndReferencedAID)
	createB := slices.Index(calls, "create "+writtenAndReferencedBID)
	createC := slices.Index(calls, "create "+writtenAndReferencedCID)

	require.NotEqual(t, -1, createA, "no create call for %s in %v", writtenAndReferencedAID, calls)
	require.NotEqual(t, -1, createB, "no create call for %s in %v", writtenAndReferencedBID, calls)
	require.NotEqual(t, -1, createC, "no create call for %s in %v", writtenAndReferencedCID, calls)

	require.Less(t, createB, createA)
	require.Less(t, createC, createA)
}
