---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Plan: 20261011130709-aa3eb933-python-support

<!-- Metadata -->
<!-- Created: 2026-10-11T14:51:31Z -->
<!-- Commit: 021b4ec4aaabdce2b0b6af19356647fdd7317d37 -->
<!-- Branch: f-libxcl -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Python developers can't use XCL today, because it exists only as a Go library. This plan brings the same Go engine to Python. The engine is compiled once as a shared library and driven through one message-based core service. A new `xcl` package on PyPI offers a synchronous and an asyncio API over it. Python programs can therefore validate, diff, apply, load and destroy configuration exactly as Go programs do, use the same Go plugins, and add their own Python functions to configuration. It serves people building command-line and Jumppad-style tools in Python, and teams running asynchronous Python services. Go users gain context-taking operations and untyped lookups, and are otherwise unaffected. The work spans the xcl repository and the new xcl-python repository, and ends with every xcl release publishing a matching Python package.

## Conventions

- **Testing & mocking (testify `require`, no table-driven tests, positive and negative cases in separate functions, tests beside the code)** — every new Go test in `coreservice`, `internal/schema`, `registry` and the root package follows it. The xcl-python suite follows the same rules with pytest: no `parametrize` tables, and one positive or negative case per test function.
- **Never write tests that inspect repository files** — the CHANGELOG entry, the Python README, the parity list and the CI/release workflows are checked by human review, never by a test. Docstring presence is checked by a test that imports the package and inspects its public objects, which is code, not repository text.
- **Shared test helpers live in `internal/testutil`** — the helper that builds the Go fixture plugins used by the root-package, coreservice and parity tests goes there. coreservice-only helpers stay in coreservice's `_test.go` files.
- **Generate test state with a real apply** — the coreservice, parity and Python state tests produce saved state by applying, never by hand-writing a state file.
- **Shared error types live in the `errors` package** — coreservice maps the existing sentinels and detail types with `errors.Is`/`errors.As`. It adds no error types of its own beyond an unexported internal status.
- **Project structure (public packages at the module top level; `/api`-style protos)** — `coreservice/` sits at the top level as a public package, `proto/xcl/core/v1/` holds the protobuf, and `bindings/libxcl/` is a nested module.
- **Dependencies (prefer stdlib, document and pin third-party deps)** — the root module adds no Go dependency, because `google.golang.org/protobuf` is already required. The nested libxcl module pins only the root module. xcl-python pins `protobuf` and `cffi` lower bounds, and the reason for each is recorded in `pyproject.toml` comments.
- **Code style (`any` over `interface{}`, gofmt/vet)** — all new Go code.
- **Patterns & architecture (dependency injection, context.Context for cancellation)** — the context-taking operations carry cancellation from `Cancel` into the engine. coreservice takes its root-module dependencies as parameters, and the Python SDK takes its transport as a parameter so tests can use a fake transport.
- **Development standards (structured logs)** — coreservice logs each call by method name through the event logger when events are on, as the design's cost note requires.
- **Assert ordering on graph parents, not provider call order** — the cancellation test asserts only that no operation *started* after the cancel. It does not assert on order between unrelated entities.
- **Glossary: entity** — Python names and docstrings say "entity" (`xcl.Entity`, `find_by_type`) for anything a configuration declares.

Not applied:
- Never modify dependency packages: no fork or module-cache code is touched.
- Database and external services: no database is involved.
- Testing provider `Changed`/`Update`: no provider logic is written.

## Architecture & Design Decisions

The shape is fixed by the binding design `libxcl.md`: the Go engine is compiled once as a C shared library, a pure-Go `coreservice` package holds all binding logic behind `Service.Call(handle, method, bytes)`, four C functions carry protobuf messages, and the Python SDK polls for work. The options below are about how to build that shape for this spec's scope: Python apps with Go plugins only, plus Python functions.

**Options weighed.**

1. **Python functions as application functions, built from a declared signature, plus one Python poll engine with two drivers, and a release that builds libxcl once in xcl** (chosen).
   - *Functions.* coreservice turns each declared Python function into a Go func with `reflect.FuncOf`/`reflect.MakeFunc`, using its `FunctionSignature`. Its registry offers the funcs through the `Functions()` method language-surface adds to `registry.Registry`. The made func encodes its arguments as JSON, enqueues a `FunctionTask` under the running operation's context, and waits for `Complete`.
   - *Python.* One `_Operation` state machine owns the poll loop. `xcl.Config` drives it on the calling thread. `xcl.aio.Config` drives it from an executor thread and dispatches tasks as asyncio tasks.
   - *Release.* The xcl tag workflow builds, signs and publishes the eight libxcl archives. xcl-python's release downloads and verifies them into wheels.
   - *Pros:* Python functions behave exactly like Go application functions: a clash fails `NewConfig`, they go through the same Go-func conversion, and sensitive values get the same redaction. This is what the spec's constraint asks for. It needs no internal package from coreservice (design § Layers). Sync and async can't drift. There is one native artifact per platform for every future language (design § One message entry point).
   - *Cons:* Numbers cross as `float64`. It needs a cross-repo release trigger, which in turn needs a token secret.
   - *Effort:* High.
2. **Python functions as a synthetic in-process plugin** that answers `GetFunctions`/`CallFunction` from the task queue.
   - *Pros:* it reuses the catalog's signature-to-cty path unchanged and keeps numbers exact.
   - *Cons:* a clash surfaces at the first operation, naming a plugin. That contradicts the language-surface rule that application-function clashes fail `NewConfig`. The catalog calls plugin functions with `context.Background()`, so cancellation can't reach them.
   - *Effort:* Medium.
3. **The same runtime as option 1, but each SDK release builds libxcl from the xcl tag itself.**
   - *Pros:* no cross-repo trigger.
   - *Cons:* a second, unsigned build of the same library per language. The xcl GitHub release would carry no libxcl assets, which the design's fallback route and signed-asset contract need (design § Distribution). The release would be driven from the SDK rather than from the xcl tag the spec names.
   - *Effort:* Medium.

**Shape of the chosen solution.**

*xcl root module (no cgo, additive).*
- The five operations gain context-taking forms: `ValidateContext`, `ApplyContext`, `DiffContext`, `DestroyContext` and `LoadContext`. `run` derives its context from the caller's, and the existing methods pass `context.Background()`.
- `Config` gains `FindEntitiesByType(path...)` and `FindOneEntity(path...)`, untyped lookups with exactly the errors `FindByType`/`FindOne` give.
- `internal/schema` gains the schema v1 reader beside the legacy one. `CreateInstanceFromSchema` sniffs which form it was given. The reader is exposed as `registry.TypeFromSchema`.
- Schema v1 is extended for language-surface's named blocks:
  - a block field typed `{"map": {"object": […]}}` is a `map[string]struct` tagged `name,block`;
  - `"label": "<field>"` adds the optional name label field to an element.
- The legacy schema keeps travelling under plugin protocol 1.

*coreservice (new package, root module, no cgo).*
- `proto/xcl/core/v1/core.proto` is generated with `buf`.
- `Service`, with `cgo.Handle` handles. A hand-written dispatch table is held to the service descriptor by a test.
- `CreateConfig` turns each message into `xcl.NewConfig` options and builds a host registry. That registry wraps a `registry.Local` for types, external plugins and plugin directories, and implements `Functions()` for the declared Python functions.
- A per-handle task queue carries `Event`, `StateTask`, `FunctionTask` and `OperationFinished` in one ordered stream. `Cancel` cancels the operation's context. The same context is the one the made function funcs and `ForeignStateStore` wait under.
- Errors are mapped to the design's code table. Language-surface adds three codes: `function_name_clash`, `invalid_function` and `invalid_type`.
- Provider tasks, `Log`, `StateGet`, `StateFind`, `DescribeTypes` and the `provided`/`implements` fields are not built. Their design field numbers are `reserved`, so the later Python-providers spec adds them unchanged.

*bindings/libxcl (nested module, cgo).*
- The four exports, built with `-buildmode=c-shared` and stamped with the release version through `-ldflags -X`.
- A release workflow builds the eight targets with zig cc, llvm-mingw and macOS runners. It publishes `libxcl_<version>_<os>_<arch>` archives with signed checksums to the xcl GitHub release, then dispatches xcl-python's release.
- Shared parity fixtures and a Go parity program live beside it, so Python can be compared with Go on every platform.

*xcl-python (new repository, `xcl` on PyPI).*
- Follows the design's layout: `_transport` (cffi ABI mode, ctypes fallback, lazy load, fork guard), `_client` (built from the descriptor), `_errors`, `_schema` (dataclass annotations to schema v1, and JSON back to instances), `_poll`, `Config`, `aio.Config`, `LocalRegistry`, `GitHubRegistry`, `Sensitive`, `Computed`, `Entity`, `mask`.
- There is no `Provider`, `Plugin` or `generate`.
- On first use the package checks `xcl_abi_version()`, then checks `GetVersion` against its own version.
- Wheels are tagged `py3-none-<platform>` for the eight platforms and published through PyPI Trusted Publishing.

**Why this direction.**
- It is the only option that keeps Python functions identical to Go application functions, which is the spec's constraint and the language-surface clash rule.
- It reaches every root-module capability through public API, as the design requires.
- It gives the sync and asyncio APIs one loop, as the spec's Technical Approach asks.
- It ships one signed libxcl per platform from the xcl tag, so "every XCL release publishes a Python package with the same version" is driven by the tag itself.

Errors follow `conventions/shared-errors-package.md`: no new Go error types are needed, and Python maps the codes. The root-module additions are public and at the module's top level (`conventions/project-structure.md`). The evidence for each rejected option is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

**xcl library (root module, no cgo)**

- **Config operations** (changed): the five operations gain context-taking forms. The run loop derives its context from the caller's, so cancelling stops new provider calls and saves progress, as a panicking event handler already does. The existing methods keep their signatures and behaviour. coreservice is the first caller, and Go users can use them too.
- **Config lookups** (changed): two untyped lookups by type segments, one for many entities and one for exactly one, as public methods on `Config`. They share the rules `FindByType`/`FindOne` already use, so they return the same `NotFound`, `NotUnique`, `UnknownType` and `NotTypeable` errors without needing a Go type.
- **Schema reader** (changed, `internal/schema`): gains the schema v1 reader beside the legacy one, and picks the form from the document. It builds a Go struct type with the tags the parser, decoder, computed carry-over and change detection already understand:
  - attributes, blocks and repeated blocks;
  - named-block maps and the optional name label;
  - optional, default, sensitive (limited to the known instantiations) and computed fields.

  The legacy producer is unchanged.
- **Registry** (changed): gains `TypeFromSchema`, the public way to turn a schema v1 document into a declared `registry.Type`. coreservice uses it, and so could any non-Go host.
- **Changelog** (changed): one entry for this spec describing the additive Go API, coreservice and libxcl.

**coreservice (new, root module, no cgo)**

- **Protocol definition**: `core.proto` in `xcl.core.v1`, generated for Go with `buf`. It is the single description of what the engine does for a host. `buf lint` and `buf breaking` keep it stable.
- **Service and dispatcher**: holds configurations by handle, maps each method name to a typed handler, and decodes requests generically from the descriptor. It returns the design's statuses. A test holds the handler table to the service descriptor.
- **Configuration builder**: turns `CreateConfigRequest` into `xcl.NewConfig` options:
  - registries;
  - the state path or the host store;
  - the state and event masks;
  - the event data level and buffer;
  - variables.

  Each local registry message becomes a host registry. That registry wraps a `registry.Local` for schema-declared types, external plugins and plugin directories, and offers the declared host functions through `Functions()`. GitHub registry messages map to `registry.NewGitHub` with every option.
- **Task queue and operations**: one queue per handle. An operation starts on a goroutine under a cancellable context. Events, state-store calls and function calls are posted as tasks, and the operation's end is posted as the last task. `Poll` hands tasks out in order, and `Complete` wakes the waiting goroutine. `Cancel` cancels the context, and every waiting task returns at once. While an operation runs, lookups are refused.
- **Host function bridge**: builds a Go func from each declared function signature. The registry offers it, so the catalog converts and checks it exactly like a Go application function. Each call becomes a `FunctionTask` under the running operation's context, and the result or error comes back to the configuration.
- **Host state store and event bridge**: a `state.StateStore` whose four methods are `StateTask`s, and an event handler that posts each event as a task. Both block until the host has taken or answered the task, so backpressure behaves as it does in Go.
- **Lookups**: entities, find by address, by type, exactly one, outputs and saved-entity encoding, all returned as JSON with sensitive values as the marker.
- **Error mapper**: maps every root-module sentinel and detail type to its stable code with details and causes, and collects configuration errors as causes with file and line.

**bindings/libxcl (new nested module, cgo)**

- **C ABI**: four exports that move bytes between C and `Service.Call`/`Release`. It uses C-allocated responses and no Go pointers. The version is stamped at link time.
- **Fixture plugins and parity harness**:
  - a slow, recording Go plugin for the thread, event-loop and cancellation tests;
  - shared parity fixture configurations;
  - a Go parity program that records validation errors, the diff, entities, outputs, state contents, load and destroy results as JSON for the Python side to compare.
- **Build and release pipeline**:
  - CI builds the library with cgo, and keeps the root module's `CGO_ENABLED=0` guard covering coreservice.
  - A tag workflow builds the eight targets and publishes signed `libxcl_<version>_<os>_<arch>` archives with checksums to the xcl GitHub release.
  - It then triggers xcl-python's release.

**xcl-python (new repository)**

- **Transport**: loads the bundled libxcl lazily on first use, with cffi in ABI mode and ctypes as the fallback. It releases the GIL during calls and frees every response buffer. It refuses any call in a process forked after first use, raising `ForkedProcessError`, which names `spawn` and creating the configuration after the fork. It checks the ABI version on load.
- **Client and errors**: builds and decodes messages from the generated descriptor. It maps status `1` errors to one exception class per code under `xcl.Error`, carrying the message, details and causes. Configuration errors list each problem with its file and line. On first use it checks that the engine version equals the package version.
- **Schema and hydration**:
  - Derives schema v1 from dataclass annotations: optional, default, `Sensitive`, `Computed`, nested block, list of blocks, dict of named blocks.
  - Registers entity classes with `@xcl.entity(type, subtype)`.
  - Turns entity JSON back into registered class instances, or into `xcl.Entity` for unregistered types.
  - Derives a `FunctionSignature` from a Python function's annotations.
- **Registries and options**: `LocalRegistry` (types, functions, external plugins, plugin directories) and `GitHubRegistry` (plugins, cache dir, trusted keys, token, API URL) are plain message builders. `Config` options cover state path or store, masks, variables, event handler, event data and buffer.
- **Poll engine**: one implementation of the operation loop. It starts the operation and polls, dispatching function tasks to the registered callables, state tasks to the store and events to the handler. It ends on the finished task, which raises the mapped exception or returns the diff. On `KeyboardInterrupt` or cancellation it sends `Cancel` and drains to the finished task.
- **Synchronous API (`xcl.Config`)**:
  - `validate`, `diff`, `apply`, `load` and `destroy`;
  - the lookups `entities`, `entity_count`, `find`, `find_by_type`, `find_one`, `all` and `outputs`;
  - `encode_saved_entity` and `close`.

  It is a context manager. Each call drives the poll engine on the calling thread, with tasks run serially.
