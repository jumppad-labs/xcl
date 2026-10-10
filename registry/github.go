package registry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/jumppad-labs/xcl/events"
)

// GitHubName is the Name of every GitHub registry
const GitHubName = "github.com"

// DefaultGitHubAPIURL is the GitHub REST API a GitHub registry reads releases
// from when no GitHubAPIURL is given
const DefaultGitHubAPIURL = "https://api.github.com"

// GitHub is a registry of plugins installed from GitHub releases laid out by
// the plugin release asset contract. Each plugin is declared by repository
// and exact release tag; when plugins load, it is downloaded for the current
// platform, verified against the release's checksums, kept in a cache and
// started as an external plugin. A cached plugin is used without contacting
// GitHub, and is verified again every time it is started.
//
// A GitHub registry wraps a local registry, which keeps its plugins and any Go
// types registered on it, so it can declare types and plugins in the same
// way.
type GitHub struct {
	mu         sync.Mutex
	local      *Local
	apiURL     string
	cacheDir   string
	httpClient *http.Client
	platform   platform

	// trustedKeys are the ASCII-armoured public keys given with
	// GitHubTrustedKeys, keys is them read when plugins load
	trustedKeys []string
	keys        openpgp.EntityList

	// token is the token given with GitHubToken, resolvedToken the token
	// requests carry, resolved when plugins load
	token         string
	resolvedToken string

	ctx context.Context
}

// GitHubOption configures a GitHub registry when it is created
type GitHubOption func(*GitHub)

// GitHubCacheDir sets the directory installed plugins are kept in. A leading
// ~/ is the home directory and environment variables are expanded. An empty
// dir keeps the default, plugins under the xcl home directory,
// ~/.xcl/cache/plugins.
func GitHubCacheDir(dir string) GitHubOption {
	return func(g *GitHub) {
		if dir != "" {
			g.cacheDir = expandPluginDirectories([]string{dir})[0]
		}
	}
}

// GitHubTrustedKeys adds ASCII-armoured OpenPGP public keys the registry
// trusts. When any are given, a plugin is used only when its release's
// checksums file carries a detached signature that verifies against one of
// them, checked when the plugin is installed and every time it is started.
// Without keys, signatures are not checked and unsigned releases install. A
// key that cannot be read fails the load, naming the registry.
func GitHubTrustedKeys(armored ...string) GitHubOption {
	return func(g *GitHub) {
		g.trustedKeys = append(g.trustedKeys, armored...)
	}
}

// GitHubToken sets the GitHub token sent with every request, which installing
// from a private repository needs. Without it the registry uses the
// GITHUB_TOKEN environment variable, then GH_TOKEN, when plugins load. Public
// repositories need no token.
func GitHubToken(token string) GitHubOption {
	return func(g *GitHub) {
		g.token = token
	}
}

// GitHubAPIURL sets the base URL of the GitHub REST API releases are read
// from. It exists for tests and GitHub-compatible endpoints, an empty url
// keeps DefaultGitHubAPIURL.
func GitHubAPIURL(url string) GitHubOption {
	return func(g *GitHub) {
		if url != "" {
			g.apiURL = strings.TrimRight(url, "/")
		}
	}
}

// NewGitHub returns an empty GitHub registry
//
//	gh := registry.NewGitHub()
//	gh.RegisterPlugin("jumppad-labs/xcl-plugin-docker", "v1.2.0")
//
//	c, err := xcl.NewConfig(xcl.WithRegistry(gh))
func NewGitHub(options ...GitHubOption) *GitHub {
	g := &GitHub{
		local:      NewLocal(),
		apiURL:     DefaultGitHubAPIURL,
		httpClient: http.DefaultClient,
		platform:   currentPlatform(),
	}

	for _, option := range options {
		option(g)
	}

	return g
}

// Name returns GitHubName
func (g *GitHub) Name() string {
	return GitHubName
}

