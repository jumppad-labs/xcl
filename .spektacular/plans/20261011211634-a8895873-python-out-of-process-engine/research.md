---
created_date: "2026-10-12"
document_status: final
closed_date: "2026-10-12"
---

# Research: 20261011211634-a8895873-python-out-of-process-engine

## Alternatives considered and rejected

- **Add a `Release` RPC to `CoreService`** so the subprocess carries release as an ordinary method. Rejected: the spec's constraint says the core service messages and the C ABI must not change, and `xcl_release` is a C function, not a CoreService method (`proto/xcl/core/v1/core.proto:17-46` has no Release). Instead the frame protocol reserves the method name `Release` at the transport layer (documented in `transport.proto`), and `xcl-core` routes it to `Service.Release`.
- **Put the stdio serve loop inside `coreservice`.** Rejected: the design says `coreservice` stays free of any transport (design "What coreservice is"; `coreservice/service.go:1-12` doc comment). The loop lives in `cmd/xcl-core`, tested with plain `go test` in that package.
- **A separate `xcl-core` Go module (like `bindings/libxcl`).** Rejected: the design's layer table puts `cmd/xcl-core` in the root module, built by the `CGO_ENABLED=0` job (`.github/workflows/go.yml:26-28`), and it needs no cgo.
- **Ship both engines in every wheel and pick at runtime / subprocess for musl only.** Rejected by the design ("Subprocess as the default transport", alternatives list) and the spec constraint "never by probing the machine at runtime".
- **Let the transport count handles it has seen in calls** to know when the last configuration closes. Rejected: a configuration created and never used would be invisible, so closing a second configuration would stop the engine under the first. The transport instead reads the handle out of a successful `CreateConfig` response (it already imports `core_pb2`).
- **Keep `cffi` a required dependency.** Rejected as the default: only the source-build in-process route uses it, `ctypes` is already the fallback (`xcl-python:xcl/_transport/inprocess.py:243-253`), and dropping it removes the only compiled dependency from a plain-Alpine install. It moves to an optional `inprocess` extra.
- **Build the release on a runner with gcc present and rely on `CGO_ENABLED=0` alone.** Rejected: the acceptance criterion says "from a job with no C compiler installed". The job hides every C compiler behind failing shims on `PATH`, the pattern `xcl-python:.github/workflows/release.yml` install-check already uses, and checks `cc` fails before building.
- **Alpine job containers for the arm64 musl jobs.** Rejected by spec constraint; keep the hand-started `docker run python:3-alpine` pattern (`xcl-python:.github/workflows/ci.yml` test-musl, `release.yml` wheels-musl).
- **A second Python test suite for the subprocess route.** Rejected by the spec's technical approach; the existing suite runs over both transports with `XCL_TRANSPORT`, and transport-specific tests are marked to run only on their route.
- **Change `Response.status` to `int32`/`sint32`.** Not done: the design fixes `uint32 status = 2`. The negative statuses cross as their two's-complement `uint32` (Go `uint32(int32(status))`, Python reinterprets to signed). Recorded as an assumption; the design may prefer to say `int32` later.

## Chosen approach — evidence

