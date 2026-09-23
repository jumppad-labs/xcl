package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/plugins/registry"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/example/plugin/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// configDir is the configuration this example applies
const configDir = "./config"

// externalPlugin is the external plugin binary, built once for the package's
// tests by TestMain
var externalPlugin string

// TestMain builds the external plugin into a temporary directory, so the
// tests never depend on a binary built by hand
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "xcl-example-plugin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create build directory: %s\n", err)
		os.Exit(1)
	}

	externalPlugin = filepath.Join(dir, "external")

	build := exec.Command("go", "build", "-o", externalPlugin, "./external")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to build the external plugin: %s\n%s", err, output)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

// declaredResourceIDs is every resource the shared example configuration
// declares, sorted, the same set the configonly example finds
var declaredResourceIDs = []string{
	"module.analytics",
	"module.analytics.output.location",
	"module.analytics.resource.postgres.analytics",
	"module.analytics.variable.db_username",
	"output.web_database",
	"resource.app.web",
	"resource.ingress.web",
	"resource.postgres.main",
	"resource.postgres.replica",
	"resource.redis.cache",
	"variable.db_password",
	"variable.db_username",
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

// logEvents returns the log events written during operation by source, in
// the order they were reported
func (r *eventRecorder) logEvents(source, operation string) []xcl.Event {
	found := []xcl.Event{}
	for _, e := range r.snapshot() {
		if e.Phase == events.PhaseLog && e.Source == source && e.Operation == operation {
			found = append(found, e)
		}
	}

	return found
}

// logRecord is what a test compares of a log event, the file is reduced to
// its name so the expected values do not depend on where the tests run
type logRecord struct {
	resourceID   string
	resourceType string
	file         string
	meta         map[string]any
}

// logRecords returns the log record of each event
func logRecords(recorded []xcl.Event) []logRecord {
	records := []logRecord{}
	for _, e := range recorded {
		file := ""
		if e.File != "" {
			file = filepath.Base(e.File)
		}

		records = append(records, logRecord{
			resourceID:   e.ResourceID,
			resourceType: e.ResourceType,
			file:         file,
			meta:         e.Meta,
		})
	}

	return records
}

// runRecordingEvents runs the example with a recorder as its event handler
// and returns the recorder once the run has succeeded
func runRecordingEvents(t *testing.T) *eventRecorder {
	t.Helper()

	recorder := &eventRecorder{}

	_, err := run(&bytes.Buffer{}, recorder.handle, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	return recorder
}

func resourceIDs(t *testing.T, found []any) []string {
	t.Helper()

	ids := []string{}
	for _, r := range found {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)
	return ids
}

func TestPluginExampleFindsDeclaredResources(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

func TestPluginExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

func TestPluginExampleFillsConnectionString(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.postgres.main location=localhost port=5432 connection_string="postgres://admin@localhost:5432/main"`)
	require.Contains(t, out.String(), `resource.postgres.replica location=replica.localhost port=5433 connection_string="postgres://admin@replica.localhost:5433/main"`)
	require.Contains(t, out.String(), `module.analytics.resource.postgres.analytics location=analytics.localhost port=5432 connection_string="postgres://analytics@analytics.localhost:5432/analytics"`)
	require.Contains(t, out.String(), `resource.redis.cache location=localhost port=6379 connection_string="redis://localhost:6379"`)
}

func TestPluginExamplePassesConnectionStringToReferencingBlock(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.app.web database_location=localhost database_user=admin analytics_location=analytics.localhost connection_string="postgres://admin@localhost:5432/main" cache_connection_string="redis://localhost:6379" url="http://web"`)
}

// TestPluginExamplePassesComputedURLToIngress asserts the url the app
// provider computes reaches the ingress block, both types come from the
// external plugin
func TestPluginExamplePassesComputedURLToIngress(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), `resource.ingress.web hostname=example.com app_url="http://web"`)
}

