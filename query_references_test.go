package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// cacheClient is registered bare as "cache_client"; its nested blocks
// reference a registered cache as a whole
type cacheClient struct {
	types.ResourceBase `xcl:",remain"`

	Primary *cacheLink `xcl:"primary,block" json:"primary,omitempty"`
	Mirror  *cacheCopy `xcl:"mirror,block" json:"mirror,omitempty"`
}

// cacheLink holds the referenced cache through a pointer field
type cacheLink struct {
	Cache *registered.Cache `xcl:"cache" json:"cache"`
}

// cacheCopy holds the referenced cache through a value field
type cacheCopy struct {
	Cache registered.Cache `xcl:"cache" json:"cache"`
}

// A nested block can reference a registered block as a whole rather than one
// of its fields. The referenced block here is a cache, which is registered in
// the bare form and so leads its own declaration rather than sitting under the
// resource type. The field the reference lands in may be a pointer or a value;
// either way it holds the configured values of the cache it names.
//
// setupCacheClientConfig is the harness. It registers the cache and the client
// that references it, both bare, and applies the cache_client fixture, which
// declares two caches and one client whose primary block references the first
// through a pointer field and whose mirror block references the second through
// a value field.
func setupCacheClientConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(
		WithType(&registered.Cache{}, registered.TypeCache),
		WithType(&cacheClient{}, "cache_client"),
		WithStateStore(store),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/cache_client/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

func TestWholeBlockReferenceFillsAPointerFieldInANestedBlock(t *testing.T) {
	c := setupCacheClientConfig(t)

	client, err := Find[cacheClient](c, "cache_client.app")
	require.NoError(t, err)
	require.NotNil(t, client.Primary)
	require.NotNil(t, client.Primary.Cache)

	// the primary block references cache.east
	require.Equal(t, "us-east", client.Primary.Cache.Location)
	require.Equal(t, "cache.east", client.Primary.Cache.Meta.ID)
}

func TestWholeBlockReferenceFillsAValueFieldInANestedBlock(t *testing.T) {
	c := setupCacheClientConfig(t)

	client, err := Find[cacheClient](c, "cache_client.app")
	require.NoError(t, err)
	require.NotNil(t, client.Mirror)

	// the mirror block references cache.west
	require.Equal(t, "eu-west", client.Mirror.Cache.Location)
	require.Equal(t, "cache.west", client.Mirror.Cache.Meta.ID)
}
