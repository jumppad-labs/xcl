package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/registry"
	statemocks "github.com/jumppad-labs/xcl/state/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupQueryConfig builds a Config whose registry has the registered type
// database and the plugin types of the TestPlugin (container, network,
// template, sidecar), then applies the query fixture, which holds three
// database blocks and two network blocks.
func setupQueryConfig(t *testing.T) *Config {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	log := logger.NewTestLogger(t)

	pr := registry.NewPluginRegistry(log)

	err := pr.RegisterType(registered.TypeDatabase, &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterPlugin(&parser.TestPlugin{})
	require.NoError(t, err)

	ss := &statemocks.MockStateStore{}
	ss.On("Exists").Return(false)
	ss.On("Load").Return(nil, nil)
	ss.On("Save", mock.Anything).Return(nil)

	c := NewConfig(
		WithPluginRegistry(pr),
		WithStateStore(ss),
	)

	path, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}
