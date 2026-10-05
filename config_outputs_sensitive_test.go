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

// applySensitiveOutputs applies the named fixture under
// config/sensitive_outputs and returns the configuration and the store the
// state was written to.
func applySensitiveOutputs(t *testing.T, name string) (*Config, *state.FileStateStore) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(WithPluginRegistry(newSecretRegistry(t)), WithStateStore(store))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive_outputs/" + name + "/main.xcl")
	require.NoError(t, err)

	require.NoError(t, c.Apply(path))

	return c, store
}

// reloadOutput reads the state the first apply wrote and returns the output
// entity with the given ID.
func reloadOutput(t *testing.T, store *state.FileStateStore, id string) *types.Output {
	t.Helper()

	records, err := store.Load()
	require.NoError(t, err)

	loaded, err := savedentity.DecodeAll(newSecretRegistry(t), records, savedentity.ReadOptions{})
	require.NoError(t, err)

	for _, entity := range loaded {
		output, ok := entity.(*types.Output)
		if ok && output.Meta.ID == id {
			return output
		}
	}

	require.Fail(t, "output not found in loaded state", id)

	return nil
}

func TestOutputWithPartlySensitiveEntityHoldsSensitiveField(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "partial")

	output, err := Find[types.Output](c, "output.x")
	require.NoError(t, err)

	value, ok := output.Value.(map[string]any)
	require.True(t, ok, "value is %T", output.Value)

	password, ok := value["password"].(types.Sensitive[string])
	require.True(t, ok, "password is %T", value["password"])
	require.Equal(t, "hunter2", password.Reveal())
}

func TestOutputWithPartlySensitiveEntityHoldsPlainFieldAsString(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "partial")

	output, err := Find[types.Output](c, "output.x")
	require.NoError(t, err)

	value, ok := output.Value.(map[string]any)
	require.True(t, ok, "value is %T", output.Value)
	require.Equal(t, "admin", value["username"])
}

func TestOutputWithPartlySensitiveEntityRecordsSensitivePath(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "partial")

	output, err := Find[types.Output](c, "output.x")
	require.NoError(t, err)

	require.Equal(t, [][]string{{"password"}}, output.SensitivePaths)
}

func TestOutputWithSensitiveValueHoldsSensitiveString(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "whole")

	output, err := Find[types.Output](c, "output.x")
	require.NoError(t, err)

	value, ok := output.Value.(types.Sensitive[string])
	require.True(t, ok, "value is %T", output.Value)
	require.Equal(t, "hunter2", value.Reveal())
}

func TestOutputWithSensitiveValueRecordsEmptyPath(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "whole")

	output, err := Find[types.Output](c, "output.x")
	require.NoError(t, err)

	require.Equal(t, [][]string{{}}, output.SensitivePaths)
}

func TestReloadedPartlySensitiveOutputHoldsSensitiveField(t *testing.T) {
	_, store := applySensitiveOutputs(t, "partial")

	output := reloadOutput(t, store, "output.x")

	value, ok := output.Value.(map[string]any)
	require.True(t, ok, "value is %T", output.Value)

	password, ok := value["password"].(types.Sensitive[string])
	require.True(t, ok, "password is %T", value["password"])
	require.Equal(t, "hunter2", password.Reveal())
	require.Equal(t, "admin", value["username"])
	require.Equal(t, [][]string{{"password"}}, output.SensitivePaths)
}

func TestReloadedWhollySensitiveOutputHoldsSensitiveString(t *testing.T) {
	_, store := applySensitiveOutputs(t, "whole")

	output := reloadOutput(t, store, "output.x")

	value, ok := output.Value.(types.Sensitive[string])
	require.True(t, ok, "value is %T", output.Value)
	require.Equal(t, "hunter2", value.Reveal())
	require.Equal(t, [][]string{{}}, output.SensitivePaths)
}

func TestOutputEntityMarshalsSensitiveValueRedacted(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "whole")

	output, err := Find[types.Output](c, "output.x")
	require.NoError(t, err)

	data, err := json.Marshal(output)
	require.NoError(t, err)

	require.Contains(t, string(data), "(sensitive)")
	require.NotContains(t, string(data), "hunter2")
}

func TestStateFileHoldsRealOutputValueAndSensitivePaths(t *testing.T) {
	_, store := applySensitiveOutputs(t, "whole")

	contents, err := os.ReadFile(store.Path())
	require.NoError(t, err)

	require.Contains(t, string(contents), "hunter2")
	require.Contains(t, string(contents), "sensitive_paths")
}

func TestSensitiveModuleOutputReachesCallerSensitiveField(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "module")

	consumer, err := Find[registered.SecretConsumer](c, "resource.secret_consumer.c")
	require.NoError(t, err)

	require.Equal(t, "hunter2", consumer.Password.Reveal())
}

func TestSensitiveModuleOutputHoldsSensitiveValue(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "module")

	output, err := Find[types.Output](c, "module.child.output.pw")
	require.NoError(t, err)

	value, ok := output.Value.(types.Sensitive[string])
	require.True(t, ok, "value is %T", output.Value)
	require.Equal(t, "hunter2", value.Reveal())
}

func TestSensitiveValueInterpolatedIntoStringReachesSensitiveField(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "interpolated")

	consumer, err := Find[registered.SecretConsumer](c, "resource.secret_consumer.c")
	require.NoError(t, err)

	require.Equal(t, "x-hunter2-y", consumer.Password.Reveal())
}

func TestSensitiveValuePassedThroughFunctionReachesSensitiveField(t *testing.T) {
	c, _ := applySensitiveOutputs(t, "function")

	consumer, err := Find[registered.SecretConsumer](c, "resource.secret_consumer.c")
	require.NoError(t, err)

	require.Equal(t, "HUNTER2", consumer.Password.Reveal())
}
