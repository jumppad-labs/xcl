---
created_date: "2026-10-31"
document_status: draft
specs:
    - 20261031180836-7529c085-plugin-release-contract-v2
---

# Plugin release asset contract, version 2

How an xcl plugin's GitHub release is laid out. A plugin written in any language ships as native executables for each platform, as a container image, or both. The release tooling publishes exactly this and the GitHub registry installs exactly this. Neither side guesses.

This replaces `plugin-release-assets.md` outright. The registry no longer installs that layout. The existing plugins, `xcl-plugin-docker` and `xcl-plugin-test`, are rebuilt from the updated plugin template and re-released under this contract.

How the host and plugin talk to each other is a separate design. This contract only records which protocol versions a package speaks.

## Release and tag

- A release is a GitHub release whose tag is the plugin's version, `v<major>.<minor>.<patch>`, with an optional pre-release suffix (`v1.2.0-rc.1`). An application pins the tag exactly, and drafts are never installed.
- The plugin's name is the repository name, for example `xcl-plugin-postgres`. `<version>` below is the tag without its leading `v`.

## Assets

```
<name>_<version>_index.xcl          which packages the release offers, with their checksums
<name>_<version>_index.xcl.sig      optional detached OpenPGP signature over the index
<name>_<version>_<os>_<arch>.zip    one per platform of the executable
```

A release offers an executable, a container, or both. A container ships no zip, because the index names its image. A release with no index isn't an xcl plugin release, and the registry refuses it.

## The index

`<name>_<version>_index.xcl` is written by the release tooling, never by hand. The registry reads it before downloading anything. It describes every package and verifies every one.

```hcl
plugin {
  name    = "xcl-plugin-postgres"
  version = "1.2.0"
}

executable {
  protocol = [2]

  platform "linux" "amd64" {
    asset    = "xcl-plugin-postgres_1.2.0_linux_amd64.zip"
    checksum = "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
  }

  platform "linux" "arm64" {
    asset    = "xcl-plugin-postgres_1.2.0_linux_arm64.zip"
    checksum = "sha256:…"
  }

  platform "darwin" "arm64" {
    asset    = "xcl-plugin-postgres_1.2.0_darwin_arm64.zip"
    checksum = "sha256:…"
  }

  platform "windows" "amd64" {
    asset    = "xcl-plugin-postgres_1.2.0_windows_amd64.zip"
    checksum = "sha256:…"
  }
}

container {
  protocol = [2]
  image    = "ghcr.io/acme/xcl-plugin-postgres@sha256:…"

  platform "linux" "amd64" {}
  platform "linux" "arm64" {}
}
```

- **`plugin`** is a singleton. `name` and `version` must match the repository and the tag.
- **One block per kind.** `executable` and `container` are each a singleton block type with only the fields that kind has, so a field that doesn't belong is an ordinary xcl error pointing at the line. Each appears at most once, and at least one must appear.
- **`protocol`** lists the plugin protocol versions the package speaks. Both kinds have it.
- **`platform "<os>" "<arch>"`** lists every platform the package supports. A package supports exactly the platforms it lists.
  - `os` is `linux`, `darwin` or `windows`.
  - `arch` is `amd64` or `arm64`.
  - Two platform blocks in one package may not have the same pair of labels.
- **No checksum, no install.** Every executable platform has a `checksum`, and the container's `image` is pinned by digest. The registry refuses an index that leaves out either, before downloading anything.
- **Unknown kinds are skipped.** A registry ignores a top-level block whose type it doesn't know, so a later contract can add kinds and older registries will choose from the ones they do know.

### Executable

- One zip for each listed platform, named by the platform's `asset`.
- **`checksum`** is the zip's SHA-256, written as `sha256:<hex>`. The prefix names the algorithm, so a later contract can add another, and matches how image digests are written.
- The binary can be built from any language: Go and Rust, TypeScript compiled with `bun build --compile` or `deno compile`, or Python frozen with PyInstaller or Nuitka.

### Container

- **`image`** names a multi-platform image as `<repository>@sha256:<hex>`, the digest of its image index. That pins every architecture at once and is the container's checksum. A tag, or a tag without a digest, is refused.
- **Platforms** are always `linux/<arch>`. Docker Desktop on macOS and Windows runs Linux containers, so an arm64 Mac matches `linux/arm64`. A container only lists platforms so the registry can skip an image built without this machine's architecture before pulling it, rather than running it under emulation.
- **`command`** (optional) replaces the image's default arguments. Without it, the image's `ENTRYPOINT` and `CMD` start the plugin.

## Executable zips

