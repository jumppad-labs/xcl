---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Context: 20261031080136-264e0eab-typescript-core-sdk

## Current State Analysis

`xcl-typescript` holds only LICENSE, a two-line README and its `.spektacular` footprint, so every file in it is new. The behaviour to match lives in `xcl-python`, at engine pin `v0.13.1` in `xcl-python:xcl-ref.txt`, whose transport, poll loop, schema derivation and hydration are the model (`xcl-python:xcl/_transport/subprocess.py`, `xcl/_poll.py`, `xcl/aio.py`, `xcl/_schema.py`). The engine program `xcl:cmd/xcl-core/main.go` already behaves as the designs need: end of input or SIGTERM releases every configuration and exits, and interrupts are discarded. `xcl:proto/xcl/core/v1/core.proto` has no changes since `v0.13.1`. xcl's release dispatches only xcl-python (`xcl:.github/workflows/release.yml:201-250`), and the bindings gate calls only Python's suite (`xcl:.github/workflows/bindings.yml:32-41`). xcl's release already builds all eight engine archives.

## Per-Task Technical Notes

### Task: Set up the package and generate its messages

**File changes**:
- `xcl-typescript:package.json` (new): name `xcl-lang`, `type: module`, `engines.node >=22`, `exports` for `.` with `types` and `import`, `files: [dist, README.md, PARITY.md]`, `dependencies` `@bufbuild/protobuf` pinned exactly, dev dependencies `typescript`, `@bufbuild/buf`, `@bufbuild/protoc-gen-es`, `@types/node`, all pinned exactly. `optionalDependencies` are added in the platform-packages task.
- `xcl-typescript:tsconfig.json` (new): strict, `target: ES2022`, `module: NodeNext`, `declaration: true`, `outDir: dist`, `rootDir: src`, `lib` with `esnext.disposable`.
- `xcl-typescript:tsconfig.test.json` (new): compiles `src` and `test` to `build/test` with standard decorators.
- `xcl-typescript:buf.gen.yaml` (new): `local: protoc-gen-es`, `target=ts`, `import_extension=js`, out `src/internal/core`.
- `xcl-typescript:Makefile` (new): targets `protos`, `engine`, `plugins`, `test`, `test-unit`, modelled on `xcl-python:Makefile:1-56`, with `XCL_SRC ?= ../xcl`. `engine` runs xcl's `make xcl-core` with `XCL_CORE_VERSION` set to the value in `src/version.ts` and `XCL_CORE_OUT=build/engine/xcl-core[.exe]`. `plugins` builds `xcl-plugin-slow`, `xcl-plugin-recording` and `parity` into `build/plugins`, as `xcl-python:Makefile` does.
- `xcl-typescript:xcl-ref.txt` (new): `v0.13.1`.
- `xcl-typescript:src/version.ts` (new): `ENGINE_VERSION = "v0.0.0-dev"`, `PACKAGE_VERSION`; the release rewrites both (see `xcl-python:xcl/_version.py:1-16`).
- `xcl-typescript:src/internal/core/` (new, generated and committed): from `proto/xcl/core/v1/core.proto` and `transport.proto`.
- `xcl-typescript:src/index.ts` (new): empty public entry point, filled by later tasks.
- `xcl-typescript:.gitignore` (new): `dist`, `build`, `node_modules`.
- `xcl-typescript:test/keys/jumppad-labs.asc` (new): the release public key, copied from `xcl-python:tests/keys/jumppad-labs.asc`.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Add the error classes

**File changes**:
- `xcl-typescript:src/errors.ts` (new): `XclError` with `code`, `message`, `details`, `problems`, all defined as enumerable; a subclass for each of the codes in `xcl:coreservice/errors.go:17-44`; `SchemaError`, `EngineNotFoundError` (names the package and install command), `EngineVersionError` (both versions), `EngineExitedError` (`code`, `signal`), `InternalBindingError`. `fromProto(error)`, mirroring `xcl-python:xcl/_errors.py` (`from_proto`, `_problem`).
- `xcl-typescript:test/unit/errors.test.ts` (new): one test per error class's mapping, the unknown code, the problem fields, and enumerability.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Run and frame the engine

