package testutil

import (
	"archive/zip"
	"bytes"
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

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

const (
	testRepository = "acme/xcl-plugin-widget"
	testTag        = "v1.0.0"
	testBinary     = "not really a plugin"
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
	require.NoError(t, os.WriteFile(binary, []byte(testBinary), 0755))

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

func downloadIndex(t *testing.T, fake *FakeGitHubRelease) string {
	t.Helper()

	return string(download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl"))
}

// platformBlock is the text of the index's platform block for os/arch, from
// its opening line to its closing brace
func platformBlock(t *testing.T, index, os, arch string) string {
	t.Helper()

	opening := "  platform \"" + os + "\" \"" + arch + "\" {\n"

	start := strings.Index(index, opening)
	require.NotEqual(t, -1, start, "no platform block for %s/%s in\n%s", os, arch, index)

	end := strings.Index(index[start:], "\n  }\n")
	require.NotEqual(t, -1, end, "unterminated platform block for %s/%s", os, arch)

	return index[start : start+end+len("\n  }\n")]
}

func sha256Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func readZip(t *testing.T, data []byte) *zip.Reader {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	return reader
}

func zipEntry(t *testing.T, data []byte, name string) (*zip.File, []byte) {
	t.Helper()

	for _, file := range readZip(t, data).File {
		if file.Name != name {
			continue
		}

		reader, err := file.Open()
		require.NoError(t, err)

		defer reader.Close()

		contents, err := io.ReadAll(reader)
		require.NoError(t, err)

		return file, contents
	}

	require.FailNow(t, "zip entry not found", name)

	return nil, nil
}

func zipManifest(t *testing.T, fake *FakeGitHubRelease, os, arch string) string {
	t.Helper()

	_, manifest := zipEntry(t, download(t, fake, fake.ZipName(os, arch)), "xcl-plugin.xcl")

	return string(manifest)
}

func verifySignature(t *testing.T, armoredKey string, index, signature []byte) error {
	t.Helper()

	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armoredKey))
	require.NoError(t, err)

	_, err = openpgp.CheckArmoredDetachedSignature(keyring, bytes.NewReader(index), bytes.NewReader(signature), nil)

	return err
}

func TestFakeReleaseListsAllAssets(t *testing.T) {
	fake := newTestRelease(t)

	release := fetchRelease(t, fake)

	require.Equal(t, "v1.0.0", release.TagName)
	require.ElementsMatch(t, []string{
		"xcl-plugin-widget_1.0.0_linux_amd64.zip",
		"xcl-plugin-widget_1.0.0_linux_arm64.zip",
		"xcl-plugin-widget_1.0.0_darwin_amd64.zip",
		"xcl-plugin-widget_1.0.0_darwin_arm64.zip",
		"xcl-plugin-widget_1.0.0_windows_amd64.zip",
		"xcl-plugin-widget_1.0.0_windows_arm64.zip",
		"xcl-plugin-widget_1.0.0_index.xcl",
		"xcl-plugin-widget_1.0.0_index.xcl.sig",
	}, release.names())
}

func TestFakeReleaseNamesMatchContract(t *testing.T) {
	fake := newTestRelease(t)

	require.Equal(t, "xcl-plugin-widget_1.0.0_linux_amd64.zip", fake.ZipName("linux", "amd64"))
	require.Equal(t, "xcl-plugin-widget_1.0.0_windows_arm64.zip", fake.ZipName("windows", "arm64"))
	require.Equal(t, "xcl-plugin-widget_1.0.0_index.xcl", fake.IndexName())
	require.Equal(t, "xcl-plugin-widget_1.0.0_index.xcl.sig", fake.SignatureName())
}

func TestFakeReleaseServesZipBytes(t *testing.T) {
	fake := newTestRelease(t)

	body := download(t, fake, "xcl-plugin-widget_1.0.0_linux_amd64.zip")

	require.NotEmpty(t, body)
	require.Equal(t, fake.Zip("linux", "amd64"), body)
}

func TestFakeReleaseServesIndexBytes(t *testing.T) {
	fake := newTestRelease(t)

	body := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl")

	require.NotEmpty(t, body)
	require.Equal(t, fake.Index(), body)
}

func TestFakeReleaseAssetWithoutOctetStreamAcceptIsMetadata(t *testing.T) {
	fake := newTestRelease(t)
	release := fetchRelease(t, fake)

	status, body := get(t, release.assetURL(t, fake.IndexName()), "", "")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(body), `"name":"xcl-plugin-widget_1.0.0_index.xcl"`)
}

