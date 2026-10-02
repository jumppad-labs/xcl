package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newInfoSlogHandler returns a Handler writing JSON lines to a buffer through
// a slog logger at the info level, and the buffer
func newInfoSlogHandler() (Handler, *bytes.Buffer) {
	buffer := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	return SlogHandler(logger), buffer
}

// decodeLines decodes every JSON line written to buffer
func decodeLines(t *testing.T, buffer *bytes.Buffer) []map[string]any {
	t.Helper()

	lines := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
		if line == "" {
			continue
		}

		decoded := map[string]any{}
		err := json.Unmarshal([]byte(line), &decoded)
		require.NoError(t, err)

		lines = append(lines, decoded)
	}

	return lines
}

func TestSlogHandlerAtInfoDropsDebugLogEvents(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{
		Time:      time.Now(),
		Source:    "core",
		Operation: "create",
		Phase:     "log",
		Meta:      map[string]any{"level": "debug", "message": "calling provider"},
	})

	require.Empty(t, buffer.String())
}

func TestSlogHandlerAtInfoWritesInfoWarnAndErrorLogEvents(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{Source: "core", Operation: "create", Phase: "log", Meta: map[string]any{"level": "info", "message": "info message"}})
	handler(Event{Source: "core", Operation: "create", Phase: "log", Meta: map[string]any{"level": "warn", "message": "warn message"}})
	handler(Event{Source: "core", Operation: "create", Phase: "log", Meta: map[string]any{"level": "error", "message": "error message"}})

	lines := decodeLines(t, buffer)
	require.Len(t, lines, 3)

	require.Equal(t, "INFO", lines[0]["level"])
	require.Equal(t, "info message", lines[0]["msg"])

	require.Equal(t, "WARN", lines[1]["level"])
	require.Equal(t, "warn message", lines[1]["msg"])

	require.Equal(t, "ERROR", lines[2]["level"])
	require.Equal(t, "error message", lines[2]["msg"])
}

func TestSlogHandlerAtInfoWritesLifecycleStartAndSuccessEventsAtInfo(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{Source: "core", Operation: "create", Phase: "start", ResourceID: "resource.network.frontend"})
	handler(Event{Source: "core", Operation: "create", Phase: "success", ResourceID: "resource.network.frontend", Duration: 2 * time.Second})

	lines := decodeLines(t, buffer)
	require.Len(t, lines, 2)

	require.Equal(t, "INFO", lines[0]["level"])
	require.Equal(t, "create start", lines[0]["msg"])

	require.Equal(t, "INFO", lines[1]["level"])
	require.Equal(t, "create success", lines[1]["msg"])
}

func TestSlogHandlerWritesErrorPhaseEventsAtError(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{
		Source:     "core",
		Operation:  "create",
		Phase:      "error",
		ResourceID: "resource.network.frontend",
		Error:      errors.New("boom"),
	})

	lines := decodeLines(t, buffer)
	require.Len(t, lines, 1)
	require.Equal(t, "ERROR", lines[0]["level"])
	require.Equal(t, "create error", lines[0]["msg"])
	require.Equal(t, "boom", lines[0]["error"])
}

func TestSlogHandlerWritesBlockedEventsAtWarn(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{Source: "core", Operation: "events", Phase: "blocked", Duration: time.Second})

	lines := decodeLines(t, buffer)
	require.Len(t, lines, 1)
	require.Equal(t, "WARN", lines[0]["level"])
	require.Equal(t, "events blocked", lines[0]["msg"])
}

func TestSlogHandlerWritesEventFieldsAndDetailsAsAttrs(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{
		Source:       "example",
		Operation:    "create",
		Phase:        "log",
		ResourceType: "network.frontend",
		ResourceID:   "resource.network.frontend",
		File:         "/tmp/main.xcl",
		Meta: map[string]any{
			"level":   "info",
			"message": "calling provider",
			"subnet":  "10.0.0.0/16",
			"retries": 3,
		},
	})

	lines := decodeLines(t, buffer)
	require.Len(t, lines, 1)

	line := lines[0]
	require.Equal(t, "calling provider", line["msg"])
	require.Equal(t, "example", line["source"])
	require.Equal(t, "create", line["operation"])
	require.Equal(t, "log", line["phase"])
	require.Equal(t, "resource.network.frontend", line["resource"])
	require.Equal(t, "network.frontend", line["type"])
	require.Equal(t, "/tmp/main.xcl", line["file"])
	require.Equal(t, "10.0.0.0/16", line["subnet"])
	require.Equal(t, float64(3), line["retries"])
	require.NotContains(t, line, "message")
}

func TestSlogHandlerWritesDetailsInKeyOrder(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{
		Operation: "create",
		Phase:     "start",
		Meta:      map[string]any{"zeta": 1, "alpha": 2, "mid": 3},
	})

	line := buffer.String()
	alpha := strings.Index(line, `"alpha"`)
	mid := strings.Index(line, `"mid"`)
	zeta := strings.Index(line, `"zeta"`)

	require.Greater(t, alpha, 0)
	require.Greater(t, mid, alpha)
	require.Greater(t, zeta, mid)
}

func TestSlogHandlerWritesTheEventTimeAsTheRecordTime(t *testing.T) {
	handler, buffer := newInfoSlogHandler()

	handler(Event{
		Time:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Operation: "create",
		Phase:     "start",
	})

	lines := decodeLines(t, buffer)
	require.Len(t, lines, 1)
	require.Equal(t, "2026-01-02T03:04:05Z", lines[0]["time"])
}
