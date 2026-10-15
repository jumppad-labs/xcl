package schema_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugins/plain/plainplugin"
	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// hostTypeOf describes t as a plugin does and builds the type the host
// decodes configuration into from the description
func hostTypeOf(t *testing.T, of reflect.Type) (reflect.Type, schema.SchemaV1) {
	t.Helper()

	data, err := schema.GenerateV1(of, "resource", "fixture", false)
	require.NoError(t, err)

	var doc schema.SchemaV1
	require.NoError(t, json.Unmarshal(data, &doc))

	instance, err := schema.CreateInstanceFromSchema(data)
	require.NoError(t, err)

	return reflect.TypeOf(instance).Elem(), doc
}

// carry sends value to the host as the plugin writes it, has the host write
// it back as it holds it, and decodes that into a new value of value's type,
// as the plugin reads what the host sends
func carry[T any](t *testing.T, value *T) *T {
	t.Helper()

	host, _ := hostTypeOf(t, reflect.TypeFor[T]())

	sent, err := wire.Marshal(value)
	require.NoError(t, err)

	held := reflect.New(host).Interface()
	require.NoError(t, json.Unmarshal(sent, held))

	returned, err := wire.Marshal(held)
	require.NoError(t, err)

	back := new(T)
	require.NoError(t, json.Unmarshal(returned, back))

	return back
}

func TestGenerateV1CarriesAContainerThroughTheHostUnchanged(t *testing.T) {
	container := &structs.Container{
		ContainerBase: structs.ContainerBase{
			ResourceBase: types.ResourceBase{
				Meta:      types.Meta{ID: "resource.container.web", Name: "web", Type: "resource", Subtype: "container"},
				DependsOn: []string{"resource.network.main"},
			},
			Default:    "hello",
			Networks:   []structs.NetworkAttachment{{ID: 1, Name: "main", Aliases: []string{"web"}, AssignedAddress: "10.0.0.2"}},
			NetworkObj: structs.Network{ResourceBase: types.ResourceBase{Meta: types.Meta{ID: "resource.network.main", Name: "main"}}, Subnet: "10.0.0.0/16", ProviderID: "net-1"},
			Build:      &structs.Build{Context: "./app", Tag: "v1", ImageID: "sha256:abc"},
			Entrypoint: []string{"/bin/sh"},
			Env:        map[string]string{"MODE": "test"},
			Volumes:    []structs.Volume{{Source: "/data", Destination: "/var/data", ReadOnly: true}},
			Ports:      []structs.Port{{Local: 8080, Remote: 80}},
			Resources:  &structs.Resources{CPU: 1000, CPUPin: []int{0, 1}, Memory: 512},
			RunAs:      &structs.User{User: "app", Group: "app"},

			CreatedNetworksMap: map[string]structs.Network{"main": {Subnet: "10.0.0.0/16", Observed: "up"}},
		},
		ContainerID: "c-1",
	}

	require.Equal(t, container, carry(t, container))
}

func TestGenerateV1DescribesAKeyAComputedFieldAndADefault(t *testing.T) {
	_, doc := hostTypeOf(t, reflect.TypeFor[structs.Container]())

	fields := map[string]schema.FieldV1{}
	for _, f := range doc.Fields {
		fields[f.Name] = f
	}

	require.Equal(t, `"hello world"`, string(fields["default"].Default))

	var network struct {
		Object []schema.FieldV1 `json:"object"`
	}
	require.NoError(t, json.Unmarshal(fields["network"].Type, &network))
	require.True(t, fields["network"].Repeated)

	attachment := map[string]schema.FieldV1{}
	for _, f := range network.Object {
		attachment[f.Name] = f
	}

	require.True(t, attachment["id"].Key)
	require.True(t, attachment["assigned_address"].Computed)
}

func TestGenerateV1DescribesAReferencedEntityWithItsMeta(t *testing.T) {
	host, _ := hostTypeOf(t, reflect.TypeFor[structs.Container]())

	field, ok := host.FieldByName("Networkobj")
	require.True(t, ok)

	_, hasMeta := field.Type.FieldByName("Meta")
	require.True(t, hasMeta)
}

func TestGenerateV1DescribesColumnsAndRowsThatHoldEachOtherOnce(t *testing.T) {
	_, doc := hostTypeOf(t, reflect.TypeFor[plainplugin.Layout]())

	require.Len(t, doc.Definitions, 1)
	require.Contains(t, doc.Definitions, "column")

	var column struct {
		Ref string `json:"ref"`
	}
	require.NoError(t, json.Unmarshal(doc.Fields[0].Type, &column))
	require.Equal(t, "column", column.Ref)
}

