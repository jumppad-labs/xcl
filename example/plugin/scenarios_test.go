package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/template"
)

// The scenarios in this file each apply the example's main configuration,
// then one of the committed variants in a second run sharing the same state,
// as a user running `xcl-docker apply` twice would. Each looks at what the
// plan of the variant reports and at what real Docker holds afterwards.

// planAfterMainConfig applies the example's main configuration keeping its
// state in a temporary directory, then returns the plan of configDir against
// that state, as `xcl-docker plan` prints it
func planAfterMainConfig(t *testing.T, configDir string) string {
	t.Helper()

	stateDir := t.TempDir()
	applyExampleWithState(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, c, configDir)
	require.NoError(t, err)

	return out.String()
}

// planAgain returns the plan of configDir against the state saved in
// stateDir, in a new Config as a later run of the program would build it
func planAgain(t *testing.T, stateDir, configDir string) string {
	t.Helper()

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	out := &bytes.Buffer{}
	err = plan(out, c, configDir)
	require.NoError(t, err)

	return out.String()
}

// applyNetworkSwap applies the example's main configuration, then
// config-swap, which adds the backend network and moves the web container to
// it, to the same state, and returns the Config of each apply
func applyNetworkSwap(t *testing.T, stateDir string) (*xcl.Config, *xcl.Config) {
	t.Helper()

	first := applyExampleWithState(t, stateDir)
	changed := applyExampleDir(t, "./config-swap", stateDir)

	return first, changed
}

// applyInitScriptMove applies the example's main configuration, then
// config-init, which renders the init script to a new destination, to the
// same state, and returns the Config of each apply
func applyInitScriptMove(t *testing.T, stateDir string) (*xcl.Config, *xcl.Config) {
	t.Helper()

	first := applyExampleWithState(t, stateDir)
	changed := applyExampleDir(t, "./config-init", stateDir)

	return first, changed
}

// applyInitScriptEdit applies the example's main configuration, then
// config-init-content, which edits only the init script's source, to the
// same state, and returns the Config of each apply
func applyInitScriptEdit(t *testing.T, stateDir string) (*xcl.Config, *xcl.Config) {
	t.Helper()

	first := applyExampleWithState(t, stateDir)
	changed := applyExampleDir(t, "./config-init-content", stateDir)

	return first, changed
}

// applyNetworkRemoval applies the example's main configuration, then
// config-remove, which removes the app network and every reference to it, to
// the same state, and returns the Config of each apply
func applyNetworkRemoval(t *testing.T, stateDir string) (*xcl.Config, *xcl.Config) {
	t.Helper()

	first := applyExampleWithState(t, stateDir)
	changed := applyExampleDir(t, "./config-remove", stateDir)

	return first, changed
}

// applyDanglingReference applies the example's main configuration, then
// tries config-dangling, which removes the app network while the container
// and templates still refer to it, to the same state. It returns the Config
// of the first apply and the error of the second.
func applyDanglingReference(t *testing.T, stateDir string) (*xcl.Config, error) {
	t.Helper()

	first := applyExampleWithState(t, stateDir)

	c, err := newConfig(nil, dockerPlugin, stateDir)
	require.NoError(t, err)

	// destroy anything the failed apply holds, should it hold anything
	t.Cleanup(func() {
		if c.EntityCount() > 0 {
			c.Destroy()
		}
	})

	return first, apply(c, "./config-dangling")
}

// Network swap: ./config then ./config-swap

func TestPlanOfNetworkSwapUpdatesTheContainerInPlace(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-swap")

	// out is not a terminal, so the diff is plain
	require.Contains(t, printed, "# docker.container.web will be updated\n")
	require.Contains(t, printed, `~ docker "container" "web" {`)
	require.Contains(t, printed, `~ network[0].name = "app" -> "backend"`)
	require.NotContains(t, printed, "docker.container.web will be replaced")
	require.NotContains(t, printed, `-/+ docker "container" "web"`)
}

