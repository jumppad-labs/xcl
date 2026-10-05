package parser

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// applyReferencesFixture applies the references fixture with the test plugin
// and returns every entity it produced
func applyReferencesFixture(t *testing.T) []any {
	t.Helper()

	path, err := filepath.Abs("../test_fixtures/config/references/main.xcl")
	require.NoError(t, err)

	p, _ := setupParser(t)

	c, err := p.Apply(context.Background(), path)
	require.NoError(t, err)

	return c.GetResources()
}

func TestParseRecordsBareReferenceAsWritten(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "resource.container.bare")

	require.Equal(t, "variable.region", cont.Meta.References["default"])
}

func TestParseRecordsTemplateAsWritten(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "resource.container.template")

	require.Equal(t, `"${variable.region}-a"`, cont.Meta.References["default"])
}

func TestParseRecordsFunctionCallAsWritten(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "resource.container.function")

	require.Equal(t, `format("%s-b", variable.region)`, cont.Meta.References["default"])
}

func TestParseRecordsMultiLineValueAsWritten(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "resource.container.multi_line")

	expected := "{\n" +
		"    REGION = variable.region\n" +
		"    MODE   = \"web\"\n" +
		"  }"

	require.Equal(t, expected, cont.Meta.References["env"])
}

func TestParseRecordsNestedBlockReferenceByPosition(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "resource.container.nested")

	require.Equal(t, "resource.network.main.meta.name", cont.Meta.References["network[1].name"])
	require.Equal(t, "variable.cpu", cont.Meta.References["resources[0].cpu"])
}

func TestParseRecordsModuleReferenceUnscoped(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "module.inner.resource.container.app")

	require.Equal(t, "variable.region", cont.Meta.References["default"])
	require.Equal(t, "resource.network.inner.meta.name", cont.Meta.References["network[0].name"])
}

func TestParseRecordsNothingForLiteralFields(t *testing.T) {
	entities := applyReferencesFixture(t)

	cont := findResource[structs.Container](t, entities, "resource.container.nested")

	// only the two fields written as references are recorded
	require.Len(t, cont.Meta.References, 2)

	require.NotContains(t, cont.Meta.References, "command")
	require.NotContains(t, cont.Meta.References, "network[0].name")
	require.NotContains(t, cont.Meta.References, "network[0].ip_address")
	require.NotContains(t, cont.Meta.References, "network[1].ip_address")
	require.NotContains(t, cont.Meta.References, "resources[0].memory")
}

func TestParseRecordsNothingForEntityWithoutReferences(t *testing.T) {
	entities := applyReferencesFixture(t)

	// read the entity's own meta rather than a copy decoded from JSON, so a
	// nil map is told apart from an empty one
	entity, err := findByID(entities, "resource.network.main")
	require.NoError(t, err)

	meta, err := types.GetMeta(entity)
	require.NoError(t, err)

	require.Nil(t, meta.References)
}
