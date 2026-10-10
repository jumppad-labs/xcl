---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Context: 20261009102148-7d0b205b-github-releases-registry

## Current State Analysis

- The public `registry` package (`registry/registry.go:22-115`) defines `Registry`, `Type`, `Plugin`, `InProcess` and `Executable`; the only registry today is the local one (`registry/local.go`), which keeps registrations as `[]localEntry{plugin, directory}` appended through the unexported `add` (`registry/local.go:33-36, 144-149`), returns them in order from `Plugins` (`registry/local.go:155-178`), and exposes `RegisterType`/`Types` (`registry/local.go:90-117`); the GitHub registry wraps it. There is no remote registry, no download code and no OpenPGP dependency (`go.mod`).
- `xcl.WithRegistry` (`options.go:22`) adds registries; each `Config` owns an `internal/catalog.Catalog` that calls `Registry.Plugins` once and `Plugin.Start` per plugin on the first operation, wrapping failures as `*PluginLoadError` (`internal/catalog/catalog.go:652-695`), and restarts external hosts before every operation through the structural `restartable` interface (`internal/catalog/catalog.go:81-86, 588-605`); the GitHub registry does not rely on that interface — it returns the plain `Executable` host and verifies on each `Plugin.Start`.
- Shared errors live in `errors/` (`errors/plugin_load_error.go`), re-exported from `config.go:71-75, 120-128`.
- The xcl home is `$HOME/.xcl`; the parser already defaults a module cache to `$HOME/.xcl/cache` (`docs/modules.md:100-115`) and docs use `~/.xcl/plugins` as a plugin directory example (`README.md:467`).
- `internal/test_fixtures/plugins/subtypeless/main.go` is a self-contained external plugin (block type `widget`); tests build plugin binaries with `go build` into temp dirs (`config_plugin_loading_test.go:78-90`, `internal/testutil/programs.go:20-32`).
- Documentation: README "Registering plugins" (`README.md:457-503`), `docs/plugins.md:306` "Registries and the catalog", `CHANGELOG.md` entries headed by spec name; website registries guide `xcl-website:src/pages/registries.mdx`, nav `xcl-website:src/components/Nav.astro:9-31`.

## Per-Task Technical Notes

### Task: Add plugin install errors

**Requirement → repo:** "A clear error when there is no build", "Downloads are checked", "Private repositories" (error shape) → `xcl`.

**File changes:**
- `errors/plugin_install_error.go` (new) — `ErrPluginNotFound`, `ErrPluginVerification` (`errors.New`, doc comments naming the GitHub registry as the producer and `errors.Is` matching); `type PluginInstallError struct{ Repository, Version, Platform string; Err error }` with pointer-receiver `Error()` (`"plugin <repo> <version> for <platform>: <err>"`, omitting empty parts) and `Unwrap() error`. Follow `errors/plugin_load_error.go:1-42`; keep the package's import list to stdlib only (knowledge `conventions/shared-errors-package.md`).
- `errors/plugin_install_error_test.go` (new) — message names repository, version and platform; `errors.Is` for each sentinel through the detail; `errors.As` recovers the detail. One behaviour per test, mirroring `errors/plugin_load_error_test.go:19-50`.
- `config.go:71-75` — after `ErrPluginLoad`, re-export `ErrPluginNotFound` and `ErrPluginVerification` with doc comments.
- `config.go:120-128` — add `PluginInstallError = xclerrors.PluginInstallError` to the alias block.
- `errors_reexport_test.go` — add identity tests for the two new sentinels and the alias, in the file's existing style.

**Complexity:** Low
**Token estimate:** ~15k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Encode the release asset contract

**Requirement → repo:** "The right build for the current platform", "Built to the release asset contract" → `xcl`.