// TestPluginExampleHoldsGeneratedTypes asserts plugin resources are held as
// types generated from the plugin's schema, so the query results printed by
// the example were copied into the shared Go types
func TestPluginExampleHoldsGeneratedTypes(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	for _, r := range found {
		_, isPostgres := r.(*resources.PostgreSQL)
		require.False(t, isPostgres, "plugin resources are held as generated types")

		_, isRedis := r.(*resources.Redis)
		require.False(t, isRedis, "plugin resources are held as generated types")

		_, isApp := r.(*resources.App)
		require.False(t, isApp, "plugin resources are held as generated types")

		_, isIngress := r.(*resources.Ingress)
		require.False(t, isIngress, "plugin resources are held as generated types")
	}
}

func TestPluginExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), "./does-not-exist", externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.Error(t, err)
}

// TestPluginExampleDefinesNoTypesOrConfig asserts the example's program and
// its two plugins hold the block types in ./resources and the configuration
// in ./config, rather than declaring either where they are used
func TestPluginExampleDefinesNoTypesOrConfig(t *testing.T) {
	for _, dir := range []string{".", "./internal", "./external"} {
		fset := token.NewFileSet()

		pkgs, err := parser.ParseDir(fset, dir, nil, 0)
		require.NoError(t, err)
		require.NotEmpty(t, pkgs)

		for _, pkg := range pkgs {
			for name, file := range pkg.Files {
				ast.Inspect(file, func(n ast.Node) bool {
					ts, ok := n.(*ast.TypeSpec)
					if !ok {
						return true
					}

					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						return true
					}

					for _, field := range st.Fields.List {
						sel, ok := field.Type.(*ast.SelectorExpr)
						if !ok {
							continue
						}

						require.NotEqual(t, "ResourceBase", sel.Sel.Name,
							"%s declares resource type %s, put it in ./resources", name, ts.Name.Name)
					}

					return true
				})
			}
		}

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)

		for _, e := range entries {
			require.NotEqual(t, ".xcl", filepath.Ext(e.Name()),
				"%s is configuration, put it in ./config", filepath.Join(dir, e.Name()))
		}
	}
}

// TestPluginExampleReportsInProcessPluginInitAtDebug asserts the message the
// in-process plugin writes from Init reaches the handler as a debug log event
// of the loading operation, sourced from the plugin
func TestPluginExampleReportsInProcessPluginInitAtDebug(t *testing.T) {
	recorder := runRecordingEvents(t)

	registering := []xcl.Event{}
	for _, e := range recorder.logEvents("ExamplePlugin", events.OperationLoad) {
		if e.Meta[events.KeyMessage] == "registering block types" {
			registering = append(registering, e)
		}
	}

	require.Equal(t, []logRecord{
		{meta: map[string]any{
			"level":       "debug",
			"message":     "registering block types",
			"block_types": "postgres, redis",
		}},
	}, logRecords(registering))
}

// TestPluginExampleReportsInProcessProviderInitAtDebug asserts each of the two
// block types the in-process plugin provides is registered with its own
// provider, each provider's Init message naming its block type
func TestPluginExampleReportsInProcessProviderInitAtDebug(t *testing.T) {
	recorder := runRecordingEvents(t)

	ready := []xcl.Event{}
	for _, e := range recorder.logEvents("ExamplePlugin", events.OperationLoad) {
		if e.Meta[events.KeyMessage] == "provider ready" {
			ready = append(ready, e)
		}
	}

	require.ElementsMatch(t, []logRecord{
		{meta: map[string]any{"level": "debug", "message": "provider ready", "provider": "postgres"}},
		{meta: map[string]any{"level": "debug", "message": "provider ready", "provider": "redis"}},
	}, logRecords(ready))
}