**File changes**:
- `xcl-typescript:src/internal/transport.ts` (new): the `Transport` interface; `STATUS_*`; a two's-complement status helper (`xcl-python:xcl/_transport/subprocess.py:85-87`).
- `xcl-typescript:src/internal/subprocess.ts` (new): engine location (`XCL_CORE_PATH`, then `createRequire(import.meta.url).resolve('<package>/package.json')`, package chosen from `process.platform`, `process.arch` and, on Linux, `process.report.getReport().header.glibcVersionRuntime`). `SubprocessTransport` modelled on `xcl-python:xcl/_transport/subprocess.py:126-345` with no threads: `spawn(path, ['--serve'], {stdio: ['pipe','pipe','inherit']})`, a buffering frame reader on `stdout` `data`, a pending-call map by `bigint` id, the handle map, `creating` count, generation. `outstanding` counter drives `child.ref()/unref()` and the pipes' `ref()/unref()` (cast to `net.Socket`). `error` handlers on `stdin` and `stdout` so a broken pipe never throws. `close`: `SIGTERM`, bounded wait of 5 s, then `SIGKILL`. The `Release` frame uses the reserved method name (`xcl:cmd/xcl-core/serve.go`).
- `xcl-typescript:test/helpers/fake-engine.ts` (new): the frame protocol over stdio, as `xcl-python:tests/fake_engine.py:1-164`.
- `xcl-typescript:test/unit/subprocess.test.ts` (new): framing with split and joined chunks, out-of-order responses, status sign, `ref`/`unref`, exit reporting, restart after exit, stale handle, locator selection per platform and libc, the missing-package message, a spawn failure.

**Complexity**: High
**Token estimate**: ~45k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Add the typed client and version check

**File changes**:
- `xcl-typescript:src/internal/client.ts` (new): `Client` from the generated `CoreService` descriptor, as `xcl-python:xcl/_client.py:34-117`: `call`, `release`, `engineVersion`, once-only `GetVersion` check of `abi_version === 1` and the version against `ENGINE_VERSION`, with a leading `v` ignored.
- `xcl-typescript:test/helpers/fake-transport.ts` (new): a scripted `Transport`.
- `xcl-typescript:test/unit/client.test.ts` (new): one test per behaviour above.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Create configurations and run operations

**File changes**:
- `xcl-typescript:src/hydrate.ts` (new): the name rule (`toSnake`, `toCamel`, `roundTrips`); `Meta` interface; the generic `Entity` view holding raw JSON, top-level attributes camelCase, `as()`. Output-name and diff-path conversion helpers (attribute segments only; `xcl:entity/path.go`).
- `xcl-typescript:src/registry.ts` (new): `LocalRegistry` (`registerExternalPlugin` taking a path or argv, `registerPluginDirectory`, `pluginPattern`), `GitHubRegistry` (`trustedKeys`, `cacheDir`, `token`, `apiUrl`, `registerPlugin`), as `xcl-python:xcl/registry.py`.
- `xcl-typescript:src/internal/poll.ts` (new): `Operation` and the drive loop, as `xcl-python:xcl/_poll.py:78-130` and `xcl/aio.py:151-179`, with `POLL_WAIT_MS = 1000`, ending on `finished`. Task dispatch is extended by later tasks.
- `xcl-typescript:src/diff.ts` (new): `Diff`, `ResourceDiff`, `Change`, `Summary`, `UNKNOWN` symbol, from `xcl-python:xcl/diff.py`.
- `xcl-typescript:src/config.ts` (new): `createConfig`, `Config` type, `validate`, `apply`, `diff` (with `revealSensitive`), `load`, `destroy`, `close`, `[Symbol.asyncDispose]`, the operation guard, generic `find`, `findByType`, `findOne`, `entities`, `entityCount`, `outputs`, `statePath`.
- `xcl-typescript:src/index.ts`: exports for the above.
- `xcl-typescript:test/unit/config.test.ts`, `xcl-typescript:test/unit/registry.test.ts`, `xcl-typescript:test/unit/diff.test.ts`, `xcl-typescript:test/unit/names.test.ts` (new): against the fake transport.

**Complexity**: High
**Token estimate**: ~40k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Test engine lifecycle and Go plugins against a real engine

