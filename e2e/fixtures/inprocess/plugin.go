// Package inprocess is the end-to-end suite's in-process plugin. It is
// registered on the plugin registry and called directly, and provides the
// postgres and redis block types of testdata/plugin. It is written against
// xcl's public plugin and logger packages only, as a plugin author's would be.
package inprocess

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/jumppad-labs/xcl/e2e/fixtures/services"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// Plugin is an in-process plugin, it is compiled into the test binary and
// called directly. It provides the postgres and redis block types, the app and
// ingress block types are provided by the external plugin in
// ../externalplugin.
type Plugin struct {
	plugins.PluginBase
}

// Ensure Plugin implements the Plugin interface
var _ plugins.Plugin = (*Plugin)(nil)

// Init registers the block types the plugin provides, with their providers. A
// plugin provides as many block types as it likes, each is registered with its
// own provider by calling RegisterResourceProvider once per type.
//
// logger is plugin scoped, for messages written outside a provider call like
// this one. xcl names the plugin, Plugin, as the source of everything
// it logs, and adds provider=<block type> to what each provider's Init logs.
// During a call the providers log through plugins.Logger(ctx) instead, which
// xcl binds to the resource and step being worked on.
func (p *Plugin) Init(logger logger.Logger, state plugins.State) error {
	logger.Debug("registering block types", "block_types", "postgres, redis")

	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"postgres",
		&services.PostgreSQL{},
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
		&services.Redis{},
		&redisProvider{},
	)
}

// postgresProvider handles the lifecycle of postgres blocks. A real provider
// would create a database, this one only fills in the connection string.
//
// Change detection comes from the embedded DefaultChanged, except that a
// changed location answers replace, see Changed.
type postgresProvider struct {
	plugins.DefaultChanged[*services.PostgreSQL]
}

var _ plugins.ResourceProvider[*services.PostgreSQL] = (*postgresProvider)(nil)

// Init is called once when the provider is registered. The logger it receives
// is plugin scoped and adds provider=postgres to every message. The provider
// does not keep it, during a call it logs through plugins.Logger(ctx).
func (p *postgresProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	logger.Debug("provider ready")

	return nil
}

// Changed answers replace when the location changes, the connection identity changes with it, so the
// e2e scenarios have a provider-decided replacement. Every other change is
// left to DefaultChanged.
func (p *postgresProvider) Changed(ctx context.Context, old *services.PostgreSQL, new *services.PostgreSQL, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	if old.Location != new.Location {
		return entity.Replace, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new, changes, dependencies)
}

// Create sets the computed connection string, configured fields are never
// changed
func (p *postgresProvider) Create(ctx context.Context, db *services.PostgreSQL) (*services.PostgreSQL, error) {
	if err := connect(db); err != nil {
		return nil, err
	}

	db.ConnectionString = connectionString(db)
	plugins.Logger(ctx).Info("created database", "connection_string", db.ConnectionString)

	return db, nil
}

// Read reports the database as configured, xcl has already carried the
// computed connection string over from the previous state
func (p *postgresProvider) Read(ctx context.Context, old *services.PostgreSQL, new *services.PostgreSQL) (*services.PostgreSQL, error) {
	plugins.Logger(ctx).Info("read database")

	return new, nil
}

// Update sets the computed connection string for the changed configuration
func (p *postgresProvider) Update(ctx context.Context, db *services.PostgreSQL, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*services.PostgreSQL, error) {
	if err := connect(db); err != nil {
		return nil, err
	}

	db.ConnectionString = connectionString(db)
	plugins.Logger(ctx).Info("updated database", "connection_string", db.ConnectionString)

	return db, nil
}

// Destroy would remove the database, there is nothing to remove here
func (p *postgresProvider) Destroy(ctx context.Context, db *services.PostgreSQL, force bool) error {
	plugins.Logger(ctx).Info("destroyed database", "force", force)

	return nil
}

func (p *postgresProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// connect stands in for opening a connection to the database. It is the one
// place the real password is needed, so it is the one place Reveal is called.
// A revealed value is no longer protected: the address holding it is checked
// and never logged or returned.
func connect(db *services.PostgreSQL) error {
	address := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(db.Username, db.Password.Reveal()),
		Host:   net.JoinHostPort(db.Location, strconv.Itoa(db.Port)),
		Path:   "/" + db.DBName,
	}

	if _, err := url.Parse(address.String()); err != nil {
		return fmt.Errorf("unable to connect to database %s", db.DBName)
	}

	return nil
}

// connectionString is the address other blocks use to reach the database. It
// leaves the password out, so it is safe to log and to keep in state.
func connectionString(db *services.PostgreSQL) string {
	return fmt.Sprintf("postgres://%s@%s:%d/%s", db.Username, db.Location, db.Port, db.DBName)
}

// redisProvider handles the lifecycle of redis blocks, the second block type
// this plugin provides. It is a separate provider, registered in Init
// alongside the postgres one.
type redisProvider struct {
	plugins.DefaultChanged[*services.Redis]
}

var _ plugins.ResourceProvider[*services.Redis] = (*redisProvider)(nil)

// Init is called once when the provider is registered, with a plugin scoped
// logger that adds provider=redis to every message
func (p *redisProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	logger.Debug("provider ready")

	return nil
}

// Create sets the computed connection string, the app block reads it through
// the external plugin
func (p *redisProvider) Create(ctx context.Context, cache *services.Redis) (*services.Redis, error) {
	cache.ConnectionString = cacheConnectionString(cache)
	plugins.Logger(ctx).Info("created cache", "connection_string", cache.ConnectionString)

	return cache, nil
}

// Read reports the cache as configured, xcl has already carried the computed
// connection string over from the previous state
func (p *redisProvider) Read(ctx context.Context, old *services.Redis, new *services.Redis) (*services.Redis, error) {
	plugins.Logger(ctx).Info("read cache")

	return new, nil
}

// Update sets the computed connection string for the changed configuration
func (p *redisProvider) Update(ctx context.Context, cache *services.Redis, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*services.Redis, error) {
	cache.ConnectionString = cacheConnectionString(cache)
	plugins.Logger(ctx).Info("updated cache", "connection_string", cache.ConnectionString)

	return cache, nil
}

// Destroy would remove the cache, there is nothing to remove here
func (p *redisProvider) Destroy(ctx context.Context, cache *services.Redis, force bool) error {
	plugins.Logger(ctx).Info("destroyed cache", "force", force)

	return nil
}

func (p *redisProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func cacheConnectionString(cache *services.Redis) string {
	return fmt.Sprintf("redis://%s:%d", cache.Location, cache.Port)
}
