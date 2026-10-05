package types_test

// This test lives in an external test package because internal/schema
// depends, through internal/wire, on the types package itself.

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

type SchemaTestBaseResource struct {
	types.ResourceBase
	Name string
}

type SchemaTestExtendedResource struct {
	SchemaTestBaseResource
	ExtraField string
}

func schemaTestCreateExtendedResource() *SchemaTestExtendedResource {
	return &SchemaTestExtendedResource{
		SchemaTestBaseResource: SchemaTestBaseResource{
			ResourceBase: types.ResourceBase{
				DependsOn: []string{"dependency1", "dependency2"},
				Disabled:  true,
				Meta: types.Meta{
					ID:      "test-id",
					Name:    "test-name",
					Type:    types.TypeResource,
					Subtype: "test-type",
				},
			},
			Name: "test-resource",
		},
		ExtraField: "extra-value",
	}
}

func TestCanGetMetaOnExtendedResourceWhenCreatedFromSchema(t *testing.T) {
	te := schemaTestCreateExtendedResource()

	sch, err := schema.GenerateSchemaFromInstance(te, 10)
	require.NoError(t, err)

	typeMapping := map[string]reflect.Type{
		"types.Meta":         reflect.TypeOf(types.Meta{}),
		"types.ResourceBase": reflect.TypeOf(types.ResourceBase{}),
	}

	ni, err := schema.CreateInstanceFromSchema(sch, typeMapping)
	require.NoError(t, err)

	// The schema doesn't preserve values, only structure.
	// This test should verify that:
	// 1. GetMeta works on the newly created instance (returns no error)
	// 2. The Meta field is of the correct type (types.Meta)
	// 3. We can set and get values on the Meta field

	// Step 1: Verify GetMeta works without error
	meta, err := types.GetMeta(ni)
	require.NoError(t, err)
	require.NotNil(t, meta)

	// Step 2: Verify the Meta is of the correct type
	require.IsType(t, &types.Meta{}, meta)

	// Step 3: Verify we can set and get values
	meta.ID = "new-id"
	meta.Name = "new-name"
	meta.Type = types.TypeResource
	meta.Subtype = "new-type"

	// Get meta again to verify the values were set
	meta2, err := types.GetMeta(ni)
	require.NoError(t, err)
	require.Equal(t, "new-id", meta2.ID)
	require.Equal(t, "new-name", meta2.Name)
	require.Equal(t, types.TypeResource, meta2.Type)
	require.Equal(t, "new-type", meta2.Subtype)
}
