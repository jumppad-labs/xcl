package xcl

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// redactEnvelope is the JSON the default event masker writes in place of a
// sensitive value
const redactEnvelope = `{"xcl_masked":"redact","value":"(sensitive)"}`

// eventMaskHMACKey is the key the HMAC event masker tests hash with
var eventMaskHMACKey = []byte("event-mask-hmac-key")

// tagEventMasker is a test-local one way masker whose output is recognisable
// in event data
type tagEventMasker struct{}

func (tagEventMasker) Name() string { return "tag" }

func (tagEventMasker) Mask(json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`"tagged-by-test"`), nil
}

var _ mask.Masker = tagEventMasker{}

// tagEnvelope is the JSON tagEventMasker's output is written as
const tagEnvelope = `{"xcl_masked":"tag","value":"tagged-by-test"}`

// applySensitiveFixtureWithEventOptions applies the sensitive fixture with an
// event handler recording every event at the given event data level, adding
// the extra options after the defaults, and returns the recorder and the
// configuration that was applied
func applySensitiveFixtureWithEventOptions(t *testing.T, level EventDataLevel, options ...ConfigOption) (*eventRecorder, *Config) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	recorder := &eventRecorder{}

	all := []ConfigOption{
		withSecretTypes(),
		WithStateStore(store),
		WithEventHandler(recorder.Record),
		WithEventData(level),
	}
	all = append(all, options...)

	c, err := NewConfig(all...)
	require.NoError(t, err)

	err = c.Apply(sensitiveBasicPath(t))
	require.NoError(t, err)

	return recorder, c
}

// eventsWithDataFor returns every recorded event for the resource id that
// carries data
func eventsWithDataFor(t *testing.T, recorder *eventRecorder, id string) []Event {
	t.Helper()

	found := []Event{}
	for _, e := range recorder.Events() {
		if e.ResourceID == id && len(e.Data) > 0 {
			found = append(found, e)
		}
	}

	require.NotEmpty(t, found, "expected at least one event carrying data for %s", id)

	return found
}

func newHMACEventMasker(t *testing.T) mask.Masker {
	t.Helper()

	masker, err := mask.HashHMACSHA256(eventMaskHMACKey)
	require.NoError(t, err)

	return masker
}

// hashOf returns the value the HMAC event masker writes for the string
func hashOf(t *testing.T, masker mask.Masker, value string) string {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	output, err := masker.Mask(encoded)
	require.NoError(t, err)

	var hash string
	require.NoError(t, json.Unmarshal(output, &hash))
	require.NotEmpty(t, hash)

	return hash
}

func TestRawEventDataRedactsSensitiveByDefault(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataRaw)

	for _, e := range eventsWithDataFor(t, recorder, eventSensitiveSecretID) {
		require.Contains(t, string(e.Data), `"password":`+redactEnvelope, "%s %s event", e.Operation, e.Phase)
		require.NotContains(t, string(e.Data), "from-literal", "%s %s event", e.Operation, e.Phase)
	}
}

func TestProcessedEventDataRedactsSensitiveByDefault(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataProcessed)

	for _, e := range eventsWithDataFor(t, recorder, eventSensitiveSecretID) {
		require.Contains(t, string(e.Data), `"password":`+redactEnvelope, "%s %s event", e.Operation, e.Phase)
		require.NotContains(t, string(e.Data), "from-literal", "%s %s event", e.Operation, e.Phase)
	}
}

func TestCustomEventMaskerOutputInEventData(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataProcessed, WithEventMask(tagEventMasker{}))

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":`+tagEnvelope)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestHashEventMaskerIsDeterministicAcrossEvents(t *testing.T) {
	masker := newHMACEventMasker(t)
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataProcessed, WithEventMask(masker))

	hash := hashOf(t, masker, "from-literal")
	envelope := `"password":{"xcl_masked":"hmac-sha256","value":"` + hash + `"}`

	// the secret and the consumer referencing it are different resources in
	// different events, both holding the same secret
	secret := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	consumer := eventDataSingleEvent(t, recorder, "resource.secret_consumer.reference", "create", "success")

	require.Contains(t, string(secret.Data), envelope)
	require.Contains(t, string(consumer.Data), envelope)

	require.NotContains(t, string(secret.Data), "from-literal")
	require.NotContains(t, string(consumer.Data), "from-literal")
}

func TestNoEventMaskCarriesRealValues(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataProcessed, WithNoEventMask())

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":"from-literal"`)
	require.NotContains(t, string(succeeded.Data), mask.EnvelopeKey)
}

func TestLastEventMaskOptionWins(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataProcessed, WithNoEventMask(), WithEventMask(tagEventMasker{}))

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":`+tagEnvelope)
	require.NotContains(t, string(succeeded.Data), "from-literal")
}

func TestLastNoEventMaskOptionWins(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventOptions(t, EventDataProcessed, WithEventMask(tagEventMasker{}), WithNoEventMask())

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	require.Contains(t, string(succeeded.Data), `"password":"from-literal"`)
	require.NotContains(t, string(succeeded.Data), "tagged-by-test")
}

func TestNewConfigWithNilEventMaskFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c, err := NewConfig(WithEventMask(nil))

	require.Error(t, err)
	require.Nil(t, c)
}

func TestEncodeSavedEntityShowsMarkerForMaskedEventData(t *testing.T) {
	masker := newHMACEventMasker(t)
	recorder, c := applySensitiveFixtureWithEventOptions(t, EventDataProcessed, WithEventMask(masker))

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	hash := hashOf(t, masker, "from-literal")
	require.Contains(t, string(succeeded.Data), hash, "the event data must hold the hash for this test to mean anything")

	out, err := c.EncodeSavedEntity(succeeded.Data)
	require.NoError(t, err)

	require.Contains(t, string(out), `password = "(sensitive)"`)
	require.NotContains(t, string(out), "from-literal")
	require.NotContains(t, string(out), hash)
	require.NotContains(t, string(out), mask.EnvelopeKey)
}

func TestEncodeSavedEntityShowsMarkerForEncryptedState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(
		withSecretTypes(),
		WithStateStore(store),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	record := encodeSavedRecordByID(t, store.Path(), eventSensitiveSecretID)

	// the saved password is an encryption envelope, whose value is the
	// ciphertext
	fields := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(record, &fields))

	encrypted, ok := mask.IsMasked(fields["password"])
	require.True(t, ok, "the saved password must be masked data")
	require.Equal(t, "aes-256-gcm", encrypted.By)
	require.NotEmpty(t, encrypted.Value)

	var ciphertext string
	require.NoError(t, json.Unmarshal(encrypted.Value, &ciphertext))
	require.NotEmpty(t, ciphertext)

	out, err := c.EncodeSavedEntity(record)
	require.NoError(t, err)

	require.Contains(t, string(out), `password = "(sensitive)"`)
	require.NotContains(t, string(out), "from-literal")
	require.NotContains(t, string(out), ciphertext)
	require.NotContains(t, string(out), mask.EnvelopeKey)
}

// errorEvents returns every recorded event in the error phase
func errorEvents(recorder *eventRecorder) []Event {
	found := []Event{}
	for _, e := range recorder.Events() {
		if e.Phase == events.PhaseError {
			found = append(found, e)
		}
	}

	return found
}

func TestErrorRedactedWithEventMaskingOff(t *testing.T) {
	f := applyLeakFixtureWithOptions(t, "function_error/main.xcl", EventDataProcessed, nil, WithNoEventMask())

	require.Error(t, f.err)
	require.NotContains(t, f.err.Error(), knownSecret)
	require.Contains(t, f.err.Error(), types.SensitiveMarker)

	failed := errorEvents(f.recorder)
	require.NotEmpty(t, failed)

	for _, e := range failed {
		require.NotNil(t, e.Error, "%s %s event", e.ResourceID, e.Operation)
		require.NotContains(t, e.Error.Error(), knownSecret, "%s %s event", e.ResourceID, e.Operation)
		require.Contains(t, e.Error.Error(), types.SensitiveMarker, "%s %s event", e.ResourceID, e.Operation)
	}
}

// TestValidationErrorRedactedWithEventMaskingOff asserts the validation error
// names the field and does not hold the secret. No value is printed in this
// error, so the marker is not asserted.
func TestValidationErrorRedactedWithEventMaskingOff(t *testing.T) {
	f := applyLeakFixtureWithOptions(t, "plain_assignment/main.xcl", EventDataProcessed, nil, WithNoEventMask())

	require.Error(t, f.err)
	require.Contains(t, f.err.Error(), `field "note"`)
	require.NotContains(t, f.err.Error(), knownSecret)

	failed := errorEvents(f.recorder)
	require.NotEmpty(t, failed)

	for _, e := range failed {
		require.NotNil(t, e.Error, "%s %s event", e.ResourceID, e.Operation)
		require.NotContains(t, e.Error.Error(), knownSecret, "%s %s event", e.ResourceID, e.Operation)
	}
}

func TestPluginLogDetailsRedactedWithEventMaskingOff(t *testing.T) {
	messages := []parser.LogMessage{leakLogMessage()}
	f := applyLeakFixtureWithOptions(t, "main.xcl", EventDataProcessed, messages, WithNoEventMask())
	require.NoError(t, f.err)

	logged := logEvents(f.recorder)
	require.NotEmpty(t, logged)

	created := logged[0]
	require.Equal(t, "created", created.Meta[events.KeyMessage])
	require.Contains(t, created.Meta, "password")
	require.Contains(t, created.Meta, "holder")

	for _, e := range logged {
		for key, value := range e.Meta {
			if key == events.KeyLevel || key == events.KeyMessage {
				continue
			}

			text := fmt.Sprintf("%v %+v %#v", value, value, value)

			require.NotContains(t, text, knownSecret, "detail %s of the %s log event", key, e.ResourceID)
			require.Contains(t, text, types.SensitiveMarker, "detail %s of the %s log event", key, e.ResourceID)
		}
	}
}