**File changes:**
- `registry/github_assets.go` (new) — unexported `platform{os, arch string}` with `currentPlatform()` from `runtime.GOOS/GOARCH` and `String()` (`"linux/arm64"`); `supportedPlatform(p) bool` (linux/darwin/windows × amd64/arm64); `assetNames(repository, tag string, p platform)` returning plugin name (repo part of `owner/repo`), version (`strings.TrimPrefix(tag, "v")`), archive `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` for windows), checksums `<name>_<version>_checksums.txt`, signature `<checksums>.sig`, binary `<name>` or `<name>.exe`; `parseChecksums(data []byte) (map[string]string, error)` reading `<64 hex>  <name>` lines (tolerate trailing newline and the `*` binary-mode marker `sha256sum -b` writes; reject other malformed lines); `exactTag` regexp `^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?$`.
- `registry/github_assets_test.go` (new) — one test per platform for archive and binary names (six tests plus Windows `.zip`/`.exe` naming), version without `v`, checksums parse success, malformed line error, missing-name lookup, pre-release tag accepted, and separate tests for rejected tags (`1.2.0`, `v1.2`, `latest`, `~1.2`, `>=1.0.0`, empty). No table-driven tests.

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Add a fake GitHub release test server

**Requirement → repo:** test infrastructure for every requirement; spec Technical Approach "Test against a local HTTP server that serves fake releases" → `xcl`.

**File changes:**
- `internal/testutil/github_release.go` (new) — `FakeGitHubRelease` built by `NewFakeGitHubRelease(t testing.TB, repository, tag string, binary string, options ...FakeReleaseOption)`. It packs `binary` into `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` for windows, binary named `<name>.exe` inside) for all six platforms (contents may be the same host binary; tests that start the plugin use the host platform), writes a `sha256sum`-format checksums file and, unless disabled, an ASCII-armoured detached signature with a key generated by `openpgp.NewEntity` from `github.com/ProtonMail/go-crypto/openpgp`. Serves via `httptest.NewServer`: `GET /repos/{owner}/{repo}/releases/tags/{tag}` → JSON `{tag_name, draft, assets:[{id,name,url}]}` where `url` points back at `GET /repos/{owner}/{repo}/releases/assets/{id}`, which requires `Accept: application/octet-stream`. Options: `WithoutPlatform(os, arch)`, `WithoutChecksums()`, `WithoutSignature()`, `SignedByOtherKey()`, `TamperArchive(os, arch)`, `RequireToken(token)` (404 without the matching `Authorization: Bearer`/`token` header, as GitHub does for private repos), `Draft()`. Accessors: `URL()`, `PublicKey() string` (armoured), `OtherPublicKey()`, `Requests() int` (atomic counter), `Close()`. Uses `t.TempDir()`, `t.Cleanup(server.Close)`, `t.Helper()` (knowledge `conventions/shared-test-helpers.md`). Must not import the root `xcl` package or `registry` (it encodes the contract independently, so tests catch a naming drift).
- `internal/testutil/plugins.go` (new) — `BuildFixturePlugin(t testing.TB, moduleRoot string) string` building `internal/test_fixtures/plugins/subtypeless` once per test binary (package-level `sync.Once`, output under `os.MkdirTemp` in the OS temp directory) and named so its repository name matches (`xcl-plugin-widget`).
- `go.mod` / `go.sum` — add `github.com/ProtonMail/go-crypto` at an exact current release (e.g. `v1.1.x`, resolved with `go get github.com/ProtonMail/go-crypto@<version>`), pinned; `go mod tidy`.
- `internal/testutil/doc.go` — mention the fake release helper.

**Complexity:** Medium
**Token estimate:** ~35k tokens
**Agent strategy:** Single agent, sequential: archive/checksum building, then signing, then the HTTP handlers; smoke-tested from the registry tests in later tasks.

### Task: Add the GitHub registry type and registration

**Requirement → repo:** "Install a plugin from a GitHub release" (declaration), "Versions are pinned exactly", "The cache defaults to the xcl home directory" → `xcl`.

