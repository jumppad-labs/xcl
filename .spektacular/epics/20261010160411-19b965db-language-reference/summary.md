---
created_date: "2026-10-11"
---

# Planning summary: 20261010160411-19b965db-language-reference

## Decisions
- **Plugin function clash timing** — language-surface. Clashes involving a plugin fail the first operation, when plugins load; application-function clashes fail `NewConfig`, as type clashes do today.
- **Registering xcl-python before it exists** — python-support. xcl-python is registered with Spektacular at `~/code/github.com/jumppad-labs/xcl-python` so its tasks can be stored; creating the GitHub repository stays a human task.
- **Home page links** — language-reference, using-xcl-with-go. Both changes stand: the reference adds "Learn the language" and a "Documentation" section after "Two examples" with the Language reference first, and the guides join it after the reference; the guides also rewrite the closing banner with Go and Python buttons. Whichever spec lands first creates the section.
- **xcl README Documentation order** — language-reference, using-xcl-with-go. Language reference, then "Using XCL with Go", then "Using XCL with Python".
- **Tabs check exemption** — language-reference, using-xcl-with-go. The per-language guides are exempt from the "no Go box without a Python tab" check, alongside the plugin authoring pages.
- **Release versions** — language-surface, python-support, language-reference, using-xcl-with-go. The latest tag is v0.2.1: language-surface ships as v0.3.0 and python-support as v0.4.0; the docs are verified against v0.4.0.
- **Home page hero** — language-reference. The hero badge shows the verified release (expected v0.4.0) and the tagline says XCL is available for Go and Python.
- **CHANGELOG entry for small xcl changes** — python-out-of-process-engine, python-release-hardening. Every spec that changes xcl adds one `## <spec name>` entry with a `**Breaking:**` list; release hardening adds one for the untrusted-key message (Breaking: None).
- **Strict mode and engine routes** — python-out-of-process-engine, python-release-hardening. `XCL_TEST_STRICT=1` fails a skip only for something the job is meant to provide: sources, fixture plugins and the plugin release everywhere; engines only as listed in `XCL_TEST_ENGINES` (`subprocess` everywhere, `inprocess` on the Linux libxcl job). Engine skips go through `tests/engines.py::skip_missing_engine`; route-marker skips stay skips.
- **Windows interrupt testing** — python-out-of-process-engine, python-release-hardening. Both use `CREATE_NEW_PROCESS_GROUP` plus `CTRL_BREAK_EVENT`; `xcl-core` discards Ctrl-C and Ctrl-Break and shares the host's process group, so the Python host drives cancellation.

## Order added for shared files
- Added: `20261011084948-c995ed25-using-xcl-with-go` now depends on `20261010160411-19b965db-language-reference`. Both change `xcl-website:src/config/site.ts`, `xcl-website:src/pages/state-masking.mdx`, `xcl-website:src/pages/index.mdx`, `xcl-website:main.go`, `xcl-website:main.py`, `xcl-website:database.postgres.main`, `xcl-website:app.web`, `xcl-website:src/components/Nav.astro`, `xcl:README.md`, `xcl:CHANGELOG.md`.
- Added: `20261011232013-da18c586-python-release-hardening` now depends on `20261011211634-a8895873-python-out-of-process-engine`. Both change `xcl:internal/testutil`, `xcl-python:.exe`, `xcl-python:sys.path`, `xcl-python:pyproject.toml`, `xcl-python:tests/conftest.py`, `xcl-python:tests/integration/conftest.py`, `xcl-python:tests/parity/conftest.py`, `xcl-python:tests/integration/test_load.py`, `xcl-python:tests/integration/test_sync.py`, `xcl-python:test_sync.py`, `xcl-python:.github/workflows/ci.yml`, `xcl-python:ci.yml`, `xcl-python:.github/scripts/musl-wheel.sh`, `xcl-python:README.md`, `xcl:CHANGELOG.md`.

## 20261011095738-fad05e41-language-surface
### Approach

