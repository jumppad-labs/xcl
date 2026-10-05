// Command appconfig shows XCL used the way a single JSON configuration file
// is used: one application configuration, deeply nested, read into Go types
// and then used by the program.
//
// The configuration it parses is one file, ./config/app.xcl, and the Go types
// it parses into are in ./resources. Between them they show every shape a
// JSON configuration is built from: objects nested inside objects, arrays of
// objects, arrays of values, and maps whose keys the Go type does not know.
// There is no plugin and no provider, the block type is a plain Go type
// registered on the plugin registry.
//
// The program prints the configuration as a tree, then prints the same
// resource as JSON, which is the document this configuration replaces. The
// database password is declared sensitive, so it prints as "(sensitive)"; the
// program reads the real value with Reveal only to build the database
// connection string, which it never prints.
//
// Set XCL_STATE_KEY to a base64 encoded 32 byte key, for example
// `XCL_STATE_KEY=$(openssl rand -base64 32) make run`, to encrypt the
// passwords in state. Without it xcl warns that state holds them in plain
// text.
//
// Run it from this directory with `make run`, see the Makefile for the other
// targets. The configuration directory can be passed as an argument:
// `go run . <config dir>`, it defaults to ./config.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/appconfig/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// stateKeyEnv names the environment variable holding the key the example
// encrypts the sensitive values in its state with: 32 bytes, base64 encoded,
// for example the output of `openssl rand -base64 32`. In a real application
// the key comes from a secret store and is never committed. Without it the
// example still runs, and xcl warns that state holds sensitive values in
// plain text.
const stateKeyEnv = "XCL_STATE_KEY"

// stateKeyFromEnv returns the key in stateKeyEnv, or nil when it is not set
func stateKeyFromEnv() ([]byte, error) {
	encoded := os.Getenv(stateKeyEnv)
	if encoded == "" {
		return nil, nil
	}

	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%s is not base64: %w", stateKeyEnv, err)
	}

	return key, nil
}

// stateMaskOptions returns the option that encrypts state with stateKey, or
// none when there is no key
func stateMaskOptions(stateKey []byte) ([]xcl.ConfigOption, error) {
	if stateKey == nil {
		return nil, nil
	}

	masker, err := mask.EncryptAES256GCM(stateKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", stateKeyEnv, err)
	}

	return []xcl.ConfigOption{xcl.WithStateMask(masker)}, nil
}

func main() {
	dir := "./config"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	// The state is encrypted when a key is given, see stateKeyEnv
	stateKey, err := stateKeyFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// Keep the state in a temporary directory, removed when the example ends
	stateDir, err := os.MkdirTemp("", "xcl-example")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	// the registry is built here so that the event receiver can share it: it
	// is what types the entity an event carries, which is how the receiver
	// shows each resource's configuration as it is created
	r := registry.NewPluginRegistry()

	_, err = run(os.Stdout, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r), r, dir, stateDir, stateKey)
	os.RemoveAll(stateDir)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// run applies the configuration in dir with the application type registered,
// keeping the state in a file in stateDir, writes the configuration to out
// as a tree and as JSON, and returns the application it parsed. Every event
// xcl produces goes to handler, a nil handler leaves xcl silent. The
// sensitive values in state are encrypted with stateKey, or written in plain
// text, with a warning, when it is nil.
func run(out io.Writer, handler xcl.EventHandler, r *registry.PluginRegistry, dir string, stateDir string, stateKey []byte) (*resources.Application, error) {

	// One block type, one Go type. Everything nested inside it is reached
	// through the fields of that type.
	if err := r.RegisterType(&resources.Application{}, "resource", "application"); err != nil {
		return nil, err
	}

	masking, err := stateMaskOptions(stateKey)
	if err != nil {
		return nil, err
	}

	options := []xcl.ConfigOption{
		xcl.WithPluginRegistry(r),
		// Keep the state in a file, Destroy works from it alone
		xcl.WithStatePath(stateDir),
		xcl.WithEventHandler(handler),
		// events carry nothing by default, this asks for each resource as
		// state records it, which is what the receiver turns back into
		// configuration text. The password is redacted in it.
		xcl.WithEventData(xcl.EventDataProcessed),
	}

	// with a key, the password is encrypted in the state file
	c, err := xcl.NewConfig(append(options, masking...)...)
	if err != nil {
		return nil, err
	}

	if err := c.Apply(dir); err != nil {
		return nil, err
	}

	// A registered type comes back as the Go type it was registered as, so
	// the whole tree below it is ordinary Go structs, slices and maps
	app, err := xcl.Find[resources.Application](c, "resource.application.api")
	if err != nil {
		return nil, err
	}

	// Reveal gives the real password. It is used here, where the program
	// needs it, and the connection string holding it is never printed
	if app.Database != nil {
		if err := checkConnectionString(databaseURL(app.Database)); err != nil {
			return nil, err
		}
	}

	printApplication(out, app)

	if err := printJSON(out, app); err != nil {
		return nil, err
	}

	return app, nil
}

