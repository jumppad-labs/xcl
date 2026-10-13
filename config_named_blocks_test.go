package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// namedBlocksConfig declares the spec's app with two named listener blocks,
// collected by name into the WebApp's Listeners map
const namedBlocksConfig = `
app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener "http" {
    port = 80
  }

  listener "https" {
    port = 443
  }
}
`

// namedBlocksChangedConfig is namedBlocksConfig with the https listener's
// port changed and nothing else
const namedBlocksChangedConfig = `
app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener "http" {
    port = 80
  }

  listener "https" {
    port = 8443
  }
}
`

// unnamedBlockIntoMapConfig writes a listener without a name into a field
// that collects listeners by name
const unnamedBlockIntoMapConfig = `
app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener {
    port = 80
  }
}
`

// duplicateBlockNameConfig writes two listeners carrying the same name
const duplicateBlockNameConfig = `
app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener "http" {
    port = 80
  }

  listener "http" {
    port = 8080
  }
}
`

// namedAndUnnamedBlocksIntoListConfig writes a named and an unnamed listener
// into a Service, whose listeners are a list rather than a map
const namedAndUnnamedBlocksIntoListConfig = `
service "api" {
  host = "api.local"

  listener "http" {
    port = 80
  }

  listener {
    port = 8080
  }
}
`

// referenceToNamedBlockConfig reads one named listener of the app by name
// from a gateway
const referenceToNamedBlockConfig = namedBlocksConfig + `
gateway "edge" {
  port = app.web.listener.https.port
}
`

// forOverNamedBlocksConfig builds a gateway's ports with a for expression
// over the app's listeners
const forOverNamedBlocksConfig = namedBlocksConfig + `
gateway "edge" {
  ports = { for name, l in app.web.listener : name => l.port }
}
`

// referenceInsideNamedBlockConfig sets the https listener's port from a
// variable, so the reference is recorded against the named block
const referenceInsideNamedBlockConfig = `
variable "https_port" {
  default = 443
}

app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener "http" {
    port = 80
  }

  listener "https" {
    port = variable.https_port
  }
}
`

// namedBlocksFixture is what a named block test works with: the registry
// holding the registered types, the state store every Config shares, the
// directory holding the configuration and the path of its one file
type namedBlocksFixture struct {
	registry   *registry.Local
	store      *state.FileStateStore
	configDir  string
	configFile string
}

// setupNamedBlocks redirects HOME to a temp dir, registers the WebApp,
// Gateway and Service types and creates a file state store in a temp dir
func setupNamedBlocks(t *testing.T) *namedBlocksFixture {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	local := registry.NewLocal()
	local.RegisterType(&registered.WebApp{}, registered.TypeWebApp)
	local.RegisterType(&registered.Gateway{}, registered.TypeGateway)
	local.RegisterType(&registered.Service{}, registered.TypeService)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	configDir := t.TempDir()

	return &namedBlocksFixture{
		registry:   local,
		store:      store,
		configDir:  configDir,
		configFile: filepath.Join(configDir, "main.xcl"),
	}
}

// writeConfig replaces the fixture's configuration with contents
func (f *namedBlocksFixture) writeConfig(t *testing.T, contents string) {
	t.Helper()

	err := os.WriteFile(f.configFile, []byte(contents), 0644)
	require.NoError(t, err)
}

// newConfig builds a fresh Config over the fixture's registry and state
// store, as a separate run would
func (f *namedBlocksFixture) newConfig(t *testing.T) *Config {
	t.Helper()

	c, err := NewConfig(WithRegistry(f.registry), WithStateStore(f.store))
	require.NoError(t, err)

	return c
}

// applyConfig writes contents as the configuration and applies it with a
// fresh Config, which it returns
func (f *namedBlocksFixture) applyConfig(t *testing.T, contents string) *Config {
	t.Helper()

	f.writeConfig(t, contents)

	c := f.newConfig(t)

	err := c.Apply(f.configFile)
	require.NoError(t, err)

	return c
}