Functions are registered on a registry, plugin functions are hosted by providers and reached over two new gRPC calls, and the language rules are implemented in XCL's forked parser (`internal/xcl`). Applications register plain Go funcs with `registry.Local.RegisterFunction(name, fn)`; `registry.Registry` gains `Functions()`; the catalog owns the function set and checks names. Plugin functions build on the existing `ResourceProvider.Functions()` with a reshaped typed `ProviderFunction`, a `plugins.NewFunction(goFunc)` helper, and `GetFunctions`/`CallFunction` RPCs added to `PluginService` (handshake stays at version 1; older plugins answer Unimplemented and offer no functions). The fork rejects `%{ }` template directives, collects named child blocks into `map[string]T` (errors for missing or duplicate names), turns every type-triggered gohcl panic into an error, and honours `optional` inside object and map values. Dependency extraction is rebuilt on `hclsyntax.Variables`, so references inside `for`, index and parenthesis expressions are found. `XCL_VAR_` is set through one shared parser-options builder.

### Milestones and tasks

- M1, configuration is read the way the language says (xcl): `XCL_VAR_` prefix; references in every expression kind, plus a variable default built from an entity; optional fields in object and map values; template directives rejected.
- M2, named blocks by name, no crashes (xcl): named child blocks into maps and lists; unsupported field types return errors; named block maps through apply, diff and state.
- M3, applications and plugins add functions (xcl): application functions; plugin providers offer functions; plugin functions called over gRPC.
- M4, ecosystem: xcl docs and CHANGELOG; release xcl (human); xcl-plugin-template example `note_filename` function; xcl-plugin-docker builds against the release; xcl-website claim corrections; xcl-vscode tokenize regression test.

### Tasks a person must do

- Release xcl with the language surface changes (expected `v0.3.0`).

### Out of scope

No `for_each` or dynamic blocks, no `local` block, no templating function, no removal of map or object attributes, no other expression changes, no docs pages beyond claim fixes, no API renames, no editor formatting, no plugin protocol v2, schema v1 or Python/Node functions, no function namespacing, no module boundary change. The provider `Init` `functions` parameter still receives nil.

### Drafting assumptions

- Plugin function clashes fail the first operation, when plugins load; application-function clashes fail `NewConfig` (decided by the user).
- Application functions are plain Go funcs; `ParserOptions.CustomFunctions` is removed; `InvalidFunctionError` for unconvertible funcs.
- Function signatures are JSON with cty type strings; supported types are string, bool, numeric kinds, and slices and string-keyed maps of those.
- Sensitive arguments reach plugin functions unmarked; the result is re-marked on the host.
- The optional name label applies to map fields and list fields whose element has zero or one label field; duplicate names are errors in maps and lists.
- Bad block field types are checked when a type is declared or a plugin loads (`InvalidTypeError`).
- The vscode grammar has no `%{` rule today, so its change is a regression test only. The site function card states 57 built-ins.

### Project-wide rules

- CHANGELOG.md: one `## <spec name>` entry, prose then a `**Breaking:**` list.
- `registry.Registry` gains `Functions() []registry.Function` (breaking for every implementation).
- `plugins.PluginHost` and `PluginEntityProvider` gain `GetFunctions` and `CallFunction`; `plugin.proto` only gains RPCs.
- `FunctionSignature` is JSON with cty JSON type strings.
- Named block maps are `map[string]T` with `xcl:"name,block"`.
- `FunctionNameClashError`, `InvalidFunctionError`, `InvalidTypeError` live in `errors/` and are re-exported from the root package.
- Fork edits keep the MPL header, add a modification notice and are logged in UPSTREAM.md.
- Plugin repos pin a released xcl, never a `replace` directive.

### Manual checks

- xcl.dev and the README contain no `HCL_VAR_` or "around sixty", and the function count and wording are right.
- In VS Code, `%{ … }` is not coloured as template syntax and `${ … }` is.
- The CHANGELOG `**Breaking:**` list matches the changed public surface.
- Following the template README from a fresh copy produces a working plugin function.
- Every claim on xcl.dev and in the README matches the latest release when tried.

## 20261011130709-aa3eb933-python-support
### Approach

