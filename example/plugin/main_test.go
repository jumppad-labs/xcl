package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/docker"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/providers"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/template"
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

	build := exec.Command("go", "build", "-o", dockerPlugin, "./cmd/docker")
	build.Dir = filepath.Join("plugins", "docker")
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

	if err := pingDocker(context.Background()); err != nil {
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

	t.Setenv("HCL_VAR_output_dir", t.TempDir())

	return applyExampleDir(t, "./config", stateDir)
}

// applyExampleDir applies the configuration in configDir in a new Config
// keeping its state in stateDir, as a separate run of the program would. The
// rendered template goes wherever HCL_VAR_output_dir already points, so a test
// applying two configurations to the same state sets it once. What was
// applied is destroyed when the test ends.
func applyExampleDir(t *testing.T, configDir, stateDir string) *xcl.Config {
	t.Helper()

	requireDocker(t)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	// destroy what was applied even when applying fails part way
	t.Cleanup(func() {
		if c.EntityCount() > 0 {
			c.Destroy()
		}
	})

	err = apply(c, configDir)
	require.NoError(t, err)

	return c
}

// applySubnetChange applies the example's main configuration, then the
// address range change in config-subnet to the same state, and returns the
// Config of each apply
func applySubnetChange(t *testing.T, stateDir string) (*xcl.Config, *xcl.Config) {
	t.Helper()

	first := applyExampleWithState(t, stateDir)
	changed := applyExampleDir(t, "./config-subnet", stateDir)

	return first, changed
}

// loadExample returns a Config holding the state saved in stateDir, built as
// a later run of the program builds it
func loadExample(t *testing.T, stateDir string) *xcl.Config {
	t.Helper()

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	err = load(c)
	require.NoError(t, err)

	return c
}

// newDockerClient returns a real Docker client, used to look at what the
// example created
func newDockerClient(t *testing.T) docker.Docker {
	t.Helper()

	dockerClient, err := docker.New()
	require.NoError(t, err)

	return dockerClient
}

func TestApplyCreatesTheDockerNetwork(t *testing.T) {
	c := applyExample(t)

	networks, err := xcl.FindByType[entities.Network](c, "docker", "network")
	require.NoError(t, err)
	require.Len(t, networks, 1)

	app := networks[0]
	require.Equal(t, "app", app.Meta.Name)
	require.NotEmpty(t, app.DockerID)

	inspect, err := newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.NoError(t, err)
	require.Equal(t, providers.CreatedByValue, inspect.Labels[providers.LabelCreatedBy])
}

func TestApplyCreatesTheDockerContainerOnTheNetwork(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[entities.Container](c, "docker.container.web")
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

	web, err := xcl.Find[entities.Container](c, "docker.container.web")
	require.NoError(t, err)

	welcome, err := xcl.Find[template.Template](c, "template.welcome")
	require.NoError(t, err)

	rendered, err := os.ReadFile(welcome.Destination)
	require.NoError(t, err)

	expected := "Welcome to the app network.\n" +
		"The web container answers at http://" + web.IPAddress + "/\n"
	require.Equal(t, expected, string(rendered))
}

func TestApplyRendersAnExecutableInitScript(t *testing.T) {
	c := applyExample(t)

	initScript, err := xcl.Find[template.Template](c, "template.init")
	require.NoError(t, err)

	info, err := os.Stat(initScript.Destination)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestApplyMountsTheInitScriptReadOnlyIntoTheContainer(t *testing.T) {
	c := applyExample(t)

	web, err := xcl.Find[entities.Container](c, "docker.container.web")
	require.NoError(t, err)

	initScript, err := xcl.Find[template.Template](c, "template.init")
	require.NoError(t, err)

	scriptPath, err := filepath.Abs(initScript.Destination)
	require.NoError(t, err)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)

	require.Len(t, inspect.Mounts, 1)
	require.Equal(t, mount.TypeBind, inspect.Mounts[0].Type)
	require.Equal(t, scriptPath, inspect.Mounts[0].Source)
	require.Equal(t, "/docker-entrypoint.d/90-xcl-init.sh", inspect.Mounts[0].Destination)
	require.False(t, inspect.Mounts[0].RW)
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

	c, err := newConfig(nil, missing, t.TempDir())
	require.NoError(t, err)

	err = apply(c, "./config")
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrPluginLoad)
	require.Contains(t, err.Error(), "from registry local failed to load")
	require.Contains(t, err.Error(), "build it with `make build` in example/plugin")
}