Each zip has `xcl-plugin.xcl` at its root. It contains the `plugin` singleton and an `executable` block with the command that starts the plugin and the one platform the zip was built for:

```hcl
plugin {
  name    = "xcl-plugin-postgres"
  version = "1.2.0"
}

executable {
  protocol = [2]
  command  = ["bin/xcl-plugin-postgres"]

  platform "linux" "amd64" {}
}
```

- **`command`** is the path of the binary relative to the zip's root, followed by any arguments. It ends in `.exe` on Windows.
- **The executable bit:** zip entries keep their Unix file mode, and the binary must be marked executable.
- **Other files.** The rest of the zip is the plugin's to lay out, for example a frozen Python plugin's support files. `README.md` and `LICENSE` may sit at the root.
- **No checksum inside.** The manifest carries no checksum of its own zip. The zip's checksum lives in the index.

## Static files

The index and the manifest hold literal values only, with no variables, functions or references. They are read before anything runs, and a signed file must read the same on every machine. Both are parsed as config-only xcl documents against a fixed set of types, and an expression in them is an error.

## Signature

- **`<name>_<version>_index.xcl.sig`** is an ASCII-armoured detached OpenPGP signature over the index. The index holds every zip's checksum and the image's digest, so signing it signs every package in the release.
- **Optional in a release:**
  - An application that supplies trusted keys accepts a release only when the signature verifies against one of them.
  - An application with no trusted keys doesn't check signatures, and trusts the index because it came from the pinned release over TLS.
- **Release-page checksums file.** A release may also attach a `sha256sum`-style checksums file for people to use. The registry never reads it.

## How the registry installs a plugin

1. Read the release for the pinned tag. A release with no index fails the install.
2. Download the index, and the signature when the application trusts keys. Check the signature.
3. Check the index is complete: `name` and `version` match the repository and tag, every executable platform has a `checksum`, and the container's `image` is pinned by digest. Anything missing fails the install.
4. **Choose a package.** Go through the kinds in the application's order, `executable` then `container` by default, and take the first that lists this machine's platform. The container is only chosen when a container runtime is present, and is matched against that runtime's platform. If neither matches, the install fails, naming the platforms the release offers and what this machine has.
5. **Fetch it.** For the executable, download only this platform's zip and check it against its `checksum`. For the container, pull the image by its digest.
6. **Check the zip's manifest against the index:** the same `name`, `version` and `protocol`, and a platform the index lists for the executable. A mismatch fails the install.
7. Cache the result. A cached plugin is checked again every time it starts, as today.

How an application sets its order or restricts kinds belongs to the registry API, not this contract.

## Producing it

- **Zips:** build each platform's binary, zip it with its `xcl-plugin.xcl`, and record the zip's SHA-256.
- **Image:** build and push a multi-platform image with `docker buildx build --platform linux/amd64,linux/arm64 --push`, and record the pushed image index's digest.
- **Index:** write the index from the zips' manifests, their checksums and the image digest.
- **Signing and publishing:** the release workflow signs the index with `gpg --armor --detach-sign` and publishes every asset with `gh release create`. Signing is set up as today.

The plugin template's `make dist` is updated to do this. Any other tool producing the same names and formats works too.

## Moving the existing plugins

- `xcl-plugin-template` gets the new `make dist` and release workflow.
- `xcl-plugin-docker` and `xcl-plugin-test` take the template's changes and each publish a new release under this contract.
- Applications and tests that pin their old tags move to the new ones. Releases under the first contract stay on GitHub, but no registry installs them.

## Depends on

- **Nested blocks named by all their labels.** In a list, xcl identifies a nested block by its full set of labels, so a package can list `linux/amd64` and `linux/arm64`. Fixed in `internal/xcl/gohcl/decode.go`.
- **A literals-only parse** for the index and the manifest. Whether it's a parser option or a check after parsing is left to planning.
- **Tolerating unknown kinds.** Today an unregistered top-level block is refused. The registry must read the index so that one is skipped instead. How is left to planning.

## Not covered

- The plugin protocol and transport, including protocol 2 and whether protocol 1 survives the rebuild. A separate design will revise the protocol 2 section of `libxcl.md`.
- How a container plugin is started, and what it can reach: mounts, network, user. A plugin that needs to reach the host, such as a Docker socket, files outside the working directory, or local processes, ships as an executable until that's designed.
- Source packages run on a runtime installed on the host. Removed for now. A later contract can add them as a new kind, which older registries will skip. During development, a plugin runs from source through the local registry, with any executable path, including a script.
- The types a plugin provides. They aren't in the index. The registry gets them from the running plugin, as today.
- Publishing the same packages to an OCI registry.
