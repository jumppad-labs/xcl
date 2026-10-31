package registry

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

const testZipAsset = "xcl-plugin-widget_1.0.0_linux_amd64.zip"

// testZipEntry is one file written into a zip by writeTestZip
type testZipEntry struct {
	name string
	mode os.FileMode
	data string
}

// writeTestZip writes a zip holding entries, in order, and returns its path
func writeTestZip(t *testing.T, entries ...testZipEntry) string {
	t.Helper()

	var out bytes.Buffer
	archive := zip.NewWriter(&out)

	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		header.SetMode(entry.mode)

		writer, err := archive.CreateHeader(header)
		require.NoError(t, err)

		_, err = writer.Write([]byte(entry.data))
		require.NoError(t, err)
	}

	require.NoError(t, archive.Close())

	path := filepath.Join(t.TempDir(), testZipAsset)
	require.NoError(t, os.WriteFile(path, out.Bytes(), 0644))

	return path
}

// writeWidgetZip writes a zip laid out as a plugin build: the binary, stored
// with mode 0644, the manifest and a README
func writeWidgetZip(t *testing.T) string {
	t.Helper()

	return writeTestZip(t,
		testZipEntry{name: "xcl-plugin-widget", mode: 0644, data: "widget binary"},
		testZipEntry{name: "xcl-plugin.xcl", mode: 0644, data: "manifest"},
		testZipEntry{name: "README.md", mode: 0644, data: "# widget"},
	)
}

func TestExtractZipWritesTheBinary(t *testing.T) {
	archive := writeWidgetZip(t)
	dest := t.TempDir()

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(dest, "xcl-plugin-widget"))
	require.NoError(t, err)
	require.Equal(t, "widget binary", string(contents))
}

func TestExtractZipWritesTheManifest(t *testing.T) {
	archive := writeWidgetZip(t)
	dest := t.TempDir()

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(dest, "xcl-plugin.xcl"))
	require.NoError(t, err)
	require.Equal(t, "manifest", string(contents))
}

func TestExtractZipWritesTheReadme(t *testing.T) {
	archive := writeWidgetZip(t)
	dest := t.TempDir()

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	require.Equal(t, "# widget", string(contents))
}

func TestExtractZipMarksTheCommandExecutableWhateverItsEntryMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no executable permission bit")
	}

	archive := writeWidgetZip(t)
	dest := t.TempDir()

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dest, "xcl-plugin-widget"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestExtractZipLeavesOtherFilesNotExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no executable permission bit")
	}

	archive := writeWidgetZip(t)
	dest := t.TempDir()

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&0111, "mode %s is executable", info.Mode())
}

func TestExtractZipKeepsTheZipsDirectories(t *testing.T) {
	archive := writeTestZip(t,
		testZipEntry{name: "bin/xcl-plugin-widget", mode: 0644, data: "widget binary"},
		testZipEntry{name: "xcl-plugin.xcl", mode: 0644, data: "manifest"},
	)
	dest := t.TempDir()

	err := extractZip(archive, testZipAsset, dest, "bin/xcl-plugin-widget")
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(dest, "bin", "xcl-plugin-widget"))
	require.NoError(t, err)
	require.Equal(t, "widget binary", string(contents))
}

func TestExtractZipRefusesAnEntryOutsideTheZipRoot(t *testing.T) {
	archive := writeTestZip(t,
		testZipEntry{name: "xcl-plugin-widget", mode: 0755, data: "widget binary"},
		testZipEntry{name: "../evil", mode: 0644, data: "escaped"},
	)
	parent := t.TempDir()
	dest := filepath.Join(parent, "entry")
	require.NoError(t, os.Mkdir(dest, 0755))

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.Error(t, err)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.NoFileExists(t, filepath.Join(parent, "evil"))
}

func TestExtractZipRefusesAnAbsoluteEntry(t *testing.T) {
	archive := writeTestZip(t,
		testZipEntry{name: "xcl-plugin-widget", mode: 0755, data: "widget binary"},
		testZipEntry{name: "/tmp/evil", mode: 0644, data: "escaped"},
	)

	err := extractZip(archive, testZipAsset, t.TempDir(), "xcl-plugin-widget")
	require.Error(t, err)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
}

func TestExtractZipRefusesToOverwriteAnExistingFile(t *testing.T) {
	archive := writeWidgetZip(t)
	dest := t.TempDir()

	existing := filepath.Join(dest, "README.md")
	require.NoError(t, os.WriteFile(existing, []byte("already here"), 0644))

	err := extractZip(archive, testZipAsset, dest, "xcl-plugin-widget")
	require.Error(t, err)

	contents, err := os.ReadFile(existing)
	require.NoError(t, err)
	require.Equal(t, "already here", string(contents))
}

func TestExtractZipRefusesSomethingThatIsNotAZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), testZipAsset)
	require.NoError(t, os.WriteFile(path, []byte("not a zip"), 0644))

	err := extractZip(path, testZipAsset, t.TempDir(), "xcl-plugin-widget")
	require.Error(t, err)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
}

func TestWithZipEntryReadsTheNamedFile(t *testing.T) {
	archive := writeWidgetZip(t)

	var contents string
	err := withZipEntry(archive, testZipAsset, "xcl-plugin.xcl", func(r io.Reader) error {
		data, err := io.ReadAll(r)
		contents = string(data)
		return err
	})
	require.NoError(t, err)

	require.Equal(t, "manifest", contents)
}

func TestWithZipEntryOfAMissingFileFailsVerification(t *testing.T) {
	archive := writeWidgetZip(t)

	err := withZipEntry(archive, testZipAsset, "missing", func(io.Reader) error { return nil })
	require.Error(t, err)

	require.ErrorIs(t, err, xclerrors.ErrPluginVerification)
	require.ErrorContains(t, err, "has no missing")
}

func TestSignatureIssuerReadsTheSigningKeyID(t *testing.T) {
	key, err := openpgp.NewEntity("issuer test", "", "", nil)
	require.NoError(t, err)

	var signature bytes.Buffer
	err = openpgp.ArmoredDetachSign(&signature, key, strings.NewReader("index"), nil)
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
