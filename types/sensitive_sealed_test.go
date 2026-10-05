package types_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/types"
)

func TestSensitiveValueInterfaceIsSealedByAnUnexportedMethod(t *testing.T) {
	interfaceType := reflect.TypeOf((*types.SensitiveValue)(nil)).Elem()

	require.Equal(t, 2, interfaceType.NumMethod())

	unexported := 0
	for index := 0; index < interfaceType.NumMethod(); index++ {
		if interfaceType.Method(index).PkgPath != "" {
			unexported++
		}
	}

	require.Equal(t, 1, unexported)
}
