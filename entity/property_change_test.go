package entity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func networkNameChange() PropertyChange {
	return PropertyChange{
		Path:   Path{}.Attribute("network").Index(0).Attribute("name"),
		Before: "app",
		After:  "backend",
	}
}

func sensitivePasswordChange() PropertyChange {
	return PropertyChange{
		Path:      Path{}.Attribute("env").Key("DB_PASSWORD"),
		Before:    "old-secret-value",
		After:     "new-secret-value",
		Sensitive: true,
	}
}

func TestPropertyChangeAtIsTrueForItsPath(t *testing.T) {
	change := networkNameChange()

	require.True(t, change.At(Path{}.Attribute("network").Index(0).Attribute("name")))
}

func TestPropertyChangeAtIsFalseForParentPath(t *testing.T) {
	change := networkNameChange()

	require.False(t, change.At(Path{}.Attribute("network")))
}

func TestPropertyChangeWithinIsTrueForContainingSetting(t *testing.T) {
	change := networkNameChange()

	require.True(t, change.Within(Path{}.Attribute("network")))
}

func TestPropertyChangeWithinIsFalseForUnrelatedSetting(t *testing.T) {
	change := networkNameChange()

	require.False(t, change.Within(Path{}.Attribute("image")))
}

func TestPropertyChangeStringShowsRealValuesWhenNotSensitive(t *testing.T) {
	change := networkNameChange()

	require.Equal(t, `network[0].name: "app" -> "backend"`, change.String())
}

func TestPropertyChangeStringShowsNullForAddedSetting(t *testing.T) {
	change := PropertyChange{
		Path:  Path{}.Attribute("image"),
		After: "nginx",
	}

	require.Equal(t, `image: null -> "nginx"`, change.String())
}

func TestPropertyChangeMarshalJSONShowsRealValuesWhenNotSensitive(t *testing.T) {
	encoded, err := json.Marshal(networkNameChange())
	require.NoError(t, err)

	require.JSONEq(t,
		`{"path":"network[0].name","before":"app","after":"backend","unknown":false,"sensitive":false}`,
		string(encoded),
	)
}

func TestPropertyChangeStringHidesSensitiveValues(t *testing.T) {
	change := sensitivePasswordChange()

	require.Equal(t, `env["DB_PASSWORD"]: (sensitive) -> (sensitive)`, change.String())
}

func TestPropertyChangeFormatVerbHidesSensitiveValues(t *testing.T) {
	printed := fmt.Sprintf("%v", sensitivePasswordChange())

	require.NotContains(t, printed, "secret-value")
	require.Contains(t, printed, "(sensitive)")
}

func TestPropertyChangeFormatPlusVerbHidesSensitiveValues(t *testing.T) {
	printed := fmt.Sprintf("%+v", sensitivePasswordChange())

	require.NotContains(t, printed, "secret-value")
	require.Contains(t, printed, "(sensitive)")
}

func TestPropertyChangeFormatGoSyntaxVerbHidesSensitiveValues(t *testing.T) {
	printed := fmt.Sprintf("%#v", sensitivePasswordChange())

	require.NotContains(t, printed, "secret-value")
	require.Contains(t, printed, "(sensitive)")
}

func TestPropertyChangeFormatHidesSensitiveValuesInsideSlice(t *testing.T) {
	printed := fmt.Sprintf("%v", []PropertyChange{sensitivePasswordChange()})

	require.NotContains(t, printed, "secret-value")
	require.Contains(t, printed, "(sensitive)")
}

func TestPropertyChangeLogValueHidesSensitiveValues(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))

	logger.Info("property changed", "change", sensitivePasswordChange())

	logged := buffer.String()
	require.NotContains(t, logged, "secret-value")
	require.Contains(t, logged, `"before":"(sensitive)"`)
	require.Contains(t, logged, `"after":"(sensitive)"`)
	require.Contains(t, logged, `"path":"env[\"DB_PASSWORD\"]"`)
}

func TestPropertyChangeLogValueShowsRealValuesWhenNotSensitive(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))

	logger.Info("property changed", "change", networkNameChange())

	logged := buffer.String()
	require.Contains(t, logged, `"before":"app"`)
	require.Contains(t, logged, `"after":"backend"`)
}

func TestPropertyChangeMarshalJSONHidesSensitiveValues(t *testing.T) {
	encoded, err := json.Marshal(sensitivePasswordChange())
	require.NoError(t, err)

	require.NotContains(t, string(encoded), "secret-value")
	require.JSONEq(t,
		`{"path":"env[\"DB_PASSWORD\"]","before":"(sensitive)","after":"(sensitive)","unknown":false,"sensitive":true}`,
		string(encoded),
	)
}

func TestPropertyChangeSensitiveFieldsHoldRealValues(t *testing.T) {
	change := sensitivePasswordChange()

	require.Equal(t, "old-secret-value", change.Before)
	require.Equal(t, "new-secret-value", change.After)
	require.True(t, change.Sensitive)
}
