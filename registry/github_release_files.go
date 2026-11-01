package registry

import (
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

// This file reads the two files version 2 of the plugin release asset
// contract defines: a release's index, <name>_<version>_index.xcl, and the
// manifest at the root of each executable zip, xcl-plugin.xcl. The installer
// and the verifier ask it what a release offers and whether a build agrees
// with it; nothing else in the registry parses either file.
//
// Both are static xcl metadata files. They are decoded with xcl's own
// decoder into fixed Go structs and no evaluation context, so a variable,
// function call or reference is refused with xcl's own diagnostic pointing at
// it, and a field a block doesn't have is an ordinary xcl error. A remain
// field collects the top-level blocks the registry doesn't know, so a later
// contract can add package kinds that this registry skips. Later xcl-owned
// metadata files should be read the same way, rather than through Config,
// which evaluates expressions, reads .vars files and the environment, and
// loads plugins.

// manifestName is the name of the manifest at the root of every executable
// zip
const manifestName = "xcl-plugin.xcl"

// checksumPattern is the only checksum form the contract allows,
// sha256:<64 lowercase hex digits>
var checksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// imageDigestPattern is the only container image form the contract allows,
// an image pinned by the digest of its image index
var imageDigestPattern = regexp.MustCompile(`^.+@sha256:[0-9a-f]{64}$`)

// releaseIndex is a decoded release index: the plugin it describes and the
// package kinds it offers
type releaseIndex struct {
	Plugin     indexPlugin      `xcl:"plugin,block"`
	Executable *indexExecutable `xcl:"executable,block"`
	Container  *indexContainer  `xcl:"container,block"`

	// Rest holds the top-level blocks of kinds this registry doesn't know,
	// which are skipped
	Rest hcl.Body `xcl:",remain"`
}

// indexPlugin names the plugin and the version a release or build is of
type indexPlugin struct {
	Name    string `xcl:"name"`
	Version string `xcl:"version"`
}

// indexExecutable is the index's executable entry: a zip for each platform
type indexExecutable struct {
	Protocol  []int           `xcl:"protocol"`
	Platforms []indexPlatform `xcl:"platform,block"`
}

// indexPlatform is one platform an executable is built for, with the zip
// that holds the build and the zip's checksum. Asset and checksum are
// optional to the decoder so the registry, not the parser, names a missing
// one.
type indexPlatform struct {
	OS       string `xcl:"os,label"`
	Arch     string `xcl:"arch,label"`
	Asset    string `xcl:"asset,optional"`
	Checksum string `xcl:"checksum,optional"`
}

// indexContainer is the index's container entry: one multi-platform image
type indexContainer struct {
	Protocol  []int           `xcl:"protocol"`
	Image     string          `xcl:"image"`
	Command   []string        `xcl:"command,optional"`
	Platforms []imagePlatform `xcl:"platform,block"`
}

// imagePlatform is a platform named by its labels alone, as a container
// lists them and a manifest names its build's
type imagePlatform struct {
	OS   string `xcl:"os,label"`
	Arch string `xcl:"arch,label"`
}

// buildManifest is a decoded xcl-plugin.xcl, the description of the build a
// zip holds
type buildManifest struct {
	Plugin     indexPlugin        `xcl:"plugin,block"`
	Executable manifestExecutable `xcl:"executable,block"`

	// Rest holds the top-level blocks of kinds this registry doesn't know,
	// which are skipped
	Rest hcl.Body `xcl:",remain"`
}

// manifestExecutable is a manifest's executable: the protocols it speaks,
// the command that starts it and the one platform it was built for
type manifestExecutable struct {
	Protocol  []int           `xcl:"protocol"`
	Command   []string        `xcl:"command"`
	Platforms []imagePlatform `xcl:"platform,block"`
}

// chosenBuild is the executable build the registry installs for a platform:
// the zip holding it and the zip's expected SHA-256
type chosenBuild struct {
	Platform platform
	Asset    string
	SHA256   [32]byte
}

// readIndex decodes a release index from src, read from the asset filename.
// An index that isn't valid xcl, uses an expression, or has a field a block
// doesn't have fails verification with xcl's diagnostic. Top-level blocks of
// unknown kinds are skipped.
func readIndex(src []byte, filename string) (*releaseIndex, error) {
	index := &releaseIndex{}
	if err := decodeStatic(src, filename, index); err != nil {
		return nil, err
	}

	return index, nil
}

// readManifest decodes the manifest at the root of the zip named asset. A
// manifest that isn't valid xcl, uses an expression, or has no command, or a
// command whose binary isn't a path inside the zip, fails verification.
func readManifest(src []byte, asset string) (*buildManifest, error) {
	manifest := &buildManifest{}
	if err := decodeStatic(src, asset+"/"+manifestName, manifest); err != nil {
		return nil, err
	}

	if len(manifest.Executable.Command) == 0 || manifest.Executable.Command[0] == "" {
		return nil, fmt.Errorf("%w: %s in %s has no command", xclerrors.ErrPluginVerification, manifestName, asset)
	}

	binary := manifest.Executable.Command[0]
	if !insideZip(binary) {
		return nil, fmt.Errorf("%w: %s in %s names command %q, which is not a path inside the zip", xclerrors.ErrPluginVerification, manifestName, asset, binary)
	}

	return manifest, nil
}

// decodeStatic decodes the static xcl file src, named filename, into target
// with no evaluation context, so only literal values are accepted
func decodeStatic(src []byte, filename string, target any) error {
	file, diagnostics := hclsyntax.ParseConfig(src, filename, hcl.InitialPos)
	if diagnostics.HasErrors() {
		return fmt.Errorf("%w: %s", xclerrors.ErrPluginVerification, diagnostics.Error())
	}

	diagnostics = gohcl.DecodeBody(file.Body, nil, target)
	if diagnostics.HasErrors() {
		return fmt.Errorf("%w: %s", xclerrors.ErrPluginVerification, diagnostics.Error())
	}

	return nil
}

// validate checks the index is complete for the release of repository
// (owner/repo) at tag before anything is downloaded: it names this plugin and
// version, every executable platform names its zip and carries a sha256
// checksum, a container is pinned by digest and lists only Linux platforms,
// every platform is one the contract supports, and at least one known kind is
// offered. A failure is a verification failure naming the problem.
func (i *releaseIndex) validate(repository, tag string) error {
	name := pluginName(repository)
	if i.Plugin.Name != name {
		return fmt.Errorf("%w: index names plugin %q, not %q", xclerrors.ErrPluginVerification, i.Plugin.Name, name)
	}

	version := releaseVersion(tag)
	if i.Plugin.Version != version {
		return fmt.Errorf("%w: index names version %q, not %q", xclerrors.ErrPluginVerification, i.Plugin.Version, version)
	}

	if i.Executable == nil && i.Container == nil {
		return fmt.Errorf("%w: index offers no package kind this registry knows (executable or container)", xclerrors.ErrPluginVerification)
	}

	if i.Executable != nil {
		if err := i.Executable.validate(); err != nil {
			return err
		}
	}

	if i.Container != nil {
		if err := i.Container.validate(); err != nil {
			return err
		}
	}

	return nil
}

// validate checks every executable platform is supported and names its zip
// and a well-formed checksum
func (e *indexExecutable) validate() error {
	if len(e.Platforms) == 0 {
		return fmt.Errorf("%w: index executable lists no platforms", xclerrors.ErrPluginVerification)
	}

	for _, p := range e.Platforms {
		current := platform{os: p.OS, arch: p.Arch}
		if !current.supported() {
			return fmt.Errorf("%w: index executable lists unsupported platform %s", xclerrors.ErrPluginVerification, current)
		}

		if p.Asset == "" {
			return fmt.Errorf("%w: index executable platform %s has no asset", xclerrors.ErrPluginVerification, current)
		}

		if !plainFileName(p.Asset) {
			return fmt.Errorf("%w: index executable platform %s names asset %q, which is not a file name", xclerrors.ErrPluginVerification, current, p.Asset)
		}

		if p.Checksum == "" {
			return fmt.Errorf("%w: index executable platform %s has no checksum", xclerrors.ErrPluginVerification, current)
		}

		if !checksumPattern.MatchString(p.Checksum) {
			return fmt.Errorf("%w: index executable platform %s has checksum %q, which is not sha256:<64 lowercase hex digits>", xclerrors.ErrPluginVerification, current, p.Checksum)
		}
	}

	return nil
}

// validate checks the container's image is pinned by digest and it lists
// only supported Linux platforms
func (c *indexContainer) validate() error {
	if !imageDigestPattern.MatchString(c.Image) {
		return fmt.Errorf("%w: index container image %q is not pinned by digest (<repository>@sha256:<64 hex digits>)", xclerrors.ErrPluginVerification, c.Image)
	}

	if len(c.Platforms) == 0 {
		return fmt.Errorf("%w: index container lists no platforms", xclerrors.ErrPluginVerification)
	}

	for _, p := range c.Platforms {
		current := platform{os: p.OS, arch: p.Arch}
		if p.OS != "linux" {
			return fmt.Errorf("%w: index container lists platform %s, a container runs only on linux", xclerrors.ErrPluginVerification, current)
		}

		if !current.supported() {
			return fmt.Errorf("%w: index container lists unsupported platform %s", xclerrors.ErrPluginVerification, current)
		}
	}

	return nil
}

// choose returns the executable build for host from a validated index. A
// container is never chosen, since this registry doesn't start containers
// yet, so a release with no executable build for host, a container-only one
// included, is not found, naming the platforms the release offers and host.
// An executable that doesn't speak the protocol this registry speaks fails
// verification.
func (i *releaseIndex) choose(host platform) (chosenBuild, error) {
	if i.Executable != nil {
		for _, p := range i.Executable.Platforms {
			if p.OS != host.os || p.Arch != host.arch {
				continue
			}

			if !slices.Contains(i.Executable.Protocol, pluginProtocol) {
				return chosenBuild{}, fmt.Errorf(
					"%w: index executable speaks protocol %s, this registry speaks protocol %d",
					xclerrors.ErrPluginVerification, protocolList(i.Executable.Protocol), pluginProtocol,
				)
			}

			sum, err := hex.DecodeString(strings.TrimPrefix(p.Checksum, "sha256:"))
			if err != nil || len(sum) != 32 {
				return chosenBuild{}, fmt.Errorf("%w: index executable platform %s has an invalid checksum", xclerrors.ErrPluginVerification, host)
			}

			build := chosenBuild{Platform: host, Asset: p.Asset}
			copy(build.SHA256[:], sum)

			return build, nil
		}
	}

	return chosenBuild{}, fmt.Errorf(
		"%w: release offers no build for %s, it offers %s",
		xclerrors.ErrPluginNotFound, host, i.offered(),
	)
}

// offered describes the platforms the index offers, each executable platform
// as os/arch and each container platform as os/arch (container), or "nothing"
func (i *releaseIndex) offered() string {
	offers := []string{}

	if i.Executable != nil {
		for _, p := range i.Executable.Platforms {
			offers = append(offers, platform{os: p.OS, arch: p.Arch}.String())
		}
	}

	if i.Container != nil {
		for _, p := range i.Container.Platforms {
			offers = append(offers, platform{os: p.OS, arch: p.Arch}.String()+" (container)")
		}
	}

	if len(offers) == 0 {
		return "nothing"
	}

	return strings.Join(offers, ", ")
}

// agrees checks the manifest describes the build the index lists for host:
// the same plugin name, version and protocols, and exactly the one platform
// host. A disagreement fails verification naming it.
func (m *buildManifest) agrees(i *releaseIndex, host platform) error {
	if m.Plugin.Name != i.Plugin.Name {
		return fmt.Errorf("%w: %s names plugin %q, the index names %q", xclerrors.ErrPluginVerification, manifestName, m.Plugin.Name, i.Plugin.Name)
	}

	if m.Plugin.Version != i.Plugin.Version {
		return fmt.Errorf("%w: %s names version %q, the index names %q", xclerrors.ErrPluginVerification, manifestName, m.Plugin.Version, i.Plugin.Version)
	}

	if i.Executable == nil {
		return fmt.Errorf("%w: %s describes an executable, the index offers none", xclerrors.ErrPluginVerification, manifestName)
	}

	if !slices.Equal(m.Executable.Protocol, i.Executable.Protocol) {
		return fmt.Errorf(
			"%w: %s speaks protocol %s, the index lists %s",
			xclerrors.ErrPluginVerification, manifestName, protocolList(m.Executable.Protocol), protocolList(i.Executable.Protocol),
		)
	}

	if len(m.Executable.Platforms) != 1 {
		return fmt.Errorf("%w: %s lists %d platforms, a build is for exactly one", xclerrors.ErrPluginVerification, manifestName, len(m.Executable.Platforms))
	}

	built := platform{os: m.Executable.Platforms[0].OS, arch: m.Executable.Platforms[0].Arch}
	if built != host {
		return fmt.Errorf("%w: %s is a build for %s, the index lists it for %s", xclerrors.ErrPluginVerification, manifestName, built, host)
	}

	return nil
}

// plainFileName reports whether name is a single file name, with no
// directory, as every asset of a release is
func plainFileName(name string) bool {
	return name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// insideZip reports whether name is a relative path that stays inside the
// zip's root
func insideZip(name string) bool {
	if strings.Contains(name, `\`) || path.IsAbs(name) {
		return false
	}

	clean := path.Clean(name)

	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

// protocolList writes protocols as an xcl list, i.e. [1, 2]
func protocolList(protocols []int) string {
	parts := make([]string, len(protocols))
	for index, protocol := range protocols {
		parts[index] = fmt.Sprint(protocol)
	}

	return "[" + strings.Join(parts, ", ") + "]"
}
