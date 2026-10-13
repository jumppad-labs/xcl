---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
designs:
    - source: design
      path: libxcl.md
epic: 20261010160411-19b965db-language-reference
---

# Feature: 20261011211634-a8895873-python-out-of-process-engine

## Overview

The first attempt to publish XCL for Python failed on two of the eight platforms because the engine shipped as a shared library that Python loads into its own process: musl-based systems refuse to load a library built the way Go builds them, and the Windows x86-64 library crashed on load. This spec changes how every Python package carries the engine: as a small bundled program the package runs alongside itself and talks to, on every platform, with exactly the same API, behaviour and results. It removes the C toolchain from the release entirely and completes the first published Python release, v0.5.0, with a package for every supported platform. It is for Python developers on every platform, Alpine included, and for the people maintaining the release.

## Requirements

- [x] **One way to run the engine, everywhere**
  Every Python package carries the engine as a separate program that the package starts and talks to, serving the same operations the shared library served, so no behaviour is reimplemented and no platform is special.
- [ ] **Works on every supported platform, Alpine included**
  Users can install XCL for Python from PyPI on all eight supported platforms without Go or a C compiler, and use it with the same API and the same results for the same configuration, plugins and state.
- [x] **The shared library remains a source-build option**
  A developer building from source can still build the shared library and select it with a user-settable override, so the in-process route stays tested and can be made the default again later without changing the API.
- [x] **Clear error when the engine is missing**
  If the installed package holds neither engine form, the first use fails with an error naming what was looked for and where.
- [x] **Engine lifecycle is invisible in normal use**
  The engine program starts on first use, is shared by every configuration in the process, is stopped when the last configuration is closed or the interpreter exits, and never outlives a host that died without cleaning up.
- [x] **Engine failures are reported, not hidden**
  If the engine program exits unexpectedly, every call waiting on it fails with an error carrying the exit status; the next use starts a fresh engine and the host process survives. The engine's own diagnostics reach the user's standard error.
- [ ] **Interrupts behave as before**
  Pressing Ctrl-C during an operation cancels it through the package, with state saved so far kept, rather than killing the engine out from under it.
- [x] **Fork works**
  A child process forked after the parent used XCL can use XCL by creating its own configuration; using a configuration created before the fork raises a clear error naming that way out.
- [x] **Same guarantees as the shared library gave**
  Version mismatch refusal, cancellation, concurrent calls, events, custom state stores and Python functions behave exactly as the python-support spec defines them.
- [ ] **Released without a C toolchain**
  Every XCL release builds the engine program for all eight supported platforms in one job that needs no C compiler, and attaches the archives to the GitHub release under a signed checksums file. The shared library is no longer part of the release, though it stays buildable and is exercised in CI.
- [ ] **A Python package for every platform**
  The Python release publishes eight wheels, each carrying its platform's engine program, and installs cleanly from PyPI on every platform, including a plain Alpine image with no compiler.
- [ ] **Package version matches the tag**
  Tagging an XCL release publishes the Python package at the same version, and the engine reports that version.
- [x] **Documented**
  The package README and the Python install documentation on xcl.dev say that the engine runs as a bundled subprocess with the same API everywhere, and that Alpine is supported.

## Constraints

- The work is built to the revised design in the `design` source at `libxcl.md`: the engine program serves the same core service over standard input and output with the framing, lifecycle, interrupt and shutdown rules, and the release shape the design settles. The plan designs within that shape and raises any disagreement rather than designing around it.
- musl support is not negotiable: dropping the musl wheels, or shipping them without a working engine, is not an acceptable outcome of this spec.
- Pure Go users stay unaffected: the engine program lives in the root module and builds with cgo disabled, and importing the library pulls in no subprocess.
- Which engine form runs is decided by what the installed package holds and one explicit override, never by probing the machine at runtime.
- The Python API, the core service messages and the C ABI must not change; the engine program carries the existing messages over a new channel, and the shared library keeps working for a source build.
- The eight supported platforms are Linux x86-64 and arm64 on glibc and on musl, macOS x86-64 and arm64, and Windows x86-64 and arm64.
- The arm64 musl CI jobs cannot use GitHub's Alpine job containers (GitHub does not run its actions in Alpine containers on arm64 runners).
- The first published Python release is v0.5.0. The existing v0.4.0 tag and its GitHub release are left as they are.
- This spec is implemented after python-support (`20261011130709-aa3eb933-python-support`), whose Python package, release workflows and parity suite it extends, and before the documentation specs in this epic.

