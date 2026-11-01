package testutil

import (
	"archive/zip"
	"bytes"
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
	"time"

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

// The fields of a build's manifest that ManifestDisagrees can make disagree
// with the release index
const (
	ManifestName     = "name"
	ManifestVersion  = "version"
	ManifestProtocol = "protocol"
	ManifestPlatform = "platform"
)

// FakeManifestName is the name of the manifest at the root of every zip
const FakeManifestName = "xcl-plugin.xcl"

// FakeGitHubRelease is one plugin release laid out by version 2 of the plugin
// release asset contract (plugin-release-assets-v2.md), served by a local
// HTTP server that answers like GitHub's release API: a zip per platform,
// each holding the plugin binary and its manifest, an index carrying every
// zip's checksum, and a detached signature over the index. It encodes the
// contract's names and formats itself, rather than asking the registry for
// them, so a test catches the registry drifting from the contract.
type FakeGitHubRelease struct {
	repository string
	tag        string
	settings   fakeReleaseSettings

	key      *openpgp.Entity
	otherKey *openpgp.Entity

	assets []fakeAsset
	zips   map[string][]byte
	index  []byte

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

// fakeContainer is the container entry a release's index offers
type fakeContainer struct {
	image     string
	platforms []string
}

// fakeReleaseSettings are what the options change about a fake release
type fakeReleaseSettings struct {
	withoutPlatforms  map[string]bool
	withoutChecksums  map[string]bool
	tampered          map[string]bool
	disagreements     map[string]string
	withoutExecutable bool
	withoutIndex      bool
	withoutSignature  bool
	signedByOther     bool
	unknownKind       bool
	indexExpression   bool
	container         *fakeContainer
	token             string
	draft             bool
	separator         string
	bothIndexes       bool
}

// FakeReleaseOption changes how a fake release is built or served
type FakeReleaseOption func(*fakeReleaseSettings)

// WithoutPlatform leaves the build for os/arch out of the release: no zip is
// attached and the index lists no platform for it
func WithoutPlatform(os, arch string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutPlatforms[os+"/"+arch] = true }
}

// WithoutExecutable leaves the executable out of the release: no zip is
// attached and the index has no executable block. With WithContainer it
// makes a container-only release.
func WithoutExecutable() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutExecutable = true }
}

// WithoutIndex leaves the index, and so its signature, out of the release.
// The zips are still attached.
func WithoutIndex() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutIndex = true }
}

// WithoutChecksum leaves the checksum out of the index's platform entry for
// os/arch. The zip is still attached.
func WithoutChecksum(os, arch string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutChecksums[os+"/"+arch] = true }
}

// WithoutSignature leaves the signature over the index out of the release
func WithoutSignature() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.withoutSignature = true }
}

// SignedByOtherKey signs the index with a key other than the one PublicKey
// returns, the one OtherPublicKey returns
func SignedByOtherKey() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.signedByOther = true }
}

// TamperZip changes the zip for os/arch after its checksum was written to the
// index, so it no longer matches
func TamperZip(os, arch string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.tampered[os+"/"+arch] = true }
}

// ManifestDisagrees writes the manifest in the zip for os/arch so that field
// (ManifestName, ManifestVersion, ManifestProtocol or ManifestPlatform)
// disagrees with the index. The index and the zip's checksum are still
// consistent with each other.
func ManifestDisagrees(os, arch, field string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.disagreements[os+"/"+arch] = field }
}

// WithUnknownKind adds a top-level block of a package kind no registry knows,
// `wasm { protocol = [3] }`, to the index
func WithUnknownKind() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.unknownKind = true }
}

// WithContainer adds a container entry to the index naming image and listing
// platforms, each as os/arch. The image and platforms are written as given,
// so a test can offer a tag instead of a digest or a platform other than
// Linux.
func WithContainer(image string, platforms ...string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) {
		s.container = &fakeContainer{image: image, platforms: platforms}
	}
}

// IndexExpression writes the index's plugin version as a function call,
// `upper("<version>")`, instead of a literal
func IndexExpression() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.indexExpression = true }
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

// WithSeparator separates the parts of every asset's name with separator, _
// or -, instead of _: the index, its signature and the zips
func WithSeparator(separator string) FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.separator = separator }
}

