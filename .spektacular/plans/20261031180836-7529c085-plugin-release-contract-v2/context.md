---
created_date: "2026-10-31"
document_status: final
closed_date: "2026-10-31"
---

# Context: 20261031180836-7529c085-plugin-release-contract-v2

## Current State Analysis

- **xcl GitHub registry** (`registry/github*.go`): installs the first release layout. It reads a per-platform `.tar.gz`/`.zip` with the binary at the root, a required `_checksums.txt` and an optional `_checksums.txt.sig`. It caches under `<cache>/github.com/<owner>/<repo>/<tag>/<os>_<arch>/` with a staging-then-rename transaction and re-verifies the archive, checksums, signature and binary on every start (`registry/github_install.go:39-187`, `registry/github_verify.go:32-278`). Asset names are centralised in `registry/github_assets.go:53-94`.
- **The fake release server** `internal/testutil/github_release.go` builds old-layout releases. It is used by the registry, root-package, coreservice and example tests.
- **Decoding:** the registry cannot use `xcl.NewConfig` (the root package imports `registry`), but it already imports internal packages, and `gohcl`/`hclsyntax` decode static files with no evaluation context. Commit 9e9893d makes two-label nested list blocks unique by all labels. Commit 348ec83 adds `DefaultChanged.ReplaceOn`.
- **Plugin repos** (template, docker, test): `make dist` builds tar.gz/zip plus `sha256sum` checksums. The release workflow signs the checksums file and publishes with `gh release create`. All pin xcl v0.9.0. Docker is at v0.3.0 and test at v0.2.0. The template and Docker providers hand-write replace loops; the test plugin has none.
- **xcl-python:** pins xcl-plugin-test v0.2.0 in `tests/plugins.toml`. Its integration helpers and tamper tests assume the archive, checksums and signature names. The README pins docker v0.3.0. The engine pin `xcl-ref.txt` is v0.12.0.
- **xcl-website:** the github-registry, plugin-template, registries, index and replacement pages describe the old layout and hand-written replace loops.

## Per-Task Technical Notes

### Task: Serve the new release layout from the fake release server

Spec requirements served: test support for every install requirement.

- `internal/testutil/github_release.go:29-480` — rewrite `NewFakeGitHubRelease`'s `build` (:260):
  - for each of `FakeReleasePlatforms` (:29), write a zip named `<name>_<version>_<os>_<arch>.zip` with `zip.Store` holding the fixture binary (mode 0755 via `SetMode`, `.exe` on windows), README.md, and an `xcl-plugin.xcl` manifest (`plugin { name, version }`, `executable { protocol = [1], command = ["<binary>"], platform "<os>" "<arch>" {} }`);
  - write `<name>_<version>_index.xcl` with each platform's `asset` and `checksum = "sha256:<hex>"`;
  - sign the index with `openpgp.ArmoredDetachSign` (:306) into `<name>_<version>_index.xcl.sig`.
- `internal/testutil/github_release.go:227-243` — replace `ArchiveName`/`ChecksumsName`/`SignatureName` with `IndexName()`, `SignatureName()`, `ZipName(platform)`, and `Zip(platform)` replacing `Archive`.
- `internal/testutil/github_release.go:83-121` — options:
  - keep `WithoutPlatform`, `WithoutSignature`, `SignedByOtherKey`, `RequireToken`, `Draft`;
  - replace `WithoutChecksums` with `WithoutIndex()` and `WithoutChecksum(platform)`, and `TamperArchive` with `TamperZip(platform)`;
  - add `ManifestDisagrees(platform, field)` (name/version/protocol/platform), `WithUnknownKind()` (adds `wasm { protocol = [3] }`), `WithContainer(image, platforms...)` and `IndexExpression()` (writes `version = upper("1.0.0")`).