// TestPluginExampleReportsInProcessProviderCreateLogsAsCreateEvents asserts
// the in-process providers' create messages reach the handler as create log
// events sourced from the plugin, naming the resource being created and the
// file it was declared in
func TestPluginExampleReportsInProcessProviderCreateLogsAsCreateEvents(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.ElementsMatch(t, []logRecord{
		{
			resourceID:   "resource.postgres.main",
			resourceType: "postgres.main",
			file:         "main.xcl",
			meta: map[string]any{
				"level":             "info",
				"message":           "created database",
				"connection_string": "postgres://admin@localhost:5432/main",
			},
		},
		{
			resourceID:   "resource.postgres.replica",
			resourceType: "postgres.replica",
			file:         "main.xcl",
			meta: map[string]any{
				"level":             "info",
				"message":           "created database",
				"connection_string": "postgres://admin@replica.localhost:5433/main",
			},
		},
		{
			resourceID:   "module.analytics.resource.postgres.analytics",
			resourceType: "postgres.analytics",
			file:         "db.xcl",
			meta: map[string]any{
				"level":             "info",
				"message":           "created database",
				"connection_string": "postgres://analytics@analytics.localhost:5432/analytics",
			},
		},
		{
			resourceID:   "resource.redis.cache",
			resourceType: "redis.cache",
			file:         "main.xcl",
			meta: map[string]any{
				"level":             "info",
				"message":           "created cache",
				"connection_string": "redis://localhost:6379",
			},
		},
	}, logRecords(recorder.logEvents("ExamplePlugin", events.OperationCreate)))
}

// TestPluginExampleReportsExternalProviderCreateLogsFromThePluginBinary
// asserts a message from the external plugin process reaches the handler as
// a create log event sourced from the plugin binary's name, naming the
// resource being created and carrying the details it was written with
func TestPluginExampleReportsExternalProviderCreateLogsFromThePluginBinary(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.ElementsMatch(t, []logRecord{
		{
			resourceID:   "resource.app.web",
			resourceType: "app.web",
			file:         "main.xcl",
			meta: map[string]any{
				"level":                   "info",
				"message":                 "created app",
				"connection_string":       "postgres://admin@localhost:5432/main",
				"cache_connection_string": "redis://localhost:6379",
				"url":                     "http://web",
			},
		},
		{
			resourceID:   "resource.ingress.web",
			resourceType: "ingress.web",
			file:         "main.xcl",
			meta: map[string]any{
				"level":    "info",
				"message":  "created ingress",
				"hostname": "example.com",
				"app_url":  "http://web",
			},
		},
	}, logRecords(recorder.logEvents("external", events.OperationCreate)))
}

// TestPluginExampleFailsWithoutExternalPlugin asserts a missing external
// plugin binary fails the run. Registering the path only records it, the
// plugin is started by the first Apply, which fails naming the path, and the
// example adds how to build the plugin
func TestPluginExampleFailsWithoutExternalPlugin(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, "./does-not-exist", filepath.Join(t.TempDir(), "state.json"))
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrPluginLoad)
	require.Contains(t, err.Error(), "./does-not-exist")
	require.Contains(t, err.Error(), ", build it with `make build` in example/plugin")
}

// allEventPhases returns every phase reported for each resource during
// operation, keyed by resource id, in the order they were reported, including
// the log events providers write during the operation
func allEventPhases(recorder *eventRecorder, operation string) map[string][]string {
	phases := map[string][]string{}
	for _, e := range recorder.snapshot() {
		if e.Operation != operation {
			continue
		}

		// the operation's own start and success concern no resource
		if e.ResourceID == "" {
			continue
		}

		phases[e.ResourceID] = append(phases[e.ResourceID], e.Phase)
	}

	return phases
}

// eventPhases returns the lifecycle phases reported for each resource during
// operation, keyed by resource id, leaving out the log events providers write
// during the operation
func eventPhases(recorder *eventRecorder, operation string) map[string][]string {
	phases := map[string][]string{}
	for id, all := range allEventPhases(recorder, operation) {
		for _, phase := range all {
			if phase == events.PhaseLog {
				continue
			}

			phases[id] = append(phases[id], phase)
		}
	}

	return phases
}

