package prettylog_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/plugins/registry"
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
	handler := prettylog.Handler(out, slog.LevelInfo, nil)

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
	handler := prettylog.Handler(out, slog.LevelInfo, nil)

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
	handler := prettylog.Handler(out, slog.LevelInfo, nil)

	handler(pluginDebugLog())

	require.Empty(t, out.String())
}

// TestHandlerAtDebugWritesDebugLogEvent asserts the same debug log message is
// written once the handler's level is debug
func TestHandlerAtDebugWritesDebugLogEvent(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelDebug, nil)

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
	handler := prettylog.Handler(out, slog.LevelInfo, nil)

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

// encodeFixtureRegistry returns a registry holding every type the encode
// fixture declares, a kind led registered type, a bare registered type and the
// plugin that provides the network and container types
func encodeFixtureRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	pr := registry.NewPluginRegistry()

	err := pr.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterBareType(registered.TypeCache, &registered.Cache{})
	require.NoError(t, err)

	err = pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	return pr
}

// applyEncodeFixture applies the encode fixture with a prettylog handler
// writing to out at the given level, which is the whole of what an application
// does to get configuration under its create lines
func applyEncodeFixture(t *testing.T, out *bytes.Buffer, level slog.Level) {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := encodeFixtureRegistry(t)

	c := xcl.NewConfig(
		xcl.WithPluginRegistry(pr),
		xcl.WithEventHandler(prettylog.Handler(out, level, pr)),
		xcl.WithEventData(xcl.EventDataProcessed),
	)

	path, err := filepath.Abs("../../internal/test_fixtures/config/encode/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)
}

// savedCacheRecord is one saved entity record for the bare cache type, the
// shape an event carries at xcl.EventDataProcessed
func savedCacheRecord() []byte {
	return []byte(`{"meta":{"id":"cache.main","name":"main","type":"cache"},"location":"us-east"}`)
}

// savedWidgetRecord is a saved entity record naming a type no registry in
// these tests knows, so converting it fails with xcl.ErrUnregisteredType
func savedWidgetRecord() []byte {
	return []byte(`{"meta":{"id":"resource.widget.main","name":"main","type":"resource","subtype":"widget"}}`)
}

// cacheCreateSuccess is a create success event carrying the saved cache record
func cacheCreateSuccess() events.Event {
	return events.Event{
		Time:         time.Now(),
		Source:       events.SourceCore,
		Operation:    events.OperationCreate,
		Phase:        events.PhaseSuccess,
		ResourceType: "cache.main",
		ResourceID:   "cache.main",
		File:         "main.xcl",
		Data:         savedCacheRecord(),
	}
}

// lineContaining returns the index of the first line holding every one of
// parts, and fails the test when no line holds them all
func lineContaining(t *testing.T, written []string, parts ...string) int {
	t.Helper()

	for i, line := range written {
		found := true
		for _, part := range parts {
			if !strings.Contains(line, part) {
				found = false
				break
			}
		}

		if found {
			return i
		}
	}

	require.Failf(t, "no line found", "no line contains %v in:\n%s", parts, strings.Join(written, "\n"))

	return -1
}

// TestHandlerWritesConfigurationAfterCreateSuccess asserts a real apply writes
// the created resource's configuration beneath the line announcing it, and
// that the configuration holds the value the provider filled in
func TestHandlerWritesConfigurationAfterCreateSuccess(t *testing.T) {
	out := &bytes.Buffer{}

	applyEncodeFixture(t, out, slog.LevelInfo)

	written := lines(out)

	success := lineContaining(t, written, "create success", "resource=resource.network.main")
	block := lineContaining(t, written, `resource "network" "main" {`)

	require.Greater(t, block, success, "the configuration should be written after the line announcing it")

	computed := lineContaining(t, written, "provider_id")
	require.Greater(t, computed, block, "the computed value belongs inside the block")
	require.Regexp(t, `provider_id\s+=\s+"id-main"`, written[computed])
}

// TestHandlerWritesNothingExtraForOtherEvents asserts an event that is not a
// create success writes no configuration, even when it carries the data
func TestHandlerWritesNothingExtraForOtherEvents(t *testing.T) {
	out := &bytes.Buffer{}
	pr := encodeFixtureRegistry(t)
	handler := prettylog.Handler(out, slog.LevelInfo, pr)

	start := cacheCreateSuccess()
	start.Phase = events.PhaseStart

	handler(start)

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "create start")
	require.NotContains(t, out.String(), `cache "main" {`)
}