func TestDestroyRemovesTheRenderedTemplate(t *testing.T) {
	c := applyExample(t)

	welcome, err := xcl.Find[template.Template](c, "template.welcome")
	require.NoError(t, err)
	require.FileExists(t, welcome.Destination)

	err = destroy(c)
	require.NoError(t, err)

	require.NoFileExists(t, welcome.Destination)
}

func TestDestroyRemovesTheDockerResources(t *testing.T) {
	c := applyExample(t)

	app, err := xcl.Find[entities.Network](c, "docker.network.app")
	require.NoError(t, err)

	web, err := xcl.Find[entities.Container](c, "docker.container.web")
	require.NoError(t, err)

	err = destroy(c)
	require.NoError(t, err)

	dockerClient := newDockerClient(t)

	_, err = dockerClient.ContainerInspect(context.Background(), web.DockerID)
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the container to be gone, got: %s", err)

	_, err = dockerClient.NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the network to be gone, got: %s", err)
}

func TestStatusPrintsTheAppliedResourcesAsATree(t *testing.T) {
	c := applyExample(t)

	app, err := xcl.Find[entities.Network](c, "docker.network.app")
	require.NoError(t, err)

	web, err := xcl.Find[entities.Container](c, "docker.container.web")
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = status(out, c)
	require.NoError(t, err)

	// out is not a terminal, so the tree is plain text
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	require.Len(t, lines, 4)
	require.Equal(t, "● docker.network.app  10.42.0.0/24 · "+app.DockerID[:12], lines[0])
	require.True(t, strings.HasPrefix(lines[1], "└── ● template.init  "), "unexpected line: %q", lines[1])
	require.Equal(t, "    └── ● docker.container.web  nginx:1.27-alpine · "+web.IPAddress+" · "+web.DockerID[:12], lines[2])
	require.True(t, strings.HasPrefix(lines[3], "        └── ● template.welcome  "), "unexpected line: %q", lines[3])
}

func TestStatusReportsWhatApplySaved(t *testing.T) {
	stateDir := t.TempDir()
	applied := applyExampleWithState(t, stateDir)

	web, err := xcl.Find[entities.Container](applied, "docker.container.web")
	require.NoError(t, err)

	c := loadExample(t, stateDir)

	out := &bytes.Buffer{}
	err = status(out, c)
	require.NoError(t, err)

	printed := out.String()
	require.Contains(t, printed, "docker.network.app")
	require.Contains(t, printed, "docker.container.web")
	require.Contains(t, printed, web.IPAddress)
	require.Contains(t, printed, "template.welcome")
}

func TestStatusWithNothingSavedPrintsNothingApplied(t *testing.T) {
	c := loadExample(t, t.TempDir())

	out := &bytes.Buffer{}
	err := status(out, c)
	require.NoError(t, err)

	require.Equal(t, "nothing applied\n", out.String())
}

func TestInspectPrintsTheContainerAsConfiguration(t *testing.T) {
	stateDir := t.TempDir()
	applied := applyExampleWithState(t, stateDir)

	web, err := xcl.Find[entities.Container](applied, "docker.container.web")
	require.NoError(t, err)

	c := loadExample(t, stateDir)

	out := &bytes.Buffer{}
	err = inspect(out, c, "docker.container.web")
	require.NoError(t, err)

	// out is not a terminal, so the text is plain
	printed := out.String()
	require.True(t, strings.HasPrefix(printed, `docker "container" "web" {`), "unexpected text:\n%s", printed)
	require.Contains(t, printed, `image       = "nginx:1.27-alpine"`)
	require.Contains(t, printed, "init_script = template.init.destination")
	require.Contains(t, printed, "name    = docker.network.app.meta.name")
	require.Contains(t, printed, `ip_address = "`+web.IPAddress+`" # set by the provider`)
	require.Contains(t, printed, `docker_id  = "`+web.DockerID+`" # set by the provider`)
}

