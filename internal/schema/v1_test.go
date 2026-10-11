package schema

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// buildV1 builds the Go type of a schema v1 document, failing the test when
// the document is rejected.
func buildV1(t *testing.T, schemaJSON string) reflect.Type {
	t.Helper()

	built, parsed, err := BuildTypeV1([]byte(schemaJSON))
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, parsed)

	return built
}

// fieldByName returns the struct field of built called name, failing the test
// when there is no such field.
func fieldByName(t *testing.T, built reflect.Type, name string) reflect.StructField {
	t.Helper()

	field, ok := built.FieldByName(name)
	require.True(t, ok, "built type has no field %s", name)

	return field
}

func TestBuildTypeV1EmbedsResourceBaseFirst(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "string"}
		]
	}`)

	require.Equal(t, reflect.Struct, built.Kind())
	require.Equal(t, 2, built.NumField())

	first := built.Field(0)
	require.Equal(t, "ResourceBase", first.Name)
	require.Equal(t, reflect.TypeOf(types.ResourceBase{}), first.Type)
	require.True(t, first.Anonymous)
	require.Equal(t, ",remain", first.Tag.Get("xcl"))
}

func TestBuildTypeV1ReturnsParsedSchema(t *testing.T) {
	_, parsed, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"subtype": "docker",
		"fields": [
			{"name": "image", "type": "string"}
		]
	}`))
	require.NoError(t, err)

	require.Equal(t, 1, parsed.SchemaVersion)
	require.Equal(t, "container", parsed.Type)
	require.Equal(t, "docker", parsed.Subtype)
	require.Len(t, parsed.Fields, 1)
	require.Equal(t, "image", parsed.Fields[0].Name)
}

func TestBuildTypeV1StringAttribute(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "string"}
		]
	}`)

	field := built.Field(1)
	require.Equal(t, "Image", field.Name)
	require.Equal(t, reflect.TypeOf(""), field.Type)
	require.False(t, field.Anonymous)
	require.Equal(t, "image", field.Tag.Get("xcl"))
	require.Equal(t, "image", field.Tag.Get("json"))
	require.Equal(t, "", field.Tag.Get("default"))
}

func TestBuildTypeV1OptionalWithDefault(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "replicas", "type": "number", "optional": true, "default": 2}
		]
	}`)

	field := fieldByName(t, built, "Replicas")
	require.Equal(t, reflect.TypeOf(0), field.Type)
	require.Equal(t, "replicas,optional", field.Tag.Get("xcl"))
	require.Equal(t, "replicas,omitempty", field.Tag.Get("json"))
	require.Equal(t, "2", field.Tag.Get("default"))
}

func TestBuildTypeV1StringDefaultKeepsText(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "network", "type": "string", "optional": true, "default": "bridge"}
		]
	}`)

	field := fieldByName(t, built, "Network")
	require.Equal(t, reflect.TypeOf(""), field.Type)
	require.Equal(t, "network,optional", field.Tag.Get("xcl"))
	require.Equal(t, "network,omitempty", field.Tag.Get("json"))
	require.Equal(t, "bridge", field.Tag.Get("default"))
}

func TestBuildTypeV1OptionalWithoutDefaultHasNoDefaultTag(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "network", "type": "string", "optional": true}
		]
	}`)

	field := fieldByName(t, built, "Network")
	require.Equal(t, "network,optional", field.Tag.Get("xcl"))
	require.Equal(t, "network,omitempty", field.Tag.Get("json"))

	_, hasDefault := field.Tag.Lookup("default")
	require.False(t, hasDefault)
}

func TestBuildTypeV1SensitiveString(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "password", "type": "string", "sensitive": true}
		]
	}`)

	field := fieldByName(t, built, "Password")
	require.Equal(t, reflect.TypeOf(types.Sensitive[string]{}), field.Type)
	require.Equal(t, "password", field.Tag.Get("xcl"))
	require.Equal(t, "password", field.Tag.Get("json"))
}

func TestBuildTypeV1SensitiveNumber(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "pin", "type": "number", "sensitive": true}
		]
	}`)

	field := fieldByName(t, built, "Pin")
	require.Equal(t, reflect.TypeOf(types.Sensitive[int]{}), field.Type)
}