// TestPluginExampleReportsCreateEventPhases asserts every resource's create
// is reported, a start and a success for a resource a provider creates, only
// a success for a builtin block, which has no provider
func TestPluginExampleReportsCreateEventPhases(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.Equal(t, map[string][]string{
		"resource.postgres.main":                       {"start", "success"},
		"resource.postgres.replica":                    {"start", "success"},
		"module.analytics.resource.postgres.analytics": {"start", "success"},
		"resource.redis.cache":                         {"start", "success"},
		"resource.app.web":                             {"start", "success"},
		"resource.ingress.web":                         {"start", "success"},
		"module.analytics":                             {"success"},
		"module.analytics.output.location":             {"success"},
		"module.analytics.variable.db_username":        {"success"},
		"output.web_database":                          {"success"},
		"variable.db_password":                         {"success"},
		"variable.db_username":                         {"success"},
	}, eventPhases(recorder, events.OperationCreate))
}

// TestPluginExampleReportsExternalProviderCreateLogsBetweenStartAndSuccess
// asserts the log events the external plugin's providers write during a
// create sit between the resource's start and success
func TestPluginExampleReportsExternalProviderCreateLogsBetweenStartAndSuccess(t *testing.T) {
	recorder := runRecordingEvents(t)

	phases := allEventPhases(recorder, events.OperationCreate)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.app.web"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.ingress.web"])
}

// TestPluginExampleReportsInProcessProviderCreateLogsBetweenStartAndSuccess
// asserts the log events the in-process plugin's providers write during a
// create sit between the resource's start and success
func TestPluginExampleReportsInProcessProviderCreateLogsBetweenStartAndSuccess(t *testing.T) {
	recorder := runRecordingEvents(t)

	phases := allEventPhases(recorder, events.OperationCreate)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.main"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.replica"])
	require.Equal(t, []string{"start", "log", "success"}, phases["module.analytics.resource.postgres.analytics"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.redis.cache"])
}

// loadEvent is what a test compares of a load lifecycle event
type loadEvent struct {
	source string
	phase  string
	meta   map[string]any
}

// loadEvents returns the load lifecycle events in the order they were
// reported, leaving out the log events plugins write while they load
func loadEvents(recorder *eventRecorder) []loadEvent {
	found := []loadEvent{}
	for _, e := range recorder.snapshot() {
		if e.Operation != events.OperationLoad || e.Phase == events.PhaseLog {
			continue
		}

		found = append(found, loadEvent{source: e.Source, phase: e.Phase, meta: e.Meta})
	}

	return found
}

// TestPluginExampleReportsPluginsLoaded asserts loading each of the two
// plugins is reported by core, a start then a success for each, once for the
// whole run, the apply and destroy share the loaded plugins
func TestPluginExampleReportsPluginsLoaded(t *testing.T) {
	recorder := runRecordingEvents(t)

	phases := []string{}
	for _, e := range loadEvents(recorder) {
		require.Equal(t, events.SourceCore, e.source)

		phases = append(phases, e.phase)
	}

	require.Equal(t, []string{"start", "success", "start", "success"}, phases)
}

// TestPluginExampleReportsBlockTypesOfLoadedPlugins asserts each plugin's
// load names the plugin as it starts, and names the block types it provides
// once it has loaded
func TestPluginExampleReportsBlockTypesOfLoadedPlugins(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.Equal(t, []loadEvent{
		{source: "core", phase: "start", meta: map[string]any{"plugin": "ExamplePlugin"}},
		{source: "core", phase: "success", meta: map[string]any{"plugin": "ExamplePlugin", "block_types": "postgres, redis"}},
		{source: "core", phase: "start", meta: map[string]any{"plugin": "external"}},
		{source: "core", phase: "success", meta: map[string]any{"plugin": "external", "block_types": "app, ingress"}},
	}, loadEvents(recorder))
}

// TestPluginExampleReportsNoErrors asserts a successful run reports no error
// event and no log message at error
func TestPluginExampleReportsNoErrors(t *testing.T) {
	recorder := runRecordingEvents(t)

	for _, e := range recorder.snapshot() {
		require.NotEqual(t, events.PhaseError, e.Phase, "unexpected error event: %+v", e)
		require.NoError(t, e.Error, "unexpected event with an error: %+v", e)
		require.NotEqual(t, events.LevelError, e.Meta[events.KeyLevel], "unexpected error log: %+v", e)
	}
}

// TestPluginExampleReportsNoWarnings asserts a successful run reports no log
// message at warn, only a problem would stand out
func TestPluginExampleReportsNoWarnings(t *testing.T) {
	recorder := runRecordingEvents(t)

	for _, e := range recorder.snapshot() {
		require.NotEqual(t, events.LevelWarn, e.Meta[events.KeyLevel], "unexpected warn log: %+v", e)
	}
}

// TestPluginExampleReportsPluginLoadingLogsAtDebug asserts every message the
// plugins write while they load is at debug, so an info receiver shows none
// of them
func TestPluginExampleReportsPluginLoadingLogsAtDebug(t *testing.T) {
	recorder := runRecordingEvents(t)

	loading := []xcl.Event{}
	loading = append(loading, recorder.logEvents("ExamplePlugin", events.OperationLoad)...)
	loading = append(loading, recorder.logEvents("external", events.OperationLoad)...)
	require.NotEmpty(t, loading)

	for _, e := range loading {
		require.Equal(t, events.LevelDebug, e.Meta[events.KeyLevel], "unexpected level for %+v", e)
	}
}

// TestPluginExampleReportsProviderCallLogsAtInfo asserts every message the
// providers write during a call is at info, so an info receiver shows what
// each provider did
func TestPluginExampleReportsProviderCallLogsAtInfo(t *testing.T) {
	recorder := runRecordingEvents(t)

	calls := []xcl.Event{}
	for _, e := range recorder.snapshot() {
		if e.Phase == events.PhaseLog && e.Operation != events.OperationLoad {
			calls = append(calls, e)
		}
	}

	// four resources from each plugin, created and destroyed
	require.Len(t, calls, 12)

	for _, e := range calls {
		require.Equal(t, events.LevelInfo, e.Meta[events.KeyLevel], "unexpected level for %+v", e)
	}
}

// TestPluginExampleReportsParseEventWithFile asserts each resource's parse is
// reported as a success by core with the file it was parsed from
func TestPluginExampleReportsParseEventWithFile(t *testing.T) {
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
		"variable.db_username":                         "main.xcl",
		"variable.db_password":                         "main.xcl",
		"resource.postgres.main":                       "main.xcl",
		"resource.postgres.replica":                    "main.xcl",
		"resource.redis.cache":                         "main.xcl",
		"module.analytics":                             "main.xcl",
		"resource.app.web":                             "main.xcl",
		"resource.ingress.web":                         "main.xcl",
		"output.web_database":                          "main.xcl",
		"module.analytics.variable.db_username":        "db.xcl",
		"module.analytics.resource.postgres.analytics": "db.xcl",
		"module.analytics.output.location":             "db.xcl",
	}, files)
}