func TestInspectFailsForAVariable(t *testing.T) {
	stateDir := t.TempDir()
	applyExampleWithState(t, stateDir)

	c := loadExample(t, stateDir)

	err := inspect(io.Discard, c, "variable.output_dir")
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrNotEncodable)
}

func TestInspectFailsForAResourceNotInTheState(t *testing.T) {
	c := loadExample(t, t.TempDir())

	err := inspect(io.Discard, c, "docker.container.web")
	require.Error(t, err)
	require.Equal(t, "no resource docker.container.web in the saved state, xcl-docker status lists them", err.Error())
}

func TestDestroyInALaterRunRemovesWhatApplySaved(t *testing.T) {
	stateDir := t.TempDir()
	applied := applyExampleWithState(t, stateDir)

	web, err := xcl.Find[entities.Container](applied, "docker.container.web")
	require.NoError(t, err)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	err = destroy(c)
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

	code := run([]string{"frobnicate"}, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), `unknown command "frobnicate"`)
	require.Contains(t, stderr.String(), "usage: xcl-docker <command> [flags]")
}

func TestRunInspectWithoutAnAddressFails(t *testing.T) {
	stderr := &bytes.Buffer{}

	code := run([]string{"inspect"}, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "inspect needs the address of the resource to inspect")
}

func TestRunApplyWithoutAPathFails(t *testing.T) {
	stderr := &bytes.Buffer{}

	code := run([]string{"apply"}, io.Discard, stderr)

	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "apply needs the path of the configuration to apply")
}

func TestSubnetChangeReplacesTheNetworkWithTheNewRange(t *testing.T) {
	_, changed := applySubnetChange(t, t.TempDir())

	app, err := xcl.Find[entities.Network](changed, "docker.network.app")
	require.NoError(t, err)
	require.Equal(t, "10.42.0.0/23", app.Subnet)

	inspect, err := newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, inspect.IPAM.Config)
	require.Equal(t, "10.42.0.0/23", inspect.IPAM.Config[0].Subnet)
}

func TestSubnetChangeRemovesTheOldNetwork(t *testing.T) {
	first, _ := applySubnetChange(t, t.TempDir())

	old, err := xcl.Find[entities.Network](first, "docker.network.app")
	require.NoError(t, err)

	_, err = newDockerClient(t).NetworkInspect(context.Background(), old.DockerID, network.InspectOptions{})
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the old network to be gone, got: %s", err)
}

func TestSubnetChangeKeepsTheContainerAttached(t *testing.T) {
	first, changed := applySubnetChange(t, t.TempDir())

	old, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	// the container is updated in place, not replaced
	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.DockerID)
	require.Equal(t, old.DockerID, web.DockerID)

	app, err := xcl.Find[entities.Network](changed, "docker.network.app")
	require.NoError(t, err)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
	require.NotNil(t, inspect.NetworkSettings)
	require.Contains(t, inspect.NetworkSettings.Networks, "app")
	require.Equal(t, app.DockerID, inspect.NetworkSettings.Networks["app"].NetworkID)
	require.Equal(t, web.IPAddress, inspect.NetworkSettings.Networks["app"].IPAddress)
}

func TestSubnetChangeRendersTemplateWithNewAddress(t *testing.T) {
	_, changed := applySubnetChange(t, t.TempDir())

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.IPAddress)

	welcome, err := xcl.Find[template.Template](changed, "template.welcome")
	require.NoError(t, err)

	rendered, err := os.ReadFile(welcome.Destination)
	require.NoError(t, err)

	expected := "Welcome to the app network.\n" +
		"The web container answers at http://" + web.IPAddress + "/\n"
	require.Equal(t, expected, string(rendered))
}

func TestPlanAfterSubnetChangeReportsNoChanges(t *testing.T) {
	stateDir := t.TempDir()
	applySubnetChange(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, c, "./config-subnet")
	require.NoError(t, err)

	require.Equal(t, "Diff: no changes, 4 unchanged.\n", out.String())
}
