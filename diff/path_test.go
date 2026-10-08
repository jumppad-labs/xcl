package diff_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
)

func TestPathStringPrintsSingleAttribute(t *testing.T) {
	path := diff.Path{}.Attribute("image")

	require.Equal(t, "image", path.String())
}

func TestPathStringPrintsIndexFollowedByAttribute(t *testing.T) {
	path := diff.Path{}.Attribute("ports").Index(0).Attribute("host")

	require.Equal(t, "ports[0].host", path.String())
}

func TestPathStringPrintsQuotedMapKey(t *testing.T) {
	path := diff.Path{}.Attribute("env").Key("LOG_LEVEL")

	require.Equal(t, `env["LOG_LEVEL"]`, path.String())
}

func TestPathStringEscapesQuoteInMapKey(t *testing.T) {
	path := diff.Path{}.Attribute("env").Key(`A"B`)

	require.Equal(t, `env["A\"B"]`, path.String())
}

func TestPathStringPrintsIndexOnlyFirstStepWithoutLeadingDot(t *testing.T) {
	path := diff.Path{}.Index(2).Attribute("host")

	require.Equal(t, "[2].host", path.String())
}

func TestPathStringOfEmptyPathIsEmpty(t *testing.T) {
	require.Equal(t, "", diff.Path{}.String())
}

func TestPathMarshalJSONEncodesAttributeAsString(t *testing.T) {
	encoded, err := json.Marshal(diff.Path{}.Attribute("image"))
	require.NoError(t, err)

	require.Equal(t, `"image"`, string(encoded))
}

func TestPathMarshalJSONEncodesIndexedPathAsString(t *testing.T) {
	encoded, err := json.Marshal(diff.Path{}.Attribute("ports").Index(0).Attribute("host"))
	require.NoError(t, err)

	require.Equal(t, `"ports[0].host"`, string(encoded))
}

func TestPathMarshalJSONEncodesMapKeyAsEscapedString(t *testing.T) {
	encoded, err := json.Marshal(diff.Path{}.Attribute("env").Key("LOG_LEVEL"))
	require.NoError(t, err)

	require.Equal(t, `"env[\"LOG_LEVEL\"]"`, string(encoded))
}

func TestPathAttributeDoesNotAliasReceiver(t *testing.T) {
	// a base with spare capacity is where an alias would show up
	base := make(diff.Path, 0, 4)
	base = base.Attribute("ports").Index(0)

	host := base.Attribute("host")
	local := base.Attribute("local")

	require.Equal(t, "ports[0].host", host.String())
	require.Equal(t, "ports[0].local", local.String())
	require.Equal(t, "ports[0]", base.String())
}

func TestPathIndexDoesNotAliasReceiver(t *testing.T) {
	base := make(diff.Path, 0, 4)
	base = base.Attribute("ports")

	first := base.Index(0)
	second := base.Index(1)

	require.Equal(t, "ports[0]", first.String())
	require.Equal(t, "ports[1]", second.String())
	require.Equal(t, "ports", base.String())
}

func TestPathKeyDoesNotAliasReceiver(t *testing.T) {
	base := make(diff.Path, 0, 4)
	base = base.Attribute("env")

	logLevel := base.Key("LOG_LEVEL")
	password := base.Key("DB_PASSWORD")

	require.Equal(t, `env["LOG_LEVEL"]`, logLevel.String())
	require.Equal(t, `env["DB_PASSWORD"]`, password.String())
	require.Equal(t, "env", base.String())
}

func TestPathConstructorsRecordStepKinds(t *testing.T) {
	path := diff.Path{}.Attribute("env").Index(3).Key("LOG_LEVEL")

	require.Equal(t, diff.Path{
		{Kind: diff.StepAttribute, Attribute: "env"},
		{Kind: diff.StepIndex, Index: 3},
		{Kind: diff.StepKey, Key: "LOG_LEVEL"},
	}, path)
}