// TestPluginExampleDestroysEverythingItApplied asserts the state saved after a run is
// empty. The raw file is read rather than loaded through a store, loading it
// would need both plugins registered again, and an empty state is an empty
// JSON array whatever the plugins provide
func TestPluginExampleDestroysEverythingItApplied(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	_, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, externalPlugin, statePath)
	require.NoError(t, err)

	saved, err := os.ReadFile(statePath)
	require.NoError(t, err)
	require.JSONEq(t, "[]", string(saved))
}

func TestPluginExamplePrintsNoResourcesRemaining(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Destroyed\n  0 resources remaining\n")
}

// TestPluginExampleInProcessProvidersReportDestroyForEveryResource asserts
// every postgres and redis resource is destroyed through the in-process
// plugin's providers, each reporting a destroy log event for the resource
func TestPluginExampleInProcessProvidersReportDestroyForEveryResource(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.ElementsMatch(t, []logRecord{
		{
			resourceID:   "resource.postgres.main",
			resourceType: "postgres.main",
			file:         "main.xcl",
			meta:         map[string]any{"level": "info", "message": "destroyed database", "force": false},
		},
		{
			resourceID:   "resource.postgres.replica",
			resourceType: "postgres.replica",
			file:         "main.xcl",
			meta:         map[string]any{"level": "info", "message": "destroyed database", "force": false},
		},
		{
			resourceID:   "module.analytics.resource.postgres.analytics",
			resourceType: "postgres.analytics",
			file:         "db.xcl",
			meta:         map[string]any{"level": "info", "message": "destroyed database", "force": false},
		},
		{
			resourceID:   "resource.redis.cache",
			resourceType: "redis.cache",
			file:         "main.xcl",
			meta:         map[string]any{"level": "info", "message": "destroyed cache", "force": false},
		},
	}, logRecords(recorder.logEvents("ExamplePlugin", events.OperationDestroy)))
}

