package testutil

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/stretchr/testify/require"
)

// FakeReleasePlatforms are the platforms a plugin release publishes a build
// for, as os/arch
var FakeReleasePlatforms = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"windows/amd64",
	"windows/arm64",
}

// FakeGitHubRelease is one plugin release laid out by the plugin release
// asset contract, served by a local HTTP server that answers like GitHub's
// release API. It encodes the contract's names itself, rather than asking the
// registry for them, so a test catches the registry drifting from the
// contract.
type FakeGitHubRelease struct {
	repository string
	tag        string
	settings   fakeReleaseSettings

	key      *openpgp.Entity
	otherKey *openpgp.Entity

	assets   []fakeAsset
	archives map[string][]byte

	server   *httptest.Server
	requests atomic.Int64

	mu     sync.Mutex
	tokens []string
}

// fakeAsset is one file attached to the fake release
type fakeAsset struct {
	id   int64
	name string
	data []byte
}

// fakeReleaseSettings are what the options change about a fake release
type fakeReleaseSettings struct {
	withoutPlatforms map[string]bool
	tampered         map[string]bool
	withoutChecksums bool
	withoutSignature bool
	signedByOther    bool
	token            string
	draft            bool
}

// FakeReleaseOption changes how a fake release is built or served
type FakeReleaseOption func(*fakeReleaseSettings)

// WithoutPlatform leaves the build for os/arch out of the release
func WithoutPlatform(os, arch string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutPlatforms[os+"/"+arch] = true }
}

// WithoutChecksums leaves the checksums file, and so its signature, out of
// the release
func WithoutChecksums() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutChecksums = true }
}

// WithoutSignature leaves the signature over the checksums file out of the
// release
func WithoutSignature() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutSignature = true }
}

// SignedByOtherKey signs the checksums file with a key other than the one
// PublicKey returns, the one OtherPublicKey returns
func SignedByOtherKey() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.signedByOther = true }
}

// TamperArchive changes the build for os/arch after its checksum was taken,
// so it no longer matches the checksums file
func TamperArchive(os, arch string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.tampered[os+"/"+arch] = true }
}

// RequireToken serves the release only to requests carrying token, as
// "Bearer <token>" or "token <token>" in the Authorization header. Any other
// request gets a 404, as GitHub answers for a private repository.
func RequireToken(token string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.token = token }
}

// Draft marks the release as a draft
func Draft() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.draft = true }
}

// NewFakeGitHubRelease builds a release of repository (owner/repo) at tag
// from the plugin binary at binary, packed for every platform in
// FakeReleasePlatforms, with a checksums file and a signature made with a
// key generated for the test, and serves it until the test ends. Every
// platform's archive holds the same binary, so only the host platform's
// build can be started.
func NewFakeGitHubRelease(t testing.TB, repository, tag, binary string, options ...FakeReleaseOption) *FakeGitHubRelease {
	t.Helper()

	settings := fakeReleaseSettings{withoutPlatforms: map[string]bool{}, tampered: map[string]bool{}}
	for _, option := range options {
		option(&settings)
	}

	contents, err := os.ReadFile(binary)
	require.NoError(t, err)

	f := &FakeGitHubRelease{
		repository: repository,
		tag:        tag,
		settings:   settings,
		key:        newFakeSigningKey(t, "xcl plugin release"),
		otherKey:   newFakeSigningKey(t, "someone else"),
		archives:   map[string][]byte{},
	}

	f.build(t, contents)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/{owner}/{repo}/releases/tags/{tag}", f.serveRelease)
	mux.HandleFunc("GET /repos/{owner}/{repo}/releases/assets/{id}", f.serveAsset)

	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)

		f.mu.Lock()
		f.tokens = append(f.tokens, requestToken(r))
		f.mu.Unlock()

		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)

	return f
}

// URL is the base URL of the fake GitHub API
func (f *FakeGitHubRelease) URL() string {
	return f.server.URL
}

// Close stops the server, later requests fail to connect
func (f *FakeGitHubRelease) Close() {
	f.server.Close()
}

