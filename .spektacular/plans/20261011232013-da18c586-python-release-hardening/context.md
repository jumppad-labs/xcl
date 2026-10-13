---
created_date: "2026-10-12"
document_status: final
closed_date: "2026-10-12"
---

# Context: 20261011232013-da18c586-python-release-hardening

## Current State Analysis

- xcl-python CI (run 38174422045, 2026-10-11) is red on every job. On py3.10 collection fails with `No module named 'tomllib'` (`xcl-python:tests/integration/docker_helpers.py:23`). Every job fails `tests/unit/test_schema.py::test_annotations_dataclass_matches_parity_fixture`, which looks for the xcl sources at `parents[3] / "xcl"` (`xcl-python:tests/unit/test_schema.py:13-15`) rather than at `XCL_SRC`.
- Windows jobs fail in four ways. The Docker probe errors with `socket.AF_UNIX` missing (`docker_helpers.py:120-192`). The fixture plugins are built without `.exe` (`xcl-python:Makefile:33`), so exec fails. `test_keyboard_interrupt_cancels_apply` fails with `Unsupported signal: 2` (`xcl-python:tests/integration/test_sync.py:742`). The parity program exits 0xC0000005, which is the in-process DLL problem and belongs to the sibling spec.
- macOS jobs: `test_sync_apply_does_not_stall_threads` reports "counter thread ran only 25 times during a 1.53s apply" (`test_sync.py:705`). The Docker and tamper tests skip, because there is no daemon.
- Alpine jobs: `libxcl.so` fails to load with a musl TLS relocation error. This belongs to the sibling spec.
- The end-to-end and registry tests install `jumppad-labs/xcl-plugin-docker` v0.2.0 (`xcl-python:tests/plugins.toml`) and need a Docker daemon (`docker_helpers.py:185-192`). The tamper tests generate their untrusted key with gpg (`xcl-python:tests/integration/test_github_tamper.py:166-184`).
- The xcl sources are located in three inconsistent ways:
  - `xcl-python:tests/integration/conftest.py:19` hard-codes the sibling and ignores `XCL_SRC`;
  - `xcl-python:tests/parity/conftest.py:26` honours `XCL_SRC`;
  - `test_schema.py` uses a parent path.
- The untrusted-key refusal (`xcl:registry/github_verify.go:98-103`) names only the signature file ("does not verify against any trusted key: openpgp: signature made by unknown entity"), not the key.
- `xcl-python:xcl-ref.txt` (`f-libxcl`) is already the single ref both CI jobs check out (`xcl-python:.github/workflows/ci.yml:35-45,86-96`).
- The xcl-plugin-test repository is registered with Spektacular at `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test`. The directory holds only the Spektacular footprint, and the GitHub repository does not exist yet.

## Per-Task Technical Notes

### Task: Read the plugin pin on every supported Python

**File changes**
- `xcl-python:pyproject.toml:41-43` — add to the `test` extra: `"tomli>=2.0; python_version < '3.11'"`, with a comment saying it is the 3.10 backport of `tomllib` and is not installed where the standard reader exists (dependencies convention: document third-party reasoning).
- `xcl-python:tests/integration/docker_helpers.py:23` — replace `import tomllib` with `if sys.version_info >= (3, 11): import tomllib` / `else: import tomli as tomllib` (import `sys`). This file is renamed in a later task ("Pin the test plugin release…"); make the change in place here so milestone 1 lands independently.
- `xcl-python:tests/unit/test_plugins_toml.py` (new) — one test: reading `tests/plugins.toml` through the helper returns a `(repository, version)` pair of non-empty strings. This exercises the reader on whichever interpreter runs it. Name the function after the behaviour; no assertion on the file's literal contents (convention: no tests that inspect repository files for their text — this asserts the reader works, not what is pinned).

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution. Verify by running `python3.10 -m pytest --collect-only -q` if a 3.10 interpreter is available, otherwise rely on CI.

### Task: Find the xcl sources through one locator

