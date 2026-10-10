package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests run the example as a reader would: they fetch the release key
// from xcl.dev, install jumppad-labs/xcl-plugin-docker from its GitHub release
// and create a real Docker network and container. They need the network and a
// Docker engine, and skip when no engine answers or when run with -short.

// requireDocker skips the test when it runs with -short or no Docker engine,
// reached through DOCKER_HOST or the default socket, answers a ping
func requireDocker(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("installs a plugin from GitHub and needs a Docker engine")
	}

	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	address, err := url.Parse(host)
	require.NoError(t, err)

	transport := &http.Transport{}
	defer transport.CloseIdleConnections()

	pingURL := "http://" + address.Host + "/_ping"
	if address.Scheme == "unix" {
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", address.Path)
		}
		pingURL = "http://docker/_ping"
	}

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	response, err := client.Get(pingURL)
	if err != nil {
		t.Skipf("no Docker engine reachable at %s: %s", host, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Skipf("no Docker engine reachable at %s: answered %s", host, response.Status)
	}
}

// inTempDir runs the rest of the test in a new temporary directory, where the
// example keeps its plugin cache, and returns the absolute path of the
// example's configuration
func inTempDir(t *testing.T) string {
	t.Helper()

	config, err := filepath.Abs("./config")
	require.NoError(t, err)

	t.Chdir(t.TempDir())

	return config
}

// applyExample applies the example's configuration with the state in
// stateDir, destroys what it applied when the test ends, and returns the
// absolute path of the configuration
func applyExample(t *testing.T, stateDir string) string {
	t.Helper()

	config := inTempDir(t)

	t.Cleanup(func() {
		runCommand("destroy", "--state", stateDir)
	})

	code, _, stderr := runCommand("apply", "--state", stateDir, config)
	require.Equal(t, 0, code, stderr)

	return config
}

func TestApplyCreatesTheNetworkAndContainer(t *testing.T) {
	requireDocker(t)

	stateDir := t.TempDir()
	applyExample(t, stateDir)

	c, err := newConfig(nil, stateDir)
	require.NoError(t, err)
	require.NoError(t, c.Load())

	_, err = c.FindResource("docker.network.xcl_plugin_basic")
	require.NoError(t, err)

	_, err = c.FindResource("docker.container.xcl_plugin_basic")
	require.NoError(t, err)
}

func TestApplyCachesThePluginRelease(t *testing.T) {
	requireDocker(t)

	applyExample(t, t.TempDir())

	entries, err := os.ReadDir(".xcl-cache")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
}

func TestApplyAgainKeepsTheSameEntities(t *testing.T) {
	requireDocker(t)

	stateDir := t.TempDir()
	config := applyExample(t, stateDir)

	code, _, stderr := runCommand("apply", "--state", stateDir, config)
	require.Equal(t, 0, code, stderr)

	c, err := newConfig(nil, stateDir)
	require.NoError(t, err)
	require.NoError(t, c.Load())
	require.Equal(t, 2, c.EntityCount())
}

func TestDestroyRemovesEverythingApplied(t *testing.T) {
	requireDocker(t)

	stateDir := t.TempDir()
	applyExample(t, stateDir)

	code, _, stderr := runCommand("destroy", "--state", stateDir)
	require.Equal(t, 0, code, stderr)

	c, err := newConfig(nil, stateDir)
	require.NoError(t, err)
	require.NoError(t, c.Load())
	require.Equal(t, 0, c.EntityCount())
}
