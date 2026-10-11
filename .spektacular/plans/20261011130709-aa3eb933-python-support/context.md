---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Context: 20261011130709-aa3eb933-python-support

## Current State Analysis

- **No binding layer exists.** xcl is a Go library only. There is no `coreservice`, no `proto/` directory, no cgo anywhere, and no release workflow (`.github/workflows/` holds only `go.yml` and `examples.yml`). Tags `v0.1.0`…`v0.2.1` were pushed by hand. Language-surface ships `v0.3.0`.
- **Operations take no context.** `config.go:337,371,438,489` and `config_diff.go:34` all go through `run` (`config.go:547`), which creates `context.Background()` (`:551`) or a cancellable child of it (`:566`). The cancellable child is cancelled only when an event handler panics.
- **Typed lookups only.** `FindByType`/`FindOne` (`query.go:180,190`) require a Go `T`. The shared rules live in the private `entitiesOf` (`:212`), `typeable` (`:290`) and `oneOf` (`:273`). `Entities()`, `FindResource()` and `Outputs()` (`config.go:262,267,309`) are already untyped.
- **Only a Go-string schema.** `internal/schema/serialize.go:13` and `deserialize.go:14` exchange Go type strings and raw tags. Sensitive generics are limited to `KnownTypes()` (`known_types.go:22`).
- **Registries.** `registry.Registry` is `Name/Types/Plugins` (`registry/registry.go:26-40`), and language-surface adds `Functions()`. `registry.Local` has `RegisterType`, `RegisterExternalPlugin`, `RegisterPluginDirectory` and `PluginPattern` (`registry/local.go:45-155`). The GitHub registry has every option the design's `GitHubRegistry` message needs.
- **Options.** Every `CreateConfigRequest` field has a matching `ConfigOption` (`options.go:22-178`). Variables reach the parser as strings (`config.go:518`).
- **Protobuf** generation is `protoc` through `make protos`, for `plugins/plugin.proto` only.
- **xcl-python** is registered at `/home/nicj/code/github.com/jumppad-labs/xcl-python`, which holds only the Spektacular footprint. It has no GitHub repository yet.

## Per-Task Technical Notes

### Requirement → repo and files

| Requirement | Repo | Main files |
|---|---|---|
| Install with pip, no toolchain; Python 3.10+ | xcl-python (wheels, CI), xcl (libxcl build and release) | `xcl-python:pyproject.toml`, `xcl-python:.github/workflows/{ci,release}.yml`, `bindings/libxcl/`, `.github/workflows/libxcl-release.yml` |
| Entity types as Python classes; annotations map to configuration | xcl (schema v1 reader), xcl-python (derivation, hydration) | `internal/schema/v1.go`, `registry/schema.go`, `xcl-python:xcl/_schema.py`, `xcl-python:xcl/_entity.py` |
| Python functions callable from configuration | xcl (coreservice host functions), xcl-python | `coreservice/functions.go`, `xcl-python:xcl/_functions.py`, `xcl-python:xcl/_poll.py` |
| Go plugins usable from Python | xcl (CreateConfig registries), xcl-python | `coreservice/config.go`, `xcl-python:xcl/registry.py` |
| Every operation available; variables; entities; outputs and diffs | xcl (root context ops, untyped lookups, coreservice), xcl-python | `config.go`, `config_diff.go`, `query.go`, `coreservice/operations.go`, `coreservice/lookups.go`, `xcl-python:xcl/config.py` |
| State location and custom stores | xcl (ForeignStateStore), xcl-python | `coreservice/state_store.go`, `xcl-python:xcl/_poll.py` |
| Events delivered to Python | xcl, xcl-python | `coreservice/events.go`, `xcl-python:xcl/events.py` |
| Errors as Python exceptions | xcl (error mapper), xcl-python | `coreservice/errors.go`, `xcl-python:xcl/_errors.py` |
| Sync API, async API, cancellation | xcl (Cancel, context ops), xcl-python | `coreservice/queue.go`, `xcl-python:xcl/config.py`, `xcl-python:xcl/aio.py` |
| Safe across fork; engine version checked | xcl-python, xcl (GetVersion) | `xcl-python:xcl/_transport/inprocess.py`, `coreservice/version.go` |
| Releases kept in step | xcl, xcl-python | `.github/workflows/libxcl-release.yml`, `xcl-python:.github/workflows/release.yml` |
| Package documentation | xcl-python | `xcl-python:README.md`, `xcl-python:PARITY.md` |

### Task: Operations take a context and stop when it is cancelled

- `config.go:337` (Validate), `:371` (Apply), `:438` (Destroy), `:489` (Load) — each body moves into `ValidateContext(ctx, paths...)` and the other context forms. The old methods become `return c.ValidateContext(context.Background(), paths...)` and so on. Doc comments say that cancelling stops new provider calls and saves progress, and that the returned error wraps `context.Canceled`.
- `config_diff.go:34` — add `DiffContext(ctx, paths, options...)`. `Diff` wraps it.
- `config.go:547-610` (`run`) — take `ctx context.Context` as a parameter.
  - With no handler, call `work(ctx, nil)`.
  - With a handler, derive `context.WithCancel(ctx)` instead of from `context.Background()`.
  - Keep the handler-panic shutdown as is.
  - `withPlugins` (`:622`) is unchanged.
- Check that the parser's apply returns `ctx.Err()`, wrapped, when cancelled between vertices (`internal/parser` walk). If Apply only stops starting calls and returns nil, wrap `context.Cause(ctx)` into the returned error in `ApplyContext` when `ctx.Err() != nil`, so the caller always sees `context.Canceled`.
- Language-surface's `c.parserOptions(emit)` is the shared builder. Don't add a second one.
- New root-package test `config_context_test.go`, using `parser.TestPlugin`-style providers from `config_delivery_test.go:70` (`setupDeliveryConfig`):
  - `TestApplyContextCancelledStopsNewProviderCalls`: a provider blocks on a channel, the test cancels and then releases it, and asserts that no further Create starts;
  - `TestApplyContextCancelledSavesProgress`: state from a real apply;
  - `TestApplyContextReturnsContextCanceled`;
  - `TestApplyContextWithCancelledContextCallsNoProvider`;
  - `TestDestroyContextCancelledKeepsRemainingResources`;
  - `TestDiffContextCancelledReturnsContextCanceled`;
  - `TestLoadContextCancelledReturnsContextCanceled`.
**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: single agent, sequential.

### Task: Entities can be looked up by type without a Go type

- `query.go:212-250` (`entitiesOf`) — split out `func (c *Config) entitiesByType(path []string) ([]any, error)`. It runs the `len(path) == 0` check (`NotTypeableError{Use: "All"}`), `typeable` (`:290`), and the meta type/subtype scan, without `addressableType` or `asType`. `entitiesOf` calls it and then converts each result.
- New `query_untyped.go`:
  - `func (c *Config) FindEntitiesByType(path ...string) ([]any, error)` returns `entitiesByType`;
  - `func (c *Config) FindOneEntity(path ...string) (any, error)` uses `oneOf` (`:273`) and returns `&xclerrors.NotFoundError{Address: strings.Join(path, ".")}` when nothing matches.
  - Doc comments say these are for hosts without a Go type, and that they return entities as stored.
