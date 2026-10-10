package registry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

const (
	installTestRepository = "acme/xcl-plugin-widget"
	installTestTag        = "v1.0.0"
)

// newInstallTestRelease serves a release of the fixture plugin as
// acme/xcl-plugin-widget v1.0.0
func newInstallTestRelease(t *testing.T, options ...testutil.FakeReleaseOption) *testutil.FakeGitHubRelease {
	t.Helper()

	binary := testutil.BuildFixturePlugin(t, "..")

	return testutil.NewFakeGitHubRelease(t, installTestRepository, installTestTag, binary, options...)
}

// newInstallTestRegistry returns a GitHub registry reading releases from fake,
// with acme/xcl-plugin-widget v1.0.0 registered, and the cache directory it
// installs into
func newInstallTestRegistry(t *testing.T, fake *testutil.FakeGitHubRelease) (*GitHub, string) {
	t.Helper()

	cache := t.TempDir()

	gh := NewGitHub(GitHubCacheDir(cache), GitHubAPIURL(fake.URL()))
	gh.RegisterPlugin(installTestRepository, installTestTag)

	return gh, cache
}

// widgetPlugin returns the one plugin the registry has, the widget plugin
func widgetPlugin(t *testing.T, gh *GitHub) *githubPlugin {
	t.Helper()

	found, err := gh.Plugins(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, found, 1)

	plugin, ok := found[0].(*githubPlugin)
	require.True(t, ok, "plugin is a %T, not a *githubPlugin", found[0])

	return plugin
}

// tagDirectory is the cache directory holding every platform's entry of
// acme/xcl-plugin-widget v1.0.0
func tagDirectory(cache string) string {
	return filepath.Join(cache, "github.com", "acme", "xcl-plugin-widget", "v1.0.0")
}

// entryDirectory is the cache entry of acme/xcl-plugin-widget v1.0.0 for
// os/arch
func entryDirectory(cache, os, arch string) string {
	return filepath.Join(tagDirectory(cache), os+"_"+arch)
}

// hostEntryDirectory is the cache entry for the platform the test runs on
func hostEntryDirectory(cache string) string {
	return entryDirectory(cache, runtime.GOOS, runtime.GOARCH)
}

// hostArchiveName is the archive of the widget plugin for the platform the
// test runs on
func hostArchiveName() string {
	if runtime.GOOS == "windows" {
		return "xcl-plugin-widget_1.0.0_windows_" + runtime.GOARCH + ".zip"
	}

	return "xcl-plugin-widget_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
}

// hostBinaryName is the widget plugin binary for the platform the test runs
// on
func hostBinaryName() string {
	if runtime.GOOS == "windows" {
		return "xcl-plugin-widget.exe"
	}

	return "xcl-plugin-widget"
}

// hostPlatform is the platform the test runs on, as os/arch
func hostPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// requireNoStagingDirectories fails the test when an .install-* directory is
// left in the tag directory
func requireNoStagingDirectories(t *testing.T, cache string) {
	t.Helper()

	staging, err := filepath.Glob(filepath.Join(tagDirectory(cache), ".install-*"))
	require.NoError(t, err)
	require.Empty(t, staging)
}

func TestGitHubPluginStartInstallsAndStartsThePlugin(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, host)
	t.Cleanup(host.Stop)

	require.NotEmpty(t, host.GetTypes())
}

func TestGitHubPluginStartKeepsArchiveChecksumsAndBinaryInTheEntry(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	t.Cleanup(host.Stop)

	entry := hostEntryDirectory(cache)
	require.FileExists(t, filepath.Join(entry, hostArchiveName()))
	require.FileExists(t, filepath.Join(entry, "xcl-plugin-widget_1.0.0_checksums.txt"))
	require.FileExists(t, filepath.Join(entry, hostBinaryName()))
}

func TestGitHubPluginStartUsesTheCacheWithoutRequests(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	first, err := plugin.Start(nil)
	require.NoError(t, err)
	t.Cleanup(first.Stop)

	requests := fake.Requests()
	fake.Close()

	second, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, second)
	t.Cleanup(second.Stop)

	require.Equal(t, requests, fake.Requests())
}

func TestGitHubPluginInstallsLinuxAmd64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "linux", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "linux", "amd64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)
	require.Equal(t, "linux_amd64", filepath.Base(filepath.Dir(binary)))

	archive, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz"))
	require.NoError(t, err)
	require.Equal(t, fake.Archive("linux", "amd64"), archive)
}

func TestGitHubPluginInstallsLinuxArm64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "linux", arch: "arm64"}
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "linux", "arm64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)
	require.Equal(t, "linux_arm64", filepath.Base(filepath.Dir(binary)))

	archive, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_linux_arm64.tar.gz"))
	require.NoError(t, err)
	require.Equal(t, fake.Archive("linux", "arm64"), archive)
}

