package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/docker"
	"github.com/jumppad-labs/xcl/example/plugin/template"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// dockerPlugin is the path of the Docker plugin binary TestMain builds for the
// tests in this package
var dockerPlugin string

func TestMain(m *testing.M) {
	buildDir, err := os.MkdirTemp("", "xcl-example-plugin-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create build directory: %s\n", err)
		os.Exit(1)
	}

	dockerPlugin = filepath.Join(buildDir, "docker-plugin")

	build := exec.Command("go", "build", "-o", dockerPlugin, "./cmd/docker-plugin")
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

// newRegistry returns a plugin registry whose plugin hosts are stopped when
// the test ends
func newRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	r := registry.NewPluginRegistry()
	t.Cleanup(func() {
		for _, host := range r.GetPluginHosts() {
			host.Stop()
		}
	})

	return r
}

// applyExample applies the example's configuration with a nil event handler,
// writing the rendered template to a temporary directory. It does not destroy
// what it applied, the caller does that or calls applyExampleWithCleanup.
func applyExample(t *testing.T) *xcl.Config {
	t.Helper()

	requireDocker(t)
	t.Setenv("HCL_VAR_output_dir", t.TempDir())

	r := newRegistry(t)

	c, err := apply(r, nil, "./config", dockerPlugin, t.TempDir())
	if c != nil {
		// destroy what was applied even when applying failed part way
		t.Cleanup(func() {
			if c.EntityCount() > 0 {
				c.Destroy()
			}
		})
	}
	require.NoError(t, err)
	require.NotNil(t, c)

	return c
}

// newDockerClient returns a real Docker client, used to look at what the
// example created
func newDockerClient(t *testing.T) docker.Client {
	t.Helper()

	client, err := docker.NewClient()
	require.NoError(t, err)

	return client
}

func TestApplyCreatesTheDockerNetwork(t *testing.T) {
	c := applyExample(t)

	networks, err := xcl.FindByType[docker.Network](c, "docker", "network")
	require.NoError(t, err)
	require.Len(t, networks, 1)

	app := networks[0]
	require.Equal(t, "app", app.Meta.Name)
	require.NotEmpty(t, app.DockerID)

	inspect, err := newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.NoError(t, err)
	require.Equal(t, docker.CreatedByValue, inspect.Labels[docker.LabelCreatedBy])
}

func TestApplyCreatesTheDockerContainerOnTheNetwork(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[docker.Container](c, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.DockerID)
	require.NotEmpty(t, web.IPAddress)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
	require.NotNil(t, inspect.NetworkSettings)
	require.Contains(t, inspect.NetworkSettings.Networks, "app")
}

func TestApplyRendersTheTemplateWithTheContainerAddress(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[docker.Container](c, "docker.container.web")
	require.NoError(t, err)

	welcome, err := xcl.Find[template.Template](c, "template.welcome")
	require.NoError(t, err)

	rendered, err := os.ReadFile(welcome.Destination)
	require.NoError(t, err)

	expected := "Welcome to the app network.\n" +
		"The web container answers at http://" + web.IPAddress + "/\n"
	require.Equal(t, expected, string(rendered))
}

func TestApplyFindsTheResourcesOfBothPlugins(t *testing.T) {
	c := applyExample(t)

	_, err := c.FindResource("docker.network.app")
	require.NoError(t, err)

	_, err = c.FindResource("docker.container.web")
	require.NoError(t, err)

	_, err = c.FindResource("template.welcome")
	require.NoError(t, err)
}

func TestApplyFailsForAMissingDockerPlugin(t *testing.T) {
	t.Setenv("HCL_VAR_output_dir", t.TempDir())

	r := newRegistry(t)
	missing := filepath.Join(t.TempDir(), "docker-plugin")

	_, err := apply(r, nil, "./config", missing, t.TempDir())
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrPluginLoad)
	require.Contains(t, err.Error(), "build it with `make build` in example/plugin")
}

func TestDestroyRemovesTheRenderedTemplate(t *testing.T) {
	c := applyExample(t)

	welcome, err := xcl.Find[template.Template](c, "template.welcome")
	require.NoError(t, err)
	require.FileExists(t, welcome.Destination)

	err = destroy(io.Discard, c)
	require.NoError(t, err)

	require.NoFileExists(t, welcome.Destination)
}

func TestDestroyRemovesTheDockerResources(t *testing.T) {
	c := applyExample(t)

	app, err := xcl.Find[docker.Network](c, "docker.network.app")
	require.NoError(t, err)

	web, err := xcl.Find[docker.Container](c, "docker.container.web")
	require.NoError(t, err)

	err = destroy(io.Discard, c)
	require.NoError(t, err)

	client := newDockerClient(t)

	_, err = client.ContainerInspect(context.Background(), web.DockerID)
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the container to be gone, got: %s", err)

	_, err = client.NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the network to be gone, got: %s", err)
}

func TestReportPrintsTheResourcesOfBothPlugins(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[docker.Container](c, "docker.container.web")
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = report(out, c)
	require.NoError(t, err)

	printed := out.String()
	require.Contains(t, printed, "## Networks")
	require.Contains(t, printed, "docker.network.app")
	require.Contains(t, printed, "## Containers")
	require.Contains(t, printed, "docker.container.web")
	require.Contains(t, printed, "## Templates")
	require.Contains(t, printed, "template.welcome")
	require.Contains(t, printed, "Welcome to the app network.")
	require.Contains(t, printed, "http://"+web.IPAddress+"/")
}
