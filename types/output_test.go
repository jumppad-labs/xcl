package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputUnmarshalWrapsMapLeafAtPath(t *testing.T) {
	output := Output{}

	err := json.Unmarshal([]byte(`{"value":{"username":"admin","password":"hunter2"},"sensitive_paths":[["password"]]}`), &output)
	require.NoError(t, err)

	value, ok := output.Value.(map[string]any)
	require.True(t, ok, "value is %T", output.Value)
	require.Equal(t, "admin", value["username"])

	password, ok := value["password"].(Sensitive[string])
	require.True(t, ok, "password is %T", value["password"])
	require.Equal(t, "hunter2", password.Reveal())
}

func TestOutputUnmarshalWrapsWholeValueAtEmptyPath(t *testing.T) {
	output := Output{}

	err := json.Unmarshal([]byte(`{"value":"hunter2","sensitive_paths":[[]]}`), &output)
	require.NoError(t, err)

	value, ok := output.Value.(Sensitive[string])
	require.True(t, ok, "value is %T", output.Value)
	require.Equal(t, "hunter2", value.Reveal())
}

func TestOutputUnmarshalMarkerAtPathGivesRedactedValue(t *testing.T) {
	output := Output{}

	err := json.Unmarshal([]byte(`{"value":{"password":"(sensitive)"},"sensitive_paths":[["password"]]}`), &output)
	require.NoError(t, err)

	value, ok := output.Value.(map[string]any)
	require.True(t, ok, "value is %T", output.Value)

	password, ok := value["password"].(SensitiveValue)
	require.True(t, ok, "password is %T", value["password"])
	require.True(t, IsRedacted(password))
}

func TestOutputUnmarshalPathLeadingNowhereLeavesValueUnchanged(t *testing.T) {
	output := Output{}

	err := json.Unmarshal([]byte(`{"value":{"username":"admin"},"sensitive_paths":[["missing"]]}`), &output)
	require.NoError(t, err)

	require.Equal(t, map[string]any{"username": "admin"}, output.Value)
}

func TestOutputUnmarshalWrapsListElementAtIndexPath(t *testing.T) {
	output := Output{}

	err := json.Unmarshal([]byte(`{"value":{"items":["a","b","c"]},"sensitive_paths":[["items","1"]]}`), &output)
	require.NoError(t, err)

	value, ok := output.Value.(map[string]any)
	require.True(t, ok, "value is %T", output.Value)

	items, ok := value["items"].([]any)
	require.True(t, ok, "items is %T", value["items"])
	require.Len(t, items, 3)
	require.Equal(t, "a", items[0])
	require.Equal(t, "c", items[2])

	second, ok := items[1].(Sensitive[string])
	require.True(t, ok, "items[1] is %T", items[1])
	require.Equal(t, "b", second.Reveal())
}
