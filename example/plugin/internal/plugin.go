package internal

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/xcl/example/plugin/resources"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// ExamplePlugin is an in-process plugin, it is compiled into this program and
// called directly. It provides the postgres and redis block types, the app and
// ingress block types are provided by the external plugin in ./external.
type ExamplePlugin struct {
	plugins.PluginBase
}

// Ensure ExamplePlugin implements the Plugin interface
var _ plugins.Plugin = (*ExamplePlugin)(nil)

// Init registers the block types the plugin provides, with their providers. A
// plugin provides as many block types as it likes, each is registered with its
// own provider by calling RegisterResourceProvider once per type.
//
// logger is plugin scoped, for messages written outside a provider call like
// this one. xcl names the plugin, ExamplePlugin, as the source of everything
// it logs, and adds provider=<block type> to what each provider's Init logs.
// During a call the providers log through plugins.Logger(ctx) instead, which
// xcl binds to the resource and step being worked on.
func (p *ExamplePlugin) Init(logger logger.Logger, state plugins.State) error {
	logger.Debug("registering block types", "block_types", "postgres, redis")

	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"postgres",
		&resources.PostgreSQL{},
		&postgresProvider{},
	)
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"redis",
		&resources.Redis{},
		&redisProvider{},
	)
}

// postgresProvider handles the lifecycle of postgres blocks. A real provider
// would create a database, this one only fills in the connection string.
//
// Change detection comes from the embedded DefaultChanged.
type postgresProvider struct {
	plugins.DefaultChanged[*resources.PostgreSQL]
}

var _ plugins.ResourceProvider[*resources.PostgreSQL] = (*postgresProvider)(nil)

// Init is called once when the provider is registered. The logger it receives
// is plugin scoped and adds provider=postgres to every message. The provider
// does not keep it, during a call it logs through plugins.Logger(ctx).
func (p *postgresProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	logger.Debug("provider ready")

	return nil
}

// Create sets the computed connection string, configured fields are never
// changed
func (p *postgresProvider) Create(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	db.ConnectionString = connectionString(db)
	plugins.Logger(ctx).Info("created database", "connection_string", db.ConnectionString)

	return db, nil
}

// Read reports the database as configured, xcl has already carried the
// computed connection string over from the previous state
func (p *postgresProvider) Read(ctx context.Context, old *resources.PostgreSQL, new *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	plugins.Logger(ctx).Info("read database")

	return new, nil
}

// Update sets the computed connection string for the changed configuration
func (p *postgresProvider) Update(ctx context.Context, db *resources.PostgreSQL) (*resources.PostgreSQL, error) {
	db.ConnectionString = connectionString(db)
	plugins.Logger(ctx).Info("updated database", "connection_string", db.ConnectionString)

	return db, nil
}

// Destroy would remove the database, there is nothing to remove here
func (p *postgresProvider) Destroy(ctx context.Context, db *resources.PostgreSQL, force bool) error {
	plugins.Logger(ctx).Info("destroyed database", "force", force)

	return nil
}

func (p *postgresProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func connectionString(db *resources.PostgreSQL) string {
	return fmt.Sprintf("postgres://%s@%s:%d/%s", db.Username, db.Location, db.Port, db.DBName)
}

// redisProvider handles the lifecycle of redis blocks, the second block type
// this plugin provides. It is a separate provider, registered in Init
// alongside the postgres one.
type redisProvider struct {
	plugins.DefaultChanged[*resources.Redis]
}

var _ plugins.ResourceProvider[*resources.Redis] = (*redisProvider)(nil)

// Init is called once when the provider is registered, with a plugin scoped
// logger that adds provider=redis to every message
func (p *redisProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	logger.Debug("provider ready")

	return nil
}

// Create sets the computed connection string, the app block reads it through
// the external plugin
func (p *redisProvider) Create(ctx context.Context, cache *resources.Redis) (*resources.Redis, error) {
	cache.ConnectionString = cacheConnectionString(cache)
	plugins.Logger(ctx).Info("created cache", "connection_string", cache.ConnectionString)

	return cache, nil
}

// Read reports the cache as configured, xcl has already carried the computed
// connection string over from the previous state
func (p *redisProvider) Read(ctx context.Context, old *resources.Redis, new *resources.Redis) (*resources.Redis, error) {
	plugins.Logger(ctx).Info("read cache")

	return new, nil
}

// Update sets the computed connection string for the changed configuration
func (p *redisProvider) Update(ctx context.Context, cache *resources.Redis) (*resources.Redis, error) {
	cache.ConnectionString = cacheConnectionString(cache)
	plugins.Logger(ctx).Info("updated cache", "connection_string", cache.ConnectionString)

	return cache, nil
}

// Destroy would remove the cache, there is nothing to remove here
func (p *redisProvider) Destroy(ctx context.Context, cache *resources.Redis, force bool) error {
	plugins.Logger(ctx).Info("destroyed cache", "force", force)

	return nil
}

func (p *redisProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func cacheConnectionString(cache *resources.Redis) string {
	return fmt.Sprintf("redis://%s:%d", cache.Location, cache.Port)
}