// WithBothIndexes attaches the index a second time, under the name the other
// separator gives it, so the release holds both
// <name>_<version>_index.xcl and <name>-<version>-index.xcl
func WithBothIndexes() FakeReleaseOption {
	return func(s *fakeReleaseSettings) { s.bothIndexes = true }
}

// NewFakeGitHubRelease builds a release of repository (owner/repo) at tag
// from the plugin binary at binary, zipped with its manifest for every
// platform in FakeReleasePlatforms, with an index carrying every zip's
// checksum and a signature over the index made with a key generated for the
// test, and serves it until the test ends. Every platform's zip holds the
// same binary, so only the host platform's build can be started.
func NewFakeGitHubRelease(t testing.TB, repository, tag, binary string, options ...FakeReleaseOption) *FakeGitHubRelease {
	t.Helper()

	settings := fakeReleaseSettings{
		withoutPlatforms: map[string]bool{},
		withoutChecksums: map[string]bool{},
		tampered:         map[string]bool{},
		disagreements:    map[string]string{},
		separator:        "_",
	}
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
		zips:       map[string][]byte{},
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

// OtherKeyID is the long key ID, 16 upper-case hex digits, of the key
// OtherPublicKey returns, as it signs: its signing subkey when it has one,
// else its primary key
func (f *FakeGitHubRelease) OtherKeyID() string {
	key, ok := f.otherKey.SigningKey(time.Now())
	if !ok {
		return fmt.Sprintf("%016X", f.otherKey.PrimaryKey.KeyId)
	}

	return fmt.Sprintf("%016X", key.PublicKey.KeyId)
}

// IndexName is the contract's name for the release's index, its parts
// separated by the release's separator
func (f *FakeGitHubRelease) IndexName() string {
	return f.indexNameWith(f.settings.separator)
}

// SignatureName is the contract's name for the signature over the index
func (f *FakeGitHubRelease) SignatureName() string {
	return f.IndexName() + ".sig"
}

// ZipName is the contract's name for the zip of the build for os/arch, its
// parts separated by the release's separator
func (f *FakeGitHubRelease) ZipName(os, arch string) string {
	return strings.Join([]string{f.pluginName(), f.version(), os, arch}, f.settings.separator) + ".zip"
}

// indexNameWith is the contract's name for the release's index, its parts
// separated by separator
func (f *FakeGitHubRelease) indexNameWith(separator string) string {
	return strings.Join([]string{f.pluginName(), f.version(), "index.xcl"}, separator)
}

// Zip is the zip of the build for os/arch as it is served, nil when the
// release has none
func (f *FakeGitHubRelease) Zip(os, arch string) []byte {
	return f.zips[os+"/"+arch]
}

// Index is the release's index as it is served, nil when the release has
// none
func (f *FakeGitHubRelease) Index() []byte {
	return f.index
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

// build zips the binary with its manifest for every platform, then writes
// the index and the signature over it
func (f *FakeGitHubRelease) build(t testing.TB, binary []byte) {
	t.Helper()

	checksums := map[string]string{}

	if !f.settings.withoutExecutable {
		for _, platform := range FakeReleasePlatforms {
			if f.settings.withoutPlatforms[platform] {
				continue
			}

			os, arch, _ := strings.Cut(platform, "/")

			command := f.pluginName()
			if os == "windows" {
				command += ".exe"
			}

			manifest := f.manifest(command, os, arch, f.settings.disagreements[platform])
			archive := zipArchive(t, command, binary, manifest)

			sum := sha256.Sum256(archive)
			checksums[platform] = "sha256:" + hex.EncodeToString(sum[:])

			if f.settings.tampered[platform] {
				archive = append(append([]byte{}, archive...), []byte("tampered")...)
			}

			f.zips[platform] = archive
			f.addAsset(f.ZipName(os, arch), archive)
		}
	}

	if f.settings.withoutIndex {
		return
	}

	f.index = f.indexFile(checksums)
	f.addAsset(f.IndexName(), f.index)

	if f.settings.bothIndexes {
		other := "-"
		if f.settings.separator == "-" {
			other = "_"
		}

		f.addAsset(f.indexNameWith(other), f.index)
	}

	if f.settings.withoutSignature {
		return
	}

	signer := f.key
	if f.settings.signedByOther {
		signer = f.otherKey
	}

	var signature bytes.Buffer
	err := openpgp.ArmoredDetachSign(&signature, signer, bytes.NewReader(f.index), nil)
	require.NoError(t, err)

	f.addAsset(f.SignatureName(), signature.Bytes())
}

// manifest writes the xcl-plugin.xcl for the build of command for os/arch,
// with disagree, when set, naming the field that must not match the index
func (f *FakeGitHubRelease) manifest(command, os, arch, disagree string) []byte {
	name := f.pluginName()
	version := f.version()
	protocol := "1"
	platformOS, platformArch := os, arch

	switch disagree {
	case ManifestName:
		name += "-other"
	case ManifestVersion:
		version += "-other"
	case ManifestProtocol:
		protocol = "2"
	case ManifestPlatform:
		platformOS, platformArch = otherPlatform(os, arch)
	}

	var out strings.Builder

	fmt.Fprintf(&out, "plugin {\n  name    = %q\n  version = %q\n}\n\n", name, version)
	fmt.Fprintf(&out, "executable {\n  protocol = [%s]\n  command  = [%q]\n\n", protocol, command)
	fmt.Fprintf(&out, "  platform %q %q {}\n}\n", platformOS, platformArch)

	return []byte(out.String())
}

// indexFile writes the release's index, listing every zip with its checksum
// from checksums (keyed by os/arch) and any container or unknown kind the
// options add
func (f *FakeGitHubRelease) indexFile(checksums map[string]string) []byte {
	var out strings.Builder

	version := strconv.Quote(f.version())
	if f.settings.indexExpression {
		version = fmt.Sprintf("upper(%q)", f.version())
	}

	fmt.Fprintf(&out, "plugin {\n  name    = %q\n  version = %s\n}\n", f.pluginName(), version)

	if !f.settings.withoutExecutable {
		out.WriteString("\nexecutable {\n  protocol = [1]\n")

		for _, platform := range FakeReleasePlatforms {
			checksum, ok := checksums[platform]
			if !ok {
				continue
			}

			os, arch, _ := strings.Cut(platform, "/")

			fmt.Fprintf(&out, "\n  platform %q %q {\n    asset    = %q\n", os, arch, f.ZipName(os, arch))

			if !f.settings.withoutChecksums[platform] {
				fmt.Fprintf(&out, "    checksum = %q\n", checksum)
			}

			out.WriteString("  }\n")
		}

		out.WriteString("}\n")
	}

	if container := f.settings.container; container != nil {
		fmt.Fprintf(&out, "\ncontainer {\n  protocol = [1]\n  image    = %q\n", container.image)

		for _, platform := range container.platforms {
			os, arch, _ := strings.Cut(platform, "/")
			fmt.Fprintf(&out, "\n  platform %q %q {}\n", os, arch)
		}

		out.WriteString("}\n")
	}

	if f.settings.unknownKind {
		out.WriteString("\nwasm {\n  protocol = [3]\n}\n")
	}

	return []byte(out.String())
}

// otherPlatform is a supported platform other than os/arch
func otherPlatform(os, arch string) (string, string) {
	if os == "linux" && arch == "amd64" {
		return "darwin", "arm64"
	}

	return "linux", "amd64"
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

// zipArchive packs binary under command, marked executable, a README and the
// manifest at the root of a zip. Entries are stored without compression:
// every test builds a release of a plugin binary for every platform, and
// compressing it is what made the registry tests outlast go test's timeout
// under the race detector.
func zipArchive(t testing.TB, command string, binary, manifest []byte) []byte {
	t.Helper()

	var out bytes.Buffer
	archive := zip.NewWriter(&out)

	writeZipEntry(t, archive, command, 0755, binary)
	writeZipEntry(t, archive, FakeManifestName, 0644, manifest)
	writeZipEntry(t, archive, "README.md", 0644, []byte("# "+command+"\n"))

	require.NoError(t, archive.Close())

	return out.Bytes()
}

// writeZipEntry stores data in archive under name with mode
func writeZipEntry(t testing.TB, archive *zip.Writer, name string, mode os.FileMode, data []byte) {
	t.Helper()

	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(mode)

	writer, err := archive.CreateHeader(header)
	require.NoError(t, err)

	_, err = writer.Write(data)
	require.NoError(t, err)
}
