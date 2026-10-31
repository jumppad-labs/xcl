---
created_date: "2026-10-31"
document_status: draft
---

# Plan: 20261031180836-7529c085-plugin-release-contract-v2

<!-- Metadata -->
<!-- Created: 2026-10-31T18:27:31Z -->
<!-- Commit: 80cb0bf -->
<!-- Branch: f-python-plugin -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Move xcl's GitHub plugin registry, the plugin template's release tooling, and the Docker and test plugins to the language-neutral release layout in `plugin-release-assets-v2.md`: a signed xcl index with checksums, per-platform zips with a manifest, and digest-pinned container entries that are validated but not yet run. It is the first step towards plugins in Python, TypeScript and other languages. Plugin authors and the applications that install their plugins get one verified way to publish and install. Old-layout releases stop installing, and the two existing plugins are re-released, with every pin, test and doc across xcl, xcl-python and the website moved to them.

## Conventions

- **Configuration files are xcl, static files are literals only** — the index and the manifest are xcl, decoded with no evaluation context, so expressions are refused.
- **Testing & Mocking (testify `require`, no table-driven tests, positive and negative cases in separate functions, tests beside their source, no tests that read repository files)** — every registry behaviour and failure gets its own test function. No test asserts on docs, README, workflow or Makefile text; release tooling is checked by running it.
- **Shared test helpers live in `internal/testutil`** — the rewritten fake release server and its zip/index/signing helpers serve the registry, root-package, coreservice and example tests, so they stay in `internal/testutil`, taking `testing.TB` and never importing the root package.
- **Shared error types live in the `errors` package, re-exported from the root** — new failure modes wrap the existing `ErrPluginNotFound` and `ErrPluginVerification` inside `PluginInstallError`. Any new detail type follows the sentinel-and-detail convention with pointer receivers.
- **Testing provider Changed and Update** — the `ReplaceOn` moves in the Docker plugin and template are proved by tests that pass the real `PropertyChange` and `DependencyChange` lists and assert the replace or update answer.
- **Never modify dependency packages** — the OpenPGP library and xcl's internal HCL fork are used as they are. The decoder needs no change.
- **Write a release plan once released work is merged** — this spec releases xcl, both plugins, xcl-python and the website, so a `release-plan` document is written after implementation.

## Architecture & Design Decisions

The work implements the settled design `plugin-release-assets-v2.md` in five repos, in release order. The core change is in xcl's existing GitHub registry. It keeps its public API and its cache-and-verify transaction, and swaps what it reads:

- **What the registry reads:** the release index instead of a checksums file, and a per-platform zip with an `xcl-plugin.xcl` manifest instead of a tar.gz holding a bare binary.
- **A new internal release-file reader** in the `registry` package decodes the index and the manifest with xcl's own decoder (`hclsyntax.ParseConfig` and `gohcl.DecodeBody`) into fixed Go structs. It decodes with no evaluation context, so a variable or function call is refused with xcl's own diagnostic, which satisfies "release descriptions are static". A remain field collects top-level blocks the registry doesn't know, so new kinds are skipped. Platforms are nested two-label blocks, which decode because of the list-block identity fix (commit 9e9893d).
- **Validation and choice:** the reader validates the index (name and version match the request, every executable platform has a `sha256:` checksum, a container is digest-pinned with Linux platforms only, at least one known kind). A chooser picks the executable build for the host platform. It never picks a container yet; a container-only release fails with the same "no build for this platform" error, listing what the release offers.

Installing keeps the shape of the current transaction (`registry/github_install.go`):

1. Download the index, plus the `.sig` when keys are trusted, into a staging directory beside the cache entry.
2. Verify the signature over the index, reusing the existing OpenPGP check.
3. Validate the index and choose the build.
4. Download only that zip and check its SHA-256 against the index.
5. Read the zip's `xcl-plugin.xcl` and check it agrees with the index entry (name, version, protocol, platform).
6. Extract the zip and mark the `command` binary executable.
7. Rename the staging directory into place.

The cache entry keeps the index, signature and zip beside the extracted files. On every start, the registry re-verifies the signature (when keys are trusted), the zip against the index checksum, and the extracted binary against its copy in the zip, as `verifyBinary` does today. `Executable` gains optional arguments so a manifest `command` with arguments reaches the process. Plugins speak protocol 1 today, and the registry refuses a build whose `protocol` list doesn't include it. New failures are wrapped in the existing `ErrPluginNotFound` and `ErrPluginVerification` sentinels inside `PluginInstallError`, following the shared errors package convention, so applications match them exactly as today. The shared fake release server in `internal/testutil` is rewritten to serve the new layout, because the registry, root-package, coreservice and example tests all use it.

The release side lives in the plugin template:

