package schema

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// rebuildFieldType describes holder as a plugin does and rebuilds a struct
// from the description the way the host does, returning the type of the field
// called Secret.
func rebuildFieldType(t *testing.T, holder any) reflect.Type {
	t.Helper()

	data, err := GenerateV1(reflect.TypeOf(holder), "holder", "", false)
	require.NoError(t, err)

	rebuilt, err := CreateInstanceFromSchema(data)
	require.NoError(t, err)

	field, ok := reflect.TypeOf(rebuilt).Elem().FieldByName("Secret")
	require.True(t, ok)

	return field.Type
}

func TestSensitiveStringFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[string] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[string]{}), rebuildFieldType(t, holder{}))
}

func TestSensitiveIntFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[int] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[int]{}), rebuildFieldType(t, holder{}))
}

func TestSensitiveInt64FieldIsRebuiltAsASensitiveInt(t *testing.T) {
	// a schema has one kind of whole number, which the host holds as an int
	type holder struct {
		Secret types.Sensitive[int64] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[int]{}), rebuildFieldType(t, holder{}))
}

func TestSensitiveFloat64FieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[float64] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[float64]{}), rebuildFieldType(t, holder{}))
}

func TestSensitiveBoolFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[bool] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[bool]{}), rebuildFieldType(t, holder{}))
}

func TestSensitiveStringSliceFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[[]string] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[[]string]{}), rebuildFieldType(t, holder{}))
}

func TestSensitiveStringMapFieldIsRebuiltWithSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[map[string]string] `xcl:"secret"`
	}

	require.Equal(t, reflect.TypeOf(types.Sensitive[map[string]string]{}), rebuildFieldType(t, holder{}))
}

func TestGenerateV1RefusesUnsupportedSensitiveType(t *testing.T) {
	type holder struct {
		Secret types.Sensitive[uint8] `xcl:"secret"`
	}

	_, err := GenerateV1(reflect.TypeOf(holder{}), "holder", "", false)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported sensitive type")
	require.ErrorContains(t, err, "types.Sensitive[uint8]")
	require.ErrorContains(t, err, "field Secret")
}
