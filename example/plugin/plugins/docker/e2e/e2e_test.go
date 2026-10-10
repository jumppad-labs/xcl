// Package e2e runs the Docker plugin end to end: it builds the plugin binary,
// serves it to an xcl host over the plugin protocol, and applies, plans and
// destroys the sample configuration in examples/basic against a real Docker
// engine. The tests skip when no engine is reachable.
//
// Both tests create Docker objects with the same names, so neither runs in
// parallel.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/docker"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/registry"
)

// basicExample is the directory holding the sample configuration the tests
// apply
const basicExample = "../examples/basic"

// dockerPlugin is the path of the Docker plugin binary TestMain builds for the
// tests in this package
var dockerPlugin string

func TestMain(m *testing.M) {
	buildDir, err := os.MkdirTemp("", "xcl-docker-plugin-e2e")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create build directory: %s\n", err)
		os.Exit(1)
	}

	dockerPlugin = filepath.Join(buildDir, "docker-plugin")

	build := exec.Command("go", "build", "-o", dockerPlugin, "./cmd/docker")
	build.Dir = ".."
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr

	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to build the Docker plugin: %s\n", err)
		os.RemoveAll(buildDir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(buildDir)
	os.Exit(code)
}

// requireDocker skips the test when no Docker engine is reachable
func requireDocker(t *testing.T) {
	t.Helper()

	if err := docker.Ping(context.Background()); err != nil {
		t.Skip(err.Error())
	}
}

// newConfig returns a Config whose local registry holds the Docker plugin
// binary TestMain built, keeping its state in stateDir
func newConfig(t *testing.T, stateDir string) *xcl.Config {
	t.Helper()

	local := registry.NewLocal()
	local.RegisterExternalPlugin(dockerPlugin)

	c, err := xcl.NewConfig(
		xcl.WithRegistry(local),
		xcl.WithStatePath(stateDir),
	)
	require.NoError(t, err)

	return c
}

// applyBasicExample applies the sample configuration with a Config keeping its
// state in stateDir. What it applied is destroyed when the test ends.
func applyBasicExample(t *testing.T, stateDir string) *xcl.Config {
	t.Helper()

	c := newConfig(t, stateDir)

	// register the destroy before applying, so a partly applied
	// configuration is removed too
	t.Cleanup(func() {
		require.NoError(t, c.Destroy())
	})

	err := c.Apply(basicExample)
	require.NoError(t, err)

	return c
}

func TestBasicExampleAppliesThenPlansNoChanges(t *testing.T) {
	requireDocker(t)

	stateDir := t.TempDir()
	applyBasicExample(t, stateDir)

	// a separate Config on the same state, as a later run of a program
	// planning the same configuration would be
	planner := newConfig(t, stateDir)

	changes, err := planner.Diff([]string{basicExample})
	require.NoError(t, err)
	require.Equal(t, 0, changes.Changed())
}

func TestBasicExampleDestroyLeavesNothingInDocker(t *testing.T) {
	requireDocker(t)

	c := applyBasicExample(t, t.TempDir())

	container, err := xcl.Find[entities.Container](c, "docker.container.xcl_plugin_basic")
	require.NoError(t, err)
	require.NotEmpty(t, container.DockerID)

	net, err := xcl.Find[entities.Network](c, "docker.network.xcl_plugin_basic")
	require.NoError(t, err)
	require.NotEmpty(t, net.DockerID)

	err = c.Destroy()
	require.NoError(t, err)

	client, err := docker.New()
	require.NoError(t, err)

	ctx := context.Background()

	_, err = client.ContainerInspect(ctx, container.DockerID)
	require.True(t, errdefs.IsNotFound(err), "container %s still exists: %v", container.DockerID, err)

	_, err = client.NetworkInspect(ctx, net.DockerID, network.InspectOptions{})
	require.True(t, errdefs.IsNotFound(err), "network %s still exists: %v", net.DockerID, err)
}
