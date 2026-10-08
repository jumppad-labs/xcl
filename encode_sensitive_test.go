package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// applyEncodeSensitiveConfig applies the given configuration text with a file
// state store and returns the applied configuration and the path the state was
// written to
func applyEncodeSensitiveConfig(t *testing.T, text string) (*Config, string) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	local.RegisterPlugin(&parser.TestPlugin{})

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	local.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret)
	local.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)

	c, err := NewConfig(
		WithRegistry(local),
		WithStateStore(store),
	)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "main.xcl")
	err = os.WriteFile(path, []byte(text), 0o600)
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c, store.Path()
}

const encodeSensitiveSecretConfig = `
resource "secret" "login" {
  username = "admin"
  password = "hunter2-value"
}
`

const encodeSensitiveCredentialConfig = `
resource "credential" "db" {
  username = "admin"
  password = "pw-value"
  pin      = 4821
}
`

func TestEncodeEntityShowsSensitiveAsMarkerByDefault(t *testing.T) {
	c, _ := applyEncodeSensitiveConfig(t, encodeSensitiveSecretConfig)

	secret := encodeEntityByID(t, c, "resource.secret.login")

	out, err := EncodeEntity(secret)
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, text)
	require.Contains(t, text, `"admin"`)
	require.NotContains(t, text, "hunter2-value")
}

func TestEncodeEntityRevealsSensitiveWhenAsked(t *testing.T) {
	c, _ := applyEncodeSensitiveConfig(t, encodeSensitiveSecretConfig)

	secret := encodeEntityByID(t, c, "resource.secret.login")

	out, err := EncodeEntity(secret, RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "hunter2-value"`, text)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeSavedEntityShowsSensitiveAsMarkerByDefault(t *testing.T) {
	c, statePath := applyEncodeSensitiveConfig(t, encodeSensitiveSecretConfig)

	record := encodeSavedRecordByID(t, statePath, "resource.secret.login")

	out, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, text)
	require.NotContains(t, text, "hunter2-value")
}

func TestEncodeSavedEntityRevealsSensitiveFromStateWhenAsked(t *testing.T) {
	c, statePath := applyEncodeSensitiveConfig(t, encodeSensitiveSecretConfig)

	record := encodeSavedRecordByID(t, statePath, "resource.secret.login")

	out, err := c.EncodeSavedEntity(record, RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `password\s+= "hunter2-value"`, text)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityShowsSensitiveNumberAsMarkerByDefault(t *testing.T) {
	c, _ := applyEncodeSensitiveConfig(t, encodeSensitiveCredentialConfig)

	credential := encodeEntityByID(t, c, "resource.credential.db")

	out, err := EncodeEntity(credential)
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `pin\s+= "\(sensitive\)"`, text)
	require.NotContains(t, text, "4821")
	require.NotContains(t, text, "pw-value")
}

func TestEncodeEntityRevealsSensitiveNumberWhenAsked(t *testing.T) {
	c, _ := applyEncodeSensitiveConfig(t, encodeSensitiveCredentialConfig)

	credential := encodeEntityByID(t, c, "resource.credential.db")

	out, err := EncodeEntity(credential, RevealSensitive())
	require.NoError(t, err)

	text := string(out)

	require.Regexp(t, `pin\s+= 4821`, text)
	require.Regexp(t, `password\s+= "pw-value"`, text)
	require.NotContains(t, text, "(sensitive)")
}

func TestEncodeEntityWithoutSensitiveFieldsIsUnchangedByDefault(t *testing.T) {
	c, _ := applyEncodeSensitiveConfig(t, `
resource "database" "main" {
  location = "us-east"
  port     = 5432
}
`)

	database := encodeEntityByID(t, c, "resource.database.main")

	out, err := EncodeEntity(database)
	require.NoError(t, err)

	expected := `resource "database" "main" {
  location = "us-east"
  port     = 5432
}
`

	require.Equal(t, expected, string(out))
}

func TestEncodeEntityWithoutSensitiveFieldsIsUnchangedWhenRevealing(t *testing.T) {
	c, _ := applyEncodeSensitiveConfig(t, `
resource "database" "main" {
  location = "us-east"
  port     = 5432
}
`)

	database := encodeEntityByID(t, c, "resource.database.main")

	out, err := EncodeEntity(database, RevealSensitive())
	require.NoError(t, err)

	expected := `resource "database" "main" {
  location = "us-east"
  port     = 5432
}
`

	require.Equal(t, expected, string(out))
}
