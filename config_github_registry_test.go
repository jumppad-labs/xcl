package xcl

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// A Config can get its plugins from a GitHub registry, which installs each
// declared release into its cache when plugins load, verifies it and starts it
// as an external plugin. These tests serve the fixture widget plugin from a
// fake GitHub release.

const (
	// githubWidgetRepository is the repository the fake widget release is
	// published from, named after the fixture plugin binary
	githubWidgetRepository = "acme/xcl-plugin-widget"

	// githubWidgetTag is the release tag of the fake widget release
	githubWidgetTag = "v1.0.0"
)

// newGitHubWidgetRelease builds the fixture widget plugin and serves it as a
// fake GitHub release of githubWidgetRepository at githubWidgetTag. It must be
// called before isolateHome, so the build uses the real module cache.
func newGitHubWidgetRelease(t *testing.T, options ...testutil.FakeReleaseOption) *testutil.FakeGitHubRelease {
	t.Helper()

	binary := testutil.BuildFixturePlugin(t, ".")

	return testutil.NewFakeGitHubRelease(t, githubWidgetRepository, githubWidgetTag, binary, options...)
}

// newGitHubWidgetRegistry returns a GitHub registry reading releases from the
// fake GitHub API at apiURL, installing into cacheDir, with the widget plugin
// declared at tag
func newGitHubWidgetRegistry(apiURL, cacheDir, tag string) *registry.GitHub {
	gh := registry.NewGitHub(registry.GitHubAPIURL(apiURL), registry.GitHubCacheDir(cacheDir))
	gh.RegisterPlugin(githubWidgetRepository, tag)

	return gh
}

// newGitHubConfig returns a Config using registries, keeping state in
// stateDir, whose plugin hosts are stopped when the test ends
func newGitHubConfig(t *testing.T, stateDir string, registries ...registry.Registry) *Config {
	t.Helper()

	options := []ConfigOption{WithStatePath(stateDir)}
	for _, r := range registries {
		options = append(options, WithRegistry(r))
	}

	c, err := NewConfig(options...)
	require.NoError(t, err)
	stopPluginHosts(t, c)

	return c
}

// cachedWidgetBinary is where the GitHub registry keeps the widget plugin
// binary for the current platform under cacheDir
func cachedWidgetBinary(cacheDir string) string {
	name := testutil.FixturePluginName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return filepath.Join(
		cacheDir,
		"github.com", "acme", "xcl-plugin-widget", githubWidgetTag,
		runtime.GOOS+"_"+runtime.GOARCH,
		name,
	)
}

func TestApplyWithGitHubPluginAndEmptyCacheSucceeds(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), githubWidgetTag)
	c := newGitHubConfig(t, t.TempDir(), gh)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.NoError(t, err)
}

func TestDiffAfterApplyWithGitHubPluginReportsNoChanges(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), githubWidgetTag)
	c := newGitHubConfig(t, t.TempDir(), gh)
	fixture := subtypelessFixture(t, "github")

	err := c.Apply(fixture)
	require.NoError(t, err)

	result, err := c.Diff([]string{fixture})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Empty(t, result.Resources)
	require.Equal(t, 0, result.Changed())
	require.Equal(t, 1, result.Summary.Unchanged)
}

func TestApplyWithGitHubPluginStoresItsResource(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), githubWidgetTag)
	c := newGitHubConfig(t, t.TempDir(), gh)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](c, "widget.app")
	require.NoError(t, err)
	require.Equal(t, "widget.app", widget.Meta.ID)
	require.Equal(t, 1, widget.Size)

	// the plugin computes part_names, so this proves the resource was created
	// by the plugin installed from the release
	require.Equal(t, "a", widget.PartNames)
}

func TestDestroyAfterApplyWithGitHubPluginSucceeds(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), githubWidgetTag)
	c := newGitHubConfig(t, t.TempDir(), gh)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.NoError(t, err)

	err = c.Destroy()
	require.NoError(t, err)

	require.Equal(t, 0, c.ResourceCount())

	_, err = Find[subtypelessWidget](c, "widget.app")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestApplyWithCachedGitHubPluginWorksOffline(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	cacheDir := t.TempDir()
	stateDir := t.TempDir()
	fixture := subtypelessFixture(t, "github")

	first := newGitHubConfig(t, stateDir, newGitHubWidgetRegistry(fake.URL(), cacheDir, githubWidgetTag))

	err := first.Apply(fixture)
	require.NoError(t, err)

	fake.Close()
	requestsBeforeOffline := fake.Requests()

	second := newGitHubConfig(t, stateDir, newGitHubWidgetRegistry(fake.URL(), cacheDir, githubWidgetTag))

	err = second.Apply(fixture)
	require.NoError(t, err)

	require.Equal(t, requestsBeforeOffline, fake.Requests())
	require.FileExists(t, cachedWidgetBinary(cacheDir))

	widget, err := Find[subtypelessWidget](second, "widget.app")
	require.NoError(t, err)
	require.Equal(t, "a", widget.PartNames)
}

func TestApplyWithMissingGitHubReleaseFailsNamingPlugin(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), "v9.9.9")
	stateDir := t.TempDir()
	c := newGitHubConfig(t, stateDir, gh)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPluginNotFound)

	var loadErr *PluginLoadError
	require.True(t, errors.As(err, &loadErr))
	require.Equal(t, "xcl-plugin-widget", loadErr.Plugin)
	require.Equal(t, "github.com", loadErr.Registry)

	require.Equal(t, 0, c.ResourceCount())

	_, err = Find[subtypelessWidget](c, "widget.app")
	require.ErrorIs(t, err, ErrNotFound)

	// nothing was written to state either
	entries, err := os.ReadDir(stateDir)
	require.NoError(t, err)
	for _, entry := range entries {
		contents, err := os.ReadFile(filepath.Join(stateDir, entry.Name()))
		require.NoError(t, err)
		require.NotContains(t, string(contents), "widget.app")
	}
}

func TestApplyWithGitHubAndLocalRegistriesUsesBoth(t *testing.T) {
	fake := newGitHubWidgetRelease(t)
	isolateHome(t)

	local := registry.NewLocal()
	local.RegisterPlugin(&PersonPlugin{})

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), githubWidgetTag)
	c := newGitHubConfig(t, t.TempDir(), local, gh)

	err := c.Apply(subtypelessFixture(t, "github_mixed"))
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](c, "widget.app")
	require.NoError(t, err)
	require.Equal(t, "a", widget.PartNames)

	ada, err := Find[person.Person](c, "resource.person.ada")
	require.NoError(t, err)
	require.Equal(t, "Ada", ada.FirstName)
}

func TestApplyWithUnsignedGitHubReleaseAndNoTrustedKeysSucceeds(t *testing.T) {
	fake := newGitHubWidgetRelease(t, testutil.WithoutSignature())
	isolateHome(t)

	gh := newGitHubWidgetRegistry(fake.URL(), t.TempDir(), githubWidgetTag)
	c := newGitHubConfig(t, t.TempDir(), gh)

	err := c.Apply(subtypelessFixture(t, "github"))
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](c, "widget.app")
	require.NoError(t, err)
	require.Equal(t, "a", widget.PartNames)
}