**File changes:**
- `registry/github.go` (new) — `const GitHubName = "github.com"`, `DefaultGitHubAPIURL = "https://api.github.com"`; `type GitHub struct{ mu sync.Mutex; local *Local; apiURL, cacheDir, token string; trustedKeys []string; platform platform; httpClient *http.Client }` — the wrapped `*Local` (created with `NewLocal()` in `NewGitHub`, unexported, never returned) holds the registrations; `type GitHubOption func(*GitHub)`; `NewGitHub(options...)` defaulting `cacheDir` to `filepath.Join(home, ".xcl", "cache", "plugins")` (via `os.UserHomeDir`, resolved lazily at load if home is unavailable at construction) and `platform` to `currentPlatform()`; `GitHubCacheDir(dir)` (expands `~/` and env with `expandPluginDirectories`, `registry/discovery.go:149-168`; empty keeps default); `GitHubAPIURL(url)` (trailing `/` trimmed; empty keeps default). `RegisterPlugin(repository, version string)` panics `xcl: registry github.com: plugin "<repository>": ...` for empty/malformed repository (must be exactly `owner/repo`, both non-empty, no extra `/`) and for a version failing `exactTag` (message says a version must be one exact release tag such as `v1.2.0`, not a range or `latest`); a valid call adds `localEntry{plugin: &githubPlugin{...}}` to the wrapped local registry through its unexported `add` (the plugin's `Start` is filled in by the install task). `Name()` returns `GitHubName` (not the wrapped registry's `local`); `RegisterType(prototype any, name ...string)` and `Types()` delegate to the wrapped `Local` (its validation and panics apply unchanged; `Types()` is nil when none are registered). Package doc in `registry/registry.go:1-12` updated to mention the GitHub registry. Follow the style of `registry/local.go:22-140`.
- `registry/github_test.go` (new) — `Name()` is `github.com`; `Types()` nil with nothing registered; a type registered with `RegisterType` is returned by `Types()`; default cache dir under `$HOME/.xcl/cache/plugins` (set `HOME` with `t.Setenv`); chosen cache dir; separate panic tests for empty version, `latest`, a range, `1.2.0` without `v`, empty repository, repository without owner, each asserting the message names the plugin (`require.PanicsWithValue` or recover-and-contains). Plugins() is covered in the install task.

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Add the GitHub release client

**Requirement → repo:** "Install a plugin from a GitHub release" (fetch), "A clear error when there is no build" → `xcl`.

**File changes:**
- `registry/github_client.go` (new) — unexported `releaseClient{baseURL, token string; http *http.Client}`; `release(ctx, repository, tag) (*githubRelease, error)` → `GET {baseURL}/repos/{owner}/{repo}/releases/tags/{tag}` with `Accept: application/vnd.github+json`, `X-GitHub-Api-Version: 2022-11-28`, `User-Agent: xcl`; 404 → `ErrPluginNotFound` wrapped as "no release <tag> in <repository>" (token wording added in the token task); draft → same not-found; other non-2xx → error with status and body excerpt. `githubRelease{TagName string; Draft bool; Assets []githubAsset}`, `githubAsset{ID int64; Name, URL string}`, `asset(name) (githubAsset, bool)`. `download(ctx, asset githubAsset, w io.Writer) error` → `GET asset.URL` with `Accept: application/octet-stream`, following redirects (Go drops `Authorization` on cross-host redirects, which is wanted). Requests built with `http.NewRequestWithContext`.
- `registry/github_client_test.go` (new) — against `testutil.NewFakeGitHubRelease`: reads a release; downloads an asset by name; missing tag → `ErrPluginNotFound` naming repository and version; draft → not found; server error → error containing the status. Client-only tests use any small file as the "binary".

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Install, cache and verify plugins at start

