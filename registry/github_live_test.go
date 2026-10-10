package registry

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The test below installs a real published plugin from github.com. It runs
// only when XCL_GITHUB_LIVE_PLUGIN names one as owner/repo@vX.Y.Z, so the
// default test run makes no network requests. XCL_GITHUB_LIVE_KEY may name a
// file holding an ASCII-armoured public key to trust, and a private
// repository needs GITHUB_TOKEN or GH_TOKEN. Run it with make test-github-live.

func TestGitHubRegistryInstallsARealRelease(t *testing.T) {
	plugin := os.Getenv("XCL_GITHUB_LIVE_PLUGIN")
	if plugin == "" {
		t.Skip("set XCL_GITHUB_LIVE_PLUGIN=owner/repo@vX.Y.Z to install a real release from github.com")
	}

	repository, version, found := strings.Cut(plugin, "@")
	require.True(t, found, "XCL_GITHUB_LIVE_PLUGIN must be owner/repo@vX.Y.Z, got %q", plugin)

	options := []GitHubOption{GitHubCacheDir(t.TempDir())}

	if keyPath := os.Getenv("XCL_GITHUB_LIVE_KEY"); keyPath != "" {
		key, err := os.ReadFile(keyPath)
		require.NoError(t, err)

		options = append(options, GitHubTrustedKeys(string(key)))
	}

	gh := NewGitHub(options...)
	gh.RegisterPlugin(repository, version)

	provided, err := gh.Plugins(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, provided, 1)

	host, err := provided[0].Start(nil)
	require.NoError(t, err)
	t.Cleanup(host.Stop)

	require.NotEmpty(t, host.GetTypes())
}
