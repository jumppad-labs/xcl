package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/inprocess"
	"github.com/jumppad-labs/xcl/e2e/fixtures/kube"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/registry"
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

// newKubeRegistry returns a local registry declaring the block types the kube
// configuration uses
func newKubeRegistry() *registry.Local {
	local := registry.NewLocal()
	local.RegisterType(&kube.ConfigMap{}, "config_map")
	local.RegisterType(&kube.Secret{}, "secret")
	local.RegisterType(&kube.Deployment{}, "deployment")
	local.RegisterType(&kube.Service{}, "service")
	local.RegisterType(&kube.Ingress{}, "ingress")

	return local
}

// newLocalRegistry returns a local registry holding the in-process plugin and
// the external plugin built by TestMain
func newLocalRegistry() *registry.Local {
	local := registry.NewLocal()
	local.RegisterPlugin(&inprocess.Plugin{})
	local.RegisterExternalPlugin(externalPlugin)

	return local
}

// newKubeConfig returns a Config for the kube configuration, with the kube
// block types declared. See newConfig for handler, stateDir and stateKey.
func newKubeConfig(t testing.TB, handler xcl.EventHandler, stateDir string, stateKey []byte) *xcl.Config {
	t.Helper()

	return newConfig(t, handler, stateDir, stateKey, xcl.WithRegistry(newKubeRegistry()))
}

// newPluginConfig returns a Config for the plugin configuration, with the
// in-process and external plugins in its local registry. See newConfig for
// handler, stateDir and stateKey.
func newPluginConfig(t testing.TB, handler xcl.EventHandler, stateDir string, stateKey []byte) *xcl.Config {
	t.Helper()

	return newConfig(t, handler, stateDir, stateKey, xcl.WithRegistry(newLocalRegistry()))
}

// newConfig returns a Config with options that keeps its state in stateDir,
// set up as an application would set it up. Events go to handler, a nil
// handler leaves xcl silent, and each event carries the resource as state
// records it. The sensitive values in state are encrypted with stateKey, a
// nil stateKey leaves them unencrypted.
func newConfig(t testing.TB, handler xcl.EventHandler, stateDir string, stateKey []byte, options ...xcl.ConfigOption) *xcl.Config {
	t.Helper()

	options = append(options,
		xcl.WithStatePath(stateDir),
		xcl.WithEventData(xcl.EventDataProcessed),
	)

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
