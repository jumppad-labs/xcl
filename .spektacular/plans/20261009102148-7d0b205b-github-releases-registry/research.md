---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Research: 20261009102148-7d0b205b-github-releases-registry

## Alternatives considered and rejected

- **Fetch and verify inside `Plugins()`, returning plain `registry.Executable`s.** Rejected: the catalog wraps a `Plugins()` error as `*PluginLoadError{Registry: ...}` with an empty `Plugin` (`internal/catalog/catalog.go:660-671`), so every install failure would be reported as "registry github.com failed to load its plugins" rather than naming the plugin the spec's acceptance criteria require. (The chosen design does return a plain `Executable` host, but only after the plugin's own `Start` has installed and verified the cache entry.)
- **Re-verify on every catalog `Restart` through a wrapping host (`verifyingHost`, `registry/github_host.go`).** Dropped at review: the user wants the registry to be a thin download-and-cache wrapper on the local registry. It would have added a host decorator relying on the catalog's structural `restartable` interface (`internal/catalog/catalog.go:81-86, 588-605`). Verification now runs on every `Plugin.Start` instead, and the host is exactly what `Executable(...).Start` returns.
- **A standalone GitHub registry with its own plugin list and `Plugins` implementation.** Rejected: duplicates the local registry's bookkeeping, ordering and starting; wrapping `*registry.Local` keeps the new code to the release client, cache and verification.
- **Use `go-github` (google/go-github) as the API client.** Rejected: the registry needs two calls (release by tag, asset download); `net/http` covers both and the `dependencies` convention prefers the standard library. go-github would add a large, frequently-versioned dependency for that.
- **Shell out to `gpg` for signature checks.** Rejected by the spec's Technical Approach; it also depends on a binary being installed on every user's machine.
- **Re-verify a cached plugin from a stored binary hash manifest.** Rejected: the manifest would not be covered by the release signature, so an attacker able to change the binary could change the manifest too. The cached archive is re-checked against the (signed) checksums file instead and the binary is compared with the archive's copy.
- **Re-extract the binary from the cached archive on every start.** Rejected: rewrites the cache on every operation and fails on Windows while the executable is running elsewhere; comparing hashes reads only.
- **Implement it as the design's `registry.NewRemote(url)`.** Rejected: the design (`plugin-registries.md`, "Remote registry") defines `NewRemote` for a future xcl-hosted registry protocol, explicitly unbuilt; GitHub releases is a different protocol. The GitHub registry is a second remote registry with its own constructor, following the same verb and rules.

## Chosen approach — evidence

- `registry/registry.go:22-63` — the `Registry` and `Plugin` interfaces the GitHub registry implements; `Plugin.Start(emit)` returns a `plugins.PluginHost`.
- `registry/registry.go:89-115` — `Executable(path)` starts a binary with `plugins.NewGRPCPluginHost` and returns the host; the installed binary reuses it unchanged ("reuse the existing external-plugin starter").
- `internal/catalog/catalog.go:652-695` — `load` calls `r.Plugins` once, then `plugin.Start` per plugin, wrapping a Start error as `*PluginLoadError{Plugin: name, Registry: registryName}` and emitting `load` start/success/error events naming the plugin. Installing inside `Start` gives every failure the plugin's name.
- `internal/catalog/catalog.go:81-86, 588-605` — the catalog restarts external hosts through the structural `restartable` interface (`Restart() error`, `Path() string`); the GitHub registry returns the plain `Executable` host, so these restarts behave as for any external plugin and the interface is not relied on.
- `registry/local.go:33-36, 90-117, 144-178` — `localEntry{plugin, directory}`, `RegisterType`/`Types`, the unexported `add` and `Plugins` returning entries in registration order; the GitHub registry wraps a `*Local`, adds `localEntry{plugin: githubPlugin}` through `add` (same package) and delegates `Plugins`, `RegisterType` and `Types` to it.
- `registry/local.go:22-60` — option pattern (`LocalOption func(*Local)`), `sync.Mutex`-guarded registration, panic-on-programmer-error messages `xcl: registry <name>: ...`; the GitHub registry mirrors it.
- `registry/local.go:178-215`, `registry/discovery.go:20-35` — events emitted from a registry via `emitEvent` with `events.SourceCore` and a `logger.New(emit, ...)` logger; the GitHub registry reports downloads and cache hits the same way under `events.OperationLoad`.
- `registry/discovery.go:149-168` — `~/` expansion helper reused for a cache directory option.
- `errors/plugin_load_error.go:1-42` — sentinel-and-detail convention for shared errors (knowledge entry `conventions/shared-errors-package.md`).
- `internal/test_fixtures/plugins/subtypeless/main.go` — an external plugin with no outside dependencies (block type `widget`), ideal as the binary packed into fake releases for apply tests.
- `config_plugin_loading_test.go:78-90`, `internal/testutil/programs.go:20-32` — how tests build plugin binaries with `go build` into `t.TempDir()`.
- Design `plugin-release-assets.md` — names: tag `v<semver>[-pre]`, archive `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` on windows), binary `<name>`/`<name>.exe` at archive root, `<name>_<version>_checksums.txt` in `sha256sum` format, `<name>_<version>_checksums.txt.sig` ASCII-armoured detached OpenPGP signature over the checksums file, plugin name = repository name.
- Design `plugin-registries.md` — `Register*` never returns an error; an empty remote name or version panics at the call; environment errors are returned when plugins load as `*PluginLoadError`; plugins load once per `Config` on the first operation.