- New `query_untyped_test.go`:
  - `TestFindEntitiesByTypeMatchesFindByType`;
  - `TestFindEntitiesByTypeUnknownTypeReturnsUnknownTypeError`;
  - `TestFindEntitiesByTypeWithoutSubtypeReturnsNotTypeableError`;
  - `TestFindOneEntityReturnsTheEntity`;
  - `TestFindOneEntityNoneReturnsNotFound`;
  - `TestFindOneEntitySeveralReturnsNotUniqueWithCount`;
  - `TestFindEntitiesByTypeReturnsPluginEntities`.

  Use the fixtures from `query_setup_test.go`.
**Complexity**: Low
**Token estimate**: ~15k
**Agent strategy**: single agent.

### Task: Block types can be declared from a language-neutral schema

- New `internal/schema/v1.go` — `type FieldV1 struct{Name, Type json.RawMessage, Kind string, Repeated, Optional, Sensitive, Computed bool, Default json.RawMessage, Format, Label string}`, `type SchemaV1 struct{SchemaVersion int; Type, Subtype string; Fields []FieldV1}`, and `func BuildTypeV1(schema []byte) (reflect.Type, *SchemaV1, error)`.
  - The first field is anonymous `types.ResourceBase` with the tag from `types/resource.go`.
  - Each field's Go name is its CamelCase name.
  - Tags follow the design's table: `xcl:"name[,optional][,computed]"` or `name,block`, `json:"name,omitempty"`, and `default:"<text>"` from `Default`.
  - `number` with `format: float` is `float64`; otherwise it is `int`.
  - `sensitive` wraps the type in the matching `types.Sensitive[T]` from `KnownTypes()` (`internal/schema/known_types.go:22`). Any other shape is an error naming the field.
  - `{"map": {"object": …}}` with `kind: block` is `map[string]struct{…}` tagged `name,block`.
  - `label` adds `Name string \`xcl:"<label>,label" json:"<label>,omitempty"\`` as the element's first field. This needs language-surface's optional-label decode.
  - Unknown `type` or `kind` values are errors.
- `internal/schema/deserialize.go:14` (`CreateInstanceFromSchema`) — before the legacy unmarshal, peek for `"schema_version": 1` and route to `BuildTypeV1`, returning `reflect.New(t).Interface()`. The legacy path is untouched.
- New `registry/schema.go` — `func TypeFromSchema(schema []byte) (Type, error)` calls `schema.BuildTypeV1` and returns `Type{Type, Subtype, Prototype: reflect.New(t).Interface()}`. Language-surface's `catalog.DeclareType` runs `gohcl.ValidateSchemaType` on it at `NewConfig`.
- New tests:
  - `internal/schema/v1_test.go`, one function each:
    - `TestBuildTypeV1StringAttribute`, `...OptionalWithDefault`, `...SensitiveString`, `...SensitiveUnsupportedShapeFails`, `...ComputedIsOptionalComputed`, `...NestedBlock`, `...RepeatedBlock`, `...NamedBlockMap`, `...LabelAddsLabelField`, `...UnknownTypeFails`, `...FloatFormat`;
    - `TestCreateInstanceFromSchemaRoutesV1`;
    - `TestCreateInstanceFromSchemaKeepsLegacy`.
  - Root package `config_schema_types_test.go`: `TestSchemaTypeDecodesAndIsFound` (the `app "web"` criterion), `TestSchemaTypeRequiredFieldMissingFailsValidation`, `TestSchemaTypeOptionalFieldTakesDefault`, `TestSchemaTypeSensitiveFieldMaskedInStateAndEvents`, `TestSchemaTypeComputedFieldCannotBeSet`, `TestSchemaTypeVolumesListInOrder`, `TestSchemaTypeListenersMapByName`.
- Knowledge `gotchas/meta-field-golden-schema.md`: the legacy golden schema is unaffected. Don't touch `serialize.go`.
**Complexity**: High
**Token estimate**: ~55k
**Agent strategy**: parallel analysis (reader vs root acceptance fixtures), sequential integration.

### Task: The core service answers configuration and lookup calls

- New `proto/xcl/core/v1/core.proto`. It contains the design's messages, limited as in the plan's Data Structures:
  - `reserved 1` in `PollResponse`, `reserved 2` in `CompleteRequest`, and `reserved 4, 5` in `DeclaredType`;
  - `DeclaredFunction` and `LocalRegistry.functions = 5`;
  - `FunctionTask` (`PollResponse.function = 5`) and `FunctionResult` (`CompleteRequest.function = 4`).

  `option go_package = "github.com/jumppad-labs/xcl/coreservice/proto;corepb"`.
- New `buf.yaml` and `buf.gen.yaml` at the repo root (module `proto/`, `protoc-gen-go` to `coreservice/proto`). `Makefile`: add `core-protos: buf generate` and `buf-lint`. The `protos` target for `plugins/plugin.proto` is unchanged.
- New `coreservice/service.go`: `Service`, `New()`, `Call(handle uint64, method string, request []byte) ([]byte, Status)`, `Release(handle) Status`, and `type Status int32` with the constants `StatusOK=0`, `StatusError=1`, `StatusUnknownMethod=-1`, `StatusBadRequest=-2`, `StatusUnknownHandle=-3`, `StatusHandleMismatch=-4`. Handles use `runtime/cgo.Handle`, which is pure Go and needs no cgo. A `handlers` map is built with a generic `unary` that decodes the request type from the descriptor (`corepb.File_xcl_core_v1_core_proto.Services().ByName("CoreService")`).
- New `coreservice/version.go`: `var Version = "dev"` (set with `-ldflags -X`) and `const ABIVersion = 1`. The `GetVersion` handler returns both.
- New `coreservice/config.go`, `createConfig(req)`:
  - It builds `[]xcl.ConfigOption`.
  - For each `Registry.local`, a `hostRegistry` wraps `registry.NewLocal(registry.PluginPattern(p))` when the pattern is set.
    - Each `DeclaredType` goes through `registry.TypeFromSchema` and then `local.RegisterType(t.Prototype, typeName...)`. Check how `RegisterType` takes names (`registry/local.go:90`). If it reads them from the prototype, the `hostRegistry` returns its own `Types()` built from `TypeFromSchema`.
    - External plugins go through `RegisterExternalPlugin`, and directories through `RegisterPluginDirectory`.
    - `Name()` returns `"local"`. `Plugins` delegates. `Functions()` is filled by the host-functions task, and is nil here.
  - A `Registry.github` becomes `registry.NewGitHub(...)` with `GitHubCacheDir`, `GitHubTrustedKeys`, `GitHubToken` and `GitHubAPIURL`, and each plugin goes through `RegisterPlugin`.
  - `state_store.path` maps to `xcl.WithStatePath`. `state_store.host` is answered by the operations task (`invalid_argument` until then).
  - The masks map to `mask.EncryptAES256GCM`, `mask.HashHMACSHA256`, `mask.Redact()` and `mask.Omit()` (check the constructor names in `mask/aes.go`, `hmac.go`, `redact.go` and `omit.go`).
  - `variables_json` is decoded into `map[string]any` and passed to `WithVariables`.
  - The event fields are stored for the operations task.
