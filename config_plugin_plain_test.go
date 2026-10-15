package xcl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugins/plain/plainplugin"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// A plugin's types may be plain structs, laid out as a Go application's are:
// gadget takes its label in a field, and hub, with neither label fields nor
// a Meta field, is a singleton. These tests apply the same configuration
// through the plugin in process and as a binary.

// plainPluginConfig declares the hub and a gadget
const plainPluginConfig = `
hub {
  address = "10.0.0.1"
}

gadget "lamp" {
  size = 3
}
`

// buildPlainPlugin builds the plain fixture plugin into a temporary directory
// and returns the path of the binary
func buildPlainPlugin(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "xcl-plugin-plain")

	build := exec.Command("go", "build", "-o", binary, "./internal/test_fixtures/plugins/plain")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "unable to build the plain plugin: %s", output)

	return binary
}

// inProcessPlainRegistry returns a registry holding the plain plugin in
// process
func inProcessPlainRegistry() *registry.Local {
	local := registry.NewLocal()
	local.RegisterPlugin(&plainplugin.Plugin{})

	return local
}

// externalPlainRegistry returns a registry holding the plain plugin binary
func externalPlainRegistry(binary string) *registry.Local {
	local := registry.NewLocal()
	local.RegisterExternalPlugin(binary)

	return local
}

// writePlainPluginConfig writes contents as main.xcl and returns its path
func writePlainPluginConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))

	return path
}