- **The release build target** builds a zip per platform with the binary, README, LICENSE and a generated `xcl-plugin.xcl`, records each zip's SHA-256, and writes `<name>_<version>_index.xcl`. It uses only shell tools (go, zip, sha256sum), as today.
- **The release workflow** signs the index instead of the checksums file, then publishes with GitHub's release CLI.
- **Copying the changes over:** the Docker and test plugins take these Makefile and workflow changes verbatim, since their copies are byte-identical apart from names.
- **`ReplaceOn`:** each plugin bumps its xcl dependency to the release carrying `DefaultChanged.ReplaceOn` and the new registry, and moves only its replace-settings loop to `ReplaceOn`. Dependency rules, and the "any change → update" rule where present, stay in `Changed`, so behaviour is preserved.
- **New releases:** both plugins then publish new releases.
- **Moving everything else over:** after that, xcl's example and docs, the Python SDK's test pin and integration helpers, and the website move to the new releases and layout, in the order the release chain fixes (xcl, plugins, xcl-python, website).

This beats the alternatives because it changes the fewest moving parts. Neither the parser nor `Config` changes, the registry keeps its transaction and errors, and the plugins copy one template change. The alternatives were decoding through `Config` (an import cycle, plus reading of `.vars` files and the environment), new parser options for literals and unknown blocks, keeping a checksums file, or a backwards-compatible dual path. They are recorded in research.md#alternatives-considered-and-rejected.

## Component Breakdown

- **Release-file reader (new, internal to the `registry` package)** — decodes a release index and a zip's manifest from bytes with xcl's own decoder, refusing expressions and skipping unknown top-level kinds. Validates an index against the requested repository and tag: name, version, checksum present and well-formed on every executable platform, container digest-pinned with Linux platforms only, at least one known kind. Checks a manifest against its index entry. Used by the installer and the verifier, so release files are parsed in one place only.
- **Asset naming (changed, internal to the registry)** — the single place that knows the new contract's names: `<name>_<version>_index.xcl`, its `.sig`, and the tag-to-version rule. Zip names are taken from the index rather than computed. The old tar.gz, checksums and `.exe` naming is removed.
- **Build chooser (new, internal to the registry)** — given a validated index and the host platform, returns the executable build for that platform. Otherwise it returns ErrPluginNotFound, naming the platforms the release offers and the host's platform. It never chooses a container.
- **Installer (changed)** — the existing staging-then-rename transaction now fetches the index (and signature), verifies and validates it, chooses a build, downloads only that zip, checks its checksum, checks its manifest, extracts it and marks the command binary executable. The cache entry layout per repository, tag and platform is unchanged. Release-client calls (release by tag, asset download, token, drafts) are reused unchanged.
- **Verifier (changed)** — signature verification moves from the checksums file to the index (same OpenPGP check and key-naming error). Archive verification becomes zip-against-index-checksum, and the per-start binary check now compares the extracted command binary with its copy in the zip. Used after a fresh install and on every start.
- **Executable starter (changed, `registry` package)** — `Executable` accepts optional arguments after the path, passed to the started process. Existing callers are unchanged.
- **Install errors (existing, `errors` package)** — every new failure (no index, incomplete index, expression, checksum mismatch, manifest mismatch, unsupported protocol, no build for this platform) wraps `ErrPluginNotFound` or `ErrPluginVerification` inside `PluginInstallError`, reaching applications through the catalog's `PluginLoadError` as today.
- **Fake GitHub release server (changed, `internal/testutil`)** — builds and serves a release in the new layout from the real fixture plugin:
  - zips per platform, each with a manifest;
  - an index with checksums;
  - an optional index signature from a generated key.

  It has switches to drop a platform, drop the index or signature, omit a checksum, tamper with a zip, write a manifest that disagrees, add an unknown kind, add a container entry (digest-pinned, tag-only or non-Linux), put an expression in the index, sign with another key, require a token, or serve a draft. It is used by the registry, root-package, coreservice and example tests.
- **Plugin template release tooling (changed, `xcl-plugin-template`)** — the release build target builds a zip per platform with the binary, README, LICENSE and a generated manifest, and writes the index with each zip's checksum. The release workflow signs the index and publishes all assets. The README's release, install and verifying sections describe the new layout.
- **Template note provider (changed)** — declares its replace settings with `DefaultChanged.ReplaceOn`. Its dependency rule and "any change → update" rule stay in `Changed`.
- **Docker plugin (changed, `xcl-plugin-docker`)** — takes the template's tooling and bumps its xcl dependency. The network and container providers move their replace-settings loops to `ReplaceOn`, keeping their other rules. README pins and release section updated. A new release is published.
- **Test plugin (changed, `xcl-plugin-test`)** — takes the template's tooling and bumps its xcl dependency. README release section updated. A new release is published. Its provider is unchanged.
- **xcl example and docs (changed, `xcl`)** — the remote-plugin example pins the Docker plugin's new release. The plugin and registry documentation describes the new layout.
- **Python SDK tests and README (changed, `xcl-python`)** — the pinned test-plugin release, the integration helpers that describe a cache entry and its release files, the tamper tests (now tampering with the index and the zip), and the README's Docker pin move to the new layout and releases. The SDK's API is unchanged.
- **Documentation site (changed, `xcl-website`)** — the GitHub registry, plugin template, registries, home and replacement pages describe the new layout, pin the new releases, and show `ReplaceOn` where they showed hand-written replace loops.

