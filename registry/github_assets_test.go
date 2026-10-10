package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testRepository = "jumppad-labs/xcl-plugin-docker"
	testTag        = "v1.2.0"

	testSumOne = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testSumTwo = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func TestNamesForLinuxAmd64ReturnsTarGzArchive(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "linux", arch: "amd64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_linux_amd64.tar.gz", names.archive)
	require.Equal(t, "xcl-plugin-docker", names.binary)
	require.False(t, names.zip)
}

func TestNamesForLinuxArm64ReturnsTarGzArchive(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "linux", arch: "arm64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_linux_arm64.tar.gz", names.archive)
	require.Equal(t, "xcl-plugin-docker", names.binary)
	require.False(t, names.zip)
}

func TestNamesForDarwinAmd64ReturnsTarGzArchive(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "darwin", arch: "amd64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_darwin_amd64.tar.gz", names.archive)
	require.Equal(t, "xcl-plugin-docker", names.binary)
	require.False(t, names.zip)
}

func TestNamesForDarwinArm64ReturnsTarGzArchive(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "darwin", arch: "arm64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_darwin_arm64.tar.gz", names.archive)
	require.Equal(t, "xcl-plugin-docker", names.binary)
	require.False(t, names.zip)
}

func TestNamesForWindowsAmd64ReturnsZipArchiveAndExeBinary(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "windows", arch: "amd64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_windows_amd64.zip", names.archive)
	require.Equal(t, "xcl-plugin-docker.exe", names.binary)
	require.True(t, names.zip)
}

func TestNamesForWindowsArm64ReturnsZipArchiveAndExeBinary(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "windows", arch: "arm64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_windows_arm64.zip", names.archive)
	require.Equal(t, "xcl-plugin-docker.exe", names.binary)
	require.True(t, names.zip)
}

func TestNamesForReturnsChecksumsName(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "linux", arch: "amd64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_checksums.txt", names.checksums)
}

func TestNamesForReturnsSignatureName(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "linux", arch: "amd64"})

	require.Equal(t, "xcl-plugin-docker_1.2.0_checksums.txt.sig", names.signature)
}

func TestNamesForTakesPluginNameFromRepositoryPart(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "linux", arch: "amd64"})

	require.Equal(t, "xcl-plugin-docker", names.plugin)
}

func TestNamesForStripsLeadingVFromTagForVersion(t *testing.T) {
	names := namesFor(testRepository, testTag, platform{os: "linux", arch: "amd64"})

	require.Equal(t, "1.2.0", names.version)
}

func TestNamesForPreReleaseTagKeepsSuffixInVersion(t *testing.T) {
	names := namesFor(testRepository, "v1.2.0-rc.1", platform{os: "linux", arch: "amd64"})

	require.Equal(t, "1.2.0-rc.1", names.version)
	require.Equal(t, "xcl-plugin-docker_1.2.0-rc.1_linux_amd64.tar.gz", names.archive)
}

func TestPlatformStringReturnsOSSlashArch(t *testing.T) {
	require.Equal(t, "linux/arm64", platform{os: "linux", arch: "arm64"}.String())
}

func TestPlatformSupportedTrueForLinuxAmd64(t *testing.T) {
	require.True(t, platform{os: "linux", arch: "amd64"}.supported())
}

func TestPlatformSupportedTrueForWindowsArm64(t *testing.T) {
	require.True(t, platform{os: "windows", arch: "arm64"}.supported())
}

func TestPlatformSupportedFalseForUnsupportedOS(t *testing.T) {
	require.False(t, platform{os: "freebsd", arch: "amd64"}.supported())
}

func TestPlatformSupportedFalseForUnsupportedArch(t *testing.T) {
	require.False(t, platform{os: "linux", arch: "386"}.supported())
}

func TestPlatformCurrentPlatformIsNotEmpty(t *testing.T) {
	current := currentPlatform()

	require.NotEmpty(t, current.os)
	require.NotEmpty(t, current.arch)
}

func TestExactTagAcceptsReleaseTag(t *testing.T) {
	require.True(t, exactTag.MatchString("v1.2.0"))
}

func TestExactTagAcceptsPreReleaseTag(t *testing.T) {
	require.True(t, exactTag.MatchString("v1.2.0-rc.1"))
}

func TestExactTagRejectsMissingLeadingV(t *testing.T) {
	require.False(t, exactTag.MatchString("1.2.0"))
}

func TestExactTagRejectsMissingPatch(t *testing.T) {
	require.False(t, exactTag.MatchString("v1.2"))
}

func TestExactTagRejectsLatest(t *testing.T) {
	require.False(t, exactTag.MatchString("latest"))
}

func TestExactTagRejectsTildeRange(t *testing.T) {
	require.False(t, exactTag.MatchString("~1.2"))
}

func TestExactTagRejectsComparisonRange(t *testing.T) {
	require.False(t, exactTag.MatchString(">=1.0.0"))
}

func TestExactTagRejectsEmpty(t *testing.T) {
	require.False(t, exactTag.MatchString(""))
}

func TestParseChecksumsReadsWellFormedFile(t *testing.T) {
	data := []byte(testSumOne + "  plugin_1.2.0_linux_amd64.tar.gz\n" +
		testSumTwo + "  plugin_1.2.0_windows_amd64.zip\n")

	sums, err := parseChecksums(data)

	require.NoError(t, err)
	require.Len(t, sums, 2)
	require.Equal(t, testSumOne, sums["plugin_1.2.0_linux_amd64.tar.gz"])
	require.Equal(t, testSumTwo, sums["plugin_1.2.0_windows_amd64.zip"])
}

func TestParseChecksumsAcceptsBinaryModeMarker(t *testing.T) {
	data := []byte(testSumOne + " *plugin_1.2.0_linux_amd64.tar.gz\n")

	sums, err := parseChecksums(data)

	require.NoError(t, err)
	require.Equal(t, testSumOne, sums["plugin_1.2.0_linux_amd64.tar.gz"])
}

func TestParseChecksumsAcceptsTrailingBlankLines(t *testing.T) {
	data := []byte(testSumOne + "  plugin_1.2.0_linux_amd64.tar.gz\n\n\n")

	sums, err := parseChecksums(data)

	require.NoError(t, err)
	require.Len(t, sums, 1)
}

func TestParseChecksumsLowercasesUppercaseHex(t *testing.T) {
	upper := "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"
	lower := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

	sums, err := parseChecksums([]byte(upper + "  plugin_1.2.0_linux_amd64.tar.gz\n"))

	require.NoError(t, err)
	require.Equal(t, lower, sums["plugin_1.2.0_linux_amd64.tar.gz"])
}

func TestParseChecksumsOmitsNameNotInFile(t *testing.T) {
	sums, err := parseChecksums([]byte(testSumOne + "  plugin_1.2.0_linux_amd64.tar.gz\n"))

	require.NoError(t, err)

	_, found := sums["plugin_1.2.0_darwin_arm64.tar.gz"]
	require.False(t, found)
}

func TestParseChecksumsRejectsLineWithoutSeparator(t *testing.T) {
	_, err := parseChecksums([]byte("nothingtoseparatehere\n"))

	require.Error(t, err)
}

func TestParseChecksumsRejectsShortHash(t *testing.T) {
	_, err := parseChecksums([]byte("abc123  plugin_1.2.0_linux_amd64.tar.gz\n"))

	require.Error(t, err)
}

func TestParseChecksumsRejectsNonHexHash(t *testing.T) {
	nonHex := "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"

	_, err := parseChecksums([]byte(nonHex + "  plugin_1.2.0_linux_amd64.tar.gz\n"))

	require.Error(t, err)
}
