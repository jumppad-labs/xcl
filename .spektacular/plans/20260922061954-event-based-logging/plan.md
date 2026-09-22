---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Plan: 20260922061954-event-based-logging

<!-- Metadata -->
<!-- Created: 2026-09-22T07:10:59Z -->
<!-- Commit: 51f1c0b -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

This plan makes xcl stop writing log output of its own. Everything xcl and its plugins report goes out as one ordered stream of events through the existing `WithEventHandler`: resource lifecycle steps, plugin discovery and loading, warnings, errors, and plugin authors' own log messages. Each event carries a time, a source and a details map. It is delivered from a bounded buffer that never drops events, it is fully delivered before `Validate`/`Apply`/`Destroy` return, and if the receiver panics, xcl shuts down gracefully before that panic is raised from the call. Application authors get one filterable view of what xcl is doing and a one-line bridge to `log/slog`. Plugin authors keep writing ordinary log messages, and those messages arrive labelled with the resource, step and plugin without the author passing any of it. The plugin registry drops its logger and loads plugins lazily, once, on first use. Examples, the library docs and xcl.dev are updated to match.

## Conventions

- **Prefer the standard library; document third-party dependencies** — the shipped logger adapter targets `log/slog`, and charmbracelet/log is justified only as an example dependency, never imported by a library package.
- **Pin dependency versions in go.mod** — any change to charm/go-plugin/grpc requirements (or protoc-generated code versions) keeps exact versions.
- **Shared error types live in the `errors` package, sentinel + detail struct with pointer receivers, re-exported from `xcl`** — the new plugin-load error follows `plugins/errors.go`'s pattern and live where root, parser and registry can all reach them without cycles.
- **Use testify `require`; NEVER table-driven tests; NEVER mix positive and negative cases in one test; favour verbose, readable tests** — every acceptance criterion becomes its own named test, with separate tests for success and failure paths (e.g. load success vs load failure).
- **Use Mockery for interface mocks** — `ProviderAdapter` and `ProviderResolver` signatures change (ctx threading, Init), so the Mockery-generated mocks in `plugins/mocks` and `internal/parser/mocks` are regenerated.
- **Generate test state with a real apply, not a hand-written state file** — graceful-shutdown and destroy tests obtain prior state by running apply first.
- **Use context.Context for request-scoped values** — the per-call bound logger travels in the provider call's ctx.
- **Implement graceful shutdown** — the receiver-panic path cancels the operation's context so no new provider call starts, lets in-flight ones finish and saves state before the panic continues.
- **Include proper logging with structured logs** — satisfied by the event stream: every diagnostic is a structured event with key/value details, and the slog adapter preserves them as attributes.
- **NEVER modify dependency packages** — go-plugin/charm/hclog behaviour is adapted around, never patched; `internal/dag` (vendored in-repo, MPL) is not changed because the cancellation check lives in the parser callbacks.
- **Use context.Context for cancellation** — the operation is stopped by cancelling its context, not by a hand-rolled flag; in-flight provider calls receive `context.WithoutCancel` so they are never aborted mid-call.
- **Go code style (gofmt/vet, `any` over `interface{}`, small interfaces)** — the rewritten `Logger` interface and new code use `any`.
- Dropped as not applicable: database/prepared statements, project-structure `/cmd`/`/api` layout (no new binaries or API definitions beyond the existing `plugins/plugin.proto`).

## Architecture & Design Decisions