**File changes**
- `xcl-python:tests/xcl_source.py` (new; importable because `tests/conftest.py:13-17` puts `tests/` on `sys.path`, as for `fakes.py`). It contains:
  - `ROOT` (the package root);
  - `path() -> Path`: `Path(os.environ["XCL_SRC"])` if set and non-empty, else `ROOT.parent / "xcl"`, resolved;
  - `strict() -> bool`: `os.environ.get("XCL_TEST_STRICT") == "1"`;
  - `skip(reason: str, *, route: str | None = None) -> NoReturn`:
    - With no `route` (sources, fixture plugins, plugin release): under strict mode it calls `pytest.fail(reason + " (XCL_TEST_STRICT=1 turns this skip into a failure)")`, otherwise `pytest.skip(reason)`.
    - With a `route` (`"subprocess"` / `"inprocess"`): it calls the sibling's `tests/engines.py::skip_missing_engine(route, reason)`, whose `expected_routes()` reads `XCL_TEST_ENGINES`.

    Do not duplicate the sibling's engine reader. Route-marker skips (`subprocess_only` / `inprocess_only`) are never routed through `skip` and are never strict failures.
  - `require(relative: str) -> Path`: returns `path() / relative` when it exists, else calls `skip(f"the xcl sources are not at {path()}; set XCL_SRC to the xcl checkout")`.
- `xcl-python:tests/conftest.py` — add a session fixture `xcl_src` returning `xcl_source.require("bindings/libxcl")` parent (the checkout root).
- `xcl-python:tests/unit/test_schema.py:13-15,54-57` — drop the `PARITY_TYPES` module constant (`parents[3] / "xcl"`). `test_annotations_dataclass_matches_parity_fixture` takes the `xcl_src` fixture and reads `xcl_src / "bindings/libxcl/testdata/parity/annotations/types.json"`.
- `xcl-python:tests/integration/conftest.py:17-19,24-28,31-48` — replace `XCL_SRC = ROOT.parent / "xcl"` with the locator. `plugins_dir` uses `xcl_source.path()` for `make plugins XCL_SRC=…`, and every `pytest.skip(...)` here becomes `xcl_source.skip(...)`. The go-missing, checkout-missing and make-failure skips use no `route`. A missing-engine skip goes through the sibling's `skip_missing_engine` with its route, either directly or via `xcl_source.skip(..., route=...)`.
- `xcl-python:tests/parity/conftest.py:23-27,33-38,41-46,49-66` — replace the module-level `XCL_SRC`/`FIXTURES` with the locator. The `xcl_src` fixture there is removed in favour of the shared one in `tests/conftest.py`, and `FIXTURES` is computed from it inside the fixtures. Its skips become `xcl_source.skip(...)`.
- `xcl-python:tests/integration/test_load.py:18-21` — if the sibling plan has not already replaced the `needs_engine` `skipif` with its route handling, make it call `skip_missing_engine("inprocess", ...)`. If the sibling's route markers already cover it, leave their handling as it is.
- `xcl-python:tests/unit/test_xcl_source.py` (new). One behaviour per function, using `monkeypatch` and `tmp_path`:
  - (1) `XCL_SRC` set → `path()` is that directory;
  - (2) `XCL_SRC` unset → `path()` is the sibling of the package root;
  - (3) `require` with the sources missing and strict off → `pytest.skip` raised with a message containing `XCL_SRC` (catch `pytest.skip.Exception`);
  - (4) the same with `XCL_TEST_STRICT=1` → `pytest.fail.Exception`;
  - (5) `skip(..., route="inprocess")` delegates to `skip_missing_engine`: with strict on and `XCL_TEST_ENGINES=subprocess` it raises `pytest.skip.Exception`;
  - (6) the same with `XCL_TEST_ENGINES=subprocess,inprocess` → `pytest.fail.Exception`. (If the sibling already tests `skip_missing_engine` this way, assert only the delegation here.)

  Positive and negative cases go in separate functions.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential: locator and its unit tests first, then migrate the three conftests and the schema test, then run the whole suite locally with `XCL_SRC` pointing at `/home/nicj/code/github.com/jumppad-labs/xcl` and once with it unset.

### Task: Build and find fixture programs with the Windows executable suffix

