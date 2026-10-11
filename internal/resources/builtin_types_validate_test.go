package resources

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/stretchr/testify/require"
)

// Every builtin type is decoded from configuration, so each must pass the
// check a declared type is given when it is registered.

func TestEveryBuiltinTypePassesValidateSchemaType(t *testing.T) {
	builtins := DefaultResources()
	require.NotEmpty(t, builtins)

	for name, prototype := range builtins {
		err := gohcl.ValidateSchemaType(reflect.TypeOf(prototype))
		require.NoError(t, err, "builtin %q", name)
	}
}
