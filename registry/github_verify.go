package registry

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// verifyEntry checks the files of a cache entry, or of an install about to
// become one, offline: the signature over the checksums file against keys
// when any are trusted, the archive against its line in the checksums file,
// and the extracted binary against the archive's copy. Every failure is
// ErrPluginVerification.
func verifyEntry(dir string, names assetNames, keys openpgp.EntityList) error {
	if err := verifyArchive(dir, names, keys); err != nil {
		return err
	}

	return verifyBinary(dir, names)
}

// verifyArchive checks the checksums file kept in dir is signed by one of
// keys, when any are trusted, and the archive matches its SHA-256 in it
func verifyArchive(dir string, names assetNames, keys openpgp.EntityList) error {
	checksums, err := os.ReadFile(filepath.Join(dir, names.checksums))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: release has no checksums file %s", xclerrors.ErrPluginVerification, names.checksums)
	}

	if err != nil {
		return err
	}

	if err := verifySignature(dir, names, checksums, keys); err != nil {
		return err
	}

	sums, err := parseChecksums(checksums)
	if err != nil {
		return fmt.Errorf("%w: %s: %s", xclerrors.ErrPluginVerification, names.checksums, err)
	}

	expected, listed := sums[names.archive]
	if !listed {
		return fmt.Errorf("%w: archive %s is not listed in %s", xclerrors.ErrPluginVerification, names.archive, names.checksums)
	}

	actual, err := fileSHA256(filepath.Join(dir, names.archive))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: archive %s is missing", xclerrors.ErrPluginVerification, names.archive)
	}

	if err != nil {
		return err
	}

	if actual != expected {
		return fmt.Errorf(
			"%w: archive %s does not match its checksum, expected %s, got %s",
			xclerrors.ErrPluginVerification, names.archive, expected, actual,
		)
	}

	return nil
}

// verifySignature checks the detached signature kept in dir verifies the
// checksums file against one of keys. Without keys it checks nothing.
func verifySignature(dir string, names assetNames, checksums []byte, keys openpgp.EntityList) error {
	if len(keys) == 0 {
		return nil
	}

	signature, err := os.Open(filepath.Join(dir, names.signature))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: release is not signed, and trusted keys were supplied", xclerrors.ErrPluginVerification)
	}

	if err != nil {
		return err
	}
	defer signature.Close()

	if _, err := openpgp.CheckArmoredDetachedSignature(keys, bytes.NewReader(checksums), signature, nil); err != nil {
		return fmt.Errorf(
			"%w: signature %s does not verify against any trusted key: %s",
			xclerrors.ErrPluginVerification, names.signature, err,
		)
	}

	return nil
}

// verifyBinary checks the extracted binary in dir is the archive's copy
func verifyBinary(dir string, names assetNames) error {
	var expected string

	err := withArchiveBinary(filepath.Join(dir, names.archive), names, func(binary io.Reader) error {
		hash := sha256.New()
		if _, err := io.Copy(hash, binary); err != nil {
			return err
		}

		expected = hex.EncodeToString(hash.Sum(nil))
		return nil
	})
	if err != nil {
		return err
	}

	actual, err := fileSHA256(filepath.Join(dir, names.binary))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: cached binary %s is missing", xclerrors.ErrPluginVerification, names.binary)
	}

	if err != nil {
		return err
	}

	if actual != expected {
		return fmt.Errorf("%w: cached binary %s does not match its archive", xclerrors.ErrPluginVerification, names.binary)
	}

	return nil
}

// extractBinary writes the plugin binary at the root of the archive at
// archivePath to dest, executable
func extractBinary(archivePath string, names assetNames, dest string) error {
	return withArchiveBinary(archivePath, names, func(binary io.Reader) error {
		file, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}

		if _, err := io.Copy(file, binary); err != nil {
			file.Close()
			return err
		}

		return file.Close()
	})
}

// withArchiveBinary calls read with the contents of the plugin binary at the
// root of the archive at archivePath. Only an entry named exactly as the
// contract names the binary, at the archive root, is read, so no other path
// in the archive is ever used.
func withArchiveBinary(archivePath string, names assetNames, read func(io.Reader) error) error {
	if names.zip {
		return withZipBinary(archivePath, names, read)
	}

	return withTarGzBinary(archivePath, names, read)
}

// withTarGzBinary reads the binary from a gzipped tar
func withTarGzBinary(archivePath string, names assetNames, read func(io.Reader) error) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%w: archive %s is not a gzipped tar: %s", xclerrors.ErrPluginVerification, names.archive, err)
	}
	defer compressed.Close()

	archive := tar.NewReader(compressed)

	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("%w: archive %s cannot be read: %s", xclerrors.ErrPluginVerification, names.archive, err)
		}

		if header.Typeflag == tar.TypeReg && path.Clean(header.Name) == names.binary {
			return read(archive)
		}
	}

	return fmt.Errorf("%w: archive %s has no %s at its root", xclerrors.ErrPluginVerification, names.archive, names.binary)
}

// withZipBinary reads the binary from a zip
func withZipBinary(archivePath string, names assetNames, read func(io.Reader) error) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("%w: archive %s is not a zip: %s", xclerrors.ErrPluginVerification, names.archive, err)
	}
	defer archive.Close()

	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || path.Clean(entry.Name) != names.binary {
			continue
		}

		contents, err := entry.Open()
		if err != nil {
			return fmt.Errorf("%w: archive %s cannot be read: %s", xclerrors.ErrPluginVerification, names.archive, err)
		}
		defer contents.Close()

		return read(contents)
	}

	return fmt.Errorf("%w: archive %s has no %s at its root", xclerrors.ErrPluginVerification, names.archive, names.binary)
}

// fileSHA256 is the lowercase hex SHA-256 of the file at path
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
