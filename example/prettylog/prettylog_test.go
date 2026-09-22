package prettylog_test

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/stretchr/testify/require"
)

// ansiCodes matches the escape sequences a terminal styles text with, the
// handler may colour its output even when writing to a buffer
var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// lines returns the lines written to out, with any styling removed
func lines(out *bytes.Buffer) []string {
	plain := ansiCodes.ReplaceAllString(out.String(), "")

	written := []string{}
	for _, line := range strings.Split(plain, "\n") {
		if line != "" {
			written = append(written, line)
		}
	}

	return written
}

// createStart is an info lifecycle event, core starting to create a resource
func createStart() events.Event {
	return events.Event{
		Time:         time.Now(),
		Source:       events.SourceCore,
		Operation:    events.OperationCreate,
		Phase:        events.PhaseStart,
		ResourceType: "postgres.main",
		ResourceID:   "resource.postgres.main",
		File:         "main.xcl",
	}
}

// pluginLog is an info log event a plugin wrote while creating a resource
func pluginLog() events.Event {
	return events.Event{
		Time:         time.Now(),
		Source:       "ExamplePlugin",
		Operation:    events.OperationCreate,
		Phase:        events.PhaseLog,
		ResourceType: "postgres.main",
		ResourceID:   "resource.postgres.main",
		File:         "main.xcl",
		Meta: map[string]any{
			events.KeyLevel:     events.LevelInfo,
			events.KeyMessage:   "created database",
			"connection_string": "postgres://admin@localhost:5432/main",
		},
	}
}

// pluginDebugLog is a debug log event a plugin wrote while it loaded
func pluginDebugLog() events.Event {
	return events.Event{
		Time:      time.Now(),
		Source:    "ExamplePlugin",
		Operation: events.OperationLoad,
		Phase:     events.PhaseLog,
		Meta: map[string]any{
			events.KeyLevel:   events.LevelDebug,
			events.KeyMessage: "registering block types",
			"block_types":     "postgres, redis",
		},
	}
}

func TestLevelFromEnvReadsDebug(t *testing.T) {
	t.Setenv(prettylog.LevelEnv, "debug")

	require.Equal(t, slog.LevelDebug, prettylog.LevelFromEnv())
}

func TestLevelFromEnvReadsInfo(t *testing.T) {
	t.Setenv(prettylog.LevelEnv, "info")

	require.Equal(t, slog.LevelInfo, prettylog.LevelFromEnv())
}

func TestLevelFromEnvReadsWarn(t *testing.T) {
	t.Setenv(prettylog.LevelEnv, "warn")

	require.Equal(t, slog.LevelWarn, prettylog.LevelFromEnv())
}

func TestLevelFromEnvReadsError(t *testing.T) {
	t.Setenv(prettylog.LevelEnv, "error")

	require.Equal(t, slog.LevelError, prettylog.LevelFromEnv())
}

// TestLevelFromEnvDefaultsToInfoWhenUnset asserts an empty variable, the way
// t.Setenv leaves it unset for the test, reads as info
func TestLevelFromEnvDefaultsToInfoWhenUnset(t *testing.T) {
	t.Setenv(prettylog.LevelEnv, "")

	require.Equal(t, slog.LevelInfo, prettylog.LevelFromEnv())
}

func TestLevelFromEnvDefaultsToInfoForUnknownLevel(t *testing.T) {
	t.Setenv(prettylog.LevelEnv, "verbose")

	require.Equal(t, slog.LevelInfo, prettylog.LevelFromEnv())
}

// TestHandlerWritesLifecycleEventWithSourceResourceAndOperation asserts an
// info lifecycle event is written as one line showing its severity, that it
// came from core, the resource and the operation
func TestHandlerWritesLifecycleEventWithSourceResourceAndOperation(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	handler(createStart())

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "INFO")
	require.Contains(t, written[0], "source=core")
	require.Contains(t, written[0], "resource=resource.postgres.main")
	require.Contains(t, written[0], "operation=create")
}

// TestHandlerWritesPluginLogEventWithMessageAndSource asserts a plugin's info
// log message is written as one line showing the message and the plugin
func TestHandlerWritesPluginLogEventWithMessageAndSource(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	handler(pluginLog())

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "INFO")
	require.Contains(t, written[0], "created database")
	require.Contains(t, written[0], "source=ExamplePlugin")
	require.Contains(t, written[0], "resource=resource.postgres.main")
}

// TestHandlerAtInfoWritesNothingForDebugLogEvent asserts a debug log message
// is below an info handler's level and is not written
func TestHandlerAtInfoWritesNothingForDebugLogEvent(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	handler(pluginDebugLog())

	require.Empty(t, out.String())
}

// TestHandlerAtDebugWritesDebugLogEvent asserts the same debug log message is
// written once the handler's level is debug
func TestHandlerAtDebugWritesDebugLogEvent(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelDebug)

	handler(pluginDebugLog())

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "DEBU")
	require.Contains(t, written[0], "registering block types")
	require.Contains(t, written[0], "source=ExamplePlugin")
}

// TestHandlerWritesOneLinePerEvent asserts every event of a sequence at or
// above the level is written as a line of its own
func TestHandlerWritesOneLinePerEvent(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	createSuccess := createStart()
	createSuccess.Phase = events.PhaseSuccess
	createSuccess.Duration = 5 * time.Millisecond

	sequence := []events.Event{
		{Time: time.Now(), Source: events.SourceCore, Operation: events.OperationApply, Phase: events.PhaseStart},
		createStart(),
		pluginLog(),
		createSuccess,
		{Time: time.Now(), Source: events.SourceCore, Operation: events.OperationApply, Phase: events.PhaseSuccess},
	}

	for _, e := range sequence {
		handler(e)
	}

	written := lines(out)
	require.Len(t, written, 5)
	require.Contains(t, written[0], "apply start")
	require.Contains(t, written[1], "create start")
	require.Contains(t, written[2], "created database")
	require.Contains(t, written[3], "create success")
	require.Contains(t, written[4], "apply success")
}
