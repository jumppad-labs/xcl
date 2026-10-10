package testutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/require"
)

const (
	testRepository = "acme/xcl-plugin-widget"
	testTag        = "v1.0.0"
)

type testRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Assets  []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

func (r testRelease) names() []string {
	names := []string{}
	for _, asset := range r.Assets {
		names = append(names, asset.Name)
	}

	return names
}

func (r testRelease) assetURL(t *testing.T, name string) string {
	t.Helper()

	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset.URL
		}
	}

	require.FailNow(t, "asset not found", name)

	return ""
}

func newTestRelease(t *testing.T, options ...FakeReleaseOption) *FakeGitHubRelease {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "xcl-plugin-widget")
	require.NoError(t, os.WriteFile(binary, []byte("not really a plugin"), 0755))

	return NewFakeGitHubRelease(t, testRepository, testTag, binary, options...)
}

func get(t *testing.T, url, token, accept string) (int, []byte) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	if accept != "" {
		request.Header.Set("Accept", accept)
	}

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return response.StatusCode, body
}

func fetchRelease(t *testing.T, fake *FakeGitHubRelease) testRelease {
	t.Helper()

	status, body := get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "", "")
	require.Equal(t, http.StatusOK, status)

	var release testRelease
	require.NoError(t, json.Unmarshal(body, &release))

	return release
}

func download(t *testing.T, fake *FakeGitHubRelease, name string) []byte {
	t.Helper()

	release := fetchRelease(t, fake)

	status, body := get(t, release.assetURL(t, name), "", "application/octet-stream")
	require.Equal(t, http.StatusOK, status)

	return body
}

func checksumLine(t *testing.T, checksums, name string) string {
	t.Helper()

	for _, line := range strings.Split(checksums, "\n") {
		if strings.HasSuffix(line, "  "+name) {
			return line
		}
	}

	require.FailNow(t, "no checksum line", name)

	return ""
}

func TestFakeReleaseListsAllAssets(t *testing.T) {
	fake := newTestRelease(t)

	release := fetchRelease(t, fake)

	require.Equal(t, "v1.0.0", release.TagName)
	require.ElementsMatch(t, []string{
		"xcl-plugin-widget_1.0.0_linux_amd64.tar.gz",
		"xcl-plugin-widget_1.0.0_linux_arm64.tar.gz",
		"xcl-plugin-widget_1.0.0_darwin_amd64.tar.gz",
		"xcl-plugin-widget_1.0.0_darwin_arm64.tar.gz",
		"xcl-plugin-widget_1.0.0_windows_amd64.zip",
		"xcl-plugin-widget_1.0.0_windows_arm64.zip",
		"xcl-plugin-widget_1.0.0_checksums.txt",
		"xcl-plugin-widget_1.0.0_checksums.txt.sig",
	}, release.names())
}

func TestFakeReleaseNamesMatchContract(t *testing.T) {
	fake := newTestRelease(t)

	require.Equal(t, "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz", fake.ArchiveName("linux", "amd64"))
	require.Equal(t, "xcl-plugin-widget_1.0.0_windows_arm64.zip", fake.ArchiveName("windows", "arm64"))
	require.Equal(t, "xcl-plugin-widget_1.0.0_checksums.txt", fake.ChecksumsName())
	require.Equal(t, "xcl-plugin-widget_1.0.0_checksums.txt.sig", fake.SignatureName())
}

func TestFakeReleaseServesArchiveBytes(t *testing.T) {
	fake := newTestRelease(t)

	body := download(t, fake, "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz")

	require.NotEmpty(t, body)
	require.Equal(t, fake.Archive("linux", "amd64"), body)
}

func TestFakeReleaseAssetWithoutOctetStreamAcceptIsMetadata(t *testing.T) {
	fake := newTestRelease(t)
	release := fetchRelease(t, fake)

	status, body := get(t, release.assetURL(t, fake.ChecksumsName()), "", "")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(body), `"name":"xcl-plugin-widget_1.0.0_checksums.txt"`)
}

