---
created_date: "2026-10-31"
document_status: final
closed_date: "2026-10-31"
---

# Research: 20261031180836-7529c085-plugin-release-contract-v2

## Alternatives considered and rejected

- **Decode the index through `xcl.NewConfig` with registered singleton types.** Rejected because of an import cycle: the root package imports `registry` (options.go:22), so `registry` cannot call `xcl.NewConfig`. Config also scans the file's directory for `.vars` files and reads `XCL_VAR_` environment variables (internal/parser/util.go:76), there is no bytes API (parser.go:816), and Apply loads plugins (parser.go:682), none of which a static release file should do. A spike in this session confirmed the shape decodes through Config (singletons plus two-label nested platforms), but the cycle rules it out for the registry.
- **A new `ParserOptions` field that disables functions and variables, plus an "ignore unknown blocks" option.** Rejected as unnecessary. Neither exists today (parser.go:137-193), and the lighter decoder below gives both behaviours with no parser change.
- **Keep a checksums file next to the index.** Rejected by the design and the user ("no checksum, no work": checksums live in the index, and the signature is over the index).
- **Keep the old tar.gz path for backwards compatibility.** Rejected by the user ("No backwards compatibility").
- **Rewrite every Docker provider's `Changed` as `ReplaceOn` only.** Rejected: the container's `Changed` has dependency rules (xcl-plugin-docker:providers/container.go:116-130) and the network's answers update for any change, including `depends_on`, which the JSON comparison ignores (providers/network.go:62-81, test network_test.go:373). Only the replace-settings loops move to `ReplaceOn`. The rest stays, to preserve behaviour.

## Chosen approach — evidence

- **Index decoding with xcl's own decoder, in the registry package:**
  - `hclsyntax.ParseConfig(src, filename, hcl.InitialPos)` and `gohcl.DecodeBody(body, nil, &v)` (internal/xcl/gohcl/decode.go:38). The registry already imports internal packages (internal/declaration, internal/schema), and the catalog uses gohcl.
  - **Expressions:** with a nil EvalContext, variables fail with "Variables not allowed" (internal/xcl/traversal.go:94) and calls with "Function calls not allowed" (hclsyntax/expression.go:247), which gives literals-only. `hclsyntax.VisitAll` (hclsyntax/walk.go:19) is available for an explicit check if a clearer message is wanted.
  - **Unknown kinds:** a `xcl:",remain"` field collects leftover blocks (gohcl/decode.go:106-121), so unknown top-level kinds are skipped rather than refused.
  - **Platforms:** nested `platform "<os>" "<arch>"` blocks with two label fields decode, and two platforms sharing an OS are accepted after commit 9e9893d (gohcl list blocks are keyed by all labels).
- **The install flow to rewrite** is concentrated in registry/github_install.go and github_verify.go:
  - `install` (:55), `download` (:115) with its staging-then-rename transaction (:121, :179-187), `verifyEntry`, `verifyArchive` and `verifySignature` (verify.go:32/42/87, OpenPGP `CheckArmoredDetachedSignature` :101), `verifyBinary` (:156, the per-start re-check), and `extractBinary` with the zip/tar readers (:190-261).
  - Asset naming is centralised in `namesFor`/`assetNames` (github_assets.go:53-94).
  - The cache entry layout `<cache>/github.com/<owner>/<repo>/<tag>/<os>_<arch>/` (install.go:107) can stay.