func TestApplyCreatesInProcessPluginPlainType(t *testing.T) {
	isolateHome(t)

	f := newPersonConfig(t, inProcessPlainRegistry(), writePlainPluginConfig(t, plainPluginConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	gadget, err := Find[plainplugin.Gadget](f.config, "gadget.lamp")
	require.NoError(t, err)
	require.Equal(t, "lamp", gadget.Handle)
	require.Equal(t, 3, gadget.Size)
	require.Equal(t, "lamp of size 3", gadget.Described)
}

func TestApplyCreatesInProcessPluginSingleton(t *testing.T) {
	isolateHome(t)

	f := newPersonConfig(t, inProcessPlainRegistry(), writePlainPluginConfig(t, plainPluginConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	hub, err := Find[plainplugin.Hub](f.config, "hub")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.1", hub.Address)
	require.Equal(t, "10.0.0.1", hub.Dialled)
}

func TestInProcessPluginSingletonWithALabelIsRefused(t *testing.T) {
	isolateHome(t)

	f := newPersonConfig(t, inProcessPlainRegistry(), writePlainPluginConfig(t, `hub "main" { address = "10.0.0.1" }`), t.TempDir())

	err := f.config.Validate(f.configFile)
	require.ErrorContains(t, err, "hub takes no labels")
}

func TestApplyCreatesExternalPluginPlainTypeAndSingleton(t *testing.T) {
	// built before HOME is isolated, so the build uses the real module cache
	binary := buildPlainPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalPlainRegistry(binary), writePlainPluginConfig(t, plainPluginConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	// the provider computes these from what it received, so they prove the
	// plain values crossed the process boundary both ways
	gadget, err := Find[plainplugin.Gadget](f.config, "gadget.lamp")
	require.NoError(t, err)
	require.Equal(t, "lamp", gadget.Handle)
	require.Equal(t, "lamp of size 3", gadget.Described)

	hub, err := Find[plainplugin.Hub](f.config, "hub")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.1", hub.Dialled)
}

func TestReapplyLeavesExternalPluginPlainTypesAsTheyAre(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	stateDir := t.TempDir()
	configFile := writePlainPluginConfig(t, plainPluginConfig)

	first := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, first.config.Apply(first.configFile))

	second := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, second.config.Apply(second.configFile))

	hub, err := Find[plainplugin.Hub](second.config, "hub")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.1", hub.Dialled)

	gadget, err := Find[plainplugin.Gadget](second.config, "gadget.lamp")
	require.NoError(t, err)
	require.Equal(t, "lamp of size 3", gadget.Described)
}

func TestPlanAfterApplyChangesNothingForInProcessPluginPlainTypes(t *testing.T) {
	isolateHome(t)

	f := newPersonConfig(t, inProcessPlainRegistry(), writePlainPluginConfig(t, plainPluginConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	// a plain type's Meta differs between the saved and the planned entity,
	// and is not a change
	changes, err := f.config.Diff([]string{f.configFile})
	require.NoError(t, err)
	require.Equal(t, 0, changes.Changed())
}

func TestPlanAfterApplyChangesNothingForExternalPluginPlainTypes(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalPlainRegistry(binary), writePlainPluginConfig(t, plainPluginConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	changes, err := f.config.Diff([]string{f.configFile})
	require.NoError(t, err)
	require.Equal(t, 0, changes.Changed())
}

func TestAReaderFindsAPluginsPlainTypeByItsPath(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	stateDir := t.TempDir()
	configFile := writePlainPluginConfig(t, plainPluginConfig)

	applier := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, applier.config.Apply(applier.configFile))

	// a separate program reading what was applied, which never registered
	// the plain types itself
	reader := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, reader.config.Load())

	gadgets, err := FindByType[plainplugin.Gadget](reader.config, "gadget")
	require.NoError(t, err)
	require.Len(t, gadgets, 1)
	require.Equal(t, "lamp", gadgets[0].Handle)
	require.Equal(t, "gadget.lamp", gadgets[0].Meta.ID)
	require.Equal(t, "lamp of size 3", gadgets[0].Described)

	hub, err := FindOne[plainplugin.Hub](reader.config, "hub")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.1", hub.Dialled)
}

// gizmoConfig declares a gizmo, of a type with a subtype and two label fields,
// with a secret and two parts
const gizmoConfig = `
gizmo "small" "spinner" {
  secret = "s3cret"

  part {
    name = "axle"
  }

  part {
    name = "wheel"
  }
}
`

func TestExternalPluginPlainTypeFillsTwoLabelFields(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalPlainRegistry(binary), writePlainPluginConfig(t, gizmoConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	gizmo, err := Find[plainplugin.Gizmo](f.config, "gizmo.small.spinner")
	require.NoError(t, err)
	require.Equal(t, "small", gizmo.Size)
	require.Equal(t, "spinner", gizmo.Handle)
}

func TestExternalPluginPlainTypeReceivesItsSecret(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalPlainRegistry(binary), writePlainPluginConfig(t, gizmoConfig), t.TempDir())
	require.NoError(t, f.config.Apply(f.configFile))

	gizmo, err := Find[plainplugin.Gizmo](f.config, "gizmo.small.spinner")
	require.NoError(t, err)

	// the provider computed this from the secret's real value
	require.Equal(t, len("s3cret"), gizmo.SecretLength)
	require.Equal(t, "(sensitive)", gizmo.Secret.String())
}

func TestExternalPluginPlainTypeKeepsComputedValuesOfNestedBlocks(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	stateDir := t.TempDir()
	configFile := writePlainPluginConfig(t, gizmoConfig)

	first := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, first.config.Apply(first.configFile))

	second := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)

	changes, err := second.config.Diff([]string{configFile})
	require.NoError(t, err)
	require.Equal(t, 0, changes.Changed())

	require.NoError(t, second.config.Apply(second.configFile))

	gizmo, err := Find[plainplugin.Gizmo](second.config, "gizmo.small.spinner")
	require.NoError(t, err)
	require.Equal(t, []plainplugin.Part{
		{Name: "axle", Serial: "small-spinner-axle"},
		{Name: "wheel", Serial: "small-spinner-wheel"},
	}, gizmo.Parts)
}

func TestExternalPluginPlainTypeIsUpdatedInPlace(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	stateDir := t.TempDir()
	configFile := writePlainPluginConfig(t, `gadget "lamp" { size = 3 }`)

	first := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, first.config.Apply(first.configFile))

	require.NoError(t, os.WriteFile(configFile, []byte(`gadget "lamp" { size = 4 }`), 0o644))

	recorder := &eventRecorder{}
	second := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir, WithEventHandler(recorder.Record))
	require.NoError(t, second.config.Apply(second.configFile))

	require.Len(t, pluginLogsWithMessage(recorder, "updated gadget"), 1)
	require.Empty(t, pluginLogsWithMessage(recorder, "created gadget"))

	gadget, err := Find[plainplugin.Gadget](second.config, "gadget.lamp")
	require.NoError(t, err)
	require.Equal(t, "lamp of size 4", gadget.Described)
	require.Equal(t, "gadget.lamp", gadget.Meta.ID)
}

