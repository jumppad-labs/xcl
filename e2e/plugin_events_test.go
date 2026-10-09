package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/state"
)

// pluginEventInProcessSource is the source of the events the in-process
// plugin writes, the name of its Go type
const pluginEventInProcessSource = "Plugin"

// pluginEventExternalSource is the source of the events the external plugin
// writes, the name of its binary
const pluginEventExternalSource = "externalplugin"

// pluginEventLifecycle applies the plugin configuration with the state
// encrypted by testStateKey, then destroys everything it applied, and returns
// every event either reported
func pluginEventLifecycle(t *testing.T) []xcl.Event {
	t.Helper()

	recorder := &testutil.EventRecorder{}
	c := newPluginConfig(t, recorder.Record, t.TempDir(), testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))
	require.NoError(t, c.Destroy())

	return recorder.Events()
}

// pluginEventPhases returns the lifecycle phases reported for each resource
// during operation, keyed by resource id, in the order they were reported. The
// operation's own start and success, which concern no resource, and the log
// events providers write during the operation are left out
func pluginEventPhases(recorded []xcl.Event, operation string) map[string][]string {
	phases := map[string][]string{}
	for _, e := range recorded {
		if e.Operation != operation || e.ResourceID == "" || e.Phase == events.PhaseLog {
			continue
		}

		phases[e.ResourceID] = append(phases[e.ResourceID], e.Phase)
	}

	return phases
}

// pluginEventLogRecord is what a test compares of a log event, the file is
// reduced to its name so the expected values do not depend on where the
// tests run
type pluginEventLogRecord struct {
	resourceID   string
	resourceType string
	file         string
	meta         map[string]any
}

// pluginEventLogRecords returns the record of each log event written during
// operation by source, in the order they were reported
func pluginEventLogRecords(recorded []xcl.Event, source, operation string) []pluginEventLogRecord {
	records := []pluginEventLogRecord{}
	for _, e := range recorded {
		if e.Phase != events.PhaseLog || e.Source != source || e.Operation != operation {
			continue
		}

		file := ""
		if e.File != "" {
			file = filepath.Base(e.File)
		}

		records = append(records, pluginEventLogRecord{
			resourceID:   e.ResourceID,
			resourceType: e.ResourceType,
			file:         file,
			meta:         e.Meta,
		})
	}

	return records
}

// pluginEventLoad is what a test compares of a load lifecycle event
type pluginEventLoad struct {
	source string
	phase  string
	meta   map[string]any
}

// pluginEventLoads returns the load lifecycle events in the order they were
// reported, leaving out the log events plugins write while they load
func pluginEventLoads(recorded []xcl.Event) []pluginEventLoad {
	found := []pluginEventLoad{}
	for _, e := range recorded {
		if e.Operation != events.OperationLoad || e.Phase == events.PhaseLog {
			continue
		}

		found = append(found, pluginEventLoad{source: e.Source, phase: e.Phase, meta: e.Meta})
	}

	return found
}

// TestPluginCreateReportsStartAndSuccessForProviderResources asserts every
// resource's create is reported, a start and a success for a resource a
// provider creates, only a success for a builtin block, which has no provider
func TestPluginCreateReportsStartAndSuccessForProviderResources(t *testing.T) {
	recorded := pluginEventLifecycle(t)

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
	}, pluginEventPhases(recorded, events.OperationCreate))
}

// TestPluginLoadReportedOnceByCore asserts loading each of the two plugins is
// reported by core, a start then a success for each, once for the whole
// lifecycle, the apply and destroy share the loaded plugins
func TestPluginLoadReportedOnceByCore(t *testing.T) {
	recorded := pluginEventLifecycle(t)

	phases := []string{}
	for _, e := range pluginEventLoads(recorded) {
		require.Equal(t, events.SourceCore, e.source)

		phases = append(phases, e.phase)
	}

	require.Equal(t, []string{"start", "success", "start", "success"}, phases)
}