**File changes**:
- `xcl-typescript:test/helpers/engine.ts` (new): engine path, strict mode (`XCL_TEST_STRICT`), as `xcl-python:tests/engines.py` and `tests/xcl_source.py`.
- `xcl-typescript:test/helpers/plugins.ts` (new): test plugin pin `jumppad-labs/xcl-plugin-test` `v0.3.1` as a constant (`xcl-python:tests/plugins.toml`), install through `GitHubRegistry`, `go_platform`.
- `xcl-typescript:test/helpers/fake-github.ts` (new): `http.createServer` serving a release, as `xcl-python:tests/integration/test_github_tamper.py:40-200`.
- `xcl-typescript:test/helpers/script.ts` (new): runs a scenario in a fresh Node process, reading `key:value` lines, as `xcl-python:tests/integration/test_engine_process.py:1-60`; process-alive checks for each platform.
- `xcl-typescript:test/integration/plugins.test.ts`, `xcl-typescript:test/integration/github-tamper.test.ts`, `xcl-typescript:test/integration/engine-process.test.ts`, `xcl-typescript:test/integration/operations.test.ts` (new).

**Complexity**: High
**Token estimate**: ~40k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Add the entity decorators in both compiler modes

**File changes**:
- `xcl-typescript:src/decorators.ts` (new): `isStandard(args)` on whether the second argument is a context object; field table in a `WeakMap<Function, FieldSpec[]>`; legacy records on `target.constructor`, standard through `context.addInitializer` on `this.constructor`, idempotent; `@Entity` stores type and subtype. Overloaded decorator types for both forms. `Int`, `Any` type markers.
- `xcl-typescript:src/sensitive.ts` (new): the `Sensitive` value and decorator factory split by `new.target`, `toString`, `toJSON`, `[util.inspect.custom]`, `reveal`, `redacted`, `isRedacted`. Behaviour from `xcl-python:xcl/_types.py:13-55`.
- `xcl-typescript:test/decorators/entities.ts` (new): shared class definitions.
- `xcl-typescript:tsconfig.decorators-standard.json`, `xcl-typescript:tsconfig.decorators-legacy.json` (new): compile the shared classes and the suite to two outputs.
- `xcl-typescript:test/decorators/fields.test.ts` (new): runs against each output.
- `xcl-typescript:test/unit/sensitive.test.ts` (new).

**Complexity**: High
**Token estimate**: ~30k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Derive schemas at registration

**File changes**:
- `xcl-typescript:src/schema.ts` (new): `schemaFor(ctor)`, following the output rules of `xcl:internal/schema/v1.go` and `xcl-python:xcl/_schema.py:59-220`: `computed` carries `nullable`; a no-initializer optional is `optional` + `nullable`; optional sensitive is `optional` only; `Number` is `format: float`, `Int` `format: int`; an empty array or object initializer gives `optional` with no `default`; a non-empty one a `default`; a primitive initializer including `""` a `default`. `meta`, `disabled`, `depends_on` reserved.
- `xcl-typescript:src/registry.ts`: `LocalRegistry.register`, `GitHubRegistry.register`, `Registration`, `DeclaredType` in the proto description.
- `xcl-typescript:test/unit/schema.test.ts` (new): one test per schema form and per refusal.

**Complexity**: High
**Token estimate**: ~30k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Return registered classes from lookups

**File changes**:
- `xcl-typescript:src/hydrate.ts`: `hydrate`, `_hydrateInto`, a `WeakMap` of source JSON, `ClassMap` (`xcl-python:xcl/_classes.py`, `xcl/_schema.py:467-594`). `Entity.as`.
- `xcl-typescript:src/config.ts`: class overloads of `find`, `findByType`, `findOne`, `all`; `entities` through the class map.
- `xcl-typescript:test/unit/hydrate.test.ts` (new), `xcl-typescript:test/integration/entities.test.ts` (new): the spec's type-rule, label, singleton and lookup criteria.
- `xcl-typescript:test/integration/typed-decorators.test.ts` (new): the same configuration applied under each decorator mode's classes.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Call TypeScript functions from configuration