**File changes**
- `xcl-python:Makefile:25-33` — add `EXE := $(shell $(PYTHON) -c "import sys; print('.exe' if sys.platform == 'win32' else '')")` beside `LIBXCL_EXT`. The `plugins` recipe builds `xcl-plugin-slow$(EXE)`, `xcl-plugin-recording$(EXE)` and `parity$(EXE)`.
- `xcl-python:tests/xcl_source.py` — add `executable(name: str) -> str` returning `name + ".exe"` on `win32`.
- `xcl-python:tests/integration/conftest.py:34,54,60` — look the binaries up as `PLUGINS / executable("xcl-plugin-slow")` and the same for recording.
- `xcl-python:tests/parity/conftest.py:52,81` — `wanted` names and the `parity` program path use `executable(...)`. `tests/parity/run_python.py:96-100` and xcl's `bindings/libxcl/cmd/parity/main.go:165` already append `.exe` for the recording plugin; leave them.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential execution. Windows behaviour is confirmed by CI.

### Task: Make the interrupt and stall tests hold on every platform

**File changes**
- `xcl-python:tests/integration/test_sync.py:709-723` (`INTERRUPTED_APPLY`) — after the SIGINT handler line, add `if hasattr(signal, "SIGBREAK"): signal.signal(signal.SIGBREAK, signal.default_int_handler)`. On Windows the break event then raises `KeyboardInterrupt` in the child exactly as SIGINT does elsewhere.
- Context for both plans: this Ctrl-Break-in-a-new-process-group method is the agreed way to test interrupts on Windows. The sibling's `xcl-core` engine program discards Ctrl-C and Ctrl-Break through `signal.Notify` (not `signal.Ignore`) and runs in the host's process group, so the Python host's `KeyboardInterrupt` handling drives cancellation. The sibling plan points its Windows interrupt coverage at this test.
- `xcl-python:tests/integration/test_sync.py:726-755` — start the child with `creationflags=subprocess.CREATE_NEW_PROCESS_GROUP` on `win32`. Deliver `signal.CTRL_BREAK_EVENT` there and `signal.SIGINT` elsewhere. The assertions (`interrupted`, not `completed`, `state:True`, elapsed < 6, return code 0) are unchanged.
- `xcl-python:tests/integration/test_sync.py:674-706` — measure a baseline before the apply: run the same `tick` loop for the same 1.5 s with no apply and record `baseline` ticks. Assert `advanced >= baseline * 0.5`, keeping a clear message, instead of `>= 50`. That still fails when an apply holds the GIL (ticks near 0) and passes on runners with coarse timers. Keep `elapsed >= 1.4` and `done is True`.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Run CI in strict mode with a GitHub token

**File changes**
- `xcl-python:.github/workflows/ci.yml` header comment — say that the end-to-end and registry tests use the signed test plugin release and need no Docker, and drop the Docker sentences. Keep the sibling's wording about strict mode, `XCL_TEST_ENGINES` and musl.
- `xcl-python:.github/workflows/ci.yml` (Test step, as the sibling plan leaves it) — keep the sibling's `XCL_TEST_STRICT: '1'` and `XCL_TEST_ENGINES` (`subprocess`, or `subprocess,inprocess` on the Linux glibc x86-64 job) unchanged. Add `GITHUB_TOKEN: ${{ github.token }}` to `env`, and run pytest with `-rs` so any remaining skips are listed in the log.
- `xcl-python:.github/workflows/ci.yml` (Alpine/musl job) and `xcl-python:.github/scripts/musl-wheel.sh` — the sibling already passes `-e XCL_TEST_STRICT -e XCL_TEST_ENGINES` into `docker run`. Add `-e GITHUB_TOKEN` there, with `GITHUB_TOKEN: ${{ github.token }}` in the step env, and nothing else. Any release-workflow test steps keep the sibling's settings; add the token only where they install the test plugin release.
- No change to how the ref is read (`ci.yml:35-45`, `86-96`): both jobs already check xcl out at `xcl-ref.txt`. Do not add any test that reads ci.yml (convention).

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Create the xcl-plugin-test repository from the template