// requireParserError returns the one ParserError a failed operation reports
func requireDiagnostic(t *testing.T, err error) *errors.Diagnostic {
	t.Helper()

	configError, ok := err.(*errors.ConfigError)
	require.True(t, ok, "expected a *errors.ConfigError, got %T: %v", err, err)
	require.Len(t, configError.Errors, 1)

	diagnostic, ok := configError.Errors[0].(*errors.Diagnostic)
	require.True(t, ok, "expected a *errors.Diagnostic, got %T: %v", configError.Errors[0], configError.Errors[0])

	return diagnostic
}

func TestApplyCollectsNamedBlocksIntoMap(t *testing.T) {
	f := setupNamedBlocks(t)
	c := f.applyConfig(t, namedBlocksConfig)

	app, err := Find[registered.WebApp](c, "app.web")
	require.NoError(t, err)

	require.Len(t, app.Listeners, 2)
	require.Equal(t, "http", app.Listeners["http"].Name)
	require.Equal(t, 80, app.Listeners["http"].Port)
	require.Equal(t, "https", app.Listeners["https"].Name)
	require.Equal(t, 443, app.Listeners["https"].Port)
}

// Validate checks the configuration without decoding it, so a listener
// without a name is only reported once the body is decoded, by Apply
func TestValidateRejectsUnnamedBlockIntoMapAtTheBlock(t *testing.T) {
	f := setupNamedBlocks(t)
	f.writeConfig(t, unnamedBlockIntoMapConfig)

	c := f.newConfig(t)

	err := c.Validate(f.configFile)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Error(), "A name is required")
	require.Contains(t, d.Error(), "listener blocks are collected by name")

	// the error is placed at the unnamed listener block, line 6 column 3
	require.Equal(t, f.configFile, d.Subject.Filename)
	require.Equal(t, 6, d.Subject.Start.Line)
	require.Equal(t, 3, d.Subject.Start.Column)
}

func TestApplyRejectsUnnamedBlockIntoMap(t *testing.T) {
	f := setupNamedBlocks(t)
	f.writeConfig(t, unnamedBlockIntoMapConfig)

	c := f.newConfig(t)

	err := c.Apply(f.configFile)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Error(), "A name is required")
	require.Equal(t, 6, d.Subject.Start.Line)
}

func TestValidateRejectsDuplicateBlockNameAtTheSecondBlock(t *testing.T) {
	f := setupNamedBlocks(t)
	f.writeConfig(t, duplicateBlockNameConfig)

	c := f.newConfig(t)

	err := c.Validate(f.configFile)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Error(), "Duplicate listener name")
	require.Contains(t, d.Error(), `A listener named "http" was already defined at `+f.configFile+":6,3")

	// the error is placed at the second block's name, line 10 column 12
	require.Equal(t, 10, d.Subject.Start.Line)
	require.Equal(t, 12, d.Subject.Start.Column)
}

func TestApplyRejectsDuplicateBlockName(t *testing.T) {
	f := setupNamedBlocks(t)
	f.writeConfig(t, duplicateBlockNameConfig)

	c := f.newConfig(t)

	err := c.Apply(f.configFile)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Contains(t, d.Error(), "Duplicate listener name")
	require.Equal(t, 10, d.Subject.Start.Line)
}

func TestApplyCollectsNamedAndUnnamedBlocksIntoList(t *testing.T) {
	f := setupNamedBlocks(t)
	c := f.applyConfig(t, namedAndUnnamedBlocksIntoListConfig)

	service, err := Find[registered.Service](c, "service.api")
	require.NoError(t, err)

	require.Len(t, service.Listeners, 2)
	require.Equal(t, "http", service.Listeners[0].Name)
	require.Equal(t, 80, service.Listeners[0].Port)
	require.Equal(t, "", service.Listeners[1].Name)
	require.Equal(t, 8080, service.Listeners[1].Port)
}

