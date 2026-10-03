package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// AddressPath returns the segments a type is reached by: its type, followed
// by its subtype where it has one.

func TestAddressPathPutsTheSubtypeAfterTheType(t *testing.T) {
	info := TypeInfo{Type: TypeResource, Subtype: "container"}

	require.Equal(t, []string{TypeResource, "container"}, info.AddressPath())
}

func TestAddressPathPutsTheSubtypeAfterAnyType(t *testing.T) {
	info := TypeInfo{Type: "server", Subtype: "big"}

	require.Equal(t, []string{"server", "big"}, info.AddressPath())
}

func TestAddressPathIsTheTypeAloneWithoutASubtype(t *testing.T) {
	info := TypeInfo{Type: "container"}

	require.Equal(t, []string{"container"}, info.AddressPath())
}

func TestAddressPathLeadsWithTheNameOfABuiltin(t *testing.T) {
	info := TypeInfo{Type: "variable", Builtin: true}

	require.Equal(t, []string{"variable"}, info.AddressPath())
}

func TestTypeKeyJoinsTypeAndSubtype(t *testing.T) {
	require.Equal(t, "server.big", TypeKey("server", "big"))
}

func TestTypeKeyIsTheTypeAloneWithoutASubtype(t *testing.T) {
	require.Equal(t, "cache", TypeKey("cache", ""))
}
