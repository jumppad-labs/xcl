package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/catalog"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

const (
	peopleConfig  = "./testdata/people.xcl"
	testPersonID  = "resource.person.test_person"
	otherPersonID = "resource.person.other_person"
)

// applyEventCollector gathers the events the parser emits, it adds the apply
// tests' queries to the shared recorder
type applyEventCollector struct {
	testutil.EventRecorder
}

// successfulResources returns the IDs of the resources for which the given
// operation succeeded, leaving out builtin types which are not resources
func (c *applyEventCollector) successfulResources(operation string) []string {
	ids := []string{}
	for _, event := range c.Events() {
		if event.Operation == operation && event.Phase == events.PhaseSuccess && strings.HasPrefix(event.ResourceID, "resource.") {
			ids = append(ids, event.ResourceID)
		}
	}

	return ids
}

// applyPeople applies the people config with a fresh parser that shares the
// catalog and state store, saves the returned state and returns the events
// the apply fired
func applyPeople(t *testing.T, cat *catalog.Catalog, store *state.FileStateStore) ([]any, *applyEventCollector) {
	t.Helper()

	collector := &applyEventCollector{}

	options := parser.DefaultOptions()
	options.ModuleCache = filepath.Join(t.TempDir(), parser.ConfigDirectory, "cache")
	options.Catalog = cat
	options.StateStore = store
	options.Emit = collector.Record

	p := parser.NewParser(options)

	st, err := p.Apply(context.Background(), peopleConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	// save the way Config does: each entity encoded with its real values
	encoded, _, err := parser.EncodeForState(st.GetResources(), nil)
	require.NoError(t, err)

	err = store.Save(encoded)
	require.NoError(t, err)

	return st.GetResources(), collector
}

// findPerson returns the person with the given ID from the given entities.
//
// Storage answers no questions about addresses, so the entities are scanned
// for the one recording the id, which an entity carries as its rendered address
func findPerson(t *testing.T, entities []any, id string) *person.Person {
	t.Helper()

	var resource any
	for _, e := range entities {
		meta, err := types.GetMeta(e)
		require.NoError(t, err)

		if meta.ID == id {
			resource = e
			break
		}
	}
	require.NotNil(t, resource, "state holds no entity with the id %s", id)

	found := &person.Person{}
	err := schema.UnmarshalUntyped(resource, found)
	require.NoError(t, err)

	return found
}

func TestExampleProviderSecondApplyMakesNoCreateOrUpdate(t *testing.T) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	cat := newPersonCatalog()

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	// apply 1: both people are created and given a person id
	_, first := applyPeople(t, cat, store)
	require.ElementsMatch(t, []string{testPersonID, otherPersonID}, first.successfulResources("create"))

	// apply 2: nothing changed, so both people are only read
	st, second := applyPeople(t, cat, store)

	require.Empty(t, second.successfulResources("create"))
	require.Empty(t, second.successfulResources("update"))
	require.ElementsMatch(t, []string{testPersonID, otherPersonID}, second.successfulResources("read"))

	// the example Read adds nothing, the person id was carried over by xcl
	require.Equal(t, "person-test-user", findPerson(t, st, testPersonID).PersonID)
	require.Equal(t, "person-other-person", findPerson(t, st, otherPersonID).PersonID)

	loaded, err := store.Load()
	require.NoError(t, err)

	// the store hands back raw records, typing them needs the catalog
	saved, err := savedentity.DecodeAll(cat, loaded, savedentity.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, "person-test-user", findPerson(t, saved, testPersonID).PersonID)
	require.Equal(t, "person-other-person", findPerson(t, saved, otherPersonID).PersonID)
}

// newPersonCatalog returns a catalog whose local registry holds the person
// plugin, compiled in
func newPersonCatalog() *catalog.Catalog {
	local := registry.NewLocal()
	local.RegisterPlugin(&PersonPlugin{})

	cat := catalog.New()
	cat.AddRegistry(local)

	return cat
}
