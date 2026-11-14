---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Research: 20261031080136-264e0eab-typescript-core-sdk

## Alternatives considered and rejected

- **Reader on a worker thread or a polling timer in the transport.** Rejected: frames can be read on the event loop as stream `data` events, so nothing blocks and `Poll` needs no special case. libxcl.md "Node and TypeScript"; spec Technical Approach. The Python transport needs a reader thread only because Python has no event loop (`xcl-python:xcl/_transport/subprocess.py:321`).
- **A native addon or job object on Windows to tie the engine to the host.** Rejected by the spec (no native addon, no FFI); the engine's exit on end of input is the safeguard (`xcl:cmd/xcl-core/serve.go` `serve`; spec Risks). xcl-python's job object (`subprocess.py:455`) is not carried over.
- **`context.metadata` / `Symbol.metadata` to carry standard-decorator field data.** Rejected: Node 22 defines no `Symbol.metadata`, and TypeScript and esbuild pass `undefined` for it there. Block classes have no class decorator, so a pending-list claimed by `@Entity` cannot work either. Chosen: a per-class field table filled by `context.addInitializer` at the probe construction that registration performs anyway (typescript-entity-types.md: "xcl reads them by constructing the class once at registration").
- **`emitDecoratorMetadata`.** Rejected by typescript-entity-types.md ("Why repeat the type in the decorator").
- **Zod, an entity builder, schema files.** Rejected by typescript-entity-types.md.
- **`auto-tag.yml` for xcl-typescript releases.** Rejected: it derives the next version from the repository's own tags and a PR label (`xcl:.github/workflows/auto-tag.yml`), so the npm version could not equal the xcl tag as the spec's "Release publishes matching package" and typescript-sdk.md "Releases" require. xcl-python's package and engine are versioned separately (`xcl-python:.github/workflows/release.yml:1-8`, `xcl/_version.py`); the sentence "as for xcl-lang on PyPI" in typescript-sdk.md is out of date on that point, and the TypeScript rule is unaffected.
- **Install checks only after publish, as xcl-python does.** Rejected as the only gate: a red check after publish cannot stop a release (`xcl-python` knowledge `gotchas/install-check-runs-after-publish.md`). Each platform's tarballs are installed and tested before publishing; a smaller check still follows publishing.
- **Recursive camelCase conversion of generic entity attributes.** Rejected: without a schema a nested object cannot be told from a map attribute, and map keys are user data (`env["LOG_LEVEL"]`, `entity/path.go`). Only attribute names at the top level are converted.
- **Tests that read README, PARITY.md, workflow files or the package's own source (a doc-comment test like `xcl-python:tests/unit/test_docstrings.py`).** Rejected: forbidden by knowledge `conventions/testing-and-mocking.md`. These go to manual checks.

## Chosen approach — evidence