- New `coreservice/lookups.go`:
  - `Entities` marshals each `cfg.Entities()` element with `encoding/json`.
  - `Find` uses `cfg.FindResource`.
  - `FindByType` uses `cfg.FindEntitiesByType`, and `FindOne` uses `cfg.FindOneEntity`.
  - `Outputs` uses `json.Marshal(cfg.Outputs())`.
  - `EncodeSavedEntity` uses `cfg.EncodeSavedEntity(data, xcl.Highlight(...)` when `highlight` is set). Check the available renderer in `highlight/`, and use the ANSI renderer.
- New `coreservice/errors.go`: `toProto(err error) *corepb.Error`. It matches with `errors.Is`/`errors.As` against every re-export in `config.go:26-170`, following the design's code table plus language-surface's `FunctionNameClashError`, `InvalidFunctionError` and `InvalidTypeError` (`errors/`). It fills details from the detail-type fields. `*xclerrors.ConfigError` becomes `config`, with each collected error as a cause. Read `errors/config_error.go` and `parser_error.go` for the file, line, column and summary fields. Anything unmatched is `internal`.
- Tests, in `coreservice/*_test.go` with a local `testClient` helper that encodes, calls and decodes:
  - `TestEveryServiceMethodHasAHandler`;
  - `TestUnknownMethodReturnsStatus`;
  - `TestBadRequestBytesReturnsStatus`;
  - `TestUnknownHandleReturnsStatus`;
  - `TestHandleZeroForConfigMethodReturnsStatus`;
  - `TestGetVersionReportsVersion`;
  - `TestCreateConfigDeclaresSchemaType`;
  - `TestCreateConfigRegistersExternalPlugin` (fixture from `internal/testutil/plugins.go`);
  - `TestCreateConfigGitHubRegistryUsesAPIURL` (`internal/testutil/github_release.go`);
  - `TestLookupsReturnSensitiveMarker`;
  - `TestOutputsReturnJSONObject`;
  - one `TestErrorCode<Name>` per code;
  - `TestConfigErrorListsCausesWithFileAndLine`.

  Lookups need an applied config, so call `Config.Apply` directly through an exported-for-test hook, or wait for the operations task and keep those tests there. Preferred: put the lookup tests in the operations task and test lookups here against a `Load` of state written by a real Go apply.
- `.github/workflows/go.yml:24-28`: the `CGO_ENABLED=0` builds already cover `./...`, which now includes `coreservice`. Add a `buf lint` step and a `buf breaking --against '.git#branch=main'` step (`bufbuild/buf-setup-action`, pinned).
**Complexity**: High
**Token estimate**: ~70k
**Agent strategy**: parallel analysis (proto/buf/dispatcher; config mapping; error mapper), sequential integration.

### Task: Operations run through the poll loop and can be cancelled

- New `coreservice/queue.go`: `taskQueue{pending chan *pending; inflight sync.Map; next atomic.Uint64}` and `run(ctx, task) (completion, error)`, as the design sketches, plus `post(task)` for events and finished tasks, which have no completion. `Poll(wait_ms)` selects on `pending` with a timer. `Complete` looks up `inflight`; an unknown or late id gives `invalid_argument` with `task_id` in the details.
- New `coreservice/operations.go`: `configuration{cfg *xcl.Config; queue; mu; running *operation; nextOp}` and `operation{id; cancel context.CancelFunc; ctx}`.
  - `startOperation(kind, fn)` returns `operation_in_progress` when one is running. It starts a goroutine that calls `cfg.ApplyContext(ctx, paths...)` (or the other forms, or `DiffContext` with `diff.RevealSensitive()` when asked), then posts `OperationFinished{error: toProto(err), diff_json}` as the last task and clears `running`.
  - `Cancel(operation_id)` calls `cancel()`. An error matching `context.Canceled` maps to code `cancelled`.
  - `Release` cancels, waits for the goroutine, then deletes the handle.
- New `coreservice/events.go`: when `CreateConfigRequest.events` is set, install `xcl.WithEventHandler(func(e Event){ queue.post(eventTask(op, e)) })`, with `WithEventBufferSize`, `WithEventData` (enum mapping), `WithEventMask` and `WithNoEventMask`. `eventTask` copies the `events.Event` fields (`events/events.go:72-125`), with `Error` through `toProto` and `Meta` as JSON. `post` blocks when the channel is full, which is the backpressure.
- New `coreservice/state_store.go`: `ForeignStateStore` implements `state.StateStore` (`state/state_store.go:9`). Each method calls `queue.run(currentOperationCtx, StateTask{kind})`. `Load` returns each entity as `json.RawMessage`, and `Save` marshals each element. This is used when `state_store.host` is set.
- `coreservice/lookups.go`: refuse lookups with `operation_in_progress` while `running != nil`.
- Tests, in `coreservice/operations_test.go` and the other test files. The `testClient` gains a `drive(handle, opID, handlers)` that polls and completes.
  - `TestApplyEndsWithFinishedTask`;
  - `TestApplyMatchesGoApply` (compare `Entities` JSON with a direct `xcl.Config` apply);
  - `TestEventsArriveBeforeFinishedInOrder`;
  - `TestEventDataMaskedByDefault`;
  - `TestHostStateStoreReceivesSaveOnApply`;
  - `TestHostStateStoreServesLoad`;
  - `TestHostStateStoreWithMaskReceivesEncryptedValues`;
  - `TestDiffFinishedCarriesDiffJSON`;
  - `TestCancelRunningApplyReturnsCancelledAndSaves`;
  - `TestCancelStartsNoProviderCallAfterwards`, using a blocking in-process `parser.TestPlugin`-style provider. Assert on calls started after the cancel, not on order (convention: graph parents);
  - `TestSecondOperationRefused`;
  - `TestLookupDuringOperationRefused`;
  - `TestLateCompleteIsInvalidArgument`;
  - `TestReleaseCancelsAndWaits`.

  State comes from real applies throughout.
**Complexity**: High
**Token estimate**: ~75k
**Agent strategy**: parallel analysis (queue/operations; events/state store), sequential integration, then run the full suite with `-race`.

### Task: Host-language functions are callable from configuration

- New `coreservice/functions.go`, `makeHostFunc(conf *configuration, name string, sig plugins.FunctionSignature) (any, error)`:
  - It maps each cty JSON type with `ctyjson.UnmarshalType`-equivalent parsing. coreservice can't import `internal/cty`, so parse the JSON type strings directly: `"string"`, `"bool"`, `"number"`, `["list",T]`, `["map",T]`, `["set",T]` as a slice, and `"dynamic"` refused.
  - The Go types are `string`, `bool`, `float64`, `[]T` and `map[string]T`.
  - `reflect.FuncOf(in, []reflect.Type{ret, errorType}, variadic)` and `reflect.MakeFunc` do the call: `json.Marshal` the args as an array, then `conf.queue.run(conf.currentOperationCtx(), FunctionTask{name, args})`; `json.Unmarshal` the result into `reflect.New(ret)`; and a `FunctionResult.error` becomes an `errors.New(message)` return.
  - When no operation is running (a function evaluated outside one), return an error rather than block.
