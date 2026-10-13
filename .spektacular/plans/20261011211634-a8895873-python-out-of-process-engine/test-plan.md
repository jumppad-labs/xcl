---
created_date: "2026-10-13"
document_status: final
closed_date: "2026-10-13"
---

# Test plan: 20261011211634-a8895873-python-out-of-process-engine

The automated tests cover the engine program (`cmd/xcl-core`: `go test ./cmd/xcl-core/ ./coreservice/ -race`), the Python subprocess transport and selection (`tests/unit`), the lifecycle guarantees against the real program (`tests/integration/test_engine_process.py`), the whole suite on both routes, and the first-`Config()` timing. The procedures below are the success metrics and reviews the plan marks as manual, plus the checks that can only run on GitHub or in the v0.5.0 release. They are listed in the order a person meets them while cutting the release.

## Before the tag: push and CI

### 1. xcl Go CI, including the Windows engine job

- **What**: the `xcl-core-windows` job in `.github/workflows/go.yml` passes. It runs the program's pipe tests and `TestCtrlBreakDoesNotStopTheEngineOnWindows`, which sends `CTRL_BREAK_EVENT` to a `CREATE_NEW_PROCESS_GROUP` child. This check only ran as a compile check locally, so it is the open half of the "an interrupt does not stop the program" criterion.
- **How**: push the xcl branch (the ref xcl-python's `xcl-ref.txt` names, `f-libxcl`) and open the Go workflow run.
- **Expected**: the `build`, `build-minimum-go`, `libxcl` and `xcl-core-windows` jobs are all green. `TestCtrlBreakDoesNotStopTheEngineOnWindows` shows `PASS`, not `SKIP`, in the log.
- **Who / when**: the person merging the spec, before tagging.

### 2. xcl-python CI on every runner, Alpine included

- **What**:
  - every matrix job builds the engine program and runs the suite in strict mode;
  - `ubuntu-latest` runs the suite a second time with `XCL_TRANSPORT=inprocess`;
  - both Alpine jobs pass with no C compiler.
- **How**: push the xcl-python branch once xcl is pushed, and open the CI workflow run.
- **Expected**:
  - all jobs are green, including `alpine / x86_64` and `alpine / aarch64`;
  - in each Alpine log the line `a C compiler is installed` is absent;
  - the `Test on the shared library` step runs only on `ubuntu-latest`.
- **Known dependency**: `tests/unit/test_schema.py::test_annotations_dataclass_matches_parity_fixture` reads the parity fixtures from `../xcl` rather than `XCL_SRC`, and fails in the Alpine container. That fix belongs to the sibling spec `20261011232013-da18c586-python-release-hardening`. Land it, or expect this one failure, before judging the Alpine jobs.
- **Who / when**: the person merging the spec, before tagging.

### 3. Manual run of the shared-library build

- **What**: the libxcl toolchain matrix still builds all eight libraries.
- **How**: GitHub Actions → `libxcl build` → Run workflow. Leave `ref` empty, or name the merged branch, and set `version` to e.g. `v0.5.0-check`.
- **Expected**:
  - the five `build-linux-windows` targets, `build-windows-arm64` and both `build-macos` targets are green;
  - eight artifacts appear: `libxcl-linux_amd64`, `libxcl-linux_arm64`, `libxcl-linux_amd64_musl`, `libxcl-linux_arm64_musl`, `libxcl-windows_amd64`, `libxcl-windows_arm64`, `libxcl-darwin_amd64` and `libxcl-darwin_arm64`;
  - nothing is attached to any release.
- **Who / when**: the person merging the spec, any time after merge.

### 4. Review the xcl CHANGELOG entry

- **Where**: `CHANGELOG.md` in xcl, the top entry, `## 20261011211634-a8895873-python-out-of-process-engine`.
- **Look for**: the entry covers
  - the `xcl-core` program and its frame messages (`proto/xcl/core/v1/transport.proto`);
  - `Service.Close`;
  - the eight `xcl-core_<version>_…` release assets built without a C toolchain and signed;
  - libxcl moved to the manual `libxcl-build.yml` while still CI-tested;
  - Python running the engine as a bundled subprocess everywhere, Alpine included, from v0.5.0.
- **Passing**: all five points are present, the entry is prose, and it ends with `**Breaking:**` and `None.`

## The release: tag v0.5.0

### 5. Watch the v0.5.0 release end to end

- **How**: push the annotated tag `v0.5.0` on xcl. This runs `.github/workflows/release.yml` and then dispatches the xcl-python `Release` workflow.
- **Expected in the xcl run**:
  - **No C compiler**: the `Check no C compiler is reachable` step prints `no C compiler is reachable`.
  - **Programs checked**: the `Check the programs` step prints `xcl-core --version: v0.5.0`.
  - **Signature**: the `Sign the checksums` step's `gpg --verify` reports a good signature from the jumppad-labs release key.
  - **Assets**: the GitHub release `v0.5.0` lists exactly these, and no `libxcl_*` asset:
    - `xcl-core_0.5.0_linux_amd64.tar.gz` and `xcl-core_0.5.0_linux_arm64.tar.gz`;
    - `xcl-core_0.5.0_linux_amd64_musl.tar.gz` and `xcl-core_0.5.0_linux_arm64_musl.tar.gz`;
    - `xcl-core_0.5.0_darwin_amd64.tar.gz` and `xcl-core_0.5.0_darwin_arm64.tar.gz`;
    - `xcl-core_0.5.0_windows_amd64.zip` and `xcl-core_0.5.0_windows_arm64.zip`;
    - `xcl-core_0.5.0_checksums.txt` and `xcl-core_0.5.0_checksums.txt.sig`.
  - **Dispatch**: the `Dispatch the xcl-python release` job is green.
- **Independent signature check**: download the checksums file and its signature, then run `gpg --import keys/jumppad-labs.asc` (from xcl-python `tests/keys/jumppad-labs.asc`) and `gpg --verify xcl-core_0.5.0_checksums.txt.sig xcl-core_0.5.0_checksums.txt`. Expect a good signature.
- **Expected in the xcl-python run**:
  - **Wheel jobs**: all eight are green, six native plus `wheels-musl` for two. In each:
    - `Download and verify the engine program` shows `OK` from `sha256sum -c`;
    - the parity suite passes in strict mode;
    - `Check the installed engine program is executable` passes.
  - **Publish**: the publish job collects 8 wheels and an sdist.
  - **PyPI**: https://pypi.org/project/xcl-lang/0.5.0/#files lists eight wheels, `py3-none-` followed by `manylinux_2_28_x86_64`, `manylinux_2_28_aarch64`, `musllinux_1_2_x86_64`, `musllinux_1_2_aarch64`, `macosx_11_0_x86_64`, `macosx_11_0_arm64`, `win_amd64` and `win_arm64`, plus `xcl_lang-0.5.0.tar.gz`.
  - **Install checks**: all eight are green, six native plus two in plain `python:3-alpine`. Each log ends with `applied and read back settings.web; engine v0.5.0`.
  - **Open questions**: the plan's open questions are answered by these logs.
    - **protobuf on Alpine**: on both musl jobs pip installs protobuf from a wheel and builds nothing from source. x86-64 was confirmed locally. If arm64 builds from source, stop and ask, because the spec forbids a compiler on Alpine.
    - **Executable bit**: the executable check passes on all eight wheel jobs. If any fails, stop and ask before adding a run-time `chmod`.
- **Who / when**: the person cutting the release, on release day.

### 6. Success metric: the README's getting-started example on all eight platforms

- **What**: "xcl-lang 0.5.0 installs and runs the README's getting-started example on all eight platforms, `python:3-alpine` x86-64 and arm64 included, on release day." The automated install check covers the apply-and-read part. This procedure runs the README example itself, which needs the Docker plugin and a Docker engine.
- **How**, on each platform with Docker available:
  1. `python -m venv v && v/bin/pip install xcl-lang==0.5.0`; on Alpine use `docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock python:3-alpine sh` with plain `pip install xcl-lang==0.5.0`.
  2. Save the README's getting-started program as `main.py`, its configuration as `infra/main.xcl`, and the jumppad-labs release key from https://xcl.dev/keys/jumppad-labs-releases.asc as `jumppad-labs-releases.asc`.
  3. Run `python main.py`.
- **Expected**:
  - it prints `nginx:1.27-alpine 1`, then the network's address, subnet and Docker id;
  - it destroys what it created and exits 0;
  - no `xcl-core` process remains afterwards (`pgrep xcl-core`, or Task Manager on Windows).
- **Platforms without a Docker engine**: on macOS arm64 runners without Docker, or Windows on arm64, record the install-check result from step 5 instead and note that the Docker part was not run.
- **Who / when**: the person cutting the release, on release day.

### 7. Success metric: release wall time drops against v0.4.0

- **What**: "the release workflow's wall time drops against the v0.4.0 run."
- **How**: open the xcl `Release` run for `v0.5.0` and the v0.4.0 `libxcl release` run 38172909823 (`gh run view 38172909823 --repo jumppad-labs/xcl`), and compare the total duration from the first job's start to the last job's end, xcl side only.
- **Expected**: the v0.5.0 duration is lower than the v0.4.0 duration. Record both numbers.
- **Who / when**: the person cutting the release, after the release.

## After the release

### 8. Review the xcl-python README on PyPI

- **Where**: https://pypi.org/project/xcl-lang/0.5.0/
- **Look for**:
  - the introduction and Install section say the engine is bundled as a program run as a subprocess, the same way, with the same API, on every platform;
  - the platform list includes Linux musl (`musllinux_1_2`), Alpine included;
  - the Errors section names `EngineNotFoundError` and `EngineExitedError`;
  - the Forking section says a child simply creates its own configuration;
  - the Development section explains `make engine`, and `make libxcl` with `XCL_TRANSPORT=inprocess`.
- **Passing**: all present. Nothing says the engine is loaded as a shared library by default.

### 9. Review the xcl.dev install section

- **Where**: https://xcl.dev/ once the xcl-website branch is merged and deployed. Locally, `npm ci && npm run preview` in xcl-website after `npm run build`.
- **Look for**:
  - an `Install` section below the hero, with Go and Python tabs;
  - the Python tab shows `pip install xcl-lang` and says the package bundles the engine and runs it as a subprocess on every platform with the same API;
  - the Python tab lists Linux on glibc and on musl (Alpine), macOS and Windows on x86-64 and arm64, and links to PyPI;
  - the hero badge reads `v0.5.0 · Go and Python`, and the page description no longer says only Go is available.
- **Passing**: all present, and choosing a tab switches the other code boxes on the page too.

### 10. Success metric: no orphaned-engine reports

- **What**: "No open issue reports an orphaned engine process after a Python program exits."
- **How**: on https://github.com/jumppad-labs/xcl-python/issues, search open issues for `orphan`, `xcl-core` and `zombie`, 30 days after the release.
- **Expected**: no open issue reports an `xcl-core` process left running after its Python program exited.
- **Note**: killing the engine itself with SIGKILL leaves its go-plugin plugin processes running, an engine-side limitation found while testing. A report about orphaned `xcl-plugin-*` processes after the engine was killed directly is that limitation, not an engine orphan.
- **Who / when**: a maintainer, 30 days after the release.