## Files examined

- xcl:registry/registry.go:22-115 — Registry/Type/Plugin interfaces, InProcess, Executable.
- xcl:registry/local.go:1-223 — local registry (wrapped by the GitHub registry: `add`, `localEntry`, `Plugins`, `RegisterType`/`Types`), option and panic conventions, discover events.
- xcl:registry/discovery.go:149-168 — env and `~/` expansion for directories.
- xcl:registry/local_test.go — test style for the registry package (require, one behaviour per test).
- xcl:internal/catalog/catalog.go:81-86, 540-720 — Use/Load/load, restartable, PluginLoadError wrapping, load events.
- xcl:errors/plugin_load_error.go:1-42 — PluginLoadError and ErrPluginLoad.
- xcl:config.go:73,128 — re-export of PluginLoadError from the root package.
- xcl:options.go:22 — `WithRegistry`.
- xcl:plugins/grpc_plugin_host.go:52-80 — Start/Restart/Path of the gRPC host.
- xcl:internal/test_fixtures/plugins/subtypeless/main.go — self-contained external fixture plugin.
- xcl:config_plugin_loading_test.go:78-90 — building a plugin binary in tests.
- xcl:config_registries_test.go — root-level registry tests (style for the new GitHub registry Config tests).
- xcl:internal/testutil/programs.go — shared helper style (`testing.TB`, `t.Helper`, `t.TempDir`).
- xcl:README.md:455-503 — "Registering plugins" section to extend.
- xcl:docs/plugins.md:306 — "Registries and the catalog" section to extend.
- xcl:docs/modules.md:100-115 — xcl home `$HOME/.xcl` is already the default home (`$HOME/.xcl/cache`).
- xcl:CHANGELOG.md — entries headed `## <spec name>`, prose then a **Breaking:** list.
- xcl:go.mod — no OpenPGP library yet; `golang.org/x/crypto` only indirect.
- xcl:.mockery.yml — mockery v3 config; no interface in this plan needs a mock (tests use a real local HTTP server).
- xcl-website:src/pages/registries.mdx — registries guide (local registry, writing your own); link the new page from it.
- xcl-website:src/components/Nav.astro:9-31 — hardcoded nav; new guide added to the "Guides" children.
- xcl-website:Makefile, package.json — `npm run build` and `make check` (`astro check`); no tests.
- xcl-website knowledge gotchas/no-redirects-on-page-removal.md — only relevant if a page is removed (none is).

## External references