## Data Structures & Interfaces

**Release index** (internal to `registry`): the decoded `<name>_<version>_index.xcl`. Fields are tagged for xcl's decoder. Unknown top-level blocks land in the remain body and are ignored.

```go
type releaseIndex struct {
    Plugin     indexPlugin      `xcl:"plugin,block"`
    Executable *indexExecutable `xcl:"executable,block"`
    Container  *indexContainer  `xcl:"container,block"`
    Rest       hcl.Body         `xcl:",remain"`        // unknown kinds, skipped
}

type indexPlugin     struct { Name, Version string }
type indexExecutable struct { Protocol []int; Platforms []indexPlatform `xcl:"platform,block"` }
type indexPlatform   struct { OS, Arch string /* labels */; Asset, Checksum string }
type indexContainer  struct { Protocol []int; Image string; Command []string; Platforms []imagePlatform }
```

**Build manifest** (internal): the decoded `xcl-plugin.xcl` at a zip's root. It has the same `plugin` block and an `executable` block carrying `protocol`, `command` and exactly one platform.

```go
type buildManifest struct {
    Plugin     indexPlugin
    Executable manifestExecutable // Protocol []int; Command []string; Platforms []imagePlatform (one)
    Rest       hcl.Body           // remain
}
```

**Chosen build** (internal): what the chooser hands the installer, the platform's asset name and its expected `sha256` digest.

```go
type chosenBuild struct { Platform platform; Asset string; SHA256 [32]byte }
```

**Reader functions** (internal):

```go
func readIndex(src []byte, filename string) (*releaseIndex, error)          // decode, no eval context
func (i *releaseIndex) validate(repository, tag string) error                // name/version, checksums, digests, linux containers, a known kind
func (i *releaseIndex) choose(host platform) (chosenBuild, error)            // executable only
func readManifest(src []byte) (*buildManifest, error)
func (m *buildManifest) agrees(i *releaseIndex, host platform) error         // name, version, protocol, platform
```

**Executable** (public, `registry` package): gains optional arguments. This is source-compatible.

```go
func Executable(path string, args ...string) Plugin
```

**Fake release server options** (`internal/testutil`): the old options that named the checksums file are replaced.

```go
WithoutPlatform(p string), WithoutIndex(), WithoutSignature(), WithoutChecksum(p string),
TamperZip(p string), ManifestDisagrees(p string, field string), WithUnknownKind(),
WithContainer(image string, platforms ...string), IndexExpression(), SignedByOtherKey(),
RequireToken(), Draft()
// accessors: URL, Close, Requests, LastToken, PublicKey, OtherPublicKey, OtherKeyID,
//            IndexName, ZipName(p), Zip(p)
```

**Wire format**: the index and manifest formats are fixed by `plugin-release-assets-v2.md`. Checksums are `sha256:<64 lowercase hex>`, and images are `<repository>@sha256:<64 hex>`. No public type exposes either file.

**Errors**: no new public types. Every failure wraps `ErrPluginNotFound` (no index, no build for this platform, only a container) or `ErrPluginVerification` (bad signature, incomplete index, expression, checksum mismatch, manifest mismatch, unsupported protocol) inside the existing `PluginInstallError`.

## Implementation Detail

- **One reader for release files.** All knowledge of the new layout sits in the registry's release-file reader and asset naming. The installer and verifier ask the reader and never parse or name files themselves. This replaces today's split between `namesFor` and `parseChecksums`. A developer reading the installer sees the design's install sequence as a straight run of calls: fetch, verify the signature, validate, choose, fetch the build, check the checksum, check the manifest, extract, commit.
- **Static decoding as a pattern.** Decoding with xcl's own decoder and no evaluation context, plus a remain field for unknown blocks, is a new pattern for xcl-owned metadata files. The reader's doc comment names it, so later metadata files (a future plugin manifest kind, a lock file) follow it rather than going through `Config`.
- **The existing transaction is followed.** Staging beside the cache entry, rename into place, a lost race treated as success, structured `load` log events for download, cache hit and verify, and verification on every start: all stay as they are, with only the files handled changing. Tests keep asserting that no staging directory is left behind after a failure.
- **Validation before download.** Everything the index alone can decide (name, version, checksums present, digests, container platforms, a known kind, a build for this platform) is refused before any build is requested. Tests prove this by counting the fake server's requests.
- **Tests follow today's shape.** Registry package unit tests cover the reader and the chooser with literal index text. Integration tests drive the installer against the fake server, one function per behaviour. Root-package Config tests install the fixture plugin through `xcl.WithRegistry` and apply with it. Old-layout tests (asset-name expectations, checksums parsing) are deleted with the code they tested.
- **Template tooling stays shell.** The release build target keeps using only go, zip and sha256sum, and generates the manifest and index with `printf` from Makefile variables. The plugins keep byte-identical copies of the template's Makefile release section and workflows, so future template changes copy over the same way.
- **Provider changes keep behaviour by construction.** Each provider keeps its `Changed` where it has rules beyond the replace settings, and drops only the loop that `ReplaceOn` now performs. Existing `Changed` tests stay unmodified and must pass, which proves behaviour is unchanged.

