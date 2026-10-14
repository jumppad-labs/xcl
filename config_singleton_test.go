package xcl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/jumppad-labs/xcl/registry"
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

// singletonRegistry declares the runner as a singleton and the client beside
// it
func singletonRegistry() *registry.Local {
	local := registry.NewLocal()
	registry.RegisterSingleton[plain.Runner](local, "runner")
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
	require.Equal(t, "runner", runner.Meta.ID)
	require.Empty(t, runner.Meta.Name)
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
	require.Contains(t, d.Error(), "Invalid block labels")
	require.Contains(t, d.Error(), "runner {}")
	require.Equal(t, 2, d.Subject.Start.Line)
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
	registry.RegisterSingleton[plain.Runner](local, "runner")
	local.RegisterType(&plain.Database{}, "runner", "big")

	c, err := NewConfig(WithRegistry(local))
	require.Nil(t, c)
	require.Error(t, err)

	var form *xclerrors.TypeFormError
	require.True(t, errors.As(err, &form))
	require.Equal(t, "runner", form.Type)
	require.False(t, form.TakesSubtype)
}

func TestRegisterSingletonPanicsOnLabelField(t *testing.T) {
	local := registry.NewLocal()

	require.Panics(t, func() {
		registry.RegisterSingleton[plain.Agent](local, "runner")
	})
}
