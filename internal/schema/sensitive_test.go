package schema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// rebuildFieldType generates a schema from instance and rebuilds a struct from
// it the way the host does, returning the type of the first field.
func rebuildFieldType(t *testing.T, instance any) reflect.Type {
	t.Helper()

	data, err := GenerateSchemaFromInstance(instance, 10)
	require.NoError(t, err)

	rebuilt, err := CreateInstanceFromSchema(data, KnownTypes())
	require.NoError(t, err)

	return reflect.TypeOf(rebuilt).Elem().Field(0).Type
}

func TestSensitiveStringFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[string] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[string]{}), fieldType)
}

func TestSensitiveIntFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[int] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[int]{}), fieldType)
}

func TestSensitiveInt64FieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[int64] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[int64]{}), fieldType)
}

func TestSensitiveFloat64FieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[float64] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[float64]{}), fieldType)
}

func TestSensitiveBoolFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[bool] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[bool]{}), fieldType)
}

func TestSensitiveStringSliceFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[[]string] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[[]string]{}), fieldType)
}

func TestSensitiveStringMapFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[map[string]string] `xcl:"secret"`
	}

	fieldType := rebuildFieldType(t, holder{})

	require.Equal(t, reflect.TypeOf(types.Sensitive[map[string]string]{}), fieldType)
}

func TestSensitiveFieldSchemaHasNoProperties(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[string] `xcl:"secret"`
	}

	data, err := GenerateSchemaFromInstance(holder{}, 10)
	require.NoError(t, err)

	parsed := Attribute{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Len(t, parsed.Properties, 1)

	secret := parsed.Properties[0]
	require.Equal(t, "types.Sensitive[string]", secret.Type)
	require.Empty(t, secret.Properties)
}

func TestCreateInstanceFromSchemaRejectsUnsupportedSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[uint8] `xcl:"secret"`
	}

	data, err := GenerateSchemaFromInstance(holder{}, 10)
	require.NoError(t, err)

	_, err = CreateInstanceFromSchema(data, KnownTypes())
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported sensitive type")
	require.ErrorContains(t, err, "types.Sensitive[uint8]")
	require.ErrorContains(t, err, "field Secret")
}

func TestParseTypeMapOfSensitive(t *testing.T) {
	pt, err := parseType("map[string]types.Sensitive[string]")
	require.NoError(t, err)

	require.True(t, pt.Map)
	require.Equal(t, "string", pt.MapKey)
	require.Equal(t, "types.Sensitive[string]", pt.Type)
}

func TestParseTypeSensitiveOfMap(t *testing.T) {
	pt, err := parseType("types.Sensitive[map[string]string]")
	require.NoError(t, err)

	require.False(t, pt.Map)
	require.False(t, pt.Slice)
	require.Equal(t, "types.Sensitive[map[string]string]", pt.Type)
}

func TestParseTypeSliceOfSensitive(t *testing.T) {
	pt, err := parseType("[]types.Sensitive[int]")
	require.NoError(t, err)

	require.True(t, pt.Slice)
	require.Equal(t, "types.Sensitive[int]", pt.Type)
}

func TestParseTypePointerToSliceOfSensitive(t *testing.T) {
	pt, err := parseType("*[]types.Sensitive[int]")
	require.NoError(t, err)

	require.True(t, pt.OuterPointer)
	require.True(t, pt.Slice)
	require.Equal(t, "types.Sensitive[int]", pt.Type)
}

func TestParseTypeMapOfMap(t *testing.T) {
	pt, err := parseType("map[string]map[string]string")
	require.NoError(t, err)

	require.True(t, pt.Map)
	require.Equal(t, "string", pt.MapKey)
	require.Equal(t, "map[string]string", pt.Type)
}

func TestParseTypeRejectsUnclosedMapKey(t *testing.T) {
	_, err := parseType("map[string")
	require.Error(t, err)
}
