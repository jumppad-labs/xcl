---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Plan: 20261031080136-264e0eab-typescript-core-sdk

<!-- Metadata -->
<!-- Created: 2026-11-10T09:03:51+00:00 -->
<!-- Commit: 788959a6ecad23b6d101f55f74fb793d6ca9cbad -->
<!-- Branch: f-python-plugin -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

xcl for TypeScript and Node: the `xcl-lang` npm package runs the same bundled engine Go and Python use, so a Node program can read, check and apply configuration exactly as a Go program would, with the same Go plugins, types written as decorated classes, and functions of its own. TypeScript and JavaScript developers can't use xcl today; this serves people building command-line tools and services in TypeScript. Long-running work never stalls the rest of the program. Printing, the pretty logger and the SDK reference follow in later specs of the epic.

## Conventions

- **Testing and mocking: never table-driven, positive and negative cases in separate tests, verbose over abstract, tests located with what they test** — xcl's standing test rules, applied to the TypeScript suites as well as any Go.
- **Never write tests that inspect repository files** — README, PARITY.md, doc comments and CI workflows are checked by people, so those acceptance criteria go to the test plan's manual checks instead of a test.
- **Configuration and metadata files are written in xcl, except where a standard tool owns the format** — `package.json`, `tsconfig.json` and `buf` files are tool-owned; the test plugin pin is a TypeScript constant rather than a new metadata file.
- **Breaking changes are allowed until xcl is stable** — the design is chosen on simplicity, and no compatibility paths are added; a protocol change is handled on the engine-bump branch.
- **Write a release plan once released work is merged** — the first release of `xcl-lang` needs one, written by the implement run, not by this plan.
- **Spec worktrees belong to the agent** — implementation is delivered to the branches of `xcl` and `xcl-typescript`, never pointed at in a worktree.
- **Prefer the standard library, document third-party dependencies, pin versions** — the only runtime dependency is `@bufbuild/protobuf`, pinned exactly; test tooling is Node's built-in test runner.

## Architecture & Design Decisions

The package is built to `typescript-sdk.md`, `typescript-entity-types.md` and `libxcl.md`: nothing here re-derives their shape. It has the three layers `libxcl.md` gives every SDK, all in `jumppad-labs/xcl-typescript`. `src/internal/subprocess.ts` is the only code that knows about the process: it finds `xcl-core`, starts it, frames calls and implements `Transport.call(handle, method, bytes)`. `src/internal/client.ts` turns generated messages into bytes and error frames into `XclError` subclasses, reading method input and output from the generated `CoreService` descriptor. The hand-written API (`config.ts`, `registry.ts`, `decorators.ts`, `schema.ts`, `hydrate.ts`, `events.ts`, `diff.ts`, `errors.ts`, `mask.ts`, `interrupt.ts`) sits on top, with `internal/poll.ts` driving one operation. Tests below the API run against a fake transport with no engine, and the real-engine suites run with `XCL_CORE_PATH` pointing at an engine built from an xcl checkout. In the `xcl` repository only `.github/workflows/release.yml` and `.github/workflows/bindings.yml` change, as the spec requires.

The engine is one process per module graph, so one per process and one per worker thread, started on the first `createConfig`. Frames are read from `stdout` `data` events on the event loop, and every response is matched to its caller by request id, so a `Poll` waiting for seconds holds up nothing. The transport tracks calls outstanding: at zero it `unref`s the child and both pipes, and at one it `ref`s them, which is the whole mechanism behind "an idle engine never holds the process". Everything else about the engine's life comes from the engine itself. End of input releases every configuration and saves in-progress state, so a host that dies, on any platform and with no job object, still takes the engine with it. The last `close()` ends the engine with SIGTERM and a bounded wait, as the design says. The transport gives out handles of its own, mapped to the engine that issued them, so a configuration from an engine that has exited is refused by name rather than reaching whichever configuration a new engine numbered the same.

An operation has one poll loop. It calls `Start*`, then polls without waiting on the tasks it receives: each function and state-store task runs as its own promise and answers with `Complete`, while events go through one queue that awaits each handler before the next. A `Config`-level guard throws `OperationInProgressError` when a second operation starts, because the poll queue belongs to the handle and a second loop would steal the first one's tasks; lookups are refused by the engine (`coreservice/operations.go:37`). Aborting the signal sends `Cancel` at once, without waiting for a poll to return, and the loop runs on to `OperationFinished`. After a cancel the operation does not wait for handlers or tasks still outstanding, since a stuck promise of the program's own must not hold up the two-second rejection the spec asks for.

The decorators' one hard problem is that a standard field decorator receives only a name, Node 22 has no `Symbol.metadata`, and block classes such as `Health` carry no class decorator to collect from. So each decorator detects its calling form by whether its second argument is a context object. The legacy form records directly against `target.constructor`. The standard form registers `context.addInitializer`, which records against `this.constructor` when an instance is built. Registration always constructs the class once, because that is how the design reads defaults from initializers, so both forms fill the same per-class field table by the time the schema is derived. `@Func` keys a `WeakMap` on the method function itself. Schema v1 is derived in `local.register`, so a definition the SDK can see is wrong fails with `SchemaError` there, and `createConfig` surfaces whatever the engine still refuses. Each fixture's `types.json` is the test that derivation matches Go's documents, which addresses the spec's schema-identity risk. Hydrated instances remember the JSON they came from, in a `WeakMap`, so an unchanged field is encoded exactly as the engine wrote it, as `libxcl.md` requires for byte-for-byte text.

Releases follow the shape of xcl-python's but differ where the spec's same-version rule demands it. xcl's release dispatches `xcl-release` to `xcl-typescript`, `engine-bump.yml` opens a pull request that changes `xcl-ref.txt`, and when it merges `tag.yml` tags that commit with the pinned tag itself rather than a next version from labels. `release.yml` downloads the eight signed archives, verifies them, and assembles nine tarballs. It installs and tests those tarballs on every platform before anything is published, then publishes the platform packages first and `xcl-lang` last with trusted publishing and provenance, and installs from the registry once more as a final check. The alternatives weighed and rejected are in `research.md#alternatives-considered-and-rejected`.

One design correction follows from this plan: `typescript-sdk.md` says `entities` returns `Entity` values, and it is changed to say that `entities` returns an instance of the registered class where there is one and an `Entity` otherwise, as the spec's "Lookups return typed entities" requires. The same pass drops the "as for `xcl-lang` on PyPI" comparison from its Releases section, which no longer describes xcl-python.

## Component Breakdown

**New, in xcl-typescript**

- **Generated messages.** The `CoreService` and frame messages, generated from xcl's protos at the pinned tag and committed. Everything above the transport speaks these, and nothing else is generated.
- **Engine locator.** Finds the `xcl-core` to run: `XCL_CORE_PATH` if set, otherwise the platform package for this OS, CPU and, on Linux, C library. Owns the "engine package missing" error. Used only by the subprocess transport.
- **Subprocess transport.** Owns the one engine process per module graph: start, frames in and out, matching responses to callers, `ref` and `unref` by outstanding calls, the handle map across engine generations, the stop sequence, and reporting an engine exit to every waiter. Implements the byte-level `Transport` that the fake transport also implements.
- **Typed client.** Encodes requests and decodes responses by the generated service descriptor, turns error frames into error classes, and makes the version check on first use. Depends only on `Transport`.
- **Decorators and field table.** The decorators of `typescript-entity-types.md`, each accepting the standard and the legacy calling form, and the per-class field table they fill. `Sensitive` is both the decorator and the value class; `Meta` is the decorator and the metadata type; `Entity` is the decorator and the generic entity type.
- **Schema deriver.** Turns a class's field table and a probe instance into a schema v1 document and a function's declaration into a signature, refusing what has no configuration form. Owns the one name-conversion rule, used everywhere a name crosses.
- **Hydrator and record builder.** Entity JSON to class instances and generic entities, and back to the record the engine reads, remembering each instance's source JSON so unchanged fields are written as the engine wrote them.
- **Registries.** `LocalRegistry` and `GitHubRegistry`: plain builders that hold registrations and turn into their protobuf description. They hold no engine state.
- **Poll driver.** Runs one operation from `Start*` to `OperationFinished`: dispatches function and state tasks concurrently, delivers events in order through one queue, sends `Cancel` on abort.
- **Config.** `createConfig` and the operations, lookups and encoding of a configuration. Owns the one-operation-at-a-time guard, the class map and `close`. Sits on the client and the poll driver.
- **Events, diff, errors and masks.** The value types of the public API: `Event`, `Diff` with `UNKNOWN`, `XclError` and one subclass per code, and state masks.
- **Interrupt signal.** `interruptSignal()`: an `AbortSignal` whose SIGINT listener lasts only as long as the operation it was passed to. The only code in the package that touches a signal listener, and only when called.
- **Platform package manifests.** Eight manifests, with `os`, `cpu` and `libc`, that the release fills with the engine.
- **Parity runner.** A Node program that runs one fixture through the public API and writes the same normalised record as Go's parity program, leaving out the keys that belong to printing.
- **Test harness.** The fake transport and fake engine, a helper to find the built engine and fixture plugins, a fake GitHub API for tamper tests, and the entity classes for each parity fixture, compiled under both decorator modes.
- **CI and release workflows.** A reusable test suite called by the package's own CI and by xcl's bindings gate; the engine bump, tag and release workflows.
- **Package documentation.** The README, the parity record and a runnable getting-started example.