// TestPluginExampleExternalProvidersReportDestroyForEveryResource asserts
// the app and ingress resources are destroyed through the external plugin's
// providers, each reporting a destroy log event sourced from the plugin
// binary's name
func TestPluginExampleExternalProvidersReportDestroyForEveryResource(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.ElementsMatch(t, []logRecord{
		{
			resourceID:   "resource.app.web",
			resourceType: "app.web",
			file:         "main.xcl",
			meta:         map[string]any{"level": "info", "message": "destroyed app", "force": false},
		},
		{
			resourceID:   "resource.ingress.web",
			resourceType: "ingress.web",
			file:         "main.xcl",
			meta:         map[string]any{"level": "info", "message": "destroyed ingress", "force": false},
		},
	}, logRecords(recorder.logEvents("external", events.OperationDestroy)))
}

// TestPluginExampleReportsDestroyEventPhases asserts every resource's destroy
// is reported, a start and a success for a resource a provider destroys, only
// a success for a builtin block, which has no provider
func TestPluginExampleReportsDestroyEventPhases(t *testing.T) {
	recorder := runRecordingEvents(t)

	require.Equal(t, map[string][]string{
		"resource.postgres.main":                       {"start", "success"},
		"resource.postgres.replica":                    {"start", "success"},
		"module.analytics.resource.postgres.analytics": {"start", "success"},
		"resource.redis.cache":                         {"start", "success"},
		"resource.app.web":                             {"start", "success"},
		"resource.ingress.web":                         {"start", "success"},
		"module.analytics":                             {"success"},
		"module.analytics.output.location":             {"success"},
		"module.analytics.variable.db_username":        {"success"},
		"output.web_database":                          {"success"},
		"variable.db_password":                         {"success"},
		"variable.db_username":                         {"success"},
	}, eventPhases(recorder, events.OperationDestroy))
}

// TestPluginExampleReportsDestroyOperationStartAndSuccess asserts the destroy
// as a whole is reported by core, starting before any resource and
// succeeding after every one
func TestPluginExampleReportsDestroyOperationStartAndSuccess(t *testing.T) {
	recorder := runRecordingEvents(t)

	destroy := []xcl.Event{}
	for _, e := range recorder.snapshot() {
		if e.Operation == events.OperationDestroy {
			destroy = append(destroy, e)
		}
	}

	require.NotEmpty(t, destroy)

	first := destroy[0]
	require.Equal(t, events.SourceCore, first.Source)
	require.Equal(t, events.PhaseStart, first.Phase)
	require.Empty(t, first.ResourceID)

	last := destroy[len(destroy)-1]
	require.Equal(t, events.SourceCore, last.Source)
	require.Equal(t, events.PhaseSuccess, last.Phase)
	require.Empty(t, last.ResourceID)
}

// TestPluginExampleReportsExternalProviderDestroyLogsBetweenStartAndSuccess
// asserts the log events the external plugin's providers write during a
// destroy sit between the resource's start and success
func TestPluginExampleReportsExternalProviderDestroyLogsBetweenStartAndSuccess(t *testing.T) {
	recorder := runRecordingEvents(t)

	phases := allEventPhases(recorder, events.OperationDestroy)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.app.web"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.ingress.web"])
}

