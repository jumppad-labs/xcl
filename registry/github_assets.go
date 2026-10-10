package registry

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// This file is the one place that knows the names and formats the plugin
// release asset contract fixes: which archive holds the build for a platform,
// what the checksums and signature files are called, what the binary inside
// the archive is called, and how the checksums file is written. The rest of
// the GitHub registry asks it for names rather than building them.

// exactTag matches one exact release tag, v<major>.<minor>.<patch> with an
// optional pre-release suffix, the only form a plugin may be pinned to
var exactTag = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?$`)

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

// assetNames are the names of the files of one release the registry reads,
// for one platform
type assetNames struct {
	// plugin is the plugin's name, the repository name
	plugin string
	// version is the tag without its leading v
	version string
	// archive is the platform's build
	archive string
	// checksums lists the SHA-256 of every archive in the release
	checksums string
	// signature is the detached signature over the checksums file
	signature string
	// binary is the plugin binary at the root of the archive
	binary string
	// zip is true when the archive is a zip rather than a gzipped tar
	zip bool
}

// namesFor returns the names of the release files for repository (owner/repo)
// at tag, for the platform p
func namesFor(repository, tag string, p platform) assetNames {
	plugin := repository
	if index := strings.LastIndex(repository, "/"); index >= 0 {
		plugin = repository[index+1:]
	}

	version := strings.TrimPrefix(tag, "v")

	names := assetNames{
		plugin:    plugin,
		version:   version,
		archive:   fmt.Sprintf("%s_%s_%s_%s.tar.gz", plugin, version, p.os, p.arch),
		checksums: fmt.Sprintf("%s_%s_checksums.txt", plugin, version),
		binary:    plugin,
	}

	if p.os == "windows" {
		names.archive = fmt.Sprintf("%s_%s_%s_%s.zip", plugin, version, p.os, p.arch)
		names.binary = plugin + ".exe"
		names.zip = true
	}

	names.signature = names.checksums + ".sig"

	return names
}

// parseChecksums reads a checksums file in the format sha256sum writes, one
// "<hex sha256>  <archive name>" per line, into archive name to lowercase hex
// SHA-256. The binary-mode marker sha256sum -b writes before the name is
// accepted, as are blank lines. Any other line is an error.
func parseChecksums(data []byte) (map[string]string, error) {
	sums := map[string]string{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	line := 0

	for scanner.Scan() {
		line++

		text := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(text) == "" {
			continue
		}

		sum, name, found := strings.Cut(text, " ")
		if !found || len(sum) != 64 {
			return nil, fmt.Errorf("checksums line %d is not \"<sha256>  <file>\"", line)
		}

		if _, err := hex.DecodeString(sum); err != nil {
			return nil, fmt.Errorf("checksums line %d has an invalid sha256", line)
		}

		// sha256sum writes a second character, a space for text mode or a *
		// for binary mode, before the name
		if len(name) < 2 || (name[0] != ' ' && name[0] != '*') {
			return nil, fmt.Errorf("checksums line %d is not \"<sha256>  <file>\"", line)
		}

		name = name[1:]
		sums[name] = strings.ToLower(sum)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("unable to read checksums: %w", err)
	}

	return sums, nil
}
