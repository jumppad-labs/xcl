package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// A Config given neither WithStatePath nor WithStateStore persists nothing,
// it is the configuration-only mode: the program declares its own Go types
// and reads the configuration back into them.

// bareFixturePath returns the absolute path of the registered bare fixture,
// which declares one cache and one database
func bareFixturePath(t *testing.T) string {
	t.Helper()

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/bare/main.xcl")
	require.NoError(t, err)

	return path
}

// newDeclaredTypesConfig returns a Config that declares the cache and database
// types on a local registry and has no state
func newDeclaredTypesConfig(t *testing.T) *Config {
	t.Helper()

	local := registry.NewLocal()
	local.RegisterType(&registered.Cache{}, registered.TypeCache)
	local.RegisterType(&registered.Database{}, "resource", registered.TypeDatabase)

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	return c
}

func TestApplyWithoutStateWritesNothing(t *testing.T) {
	// the fixture path is made absolute before the working directory moves
	path := bareFixturePath(t)

	workingDir := t.TempDir()
	t.Chdir(workingDir)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	c := newDeclaredTypesConfig(t)

	err := c.Apply(path)
	require.NoError(t, err)
	require.NotEmpty(t, c.Entities())

	workingEntries, err := os.ReadDir(workingDir)
	require.NoError(t, err)
	require.Len(t, workingEntries, 0)

	homeEntries, err := os.ReadDir(homeDir)
	require.NoError(t, err)
	require.Len(t, homeEntries, 0)
}

func TestDeclaredTypesDecodeIntoGoValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newDeclaredTypesConfig(t)

	err := c.Apply(bareFixturePath(t))
	require.NoError(t, err)

	database, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.Equal(t, "us-east", database.Location)
	require.Equal(t, 5432, database.Port)

	cache, err := Find[registered.Cache](c, "cache.main")
	require.NoError(t, err)
	require.Equal(t, "us-east", cache.Location)

	target := databasesAndCaches{}
	err = Decode(c, &target)
	require.NoError(t, err)

	require.Len(t, target.Databases, 1)
	require.Same(t, database, target.Databases[0])

	require.Len(t, target.Caches, 1)
	require.Same(t, cache, target.Caches[0])
}