**File changes**:
- `xcl-typescript:src/decorators.ts`: `@Func(params, result, options?)`, in a `WeakMap` keyed on the method.
- `xcl-typescript:src/registry.ts`: `registerFunctions(instance)`, walking the prototype chain and binding each method; duplicates are kept in the list, because the engine reports the clash (`xcl:coreservice/config.go:168`). `DeclaredFunction` signature JSON, as `xcl-python:xcl/_schema.py:394-449`.
- `xcl-typescript:src/internal/poll.ts`: `function` task handling, `FunctionResult`; a late `invalid_argument` from `Complete` is ignored.
- `xcl-typescript:test/unit/functions.test.ts` (new), `xcl-typescript:test/integration/functions.test.ts` (new).

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Pass variables and encode entities as configuration

**File changes**:
- `xcl-typescript:src/hydrate.ts`: `recordOf`, `recordBytes`, mirroring `xcl-python:xcl/_schema.py:601-763`, with a `Meta` to record conversion from `xcl/_entity.py:57-76`.
- `xcl-typescript:src/config.ts`: `encodeEntity`, `encodeSavedEntity`, `variables` with the name rule on top-level keys, `EncodeOptions` to proto.
- `xcl-typescript:test/unit/record.test.ts` (new), `xcl-typescript:test/integration/encoding.test.ts` (new).

**Complexity**: Medium
**Token estimate**: ~22k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Compare results with Go on the parity fixtures

**File changes**:
- `xcl-typescript:test/parity/run.ts` (new): the record stages of `xcl-python:tests/parity/run_python.py`, and the `Normaliser`, less the printing keys.
- `xcl-typescript:test/parity/types.ts` (new): one class per fixture type, as `xcl-python:tests/parity/types.py`, and the hand-built `encode` entity.
- `xcl-typescript:test/parity/parity.test.ts` (new): one test per fixture in `xcl:bindings/libxcl/testdata/parity`, `firstDifference` from `xcl-python:tests/parity/test_parity.py:16-55`.
- `xcl-typescript:test/parity/types.test.ts` (new): one test per fixture.
- `xcl-typescript:test/parity/normaliser.test.ts` (new).

**Complexity**: High
**Token estimate**: ~35k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Deliver events to TypeScript handlers

**File changes**:
- `xcl-typescript:src/events.ts` (new): `Event`, `EventDataLevel`, from `xcl-python:xcl/events.py`.
- `xcl-typescript:src/internal/poll.ts`: the event queue (`xcl-python:xcl/aio.py:206-218`).
- `xcl-typescript:src/config.ts`: `onEvent`, `eventData`, `eventBufferSize`, `eventMask`, `noEventMask`.
- `xcl-typescript:test/unit/events.test.ts` (new), `xcl-typescript:test/integration/events.test.ts` (new): asserting on event data as `xcl-python`'s `gotchas/aio-state-tasks-run-beside-handlers.md` advises.

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Support custom state stores and masks

**File changes**:
- `xcl-typescript:src/mask.ts` (new): `aes256gcm`, `hmacSha256`, `redact`, `omit`, from `xcl-python:xcl/mask.py`.
- `xcl-typescript:src/internal/poll.ts`: `state` task handling, `StateResult`, as `xcl-python:xcl/aio.py:462-483`.
- `xcl-typescript:src/config.ts`: `stateStore`, `stateMask`, refusing both `statePath` and `stateStore`.
- `xcl-typescript:test/unit/state.test.ts`, `xcl-typescript:test/integration/state.test.ts` (new), decoding state files rather than matching strings (`xcl` `gotchas/file-state-is-indented-json.md`).

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Cancel operations with an AbortSignal and Ctrl-C

**File changes**:
- `xcl-typescript:src/interrupt.ts` (new): `interruptSignal()`, a `WeakMap` of signal to release function, `process.once('SIGINT')`, removal in `finally`, and for `AbortSignal.any` on fire or process exit.
- `xcl-typescript:src/internal/poll.ts`: abort listener calling `Cancel` immediately; the `cause` set from `signal.reason`; no wait for outstanding work after a cancel.
- `xcl-typescript:src/config.ts`: `signal` on `validate`, `apply`, `diff`, `load`, `destroy`.
- `xcl-typescript:test/unit/cancel.test.ts`, `xcl-typescript:test/unit/interrupt.test.ts` (new).
- `xcl-typescript:test/integration/cancel.test.ts`, `xcl-typescript:test/integration/interrupt.test.ts` (new): a slow-plugin gate before a `test "item"` that must not be recorded; signals through a fresh process's group, POSIX only (`xcl-python:tests/integration/test_engine_process.py:530`).

