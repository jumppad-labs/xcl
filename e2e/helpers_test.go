package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/inprocess"
	"github.com/jumppad-labs/xcl/e2e/fixtures/kube"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// kubeConfigDir is the Kubernetes-like configuration, decoded into the types
// in fixtures/kube
const kubeConfigDir = "testdata/kube"

// pluginConfigDir is the configuration whose block types are provided by the
// in-process and external plugins
const pluginConfigDir = "testdata/plugin"

// testPassword is the database password the kube configuration's secret
// reads from the DB_PASSWORD environment variable
const testPassword = "e2e-s3cret-password"

// testStateKey is the 32 byte key the tests encrypt the sensitive values in
// state with, fixed so every run encrypts the same way
var testStateKey = []byte("0123456789abcdef0123456789abcdef")

// registerKubeTypes registers the block types the kube configuration uses
func registerKubeTypes(t testing.TB, r *registry.PluginRegistry) {
	t.Helper()

	require.NoError(t, r.RegisterType(&kube.ConfigMap{}, "config_map"))
	require.NoError(t, r.RegisterType(&kube.Secret{}, "secret"))
	require.NoError(t, r.RegisterType(&kube.Deployment{}, "deployment"))
	require.NoError(t, r.RegisterType(&kube.Service{}, "service"))
	require.NoError(t, r.RegisterType(&kube.Ingress{}, "ingress"))
}

// registerPlugins registers the in-process plugin and the external plugin
// built by TestMain, and stops the external plugin's process when the test
// ends
func registerPlugins(t testing.TB, r *registry.PluginRegistry) {
	t.Helper()

	t.Cleanup(func() {
		for _, host := range r.GetPluginHosts() {
			host.Stop()
		}
	})

	require.NoError(t, r.RegisterPlugin(&inprocess.Plugin{}))
	require.NoError(t, r.RegisterPluginWithPath(externalPlugin))
}

// newKubeConfig returns a Config for the kube configuration, with the kube
// block types registered on r. See newConfig for handler, stateDir and
// stateKey.
func newKubeConfig(t testing.TB, r *registry.PluginRegistry, handler xcl.EventHandler, stateDir string, stateKey []byte) *xcl.Config {
	t.Helper()

	registerKubeTypes(t, r)

	return newConfig(t, r, handler, stateDir, stateKey)
}

// newPluginConfig returns a Config for the plugin configuration, with the
// in-process and external plugins registered on r. See newConfig for
// handler, stateDir and stateKey.
func newPluginConfig(t testing.TB, r *registry.PluginRegistry, handler xcl.EventHandler, stateDir string, stateKey []byte) *xcl.Config {
	t.Helper()

	registerPlugins(t, r)

	return newConfig(t, r, handler, stateDir, stateKey)
}

// newConfig returns a Config using r that keeps its state in stateDir, set up
// as an application would set it up. Events go to handler, a nil handler
// leaves xcl silent, and each event carries the resource as state records it.
// The sensitive values in state are encrypted with stateKey, a nil stateKey
// leaves them unencrypted.
func newConfig(t testing.TB, r *registry.PluginRegistry, handler xcl.EventHandler, stateDir string, stateKey []byte) *xcl.Config {
	t.Helper()

	options := []xcl.ConfigOption{
		xcl.WithPluginRegistry(r),
		xcl.WithStatePath(stateDir),
		xcl.WithEventData(xcl.EventDataProcessed),
	}

	if handler != nil {
		options = append(options, xcl.WithEventHandler(handler))
	}

	if stateKey != nil {
		masker, err := mask.EncryptAES256GCM(stateKey)
		require.NoError(t, err)

		options = append(options, xcl.WithStateMask(masker))
	}

	c, err := xcl.NewConfig(options...)
	require.NoError(t, err)

	return c
}