**Shape.** All work lands in two repos: the library (`xclconfig`, root `/home/nicj/code/github.com/jumppad-labs/xcl`) and the docs site (`xcl-website`, root `/home/nicj/code/github.com/jumppad-labs/xcl-website`). In the library, a new leaf package `events` owns the single flat `Event` (today's fields plus `Time`, `Source` and a generic `Meta map[string]any`), the `Handler` type, the reserved Meta keys `level`/`message`, level/phase/source constants, and a ready-made `log/slog` adapter (`events.SlogHandler`). The root package re-exports `Event`/`EventHandler` as aliases, and the parser's duplicate `ParserEvent` is deleted. The `logger` package keeps its `Logger` interface (`Info/Debug/Warn/Error(msg, args...)`) and `WithTag`, but its only implementation becomes a thin wrapper that turns each call into an `Event` with `Phase: "log"`, the bound operation, resource and source, and the level, message and key/value details in `Meta`. The stdout logger, the event-first text formatting, the test logger and every `log.SetOutput` call are deleted, so nothing in the library can write output. Every emitter (the parser, registry, hosts, adapters and plugin loggers) holds only an `events.Emit` function. With no receiver it is a no-op, which is how xcl stays silent by default.

**Delivery and panics.** Each `Validate`/`Apply`/`Destroy` goes through one runner in `config.go`. It creates a bounded queue (`internal/eventstream`, sized by a new `WithEventBufferSize` option with a documented default), starts the operation's work on a **worker goroutine**, and uses the **caller's own goroutine as the single delivery goroutine**. The caller drains the queue into the receiver until the worker has finished and the queue is empty, so every event is delivered, in order and one at a time, before the call returns. Emitting never waits for the receiver. It waits only when the queue is full. The first emitter to block opens a "blocked stretch", and when space frees up a single `blocked` event carrying the stretch's duration is placed in a one-slot side channel that the drainer delivers ahead of the queue. It never takes a queue slot. Because the receiver runs on the caller's goroutine, a receiver panic already happens where the spec wants it raised. A deferred shutdown in the runner detects the incomplete drain without calling `recover`. It cancels the operation's `context.Context`, which `callProvider` and the destroy callback check (`ctx.Err()`) before starting any provider call, switches the queue to discard so blocked emitters are released, and waits for the worker. The worker lets in-flight calls finish and saves state through the existing partial-state (`progress.buildState`) and per-resource destroy save paths. The panic then continues with its **original value and original stack**. This goes beyond the spec's Technical Approach wording ("catch on the delivery goroutine, raise again on the caller"): recovering on one goroutine and panicking again on another loses the receiver's stack, which the acceptance criteria require (see `research.md#alternatives-considered-and-rejected`). The walk currently has no way to stop new provider calls and runs every call on `context.Background()`, so the operation context is new. It is threaded from the runner through the parser's `Validate`/`Apply`/`Destroy` into the walk callbacks. Provider calls receive `context.WithoutCancel(ctx)`, which carries the bound logger but not the cancellation, because the spec requires in-flight calls to finish.

**Context and the plugin boundary.** Before each provider call, `callProvider` and the destroy callback put a logger bound to the resource (ID, `type.name`, file) and the step into the call's `context.Context`. Each host adds the plugin's name as `Source`. Providers retrieve the logger with `plugins.Logger(ctx)`. The stored-logger pattern is not used for this, because concurrent calls to one provider would race on a logger stored in a field. `ctx` is threaded through every `PluginEntityProvider`, `PluginBase`, gRPC adapter and wrapper method that currently drops it. Across gRPC, the host registers the bound logger under a fresh call ID, sends the ID as gRPC metadata, and the plugin's server builds a `GRPCLogger` that stamps the ID on each `LogRequest` (new `call_id` field). The host callback resolves the ID back to the bound logger, which routes the log to the right Config's queue with the right resource and step. The per-RPC `SetLogger` re-`Init` is removed. Logs a plugin writes outside a call (`Plugin.Init`, go-plugin/hclog lines) go to the registry's *active* emitter, which is the one set by the operation currently running (last started wins) and is silent otherwise. The adapter's "calling provider" line is deleted, because the lifecycle `start` event already covers it.

**Registry and examples.** `registry.NewPluginRegistry()` takes no logger. `RegisterPlugin`, `RegisterPluginWithPath` and `DiscoverPlugins(dirs, pattern)` only record. `RegisterType` keeps its immediate builtin/registered clash check. A mutex-guarded `Load(emit)` runs once per registry (its result, including a failure, is cached). The runner calls it at the start of every operation, before state is loaded, and it emits `discover`/`load` events and plugin-clash errors. The shipped `events.SlogHandler` maps log events to their own level and every other event to Info, or Error for error phases. The examples replace `example/eventlog` with `example/prettylog`, which is the slog adapter with a charmbracelet/log handler. Only example packages import charm, so the library's import graph stays charm-free even though the module's `go.mod` keeps it. Rejected options, including re-initialising providers per call, resource context in every request message, a dedicated delivery goroutine, and a synchronous mutex-guarded handler, are in `research.md#alternatives-considered-and-rejected`. The applicable conventions are listed in the Conventions section: a shared `errors` package for new sentinels, testify `require` with no table tests, mockery regeneration, and stdlib-first (`log/slog`).

## Component Breakdown

- **Event model (`events` package, new, library)** — Owns the single flat `Event` (operation, phase, resource type/ID, file, duration, error, data, plus new `Time`, `Source` and generic `Meta`), the `Handler` receiver type, the `Emit` function type every emitter holds, and the published constants: reserved Meta keys (`level`, `message`), levels (debug/info/warn/error), phases (start/success/error/log/blocked), the `core` source and core operation names (parse, validate, apply, destroy, create, read, changed, update, discover, load, events). A leaf with only standard-library imports, so the parser, logger, plugins, registry and root package can all depend on it without cycles. Replaces the parser's duplicate event type.

- **slog adapter (`events.SlogHandler`, new, library)** — Converts each `Event` into a `log/slog` record on a caller-supplied `*slog.Logger`: log events at their own level with their message and details as attributes; lifecycle, discover and load events at Info (Error for error phases); blocked announcements at Warn. Severity filtering is the slog handler's level, so connecting it is one line. Consumes only the Event model.

- **Event stream (`internal/eventstream`, new, library)** — Owns delivery for one operation: a bounded queue sized by configuration, non-blocking emit when there is room, waiting (never dropping) when full, blocked-stretch tracking with a single out-of-band `blocked` announcement, a drain loop run on the caller's goroutine that calls the receiver one event at a time in emit order, and a discard mode used during panic shutdown. Stamps `Time` and a default `core` `Source` on events that lack them. Used only by the operation runner.

- **Operation runner (`Config`, changed, library)** — `Validate`, `Apply` and `Destroy` all go through one runner. It loads the plugin registry first, activates this operation's emitter on the registry, emits the operation's start and success/error events, and runs the work (state load, parse, walk, save) on a worker goroutine while the caller drains the event stream. With no receiver it runs the work inline and emits nowhere. On a receiver panic, a deferred shutdown cancels the operation context, switches the stream to discard, and waits for the worker to finish in-flight calls and save state, then lets the panic continue unchanged. It also owns the new `WithEventBufferSize` option, and no longer builds a default registry with a stdout logger or caches the address parser before plugins have loaded.

- **Logger (`logger` package, changed, library)** — Keeps the `Logger` interface and `WithTag`, and becomes a thin event builder. A logger holds an `Emit` plus a bound base event (source, operation, resource identity), and each call emits one `log`-phase event with level, message and key/value details in `Meta`. The reserved keys always win over caller details, and a `resource` tag fills in `ResourceID`. The stdout logger, the text formatting, the test logger and nil-logger fallbacks are removed. Used by the parser, registry, hosts, adapters and plugin authors.

- **Parser (`internal/parser`, changed, library)** — Emits through an `Emit` in its options instead of `OnParserEvent` and a `Logger`. Before each provider call it binds a logger for that resource and step into the call's context. Its `Validate`, `Apply` and `Destroy` take the operation's `context.Context`, and the walk callbacks check it before starting any provider call, handing providers a non-cancelling copy. It emits error events at the failure sites that emit nothing today, including validate-stage problems. Its configured-value warning becomes a bound warn log event. It stops changing the process-wide stdlib logger.

- **Plugin runtime (`plugins` package: plugin base, adapter, direct host, gRPC host/server/clients/callback, hclog adapter, proto; changed, library)** — Threads `ctx` through every provider-facing method. It provides `plugins.Logger(ctx)` so providers get the per-call bound logger without passing context themselves. Each host stamps its plugin name as the source. Across gRPC, the host maps a per-call ID (sent as gRPC metadata, returned on each `LogRequest`) back to the bound logger. Out-of-call logs from plugin init and go-plugin/hclog go to the registry's active emitter. It removes the "calling provider" log, the per-RPC `SetLogger` re-initialisation and the logger field on hosts. Consumed by the registry and parser.

- **Plugin registry (`plugins/registry`, changed, library)** — Holds no logger. `RegisterPlugin`, `RegisterPluginWithPath` and `DiscoverPlugins` only record. `RegisterType` keeps its immediate clash checks against builtin and registered types. A concurrency-safe `Load(emit)` discovers, starts and clash-checks plugins once per registry, reporting `discover`/`load` events to the loading operation, and caches the result. It also keeps the "active emitter" that out-of-call plugin logs use. Consumed by the operation runner, the parser and the file state store.

- **Shared errors (`errors` package, changed, library)** — Adds the sentinel and detail type for a plugin that fails to load (naming the plugin), re-exported from `xcl`, following the existing sentinel-plus-detail pattern.

- **Pretty example receiver (`example/prettylog`, new, examples) and example programs (changed)** — Replaces `example/eventlog`. It builds the slog adapter over a charmbracelet/log handler, styled so each line shows severity, source, and the resource and step where relevant. Every example program sets it up with one line, and no longer builds a logger or passes one to the registry. The example plugins (in-process and external) and the test plugins log through `plugins.Logger(ctx)`, and example tests assert on recorded events.

- **Library docs (README, `docs/`, `plugins/README_test_helpers.md`, CHANGELOG; changed)** — Describe the event stream and its fields, the slog adapter, logging from a plugin, lazy registration and loading, and the removal of the registry logger.

- **Docs site (`xcl-website`, changed)** — New pages for events and logging and for plugin logging and registration, linked from the navigation and the README pages table. Existing pages and snippets are updated to the new API, with no remaining references to passing a logger to the registry.

## Data Structures & Interfaces

**`events.Event`: the one flat event shape.** It extends today's `xcl.Event` with a time, a source and a generic details map. The root package re-exports it as `xcl.Event`. There are no other event types.

```go
package events

type Event struct {
    Time         time.Time      // when it happened; stamped at emit if zero
    Source       string         // "core", or the plugin's name
    Operation    string         // parse | validate | apply | destroy | create | read | changed | update | discover | load | events
    Phase        string         // start | success | error | log | blocked
    ResourceType string         // "<type>.<name>"
    ResourceID   string         // full address
    File         string         // file the resource was declared in
    Duration     time.Duration  // success/error, and the length of a blocked stretch
    Error        error          // error phase
    Data         []byte         // serialized resource, lifecycle events only
    Meta         map[string]any // details; log events carry KeyLevel and KeyMessage here
}

type Handler func(Event) // the receiver; called one event at a time, in emit order
type Emit    func(Event) // what every emitter holds; a nil Emit is silent

const (
    KeyLevel   = "level"
    KeyMessage = "message"
    SourceCore = "core"
    LevelDebug, LevelInfo, LevelWarn, LevelError = "debug", "info", "warn", "error"
    PhaseStart, PhaseSuccess, PhaseError, PhaseLog, PhaseBlocked = ...
    OperationParse, OperationValidate, OperationApply, OperationDestroy, OperationCreate,
    OperationRead, OperationChanged, OperationUpdate, OperationDiscover, OperationLoad,
    OperationEvents = ...
)

func SlogHandler(logger *slog.Logger) Handler // ready-made adapter to log/slog
```

**`xcl` public surface (root package).** `Event` and `EventHandler` become aliases of the `events` types, so `WithEventHandler` does not change. One option is new:

```go
type Event = events.Event
type EventHandler = events.Handler

func WithEventHandler(handler EventHandler) ConfigOption // unchanged
func WithEventBufferSize(size int) ConfigOption          // new; limits undelivered events, default DefaultEventBufferSize
const DefaultEventBufferSize = 1024

var ErrPluginLoad = xclerrors.ErrPluginLoad             // re-exported sentinel
type PluginLoadError = xclerrors.PluginLoadError        // detail: Plugin string; Err error
```

**`logger.Logger`: the logging API that stays.** Its only implementation emits one log event per call.

```go
package logger

type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}

func New(emit events.Emit, base events.Event) Logger  // base carries Source, Operation and resource identity
func WithTag(l Logger, key string, value any) Logger  // adds a detail; the "resource" key fills ResourceID
func Nop() Logger                                     // never emits; used when nothing is bound
```

**`plugins`: provider-facing context and the changed plugin contracts.**

```go
func Logger(ctx context.Context) logger.Logger                        // the bound per-call logger, Nop if none
func WithLogger(ctx context.Context, l logger.Logger) context.Context // used by the parser and the hosts

type Plugin interface {                 // SetLogger removed
    Init(logger Logger, state State) error  // logger is plugin-scoped and routes to the active emitter
    SetState(state State)
    PluginEntityProvider
}

type PluginEntityProvider interface {   // every method now takes ctx
    GetTypes() []RegisteredType
    Validate(ctx context.Context, entityType, entitySubType string, data []byte) error
    Create(ctx context.Context, entityType, entitySubType string, data []byte) ([]byte, error)
    Destroy(ctx context.Context, entityType, entitySubType string, data []byte) error // force stays out of scope
    Read(ctx context.Context, entityType, entitySubType string, old, new []byte) ([]byte, error)
    Update(ctx context.Context, entityType, entitySubType string, data []byte) ([]byte, error)
    Changed(ctx context.Context, entityType, entitySubType string, old, new []byte) (bool, error)
}
```

`ResourceProvider[T]` and `ProviderAdapter` keep their signatures. Each already takes `ctx` on every lifecycle method. The `logger` given to `Init` stays as the plugin-scoped logger for logs written outside a call.

**Host constructors** stop taking a logger. They take the emitter used for plugin-scoped logs, and hosts stamp their plugin name as `Source`:

```go
func NewDirectPluginHost(emit events.Emit, state State, plugin Plugin) (*DirectPluginHost, error)
func NewGRPCPluginHost(emit events.Emit, state State) *GRPCPluginHost
```

**`registry.PluginRegistry`: records plugins, loads them lazily, holds no logger.**

```go
func NewPluginRegistry() *PluginRegistry
func (r *PluginRegistry) RegisterType(name string, resource any) error         // immediate builtin/registered clash check
func (r *PluginRegistry) RegisterBareType(name string, resource any) error
func (r *PluginRegistry) RegisterPlugin(plugin plugins.Plugin) error           // records only
func (r *PluginRegistry) RegisterPluginWithPath(path string) error             // records only; missing path is not an error here
func (r *PluginRegistry) DiscoverPlugins(directories []string, pattern string) // records only
func (r *PluginRegistry) Load(emit events.Emit) error                          // once per registry; result cached
func (r *PluginRegistry) Activate(emit events.Emit) (deactivate func())        // route out-of-call plugin logs
```

**gRPC wire contract (`plugins/plugin.proto`).** `LogRequest` gains a call ID. The host sends the same ID as gRPC metadata (key `xcl-call-id`) on every `PluginService` call. Request messages are otherwise unchanged. Detail values still arrive as strings, which the spec lists as a non-goal.

```proto
message LogRequest {
  string message = 1;
  repeated string args = 2;
  string call_id = 3; // empty for logs written outside a provider call
}
```

**`internal/eventstream.Stream`: per-operation delivery, internal only.**

```go
func New(size int) *Stream
func (s *Stream) Emit(e events.Event)                            // waits only when full; never drops
func (s *Stream) Drain(receiver events.Handler, done <-chan struct{}) // caller's goroutine; returns when done and empty
func (s *Stream) Discard()                                       // panic shutdown: release emitters, deliver nothing
```

**`parser.ParserOptions`.** `Logger` and `OnParserEvent` are removed. It gains `Emit events.Emit`. `Parser.Validate`, `Parser.Apply` and `Parser.Destroy` take a leading `ctx context.Context`: the walk callbacks check `ctx.Err()` before every provider call, and pass `context.WithoutCancel(ctx)` (plus the bound logger) to the provider. `ParserEvent`, `fireParserEvent` and `fireParseEvent` are deleted in favour of emit helpers that build `events.Event`.

## Implementation Detail

**New pattern: one operation runner.** Today `Validate`, `Apply` and `Destroy` each build a parser and call it inline. They will all go through a single runner that owns the operation's lifetime. The runner:
- loads the registry;
- activates the operation's emitter on the registry, and deactivates it on return;
- emits the operation's start event;
- runs the work on a worker goroutine while the caller drains the stream;
- emits the success or error event carrying the returned error.

Each public method is then only the work closure it passes in. With no receiver the runner calls the closure inline with a nil emit, so the common case has no goroutine and allocates nothing. A reader following a failure or a panic therefore has one place to look. The runner's deferred shutdown does not recover. It notices that the drain did not complete, then cancels the operation context, discards and waits. This is a small, deliberate idiom, and it needs a comment explaining why `recover` is absent.

**New pattern: context carries the per-call logger.** Provider code stops storing a logger for use during calls and asks the context for one. The parser binds the resource and step, each host adds its plugin name as the source, and across gRPC the call ID is an internal detail of the wrapper and server. It never surfaces to plugin authors. Plugin code reads the same whether it runs in-process or as a separate process: `plugins.Logger(ctx).Info("created", "remote_id", id)`. The logger passed to `Init` stays, but its doc comment says it is only for messages written outside a call.

**Code-shape changes.**
- **Events leave the parser for a leaf package.** Both the parser's event type and its fire helpers go. Emission is done by small helpers that build an `events.Event` from a resource's meta. These helpers stamp resource type, ID and file consistently, so no site assembles the fields by hand.
- **The `logger` package shrinks to an event builder.** Its formatting, stdout and test implementations go. Library code that used to hold a `Logger` holds an `Emit` instead, and builds a logger only where it writes messages.
- **The registry splits into "record" and "load".** Registration methods become cheap bookkeeping. The current eager code (host construction, starting processes, clash checks, discovery) moves almost unchanged into `Load`, behind a mutex and a cached result. Reads of types and hosts take a read lock, because loading may now happen while another Config shares the registry.
- **`ctx` is threaded end to end** through the plugin base, hosts, the gRPC wrapper, server and adapters. Several of these drop it today, so this is a wide but mechanical signature change. Mocks are regenerated afterwards.
- **The walks gain an operation context.** The runner creates a cancellable context per operation and passes it into the parser. Walk callbacks check it just before each provider call, and providers get a non-cancelling copy so a running call is never aborted. A resource skipped because the context was cancelled is treated like one that was never reached, so the existing partial-state builder and per-resource destroy saves record exactly what was created. The vendored dag walker is not modified.
- **Error sites become emit sites.** Every failure path in the walk callbacks, validation, plugin loading and the state save emits an error event with the same error it returns. The rule "an error returned is an error emitted" can then be checked at each return.

**Existing patterns followed.** Error types follow the shared `errors` package's sentinel-plus-detail pattern and are re-exported from `xcl`. Options stay functional `ConfigOption`s. Tests keep the event-recorder style already used in the root event tests: a mutex-guarded recorder with `find` helpers and `require` assertions, one behaviour per test. The example programs keep their `run(...)` shape, but take an `EventHandler` instead of a `Logger`, so tests inject a recorder and `main` injects the pretty receiver in one line.

**Developer experience after the change.**
- **Application authors** see one option (`WithEventHandler(events.SlogHandler(logger))`) and one optional tuning knob, the buffer size.
- **Plugin authors** see `plugins.Logger(ctx)`, and nothing about resources or steps to pass themselves.
- **Contributors** see no `logger` fields threaded through constructors. Anything that reports something emits an event, and nothing can print.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references (`design ref list` reported 0), and the user declined a design document during the spec interview.
- **Spec `20260922061954-event-based-logging`** is the source of the requirements and acceptance criteria. It is final and closed, so nothing needs to land first.
- **Prior plans.** None have to land first. The query-api-v2 and destroy-cycle work is already on `main` (HEAD `51f1c0b`). This plan changes the log line format that destroy-cycle introduced (`event=<name>`), and the `CHANGELOG` records that.
- **Go standard library `log/slog`** (Go 1.21+; the module declares `go 1.25.0`) is the target of the shipped adapter. It needs no change and adds no dependency.
- **`github.com/charmbracelet/log` v0.4.2** implements `slog.Handler`, and the pretty example receiver uses it as its handler. It stays pinned in `go.mod`. It moves from a library import (the stdout logger) to example-only imports, and needs no version change.
- **`github.com/hashicorp/go-plugin` v1.6.3** is the external plugin process, broker and stderr capture. It is used as-is. Its host-side logger is still fed by xcl's hclog adapter, which is retargeted to emit events. No upgrade.
- **`github.com/hashicorp/go-hclog` v0.14.1** is required by go-plugin's `ClientConfig.Logger`. It is kept only for the adapter, with no version change.
- **`google.golang.org/grpc` v1.73.0 (`metadata` package) and `google.golang.org/protobuf` v1.36.6** carry the per-call ID as gRPC metadata, plus the new `LogRequest.call_id` field. Existing versions are used.
- **Build tooling: `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc`** regenerate `plugins/proto` from `plugins/plugin.proto` through the Makefile `protos` target. All are installed locally. The generated files are committed.
- **Build tooling: `mockery`** regenerates the `ProviderAdapter`, `ProviderResolver`, `State` and `StateStore` mocks through the Makefile mocks target after the signature changes. It is installed locally.
- **Internal `internal/dag`** (the vendored walker) is used unchanged. The cancellation check lives in the parser callbacks, so the vendored code is not modified.
- **Internal `errors` package** gains the plugin-load sentinel and detail type. Its thin import list must be kept.
- **Internal `state` (`FileStateStore`)** holds the registry and calls `CreateResource` from `Load`. It has no code change, but it depends on the runner loading plugins before state is loaded.
- **Docs site repo `xcl-website`** (Astro 5 + MDX, Node 22+) gets new and updated pages. It depends on the library API from this plan being settled, so the docs phase comes last. It is checked with the site's Astro type-check and build.

## Testing Approach

**Test types.**
- **Unit tests** cover the new leaf pieces: the event model and slog adapter, the logger-as-event-builder, the event stream's queueing, blocking and ordering, and the registry's record and load split.
- **Integration tests at the `Config` level** cover the behaviour the spec actually promises. They go through `Validate`/`Apply`/`Destroy` with the parser's `TestPlugin` in process, and with the built example plugin binary as an external process.
- **End-to-end example tests** run each example program's `run(...)` with a recording receiver.
- **Static source checks** are small Go tests that parse the repository's own sources. They enforce the "nothing prints" and "examples use one receiver" metrics, so a regression fails the test suite rather than relying on review.

All tests follow the project conventions: testify `require`, no table-driven tests, positive and negative cases in separate tests, and verbose, descriptive test names. They reuse the existing mutex-guarded event-recorder style. Prior state for destroy and shutdown tests comes from a real apply, never a hand-written state file.

**Most coverage goes to the event stream and the operation runner**, because they carry the hardest guarantees. Load-bearing assertions:
- **Emitting does not wait for the receiver.** With a one-second receiver and a large buffer, the last provider call finishes long before N seconds have passed.
- **When the buffer is full, emitters wait and nothing is dropped.** With a buffer of 1 and a slow receiver, the count of delivered events equals the count with a fast receiver.
- **One blocked stretch gives exactly one `blocked` event** reporting the stretch's duration, and that event never takes a queue slot.
- **The receiver is never entered concurrently**, and each resource's events arrive in start → log → success/error order.
- **Every event is delivered before `Validate`/`Apply`/`Destroy` returns, error paths included.** Each event's time falls within the call.
- **A receiver that panics on its third event makes `Apply` panic with the same value.** A recovering caller's stack trace contains the receiver's function. State records exactly the resources whose create finished, and no provider call starts after the panic. This test uses several independent resources with provider-side call recording.
- **Errors still stop processing, with or without a receiver.** Dependants of a failed resource are not created, and the error event carries the same failure `Apply` returned.
- **A slow receiver, or one that ignores every event, does not change the result.** Created resources and the returned error match a run with no receiver.

**The plugin boundary is tested symmetrically.** The same provider logic runs as an in-process plugin and as the external example binary. Tests assert that log events from create and read carry the same severity, message, detail names, detail values rendered as text, resource ID, type, file and step. Only `Source` may differ. Registry tests cover:
- construction and registration with no logger;
- a missing external plugin path accepted at registration, then failing the first operation with an error naming it;
- no plugin process running before the first operation;
- one process start across two Configs sharing a registry;
- per-Config routing of plugin log events;
- immediate clash errors for builtin and registered types, deferred clash errors for plugin types;
- discovery, load and rejection events for a directory holding one valid and one invalid plugin.

**The global side-effect regression test** writes with the standard library logger after each operation and asserts the message reaches its original destination.

**Existing tests that assert on log text are rewritten, not deleted.** This covers the configured-value warning, plugin e2e log tests, example log tests and hclog adapter tests. They assert on recorded events with the same intent: the warning becomes a warn log event bound to the resource and step, and "plugin loaded" becomes a `load` success event. Tests of deleted code (stdout logger formatting, event-first text layout, test logger) are removed along with that code.

**Spec success metrics → verification.**
- **Direct log calls in xcl's own code, outside the logging wrapper itself: 0.** Behavioural test (static): a source-scanning test over every non-test, non-example library package asserts there are no stdlib `log` imports, no `fmt.Print*`, and no `os.Stdout`/`os.Stderr` references. It also asserts that `logger` implementations appear only in the `logger` package. Vendored `internal/xcl` and `internal/cty` and the unused resource printer are excluded, and each exclusion is listed explicitly in the test.
- **Output from xcl when no receiver is set: 0 bytes to standard output or standard error, across the full example suite.** Behavioural test: each example's `run(...)` is executed with no receiver while the process's stdout and stderr file descriptors are redirected to pipes. This includes the plugin example with both in-process and external plugins, through validate, apply and destroy. The test asserts both pipes stay empty. The program's own report goes to the injected writer, not to stdout.
- **Every example runs its whole output through one receiver set up with a single line, and none builds or passes a logger for the plugin registry.** Behavioural test (static): a source check over every example `main.go` asserts exactly one `WithEventHandler` call using the pretty receiver, no import of the `logger` package, and a `NewPluginRegistry()` call with no arguments. The example tests also assert that plugin log messages arrive at the injected receiver.
- **Events lost in normal operation or when the buffer fills: 0.** Behavioural test: the buffer-of-1 slow-receiver count equals the fast-receiver count, and a normal run's event count matches the expected per-resource sequence.
- **Example plugins that log with resource context: all of them do so without passing the context themselves.** Behavioural test: the example tests assert that every provider log event from the in-process and external example plugins carries resource ID, type, file and step. A static check asserts that no example provider log call passes a `resource` detail.

**Deliberate gaps.**
- **The styled appearance of the pretty example receiver** (colours, layout) is Manual — captured in the implementation test plan. Automated tests only assert that it is the receiver every example uses and that it produces a line for every event.
- **The docs site pages' content and rendering** are Manual — captured in the implementation test plan. The site must type-check and build, and a text search asserts there are no remaining references to passing a logger to the registry.
- No new tests are added to the vendored dag walker, because it is not modified.

## Milestones & Phases

### Milestone 1: xcl reports its own work through one silent, ordered event stream

**What changes**: Applications that call `Validate`, `Apply` or `Destroy` receive xcl's own activity as one stream of events through the existing `WithEventHandler`. Each event now carries a timestamp, a source (`core`), and a details map. There are also operation-level start and finish events, and an error event for every failure the call returns. The receiver is fed one event at a time, in order, from a bounded buffer the application can size. xcl never waits for the receiver to process an event. When the buffer is full, emitters wait rather than drop events, and a single "blocked" event announces how long they waited. Everything is delivered before the call returns. A receiver that panics no longer crashes a walk half-way through. xcl starts no new provider calls, lets running ones finish, and saves state, and the application then sees the receiver's own panic raised from the call. xcl stops changing the process-wide standard logger and never writes to standard output itself. A ready-made adapter connects the stream to a `log/slog` logger in one line. Plugin log messages still go through the old logger in this milestone.

**Validation point**: The root-level event, delivery, ordering, blocking, panic-shutdown and global-logger tests pass with the in-process test plugin. The slog adapter test shows an "info" logger receiving lifecycle events but no debug log events. The static scan finds no stdlib `log` usage or `log.SetOutput` in library code. The full test suite passes.

#### - [ ] Phase 1.1: One event shape and a ready-made slog adapter
**Repo:** xclconfig

This phase introduces a small shared package that defines the single flat event every part of xcl will use. The event gains a time, a source and a details map, and the package publishes the reserved detail names and the standard level, phase and operation names. The package also ships the adapter that connects the stream to a standard-library `slog` logger. The root package's `Event` and `EventHandler` become aliases of the new types, so existing receivers keep compiling. The logger package gains an implementation that turns each log call into a log event. The old implementations are left in place until Milestone 2 so that nothing breaks in between.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-one-event-shape-and-a-ready-made-slog-adapter)

**Acceptance criteria**:
- [ ] Application code that uses `xcl.Event` and `xcl.WithEventHandler` compiles unchanged, and its events now carry a time, a source and a details map.
- [ ] A log message written with severity, text and details becomes one event with phase `log`. The severity and text are under the published reserved names and the details keep their names, and a caller detail named `level` or `message` cannot overwrite them.
- [ ] An adapter connected to an "info" `slog` logger writes info, warn and error log events plus lifecycle events, and writes no debug log events.
- [ ] The new package depends only on the standard library.

#### - [ ] Phase 1.2: Bounded, ordered, fire-and-forget delivery
**Repo:** xclconfig

This phase builds the internal delivery queue used by one operation. Emitting never waits for the receiver. It waits only when the configured limit of undelivered events is reached, and never drops an event. One stretch of waiting is announced by a single "blocked" event that states how long it lasted and never takes a queue slot. The receiver is called one event at a time and in order, from a drain loop that the caller runs. A discard mode releases waiting emitters during an emergency shutdown.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-bounded-ordered-fire-and-forget-delivery)

