package registry

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// This file is the one place that knows the names version 2 of the plugin
// release asset contract fixes: the plugin's name and version, and the names
// of the release's index and its signature. The zips' names are taken from
// the index, never computed. The rest of the GitHub registry asks it for
// names rather than building them.

// exactTag matches one exact release tag, v<major>.<minor>.<patch> with an
// optional pre-release suffix, the only form a plugin may be pinned to
var exactTag = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?$`)

// pluginProtocol is the plugin protocol version this registry's host speaks.
// A build whose index entry doesn't list it is refused.
const pluginProtocol = 1

// pluginName is the plugin's name for repository (owner/repo): the
// repository's name
func pluginName(repository string) string {
	if index := strings.LastIndex(repository, "/"); index >= 0 {
		return repository[index+1:]
	}

	return repository
}

// releaseVersion is the version a release tag names: the tag without its
// leading v
func releaseVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// indexName is the name of the index of the release of repository at tag,
// <name>_<version>_index.xcl
func indexName(repository, tag string) string {
	return fmt.Sprintf("%s_%s_index.xcl", pluginName(repository), releaseVersion(tag))
}

// indexSignatureName is the name of the detached signature over the index of
// the release of repository at tag
func indexSignatureName(repository, tag string) string {
	return indexName(repository, tag) + ".sig"
}

// platform is an operating system and architecture a plugin is built for
type platform struct {
	os   string
	arch string
}

// currentPlatform is the platform this program is running on
func currentPlatform() platform {
	return platform{os: runtime.GOOS, arch: runtime.GOARCH}
}

// String returns the platform as os/arch, i.e. linux/arm64
func (p platform) String() string {
	return p.os + "/" + p.arch
}

// supported reports whether the contract publishes builds for the platform:
// linux, darwin and windows on amd64 and arm64
func (p platform) supported() bool {
	switch p.os {
	case "linux", "darwin", "windows":
	default:
		return false
	}

	return p.arch == "amd64" || p.arch == "arm64"
}