func TestBuildTypeV1SensitiveListOfStrings(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "tokens", "type": {"list": "string"}, "sensitive": true}
		]
	}`)

	field := fieldByName(t, built, "Tokens")
	require.Equal(t, reflect.TypeOf(types.Sensitive[[]string]{}), field.Type)
}

func TestBuildTypeV1SensitiveMapOfStrings(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "env", "type": {"map": "string"}, "sensitive": true}
		]
	}`)

	field := fieldByName(t, built, "Env")
	require.Equal(t, reflect.TypeOf(types.Sensitive[map[string]string]{}), field.Type)
}

func TestBuildTypeV1SensitiveUnsupportedShapeFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "ports", "type": {"list": "number"}, "sensitive": true}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "ports")
	require.Contains(t, err.Error(), "sensitive")
}

func TestBuildTypeV1ComputedIsOptionalComputed(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "endpoint", "type": "string", "computed": true}
		]
	}`)

	field := fieldByName(t, built, "Endpoint")
	require.Equal(t, reflect.TypeOf(""), field.Type)
	require.Equal(t, "endpoint,optional,computed", field.Tag.Get("xcl"))
	require.Equal(t, "endpoint,omitempty", field.Tag.Get("json"))
}

func TestBuildTypeV1NestedBlock(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "health",
				"kind": "block",
				"type": {"object": [
					{"name": "path", "type": "string"},
					{"name": "interval", "type": "number", "optional": true}
				]}
			}
		]
	}`)

	field := fieldByName(t, built, "Health")
	require.Equal(t, reflect.Struct, field.Type.Kind())
	require.Equal(t, "health,block", field.Tag.Get("xcl"))
	require.Equal(t, "health,omitempty", field.Tag.Get("json"))

	// the element has no ResourceBase of its own, only the declared fields
	require.Equal(t, 2, field.Type.NumField())

	path := field.Type.Field(0)
	require.Equal(t, "Path", path.Name)
	require.Equal(t, reflect.TypeOf(""), path.Type)
	require.Equal(t, "path", path.Tag.Get("xcl"))
	require.Equal(t, "path", path.Tag.Get("json"))

	interval := field.Type.Field(1)
	require.Equal(t, "Interval", interval.Name)
	require.Equal(t, reflect.TypeOf(0), interval.Type)
	require.Equal(t, "interval,optional", interval.Tag.Get("xcl"))
	require.Equal(t, "interval,omitempty", interval.Tag.Get("json"))
}

func TestBuildTypeV1OptionalNestedBlockIsPointer(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "health",
				"kind": "block",
				"optional": true,
				"type": {"object": [
					{"name": "path", "type": "string"}
				]}
			}
		]
	}`)

	field := fieldByName(t, built, "Health")
	require.Equal(t, reflect.Pointer, field.Type.Kind())
	require.Equal(t, reflect.Struct, field.Type.Elem().Kind())
	require.Equal(t, "Path", field.Type.Elem().Field(0).Name)
	require.Equal(t, "health,block", field.Tag.Get("xcl"))
	require.Equal(t, "health,omitempty", field.Tag.Get("json"))
}

func TestBuildTypeV1RepeatedBlock(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "port",
				"kind": "block",
				"repeated": true,
				"type": {"object": [
					{"name": "local", "type": "number"},
					{"name": "remote", "type": "number"}
				]}
			}
		]
	}`)

	field := fieldByName(t, built, "Port")
	require.Equal(t, reflect.Slice, field.Type.Kind())
	require.Equal(t, reflect.Struct, field.Type.Elem().Kind())
	require.Equal(t, 2, field.Type.Elem().NumField())
	require.Equal(t, "Local", field.Type.Elem().Field(0).Name)
	require.Equal(t, "Remote", field.Type.Elem().Field(1).Name)
	require.Equal(t, "port,block", field.Tag.Get("xcl"))
	require.Equal(t, "port,omitempty", field.Tag.Get("json"))
}

