package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/types"
)

// destroyIDs returns the IDs of the entities in targets, in order
func destroyIDs(t *testing.T, targets []any) []string {
	t.Helper()

	ids := []string{}
	for _, target := range targets {
		meta, err := types.GetMeta(target)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	return ids
}

func TestDecisionsToDestroyListsReplacedAndRemoved(t *testing.T) {
	previous, _ := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()
	record.decide("resource.network.replaced", decision{action: diff.ActionReplace, reason: diff.ReplaceProvider})
	record.decide("resource.container.user", decision{action: diff.ActionDelete})

	targets := record.toDestroy(previous)

	require.Equal(t, []string{
		"resource.network.replaced",
		"resource.container.user",
	}, destroyIDs(t, targets))
}

func TestDecisionsToDestroyReturnsTheSavedEntities(t *testing.T) {
	previous, _ := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()
	record.decide("resource.network.replaced", decision{action: diff.ActionReplace, reason: diff.ReplaceProvider})

	targets := record.toDestroy(previous)

	require.Len(t, targets, 1)
	require.Same(t, requireEntity(t, previous, "resource.network.replaced"), targets[0])
}

func TestDecisionsToDestroyOmitsUpdatedAndUnchanged(t *testing.T) {
	previous, _ := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()
	record.decide("resource.network.replaced", decision{action: diff.ActionCreate})
	record.decide("resource.network.updated", decision{action: diff.ActionUpdate})
	record.decide("resource.network.same", decision{})

	targets := record.toDestroy(previous)

	require.Empty(t, targets)
}

func TestDecisionsToDestroyOmitsUndecidedEntities(t *testing.T) {
	previous, _ := parseForDependencies(t, twoDependenciesConfig)

	record := newDiffRecorder()

	targets := record.toDestroy(previous)

	require.Empty(t, targets)
}