// Requests is how many requests the server has received
func (f *FakeGitHubRelease) Requests() int {
	return int(f.requests.Load())
}

// LastToken is the token the most recent request carried, empty when it
// carried none or no request was made
func (f *FakeGitHubRelease) LastToken() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.tokens) == 0 {
		return ""
	}

	return f.tokens[len(f.tokens)-1]
}

// PublicKey is the ASCII-armoured public key the release is signed with
func (f *FakeGitHubRelease) PublicKey() string {
	return armoredPublicKey(f.key)
}

// OtherPublicKey is the ASCII-armoured public key of a key that did not sign
// the release, unless the release was built with SignedByOtherKey
func (f *FakeGitHubRelease) OtherPublicKey() string {
	return armoredPublicKey(f.otherKey)
}

// Archive is the build for os/arch as it is served, nil when the release has
// none
func (f *FakeGitHubRelease) Archive(os, arch string) []byte {
	return f.archives[os+"/"+arch]
}

// ArchiveName is the contract's name for the build for os/arch
func (f *FakeGitHubRelease) ArchiveName(os, arch string) string {
	name := fmt.Sprintf("%s_%s_%s_%s.tar.gz", f.pluginName(), f.version(), os, arch)
	if os == "windows" {
		name = fmt.Sprintf("%s_%s_%s_%s.zip", f.pluginName(), f.version(), os, arch)
	}

	return name
}

// ChecksumsName is the contract's name for the release's checksums file
func (f *FakeGitHubRelease) ChecksumsName() string {
	return fmt.Sprintf("%s_%s_checksums.txt", f.pluginName(), f.version())
}

// SignatureName is the contract's name for the signature over the checksums
// file
func (f *FakeGitHubRelease) SignatureName() string {
	return f.ChecksumsName() + ".sig"
}

// pluginName is the repository's name, the contract's plugin name
func (f *FakeGitHubRelease) pluginName() string {
	_, name, _ := strings.Cut(f.repository, "/")
	return name
}

// version is the tag without its leading v
func (f *FakeGitHubRelease) version() string {
	return strings.TrimPrefix(f.tag, "v")
}

// build packs the binary for every platform and writes the checksums file
// and the signature
func (f *FakeGitHubRelease) build(t testing.TB, binary []byte) {
	t.Helper()

	var checksums strings.Builder

	for _, platform := range FakeReleasePlatforms {
		if f.settings.withoutPlatforms[platform] {
			continue
		}

		os, arch, _ := strings.Cut(platform, "/")

		var archive []byte
		if os == "windows" {
			archive = zipArchive(t, f.pluginName()+".exe", binary)
		} else {
			archive = tarGzArchive(t, f.pluginName(), binary)
		}

		sum := sha256.Sum256(archive)
		fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(sum[:]), f.ArchiveName(os, arch))

		if f.settings.tampered[platform] {
			archive = append(append([]byte{}, archive...), []byte("tampered")...)
		}

		f.archives[platform] = archive
		f.addAsset(f.ArchiveName(os, arch), archive)
	}

	if f.settings.withoutChecksums {
		return
	}

	f.addAsset(f.ChecksumsName(), []byte(checksums.String()))

	if f.settings.withoutSignature {
		return
	}

	signer := f.key
	if f.settings.signedByOther {
		signer = f.otherKey
	}

	var signature bytes.Buffer
	err := openpgp.ArmoredDetachSign(&signature, signer, strings.NewReader(checksums.String()), nil)
	require.NoError(t, err)

	f.addAsset(f.SignatureName(), signature.Bytes())
}

// addAsset attaches a file to the release
func (f *FakeGitHubRelease) addAsset(name string, data []byte) {
	f.assets = append(f.assets, fakeAsset{id: int64(len(f.assets) + 1), name: name, data: data})
}

