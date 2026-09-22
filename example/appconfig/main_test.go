package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/example/appconfig/resources"
	"github.com/stretchr/testify/require"
)

// configDir is the configuration this example parses
const configDir = "./config"

// application runs the example and returns the application it parsed, every
// test here is about that one resource
func application(t *testing.T) *resources.Application {
	t.Helper()

	app, err := run(&bytes.Buffer{}, nil, configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	return app
}

func TestAppConfigExampleParsesTheApplication(t *testing.T) {
	app := application(t)

	require.Equal(t, "resource.application.api", app.Meta.ID)
	require.Equal(t, "checkout-api", app.Name)
}

// TestAppConfigExampleReadsVariables asserts an attribute set from a variable
// takes the variable's default
func TestAppConfigExampleReadsVariables(t *testing.T) {
	app := application(t)

	require.Equal(t, "production", app.Environment)
	require.Equal(t, "postgres.internal", app.Database.Host)
}

// TestAppConfigExampleDecodesListsAndMaps asserts a list attribute keeps its
// order and a map attribute keeps every key
func TestAppConfigExampleDecodesListsAndMaps(t *testing.T) {
	app := application(t)

	require.Equal(t, []string{"platform-team", "payments-team"}, app.Owners)

	require.Equal(t, map[string]string{"tier": "edge", "component": "api"}, app.Labels)

	require.Equal(t, "verify-full", app.Database.Options["sslmode"])
	require.Equal(t, "checkout-api", app.Database.Options["application_name"])
}

// TestAppConfigExampleDecodesNestedBlocks asserts a block nested four deep is
// decoded, application holds server, which holds tls, which holds client_auth
func TestAppConfigExampleDecodesNestedBlocks(t *testing.T) {
	app := application(t)

	require.NotNil(t, app.Server)
	require.Equal(t, "0.0.0.0", app.Server.Host)
	require.Equal(t, 8443, app.Server.Port)

	require.NotNil(t, app.Server.TLS)
	require.True(t, app.Server.TLS.Enabled)
	require.Equal(t, []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"}, app.Server.TLS.Ciphers)

	require.NotNil(t, app.Server.TLS.ClientAuth)
	require.Equal(t, "require_and_verify", app.Server.TLS.ClientAuth.Mode)
	require.Equal(t, "/etc/certs/ca.pem", app.Server.TLS.ClientAuth.CAFile)

	require.NotNil(t, app.Server.Timeouts)
	require.Equal(t, 15, app.Server.Timeouts.Read)
	require.Equal(t, 60, app.Server.Timeouts.Idle)
}

// TestAppConfigExampleDecodesRepeatedBlocks asserts a block written more than
// once is decoded into a slice, in the order it was written
func TestAppConfigExampleDecodesRepeatedBlocks(t *testing.T) {
	app := application(t)

	require.Len(t, app.Database.Replicas, 2)
	require.Equal(t, "postgres-replica-1.internal", app.Database.Replicas[0].Host)
	require.Equal(t, 70, app.Database.Replicas[0].Weight)
	require.Equal(t, "postgres-replica-2.internal", app.Database.Replicas[1].Host)
	require.Equal(t, 30, app.Database.Replicas[1].Weight)
}

// TestAppConfigExampleDecodesRepeatedBlocksInsideRepeatedBlocks asserts the
// routes of each service are decoded, a repeated block inside a block that
// repeats itself
func TestAppConfigExampleDecodesRepeatedBlocksInsideRepeatedBlocks(t *testing.T) {
	app := application(t)

	require.Len(t, app.Services, 2)
	require.Equal(t, "payments", app.Services[0].Name)
	require.Equal(t, "inventory", app.Services[1].Name)

	payments := app.Services[0]
	require.Len(t, payments.Routes, 2)
	require.Equal(t, "/v1/charges", payments.Routes[0].Path)
	require.Equal(t, []string{"POST"}, payments.Routes[0].Methods)
	require.Equal(t, []string{"POST", "DELETE"}, payments.Routes[1].Methods)

	require.NotNil(t, payments.Routes[0].RateLimit)
	require.Equal(t, 50, payments.Routes[0].RateLimit.RequestsPerSecond)
	require.Equal(t, 100, payments.Routes[0].RateLimit.Burst)

	require.Equal(t, map[string]string{"x-service": "checkout-api"}, payments.Headers)
	require.Equal(t, 3, payments.Retry.Attempts)
}

// TestAppConfigExampleLeavesOmittedValuesEmpty asserts an optional attribute
// and an optional block the configuration leaves out stay empty, the
// inventory route sets no rate limit and its retry no backoff
func TestAppConfigExampleLeavesOmittedValuesEmpty(t *testing.T) {
	app := application(t)

	inventory := app.Services[1]
	require.Len(t, inventory.Routes, 1)
	require.Nil(t, inventory.Routes[0].RateLimit)
	require.Empty(t, inventory.Headers)
	require.Equal(t, 0, inventory.Retry.Backoff)
}

// TestAppConfigExampleDecodesFloats asserts a float attribute keeps its
// fractional value
func TestAppConfigExampleDecodesFloats(t *testing.T) {
	app := application(t)

	require.NotNil(t, app.Telemetry.Tracing)
	require.Equal(t, 0.1, app.Telemetry.Tracing.SampleRate)
	require.True(t, app.Telemetry.Metrics.Enabled)
	require.Equal(t, "json", app.Telemetry.Logging.Format)
}

// TestAppConfigExampleWritesTheApplicationAsJSON asserts the printed JSON
// holds the same tree the Go types do, the json tags naming each field
func TestAppConfigExampleWritesTheApplicationAsJSON(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	_, document, found := bytes.Cut(out.Bytes(), []byte("## JSON\n"))
	require.True(t, found)

	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(document, &decoded))

	require.Equal(t, "checkout-api", decoded["name"])

	server, ok := decoded["server"].(map[string]any)
	require.True(t, ok)

	tls, ok := server["tls"].(map[string]any)
	require.True(t, ok)

	clientAuth, ok := tls["client_auth"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "require_and_verify", clientAuth["mode"])

	services, ok := decoded["service"].([]any)
	require.True(t, ok)
	require.Len(t, services, 2)
}

// methodFormLookups are the lookups that the configuration also offers as
// generic methods from Go 1.27, the form an example must not be written in
var methodFormLookups = []string{"Find", "FindByType", "FindOne", "All"}

// TestAppConfigExampleUsesPortableLookupForm asserts the example looks
// entities up through the package level functions rather than the generic
// methods of the same name. Generic methods arrived in Go 1.27, so the method
// form would stop the code a reader copies from compiling on the project's
// minimum supported version. Entities, EntityCount and Outputs are ordinary
// methods and are not affected
func TestAppConfigExampleUsesPortableLookupForm(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	require.NoError(t, err)

	lookups := 0

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// a lookup names its type at the call, xcl.Find[T](c, address), so the
		// function called is the selector wrapped in an index expression
		fun := call.Fun
		switch indexed := fun.(type) {
		case *ast.IndexExpr:
			fun = indexed.X
		case *ast.IndexListExpr:
			fun = indexed.X
		}

		selector, ok := fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if !slices.Contains(methodFormLookups, selector.Sel.Name) {
			return true
		}

		qualifier, ok := selector.X.(*ast.Ident)
		require.True(t, ok,
			"main.go calls %s as a method, which needs Go 1.27, call xcl.%s instead", selector.Sel.Name, selector.Sel.Name)

		require.Equal(t, "xcl", qualifier.Name,
			"main.go calls the %s method form, which needs Go 1.27, call xcl.%s instead", selector.Sel.Name, selector.Sel.Name)

		lookups++

		return true
	})

	require.NotZero(t, lookups, "the guard found no lookups in main.go, so it proves nothing")
}