- Design `libxcl.md` sections "The transport: xcl-core over stdio", "Process lifecycle", "Starting the engine", "Choosing the transport", "Artifacts per release", "Distribution": framing (4-byte big-endian length + protobuf `Request{id,handle,method,payload}` / `Response{id,status,payload}` in `proto/xcl/core/v1/transport.proto`), out-of-order responses, `xcl-core --serve` / `--version`, stdin EOF exit, SIGINT ignored, SIGTERM/TerminateProcess with bounded wait, Windows job object kill-on-close, `EngineExitedError`, `EngineNotFoundError`, `ForkedProcessError`, `XCL_TRANSPORT=inprocess`, asset names `xcl-core_<version>_<os>_<arch>[_musl].tar.gz` (`.zip` Windows) with `xcl-core_<version>_checksums.txt(.sig)`, one `CGO_ENABLED=0` job, libxcl matrix moved to a manually triggered workflow.
- `coreservice/service.go:71` `Service.Call` is concurrency-safe; `Service.Release` (`service.go:111`) cancels and waits for state save, then closes the queue so a blocked Poll wakes. Poll blocks up to `wait_ms` (`coreservice/operations.go:197`, `queue.go:90`) — so the server must run each request on its own goroutine.
- `coreservice/version.go:8` `Version` stamped by `-X github.com/jumppad-labs/xcl/coreservice.Version=...` (`Makefile:51`); `ABIVersion = 1`; `GetVersionResponse.abi_version` exists (`core.proto:65-69`).
- go-plugin v1.6.3 sets `cmd.Stdin = os.Stdin` for every external plugin (`$GOMODCACHE/github.com/hashicorp/go-plugin@v1.6.3/client.go:660`): in `xcl-core` that would hand the frame pipe to plugins. Reassigning the `os.Stdin` variable to a `/dev/null` (`os.DevNull`) file before serving, after keeping the original for the protocol, fixes it portably. Plugin stdout is a handshake pipe and plugin stderr is routed into events (`plugins/grpc_plugin_host.go:101-111`, go-plugin `SyncStdout/SyncStderr` default to `io.Discard`).
- Nothing in the root module writes to stdout except `logger.NewResourcePrinter` (`logger/pretty_printer.go:108`, unused by the library). `xcl-core` still reassigns `os.Stdout = os.Stderr` after capturing the protocol writer, as a guard.
- Existing release workflow to replace: `.github/workflows/libxcl-release.yml` (settings → zig/llvm-mingw/macOS matrix → package & sign → attach → dispatch `xcl-release` to xcl-python). Its settings, package/sign/attach and dispatch jobs carry over unchanged in shape; only the build becomes one job.
- `go.yml` `libxcl` job (build + exported-symbol check + `make test-libxcl`) already satisfies "CI builds the shared library on Linux x86-64 and runs its smoke test on every change".
- xcl-python transport seam: `xcl/_transport/__init__.py` `Transport` protocol (`call`, `release`); `xcl/_client.py:120-131` `default_client()` builds `Client(InProcessTransport())` — the single selection point to change. `Client._ensure_version` checks the package version once.
- Poll loop interrupt handling already cancels and drains on `KeyboardInterrupt` (`xcl-python:xcl/_poll.py:176-215`), and the asyncio API runs blocking calls in executor threads (`xcl/aio.py:5-7`), so a blocking subprocess transport keeps both working.
- Wheel tagging: `hatch_build.py` tags `py3-none-$XCL_WHEEL_PLATFORM`; `pyproject.toml` `artifacts = ["xcl/_lib/*"]` — add `xcl/_bin/*`.
- Parity/fixture plugin builds: `xcl-python:Makefile:33` builds `bindings/libxcl/testdata/plugins/{slow,recording}` and `bindings/libxcl/cmd/parity` with plain `go build` (no cgo in those packages).

## Files examined

- `xcl:proto/xcl/core/v1/core.proto:17-69` — CoreService methods (no Release); GetVersionResponse has version and abi_version.
- `xcl:coreservice/service.go:1-200` — Status codes (-4..1), Call/Release, handler table held to descriptor.
- `xcl:coreservice/version.go` — Version var ("dev"), ABIVersion 1.
- `xcl:coreservice/operations.go:29,103,177,197`, `queue.go:90,115` — operations, release waits for state, cancel, poll blocking.
- `xcl:bindings/libxcl/main.go`, `callers.go`, `smoke_test.go` — four C exports over `coreservice.New()`; smoke tests through the ABI.
- `xcl:bindings/libxcl/cmd/parity/main.go`, `testdata/parity/*`, `testdata/plugins/{recording,slow}` — parity program and fixture plugins used by xcl-python.
- `xcl:Makefile:35-60` — `libxcl`, `test-libxcl`, `LIBXCL_LDFLAGS`; `core-protos` runs `buf generate` (`buf.gen.yaml`, `buf.work.yaml`, `proto/buf.yaml`).
- `xcl:.github/workflows/libxcl-release.yml` — current release (zig, llvm-mingw, macOS runners, package, sign, attach, dispatch).
- `xcl:.github/workflows/go.yml` — CGO_ENABLED=0 cross builds, tests with -race, buf lint/breaking, min-Go build incl. libxcl, libxcl smoke job.
- `xcl:registry/github_assets.go:72-97` — asset naming contract (no `_musl` in Go code; extension only in workflow).
- `xcl:CHANGELOG.md:1-30` — `## <spec-name>` prose entries ending with `**Breaking:**` and `None.`; newest at top.
- `xcl:README.md:195-196` — "Other languages" pointer to xcl-lang.
- `go-plugin@v1.6.3/client.go:660` — plugins inherit `os.Stdin`.
- `xcl-python:xcl/_transport/__init__.py`, `inprocess.py` — Transport protocol, ForkGuard, XCL_LIBRARY_PATH, cffi/ctypes loaders.
- `xcl-python:xcl/_client.py` — typed client, version check, default_client.
- `xcl-python:xcl/_errors.py:256-303` — ForkedProcessError (names spawn/post_fork), EngineVersionError, InternalBindingError.
- `xcl-python:xcl/_poll.py:100-215` — cancel/drain on KeyboardInterrupt.
- `xcl-python:xcl/_version.py` — `0.4.0.dev0`.
- `xcl-python:Makefile`, `pyproject.toml`, `hatch_build.py`, `.gitignore`, `xcl-ref.txt` (`f-libxcl`) — build and packaging.
- `xcl-python:.github/workflows/ci.yml`, `release.yml`, `.github/scripts/musl-wheel.sh` — CI matrix, Alpine by hand, release from libxcl archives, install checks.
- `xcl-python:tests/integration/conftest.py`, `tests/parity/conftest.py`, `tests/integration/test_load.py`, `test_sync.py:674-780`, `tests/unit/test_transport.py`, `test_fork.py` — engine fixtures keyed on `default_library_path()`, ctypes fallback test, SIGINT test (signals only the Python process), fork tests.
- `xcl-python:README.md:1-30,229-244` — install/platform text says "shared library"; fork section.
- `xcl-website:src/pages/index.mdx:1-25` — only install is `go get`; badge `v0.4.0 · Go and Python`; description "Go implementation available now".
- `xcl-website:src/config/site.ts:10-14` — `pypiURL` defined, unused.
- `xcl-website:src/components/InstallBlock.astro`, `CodeTabs.astro` — install block and Go/Python tabs.
- `xcl-website:.github/workflows/deploy.yml`, `package.json`, `Makefile` — build only (`npm run build`, `make check`); no content tests.

