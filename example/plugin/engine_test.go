package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// shortTempDir returns a temporary directory with a short path, a unix socket
// path is limited to around 108 bytes and t.TempDir can exceed that
func shortTempDir(t *testing.T) string {
	t.Helper()

	directory, err := os.MkdirTemp("", "ping")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(directory)
	})

	return directory
}

// pingHandler answers /_ping with the given status
func pingHandler(status int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
		writer.Write([]byte("OK"))
	})

	return mux
}

func TestPingDockerSucceedsWhenAnEngineAnswers(t *testing.T) {
	socket := filepath.Join(shortTempDir(t), "docker.sock")

	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(pingHandler(http.StatusOK))
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "unix://"+socket)

	err = pingDocker(context.Background())
	require.NoError(t, err)
}

func TestPingDockerSucceedsOverTCP(t *testing.T) {
	server := httptest.NewServer(pingHandler(http.StatusOK))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))

	err := pingDocker(context.Background())
	require.NoError(t, err)
}

func TestPingDockerFailsWhenNoEngineIsReachable(t *testing.T) {
	socket := filepath.Join(shortTempDir(t), "missing.sock")

	t.Setenv("DOCKER_HOST", "unix://"+socket)

	err := pingDocker(context.Background())
	require.ErrorContains(t, err, "no Docker engine reachable")
}

func TestPingDockerFailsWhenTheEngineAnswersAnError(t *testing.T) {
	server := httptest.NewServer(pingHandler(http.StatusInternalServerError))
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))

	err := pingDocker(context.Background())
	require.ErrorContains(t, err, "no Docker engine reachable")
}

func TestPingDockerSkipsAnAddressItCannotDial(t *testing.T) {
	t.Setenv("DOCKER_HOST", "ssh://user@host")

	err := pingDocker(context.Background())
	require.NoError(t, err)
}