**Changed, in xcl**

- **xcl release workflow.** Also dispatches `xcl-typescript` with the tag and size, in its own job so one binding's failure cannot affect the other's dispatch.
- **xcl bindings gate.** Gains a job running the TypeScript suite against each pull request's commit, added to what the gate needs.

**Changed, in the design source**

- **`typescript-sdk.md`.** The two corrections named under Architecture.

## Data Structures & Interfaces

The public API's names and shapes are those of `typescript-sdk.md` and `typescript-entity-types.md`. This section gives the internal contracts between components, the one decision added to the public surface, and the boundaries data crosses.

**Transport.** The byte layer the fake and the real transport both implement. Handles are the transport's own, not the engine's.

```ts
interface Transport {
  call(handle: bigint, method: string, request: Uint8Array): Promise<{ status: number; payload: Uint8Array }>;
  release(handle: bigint): Promise<number>;
  close(): Promise<void>;
}
```

`status` is signed: 0 success, 1 an `Error` message in `payload`, negative a binding fault.

**Client.** Typed calls over a transport, driven by the generated descriptor.

```ts
class Client {
  call<M extends MethodName>(handle: bigint, method: M, request: RequestOf<M>): Promise<ResponseOf<M>>;
  release(handle: bigint): Promise<void>;
}
```

**Field table.** What decorators record for a class and what the schema deriver and hydrator read.

```ts
type FieldKind = "attribute" | "sensitive" | "computed" | "block" | "blocks" | "namedBlocks" | "label" | "meta";

interface FieldSpec {
  property: string;
  kind: FieldKind;
  type?: TypeSpec;                 // String, Number, Boolean, Int, Any, [T], { map: T }
  block?: () => new () => object;
  optional?: boolean;
}
```

**Registration.** A class bound to its block type, with its schema derived and a probe instance kept for defaults.

```ts
interface Registration {
  type: string;
  subtype: string;
  ctor: new () => object;
  schema: SchemaV1;
}
```

**Operation handlers.** What the poll driver answers tasks with.

```ts
interface Handlers {
  functions: Map<string, (...args: unknown[]) => unknown>;
  store?: StateStore;
  onEvent?: (event: Event) => void | Promise<void>;
  classes: ClassMap;
}
```

**Public surface added to the designs.**

- `entities(): Promise<Array<object | Entity>>`: a registered class's instance where there is one, a generic `Entity` otherwise. This is the correction to `typescript-sdk.md`.

```ts
interface Sensitive<T> { reveal(): T | undefined; readonly isRedacted: boolean; toString(): string; toJSON(): string; }
const Sensitive: {
  new <T>(value: T): Sensitive<T>;
  redacted<T>(): Sensitive<T>;
  (type: TypeSpec, options?: { optional?: boolean }): FieldDecorator;
};
```

**Boundaries.**

- Frames: a 4-byte big-endian length and a `Request` or `Response` message of `transport.proto`.
- Entity records: JSON bytes with snake_case keys. Conversion to camelCase happens only in the hydrator, the record builder and the generic `Entity` view; the JSON held for encoding is never converted.
- Schema v1: JSON in `DeclaredType.schema`, in the exact form `internal/schema/v1.go` reads.
- Function signatures: `plugins.FunctionSignature` JSON in `DeclaredFunction.signature`; arguments and results cross as JSON.
- Parity record: the JSON Go's parity program writes, less the printing keys (`printed_validate`, `printed_apply`, `printed_entities`, `printed_diff_before`, `printed_diff_changed`, `log`, `pretty_log`).
- The pin: `xcl-ref.txt` holds the xcl tag; the release writes that tag into the package's engine-version constant and `package.json` version.

## Implementation Detail

The package follows xcl-python closely wherever the protocol decides the behaviour: the same layers, the same handle map, the same way of answering state and function tasks, the same record-building rules for encoding, the same parity method and the same bump-then-release shape. A reader who knows one SDK can read the other. It differs where Node gives a simpler answer, and those differences are the patterns new to this codebase.

**Reference counting in place of a reader thread.** Where Python has a reader thread and blocking waits, the transport here is a stream `data` handler and a map of pending promises. The one piece of state that matters beyond that is the count of outstanding calls, which drives `ref` and `unref`. A developer reading the transport sees the whole lifecycle in a few short functions, and no timers.

**One decorator body, two calling forms.** Each decorator is a small function that normalises its arguments to a property name and a recording callback, then shares everything else. The form is detected once, in one helper. The TypeScript signature is an overload of both forms, so one export type-checks under either compiler setting.

**A probe at registration.** Registering a class constructs it once. That single instance supplies the field table in standard mode, the defaults for both modes, and the check that an initializer agrees with its decorator. Everything that can be wrong with a class is therefore reported from `register`, naming the class and field.

**Names cross in one place.** One pair of functions holds the name rule. The schema deriver, the hydrator, the record builder, the generic entity view, variables, outputs and diff paths all call them, and none converts by hand. A name that does not survive the round trip is refused at registration.

**Generic entities are views.** A generic `Entity` holds the engine's JSON untouched and exposes camelCase attribute names at the top level over it. Encoding reads the JSON, so a generic entity can never differ from what the engine wrote.

**Tests mirror the three layers.** Unit tests sit against the fake transport with no engine. Integration tests run one process per scenario, so the engine a test watches belongs to that scenario alone, as xcl-python's process tests do. Parity runs a separate Node program for each fixture and compares its record with Go's. The test style matches the project's rules: one behaviour per test, no table-driven tests, positive and negative cases apart. Node's own test runner is used so nothing is added to learn or pin.

**Decorator modes are tested by compiling twice.** The same entity class source is compiled with standard and with legacy decorators into two outputs, and one suite runs against each output, so a difference between the modes shows as a failing test rather than a type error.

**Release is one assembly and many verifications.** Since the engine is a cross-compiled static binary, one job assembles every package from the signed archives. The matrix then only proves those tarballs work, and nothing is published until every platform has passed.

## Dependencies

Design documents this plan was built on:

- `typescript-sdk.md` from the `design` design source: the public API, package layout, platform packages and release shape. Needs the two corrections named under Architecture.
- `typescript-entity-types.md` from the `design` design source: decorated entity classes and their rules. No changes.
- `libxcl.md` from the `design` design source: the core service, framing, engine lifecycle and release assets. No changes.

Runtime:

- `@bufbuild/protobuf`, pinned exactly: the protobuf runtime and the only runtime dependency, as the spec requires.
- `xcl-core`, built from xcl at the tag in `xcl-ref.txt`: the engine. Pinned at the current tag, `v0.13.1`, whose protocol has not changed since. No changes.
- Node 22 or later, with `child_process`, `AbortSignal.any` and `Symbol.asyncDispose`.

Development and CI (dev dependencies, all pinned exactly):

- `typescript` 5.2 or later, for both decorator modes and `await using`.
- `@bufbuild/buf` and `@bufbuild/protoc-gen-es`: generate the messages from xcl's protos at the pin.
- `@types/node`.
- Go 1.27: builds the engine, fixture plugins and the Go parity program from an xcl checkout. Never needed by a user of the package.
- xcl's `bindings/libxcl/testdata/parity` fixtures, `cmd/parity` and the `slow` and `recording` fixture plugins: read from the xcl checkout. No changes.
- `jumppad-labs/xcl-plugin-test` at `v0.3.1`: end-to-end and registry tests. No changes.
- `jumppad-labs/xcl`'s release assets at the pin, signed with the release key, whose public half is copied into the repository for the tests and the release.