Built on the binding `libxcl.md` design, limited to what this spec needs: a pure-Go `coreservice` holding all binding logic behind `Service.Call`; `proto/xcl/core/v1/core.proto` generated with buf; a nested cgo module `bindings/libxcl` with four C exports; and a Python SDK in three layers (cffi transport with a ctypes fallback, a typed client, the hand-written API). Python functions are application functions: coreservice builds each as a Go func from its declared `FunctionSignature` and offers it through its registry's `Functions()`, so clashes and redaction behave as for Go functions. One Python poll engine has a sync driver and an asyncio driver; cancellation goes through new context-taking operations. An xcl tag builds, signs and publishes eight libxcl archives and dispatches the xcl-python release, which verifies them, builds eight `py3-none-<platform>` wheels and publishes to PyPI with Trusted Publishing.

### Milestones and tasks

- M1, the xcl engine can be driven from another language: context-taking operations; untyped lookups (`FindEntitiesByType`, `FindOneEntity`); schema v1 reader and `registry.TypeFromSchema`; coreservice CreateConfig and lookups; coreservice operations through the poll loop plus cancel; host-language functions; libxcl shared library and CI.
- M2, a Python program can configure, apply and query: Go fixture plugins and the Go parity program; create the xcl-python repository (human); safe engine loading (packaging, transport, client, errors, version and ABI checks, fork guard); Python classes describe entity types; the sync API; Python functions; Go plugins from a path, a directory and the GitHub registry with a tamper test; parity with Go.
- M3, async without blocking: the asyncio API never blocks and can be cancelled.
- M4, every xcl release ships a matching PyPI package: xcl release workflow for 8 signed archives; signing key and dispatch token (human); xcl-python CI matrix and release workflow; PyPI project and Trusted Publishing (human); README, docstrings and PARITY.md; CHANGELOG entry; cut the first release with Python support (human).

### Tasks a person must do

- Create the GitHub repository jumppad-labs/xcl-python and connect the registered local directory to it.
- Add the release signing key and dispatch token to xcl.
- Set up the PyPI project and Trusted Publishing.
- Cut the first xcl release with Python support (expected `v0.4.0`).

### Out of scope

In-process Python providers (proto numbers reserved), out-of-process Python plugins and protocol v2, distributing Python plugins through the GitHub registry, Node, xcl.dev Python docs, `DescribeTypes` and codegen, the shared `types.proto` move, switching the plugin schema producer to v1, the xcl-core transport and Wasm, the sync `max_workers` option, Python `decode`/`EncodeEntity`, platforms beyond the eight.

### Drafting assumptions

- xcl-python is registered with Spektacular before its GitHub repository exists (decided by the user).
- Python functions are reflected Go funcs; cty `number` maps to `float64`, so integers beyond 2^53 lose precision.
- Schema v1 is extended with a named-block map form and a `label` property.
- The sync API runs tasks serially; cancellation raises `xcl.CancelledError`, and under asyncio `asyncio.CancelledError` is re-raised after the engine stops.
- Plugin entities are read as `xcl.Entity`; PARITY.md lives in xcl-python.
- Python CI builds libxcl from source; releases use only the signed archives. Musl archives carry an `_musl` suffix.
- Open at implementation: whether language-surface's conversion accepts `MakeFunc` funcs, whether a cancelled Apply wraps `context.Canceled`, and whether the PyPI name `xcl` is free (stop and ask if not).

### Project-wide rules

- CHANGELOG.md gets one entry with `**Breaking:** None.`
- The language-surface function contract is used unchanged.
- Schema v1's named-block map form and `label` are what later protocol v2 or Python-provider work reuses.
- The CoreService error-code table is append-only; proto field numbers are reserved for provider work.
- The root module stays `CGO_ENABLED=0`; cgo lives only in `bindings/libxcl`.
- `buf lint` and `buf breaking` run in CI for `proto/`.
- xcl gets its first release workflow; libxcl assets reuse the plugin asset, checksum and signature contract and key.
- The PyPI version equals the xcl tag without the `v`, checked on load.
- Root API additions are additive only.

### Manual checks

- Follow the README alone from `pip install xcl` to a working apply against a Go plugin.
- Review PARITY.md against the Go API of the released tag.
- Review the CHANGELOG entry: additions described, Breaking None.
- Review the PyPI project page: install, sync getting started, then the asyncio section.
- Watch the first tagged release end to end: 8 signed archives, 8 wheels plus an sdist, install checks green on every platform.