func TestFakeReleaseIndexIsValidXcl(t *testing.T) {
	fake := newTestRelease(t, WithUnknownKind(), WithContainer("ghcr.io/acme/widget@sha256:0000000000000000000000000000000000000000000000000000000000000000", "linux/amd64", "linux/arm64"))

	_, diagnostics := hclsyntax.ParseConfig(fake.Index(), fake.IndexName(), hcl.InitialPos)

	require.False(t, diagnostics.HasErrors(), diagnostics.Error())
}

func TestFakeReleaseIndexNamesPluginAndVersion(t *testing.T) {
	fake := newTestRelease(t)

	index := downloadIndex(t, fake)

	require.True(t, strings.HasPrefix(index, "plugin {\n  name    = \"xcl-plugin-widget\"\n  version = \"1.0.0\"\n}\n"), index)
}

func TestFakeReleaseIndexListsExecutableProtocolOne(t *testing.T) {
	fake := newTestRelease(t)

	index := downloadIndex(t, fake)

	require.Contains(t, index, "\nexecutable {\n  protocol = [1]\n")
}

func TestFakeReleaseIndexListsEveryPlatform(t *testing.T) {
	fake := newTestRelease(t)

	index := downloadIndex(t, fake)

	for _, platform := range FakeReleasePlatforms {
		os, arch, _ := strings.Cut(platform, "/")
		require.Contains(t, platformBlock(t, index, os, arch), "asset    = \"xcl-plugin-widget_1.0.0_"+os+"_"+arch+".zip\"")
	}
}

func TestFakeReleaseIndexChecksumMatchesZip(t *testing.T) {
	fake := newTestRelease(t)

	index := downloadIndex(t, fake)
	zip := download(t, fake, "xcl-plugin-widget_1.0.0_linux_amd64.zip")

	require.Contains(t, platformBlock(t, index, "linux", "amd64"), "checksum = \""+sha256Checksum(zip)+"\"")
}

func TestFakeReleaseIndexChecksumsMatchEveryZip(t *testing.T) {
	fake := newTestRelease(t)

	index := downloadIndex(t, fake)

	for _, platform := range FakeReleasePlatforms {
		os, arch, _ := strings.Cut(platform, "/")
		require.Contains(t, platformBlock(t, index, os, arch), "checksum = \""+sha256Checksum(fake.Zip(os, arch))+"\"")
	}
}

func TestFakeReleaseIndexHasNoContainerByDefault(t *testing.T) {
	fake := newTestRelease(t)

	require.NotContains(t, downloadIndex(t, fake), "container {")
}

func TestFakeReleaseZipHoldsBinaryManifestAndReadme(t *testing.T) {
	fake := newTestRelease(t)

	names := []string{}
	for _, file := range readZip(t, fake.Zip("linux", "amd64")).File {
		names = append(names, file.Name)
	}

	require.ElementsMatch(t, []string{"xcl-plugin-widget", "xcl-plugin.xcl", "README.md"}, names)
}

func TestFakeReleaseZipBinaryIsExecutable(t *testing.T) {
	fake := newTestRelease(t)

	file, contents := zipEntry(t, fake.Zip("linux", "amd64"), "xcl-plugin-widget")

	require.Equal(t, testBinary, string(contents))
	require.Equal(t, os.FileMode(0755), file.Mode().Perm())
}

func TestFakeReleaseWindowsZipBinaryEndsInExe(t *testing.T) {
	fake := newTestRelease(t)

	_, contents := zipEntry(t, fake.Zip("windows", "amd64"), "xcl-plugin-widget.exe")

	require.Equal(t, testBinary, string(contents))
}

func TestFakeReleaseZipManifestDescribesBuild(t *testing.T) {
	fake := newTestRelease(t)

	expected := `plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`

	require.Equal(t, expected, zipManifest(t, fake, "linux", "amd64"))
}