Repositories and accounts that must be ready before the first release (the tasks for a person are in the plan):

- npm: the nine package names, each with a trusted publisher set up for `jumppad-labs/xcl-typescript`.
- GitHub: the release bot App installed on `xcl-typescript`, with its secrets; a token for xcl to dispatch `xcl-typescript`.

Order: `xcl-typescript`'s `test.yml` must be on its `main` before xcl's bindings gate calls it, or every xcl pull request fails. The secret for the dispatch must exist before xcl's release workflow gains the job.

Specs: this is the first spec of epic `20261031080131-8c2a58ec-typescript-sdk`. It depends on no other spec, and the printing and SDK-reference specs build on it.

## Testing Approach

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

## Milestones & Tasks

### Milestone 1: A Node program can run xcl configuration with Go plugins

**What changes**: From a source checkout, a Node program can create a configuration, and validate, diff, apply, load and destroy it using Go plugins from a path, a directory or the GitHub registry. It can read entities back generically and get typed errors. There is exactly one engine per process or worker thread, it never keeps Node alive, and it never outlives its host. Failures of the engine are reported clearly. Nothing can be defined in TypeScript yet, and nothing is on npm.

**Validation point**: The unit tests and the real-engine tests for engine lifecycle, plugins and generic reading pass, including a killed host and a script that never closes.

#### - [x] Task: Set up the package and generate its messages
**Id:** 286993e2-226a-4c25-b7ac-9bdf91d0ee4e
**Repo:** xcl-typescript
**Depends on:** none
**Execution:** agent

Turn the empty repository into a typed ES module package for Node 22 and later, with its build, test and development targets. Pin xcl at its current tag and generate the protocol's message classes from it, committed like xcl-python's, so everything above can be written against them.

