---
created_date: "2026-10-12"
document_status: final
closed_date: "2026-10-12"
---

# Plan: 20261011211634-a8895873-python-out-of-process-engine

<!-- Metadata -->
<!-- Created: 2026-10-12T07:44:22Z -->
<!-- Commit: e59ce2f (xcl); 26e578a (xcl-python) -->
<!-- Branch: f-libxcl -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git (with xcl-python and xcl-website) -->

## Overview

Python developers on Alpine could not use XCL: the first published Python release failed on musl and on Windows x86-64, because the engine shipped as a Go shared library loaded into the Python process. This plan ships the engine instead as a small bundled program, `xcl-core`, built without cgo for all eight platforms. Every Python package starts it and talks to it over pipes, with exactly the same API, behaviour and results everywhere. It removes the C toolchain from the release, keeps the shared library buildable and tested as a source-build option, and ends with v0.5.0, the first published Python release. It serves Python developers on every platform, Alpine included, and the people who cut XCL releases. Go users are unaffected.

## Conventions

- **Testing & mocking (testify `require`, no table-driven tests, positive and negative cases in separate functions, tests beside the code they test)** — every new Go test in `cmd/xcl-core` follows it, and the xcl-python tests follow the same rules in pytest: no `parametrize` tables, one positive or negative case per function.
- **Never write tests that inspect repository files** — the release workflows, the CHANGELOG entry, the READMEs and the xcl.dev page are checked by human review; no test reads them.
- **Project structure (`/cmd` for main applications; public packages at the module top level; protos under `proto/`)** — the engine program goes in `cmd/xcl-core`, and the frame messages go in `proto/xcl/core/v1/transport.proto` beside `core.proto`.
- **Dependencies (prefer the standard library; document and pin third-party deps)** — `xcl-core` adds no Go dependency, and the Python transport uses only the standard library (`subprocess`, `threading`, `struct`, `ctypes` for the Windows job object). Moving `cffi` to an extra is recorded with its reason in `pyproject.toml`.
- **Code style (gofmt/vet, `any` over `interface{}`, descriptive names)** — all new Go code.
- **Patterns & architecture (dependency injection; graceful shutdown for services)** — the serve loop takes its reader, writer and `*coreservice.Service` as parameters so tests drive it in memory. Shutdown on end of input or SIGTERM releases every configuration, so state is saved before exit. `SubprocessTransport` takes its program path and process launcher as parameters so unit tests run against a fake engine.
- **Development standards (structured logs)** — the engine writes its own diagnostics to standard error only. The protocol owns standard output.
- **Shared test helpers live in `internal/testutil`** — any helper the `cmd/xcl-core` tests share with other packages (for example building a fixture plugin) goes there. Helpers used only by `cmd/xcl-core` stay in its `_test.go` files.

Not applied:
- Database and external services, config parser options builder, hclsyntax references, entity/glossary naming, provider Changed/Update testing, graph-parent ordering, test state from a real apply: this work touches none of the parser, providers or state format. The parity suite already produces its state by applying.
- Shared errors package: no new Go error types are added. The new errors are Python exceptions.
- Never modify dependency packages: go-plugin's stdin behaviour is worked around in `xcl-core`, not patched.

## Architecture & Design Decisions

The work is built to the revised design `libxcl.md` (source `design`), sections "The transport: xcl-core over stdio", "Starting the engine", "Choosing the transport" and "Shipping and packaging". It adds a second transport under the unchanged `coreservice`. A static engine program, `cmd/xcl-core` in the xcl root module, serves `coreservice.Service` over its standard input and output. Each Python package bundles that program, starts it on first use and talks to it through a new `SubprocessTransport`. The client, the poll loop and the public API above the transport do not change. The release builds the program for all eight platforms in one `CGO_ENABLED=0` job. The shared library stays in the tree, CI-tested, and selectable from a source build with `XCL_TRANSPORT=inprocess`. The work spans three repos:
- **xcl**: the program, the frame messages, the build targets, the release workflows, the changelog and README.
- **xcl-python**: the transport, transport selection, the new errors, packaging, CI and release, and the README.
- **xcl-website**: a Python install section on the home page.

**Engine program (xcl).** `proto/xcl/core/v1/transport.proto` defines the two frame messages exactly as the design writes them: `Request{id, handle, method, payload}` and `Response{id, status, payload}`. buf generates them into `coreservice/proto`. They are the only addition to the protocol. Three choices shape the program:
- **Frame-level `Release`.** A `Request` whose method is `Release` frees its handle through `Service.Release`, so release crosses without a new `CoreService` method. This is required because the spec forbids changing the core service messages and the C ABI.
- **Negative statuses in `uint32`.** The design's `uint32 status` carries the negative statuses as their two's-complement value.
- **Stdio protection.** Before serving, the program keeps the original standard input and output for the protocol. It then points the `os.Stdin` variable at the null device, because go-plugin hands `os.Stdin` to every external plugin it starts. It also points `os.Stdout` at standard error, so nothing but frames can reach the protocol pipe.

The serve loop lives in the `main` package, not in `coreservice`, which stays transport-free as the design requires:
- one reader decodes frames;
- each request runs on its own goroutine, so a blocking `Poll` never holds up a `Log` or a `Cancel`;
- one mutex serialises writes;
- end of input, or `SIGTERM`, releases every live configuration and exits. Releasing waits for state to be saved. A small additive `Service.Close` does this.

`SIGINT`, and on Windows both `CTRL_C_EVENT` and `CTRL_BREAK_EVENT` (Go reports each as `os.Interrupt`), are caught and discarded, so the host decides. The program does not rely on `signal.Ignore`: on Windows an ignored console control event falls through to the default handler, which ends the process. `xcl-core --version` prints the stamped version, which `-X coreservice.Version` sets as it does for libxcl. Behaviour tests stay plain `go test` tests: the loop is tested against in-memory pipes, and a smoke test builds the real binary and drives it over pipes. This follows the convention that tests sit beside their code and that a smoke test builds and runs the real binary.

**Python transport (xcl-python).** `xcl/_transport/subprocess.py` adds `SubprocessTransport`, with the same `call`/`release` signature as `InProcessTransport`. It owns everything about the process:
- **Lazy start.** It starts `xcl/_bin/xcl-core[.exe] --serve` on the first call, with a pipe pair and standard error inherited. On Windows it puts the process in a job object with kill-on-close (through `ctypes`, no new dependency).
- **Handshake.** Its first exchange is `GetVersion`, checking `abi_version`. The client keeps checking the package version, as today.
- **Reader thread and waiters.** One reader thread matches responses to waiting callers by `id`. Callers wait on per-call events in short slices, so the GIL is released and Ctrl-C is still delivered promptly on Windows.
- **Handle tracking.** It records the handle of each successful `CreateConfig` response. The release of the last live handle, or interpreter exit (`atexit`), closes standard input and terminates the process with a bounded wait. An engine that exits on end of input is the backstop when the host is killed.
- **Engine death.** If the pipe closes, every waiter gets `xcl.EngineExitedError` with the exit status or signal. The next call starts a fresh engine. A handle from the dead engine raises `EngineExitedError` instead of an internal binding error.
- **Fork.** The transport records the pid it started under, and `os.register_at_fork` resets its state in a child. In the child, a handle created before the fork raises `xcl.ForkedProcessError`, naming the way out: create the configuration after the fork. A new configuration starts the child's own engine.

**Choosing the transport.** `default_client()` selects the transport from what the package holds and `XCL_TRANSPORT`, never by probing the machine:
- unset (or `subprocess`) selects the program;
- `inprocess` selects `xcl/_lib/libxcl.*`;
- any other value raises an error;
- an absent engine raises `xcl.EngineNotFoundError` naming both expected paths.

`cffi` moves to an optional `inprocess` extra, because the ctypes fallback already covers the source-build route. Each wheel carries the program in `xcl/_bin`, built by `make engine` in development, or downloaded and verified from the xcl release in the release workflow. The Python API is unchanged apart from the two exception classes the design names.

**Release and CI.** `.github/workflows/release.yml` replaces `libxcl-release.yml`:
- `settings` resolves the tag as today;
- one `build` job, with every C compiler hidden from `PATH` and `CGO_ENABLED=0`, builds the eight `xcl-core_<version>_<os>_<arch>[_musl]` archives. The two musl names carry the same static Linux binary;
- `package` signs the checksums with the jumppad-labs key, attaches the archives and dispatches xcl-python.

The libxcl toolchain matrix moves unchanged into a `workflow_dispatch`-only `libxcl-build.yml`. The existing `go.yml` `libxcl` job keeps building and smoke-testing the library on every change. On the xcl-python side:
- the release downloads `xcl-core` archives instead of libxcl;
- the existing hand-started Alpine pattern stays for musl;
- the install checks run an apply-and-read script, not only `Config().close()`;
- CI runs the whole suite on the engine program everywhere, and a second time with `XCL_TRANSPORT=inprocess` on Linux glibc x86-64.