func TestBuildTypeV1NestsBlocksThatHoldThemselvesAsDeepAsConfigurationMay(t *testing.T) {
	host, _ := hostTypeOf(t, reflect.TypeFor[plainplugin.Layout]())

	// column, row, column, … each a list of the next, until a level holds
	// no further block
	depth := 0
	element := host
	for {
		var next reflect.StructField
		found := false
		for _, name := range []string{"Column", "Row"} {
			if f, ok := element.FieldByName(name); ok {
				next, found = f, true
			}
		}

		if !found {
			break
		}

		depth++
		element = next.Type.Elem()
	}

	require.Equal(t, schema.MaxNestingDepth, depth)
}

func TestGenerateV1CarriesANestedLayoutThroughTheHostUnchanged(t *testing.T) {
	// the label travels in the entity's meta, so the plain type's label field
	// is no part of what the schema describes
	layout := &plainplugin.Layout{
		Columns: []plainplugin.Column{{
			Width: "60",
			Rows: []plainplugin.Row{{
				Height:  "50",
				Columns: []plainplugin.Column{{Width: "30"}, {Width: "70", Rows: []plainplugin.Row{{Height: "100"}}}},
			}},
		}},
	}

	require.Equal(t, layout, carry(t, layout))
}

func TestGenerateV1NamesTwoTypesThatHoldThemselvesUnderOneBlockApart(t *testing.T) {
	type leaf struct {
		Value string `xcl:"value" json:"value"`
		Nodes []leaf `xcl:"node,block" json:"node,omitempty"`
	}

	type branch struct {
		Value string   `xcl:"value" json:"value"`
		Nodes []branch `xcl:"node,block" json:"node,omitempty"`
	}

	type holder struct {
		Leaves []leaf `xcl:"node,block" json:"node,omitempty"`
		Group  *struct {
			Branches []branch `xcl:"node,block" json:"node,omitempty"`
		} `xcl:"group,block" json:"group,omitempty"`
	}

	_, doc := hostTypeOf(t, reflect.TypeFor[holder]())

	require.Len(t, doc.Definitions, 2)
	require.Contains(t, doc.Definitions, "node")
	require.Contains(t, doc.Definitions, "node_2")
}

func TestGenerateV1RefusesAValueThatHoldsItself(t *testing.T) {
	type tree struct {
		Name     string `xcl:"name" json:"name"`
		Children []tree `xcl:"children,optional" json:"children,omitempty"`
	}

	type holder struct {
		Tree tree `xcl:"tree" json:"tree"`
	}

	_, err := schema.GenerateV1(reflect.TypeFor[holder](), "resource", "fixture", false)
	require.ErrorContains(t, err, "holds itself, which only a block can")
}

func TestGenerateV1RefusesABlockHoldingItsOwnBlockType(t *testing.T) {
	type outer struct {
		Inner []outer `xcl:"inner,block" json:"inner,omitempty"`
	}

	_, err := schema.GenerateV1(reflect.TypeFor[outer](), "resource", "fixture", false)
	require.ErrorContains(t, err, "only a block's own blocks can hold themselves")
}

func TestGenerateV1RefusesAComputedFieldThatIsNotOptional(t *testing.T) {
	type holder struct {
		ID string `xcl:"id,computed" json:"id"`
	}

	_, err := schema.GenerateV1(reflect.TypeFor[holder](), "resource", "fixture", false)
	require.ErrorContains(t, err, "field ID: a computed field must also be optional")
}

func TestGenerateV1RefusesAJSONNameOtherThanTheXCLName(t *testing.T) {
	type holder struct {
		Ports []int `xcl:"port,optional" json:"ports"`
	}

	_, err := schema.GenerateV1(reflect.TypeFor[holder](), "resource", "fixture", false)
	require.ErrorContains(t, err, `field Ports: its json name "ports" is not its xcl name "port"`)
}

func TestGenerateV1RefusesAFieldWithoutAnXCLTag(t *testing.T) {
	type holder struct {
		State string `json:"state"`
	}

	_, err := schema.GenerateV1(reflect.TypeFor[holder](), "resource", "fixture", false)
	require.ErrorContains(t, err, "field State: has no xcl tag")
}

func TestGenerateV1MarksASingleton(t *testing.T) {
	data, err := schema.GenerateV1(reflect.TypeFor[plainplugin.Hub](), "hub", "", true)
	require.NoError(t, err)

	parsed, err := schema.ParseV1(data)
	require.NoError(t, err)
	require.True(t, parsed.Singleton)
}

func TestBuildTypeV1RefusesAReferenceToNoDefinition(t *testing.T) {
	_, _, err := schema.BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "layout",
		"fields": [{"name": "column", "kind": "block", "repeated": true, "type": {"ref": "column"}}]
	}`))

	require.ErrorContains(t, err, `field column: refers to "column", which the schema does not define`)
}
