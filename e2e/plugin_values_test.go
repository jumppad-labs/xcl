package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/services"
	"github.com/jumppad-labs/xcl/types"
)

// pluginServiceValues is what a test compares of a postgres or redis block
// the in-process plugin's providers created
type pluginServiceValues struct {
	location         string
	port             int
	connectionString string
}

func TestApplyFindsEveryResourceDeclaredForPlugins(t *testing.T) {
	c := applyPlugin(t, nil)

	require.Equal(t, pluginDeclaredIDs, pluginEntityIDs(t, c.Entities()))
}

// TestProviderFillsComputedValue asserts the computed connection_string of
// every postgres and redis block, including the one inside a module, holds
// the value the provider filled in when it created the block
func TestProviderFillsComputedValue(t *testing.T) {
	c := applyPlugin(t, nil)

	databases, err := xcl.FindByType[services.PostgreSQL](c, "resource", "postgres")
	require.NoError(t, err)

	caches, err := xcl.FindByType[services.Redis](c, "resource", "redis")
	require.NoError(t, err)

	found := map[string]pluginServiceValues{}
	for _, db := range databases {
		found[db.Meta.ID] = pluginServiceValues{location: db.Location, port: db.Port, connectionString: db.ConnectionString}
	}

	for _, cache := range caches {
		found[cache.Meta.ID] = pluginServiceValues{location: cache.Location, port: cache.Port, connectionString: cache.ConnectionString}
	}

	require.Equal(t, map[string]pluginServiceValues{
		"resource.postgres.main": {
			location:         "localhost",
			port:             5432,
			connectionString: "postgres://admin@localhost:5432/main",
		},
		"resource.postgres.replica": {
			location:         "replica.localhost",
			port:             5433,
			connectionString: "postgres://admin@replica.localhost:5433/main",
		},
		"module.analytics.resource.postgres.analytics": {
			location:         "analytics.localhost",
			port:             5432,
			connectionString: "postgres://analytics@analytics.localhost:5432/analytics",
		},
		"resource.redis.cache": {
			location:         "localhost",
			port:             6379,
			connectionString: "redis://localhost:6379",
		},
	}, found)
}

// TestComputedValueCrossesPlugins asserts the app block, provided by the
// external plugin, receives the values it references: a variable, a module
// output, configured and computed values of a postgres block and the computed
// value of a redis block, both provided by the in-process plugin, and holds
// the url its own provider computed
func TestComputedValueCrossesPlugins(t *testing.T) {
	c := applyPlugin(t, nil)

	app, err := xcl.Find[services.App](c, "resource.app.web")
	require.NoError(t, err)

	require.Equal(t, "resource.app.web", app.Meta.ID)
	require.Equal(t, "localhost", app.DatabaseLocation)
	require.Equal(t, "admin", app.DatabaseUser)
	require.Equal(t, "analytics.localhost", app.AnalyticsLocation)
	require.Equal(t, "postgres://admin@localhost:5432/main", app.ConnectionString)
	require.Equal(t, "redis://localhost:6379", app.CacheConnectionString)
	require.Equal(t, "http://web", app.URL)
}

// TestComputedValueCrossesTypesOfOnePlugin asserts the url the app provider
// computes reaches the ingress block, both types come from the external
// plugin
func TestComputedValueCrossesTypesOfOnePlugin(t *testing.T) {
	c := applyPlugin(t, nil)

	ingress, err := xcl.Find[services.Ingress](c, "resource.ingress.web")
	require.NoError(t, err)

	require.Equal(t, "resource.ingress.web", ingress.Meta.ID)
	require.Equal(t, "example.com", ingress.Hostname)
	require.Equal(t, "http://web", ingress.AppURL)
}

// TestPluginResourcesAreHeldAsGeneratedTypes asserts resources of a plugin's
// block types are held as types generated from the plugin's schema, not the
// plugin's own Go types, so a lookup copies them into the Go type it is given
func TestPluginResourcesAreHeldAsGeneratedTypes(t *testing.T) {
	c := applyPlugin(t, nil)

	for _, r := range c.Entities() {
		_, isPostgres := r.(*services.PostgreSQL)
		require.False(t, isPostgres, "plugin resources are held as generated types")

		_, isRedis := r.(*services.Redis)
		require.False(t, isRedis, "plugin resources are held as generated types")

		_, isApp := r.(*services.App)
		require.False(t, isApp, "plugin resources are held as generated types")

		_, isIngress := r.(*services.Ingress)
		require.False(t, isIngress, "plugin resources are held as generated types")
	}
}

// TestFindReturnsRootOutputValue asserts an output of the root configuration
// is read back by its address, and holds the value itself rather than the
// declaration that produced it
func TestFindReturnsRootOutputValue(t *testing.T) {
	c := applyPlugin(t, nil)

	output, err := xcl.Find[types.Output](c, "output.web_database")
	require.NoError(t, err)

	require.Equal(t, "localhost", output.Value)
}

// TestFindReturnsModuleOutputValue asserts an output a module publishes is
// read back by its address inside the module, and holds the value itself
func TestFindReturnsModuleOutputValue(t *testing.T) {
	c := applyPlugin(t, nil)

	output, err := xcl.Find[types.Output](c, "module.analytics.output.location")
	require.NoError(t, err)

	require.Equal(t, "analytics.localhost", output.Value)
}

// TestOutputsHoldsModuleOutputs asserts every published value is reachable
// at once, a module's output counted alongside the root's
func TestOutputsHoldsModuleOutputs(t *testing.T) {
	c := applyPlugin(t, nil)

	require.Len(t, c.Outputs(), 2)
}