- GitHub REST API, "Get a release by tag name" (`GET /repos/{owner}/{repo}/releases/tags/{tag}`) and "Get a release asset" (`GET /repos/{owner}/{repo}/releases/assets/{id}` with `Accept: application/octet-stream`, redirects to storage) — the two calls the registry makes; the asset API endpoint works for private repositories with a token, unlike `browser_download_url`.
- GitHub returns 404 (not 403) for a private repository without a token — why the error says "not found, or it is private and needs a GitHub token".
- github.com/ProtonMail/go-crypto/openpgp — maintained fork of the deprecated `golang.org/x/crypto/openpgp`; `openpgp.ReadArmoredKeyRing` and `openpgp.CheckArmoredDetachedSignature` cover key parsing and detached-signature checks.
- The plugin template's `make dist` and release workflow — the producer side: contract archive and checksums names, and the signature the template's release workflow produces (gpg, armoured detached over the checksums).
- `sha256sum` output format (`<hex>  <name>`).

## Prior plans / specs consulted

- Spec `20261009102148-7d0b205b-github-releases-registry` — the spec being planned.
- Epic `20261009092551-82db0140-plugin-template` — sibling specs: docker example standard layout (independent) and the plugin template, which depends on this spec and publishes the asset contract.
- Design `plugin-registries.md` (draft, source `design`) and `plugin-release-assets.md` (final, source `design`) — both binding.

## Open assumptions

