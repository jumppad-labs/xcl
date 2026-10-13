---
created_date: "2026-10-12"
document_status: final
closed_date: "2026-10-12"
---

# Plan: 20261011232013-da18c586-python-release-hardening

<!-- Metadata -->
<!-- Created: 2026-10-12T08:55:40+01:00 -->
<!-- Commit: e59ce2f -->
<!-- Branch: f-libxcl -->
<!-- Repository: jumppad-labs/xcl (plan store); work in xcl-python, xcl-plugin-test, xcl -->

## Overview

This plan makes the xcl-python test suite release-grade. The suite will run on every supported CPython from 3.10 up, on Linux, macOS and Windows, and from any checkout layout. A purpose-built, signed test plugin (jumppad-labs/xcl-plugin-test) replaces the Docker example plugin in the end-to-end and GitHub-registry tests, so every Python release is verified the same way on every platform without a Docker engine. It is for the people cutting XCL releases, who need the CI matrix to be a trustworthy gate.

## Conventions

- **One CHANGELOG entry per spec in xcl's CHANGELOG.md** (cross-plan rule settled for this epic). The xcl task in this plan adds a `## 20261011232013-da18c586-python-release-hardening` entry at the top: prose describing the change, ending with `**Breaking:**` and `None.`. No test reads the CHANGELOG; it is checked by review.
- **Testing & Mocking: testify `require`, Mockery doubles, NEVER table-driven tests, NEVER mix positive and negative cases, tests live beside the code they test, NEVER tests that inspect repository files** — applies to the new Go plugin's unit and e2e tests (provider tests use the Mockery double of the log client), and in spirit to the new and changed Python tests: one behaviour per test function, no test asserting on CI YAML, README or plugins.toml text.
- **Testing provider Changed and Update** — the test plugin's provider `Update`/`Changed` tests build the real `[]entity.PropertyChange` xcl would pass and assert the call made on the mocked log client.
- **Assert ordering on graph parents, not provider call order** — the recorded-operations assertions compare order only between `test "item"` blocks linked through `input`; unlinked items are asserted as a set.
- **Generate test state with a real apply** — the sensitive-masking-in-state test reads the state written by a real apply under `state_mask`, never a hand-written state file.
- **Dependencies: prefer the standard library, document third-party reasoning, pin versions** — `tomli` is added only below Python 3.11 with a comment saying why, and the plugin pins its xcl module version in go.mod.
- **Go Code Style & Standards** — gofmt/go vet/staticcheck via the template's lint target, descriptive names, explicit error handling in the plugin.
- **Development Standards: structured logging** — the provider logs through the `logger.Logger` the plugin is given, as the template does.
- **Shared test helpers live in `internal/testutil`** — the new accessor for the fake release's other signing key goes in xcl's `internal/testutil` beside the existing fake GitHub release, not in a registry `_test.go` copy.

Deliberately not applied: the config-parser-options builder, hclsyntax reference finding, the shared errors package and the database conventions concern xcl internals this plan does not touch. The registry change reuses the existing `ErrPluginVerification` wrapping as it is. The Go project-structure convention (`/cmd`, `/internal`, `/pkg`) is superseded for the plugin by the plugin template's standard layout, which the spec mandates.

## Architecture & Design Decisions

The work lands in three registered repos. A new Go plugin, **xcl-plugin-test** (`/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test`), is generated from xcl-plugin-template and published as a signed GitHub release. **xcl-python** (`/home/nicj/code/github.com/jumppad-labs/xcl-python`) swaps the Docker example plugin for that release in its end-to-end and GitHub-registry tests and fixes the test-harness faults that stop its CI matrix from going green. **xcl** gets one small change. The asset contract the plugin must satisfy (`registry/github_assets.go`) is used as it is. The refusal of a release signed by an untrusted key (`registry/github_verify.go:98-103`) currently names only the signature file. It will also name the key that made the signature, its long key ID, so the acceptance criterion "refused naming the key" holds. The existing "does not verify against any trusted key" wording stays, so the existing tamper tests keep passing unchanged. The test plugin has to be a repository of its own because the GitHub registry names every asset and the binary after the repository. That rules out shipping it as extra assets of the xcl release, and it rules out reusing the template's sample plugin, because test-only behaviour would then reach every plugin made from the template. See `research.md#alternatives-considered-and-rejected`.

**Test plugin shape.** The plugin keeps the template's layout: `entities/`, `providers/`, a `client/` layer behind an interface with a Mockery double, `cmd/xcl-plugin-test`, `e2e/`, the dist build, and `release.yml` signing the checksums with the jumppad-labs release key. It registers one block type, `test "item" "<name>"`, modelled on xcl's recording fixture (`bindings/libxcl/testdata/plugins/recording/main.go`). The block has these fields:
- `log`: required. The path of a JSON-lines file. Every lifecycle call (create, read, update, destroy) appends `{"operation","address"}` to it before doing its work.
- `value`: updated in place.
- `input`: set from another item's output to make a dependency.
- `secret`: a `types.Sensitive[string]`.
- `output`: computed by create and update as `<name>-<value>`.

Recording to a file rather than into state means a destroy can still be read back after the entity has gone. It also works the same whether the plugin runs in-process or as a separate program. Python tests that assert an order use two items linked through `input`, so the order they check is fixed by the graph rather than by luck (convention: assert ordering on graph parents). The release is cut from tag `v0.1.0` by a person, because the signing secrets and the GitHub repository are human-owned.

**xcl-python harness.** Shared test helpers move from Docker-specific to plugin-agnostic:
- `docker_helpers.py` becomes `release_helpers.py`. It reads the pin `[test]` from `tests/plugins.toml` through `tomllib`, falling back to `tomli` on 3.10. `tomli` is installed only under the marker `python_version < "3.11"`, so no third-party dependency is added where a standard reader exists.
- The Docker daemon probe, which failed with `AF_UNIX` on Windows, is deleted.
- `test_docker_plugin.py` becomes `test_test_plugin.py`. It keeps the four start-path and generic-entity tests, retargeted at `configs/test_plugin/main.xcl`. It adds tests for the recorded operation order after apply and destroy, for the computed `output`, and for `secret` being redacted in events and masked in state under `state_mask`.
- `test_github_tamper.py` keeps its four tests and `FakeGitHub` as they are and only installs the new release. Its untrusted key is still generated with gpg, which every CI runner image provides (on Windows through the Git for Windows tools on the bash `PATH`).
- The location of the xcl sources is resolved once, in `tests/xcl_source.py`: `XCL_SRC` if set, otherwise the sibling `../xcl`. Tests in `unit/`, `integration/` and `parity/` get it from a fixture that skips with "set XCL_SRC to the xcl checkout". That fixes `tests/unit/test_schema.py` and the integration conftest, which currently ignores the variable.
- In CI, `XCL_TEST_STRICT=1` turns a harness skip into a failure only for something the CI job is meant to provide. The xcl sources, the fixture plugins and the plugin release are always meant to be there, and those skips fail under strict mode on every job. Engines follow the sibling out-of-process-engine plan's signal, which that plan introduces along with the CI and release steps that set it:
  - `XCL_TEST_ENGINES` is `subprocess` on every job, plus `inprocess` only on the Linux glibc x86-64 job that builds libxcl.
  - A missing-engine skip goes through the sibling's `tests/engines.py::skip_missing_engine(route, reason)`, whose `expected_routes()` reads `XCL_TEST_ENGINES`. It fails only for a route the job is meant to provide.
  - Route-marker skips (`subprocess_only` / `inprocess_only`) are never strict failures.

This keeps the "zero skipped e2e/registry tests" metric honest without failing jobs for routes they deliberately don't run.

**Platform fixes in scope.** The fixture plugins and the parity program are built with `.exe` on Windows, both in the Makefile and in the fixture paths. The keyboard-interrupt test reaches its child through `CREATE_NEW_PROCESS_GROUP` and `CTRL_BREAK_EVENT` on Windows; the child maps `SIGBREAK` to `KeyboardInterrupt`. Both plans in the epic test interrupts on Windows this way. The sibling's `xcl-core` engine program discards Ctrl-C and Ctrl-Break through `signal.Notify` (not `signal.Ignore`) and runs in the host's process group, so the Python host's `KeyboardInterrupt` handling drives cancellation. The sibling plan points its Windows interrupt coverage at this test. The thread-stall test compares its tick count against a baseline measured on the same runner rather than a fixed 50.

**CI source.** `xcl-ref.txt` stays the single recorded xcl ref that both CI jobs read. The Test step also passes `GITHUB_TOKEN`, so 30 matrix jobs downloading the release stay inside GitHub's rate limit. Switching the ref to `main` is a one-line change, made once f-libxcl has merged.

**Left to the sibling spec.** The in-process DLL crash on Windows and the musl load failure are left to the sibling out-of-process-engine spec. This plan doesn't touch the transport.

**Naming the untrusted key.** When the detached signature doesn't verify because its issuer is not among the trusted keys, the engine reads the issuer key ID from the signature packet. It adds that ID to the verification error, which keeps its `ErrPluginVerification` wrapping and its existing text. This is the only engine change. A new Python registry test, added beside the unchanged tamper tests, asserts that the refusal names the jumppad-labs release key's ID when another key is trusted.