**Requirement → repo:** "Install a plugin from a GitHub release", "The right build for the current platform", "A clear error when there is no build", "Downloads are checked", "Downloads are kept and reused", constraints "A release must publish checksums", "Cached plugins are checked before every start" (read as every `Plugin.Start`), "Plugins run as external plugins", Technical Approach "atomic cache entries", "keep checksums and signature in the cache" → `xcl`.

**File changes:**
- `registry/github_verify.go` (new) — `verifyEntry(dir string, names assetNames, keys openpgp.EntityList) error`: read the kept checksums file (missing → `ErrPluginVerification` "release has no checksums file"); look up the archive's line (missing → verification error "archive not listed"); SHA-256 the archive and compare (mismatch → verification error naming archive, expected and actual); stream the binary out of the archive (`archive/tar`+`compress/gzip`, or `archive/zip`) hashing it, and compare with the SHA-256 of the extracted binary (mismatch → verification error "cached binary does not match its archive"). Signature hook left for the signatures task (`keys` nil → skipped). `extractBinary(archivePath, binaryName, dest string) error` — only the root entry named `binaryName`, reject path traversal, write mode `0755`.
- `registry/github_install.go` (new) — `githubPlugin{registry *GitHub; repository, tag string}` implementing `Plugin`: `Name()` returns the repository's name part. `Start(emit)`: compute `names`; reject unsupported platform; `entry := filepath.Join(cacheDir, "github.com", owner, repo, tag, os+"_"+arch)`; if entry exists → log "using cached plugin" and `verifyEntry`; else `install`: `os.MkdirAll(parent)`, `os.MkdirTemp(parent, ".install-*")`, `defer os.RemoveAll(tmp)`, fetch release (client), require archive asset (missing → `ErrPluginNotFound` "no build for <platform>") and checksums asset (missing → `ErrPluginVerification` "release has no checksums file"), download both, `verifyEntry(tmp, ...)` after `extractBinary`, then `os.Rename(tmp, entry)`; if rename fails because entry now exists, fall through to verifying the existing entry. Every error wrapped in `&xclerrors.PluginInstallError{Repository, Version: tag, Platform}`. Verification runs on every `Start`, cached or fresh. Then `return Executable(binaryPath).Start(emit)` — the host is returned exactly as `Executable` gives it, with no wrapper. Log download/cache-hit/verified messages through `logger.New(emit, events.Event{Source: events.SourceCore, Operation: events.OperationLoad, Meta: {"registry": GitHubName, "plugin": name}})`, as `registry/discovery.go:20-35`.
- `registry/github.go` — `Plugins(ctx, emit)` stores `ctx` for the client calls made in `Start` (fall back to `context.Background()`) and delegates to the wrapped `local.Plugins(ctx, emit)`, which returns one `githubPlugin` per registration in registration order (no network).
- `registry/github_install_test.go` (new) — with the fake release and the host-platform fixture binary: fresh install starts and the entry holds archive, checksums and binary; second `Start` with the server closed succeeds and `Requests()` is unchanged; six tests forcing `platform` to each supported platform and asserting the cached binary file name and that it came from that platform's archive (do not start non-host binaries); missing platform → `ErrPluginNotFound`, message names repository, version, platform, cache entry absent; no checksums → `ErrPluginVerification`, entry absent; tampered archive → `ErrPluginVerification`, entry absent; cached binary overwritten → `Start` fails before starting; cached archive overwritten → fails; after a successful start, altering the cached binary makes the next `Start` of the same plugin fail without starting a process; `Plugins()` returns the registered plugins in registration order through the wrapped local registry; an interrupted install (temp dir left behind) is ignored and does not count as a hit.
- No `TestMain` is added: tests call `testutil.BuildFixturePlugin`, which builds once per test binary.

**Complexity:** High
**Token estimate:** ~60k tokens
**Agent strategy:** Parallel analysis, sequential integration: one agent writes the verifier and extraction with their tests while another writes the cache entry layout and atomic install transaction; a single agent then integrates the plugin's `Start` (delegating to `Executable`) and `Plugins` delegation and runs the full package tests.