func TestExternalPluginPlainTypeReadAsGoneIsCreatedAgain(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	stateDir := t.TempDir()
	configFile := writePlainPluginConfig(t, `gadget "lamp" { size = 3 }`)

	first := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir)
	require.NoError(t, first.config.Apply(first.configFile))

	require.NoError(t, os.WriteFile(configFile, []byte(fmt.Sprintf(`gadget "lamp" { size = %d }`, plainplugin.Lost)), 0o644))

	recorder := &eventRecorder{}
	second := newPersonConfig(t, externalPlainRegistry(binary), configFile, stateDir, WithEventHandler(recorder.Record))
	require.NoError(t, second.config.Apply(second.configFile))

	require.Len(t, pluginLogsWithMessage(recorder, "created gadget"), 1)

	gadget, err := Find[plainplugin.Gadget](second.config, "gadget.lamp")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("lamp of size %d", plainplugin.Lost), gadget.Described)
}

func TestDestroyRemovesExternalPluginSingleton(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	recorder := &eventRecorder{}
	f := newPersonConfig(t, externalPlainRegistry(binary), writePlainPluginConfig(t, plainPluginConfig), t.TempDir(), WithEventHandler(recorder.Record))
	require.NoError(t, f.config.Apply(f.configFile))
	require.NoError(t, f.config.Destroy())

	destroyed := pluginLogsWithMessage(recorder, "destroyed hub")
	require.Len(t, destroyed, 1)
	require.Equal(t, "hub", destroyed[0].ResourceID)
	require.Equal(t, 0, f.config.ResourceCount())
}

// nestedLayout is a layout whose blocks nest depth deep, a column holding a
// row holding a column and so on, two columns beside each other at every
// column level
func nestedLayout(depth int) string {
	var body func(level int) string
	body = func(level int) string {
		if level > depth {
			return ""
		}

		if level%2 == 1 {
			return fmt.Sprintf("column {\n%s}\ncolumn {}\n", body(level+1))
		}

		return fmt.Sprintf("row {\n%s}\n", body(level+1))
	}

	return fmt.Sprintf("layout \"main\" {\n%s}\n", body(1))
}

// columnsIn counts the columns nestedLayout writes depth deep
func columnsIn(depth int) int {
	return 2 * ((depth + 1) / 2)
}

func TestApplyNestsALayoutAsDeepAsConfigurationMay(t *testing.T) {
	binary := buildPlainPlugin(t)
	isolateHome(t)

	for name, local := range map[string]*registry.Local{
		"in process": inProcessPlainRegistry(),
		"external":   externalPlainRegistry(binary),
	} {
		t.Run(name, func(t *testing.T) {
			f := newPersonConfig(t, local, writePlainPluginConfig(t, nestedLayout(schema.MaxNestingDepth)), t.TempDir())
			require.NoError(t, f.config.Apply(f.configFile))

			// the provider counts the columns it received, at every depth
			layout, err := Find[plainplugin.Layout](f.config, "layout.main")
			require.NoError(t, err)
			require.Equal(t, columnsIn(schema.MaxNestingDepth), layout.Counted)
		})
	}
}

func TestValidateRefusesALayoutNestedDeeperThanConfigurationMay(t *testing.T) {
	isolateHome(t)

	f := newPersonConfig(t, inProcessPlainRegistry(), writePlainPluginConfig(t, nestedLayout(schema.MaxNestingDepth+1)), t.TempDir())

	err := f.config.Validate(f.configFile)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Equal(t, "Blocks nested too deep", d.Summary)
	require.Equal(t, fmt.Sprintf("column is nested %d blocks deep, and blocks nest at most %d deep", schema.MaxNestingDepth+1, schema.MaxNestingDepth), d.Detail)
}
