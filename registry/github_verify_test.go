package registry

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// writeTestTarGz writes a gzipped tar holding one file, name, with contents,
// and returns its path
func writeTestTarGz(t *testing.T, name string, contents []byte) string {
	t.Helper()

	var out bytes.Buffer
	compressed := gzip.NewWriter(&out)
	archive := tar.NewWriter(compressed)

	err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(contents)), Typeflag: tar.TypeReg})
	require.NoError(t, err)

	_, err = archive.Write(contents)
	require.NoError(t, err)

	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())

	path := filepath.Join(t.TempDir(), "xcl-plugin-widget_1.0.0_linux_amd64.tar.gz")
	require.NoError(t, os.WriteFile(path, out.Bytes(), 0644))

	return path
}

// writeTestZip writes a zip holding one file, name, with contents, and
// returns its path
func writeTestZip(t *testing.T, name string, contents []byte) string {
	t.Helper()

	var out bytes.Buffer
	archive := zip.NewWriter(&out)

	writer, err := archive.Create(name)
	require.NoError(t, err)

	_, err = writer.Write(contents)
	require.NoError(t, err)

	require.NoError(t, archive.Close())

	path := filepath.Join(t.TempDir(), "xcl-plugin-widget_1.0.0_windows_amd64.zip")
	require.NoError(t, os.WriteFile(path, out.Bytes(), 0644))

	return path
}

func TestExtractBinaryWritesTheBinaryFromATarGz(t *testing.T) {
	archive := writeTestTarGz(t, "xcl-plugin-widget", []byte("widget binary"))
	names := namesFor("acme/xcl-plugin-widget", "v1.0.0", platform{os: "linux", arch: "amd64"})
	dest := filepath.Join(t.TempDir(), "xcl-plugin-widget")

	err := extractBinary(archive, names, dest)
	require.NoError(t, err)

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, []byte("widget binary"), contents)
}

func TestExtractBinaryFromATarGzIsExecutable(t *testing.T) {
	archive := writeTestTarGz(t, "xcl-plugin-widget", []byte("widget binary"))
	names := namesFor("acme/xcl-plugin-widget", "v1.0.0", platform{os: "linux", arch: "amd64"})
	dest := filepath.Join(t.TempDir(), "xcl-plugin-widget")

	err := extractBinary(archive, names, dest)
	require.NoError(t, err)

	info, err := os.Stat(dest)
	require.NoError(t, err)
	require.NotZero(t, info.Mode().Perm()&0100, "mode %s is not executable", info.Mode())
}

func TestExtractBinaryWritesTheBinaryFromAZip(t *testing.T) {
	archive := writeTestZip(t, "xcl-plugin-widget.exe", []byte("widget binary"))
	names := namesFor("acme/xcl-plugin-widget", "v1.0.0", platform{os: "windows", arch: "amd64"})
	dest := filepath.Join(t.TempDir(), "xcl-plugin-widget.exe")

	err := extractBinary(archive, names, dest)
	require.NoError(t, err)

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, []byte("widget binary"), contents)
}

func TestExtractBinaryFromAZipIsExecutable(t *testing.T) {
	archive := writeTestZip(t, "xcl-plugin-widget.exe", []byte("widget binary"))
	names := namesFor("acme/xcl-plugin-widget", "v1.0.0", platform{os: "windows", arch: "amd64"})
	dest := filepath.Join(t.TempDir(), "xcl-plugin-widget.exe")

	err := extractBinary(archive, names, dest)
	require.NoError(t, err)

	info, err := os.Stat(dest)
	require.NoError(t, err)
	require.NotZero(t, info.Mode().Perm()&0100, "mode %s is not executable", info.Mode())
}

func TestExtractBinaryFailsWhenTarGzHasNoBinaryAtItsRoot(t *testing.T) {
	archive := writeTestTarGz(t, "sub/xcl-plugin-widget", []byte("widget binary"))
	names := namesFor("acme/xcl-plugin-widget", "v1.0.0", platform{os: "linux", arch: "amd64"})
	dest := filepath.Join(t.TempDir(), "xcl-plugin-widget")

	err := extractBinary(archive, names, dest)
	require.Error(t, err)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.NoFileExists(t, dest)
}

func TestExtractBinaryFailsWhenZipHasNoBinaryAtItsRoot(t *testing.T) {
	archive := writeTestZip(t, "sub/xcl-plugin-widget.exe", []byte("widget binary"))
	names := namesFor("acme/xcl-plugin-widget", "v1.0.0", platform{os: "windows", arch: "amd64"})
	dest := filepath.Join(t.TempDir(), "xcl-plugin-widget.exe")

	err := extractBinary(archive, names, dest)
	require.Error(t, err)

	require.True(t, errors.Is(err, xclerrors.ErrPluginVerification))
	require.NoFileExists(t, dest)
}

func TestSignatureIssuerReadsTheSigningKeyID(t *testing.T) {
	key, err := openpgp.NewEntity("issuer test", "", "", nil)
	require.NoError(t, err)

	var signature bytes.Buffer
	err = openpgp.ArmoredDetachSign(&signature, key, strings.NewReader("checksums"), nil)
	require.NoError(t, err)

	keyID, ok := signatureIssuer(signature.Bytes())

	require.True(t, ok)
	require.Equal(t, key.PrimaryKey.KeyId, keyID)
}

func TestSignatureIssuerOfSomethingThatIsNotASignatureReportsFalse(t *testing.T) {
	keyID, ok := signatureIssuer([]byte("not an armoured signature"))

	require.False(t, ok)
	require.Zero(t, keyID)
}
