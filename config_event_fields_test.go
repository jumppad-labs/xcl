package xcl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplyEventsCarryATimeAndTheCoreSource asserts every event an Apply
// passes to the handler has a time and the source "core"
func TestApplyEventsCarryATimeAndTheCoreSource(t *testing.T) {
	recorder := applyQueryFixtureWithEvents(t)

	require.NotEmpty(t, recorder.events)
	for _, e := range recorder.events {
		require.False(t, e.Time.IsZero(), "event %s %s %s has no time", e.ResourceID, e.Operation, e.Phase)
		require.Equal(t, "core", e.Source)
	}
}

// TestEventHasADetailsMap asserts application code can set details on an
// xcl.Event
func TestEventHasADetailsMap(t *testing.T) {
	e := Event{}
	e.Meta = map[string]any{"subnet": "10.0.0.0/16"}

	require.Equal(t, "10.0.0.0/16", e.Meta["subnet"])
}
