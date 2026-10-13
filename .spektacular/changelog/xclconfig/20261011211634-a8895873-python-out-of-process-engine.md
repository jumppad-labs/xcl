---
created_date: "2026-10-13"
document_status: draft
project: xclconfig
spec: 20261011211634-a8895873-python-out-of-process-engine
plan: 20261011211634-a8895873-python-out-of-process-engine
---

# The engine as a standalone program, released without a C toolchain

xcl now ships its engine as `xcl-core`, a small static program that any language can start and drive over its standard input and output, with the same messages, results and errors as the shared library. Tagging a release builds it for Linux, macOS and Windows on x86-64 and arm64, with Alpine (musl) names too, in one job with no C compiler. The archives are attached under a signed checksums file. Go users are unaffected: the library still needs no cgo, and nothing it does changes.

> Derived from project xcl (jumppad-labs), spec/plan 20261011211634-a8895873-python-out-of-process-engine. See the project-level record for the full feature.

## What changed in this repo

- **The program.** `cmd/xcl-core` serves `coreservice` over stdin and stdout.
  - Requests are length-prefixed `Request` frames, and each is answered by a `Response` with the same id. Requests run concurrently, so a waiting `Poll` holds nothing up.
  - The method `Release` frees a handle.
  - Standard output is reserved for frames, and plugins get the null device as stdin.
  - Interrupts are discarded, including Ctrl-Break on Windows.
  - End of input or SIGTERM releases every configuration, saving state, and exits 0. A framing error exits 1.
  - `--version` prints the stamped version.
- **Protocol.** `proto/xcl/core/v1/transport.proto` holds the two frame messages, generated into `coreservice/proto`. The CoreService messages and the C ABI are unchanged.
- **Core service.** `coreservice.Service.Close()` releases every configuration it holds and waits for each to save its state.
- **Build.** `make xcl-core` builds the program with cgo disabled for any `GOOS`/`GOARCH`, stamped with `XCL_CORE_VERSION`.
- **Release.** `.github/workflows/release.yml` replaces `libxcl-release.yml`. One job, with cgo disabled and the C compilers hidden, builds, checks, archives, signs and attaches the eight `xcl-core_<version>_…` assets, then dispatches the xcl-python release.
- **Shared library.** `.github/workflows/libxcl-build.yml`, run by hand, builds the shared library for all eight platforms as workflow artifacts. libxcl is no longer attached to releases, and `go.yml` keeps building and smoke-testing it.
- **Go CI.** `go.yml` gained a Windows job that runs the program's pipe and Ctrl-Break tests.
- **Docs.** The CHANGELOG has an entry for this spec, and the README's "Other languages" note says Python runs the engine as a bundled program on every platform.

## Why

Python, and later other languages, could not load the Go shared library on musl, and the Windows x86-64 library crashed on load. A static program started as a subprocess works everywhere, and it cross-compiles for every target from one machine without a C toolchain.

## Deviations

- The program also discards SIGPIPE, so a vanished host cannot kill it before configurations are released.
- Every release binary's recorded build settings are checked for `CGO_ENABLED=0`.
- `libxcl-build.yml` takes a `version` input.
- Go CI gained the Windows job.
- The Windows Ctrl-Break test, the signed release, the dispatch and the manual libxcl run are outstanding until the code is pushed and v0.5.0 is tagged.