// printApplication walks the application and writes it as a tree, one line
// per block, so the shape of the configuration is visible
func printApplication(out io.Writer, app *resources.Application) {
	fmt.Fprintln(out, "## Application")
	fmt.Fprintf(out, "  %s name=%s environment=%s\n", app.Meta.ID, app.Name, app.Environment)
	fmt.Fprintf(out, "    owners=%s tier=%s\n", strings.Join(app.Owners, ","), app.Labels["tier"])

	// A block that appears once is a pointer, nil when it is left out
	if app.Server != nil {
		fmt.Fprintf(out, "  server host=%s port=%d\n", app.Server.Host, app.Server.Port)

		if app.Server.TLS != nil {
			fmt.Fprintf(out, "    tls enabled=%t ciphers=%d\n", app.Server.TLS.Enabled, len(app.Server.TLS.Ciphers))

			// Four blocks deep: application, server, tls, client_auth
			if app.Server.TLS.ClientAuth != nil {
				fmt.Fprintf(out, "      client_auth mode=%s ca_file=%s\n", app.Server.TLS.ClientAuth.Mode, app.Server.TLS.ClientAuth.CAFile)
			}
		}

		if app.Server.Timeouts != nil {
			fmt.Fprintf(out, "    timeouts read=%d write=%d idle=%d\n", app.Server.Timeouts.Read, app.Server.Timeouts.Write, app.Server.Timeouts.Idle)
		}
	}

	if app.Database != nil {
		fmt.Fprintf(out, "  database driver=%s host=%s name=%s\n", app.Database.Driver, app.Database.Host, app.Database.Name)

		if app.Database.Pool != nil {
			fmt.Fprintf(out, "    pool max_open=%d max_idle=%d\n", app.Database.Pool.MaxOpen, app.Database.Pool.MaxIdle)
		}

		// A repeated block is a slice, in the order the blocks were written
		for _, replica := range app.Database.Replicas {
			fmt.Fprintf(out, "    replica host=%s weight=%d\n", replica.Host, replica.Weight)
		}

		fmt.Fprintf(out, "    options sslmode=%s\n", app.Database.Options["sslmode"])
	}

	if app.Telemetry != nil {
		if app.Telemetry.Logging != nil {
			fmt.Fprintf(out, "  telemetry logging level=%s format=%s\n", app.Telemetry.Logging.Level, app.Telemetry.Logging.Format)
		}

		if app.Telemetry.Tracing != nil {
			fmt.Fprintf(out, "  telemetry tracing endpoint=%s sample_rate=%v\n", app.Telemetry.Tracing.Endpoint, app.Telemetry.Tracing.SampleRate)
		}
	}

	// Repeated blocks holding repeated blocks of their own
	for _, service := range app.Services {
		fmt.Fprintf(out, "  service %s url=%s\n", service.Name, service.URL)

		if service.Retry != nil {
			fmt.Fprintf(out, "    retry attempts=%d backoff=%d\n", service.Retry.Attempts, service.Retry.Backoff)
		}

		for _, route := range service.Routes {
			fmt.Fprintf(out, "    route %s methods=%s\n", route.Path, strings.Join(route.Methods, ","))

			if route.RateLimit != nil {
				fmt.Fprintf(out, "      rate_limit requests_per_second=%d burst=%d\n", route.RateLimit.RequestsPerSecond, route.RateLimit.Burst)
			}
		}
	}
}

// databaseURL builds the connection string for the database. It holds the real
// password, read with Reveal, so it must not be printed or logged.
func databaseURL(db *resources.Database) string {
	connection := url.URL{
		Scheme: db.Driver,
		User:   url.UserPassword(db.Username, db.Password.Reveal()),
		Host:   net.JoinHostPort(db.Host, strconv.Itoa(db.Port)),
		Path:   "/" + db.Name,
	}

	return connection.String()
}

// checkConnectionString stands in for opening the database: it checks the
// connection string is complete without ever showing it
func checkConnectionString(connection string) error {
	parsed, err := url.Parse(connection)
	if err != nil {
		return fmt.Errorf("invalid database connection string")
	}

	if _, hasPassword := parsed.User.Password(); !hasPassword {
		return fmt.Errorf("database connection string has no password")
	}

	return nil
}

// printJSON writes the application as JSON. The json tags on the Go types
// name the fields, so this is the document the configuration replaces. A
// sensitive field is written as "(sensitive)"; state keeps the real value.
func printJSON(out io.Writer, app *resources.Application) error {
	d, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "## JSON")
	fmt.Fprintln(out, string(d))

	return nil
}
