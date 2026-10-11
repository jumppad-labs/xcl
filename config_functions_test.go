package xcl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// slugify is an application function with a signature configuration can
// call: it lower-cases a title and joins its words with dashes
func slugify(title string) (string, error) {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-")), nil
}

// shout is an application function that upper-cases its argument, so a
// sensitive argument's value is visible in the result unless the result is
// marked sensitive too
func shout(s string) (string, error) {
	return strings.ToUpper(s), nil
}

// failWithArgument is an application function whose error repeats its
// argument, as a careless function would
func failWithArgument(s string) (string, error) {
	return "", fmt.Errorf("bad value %s", s)
}

// applicationFunctionConfig calls an application function to set a service's
// host
const applicationFunctionConfig = `
service "api" {
  host = slugify("My App")
}
`

// applicationFunctionWithListConfig passes a list literal and a number to an
// application function that takes a slice and an int
const applicationFunctionWithListConfig = `
service "api" {
  host = pick(["edge", "core", "api"], 2)
}
`

// sensitiveArgumentToFailingFunctionConfig passes a secret to an application
// function that fails, repeating its argument in the error
const sensitiveArgumentToFailingFunctionConfig = `
resource "secret" "db" {
  password = "s3cr3t-value"
}

resource "secret_consumer" "app" {
  password = fail(resource.secret.db.password)
}
`

// sensitiveArgumentIntoSensitiveFieldConfig passes a secret through an
// application function into a sensitive field
const sensitiveArgumentIntoSensitiveFieldConfig = `
resource "secret" "db" {
  password = "s3cr3t-value"
}

resource "secret_consumer" "app" {
  password = shout(resource.secret.db.password)
}
`

// sensitiveArgumentIntoPlainFieldConfig passes a secret through an
// application function into a plain field
const sensitiveArgumentIntoPlainFieldConfig = `
resource "secret" "db" {
  password = "s3cr3t-value"
}

resource "secret_consumer" "app" {
  note = shout(resource.secret.db.password)
}
`

// applicationFunctionFixture is what an application function test works
// with: the registry the functions and types are registered on, the state
// store, and the path of the one configuration file
type applicationFunctionFixture struct {
	registry   *registry.Local
	store      *state.FileStateStore
	configFile string
}

// setupApplicationFunctions redirects HOME to a temp dir, registers the
// Service and secret types and creates a file state store in a temp dir. The
// test registers the functions it needs on the fixture's registry.
func setupApplicationFunctions(t *testing.T) *applicationFunctionFixture {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	local.RegisterType(&registered.Service{}, registered.TypeService)
	local.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret)
	local.RegisterType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	return &applicationFunctionFixture{
		registry:   local,
		store:      store,
		configFile: filepath.Join(t.TempDir(), "main.xcl"),
	}
}

// writeConfig replaces the fixture's configuration with contents
func (f *applicationFunctionFixture) writeConfig(t *testing.T, contents string) {
	t.Helper()

	err := os.WriteFile(f.configFile, []byte(contents), 0644)
	require.NoError(t, err)
}

// newConfig builds a Config over the fixture's registry and state store,
// returning what NewConfig returns so a test can assert on its failure
func (f *applicationFunctionFixture) newConfig(t *testing.T, registries ...registry.Registry) (*Config, error) {
	t.Helper()

	options := []ConfigOption{WithStateStore(f.store)}

	if len(registries) == 0 {
		registries = []registry.Registry{f.registry}
	}

	for _, r := range registries {
		options = append(options, WithRegistry(r))
	}

	return NewConfig(options...)
}

// applyConfig writes contents as the configuration and applies it with a
// fresh Config, returning the Config and the error Apply reported
func (f *applicationFunctionFixture) applyConfig(t *testing.T, contents string) (*Config, error) {
	t.Helper()

	f.writeConfig(t, contents)

	c, err := f.newConfig(t)
	require.NoError(t, err)

	return c, c.Apply(f.configFile)
}

// Criterion 1: an application registers a function with no plugin and
// configuration calls it, the entity holding the function's result.

func TestApplyCallsApplicationFunction(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("slugify", slugify)

	c, err := f.applyConfig(t, applicationFunctionConfig)
	require.NoError(t, err)

	service, err := Find[registered.Service](c, "service.api")
	require.NoError(t, err)
	require.Equal(t, "my-app", service.Host)
}

func TestApplyCallsApplicationFunctionWithAListLiteral(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("pick", func(names []string, index int) (string, error) {
		if index < 0 || index >= len(names) {
			return "", fmt.Errorf("index %d out of range", index)
		}

		return names[index], nil
	})

	c, err := f.applyConfig(t, applicationFunctionWithListConfig)
	require.NoError(t, err)

	service, err := Find[registered.Service](c, "service.api")
	require.NoError(t, err)
	require.Equal(t, "api", service.Host)
}

// Criterion 2: a function named like a builtin is refused by NewConfig,
// naming the function, the application function and the builtin.

func TestNewConfigReportsApplicationFunctionClashWithBuiltin(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("upper", shout)

	c, err := f.newConfig(t)
	require.Nil(t, c)
	require.Error(t, err)

	var clash *FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T: %v", err, err)
	require.Equal(t, "upper", clash.Name)
	require.True(t, strings.HasPrefix(clash.Provider, "function func("), "Provider should name the Go func, got %q", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "builtin", clash.Existing)
	require.Empty(t, clash.ExistingRegistry)

	require.Contains(t, err.Error(), `"upper"`)
	require.Contains(t, err.Error(), "builtin")
	require.Contains(t, err.Error(), "function func(string) (string, error)")
}

