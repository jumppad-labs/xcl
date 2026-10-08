package prettylog_test

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/registry"
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

// encodeFixtureOptions returns the options declaring every type the encode
// fixture uses: a local registry declaring a type under resource and one
// without a subtype, and holding the plugin that provides the network and
// container types
func encodeFixtureOptions() []xcl.ConfigOption {
	local := registry.NewLocal()
	local.RegisterType(&Database{}, "resource", typeDatabase)
	local.RegisterType(&Cache{}, typeCache)
	local.RegisterPlugin(&fixturePlugin{})

	return []xcl.ConfigOption{
		xcl.WithRegistry(local),
	}
}

// applyFixture applies the fixture at path with a prettylog handler writing
// to out at the given level, at the event data level given. The handler is
// made from only the writer and the level, which is the whole of what an
// application does to get configuration under its create lines: the Config
// delivering each event is what turns its data into an entity.
func applyFixture(t *testing.T, out *bytes.Buffer, level slog.Level, data xcl.EventDataLevel, path string) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	options := append(encodeFixtureOptions(),
		xcl.WithEventHandler(prettylog.Handler(out, level)),
		xcl.WithEventData(data),
	)

	c, err := xcl.NewConfig(options...)
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)
}

// applyEncodeFixture applies the encode fixture, which declares every shape
// configuration text has to handle, asking for processed event data
func applyEncodeFixture(t *testing.T, out *bytes.Buffer, level slog.Level) {
	t.Helper()

	applyFixture(t, out, level, xcl.EventDataProcessed, "testdata/encode/main.xcl")
}

// applyCacheFixture applies the cache fixture, a single bare cache, asking
// for processed event data
func applyCacheFixture(t *testing.T, out *bytes.Buffer, level slog.Level) {
	t.Helper()

	applyFixture(t, out, level, xcl.EventDataProcessed, "testdata/cache/main.xcl")
}

// savedCacheRecord is the saved entity record for the cache fixture's cache,
// the shape an event carries at xcl.EventDataProcessed
func savedCacheRecord() []byte {
	return []byte(`{"meta":{"id":"cache.main","name":"main","type":"cache"},"location":"us-east"}`)
}

// cacheCreateSuccess is a hand-built create success event carrying the saved
// cache record. It was not delivered by a Config, so it carries no way to
// turn that record into an entity.
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

// configurationLines returns the raw lines written to out directly after the
// cache's create success line that are indented beneath it, leaving any
// styling in place
func configurationLines(t *testing.T, out *bytes.Buffer) []string {
	t.Helper()

	raw := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")

	plain := []string{}
	for _, line := range raw {
		plain = append(plain, ansiCodes.ReplaceAllString(line, ""))
	}

	success := lineContaining(t, plain, "create success", "resource=cache.main")

	configuration := []string{}
	for _, line := range raw[success+1:] {
		if !strings.HasPrefix(line, "  ") {
			break
		}
		configuration = append(configuration, line)
	}

	require.NotEmpty(t, configuration, "the configuration should follow the create success line")

	return configuration
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

// TestHandlerWritesConfigurationWithoutRegistry asserts a handler made from
// only a writer and a level, given no registry of any kind, writes a created
// resource's configuration text beneath its success line when a real Config
// delivers the event
func TestHandlerWritesConfigurationWithoutRegistry(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "")

	out := &bytes.Buffer{}

	applyCacheFixture(t, out, slog.LevelInfo)

	require.Equal(t, []string{
		`  cache "main" {`,
		`    location = "us-east"`,
		`  }`,
	}, configurationLines(t, out))
}

// TestHandlerWritesNothingExtraForOtherEvents asserts an event that is not a
// create success writes no configuration, even when it carries the data
func TestHandlerWritesNothingExtraForOtherEvents(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	start := cacheCreateSuccess()
	start.Phase = events.PhaseStart

	handler(start)

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "create start")
	require.NotContains(t, out.String(), `cache "main" {`)
}

// TestHandlerWritesNothingExtraWithoutData asserts a hand-built create success
// that carries no data writes only its own line
func TestHandlerWritesNothingExtraWithoutData(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	success := cacheCreateSuccess()
	success.Data = nil

	handler(success)

	written := lines(out)
	require.Len(t, written, 1)
	require.Contains(t, written[0], "create success")
	require.NotContains(t, out.String(), `cache "main" {`)
	require.NotContains(t, out.String(), "unable to show configuration")
}

