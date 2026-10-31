package registry

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

const (
	releaseRepository = "acme/xcl-plugin-widget"
	releaseTag        = "v1.0.0"
	releaseIndexFile  = "xcl-plugin-widget_1.0.0_index.xcl"
	releaseZip        = "xcl-plugin-widget_1.0.0_linux_amd64.zip"

	checksumAmd64 = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	checksumArm64 = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	checksumMac   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	imageDigest = "ghcr.io/acme/xcl-plugin-widget@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// validIndex offers executable builds for linux/amd64, linux/arm64 and
// darwin/arm64
const validIndex = `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }

  platform "linux" "arm64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_arm64.zip"
    checksum = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }

  platform "darwin" "arm64" {
    asset    = "xcl-plugin-widget_1.0.0_darwin_arm64.zip"
    checksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
}
`

// containerOnlyIndex offers only a digest-pinned container
const containerOnlyIndex = `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

container {
  protocol = [1]
  image    = "ghcr.io/acme/xcl-plugin-widget@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

  platform "linux" "amd64" {}
  platform "linux" "arm64" {}
}
`

// validManifest describes the linux/amd64 build of validIndex
const validManifest = `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`

var linuxAmd64 = platform{os: "linux", arch: "amd64"}

func mustReadIndex(t *testing.T, src string) *releaseIndex {
	t.Helper()

	index, err := readIndex([]byte(src), releaseIndexFile)
	require.NoError(t, err)

	return index
}

func mustReadManifest(t *testing.T, src string) *buildManifest {
	t.Helper()

	manifest, err := readManifest([]byte(src), releaseZip)
	require.NoError(t, err)

	return manifest
}

func TestIndexNameFollowsContract(t *testing.T) {
	require.Equal(t, "xcl-plugin-widget_1.0.0_index.xcl", indexName("acme/xcl-plugin-widget", "v1.0.0"))
}

func TestIndexNameKeepsPreReleaseSuffix(t *testing.T) {
	require.Equal(t, "xcl-plugin-widget_1.2.0-rc.1_index.xcl", indexName("acme/xcl-plugin-widget", "v1.2.0-rc.1"))
}

func TestIndexSignatureNameFollowsContract(t *testing.T) {
	require.Equal(t, "xcl-plugin-widget_1.0.0_index.xcl.sig", indexSignatureName("acme/xcl-plugin-widget", "v1.0.0"))
}

func TestReadIndexDecodesValidIndex(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	require.Equal(t, "xcl-plugin-widget", index.Plugin.Name)
	require.Equal(t, "1.0.0", index.Plugin.Version)
	require.NotNil(t, index.Executable)
	require.Nil(t, index.Container)
	require.Equal(t, []int{1}, index.Executable.Protocol)
	require.Len(t, index.Executable.Platforms, 3)
	require.Equal(t, indexPlatform{
		OS:       "linux",
		Arch:     "amd64",
		Asset:    "xcl-plugin-widget_1.0.0_linux_amd64.zip",
		Checksum: checksumAmd64,
	}, index.Executable.Platforms[0])
}

func TestReadIndexDecodesTwoPlatformsSharingAnOS(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	require.Equal(t, "linux", index.Executable.Platforms[0].OS)
	require.Equal(t, "amd64", index.Executable.Platforms[0].Arch)
	require.Equal(t, "linux", index.Executable.Platforms[1].OS)
	require.Equal(t, "arm64", index.Executable.Platforms[1].Arch)
}

func TestReadIndexDecodesContainer(t *testing.T) {
	index := mustReadIndex(t, containerOnlyIndex)

	require.Nil(t, index.Executable)
	require.NotNil(t, index.Container)
	require.Equal(t, imageDigest, index.Container.Image)
	require.Equal(t, []imagePlatform{{OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}}, index.Container.Platforms)
}

func TestReadIndexSkipsUnknownKind(t *testing.T) {
	index := mustReadIndex(t, validIndex+`
wasm {
  protocol = [3]
  module   = "plugin.wasm"
}
`)

	require.Len(t, index.Executable.Platforms, 3)
	require.NoError(t, index.validate(releaseRepository, releaseTag))
}

func TestReadIndexRefusesVariable(t *testing.T) {
	_, err := readIndex([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = var.version
}
`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "Variables not allowed")
	require.ErrorContains(t, err, releaseIndexFile+":4,")
}

func TestReadIndexRefusesReference(t *testing.T) {
	_, err := readIndex([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = plugin.name
}
`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "Variables not allowed")
	require.ErrorContains(t, err, releaseIndexFile+":4,")
}

func TestReadIndexRefusesFunctionCall(t *testing.T) {
	_, err := readIndex([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = upper("1.0.0")
}
`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "Function calls not allowed")
	require.ErrorContains(t, err, releaseIndexFile+":4,")
}

func TestReadIndexRefusesDuplicatePlatform(t *testing.T) {
	_, err := readIndex([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "a.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }

  platform "linux" "amd64" {
    asset    = "b.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "platform")
}

func TestReadIndexRefusesFieldTheKindDoesNotHave(t *testing.T) {
	_, err := readIndex([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  image    = "ghcr.io/acme/widget:latest"

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "image")
}

func TestReadIndexRefusesMissingPluginBlock(t *testing.T) {
	_, err := readIndex([]byte(`
executable {
  protocol = [1]
}
`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "plugin")
}

func TestReadIndexRefusesInvalidSyntax(t *testing.T) {
	_, err := readIndex([]byte(`plugin {`), releaseIndexFile)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, releaseIndexFile)
}

func TestValidateAcceptsCompleteIndex(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	require.NoError(t, index.validate(releaseRepository, releaseTag))
}

func TestValidateAcceptsContainerOnlyIndex(t *testing.T) {
	index := mustReadIndex(t, containerOnlyIndex)

	require.NoError(t, index.validate(releaseRepository, releaseTag))
}

func TestValidateAcceptsContainerAlongsideExecutable(t *testing.T) {
	index := mustReadIndex(t, validIndex+`
container {
  protocol = [1]
  image    = "ghcr.io/acme/xcl-plugin-widget@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

  platform "linux" "amd64" {}
}
`)

	require.NoError(t, index.validate(releaseRepository, releaseTag))
}

func TestValidateRefusesWrongPluginName(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	err := index.validate("acme/xcl-plugin-other", releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `index names plugin "xcl-plugin-widget", not "xcl-plugin-other"`)
}

func TestValidateRefusesWrongVersion(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	err := index.validate(releaseRepository, "v2.0.0")

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `index names version "1.0.0", not "2.0.0"`)
}

func TestValidateRefusesMissingChecksum(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }

  platform "linux" "arm64" {
    asset = "xcl-plugin-widget_1.0.0_linux_arm64.zip"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index executable platform linux/arm64 has no checksum")
}

func TestValidateRefusesChecksumWithoutAlgorithm(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index executable platform linux/amd64 has checksum")
	require.ErrorContains(t, err, "not sha256:<64 lowercase hex digits>")
}

func TestValidateRefusesShortChecksum(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9f86d081"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "not sha256:<64 lowercase hex digits>")
}

func TestValidateRefusesUppercaseChecksum(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9F86D081884C7D659A2FEAA0C55AD015A3BF4F1B2B0B822CD15D6C15B0F00A08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "not sha256:<64 lowercase hex digits>")
}

func TestValidateRefusesMissingAsset(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index executable platform linux/amd64 has no asset")
}

func TestValidateRefusesExecutableWithoutPlatforms(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index executable lists no platforms")
}

func TestValidateRefusesUnsupportedExecutablePlatform(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "freebsd" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_freebsd_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "unsupported platform freebsd/amd64")
}

func TestValidateRefusesContainerImageByTag(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

container {
  protocol = [1]
  image    = "ghcr.io/acme/xcl-plugin-widget:1.0.0"

  platform "linux" "amd64" {}
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `index container image "ghcr.io/acme/xcl-plugin-widget:1.0.0" is not pinned by digest`)
}

func TestValidateRefusesContainerImageWithShortDigest(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

container {
  protocol = [1]
  image    = "ghcr.io/acme/xcl-plugin-widget@sha256:bbbb"

  platform "linux" "amd64" {}
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "is not pinned by digest")
}

func TestValidateRefusesNonLinuxContainerPlatform(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

container {
  protocol = [1]
  image    = "ghcr.io/acme/xcl-plugin-widget@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

  platform "linux" "amd64" {}
  platform "darwin" "arm64" {}
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index container lists platform darwin/arm64, a container runs only on linux")
}

func TestValidateRefusesIndexWithNoKnownKind(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

wasm {
  protocol = [3]
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index offers no package kind this registry knows")
}

func TestChooseReturnsHostBuild(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	build, err := index.choose(linuxAmd64)
	require.NoError(t, err)

	expected, err := hex.DecodeString("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08")
	require.NoError(t, err)

	require.Equal(t, linuxAmd64, build.Platform)
	require.Equal(t, "xcl-plugin-widget_1.0.0_linux_amd64.zip", build.Asset)
	require.Equal(t, expected, build.SHA256[:])
}

func TestChooseReturnsBuildForOtherArchitectureOfSameOS(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	build, err := index.choose(platform{os: "linux", arch: "arm64"})
	require.NoError(t, err)

	require.Equal(t, "xcl-plugin-widget_1.0.0_linux_arm64.zip", build.Asset)
}

func TestChooseRefusesPlatformTheReleaseDoesNotOffer(t *testing.T) {
	index := mustReadIndex(t, validIndex)

	_, err := index.choose(platform{os: "windows", arch: "amd64"})

	require.ErrorIs(t, err, xclerrors.ErrPluginNotFound)
	require.ErrorContains(t, err, "release offers no build for windows/amd64, it offers linux/amd64, linux/arm64, darwin/arm64")
}

func TestChooseRefusesContainerOnlyRelease(t *testing.T) {
	index := mustReadIndex(t, containerOnlyIndex)

	_, err := index.choose(linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginNotFound)
	require.ErrorContains(t, err, "release offers no build for linux/amd64, it offers linux/amd64 (container), linux/arm64 (container)")
}

func TestChooseIgnoresContainerAlongsideExecutable(t *testing.T) {
	index := mustReadIndex(t, validIndex+`
container {
  protocol = [1]
  image    = "ghcr.io/acme/xcl-plugin-widget@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

  platform "linux" "amd64" {}
}
`)

	build, err := index.choose(linuxAmd64)
	require.NoError(t, err)

	require.Equal(t, "xcl-plugin-widget_1.0.0_linux_amd64.zip", build.Asset)
}

func TestChooseRefusesExecutableThatDoesNotSpeakProtocolOne(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [2]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	_, err := index.choose(linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "index executable speaks protocol [2], this registry speaks protocol 1")
}

func TestChooseAcceptsExecutableSpeakingSeveralProtocols(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1, 2]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-widget_1.0.0_linux_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	build, err := index.choose(linuxAmd64)
	require.NoError(t, err)

	require.Equal(t, "xcl-plugin-widget_1.0.0_linux_amd64.zip", build.Asset)
}

func TestReadManifestDecodesManifest(t *testing.T) {
	manifest := mustReadManifest(t, validManifest)

	require.Equal(t, "xcl-plugin-widget", manifest.Plugin.Name)
	require.Equal(t, "1.0.0", manifest.Plugin.Version)
	require.Equal(t, []int{1}, manifest.Executable.Protocol)
	require.Equal(t, []string{"xcl-plugin-widget"}, manifest.Executable.Command)
	require.Equal(t, []imagePlatform{{OS: "linux", Arch: "amd64"}}, manifest.Executable.Platforms)
}

func TestReadManifestAcceptsCommandWithArguments(t *testing.T) {
	manifest := mustReadManifest(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["bin/xcl-plugin-widget", "--serve"]

  platform "linux" "amd64" {}
}
`)

	require.Equal(t, []string{"bin/xcl-plugin-widget", "--serve"}, manifest.Executable.Command)
}

func TestReadManifestSkipsUnknownKind(t *testing.T) {
	manifest := mustReadManifest(t, validManifest+`
wasm {
  protocol = [3]
}
`)

	require.Equal(t, "xcl-plugin-widget", manifest.Plugin.Name)
}

func TestReadManifestRefusesVariable(t *testing.T) {
	_, err := readManifest([]byte(`
plugin {
  name    = var.name
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`), releaseZip)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "Variables not allowed")
	require.ErrorContains(t, err, releaseZip+"/xcl-plugin.xcl:3,")
}

func TestReadManifestRefusesFunctionCall(t *testing.T) {
	_, err := readManifest([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = [lower("XCL-PLUGIN-WIDGET")]

  platform "linux" "amd64" {}
}
`), releaseZip)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "Function calls not allowed")
	require.ErrorContains(t, err, releaseZip+"/xcl-plugin.xcl:9,")
}

func TestReadManifestRefusesEmptyCommand(t *testing.T) {
	_, err := readManifest([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = []

  platform "linux" "amd64" {}
}
`), releaseZip)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "xcl-plugin.xcl in "+releaseZip+" has no command")
}

