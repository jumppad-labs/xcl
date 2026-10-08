package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// An entity has a type and an optional subtype. These tests stand where an
// application stands: they register server with the subtype big, declare
// server "big" "web" and reach it through the public API.

// setupSubtypedConfig applies the subtyped fixture with server registered
// under the subtype big, cache without a subtype and app under resource
func setupSubtypedConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(
		WithType(&registered.Server{}, registered.TypeServer, registered.SubtypeBig),
		WithType(&registered.Cache{}, registered.TypeCache),
		WithType(&registered.App{}, "resource", registered.TypeApp),
		WithStateStore(store),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/subtyped/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

func TestFindResourceReachesAnEntityByTypeSubtypeAndName(t *testing.T) {
	c := setupSubtypedConfig(t)

	entity, err := c.FindResource("server.big.web")
	require.NoError(t, err)

	server, ok := entity.(*registered.Server)
	require.True(t, ok, "expected *registered.Server, got %T", entity)
	require.Equal(t, "eu-west", server.Location)
}

func TestFindByTypeReturnsEntitiesOfATypeAndSubtype(t *testing.T) {
	c := setupSubtypedConfig(t)

	servers, err := FindByType[registered.Server](c, registered.TypeServer, registered.SubtypeBig)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Equal(t, "server.big.web", servers[0].Meta.ID)
}

func TestFindByTypeRejectsATypeThatTakesASubtypeOnItsOwn(t *testing.T) {
	c := setupSubtypedConfig(t)

	servers, err := FindByType[registered.Server](c, registered.TypeServer)
	require.ErrorIs(t, err, ErrNotTypeable)
	require.Nil(t, servers)

	var notTypeable *NotTypeableError
	require.ErrorAs(t, err, &notTypeable)
	require.Contains(t, notTypeable.Use, `"server"`)
}

func TestFindByTypeRejectsASubtypeThatWasNotRegistered(t *testing.T) {
	c := setupSubtypedConfig(t)

	servers, err := FindByType[registered.Server](c, registered.TypeServer, "small")
	require.ErrorIs(t, err, ErrUnknownType)
	require.Nil(t, servers)
}

func TestAllReturnsEntitiesOfAGoTypeRegisteredWithASubtype(t *testing.T) {
	c := setupSubtypedConfig(t)

	servers, err := All[registered.Server](c)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Equal(t, "server.big.web", servers[0].Meta.ID)
}

func TestEncodeEntityWritesTheTypeAndSubtypeOfAnEntity(t *testing.T) {
	c := setupSubtypedConfig(t)

	entity, err := c.FindResource("server.big.web")
	require.NoError(t, err)

	text, err := EncodeEntity(entity)
	require.NoError(t, err)
	require.Contains(t, string(text), `server "big" "web" {`)
}
