package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// TODO: These tests need to be rewritten for the new architecture where Config
// orchestrates Parser and State, rather than managing resources directly.
// Most of the functionality being tested here has moved to State or is now
// handled by parsing HCL files through Config.Apply()

// TestNewConfig tests that NewConfig creates a valid config with initialized state
func TestNewConfig(t *testing.T) {
	c, err := NewConfig()
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, 0, c.ResourceCount())
	require.NotNil(t, c.GetResources())
}

// TestFindResourceReturnsNotFoundError tests that FindResource returns an error for non-existent resources
func TestFindResourceReturnsNotFoundError(t *testing.T) {
	c, err := NewConfig()
	require.NoError(t, err)

	r, err := c.FindResource("resource.container.notexist")
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, r)
}

/*
// The tests below test the old Config implementation that directly managed resources.
// They need to be rewritten to test the new architecture where:
// - Config.Apply() parses HCL files and applies them
// - Config.Validate() parses and returns a diff
// - Resources are created by parsing, not manually
// - State is managed by Parser and returned to Config

func testSetupConfig(t *testing.T) (*Config, []any) {
	typs := resources.DefaultResources()
	typs[structs.TypeNetwork] = &structs.Network{}
	typs[structs.TypeContainer] = &structs.Container{}
	typs[structs.TypeTemplate] = &structs.Template{}

	var1, _ := typs.CreateResource(resources.TypeVariable, "var1")

	net1, _ := typs.CreateResource(structs.TypeNetwork, "cloud")

	mod1, _ := typs.CreateResource(resources.TypeModule, "module1")
	types.AppendUniqueLink(mod1, "resource.network.cloud")

	var2, _ := typs.CreateResource(resources.TypeVariable, "var2")
	meta, _ := types.GetMeta(var2)
	meta.Module = "module1"

	mod2, _ := typs.CreateResource(resources.TypeModule, "module2")
	meta, _ = types.GetMeta(mod2)
	meta.Module = "module1"

	// depending on a module should return all resources and
	// all child resources
	con1, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	types.AppendUniqueLink(con1, "module.module1")

	// con2 is embedded in module1
	con2, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	meta, _ = types.GetMeta(con2)
	meta.Module = "module1"

	// con3 is loaded from a module inside module2
	con3, _ := typs.CreateResource(structs.TypeContainer, "test_dev")
	meta, _ = types.GetMeta(con3)
	meta.Module = "module1.module2"

	// con4 is loaded from a module inside module2
	con4, _ := typs.CreateResource(structs.TypeContainer, "test_dev2")
	meta, _ = types.GetMeta(con4)
	meta.Module = "module1.module2"

	// depends on would be added relative as a resource
	// when a resource is defined, it has no idea on its
	// module
	types.AppendUniqueLink(con4, "resource.container.test_dev")

	out1, _ := typs.CreateResource(resources.TypeOutput, "fqdn")
	meta, _ = types.GetMeta(out1)
	meta.Module = "module1.module2"

	out2, _ := typs.CreateResource(resources.TypeOutput, "out")
	types.AppendUniqueLink(out2, "resource.network.cloud.id")
	types.AppendUniqueLink(out2, "resource.container.test_dev")

	c, err := NewConfig()
	require.NoError(t, err)
	err := c.addResource(net1, nil)
	require.NoError(t, err)

	err = c.addResource(var1, nil)
	require.NoError(t, err)

	// add the modules
	err = c.addResource(mod1, nil)
	require.NoError(t, err)

	err = c.addResource(var2, nil)
	require.NoError(t, err)

	err = c.addResource(mod2, nil)
	require.NoError(t, err)

	err = c.addResource(con1, nil)
	require.NoError(t, err)

	err = c.addResource(con2, nil)
	require.NoError(t, err)

	err = c.addResource(con3, nil)
	require.NoError(t, err)

	err = c.addResource(con4, nil)
	require.NoError(t, err)

	err = c.addResource(out1, nil)
	require.NoError(t, err)

	err = c.addResource(out2, nil)
	require.NoError(t, err)

	return c, []any{
		net1,
		con1,
		mod1,
		mod2,
		con2,
		con3,
		con4,
		out1,
		out2,
		var1,
		var2,
	}
}

func TestResourceCount(t *testing.T) {
	c, r := testSetupConfig(t)
	require.Equal(t, len(r), c.ResourceCount())
}

func TestAddResourceExistsReturnsError(t *testing.T) {
	c, r := testSetupConfig(t)

	err := c.AppendResource(r[3])
	require.Error(t, err)
}

func TestFindResourceFindsContainer(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("resource.container.test_dev")
	require.NoError(t, err)
	require.Equal(t, r[1], cl)
}

func TestFindResourceFindsVariable(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("variable.var1")
	require.NoError(t, err)
	require.Equal(t, r[9], cl)
}

func TestFindResourceFindsModuleVariable(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("module.module1.variable.var2")
	require.NoError(t, err)
	require.Equal(t, r[10], cl)
}

func TestFindOutputFindsOutput(t *testing.T) {
	c, _ := testSetupConfig(t)

	_, err := c.FindResource("output.out")
	require.NoError(t, err)
}

func TestFindOutputFindsModule(t *testing.T) {
	c, _ := testSetupConfig(t)

	_, err := c.FindResource("module.module1")
	require.NoError(t, err)
}

func TestFindResourceFindsModuleOutput(t *testing.T) {
	c, r := testSetupConfig(t)

	out, err := c.FindResource("module.module1.module2.output.fqdn")
	require.NoError(t, err)
	require.Equal(t, r[7], out)
}

func TestFindResourceFindsModuleOutputWithIndex(t *testing.T) {
	c, r := testSetupConfig(t)

	out, err := c.FindResource("module.module1.module2.output.fqdn.0")
	require.NoError(t, err)
	require.Equal(t, r[7], out)
}

func TestFindResourceFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindResource("module.module1.resource.container.test_dev")
	require.NoError(t, err)
	require.Equal(t, r[4], cl)
}

func TestFindRelativeResourceWithParentFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindRelativeResource("resource.container.test_dev", "module1")
	require.NoError(t, err)
	require.Equal(t, r[4], cl)
}

func TestFindRelativeResourceWithModuleAndParentFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindRelativeResource("module.module2.resource.container.test_dev", "module1")
	require.NoError(t, err)
	require.Equal(t, r[5], cl)
}

func TestFindRelativeResourceWithModuleAndNoParentFindsClusterInModule(t *testing.T) {
	c, r := testSetupConfig(t)

	cl, err := c.FindRelativeResource("module.module1.resource.container.test_dev", "")
	require.NoError(t, err)
	require.Equal(t, r[4], cl)
}

func TestFindResourceReturnsNotFoundError(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindResource("resource.container.notexist")
	require.Error(t, err)
	require.IsType(t, ResourceNotFoundError{}, err)
	require.Nil(t, cl)
}

func TestFindResourcesByTypeContainers(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindResourcesByType("container")
	require.NoError(t, err)
	require.Len(t, cl, 4)
}

func TestFindModuleResourcesFindsResources(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindModuleResources("module.module1", false)
	require.NoError(t, err)

	// should have one resource and one module
	require.Len(t, cl, 3)
}

func TestFindModuleResourcesFindsResourcesWithChildren(t *testing.T) {
	c, _ := testSetupConfig(t)

	cl, err := c.FindModuleResources("module.module1", true)
	require.NoError(t, err)
	require.Len(t, cl, 6)
}

func TestRemoveResourceRemoves(t *testing.T) {
	c, _ := testSetupConfig(t)

	err := c.RemoveResource(c.Resources[0])
	require.NoError(t, err)
	require.Len(t, c.Resources, 10)
}

func TestRemoveResourceNotFoundReturnsError(t *testing.T) {
	typs := resources.DefaultResources()
	typs[structs.TypeNetwork] = &structs.Network{}

	c, _ := testSetupConfig(t)
	net1, _ := typs.CreateResource(structs.TypeNetwork, "notfound")

	err := c.RemoveResource(net1)
	require.Error(t, err)
	require.Len(t, c.Resources, 11)
}

// TestToJSONSerializesJSON - functionality moved to StateStore tests
// func TestToJSONSerializesJSON(t *testing.T) {
//	c, _ := testSetupConfig(t)
//
//	d, err := c.ToJSON()
//	require.NoError(t, err)
//	require.Greater(t, len(d), 0)
//
//	require.Contains(t, string(d), `"name": "test_dev"`)
//}

func TestAppendResourcesMerges(t *testing.T) {
	typs := resources.DefaultResources()
	typs[structs.TypeNetwork] = &structs.Network{}

	c, _ := testSetupConfig(t)

	c2, err := NewConfig()
	require.NoError(t, err)
	net1, err := typs.CreateResource(structs.TypeNetwork, "cloud2")
	require.NoError(t, err)
	c2.addResource(net1, nil)

	err = c.AppendResourcesFromConfig(c2)
	require.NoError(t, err)

	net2, err := c.FindResource("resource.network.cloud2")
	require.NoError(t, err)
	require.Equal(t, net1, net2)
}

func TestAppendResourcesWhenExistsReturnsError(t *testing.T) {
	typs := resources.DefaultResources()
	typs[structs.TypeNetwork] = &structs.Network{}

	c, _ := testSetupConfig(t)

	c2, err := NewConfig()
	require.NoError(t, err)
	net1, err := typs.CreateResource(structs.TypeNetwork, "cloud")
	require.NoError(t, err)
	c2.addResource(net1, nil)

	err = c.AppendResourcesFromConfig(c2)
	require.Error(t, err)
}

// TODO: Move these tests to parser_test.go since Walk is now private to Parser
// TestProcessForwardExecutesCallbacksInCorrectOrder
// TestProcessReverseExecutesCallbacksInCorrectOrder
// TestProcessCallbackErrorHaltsExecution
/*
func TestProcessForwardExecutesCallbacksInCorrectOrder(t *testing.T) {
	c, _ := testSetupConfig(t)

	calls := []string{}
	callSync := sync.Mutex{}
	err := c.Walk(
		func(r any) error {
			callSync.Lock()

			meta, err := types.GetMeta(r)
			if err != nil {
				return err
			}
			calls = append(calls, resources.FQRN{
				Module:   meta.Module,
				Resource: meta.Name,
				Type:     meta.Type,
			}.String())

			callSync.Unlock()

			return nil
		},
		false,
	)

	require.NoError(t, err)

	// test_dev depends on cloud so should always be called after it
	requireBefore(t, "resource.network.cloud", "module.module1.resource.container.test_dev", calls)

	// out.out depends on resource.container.test_dev depends on module 1 so the container should be called last
	// after all resources in module 1 have been created
	require.Equal(t, "output.out", calls[8])
}

func TestProcessReverseExecutesCallbacksInCorrectOrder(t *testing.T) {
	c, _ := testSetupConfig(t)

	calls := []string{}
	callSync := sync.Mutex{}
	err := c.Walk(
		func(r any) error {
			callSync.Lock()

			meta, err := types.GetMeta(r)
			if err != nil {
				return err
			}
			calls = append(calls, resources.FQRN{
				Module:   meta.Module,
				Resource: meta.Name,
				Type:     meta.Type,
			}.String())

			callSync.Unlock()

			return nil
		},
		true,
	)

	require.NoError(t, err)

	// resource.container.test_dev depends on module.module1 so the call back for test_dev
	// should happen first before anything else
	require.Equal(t, "output.out", calls[0])
	requireBefore(t, "resource.container.test_dev", "module.module1.module2.output.fqdn", calls)
}

func TestProcessCallbackErrorHaltsExecution(t *testing.T) {
	c, _ := testSetupConfig(t)

	calls := []string{}
	callSync := sync.Mutex{}
	err := c.Walk(
		func(r any) error {
			callSync.Lock()
			meta, err := types.GetMeta(r)
			if err != nil {
				return err
			}
			calls = append(calls, resources.FQRN{
				Module:   meta.Module,
				Resource: meta.Name,
				Type:     meta.Type,
			}.String())

			callSync.Unlock()

			fmt.Println(meta.Name)

			if meta.Name == "cloud" {
				return fmt.Errorf("boom")
			}

			return nil
		},
		false,
	)

	// we should get an error from process
	require.Error(t, err)

	// process should stop the callbacks, there should only
	// be one callback network cloud
	require.Equal(t, 1, len(calls))
}
*/

