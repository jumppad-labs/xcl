---
created_date: "2026-10-10"
document_status: final
closed_date: "2026-10-10"
---

# GitHub releases plugin registry

## What was built

xcl gained a GitHub Releases plugin registry. An application declares a plugin by GitHub repository and exact release tag in one line, `registry.NewGitHub()` then `RegisterPlugin("owner/repo", "v1.2.0")`, and gives the registry to its `Config` with `xcl.WithRegistry`, beside a local registry or any other. When plugins load, each declared plugin finds its entry in a cache (by default `~/.xcl/cache/plugins/github.com/<owner>/<repo>/<tag>/<os>_<arch>/`, or a directory chosen with `GitHubCacheDir`) or installs it: it reads the release through the GitHub REST API, downloads the build for the current platform and the release's checksums file (and its signature, when it has one) into a temporary directory, verifies the archive, extracts the binary and moves the entry into place in one step. On a cache hit and a fresh install alike, the entry is then verified offline, the archive against the checksums, the binary against the archive's copy and, when the application trusts keys (`GitHubTrustedKeys`), the detached OpenPGP signature over the checksums file against its current keys, before the binary is started as an ordinary external plugin. Private repositories install with a token from `GitHubToken`, `GITHUB_TOKEN` or `GH_TOKEN`.

A non-exact version or malformed repository panics at the registration line, naming the plugin. Install failures reach the application as a `*PluginLoadError` naming the plugin and the `github.com` registry, wrapping a new `*PluginInstallError` (repository, version, platform) that matches the new `ErrPluginNotFound` or `ErrPluginVerification`. The registry wraps a local registry, so the local registry's ordering, events and type declarations are reused; only the release client, the asset-contract naming, the verifier and the cache are new. The project gained a shared fake GitHub release server for tests, Config-level tests that install and apply a real fixture plugin over HTTP, an opt-in test against real GitHub (`make test-github-live`), and the dependency `github.com/ProtonMail/go-crypto` for signature checks.

The README, the plugin architecture guide and the changelog in `xcl`, and a new "Installing from GitHub" guide on the documentation site (linked from the Guides navigation and the registries page) explain declaring, pinning, trusting keys, private repositories and the cache.

## Why it matters

Applications that used xcl plugins had to build or ship each plugin binary themselves. Now they can depend on a published plugin, including a private one, with one line and no build step, and plugin authors who publish with the plugin template become installable that way. Downloads are always checked against the release's checksums, optionally against signing keys the application chooses, and cached plugins are re-checked before every start while working offline.

## Deviations from the plan

- `github.com/ProtonMail/go-crypto` is pinned at v1.1.6 rather than the newest release, because newer releases force upgrades of many `golang.org/x` modules; its indirect `github.com/cloudflare/circl` was raised to v1.6.1.
- Asset-contract helpers are named `namesFor` and `platform.supported()` rather than `assetNames(...)` and `supportedPlatform(p)`.
- `Plugins` delegation was added with the registry type rather than with the install task; the default cache directory is resolved on each use rather than at construction.
- The archive is verified before extraction as well as with the whole entry after it; an unsupported platform is reported as not found before any request.
- The fake release helper gained `LastToken`, `Archive` and contract-name accessors; the Config test for an unsigned release without keys was written with the other Config tests.