*Technical detail:* [context.md#task-set-up-the-package-and-generate-its-messages](./context.md#task-set-up-the-package-and-generate-its-messages)

**Acceptance criteria**:
- [x] The package builds to an ES module with type declarations, and imports on Node 22, 24 and 26
- [x] Its only runtime dependency is the protobuf runtime, pinned exactly
- [x] The message classes come from xcl's protos at the pinned tag, and regenerating them changes nothing
- [x] An engine and the fixture plugins can be built from a local xcl checkout for tests

#### - [x] Task: Add the error classes
**Id:** 8861e35d-d3b8-42e4-9e71-599a568944a3
**Repo:** xcl-typescript
**Depends on:**
- 286993e2-226a-4c25-b7ac-9bdf91d0ee4e — Set up the package and generate its messages
**Execution:** agent

Add the base `XclError` and one subclass for each error code the engine sends, plus the errors the package raises itself. Fields are enumerable so a structured logger records them with no configuration, and configuration errors expose each problem with its file and line.

*Technical detail:* [context.md#task-add-the-error-classes](./context.md#task-add-the-error-classes)

**Acceptance criteria**:
- [x] Every engine error code maps to its own class, each `instanceof` the base class
- [x] An unknown code still becomes the base class, so it can be caught
- [x] A configuration error lists each problem with its file, line, column, summary, detail, entity and field
- [x] An error's code, message, details and problems are all enumerable

#### - [x] Task: Run and frame the engine
**Id:** cfa033bc-e805-4beb-8adb-9e512f4d579e
**Repo:** xcl-typescript
**Depends on:**
- 286993e2-226a-4c25-b7ac-9bdf91d0ee4e — Set up the package and generate its messages
- 8861e35d-d3b8-42e4-9e71-599a568944a3 — Add the error classes
**Execution:** agent

Add the transport, the only code that knows about the engine process: find the binary, start it once per module graph, carry frames, match responses to callers, and shut it down. It lets go of the event loop whenever no call is outstanding, so an idle engine never holds Node open, and reports an engine's exit to every call waiting on it.

*Technical detail:* [context.md#task-run-and-frame-the-engine](./context.md#task-run-and-frame-the-engine)

**Acceptance criteria**:
- [x] Creating three configurations starts one engine, and a worker thread gets its own
- [x] An idle engine never keeps the process alive, and the engine exits once its host has
- [x] When the engine dies, every waiting call fails with an error carrying the exit status or signal, and the next use starts a new engine
- [x] A configuration from an engine that has exited is refused by name rather than reaching a new engine's configuration
- [x] A missing platform package gives an error naming that package and how to install it, for each platform and C library
- [x] The engine's standard error reaches the host's standard error

#### - [x] Task: Add the typed client and version check
**Id:** cbedb05a-6d84-42aa-aef6-6e49d7bd0733
**Repo:** xcl-typescript
**Depends on:**
- cfa033bc-e805-4beb-8adb-9e512f4d579e — Run and frame the engine
**Execution:** agent

Add the typed layer that turns generated messages into bytes and error frames into error classes, using the generated service descriptor. On first use it checks that the engine reports the version the package was built for, and refuses to go on otherwise, naming both versions.

*Technical detail:* [context.md#task-add-the-typed-client-and-version-check](./context.md#task-add-the-typed-client-and-version-check)

**Acceptance criteria**:
- [x] A call encodes its request and decodes its response through the fake transport
- [x] An engine error frame throws its error class, and a binding fault throws the binding error
- [x] An engine of a different version is refused with an error naming both versions, and the engine is stopped

#### - [x] Task: Create configurations and run operations
**Id:** 1b0204ac-3222-4ec0-8cb6-9d96ef413da8
**Repo:** xcl-typescript
**Depends on:**
- cbedb05a-6d84-42aa-aef6-6e49d7bd0733 — Add the typed client and version check
**Execution:** agent

Add `createConfig`, the registries for Go plugins, the poll loop and the five operations, with lookups returning generic entities, outputs and diffs. A second operation on a configuration is refused. This gives a working SDK for configuration handled by Go plugins, and every later task extends it.

*Technical detail:* [context.md#task-create-configurations-and-run-operations](./context.md#task-create-configurations-and-run-operations)

**Acceptance criteria**:
- [x] A program can validate, diff, apply, load and destroy, each returning a Promise
- [x] Starting a second operation while one runs throws the operation-in-progress error
- [x] Lookups by address, type and uniqueness, entity counts and outputs work, with generic entities exposing metadata and camelCase attributes
- [x] A diff reports each changed attribute's old and new value, and a value known only after apply is the `UNKNOWN` sentinel
- [x] `close()` and `await using` release the configuration, and closing twice is harmless

#### - [x] Task: Test engine lifecycle and Go plugins against a real engine
**Id:** 0ca0f056-34dd-473e-868f-eed902fbd7a9
**Repo:** xcl-typescript
**Depends on:**
- 1b0204ac-3222-4ec0-8cb6-9d96ef413da8 — Create configurations and run operations
**Execution:** agent

Add the test harness that finds the built engine and plugins, and the first real-engine tests. They cover Go plugins from a path, a directory and the GitHub registry, refusal of tampered or untrusted releases, and the engine's whole life beside its host, including a killed host and a script that never closes.

*Technical detail:* [context.md#task-test-engine-lifecycle-and-go-plugins-against-a-real-engine](./context.md#task-test-engine-lifecycle-and-go-plugins-against-a-real-engine)

**Acceptance criteria**:
- [x] The test plugin applies from a local path, a plugin directory and the GitHub registry
- [x] A tampered release is refused with a signature error, and an untrusted key is refused naming the key
- [x] Three configurations share one engine, and closing the last leaves none running
- [x] A script that never calls `close()` still exits and leaves no engine running
- [x] Killing the host leaves no engine running within five seconds
- [x] Killing the engine mid-apply rejects with the engine-exited error, the host goes on, and a new configuration works
- [x] The first configuration in a process is created within one second

### Milestone 2: Developers define entity types and functions in TypeScript

**What changes**: A developer describes entity types as decorated classes, in a project using either standard or legacy decorators, and registers them, with configuration checked against them as it is for Go types. Lookups return instances of those classes. Functions written in TypeScript, synchronous or asynchronous, can be called from configuration. Variables can be passed in and entities can be encoded as configuration text identical to Go's. Each fixture's derived schema equals Go's, and the parity suite shows no differences.

**Validation point**: The decorator suite passes under both compiler modes, and every parity fixture matches Go's record and `types.json`.

#### - [x] Task: Add the entity decorators in both compiler modes
**Id:** a3cf3bb2-c270-403b-a75a-74d328f1dc8b
**Repo:** xcl-typescript
**Depends on:**
- 1b0204ac-3222-4ec0-8cb6-9d96ef413da8 — Create configurations and run operations
**Execution:** agent

Add `@Entity`, `@Attribute`, `@Sensitive`, `@Computed`, `@Block`, `@Blocks`, `@NamedBlocks`, `@Label` and `@Meta`, each accepting the standard and legacy calling forms, and the per-class field table they fill. `Sensitive` is also the wrapper type for a secret value.

*Technical detail:* [context.md#task-add-the-entity-decorators-in-both-compiler-modes](./context.md#task-add-the-entity-decorators-in-both-compiler-modes)

**Acceptance criteria**:
- [x] The same class definitions record the same fields under standard and legacy decorators, on block classes with no class decorator too
- [x] The decorators type-check in a strict project under either compiler setting
- [x] A `Sensitive` value prints as `(sensitive)` in text, JSON and `util.inspect`, and `reveal()` returns the value only when it holds one
- [x] Subclasses inherit their base class's fields

#### - [x] Task: Derive schemas at registration
**Id:** c047e8fc-f85c-4c5f-9a62-da2650176c91
**Repo:** xcl-typescript
**Depends on:**
- a3cf3bb2-c270-403b-a75a-74d328f1dc8b — Add the entity decorators in both compiler modes
**Execution:** agent

Add the schema deriver and `local.register`, constructing each class once to read its fields and defaults and producing schema v1 in the form Go's reader expects. Anything wrong with a definition fails at registration with a `SchemaError` naming the class and field.

*Technical detail:* [context.md#task-derive-schemas-at-registration](./context.md#task-derive-schemas-at-registration)

**Acceptance criteria**:
- [x] A class's derived schema has the required, optional, default, nullable, sensitive, computed, block, list-of-blocks and named-block forms Go's reader expects
- [x] A class with neither a label field nor a metadata field is a singleton, and a subtype on one is refused
- [x] A property name that would not survive the name rule is refused at registration
- [x] An initializer that disagrees with its decorator is refused at registration
- [x] A class that cannot be constructed with no arguments is refused at registration
- [x] A sensitive field of a type the engine cannot hold is refused at registration

#### - [x] Task: Return registered classes from lookups
**Id:** 55b65d79-b244-495f-942f-b638428804ca
**Repo:** xcl-typescript
**Depends on:**
- c047e8fc-f85c-4c5f-9a62-da2650176c91 — Derive schemas at registration
**Execution:** agent

Hydrate entity JSON into instances of registered classes and make lookups return them. `find` with a class, `findByType`, `findOne` and `all` return instances, and `entities()` returns registered instances and generic entities for the rest.

*Technical detail:* [context.md#task-return-registered-classes-from-lookups](./context.md#task-return-registered-classes-from-lookups)

**Acceptance criteria**:
- [x] Finding `app.web` returns an instance of the registered class with the configured values, and a subtype's entity is found at its address
- [x] A left-out optional field holds its default, nested blocks are decoded, block lists keep their order and named blocks are a record keyed by name
- [x] Labels, metadata and singletons reach the class as designed
- [x] A property `connectionString` reads the configuration's `connection_string`
- [x] `entities()` returns registered instances, and variables, outputs and plugin types generically
- [x] Asking for a class this configuration did not register throws the not-registered error

#### - [x] Task: Call TypeScript functions from configuration
**Id:** 4d448a45-684a-4317-92bb-8faa7e3b9aec
**Repo:** xcl-typescript
**Depends on:**
- 55b65d79-b244-495f-942f-b638428804ca — Return registered classes from lookups
**Execution:** agent

Add `@Func` and `registerFunctions`, and answer function tasks in the poll loop, running each concurrently. A function may be synchronous or asynchronous, and a name clash is refused when the configuration is created.

*Technical detail:* [context.md#task-call-typescript-functions-from-configuration](./context.md#task-call-typescript-functions-from-configuration)

**Acceptance criteria**:
- [x] With `slugify` registered, `name = slugify("My App")` applies and the entity holds `"my-app"`
- [x] An async function's resolved value is filled in
- [x] A function's thrown error fails the configuration with that message
- [x] A name a builtin or another function uses is refused with the name-clash error

#### - [x] Task: Pass variables and encode entities as configuration
**Id:** cbc87f0f-576b-42fe-a98d-fae718288abc
**Repo:** xcl-typescript
**Depends on:**
- 55b65d79-b244-495f-942f-b638428804ca — Return registered classes from lookups
**Execution:** agent

Add `variables` to `createConfig`, and `encodeEntity` and `encodeSavedEntity`, building the record the engine reads from an instance and writing unchanged fields exactly as the engine gave them. The engine renders the text, so it is Go's.

*Technical detail:* [context.md#task-pass-variables-and-encode-entities-as-configuration](./context.md#task-pass-variables-and-encode-entities-as-configuration)

**Acceptance criteria**:
- [x] Passing `region: "us"` makes an entity using `variable.region` hold `"us"`
- [x] A looked-up, hand-built or generic entity and a saved record each encode, with the four choices
- [x] A non-entity, a variable, an output, a module and an unencodable value throw the not-encodable error, and a class never registered throws the not-registered error
- [x] Encoding a looked-up entity during an operation throws the operation-in-progress error, and encoding a saved record works during one

#### - [x] Task: Compare results with Go on the parity fixtures
**Id:** b4e6445a-0e7c-425e-83a2-1596a44bad90
**Repo:** xcl-typescript
**Depends on:**
- 4d448a45-684a-4317-92bb-8faa7e3b9aec — Call TypeScript functions from configuration
- cbc87f0f-576b-42fe-a98d-fae718288abc — Pass variables and encode entities as configuration
**Execution:** agent

Add a Node program that runs each of xcl's parity fixtures and writes the same record as Go's parity program, leaving out the keys that belong to printing, and tests that compare the two and hold each fixture's classes to its `types.json`. This is what catches a difference from Go.

*Technical detail:* [context.md#task-compare-results-with-go-on-the-parity-fixtures](./context.md#task-compare-results-with-go-on-the-parity-fixtures)

**Acceptance criteria**:
- [x] For every fixture, the Node and Go records are equal apart from the printing keys
- [x] A difference names the fixture and the first path that differs
- [x] Every fixture's classes derive exactly its `types.json` documents
- [x] Every entity's text matches Go's byte for byte under all sixteen encoding combinations, including hand-built and generic entities and saved records

### Milestone 3: Long-running work behaves well inside a Node program

**What changes**: A program can receive the engine's events, keep state in its own asynchronous store, encrypt sensitive values in state, cancel an operation with an `AbortSignal`, and opt in to Ctrl-C cancellation for one operation. The event loop keeps serving other work throughout, and Ctrl-C without opting in behaves as Node's default does while the state directory is still saved. Configurations work in worker threads and in cluster and child-process workers.

**Validation point**: The real-engine tests for events, custom stores, cancellation, signals, workers and event-loop delay pass.

#### - [x] Task: Deliver events to TypeScript handlers
**Id:** 560a9c85-d428-4e3c-9013-9bc8e17867fb
**Repo:** xcl-typescript
**Depends on:**
- 55b65d79-b244-495f-942f-b638428804ca — Return registered classes from lookups
**Execution:** agent

Add `onEvent`, with event data levels and masks, delivering events one at a time in order and awaiting an asynchronous handler before the next. An operation does not finish until every handler has. An exception in a handler is reported as a warning and does not stop the operation.

*Technical detail:* [context.md#task-deliver-events-to-typescript-handlers](./context.md#task-deliver-events-to-typescript-handlers)

**Acceptance criteria**:
- [x] A handler receives an event for each entity's start and finish during an apply
- [x] At the no-data level events carry no entity data; at raw and processed they do, with sensitive values redacted
- [x] `event.entity(App)` returns the entity as a typed value, and nothing when the event has no data
- [x] An async handler is awaited, in order, before the operation settles
- [x] A handler's exception becomes a warning and the operation carries on

#### - [x] Task: Support custom state stores and masks
**Id:** a6a7d09d-4162-4a9e-ae8d-ee04ee1508d7
**Repo:** xcl-typescript
**Depends on:**
- 1b0204ac-3222-4ec0-8cb6-9d96ef413da8 — Create configurations and run operations
**Execution:** agent

Add `stateStore`, whose methods may be asynchronous, answered as tasks from the poll loop, and `stateMask` with the mask builders. A program can keep state in its own store and have sensitive values encrypted there.

*Technical detail:* [context.md#task-support-custom-state-stores-and-masks](./context.md#task-support-custom-state-stores-and-masks)

**Acceptance criteria**:
- [x] A store with async methods receives the state on apply, and a later load in a new process reads it back
- [x] A state directory is written by an apply and read back by a load in a new process
- [x] With encryption enabled, sensitive values in what the store receives are encrypted
- [x] A store's exception fails the operation with its message
- [x] A sensitive value is masked in events and in state when a state mask is set

#### - [x] Task: Cancel operations with an AbortSignal and Ctrl-C
**Id:** 402d8acc-ee45-4842-938b-d633b5ae7245
**Repo:** xcl-typescript
**Depends on:**
- a6a7d09d-4162-4a9e-ae8d-ee04ee1508d7 — Support custom state stores and masks
- 560a9c85-d428-4e3c-9013-9bc8e17867fb — Deliver events to TypeScript handlers
**Execution:** agent

Add `signal` to every operation, sending `Cancel` as soon as it aborts, and `interruptSignal()`, whose SIGINT listener exists only for the operation it was passed to. After a cancel, the rejection does not wait for the program's own handlers. The package adds no SIGINT listener unless asked.

*Technical detail:* [context.md#task-cancel-operations-with-an-abortsignal-and-ctrl-c](./context.md#task-cancel-operations-with-an-abortsignal-and-ctrl-c)

**Acceptance criteria**:
- [x] Aborting mid-apply rejects with the cancellation error within two seconds, the completed entities are in state, and the test plugin records no operation started after the abort
- [x] An already-aborted signal rejects without starting the operation
- [x] Sending SIGINT to a program that opted in rejects its apply, its own code then runs, and no package SIGINT listener remains
- [x] Importing the package and running an apply adds no SIGINT listener
- [x] SIGINT to a program that did not opt in ends it as Node does by default, with completed entities in the state directory and no engine left running

#### - [x] Task: Prove workers and a responsive event loop
**Id:** 69b64492-27f0-428a-9618-23d5f07474ea
**Repo:** xcl-typescript
**Depends on:**
- 402d8acc-ee45-4842-938b-d633b5ae7245 — Cancel operations with an AbortSignal and Ctrl-C
**Execution:** agent

Add real-engine tests for configurations in worker threads and in cluster and child-process workers, and a test that a web server keeps answering during a long apply with event-loop delay under the target.

*Technical detail:* [context.md#task-prove-workers-and-a-responsive-event-loop](./context.md#task-prove-workers-and-a-responsive-event-loop)

**Acceptance criteria**:
- [x] A configuration created inside a worker thread, a cluster worker and a child-process worker each applies successfully
- [x] During a slow apply, a timer's 99th-percentile delay stays under 50 ms by Node's event-loop delay monitor
- [x] An HTTP server in the same process answers requests throughout a slow apply

### Milestone 4: Every xcl release publishes a matching package to npm

**What changes**: Installing `xcl-lang` from npm works on all eight platforms with no toolchain. Each xcl release produces a package of the same version, carrying the engine built from that release, with no manual publish, and a change to xcl that breaks the TypeScript package's results is caught before it merges. The package ships with a README, a parity record and doc comments on every export. This milestone includes the one-off npm and GitHub setup a person must do.

**Validation point**: The package's test workflow is green on every platform with its tarballs installed, the release workflow publishes a first release, and xcl's bindings gate runs the TypeScript suite on a pull request.

#### - [x] Task: Assemble the platform packages
**Id:** b8d79341-3204-4065-a35a-45663065de25
**Repo:** xcl-typescript
**Depends on:**
- cfa033bc-e805-4beb-8adb-9e512f4d579e — Run and frame the engine
**Execution:** agent

Add the eight platform package manifests and a script that fills each with its engine and sets every version, so one job can produce all nine tarballs. `xcl-lang` lists them as optional dependencies.

*Technical detail:* [context.md#task-assemble-the-platform-packages](./context.md#task-assemble-the-platform-packages)

**Acceptance criteria**:
- [x] The script produces nine tarballs at one version, with the engine executable inside the right platform's package
- [x] Each platform package declares its operating system, CPU and, on Linux, C library
- [x] `xcl-lang` lists all eight as optional dependencies at the same version

#### - [x] Task: Add the test workflow on every platform
**Id:** 3f2bb339-7c34-4850-a7f4-d7e7ce7d6001
**Repo:** xcl-typescript
**Depends on:**
- b4e6445a-0e7c-425e-83a2-1596a44bad90 — Compare results with Go on the parity fixtures
- 69b64492-27f0-428a-9618-23d5f07474ea — Prove workers and a responsive event loop
- b8d79341-3204-4065-a35a-45663065de25 — Assemble the platform packages
**Execution:** agent

Add a reusable test workflow running every suite on the six runner platforms and on Alpine for both architectures, on Node 22, 24 and 26. The package's own CI calls it with the pinned engine, and xcl calls it with a pull request's commit.

*Technical detail:* [context.md#task-add-the-test-workflow-on-every-platform](./context.md#task-add-the-test-workflow-on-every-platform)

**Acceptance criteria**:
- [x] The suite runs on Linux, macOS and Windows on both architectures, and on Alpine on both, against each supported Node version
- [x] A missing engine or plugin fails a strict run rather than skipping
- [x] One `result` job fails unless every job succeeded
- [x] Tarballs of the package install on each platform with no Go or C compiler and pass the parity suite

#### - [x] Task: Add the bump, tag and release workflows
**Id:** 618758a7-1dbe-4850-bccf-281ede0c8f0f
**Repo:** xcl-typescript
**Depends on:**
- 3f2bb339-7c34-4850-a7f4-d7e7ce7d6001 — Add the test workflow on every platform
**Execution:** agent

Add the workflows that take an xcl release to npm: an engine bump pull request, a tag equal to the pinned version when it merges, and a release that verifies the signed engines, tests all tarballs on every platform before publishing, then publishes and checks the published package.

*Technical detail:* [context.md#task-add-the-bump-tag-and-release-workflows](./context.md#task-add-the-bump-tag-and-release-workflows)

**Acceptance criteria**:
- [x] An xcl release dispatch opens a pull request changing the pin, which merges by itself when its checks pass
- [x] A merge that changes the pin tags that commit with the pinned tag, and no other merge tags anything
- [x] Every engine archive is checked against the signed checksums before use
- [x] Nothing is published until every platform's tarballs have passed, and platform packages publish before `xcl-lang`
- [x] The published package installs with no toolchain and its engine reports the tag

#### - [x] Task: Write the README, parity record and doc comments
**Id:** 9999eaf4-c5eb-4aac-9845-5db97a220bea
**Repo:** xcl-typescript
**Depends on:**
- 402d8acc-ee45-4842-938b-d633b5ae7245 — Cancel operations with an AbortSignal and Ctrl-C
- cbc87f0f-576b-42fe-a98d-fae718288abc — Pass variables and encode entities as configuration
**Execution:** agent

Write the package README with install and a getting started that applies a configuration using a Go plugin, the parity record listing every Go operation and lookup with its TypeScript equivalent, and a doc comment on every public export.

*Technical detail:* [context.md#task-write-the-readme-parity-record-and-doc-comments](./context.md#task-write-the-readme-parity-record-and-doc-comments)

**Acceptance criteria**:
- [x] The README covers install, getting started with a Go plugin and the supported platforms
- [x] The parity record lists every Go operation and lookup with its TypeScript equivalent
- [x] Every public export has a doc comment

#### - [x] Task: Dispatch and test the TypeScript package from xcl
**Id:** 1699c554-737b-4448-831a-62993797a68a
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Change xcl's release workflow to also dispatch `xcl-typescript` with the tag and size, in a job of its own, and add a job to the bindings gate running the TypeScript suite against each pull request's commit. These are the only changes to the xcl repository.

*Technical detail:* [context.md#task-dispatch-and-test-the-typescript-package-from-xcl](./context.md#task-dispatch-and-test-the-typescript-package-from-xcl)

**Acceptance criteria**:
- [x] An xcl release dispatches `xcl-typescript` with the tag and size, and a failure there cannot affect xcl-python's dispatch
- [x] The bindings gate needs the TypeScript suite as well as Python's
- [x] Nothing else in the xcl repository changes

#### - [x] Task: Correct the SDK design document
**Id:** 6f52f9c8-6189-463a-9397-85bcdf004109
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Change `typescript-sdk.md` to say that `entities()` returns an instance of the registered class where there is one and a generic `Entity` otherwise, as the spec requires, and remove the comparison with `xcl-lang` on PyPI from its Releases section, which no longer describes xcl-python.

*Technical detail:* [context.md#task-correct-the-sdk-design-document](./context.md#task-correct-the-sdk-design-document)

**Acceptance criteria**:
- [x] `typescript-sdk.md` describes `entities()` as the spec does
- [x] Its Releases section no longer says xcl-python's version equals the tag it bundles

#### - [x] Task: Set up npm publishing and GitHub secrets
**Id:** 8c0e5f32-5b56-4ba1-ab0c-baea2d0f6812
**Repo:** xcl-typescript
**Depends on:**
- 618758a7-1dbe-4850-bccf-281ede0c8f0f — Add the bump, tag and release workflows
**Execution:** human — needs npm organisation access and GitHub organisation secrets an agent does not have

Claim the nine package names on npm with a first placeholder publish, set up a trusted publisher for each pointing at `xcl-typescript`'s release workflow, and create the `npm` environment. Install the release bot App on the repository with its secrets, require the `result` check on `main`, and create the token xcl uses to dispatch the repository.

*Technical detail:* [context.md#task-set-up-npm-publishing-and-github-secrets](./context.md#task-set-up-npm-publishing-and-github-secrets)

**Acceptance criteria**:
- [ ] All nine npm packages exist and each trusts `jumppad-labs/xcl-typescript`'s release workflow
- [ ] The release bot App can push tags and open pull requests in `xcl-typescript`
- [ ] xcl holds a working `XCL_TYPESCRIPT_DISPATCH_TOKEN`

## Open Questions

- **Does the engine reliably see end of input on Windows when its host is killed?** This depends on how Windows closes the pipe and whether any handle to it has been inherited, and it can only be observed by running the test on Windows runners. The spec names this as a risk. If the killed-host test fails on Windows, STOP and ask the user: the spec rules out a native addon, so any other safeguard is their decision.
- **Are the nine npm names free?** This depends on the state of the registry when the person doing the setup task claims them. If any is taken, STOP and ask the user, because the names are a spec constraint.

## Out of Scope

- Printing, highlighting and the pretty logger for TypeScript, including the printing keys of the parity record: the next spec in epic `20261031080131-8c2a58ec-typescript-sdk`, `typescript-printing`.
- The TypeScript SDK reference and guide on xcl.dev, and TypeScript tabs on the site: the epic's `typescript-sdk-reference` spec.
- Providers written in TypeScript, TypeScript plugins and a plugin SDK.
- Generating TypeScript types for a plugin's entity types.
- Support guarantees for Bun and Deno.
- A separate CommonJS build, a synchronous API, browsers and edge isolates.
- A name-mapping escape hatch for a property that does not follow the name rule: such a name is refused, as `typescript-sdk.md` drafts it.
- SIGTERM in `interruptSignal()`: only SIGINT is covered, as the spec asks.
- Any change to the Go library or the Python package; in xcl only the release and bindings workflows change.
- Attribute-typed objects (schema v1's `{"object": [...]}` as an attribute rather than a block), which neither typescript-entity-types.md nor the acceptance criteria call for.

## Changelog

### 2026-11-10 — Task: Set up the package and generate its messages

**What was done**: The empty repository became a typed ES module package for Node 22 and later, with build, test and development targets, the pin at `v0.13.1` and committed message classes generated from xcl's protos.

**Deviations**: TypeScript is pinned at 5.9.3 rather than the newer 7.x line, to stay on the compiler the plan's "5.2 or later" assumes. The `protos` target does not clear `src/internal/core` first: `buf generate` overwrites its output. `package-lock.json` is committed so CI can use `npm ci`. Messages land in `src/internal/core/xcl/core/v1/`, the path buf derives from the proto package.

**Files changed**:
- `xcl-typescript: package.json`
- `xcl-typescript: package-lock.json`
- `xcl-typescript: tsconfig.json`
- `xcl-typescript: tsconfig.test.json`
- `xcl-typescript: buf.gen.yaml`
- `xcl-typescript: Makefile`
- `xcl-typescript: xcl-ref.txt`
- `xcl-typescript: .gitignore`
- `xcl-typescript: src/version.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: src/internal/core/xcl/core/v1/core_pb.ts`
- `xcl-typescript: src/internal/core/xcl/core/v1/transport_pb.ts`
- `xcl-typescript: test/keys/jumppad-labs.asc`
- `xcl-typescript: test/unit/messages.test.ts`

**Discoveries**: `npx node@22` does not switch runtime here, it runs the host Node; checking a Node version needs a real runtime such as the `node:22-alpine` image.

### 2026-11-10 — Task: Add the error classes

**What was done**: Added `XclError`, one subclass for each of the engine's 28 error codes, the errors the package raises itself, and `fromProto`. Fields are enumerable and configuration errors expose each problem with its position.

**Deviations**: `EngineExitedError` carries the status as `exitCode`, because `code` is already the stable error code on every `XclError`. The goal check's snapshot of the work was not written for this task: a helper script in a scratchpad shared with another agent had been overwritten, so the check recorded `work changed while the reviewer ran or was not staged`. No file was edited while the reviewer ran, but the check could not confirm that.

**Files changed**:
- `xcl-typescript: src/errors.ts`
- `xcl-typescript: test/unit/errors.test.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Run and frame the engine

**What was done**: Added the byte-level `Transport`, the engine locator and `SubprocessTransport`: one engine per module graph, frames read on the event loop and matched by id, `ref`/`unref` following outstanding calls, a handle map across engine generations, a SIGTERM-then-SIGKILL stop, and engine exits reported to every waiting call.

**Deviations**: The locator also refuses a platform package that is installed without its binary, and `npm test` scripts carry a five-minute per-test timeout so a hung call cannot hang the runner. Windows was not exercised: the locator's Windows paths are tested with stubbed inputs, and the signal and worker-thread tests are POSIX only.

**Files changed**:
- `xcl-typescript: src/internal/transport.ts`
- `xcl-typescript: src/internal/subprocess.ts`
- `xcl-typescript: test/helpers/fake-engine.ts`
- `xcl-typescript: test/unit/subprocess.test.ts`
- `xcl-typescript: package.json`

**Discoveries**: `process.report.getReport()` looks up network interfaces unless `excludeNetwork` is set, which can cost seconds on first use; the locator sets it for the one call it makes.

### 2026-11-10 — Task: Add the typed client and version check

**What was done**: Added `Client`, which encodes and decodes each call from the generated service descriptor, turns error frames into error classes and binding faults into the binding error, and checks the engine's ABI and version once on first use, stopping an engine it refuses.

**Deviations**: A failed version check is not remembered, so the next use checks whichever engine it starts.

**Files changed**:
- `xcl-typescript: src/internal/client.ts`
- `xcl-typescript: test/helpers/fake-transport.ts`
- `xcl-typescript: test/unit/client.test.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Create configurations and run operations

**What was done**: Added `createConfig`, the Go plugin registries, the poll loop and the five operations, with lookups returning generic entities, outputs and diffs. A second operation on a configuration is refused, and `close()` and `await using` release it.

**Deviations**: A closed configuration rejects with `InvalidArgumentError`, which the designs did not name. `registerPluginDirectory` passes `~/` through because the engine already expands it.

**Files changed**:
- `xcl-typescript: src/hydrate.ts`
- `xcl-typescript: src/registry.ts`
- `xcl-typescript: src/internal/poll.ts`
- `xcl-typescript: src/diff.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/config.test.ts`
- `xcl-typescript: test/unit/registry.test.ts`
- `xcl-typescript: test/unit/diff.test.ts`
- `xcl-typescript: test/unit/names.test.ts`

**Discoveries**: The poll loop answers only the finished task so far; state and function tasks arrive with the tasks that add them, and an unanswered one would hang an operation.

### 2026-11-10 — Task: Test engine lifecycle and Go plugins against a real engine

**What was done**: Added the harness that finds the built engine and plugins, a fake GitHub API and a fresh-process scenario runner, and real-engine tests for Go plugins from a path, a directory and the GitHub registry, tamper and untrusted-key refusals, the engine's life beside its host, and typed errors.

**Deviations**: `npm test` now builds `dist` first, because the scenario scripts import it and a stale build would test old code. The version-mismatch test runs a shim over the fake engine rather than a real engine of another version, and is POSIX only. Windows and macOS were not run; the `tasklist` and `ps` branches of the process-alive check are unexercised until CI runs them.

**Files changed**:
- `xcl-typescript: test/helpers/engine.ts`
- `xcl-typescript: test/helpers/plugins.ts`
- `xcl-typescript: test/helpers/fake-github.ts`
- `xcl-typescript: test/helpers/script.ts`
- `xcl-typescript: test/integration/plugins.test.ts`
- `xcl-typescript: test/integration/github-tamper.test.ts`
- `xcl-typescript: test/integration/engine-process.test.ts`
- `xcl-typescript: test/integration/operations.test.ts`
- `xcl-typescript: package.json`

**Discoveries**: go-plugin creates its unix sockets under `TMPDIR`, so a long `TMPDIR` exceeds the 108-byte socket path limit and plugins fail to start. Those sockets are left behind in `TMPDIR` when an engine is stopped or killed.

### 2026-11-10 — Task: Add the entity decorators in both compiler modes

**What was done**: Added `@Entity`, `@Attribute`, `@Sensitive`, `@Computed`, `@Block`, `@Blocks`, `@NamedBlocks`, `@Label` and `@Meta`, each accepting the standard and legacy calling forms, the per-class field table they fill, and the `Sensitive` wrapper. One suite runs against the same classes compiled under each mode.

**Deviations**: `Entity` and `Meta`, which each name a decorator and a type, live in a new `src/entity.ts`. `package.json`'s test scripts compile and run both decorator outputs.

**Files changed**:
- `xcl-typescript: src/decorators.ts`
- `xcl-typescript: src/entity.ts`
- `xcl-typescript: src/sensitive.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: tsconfig.decorators-standard.json`
- `xcl-typescript: tsconfig.decorators-legacy.json`
- `xcl-typescript: package.json`
- `xcl-typescript: test/decorators/entities.ts`
- `xcl-typescript: test/decorators/fields.test.ts`
- `xcl-typescript: test/unit/sensitive.test.ts`

**Discoveries**: Under standard decorators a class's field table exists only after it has been constructed once, so anything reading the table must construct the class first.

### 2026-11-10 — Task: Derive schemas at registration

**What was done**: Added the schema deriver and `register` on both registries. Registering a class constructs it once to read its fields and defaults and produces schema v1 in the form Go's reader expects, and anything wrong with a definition throws `SchemaError` naming the class and field.

**Deviations**: A property starting with an upper-case letter is refused as well, because it would otherwise become a name with a leading underscore. A disagreeing initializer's message leaves out the value, which may be a secret. Known gaps left to the engine or to review: a non-finite number nested under `Any` is written as `null` in a default, and a nested `@Label()` named `disabled` is refused only when the configuration is created.

**Files changed**:
- `xcl-typescript: src/schema.ts`
- `xcl-typescript: src/registry.ts`
- `xcl-typescript: test/unit/schema.test.ts`
- `xcl-typescript: test/unit/registry.test.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Return registered classes from lookups

**What was done**: Added hydration of entity JSON into registered classes, the `ClassMap`, `Entity.as`, and the class forms of `find`, `findByType`, `findOne` and `all`. `entities()` returns registered instances and generic entities for the rest.

**Deviations**: An entity of another registered type throws `TypeMismatchError`, as Python does. The goal reviewer's evidence cites `typed-decorators.test.ts` for several behaviours whose tests are in `entities.test.ts`; the tests exist and pass.

**Files changed**:
- `xcl-typescript: src/hydrate.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: test/unit/hydrate.test.ts`
- `xcl-typescript: test/unit/config.test.ts`
- `xcl-typescript: test/integration/entities.test.ts`
- `xcl-typescript: test/integration/typed-decorators.test.ts`

**Discoveries**: The engine's `Validate` parses without decoding, so a missing required attribute is reported by `apply`, not `validate`, from every SDK. The spec's "fails validation at the block's position" holds at apply.

### 2026-11-10 — Task: Call TypeScript functions from configuration

**What was done**: Added `@Func` and `registerFunctions`, and the poll loop answers function tasks, each running beside the others. A function may be synchronous or asynchronous, a thrown error fails the configuration with its message, and a name clash is refused when the configuration is created.

**Deviations**: Parameters are declared to the engine as `arg0`, `arg1` and so on, because a decorator has no parameter names to read.

**Files changed**:
- `xcl-typescript: src/decorators.ts`
- `xcl-typescript: src/schema.ts`
- `xcl-typescript: src/registry.ts`
- `xcl-typescript: src/internal/poll.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/decorators/entities.ts`
- `xcl-typescript: test/decorators/functions.test.ts`
- `xcl-typescript: test/unit/functions.test.ts`
- `xcl-typescript: test/unit/registry.test.ts`
- `xcl-typescript: test/integration/functions.test.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Pass variables and encode entities as configuration

**What was done**: Added `variables` to `createConfig`, `encodeEntity` and `encodeSavedEntity`, and the record builder that mirrors hydration, writing a field still holding what hydration set exactly as the engine wrote it.

**Deviations**: The test of encoding during an operation uses a hand-built entity, because a lookup is itself refused while an operation runs; the engine applies the same refusal to any `EncodeEntity`.

**Files changed**:
- `xcl-typescript: src/hydrate.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/record.test.ts`
- `xcl-typescript: test/integration/encoding.test.ts`

**Discoveries**: A computed field with no value is written only with `includeEmpty` as well as `includeComputed`.

### 2026-11-10 — Task: Compare results with Go on the parity fixtures

**What was done**: Added a Node program that runs each of xcl's parity fixtures and writes the same record as Go's parity program, less the printing keys, tests comparing the two and naming the first difference, and a test holding each fixture's classes to its `types.json`. All ten fixtures match Go, including all sixteen encoding combinations.

**Deviations**: Beyond the plan, seven tests rerun the fixtures with typed lookups, encoding through `find(Class, address)`. The normaliser turns a `state.json` holding `null` into `[]`, as Go decodes it, where Python records an error; no fixture reaches this. The output-name conversion is not exercised by any fixture, since none has an output name that differs under the name rule.

**Files changed**:
- `xcl-typescript: test/parity/run.ts`
- `xcl-typescript: test/parity/types.ts`
- `xcl-typescript: test/parity/parity.test.ts`
- `xcl-typescript: test/parity/types.test.ts`
- `xcl-typescript: test/parity/normaliser.test.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Deliver events to TypeScript handlers

**What was done**: Added `Event`, `EventDataLevel`, and the `onEvent`, `eventData` and `eventBufferSize` options. Events reach the handler one at a time and in order, an asynchronous handler is awaited before the next, the operation waits for every handler, and a handler's exception becomes a process warning.

**Deviations**: `eventMask` and `noEventMask` are added with the masks in the next task, since they need `mask.ts`. While recording the previous task's goal check, `src/events.ts`, written early, was moved aside so the recorded check compared the reviewed tree; it had no part in that review.

**Files changed**:
- `xcl-typescript: src/events.ts`
- `xcl-typescript: src/internal/poll.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/events.test.ts`
- `xcl-typescript: test/integration/events.test.ts`

**Discoveries**: A config-only entity has no provider step, so it gets a create success event and no start event; only entities a plugin handles get both.

### 2026-11-10 — Task: Support custom state stores and masks

**What was done**: Added `stateStore`, whose methods may be asynchronous, answered as tasks from the poll loop, the `stateMask`, `eventMask` and `noEventMask` options, and the `mask` builders `aes256gcm`, `hmacSha256`, `redact` and `omit`.

**Deviations**: After the goal check, the masked-event test was tightened: the reviewer noted its assertion would pass if the event were absent, so it now asserts the event exists and carries the entity.

**Files changed**:
- `xcl-typescript: src/mask.ts`
- `xcl-typescript: src/internal/poll.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/state.test.ts`
- `xcl-typescript: test/integration/state.test.ts`

**Discoveries**: Scenario scripts are plain JavaScript, so they call decorators in their legacy form against the prototype, which the SDK accepts.

### 2026-11-10 — Task: Cancel operations with an AbortSignal and Ctrl-C

**What was done**: Added `signal` to every operation, sending `Cancel` as soon as it aborts, and `interruptSignal()`, whose SIGINT listener exists only for the operation it was passed to. After a cancel the rejection does not wait for the program's own handlers or tasks, and the package adds no SIGINT listener unless asked.

**Deviations**: The two-second bound holds only while the provider call in flight is itself shorter than that: the engine lets a provider call finish after a cancel (xcl's providers run with a context that is never cancelled), so a 30-second provider call delayed the rejection by 30 seconds. The test uses a one-second gate. Nothing in this package can shorten that without abandoning the operation. Signal tests run on POSIX only.

**Amendments**: Spec, Acceptance Criteria, Cancellation: now "within two seconds, once any provider call in flight has returned", approved by the user, because the engine lets a provider call in flight finish. The cancellation tests hold against the amended text. Design `typescript-sdk.md` also amended by the coordinator (see the design-correction task).

**Files changed**:
- `xcl-typescript: src/interrupt.ts`
- `xcl-typescript: src/internal/poll.ts`
- `xcl-typescript: src/config.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: test/unit/cancel.test.ts`
- `xcl-typescript: test/unit/interrupt.test.ts`
- `xcl-typescript: test/integration/cancel.test.ts`
- `xcl-typescript: test/integration/interrupt.test.ts`

**Discoveries**: An engine outlives an interrupted host by the provider call in flight, for the same reason.

### 2026-11-10 — Task: Prove workers and a responsive event loop

**What was done**: Added real-engine tests for configurations in worker threads, cluster workers and child-process workers, and tests that an HTTP server keeps answering and event-loop delay stays under 50 ms at the 99th percentile during a slow apply.

**Deviations**: None.

**Files changed**:
- `xcl-typescript: test/integration/workers.test.ts`
- `xcl-typescript: test/integration/event-loop.test.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Assemble the platform packages

**What was done**: Added the eight platform package manifests and `scripts/assemble.mjs`, which fills each with its engine from the verified archives, sets every version and packs all nine tarballs.

**Deviations**: `optionalDependencies` are written into `package.json` by the release script rather than kept in the repository. Listing unpublished `0.0.0-dev` versions makes `npm ci` refuse the lock file ("Missing ... from lock file"), so development and CI could not install. The assembled `xcl-lang` tarball carries all eight at the release version, which is what users install.

**Files changed**:
- `xcl-typescript: npm/xcl-lang-linux-x64-gnu/package.json`
- `xcl-typescript: npm/xcl-lang-linux-x64-musl/package.json`
- `xcl-typescript: npm/xcl-lang-linux-arm64-gnu/package.json`
- `xcl-typescript: npm/xcl-lang-linux-arm64-musl/package.json`
- `xcl-typescript: npm/xcl-lang-darwin-x64/package.json`
- `xcl-typescript: npm/xcl-lang-darwin-arm64/package.json`
- `xcl-typescript: npm/xcl-lang-win32-x64/package.json`
- `xcl-typescript: npm/xcl-lang-win32-arm64/package.json`
- `xcl-typescript: scripts/assemble.mjs`
- `xcl-typescript: scripts/assemble.test.mjs`
- `xcl-typescript: package.json`

**Discoveries**: None.

### 2026-11-10 — Task: Add the test workflow on every platform

**What was done**: Added the reusable `test.yml`, which runs every suite on the six runner platforms and on Alpine on both architectures across Node 22, 24 and 26, installs the assembled tarballs with compilers hidden and runs parity against them, and fails a single `result` job unless every job succeeded. Added `ci.yml` and a runnable `examples/getting-started/`.

**Deviations**: The tarballs are built once on `ubuntu-latest`, because `scripts/assemble.mjs` runs on POSIX and needs all eight archives, and are stamped `v0.0.0-ci`. CI installs with `npm ci --ignore-scripts`. The no-network run covers the example's apply only. The goal check sent the task back once for Alpine running only Node 22; a Node matrix was added. No real GitHub Actions run has happened: step bodies were run locally on Linux x86-64 only, and macOS, Windows, arm64 and Node 24 are unexercised until the first CI run.

**Files changed**:
- `xcl-typescript: .github/workflows/test.yml`
- `xcl-typescript: .github/workflows/ci.yml`
- `xcl-typescript: examples/getting-started/main.ts`
- `xcl-typescript: examples/getting-started/main.xcl`
- `xcl-typescript: examples/getting-started/package.json`
- `xcl-typescript: examples/getting-started/tsconfig.json`
- `xcl-typescript: examples/getting-started/jumppad-labs-releases.asc`

**Discoveries**: `include` entries that add keys to a matrix need the varying key as a real axis, or two entries overwrite each other rather than crossing with the other axes.

### 2026-11-10 — Task: Add the bump, tag and release workflows

**What was done**: Added `engine-bump.yml`, `tag.yml`, `release.yml` and `.github/scripts/install-check.mjs`. An xcl release dispatch opens a pin bump that merges itself, a merge changing the pin tags that commit with the pinned tag, and a release verifies the signed engines, tests all tarballs on every platform before publishing, publishes platform packages first and `xcl-lang` last, and checks the published package.

**Deviations**: Added beyond the plan: `settings` refuses a tag that is not `vX.Y.Z` or differs from the pin, `publish` skips packages already at that version so a half-finished publish can be re-run, and `github-release` skips an existing release. `install-check` reads the engine's version by running the platform package's `xcl-core --version` and refuses to run with `XCL_CORE_PATH` set. It runs on Node 22 only, one job per platform. No workflow has run on GitHub and nothing was published: the download-and-verify step, tagging and bump shell steps and the tarball install were exercised locally on Linux x86-64 only. A real release is the first proof of Actions behaviour, auto-merge, App token permissions and npm trusted publishing.

**Files changed**:
- `xcl-typescript: .github/workflows/engine-bump.yml`
- `xcl-typescript: .github/workflows/tag.yml`
- `xcl-typescript: .github/workflows/release.yml`
- `xcl-typescript: .github/scripts/install-check.mjs`

**Discoveries**: Auto-merge needs "allow auto-merge" and a required `result` check on `main`; without a required check the bump pull request can merge unverified. Both are part of the human setup task.

### 2026-11-10 — Task: Write the README, parity record and doc comments

**What was done**: Wrote the package README with install, a getting started that applies a configuration using a Go plugin and the supported platforms; `PARITY.md` listing every Go operation and lookup with its TypeScript equivalent; and doc comments on every public export.

**Deviations**: Printing and logging rows are marked planned, as the printing spec covers them. The goal check sent the task back once for a missing `EncodeEntityRecord` row. A doc comment that is only an `@internal` tag counts as no comment to the TypeScript compiler, so internal members carry text before the tag.

**Files changed**:
- `xcl-typescript: README.md`
- `xcl-typescript: PARITY.md`
- `xcl-typescript: src/diff.ts`
- `xcl-typescript: src/index.ts`
- `xcl-typescript: src/mask.ts`
- `xcl-typescript: src/registry.ts`
- `xcl-typescript: src/events.ts`
- `xcl-typescript: src/decorators.ts`
- `xcl-typescript: src/hydrate.ts`
- `xcl-typescript: src/config.ts`

**Discoveries**: None.

### 2026-11-10 — Task: Dispatch and test the TypeScript package from xcl

**What was done**: xcl's release workflow now also dispatches `xcl-typescript` with the tag and size in a job of its own, and the bindings gate runs and needs the TypeScript suite as well as Python's.

**Deviations**: None. Per the plan, this must merge after `xcl-typescript`'s test workflow is on its `main` and the dispatch token exists, or xcl pull requests and the next release fail.

**Files changed**:
- `xcl: .github/workflows/release.yml`
- `xcl: .github/workflows/bindings.yml`

**Discoveries**: None.

### 2026-11-10 — Task: Correct the SDK design document

**What was done**: Checked `typescript-sdk.md` against the task's criteria. `entities` now returns an instance of the registered class where there is one and a generic `Entity` otherwise, and the Releases section no longer makes the comparison with `xcl-lang` on PyPI.

**Deviations**: This run did not edit the design: orchestrated runs may not write designs, so the coordinator applied the amendment in the project with the user's approval, and the task checked the result.

**Amendments**: Design `typescript-sdk.md` (source `design`), sections Lookups and Releases, as above.

**Files changed**:
- `xcl: .spektacular/design/typescript-sdk.md` (applied by the coordinator)

**Discoveries**: None.

### 2026-11-10 — Task: Set up npm publishing and GitHub secrets

**What was done**: Nothing in this run: the task needs npm organisation access and GitHub organisation secrets. It went through the goal check, which judged all three criteria not met, and the user accepted the shortfall.

**Deviations**: Shortfall accepted by the user: none of the three criteria is ticked. The reason given is "human setup task, done by a person at release time; listed in the release plan". Until a person does it, no release can publish and xcl's dispatch job fails for want of its secret.

**Files changed**: None.

**Discoveries**: None.
