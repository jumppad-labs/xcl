---
created_date: "2026-10-13"
document_status: final
closed_date: "2026-10-13"
---

# Test plan: 20261011232013-da18c586-python-release-hardening

The automated tests cover the behaviour. These are the success metrics and reviews the plan marks as manual. Several of them can only run once three things are in place, in this order:

1. This spec's xcl change, the untrusted-key refusal naming the key, is on xcl `f-libxcl`, the ref in xcl-python's `xcl-ref.txt`.
2. jumppad-labs/xcl-plugin-test `v0.1.0` is published from the plugin built here. This is the outstanding human task "Publish the first signed test plugin release", and the plan's changelog has its steps. The `v0.1.0` that exists today was made from the template, has `xcl-plugin-template_*` assets, and must be deleted first.
3. xcl-python `main` carries this spec's changes.

## 1. Success metric: the xcl-python CI matrix is fully green on the first run

- **What to measure**: the conclusion of every job of xcl-python's `CI` workflow. That is 6 runners × 5 Pythons (`ubuntu-latest`, `ubuntu-24.04-arm`, `macos-15-intel`, `macos-latest`, `windows-latest`, `windows-11-arm` × `3.10`, `3.11`, `3.12`, `3.13`, `3.x`), plus `alpine / x86_64`, `alpine / aarch64` and `Check the package`. Threshold: every job `success`, with 0 failed and 0 errored tests.
- **How**: once prerequisites 1 to 3 are in place, push xcl-python `main` (or run `gh workflow run ci.yml -R jumppad-labs/xcl-python`). Then:
  - `gh run list -R jumppad-labs/xcl-python --workflow ci.yml -L 1` gives the run id;
  - `gh run view <id> -R jumppad-labs/xcl-python --json jobs --jq '.jobs[] | "\(.name) \(.conclusion)"'` lists each job's conclusion.
- **Expected result**: every line ends `success`. Look at these jobs in particular, because they could only be checked on GitHub's runners:
  - `windows-*`: `test_sync.py::test_keyboard_interrupt_cancels_apply` passes (Ctrl-Break in a new process group), and the slow and recording fixture plugins and the `parity.exe` program start.
  - `macos-*`: `test_sync.py::test_sync_apply_does_not_stall_threads` passes (baseline-calibrated), and `test_subprocess_transport.py::test_engine_killed_fails_waiting_call_with_signal` passes (the fake engine kills its interpreter stub).
  - `*/py3.10`: collection succeeds (`tomli` fallback).
  - every job: `test_schema.py::test_annotations_dataclass_matches_parity_fixture` passes (it reads `XCL_SRC`).
  - every job: `test_github_tamper.py::test_untrusted_key_refusal_names_signing_key` passes. This needs prerequisite 1.
- **If it fails**: if a Windows job's interrupt test fails because the break event does not reach the child, or the engine is not cancelled, that is the plan's open question and goes to the user; do not skip the test. If the `other_key` fixture skips or fails on a runner, also take it to the user.
- **Who / when**: the person cutting the next xcl-python release, on the first CI run after this spec lands.

Also check the plugin's own CI once its branch is pushed. `gh run list -R jumppad-labs/xcl-plugin-test --workflow ci.yml -L 1` must show the `check` job (build, lint, test) and the `dist` job green.

## 2. Success metric: zero Docker-gated skips in xcl-python's CI

- **What to measure**: the skips in one CI job per platform. Threshold: 0 skips in `tests/integration/test_test_plugin.py` and `tests/integration/test_github_tamper.py`, and no skip anywhere that mentions Docker.
- **How**: the Test step runs `pytest -q -rs`, so each log ends with a `SKIPPED [n] file:line: reason` list. Download one job each for `ubuntu-latest`, `macos-latest`, `windows-latest` and `alpine / x86_64`:
  - `gh run view <run> -R jumppad-labs/xcl-python --job <job-id> --log | grep -E "SKIPPED|passed|failed"`
  - the job ids come from `gh run view <run> -R jumppad-labs/xcl-python --json jobs --jq '.jobs[] | "\(.databaseId) \(.name)"'`.
- **Expected result**: none of the SKIPPED lines names `test_test_plugin.py` or `test_github_tamper.py`, and none mentions Docker. The skips that may remain:
  - route skips ("runs only on the inprocess route…" / "…subprocess route…");
  - platform skips: `os.fork is not available`, `POSIX signals and process groups are not available on Windows`, and `SIGKILL is not available` on Windows.
- **Who / when**: same as 1, on the same run.

## 3. Review: the first jumppad-labs/xcl-plugin-test release