**Complexity**: High
**Token estimate**: ~25k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Prove workers and a responsive event loop

**File changes**:
- `xcl-typescript:test/integration/workers.test.ts` (new).
- `xcl-typescript:test/integration/event-loop.test.ts` (new): `perf_hooks.monitorEventLoopDelay`, `xcl-plugin-slow`, as `xcl-python:tests/integration/test_aio.py:344-420`.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Assemble the platform packages

**File changes**:
- `xcl-typescript:npm/xcl-lang-linux-x64-gnu/package.json` and the seven others (`-linux-x64-musl`, `-linux-arm64-gnu`, `-linux-arm64-musl`, `-darwin-x64`, `-darwin-arm64`, `-win32-x64`, `-win32-arm64`) (new): `os`, `cpu`, `libc`, `preferUnplugged: true`, `files: ["bin"]`.
- `xcl-typescript:package.json`: `optionalDependencies`.
- `xcl-typescript:scripts/assemble.mjs` (new): takes a directory of verified engine archives and a version; sets versions, `src/version.ts`, copies each binary to `npm/<pkg>/bin/` with mode 755, runs `npm pack`.
- `xcl-typescript:scripts/assemble.test.mjs` (new).

**Complexity**: Medium
**Token estimate**: ~15k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Add the test workflow on every platform

**File changes**:
- `xcl-typescript:.github/workflows/test.yml` (new): `workflow_call` with `typescript-ref` and `xcl-ref`, modelled on `xcl-python:.github/workflows/test.yml:1-224`. Matrix `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-latest`, `macos-15-intel`, `windows-latest`, `windows-11-arm`, Node `22`, `24`, `26`; `node:22-alpine` by hand on both architectures, as xcl-python does.
- `xcl-typescript:.github/workflows/ci.yml` (new): as `xcl-python:.github/workflows/ci.yml`.
- `xcl-typescript:examples/getting-started/` (new): applied in CI against the test plugin, and type-checked in a strict project; also used by the install check.

**Complexity**: Medium
**Token estimate**: ~18k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Add the bump, tag and release workflows

**File changes**:
- `xcl-typescript:.github/workflows/engine-bump.yml` (new): `xcl-python:.github/workflows/engine-bump.yml` without labels.
- `xcl-typescript:.github/workflows/tag.yml` (new): on push to `main` with `paths: [xcl-ref.txt]`, pushes the tag in `xcl-ref.txt` using the release bot App token, if it does not exist.
- `xcl-typescript:.github/workflows/release.yml` (new): jobs `settings`, `assemble` (one Linux job: download, `gpg --verify` against `test/keys/jumppad-labs.asc`, `sha256sum -c` as `xcl-python:.github/workflows/release.yml:113-138`, `scripts/assemble.mjs`), `verify` (every platform and Alpine, install the tarballs with compilers hidden, parity), `publish` (environment `npm`, `id-token: write`, npm 11.5.1 or later, platform packages then `xcl-lang`, `--provenance`), `install-check` (from the registry), `github-release`.
- `xcl-typescript:.github/scripts/install-check.mjs` (new): runs the getting-started flow and asserts the engine reports the tag, as `xcl-python:.github/scripts/install_check.py`.

**Complexity**: High
**Token estimate**: ~25k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Write the README, parity record and doc comments

**File changes**:
- `xcl-typescript:README.md`: replaced.
- `xcl-typescript:PARITY.md` (new): from `xcl-python:PARITY.md`, with printing rows marked for the printing spec.
- `xcl-typescript:src/`: doc comments on every export.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Dispatch and test the TypeScript package from xcl

**File changes**:
- `xcl:.github/workflows/release.yml:195-250`: a new job `dispatch-typescript` needing `settings` and `build`, using a new secret `XCL_TYPESCRIPT_DISPATCH_TOKEN`, repeating the size step of `dispatch`. Header comment names both bindings.
- `xcl:.github/workflows/bindings.yml:32-41`: a `typescript` job calling `jumppad-labs/xcl-typescript/.github/workflows/test.yml@main` with `typescript-ref: main` and `xcl-ref` from the pull request head; added to `bindings` `needs`. Header comment updated.