func TestFakeReleaseChecksumsListArchiveSum(t *testing.T) {
	fake := newTestRelease(t)

	checksums := string(download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt"))

	sum := sha256.Sum256(fake.Archive("linux", "amd64"))
	expected := hex.EncodeToString(sum[:]) + "  xcl-plugin-widget_1.0.0_linux_amd64.tar.gz"

	require.Equal(t, expected, checksumLine(t, checksums, "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz"))
}

func TestFakeReleaseWithoutPlatformOmitsArchive(t *testing.T) {
	fake := newTestRelease(t, WithoutPlatform("darwin", "arm64"))

	release := fetchRelease(t, fake)

	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_darwin_arm64.tar.gz")
	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_darwin_amd64.tar.gz")
	require.Nil(t, fake.Archive("darwin", "arm64"))
	require.Len(t, release.Assets, 7)
}

func TestFakeReleaseWithoutPlatformOmitsChecksumLine(t *testing.T) {
	fake := newTestRelease(t, WithoutPlatform("darwin", "arm64"))

	checksums := string(download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt"))

	require.NotContains(t, checksums, "darwin_arm64")
}

func TestFakeReleaseWithoutChecksumsOmitsChecksumsAndSignature(t *testing.T) {
	fake := newTestRelease(t, WithoutChecksums())

	release := fetchRelease(t, fake)

	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_checksums.txt")
	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_checksums.txt.sig")
	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz")
	require.Len(t, release.Assets, 6)
}

func TestFakeReleaseWithoutSignatureOmitsOnlySignature(t *testing.T) {
	fake := newTestRelease(t, WithoutSignature())

	release := fetchRelease(t, fake)

	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_checksums.txt.sig")
	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_checksums.txt")
	require.Len(t, release.Assets, 7)
}

func TestFakeReleaseSignatureVerifiesAgainstPublicKey(t *testing.T) {
	fake := newTestRelease(t)

	checksums := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt.sig")

	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(fake.PublicKey()))
	require.NoError(t, err)

	_, err = openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(string(checksums)), strings.NewReader(string(signature)), nil)
	require.NoError(t, err)
}

func TestFakeReleaseSignatureDoesNotVerifyAgainstOtherKey(t *testing.T) {
	fake := newTestRelease(t)

	checksums := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt.sig")

	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(fake.OtherPublicKey()))
	require.NoError(t, err)

	_, err = openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(string(checksums)), strings.NewReader(string(signature)), nil)
	require.Error(t, err)
}

func TestFakeReleaseSignedByOtherKeyDoesNotVerifyAgainstPublicKey(t *testing.T) {
	fake := newTestRelease(t, SignedByOtherKey())

	checksums := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt.sig")

	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(fake.PublicKey()))
	require.NoError(t, err)

	_, err = openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(string(checksums)), strings.NewReader(string(signature)), nil)
	require.Error(t, err)
}

func TestFakeReleaseSignedByOtherKeyVerifiesAgainstOtherPublicKey(t *testing.T) {
	fake := newTestRelease(t, SignedByOtherKey())

	checksums := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt.sig")

	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(fake.OtherPublicKey()))
	require.NoError(t, err)

	_, err = openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(string(checksums)), strings.NewReader(string(signature)), nil)
	require.NoError(t, err)
}

func TestFakeReleaseUntamperedArchiveMatchesChecksum(t *testing.T) {
	fake := newTestRelease(t)

	checksums := string(download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt"))

	sum := sha256.Sum256(fake.Archive("linux", "arm64"))
	line := checksumLine(t, checksums, "xcl-plugin-widget_1.0.0_linux_arm64.tar.gz")

	require.True(t, strings.HasPrefix(line, hex.EncodeToString(sum[:])))
}

func TestFakeReleaseTamperArchiveDiffersFromChecksum(t *testing.T) {
	fake := newTestRelease(t, TamperArchive("linux", "arm64"))

	checksums := string(download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt"))

	served := download(t, fake, "xcl-plugin-widget_1.0.0_linux_arm64.tar.gz")
	sum := sha256.Sum256(served)
	line := checksumLine(t, checksums, "xcl-plugin-widget_1.0.0_linux_arm64.tar.gz")

	require.False(t, strings.HasPrefix(line, hex.EncodeToString(sum[:])))
}

func TestFakeReleaseTamperArchiveLeavesOtherArchivesIntact(t *testing.T) {
	fake := newTestRelease(t, TamperArchive("linux", "arm64"))

	checksums := string(download(t, fake, "xcl-plugin-widget_1.0.0_checksums.txt"))

	sum := sha256.Sum256(fake.Archive("linux", "amd64"))
	line := checksumLine(t, checksums, "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz")

	require.True(t, strings.HasPrefix(line, hex.EncodeToString(sum[:])))
}

func TestFakeReleaseRequireTokenRejectsRequestWithoutToken(t *testing.T) {
	fake := newTestRelease(t, RequireToken("s3cret"))

	status, _ := get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "", "")

	require.Equal(t, http.StatusNotFound, status)
}

func TestFakeReleaseRequireTokenRejectsWrongToken(t *testing.T) {
	fake := newTestRelease(t, RequireToken("s3cret"))

	status, _ := get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "wrong", "")

	require.Equal(t, http.StatusNotFound, status)
}