// serveRelease answers GET /repos/{owner}/{repo}/releases/tags/{tag}
func (f *FakeGitHubRelease) serveRelease(w http.ResponseWriter, r *http.Request) {
	if !f.allowed(r) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		return
	}

	type asset struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	release := struct {
		TagName string  `json:"tag_name"`
		Draft   bool    `json:"draft"`
		Assets  []asset `json:"assets"`
	}{TagName: f.tag, Draft: f.settings.draft, Assets: []asset{}}

	for _, a := range f.assets {
		release.Assets = append(release.Assets, asset{
			ID:   a.id,
			Name: a.name,
			URL:  fmt.Sprintf("%s/repos/%s/releases/assets/%d", f.server.URL, f.repository, a.id),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(release)
}

// serveAsset answers GET /repos/{owner}/{repo}/releases/assets/{id}, with the
// asset's bytes when the request accepts application/octet-stream
func (f *FakeGitHubRelease) serveAsset(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("owner")+"/"+r.PathValue("repo") != f.repository || !f.authorized(r) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		return
	}

	for _, a := range f.assets {
		if a.id != id {
			continue
		}

		if r.Header.Get("Accept") != "application/octet-stream" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"id":%d,"name":%q}`, a.id, a.name)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(a.data)
		return
	}

	http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
}

// allowed reports whether r asks for this release and may see it
func (f *FakeGitHubRelease) allowed(r *http.Request) bool {
	return r.PathValue("owner")+"/"+r.PathValue("repo") == f.repository &&
		r.PathValue("tag") == f.tag &&
		f.authorized(r)
}

// authorized reports whether r carries the token the release requires, if
// it requires one
func (f *FakeGitHubRelease) authorized(r *http.Request) bool {
	return f.settings.token == "" || requestToken(r) == f.settings.token
}

// requestToken is the token in r's Authorization header, empty without one
func requestToken(r *http.Request) string {
	header := r.Header.Get("Authorization")

	if token, found := strings.CutPrefix(header, "Bearer "); found {
		return token
	}

	if token, found := strings.CutPrefix(header, "token "); found {
		return token
	}

	return ""
}

// newFakeSigningKey generates an OpenPGP key for signing a fake release
func newFakeSigningKey(t testing.TB, name string) *openpgp.Entity {
	t.Helper()

	entity, err := openpgp.NewEntity(name, "", "", nil)
	require.NoError(t, err)

	return entity
}

// armoredPublicKey returns entity's public key, ASCII-armoured
func armoredPublicKey(entity *openpgp.Entity) string {
	var out bytes.Buffer

	writer, err := armor.Encode(&out, openpgp.PublicKeyType, nil)
	if err != nil {
		panic(err)
	}

	if err := entity.Serialize(writer); err != nil {
		panic(err)
	}

	if err := writer.Close(); err != nil {
		panic(err)
	}

	return out.String()
}

// tarGzArchive packs binary at the root of a gzipped tar under name
func tarGzArchive(t testing.TB, name string, binary []byte) []byte {
	t.Helper()

	var out bytes.Buffer
	compressed := gzip.NewWriter(&out)
	archive := tar.NewWriter(compressed)

	err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(binary)), Typeflag: tar.TypeReg})
	require.NoError(t, err)

	_, err = archive.Write(binary)
	require.NoError(t, err)

	readme := []byte("# " + name + "\n")
	err = archive.WriteHeader(&tar.Header{Name: "README.md", Mode: 0644, Size: int64(len(readme)), Typeflag: tar.TypeReg})
	require.NoError(t, err)

	_, err = archive.Write(readme)
	require.NoError(t, err)

	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())

	return out.Bytes()
}

// zipArchive packs binary at the root of a zip under name
func zipArchive(t testing.TB, name string, binary []byte) []byte {
	t.Helper()

	var out bytes.Buffer
	archive := zip.NewWriter(&out)

	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0755)

	writer, err := archive.CreateHeader(header)
	require.NoError(t, err)

	_, err = writer.Write(binary)
	require.NoError(t, err)

	readme, err := archive.Create("README.md")
	require.NoError(t, err)

	_, err = readme.Write([]byte("# " + name + "\n"))
	require.NoError(t, err)

	require.NoError(t, archive.Close())

	return out.Bytes()
}
