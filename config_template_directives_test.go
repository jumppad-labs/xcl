package xcl

import (
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/stretchr/testify/require"
)

// templateDirectiveConfig uses a %{ if } directive inside a quoted string,
// which XCL rejects because it has no string templating
const templateDirectiveConfig = `resource "network" "main" {
  subnet = "%{ if true }10.0.0.0/16%{ endif }"
}
`

// interpolatedReferenceConfig builds a string from a reference inside ${ },
// which is interpolation and stays supported
const interpolatedReferenceConfig = `resource "network" "main" {
  subnet = "10.0.0.0/16"
}

resource "network" "mirror" {
  subnet = "${resource.network.main.subnet}"
}
`

func TestValidateRejectsTemplateDirectiveAtItsPosition(t *testing.T) {
	fixture := setupDeliveryConfig(t, templateDirectiveConfig)

	err := fixture.config.Validate(fixture.configFile)
	require.Error(t, err)

	ce, ok := err.(*errors.ConfigError)
	require.True(t, ok, "Validate should report failure as a *errors.ConfigError")
	require.NotEmpty(t, ce.Errors)

	d, ok := ce.Errors[0].(*errors.Diagnostic)
	require.True(t, ok, "the problem should be a *errors.Diagnostic, got %T: %v", ce.Errors[0], ce.Errors[0])

	require.Equal(t, 2, d.Subject.Start.Line, "the directive is on the second line of the file")
	require.Equal(t, 13, d.Subject.Start.Column, "the directive follows the opening quote of the subnet value")
	require.Contains(t, d.Error(), "XCL has no string templating")
	require.Contains(t, d.Error(), "format")
}

func TestApplyFillsInterpolatedReference(t *testing.T) {
	fixture := setupDeliveryConfig(t, interpolatedReferenceConfig)

	err := fixture.config.Apply(fixture.configFile)
	require.NoError(t, err)

	mirror, err := Find[structs.Network](fixture.config, "resource.network.mirror")
	require.NoError(t, err)
	require.Equal(t, "10.0.0.0/16", mirror.Subnet)
}
