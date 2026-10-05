package testing

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/schema"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

type credentialFixture struct {
	types.ResourceBase `xcl:",remain"`

	Username string                  `xcl:"username,optional" json:"username,omitempty"`
	Password types.Sensitive[string] `xcl:"password" json:"password"`
	Pin      types.Sensitive[int]    `xcl:"pin,optional" json:"pin"`
}

func TestParseHCLFileDecodesSensitiveFields(t *testing.T) {
	pluginSchema, err := schema.GenerateSchemaFromInstance(&credentialFixture{}, 10)
	require.NoError(t, err)

	result := ParseHCLFile(t, "testdata/credential.xcl", pluginSchema, credentialFixture{})

	require.Equal(t, 1, result.Count)
	require.Equal(t, "admin", result.Objects[0].Username)
	require.Equal(t, "s3cret", result.Objects[0].Password.Reveal())
	require.Equal(t, 1234, result.Objects[0].Pin.Reveal())
}
