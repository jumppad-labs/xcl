package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

const (
	clientTestRepository = "acme/xcl-plugin-widget"
	clientTestTag        = "v1.0.0"
	clientTestArchive    = "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz"
)

func newClientTestRelease(t *testing.T, options ...testutil.FakeReleaseOption) *testutil.FakeGitHubRelease {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "xcl-plugin-widget")
	require.NoError(t, os.WriteFile(binary, []byte("fake plugin binary"), 0755))

	return testutil.NewFakeGitHubRelease(t, clientTestRepository, clientTestTag, binary, options...)
}

func TestReleaseReadsExistingRelease(t *testing.T) {
	fake := newClientTestRelease(t)
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	release, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.NoError(t, err)

	require.Equal(t, "v1.0.0", release.TagName)
	require.False(t, release.Draft)

	_, found := release.asset(clientTestArchive)
	require.True(t, found)
}

func TestReleaseAssetFindsByExactName(t *testing.T) {
	fake := newClientTestRelease(t)
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	release, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.NoError(t, err)

	asset, found := release.asset(clientTestArchive)
	require.True(t, found)
	require.Equal(t, clientTestArchive, asset.Name)
	require.NotEmpty(t, asset.URL)
}

func TestReleaseAssetUnknownNameIsNotFound(t *testing.T) {
	fake := newClientTestRelease(t)
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	release, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.NoError(t, err)

	_, found := release.asset("xcl-plugin-widget_1.0.0_plan9_amd64.tar.gz")
	require.False(t, found)
}

func TestDownloadWritesArchiveBytes(t *testing.T) {
	fake := newClientTestRelease(t)
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	release, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.NoError(t, err)

	asset, found := release.asset(clientTestArchive)
	require.True(t, found)

	var written bytesBuffer
	require.NoError(t, client.download(context.Background(), asset, &written))
	require.Equal(t, fake.Archive("linux", "amd64"), written.data)
}

func TestDownloadToWritesFile(t *testing.T) {
	fake := newClientTestRelease(t)
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	release, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.NoError(t, err)

	asset, found := release.asset(clientTestArchive)
	require.True(t, found)

	path := filepath.Join(t.TempDir(), "plugin.tar.gz")
	require.NoError(t, downloadTo(context.Background(), client, asset, path))

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, fake.Archive("linux", "amd64"), contents)
}

func TestReleaseMissingTagIsPluginNotFound(t *testing.T) {
	fake := newClientTestRelease(t)
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	_, err := client.release(context.Background(), clientTestRepository, "v9.9.9")
	require.ErrorIs(t, err, xclerrors.ErrPluginNotFound)
	require.Contains(t, err.Error(), "acme/xcl-plugin-widget")
	require.Contains(t, err.Error(), "v9.9.9")
}

func TestReleaseDraftIsPluginNotFound(t *testing.T) {
	fake := newClientTestRelease(t, testutil.Draft())
	client := &releaseClient{baseURL: fake.URL(), http: http.DefaultClient}

	_, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.ErrorIs(t, err, xclerrors.ErrPluginNotFound)
}

func TestReleaseServerErrorIsNotPluginNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := &releaseClient{baseURL: server.URL, http: http.DefaultClient}

	_, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
	require.NotErrorIs(t, err, xclerrors.ErrPluginNotFound)
}

func TestReleaseSendsGitHubHeaders(t *testing.T) {
	var headers http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.Write([]byte(`{"tag_name":"v1.0.0","draft":false,"assets":[]}`))
	}))
	t.Cleanup(server.Close)

	client := &releaseClient{baseURL: server.URL, http: http.DefaultClient}

	_, err := client.release(context.Background(), clientTestRepository, clientTestTag)
	require.NoError(t, err)

	require.Equal(t, "application/vnd.github+json", headers.Get("Accept"))
	require.Equal(t, "2022-11-28", headers.Get("X-GitHub-Api-Version"))
	require.Equal(t, "xcl", headers.Get("User-Agent"))
}

func TestDownloadSendsOctetStreamAccept(t *testing.T) {
	var headers http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.Write([]byte("bytes"))
	}))
	t.Cleanup(server.Close)

	client := &releaseClient{baseURL: server.URL, http: http.DefaultClient}
	asset := githubAsset{ID: 1, Name: clientTestArchive, URL: server.URL + "/asset"}

	var written bytesBuffer
	require.NoError(t, client.download(context.Background(), asset, &written))

	require.Equal(t, "application/octet-stream", headers.Get("Accept"))
	require.Equal(t, "2022-11-28", headers.Get("X-GitHub-Api-Version"))
	require.Equal(t, "xcl", headers.Get("User-Agent"))
}

// bytesBuffer collects what download writes
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}
