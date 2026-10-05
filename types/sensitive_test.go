package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

const testSecret = "s3cr3t-value"

// compile-time assertion that Sensitive implements the sealed interface.
var _ SensitiveValue = Sensitive[string]{}

type sensitiveHolder struct {
	Name     string            `json:"name"`
	Password Sensitive[string] `json:"password"`
}

type sensitiveCount struct {
	Count Sensitive[int] `json:"count"`
}

func TestNewSensitiveRevealReturnsTheWrappedValue(t *testing.T) {
	value := NewSensitive(testSecret)

	require.Equal(t, testSecret, value.Reveal())
}

func TestSensitiveFormatsVerbVAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%v", NewSensitive(testSecret)))
}

func TestSensitiveFormatsVerbPlusVAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%+v", NewSensitive(testSecret)))
}

func TestSensitiveFormatsVerbHashVAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%#v", NewSensitive(testSecret)))
}

func TestSensitiveFormatsVerbSAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%s", NewSensitive(testSecret)))
}

func TestSensitiveFormatsVerbQAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%q", NewSensitive(testSecret)))
}

func TestSensitiveFormatsVerbDAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%d", NewSensitive(testSecret)))
}

func TestSensitiveFormatsVerbXAsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", fmt.Sprintf("%x", NewSensitive(testSecret)))
}

func TestSensitiveFieldInStructFormattedWithVHidesTheSecret(t *testing.T) {
	holder := sensitiveHolder{Name: "db", Password: NewSensitive(testSecret)}

	output := fmt.Sprintf("%v", holder)

	require.Contains(t, output, SensitiveMarker)
	require.NotContains(t, output, testSecret)
}

func TestSensitiveFieldInStructFormattedWithPlusVHidesTheSecret(t *testing.T) {
	holder := sensitiveHolder{Name: "db", Password: NewSensitive(testSecret)}

	output := fmt.Sprintf("%+v", holder)

	require.Contains(t, output, SensitiveMarker)
	require.NotContains(t, output, testSecret)
}

func TestSensitiveStringReturnsTheMarker(t *testing.T) {
	require.Equal(t, "(sensitive)", NewSensitive(testSecret).String())
}

func TestSensitiveLogsAsTheMarkerWithTheTextHandler(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buffer, nil))

	logger.Info("connecting", "password", NewSensitive(testSecret))

	require.Contains(t, buffer.String(), SensitiveMarker)
	require.NotContains(t, buffer.String(), testSecret)
}

func TestSensitiveLogsAsTheMarkerWithTheJSONHandler(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))

	logger.Info("connecting", "password", NewSensitive(testSecret))

	require.Contains(t, buffer.String(), SensitiveMarker)
	require.NotContains(t, buffer.String(), testSecret)
}

func TestSensitiveMarshalTextReturnsTheMarker(t *testing.T) {
	text, err := NewSensitive(testSecret).MarshalText()

	require.NoError(t, err)
	require.Equal(t, "(sensitive)", string(text))
}

func TestSensitiveMarshalsToJSONAsTheMarker(t *testing.T) {
	holder := sensitiveHolder{Name: "db", Password: NewSensitive(testSecret)}

	data, err := json.Marshal(holder)

	require.NoError(t, err)
	require.JSONEq(t, `{"name":"db","password":"(sensitive)"}`, string(data))
	require.NotContains(t, string(data), testSecret)
}

func TestSensitiveUnmarshalsRealJSONToTheRealValue(t *testing.T) {
	var holder sensitiveHolder

	err := json.Unmarshal([]byte(`{"password":"abc"}`), &holder)

	require.NoError(t, err)
	require.Equal(t, "abc", holder.Password.Reveal())
	require.False(t, IsRedacted(holder.Password))
}

func TestSensitiveUnmarshalsTheMarkerToARedactedValue(t *testing.T) {
	var holder sensitiveHolder

	err := json.Unmarshal([]byte(`{"password":"(sensitive)"}`), &holder)

	require.NoError(t, err)
	require.True(t, IsRedacted(holder.Password))
}

func TestSensitiveUnmarshalledFromTheMarkerPrintsTheMarker(t *testing.T) {
	var holder sensitiveHolder

	err := json.Unmarshal([]byte(`{"password":"(sensitive)"}`), &holder)

	require.NoError(t, err)
	require.Equal(t, "(sensitive)", fmt.Sprintf("%v", holder.Password))
}

func TestSensitiveUnmarshalledFromTheMarkerRevealsTheZeroValue(t *testing.T) {
	var holder sensitiveHolder

	err := json.Unmarshal([]byte(`{"password":"(sensitive)"}`), &holder)

	require.NoError(t, err)
	require.Equal(t, "", holder.Password.Reveal())
}

func TestSensitiveOfANonStringUnmarshalsARealJSONNumber(t *testing.T) {
	var holder sensitiveCount

	err := json.Unmarshal([]byte(`{"count":42}`), &holder)

	require.NoError(t, err)
	require.Equal(t, 42, holder.Count.Reveal())
	require.False(t, IsRedacted(holder.Count))
}

func TestSensitiveOfANonStringUnmarshalsTheMarkerToARedactedValue(t *testing.T) {
	var holder sensitiveCount

	err := json.Unmarshal([]byte(`{"count":"(sensitive)"}`), &holder)

	require.NoError(t, err)
	require.True(t, IsRedacted(holder.Count))
	require.Equal(t, 0, holder.Count.Reveal())
}

func TestSensitiveUnmarshalRejectsAValueOfTheWrongType(t *testing.T) {
	var holder sensitiveCount

	err := json.Unmarshal([]byte(`{"count":"not a number"}`), &holder)

	require.Error(t, err)
}

func TestSensitiveRevealAnyReturnsTheRealValue(t *testing.T) {
	var value SensitiveValue = NewSensitive(testSecret)

	require.Equal(t, testSecret, value.RevealAny())
}

func TestIsRedactedIsFalseForANewSensitiveValue(t *testing.T) {
	require.False(t, IsRedacted(NewSensitive(testSecret)))
}
