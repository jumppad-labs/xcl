---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
designs:
    - source: design
      path: plugin-release-assets.md
    - source: design
      path: plugin-registries.md
epic: 20261009092551-82db0140-plugin-template
---

# Feature: 20261009102148-7d0b205b-github-releases-registry

## Overview

Applications that use xcl plugins today have to build or ship each plugin binary themselves, so there's no simple way to depend on a plugin someone else has published. This feature lets an application install a plugin straight from a GitHub repository's releases: it names the repository and an exact version, and xcl downloads the build for the current platform, checks it, keeps a copy, and starts it. Plugin authors who publish with the plugin template become installable with one line, and application authors can depend on published plugins, including private ones, without a build step.

## Requirements

- [ ] **Install a plugin from a GitHub release**
  Application authors can declare a plugin by GitHub repository and exact version, and xcl uses that release's plugin as an external plugin when the configuration is applied, planned or destroyed.
- [ ] **The right build for the current platform**
  xcl picks the release's build for the operating system and architecture it is running on, supporting Linux, macOS and Windows on amd64 and arm64.
- [ ] **A clear error when there is no build**
  When the release, or a build for the current platform, doesn't exist, xcl reports which repository, version and platform it was looking for, and nothing is applied.
- [ ] **Downloads are checked**
  xcl verifies every downloaded plugin against the release's published checksums before using it, and refuses a plugin that doesn't match.
- [ ] **Signatures checked against keys the application chooses**
  An application can supply the public keys it trusts. When it does, a plugin is used only if its release carries a signature that verifies against one of those keys. When it supplies none, unsigned releases install normally.
- [ ] **Private repositories**
  A plugin can be installed from a private repository when a GitHub token is available to the application. Public repositories need no token.
- [ ] **Downloads are kept and reused**
  A verified plugin is kept in a cache, and the application can choose where the cache is. A version already in the cache is used without contacting GitHub again, so applies work offline.
- [ ] **Works alongside the other registries**
  The GitHub registry can be used together with the local registry in the same application, each providing plugins to the same configuration.
- [ ] **Documentation shows how to install from GitHub**
  The project's guides, README and the documentation site explain how to install a plugin from GitHub releases. They cover pinning a version, trusting keys, private repositories and the cache.

## Constraints

- **Tests follow the repository rules.** testify `require`, Mockery for test doubles, no table-driven tests, and positive and negative cases in separate test functions.
- **Versions are pinned exactly.** A plugin is declared with one exact release version; anything else is rejected.
- **A release must publish checksums.** A release without a checksums file for its builds is refused.
- **Cached plugins are checked before every start.** A cached plugin is verified again against its release's checksums, and against the application's current trusted keys when it supplies any, before it runs.
- **The cache defaults to the xcl home directory.** Unless the application chooses another location, plugins are cached under the xcl home directory, per repository and version.
- **Built to the existing registry design.** The design `plugin-registries.md` (source `design`) governs how registries provide plugins to a configuration; this registry is one of its remote registries.
- **Built to the release asset contract.** The design `plugin-release-assets.md` (source `design`) settles how a plugin release names its builds, checksums and signature. The registry installs exactly what it describes, and the plugin template publishes it.
- **Plugins run as external plugins.** A plugin installed from GitHub runs as a separate program, with the same contract as any other external plugin.

## Acceptance Criteria

- [ ] **A published plugin installs and applies**
  An application that declares a plugin by repository and version, with an empty cache and network access, applies a configuration using that plugin's resources; the next plan reports no changes.
- [ ] **The current platform's build is used**
  On each of linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 and windows/arm64, the plugin kept in the cache and started is the release's build named for that platform.
- [ ] **A missing build is reported clearly**
  Declaring a version that has no release, or a release with no build for the current platform, fails before anything is applied, with an error naming the repository, version and platform.
- [ ] **A tampered download is refused**
  When a downloaded plugin doesn't match the release's checksums, xcl refuses to start it, reports the mismatch, and keeps nothing in the cache.
- [ ] **A signature from an untrusted key is refused**
  With a trusted key supplied, a release signed with a different key, or not signed at all, is refused with an error naming the plugin, and nothing is kept in the cache; a release signed with the trusted key installs.
- [ ] **Unsigned releases install without keys**
  With no trusted key supplied, an unsigned release with valid checksums installs and applies.
- [ ] **A private repository installs with a token**
  A plugin from a private repository installs when a GitHub token is available, and, without a token, fails with an error naming the repository and saying it was not found or needs a token.
- [ ] **A non-exact version is rejected**
  Declaring a plugin with a version range, "latest" or no version fails before anything is downloaded, with an error naming the plugin.
- [ ] **A release without checksums is refused**
  A release that publishes builds but no checksums file is refused with an error naming the plugin and version, and nothing is kept in the cache.
- [ ] **A changed cached plugin is refused**
  When a cached plugin's file no longer matches its release's checksums, or no longer verifies against the application's current trusted keys, it is refused before it runs.
- [ ] **A cached version works offline**
  After a version has been installed once, applying the same configuration with no network access succeeds, using the cached plugin. A different cache location chosen by the application is where the plugin is kept.
- [ ] **The GitHub and local registries work together**
  One application uses a plugin from the local registry and a plugin from the GitHub registry in the same configuration, and both apply.
- [ ] **The documentation covers installing from GitHub**
  The guides and README describe installing from GitHub releases, and the documentation site has a page covering version pinning, trusted keys, private repositories and the cache; the site builds.

## Technical Approach

- Wire the registry in through `xcl.WithRegistry`, beside the local registry.
- Reuse the existing external-plugin starter for installed binaries.
- Keep each release's checksums file, and its signature, in the cache beside the plugin, so a cached plugin can be checked again offline.
- Read the GitHub token from the standard environment variables (`GITHUB_TOKEN`, then `GH_TOKEN`), with an option to pass one explicitly.
- Use a maintained Go OpenPGP library for signature checks rather than shelling out to `gpg`.
- Write cache entries atomically (download to a temporary file, verify, then move into place), so an interrupted download never leaves a usable but unverified plugin.
- Test against a local HTTP server that serves fake releases, so the tests need no network; keep any test against real GitHub opt-in.

## Success Metrics

- **One line to depend on a plugin.** Declaring a published plugin in an application takes a single registration line, with no build step.
- **Fast after first use.** Starting a cached plugin adds no noticeable time to an apply compared with a local plugin binary.

## Non-Goals

- **A hosted registry or marketplace.**
- **Plugin discovery or search.** The application names the repository it wants.
- **Other release hosts.** GitLab, plain HTTP servers and OCI registries are not covered.
- **Clearing or pruning the cache.** Old versions are kept until removed by hand.

