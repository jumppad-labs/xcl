package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// AddressPath returns the segments a type is reached by, and a type is reached
// by one form only: a kind led type sits under resource, a bare one and a
// builtin lead with their own name.

func TestAddressPathPutsAKindLedTypeUnderResource(t *testing.T) {
	info := TypeInfo{Name: "container"}

	require.Equal(t, []string{TypeResource, "container"}, info.AddressPath())
}

func TestAddressPathLeadsWithTheNameOfABareType(t *testing.T) {
	info := TypeInfo{Name: "container", Bare: true}

	require.Equal(t, []string{"container"}, info.AddressPath())
}

func TestAddressPathLeadsWithTheNameOfABuiltin(t *testing.T) {
	info := TypeInfo{Name: "variable", Builtin: true}

	require.Equal(t, []string{"variable"}, info.AddressPath())
}
