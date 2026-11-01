package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

const (
	installTestRepository = "acme/xcl-plugin-widget"
	installTestTag        = "v1.0.0"

	// installTestIndex is the contract's name for the index of
	// acme/xcl-plugin-widget v1.0.0
	installTestIndex = "xcl-plugin-widget_1.0.0_index.xcl"

	// installTestSignature is the contract's name for the signature over
	// installTestIndex
	installTestSignature = "xcl-plugin-widget_1.0.0_index.xcl.sig"

	// installTestHyphenIndex is the name of the index of acme/xcl-plugin-widget
	// v1.0.0 when its asset names are separated with -
	installTestHyphenIndex = "xcl-plugin-widget-1.0.0-index.xcl"

	// installTestHyphenSignature is the name of the signature over
	// installTestHyphenIndex
	installTestHyphenSignature = "xcl-plugin-widget-1.0.0-index.xcl.sig"

	// installTestImage is a container image pinned by digest
	installTestImage = "ghcr.io/acme/widget@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
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

// hostZipName is the zip of the widget plugin for the platform the test runs
// on
func hostZipName() string {
	return "xcl-plugin-widget_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"
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

func TestGitHubPluginStartKeepsTheReleaseFilesAndExtractedBuildInTheEntry(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	t.Cleanup(host.Stop)

	entry := hostEntryDirectory(cache)
	require.FileExists(t, filepath.Join(entry, installTestIndex))
	require.FileExists(t, filepath.Join(entry, installTestSignature))
	require.FileExists(t, filepath.Join(entry, hostZipName()))
	require.FileExists(t, filepath.Join(entry, hostBinaryName()))
	require.FileExists(t, filepath.Join(entry, "xcl-plugin.xcl"))
	require.FileExists(t, filepath.Join(entry, "README.md"))
}

func TestGitHubPluginInstallKeepsTheIndexAsServed(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	index, err := os.ReadFile(filepath.Join(hostEntryDirectory(cache), installTestIndex))
	require.NoError(t, err)
	require.Equal(t, fake.Index(), index)
}

func TestGitHubPluginInstallMarksTheExtractedBinaryExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no executable permission bit")
	}

	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	info, err := os.Stat(binary)
	require.NoError(t, err)
	require.NotZero(t, info.Mode().Perm()&0100, "mode %s is not executable", info.Mode())
}

func TestGitHubPluginInstallReturnsNoArgumentsForACommandWithoutAny(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, args, err := plugin.install(nil)
	require.NoError(t, err)

	require.Empty(t, args)
}

func TestGitHubPluginInstallOfASixPlatformReleaseDownloadsExactlyOneBuild(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	// the release, the index, its signature and the one zip for this
	// platform
	require.Equal(t, 4, fake.Requests())

	zips, err := filepath.Glob(filepath.Join(hostEntryDirectory(cache), "*.zip"))
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(hostEntryDirectory(cache), hostZipName())}, zips)
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

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "linux", "amd64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)

	zip, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_linux_amd64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Zip("linux", "amd64"), zip)
}

func TestGitHubPluginInstallsLinuxArm64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "linux", arch: "arm64"}
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "linux", "arm64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)

	zip, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_linux_arm64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Zip("linux", "arm64"), zip)
}

func TestGitHubPluginInstallsDarwinAmd64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "darwin", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "darwin", "amd64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)

	zip, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_darwin_amd64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Zip("darwin", "amd64"), zip)
}

func TestGitHubPluginInstallsDarwinArm64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "darwin", arch: "arm64"}
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "darwin", "arm64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget"), binary)

	zip, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_darwin_arm64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Zip("darwin", "arm64"), zip)
}

func TestGitHubPluginInstallsWindowsAmd64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "windows", arch: "amd64"}
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "windows", "amd64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget.exe"), binary)

	zip, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_windows_amd64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Zip("windows", "amd64"), zip)
}

func TestGitHubPluginInstallsWindowsArm64Build(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	gh.platform = platform{os: "windows", arch: "arm64"}
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := entryDirectory(cache, "windows", "arm64")
	require.Equal(t, filepath.Join(entry, "xcl-plugin-widget.exe"), binary)

	zip, err := os.ReadFile(filepath.Join(entry, "xcl-plugin-widget_1.0.0_windows_arm64.zip"))
	require.NoError(t, err)
	require.Equal(t, fake.Zip("windows", "arm64"), zip)
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
	require.Equal(t, "acme/xcl-plugin-widget", installError.Repository)
	require.Equal(t, "v1.0.0", installError.Version)
	require.Equal(t, hostPlatform(), installError.Platform)
	require.True(t, errors.Is(installError.Err, xclerrors.ErrPluginNotFound))
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

func TestGitHubPluginStartWithoutPlatformBuildDownloadsNoBuild(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutPlatform(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	// the release, the index and its signature, and no zip
	require.Equal(t, 3, fake.Requests())
}

func TestGitHubPluginStartWithoutIndexIsNotFound(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutIndex())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginNotFound)
	require.ErrorContains(t, err, "not a plugin release in the supported layout")
	require.ErrorContains(t, err, installTestIndex)
}

