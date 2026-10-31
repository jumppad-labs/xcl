package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