- **Signature and key handling stay** (github.go:227 `readTrustedKeys`, verify.go:127 `signatureIssuer`); only the signed file changes, from the checksums file to the index.
- **Errors:** `ErrPluginNotFound`, `ErrPluginVerification` and `PluginInstallError{Repository, Version, Platform, Err}` (errors/plugin_install_error.go:14-53), re-exported from the root package (config.go:76-153). New failure modes (no index, incomplete index, manifest mismatch, expression) fit under these two sentinels.
- **Starting the binary:** `Executable(path)` (registry/registry.go:117) starts `plugins.NewGRPCPluginHost(...).Start(path)` with no arguments. The manifest's `command` is a path relative to the zip root, plus optional arguments.
- **Test server:** `testutil.NewFakeGitHubRelease` (internal/testutil/github_release.go:129) hard-codes the old names (:227-243). It builds tar.gz/zip archives (:443/:473) and signs with generated keys (:306, :410). It has options `WithoutPlatform`, `WithoutChecksums`, `WithoutSignature`, `SignedByOtherKey`, `TamperArchive`, `RequireToken` and `Draft` (:83-121). Users: the registry tests, config_github_registry_test.go, coreservice/config_test.go:125,141, and example/remoteplugin/release_test.go:70.
- **Template release tooling:**
  - The `dist` target, xcl-plugin-template:Makefile:69-88: six platforms (:15), `CGO_ENABLED=0 go build -trimpath`, tar/zip, then `sha256sum` into `_checksums.txt`.
  - The release workflow, `.github/workflows/release.yml`:
    - requires `GPG_PRIVATE_KEY` (:22-29);
    - runs `make test` and `make dist VERSION=$GITHUB_REF_NAME`;
    - signs `dist/*_checksums.txt` with `gpg --armor --detach-sign` (:49-55);
    - publishes with `gh release create --verify-tag dist/*` (:57-60).
  - The Docker and test plugins carry byte-identical workflows and the same `dist` logic (Makefiles differ only in `PLUGIN` and the cmd path).
- **ReplaceOn adoption (behaviour-preserving):**
  - **Template:** providers/note.go:107-133 has the loop (:111-117), then a replaced dependency means replace (:122-126), then any change means update (:127-130), then the default. Only the loop moves to `ReplaceOn`.
  - **Docker network:** providers/network.go:62-81. The loop moves, and the "changes → update" rule stays.
  - **Docker container:** container.go:97-133. The loop (:108-114) moves, and the dependency rules stay.
  - **Test plugin:** item.go has no replace list, so no change.

## Files examined

