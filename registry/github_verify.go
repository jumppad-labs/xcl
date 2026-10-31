package registry

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// verifyEntry checks a cache entry offline, as it is on every start: the
// signature over the index against keys when any are trusted, the index
// itself, the zip of the build for host against its checksum in the index,
// the zip's manifest against the index, and the extracted binary against its
// copy in the zip. It returns the command the manifest names, the binary's
// path inside the entry followed by its arguments. Every failure is
// ErrPluginVerification, apart from an index that offers no build for host,
// which is ErrPluginNotFound.
func verifyEntry(dir, repository, tag string, host platform, keys openpgp.EntityList) ([]string, error) {
	index, build, err := checkIndex(dir, repository, tag, host, keys)
	if err != nil {
		return nil, err
	}

	manifest, err := checkBuild(dir, index, build, host)
	if err != nil {
		return nil, err
	}

	command := manifest.Executable.Command
	if err := verifyExtracted(dir, build.Asset, command[0]); err != nil {
		return nil, err
	}

	return command, nil
}

// checkIndex checks the index of the release of repository at tag kept in
// dir: its signature against keys when any are trusted, then that it is
// complete, and returns it with the build it offers for host. Nothing here
// needs the build itself, so an install checks it before downloading one.
func checkIndex(dir, repository, tag string, host platform, keys openpgp.EntityList) (*releaseIndex, chosenBuild, error) {
	name := indexName(repository, tag)

	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, chosenBuild{}, fmt.Errorf("%w: index %s is missing", xclerrors.ErrPluginVerification, name)
	}

	if err != nil {
		return nil, chosenBuild{}, err
	}

	if err := verifySignature(dir, indexSignatureName(repository, tag), data, keys); err != nil {
		return nil, chosenBuild{}, err
	}

	index, err := readIndex(data, name)
	if err != nil {
		return nil, chosenBuild{}, err
	}

	if err := index.validate(repository, tag); err != nil {
		return nil, chosenBuild{}, err
	}

	build, err := index.choose(host)
	if err != nil {
		return nil, chosenBuild{}, err
	}

	return index, build, nil
}

// checkBuild checks the zip of build kept in dir: that it matches its
// checksum in index, that its manifest agrees with index for host, and that
// it holds the binary the manifest's command names. It returns the manifest.
func checkBuild(dir string, index *releaseIndex, build chosenBuild, host platform) (*buildManifest, error) {
	zipPath := filepath.Join(dir, build.Asset)

	sum, err := fileSHA256(zipPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: zip %s is missing", xclerrors.ErrPluginVerification, build.Asset)
	}

	if err != nil {
		return nil, err
	}

	if sum != build.SHA256 {
		return nil, fmt.Errorf(
			"%w: zip %s does not match its checksum in the index, expected sha256:%x, got sha256:%x",
			xclerrors.ErrPluginVerification, build.Asset, build.SHA256, sum,
		)
	}

	var source []byte
	err = withZipEntry(zipPath, build.Asset, manifestName, func(r io.Reader) error {
		source, err = io.ReadAll(r)
		return err
	})
	if err != nil {
		return nil, err
	}

	manifest, err := readManifest(source, build.Asset)
	if err != nil {
		return nil, err
	}

	if err := manifest.agrees(index, host); err != nil {
		return nil, err
	}

	command := manifest.Executable.Command[0]
	if err := withZipEntry(zipPath, build.Asset, command, func(io.Reader) error { return nil }); err != nil {
		return nil, err
	}

	return manifest, nil
}

// verifySignature checks the detached signature named signatureName, kept in
// dir, verifies index against one of keys. Without keys it checks nothing.
func verifySignature(dir, signatureName string, index []byte, keys openpgp.EntityList) error {
	if len(keys) == 0 {
		return nil
	}

	signature, err := os.ReadFile(filepath.Join(dir, signatureName))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: release is not signed, and trusted keys were supplied", xclerrors.ErrPluginVerification)
	}

	if err != nil {
		return err
	}

	_, err = openpgp.CheckArmoredDetachedSignature(keys, bytes.NewReader(index), bytes.NewReader(signature), nil)
	if err == nil {
		return nil
	}

	// a signature made by a key that is not trusted names the key, so the
	// caller can see which key they would need to trust
	if errors.Is(err, pgperrors.ErrUnknownIssuer) {
		if keyID, ok := signatureIssuer(signature); ok {
			return fmt.Errorf(
				"%w: signature %s does not verify against any trusted key: signed by key %016X, which is not trusted",
				xclerrors.ErrPluginVerification, signatureName, keyID,
			)
		}
	}

	return fmt.Errorf(
		"%w: signature %s does not verify against any trusted key: %s",
		xclerrors.ErrPluginVerification, signatureName, err,
	)
}