What the person does:
1. On GitHub, create `jumppad-labs/xcl-plugin-test` (public) with "Use this template" from `jumppad-labs/xcl-plugin-template`.
2. Add the repository secrets `GPG_PRIVATE_KEY` (armoured private half of the jumppad-labs release key, whose public half is `xcl-python:tests/keys/jumppad-labs.asc`) and `GPG_PASSPHRASE`. Or grant the repository access to the organisation secrets if they are org-level.
3. Make `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test` the checkout. That directory already holds the Spektacular footprint (`.spektacular/repo.yaml`, `.spektacular/knowledge/`), so cloning into it directly fails. Use `git init`, `git remote add origin git@github.com:jumppad-labs/xcl-plugin-test.git`, `git fetch origin`, `git checkout -b main --track origin/main`, then commit `.spektacular/`.

How it is checked: `spektacular repo list` shows `xcl-plugin-test` with that root, `git -C <root> remote -v` names the new repository, and the repository's Actions secrets list both GPG secrets.

### Task: Build the test item plugin

**File changes** (all in `xcl-plugin-test`, starting from the template's files)
- `xcl-plugin-test:go.mod:1-7` — module `github.com/jumppad-labs/xcl-plugin-test`, requiring the latest xcl tag (`v0.4.0` today; check with `git -C /home/nicj/code/github.com/jumppad-labs/xcl tag`). Run `go mod tidy`.
- `xcl-plugin-test:Makefile` — `PLUGIN := xcl-plugin-test`. The `build` and `dist` targets build `./cmd/xcl-plugin-test` (template: `./cmd/notes`, Makefile `build` and `dist` recipes).
- `xcl-plugin-test:entities/item.go` (replaces `entities/note.go`) — `Item` as in plan Data Structures: `Log`, `Value`, `Input`, `Secret types.Sensitive[string]` (`xcl:"secret,optional"`, as `xcl:e2e/fixtures/recorder/recorder.go:39`) and `Output` (`xcl:"output,optional,computed"`), each with doc comments in the template's style.
- `xcl-plugin-test:client/oplog/oplog.go` (replaces `client/files/files.go`):
  - `Operation{Operation, Address string}` with JSON tags `operation`/`address`;
  - the `OperationLog` interface (`Append`, `Read`);
  - `NewFile()`, which appends JSON lines under a mutex and creates the file 0644;
  - `Read`, which returns an empty slice for a missing file.

  Model it on `xcl:bindings/libxcl/testdata/plugins/recording/main.go:52-74`.
- `xcl-plugin-test:client/oplog/oplog_test.go` — separate tests: append-then-read keeps order; read of a missing file is empty; append to an unwritable path returns an error.
- `xcl-plugin-test:.mockery.yml` and `client/oplog/mocks/mock_oplog.go` — regenerate the double with `make generate` (template pins mockery v3.8.0).
- `xcl-plugin-test:providers/item.go` (replaces `providers/note.go`) — `ItemProvider` built by `NewItemProvider(log oplog.OperationLog)`:
  - Create, Read, Update and Destroy each call `log.Append(ctx, item.Log, Operation{op, item.Meta.ID})` first and return its error.
  - Create and Update set `Output = Meta.Name + "-" + Value`; Read keeps the old `Output`.
  - Uses `plugins.DefaultChanged` (every setting updates in place).
  - `Functions()` returns nil, and logging goes through the given `logger.Logger`.
- `xcl-plugin-test:providers/item_test.go` (replaces `providers/note_test.go`) — strict Mockery doubles, testify `require`, one behaviour per test:
  - each lifecycle call appends the right record;
  - an append error fails Create and leaves no output;
  - Create and Update compute `output`;
  - Read preserves `output`;
  - `Changed` with a real `[]entity.PropertyChange` for `value` answers update rather than replace (testing-provider-changed-and-update convention).
- `xcl-plugin-test:plugin.go` and `plugin_test.go` — package `testplugin`, `Plugin` registering type `test`, subtype `item`, with `providers.NewItemProvider(oplog.NewFile())`.
- `xcl-plugin-test:cmd/xcl-plugin-test/main.go` (replaces `cmd/notes/main.go`) — `plugins.Serve(&testplugin.Plugin{})`.
- `xcl-plugin-test:examples/basic/main.xcl` — two items, `second.input = test.item.first.output`, a `secret`, and `log = variable.log`.
- `xcl-plugin-test:e2e/main_test.go`, `inprocess_test.go`, `external_test.go`, `changes_test.go`, `stateonly/stateonly_test.go` — port the template's e2e to the item type. Apply the example with `log` in `t.TempDir()`, re-plan with no changes, change `value` and assert an update, then destroy. Read the log back and assert the create order (first before second) and the destroy order (second before first) only between the linked items (assert-ordering convention). Assert `output` and that `secret` is reported sensitive.
- `xcl-plugin-test:README.md` — rewrite: purpose (release-grade plugin for testing xcl bindings' registry, verification and lifecycle; not an example to copy, which is xcl-plugin-docker or the template), the block type, the log format, and how releases are cut (`make release VERSION=…`).
- `xcl-plugin-test:.github/workflows/ci.yml`, `release.yml`, `current.yml` — keep as generated from the template. Only check that nothing names `notes`.

**Complexity**: Medium
**Token estimate**: ~45k tokens
**Agent strategy**: 2-3 parallel agents: (a) entities, the oplog client with tests and the Mockery double; (b) the provider with unit tests (after a's interface exists); (c) plugin, cmd, example, e2e and README. Then one sequential integration pass running `make build lint test dist VERSION=v0.0.0-snapshot`.

### Task: Publish the first signed test plugin release

What the person does: merge the plugin work to `main` in jumppad-labs/xcl-plugin-test and wait for CI to go green. Then run `make release VERSION=v0.1.0` from the checkout, which tags and pushes `v0.1.0`. The release workflow (template `.github/workflows/release.yml`) tests, builds `make dist`, signs the checksums with the release key and runs `gh release create`.

How it is checked: the GitHub release `v0.1.0` lists `xcl-plugin-test_0.1.0_{linux,darwin,windows}_{amd64,arm64}` archives (`.zip` for Windows), `xcl-plugin-test_0.1.0_checksums.txt` and `xcl-plugin-test_0.1.0_checksums.txt.sig`. `gpg --verify` of the `.sig` against `xcl-python:tests/keys/jumppad-labs.asc` succeeds.

### Task: Pin the test plugin release in plugin-agnostic release helpers

**File changes**
- `xcl-python:tests/plugins.toml:1-7` — replace the `[docker]` table with `[test] repository = "jumppad-labs/xcl-plugin-test"`, `version = "v0.1.0"`. Update the comment to describe the test plugin and the release key.
- `xcl-python:tests/integration/docker_helpers.py` → `xcl-python:tests/integration/release_helpers.py` (rename with `git mv`). Changes:
  - `plugin_release()` reads `["test"]`, keeping the tomllib/tomli import from the first task.
  - `DOCKER_CONFIG` becomes `TEST_PLUGIN_CONFIG` pointing at `configs/test_plugin/main.xcl`.
  - The Docker constants (`:38-41`) are replaced by `FIRST_ADDRESS = "test.item.first"` and `SECOND_ADDRESS = "test.item.second"`.
  - `DEFAULT_DOCKER_HOST`, `_UnixHTTPConnection`, `daemon_unreachable`, `docker_unavailable` and `require_docker` (`:120-192`) are deleted.
  - The `installed_release` fixture (`:195-224`) wraps install failures with `xcl_source.skip(...)` under strict rules. It validates the new config with a `log` variable pointing into a temp dir.
  - Update the module docstring.
- `xcl-python:tests/integration/configs/docker/main.xcl` → `xcl-python:tests/integration/configs/test_plugin/main.xcl`, holding:
  - `variable "log" {}`;
  - `test "item" "first" { log = variable.log, value = "one", secret = "hunter2" }`;
  - `test "item" "second" { log = variable.log, value = "two", input = test.item.first.output }`.

  Line positions should be stable, because a test asserts `meta.line`.
**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential. Do this together with the two following tasks in one session if possible, because both import from the renamed module.

### Task: Replace the Docker plugin tests with test plugin tests

**File changes**
- `xcl-python:tests/integration/test_docker_plugin.py` → `xcl-python:tests/integration/test_test_plugin.py` (rename). `pytestmark` drops `require_docker`. Every config call passes `variables={"log": str(tmp_path / "ops.jsonl")}`; check the `Config` signature in `xcl-python:xcl/config.py:47-60`.
  - Port `test_docker_plugin_from_github_registry`, `_from_path` and `_from_directory` (`:50-123`, renamed `test_test_plugin_from_…`). Assert two entities applied, `output` values `first-one` and `second-two`, and zero remaining after destroy. The directory test keeps its sidecar-files assertion.
  - Port `test_plugin_entities_readable_generically` (`:126-154`) to the items:
    - `meta.type == "test"`, `meta.subtype == "item"`, `meta.file`, `meta.line`;
    - `second.meta.references == {"input": "test.item.first.output"}`, and the link present;
    - `__getitem__`, `in` and `to_dict` behave as before;
    - the `AttributeError`/`KeyError` cases stay in their own test, so positive and negative cases are separate.
  - New `test_recorded_operations_follow_dependencies`: apply then destroy, read `ops.jsonl` with `json.loads` per line, and assert the create of first precedes the create of second and the destroy of second precedes the destroy of first. These are linked items (assert-ordering convention).
  - New `test_computed_output_filled_in`.
  - New `test_sensitive_secret_redacted_in_lookup`, mirroring `test_sync.py:156-166`.
  - New `test_sensitive_secret_redacted_in_events`, using `on_event` and `event_data="processed"` as in `test_sync.py:192-200`.
  - New `test_sensitive_secret_masked_in_state`: a real apply with `state_store=MemoryStore()` and `state_mask=xcl.mask.aes256gcm(KEY)` as in `test_sync.py:180-189`; the record for `test.item.first` has no `hunter2`. Import or copy `MemoryStore`/`KEY` from the sync test helpers (`tests/integration/sync_entities.py` or `test_sync.py`), whichever is importable.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution. Run the module locally against the published release (needs network) with `XCL_TEST_STRICT=1`.

### Task: Retarget the tamper tests at the test plugin

**File changes**
- `xcl-python:tests/integration/test_github_tamper.py:1-9` — docstring: "the Docker plugin release" becomes "the test plugin release".
- `xcl-python:tests/integration/test_github_tamper.py:23-42` — import from `release_helpers`. Drop `require_docker` from the imports, `__all__` and `pytestmark`. Import `TEST_PLUGIN_CONFIG`, `FIRST_ADDRESS`.
- `xcl-python:tests/integration/test_github_tamper.py:166-184` — the gpg-generated `other_key` fixture is kept unchanged (spec constraint: the tamper tests are unchanged apart from the plugin they install). CI provides gpg on every runner: ubuntu and macOS images ship GnuPG, and the Windows bash shell has Git for Windows' `gpg` on `PATH`. If a runner skips it, see the plan's Open Questions.
- `xcl-python:tests/integration/test_github_tamper.py:196,218,237,255-271` — `DOCKER_CONFIG` → `TEST_PLUGIN_CONFIG`, with the `log` variable supplied. In `test_fake_api_serves_valid_release_installs`, replace the network/container assertions with the equivalent item assertions (`first.output == "first-one"`), keeping every request, cache and remaining-count assertion as it is.
- Test function names and their assertions about refusal (`code`, `details`, the "does not verify against any trusted key" text, request counts, not-cached) are unchanged.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Name the untrusted signing key in the registry refusal

**File changes**
- `xcl:registry/github_verify.go:83-105` (`verifySignature`) — when `openpgp.CheckArmoredDetachedSignature` fails with `errors.Is(err, pgperrors.ErrUnknownIssuer)`:
  - re-read the signature through `armor.Decode` + `packet.Read`;
  - take `*packet.Signature`'s `IssuerKeyId`, or the low 64 bits of `IssuerFingerprint` when that is nil;
  - return `fmt.Errorf("%w: signature %s does not verify against any trusted key: signed by key %016X, which is not trusted", xclerrors.ErrPluginVerification, names.signature, keyID)`.

  Every other error keeps the existing message with the openpgp error appended. Factor the issuer read into a small helper, `signatureIssuer(signature []byte) (uint64, bool)`, so the signature reader isn't consumed twice: read the signature bytes once with `io.ReadAll`, then pass `bytes.NewReader` to both. Use `github.com/ProtonMail/go-crypto` packages already in go.mod (`openpgp/armor`, `openpgp/packet`, `openpgp/errors`); no new dependency.
- `xcl:registry/github_signature_test.go:56-69` — leave `TestGitHubPluginStartWithTrustedKeyRefusesReleaseSignedByOtherKey` unchanged. Add `TestGitHubPluginStartWithTrustedKeyRefusalNamesTheSigningKey`, which builds `newInstallTestRelease(t, testutil.SignedByOtherKey())`, trusts `fake.PublicKey()`, and requires the error to contain the other key's ID formatted `%016X`.
- `xcl:internal/testutil/github_release.go:197-205` — add `OtherKeyID() string`, returning `fmt.Sprintf("%016X", f.otherKey.PrimaryKey.KeyId)`, or the signing subkey's ID if the fake signs with a subkey; check `newFakeSigningKey` at `:397-405`. This follows the shared-test-helpers convention: helpers live in `internal/testutil`.
- `xcl:CHANGELOG.md:1-3` — add a `## 20261011232013-da18c586-python-release-hardening` entry at the top, directly under `# Changelog`, in the existing entries' form. Write prose saying that a plugin release signed by a key that isn't trusted is now refused with an error naming the signing key's long (16-hex-digit) ID, alongside the existing "does not verify against any trusted key" wording, so callers can see which key they would need to trust. End with `**Breaking:**` and `None.`. No test reads the CHANGELOG.
- `xcl:registry/github_verify_test.go` — a unit test of `signatureIssuer` on a signature made by a generated key (positive), and a separate test on garbage input returning `false` (negative).

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution. Run `go test ./registry/... ./internal/testutil/...` and `go vet ./...`. The change must reach xcl's `f-libxcl` branch (the ref xcl-python CI builds from) before the Python task that asserts it can pass in CI.

### Task: Assert the untrusted-key refusal names the signing key

**File changes**
- `xcl-python:tests/integration/release_helpers.py` — add `RELEASE_SIGNING_KEY_ID`, the 16-hex-digit issuer key ID of the test plugin release's signature. Read it once at implementation from the published `.sig` (`gpg --list-packets <sig>` prints `keyid`); it is the release-key subkey if the key signs with one. Comment where it came from.
- `xcl-python:tests/integration/test_github_tamper.py` — append a new test, `test_untrusted_key_refusal_names_signing_key`, built like `test_untrusted_key_refused` (`:211-224`): the real assets served by `fake_github`, and `other_key` trusted. It asserts `PluginVerificationError` and `RELEASE_SIGNING_KEY_ID in str(raised.value)`. The four existing tests are not edited.

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential execution. Needs an engine built from an xcl checkout that includes the previous task.

### Task: Describe the test setup in the README

**File changes**
- `xcl-python:README.md:240-246` (Development) — explain that the suite finds the xcl sources through `XCL_SRC`, defaulting to a sibling `../xcl` checkout, and that tests needing them skip with a message naming `XCL_SRC` otherwise. Say that `make plugins` builds the fixture plugins from the same location, that the end-to-end and registry tests install the signed `jumppad-labs/xcl-plugin-test` release pinned in `tests/plugins.toml` and need network access but no Docker, and that `XCL_TEST_STRICT=1` (set in CI) turns harness skips into failures.
- `xcl-python:README.md:28-95` (Getting started) — unchanged. The Docker plugin stays the documented example.

**Complexity**: Low
**Token estimate**: ~4k tokens
**Agent strategy**: Single agent, sequential execution.

## Testing Strategy

Per task:
- **Read the plugin pin on every supported Python**: a unit test reads the pin on the running interpreter. The CI py3.10 jobs prove collection.
- **Find the xcl sources through one locator**: locator unit tests (env honoured, sibling default, skip naming `XCL_SRC`, strict failure without a route, and route skips delegated to the sibling's `skip_missing_engine`). The schema test passes on CI, where the checkout is `xcl-src`.
- **Build and find fixture programs with the Windows executable suffix**: covered by the existing async, sync, diff and parity tests on the Windows CI jobs. There is no new test.
- **Make the interrupt and stall tests hold on every platform**: the two existing tests, with unchanged behavioural assertions, run on all platforms.
- **Run CI in strict mode with a GitHub token**: no test reads the workflow (convention). It is verified by CI behaviour and by review.
- **Build the test item plugin**: provider unit tests with Mockery doubles, oplog client unit tests, and in-process, external and state-only e2e tests. Recorded order is asserted only between linked items.
- **Pin the test plugin release in plugin-agnostic release helpers / Replace the Docker plugin tests / Retarget the tamper tests**: integration tests against the real v0.1.0 release. The tests cover start paths, generic reads, recorded operation order, computed output, and sensitive redaction in lookups, events and masked state. The four tamper tests are kept as they are, including their gpg-generated untrusted key.
- **Name the untrusted signing key in the registry refusal**: a new Go unit test that the refusal names the other key's ID, unit tests for the issuer reader, and the existing signature tests unchanged.
- **Assert the untrusted-key refusal names the signing key**: one new Python registry test against the real release.
- **Describe the test setup in the README**: human review only.

Success metrics:
- CI fully green on the first run: Manual — captured in the implementation test plan.
- Zero Docker-gated skips: Behavioural, via strict mode with no Docker gate left. Also Manual — captured in the implementation test plan, as a check of the skip summary.

Manual checks:
- The v0.1.0 release assets and their signature.
- A non-sibling checkout with `XCL_SRC` set, and the skip message when it is unset.
- A ref switch through `xcl-ref.txt` alone.
- A README review.
- A review of the xcl CHANGELOG entry for this spec.

## Project References

- Spec: `20261011232013-da18c586-python-release-hardening` (no design references).
- Epic: `20261010160411-19b965db-language-reference`. Sibling spec `20261011211634-a8895873-python-out-of-process-engine` touches the same xcl-python files (`.github/workflows/ci.yml`, `Makefile`, `tests/integration/conftest.py`, `tests/parity/conftest.py`), and the epic orchestrator orders the two.
- Knowledge (repo `xcl`):
  - conventions: `shared-test-helpers`, `testing-and-mocking`, `testing-provider-changed-and-update`, `assert-ordering-on-graph-parents`, `test-state-from-real-apply`, `dependencies`, `code-style`, `development-standards`;
  - gotcha: `register-repo-before-planning`;
  - learning: `releases-tagged-by-hand`.
- Repo roots:
  - xcl `/home/nicj/code/github.com/jumppad-labs/xcl`
  - xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`
  - xcl-plugin-template `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-template`
  - xcl-plugin-test `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-test`
  - xcl-plugin-docker `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-docker` (reference only)

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The milestone 1 tasks are Low to Medium and can be done in one xcl-python session. The plugin task is Medium (~45k) and splits as described in its entry. The milestone 3 tasks share the renamed helper module and are best done in one session.

## Migration Notes

- `tests/integration/docker_helpers.py` is renamed to `release_helpers.py`, and `test_docker_plugin.py` to `test_test_plugin.py`. Use `git mv` so history follows.
- `tests/plugins.toml` changes its table from `[docker]` to `[test]`. Anyone with local scripts reading the old table must update them.
- The xcl-plugin-test local directory must become a git checkout (human task) before any agent task runs in it. Implement-workflow worktrees need it.

## Performance Considerations

- Each CI job downloads one small plugin archive once per session. `GITHUB_TOKEN` keeps the ~30 jobs inside the API rate limit.
- The suite no longer pulls the nginx image or creates Docker networks, which shortens the integration tests on Linux.
- The stall test's baseline adds about 1.5 s to that one test.
