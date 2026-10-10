---
created_date: "2026-10-09"
---

# Planning summary: 20261009092551-82db0140-plugin-template

## Decisions
- **Changelog entry per spec.** Plans involved: github-releases-registry, plugin-template, docker-example-standard-layout. The registry and template plans each wrote a top `CHANGELOG.md` entry headed with the spec name (prose, then a **Breaking:** list); the docker-example plan left the changelog to the implement workflow. Outcome: every plan writes its own entry in that format; the docker-example plan's "Update the README and guides to the new layout" task now adds it, with a **Breaking:** list naming the plugin's move to its own module and the removed `resources/`, `client/` and `main.go`.
- **GitHub registry wraps the local registry (review).** Plan involved: github-releases-registry. Outcome: the registry is a download-and-cache wrapper on `registry.Local`; each plugin start downloads if needed, verifies, then starts through `registry.Executable`. The verifying host that re-checked on every catalog `Restart` was dropped.
- **No GoReleaser (review).** Plans involved: plugin-template, github-releases-registry; design `plugin-release-assets.md`. Outcome: the template packages releases with `make dist` (go build per platform, tar/zip, `sha256sum`), signs the checksums with plain `gpg` (no `crazy-max/ghaction-import-gpg`) and publishes with `gh release create`; the design's "Producing it" section and the registry plan's signature question were reworded to match.
- **Release signing key published on xcl.dev (review).** Plan involved: plugin-template. Outcome: the shared jumppad-labs Ed25519 key (fingerprint `15BD A684 3A1F A0EA D1AA  5190 F775 DA00 AFD4 B502`) is served at `https://xcl.dev/keys/jumppad-labs-releases.asc` by an xcl-website agent task, with its fingerprint on the plugin template page.

## Order added for shared files
- Added: `20261009102148-7d0b205b-github-releases-registry` now depends on `20261009102138-48e95432-docker-example-standard-layout`. Both change `xclconfig:go.sum`, `xclconfig:README.md`, `xclconfig:docs/plugins.md`, `xclconfig:CHANGELOG.md`.

## 20261009102138-48e95432-docker-example-standard-layout
### Approach
The plugin example's Docker plugin (`example/plugin/plugins/docker`) is rebuilt in place as its own Go module, laid out to `plugin-layout.md`: `plugin.go` in the root package, `cmd/docker/main.go`, `entities/`, `providers/`, `client/docker` (the narrow Docker SDK interface) and a new `client/containers` task layer with an `ErrNotFound` sentinel, Mockery `mocks/` beside each client, `examples/basic`, `e2e/`, `Makefile` and `README.md`. Providers depend only on `containers.Tasks`, so `client/` is the only importer of Docker libraries; Docker calls, their order, error messages and log lines are unchanged. The example app imports only `entities`, and its engine check is reimplemented with the standard library (`GET /_ping` on `DOCKER_HOST`).

### Milestones and tasks
- M1, the app reads state with only the entity types: create the plugin module with entities and SDK client; add the container task layer; move providers onto it; make the plugin type importable and serve it from `cmd/docker`; point the app at entities only; build and test the module in CI and the e2e runner.
- M2, the plugin reads, decides and is tested as the standard describes: put every example file in the standard order; add a sample configuration and e2e test; write the plugin README.
- M3, docs: update the README, guides and changelog entry (xcl); update the documentation site pages (xcl-website).

### Tasks for a person
None. All 11 tasks are agent tasks.

### Out of scope
Other examples and fixtures; restructuring the in-process template plugin (reordered only); porting jumppad's resources; writing the standard layout into the guides and the template itself (plugin-template spec); installing or releasing via a remote registry (github-releases-registry spec); making `Read` query Docker; Podman e2e.