func TestFakeReleaseRequireTokenAcceptsBearerToken(t *testing.T) {
	fake := newTestRelease(t, RequireToken("s3cret"))

	status, body := get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "s3cret", "")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(body), `"tag_name":"v1.0.0"`)
}

func TestFakeReleaseRequireTokenGuardsAssetDownload(t *testing.T) {
	fake := newTestRelease(t, RequireToken("s3cret"))

	status, body := get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "s3cret", "")
	require.Equal(t, http.StatusOK, status)

	var release testRelease
	require.NoError(t, json.Unmarshal(body, &release))

	url := release.assetURL(t, fake.ArchiveName("linux", "amd64"))

	status, _ = get(t, url, "", "application/octet-stream")
	require.Equal(t, http.StatusNotFound, status)

	status, body = get(t, url, "s3cret", "application/octet-stream")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, fake.Archive("linux", "amd64"), body)
}

func TestFakeReleaseLastTokenRecordsToken(t *testing.T) {
	fake := newTestRelease(t, RequireToken("s3cret"))

	get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "s3cret", "")

	require.Equal(t, "s3cret", fake.LastToken())
}

func TestFakeReleaseLastTokenEmptyWithoutToken(t *testing.T) {
	fake := newTestRelease(t)

	require.Equal(t, "", fake.LastToken())

	get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "", "")

	require.Equal(t, "", fake.LastToken())
}

func TestFakeReleaseDraftIsFalseByDefault(t *testing.T) {
	fake := newTestRelease(t)

	require.False(t, fetchRelease(t, fake).Draft)
}

func TestFakeReleaseDraftSetsDraft(t *testing.T) {
	fake := newTestRelease(t, Draft())

	require.True(t, fetchRelease(t, fake).Draft)
}

func TestFakeReleaseUnknownTagIsNotFound(t *testing.T) {
	fake := newTestRelease(t)

	status, _ := get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v9.9.9", "", "")

	require.Equal(t, http.StatusNotFound, status)
}

func TestFakeReleaseUnknownRepositoryIsNotFound(t *testing.T) {
	fake := newTestRelease(t)

	status, _ := get(t, fake.URL()+"/repos/acme/other/releases/tags/v1.0.0", "", "")

	require.Equal(t, http.StatusNotFound, status)
}

func TestFakeReleaseRequestsCountsRequests(t *testing.T) {
	fake := newTestRelease(t)
	require.Equal(t, 0, fake.Requests())

	get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0", "", "")
	get(t, fake.URL()+"/repos/acme/xcl-plugin-widget/releases/tags/v9.9.9", "", "")

	require.Equal(t, 2, fake.Requests())
}

func TestFakeReleaseRequestsFailAfterClose(t *testing.T) {
	fake := newTestRelease(t)
	url := fake.URL() + "/repos/acme/xcl-plugin-widget/releases/tags/v1.0.0"

	fake.Close()

	_, err := http.Get(url)
	require.Error(t, err)
}

func TestBuildFixturePluginBuildsNamedBinary(t *testing.T) {
	path := BuildFixturePlugin(t, "../..")

	require.Equal(t, "xcl-plugin-widget", filepath.Base(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, info.IsDir())
}
