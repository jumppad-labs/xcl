---
created_date: "2026-10-11"
document_status: draft
specs:
    - 20261011130709-aa3eb933-python-support
---

# libxcl: xcl for other languages

xcl is a Go library. An application imports `github.com/jumppad-labs/xcl`, registers its Go types and plugins, and calls `Apply`, `Validate`, `Diff`, `Destroy` and `Load` on a `Config`. libxcl makes the same engine available to Python and Node without reimplementing any of it: the Go core is compiled once as a C shared library, every operation is one protobuf message passed through one C function, and providers written in the host language are driven by the core through a task queue the host polls.

The protobuf is not an RPC interface. Nothing listens on a socket and no second process runs: a binding loads `libxcl` into its own process through FFI and calls a C function. Protobuf defines what travels through that function, the method names and the request and response layouts, so the C ABI stays four functions while the API grows in the `.proto` files. Because the API is messages rather than C functions, the same messages could later be carried over stdio by a separate `xcl-core` process; that transport is deferred, see [Deferred: the out-of-process transport](#deferred-the-out-of-process-transport).

## Goals and non-goals

Goals:

- A Python program can do everything a Go program does with xcl: build a configuration, register types, run every operation, read entities, outputs, events and diffs, use the GitHub registry and external plugins, and write providers in Python that the core calls during an apply.
- The parser, dependency graph, lifecycle, change detection, sensitive handling, state and events stay in Go and run unchanged. No language reimplements any of them.
- One API definition. The `CoreService` protobuf is the only description of what the core does for a host; the C ABI carries it, and every language generates its message types from it.
- Pure Go users are unaffected. Importing the root module pulls in no cgo, no subprocess and no shared library.
- Plugins written in one language work with applications written in another.

Non-goals:

- A browser or Wasm build. A validate-only Wasm build is a later option, not part of this design.
- A C API for C programs. The C ABI exists to be called by a language binding, not written against by hand.
- Supporting two Go runtimes in one process. See the hazards section.
- An out-of-process transport in the first release. The design keeps it possible; see [Deferred: the out-of-process transport](#deferred-the-out-of-process-transport).

## The guarantee for Go users

The root module `github.com/jumppad-labs/xcl` is unchanged by this work. Nothing in it imports `"C"`, nothing in it starts a process, and `go build ./...` and `go test ./...` need no C toolchain. The Go CI job already builds the root with `CGO_ENABLED=0` for darwin, linux and windows (`.github/workflows/go.yml`); that build stays as the guard, and the `coreservice` package added by this design is built by it too, so cgo cannot leak into the library without failing CI. External Go plugins keep the protocol they have, and gain protocol v2 by rebuilding.

## Architecture

### Layers

| Layer | Path | cgo | Role |
|---|---|---|---|
| Library | `github.com/jumppad-labs/xcl` (root module) | no | Unchanged. Go users import it directly. |
| Core service | `coreservice/` (package in the root module) | no | Implements `CoreService` on the public xcl API: handles, the message dispatcher, the foreign provider, state store and event adapters, the task queue, error mapping. Tested with plain `go test`. |
| Shared library | `bindings/libxcl/` (its own `go.mod`) | yes | `package main`, `//export xcl_call` and three more exports. A few hundred lines that move bytes between C and `coreservice`. |
| Language SDKs | `xcl-python`, `xcl-node` (own repositories) | no | Load libxcl, speak `CoreService`, present a native API, derive schemas from native types, run the poll loop. |
| Subprocess (deferred) | `cmd/xcl-core/` (in the root module) | no | Not built in the first release. Would serve the same `coreservice` over stdio. |

Dependencies point one way: SDK → transport → `coreservice` → `xcl`. The root module never imports `coreservice`, and `coreservice` imports only the public packages of the root module (`xcl`, `registry`, `plugins`, `state`, `events`, `diff`, `mask`, `types`, `entity`, `errors`, `logger`). Where `coreservice` needs something internal, that something is made public in the root module first.

### Why the shared library is a nested module

A Go package that imports `"C"` has no buildable files when `CGO_ENABLED=0`. If `bindings/libxcl` sat inside the root module, `go build ./...` and `go vet ./...` on a machine without a C compiler, or in the `CGO_ENABLED=0` CI job, would fail for everyone, including users who only want the library. Giving `bindings/libxcl` its own `go.mod` keeps it out of the root module's `./...`. During development its `go.mod` carries `replace github.com/jumppad-labs/xcl => ../..`; both modules are tagged together on release, with the nested module requiring the root at the same tag. The examples under `example/` already follow this pattern, each with its own `go.mod`, listed in the `Makefile` and built as separate CI jobs.

If `cmd/xcl-core` is built later it needs no cgo, so it would sit in the root module and be built by the `CGO_ENABLED=0` job like everything else.

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

The shared library is thin: it turns C bytes into `Call` and back. Every test of behaviour is a Go test against `Service.Call`, which needs neither a C compiler nor Python. `coreservice` must stay free of any transport: that is what keeps a later out-of-process transport a thin wrapper rather than a second implementation.

## The C ABI

Four exported functions. Nothing else is exported.

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
}
```

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
| `operation_in_progress` | a lookup was made while an operation was running on the handle |
| `invalid_argument` | a request that parsed but makes no sense, e.g. no paths |
| `internal` | anything not matched above |

The code list is part of the ABI contract: a code is never renamed or reused, only added. Bindings map codes to native exceptions (`xcl.NotFoundError`, `xcl.PluginLoadError`, ...) with an `xcl.Error` base class holding `code`, `details` and `causes`, so an unknown code still raises something catchable.

### Threading

`xcl_call` may be called from any thread, concurrently, with any handles. Each call runs on a goroutine the cgo runtime provides; nothing is pinned to the calling thread. Within one handle:

- At most one operation (`StartValidate`, `StartApply`, `StartDiff`, `StartDestroy`, `StartLoad`) runs at a time; starting a second returns `operation_in_progress`.
- `Poll`, `Complete`, `Log`, `StateGet`, `StateFind` and `Cancel` are allowed while an operation runs. They are the host's side of the operation.
- Lookups (`Find*`, `Entities`, `Outputs`, `EncodeSavedEntity`) are refused with `operation_in_progress` while an operation runs, because `Config.Apply` and `Config.Destroy` replace the entity slice when they finish and the root module does not promise concurrent reads during an operation.

The binding is told to release the GIL (Python) or use an asynchronous call (Node) around `xcl_call`, since `Poll` blocks for up to its wait time.

## The CoreService

The service is defined once, in `coreservice/coreservice.proto`, package `xcl.core.v1`. Entity data crosses as JSON bytes, exactly as it already does between the parser and a `plugins.ProviderAdapter` (`[]byte` in, `[]byte` out), so the core's existing serialisation (`internal/wire`) is used unchanged and no message mirrors an entity's fields. Protobuf gives the envelope: method names, stable field numbers, enums, and generated encoders in every target language.

Method names are the `method` string of `xcl_call`. The `service` block is documentation of the mapping; nothing generates a gRPC server from it for the in-process transport.

```protobuf
syntax = "proto3";
package xcl.core.v1;
option go_package = "github.com/jumppad-labs/xcl/coreservice/proto";

service CoreService {
  // Library
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);

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
  rpc EncodeSavedEntity(EncodeSavedEntityRequest) returns (EncodeSavedEntityResponse);

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

Why every operation is started rather than called synchronously: a synchronous `Apply` that drives host-language providers would need the host to be in two places at once, blocked in `xcl_call` and running its provider. Starting the operation and polling removes that, and would make a later out-of-process transport identical.

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

message EncodeSavedEntityRequest  { bytes entity = 1; bool highlight = 2; }
message EncodeSavedEntityResponse { bytes text = 1; }
```

`Find[T]`, `FindByType[T]`, `FindOne[T]` and `All[T]` are Go generics; their type parameter does two things, picks the address segments (`All`) and converts the entity. Over the wire the host passes the segments and does the conversion itself, so `All[T]` becomes `FindByType` with the segments the host derived from its `@xcl.entity("database", "postgres")` registration. Entities returned by lookups are serialised through `encoding/json`, so every sensitive value is the marker `"(sensitive)"`; a host never receives a secret from a lookup, which is what `query_sensitive.go` guarantees to Go callers. Provider tasks are the exception: they carry real values through `internal/wire`, as they do for Go providers.

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

Each SDK has three layers, of which only the bottom one touches the C library:

```
xcl.Config.apply()          public API: dataclasses, poll loop, exceptions (hand-written)
        │
_Client.call(h, "StartApply", StartApplyRequest)
        │                   typed: encode request, decode response, map Error to exception
_InProcessTransport.call(h, "StartApply", b"...")
        │                   bytes only: the FFI calls
libxcl  xcl_call / xcl_free
```

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

The split keeps memory ownership, the GIL and fork detection in one place; lets SDK tests run against a fake transport with no libxcl; and keeps the deferred out-of-process transport a second class with the same `call` signature. Node has the same three layers, with koffi in place of cffi. The public API stays hand-written because it is where the idiomatic language lives; generated code stops at the message layer.

### Keeping the contract stable

Generation moves from the `protoc` line in the `Makefile` to `buf`:

- `buf.yaml` and `buf.gen.yaml` in each repository; `make protos` runs `buf generate`.
- `buf breaking --against '.git#branch=main'` in CI fails any change that renumbers, retypes or removes a field or method, enforcing the rule that codes, fields and methods are added and never renamed or reused.
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
    inprocess.py     loads libxcl through cffi ABI mode (ctypes fallback), lazy, fork-aware
  _client.py         typed client built from the CoreService descriptor
  _core/
    core_pb2.py      generated from coreservice.proto and the shared types proto
  _schema.py         dataclass / Pydantic -> schema v1; schema v1 -> hydration
  _poll.py           the poll loop, sync and asyncio variants
  _errors.py         code -> exception class
  plugin/            out-of-process plugin server: grpcio, PluginService, handshake
  aio.py             asyncio API
  generate.py        python -m xcl generate
  _lib/              libxcl.so / .dylib / .dll for this wheel's platform
```

### Loading the library

cffi in ABI mode (`ffi.dlopen`) with the four-function `cdef` above, so the wheel contains no compiled extension and one wheel per platform serves every CPython and PyPy version (`py3-none-manylinux_2_28_x86_64`, `py3-none-macosx_11_0_arm64`, `py3-none-win_amd64`, ...). ctypes is the fallback when cffi is not installed; both release the GIL for the duration of a foreign call, which is what lets `Poll` block without stalling other Python threads.

The library is loaded on first use, not at `import xcl`. Loading a Go c-shared library starts the Go runtime and its threads, and a process that imports `xcl` then forks (gunicorn, celery prefork, `multiprocessing` with the fork start method) would fork a live Go runtime, which Go does not support (golang/go#31860). Lazy loading means a parent that imports `xcl` but never uses it before forking is safe. For a parent that did use it:

- `os.register_at_fork(after_in_child=...)` marks the child, and the transport also compares `os.getpid()` with the pid it loaded under.
- Any call in a marked child raises `xcl.ForkedProcessError` with the two ways out: use the `spawn` start method, or create the configuration after the fork (gunicorn's `post_fork` hook, celery's `worker_process_init` signal) so each child loads its own runtime.

Temporal's Python SDK documents the same limit for its Rust core ("Clients do not work across forks"); this design refuses rather than failing somewhere inside the runtime.

On load the binding calls `xcl_abi_version()` and refuses to continue when it differs from the version the binding was generated against, then `GetVersion` and checks the core version equals the package version.

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

Repository `xcl-node`, package `@xcl/core`. It follows the same ABI and protocol with the same shapes, so only the bindings differ:

- Loading: koffi in ABI mode (no compile step, `koffi.load` of the bundled library and `koffi.func` for the four exports, with `.async` for `Poll` so the event loop is never blocked), or a prebuilt Node-API addon if koffi proves unsuitable. The library ships in `@xcl/core-linux-x64-gnu`, `@xcl/core-linux-arm64-musl`, `@xcl/core-darwin-arm64`, `@xcl/core-win32-x64` and so on as `optionalDependencies` with `os`, `cpu` and `libc` fields, the esbuild and SWC pattern; `@xcl/core` picks the one that installed.
- Schemas from Zod schemas (`z.object({...})` with `xcl.sensitive()`, `xcl.computed()`, `xcl.block()` refinements) or from decorators on classes; property names are camelCase in TypeScript and converted to snake_case in schema v1 and entity JSON by one rule (`connectionString` ↔ `connection_string`), applied on both directions.
- API is Promise-based only: `await cfg.apply(paths)`; provider methods return Promises; tasks run concurrently on the event loop. No sync variant.
- Fork: Node's `cluster` and `child_process.fork` spawn, they do not `fork(2)`, so the hazard does not arise.

## Out-of-process plugins in other languages and protocol v2

External plugins already use a protocol a non-Go program can speak: `PluginService` in `plugins/plugin.proto` carries entity data as JSON bytes and types as a schema blob, over gRPC, under HashiCorp go-plugin with `HandshakeConfig{ProtocolVersion: 1, MagicCookieKey: "HCLCONFIG_PLUGIN", MagicCookieValue: "xcl"}` (`plugins/grpc_plugin.go`). A non-Go plugin checks the cookie in its environment, listens on loopback, prints the handshake line and serves `PluginService` plus the gRPC health service named `plugin`, as go-plugin's non-Go guide describes.

The one obstacle is the callback direction. The plugin's logs and state lookups reach the host through `HostCallbackService`, which the host serves over go-plugin's `GRPCBroker` (`SetupHostCallbackService` in `plugins/grpc_plugin.go`, service id 1). The broker's stream protocol is internal to go-plugin and undocumented, which rules it out for a Python plugin.

Protocol v2 removes it:

- The host serves `HostCallbackService` itself on a loopback listener (a Unix socket on Linux and macOS, TCP on Windows) before starting the plugin, and passes its address in the plugin's environment as `XCL_HOST_ADDRESS`, beside the magic cookie go-plugin already passes. The plugin dials it with ordinary gRPC. An environment variable rather than a `Configure` RPC because a plugin's `Init` runs before the host's first RPC and may log.
- `GetTypesResponse.types[].schema` carries schema v1.
- Negotiation uses go-plugin's `VersionedPlugins`: the host offers `{1: v1 plugin, 2: v2 plugin}`, a Go plugin built against the new `plugins.Serve` offers both, and go-plugin picks the highest in common. Under 1 the plugin emits the legacy schema and uses the broker; under 2, schema v1 and `XCL_HOST_ADDRESS`. An existing plugin binary keeps working unchanged; a new plugin binary works with an older host. A non-Go plugin implements only 2.
- `LogRequest.call_id` and the `xcl-call-id` metadata are unchanged, so per-call logging works for every language.

Cross-language mixing follows: a Python application drives the Go Docker plugin through `external_plugins` or the GitHub registry; a Go application drives a Python plugin through `RegisterExternalPlugin` on a script or launcher that has a Python interpreter. libxcl depends on protocol v2 only for the Python SDK's `plugin.serve()`; the in-process path does not use it.

## Go c-shared hazards and mitigations

| Hazard | Source | Mitigation |
|---|---|---|
| Two Go runtimes in one process are unsupported: "We do not support multiple Go runtimes in a single process… no plans to ever make it work" | golang/go#18976 | One Go library per process: libxcl is bundled only by the `xcl` package, never by a plugin package, so two xcl-using packages share one copy. A process that loads another Go-built extension beside libxcl is documented as unsupported. |
| No safe `fork()` after the runtime starts | golang/go#31860 | Lazy load, so a parent that never used xcl forks safely; fork detection with `os.register_at_fork` and a pid check; `ForkedProcessError` naming `spawn` and creating the configuration after the fork. |
| Signal handlers: c-shared installs handlers for synchronous signals and `SIGPIPE`, chaining to existing ones | os/signal package docs | The core never calls `signal.Notify`, so `SIGINT` stays with the host; Python's `KeyboardInterrupt` arrives between `Poll` calls and the SDK sends `Cancel`. A test presses Ctrl-C during an apply and checks state was saved. |
| Go pointers passed to C must stay valid and unmoved | cgo pointer rules | No Go pointer crosses: response buffers are `C.malloc`ed, requests are copied, objects are reached through `cgo.Handle` integers. |
| A crash in the core takes the host process down | nature of in-process code | Same as any native extension; `GOTRACEBACK` is honoured for diagnosis. The deferred out-of-process transport would isolate it. |
| A C toolchain is needed to build the library | `-buildmode=c-shared` | Only `bindings/libxcl` needs it, and only in the release matrix; nothing in the root module does. |
| glibc symbol versions from the build machine leak into the binary | measured: a build on Arch needs `GLIBC_2.34` | Release builds target `manylinux_2_28` with `zig cc -target x86_64-linux-gnu.2.28` or in a manylinux container. |

## Deferred: the out-of-process transport

The first release ships the in-process transport only. A static `xcl-core` binary serving `coreservice` over stdio (length-prefixed `Frame{id, handle, method, request}` and `Frame{id, status, response}`, answered out of order so a long `Poll` does not block a `Log`) is designed for but not built.

What it would cover, and how much that matters:

| Case | How common | Without the transport |
|---|---|---|
| A process that loads xcl, then forks (gunicorn `preload_app`, celery prefork, `multiprocessing` fork) | The only common case | Lazy loading covers a parent that never used xcl. A parent that did gets `ForkedProcessError` naming `spawn` and the post-fork hooks. |
| Another Go-built extension in the same process | Rare; few Python packages are built from Go | Documented as unsupported. |
| Platforms outside the eight targets, or hosts that forbid native libraries | Rare | Unsupported until asked for. |
| Isolating a crash in the core | Wanted occasionally | As any native extension. |

What it would cost: a second transport in every SDK (framing, multiplexing, child-process management, and tests for both paths), roughly doubling each wheel by shipping `xcl-core` beside `libxcl` (about 6.5 MB more compressed per platform), and a further binary to build, sign and release. Temporal, the closest precedent for an in-process core driven by polling, ships no out-of-process fallback and documents its fork limit instead.

What keeps it cheap to add later: `coreservice` holds no transport code, the whole API is protobuf messages, and the SDKs already put every FFI call behind a `call(handle, method, bytes)` transport. Adding it is `cmd/xcl-core` in the root module and one transport class per SDK; the API, the client and everything above them are unchanged. It is built when a reported case needs it.

## Shipping and packaging

### Artifacts per release

| Artifact | Targets | Size (measured, Go 1.27.1, linux/amd64) |
|---|---|---|
| `libxcl` c-shared | linux x86-64 and arm64, glibc (`manylinux_2_28`) and musl; macOS x86-64 and arm64; Windows x86-64 and arm64: 8 | 26.5 MB; 19.0 MB stripped (`-trimpath -ldflags="-s -w"`); 6.9 MB gzip -9 |

For comparison, a static `xcl-core` (deferred) measured 17.1 MB stripped and 6.5 MB gzip.

go-getter and the AWS and GCS SDKs appear in `go.mod` as indirect dependencies but are not linked into these binaries.

Build tooling: `zig cc` cross-compiles the Linux and Windows x86-64 c-shared targets from one Linux runner (`-target x86_64-linux-gnu.2.28`, `x86_64-linux-musl`, `x86_64-windows-gnu`); Windows arm64 uses llvm-mingw; macOS builds run on GitHub's macOS runners with `MACOSX_DEPLOYMENT_TARGET=11.0` (proposal).

### Distribution

Default route: the library travels inside each ecosystem's platform package.

- PyPI: one wheel per platform, tagged `py3-none-<platform>`, containing the matching `libxcl`. Eight wheels at about 7 MB compressed is about 56 MB per release; against PyPI's default 10 GB project limit that is about 180 releases before asking for more, and the 100 MB per-file limit is not approached. Per user it is 19 MB in a virtualenv.
- npm: `@xcl/core` plus one `@xcl/core-<os>-<arch>[-<libc>]` package per target as `optionalDependencies`, about 19 MB in `node_modules`.
- GitHub release of `jumppad-labs/xcl`: the raw libraries as `libxcl_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) with `libxcl_<version>_checksums.txt` and `libxcl_<version>_checksums.txt.sig`, the same asset contract the GitHub registry already verifies for plugins (`registry/github_assets.go`, `registry/github_verify.go`), signed with the same key.

Fallbacks, in order: download the matching archive from the release into `~/.xcl/core/<version>/`, verifying it with the registry's download, checksum and signature code; build from the sdist, which needs Go 1.25 or later and a C compiler.

### Versioning and provenance

- The language package version equals the xcl tag it bundles. `xcl-python 1.4.0` carries `libxcl` built from `xcl v1.4.0`, and `GetVersion` is checked on load.
- `xcl_abi_version()` returns the C ABI version, starting at 1. It changes only when a function signature or status code changes; adding a `CoreService` method or message field does not change it, since those are protobuf-compatible. Whether it stays independent of the core version is open; the design assumes it does.
- PyPI Trusted Publishing from the release workflow; npm provenance; the GitHub release assets signed with the registry key.

## Design decisions

### libxcl over Wasm, subprocess-only or reimplementation

Decision: compile the Go core as a C shared library and bind it; design for, but defer, an out-of-process transport.

Alternatives:

- A WebAssembly core (`GOOS=wasip1`, or TinyGo, run under wasmtime-py or Node's WebAssembly). One artifact for every platform, sandboxed, usable in a browser. Rejected as the primary route: WASI preview 1 cannot spawn processes or open sockets, so external plugins and the GitHub registry stop working; Go's Wasm target is single-threaded, so the parallel graph walk is lost; TinyGo lacks the reflection the schema reader depends on (`reflect.StructOf`); esbuild reports its Wasm build "10x slower"; OPA's Wasm route left each SDK reimplementing builtins. Kept as a later validate-only build for a browser playground or VS Code web.
- Subprocess only (an `xcl-core` process, no library). No cgo, crash isolation, no C toolchain, six targets. Rejected as the only route because every provider call, state call and event becomes an IPC round trip, process management is the SDK's problem, and spawning is forbidden in some environments. Pkl went subprocess-first, made users install the CLI themselves and is now adding libpkl with the same message protocol; jsii and CDKTF required Node on every machine and CDKTF was archived on 10 December 2025. Kept possible as a later second transport of the same messages; see [Defer the out-of-process transport](#defer-the-out-of-process-transport).
- Reimplementing xcl per language. Rejected: python-hcl2 reimplemented the syntax only and fragmented into forks, and HashiCorp compiled Go HCL to Wasm for `@cdktf/hcl2json` rather than port it. xcl is far more than syntax: graph, lifecycle, change detection, sensitive handling and state would each need a second implementation to keep in step.
- purego or dlopen for Go consumers, as KCL's Go SDK does. Unnecessary: Go users import the module.

Why libxcl: the engine runs in process at Go speed, with its parallelism, plugins and registry intact; one artifact per platform rather than one per language and platform; and the measured artifact (19 MB stripped, 7 MB compressed) fits every package registry's limits. KCL and Temporal ship the same way. CUE's libcue (Go c-shared) and go-jsonnet's C bindings show the Go side works, with go-jsonnet's "not battle-tested" note and missing Windows wheels marking exactly the shipping work this design budgets for.

Sources: https://github.com/golang/go/issues/18976, https://github.com/golang/go/issues/31860, https://esbuild.github.io/plugins/, https://pkl-lang.org/main/current/bindings-specification/index.html, https://github.com/cue-lang/libcue, https://github.com/google/go-jsonnet.

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

Alternative: put the export file in the root module behind a build tag. Rejected: a package importing `"C"` has no buildable files under `CGO_ENABLED=0`, so `go build ./...` and `go vet ./...` in the root would fail without a C toolchain, and a build tag that users must know about is a trap. Alternative: put all binding logic in the cgo module. Rejected: it could only be tested with a C compiler and a foreign caller, and a later out-of-process transport would have to duplicate it. With the split, every behaviour is tested by `go test` against `Service.Call`, the cgo file is small enough to read in one sitting, and the root module's CI guard holds.

### Bundled platform packages over download or system install

Decision: each wheel and npm platform package carries its library; download from the signed release and build from source are fallbacks.

Alternatives: download on first use (small packages, but installs break offline and behind proxies, and a download at import time is a supply-chain surface); a system package the user installs (Pkl's early route, which pushed installation onto users). Bundling costs about 7 MB compressed per platform package and 19 MB on disk, within PyPI's 100 MB per file and workable against the 10 GB project limit (about 180 releases before a limit increase), and matches how esbuild (26 platform packages), SWC and Temporal (7 abi3 wheels of 14 to 15 MB) ship.

Source: https://docs.pypi.org/project-management/storage-limits/.

### Defer the out-of-process transport

Decision: ship the in-process transport only; build `xcl-core` when a reported case needs it.

Alternative: ship `xcl-core` as a fallback from the first release. Rejected for now: the only common case it covers, a process that uses xcl and then forks, is already handled by lazy loading and a clear `ForkedProcessError`; the other cases are rare; and it would double the wheel size and add a second transport to build and test in every SDK. Temporal ships an in-process core with polling and no fallback. Nothing in the design closes the door: the API is messages, `coreservice` is transport-free and every SDK already isolates FFI behind a transport class. See [Deferred: the out-of-process transport](#deferred-the-out-of-process-transport).

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
4. `CoreService` and the `buf` setup, `coreservice`, `bindings/libxcl`, the Python binding and the wheels. The context-taking operation methods in the root module. The eight-target matrix with Trusted Publishing is set up first, since shipping is the long pole.
5. Node and TypeScript binding and SDK.
6. Later, when needed: the `xcl-core` out-of-process transport; a validate-only Wasm build.

## Open questions

- Python plugin distribution through the GitHub registry: a frozen single-file binary that the registry runs like any plugin, or a `runtime` field in the release assets with a managed interpreter environment. Out-of-process Python plugins work from a path either way; distribution is decided later.
- Sync or asyncio first in the Python API. The design proposes both over one poll loop, with the synchronous `xcl.Config` running tasks serially by default. Proposal only.
- Host-language HCL functions. They would follow the task pattern (`FunctionTask`, completed with the result), with the core building the cty function from a declared signature as `internal/functions` builds one from a Go function today. Not designed here.
- Whether the C ABI version is independent of the core version. The design assumes an ABI version that changes only with the four functions and the status codes, and a core version that changes every release.
- The `format` hint on `number` is a proposal, so generated code can say `int`.
- Schema v1 does not model the `key` tag option (which pairs elements of a list of blocks for change detection) or the `body` kind that `internal/xcl/tags` parses. Whether `key` needs a schema property is undecided.
- `MACOSX_DEPLOYMENT_TARGET` and the exact musl and Windows toolchain versions are proposals for the release matrix.