- **Asynchronous API (`xcl.aio.Config`)**: the same surface as awaitables. Polling runs on a dedicated executor thread. Function, handler and store calls run as asyncio tasks when they are `async def`, and in the default executor otherwise. Cancelling the awaiting task cancels the engine's work.
- **Values**: `Sensitive[T]` (masked repr, `reveal()`), `Computed[T]`, `Entity` (a mapping with `.meta`), `Event`, `Diff`, and the `mask` constructors (`aes256gcm`, `hmac_sha256`, `redact`, `omit`).
- **Packaging and release**: generated `_pb2` modules from the xcl tag's protos, and wheels tagged `py3-none-<platform>` with the bundled library for the eight platforms. The release workflow downloads and verifies the xcl release's libxcl archives, builds the wheels, publishes them with Trusted Publishing, then installs and smoke-tests from PyPI on each platform.
- **Docs**: a README covering install, synchronous getting started against the Docker plugin, then asyncio. Every public name has a docstring. A parity list maps each Go operation and lookup to its sync and async equivalents.

## Data Structures & Interfaces

**Root module additions (Go, additive)**

```go
// Context-taking operations; the existing methods call these with context.Background().
func (c *Config) ValidateContext(ctx context.Context, paths ...string) error
func (c *Config) ApplyContext(ctx context.Context, paths ...string) error
func (c *Config) DiffContext(ctx context.Context, paths []string, options ...diff.Option) (*diff.Diff, error)
func (c *Config) DestroyContext(ctx context.Context) error
func (c *Config) LoadContext(ctx context.Context) error

// Untyped lookups by type segments, with the same errors FindByType/FindOne give.
func (c *Config) FindEntitiesByType(path ...string) ([]any, error)
func (c *Config) FindOneEntity(path ...string) (any, error)

// registry: a declared, config-only type built from a schema v1 document.
func TypeFromSchema(schema []byte) (Type, error)
```

A cancelled operation returns an error wrapping `context.Canceled`, after saving the progress made.

**Schema v1 (JSON, the type contract between a host and the core)**

This is the design's format: `schema_version`, `type`, `subtype`, and `fields` with `name`, `type`, `kind`, `repeated`, `optional`, `sensitive`, `computed`, `default` and `format`. It has two additions for language-surface's named blocks:

```json
{"name": "listener", "kind": "block", "label": "name",
 "type": {"map": {"object": [{"name": "port", "type": "number", "format": "int"}]}}}
{"name": "volume", "kind": "block", "repeated": true, "label": "name",
 "type": {"object": [{"name": "source", "type": "string"}]}}
```

- A `block` whose type is `{"map": {"object": …}}` is a map of named blocks keyed by name.
- `label` names an element field filled with the block's name. It is optional in a list and filled from the key in a map.

**CoreService (`proto/xcl/core/v1/core.proto`, package `xcl.core.v1`)**

These are the design's messages. Only what this spec builds is defined. The provider-only numbers are `reserved`.

```protobuf
service CoreService {
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);
  rpc CreateConfig(CreateConfigRequest) returns (CreateConfigResponse);
  rpc StartValidate(StartValidateRequest) returns (StartOperationResponse);
  rpc StartApply(StartApplyRequest)       returns (StartOperationResponse);
  rpc StartDiff(StartDiffRequest)         returns (StartOperationResponse);
  rpc StartDestroy(StartDestroyRequest)   returns (StartOperationResponse);
  rpc StartLoad(StartLoadRequest)         returns (StartOperationResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);
  rpc Poll(PollRequest) returns (PollResponse);
  rpc Complete(CompleteRequest) returns (CompleteResponse);
  rpc Entities(EntitiesRequest) returns (EntitiesResponse);
  rpc Find(FindRequest) returns (EntityResponse);
  rpc FindByType(FindByTypeRequest) returns (EntitiesResponse);
  rpc FindOne(FindByTypeRequest) returns (EntityResponse);
  rpc Outputs(OutputsRequest) returns (OutputsResponse);
  rpc EncodeSavedEntity(EncodeSavedEntityRequest) returns (EncodeSavedEntityResponse);
}

message GetVersionResponse { string version = 1; uint32 abi_version = 2; }

message LocalRegistry {
  string plugin_pattern = 1;
  repeated DeclaredType types = 2;
  repeated string external_plugins = 3;
  repeated string plugin_directories = 4;
  repeated DeclaredFunction functions = 5;     // new: host-language functions
}
message DeclaredType     { string type = 1; string subtype = 2; bytes schema = 3; reserved 4, 5; }
message DeclaredFunction { string name = 1; bytes signature = 2; } // plugins.FunctionSignature JSON

message PollResponse {
  oneof task {
    StateTask state = 2;
    Event event = 3;
    OperationFinished finished = 4;
    FunctionTask function = 5;                 // new
  }
  reserved 1;                                  // ProviderTask, later spec
}
message FunctionTask   { uint64 task_id = 1; uint64 operation_id = 2; string name = 3; bytes args = 4; } // JSON array
message FunctionResult { bytes result = 1; Error error = 2; }                                             // JSON value

message CompleteRequest {
  uint64 task_id = 1;
  oneof result { StateResult state = 3; FunctionResult function = 4; }
  reserved 2;                                  // ProviderResult, later spec
}
```

The following are exactly as the design writes them:
- `CreateConfigRequest`, `Registry`, `GitHubRegistry`, `StateStore`, `Mask` and `EventDataLevel`;
- the operation requests, `StartOperationResponse`, `Cancel*` and `PollRequest`;
- `StateTask`, `StateResult`, `Event` and `OperationFinished`;
- the lookup messages and `Error`.

**C ABI (`bindings/libxcl`)**

The design's four functions: `xcl_abi_version`, `xcl_call`, `xcl_free` and `xcl_release`. Statuses are `0`, `1`, `-1` to `-4`, and the ABI version is `1`.

**Error codes**

These are the design's table plus three:
- `function_name_clash` (`FunctionNameClashError`; details `name`, `provider`, `registry`, `existing`, `existing_registry`);
- `invalid_function` (`InvalidFunctionError`; `name`, `registry`);
- `invalid_type` (`InvalidTypeError`; `type`, `registry`).

A `config` error carries each problem as a cause with `file`, `line`, `column` and `summary` in details.

**Python public API (`xcl`)**

```python
@xcl.entity("database", "postgres")          # type, optional subtype; class decorator
@dataclass
class Postgres(xcl.Resource): ...            # fields typed with T | None, defaults, xcl.Sensitive[T],
                                             # xcl.Computed[T], nested dataclass, list[Block],
                                             # dict[str, Block], xcl.field(name=..., block=True, label=...)

class LocalRegistry:
    def register_type(self, cls: type) -> None
    def register_function(self, fn: Callable, name: str | None = None) -> None
    def register_external_plugin(self, path: str) -> None
    def register_plugin_directory(self, path: str) -> None

class GitHubRegistry:
    def __init__(self, *, cache_dir=None, trusted_keys=(), token=None, api_url=None): ...
    def register_plugin(self, repository: str, version: str) -> None
    def register_type(self, cls: type) -> None

class Config:                                 # xcl.aio.Config: same names, every operation and lookup async
    def __init__(self, *, registries=(), state_path=None, state_store=None, state_mask=None,
                 variables=None, on_event=None, event_data="none", event_buffer_size=0,
                 event_mask=None, no_event_mask=False): ...
    def validate(self, *paths: str) -> None
    def apply(self, *paths: str) -> None
    def diff(self, *paths: str, reveal_sensitive: bool = False) -> Diff
    def destroy(self) -> None
    def load(self) -> None
    def entities(self) -> list[object]
    def entity_count(self) -> int
    def find(self, cls_or_none, address: str) -> object
    def find_by_type(self, cls_or_none, *path: str) -> list[object]
    def find_one(self, cls_or_none, *path: str) -> object
    def all(self, cls: type) -> list[object]
    def outputs(self) -> dict[str, object]
    def encode_saved_entity(self, data: bytes, *, highlight: bool = False) -> str
    def close(self) -> None                  # also __enter__/__exit__ (async with for aio)

class StateStore(Protocol):                   # sync, or async def under xcl.aio
    def load(self) -> list[bytes | dict]: ...
    def save(self, entities: list[bytes]) -> None: ...
    def exists(self) -> bool: ...
    def clear(self) -> None: ...
```

Exceptions: `xcl.Error(code, message, details, causes)` is the base class. There is one subclass per code: `NotFoundError`, `NotUniqueError`, `ConfigError`, `PluginLoadError`, `PluginVerificationError`, `FunctionNameClashError`, `CancelledError` (which also subclasses `asyncio.CancelledError` under `xcl.aio`), and so on. There are also `ForkedProcessError`, `EngineVersionError` and `InternalBindingError`.

Values: `Sensitive[T]` (masked `repr`/`str`, `reveal()`), `Computed[T]`, `Entity` (a mapping with `.meta`), `Event` (the `events.Event` fields, plus `entity()`), `Diff` (resources and changes with `before`/`after`/`unknown`), `UNKNOWN`, and `mask.aes256gcm(key)`, `mask.hmac_sha256(key)`, `mask.redact()`, `mask.omit()`.

**Serialization boundaries**
- Entity data crosses as JSON. Lookups carry the sensitive marker. State tasks carry real values, or masked envelopes when `state_mask` is set. Event data is masked by the event masker.
- Function arguments are a JSON array and the result is a JSON value, encoded against the declared cty types. Sensitive arguments arrive unmarked.
- Diffs cross as `json.Marshal` of `diff.Diff`. Variables cross as a JSON object.

## Implementation Detail

**A new module boundary, held one way.**
- Dependencies point one way: xcl-python → libxcl → coreservice → the root module's public packages.
- The root module never imports coreservice. coreservice reaches only public API, so every capability it needs is first added to the root module as an ordinary, documented Go API. That covers the context-taking operations, the untyped lookups and `TypeFromSchema`, and Go users benefit from each one.
- coreservice holds no transport code. A developer reads it as a plain Go package with plain `go test` tests against `Service.Call`, without a C compiler or Python.
- The cgo file is small enough to read in one sitting and contains nothing worth testing in Go beyond a build-and-call smoke test.

**Inversion of control through one ordered task stream.**
- This is a new pattern for xcl, taken from the design. The engine never calls into the host.
- Anything that needs the host posts a task on the handle's queue and waits for `Complete` or for the operation's context to end:
  - a function call during the walk;
  - a state-store method;
  - an event.
- Events and the finished task share the queue, so the host sees one ordered stream and the finished task is always last.
- One pattern covers all three task kinds. The queue's `run(ctx, task)` helper is the only place that waits, which makes cancellation uniform: cancelling the operation's context releases every waiter at once.

**Functions ride the existing registry path.**
- A declared Python function becomes a real Go func value, made by reflection from its signature. The registry offers it, and the catalog converts and clash-checks it at `NewConfig`, exactly as it does a hand-written Go function.
- The Go-func reflection, the sensitive redaction wrapper and the clash errors language-surface builds are reused unchanged, and no parallel function mechanism appears in the engine.
- The made func reads the running operation's context from the configuration. There is at most one operation per handle, so this is unambiguous.

**The root module's run loop gains a caller context, and nothing else changes.**
- `run` already cancels its context when an event handler panics, and the parser already stops starting provider calls and saves progress on cancellation.
- The context-taking methods thread the caller's context into that path, and the old methods become one-line wrappers. Behaviour without cancellation is unchanged.

**Schema v1 beside the legacy schema.**
- The reader is a second path in `internal/schema`, chosen by sniffing the document. It builds `reflect.StructOf` types whose tags are exactly those `internal/xcl/tags` parses, so the parser, decoder, change detection and computed carry-over run on them unchanged.
- It follows the existing rebuilt-plugin-type rules: `ResourceBase` first, and sensitive values only for the known instantiations (`gotchas/plugin-types-rebuilt-with-structof`).
- The legacy writer and protocol 1 are untouched.

**Python: three layers, one loop, two faces.**
- The SDK follows the design's layering: a bytes-only transport, a typed client built from the generated descriptor, and a hand-written idiomatic API. All FFI, GIL release, buffer freeing and fork detection sit in the transport.
- Tests above the transport run against a fake transport that wraps an in-memory script of responses. Integration tests run against the real library.
- The poll loop is a single `_Operation` object that yields tasks and accepts results. The sync `Config` drives it inline. `aio.Config` drives it from an executor thread and fans tasks out as asyncio tasks. Neither face has loop logic of its own.
- Schema derivation and hydration are pure functions over dataclass annotations, unit-testable without the engine.

**Cross-repo release flow, driven by the xcl tag.**
- Today xcl tags are pushed by hand with no workflow. This plan adds xcl's first release workflow, modelled on the plugin template's signed-asset release.
- Pushing a `v*` tag builds the eight libxcl targets, attaches signed archives to the GitHub release, and dispatches xcl-python's release with the tag.
- xcl-python's release:
  1. checks out xcl at the tag and generates the Python messages;
  2. downloads and verifies the archives with the same checksum and signature contract the plugin registry uses;
  3. builds the eight `py3-none-<platform>` wheels and an sdist, and publishes them with Trusted Publishing;
  4. installs from PyPI on each platform and runs the smoke check.
- Version equality holds by construction. The wheel's version is the tag without the `v`, the library is stamped with the tag, and the SDK checks both on load.

**Parity by shared fixtures.**
- The xcl repo owns a set of parity fixture configurations, the Go fixture plugins they use, and a Go program that runs each fixture through the Go API and writes a normalised JSON record.
- xcl-python's parity test runs the same fixtures through the Python API, writes the same record shape and compares the two. Both run in xcl-python's CI matrix on every platform.
- Records are normalised for temp paths and timestamps (`gotchas/event-data-includes-temp-paths`) and compared as decoded JSON, never as strings (`gotchas/file-state-is-indented-json`).

## Dependencies

- **`libxcl.md` from the `design` source** — the binding design this plan is built on: the coreservice package, the C ABI, the CoreService messages, the poll model, schema v1, the Python SDK layout, packaging and hazards. It is extended, not changed:
  - only the parts this spec builds are defined;
  - provider numbers are reserved;
  - `FunctionTask` is added along the "host-language functions" line the design sketches;
  - schema v1 gains the named-block map form and `label`;
  - three error codes are added.