func TestFakeReleaseWindowsZipManifestCommandEndsInExe(t *testing.T) {
	fake := newTestRelease(t)

	manifest := zipManifest(t, fake, "windows", "arm64")

	require.Contains(t, manifest, "command  = [\"xcl-plugin-widget.exe\"]")
	require.Contains(t, manifest, "platform \"windows\" \"arm64\" {}")
}

func TestFakeReleaseSignatureVerifiesAgainstPublicKey(t *testing.T) {
	fake := newTestRelease(t)

	index := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl.sig")

	require.NoError(t, verifySignature(t, fake.PublicKey(), index, signature))
}

func TestFakeReleaseSignatureDoesNotVerifyAgainstOtherKey(t *testing.T) {
	fake := newTestRelease(t)

	index := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl.sig")

	require.Error(t, verifySignature(t, fake.OtherPublicKey(), index, signature))
}

func TestFakeReleaseSignedByOtherKeyDoesNotVerifyAgainstPublicKey(t *testing.T) {
	fake := newTestRelease(t, SignedByOtherKey())

	index := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl.sig")

	require.Error(t, verifySignature(t, fake.PublicKey(), index, signature))
}

func TestFakeReleaseSignedByOtherKeyVerifiesAgainstOtherPublicKey(t *testing.T) {
	fake := newTestRelease(t, SignedByOtherKey())

	index := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl")
	signature := download(t, fake, "xcl-plugin-widget_1.0.0_index.xcl.sig")

	require.NoError(t, verifySignature(t, fake.OtherPublicKey(), index, signature))
}

func TestFakeReleaseWithoutPlatformOmitsZip(t *testing.T) {
	fake := newTestRelease(t, WithoutPlatform("darwin", "arm64"))

	release := fetchRelease(t, fake)

	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_darwin_arm64.zip")
	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_darwin_amd64.zip")
	require.Nil(t, fake.Zip("darwin", "arm64"))
	require.Len(t, release.Assets, 7)
}

func TestFakeReleaseWithoutPlatformOmitsIndexPlatform(t *testing.T) {
	fake := newTestRelease(t, WithoutPlatform("darwin", "arm64"))

	index := downloadIndex(t, fake)

	require.NotContains(t, index, `platform "darwin" "arm64"`)
	require.Contains(t, index, `platform "darwin" "amd64"`)
}

func TestFakeReleaseWithoutExecutableOmitsZips(t *testing.T) {
	fake := newTestRelease(t, WithoutExecutable())

	release := fetchRelease(t, fake)

	require.ElementsMatch(t, []string{
		"xcl-plugin-widget_1.0.0_index.xcl",
		"xcl-plugin-widget_1.0.0_index.xcl.sig",
	}, release.names())
}

func TestFakeReleaseWithoutExecutableOmitsExecutableBlock(t *testing.T) {
	fake := newTestRelease(t, WithoutExecutable())

	index := downloadIndex(t, fake)

	require.NotContains(t, index, "executable {")
	require.Contains(t, index, "plugin {")
}

func TestFakeReleaseWithoutIndexOmitsIndexAndSignature(t *testing.T) {
	fake := newTestRelease(t, WithoutIndex())

	release := fetchRelease(t, fake)

	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_index.xcl")
	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_index.xcl.sig")
	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_linux_amd64.zip")
	require.Len(t, release.Assets, 6)
	require.Nil(t, fake.Index())
}

func TestFakeReleaseWithoutSignatureOmitsOnlySignature(t *testing.T) {
	fake := newTestRelease(t, WithoutSignature())

	release := fetchRelease(t, fake)

	require.NotContains(t, release.names(), "xcl-plugin-widget_1.0.0_index.xcl.sig")
	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_index.xcl")
	require.Len(t, release.Assets, 7)
}

func TestFakeReleaseWithoutChecksumOmitsChecksumForPlatform(t *testing.T) {
	fake := newTestRelease(t, WithoutChecksum("linux", "arm64"))

	block := platformBlock(t, downloadIndex(t, fake), "linux", "arm64")

	require.NotContains(t, block, "checksum")
	require.Contains(t, block, "asset    = \"xcl-plugin-widget_1.0.0_linux_arm64.zip\"")
}