func TestBuildTypeV1NamedBlockMap(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "volume",
				"kind": "block",
				"type": {"map": {"object": [
					{"name": "source", "type": "string"},
					{"name": "destination", "type": "string"}
				]}}
			}
		]
	}`)

	field := fieldByName(t, built, "Volume")
	require.Equal(t, reflect.Map, field.Type.Kind())
	require.Equal(t, reflect.TypeOf(""), field.Type.Key())
	require.Equal(t, reflect.Struct, field.Type.Elem().Kind())
	require.Equal(t, 2, field.Type.Elem().NumField())
	require.Equal(t, "Source", field.Type.Elem().Field(0).Name)
	require.Equal(t, "Destination", field.Type.Elem().Field(1).Name)
	require.Equal(t, "volume,block", field.Tag.Get("xcl"))
	require.Equal(t, "volume,omitempty", field.Tag.Get("json"))
}

func TestBuildTypeV1LabelAddsLabelField(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "volume",
				"kind": "block",
				"repeated": true,
				"label": "name",
				"type": {"object": [
					{"name": "source", "type": "string"}
				]}
			}
		]
	}`)

	field := fieldByName(t, built, "Volume")
	require.Equal(t, reflect.Slice, field.Type.Kind())

	element := field.Type.Elem()
	require.Equal(t, 2, element.NumField())

	label := element.Field(0)
	require.Equal(t, "Name", label.Name)
	require.Equal(t, reflect.TypeOf(""), label.Type)
	require.Equal(t, "name,label", label.Tag.Get("xcl"))
	require.Equal(t, "name,omitempty", label.Tag.Get("json"))

	require.Equal(t, "Source", element.Field(1).Name)
}

func TestBuildTypeV1LabelOnNamedBlockMap(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "volume",
				"kind": "block",
				"label": "name",
				"type": {"map": {"object": [
					{"name": "source", "type": "string"}
				]}}
			}
		]
	}`)

	field := fieldByName(t, built, "Volume")
	require.Equal(t, reflect.Map, field.Type.Kind())

	label := field.Type.Elem().Field(0)
	require.Equal(t, "Name", label.Name)
	require.Equal(t, "name,label", label.Tag.Get("xcl"))
}

func TestBuildTypeV1FloatFormat(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "cpu", "type": "number", "format": "float"}
		]
	}`)

	field := fieldByName(t, built, "Cpu")
	require.Equal(t, reflect.TypeOf(0.0), field.Type)
}

func TestBuildTypeV1IntFormat(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "memory", "type": "number", "format": "int"}
		]
	}`)

	field := fieldByName(t, built, "Memory")
	require.Equal(t, reflect.TypeOf(0), field.Type)
}

func TestBuildTypeV1BoolAttribute(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "privileged", "type": "bool"}
		]
	}`)

	field := fieldByName(t, built, "Privileged")
	require.Equal(t, reflect.TypeOf(true), field.Type)
}

func TestBuildTypeV1ListAndMapAttributes(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "args", "type": {"list": "string"}},
			{"name": "limits", "type": {"map": "number"}}
		]
	}`)

	args := fieldByName(t, built, "Args")
	require.Equal(t, reflect.TypeOf([]string{}), args.Type)
	require.Equal(t, "args", args.Tag.Get("xcl"))

	limits := fieldByName(t, built, "Limits")
	require.Equal(t, reflect.TypeOf(map[string]int{}), limits.Type)
	require.Equal(t, "limits", limits.Tag.Get("xcl"))
}

func TestBuildTypeV1ListOfFloatsUsesFormat(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "weights", "type": {"list": "number"}, "format": "float"}
		]
	}`)

	field := fieldByName(t, built, "Weights")
	require.Equal(t, reflect.TypeOf([]float64{}), field.Type)
}

func TestBuildTypeV1ObjectAttributeIsNestedStruct(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "resources",
				"type": {"object": [
					{"name": "cpu", "type": "number"},
					{"name": "memory", "type": "string", "optional": true}
				]}
			}
		]
	}`)

	field := fieldByName(t, built, "Resources")
	require.Equal(t, reflect.Struct, field.Type.Kind())

	// an object attribute is an attribute, not a block
	require.Equal(t, "resources", field.Tag.Get("xcl"))
	require.Equal(t, "resources", field.Tag.Get("json"))

	require.Equal(t, 2, field.Type.NumField())

	cpu := field.Type.Field(0)
	require.Equal(t, "Cpu", cpu.Name)
	require.Equal(t, reflect.TypeOf(0), cpu.Type)
	require.Equal(t, "cpu", cpu.Tag.Get("xcl"))

	memory := field.Type.Field(1)
	require.Equal(t, "Memory", memory.Name)
	require.Equal(t, reflect.TypeOf(""), memory.Type)
	require.Equal(t, "memory,optional", memory.Tag.Get("xcl"))
}

func TestBuildTypeV1AnyType(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "extra", "type": "any"}
		]
	}`)

	field := fieldByName(t, built, "Extra")
	require.Equal(t, reflect.TypeOf((*any)(nil)).Elem(), field.Type)
	require.Equal(t, reflect.Interface, field.Type.Kind())
}

