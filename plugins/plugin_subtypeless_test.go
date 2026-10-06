package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
)

// A block type may be registered with no subtype, written widget "<name>" {}.
// These tests register the loggingProvider for widget with an empty subtype.

func TestRegisterResourceProviderWithoutSubtypeTagsInitLogsWithType(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	pluginLogger := logger.New(recorder.Record, events.Event{Source: "WidgetPlugin"})

	err := RegisterResourceProvider(&PluginBase{}, pluginLogger, emptyState{}, "widget", "", &testResource{}, &loggingProvider{})
	require.NoError(t, err)

	logs := testutil.EventsWithPhase(recorder.Events(), events.PhaseLog)
	require.Len(t, logs, 1)
	require.Equal(t, map[string]any{
		"level":    "debug",
		"message":  "provider initialised",
		"provider": "widget",
	}, logs[0].Meta)
}

func TestPluginBaseCreatesTypeRegisteredWithoutSubtype(t *testing.T) {
	base := &PluginBase{}

	err := RegisterResourceProvider(base, logger.Nop(), emptyState{}, "widget", "", &testResource{}, &loggingProvider{})
	require.NoError(t, err)

	recorder := &testutil.EventRecorder{}
	ctx := WithLogger(context.Background(), logger.New(recorder.Record, events.Event{ResourceID: "widget.main"}))

	data, err := base.Create(ctx, "widget", "", []byte(`{"name":"main","count":3}`))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"meta": {"id": "", "name": "", "type": "", "file": "", "line": 0, "column": 0},
		"name": "main",
		"count": 3
	}`, string(data))

	// the provider's Create logs the name it was given, so the call reached it
	logs := testutil.EventsWithPhase(recorder.Events(), events.PhaseLog)
	require.Len(t, logs, 1)
	require.Equal(t, "creating test resource", logs[0].Meta["message"])
	require.Equal(t, "main", logs[0].Meta["name"])
}

func TestPluginBaseReportsUnknownTypeWithoutTrailingSeparator(t *testing.T) {
	base := &PluginBase{}

	_, err := base.Create(context.Background(), "gadget", "", []byte(`{"name":"main"}`))
	require.EqualError(t, err, "no registered type found for gadget")
}