### Task: Apply configurations with plugins from GitHub

**Requirement → repo:** acceptance "A published plugin installs and applies", "A missing build is reported clearly", "A cached version works offline", "The GitHub and local registries work together" → `xcl`.

**File changes:**
- `config_github_registry_test.go` (new, root package) — the root package has no `TestMain`; build the fixture plugin through `testutil.BuildFixturePlugin`, whose own `sync.Once` builds it once per test binary. Tests: (1) empty temp cache, `registry.NewGitHub(registry.GitHubAPIURL(fake.URL()), registry.GitHubCacheDir(dir))` + one `RegisterPlugin("acme/xcl-plugin-widget", "v1.0.0")`, `xcl.NewConfig(xcl.WithRegistry(gh), xcl.WithStatePath(t.TempDir()))`, `Apply` a `widget` fixture config, then `Diff` reports no changes; (2) after an install, `fake.Close()`, a new `Config` over the same cache and state applies successfully and `fake.Requests()` did not grow; the binary exists under the chosen cache dir; (3) missing tag → `Apply` returns `*xcl.PluginLoadError` with `Plugin == "xcl-plugin-widget"`, `Registry == "github.com"`, `errors.Is(err, xcl.ErrPluginNotFound)`, state shows nothing applied; (4) local registry with an in-process plugin or declared type providing another block type plus the GitHub registry; a config using both applies. State for (2) comes from the real apply in the same test (knowledge `conventions/test-state-from-real-apply.md`).
- `internal/test_fixtures/config/github/main.xcl` (new) — a `widget "app" { size = 1 part { name = "a" } }` config; and `internal/test_fixtures/config/github_mixed/main.xcl` using both registries' block types.

**Complexity:** Medium
**Token estimate:** ~35k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Check release signatures against trusted keys

**Requirement → repo:** "Signatures checked against keys the application chooses", constraint "Cached plugins are checked … against the application's current trusted keys", Technical Approach "maintained Go OpenPGP library" → `xcl`.

**File changes:**
- `registry/github.go` — `GitHubTrustedKeys(armored ...string) GitHubOption` (appends); in `Plugins`, parse all keys with `openpgp.ReadArmoredKeyRing` into one `openpgp.EntityList`; a parse failure returns an error (catalog wraps it as `*PluginLoadError{Registry: "github.com"}`) naming which key (by index) failed.
- `registry/github_install.go` — when keys are trusted, require the signature asset (missing → `ErrPluginVerification` "release is not signed, and trusted keys were supplied") and download it into the temp entry; when keys are not trusted, download the signature anyway if present so a later run with keys can verify offline (keep it), but do not check it.
- `registry/github_verify.go` — when `keys` is non-empty: read `<checksums>.sig` (missing → verification error "not signed"), `openpgp.CheckArmoredDetachedSignature(keys, checksumsReader, sigReader, nil)`; failure → `ErrPluginVerification` "signature does not verify against any trusted key". Runs on install and on every `Start` of the plugin, cached or fresh, always with the registry's current keys.
- `registry/github_signature_test.go` (new) — trusted key + signed release installs; other key → refused, entry absent, message names plugin; unsigned with keys → refused, entry absent; no keys + unsigned → installs; cached entry installed without keys, then a new registry with a key that did not sign it → refused before start; malformed armoured key → `Plugins` error; signature file altered in the cache → refused.
- `config_github_registry_test.go` — one Config test: with no keys, an unsigned release applies (acceptance "Unsigned releases install without keys").

**Complexity:** Medium
**Token estimate:** ~35k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Install from private repositories with a token

**Requirement → repo:** "Private repositories", Technical Approach "GITHUB_TOKEN, then GH_TOKEN, with an option" → `xcl`.

