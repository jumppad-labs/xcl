---
created_date: "2026-10-11"
document_status: draft
spec: 20261023075136-42c51a7d-printing-tokens-over-the-engine
specs:
    - 20261011130709-aa3eb933-python-support
    - 20261011211634-a8895873-python-out-of-process-engine
    - 20261016064106-8931613a-python-encode-entity
---

# xcl for other languages: the core service and its transports

xcl is a Go library. An application imports `github.com/jumppad-labs/xcl`, registers its Go types and plugins, and calls `Apply`, `Validate`, `Diff`, `Destroy` and `Load` on a `Config`. This design makes the same engine available to Python, Node and any other language without reimplementing any of it: the Go core is wrapped once as a **core service** whose every operation is one protobuf message, a small static `xcl-core` program carries those messages over its standard input and output to the host process, and providers written in the host language are driven by the core through a task queue the host polls. The same messages can also cross a four-function C ABI from a shared library loaded in process (`libxcl`); that transport is kept buildable but is not what the SDKs ship.

The protobuf is not an RPC interface in the gRPC sense: nothing listens on a socket. A binding starts the bundled `xcl-core` with two pipes and exchanges length-prefixed frames with it; protobuf defines the method names and the request and response layouts, so the transport stays a handful of lines while the API grows in the `.proto` files. Because the API is messages rather than functions, the transport is interchangeable: the shared library carries the same messages through one C function, and a static archive or a Wasm build could carry them too. Which transport an SDK ships is a packaging decision, revisited in [Subprocess as the default transport](#subprocess-as-the-default-transport-and-the-way-back-in-process); the API, the client and everything above them do not change with it.

## Goals and non-goals

Goals:

- A Python program can do everything a Go program does with xcl: build a configuration, register types, run every operation, read entities, outputs, events and diffs, use the GitHub registry and external plugins, and write providers in Python that the core calls during an apply.
- The parser, dependency graph, lifecycle, change detection, sensitive handling, state and events stay in Go and run unchanged. No language reimplements any of them.
- One API definition. The `CoreService` protobuf is the only description of what the core does for a host; the C ABI carries it, and every language generates its message types from it.
- Pure Go users are unaffected. Importing the root module pulls in no cgo, no subprocess and no shared library.
- Every supported platform gets the same package, musl included. Alpine is a common base image for Python, Java and .NET services, and a binding must work there exactly as on glibc.
- No C toolchain in the release path. The engine the SDKs ship is cross-compiled for every target from one runner with cgo disabled.
- Plugins written in one language work with applications written in another.

Non-goals:

- A browser or Wasm build. A validate-only Wasm build is a later option, not part of this design.
- A C API for C programs. The C ABI exists to be called by a language binding, not written against by hand.
- Supporting two Go runtimes in one process. See the hazards section.
- Shipping the in-process shared library in any SDK package. `bindings/libxcl` stays buildable and tested for the day it is wanted again; see [the way back in process](#subprocess-as-the-default-transport-and-the-way-back-in-process).
- Platforms that forbid starting a process (iOS and store-sandboxed apps, browsers, edge isolates). They also forbid loading a Go shared library; they need a static archive or a Wasm build, which the message API allows as further transports when asked for.

## The guarantee for Go users

The root module `github.com/jumppad-labs/xcl` is unchanged by this work. Nothing in it imports `"C"`, nothing in it starts a process, and `go build ./...` and `go test ./...` need no C toolchain. The Go CI job already builds the root with `CGO_ENABLED=0` for darwin, linux and windows (`.github/workflows/go.yml`); that build stays as the guard, and the `coreservice` package added by this design is built by it too, so cgo cannot leak into the library without failing CI. External Go plugins keep the protocol they have, and gain protocol v2 by rebuilding.

## Architecture

### Layers

| Layer | Path | cgo | Role |
|---|---|---|---|
| Library | `github.com/jumppad-labs/xcl` (root module) | no | Unchanged. Go users import it directly. |
| Core service | `coreservice/` (package in the root module) | no | Implements `CoreService` on the public xcl API: handles, the message dispatcher, the foreign provider, state store and event adapters, the task queue, error mapping. Tested with plain `go test`. |
| Engine program | `cmd/xcl-core/` (in the root module) | no | A static binary serving `coreservice` over stdio. The transport every SDK ships. Built by the `CGO_ENABLED=0` job for every target from one runner. |
| Shared library | `bindings/libxcl/` (its own `go.mod`) | yes | `package main`, `//export xcl_call` and three more exports: the same `coreservice` behind a four-function C ABI, for a host that links or loads it in process. Kept buildable and smoke-tested; not shipped in SDK packages. |
| Language SDKs | `xcl-python`, `xcl-node` (own repositories) | no | Start `xcl-core`, speak `CoreService`, present a native API, derive schemas from native types, run the poll loop. |

Dependencies point one way: SDK → transport → `coreservice` → `xcl`. The root module never imports `coreservice`, and `coreservice` imports only the public packages of the root module (`xcl`, `registry`, `plugins`, `state`, `events`, `diff`, `mask`, `types`, `entity`, `errors`, `logger`). Where `coreservice` needs something internal, that something is made public in the root module first.

### Why the shared library is a nested module

A Go package that imports `"C"` has no buildable files when `CGO_ENABLED=0`. If `bindings/libxcl` sat inside the root module, `go build ./...` and `go vet ./...` on a machine without a C compiler, or in the `CGO_ENABLED=0` CI job, would fail for everyone, including users who only want the library. Giving `bindings/libxcl` its own `go.mod` keeps it out of the root module's `./...`. During development its `go.mod` carries `replace github.com/jumppad-labs/xcl => ../..`; both modules are tagged together on release, with the nested module requiring the root at the same tag. The examples under `example/` already follow this pattern, each with its own `go.mod`, listed in the `Makefile` and built as separate CI jobs.

`cmd/xcl-core` needs no cgo, so it sits in the root module and is built by the `CGO_ENABLED=0` job like everything else; it cross-compiles for every target from one runner with no C toolchain, which is why it is the transport the SDKs ship.

### What `coreservice` is

`coreservice` is the whole of the binding logic, written once and transport-free:

```go
package coreservice

// Service holds every configuration a host has created, by handle, and
// dispatches calls to them. One Service per process.
type Service struct { ... }

func New() *Service

// Call dispatches one CoreService method. method is the method name,
// request the serialised request message. It returns the serialised
// response and a status; see Status. It is safe for concurrent use.
func (s *Service) Call(handle uint64, method string, request []byte) (response []byte, status Status)

// Release frees the configuration behind handle and everything it owns.
func (s *Service) Release(handle uint64) Status
```

The shared library is thin: it turns C bytes into `Call` and back. Every test of behaviour is a Go test against `Service.Call`, which needs neither a C compiler nor Python. `coreservice` must stay free of any transport: that is what keeps both transports thin wrappers rather than second implementations.

## The C ABI

The in-process transport. Four exported functions, nothing else; the same messages the engine program carries over stdio cross here through one call. It is the natural shape for a host that links the engine statically (a static archive is the same code with a different `-buildmode`) and for a host where starting a process is impossible, and it is what the SDKs would load if the in-process route is ever made the default again. The SDKs do not ship it today; the reasons are in [Go c-shared hazards](#go-c-shared-hazards-and-why-the-sdks-do-not-ship-it) and the decision section.

```c
#include <stddef.h>
#include <stdint.h>

/* The ABI version this library implements. A binding compares it with the
   version it was written against before making any other call. */
uint32_t xcl_abi_version(void);

/* Dispatches one CoreService method.
   handle   a configuration handle from CreateConfig, or 0 for a method that
            needs none (CreateConfig, GetVersion).
   method   the CoreService method name, NUL terminated, e.g. "StartApply".
   req      the serialised request message; read during the call only.
   resp     on return, a buffer the caller frees with xcl_free; NULL when
            resp_len is 0.
   Returns a status, see below. */
int32_t xcl_call(uint64_t handle,
                 const char *method,
                 const uint8_t *req, size_t req_len,
                 uint8_t **resp, size_t *resp_len);

/* Frees a buffer returned through resp. NULL is ignored. */
void xcl_free(uint8_t *resp);

/* Frees the configuration behind handle, cancelling any operation in
   progress and waiting for it to stop. Returns a status. */
int32_t xcl_release(uint64_t handle);
```

Status values:

| Value | Meaning | `resp` holds |
|---|---|---|
| `0` | Success | the method's response message |
| `1` | Application error | an `Error` message (below) |
| `-1` | Unknown method | nothing |
| `-2` | Request bytes do not parse as the method's request | nothing |
| `-3` | Unknown or released handle | nothing |
| `-4` | Method needs a handle and got 0, or the reverse | nothing |

Negative statuses are programming errors in the binding and never carry a message; a binding raises them as its own internal error. Status `1` is the normal failure path and carries a structured `Error`.

### Memory ownership

- Request bytes belong to the caller. The core copies them (`C.GoBytes`) before `xcl_call` returns and never keeps a pointer into caller memory.
- Response bytes are allocated with C `malloc` (`C.CBytes`), so they live in C memory, are not moved by the Go garbage collector and need no pinning. The caller frees each one with `xcl_free`, exactly once.
- No Go pointer ever crosses to C, in either direction. This is what keeps the library inside the cgo pointer-passing rules without `runtime.Pinner`.
- Strings in messages are UTF-8; the method name is the only C string.

### Handles

A handle is a `uint64` produced by `runtime/cgo.Handle` from the `*coreservice.configuration` it names. C sees an integer; the Go object is reachable only through the handle table, so the garbage collector keeps it alive until `xcl_release` deletes the handle. There is one kind of handle, the configuration. Registries, state stores and event handlers are described in the `CreateConfig` request rather than being handles of their own, which keeps the lifetime rules to one sentence: everything a configuration owns is released with it. The same pattern is used by go-jsonnet's C bindings and by gopy.

Handles are process-wide and valid from any thread. A handle used after release returns `-3`.

### Error model

Every application error crosses as one message:

```protobuf
message Error {
  // code is stable across releases and is what a binding switches on to pick
  // an exception class. message is for people.
  string code = 1;
  string message = 2;
  // details carry the fields the Go detail type carries, by name: address,
  // type, plugin, registry, repository, version, platform, count, ...
  map<string, string> details = 3;
  // causes are the wrapped errors, in order, for an error that collects
  // several, such as a configuration that fails validation in three places.
  repeated Error causes = 4;
  // report is the error's report as Go prints it with xcl.PrintError,
  // labelled; set only when the error holds a configuration problem.
  repeated Token report = 5;
}
```

An error that holds a configuration problem carries its full report in `report`: the report Go's `xcl.PrintError` prints, with its severity, summary, location, the numbered lines of source around the fault with the fault marked, and the detail, as labelled text (see [Labelled text](#labelled-text)). The core lays the report out once, when it converts the error, with the same function `xcl.PrintError` uses, so the report's plain text is Go's plain report exactly. It is sent unwrapped: the detail is one line, and each language wraps it to its own width when it prints. The report is set on every error that holds a configuration problem, wherever it is in the error's tree, and on each cause that holds one, so the top-level error of a failed apply carries every problem's report in order and each cause carries its own. That includes a provider failure the parser reports at its block (code `provider`) and errors carried by events while an operation runs, so a host prints a problem with its source from the error it already received, including from an event handler, and never asks the engine again. An error with no configuration problem, such as `not_found`, has an empty `report` and its message unchanged. The report travels on every `Error` rather than through a separate call, so printing one costs no engine call.

`coreservice` maps Go errors to codes with `errors.Is` and `errors.As` against the sentinels and detail types the root module exports from `config.go` and `errors.go`:

| Code | Go error |
|---|---|
| `not_found` | `xcl.ErrNotFound` (`NotFoundError.Address` in details) |
| `not_unique` | `xcl.ErrNotUnique` (`count`) |
| `unknown_type` | `xcl.ErrUnknownType` |
| `not_typeable` | `xcl.ErrNotTypeable` |
| `not_registered` | `xcl.ErrNotRegistered` |
| `type_mismatch` | `xcl.ErrTypeMismatch` |
| `not_an_entity` | `xcl.ErrNotAnEntity` |
| `type_name_clash` | `*xcl.TypeNameClashError` |
| `type_form` | `*xcl.TypeFormError` |
| `plugin_load` | `xcl.ErrPluginLoad` (`plugin`, `registry`) |
| `plugin_not_found` | `xcl.ErrPluginNotFound` (`repository`, `version`, `platform`) |
| `plugin_verification` | `xcl.ErrPluginVerification` |
| `empty_configuration` | `xcl.ErrEmptyConfiguration` |
| `unregistered_type` | `xcl.ErrUnregisteredType` |
| `invalid_saved_data` | `xcl.ErrInvalidSavedData` |
| `not_encodable` | `xcl.ErrNotEncodable` |
| `unrecoverable` | `xcl.ErrUnrecoverable` |
| `mask_not_reversible` | `xcl.ErrMaskNotReversible` |
| `config` | `*errors.ConfigError`; each collected error becomes a cause, itself mapped |
| `provider` | an error a provider returned, with `address` and `step` in details |
| `cancelled` | the operation was cancelled by the host |
| `operation_in_progress` | a lookup other than `EncodeSavedEntity` was made while an operation was running on the handle |
| `invalid_argument` | a request that parsed but makes no sense, e.g. no paths |
| `invalid_theme` | `*errors.InvalidThemeError` from `ConvertTheme`, a VS Code theme it cannot use (`source`, `reason`) |
| `internal` | anything not matched above |

The code list is part of the ABI contract: a code is never renamed or reused, only added. Bindings map codes to native exceptions (`xcl.NotFoundError`, `xcl.PluginLoadError`, ...) with an `xcl.Error` base class holding `code`, `details` and `causes`, so an unknown code still raises something catchable.

### Threading

`xcl_call` may be called from any thread, concurrently, with any handles. Each call runs on a goroutine the cgo runtime provides; nothing is pinned to the calling thread. Within one handle:

- At most one operation (`StartValidate`, `StartApply`, `StartDiff`, `StartDestroy`, `StartLoad`) runs at a time; starting a second returns `operation_in_progress`.
- `Poll`, `Complete`, `Log`, `StateGet`, `StateFind` and `Cancel` are allowed while an operation runs. They are the host's side of the operation.
- Lookups (`Find*`, `Entities`, `Outputs`, `EncodeEntity`) are refused with `operation_in_progress` while an operation runs, because `Config.Apply` and `Config.Destroy` replace the entity slice when they finish and the root module does not promise concurrent reads during an operation.
- `EncodeSavedEntity` is allowed while an operation runs, including from a host's handling of the operation's events, because it reads none of the configuration's entities: it types the record through the plugin catalog, whose use counting keeps the operation's plugins running, and renders it. A host can therefore render each resource's saved record as its create success event arrives, and the text is the same as after the operation.

The binding is told to release the GIL (Python) or use an asynchronous call (Node) around `xcl_call`, since `Poll` blocks for up to its wait time.

## The CoreService

The service is defined once, in `coreservice/coreservice.proto`, package `xcl.core.v1`. Entity data crosses as JSON bytes, exactly as it already does between the parser and a `plugins.ProviderAdapter` (`[]byte` in, `[]byte` out), so the core's existing serialisation (`internal/wire`) is used unchanged and no message mirrors an entity's fields. Protobuf gives the envelope: method names, stable field numbers, enums, and generated encoders in every target language.

Method names are the `method` string of `xcl_call`. The `service` block is documentation of the mapping; nothing generates a gRPC server from it for the in-process transport.

```protobuf
syntax = "proto3";
package xcl.core.v1;
option go_package = "github.com/jumppad-labs/xcl/coreservice/proto";

service CoreService {
  // Library (handle 0: no configuration is needed)
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);
  rpc ConvertTheme(ConvertThemeRequest) returns (ConvertThemeResponse);

  // Configuration lifecycle (CreateConfig takes handle 0 and returns one)
  rpc CreateConfig(CreateConfigRequest) returns (CreateConfigResponse);

  // Operations: each starts work on the handle and returns at once. The host
  // polls until it receives an OperationFinished task for the operation.
  rpc StartValidate(StartValidateRequest) returns (StartOperationResponse);
  rpc StartApply(StartApplyRequest) returns (StartOperationResponse);
  rpc StartDiff(StartDiffRequest) returns (StartOperationResponse);
  rpc StartDestroy(StartDestroyRequest) returns (StartOperationResponse);
  rpc StartLoad(StartLoadRequest) returns (StartOperationResponse);
  rpc Cancel(CancelRequest) returns (CancelResponse);

  // Host side of an operation
  rpc Poll(PollRequest) returns (PollResponse);
  rpc Complete(CompleteRequest) returns (CompleteResponse);
  rpc Log(LogRequest) returns (LogResponse);
  rpc StateGet(StateGetRequest) returns (StateGetResponse);
  rpc StateFind(StateFindRequest) returns (StateFindResponse);

  // Lookups, between operations
  rpc Entities(EntitiesRequest) returns (EntitiesResponse);
  rpc Find(FindRequest) returns (EntityResponse);
  rpc FindByType(FindByTypeRequest) returns (EntitiesResponse);
  rpc FindOne(FindByTypeRequest) returns (EntityResponse);
  rpc Outputs(OutputsRequest) returns (OutputsResponse);
  // EncodeSavedEntity reads none of the entities, so unlike the other
  // lookups it is also allowed while an operation runs
  rpc EncodeSavedEntity(EncodeSavedEntityRequest) returns (EncodeSavedEntityResponse);
  rpc EncodeEntity(EncodeEntityRequest) returns (EncodeEntityResponse);

  // Types
  rpc DescribeTypes(DescribeTypesRequest) returns (DescribeTypesResponse);
}
```

### Creating a configuration

`CreateConfigRequest` is the whole of `xcl.NewConfig(opts...)`: every option in `options.go` has a field, and every registry is described rather than referenced.

```protobuf
message CreateConfigRequest {
  repeated Registry registries = 1;          // WithRegistry, in order
  StateStore state_store = 2;                // WithStateStore / WithStatePath
  Mask state_mask = 3;                       // WithStateMask
  bool events = 4;                           // WithEventHandler: deliver events through Poll
  int32 event_buffer_size = 5;               // WithEventBufferSize, 0 is the default
  EventDataLevel event_data = 6;             // WithEventData
  Mask event_mask = 7;                       // WithEventMask; unset keeps mask.Redact()
  bool no_event_mask = 8;                    // WithNoEventMask
  bytes variables_json = 9;                  // WithVariables, a JSON object
}

message CreateConfigResponse { uint64 handle = 1; }

message Registry {
  oneof kind {
    LocalRegistry local = 1;
    GitHubRegistry github = 2;
  }
}

message LocalRegistry {
  string plugin_pattern = 1;                 // registry.PluginPattern, empty keeps the default
  repeated DeclaredType types = 2;           // RegisterType, and host-provided types
  repeated string external_plugins = 3;      // RegisterExternalPlugin
  repeated string plugin_directories = 4;    // RegisterPluginDirectory
}

// DeclaredType is a block type the host defines. Without a provider it is a
// config-only type, as registry.RegisterType declares; with one, the host's
// provider is called through Poll for every lifecycle step.
message DeclaredType {
  string type = 1;
  string subtype = 2;
  bytes schema = 3;                          // schema v1 JSON
  bool provided = 4;
  // implements lists the lifecycle steps the host's provider defines. A
  // step not listed takes the core's default: Validate accepts, Changed is
  // plugins.DefaultChanged. Create, Read, Update and Destroy are required.
  repeated Step implements = 5;
}

message GitHubRegistry {
  repeated GitHubPlugin plugins = 1;         // RegisterPlugin("owner/repo", "vX.Y.Z")
  string cache_dir = 2;                      // GitHubCacheDir
  repeated string trusted_keys = 3;          // GitHubTrustedKeys, armoured
  string token = 4;                          // GitHubToken
  string api_url = 5;                        // GitHubAPIURL
}
message GitHubPlugin { string repository = 1; string version = 2; }

message StateStore {
  oneof kind {
    string path = 1;                         // WithStatePath(dir)
    bool host = 2;                           // the host implements the store, see StateTask
  }
}

message Mask {
  oneof kind {
    bytes aes256gcm_key = 1;                 // mask.EncryptAES256GCM
    bytes hmac_sha256_key = 2;               // mask.HashHMACSHA256
    bool redact = 3;                         // mask.Redact
    bool omit = 4;                           // mask.Omit
  }
}

enum EventDataLevel { EVENT_DATA_NONE = 0; EVENT_DATA_RAW = 1; EVENT_DATA_PROCESSED = 2; }
enum Step { STEP_INIT = 0; STEP_VALIDATE = 1; STEP_CREATE = 2; STEP_READ = 3;
            STEP_CHANGED = 4; STEP_UPDATE = 5; STEP_DESTROY = 6; }
```

`coreservice` turns a `LocalRegistry` into one `registry.Registry` of its own. `registry.Registry` is a public interface (`Name`, `Types`, `Plugins`) written for third parties, so no change to the root module is needed: config-only `DeclaredType`s become `registry.Type{Prototype: built}` where `built` is the `reflect.StructOf` type the schema v1 reader produces, and provided types are gathered into one `registry.Plugin` whose `Start` returns a `plugins.PluginHost` that routes every lifecycle call to a `ForeignProviderAdapter`. External plugins, directories and the GitHub registry map directly onto `registry.NewLocal` and `registry.NewGitHub`.

### Operations

```protobuf
message StartValidateRequest { repeated string paths = 1; }
message StartApplyRequest    { repeated string paths = 1; }
message StartDiffRequest     { repeated string paths = 1; bool reveal_sensitive = 2; }
message StartDestroyRequest  {}
message StartLoadRequest     {}
message StartOperationResponse { uint64 operation_id = 1; }

message CancelRequest  { uint64 operation_id = 1; }
message CancelResponse {}
```

An operation runs on a goroutine inside the core. It finishes with an `OperationFinished` task carrying the error, if any, and for a diff the result. The host learns nothing about the operation except through `Poll`, so the same code path serves an operation with no host providers and no events (one `Poll` returns `OperationFinished`) and one with both.

Why every operation is started rather than called synchronously: a synchronous `Apply` that drives host-language providers would need the host to be in two places at once, blocked in `xcl_call` and running its provider. Starting the operation and polling removes that, and makes the two transports identical above the byte layer.

### Polling

```protobuf
message PollRequest {
  // wait_ms is how long Poll may block waiting for a task. 0 returns at
  // once. A Poll that times out returns no task and no error.
  uint32 wait_ms = 1;
}

message PollResponse {
  oneof task {
    ProviderTask provider = 1;
    StateTask state = 2;
    Event event = 3;
    OperationFinished finished = 4;
  }
}

// ProviderTask is one lifecycle step the core needs the host's provider to
// run. The host answers with Complete, naming the task id.
message ProviderTask {
  uint64 task_id = 1;
  uint64 operation_id = 2;
  Step step = 3;
  string type = 4;
  string subtype = 5;
  string address = 6;                        // the entity's address, for logging and errors
  bytes entity = 7;                          // Validate, Create, Update, Destroy: the entity
  bytes old_entity = 8;                      // Read, Changed: the saved copy
  bytes new_entity = 9;                      // Read, Changed: the configured copy
  bool force = 10;                           // Destroy
  repeated PropertyChange changes = 11;      // Changed, Update
  repeated DependencyChange dependencies = 12; // Changed, Update
}

// StateTask is one call on the host's state store, when the configuration
// was created with StateStore.host.
message StateTask {
  uint64 task_id = 1;
  uint64 operation_id = 2;
  enum Kind { LOAD = 0; SAVE = 1; EXISTS = 2; CLEAR = 3; }
  Kind kind = 3;
  repeated bytes entities = 4;               // SAVE: each entity's JSON record, real sensitive values
}

message OperationFinished {
  uint64 operation_id = 1;
  Error error = 2;                           // unset on success
  bytes diff_json = 3;                       // StartDiff: json.Marshal of the diff.Diff
  repeated Token diff_tokens = 4;            // StartDiff: the change summary xcl.PrintDiff prints, labelled
}

message CompleteRequest {
  uint64 task_id = 1;
  oneof result {
    ProviderResult provider = 2;
    StateResult state = 3;
  }
}
message CompleteResponse {}

message ProviderResult {
  bytes entity = 1;                          // Create, Read, Update: the entity returned
  Change change = 2;                         // Changed
  Error error = 3;                           // the provider failed; message is the provider's
  bool not_found = 4;                        // Read: the real resource no longer exists
}

message StateResult {
  repeated bytes entities = 1;               // LOAD
  bool exists = 2;                           // EXISTS
  Error error = 3;
}
```

`PropertyChange`, `DependencyChange`, `PathStep`, `StepKind` and `Change` are the messages `plugins/plugin.proto` already defines, moved to a shared `xcl.types.v1` file both protos import, so a change list crosses to a host provider in exactly the form it crosses to an external plugin.

### Logging and state access from a provider

```protobuf
message LogRequest {
  uint64 task_id = 1;                        // 0 for a message written outside a task
  string level = 2;                          // debug, info, warn, error
  string message = 3;
  bytes details_json = 4;                    // a JSON object of key/value details
}
message LogResponse {}

message StateGetRequest  { uint64 task_id = 1; string key = 2; }
message StateGetResponse { bytes entity = 1; }
message StateFindRequest  { uint64 task_id = 1; string pattern = 2; }
message StateFindResponse { repeated bytes entities = 1; }
```

### Events

```protobuf
message Event {
  int64 time_unix_nano = 1;
  string source = 2;                         // "core" or the plugin's name
  string operation = 3;
  string phase = 4;
  string resource_type = 5;                  // "postgres.main"
  string resource_id = 6;                    // "resource.postgres.main"
  string file = 7;
  int64 duration_nano = 8;
  Error error = 9;
  bytes data = 10;                           // the entity, masked by the event masker
  bytes meta_json = 11;                      // Meta as a JSON object: level, message, details
}
```

This is `events.Event` field for field (`events/events.go`). `Meta` is `map[string]any`, so it crosses as JSON. `Event.Entity()` in Go decodes `Data` through the configuration's types; the host SDK does the equivalent by reading `meta.type` and `meta.subtype` out of the JSON and hydrating the registered class.

### Lookups

```protobuf
message EntitiesRequest  {}
message EntitiesResponse { repeated bytes entities = 1; }   // JSON, sensitive values as the marker

message FindRequest       { string address = 1; }
message EntityResponse    { bytes entity = 1; }
message FindByTypeRequest { repeated string path = 1; }     // FindByType / FindOne segments

message OutputsRequest  {}
message OutputsResponse { bytes outputs_json = 1; }          // {"output.name": value, ...}

// The four choices of Go's encoder, all off by default
message EncodeOptions {
  bool include_computed = 1;   // xcl.IncludeComputed()
  bool include_empty    = 2;   // xcl.IncludeEmpty()
  bool reveal_sensitive = 3;   // xcl.RevealSensitive()
  bool show_references  = 4;   // xcl.ShowReferences()
}

// field 2 was highlight, terminal colour applied by the engine: reserved by
// number and name, never reused
message EncodeSavedEntityRequest  { bytes entity = 1; reserved 2; reserved "highlight"; EncodeOptions options = 3; bool tokens = 4; }
message EncodeSavedEntityResponse { bytes text = 1; repeated Token tokens = 2; }   // tokens empty unless asked

message EncodeEntityRequest  { bytes entity = 1; reserved 2; reserved "highlight"; EncodeOptions options = 3; bool tokens = 4; }
message EncodeEntityResponse { bytes text = 1; repeated Token tokens = 2; }
```

`Find[T]`, `FindByType[T]`, `FindOne[T]` and `All[T]` are Go generics; their type parameter does two things, picks the address segments (`All`) and converts the entity. Over the wire the host passes the segments and does the conversion itself, so `All[T]` becomes `FindByType` with the segments the host derived from its `@xcl.entity("database", "postgres")` registration. Entities returned by lookups are serialised through `encoding/json`, so every sensitive value is the marker `"(sensitive)"`; a host never receives a secret from a lookup, which is what `query_sensitive.go` guarantees to Go callers. Provider tasks are the exception: they carry real values through `internal/wire`, as they do for Go providers.

Encoding an entity as configuration text is a lookup too. `EncodeEntity` reads the engine's own copy of the entity, so like the other lookups it is refused while an operation runs; `EncodeSavedEntity` reads none of the configuration's entities, so it is allowed during an operation, including from a host's event handling, and an empty, invalid or unrecognised record reports the same error then as outside one. `EncodeSavedEntity` renders a saved record, as state stores it and events carry it, through `Config.EncodeSavedEntity`. `EncodeEntity` renders an entity's record in the shape a lookup returns it, possibly edited by the host or built by hand, through `Config.EncodeEntityRecord`. Both take `EncodeOptions`, the four choices of Go's encoder, and an unset `options` is Go's default output, so a saved-record call that sends none renders as it always has. Because a lookup's record holds the marker for every sensitive field, set or not, `EncodeEntity` fills each sensitive value still holding the marker from the engine's own copy of the entity, found by `meta.id`, so the text is what Go writes for that entity: the marker for a set value, the configured value with `reveal_sensitive`, and nothing for one never set; an empty list or map, which the lookup left out of its JSON, is taken from the same copy. A value the host holds for real, such as an edit or a value in a record built by hand, is shown as it is, and a record whose id names nothing is not filled. A host builds the record from its own object; an SDK that hydrated the object from a lookup writes each field it has not changed exactly as the lookup returned it, so values the host language filled in, such as defaults, do not reach the text. Both calls go through the one rendering path Go's `EncodeEntity` uses, so the text is identical to Go's. With `tokens` set, either call also returns the text labelled by the core's lexer, `highlight.Tokenize`, whose texts joined are `text`; a caller that does not ask pays nothing. The engine applies no colour itself: the `highlight` field older callers set to ask for terminal colour is reserved, and such a request, which still parses because protobuf skips unknown fields, is answered with plain text.

### Labelled text

Resources, change summaries and error reports cross as labelled text: the plain text split into pieces, each tagged with what it is. Lexing and layout stay in the core; each language only formats the pieces, plain or styled as its users choose (see `printing-and-highlighting.md` for the formatters in each language).

```protobuf
// A value's category is kind / 100 * 100. Numbers are Go's highlight.Kind
// and never change; each name is TOKEN_KIND_ and the Go name in upper snake
// case.
enum TokenKind {
  TOKEN_KIND_TEXT = 0;
  TOKEN_KIND_BLOCK = 100;  TOKEN_KIND_BLOCK_TYPE = 101;  TOKEN_KIND_NESTED_BLOCK = 102;
  TOKEN_KIND_LABEL = 200;  TOKEN_KIND_TYPE_LABEL = 201;  TOKEN_KIND_NAME_LABEL = 202;
  TOKEN_KIND_ATTRIBUTE = 300;
  TOKEN_KIND_STRING = 400; TOKEN_KIND_STRING_ESCAPE = 401; TOKEN_KIND_HEREDOC = 402;
                           TOKEN_KIND_HEREDOC_MARKER = 403; TOKEN_KIND_HEREDOC_OPERATOR = 404;
  TOKEN_KIND_NUMBER = 500;
  TOKEN_KIND_CONSTANT = 600;
  TOKEN_KIND_FUNCTION = 700; TOKEN_KIND_BUILTIN_FUNCTION = 701;
  TOKEN_KIND_REFERENCE = 800; TOKEN_KIND_MEMBER = 801;
  TOKEN_KIND_OPERATOR = 900;
  TOKEN_KIND_PUNCTUATION = 1000; TOKEN_KIND_INTERPOLATION_BEGIN = 1001; TOKEN_KIND_INTERPOLATION_END = 1002;
                                 TOKEN_KIND_TEMPLATE_DIRECTIVE = 1003; TOKEN_KIND_ACCESSOR = 1004;
  TOKEN_KIND_COMMENT = 1100; TOKEN_KIND_LINE_COMMENT = 1101; TOKEN_KIND_HASH_COMMENT = 1102; TOKEN_KIND_BLOCK_COMMENT = 1103;
  TOKEN_KIND_DIFF = 1200; TOKEN_KIND_INSERTED = 1201; TOKEN_KIND_DELETED = 1202; TOKEN_KIND_CHANGED = 1203;
  TOKEN_KIND_PROBLEM = 1300; TOKEN_KIND_ERROR = 1301; TOKEN_KIND_WARNING = 1302;
  TOKEN_KIND_REPORT = 1400; TOKEN_KIND_SUMMARY = 1401; TOKEN_KIND_LOCATION = 1402; TOKEN_KIND_GUTTER = 1403; TOKEN_KIND_DETAIL = 1404;
}

message Token { TokenKind kind = 1; string text = 2; }   // the texts joined in order are the plain text

message ConvertThemeRequest  { bytes vscode_theme_json = 1; }   // JSON, comments and trailing commas allowed
message ConvertThemeResponse { map<int32, string> styles = 1; } // TokenKind number to a style such as "bold #ff8800"
```

Kinds are numbered in blocks of 100: the value ending in 00 is a category, and a value's category is its number divided by 100, times 100, in every language. proto3 enums are open, so a kind a binding built against an older protocol does not know still arrives as its number and falls into its category, and is styled as that category is; a theme that styles `TOKEN_KIND_COMMENT` styles every kind of comment. The numbers mirror Go's `highlight.Kind`, and a test in `coreservice` holds the enum to `highlight.Kinds()` in both directions, by number and name, so the two cannot drift. Labelled text arrives in three places: `Error.report`, the `tokens` of both encode calls when asked, and `OperationFinished.diff_tokens` for a finished diff, which describes the same diff as `diff_json` because the core derives both from the one value.

`ConvertTheme` turns a VS Code colour theme into a style string per kind, through the core's TextMate selector rules, as Go's `highlight.LoadVSCodeTheme` does. It takes handle 0, so a host converts a theme without creating a configuration. Only the kinds the theme styles appear in `styles`; every other kind keeps the language's default. A theme the core cannot use fails with `invalid_theme`, whose `source` and `reason` details say what was wrong, and an empty request with `invalid_argument`.

### Types

```protobuf
message DescribeTypesRequest  {}
message DescribeTypesResponse { repeated DescribedType types = 1; }
message DescribedType { string type = 1; string subtype = 2; bytes schema = 3; string source = 4; }
```

`DescribeTypes` loads the configuration's plugins and returns every type they provide as schema v1, including types whose plugin sent the legacy schema (the core converts). It is what code generation reads.

## Generating code from the protos

The `.proto` files are the API. Every language generates its message types from them; no language generates an RPC client or server for `CoreService`.

### The files

```
proto/
  xcl/types/v1/types.proto   PropertyChange, DependencyChange, PathStep, StepKind, Change
                             (moved out of plugin.proto so both protos below share them)
  xcl/core/v1/core.proto     CoreService: CreateConfig, StartApply, Poll, Complete, ...
plugins/plugin.proto         PluginService, HostCallbackService (unchanged; imports types.proto)
```

### What each language generates

| Where | From | Generator | Output | Used for |
|---|---|---|---|---|
| xcl (Go) | `core.proto`, `types.proto` | `protoc-gen-go` only | `coreservice/proto/*.pb.go` | decoding requests and encoding responses in `coreservice` |
| xcl (Go) | `plugin.proto` | `protoc-gen-go` + `protoc-gen-go-grpc` | `plugins/proto/` (as today) | gRPC with external plugins |
| xcl-python | `core.proto`, `types.proto` | `protoc --python_out --pyi_out` | `xcl/_core/*_pb2.py` + `.pyi` stubs | building requests and reading responses for `xcl_call` |
| xcl-python | `plugin.proto` | `grpc_tools.protoc` (adds `*_pb2_grpc.py`) | `xcl/plugin/` | `plugin.serve()`, a real gRPC server |
| xcl-node | `core.proto`, `types.proto` | `protoc-gen-es` (`@bufbuild/protobuf`) | `src/core/*_pb.ts` | as Python |
| xcl-node | `plugin.proto` | `protoc-gen-es` + `@grpc/grpc-js` | `src/plugin/` | Node plugin server |

`plugin.proto` gets gRPC code because external plugins are separate processes. `core.proto` gets message classes only; its `service CoreService` block is the machine-readable list of methods, read through the generated descriptors.

### Go: dispatching without gRPC

`coreservice` maps the `method` string to a handler through a hand-written table, and a test holds the table to the `.proto` through the generated service descriptor:

```go
// handlers maps each CoreService method to its implementation.
var handlers = map[string]handler{
    "CreateConfig": unary(func(s *Service, _ uint64, req *pb.CreateConfigRequest) (*pb.CreateConfigResponse, error) {
        return s.createConfig(req)
    }),
    "StartApply": unary((*configuration).startApply),
    "Poll":       unary((*configuration).poll),
    // ...
}

// unary adapts a typed method to bytes in, bytes out.
func unary[Req, Resp proto.Message](fn func(...) (Resp, error)) handler { ... }
```

```go
func TestEveryServiceMethodHasAHandler(t *testing.T) {
    methods := pb.File_xcl_core_v1_core_proto.Services().ByName("CoreService").Methods()
    for i := 0; i < methods.Len(); i++ {
        name := string(methods.Get(i).Name())
        _, ok := handlers[name]
        require.True(t, ok, "CoreService.%s has no handler", name)
    }
}
```

The request type of each method comes from the descriptor too (`methods.Get(i).Input()`), so `unary` decodes generically and returns status `-2` when the bytes do not parse.

### SDKs: a typed client, a byte transport, and a hand-written API

Each SDK has three layers, of which only the bottom one knows how the engine is reached:

```
xcl.Config.apply()          public API: dataclasses, poll loop, exceptions (hand-written)
        │
_Client.call(h, "StartApply", StartApplyRequest)
        │                   typed: encode request, decode response, map Error to exception
_SubprocessTransport.call(h, "StartApply", b"...")
        │                   bytes only: one frame out, one frame back
xcl-core  stdin / stdout
```

A second transport class, `_InProcessTransport`, speaks the C ABI through cffi instead; it has the same `call` signature and is used only from a source checkout that built the shared library.

The typed client is built from the generated descriptor at run time, so nothing beyond the `_pb2` modules is generated:

```python
from xcl._core import core_pb2

_service = core_pb2.DESCRIPTOR.services_by_name["CoreService"]

class _Client:
    def __init__(self, transport):
        self._transport = transport

    def call(self, handle: int, method: str, request):
        desc = _service.methods_by_name[method]           # KeyError is an SDK bug
        status, body = self._transport.call(handle, method, request.SerializeToString())
        if status == 1:
            raise _errors.from_proto(core_pb2.Error.FromString(body))
        if status < 0:
            raise _errors.InternalBindingError(method, status)
        return _message_class(desc.output_type).FromString(body)
```

The transport is the only C-specific code in the SDK:

```python
from cffi import FFI

ffi = FFI()
ffi.cdef("""
    uint32_t xcl_abi_version(void);
    int32_t  xcl_call(uint64_t handle, const char *method,
                      const uint8_t *req, size_t req_len,
                      uint8_t **resp, size_t *resp_len);
    void     xcl_free(uint8_t *resp);
    int32_t  xcl_release(uint64_t handle);
""")

class _InProcessTransport:
    def __init__(self, path):
        self._lib = ffi.dlopen(path)          # done lazily, on first use

    def call(self, handle: int, method: str, request: bytes) -> tuple[int, bytes]:
        resp = ffi.new("uint8_t **")
        resp_len = ffi.new("size_t *")
        status = self._lib.xcl_call(handle, method.encode(), request, len(request), resp, resp_len)
        try:
            body = ffi.buffer(resp[0], resp_len[0])[:] if resp_len[0] else b""
        finally:
            self._lib.xcl_free(resp[0])       # copied into Python, so free the C buffer at once
        return status, body
```

The split keeps process management, the GIL and fork detection in one place; lets SDK tests run against a fake transport with no engine at all; and keeps the in-process transport a second class with the same `call` signature. Node has the same three layers, with `child_process` in place of `subprocess`. The public API stays hand-written because it is where the idiomatic language lives; generated code stops at the message layer.

### Keeping the contract stable

Generation moves from the `protoc` line in the `Makefile` to `buf`:

- `buf.yaml` and `buf.gen.yaml` in each repository; `make protos` runs `buf generate`.
- `buf breaking --against '.git#branch=main'` in CI fails any change that renumbers or retypes a field, removes a method, or removes a field without reserving both its number and its name, enforcing the rule that codes, fields and methods are added and never renamed or reused. The FILE rules are used with `FIELD_NO_DELETE` swapped for `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`, since `FIELD_NO_DELETE` refuses even a fully reserved removal.
- `buf lint` keeps names consistent (`xcl.core.v1`, `FooRequest` and `FooResponse`).

### How the SDKs get the protos

The protos live in the xcl repository under `proto/`. Each SDK's release CI checks out `jumppad-labs/xcl` at the tag it bundles and runs `buf generate` against it; the generated files are committed in the SDK, as `plugins/proto/` is committed here. Core and SDK are built from the same tag, and `GetVersion` is checked on load, so they cannot drift.

## Host-language providers: the polling model

A provider written in Python must be called by the core at the right point of an apply, with the right data, and its answer fed back into the lifecycle. The core never calls into the host. Instead, the core enqueues a task and blocks the goroutine that needs the answer; the host, which is polling, runs the provider and posts the result; the goroutine wakes. This is the model Temporal's sdk-core uses between its Rust core and its Python, TypeScript and .NET SDKs.

### ForeignProviderAdapter

`coreservice` implements `plugins.ProviderAdapter` (`plugins/adapter.go`) once more, beside `TypedProviderAdapter[T]` and `GRPCResourceProviderAdapter`:

```go
// ForeignProviderAdapter runs a provider that lives in the host language.
// Each lifecycle method enqueues one task and blocks until the host
// completes it or ctx is cancelled.
type ForeignProviderAdapter struct {
    queue      *taskQueue
    typ        string
    subtype    string
    implements map[proto.Step]bool
}

func (a *ForeignProviderAdapter) Create(ctx context.Context, entityData []byte) ([]byte, error) {
    result, err := a.queue.run(ctx, &proto.ProviderTask{
        Step: proto.Step_STEP_CREATE, Type: a.typ, Subtype: a.subtype,
        Address: addressOf(entityData), Entity: entityData,
    })
    if err != nil {
        return nil, err           // ctx cancelled, or the host failed the task
    }
    return result.Entity, nil
}

func (a *ForeignProviderAdapter) Read(ctx context.Context, old, new []byte) ([]byte, error) {
    result, err := a.queue.run(ctx, &proto.ProviderTask{Step: proto.Step_STEP_READ, OldEntity: old, NewEntity: new, ...})
    if err != nil {
        return nil, err
    }
    if result.NotFound {
        return nil, plugins.ErrNotFound   // the parser creates the resource again
    }
    return result.Entity, nil
}

func (a *ForeignProviderAdapter) Changed(ctx context.Context, old, new []byte, changes []entity.PropertyChange, deps []entity.DependencyChange) (entity.Change, error) {
    if !a.implements[proto.Step_STEP_CHANGED] {
        return defaultChanged(old, new)   // the comparison plugins.DefaultChanged makes
    }
    ...
}
```

The parser (`internal/parser/lifecycle.go`) calls this adapter through `plugins.PluginHost` exactly as it calls the other two, so ordering, parallelism over unrelated resources, the decide-then-act sequence, saving state after a failure, sensitive masking in events and the per-call logger all stay where they are. `Read` returning `plugins.ErrNotFound` is how an external plugin's `not_found` flag already reaches the parser (`grpcPluginWrapper.Read` in `plugins/grpc_plugin_host.go`); the foreign adapter does the same from `ProviderResult.not_found`.

`plugins.DefaultChanged[T]` is generic, but its comparison works on `wire.Marshal` output with `meta`, `depends_on` and `disabled` removed (`plugins/changed.go`). That comparison is extracted into a non-generic `plugins.DefaultChangedJSON(old, new []byte) (entity.Change, error)` which both the generic type and the foreign adapter call; a small, additive change to the root module.

### The task queue

One queue per configuration handle. It is a channel of pending tasks plus a map of in-flight tasks by id:

```go
type taskQueue struct {
    pending  chan *pending            // bounded; the parser's parallelism bounds it in practice
    inflight sync.Map                 // task_id -> *pending
    next     atomic.Uint64
}

type pending struct {
    task   *proto.PollResponse        // what Poll hands the host
    done   chan completion            // what Complete delivers
    logger logger.Logger             // the call's logger, from plugins.Logger(ctx)
    cancel context.CancelFunc
}

func (q *taskQueue) run(ctx context.Context, t *proto.ProviderTask) (*proto.ProviderResult, error) {
    t.TaskId = q.next.Add(1)
    p := &pending{task: wrap(t), done: make(chan completion, 1), logger: plugins.Logger(ctx)}
    q.inflight.Store(t.TaskId, p)
    defer q.inflight.Delete(t.TaskId)

    select {
    case q.pending <- p:
    case <-ctx.Done():
        return nil, ctx.Err()
    }

    select {
    case c := <-p.done:
        return c.result, c.err
    case <-ctx.Done():
        return nil, ctx.Err()      // a late Complete for this id is accepted and discarded
    }
}
```

`Poll` takes from `pending`, waiting up to `wait_ms`; events and `OperationFinished` go through the same channel so the host sees everything in one ordered stream. `Complete` looks the id up in `inflight`; an id that is unknown, already completed or cancelled returns `invalid_argument` with `task_id` in details, and is otherwise harmless.

Task ids are unique within a handle for its lifetime, never reused. The `operation_id` on each task lets a host that runs operations back to back attribute late events correctly.

### Running the poll loop while Apply runs

The SDK hides the loop. For a synchronous Python `cfg.apply(paths)`:

1. `StartApply` returns `operation_id`.
2. Loop: `Poll(wait_ms=1000)` on the calling thread, with the GIL released during the call.
3. A `ProviderTask` is dispatched to the registered provider object; the result or exception becomes `Complete`.
4. A `StateTask` is dispatched to the host's state store object.
5. An `Event` is passed to the `on_event` callback.
6. `OperationFinished` ends the loop; its `error` is raised as the mapped exception.

Concurrency of provider tasks is the host's choice. xcl walks unrelated resources in parallel, so several `ProviderTask`s can be outstanding at once. A host that runs them one at a time on the polling thread is correct and loses only the parallelism; a host that hands each to a worker (thread pool, asyncio task) keeps it. The Python SDK runs tasks serially by default in the sync API and concurrently in the asyncio API; see the Python section.

### Timeouts and cancellation

Nothing times out by itself: a Go provider has no deadline either, and a task that waits an hour for a slow Python provider is correct. Two things end a wait early:

- `Cancel(operation_id)` cancels the operation's context. Every blocked `run` returns `ctx.Err()`, which the parser treats as a failed provider call: it saves the progress made, stops starting new calls and lets `Apply` return `context.Canceled`, surfaced as code `cancelled`. Tasks the host already took are left for the host to finish or drop; their `Complete` is accepted and discarded.
- `xcl_release(handle)` cancels any operation, waits for it to return (so state is saved), then frees everything.

The public `Config.Apply(paths ...string)` takes no context today; `run` in `config.go` creates its own and cancels it only when an event handler panics. `coreservice` needs a way in. The smallest addition to the root module is a context-taking form of each operation, `ApplyContext(ctx, paths...)`, `ValidateContext`, `DiffContext`, `DestroyContext`, `LoadContext`, with the existing methods calling them with `context.Background()`. This is additive and benefits Go users too.

A host that stops polling while an operation is running leaves the operation blocked on a task forever; in process that is the host's own hang, and a host that exits takes the core with it. The configuration's state was saved after the last completed step, as it is after any failed apply.

### Logging from a provider

A `ProviderTask` carries its `task_id`; the SDK gives the provider a `ctx.log` bound to it, and each call becomes `Log{task_id, level, message, details_json}`. `coreservice` writes it to the pending task's logger, the one the parser put in the call's context (`plugins.Logger(ctx)`), so the message becomes an event naming the resource, the step and the plugin exactly as a Go provider's `plugins.Logger(ctx).Info(...)` does. This mirrors `GRPCHostCallbackServer.loggerFor` and `callLoggers` in `plugins/grpc_calls.go`, which do the same for external plugins using the `xcl-call-id` metadata. A `Log` with `task_id` 0, or for a task that has returned, goes to the plugin-scoped logger the foreign plugin host was initialised with.

Detail values are a JSON object, so numbers and booleans keep their types; the gRPC callback path flattens them to strings and parses them back, which this design does not repeat.

### State access from a provider

`StateGet` and `StateFind` are the `plugins.State` interface (`plugins/state.go`) as calls from the host to the core. They are answered from the `State` the foreign plugin host was initialised with. Note that today both `registry.InProcess` and `registry.Executable` start their hosts with a nil `State` (`registry/registry.go`), and the gRPC callback server answers "state service not available" when it is nil; the foreign host follows whatever the core does here, and the messages exist so that nothing in the ABI changes when it is wired.

### The host as state store

`state.StateStore` (`state/state_store.go`) exchanges plain values: `Load() ([]any, error)`, `Save([]any) error`, `Exists() bool`, `Clear() error`, where each element is an entity's JSON record. A `ForeignStateStore` implements it by enqueuing a `StateTask` and blocking, exactly like the provider adapter; `Load` returns each `entities` element as a `json.RawMessage`, which the interface permits. `Save` is called by `Config.Apply` after the parser returns and by `Destroy` after each resource, both inside the operation goroutine, so the host is already polling. The records carry real sensitive values when no state mask is set, as they do for a Go store; `state_mask` in `CreateConfig` encrypts them before they cross.

### The host as event handler

`events` in `CreateConfig` installs an `xcl.EventHandler` in the core that pushes each event onto the same queue as an `Event` task, with no completion. `Config.run` drains events on the goroutine that called `Apply`, one at a time, blocking emitters only when the buffer (`WithEventBufferSize`) is full; the handler blocks when the host is slow to poll, so backpressure reaches providers exactly as it does in Go. Because `run` returns only after every event is delivered, `OperationFinished` is always the last task of an operation.

## Schema v1

### Why a new schema

A type a plugin provides reaches the host as a schema, because the host has no Go type to decode into. Today that schema (`internal/schema/serialize.go`) is a tree of `Attribute{Name, Type, Tags, Anonymous, Properties}` whose `Type` is a Go type string (`map[string]interface {}`, `[]*structs.Volume`, `types.Sensitive[string]`) and whose `Tags` is the raw struct tag (`xcl:"vars,optional" json:"vars,omitempty"`). `deserialize.go` parses the type strings back and rebuilds the struct with `reflect.StructOf`. It works for Go talking to Go and nothing else: a Python plugin cannot sensibly emit `types.ResourceBase`.

Schema v1 says what a block type is in terms any language has.

### The format

```json
{
  "schema_version": 1,
  "type": "database",
  "subtype": "postgres",
  "fields": [
    {"name": "location", "type": "string"},
    {"name": "port", "type": "number", "format": "int", "optional": true, "default": 5432},
    {"name": "password", "type": "string", "optional": true, "sensitive": true},
    {"name": "connection_string", "type": "string", "computed": true},
    {"name": "tags", "type": {"map": "string"}, "optional": true},
    {"name": "volume", "kind": "block", "repeated": true,
     "type": {"object": [
       {"name": "source", "type": "string"},
       {"name": "read_only", "type": "bool", "optional": true}
     ]}}
  ]
}
```

Field properties:

| Property | Values | Meaning |
|---|---|---|
| `name` | string | the name configuration uses; snake_case |
| `type` | `"string"`, `"number"`, `"bool"`, `"any"`, `{"list": T}`, `{"map": T}`, `{"object": [fields]}` | the value's type |
| `kind` | `"attribute"` (default), `"block"` | an attribute `name = ...` or a nested block `name { ... }` |
| `repeated` | bool | a block that may appear more than once; attributes use `list` |
| `optional` | bool | may be omitted |
| `sensitive` | bool | the value is a secret; see below |
| `computed` | bool | owned by the provider; implies optional |
| `default` | JSON value | the value when omitted; only on optional attributes |
| `format` | `"int"`, `"float"` | hint on a `number`; `int` by default when absent from a Go `int` field. Proposal, so that code generation can write `port: int` rather than `float`. JSON has one number type, so it changes nothing on the wire. |

`ResourceBase` (`depends_on`, `disabled`, `meta`) is never listed; every block type has it and the core adds it. The legacy schema listed it as an anonymous `types.ResourceBase` property, which only a Go reader could use.

### How the core builds a Go type from it

`internal/schema` gains a v1 reader beside the legacy one. `CreateInstanceFromSchema` sniffs the document: an object with `"schema_version": 1` takes the new path, anything else the legacy path, so a plugin built against today's xcl keeps loading. The v1 reader builds `reflect.StructField`s:

| Schema | Go field type | `xcl` tag | `json` tag |
|---|---|---|---|
| `string` | `string` | `name` (+`,optional`) | `name,omitempty` |
| `number` / `int` | `int` | | |
| `number` / `float` | `float64` | | |
| `bool` | `bool` | | |
| `any` | `any` | | |
| `{"list": T}` | `[]T` | | |
| `{"map": T}` | `map[string]T` | | |
| `{"object": [...]}`, kind attribute | nested anonymous struct | | |
| kind `block` | nested anonymous struct | `name,block` | |
| kind `block`, `repeated` | `[]struct` | `name,block` | |
| `sensitive` | `types.Sensitive[T]` | | |
| `computed` | as above | `name,optional,computed` | |
| `default` | as above | plus `default:"<text>"` | |

The field's Go name is the schema name in CamelCase (`connection_string` → `ConnectionString`); `reflect.StructOf` needs an exported name and nothing else reads it. The first field is always `types.ResourceBase`, anonymous, tagged as it is in `types/resource.go`. Defaults use the `default:"..."` tag because the parser already applies them with `creasty/defaults` (`internal/parser/callbacks.go`). The tag options written are exactly those `internal/xcl/tags` parses, so the parser, graph builder, decoder, computed-field carry-over and change detection run on the built type with no knowledge that it came from a schema.

`sensitive` is limited to the instantiations the core can build at run time: `types.Sensitive[string]`, `[int]`, `[int64]`, `[float64]`, `[bool]`, `[[]string]` and `[map[string]string]` (`schema.KnownTypes` in `internal/schema/known_types.go`). A schema marking any other shape sensitive fails to load with a clear error, as the legacy path does for an unknown `types.Sensitive[...]` string. Go cannot instantiate a generic at run time, so this is a core limit, not a schema one; the list can grow by adding to `KnownTypes`.

### Producers

`schema.GenerateSchemaFromInstance` becomes one producer of v1: it walks a Go struct as today, reads each field's `xcl` tag with `tags.Parse` and emits the v1 field. It recognises `types.Sensitive[T]` (as `isSensitiveType` does now) and emits `sensitive: true` with `T`'s schema type. The Python SDK is the second producer, from dataclass or Pydantic type hints; the Node SDK the third, from Zod or decorators.

### Where schema v1 travels

- In process, from `DeclaredType.schema` in `CreateConfig`.
- From an external plugin, in `GetTypesResponse.types[].schema` under plugin protocol v2 (under v1 the legacy form is sent, see below).
- To code generators, from `DescribeTypes`.
- In state? No. State records are entity JSON and name their type in `meta`; the schema is looked up from the registry, as now.

### Sensitive, computed and unknown values in the host

- Sensitive. A host provider receives real values in provider tasks, as a Go provider does, and the SDK wraps each field the schema marks sensitive in a `Sensitive` type whose `repr`/`str`/JSON form is `(sensitive)` and whose `reveal()` returns the value. Lookups return the marker string in place of the value; the SDK's `Sensitive` wrapper for a looked-up entity holds no value and `reveal()` returns `None`, matching `Sensitive[T].Reveal()` on a redacted value. Event data carries the configured event masker's envelope `{"xcl_masked": "redact", "value": "(sensitive)"}` (`mask/mask.go`), which the SDK recognises by the reserved `xcl_masked` key. Change lists carry real values for sensitive settings with `sensitive: true`, so a provider can act on them; the SDK masks them when printing.
- Computed. The core carries a computed field's saved value onto the configured copy before `Read` (`internal/parser/computed.go`); the host sees it filled in, as a Go provider does. A host that sets a non-computed field in `Read`'s result is doing the same wrong thing a Go provider would, and the core treats it the same.
- Unknown. A value only known once the apply has run appears in `Changed` tasks as a `PropertyChange` with `unknown: true` and no `after`, and in diffs as `Change.unknown`. It never appears in an entity payload. The SDK exposes it as the sentinel `xcl.UNKNOWN` on the change object.

### Code generation for plugin-provided types

A Python application using the Go Docker plugin wants `Container` as a typed class, not a dict. Every comparable tool generates these: Pulumi SDKs, Dagger's client codegen, Pkl's `pkl-gen-*`, KCL's `kcl import`. The generator is in the SDK, not the core: the core answers `DescribeTypes` with schema v1 and knows nothing about Python syntax. For Python:

```
python -m xcl generate --plugin ./bin/xcl-plugin-docker --out ./xcl_types/docker.py
python -m xcl generate --github jumppad-labs/xcl-plugin-docker@v1.2.0 --out ./xcl_types/docker.py
```

creates a `Config` with the plugin registered, calls `DescribeTypes`, and renders one `@xcl.entity(type, subtype) @dataclass class ...` per type with `Sensitive[T]`, `Computed[T]` and nested block classes. The repository has no `xcl` command-line program today; `xcl generate python` is the spelling for such a program when one exists, and the module form is what the SDK ships. Types the application defines itself need no generation: the SDK derives their schema at registration.

## The Python binding and SDK

Repository `xcl-python`, package `xcl` on PyPI. Version equals the xcl tag it bundles.

### Layout

```
xcl/
  __init__.py        public API: Config, LocalRegistry, GitHubRegistry, entity, Resource,
                     Provider, Plugin, Sensitive, Computed, UNKNOWN, errors, mask
  _transport/
    __init__.py      Transport protocol: call(handle, method, bytes) -> (status, bytes)
    subprocess.py    starts the bundled xcl-core, frames CoreService calls over its stdio
    inprocess.py     loads libxcl through cffi ABI mode (ctypes fallback); source builds only
  _client.py         typed client built from the CoreService descriptor
  _core/
    core_pb2.py      generated from coreservice.proto and the shared types proto
  _schema.py         dataclass / Pydantic -> schema v1; schema v1 -> hydration
  _poll.py           the poll loop, sync and asyncio variants
  _errors.py         code -> exception class
  plugin/            out-of-process plugin server: grpcio, PluginService, handshake
  aio.py             asyncio API
  generate.py        python -m xcl generate
  _bin/              xcl-core for this wheel's platform
  _lib/              libxcl.so / .dylib / .dll, present only in a source build that asked for it
```

### Starting the engine

The engine is started on first use, not at `import xcl`: `xcl-core --serve` from the package's own directory, with a pipe pair for its standard input and output and its standard error inherited, so Go runtime diagnostics and `GOTRACEBACK` output reach the user. The first exchange is `GetVersion`; the package refuses an engine whose version differs from its own, naming both. One engine serves every configuration in the host process; it is ended when the last configuration is closed or at interpreter exit, with `SIGTERM` (`TerminateProcess` on Windows) and a bounded wait. All of this lives in the transport class; nothing above it knows there is a process.

The wheel contains no compiled extension, so one wheel per platform serves every CPython and PyPy version (`py3-none-manylinux_2_28_x86_64`, `py3-none-musllinux_1_2_aarch64`, `py3-none-macosx_11_0_arm64`, `py3-none-win_amd64`, ...). The transport's calls release the GIL while they wait on the pipe, which is what lets `Poll` block without stalling other Python threads.

Fork is unremarkable on this route. A child forked after the parent used xcl inherits the parent's pipes but must not use them: the transport records the pid it started under, and a call from a different pid on a configuration created before the fork raises `xcl.ForkedProcessError`, naming the way out, which is to create the configuration after the fork. A new configuration in the child simply starts the child's own engine. There is no Go runtime in the host process, so gunicorn's `preload_app`, celery's prefork pool and `multiprocessing` with the fork start method all work with that one rule.

### Choosing the transport

The wheel decides, and every wheel carries `xcl/_bin/xcl-core`. A source build can additionally produce `xcl/_lib/libxcl.*` (`make libxcl`), and then `XCL_TRANSPORT=inprocess` selects the shared library; without the variable the engine program is used even when the library is present. When neither is present the first use raises `xcl.EngineNotFoundError` naming both paths. There is no probing of the machine at runtime and no silent fallback: which engine runs is a property of the installed package and the one variable, so it is the same on every machine the package is installed on, and the parity suite runs against each wheel exactly as a user would.

The in-process route exists so the suite can run both transports from one checkout, so the C ABI stays exercised, and so the SDK can switch its default back without an API change if the in-process route becomes viable again; see the decision section.

### API

```python
import xcl
from dataclasses import dataclass

@xcl.entity("database", "postgres")
@dataclass
class Postgres(xcl.Resource):
    location: str
    port: int = 5432
    password: xcl.Sensitive[str] | None = None
    connection_string: xcl.Computed[str] = None
    volume: list[Volume] = xcl.field(default_factory=list, block=True)

class PostgresProvider(xcl.Provider[Postgres]):
    def init(self, ctx: xcl.Context) -> None: ...
    def create(self, ctx: xcl.Context, db: Postgres) -> Postgres:
        ctx.log.info("creating database", location=db.location)
        db.connection_string = f"postgres://{db.location}:{db.port}"
        return db
    def read(self, ctx, old: Postgres, new: Postgres) -> Postgres:
        if not exists(old.connection_string):
            raise xcl.NotFound
        return new
    def update(self, ctx, db: Postgres, changes: list[xcl.PropertyChange], deps: list[xcl.DependencyChange]) -> Postgres: ...
    def destroy(self, ctx, db: Postgres, force: bool) -> None: ...
    # changed() and validate() are optional; the core's defaults apply

local = xcl.LocalRegistry()
local.register_type(Settings)                       # config-only, like RegisterType
local.register_provider(PostgresProvider())         # provided: Postgres from the generic parameter
local.register_external_plugin("./bin/xcl-plugin-docker")
local.register_plugin_directory("~/.xcl/plugins")

github = xcl.GitHubRegistry(trusted_keys=[PUBLIC_KEY])
github.register_plugin("jumppad-labs/xcl-plugin-docker", "v1.2.0")

cfg = xcl.Config(
    registries=[local, github],
    state_path="./.xcl",                            # or state_store=MyStore()
    state_mask=xcl.mask.aes256gcm(key),
    variables={"region": "eu-west-1"},
    on_event=lambda e: print(e.operation, e.phase, e.resource_id),
    event_data="processed",
)

cfg.validate("./infra")
plan = cfg.diff(["./infra"])                        # xcl.Diff, from diff_json
cfg.apply("./infra")
db = cfg.find(Postgres, "database.postgres.main")   # hydrated Postgres
dbs = cfg.find_by_type(Postgres, "database", "postgres")
all_dbs = cfg.all(Postgres)                         # segments from the @xcl.entity registration
one = cfg.find_one(Postgres, "database", "postgres")
outputs = cfg.outputs()                             # dict
entities = cfg.entities()                           # hydrated where the class is registered, xcl.Entity otherwise
cfg.load()
cfg.destroy()
cfg.close()                                         # xcl_release; Config is also a context manager
```

Mapping to the wire:

- `xcl.Config(...)` builds `CreateConfigRequest`. `LocalRegistry` and `GitHubRegistry` are plain builders of `Registry` messages; they hold no handle.
- `register_provider` reads `Postgres` from the `Provider[Postgres]` generic parameter, derives schema v1 from the dataclass, lists the lifecycle methods the class defines in `implements`, and marks the type `provided`.
- Schema derivation: `Optional[T]`, `T | None` or a default makes a field optional; a default becomes `default`; a dataclass-typed field or `list[Dataclass]` is a block (`repeated` for the list), or `xcl.field(block=True)` says so explicitly; `xcl.Sensitive[T]` sets `sensitive`; `xcl.Computed[T]` sets `computed` and `optional`; `Annotated[T, xcl.field(name="...", block=True)]` overrides. Pydantic `BaseModel` classes are read through their fields the same way. `int` is `number`/`int`, `float` is `number`/`float`, `dict[str, T]` is `map`, `list[T]` of a non-dataclass is `list`, `Any` is `any`.
- Hydration: an entity's JSON names its type in `meta.type` and `meta.subtype`; the class registered under that pair with `@xcl.entity` is instantiated from the JSON, nested blocks recursively. An entity with no registered class, such as a Go plugin's type without generated code, becomes `xcl.Entity`, a mapping with a `.meta`.
- Operations run the poll loop described above. `cfg.apply` blocks the calling thread; `Poll` releases the GIL.
- Exceptions: `xcl.Error(code, message, details, causes)` and one subclass per code (`xcl.NotFoundError`, `xcl.NotUniqueError`, `xcl.PluginLoadError`, `xcl.ConfigError`, ...). A provider that raises inside a task has its exception turned into `ProviderResult.error` with the exception's message, and the apply fails on that resource as it would in Go; `raise xcl.NotFound` from `read` becomes `not_found`.
- `on_event` receives `xcl.Event` with the fields of `events.Event`; `event.entity()` hydrates `data` when present.
- A custom store is any object with `load() -> list[bytes | dict]`, `save(list[bytes])`, `exists() -> bool`, `clear()`; it is called from `StateTask`s.

### Sync and asyncio

Proposal: one implementation of the poll loop with two faces.

- `xcl.Config` is synchronous. Provider tasks run on the polling thread, one at a time, in the order the core issues them. Correct and simple; a slow provider delays the others. `xcl.Config(..., max_workers=N)` runs tasks on a `ThreadPoolExecutor` instead, for providers known to be thread-safe.
- `xcl.aio.Config` is asyncio. `await cfg.apply(...)` runs `Poll` in a dedicated executor thread and dispatches each task as an asyncio task, so `async def create(...)` providers overlap naturally, which is what the core's parallel graph walk expects. Sync provider methods are run in the default executor.

Both are offered because the polling model costs the sync API nothing extra and most scripts are synchronous, while the asyncio API is the better fit for the model. Which is documented first is undecided.

### Out-of-process Python plugins

```python
plugin = xcl.Plugin(providers=[PostgresProvider()])
if __name__ == "__main__":
    plugin.serve()
```

`serve()` reads the host's callback address from the environment (protocol v2 below), starts a grpcio server implementing `PluginService`, writes the go-plugin handshake line `1|2|tcp|127.0.0.1:PORT|grpc` to stdout, and serves: `GetTypes` returns schema v1 for each provider's class; the lifecycle methods decode `entity_data` into the class, call the provider and encode the result; `ReadResponse.not_found` is set when `read` raises `xcl.NotFound`; `ctx.log` sends `HostCallbackService.Info/...` with the `xcl-call-id` metadata the host sent. The same `xcl.Plugin` object serves in process through `register_provider`, so a provider is written once and hosted either way. The plugin depends on `grpcio` and `protobuf`, not on libxcl; a Python plugin package is pure Python and needs no build matrix.

## Node and TypeScript

Repository `xcl-node`, package `@xcl/core`. It follows the same protocol with the same shapes, so only the bindings differ:

- Engine: `child_process.spawn` of the bundled `xcl-core` with piped stdio, frames read on the event loop, so nothing blocks it and `Poll` needs no special casing. The binary ships in `@xcl/core-linux-x64-gnu`, `@xcl/core-linux-arm64-musl`, `@xcl/core-darwin-arm64`, `@xcl/core-win32-x64` and so on as `optionalDependencies` with `os`, `cpu` and `libc` fields, the esbuild pattern (esbuild ships an executable this way); `@xcl/core` picks the one that installed. No FFI library and no native addon.
- Schemas from Zod schemas (`z.object({...})` with `xcl.sensitive()`, `xcl.computed()`, `xcl.block()` refinements) or from decorators on classes; property names are camelCase in TypeScript and converted to snake_case in schema v1 and entity JSON by one rule (`connectionString` ↔ `connection_string`), applied on both directions.
- API is Promise-based only: `await cfg.apply(paths)`; provider methods return Promises; tasks run concurrently on the event loop. No sync variant.
- Fork: Node's `cluster` and `child_process.fork` spawn, they do not `fork(2)`, so each worker starts its own engine.

Any further language, Java, .NET, Ruby, PHP, Rust, is the same recipe: a process launcher, a frame reader, the generated messages and a hand-written API. JARs and NuGet packages carry per-platform native files already (`runtimes/<rid>/native/` in NuGet; resources extracted at run time in a JAR), and an executable travels the same way as a library would; a JAR-based binding needs a configurable extraction directory for hosts whose temporary directory is mounted `noexec`, the same accommodation Netty and Testcontainers make.

## Out-of-process plugins in other languages and protocol v2

External plugins already use a protocol a non-Go program can speak: `PluginService` in `plugins/plugin.proto` carries entity data as JSON bytes and types as a schema blob, over gRPC, under HashiCorp go-plugin with `HandshakeConfig{ProtocolVersion: 1, MagicCookieKey: "HCLCONFIG_PLUGIN", MagicCookieValue: "xcl"}` (`plugins/grpc_plugin.go`). A non-Go plugin checks the cookie in its environment, listens on loopback, prints the handshake line and serves `PluginService` plus the gRPC health service named `plugin`, as go-plugin's non-Go guide describes.

The one obstacle is the callback direction. The plugin's logs and state lookups reach the host through `HostCallbackService`, which the host serves over go-plugin's `GRPCBroker` (`SetupHostCallbackService` in `plugins/grpc_plugin.go`, service id 1). The broker's stream protocol is internal to go-plugin and undocumented, which rules it out for a Python plugin.

Protocol v2 removes it:

- The host serves `HostCallbackService` itself on a loopback listener (a Unix socket on Linux and macOS, TCP on Windows) before starting the plugin, and passes its address in the plugin's environment as `XCL_HOST_ADDRESS`, beside the magic cookie go-plugin already passes. The plugin dials it with ordinary gRPC. An environment variable rather than a `Configure` RPC because a plugin's `Init` runs before the host's first RPC and may log.
- `GetTypesResponse.types[].schema` carries schema v1.
- Negotiation uses go-plugin's `VersionedPlugins`: the host offers `{1: v1 plugin, 2: v2 plugin}`, a Go plugin built against the new `plugins.Serve` offers both, and go-plugin picks the highest in common. Under 1 the plugin emits the legacy schema and uses the broker; under 2, schema v1 and `XCL_HOST_ADDRESS`. An existing plugin binary keeps working unchanged; a new plugin binary works with an older host. A non-Go plugin implements only 2.
- `LogRequest.call_id` and the `xcl-call-id` metadata are unchanged, so per-call logging works for every language.

Cross-language mixing follows: a Python application drives the Go Docker plugin through `external_plugins` or the GitHub registry; a Go application drives a Python plugin through `RegisterExternalPlugin` on a script or launcher that has a Python interpreter. libxcl depends on protocol v2 only for the Python SDK's `plugin.serve()`; the in-process path does not use it.

## Go c-shared hazards, and why the SDKs do not ship it

These are the reasons the in-process transport is built but not shipped. Each is real today; several would go away with upstream Go changes, which is what the way back in process waits on.

| Hazard | Source | Consequence in process | On the subprocess route |
|---|---|---|---|
| musl's loader refuses to `dlopen` a Go c-shared library (`initial-exec TLS resolves to dynamic definition`) | golang/go#54805, open; measured on Alpine with Go 1.25 and 1.27, `zig cc` and native builds | No Python, Java or .NET on Alpine, where all three commonly run. A library linked at build time loads, because static TLS is allocated for libraries present at startup; only loading after start is refused, which is what every binding does | Does not arise |
| Two Go runtimes in one process are unsupported: "We do not support multiple Go runtimes in a single process… no plans to ever make it work" | golang/go#18976 | A process that loads another Go-built extension beside libxcl is unsupported | Does not arise |
| No safe `fork()` after the runtime starts | golang/go#31860 | Lazy loading plus fork detection and `ForkedProcessError` naming `spawn` and post-fork hooks; a parent that used xcl cannot fork safely | A forked child starts its own engine |
| Signal handlers: c-shared installs handlers for synchronous signals and `SIGPIPE`, chaining to existing ones | os/signal package docs | Usually fine; the JVM and .NET use `SIGSEGV` for null checks and safepoints and .NET uses `SIGRTMIN` for GC suspension, and a handler fight ends in a crash with a Go frame in the middle | The engine owns its signals; `SIGINT` is ignored in the engine and the host cancels |
| Go pointers passed to C must stay valid and unmoved | cgo pointer rules | No Go pointer crosses: response buffers are `C.malloc`ed, requests are copied, objects are reached through handle integers | Does not arise |
| A crash in the core takes the host process down | nature of in-process code | As any native extension | The host gets `EngineExitedError` and survives |
| A C toolchain per target is needed to build the library | `-buildmode=c-shared` | `zig cc` for Linux and Windows x86-64, llvm-mingw for Windows arm64, Apple's runners for macOS, glibc symbol-version pinning (`manylinux_2_28`); the first Windows x86-64 build crashed on load, a toolchain interaction still to be understood | One `CGO_ENABLED=0` job cross-compiles every target |

## The transport: `xcl-core` over stdio

`cmd/xcl-core` is a static binary in the root module that serves `coreservice` over its standard input and output. It runs the same `coreservice.Service` as `libxcl`, so the API, the handles, the task queue, the error codes and every test of behaviour are shared; the binary is a main package of about the size of the cgo export file.

### Framing

Each direction carries length-prefixed frames, a 4-byte big-endian length followed by a protobuf message:

```protobuf
message Request  { uint64 id = 1; uint64 handle = 2; string method = 3; bytes payload = 4; }
message Response { uint64 id = 1; uint32 status = 2; bytes payload = 3; }
```

`id` is chosen by the host and echoed back. `handle`, `method`, `status` and `payload` mean exactly what the four C functions' arguments mean, so `Response.status` carries the same status codes and `payload` the same `CoreService` response or `Error` message. Responses may arrive out of order: the core runs each request on its own goroutine, so a `Poll` that blocks for seconds does not hold up a `Log` or a `Cancel` sent while it waits. The host's transport has one reader that matches responses to waiting callers by `id` (a thread in Python, the event loop in Node).

The frame messages live in `proto/xcl/core/v1/transport.proto` beside `core.proto`, generated like the rest; they are the only thing the transport adds to the protocol.

### Process lifecycle

- One `xcl-core` per host process, started on first use with `xcl-core --serve` and no other arguments, a pipe pair for stdin and stdout, and stderr inherited. `xcl-core --version` prints the version for diagnosis.
- The first exchange is `GetVersion`; the host refuses a core whose version differs from the package version, as the in-process transport does.
- The engine's standard output belongs to the protocol. The core never writes to it except through frames; logging goes to stderr; external plugins are the core's own subprocesses and their output is already captured by go-plugin.
- The engine exits when its standard input reaches end of file, so a host that dies without cleaning up (`SIGKILL`, a crash) takes the engine with it; this is go-plugin's rule for the same situation. On Windows the host also puts the engine in a job object with kill-on-close.
- `Config.close()` sends `Release` for its handle; the process is ended when the last handle is released or at interpreter exit, with `SIGTERM` (`TerminateProcess` on Windows) and a bounded wait.
- If the core exits or its pipe closes while calls are outstanding, every waiting caller gets `EngineExitedError` carrying the exit status; the next use starts a fresh core, and configurations created before the exit are gone, as they would be after an in-process crash, except that the host survives.
- Interrupts: a terminal's Ctrl-C reaches both processes in the group. The engine ignores `SIGINT` (and `CTRL_C_EVENT` on Windows) and leaves the decision to the host, whose `KeyboardInterrupt` arrives between `Poll` calls and sends `Cancel`, exactly as designed for the in-process route; state saved before the cancel is kept.
- Cancellation, timeouts, logging and state access are unchanged: they are `CoreService` messages and tasks, and the poll loop does not know which transport carries them.

### What it costs

A pipe round trip is about 20 to 50 µs against about 5 µs for a call into the library, and starting the engine takes 10 to 50 ms once per host process. For a tool that parses configuration and drives plugins that are themselves subprocesses this is noise: a configuration with a thousand function calls evaluated over three decode passes is a few thousand round trips, well under a second even at the slow end. Each platform package carries a 17 MB (6.5 MB compressed) static binary in place of a 19 MB library. The transport class is a few hundred lines per SDK, with the lifecycle rules above and their tests.

### Where it cannot run

iOS and store-sandboxed desktop apps, browsers, and edge isolates (Cloudflare Workers, Vercel Edge, Deno Deploy) forbid starting a process. They forbid loading a Go shared library too. iOS can link a static archive (`-buildmode=c-archive`, which Go supports there) carrying the same messages through the C ABI; browsers need the Wasm build. Both are further transports of the same protocol, built when a binding for such a platform is wanted. Serverless platforms that run a normal process (AWS Lambda, Azure Functions, Cloud Run) are fine.

## Shipping and packaging

### Artifacts per release

| Artifact | Targets | Size (measured, Go 1.27.1, linux/amd64) |
|---|---|---|
| `xcl-core` static binary | linux x86-64 and arm64, glibc and musl (one static binary serves both, released under both names so each wheel takes its own); macOS x86-64 and arm64; Windows x86-64 and arm64: 8, built with `CGO_ENABLED=0` from one runner | 17.1 MB stripped; 6.5 MB gzip -9 |
| `libxcl` c-shared | not released. Built and smoke-tested on Linux x86-64 in CI so the C ABI keeps working; a release of it is added when a consumer needs one | 26.5 MB; 19.0 MB stripped (`-trimpath -ldflags="-s -w"`); 6.9 MB gzip -9 |

go-getter and the AWS and GCS SDKs appear in `go.mod` as indirect dependencies but are not linked into these binaries.

Build tooling: `go build` with `CGO_ENABLED=0`, `GOOS` and `GOARCH` for all eight targets in one job on one Linux runner. No C compiler, no cross toolchain and no macOS runner are in the release path. (The shared library, when it is built, needs `zig cc` for Linux and Windows x86-64, llvm-mingw for Windows arm64 and Apple's runners for macOS; that matrix is kept in a manually triggered workflow, not in the release.)

### Distribution

Default route: the library travels inside each ecosystem's platform package.

- PyPI: one wheel per platform, tagged `py3-none-<platform>`, containing the matching `xcl-core`. Eight wheels at about 7 MB compressed is about 56 MB per release; against PyPI's default 10 GB project limit that is about 180 releases before asking for more, and the 100 MB per-file limit is not approached. Per user it is 19 MB in a virtualenv.
- npm: `@xcl/core` plus one `@xcl/core-<os>-<arch>[-<libc>]` package per target as `optionalDependencies`, about 19 MB in `node_modules`.
- GitHub release of `jumppad-labs/xcl`: the binaries as `xcl-core_<version>_<os>_<arch>[_musl].tar.gz` (`.zip` on Windows), listed in `xcl-core_<version>_checksums.txt` with `xcl-core_<version>_checksums.txt.sig`, the same asset contract the GitHub registry already verifies for plugins (`registry/github_assets.go`, `registry/github_verify.go`), signed with the same key.

Fallbacks, in order: download the matching archive from the release into `~/.xcl/core/<version>/`, verifying it with the registry's download, checksum and signature code; build from the sdist, which needs Go 1.25 or later and nothing else.

### Versioning and provenance

- The language package version equals the xcl tag it bundles. `xcl-lang 1.4.0` carries `xcl-core` built from `xcl v1.4.0`, and `GetVersion` is checked on start.
- The transport frames and status codes are versioned with the C ABI version (`xcl_abi_version()`, starting at 1): it changes only when a frame field, a function signature or a status code changes; adding a `CoreService` method or message field does not change it, since those are protobuf-compatible.
- PyPI Trusted Publishing from the release workflow; npm provenance; the GitHub release assets signed with the registry key; `xcl-core` for Windows is Authenticode-signed when a certificate is available, because a freshly started unknown executable is what antivirus heuristics flag.

## Design decisions

### A message API with a subprocess transport, over a shared library, Wasm or reimplementation

Decision: wrap the Go core once as a transport-free core service whose API is protobuf messages; ship it to every language as a static `xcl-core` program spoken to over stdio; keep the C shared library as a second, unshipped transport of the same messages.

This decision was first taken the other way round: the shared library as the shipped transport, the subprocess deferred. The first release attempt reversed it. The musl wheels could not load the library at all (golang/go#54805), the Windows x86-64 library crashed on load, and the C toolchain matrix was the long pole of the release; meanwhile the subprocess costs that had justified deferring it, a pipe round trip per call and process management in the SDK, measured as noise and as a few hundred lines. The reasons are set out in [Go c-shared hazards](#go-c-shared-hazards-and-why-the-sdks-do-not-ship-it) and [What it costs](#what-it-costs).

Alternatives:

- A WebAssembly core (`GOOS=wasip1`, or TinyGo, run under wasmtime-py or Node's WebAssembly). One artifact for every platform, sandboxed, usable in a browser. Rejected as the primary route: WASI preview 1 cannot spawn processes or open sockets, so external plugins and the GitHub registry stop working; Go's Wasm target is single-threaded, so the parallel graph walk is lost; TinyGo lacks the reflection the schema reader depends on (`reflect.StructOf`); esbuild reports its Wasm build "10x slower"; OPA's Wasm route left each SDK reimplementing builtins. Kept as a later validate-only build for a browser playground or VS Code web.
- The shared library as the shipped transport (the original decision). In process at function-call latency, with no process to manage. Rejected as the default for the hazards above, the first two of which have no fix on our side. Kept buildable and tested as the way back in process; see [Subprocess as the default transport](#subprocess-as-the-default-transport-and-the-way-back-in-process).
- Reimplementing xcl per language. Rejected: python-hcl2 reimplemented the syntax only and fragmented into forks, and HashiCorp compiled Go HCL to Wasm for `@cdktf/hcl2json` rather than port it. xcl is far more than syntax: graph, lifecycle, change detection, sensitive handling and state would each need a second implementation to keep in step.
- purego or dlopen for Go consumers, as KCL's Go SDK does. Unnecessary: Go users import the module.

Precedents for the subprocess route: Playwright, whose bindings for every language start one bundled driver and talk to it over pipes; Pkl's Go and Swift bindings, which run `pkl server` over stdio (their complaint was making users install the CLI, which bundling avoids); and go-plugin, which is how xcl's own plugins already run. Temporal ships in process because its core is Rust, which has none of the Go hazards. CUE's libcue and go-jsonnet's C bindings show the Go side of a shared library works, with go-jsonnet's "not battle-tested" note and missing Windows wheels marking the shipping work this design no longer has to do.

Sources: https://github.com/golang/go/issues/54805, https://github.com/golang/go/issues/18976, https://github.com/golang/go/issues/31860, https://esbuild.github.io/plugins/, https://pkl-lang.org/main/current/bindings-specification/index.html, https://playwright.dev/docs/languages, https://github.com/cue-lang/libcue, https://github.com/google/go-jsonnet.

### One message entry point over per-function or per-language bindings

Decision: `xcl_call(handle, method, bytes) -> bytes` plus free, ABI version and release. Nothing else is exported. Protobuf defines the messages; every language generates them, and each binding hand-writes only the four FFI declarations.

Alternatives:

1. One shared wide C API that each language wraps function by function: `xcl_config_new`, `xcl_register_type`, `xcl_apply`, `xcl_find`, ..., each with its own arguments, C structs and callback tables.
2. A separate Go shim per language: a CPython extension built from Go (go-jsonnet's route) and a separate Node addon built from Go.

| | This design | 1. Wide C API | 2. Per-language Go shims |
|---|---|---|---|
| Hand-written binding code per language | 4 C functions; messages generated | every exported function (about 20 operations plus callback tables) | a whole extension per language |
| How data types reach a language | generated from one `.proto` | hand-written structs or JSON shapes in every language | hand-written in each shim |
| Adding an operation or field | change the `.proto` and regenerate; old bindings keep working; ABI unchanged | a new export, an ABI version bump, every binding updated | every shim changed |
| Native artifacts per release | one `libxcl` per platform, shared by every language | one per platform | one per language per platform, and per Python version when the shim uses the CPython API (go-jsonnet ships per CPython version) |
| Where binding logic lives | once, in Go (`coreservice`), tested with `go test` | partly in each language's glue | duplicated in each shim |
| Two Go runtimes in one process | impossible: one library | as this design | possible when a process loads two shims, which Go does not support |
| Contract checking | `buf breaking` in CI | by hand | by hand |

With this design a new language costs four FFI declarations and a code-generation step, and the core can grow without touching a binding. With either alternative every language pays for every operation, and every change to the core is paid again in every language.

Costs accepted: encoding and decoding protobuf on every call, which is small beside parsing configuration or running a provider and would only show in a tight loop of thousands of tiny lookups, which xcl's API is not shaped for; requests are bytes rather than named C functions when debugging, offset by `coreservice` logging each call by method name; and each SDK depends on a protobuf runtime (`protobuf` on PyPI, `@bufbuild/protobuf` on npm). KCL (`kcl_service_call` with protobuf, about fifteen languages), Pkl (one message protocol) and Temporal (bridge calls carrying serialised messages, four languages) all converged on this shape.

### Protobuf over JSON per function

Decision: protobuf for the envelope and every structured message; JSON for entity data inside messages.

Alternative: JSON for everything. Rejected for the envelope: protobuf gives stable field numbers, enums, `bytes` fields that carry entity JSON without re-encoding, and generated types in every target language, and `plugins/plugin.proto` already defines the change-list messages the provider tasks need. JSON is kept where it already is the format: entity records, which the core produces and consumes as JSON today and which the host hydrates into native objects. Encoding the whole envelope in JSON would make the change list and event messages hand-maintained in each language.

### Polling over C callbacks for host-language providers

Decision: the core enqueues provider, state and event tasks; the host polls and completes them. Go never calls into the host.

Alternative: C function pointers (a provider vtable) the core calls from its goroutines. Rejected: a Go goroutine calling into Python must acquire the GIL on a thread Python has never seen, which works but is fragile and serialises everything; it does not fit asyncio at all, since the callback cannot await; in Node it needs thread-safe functions; and every callback needs a trampoline like go-jsonnet's native callback bridge. Polling needs none of that: the host runs providers wherever it likes (its own thread, an asyncio task), the GIL is held only by Python, and there are no function pointers in the ABI. The cost is one extra `xcl_call` per task, which is negligible beside what a provider does. Temporal's sdk-core inverted control the same way for the same reasons.

Source: https://raw.githubusercontent.com/temporalio/sdk-core/master/ARCHITECTURE.md.

### A nested module for the shared library and a pure-Go `coreservice`

Decision: `coreservice` is a plain package in the root module; `bindings/libxcl` is a separate module that imports it.

Alternative: put the export file in the root module behind a build tag. Rejected: a package importing `"C"` has no buildable files under `CGO_ENABLED=0`, so `go build ./...` and `go vet ./...` in the root would fail without a C toolchain, and a build tag that users must know about is a trap. Alternative: put all binding logic in the cgo module. Rejected: it could only be tested with a C compiler and a foreign caller, and the engine program would have to duplicate it. With the split, every behaviour is tested by `go test` against `Service.Call`, both transports are thin, and the root module's CI guard holds.

### Bundled platform packages over download or system install

Decision: each wheel and npm platform package carries its engine program; download from the signed release and build from source are fallbacks.

Alternatives: download on first use (small packages, but installs break offline and behind proxies, and a download at import time is a supply-chain surface); a system package the user installs (Pkl's early route, which pushed installation onto users). Bundling costs about 7 MB compressed per platform package and 19 MB on disk, within PyPI's 100 MB per file and workable against the 10 GB project limit (about 180 releases before a limit increase), and matches how esbuild (26 platform packages), SWC and Temporal (7 abi3 wheels of 14 to 15 MB) ship.

Source: https://docs.pypi.org/project-management/storage-limits/.

### Subprocess as the default transport, and the way back in process

Decision: every SDK ships `xcl-core` and uses it on every platform. The shared library is built and smoke-tested in CI, selectable from a source build with `XCL_TRANSPORT=inprocess`, and not released.

Alternatives considered on the way here: subprocess for musl only, in process elsewhere (two shipped transports to test in every SDK, and the Windows x86-64 and toolchain problems stay); drop musl (rejected by the project: Alpine is where Python services run); `LD_PRELOAD` the library (an environment variable outside the package, nothing for a wheel to do); both engines in every wheel with runtime probing (doubles every wheel, and the engine that runs would depend on the machine rather than on the package).

The way back. The in-process route becomes attractive again when the Go toolchain removes the hazards that are not ours to fix: a dynamic TLS model in the linker so `dlopen` works on musl (golang/go#54805; the arm64 change landed in 2025, x86-64 is pending), and a cross-compiled Windows x86-64 library that loads. When both hold, the switch is a packaging change: the wheel carries the library, `_InProcessTransport` becomes the default, and the API, the client, the protocol and the tests above the transport are untouched, because the message API was kept transport-free for exactly this. Until then the SDKs stay on the subprocess, and the C ABI is kept honest by CI rather than by users.

### Schema v1 over the Go-string schema

Decision: a language-neutral schema with `name`, `type`, `optional`, `kind`, `repeated`, `sensitive`, `computed`, `default` and a `format` hint; the legacy schema still read.

Alternative: keep the Go-string schema and teach each SDK to emit `[]*pkg.Type` strings and raw struct tags. Rejected: the strings encode Go package paths and generic instantiations a Python program has no reason to know, the raw tag is Go syntax, and `types.ResourceBase` as an anonymous property is meaningless elsewhere. The core already rebuilds plugin types by reflection from a schema, so the only change is what the schema says; the parser, decoder and change detection are untouched. Reading both formats keeps every existing plugin loading.

### Codegen for plugin types plus runtime derivation for application types

Decision: types a Go plugin provides are turned into typed classes by `python -m xcl generate`; types the application defines are derived at registration from the native class.

Alternatives: generate everything (an extra step for the application's own types, which it already has as classes); derive everything at run time (a Go plugin's types would only ever be dicts, with no IDE support). Every comparable tool with a cross-language story generates client types for things defined elsewhere (Pulumi, Dagger, Pkl, KCL) and lets the application write its own natively. The generator lives in the SDK because the core should know nothing about Python syntax; `DescribeTypes` gives it schema v1.

Source: https://www.kcl-lang.io/docs/reference/xlang-api/overview.

## Phasing

1. Schema v1. The reader in `internal/schema` accepting both formats; `GenerateSchemaFromInstance` emitting v1; the catalog using it. In-process only; the wire format of protocol 1 unchanged. No binding yet.
2. Plugin protocol v2. `VersionedPlugins`, the host callback listener and `XCL_HOST_ADDRESS`, schema v1 in `GetTypes` under 2, protocol 1 kept.
3. Python plugin SDK, out of process. `xcl.Plugin.serve()` over grpcio against protocol v2. This proves schema derivation and hydration with no shared library.
4. `CoreService` and the `buf` setup, `coreservice`, `bindings/libxcl`, the Python binding and the wheels. The context-taking operation methods in the root module. (Done, with the library as the shipped transport; the first release attempt failed on musl and Windows x86-64.)
4b. `cmd/xcl-core`, the transport frames, the subprocess transport as the Python package's default, every wheel carrying the program, the release matrix reduced to the cgo-disabled job, the library kept as a CI-tested source-build option, and the parity suite run over both transports.
5. Node and TypeScript binding and SDK.
6. Later, when needed: a validate-only Wasm build.

## Open questions

- Python plugin distribution through the GitHub registry: a frozen single-file binary that the registry runs like any plugin, or a `runtime` field in the release assets with a managed interpreter environment. Out-of-process Python plugins work from a path either way; distribution is decided later.
- Sync or asyncio first in the Python API. The design proposes both over one poll loop, with the synchronous `xcl.Config` running tasks serially by default. Proposal only.
- Host-language HCL functions. They would follow the task pattern (`FunctionTask`, completed with the result), with the core building the cty function from a declared signature as `internal/functions` builds one from a Go function today. Not designed here.
- Whether the C ABI version is independent of the core version. The design assumes an ABI version that changes only with the four functions and the status codes, and a core version that changes every release.
- The `format` hint on `number` is a proposal, so generated code can say `int`.
- Schema v1 does not model the `key` tag option (which pairs elements of a list of blocks for change detection) or the `body` kind that `internal/xcl/tags` parses. Whether `key` needs a schema property is undecided.
- `MACOSX_DEPLOYMENT_TARGET` and the exact toolchain versions for the shared library's manual workflow are proposals.
- When to move back in process: once golang/go#54805 is fixed for x86-64 and the Windows x86-64 library loads, whether the gain (function-call latency, no process) is worth re-entering the C toolchain matrix. Decided then, with the measurements of the day.
- Whether `libxcl` should be released at all before a consumer asks for it.
