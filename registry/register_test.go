package registry

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type plainThing struct {
	Size int `xcl:"size,optional"`
}

func TestRegisterDeclaresTheGoType(t *testing.T) {
	local := NewLocal()

	Register[plainThing](local, "thing")

	declared := local.Types()
	require.Len(t, declared, 1)
	require.Equal(t, "thing", declared[0].Type)
	require.Equal(t, reflect.TypeOf(&plainThing{}), reflect.TypeOf(declared[0].Prototype))
}

func TestRegisterPanicsForAPointerType(t *testing.T) {
	local := NewLocal()

	require.Panics(t, func() {
		Register[*plainThing](local, "thing")
	})
}