func TestPlanOfNetworkSwapCreatesTheBackendNetwork(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-swap")

	require.Contains(t, printed, "# docker.network.backend will be created")
	require.Contains(t, printed, `+ docker "network" "backend" {`)
	require.Contains(t, printed, `+ subnet = "10.43.0.0/24"`)
}

func TestPlanOfNetworkSwapCountsOneCreateTwoUpdatesAndNoReplace(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-swap")

	// backend is created; the container and the welcome template, which
	// reads the container's address, are updated
	require.Contains(t, printed, "Diff: 1 to create, 2 to update, 0 to replace, 0 to delete, 2 unchanged.")
}

func TestNetworkSwapKeepsTheSameRunningContainer(t *testing.T) {
	first, changed := applyNetworkSwap(t, t.TempDir())

	old, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.DockerID)
	require.Equal(t, old.DockerID, web.DockerID)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
}

func TestNetworkSwapAttachesTheContainerToBackend(t *testing.T) {
	_, changed := applyNetworkSwap(t, t.TempDir())

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)

	backend, err := xcl.Find[entities.Network](changed, "docker.network.backend")
	require.NoError(t, err)
	require.NotEmpty(t, backend.DockerID)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.NetworkSettings)
	require.Contains(t, inspect.NetworkSettings.Networks, "backend")
	require.Equal(t, backend.DockerID, inspect.NetworkSettings.Networks["backend"].NetworkID)
}

func TestNetworkSwapDetachesTheContainerFromApp(t *testing.T) {
	_, changed := applyNetworkSwap(t, t.TempDir())

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.NetworkSettings)
	require.NotContains(t, inspect.NetworkSettings.Networks, "app")
}

func TestNetworkSwapSavesTheContainerAddressOnBackend(t *testing.T) {
	_, changed := applyNetworkSwap(t, t.TempDir())

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.IPAddress)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.NetworkSettings)
	require.Contains(t, inspect.NetworkSettings.Networks, "backend")
	require.Equal(t, inspect.NetworkSettings.Networks["backend"].IPAddress, web.IPAddress)
}

func TestNetworkSwapKeepsTheAppNetwork(t *testing.T) {
	first, changed := applyNetworkSwap(t, t.TempDir())

	old, err := xcl.Find[entities.Network](first, "docker.network.app")
	require.NoError(t, err)

	app, err := xcl.Find[entities.Network](changed, "docker.network.app")
	require.NoError(t, err)
	require.Equal(t, old.DockerID, app.DockerID)

	_, err = newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.NoError(t, err)
}

func TestPlanAfterNetworkSwapReportsNoChanges(t *testing.T) {
	stateDir := t.TempDir()
	applyNetworkSwap(t, stateDir)

	printed := planAgain(t, stateDir, "./config-swap")

	require.Equal(t, "Diff: no changes, 5 unchanged.\n", printed)
}

// Init-script rebuild: ./config then ./config-init

func TestPlanOfInitScriptMoveReplacesTheContainerBecauseTheTemplateIsReplaced(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-init")

	// out is not a terminal, so the diff is plain
	require.Contains(t, printed, "# docker.container.web will be replaced because template.init is replaced")
	require.Contains(t, printed, `-/+ docker "container" "web" {`)
	require.Contains(t, printed, "# template.init will be replaced, it cannot be updated in place")
	require.Contains(t, printed, `-/+ template "init" {`)
}

func TestPlanOfInitScriptMoveCountsTwoReplacesAndOneUpdate(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-init")

	// the init template and the container are replaced; the welcome
	// template, which reads the new container's address, is updated
	require.Contains(t, printed, "Diff: 0 to create, 1 to update, 2 to replace, 0 to delete, 1 unchanged.")
}

func TestInitScriptMoveCreatesANewRunningContainer(t *testing.T) {
	first, changed := applyInitScriptMove(t, t.TempDir())

	old, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.DockerID)
	require.NotEqual(t, old.DockerID, web.DockerID)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
}

func TestInitScriptMoveRemovesTheOldContainer(t *testing.T) {
	first, _ := applyInitScriptMove(t, t.TempDir())

	old, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	_, err = newDockerClient(t).ContainerInspect(context.Background(), old.DockerID)
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the old container to be gone, got: %s", err)
}