## External references

- golang/go#54805 — musl refuses dlopen of Go c-shared (initial-exec TLS); why the library cannot ship on Alpine.
- go-plugin "received EOF" rule / `client.go` — precedent for exiting on stdin EOF and the stdin-inheritance hazard.
- Win32 job objects (`CreateJobObjectW`, `SetInformationJobObject` with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, `AssignProcessToJobObject`) — via `ctypes` in the stdlib, no pywin32.
- Python `os.register_at_fork` — reset transport state in a forked child.
- pip/wheel executable-bit handling — pip sets the executable bit from the zip entry's external attributes; the installed binary must be executable (verified in release install checks).

## Prior plans / specs consulted

- Plan `20261011130709-aa3eb933-python-support` (historical): the libxcl release workflow and xcl-python CI/release were built there; conventions it applied (pytest without parametrize tables, no tests reading repo files, changelog entry per spec with **Breaking:**, human task to cut the tag); the first release (v0.4.0, run 38172909823) failed on musl and Windows x86-64, which this spec answers.
- Spec `20261011232013-da18c586-python-release-hardening` (sibling, planned in parallel): owns tomllib on 3.10, XCL_SRC fixture location, the signed test plugin replacing Docker, and the single recorded xcl ref. It edits `xcl-python` `ci.yml` and test conftests too — file overlap to be ordered by the orchestrator.
- Epic `20261010160411-19b965db-language-reference`: this spec depends on python-support and precedes the docs specs.

## Open assumptions

- The two's-complement `uint32` carriage of negative statuses in `Response.status` is acceptable to the design's owner.
- `pip` keeps the executable bit on `xcl/_bin/xcl-core` from the wheel on every platform; if an install check finds it not executable, STOP and ask.
- `python:3-alpine` has no compiler and installs `protobuf` from a musllinux wheel; if `protobuf` needs building there, STOP and ask.
- A first `Config()` (engine start + GetVersion + CreateConfig) finishes within one second on GitHub runners, Windows included (antivirus scanning a fresh executable is the risk).
- go-plugin reads the `os.Stdin` variable at plugin start, so reassigning it in `xcl-core` keeps frames away from plugins.
- No Authenticode certificate is available; Windows signing is not done in this spec.

## Drafting assumptions

