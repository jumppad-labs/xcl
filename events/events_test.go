package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newDataEvent returns an event carrying data, as a resource success event
// with event data turned on does
func newDataEvent() Event {
	return Event{
		Time:         time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC),
		Source:       SourceCore,
		Operation:    OperationCreate,
		Phase:        PhaseSuccess,
		ResourceType: "secret.literal",
		ResourceID:   "resource.secret.literal",
		Data:         []byte(`{"username":"admin"}`),
	}
}

// jsonKeys marshals the event and returns the names of its top level fields
func jsonKeys(t *testing.T, e Event) []string {
	t.Helper()

	out, err := json.Marshal(e)
	require.NoError(t, err)

	fields := map[string]any{}
	err = json.Unmarshal(out, &fields)
	require.NoError(t, err)

	keys := []string{}
	for key := range fields {
		keys = append(keys, key)
	}

	return keys
}

func TestEventMarshalsToJSONWithoutDecoder(t *testing.T) {
	plain := newDataEvent()
	decoding := newDataEvent().WithEntityDecoder(func(data []byte) (any, error) {
		return "decoded", nil
	})

	require.ElementsMatch(t, jsonKeys(t, plain), jsonKeys(t, decoding))
}

func TestEventEntityWithDataButNoDecoderFails(t *testing.T) {
	e := newDataEvent()

	entity, err := e.Entity()
	require.Error(t, err)
	require.Nil(t, entity)
}

func TestEventEntityUsesAttachedDecoder(t *testing.T) {
	received := []byte{}
	e := newDataEvent().WithEntityDecoder(func(data []byte) (any, error) {
		received = data
		return "decoded", nil
	})

	entity, err := e.Entity()
	require.NoError(t, err)
	require.Equal(t, "decoded", entity)
	require.Equal(t, `{"username":"admin"}`, string(received))
}

func TestEventEntityWithoutDataReturnsNilEvenWithDecoder(t *testing.T) {
	called := false
	e := Event{Operation: OperationApply, Phase: PhaseStart}.WithEntityDecoder(func(data []byte) (any, error) {
		called = true
		return "decoded", nil
	})

	entity, err := e.Entity()
	require.NoError(t, err)
	require.Nil(t, entity)
	require.False(t, called)
}