func TestInitScriptMoveMountsTheMovedScriptIntoTheNewContainer(t *testing.T) {
	_, changed := applyInitScriptMove(t, t.TempDir())

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)

	initScript, err := xcl.Find[template.Template](changed, "template.init")
	require.NoError(t, err)
	require.Equal(t, "init-v2.sh", filepath.Base(initScript.Destination))

	scriptPath, err := filepath.Abs(initScript.Destination)
	require.NoError(t, err)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.Len(t, inspect.Mounts, 1)
	require.Equal(t, scriptPath, inspect.Mounts[0].Source)
}

func TestPlanAfterInitScriptMoveReportsNoChanges(t *testing.T) {
	stateDir := t.TempDir()
	applyInitScriptMove(t, stateDir)

	printed := planAgain(t, stateDir, "./config-init")

	require.Equal(t, "Diff: no changes, 4 unchanged.\n", printed)
}

// Init-script content edit: ./config then ./config-init-content

func TestPlanOfInitScriptEditUpdatesOnlyTheTemplate(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-init-content")

	// out is not a terminal, so the diff is plain
	require.Contains(t, printed, "# template.init will be updated")
	require.Contains(t, printed, `~ template "init" {`)
	require.NotContains(t, printed, "docker.container.web")
	require.NotContains(t, printed, `docker "container" "web"`)
}

func TestPlanOfInitScriptEditCountsOneUpdateAndNoReplace(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-init-content")

	require.Contains(t, printed, "Diff: 0 to create, 1 to update, 0 to replace, 0 to delete, 3 unchanged.")
}

func TestInitScriptEditKeepsTheSameRunningContainer(t *testing.T) {
	first, changed := applyInitScriptEdit(t, t.TempDir())

	old, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.DockerID)
	require.Equal(t, old.DockerID, web.DockerID)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
}

func TestInitScriptEditRendersTheNewContent(t *testing.T) {
	_, changed := applyInitScriptEdit(t, t.TempDir())

	initScript, err := xcl.Find[template.Template](changed, "template.init")
	require.NoError(t, err)

	rendered, err := os.ReadFile(initScript.Destination)
	require.NoError(t, err)

	expected := "#!/bin/sh\n" +
		"echo \"web is on the app network\" > /usr/share/nginx/html/init.txt\n"
	require.Equal(t, expected, string(rendered))
}

func TestPlanAfterInitScriptEditReportsNoChanges(t *testing.T) {
	stateDir := t.TempDir()
	applyInitScriptEdit(t, stateDir)

	printed := planAgain(t, stateDir, "./config-init-content")

	require.Equal(t, "Diff: no changes, 4 unchanged.\n", printed)
}

// Network removal: ./config then ./config-remove

func TestPlanOfNetworkRemovalDeletesTheNetworkAndUpdatesTheContainer(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-remove")

	// out is not a terminal, so the diff is plain
	require.Contains(t, printed, "# docker.network.app will be deleted")
	require.Contains(t, printed, `- docker "network" "app" {}`)
	require.Contains(t, printed, "# docker.container.web will be updated\n")
	require.Contains(t, printed, `~ docker "container" "web" {`)
	require.Contains(t, printed, `- network[0] = { aliases = ["web"], name = "app" }`)
	require.NotContains(t, printed, "docker.container.web will be replaced")
	require.NotContains(t, printed, `-/+ docker "container" "web"`)
}

func TestPlanOfNetworkRemovalCountsThreeUpdatesOneDeleteAndNoReplace(t *testing.T) {
	printed := planAfterMainConfig(t, "./config-remove")

	// the container and both templates, which no longer name the network,
	// are updated; the network is deleted
	require.Contains(t, printed, "Diff: 0 to create, 3 to update, 0 to replace, 1 to delete, 0 unchanged.")
}