func TestFakeReleaseWithoutChecksumKeepsOtherChecksums(t *testing.T) {
	fake := newTestRelease(t, WithoutChecksum("linux", "arm64"))

	block := platformBlock(t, downloadIndex(t, fake), "linux", "amd64")

	require.Contains(t, block, "checksum = \""+sha256Checksum(fake.Zip("linux", "amd64"))+"\"")
}

func TestFakeReleaseWithoutChecksumStillServesZip(t *testing.T) {
	fake := newTestRelease(t, WithoutChecksum("linux", "arm64"))

	require.Contains(t, fetchRelease(t, fake).names(), "xcl-plugin-widget_1.0.0_linux_arm64.zip")
}

func TestFakeReleaseTamperZipDiffersFromChecksum(t *testing.T) {
	fake := newTestRelease(t, TamperZip("linux", "arm64"))

	served := download(t, fake, "xcl-plugin-widget_1.0.0_linux_arm64.zip")
	block := platformBlock(t, downloadIndex(t, fake), "linux", "arm64")

	require.Contains(t, block, "checksum = \"sha256:")
	require.NotContains(t, block, sha256Checksum(served))
}

func TestFakeReleaseTamperZipLeavesOtherZipsIntact(t *testing.T) {
	fake := newTestRelease(t, TamperZip("linux", "arm64"))

	block := platformBlock(t, downloadIndex(t, fake), "linux", "amd64")

	require.Contains(t, block, "checksum = \""+sha256Checksum(fake.Zip("linux", "amd64"))+"\"")
}

func TestFakeReleaseManifestDisagreesOnName(t *testing.T) {
	fake := newTestRelease(t, ManifestDisagrees("linux", "amd64", ManifestName))

	manifest := zipManifest(t, fake, "linux", "amd64")

	require.Contains(t, manifest, "name    = \"xcl-plugin-widget-other\"")
}

func TestFakeReleaseManifestDisagreesOnVersion(t *testing.T) {
	fake := newTestRelease(t, ManifestDisagrees("linux", "amd64", ManifestVersion))

	manifest := zipManifest(t, fake, "linux", "amd64")

	require.Contains(t, manifest, "version = \"1.0.0-other\"")
}

func TestFakeReleaseManifestDisagreesOnProtocol(t *testing.T) {
	fake := newTestRelease(t, ManifestDisagrees("linux", "amd64", ManifestProtocol))

	manifest := zipManifest(t, fake, "linux", "amd64")

	require.Contains(t, manifest, "protocol = [2]")
}

func TestFakeReleaseManifestDisagreesOnPlatform(t *testing.T) {
	fake := newTestRelease(t, ManifestDisagrees("linux", "amd64", ManifestPlatform))

	manifest := zipManifest(t, fake, "linux", "amd64")

	require.Contains(t, manifest, "platform \"darwin\" \"arm64\" {}")
	require.NotContains(t, manifest, "platform \"linux\" \"amd64\"")
}

func TestFakeReleaseManifestDisagreesKeepsZipMatchingChecksum(t *testing.T) {
	fake := newTestRelease(t, ManifestDisagrees("linux", "amd64", ManifestName))

	block := platformBlock(t, downloadIndex(t, fake), "linux", "amd64")

	require.Contains(t, block, "checksum = \""+sha256Checksum(fake.Zip("linux", "amd64"))+"\"")
}

func TestFakeReleaseManifestDisagreesLeavesOtherManifestsAgreeing(t *testing.T) {
	fake := newTestRelease(t, ManifestDisagrees("linux", "amd64", ManifestName))

	manifest := zipManifest(t, fake, "linux", "arm64")

	require.Contains(t, manifest, "name    = \"xcl-plugin-widget\"\n")
}

func TestFakeReleaseWithUnknownKindAddsBlock(t *testing.T) {
	fake := newTestRelease(t, WithUnknownKind())

	require.Contains(t, downloadIndex(t, fake), "\nwasm {\n  protocol = [3]\n}\n")
}

func TestFakeReleaseWithoutUnknownKindHasNoBlock(t *testing.T) {
	fake := newTestRelease(t)

	require.NotContains(t, downloadIndex(t, fake), "wasm")
}