### Chosen direction: serve loop in cmd/xcl-core, one reader thread plus per-call waiters in Python (architecture)
- **Decision**: Build the design's shape with (a) the stdio serve loop in the `main` package of `cmd/xcl-core`, one reader goroutine and one goroutine per request, writes serialised by a mutex, end of input or SIGTERM releasing every configuration via an additive `Service.Close`; and (b) a Python `SubprocessTransport` with one reader thread matching responses by id to per-call waiters, the transport owning process start, handle tracking, shutdown, engine-death and fork handling. Key trade-offs: the transport decodes `CreateConfig` responses to track live handles (a small layering leak, keeps the `Transport` protocol unchanged); waits are sliced so Ctrl-C is delivered on Windows; handles from a dead engine raise `EngineExitedError` rather than an internal binding error.
- **Rationale**: It is the smallest implementation of the design's transport that keeps `coreservice` transport-free, keeps behaviour tests as plain Go tests, and leaves everything above the Python transport untouched.
- **Rejected**: A serve-loop package (`internal/stdioserver` or `coreservice/stdio`) — more surface for one consumer and against "coreservice stays transport-free"; lock-step Python calls without a reader thread — a blocking Poll would hold up Log and Cancel, which the design's out-of-order responses exist to prevent; an asyncio-native transport — the synchronous API needs threads anyway.

### Convention selection (architecture)
- **Decision**: Apply testing & mocking, no repo-file tests, project structure, dependencies, code style, patterns & architecture, structured logs and shared test helpers; drop the parser-, provider- and state-format-specific conventions and the shared errors package.
- **Rationale**: The work is a new program, a transport, packaging and workflows; it touches no parser, provider or Go error type.
- **Rejected**: Listing every convention (noise).

### Release crosses the frame protocol as a reserved method name (discovery)
- **Decision**: The frame `Request` with `method = "Release"` and a handle frees that configuration through `Service.Release`; it is documented in `transport.proto`, not added to `CoreService`.
- **Rationale**: The design says `Config.close()` "sends Release for its handle", and the spec forbids changing the core service messages or the C ABI; `Release` is the C ABI's fourth function carried over the new channel.
- **Rejected**: A `Release` RPC in `core.proto` (changes the core service messages).

### Negative statuses cross as two's-complement uint32 (discovery)
- **Decision**: Keep the design's `uint32 status = 2`; Go writes `uint32(int32(status))` and Python reinterprets the value as signed 32-bit.
- **Rationale**: Follows the design literally while carrying the existing -1..-4 statuses unchanged.
- **Rejected**: Changing the field to `int32`/`sint32` (departs from the written design; can be raised with the design owner at review).

### cffi becomes an optional extra (discovery)
- **Decision**: `cffi` moves from a required dependency to an `inprocess` extra; the in-process transport keeps falling back to `ctypes`.
- **Rationale**: Only the source-build in-process route uses it; a plain-Alpine install then needs nothing beyond `protobuf`.
- **Rejected**: Keeping it required (extra install weight and a compiled dependency on every platform for an unused route).

### The release job proves "no C compiler" with PATH shims (discovery)
- **Decision**: The single build job hides gcc, cc, clang and zig behind failing shims on `PATH`, sets `CGO_ENABLED=0`, and asserts `cc --version` fails before building.
- **Rationale**: Same pattern the xcl-python install check already uses; ubuntu runners ship gcc and cannot uninstall it cheaply.
- **Rejected**: A `golang:alpine` job container (extra apk setup for gpg, gh, zip; different from every other job).

### No XCL_CORE_PATH override (discovery)
- **Decision**: The engine program is always `xcl/_bin/xcl-core[.exe]` inside the package; `XCL_TRANSPORT` is the one selection override. The existing `XCL_LIBRARY_PATH` stays as the in-process route's development path.
- **Rationale**: The spec says which engine runs is decided by what the package holds and one explicit override.
- **Rejected**: A path override for the program (a second knob that changes which engine runs).

