package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ Registry = NewGitHub()

// recoverPanic runs fn and returns the panic message, or an empty string when
// fn does not panic
func recoverPanic(fn func()) (message string) {
	defer func() {
		if r := recover(); r != nil {
			message = fmt.Sprint(r)
		}
	}()

	fn()

	return ""
}

func TestGitHubNameIsGitHubCom(t *testing.T) {
	require.Equal(t, "github.com", NewGitHub().Name())
}

func TestGitHubTypesIsNilWithNothingRegistered(t *testing.T) {
	require.Nil(t, NewGitHub().Types())
}

func TestGitHubTypesReturnsRegisteredType(t *testing.T) {
	gh := NewGitHub()
	gh.RegisterType(&Thing{}, "resource", "thing")

	declared := gh.Types()

	require.Len(t, declared, 1)
	require.Equal(t, Type{Type: "resource", Subtype: "thing", Prototype: &Thing{}}, declared[0])
}

func TestGitHubDefaultCacheRootIsUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := NewGitHub().cacheRoot()

	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".xcl", "cache", "plugins"), root)
}

func TestGitHubCacheDirIsUsed(t *testing.T) {
	dir := t.TempDir()

	root, err := NewGitHub(GitHubCacheDir(dir)).cacheRoot()

	require.NoError(t, err)
	require.Equal(t, dir, root)
}

func TestGitHubCacheDirExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := NewGitHub(GitHubCacheDir("~/plugins")).cacheRoot()

	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, "plugins"), root)
}

func TestGitHubCacheDirEmptyKeepsDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := NewGitHub(GitHubCacheDir("")).cacheRoot()

	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".xcl", "cache", "plugins"), root)
}

func TestGitHubDefaultAPIURL(t *testing.T) {
	require.Equal(t, DefaultGitHubAPIURL, NewGitHub().apiURL)
	require.Equal(t, "https://api.github.com", NewGitHub().apiURL)
}

func TestGitHubAPIURLTrimsTrailingSlash(t *testing.T) {
	gh := NewGitHub(GitHubAPIURL("http://x/"))

	require.Equal(t, "http://x", gh.apiURL)
}

func TestGitHubAPIURLEmptyKeepsDefault(t *testing.T) {
	gh := NewGitHub(GitHubAPIURL(""))

	require.Equal(t, DefaultGitHubAPIURL, gh.apiURL)
}

func TestGitHubRegisterPluginValidDoesNotPanic(t *testing.T) {
	gh := NewGitHub()

	require.NotPanics(t, func() { gh.RegisterPlugin("acme/xcl-plugin-widget", "v1.2.0") })
	require.Len(t, gh.local.entries, 1)
}

func TestGitHubRegisterPluginAcceptsPreRelease(t *testing.T) {
	gh := NewGitHub()

	require.NotPanics(t, func() { gh.RegisterPlugin("acme/xcl-plugin-widget", "v1.2.0-rc.1") })
	require.Len(t, gh.local.entries, 1)
}

func TestGitHubPluginsReturnsPluginInRegistrationOrder(t *testing.T) {
	gh := NewGitHub()
	gh.RegisterPlugin("acme/xcl-plugin-widget", "v1.2.0")

	found, err := gh.Plugins(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, "xcl-plugin-widget", found[0].Name())
}

func TestGitHubPluginsReturnsSecondPluginAfterFirst(t *testing.T) {
	gh := NewGitHub()
	gh.RegisterPlugin("acme/xcl-plugin-widget", "v1.2.0")
	gh.RegisterPlugin("acme/xcl-plugin-gadget", "v0.1.0")

	found, err := gh.Plugins(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, found, 2)
	require.Equal(t, "xcl-plugin-widget", found[0].Name())
	require.Equal(t, "xcl-plugin-gadget", found[1].Name())
}

func TestGitHubRegisterPluginPanicsOnEmptyVersion(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("acme/xcl-plugin-widget", "") })

	require.Contains(t, message, `xcl: registry github.com: plugin "acme/xcl-plugin-widget"`)
}

func TestGitHubRegisterPluginPanicsOnLatest(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("acme/xcl-plugin-widget", "latest") })

	require.Contains(t, message, `xcl: registry github.com: plugin "acme/xcl-plugin-widget"`)
}

func TestGitHubRegisterPluginPanicsOnTildeRange(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("acme/xcl-plugin-widget", "~1.2") })

	require.Contains(t, message, `xcl: registry github.com: plugin "acme/xcl-plugin-widget"`)
}

func TestGitHubRegisterPluginPanicsOnComparisonRange(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("acme/xcl-plugin-widget", ">=1.0.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin "acme/xcl-plugin-widget"`)
}

func TestGitHubRegisterPluginPanicsOnVersionWithoutV(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("acme/xcl-plugin-widget", "1.2.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin "acme/xcl-plugin-widget"`)
}

func TestGitHubRegisterPluginPanicsOnEmptyRepository(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("", "v1.2.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin ""`)
}

func TestGitHubRegisterPluginPanicsOnRepositoryWithoutOwner(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("xcl-plugin-widget", "v1.2.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin "xcl-plugin-widget"`)
}

func TestGitHubRegisterPluginPanicsOnRepositoryWithExtraSlash(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("a/b/c", "v1.2.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin "a/b/c"`)
}

func TestGitHubRegisterPluginPanicsOnRepositoryWithEmptyOwner(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("/repo", "v1.2.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin "/repo"`)
}

func TestGitHubRegisterPluginPanicsOnRepositoryWithEmptyName(t *testing.T) {
	message := recoverPanic(func() { NewGitHub().RegisterPlugin("owner/", "v1.2.0") })

	require.Contains(t, message, `xcl: registry github.com: plugin "owner/"`)
}

func TestGitHubRegisterPluginPanicAddsNoPlugin(t *testing.T) {
	gh := NewGitHub()

	recoverPanic(func() { gh.RegisterPlugin("acme/xcl-plugin-widget", "latest") })

	require.Empty(t, gh.local.entries)
}

func TestGitHubRegisterPluginPanicMakesNoRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gh := NewGitHub(GitHubAPIURL(server.URL))

	message := recoverPanic(func() { gh.RegisterPlugin("acme/xcl-plugin-widget", "latest") })

	require.NotEmpty(t, message)
	require.Equal(t, int32(0), requests.Load())
}