// signatureIssuer returns the long key ID of the key that made the armoured
// detached signature, from its issuer key ID, or from the low 64 bits of its
// issuer fingerprint when it carries only that. It reports false when the
// signature cannot be read or names no issuer.
func signatureIssuer(signature []byte) (uint64, bool) {
	block, err := armor.Decode(bytes.NewReader(signature))
	if err != nil {
		return 0, false
	}

	read, err := packet.Read(block.Body)
	if err != nil {
		return 0, false
	}

	sig, ok := read.(*packet.Signature)
	if !ok {
		return 0, false
	}

	if sig.IssuerKeyId != nil {
		return *sig.IssuerKeyId, true
	}

	if len(sig.IssuerFingerprint) >= 8 {
		fingerprint := sig.IssuerFingerprint
		return binary.BigEndian.Uint64(fingerprint[len(fingerprint)-8:]), true
	}

	return 0, false
}

// verifyExtracted checks the binary command names, extracted into dir, is
// the copy in the zip asset kept beside it, so a binary changed in the cache
// is never started
func verifyExtracted(dir, asset, command string) error {
	var expected [32]byte
	err := withZipEntry(filepath.Join(dir, asset), asset, command, func(r io.Reader) error {
		hash := sha256.New()
		if _, err := io.Copy(hash, r); err != nil {
			return err
		}

		copy(expected[:], hash.Sum(nil))

		return nil
	})
	if err != nil {
		return err
	}

	actual, err := fileSHA256(filepath.Join(dir, filepath.FromSlash(path.Clean(command))))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: cached binary %s is missing", xclerrors.ErrPluginVerification, command)
	}

	if err != nil {
		return err
	}

	if actual != expected {
		return fmt.Errorf("%w: cached binary %s does not match its copy in zip %s", xclerrors.ErrPluginVerification, command, asset)
	}

	return nil
}

// extractZip extracts every file of the zip at zipPath, named asset, into
// dest, keeping the zip's directories, and marks the binary command names
// executable whatever mode its entry carries. An entry whose path leaves
// dest, or that is neither a file nor a directory, fails verification, and
// no existing file is overwritten.
func extractZip(zipPath, asset, dest, command string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("%w: %s is not a zip: %s", xclerrors.ErrPluginVerification, asset, err)
	}
	defer reader.Close()

	binary := path.Clean(command)

	for _, file := range reader.File {
		if !insideZip(file.Name) {
			return fmt.Errorf("%w: zip %s has entry %q outside its root", xclerrors.ErrPluginVerification, asset, file.Name)
		}

		name := path.Clean(file.Name)
		target := filepath.Join(dest, filepath.FromSlash(name))
		mode := file.Mode()

		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("unable to extract %s from zip %s: %w", file.Name, asset, err)
			}

			continue
		case !mode.IsRegular():
			return fmt.Errorf("%w: zip %s has entry %q that is not a regular file", xclerrors.ErrPluginVerification, asset, file.Name)
		}

		perm := os.FileMode(0644)
		if name == binary || mode.Perm()&0111 != 0 {
			perm = 0755
		}

		if err := extractZipFile(file, target, perm); err != nil {
			return fmt.Errorf("unable to extract %s from zip %s: %w", file.Name, asset, err)
		}
	}

	// the registry, not the zip, decides the binary is executable
	if err := os.Chmod(filepath.Join(dest, filepath.FromSlash(binary)), 0755); err != nil {
		return fmt.Errorf("unable to mark %s executable: %w", command, err)
	}

	return nil
}

// extractZipFile writes the zip entry file to target with perm, creating its
// directory, and refuses to overwrite a file already there
func extractZipFile(file *zip.File, target string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	contents, err := file.Open()
	if err != nil {
		return err
	}
	defer contents.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, contents); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

// withZipEntry calls read with the contents of the regular file name in the
// zip at zipPath, named asset. A zip that cannot be read, or has no such
// file, fails verification.
func withZipEntry(zipPath, asset, name string, read func(io.Reader) error) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("%w: %s is not a zip: %s", xclerrors.ErrPluginVerification, asset, err)
	}
	defer reader.Close()

	want := path.Clean(name)

	for _, file := range reader.File {
		if path.Clean(file.Name) != want || !file.Mode().IsRegular() {
			continue
		}

		contents, err := file.Open()
		if err != nil {
			return fmt.Errorf("%w: zip %s cannot be read: %s", xclerrors.ErrPluginVerification, asset, err)
		}
		defer contents.Close()

		return read(contents)
	}

	return fmt.Errorf("%w: zip %s has no %s", xclerrors.ErrPluginVerification, asset, name)
}

// fileSHA256 is the SHA-256 of the file at path
func fileSHA256(path string) ([32]byte, error) {
	var sum [32]byte

	file, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return sum, err
	}

	copy(sum[:], hash.Sum(nil))

	return sum, nil
}
