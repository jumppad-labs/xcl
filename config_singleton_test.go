package xcl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// singletonConfig declares the runner without labels, and a client reading
// it by reference
const singletonConfig = `
runner {
  server = "https://maalstrom.app"
  listen = "127.0.0.1:7660"
}

client "web" {
  server = runner.server
  runner = runner
}
`

// singletonWithLabelConfig gives the runner a name it does not take
const singletonWithLabelConfig = `
runner "nas" {
  server = "https://maalstrom.app"
}
`

// singletonTwiceConfig declares the runner twice in one module
const singletonTwiceConfig = `
runner {
  server = "https://maalstrom.app"
}

runner {
  server = "https://example.com"
}
`

// singletonRegistry declares the runner, a singleton as it has no label
// fields and no Meta field, and the client beside it
func singletonRegistry() *registry.Local {
	local := registry.NewLocal()
	registry.Register[plain.Runner](local, "runner")
	registry.Register[plain.Client](local, "client")

	return local
}

// writeSingletonConfig writes contents as main.xcl in a directory of its own
// and returns its path
func writeSingletonConfig(t *testing.T, contents string) string {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))

	return path
}

func TestSingletonIsDeclaredWithoutLabels(t *testing.T) {
	path := writeSingletonConfig(t, singletonConfig)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	runner, err := FindOne[plain.Runner](c, "runner")
	require.NoError(t, err)

	require.Equal(t, "https://maalstrom.app", runner.Server)
	require.Equal(t, "127.0.0.1:7660", runner.Listen)
}

func TestSingletonIsIdentifiedByItsType(t *testing.T) {
	path := writeSingletonConfig(t, singletonConfig)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	entity, err := c.FindOneEntity("runner")
	require.NoError(t, err)

	meta, err := types.GetMeta(entity)
	require.NoError(t, err)
	require.Equal(t, "runner", meta.ID)
	require.Empty(t, meta.Name)
}

func TestSingletonIsFoundByItsAddress(t *testing.T) {
	path := writeSingletonConfig(t, singletonConfig)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	runner, err := Find[plain.Runner](c, "runner")
	require.NoError(t, err)
	require.Equal(t, "https://maalstrom.app", runner.Server)
}

func TestSingletonIsReferencedByItsType(t *testing.T) {
	path := writeSingletonConfig(t, singletonConfig)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	client, err := Find[plain.Client](c, "client.web")
	require.NoError(t, err)

	require.Equal(t, "https://maalstrom.app", client.Server)
	require.Equal(t, "https://maalstrom.app", client.Runner.Server)
	require.Equal(t, "127.0.0.1:7660", client.Runner.Listen)
}

func TestSingletonSurvivesSavedState(t *testing.T) {
	path := writeSingletonConfig(t, singletonConfig)
	dir := t.TempDir()

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(dir))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	loaded, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(dir))
	require.NoError(t, err)
	require.NoError(t, loaded.Load())

	runner, err := Find[plain.Runner](loaded, "runner")
	require.NoError(t, err)
	require.Equal(t, "https://maalstrom.app", runner.Server)
}

func TestSingletonWithLabelIsRefused(t *testing.T) {
	path := writeSingletonConfig(t, singletonWithLabelConfig)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)

	err = c.Validate(path)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Equal(t, "Unexpected label", d.Summary)
	require.Equal(t, `runner takes no labels, there is one of it: write it as runner { … }, without "nas".`, d.Detail)

	// placed at the label, "nas" on line 2 from column 8
	require.Equal(t, 2, d.Subject.Start.Line)
	require.Equal(t, 8, d.Subject.Start.Column)
	require.Equal(t, 13, d.Subject.End.Column)
}

func TestSingletonDeclaredTwiceIsRefused(t *testing.T) {
	path := writeSingletonConfig(t, singletonTwiceConfig)

	c, err := NewConfig(WithRegistry(singletonRegistry()), WithStatePath(t.TempDir()))
	require.NoError(t, err)

	err = c.Validate(path)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Error(), "Duplicate block")
	require.Contains(t, d.Error(), "runner is declared once")
	require.Equal(t, 6, d.Subject.Start.Line)
}

func TestNewConfigFailsOnSingletonAndSubtypeUnderOneKeyword(t *testing.T) {
	local := registry.NewLocal()
	registry.Register[plain.Runner](local, "runner")
	registry.Register[plain.Database](local, "runner", "big")

	c, err := NewConfig(WithRegistry(local))
	require.Nil(t, c)
	require.Error(t, err)

	var form *xclerrors.TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "runner", form.Type)
	require.False(t, form.TakesSubtype)
}

func TestRegisterPanicsOnASubtypeForASingleton(t *testing.T) {
	local := registry.NewLocal()

	require.PanicsWithValue(t,
		`xcl: registry local: type "runner.big": plain.Runner has no label fields and no Meta field, so its block is written without labels, and its type takes no subtype`,
		func() {
			registry.Register[plain.Runner](local, "runner", "big")
		})
}

// runnerSchemaDocument declares the runner as a singleton in a schema, as a
// host in another language does
const runnerSchemaDocument = `{"schema_version": 1, "type": "runner", "singleton": true, "fields": [
  {"name": "server", "type": "string"}
]}`

func TestSchemaTypeMarkedSingletonIsDeclaredWithoutLabels(t *testing.T) {
	local := newSchemaRegistry(t, runnerSchemaDocument)
	path := writeSingletonConfig(t, `runner { server = "https://maalstrom.app" }`)

	c, err := NewConfig(WithRegistry(local), WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	runner := findAsMap(t, c, "runner")
	require.Equal(t, "https://maalstrom.app", runner["server"])
}

func TestSchemaTypeMarkedSingletonSurvivesSavedState(t *testing.T) {
	path := writeSingletonConfig(t, `runner { server = "https://maalstrom.app" }`)
	dir := t.TempDir()

	c, err := NewConfig(WithRegistry(newSchemaRegistry(t, runnerSchemaDocument)), WithStatePath(dir))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	loaded, err := NewConfig(WithRegistry(newSchemaRegistry(t, runnerSchemaDocument)), WithStatePath(dir))
	require.NoError(t, err)
	require.NoError(t, loaded.Load())

	runner := findAsMap(t, loaded, "runner")
	require.Equal(t, "https://maalstrom.app", runner["server"])
	require.NotContains(t, runner, "XclSingleton")
}

func TestSchemaTypeMarkedSingletonTakesNoSubtype(t *testing.T) {
	_, err := registry.TypeFromSchema([]byte(`{"schema_version": 1, "type": "runner", "subtype": "big", "singleton": true, "fields": []}`))
	require.ErrorContains(t, err, "a singleton takes no subtype")
}

func TestATypeWithAMetaFieldIsNamed(t *testing.T) {
	type named struct {
		Value string `xcl:"value,optional"`
		Meta  types.Meta
	}

	local := registry.NewLocal()
	registry.Register[named](local, "named")

	path := writeSingletonConfig(t, `named "one" { value = "kept" }`)

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)
	require.NoError(t, c.Apply(path))

	found, err := Find[named](c, "named.one")
	require.NoError(t, err)
	require.Equal(t, "one", found.Meta.Name)
}
