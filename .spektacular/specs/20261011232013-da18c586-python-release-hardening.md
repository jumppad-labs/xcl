---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
epic: 20261010160411-19b965db-language-reference
---

# Feature: 20261011232013-da18c586-python-release-hardening

## Overview

The first attempt to publish XCL for Python showed two gaps that stop a release from going out cleanly: the test suite doesn't run on the oldest supported Python or outside the author's machine layout, and the end-to-end tests depend on a real Docker engine. This spec makes the test suite run everywhere it is promised to and replaces the Docker dependency with a purpose-built test plugin, so every Python release is verified the same way on every platform. It is for the people cutting XCL releases.

## Requirements

- [x] **Test suite runs on every supported Python**
  The full xcl-python test suite runs on CPython 3.10 through the latest stable release without import errors.
- [x] **Test suite runs from any checkout layout**
  Every test that needs the xcl sources finds them through the location the suite is given, never by assuming where the xcl checkout sits on disk; when no location is given and none is found, those tests skip with a message naming what to set.
- [ ] **No Docker in the test suite**
  The end-to-end and GitHub-registry tests use a purpose-built test plugin and need no Docker engine; the suite passes in full on every CI platform.
- [ ] **Test plugin published as a signed release**
  The test plugin is published as a release with signed assets, so the tests exercise the real download, checksum and signature path, including refusal of a tampered asset and of an untrusted key.
- [x] **Test plugin covers the plugin behaviours the tests need**
  The test plugin's provider records the operations it is asked to perform, so tests can assert what ran, and its types include sensitive and computed fields so masking and computed values are covered through an external plugin.
- [x] **CI builds the engine from one recorded source**
  xcl-python's CI builds the engine from a single recorded xcl ref, so switching the source line (today the libxcl branch, xcl's main line once that work merges) is one change.

## Constraints

- The Docker example plugin stays the documented example and keeps its own repository and tests; only xcl-python's tests stop depending on it.
- The test plugin follows the standard plugin layout and release-asset contract the GitHub registry already verifies, with assets signed by the jumppad-labs release key.
- This spec is implemented after python-support (`20261011130709-aa3eb933-python-support`) and before the documentation specs in this epic.
- The existing fake-GitHub-API tamper tests are retained unchanged apart from the plugin they install.
- xcl-python supports CPython 3.10 as its minimum version; nothing in this work may raise that floor, and the test suite adds no third-party dependency on Python versions that already ship a standard TOML reader.
- Platform wheels ship a prebuilt engine and must install without a compiler on the target platform.

## Acceptance Criteria

- [ ] **Green on 3.10**
  Every CPython version in the CI matrix, from 3.10 to the latest stable release, passes on Linux, macOS and Windows.
- [x] **Fixtures found through the given location**
  With the xcl sources checked out to a directory that is not next to the package checkout, and the suite told where they are, every test passes; with no location given and no sibling checkout, tests that need them skip with a message naming what to set.
- [ ] **Suite passes without Docker**
  On a runner with no Docker engine, the full xcl-python suite passes with no skipped end-to-end or registry tests.
- [ ] **Registry path exercised by the test plugin**
  Installing the test plugin from the GitHub registry downloads it, verifies its checksum and signature, and an apply through it succeeds; a tampered asset is refused with a signature error, and an asset signed by an untrusted key is refused naming the key.
- [x] **Recorded operations**
  After an apply and a destroy through the test plugin, the test can read back the ordered list of operations the provider performed, and a sensitive attribute of its type is masked in events and state while a computed one is filled in.
- [x] **CI source switch**
  CI builds the engine from the xcl ref recorded for the package, and changing that single ref is enough to switch the build source.

## Technical Approach

- Generate the test plugin from the plugin template so its CI, signing and release follow the standard pattern; whether it lives in its own repository or ships as assets of an existing release is for the plan to decide.
- Read TOML with a fallback reader on Python 3.10, where the standard one does not exist.

## Success Metrics

- The xcl-python CI matrix is fully green on the first run after this lands, on every platform and Python version.
- The number of Docker-gated skips in xcl-python's CI is zero on every platform.

## Non-Goals

- The engine program, the musl wheels and the Windows x86-64 wheel are out of scope here; the python-out-of-process-engine spec covers them.
- Adding Python-written plugins or a Python plugin SDK.
- Reworking the release workflows beyond the test plugin's release.