// eventRecorder records every event the run reports, it is the handler the
// tests pass in place of the example's pretty printer. The handler is never
// called concurrently, the mutex guards the reads the tests make
type eventRecorder struct {
	mu     sync.Mutex
	events []xcl.Event
}

func (r *eventRecorder) handle(e xcl.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, e)
}

// snapshot returns a copy of every event recorded so far
func (r *eventRecorder) snapshot() []xcl.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]xcl.Event{}, r.events...)
}

// runRecordingEvents runs the example with a recorder as its event handler
// and returns the recorder once the run has succeeded
func runRecordingEvents(t *testing.T) *eventRecorder {
	t.Helper()

	recorder := &eventRecorder{}

	_, err := run(&bytes.Buffer{}, recorder.handle, configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	return recorder
}

// TestAppConfigExampleReportsParseEventWithFile asserts each block's parse is
// reported as a success by core with the file it was parsed from
func TestAppConfigExampleReportsParseEventWithFile(t *testing.T) {
	recorder := runRecordingEvents(t)

	files := map[string]string{}
	for _, e := range recorder.snapshot() {
		if e.Operation != events.OperationParse {
			continue
		}

		require.Equal(t, events.SourceCore, e.Source)
		require.Equal(t, events.PhaseSuccess, e.Phase)
		files[e.ResourceID] = filepath.Base(e.File)
	}

	require.Equal(t, map[string]string{
		"variable.environment":     "app.xcl",
		"variable.db_host":         "app.xcl",
		"resource.application.api": "app.xcl",
		"output.listen_address":    "app.xcl",
	}, files)
}

// TestAppConfigExampleReportsApplicationCreated asserts the application's
// create is reported as a success by core, naming its type and file
func TestAppConfigExampleReportsApplicationCreated(t *testing.T) {
	recorder := runRecordingEvents(t)

	created := []xcl.Event{}
	for _, e := range recorder.snapshot() {
		if e.Operation == events.OperationCreate && e.ResourceID == "resource.application.api" {
			created = append(created, e)
		}
	}

	require.Len(t, created, 1)
	require.Equal(t, events.SourceCore, created[0].Source)
	require.Equal(t, events.PhaseSuccess, created[0].Phase)
	require.Equal(t, "application.api", created[0].ResourceType)
	require.Equal(t, "app.xcl", filepath.Base(created[0].File))
}

// TestAppConfigExampleReportsNoErrors asserts a successful run reports no
// error event
func TestAppConfigExampleReportsNoErrors(t *testing.T) {
	recorder := runRecordingEvents(t)

	for _, e := range recorder.snapshot() {
		require.NotEqual(t, events.PhaseError, e.Phase, "unexpected error event: %+v", e)
		require.NoError(t, e.Error, "unexpected event with an error: %+v", e)
	}
}

// capturedOutput is what was written to the process's standard output and
// standard error while a function ran
type capturedOutput struct {
	stdout string
	stderr string
}

// captureStandardStreams runs fn with os.Stdout and os.Stderr redirected to
// pipes, and returns what was written to each. The streams are restored when
// fn returns, and again in cleanup should fn fail the test
func captureStandardStreams(t *testing.T, fn func()) capturedOutput {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	restore := func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}
	t.Cleanup(restore)

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var drained sync.WaitGroup
	drained.Add(2)

	go func() {
		defer drained.Done()
		_, _ = io.Copy(stdout, stdoutReader)
	}()

	go func() {
		defer drained.Done()
		_, _ = io.Copy(stderr, stderrReader)
	}()

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	fn()

	restore()

	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())
	drained.Wait()

	require.NoError(t, stdoutReader.Close())
	require.NoError(t, stderrReader.Close())

	return capturedOutput{stdout: stdout.String(), stderr: stderr.String()}
}

// TestRunWithoutReceiverWritesNothingToStdoutOrStderr asserts xcl writes
// nothing of its own when no event handler is given, the report the example
// prints goes to out alone
func TestRunWithoutReceiverWritesNothingToStdoutOrStderr(t *testing.T) {
	out := &bytes.Buffer{}

	var runErr error
	captured := captureStandardStreams(t, func() {
		_, runErr = run(out, nil, configDir, filepath.Join(t.TempDir(), "state.json"))
	})

	require.NoError(t, runErr)
	require.Empty(t, captured.stdout)
	require.Empty(t, captured.stderr)
	require.Contains(t, out.String(), "## JSON\n")
}