- `internal/testutil/github_release.go:443-480` — drop `tarGzArchive`. Keep a zip writer.
- `internal/testutil/github_release_test.go` — rewrite the fake's own tests: names served, index checksums equal the zips' sha256, the signature verifies against `PublicKey()`, and each option's effect (one test per option).
**Complexity**: Medium
**Token estimate**: ~40k
**Agent strategy**: single agent, sequential (one file and its test).

### Task: Read and validate release indexes and build manifests

Spec requirements served: incomplete indexes refused, static descriptions, unknown kinds ignored, container entries validated, unsupported machines' error, builds agree with their index.

- `registry/github_release_files.go` (new) — `releaseIndex`, `indexPlugin`, `indexExecutable`, `indexPlatform`, `indexContainer`, `imagePlatform`, `buildManifest` (see the plan's Data Structures). Decode with `hclsyntax.ParseConfig(src, filename, hcl.InitialPos)` and `gohcl.DecodeBody(body, nil, &v)` (internal/xcl/gohcl/decode.go:38), with a `xcl:",remain"` body field to skip unknown kinds (decode.go:106-121). The nil EvalContext refuses variables (internal/xcl/traversal.go:94) and calls (internal/xcl/hclsyntax/expression.go:247), so return those diagnostics wrapped in `ErrPluginVerification`. The doc comment names the "static xcl metadata file" pattern.
- `registry/github_release_files.go` — `validate(repository, tag)`:
  - `plugin.name` equals the repository's name part and `plugin.version` equals the tag without `v` (rule in github_assets.go);
  - every executable platform has `checksum` matching `^sha256:[0-9a-f]{64}$` and a non-empty `asset`;
  - container `image` matches `^.+@sha256:[0-9a-f]{64}$`, and every container platform's OS is `linux`;
  - at least one of executable or container is present;
  - the platform OS/arch values are among linux/darwin/windows and amd64/arm64.
- `registry/github_release_files.go` — `choose(host)` returns the executable platform equal to the host. Otherwise ErrPluginNotFound with the offered platforms and the host, which also covers container-only releases. `agrees(index, host)` compares name, version, protocol (equal lists) and the manifest's single platform against the index entry.
- `registry/github_assets.go:21-94,103` — reduce to the tag rule, `indexName`, `signatureName`, and the protocol check (accept builds whose protocol list contains 1). Remove `namesFor`'s archive/checksums/binary naming and `parseChecksums`. Keep `platform.supported` (:41) for the host check.
- `registry/github_release_files_test.go` (new) — one test per acceptance criterion, with literal index text. Delete the old-name and checksums tests in `registry/github_assets_test.go`.
**Complexity**: Medium
**Token estimate**: ~35k
**Agent strategy**: single agent, sequential.

### Task: Install plugins from releases in the new layout

Spec requirements served: releases described by an index, signature over the release, only this machine's build, builds verified, builds agree, cache re-verified, container not run.

- `registry/github_install.go:55-187` — `install`/`download`: keep the staging directory (:121) and rename (:179-187). Fetch the release (:132) and the index asset (ErrPluginNotFound when absent: "not a plugin release in the supported layout"). When keys are trusted, fetch `.sig` (missing → ErrPluginVerification) and verify before validating. Then validate, choose, download only the chosen zip, check its sha256 against the index, read `xcl-plugin.xcl` from the zip, check it with `agrees`, and extract all regular files under the entry root (refusing absolute or `..` paths), setting 0755 on the file named by `command[0]`. Keep the structured `load` events.
- `registry/github_install.go:93,98-107` — return the path `entry/<command[0]>` plus `command[1:]`. The entry layout is unchanged.
- `registry/github_verify.go:32-278` — `verifyEntry`: verify the signature over the cached index (when keys are trusted), re-validate the index, check the cached zip against its checksum, and check the extracted command binary against its copy in the zip (rework `verifyBinary` :156 and `withZipBinary` :253). Remove `verifyArchive`'s checksums-file logic, `withTarGzBinary` (:219) and the tar and gzip imports. Keep `verifySignature` (:87) and `signatureIssuer` (:127), now naming the index signature.
- `registry/github_install.go:39-50` — `Start` calls `Executable(binary, args...).Start(emit)`.
- `registry/registry.go:117-140` — `Executable(path string, args ...string)`. Store the args and pass them to the host.
- `plugins/grpc_plugin_host.go:61,105` — let the host start a command with arguments (e.g. `Start(pluginPath string, args ...string)` using `exec.Command(pluginPath, args...)`). Check first that no interface declares `Start(string) error`; at planning time none was found.
- `registry/github_install_test.go`, `registry/github_signature_test.go`, `registry/github_verify_test.go` — rewrite on the new fake. One function per behaviour in the plan's Testing Approach. Keep `requireNoStagingDirectories`. Use the request counting (`Requests()`) to prove no build is fetched before refusal and exactly one build is fetched.
- `config_github_registry_test.go:27-70` — move the helpers to the new fake and keep the apply/destroy tests. Add a Config test for a container-only release failing with ErrPluginNotFound.
- `coreservice/config_test.go:125,141`, `example/remoteplugin/release_test.go:70` — adjust to the fake's renamed accessors. Behaviour is unchanged.
- `registry/registry_test.go` (or the existing Executable tests) — a test that `Executable(path, "--flag")` starts the process with the argument (use a fixture binary that records its args, or check `os.Args` in a test fixture).
- `registry/github_live_test.go:18` — no code change. It runs against the new test-plugin release once published.
**Complexity**: High
**Token estimate**: ~90k
**Agent strategy**: parallel analysis of install, verify and tests, then sequential integration: install and verify first, then the tests, then the coreservice and example fixes.

### Task: Describe the new release layout in xcl's docs

Spec requirements served: documentation describes the new layout.

- `docs/plugins.md:355-395` — rewrite the GitHub registry section: what a release must contain (index, zips, checksums in the index, optional signature over the index), the cache entry contents (index, signature, zip, extracted files), re-verification on start, and which failures produce `ErrPluginNotFound` and which `ErrPluginVerification`.
- `docs/plugin-layout.md:215-232` — the "Releasing" section lists the new assets.
- `README.md:170-235` — check the GitHub-registry paragraph and example list. Remove any old-layout wording.
- The changelog entry is written by the implement workflow's changelog step, not here.
**Complexity**: Low
**Token estimate**: ~15k
**Agent strategy**: single agent.

### Task: Name the plugin handshake cookie XCL_PLUGIN

Spec requirements served: local plugins run from a command (the handshake a hand-written plugin checks), and the existing plugins are re-released.

- `plugins/grpc_plugin.go:89-92` — `MagicCookieKey: "XCL_PLUGIN"`, value `xcl` unchanged, `ProtocolVersion: 1` unchanged. Host (`plugins/grpc_plugin_host.go:103`) and server (`plugins/serve.go:23`) both use `HandshakeConfig`, so this is the only code change.
- Search the repo for `HCLCONFIG` and fix any test, fixture or doc that names it (at planning time only grpc_plugin.go did). Fixture plugins under `internal/test_fixtures` and `bindings/libxcl/testdata` are rebuilt from source by the tests, so they follow automatically.
- Tests: the existing external-plugin and GitHub-registry tests start real plugin binaries built from this module, which proves host and plugin agree. Add a test that a plugin binary started without `XCL_PLUGIN` refuses to serve (go-plugin prints its "not meant to be executed directly" message and exits non-zero), as a separate function from the positive case.
- The `libxcl.md` design quotes the old cookie; the protocol 2 transport design revises that section, so it isn't edited here.
- Convention: names outside the code carry the `XCL_` prefix (decisions/xcl-is-its-own-language.md).
**Complexity**: Low
**Token estimate**: ~8k
**Agent strategy**: single agent.

### Task: Release xcl with the new registry

Human task. Open a PR from `f-python-plugin` (or the implement branch) to `main` carrying the registry change, `ReplaceOn` (348ec83), the nested-block fix (9e9893d) and the `XCL_PLUGIN` cookie. Label it `minor`, since before 1.0 a breaking change is `minor` (release-chain knowledge). Merge it. xcl's `tag.yml` pushes the tag, and `release.yml` publishes and dispatches xcl-python's `engine-bump.yml`.

Check: the GitHub release exists for the new tag, and xcl-python has an `engine/<tag>` PR open. Note the tag; the plugin tasks bump to it.

### Task: Publish releases in the new layout from the plugin template

Spec requirements served: plugin authors produce a release with one command; existing plugin behaviour unchanged.

- `xcl-plugin-template:Makefile:64-88` — `dist` for each platform:
  - build into `dist/stage/$platform` as today;
  - write `xcl-plugin.xcl` there with `printf` (plugin name `$(PLUGIN)`, version `$(DIST_VERSION)`, `protocol = [1]`, `command = ["$(PLUGIN)$ext"]`, the one platform);
  - copy README and LICENSE;
  - `(cd "$stage" && zip -q -X "../../$archive.zip" *)` for every platform, including linux and darwin. Check that the mode is kept; the registry sets 0755 regardless;
  - after the loop, write `dist/$(PLUGIN)_$(DIST_VERSION)_index.xcl` with the `plugin` and `executable` blocks and one `platform` per zip carrying `asset` and `checksum = "sha256:$(sha256sum <zip> | cut -d' ' -f1)"`.

  Remove the tar and checksums lines (:83, :87). Update the comment block (:64-68).
- `xcl-plugin-template:.github/workflows/release.yml:49-55` — sign `dist/*_index.xcl` into `$index.sig` with the same gpg flags. Keep the key check (:22-29) and `gh release create ... dist/*` (:57-60).
- `xcl-plugin-template:.github/workflows/ci.yml:34-51` — the `dist` job is unchanged (`make dist VERSION=v0.0.0-snapshot`, `ls -l dist/`).
- `xcl-plugin-template:go.mod:6` — bump `github.com/jumppad-labs/xcl` to the new release tag, then `go mod tidy`.
- `xcl-plugin-template:providers/note.go:28-35,41,107-133` — the constructor sets `DefaultChanged: plugins.DefaultChanged[*entities.Note]{ReplaceOn: ReplaceSettings}`. Remove the loop (:111-117). Keep the dependency-replace rule (:122-126) and the "changes → update" rule, then defer to `p.DefaultChanged.Changed`. Check that the constructor is the only place `NoteProvider` is built.
- `xcl-plugin-template:providers/note_test.go:161-240` — unchanged. They must pass.
- `xcl-plugin-template:README.md:155-179,312-387` — the change-decision section shows `ReplaceOn`; the release asset list (:324-330), tools list (:339-340), install and verifying sections describe the index, zips and index signature.
**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: single agent, sequential. Run `make dist VERSION=v0.0.0-snapshot` locally and inspect the index and a zip, then run `make test`.

### Task: Publish a template release in the new layout

Human task. In xcl-plugin-template, run `make release VERSION=vX.Y.Z` (next version after the template's current release) to push the tag. `release.yml` builds, signs the index with the `GPG_PRIVATE_KEY`/`GPG_PASSPHRASE` secrets and publishes. Check: the release has `xcl-plugin-template_<v>_index.xcl`, `.sig`, and six zips, and the signature verifies with `gpg --verify` against https://xcl.dev/keys/jumppad-labs-releases.asc.

### Task: Move the Docker plugin to the new release layout

Spec requirements served: existing plugins re-released; behaviour unchanged.

- `xcl-plugin-docker:Makefile:61-84` — the same `dist` change as the template (only `PLUGIN` and `./cmd/docker` differ).
- `xcl-plugin-docker:.github/workflows/release.yml` — the same signing change as the template. The file stays byte-identical to the template's.
- `xcl-plugin-docker:go.mod:8` — bump xcl to the new release, then `go mod tidy`.
- `xcl-plugin-docker:providers/network.go:19-35,62-81,130-134` — the constructor sets `ReplaceOn: networkReplaceSettings`. Remove the loop (:69-75). Keep `if len(changes) > 0 { return entity.Update, nil }`, then the default.
- `xcl-plugin-docker:providers/container.go:97-133,315-320` — the constructor sets `ReplaceOn: replaceSettings`. Remove the loop (:108-114). Keep the dependency rules (:116-130) and the update rule.
- `xcl-plugin-docker:providers/network_test.go:356-389`, `container_test.go:481-604` — unchanged. They must pass.
- `xcl-plugin-docker:README.md:48,86,181-197` — pins to the plugin's upcoming release, and the release section describes the new layout.
**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: single agent. Run `make test`, `make e2e` where Docker is available, and `make dist VERSION=v0.0.0-snapshot`.

### Task: Move the test plugin to the new release layout

Spec requirements served: existing plugins re-released.

- `xcl-plugin-test:Makefile:64-87` — the same `dist` change as the template (only `PLUGIN` and `./cmd/xcl-plugin-test` differ).
- `xcl-plugin-test:.github/workflows/release.yml` — the same signing change.
- `xcl-plugin-test:go.mod:6` — bump xcl to the new release.
- `xcl-plugin-test:README.md:4-6,86-101` — describe the index, zips and index signature.
- `xcl-plugin-test:providers/item.go` — no change.
**Complexity**: Low
**Token estimate**: ~15k
**Agent strategy**: single agent.

### Task: Publish the Docker plugin's new release

Human task. In xcl-plugin-docker, `make release VERSION=v0.4.0` (the next minor after v0.3.0). Check: the release holds the index, its signature and six zips, and xcl's live test passes against it: `XCL_GITHUB_LIVE_PLUGIN=jumppad-labs/xcl-plugin-docker@v0.4.0 XCL_GITHUB_LIVE_KEY=<key file> make test-github-live`.

### Task: Publish the test plugin's new release

Human task. In xcl-plugin-test, `make release VERSION=v0.3.0` (the next minor after v0.2.0). Check: the release holds the index, its signature and six zips, and `XCL_GITHUB_LIVE_PLUGIN=jumppad-labs/xcl-plugin-test@v0.3.0 make test-github-live` passes in xcl with the jumppad-labs key.

### Task: Point xcl's remote-plugin example at the Docker plugin's new release

Spec requirements served: users of the existing plugins move to the new releases.

- `example/remoteplugin/main.go:167-173` — pin `jumppad-labs/xcl-plugin-docker` to the release published by the Docker release task (v0.4.0 expected).
- `example/remoteplugin/README.md` (if it names the tag), `README.md:176-234` — update any Docker plugin tag.
**Complexity**: Low
**Token estimate**: ~8k
**Agent strategy**: single agent. Run the example's tests.

### Task: Move the Python SDK's tests and README to the new releases

Spec requirements served: users of the existing plugins move to the new releases.

- `xcl-python:tests/plugins.toml:8` — `version = "v0.3.0"` (the test plugin's new release).
- `xcl-python:tests/integration/release_helpers.py:37-41,77-120,164` — the comment says the key signs the release's index. `ReleaseFiles` fields become `index_name`, `signature_name` (`<name>_<bare>_index.xcl.sig`) and `zip_name` (`<name>_<bare>_<goos>_<goarch>.zip`); drop `archive_name`/`checksums_name`. The cache entry holds the index, signature, zip, the extracted binary, `xcl-plugin.xcl`, README and LICENSE.
- `xcl-python:tests/integration/test_test_plugin.py:87,103-112` — the directory test asserts the entry's files under the new names. Only the binary is executable; check that the sidecars don't match the `xcl-plugin-*` pattern as executables.
- `xcl-python:tests/integration/test_github_tamper.py:131-243` — `real_assets` serves the index, signature and zip. Tamper the index's checksum line for the zip (a signature failure with keys) and tamper the zip's bytes (a checksum failure). The untrusted-key test is unchanged. In the unsigned test, assert the index and zip are cached and the signature is not.
- `xcl-python:README.md:66,711,751-757` — pin docker `v0.4.0`. The signature wording now refers to the release index.
- This lands after the `engine/<xcl tag>` bump PR merges, or on that branch if the bump's checks fail on the old tests (gotchas/ci-regenerates-stubs.md in the xcl-python store).
**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: single agent. Run the integration suite with `GITHUB_TOKEN=$(gh auth token)` (learnings/test-suite-in-a-spec-worktree.md).

### Task: Describe the new release layout on the documentation site

Spec requirements served: documentation describes the new layout; users move to the new releases.

- `xcl-website:src/pages/github-registry.mdx:34,50,110-129,131-168,202-245,256-257` — new tags (docker v0.4.0); "What a release must contain" lists the index, zips, checksums in the index and the optional index signature; the signing section says the index is signed; the cache tree shows the index, `.sig`, zip and extracted files; the errors line is reworded.
- `xcl-website:src/pages/plugin-template.mdx:84-92,155-174,186-249` — `ReplaceOn` snippet and rules; the release section lists the new assets and that the workflow signs the index; template pins to its new release.
- `xcl-website:src/pages/registries.mdx:329-333`, `index.mdx:286-288` — docker v0.4.0, and "checked against its index checksum and signature".
- `xcl-website:src/pages/replacement.mdx:70-86,244-310` — DefaultChanged with `ReplaceOn`; the network example uses `ReplaceOn`, and the container example keeps its dependency rules with `ReplaceOn` for the settings.
**Complexity**: Medium
**Token estimate**: ~25k
**Agent strategy**: single agent. Run the site build (`npm run build`).

### Task: Register a local plugin as a command

Spec requirements served: local plugins run from a command.

- `registry/local.go:178-183` — `RegisterExternalPlugin(command string, args ...string)` adds `Executable(command, args...)`. Update the doc comment and the package example at :61 to show `RegisterExternalPlugin("python3", "plugin.py")` alongside a binary path. Callers passing a single path (`example/plugin/main.go:237`, `bindings/libxcl/cmd/parity/main.go:200`) need no change.
- `registry/registry.go:117-133` — rename `Executable` to the private `executablePlugin` and update its callers in `registry/local.go`, `registry/github_install.go` and the registry tests; drop it from `docs/plugins.md:321-323`. `executable.Name()`: with no arguments, `plugins.PluginBinaryName(path)` as today; with arguments, the full command line, the program and its arguments joined by spaces (spec amendment: a plugin's name is only a label in load events and errors). The GitHub registry is unaffected, because `githubPlugin.Name()` (registry/github_install.go:30) names by repository.
- `proto/xcl/core/v1/core.proto:206-212` — add `message ExternalPlugin { repeated string command = 1; }`; in `LocalRegistry`, retire field 3 by reserving both its number and its name (`reserved 3; reserved "external_plugins";`), since CI's `buf breaking` refuses a removal that doesn't reserve both (gotchas/buf-field-no-delete.md in the xcl store); this is to satisfy CI, not for compatibility, and add `repeated ExternalPlugin external_plugin_commands = 6;`. Regenerate with `make core-protos`, then run `make buf-lint` and `make buf-breaking`.
- `coreservice/config.go:187-189` — for each entry of `GetExternalPluginCommands()`, refuse an empty command with the existing invalid-argument error style, then call `RegisterExternalPlugin(command[0], command[1:]...)`.
- `internal/test_fixtures/plugins/launcher` (new) — a tiny Go program that execs the binary named by its first argument with the rest, passing its environment through (`syscall.Exec` on Unix; on Windows run it as a child, wire stdio and exit with its code). Build it with a `testutil` helper beside `BuildFixturePlugin` (internal/testutil/plugins.go:47).
- Tests: `registry/local_test.go` — a command with arguments starts the fixture plugin through the launcher and its `Name()` is the full command line; a single path keeps the binary name. Root package — a Config test applies and destroys through a plugin registered as launcher plus plugin. `coreservice/config_test.go` — a command in `LocalRegistry` starts the plugin; an empty command is refused (separate function).
- Docs — `docs/plugins.md:330-345`, `docs/plugin-layout.md:59` and `README.md:161`: show the command form beside the binary path. Reference docs only; the guide to writing a non-Go plugin is a follow-up spec.
- go-plugin notes for whoever writes a Python plugin by hand: the host sets `XCL_PLUGIN=xcl` (plugins/grpc_plugin.go:89-92, renamed in milestone 1); the plugin serves `PluginService` from `plugins/plugin.proto` and the gRPC health service with service name `plugin` reporting SERVING, then prints `1|<app protocol version>|tcp|127.0.0.1:<port>|grpc` on stdout. The log and state callbacks go through go-plugin's broker and are unavailable without it.
**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: single agent, sequential: the registry and naming first, then the proto and core service, then the docs.

### Task: Register a local plugin as a command from Python

Spec requirements served: local plugins run from a command.

- `xcl-python:xcl/registry.py:42,82-84,104` — `_external_plugins: list[list[str]]`. `register_external_plugin(command)` accepts `str | os.PathLike[str]` (a one-word command) or a non-string sequence of them (`os.fspath` each); refuse an empty sequence with the SDK's existing error type for bad registrations. `_to_proto` sends `external_plugin_commands=[core_pb2.ExternalPlugin(command=c) for c in ...]`. Check whether the asyncio API has its own registry, and change it the same way if so.
- Build against the local xcl checkout: `make` targets with `XCL_SRC=../xcl` regenerate the protos and build `libxcl` and `xcl-core` from source (Makefile:1-45). Don't bump `xcl-ref.txt`; the engine bump comes with an xcl release.
- Tests: a unit test for each accepted form and for the empty-sequence refusal (separate functions). An integration test registers `[sys.executable, <launcher.py>, <fixture plugin>]`, where `launcher.py` is a test helper that `os.execv`s its argument (child process plus exit code on Windows), and applies through it in the sync and the asyncio API. Existing `register_external_plugin(path)` tests stay unchanged.
- `xcl-python:README.md` — the registering-plugins section shows `register_external_plugin(["python3", "plugin.py"])` beside a path.
- Leave the work on its branch: it can't merge until an xcl release with the proto change is pinned in `xcl-ref.txt` (see the plan's open questions).
**Complexity**: Low
**Token estimate**: ~20k
**Agent strategy**: single agent. Run the suite with the local engine built from `XCL_SRC`.

## Testing Strategy

Per task:

- **Fake release server:** its own unit tests, one per option.
- **Reader and chooser:** registry unit tests with literal index text, one per validation rule and per agreement field.
- **Installer:** registry integration tests on the fake server, one per behaviour, using request counts and `requireNoStagingDirectories`; root-package Config tests apply and destroy through an installed plugin; the coreservice and example tests keep passing.
- **xcl docs:** reviewed by hand (manual item).
- **Template, Docker and test plugins:** existing provider `Changed` tests unchanged and passing, plus e2e suites and CI `dist` jobs.
- **Python SDK:** integration suite against the new test-plugin release.
- **Website:** site build plus a manual page review.
- **Local commands:** registry, root-package and coreservice tests through a launcher fixture; Python SDK unit and integration tests built against the local xcl checkout.

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

- **Plugin repositories:** existing provider `Changed` tests stay untouched and must pass after moving to `ReplaceOn`, which proves the constraint "rebuilding must not change behaviour". The template, Docker plugin and test plugin keep their e2e suites. Each repo's CI `dist` job runs `make dist` on every push, so a broken release build fails CI.
- **Python SDK:** the integration tests (pinned test-plugin install, tamper tests rewritten to tamper with the index and zip, unsigned install) run against the new release once it exists.

**Deliberate gaps:**

- No test reads docs, README, workflow or Makefile text (project convention). Docs and release tooling are checked by running them and by review.
- Darwin and Windows installs aren't exercised in CI, which runs Linux.

**Success metrics:**

- **"A new release is produced from the template with one release command and the workflow, no hand-written files"** — partly behavioural: the CI `dist` job builds a complete release, zips plus index, from `make dist` alone. Publishing a real release is **Manual — captured in the implementation test plan**.
- **"Pushing a version tag produces a verifiable release with no manual step"** — **Manual — captured in the implementation test plan** (push a tag on each plugin and confirm the published release verifies).
- **"Both plugins install from their new releases, from Go and Python, verified against the published key, on every CI platform"** — behavioural through the Python SDK's integration tests on its CI matrix and the opt-in live test. Running the live test against the published Docker and test releases on darwin and windows is **Manual — captured in the implementation test plan**.

**Manual reviews:**

- **Manual — captured in the implementation test plan**: review the docs in xcl, the template, the Docker and test plugin READMEs, the Python SDK README and the website pages for accuracy against the new layout, with no old-layout wording left.
- **Manual — captured in the implementation test plan**: run the documented getting-started example against the Docker plugin's new release and confirm it creates and destroys its network and container.
- **Manual — captured in the implementation test plan**: build the website and check the changed pages in a browser.

## Project References

- Spec: `20261031180836-7529c085-plugin-release-contract-v2`.
- Designs (`design` source): `plugin-release-assets-v2.md` (binding), `plugin-layout.md` (ReplaceOn rule), `plugin-release-assets.md` (superseded, archaeology).
- Knowledge (repo tier, xcl store): conventions (config-files-are-xcl, testing-and-mocking, shared-test-helpers, shared-errors-package, testing-provider-changed-and-update, never-modify-dependencies, release-plan-after-implementation); architecture/release-chain.md; gotchas/nested-blocks-named-by-first-label.md. In the xcl-python store: learnings/test-suite-in-a-spec-worktree.md, gotchas/ci-regenerates-stubs.md.
- Repo roots (`spektacular repo list`): xcl `/home/nicj/code/github.com/jumppad-labs/xcl`, xcl-plugin-template `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-template`, xcl-plugin-docker `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-docker`, xcl-plugin-test `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test`, xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`, xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

- **No backwards compatibility.** Once xcl is released, applications pinning an old-layout plugin release fail to load it with `ErrPluginNotFound` ("not a plugin release in the supported layout"). Applications must move their pins to the new releases (docker v0.4.0, test v0.3.0).
- **Breaking changes are expected** (xcl isn't stable yet). Plugins built against an earlier xcl check `HCLCONFIG_PLUGIN` and won't start under the new host, and SDKs built against the old CoreService proto must be regenerated. The Docker and test plugins are rebuilt here, and the Python SDK regenerates its stubs from xcl.
- **Existing cache entries** under `~/.xcl/cache/plugins` for old-layout releases are never read again; an old tag pin fails before the cache is consulted. Nothing is pruned, as today.
- **CoreService field 3 is reserved.** `LocalRegistry.external_plugins` (strings) is replaced by `external_plugin_commands` (commands).
- **Release order:** xcl, then the template, Docker and test plugin releases, then the xcl example, xcl-python pins and the website.

## Performance Considerations

An install downloads the small index (plus signature) and one zip instead of one archive plus the checksums file, so bytes per install are roughly unchanged and don't grow with the number of platforms. Per-start verification hashes the cached zip and the extracted binary, as today.