func TestNetworkRemovalDeletesTheDockerNetwork(t *testing.T) {
	first, _ := applyNetworkRemoval(t, t.TempDir())

	app, err := xcl.Find[entities.Network](first, "docker.network.app")
	require.NoError(t, err)

	_, err = newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.Error(t, err)
	require.True(t, dockerclient.IsErrNotFound(err), "expected the network to be gone, got: %s", err)
}

func TestNetworkRemovalKeepsTheSameRunningContainer(t *testing.T) {
	first, changed := applyNetworkRemoval(t, t.TempDir())

	old, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.NotEmpty(t, web.DockerID)
	require.Equal(t, old.DockerID, web.DockerID)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
}

func TestNetworkRemovalLeavesTheContainerWithNoNetwork(t *testing.T) {
	_, changed := applyNetworkRemoval(t, t.TempDir())

	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.Empty(t, web.Networks)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.NotNil(t, inspect.NetworkSettings)
	require.NotContains(t, inspect.NetworkSettings.Networks, "app")
	require.Empty(t, inspect.NetworkSettings.Networks)
}

func TestNetworkRemovalClearsTheSavedContainerAddress(t *testing.T) {
	_, changed := applyNetworkRemoval(t, t.TempDir())

	// the container is on no network, so Docker reports no address for it and
	// the provider's Update returns an empty ip_address
	web, err := xcl.Find[entities.Container](changed, "docker.container.web")
	require.NoError(t, err)
	require.Empty(t, web.IPAddress)
}

func TestNetworkRemovalRemovesTheNetworkFromTheState(t *testing.T) {
	_, changed := applyNetworkRemoval(t, t.TempDir())

	_, err := changed.FindResource("docker.network.app")
	require.Error(t, err)
	require.ErrorIs(t, err, xcl.ErrNotFound)
}

func TestPlanAfterNetworkRemovalReportsNoChanges(t *testing.T) {
	stateDir := t.TempDir()
	applyNetworkRemoval(t, stateDir)

	printed := planAgain(t, stateDir, "./config-remove")

	require.Equal(t, "Diff: no changes, 3 unchanged.\n", printed)
}

// Dangling reference: ./config then ./config-dangling

func TestDanglingReferenceFailsValidation(t *testing.T) {
	_, err := applyDanglingReference(t, t.TempDir())

	require.Error(t, err)
	require.Contains(t, err.Error(), "resource 'docker.container.web' refers to 'docker.network.app.meta.name', which")
	require.Contains(t, err.Error(), "is not defined anywhere in the configuration")
}

func TestDanglingReferenceKeepsTheDockerNetwork(t *testing.T) {
	first, _ := applyDanglingReference(t, t.TempDir())

	app, err := xcl.Find[entities.Network](first, "docker.network.app")
	require.NoError(t, err)

	inspect, err := newDockerClient(t).NetworkInspect(context.Background(), app.DockerID, network.InspectOptions{})
	require.NoError(t, err)
	require.Equal(t, app.DockerID, inspect.ID)
}

func TestDanglingReferenceKeepsTheSameRunningContainer(t *testing.T) {
	first, _ := applyDanglingReference(t, t.TempDir())

	web, err := xcl.Find[entities.Container](first, "docker.container.web")
	require.NoError(t, err)

	app, err := xcl.Find[entities.Network](first, "docker.network.app")
	require.NoError(t, err)

	inspect, err := newDockerClient(t).ContainerInspect(context.Background(), web.DockerID)
	require.NoError(t, err)
	require.Equal(t, web.DockerID, inspect.ID)
	require.NotNil(t, inspect.State)
	require.True(t, inspect.State.Running)
	require.NotNil(t, inspect.NetworkSettings)
	require.Contains(t, inspect.NetworkSettings.Networks, "app")
	require.Equal(t, app.DockerID, inspect.NetworkSettings.Networks["app"].NetworkID)
}

func TestDanglingReferenceLeavesTheSavedStateUnchanged(t *testing.T) {
	stateDir := t.TempDir()
	applyDanglingReference(t, stateDir)

	// the main configuration still matches the saved state exactly
	printed := planAgain(t, stateDir, "./config")

	require.Equal(t, "Diff: no changes, 4 unchanged.\n", printed)
}
