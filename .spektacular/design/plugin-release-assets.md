---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
spec: 20261009102148-7d0b205b-github-releases-registry
specs:
    - 20261009102148-7d0b205b-github-releases-registry
    - 20261009092551-82db0140-plugin-template
---

# Plugin release asset contract

How an xcl plugin's GitHub release is laid out. The plugin template's release automation publishes exactly this, and the GitHub Releases plugin registry installs exactly this. Neither side guesses.

## Release and tag

- A release is a GitHub release whose tag is the plugin's version, `v<major>.<minor>.<patch>`, with an optional pre-release suffix (`v1.2.0-rc.1`).
- An application pins that tag exactly. Drafts are never installed.
- The plugin's name is the repository name, for example `xcl-plugin-docker`.

## Builds

- **Platforms:** one archive per supported platform.
  - Operating systems: `linux`, `darwin`, `windows`.
  - Architectures: `amd64`, `arm64`.
- **Archive name:** `<name>_<version>_<os>_<arch>.tar.gz`, using `.zip` for `windows`. The `<version>` is the tag without its leading `v`, as in `xcl-plugin-docker_1.2.0_linux_arm64.tar.gz`.
- **Archive contents:**
  - The plugin binary at the archive root, named `<name>`, or `<name>.exe` on Windows.
  - Optionally `README.md` and `LICENSE`.
  - The registry uses only the binary.
- **Missing platforms:** a release may leave a platform out. The registry then reports that the release has no build for the current platform.

## Checksums

- **File:** `<name>_<version>_checksums.txt` lists the SHA-256 of every archive in the release.
- **Format:** one archive per line, as `<hex sha256>  <archive name>` (two spaces), the format `sha256sum` writes.
- **Required:** every release must have it. The registry refuses an archive that isn't listed or doesn't match.

## Signature

- **File:** `<name>_<version>_checksums.txt.sig`, an ASCII-armoured, detached OpenPGP signature over the checksums file.
- **What it covers:** signing the checksums file signs every archive through its checksum, so no archive carries its own signature.
- **Optional in a release:** a release may be unsigned.
  - An application that supplies trusted public keys accepts a release only when this signature verifies against one of them.
  - An application with no trusted keys does not check signatures.

## Where the registry finds them

- **Lookup:** the registry reads the release for the pinned tag, then downloads the current platform's archive, the checksums file and, when it trusts keys, the signature, all by the names above.
- **Private repositories:** these use the same names, fetched with a GitHub token.

## Producing it

GoReleaser produces this layout with its default naming for archives and checksums, plus a `signs` step that runs GPG over the checksums. The template's release automation uses it, with the signing key held as a repository secret. Any other tool producing the same names and formats works too.