### Drafting assumptions
- "Changes only layout and ordering" means no observable behaviour change; if a call or message can't be kept identical, the implementer stops and asks.
- The Docker plugin is its own module, reached by the app through `replace => ./plugins/docker`.
- Two client packages: `client/docker` (SDK interface) and `client/containers` (task layer); `containers.ErrNotFound` stays plugin-private, not in `xcl/errors`.
- The app's engine check handles unix and tcp `DOCKER_HOST` and skips schemes it can't dial.
- App tests may still import the Docker SDK; the success metric concerns the app build.
- Moved unit tests keep names and outcomes, not exact inputs; the network subnet-replace test now passes the `subnet` change because `Changed` uses `change.Within`.
- One sample, `examples/basic`, with block names unique to it (`xcl_plugin_basic`).
- Old `resources/`, `client/` and `main.go` are deleted in the plugin-type task.

### Project-wide rules
- Standard plugin layout from `plugin-layout.md`, including a nested module per plugin, `client/<backend>` packages with `mocks/` beside them, and a task layer as the only importer of backend libraries.
- File order: exported first; provider methods `Init, Create, Read, Changed, Update, Destroy, Functions`; then unexported helpers, vars and consts.
- Explicit `Changed`: replace settings listed once and matched with `change.Within`; other changes update; otherwise `DefaultChanged`.
- No repo-inspecting tests; import, `go.mod`, doc and file-order checks are review only.
- Every example module is listed in CI's minimum-Go loop and `e2e/examples_test.go`.
- `docs/plugin-developer-guide.md` is shared with the plugin-template plan; whichever lands second rebases.
- Changelog: one top `CHANGELOG.md` entry headed with the spec name, prose then a **Breaking:** list (settled in this run).

### Manual checks
- One shape everywhere: the rebuilt plugin and every doc path match the design's tree and part descriptions.
- App stays light: the app's package dependency list contains no Docker SDK package.
- File order reviewed in every plugin example source file.
- The xcl-website site builds and shows only paths that exist.
- With Docker, the example's run, swap, replace, rebuild-init and remove-network targets behave as before, and the plugin module's build, test and generate targets work.

