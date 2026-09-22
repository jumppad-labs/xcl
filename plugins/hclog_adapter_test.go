package plugins

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
)

// newRecordedHCLogAdapter returns an hclog adapter writing to an event logger
// whose events are recorded by the returned recorder
func newRecordedHCLogAdapter() (hclog.Logger, *eventRecorder) {
	recorder := &eventRecorder{}
	l := newHCLogAdapter(logger.New(recorder.emit, events.Event{Source: "external", Operation: events.OperationLoad}))

	return l, recorder
}

func TestHCLogAdapterPassesOnDebugAndInfoAtDebugAndWarnAndErrorAtTheirLevel(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Debug("starting plugin", "path", "build/external")
	l.Info("plugin process exited")
	l.Warn("plugin slow")
	l.Error("plugin failed", "error", "boom")

	recorded := recorder.recorded()
	require.Len(t, recorded, 4)
	require.Equal(t, map[string]any{
		"level":     "debug",
		"message":   "starting plugin",
		"component": "go-plugin",
		"path":      "build/external",
	}, recorded[0].Meta)
	require.Equal(t, map[string]any{
		"level":     "debug",
		"message":   "plugin process exited",
		"component": "go-plugin",
	}, recorded[1].Meta)
	require.Equal(t, map[string]any{
		"level":     "warn",
		"message":   "plugin slow",
		"component": "go-plugin",
	}, recorded[2].Meta)
	require.Equal(t, map[string]any{
		"level":     "error",
		"message":   "plugin failed",
		"component": "go-plugin",
		"error":     "boom",
	}, recorded[3].Meta)
}

func TestHCLogAdapterKeepsTheLoggersSourceAndOperation(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Warn("plugin slow")

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, "external", recorded[0].Source)
	require.Equal(t, events.OperationLoad, recorded[0].Operation)
	require.Equal(t, events.PhaseLog, recorded[0].Phase)
}

func TestHCLogAdapterDropsTrace(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Trace("waiting for stdio data")
	l.Log(hclog.Trace, "waiting for stdio data")

	require.Empty(t, recorder.recorded())
	require.False(t, l.IsTrace())
	require.True(t, l.IsDebug())
}

func TestHCLogAdapterLogPassesOnAtTheGivenLevel(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Log(hclog.Debug, "plugin address")
	l.Log(hclog.Info, "using plugin")
	l.Log(hclog.Warn, "plugin slow")
	l.Log(hclog.Error, "plugin failed")

	recorded := recorder.recorded()
	require.Len(t, recorded, 4)
	require.Equal(t, map[string]any{"level": "debug", "message": "plugin address", "component": "go-plugin"}, recorded[0].Meta)
	require.Equal(t, map[string]any{"level": "debug", "message": "using plugin", "component": "go-plugin"}, recorded[1].Meta)
	require.Equal(t, map[string]any{"level": "warn", "message": "plugin slow", "component": "go-plugin"}, recorded[2].Meta)
	require.Equal(t, map[string]any{"level": "error", "message": "plugin failed", "component": "go-plugin"}, recorded[3].Meta)
}

func TestHCLogAdapterWithAddsImpliedArgs(t *testing.T) {
	adapter, recorder := newRecordedHCLogAdapter()
	l := adapter.With("pid", 123).Named("external")

	l.Info("plugin started", "path", "build/external")

	require.Equal(t, []interface{}{"pid", 123}, l.ImpliedArgs())

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, map[string]any{
		"level":     "debug",
		"message":   "plugin started",
		"component": "go-plugin",
		"pid":       123,
		"path":      "build/external",
	}, recorded[0].Meta)
}

func TestHCLogAdapterNamedAppendsToTheName(t *testing.T) {
	l := newHCLogAdapter(logger.Nop()).Named("plugin").Named("stdio")

	require.Equal(t, "plugin.stdio", l.Name())
	require.Equal(t, "external", l.ResetNamed("external").Name())
}

func TestHCLogAdapterStandardWriterWritesEachLineAtDebug(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	_, err := l.StandardWriter(nil).Write([]byte("first line\nsecond line\n"))
	require.NoError(t, err)

	recorded := recorder.recorded()
	require.Len(t, recorded, 2)
	require.Equal(t, map[string]any{"level": "debug", "message": "first line", "component": "go-plugin"}, recorded[0].Meta)
	require.Equal(t, map[string]any{"level": "debug", "message": "second line", "component": "go-plugin"}, recorded[1].Meta)
}

func TestHCLogAdapterGivesEveryLogTheGoPluginComponent(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Warn("plugin slow", "pid", 123)

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, "go-plugin", recorded[0].Meta["component"])
	require.Equal(t, 123, recorded[0].Meta["pid"])
}

func TestHCLogAdapterDoesNotWriteAnEventDetail(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Warn("plugin slow")

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.NotContains(t, recorded[0].Meta, "event")
}

func TestHCLogAdapterGivesTheGoPluginComponentToAPluginTaggedLog(t *testing.T) {
	recorder := &eventRecorder{}
	tagged := logger.WithTag(logger.New(recorder.emit, events.Event{}), "plugin", "external")
	l := newHCLogAdapter(tagged)

	l.Debug("starting plugin", "path", "build/external")

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, map[string]any{
		"level":     "debug",
		"message":   "starting plugin",
		"component": "go-plugin",
		"plugin":    "external",
		"path":      "build/external",
	}, recorded[0].Meta)
}

func TestHCLogAdapterForNilLoggerDiscards(t *testing.T) {
	l := newHCLogAdapter(nil)

	require.NotPanics(t, func() {
		l.Error("plugin failed")
	})
}

func TestHCLogAdapterBuriesEndOfStdioStream(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Debug("received EOF, stopping recv loop", "err", "rpc error: code = Unavailable desc = error reading from server: EOF")

	require.Empty(t, recorder.recorded())
}

func TestHCLogAdapterPassesOnEndOfStdioStreamAboveDebug(t *testing.T) {
	l, recorder := newRecordedHCLogAdapter()

	l.Error("received EOF, stopping recv loop")

	recorded := recorder.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, map[string]any{
		"level":     "error",
		"message":   "received EOF, stopping recv loop",
		"component": "go-plugin",
	}, recorded[0].Meta)
}