// TestPluginExampleReportsInProcessProviderDestroyLogsBetweenStartAndSuccess
// asserts the log events the in-process plugin's providers write during a
// destroy sit between the resource's start and success
func TestPluginExampleReportsInProcessProviderDestroyLogsBetweenStartAndSuccess(t *testing.T) {
	recorder := runRecordingEvents(t)

	phases := allEventPhases(recorder, events.OperationDestroy)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.main"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.replica"])
	require.Equal(t, []string{"start", "log", "success"}, phases["module.analytics.resource.postgres.analytics"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.redis.cache"])
}

// TestPluginExampleRetrievesPublishedValues asserts a value the configuration
// publishes is read back by its address, and comes back as the value itself
// rather than the declaration that produced it. Both the root configuration
// and the module publish one
func TestPluginExampleRetrievesPublishedValues(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Published\n")
	require.Contains(t, out.String(), "  output.web_database=\"localhost\"\n")
	require.Contains(t, out.String(), "  module.analytics.output.location=\"analytics.localhost\"\n")
}

// TestPluginExamplePrintsPublishedTotal asserts every published value is
// reachable at once, the total counting the module's output alongside the
// root's
func TestPluginExamplePrintsPublishedTotal(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "  2 published in total\n")
}

// methodFormLookups are the lookups that the configuration also offers as
// generic methods from Go 1.27, the form an example must not be written in
var methodFormLookups = []string{"Find", "FindByType", "FindOne", "All"}