## 20261009102148-7d0b205b-github-releases-registry
### Approach
A thin download-and-cache wrapper on the existing local registry. `registry.NewGitHub(options...)` holds an internal `*registry.Local`, keeps its own `Name()` (`github.com`), delegates `Plugins()` and passes `RegisterType`/`Types` through, so it is a superset of the local registry. `RegisterPlugin("owner/repo", "v1.2.0")` adds an entry to the wrapped registry whose `Start` downloads the platform archive, checksums and (with trusted keys) signature into an atomically renamed cache entry under `$HOME/.xcl/cache/plugins` if not cached; verifies the entry on every start (archive SHA-256 against the checksums, the OpenPGP signature when keys are set, the binary against the archive's copy); then returns exactly what `registry.Executable(cachedBinaryPath).Start(emit)` returns. There is no host wrapper and no reliance on the catalog's `restartable` interface; starting, naming, ordering and events come from the local registry. Options: `GitHubCacheDir`, `GitHubTrustedKeys`, `GitHubToken` (else `GITHUB_TOKEN`, then `GH_TOKEN`), `GitHubAPIURL`. HTTP uses the standard library; signatures use `github.com/ProtonMail/go-crypto`. Failures reach the application as `*PluginLoadError` naming the plugin.

### Milestones and tasks
- M1, install a public plugin from a GitHub release: plugin install errors; encode the release asset contract; fake GitHub release test server; GitHub registry type and registration; GitHub release client; install, cache and verify at start; apply configurations with plugins from GitHub.
- M2, trusted keys and private repositories: check release signatures; install from private repositories with a token; opt-in test against real GitHub.
- M3, docs: README, `docs/plugins.md` and changelog (xcl); new `src/pages/github-registry.mdx` guide with nav entry and link from `registries.mdx` (xcl-website).

### Tasks for a person
None. All 12 tasks are agent tasks.

### Out of scope
A hosted registry or marketplace (`NewRemote` stays unbuilt); discovery or search; other release hosts; cache pruning; version ranges, `latest` or update checks; fetching only the plugins a configuration uses and `registry.Connect`; publishing releases (plugin-template spec); rate-limit handling and retries.

### Drafting assumptions
- Non-exact versions (empty, range, `latest`, missing `v`) panic at `RegisterPlugin`, naming the plugin. The spec says "an error naming the plugin"; the registries design says single-call mistakes panic. To confirm at review.
- The exact tag with its leading `v` is required.
- `Name()` is `github.com`; default cache `$HOME/.xcl/cache/plugins/github.com/<owner>/<repo>/<tag>/<os>_<arch>/`.
- A trusted key that won't parse fails the load as a `*PluginLoadError` naming the registry, not a panic at construction.
- "Cached plugins are re-verified before each start" is read as each `Plugin.Start` (once per `Config` load), not each catalog `Restart` before an operation; tampering during a long-running `Config` is caught at the next load. Re-verifying on every restart through a wrapping host was dropped at review.
- The GitHub registry depends on the local registry's unexported `add` and `localEntry`.
- The archive stays in the cache so a cached plugin can be re-checked offline.
- Open for implementation: if go-crypto can't verify the gpg signature a real template release produces, the implementer stops and asks rather than loosening verification.

### Project-wide rules
- Changelog: one top `CHANGELOG.md` entry headed with the spec name, prose then a **Breaking:** list; the new dependency and its reason are recorded there.
- Shared errors in the `errors` package, re-exported from the root: `ErrPluginNotFound`, `ErrPluginVerification`, `*PluginInstallError` (repository, version, platform).
- Asset contract as written in `plugin-release-assets.md`: `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows), `<name>_<version>_checksums.txt` in sha256sum format, `<checksums>.sig` ASCII-armoured detached, binary at archive root, plugin name = repo name, tag `v<semver>[-pre]`.
- Platforms: linux, darwin, windows on amd64 and arm64.
- xcl home `$HOME/.xcl`, cached plugins under `$HOME/.xcl/cache/plugins`.
- Fake GitHub release server in `internal/testutil`, importing neither the root package nor `registry`.
- New dependency `github.com/ProtonMail/go-crypto`, pinned exactly.
- No tests read README, docs or website files.

### Manual checks
- A reviewer confirms README and site examples need one registration line and no build step.
- Apply time with a cached GitHub plugin compared against the same binary registered as a local external plugin.
- On real darwin and windows machines, the right build is cached and starts (CI runs linux only).
- The opt-in real-GitHub test passes against a published, signed plugin release from a public and a private repository, once the template has produced one.
- README, plugin architecture guide and changelog entry reviewed against the shipped API.
- The documentation site builds; the new guide page and nav entry are reviewed in a browser.

## 20261009092551-82db0140-plugin-template
### Approach
A new GitHub template repository, `jumppad-labs/xcl-plugin-template` (created, pushed and marked as a template during the plan review, registered as `xcl-plugin-template` with its root at the local checkout), built exactly to `plugin-layout.md`, releasing per `plugin-release-assets.md`. The sample is one domain, `notes`, with resource `notes "note"` writing a note to a file through a narrow `client/files` interface with a strict Mockery double. An exported `ReplaceSettings` list (`directory`, `name`) drives an explicit `Changed`; `Update` acts only on the changes it is told. No host app: `e2e/` holds in-process and external lifecycle tests, and `e2e/stateonly` reads applied notes using only `entities` as the behavioural proof that entity types stand alone. GitHub Actions run CI on every change, a snapshot `make dist` of all six platforms and a weekly run against the latest xcl; a pushed version tag runs `make dist`, signs the checksums with plain gpg from repository secrets and publishes with `gh release create`. Docs: new `docs/plugin-layout.md` in xcl plus links and changelog entry, and a new xcl-website page with nav entry.

### Milestones and tasks
- M1, a new plugin builds, tests and applies from the template: scaffold the module and files client; note entity and provider; plugin type, program and sample configuration; e2e and state-reader tests; template README.
- M2, CI and signed releases: continuous integration; signed release automation; publish the template's first signed release (person).
- M3, docs: document the standard plugin layout (xcl); publish the jumppad-labs release signing public key at `https://xcl.dev/keys/jumppad-labs-releases.asc` (xcl-website); add the plugin template page to the site, with the key's fingerprint (xcl-website).

### Tasks for a person
- Push the template's first release tag.

### Out of scope
An in-process vs external parity test; a hosted registry or marketplace; porting jumppad's resources; rebuilding the docker example (sibling plan); any change to the plugin contract or xcl's Go code; a generator or rename tool; automated layout or file-order checks; running darwin and windows binaries in CI.

### Drafting assumptions
- Module `github.com/jumppad-labs/xcl-plugin-template`.
- Sample `notes "note"` on `client/files`: `directory` and `name` replace; `content` and `mode` update in place; a mode-only update only changes the mode.
- No host program; registrations live in the e2e tests and the README quotes them.
- Any replaced dependency means replace, with a comment on where to narrow it.
- Release, CI, `.gitignore` and `LICENSE` are repository tooling, not layout parts.
- Every `Files` method takes `ctx` and returns the client's own `Info` type.
- The template requires xcl `v0.1.1`, tagged during the plan review, which carries the current plugin contract.
- Signing key RSA 4096 or Ed25519, both verifiable by the registry's OpenPGP library.
- Open for implementation: whether `xcl.FindByType[entities.Note]` reads state when the plugin runs from its binary, and whether the gpg signature the release workflow produces verifies with the registry's library; if not, stop and ask.

### Project-wide rules
- Changelog: one top `CHANGELOG.md` entry in xcl headed with the spec name, prose then a **Breaking:** list ("- None.").
- Release asset contract implemented by `make dist` (go build per platform, tar/zip with the binary at the archive root, `sha256sum`) matching `plugin-release-assets.md`; Windows `.zip`; armoured detached `.sig` over the checksums; plugin name = repo name. No GoReleaser (user decision at review).
- Signing: the shared jumppad-labs Ed25519 key (fingerprint `15BD A684 3A1F A0EA D1AA  5190 F775 DA00 AFD4 B502`) in repo secrets, imported with plain `gpg --batch --import` and used for `gpg --armor --detach-sign`; the workflow fails before publishing when the key secret is empty. Its public key is served from the documentation site at `https://xcl.dev/keys/jumppad-labs-releases.asc`, with the fingerprint printed on the plugin template page.
- The written layout guide is `docs/plugin-layout.md` in xcl, owned by this plan.
- `README.md`, `docs/README.md` and `docs/plugin-developer-guide.md` are shared with the docker plan; whichever lands second rebases.
- Site nav: Guides entry "Plugin template" at `/plugin-template/`, linking to `/github-registry/`.
- Plugin contract used unchanged; the template requires xcl `v0.1.1`.
- No tests read README, docs, changelog, workflow or Makefile release targets.
- Third-party tools pinned and run without installing (Mockery v3.8.0, staticcheck); release packaging uses only go, tar/zip, sha256sum, gpg and gh.

### Manual checks
- A new author goes from "Use this template" to a working plugin in under 15 minutes using only the README.
- An author adds a second resource using only the README and the layout guide.
- First green weekly run against a real xcl release.
- Template tree, layout guide and site page all match `plugin-layout.md`.
- A tagged release has six archives, a checksums file and a `.sig` named per the contract, published with no manual steps, and the signature verifies against the published public key.
- Installing that release through the GitHub registry with a trusted key and applying the sample (also closes the registry plan's real-GitHub check).
- Every README command succeeds unchanged on a repo created from the template.
- A deliberate vet, format or test failure is reported by CI on the pull request.
- Exported members first and provider methods in lifecycle order in every source file.
- The site builds; new page, nav entry, layout guide, README links and changelog entry reviewed for accuracy.
