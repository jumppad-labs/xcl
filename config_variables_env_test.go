package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/stretchr/testify/require"
)

// regionNetworkConfig declares a variable with a default and a network whose
// subnet is that variable, so a test can see which value the variable held
// when the network was applied
const regionNetworkConfig = `
variable "region" {
  default = "eu"
}

resource "network" "main" {
  subnet = variable.region
}
`

// pathReferenceConfig refers to a variable the configuration never declares.
// PATH is set in every process environment, so this is exactly the case where
// an unprefixed environment variable must not leak in as a variable
const pathReferenceConfig = `
resource "network" "main" {
  subnet = variable.PATH
}
`

func TestApplyOverridesVariableFromXCLVarPrefixedEnvironment(t *testing.T) {
	t.Setenv("XCL_VAR_region", "us")

	f := setupDeliveryConfig(t, regionNetworkConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	network, err := Find[structs.Network](f.config, "resource.network.main")
	require.NoError(t, err)
	require.Equal(t, "us", network.Subnet)
}

func TestApplyIgnoresUnprefixedEnvironmentVariable(t *testing.T) {
	t.Setenv("region", "us")

	f := setupDeliveryConfig(t, regionNetworkConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	network, err := Find[structs.Network](f.config, "resource.network.main")
	require.NoError(t, err)
	require.Equal(t, "eu", network.Subnet)
}

func TestApplyIgnoresHCLVarPrefixedEnvironmentVariable(t *testing.T) {
	t.Setenv("HCL_VAR_region", "us")

	f := setupDeliveryConfig(t, regionNetworkConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	network, err := Find[structs.Network](f.config, "resource.network.main")
	require.NoError(t, err)
	require.Equal(t, "eu", network.Subnet)
}

func TestValidateReportsEnvironmentVariableWithoutPrefixAsUnknownVariable(t *testing.T) {
	// the point of the test is that PATH is present in the environment and
	// must still not be visible as variable.PATH
	_, present := os.LookupEnv("PATH")
	require.True(t, present, "PATH must be set in the environment for this test")

	f := setupDeliveryConfig(t, pathReferenceConfig)

	err := f.config.Validate(f.configFile)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.Len(t, ce.Errors, 1)

	pe, ok := ce.Errors[0].(*errors.Diagnostic)
	require.True(t, ok, "the problem should be a *errors.Diagnostic")
	require.Equal(t, "resource 'resource.network.main' refers to 'variable.PATH', which is not defined anywhere in the configuration", pe.Detail)
}

func TestDiffReadsPrefixedEnvironmentVariable(t *testing.T) {
	t.Setenv("XCL_VAR_region", "us")

	f := setupDeliveryConfig(t, regionNetworkConfig)

	result, err := f.config.Diff([]string{filepath.Dir(f.configFile)})
	require.NoError(t, err)

	require.Equal(t, []string{"resource.network.main create"}, resourceActions(result))

	network := diffResourceFor(t, result, "resource.network.main")

	var subnet *diff.Change
	for i := range network.Changes {
		if network.Changes[i].Path.String() == "subnet" {
			subnet = &network.Changes[i]
		}
	}
	require.NotNil(t, subnet, "expected a subnet change, got: %v", changePaths(network.Changes))
	require.Nil(t, subnet.Before)
	require.Equal(t, "us", subnet.After)
}