## 20261011211634-a8895873-python-out-of-process-engine
### Approach

The engine runs as its own program everywhere, per the revised `libxcl.md` design. `cmd/xcl-core` is a cgo-free program in the xcl root module that serves the unchanged `coreservice.Service` over stdin/stdout using length-prefixed `Request`/`Response` frames defined in `proto/xcl/core/v1/transport.proto`; `Release` is a reserved frame method, so no CoreService method or C ABI change is needed. The serve loop lives in the program (coreservice stays transport-free), runs each request on its own goroutine, discards Ctrl-C and Ctrl-Break, and on stdin EOF or SIGTERM releases every configuration through a new additive `Service.Close`. Because go-plugin hands `os.Stdin` to every plugin it starts, the program keeps the real stdio for frames and points `os.Stdin` at the null device and `os.Stdout` at stderr. Python gains a `SubprocessTransport` with the same interface as the in-process one: lazy start with a version/ABI handshake, one reader thread matching responses by id, waits in short slices (GIL released, Ctrl-C delivered on Windows), live-handle tracking, stop on last release or `atexit`, a Windows job object, `EngineExitedError` then a fresh engine, and fork detection. Selection looks only at the package contents and `XCL_TRANSPORT` (`inprocess` picks the shared library); no probing, no fallback. One release job builds all eight targets with `CGO_ENABLED=0` and no reachable C compiler; the libxcl toolchain matrix moves to a manual workflow.

### Milestones and tasks

- M1, the xcl engine runs as a standalone program (xcl): the frame messages join the protocol; the engine program serves the core service over stdin/stdout.
- M2, Python runs the engine as a bundled subprocess on every platform (xcl-python): the subprocess transport; the package runs the bundled program by default and the shared library on request (selection, packaging, `cffi` as an extra, version 0.5.0.dev0, route markers, lifecycle tests); CI tests the program everywhere and the shared library on Linux.
- M3, every release publishes the program and eight wheels with no C toolchain: tagging xcl releases the program for eight platforms (xcl); the Python release builds eight wheels around it and checks them on every platform (xcl-python); the xcl-python README; an install section on xcl.dev (xcl-website); xcl CHANGELOG and README; cut v0.5.0 (human).

### Tasks a person must do

- Cut the v0.5.0 release (push the tag that publishes the engine archives and the PyPI wheels).

### Out of scope

Releasing libxcl or shipping it in a wheel; the Node binding; the release-hardening work (tomllib on 3.10, `XCL_SRC`, the test plugin, the recorded xcl ref); moving back in process; platforms that forbid starting a process; Authenticode signing; the design's download-into-`~/.xcl/core` and build-from-sdist fallbacks (the design says no silent fallback); Python providers, protocol v2, Python plugins, code generation; any Python API, CoreService or C ABI change beyond two new exception classes; changing v0.4.0; new xcl.dev pages or navigation.

### Drafting assumptions

