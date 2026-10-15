package xcl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// A singleton goes through every operation an entity does: it is planned,
// rendered, encoded, ordered, disabled and declared in modules like any
// other.

// writeSingletonFiles writes files, by name, into a directory of their own and
// returns it
func writeSingletonFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	for name, contents := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	return dir
}

// A plan lists only entities with a provider, so these use the plain plugin's
// singleton, hub.

func TestPlanUpdatesAChangedPluginSingletonAtItsAddress(t *testing.T) {
	isolateHome(t)

	stateDir := t.TempDir()
	configFile := writePlainPluginConfig(t, `hub { address = "10.0.0.1" }`)

	applier := newPersonConfig(t, inProcessPlainRegistry(), configFile, stateDir)
	require.NoError(t, applier.config.Apply(applier.configFile))

	require.NoError(t, os.WriteFile(configFile, []byte(`hub { address = "10.0.0.2" }`), 0o644))

	planner := newPersonConfig(t, inProcessPlainRegistry(), configFile, stateDir)

	changes, err := planner.config.Diff([]string{configFile})
	require.NoError(t, err)
	require.Equal(t, 1, changes.Changed())
	require.Equal(t, "hub", changes.Resources[0].Address)
	require.Equal(t, diff.ActionUpdate, changes.Resources[0].Action)
}

func TestPlanRendersAPluginSingletonWithoutLabels(t *testing.T) {
	isolateHome(t)

	f := newPersonConfig(t, inProcessPlainRegistry(), writePlainPluginConfig(t, `hub { address = "10.0.0.1" }`), t.TempDir())

	changes, err := f.config.Diff([]string{f.configFile})
	require.NoError(t, err)

	text := string(highlight.Text.Render(changes.Pieces()))
	require.Contains(t, text, "hub {")
	require.NotContains(t, text, `hub ""`)
}

func TestEncodeEntityWritesASingletonWithoutLabels(t *testing.T) {
	path := writeSingletonConfig(t, `runner { server = "https://maalstrom.app" }`)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	entity, err := c.FindOneEntity("runner")
	require.NoError(t, err)

	out, err := EncodeEntity(entity)
	require.NoError(t, err)
	require.Equal(t, "runner {\n  server = \"https://maalstrom.app\"\n}\n", string(out))
}

func TestEncodeSavedEntityWritesASingletonWithoutLabels(t *testing.T) {
	path := writeSingletonConfig(t, `runner { server = "https://maalstrom.app" }`)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStateStore(store))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	record := encodeSavedRecordByID(t, store.Path(), "runner")

	out, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(out), "runner {\n"), "got:\n%s", out)
}

func TestDependsOnASingletonOrdersAfterIt(t *testing.T) {
	path := writeSingletonConfig(t, `
runner {
  server = "https://maalstrom.app"
}

client "web" {
  server     = "https://example.com"
  depends_on = ["runner"]
}
`)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	client, err := c.FindOneEntity("client")
	require.NoError(t, err)

	meta, err := types.GetMeta(client)
	require.NoError(t, err)
	require.Contains(t, meta.Links, "runner")
}

func TestDisabledSingletonIsLeftOut(t *testing.T) {
	path := writeSingletonConfig(t, `
runner {
  server   = "https://maalstrom.app"
  disabled = true
}
`)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	entity, err := c.FindOneEntity("runner")
	require.NoError(t, err)

	disabled, err := types.GetDisabled(entity)
	require.NoError(t, err)
	require.True(t, disabled)
}

func TestSingletonInAModuleIsReachedThroughItsOutput(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "nas" {
  source = "./nas"
}

client "web" {
  server = module.nas.output.server
}
`,
		"nas/main.xcl": `
runner {
  server = "https://maalstrom.app"
}

output "server" {
  value = runner.server
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(dir))

	client, err := Find[plain.Client](c, "client.web")
	require.NoError(t, err)
	require.Equal(t, "https://maalstrom.app", client.Server)

	runner, err := Find[plain.Runner](c, "module.nas.runner")
	require.NoError(t, err)
	require.Equal(t, "https://maalstrom.app", runner.Server)
}

