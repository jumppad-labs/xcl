---
created_date: "2026-11-02"
document_status: draft
project: xclconfig
spec: 20261031180836-7529c085-plugin-release-contract-v2
plan: 20261031180836-7529c085-plugin-release-contract-v2
---

# Releases in the v2 layout, and the template renamed xcl-plugin-template-go

A plugin made from the template now publishes releases the current GitHub registry installs: a zip per platform carrying a manifest, and a signed index listing every zip with its checksum. One command, `make release`, still produces the whole release. The template is now called `xcl-plugin-template-go`, and it declares its replace settings once with `ReplaceOn`.

> Derived from project xcl (jumppad-labs/xcl), spec/plan 20261031180836-7529c085-plugin-release-contract-v2. See the project-level record for the full feature.

## What changed

- **Release tooling:**
  - `make dist` builds `<PLUGIN>-<version>-<os>-<arch>.zip` for six platforms, each holding the binary, README, LICENSE and a generated `xcl-plugin.xcl`.
  - It writes `<PLUGIN>-<version>-index.xcl` listing each zip with its `sha256:` checksum.
  - The release workflow signs the index.
  - The tarballs and checksums file are gone.
- **Rename:** the module path, binary name and asset prefix are `xcl-plugin-template-go`, after the repository's new name.
- **Provider:**
  - `NewNoteProvider` hands `ReplaceSettings` to `DefaultChanged` as `ReplaceOn`.
  - `Changed` defers to it, then answers update for any other reported change. Every answer is as before.
  - Two tests cover it.
- **Dependencies and docs:**
  - Builds against xcl v0.13.1, which installs `-`-separated releases.
  - The README describes the new release layout and how to install it.
- First release: **v0.1.0**.

## Why

The GitHub registry installs only the new layout, which works for plugins built in any language. The template is the starting point for new plugins, so it has to publish that layout.
