---
created_date: "2026-10-13"
document_status: final
closed_date: "2026-10-13"
---

# Python release hardening

## What was built

xcl-python's test suite now runs on every supported CPython from 3.10 up, on Linux, macOS and Windows, from any checkout layout and without Docker. A purpose-built, signed test plugin replaces the Docker example plugin in the end-to-end and GitHub-registry tests. The work spans three repositories.

**xcl-python (the test suite and CI)**

- **CPython 3.10**: the tests read their pinned plugin release with `tomllib`, or with its `tomli` backport on 3.10. `tomli` is installed with the `test` extra only below 3.11.
- **One xcl source locator**: `tests/xcl_source.py` is the one place the tests find the xcl sources: `XCL_SRC`, else a sibling `../xcl`. A session fixture, `xcl_src`, serves every test that needs them. The schema test, the integration and parity conftests, the parity type tests and the engine-version test used to have four private ways of doing this.
- **One skip helper**: `xcl_source.skip` is the single entry point for harness skips. Under `XCL_TEST_STRICT=1`, as CI runs, a missing source checkout, fixture plugin build or plugin release fails the run instead of skipping. A missing engine is handed to the existing `engines.skip_missing_engine`, so it fails only for a route `XCL_TEST_ENGINES` lists.
- **Windows executables**: `make plugins` builds the fixture plugins and the Go parity program with `.exe` on Windows, and the fixtures look them up by the same names.
- **Interrupt test**: on Windows the keyboard-interrupt test starts its child in a new process group and sends Ctrl-Break, which the child maps to `KeyboardInterrupt`.
- **Stall test**: the thread-stall test compares the apply against an idle baseline measured on the same machine.
- **Killed-engine test**: the fake engine used by the transport unit tests now kills its interpreter stub before itself. On macOS, a stub between the fake and the test turned its SIGKILL into exit status 137.
- **CI**: every job passes `GITHUB_TOKEN` to the tests and lists skips with `pytest -rs`. The Alpine job installs `gnupg` for the registry tests. The workflow no longer describes or needs Docker for the main matrix.
- **Test plugin in place of Docker**: the tests install `jumppad-labs/xcl-plugin-test` `v0.1.0`, pinned in `tests/plugins.toml`, through generic release helpers. The Docker daemon probe is gone.
- **New end-to-end tests** start the plugin through the GitHub registry, by path and by directory, and read its entities generically. They also cover:
  - the recorded operation order of two linked items (create first, then second; destroy second, then first);
  - the computed `output`;
  - the sensitive `secret` redacted in lookups and event data, and absent from state written under an AES-GCM mask.
- **Tamper tests**: the four tests are unchanged apart from the plugin they install. A new test checks that the untrusted-key refusal names the release signing key, `F775DA00AFD4B502`.
- **README**: the Development section describes `XCL_SRC`, the test plugin and strict mode. Getting started still uses the Docker plugin.

**xcl-plugin-test (new plugin)**

- Generated from xcl-plugin-template, and built against xcl v0.5.3.
- One block type, `test "item"`, with `log`, `value`, `input`, a sensitive `secret` and a computed `output`. Its provider appends a JSON line, `{"operation","address"}`, to the item's log before each create, read, update and destroy, through a mockable `OperationLog` client.
- Tested with:
  - provider unit tests on a Mockery double;
  - client tests;
  - in-process, external and state-only end-to-end tests.
- Builds six signed-release archives named for `xcl-plugin-test`.

**xcl (engine)**

- The GitHub registry's refusal of a release signed by an untrusted key keeps its wording and `ErrPluginVerification`, and now ends "signed by key `<16-hex-digit key ID>`, which is not trusted".
- `internal/testutil.FakeGitHubRelease` gains `OtherKeyID`.
- The CHANGELOG has an entry for this spec, with no breaking change.

## Why it matters

The first attempt to publish XCL for Python showed two things that stop a release going out cleanly. The test suite did not run on the oldest supported Python or outside the author's checkout layout. And the end-to-end tests needed a real Docker engine, so on most CI platforms they silently skipped. With these changes, CI can be a trustworthy gate for the people cutting releases: every Python release is verified the same way on every platform, through the real GitHub download, checksum and signature path, and a CI job that is missing something it should have fails instead of skipping.

## Deviations from the plan

- **Human task outstanding**: "Publish the first signed test plugin release" is not done and is left unchecked. A `v0.1.0` made from the template, with `xcl-plugin-template_*` assets, already exists on the repository and must be deleted before the real `v0.1.0` is tagged; the steps are in the plan's changelog and test plan. The tasks that depend on the release were implemented anyway. They were verified against a release built locally from the plugin, signed with a throwaway key and served through the suite's fake GitHub API.
- **Unplanned CI fix**: the macOS failure of `test_engine_killed_fails_waiting_call_with_signal` (`137` instead of `-SIGKILL`) was fixed in the fake engine and its launcher. The cause is an interpreter stub that reports signals as 128+n.
- **Changed while implementing**:
  - The stall threshold is a quarter of the idle baseline, not half.
  - The Alpine CI job installs `gnupg`.
  - The pin-reader test lives in `tests/integration`.
  - The out-of-process-engine spec's own source locator in `test_engine_process.py` was folded into the shared one.
  - The parity fixtures became a `parity_fixtures` fixture.
  - The test plugin configuration's `log` variable has a default, because xcl requires one.
  - The plugin requires xcl v0.5.3, not v0.4.0.
- **Not checked on GitHub runners**: Windows, macOS, the plugin's own CI and the xcl-python matrix as a whole. The acceptance criteria that need them are left unchecked, and the test plan says how to check them.
