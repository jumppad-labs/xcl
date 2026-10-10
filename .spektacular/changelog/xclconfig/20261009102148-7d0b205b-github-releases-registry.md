---
created_date: "2026-10-10"
document_status: draft
project: xclconfig
spec: 20261009102148-7d0b205b-github-releases-registry
plan: 20261009102148-7d0b205b-github-releases-registry
---

# Install plugins from GitHub releases

xcl can now install plugins straight from GitHub releases. An application declares a plugin with one line, naming the repository and an exact release version, and xcl downloads the build for the machine it runs on, checks it against the release's checksums (and, if the application supplies trusted keys, its signature), keeps it in a local cache and starts it. Cached plugins work offline and are checked again every time they start, and private repositories install with a GitHub token.

> Derived from project xcl (github.com/jumppad-labs/xcl), spec/plan 20261009102148-7d0b205b-github-releases-registry. See the project-level record for the full feature.

## What changed in this repo

- **New GitHub registry** in the `registry` package: `NewGitHub` with the options `GitHubCacheDir`, `GitHubTrustedKeys`, `GitHubToken` and `GitHubAPIURL`, `RegisterPlugin(repository, version)` (panics on a non-exact version or malformed repository), and `RegisterType`/`Types` passed through to a wrapped local registry. Files: `registry/github.go`, `registry/github_assets.go`, `registry/github_client.go`, `registry/github_install.go`, `registry/github_verify.go`; package doc in `registry/registry.go`.
- **New errors**: `ErrPluginNotFound`, `ErrPluginVerification` and `PluginInstallError` in `errors/plugin_install_error.go`, re-exported from the root package in `config.go`.
- **Dependency**: `github.com/ProtonMail/go-crypto` v1.1.6 (OpenPGP, the maintained successor of `golang.org/x/crypto/openpgp`), with `github.com/cloudflare/circl` v1.6.1 indirect.
- **Tests**: unit and integration tests beside each registry file, signature and token tests, root-package Config tests (`config_github_registry_test.go` with fixtures under `internal/test_fixtures/config/github*`), error tests, and a shared fake GitHub release server and fixture-plugin builder in `internal/testutil`. An opt-in live test runs with `make test-github-live`.
- **Documentation**: README "Installing plugins from GitHub" and a quick-start pointer, `docs/plugins.md` "Registries and the catalog", and a `CHANGELOG.md` entry.

## Why

Applications had to build or ship plugin binaries themselves; this lets them depend on published and private plugins in one line, with downloads verified and cached, and gives plugins published with the plugin template a one-line install path.
