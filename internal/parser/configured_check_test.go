package parser

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

var (
	networkType   = reflect.TypeOf(&structs.Network{})
	containerType = reflect.TypeOf(&structs.Container{})
)

// parseResourceBody parses src as the body of a resource block
func parseResourceBody(t *testing.T, src string) *hclsyntax.Body {
	t.Helper()

	file, diags := hclsyntax.ParseConfig([]byte(src), "resource.xcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), diags.Error())

	body, ok := file.Body.(*hclsyntax.Body)
	require.True(t, ok)

	return body
}

// toJSON serializes a resource the way the lifecycle does before and after a
// provider call
func toJSON(t *testing.T, resource any) []byte {
	t.Helper()

	data, err := json.Marshal(resource)
	require.NoError(t, err)

	return data
}

func TestChangedConfiguredValuesReportsChangedLiteralField(t *testing.T) {
	body := parseResourceBody(t, `subnet = "10.0.0.0/16"`)

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	after := &structs.Network{Subnet: "10.9.0.0/16"}

	changed := changedConfiguredValues(body, networkType, toJSON(t, before), toJSON(t, after))

	require.Equal(t, []string{"subnet"}, changed)
}

func TestChangedConfiguredValuesIgnoresComputedField(t *testing.T) {
	body := parseResourceBody(t, `subnet = "10.0.0.0/16"`)

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	after := &structs.Network{Subnet: "10.0.0.0/16", ProviderID: "id-one"}

	changed := changedConfiguredValues(body, networkType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestChangedConfiguredValuesIgnoresReferenceSetField(t *testing.T) {
	body := parseResourceBody(t, `subnet = resource.network.a.subnet`)

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	after := &structs.Network{Subnet: "10.9.0.0/16"}

	changed := changedConfiguredValues(body, networkType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestChangedConfiguredValuesIgnoresModuleReferenceSetField(t *testing.T) {
	body := parseResourceBody(t, `subnet = module.networking.output.subnet`)

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	after := &structs.Network{Subnet: "10.9.0.0/16"}

	changed := changedConfiguredValues(body, networkType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestChangedConfiguredValuesReportsNestedBlockField(t *testing.T) {
	body := parseResourceBody(t, `
network {
  name       = "n1"
  ip_address = "10.0.0.10"
}
`)

	before := &structs.Container{}
	before.Networks = []structs.NetworkAttachment{{Name: "n1", IPAddress: "10.0.0.10"}}

	after := &structs.Container{}
	after.Networks = []structs.NetworkAttachment{{Name: "n1", IPAddress: "10.0.0.99"}}

	changed := changedConfiguredValues(body, containerType, toJSON(t, before), toJSON(t, after))

	require.Equal(t, []string{"network[0].ip_address"}, changed)
}

// A provider changing a nested configured value is covered here rather than in
// a lifecycle scenario: the test plugin can only change a network's subnet.
func TestChangedConfiguredValuesReportsNestedBlockFieldOfSecondElement(t *testing.T) {
	body := parseResourceBody(t, `
network {
  id   = 1
  name = "n1"
}

network {
  id         = 2
  name       = "n2"
  ip_address = "10.0.0.20"
}
`)

	before := &structs.Container{}
	before.Networks = []structs.NetworkAttachment{
		{ID: 1, Name: "n1"},
		{ID: 2, Name: "n2", IPAddress: "10.0.0.20"},
	}

	after := &structs.Container{}
	after.Networks = []structs.NetworkAttachment{
		{ID: 1, Name: "n1"},
		{ID: 2, Name: "n2", IPAddress: "10.0.0.99"},
	}

	changed := changedConfiguredValues(body, containerType, toJSON(t, before), toJSON(t, after))

	require.Equal(t, []string{"network[1].ip_address"}, changed)
}

func TestChangedConfiguredValuesIgnoresNestedComputedField(t *testing.T) {
	body := parseResourceBody(t, `
network {
  name = "n1"
}
`)

	before := &structs.Container{}
	before.Networks = []structs.NetworkAttachment{{Name: "n1"}}

	after := &structs.Container{}
	after.Networks = []structs.NetworkAttachment{{Name: "n1", AssignedAddress: "assigned-n1"}}

	changed := changedConfiguredValues(body, containerType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestChangedConfiguredValuesIgnoresNestedReferenceSetField(t *testing.T) {
	body := parseResourceBody(t, `
network {
  name = resource.network.first.meta.name
}
`)

	before := &structs.Container{}
	before.Networks = []structs.NetworkAttachment{{Name: "first"}}

	after := &structs.Container{}
	after.Networks = []structs.NetworkAttachment{{Name: "renamed"}}

	changed := changedConfiguredValues(body, containerType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestChangedConfiguredValuesReportsAddedListElementOnce(t *testing.T) {
	body := parseResourceBody(t, `
network {
  id   = 1
  name = "n1"
}
`)

	before := &structs.Container{}
	before.Networks = []structs.NetworkAttachment{{ID: 1, Name: "n1"}}

	after := &structs.Container{}
	after.Networks = []structs.NetworkAttachment{
		{ID: 1, Name: "n1"},
		{ID: 2, Name: "added-by-provider"},
	}

	changed := changedConfiguredValues(body, containerType, toJSON(t, before), toJSON(t, after))

	require.Equal(t, []string{"network"}, changed)
}

func TestChangedConfiguredValuesReportsNothingWhenUnchanged(t *testing.T) {
	body := parseResourceBody(t, `
network {
  id         = 1
  name       = "n1"
  ip_address = "10.0.0.10"
}

env = {
  MODE = "test"
}
`)

	before := &structs.Container{}
	before.Networks = []structs.NetworkAttachment{{ID: 1, Name: "n1", IPAddress: "10.0.0.10"}}
	before.Env = map[string]string{"MODE": "test"}

	after := &structs.Container{}
	after.Networks = []structs.NetworkAttachment{{ID: 1, Name: "n1", IPAddress: "10.0.0.10"}}
	after.Env = map[string]string{"MODE": "test"}

	changed := changedConfiguredValues(body, containerType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestChangedConfiguredValuesIgnoresMetadata(t *testing.T) {
	body := parseResourceBody(t, `subnet = "10.0.0.0/16"`)

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	before.Meta = types.Meta{ID: "resource.network.one", File: "one.xcl", Line: 1}

	after := &structs.Network{Subnet: "10.0.0.0/16"}
	after.Meta = types.Meta{ID: "resource.network.one", File: "other.xcl", Line: 7, Status: types.StatusCreated}

	changed := changedConfiguredValues(body, networkType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, changed)
}

func TestWarnChangedConfiguredValuesEmitsWarnLogEventNamingTheField(t *testing.T) {
	body := parseResourceBody(t, `subnet = "10.0.0.0/16"`)
	collector := &eventCollector{}

	base := events.Event{
		Source:       events.SourceCore,
		Operation:    events.OperationCreate,
		ResourceType: "network.one",
		ResourceID:   "resource.network.one",
		File:         "one.xcl",
	}
	log := logger.New(collector.collect, base)

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	after := &structs.Network{Subnet: "10.9.0.0/16"}

	warnChangedConfiguredValues(log, "resource.network.one", body, networkType, toJSON(t, before), toJSON(t, after))

	recorded := collector.all()
	require.Len(t, recorded, 1)

	warning := recorded[0]
	require.Equal(t, events.SourceCore, warning.Source)
	require.Equal(t, events.OperationCreate, warning.Operation)
	require.Equal(t, events.PhaseLog, warning.Phase)
	require.Equal(t, "network.one", warning.ResourceType)
	require.Equal(t, "resource.network.one", warning.ResourceID)
	require.Equal(t, "one.xcl", warning.File)
	require.Equal(t, map[string]any{
		events.KeyLevel:   events.LevelWarn,
		events.KeyMessage: "provider changed a configured value",
		"field":           "subnet",
	}, warning.Meta)
}

func TestWarnChangedConfiguredValuesEmitsNothingWhenUnchanged(t *testing.T) {
	body := parseResourceBody(t, `subnet = "10.0.0.0/16"`)
	collector := &eventCollector{}
	log := logger.New(collector.collect, events.Event{ResourceID: "resource.network.one"})

	before := &structs.Network{Subnet: "10.0.0.0/16"}
	after := &structs.Network{Subnet: "10.0.0.0/16", ProviderID: "id-one"}

	warnChangedConfiguredValues(log, "resource.network.one", body, networkType, toJSON(t, before), toJSON(t, after))

	require.Empty(t, collector.all())
}