func TestEachModuleHoldsASingletonOfItsOwn(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
runner {
  server = "https://root.example.com"
}

module "nas" {
  source = "./nas"
}
`,
		"nas/main.xcl": `
runner {
  server = "https://nas.example.com"
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(dir))

	root, err := Find[plain.Runner](c, "runner")
	require.NoError(t, err)
	require.Equal(t, "https://root.example.com", root.Server)

	nas, err := Find[plain.Runner](c, "module.nas.runner")
	require.NoError(t, err)
	require.Equal(t, "https://nas.example.com", nas.Server)
}

func TestTheSameEntityInTwoModulesIsNotDeclaredTwice(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "one" {
  source = "./shared"
}

module "two" {
  source = "./shared"
}
`,
		"shared/main.xcl": `
client "web" {
  server = "https://example.com"
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(dir))

	_, err = Find[plain.Client](c, "module.one.client.web")
	require.NoError(t, err)

	_, err = Find[plain.Client](c, "module.two.client.web")
	require.NoError(t, err)
}

func TestEachInstanceOfAModuleHoldsASingletonOfItsOwn(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "one" {
  source = "./shared"
}

module "two" {
  source = "./shared"
}
`,
		"shared/main.xcl": `
runner {
  server = "https://example.com"
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(dir))

	_, err = Find[plain.Runner](c, "module.one.runner")
	require.NoError(t, err)

	_, err = Find[plain.Runner](c, "module.two.runner")
	require.NoError(t, err)
}

func TestRootSingletonIsPassedIntoAModule(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
runner {
  server = "https://maalstrom.app"
}

module "nas" {
  source = "./nas"

  variables = {
    server = runner.server
  }
}
`,
		"nas/main.xcl": `
variable "server" {
  default = ""
}

client "web" {
  server = variable.server
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(dir))

	client, err := Find[plain.Client](c, "module.nas.client.web")
	require.NoError(t, err)
	require.Equal(t, "https://maalstrom.app", client.Server)
}

func TestSingletonInAModuleCannotBeReferencedFromOutsideIt(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "nas" {
  source = "./nas"
}

client "web" {
  server = module.nas.runner.server
}
`,
		"nas/main.xcl": `
runner {
  server = "https://maalstrom.app"
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)

	err = c.Validate(dir)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Detail, "'module.nas.runner.server', which is inside module 'nas'; only a module's outputs can be referenced from outside it")
	require.NotContains(t, d.Detail, "not defined")
}

func TestDependsOnASingletonInAModuleIsRefused(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "nas" {
  source = "./nas"
}

client "web" {
  depends_on = ["module.nas.runner"]

  server = "https://example.com"
}
`,
		"nas/main.xcl": `
runner {
  server = "https://maalstrom.app"
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)

	err = c.Validate(dir)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Detail, "'module.nas.runner', which is inside module 'nas'; only a module's outputs can be referenced from outside it")
}

func TestSingletonDeclaredTwiceInAModuleIsRefused(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "nas" {
  source = "./nas"
}
`,
		"nas/main.xcl": `
runner {
  server = "https://maalstrom.app"
}

runner {
  server = "https://example.com"
}
`,
	})

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)

	err = c.Validate(dir)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Error(), "Duplicate block")
	require.Equal(t, filepath.Join(dir, "nas", "main.xcl"), d.Subject.Filename)
	require.Equal(t, 6, d.Subject.Start.Line)
}

func TestSingletonInAModuleSurvivesSavedState(t *testing.T) {
	dir := writeSingletonFiles(t, map[string]string{
		"main.xcl": `
module "nas" {
  source = "./nas"
}
`,
		"nas/main.xcl": `
runner {
  server = "https://maalstrom.app"
}
`,
	})
	state := t.TempDir()

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(state))
	require.NoError(t, err)
	require.NoError(t, c.Apply(dir))

	loaded, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(state))
	require.NoError(t, err)
	require.NoError(t, loaded.Load())

	runner, err := Find[plain.Runner](loaded, "module.nas.runner")
	require.NoError(t, err)
	require.Equal(t, "https://maalstrom.app", runner.Server)
}
