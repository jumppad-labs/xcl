package parser

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// testStateContainer stands in for a plugin provided resource, it is declared
// by a resource stanza carrying a variety, i.e. resource "container" "mine"
type testStateContainer struct {
	types.ResourceBase `xcl:",remain"`
}

// newTestStateContainer returns a container entity carrying the metadata the
// parser records for it. State stores entities and resolves no addresses, so
// the id is set here the way a parse sets it, rather than being derived when
// the entity is stored
func newTestStateContainer(t *testing.T, name, module string) *testStateContainer {
	t.Helper()

	r := &testStateContainer{}

	meta, err := types.GetMeta(r)
	require.NoError(t, err)

	meta.Type = types.TypeResource
	meta.Subtype = "container"
	meta.Name = name
	meta.Module = module

	meta.ID = fmt.Sprintf("resource.container.%s", name)
	if module != "" {
		meta.ID = fmt.Sprintf("module.%s.resource.container.%s", module, name)
	}

	return r
}

// testStateWithContainer returns a state holding a single container resource
// that lives in the module parent.sub
func testStateWithContainer(t *testing.T) (*State, *testStateContainer) {
	t.Helper()

	s := NewState()

	r := newTestStateContainer(t, "mine", "parent.sub")

	err := s.AppendResource(r)
	require.NoError(t, err)

	return s, r
}

// entityByID returns the entity the state holds under the given id. Storage
// answers no questions about addresses, so a test that wants one entity back
// scans what is held and compares the id each entity already records
func entityByID(s *State, id string) (any, error) {
	for _, e := range s.GetResources() {
		meta, err := types.GetMeta(e)
		if err != nil {
			continue
		}

		if meta.ID == id {
			return e, nil
		}
	}

	return nil, state.ResourceNotFoundError{Resource: id}
}

func TestAppendResourceStoresTheEntity(t *testing.T) {
	s, want := testStateWithContainer(t)

	require.Equal(t, 1, s.ResourceCount())
	require.Same(t, want, s.GetResources()[0])
}

func TestAppendResourceRejectsAnEntityWithoutResourceBase(t *testing.T) {
	s := NewState()

	err := s.AppendResource(&struct{ Name string }{Name: "mine"})
	require.Error(t, err)
	require.Equal(t, 0, s.ResourceCount())
}

// A duplicate is an entity of the same kind, variety, name and module as one
// already held. It is recognised from the metadata the entity carries, no
// address is resolved to decide it
func TestAppendResourceRejectsADuplicateEntity(t *testing.T) {
	s, _ := testStateWithContainer(t)

	duplicate := newTestStateContainer(t, "mine", "parent.sub")

	err := s.AppendResource(duplicate)
	require.Error(t, err)
	require.IsType(t, state.ResourceExistsError{}, err)
	require.ErrorContains(t, err, "mine")

	require.Equal(t, 1, s.ResourceCount())
}

// The module is part of what makes two entities distinct, so the same variety
// and name declared in two different modules are two entities, not one
func TestAppendResourceAcceptsTheSameVarietyAndNameInDifferentModules(t *testing.T) {
	s := NewState()

	first := newTestStateContainer(t, "mine", "parent.sub")
	second := newTestStateContainer(t, "mine", "other")

	err := s.AppendResource(first)
	require.NoError(t, err)

	err = s.AppendResource(second)
	require.NoError(t, err)

	require.Equal(t, 2, s.ResourceCount())
	require.Same(t, first, s.GetResources()[0])
	require.Same(t, second, s.GetResources()[1])
}

func TestAppendResourceAcceptsTheSameNameInTheRootModuleAndInAModule(t *testing.T) {
	s := NewState()

	root := newTestStateContainer(t, "mine", "")
	nested := newTestStateContainer(t, "mine", "parent.sub")

	err := s.AppendResource(root)
	require.NoError(t, err)

	err = s.AppendResource(nested)
	require.NoError(t, err)

	require.Equal(t, 2, s.ResourceCount())
}

func TestRemoveResourceRemovesTheEntity(t *testing.T) {
	s, stored := testStateWithContainer(t)

	err := s.RemoveResource(stored)
	require.NoError(t, err)

	require.Equal(t, 0, s.ResourceCount())
	require.Empty(t, s.GetResources())
}

// The entity is identified by the metadata it carries, so a separate value
// describing the same entity removes it
func TestRemoveResourceRemovesTheEntityMatchingTheGivenMetadata(t *testing.T) {
	s, _ := testStateWithContainer(t)

	err := s.RemoveResource(newTestStateContainer(t, "mine", "parent.sub"))
	require.NoError(t, err)

	require.Equal(t, 0, s.ResourceCount())
}

func TestRemoveResourceReturnsNotFoundForAnEntityThatIsNotHeld(t *testing.T) {
	s, _ := testStateWithContainer(t)

	err := s.RemoveResource(newTestStateContainer(t, "other", "parent.sub"))
	require.Error(t, err)
	require.IsType(t, state.ResourceNotFoundError{}, err)

	require.Equal(t, 1, s.ResourceCount())
}

func TestResourceCountCountsEveryStoredEntity(t *testing.T) {
	s := NewState()
	require.Equal(t, 0, s.ResourceCount())

	err := s.AppendResource(newTestStateContainer(t, "first", ""))
	require.NoError(t, err)

	err = s.AppendResource(newTestStateContainer(t, "second", ""))
	require.NoError(t, err)

	require.Equal(t, 2, s.ResourceCount())
}

func TestBytesSerialisesTheStoredEntities(t *testing.T) {
	s, _ := testStateWithContainer(t)

	data, err := s.Bytes()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	stored := []testStateContainer{}
	err = json.Unmarshal(data, &stored)
	require.NoError(t, err)

	require.Len(t, stored, 1)
	require.Equal(t, "module.parent.sub.resource.container.mine", stored[0].Meta.ID)
	require.Equal(t, "mine", stored[0].Meta.Name)
	require.Equal(t, "container", stored[0].Meta.Subtype)
	require.Equal(t, "parent.sub", stored[0].Meta.Module)
}
