package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// variableFromEntityConfig declares a variable whose default is built from a
// field of an applied entity, and a gateway that reads the variable
const variableFromEntityConfig = `
service "api" {
  host = "api.local"
}

variable "url" {
  default = "http://${service.api.host}"
}

gateway "edge" {
  address = variable.url
}
`

func TestApplyBuildsVariableDefaultFromEntity(t *testing.T) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	local := registry.NewLocal()
	local.RegisterType(&registered.Service{}, registered.TypeService)
	local.RegisterType(&registered.Gateway{}, registered.TypeGateway)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	configFile := filepath.Join(t.TempDir(), "main.xcl")
	err = os.WriteFile(configFile, []byte(variableFromEntityConfig), 0644)
	require.NoError(t, err)

	c, err := NewConfig(WithRegistry(local), WithStateStore(store))
	require.NoError(t, err)

	err = c.Apply(configFile)
	require.NoError(t, err)

	gateway, err := Find[registered.Gateway](c, "gateway.edge")
	require.NoError(t, err)

	require.Equal(t, "http://api.local", gateway.Address)
}
