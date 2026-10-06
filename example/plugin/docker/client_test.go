package docker

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPingFailsWhenNoEngineIsReachable(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix://"+filepath.Join(t.TempDir(), "none.sock"))

	err := Ping(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "no Docker engine reachable")
}

func TestNewClientReturnsAClientFromTheEnvironment(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix://"+filepath.Join(t.TempDir(), "none.sock"))

	c, err := NewClient()

	require.NoError(t, err)
	require.NotNil(t, c)
}