## Dependencies

- **Design `plugin-release-assets-v2.md` from the `design` design source** — the settled release layout this plan implements: asset names, index and manifest shape, checksum and digest forms, signature, install sequence, unknown-kind skipping, no backwards compatibility. No change needed.
- **Design `plugin-layout.md` from the `design` design source** — the `ReplaceOn` rule the plugin providers adopt ("replace settings are declared; `Changed` only for rules beyond them"). No change needed.
- **Commit 9e9893d (xcl, landed)** — nested list blocks are named by all their labels, so an index can list two platforms sharing an OS. Must be in the xcl release the plugins and Python SDK pick up.
- **Commit 348ec83 (xcl, landed)** — `DefaultChanged.ReplaceOn`, which the Docker plugin and template adopt. Must be in an xcl release before the plugins bump to it.
- **`registry` package (existing)** — the GitHub registry, release client, cache transaction, verifier and `Executable`. Changed as described. Its public API is unchanged apart from `Executable`'s optional arguments.
- **`internal/xcl` decoder (existing)** — `hclsyntax.ParseConfig` and `gohcl.DecodeBody` decode the release files. Used as they are.
- **`errors` package (existing)** — `ErrPluginNotFound`, `ErrPluginVerification`, `PluginInstallError`. Reused, no new types.
- **`internal/testutil` fake release server and `BuildFixturePlugin` (existing)** — the fake is rewritten to the new layout. Its users in coreservice and example/remoteplugin tests follow.
- **`github.com/ProtonMail/go-crypto` (existing third-party)** — OpenPGP signature verification, now over the index. No version change.
- **Go standard library** — `archive/zip`, `crypto/sha256`, `net/http`, `os`. The tar and gzip readers are no longer needed by the registry.
- **xcl release (planning dependency)** — the template, Docker and test plugins can only bump to `ReplaceOn` and be verified against the new registry once xcl is released with this change. xcl releases first.
- **xcl-plugin-template, xcl-plugin-docker, xcl-plugin-test (registered repos)** — release tooling and providers changed. Each plugin's new release must be published (a person pushes the tag) before the Python SDK pin, the xcl example pin and the docs move to it.
- **xcl-python (registered repo)** — integration tests and README follow the new releases. Its engine pin (xcl-ref.txt) must carry the xcl release with the new registry first. The bump is automated by xcl's release dispatch.
- **xcl-website (registered repo)** — pages describe the new layout. Deploys on every push to main, so it merges only after the releases it describes are published.
- **GitHub Releases and the jumppad-labs signing key (external)** — releases are published with GitHub's release CLI and signed with the existing `GPG_PRIVATE_KEY`/`GPG_PASSPHRASE` secrets. The public key at xcl.dev/keys is unchanged.
- **Prior plan `20261009102148-7d0b205b-github-releases-registry`** — built the registry and the fake server this plan rewrites. Archaeology only.

## Testing Approach

**Tests in xcl:**

- **Unit tests in the registry package for the release-file reader and chooser,** using literal index and manifest text.
  - A valid index decodes.
  - An index with two platforms sharing an OS decodes.
  - Each validation failure is its own test: wrong name, wrong version, missing checksum, malformed checksum, tag-only image, non-Linux container platform, no known kind.
  - A variable and a function call are each refused with xcl's diagnostic.
  - An unknown kind is skipped.
  - The chooser picks the host's build, refuses a platform the release doesn't offer (naming both sides), and refuses a container-only release.
  - A manifest that agrees passes, and each disagreement (name, version, protocol, platform) fails.
- **Integration tests in the registry package** drive the installer against the rewritten fake release server, one behaviour per function:
  - a fresh install;
  - a signed install with a trusted key;
  - signature failures (no signature, another key) with no build requested;
  - an unsigned install without keys;
  - a release with no index;
  - an incomplete index refused with zero build requests;
  - only the host's build requested out of six;
  - a tampered zip refused and not cached;
  - a manifest that disagrees refused;
  - an unknown kind ignored;
  - a container alongside builds installs the build, and a container-only release fails;
  - a cache hit with no requests;
  - a cached zip or binary altered on disk refused on the next start;
  - no staging directories left after any failure.

  These are the load-bearing tests: together they cover every acceptance criterion about installing.
- **Config-level integration tests in the root package** install the fixture plugin from the fake server through `xcl.WithRegistry` and run a real apply and destroy. Existing coreservice and example/remoteplugin tests keep passing on the rewritten fake.
- **A unit test for `Executable` arguments** shows that arguments reach the started process.
- **The fake release server's own tests** prove it serves the layout the design fixes: names, checksums matching the zips, a signature that verifies, and each switch's effect.
- **The opt-in live test** (`XCL_GITHUB_LIVE_PLUGIN`) runs against the new test-plugin release once it's published. It isn't part of the default suite.

