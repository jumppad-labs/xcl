package resources

import (
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// newMatchContainer returns a container entity carrying the metadata a parse
// records for it. Matching compares metadata, it parses nothing, so the axes
// are set here the way a parse sets them.
func newMatchContainer(t *testing.T, name, module string) *testContainer {
	t.Helper()

	r := &testContainer{}

	meta, err := types.GetMeta(r)
	require.NoError(t, err)

	meta.Type = types.TypeResource
	meta.Subtype = typeTestContainer
	meta.Name = name
	meta.Module = module

	return r
}

func TestMatchFindsTheEntityAnAddressNames(t *testing.T) {
	want := newMatchContainer(t, "mine", "")
	entities := []any{newMatchContainer(t, "other", ""), want}

	fqrn, err := ParseFQRN("resource.container.mine")
	require.NoError(t, err)

	got, found := Match(entities, fqrn)
	require.True(t, found)
	require.Same(t, want, got)
}

// An address may carry the attribute the caller went on to read. The attribute
// is no part of naming the entity, so it is ignored.
func TestMatchIgnoresATrailingAttribute(t *testing.T) {
	want := newMatchContainer(t, "main", "")
	entities := []any{want}

	bare, err := ParseFQRN("resource.container.main")
	require.NoError(t, err)

	withAttribute, err := ParseFQRN("resource.container.main.location")
	require.NoError(t, err)

	fromBare, found := Match(entities, bare)
	require.True(t, found)

	fromAttribute, found := Match(entities, withAttribute)
	require.True(t, found)

	require.Same(t, want, fromBare)
	require.Same(t, want, fromAttribute)
}

func TestMatchDoesNotFindAnEntityDeclaredInAnotherModule(t *testing.T) {
	entities := []any{newMatchContainer(t, "mine", "shared")}

	fqrn, err := ParseFQRN("resource.container.mine")
	require.NoError(t, err)

	got, found := Match(entities, fqrn)
	require.False(t, found)
	require.Nil(t, got)
}

func TestMatchModuleReturnsTheEntitiesDeclaredInTheModule(t *testing.T) {
	member := newMatchContainer(t, "mine", "shared")
	entities := []any{newMatchContainer(t, "root", ""), member}

	fqrn, err := ParseFQRN("module.shared")
	require.NoError(t, err)

	found, err := MatchModule(entities, fqrn, false)
	require.NoError(t, err)

	require.Len(t, found, 1)
	require.Same(t, member, found[0])
}

func TestMatchModuleReturnsNestedModuleEntitiesWhenIncludingSubModules(t *testing.T) {
	member := newMatchContainer(t, "mine", "shared")
	nested := newMatchContainer(t, "deep", "shared.inner")
	entities := []any{newMatchContainer(t, "root", ""), member, nested}

	fqrn, err := ParseFQRN("module.shared")
	require.NoError(t, err)

	found, err := MatchModule(entities, fqrn, true)
	require.NoError(t, err)

	require.Len(t, found, 2)
	require.Contains(t, found, any(member))
	require.Contains(t, found, any(nested))
}

func TestMatchModuleOmitsNestedModuleEntitiesWhenNotIncludingSubModules(t *testing.T) {
	member := newMatchContainer(t, "mine", "shared")
	nested := newMatchContainer(t, "deep", "shared.inner")
	entities := []any{member, nested}

	fqrn, err := ParseFQRN("module.shared")
	require.NoError(t, err)

	found, err := MatchModule(entities, fqrn, false)
	require.NoError(t, err)

	require.Len(t, found, 1)
	require.Same(t, member, found[0])
}

func TestMatchModuleReturnsNothingForAModuleNothingWasDeclaredIn(t *testing.T) {
	entities := []any{newMatchContainer(t, "mine", "shared")}

	fqrn, err := ParseFQRN("module.other")
	require.NoError(t, err)

	found, err := MatchModule(entities, fqrn, true)
	require.NoError(t, err)
	require.Empty(t, found)
}

func TestMatchModuleErrorsWhenTheAddressIsNotAModule(t *testing.T) {
	entities := []any{newMatchContainer(t, "mine", "shared")}

	fqrn, err := ParseFQRN("resource.container.mine")
	require.NoError(t, err)

	found, err := MatchModule(entities, fqrn, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "does not name a module")
	require.Nil(t, found)
}

// A module's name is a path segment, not a string prefix. Before this was
// anchored, asking for module "shared" also returned entities of a sibling
// module named "sharedother", which was carried over unchanged from the
// storage layer this matcher replaced.
func TestMatchModuleOmitsASiblingModuleWithASharedPrefix(t *testing.T) {
	entities := []any{
		newMatchContainer(t, "inside", "shared"),
		newMatchContainer(t, "nested", "shared.nested"),
		newMatchContainer(t, "sibling", "sharedother"),
	}

	fqrn, err := ParseFQRN("module.shared")
	require.NoError(t, err)

	found, err := MatchModule(entities, fqrn, true)
	require.NoError(t, err)
	require.Len(t, found, 2)

	for _, e := range found {
		meta, err := types.GetMeta(e)
		require.NoError(t, err)
		require.NotEqual(t, "sibling", meta.Name, "a sibling module must not be reached by prefix")
	}
}
