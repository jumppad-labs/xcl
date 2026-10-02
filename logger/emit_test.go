package logger

import (
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/stretchr/testify/require"
)

// eventCollector records every event emitted to it
type eventCollector struct {
	events []events.Event
}

func (c *eventCollector) emit(e events.Event) {
	c.events = append(c.events, e)
}

func TestNewEmitsOneEventPerLogCall(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Debug("one")
	l.Info("two")
	l.Warn("three")
	l.Error("four")

	require.Len(t, collector.events, 4)
}

func TestNewEmitsEventsWithTheLogPhase(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{Phase: "start"})

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "log", collector.events[0].Phase)
}

func TestNewPutsTheLevelAndMessageInMeta(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	require.Len(t, collector.events, 4)
	require.Equal(t, map[string]any{"level": "debug", "message": "debug message"}, collector.events[0].Meta)
	require.Equal(t, map[string]any{"level": "info", "message": "info message"}, collector.events[1].Meta)
	require.Equal(t, map[string]any{"level": "warn", "message": "warn message"}, collector.events[2].Meta)
	require.Equal(t, map[string]any{"level": "error", "message": "error message"}, collector.events[3].Meta)
}

func TestNewKeepsTheNamesOfDetails(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Info("calling provider", "subnet", "10.0.0.0/16", "retries", 3)

	require.Len(t, collector.events, 1)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "calling provider",
		"subnet":  "10.0.0.0/16",
		"retries": 3,
	}, collector.events[0].Meta)
}

func TestNewDetailNamedLevelDoesNotOverrideTheLevel(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Debug("calling provider", "level", "error")

	require.Len(t, collector.events, 1)
	require.Equal(t, "debug", collector.events[0].Meta["level"])
}

func TestNewDetailNamedMessageDoesNotOverrideTheMessage(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Info("calling provider", "message", "something else")

	require.Len(t, collector.events, 1)
	require.Equal(t, "calling provider", collector.events[0].Meta["message"])
}

func TestNewStoresATrailingArgumentWithoutAValueUnderBadKey(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Info("calling provider", "subnet", "10.0.0.0/16", "orphan")

	require.Len(t, collector.events, 1)
	require.Equal(t, "orphan", collector.events[0].Meta["!BADKEY"])
	require.Equal(t, "10.0.0.0/16", collector.events[0].Meta["subnet"])
}

func TestNewFormatsANonStringDetailKey(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{})

	l.Info("calling provider", 42, "answer")

	require.Len(t, collector.events, 1)
	require.Equal(t, "answer", collector.events[0].Meta["42"])
}

func TestNewCopiesTheBaseEventIdentity(t *testing.T) {
	collector := &eventCollector{}
	l := New(collector.emit, events.Event{
		Source:       "example",
		Operation:    "create",
		ResourceType: "network.frontend",
		ResourceID:   "resource.network.frontend",
		File:         "/tmp/main.xcl",
	})

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	e := collector.events[0]
	require.Equal(t, "example", e.Source)
	require.Equal(t, "create", e.Operation)
	require.Equal(t, "network.frontend", e.ResourceType)
	require.Equal(t, "resource.network.frontend", e.ResourceID)
	require.Equal(t, "/tmp/main.xcl", e.File)
}

func TestNewWithANilEmitIsSilent(t *testing.T) {
	l := New(nil, events.Event{})

	require.NotPanics(t, func() {
		l.Debug("one", "key", "value")
		l.Info("two")
		l.Warn("three")
		l.Error("four")
	})
}

func TestNopIsSilent(t *testing.T) {
	l := Nop()

	require.NotPanics(t, func() {
		l.Info("calling provider", "key", "value")
	})
}

func TestWithTagResourceOnAnEventLoggerFillsResourceID(t *testing.T) {
	collector := &eventCollector{}
	l := WithTag(New(collector.emit, events.Event{}), "resource", "resource.network.frontend")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "resource.network.frontend", collector.events[0].ResourceID)
	require.Equal(t, map[string]any{"level": "info", "message": "calling provider"}, collector.events[0].Meta)
}

func TestWithTagOnAnEventLoggerAddsTheTagAsADetail(t *testing.T) {
	collector := &eventCollector{}
	l := WithTag(New(collector.emit, events.Event{}), "provider", "postgres")

	l.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, map[string]any{
		"level":    "info",
		"message":  "calling provider",
		"provider": "postgres",
	}, collector.events[0].Meta)
}

func TestWithTagOnAnEventLoggerDoesNotChangeTheOriginal(t *testing.T) {
	collector := &eventCollector{}
	original := New(collector.emit, events.Event{})
	_ = WithTag(original, "provider", "postgres")

	original.Info("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, map[string]any{"level": "info", "message": "calling provider"}, collector.events[0].Meta)
}

func TestWithTagDetailNamedLevelDoesNotOverrideTheLevel(t *testing.T) {
	collector := &eventCollector{}
	l := WithTag(New(collector.emit, events.Event{}), "level", "error")

	l.Warn("calling provider")

	require.Len(t, collector.events, 1)
	require.Equal(t, "warn", collector.events[0].Meta["level"])
}