// TestPluginExampleUsesPortableLookupForm asserts the example looks entities
// up through the package level functions rather than the generic methods of
// the same name. Generic methods arrived in Go 1.27, so the method form would
// stop the code a reader copies from compiling on the project's minimum
// supported version. Entities, EntityCount and Outputs are ordinary methods
// and are not affected
func TestPluginExampleUsesPortableLookupForm(t *testing.T) {
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

// TestRunWithoutReceiverWritesNothingToStdoutOrStderr asserts xcl, both
// plugins and the external plugin process write nothing of their own when no
// event handler is given, the report the example prints goes to out alone
func TestRunWithoutReceiverWritesNothingToStdoutOrStderr(t *testing.T) {
	out := &bytes.Buffer{}

	var runErr error
	captured := captureStandardStreams(t, func() {
		_, runErr = run(out, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	})

	require.NoError(t, runErr)
	require.Empty(t, captured.stdout)
	require.Empty(t, captured.stderr)
	require.Contains(t, out.String(), "## Resources\n")
}

// renderEvents runs the example with the pretty printer the program itself
// uses, writing to a buffer rather than the terminal, and returns everything
// it wrote. The registry is shared with the printer exactly as main shares
// it, since it is what types the entity an event carries
func renderEvents(t *testing.T) string {
	t.Helper()

	r := registry.NewPluginRegistry()
	rendered := &bytes.Buffer{}

	_, err := run(&bytes.Buffer{}, prettylog.Handler(rendered, slog.LevelInfo, r), r, configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	return rendered.String()
}

// TestPluginExampleShowsPostgresConfigurationAfterCreate asserts the
// configuration of a created resource is written beneath the line announcing
// it, holding the values that were configured, the nested block, and the
// connection string the provider filled in
func TestPluginExampleShowsPostgresConfigurationAfterCreate(t *testing.T) {
	rendered := renderEvents(t)

	created := strings.Index(rendered, "create success source=core operation=create phase=success resource=resource.postgres.main")
	require.NotEqual(t, -1, created, "the create success of resource.postgres.main was not reported")

	block := strings.Index(rendered, `resource "postgres" "main" {`)
	require.NotEqual(t, -1, block, "the configuration of resource.postgres.main was not shown")

	require.Greater(t, block, created, "the configuration was shown before the line announcing the create")

	// the formatter aligns the equals signs to the longest name in the block,
	// so the gap before one is matched rather than written out
	require.Regexp(t, `port\s+= 5432`, rendered)
	require.Contains(t, rendered, "timeouts {")
	require.Regexp(t, `connection_string\s+=\s+"postgres://admin@localhost:5432/main"`, rendered)
}

// TestPluginExampleShowsEveryCreatedEntity asserts every resource the example
// creates has its configuration shown, including the one declared inside a
// module
func TestPluginExampleShowsEveryCreatedEntity(t *testing.T) {
	rendered := renderEvents(t)

	require.Contains(t, rendered, `resource "postgres" "main" {`)
	require.Contains(t, rendered, `resource "postgres" "replica" {`)
	require.Contains(t, rendered, `resource "postgres" "analytics" {`)
	require.Contains(t, rendered, `resource "redis" "cache" {`)
	require.Contains(t, rendered, `resource "app" "web" {`)
	require.Contains(t, rendered, `resource "ingress" "web" {`)
}

// TestPluginExampleConvertsExternalPluginTypes asserts a type provided by the
// external plugin binary, which xcl holds as a type generated from the
// plugin's schema, is written as configuration just as an in-process type is
func TestPluginExampleConvertsExternalPluginTypes(t *testing.T) {
	rendered := renderEvents(t)

	require.Contains(t, rendered, `resource "app" "web" {`)
	require.Contains(t, rendered, `resource "ingress" "web" {`)
}

// stateAtApply captures the state file as it stood when the apply succeeded.
// The run destroys everything it applied before it returns, which leaves the
// file holding an empty array, so the records have to be read while they are
// still there
type stateAtApply struct {
	path    string
	records []json.RawMessage
	err     error
}

// handle reads the state file once the apply has succeeded, which is after
// the state was saved and before the destroy empties it again
func (s *stateAtApply) handle(e xcl.Event) {
	if e.Operation != events.OperationApply || e.Phase != events.PhaseSuccess {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		s.err = err
		return
	}

	s.err = json.Unmarshal(data, &s.records)
}

// savedID returns the address a saved record carries, which is how a record
// is matched to the entity it was written from
func savedID(t *testing.T, record json.RawMessage) string {
	t.Helper()

	var envelope struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}

	require.NoError(t, json.Unmarshal(record, &envelope))
	require.NotEmpty(t, envelope.Meta.ID)

	return envelope.Meta.ID
}

// entityWithID returns the entity whose address is id
func entityWithID(t *testing.T, entities []any, id string) any {
	t.Helper()

	for _, entity := range entities {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		if meta.ID == id {
			return entity
		}
	}

	require.Failf(t, "entity not found", "the run returned no entity with the address %s", id)
	return nil
}

// TestPluginExampleEntityAndStateAgree asserts the configuration text of a
// saved record is identical to the text of the entity it was written from,
// for every record the apply saved. A variable, output or module is never
// written as configuration, so those records are skipped
func TestPluginExampleEntityAndStateAgree(t *testing.T) {
	r := registry.NewPluginRegistry()
	statePath := filepath.Join(t.TempDir(), "state.json")
	saved := &stateAtApply{path: statePath}

	applied, err := run(&bytes.Buffer{}, saved.handle, r, configDir, externalPlugin, statePath)
	require.NoError(t, err)

	require.NoError(t, saved.err)
	require.NotEmpty(t, saved.records)

	compared := 0

	for _, record := range saved.records {
		fromState, err := xcl.EncodeSavedEntity(r, record)
		if errors.Is(err, xcl.ErrNotEncodable) {
			continue
		}
		require.NoError(t, err)

		id := savedID(t, record)

		fromEntity, err := xcl.EncodeEntity(entityWithID(t, applied, id))
		require.NoError(t, err)

		require.Equal(t, string(fromEntity), string(fromState), "the saved record and the entity disagree for %s", id)

		compared++
	}

	require.NotZero(t, compared, "no record was encodable, so the comparison proves nothing")
}

// TestPluginExampleEveryEntityConverts asserts every entity the run returns
// either converts to configuration text or is refused as not encodable, which
// is what a variable, output or module is. No other failure is allowed
func TestPluginExampleEveryEntityConverts(t *testing.T) {
	applied, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, externalPlugin, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)
	require.NotEmpty(t, applied)

	converted := 0

	for _, entity := range applied {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		text, err := xcl.EncodeEntity(entity)
		if err != nil {
			require.ErrorIs(t, err, xcl.ErrNotEncodable, "%s failed for another reason", meta.ID)
			require.Empty(t, text, "%s returned text with its failure", meta.ID)
			continue
		}

		require.NotEmpty(t, text, "%s converted to nothing", meta.ID)

		converted++
	}

	require.NotZero(t, converted, "nothing converted, so the check proves nothing")
}