func TestGitHubPluginStartWithoutIndexDownloadsNothing(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutIndex())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	// only the release is read
	require.Equal(t, 1, fake.Requests())
}

func TestGitHubPluginStartWithoutIndexLeavesNothingInTheCache(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutIndex())
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithoutChecksumFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutChecksum(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "has no checksum")
}

func TestGitHubPluginStartWithoutChecksumDownloadsNoBuild(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutChecksum(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	// the release, the index and its signature, and no zip
	require.Equal(t, 3, fake.Requests())
}

func TestGitHubPluginStartWithoutChecksumLeavesNothingInTheCache(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutChecksum(runtime.GOOS, runtime.GOARCH))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithIndexExpressionFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.IndexExpression())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
}

func TestGitHubPluginStartWithIndexExpressionDownloadsNoBuild(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.IndexExpression())
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	// the release, the index and its signature, and no zip
	require.Equal(t, 3, fake.Requests())
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithTamperedZipFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.TamperZip(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.ErrorContains(t, err, "does not match its checksum in the index")
}

func TestGitHubPluginStartWithTamperedZipIsAPluginInstallError(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.TamperZip(runtime.GOOS, runtime.GOARCH))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	var installError *xclerrors.PluginInstallError
	require.True(t, errors.As(err, &installError))
	require.Equal(t, "acme/xcl-plugin-widget", installError.Repository)
	require.Equal(t, "v1.0.0", installError.Version)
	require.Equal(t, hostPlatform(), installError.Platform)
	require.True(t, errors.Is(installError.Err, xclerrors.ErrPluginVerification))
}

func TestGitHubPluginStartWithTamperedZipLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.TamperZip(runtime.GOOS, runtime.GOARCH))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithManifestNamingAnotherPluginFailsVerificationAndLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.ManifestDisagrees(runtime.GOOS, runtime.GOARCH, testutil.ManifestName))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `names plugin "xcl-plugin-widget-other"`)
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithManifestNamingAnotherVersionFailsVerificationAndLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.ManifestDisagrees(runtime.GOOS, runtime.GOARCH, testutil.ManifestVersion))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `names version "1.0.0-other"`)
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithManifestNamingAnotherProtocolFailsVerificationAndLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.ManifestDisagrees(runtime.GOOS, runtime.GOARCH, testutil.ManifestProtocol))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "speaks protocol [2], the index lists [1]")
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithManifestNamingAnotherPlatformFailsVerificationAndLeavesNoEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.ManifestDisagrees(runtime.GOOS, runtime.GOARCH, testutil.ManifestPlatform))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "the index lists it for "+hostPlatform())
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginInstallsTheNativeBuildOfAReleaseAlsoOfferingAContainer(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithContainer(installTestImage, "linux/amd64", "linux/arm64"))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	require.Equal(t, filepath.Join(hostEntryDirectory(cache), hostBinaryName()), binary)
	require.FileExists(t, binary)
}

func TestGitHubPluginStartOfAContainerOnlyReleaseIsNotFound(t *testing.T) {
	fake := newInstallTestRelease(t,
		testutil.WithoutExecutable(),
		testutil.WithContainer(installTestImage, "linux/amd64", "linux/arm64"),
	)
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginNotFound)
	require.ErrorContains(t, err, "release offers no build for "+hostPlatform())
	require.ErrorContains(t, err, "linux/amd64 (container), linux/arm64 (container)")
}

func TestGitHubPluginStartOfAContainerOnlyReleaseLeavesNothingInTheCache(t *testing.T) {
	fake := newInstallTestRelease(t,
		testutil.WithoutExecutable(),
		testutil.WithContainer(installTestImage, "linux/amd64", "linux/arm64"),
	)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithNonLinuxContainerPlatformFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithContainer(installTestImage, "darwin/arm64"))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "a container runs only on linux")
}

