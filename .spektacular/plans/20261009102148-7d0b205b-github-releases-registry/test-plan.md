---
created_date: "2026-10-10"
document_status: final
closed_date: "2026-10-10"
---

# Test plan: 20261009102148-7d0b205b-github-releases-registry

Manual procedures for the success metrics and reviews the plan's Testing Approach marks as manual. Everything else is covered by the automated tests in `registry/github_*_test.go`, `internal/testutil/github_release_test.go`, `errors/plugin_install_error_test.go` and `config_github_registry_test.go` in the `xcl` repository.

## Success metric: one line to depend on a plugin

- **What to measure**: declaring a published plugin takes a single registration line and no build step.
- **How**: open `README.md` ("With plugins" and "Installing plugins from GitHub"), `docs/plugins.md` ("Registries and the catalog") and the site page `/github-registry/` (`xcl-website: src/pages/github-registry.mdx`). In each example, count the lines needed to declare one plugin beyond creating the registry.
- **Expected result**: exactly one `gh.RegisterPlugin("owner/repo", "vX.Y.Z")` line per plugin, with `registry.NewGitHub()` and `xcl.WithRegistry(gh)`, and no instruction to build or download a binary.
- **Who / when**: the reviewer of this change, before merge.

## Success metric: fast after first use

- **What to measure**: an apply using a cached GitHub plugin takes no noticeably longer than the same binary registered as a local external plugin. Threshold: the median of five applies is within 250 ms of the local median for a 20-40 MB plugin.
- **How**: in a scratch program, install a plugin once with `registry.NewGitHub(registry.GitHubCacheDir("/tmp/xcl-cache"))` and `RegisterPlugin(...)`; then time five `Apply` runs of the same configuration with a fresh `Config` each, (a) through the GitHub registry over the warm cache and (b) through `registry.NewLocal().RegisterExternalPlugin("/tmp/xcl-cache/github.com/<owner>/<repo>/<tag>/<os>_<arch>/<name>")`. Use `time` around the program or `time.Since` around `NewConfig` + `Apply`.
- **Expected result**: (a) median minus (b) median ≤ 250 ms; the difference is the per-start re-verification (hashing the archive and the binary).
- **Who / when**: a maintainer, before the first release that ships the GitHub registry.

## Manual review: real darwin and windows machines

- **What to check**: the right build is cached and starts on platforms CI does not run.
- **Where / how**: on darwin/arm64, darwin/amd64, windows/amd64 and windows/arm64 where available, run `make test-github-live` (or `go test -count=1 -run TestGitHubRegistryInstallsARealRelease -v ./registry/` on windows) with `XCL_GITHUB_LIVE_PLUGIN=<owner>/<repo>@<tag>` naming a release with all six builds.
- **Pass**: the test passes, and the temporary cache entry is `<os>_<arch>` for that machine holding `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` and `<name>.exe` on windows).
- **Who / when**: a maintainer with access to those machines, before the first release.

## Manual review: real GitHub releases (public, signed and private)

- **What to check**: the registry works against GitHub's real API, redirects to asset storage and signatures produced by the plugin template's release workflow.
- **Where / how**: once the plugin template spec has published a signed release, run `make test-github-live` with `XCL_GITHUB_LIVE_PLUGIN=<owner>/<repo>@<tag>` and `XCL_GITHUB_LIVE_KEY=<path to the armoured public key>`; then repeat against a private repository's release with `GITHUB_TOKEN` set, and once without it.
- **Pass**: public and private (with a token) installs pass with the key trusted; without the token the private run fails saying the release was not found or the repository needs a token. If the signature does not verify against a genuine release, stop and raise it rather than loosening verification (the plan's open question on key algorithms).
- **Who / when**: a maintainer, after the plugin template's first signed release.

## Manual review: repository documentation

- **What to check**: README "Installing plugins from GitHub" and the quick-start pointer, `docs/plugins.md` "Registries and the catalog" and the `CHANGELOG.md` entry for this spec match the shipped API.
- **Where / how**: compare them with `registry/github.go` (`NewGitHub`, `GitHubCacheDir`, `GitHubTrustedKeys`, `GitHubToken`, `GitHubAPIURL`, `RegisterPlugin`), `errors/plugin_install_error.go` and `config.go` re-exports.
- **Pass**: every option, default (`~/.xcl/cache/plugins`, `GITHUB_TOKEN` then `GH_TOKEN`), error name and behaviour described exists as written; the changelog names the `github.com/ProtonMail/go-crypto` dependency and its reason.
- **Who / when**: the reviewer of this change, before merge.

## Manual review: documentation site page

- **What to check**: the new guide page and its navigation entry.
- **Where / how**: in `xcl-website`, run `npm ci` (fresh checkout), `npm run build`, `make check`, then `npm run preview` and open `/github-registry/` and `/registries/` in a browser, at desktop and phone width.
- **Pass**: the build and check report no errors; "Installing from GitHub" appears under Guides and opens the page; the registries page links to it; the page covers version pinning, trusted keys, private repositories and the cache, and its examples match the shipped API.
- **Who / when**: the reviewer of this change, before merge.