- xcl:registry/github.go:109,140,185,227,243,266 — constructor, `RegisterPlugin`, `Plugins` (keys and token), cache root.
- xcl:registry/github_assets.go:21,41,53,72,103 — tag pattern, supported platforms, asset names, checksums parser.
- xcl:registry/github_install.go:39-187 — `Start`/`install`/`download`, cache entry layout, staging and rename.
- xcl:registry/github_verify.go:32-278 — entry/archive/signature/binary verification, extraction (mode 0755 at :192).
- xcl:registry/github_client.go:17-161 — release-by-tag, asset download, draft/404 → ErrPluginNotFound.
- xcl:registry/registry.go:117-133 — `Executable(path)` and `Start`.
- xcl:registry/github_*_test.go, config_github_registry_test.go — helpers listed in rehydration cues. github_assets_test.go expects tar.gz names.
- xcl:registry/github_live_test.go:18 — opt-in `XCL_GITHUB_LIVE_PLUGIN=owner/repo@tag`, `XCL_GITHUB_LIVE_KEY`. `make test-github-live` (Makefile:84). Not in CI.
- xcl:internal/testutil/github_release.go:29-480 and github_release_test.go — the fake release server and its own tests.
- xcl:internal/testutil/plugins.go:47 — `BuildFixturePlugin` builds internal/test_fixtures/plugins/subtypeless.
- xcl:example/remoteplugin/main.go:167-173 — hard-codes `jumppad-labs/xcl-plugin-docker` `v0.3.0` and the xcl.dev key URL. release_test.go:70 uses the fake.
- xcl:coreservice/config_test.go:125,141 — uses the fake release server.
- xcl:errors/plugin_install_error.go, plugin_load_error.go — sentinels and detail types.
- xcl:internal/parser/parser.go:137-193,682,816,860,890-930 — options, plugin load, file parse, unknown-block errors.
- xcl:internal/xcl/gohcl/decode.go:38,106-121 — DecodeBody, remain handling.
- xcl:docs/plugins.md, docs/plugin-developer-guide.md — plugin and registry documentation (the README's registering-plugins section too).
- xcl-plugin-template:Makefile:8-100; .github/workflows/release.yml, ci.yml (dist job :34-51), current.yml; go.mod:6 (xcl v0.9.0); README:155-179 (change decision), 283-387 (signing, release, install, verifying); providers/note.go:28-35,107-133; providers/note_test.go:161-240.
- xcl-plugin-docker:Makefile (dist :66-84); workflows identical to the template; go.mod:8 (xcl v0.9.0); tags v0.3.0 (latest); providers/container.go:97-133,315-320; network.go:62-81,130-134; tests container_test.go:481-604, network_test.go:356-389; README:48,86 (stale v0.1.1 pins), 181-197 (release layout).
- xcl-plugin-test:Makefile (dist :69-87); tags v0.2.0 (latest); go.mod:6; providers/item.go:24-27 (no replace list); README:4-6,86-101.
- xcl-python:tests/plugins.toml (pins xcl-plugin-test v0.2.0); tests/integration/release_helpers.py:34-164 (cache entry shape, archive/checksums/signature names, key id F775DA00AFD4B502); test_test_plugin.py:87,103-112; test_github_tamper.py:131-243; README:66 (docker v0.3.0), 711, 751-757; xcl-ref.txt (v0.12.0).
- xcl-website:src/pages/github-registry.mdx:34,50,110-168,202-257; plugin-template.mdx:84-92,155-174,186-249; registries.mdx:329-333; index.mdx:22,286-288; replacement.mdx:70-86,244-310 (hand-written replace loops to update to `ReplaceOn`); public/keys/jumppad-labs-releases.asc.

## External references

- HashiCorp go-plugin: the external-plugin handshake used by `Executable`. Unchanged here, but it decides that `command` arguments must reach the process launcher.
- Info-ZIP `zip`: stores Unix permissions in the external attributes by default on Unix hosts, so the executable bit survives `zip -q` in `make dist`. Go's `archive/zip` exposes them through `FileInfo().Mode()`.
- GitHub REST API, release-by-tag and release-asset endpoints: unchanged usage (github_client.go).

## Prior plans / specs consulted

- 20261009102148-7d0b205b-github-releases-registry (plan) — how the registry, fake server, verifier, cache transaction and errors were built. Its conventions list (testutil for shared helpers, errors package, ProtonMail go-crypto, structured `load` events) carries over.
- 20261009092551-82db0140-plugin-template (epic and plan) — origin of `make dist` and the release workflow. Archaeology only.
- Designs: `plugin-release-assets-v2.md` (binding), `plugin-layout.md` (ReplaceOn section, final), `plugin-release-assets.md` (superseded).

## Open assumptions

- Decoding with `gohcl` against fixed structs counts as "read through xcl itself" (the constraint). It is xcl's own decoder, not the Config parser. If the user meant the full Config pipeline, implement must stop and ask.
- `zip -q` on the GitHub Actions Ubuntu runner keeps the executable bit, so extraction can honour the entry's mode. Planned mitigation: the registry sets 0755 on the binary named by `command` regardless.
- The Python SDK's integration tests can move to the new xcl-plugin-test release only after an xcl release containing the new registry, bumped into xcl-ref.txt. The release order is xcl, then the plugins, then the pins.
- The Docker and test plugins must depend on an xcl release that contains `DefaultChanged.ReplaceOn` (commit 348ec83) before they build with it.
- The existing plugins speak protocol 1, so their indexes list `protocol = [1]`, and the registry accepts protocol 1 only for now.

## Drafting assumptions

### Decode release files with gohcl, not Config (discovery)
- **Decision**: the registry decodes the index and manifest with `hclsyntax.ParseConfig` plus `gohcl.DecodeBody(body, nil, &v)` into fixed structs. A `remain` field skips unknown kinds, and the nil EvalContext refuses expressions.
- **Rationale**: the root package imports `registry`, so the registry cannot use `xcl.NewConfig` (import cycle). Config also reads `.vars` files and environment variables, which a static release file must not use. gohcl is xcl's own decoder, so the constraint "read through xcl itself" holds.
- **Rejected**: new ParserOptions to turn off functions, variables and unknown-block errors (more surface, no gain); moving the registry under the root package (breaks the public package layout).

### Only native builds are chosen (discovery)
- **Decision**: the registry validates container entries but only ever chooses an executable package.
- **Rationale**: the spec's requirement "Container entries are validated but not yet run". How containers start is not designed.
- **Rejected**: choosing containers behind an option (no launch design exists).

### Chosen direction: keep the registry transaction, swap the release files (architecture)
- **Decision**: a new internal reader for the index and manifest in `registry`, the existing install transaction rewritten around it, the fake release server rewritten, and template tooling copied to both plugins.
- **Rationale**: fewest moving parts. Neither the parser nor Config changes, and the public API and errors stay. It is the design's install sequence step for step.
- **Rejected**: Config-based decoding (import cycle), new parser options, a dual old/new path (no backwards compatibility).

### Executable gains optional arguments (architecture)
- **Decision**: `registry.Executable(path string, args ...string)`, so the manifest's `command` arguments reach the process.
- **Rationale**: the design allows arguments in `command`, and variadic arguments let a bare path stay the simplest call.
- **Rejected**: refusing manifests with arguments (would contradict the design).

### Conventions selected (architecture)
- **Decision**: kept config-files-are-xcl, testing & mocking, shared test helpers, shared errors, testing provider Changed/Update, never modify dependencies, release plan. Dropped graph-ordering, the parser options builder (no parser change), test state from a real apply (no state involved), and find-references (no reference extraction).
- **Rationale**: only those bear on the registry, the fake server, the plugin providers and the release.

### Which sentinel each failure wraps (data_structures)
- **Decision**: ErrPluginNotFound for no index, no build for this platform, and container-only releases. ErrPluginVerification for signature, incomplete index, expression, checksum, manifest and protocol failures.
- **Rationale**: matches today's split (a missing archive is not-found; a missing checksums file or a mismatch is verification), so no new public error types are needed.
- **Rejected**: a new sentinel per failure (more public surface, no consumer asking for it).

### Release versions (tasks)
- **Decision**: xcl releases with the `minor` label; the Docker plugin goes v0.3.0 → v0.4.0 and the test plugin v0.2.0 → v0.3.0; the template takes its next minor.
- **Rationale**: before 1.0 a breaking change is minor (release-chain knowledge), and the new layout breaks installs of old releases.
- **Rejected**: patch (it's a breaking change); 1.0 (not this spec's call).

### Template releases too (tasks)
- **Decision**: a human task publishes a template release in the new layout.
- **Rationale**: the template README and the website's plugin-template page pin template releases, and those pins must move to a new-layout release.
- **Rejected**: leaving the template unreleased (its docs would pin an old-layout release the new registry refuses).

### Changelog written by the implement workflow (tasks)
- **Decision**: no task edits CHANGELOG.md; the implement workflow's changelog step writes the entry.
- **Rationale**: CHANGELOG.md sections are per spec, which matches the workflow-owned changelog store.

### Run a plugin from source (revision after the walkthrough)
- **Decision**: the plan ends with milestone 4. `Local.RegisterExternalPlugin(command string, args ...string)`, a CoreService `ExternalPlugin` command message, and the Python SDK's `register_external_plugin` accepting a path or a list. No release task: the Python change is built against the local xcl checkout and merges once a later xcl release is pinned.
- **Rationale**: the user asked for the plan to "end with source" so they can hand-build a Python plugin and get a feel for the API, and chose running a plugin from a command over reordering the releases. The example Python plugin repo and its docs are follow-up specs in the epic.
- **Rejected**: a hand-written Python example in this plan (the user moved it to its own spec); a `cwd` option (the design rules it out); a new Go function beside `RegisterExternalPlugin` (one function with variadic arguments is simpler).

### A plugin started from a command is named after its last argument (revision)
- **Decision**: `executable.Name()` is the binary name with no arguments, and the last argument's base name without its extension otherwise.
- **Rationale**: the name only appears in load events and errors (internal/catalog/catalog.go:830-860), and `python3` for every Python plugin would be useless there. GitHub plugins are named by repository (registry/github_install.go:30), so a manifest command with arguments isn't affected.
- **Rejected**: naming by the program (every Python plugin is `python3`); a name parameter (more API for a log label).

### A plugin started from a command is named by its full command line (revision, 2026-11-01)
- **Decision**: `executable.Name()` is the binary name with no arguments, and the full command line (the program and its arguments joined by spaces) otherwise. `Executable` becomes private, since the local registry's command form replaces it and nothing outside the `registry` package calls it. The spec's naming criterion was amended to match.
- **Rationale**: the name is only a label in load events and errors (internal/catalog/catalog.go:846). The last-argument rule is a guess that breaks on commands such as `npx some-plugin --verbose`, while the full command line always says exactly what failed to start. A public `Executable` built a `registry.Plugin` that no public method accepted.
- **Rejected**: the last-argument rule above (superseded); keeping `Executable` public with a `Local.Register(Plugin)` method (two ways to do one thing, for third-party registries nobody has written); container plugins as commands (`docker run` doesn't fit go-plugin's host socket, pull-by-digest or stop semantics; they need their own registry and design).

### The Python SDK accepts a path or a list (revision)
- **Decision**: `register_external_plugin` takes a path (a one-word command) or a sequence.
- **Rationale**: simplicity for the developer, not compatibility: a compiled plugin is just a path, so making it a one-item list adds noise. Go's variadic form reads the same way.
- **Rejected**: list-only, as `python-plugins.md` sketches (a single binary would need wrapping in a list).

### Rename the handshake cookie to XCL_PLUGIN (revision)
- **Decision**: a milestone 1 task renames go-plugin's magic cookie key from `HCLCONFIG_PLUGIN` to `XCL_PLUGIN`, before the xcl release.
- **Rationale**: the user: "XCL_PLUGIN HCLCONFIG does not exist". Hand-written plugins will check this name, so it should be right before anyone writes one.
- **Rejected**: renaming later (every plugin built in between would need rebuilding again); accepting both names (go-plugin takes a single cookie key).

### Breaking changes are allowed (revision)
- **Decision**: the spec's constraint "the registry's public API doesn't change" is replaced by "breaking changes are allowed". Nothing is kept only for compatibility.
- **Rationale**: the user: "I don't care about breaking changes, we are not released".
- **Effect on the plan**: no task changes. Choices that had been justified by compatibility are kept on simplicity grounds: `Executable` and `RegisterExternalPlugin` take variadic arguments, the Python SDK takes a path or a list, and the two existing error sentinels are reused. The CoreService field is still retired by reserving it, because CI's `buf breaking` requires that. Unknown package kinds are still ignored, because that is the design's rule for releases made after this one.

## Rehydration cues

- Read the design: `spektacular design read --data '{"source":"design","path":"plugin-release-assets-v2.md"}'`.
- Read the spec: `spektacular spec file read 20261031180836-7529c085-plugin-release-contract-v2`.
- Re-read xcl:registry/github_install.go and github_verify.go in full before changing the install flow, and internal/testutil/github_release.go before rewriting the fake.
- Test helpers: `newInstallTestRelease`, `newInstallTestRegistry`, `hostEntryDirectory`, `requireNoStagingDirectories` (github_install_test.go:24-104); `newSignatureTestRegistry`; `newGitHubWidgetRelease`/`newGitHubWidgetRegistry`/`newGitHubConfig`/`cachedWidgetBinary` (config_github_registry_test.go:33-70).
- `spektacular repo list` for the repo roots: xcl-plugin-template, xcl-plugin-docker, xcl-plugin-test, xcl-python, xcl-website.
- The spike that decoded the index shape through Config: session scratchpad `indexprobe/` (not durable). Re-create it if needed.
