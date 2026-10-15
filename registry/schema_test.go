package registry

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func TestTypeFromSchemaReturnsDeclaredType(t *testing.T) {
	declared, err := TypeFromSchema([]byte(`{
		"schema_version": 1,
		"type": "container",
		"subtype": "docker",
		"fields": [
			{"name": "image", "type": "string"},
			{"name": "replicas", "type": "number", "optional": true, "default": 1}
		]
	}`))
	require.NoError(t, err)

	require.Equal(t, "container", declared.Type)
	require.Equal(t, "docker", declared.Subtype)
	require.NotNil(t, declared.Prototype)

	prototypeType := reflect.TypeOf(declared.Prototype)
	require.Equal(t, reflect.Pointer, prototypeType.Kind())
	require.False(t, reflect.ValueOf(declared.Prototype).IsNil())
	require.Equal(t, reflect.Struct, prototypeType.Elem().Kind())

	// a plain type, named because it reads its label from its Meta field
	meta := prototypeType.Elem().Field(0)
	require.Equal(t, "Meta", meta.Name)
	require.False(t, meta.Anonymous)
	require.Equal(t, reflect.TypeOf(types.Meta{}), meta.Type)

	image, ok := prototypeType.Elem().FieldByName("Image")
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(""), image.Type)
	require.Equal(t, "image", image.Tag.Get("xcl"))

	replicas, ok := prototypeType.Elem().FieldByName("Replicas")
	require.True(t, ok)
	require.Equal(t, reflect.TypeOf(0), replicas.Type)
	require.Equal(t, "replicas,optional", replicas.Tag.Get("xcl"))
	require.Equal(t, "1", replicas.Tag.Get("default"))

	// the prototype registers exactly as a hand-written type does
	local := NewLocal()
	require.NotPanics(t, func() {
		local.RegisterType(declared.Prototype, declared.Type, declared.Subtype)
	})

	registered := local.Types()
	require.Len(t, registered, 1)
	require.Equal(t, "container", registered[0].Type)
	require.Equal(t, "docker", registered[0].Subtype)
	require.Equal(t, prototypeType, reflect.TypeOf(registered[0].Prototype))
}

func TestTypeFromSchemaWithoutSubtype(t *testing.T) {
	declared, err := TypeFromSchema([]byte(`{
		"schema_version": 1,
		"type": "network",
		"fields": [
			{"name": "subnet", "type": "string"}
		]
	}`))
	require.NoError(t, err)

	require.Equal(t, "network", declared.Type)
	require.Equal(t, "", declared.Subtype)
	require.NotNil(t, declared.Prototype)

	local := NewLocal()
	require.NotPanics(t, func() {
		local.RegisterType(declared.Prototype, declared.Type)
	})

	registered := local.Types()
	require.Len(t, registered, 1)
	require.Equal(t, "network", registered[0].Type)
	require.Equal(t, "", registered[0].Subtype)
}

func TestTypeFromSchemaInvalidFails(t *testing.T) {
	declared, err := TypeFromSchema([]byte(`{
		"schema_version": 1,
		"type": "container",
		"fields": [
			{"name": "image", "type": "text"}
		]
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "schema: ")
	require.Contains(t, err.Error(), "image")
	require.Equal(t, Type{}, declared)
}

func TestTypeFromSchemaWrongVersionFails(t *testing.T) {
	_, err := TypeFromSchema([]byte(`{
		"schema_version": 2,
		"type": "container",
		"fields": []
	}`))

	require.Error(t, err)
	require.Contains(t, err.Error(), "schema: ")
	require.Contains(t, err.Error(), "unsupported schema_version 2")
}