## Component Breakdown

- **Test plugin entity (xcl-plugin-test, new)** — owns the `test "item"` block type's schema: the log path, the in-place `value`, the dependency-making `input`, the sensitive `secret` and the computed `output`. Holds no behaviour, so a Go application can read applied items with this package alone, as the template prescribes.
- **Operation log client (xcl-plugin-test, new)** — the backend layer behind a small interface that appends one JSON record per operation to a file and reads records back. The provider calls it; its Mockery double stands in for it in provider unit tests.
- **Test item provider (xcl-plugin-test, new)** — implements create, read, update and destroy for `test "item"`: records each operation through the log client before acting, and fills in `output`. It is registered by the plugin component and is the only code that touches the log.
- **Plugin and plugin program (xcl-plugin-test, new)** — registers the block type with its provider for in-process use and serves the same plugin as the `xcl-plugin-test` program over gRPC for external use.
- **Plugin build, CI and signed release (xcl-plugin-test, new, from template)** — builds the per-platform archives, checksums file and detached signature in the asset layout the GitHub registry verifies; CI builds, lints and tests every change; the release workflow signs with the jumppad-labs release key on a version tag.
- **Pinned test-plugin release (xcl-python, changed)** — the single record of which test plugin repository and version the Python suite installs, replacing the Docker pin.
- **Release helpers (xcl-python, changed from the Docker helpers)** — read the pin (with the TOML fallback on 3.10), work out the release's cache entry and asset names for this host, build the GitHub registry, install the release once per session and expose the trusted key. Used by the external-plugin tests and the tamper tests; no Docker probing remains.
- **External-plugin tests (xcl-python, changed from the Docker plugin tests)** — start the test plugin through the GitHub registry, by path and by directory, read its entities generically, and assert the recorded operation order, the computed value and the sensitive masking in events and state.
- **GitHub tamper tests (xcl-python, unchanged tests, retargeted)** — the fake GitHub API and its four refusal/acceptance tests, now serving the test plugin's real release files; their gpg-generated untrusted key is kept as is.
- **Registry signature refusal (xcl, changed)** — the GitHub registry's signature check, which, on a signature made by a key that is not trusted, now names that key's ID in the verification error alongside the existing wording. Consumed by every binding through the engine's error.
- **xcl source locator (xcl-python, new)** — resolves where the xcl sources are (the given location, else the sibling checkout) and gives every test that needs them a fixture that skips with a message naming the variable to set, or fails when CI runs in strict mode. Consumed by the unit schema test, the integration fixtures and the parity fixtures.
- **Fixture build (xcl-python, changed)** — builds the fixture plugins and the parity program with the platform's executable suffix; the fixtures that locate them use the same suffix.
- **Platform-sensitive integration tests (xcl-python, changed)** — the keyboard-interrupt test delivers its interrupt in the way each platform supports, and the thread-stall test judges progress against a baseline measured on the same machine.
- **CI workflow (xcl-python, changed)** — builds the engine and fixtures from the one recorded xcl ref, runs the suite with a GitHub token and strict mode, and no longer mentions or depends on Docker for the main matrix.
- **Developer documentation (xcl-python, changed)** — the README's Development section describes the xcl source location variable and the test plugin; the Getting started example keeps the Docker plugin as the documented example.

## Data Structures & Interfaces

**`Item` (xcl-plugin-test entities)** — the `test "item" "<name>"` block type, the contract between configuration, the provider and anything that reads applied items (Python reads it generically as `xcl.Entity`).

```go
type Item struct {
    types.ResourceBase `xcl:",remain"`
    Log    string                  `xcl:"log"`                       // JSON-lines file every operation appends to
    Value  string                  `xcl:"value,optional"`            // changed in place (update)
    Input  string                  `xcl:"input,optional"`            // usually another item's output, making a dependency
    Secret types.Sensitive[string] `xcl:"secret,optional"`           // masked in events and state
    Output string                  `xcl:"output,optional,computed"`  // "<name>-<value>", set by create and update
}
```

**`Operation` record (xcl-plugin-test client, serialised as one JSON line)** — the read-back contract between the plugin process and the tests, in Go and in Python. Lines are appended in the order the provider starts each call.

```json
{"operation": "create|read|update|destroy", "address": "test.item.<name>"}
```

**`OperationLog` interface (xcl-plugin-test client)** — the backend seam the provider depends on, with a file-backed implementation and a Mockery double.

```go
type OperationLog interface {
    Append(ctx context.Context, path string, record Operation) error
    Read(ctx context.Context, path string) ([]Operation, error) // used by the plugin's own e2e tests
}
```

