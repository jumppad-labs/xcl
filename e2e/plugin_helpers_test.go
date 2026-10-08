package e2e_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/types"
)

// pluginInProcessSource is the source xcl names on everything the in-process
// plugin, inprocess.Plugin, logs: the plugin's type name
const pluginInProcessSource = "Plugin"

// pluginExternalSource is the source xcl names on everything the external
// plugin logs: the name of the binary TestMain builds
const pluginExternalSource = "externalplugin"

// pluginDeclaredIDs is every resource the plugin configuration declares,
// sorted
var pluginDeclaredIDs = []string{
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

// applyPlugin applies the plugin configuration, with both plugins registered
// and state encrypted with testStateKey, and returns the applied Config.
// Events go to handler, a nil handler leaves xcl silent.
func applyPlugin(t *testing.T, handler xcl.EventHandler) *xcl.Config {
	t.Helper()

	c := newPluginConfig(t, handler, t.TempDir(), testStateKey)
	require.NoError(t, c.Apply(pluginConfigDir))

	return c
}

// pluginLifecycleEvents applies then destroys the plugin configuration and
// returns every event reported, in the order they were reported
func pluginLifecycleEvents(t *testing.T) []xcl.Event {
	t.Helper()

	recorder := &testutil.EventRecorder{}
	c := applyPlugin(t, recorder.Record)
	require.NoError(t, c.Destroy())

	return recorder.Events()
}

// pluginEntityIDs returns the ids of entities, sorted
func pluginEntityIDs(t *testing.T, entities []any) []string {
	t.Helper()

	ids := []string{}
	for _, entity := range entities {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)
	return ids
}

// pluginLogEvents returns the log events in recorded written by source
// during operation, in the order they were reported
func pluginLogEvents(recorded []xcl.Event, source, operation string) []xcl.Event {
	found := []xcl.Event{}
	for _, e := range recorded {
		if e.Phase == events.PhaseLog && e.Source == source && e.Operation == operation {
			found = append(found, e)
		}
	}

	return found
}

// pluginLogEventsWithMessage returns the events in logged whose message is
// message
func pluginLogEventsWithMessage(logged []xcl.Event, message string) []xcl.Event {
	found := []xcl.Event{}
	for _, e := range logged {
		if e.Meta[events.KeyMessage] == message {
			found = append(found, e)
		}
	}

	return found
}

// pluginLogRecord is what a test compares of a log event, the file is reduced
// to its name so the expected values do not depend on where the tests run
type pluginLogRecord struct {
	resourceID   string
	resourceType string
	file         string
	meta         map[string]any
}

// pluginLogRecords returns the log record of each event in logged
func pluginLogRecords(logged []xcl.Event) []pluginLogRecord {
	records := []pluginLogRecord{}
	for _, e := range logged {
		file := ""
		if e.File != "" {
			file = filepath.Base(e.File)
		}

		records = append(records, pluginLogRecord{
			resourceID:   e.ResourceID,
			resourceType: e.ResourceType,
			file:         file,
			meta:         e.Meta,
		})
	}

	return records
}

// pluginResourcePhases returns every phase reported for each resource in
// recorded during operation, keyed by resource id, in the order they were
// reported, including the log events providers write during the operation.
// The operation's own start and success, which concern no resource, are left
// out.
func pluginResourcePhases(recorded []xcl.Event, operation string) map[string][]string {
	phases := map[string][]string{}
	for _, e := range recorded {
		if e.Operation != operation || e.ResourceID == "" {
			continue
		}

		phases[e.ResourceID] = append(phases[e.ResourceID], e.Phase)
	}

	return phases
}
