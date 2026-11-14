---
created_date: "2026-11-02"
document_status: final
closed_date: "2026-11-02"
---

# Plugin release contract v2

## What was built

Plugins installed from GitHub releases now use a new release layout:

- `<name>-<version>-index.xcl` (or `_` throughout), the release's index. It is a static xcl file naming the plugin and its version, and listing every package the release offers. For an executable that is each platform's zip with its `sha256:` checksum; for a container, its image pinned by digest.
- `<name>-<version>-<os>-<arch>.zip`, one per platform, holding the binary and an `xcl-plugin.xcl` manifest naming the plugin, its version, its protocol, the command that starts it and its platform.
- An optional detached OpenPGP signature over the index. Because the index holds every zip's checksum, this one signature vouches for the whole release.

Across the repositories:

- **xcl, the GitHub registry.** It reads only the index and its signature first, and checks the signature when keys are trusted. It checks the index is complete: the name and version match, every platform has a checksum, a container is pinned by digest and lists only Linux, and every value is a literal. Only then does it download this machine's zip, which it checks against its checksum and against its manifest before extracting it.
  - A release without an index, an incomplete index, a tampered zip or a manifest that disagrees is refused before anything is cached.
  - Package kinds it doesn't know are skipped. Container entries are validated but not run.
  - A release offering nothing this machine runs fails, naming the platforms it offers and this one.
  - A cached plugin is checked again on every start.
  - The index is found under either separator, `_` or `-`.
- **xcl, other changes in the same releases.**
  - Local plugins can be registered as a command, a program and its arguments, as well as a binary path. The core service carries commands in a new `external_plugin_commands` field.
  - go-plugin's handshake cookie is now `XCL_PLUGIN`.
  - xcl released this as **v0.13.0** and **v0.13.1**.
- **Plugin template**, renamed `xcl-plugin-template-go`.
  - `make dist` builds the zips and the index, and the release workflow signs the index.
  - It declares its replace settings with `DefaultChanged`'s `ReplaceOn`.
  - First release: **v0.1.0**.
- **Docker plugin and test plugin.** Both took the template's release tooling and were re-released as Docker **v0.4.1** and test plugin **v0.3.1**. The Docker providers declare their replace settings with `ReplaceOn` and behave exactly as before.
- **xcl-python**, on the engine v0.13.1 bump.
  - `register_external_plugin` takes a path or a list holding a program and its arguments.
  - The protocol stubs are regenerated.
  - The integration tests install the test plugin v0.3.1. The tamper tests tamper with the index and the zip separately.
  - The README pins Docker v0.4.1.
- **Documentation, in the xcl repository and on the website:**
  - It describes the new layout, the cache and the errors.
  - It pins the new releases.
  - It shows `ReplaceOn`.

## Why it matters

It is the first step towards plugins written in any language. The release layout no longer assumes a Go binary in a tarball. One signed index describes native builds and, later, container images, and an install fetches and verifies only what the machine needs. Running a local plugin from a command, such as `python3 plugin.py`, lets a plugin written in another language be tried from source before any SDK or release exists.

## Deviations from the plan

- **Either separator.** The design was amended: asset names may use `-` or `_`, the author's choice, and the registry finds the index under either. The template and both plugins use `-`, which needed xcl v0.13.1.
- **The template was renamed** to `jumppad-labs/xcl-plugin-template-go`. Links, module path, binary and asset names followed.
- **Release versions.** The Docker plugin's v0.4.0 and the test plugin's v0.3.0 were tagged from a stale local `main` and published in the old layout. They were re-released as v0.4.1 and v0.3.1, which every pin uses.
- **`Changed` in the template and the Docker plugin** defers to `DefaultChanged` first, then maps a reported change with no other answer to `Update`. Keeping the old "any change → update" rule ahead of the deferral would have hidden the `ReplaceOn` settings.
- **Command registration in xcl ran early.** It shipped in v0.13.0, which retired the old `external_plugins` field. xcl's PRs merged with `breaks-bindings`. The Python fix and the Python command support landed together on xcl-python's engine bump (PR #26, `engine/v0.13.1`), not on a later branch.
- **The handshake-cookie criterion** was reworded with approval: the `libxcl.md` design still quotes the old cookie and is revised by the protocol 2 design.
- **The website's `ReplaceOn` example** quotes the released Docker plugin's providers. xcl's in-repo Docker example still writes its replace loops by hand.
- **Known follow-up:** the Docker plugin's README still pins v0.4.0, and should pin v0.4.1.
