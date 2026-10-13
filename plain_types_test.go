package xcl

import (
	"path/filepath"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// plainRegistry declares the plain fixture's types.
func plainRegistry() *registry.Local {
	local := registry.NewLocal()
	local.RegisterType(&plain.Agent{}, "agent")
	local.RegisterType(&plain.Database{}, "database", "postgres")
	local.RegisterType(&plain.Organization{}, "organization")
	local.RegisterType(&plain.Job{}, "job")
	local.RegisterType(&plain.Unlabelled{}, "unlabelled")

	return local
}

// applyPlainWithState applies the plain fixture, saving state in dir.
func applyPlainWithState(t *testing.T, dir string) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c, err := NewConfig(WithRegistry(plainRegistry()), WithStatePath(dir))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/plain/main.xcl")
	require.NoError(t, err)

	require.NoError(t, c.Apply(path))

	return c
}

// loadPlainState reads back the state applyPlainWithState saved in dir, in a
// Config of its own.
func loadPlainState(t *testing.T, dir string) *Config {
	t.Helper()

	c, err := NewConfig(WithRegistry(plainRegistry()), WithStatePath(dir))
	require.NoError(t, err)

	require.NoError(t, c.Load())

	return c
}

func TestPlainTypesSurviveSavedState(t *testing.T) {
	dir := t.TempDir()
	applyPlainWithState(t, dir)

	loaded := loadPlainState(t, dir)

	database, err := Find[plain.Database](loaded, "database.postgres.main")
	require.NoError(t, err)
	require.Equal(t, "postgres", database.Engine)
	require.Equal(t, "main", database.Name)
	require.Equal(t, 5432, database.Port)
}

func TestPlainTypesKeepAFalsePointerInSavedState(t *testing.T) {
	dir := t.TempDir()
	applyPlainWithState(t, dir)

	loaded := loadPlainState(t, dir)

	agent, err := Find[plain.Agent](loaded, "agent.ollama")
	require.NoError(t, err)
	require.NotNil(t, agent.Enabled)
	require.False(t, *agent.Enabled)
}

func TestPlainTypesKeepNestedBlocksInSavedState(t *testing.T) {
	dir := t.TempDir()
	applyPlainWithState(t, dir)

	loaded := loadPlainState(t, dir)

	organization, err := Find[plain.Organization](loaded, "organization.hidden")
	require.NoError(t, err)
	require.Len(t, organization.Overrides, 1)
	require.Equal(t, "claude", organization.Overrides[0].Name)
}

func TestPlainTypesKeepTheirMetaFieldInSavedState(t *testing.T) {
	dir := t.TempDir()
	applyPlainWithState(t, dir)

	loaded := loadPlainState(t, dir)

	job, err := Find[plain.Job](loaded, "job.deploy")
	require.NoError(t, err)
	require.Equal(t, "job.deploy", job.Meta.ID)
	require.Equal(t, 25, job.Meta.Line)
}

func TestEncodeEntityWritesAStoredPlainEntity(t *testing.T) {
	c := setupPlainConfig(t)

	stored, err := c.FindOneEntity("database", "postgres")
	require.NoError(t, err)

	encoded, err := EncodeEntity(stored)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `database "postgres" "main"`)
	require.Contains(t, string(encoded), "port = 5432")
}

func TestEncodeEntityRefusesAPlainValue(t *testing.T) {
	c := setupPlainConfig(t)

	database, err := Find[plain.Database](c, "database.postgres.main")
	require.NoError(t, err)

	_, err = EncodeEntity(database)
	require.ErrorIs(t, err, xclerrors.ErrNotEncodable)
	require.ErrorContains(t, err, "encode the entity itself")
}