- Three layers and a fake transport: libxcl.md "SDKs: a typed client, a byte transport, and a hand-written API"; typescript-sdk.md "Package layout"; model `xcl-python:xcl/_client.py`, `xcl/_transport/subprocess.py`, `tests/fake_engine.py`.
- Frame contract: `xcl:proto/xcl/core/v1/transport.proto` (`Request`, `Response`; status is two's-complement `uint32`); `xcl:cmd/xcl-core/serve.go` (256 MiB frame limit; `Release` is a reserved method name, not a CoreService method; responses arrive out of order; a garbled frame exits 1).
- Engine lifecycle: `xcl:cmd/xcl-core/main.go` header comment: end of input or SIGTERM releases every configuration, saving the state of an operation in progress, and exits 0; SIGINT and Ctrl-C are discarded, so the host decides; stdout carries frames only.
- Poll loop and tasks: `xcl-python:xcl/_poll.py`, `xcl/aio.py:151-236` (one consumer for events, each state or function task concurrent, late `Complete` answered with `invalid_argument` and ignored).
- Operations are refused while one runs: `xcl:coreservice/operations.go:37`, `coreservice/errors.go:65`.
- Function name clashes come from the engine, including two in one registry: `xcl:coreservice/config.go:168` returns every declared function; `registry` reports the clash.
- Schema v1 reader and its exact rules: `xcl:internal/schema/v1.go` (`singleton`, `nullable`, `label`, `default`); expected documents `xcl:bindings/libxcl/testdata/parity/*/types.json`. Observed rules: `computed` carries `nullable: true` and no `optional`; an optional sensitive field is `optional` and never `nullable`; a no-initializer optional is `optional` + `nullable`; a required sensitive is neither; `@Blocks` and `@NamedBlocks` carry no `optional`; a named block's type is `{"map": {"object": [...]}}`; an element's label field is dropped from `fields` and named by `label`; an empty-list initializer gives `optional` with no `default`; `""` is a real default; integer is `format: int`, float `format: float`.
- Derivation and hydration to mirror: `xcl-python:xcl/_schema.py` (`schema_for`, `_field_doc`, `hydrate`, `record_of`, `_unchanged`), `xcl/_entity.py`, `xcl/_types.py`, `xcl/_errors.py`.
- Parity: `xcl:bindings/libxcl/cmd/parity/main.go`, fixtures `bindings/libxcl/testdata/parity` with its README; runner to mirror `xcl-python:tests/parity/run_python.py`, `tests/parity/test_parity.py`, `tests/parity/test_types.py`, `tests/parity/types.py`.
- Diff path: `xcl:entity/path.go` marshals a path as its written string (`image`, `ports[0].host`, `env["LOG_LEVEL"]`).
- Outputs are keyed by address: `xcl:config.go:354`.
- Release assets: eight archives including `darwin_amd64` (`xcl:.github/workflows/release.yml`); dispatch to xcl-python is one job using `XCL_PYTHON_DISPATCH_TOKEN`; bindings gate `xcl:.github/workflows/bindings.yml` documents "To add a language, call its suite as another job and add that job to the needs of bindings".
- Bump, test and release flow to mirror: `xcl-python:.github/workflows/engine-bump.yml`, `test.yml`, `ci.yml`, `release.yml`, `.github/scripts/musl-wheel.sh`, `.github/scripts/install_check.py`.
- Test plugin: `xcl-plugin-test` `test "item"` blocks append each operation to a log (`xcl-plugin-test:README.md`), pinned at `v0.3.1` by `xcl-python:tests/plugins.toml`; tamper and untrusted-key tests served by a fake GitHub API (`xcl-python:tests/integration/test_github_tamper.py`); fixture plugins `slow` and `recording` from `xcl:bindings/libxcl/testdata/plugins`.

## Files examined

- xcl-typescript: empty apart from LICENSE, README and `.spektacular`.
- xcl-python:xcl/_transport/subprocess.py: frame reader, handle map, stop sequence, generation handling.
- xcl-python:xcl/_client.py: descriptor-driven typed client, version check on first use.
- xcl-python:xcl/_poll.py, xcl/aio.py: operation driver, cancellation, async answers.
- xcl-python:xcl/_schema.py: schema v1 derivation, hydration, entity records.
- xcl-python:xcl/config.py, registry.py, _entity.py, _types.py, _functions.py, _classes.py, events.py, diff.py, mask.py, _errors.py, _version.py: the public surface and its semantics.
- xcl-python:tests/parity/*, tests/fake_engine.py, tests/engines.py, tests/xcl_source.py, tests/integration/test_engine_process.py, test_aio.py, test_github_tamper.py, test_test_plugin.py, release_helpers.py: test structure to mirror.
- xcl-python:Makefile, xcl-ref.txt, PARITY.md, .github/workflows/*, .github/scripts/*: build, pin, release.
- xcl:proto/xcl/core/v1/core.proto, transport.proto: the current protocol (FunctionTask present; Log, StateGet, DescribeTypes absent, so not used).
- xcl:cmd/xcl-core/main.go, serve.go: engine program behaviour.
- xcl:coreservice/errors.go, operations.go, config.go, functions.go: error codes, operation guard, function declaration.
- xcl:internal/schema/v1.go: schema v1 rules.
- xcl:entity/path.go, diff/diff.go, config.go:354: diff JSON, path form, outputs.
- xcl:.github/workflows/release.yml, bindings.yml, auto-tag.yml: dispatch, bindings gate, tagging.
- xcl:bindings/libxcl/cmd/parity/main.go, testdata/parity/README.md, testdata/parity/*/types.json: parity contract.
- xcl-plugin-test:README.md: test plugin behaviour.

## External references

- TC39 decorators and TypeScript 5.0 decorators: field context shape, `addInitializer`, legacy `(target, key)` form.
- `@bufbuild/protobuf` v2 and `protoc-gen-es`: generated `*_pb.ts`, service descriptors, `create`, `toBinary`, `fromBinary`, `bigint` for `uint64`.
- esbuild's packaging: platform packages in `optionalDependencies` with `os`, `cpu`, `libc`, and `preferUnplugged` for Yarn PnP.
- npm trusted publishing: needs npm 11.5.1 or later, `id-token: write`, and a per-package publisher configured on npmjs.com, so a package must exist before it can use it.
- Node docs: `subprocess.unref`, `net.Socket.unref`, `process.report`, signals not delivered to worker threads, `AbortSignal.any`, `monitorEventLoopDelay`, `Symbol.asyncDispose`.

## Prior plans / specs consulted

- Spec 20261031080136-264e0eab-typescript-core-sdk, the spec planned here.
- Designs `typescript-sdk.md`, `typescript-entity-types.md`, `libxcl.md`, source `design`: binding.
- Python specs named in the spec's `sources` were not re-read; the code and knowledge entries are the current picture.

## Open assumptions

- `xcl-lang` and the eight platform names are free on npm.
- GitHub keeps offering `macos-15-intel` for the macOS x86-64 jobs.
- Node 26 is available to `actions/setup-node` and as a container image.
- npm's `libc` filter may be ignored by some package managers: the runtime C library check decides either way.
- The next xcl release after the pin carries the protocol this plan was read against.

## Drafting assumptions

### `entities()` returns registered classes (discovery)
- **Decision**: `config.entities()` returns an instance of the registered class where there is one, and a generic `Entity` otherwise. `find(address)` with no class stays generic. `typescript-sdk.md` is corrected to say so.
- **Rationale**: the spec's "Lookups return typed entities" asks for it and xcl-python behaves this way. typescript-sdk.md said `entities` returns `Entity` values. The orchestrator settled this as option A from the user's approval of the spec; the user was away, so it is listed for review.
- **Rejected**: always generic, which the spec's acceptance criterion forbids.

### Platform engine packages are not separate packages (discovery)
- **Decision**: `xcl-lang` plus eight platform packages stands alongside `decisions/one-package-per-language.md`. A clarifying line was added to that entry.
- **Rationale**: the user approved the esbuild-style layout on 2026-10-31 and the spec requires it. A user installs one package and all nine share a version.
- **Rejected**: one npm package carrying all eight binaries, which the spec's constraints rule out.

### Generic entity attribute names convert at the top level only (discovery)
- **Decision**: a generic entity's top-level attribute names are camelCase; nested values are left as the engine wrote them. Output names convert in the last address segment only. Diff paths convert attribute segments and leave map keys and indexes.
- **Rationale**: with no schema, a nested object cannot be told from a map attribute, and map keys are user data.
- **Rejected**: recursive conversion, which corrupts map keys.

### npm version equals the pin; only engine bumps release (discovery)
- **Decision**: `tag.yml` tags a merge to main with the tag in `xcl-ref.txt` when that tag does not exist yet. A package-only change ships with the next xcl release.
- **Rationale**: the spec requires the same version as the xcl tag, which `auto-tag.yml` cannot guarantee.
- **Rejected**: xcl-python's separate version line, which breaks the spec's "same version".

### Chosen direction: one transport per module graph, poll loop per operation, field table filled at the registration probe (architecture)
- **Decision**: build the three layers the designs fix, with a module-level engine (per thread), reference counting of outstanding calls for `ref`/`unref`, one poll loop per operation with concurrent function and state tasks and a serial event queue, and decorators that fill a per-class table: directly in legacy mode, through `addInitializer` at the registration probe in standard mode.
- **Rationale**: this is the only way to read standard-decorator fields on block classes with no class decorator on Node 22, and it needs no compiler setting. Event-loop reads make `Poll` cost nothing.
- **Rejected**: `Symbol.metadata` (absent on Node 22), a pending list claimed by `@Entity` (block classes have no class decorator), a reader thread (needless on an event loop), an engine per configuration (the spec forbids it).

### Conventions selected (architecture)
- **Decision**: keep testing-and-mocking, no repository-inspecting tests, config-files-are-xcl, no-compatibility-before-stable, release-plan-after-implementation, spec-worktrees and dependencies. Drop the Go-only entries (code style, parser options, hclsyntax references, error package, testutil, plugin testing, graph ordering, state fixtures).
- **Rationale**: the dropped entries concern Go code this plan does not touch.
- **Rejected**: listing every convention.

### Handlers and tasks are not awaited after a cancel (architecture)
- **Decision**: after an abort the promise rejects on `OperationFinished` without waiting for outstanding handlers or tasks. Without an abort, the operation waits for every event handler.
- **Rationale**: the spec bounds the rejection at two seconds and a program's own promise can hang.
- **Rejected**: awaiting everything, as xcl-python's asyncio driver can, because it cancels its tasks and JavaScript promises cannot be cancelled.

### A failing event handler warns and does not stop the operation (architecture)
- **Decision**: an exception in `onEvent` is reported with `process.emitWarning`; the operation and later events go on.
- **Rationale**: xcl-python's asyncio driver ignores such failures so the operation is not affected; a warning avoids losing them silently.
- **Rejected**: failing the operation, which differs from xcl-python.

### `Sensitive` is both a decorator factory and a constructor (components)
- **Decision**: one exported `Sensitive` value: `Sensitive(String)` is the decorator and `new Sensitive("s3cret")` builds a value; `Sensitive.redacted()` makes one with no value. They are told apart by `new.target`.
- **Rationale**: the design uses `@Sensitive(String)` and types the field `Sensitive<string>`, while the spec requires hand-built entities, and a hand-built sensitive value needs a constructor. The name must serve both.
- **Rejected**: a second export for the value, which the design's type name would not match.

### Every `Meta` field but `name` is optional in the type (components)
- **Decision**: `Meta` has `name: string` required and the rest optional.
- **Rationale**: the design's `draft.meta = { name: "web" }` must compile in a strict project.
- **Rejected**: all fields required, which refuses the design's own example.

### Small additions for Go parity (components)
- **Decision**: add `GitHubRegistry.register`, array-form `registerExternalPlugin`, `eventBufferSize`, `eventMask`, `noEventMask`, `revealSensitive` on `diff`, and an `Any` attribute type.
- **Rationale**: the protocol carries them, the Go API and xcl-python have them, and the parity record lists every Go operation. The design does not forbid them.
- **Rejected**: leaving them out, which leaves gaps in the parity record.

### Node's built-in test runner (implementation_detail)
- **Decision**: tests use `node:test` and `node:assert/strict`, compiled by `tsc`.
- **Rationale**: no extra dependency, and compiling tests with `tsc` under both settings is what the decorator modes need.
- **Rejected**: vitest, whose esbuild transform would hide compiler-mode differences.

### Signal tests run on Linux and macOS, Windows is a manual check (testing_approach)
- **Decision**: the Ctrl-C acceptance tests are automated on POSIX; Windows goes to the test plan.
- **Rationale**: `child.kill("SIGINT")` on Windows ends the process outright, and sending a console Ctrl-C needs native code the spec rules out, even in a helper.
- **Rejected**: a native test helper; skipping the case altogether.

### Parity omits printing keys (testing_approach)
- **Decision**: the TypeScript record and comparison leave out `printed_*`, `log` and `pretty_log`.
- **Rationale**: printing and the logger are the next spec's work and a non-goal here.
- **Rejected**: producing these keys now.

### First publish of the nine package names is a one-off by a person (tasks)
- **Decision**: a person publishes each name once as a placeholder to set up trusted publishing.
- **Rationale**: npm cannot set a trusted publisher on a package that does not exist yet. Every later release needs no manual step, as the spec asks.
- **Rejected**: a long-lived npm token in CI, which the spec's trusted publishing rules out.

### macOS x86-64 is tested even though xcl-python dropped it (tasks)
- **Decision**: the matrix includes `macos-15-intel`.
- **Rationale**: the spec names macOS x86-64 among the eight platforms. xcl's release already builds it.
- **Rejected**: dropping it to match xcl-python, which contradicts the spec.

## Rehydration cues

- `spektacular spec file read 20261031080136-264e0eab-typescript-core-sdk`
- `spektacular design read --data '{"source":"design","path":"typescript-sdk.md"}'`, then `typescript-entity-types.md`, `libxcl.md`
- `spektacular knowledge read` for `architecture/release-chain.md` (xcl), `gotchas/install-check-runs-after-publish.md`, `gotchas/ci-regenerates-stubs.md`, `learnings/test-suite-in-a-spec-worktree.md` (xcl-python)
- Re-read `xcl-python:xcl/aio.py`, `xcl/_schema.py`, `xcl/_transport/subprocess.py`, `tests/parity/run_python.py`, `.github/workflows/release.yml`
