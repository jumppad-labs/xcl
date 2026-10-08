package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// applySensitiveFixture registers the secret types and applies the named
// fixture under config/sensitive, returning the configuration and the error
// the apply produced.
func applySensitiveFixture(t *testing.T, name string) (*Config, error) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(
		WithType(&registered.Secret{}, "resource", registered.TypeSecret),
		WithType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer),
		WithStateStore(store),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive/" + name + "/main.xcl")
	require.NoError(t, err)

	return c, c.Apply(path)
}

func TestSensitiveFieldSetFromLiteralRevealsValue(t *testing.T) {
	c, err := applySensitiveFixture(t, "basic")
	require.NoError(t, err)

	secret, err := Find[registered.Secret](c, "resource.secret.literal")
	require.NoError(t, err)

	require.Equal(t, "admin", secret.Username)
	require.Equal(t, "from-literal", secret.Password.Reveal())
}

func TestSensitiveFieldSetFromVariableRevealsValue(t *testing.T) {
	c, err := applySensitiveFixture(t, "basic")
	require.NoError(t, err)

	secret, err := Find[registered.Secret](c, "resource.secret.from_variable")
	require.NoError(t, err)

	require.Equal(t, "from-variable", secret.Password.Reveal())
}

func TestSensitiveFieldSetFromReferenceRevealsValue(t *testing.T) {
	c, err := applySensitiveFixture(t, "basic")
	require.NoError(t, err)

	consumer, err := Find[registered.SecretConsumer](c, "resource.secret_consumer.reference")
	require.NoError(t, err)

	require.Equal(t, "from-literal", consumer.Password.Reveal())
}

func TestSensitiveValueReadIntoPlainFieldFailsNamingTheField(t *testing.T) {
	_, err := applySensitiveFixture(t, "plain_field")
	require.Error(t, err)

	// validation refuses it before anything is decoded, naming the field
	require.Contains(t, err.Error(), `field "note" of resource.secret_consumer.plain`)
	require.Contains(t, err.Error(), "not declared sensitive")
}

func TestSensitiveValuePassedThroughModuleInputRevealsInsideModule(t *testing.T) {
	c, err := applySensitiveFixture(t, "module_input")
	require.NoError(t, err)

	consumer, err := Find[registered.SecretConsumer](c, "module.child.resource.secret_consumer.inner")
	require.NoError(t, err)

	require.Equal(t, "from-literal", consumer.Password.Reveal())
}
