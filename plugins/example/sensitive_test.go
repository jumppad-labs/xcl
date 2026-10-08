package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/catalog"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	plugintesting "github.com/jumppad-labs/xcl/plugins/testing"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

const (
	tokenConfig   = "./testdata/people_token.xcl"
	tokenPersonID = "resource.person.token_person"
)

// applyTokenPerson applies the token fixture with a fresh parser sharing the
// catalog and store, saves the state the way Config does and returns the
// events the apply fired
func applyTokenPerson(t *testing.T, cat *catalog.Catalog, store *state.FileStateStore, level events.DataLevel) ([]any, *applyEventCollector) {
	t.Helper()

	collector := &applyEventCollector{}

	options := parser.DefaultOptions()
	options.ModuleCache = filepath.Join(t.TempDir(), parser.ConfigDirectory, "cache")
	options.Catalog = cat
	options.StateStore = store
	options.Emit = collector.Record
	options.EventData = level
	options.EventMask = mask.Redact()

	p := parser.NewParser(options)

	st, err := p.Apply(context.Background(), tokenConfig)
	require.NoError(t, err)
	require.NotNil(t, st)

	encoded, _, err := parser.EncodeForState(st.GetResources(), nil)
	require.NoError(t, err)
	require.NoError(t, store.Save(encoded))

	return st.GetResources(), collector
}

// eventData returns the data of the one success event for the person and
// operation
func (c *applyEventCollector) eventData(t *testing.T, operation string) []byte {
	t.Helper()

	var data []byte
	count := 0
	for _, event := range c.Events() {
		if event.ResourceID == tokenPersonID && event.Operation == operation && event.Phase == events.PhaseSuccess {
			data = event.Data
			count++
		}
	}
	require.Equal(t, 1, count, "expected exactly one %s success event", operation)

	return data
}

func TestExamplePersonTokenRevealsAfterApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	resources, _ := applyTokenPerson(t, newPersonCatalog(), store, events.DataNone)

	require.Equal(t, "tok-s3cret", findPerson(t, resources, tokenPersonID).Token.Reveal())
}

func TestExamplePersonTokenIsRealInStateFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	applyTokenPerson(t, newPersonCatalog(), store, events.DataNone)

	contents, err := os.ReadFile(store.Path())
	require.NoError(t, err)

	require.Contains(t, string(contents), "tok-s3cret")
	require.NotContains(t, string(contents), "(sensitive)")
}

func TestExamplePersonTokenRevealsInDecodedState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	cat := newPersonCatalog()

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	applyTokenPerson(t, cat, store, events.DataNone)

	reloaded, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	records, err := reloaded.Load()
	require.NoError(t, err)

	saved, err := savedentity.DecodeAll(cat, records, savedentity.ReadOptions{})
	require.NoError(t, err)

	require.Equal(t, "tok-s3cret", findPerson(t, saved, tokenPersonID).Token.Reveal())
}

// The second apply uses a new parser and registry, so the real token it sees
// in the carried over state and the saved file came from the first apply.
func TestExamplePersonTokenRevealsAfterSecondApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	firstStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)
	applyTokenPerson(t, newPersonCatalog(), firstStore, events.DataNone)

	secondStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	resources, second := applyTokenPerson(t, newPersonCatalog(), secondStore, events.DataNone)

	// the person was only read, so it came from state and the configuration
	require.Empty(t, second.successfulResources("create"))
	require.Equal(t, []string{tokenPersonID}, second.successfulResources("read"))
	require.Equal(t, "person-token-holder", findPerson(t, resources, tokenPersonID).PersonID)
	require.Equal(t, "tok-s3cret", findPerson(t, resources, tokenPersonID).Token.Reveal())

	contents, err := os.ReadFile(secondStore.Path())
	require.NoError(t, err)
	require.Contains(t, string(contents), "tok-s3cret")
	require.NotContains(t, string(contents), "(sensitive)")
}

func TestExamplePersonProcessedEventDataShowsOnlyTheMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	_, collector := applyTokenPerson(t, newPersonCatalog(), store, events.DataProcessed)

	data := collector.eventData(t, "create")
	require.NotEmpty(t, data)

	require.Contains(t, string(data), `"token":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(data), "tok-s3cret")
}

func TestExamplePersonRawEventDataShowsOnlyTheMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	_, collector := applyTokenPerson(t, newPersonCatalog(), store, events.DataRaw)

	data := collector.eventData(t, "create")
	require.NotEmpty(t, data)

	require.Contains(t, string(data), `"token":{"xcl_masked":"redact","value":"(sensitive)"}`)
	require.NotContains(t, string(data), "tok-s3cret")
}

// An external plugin process receives the real token through Create and Read
// and returns it in the result.
func TestExternalPluginCreateReceivesAndReturnsRealToken(t *testing.T) {
	require.NoError(t, plugintesting.BuildPlugin(t, "."))

	ph := setupExternalPlugin(t)

	data := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, tokenConfig, person.Person{})
	require.Len(t, data, 1)
	require.Contains(t, string(data[0]), `"token":"tok-s3cret"`)

	result, err := ph.Create(context.Background(), "resource", "person", data[0])
	require.NoError(t, err)

	created := person.Person{}
	require.NoError(t, json.Unmarshal(result, &created))

	require.Equal(t, "tok-s3cret", created.Token.Reveal())
	require.Equal(t, "person-token-holder", created.PersonID)
}

func TestExternalPluginReadReceivesAndReturnsRealToken(t *testing.T) {
	require.NoError(t, plugintesting.BuildPlugin(t, "."))

	ph := setupExternalPlugin(t)

	data := plugintesting.ParseHCLWithPluginSchemaToEntityData(t, ph, tokenConfig, person.Person{})
	require.Len(t, data, 1)

	result, err := ph.Read(context.Background(), "resource", "person", data[0], data[0])
	require.NoError(t, err)

	read := person.Person{}
	require.NoError(t, json.Unmarshal(result, &read))

	require.Equal(t, "tok-s3cret", read.Token.Reveal())
}