- The catalog calls `Plugin.Start` once per `Config` when plugins load (`internal/catalog/catalog.go:652-695`), so verification runs per plugin start rather than per operation; a cache entry altered while a long-running `Config` is in use is caught at the next `Config`'s plugin start, not at the catalog's next restart. Accepted at review. (The catalog's structural `restartable` check is no longer relied on: the host is the plain `Executable` host.)
- `registry.Local`'s unexported `add`/`localEntry` stay usable from the same package; if they are renamed, the GitHub registry's `RegisterPlugin` changes with them.
- GitHub's release-asset download redirects to a storage host; Go's client drops the `Authorization` header on cross-host redirects, which is the desired behaviour (storage URLs are pre-signed).
- ProtonMail/go-crypto supports the signature the template's release workflow produces (gpg, armoured detached over the checksums), made with a default GPG key (RSA/EdDSA).

## Drafting assumptions

### Chosen direction: GitHub registry as a download-and-cache wrapper on the local registry, installing and re-verifying at plugin start (architecture)
- **Decision**: `registry.NewGitHub(options...)` holds an internal `*registry.Local`; `RegisterPlugin("owner/repo", "vX.Y.Z")` adds `localEntry{plugin: githubPlugin}` to it through the unexported `add`; `Plugins()`, `RegisterType` and `Types` delegate to it, so the GitHub registry is a superset of the local one while reporting its own `Name()` `github.com`. Each plugin's `Start` installs into the cache if needed, verifies the entry, then returns `Executable(cachedBinaryPath).Start(emit)` unchanged; cache entries are moved into place atomically; stdlib HTTP plus ProtonMail/go-crypto.
- **Rationale**: built on `plugin-registries.md` (one `WithRegistry`, panic for single-call mistakes, load-time errors as `*PluginLoadError`) and `plugin-release-assets.md` (exact names); reuses the local registry's starting, naming, ordering and events so the only new code is the release client, the cache and verification; gives every error the plugin's name and re-checks the cache on every plugin start without touching the core.
- **Rejected**: see research.md alternatives (fetch in Plugins, standalone registry, wrapping host, NewRemote, go-github, gpg, manifest).

### Non-exact versions panic at RegisterPlugin (architecture)
- **Decision**: an empty version, a range, `latest` or anything that is not a single `v<semver>` tag panics at `RegisterPlugin` with a message naming the plugin; the tag must be given exactly (leading `v` required).
- **Rationale**: `plugin-registries.md` says an empty remote version panics at the call and that single-call mistakes panic; the spec's "fails before anything is downloaded, with an error naming the plugin" is met by that panic. Requiring the exact tag matches "an application pins that tag exactly" in the asset contract.
- **Rejected**: returning a load-time error (would split one mistake class across two reporting places); accepting `1.2.0` without `v` (ambiguous with the design's "pins that tag exactly").

### Registry name and options (architecture)
- **Decision**: `Name()` is `github.com`; options `GitHubCacheDir`, `GitHubTrustedKeys`, `GitHubToken`, `GitHubAPIURL`; default cache `$HOME/.xcl/cache/plugins/github.com/<owner>/<repo>/<version>/<os>_<arch>/`.
- **Rationale**: the design names registries by host; `$HOME/.xcl/cache` is the existing xcl home cache default; per-platform subdirectory keeps a shared cache valid across machines.
- **Rejected**: `$HOME/.xcl/plugins` (the documented `RegisterPluginDirectory` example directory, would mix discovered and installed binaries).

### Malformed trusted keys are a load-time registry error (architecture)
- **Decision**: armoured keys are parsed when plugins load; a key that does not parse fails the load as `*PluginLoadError` naming the registry.
- **Rationale**: keys are typically read from files or secrets at run time, an environment input rather than a code mistake.
- **Rejected**: panicking in `NewGitHub`.

### Install inside Plugin.Start, not Registry.Plugins (discovery)
- **Decision**: `Plugins()` returns one lazy plugin per registration; each plugin downloads/verifies (or re-verifies its cache) in `Start`.
- **Rationale**: the catalog names the plugin in `*PluginLoadError` and load events only for `Start` failures; the spec's errors must name the plugin.
- **Rejected**: fetching in `Plugins()` (errors would name only the registry).

### Re-verify on every plugin start, with no host wrapper (discovery)
- **Decision**: the spec's "cached plugins are re-verified before each start" is read as each start of the plugin — each `Plugin.Start` the catalog makes — not each catalog `Restart` before an operation. `Start` verifies the cache entry (checksums, plus signature when trusted keys are set) on every call, cached or fresh, and returns exactly the host `Executable(cachedBinaryPath).Start(emit)` returns.
- **Rationale**: keeps the registry a thin download-and-cache wrapper on the local registry, as the user asked at review; no host decorator and no reliance on the catalog's structural `restartable` interface.
- **Rejected**: re-verify on every `Restart` through a wrapping host (`verifyingHost`, `registry/github_host.go`) — dropped at review: user wants the registry to be a thin download-and-cache wrapper on the local registry.

### Keep the archive in the cache (discovery)
- **Decision**: the cache entry holds the archive, the checksums file, the signature (when present) and the extracted binary; re-verification hashes the archive against the checksums, checks the signature when keys are trusted, and compares the binary with the archive's copy.
- **Rationale**: the checksums cover archives, not binaries; this is the only offline check rooted in the signed file.
- **Rejected**: a hash manifest (unsigned) and re-extracting on each start (writes, Windows locking).

### Stdlib HTTP client, ProtonMail/go-crypto for OpenPGP (discovery)
- **Decision**: `net/http` for the two GitHub API calls; `github.com/ProtonMail/go-crypto` pinned in go.mod for signatures.
- **Rationale**: dependencies convention prefers stdlib; spec asks for a maintained Go OpenPGP library, and `x/crypto/openpgp` is deprecated.
- **Rejected**: go-github, `gpg` subprocess.

### Split signatures and tokens into their own milestone (milestones)
- **Decision**: Milestone 1 delivers public, checksum-verified installs with caching; Milestone 2 adds trusted keys and tokens; Milestone 3 documentation.
- **Rationale**: Milestone 1 is independently useful and exercises the whole install/verify/cache/start path; keys and tokens layer onto the verifier and client without reshaping it.
- **Rejected**: one large milestone (no intermediate validation point); docs inside each milestone (the site page covers all features at once).

## Rehydration cues

- `spektacular spec file read 20261009102148-7d0b205b-github-releases-registry`
- `spektacular design read --data '{"source":"design","path":"plugin-release-assets.md"}'` and `... "path":"plugin-registries.md"`
- `spektacular knowledge always-applied --tier repo --filter xcl`
- Re-read `registry/registry.go`, `registry/local.go`, `internal/catalog/catalog.go:540-720`, `errors/plugin_load_error.go`.
