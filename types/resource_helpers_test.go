package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type TestBaseResource struct {
	ResourceBase
	Name string
}

type TestExtendedResource struct {
	TestBaseResource
	ExtraField string
}

func testCreateBasicResource() *TestBaseResource {
	return &TestBaseResource{
		ResourceBase: ResourceBase{
			DependsOn: []string{"dependency1", "dependency2"},
			Disabled:  true,
			Meta: Meta{
				ID:      "test-id",
				Name:    "test-name",
				Type:    TypeResource,
				Subtype: "test-type",
			},
		},
		Name: "test-resource",
	}
}

func testCreateExtendedResource() *TestExtendedResource {
	br := testCreateBasicResource()
	return &TestExtendedResource{
		TestBaseResource: *br,
		ExtraField:       "extra-value",
	}
}

func TestCanGetMetaOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "test-id", meta.ID)
	require.Equal(t, "test-name", meta.Name)
	require.Equal(t, TypeResource, meta.Type)
	require.Equal(t, "test-type", meta.Subtype)
}

func TestCanSetMetaOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	meta.ID = "new-id"
	meta.Name = "new-name"
	meta.Type = TypeResource
	meta.Subtype = "new-type"

	newMeta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "new-id", newMeta.ID)
	require.Equal(t, "new-name", newMeta.Name)
	require.Equal(t, TypeResource, newMeta.Type)
	require.Equal(t, "new-type", newMeta.Subtype)
}

func TestCanGetMetaOnExtendedResource(t *testing.T) {
	te := testCreateExtendedResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "test-id", meta.ID)
	require.Equal(t, "test-name", meta.Name)
	require.Equal(t, TypeResource, meta.Type)
	require.Equal(t, "test-type", meta.Subtype)
}

func TestCanSetMetaOnExtendedResource(t *testing.T) {
	te := testCreateExtendedResource()

	meta, err := GetMeta(te)
	require.NoError(t, err)

	meta.ID = "new-id"
	meta.Name = "new-name"
	meta.Type = TypeResource
	meta.Subtype = "new-type"

	newMeta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, "new-id", newMeta.ID)
	require.Equal(t, "new-name", newMeta.Name)
	require.Equal(t, TypeResource, newMeta.Type)
	require.Equal(t, "new-type", newMeta.Subtype)
}

func TestCanGetDependenciesOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	require.Len(t, deps, 2)
	require.Contains(t, deps, "dependency1")
	require.Contains(t, deps, "dependency2")
}

func TestCanSetDependenciesOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	deps = append(deps, "dependency3")

	err = SetDependencies(te, deps)
	require.NoError(t, err)

	newDeps, err := GetDependencies(te)
	require.NoError(t, err)
	require.Len(t, newDeps, 3)
	require.Contains(t, newDeps, "dependency1")
	require.Contains(t, newDeps, "dependency2")
	require.Contains(t, newDeps, "dependency3")
}

func TestAppendUniqueLinkAddsOnce(t *testing.T) {
	te := testCreateBasicResource()

	err := AppendUniqueLink(te, "resource.network.b")
	require.NoError(t, err)
	err = AppendUniqueLink(te, "resource.network.b")
	require.NoError(t, err)

	meta, err := GetMeta(te)
	require.NoError(t, err)

	require.Equal(t, []string{"resource.network.b"}, meta.Links)
}

func TestAppendUniqueLinkLeavesDependsOnUntouched(t *testing.T) {
	te := testCreateBasicResource()

	err := AppendUniqueLink(te, "resource.network.b")
	require.NoError(t, err)

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	require.Equal(t, []string{"dependency1", "dependency2"}, deps)
}

func TestCanGetDependenciesOnExtendedResource(t *testing.T) {
	te := testCreateExtendedResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	require.Len(t, deps, 2)
	require.Contains(t, deps, "dependency1")
	require.Contains(t, deps, "dependency2")
}

func TestCanSetDependenciesOnExtendedResource(t *testing.T) {
	te := testCreateBasicResource()

	deps, err := GetDependencies(te)
	require.NoError(t, err)

	deps = append(deps, "dependency3")

	err = SetDependencies(te, deps)
	require.NoError(t, err)

	newDeps, err := GetDependencies(te)
	require.NoError(t, err)
	require.Len(t, newDeps, 3)
	require.Contains(t, newDeps, "dependency1")
	require.Contains(t, newDeps, "dependency2")
	require.Contains(t, newDeps, "dependency3")
}

func TestCanGetDisabledOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	d, err := GetDisabled(te)
	require.NoError(t, err)

	require.True(t, d)
}

func TestCanSetDisabledOnBasicResource(t *testing.T) {
	te := testCreateBasicResource()

	err := SetDisabled(te, false)
	require.NoError(t, err)

	d, err := GetDisabled(te)
	require.NoError(t, err)
	require.False(t, d)
}

func TestCanGetDisabledOnExtendedResource(t *testing.T) {
	te := testCreateBasicResource()

	d, err := GetDisabled(te)
	require.NoError(t, err)

	require.True(t, d)
}

func TestCanSetDisabledOnExtendedResource(t *testing.T) {
	te := testCreateBasicResource()

	err := SetDisabled(te, false)
	require.NoError(t, err)

	d, err := GetDisabled(te)
	require.NoError(t, err)
	require.False(t, d)
}

// AddressType returns the segment an entity is reached by in its address: the
// variety for a resource kind entity, and the stanza kind for one declared
// with a single label, which has no variety of its own.

func TestAddressTypeReturnsTheVarietyOfAResource(t *testing.T) {
	meta := Meta{
		Name:    "mine",
		Type:    TypeResource,
		Subtype: "container",
	}

	require.Equal(t, "container", meta.AddressType())
}

func TestAddressTypeReturnsTheKindOfASingleLabelStanza(t *testing.T) {
	meta := Meta{
		Name: "cpu_resources",
		Type: "variable",
	}

	require.Empty(t, meta.Subtype)
	require.Equal(t, "variable", meta.AddressType())
}
