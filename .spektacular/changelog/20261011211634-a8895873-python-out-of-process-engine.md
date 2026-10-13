---
created_date: "2026-10-13"
document_status: final
closed_date: "2026-10-13"
---

# Python runs the xcl engine as a bundled subprocess on every platform

## What was built

The xcl engine is now a small static program, `xcl-core`, that any language can start and talk to. The Python package bundles that program and runs it as a subprocess on all eight supported platforms, Alpine included, instead of loading a Go shared library into the Python process. The work spans three repos.

**xcl (the engine and its release)**
- `cmd/xcl-core` is a cgo-free program in the root module.
  - `xcl-core --serve` serves the existing core service over standard input and output. Frames are a 4-byte big-endian length followed by a `Request` or `Response` message from the new `proto/xcl/core/v1/transport.proto`.
  - Each request runs on its own goroutine, so a waiting `Poll` never holds up another call. The reserved method name `Release` frees a handle.
  - Negative statuses cross as two's-complement `uint32`.
  - Standard output carries only frames: external plugins get the null device as their standard input, and the program's own output goes to standard error.
  - Interrupts (Ctrl-C, and Ctrl-Break on Windows) are discarded so the host decides what to cancel.
  - End of input or SIGTERM releases every configuration, saving state, and exits 0.
  - `--version` prints the stamped version, and `make xcl-core` builds it for any platform.
- `coreservice.Service` gained the additive `Close`.
- Tagging a release now runs `.github/workflows/release.yml`, which replaces `libxcl-release.yml`. One job, with cgo disabled and every C compiler hidden behind failing shims, builds the eight `xcl-core_<version>_<os>_<arch>[_musl]` archives, then signs the checksums with the jumppad-labs key, attaches them and dispatches xcl-python.
- The shared library's toolchain matrix moved to the manual `libxcl-build.yml`. Go CI still builds and smoke-tests libxcl on every change, and gained a Windows job for the program's Ctrl-Break test.
- The changelog and README were updated.

**xcl-python (the package)**
- `SubprocessTransport` starts the bundled program on first use and checks its ABI. One reader thread matches responses to waiting calls, and calls wait in GIL-releasing slices.
- One engine is shared by every configuration. It stops on the last release, on `close()` or at interpreter exit, and dies with its host through end of input, plus a job object on Windows.
- When the engine dies, every waiting call raises the new `xcl.EngineExitedError` and the next use starts a fresh engine.
- In a forked child, a pre-fork configuration raises `ForkedProcessError` naming the way out.
- Transport selection (`XCL_TRANSPORT`) chooses the program by default and the source-built shared library with `inprocess`. A missing engine raises the new `xcl.EngineNotFoundError` naming both paths.
- Packaging:
  - wheels carry `xcl/_bin/xcl-core`;
  - `cffi` moved to an `inprocess` extra;
  - the version is now on the 0.5.0 line.
- Tests:
  - fake-engine unit tests for the transport;
  - selection and strict-skip unit tests;
  - integration tests for one engine per process, cleanup on close, exit and host kill, engine death, process-group interrupt, fork, version mismatch, the first-`Config()` timing and a missing program;
  - strict mode (`XCL_TEST_STRICT`, `XCL_TEST_ENGINES`) so CI can never silently skip an engine it built.
- CI runs the suite on the program everywhere, again on the shared library on Linux x86-64, and in Alpine with no C compiler.
- The release workflow builds the wheels around the verified `xcl-core` archives and runs parity strictly on each. Its install checks run a new apply-and-read script, the musl ones in plain `python:3-alpine`.
- The README describes the bundled engine.

**xcl-website**
- The home page gained an Install section with Go and Python tabs: `pip install xcl-lang`, the bundled-subprocess sentence, the eight platforms with Alpine and a PyPI link. The badge now reads v0.5.0.

## Why it matters

The first attempt to publish XCL for Python failed on two of eight platforms. musl refuses to `dlopen` a Go c-shared library, and the Windows x86-64 library crashed on load. Running the engine as a separate, statically linked program removes the dynamic loading entirely:
- Python developers on Alpine, and everywhere else, get the same API, behaviour and results.
- The release needs no C toolchain.
- The shared library stays buildable and tested as a source-build option, and can become the default again later without an API change.

## Deviations from the plan

- **Python handles.** The transport hands out its own configuration handles, each mapped to the engine that issued it. Engine handles restart at 1 in every new engine, so a configuration from a dead engine, or from the parent of a fork, could otherwise reach the wrong configuration.
- **xcl-core shutdown.** The program also discards SIGPIPE, so a vanished host becomes a write error and configurations are still released. The serve loop takes a context and closes the service twice around draining in-flight requests.
- **Windows CI.** Go CI gained an `xcl-core-windows` job, because Go CI only tested on Linux and the Windows Ctrl-Break smoke test would otherwise never run.
- **Release checks.** The release workflow also checks that every binary was built with cgo disabled (`go version -m`). `libxcl-build.yml` takes a `version` input.
- **Go on Alpine.** The Alpine CI job and the musl wheel script install Go from the official archive, because Alpine's `go` package pulls in gcc. The musl wheel script previously did not install Go at all, although `make plugins` needs it.
- **Unchecked acceptance criteria.** These are left unchecked until GitHub runs and the v0.5.0 release:
  - the Windows Ctrl-Break test;
  - signature verification with the real key;
  - the release dispatch;
  - the manual libxcl run;
  - the eight PyPI wheels and install checks.

  The human task "Cut the v0.5.0 release" is open, and the test plan lists every outstanding check.
- **Verified locally:**
  - root `go test ./... -race` and `make test-libxcl`;
  - the full Python suite on both routes;
  - the Alpine CI script in docker;
  - a glibc wheel in a fresh venv;
  - a musl wheel in plain `python:3-alpine` running the install check;
  - a dry run of the release build and signing with compiler shims;
  - the website build.

## Discoveries

- Cancelling an operation does not interrupt a provider call already running, on either route (engine/plugin behaviour, outside this spec).
- An engine killed with SIGKILL leaves its go-plugin plugin processes running. A killed host does not, because the engine sees end of input and releases its plugins.
- On Alpine, `apk add go` installs gcc and musl-dev.
- `go version -m` records `CGO_ENABLED` but not `-ldflags` for a `-trimpath` build.
