package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// githubPlugin is one plugin declared with GitHub.RegisterPlugin, kept by the
// registry's wrapped local registry. Starting it installs its release into
// the cache, or finds it there, verifies the cache entry and only then starts
// the cached binary as an external plugin, so every start of the plugin is
// verified.
type githubPlugin struct {
	registry   *GitHub
	repository string
	tag        string
}

// Name returns the repository's name, the plugin's name under the release
// asset contract
func (p *githubPlugin) Name() string {
	_, name, _ := strings.Cut(p.repository, "/")
	return name
}

// Start installs the plugin, or finds it in the cache, verifies it and starts
// it, returning the host started for the cached binary with the arguments
// its manifest names. A plugin that cannot be installed or verified
// is never started, and the error is a PluginInstallError naming the
// repository, version and platform.
func (p *githubPlugin) Start(emit events.Emit) (plugins.PluginHost, error) {
	binary, args, err := p.install(emit)
	if err != nil {
		return nil, &xclerrors.PluginInstallError{
			Repository: p.repository,
			Version:    p.tag,
			Platform:   p.registry.platform.String(),
			Err:        err,
		}
	}

	return executablePlugin(binary, args...).Start(emit)
}

// install returns the path of the plugin's verified binary in the cache and
// the arguments its manifest starts it with, installing the release first
// when the cache has no entry for it
func (p *githubPlugin) install(emit events.Emit) (string, []string, error) {
	current := p.registry.platform

	log := logger.New(emit, events.Event{
		Source:    events.SourceCore,
		Operation: events.OperationLoad,
		Meta:      map[string]any{"registry": GitHubName, "plugin": pluginName(p.repository)},
	})

	if !current.supported() {
		return "", nil, fmt.Errorf("%w: no build is published for %s", xclerrors.ErrPluginNotFound, current)
	}

	entry, err := p.entry()
	if err != nil {
		return "", nil, err
	}

	_, err = os.Stat(entry)

	switch {
	case err == nil:
		log.Debug("using cached plugin", "repository", p.repository, "version", p.tag, "entry", entry)
	case errors.Is(err, fs.ErrNotExist):
		if err := p.download(log, entry); err != nil {
			return "", nil, err
		}
	default:
		return "", nil, err
	}

	command, err := verifyEntry(entry, p.repository, p.tag, current, p.registry.trusted())
	if err != nil {
		return "", nil, err
	}

	log.Debug("verified plugin", "repository", p.repository, "version", p.tag, "entry", entry)

	return filepath.Join(entry, filepath.FromSlash(path.Clean(command[0]))), command[1:], nil
}

// entry is the plugin's cache entry,
// <cache>/github.com/<owner>/<repo>/<tag>/<os>_<arch>
func (p *githubPlugin) entry() (string, error) {
	root, err := p.registry.cacheRoot()
	if err != nil {
		return "", err
	}

	owner, repo, _ := strings.Cut(p.repository, "/")
	current := p.registry.platform

	return filepath.Join(root, GitHubName, owner, repo, p.tag, current.os+"_"+current.arch), nil
}

// download installs the release into entry as one step, into a temporary
// directory beside the entry. It downloads the index, and its signature, and
// checks them before downloading anything else, so a release that is
// unsigned when keys are trusted, or whose index is incomplete or offers no
// build for this platform, is refused without a build being fetched. Only
// this platform's zip is then downloaded and checked against the index, its
// files extracted, and the directory renamed into place. A failure at any
// point removes the temporary directory, so nothing unverified is ever left
// where a later start would find it.
func (p *githubPlugin) download(log logger.Logger, entry string) error {
	parent := filepath.Dir(entry)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("unable to create the plugin cache: %w", err)
	}

	staging, err := os.MkdirTemp(parent, ".install-*")
	if err != nil {
		return fmt.Errorf("unable to create the plugin cache: %w", err)
	}
	defer os.RemoveAll(staging)

	ctx := p.registry.requestContext()
	client := p.registry.client()
	current := p.registry.platform

	log.Info("downloading plugin", "repository", p.repository, "version", p.tag, "platform", current.String())

	release, err := client.release(ctx, p.repository, p.tag)
	if err != nil {
		return err
	}

	index, err := indexAsset(release, p.repository, p.tag)
	if err != nil {
		return err
	}

	assets := []githubAsset{index}

	// the signature is kept whenever the release has one, so a later run
	// that trusts keys can check it offline, but it is required only when
	// keys are trusted
	keys := p.registry.trusted()
	signature, signed := release.asset(signatureName(index.Name))

	switch {
	case signed:
		assets = append(assets, signature)
	case len(keys) > 0:
		return fmt.Errorf("%w: release is not signed, and trusted keys were supplied", xclerrors.ErrPluginVerification)
	}

	for _, asset := range assets {
		if err := downloadTo(ctx, client, asset, filepath.Join(staging, asset.Name)); err != nil {
			return err
		}
	}

	releaseIndex, build, err := checkIndex(staging, p.repository, p.tag, current, keys)
	if err != nil {
		return err
	}

	zipAsset, found := release.asset(build.Asset)
	if !found {
		return fmt.Errorf(
			"%w: release %s of %s has no asset %s, which its index lists for %s",
			xclerrors.ErrPluginNotFound, p.tag, p.repository, build.Asset, current,
		)
	}

	if err := downloadTo(ctx, client, zipAsset, filepath.Join(staging, zipAsset.Name)); err != nil {
		return err
	}

	manifest, err := checkBuild(staging, releaseIndex, build, current)
	if err != nil {
		return err
	}

	if err := extractZip(filepath.Join(staging, build.Asset), build.Asset, staging, manifest.Executable.Command[0]); err != nil {
		return err
	}

	if err := os.Rename(staging, entry); err != nil {
		// another process installed the same entry first, it is verified as
		// any cached entry is
		if _, statErr := os.Stat(entry); statErr == nil {
			return nil
		}

		return fmt.Errorf("unable to add the plugin to the cache: %w", err)
	}

	log.Info("installed plugin", "repository", p.repository, "version", p.tag, "entry", entry)

	return nil
}

// indexAsset is the index among the assets of release, the release of
// repository at tag, under either of the names it may have. A release with
// neither is not a plugin release, and one with both is refused, since the
// registry can't tell which of them describes the release.
func indexAsset(release *githubRelease, repository, tag string) (githubAsset, error) {
	names := indexNames(repository, tag)

	found := []githubAsset{}
	for _, name := range names {
		if asset, ok := release.asset(name); ok {
			found = append(found, asset)
		}
	}

	switch len(found) {
	case 0:
		return githubAsset{}, fmt.Errorf(
			"%w: release %s of %s is not a plugin release in the supported layout, it has no index %s",
			xclerrors.ErrPluginNotFound, tag, repository, strings.Join(names, " or "),
		)
	case 1:
		return found[0], nil
	default:
		return githubAsset{}, fmt.Errorf(
			"%w: release %s of %s has two indexes, %s and %s, and must have one",
			xclerrors.ErrPluginVerification, tag, repository, found[0].Name, found[1].Name,
		)
	}
}
