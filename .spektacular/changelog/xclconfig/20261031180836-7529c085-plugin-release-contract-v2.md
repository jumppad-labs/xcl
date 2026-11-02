---
created_date: "2026-11-02"
document_status: draft
project: xclconfig
spec: 20261031180836-7529c085-plugin-release-contract-v2
plan: 20261031180836-7529c085-plugin-release-contract-v2
---

# Plugin releases in the v2 layout, and local plugins as commands

The GitHub registry installs plugin releases in a new layout: a signed index describing every package, plus one zip per platform carrying a manifest. It verifies the index before downloading anything, then downloads only this machine's build and checks it, both on install and every time a cached plugin starts. Local plugins can now be registered as a command, a program and its arguments, so a plugin can run from source, for example `python3 plugin.py`.

> Derived from project xcl (jumppad-labs/xcl), spec/plan 20261031180836-7529c085-plugin-release-contract-v2. See the project-level record for the full feature.

## What changed

- **The GitHub registry installs the v2 layout.**
  - An install downloads the release's index, and its signature when there is one, and checks the signature when keys are trusted.
  - It then checks the index is complete, refusing expressions in place of literal values, and chooses this machine's build. Only that zip is downloaded and checked against its `sha256:` checksum and its `xcl-plugin.xcl` manifest.
  - The zip is extracted into a staging directory and moved into the cache in one step.
  - A cached entry is checked again, offline, on every start.
  - Unknown package kinds are skipped. Container entries are validated but not run.
  - A release with no index, or nothing for this machine, fails with `ErrPluginNotFound`. Everything else fails with `ErrPluginVerification`.
  - The tar.gz and checksums-file code is gone.
- **Either separator.** The index is found as `<name>_<version>_index.xcl` or `<name>-<version>-index.xcl`. A release holding both is refused, and the signature's name follows the index's.
- **Local plugins as commands.**
  - `Local.RegisterExternalPlugin(command string, args ...string)` takes a binary alone, or a program and its arguments. A plugin started with arguments is named by its full command line.
  - `Executable` is now private.
  - The CoreService's `LocalRegistry` message carries `external_plugin_commands`, and the old `external_plugins` field is reserved.
- **Handshake cookie.** go-plugin's magic cookie key is now `XCL_PLUGIN`.
- **Tests.**
  - The fake GitHub release server serves the new layout, with a switch for each failure and options for the separator.
  - New tests cover the reader, the installer and the cache, plus Config-level apply and destroy through an installed plugin.
  - A launcher fixture starts plugins through a command.
- **Docs and pins.**
  - `docs/plugins.md` and `docs/plugin-layout.md` describe the new layout and the command form.
  - Template links point at `xcl-plugin-template-go`.
  - The README and `example/remoteplugin` pin the Docker plugin v0.4.1.
- Released as **v0.13.0** and **v0.13.1** (either separator).

## Why

A release layout built around one signed index works for a plugin built in any language, and lets later releases offer new package kinds without breaking older applications. Running a local plugin from a command lets plugins in other languages be tried before any SDK or release exists.

## Upgrading

- Plugin releases in the old layout, with tarballs and a checksums file, no longer install. Pin a release in the new layout, such as Docker v0.4.1 or the test plugin v0.3.1.
- A cache entry left by an old-layout release fails verification. Delete it.
- Code calling `registry.Executable` uses `RegisterExternalPlugin` instead.
