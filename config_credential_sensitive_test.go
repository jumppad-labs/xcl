package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/stretchr/testify/require"
)

// TestApplyKeepsSensitivePluginFieldsRevealable asserts that a plugin type
// with types.Sensitive fields, rebuilt on the host from its schema, decodes
// the configured values and the resulting entity can reveal them.
func TestApplyKeepsSensitivePluginFieldsRevealable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credential.xcl")
	err := os.WriteFile(path, []byte(`
resource "credential" "db" {
  username = "admin"
  password = "s3cret"
  pin      = 1234
}
`), 0o600)
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Apply(path)
	require.NoError(t, err)

	credential, err := Find[structs.Credential](c, "resource.credential.db")
	require.NoError(t, err)

	require.Equal(t, "admin", credential.Username)
	require.Equal(t, "s3cret", credential.Password.Reveal())
	require.Equal(t, 1234, credential.Pin.Reveal())
}