// Criterion 3: two application functions with the same name, in one registry
// or across two, are refused by NewConfig, naming both and their registries.

func TestNewConfigReportsApplicationFunctionClashInOneRegistry(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("slugify", slugify)
	f.registry.RegisterFunction("slugify", shout)

	c, err := f.newConfig(t)
	require.Nil(t, c)
	require.Error(t, err)

	var clash *FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T: %v", err, err)
	require.Equal(t, "slugify", clash.Name)
	require.Equal(t, "function func(string) (string, error)", clash.Provider)
	require.Equal(t, "local", clash.Registry)
	require.Equal(t, "function func(string) (string, error)", clash.Existing)
	require.Equal(t, "local", clash.ExistingRegistry)
}

func TestNewConfigReportsApplicationFunctionClashAcrossRegistries(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("slugify", slugify)

	other := registry.NewLocal()
	other.RegisterFunction("slugify", shout)

	first := namedRegistry{Registry: f.registry, name: "first"}
	second := namedRegistry{Registry: other, name: "second"}

	c, err := f.newConfig(t, first, second)
	require.Nil(t, c)
	require.Error(t, err)

	var clash *FunctionNameClashError
	require.True(t, errors.As(err, &clash), "expected a *FunctionNameClashError, got %T: %v", err, err)
	require.Equal(t, "slugify", clash.Name)
	require.Equal(t, "function func(string) (string, error)", clash.Provider)
	require.Equal(t, "second", clash.Registry)
	require.Equal(t, "function func(string) (string, error)", clash.Existing)
	require.Equal(t, "first", clash.ExistingRegistry)

	require.Contains(t, err.Error(), "registry first")
	require.Contains(t, err.Error(), "registry second")
}

// Criterion 4: a function whose signature configuration cannot call is
// refused by NewConfig with an error naming it.

func TestNewConfigReportsInvalidApplicationFunction(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("broken", func(s string) string { return s })

	c, err := f.newConfig(t)
	require.Nil(t, c)
	require.Error(t, err)

	var invalid *InvalidFunctionError
	require.True(t, errors.As(err, &invalid), "expected an *InvalidFunctionError, got %T: %v", err, err)
	require.Equal(t, "broken", invalid.Name)
	require.Equal(t, "local", invalid.Registry)

	require.Contains(t, err.Error(), `"broken"`)
	require.Contains(t, err.Error(), "must return a result and an error")
}

func TestNewConfigReportsApplicationFunctionWithUnsupportedParameter(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("broken", func(x struct{}) (string, error) { return "", nil })

	c, err := f.newConfig(t)
	require.Nil(t, c)
	require.Error(t, err)

	var invalid *InvalidFunctionError
	require.True(t, errors.As(err, &invalid), "expected an *InvalidFunctionError, got %T: %v", err, err)
	require.Equal(t, "broken", invalid.Name)
	require.Equal(t, "local", invalid.Registry)

	require.Contains(t, err.Error(), `"broken"`)
	require.Contains(t, err.Error(), "parameter 0")
	require.Contains(t, err.Error(), "is not supported")
}

// Criterion 5: a sensitive argument to an application function is kept out
// of the function's error, and the function's result is sensitive when an
// argument is.

func TestApplicationFunctionErrorHidesSensitiveArgument(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("fail", failWithArgument)

	_, err := f.applyConfig(t, sensitiveArgumentToFailingFunctionConfig)
	require.Error(t, err)

	require.Contains(t, err.Error(), "(sensitive)")
	require.NotContains(t, err.Error(), "s3cr3t-value")
	require.NotContains(t, err.Error(), "S3CR3T-VALUE")
}

func TestApplicationFunctionResultIsSensitiveWhenAnArgumentIs(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("shout", shout)

	c, err := f.applyConfig(t, sensitiveArgumentIntoSensitiveFieldConfig)
	require.NoError(t, err)

	consumer, err := Find[registered.SecretConsumer](c, "resource.secret_consumer.app")
	require.NoError(t, err)
	require.Equal(t, "S3CR3T-VALUE", consumer.Password.Reveal())
}

func TestApplicationFunctionResultWithASensitiveArgumentIsRejectedIntoAPlainField(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("shout", shout)

	f.writeConfig(t, sensitiveArgumentIntoPlainFieldConfig)

	c, err := f.newConfig(t)
	require.NoError(t, err)

	err = c.Validate(f.configFile)
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.app")
}

func TestApplyCreatesNothingWhenApplicationFunctionResultWithASensitiveArgumentIsAssignedToAPlainField(t *testing.T) {
	f := setupApplicationFunctions(t)
	f.registry.RegisterFunction("shout", shout)

	c, err := f.applyConfig(t, sensitiveArgumentIntoPlainFieldConfig)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "s3cr3t-value")
	require.NotContains(t, err.Error(), "S3CR3T-VALUE")

	_, err = Find[registered.SecretConsumer](c, "resource.secret_consumer.app")
	require.Error(t, err)
}