**File changes:**
- `registry/github.go` — `GitHubToken(token string) GitHubOption`; in `Plugins` resolve the token: option, else `os.Getenv("GITHUB_TOKEN")`, else `os.Getenv("GH_TOKEN")`; pass to the client.
- `registry/github_client.go` — send `Authorization: Bearer <token>` on API and asset requests when set; on 404 with no token, the not-found error reads "<repository> release <tag> was not found, or the repository is private and needs a GitHub token (set GITHUB_TOKEN)"; with a token, a 404 reads "not found" and a 401/403 reads "token was rejected".
- `registry/github_token_test.go` (new) — `RequireToken` release installs with the option; with `GITHUB_TOKEN` (`t.Setenv`); with `GH_TOKEN`; option beats `GITHUB_TOKEN`; `GITHUB_TOKEN` beats `GH_TOKEN` (fake server records the token it saw); no token → `ErrPluginNotFound` with the message naming the repository and mentioning a token. Each test clears both variables first with `t.Setenv(..., "")`.

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Add an opt-in test against real GitHub

**Requirement → repo:** Technical Approach "keep any test against real GitHub opt-in" → `xcl`.

**File changes:**
- `registry/github_live_test.go` (new) — `TestGitHubRegistryInstallsARealRelease`: `t.Skip` unless `XCL_GITHUB_LIVE_PLUGIN` is set to `owner/repo@vX.Y.Z`; optional `XCL_GITHUB_LIVE_KEY` (path to an armoured public key) adds `GitHubTrustedKeys`; token from the environment as usual. Installs into `t.TempDir()`, starts the plugin, asserts the host reports at least one type, stops it.
- `Makefile` — add a `test-github-live` target documenting the variables (optional convenience, no CI change).

**Complexity:** Low
**Token estimate:** ~10k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Document the GitHub registry in the repository

**Requirement → repo:** "Documentation shows how to install from GitHub" (guides, README) → `xcl`.

**File changes:**
- `README.md:457-503` — after the "Registering plugins" local-registry text, add an "Installing plugins from GitHub" subsection: one-line example with `registry.NewGitHub()` and `RegisterPlugin("owner/repo", "v1.2.0")`; exact pinning (panics otherwise); trusted keys with `GitHubTrustedKeys`; private repositories via `GITHUB_TOKEN`/`GH_TOKEN`/`GitHubToken`; cache default `~/.xcl/cache/plugins`, `GitHubCacheDir`, offline reuse, re-verification on every plugin start, no automatic pruning; the errors (`ErrPluginNotFound`, `ErrPluginVerification`, `PluginInstallError`) inside `PluginLoadError`. Also update the "With plugins" quick start near `README.md:95-138` if it lists registries.
- `docs/plugins.md:306` — in "Registries and the catalog", add the GitHub registry beside the local one: a download-and-cache wrapper on the local registry, install and verification on every plugin start, then the ordinary external-plugin host, cache layout, and a pointer to the asset contract (archive/checksums/signature names).
- `CHANGELOG.md:1-3` — new top entry `## 20261009102148-7d0b205b-github-releases-registry`: prose describing `registry.NewGitHub`, options, caching, verification and errors; the new dependency `github.com/ProtonMail/go-crypto` and why (maintained OpenPGP, `x/crypto/openpgp` deprecated). No **Breaking:** list items are expected; state "None." if the section is kept.

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Add the GitHub registry guide to the documentation site

**Requirement → repo:** "Documentation shows how to install from GitHub" (documentation site) → `xcl-website`.

**File changes:**
- `xcl-website:src/pages/github-registry.mdx` (new) — frontmatter `layout: ../layouts/Shell.astro`, `title: "Installing plugins from GitHub - xcl"`, `description`; `<Hero>`, `<Prose>` with sections: declaring a plugin (one line), pinning a version, what a release must contain (archive/checksums/signature names from the asset contract), trusted keys, private repositories and tokens, the cache (default location, choosing one, offline use, re-verification, pruning by hand), errors; closing `<CtaBanner>` linking to `/registries/`. Follow `xcl-website:src/pages/registries.mdx` structure; fenced code blocks with `title="main.go"`.
- `xcl-website:src/components/Nav.astro:19-29` — add `{ label: "Installing from GitHub", href: "/github-registry/" }` to the Guides children.
- `xcl-website:src/pages/registries.mdx:142` — in "several registries", add a short paragraph and link to `/github-registry/`.

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution; verify with `npm run build` and `make check` in the website repo.