- `coreservice/config.go` `hostRegistry.Functions()` returns `[]registry.Function{{Name, Func: made}}` for each `DeclaredFunction`. A malformed signature JSON or an unsupported type returns `invalid_argument` from `CreateConfig`. Clash and conversion errors come from `xcl.NewConfig` (language-surface `catalog.DeclareFunction`) and map to `function_name_clash` / `invalid_function`.
- `PollResponse.function` and `CompleteRequest.function` are routed in `poll` and `complete`.
- Tests in `coreservice/functions_test.go`:
  - `TestHostFunctionResultFilledIn` (slugify);
  - `TestHostFunctionErrorFailsApplyAtPosition`;
  - `TestHostFunctionClashWithBuiltinFailsCreateConfig` (`upper`);
  - `TestTwoHostFunctionsSameNameFailCreateConfig`;
  - `TestHostFunctionUnsupportedTypeFailsCreateConfig`;
  - `TestHostFunctionReceivesSensitiveArgumentUnmarked`;
  - `TestHostFunctionResultMarkedSensitive`;
  - `TestCancelWithOutstandingFunctionTaskEndsPromptly`;
  - `TestHostFunctionListAndMapArguments`;
  - `TestMakeHostFuncVariadic`.

  If language-surface's conversion rejects variadic funcs, the last test asserts `invalid_function` instead (research open assumption).
**Complexity**: Medium
**Token estimate**: ~40k
**Agent strategy**: single agent, sequential.

### Task: The engine is built as a C shared library

- New `bindings/libxcl/go.mod`: `module github.com/jumppad-labs/xcl/bindings/libxcl`, `go 1.25.0`, `require github.com/jumppad-labs/xcl v0.0.0`, and `replace github.com/jumppad-labs/xcl => ../..`.
- New `bindings/libxcl/main.go`: `package main`, `import "C"`, and a process-wide `service = coreservice.New()`. Exports:
  - `//export xcl_abi_version` returns `coreservice.ABIVersion`;
  - `xcl_call` copies the request with `C.GoBytes` and `C.GoString(method)`, calls `service.Call`, sets `*resp = (*C.uint8_t)(C.CBytes(out))` and `*resp_len`, and returns the status;
  - `xcl_free(p)` calls `C.free` when non-nil;
  - `xcl_release(h)` returns `service.Release`.

  Add `func main() {}`.
- New `bindings/libxcl/xcl.h` matching the design's declarations, for reference and the smoke test.
- New `bindings/libxcl/smoke_test.go`: a Go test with cgo that `dlopen`s the built library through a small C helper in the test package, or calls the exported functions directly within the package. It checks the ABI version, `GetVersion` (the stamped value when built with `-ldflags -X`), `CreateConfig`, `StartValidate`, polling to finished, and `xcl_release`.
- `Makefile`: add `LIBXCL_VERSION ?= dev` and a `libxcl` target running `cd bindings/libxcl && CGO_ENABLED=1 go build -buildmode=c-shared -trimpath -ldflags="-s -w -X github.com/jumppad-labs/xcl/coreservice.Version=$(LIBXCL_VERSION)" -o $(LIBXCL_OUT)`, with the extension per `GOOS`. Add a `test-libxcl` target.
- `.github/workflows/go.yml`: add a `libxcl` job on `ubuntu-latest` (Go 1.27.0, CGO on) that builds the library and runs its tests. Also extend the `build-minimum-go` loop to build the nested module.
**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: single agent.

### Task: Go fixture plugins and the Go side of the parity check

- New `bindings/libxcl/testdata/plugins/slow/main.go`: an external plugin (served with `plugins.Serve`, as `e2e/fixtures/externalplugin` does) providing `resource "slow" "<name>" { delay_ms = 500 }`, whose Create sleeps for `delay_ms`.
- New `bindings/libxcl/testdata/plugins/recording/main.go`: `resource "recording" "<name>" { log = "<path>", delay_ms = … }`. Every lifecycle call appends `{"step":"create","address":…,"time":…}` to `log` *before* doing its work, then sleeps. It is modelled on `e2e/fixtures/recorder/recorder.go`.
- New `bindings/libxcl/testdata/parity/<fixture>/` directories, each with `main.xcl`, an optional `changed/main.xcl` for the diff, and `types.json`. `types.json` declares the schema v1 types both sides register: Go registers them through `registry.TypeFromSchema`, and Python from equivalent dataclasses that the Python parity task writes.
  - Fixtures: `basic_types`, `annotations` (required/optional/default/sensitive/computed/nested/list/map), `functions` (`slugify` declared as a Go func on the Go side), `variables`, `outputs`, `plugin_entities` (the recording plugin), `invalid` (several validation errors).
- New `bindings/libxcl/cmd/parity/main.go`: `parity -fixture <dir> -plugins <dir> -out <file>`. It registers the types and functions, then runs Validate, Diff (before and after apply), Apply, Entities, Outputs, a state-file read, Load in a fresh `xcl.Config` (new process via `os/exec` of itself with `-load`) and Destroy. It writes one JSON record with temp paths replaced by `<tmp>` and times removed (knowledge `gotchas/event-data-includes-temp-paths.md`), and decoded state rather than raw text (knowledge `gotchas/file-state-is-indented-json.md`).
- New `bindings/libxcl/cmd/parity/main_test.go`: `TestParityRecordIsDeterministic` (run twice, compare) and `TestParityRecordCoversEveryStage`.
- `internal/testutil/plugins.go`: leave as is. These fixtures belong to the nested module and are built by the Python CI with `go build`.
**Complexity**: Medium
**Token estimate**: ~45k
**Agent strategy**: 2 parallel agents (plugins; parity program and fixtures).

### Task: Create the GitHub repository jumppad-labs/xcl-python and connect the registered local directory to it

- xcl-python is already registered in the Spektacular project. Its root is `/home/nicj/code/github.com/jumppad-labs/xcl-python`, which holds only the `.spektacular` footprint `spektacular repo add` created.
- A person creates `github.com/jumppad-labs/xcl-python` (public, Apache-2.0, default branch `main`) with no initial commit.
- In the root, they run `git init -b main`, add the Apache-2.0 `LICENSE`, `git remote add origin git@github.com:jumppad-labs/xcl-python.git`, then commit and push the footprint and licence.
- Check: `spektacular repo list` shows `xcl-python` with its root on disk, and `git -C /home/nicj/code/github.com/jumppad-labs/xcl-python remote -v` shows the GitHub origin.

### Task: The Python package loads the engine safely

- `xcl-python:pyproject.toml`:
  - `name = "xcl"`, `requires-python = ">=3.10"`, `dependencies = ["protobuf>=5.26", "cffi>=1.16"]` (each with a comment giving the reason);
  - build backend `hatchling`, with a build hook that marks the wheel platform-specific and tags it `py3-none-<platform>` from `XCL_WHEEL_PLATFORM`;
  - `version` from `xcl/_version.py`;
  - `readme = "README.md"`.
