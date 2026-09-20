// Package resources holds the Go types for the application configuration in
// ../config/app.xcl, the kind of configuration that is usually written as a
// single JSON file: one object with objects inside it, arrays of objects, and
// a few free form maps.
//
// Only the top level type is a resource, so only it embeds
// types.ResourceBase. Everything below it is a plain Go struct, reached
// through the field that holds it, exactly as it would be reached through a
// JSON document.
//
// The shapes a JSON configuration is built from all have an equivalent here:
//
//   - an object becomes a nested block, held in a pointer, i.e. server
//   - an array of objects becomes a repeated block, held in a slice, i.e.
//     service, and each of those can hold repeated blocks of its own
//   - an array of values becomes a list attribute, i.e. ciphers
//   - an object with keys that are not known ahead of time becomes a map
//     attribute, i.e. labels
package resources

import "github.com/jumppad-labs/xcl/types"

// Application defines the block type `application`, the root of the
// configuration. Everything else in the file hangs off it.
type Application struct {
	types.ResourceBase `xcl:",remain"`

	Name        string `xcl:"name" json:"name"`
	Environment string `xcl:"environment" json:"environment"`

	// Owners is a list of values, an optional attribute is left empty when
	// the configuration does not set it
	Owners []string `xcl:"owners,optional" json:"owners,omitempty"`

	// Labels is a map, for keys the Go type does not know ahead of time
	Labels map[string]string `xcl:"labels,optional" json:"labels,omitempty"`

	// Server and Database appear once, so they are pointers. A pointer block
	// left out of the configuration is nil rather than an empty struct.
	Server   *Server   `xcl:"server,block" json:"server"`
	Database *Database `xcl:"database,block" json:"database"`

	// Telemetry is optional, this configuration sets it
	Telemetry *Telemetry `xcl:"telemetry,block" json:"telemetry,omitempty"`

	// Services is a repeated block, one entry per service block in the file,
	// in the order they are written
	Services []Service `xcl:"service,block" json:"service,omitempty"`
}

// Server is the nested `server` block of an application, it nests two blocks
// of its own
type Server struct {
	Host string `xcl:"host" json:"host"`
	Port int    `xcl:"port" json:"port"`

	TLS      *TLS      `xcl:"tls,block" json:"tls,omitempty"`
	Timeouts *Timeouts `xcl:"timeouts,block" json:"timeouts,omitempty"`
}

// TLS is the nested `tls` block of a server, and nests a block of its own in
// turn: application, server, tls, client_auth is four blocks deep. Blocks can
// be nested as deeply as the configuration needs.
type TLS struct {
	Enabled  bool   `xcl:"enabled" json:"enabled"`
	CertFile string `xcl:"cert_file" json:"cert_file"`
	KeyFile  string `xcl:"key_file" json:"key_file"`

	Ciphers []string `xcl:"ciphers,optional" json:"ciphers,omitempty"`

	ClientAuth *ClientAuth `xcl:"client_auth,block" json:"client_auth,omitempty"`
}

// ClientAuth is the nested `client_auth` block of a tls block
type ClientAuth struct {
	Mode   string `xcl:"mode" json:"mode"`
	CAFile string `xcl:"ca_file" json:"ca_file"`
}

// Timeouts is the nested `timeouts` block of a server, holding seconds
type Timeouts struct {
	Read  int `xcl:"read,optional" json:"read,omitempty"`
	Write int `xcl:"write,optional" json:"write,omitempty"`
	Idle  int `xcl:"idle,optional" json:"idle,omitempty"`
}

// Database is the nested `database` block of an application. It holds a block
// that appears once, a block that repeats, and a map of driver settings.
type Database struct {
	Driver   string `xcl:"driver" json:"driver"`
	Host     string `xcl:"host" json:"host"`
	Port     int    `xcl:"port" json:"port"`
	Name     string `xcl:"name" json:"name"`
	Username string `xcl:"username" json:"username"`
	Password string `xcl:"password" json:"password"`

	Pool *Pool `xcl:"pool,block" json:"pool,omitempty"`

	// Replicas is a repeated block, the configuration declares two
	Replicas []Replica `xcl:"replica,block" json:"replica,omitempty"`

	// Options carries driver settings the Go type does not name
	Options map[string]string `xcl:"options,optional" json:"options,omitempty"`
}

// Pool is the nested `pool` block of a database
type Pool struct {
	MaxOpen     int `xcl:"max_open" json:"max_open"`
	MaxIdle     int `xcl:"max_idle" json:"max_idle"`
	MaxLifetime int `xcl:"max_lifetime,optional" json:"max_lifetime,omitempty"`
}

// Replica is a nested `replica` block of a database
type Replica struct {
	Host   string `xcl:"host" json:"host"`
	Port   int    `xcl:"port" json:"port"`
	Weight int    `xcl:"weight,optional" json:"weight,omitempty"`
}

// Telemetry is the nested `telemetry` block of an application, grouping three
// blocks that each configure one thing
type Telemetry struct {
	Logging *Logging `xcl:"logging,block" json:"logging,omitempty"`
	Metrics *Metrics `xcl:"metrics,block" json:"metrics,omitempty"`
	Tracing *Tracing `xcl:"tracing,block" json:"tracing,omitempty"`
}

// Logging is the nested `logging` block of a telemetry block
type Logging struct {
	Level  string `xcl:"level" json:"level"`
	Format string `xcl:"format" json:"format"`
}

// Metrics is the nested `metrics` block of a telemetry block
type Metrics struct {
	Enabled bool   `xcl:"enabled" json:"enabled"`
	Path    string `xcl:"path" json:"path"`
}

// Tracing is the nested `tracing` block of a telemetry block, its sample rate
// is a float
type Tracing struct {
	Enabled    bool    `xcl:"enabled" json:"enabled"`
	Endpoint   string  `xcl:"endpoint" json:"endpoint"`
	SampleRate float64 `xcl:"sample_rate,optional" json:"sample_rate,omitempty"`
}

// Service is a repeated `service` block of an application, an upstream the
// application calls. It repeats, and holds repeated blocks of its own.
type Service struct {
	Name string `xcl:"name" json:"name"`
	URL  string `xcl:"url" json:"url"`

	// Headers is sent with every request to this service
	Headers map[string]string `xcl:"headers,optional" json:"headers,omitempty"`

	Retry *Retry `xcl:"retry,block" json:"retry,omitempty"`

	// Routes repeat inside a block that repeats itself
	Routes []Route `xcl:"route,block" json:"route,omitempty"`
}

// Retry is the nested `retry` block of a service
type Retry struct {
	Attempts int `xcl:"attempts" json:"attempts"`
	Backoff  int `xcl:"backoff,optional" json:"backoff,omitempty"`
}

// Route is a nested `route` block of a service, and nests a block of its own:
// application, service, route, rate_limit is the second path through this
// configuration that is four blocks deep
type Route struct {
	Path    string   `xcl:"path" json:"path"`
	Methods []string `xcl:"methods" json:"methods"`

	RateLimit *RateLimit `xcl:"rate_limit,block" json:"rate_limit,omitempty"`
}

// RateLimit is the nested `rate_limit` block of a route
type RateLimit struct {
	RequestsPerSecond int `xcl:"requests_per_second" json:"requests_per_second"`
	Burst             int `xcl:"burst,optional" json:"burst,omitempty"`
}
