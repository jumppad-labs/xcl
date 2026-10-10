package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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
// it, returning the host Executable starts for the cached binary. A plugin
// that cannot be installed or verified is never started, and the error is a
// PluginInstallError naming the repository, version and platform.
func (p *githubPlugin) Start(emit events.Emit) (plugins.PluginHost, error) {
	binary, err := p.install(emit)
	if err != nil {
		return nil, &xclerrors.PluginInstallError{
			Repository: p.repository,
			Version:    p.tag,
			Platform:   p.registry.platform.String(),
			Err:        err,
		}
	}

	return Executable(binary).Start(emit)
}

// install returns the path of the plugin's verified binary in the cache,
// installing the release first when the cache has no entry for it
func (p *githubPlugin) install(emit events.Emit) (string, error) {
	current := p.registry.platform
	names := namesFor(p.repository, p.tag, current)

	log := logger.New(emit, events.Event{
		Source:    events.SourceCore,
		Operation: events.OperationLoad,
		Meta:      map[string]any{"registry": GitHubName, "plugin": names.plugin},
	})

	if !current.supported() {
		return "", fmt.Errorf("%w: no build is published for %s", xclerrors.ErrPluginNotFound, current)
	}

	entry, err := p.entry()
	if err != nil {
		return "", err
	}

	_, err = os.Stat(entry)

	switch {
	case err == nil:
		log.Debug("using cached plugin", "repository", p.repository, "version", p.tag, "entry", entry)
	case errors.Is(err, fs.ErrNotExist):
		if err := p.download(log, entry, names); err != nil {
			return "", err
		}
	default:
		return "", err
	}

	if err := verifyEntry(entry, names, p.registry.trusted()); err != nil {
		return "", err
	}

	log.Debug("verified plugin", "repository", p.repository, "version", p.tag, "entry", entry)

	return filepath.Join(entry, names.binary), nil
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

// download installs the release into entry as one step: everything is
// downloaded into a temporary directory beside the entry, verified there, the
// binary extracted, and the directory renamed into place. A failure at any
// point removes the temporary directory, so nothing unverified is ever left
// where a later start would find it.
func (p *githubPlugin) download(log logger.Logger, entry string, names assetNames) error {
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

	log.Info("downloading plugin", "repository", p.repository, "version", p.tag, "platform", p.registry.platform.String())

	release, err := client.release(ctx, p.repository, p.tag)
	if err != nil {
		return err
	}

	archive, found := release.asset(names.archive)
	if !found {
		return fmt.Errorf(
			"%w: release %s of %s has no build for %s, %s",
			xclerrors.ErrPluginNotFound, p.tag, p.repository, p.registry.platform, names.archive,
		)
	}

	checksums, found := release.asset(names.checksums)
	if !found {
		return fmt.Errorf("%w: release has no checksums file %s", xclerrors.ErrPluginVerification, names.checksums)
	}

	assets := []githubAsset{archive, checksums}

	// the signature is kept whenever the release has one, so a later run
	// that trusts keys can check it offline, but it is required only when
	// keys are trusted
	keys := p.registry.trusted()
	signature, signed := release.asset(names.signature)

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

	if err := verifyArchive(staging, names, keys); err != nil {
		return err
	}

	if err := extractBinary(filepath.Join(staging, names.archive), names, filepath.Join(staging, names.binary)); err != nil {
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