// TestPluginLoadNamesPluginAndBlockTypes asserts each plugin's load names the
// plugin and the registry it came from as it starts, and names the block types it provides once it has
// loaded. The plugins load in the order they were registered
func TestPluginLoadNamesPluginAndBlockTypes(t *testing.T) {
	recorded := pluginEventLifecycle(t)

	require.Equal(t, []pluginEventLoad{
		{source: "core", phase: "start", meta: map[string]any{"plugin": pluginEventInProcessSource, "registry": "local"}},
		{source: "core", phase: "success", meta: map[string]any{"plugin": pluginEventInProcessSource, "registry": "local", "block_types": "postgres, redis"}},
		{source: "core", phase: "start", meta: map[string]any{"plugin": pluginEventExternalSource, "registry": "local"}},
		{source: "core", phase: "success", meta: map[string]any{"plugin": pluginEventExternalSource, "registry": "local", "block_types": "app, ingress, recorder"}},
	}, pluginEventLoads(recorded))
}

// TestPluginLifecycleReportsNoErrors asserts a successful apply and destroy
// with plugins reports no error event, no event carrying an error and no log
// message at error
func TestPluginLifecycleReportsNoErrors(t *testing.T) {
	recorded := pluginEventLifecycle(t)
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.NotEqual(t, events.PhaseError, e.Phase, "unexpected error event: %+v", e)
		require.NoError(t, e.Error, "unexpected event with an error: %+v", e)
		require.NotEqual(t, events.LevelError, e.Meta[events.KeyLevel], "unexpected error log: %+v", e)
	}
}

// TestPluginLifecycleWithKeyReportsNoWarnings asserts a successful apply and
// destroy with plugins and a state key reports no log message at warn, only a
// problem would stand out
func TestPluginLifecycleWithKeyReportsNoWarnings(t *testing.T) {
	recorded := pluginEventLifecycle(t)
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.NotEqual(t, events.LevelWarn, e.Meta[events.KeyLevel], "unexpected warn log: %+v", e)
	}
}

// TestPluginParseEventCarriesFile asserts each resource's parse is reported as
// a success by core with the file it was parsed from, a resource inside a
// module with the module's file
func TestPluginParseEventCarriesFile(t *testing.T) {
	recorded := pluginEventLifecycle(t)

	files := map[string]string{}
	for _, e := range recorded {
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

// TestPluginDestroyReportsStartAndSuccessForProviderResources asserts every
// resource's destroy is reported, a start and a success for a resource a
// provider destroys, only a success for a builtin block, which has no provider
func TestPluginDestroyReportsStartAndSuccessForProviderResources(t *testing.T) {
	recorded := pluginEventLifecycle(t)

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
	}, pluginEventPhases(recorded, events.OperationDestroy))
}

// TestPluginDestroyOperationReportsStartFirstAndSuccessLast asserts the
// destroy as a whole is reported by core, starting before any resource and
// succeeding after every one, providers taking part
func TestPluginDestroyOperationReportsStartFirstAndSuccessLast(t *testing.T) {
	recorded := pluginEventLifecycle(t)

	destroy := []xcl.Event{}
	for _, e := range recorded {
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

// TestInProcessProviderDestroyLogsEveryResource asserts every postgres and
// redis resource is destroyed through the in-process plugin's providers, each
// reporting a destroy log event for the resource
func TestInProcessProviderDestroyLogsEveryResource(t *testing.T) {
	recorded := pluginEventLifecycle(t)

	require.ElementsMatch(t, []pluginEventLogRecord{
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
	}, pluginEventLogRecords(recorded, pluginEventInProcessSource, events.OperationDestroy))
}

// TestExternalProviderDestroyLogsEveryResource asserts the app and ingress
// resources are destroyed through the external plugin's providers, each
// reporting a destroy log event sourced from the plugin binary's name
func TestExternalProviderDestroyLogsEveryResource(t *testing.T) {
	recorded := pluginEventLifecycle(t)

	require.ElementsMatch(t, []pluginEventLogRecord{
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
	}, pluginEventLogRecords(recorded, pluginEventExternalSource, events.OperationDestroy))
}

// TestPluginDestroyLeavesSavedStateEmpty asserts the state saved after a
// destroy through the providers is empty. The raw file is read rather than
// loaded through a store, loading it would need both plugins registered
// again, and an empty state is an empty JSON array whatever the plugins
// provide
func TestPluginDestroyLeavesSavedStateEmpty(t *testing.T) {
	stateDir := t.TempDir()
	c := newPluginConfig(t, nil, stateDir, testStateKey)

	require.NoError(t, c.Apply(pluginConfigDir))
	require.NoError(t, c.Destroy())

	saved, err := os.ReadFile(filepath.Join(stateDir, state.StateFileName))
	require.NoError(t, err)
	require.JSONEq(t, "[]", string(saved))
}