## Acceptance Criteria

- [ ] **Alpine install and apply**
  In a fresh `python:3-alpine` image on x86-64 and on arm64, with no Go or C compiler, `pip install xcl-lang==0.5.0` succeeds and a program registers a class, applies a configuration, reads the entity back and closes; the engine version it reports is `v0.5.0`.
- [ ] **Parity on every wheel**
  The Go–Python parity fixtures produce results identical to Go on each of the eight wheels.
- [x] **Parity on the source-built library**
  From a source checkout with both engine forms built, with the override enabled the full test suite passes on the in-process route on Linux glibc, and with it disabled the same suite passes on the engine program.
- [x] **Missing engine named**
  With the engine program removed from an installed package, the first use raises an error naming the program's and the library's expected locations.
- [x] **One engine per process, cleaned up**
  Creating three configurations in one process starts one engine process; closing the last one, or exiting the interpreter, leaves no engine process running; killing the host process with no chance to clean up leaves no engine process running within a few seconds.
- [x] **Engine death is surfaced**
  Killing the engine process while an apply is in progress makes the apply raise the engine-exited error carrying the signal or exit status; the host process continues, and a new configuration created afterwards works.
- [x] **Ctrl-C cancels**
  Sending an interrupt to a program mid-apply ends the apply with the cancellation error, the entities completed before it are in state, and no engine process is left running.
- [x] **Fork**
  A child forked after the parent used XCL applies a configuration it created itself; using the parent's configuration raises the fork error naming that alternative.
- [x] **Same guarantees**
  An engine of a different version is refused naming both versions; cancelling an asyncio apply stops it with no provider call recorded after the cancellation; a slow plugin operation does not stall other Python threads or other coroutines; events, a custom state store and a Python function all round-trip.
- [ ] **Release attaches the engine program**
  Tagging xcl v0.5.0 attaches eight engine-program archives listed in one checksums file whose signature verifies with the jumppad-labs key, from a job with no C compiler installed; no shared-library archive is attached.
- [x] **Shared library still builds**
  CI builds the shared library on Linux x86-64 and runs its smoke test through the C ABI on every change.
- [ ] **Eight wheels published**
  The xcl-python release for v0.5.0 publishes eight wheels and the sdist to PyPI as xcl-lang 0.5.0, and the install check passes on all eight platforms, the two musl ones in a plain Alpine image.
- [ ] **Docs say so**
  The package README on PyPI and the xcl.dev Python install section each state that the engine runs as a bundled subprocess on every platform with the same API, and list Alpine among the supported platforms.

## Technical Approach

- Per the design constraint, keep the engine program a thin wrapper over the core service, so behaviour tests stay plain Go tests and the two transports cannot drift.
- Replace the shared-library release matrix with the single cgo-disabled job rather than adding to it; move the shared library's toolchain matrix to a manually triggered workflow so it can still be exercised.
- The override is expected to be an environment variable, so the test suite can flip it per run.
- Prefer running the existing Python test suite and parity fixtures over both transports with one switch, rather than writing a second suite.
- Keep the existing hand-started Alpine container pattern for the musl jobs on arm64 runners.
- Risks to watch: process cleanup on Windows (a job object) and at interpreter exit; the engine exiting on end of input so a killed host cannot orphan it; a blocked reader thread that should release the GIL so other Python threads keep running; Windows antivirus heuristics on a freshly started executable, which code signing addresses when a certificate is available.

## Success Metrics

- xcl-lang 0.5.0 is installable and runs the README's getting-started example on all eight platforms, including `python:3-alpine` x86-64 and arm64, on the day of release.
- The parity fixtures show zero differences between Python and Go on every wheel on every release.
- A first `Config()` completes within one second on the CI runners, and the release workflow's wall time drops against the v0.4.0 run.
- No open issue reports an orphaned engine process after a Python program exits.

## Non-Goals

- Releasing the shared library as an artifact, or shipping it in any wheel; it stays a source-build option until a consumer asks for it.
- The Node binding and its transport (a later phase of the design).
- The CI test fixes (Python 3.10 `tomllib`, the fixture path) and the test plugin that replaces the Docker plugin in end-to-end tests: the separate python-release-hardening spec.
- Moving back to the in-process route when Go's linker gains dynamic TLS and the Windows library loads; the design records that as the way back, decided then, not here.
- Platforms that forbid starting a process (iOS, store sandboxes, browsers, edge isolates); they need a static archive or a Wasm build, neither part of this spec.