## Testing Strategy

Per task:

- **Add plugin install errors** — unit tests in `errors/` for message, `errors.Is`, `errors.As`; root re-export identity tests.
- **Encode the release asset contract** — pure unit tests: one per platform for names, checksums parsing success and failure, accepted and rejected tags in separate functions.
- **Add a fake GitHub release test server** — exercised by the registry tests that use it; no tests of its own beyond what callers prove.
- **Add the GitHub registry type and registration** — unit tests for name, types (none by default, pass-through `RegisterType`), cache defaults and each panic case separately.
- **Add the GitHub release client** — integration tests against the fake server: success, missing tag, draft, server error.
- **Install, cache and verify plugins at start** — integration tests against the fake server for fresh install, cache hit with server closed (no requests), six forced platforms, each refusal with an empty cache, cached-file tampering refused on the first and on a later start, plugins listed in order through the wrapped local registry.
- **Apply configurations with plugins from GitHub** — root-package Config tests: apply then no-change plan, offline apply from a chosen cache, missing release fails before apply with `*PluginLoadError`, GitHub plus local registries together.
- **Check release signatures against trusted keys** — trusted key installs, other key and unsigned refused, no keys installs unsigned, key change refuses cached plugin, malformed key fails the load, altered cached signature refused; one Config test for unsigned without keys.
- **Install from private repositories with a token** — token by option and each variable, precedence, and the no-token message.
- **Add an opt-in test against real GitHub** — skipped by default; installs and starts a named real release when enabled.
- **Documentation tasks** — no automated tests (project rule: no tests read repository files); checked by manual review and the website build.

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

## Project References

- Spec: `20261009102148-7d0b205b-github-releases-registry`.
- Designs: `plugin-registries.md` and `plugin-release-assets.md` from the `design` design source.
- Epic: `20261009092551-82db0140-plugin-template` (sibling specs: docker example standard layout; plugin template, which depends on this spec).
- Knowledge (xcl): conventions `testing-and-mocking.md`, `shared-test-helpers.md`, `shared-errors-package.md`, `dependencies.md`, `project-structure.md`, `code-style.md`, `patterns-and-architecture.md`, `development-standards.md`, `never-modify-dependencies.md`, `test-state-from-real-apply.md`; gotcha `example-modules-cannot-import-internal.md` (examples are not touched).
- Knowledge (xcl-website): gotcha `no-redirects-on-page-removal.md` (no page is removed).
- Repo roots: `xcl` → `/home/nicj/code/github.com/jumppad-labs/xcl`; `xcl-website` → `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Total estimate across tasks ≈ 310k tokens. The install task is the only High task; split it across agents as its notes describe. Milestone 1 tasks without dependencies (errors, asset contract, fake server, registry type) can run in parallel.

## Migration Notes

None. The feature is additive: no existing API, state format or plugin protocol changes. Applications opt in by adding a GitHub registry. The new `github.com/ProtonMail/go-crypto` module becomes a dependency of the main module.

## Performance Considerations

- A cache hit makes no network requests. Each plugin start (each `Plugin.Start` the catalog makes, once per `Config` when plugins load — not each operation's host restart) hashes the cached archive and decompresses it to hash the binary; for a typical 20-40 MB plugin this is tens to low hundreds of milliseconds per plugin start, the price of re-verifying offline from the signed checksums. Catalog restarts before later operations reuse the `Executable` host and add no verification cost. The spec's "fast after first use" metric is checked manually against a local binary.
- Downloads stream to disk; nothing holds an archive in memory.