**Tests in the other repos:**

- **Plugin repositories:** existing provider `Changed` tests stay untouched and must pass after moving to `ReplaceOn`, which proves the constraint "rebuilding must not change behaviour". The template, Docker plugin and test plugin keep their e2e suites. Each repo's CI `dist` job runs the release build target on every push, so a broken release build fails CI.
- **Python SDK:** the integration tests (pinned test-plugin install, tamper tests rewritten to tamper with the index and zip, unsigned install) run against the new release once it exists.

**Deliberate gaps:**

- No test reads docs, README, workflow or Makefile text (project convention). Docs and release tooling are checked by running them and by review.
- Darwin and Windows installs aren't exercised in CI, which runs Linux.

**Success metrics:**

- **"A new release is produced from the template with one release command and the workflow, no hand-written files"** — partly behavioural: the CI `dist` job builds a complete release, zips plus index, from the release build target alone. Publishing a real release is **Manual — captured in the implementation test plan**.
- **"Pushing a version tag produces a verifiable release with no manual step"** — **Manual — captured in the implementation test plan** (push a tag on each plugin and confirm the published release verifies).
- **"Both plugins install from their new releases, from Go and Python, verified against the published key, on every CI platform"** — behavioural through the Python SDK's integration tests on its CI matrix and the opt-in live test. Running the live test against the published Docker and test releases on darwin and windows is **Manual — captured in the implementation test plan**.

**Manual reviews:**

- **Manual — captured in the implementation test plan**: review the docs in xcl, the template, the Docker and test plugin READMEs, the Python SDK README and the website pages for accuracy against the new layout, with no old-layout wording left.
- **Manual — captured in the implementation test plan**: run the documented getting-started example against the Docker plugin's new release and confirm it creates and destroys its network and container.
- **Manual — captured in the implementation test plan**: build the website and check the changed pages in a browser.

## Milestones & Tasks

### Milestone 1: xcl installs plugins from releases in the new layout

**What changes**: Applications that install plugins from GitHub releases now read the release's index, refuse anything unverified before downloading, fetch only the build for their machine, and check that build against the index. Releases in the old layout are no longer installed. The registry's API is unchanged, so applications change nothing in their code. Container entries are understood and validated but not run. Until the plugins re-release, only the fake test releases use the new layout, so this milestone is proved entirely by xcl's own tests.

**Validation point**: the registry, root-package, coreservice and example test suites pass against the rewritten fake release server, including every install failure mode, and xcl's CI is green.

#### - [ ] Task: Serve the new release layout from the fake release server
**Id:** 7c1f4231-863f-4795-8602-84df2df92afc
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Rewrite the shared test helper that fakes a GitHub release so it builds and serves releases in the new layout:

- a zip per platform, each holding the plugin binary and its manifest;
- an index carrying every zip's checksum;
- an optional signature over the index.

It gains switches for every failure the registry must handle, and its own tests prove the layout it serves matches the design. The registry's tests in later tasks build on it.