func TestReadManifestRefusesAbsoluteCommand(t *testing.T) {
	_, err := readManifest([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["/usr/bin/xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`), releaseZip)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `names command "/usr/bin/xcl-plugin-widget", which is not a path inside the zip`)
}

func TestReadManifestRefusesCommandOutsideZip(t *testing.T) {
	_, err := readManifest([]byte(`
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["bin/../../xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`), releaseZip)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "which is not a path inside the zip")
}

func TestManifestAgreesWithIndex(t *testing.T) {
	index := mustReadIndex(t, validIndex)
	manifest := mustReadManifest(t, validManifest)

	require.NoError(t, manifest.agrees(index, linuxAmd64))
}

func TestManifestDisagreesOnName(t *testing.T) {
	index := mustReadIndex(t, validIndex)
	manifest := mustReadManifest(t, `
plugin {
  name    = "xcl-plugin-other"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`)

	err := manifest.agrees(index, linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `xcl-plugin.xcl names plugin "xcl-plugin-other", the index names "xcl-plugin-widget"`)
}

func TestManifestDisagreesOnVersion(t *testing.T) {
	index := mustReadIndex(t, validIndex)
	manifest := mustReadManifest(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.1"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`)

	err := manifest.agrees(index, linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `xcl-plugin.xcl names version "1.0.1", the index names "1.0.0"`)
}

func TestManifestDisagreesOnProtocol(t *testing.T) {
	index := mustReadIndex(t, validIndex)
	manifest := mustReadManifest(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1, 2]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
}
`)

	err := manifest.agrees(index, linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "xcl-plugin.xcl speaks protocol [1, 2], the index lists [1]")
}

func TestManifestDisagreesOnPlatform(t *testing.T) {
	index := mustReadIndex(t, validIndex)
	manifest := mustReadManifest(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "darwin" "arm64" {}
}
`)

	err := manifest.agrees(index, linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "xcl-plugin.xcl is a build for darwin/arm64, the index lists it for linux/amd64")
}

func TestManifestDisagreesWhenListingTwoPlatforms(t *testing.T) {
	index := mustReadIndex(t, validIndex)
	manifest := mustReadManifest(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]
  command  = ["xcl-plugin-widget"]

  platform "linux" "amd64" {}
  platform "linux" "arm64" {}
}
`)

	err := manifest.agrees(index, linuxAmd64)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "xcl-plugin.xcl lists 2 platforms, a build is for exactly one")
}

func TestValidateRefusesAssetInADirectory(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "../evil.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, `names asset "../evil.zip", which is not a file name`)
}

func TestValidateRefusesAssetWithABackslash(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = "sub\\evil.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "which is not a file name")
}

func TestValidateRefusesDotDotAsAsset(t *testing.T) {
	index := mustReadIndex(t, `
plugin {
  name    = "xcl-plugin-widget"
  version = "1.0.0"
}

executable {
  protocol = [1]

  platform "linux" "amd64" {
    asset    = ".."
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }
}
`)

	err := index.validate(releaseRepository, releaseTag)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "which is not a file name")
}