- `xcl-python:Makefile`:
  - `XCL_SRC ?= ../xcl`;
  - `protos`: `buf generate $(XCL_SRC)/proto --template buf.gen.yaml` writes `xcl/_core/core_pb2.py` and `.pyi`;
  - `libxcl`: `make -C $(XCL_SRC) libxcl LIBXCL_VERSION=v$(VERSION) LIBXCL_OUT=$(PWD)/xcl/_lib/<libname>`;
  - `test`.
- `xcl-python:buf.gen.yaml`: the `python` and `pyi` plugins (remote `buf.build/protocolbuffers/python`, pinned).
- `xcl-python:xcl/_transport/__init__.py`: a `Transport` Protocol, `call(handle, method, request) -> tuple[int, bytes]` and `release(handle) -> int`.
- `xcl-python:xcl/_transport/inprocess.py`:
  - `_InProcessTransport` with lazy `_load()`. The library path is `XCL_LIBRARY_PATH` if set, otherwise `xcl/_lib/libxcl.{so,dylib,dll}`. The library is loaded with cffi `ffi.dlopen` using the design's `cdef`, falling back to `ctypes.CDLL` when cffi import fails.
  - `call` copies the response and calls `xcl_free` in `finally`.
  - Fork guard: `os.register_at_fork(after_in_child=_mark_forked)` is registered at import, and only marks a child when the library was loaded before the fork. A pid comparison is also made. A marked child raises `ForkedProcessError`, whose message names `multiprocessing.set_start_method("spawn")` and creating the configuration after the fork (gunicorn `post_fork`, celery `worker_process_init`).
  - On first load it checks `xcl_abi_version() == 1`, or raises `EngineVersionError`.
- `xcl-python:xcl/_client.py`: `_Client.call(handle, method, request)` as in the design. The output class comes from `core_pb2.DESCRIPTOR.services_by_name["CoreService"].methods_by_name[method].output_type` through `message_factory.GetMessageClass`. On first use it calls `GetVersion` and compares it with `xcl.__version__` after normalising the leading `v`; a mismatch raises `EngineVersionError(package, engine)`.
- `xcl-python:xcl/_errors.py`: `Error(Exception)` with `code`, `message`, `details`, `causes`. There is one subclass per code: `not_found`, `not_unique`, `unknown_type`, `not_typeable`, `not_registered`, `type_mismatch`, `not_an_entity`, `type_name_clash`, `type_form`, `plugin_load`, `plugin_not_found`, `plugin_verification`, `empty_configuration`, `unregistered_type`, `invalid_saved_data`, `not_encodable`, `unrecoverable`, `mask_not_reversible`, `config`, `provider`, `cancelled`, `operation_in_progress`, `invalid_argument`, `function_name_clash`, `invalid_function`, `invalid_type` and `internal`. Add `from_proto(err)`, plus `ForkedProcessError`, `EngineVersionError` and `InternalBindingError(method, status)`. `ConfigError` exposes `problems` (each with file, line, column, message).
- `xcl-python:xcl/__init__.py`: exports, with `__version__`.
- Tests in `xcl-python:tests/unit/` (a fake transport in `tests/fakes.py`):
  - `test_import_does_not_load_engine`;
  - `test_engine_version_mismatch_names_both`;
  - `test_abi_version_mismatch_raises`;
  - `test_each_code_raises_its_class` (one function per class family, no parametrize);
  - `test_unknown_code_raises_base_error`;
  - `test_negative_status_raises_internal_binding_error`;
  - `test_forked_child_after_use_raises` (real `os.fork` on POSIX, skipped on Windows);
  - `test_forked_child_after_import_only_works`.

  Integration `tests/integration/test_load.py` checks that `xcl.Config().close()` works against the built library.
**Complexity**: High
**Token estimate**: ~55k
**Agent strategy**: parallel analysis (packaging/build; transport/client/errors), sequential integration.

### Task: Python classes describe entity types

- `xcl-python:xcl/_entity.py`: `entity(type, subtype=None)` decorator, which records `__xcl_type__`/`__xcl_subtype__` and registers the class in a module registry used for hydration. Also `Resource` (base dataclass with `meta: Meta | None`, `depends_on`, `disabled`, mapping `ResourceBase`), `Meta`, and `Entity` (a `Mapping` with `.meta`, `.address`).
- `xcl-python:xcl/_types.py`:
  - `Sensitive[T]`, a generic with `__repr__`/`__str__` giving `(sensitive)` and `reveal()`;
  - `Computed[T]`, an `Annotated` marker;
  - `field(name=None, block=None, label=None, default=..., default_factory=...)`, which wraps `dataclasses.field` with metadata;
  - `UNKNOWN`.
- `xcl-python:xcl/_schema.py`:
  - `schema_for(cls) -> dict`. It uses `typing.get_type_hints(include_extras=True)`, applies the design's derivation rules plus `dict[str, Dataclass]` as a named-block map with `label` set when the element has a `name` field marked `xcl.field(label=…)`, and refuses unsupported annotations and sensitive shapes outside the engine's known set (str, int, float, bool, list[str], dict[str, str]).
  - `hydrate(data: dict, cls=None)`, which handles the `"(sensitive)"` marker and the `{"xcl_masked": …}` envelope.
  - `signature_for(fn) -> dict`, giving `FunctionSignature` JSON with cty JSON type strings: `"string"`, `"bool"`, `"number"`, `["list",T]`, `["map",T]`, `*args` as `Variadic`.
- Tests in `xcl-python:tests/unit/test_schema.py`, `test_hydrate.py`, `test_signature.py`. There is one function per annotation form and per refusal, for example `test_optional_union_none_is_optional`, `test_default_becomes_default`, `test_sensitive_str_sets_sensitive`, `test_sensitive_list_of_int_is_refused`, `test_computed_is_optional_computed`, `test_dataclass_field_is_block`, `test_list_of_dataclass_is_repeated_block`, `test_dict_of_dataclass_is_named_block_map`, `test_hydrate_named_block_map_to_dict`, `test_unregistered_type_becomes_entity`, `test_sensitive_repr_masked`, `test_looked_up_sensitive_reveals_none`, `test_signature_str_to_str`, `test_signature_rejects_object_param`.
**Complexity**: Medium
**Token estimate**: ~45k
**Agent strategy**: single agent.

### Task: The synchronous API runs every operation and lookup

- `xcl-python:xcl/registry.py`:
  - `LocalRegistry` (`register_type`, `register_function` stub completed by the functions task, `register_external_plugin`, `register_plugin_directory`, `plugin_pattern`) and `GitHubRegistry` (constructor options, `register_plugin`, `register_type`);
  - `_to_proto()` builds `Registry` messages.
- `xcl-python:xcl/mask.py`: `aes256gcm(key)`, `hmac_sha256(key)`, `redact()`, `omit()`, each returning a `Mask` message builder.
- `xcl-python:xcl/_poll.py`: `_Operation(client, handle, op_id, dispatch)`, a generator-style engine.
  - `step(wait_ms)` returns the next task or `None`.
  - `complete(task_id, result)`.
  - `finish(task)` raises the mapped error or returns the diff.
  - `cancel()` sends `Cancel`.

  The dispatch table maps state tasks to store methods (or to the state path, which needs none), event tasks to `on_event`, and function tasks to the functions task.