// RegisterPlugin declares the plugin published as releases of repository,
// given as owner/repo, pinned to version, one exact release tag such as
// v1.2.0 or v1.2.0-rc.1. The plugin is named after the repository. Nothing is
// downloaded here, the release is installed, or found in the cache, when
// plugins load.
//
// A declaration that is wrong on every run is a programmer error and panics
// here, naming the plugin: a repository that is not owner/repo, or a version
// that is empty, a range, latest or anything other than one exact tag. A
// release that cannot be found or does not verify is reported when plugins
// load.
func (g *GitHub) RegisterPlugin(repository, version string) {
	if err := validateRepository(repository); err != nil {
		panic(fmt.Sprintf("xcl: registry %s: plugin %q: %s", GitHubName, repository, err))
	}

	if !exactTag.MatchString(version) {
		panic(fmt.Sprintf(
			"xcl: registry %s: plugin %q: version %q must be one exact release tag such as v1.2.0, not a range or latest",
			GitHubName, repository, version,
		))
	}

	g.local.add(localEntry{plugin: &githubPlugin{registry: g, repository: repository, tag: version}})
}

// RegisterType declares prototype, a plain Go type, as a block type, as
// Local.RegisterType does
func (g *GitHub) RegisterType(prototype any, name ...string) {
	g.local.RegisterType(prototype, name...)
}

// Types returns the Go types declared with RegisterType, in the order they
// were registered, or nil when none were
func (g *GitHub) Types() []Type {
	return g.local.Types()
}

// Plugins returns one plugin per RegisterPlugin call, and any other plugin
// registered on the wrapped local registry, in registration order. Nothing is
// downloaded here: each plugin installs, or finds in the cache, and verifies
// its release when it is started, so a failure names the plugin. ctx bounds
// the requests those starts make to GitHub. It fails when a trusted key
// cannot be read.
func (g *GitHub) Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	keys, err := readTrustedKeys(g.trustedKeys)
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.ctx = ctx
	g.keys = keys
	g.resolvedToken = resolveToken(g.token)
	g.mu.Unlock()

	return g.local.Plugins(ctx, emit)
}

// requestContext is the context given to Plugins, or the background context before
// plugins load
func (g *GitHub) requestContext() context.Context {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.ctx == nil {
		return context.Background()
	}

	return g.ctx
}

// trusted is the trusted keys read when plugins loaded, empty when none were
// given
func (g *GitHub) trusted() openpgp.EntityList {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.keys
}

// readTrustedKeys reads ASCII-armoured public keys into one key ring
func readTrustedKeys(armored []string) (openpgp.EntityList, error) {
	var keys openpgp.EntityList

	for index, key := range armored {
		read, err := openpgp.ReadArmoredKeyRing(strings.NewReader(key))
		if err != nil {
			return nil, fmt.Errorf("trusted key %d cannot be read: %w", index+1, err)
		}

		keys = append(keys, read...)
	}

	return keys, nil
}

// resolveToken is token, or else GITHUB_TOKEN, or else GH_TOKEN
func resolveToken(token string) string {
	if token != "" {
		return token
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return os.Getenv("GH_TOKEN")
}

// client is the release client the registry's plugins download with,
// carrying the token resolved when plugins loaded
func (g *GitHub) client() *releaseClient {
	g.mu.Lock()
	defer g.mu.Unlock()

	return &releaseClient{baseURL: g.apiURL, token: g.resolvedToken, http: g.httpClient}
}

// cacheRoot is the directory installed plugins are kept in, the default under
// the home directory unless GitHubCacheDir chose one
func (g *GitHub) cacheRoot() (string, error) {
	if g.cacheDir != "" {
		return g.cacheDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to find the home directory for the plugin cache: %w", err)
	}

	return filepath.Join(home, ".xcl", "cache", "plugins"), nil
}

// validateRepository checks repository is owner/repo
func validateRepository(repository string) error {
	owner, name, found := strings.Cut(repository, "/")
	if !found || owner == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("repository must be given as owner/repo")
	}

	return nil
}