- The serve loop is in the `cmd/xcl-core` main package; Python uses one reader thread with per-call waiters and decodes `CreateConfig` responses to track handles.
- `Release` travels as a reserved frame method name, documented in `transport.proto`.
- Negative statuses cross the design's `uint32 status` field as two's complement and Python reads them as signed (the design could say `int32` instead; raise with the design's owner).
- `cffi` becomes the optional `inprocess` extra; ctypes stays the fallback.
- "No C compiler" in the release job is shown with failing compiler shims on PATH plus an assertion that `cc` fails.
- No `XCL_CORE_PATH` override; `XCL_TRANSPORT` is the only selection override and `XCL_LIBRARY_PATH` stays for the in-process route.
- xcl.dev had no Python install section, so the home page gains one with Go and Python tabs, and the badge moves to v0.5.0.
- `libxcl-release.yml` is replaced by `release.yml` plus a manual `libxcl-build.yml`.
- The one-second first `Config()` metric is an integration test on every CI platform; the README getting-started example stays a manual check (it needs Docker).
- Lifecycle tests go in a new `test_engine_process.py`; the version-mismatch test builds a second program stamped `v9.9.9`.
- `xcl-core` discards interrupts with `signal.Notify`, not `signal.Ignore` (believed, not yet verified, that an ignored console event on Windows kills the process; the Windows smoke test confirms).

### Project-wide rules

- CHANGELOG: one `## <spec name>` prose entry at the top of xcl's CHANGELOG.md, ending with `**Breaking:**` and `None.`
- Tests never read workflows, READMEs, the changelog or site pages; Go tests use testify `require`, no tables, positive and negative cases apart; pytest has no `parametrize` tables.
- Release assets are `xcl-core_<version>_<os>_<arch>[_musl].tar.gz` (`.zip` on Windows) with `xcl-core_<version>_checksums.txt(.sig)`, signed with the jumppad-labs key; `<version>` has no leading `v`.
- xcl's release workflow is `release.yml`; `libxcl-release.yml` is removed; `libxcl-build.yml` is manual only; secret names are unchanged.
- Transport selection is decided only by the installed package plus `XCL_TRANSPORT` (`inprocess`).
- The frame protocol reserves the method name `Release`, is covered by `buf lint` and `buf breaking`, and by ABI version 1.
- The wheel carries `xcl/_bin/xcl-core[.exe]`; `cffi` is an extra; the package version moves to the 0.5.0 line.
- Strict mode: every CI and release test step sets `XCL_TEST_STRICT=1`; a skip fails only for something the job is meant to provide.
- `XCL_TEST_ENGINES` lists the engines a job provides: `subprocess` on every job, plus `inprocess` only on the Linux glibc x86-64 job that builds libxcl.
- Every missing-engine skip goes through `tests/engines.py::skip_missing_engine(route, reason)`; route-marker skips (`subprocess_only` / `inprocess_only`) are never strict failures.
- Windows interrupts are tested with `CREATE_NEW_PROCESS_GROUP` and `CTRL_BREAK_EVENT`, the child mapping SIGBREAK to KeyboardInterrupt.
- `xcl-core` discards Ctrl-C and Ctrl-Break with `signal.Notify` and runs in the host's process group.
- The xcl work must be on the ref `xcl-ref.txt` names (today `f-libxcl`) before xcl-python's CI can go green.
- Cutting a release tag is a human task.

### Manual checks

- Run the README's getting-started example (Docker plugin) against `xcl-lang 0.5.0`.
- Compare the v0.5.0 release workflow's wall time with the v0.4.0 run (38172909823).
- After release, review the issue tracker for reports of orphaned engine processes.
- Review the xcl CHANGELOG entry for this spec.
- Review the xcl-python README as shown on PyPI: bundled subprocess, same API everywhere, Alpine listed.
- Review the xcl.dev home page install section: Python tab text, Alpine listed, badge at v0.5.0.
- Watch the v0.5.0 release end to end: eight `xcl-core` archives and a signed checksums file, no libxcl archive, no C compiler in the build log, eight wheels and an sdist on PyPI, every install check green.
- Run the manual libxcl build workflow once and confirm it still builds all eight libraries.

## 20261011232013-da18c586-python-release-hardening
### Approach

A new repository, jumppad-labs/xcl-plugin-test (registered with Spektacular at `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test`), is generated from xcl-plugin-template and published as a signed release, v0.1.0. It must be its own repository because the GitHub registry names every asset and the binary after the repository; extra assets on the xcl release or reusing the template's sample plugin were rejected. Its single block type, `test "item"`, is modelled on xcl's recording fixture (`log`, `value`, `input`, a sensitive `secret`, a computed `output`) and appends each operation to a JSON-lines log, so tests can read back a destroy. xcl-python's test harness stops being Docker-specific: Python 3.10 reads TOML through `tomli` (installed only below 3.11); one locator, `tests/xcl_source.py`, finds the xcl sources (`XCL_SRC`, else the sibling checkout); CI strict mode turns harness skips into failures, routing engine skips through the out-of-process plan's `skip_missing_engine`; CI passes `GITHUB_TOKEN`. Extra CI failures are fixed here: the `.exe` suffix for Windows fixtures, Windows interrupts by Ctrl-Break to a new process group, and a macOS stall test calibrated against an idle baseline. xcl's untrusted-key refusal also names the signing key's ID, keeping its existing wording.

### Milestones and tasks

- M1, the suite runs on every supported Python, platform and checkout layout (xcl-python): read the plugin pin on every supported Python; find the xcl sources through one locator; build and find fixture programs with the Windows executable suffix; make the interrupt and stall tests hold on every platform; run CI in strict mode with a GitHub token.
- M2, a signed test plugin is published: create the xcl-plugin-test repository from the template (human); build the test item plugin (xcl-plugin-test); publish the first signed release, tag v0.1.0 (human).
- M3, end-to-end and registry tests run without Docker: pin the test plugin release in plugin-agnostic release helpers; replace the Docker plugin tests; retarget the tamper tests (xcl-python); name the untrusted signing key in the registry refusal, with its CHANGELOG entry (xcl); assert the refusal names the key; describe the test setup in the README (xcl-python).

### Tasks a person must do

- Create jumppad-labs/xcl-plugin-test from the template, add the release-key secrets (`GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`), turn the registered local directory into the checkout and commit its footprint.
- Tag xcl-plugin-test v0.1.0 to publish the first signed release.

### Out of scope

The engine program, subprocess transport, musl wheels and the Windows x86-64 wheel (including the Windows parity access violation and the Alpine TLS load error), which belong to the out-of-process engine spec; Python plugins or a Python plugin SDK; reworking release workflows beyond the test plugin's own; any change to xcl-plugin-docker, which stays the documented example; switching `xcl-ref.txt` to `main` (a one-line follow-up once f-libxcl merges); moving xcl's own tests that mention xcl-plugin-docker.

### Drafting assumptions

- The tamper tests keep their gpg-generated untrusted key; if a runner lacks gpg, stop and ask (fallback: a committed public key, needs the user's agreement).
- The untrusted-key refusal gains the signer's key ID with a new Go test and a new Python test; existing tamper tests are not edited.
- Block type `test "item"` with `log`, `value`, `input`, `secret`, `output`; each log record is `{"operation","address"}`.
- `GITHUB_TOKEN` is passed to the CI test step because about 30 matrix jobs download the release.
- Milestone order is harness first, then the plugin release, then the Docker swap; the ref switch is a follow-up, not a task.
- Strict mode only affects harness-level skips; `skipif` platform markers and route markers are untouched.
- The stall test passes when ticks during the apply reach at least 50% of the idle baseline.
- The test plugin requires the latest xcl tag at implementation time (v0.4.0 today); the plugin protocol is unchanged since v0.3.0.
- If the out-of-process engine plan has already reshaped the interrupt, stall or fixture-lookup tests or the CI step, this plan's intent is applied to the new shape; if it no longer applies, stop and ask.

### Project-wide rules

- CHANGELOG: one `## 20261011232013-da18c586-python-release-hardening` prose entry at the top of xcl's CHANGELOG.md, ending with `**Breaking:**` and `None.`, checked by review, never by a test.
- Strict mode: with `XCL_TEST_STRICT=1`, missing xcl sources, fixture plugins or the plugin release fail every job; a missing engine fails only for engines the job provides.
- `XCL_TEST_ENGINES` and `tests/engines.py::skip_missing_engine` come from the out-of-process engine plan: `subprocess` everywhere, plus `inprocess` only on the Linux glibc x86-64 libxcl job; `strict_skip(reason, *, route=None)` hands engine skips to it; route-marker skips are never strict failures.
- Windows interrupts are tested with `CTRL_BREAK_EVENT` and `CREATE_NEW_PROCESS_GROUP`, the child mapping SIGBREAK to KeyboardInterrupt; `xcl-core` discards Ctrl-C/Ctrl-Break and runs in the host's process group, so the host drives cancellation.
- `XCL_SRC` is the only way tests locate the xcl sources, through `tests/xcl_source.py`.
- xcl-python CI builds the engine and fixtures from the single ref in `xcl-ref.txt`.
- `tests/plugins.toml` pins `jumppad-labs/xcl-plugin-test` v0.1.0 under a `[test]` table.
- Fixture binaries carry the platform's executable suffix, in the Makefile and in fixture lookups.
- xcl's untrusted-key refusal keeps "does not verify against any trusted key" and adds "signed by key <ID>, which is not trusted".
- Test plugins live in their own repos, generated from xcl-plugin-template and signed with the jumppad-labs release key.

### Manual checks

- The xcl-python CI matrix is fully green on the first run after both this plan and the out-of-process engine plan land.
- One run per platform shows zero Docker-gated skips in its skip summary.
- xcl-plugin-test v0.1.0 has six platform archives, a checksums file and a `.sig` that verifies against the jumppad-labs key.
- With xcl checked out away from xcl-python and `XCL_SRC` set, the suite passes; with it unset and no sibling checkout, tests skip with the `XCL_SRC` message.
- On a branch, changing only `xcl-ref.txt` switches every CI job's build source.
- Review the README Development section; Getting started still shows the Docker plugin.
- Review the xcl CHANGELOG entry for this spec.

## 20261010160411-19b965db-language-reference
### Approach

A hand-written `CodeTabs.astro` component in xcl-website takes one fenced block per language through `go` and `python` slots, so Expressive Code still highlights each. A head script sets `<html data-lang>` from `localStorage["xcl-lang"]` (default Go) before paint; CSS shows the matching panel in every box; clicks update the attribute and the stored value, with storage access guarded. The language reference is eight new pages under `/language/`: overview, entities, attributes and blocks, references, expressions, variables and outputs, modules, functions. Rejected: moving to Starlight (restructures every page with no redirects) and a third-party tabs package with one long page (choices not shared or remembered).

### Milestones and tasks

- M1, every code example speaks Go and Python (xcl-website): tabbed code boxes; Python tabs on the registries, GitHub registry and events pages; on the sensitive values, state masking, configuration text and diff pages; on the home page and the configuration-only example; plugin authoring pages say plugins are Go and tab the application code.
- M2, XCL has a language reference (xcl-website): overview, entities, and attributes and blocks; references and expressions; variables and outputs, and modules; functions (all 57 built-ins).
- M3, the reference is reachable and matches the release: nav and home page links, plus the hero badge and tagline (xcl-website); xcl README and changelog (xcl); try every example against the release and correct the pages (xcl-website), stopping if v0.4.0 and the `xcl` PyPI package aren't published.

### Tasks a person must do

None.

### Out of scope

Automated tests of examples or tabs; the highlighter; library fixes; prose rewrites of existing pages; the Go and Python guides; Python plugin authoring; Python forms of Go-only features (custom registry, custom masker, `EncodeEntity`, custom highlight renderers, in-process plugin registration); Python example programs in the xcl repo; restructuring or renaming pages; API renames; locals.

### Drafting assumptions

- The plugin authoring pages are plugin-template, examples/plugins, replacement and plugin-logging: plugin code stays Go-only, application boxes get Python tabs, and each states plugins are Go for now and usable from Go and Python.
- Where an application feature has no Python form, the Python tab says so and links the xcl-python parity list.
- The home page hero badge shows the verified release (expected v0.4.0) and the tagline says XCL is available for Go and Python (decided by the user at epic review).
- Where Go registers a plugin in process, the Python tab loads it as a separate program and says why.
- Tab behaviour is checked by hand; the site's type check and build are the automated gates.

### Project-wide rules

- Nav: a new "Language" dropdown between "Features" and "Examples", overview first.
- Home page: a "Learn the language" link in "What is XCL?" and a new "Documentation" section after "Two examples" with the Language reference first; the guides follow it there.
- xcl README: "Language reference" is the first item of the Documentation list.
- Slugs under `/language/` are chosen once (no redirects).
- `CodeTabs` with `go` and `python` slots, `<html data-lang>`, storage key `xcl-lang`; the per-language guides use no tabs.
- `site.ts` gains `pythonRepoURL`, `pythonParityURL` and `pypiURL`.
- CHANGELOG entry with `**Breaking:** None`.
- Type mapping lives in the reference; API walkthroughs live in the guides.

### Manual checks

- Every language topic removed from the README has a home on xcl.dev; locals map to defaults built from other values.
- Every acceptance area has its own heading anchor.
- No page outside the four plugin authoring pages and the two per-language guides has a Go box without a Python tab.
- Picking Python switches every box and carries to the next page; reload, keyboard switching and blocked storage work.
- The reference is in the nav, linked from the home page and the README, and every link opens it.
- The reference covers each acceptance area of the spec.
- Terminology review of the reference pages.
- The four plugin authoring pages carry the "plugins are Go for now" statement.
- The CHANGELOG entry reads as prose then `**Breaking:** None`.

## 20261011084948-c995ed25-using-xcl-with-go
### Approach

Two standalone MDX pages on xcl.dev built from the site's existing parts (Hero, Prose, CtaBanner): `/using-xcl-with-go/` and `/using-xcl-with-python/`. Each grows one program section by section and ends with the full listing, using the same configuration (a variable, `database "postgres" "main"`, `app "web"` calling a `slugify` application function, an output). The Go guide uses registered types and no plugin: function-form lookups, `ApplyContext` with Ctrl-C handling, `Find`/`FindByType`/`FindOne`/`All`, `Entities` and `FindEntitiesByType`, outputs, `ErrNotFound` and `NotUniqueError`, `Load` in a later run, and `Destroy`. The Python guide uses `@xcl.entity` dataclasses, `LocalRegistry` with `register_type` and `register_function`, the Go Docker plugin through `GitHubRegistry`, the sync `xcl.Config`, then `xcl.aio.Config` with when to use it, cancellation and fork safety. Python's generated API docs are added with pdoc, published to xcl-python's GitHub Pages.

### Milestones and tasks

- M1, the Go guide: write the guide (xcl-website); nav and home page (xcl-website); link from the xcl README (xcl).
- M2, Python API docs: generate and publish with pdoc, a docs workflow and README links (xcl-python); turn on GitHub Pages (human).
- M3, the Python guide: write the guide plus `pythonReferenceURL` in `site.ts` (xcl-website); nav and home page (xcl-website); link from the xcl README and record the guides in the changelog (xcl).
- All guide work waits on python-support's release (expected v0.4.0), recorded as a gate in the context document.

### Tasks a person must do

- Turn on GitHub Pages for xcl-python and publish the API docs.

### Out of scope

Automated tests and link checkers; xcl-vscode; library fixes; changes to existing topic pages beyond nav, home and links; tabs, the type mapping and Python on other pages (the reference spec); a plugin in the Go guide; Python plugins or providers; Node; changes to the python-support API or PARITY.md; a custom domain for the API docs; redirects.

### Drafting assumptions

- Python's generated API docs are pdoc output at `https://jumppad-labs.github.io/xcl-python/`.
- The Python guide uses xcl-plugin-docker through `GitHubRegistry`, so Docker is a prerequisite.
- The asyncio example uses only the standard library; web frameworks are named in prose.
- The home page's closing banner becomes "Start building with XCL" with Go (primary) and Python (secondary) buttons.
- The Go guide imports `xclerrors` for `ConfigError`, which the root package doesn't re-export.

### Project-wide rules

- Nav: both guides are the first two children of the `Guides` dropdown, so its label opens the Go guide.
- Home page: the closing CtaBanner points to both guides, and both guides are listed in the "Documentation" section right after the Language reference.
- xcl README: Language reference, then "Using XCL with Go", then "Using XCL with Python".
- Website README: a page-table row for each guide.
- The per-language guides have no Go|Python tabs and are exempt from the reference's tabs check.
- Guides link to the language reference for type mapping and don't repeat it.
- CHANGELOG entry with `**Breaking:** None`.
- `site.ts` gains `pythonReferenceURL` beside `goReferenceURL`.
- Go examples use function-form lookups.

### Manual checks

- Every Go API topic removed from the README has a home on xcl.dev.
- Getting-started questions can be answered with a link to the matching guide.
- Go run-through in an empty directory against the release tag; output matches the guide.
- Python run-through in a fresh virtualenv with Docker, including the asyncio apply.
- The nav, the home page (banner and Documentation section) and the xcl README link both guides in the agreed order.
- Each guide's link-outs resolve, including its language's API docs.
- Features marked not yet supported are really unsupported in the release.
- Terminology review; each guide shows only its own language; CHANGELOG entry reads prose then Breaking None.
