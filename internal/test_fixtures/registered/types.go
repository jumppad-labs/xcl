// Package registered holds plain Go types that tests register with
// PluginRegistry.RegisterType, they have no plugin and no provider.
package registered

import "github.com/jumppad-labs/xcl/types"

// TypeDatabase is the string resource type for Database resources
const TypeDatabase = "database"

// TypeApp is the string resource type for App resources
const TypeApp = "app"

// TypeConsumer is the string resource type for Consumer resources
const TypeConsumer = "consumer"

// TypeCache is the type of Cache entities, registered without a subtype so
// that it leads its own declaration with only a name, cache "main" {}
const TypeCache = "cache"

// TypeServer is the type of Server entities, registered with the subtype
// SubtypeBig so that they are declared server "big" "web" {}
const TypeServer = "server"

// SubtypeBig is the subtype Server entities are registered under
const SubtypeBig = "big"

// Database is a registered type with a nested block and a computed field
type Database struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Port     int    `xcl:"port" json:"port"`

	Timeouts *Timeouts `xcl:"timeouts,block" json:"timeouts,omitempty"`

	// ConnectionString is computed, nothing ever sets it for a registered type
	ConnectionString string `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`
}

// Timeouts is the nested timeouts block of a Database
type Timeouts struct {
	Connect int `xcl:"connect" json:"connect"`
	Read    int `xcl:"read,optional" json:"read,omitempty"`
}

// App reads a variable, fields of a Database and a module output
type App struct {
	types.ResourceBase `xcl:",remain"`

	Environment      string `xcl:"environment" json:"environment"`
	DatabaseLocation string `xcl:"database_location" json:"database_location"`
	DatabasePort     int    `xcl:"database_port" json:"database_port"`
	SharedLocation   string `xcl:"shared_location" json:"shared_location"`
}

// Consumer reads a field of an App
type Consumer struct {
	types.ResourceBase `xcl:",remain"`

	AppEnvironment string `xcl:"app_environment" json:"app_environment"`
}

// Cache is registered as the type "cache" without a subtype, it is declared
// with a single label and is addressed cache.<name>
type Cache struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
}

// Server is registered as the type "server" with the subtype "big", a type
// other than resource that takes a subtype. It is declared
// server "big" "<name>" {} and addressed server.big.<name>
type Server struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Size     int    `xcl:"size,optional" json:"size,omitempty"`
}

// TypeSecret is the type of Secret entities
const TypeSecret = "secret"

// TypeSecretConsumer is the type of SecretConsumer entities
const TypeSecretConsumer = "secret_consumer"

// Secret holds a sensitive password beside a plain username
type Secret struct {
	types.ResourceBase `xcl:",remain"`

	Username string                  `xcl:"username,optional" json:"username,omitempty"`
	Password types.Sensitive[string] `xcl:"password" json:"password"`
}

// SecretConsumer reads values that may be sensitive into a sensitive field
// and a plain field
type SecretConsumer struct {
	types.ResourceBase `xcl:",remain"`

	Password types.Sensitive[string] `xcl:"password,optional" json:"password"`
	Note     string                  `xcl:"note,optional" json:"note,omitempty"`
}

// TypeSecretShapes is the type of SecretShape entities
const TypeSecretShapes = "secret_shape"

// SecretShape has plain and sensitive fields of object and list shapes
type SecretShape struct {
	types.ResourceBase `xcl:",remain"`

	Tags    map[string]string                  `xcl:"tags,optional" json:"tags,omitempty"`
	Items   []string                           `xcl:"items,optional" json:"items,omitempty"`
	Secrets map[string]types.Sensitive[string] `xcl:"secrets,optional" json:"secrets,omitempty"`
	Login   *SecretLogin                       `xcl:"login,optional" json:"login,omitempty"`
}

// SecretLogin has a plain and a sensitive inner field
type SecretLogin struct {
	User     string                  `xcl:"user,optional" json:"user,omitempty"`
	Password types.Sensitive[string] `xcl:"password,optional" json:"password,omitempty"`
}
