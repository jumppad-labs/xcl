---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Plan: 20261009102148-7d0b205b-github-releases-registry

<!-- Metadata -->
<!-- Created: 2026-10-09T14:40:45+01:00 -->
<!-- Commit: a000ffa81afd341da8e391aef4f58e076138789c -->
<!-- Branch: f-plugin-registries -->
<!-- Repository: github.com/jumppad-labs/xcl -->

## Overview

This plan adds a GitHub Releases plugin registry to xcl: an application declares a plugin by GitHub repository and exact release tag in one line, and xcl downloads the build for the current platform, verifies it against the release's checksums (and, when the application trusts keys, its OpenPGP signature), caches it under the xcl home directory and starts it as an external plugin. It removes the need for applications to build or ship plugin binaries themselves, lets them depend on published and private plugins, and gives plugin authors who publish with the plugin template a one-line install path. It is built to the `plugin-registries.md` and `plugin-release-assets.md` designs, and documented in the repository and on the documentation site.

## Conventions

- **Testing & Mocking (testify `require`, Mockery, no table-driven tests, positive and negative cases in separate functions, tests beside their source, no tests that read repository files)** — every behaviour of the registry and its errors gets its own test function in `registry/` and the root package; no test asserts on README, docs or website text.
- **Shared test helpers live in `internal/testutil`** — the fake GitHub release server and archive/signing helpers are used by both `registry` and root-package tests, so they go in `internal/testutil` taking `testing.TB`, not into either package's `_test.go` files; `testutil` must not import the root package.
- **Shared error types live in the `errors` package, re-exported from the root** — `ErrPluginNotFound`, `ErrPluginVerification` and `*PluginInstallError` follow the sentinel-and-detail convention with pointer receivers and wrapping.
- **Dependencies: prefer the standard library, document third-party reasons, pin versions** — `net/http`, `archive/tar`, `archive/zip`, `crypto/sha256` for everything except OpenPGP; `github.com/ProtonMail/go-crypto` pinned with its reason in the changelog and research.
- **Project structure: public packages at the module top level** — the GitHub registry is added to the existing public `registry` package, not under `/pkg`.
- **Code style (gofmt, go vet, descriptive names, `any`)** — applies to all new Go code.
- **Patterns & architecture: dependency injection, `context.Context`** — the API URL, token, cache directory, keys and (in package tests) the platform are injected through options and fields; HTTP requests carry the `ctx` given to `Plugins`.
- **Development standards: structured logs** — downloads, cache hits and verification are reported as structured `load` log events through `logger.New(emit, ...)`, as the local registry does for discovery.
- **Never modify dependency packages** — the OpenPGP library is used as published, never patched in the module cache.
- **Generate test state with a real apply** — the "next plan reports no changes" and offline tests produce their state by a real first apply, not a hand-written state file.

## Architecture & Design Decisions

The feature is a new remote registry in the existing public `registry` package (repo `xcl`, new files `registry/github*.go`), built to the two referenced designs: `plugin-registries.md` fixes how it plugs into a `Config` (one more `registry.Registry` given with `xcl.WithRegistry`, `Register*` never returns an error, programmer mistakes panic at the call, environment problems surface when plugins load as `*PluginLoadError`), and `plugin-release-assets.md` fixes exactly which release, archive, checksums file and signature it reads. An application writes one line per plugin:

```go
gh := registry.NewGitHub()                                  // Name() == "github.com"
gh.RegisterPlugin("jumppad-labs/xcl-plugin-docker", "v1.2.0")

c, err := xcl.NewConfig(xcl.WithRegistry(local), xcl.WithRegistry(gh))
```