- `xcl-python:xcl/config.py`: `Config(...)` builds `CreateConfigRequest`. `variables` go to JSON, `event_data` maps `"none" | "raw" | "processed"` to the enum, `state_store` sets `host=True` and keeps the object, and `on_event` sets `events=True`.
  - The operations start and then drive `_Operation` on the calling thread with `Poll(wait_ms=200)`. `KeyboardInterrupt` calls `cancel()`, drains to finished, and re-raises.
  - Lookups decode JSON and hydrate.
  - `close()` releases, and `__enter__`/`__exit__` are provided.
- `xcl-python:xcl/events.py`: `Event` dataclass (fields of `events.Event`; `entity()` hydrates `data`).
- `xcl-python:xcl/diff.py`: `Diff`, `ResourceDiff`, `Change` from `diff_json` (shape from `xcl:diff/diff.go`).
- Integration tests in `xcl-python:tests/integration/`, one function per spec criterion:
  - `test_registered_class_decodes_and_is_found`;
  - `test_subtype_class_found_at_address`;
  - `test_required_field_missing_fails_at_block`;
  - `test_optional_field_gets_default`;
  - `test_sensitive_masked_in_events_and_state`;
  - `test_computed_field_cannot_be_set`;
  - `test_nested_block_decoded`;
  - `test_two_volumes_in_order`;
  - `test_listeners_dict_by_name`;
  - `test_variable_supplied`;
  - `test_outputs_dict`;
  - `test_diff_reports_old_and_new`;
  - `test_state_directory_load_in_new_process` (`subprocess` running a helper script);
  - `test_custom_store_round_trip_new_process`;
  - `test_custom_store_encrypted_with_state_mask`;
  - `test_events_start_and_finish_per_entity`;
  - `test_event_data_redacted`;
  - `test_missing_address_raises_not_found`;
  - `test_find_one_several_raises_not_unique`;
  - `test_invalid_config_raises_config_error_with_positions`;
  - `test_sync_apply_does_not_stall_threads` (slow plugin from `xcl:bindings/libxcl/testdata/plugins/slow`, built by a session fixture with `go build` and skipped when Go is absent);
  - `test_keyboard_interrupt_cancels_apply`.
**Complexity**: High
**Token estimate**: ~75k
**Agent strategy**: parallel analysis (registries/options/messages; poll engine; lookups/hydration wiring), sequential integration.

### Task: Python functions are callable from configuration

- `xcl-python:xcl/registry.py` `LocalRegistry.register_function(fn, name=None)`: stores `(name or fn.__name__, fn, signature_for(fn))` and emits `DeclaredFunction{name, signature: json}`.
- `xcl-python:xcl/_functions.py`: `call_function(fn, args_json) -> FunctionResult`. It decodes the JSON array, converts lists and dicts, calls the function, and encodes the result. An exception becomes `FunctionResult.error.message = str(exc)`.
- `xcl-python:xcl/_poll.py`: the `function` dispatch looks up the callable by name on the config's registries.
- Integration tests in `xcl-python:tests/integration/test_functions.py`:
  - `test_slugify_result_filled_in`;
  - `test_raising_function_fails_apply_at_call_position`;
  - `test_function_named_like_builtin_raises_clash`;
  - `test_two_functions_same_name_raise_clash`;
  - `test_sensitive_argument_plain_and_result_sensitive`.
**Complexity**: Medium
**Token estimate**: ~25k
**Agent strategy**: single agent.

### Task: Go plugins are usable from Python

- `xcl-python:tests/integration/test_docker_plugin.py`:
  - A session fixture downloads the `jumppad-labs/xcl-plugin-docker` release built against the xcl version under test (version pinned in `tests/plugins.toml`) for the current platform, or builds it from a local checkout when `XCL_PLUGIN_DOCKER_SRC` is set.
  - It skips unless `docker info` succeeds.
  - Tests: `test_docker_plugin_from_path`, `test_docker_plugin_from_directory`, `test_docker_plugin_from_github_registry` (real GitHub with the published public key from `tests/keys/jumppad-labs.asc`), `test_plugin_entities_readable_generically`.
- `xcl-python:tests/integration/test_github_tamper.py`: a local `http.server` fake GitHub API serves the real release's assets with the checksums file altered, and points `GitHubRegistry(api_url=…, trusted_keys=[key], cache_dir=tmp)` at it. `test_tampered_release_refused` expects `PluginVerificationError`. The route shape follows `xcl:internal/testutil/github_release.go` and `xcl:registry/github_client.go`.
- Example config in `xcl-python:tests/integration/configs/docker/main.xcl`, a network and a container, mirroring `xcl-plugin-docker:examples/basic`.
**Complexity**: Medium
**Token estimate**: ~35k
**Agent strategy**: single agent.

### Task: Python gives the same results as Go on the parity fixtures

- `xcl-python:tests/parity/types.py`: dataclasses equivalent to each fixture's `types.json`. The test checks that `schema_for(cls)` equals the JSON, which keeps the two sides declaring the same types.
- `xcl-python:tests/parity/run_python.py`: the Python counterpart of `xcl:bindings/libxcl/cmd/parity`, with the same stages, normalisation and record shape. Load runs in a `subprocess`.
- `xcl-python:tests/parity/test_parity.py`: one test function per fixture (`test_parity_basic_types`, `test_parity_annotations`, `test_parity_functions`, `test_parity_variables`, `test_parity_outputs`, `test_parity_plugin_entities`, `test_parity_invalid`). Each builds the Go parity program and the recording plugin from `$XCL_SRC` (session fixture), runs both, and compares the decoded records. On mismatch it reports the fixture name and the first differing JSON path.
**Complexity**: Medium
**Token estimate**: ~40k
**Agent strategy**: single agent.

### Task: The asyncio API never blocks and can be cancelled

- `xcl-python:xcl/aio.py`: `Config`, with the same constructor and methods, all `async`. Each operation:
  1. starts the operation with `await loop.run_in_executor(self._executor, client.call, …)`, using a dedicated single-thread executor per config for `Poll`;
  2. loops on `await loop.run_in_executor(self._executor, op.step, 200)`;
  3. dispatches each task with `asyncio.create_task`. A coroutine function is awaited, and a plain one goes through `loop.run_in_executor(None, …)`;
  4. completes through the executor.

  On `asyncio.CancelledError` it calls `op.cancel()`, cancels outstanding dispatch tasks, keeps stepping until finished, then re-raises `CancelledError`. Lookups also run in the executor. `close()` is an async method, and `__aenter__`/`__aexit__` are provided.