func TestBuildTypeV1UnknownTypeFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "text"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "image")
	require.Contains(t, err.Error(), `unknown type "text"`)
}

func TestBuildTypeV1MissingTypeFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "image")
	require.Contains(t, err.Error(), "no type")
}

func TestBuildTypeV1UnknownKindFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "string", "kind": "property"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "image")
	require.Contains(t, err.Error(), `unknown kind "property"`)
}

func TestBuildTypeV1UnknownNumberFormatFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "cpu", "type": "number", "format": "decimal"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "cpu")
	require.Contains(t, err.Error(), `unknown number format "decimal"`)
}

func TestBuildTypeV1ReservedNameFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "meta", "type": "string"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "field meta")
	require.Contains(t, err.Error(), "cannot be declared")
}

func TestBuildTypeV1ReservedNameIsAllowedInsideBlock(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "health",
				"kind": "block",
				"type": {"object": [
					{"name": "meta", "type": "string"}
				]}
			}
		]
	}`)

	field := fieldByName(t, built, "Health")
	require.Equal(t, "Meta", field.Type.Field(0).Name)
}

func TestBuildTypeV1DuplicateGoNameFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "my_name", "type": "string"},
			{"name": "my__name", "type": "string"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "my__name")
	require.Contains(t, err.Error(), "MyName")
}

func TestBuildTypeV1DefaultOnRequiredFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "replicas", "type": "number", "default": 2}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "replicas")
	require.Contains(t, err.Error(), "only an optional attribute has a default")
}

func TestBuildTypeV1DefaultOnComputedFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "endpoint", "type": "string", "computed": true, "default": "localhost"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
	require.Contains(t, err.Error(), "computed")
}

func TestBuildTypeV1DefaultOfWrongTypeFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "replicas", "type": "number", "optional": true, "default": true}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "replicas")
	require.Contains(t, err.Error(), "is not a int")
}

func TestBuildTypeV1SensitiveBlockFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "auth",
				"kind": "block",
				"sensitive": true,
				"type": {"object": [
					{"name": "token", "type": "string"}
				]}
			}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "auth")
	require.Contains(t, err.Error(), "a block cannot be sensitive")
}

func TestBuildTypeV1RepeatedNamedBlockMapFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "volume",
				"kind": "block",
				"repeated": true,
				"type": {"map": {"object": [
					{"name": "source", "type": "string"}
				]}}
			}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "volume")
	require.Contains(t, err.Error(), "cannot be repeated")
}

func TestBuildTypeV1BlockWithScalarTypeFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "health", "kind": "block", "type": "string"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "health")
	require.Contains(t, err.Error(), "a block's type must be an object or a map of objects")
}

func TestBuildTypeV1NestedFieldErrorNamesPath(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{
				"name": "health",
				"kind": "block",
				"type": {"object": [
					{"name": "path", "type": "url"}
				]}
			}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "field health.path")
}

func TestBuildTypeV1WrongVersionFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 2,
		"type": "container",
		"fields": []
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported schema_version 2")
}

func TestBuildTypeV1NoTypeFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"fields": []
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "names no type")
}

func TestBuildTypeV1InvalidJSONFails(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{"schema_version": 1,`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "not valid JSON")
}

