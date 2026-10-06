package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/logger"
)

func TestLoggerWithoutABoundLoggerReturnsALogger(t *testing.T) {
	l := Logger(context.Background())

	require.NotNil(t, l)
	require.NotPanics(t, func() {
		l.Info("creating", "name", "web")
	})
}

func TestLoggerWithoutABoundLoggerIsNop(t *testing.T) {
	l := Logger(context.Background())

	require.Equal(t, logger.Nop(), l)
}

func TestLoggerForANilContextIsNop(t *testing.T) {
	//nolint:staticcheck // a nil context is the edge being tested
	l := Logger(nil)

	require.Equal(t, logger.Nop(), l)
}

func TestLoggerForABoundNilLoggerIsNop(t *testing.T) {
	ctx := WithLogger(context.Background(), nil)

	l := Logger(ctx)

	require.Equal(t, logger.Nop(), l)
}

func TestLoggerReturnsTheBoundLogger(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	bound := logger.New(recorder.Record, events.Event{Source: "example", ResourceID: "resource.test.web"})
	ctx := WithLogger(context.Background(), bound)

	Logger(ctx).Info("creating", "name", "web")

	recorded := recorder.Events()
	require.Len(t, recorded, 1)
	require.Equal(t, "example", recorded[0].Source)
	require.Equal(t, "resource.test.web", recorded[0].ResourceID)
	require.Equal(t, events.PhaseLog, recorded[0].Phase)
	require.Equal(t, map[string]any{
		"level":   "info",
		"message": "creating",
		"name":    "web",
	}, recorded[0].Meta)
}

func TestWithLoggerDoesNotChangeTheParentContext(t *testing.T) {
	recorder := &testutil.EventRecorder{}
	parent := context.Background()
	_ = WithLogger(parent, logger.New(recorder.Record, events.Event{}))

	Logger(parent).Info("creating")

	require.Empty(t, recorder.Events())
}