**Note**: Merge after the `xcl-typescript` test workflow is on its `main` and the dispatch token exists, or xcl pull requests and the next release fail. The release plan orders this.

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Correct the SDK design document

**File changes**:
- `xcl:.spektacular/design/typescript-sdk.md`: edited only through the `spek-design` skill or `spektacular design` commands, in sections "Lookups" and "Releases".

**Complexity**: Low
**Token estimate**: ~3k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Set up npm publishing and GitHub secrets

**What to do**: npm trusted publishing can only be configured on a package that already exists, so publish each of `xcl-lang` and the eight `xcl-lang-<os>-<arch>[-<libc>]` names once as `0.0.0-placeholder` under the organisation's account, then set each one's trusted publisher to repository `jumppad-labs/xcl-typescript`, workflow `release.yml`, environment `npm`. On GitHub: install the release bot App on `xcl-typescript`; add `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` as repository secrets; enable auto-merge and require the `result` check on `main`; add `XCL_TYPESCRIPT_DISPATCH_TOKEN` to `jumppad-labs/xcl` (contents read, actions write on `xcl-typescript`). Checked by running `engine-bump.yml` by hand for the current tag and seeing the release through.

## Testing Strategy

Per-task detail: unit tests and fakes are named in each task's file changes. The real-engine suites need `make engine plugins` from an xcl checkout, `XCL_CORE_PATH` set, and `GITHUB_TOKEN` to stay within GitHub's rate limit for the test plugin's release (`xcl-python` knowledge `learnings/test-suite-in-a-spec-worktree.md`). A run with `XCL_TEST_STRICT=1` fails instead of skipping when an engine, plugin or fixture is missing, as CI always does. After changing xcl's `bindings/libxcl/cmd/parity`, rebuild the parity program, or the comparison uses the old Go record.

Tests sit in three layers, following xcl-python's arrangement and the project's rules: one behaviour per test, no table-driven tests, positive and negative cases in separate tests, verbose over abstract. They use Node's built-in test runner.

**Unit tests, no engine.** These take most of the coverage, because the logic that is new to this SDK lives here. They guarantee that:

- framing handles split and joined chunks, out-of-order responses and two's-complement statuses;
- the client maps every error code to its class;
- `ref` and `unref` follow outstanding calls;
- engine exit fails every waiter, with its status, and a later use starts afresh;
- the name rule round-trips and refuses what would not;
- every decorator form records the same field table;
- schema derivation and every refusal at registration behave as designed;
- hydration and record building mirror each other;
- the poll driver runs tasks concurrently, delivers events in order, sends `Cancel` at once on abort and rejects without waiting on stuck promises;
- `interruptSignal` adds its listener only when called and removes it when the operation settles.

A separate suite runs the same entity classes compiled under each decorator mode.

**Integration tests, real engine.** Each runs against an engine built from an xcl checkout, with the fixture plugins and the signed test-plugin release. Scenarios that depend on process state run in a fresh Node process. They guarantee every behaviour in the spec's acceptance criteria that a program can observe: lookups and typed entities, type rules, function calls, plugins from a path, a directory and the GitHub registry, tamper and untrusted-key refusals, state directory and custom stores, encryption, events at each level, typed errors, cancellation, Ctrl-C with and without opting in, one engine per process, a killed host, a killed engine, a script that never closes, stderr passing through, the missing engine, the version mismatch, worker threads and cluster and child-process workers. Signal tests run on Linux and macOS, because Node cannot send a console Ctrl-C to a child on Windows.

**Parity tests.** A Node program runs each of xcl's parity fixtures and writes a record, which is compared with the Go program's record for the same fixture. This guarantees the same validation errors, diff, entities, outputs, state contents, load and destroy, and byte-for-byte configuration text for every entity under all sixteen combinations of the encode choices. A separate check holds each fixture's classes to its `types.json`, so a derived schema equals Go's. Printing keys are left out here and belong to the printing spec.

**Packaging checks in CI.** Tarballs of all nine packages are installed on every platform with no Go or C compiler on the path. The parity suite and an install check then run against what was installed. On Linux the installed package also runs with networking off, inside a container.

