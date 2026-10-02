package xcl

import (
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/stretchr/testify/require"
)

// singleNetworkConfig declares one network, the only resource a provider is
// called for
const singleNetworkConfig = `
resource "network" "one" {
  subnet = "10.0.1.0/24"
}
`

// logEvents returns every log event the recorder has received
func logEvents(recorder *eventRecorder) []Event {
	found := []Event{}
	for _, e := range recorder.snapshot() {
		if e.Phase == events.PhaseLog {
			found = append(found, e)
		}
	}

	return found
}

// logEventIndex returns the position of the first log event for id written
// during operation, or -1 when there is none
func logEventIndex(recorder *eventRecorder, id, operation string) int {
	for i, e := range recorder.snapshot() {
		if e.Phase == events.PhaseLog && e.ResourceID == id && e.Operation == operation {
			return i
		}
	}

	return -1
}

func TestPluginLogBecomesEventWithSeverityMessageAndDetail(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))
	f.plugin.SetLogOnCreate(parser.LogMessage{Level: "info", Message: "something happened", Args: []any{"remote_id", 213}})

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	logged := logEvents(recorder)
	require.Len(t, logged, 1)
	require.Equal(t, events.LevelInfo, logged[0].Meta[events.KeyLevel])
	require.Equal(t, "something happened", logged[0].Meta[events.KeyMessage])
	require.Equal(t, 213, logged[0].Meta["remote_id"])
}

func TestPluginLogCarriesResourceTypeFileAndCreateStep(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))
	f.plugin.SetLogOnCreate(parser.LogMessage{Level: "info", Message: "something happened", Args: []any{"remote_id", 213}})

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	logged := logEvents(recorder)
	require.Len(t, logged, 1)
	require.Equal(t, "resource.network.one", logged[0].ResourceID)
	require.Equal(t, "network.one", logged[0].ResourceType)
	require.Equal(t, f.configFile, logged[0].File)
	require.Equal(t, events.OperationCreate, logged[0].Operation)
}

func TestPluginLogCarriesReadStep(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))

	// the first apply creates the network, the second finds it in the state
	// and reads it
	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	f.plugin.SetLogOnRead(parser.LogMessage{Level: "info", Message: "read the network", Args: []any{"remote_id", 213}})

	err = f.config.Apply(f.configFile)
	require.NoError(t, err)
	require.Contains(t, f.plugin.GetReadResources(), "resource.network.one")

	readLogs := []Event{}
	for _, e := range logEvents(recorder) {
		if e.Operation == events.OperationRead {
			readLogs = append(readLogs, e)
		}
	}

	require.Len(t, readLogs, 1)
	require.Equal(t, "read the network", readLogs[0].Meta[events.KeyMessage])
	require.Equal(t, "resource.network.one", readLogs[0].ResourceID)
	require.Equal(t, "network.one", readLogs[0].ResourceType)
	require.Equal(t, f.configFile, readLogs[0].File)
}

func TestPluginLogNamesPluginAsSource(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))
	f.plugin.SetLogOnCreate(parser.LogMessage{Level: "info", Message: "something happened"})

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	logged := logEvents(recorder)
	require.Len(t, logged, 1)
	require.Equal(t, "TestPlugin", logged[0].Source)
}

func TestPluginLogSitsBetweenCreateStartAndSuccess(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))
	f.plugin.SetLogOnCreate(parser.LogMessage{Level: "info", Message: "something happened"})

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	startIndex := eventIndex(recorder, "resource.network.one", events.OperationCreate, events.PhaseStart)
	logIndex := logEventIndex(recorder, "resource.network.one", events.OperationCreate)
	successIndex := eventIndex(recorder, "resource.network.one", events.OperationCreate, events.PhaseSuccess)

	require.NotEqual(t, -1, startIndex, "no create start")
	require.NotEqual(t, -1, logIndex, "no create log")
	require.NotEqual(t, -1, successIndex, "no create success")
	require.Less(t, startIndex, logIndex, "the log arrived before the create start")
	require.Less(t, logIndex, successIndex, "the log arrived after the create success")
}

func TestApplyingOneResourceGivesOneCreateStartAndNoRestatingLog(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	require.Len(t, recorder.find("resource.network.one", events.OperationCreate, events.PhaseStart), 1)
	require.Empty(t, logEvents(recorder))
}

func TestPluginLogsAtEverySeverityReachReceiver(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, singleNetworkConfig, WithEventHandler(recorder.handle))
	f.plugin.SetLogOnCreate(
		parser.LogMessage{Level: "debug", Message: "a debug message"},
		parser.LogMessage{Level: "info", Message: "an info message"},
		parser.LogMessage{Level: "warn", Message: "a warn message"},
		parser.LogMessage{Level: "error", Message: "an error message"},
	)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	logged := logEvents(recorder)
	require.Len(t, logged, 4)

	levels := map[string]any{}
	for _, e := range logged {
		levels[e.Meta[events.KeyMessage].(string)] = e.Meta[events.KeyLevel]
	}

	require.Equal(t, map[string]any{
		"a debug message":  events.LevelDebug,
		"an info message":  events.LevelInfo,
		"a warn message":   events.LevelWarn,
		"an error message": events.LevelError,
	}, levels)
}