// savedReferences returns the meta.references the saved record holds, nil
// when it holds none
func savedReferences(t *testing.T, record json.RawMessage) map[string]string {
	t.Helper()

	saved := struct {
		Meta struct {
			References map[string]string `json:"references"`
		} `json:"meta"`
	}{}

	err := json.Unmarshal(record, &saved)
	require.NoError(t, err)

	return saved.Meta.References
}

func TestApplySavesReferencesInState(t *testing.T) {
	c, _, statePath := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)
	databaseMeta, err := types.GetMeta(database)
	require.NoError(t, err)

	require.Equal(t, map[string]string{"location": "variable.region"}, databaseMeta.References)

	savedDatabase := savedReferences(t, encodeSavedRecordByID(t, statePath, encodeDatabaseID))
	require.Equal(t, databaseMeta.References, savedDatabase)

	container := encodeEntityByID(t, c, encodeContainerID)
	containerMeta, err := types.GetMeta(container)
	require.NoError(t, err)

	expectedContainer := map[string]string{
		"network[0].name": "resource.network.main.meta.name",
		"network[1].name": "resource.network.main.meta.name",
	}
	require.Equal(t, expectedContainer, containerMeta.References)

	savedContainer := savedReferences(t, encodeSavedRecordByID(t, statePath, encodeContainerID))
	require.Equal(t, containerMeta.References, savedContainer)
}

