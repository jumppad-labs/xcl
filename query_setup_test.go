package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
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

	local := registry.NewLocal()

	local.RegisterPlugin(&parser.TestPlugin{})

	ss := &statemocks.MockStateStore{}
	ss.On("Exists").Return(false)
	ss.On("Load").Return(nil, nil)
	ss.On("Save", mock.Anything).Return(nil)

	c, err := NewConfig(
		WithType(&registered.Database{}, "resource", registered.TypeDatabase),
		WithRegistry(local),
		WithStateStore(ss),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/query/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}

// setupOutputEntitiesConfig applies the output entities fixture, which declares
// one output at the root and one inside a module. It needs no registered or
// plugin types, outputs and modules are kinds xcl interprets itself.
func setupOutputEntitiesConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	ss := &statemocks.MockStateStore{}
	ss.On("Exists").Return(false)
	ss.On("Load").Return(nil, nil)
	ss.On("Save", mock.Anything).Return(nil)

	c, err := NewConfig(
		WithStateStore(ss),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/output_entities/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.NoError(t, err)

	return c
}