func TestGitHubPluginInstallsDarwinAmd64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "darwin", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "darwin", "amd64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)
	require.Equal(t, "darwin_amd64", filepath.Base(filepath.Dir(binary)))

	archive, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_darwin_amd64.tar.gz"))
	require.NoError(t, err)
	require.Equal(t, fake.Archive("darwin", "amd64"), archive)
}

func TestGitHubPluginInstallsDarwinArm64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "darwin", arch: "arm64"}
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "darwin", "arm64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)
	require.Equal(t, "darwin_arm64", filepath.Base(filepath.Dir(binary)))

	archive, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_darwin_arm64.tar.gz"))
	require.NoError(t, err)
	require.Equal(t, fake.Archive("darwin", "arm64"), archive)
}

func TestGitHubPluginInstallsWindowsAmd64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "windows", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "windows", "amd64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget.exe"), binary)
	require.Equal(t, "windows_amd64", filepath.Base(filepath.Dir(binary)))

	archive, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_windows_amd64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Archive("windows", "amd64"), archive)
}

func TestGitHubPluginInstallsWindowsArm64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "windows", arch: "arm64"}
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "windows", "arm64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget.exe"), binary)
	require.Equal(t, "windows_arm64", filepath.Base(filepath.Dir(binary)))

	archive, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_windows_arm64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Archive("windows", "arm64"), archive)
}

func TestGitHubPluginStartWithoutPlatformBuildIsNotFound(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutPlatform(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginNotFound))
}

func TestGitHubPluginStartWithoutPlatformBuildIsAPluginInstallError(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutPlatform(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	var installError *xclerrors.PluginInstallError
	require.True(t, errors.As(err, &installError))
	require.ErrorContains(t, err, "acme/xcl-plugin-widget")
	require.ErrorContains(t, err, "v1.0.0")
	require.ErrorContains(t, err, hostPlatform())
}

func TestGitHubPluginStartWithoutPlatformBuildLeavesNothingInTheCache(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutPlatform(runtime.GOOS, runtime.GOARCH))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithoutChecksumsFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutChecksums())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
}

func TestGitHubPluginStartWithoutChecksumsLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutChecksums())
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithTamperedArchiveFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.TamperArchive(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
}

func TestGitHubPluginStartWithTamperedArchiveLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.TamperArchive(runtime.GOOS, runtime.GOARCH))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithAlteredCachedBinaryFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.install(nil)
	require.NoError(t, err)

	binary := filepath.Join(hostEntryDirectory(cache), hostBinaryName())
	err = os.WriteFile(binary, []byte("not the plugin"), 0755)
	require.NoError(t, err)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
}

func TestGitHubPluginStartWithAlteredCachedArchiveFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.install(nil)
	require.NoError(t, err)

	archive := filepath.Join(hostEntryDirectory(cache), hostArchiveName())
	err = os.WriteFile(archive, []byte("not the archive"), 0644)
	require.NoError(t, err)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
}

func TestGitHubPluginStartVerifiesTheCacheAfterASuccessfulStart(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	first, err := plugin.Start(nil)
	require.NoError(t, err)
	t.Cleanup(first.Stop)

	// the binary is running, so it is replaced rather than written over
	binary := filepath.Join(hostEntryDirectory(cache), hostBinaryName())
	require.NoError(t, os.Remove(binary))
	require.NoError(t, os.WriteFile(binary, []byte("not the plugin"), 0755))

	second, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, second)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
}

func TestGitHubPluginsReturnsEveryRegisteredPluginInOrderFromTheLocalRegistry(t *testing.T) {
	gh := NewGitHub(GitHubCacheDir(t.TempDir()))
	gh.RegisterPlugin("acme/xcl-plugin-widget", "v1.0.0")
	gh.RegisterPlugin("acme/xcl-plugin-gadget", "v2.0.0")

	found, err := gh.Plugins(context.Background(), nil)
	require.NoError(t, err)

	require.Equal(t, []string{"xcl-plugin-widget", "xcl-plugin-gadget"}, pluginNames(found))
}

func TestGitHubPluginStartIgnoresAnInterruptedInstall(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	leftover := filepath.Join(tagDirectory(cache), ".install-123")
	require.NoError(t, os.MkdirAll(leftover, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(leftover, hostBinaryName()), []byte("partial"), 0755))

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, host)
	t.Cleanup(host.Stop)

	require.Greater(t, fake.Requests(), 0)
}

func TestGitHubPluginStartOnUnsupportedPlatformIsNotFound(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "freebsd", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginNotFound))
}

func TestGitHubPluginStartOnUnsupportedPlatformMakesNoRequests(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "freebsd", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.Equal(t, 0, fake.Requests())
}