**Release asset contract (unchanged, from xcl's GitHub registry)** — `xcl-plugin-test_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) with `xcl-plugin-test[.exe]` at its root, `xcl-plugin-test_<version>_checksums.txt` and `xcl-plugin-test_<version>_checksums.txt.sig` signed by the jumppad-labs release key.

**`tests/plugins.toml` pin (xcl-python)** — the table name changes from `docker` to `test`:

```toml
[test]
repository = "jumppad-labs/xcl-plugin-test"
version = "v0.1.0"
```

**xcl source locator (xcl-python tests)** — `xcl_source.path() -> Path` (the `XCL_SRC` environment variable, else the sibling `../xcl`, resolved) and a session fixture `xcl_src` returning it or skipping with "set XCL_SRC to the xcl checkout" (failing instead when `XCL_TEST_STRICT=1`). A `strict_skip(reason, *, route=None)` helper is the single skip entry point for this plan's fixtures:
- With no `route` (sources, fixture plugins, plugin release), it calls `pytest.fail` under `XCL_TEST_STRICT=1` and `pytest.skip` otherwise.
- With a `route` (`"subprocess"` or `"inprocess"`), it delegates to the sibling's `tests/engines.py::skip_missing_engine(route, reason)`, which fails only when `expected_routes()` (from `XCL_TEST_ENGINES`) lists that route.

This plan defines no engine reader of its own.

**Untrusted-key refusal message (xcl registry, changed)** — still wraps `ErrPluginVerification` and keeps its existing text, with the signing key appended:

```
plugin verification failed: signature <name>_<v>_checksums.txt.sig does not verify against any trusted key: signed by key <16-hex-digit key ID>, which is not trusted
```

It reaches Python unchanged as `xcl.PluginVerificationError`'s message.

**Environment variables (xcl-python test harness)** — `XCL_SRC` (existing, now honoured everywhere), `XCL_TEST_STRICT` and `XCL_TEST_ENGINES` (both introduced by the sibling plan; `XCL_TEST_ENGINES` is `subprocess`, or `subprocess,inprocess` on the Linux glibc x86-64 job; this plan extends strict mode to sources, fixture plugins and the plugin release), `GITHUB_TOKEN` (existing engine behaviour, now supplied by CI).

No other public types or Python package APIs change, and the xcl-python `xcl` package itself is untouched.

## Implementation Detail

**A plugin made the way plugin authors make one.** The test plugin follows the template exactly: entities in a behaviour-free package, a provider per type built by a constructor that takes its client, a client interface with a Mockery double, a plugin type that registers in-process, and a thin program that serves it. A developer opening it should see the template with the notes example swapped for a test item. Nothing new is invented here. The only departure from the template is that the "backend" is an append-only operation log rather than a files client. That keeps the plugin free of any external service, so it runs the same on every CI platform.

**Recording is a side channel, not state.** The provider writes the operation record before it does the work, as the existing recording fixture does. A test therefore sees every call that started, including one that later fails, and the record survives destroy. The log is plain JSON lines, so Python reads it with the standard library and the Go e2e tests read it through the client. Ordering claims are made only between items linked by `input`. That follows the project's rule that concurrent, unlinked vertices have no defined call order.

**The Python harness is plugin-agnostic.** Today the Docker plugin is baked into the helper names, constants and skip logic. After this change the helpers describe "the pinned external plugin release": its pin, its cache entry, its registry and its keys. The tests that use them talk about a test item. The tamper tests are untouched apart from their imports and the configuration they validate. Their gpg-generated untrusted key is kept, and the one new assertion, that the refusal names the signing key, lives in a test of its own. That keeps the spec's promise that they are "retained unchanged apart from the plugin they install".

**One way to find xcl, one way to skip.** Three copies of "where is the xcl checkout" collapse into a single locator module next to `fakes.py`, which is already importable from every test directory. Every harness-level skip goes through one helper:
- Locally it skips, with a message naming what to set.
- Under the CI strict flag it fails for sources, fixture plugins and the plugin release on every job. For a missing engine it defers to the sibling's `skip_missing_engine`, which fails only for a route the job's `XCL_TEST_ENGINES` lists. A misconfigured runner therefore cannot pass by skipping, and route-marker skips for a route the job doesn't run still skip.

Legitimate platform skips stay ordinary `skipif` markers and are not affected. Examples are `fork` on Windows and an architecture with no published plugin build.

**Platform differences are handled where they arise, not by skipping.** Executable names gain the platform suffix in the build and in the fixtures that look binaries up. The interrupt test chooses its delivery mechanism by platform, and the child it starts maps the Windows break signal onto `KeyboardInterrupt`, so the behaviour under test is the same. The stall test calibrates against an idle baseline on the same runner, rather than a fixed tick count that slow macOS runners miss.

**Release coupling stays loose.** The Python suite pins one test-plugin version. Bumping it is a one-line change to the pin, separate from the xcl ref the engine is built from. The xcl ref remains the one recorded source the CI build reads.

## Dependencies

- **Spec 20261011130709-aa3eb933-python-support (must have landed)** — provides the xcl-python package, its test suite, `xcl-ref.txt` and the CI workflow this plan repairs. Landed (xcl-python main at `26e578a`).
- **Spec 20261011211634-a8895873-python-out-of-process-engine (sibling, planned in parallel)** — owns the engine program and subprocess transport. That covers the Windows parity access violation and the Alpine musl load failure, so this plan does not fix them. Both plans edit xcl-python's CI workflow, Makefile and the integration and parity conftests, and the epic orchestrator orders the two. The CI matrix can only be fully green once both have landed.
- **xcl-plugin-template (unchanged)** — the source the test plugin is generated from: layout, Makefile `dist` target, CI and signed `release.yml`.
- **xcl Go module, latest tag at implementation time (v0.4.0 today; unchanged)** — the plugin SDK the test plugin is built against (`plugins`, `types`, `entity`, `logger`). The plugin protocol has not changed since v0.3.0, so the plugin also runs under the engine built from f-libxcl.
- **xcl GitHub registry and verification (changed: the untrusted-key refusal names the key)** — the asset naming, checksum and detached-signature contract the release must satisfy, unchanged. Only the untrusted-key error text gains the signing key's ID. The change lands on xcl's `f-libxcl` line, which xcl-python CI builds from. The engine also reads `GITHUB_TOKEN`.
- **jumppad-labs release signing key (human-held)** — the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` secrets the test plugin's release workflow signs with. Its public half is already committed in xcl-python as `tests/keys/jumppad-labs.asc`.
- **GitHub repository jumppad-labs/xcl-plugin-test (must be created by a person before the plugin milestone)** — registered with Spektacular at `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test`. That directory holds only the Spektacular footprint until a person makes it the repository's checkout.
- **`tomli` (new, test-only, Python < 3.11 only)** — the TOML reader with the `tomllib` API, for CPython 3.10, which has no standard reader. It is not installed on 3.11 and later.
- **pytest ≥ 8 (existing)** — the test runner. No other new Python dependencies.
- **xcl-plugin-docker (unchanged)** — stays the documented example plugin, with its own repository and tests. xcl-python's README keeps using it, and only its tests stop depending on it.
- **f-libxcl merging into xcl main (later, human)** — the trigger for switching `xcl-ref.txt` from `f-libxcl` to `main`.
- Design documents this plan was built on: none. The spec carries no design references.

## Testing Approach

**Test plugin (Go, xcl-plugin-test).** It follows the template's own test layers:
- Provider unit tests use a Mockery double of the operation log. Create, read, update and destroy each record the right operation and address, and create and update compute `output`.
- `Changed` and `Update` are tested with the real property changes xcl would pass.
- The file-backed log client gets unit tests against a temp directory: append then read keeps order, and a missing file reads as empty.
- End-to-end tests apply, re-plan and destroy the sample in-process and as a separate program, and read the log back. The template's state-only e2e is kept.

Each test covers one behaviour, positive and negative cases sit in separate functions, testify `require` is used throughout, and there are no table-driven tests. The release workflow runs the full test suite before it signs anything.

**Python integration tests (xcl-python), the load-bearing layer.** The test plugin's real signed release is installed from GitHub once per session. The tests then guarantee that:
- (a) the plugin starts through the GitHub registry, by path and by directory, and its entities read back generically with meta, references and links;
- (b) after an apply and a destroy of two linked items, the recorded operations come back in dependency order (create parent then child; destroy child then parent);
- (c) the computed `output` is filled in;
- (d) `secret` is redacted in lookups and in processed event data, and absent in plain text from state written under an AES-GCM `state_mask` by a real apply;
- (e) the existing tamper tests still pass unchanged. A valid release served by the fake API installs. A tampered checksums file is refused with a signature error and not cached. A release checked against an untrusted key is refused with "does not verify against any trusted key". A release with no keys trusted installs unsigned.

**xcl registry (Go).** A new unit test beside the existing signature tests: a release signed by a key other than the trusted one is refused with a verification error that names the signing key's ID. It sits in its own function, separate from the existing refusal test, which is unchanged.

A new Python registry test sits beside the unchanged tamper tests. When the fake API serves the real release and a different key is trusted, the refusal names the jumppad-labs release key's ID.

**Python harness tests.** Unit tests cover the xcl source locator in three cases:
- `XCL_SRC` honoured when set;
- the sibling checkout used when it is not set;
- a skip whose message names `XCL_SRC` when neither exists, and a failure instead under strict mode.

They also cover the TOML pin reader on the running interpreter. The schema-parity unit test now gets its fixture through the locator.

The interrupt test and the stall test keep their assertions about behaviour: the interrupt ends the apply promptly with state saved, and other threads keep running during an apply. What changes is how the interrupt is delivered on each platform and how the stall test's threshold is calibrated.

**Regression coverage is CI itself.** The full matrix (Linux, macOS, Windows; CPython 3.10 to latest) runs with strict mode on. Any fixture that cannot find the fixture plugins, the xcl sources, the plugin release or an engine route the job lists in `XCL_TEST_ENGINES` fails instead of skipping. Only route-marker skips (such as `inprocess_only` on jobs that don't build libxcl) still skip.

**Success metrics.**
- *"The xcl-python CI matrix is fully green on the first run after this lands, on every platform and Python version"* — **Manual — captured in the implementation test plan**. It is observed on the first CI run after both this plan and the sibling engine plan have landed.
- *"The number of Docker-gated skips in xcl-python's CI is zero on every platform"* — **Behavioural test**. No Docker gate remains in the suite, and strict mode makes any remaining harness skip in the e2e and registry tests a CI failure, so a green run proves zero such skips. Also **Manual — captured in the implementation test plan**: read the skip summary of one run per platform to confirm.

**Acceptance criteria mapped.**
- Green on 3.10: CI matrix, behavioural.
- Fixtures found through the given location: locator unit tests, plus the manual non-sibling checkout check below.
- Suite passes without Docker: CI runners use no Docker for the main matrix, and strict mode is on.
- Registry path exercised: (a) and (e) above.
- Recorded operations: (b), (c) and (d) above.
- CI source switch: manual, below. The switch to `main` itself is a follow-up once f-libxcl merges, not part of this plan.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: the first jumppad-labs/xcl-plugin-test release (`v0.1.0`) is published with six platform archives, the checksums file and a `.sig` that verifies against `tests/keys/jumppad-labs.asc`.
- **Manual — captured in the implementation test plan**: with xcl checked out somewhere other than next to xcl-python and `XCL_SRC` pointing at it, the full suite passes. With `XCL_SRC` unset and no sibling checkout, the tests that need the sources skip with a message naming `XCL_SRC`.
- **Manual — captured in the implementation test plan**: on a branch, changing only `xcl-ref.txt` (for example to the `v0.4.0` tag, or to `main` once f-libxcl has merged) makes every CI job check out and build the engine and fixtures from that ref, with no other edit.
- **Manual — captured in the implementation test plan**: reviewing the README Development section, which describes `XCL_SRC` and the test plugin while the Getting started example still uses the Docker plugin.

- **Manual — captured in the implementation test plan**: review the xcl CHANGELOG entry for this spec. It should describe the untrusted-key refusal naming the signing key's long ID and end with `**Breaking:**` / `None.`.

**Deliberate gaps.**
- No test inspects CI YAML, README, CHANGELOG or `plugins.toml` text (project convention). Those are checked by review.
- The xcl engine gets only one new unit test, for the key-naming refusal.
- No tests for the Windows DLL crash or musl loading, which belong to the sibling spec.

## Milestones & Tasks

### Milestone 1: The Python test suite runs on every supported Python, platform and checkout layout

**What changes**: Today the suite stops at collection on CPython 3.10 and fails a schema test wherever xcl isn't checked out next to xcl-python. On Windows it can't start its fixture plugins and can't interrupt an apply, and its thread-stall check fails on slow macOS runners. After this milestone, these all work:
- The suite imports cleanly on 3.10 through the latest release.
- Every test that needs the xcl sources finds them through `XCL_SRC`, or the sibling checkout by default, and skips with a message naming `XCL_SRC` when neither exists.
- Fixture plugins start on Windows.
- The interrupt and stall tests check the same behaviour on every platform.

CI keeps the sibling plan's strict mode and engine signal, and extends strict failures to a missing fixture plugin, source checkout or plugin release. It also supplies a GitHub token for plugin downloads.

**Validation point**: On the full CI matrix, every test outside the Docker-plugin and tamper modules passes. The only exceptions are the failures owned by the sibling engine spec: the Windows in-process parity crash and the Alpine load error. Locally, with `XCL_SRC` unset and no sibling checkout, the source-dependent tests skip with the `XCL_SRC` message.

#### - [x] Task: Read the plugin pin on every supported Python
**Id:** d8fe0539-eaef-4f55-a874-3c6569b58a42
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

The release helpers read the pinned plugin release from a TOML file with `tomllib`, which only exists from Python 3.11. On 3.10 the two plugin test modules fail to import before a single test runs. This task reads TOML through `tomllib` where it exists and through the `tomli` backport on 3.10. The backport is installed with the test extras only on 3.10.

*Technical detail:* [context.md#task-read-the-plugin-pin-on-every-supported-python](./context.md#task-read-the-plugin-pin-on-every-supported-python)

**Acceptance criteria**:
- [x] On CPython 3.10 every test module imports and is collected without an import error
- [x] On CPython 3.11 and later no TOML package beyond the standard library is installed with the test extras
- [x] The pinned repository and version are read the same way on every supported Python

#### - [x] Task: Find the xcl sources through one locator
**Id:** 78f1a21e-e0c2-43f6-8272-a90f53c5b644
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Three places work out where the xcl checkout is, and they disagree:
- the schema unit test assumes a sibling directory and fails outright;
- the integration fixtures ignore `XCL_SRC`;
- only the parity fixtures honour it.

This task adds one locator that every test uses: `XCL_SRC` if set, otherwise the sibling checkout. It also adds one skip helper, which skips with a message naming `XCL_SRC` locally and fails instead when the suite runs in strict mode.

*Technical detail:* [context.md#task-find-the-xcl-sources-through-one-locator](./context.md#task-find-the-xcl-sources-through-one-locator)

**Acceptance criteria**:
- [x] With the xcl sources anywhere on disk and `XCL_SRC` pointing at them, the schema, integration and parity tests find them
- [x] With `XCL_SRC` unset and no sibling checkout, every test that needs the sources skips with a message telling the developer to set `XCL_SRC`
- [x] With strict mode on, a missing fixture plugin build or xcl source checkout fails the run instead of skipping
- [x] With strict mode on, a missing engine is reported through the sibling's `skip_missing_engine`, failing only for a route listed in `XCL_TEST_ENGINES`; route-marker skips (`subprocess_only` / `inprocess_only`) stay skips
- [x] Legitimate platform skips (such as fork on Windows) are unaffected by strict mode

#### - [x] Task: Build and find fixture programs with the Windows executable suffix
**Id:** ffaad710-b276-4087-ae44-781a45a46837
**Repo:** xcl-python
**Depends on:**
- 78f1a21e-e0c2-43f6-8272-a90f53c5b644 — Find the xcl sources through one locator
**Execution:** agent

On Windows the fixture plugins and the Go parity program are built without `.exe`, so the engine can't start them. This task builds them with the platform's executable suffix and makes the fixtures that look them up use the same names.

*Technical detail:* [context.md#task-build-and-find-fixture-programs-with-the-windows-executable-suffix](./context.md#task-build-and-find-fixture-programs-with-the-windows-executable-suffix)

**Acceptance criteria**:
- [ ] On Windows the slow and recording fixture plugins start and the async, sync and diff tests that use them pass
- [x] On Linux and macOS the fixture programs keep their current names and behaviour
- [ ] The Go parity program runs on Windows from the same build directory

#### - [x] Task: Make the interrupt and stall tests hold on every platform
**Id:** 7ea63d06-86ef-4ea5-8bbb-0c6eff176922
**Repo:** xcl-python
**Depends on:**
- ffaad710-b276-4087-ae44-781a45a46837 — Build and find fixture programs with the Windows executable suffix
**Execution:** agent

Two tests fail for reasons that have nothing to do with the package. Windows can't send SIGINT to a child process, so the keyboard-interrupt test errors. Slow macOS runners tick a background thread less often than the stall test's fixed threshold of 50. This task delivers the interrupt the way each platform supports, so the child still sees a `KeyboardInterrupt`; on Windows that is Ctrl-Break in a new process group, the method both plans in the epic use. The sibling's `xcl-core` program discards Ctrl-C and Ctrl-Break and runs in the host's process group, so cancellation is driven by the Python host. The sibling plan relies on this test for its Windows interrupt coverage. The task also has the stall test compare against a baseline tick count measured on the same machine.

*Technical detail:* [context.md#task-make-the-interrupt-and-stall-tests-hold-on-every-platform](./context.md#task-make-the-interrupt-and-stall-tests-hold-on-every-platform)

**Acceptance criteria**:
- [ ] On Windows, interrupting a running apply in a child process ends it promptly, reports the interruption and leaves state saved, as on Linux and macOS
- [x] The stall test still fails when an apply blocks other Python threads, and passes on slow runners where threads keep running
- [x] Neither test is skipped on any CI platform

#### - [x] Task: Run CI in strict mode with a GitHub token
**Id:** 2f402e04-31f7-44f4-9e95-885703a1095a
**Repo:** xcl-python
**Depends on:**
- 78f1a21e-e0c2-43f6-8272-a90f53c5b644 — Find the xcl sources through one locator
**Execution:** agent

The sibling plan's CI and release steps already set `XCL_TEST_STRICT=1` and `XCL_TEST_ENGINES` (`subprocess` everywhere, plus `inprocess` on the Linux glibc x86-64 job), including the Alpine `docker run -e` and `musl-wheel.sh`. This task keeps that shape and extends it rather than introducing it. It hands the job's GitHub token to the tests, through the same `-e` passing in Alpine, so that 30 jobs installing a plugin release stay inside GitHub's rate limit. It also drops the workflow description's Docker wording, and the build still comes from the one recorded xcl ref.

*Technical detail:* [context.md#task-run-ci-in-strict-mode-with-a-github-token](./context.md#task-run-ci-in-strict-mode-with-a-github-token)

**Acceptance criteria**:
- [x] Every matrix job runs the suite in strict mode with a GitHub token available to the engine
- [x] Every job, Alpine included, keeps the sibling's `XCL_TEST_STRICT=1` and `XCL_TEST_ENGINES` settings unchanged and also has the GitHub token
- [x] The engine and fixture plugins in every job are still built from the single xcl ref recorded in the repository
- [x] The workflow no longer describes or relies on a Docker engine for the main test matrix

### Milestone 2: A signed test plugin is published for release testing

**What changes**: A new plugin, jumppad-labs/xcl-plugin-test, generated from the plugin template, is published as a signed GitHub release (`v0.1.0`) in the standard asset layout. Its `test "item"` block records every operation it is asked to perform to a log the caller chooses. It carries a sensitive field and a computed field, so masking and computed values can be exercised through an external plugin without any outside service. This gives every xcl binding a release-grade plugin to test the real download, checksum and signature path against.

**Validation point**: The plugin's CI passes (build, lint, unit and end-to-end tests). The `v0.1.0` release lists six platform archives, the checksums file and a signature that verifies against the jumppad-labs release key.

#### - [x] Task: Create the xcl-plugin-test repository from the template
**Id:** 46405e9d-c75f-4264-b0a7-3d21a4f5879c
**Repo:** xcl-plugin-test
**Depends on:** none
**Execution:** human — creates a GitHub repository outside any checkout and installs the jumppad-labs release signing key as repository secrets

A person creates jumppad-labs/xcl-plugin-test on GitHub from the xcl-plugin-template template. They give it the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` secrets for the jumppad-labs release key, and turn the already-registered local directory into its checkout. The Spektacular footprint in that directory is kept and committed. Nothing else in the plugin can be built until this exists.

*Technical detail:* [context.md#task-create-the-xcl-plugin-test-repository-from-the-template](./context.md#task-create-the-xcl-plugin-test-repository-from-the-template)

**Acceptance criteria**:
- [x] jumppad-labs/xcl-plugin-test exists as a public repository generated from the template
- [x] The repository has the release-key signing secrets the release workflow needs
- [x] The registered local directory is a git checkout of the new repository with its Spektacular footprint committed

#### - [x] Task: Build the test item plugin
**Id:** ff6fefc1-7ae5-4ff2-be61-db1b5d93bff5
**Repo:** xcl-plugin-test
**Depends on:**
- 46405e9d-c75f-4264-b0a7-3d21a4f5879c — Create the xcl-plugin-test repository from the template
**Execution:** agent

This task turns the template's notes example into the test plugin: one `test "item"` block type whose provider appends every operation it starts to a JSON-lines log, with a sensitive `secret` and a computed `output`. The template's layout, client seam, Mockery double, in-process and external end-to-end tests, CI and signed release workflow are kept, and only the names change. The README says what the plugin is for and that it is not an example to copy.

*Technical detail:* [context.md#task-build-the-test-item-plugin](./context.md#task-build-the-test-item-plugin)

**Acceptance criteria**:
- [x] Applying two linked items and destroying them leaves a log listing create, create, destroy, destroy in dependency order
- [x] An item's `output` is filled in after create and recomputed after an in-place update of `value`
- [x] An item's `secret` is treated as sensitive by xcl
- [ ] The plugin builds, lints and passes its unit and end-to-end tests in CI, both in-process and as a separate program
- [x] The release build produces six platform archives and a checksums file named for `xcl-plugin-test`

#### - [x] Task: Publish the first signed test plugin release
**Id:** eafec176-f773-4b3d-8288-18e5391d12a5
**Repo:** xcl-plugin-test
**Depends on:**
- ff6fefc1-7ae5-4ff2-be61-db1b5d93bff5 — Build the test item plugin
**Execution:** human — pushing a release tag publishes a signed release, an action outside the repository that uses the signing secrets

A person merges the plugin to the main branch and tags `v0.1.0`, and the release workflow builds, signs and publishes it. They confirm the release carries every asset the GitHub registry expects.

*Technical detail:* [context.md#task-publish-the-first-signed-test-plugin-release](./context.md#task-publish-the-first-signed-test-plugin-release)

**Acceptance criteria**:
- [x] Release `v0.1.0` of jumppad-labs/xcl-plugin-test is published, not a draft
- [x] It carries six platform archives, the checksums file and a detached signature over it

### Milestone 3: xcl-python's end-to-end and registry tests run without Docker

**What changes**: The Python suite installs the test plugin from its signed release instead of the Docker example plugin. It proves the GitHub registry, path and directory start paths, generic entity reads, recorded operation order, computed values and sensitive masking. The tamper tests are unchanged apart from the plugin they install. A refusal for an untrusted key now also names the key that signed the release. No Docker engine is needed anywhere in the suite. The Docker plugin stays the documented example in the README, and the Development section explains `XCL_SRC` and the test plugin.

**Validation point**: On a machine with no Docker engine, the full xcl-python suite passes with no skipped end-to-end or registry tests. In CI with strict mode, every matrix job's Docker-plugin and tamper tests pass, apart from failures still owned by the sibling engine spec.

#### - [x] Task: Pin the test plugin release in plugin-agnostic release helpers
**Id:** 4aa20a4a-b233-43df-92b0-5262a1f0f2ad
**Repo:** xcl-python
**Depends on:**
- d8fe0539-eaef-4f55-a874-3c6569b58a42 — Read the plugin pin on every supported Python
- 78f1a21e-e0c2-43f6-8272-a90f53c5b644 — Find the xcl sources through one locator
- eafec176-f773-4b3d-8288-18e5391d12a5 — Publish the first signed test plugin release
**Execution:** agent

The suite's pinned release moves from the Docker plugin to `jumppad-labs/xcl-plugin-test` `v0.1.0`. The Docker helpers become general release helpers: the pin, the cache entry, the registry, a once-per-session install and the trusted key. The Docker daemon probe is removed. This task also adds the test plugin configuration the tests apply.

*Technical detail:* [context.md#task-pin-the-test-plugin-release-in-plugin-agnostic-release-helpers](./context.md#task-pin-the-test-plugin-release-in-plugin-agnostic-release-helpers)

**Acceptance criteria**:
- [x] The suite's single plugin pin names the test plugin release
- [ ] The test plugin release is installed and verified once per test session from GitHub with the jumppad-labs key
- [x] Nothing in the suite probes for or needs a Docker engine
- [x] A failure to install the release fails the run under strict mode instead of skipping

#### - [x] Task: Replace the Docker plugin tests with test plugin tests
**Id:** 146a5ff4-3f98-4a92-988d-8454dd07e8c0
**Repo:** xcl-python
**Depends on:**
- 4aa20a4a-b233-43df-92b0-5262a1f0f2ad — Pin the test plugin release in plugin-agnostic release helpers
**Execution:** agent

The four Docker plugin tests are ported to the test plugin: start through the GitHub registry, by path and by directory, and read entities generically. New tests cover the recorded operation order after an apply and a destroy of linked items, the computed value, and sensitive masking in lookups, events and masked state.

*Technical detail:* [context.md#task-replace-the-docker-plugin-tests-with-test-plugin-tests](./context.md#task-replace-the-docker-plugin-tests-with-test-plugin-tests)

**Acceptance criteria**:
- [x] The test plugin starts and applies through the GitHub registry, from its binary path and from its directory
- [x] Applied items read back generically with their meta, references and links
- [x] After an apply and a destroy the test reads back the operations in dependency order
- [x] The computed value is filled in, and the sensitive value is redacted in lookups and event data and not stored in plain text in masked state
- [x] No Docker plugin test remains in the suite

#### - [x] Task: Retarget the tamper tests at the test plugin
**Id:** 7f99a20f-f4df-493b-9a8e-87fdf79bb4c1
**Repo:** xcl-python
**Depends on:**
- 4aa20a4a-b233-43df-92b0-5262a1f0f2ad — Pin the test plugin release in plugin-agnostic release helpers
**Execution:** agent

The fake GitHub API tests keep their four test functions and assertions as they are. They now serve the test plugin's real release files and validate the test plugin configuration. Their gpg-generated untrusted key is kept as it is, as the spec requires.

*Technical detail:* [context.md#task-retarget-the-tamper-tests-at-the-test-plugin](./context.md#task-retarget-the-tamper-tests-at-the-test-plugin)

**Acceptance criteria**:
- [x] A tampered checksums file is refused with a signature error and nothing is cached
- [x] A release checked against a key that did not sign it is refused, with the message saying it does not verify against any trusted key
- [x] A valid release served by the fake API installs and applies, and an unsigned one installs when no keys are trusted
- [ ] The four tests run, rather than skip, on every CI platform

#### - [x] Task: Name the untrusted signing key in the registry refusal
**Id:** 66c6aaa8-4fdf-491b-b36c-8e91ef0e4ed9
**Repo:** xcl
**Depends on:** none
**Execution:** agent

When a plugin release is signed by a key that is not trusted, the engine refuses it but says only that the signature doesn't verify against any trusted key. This task makes the refusal also name the key that made the signature, by its key ID. Someone installing a plugin can then tell which key they would need to trust, and the spec's "refused naming the key" holds for every binding. The existing wording is kept, so nothing that matches on it breaks. The task also adds this spec's entry at the top of xcl's CHANGELOG.

*Technical detail:* [context.md#task-name-the-untrusted-signing-key-in-the-registry-refusal](./context.md#task-name-the-untrusted-signing-key-in-the-registry-refusal)

**Acceptance criteria**:
- [x] A release signed by an untrusted key is refused with a verification error that names the signing key's ID
- [x] The refusal still says the signature does not verify against any trusted key, and is still a plugin verification error
- [x] Tampered, unsigned and correctly signed releases behave exactly as before
- [x] xcl's CHANGELOG opens with an entry for this spec. It explains that the untrusted-key refusal now names the signing key's long ID while keeping the existing wording, and ends by stating there is no breaking change

#### - [x] Task: Assert the untrusted-key refusal names the signing key
**Id:** df430a08-97ca-4594-b7c5-e817826b1a2f
**Repo:** xcl-python
**Depends on:**
- 7f99a20f-f4df-493b-9a8e-87fdf79bb4c1 — Retarget the tamper tests at the test plugin
- 66c6aaa8-4fdf-491b-b36c-8e91ef0e4ed9 — Name the untrusted signing key in the registry refusal
**Execution:** agent

A new registry test, added beside the unchanged tamper tests, installs the test plugin's real release through the fake GitHub API while trusting a different key. It checks that the refusal names the jumppad-labs release key that actually signed the release. This proves the "refused naming the key" criterion through the Python API.

*Technical detail:* [context.md#task-assert-the-untrusted-key-refusal-names-the-signing-key](./context.md#task-assert-the-untrusted-key-refusal-names-the-signing-key)

**Acceptance criteria**:
- [x] Trusting a key other than the release key, installing the test plugin is refused with a plugin verification error naming the release signing key's ID
- [x] The existing four tamper tests are unchanged apart from the plugin they install

#### - [x] Task: Describe the test setup in the README
**Id:** ba4a50af-153d-4004-8a0e-99a592556d33
**Repo:** xcl-python
**Depends on:**
- 146a5ff4-3f98-4a92-988d-8454dd07e8c0 — Replace the Docker plugin tests with test plugin tests
**Execution:** agent

The README's Development section explains:
- where the suite looks for the xcl sources and how to point it elsewhere with `XCL_SRC`;
- that the end-to-end and registry tests install the signed test plugin and need no Docker;
- what strict mode does.

The Getting started example keeps the Docker plugin as the documented example.

*Technical detail:* [context.md#task-describe-the-test-setup-in-the-readme](./context.md#task-describe-the-test-setup-in-the-readme)

**Acceptance criteria**:
- [x] A developer can learn from the README how to run the suite against an xcl checkout anywhere on disk
- [x] The README says the tests use the test plugin and need no Docker, while its Getting started example still shows the Docker plugin

## Open Questions

- **Does the Windows break event reach the child apply on GitHub's Windows runners?** This depends on how the runner's console and process groups behave, which can only be seen by running CI. The sibling's `xcl-core` engine program discards Ctrl-C and Ctrl-Break through `signal.Notify` (not `signal.Ignore`) and runs in the host's process group, so the Python host's `KeyboardInterrupt` handling drives cancellation. If `CTRL_BREAK_EVENT` doesn't produce a `KeyboardInterrupt` in the child, or the cancellation doesn't stop the engine, STOP and ask the user. Do not skip the test on Windows.
- **Has the sibling engine plan already reshaped the interrupt, stall or fixture-lookup tests, or the CI test step, by the time this plan runs?** That depends on the order the epic orchestrator picks for the two plans. If those tests or steps have changed, apply this plan's intent (platform-appropriate interrupt delivery, baseline-calibrated stall check, `.exe` lookups, strict mode and token) to the new shape. If the intent no longer applies, STOP and ask the user before dropping it.

- **Does gpg generate the untrusted key on every CI runner, Windows in particular?** That depends on the runner images (Git for Windows' gpg on the bash `PATH`, with Windows-style temp paths), so it can only be seen in CI. If the `other_key` fixture skips or fails on any runner, STOP and ask the user. Keeping the tamper tests unchanged is a spec constraint, so the alternative is a committed throwaway public key fixture, which would need their agreement.

## Out of Scope

- **The engine program, the subprocess transport, the musl wheels and the Windows x86-64 wheel** (spec Non-Goal). These include the Windows in-process parity access violation and the Alpine `libxcl.so` TLS load failure seen in CI. They are tracked by spec 20261011211634-a8895873-python-out-of-process-engine.
- **Python-written plugins or a Python plugin SDK** (spec Non-Goal).
- **Reworking release workflows beyond the test plugin's own release** (spec Non-Goal). xcl-python's `release.yml` and xcl's `libxcl-release.yml` are untouched.
- **Changing the Docker example plugin or its tests** (spec Constraint). jumppad-labs/xcl-plugin-docker keeps its repository, tests and place as the README's documented example.
- **Switching `xcl-ref.txt` from `f-libxcl` to `main`.** This plan makes and checks that the switch is a single change. Making it waits for f-libxcl to merge into xcl's main line after the epic completes, and is a one-line follow-up at that point.
- **Moving xcl's own tests that mention xcl-plugin-docker** (asset-name unit tests, the remote-plugin example) to the test plugin. The spec scopes the Docker removal to xcl-python's suite.

## Changelog

### 2026-10-13 — Task: Read the plugin pin on every supported Python

**What was done**: The release helpers now read `tests/plugins.toml` through `tomllib` on CPython 3.11+ and through the `tomli` backport on 3.10, and the `test` extra installs `tomli` only under `python_version < '3.11'`, with a comment saying why. A test reads the pin through the helper on whichever interpreter runs it.

**Deviations**: The pin-reader test lives at `tests/integration/test_plugin_pin.py` rather than `tests/unit/test_plugins_toml.py`: the helper module sits in `tests/integration/`, which pytest's prepend import mode puts on `sys.path` only for tests in that directory, so a unit test could not import it without a second import path. The test needs no engine. Test authored directly rather than through a sub-agent, given its size.

**Files changed**:
- `xcl-python: pyproject.toml`
- `xcl-python: tests/integration/docker_helpers.py`
- `xcl-python: tests/integration/test_plugin_pin.py`

**Discoveries**: Verified locally with a uv-managed CPython 3.10.21: the full suite collected and passed (244 passed, 1 route skip); before the change collection failed with `No module named 'tomllib'`. On 3.14 `tomli` is not installed with `.[test]`.

### 2026-10-13 — Task: Find the xcl sources through one locator

**What was done**: Added `tests/xcl_source.py`: `path()` (`XCL_SRC`, else the sibling `../xcl`), `strict()`, `skip(reason, *, route=None)` (no route: fail under `XCL_TEST_STRICT=1`, else skip; with a route: delegates to the sibling's `engines.skip_missing_engine`) and `require(relative)`. A session fixture `xcl_src` in `tests/conftest.py` is the one way tests get the checkout. The schema unit test, the integration and parity conftests and the parity type tests now use it, and every harness skip in those conftests goes through `xcl_source.skip`.

**Deviations**: The out-of-process-engine spec had added a fourth private locator, `xcl_source()` in `tests/integration/test_engine_process.py`; it was removed and `test_engine_of_other_version_refused` now takes the `xcl_src` fixture (its go-missing skip goes through `xcl_source.skip`). The parity conftest's `FIXTURES` module constant, which `tests/parity/test_types.py` imported, became a `parity_fixtures` session fixture, and the seven type tests take it. Unit tests written directly rather than through a sub-agent.

**Files changed**:
- `xcl-python: tests/xcl_source.py`
- `xcl-python: tests/conftest.py`
- `xcl-python: tests/unit/test_schema.py`
- `xcl-python: tests/unit/test_xcl_source.py`
- `xcl-python: tests/integration/conftest.py`
- `xcl-python: tests/integration/test_engine_process.py`
- `xcl-python: tests/parity/conftest.py`
- `xcl-python: tests/parity/test_types.py`

**Discoveries**: Checked from a copy of the package with no sibling `xcl`: with `XCL_SRC` unset, the 16 source-dependent tests skip with "the xcl sources are not at …; set XCL_SRC to the xcl checkout"; with `XCL_TEST_STRICT=1` they error instead; with `XCL_SRC` set, 254 pass. The suite passes strict on CPython 3.10 and 3.14.

### 2026-10-13 — Task: Build and find fixture programs with the Windows executable suffix

**What was done**: `make plugins` builds `xcl-plugin-slow`, `xcl-plugin-recording` and `parity` with `$(EXE)`, the `.exe`-on-Windows suffix the out-of-process-engine spec had already defined in the Makefile for the engine program. `xcl_source.executable(name)` gives the same name in Python, and the integration conftest (`SLOW`, `RECORDING`) and the parity conftest (`wanted`, the `parity` program path) look the binaries up through it.

**Deviations**: The Makefile already had an `EXE` variable (added by the sibling for `xcl-core`), so it was reused rather than adding a second one. The two Windows acceptance criteria are left unchecked: they can only be observed on the Windows CI runners, which this run cannot push to. Locally, Linux names are unchanged (strict suite 254 passed) and `make -n plugins EXE=.exe` names all three programs with `.exe`.

**Files changed**:
- `xcl-python: Makefile`
- `xcl-python: tests/xcl_source.py`
- `xcl-python: tests/integration/conftest.py`
- `xcl-python: tests/parity/conftest.py`

**Discoveries**: xcl's directory discovery (`registry/discovery.go`) and plugin naming (`plugins/grpc_plugin_host.go`) already strip `.exe` on Windows, so the `.exe` fixture plugins are found by directory too. Outstanding (needs GitHub runners): the Windows jobs start the fixture plugins and the Go parity program.

### 2026-10-13 — Task: Make the interrupt and stall tests hold on every platform

**What was done**: The keyboard-interrupt test starts its child in a new process group on Windows and sends `CTRL_BREAK_EVENT`, and SIGINT elsewhere; the child maps `SIGBREAK` to `KeyboardInterrupt` where it exists, so the assertions are unchanged. The stall test now measures a baseline tick count for 1.5 s with nothing running, through a shared `count_ticks` helper, and requires the apply to let the thread tick at least half as often, instead of a fixed 50.

**Deviations**: None. The Windows acceptance criterion is left unchecked until a Windows CI run shows the break event reaching the child (the plan's Open Question).

**Files changed**:
- `xcl-python: tests/integration/test_sync.py`

**Discoveries**: Checked that the calibrated check still catches a stall: a call that holds the GIL for 3.5 s let the counter tick 0 times against a baseline of 149. Both tests pass strict locally on Linux. Outstanding (needs GitHub runners): the Windows interrupt and the macOS stall test.

### 2026-10-13 — Task: Run CI in strict mode with a GitHub token

**What was done**: Both Test steps of `ci.yml` and the Alpine job now pass `GITHUB_TOKEN: ${{ github.token }}` to the tests (into Alpine with `docker run -e GITHUB_TOKEN`) and run `pytest -q -rs`, so any skip is listed in the log. The sibling's `XCL_TEST_STRICT=1` and `XCL_TEST_ENGINES` settings are unchanged, and both jobs still check xcl out at `xcl-ref.txt`. The header comment drops the Docker wording and describes strict mode over sources, fixture plugins and the plugin release, and the signed test plugin release.

**Deviations**:
- Fixed CI failure (e), not in the plan: on macOS `tests/unit/test_subprocess_transport.py::test_engine_killed_fails_waiting_call_with_signal` saw `137` instead of `-SIGKILL`. The fake engine is started with `sys.executable`, and on the macOS runners (framework Python under `/Library/Frameworks`) that process is an interpreter stub that waits for the real interpreter and reports its signal as 128+n, as a shell does. Reproduced locally by running the test with a non-`exec` shell wrapper as `sys.executable` (same `137`). `FakeLauncher` now passes `FAKE_LAUNCHER_PID`, and the fake engine's `Kill` sends SIGKILL to its parent first when that parent is not the launcher, so the process the transport started ends by the signal, as the real engine does. The transport is unchanged: the real engine is a Go program with no stub.
- The Alpine job installs `gnupg`: the tamper tests' `other_key` fixture generates its untrusted key with gpg, and `python:3-alpine` has none, so those registry tests would skip there once they no longer skip for Docker.
- The stall test's threshold is a quarter of the idle baseline rather than the plan's half. The failing macOS runs ticked 36 and 48 times in about 1.56 s, and with an unknown baseline half was too close to call; a real stall still ticks about 0 times (measured: 0 against a baseline of 149).
- Acceptance criterion 4 was ticked once milestone 3's agent tasks had replaced the Docker-plugin tests and deleted the Docker probe; until then the workflow ran tests that needed Docker.

**Files changed**:
- `xcl-python: .github/workflows/ci.yml`
- `xcl-python: tests/fake_engine.py`
- `xcl-python: tests/unit/test_subprocess_transport.py`
- `xcl-python: tests/integration/test_sync.py`

**Discoveries**: `release.yml` and `.github/scripts/musl-wheel.sh` run only `tests/parity` and `tests/integration/test_load.py`, which install no plugin release, so they need no token. Locally, strict, with a token: subprocess route 254 passed and 1 route skip; in-process route (`make libxcl`, `XCL_TEST_ENGINES=subprocess,inprocess`) 243 passed and 12 route skips. Outstanding (needs GitHub runners): the whole matrix, and in particular macOS (killed engine, stall test) and Windows.

### 2026-10-13 — Task: Create the xcl-plugin-test repository from the template

**What was done**: Human task, already carried out by the user before this run; verified rather than performed. `jumppad-labs/xcl-plugin-test` is a public repository generated from `jumppad-labs/xcl-plugin-template`, its Actions secrets list `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` (both set 2026-10-12), and `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test` is a checkout of it (`origin` = `git@github.com:jumppad-labs/xcl-plugin-test.git`) whose second commit, `17c96b7 Add spektacular folder`, commits the Spektacular footprint.

**Deviations**: A `v0.1.0` tag and release already exist on the repository, made from the template's initial commit (`d1a3b60`): its assets are named `xcl-plugin-template_0.1.0_*`, so the GitHub registry cannot install it as `xcl-plugin-test`. It has to be deleted before the plugin's real `v0.1.0` is tagged (see the publish task).

**Files changed**:
- None (no code; the repository and its secrets are on GitHub).

**Discoveries**: The template-made `v0.1.0` release proves the signing secrets work: the release workflow signed and published it.

### 2026-10-13 — Task: Build the test item plugin

**What was done**: Turned the template's notes example into the test plugin, module `github.com/jumppad-labs/xcl-plugin-test`, package `testplugin`. `entities.Item` is `test "item"` (`log`, `value`, `input`, sensitive `secret`, computed `output`). `client/oplog` holds `Operation`, the `OperationLog` interface and the file-backed `File` (JSON lines appended under a mutex, a missing file reads as empty), with its Mockery double regenerated by `make generate`. `providers.ItemProvider` appends each create, read, update and destroy to the item's log before acting, computes `output` as `<name>-<value>` on create and update, keeps it on read, and uses `plugins.DefaultChanged`. `cmd/xcl-plugin-test` serves the plugin; the Makefile builds `xcl-plugin-test`; `examples/basic` holds two linked items; the README says what the plugin is for and that it is not an example to copy.

**Deviations**:
- Requires xcl `v0.5.3`, the latest tag now, rather than the plan's `v0.4.0` ("latest tag at implementation time").
- The e2e behaviours (apply/plan/destroy, create and destroy order between the linked items, computed output, in-place update recomputing output, secret masked in processed event data) live once in `e2e/behaviour_test.go` as functions taking a config constructor, called by one named test per route in `inprocess_test.go` and `external_test.go`; `changes_test.go` checks a value change plans an update. Not table-driven.
- The weekly `current.yml` comment now speaks of the test plugin instead of the template; `ci.yml` and `release.yml` are unchanged.
- Tests written in the main context rather than through a sub-agent.
- Acceptance criterion 4 ("passes … in CI") is left unchecked: `make build`, `make lint` (vet, gofmt, staticcheck) and `make test` (race, unit, in-process, external and state-only e2e) pass locally, but the branch has not run on GitHub Actions, which needs a push.

**Files changed**:
- `xcl-plugin-test: go.mod`
- `xcl-plugin-test: go.sum`
- `xcl-plugin-test: Makefile`
- `xcl-plugin-test: .mockery.yml`
- `xcl-plugin-test: .github/workflows/current.yml`
- `xcl-plugin-test: README.md`
- `xcl-plugin-test: plugin.go`
- `xcl-plugin-test: plugin_test.go`
- `xcl-plugin-test: entities/item.go`
- `xcl-plugin-test: client/oplog/oplog.go`
- `xcl-plugin-test: client/oplog/oplog_test.go`
- `xcl-plugin-test: client/oplog/mocks/mock_operation_log.go`
- `xcl-plugin-test: providers/item.go`
- `xcl-plugin-test: providers/item_test.go`
- `xcl-plugin-test: cmd/xcl-plugin-test/main.go`
- `xcl-plugin-test: examples/basic/main.xcl`
- `xcl-plugin-test: e2e/main_test.go`
- `xcl-plugin-test: e2e/behaviour_test.go`
- `xcl-plugin-test: e2e/inprocess_test.go`
- `xcl-plugin-test: e2e/external_test.go`
- `xcl-plugin-test: e2e/changes_test.go`
- `xcl-plugin-test: e2e/stateonly/stateonly_test.go`
- Removed: `xcl-plugin-test: entities/note.go`, `providers/note.go`, `providers/note_test.go`, `client/files/` (client, test, mock), `cmd/notes/main.go`

**Discoveries**: `types.Sensitive[T]` has an unexported value; build it with `types.NewSensitive(v)` and read it with `Reveal()`. `Config.ResourceCount` is deprecated in v0.5.3 (staticcheck SA1019), and `EntityCount` counts variables too, so "nothing left after destroy" is asserted with `FindByType`. `make dist VERSION=v0.0.0-snapshot` produced the six archives and `xcl-plugin-test_0.0.0-snapshot_checksums.txt` (zip was supplied locally through a `python3 -m zipfile` shim; the CI runner has zip).

### 2026-10-13 — Task: Pin the test plugin release in plugin-agnostic release helpers

**What was done**: `tests/plugins.toml` pins `[test]` `jumppad-labs/xcl-plugin-test` `v0.1.0`. `docker_helpers.py` became `release_helpers.py` (`git mv`): `plugin_release()` reads `["test"]`, `TEST_PLUGIN_CONFIG` points at `configs/test_plugin/main.xcl` (moved with `git mv` from `configs/docker/`), `FIRST_ADDRESS`/`SECOND_ADDRESS` replace the Docker constants, `log_variables(log)` gives the configuration its log, and the Docker probe (`DEFAULT_DOCKER_HOST`, `_UnixHTTPConnection`, `daemon_unreachable`, `docker_unavailable`, `require_docker`) is deleted. `installed_release` validates the test plugin configuration and turns an install failure into `xcl_source.skip(...)`, a failure under strict mode.

**Deviations**:
- `variable "log"` has `default = "ops.jsonl"`: xcl refuses a variable block without a default ("The argument \"default\" is required"), so the plan's `variable "log" {}` does not parse. The tests always pass `log`.
- This task depends on the human task "Publish the first signed test plugin release", which is not done. It was implemented anyway, as the orchestrator's CI goal needs the Docker probe gone (Windows `AF_UNIX`). Acceptance criterion 2 is left unchecked until the real release exists: today `v0.1.0` of the repository is the template-made release, whose assets are named `xcl-plugin-template_*`, so the install reports "has no build for linux/amd64" and the tests skip locally (fail under strict).
- Verified instead against a release built locally from the plugin worktree (`make dist VERSION=v0.1.0`), signed with a throwaway gpg key and served through the suite's own `FakeGitHub` by a scratch, uncommitted pytest plugin that points `xcl.GitHubRegistry`'s default API and the trusted key at it.

**Files changed**:
- `xcl-python: tests/plugins.toml`
- `xcl-python: tests/integration/release_helpers.py` (renamed from `tests/integration/docker_helpers.py`)
- `xcl-python: tests/integration/configs/test_plugin/main.xcl` (renamed from `tests/integration/configs/docker/main.xcl`)
- `xcl-python: tests/integration/test_plugin_pin.py`

**Discoveries**: An xcl `variable` block must have a `default`, even when every caller supplies the value.

### 2026-10-13 — Task: Replace the Docker plugin tests with test plugin tests

**What was done**: `test_docker_plugin.py` became `test_test_plugin.py` (`git mv`), with no Docker gate. The registry, path and directory start tests apply the two items with the log in the test's temp directory and check their values, `output`s (`first-one`, `second-two`), `second.input`, and that no item remains after destroy; the directory test keeps its sidecar-files check. The generic-read test checks meta (`type`/`subtype`, `file`, `line` 9 and 15, `status`), `second.meta.references` and links, `__getitem__`, `in` and `to_dict`, with the `AttributeError`/`KeyError` cases in their own test. New tests: `test_recorded_operations_follow_dependencies` (creates first then second, destroys second then first, read from `ops.jsonl`), `test_computed_output_filled_in`, `test_sensitive_secret_redacted_in_lookup`, `test_sensitive_secret_redacted_in_events` (`event_data="processed"`) and `test_sensitive_secret_masked_in_state` (a real apply into `MemoryStore` under `xcl.mask.aes256gcm`).

**Deviations**:
- `second.meta.references` is `{"log": "variable.log", "input": "test.item.first.output"}`: the variable reference is recorded too.
- A generic `xcl.Entity` has no type to rebuild an `xcl.Sensitive` from, so the redacted secret reads back as the string `"(sensitive)"`; the lookup test asserts that and that `hunter2` is nowhere in `to_dict()`, rather than `is_redacted`/`reveal()` as `test_sync.py` does for a typed entity.
- After-destroy emptiness is checked with `find_by_type(None, "test", "item") == []`, which names what must be gone, rather than `entity_count() == 0`.
- Verified against the locally built and signed release (see the pin task); the real release is pending the human task.

**Files changed**:
- `xcl-python: tests/integration/test_test_plugin.py` (renamed from `tests/integration/test_docker_plugin.py`)

**Discoveries**: None.

### 2026-10-13 — Task: Retarget the tamper tests at the test plugin

**What was done**: The tamper tests import from `release_helpers`, drop `require_docker` from their imports, `__all__` and `pytestmark`, validate and apply `TEST_PLUGIN_CONFIG` with a `log` variable, and the docstring names the test plugin release. In `test_fake_api_serves_valid_release_installs` the network and container checks became `first.output == "first-one"`; every request, cache and remaining-count assertion, the four test names and their refusal assertions, `FakeGitHub` and the gpg-generated `other_key` fixture are unchanged.

**Deviations**: Acceptance criterion 4 ("run, rather than skip, on every CI platform") is left unchecked until CI runs with the real release. The Alpine job now installs `gnupg` (recorded under the CI task) so `other_key` does not skip there; the other runner images ship gpg. Verified locally against the locally built and signed release: all four pass, strict.

**Files changed**:
- `xcl-python: tests/integration/test_github_tamper.py`

**Discoveries**: `entity_count()` after destroy is still 0 with the configuration's variable, so the remaining-count assertion was kept as it was.

### 2026-10-13 — Task: Name the untrusted signing key in the registry refusal

**What was done**: `verifySignature` reads the signature file once and, when `openpgp.CheckArmoredDetachedSignature` fails with `pgperrors.ErrUnknownIssuer`, returns `ErrPluginVerification` with the existing "signature <file> does not verify against any trusted key" text followed by "signed by key <%016X>, which is not trusted". The ID comes from the new `signatureIssuer(signature []byte) (uint64, bool)`, which decodes the armour, reads the signature packet and takes `IssuerKeyId`, or the low 64 bits of `IssuerFingerprint`. Every other failure, and a signature whose issuer cannot be read, keeps the previous message. `testutil.FakeGitHubRelease.OtherKeyID()` gives the ID of the key `SignedByOtherKey` signs with. xcl's CHANGELOG opens with this spec's entry, ending `**Breaking:**` / `None.`.

**Deviations**: `OtherKeyID` uses the other key's signing key (`Entity.SigningKey`), falling back to the primary key, rather than `PrimaryKey.KeyId` alone, so it stays right if the fake ever signs with a subkey. No new dependency: `openpgp/armor`, `openpgp/errors` and `openpgp/packet` are from `github.com/ProtonMail/go-crypto` v1.1.6, already required.

**Files changed**:
- `xcl: registry/github_verify.go`
- `xcl: registry/github_verify_test.go`
- `xcl: registry/github_signature_test.go`
- `xcl: internal/testutil/github_release.go`
- `xcl: CHANGELOG.md`

**Discoveries**: `go test ./...` and `go vet` of the touched packages pass in the xcl worktree; `TestGitHubPluginStartWithTrustedKeyRefusesReleaseSignedByOtherKey` is unchanged and still passes beside the new `…RefusalNamesTheSigningKey`. The change reaches xcl-python CI only once it is on xcl `f-libxcl`, the ref in `xcl-ref.txt`.

### 2026-10-13 — Task: Assert the untrusted-key refusal names the signing key

**What was done**: `release_helpers.RELEASE_SIGNING_KEY_ID = "F775DA00AFD4B502"`, with a comment on where it comes from, and a new `test_untrusted_key_refusal_names_signing_key` beside the four unchanged tamper tests: the real release files served by `fake_github`, `other_key` trusted, and the `PluginVerificationError` must contain "signed by key F775DA00AFD4B502, which is not trusted".

**Deviations**: The key ID was read with `gpg --list-packets` from the `.sig` of the release already on jumppad-labs/xcl-plugin-test (the template-made `v0.1.0`, signed by the release workflow with the same secrets), since the plugin's own release is not published yet: issuer `F775DA00AFD4B502`, fingerprint `15BDA6843A1FA0EAD1AA5190F775DA00AFD4B502`, the primary key of `tests/keys/jumppad-labs.asc`, which has no signing subkey. Verified locally with the engine and the shared library rebuilt from the xcl worktree, against the locally signed release (the scratch plugin substitutes the local key's ID).

**Files changed**:
- `xcl-python: tests/integration/release_helpers.py`
- `xcl-python: tests/integration/test_github_tamper.py`

**Discoveries**: In CI this test needs an engine built from an xcl ref that includes the key-naming change; until that change is on `f-libxcl` it fails there.

### 2026-10-13 — Task: Describe the test setup in the README

**What was done**: The README's Development section now says the tests find the xcl sources through `XCL_SRC`, else a sibling `../xcl`, how to run against a checkout elsewhere, and that tests needing the sources skip naming `XCL_SRC`; that the end-to-end and registry tests install the signed `jumppad-labs/xcl-plugin-test` release pinned in `tests/plugins.toml` and need network but no Docker, with `GITHUB_TOKEN` for the rate limit; and what `XCL_TEST_STRICT=1` fails and what stays a skip. Getting started still shows the Docker plugin.

**Deviations**: None.

**Files changed**:
- `xcl-python: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: Publish the first signed test plugin release (outstanding: human)

**What was done**: Nothing; this is the user's task and stays unchecked. The agent tasks that depend on it were implemented and verified against a locally built, locally signed release. Until it is done, xcl-python's test-plugin and tamper tests cannot install the pinned release: they skip locally and fail in CI under strict mode.

**Deviations**: A `v0.1.0` tag and release already exist on jumppad-labs/xcl-plugin-test, made from the template's initial commit, with assets named `xcl-plugin-template_0.1.0_*`. They must be removed before the plugin's `v0.1.0` is tagged.

**Files changed**:
- None.

**Discoveries**: Steps for the user, from the xcl-plugin-test checkout once this spec's branch is merged:
1. Push `main` and wait for the plugin's CI (build, lint, test, dist) to go green.
2. Remove the template-made release and its tag: `gh release delete v0.1.0 -R jumppad-labs/xcl-plugin-test --cleanup-tag --yes`, then `git tag -d v0.1.0` locally.
3. Tag and push: `make release VERSION=v0.1.0`. The release workflow tests, builds, signs with `GPG_PRIVATE_KEY`/`GPG_PASSPHRASE` and publishes.
4. Check: `gh release view v0.1.0 -R jumppad-labs/xcl-plugin-test` lists `xcl-plugin-test_0.1.0_{linux,darwin,windows}_{amd64,arm64}` (`.zip` for Windows), `xcl-plugin-test_0.1.0_checksums.txt` and `.sig`, and `gpg --verify` of the `.sig` against xcl-python's `tests/keys/jumppad-labs.asc` succeeds (issuer `F775DA00AFD4B502`).
If reusing `v0.1.0` is not wanted, tag `v0.1.1` instead and change `version` in xcl-python's `tests/plugins.toml`, a one-line change.

### 2026-10-13 — Task: Publish the first signed test plugin release (done)

**What was done**: The user pushed xcl-plugin-test `main` (its CI, run 38384180661, passed), deleted the template-made `v0.1.0` release and tag, and ran `make release VERSION=v0.1.0`. Release run 38384311373 published `v0.1.0`, which is not a draft. It carries `xcl-plugin-test_0.1.0_{darwin,linux}_{amd64,arm64}.tar.gz`, `xcl-plugin-test_0.1.0_windows_{amd64,arm64}.zip`, `xcl-plugin-test_0.1.0_checksums.txt` and its `.sig`. `gpg --verify` of the `.sig` against xcl-python's `tests/keys/jumppad-labs.asc` reports a good signature from `15BDA6843A1FA0EAD1AA5190F775DA00AFD4B502` (issuer `F775DA00AFD4B502`).

**Deviations**: None. The tag is `v0.1.0`, so `tests/plugins.toml` is unchanged.

**Files changed**:
- None.

**Discoveries**: xcl-python's CI tamper test for the key-naming message needs xcl's merged change on `f-libxcl`, the ref in `xcl-ref.txt`. A CI run started before that push fails that one test.