func TestGitHubPluginStartWithNonLinuxContainerPlatformDownloadsNoBuild(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithContainer(installTestImage, "darwin/arm64"))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	// the release, the index and its signature, and no zip
	require.Equal(t, 3, fake.Requests())
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithContainerNamedByTagFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithContainer("ghcr.io/acme/widget:1.0.0", "linux/amd64"))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "is not pinned by digest")
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginInstallIgnoresAnUnknownPackageKind(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithUnknownKind())
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	binary, _, err := plugin.install(nil)
	require.NoError(t, err)

	require.Equal(t, filepath.Join(hostEntryDirectory(cache), hostBinaryName()), binary)
	require.FileExists(t, binary)
}

func TestGitHubPluginStartWithAlteredCachedBinaryFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	binary := filepath.Join(hostEntryDirectory(cache), hostBinaryName())
	err = os.WriteFile(binary, []byte("not the plugin"), 0755)
	require.NoError(t, err)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.ErrorContains(t, err, "does not match its copy in zip")
}

func TestGitHubPluginStartWithAlteredCachedZipFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	zip := filepath.Join(hostEntryDirectory(cache), hostZipName())
	err = os.WriteFile(zip, []byte("not the zip"), 0644)
	require.NoError(t, err)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.ErrorContains(t, err, "does not match its checksum in the index")
}

func TestGitHubPluginStartWithAlteredCachedIndexFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	// the index is changed to carry another checksum for this platform's
	// zip, so it stays a valid index but no longer describes the cached zip
	served := sha256.Sum256(fake.Zip(runtime.GOOS, runtime.GOARCH))
	other := "sha256:" + strings.Repeat("0", 64)

	indexPath := filepath.Join(hostEntryDirectory(cache), installTestIndex)
	index, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	altered := strings.Replace(string(index), "sha256:"+hex.EncodeToString(served[:]), other, 1)
	require.NotEqual(t, string(index), altered)
	require.NoError(t, os.WriteFile(indexPath, []byte(altered), 0644))

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.ErrorContains(t, err, "does not match its checksum in the index")
}

func TestGitHubPluginStartWithMissingCachedIndexFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	require.NoError(t, os.Remove(filepath.Join(hostEntryDirectory(cache), installTestIndex)))

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.ErrorContains(t, err, "is missing")
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

func TestGitHubPluginStartInstallsAndStartsAHyphenatedRelease(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithSeparator("-"))
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, host)
	t.Cleanup(host.Stop)

	require.NotEmpty(t, host.GetTypes())
}

func TestGitHubPluginStartKeepsAHyphenatedReleasesFilesInTheEntry(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithSeparator("-"))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	t.Cleanup(host.Stop)

	entry := hostEntryDirectory(cache)
	require.FileExists(t, filepath.Join(entry, installTestHyphenIndex))
	require.FileExists(t, filepath.Join(entry, installTestHyphenSignature))
	require.FileExists(t, filepath.Join(entry, fake.ZipName(runtime.GOOS, runtime.GOARCH)))
	require.FileExists(t, filepath.Join(entry, hostBinaryName()))
	require.NoFileExists(t, filepath.Join(entry, installTestIndex))
}

func TestGitHubPluginStartVerifiesACachedHyphenatedReleaseWithoutRequests(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithSeparator("-"))
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

func TestGitHubPluginStartWithMissingCachedHyphenIndexFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithSeparator("-"))
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	require.NoError(t, os.Remove(filepath.Join(hostEntryDirectory(cache), installTestHyphenIndex)))

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "is missing")
}

func TestGitHubPluginStartWithoutIndexNamesBothIndexNames(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutIndex())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.ErrorContains(t, err, installTestIndex)
	require.ErrorContains(t, err, installTestHyphenIndex)
}

func TestGitHubPluginStartWithBothIndexesFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithBothIndexes())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "two indexes")
	require.ErrorContains(t, err, installTestIndex)
	require.ErrorContains(t, err, installTestHyphenIndex)
}

func TestGitHubPluginStartWithBothIndexesDownloadsNothing(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithBothIndexes())
	gh, _ := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	// only the release is read
	require.Equal(t, 1, fake.Requests())
}

func TestGitHubPluginStartWithBothIndexesLeavesNothingInTheCache(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithBothIndexes())
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.Start(nil)
	require.Error(t, err)

	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithBothIndexesInTheCacheFailsVerification(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh, cache := newInstallTestRegistry(t, fake)
	plugin := widgetPlugin(t, gh)

	_, _, err := plugin.install(nil)
	require.NoError(t, err)

	entry := hostEntryDirectory(cache)
	require.NoError(t, os.WriteFile(filepath.Join(entry, installTestHyphenIndex), fake.Index(), 0644))

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "two indexes")
}