**Deliberate gaps.** No test reads the README, the parity record, doc comments or workflow files, as the project's testing rules require. Those are checked by people below. Nothing in the package's own release workflow is tested by an automated test, and a first real release proves it.

**Success metrics.**

- A developer goes from installing to a working apply against a Go plugin by following the README alone: **Manual — captured in the implementation test plan**.
- Every Go operation and lookup has a TypeScript equivalent, checked against the parity record at each release: **Manual — captured in the implementation test plan**.
- The Go parity fixtures show zero differences on every release, on all eight platforms: behavioural test. The parity suite runs against each platform's tarballs in the release, before anything is published, and a difference fails the release.
- A web service keeps serving during a long apply, with event-loop delay under 50 ms at the 99th percentile: behavioural test. An HTTP server in the same process answers throughout a slow apply, and the p99 from Node's event-loop delay monitor is asserted.
- The first configuration in a process is created within one second on the CI runners: behavioural test, a time assertion in a fresh process.
- No open issue reports an orphaned engine process after a Node program exits: **Manual — captured in the implementation test plan** (issue tracker watched after release). The behaviour itself is covered by the engine-cleanup tests on all eight platforms.

**Manual reviews.**

- **Manual — captured in the implementation test plan**: the first release works end to end: an xcl release tag leads with no manual publish to nine packages on npm at the same version with provenance, and that version's engine reports the tag.
- **Manual — captured in the implementation test plan**: a deliberately parity-breaking change to xcl, and to the package, fails its run.
- **Manual — captured in the implementation test plan**: on a fresh macOS and Windows machine with the network disconnected after install, a script that imports the package and creates and closes a configuration runs on Node 22, 24 and 26.
- **Manual — captured in the implementation test plan**: Ctrl-C on Windows, with and without opting in.
- **Manual — captured in the implementation test plan**: the npm page shows the README with install and a getting started that applies a configuration using a Go plugin, and the README's example matches the checked example.
- **Manual — captured in the implementation test plan**: the parity record lists every Go operation and lookup with its TypeScript equivalent.
- **Manual — captured in the implementation test plan**: every public export has a doc comment.
- **Manual — captured in the implementation test plan**: installing with npm, pnpm, yarn and bun picks the right platform package, including on Alpine, and a package manager that skips optional dependencies produces the named error.

## Project References

- Spec `20261031080136-264e0eab-typescript-core-sdk`, epic `20261031080131-8c2a58ec-typescript-sdk`.
- Designs from source `design`: `typescript-sdk.md`, `typescript-entity-types.md`, `libxcl.md`.
- Knowledge, store xcl: `architecture/release-chain.md`, `decisions/one-package-per-language.md`, `conventions/testing-and-mocking.md`, `conventions/config-files-are-xcl.md`, `conventions/no-compatibility-before-stable.md`, `gotchas/file-state-is-indented-json.md`.
- Knowledge, store xcl-python: `gotchas/ci-regenerates-stubs.md`, `gotchas/install-check-runs-after-publish.md`, `gotchas/aio-state-tasks-run-beside-handlers.md`, `learnings/test-suite-in-a-spec-worktree.md`.
- Repo roots: `xcl-typescript` at /home/nicj/code/github.com/jumppad-labs/xcl-typescript, `xcl` at /home/nicj/code/github.com/jumppad-labs/xcl, `xcl-python` at /home/nicj/code/github.com/jumppad-labs/xcl-python (read only, the model), `xcl-plugin-test` at /home/nicj/code/github.com/jumppad-labs/xcl-plugin-test (read only).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

None. This is a new package, and the changes to xcl add jobs. An engine bump that removes a protocol field needs the package's change on the bump branch (`xcl-python` knowledge `gotchas/ci-regenerates-stubs.md`).

## Performance Considerations

Event-loop delay must stay under 50 ms at the 99th percentile during an apply, and the first configuration must be created within one second on CI, both asserted by tests. A pipe round trip is tens of microseconds (`libxcl.md`, "What it costs"), so per-call overhead is not a concern. Very large integers in entity JSON lose precision as JavaScript numbers; an xcl value beyond 2^53 is rare and is not handled separately.
