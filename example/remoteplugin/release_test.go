package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// dockerPluginSource is the Docker plugin's entry point in this repository,
// resolved before any test changes directory
var dockerPluginSource, _ = filepath.Abs("../plugin/plugins/docker/cmd/docker")

var (
	buildDockerPlugin sync.Once
	dockerPlugin      string
	dockerPluginErr   error
)

// builtDockerPlugin builds the Docker plugin from this repository once, so it
// speaks the plugin protocol of the xcl it is tested with, and returns the
// path of the binary
func builtDockerPlugin(t *testing.T) string {
	t.Helper()

	buildDockerPlugin.Do(func() {
		dir, err := os.MkdirTemp("", "xcl-remoteplugin-")
		if err != nil {
			dockerPluginErr = err
			return
		}

		dockerPlugin = filepath.Join(dir, "xcl-plugin-docker")

		build := exec.Command("go", "build", "-o", dockerPlugin, ".")
		build.Dir = dockerPluginSource

		if output, err := build.CombinedOutput(); err != nil {
			dockerPluginErr = &buildError{err: err, output: string(output)}
		}
	})

	require.NoError(t, dockerPluginErr)

	return dockerPlugin
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string {
	return "building the Docker plugin: " + e.err.Error() + "\n" + e.output
}

// useFakeRelease serves the Docker plugin built from this repository as the
// signed GitHub release the example installs, with the key it is signed with,
// and points the example at both for the rest of the test
func useFakeRelease(t *testing.T) {
	t.Helper()

	release := testutil.NewFakeGitHubRelease(t, "jumppad-labs/xcl-plugin-docker", "v0.2.0", builtDockerPlugin(t))

	key := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, release.PublicKey())
	}))
	t.Cleanup(key.Close)

	previous := source
	source = pluginSource{apiURL: release.URL(), keyURL: key.URL}
	t.Cleanup(func() { source = previous })
}