**Acceptance criteria**:
- [ ] With room in the buffer, emitting returns immediately even when the receiver is slow.
- [ ] With a limit of 1 and a slow receiver, every emitted event is delivered.
- [ ] A stretch of blocked emitting produces exactly one "blocked" event carrying its duration.
- [ ] Events arrive in the order they were emitted, and the receiver is never entered concurrently.
- [ ] Every event has a time and a source, and the source defaults to `core`.

#### - [ ] Phase 1.3: The parser emits events, reports every failure, and can be cancelled
**Repo:** xclconfig

This phase replaces the parser's private event type, callback and logger with the shared emitter. Every event it produces then carries a time, a source, and the resource's type, identity and file. Failures that are returned today without any event now also emit an error event. These are a missing provider, serialisation and decode problems, and validation problems. The configured-value warning becomes a warn log event bound to the resource and step. The walks take an operation context, and once it is cancelled no new provider call starts; calls already running are not interrupted. The walks also stop silencing the application's standard logger.

*Technical detail:* [context.md#phase-13](./context.md#phase-13-the-parser-emits-events-reports-every-failure-and-can-be-cancelled)

**Acceptance criteria**:
- [ ] Parse, create, read, changed, update and destroy events reach the emitter with the same operations and phases as before, plus time, source `core` and the declaring file.
- [ ] Every error the parser returns from a walk or from validation has a matching error event.
- [ ] When a provider changes a configured value, one warn log event names the resource, the field and the step, and nothing is printed.
- [ ] Once the operation's context is cancelled, no further provider call starts, calls already running finish normally, and resources already created are still recorded.
- [ ] A message written with the standard library logger after a walk appears at its original destination.

#### - [ ] Phase 1.4: One operation runner with graceful receiver-panic shutdown
**Repo:** xclconfig

This phase routes `Validate`, `Apply` and `Destroy` through one runner. The runner emits operation-level start and finish events and does the work on a worker goroutine while the caller delivers events to the receiver. Everything is delivered before the call returns, and the buffer limit can be set with a new option. If the receiver panics, the runner stops new provider calls, lets running ones finish and saves state. It then lets the receiver's own panic continue, with its original value and stack. With no receiver, the work runs directly and xcl is silent, and the default registry no longer carries a stdout logger.

*Technical detail:* [context.md#phase-14](./context.md#phase-14-one-operation-runner-with-graceful-receiver-panic-shutdown)

**Acceptance criteria**:
- [ ] When validate, apply or destroy returns, including with an error, the receiver has already received every event for that call, including the operation's error event.
- [ ] With a one-second receiver and a large buffer, provider calls finish well before the receiver has processed every event, and apply still returns only after the last one is handled.
- [ ] A receiver that panics on its third event makes apply panic with that same value. The recovered stack trace contains the receiver's code, state holds exactly the resources whose create finished, and no provider call starts after the panic.
- [ ] A failing provider stops dependants and returns the same error with or without a receiver, and a slow receiver or one that ignores events does not change the outcome.
- [ ] An application can set the buffer limit, and leaving it unset uses the documented default.
- [ ] With no receiver set, an apply with the in-process test plugin writes nothing to standard output or standard error.

### Milestone 2: Plugins log into the stream with resource context, and load when first needed

**What changes**: Plugin authors keep writing ordinary `Info`/`Debug`/`Warn`/`Error` messages, obtained from the call's context. Each message arrives in the application's receiver as a log event carrying its severity, message and details, and names the plugin as its source. It is also labelled with the resource, its type, its file and the lifecycle step, and the plugin author passes none of these. This works the same for in-process plugins and for plugins running as separate processes. Messages go only to the configuration whose operation produced them. The plugin registry no longer takes a logger. Registering a plugin only records it. Plugins are discovered and loaded once per registry, on the first validate, apply or destroy, and discovery, loading and rejection are reported as events. A plugin that fails to load fails that operation with an error naming it. The duplicate "calling provider" message disappears, and xcl's library code contains no logger that can print.

**Validation point**: The in-process and external symmetry tests pass: same severity, message, details and context, with only the source differing. So do the lazy-load, load-once, per-Config routing, deferred-clash and discovery-event tests, and the full no-output test with both plugin kinds and no receiver. The static scan confirms that no logger implementation exists outside the `logger` package. The full test suite passes, including the external plugin e2e tests.

#### - [ ] Phase 2.1: Plugin authors log from the call's context
**Repo:** xclconfig

This phase finishes turning the logger package into a pure event builder: the stdout, text-formatting and test loggers are removed, so nothing in the library can print. Provider calls now carry a context all the way through the plugin base and the in-process host. Before each call, the parser binds a logger to the resource and step, and the host stamps the plugin's name as its source. Plugin authors fetch that logger from the context with one call. The per-call re-initialisation and the duplicate "calling provider" message are removed. The in-process test plugins switch to the new style.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-plugin-authors-log-from-the-calls-context)

**Acceptance criteria**:
- [ ] An in-process provider that logs "something happened" at info with `remote_id=213` during create produces one event carrying that severity, text and detail. The event is sourced to the plugin and names the resource ID, type, file and the step `create`. The same holds for read.
- [ ] For a provider that logs during create, the receiver sees create start, then that log event marked as create, then create success.
- [ ] Applying one resource gives exactly one create start event and no log event restating it.
- [ ] Log messages at debug, info, warn and error all reach the receiver with their severity.
- [ ] No library package contains a logger that writes anywhere other than the event stream.

#### - [ ] Phase 2.2: The plugin registry records plugins and loads them on first use
**Repo:** xclconfig

This phase removes the logger from the plugin registry and splits registration from loading. Registering a plugin, a plugin path or a discovery directory only records it. On the first validate, apply or destroy, the registry discovers, starts and clash-checks plugins, once for every Config that shares it. It reports discovery, loading and rejection as events to that operation. Registering a type that clashes with a builtin or already-registered type still fails immediately, and clashes with plugin types surface when plugins load. A plugin that fails to load fails the operation with an error that names it.

*Technical detail:* [context.md#phase-22](./context.md#phase-22-the-plugin-registry-records-plugins-and-loads-them-on-first-use)

**Acceptance criteria**:
- [ ] A registry can be created, have plugins registered and have discovery directories added without any logger.
- [ ] Registering an external plugin path that does not exist returns no error. The first validate fails with an error naming the plugin, and no plugin process runs before that call.
- [ ] Two Configs sharing one registry and each validated start each external plugin process only once.
- [ ] A type named like a builtin or an already-registered type is rejected at registration. A type named like a plugin's type is accepted at registration, and the first validate fails with a clash error.
- [ ] Validating with a discovery directory holding one valid and one invalid plugin gives the receiver events for the discovery, the successful load and the rejection.

#### - [ ] Phase 2.3: The same logging across the plugin process boundary
**Repo:** xclconfig

This phase brings plugins that run as separate processes up to the same behaviour as in-process ones. Each provider call carries a call identifier to the plugin, so log messages the plugin writes during that call come back labelled with it. The host then attributes each message to the right resource, step and configuration. go-plugin's own messages and the plugin's initialisation logs become events on the stream of the operation currently running, instead of being written anywhere. The context is threaded through the gRPC client and server, which no longer re-initialise providers on every call.

*Technical detail:* [context.md#phase-23](./context.md#phase-23-the-same-logging-across-the-plugin-process-boundary)

**Acceptance criteria**:
- [ ] The same provider logic run in-process and as an external plugin produces log events with the same severity, message, detail names, detail values as text, resource context and step. Only the source differs.
- [ ] With two Configs sharing a registry, each with its own receiver, each receiver gets only the plugin log events for its own operations.
- [ ] Validating, applying and destroying with an in-process and an external plugin and no receiver writes nothing to standard output or standard error, and creates no log files.
- [ ] With a receiver set, every message the external plugin logs appears in the receiver and nothing appears anywhere else.

### Milestone 3: Examples and documentation show the new way to watch xcl

**What changes**: Every example program shows its lifecycle events, plugin log messages and errors through one styled terminal receiver, set up in a single line. The receiver is built on the slog adapter with a charmbracelet handler, and no example builds a logger or passes one to the plugin registry. The example plugins log without passing resource context. The library README and docs, and the xcl.dev documentation site, explain the event stream and its fields, how to connect it to a logger, how plugin authors write log messages, and the lazy plugin registration and loading model. No page mentions passing a logger to the registry any more.

**Validation point**: The example tests and the static example check pass. Running each example shows styled lines with severity, source, and resource and step where relevant, which is confirmed by hand. The docs site type-checks and builds, and a search of both repositories finds no `NewPluginRegistry(` call with a logger argument.

#### - [ ] Phase 3.1: A styled example receiver used by every example
**Repo:** xclconfig

This phase replaces the example event logger with a styled terminal receiver built on the slog adapter and a charmbracelet handler. Every example program sets it up in one line and no longer builds or passes a logger. The example plugins log from the call's context without adding resource details themselves. The example tests are rewritten to assert on recorded events, and source checks lock in the "silent library" and "one receiver per example" success metrics.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-a-styled-example-receiver-used-by-every-example)

**Acceptance criteria**:
- [ ] Running any example shows its lifecycle events, plugin log messages and errors through the styled receiver, each line showing severity, source, and resource and step where relevant.
- [ ] No example program builds a logger or passes one to the plugin registry, and each sets up its receiver in one line.
- [ ] Every log event from the example plugins carries resource context that the plugin code does not pass itself.
- [ ] The library's own packages still do not import the charmbracelet logger.
- [ ] Across the full example suite, xcl writes nothing to standard output or standard error when no receiver is set.

#### - [ ] Phase 3.2: Library documentation describes the event stream
**Repo:** xclconfig

This phase updates the README, the docs folder, the plugin test-helper notes and the changelog. They now describe the event stream and its fields, the slog adapter and the buffer option, logging from a plugin through the call's context, and the lazy registration and loading model. All mention of passing a logger to the registry, and of the old log line format, is removed.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-library-documentation-describes-the-event-stream)

**Acceptance criteria**:
- [ ] A reader of the README and docs can connect xcl to a slog logger, write a log message from a plugin, and understand when plugins load, without reading code.
- [ ] No library document references passing a logger to the plugin registry, the stdout logger, or the "calling provider" message.
- [ ] The changelog records the breaking API changes.

#### - [ ] Phase 3.3: The documentation site covers events, logging and plugin loading
**Repo:** xcl-website

This phase adds pages to xcl.dev for the event stream and connecting it to a logger, and for logging from a plugin and plugin registration and lazy loading. It links the new pages from the navigation. It updates the home page and the three example pages to the new API and output. No page mentions passing a logger to the registry any more.

*Technical detail:* [context.md#phase-33](./context.md#phase-33-the-documentation-site-covers-events-logging-and-plugin-loading)

**Acceptance criteria**:
- [ ] The site has pages covering the event stream and its contents, connecting it to a logger, logging from a plugin, and plugin registration and lazy loading, all reachable from the navigation.
- [ ] Code shown on the example pages matches the example programs, and the sample output matches the styled receiver.
- [ ] No page references passing a logger to the plugin registry.
- [ ] The site type-checks and builds.

## Open Questions

- **Does go-plugin log anything to the host process's own stderr, outside the `ClientConfig.Logger` we supply?** Examples would be process-exit notices written after the host logger is replaced, or output from `plugin.CleanupClients`. This depends on go-plugin v1.6.3 runtime behaviour, which the no-output test with an external plugin will show. *If the test finds bytes on stderr that xcl cannot route through the supplied logger*, STOP and ask the user whether to set `ClientConfig.Stderr`/`SyncStderr` to a writer that emits events, or to accept the output as a documented exception.
- **Does the in-process `TestPlugin` make overlapping creates reliable enough for the panic-shutdown test to be deterministic?** The test needs a panic to land while some creates are in flight and others have not started. Running the test repeatedly under `-race` will show whether it is. *If it proves flaky*, add a synchronisation hook to `TestPlugin`, such as a create that blocks on a channel the test controls, rather than relying on sleeps. This is an implementation detail and needs no user input.

## Out of Scope

- **Declaring plugins in configuration files.** Plugins are still registered in code. Lazy loading prepares for configuration-based registration, which is the intended next spec (spec Non-Goals).
- **Supported adapters for loggers other than `log/slog`.** The charmbracelet receiver is only an example. Applications that use other loggers write their own receiver (spec Non-Goals).
- **More than one receiver per configuration.** Applications fan events out from their own receiver (spec Non-Goals).
- **Keeping value types across the plugin process boundary.** Detail values from external plugins may arrive as text. Their names are kept, and the host's existing best-effort re-typing stays (spec Non-Goals).
- **Tracing and metrics** such as OpenTelemetry spans or counters derived from events (spec Non-Goals).
- **Storing or replaying events.** Events are delivered live only (spec Non-Goals).
- **Handling panics raised by providers.** Only receiver panics get graceful shutdown. A provider panic on a walker goroutine behaves as it does today. This is a deliberate design boundary.
- **Passing the `force` flag through the plugin-level `Destroy` and the gRPC `DestroyRequest`.** This is an existing gap noticed during research, and it is unrelated to events.
- **The unused resource printer in the logger package** (`ResourcePrinter`). It is not logging and only prints when called explicitly. It is left untouched, and a later clean-up can remove it.
- **Structured, typed values over gRPC**, for example changing `LogRequest.args` to a typed or JSON payload. Only a call ID is added to the wire contract.
- **A line number on events.** Resource context is ID, type and declaring file, as the spec asks.
