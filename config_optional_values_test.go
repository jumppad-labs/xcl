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

// optionalMapValueConfig writes a map of endpoint objects whose optional tls
// field is left out of the http entry
const optionalMapValueConfig = `
proxy "edge" {
  listeners = {
    http = { port = 80 }
  }
}
`

func TestApplyAllowsOmittedOptionalFieldInMapValue(t *testing.T) {
	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	local := registry.NewLocal()
	local.RegisterType(&registered.Proxy{}, registered.TypeProxy)

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	configFile := filepath.Join(t.TempDir(), "main.xcl")
	err = os.WriteFile(configFile, []byte(optionalMapValueConfig), 0644)
	require.NoError(t, err)

	c, err := NewConfig(WithRegistry(local), WithStateStore(store))
	require.NoError(t, err)

	err = c.Apply(configFile)
	require.NoError(t, err)

	proxy, err := Find[registered.Proxy](c, "proxy.edge")
	require.NoError(t, err)

	require.Len(t, proxy.Listeners, 1)
	require.Equal(t, 80, proxy.Listeners["http"].Port)
	require.False(t, proxy.Listeners["http"].TLS)
}