Transport-specific tests are marked to run only on their route. Per the testing conventions, no test reads workflows, READMEs or the changelog. Rejected alternatives are listed in [research.md](./research.md#alternatives-considered-and-rejected):
- a `Release` RPC;
- the serve loop in `coreservice`;
- a separate module for the program;
- a second Python suite;
- handle counting from calls;
- a job container for the release build.

## Component Breakdown

**xcl**

- **Frame messages (new).** The two messages that wrap every call on the pipe: a request carrying a host-chosen id, a handle, a method name and the serialised CoreService request, and a response carrying the same id, a status and the serialised response or `Error`. They sit beside the CoreService protocol, are generated with it, and are covered by `buf lint` and `buf breaking`. They also reserve the method name `Release` for freeing a handle. Both the engine program and the Python transport speak them.
- **Engine program `xcl-core` (new).** A static, cgo-free program in the root module. It owns the process side of the transport:
  - command-line handling: `--serve` and `--version`;
  - protecting standard input and output from everything but frames, including the external plugins it starts;
  - the serve loop: concurrent dispatch to `coreservice.Service`, serialised writes;
  - signal policy: interrupt (Ctrl-C, and Ctrl-Break on Windows) caught and discarded, terminate handled;
  - shutdown that releases every configuration before exit.

  It holds no binding logic; `coreservice` does all of that.
- **Core service (changed, additively).** Gains one method to release every configuration it holds, waiting for each to save its state. The engine program uses it at shutdown. Everything else, including the C ABI path, is unchanged.
- **Build targets (changed).** The `Makefile` gains a target that builds the engine program for any `GOOS`/`GOARCH` with cgo disabled, stamped with a version exactly as the library target stamps it. The library targets stay.
- **Release workflow (replaced).**
  - On a version tag, one cgo-free job with no C compiler on the path builds the eight platform archives of the engine program.
  - The workflow then writes and signs the checksums, attaches everything to the GitHub release and dispatches the xcl-python release.
  - Run by hand with publishing off, it builds and keeps the assets as a workflow artifact, as today.
- **Shared-library build workflow (new, manual only).** The former release matrix (zig, llvm-mingw, macOS runners), triggered by hand. It builds the library for every platform as workflow artifacts and never attaches them to a release.
- **Go CI (unchanged in shape).** It keeps building and smoke-testing the library on every change. It now also builds and tests the engine program, through the existing root-module jobs.
- **Changelog and README (changed).** The xcl changelog gets an entry for this spec. The README's "Other languages" note says the engine reaches Python as a bundled program.

**xcl-python**

- **Subprocess transport (new).** The second implementation of the existing transport protocol. It owns:
  - the engine process: lazy start, Windows job object, version and ABI handshake;
  - framing, and the reader thread that matches responses to waiting callers;
  - tracking the live configuration handles of the current engine;
  - stopping the engine when the last handle is released or the interpreter exits;
  - reporting engine death to every waiter, and restarting on next use;
  - fork detection.

  Nothing above it knows a process exists.
- **In-process transport (kept).** Unchanged in behaviour, now used only when selected explicitly. Its fork guard keeps its existing message.
- **Transport selection (new).** The single place that decides which transport the default client uses. It looks only at what the installed package holds and at `XCL_TRANSPORT`, and raises the engine-not-found error naming both expected locations when the chosen engine is absent.
- **Typed client (changed minimally).** Built over whichever transport selection returns. It still checks the package version on first use, and still maps statuses and error codes to exceptions.
- **Errors (changed).**
  - Two new exception classes from the design: engine-not-found and engine-exited, the latter carrying the exit status or signal.
  - The fork error gains the subprocess route's message, which names creating the configuration after the fork.
- **Generated messages (changed).** The frame messages are generated next to the CoreService messages, from the same xcl checkout.
- **Packaging (changed).**
  - The wheel carries the engine program for its platform.
  - The development build places the program from the xcl checkout; the library stays a separate, optional development target.
  - `cffi` becomes an optional extra.
  - The version moves to the 0.5.0 line.
- **CI workflow (changed).**
  - Every platform job builds the engine program instead of the library and runs the suite on it.
  - Linux glibc x86-64 also builds the library and runs the same suite with the in-process override.
  - The Alpine jobs keep the hand-started container pattern.
- **Release workflow (changed).**
  - Wheels take the verified engine-program archive from the xcl release.
  - The parity suite runs against each installed wheel.
  - The install checks run an apply-and-read program and report the engine version on all eight platforms, the two musl ones in a plain Alpine image.
- **Test suite (changed).**
  - The engine fixtures follow the selected transport.
  - Transport-specific tests run only on their route.
  - New unit tests drive the subprocess transport against a scripted fake engine.
  - New integration tests cover the lifecycle, engine death, interrupt and fork against the real program.
- **README (changed).** It describes the engine as a bundled subprocess with the same API everywhere, lists Alpine among the platforms, and rewrites the fork section for the subprocess route.

**xcl-website**

- **Home page (changed).** Gains an install section with Go and Python tabs. The Python tab says the package bundles the engine as a subprocess on every platform with the same API, and lists the eight platforms, Alpine included. The hero badge and description move to v0.5.0 and Go and Python.

## Data Structures & Interfaces

**Frame messages (xcl, `xcl.core.v1`, new file beside `core.proto`).** These are the only addition to the protocol, exactly as the design writes them. Each frame on the pipe is a 4-byte big-endian length followed by one of these messages.

```protobuf
// Request is one call from the host. id is chosen by the host and echoed
// back; handle, method and payload mean what xcl_call's arguments mean.
// The method "Release" frees handle, as xcl_release does, with an empty payload.
message Request  { uint64 id = 1; uint64 handle = 2; string method = 3; bytes payload = 4; }

// Response answers the Request with the same id. status is xcl_call's status;
// the negative statuses cross as their two's-complement uint32 value.
message Response { uint64 id = 1; uint32 status = 2; bytes payload = 3; }
```

Responses may arrive in any order. The status codes and payload contents are the C ABI's, unchanged, and the ABI version (1) covers the frames too.

**Core service, one additive method (xcl, Go).**

```go
// Close releases every configuration the Service holds, cancelling each
// operation in progress and waiting for its state to be saved. Used by the
// engine program when its input ends or it is asked to terminate.
func (s *Service) Close()
```

**Engine program contract (xcl).**

```
xcl-core --serve      serve CoreService over stdin/stdout until stdin ends or SIGTERM
xcl-core --version    print the stamped version (e.g. v0.5.0) and exit 0
xcl-core              usage on stderr, exit 2
```

Its exit statuses:
- 0 after end of input or `SIGTERM`, once every configuration is released;
- 1 on a framing error, such as a truncated frame or an undecodable `Request`, with a message on stderr.

Standard error carries the engine's own diagnostics. Standard output carries only frames.

**Serve loop (xcl, unexported, in the engine program).**

```go
// serve reads Request frames from in until it ends, dispatches each on its
// own goroutine to service, and writes Response frames to out, one at a time.
// It releases every configuration (service.Close) before returning.
func serve(in io.Reader, out io.Writer, service *coreservice.Service) error
```

**Release asset contract (xcl release).** The registry's naming, with the `_musl` arch suffix the libxcl workflow introduced:

```
xcl-core_<version>_<os>_<arch>[_musl].tar.gz   (xcl-core at the archive root)
xcl-core_<version>_windows_<arch>.zip          (xcl-core.exe at the archive root)
xcl-core_<version>_checksums.txt               sha256sum format, all eight archives
xcl-core_<version>_checksums.txt.sig           armoured detached signature, jumppad-labs key
```

`<version>` is the tag without the `v`. The targets are `linux_{amd64,arm64}`, `linux_{amd64,arm64}_musl` (the same static binary under a second name), `darwin_{amd64,arm64}` and `windows_{amd64,arm64}`.

**Python transport protocol (xcl-python, unchanged).** `Transport.call(handle, method, request) -> (status, bytes)` and `Transport.release(handle) -> status`. Both transports implement it, and the client and everything above see only this.

**Subprocess transport (xcl-python, new).**

```python
class SubprocessTransport:
    def __init__(self, program: Path | None = None, *, launcher: Callable[..., Popen] | None = None) -> None: ...
    def call(self, handle: int, method: str, request: bytes) -> tuple[int, bytes]: ...
    def release(self, handle: int) -> int: ...
    def close(self) -> None: ...              # stop the engine now; also registered with atexit
    @property
    def pid(self) -> int | None: ...          # the running engine's pid, for diagnostics and tests
```

**Transport selection (xcl-python, new).**

```python
TRANSPORT_ENV = "XCL_TRANSPORT"              # unset or "subprocess": the program; "inprocess": the library
def program_path() -> Path: ...              # xcl/_bin/xcl-core, or xcl-core.exe on Windows
def select_transport() -> Transport: ...     # raises EngineNotFoundError, or ValueError for an unknown value
```

**New exceptions (xcl-python, exported from `xcl`).** Both subclass `xcl.Error`, so `code`, `details` and `causes` work as for every other error.

```python
class EngineNotFoundError(Error):   # code "engine_not_found"; details: program, library, transport
class EngineExitedError(Error):     # code "engine_exited"; details: status or signal; .returncode
```

`ForkedProcessError` keeps its class and code. The subprocess transport passes it a message naming the way out: create the configuration in the child after the fork.

**Wheel layout (xcl-python).** `xcl/_bin/xcl-core` (or `xcl-core.exe`) is present in every wheel, executable. `xcl/_lib/libxcl.*` is present only in a source build that ran the library target.

## Implementation Detail

**A second thin transport over one core service.** The pattern libxcl set is followed exactly. libxcl is a `main` package that moves bytes between C and `Service.Call`. The engine program is a `main` package that moves bytes between frames and `Service.Call`. Neither holds binding logic. A reader of the engine program sees four parts:
- the flags;
- the stdio takeover: the protocol's reader and writer are captured first, then `os.Stdin` and `os.Stdout` are pointed away;
- signal setup;
- one call into the serve loop.

The serve loop is the only new concurrency shape on the Go side:
- one goroutine reads frames;
- a goroutine per request calls the service;
- a mutex guards the writer;
- a wait group lets shutdown drain in-flight requests before releasing configurations.

Behaviour stays tested once, in `coreservice`. The program's own tests cover only what it adds: framing, out-of-order answers, `Release`, shutdown, the stdio takeover and signals.

**Process management lives entirely in the Python transport class.** The design's layering (public API → typed client → byte transport) is kept. The client gains no knowledge of processes, and the public API is untouched. Inside the transport:
- **State.** A small state object holds the running engine: process, pipes, reader thread, waiting calls by id, live handles, the pid it was started under, and a generation number.
- **Start and stop under one lock.** Start and stop are serialised by one lock, and calls take a snapshot of the current engine.
- **Death notice.** The reader thread is the only consumer of the engine's output. When it sees end of output it reaps the process and fails every waiter with the exit status. It then clears the state, so the next call starts a new generation.
- **Fork.** A fork hook in the child discards the parent's state without touching the parent's process, and resets the lock. The parent's process and pipes stay the parent's. Handles from before the fork are remembered only so they can be refused by name.
- **Windows.** The job object and termination use `ctypes` calls. They are confined to two small helpers, so the POSIX path reads plainly.

**Selection replaces a hard-wired constructor.** The default client used to construct the in-process transport directly. It now asks one selection function. That function looks at the package directory and one environment variable, and is the only place either transport is chosen. The in-process transport keeps its fork guard and loader seam, and loses nothing. This is the hook the design's "way back in process" relies on: changing the default is one line in selection.

**Fake engine for unit tests.** The transport's unit tests use a scripted fake engine: a short Python program in the test tree that speaks the frame protocol. It can delay, reorder, exit with a status, or die by signal on command. The tests can then cover framing, out-of-order matching, death, restart, handle tracking and fork without Go. This mirrors how the in-process transport is tested through a fake loader. Integration tests then repeat the lifecycle guarantees against the real program.

**One suite, two routes.** The engine fixtures in the test tree stop assuming a library path. They ask selection what engine the run will use. When that engine is missing, they skip with a message naming the missing build target, through one engine-skip helper.

That helper is what CI's strict mode builds on:
- `XCL_TEST_ENGINES` lists the routes a job provides: `subprocess` on every job, plus `inprocess` on the Linux job that builds the library.
- Under `XCL_TEST_STRICT=1`, a missing engine for a route in that list fails the run instead of skipping.
- A missing engine for a route the job does not build stays a skip.
- Tests skipped because they belong to the other route are never strict failures.

The sibling release-hardening plan's `strict_skip` helper, implemented after this one, takes engine skips through this helper. Two markers keep route-specific tests honest: one for the subprocess route and one for the in-process route. The ctypes fallback and the Go-runtime fork guard are in-process tests. Lifecycle, engine death and child-process fork are subprocess tests. Everything else runs on both.

**Workflows keep their shape.** The xcl release keeps the existing `settings → build → package/sign/attach → dispatch` jobs, with the build collapsed from four jobs to one. The library's matrix is moved verbatim behind a manual trigger. The xcl-python release keeps its per-platform wheel jobs and hand-started Alpine jobs. Only the archive it downloads and the directory it fills change. Its install checks run a small apply-and-read program rather than a bare import.

## Dependencies

**Design documents this plan was built on**
- `libxcl.md` from the `design` design source (revised 2026-10-12) is the settled shape this plan implements:
  - the `xcl-core` stdio transport: frames, process lifecycle, interrupts, cost;
  - transport selection with `XCL_TRANSPORT=inprocess`;
  - `EngineNotFoundError`, `EngineExitedError` and `ForkedProcessError` on the subprocess route;
  - the release shape: one `CGO_ENABLED=0` job, `xcl-core_<version>_…` assets signed with the registry key, and the library matrix in a manual workflow.

**Planning dependencies**
- **Spec `20261011130709-aa3eb933-python-support` must have landed (it has).** It provides `coreservice`, the C ABI and `bindings/libxcl`, the xcl-python package with its transport seam, the CI and release workflows, the fixture plugins and the parity suite that this plan extends. It needs no changes beyond those planned here.
- **Sibling spec `20261011232013-da18c586-python-release-hardening`, planned in parallel.** It owns CI fixes (tomllib on 3.10, `XCL_SRC` fixture lookup), the signed test plugin replacing the Docker plugin, and the single recorded xcl ref. Both plans edit xcl-python's `ci.yml`, `release.yml`, `musl-wheel.sh` and the test fixtures (`tests/integration/conftest.py`, `tests/parity/conftest.py`), so they must be ordered. No functional dependency runs either way.
- **xcl must carry this plan's xcl changes on the ref xcl-python's CI builds from** (`xcl-ref.txt`, today `f-libxcl`). xcl-python's CI and wheel builds compile `cmd/xcl-core` and generate `transport.proto` from that checkout. The xcl tasks therefore come before the xcl-python tasks, and the xcl work must be pushed before xcl-python's CI can go green.
- **Docs specs in this epic** (`using-xcl-with-go`, and any revision of the language reference) come after this plan and inherit the home page's install section.

**Runtime and build dependencies**
- `github.com/jumppad-labs/xcl/coreservice` and its generated `coreservice/proto`: served unchanged by the engine program. It gains an additive `Close`, and the frame messages are generated into the same package.
- `google.golang.org/protobuf` (already required by the root module) encodes the frames. No new Go dependency.
- `github.com/hashicorp/go-plugin` v1.6.3 (already required). It passes `os.Stdin` to every plugin it starts, which the engine program must neutralise. It needs no change and is not modified.
- `buf` (CI, `make core-protos`) generates the frame messages, and `buf lint` and `buf breaking` cover them.
- Python `protobuf>=5.26` (already required) carries the CoreService and frame messages. It must install from a wheel on musllinux x86-64 and aarch64 so a plain Alpine install needs no compiler.
- Python standard library only for the transport (`subprocess`, `threading`, `struct`, `atexit`, `os.register_at_fork`, `ctypes` for the Windows job object).
- `cffi` moves from required to an optional `inprocess` extra. `ctypes` remains the fallback.
- `protoc` 28.x in xcl-python CI generates `core_pb2` and now `transport_pb2` (`make protos`).
- GitHub Actions secrets already in place on xcl, reused unchanged: `XCL_RELEASE_GPG_KEY`, `XCL_RELEASE_GPG_PASSPHRASE` and `XCL_PYTHON_DISPATCH_TOKEN`. The PyPI trusted publisher for `xcl-lang` (environment `pypi`, workflow `release.yml`) is also already in place.
- GitHub runners `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-15-intel`, `macos-latest`, `windows-latest` and `windows-11-arm`, and the `python:3-alpine` image, all as used today.

## Testing Approach

**Kinds of test**

- **Go unit tests for the engine program (xcl, no cgo).** The serve loop runs against in-memory pipes and a real `coreservice.Service`, through a small frame-writing test client. The tests cover:
  - a request answered with the same id;
  - responses arriving out of order: a `Log` or `Cancel` answered while a `Poll` blocks;
  - `Release` freeing a handle, and a released handle then refused with the unknown-handle status;
  - negative statuses crossing and decoding back to their values;
  - end of input releasing every configuration and saving state;
  - a truncated frame ending the loop with an error.

  `Service.Close` gets its own coreservice test, with state saved from a real apply. These follow the conventions: testify `require`, one behaviour per function, positive and negative cases apart, beside the code.
- **Engine program smoke tests (xcl, Go, real binary).** Build `xcl-core` with a stamped version, start it over pipes and check:
  - `--version` prints the stamp;
  - `GetVersion`, `CreateConfig`, a validate through the poll loop, and `Release` all work;
  - closing standard input makes it exit 0;
  - an interrupt does not stop it: `SIGINT` on POSIX, and on Windows `CTRL_BREAK_EVENT` sent to a new process group (`CREATE_NEW_PROCESS_GROUP`), the way both plans test interrupts on Windows;
  - `SIGTERM` stops it cleanly (POSIX);
  - a configuration loading an external fixture plugin still answers frames, which proves the plugin did not take standard input.

  This follows the convention that a smoke test builds and runs the real binary.
- **libxcl smoke test (unchanged).** It keeps running in Go CI on every change and proves the C ABI still works.
- **Python unit tests (xcl-python, no Go).** `SubprocessTransport` runs against a scripted fake engine, a Python program speaking the frame protocol. The tests cover:
  - framing round trips;
  - out-of-order matching;
  - lazy start, and one engine for several configurations;
  - the engine stopping after the last release, and at `close`;
  - every waiter getting `EngineExitedError` with the status when the fake exits or is killed;
  - a restart on the next call, and a stale handle refused with `EngineExitedError`;
  - the ABI handshake refusing a different ABI version;
  - a call in a forked child on a pre-fork handle raising `ForkedProcessError` naming the way out, while a new handle in the child starts a new engine.

  Selection gets its own tests: the default route, the override, an unknown override value, and the missing-engine error naming both paths. Style mirrors the Go conventions: no `parametrize` tables, and positive and negative cases in separate functions.
- **Python integration tests (against the real engine program).** One or more per acceptance criterion that can run in CI:
  - three configurations share one engine process;
  - closing the last configuration, or interpreter exit, leaves no engine;
  - `SIGKILL` of the host leaves no engine within a few seconds (POSIX), and terminating the host leaves none on Windows through the job object;
  - killing the engine mid-apply raises `EngineExitedError` carrying the signal, the host continues, and a new configuration works;
  - an interrupt sent to the process group mid-apply ends with the cancellation error, keeps completed entities in state and leaves no engine;
  - a forked child applies its own configuration, and the parent's configuration raises the fork error;
  - an engine of a different version is refused naming both versions;
  - the first `Config()` in a fresh interpreter completes within one second;
  - with the program removed from a copy of the package, the first use names both expected paths.

  The existing suite (sync, asyncio cancellation, threads not stalled, events, custom stores, Python functions) runs unchanged on the engine program. That is the "same guarantees" check.
- **Both routes from one checkout.** The whole suite runs on the engine program on every CI platform. It runs a second time with `XCL_TRANSPORT=inprocess` on Linux glibc x86-64 with both engines built. Route-specific tests carry a marker and run only on their route.
- **Strict mode.** Every CI and release test step this plan touches sets `XCL_TEST_STRICT=1` and `XCL_TEST_ENGINES` to the routes the job builds. A job that should have built an engine fails rather than silently skipping, while a route the job does not build still skips.
- **Parity (cross-language contract).** The existing Go–Python parity fixtures run against each installed wheel in the release, on all eight platforms, and gate publishing.
- **Release checks (workflow, not unit tests).**
  - The xcl release asserts that no C compiler is reachable, that there are eight archives, and that the signature verifies.
  - The xcl-python release installs `xcl-lang==<version>` from PyPI on all eight platforms with no Go or C compiler on the path. The two musl platforms use a plain `python:3-alpine` image.
  - The install check then runs a program that registers a class, applies a configuration, reads the entity back and closes, and asserts that the engine reports the tag.

**Most coverage** goes to the Python subprocess transport. Every lifecycle guarantee in the spec lives there: start, sharing, stop, death, restart, interrupt and fork. Fake-engine unit tests cover the edge cases deterministically, and integration tests prove them against the real program.

**What the tests guarantee**
- Every CoreService call behaves identically over the engine program and the library: the same suite passes on both.
- One engine per process, never orphaned: not after close, not at exit, not after the host is killed.
- Engine failure is a typed, catchable error with the exit status. The host survives, and the next use recovers.
- Ctrl-C cancels through the package and keeps saved state. A forked child gets either a working engine of its own or a named error.
- Standard output of the engine carries only frames, even with external plugins running.

**Deliberate gaps**
- No test reads the workflows, READMEs, the changelog or the website page (convention). They are reviewed by a person.
- The host-kill and process-group interrupt tests in this plan use POSIX signals on Linux and macOS. On Windows the job object is exercised by terminating the host process.
- Windows interrupt coverage of the Python package is the sibling release-hardening plan's keyboard-interrupt test. It starts the child with `CREATE_NEW_PROCESS_GROUP` and sends `CTRL_BREAK_EVENT`, and the child maps SIGBREAK to KeyboardInterrupt. The engine shares the child's process group, so it receives the same Ctrl-Break and must survive it, which this plan's engine smoke test proves on Windows.
- No Authenticode signing test: no certificate is available.
- The sibling release-hardening spec owns the Docker-free plugin tests and the 3.10 fixes. This plan does not add or change Docker-gated tests.

**Success metrics**
- *xcl-lang 0.5.0 installs and runs the README's getting-started example on all eight platforms, `python:3-alpine` x86-64 and arm64 included, on release day.*
  - A **behavioural test** in the release's install check runs a self-contained apply-and-read program from PyPI on all eight platforms, the two musl ones in plain Alpine.
  - Running the README's getting-started example itself, which needs the Docker plugin and a Docker engine, is **Manual — captured in the implementation test plan**.
- *Zero Python–Go parity differences on every wheel on every release.* A **behavioural test**: the parity suite runs against each installed wheel in the release workflow and blocks publishing on any difference.
- *A first `Config()` completes within one second on the CI runners.* A **behavioural test**: an integration test times the first `Config()` in a fresh interpreter and asserts it finishes under one second. It runs on every CI platform.
- *The release workflow's wall time drops against the v0.4.0 run.* **Manual — captured in the implementation test plan** (compare the v0.5.0 run's duration with run 38172909823).
- *No open issue reports an orphaned engine process after a Python program exits.* **Manual — captured in the implementation test plan** (issue tracker review after release). The orphan guarantees themselves are behavioural tests above.