// TestHandlerWritesNothingExtraWhenConfigSendsNoData asserts a real apply that
// has not asked for xcl.EventDataProcessed, which is every run by default,
// still writes its create success line and no configuration block
func TestHandlerWritesNothingExtraWhenConfigSendsNoData(t *testing.T) {
	out := &bytes.Buffer{}

	applyFixture(t, out, slog.LevelInfo, xcl.EventDataNone, "testdata/cache/main.xcl")

	written := lines(out)
	lineContaining(t, written, "create success", "resource=cache.main")
	require.NotContains(t, out.String(), `cache "main" {`)
	require.NotContains(t, out.String(), "unable to show configuration")
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

// TestHandlerReportsDataItCannotDecode asserts a hand-built create success
// carrying data, which no Config delivered and so has no way to become an
// entity, is reported as one warning naming the resource
func TestHandlerReportsDataItCannotDecode(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	handler(cacheCreateSuccess())

	written := lines(out)

	warning := lineContaining(t, written, "unable to show configuration")
	require.Contains(t, written[warning], "WARN")
	require.Contains(t, written[warning], "resource=cache.main")
	require.Contains(t, written[warning], "error=")
	require.NotContains(t, out.String(), `cache "main" {`)
}

// TestHandlerKeepsWritingAfterAFailure asserts a resource that cannot be shown
// does not stop the events after it being written
func TestHandlerKeepsWritingAfterAFailure(t *testing.T) {
	out := &bytes.Buffer{}
	handler := prettylog.Handler(out, slog.LevelInfo)

	handler(cacheCreateSuccess())
	handler(pluginLog())

	written := lines(out)

	warning := lineContaining(t, written, "unable to show configuration")
	later := lineContaining(t, written, "created database")

	require.Greater(t, later, warning, "the event after the failure should still be written")
}

// TestHandlerColoursConfigurationWhenColourIsForced asserts that when the
// writer is to show colour the configuration is coloured by xcl's renderer,
// and that removing the colour leaves exactly the plain configuration
func TestHandlerColoursConfigurationWhenColourIsForced(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("CLICOLOR", "")
	t.Setenv("NO_COLOR", "")

	out := &bytes.Buffer{}

	applyCacheFixture(t, out, slog.LevelInfo)

	configuration := configurationLines(t, out)

	require.True(t, strings.HasPrefix(configuration[0], "  \x1b[1;35mcache\x1b[0m"),
		"the block type should be coloured by the default theme, got %q", configuration[0])
	require.Contains(t, strings.Join(configuration, "\n"), "\x1b[34mlocation\x1b[0m")
	require.Contains(t, strings.Join(configuration, "\n"), "\x1b[33m")

	c, err := xcl.NewConfig(encodeFixtureOptions()...)
	require.NoError(t, err)

	plain, err := c.EncodeSavedEntity(savedCacheRecord(), xcl.IncludeComputed())
	require.NoError(t, err)

	expected := []string{}
	for _, line := range strings.Split(strings.TrimRight(string(plain), "\n"), "\n") {
		if line != "" {
			line = "  " + line
		}
		expected = append(expected, line)
	}

	stripped := []string{}
	for _, line := range configuration {
		stripped = append(stripped, ansiCodes.ReplaceAllString(line, ""))
	}

	require.Equal(t, expected, stripped)
	require.Equal(t, []string{
		`  cache "main" {`,
		`    location = "us-east"`,
		`  }`,
	}, stripped)
}

// TestHandlerWritesPlainConfigurationToNonTerminal asserts that a writer
// which is not a terminal, such as a buffer, gets the configuration with no
// colour at all
func TestHandlerWritesPlainConfigurationToNonTerminal(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "")

	out := &bytes.Buffer{}

	applyCacheFixture(t, out, slog.LevelInfo)

	configuration := configurationLines(t, out)

	require.Equal(t, []string{
		`  cache "main" {`,
		`    location = "us-east"`,
		`  }`,
	}, configuration)
	require.NotContains(t, strings.Join(configuration, "\n"), "\x1b")
}
