package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// newSensitiveCheckConfig returns a config with the secret types registered
func newSensitiveCheckConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	declared := registry.NewLocal()
	declared.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret)
	declared.RegisterType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer)
	declared.RegisterType(&registered.SecretShape{}, "resource", registered.TypeSecretShapes)

	c, err := NewConfig(
		WithRegistry(declared),
		WithStateStore(store),
	)
	require.NoError(t, err)

	return c
}

func sensitiveCheckPath(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive_check/" + name + "/main.xcl")
	require.NoError(t, err)

	return path
}

// validateSensitiveCheck validates the named fixture under config/sensitive_check
func validateSensitiveCheck(t *testing.T, name string) error {
	t.Helper()

	return newSensitiveCheckConfig(t).Validate(sensitiveCheckPath(t, name))
}

// requireSensitiveRejected asserts err is a ConfigError naming field and entity
func requireSensitiveRejected(t *testing.T, err error, field string, entity string) {
	t.Helper()

	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)

	// the message is wrapped for display, so check each part on its own
	require.Contains(t, err.Error(), `field "`+field+`"`)
	require.Contains(t, err.Error(), entity)
}

func TestValidateRejectsSensitiveFieldReferencedIntoPlainField(t *testing.T) {
	err := validateSensitiveCheck(t, "direct_plain")
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.c")
}

func TestValidateRejectsSensitiveFieldInterpolatedIntoPlainField(t *testing.T) {
	err := validateSensitiveCheck(t, "template_plain")
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.c")
}

func TestValidateRejectsSensitiveFieldPassedThroughFunctionIntoPlainField(t *testing.T) {
	err := validateSensitiveCheck(t, "function_plain")
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.c")
}

func TestValidateRejectsSensitiveFieldInObjectConstructorIntoPlainMap(t *testing.T) {
	err := validateSensitiveCheck(t, "object_plain")
	requireSensitiveRejected(t, err, "tags[k]", "resource.secret_shape.c")
}

func TestValidateRejectsSensitiveFieldInTupleIntoPlainList(t *testing.T) {
	err := validateSensitiveCheck(t, "tuple_plain")
	requireSensitiveRejected(t, err, "items[1]", "resource.secret_shape.c")
}

func TestValidateRejectsChildModuleSensitiveOutputUsedInPlainField(t *testing.T) {
	err := validateSensitiveCheck(t, "module_output_plain")
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.c")
}

func TestValidateRejectsSensitiveModuleInputUsedInPlainFieldInsideModule(t *testing.T) {
	err := validateSensitiveCheck(t, "module_input_plain")
	requireSensitiveRejected(t, err, "note", "module.child.resource.secret_consumer.inner")
}

func TestValidateRejectsConditionalSensitiveValueIntoPlainField(t *testing.T) {
	err := validateSensitiveCheck(t, "conditional_plain")
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.c")
}

func TestValidateRejectsSensitivePartOfObjectIntoPlainStructField(t *testing.T) {
	err := validateSensitiveCheck(t, "object_mixed_struct_plain")
	requireSensitiveRejected(t, err, "login.user", "resource.secret_shape.c")
}

func TestValidateAcceptsSensitiveFieldReferencedIntoSensitiveField(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "direct_sensitive"))
}

func TestValidateAcceptsSensitiveFieldInterpolatedIntoSensitiveField(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "template_sensitive"))
}

func TestValidateAcceptsSensitiveFieldPassedThroughFunctionIntoSensitiveField(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "function_sensitive"))
}

func TestValidateAcceptsSensitiveFieldInObjectConstructorIntoSensitiveMap(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "object_sensitive"))
}

func TestValidateAcceptsChildModuleSensitiveOutputIntoSensitiveField(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "module_output_sensitive"))
}

func TestValidateAcceptsPlainValuesIntoSensitiveField(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "plain_into_sensitive"))
}

func TestValidateAcceptsSensitiveValueIntoOutputValue(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "into_output"))
}

func TestValidateAcceptsObjectWithSensitivePartIntoStructWithSensitiveElement(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "object_mixed_struct"))
}

func TestValidateAcceptsPlainReferenceIntoPlainField(t *testing.T) {
	require.NoError(t, validateSensitiveCheck(t, "plain_into_plain"))
}

func TestApplyCreatesNothingWhenSensitiveValueAssignedToPlainField(t *testing.T) {
	c := newSensitiveCheckConfig(t)

	err := c.Apply(sensitiveCheckPath(t, "direct_plain"))
	requireSensitiveRejected(t, err, "note", "resource.secret_consumer.c")

	require.Equal(t, 0, c.ResourceCount())
}
