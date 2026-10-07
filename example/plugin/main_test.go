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
	"github.com/jumppad-labs/xcl/example/plugin/docker/client"
	"github.com/jumppad-labs/xcl/example/plugin/docker/resources"
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

	build := exec.Command("go", "build", "-o", dockerPlugin, "./docker")
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

	if err := client.Ping(context.Background()); err != nil {
		t.Skip(err.Error())
	}
}

// applyExample applies the example's configuration with a nil event handler,
// keeping the state in a temporary directory and writing the rendered template
// to another. What it applied is destroyed when the test ends.
func applyExample(t *testing.T) *xcl.Config {
	t.Helper()

	return applyExampleWithState(t, t.TempDir())
}

// applyExampleWithState applies the example's configuration as applyExample
// does, keeping the state in stateDir so a test can read it back in a second
// Config, as a later run of the program would
func applyExampleWithState(t *testing.T, stateDir string) *xcl.Config {
	t.Helper()

	requireDocker(t)
	t.Setenv("HCL_VAR_output_dir", t.TempDir())

	c, err := newConfig(registry.NewPluginRegistry(), nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	// destroy what was applied even when applying fails part way
	t.Cleanup(func() {
		if c.EntityCount() > 0 {
			c.Destroy()
		}
	})

	err = apply(c, "./config")
	require.NoError(t, err)

	return c
}

// loadExample returns a Config holding the state saved in stateDir, built as
// a later run of the program builds it
func loadExample(t *testing.T, stateDir string) *xcl.Config {
	t.Helper()

	c, err := newConfig(registry.NewPluginRegistry(), nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	err = load(c)
	require.NoError(t, err)

	return c
}

// newDockerClient returns a real Docker client, used to look at what the
// example created
func newDockerClient(t *testing.T) client.Docker {
	t.Helper()

	dockerClient, err := client.New()
	require.NoError(t, err)

	return dockerClient
}

func TestApplyCreatesTheDockerNetwork(t *testing.T) {
	c := applyExample(t)

	networks, err := xcl.FindByType[resources.Network](c, "docker", "network")
	require.NoError(t, err)
	require.Len(t, networks, 1)

	app := networks[0]
	require.Equal(t, "app", app.Meta.Name)
	require.NotEmpty(t, app.DockerID)

	inspect, err := newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.NoError(t, err)
	require.Equal(t, resources.CreatedByValue, inspect.Labels[resources.LabelCreatedBy])
}

func TestApplyCreatesTheDockerContainerOnTheNetwork(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[resources.Container](c, "docker.container.web")
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

	web, err := xcl.Find[resources.Container](c, "docker.container.web")
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

	missing := filepath.Join(t.TempDir(), "docker-plugin")

	c, err := newConfig(registry.NewPluginRegistry(), nil, missing, t.TempDir())
	require.NoError(t, err)

	err = apply(c, "./config")
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

	app, err := xcl.Find[resources.Network](c, "docker.network.app")
	require.NoError(t, err)

	web, err := xcl.Find[resources.Container](c, "docker.container.web")
	require.NoError(t, err)

	err = destroy(io.Discard, c)
	require.NoError(t, err)

	dockerClient := newDockerClient(t)

	_, err = dockerClient.ContainerInspect(context.Background(), web.DockerID)
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the container to be gone, got: %s", err)

	_, err = dockerClient.NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the network to be gone, got: %s", err)
}

func TestReportPrintsTheResourcesOfBothPlugins(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[resources.Container](c, "docker.container.web")
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

func TestStatusReportsWhatApplySaved(t *testing.T) {
	stateDir := t.TempDir()
	applied := applyExampleWithState(t, stateDir)

	web, err := xcl.Find[resources.Container](applied, "docker.container.web")
	require.NoError(t, err)

	c := loadExample(t, stateDir)

	out := &bytes.Buffer{}
	err = report(out, c)
	require.NoError(t, err)

	printed := out.String()
	require.Contains(t, printed, "docker.network.app")
	require.Contains(t, printed, "docker.container.web")
	require.Contains(t, printed, "ip_address="+web.IPAddress)
	require.Contains(t, printed, "template.welcome")
}

func TestStatusWithNothingSavedReportsNothing(t *testing.T) {
	c := loadExample(t, t.TempDir())

	out := &bytes.Buffer{}
	err := report(out, c)
	require.NoError(t, err)

	require.Equal(t, "## Networks\n## Containers\n## Templates\n", out.String())
}

func TestDestroyInALaterRunRemovesWhatApplySaved(t *testing.T) {
	stateDir := t.TempDir()
	applied := applyExampleWithState(t, stateDir)

	web, err := xcl.Find[resources.Container](applied, "docker.container.web")
	require.NoError(t, err)

	c, err := newConfig(registry.NewPluginRegistry(), nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	err = destroy(io.Discard, c)
	require.NoError(t, err)

	_, err = newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the container to be gone, got: %s", err)

	remaining := loadExample(t, stateDir)
	require.Equal(t, 0, remaining.EntityCount())
}

func TestRunWithoutACommandPrintsUsage(t *testing.T) {
	stderr := &bytes.Buffer{}

	code := run(nil, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "usage: xcl-docker <command> [flags]")
}

func TestRunWithAnUnknownCommandFails(t *testing.T) {
	stderr := &bytes.Buffer{}

	code := run([]string{"plan"}, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), `unknown command "plan"`)
	require.Contains(t, stderr.String(), "usage: xcl-docker <command> [flags]")
}

func TestRunApplyWithoutAPathFails(t *testing.T) {
	stderr := &bytes.Buffer{}

	code := run([]string{"apply"}, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "apply needs the path of the configuration to apply")
}