- **Spec `20261011095738-fad05e41-language-surface` and its final plan — must land and be released first.** This plan builds on several things that plan delivers:
  - `Registry.Functions()` and `registry.Local.RegisterFunction`;
  - the `FunctionSignature` JSON contract;
  - the Go-func conversion (string, bool, numeric kinds, slices and string-keyed maps);
  - app-function clash checks at `NewConfig`;
  - `FunctionNameClashError`, `InvalidFunctionError` and `InvalidTypeError`;
  - named-block maps and the optional list label;
  - `XCL_VAR_` and the shared parser-options builder, to which the context-taking operations thread the caller's context.

  It ships in xcl v0.3.0. This plan's xcl work starts from that code.
- **xcl root packages** — used by coreservice without change: `xcl`, `registry`, `plugins`, `state`, `events`, `diff`, `mask`, `types`, `entity`, `errors` and `logger`. Some of them change additively:
  - `xcl` gains the context operations and untyped lookups;
  - `registry` gains `TypeFromSchema`;
  - `internal/schema` gains the v1 reader.
- **`google.golang.org/protobuf` v1.36.6** — already in the root `go.mod`. It encodes the CoreService messages, and needs no version change.
- **`buf` CLI** — new development and CI tool for `buf generate`, `buf lint` and `buf breaking` over `proto/`. The Makefile's existing `protoc` target for `plugins/plugin.proto` is left as is.
- **A C toolchain for cgo** — needed only by `bindings/libxcl`:
  - zig (`zig cc`) for the Linux glibc 2.28 and musl targets and for Windows x86-64;
  - llvm-mingw for Windows arm64;
  - the Xcode toolchain on GitHub's macOS runners.

  The root module stays `CGO_ENABLED=0`.