- **What to look at**: the GitHub release `v0.1.0` of jumppad-labs/xcl-plugin-test.
- **How**:
  - `gh release view v0.1.0 -R jumppad-labs/xcl-plugin-test` shows the release;
  - in an empty directory, `gh release download v0.1.0 -R jumppad-labs/xcl-plugin-test --pattern '*checksums*'`, then `gpg --import <xcl-python>/tests/keys/jumppad-labs.asc && gpg --verify xcl-plugin-test_0.1.0_checksums.txt.sig xcl-plugin-test_0.1.0_checksums.txt`.
- **Passing**:
  - the release is not a draft;
  - it has exactly these assets: `xcl-plugin-test_0.1.0_linux_amd64.tar.gz`, `…_linux_arm64.tar.gz`, `…_darwin_amd64.tar.gz`, `…_darwin_arm64.tar.gz`, `…_windows_amd64.zip`, `…_windows_arm64.zip`, `xcl-plugin-test_0.1.0_checksums.txt` and `xcl-plugin-test_0.1.0_checksums.txt.sig`, with no `xcl-plugin-template_*` asset left;
  - gpg reports a good signature from key `15BDA6843A1FA0EAD1AA5190F775DA00AFD4B502` (Jumppad Labs).
- **Who / when**: the user, straight after tagging.

## 4. Review: the suite with xcl checked out away from xcl-python

- **What to look at**: the source-dependent tests: `tests/unit/test_schema.py`, `tests/parity/*`, `tests/integration/test_engine_process.py::test_engine_of_other_version_refused`, and the fixture-plugin build.
- **How**:
  1. Copy the xcl-python checkout somewhere with no sibling `xcl`, for example `git clone <xcl-python> /tmp/away/xcl-python`. In that copy, create a venv and run `pip install -e '.[test]'`.
  2. Run `make protos engine plugins XCL_SRC=<xcl checkout> PYTHON=<venv python>`, then `XCL_SRC=<xcl checkout> python -m pytest -q -rs`.
  3. Run `env -u XCL_SRC python -m pytest -q -rs`.
  4. Run `env -u XCL_SRC XCL_TEST_STRICT=1 XCL_TEST_ENGINES=subprocess python -m pytest -q`.
- **Passing**:
  - Step 2: everything passes, with only the route skip for `test_load.py`.
  - Step 3: 16 tests skip, each with "the xcl sources are not at …/xcl; set XCL_SRC to the xcl checkout", and the rest pass.
  - Step 4: the same 16 tests error with that message plus "(XCL_TEST_STRICT=1 turns this skip into a failure)".

  Checked by the implementer on 2026-10-13 with a copied tree: 254 passed (step 2), 16 skipped (step 3) and 16 errors (step 4).
- **Who / when**: a reviewer of this change, before release.

## 5. Review: switching the CI source is one change

- **What to look at**: xcl-python's `.github/workflows/ci.yml`. Both the `test` and `test-musl` jobs read `xcl-ref.txt` in "Read the xcl ref" and check xcl out at it as `xcl-src`.
- **How**: on a branch, change only `xcl-ref.txt`, for example to `v0.5.3`, or to `main` once f-libxcl has merged. Push it and open the run's "Read the xcl ref" and checkout steps in one job of each kind.
- **Passing**: every job checks out `jumppad-labs/xcl` at the new ref and builds the engine, protos and fixture plugins from it (`make protos engine plugins XCL_SRC="$PWD/xcl-src"`), with no other file changed. The run may fail where an older ref lacks this spec's xcl change; the review is about the source switch, not the result.
- **Who / when**: a reviewer, once, before switching `xcl-ref.txt` to `main`.

## 6. Review: the README

- **What to look at**: the `## Development` section and the `## Getting started` section of xcl-python's `README.md`.
- **Passing**:
  - Development explains that the tests find the xcl sources through `XCL_SRC` (default `../xcl`), how to run against a checkout elsewhere, and that source-dependent tests skip naming `XCL_SRC`;
  - it says the end-to-end and registry tests install the signed `jumppad-labs/xcl-plugin-test` release pinned in `tests/plugins.toml`, need network but no Docker, and can use `GITHUB_TOKEN`;
  - it says what `XCL_TEST_STRICT=1` fails and what stays a skip;
  - Getting started still registers `jumppad-labs/xcl-plugin-docker` and applies the Docker network and container.
- **Who / when**: a reviewer of this change, before release.

## 7. Review: xcl's CHANGELOG entry

- **What to look at**: the top of xcl's `CHANGELOG.md`, the `## 20261011232013-da18c586-python-release-hardening` entry directly under `# Changelog`.
- **Passing**:
  - the entry says an untrusted-key refusal now ends "signed by key <16-hex-digit long key ID>, which is not trusted";
  - it says the refusal keeps `ErrPluginVerification` and the "does not verify against any trusted key" wording, and that tampered, unsigned and correctly signed releases are unchanged;
  - it ends with `**Breaking:**` and `None.`.
- **Who / when**: a reviewer of this change, before the xcl release that carries it.