**Manual reviews**
- **Manual — captured in the implementation test plan:** review the xcl CHANGELOG entry for this spec. It should cover the engine program, the frame messages, the release assets, the library moved to a manual workflow, and **Breaking:** none for Go users.
- **Manual — captured in the implementation test plan:** review the xcl-python README on the PyPI page of 0.5.0. It should say the engine runs as a bundled subprocess with the same API on every platform, and list Alpine.
- **Manual — captured in the implementation test plan:** review the xcl.dev home page install section. It should show the Python tab saying the same, list Alpine, and show the badge at v0.5.0.
- **Manual — captured in the implementation test plan:** watch the v0.5.0 release end to end:
  - eight `xcl-core` archives and a signed checksums file attached to the xcl release, with no libxcl archive;
  - the build job log showing no C compiler;
  - eight wheels and an sdist on PyPI;
  - every install check green.
- **Manual — captured in the implementation test plan:** run the manual libxcl build workflow once and confirm it still builds all eight libraries.

## Milestones & Tasks

### Milestone 1: The xcl engine runs as a standalone program any language can drive

**What changes**
- xcl gains `xcl-core`, a small static program built without cgo for every platform. It serves the existing core service over its standard input and output, so a host in any language can drive the engine by starting it and exchanging framed messages. No library has to be loaded into the host.
- The program shares the shared library's behaviour, statuses and messages. It exits when its host goes away, and it ignores Ctrl-C so the host decides what to cancel.
- The shared library keeps building and passing its smoke test.
- Go users see nothing new beyond one additive method on the core service. Importing the library still pulls in no cgo and no subprocess.

