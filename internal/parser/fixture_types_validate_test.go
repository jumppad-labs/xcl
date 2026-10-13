package parser

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/stretchr/testify/require"
)

// The fixture types the parser tests decode into must pass the check a
// declared type is given when it is registered, otherwise the check would
// reject a legitimate type.

func TestEveryRegisteredFixtureTypePassesValidateSchemaType(t *testing.T) {
	harness := setupRegisteredTypes(t)

	checked := 0
	for _, info := range harness.registry.Types() {
		if info.Prototype == nil {
			continue
		}

		err := gohcl.ValidateSchemaType(reflect.TypeOf(info.Prototype))
		require.NoError(t, err, "type %q", info.Key())
		checked++
	}

	require.NotZero(t, checked)
}

func TestEverySecretFixtureTypePassesValidateSchemaType(t *testing.T) {
	for _, prototype := range []any{&registered.Secret{}, &registered.SecretConsumer{}, &registered.SecretShape{}} {
		err := gohcl.ValidateSchemaType(reflect.TypeOf(prototype))
		require.NoError(t, err, "type %T", prototype)
	}
}

func TestEveryTestPluginTypePassesValidateSchemaType(t *testing.T) {
	for _, prototype := range []any{&structs.Container{}, &structs.Sidecar{}, &structs.Network{}, &structs.Credential{}, &structs.Template{}} {
		err := gohcl.ValidateSchemaType(reflect.TypeOf(prototype))
		require.NoError(t, err, "type %T", prototype)
	}
}
