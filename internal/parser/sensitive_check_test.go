package parser

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/stretchr/testify/require"
)

func TestAttributePathSplitsDottedName(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, attributePath("a.b"))
}

func TestAttributePathSplitsIndexAndAttribute(t *testing.T) {
	require.Equal(t, []string{"network", "0", "name"}, attributePath("network[0].name"))
}

func TestAttributePathSplitsQuotedKey(t *testing.T) {
	require.Equal(t, []string{"tags", "env"}, attributePath(`tags["env"]`))
}

func TestAttributePathOfEmptyStringIsEmpty(t *testing.T) {
	require.Empty(t, attributePath(""))
}

func TestTypeSensitivityMarksPasswordWhole(t *testing.T) {
	judged := typeSensitivity(reflect.TypeOf(&registered.Secret{}), map[reflect.Type]bool{})
	require.NotNil(t, judged)

	require.False(t, judged.whole)
	require.True(t, judged.parts["password"].whole)
}

func TestTypeSensitivityLeavesUsernameAbsent(t *testing.T) {
	judged := typeSensitivity(reflect.TypeOf(&registered.Secret{}), map[reflect.Type]bool{})
	require.NotNil(t, judged)

	_, found := judged.parts["username"]
	require.False(t, found)
}