**Validation point**:
- The root module's tests pass with the race detector, including the engine program's serve-loop and smoke tests.
- The `CGO_ENABLED=0` cross builds for darwin, linux and windows now include the program.
- `buf lint` passes with the frame messages.
- The libxcl CI job still builds and smoke-tests the library.

#### - [x] Task: The frame messages are part of the protocol
**Id:** 1c4a2ab7-d1eb-4fff-a934-58d965d8ead7
**Repo:** xcl
**Depends on:** none
**Execution:** agent

This adds the two messages that carry every call between a host and the engine program: a request with an id, a handle, a method name and the serialised CoreService request, and a response with the same id, a status and the serialised reply. They sit beside the CoreService protocol, are generated with it, and are covered by the same lint and breaking-change checks. The method name `Release` is reserved for freeing a handle, so no CoreService method is added.

*Technical detail:* [context.md#task-the-frame-messages-are-part-of-the-protocol](./context.md#task-the-frame-messages-are-part-of-the-protocol)

**Acceptance criteria**:
- [x] The request and response messages exist exactly as the design writes them, and Go code for them is generated beside the CoreService messages.
- [x] The protocol's lint check passes, and the CoreService messages and methods are unchanged.
- [x] The messages' documentation says how `Release` and negative statuses are carried.

#### - [x] Task: The engine program serves the core service over standard input and output
**Id:** 46cf5eda-9cd3-450a-87e6-267679e0146f
**Repo:** xcl
**Depends on:**
- 1c4a2ab7-d1eb-4fff-a934-58d965d8ead7 — The frame messages are part of the protocol
**Execution:** agent

This adds `xcl-core`, a cgo-free program in the root module. It reads framed requests on standard input, answers each one on its own goroutine through the core service, and writes framed responses on standard output, so a slow poll never holds up another call. The program:
- keeps standard input and output for frames alone, including against the external plugins it starts;
- ignores Ctrl-C, and Ctrl-Break on Windows;
- on end of input or a terminate signal, releases every configuration so state is saved, and exits.

The core service gains one additive method that releases everything it holds, and the build gains a target that builds the program for any platform with a stamped version.

*Technical detail:* [context.md#task-the-engine-program-serves-the-core-service-over-standard-input-and-output](./context.md#task-the-engine-program-serves-the-core-service-over-standard-input-and-output)

**Acceptance criteria**:
- [x] A host that writes framed requests gets each answered with the same id. A `Log` or `Cancel` sent while a `Poll` waits is answered before the `Poll` returns.
- [x] `Release` frees a configuration. A later call on that handle gets the unknown-handle status, and every negative status decodes back to its value.
- [x] Closing the program's standard input makes it release every configuration, with an apply's state saved, and exit with status 0.
- [ ] An interrupt does not stop the program: `SIGINT` on Linux and macOS, and Ctrl-Break sent to its process group on Windows. A terminate signal stops it cleanly.
- [x] A configuration that starts an external plugin keeps getting answers. Nothing but frames ever appears on standard output.
- [x] `xcl-core --version` prints the version it was built with, and a build with cgo disabled succeeds for every one of the eight targets.
- [x] Importing the root module still starts no process and needs no C compiler.

### Milestone 2: Python runs the engine as a bundled subprocess, the same way on every platform

**What changes**
- The Python package starts the bundled engine program on first use and talks to it, instead of loading the shared library. Python developers on Alpine, and everywhere else, get the same API and the same results as before.
- One engine serves every configuration in the process. It stops when the last configuration closes, when the interpreter exits, or when the host is killed.
- If the engine dies, every waiting call raises an error carrying its exit status, and the next use starts a fresh engine.
- Ctrl-C cancels an operation and keeps saved state. A forked child can create its own configuration.
- A developer building from source can still build the shared library and select it with `XCL_TRANSPORT=inprocess`. If neither engine is present, the first use names where both were expected.
- The package's CI runs the whole suite on the engine program on every platform, and on the library too on Linux.

**Validation point**:
- xcl-python's CI is green on every runner and Python version, both Alpine jobs included, with the engine program.
- On Linux glibc x86-64 the same suite passes again with `XCL_TRANSPORT=inprocess`.
- The parity fixtures show no differences on either route.

#### - [x] Task: Python talks to the engine program through a subprocess transport
**Id:** bb7eb51b-a035-41dc-a3f6-3b8108daef3e
**Repo:** xcl-python
**Depends on:**
- 1c4a2ab7-d1eb-4fff-a934-58d965d8ead7 — The frame messages are part of the protocol
- 46cf5eda-9cd3-450a-87e6-267679e0146f — The engine program serves the core service over standard input and output
**Execution:** agent

This adds the subprocess transport, a second implementation of the package's existing transport protocol. It starts the bundled engine program on first use and checks its version and ABI. One reader thread matches responses to waiting calls, so other threads keep running. It owns the process lifecycle:
- one engine shared by every configuration;
- the engine stopped when the last one closes or the interpreter exits;
- on Windows, a job object so the engine dies with its host;
- an error carrying the exit status when the engine dies, and a fresh engine on next use;
- a named error for a configuration used across a fork.

The package gains the two errors the design names, and its development build gains a target that builds the program from the xcl checkout.

*Technical detail:* [context.md#task-python-talks-to-the-engine-program-through-a-subprocess-transport](./context.md#task-python-talks-to-the-engine-program-through-a-subprocess-transport)

**Acceptance criteria**:
- [x] Calls made from several threads at once each get their own response, even when the engine answers out of order.
- [x] The engine starts on the first call, not at import. Several configurations share one engine, which stops once the last configuration is released or the transport is closed.
- [x] When the engine exits or is killed, every waiting call raises `xcl.EngineExitedError` carrying the exit status or signal. The next call starts a new engine, and a configuration from the dead engine raises `EngineExitedError`, not an internal error.
- [x] An engine reporting a different ABI version is refused, naming both versions.
- [x] In a forked child, a configuration created before the fork raises `xcl.ForkedProcessError` naming the way out, while a new configuration starts the child's own engine.
- [x] The engine's standard error reaches the host's standard error.

#### - [x] Task: The package runs the bundled engine program by default and the shared library on request
**Id:** a08d3d86-6310-4e19-beeb-2992a7b0c11c
**Repo:** xcl-python
**Depends on:**
- bb7eb51b-a035-41dc-a3f6-3b8108daef3e — Python talks to the engine program through a subprocess transport
**Execution:** agent

The default client now chooses its transport from what the installed package holds and one override, `XCL_TRANSPORT`:
- unset, the bundled engine program;
- `inprocess`, the shared library from a source build;
- an engine that is absent, `xcl.EngineNotFoundError`, naming where both forms were expected.

Each wheel carries the program, `cffi` becomes an optional extra for the in-process route, and the version moves to the 0.5.0 line. The test fixtures follow the selected route, so the whole suite runs on either engine. New integration tests prove the lifecycle, engine-death, interrupt and fork guarantees against the real program.

*Technical detail:* [context.md#task-the-package-runs-the-bundled-engine-program-by-default-and-the-shared-library-on-request](./context.md#task-the-package-runs-the-bundled-engine-program-by-default-and-the-shared-library-on-request)

**Acceptance criteria**:
- [x] Without `XCL_TRANSPORT` the package runs the engine program even when the shared library is also present. With `XCL_TRANSPORT=inprocess` it loads the library.
- [x] With the engine program removed from an installed package, the first use raises `xcl.EngineNotFoundError` naming the program's and the library's expected locations.
- [x] Creating three configurations starts one engine process. Closing the last one, or the interpreter exiting, leaves none running. Killing the host with no chance to clean up leaves none running within a few seconds.
- [x] Killing the engine during an apply raises `EngineExitedError` with the signal or status. The host carries on, and a new configuration created afterwards applies.
- [x] An interrupt sent to the program's process group mid-apply ends the apply with the cancellation error. The entities completed before it are in state, and no engine is left running.
- [x] A child forked after the parent used XCL applies a configuration it created itself. Using the parent's configuration in the child raises the fork error naming that alternative.
- [x] An engine of a different version is refused naming both versions, and a first `Config()` in a fresh interpreter completes within one second.
- [x] The full existing suite (asyncio cancellation, threads not stalled, events, custom state stores, Python functions, parity) passes on the engine program. With both engines built and `XCL_TRANSPORT=inprocess`, it passes on the shared library.
- [x] A wheel built from the checkout carries the engine program for its platform, executable, and installing it needs neither Go nor a C compiler.
- [x] A missing engine skips its tests, unless `XCL_TEST_STRICT=1` is set and the engine's route is listed in `XCL_TEST_ENGINES`, in which case the run fails. A test that belongs to the other route always skips.

#### - [x] Task: CI tests the package on the engine program everywhere and on the shared library on Linux
**Id:** a7327c0c-1ee8-4248-bf1d-e8427d4b6862
**Repo:** xcl-python
**Depends on:**
- a08d3d86-6310-4e19-beeb-2992a7b0c11c — The package runs the bundled engine program by default and the shared library on request
**Execution:** agent

The CI workflow builds the engine program from the recorded xcl checkout on every runner and Python version, and runs the suite on it. The Alpine jobs keep starting their container by hand and now need no C compiler for the engine. The Linux glibc x86-64 job also builds the shared library and runs the same suite a second time with the in-process override, so the in-process route stays tested.

*Technical detail:* [context.md#task-ci-tests-the-package-on-the-engine-program-everywhere-and-on-the-shared-library-on-linux](./context.md#task-ci-tests-the-package-on-the-engine-program-everywhere-and-on-the-shared-library-on-linux)

**Acceptance criteria**:
- [x] Every runner and Python version in the matrix, and both Alpine jobs, run the suite on the engine program.
- [x] The Linux glibc x86-64 job runs the suite a second time on the shared library with `XCL_TRANSPORT=inprocess`.
- [x] Neither Alpine job installs a C compiler to build the engine.
- [x] Every test run sets `XCL_TEST_STRICT=1`, with `XCL_TEST_ENGINES` naming `subprocess` on every job and also `inprocess` on the Linux job's in-process run. An engine the job should have built can never be silently skipped.

### Milestone 3: Every XCL release publishes the engine program and eight Python wheels without a C toolchain

**What changes**
- Tagging an xcl release builds the engine program for all eight platforms in one job with no C compiler, and attaches the archives under a signed checksums file. No shared-library archive is attached; the library's toolchain matrix becomes a workflow run by hand.
- The Python release builds a wheel per platform carrying that platform's verified engine program, runs the parity suite against each wheel, and publishes eight wheels and an sdist to PyPI at the tag's version.
- It then installs from PyPI on all eight platforms, the two musl ones in a plain Alpine image, and runs a small apply-and-read program.
- The package README, the xcl.dev home page and the xcl changelog say the engine runs as a bundled subprocess everywhere and that Alpine is supported.
- The milestone ends with v0.5.0, the first published Python release.

**Validation point**:
- The v0.5.0 tag attaches eight `xcl-core` archives and a checksums file whose signature verifies, from a job whose log shows no C compiler.
- `xcl-lang 0.5.0` is on PyPI with eight wheels and an sdist.
- Every install check passes, and the engine reports `v0.5.0`.

#### - [x] Task: Tagging xcl releases the engine program for eight platforms with no C toolchain
**Id:** 9d2e550d-a82e-408e-8cc7-5d338ff3420d
**Repo:** xcl
**Depends on:**
- 46cf5eda-9cd3-450a-87e6-267679e0146f — The engine program serves the core service over standard input and output
**Execution:** agent

The release workflow is replaced by one that builds the engine program for all eight platforms in a single job, with cgo disabled and every C compiler hidden from the path. It packages each build under the registry's asset names, writes and signs the checksums with the jumppad-labs key, attaches them to the GitHub release and dispatches the xcl-python release. The shared library's toolchain matrix moves unchanged into a workflow run only by hand, which keeps its builds as workflow artifacts and attaches nothing. Go CI keeps building and smoke-testing the library on every change.

*Technical detail:* [context.md#task-tagging-xcl-releases-the-engine-program-for-eight-platforms-with-no-c-toolchain](./context.md#task-tagging-xcl-releases-the-engine-program-for-eight-platforms-with-no-c-toolchain)

**Acceptance criteria**:
- [x] A tag push builds eight `xcl-core` archives, Linux x86-64 and arm64 on glibc and musl, macOS x86-64 and arm64, and Windows x86-64 and arm64, in one job that fails if a C compiler is reachable.
- [ ] The archives are listed in one checksums file with a detached signature that verifies with the jumppad-labs key, and all are attached to the release. No libxcl archive is attached.
- [x] Each archived program reports the tag as its version.
- [ ] The xcl-python release is dispatched with the tag once the assets are attached. Run by hand with publishing off, the workflow only keeps the assets as an artifact.
- [ ] The shared library's eight-platform build can still be run by hand and produces the eight libraries as workflow artifacts.

#### - [x] Task: The Python release builds eight wheels around the engine program and checks them on every platform
**Id:** 92f42c4a-de67-409c-8e67-42694af0d8ba
**Repo:** xcl-python
**Depends on:**
- a08d3d86-6310-4e19-beeb-2992a7b0c11c — The package runs the bundled engine program by default and the shared library on request
- 9d2e550d-a82e-408e-8cc7-5d338ff3420d — Tagging xcl releases the engine program for eight platforms with no C toolchain
**Execution:** agent

For each of the eight platforms, the release downloads the engine-program archive and the signed checksums from the xcl release, verifies them, and places the program in the package. It then builds the wheel, checks the engine reports the tag, and runs the parity suite against the installed wheel. The musl wheels keep the hand-started Alpine container. After publishing to PyPI, the install checks install `xcl-lang` at the tag's version on all eight platforms with no Go or C compiler, the musl ones in a plain Alpine image. There they run a program that registers a class, applies a configuration, reads the entity back and closes.

*Technical detail:* [context.md#task-the-python-release-builds-eight-wheels-around-the-engine-program-and-checks-them-on-every-platform](./context.md#task-the-python-release-builds-eight-wheels-around-the-engine-program-and-checks-them-on-every-platform)

**Acceptance criteria**:
- [ ] Each of the eight wheels carries its platform's engine program, verified against the signed checksums from the xcl release.
- [x] The parity suite runs against every installed wheel before anything is published, and any difference stops the release.
- [ ] Eight wheels and an sdist are published to PyPI at the tag's version.
- [ ] On all eight platforms, with no Go or C compiler, the published package installs, applies and reads back an entity, and reports the tag as the engine version. The two musl platforms run in a plain `python:3-alpine` image.

#### - [x] Task: The Python package README describes the bundled engine
**Id:** fe764af7-e6d7-4153-895a-6dce2e665eba
**Repo:** xcl-python
**Depends on:**
- a08d3d86-6310-4e19-beeb-2992a7b0c11c — The package runs the bundled engine program by default and the shared library on request
**Execution:** agent

The README, which is also the PyPI page, and the package description now say:
- the engine runs as a bundled subprocess with the same API on every platform, Alpine included;
- the engine starts on first use, is shared, and stops on its own;
- a forked child simply creates its own configuration.

The development section explains building the program, and optionally the shared library with the in-process override.

*Technical detail:* [context.md#task-the-python-package-readme-describes-the-bundled-engine](./context.md#task-the-python-package-readme-describes-the-bundled-engine)

**Acceptance criteria**:
- [x] The README states that the engine runs as a bundled subprocess on every platform with the same API, and lists Alpine (musl) among the supported platforms.
- [x] The forking section gives the subprocess route's rule, and the errors section names `EngineNotFoundError` and `EngineExitedError`.
- [x] The development section explains `XCL_TRANSPORT=inprocess` and how to build each engine form.
- [x] Nothing in the README or the package description still says the engine is loaded as a shared library by default.

#### - [x] Task: xcl.dev shows how to install XCL for Python
**Id:** a91c5228-6bb5-4ccf-8dbc-87070f3da914
**Repo:** xcl-website
**Depends on:**
- a08d3d86-6310-4e19-beeb-2992a7b0c11c — The package runs the bundled engine program by default and the shared library on request
**Execution:** agent

The home page gains an install section with Go and Python tabs. The Python tab gives `pip install xcl-lang`, says the package bundles the engine and runs it as a subprocess on every platform with the same API, and lists the eight platforms, Alpine included, linking the PyPI project. The hero badge and the page description move to v0.5.0 and to Go and Python.

*Technical detail:* [context.md#task-xcldev-shows-how-to-install-xcl-for-python](./context.md#task-xcldev-shows-how-to-install-xcl-for-python)

**Acceptance criteria**:
- [x] The home page has an install section whose Python tab shows `pip install xcl-lang` and says the engine runs as a bundled subprocess on every platform with the same API.
- [x] The Python tab lists Alpine among the supported platforms and links the PyPI project.
- [x] The hero badge reads v0.5.0, and the page description no longer says only Go is available.
- [x] The site builds.

#### - [x] Task: The xcl changelog and README record the engine program
**Id:** 3abbaf58-6386-42c8-ba52-6fc30c6d83f7
**Repo:** xcl
**Depends on:**
- 46cf5eda-9cd3-450a-87e6-267679e0146f — The engine program serves the core service over standard input and output
- 9d2e550d-a82e-408e-8cc7-5d338ff3420d — Tagging xcl releases the engine program for eight platforms with no C toolchain
**Execution:** agent

xcl's changelog gets one entry for this spec. It describes:
- the engine program and its frame messages;
- the core service's new release-everything method;
- the release now attaching `xcl-core` archives built without a C toolchain;
- the shared library moving to a manually run workflow while staying CI-tested;
- Python running the engine as a bundled subprocess everywhere, Alpine included, from v0.5.0.

The README's note on other languages says the same in a sentence.

*Technical detail:* [context.md#task-the-xcl-changelog-and-readme-record-the-engine-program](./context.md#task-the-xcl-changelog-and-readme-record-the-engine-program)

**Acceptance criteria**:
- [x] The changelog has a `## 20261011211634-a8895873-python-out-of-process-engine` entry at the top, in prose, ending with **Breaking:** and `None.`
- [x] The README says Python runs the engine as a bundled program on every platform, with no shared library to load.

#### - [ ] Task: Cut the v0.5.0 release
**Id:** 7ac16d04-6d56-48fd-b4b6-94e79445158c
**Repo:** xcl
**Depends on:**
- a7327c0c-1ee8-4248-bf1d-e8427d4b6862 — CI tests the package on the engine program everywhere and on the shared library on Linux
- 92f42c4a-de67-409c-8e67-42694af0d8ba — The Python release builds eight wheels around the engine program and checks them on every platform
- fe764af7-e6d7-4153-895a-6dce2e665eba — The Python package README describes the bundled engine
- a91c5228-6bb5-4ccf-8dbc-87070f3da914 — xcl.dev shows how to install XCL for Python
- 3abbaf58-6386-42c8-ba52-6fc30c6d83f7 — The xcl changelog and README record the engine program
**Execution:** human — publishing a release tag is an action outside the repo's code, and it triggers PyPI publishing under the project's credentials

A person merges the xcl, xcl-python and xcl-website work and pushes the xcl tag `v0.5.0`. This starts the chain that attaches the engine-program archives, publishes `xcl-lang 0.5.0` to PyPI and runs the install checks on every platform. The existing v0.4.0 tag and its release are left as they are.

*Technical detail:* [context.md#task-cut-the-v050-release](./context.md#task-cut-the-v050-release)

**Acceptance criteria**:
- [ ] The xcl v0.5.0 release carries eight `xcl-core` archives and a signed checksums file, and no libxcl archive.
- [ ] `xcl-lang 0.5.0` is on PyPI with eight wheels and an sdist, and its install checks passed on all eight platforms.

## Open Questions

- **Does the first `Config()` stay under one second on the Windows runners?** It depends on how long antivirus takes to scan a freshly unpacked executable on GitHub's Windows images, which only shows in CI.
  - If the timing test fails only on Windows, and the time is spent before the engine answers `GetVersion`, STOP and ask the user. The options are to relax the metric for Windows, or to pursue Authenticode signing, which needs a certificate.
  - Do not loosen the assertion on your own.
- **Does pip keep the executable bit on `xcl/_bin/xcl-core` from the wheel on every platform and installer?** It depends on hatchling writing the mode into the wheel and on pip and other installers applying it, which is checked once a wheel is built and installed.
  - If the release's executable check fails, add a step that sets the mode before building and recheck.
  - If an installer drops the mode regardless, STOP and ask the user before adding a run-time `chmod`, which may fail in read-only site-packages.
- **Does `python:3-alpine` install `protobuf` without a compiler on both architectures?** It depends on PyPI carrying musllinux wheels of `protobuf` for the resolved version, which the install check shows. If pip falls back to building from source there, STOP and ask the user, because the spec forbids a compiler on Alpine.

No other questions remain open. Every design choice is settled in the architecture and recorded in the assumption log.

## Out of Scope

- **Releasing the shared library, or shipping it in any wheel.** It stays a source-build option, built by hand through the manual workflow and smoke-tested in CI, until a consumer asks for it (spec non-goal; design open question "whether libxcl should be released at all").
- **The Node binding and its transport** (spec non-goal; design phase 5).
- **The CI test fixes and the Docker replacement:** Python 3.10 `tomllib`, the `XCL_SRC` fixture path, the signed test plugin replacing xcl-plugin-docker in end-to-end tests, and the single recorded xcl ref. These belong to the sibling spec `20261011232013-da18c586-python-release-hardening`.
- **Moving back to the in-process route** once Go's linker supports dynamic TLS on musl and the Windows x86-64 library loads. The design records this as the way back, decided then with that day's measurements.
- **Platforms that forbid starting a process** (iOS, store sandboxes, browsers, edge isolates), and the static-archive or Wasm transports they would need (spec non-goal).
- **Authenticode signing of the Windows engine program.** The design asks for it when a certificate is available, and none is, so it is a follow-up for when one exists.
- **The download-into-`~/.xcl/core/<version>/` and build-from-sdist fallbacks** the design lists after bundling. Every wheel bundles the program, and the sdist keeps its current development-only role.
- **Python providers, protocol v2, Python plugins, `DescribeTypes` and code generation.** They are unchanged by this plan, and later specs in the design's phasing cover them.
- **Changes to the Python API, the CoreService messages or the C ABI**, beyond the two new exception classes the design names (spec constraint).
- **Republishing or altering v0.4.0.** The tag and its GitHub release are left as they are (spec constraint).
- **New xcl.dev pages or navigation for Python.** The docs specs in this epic write the getting-started guides; this plan adds only the home page install section.

## Changelog

### 2026-10-13 — Task: The frame messages are part of the protocol

**What was done**: Added `proto/xcl/core/v1/transport.proto` with the design's `Request{id, handle, method, payload}` and `Response{id, status, payload}` frame messages, documenting the 4-byte big-endian length prefix, host-chosen ids echoed back, out-of-order responses, the reserved `Release` method name and the two's-complement carriage of negative statuses. Generated `coreservice/proto/transport.pb.go` with `buf generate`.

**Deviations**: The `go_package` option carries the `;corepb` package name suffix that `core.proto` already uses, so the generated code shares the `corepb` package. No dedicated test was written for this task, as the plan's testing strategy says; the serve-loop tests of the next task exercise the messages.

**Files changed**:
- `xcl: proto/xcl/core/v1/transport.proto`
- `xcl: coreservice/proto/transport.pb.go`

**Discoveries**: `buf breaking --against '.git#branch=main'` (the Makefile's `buf-breaking` target) fails locally on a pre-existing issue ("plugins/plugin.proto was deleted") unrelated to this work; CI uses `subdir=proto`, and the check against `HEAD` passes.

### 2026-10-13 — Task: The engine program serves the core service over standard input and output

**What was done**: Added `cmd/xcl-core`, a cgo-free program in the root module. `--version` prints the stamped version; `--serve` keeps the original stdin/stdout for frames, points `os.Stdin` at the null device and `os.Stdout` at stderr, discards interrupts (and SIGPIPE) through `signal.Notify`, and serves until end of input or SIGTERM. The serve loop decodes length-prefixed `Request` frames, answers each on its own goroutine through `coreservice.Service` (method `Release` goes to `Service.Release`), writes `Response` frames under a mutex with negative statuses as two's-complement `uint32`, and on end of input, SIGTERM or a failed write releases every configuration and drains in-flight requests. `coreservice.Service` gained the additive `Close`, and the Makefile gained the `xcl-core` target. Serve-loop, smoke and `Close` tests were written beside the code, and a Windows job in `go.yml` runs the program's pipe and Ctrl-Break smoke tests.

**Deviations**:
- `serve` takes a `context.Context` first (`serve(ctx, in, out, service)`), as the plan's context.md describes, rather than the three-argument form in the plan's interface sketch; SIGTERM cancels the context.
- The program also discards SIGPIPE, so a host that has gone away turns the next write into an error rather than killing the program before configurations are released.
- The loop calls `Service.Close` a second time after draining in-flight requests, so a configuration created by a request still in flight during the first release is released too; new requests arriving once shutdown has begun are dropped.
- The Makefile target honours `GOOS`/`GOARCH` passed as make variables (deriving `.exe` from them) as well as `XCL_CORE_GOOS`/`XCL_CORE_GOARCH`.
- The external-plugin smoke test uses the root module's `internal/testutil.BuildFixturePlugin` (the widget plugin) rather than the nested libxcl fixture plugins.
- `.github/workflows/go.yml` gained an `xcl-core-windows` job (the plan expected no go.yml change): Go CI only tested on Linux, so without it the Ctrl-Break smoke test would never run. It runs the pipe and Ctrl-Break tests on `windows-latest`; the external plugin test is left to Linux because the fixture plugin helper builds no `.exe`.
- The acceptance criterion on interrupts is left unchecked: SIGINT and SIGTERM pass on Linux, but the Windows Ctrl-Break test was only compile-checked here (`GOOS=windows go vet`); it runs in the new Windows CI job, which needs a push to execute.
- Doc comments in `coreservice/service.go` and `coreservice/version.go` now name xcl-core beside libxcl.

**Files changed**:
- `xcl: cmd/xcl-core/main.go`
- `xcl: cmd/xcl-core/serve.go`
- `xcl: cmd/xcl-core/serve_test.go`
- `xcl: cmd/xcl-core/smoke_test.go`
- `xcl: cmd/xcl-core/signals_windows_test.go`
- `xcl: cmd/xcl-core/signals_other_test.go`
- `xcl: coreservice/service.go`
- `xcl: coreservice/service_test.go`
- `xcl: coreservice/version.go`
- `xcl: Makefile`
- `xcl: .gitignore`
- `xcl: .github/workflows/go.yml`

**Discoveries**: CoreService has no `Log` method, so the out-of-order test sends a `Cancel` while a `Poll` waits; cancelling a finished operation keeps the ordering deterministic. go-plugin v1.6.3 hands `os.Stdin` (the variable) to every plugin it starts, so reassigning the variable before serving is enough to keep plugins off the frame pipe. Go writes to a broken fd 1 raise SIGPIPE and kill the process unless SIGPIPE is notified. `go test ./... -race` on the root module passes; `make test-libxcl` still passes.

### 2026-10-13 — Task: Python talks to the engine program through a subprocess transport

**What was done**: Added `xcl/_transport/subprocess.py` with `SubprocessTransport`: it starts `xcl/_bin/xcl-core --serve` on the first call with a pipe pair and inherited stderr, checks the ABI through `GetVersion`, matches responses to waiting calls by id on one daemon reader thread, waits in 0.1 s slices, tracks configurations from `CreateConfig` responses, stops the engine (close stdin, bounded wait, terminate, kill) when the last configuration is released, on `close()` and at exit, fails every waiter with `EngineExitedError` when the engine's output ends and starts a fresh engine on the next call, refuses pre-fork configurations in a forked child with `ForkedProcessError`, and on Windows puts the engine in a kill-on-close job object through ctypes. Added `EngineNotFoundError` and `EngineExitedError` to `xcl/_errors.py` and the package exports, the `transport.proto` messages to `make protos` (generated `transport_pb2`), and a `make engine` target. Unit tests drive the transport against `tests/fake_engine.py`.

**Deviations**:
- The transport gives out its own configuration handles and maps each to the engine that issued it and that engine's handle, rewriting the `CreateConfig` response. A new engine numbers its handles from 1 again, so with engine-issued handles a configuration of a dead engine (or of the parent of a fork) would silently reach whichever configuration the new engine numbered the same; the plan's `_dead_handles`/`_forked_handles` sets could not tell them apart. Unknown transport handles return the unknown-handle status (-3) without contacting the engine.
- A forked child closes its copies of the parent engine's pipes, so a child that outlives its parent cannot keep the parent's engine alive.
- Failure to create or assign the Windows job object is a `RuntimeWarning`, not an error: the engine still ends with its input.
- Waiters are failed before the transport lock is taken, and a failed start does not join the reader, so an engine that dies during its handshake cannot deadlock the start.
- `ForkedProcessError`'s docstring now describes both routes; its default (in-process) message is unchanged.
- `select.py`, the `default_client()` change, packaging, version and test-fixture changes landed in the worktree at the same time but belong to the next task and are recorded there.

**Files changed**:
- `xcl-python: xcl/_transport/subprocess.py`
- `xcl-python: xcl/_transport/__init__.py`
- `xcl-python: xcl/_errors.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: xcl/_core/transport_pb2.py`
- `xcl-python: xcl/_core/transport_pb2.pyi`
- `xcl-python: Makefile`
- `xcl-python: .gitignore`
- `xcl-python: tests/fake_engine.py`
- `xcl-python: tests/unit/test_subprocess_transport.py`
- `xcl-python: tests/unit/test_engine_errors.py`

**Discoveries**: The committed `core_pb2.py` is protobuf 7.36.1 gencode, so `transport_pb2.py` was generated with the same local protoc (libprotoc 36.1) to match; CI regenerates both with protoc 28.x. Handles are only unique within one engine process, so any host that restarts the engine must not hand engine handles straight to its callers.

### 2026-10-13 — Task: The package runs the bundled engine program by default and the shared library on request

**What was done**: Added `xcl/_transport/select.py` (`TRANSPORT_ENV`, `selected_route()`, `select_transport()`): unset, empty or `subprocess` selects `xcl/_bin/xcl-core`, `inprocess` selects the shared library, any other value raises `ValueError`, and a missing selected engine raises `EngineNotFoundError` naming both paths, with no fallback. `default_client()` builds its client over `select_transport()`, still lazily. Packaging: version `0.5.0.dev0`, `cffi` moved to an `inprocess` extra, `xcl/_bin/*` added to the wheel artifacts and excluded from the sdist, and the package, `hatch_build.py` and in-process module docstrings describe the bundled program. The test tree gained `tests/engines.py` (`expected_routes`, `skip_missing_engine`, `selected_engine`), the `subprocess_only`/`inprocess_only` markers with a collection hook, engine fixtures that follow the selected route, unit tests for selection and the strict-skip helper, and `tests/integration/test_engine_process.py` covering one engine per process, stop on close/exit/host kill, engine death, process-group interrupt, fork, version mismatch, first-`Config()` timing and the missing program.

**Deviations**:
- The engine fixtures call `tests/engines.py::selected_engine()`, which checks the selected engine's file and raises the same `EngineNotFoundError` message through `skip_missing_engine`, rather than constructing a transport with `select_transport()`; the effect is the same and no transport is built at fixture time.
- `test_load.py`'s `needs_engine` became `pytest.mark.usefixtures("engine")`, so strict mode applies to it.
- The interrupt integration test signals the process group while a short create is in flight and asserts the next resource never starts, because a provider call already running is not interrupted by cancellation on either route (see Discoveries).
- The engine-kill tests list the engine's plugin children before the kill and end them afterwards (see Discoveries).
- Locally verified: the full suite passes on the engine program (243 passed, 1 in-process-only skip, also under `XCL_TEST_STRICT=1 XCL_TEST_ENGINES=subprocess`) and on the shared library with `XCL_TRANSPORT=inprocess` (232 passed, 12 subprocess-only skips); with the program removed, strict mode errors and non-strict mode skips; a wheel built with `XCL_WHEEL_PLATFORM=manylinux_2_28_x86_64` carries `xcl/_bin/xcl-core` with mode 0755 and installs into a fresh venv without cffi, Go or a C compiler.

**Files changed**:
- `xcl-python: xcl/_transport/select.py`
- `xcl-python: xcl/_client.py`
- `xcl-python: xcl/_transport/inprocess.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: xcl/_version.py`
- `xcl-python: pyproject.toml`
- `xcl-python: hatch_build.py`
- `xcl-python: tests/conftest.py`
- `xcl-python: tests/engines.py`
- `xcl-python: tests/integration/conftest.py`
- `xcl-python: tests/parity/conftest.py`
- `xcl-python: tests/integration/test_load.py`
- `xcl-python: tests/integration/test_engine_process.py`
- `xcl-python: tests/unit/test_transport_selection.py`
- `xcl-python: tests/unit/test_engines_helper.py`

**Discoveries**:
- Cancelling an apply does not interrupt a provider call already running, on either route: the slow fixture plugin's wait never sees its context cancelled, so a SIGINT during a 30 s create waits out the 30 s. The existing `test_keyboard_interrupt_cancels_apply` passes only because its 3 s delay ends inside its bound. This is engine/plugin behaviour, outside this spec.
- An engine ended by SIGKILL leaves its go-plugin plugin processes running as orphans (go-plugin plugins do not watch for their host dying). Ending the host instead lets the engine release its configurations and its plugins on end of input, so the spec's host-kill guarantee holds; only a directly killed engine leaks plugins.

### 2026-10-13 — Task: CI tests the package on the engine program everywhere and on the shared library on Linux

**What was done**: `.github/workflows/ci.yml` builds the engine program (`make protos engine plugins`) on every runner and Python version and runs the suite with `XCL_TEST_STRICT=1` and `XCL_TEST_ENGINES=subprocess`. On `ubuntu-latest` it then builds the shared library, installs `.[test,inprocess]`, and runs the suite again with `XCL_TRANSPORT=inprocess` and `XCL_TEST_ENGINES=subprocess,inprocess`. The Alpine jobs keep the hand-started `python:3-alpine` container, build the engine program, pass the strict-mode variables, and install no C compiler. The header comment describes the routes and strict mode.

**Deviations**:
- The Alpine jobs install Go from the official go.dev archive (`GO_VERSION` passed into the container) instead of `apk add go`, and check that neither `cc` nor `gcc` is on the path: Alpine's `go` package pulls in `gcc` and `musl-dev`, so dropping `build-base` alone would still install a C compiler. Only `libgcc`, a runtime library, remains.
- Verified locally by running the Alpine job's script in docker on x86-64 against the worktrees. The engine program builds with no C compiler, and every engine test passes in strict mode: 234 passed, with skips only for Docker-daemon tests, the in-process-only test and the version test that needs Go on the path. One test failed: `tests/unit/test_schema.py::test_annotations_dataclass_matches_parity_fixture`. It reads the parity fixtures from `ROOT.parent / "xcl"` rather than `XCL_SRC`, which is the fixture-path fix owned by the sibling spec `20261011232013-da18c586-python-release-hardening`. It fails the same way before this change.
- GitHub-hosted runs of the workflow are outstanding: nothing is pushed.

**Files changed**:
- `xcl-python: .github/workflows/ci.yml`

**Discoveries**: On Alpine, `apk add go` installs gcc and musl-dev as dependencies. A job that must prove "no C compiler" has to take Go from the official archive, whose toolchain runs on musl.

### 2026-10-13 — Task: Tagging xcl releases the engine program for eight platforms with no C toolchain

**What was done**: Added `.github/workflows/release.yml`, which replaces `libxcl-release.yml` (deleted). It keeps the `settings` and `dispatch` jobs. A single `build` job on `ubuntu-latest` runs with `CGO_ENABLED=0` and puts failing shims in front of `cc`, `gcc`, `clang`, `zig`, `c++`, `g++`, `clang++` and the cross gcc names, then checks that none answers. It then:
- builds `xcl-core` for linux, darwin and windows on amd64 and arm64 with `make xcl-core`, and copies the two Linux builds to the `_musl` names;
- checks that the linux_amd64 program reports the tag, and that every binary's recorded build settings show `CGO_ENABLED=0`;
- archives the eight builds as `xcl-core_<version>_<target>.tar.gz` or `.zip` and asserts there are eight;
- writes, signs and verifies the checksums, uploads the `release-assets` artifact, and attaches the assets when publishing.

The libxcl toolchain matrix (`build-linux-windows`, `build-windows-arm64`, `build-macos`) moved verbatim into the new `workflow_dispatch`-only `.github/workflows/libxcl-build.yml`. It takes optional `ref` and `version` inputs, uploads the `libxcl-<target>` artifacts, and does no packaging, signing, attaching or dispatch. `go.yml`'s `libxcl` job is unchanged.

**Deviations**:
- The plan expected only a linux_amd64 version check. The workflow also reads each binary's build settings with `go version -m` to prove cgo was off for all eight, because the darwin, windows and arm64 programs cannot be run on the release runner.
- `libxcl-build.yml` gained a `version` input that stamps the libraries, defaulting to `dev`. The plan described stamping with the ref.
- Verified locally:
  - The build-and-package shell ran with the compiler shims in front of `PATH` and `CGO_ENABLED=0`: eight archives, `--version` equal to the tag, `CGO_ENABLED=0` recorded in every binary, and a checksums file whose entries verify.
  - The sign-and-verify commands ran with a throwaway GPG key.
  - `zip` is not installed locally, so the two `.zip` archives were made with Python's zipfile for the dry run. The ubuntu runner has `zip`, which the old workflow already used.
- Three acceptance criteria are left unchecked until a real tag run or manual workflow run:
  - the signature verifying with the jumppad-labs key, and the assets attached;
  - the dispatch to xcl-python, and a manual run keeping only the artifact;
  - the manual `libxcl-build.yml` run producing the eight libraries.

  Nothing is pushed or tagged. The v0.5.0 tag is cut by the user in the final task.

**Files changed**:
- `xcl: .github/workflows/release.yml`
- `xcl: .github/workflows/libxcl-build.yml`
- `xcl: .github/workflows/libxcl-release.yml` (deleted)

**Discoveries**: `go version -m` on a binary built with `-trimpath` does not record `-ldflags`, so the stamped version of a cross-built program cannot be read without running it. `CGO_ENABLED`, `GOOS` and `GOARCH` are recorded.

### 2026-10-13 — Task: The Python release builds eight wheels around the engine program and checks them on every platform

**What was done**: `.github/workflows/release.yml` (xcl-python) now builds each wheel around the engine program:
- Each wheel job downloads `xcl-core_<version>_<asset>` and the signed `xcl-core_<version>_checksums.txt(.sig)` from the xcl release, verifies the signature and the archive, and places `xcl-core[.exe]` in `xcl/_bin`, executable. The matrix's `lib` column became `exe`.
- The wheel test runs parity and `test_load.py` in strict mode (`XCL_TEST_STRICT=1`, `XCL_TEST_ENGINES=subprocess`).
- A new step asserts the installed program is executable, importing from outside the checkout.
- `.github/scripts/musl-wheel.sh` does the same for the musl archive, drops `build-base`, and exports the strict variables.
- New `.github/scripts/install_check.py` registers the README's `settings` class, applies a configuration, reads the entity back, closes, and asserts `xcl.engine_version()` equals `TAG`. The install-check jobs sparse-check out `.github/scripts` and run it, the musl ones inside plain `python:3-alpine` with the checkout mounted.

**Deviations**:
- `musl-wheel.sh` never installed Go, although `make plugins` needs it. It now installs Go from the official go.dev archive (`GO_VERSION` passed into the container), because Alpine's `go` package pulls in gcc.
- The executable-bit check runs from `$RUNNER_TEMP` (from `/tmp` in Alpine), so it inspects the wheel installed in site-packages. Without that, it would read the checkout's `xcl/_bin`.
- Verified locally:
  - A wheel built with `XCL_WHEEL_PLATFORM=manylinux_2_28_x86_64` carries `xcl/_bin/xcl-core` at mode 0755. It installs into a fresh venv with only protobuf, and `install_check.py` passes.
  - A `musllinux_1_2_x86_64` wheel installed into a plain `python:3-alpine` container with no C compiler. protobuf came from a musllinux wheel, and `install_check.py` passed. This answers the plan's open question for x86-64; arm64 is outstanding.
  - The parity suite passes on the engine program.
- Criterion ticked: parity on every installed wheel before publishing. The `publish` job needs the wheel jobs, which run parity strictly.
- Left unchecked until the v0.5.0 run: verified archives in all eight wheels, eight wheels and an sdist on PyPI, and the eight install checks from PyPI.

**Files changed**:
- `xcl-python: .github/workflows/release.yml`
- `xcl-python: .github/scripts/musl-wheel.sh`
- `xcl-python: .github/scripts/install_check.py`

**Discoveries**: hatchling keeps the 0755 mode of `xcl/_bin/xcl-core` in the wheel, and pip and uv apply it on install. The plan's open question about the executable bit holds on Linux.

### 2026-10-13 — Task: The Python package README describes the bundled engine

**What was done**: The xcl-python README, which is also the PyPI page, now says the engine is bundled as a small program run as a subprocess the same way on every platform, Alpine included, with the same API and results. The Install section lists glibc and musl (Alpine) Linux, macOS and Windows on x86-64 and arm64, says one wheel per platform serves every CPython 3.10+, and describes the engine starting on first use, being shared, stopping on its own and never outliving its host. The Errors section names `EngineNotFoundError` and `EngineExitedError`. Forking is rewritten for the subprocess route, where a child creates its own configuration and gunicorn `preload_app`, celery prefork and `multiprocessing` fork work. The Development section covers `make protos engine plugins`, and `make libxcl` with `pip install -e '.[test,inprocess]'` and `XCL_TRANSPORT=inprocess`, plus the stricter fork rule on that route. The `pyproject.toml` description, changed in the selection task, reads consistently.

**Deviations**: None. `twine check` on an sdist built from the worktree passes.

**Files changed**:
- `xcl-python: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: xcl.dev shows how to install XCL for Python

**What was done**: The home page (`src/pages/index.mdx`) gained an `Install` section between the hero and "What is XCL?", with `CodeTabs`. The Go tab gives `go get github.com/jumppad-labs/xcl` and notes that no cgo is needed. The Python tab gives `pip install xcl-lang` and says the package is imported as `xcl`, bundles the engine as a small program run as a subprocess on every platform with the same API and results and no Go or C compiler, and lists wheels for Linux on glibc and on musl (Alpine), macOS and Windows on x86-64 and arm64, linking `pypiURL`. The hero badge reads `v0.5.0 · Go and Python`, and the page description ends "Available for Go and Python." The hero's `InstallBlock` stays Go-only.

**Deviations**: The Go tab has a one-line note under the command, so the two tabs read alike. `npm ci && npm run build` in the worktree builds all 21 pages, and the built `index.html` renders both fences as code blocks with the PyPI link.

**Files changed**:
- `xcl-website: src/pages/index.mdx`

**Discoveries**: None.

### 2026-10-13 — Task: The xcl changelog and README record the engine program

**What was done**: `CHANGELOG.md` has a `## 20261011211634-a8895873-python-out-of-process-engine` entry at the top, in prose. It covers:
- the `xcl-core` program and its frame messages, `Release`, the stdio, signal and shutdown rules, `--version` and `make xcl-core`;
- `Service.Close`;
- the release building eight `xcl-core` archives in one cgo-free job with no C compiler, signed as before, from `release.yml`;
- libxcl kept, CI-tested and built by the manual `libxcl-build.yml` but no longer released;
- Python running the engine as a bundled subprocess everywhere, Alpine included, from v0.5.0.

The entry ends with `**Breaking:**` and `None.` The README's "Other languages" note says the wheel bundles the `xcl-core` program, run as a subprocess on every platform, Alpine included, with no shared library to load.

**Deviations**: None.

**Files changed**:
- `xcl: CHANGELOG.md`
- `xcl: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: Cut the v0.5.0 release

**What was done**: Nothing. This is a human task: publishing the `v0.5.0` tag triggers PyPI publishing under the project's credentials, and pushes are on hold. It stays unchecked, for the person who merges this spec's branches and pushes the tag. Every agent task it depends on is implemented and verified locally.

**Deviations**: These release-dependent checks are outstanding until that run, and are carried into the test plan:
- the jumppad-labs signature on the `xcl-core` checksums, and the assets attached with no libxcl archive;
- the dispatch to xcl-python;
- eight wheels and an sdist on PyPI;
- the eight install checks from PyPI;
- arm64 Alpine;
- the Windows Ctrl-Break smoke test in the new `xcl-core-windows` Go CI job;
- one manual `libxcl-build.yml` run.

**Files changed**:
- None.

**Discoveries**: Before CI can go green, the person must push this spec's xcl changes to the ref in xcl-python's `xcl-ref.txt` (`f-libxcl`). xcl-python's CI and wheel builds compile `cmd/xcl-core` and generate `transport.proto` from that ref.
