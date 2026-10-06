package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
)

// TestInProcessPluginInitLogsAtDebug asserts the message the in-process
// plugin writes from Init reaches the handler as a debug log event of the
// loading operation, sourced from the plugin
func TestInProcessPluginInitLogsAtDebug(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	loading := pluginLogEvents(recorded, pluginInProcessSource, events.OperationLoad)
	registering := pluginLogEventsWithMessage(loading, "registering block types")

	require.Equal(t, []pluginLogRecord{
		{meta: map[string]any{
			"level":       "debug",
			"message":     "registering block types",
			"block_types": "postgres, redis",
		}},
	}, pluginLogRecords(registering))
}

// TestInProcessProviderInitLogsAtDebug asserts each of the two block types
// the in-process plugin provides is registered with its own provider, each
// provider's Init message reaching the handler at debug naming its block type
func TestInProcessProviderInitLogsAtDebug(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	loading := pluginLogEvents(recorded, pluginInProcessSource, events.OperationLoad)
	ready := pluginLogEventsWithMessage(loading, "provider ready")

	require.ElementsMatch(t, []pluginLogRecord{
		{meta: map[string]any{"level": "debug", "message": "provider ready", "provider": "postgres"}},
		{meta: map[string]any{"level": "debug", "message": "provider ready", "provider": "redis"}},
	}, pluginLogRecords(ready))
}

// TestInProcessProviderCreateLogsReportedAsCreateEvents asserts the
// in-process providers' create messages reach the handler as create log
// events sourced from the plugin, naming the resource being created and the
// file it was declared in
func TestInProcessProviderCreateLogsReportedAsCreateEvents(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	require.ElementsMatch(t, []pluginLogRecord{
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
	}, pluginLogRecords(pluginLogEvents(recorded, pluginInProcessSource, events.OperationCreate)))
}

// TestExternalProviderCreateLogsSourcedFromPluginBinary asserts a message
// from the external plugin process reaches the handler as a create log event
// sourced from the plugin binary's name, naming the resource being created
// and carrying the details it was written with
func TestExternalProviderCreateLogsSourcedFromPluginBinary(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	require.ElementsMatch(t, []pluginLogRecord{
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
	}, pluginLogRecords(pluginLogEvents(recorded, pluginExternalSource, events.OperationCreate)))
}

// TestExternalProviderCreateLogsBetweenStartAndSuccess asserts the log events
// the external plugin's providers write during a create sit between the
// resource's own start and success
func TestExternalProviderCreateLogsBetweenStartAndSuccess(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	phases := pluginResourcePhases(recorded, events.OperationCreate)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.app.web"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.ingress.web"])
}

// TestInProcessProviderCreateLogsBetweenStartAndSuccess asserts the log
// events the in-process plugin's providers write during a create sit between
// the resource's own start and success
func TestInProcessProviderCreateLogsBetweenStartAndSuccess(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	phases := pluginResourcePhases(recorded, events.OperationCreate)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.main"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.replica"])
	require.Equal(t, []string{"start", "log", "success"}, phases["module.analytics.resource.postgres.analytics"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.redis.cache"])
}

// TestPluginLoadingLogsAtDebug asserts every message the plugins write while
// they load is at debug, so an info receiver shows none of them
func TestPluginLoadingLogsAtDebug(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	loading := []xcl.Event{}
	loading = append(loading, pluginLogEvents(recorded, pluginInProcessSource, events.OperationLoad)...)
	loading = append(loading, pluginLogEvents(recorded, pluginExternalSource, events.OperationLoad)...)
	require.NotEmpty(t, loading)

	for _, e := range loading {
		require.Equal(t, events.LevelDebug, e.Meta[events.KeyLevel], "unexpected level for %+v", e)
	}
}

// TestProviderCallLogsAtInfo asserts every message the providers write during
// a call is at info, so an info receiver shows what each provider did
func TestProviderCallLogsAtInfo(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	calls := []xcl.Event{}
	for _, e := range recorded {
		if e.Phase == events.PhaseLog && e.Operation != events.OperationLoad {
			calls = append(calls, e)
		}
	}

	// six provider resources, four from the in-process plugin and two from
	// the external one, each created and destroyed
	require.Len(t, calls, 12)

	for _, e := range calls {
		require.Equal(t, events.LevelInfo, e.Meta[events.KeyLevel], "unexpected level for %+v", e)
	}
}

// TestExternalProviderDestroyLogsBetweenStartAndSuccess asserts the log
// events the external plugin's providers write during a destroy sit between
// the resource's own start and success
func TestExternalProviderDestroyLogsBetweenStartAndSuccess(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	phases := pluginResourcePhases(recorded, events.OperationDestroy)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.app.web"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.ingress.web"])
}

// TestInProcessProviderDestroyLogsBetweenStartAndSuccess asserts the log
// events the in-process plugin's providers write during a destroy sit between
// the resource's own start and success
func TestInProcessProviderDestroyLogsBetweenStartAndSuccess(t *testing.T) {
	recorded := pluginLifecycleEvents(t)

	phases := pluginResourcePhases(recorded, events.OperationDestroy)
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.main"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.postgres.replica"])
	require.Equal(t, []string{"start", "log", "success"}, phases["module.analytics.resource.postgres.analytics"])
	require.Equal(t, []string{"start", "log", "success"}, phases["resource.redis.cache"])
}