// stripSavedReferences rewrites the state file at statePath without the
// meta.references of any record, as state written before references were
// recorded would be
func stripSavedReferences(t *testing.T, statePath string) {
	t.Helper()

	data, err := os.ReadFile(statePath)
	require.NoError(t, err)

	records := []map[string]json.RawMessage{}
	err = json.Unmarshal(data, &records)
	require.NoError(t, err)

	for _, record := range records {
		meta := map[string]json.RawMessage{}
		err = json.Unmarshal(record["meta"], &meta)
		require.NoError(t, err)

		delete(meta, "references")

		record["meta"], err = json.Marshal(meta)
		require.NoError(t, err)
	}

	data, err = json.MarshalIndent(records, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(statePath, data, 0644)
	require.NoError(t, err)
}

// State is written by a real apply and only the meta.references key is then
// removed from it, the one change that turns it into state saved before
// references were recorded
func TestStateWithoutReferencesStillLoads(t *testing.T) {
	_, reg, statePath := applyEncodeFixture(t)

	stripSavedReferences(t, statePath)

	record := encodeSavedRecordByID(t, statePath, encodeDatabaseID)
	require.Nil(t, savedReferences(t, record))

	// the saved record still decodes, with its resolved value
	store, err := state.NewFileStateStore(filepath.Dir(statePath))
	require.NoError(t, err)

	records, err := store.Load()
	require.NoError(t, err)

	loaded, err := savedentity.DecodeAll(reg, records)
	require.NoError(t, err)

	var database *registered.Database
	for _, entity := range loaded {
		candidate, ok := entity.(*registered.Database)
		if ok && candidate.Meta.ID == encodeDatabaseID {
			database = candidate
		}
	}

	require.NotNil(t, database)
	require.Equal(t, "us-east", database.Location)
	require.Nil(t, database.Meta.References)

	// the saved record still converts to configuration text showing the
	// resolved value
	text, err := EncodeSavedEntity(reg, record)
	require.NoError(t, err)
	require.Regexp(t, `location\s+= "us-east"`, string(text))

	// asking for references still works, the record has none to show so the
	// resolved value is written
	withReferences, err := EncodeSavedEntity(reg, record, ShowReferences())
	require.NoError(t, err)
	require.Regexp(t, `location\s+= "us-east"`, string(withReferences))
	require.NotContains(t, string(withReferences), "variable.region")

	// a later run reads the stripped state back and applies over it
	second, err := NewConfig(WithPluginRegistry(reg), WithStateStore(store))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/encode/main.xcl")
	require.NoError(t, err)

	err = second.Apply(path)
	require.NoError(t, err)

	applied := encodeEntityByID(t, second, encodeDatabaseID)
	require.Equal(t, "us-east", applied.(*registered.Database).Location)
}
