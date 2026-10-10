package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// newSignatureTestRegistry returns a GitHub registry reading releases from
// apiURL into cache, trusting keys, with acme/xcl-plugin-widget v1.0.0
// registered
func newSignatureTestRegistry(t *testing.T, apiURL, cache string, keys ...string) *GitHub {
	t.Helper()

	options := []GitHubOption{GitHubCacheDir(cache), GitHubAPIURL(apiURL)}
	if len(keys) > 0 {
		options = append(options, GitHubTrustedKeys(keys...))
	}

	gh := NewGitHub(options...)
	gh.RegisterPlugin(installTestRepository, installTestTag)

	return gh
}

func TestGitHubPluginStartWithTrustedKeyStartsSignedRelease(t *testing.T) {
	fake := newInstallTestRelease(t)
	gh := newSignatureTestRegistry(t, fake.URL(), t.TempDir(), fake.PublicKey())
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, host)
	t.Cleanup(host.Stop)
}

func TestGitHubPluginStartWithTrustedKeyKeepsSignatureInTheEntry(t *testing.T) {
	fake := newInstallTestRelease(t)
	cache := t.TempDir()
	gh := newSignatureTestRegistry(t, fake.URL(), cache, fake.PublicKey())
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	t.Cleanup(host.Stop)

	require.FileExists(t, filepath.Join(hostEntryDirectory(cache), fake.SignatureName()))
}

func TestGitHubPluginStartWithTrustedKeyRefusesReleaseSignedByOtherKey(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.SignedByOtherKey())
	cache := t.TempDir()
	gh := newSignatureTestRegistry(t, fake.URL(), cache, fake.PublicKey())
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)
	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.Contains(t, err.Error(), installTestRepository)
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginStartWithTrustedKeyRefusesUnsignedRelease(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutSignature())
	cache := t.TempDir()
	gh := newSignatureTestRegistry(t, fake.URL(), cache, fake.PublicKey())
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)
	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.NoDirExists(t, hostEntryDirectory(cache))
	requireNoStagingDirectories(t, cache)
}

func TestGitHubPluginInstallWithoutTrustedKeysInstallsUnsignedRelease(t *testing.T) {
	fake := newInstallTestRelease(t, testutil.WithoutSignature())
	cache := t.TempDir()
	gh := newSignatureTestRegistry(t, fake.URL(), cache)
	plugin := widgetPlugin(t, gh)

	binary, err := plugin.install(nil)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(hostEntryDirectory(cache), hostBinaryName()), binary)
	require.FileExists(t, binary)
	require.NoFileExists(t, filepath.Join(hostEntryDirectory(cache), fake.SignatureName()))
}

func TestGitHubPluginInstallWithoutTrustedKeysKeepsSignatureOfSignedRelease(t *testing.T) {
	fake := newInstallTestRelease(t)
	cache := t.TempDir()
	gh := newSignatureTestRegistry(t, fake.URL(), cache)
	plugin := widgetPlugin(t, gh)

	_, err := plugin.install(nil)
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(hostEntryDirectory(cache), fake.SignatureName()))
}

func TestGitHubPluginStartRefusesCachedEntryNotSignedByNowTrustedKeyOffline(t *testing.T) {
	fake := newInstallTestRelease(t)
	cache := t.TempDir()

	installer := newSignatureTestRegistry(t, fake.URL(), cache)
	_, err := widgetPlugin(t, installer).install(nil)
	require.NoError(t, err)

	fake.Close()

	gh := newSignatureTestRegistry(t, fake.URL(), cache, fake.OtherPublicKey())
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)
	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
}

func TestGitHubPluginStartUsesCachedEntrySignedByTrustedKeyOffline(t *testing.T) {
	fake := newInstallTestRelease(t)
	cache := t.TempDir()

	installer := newSignatureTestRegistry(t, fake.URL(), cache, fake.PublicKey())
	_, err := widgetPlugin(t, installer).install(nil)
	require.NoError(t, err)

	fake.Close()

	gh := newSignatureTestRegistry(t, fake.URL(), cache, fake.PublicKey())
	plugin := widgetPlugin(t, gh)

	host, err := plugin.Start(nil)
	require.NoError(t, err)
	require.NotNil(t, host)
	t.Cleanup(host.Stop)
}

func TestGitHubPluginsFailsOnMalformedTrustedKey(t *testing.T) {
	gh := NewGitHub(GitHubCacheDir(t.TempDir()), GitHubTrustedKeys("not a key"))
	gh.RegisterPlugin(installTestRepository, installTestTag)

	found, err := gh.Plugins(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, found)
	require.Contains(t, err.Error(), "trusted key 1")
}

func TestGitHubPluginStartRefusesCachedEntryWithAlteredSignature(t *testing.T) {
	fake := newInstallTestRelease(t)
	cache := t.TempDir()
	gh := newSignatureTestRegistry(t, fake.URL(), cache, fake.PublicKey())
	plugin := widgetPlugin(t, gh)

	_, err := plugin.install(nil)
	require.NoError(t, err)

	signaturePath := filepath.Join(hostEntryDirectory(cache), fake.SignatureName())
	signature, err := os.ReadFile(signaturePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(signaturePath, signature[:len(signature)/2], 0644))

	host, err := plugin.Start(nil)
	require.Error(t, err)
	require.Nil(t, host)
	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
}
