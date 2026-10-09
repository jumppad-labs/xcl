package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathStringPrintsSingleAttribute(t *testing.T) {
	path := Path{}.Attribute("image")

	require.Equal(t, "image", path.String())
}

func TestPathStringPrintsIndexFollowedByAttribute(t *testing.T) {
	path := Path{}.Attribute("ports").Index(0).Attribute("host")

	require.Equal(t, "ports[0].host", path.String())
}

func TestPathStringPrintsQuotedMapKey(t *testing.T) {
	path := Path{}.Attribute("env").Key("LOG_LEVEL")

	require.Equal(t, `env["LOG_LEVEL"]`, path.String())
}

func TestPathStringEscapesQuoteInMapKey(t *testing.T) {
	path := Path{}.Attribute("env").Key(`A"B`)

	require.Equal(t, `env["A\"B"]`, path.String())
}

func TestPathStringPrintsIndexOnlyFirstStepWithoutLeadingDot(t *testing.T) {
	path := Path{}.Index(2).Attribute("host")

	require.Equal(t, "[2].host", path.String())
}

func TestPathStringOfEmptyPathIsEmpty(t *testing.T) {
	require.Equal(t, "", Path{}.String())
}

func TestPathMarshalJSONEncodesAttributeAsString(t *testing.T) {
	encoded, err := json.Marshal(Path{}.Attribute("image"))
	require.NoError(t, err)

	require.Equal(t, `"image"`, string(encoded))
}

func TestPathMarshalJSONEncodesIndexedPathAsString(t *testing.T) {
	encoded, err := json.Marshal(Path{}.Attribute("ports").Index(0).Attribute("host"))
	require.NoError(t, err)

	require.Equal(t, `"ports[0].host"`, string(encoded))
}

func TestPathMarshalJSONEncodesMapKeyAsEscapedString(t *testing.T) {
	encoded, err := json.Marshal(Path{}.Attribute("env").Key("LOG_LEVEL"))
	require.NoError(t, err)

	require.Equal(t, `"env[\"LOG_LEVEL\"]"`, string(encoded))
}

func TestPathAttributeDoesNotAliasReceiver(t *testing.T) {
	// a base with spare capacity is where an alias would show up
	base := make(Path, 0, 4)
	base = base.Attribute("ports").Index(0)

	host := base.Attribute("host")
	local := base.Attribute("local")

	require.Equal(t, "ports[0].host", host.String())
	require.Equal(t, "ports[0].local", local.String())
	require.Equal(t, "ports[0]", base.String())
}

func TestPathIndexDoesNotAliasReceiver(t *testing.T) {
	base := make(Path, 0, 4)
	base = base.Attribute("ports")

	first := base.Index(0)
	second := base.Index(1)

	require.Equal(t, "ports[0]", first.String())
	require.Equal(t, "ports[1]", second.String())
	require.Equal(t, "ports", base.String())
}

func TestPathKeyDoesNotAliasReceiver(t *testing.T) {
	base := make(Path, 0, 4)
	base = base.Attribute("env")

	logLevel := base.Key("LOG_LEVEL")
	password := base.Key("DB_PASSWORD")

	require.Equal(t, `env["LOG_LEVEL"]`, logLevel.String())
	require.Equal(t, `env["DB_PASSWORD"]`, password.String())
	require.Equal(t, "env", base.String())
}

func TestPathConstructorsRecordStepKinds(t *testing.T) {
	path := Path{}.Attribute("env").Index(3).Key("LOG_LEVEL")

	require.Equal(t, Path{
		{Kind: StepAttribute, Attribute: "env"},
		{Kind: StepIndex, Index: 3},
		{Kind: StepKey, Key: "LOG_LEVEL"},
	}, path)
}

func TestPathEqualIsTrueForSameSteps(t *testing.T) {
	first := Path{}.Attribute("network").Index(0).Attribute("name")
	second := Path{}.Attribute("network").Index(0).Attribute("name")

	require.True(t, first.Equal(second))
}

func TestPathEqualIsTrueForTwoEmptyPaths(t *testing.T) {
	require.True(t, Path{}.Equal(Path{}))
}

func TestPathEqualIsFalseForDifferentLength(t *testing.T) {
	short := Path{}.Attribute("network")
	long := Path{}.Attribute("network").Index(0)

	require.False(t, short.Equal(long))
}

func TestPathEqualIsFalseForDifferentStep(t *testing.T) {
	first := Path{}.Attribute("network").Index(0)
	second := Path{}.Attribute("network").Index(1)

	require.False(t, first.Equal(second))
}

func TestPathEqualIsFalseForDifferentStepKind(t *testing.T) {
	first := Path{}.Attribute("env").Key("0")
	second := Path{}.Attribute("env").Index(0)

	require.False(t, first.Equal(second))
}

func TestPathWithinIsTrueForPrefix(t *testing.T) {
	path := Path{}.Attribute("network").Index(0).Attribute("name")

	require.True(t, path.Within(Path{}.Attribute("network")))
}

func TestPathWithinIsTrueForIndexedPrefix(t *testing.T) {
	path := Path{}.Attribute("network").Index(0).Attribute("name")

	require.True(t, path.Within(Path{}.Attribute("network").Index(0)))
}

func TestPathWithinIsTrueForItself(t *testing.T) {
	path := Path{}.Attribute("network").Index(0).Attribute("name")

	require.True(t, path.Within(Path{}.Attribute("network").Index(0).Attribute("name")))
}

func TestPathWithinIsTrueForEmptyPrefix(t *testing.T) {
	path := Path{}.Attribute("image")

	require.True(t, path.Within(Path{}))
}

func TestPathWithinIsFalseForSiblingAttribute(t *testing.T) {
	path := Path{}.Attribute("network").Index(0).Attribute("name")

	require.False(t, path.Within(Path{}.Attribute("image")))
}

func TestPathWithinIsFalseForSiblingIndex(t *testing.T) {
	path := Path{}.Attribute("network").Index(0).Attribute("name")

	require.False(t, path.Within(Path{}.Attribute("network").Index(1)))
}

func TestPathWithinIsFalseForLongerPath(t *testing.T) {
	path := Path{}.Attribute("network")

	require.False(t, path.Within(Path{}.Attribute("network").Index(0)))
}

func TestPathWithinIsFalseForAttributeSharingNamePrefix(t *testing.T) {
	path := Path{}.Attribute("networks")

	require.False(t, path.Within(Path{}.Attribute("network")))
}
