package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// clearTokenEnvironment empties both token variables for the test
func clearTokenEnvironment(t *testing.T) {
	t.Helper()

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
}

func TestGitHubInstallUsesTheTokenOption(t *testing.T) {
	clearTokenEnvironment(t)

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh := NewGitHub(GitHubCacheDir(t.TempDir()), GitHubAPIURL(fake.URL()), GitHubToken("secret"))
	gh.RegisterPlugin(installTestRepository, installTestTag)

	binary, err := widgetPlugin(t, gh).install(nil)
	require.NoError(t, err)
	require.FileExists(t, binary)
	require.Equal(t, "secret", fake.LastToken())
}

func TestGitHubInstallUsesGithubTokenVariable(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GITHUB_TOKEN", "secret")

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh, _ := newInstallTestRegistry(t, fake)

	binary, err := widgetPlugin(t, gh).install(nil)
	require.NoError(t, err)
	require.FileExists(t, binary)
	require.Equal(t, "secret", fake.LastToken())
}

func TestGitHubInstallUsesGhTokenVariable(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GH_TOKEN", "secret")

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh, _ := newInstallTestRegistry(t, fake)

	binary, err := widgetPlugin(t, gh).install(nil)
	require.NoError(t, err)
	require.FileExists(t, binary)
	require.Equal(t, "secret", fake.LastToken())
}

func TestGitHubInstallTokenOptionBeatsGithubTokenVariable(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GITHUB_TOKEN", "wrong")

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh := NewGitHub(GitHubCacheDir(t.TempDir()), GitHubAPIURL(fake.URL()), GitHubToken("secret"))
	gh.RegisterPlugin(installTestRepository, installTestTag)

	_, err := widgetPlugin(t, gh).install(nil)
	require.NoError(t, err)
	require.Equal(t, "secret", fake.LastToken())
}

func TestGitHubInstallGithubTokenBeatsGhToken(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("GH_TOKEN", "wrong")

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh, _ := newInstallTestRegistry(t, fake)

	_, err := widgetPlugin(t, gh).install(nil)
	require.NoError(t, err)
	require.Equal(t, "secret", fake.LastToken())
}

func TestGitHubInstallPublicReleaseSendsNoAuthorization(t *testing.T) {
	clearTokenEnvironment(t)

	fake := newInstallTestRelease(t)
	gh, _ := newInstallTestRegistry(t, fake)

	_, err := widgetPlugin(t, gh).install(nil)
	require.NoError(t, err)
	require.Equal(t, "", fake.LastToken())
}

func TestGitHubInstallPrivateReleaseWithoutTokenIsPluginNotFound(t *testing.T) {
	clearTokenEnvironment(t)

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh, _ := newInstallTestRegistry(t, fake)

	_, err := widgetPlugin(t, gh).install(nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrPluginNotFound))
	require.Contains(t, err.Error(), "acme/xcl-plugin-widget")
	require.Contains(t, err.Error(), "token")
}

func TestGitHubInstallPrivateReleaseWithWrongTokenIsPluginNotFound(t *testing.T) {
	clearTokenEnvironment(t)

	fake := newInstallTestRelease(t, testutil.RequireToken("secret"))
	gh := NewGitHub(GitHubCacheDir(t.TempDir()), GitHubAPIURL(fake.URL()), GitHubToken("wrong"))
	gh.RegisterPlugin(installTestRepository, installTestTag)

	_, err := widgetPlugin(t, gh).install(nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, xclerrors.ErrPluginNotFound))
	require.NotContains(t, err.Error(), "set GITHUB_TOKEN")
}

func TestReleaseClientRejectedTokenSaysSo(t *testing.T) {
	clearTokenEnvironment(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := &releaseClient{baseURL: server.URL, token: "secret", http: http.DefaultClient}

	_, err := client.release(context.Background(), installTestRepository, installTestTag)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token was rejected")
}

func TestResolveTokenPrefersTheOption(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GITHUB_TOKEN", "from-github")
	t.Setenv("GH_TOKEN", "from-gh")

	require.Equal(t, "option", resolveToken("option"))
}

func TestResolveTokenFallsBackToGithubToken(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GITHUB_TOKEN", "from-github")
	t.Setenv("GH_TOKEN", "from-gh")

	require.Equal(t, "from-github", resolveToken(""))
}

func TestResolveTokenFallsBackToGhToken(t *testing.T) {
	clearTokenEnvironment(t)
	t.Setenv("GH_TOKEN", "from-gh")

	require.Equal(t, "from-gh", resolveToken(""))
}

func TestResolveTokenIsEmptyWithoutAnySource(t *testing.T) {
	clearTokenEnvironment(t)

	require.Equal(t, "", resolveToken(""))
}
