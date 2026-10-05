package parser

import (
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// eventDataSecret has a sensitive field beside a plain one
type eventDataSecret struct {
	Username string                  `json:"username"`
	Password types.Sensitive[string] `json:"password"`
}

// eventDataPlain has no sensitive field
type eventDataPlain struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

// secretPre returns the provider-call bytes for a secret, which hold the real
// value
func secretPre(t *testing.T) ([]byte, *eventDataSecret) {
	t.Helper()

	secret := &eventDataSecret{Username: "admin", Password: types.NewSensitive("hunter2")}

	pre, err := wire.Marshal(secret)
	require.NoError(t, err)
	require.Contains(t, string(pre), "hunter2", "the provider-call bytes must hold the real value")

	return pre, secret
}

func TestEventDataRawStartShowsOnlyTheMarker(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataRaw, EventMask: mask.Redact()}, events.PhaseStart, pre, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataRawSuccessShowsOnlyTheMarker(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataRaw, EventMask: mask.Redact()}, events.PhaseSuccess, pre, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataRawErrorShowsOnlyTheMarker(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataRaw, EventMask: mask.Redact()}, events.PhaseError, pre, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataProcessedStartShowsOnlyTheMarker(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataProcessed, EventMask: mask.Redact()}, events.PhaseStart, pre, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataProcessedSuccessShowsOnlyTheMarker(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataProcessed, EventMask: mask.Redact()}, events.PhaseSuccess, pre, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataProcessedErrorShowsOnlyTheMarker(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataProcessed, EventMask: mask.Redact()}, events.PhaseError, pre, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataRawWithoutPreShowsOnlyTheMarker(t *testing.T) {
	secret := &eventDataSecret{Username: "admin", Password: types.NewSensitive("hunter2")}

	data := eventData(&ParserOptions{EventData: events.DataRaw, EventMask: mask.Redact()}, events.PhaseSuccess, nil, secret)

	require.JSONEq(t, `{"username":"admin","password":{"xcl_masked":"redact","value":"(sensitive)"}}`, string(data))
	require.NotContains(t, string(data), "hunter2")
}

func TestEventDataRawReturnsPreUnchangedForAnEntityWithoutSensitiveFields(t *testing.T) {
	plain := &eventDataPlain{Name: "web", Port: 8080}
	pre := []byte(`{"name":"web","port":8080}`)

	data := eventData(&ParserOptions{EventData: events.DataRaw}, events.PhaseStart, pre, plain)

	require.Equal(t, pre, data)
}

func TestEventDataProcessedStartReturnsPreUnchangedForAnEntityWithoutSensitiveFields(t *testing.T) {
	plain := &eventDataPlain{Name: "web", Port: 8080}
	pre := []byte("{\"port\": 8080,  \"name\":\"web\"}")

	data := eventData(&ParserOptions{EventData: events.DataProcessed}, events.PhaseStart, pre, plain)

	require.Equal(t, pre, data)
}

func TestEventDataIsEmptyAtDataNone(t *testing.T) {
	pre, secret := secretPre(t)

	data := eventData(&ParserOptions{EventData: events.DataNone}, events.PhaseStart, pre, secret)

	require.Nil(t, data)
}