func TestBuildTypeV1ErrorNamesSchemaType(t *testing.T) {
	_, _, err := BuildTypeV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"subtype": "docker",
		"fields": [
			{"name": "image", "type": "text"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "schema for type container.docker")
}

func TestBuildTypeV1CamelCasesFieldNames(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "database",
		"fields": [
			{"name": "connection_string", "type": "string"},
			{"name": "max_idle_connections", "type": "number"}
		]
	}`)

	connection := fieldByName(t, built, "ConnectionString")
	require.Equal(t, "connection_string", connection.Tag.Get("xcl"))
	require.Equal(t, "connection_string", connection.Tag.Get("json"))

	maxIdle := fieldByName(t, built, "MaxIdleConnections")
	require.Equal(t, "max_idle_connections", maxIdle.Tag.Get("xcl"))
	require.Equal(t, "max_idle_connections", maxIdle.Tag.Get("json"))
}

func TestBuildTypeV1EmptyFieldsBuildsResourceBaseOnly(t *testing.T) {
	built := buildV1(t, `{
		"schema_version": 1,
		"type": "marker",
		"fields": []
	}`)

	require.Equal(t, 1, built.NumField())
	require.Equal(t, "ResourceBase", built.Field(0).Name)
}

func TestIsV1RecognisesVersionOne(t *testing.T) {
	require.True(t, IsV1([]byte(`{"schema_version": 1, "type": "container", "fields": []}`)))
}

func TestIsV1RejectsLegacySchema(t *testing.T) {
	type holder struct {
		types.ResourceBase `xcl:",remain"`

		Image string `xcl:"image"`
	}

	data, err := GenerateSchemaFromInstance(holder{}, 10)
	require.NoError(t, err)

	require.False(t, IsV1(data))
}

func TestIsV1RejectsOtherVersion(t *testing.T) {
	require.False(t, IsV1([]byte(`{"schema_version": 2, "type": "container", "fields": []}`)))
}

func TestIsV1RejectsInvalidJSON(t *testing.T) {
	require.False(t, IsV1([]byte(`{"schema_version": 1`)))
}

func TestParseV1ReadsDocument(t *testing.T) {
	parsed, err := ParseV1([]byte(`{
		"schema_version": 1,
		"type": "container",
		"subtype": "docker",
		"fields": [
			{"name": "image", "type": "string", "optional": true, "sensitive": true, "computed": true}
		]
	}`))
	require.NoError(t, err)

	require.Equal(t, "container", parsed.Type)
	require.Equal(t, "docker", parsed.Subtype)
	require.Len(t, parsed.Fields, 1)
	require.Equal(t, "image", parsed.Fields[0].Name)
	require.True(t, parsed.Fields[0].Optional)
	require.True(t, parsed.Fields[0].Sensitive)
	require.True(t, parsed.Fields[0].Computed)
}

func TestParseV1WrongVersionFails(t *testing.T) {
	_, err := ParseV1([]byte(`{"schema_version": 3, "type": "container", "fields": []}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported schema_version 3")
}

func TestCreateInstanceFromSchemaRoutesV1(t *testing.T) {
	instance, err := CreateInstanceFromSchema([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "string"}
		]
	}`), KnownTypes())
	require.NoError(t, err)
	require.NotNil(t, instance)

	instanceType := reflect.TypeOf(instance)
	require.Equal(t, reflect.Pointer, instanceType.Kind())
	require.Equal(t, reflect.Struct, instanceType.Elem().Kind())

	first := instanceType.Elem().Field(0)
	require.Equal(t, "ResourceBase", first.Name)
	require.True(t, first.Anonymous)
	require.Equal(t, reflect.TypeOf(types.ResourceBase{}), first.Type)

	image := fieldByName(t, instanceType.Elem(), "Image")
	require.Equal(t, reflect.TypeOf(""), image.Type)
	require.Equal(t, "image", image.Tag.Get("xcl"))

	// the instance is usable as a resource through the embedded ResourceBase
	_, err = types.GetMeta(instance)
	require.NoError(t, err)
}

func TestCreateInstanceFromSchemaV1InvalidFails(t *testing.T) {
	_, err := CreateInstanceFromSchema([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "text"}
		]
	}`), KnownTypes())

	require.Error(t, err)
	require.Contains(t, err.Error(), "image")
}

func TestCreateInstanceFromSchemaKeepsLegacy(t *testing.T) {
	type holder struct {
		types.ResourceBase `xcl:",remain"`

		Image string `xcl:"image" json:"image"`
	}

	data, err := GenerateSchemaFromInstance(holder{}, 10)
	require.NoError(t, err)

	rebuilt, err := CreateInstanceFromSchema(data, KnownTypes())
	require.NoError(t, err)

	rebuiltType := reflect.TypeOf(rebuilt)
	require.Equal(t, reflect.Pointer, rebuiltType.Kind())
	require.Equal(t, reflect.Struct, rebuiltType.Elem().Kind())

	first := rebuiltType.Elem().Field(0)
	require.Equal(t, "ResourceBase", first.Name)
	require.True(t, first.Anonymous)

	image := fieldByName(t, rebuiltType.Elem(), "Image")
	require.Equal(t, reflect.TypeOf(""), image.Type)
	require.Equal(t, "image", image.Tag.Get("xcl"))
}