- **GitHub Actions runners** — `ubuntu-latest`, `ubuntu-24.04-arm`, an Intel macOS runner, `macos-latest` (arm64), `windows-latest` and `windows-11-arm`, with Alpine containers for the musl checks.
- **The jumppad-labs release signing key** — signs `libxcl_<version>_checksums.txt`, the same contract the GitHub plugin registry verifies. It is needed as an xcl repository secret (a person's task).
- **New repository `jumppad-labs/xcl-python`** — does not exist yet, on disk or in the Spektacular registry. A person creates it, registers it with `spektacular repo add`, and sets up the PyPI project `xcl` with a Trusted Publisher. A person also adds a dispatch token secret to xcl.
- **Python runtime dependencies (xcl-python)**:
  - `protobuf` (≥ 5, the upb runtime) for the message classes;
  - `cffi` (≥ 1.16) for ABI-mode loading, with ctypes from the standard library as the fallback.
- **Python development and CI dependencies**: `pytest`, `pytest-asyncio` (or `asyncio.run` in plain tests), `grpcio-tools` or `buf` for generation, `build`, `wheel` and `twine check`.
- **The Docker example plugin (`jumppad-labs/xcl-plugin-docker`)** — a signed release, rebuilt against xcl v0.3.0 by language-surface. Python's path, plugin-directory and registry acceptance tests use it, and they need a Docker engine on the Linux runner.
- **Prior work known from the CHANGELOG, used as precedent only**:
  - `20261009102148-7d0b205b-github-releases-registry` (the asset and signature contract reused for libxcl);
  - `20261009092551-82db0140-plugin-template` (the signed `make dist` release, the model for xcl's first release workflow).

  Neither needs to change.

## Testing Approach

**Kinds of test**

- **Go unit tests (xcl).** These follow the existing conventions: testify `require`, one behaviour per function, positive and negative cases apart, beside the code.
  - Schema v1 reader: each field form builds the expected tags and Go types, and an unsupported sensitive shape is refused with a clear error.
  - `TypeFromSchema`.
  - Untyped lookups: the same errors as the generic forms.
  - Context-taking operations: a cancelled apply saves progress and returns `context.Canceled`.
- **coreservice tests (Go, no cgo).** This is the bulk of the Go coverage, because coreservice is where the binding behaviour lives. Every test drives `Service.Call` with encoded requests, exactly as a binding would, through a small test client helper that polls and completes tasks. They cover:
  - every method has a handler (held against the descriptor), and each negative status;
  - `CreateConfig` mapping of each option;
  - each operation's finished task;
  - event order, with finished always last;
  - state tasks for a host store, with masking when a state mask is set;
  - function tasks: result, error, clash at `CreateConfig`, invalid signature;
  - cancellation releasing waiting tasks, and no provider call starting afterwards;
  - lookups refused during an operation;
  - error mapping for every code, including config causes with file and line;
  - lookups returning the sensitive marker.

  State comes from real applies.
- **libxcl smoke tests (Go with cgo, in the nested module).** Load the built library and call `xcl_abi_version`, `GetVersion`, `CreateConfig`, one validate and `xcl_release`, checking buffers are freed and statuses are right. Behaviour is not retested here.
- **Python unit tests (xcl-python, pytest, no engine).** Run against a fake transport:
  - schema derivation for every annotation form;
  - signature derivation for functions;
  - hydration into registered classes and `xcl.Entity`;
  - `Sensitive` masking;
  - error-code mapping to exception classes;
  - the version and ABI check;
  - fork detection (with a simulated pid change);
  - the poll engine's dispatch, drain and cancel paths.

  Pytest style mirrors the Go conventions: no `parametrize` tables, and positive and negative cases in separate functions.
- **Python integration tests (against the real libxcl).** One or more tests for each acceptance criterion:
  - registered class decode, and subtype addressing;
  - each annotation rule;
  - `slugify`, and an async function;
  - variables;
  - unregistered plugin types read generically;
  - outputs and a single-attribute diff;
  - state directory and custom store across a new process, including encryption;
  - events with redaction;
  - typed exceptions;
  - a second thread progressing during a sync apply;
  - other coroutines running during an async apply;
  - cancellation with the recording plugin, showing no operation started after the cancel;
  - fork protection in both directions;
  - version mismatch;
  - docstrings on every public name.

  The slow and recording Go fixture plugins come from the xcl checkout and are built in CI.
- **Go plugin tests (Python, Linux with a Docker engine).** Apply with the Docker example plugin loaded from a path, from a plugin directory and from the GitHub registry. A local fake GitHub API serves a release whose checksums file has been tampered with, and the install is refused with the verification error. These skip when no Docker engine is reachable, as the plugin's own real-engine tests do.
- **Parity tests (cross-language contract).** The Go parity program and the Python parity test run the shared fixtures. They compare, as decoded and normalised JSON:
  - validation errors;
  - the diff;
  - entities and outputs after apply;
  - state file contents;
  - entities after a load in a new process;
  - the empty state after destroy.

  They run in xcl-python's CI matrix on all eight platforms and gate the release.
- **Release checks (workflow, not unit tests).** After publishing, the release workflow installs `xcl==<version>` from PyPI on each of the eight platforms with no Go or C compiler on the path. It runs `python -c "import xcl; xcl.Config().close()"`, and checks that the engine reports the tag.

**Most coverage** goes to coreservice and the Python poll engine. Together they carry the inversion of control, cancellation, ordering and error mapping that every other behaviour rests on.

**What the tests guarantee**
- A Python program gets the same validation errors, diffs, entities, outputs and state as a Go program for the same inputs.
- Python functions are application functions: same conversion, same clash timing, same redaction.
- The finished task is always the last task of an operation. Cancelling stops new engine work and still saves state.
- No call blocks the event loop or holds the GIL while the engine works.
- A forked child after use, or a mismatched engine, fails with a clear, named error.

**Deliberate gaps**
- No tests read the README, CHANGELOG, parity list or workflow files (convention). These are reviewed by a person.
- No Python-provider, protocol v2 or codegen tests, because none of these is built.
- The Docker plugin tests run on Linux x86-64 only, because a Docker engine isn't available on the other runners. Parity on every platform uses the self-contained Go fixture plugins instead.
- No fuzzing of the C ABI. Request parsing is protobuf's, and negative statuses are tested one by one.

**Success metrics**
- *A Python developer goes from `pip install xcl` to a working apply against a Go plugin by following the package README alone.* **Manual — captured in the implementation test plan.**
- *Every operation and lookup in the Go API has a Python equivalent in both the synchronous and asyncio APIs, checked against a parity list at release.*
  - A **behavioural test** asserts that `xcl.Config` and `xcl.aio.Config` expose the same public operations and lookups, with each async one awaitable.
  - Checking the parity list against the Go API at release is **Manual — captured in the implementation test plan.**
- *The Go–Python parity fixtures pass on every release, on every published platform.* Covered by a **behavioural test**: the parity suite runs in the eight-platform CI matrix, and the release workflow refuses to publish unless it passes for the tag.
- *An async web service using the asyncio API keeps serving other requests during a long apply.* Covered by a **behavioural test**:
  - an asyncio HTTP server on the same event loop answers a series of requests within a short bound;
  - meanwhile an `aio.Config.apply` runs against the slow fixture plugin.

**Manual reviews**
- **Manual — captured in the implementation test plan:** review the xcl CHANGELOG entry for this spec. It should describe the context-taking operations, untyped lookups, `TypeFromSchema`, coreservice and libxcl, with **Breaking:** None.
- **Manual — captured in the implementation test plan:** review the PyPI project page of the first release. It should show the README with install, synchronous getting started and the asyncio section.
- **Manual — captured in the implementation test plan:** review `PARITY.md` against the Go API of the released xcl tag.
- **Manual — captured in the implementation test plan:** watch the first tagged release end to end. It should attach eight signed libxcl archives to the xcl release, publish eight wheels and an sdist to PyPI at the same version, and pass the install checks on every platform.

## Milestones & Tasks

### Milestone 1: The xcl engine can be driven from another language

**What changes**
- Go users gain context-taking forms of every operation, so a cancelled context stops an apply and still saves progress.
- They also gain untyped lookups by type, and a way to declare a block type from a language-neutral schema, including named-block maps.
- On top of these, xcl gains the core service: one message-based API covering creating a configuration, every operation, events, host state stores, host functions, cancellation and lookups. It is tested entirely in Go.
- xcl also gains a C shared library, built in CI, that exposes that API through four C functions.
- The library for Go users stays free of cgo.

**Validation point**: The root-package, schema and coreservice test suites pass with the race detector. The `CGO_ENABLED=0` cross builds of the root module, now including coreservice, still pass. CI builds `libxcl` with cgo and its smoke test calls through the C ABI. `buf lint` passes on the new protos.

#### - [x] Task: Operations take a context and stop when it is cancelled
**Id:** cb40a3bf-11de-40c8-a007-f76ad82b3cec
**Repo:** xcl
**Depends on:** none
**Execution:** agent

The five operations gain context-taking forms, so a caller can stop an operation. Validate, Apply, Diff, Destroy and Load each get a form that takes a context, and the existing methods become wrappers that pass a background context. Cancelling the context stops new provider calls and still saves the progress made, as already happens when an event handler panics. coreservice needs this to make cancellation real, and Go users get it too.

*Technical detail:* [context.md#task-operations-take-a-context-and-stop-when-it-is-cancelled](./context.md#task-operations-take-a-context-and-stop-when-it-is-cancelled)

**Acceptance criteria**:
- [x] Cancelling the context during an apply makes it return an error that matches `context.Canceled`.
- [x] After a cancelled apply, no provider call starts, and the entities already applied are in saved state.
- [x] Every existing operation behaves exactly as before when called without a context.
- [x] A context that is already cancelled stops an operation before any provider call.

#### - [x] Task: Entities can be looked up by type without a Go type
**Id:** a4e7390e-ef84-4001-a968-feff2d778554
**Repo:** xcl
**Depends on:** none
**Execution:** agent

`FindByType` and `FindOne` need a Go type parameter, which a host language does not have. This task adds untyped forms on `Config` that return the entities as stored. They use the same rules as the generic forms: type and subtype matching, unknown types, types that need a subtype, and more than one match. A Python program therefore gets exactly the errors a Go program gets.

*Technical detail:* [context.md#task-entities-can-be-looked-up-by-type-without-a-go-type](./context.md#task-entities-can-be-looked-up-by-type-without-a-go-type)

**Acceptance criteria**:
- [x] Looking up `database`, `postgres` returns every entity of that type and subtype, matching `FindByType` for the registered Go type.
- [x] An unknown type, and a type that needs a subtype but was given none, return the same errors as `FindByType`.
- [x] Looking up exactly one entity returns it. Finding none returns a not-found error, and finding several returns a not-unique error with the count.
- [x] Plugin-provided entities are returned by the untyped lookups.

#### - [x] Task: Block types can be declared from a language-neutral schema
**Id:** d2d86531-20de-4554-aaa1-d36112ed247c
**Repo:** xcl
**Depends on:** none
**Execution:** agent

This adds schema v1 from the libxcl design, a JSON description of a block type that any language can write, and a reader that builds the Go type the parser decodes into. It covers:
- attributes and nested blocks;
- lists of blocks and maps of named blocks with their optional name label;
- optional fields, defaults, sensitive fields and computed fields.

The registry gains a public way to declare a type from such a schema. The existing plugin schema keeps working unchanged.

*Technical detail:* [context.md#task-block-types-can-be-declared-from-a-language-neutral-schema](./context.md#task-block-types-can-be-declared-from-a-language-neutral-schema)

**Acceptance criteria**:
- [x] A type declared from a schema decodes `app "web" { image = "nginx" }` and is found at `app.web` with `image` set.
- [x] Optional fields take their default, a required field missing is a validation error at the block, and a computed field cannot be set in configuration.
- [x] A sensitive field is masked in events and state.
- [x] Two `volume` blocks become a list of two in order, and two `listener "…"` blocks become a map keyed by name.
- [x] A schema marking an unsupported shape as sensitive, or an unknown field type, is refused with an error naming the field.
- [x] Plugins sending the existing schema form still load and decode as before.

#### - [x] Task: The core service answers configuration and lookup calls
**Id:** 845a37c8-a730-4709-9a1b-8ee28bd52ded
**Repo:** xcl
**Depends on:**
- a4e7390e-ef84-4001-a968-feff2d778554 — Entities can be looked up by type without a Go type
- d2d86531-20de-4554-aaa1-d36112ed247c — Block types can be declared from a language-neutral schema
**Execution:** agent

This creates the `coreservice` package and its protobuf API, generated with `buf`. It covers:
- handles for configurations;
- a dispatcher that turns a method name and request bytes into a response and status;
- `GetVersion`;
- `CreateConfig`, mapping every configuration option and both kinds of registry;
- the lookups: entities, find, find by type, find one, outputs and saved-entity encoding.

Every root-module error maps to a stable code with its details. This is the foundation the operations and the shared library build on.

*Technical detail:* [context.md#task-the-core-service-answers-configuration-and-lookup-calls](./context.md#task-the-core-service-answers-configuration-and-lookup-calls)

**Acceptance criteria**:
- [x] Every method in the service definition has a handler, and an unknown method, unparsable bytes, an unknown handle or a wrong use of handle 0 each give their own status.
- [x] Creating a configuration with a schema-declared type, external plugins, a GitHub registry, state, masks, events and variables gives the same behaviour as the equivalent Go options.
- [x] Lookups return entities as JSON with sensitive values shown as the marker, and outputs as a JSON object.
- [x] Each root-module error comes back as its documented code with its details. A configuration error lists every problem with its file and line.
- [x] `GetVersion` reports the version the library was built with.
- [x] The root module still builds with cgo disabled, including this package.

#### - [x] Task: Operations run through the poll loop and can be cancelled
**Id:** b5492df1-0bd4-488f-9c98-9104399dab53
**Repo:** xcl
**Depends on:**
- cb40a3bf-11de-40c8-a007-f76ad82b3cec — Operations take a context and stop when it is cancelled
- 845a37c8-a730-4709-9a1b-8ee28bd52ded — The core service answers configuration and lookup calls
**Execution:** agent

This adds the operation side of the core service:
- starting validate, apply, diff, destroy and load;
- the per-configuration task queue, polled and completed by the host;
- events delivered as tasks, with the operation's end always last;
- a state store the host implements, called through tasks;
- cancellation of a running operation.

While an operation runs, lookups are refused, so a host never reads half-applied entities.

*Technical detail:* [context.md#task-operations-run-through-the-poll-loop-and-can-be-cancelled](./context.md#task-operations-run-through-the-poll-loop-and-can-be-cancelled)

**Acceptance criteria**:
- [x] An apply started through the service ends with a finished task. Its entities and outputs then match a Go apply of the same configuration.
- [x] With events on, every event of the operation is delivered before the finished task, in order, with sensitive data masked as configured.
- [x] A host state store receives the state on apply and serves it back on load. With a state mask, sensitive values it receives are encrypted.
- [x] A diff's finished task carries the diff, which matches the Go diff for the same configuration.
- [x] Cancelling a running apply ends it with the cancelled code, saves progress, and starts no provider call afterwards.
- [x] A second operation, or a lookup, on a handle with an operation running is refused with `operation_in_progress`.

#### - [x] Task: Host-language functions are callable from configuration
**Id:** c52e2bcb-98fd-48e9-8b14-10f749acc548
**Repo:** xcl
**Depends on:**
- b5492df1-0bd4-488f-9c98-9104399dab53 — Operations run through the poll loop and can be cancelled
**Execution:** agent

A host declares functions by name and signature when it creates a configuration. The core turns each one into an application function, so it is converted, clash-checked and redacted exactly like a Go application function. When configuration calls it, the call is handed to the host as a function task, and the host's result or error is filled in. Function calls wait under the operation's context, so cancelling the operation releases them.

*Technical detail:* [context.md#task-host-language-functions-are-callable-from-configuration](./context.md#task-host-language-functions-are-callable-from-configuration)

**Acceptance criteria**:
- [x] With `slugify` declared by the host, `name = slugify("My App")` applies, and the entity holds what the host returned.
- [x] A host function that returns an error fails the apply with that message at the call's position.
- [x] Declaring a function named `upper`, or two functions with one name, fails configuration creation with the function name clash code naming both sources.
- [x] A signature using an unsupported type fails configuration creation with the invalid function code.
- [x] A sensitive argument reaches the host unmarked, and the result is marked sensitive in the entity.
- [x] Cancelling an operation while a function task is outstanding ends the operation promptly.

#### - [x] Task: The engine is built as a C shared library
**Id:** ea672f25-b833-4738-81d5-1c14aa2811a7
**Repo:** xcl
**Depends on:**
- 845a37c8-a730-4709-9a1b-8ee28bd52ded — The core service answers configuration and lookup calls
**Execution:** agent

A new nested Go module, `bindings/libxcl`, exports the four C functions from the design over the core service. It is built with cgo as a shared library whose version is stamped at build time. CI builds it on Linux and runs a smoke test through the C ABI. The root module's existing cgo-free builds stay as they are, so Go users are unaffected.

*Technical detail:* [context.md#task-the-engine-is-built-as-a-c-shared-library](./context.md#task-the-engine-is-built-as-a-c-shared-library)

**Acceptance criteria**:
- [x] Building the library produces a shared object exporting exactly the four design functions.
- [x] A smoke test loads the library, reads the ABI version, gets the stamped version, creates and validates a configuration, and releases it, freeing every response.
- [x] CI builds and smoke-tests the library on every change, and the root module's cgo-free builds still pass.
- [x] A make target builds the library for the local platform with a chosen version.

### Milestone 2: A Python program can configure, apply and query with XCL

**What changes**
- A new `xcl` Python package, in the new `jumppad-labs/xcl-python` repository, loads the engine and offers the synchronous API. Python developers can:
  - define entity types as dataclasses;
  - register Python functions;
  - use Go plugins from a path, a directory or the GitHub registry;
  - pass variables;
  - keep state in a directory or in their own store, with encryption;
  - receive events;
  - validate, diff, apply, load and destroy;
  - read entities, outputs and diffs back as Python values.
- Failures are typed exceptions.
- The package refuses a mismatched engine and refuses to run after an unsafe fork.
- Python and Go give the same results on a shared set of fixtures.

At this stage the package is installed from source with a locally built engine.

**Validation point**: The xcl-python unit and integration suites pass against a locally built `libxcl` on Linux, with the Docker plugin tests running against a real engine. The Go–Python parity suite passes.

#### - [x] Task: Go fixture plugins and the Go side of the parity check
**Id:** 856330ba-2549-4926-bed4-cda65274ee2a
**Repo:** xcl
**Depends on:** none
**Execution:** agent

The Python tests need small Go plugins that run anywhere:
- a slow plugin, so the tests can show other threads and coroutines keep running;
- a recording plugin, which writes down every operation it starts, so cancellation can be checked.

The parity check needs a shared set of fixture configurations and a Go program that runs each one through the Go API. That program records validation errors, the diff, entities, outputs, state, load and destroy results as normalised JSON. This task adds all of these beside the shared library, so every SDK can use them.

*Technical detail:* [context.md#task-go-fixture-plugins-and-the-go-side-of-the-parity-check](./context.md#task-go-fixture-plugins-and-the-go-side-of-the-parity-check)

**Acceptance criteria**:
- [x] The slow plugin's create takes a configurable time, and the recording plugin appends every lifecycle call it starts to a file named in its configuration.
- [x] The parity program, given a fixture directory, writes one JSON record covering validation, diff, apply, outputs, state, load in a new process, and destroy.
- [x] Records contain no temp paths or timestamps, so two runs of the same fixture are identical.
- [x] The fixtures cover registered types with every annotation kind, an application function, variables, outputs, a sensitive value, a plugin entity and an invalid configuration.

#### - [x] Task: Create the GitHub repository jumppad-labs/xcl-python and connect the registered local directory to it
**Id:** e67db340-e5f6-425c-a374-2da571323cd8
**Repo:** xcl-python
**Depends on:** none
**Execution:** human — creating a GitHub repository under the jumppad-labs organisation is an action outside any repo

The Python package lives in its own repository. It is already registered with this Spektacular project, at a local directory next to the other xcl repositories, but it does not exist on GitHub yet. A person creates `jumppad-labs/xcl-python` on GitHub and turns the registered local directory into a clone of it, so the Python tasks can be carried out and pushed there.

*Technical detail:* [context.md#task-create-the-github-repository-jumppad-labsxcl-python-and-connect-the-registered-local-directory-to-it](./context.md#task-create-the-github-repository-jumppad-labsxcl-python-and-connect-the-registered-local-directory-to-it)

**Acceptance criteria**:
- [x] `jumppad-labs/xcl-python` exists on GitHub with an Apache-2.0 licence, matching xcl's.
- [x] The registered local directory is a git repository whose `origin` is that GitHub repository, and it keeps its Spektacular footprint.

#### - [x] Task: The Python package loads the engine safely
**Id:** 0a526314-639b-4aba-9c65-f67ea9343e6c
**Repo:** xcl-python
**Depends on:**
- e67db340-e5f6-425c-a374-2da571323cd8 — Create the GitHub repository jumppad-labs/xcl-python and connect the registered local directory to it
- ea672f25-b833-4738-81d5-1c14aa2811a7 — The engine is built as a C shared library
**Execution:** agent

This sets up the `xcl` package:
- project metadata for CPython 3.10 and later;
- message classes generated from xcl's protos;
- a development build that compiles the engine from a local xcl checkout into the package.

It adds the bottom two layers of the SDK. The transport loads the library lazily with cffi, falling back to ctypes, releases the GIL during calls and frees every response. The typed client turns statuses into exceptions. It also adds the exception hierarchy, the ABI and engine version checks, and the fork guard.

*Technical detail:* [context.md#task-the-python-package-loads-the-engine-safely](./context.md#task-the-python-package-loads-the-engine-safely)

**Acceptance criteria**:
- [x] `import xcl` does not load the engine; the first call does.
- [x] An engine whose version differs from the package's raises an error naming both versions, and so does a different ABI version.
- [x] In a child forked after the parent used XCL, any call raises the fork error. The error names the `spawn` start method and creating the configuration after the fork.
- [x] A child forked from a parent that only imported XCL works normally.
- [x] Each error code raises its own exception class, carrying the message, details and causes. An unknown code raises the base `xcl.Error`.
- [x] Unit tests run against a fake transport with no engine.

#### - [x] Task: Python classes describe entity types
**Id:** 62cb1d61-f657-4571-8f13-4abc97c68eaa
**Repo:** xcl-python
**Depends on:**
- 0a526314-639b-4aba-9c65-f67ea9343e6c — The Python package loads the engine safely
**Execution:** agent

A developer marks a dataclass with `@xcl.entity(type, subtype)` and describes its fields with ordinary annotations:
- optional fields and defaults;
- `xcl.Sensitive[T]` and `xcl.Computed[T]`;
- nested blocks, lists of blocks and dicts of named blocks;
- `xcl.field` for overrides.

This task derives the schema v1 document from those annotations and turns entity JSON back into instances of the registered class, or into a generic `xcl.Entity` for types the program has not registered. It also derives a function signature from a Python function's annotations.

*Technical detail:* [context.md#task-python-classes-describe-entity-types](./context.md#task-python-classes-describe-entity-types)

**Acceptance criteria**:
- [x] Each annotation form produces the schema field the design and the named-block extension describe.
- [x] An annotation with no configuration form, or a sensitive type the engine cannot hold, is refused at registration with an error naming the field.
- [x] Entity JSON hydrates into the registered class, nested blocks included. A dict of named blocks becomes a dict keyed by name.
- [x] An entity of an unregistered type becomes an `xcl.Entity` whose address and attributes are readable by name.
- [x] `Sensitive` values print as `(sensitive)` and reveal their value only through `reveal()`. A looked-up sensitive value reveals `None`.
- [x] A function annotated `(str) -> str`, or with lists, dicts, bools and numbers, produces the signature JSON the engine expects. An unsupported annotation is refused.

#### - [x] Task: The synchronous API runs every operation and lookup
**Id:** bb272fd2-e209-4895-9941-dd58e7b5a03c
**Repo:** xcl-python
**Depends on:**
- 62cb1d61-f657-4571-8f13-4abc97c68eaa — Python classes describe entity types
- b5492df1-0bd4-488f-9c98-9104399dab53 — Operations run through the poll loop and can be cancelled
**Execution:** agent

This adds `xcl.Config`, `xcl.LocalRegistry` and `xcl.GitHubRegistry`, and the shared poll engine that both the sync and async APIs will drive. `xcl.Config` has every option: state directory or custom store, state mask, variables, event handler, event data and buffer, and event mask. A program can:
- validate, diff, apply, load and destroy;
- read entities, find by address, by type, exactly one and all of a type;
- read outputs and encode saved entities.

The poll engine runs on the calling thread with the GIL released while it waits. Ctrl-C cancels the engine's work.

*Technical detail:* [context.md#task-the-synchronous-api-runs-every-operation-and-lookup](./context.md#task-the-synchronous-api-runs-every-operation-and-lookup)

**Acceptance criteria**:
- [x] With a class registered as `app`, `app "web" { image = "nginx" }` validates and applies, and `find(App, "app.web")` returns an `App` with `image == "nginx"`. A `database`/`postgres` class is found at `database.postgres.main`.
- [x] Each annotation rule from the spec holds: required, optional default, sensitive masked, computed not settable, nested block, list of two volumes in order, dict of named listeners.
- [x] Passing `region = "us"` as a variable makes an entity using `variable.region` hold `"us"`.
- [x] After apply, outputs come back as a dict. A diff after changing one attribute reports its old and new values.
- [x] With a state directory, an apply writes state there and a load in a new process reads it back. A custom store object receives the state on apply and serves a later load in a new process, with sensitive values encrypted when a state mask is set.
- [x] An event handler receives a start and finish event for each entity, with sensitive event data redacted.
- [x] A missing address raises the not-found exception, several entities for `find_one` raise the not-unique exception, and invalid configuration raises the config exception listing each problem's file and line.
- [x] While a synchronous apply runs a slow plugin operation, a second Python thread keeps making progress.

#### - [x] Task: Python functions are callable from configuration
**Id:** dec2616c-3368-48ce-bcd8-f866594eae04
**Repo:** xcl-python
**Depends on:**
- bb272fd2-e209-4895-9941-dd58e7b5a03c — The synchronous API runs every operation and lookup
- c52e2bcb-98fd-48e9-8b14-10f749acc548 — Host-language functions are callable from configuration
**Execution:** agent

A developer registers a Python function on a local registry, and configuration can call it by name. The registry sends the function's derived signature when the configuration is created. The poll engine runs each function task with the decoded arguments and returns the result, or the exception's message as an error. Name clashes and unusable signatures surface as their own exceptions when the configuration is created, as they do in Go.

*Technical detail:* [context.md#task-python-functions-are-callable-from-configuration](./context.md#task-python-functions-are-callable-from-configuration)

**Acceptance criteria**:
- [x] With `slugify` registered, `name = slugify("My App")` applies and the entity holds `"my-app"`.
- [x] A function that raises fails the apply with a config exception naming the call's file and line and the raised message.
- [x] Registering a function named like a built-in, or two with one name, raises the function name clash exception when the configuration is created.
- [x] A sensitive argument reaches the function as a plain value, and the result is held as sensitive.

#### - [x] Task: Go plugins are usable from Python
**Id:** 3a49f297-bb1a-4703-8a05-f412ff8c8657
**Repo:** xcl-python
**Depends on:**
- bb272fd2-e209-4895-9941-dd58e7b5a03c — The synchronous API runs every operation and lookup
**Execution:** agent

A Python program uses the Docker example plugin, loaded three ways: from a binary path, from a plugin directory and from the GitHub registry with trusted keys. This task proves each path end to end. It also adds a test that serves a tampered release from a local fake GitHub API and expects the install to be refused. Unregistered plugin types are read through the generic entity.

*Technical detail:* [context.md#task-go-plugins-are-usable-from-python](./context.md#task-go-plugins-are-usable-from-python)

**Acceptance criteria**:
- [x] Configuration whose entities are handled by the Docker plugin applies when the plugin is loaded from a path, from a plugin directory and from the GitHub registry.
- [x] A release whose checksums file was tampered with is refused with the plugin verification exception.
- [x] Docker plugin entities, with no Python class registered, are returned with their address and attributes readable by name.
- [x] The tests skip with a clear reason when no Docker engine is reachable.

#### - [x] Task: Python gives the same results as Go on the parity fixtures
**Id:** 19083e6a-248a-41de-bf95-f2fa311af6a5
**Repo:** xcl-python
**Depends on:**
- 856330ba-2549-4926-bed4-cda65274ee2a — Go fixture plugins and the Go side of the parity check
- dec2616c-3368-48ce-bcd8-f866594eae04 — Python functions are callable from configuration
**Execution:** agent

The Python side of the parity check runs each shared fixture through the Python API and writes the same normalised record the Go parity program writes. It then compares the two. The fixtures and Go programs come from the xcl checkout the package is built against, so a mismatch shows the binding has changed behaviour.

*Technical detail:* [context.md#task-python-gives-the-same-results-as-go-on-the-parity-fixtures](./context.md#task-python-gives-the-same-results-as-go-on-the-parity-fixtures)

**Acceptance criteria**:
- [x] For every fixture, Python and Go give the same validation errors, the same diff, the same entities and outputs after apply, and the same state contents.
- [x] A load in a new process reads back the same entities in both, and a destroy removes every entity and empties the state in both.
- [x] A difference fails the test with the fixture name and the first differing field.

### Milestone 3: Asynchronous Python services can use XCL without blocking

**What changes**
- `xcl.aio.Config` offers every operation and lookup as an awaitable that never blocks the event loop.
- Python functions, event handlers and custom state stores may be `async def`.
- Cancelling an awaited operation stops the engine's work promptly rather than leaving it running.
- A web service can keep answering requests while a long apply runs.

**Validation point**: The asyncio integration tests pass:
- an async function is awaited;
- other coroutines and an HTTP server keep running during a slow apply;
- cancellation ends the apply promptly, and the recording plugin shows no operation started after it;
- sync and async expose the same operations and lookups.

#### - [x] Task: The asyncio API never blocks and can be cancelled
**Id:** 0b6f8555-7035-4bf4-9890-4592a0d105d8
**Repo:** xcl-python
**Depends on:**
- dec2616c-3368-48ce-bcd8-f866594eae04 — Python functions are callable from configuration
**Execution:** agent

`xcl.aio.Config` offers every operation and lookup of the synchronous API as awaitables. It drives the same poll engine from a dedicated executor thread. Function, event handler and state store tasks run as asyncio tasks when they are `async def`, and in the default executor otherwise. Cancelling the awaiting task sends a cancel to the engine, waits for it to stop and save, then lets the cancellation through.

*Technical detail:* [context.md#task-the-asyncio-api-never-blocks-and-can-be-cancelled](./context.md#task-the-asyncio-api-never-blocks-and-can-be-cancelled)

**Acceptance criteria**:
- [x] A registered `async def` function is awaited and its result filled in.
- [x] An async event handler and an async custom state store work through apply and load.
- [x] While an asyncio apply runs a slow plugin operation, other coroutines on the same event loop keep running.
- [x] An asyncio HTTP server on the same loop keeps answering requests within a short bound during a long apply.
- [x] Cancelling an asyncio apply mid-way ends it promptly with a cancellation error, and the recording plugin shows no operation started after the cancel.
- [x] `xcl.Config` and `xcl.aio.Config` expose the same operations and lookups, with every async one awaitable.

### Milestone 4: Every XCL release ships a matching Python package on PyPI

**What changes**
- Tagging an XCL release now builds the engine for eight platforms and attaches signed archives to the GitHub release.
- It then publishes the `xcl` Python package at the same version to PyPI, with one wheel per platform. Users run `pip install xcl` with no Go or C compiler, on CPython 3.10 and later.
- The package ships a README that leads with synchronous getting started against the Docker plugin, followed by asyncio, plus docstrings on every public name and a parity list against the Go API.
- The xcl changelog records the change.

**Validation point**:
- The xcl-python CI matrix passes on all eight platforms and CPython 3.10–3.13 plus the latest stable release.
- A person tags the first release. The workflow attaches eight signed libxcl archives and publishes eight wheels and an sdist at the same version.
- The post-publish install check passes on every platform.
- The manual reviews in the Testing Approach are carried into the implementation test plan.

#### - [x] Task: Tagging xcl publishes signed engine builds for eight platforms
**Id:** 7f6c4f43-6836-4b73-8a38-ec2caf9de9a6
**Repo:** xcl
**Depends on:**
- ea672f25-b833-4738-81d5-1c14aa2811a7 — The engine is built as a C shared library
**Execution:** agent

xcl gets its first release workflow. When a version tag is pushed, it does the following:
1. Builds the shared library for Linux x86-64 and arm64 (glibc and musl), macOS x86-64 and arm64, and Windows x86-64 and arm64, each stamped with the tag.
2. Packages each build in the same archive, checksum and signature layout the plugin registry already verifies.
3. Attaches the archives to the GitHub release.
4. Triggers the xcl-python release for the same tag.

*Technical detail:* [context.md#task-tagging-xcl-publishes-signed-engine-builds-for-eight-platforms](./context.md#task-tagging-xcl-publishes-signed-engine-builds-for-eight-platforms)

**Acceptance criteria**:
- [ ] A tag push produces eight `libxcl_<version>_<os>_<arch>` archives, a checksums file and its detached signature on the xcl GitHub release.
- [ ] Each library reports the tag as its version, and the Linux glibc builds need no newer glibc than 2.28.
- [ ] The workflow triggers the xcl-python release with the tag once the assets are attached.
- [ ] A manual run of the workflow without publishing builds all eight targets, so the matrix can be checked before a real release.

#### - [x] Task: Add the release signing key and dispatch token to xcl
**Id:** 15487d49-c3a6-46a5-aeed-4e4eaaecd57d
**Repo:** xcl
**Depends on:**
- 7f6c4f43-6836-4b73-8a38-ec2caf9de9a6 — Tagging xcl publishes signed engine builds for eight platforms
**Execution:** human — needs the jumppad-labs release signing key and permission to create repository secrets and tokens

The release workflow signs the checksums with the jumppad-labs release key and triggers xcl-python's release. A person adds the signing key, its passphrase and a token allowed to dispatch workflows in xcl-python as secrets on the xcl repository.

*Technical detail:* [context.md#task-add-the-release-signing-key-and-dispatch-token-to-xcl](./context.md#task-add-the-release-signing-key-and-dispatch-token-to-xcl)

**Acceptance criteria**:
- [x] The xcl repository has the signing key, passphrase and dispatch token secrets the workflow names.
- [ ] A manual dry run of the release workflow signs the checksums file with the key the plugin registry trusts.

#### - [x] Task: xcl-python is tested on every platform and released from the xcl tag
**Id:** 53b3da1c-4b2c-48c4-97ed-3a1cd96312bb
**Repo:** xcl-python
**Depends on:**
- 0b6f8555-7035-4bf4-9890-4592a0d105d8 — The asyncio API never blocks and can be cancelled
- 3a49f297-bb1a-4703-8a05-f412ff8c8657 — Go plugins are usable from Python
- 19083e6a-248a-41de-bf95-f2fa311af6a5 — Python gives the same results as Go on the parity fixtures
**Execution:** agent

**CI.** Every change is tested on the eight platforms against CPython 3.10, 3.11, 3.12, 3.13 and the latest stable release, with the engine built from the matching xcl checkout.

**Release.** The workflow is triggered by the xcl release or run by hand for a tag. It:
1. checks out xcl at the tag and regenerates the message classes;
2. downloads and verifies the signed engine archives;
3. builds one wheel per platform plus an sdist at the tag's version;
4. runs the parity suite against them and publishes to PyPI with Trusted Publishing;
5. installs from PyPI on every platform to check the published package.

*Technical detail:* [context.md#task-xcl-python-is-tested-on-every-platform-and-released-from-the-xcl-tag](./context.md#task-xcl-python-is-tested-on-every-platform-and-released-from-the-xcl-tag)

**Acceptance criteria**:
- [ ] CI runs the full suite on all eight platforms for each supported CPython version.
- [ ] The release builds eight wheels tagged `py3-none-<platform>`, each containing the matching engine, and an sdist, all at the tag's version without the `v`.
- [ ] The release refuses to publish when the archive signature does not verify or the parity suite fails.
- [ ] After publishing, `pip install xcl==<version>` and `python -c "import xcl; xcl.Config().close()"` succeed on each platform with no Go or C compiler, and the engine reports the tag.

#### - [x] Task: Set up the PyPI project and trusted publishing
**Id:** fe0be452-82c1-43e5-8ca6-99af0f2bace0
**Repo:** xcl-python
**Depends on:**
- 53b3da1c-4b2c-48c4-97ed-3a1cd96312bb — xcl-python is tested on every platform and released from the xcl tag
**Execution:** human — needs the jumppad-labs PyPI account and GitHub environment settings

Publishing uses PyPI Trusted Publishing, so no API token is stored. A person makes sure the `xcl` project on PyPI belongs to jumppad-labs, adds xcl-python's release workflow as its trusted publisher, and creates the protected `pypi` environment in xcl-python that the workflow publishes from.

*Technical detail:* [context.md#task-set-up-the-pypi-project-and-trusted-publishing](./context.md#task-set-up-the-pypi-project-and-trusted-publishing)

**Acceptance criteria**:
- [x] The `xcl` project on PyPI is owned by jumppad-labs and lists xcl-python's release workflow and `pypi` environment as a trusted publisher.
- [x] The xcl-python repository has a `pypi` environment that the release job uses.

#### - [x] Task: The Python package is documented
**Id:** b4542c17-081c-40b2-8d65-9624f545f22e
**Repo:** xcl-python
**Depends on:**
- 0b6f8555-7035-4bf4-9890-4592a0d105d8 — The asyncio API never blocks and can be cancelled
- 3a49f297-bb1a-4703-8a05-f412ff8c8657 — Go plugins are usable from Python
**Execution:** agent

This writes the package README that PyPI shows. It covers:
- installing with pip;
- a synchronous getting started that defines an entity class, registers the Docker plugin and applies;
- an asyncio section.

It also gives every public class and function a docstring, and adds a parity list mapping each Go operation and lookup to its synchronous and asynchronous equivalents. A test checks that every public name has a docstring.

*Technical detail:* [context.md#task-the-python-package-is-documented](./context.md#task-the-python-package-is-documented)

**Acceptance criteria**:
- [x] The README covers install, synchronous getting started against the Docker plugin, and an asynchronous section, in that order, and the package metadata uses it as the PyPI description.
- [x] Every public name in the package has a docstring, which a test checks by importing the package.
- [x] The parity list names every Go operation and lookup with its sync and async equivalent, or why it does not apply.
- [x] The README states the supported platforms and Python versions, and the fork limitation with its two ways out.

#### - [x] Task: Record the Python support in the xcl changelog
**Id:** 494e66eb-3adf-4dc0-a90b-57b4145c42ec
**Repo:** xcl
**Depends on:**
- c52e2bcb-98fd-48e9-8b14-10f749acc548 — Host-language functions are callable from configuration
- ea672f25-b833-4738-81d5-1c14aa2811a7 — The engine is built as a C shared library
- 7f6c4f43-6836-4b73-8a38-ec2caf9de9a6 — Tagging xcl publishes signed engine builds for eight platforms
**Execution:** agent

xcl's changelog gets one entry for this spec. It describes:
- the context-taking operations;
- the untyped lookups;
- declaring types from schema v1;
- the core service, the shared library and its release assets;
- the Python package.

The xcl README gains a short pointer to the Python package.

*Technical detail:* [context.md#task-record-the-python-support-in-the-xcl-changelog](./context.md#task-record-the-python-support-in-the-xcl-changelog)

**Acceptance criteria**:
- [x] The changelog has a `## 20261011130709-aa3eb933-python-support` entry, in prose, followed by **Breaking:** None.
- [x] The xcl README says the engine is available to Python as `pip install xcl` and links the xcl-python repository.

#### - [x] Task: Cut the first xcl release with Python support
**Id:** 75d985e2-10fa-4cdc-896f-afe9d9d2df79
**Repo:** xcl
**Depends on:**
- 15487d49-c3a6-46a5-aeed-4e4eaaecd57d — Add the release signing key and dispatch token to xcl
- fe0be452-82c1-43e5-8ca6-99af0f2bace0 — Set up the PyPI project and trusted publishing
- b4542c17-081c-40b2-8d65-9624f545f22e — The Python package is documented
- 494e66eb-3adf-4dc0-a90b-57b4145c42ec — Record the Python support in the xcl changelog
**Execution:** human — publishing a release tag of xcl is an action outside the repo's code

A person merges the xcl and xcl-python work and pushes the next xcl version tag after v0.3.0. This starts the release chain that publishes the engine archives and the matching Python package.

*Technical detail:* [context.md#task-cut-the-first-xcl-release-with-python-support](./context.md#task-cut-the-first-xcl-release-with-python-support)

**Acceptance criteria**:
- [ ] A tagged xcl release carries the eight signed engine archives.
- [ ] The same version of `xcl` is on PyPI with eight wheels and an sdist, and its install checks passed on every platform.

## Open Questions

- **Does language-surface's Go-func conversion accept functions made with `reflect.MakeFunc`, including variadic ones?** This depends on how the landed `CreateCtyFunctionFromGoFunc`/`goFuncSignature` inspects the func: by its `reflect.Type`, or by anything else. It only shows once that code exists.
  - If made funcs are accepted but variadic ones are not, refuse variadic Python functions with `invalid_function` and note it in the README.
  - If made funcs are refused at all, STOP and ask the user, because Python functions would then need a change to the root module's function path.
- **Does a cancelled `Apply` already return an error wrapping `context.Canceled`, or only stop starting calls?** This depends on how the parser's DAG walk reports a cancelled context, which the context-operations task will see when it adds the first cancellation test. The task's technical notes give the fallback: wrap `ctx.Err()` in `ApplyContext`. Stop and ask only if cancelling leaves state unsaved.
- **Is the PyPI project name `xcl` free, or already owned by jumppad-labs?** This can only be settled by the person setting up PyPI. If someone else owns the name, that task STOPs and asks the user, because the spec fixes `pip install xcl`.

No other questions remain open. Every design choice is settled in the architecture and recorded in the assumption log.

## Out of Scope

- No Python providers registered in process. These are a spec non-goal and an interview decision; Python programs use Go-compiled plugins and config-only types. None of the design's provider pieces is built: `ProviderTask`/`ProviderResult`, `ForeignProviderAdapter`, `DefaultChangedJSON`, the `Log`/`StateGet`/`StateFind` methods, or `DeclaredType.provided`/`implements`. Their field numbers are reserved for the later Python-providers spec in this epic.
- No out-of-process Python plugins, Python plugin SDK (`xcl.Plugin.serve()`) or plugin protocol v2 (`VersionedPlugins`, `XCL_HOST_ADDRESS`, schema v1 under `GetTypes`). These are a spec non-goal and belong to a later spec.
- No distribution of Python plugins through the GitHub registry (spec non-goal; a design open question).
- No Node or TypeScript binding (spec non-goal; design phase 5).
- No Python documentation on xcl.dev. The epic's docs specs write it once this API settles. This plan ships the package README, docstrings and `PARITY.md` only.
- No `DescribeTypes` method and no `python -m xcl generate` code generation for plugin types. Plugin entities are read through the generic `xcl.Entity`. This is a later spec, when typed plugin classes are wanted.
- The shared `xcl.types.v1` proto is not created. The change-list messages stay in `plugins/plugin.proto` until provider tasks need them.
- The plugin schema producer is not switched to schema v1. Plugins keep sending the legacy schema under protocol 1. The v1 reader accepts both forms.
- No out-of-process `xcl-core` transport and no Wasm build (design: deferred).
- No `max_workers` thread pool for the synchronous API, and no Python `decode`/`EncodeEntity` equivalents (recorded in `PARITY.md` as not applicable).
- No support for platforms beyond the eight targets, or for running another Go-built extension in the same process (design hazards: documented as unsupported).
- No change to xcl's existing Go API beyond the additive context operations, untyped lookups and `registry.TypeFromSchema`.

## Changelog

### 2026-10-11 — Task: Operations take a context and stop when it is cancelled

**What was done**: Added `ValidateContext`, `ApplyContext`, `DiffContext`, `DestroyContext` and `LoadContext` to `Config`; the existing five operations are now one-line wrappers that pass `context.Background()`. `run` takes the caller's context and, with an event handler, derives its cancellable child from it instead of from `context.Background()`, so cancelling the caller's context stops new provider calls through the parser's existing cancellation path and the progress made is saved as before. `ValidateContext` and `LoadContext` check `ctx.Err()` before reading, because neither path reaches a provider.

**Deviations**: None of substance. The plan's fallback of wrapping `ctx.Err()` in `ApplyContext` was not needed: the parser already appends an error wrapping `ctx.Err()` to its `ConfigError`, whose `Unwrap() []error` makes `errors.Is(err, context.Canceled)` hold. The destroy cancellation test uses a pre-cancelled context rather than cancelling from an event handler, because at `Config` level events are delivered on a separate goroutine and a handler-driven cancel races the walker.

**Files changed**:
- `xcl: config.go`
- `xcl: config_diff.go`
- `xcl: config_context_test.go`

**Discoveries**: `parser.Validate` ignores its context entirely, so a context-aware validate has to check the context itself. `state.NewFileStateStore` creates an empty state file up front, so `Exists()` is true before any apply; tests assert on loaded records instead. Language-surface's `GoFuncSignature` rejects variadic funcs (per its changelog record), which settles the plan's open question: host functions declared variadic will be refused with `invalid_function`.
### 2026-10-11 — Task: Entities can be looked up by type without a Go type

**What was done**: Added `Config.FindEntitiesByType(path ...string) ([]any, error)` and `Config.FindOneEntity(path ...string) (any, error)` in `query_untyped.go`. The type and subtype scan behind `FindByType`/`FindOne` was split out of `entitiesOf` into `entitiesByType`, which both the typed and the untyped lookups use, so they share the segment rules and the `NotTypeable`, `UnknownType`, `NotFound` and `NotUnique` errors exactly.

**Deviations**: None. Plugin-provided entities come back as the schema-built struct the Config stores (not the plugin's own Go type), which is the "as stored" contract and what the core service's JSON lookups need.

**Files changed**:
- `xcl: query.go`
- `xcl: query_untyped.go`
- `xcl: query_untyped_test.go`

**Discoveries**: None beyond the above.
### 2026-10-11 — Task: Block types can be declared from a language-neutral schema

**What was done**: Added the schema v1 reader in `internal/schema/v1.go` (`SchemaV1`, `FieldV1`, `IsV1`, `ParseV1`, `BuildTypeV1`), which builds a `reflect.StructOf` type embedding `types.ResourceBase` with the `xcl`, `json` and `default` tags the parser already reads: attributes (string, number as int or float64, bool, any, list, map, object), nested, optional, repeated and named-map blocks, the `label` element field, optional, default, sensitive (limited to `KnownTypes`) and computed fields. `CreateInstanceFromSchema` sniffs `schema_version: 1` and routes to it, leaving the legacy path untouched. `registry.TypeFromSchema(doc)` is the public way to declare such a type.

**Deviations**: Two points of existing xcl behaviour, applying to hand-written types as much as to schema types, shape how two criteria are met. (1) `Config.Validate` deliberately leaves missing required attributes to decoding (`gohcl.CheckBody` clears `Required`), so "a required field missing is a validation error at the block" is met by `Apply`/`Diff` reporting a `*errors.ParserError` positioned at the block's file and line; `Validate` alone returns nil. Recorded as an assumption: the spec's "fails validation at the block's position" is read as this positioned configuration error, which is what a Go program gets too. (2) "A sensitive field is masked in state" holds with a reversible state mask (AES); without one, xcl keeps real sensitive values in state by design, and `mask.Redact()` is refused as a state mask. Events redact by default and lookups show the marker.

**Files changed**:
- `xcl: internal/schema/v1.go`
- `xcl: internal/schema/v1_test.go`
- `xcl: internal/schema/deserialize.go`
- `xcl: registry/schema.go`
- `xcl: registry/schema_test.go`
- `xcl: config_schema_types_test.go`

**Discoveries**: An optional nested block is built as a pointer to the element struct so it may be left out; a required one is the struct itself. The list label field is filled only when a block gives a name, while a map block always fills it from the key. A string default is written as its own text in the `default` tag and anything else as JSON, which `creasty/defaults` parses.
### 2026-10-11 — Task: The core service answers configuration and lookup calls

**What was done**: Added `proto/xcl/core/v1/core.proto`, the CoreService protocol, generated into `coreservice/proto` with buf (`make core-protos`, `buf lint`, `buf breaking` in CI), and the `coreservice` package: `Service` with `Call`/`Release` and the six statuses, a hand-written dispatch table held to the service descriptor by a test, `GetVersion` (link-time `Version`, `ABIVersion` 1), `CreateConfig` mapping every option (local registries with schema-declared types, external plugins and plugin directories; GitHub registries with every option; state path; AES, HMAC, redact and omit masks; event buffer, data level and mask; variables), the lookups (`Entities`, `Find`, `FindByType`, `FindOne`, `Outputs`, `EncodeSavedEntity` with optional ANSI highlighting) returning JSON with the sensitive marker, and the error mapper for every documented code with its details and the config error's positioned causes. The operation methods have handlers that answer "not available" until the next task.

**Deviations**: Handles are a `uint64` from the service's own never-reused table rather than `runtime/cgo.Handle`: `runtime/cgo` does not link under `CGO_ENABLED=0`, so the design's handle mechanism would break the design's own cgo-free guard; every stated property of a handle (process-wide, any thread, `-3` after release) holds. The local buf 1.27 only reads v1 config, so the module is `proto/buf.yaml` with a root `buf.work.yaml`; CI pins buf 1.50.0, which reads both. Lint excepts the rules the design's shared request/response messages and enum value names would trip. The `provider` code is detected from the parser's "<step> failed for <address>: " message, as no structured provider error exists. `state_store.host` and `functions` answer `invalid_argument` until their tasks. A real data race was found and fixed in the root package: `Config.addressParser` cached the address parser without synchronisation, so concurrent `FindResource` calls raced; the cache is now an `atomic.Pointer`.

**Files changed**:
- `xcl: proto/xcl/core/v1/core.proto`
- `xcl: proto/buf.yaml`
- `xcl: buf.work.yaml`
- `xcl: buf.gen.yaml`
- `xcl: Makefile`
- `xcl: .github/workflows/go.yml`
- `xcl: coreservice/proto/core.pb.go`
- `xcl: coreservice/service.go`
- `xcl: coreservice/version.go`
- `xcl: coreservice/config.go`
- `xcl: coreservice/lookups.go`
- `xcl: coreservice/errors.go`
- `xcl: coreservice/operations.go`
- `xcl: coreservice/service_test.go`
- `xcl: coreservice/config_test.go`
- `xcl: coreservice/lookups_test.go`
- `xcl: coreservice/errors_test.go`
- `xcl: config.go`
- `xcl: config_plugin_loading_test.go`

**Discoveries**: `runtime/cgo.Handle` cannot be used by a package that must build with cgo disabled. `Config.addressParser`'s lazy cache was unsynchronised, a latent race for any Go program looking entities up from several goroutines. `configuration.release()` is a no-op until the operations task, so external plugin processes started by a configuration are only stopped when the process exits; the operations task must stop them on release. `mask.Redact()` is refused as a state mask (`mask_not_reversible`).
### 2026-10-11 — Task: Operations run through the poll loop and can be cancelled

**What was done**: Added the operation side of the core service. `coreservice/queue.go` is the per-configuration task queue: one ordered stream of event, state, function and finished tasks, with `run` (post and wait under the operation's context), `post` (no completion), `poll` (wait up to `wait_ms`) and `complete`. `operations.go` starts each of the five operations on a goroutine under a cancellable context, posts `OperationFinished` as its last task (with the diff as JSON for a diff), refuses a second operation or a lookup while one runs, implements `Cancel`, `Poll` and `Complete`, and `release` cancels and waits. `events.go` installs the event handler that posts each event as a task, `state_store.go` is the host state store whose four methods are state tasks, and `config.go` wires both from `CreateConfig`.

**Deviations**: A provider that fails during a lifecycle step reaches the host inside the apply's positioned config error, so the `provider` code with `step` and `address` is now detected from the positioned cause's message as well as from a bare error; the cause also keeps `file`, `line`, `column` and `summary`. Destroy with a host store never sends `CLEAR`: the engine saves after each removed record, ending with an empty save, which the tests pin. `Event.resource_type` is "<type>.<name>" as `events.Event` defines it. The design's per-call debug logging through the event logger was not added: outside an operation there is no event stream to carry it, and inside one it would only log the host's own `Poll` and `Complete` calls.

**Files changed**:
- `xcl: coreservice/queue.go`
- `xcl: coreservice/operations.go`
- `xcl: coreservice/events.go`
- `xcl: coreservice/state_store.go`
- `xcl: coreservice/config.go`
- `xcl: coreservice/errors.go`
- `xcl: coreservice/service_test.go`
- `xcl: coreservice/operations_test.go`
- `xcl: coreservice/queue_test.go`
- `xcl: coreservice/config_test.go`

**Discoveries**: `diff.Diff` cannot be unmarshalled from its own JSON because `entity.Path` has `MarshalJSON` but no `UnmarshalJSON`; a host decodes `diff_json` into its own shape. A config-only registered type is left out of diffs, so diff tests need a provider-backed type. An apply with events on delivers, in order: apply/start, the state EXISTS task, parse/success, EXISTS again, log, create/success, SAVE, log, apply/success, then finished.
### 2026-10-11 — Task: Host-language functions are callable from configuration

**What was done**: `coreservice/functions.go` builds a real Go func for each `DeclaredFunction` with `reflect.FuncOf`/`reflect.MakeFunc` from its `FunctionSignature` (string, bool, number as float64, list and set as a slice, map as a string-keyed map), and the host registry offers it through `Functions()`, so the catalog converts, clash-checks and redacts it exactly as a Go application function. A call encodes its arguments as a JSON array, posts a `FunctionTask` under the running operation's context and decodes the host's `FunctionResult` into the declared return type; a host error fails the call with its message. An unsupported type, a variadic signature, a missing return, malformed signature JSON or an empty name fails `CreateConfig` with `invalid_function`, and a clash with a builtin or another function with `function_name_clash` from `NewConfig`.

**Deviations**: The plan allowed `invalid_argument` for a malformed signature; the acceptance criterion's `invalid_function` is used for every unusable declaration, and `CreateConfig` lets an `InvalidFunctionError` through with its own code rather than wrapping it. Variadic functions are refused rather than built, because language-surface's Go-func conversion rejects them, as the plan's open question anticipated. A root-module fix was needed for cancellation: the parser only reported `context.Canceled` when the walk had no other error, so cancelling with a function task outstanding (whose decode error reads "context canceled") returned a plain config error; `parser.Apply`, `Diff` and `Destroy` now always append the cancellation beside the walk's errors, so `errors.Is(err, context.Canceled)` holds for Go callers and the host sees `cancelled`.

**Files changed**:
- `xcl: coreservice/functions.go`
- `xcl: coreservice/config.go`
- `xcl: coreservice/functions_test.go`
- `xcl: coreservice/service_test.go`
- `xcl: coreservice/config_test.go`
- `xcl: internal/parser/parser.go`
- `xcl: internal/parser/destroy.go`

**Discoveries**: A function is evaluated on every pass that decodes the configuration (parse, decide and apply), so a host sees several tasks per call site and must answer each; tests assert on arguments seen, not on counts. HCL turns a function's error into a diagnostic, so the identity of the Go error is lost below the parser; anything a caller must match with `errors.Is` has to be appended by the parser itself.
### 2026-10-11 — Task: The engine is built as a C shared library

**What was done**: Added the nested module `bindings/libxcl`, the only place cgo is used, with `go.mod` replacing the root module from the checkout during development. `main.go` exports the design's four functions over `coreservice.New()`: `xcl_abi_version`, `xcl_call` (copies the request with `C.GoBytes`, returns the response in `C.CBytes` memory), `xcl_free` and `xcl_release`; `xcl.h` documents the ABI. `callers.go` wraps the exports through C-typed values for `smoke_test.go`, which checks the ABI version, the stamped engine version, creating a configuration, one validate through the poll loop, release, an application error and the negative statuses. The `Makefile` gains `libxcl` (c-shared build, `-trimpath -ldflags "-s -w -X coreservice.Version=$(LIBXCL_VERSION)"`, output `LIBXCL_OUT` with the extension per `GOOS`, `CC`/`GOOS`/`GOARCH` passing through) and `test-libxcl`. CI gains a `libxcl` job that builds the library, checks that exactly the four symbols are exported and runs the smoke test, and the minimum-Go job builds and vets the nested module.

**Deviations**: The smoke test was written in the main context together with the library, before the workflow reached this task, since the library and its test are a few dozen lines that only prove bytes cross the boundary. The test calls the exported functions directly within the package through thin C-typed wrappers rather than `dlopen`ing the built library, the second of the two forms the plan allowed; the exported-symbol check in CI covers the built artifact. The nested module's `go.mod` lists the root module's dependencies as indirect requirements, which `go mod tidy` needs beside the `replace`.

**Files changed**:
- `xcl: bindings/libxcl/go.mod`
- `xcl: bindings/libxcl/go.sum`
- `xcl: bindings/libxcl/main.go`
- `xcl: bindings/libxcl/xcl.h`
- `xcl: bindings/libxcl/callers.go`
- `xcl: bindings/libxcl/smoke_test.go`
- `xcl: Makefile`
- `xcl: .github/workflows/go.yml`
- `xcl: .gitignore`

**Discoveries**: A `_test.go` file cannot `import "C"`, so any test of exported cgo functions needs a non-test helper file. The stripped library is about 22 MB on linux/amd64 with gcc 16, in line with the design's measurement. `go build -buildmode=c-shared` writes a header beside the library, which the `build/` directory is ignored for.
### 2026-10-11 — Task: Go fixture plugins and the Go side of the parity check

**What was done**: Added two external fixture plugins under `bindings/libxcl/testdata/plugins`: `slow` (`resource "slow"`, whose create, update and destroy take `delay_ms`) and `recording` (`resource "recording"`, which appends a `{step, address, time}` JSON line to the block's `log` file before each lifecycle call, then waits `delay_ms`). Added the seven parity fixtures under `bindings/libxcl/testdata/parity` (`basic_types`, `annotations`, `functions`, `variables`, `outputs`, `plugin_entities`, `invalid`), each with the schema v1 `types.json` both sides register, `main.xcl`, and where needed `changed/main.xcl` and `variables.json`, with a README. Added `bindings/libxcl/cmd/parity`, which runs a fixture through the Go API and writes one normalised JSON record covering validate, diff, apply, entities, outputs, state, the changed diff, load in a new process (it execs itself with `-load`) and destroy, with temp and fixture paths replaced by `<tmp>` and `<fixture>`.

**Deviations**: `meta.links` was nondeterministic: the parser walked a block's attributes in map order, so an entity with two references recorded them in random order and two runs of one fixture differed. The parser now walks attributes in the order they were written, which fixes it for Go users too. The `invalid` fixture has no unknown block type, because that stops the parse before any other problem is reported; it carries a computed field set in configuration, an unknown attribute and a missing required attribute instead. Problems in a record are sorted so the record is stable.

**Files changed**:
- `xcl: bindings/libxcl/testdata/plugins/slow/main.go`
- `xcl: bindings/libxcl/testdata/plugins/slow/main_test.go`
- `xcl: bindings/libxcl/testdata/plugins/recording/main.go`
- `xcl: bindings/libxcl/testdata/plugins/recording/main_test.go`
- `xcl: bindings/libxcl/testdata/parity/README.md`
- `xcl: bindings/libxcl/testdata/parity/basic_types/types.json`
- `xcl: bindings/libxcl/testdata/parity/basic_types/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/basic_types/changed/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/annotations/types.json`
- `xcl: bindings/libxcl/testdata/parity/annotations/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/annotations/changed/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/functions/types.json`
- `xcl: bindings/libxcl/testdata/parity/functions/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/variables/types.json`
- `xcl: bindings/libxcl/testdata/parity/variables/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/variables/variables.json`
- `xcl: bindings/libxcl/testdata/parity/outputs/types.json`
- `xcl: bindings/libxcl/testdata/parity/outputs/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/plugin_entities/types.json`
- `xcl: bindings/libxcl/testdata/parity/plugin_entities/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/plugin_entities/changed/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/plugin_entities/variables.json`
- `xcl: bindings/libxcl/testdata/parity/invalid/types.json`
- `xcl: bindings/libxcl/testdata/parity/invalid/main.xcl`
- `xcl: bindings/libxcl/cmd/parity/main.go`
- `xcl: bindings/libxcl/cmd/parity/main_test.go`
- `xcl: internal/parser/parser.go`

**Discoveries**: The builtin for a collection's length is `len`. A `variable` block must carry a `default`, so a fixture that takes its value from the caller still declares one. An output entity's JSON carries an empty `"CtyValue": {}` key from an exported field, which a host's hydration has to tolerate. `json.MarshalIndent` HTML-escapes `<` and `>`, so the `<tmp>` and `<fixture>` placeholders appear escaped in the written file and compare correctly once decoded. go-plugin prints "[ERR] plugin: plugin acceptAndServe error: broker closed" on stderr when an external plugin is stopped; it is harmless.
### 2026-10-11 — Task: Create the GitHub repository jumppad-labs/xcl-python and connect the registered local directory to it

**What was done**: A person created `github.com/jumppad-labs/xcl-python` and connected the registered main checkout `/home/nicj/code/github.com/jumppad-labs/xcl-python` to it as `origin`, with `main` (the "Add spektacular setup" commit) pushed and tracking `origin/main`. On the code side, the spec branch adds the Apache-2.0 `LICENSE`, a copy of xcl's, and a `.gitignore` for virtual environments, build output and the locally built engine.

**Deviations**: The licence was committed on the spec branch in the worktree rather than pushed to `main` by the person, so it reaches `main` when the orchestrator merges the branch.

**Files changed**:
- `xcl-python: LICENSE`
- `xcl-python: .gitignore`

**Discoveries**: None.
### 2026-10-11 — Task: The Python package loads the engine safely

**What was done**: Set up the `xcl` package in xcl-python: `pyproject.toml` (hatchling, CPython 3.10+, `protobuf` and `cffi` with the reason for each, version from `xcl/_version.py`, README as the description, the engine in `xcl/_lib` packaged as an artifact) with `hatch_build.py` tagging wheels `py3-none-<platform>` from `XCL_WHEEL_PLATFORM`; a `Makefile` with `protos` (protoc from the xcl checkout's `proto/` into `xcl/_core`), `libxcl` (the engine built into `xcl/_lib`, stamped with the package version), `plugins` and `test`. The bottom two layers of the SDK: `xcl/_transport` (the `Transport` protocol and `InProcessTransport`, which loads the library lazily with cffi in ABI mode or ctypes as the fallback, copies and frees every response, checks the ABI version on load, and refuses use in a child forked after the engine was loaded through `ForkGuard`, a fork hook plus a pid check, with `ForkedProcessError` naming `spawn` and creating the configuration after the fork), `xcl/_client.py` (the descriptor-built `Client`, mapping status 1 to the exception for the code and negative statuses to `InternalBindingError`, and checking the engine version against the package's on first use), `xcl/_errors.py` (`Error` with one subclass per code, `from_proto`, `ConfigError.problems`, `ForkedProcessError`, `EngineVersionError`, `InternalBindingError`), a minimal `Config` that creates and releases a configuration, and `xcl.engine_version()`.

**Deviations**: Messages are generated with `protoc` rather than buf's remote Python plugin, so development needs no network; protoc lays the file out under `xcl/core/v1`, which would shadow the package, so `make protos` moves it to `xcl/_core/core_pb2.py`. The generated messages are committed, as xcl commits its own. A minimal `Config` (create, `close`, context manager) is included so `xcl.Config().close()` runs against the built engine; its operations and lookups come with the synchronous API task. The fork guard is a class with an injectable instance, so it is tested with a real `os.fork` and no engine.

**Files changed**:
- `xcl-python: pyproject.toml`
- `xcl-python: hatch_build.py`
- `xcl-python: Makefile`
- `xcl-python: README.md`
- `xcl-python: xcl-ref.txt`
- `xcl-python: xcl/__init__.py`
- `xcl-python: xcl/_version.py`
- `xcl-python: xcl/_errors.py`
- `xcl-python: xcl/_client.py`
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/_transport/__init__.py`
- `xcl-python: xcl/_transport/inprocess.py`
- `xcl-python: xcl/_core/__init__.py`
- `xcl-python: xcl/_core/core_pb2.py`
- `xcl-python: xcl/_core/core_pb2.pyi`
- `xcl-python: tests/conftest.py`
- `xcl-python: tests/fakes.py`
- `xcl-python: tests/unit/test_import.py`
- `xcl-python: tests/unit/test_client.py`
- `xcl-python: tests/unit/test_transport.py`
- `xcl-python: tests/unit/test_fork.py`
- `xcl-python: tests/unit/test_errors.py`
- `xcl-python: tests/unit/test_config.py`
- `xcl-python: tests/integration/test_load.py`

**Discoveries**: Python 3.12+ warns on `fork()` in a multi-threaded process, which the Go runtime makes every process that has loaded the engine; the fork tests filter the warning since the child only checks the guard and exits. `hatchling` needs `artifacts` for files git ignores, which the built engine is. The development engine is stamped `v<package version>` so the strict version check passes locally; a release stamps the tag.
### 2026-10-11 — Task: Python classes describe entity types

**What was done**: Added `xcl/_types.py` (`Sensitive[T]` with a masked `repr` and `reveal()`, `Computed[T]` as an annotated optional, `field(name=, block=, label=, default=, default_factory=)` and `UNKNOWN`), `xcl/_entity.py` (`Meta`, the `Resource` base, the `@entity(type, subtype)` decorator that validates the class, derives and stores its schema and registers it, `Entity` for entities of unregistered types, and the lookup helpers) and `xcl/_schema.py` (`schema_for`, which derives the schema v1 document from dataclass annotations including the named-block map and label extension; `signature_for`, which derives a `FunctionSignature` from a function's annotations; `hydrate`, which turns entity JSON into the registered class, nested blocks included, with the sensitive marker and masked envelopes as redacted `Sensitive` values, or into `Entity`). The names are exported from `xcl`.

**Deviations**: A class registered with `@entity` must itself be decorated with `@dataclass`: a `Resource` subclass that is not passes `dataclasses.is_dataclass` through inheritance and would describe a type with none of its attributes, so it is refused with an error saying so. A label field is marked `xcl.field(label=True)` on the element class, rather than named on the parent field, so one element class serves lists and maps alike; `computed` fields carry no `optional` key, as computed implies optional.

**Files changed**:
- `xcl-python: xcl/_types.py`
- `xcl-python: xcl/_schema.py`
- `xcl-python: xcl/_entity.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/unit/test_schema.py`
- `xcl-python: tests/unit/test_signature.py`
- `xcl-python: tests/unit/test_hydrate.py`
- `xcl-python: tests/unit/test_types.py`
- `xcl-python: tests/unit/test_entity.py`

**Discoveries**: The dataclass for the `annotations` parity fixture derives exactly its `types.json`, which the test pins, so the two sides declare the same type. The element dataclass of a block must list its required fields before the label field, which has a default, as a plain dataclass orders them.
### 2026-10-11 — Task: The synchronous API runs every operation and lookup

**What was done**: Completed `xcl.Config` with every option (registries, state path or custom store, state mask, variables, event handler, event data level, buffer and mask) and every operation and lookup: `validate`, `apply`, `diff`, `destroy`, `load`, `entities`, `entity_count`, `find`, `find_by_type`, `find_one`, `all`, `outputs`, `encode_saved_entity`. Added `xcl/registry.py` (`LocalRegistry` with `register_type`, `register_function`, `register_external_plugin`, `register_plugin_directory`; `GitHubRegistry` with every option) as message builders, `xcl/mask.py` (`aes256gcm`, `hmac_sha256`, `redact`, `omit`), `xcl/events.py` (`Event` with `entity()`), `xcl/diff.py` (`Diff`, `ResourceDiff`, `Change`, `Summary`), `xcl/_functions.py` (calling a registered function for a task) and `xcl/_poll.py`: the one poll engine, `Operation` (`step`, `complete`, `cancel`, `finish`), the task answerers and the synchronous `drive`, which runs tasks serially on the calling thread with the GIL released while the engine is polled, and on `KeyboardInterrupt` cancels, drains to the finished task and re-raises.

**Deviations**: `find`, `find_by_type` and `find_one` take the class first, as the design writes them; with `None` they return the registered class or `xcl.Entity`, and a class that does not match the entity's type raises `TypeMismatchError`. `validate` does not report a missing required attribute (pre-existing engine behaviour); `apply` reports it positioned at the block, which the test pins. Cancelling through `KeyboardInterrupt` waits for the provider call in flight, as a cancelled Go apply does. `_functions.py` is included here because the poll engine dispatches function tasks; the functions task adds its tests.

**Files changed**:
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/registry.py`
- `xcl-python: xcl/mask.py`
- `xcl-python: xcl/events.py`
- `xcl-python: xcl/diff.py`
- `xcl-python: xcl/_poll.py`
- `xcl-python: xcl/_functions.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: tests/integration/conftest.py`
- `xcl-python: tests/integration/sync_entities.py`
- `xcl-python: tests/integration/test_sync.py`

**Discoveries**: `Event.resource_type` is the entity's "<type>.<name>", the same as `resource_id`, as `events.Event` documents. The engine aligns `=` in encoded configuration text. A `variable` record is not encodable as configuration. The fixture plugins are built by `make plugins` from the xcl checkout; the integration conftest builds them once per session and skips without Go.
### 2026-10-11 — Task: Python functions are callable from configuration

**What was done**: `LocalRegistry.register_function(fn, name=None)` derives the function's signature from its annotations and declares it when the configuration is created; the poll engine answers each function task by calling the registered function with the decoded JSON arguments (whole numbers as `int`) and returning the result as JSON, or the exception's message as the call's error. Added `tests/integration/test_functions.py`, one test per criterion: the result filled in, arguments seen, a raising function failing the apply at the call's position, a clash with a builtin and between two registries raising `FunctionNameClashError` at configuration creation, an unsupported annotation refused at registration, a variadic function refused by the engine, a sensitive argument arriving plain with the result held sensitive, list, map, number and bool arguments, and registering under another name.

**Deviations**: The code (`register_function`, `_functions.py` and the poll engine's dispatch) landed with the synchronous API task, because the poll engine dispatches function tasks; this task adds its tests. An unsupported parameter annotation is refused in Python at registration, before the engine sees it, while a variadic signature is sent and refused by the engine with `invalid_function`, as planned.

**Files changed**:
- `xcl-python: tests/integration/test_functions.py`

**Discoveries**: None beyond those recorded for the core service: a function is evaluated on every decode pass, so a host sees several tasks per call site.
### 2026-10-11 — Task: Go plugins are usable from Python

**What was done**: Added the Docker example plugin tests: `tests/plugins.toml` pins the release (`jumppad-labs/xcl-plugin-docker` v0.2.0), `tests/keys/jumppad-labs.asc` is the jumppad-labs release key published at xcl.dev, and `tests/integration/configs/docker/main.xcl` declares a network and an nginx container. `test_docker_plugin.py` applies that configuration with the plugin loaded from the GitHub registry (installed once per session with the trusted key), from the installed binary's path and from its directory, and reads the plugin's entities generically as `xcl.Entity`, destroying in every case. `test_github_tamper.py` serves the real release assets from a local fake GitHub API and shows a checksums file with one digit changed, and a release signed by a key that is not trusted, are refused with `xcl.PluginVerificationError`, with a positive control through the same fake. The tests skip with a clear reason when no Docker daemon is reachable.

**Deviations**: The published Docker plugin release is v0.2.0, dated before xcl v0.3.0; the plugin protocol is additive, so it loads and runs unchanged, and the pin names it. The session install runs `validate`, which installs and starts the plugin without touching Docker. Docker availability is checked by pinging `DOCKER_HOST` (podman's `docker` shim ignores it, the Go plugin honours it) before `docker info`. A release served with no `.sig` installs when no trusted keys are given, the engine's documented behaviour, which a test pins.

**Files changed**:
- `xcl-python: tests/plugins.toml`
- `xcl-python: tests/keys/jumppad-labs.asc`
- `xcl-python: tests/integration/configs/docker/main.xcl`
- `xcl-python: tests/integration/docker_helpers.py`
- `xcl-python: tests/integration/test_docker_plugin.py`
- `xcl-python: tests/integration/test_github_tamper.py`

**Discoveries**: The GitHub registry keeps the archive beside the binary, checksums and signature in the cache entry `<cache>/github.com/<owner>/<repo>/<tag>/<os>_<arch>/`. Plugin-directory discovery requires the executable bit, so the sidecar files in a cache entry are skipped and the entry directory can be registered directly. A tampered checksums file fails at the signature step with a trusted key and at the checksum step without one.
### 2026-10-11 — Task: Python gives the same results as Go on the parity fixtures

**What was done**: Added `tests/parity`: `types.py` holds one dataclass per fixture type and `register(fixture)` registers them right before a run; `test_types.py` checks, per fixture, that the derived schema documents equal the fixture's `types.json`, so the two sides declare the same types; `run_python.py` is the Python counterpart of xcl's `cmd/parity`, stage for stage (validate, diff, apply, entities, outputs, state, the changed diff, load in a new process, destroy) with the same normalisation (paths to `<tmp>` and `<fixture>`, problems sorted with Go's map-print key, entities and state by id); `test_parity.py` runs both programs per fixture and compares the decoded records, failing with the fixture name and the first differing JSON path. All seven fixtures give identical records.

**Deviations**: The `outputs` fixture's number list omitted `format`, which Go defaults to `int` but the Python deriver always writes; the fixture now spells `int` out like every other, so the strict document equality holds. `run_python.py` reads the raw entity JSON through the client rather than hydrated objects, since the record is the engine's JSON. `tests/parity/types.py` shadows the standard library's `types` module when the runner is executed as a script, so the runner fixes `sys.path` before importing anything else.

**Files changed**:
- `xcl-python: tests/parity/__init__.py`
- `xcl-python: tests/parity/conftest.py`
- `xcl-python: tests/parity/types.py`
- `xcl-python: tests/parity/run_python.py`
- `xcl-python: tests/parity/test_parity.py`
- `xcl-python: tests/parity/test_types.py`
- `xcl: bindings/libxcl/testdata/parity/outputs/types.json`

**Discoveries**: `HOME` set in `os.environ` before the engine loads is seen by the Go runtime inside the library, so a Python program can isolate the plugin cache the same way a Go test does. Go's `errors.As` on a `ConfigError` reaches the first positioned cause, which the Python side reproduces by walking `causes` depth first.
### 2026-10-11 — Task: The asyncio API never blocks and can be cancelled

**What was done**: Added `xcl/aio.py` with `xcl.aio.Config`: the same constructor as `xcl.Config` and every operation and lookup as an awaitable. The engine is polled from a one-thread executor through the shared poll engine's `step`, and each task it posts is dispatched as an asyncio task: an `async def` function, event handler or state-store method is awaited on the loop and a plain one runs in the default executor. Cancelling the awaiting task cancels outstanding dispatches, sends `Cancel`, drains to the finished task under a shield so the engine saves its progress, then re-raises `asyncio.CancelledError`. `tests/integration/test_aio.py` covers an async and a sync function, an async event handler, async and sync state stores, other coroutines and an asyncio HTTP server progressing during a slow apply, prompt cancellation, no provider call starting after a cancel (from the recording plugin's log), every lookup awaited, diff and destroy, the async context manager, and the sync and async surfaces being equal with every async method a coroutine function.

**Deviations**: A lifecycle call already running when an operation is cancelled finishes first, because the engine gives a provider a context that is never cancelled; "ends promptly" therefore means within one provider call, which the test bounds at a few seconds, while function and state tasks waiting on the host are released at once. The asyncio configuration is created on `open()`, `async with` or the first operation rather than in `__init__`, since creating it loads the engine and must not block the loop. `xcl.aio.Config` has `open()` beside the sync surface.

**Files changed**:
- `xcl-python: xcl/aio.py`
- `xcl-python: tests/integration/test_aio.py`

**Discoveries**: A cancelled apply leaves the already parsed entities in the configuration, so `entities()` after a cancel is not empty. `asyncio.wait_for` on a task cancelled from outside raises `CancelledError` in the awaiter, which a test must catch rather than expect `TimeoutError`.
### 2026-10-11 — Task: Tagging xcl publishes signed engine builds for eight platforms

**What was done**: Added `.github/workflows/libxcl-release.yml`, xcl's first release workflow. On a `v*` tag push, or by hand for a tag with `publish` off, it builds libxcl for Linux x86-64 and arm64 (glibc 2.28 and musl, cross-compiled with `zig cc` on one Linux runner), Windows x86-64 (zig) and arm64 (llvm-mingw), and macOS x86-64 and arm64 (GitHub's runners, deployment target 11.0), each stamped with the tag through `make libxcl`; checks the glibc builds need no symbol newer than 2.28; packages each as `libxcl_<version>_<os>_<arch>[_musl].tar.gz` (`.zip` on Windows) with the header, writes `libxcl_<version>_checksums.txt`, signs it with the jumppad-labs release key from the repository secrets and verifies the signature; keeps the assets as a workflow artifact; and, when publishing, creates the release if needed, attaches the assets and dispatches `xcl-release` to `jumppad-labs/xcl-python` with the tag.

**Deviations**: The four acceptance criteria are left unchecked: they describe what a run of the workflow on GitHub produces, which the next task's manual dry run (with the signing key in place) and the first real release verify. Locally the workflow parses, `make libxcl` takes `CC`, `GOOS` and `GOARCH` as the matrix passes them, and the package job's shell was run over eight stand-in builds: eight archives with the asset names the registry verifies, an eight-line checksums file and a detached signature that verifies. The asset version drops the leading `v`, as the registry's contract does. The musl suffix is on the arch, an extension of the contract the plan records.

**Files changed**:
- `xcl: .github/workflows/libxcl-release.yml`

**Discoveries**: A libxcl built with the local gcc 16 needs glibc 2.34 symbols, which is why the release builds target `gnu.2.28` through zig; the workflow's symbol check fails a build that regresses. `zip` is not installed on this machine, while ubuntu runners have it.
### 2026-10-11 — Task: xcl-python is tested on every platform and released from the xcl tag

**What was done**: Added `.github/workflows/ci.yml`: a matrix of `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-13`, `macos-latest`, `windows-latest` and `windows-11-arm` against CPython 3.10, 3.11, 3.12, 3.13 and the latest `3.x`, plus Alpine containers on both Linux runners for musl, each checking out xcl at the ref in `xcl-ref.txt`, generating the messages, building the engine and the fixture plugins with Go and a C compiler, and running the suite; and a package check (`build --sdist`, `twine check`). Added `.github/workflows/release.yml`, run by the `xcl-release` dispatch from xcl's release or by hand for a tag: per platform it downloads the engine archive, checksums and signature from the xcl release, verifies the signature against `tests/keys/jumppad-labs.asc` and the archive against the checksums, places the engine in the package, sets the version from the tag, regenerates the messages from xcl at the tag, checks the engine reports the tag, builds a wheel tagged `py3-none-<platform>`, installs that wheel and runs the parity suite and the load test against it; builds an sdist; publishes everything from the `pypi` environment with Trusted Publishing; and installs `xcl==<version>` from PyPI on every platform with every Go and C compiler hidden from the path, running the smoke import and the engine version check. `pyproject.toml` lists what the sdist holds and excludes the built engine.

**Deviations**: The four acceptance criteria are left unchecked: they describe runs on GitHub, verified by the first release. Locally both workflows parse, the sdist builds and passes `twine check` without the engine or stray files, a wheel built with `XCL_WHEEL_PLATFORM=manylinux_2_28_x86_64` carries the tag and the engine, and the suite passes. Messages are generated with `protoc` (`arduino/setup-protoc`) rather than buf's remote plugin. The release's wheel test runs the parity suite and the load test rather than the whole suite, since the Docker tests need a daemon the runners lack.

**Files changed**:
- `xcl-python: .github/workflows/ci.yml`
- `xcl-python: .github/workflows/release.yml`
- `xcl-python: pyproject.toml`

**Discoveries**: hatchling's sdist needs `only-include` to keep a repository's other directories (such as `.spektacular`) out; `include` alone does not. Python 3.12+ warns about forking a multi-threaded process, which every process that has loaded the engine is.
### 2026-10-11 — Task: The Python package is documented

**What was done**: Wrote the package `README.md`: install (`pip install xcl-lang`, the eight platforms and CPython 3.10+), a synchronous getting started that defines an entity class, registers the Docker example plugin through `GitHubRegistry` with the trusted release key, applies, finds and destroys, then entity types, functions, variables, state and events, the asyncio section, errors, fork safety with its two ways out, development and the licence. Wrote `PARITY.md`, mapping every Go operation, lookup, option and registry call to its `xcl.Config` and `xcl.aio.Config` equivalent, with the "not applicable" rows (`Decode`, `As`, `EncodeEntity`, the deprecated `GetResources`/`ResourceCount`). Gave every public module, class, function, method and property a docstring, and added `tests/unit/test_docstrings.py`, which imports the package and walks `xcl.__all__`, `xcl.aio` and `xcl.mask` with `inspect`. `pyproject.toml` uses the README as the description; `twine check` passes on the built wheel and sdist.

**Deviations**: The getting-started example registers a config-only class beside the Docker plugin rather than only plugin types, to show both in one program.

**Amendments**: The spec's Requirements ("Install with pip, no toolchain"), Acceptance Criteria ("Clean install on every platform") and Success Metrics were amended by the orchestrator during the PyPI task: the name `xcl` on PyPI belongs to another project, so the distribution is published as `xcl-lang` and imported as `xcl`. The rename was applied to `pyproject.toml`, the README, the release workflow's uninstall and install-from-PyPI steps, the transport and client messages, the package docstring, and xcl's CHANGELOG entry and README note; the suite, a tagged wheel (`xcl_lang-<version>-py3-none-<platform>.whl`), the sdist and `twine check` were rerun green. The already ticked CI/release task's criteria name `pip install xcl==<version>` in the plan text; the workflow now installs `xcl-lang==<version>`, and those criteria remain to be verified by the first release.

**Files changed**:
- `xcl-python: README.md`
- `xcl-python: PARITY.md`
- `xcl-python: tests/unit/test_docstrings.py`
- `xcl-python: xcl/__init__.py`
- `xcl-python: xcl/_client.py`
- `xcl-python: xcl/_errors.py`
- `xcl-python: xcl/_transport/inprocess.py`
- `xcl-python: xcl/_entity.py`
- `xcl-python: xcl/_poll.py`
- `xcl-python: xcl/registry.py`
- `xcl-python: pyproject.toml`
- `xcl-python: .github/workflows/release.yml`

**Discoveries**: The docstring walk has to unwrap `property`, `classmethod` and `staticmethod` objects to reach the function whose `__doc__` is checked.
### 2026-10-11 — Task: Record the Python support in the xcl changelog

**What was done**: Inserted the `## 20261011130709-aa3eb933-python-support` entry at the top of `CHANGELOG.md`, above language-surface's, in prose: the Python package (`xcl-lang` on PyPI, imported as `xcl`) and what a Python program can do; the context-taking operations and the cancellation they carry; the untyped lookups; links recorded in written order and the atomic address parser; schema v1 and `registry.TypeFromSchema`; the `coreservice` package and its protocol; the `bindings/libxcl` module, its four C functions, `make libxcl` and the release assets; and the fixture plugins and parity program; then **Breaking:** None. Added the "Other languages" section to `README.md`, saying the engine is available to Python as `xcl-lang` from jumppad-labs/xcl-python and that Go users are unaffected.

**Deviations**: None. The entry also records the three root-module fixes made during the work (cancellation always reported, links in written order, the address parser published atomically), since Go users see them.

**Files changed**:
- `xcl: CHANGELOG.md`
- `xcl: README.md`

**Discoveries**: None.
### 2026-10-11 — Task: Add the release signing key and dispatch token to xcl

**What was done**: The person added the secrets `XCL_RELEASE_GPG_KEY`, `XCL_RELEASE_GPG_PASSPHRASE` and `XCL_PYTHON_DISPATCH_TOKEN` to `jumppad-labs/xcl` (2026-10-11, about 20:26-20:32Z), the names the release workflow reads.

**Deviations**: The manual dry run of the release workflow was skipped by the user's decision; the real release (workflow run 38172909823 for `v0.4.0`) verifies the workflow and the signature, so the dry-run criterion stays unchecked and is confirmed by that run.

**Files changed**:
- none (repository settings)

**Discoveries**: None.

### 2026-10-11 — Task: Set up the PyPI project and trusted publishing

**What was done**: The person added a PyPI pending publisher for the project `xcl-lang` (owner `jumppad-labs`, repository `xcl-python`, workflow `release.yml`, environment `pypi`) and created the `pypi` environment on `jumppad-labs/xcl-python`, which the release workflow's `publish` job runs in with `id-token: write`.

**Deviations**: The project is `xcl-lang`, not `xcl`: the name on PyPI belongs to another owner, so the spec was amended and the distribution renamed, with the import name kept as `xcl`. A pending publisher is used, since the project has no release yet; PyPI turns it into the project's publisher on the first upload.

**Files changed**:
- none (PyPI and repository settings)

**Discoveries**: None.

### 2026-10-11 — Task: Cut the first xcl release with Python support

**What was done**: Both spec branches were merged, xcl at `93e1904` on `f-libxcl` and xcl-python at `81673e5` on `main`, and the orchestrator tagged xcl `v0.4.0` on `93e1904` (annotated tag `a9eaf06`) and pushed it. The libxcl release workflow run 38172909823 was queued by the push; it builds and signs the eight archives, attaches them to the release and dispatches the xcl-python release, which publishes `xcl-lang==0.4.0`.

**Deviations**: The tag was cut by the orchestrator rather than by the person, by the user's decision. The task's two acceptance criteria, the eight signed archives on the release and the same version on PyPI with its install checks passed, are confirmed by run 38172909823 and the xcl-python release run it dispatches; they stay unchecked until then, as do the CI-only criteria of the release-workflow and CI/release tasks.

**Files changed**:
- none (a tag on xcl)

**Discoveries**: None.