- `xcl-python:xcl/_poll.py`: no loop logic is added. `aio` uses `step` and `complete` only.
- Integration tests in `xcl-python:tests/integration/test_aio.py`. They use plain `asyncio.run` inside each test, or `pytest-asyncio` if adopted, with no tables:
  - `test_async_function_awaited`;
  - `test_async_event_handler`;
  - `test_async_state_store_round_trip`;
  - `test_other_coroutines_run_during_apply` (a ticker counts while the slow plugin runs);
  - `test_http_server_answers_during_long_apply` (`asyncio.start_server`, with 20 requests each answered within 100 ms);
  - `test_cancel_apply_ends_promptly` (`wait_for` with a bound);
  - `test_cancel_apply_starts_no_operation_after_cancel` (reads the recording plugin's log and asserts no `create` started after the cancel timestamp);
  - `test_sync_and_async_expose_same_surface` (compares public method names, and checks the async ones with `inspect.iscoroutinefunction`).
**Complexity**: High
**Token estimate**: ~50k
**Agent strategy**: single agent, sequential (driver first, then the cancellation path, then the tests).

### Task: Tagging xcl publishes signed engine builds for eight platforms

- New `.github/workflows/libxcl-release.yml`. It runs `on: push: tags: ['v*']` and `workflow_dispatch` (input `tag`, `publish: false`). Jobs:
  - `build-linux-windows` on `ubuntu-latest`, with `goto-bus-stop/setup-zig` pinned and Go 1.27.0. The matrix covers:
    - `x86_64-linux-gnu.2.28`, `aarch64-linux-gnu.2.28`, `x86_64-linux-musl` and `aarch64-linux-musl` (`CC="zig cc -target …"`, `GOOS=linux`);
    - `x86_64-windows-gnu` (`GOOS=windows`, `GOARCH=amd64`).

    Each runs `make libxcl LIBXCL_VERSION=${tag} LIBXCL_OUT=…`.
  - `build-windows-arm64`: llvm-mingw (pinned release) as `CC=aarch64-w64-mingw32-clang`, `GOOS=windows`, `GOARCH=arm64`.
  - `build-macos`: `macos-latest` for arm64, and the Intel runner for x86-64, with `MACOSX_DEPLOYMENT_TARGET=11.0`.
  - `package`:
    - Archives each library as `libxcl_<version>_<os>_<arch>[_musl].tar.gz` (`.zip` on Windows), following the naming in `registry/github_assets.go`. Musl gets an `_musl` suffix on the arch; this is an extension recorded in the assumptions.
    - Writes `libxcl_<version>_checksums.txt` and signs it with `gpg --detach-sign --armor` using the `XCL_RELEASE_GPG_KEY` and `XCL_RELEASE_GPG_PASSPHRASE` secrets, as the plugin template's `make dist` does.
    - Checks glibc symbol versions with `objdump -T | grep GLIBC_` ≤ 2.28.
    - Runs `gh release upload`.
  - `dispatch`: `gh api repos/jumppad-labs/xcl-python/dispatches -f event_type=xcl-release -f client_payload[tag]=${tag}` using `XCL_PYTHON_DISPATCH_TOKEN`. It is skipped when `publish` is false.
- `Makefile` `libxcl`: honour `LIBXCL_OUT` and pass through `CC`, `GOOS` and `GOARCH`.
**Complexity**: Medium
**Token estimate**: ~35k
**Agent strategy**: single agent.

### Task: Add the release signing key and dispatch token to xcl

- In the GitHub settings of `jumppad-labs/xcl`, add the secrets:
  - `XCL_RELEASE_GPG_KEY`: the armoured private key the plugin releases are signed with;
  - `XCL_RELEASE_GPG_PASSPHRASE`;
  - `XCL_PYTHON_DISPATCH_TOKEN`: a fine-grained token with `contents: read` and `actions: write` on `jumppad-labs/xcl-python`.
- Check: run the release workflow by hand with `publish: false`. The uploaded workflow artifact's `.sig` should verify with `gpg --verify` against the public key the plugin registry tests trust.

### Task: xcl-python is tested on every platform and released from the xcl tag

- `xcl-python:.github/workflows/ci.yml`: a matrix of `{ubuntu-latest, ubuntu-24.04-arm, <intel macOS>, macos-latest, windows-latest, windows-11-arm}` × `{3.10, 3.11, 3.12, 3.13, 3.x}`, plus Alpine `python:3.x-alpine` container jobs on both Linux runners for musl.
  - Steps: check out xcl at the ref in `xcl-ref.txt` (the tag the package tracks, or `main` on development), set up Go 1.27 and a C compiler for building libxcl in CI only, run `make protos libxcl`, install with `pip install -e .[test]`, and run `pytest`.
  - Docker plugin tests run where Docker is present, which is the Linux x86-64 runner.
- `xcl-python:.github/workflows/release.yml`: `on: repository_dispatch: types: [xcl-release]` and `workflow_dispatch` (input `tag`). Jobs:
  - `wheels` (matrix of the eight platforms):
    1. download `libxcl_<v>_<os>_<arch>` and the checksums and signature from the xcl release with `gh release download`;
    2. verify with `gpg --verify` against `keys/jumppad-labs.asc` and `sha256sum -c`;
    3. place the library in `xcl/_lib/`;
    4. set the version from the tag with the `v` stripped;
    5. regenerate the protos from xcl at the tag;
    6. run `python -m build --wheel` with `XCL_WHEEL_PLATFORM` set to the PEP 600/656 tag (`manylinux_2_28_x86_64`, `manylinux_2_28_aarch64`, `musllinux_1_2_x86_64`, `musllinux_1_2_aarch64`, `macosx_11_0_x86_64`, `macosx_11_0_arm64`, `win_amd64`, `win_arm64`);
    7. install the built wheel on the native runner or container and run the parity suite and smoke import.
  - `sdist`.
  - `publish`: `environment: pypi`, `permissions: id-token: write`, `pypa/gh-action-pypi-publish` pinned.
  - `install-check` (matrix of the eight platforms, needs `publish`): on a runner with no Go (`PATH` scrubbed of `go`, `gcc`, `cc`), retry `pip install xcl==<v>` until it is available, then run `python -c "import xcl; xcl.Config().close()"` and a check that `xcl._engine_version()` equals the tag.
- `xcl-python:keys/jumppad-labs.asc`: the public release key.
- `xcl-python:xcl-ref.txt`.
**Complexity**: High
**Token estimate**: ~50k
**Agent strategy**: parallel analysis (CI matrix; release pipeline), sequential integration.

### Task: Set up the PyPI project and trusted publishing

- On pypi.org, make sure the `xcl` project exists under the jumppad-labs organisation. If the name is taken by someone else, stop and raise it with the user, because the spec fixes `pip install xcl`.
- Add a trusted publisher: owner `jumppad-labs`, repository `xcl-python`, workflow `release.yml`, environment `pypi`. Use a pending publisher if the project has no release yet.
- In `jumppad-labs/xcl-python` settings, create the `pypi` environment, restricted to tags or the default branch.
- Check: the trusted publisher shows on the PyPI project's publishing page, and the environment exists.

### Task: The Python package is documented

- `xcl-python:README.md`:
  - Install: `pip install xcl`, the supported platforms and CPython versions.
  - Getting started (sync): a dataclass entity, the Docker plugin through `GitHubRegistry` with the trusted key, `apply`, `find`, `destroy`.
  - Variables, state, events and functions in brief.
  - Asyncio: `xcl.aio.Config`, async functions and handlers, and cancellation.
  - Errors.
  - Fork safety (`spawn`, or create after fork).
  - Links to xcl and xcl.dev.
- `xcl-python:PARITY.md`: a table of each Go operation and lookup against its `xcl.Config` and `xcl.aio.Config` equivalent, with the "not applicable" rows from the assumptions.
- Docstrings on every public module, class, function and method in `xcl/__init__.py`'s `__all__`, `xcl.aio` and `xcl.mask`.
- `xcl-python:tests/unit/test_docstrings.py`: `test_every_public_name_has_docstring` walks `xcl.__all__`, `xcl.aio` and `xcl.mask` with `inspect` and asserts on `__doc__`. This exercises code objects, not repository files.
- `pyproject.toml` `readme = "README.md"`, which is already set by the packaging task. Confirm `twine check` passes in CI.
**Complexity**: Low
**Token estimate**: ~25k
**Agent strategy**: single agent.

### Task: Record the Python support in the xcl changelog

- `CHANGELOG.md:1-3`: insert `## 20261011130709-aa3eb933-python-support` after `# Changelog`, above language-surface's entry. It has prose paragraphs covering:
  - `ValidateContext`/`ApplyContext`/`DiffContext`/`DestroyContext`/`LoadContext`;
  - `Config.FindEntitiesByType`/`FindOneEntity`;
  - `registry.TypeFromSchema` and schema v1 (with map blocks and `label`), with the legacy plugin schema still read;
  - the `coreservice` package and `proto/xcl/core/v1`;
  - the `bindings/libxcl` nested module and its four C functions;
  - release assets `libxcl_<version>_<os>_<arch>` signed with the plugin key;
  - the `xcl` PyPI package.

  Then `**Breaking:**` and `None.`.
- `README.md`: a short "Other languages" note. The engine is available to Python as `pip install xcl` (link `jumppad-labs/xcl-python`), and Go users are unaffected.
- No tests (convention).
**Complexity**: Low
**Token estimate**: ~10k
**Agent strategy**: single agent.

### Task: Cut the first xcl release with Python support

- A person merges the xcl branch and xcl-python's `main`, then pushes the next xcl tag after `v0.3.0` (expected `v0.4.0`, a minor release, since every change is additive) with `git tag v0.4.0 && git push origin v0.4.0`.
- Check:
  - the xcl release shows eight `libxcl_` archives, the checksums and the `.sig`;
  - the xcl-python release run is green through `install-check`;
  - `https://pypi.org/project/xcl/0.4.0/` lists eight wheels and an sdist;
  - `pip install xcl==0.4.0` on a clean machine imports.

## Testing Strategy

Per-task test names and files are listed under each task above. Across the plan:

- **Context operations, untyped lookups, schema v1:**
  - root-package tests through `NewConfig`/`ApplyContext`;
  - unit tests in `internal/schema`;
  - one function per behaviour, with state from real applies.
- **Core service tasks:** Go tests drive `Service.Call` through a local test client that polls and completes. They cover:
  - every status;
  - every option mapping;
  - every error code;
  - event order with the finished task last;
  - host state store and host functions;
  - cancellation, with no provider call started afterwards;
  - lookups refused during operations.

  Run with `-race`.
- **libxcl:** a cgo smoke test through the four exports, plus CI build jobs. The `CGO_ENABLED=0` guard covers coreservice.
- **Fixtures and Go parity program:** determinism and stage-coverage tests.
- **Python packaging/transport:** unit tests against a fake transport, covering version and ABI checks, error mapping, the fork guard and lazy load.
- **Python schema:** unit tests per annotation form and per refusal, plus hydration and signature derivation.
- **Sync API, functions and Go plugins:** integration tests against the real library, one per spec acceptance criterion. Docker tests are Linux-only and skip without an engine. The tamper test uses a local fake GitHub API.
- **Parity:** one test per fixture comparing the Go and Python records, run on all eight platforms in CI and gating the release.
- **Asyncio:** integration tests for async functions, handlers and stores, loop responsiveness (ticker and HTTP server), cancellation with the recording plugin, and sync/async surface equality.
- **Release workflows:** dry runs and the post-publish install check. These have no unit tests.
- **Docs and CHANGELOG:** no tests read them (convention). The docstring test inspects imported objects.

Manual items (**Manual — captured in the implementation test plan**):
- following the README alone from `pip install xcl` to a working apply against a Go plugin;
- reviewing `PARITY.md` against the Go API at release;
- the xcl CHANGELOG entry;
- the PyPI project page;
- watching the first tagged release end to end.

## Project References

- Spec: `20261011130709-aa3eb933-python-support` (`spektacular spec file read 20261011130709-aa3eb933-python-support`).
- Design `libxcl.md` from the `design` source (binding).
- Upstream plan: `20261011095738-fad05e41-language-surface` (final; `spektacular plan file read 20261011095738-fad05e41-language-surface plan` and `context`).
- Knowledge entries in the xcl store:
  - conventions: `testing-and-mocking`, `shared-test-helpers`, `test-state-from-real-apply`, `shared-errors-package`, `project-structure`, `dependencies`, `code-style`, `patterns-and-architecture`, `development-standards`, `assert-ordering-on-graph-parents`;
  - glossary: `entity`;
  - gotchas: `plugin-types-rebuilt-with-structof`, `registered-types-convert-only-to-themselves`, `event-data-includes-temp-paths`, `file-state-is-indented-json`, `meta-field-golden-schema`, `sensitive-values-leak-through-fmt`;
  - architecture: `config-is-the-public-query-surface`.
- Repo roots:
  - xcl: `/home/nicj/code/github.com/jumppad-labs/xcl`;
  - xcl-python: `/home/nicj/code/github.com/jumppad-labs/xcl-python` (registered, GitHub repo created by a person);
  - xcl-plugin-docker: `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-docker` (used by the tests, not changed).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

**Parallel work.**
- In milestone 1, the three root-module tasks (context operations, untyped lookups, schema v1) and the fixture/parity task can run in parallel.
- In coreservice, protocol → operations → functions run in sequence. libxcl can start once the protocol task lands.
- In xcl-python, packaging → schema → sync API run in sequence. Functions, Go plugins and parity can then run in parallel, and asyncio follows functions.
- The release tasks come last, with the human tasks between them.

## Migration Notes

- None for Go users. Every root-module change is additive, and the existing operation methods keep their signatures and behaviour.
- Third-party `registry.Registry` implementations are unaffected beyond language-surface's `Functions()`.
- New: the xcl release gains `libxcl_<version>_*` assets. Consumers of the release page should expect them beside the source archives.

## Performance Considerations

- Each `xcl_call` encodes and decodes protobuf. This is negligible beside parsing or a provider call (design § costs accepted).
- `Poll` waits up to 200 ms per call in both Python drivers. This bounds cancellation latency at the Python side to one poll interval plus the engine's stop time.
- Events cross one task at a time. `WithEventBufferSize` backpressure applies exactly as in Go, so a slow Python handler slows emitters rather than growing memory.
- Python functions are called serially in the sync API, and concurrently as asyncio tasks in the async API. A function inside a large `for` expression pays one task round trip per element.
- Each wheel is about 7 MB compressed (19 MB installed). Eight wheels per release come to about 56 MB against PyPI's 10 GB project limit.