*Technical detail:* [context.md#task-serve-the-new-release-layout-from-the-fake-release-server](./context.md#task-serve-the-new-release-layout-from-the-fake-release-server)

**Acceptance criteria**:
- [ ] A fake release serves an index, a zip per platform and, when asked, a signature, named as the design fixes them.
- [ ] The checksums in a fake index match the zips it serves, and its signature verifies against the fake's public key.
- [ ] Each switch (dropped platform, no index, no signature, missing checksum, tampered zip, disagreeing manifest, unknown kind, container entry, expression, other signing key, token, draft) changes the served release as named.

#### - [ ] Task: Read and validate release indexes and build manifests
**Id:** 75c63fab-8b06-4f97-a174-5ab17d41dfb5
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Add the registry's internal reader for the release index and a build's manifest, using xcl's own decoder with no evaluation context, so expressions are refused and unknown kinds are skipped. It validates an index against the requested plugin and version, chooses the native build for a platform, and checks that a manifest agrees with its index entry. Everything the registry knows about the new file formats lives here.

*Technical detail:* [context.md#task-read-and-validate-release-indexes-and-build-manifests](./context.md#task-read-and-validate-release-indexes-and-build-manifests)

**Acceptance criteria**:
- [ ] A valid index decodes, including two platforms that share an operating system.
- [ ] An index naming the wrong plugin or version, missing or malforming a checksum, naming a container by tag, listing a non-Linux container platform, or offering no known kind is refused with an error naming the problem.
- [ ] An index or manifest using a variable or a function call is refused.
- [ ] An index with an extra unknown kind decodes, and the unknown kind is ignored.
- [ ] The host's native build is chosen. A release with no build for the host, or with only a container, is refused with an error listing the platforms offered and the host's platform.
- [ ] A manifest that disagrees with its index on name, version, protocol or platform is refused.

#### - [ ] Task: Install plugins from releases in the new layout
**Id:** 1ee47308-1326-4819-b3be-ad6364436024
**Repo:** xcl
**Depends on:**
- 7c1f4231-863f-4795-8602-84df2df92afc — Serve the new release layout from the fake release server
- 75c63fab-8b06-4f97-a174-5ab17d41dfb5 — Read and validate release indexes and build manifests
**Execution:** agent

Rewrite the GitHub registry's install and verify steps around the new layout:

- fetch and verify the index;
- refuse anything incomplete before downloading;
- download only this machine's zip and check it against the index;
- check its manifest, extract it, and start the binary its manifest names.

A cached plugin is re-verified on every start. The old tar.gz and checksums-file handling is removed. The plugin-starting function also accepts arguments, so a manifest command with arguments works. Tests elsewhere in the repo that use the fake release server keep passing.

*Technical detail:* [context.md#task-install-plugins-from-releases-in-the-new-layout](./context.md#task-install-plugins-from-releases-in-the-new-layout)

**Acceptance criteria**:
- [ ] A plugin installs from a fake release in the new layout and is used by a real apply and destroy.
- [ ] A release with no index, or with an incomplete index, fails to load, and no build is downloaded.
- [ ] With trusted keys, a release whose index signature is missing or from another key fails to load and no build is downloaded. Without keys, an unsigned release installs.
- [ ] Installing a release offering six platforms downloads exactly one build.
- [ ] A tampered zip, or a manifest that disagrees with its index, fails to load and leaves nothing in the cache.
- [ ] A second load uses the cache with no requests, and a cached build altered on disk fails the next load.
- [ ] A release with a container alongside native builds installs the native build. A container-only release fails with the no-build error.
- [ ] Applications see the same error kinds as before: not found, or failed verification.

#### - [ ] Task: Describe the new release layout in xcl's docs
**Id:** 54638801-3f11-4d89-b644-fe9eb29e69e2
**Repo:** xcl
**Depends on:**
- 1ee47308-1326-4819-b3be-ad6364436024 — Install plugins from releases in the new layout
**Execution:** agent

Update xcl's plugin and registry documentation. It should describe what a release must contain (index, zips, checksums in the index, optional signature over the index), what the cache holds, and which failures produce which errors. Remove every description of the old layout.

*Technical detail:* [context.md#task-describe-the-new-release-layout-in-xcls-docs](./context.md#task-describe-the-new-release-layout-in-xcls-docs)

**Acceptance criteria**:
- [ ] xcl's docs describe the release index, per-platform zips, checksums in the index and the optional index signature.
- [ ] No xcl doc describes per-platform tar.gz archives, a checksums file or a signature over a checksums file.

#### - [ ] Task: Release xcl with the new registry
**Id:** 58dc2f75-6b38-4013-827e-d4b6af4e8c52
**Repo:** xcl
**Depends on:**
- 1ee47308-1326-4819-b3be-ad6364436024 — Install plugins from releases in the new layout
- 54638801-3f11-4d89-b644-fe9eb29e69e2 — Describe the new release layout in xcl's docs
**Execution:** human — releasing is an action outside the repo (merging the labelled PR that tags and publishes xcl)

Merge the branch carrying the new registry, `ReplaceOn` and the nested-block fix with the release label, so xcl's automation tags and publishes a release and dispatches the Python SDK's engine bump. The plugins need this release before they can build against `ReplaceOn` and be installed by the new registry.

*Technical detail:* [context.md#task-release-xcl-with-the-new-registry](./context.md#task-release-xcl-with-the-new-registry)

**Acceptance criteria**:
- [ ] An xcl release containing the new registry, `ReplaceOn` and the nested-block fix is published on GitHub.
- [ ] The Python SDK's engine bump to that release has been opened.

### Milestone 2: Plugin authors publish releases in the new layout

**What changes**: The plugin template's release command and workflow produce and publish a release in the new layout, with zips, an index with checksums and a signature over the index. Its provider declares its replace settings instead of hand-writing them. The Docker and test plugins take the same tooling and provider change, and each publishes a new release. Plugin behaviour is unchanged.

**Validation point**: each plugin repo's CI (tests, e2e and the `dist` build) is green on the xcl release from milestone 1. Each plugin's new release is published, and xcl's opt-in live test installs the test plugin's new release, verified against the jumppad-labs key.

#### - [ ] Task: Publish releases in the new layout from the plugin template
**Id:** bc7d2bfe-aec4-4314-a880-e1320ae7cd0a
**Repo:** xcl-plugin-template
**Depends on:**
- 58dc2f75-6b38-4013-827e-d4b6af4e8c52 — Release xcl with the new registry
**Execution:** agent

Change the template's release build to produce a zip per platform with a generated manifest and an index carrying each zip's checksum, and the release workflow to sign the index instead of a checksums file. Bump the template to the new xcl release, move its note provider's replace settings to `ReplaceOn` while keeping its other rules, and update the README's release, install and verifying sections.

*Technical detail:* [context.md#task-publish-releases-in-the-new-layout-from-the-plugin-template](./context.md#task-publish-releases-in-the-new-layout-from-the-plugin-template)

**Acceptance criteria**:
- [ ] The release build produces one zip per supported platform, each with its manifest and an executable binary, and an index whose checksums match the zips.
- [ ] The release workflow signs the index and publishes the index, its signature and the zips.
- [ ] The note provider answers replace and update for exactly the same changes as before.
- [ ] The README describes the new release layout and no longer describes the old one.

#### - [ ] Task: Publish a template release in the new layout
**Id:** 0f9236c4-3d3d-449e-9a01-d6cd28d77794
**Repo:** xcl-plugin-template
**Depends on:**
- bc7d2bfe-aec4-4314-a880-e1320ae7cd0a — Publish releases in the new layout from the plugin template
**Execution:** human — releasing needs pushing a version tag, which runs the workflow with the signing-key secrets

Push the template's next version tag so its release workflow publishes a signed release in the new layout, which the template's docs and the website pin.

*Technical detail:* [context.md#task-publish-a-template-release-in-the-new-layout](./context.md#task-publish-a-template-release-in-the-new-layout)

**Acceptance criteria**:
- [ ] The template's new release is published with an index, its signature and a zip per platform.

#### - [ ] Task: Move the Docker plugin to the new release layout
**Id:** 40cff90a-96e2-417f-bfab-8a31ff5ce7d5
**Repo:** xcl-plugin-docker
**Depends on:**
- bc7d2bfe-aec4-4314-a880-e1320ae7cd0a — Publish releases in the new layout from the plugin template
**Execution:** agent

Copy the template's release build and workflow changes, bump to the new xcl release, and move the network and container providers' replace-settings lists to `ReplaceOn`. Their dependency rules and update rules stay as they are. Update the README's pins and release section.

*Technical detail:* [context.md#task-move-the-docker-plugin-to-the-new-release-layout](./context.md#task-move-the-docker-plugin-to-the-new-release-layout)

**Acceptance criteria**:
- [ ] The release build produces zips and an index in the new layout.
- [ ] The network and container providers answer replace and update for exactly the same changes and dependencies as before.
- [ ] The README describes the new release layout.

#### - [ ] Task: Move the test plugin to the new release layout
**Id:** a6d3deb6-98ce-4830-ab07-b6d328ac2a1b
**Repo:** xcl-plugin-test
**Depends on:**
- bc7d2bfe-aec4-4314-a880-e1320ae7cd0a — Publish releases in the new layout from the plugin template
**Execution:** agent

Copy the template's release build and workflow changes, bump to the new xcl release, and update the README's release section. The test plugin's provider has no replace settings and doesn't change.

*Technical detail:* [context.md#task-move-the-test-plugin-to-the-new-release-layout](./context.md#task-move-the-test-plugin-to-the-new-release-layout)

**Acceptance criteria**:
- [ ] The release build produces zips and an index in the new layout.
- [ ] The test plugin's tests pass unchanged on the new xcl release.
- [ ] The README describes the new release layout.

#### - [ ] Task: Publish the Docker plugin's new release
**Id:** e6f9e274-3cfe-43d9-b28f-cc99fe8fd1a0
**Repo:** xcl-plugin-docker
**Depends on:**
- 40cff90a-96e2-417f-bfab-8a31ff5ce7d5 — Move the Docker plugin to the new release layout
**Execution:** human — releasing needs pushing a version tag, which runs the workflow with the signing-key secrets

Push the Docker plugin's next version tag so a signed release in the new layout is published.

*Technical detail:* [context.md#task-publish-the-docker-plugins-new-release](./context.md#task-publish-the-docker-plugins-new-release)

**Acceptance criteria**:
- [ ] The Docker plugin's new release is published with an index, its signature and a zip per platform.

#### - [ ] Task: Publish the test plugin's new release
**Id:** db0022b3-b45f-4beb-a5df-108e8dc59b27
**Repo:** xcl-plugin-test
**Depends on:**
- a6d3deb6-98ce-4830-ab07-b6d328ac2a1b — Move the test plugin to the new release layout
**Execution:** human — releasing needs pushing a version tag, which runs the workflow with the signing-key secrets

Push the test plugin's next version tag so a signed release in the new layout is published for the Python SDK and xcl's live test to use.

*Technical detail:* [context.md#task-publish-the-test-plugins-new-release](./context.md#task-publish-the-test-plugins-new-release)

**Acceptance criteria**:
- [ ] The test plugin's new release is published with an index, its signature and a zip per platform.

### Milestone 3: Everything points at the new releases

**What changes**: xcl's remote-plugin example, the Python SDK's integration tests and README, and the documentation site all use the plugins' new releases and describe the new layout. Nothing anywhere describes or pins the old layout.

**Validation point**: the Python SDK's integration suite installs the new test-plugin release on its CI matrix, the example runs against the Docker plugin's new release, and the website builds with the updated pages.

#### - [ ] Task: Point xcl's remote-plugin example at the Docker plugin's new release
**Id:** 9e007f42-f82d-43ea-b968-755c8e8c6ebc
**Repo:** xcl
**Depends on:**
- e6f9e274-3cfe-43d9-b28f-cc99fe8fd1a0 — Publish the Docker plugin's new release
**Execution:** agent

Change the remote-plugin example and any xcl doc that names a Docker plugin release so they pin the new release, which the new registry can install.

*Technical detail:* [context.md#task-point-xcls-remote-plugin-example-at-the-docker-plugins-new-release](./context.md#task-point-xcls-remote-plugin-example-at-the-docker-plugins-new-release)

**Acceptance criteria**:
- [ ] The remote-plugin example and xcl's docs pin the Docker plugin's new release, and none pins an old-layout release.

#### - [ ] Task: Move the Python SDK's tests and README to the new releases
**Id:** a90a1c0b-3c63-4957-a539-62b880d54c86
**Repo:** xcl-python
**Depends on:**
- db0022b3-b45f-4beb-a5df-108e8dc59b27 — Publish the test plugin's new release
- e6f9e274-3cfe-43d9-b28f-cc99fe8fd1a0 — Publish the Docker plugin's new release
**Execution:** agent

Pin the test plugin's new release. Update the integration helpers that describe a cache entry and its release files, and rewrite the tamper tests to tamper with the index and the zip. Update the README's Docker plugin pin. This lands on top of the engine bump that carries xcl's new registry.

*Technical detail:* [context.md#task-move-the-python-sdks-tests-and-readme-to-the-new-releases](./context.md#task-move-the-python-sdks-tests-and-readme-to-the-new-releases)

**Acceptance criteria**:
- [ ] The integration tests install the test plugin's new release, verified against the jumppad-labs key, and apply through it.
- [ ] A tampered index or zip is refused, an untrusted signature is refused, and an unsigned release installs without keys.
- [ ] The README pins the Docker plugin's new release and describes no old-layout files.

#### - [ ] Task: Describe the new release layout on the documentation site
**Id:** eb8666c3-a7fa-4bbd-a0ef-ad5514aeec53
**Repo:** xcl-website
**Depends on:**
- e6f9e274-3cfe-43d9-b28f-cc99fe8fd1a0 — Publish the Docker plugin's new release
- 0f9236c4-3d3d-449e-9a01-d6cd28d77794 — Publish a template release in the new layout
**Execution:** agent

Update the GitHub registry, plugin template, registries, home and replacement pages:

- describe the release index, per-platform zips, checksums in the index and the signature over the index;
- describe the new cache contents and errors;
- pin the new releases;
- show `ReplaceOn` where the pages showed hand-written replace loops.

*Technical detail:* [context.md#task-describe-the-new-release-layout-on-the-documentation-site](./context.md#task-describe-the-new-release-layout-on-the-documentation-site)

**Acceptance criteria**:
- [ ] The site's registry and plugin-template pages describe the new release layout, cache and errors.
- [ ] No page describes per-platform tar.gz archives, a checksums file or a signature over a checksums file, or pins an old-layout release.
- [ ] The replacement page shows replace settings declared with `ReplaceOn`.
- [ ] The site builds.

## Open Questions

- **Whether `zip -q -X` on the GitHub Actions Ubuntu runner keeps the binary's executable bit.** This depends on the runner's zip build, and only shows up in a real CI run. The registry sets 0755 on the binary named by the manifest's `command` either way, so installs don't depend on it. If the template's CI `dist` job shows the bit is lost, the implementer notes it in the template README, and doesn't add any workaround beyond the registry's.
- **Whether the Python SDK's automatic engine bump fails on the old integration tests** once xcl's new registry is pinned, because the pinned test-plugin release is still old-layout. This depends on the order the bump PR and the test-plugin release land in, which is known only at release time. If it fails, the Python task's changes land on the `engine/<tag>` branch, as the release-chain knowledge describes. If the test-plugin release isn't published yet, STOP and ask the user how to sequence it.

## Out of Scope

- **Choosing and starting container plugins.** Container entries are validated only. Starting them waits for the container-launch and protocol 2 transport designs (the "Not covered" items in `plugin-release-assets-v2.md`).
- **Plugin protocol 2 and its stdin/stdout transport.** Plugins keep speaking protocol 1, and indexes list `protocol = [1]`. A transport design will revise `libxcl.md`.
- **Python and TypeScript plugins.** `python-plugins.md` becomes its own spec once the transport is designed.
- **Types in the index**, fetching only the plugins a configuration uses, and generating code from an index. These are later work.
- **OCI distribution** and a command-line tool for building or publishing plugins.
- **The release plan.** It is written with the `release-plan` skill after implementation, per the release-plan convention, not as a task here.
- **Restoring the TypeScript spec workflow.** That's housekeeping done when this planning session ends, not part of the plan.