func TestApplyResolvesReferenceToNamedBlock(t *testing.T) {
	f := setupNamedBlocks(t)
	c := f.applyConfig(t, referenceToNamedBlockConfig)

	gateway, err := Find[registered.Gateway](c, "gateway.edge")
	require.NoError(t, err)

	require.Equal(t, 443, gateway.Port)

	meta, err := types.GetMeta(gateway)
	require.NoError(t, err)

	require.Equal(t, []string{"app.web.listener.https.port"}, meta.Links)
	require.Equal(t, "app.web.listener.https.port", meta.References["port"])
}

func TestApplyEvaluatesForOverNamedBlocks(t *testing.T) {
	f := setupNamedBlocks(t)
	c := f.applyConfig(t, forOverNamedBlocksConfig)

	gateway, err := Find[registered.Gateway](c, "gateway.edge")
	require.NoError(t, err)

	require.Equal(t, map[string]int{"http": 80, "https": 443}, gateway.Ports)

	meta, err := types.GetMeta(gateway)
	require.NoError(t, err)

	require.Equal(t, []string{"app.web.listener"}, meta.Links)
}

func TestApplyRecordsReferenceInsideNamedBlockByName(t *testing.T) {
	f := setupNamedBlocks(t)
	c := f.applyConfig(t, referenceInsideNamedBlockConfig)

	app, err := Find[registered.WebApp](c, "app.web")
	require.NoError(t, err)

	require.Equal(t, 443, app.Listeners["https"].Port)

	meta, err := types.GetMeta(app)
	require.NoError(t, err)

	require.Equal(t, map[string]string{`listener["https"].port`: "variable.https_port"}, meta.References)
	require.Equal(t, []string{"variable.https_port"}, meta.Links)
}

func TestEncodeEntityWritesReferenceBackInsideNamedBlock(t *testing.T) {
	f := setupNamedBlocks(t)
	c := f.applyConfig(t, referenceInsideNamedBlockConfig)

	app, err := Find[registered.WebApp](c, "app.web")
	require.NoError(t, err)

	out, err := EncodeEntity(app, ShowReferences())
	require.NoError(t, err)

	expected := `app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener "http" {
    port = 80
  }

  listener "https" {
    port = variable.https_port
  }
}
`

	require.Equal(t, expected, string(out))
}

func TestSecondApplyWithNamedBlocksReportsNoChanges(t *testing.T) {
	f := setupNamedBlocks(t)
	f.applyConfig(t, namedBlocksConfig)

	saved, err := os.ReadFile(f.store.Path())
	require.NoError(t, err)

	second := f.newConfig(t)

	result, err := second.Diff([]string{f.configDir})
	require.NoError(t, err)

	require.Empty(t, result.Resources)
	require.Equal(t, 0, result.Changed())
	require.Equal(t, 0, result.Summary.Create)
	require.Equal(t, 0, result.Summary.Update)
	require.Equal(t, 0, result.Summary.Replace)
	require.Equal(t, 0, result.Summary.Delete)

	// applying again saves exactly what the first apply saved
	err = second.Apply(f.configFile)
	require.NoError(t, err)

	savedAgain, err := os.ReadFile(f.store.Path())
	require.NoError(t, err)
	require.Equal(t, string(saved), string(savedAgain))
}

func TestSecondApplyChangesOnlyTheNamedBlockThatChanged(t *testing.T) {
	f := setupNamedBlocks(t)
	f.applyConfig(t, namedBlocksConfig)

	second := f.applyConfig(t, namedBlocksChangedConfig)

	app, err := Find[registered.WebApp](second, "app.web")
	require.NoError(t, err)

	require.Len(t, app.Listeners, 2)
	require.Equal(t, 8443, app.Listeners["https"].Port)
	require.Equal(t, "https", app.Listeners["https"].Name)
	require.Equal(t, 80, app.Listeners["http"].Port)
	require.Equal(t, "http", app.Listeners["http"].Name)
}