### Website Python install section on the home page (architecture)
- **Decision**: xcl.dev has no Python install section today; add one to `src/pages/index.mdx` (an Install section with Go and Python tabs, `pip install xcl-lang`, the bundled-subprocess sentence and the eight platforms with Alpine), and bump the hero badge to v0.5.0.
- **Rationale**: The acceptance criterion names "the xcl.dev Python install section"; the home page holds the site's only install block, and the getting-started guides belong to later docs specs.
- **Rejected**: A new `/python` page (navigation and guide structure are the docs specs' to decide).

### Release workflow renamed (architecture)
- **Decision**: `libxcl-release.yml` is replaced by `release.yml` (engine program) and a manual-only `libxcl-build.yml` (library matrix, artifacts only, never attached).
- **Rationale**: The release no longer builds libxcl; the name should say what it releases. The dispatch secret and signing secrets are reused unchanged.
- **Rejected**: Keeping the old file name (misleading) or deleting the library matrix (the spec keeps it buildable on demand).

### First Config() timing asserted in an integration test (testing_approach)
- **Decision**: The one-second success metric becomes an integration test timing the first `Config()` in a fresh interpreter on every CI platform.
- **Rationale**: The metric is stated for CI runners, so CI is where to assert it; a failure there is the signal the metric is missed.
- **Rejected**: A manual check (lets the metric drift silently); a looser bound (changes the metric).

### README example run kept manual; install check runs a self-contained program (testing_approach)
- **Decision**: The release install check runs a self-contained apply-and-read program on all eight platforms; running the README's Docker-plugin example is a manual check.
- **Rationale**: The README example needs a Docker engine, which only the Linux x86-64 runner has; the acceptance criterion's install check is fully automatable.
- **Rejected**: Changing the README example to avoid Docker (the README content is the package's, and the sibling spec owns the Docker replacement in tests).

### Task split and ordering (tasks)
- **Decision**: Eleven tasks: two in xcl for the protocol and program (M1); transport, selection-with-packaging-and-integration-tests, and CI in xcl-python (M2); the xcl release workflow, the xcl-python release, the Python README, the xcl.dev home page, the xcl changelog, and a human task to cut v0.5.0 (M3). xcl-python tasks depend on the xcl program tasks because CI and `make engine` build from the xcl checkout.
- **Rationale**: One repo per task, each independently verifiable; the release cannot be exercised until the program exists.
- **Rejected**: Folding the release workflow into the program task (mixes CI with code, harder to verify); a separate task for `Service.Close` (too small, only meaningful with the serve loop).

### Integration tests for lifecycle live in a new test module (tasks)
- **Decision**: Engine-process lifecycle tests go in a new `tests/integration/test_engine_process.py` marked `subprocess_only`; the existing tests are untouched and run on both routes.
- **Rationale**: Keeps the sibling spec's edits to existing test files and fixtures conflict-light, and keeps route-specific tests in one place.
- **Rejected**: Spreading lifecycle tests through `test_sync.py`.

### Version-mismatch integration test builds a second stamped program (tasks)
- **Decision**: The "different version refused" test builds `xcl-core` stamped `v9.9.9` from `XCL_SRC` into a temporary package copy, skipping when Go or the checkout is absent; the fake-engine unit test covers the ABI half everywhere.
- **Rationale**: Exercises the real handshake without shipping a test-only flag in the engine.
- **Rejected**: A hidden `--fake-version` flag on `xcl-core` (test hook in a shipped binary).

### Engine skips are strict-mode aware (cross-plan, settled by the user)
- **Decision**: A missing-engine skip goes through `tests/engines.py::skip_missing_engine(route, reason)`. It fails under `XCL_TEST_STRICT=1` only when the route is listed in `XCL_TEST_ENGINES`, the routes the CI job builds (`subprocess` everywhere, plus `inprocess` on the Linux job that builds the library). Every CI and release test step this plan touches sets both variables.
- **Rationale**: Agreed with the sibling release-hardening plan, whose `strict_skip` turns harness skips into failures in CI. Route skips for an engine the job does not build must stay skips.
- **Rejected**: Failing every engine skip under strict mode (would fail the in-process route on jobs that never build the library); leaving engine skips outside strict mode (a job that should have built the program could go green with skipped tests).

### Windows interrupts: Ctrl-Break to a new process group; the engine discards it (cross-plan, settled by the user)
- **Decision**: `xcl-core` catches and discards `os.Interrupt`, which on Windows covers both `CTRL_C_EVENT` and `CTRL_BREAK_EVENT`, using `signal.Notify` rather than `signal.Ignore`. Its smoke test sends `CTRL_BREAK_EVENT` to a `CREATE_NEW_PROCESS_GROUP` child on Windows. The Python package's Windows interrupt coverage is the sibling plan's keyboard-interrupt test.
- **Rationale**: Both plans agreed on this mechanism for testing interrupts on Windows. An ignored console control event on Windows falls through to the default handler, which would end the engine.
- **Rejected**: Leaving Windows interrupts untested.

## Rehydration cues

- `spektacular design read --data '{"source":"design","path":"libxcl.md"}'` — sections "The transport: xcl-core over stdio", "Starting the engine", "Choosing the transport", "Shipping and packaging".
- `spektacular spec file read 20261011211634-a8895873-python-out-of-process-engine`; sibling `20261011232013-da18c586-python-release-hardening`.
- Re-read `xcl/coreservice/service.go`, `xcl/bindings/libxcl/main.go`, `xcl/.github/workflows/libxcl-release.yml`, `xcl-python/xcl/_transport/inprocess.py`, `xcl-python/xcl/_client.py`, `xcl-python/.github/workflows/release.yml`.
- `spektacular knowledge always-applied --tier repo --filter xcl` for conventions.