func TestFakeReleaseWithContainerAddsContainerBlock(t *testing.T) {
	image := "ghcr.io/acme/widget@sha256:1111111111111111111111111111111111111111111111111111111111111111"
	fake := newTestRelease(t, WithContainer(image, "linux/amd64", "linux/arm64"))

	expected := `
container {
  protocol = [1]
  image    = "ghcr.io/acme/widget@sha256:1111111111111111111111111111111111111111111111111111111111111111"

  platform "linux" "amd64" {}

  platform "linux" "arm64" {}
}
`

	require.Contains(t, downloadIndex(t, fake), expected)
}

func TestFakeReleaseWithContainerWritesImageAndPlatformsAsGiven(t *testing.T) {
	fake := newTestRelease(t, WithContainer("ghcr.io/acme/widget:latest", "darwin/arm64"))

	index := downloadIndex(t, fake)

	require.Contains(t, index, "image    = \"ghcr.io/acme/widget:latest\"")
	require.Contains(t, index, "platform \"darwin\" \"arm64\" {}")
}

func TestFakeReleaseWithContainerKeepsExecutable(t *testing.T) {
	fake := newTestRelease(t, WithContainer("ghcr.io/acme/widget@sha256:1111111111111111111111111111111111111111111111111111111111111111", "linux/amd64"))

	require.Len(t, fetchRelease(t, fake).Assets, 8)
	require.Contains(t, downloadIndex(t, fake), "executable {")
}

func TestFakeReleaseIndexExpressionWritesFunctionCall(t *testing.T) {
	fake := newTestRelease(t, IndexExpression())

	require.Contains(t, downloadIndex(t, fake), "version = upper(\"1.0.0\")")
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

	url := release.assetURL(t, fake.ZipName("linux", "amd64"))

	status, _ = get(t, url, "", "application/octet-stream")
	require.Equal(t, http.StatusNotFound, status)

	status, body = get(t, url, "s3cret", "application/octet-stream")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, fake.Zip("linux", "amd64"), body)
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

func TestFakeReleaseWithHyphenSeparatorNamesEveryAssetWithHyphens(t *testing.T) {
	fake := newTestRelease(t, WithSeparator("-"))

	release := fetchRelease(t, fake)

	require.ElementsMatch(t, []string{
		"xcl-plugin-widget-1.0.0-linux-amd64.zip",
		"xcl-plugin-widget-1.0.0-linux-arm64.zip",
		"xcl-plugin-widget-1.0.0-darwin-amd64.zip",
		"xcl-plugin-widget-1.0.0-darwin-arm64.zip",
		"xcl-plugin-widget-1.0.0-windows-amd64.zip",
		"xcl-plugin-widget-1.0.0-windows-arm64.zip",
		"xcl-plugin-widget-1.0.0-index.xcl",
		"xcl-plugin-widget-1.0.0-index.xcl.sig",
	}, release.names())
}

func TestFakeReleaseWithHyphenSeparatorNamesMatchContract(t *testing.T) {
	fake := newTestRelease(t, WithSeparator("-"))

	require.Equal(t, "xcl-plugin-widget-1.0.0-linux-amd64.zip", fake.ZipName("linux", "amd64"))
	require.Equal(t, "xcl-plugin-widget-1.0.0-index.xcl", fake.IndexName())
	require.Equal(t, "xcl-plugin-widget-1.0.0-index.xcl.sig", fake.SignatureName())
}

func TestFakeReleaseWithHyphenSeparatorIndexNamesHyphenatedZips(t *testing.T) {
	fake := newTestRelease(t, WithSeparator("-"))

	index := string(download(t, fake, fake.IndexName()))

	require.Contains(t, index, `"xcl-plugin-widget-1.0.0-linux-amd64.zip"`)
	require.NotContains(t, index, `"xcl-plugin-widget_1.0.0_linux_amd64.zip"`)
}

func TestFakeReleaseWithBothIndexesAttachesTheIndexUnderBothNames(t *testing.T) {
	fake := newTestRelease(t, WithBothIndexes())

	release := fetchRelease(t, fake)

	require.Contains(t, release.names(), "xcl-plugin-widget_1.0.0_index.xcl")
	require.Contains(t, release.names(), "xcl-plugin-widget-1.0.0-index.xcl")
	require.Equal(t, fake.Index(), download(t, fake, "xcl-plugin-widget-1.0.0-index.xcl"))
}
