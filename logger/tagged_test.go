package logger

import (
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/stretchr/testify/require"
)

// plainLogger is a Logger that is not an event logger, it discards every call
type plainLogger struct{}

func (plainLogger) Info(msg string, args ...any)  {}
func (plainLogger) Debug(msg string, args ...any) {}
func (plainLogger) Warn(msg string, args ...any)  {}
func (plainLogger) Error(msg string, args ...any) {}

func TestWithTagNestedKeepsEveryTag(t *testing.T) {
	collector := &eventCollector{}
	pluginLogger := WithTag(New(collector.emit, events.Event{}), "plugin", "example")
	providerLogger := WithTag(pluginLogger, "provider", "postgres")

	providerLogger.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, map[string]any{
		"level":    "info",
		"message":  "calling provider",
		"plugin":   "example",
		"provider": "postgres",
	}, collector.events[0].Meta)
}

func TestWithTagNestedDoesNotAddTheInnerTagToTheOuterLogger(t *testing.T) {
	collector := &eventCollector{}
	pluginLogger := WithTag(New(collector.emit, events.Event{}), "plugin", "example")
	_ = WithTag(pluginLogger, "provider", "postgres")

	pluginLogger.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "calling provider",
		"plugin":  "example",
	}, collector.events[0].Meta)
}

func TestWithTagLaterTagWithTheSameKeyWins(t *testing.T) {
	collector := &eventCollector{}
	first := WithTag(New(collector.emit, events.Event{}), "provider", "postgres")
	second := WithTag(first, "provider", "mysql")

	second.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "mysql", collector.events[0].Meta["provider"])
}

func TestWithTagDetailGivenWithTheMessageOverridesATagWithTheSameKey(t *testing.T) {
	collector := &eventCollector{}
	l := WithTag(New(collector.emit, events.Event{}), "provider", "postgres")

	l.Info("calling provider", "provider", "mysql")

	require.Len(t, collector.events, 1)
	require.Equal(t, "mysql", collector.events[0].Meta["provider"])
}

func TestWithTagResourceIsNotAddedAsADetail(t *testing.T) {
	collector := &eventCollector{}
	l := WithTag(New(collector.emit, events.Event{}), "resource", "resource.network.frontend")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.NotContains(t, collector.events[0].Meta, "resource")
}

func TestWithTagResourceFormatsANonStringValue(t *testing.T) {
	collector := &eventCollector{}
	l := WithTag(New(collector.emit, events.Event{}), "resource", 42)

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "42", collector.events[0].ResourceID)
}

func TestWithTagReturnsANonEventLoggerUnchanged(t *testing.T) {
	plain := plainLogger{}

	tagged := WithTag(plain, "provider", "postgres")

	require.Equal(t, Logger(plain), tagged)
}

func TestWithTagReturnsNilForANilLogger(t *testing.T) {
	require.Nil(t, WithTag(nil, "plugin", "example"))
}

func TestWithSourceSetsTheSource(t *testing.T) {
	collector := &eventCollector{}
	l := WithSource(New(collector.emit, events.Event{Source: "core"}), "example")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "example", collector.events[0].Source)
}

func TestWithSourceKeepsTheRestOfTheBaseEvent(t *testing.T) {
	collector := &eventCollector{}
	base := events.Event{
		Source:       "core",
		Operation:    "create",
		ResourceType: "network.frontend",
		ResourceID:   "resource.network.frontend",
		File:         "/tmp/main.xcl",
	}
	l := WithSource(New(collector.emit, base), "example")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	e := collector.events[0]
	require.Equal(t, "create", e.Operation)
	require.Equal(t, "network.frontend", e.ResourceType)
	require.Equal(t, "resource.network.frontend", e.ResourceID)
	require.Equal(t, "/tmp/main.xcl", e.File)
}

func TestWithSourceKeepsTags(t *testing.T) {
	collector := &eventCollector{}
	tagged := WithTag(New(collector.emit, events.Event{}), "provider", "postgres")
	l := WithSource(tagged, "example")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, map[string]any{
		"level":    "info",
		"message":  "calling provider",
		"provider": "postgres",
	}, collector.events[0].Meta)
}

func TestWithSourceKeepsAResourceSetByATag(t *testing.T) {
	collector := &eventCollector{}
	tagged := WithTag(New(collector.emit, events.Event{}), "resource", "resource.network.frontend")
	l := WithSource(tagged, "example")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "resource.network.frontend", collector.events[0].ResourceID)
	require.Equal(t, "example", collector.events[0].Source)
}

func TestWithSourceDoesNotChangeTheOriginal(t *testing.T) {
	collector := &eventCollector{}
	original := New(collector.emit, events.Event{Source: "core"})
	_ = WithSource(original, "example")

	original.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "core", collector.events[0].Source)
}

func TestWithSourceTagAddedAfterwardsDoesNotReachTheOriginal(t *testing.T) {
	collector := &eventCollector{}
	original := WithTag(New(collector.emit, events.Event{}), "provider", "postgres")
	sourced := WithSource(original, "example")
	_ = WithTag(sourced, "plugin", "example")

	original.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.NotContains(t, collector.events[0].Meta, "plugin")
}

func TestWithSourceReturnsANonEventLoggerUnchanged(t *testing.T) {
	plain := plainLogger{}

	sourced := WithSource(plain, "example")

	require.Equal(t, Logger(plain), sourced)
}

func TestWithSourceReturnsNilForANilLogger(t *testing.T) {
	require.Nil(t, WithSource(nil, "example"))
}