with options fixed at construction, as `registry.NewLocal` does (`registry/local.go:40-70`): `registry.GitHubCacheDir(dir)` (default `$HOME/.xcl/cache/plugins`, the xcl home the parser already defaults its cache under), `registry.GitHubTrustedKeys(armored ...string)`, `registry.GitHubToken(token)` (otherwise `GITHUB_TOKEN`, then `GH_TOKEN`) and `registry.GitHubAPIURL(url)` (default `https://api.github.com`, used by tests' local HTTP server). `RegisterPlugin` panics, naming the plugin, for a repository not in `owner/repo` form or a version that is not one exact release tag (`v<major>.<minor>.<patch>` with an optional pre-release suffix) — an empty version, a range or `latest` alike — because the design classes a malformed single registration as a programmer error reported at the call, before anything is downloaded.

**A download-and-cache wrapper on the local registry.** `registry.GitHub` holds an internal `*registry.Local` and is a superset of it: `Plugins()` delegates to the wrapped local registry, and `RegisterType`/`Types` are pass-throughs, so the local registry's starting, naming, ordering and events are reused rather than rewritten. `RegisterPlugin("owner/repo", "v1.2.0")` validates its arguments and adds an entry to the wrapped local registry (same package, through its unexported `add`, as a `localEntry{plugin: ...}`) whose plugin downloads, caches and verifies before delegating to `registry.Executable`. The wrapper still reports its own `Name()`, `github.com`; the wrapped `Local` is internal and never exposed. The only new code is the release client, the cache and verification.

**Install at start, verify at every start.** `Plugins()` does no network work: through the wrapped local registry it returns one plugin per registration, in registration order, named after the repository (the design's plugin name). The catalog calls each plugin's `Start` once per `Config` and wraps a failure as `*PluginLoadError{Plugin, Registry}` with `load` events naming the plugin (`internal/catalog/catalog.go:673-693`), so installing inside `Start` gives every error — missing release, no build for the platform, missing checksums, checksum mismatch, untrusted signature, private repository without a token — the plugin's name without changing the core. `Start` resolves the cache entry `<cache>/github.com/<owner>/<repo>/<version>/<os>_<arch>/`; on a miss it calls `GET /repos/{owner}/{repo}/releases/tags/{tag}`, downloads the platform archive, the checksums file and (when keys are trusted) the signature through the asset API (works for private repositories with a token), verifies, extracts the binary, and moves the whole entry into place with a single rename, so an interrupted or failed install leaves nothing usable in the cache. Then — on a hit or a fresh install alike — it verifies the entry: the archive's SHA-256 must match its line in the kept checksums file, the signature must verify against a trusted key when any are supplied, and the extracted binary must match the archive's copy. Only then does it delegate to the existing starter, `registry.Executable(cachedBinaryPath).Start(emit)` (`registry/registry.go:89-115`), and return exactly the host that call returns — there is no host wrapper. Every `Plugin.Start` the catalog makes therefore re-checks the cached plugin offline, against the application's *current* trusted keys, before its process starts; the spec's "cached plugins are re-verified before each start" is read as each start of the plugin, not each catalog `Restart` before an operation.

**Errors and dependencies.** Following the shared-errors convention, two sentinels — `ErrPluginNotFound` (no such release, no build for the platform, or a private repository without a token) and `ErrPluginVerification` (missing checksums, mismatch, missing or untrusted signature) — and a detail type `*PluginInstallError{Repository, Version, Platform, Err}` live in the `errors` package with pointer receivers and wrapping, and are re-exported from the root package beside `PluginLoadError`. HTTP uses the standard library; OpenPGP uses `github.com/ProtonMail/go-crypto/openpgp`, pinned in `go.mod` with its reason recorded, per the dependencies convention and the spec. Tests run against a fake release served by `httptest` and built by a shared helper in `internal/testutil` that packs a real fixture plugin binary into the contract's archive layout and signs the checksums with a key generated in the test; a real-GitHub test is opt-in behind an environment variable.

**Why this beats the alternatives.** Fetching in `Plugins()` would report failures against the registry only; a standalone registry would duplicate the local registry's plugin bookkeeping, starting and events, where wrapping it keeps the new code to download, cache and verify; a host wrapper that re-verifies on every catalog `Restart` was dropped at review in favour of this thin download-and-cache wrapper, which verifies on every plugin start; a hash manifest for re-verification would not be covered by the signature; `go-github` and `gpg` add dependencies the standard library and a Go OpenPGP library make unnecessary; and reusing the design's `NewRemote` would conflate GitHub releases with the unbuilt xcl-hosted registry protocol. Documentation lands in `README.md`, `docs/plugins.md` and `CHANGELOG.md` in `xcl`, and a new guide page plus nav entry in `xcl-website`. See `research.md#alternatives-considered-and-rejected` for the evidence.

## Component Breakdown

- **GitHub registry (new, `registry` package)** — the public `registry.GitHub` type and its constructor and options, wrapping an internal `*registry.Local`. Owns registration (validating `owner/repo` and the exact tag, panicking on a malformed call, then adding an installable plugin to the wrapped local registry), the registry's name (`github.com`), and trusted-key parsing and token resolution at load. Implements the existing `registry.Registry` interface by delegating `Plugins` to the wrapped local registry, and passes `RegisterType`/`Types` through to it, so it is a superset of the local registry.
- **Release client (new, internal to the registry)** — talks to the GitHub REST API: reads a release by tag and downloads named assets through the asset endpoint, sending the token when there is one. Translates GitHub's answers into the registry's errors: a missing release or a private repository without a token becomes "not found, or private and needs a token"; a release missing the platform archive or the checksums file becomes the matching install error. Used only by the installer.
- **Asset contract (new, internal to the registry)** — the single place that knows the names `plugin-release-assets.md` fixes: archive name per platform (`.tar.gz`, `.zip` on Windows), checksums and signature file names, the binary's name inside the archive, the tag-to-version rule, and parsing `sha256sum`-format checksums. Used by the installer and the verifier so neither guesses names.
- **Verifier (new, internal to the registry)** — checks a set of files offline: archive hash against the checksums file, the detached OpenPGP signature over the checksums file against the trusted keys when any are supplied, and the extracted binary against the archive's copy. Used before a fresh install is moved into the cache and again on every start of the plugin, cached or fresh.
- **Cache and installer (new, internal to the registry)** — owns the cache layout (per repository, version and platform under the cache directory) and the install transaction: download into a temporary directory beside the entry, verify, extract the binary, then rename into place, so a failed or interrupted install leaves nothing usable. On a hit it does no network work. Reports downloads, cache hits and verification as structured `load` log events.
- **Verified plugin (new, internal to the registry)** — the `registry.Plugin` each `RegisterPlugin` call adds to the wrapped local registry. Its `Start` installs or finds the cache entry, verifies it, and then delegates to `registry.Executable(cachedBinaryPath).Start(emit)`, returning that host unchanged; there is no host wrapper.
- **Install errors (new, `errors` package, re-exported from the root package)** — `ErrPluginNotFound`, `ErrPluginVerification` and `*PluginInstallError` (repository, version, platform, cause). Produced by the installer and verifier; reach the application wrapped in the catalog's existing `*PluginLoadError`, which names the plugin and the `github.com` registry.
- **Fake GitHub release server (new, shared test helper)** — builds a release in the contract's layout from a real fixture plugin binary (archives per platform, checksums, optional signature from a generated key) and serves it, plus the release-by-tag JSON, from a local HTTP server, with switches to drop a platform, the checksums or the signature, tamper with an archive, sign with another key, or require a token. Used by the registry's package tests and the root package's Config tests.
- **Local registry, catalog and Executable starter (existing, unchanged)** — the wrapped local registry keeps its plugin bookkeeping, ordering and events; the catalog keeps calling `Plugins` then `Start`; `registry.Executable` keeps starting binaries over gRPC and its host is restarted by the catalog as any external host is. The GitHub registry plugs in without changes to any of them.
- **Documentation (changed)** — README "Registering plugins", the plugin architecture guide's registries section, the changelog, and a new documentation site guide linked from the registries page and the site nav.

## Data Structures & Interfaces

**Public API, `registry` package.** One new registry type implementing the existing `registry.Registry` interface, with options fixed at construction in the style of `NewLocal`:

```go
// GitHubName is the Name of every GitHub registry
const GitHubName = "github.com"

// GitHub installs plugins from GitHub releases laid out by the plugin
// release asset contract; it wraps a local registry
type GitHub struct { /* options, mutex, local *Local */ }

type GitHubOption func(*GitHub)

func NewGitHub(options ...GitHubOption) *GitHub
func GitHubCacheDir(dir string) GitHubOption           // default $HOME/.xcl/cache/plugins
func GitHubTrustedKeys(armored ...string) GitHubOption // ASCII-armoured OpenPGP public keys
func GitHubToken(token string) GitHubOption            // default $GITHUB_TOKEN, then $GH_TOKEN
func GitHubAPIURL(url string) GitHubOption             // default https://api.github.com

func (g *GitHub) RegisterPlugin(repository, version string)   // "owner/repo", "v1.2.0"; panics when malformed
func (g *GitHub) RegisterType(prototype any, name ...string)   // pass-through to the wrapped Local
func (g *GitHub) Name() string                                 // GitHubName
func (g *GitHub) Types() []Type                                // pass-through: the wrapped Local's types
func (g *GitHub) Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error) // delegates to the wrapped Local
```

`RegisterPlugin` mirrors the local registry's verb with remote arguments, as `plugin-registries.md` requires, and records its plugin in the wrapped local registry; each returned `Plugin` is named after the repository (`xcl-plugin-docker`), the plugin name the asset contract fixes. The wrapped `*Local` is unexported and never returned.

**Shared errors, `errors` package (re-exported from `xcl`).** Sentinel-and-detail, matched with `errors.Is` and `errors.As`, and always reaching the application inside the catalog's existing `*PluginLoadError`:

```go
var ErrPluginNotFound = errors.New("plugin release not found")     // no release, no build for the platform, or private without a token
var ErrPluginVerification = errors.New("plugin failed verification") // no checksums, mismatch, missing or untrusted signature

type PluginInstallError struct {
    Repository string // "owner/repo"
    Version    string // the pinned tag
    Platform   string // "linux/arm64"
    Err        error  // wraps one of the sentinels with the detail
}
func (e *PluginInstallError) Error() string
func (e *PluginInstallError) Unwrap() error
```

**Internal contracts, `registry` package (unexported).**

- `release` — the parts of GitHub's release-by-tag JSON the registry reads: `tag_name`, `draft`, and `assets[]` of `{id, name, url}`. Assets are found by exact name only.
- `assetNames` — computed from repository name, tag and platform: archive, checksums file, signature file and binary name, per `plugin-release-assets.md`.
- `checksums` — the parsed checksums file, archive name to lowercase hex SHA-256.
- `platform` — `{os, arch}` defaulting to `runtime.GOOS`/`runtime.GOARCH`; a field so package tests can select each of the six supported platforms.
- `cacheEntry` — the directory `<cache>/github.com/<owner>/<repo>/<tag>/<os>_<arch>/` holding the archive, the checksums file, the signature when the release has one, and the extracted binary. It is the unit of the atomic install and of every re-verification.
- `githubPlugin` — the `Plugin` added to the wrapped local registry as `localEntry{plugin: ...}`; its `Start` installs or finds, verifies, then returns `Executable(binaryPath).Start(emit)` unchanged.

**Serialization boundaries.** GitHub REST JSON in (release by tag); raw asset bytes in (`Accept: application/octet-stream`); `sha256sum`-format checksums text and an ASCII-armoured detached signature, both kept on disk unchanged. Nothing new is written to xcl state or to the plugin protocol.

**Shared test helper, `internal/testutil`.** A fake release server: `NewFakeGitHubRelease(t, repository, tag, binary)` builds the archives, checksums and (optionally) a signature, serves them under an `httptest.Server`, and exposes switches (omit a platform, omit checksums, omit or foreign-sign the signature, tamper an archive, require a token) plus the generated armoured public key and the server URL.

## Implementation Detail

**Followed patterns.** The GitHub registry wraps the local registry rather than re-implementing it: plugin bookkeeping, registration order, `Plugins`, type registration and starting are the local registry's, reached by delegation. What it adds is written the way the local registry is: a mutex-guarded struct, functional options fixed at construction, `Register*` methods that only record and return nothing, panics prefixed `xcl: registry github.com:` for single-call mistakes, and structured log events emitted from xcl's core source with the registry's name in their metadata. Errors follow the sentinel-and-detail shape already used for `PluginLoadError`. Starting a binary reuses `registry.Executable` unchanged; the local registry, the catalog and `Config` are not touched.

**New pattern: a self-verifying plugin.** The registry introduces a plugin whose `Start` does real work before delegating — resolve or install the cache entry, verify it, then hand off to `registry.Executable(...).Start` — so every start of the plugin is verified. This is the first `registry.Plugin` that is more than a thin starter; it stays private to the registry, a reader sees the whole "install, verify, start" flow in one place, and the host it returns is the ordinary `Executable` host, so nothing depends on how the catalog recognises restartable hosts.

**New pattern: an atomic cache transaction.** Installs never write into a live cache entry. Everything is downloaded into a fresh temporary directory beside the entry, verified there, the binary extracted and made executable, and the directory renamed into place. If the rename finds the entry already present (a concurrent install by another process), the temporary directory is discarded and the existing entry is verified as a normal hit. A failure at any step removes the temporary directory, so a tampered, unsigned or incomplete release leaves nothing in the cache.

**Code shape inside the registry package.** The GitHub registry is split by responsibility into small files rather than one large one: the public type and options; the asset-contract naming and checksums parsing (pure functions, the easiest place to read the contract); the HTTP release client; the verifier; and the cache/installer with the verified plugin. Each is unit-tested in its own test file. The release client is a concrete type pointed at a base URL rather than an interface with a mock: the tests use a real local HTTP server, as the spec asks, so no Mockery mock is introduced.

**Cross-package shape.** Two sentinels and one detail type are added to the shared `errors` package and re-exported from `xcl`. One new third-party module (the maintained OpenPGP library) is added. A reusable fake-release helper is added to `internal/testutil`; it builds a real plugin binary once per test binary run and packs it per platform, so Config-level tests exercise a genuine external plugin installed over HTTP.

**Documentation shape.** README gains an "Installing plugins from GitHub" subsection under plugin registration; the plugin architecture guide adds the GitHub registry beside the local one in its registries section; the changelog gets one entry for this spec; the documentation site gets a new guide page in the "Guides" nav, linked from the registries page, covering version pinning, trusted keys, private repositories and the cache.

## Dependencies

- **Design `plugin-registries.md` from the `design` design source** — the settled registry API this plan implements against: `xcl.WithRegistry`, the `Registry`/`Plugin` interfaces, `Register*` returning nothing, panics for single-call mistakes, load-time `*PluginLoadError`. No change needed.
- **Design `plugin-release-assets.md` from the `design` design source** — the settled release layout the registry installs: tag form, archive, checksums and signature names and formats, binary location, missing-platform behaviour. No change needed.
- **`registry` package (existing)** — `Registry`, `Plugin`, `Executable`, and the local registry (`registry/local.go`), which the GitHub registry wraps: `NewLocal`, `Plugins`, `RegisterType`/`Types` and the unexported `add`/`localEntry`, plus its option and event conventions. Extended with new files; existing code unchanged.
- **`internal/catalog` (existing)** — calls `Plugins` and `Start`, wraps failures as `*PluginLoadError`. Relied on unchanged; its structural `restartable` interface is not relied on (the GitHub registry returns the plain `Executable` host).
- **`plugins` gRPC host (existing)** — `NewGRPCPluginHost`, `Restart`, `Path`; reused through `Executable`. Unchanged.
- **`errors` package and root re-exports (existing)** — gains `ErrPluginNotFound`, `ErrPluginVerification`, `*PluginInstallError`; root package re-exports them beside `PluginLoadError`.
- **`internal/testutil` (existing)** — gains the fake GitHub release server helper.
- **`internal/test_fixtures/plugins/subtypeless` (existing)** — the self-contained external plugin packed into fake releases. Unchanged.
- **`github.com/ProtonMail/go-crypto` (new third-party module)** — OpenPGP key parsing and detached-signature verification; the maintained successor to the deprecated `golang.org/x/crypto/openpgp`. Pinned to an exact version in `go.mod`; reason recorded in the changelog entry.
- **Go standard library** — `net/http` (GitHub REST API), `net/http/httptest` (tests), `archive/tar`, `compress/gzip`, `archive/zip`, `crypto/sha256`, `os` (atomic rename). No other third-party library.
- **GitHub REST API (external service)** — release-by-tag and release-asset endpoints; reached only on a cache miss. Tests use a local fake; one opt-in test may hit real GitHub.
- **`xcl-website` repo** — gains a guide page and a nav entry; its existing build and type check must still pass.
- **Sibling specs in epic `20261009092551-82db0140-plugin-template`** — nothing must land first: this spec has no upstream dependency. The plugin template spec depends on this one and publishes the layout this registry installs; the docker example spec is independent.
- **Prior plan for the config and plugin registries (`20261008071608-eb05cae0-config-and-plugin-registries`)** — already landed; it is what built the `registry` package and `WithRegistry` this plan extends.

## Testing Approach

**Kinds of tests.**

- **Unit tests in the `registry` package** for each internal piece: asset-name computation for all six platforms (`.zip` and `.exe` on Windows), checksums parsing (valid lines, malformed lines, missing entries), signature verification (trusted key, other key, no signature, no keys), registration panics (empty, range, `latest`, malformed repository), token resolution order (`GitHubToken` option, then `GITHUB_TOKEN`, then `GH_TOKEN`), and default and chosen cache locations.
- **Integration tests in the `registry` package against a real local HTTP server** (the shared fake-release helper) for the install path: a fresh install, a cache hit with the server stopped, each failure mode, and the re-check on every start of a cached plugin. The platform is injected so each of the six platforms selects and caches its own build.
- **Config-level integration tests in the root package** that install the self-contained fixture plugin from the fake server through `xcl.WithRegistry` and run real `Apply`, `Diff` and `Destroy`, including alongside a local registry. State for follow-up operations comes from a real first apply.
- **Error-type unit tests in the `errors` package** for the new sentinels and detail type, in the existing `PluginLoadError` test style.
- **One opt-in test against real GitHub**, skipped unless an environment variable names a published plugin repository and tag; it never runs in the default suite.

All tests follow the project's rules: testify `require`, one behaviour per test function with positive and negative cases separated, no table-driven tests, tests beside the code they test, and no test that reads README, docs, website or other repository files. No Mockery mock is needed because the HTTP boundary is exercised with a real server.

**Load-bearing assertions.** The tests guarantee that: a declared plugin installs from an empty cache and its resources apply, and the next plan has no changes; the cached and started binary is the build named for the current platform; a missing release or platform build fails before anything applies, naming repository, version and platform; a tampered archive, a release without checksums, or (with keys trusted) an unsigned or foreign-signed release is refused, leaves nothing in the cache, and names the plugin; without keys an unsigned release installs; a private repository installs with a token and without one fails naming the repository and saying it was not found or needs a token; a non-exact version panics naming the plugin before any request reaches the server; a cache entry whose archive, binary or signature no longer verifies (including after the trusted keys change) is refused before the process starts, on every start of the plugin; a cached version applies with the server stopped, from a chosen cache directory; and GitHub and local registries load together.

**Success metrics.**

- **One line to depend on a plugin** — behavioural: the Config-level tests declare the plugin with a single `RegisterPlugin` call on a default `NewGitHub()` (plus the test-only API URL option) and nothing else; the documented README and site examples are also **Manual — captured in the implementation test plan** (a reviewer confirms the published examples need one registration line and no build step).
- **Fast after first use** — behavioural proxy: a cache-hit test asserts the fake server receives no requests at all; the timing itself is **Manual — captured in the implementation test plan** (compare apply time with a cached GitHub plugin against the same binary registered as a local external plugin).

**Manual reviews.**

- **Manual — captured in the implementation test plan**: run the install on real darwin and windows machines (amd64 and arm64 where available) to confirm the right build is cached and starts, since CI runs linux only.
- **Manual — captured in the implementation test plan**: run the opt-in real-GitHub test against a published, signed plugin release (public and private repository), once one exists from the plugin template spec.
- **Manual — captured in the implementation test plan**: review README, the plugin architecture guide and the changelog entry for accuracy against the shipped API.
- **Manual — captured in the implementation test plan**: build the documentation site and review the new guide page and its nav entry in a browser.

**Deliberate gaps.** No test exercises GitHub's real redirect to asset storage or rate limiting; those are covered only by the opt-in test. Cross-platform behaviour is unit- and integration-tested by injecting the platform, not by running on each OS in CI.

## Milestones & Tasks

### Milestone 1: Install a public plugin from a GitHub release

**What changes**: An application can declare a plugin with one line — a GitHub repository and an exact release tag — and xcl downloads that release's build for the machine it runs on, checks it against the release's published checksums, keeps it in a cache under the xcl home directory (or a directory the application chooses), and starts it as an external plugin. A cached version is reused without contacting GitHub, so applies work offline, and is re-checked every time the plugin is started. Mistakes are reported clearly: a non-exact version fails at the registration line, and a missing release, a release with no build for this platform, a release without checksums or a tampered download fails before anything is applied, naming the plugin, and leaves nothing in the cache. Signatures and private repositories are not handled yet: every release is treated as public and unsigned.

**Validation point**: The registry and Config-level tests pass against the local fake release server: a fresh install applies and the next plan reports no changes; each of the six platforms caches its own build; the failure cases are refused with the named details and an empty cache; a cached version applies with the server stopped; and the GitHub and local registries load together.

#### - [x] Task: Add plugin install errors
**Id:** 817adb3d-9fcb-4ac7-9497-f8d3238a99ca
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds the shared errors a remote registry reports when it cannot install a plugin: one for a release or build that cannot be found, one for a download that fails verification, and a detail type naming the repository, version and platform. They live with the other shared errors and are re-exported from the root package so applications can match them.

*Technical detail:* [context.md#task-add-plugin-install-errors](./context.md#task-add-plugin-install-errors)

**Acceptance criteria**:
- [x] An application can tell "not found" failures from "failed verification" failures with `errors.Is`, through the root package.
- [x] The install error's message names the repository, version and platform, and its detail can be recovered with `errors.As`.

#### - [x] Task: Encode the release asset contract
**Id:** 05fd3a98-1211-4431-a9d0-1e9003af6af9
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Puts every name and format the plugin release asset contract fixes in one place in the registry package: the archive name for each platform, the checksums and signature file names, the binary's name inside the archive, and parsing the `sha256sum`-format checksums file. Every other part of the registry asks this code for names instead of building them.

*Technical detail:* [context.md#task-encode-the-release-asset-contract](./context.md#task-encode-the-release-asset-contract)

**Acceptance criteria**:
- [x] For each of the six supported platforms the archive and binary names match the contract, with `.zip` and `.exe` on Windows and the version taken from the tag without its `v`.
- [x] A well-formed checksums file is read into archive names and hashes, and a malformed one is reported as an error.

#### - [x] Task: Add a fake GitHub release test server
**Id:** cd1af5fa-d5f4-44e4-947d-278aba10afc5
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds a shared test helper that builds a plugin release in the contract's layout from a real plugin binary — per-platform archives, a checksums file and an optional signature from a key generated in the test — and serves it from a local HTTP server that answers like GitHub's release API. Switches let tests drop a platform, the checksums or the signature, tamper with an archive, sign with a different key, require a token, count requests and stop the server.

*Technical detail:* [context.md#task-add-a-fake-github-release-test-server](./context.md#task-add-a-fake-github-release-test-server)

**Acceptance criteria**:
- [x] Tests in more than one package can stand up a fake release of a real plugin binary without network access.
- [x] Each failure the registry must handle can be produced by a switch on the fake release.

#### - [x] Task: Add the GitHub registry type and registration
**Id:** 8e40a580-1a20-470c-8724-dca2bbb2e0dd
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds the public GitHub registry as a wrapper on the existing local registry: its constructor, the cache-directory and API-URL options, `RegisterType` and `Types` passed through to the wrapped local registry, and `RegisterPlugin`, which records a repository and exact release tag. A malformed registration — a repository not in `owner/repo` form, or a version that is empty, a range, `latest` or anything other than one exact tag — panics at the call naming the plugin, as the registry design requires.

*Technical detail:* [context.md#task-add-the-github-registry-type-and-registration](./context.md#task-add-the-github-registry-type-and-registration)

**Acceptance criteria**:
- [x] An application can create a GitHub registry and declare a plugin with one line.
- [x] A version range, `latest` or a missing version fails at the registration line with a message naming the plugin, before anything is downloaded.
- [x] The registry is named `github.com`, declares no Go types unless the application registers them on it (as on a local registry), and defaults its cache to the xcl home directory.

#### - [x] Task: Add the GitHub release client
**Id:** 31088189-7335-46b7-bc3d-4ce392021f48
**Repo:** xcl
**Depends on:**
- 817adb3d-9fcb-4ac7-9497-f8d3238a99ca — Add plugin install errors
- cd1af5fa-d5f4-44e4-947d-278aba10afc5 — Add a fake GitHub release test server
**Execution:** agent

Adds the small HTTP client that reads a release by its tag and downloads named assets from it through GitHub's asset API. It turns a missing release into a "not found" install error, ignores draft releases, and finds assets only by their exact names.

*Technical detail:* [context.md#task-add-the-github-release-client](./context.md#task-add-the-github-release-client)

**Acceptance criteria**:
- [x] A release that exists is read and its assets can be downloaded by name.
- [x] A tag with no release, or a draft release, is reported as not found, naming the repository and version.

#### - [x] Task: Install, cache and verify plugins at start
**Id:** 70fc09bb-1048-42d3-b555-4a7d43599499
**Repo:** xcl
**Depends on:**
- 05fd3a98-1211-4431-a9d0-1e9003af6af9 — Encode the release asset contract
- 8e40a580-1a20-470c-8724-dca2bbb2e0dd — Add the GitHub registry type and registration
- 31088189-7335-46b7-bc3d-4ce392021f48 — Add the GitHub release client
**Execution:** agent

Makes the registry provide working plugins. Each registered plugin, when started, finds its cache entry or installs it — downloading the platform archive and checksums into a temporary directory, verifying the archive, extracting the binary and moving the entry into place in one step — then verifies the entry and hands the binary to the existing external-plugin starter. The registry's plugins are kept and listed by the wrapped local registry. Verification runs on every start of the plugin, cached or fresh, so a changed cached file is refused before it runs.

*Technical detail:* [context.md#task-install-cache-and-verify-plugins-at-start](./context.md#task-install-cache-and-verify-plugins-at-start)

**Acceptance criteria**:
- [x] A plugin installs from an empty cache and starts, and a second start uses the cache without contacting the server.
- [x] The cached and started binary is the build named for the current platform, for each of the six supported platforms.
- [x] A missing platform build, a release without checksums, or a tampered archive is refused naming the plugin, and nothing is left in the cache.
- [x] A cached archive or binary that no longer matches the checksums is refused before the plugin's process starts, on every start.

#### - [x] Task: Apply configurations with plugins from GitHub
**Id:** 6425e0e3-62eb-4a8e-8a6b-469b8da7ac9b
**Repo:** xcl
**Depends on:**
- 70fc09bb-1048-42d3-b555-4a7d43599499 — Install, cache and verify plugins at start
**Execution:** agent

Proves the registry end to end through a `Config`: a configuration using a plugin installed from a fake GitHub release applies, plans with no changes and destroys; a cached version applies with the server stopped from a cache directory the application chose; a failing install fails the operation before anything is applied; and a GitHub registry and a local registry provide plugins to the same configuration.

*Technical detail:* [context.md#task-apply-configurations-with-plugins-from-github](./context.md#task-apply-configurations-with-plugins-from-github)

**Acceptance criteria**:
- [x] With an empty cache, a configuration using a GitHub-installed plugin applies and the next plan reports no changes.
- [x] After one install, the same configuration applies with no network access, using the chosen cache directory.
- [x] A missing release fails the apply before anything is applied, with a plugin load error naming the plugin and the `github.com` registry.
- [x] One configuration uses a plugin from the local registry and one from the GitHub registry, and both apply.

### Milestone 2: Trust signing keys and install from private repositories

**What changes**: An application can supply the public keys it trusts, and from then on a plugin is used only when its release's checksums are signed by one of them — a release signed by another key, or not signed, is refused naming the plugin and kept out of the cache, and cached plugins are re-checked against the application's current keys every time they are started. Without keys, unsigned releases keep installing as before. Plugins can also be installed from private repositories when a GitHub token is available, from an option or the standard `GITHUB_TOKEN`/`GH_TOKEN` variables; without one, the error names the repository and says it was not found or needs a token.

**Validation point**: The signature and token tests pass against the fake release server: trusted-key installs succeed, foreign-key and unsigned releases are refused with an empty cache, changing the trusted keys refuses an already-cached plugin, unsigned releases install without keys, and a token-protected release installs with a token and fails clearly without one.

#### - [x] Task: Check release signatures against trusted keys
**Id:** 1e912b2d-d170-4fc0-83e5-48df9fa8f535
**Repo:** xcl
**Depends on:**
- 70fc09bb-1048-42d3-b555-4a7d43599499 — Install, cache and verify plugins at start
**Execution:** agent

Adds the trusted-keys option and signature checking with a maintained Go OpenPGP library. When keys are supplied, the release's signature over its checksums file is downloaded, kept in the cache and must verify against one of the keys, at install and on every start of the plugin; without keys, signatures are not checked. A key that cannot be read fails the load naming the registry.

*Technical detail:* [context.md#task-check-release-signatures-against-trusted-keys](./context.md#task-check-release-signatures-against-trusted-keys)

**Acceptance criteria**:
- [x] With a trusted key, a release signed by that key installs and a release signed by another key, or unsigned, is refused naming the plugin, with nothing cached.
- [x] With no trusted keys, an unsigned release with valid checksums installs and applies.
- [x] A cached plugin that no longer verifies against the application's current trusted keys is refused before it runs.

#### - [x] Task: Install from private repositories with a token
**Id:** f7f8f11e-5c82-4c93-8b28-963372386a86
**Repo:** xcl
**Depends on:**
- 31088189-7335-46b7-bc3d-4ce392021f48 — Add the GitHub release client
**Execution:** agent

Adds the token option and reads `GITHUB_TOKEN`, then `GH_TOKEN`, when none is given, sending the token on every GitHub request. Without a token, a repository GitHub hides is reported naming the repository and saying it was not found or needs a token. Public repositories keep working with no token.

*Technical detail:* [context.md#task-install-from-private-repositories-with-a-token](./context.md#task-install-from-private-repositories-with-a-token)

**Acceptance criteria**:
- [x] A plugin from a token-protected release installs when a token is given by option or environment variable.
- [x] Without a token, the failure names the repository and says it was not found or needs a token.
- [x] The explicit option wins over `GITHUB_TOKEN`, which wins over `GH_TOKEN`.

#### - [x] Task: Add an opt-in test against real GitHub
**Id:** 9d2c63fe-5500-459b-ae70-2b9b5f473c12
**Repo:** xcl
**Depends on:**
- 1e912b2d-d170-4fc0-83e5-48df9fa8f535 — Check release signatures against trusted keys
- f7f8f11e-5c82-4c93-8b28-963372386a86 — Install from private repositories with a token
**Execution:** agent

Adds one test that installs and starts a real published plugin from github.com, skipped unless environment variables name the repository, tag and (optionally) a trusted key. It gives maintainers a way to check the registry against GitHub's real redirects and assets without adding network access to the default test run.

*Technical detail:* [context.md#task-add-an-opt-in-test-against-real-github](./context.md#task-add-an-opt-in-test-against-real-github)

**Acceptance criteria**:
- [x] The default test run skips the test and makes no network requests.
- [x] With the environment variables set, the test installs the named release into a temporary cache and starts it.

### Milestone 3: Documentation shows how to install plugins from GitHub

**What changes**: Application authors can learn the feature from the project's own documentation: the README and the plugin architecture guide explain declaring a plugin from GitHub, pinning a version, trusting keys, private repositories and the cache, the changelog records the new registry and its dependency, and the documentation site gains a guide page reachable from its navigation and from the registries page.

**Validation point**: The documentation site builds and type-checks, the new page appears in the Guides navigation, and a reviewer confirms the README, guide, changelog and site page match the shipped API.

#### - [x] Task: Document the GitHub registry in the repository
**Id:** bfed6c67-8b75-4675-a8b6-199f7f77c0ed
**Repo:** xcl
**Depends on:**
- 1e912b2d-d170-4fc0-83e5-48df9fa8f535 — Check release signatures against trusted keys
- f7f8f11e-5c82-4c93-8b28-963372386a86 — Install from private repositories with a token
**Execution:** agent

Explains installing plugins from GitHub releases in the README and the plugin architecture guide — the one-line declaration, exact version pinning, trusted keys, private repositories and tokens, and the cache and its location — and adds a changelog entry for this spec recording the new registry, the new errors and the new OpenPGP dependency with its reason.

*Technical detail:* [context.md#task-document-the-github-registry-in-the-repository](./context.md#task-document-the-github-registry-in-the-repository)

**Acceptance criteria**:
- [x] The README shows how to install a plugin from GitHub and covers pinning, trusted keys, private repositories and the cache.
- [x] The plugin architecture guide describes the GitHub registry beside the local one.
- [x] The changelog has an entry for this spec describing the registry and its new dependency.

#### - [x] Task: Add the GitHub registry guide to the documentation site
**Id:** 704f35a6-88a5-4adc-8d8c-a52505009a9d
**Repo:** xcl-website
**Depends on:**
- 1e912b2d-d170-4fc0-83e5-48df9fa8f535 — Check release signatures against trusted keys
- f7f8f11e-5c82-4c93-8b28-963372386a86 — Install from private repositories with a token
**Execution:** agent

Adds a documentation site page on installing plugins from GitHub releases, covering version pinning, trusted keys, private repositories and the cache, and what a plugin author must publish. The page is added to the Guides navigation and linked from the registries page.

*Technical detail:* [context.md#task-add-the-github-registry-guide-to-the-documentation-site](./context.md#task-add-the-github-registry-guide-to-the-documentation-site)

**Acceptance criteria**:
- [x] The site has a page covering version pinning, trusted keys, private repositories and the cache.
- [x] The page is reachable from the Guides navigation and from the registries page.
- [x] The site builds and type-checks.

## Open Questions

- **Does ProtonMail/go-crypto verify the signature the template's release workflow produces (gpg, armoured detached over the checksums) with the template's key type?** Depends on the key algorithm the plugin template spec chooses (RSA or EdDSA are both supported by the library; an unusual algorithm may not be). Only discoverable once a real signed release exists. The implementer verifies with keys generated by the library itself; if the opt-in real-GitHub test later fails on a genuine release, STOP and ask the user rather than loosening verification.

No other implementation-time uncertainties: every other choice is recorded in the plan's decisions.

## Out of Scope

- **A hosted registry or marketplace** — spec non-goal; the design's `registry.NewRemote` for an xcl-hosted registry stays unbuilt.
- **Plugin discovery or search** — spec non-goal; the application names the repository it wants.
- **Other release hosts** — spec non-goal: GitLab, plain HTTP servers and OCI registries are not covered (the API URL option exists for tests and GitHub-compatible endpoints only, and is not documented as a way to use other hosts).
- **Clearing or pruning the cache** — spec non-goal; old versions stay until removed by hand.
- **Version ranges, `latest` and update checks** — the spec requires exact pins; nothing resolves or suggests newer versions.
- **Fetching only the plugins a configuration uses** — left open by `plugin-registries.md`; every registered plugin is installed when plugins load.
- **Connecting to plugins already running elsewhere (`registry.Connect`)** — left open by `plugin-registries.md`.
- **Publishing releases in the contract's layout** — the plugin template spec (`20261009092551-82db0140-plugin-template`) owns the release packaging (`make dist`) and signing; this plan only installs what that layout describes.
- **Rate-limit handling and retries against GitHub** — a failed request fails the load with GitHub's status; no backoff or retry is added.

## Changelog


### 2026-10-10 — Task: Add plugin install errors

**What was done**: Added `ErrPluginNotFound`, `ErrPluginVerification` and `*PluginInstallError{Repository, Version, Platform, Err}` to the shared `errors` package, and re-exported the sentinels and the detail alias from the root package beside `PluginLoadError`. Tests cover the message, `errors.Is` for each sentinel (and not the other), `errors.As` through wraps and through a `PluginLoadError`.

**Deviations**: None

**Files changed**:
- `xcl: errors/plugin_install_error.go`
- `xcl: errors/plugin_install_error_test.go`
- `xcl: config.go`
- `xcl: errors_reexport_test.go`

**Discoveries**: `PluginInstallError.Error()` reads `plugin <repo> <version> for <platform>: <cause>`, omitting empty parts; the catalog's `PluginLoadError` unwraps to a slice, so `errors.Is` reaches the install sentinel through both layers.

### 2026-10-10 — Task: Encode the release asset contract

**What was done**: Added `registry/github_assets.go`, the one place holding the plugin release asset contract: `exactTag`, the `platform` type (`currentPlatform`, `String`, `supported`), `namesFor(repository, tag, platform)` returning the archive, checksums, signature and binary names, and `parseChecksums` for `sha256sum`-format files. Tests cover all six platforms, accepted and rejected tags and checksums parsing success and failure, each in its own function.

**Deviations**: Named the helpers `namesFor` (returning an `assetNames` struct) and `platform.supported()` instead of the context's `assetNames(...)` function and `supportedPlatform(p)`; behaviour as planned.

**Files changed**:
- `xcl: registry/github_assets.go`
- `xcl: registry/github_assets_test.go`

**Discoveries**: None

### 2026-10-10 — Task: Add a fake GitHub release test server

**What was done**: Added `internal/testutil/github_release.go`, a fake GitHub release served by `httptest` that packs a binary into the contract's archives for all six platforms, writes a `sha256sum`-format checksums file and an armoured detached signature from a key generated in the test, and answers the release-by-tag and asset endpoints. Options drop a platform, the checksums or the signature, sign with another key, tamper an archive, require a token or mark a draft; accessors give the URL, request count, last token, public keys, archives and contract names. Added `BuildFixturePlugin`, building the `subtypeless` fixture plugin once per test binary as `xcl-plugin-widget`, and smoke tests for both.

**Deviations**: Pinned `github.com/ProtonMail/go-crypto` at v1.1.6 rather than the newest v1.5.2: v1.5.2 forces upgrades of `golang.org/x/{text,tools,net,sys,sync,crypto,mod}` across the module, v1.1.6 adds only itself. Its indirect `github.com/cloudflare/circl` was raised from v1.3.7 to v1.6.1 to avoid known advisories in older circl releases. Added `LastToken`, `Archive`, `ArchiveName`, `ChecksumsName` and `SignatureName` accessors beyond the context's list, for the token-precedence and platform tests.

**Files changed**:
- `xcl: internal/testutil/github_release.go`
- `xcl: internal/testutil/github_release_test.go`
- `xcl: internal/testutil/plugins.go`
- `xcl: internal/testutil/doc.go`
- `xcl: go.mod`
- `xcl: go.sum`

**Discoveries**: `go get github.com/ProtonMail/go-crypto@latest` bumps much of `golang.org/x/...`; pin an older release when dependency churn matters. The fixture plugin is built into `os.MkdirTemp` once per test binary and that directory is not removed (no test owns it).

### 2026-10-10 — Task: Add the GitHub registry type and registration

**What was done**: Added the public `registry.GitHub` (`registry/github.go`): `NewGitHub`, `GitHubCacheDir` (expands `~/` and env vars), `GitHubAPIURL`, `RegisterPlugin` validating `owner/repo` and the exact tag and panicking `xcl: registry github.com: plugin "<repository>": ...` otherwise, `RegisterType`/`Types` passed through to a wrapped unexported `*Local`, `Name()` returning `github.com`, and `Plugins` delegating to the wrapped local registry. Each registration adds a `githubPlugin` (named after the repository) to the wrapped registry through `add`; its `Start` was a placeholder until the install task. Package doc mentions the GitHub registry.

**Deviations**: `Plugins` delegation (planned for the install task) was added here so the type satisfies `registry.Registry` from the start. The default cache directory is resolved lazily on each use (`cacheRoot`) rather than at construction.

**Files changed**:
- `xcl: registry/github.go`
- `xcl: registry/github_install.go`
- `xcl: registry/github_test.go`
- `xcl: registry/registry.go`

**Discoveries**: None

### 2026-10-10 — Task: Add the GitHub release client

**What was done**: Added `registry/github_client.go`, a small `net/http` client pointed at a base URL: `release` reads `GET /repos/{owner}/{repo}/releases/tags/{tag}` with the GitHub API headers, `download` streams an asset through its API URL with `Accept: application/octet-stream`, and `downloadTo` writes one into a new file. A 404 or a draft release is `ErrPluginNotFound` naming the repository and tag; any other non-2xx status is an error carrying the status and the start of the body. Assets are found only by exact name. `GitHub.client()` builds the client from the registry's API URL.

**Deviations**: None

**Files changed**:
- `xcl: registry/github_client.go`
- `xcl: registry/github_client_test.go`
- `xcl: registry/github.go`

**Discoveries**: None

### 2026-10-10 — Task: Install, cache and verify plugins at start

**What was done**: Gave `githubPlugin` its real `Start` (`registry/github_install.go`): it resolves the cache entry `<cache>/github.com/<owner>/<repo>/<tag>/<os>_<arch>/`; on a miss it reads the release, requires the platform archive (`ErrPluginNotFound` otherwise) and the checksums file (`ErrPluginVerification` otherwise), downloads both into a `.install-*` directory beside the entry, verifies the archive, extracts the binary and renames the directory into place; then, on a hit or a fresh install alike, verifies the entry (archive against the checksums, binary against the archive's copy) and returns `Executable(binary).Start(emit)` unchanged. Every failure is a `*PluginInstallError` naming repository, version and platform. `registry/github_verify.go` holds the offline verifier and extraction, reading only the root entry named as the contract names the binary. Downloads, cache hits and verification are logged as `load` events from xcl's core.

**Deviations**: The archive is verified before extraction as well as with the whole entry after it, so an untrusted archive is never decompressed. An unsupported platform is reported as `ErrPluginNotFound` before any request.

**Files changed**:
- `xcl: registry/github_install.go`
- `xcl: registry/github_verify.go`
- `xcl: registry/github_install_test.go`
- `xcl: registry/github_verify_test.go`

**Discoveries**: Linux refuses to overwrite a running executable (ETXTBSY), so tests that alter a started cached binary delete and rewrite it. A `.install-*` directory left by a killed process is ignored (never a cache hit) but is not cleaned up, in line with the spec's no-pruning non-goal.

### 2026-10-10 — Task: Apply configurations with plugins from GitHub

**What was done**: Added root-package Config tests (`config_github_registry_test.go`) that install the real `xcl-plugin-widget` fixture plugin from the fake release through `xcl.WithRegistry` with one `RegisterPlugin` call: apply from an empty cache then a no-change `Diff`, the computed resource in state, `Destroy`, an offline apply from a chosen cache after the server is stopped (state from the real first apply, no new requests, binary under `<cache>/github.com/acme/xcl-plugin-widget/v1.0.0/<os>_<arch>/`), a missing release failing with a `*PluginLoadError` naming `xcl-plugin-widget` and `github.com` and matching `ErrPluginNotFound` with nothing applied, and a GitHub registry beside a local one in one configuration. Added the `github` and `github_mixed` config fixtures.

**Deviations**: The Config test for an unsigned release installing without trusted keys, planned for the signatures task, was written here since it needs no signature code.

**Files changed**:
- `xcl: config_github_registry_test.go`
- `xcl: internal/test_fixtures/config/github/main.xcl`
- `xcl: internal/test_fixtures/config/github_mixed/main.xcl`

**Discoveries**: Root-package tests that call `isolateHome` must build the fixture plugin first, since the build needs the real module cache under `$HOME`.

### 2026-10-10 — Task: Check release signatures against trusted keys

**What was done**: Added `GitHubTrustedKeys(armored ...string)`; `Plugins` reads the keys with `openpgp.ReadArmoredKeyRing` (a key that cannot be read fails the load, "trusted key N cannot be read", which the catalog reports against the `github.com` registry). With keys trusted, an install requires the release's `<checksums>.sig` and the verifier checks the armoured detached signature over the checksums file with `openpgp.CheckArmoredDetachedSignature` before the archive is trusted, at install and on every start, always with the registry's current keys. Without keys the signature is still downloaded and kept when the release has one, but not checked.

**Deviations**: None

**Files changed**:
- `xcl: registry/github.go`
- `xcl: registry/github_install.go`
- `xcl: registry/github_verify.go`
- `xcl: registry/github_signature_test.go`

**Discoveries**: None

### 2026-10-10 — Task: Install from private repositories with a token

**What was done**: Added `GitHubToken(token)`; `Plugins` resolves the token from the option, else `GITHUB_TOKEN`, else `GH_TOKEN`, and the release client sends `Authorization: Bearer <token>` on every API and asset request (Go drops it on a redirect to asset storage on another host). Without a token, a 404 says the release was not found or the repository is private and needs a GitHub token; with one, a 404 is plain not found and a 401/403 says the token was rejected.

**Deviations**: None

**Files changed**:
- `xcl: registry/github.go`
- `xcl: registry/github_client.go`
- `xcl: registry/github_token_test.go`

**Discoveries**: GitHub answers a private repository without a token with 404, not 401/403, so a missing-release error cannot tell "absent" from "private"; the message names both.

### 2026-10-10 — Task: Add an opt-in test against real GitHub

**What was done**: Added `TestGitHubRegistryInstallsARealRelease` (`registry/github_live_test.go`), skipped unless `XCL_GITHUB_LIVE_PLUGIN=owner/repo@vX.Y.Z` is set; `XCL_GITHUB_LIVE_KEY` optionally names an armoured public key file to trust, and the token comes from the environment as usual. It installs into a temporary cache, starts the plugin, requires at least one type and stops it. A `make test-github-live` target runs it. The default run skips it with no network requests.

**Deviations**: The task's deliverable is itself the test, so the test step wrote no further tests; verification confirmed the default run skips it.

**Files changed**:
- `xcl: registry/github_live_test.go`
- `xcl: Makefile`

**Discoveries**: None

### 2026-10-10 — Task: Document the GitHub registry in the repository

**What was done**: README gains an "Installing plugins from GitHub" subsection after "Registering plugins" (the one-line declaration, exact pinning and its panics, trusted keys, private repositories and tokens, the cache layout, offline use, re-verification on every start, no pruning, and the errors) and a short pointer in the "With plugins" quick start. `docs/plugins.md` describes the GitHub registry beside the local one in "Registries and the catalog": the wrapper on the local registry, install and verification in `Start`, the atomic cache entry, the ordinary `Executable` host, the errors and the release asset contract. `CHANGELOG.md` has an entry for this spec covering the registry, its options, caching, verification, the new errors and the `ProtonMail/go-crypto` dependency with its reason, with no breaking changes.

**Deviations**: None

**Files changed**:
- `xcl: README.md`
- `xcl: docs/plugins.md`
- `xcl: CHANGELOG.md`

**Discoveries**: None

### 2026-10-10 — Task: Add the GitHub registry guide to the documentation site

**What was done**: Added the guide page `src/pages/github-registry.mdx` (declaring a plugin, pinning a version, what a release must contain, trusting signing keys, private repositories, the cache, errors, and a closing link to the registries guide), the "Installing from GitHub" entry in the Guides navigation, and a paragraph linking to it from the registries page's "Several registries and load order" section. `npm run build` builds the page and `astro check` reports no errors or warnings.

**Deviations**: None

**Files changed**:
- `xcl-website: src/pages/github-registry.mdx`
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/registries.mdx`

**Discoveries**: None

### 2026-10-10 — Final verification: example modules

**What was done**: The full `go test ./...` run found the example modules (which require the root module through a local `replace`) missing `go.sum` entries for the new OpenPGP dependency, failing the e2e example tests. `go mod tidy` in each example module adds `github.com/ProtonMail/go-crypto`, `github.com/cloudflare/circl` and `golang.org/x/crypto` as indirect requirements; the whole suite then passes.

**Deviations**: Example module `go.mod`/`go.sum` files changed, which the plan did not list.

**Files changed**:
- `xcl: example/configonly/go.mod`
- `xcl: example/configonly/go.sum`
- `xcl: example/plugin/go.mod`
- `xcl: example/plugin/go.sum`
- `xcl: example/plugin/plugins/docker/go.mod`
- `xcl: example/plugin/plugins/docker/go.sum`
- `xcl: example/prettylog/go.mod`
- `xcl: example/prettylog/go.sum`

**Discoveries**: Adding a dependency to the root `xcl` module requires `go mod tidy` in every example module under `example/`, since they `replace` the root module locally and the e2e tests build them.