// TestHandlerWritesNothingExtraWithoutData asserts a create success that
// carries no data writes only its own line, which is every run that has not
// asked for xcl.EventDataProcessed
func TestHandlerWritesNothingExtraWithoutData(t *testing.T) {
	out := &bytes.Buffer{}
	pr := encodeFixtureRegistry(t)
	handler := prettylog.Handler(out, slog.LevelInfo, pr)

	success := cacheCreateSuccess()
	success.Data = nil

	handler(success)

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "create success")
	require.NotContains(t, out.String(), `cache "main" {`)
}

// TestHandlerWritesNothingWhenRegistryIsNil asserts an application that passes
// no registry gets the plain lines, there being nothing to type the record with
func TestHandlerWritesNothingWhenRegistryIsNil(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo, nil)

	handler(cacheCreateSuccess())

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "create success")
	require.NotContains(t, out.String(), `cache "main" {`)
}

// TestHandlerWritesNothingAboveInfoLevel asserts a run asking only for
// warnings stays terse and writes no configuration
func TestHandlerWritesNothingAboveInfoLevel(t *testing.T) {
	out := &bytes.Buffer{}

	applyEncodeFixture(t, out, slog.LevelWarn)

	require.NotContains(t, out.String(), `resource "network" "main" {`)
}

// TestHandlerSkipsBuiltinsSilently asserts the variable the fixture declares,
// which is never written as configuration, produces no warning at all
func TestHandlerSkipsBuiltinsSilently(t *testing.T) {
	out := &bytes.Buffer{}

	applyEncodeFixture(t, out, slog.LevelInfo)

	require.NotContains(t, out.String(), "unable to show configuration")
}

// TestHandlerReportsUnconvertibleData asserts a record naming a type the
// registry does not know is reported as one warning naming the resource
func TestHandlerReportsUnconvertibleData(t *testing.T) {
	out := &bytes.Buffer{}
	pr := encodeFixtureRegistry(t)
	handler := prettylog.Handler(out, slog.LevelInfo, pr)

	unconvertible := cacheCreateSuccess()
	unconvertible.ResourceType = "widget.main"
	unconvertible.ResourceID = "resource.widget.main"
	unconvertible.Data = savedWidgetRecord()

	handler(unconvertible)

	written := lines(out)

	warning := lineContaining(t, written, "unable to show configuration")
	require.Contains(t, written[warning], "WARN")
	require.Contains(t, written[warning], "resource=resource.widget.main")
	require.Contains(t, written[warning], "error=")
}

// TestHandlerKeepsWritingAfterAFailure asserts a resource that cannot be shown
// does not stop the events after it being written
func TestHandlerKeepsWritingAfterAFailure(t *testing.T) {
	out := &bytes.Buffer{}
	pr := encodeFixtureRegistry(t)
	handler := prettylog.Handler(out, slog.LevelInfo, pr)

	unconvertible := cacheCreateSuccess()
	unconvertible.ResourceType = "widget.main"
	unconvertible.ResourceID = "resource.widget.main"
	unconvertible.Data = savedWidgetRecord()

	handler(unconvertible)
	handler(pluginLog())

	written := lines(out)

	warning := lineContaining(t, written, "unable to show configuration")
	later := lineContaining(t, written, "created database")

	require.Greater(t, later, warning, "the event after the failure should still be written")
}
