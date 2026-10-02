---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Context: 20260922061954-event-based-logging

## Current State Analysis

- **Two parallel reporting systems.** Lifecycle events go through `xcl.Event`/`WithEventHandler` (`events.go:11-67`, `options.go:32`), converted from the parser's duplicate `ParserEvent` (`internal/parser/events.go:6-55`). Diagnostics go through a separate `logger.Logger` (`logger/logger.go:4-9`), passed into the plugin registry (`plugins/registry/plugin_registry.go:29`), discovery (`plugin_discovery.go:20`), hosts (`plugins/direct_plugin_host.go:16`, `plugins/grpc_plugin_host.go:27`) and providers (`plugins/adapter.go:86-95`). Log calls smuggle `"event", "<name>"` pairs that `logger/event.go` pulls back out to put in front of the text line.
- **xcl prints by default.** `NewConfig` builds a registry with `logger.NewStdOutLogger()` (`config.go:116`), and `DefaultOptions` does the same (`internal/parser/parser.go:119`). `NewParser` falls back to a zero `StdOutLogger` whose inner logger is nil (`parser.go:166-169`). That is a latent nil dereference: the configured-value warning (`internal/parser/configured_check.go:23`) would hit it through `Config.Apply`.
- **Process-wide side effect.** `log.SetOutput(io.Discard)` is called at `internal/parser/parser.go:1269` and `internal/parser/destroy.go:69`. It serves no purpose, because `internal/dag` never uses the stdlib `log` package.
- **Handler concurrency.** The handler is called synchronously and concurrently from dag walker goroutines (`internal/dag/walk.go:293-300,377`) with no lock. The `EventHandler` doc says so (`events.go:42-47`).
- **No cancellation.** A failure only skips its dependants through `waitDeps` (`walk.go:402-440`). Independent branches keep calling providers. Every provider call uses `context.Background()` (`internal/parser/lifecycle.go:121,174,200,215,246`; `callbacks.go:259`). Nothing calls `recover()`.
- **Silent error sites.** Missing provider (`lifecycle.go:80-83`; `callbacks.go:225-238`), marshal failure (`callbacks.go:241-254`), decode, context and disabled-evaluation errors in `walkCallback`, and validate-stage problems all return errors without emitting an event.
- **Duplicate message.** `TypedProviderAdapter.debugCall` logs "calling provider" (`plugins/adapter.go:101-112`) alongside the lifecycle start event.
- **Eager registry.** `RegisterPlugin` initialises the plugin immediately (`plugin_registry.go:308-325`). `RegisterPluginWithPath` spawns the process (`:328-349`). `DiscoverAndLoadPlugins(logger, dirs, pattern)` scans and loads at once (`:352-404`). The registry has no mutex, even though it is shared across Configs (`options.go:13`) and read from walker goroutines.
- **Plugin logger plumbing.** In-process providers keep the logger from `Init`. External plugins re-run `SetLogger` → `Adapter.Init` on every RPC (`plugins/grpc_server.go:43-168`, `plugins/plugin.go:88-99`), which races under concurrent calls. Log RPCs carry only `message` and stringified `args` (`plugins/plugin.proto` `LogRequest`). The host callback forwards to one fixed logger per plugin (`plugins/grpc_host_callback.go:28-54`), so it cannot tell which call or Config a log belongs to. go-plugin's own output reaches the host through `newHCLogAdapter` (`plugins/grpc_plugin_host.go:54`). Plugin stdout and stderr after `Serve` are discarded.
- **Examples.** Every example builds `logger.NewStdOutLogger()`, passes it to `NewPluginRegistry(log)`, and logs events through `example/eventlog`. Their tests assert on tagged text log lines.

## Per-Phase Technical Notes

### Requirement → repo and files

All requirements except "Docs site updated" are carried out in **xclconfig** (root `/home/nicj/code/github.com/jumppad-labs/xcl`). "Docs site updated" is carried out in **xcl-website** (root `/home/nicj/code/github.com/jumppad-labs/xcl-website`).

| Requirement | Phase(s) | Main files |
|---|---|---|
| Silent by default | 1.4, 2.1, 2.3, 3.1 | `config.go`, `logger/*`, `plugins/hclog_adapter.go`, `plugins/grpc_plugin_host.go` |
| No global side effects | 1.3 | `internal/parser/parser.go:1269`, `internal/parser/destroy.go:69` |
| One stream for everything | 1.4, 2.1–2.3 | `config.go`, `plugins/*`, `plugins/registry/*` |
| Ready-made logger adapter | 1.1 | `events/slog.go` (new) |
| Pretty logger example | 3.1 | `example/prettylog/` (new), `example/*/main.go` |
| Log calls become events; severity carried; arbitrary detail; timestamp; source | 1.1, 1.2, 2.1 | `events/`, `logger/`, `internal/eventstream/` |
| No duplicate messages | 2.1 | `plugins/adapter.go:101-112` |
| Resource and step bound; works across boundary; logs sit within step | 2.1, 2.3 | `internal/parser/lifecycle.go:321-350`, `internal/parser/callbacks.go:240-280`, `plugins/*` |
| Registry needs no logging; discovery/loading as events; lazy; once; routing; immediate checks | 2.2, 2.3 | `plugins/registry/*`, `config.go`, `errors/` |
| Fire and forget; bounded; block don't drop; blocking announced; ordered; delivered before return | 1.2, 1.4 | `internal/eventstream/`, `config.go`, `options.go` |
| Errors still stop; errors are events; receiver cannot affect; panics not absorbed; graceful shutdown | 1.3, 1.4 | `internal/parser/*`, `config.go` |
| Docs site updated | 3.3 | `xcl-website:src/pages/*`, `xcl-website:src/components/Nav.astro` |

### Phase 1.1: One event shape and a ready-made slog adapter

**File changes**
- **New `events/events.go`:**
  - `Event` holds today's 8 fields from `events.go:12-41`, plus `Time time.Time`, `Source string` and `Meta map[string]any`.
  - Types `Handler func(Event)` and `Emit func(Event)`.
  - Constants: `KeyLevel`/`KeyMessage`, `SourceCore`, the `Level*`, `Phase*` (start/success/error/log/blocked) and `Operation*` names (parse, validate, apply, destroy, create, read, changed, update, discover, load, events).
  - Doc comments state the reserved-key rule and that `Data` is lifecycle-only.
  - Imports: stdlib only (`time`, `log/slog`, `context`).
- **New `events/slog.go`:** `SlogHandler(logger *slog.Logger) Handler`.
  - Log phase: `Meta[KeyLevel]` maps to `slog.LevelDebug`/`Info`/`Warn`/`Error`. The message is `Meta[KeyMessage]`. The remaining Meta entries become attrs in sorted key order, for stable output.
  - Error phase: `LevelError`. Blocked: `LevelWarn`. Everything else: `LevelInfo`. The message is `"<operation> <phase>"`.
  - Common attrs: `source`, `operation`, `phase`, `resource` (ID), `type`, `file`, `duration` (if non-zero), `error` (if set), and `time` via `slog.Record` time = `e.Time`.
  - It uses `logger.Handler().Enabled` before building the record, so filtering is cheap.
  - It does not attach `Data`.
- **`events.go` (root):**
  - Replace the struct and the `EventHandler` type with `type Event = events.Event` and `type EventHandler = events.Handler`.
  - Keep `parserEventHandler` until 1.3 removes it. In this phase, adapt it to build `events.Event` with `Time: time.Now()` and `Source: events.SourceCore`, and to copy `File`.
  - Update the `EventHandler` doc: it is called one event at a time (this becomes true in 1.4; write the final wording now).
- **`logger/logger.go`:** change `interface{}` to `any` (conventions).
- **New `logger/emit.go`:** additive, and the old implementations stay until 2.1.
  - `New(emit events.Emit, base events.Event) Logger` builds an `eventLogger{emit, base, tags map[string]any}`.
  - Each method copies `base`, sets `Phase: events.PhaseLog`, and fills `Meta` with the tags, then the args paired as key/value. An odd trailing arg goes under `"!BADKEY"`, as slog does, and a non-string key is converted with `fmt.Sprint`. `Meta[KeyLevel]` and `Meta[KeyMessage]` are written **last**, so they win.
  - A nil `emit` gives a no-op.
- **`logger/tagged.go`:** `WithTag` on an `*eventLogger` returns a copy with the tag added. The key `"resource"` sets `base.ResourceID` instead of a Meta entry. The existing text-tagging behaviour for other `Logger` implementations stays until 2.1. Also add `Nop()`.

**Tests**
- New `events/slog_test.go`, one test per behaviour:
  - an info slog drops debug log events;
  - an info slog writes info/warn/error log events;
  - an info slog writes lifecycle start and success events at info;
  - error-phase events are written at error;
  - details become attrs.
  Use `slog.NewJSONHandler` over a `bytes.Buffer` and decode the lines.
- New `logger/emit_test.go`:
  - level and message go into Meta;
  - details keep their names;
  - a detail named `level` does not override the level;
  - a detail named `message` does not override the message;
  - a resource tag fills ResourceID;
  - a nil emit is silent;
  - the phase is `log`.

**Complexity:** Low · **Token estimate:** ~25k · **Agent strategy:** Single agent, sequential.

### Phase 1.2: Bounded, ordered, fire-and-forget delivery

**File changes**
- **New `internal/eventstream/stream.go`:** `Stream{queue chan events.Event; mu sync.Mutex; waiting int; stretchStart time.Time; blocked *events.Event /* one-slot side channel */; discard atomic.Bool; wake chan struct{} /* cap 1 */}`.
- **`New(size int)`:** a `size < 1` is treated as 1.
- **`Emit(e)`:**
  1. Stamp `e.Time` if it is zero and `e.Source = events.SourceCore` if it is empty.
  2. If `discard` is set, return.
  3. `select { case queue <- e: return; default: }`.
  4. Otherwise, under `mu`: `waiting++`, and set `stretchStart = now` if this is the first waiter. Then do a blocking `select { case queue <- e: case <-discardCh: return }`.
  5. Under `mu`: `waiting--`. If it reaches 0, store a pending blocked event (`Operation: events.OperationEvents`, `Phase: events.PhaseBlocked`, `Duration: since(stretchStart)`, `Source: core`) in `blocked`, then do a non-blocking send on `wake`.
  6. A stretch is from the first waiter to the moment no emitter is waiting, which gives one announcement per stretch.
- **`Drain(receiver, done <-chan struct{})`:** loop.
  - Deliver the pending `blocked` event first if set, clearing it under `mu`.
  - Then `select` on `queue`, `wake` and `done`.
  - When `done` is closed, drain the remaining queue and any pending blocked event, then return.
  - It never runs the receiver while holding `mu`.
- **`Discard()`:** set the flag and close `discardCh` (with a `sync.Once` guard), so blocked emitters return. It also drains and drops the queue in a goroutine until `done`, so emitters in their fast path never block.

**Tests** in new `internal/eventstream/stream_test.go`, each a separate test function:
- emit returns immediately with room and a slow receiver;
- limit 1 with a slow receiver delivers all N events (compare the counts);
- a single stretch produces exactly one blocked event with a positive duration;
- the blocked event does not reduce delivered regular events;
- delivery order equals emit order for one emitter;
- the receiver is never entered concurrently (atomic in-flight counter);
- Drain returns only after done and an empty queue;
- Discard releases a blocked emitter;
- time and source defaults are stamped.
Run with `-race`.

**Complexity:** Medium · **Token estimate:** ~30k · **Agent strategy:** Single agent, sequential. Concurrency-sensitive, so keep it in one context.

### Phase 1.3: The parser emits events, reports every failure, and can be cancelled

**File changes**
- **`internal/parser/events.go`:**
  - Delete `ParserEvent`, `fireParserEvent` and `fireParseEvent` (`:6-55`).
  - Add `emit(options, e events.Event)`, which is nil-safe.
  - Add `lifecycleEvent(meta *types.Meta, operation, phase string, duration, err, data) events.Event`, which fills `ResourceType: resourceType(meta)`, `ResourceID`, `File: meta.File` and `Source: core`.
  - Add `parseEvent(resourceType, resourceID, file, err)`.
- **`internal/parser/parser.go`:**
  - `ParserOptions`: remove `Logger` (`:65-66`) and `OnParserEvent` (`:91-93`). Add `Emit events.Emit`.
  - `Parser.Validate` (`:371`), `Parser.Apply` (`:231`) and `Parser.Destroy` (`:322`) take a leading `ctx context.Context`, threaded into `p.walk` (`:1235`) and the destroyer (`destroy.go:43`), and captured by `walkCallback`/`destroyWalkCallback` and `resourceLifecycle`.
  - `DefaultOptions()` (`:111-127`) builds the registry with `registry.NewPluginRegistry(nil)` until 2.2.
  - `NewParser` removes the `StdOutLogger` default (`:166-169`).
  - Replace the `fireParseEvent` calls at `:511`, `:538`, `:562`, `:581`, `:595`, `:607`, `:881` and `:1031`.
  - Remove `log.SetOutput(io.Discard)` at `:1269` and the `log`/`io` imports if they become unused.
  - Validate (`:371` onward): for every problem collected into the returned `ConfigError` by `p.validate()`, emit `Operation: validate, Phase: error`, with the resource ID/type/file when the problem is tied to a resource (from the `ParserError`), otherwise file only.
- **`internal/parser/lifecycle.go`:**
  - Replace the `fireParserEvent` calls at `:76`, `:330`, `:336`, `:343` and `:348`.
  - At the missing-provider return (`:80-83`), emit an error event for the step being attempted.
  - Cancellation check: `lifecycle.run` checks `l.ctx.Err() != nil` before each provider step (create, read, changed, update, rebuild destroy) and, if cancelled, returns the unexported sentinel `errNotReached` without firing a start event. No public error is added: the value is never returned to callers, because a cancelled operation only happens on the receiver-panic path and that panic continues.
  - `callProvider` (`:321`) builds the provider call's context as `plugins.WithLogger(context.WithoutCancel(l.ctx), ...)` in 2.1; in this phase `context.WithoutCancel(l.ctx)` replaces `context.Background()` at `:121,174,200,215,246`.
  - `warnChangedConfiguredValues` (`:304-316`): build `logger.New(l.options.Emit, lifecycleEvent(meta, operation, "", ...))` with `Operation` set to the step (create/update/read, from the call sites at `:128`, `:195` and `:222`, which pass the operation through). Pass that logger in.
- **`internal/parser/configured_check.go:17-25`:** keep the signature, taking a `logger.Logger`. Remove the `"event", "configured_value_changed"` pair and keep `"field", path`. The resource is already bound.
- **`internal/parser/callbacks.go`:**
  - `walkCallback` (`:37-166`): at each error return (decode, context, disabled evaluation, and the `panic(err)`/"no body found" sites at `:57`/`:69` are left as they are), emit an error event carrying the wrapped error before returning the diagnostics.
  - `destroyWalkCallback` (`:188-287`):
    - replace `:214`, `:257`, `:263` and `:278`;
    - add error events for "no provider found" (`:225-238`) and marshal failure (`:241-254`);
    - add the `ctx.Err()` check before `:257`, and pass `context.WithoutCancel(ctx)` to `adapter.Destroy` at `:259`.
  - Resources skipped by cancellation in destroy return `nil` diagnostics and are left in state untouched: the resource is not reported destroyed, so the per-resource save keeps it.
  - In apply, a step returning `errNotReached` must not mark the resource failed. `walkCallback` treats `errors.Is(err, errNotReached)` as "not reached": it does not record progress and returns an empty diagnostic, so `progress.buildState` (`internal/parser/progress.go:44-76`) keeps the previous entry and new resources are left out.
- **`internal/parser/destroy.go:69`:** remove `log.SetOutput(io.Discard)` and the unused imports.
- **Root `config.go`:** pass `Emit: c.emitFor(...)`, a temporary adapter that calls `c.eventHandler` directly, until 1.4 replaces it. Delete `parserEventHandler` from `events.go`.

**Tests**
- Update `internal/parser/parse_events_test.go`, `lifecycle_test.go` (`eventCollector` `:186-205`, harness `:94-110`), `destroy_test.go:410,451`, `registered_types_test.go:96,243-260,353-383`, `parse_test.go:751,1039,1114,1159`, `plugins/example/apply_test.go:32-57` and `config_events_test.go` to record `events.Event` via `options.Emit`.
- Replace `recording_logger_test.go` with an event recorder.
- `configured_check_test.go:238,255` and `lifecycle_test.go:972-1008` assert warn log events with `Meta["field"]`, `ResourceID` and `Operation`.
- New tests:
  - a missing provider emits an error event;
  - a validate reference problem emits a validate error event;
  - an already-cancelled context starts no provider calls;
  - cancelling mid-walk leaves unreached new resources out of the partial state;
  - cancelling mid-walk does not cancel the context a running provider call received (the TestPlugin records `ctx.Err()` at the end of a slow create and it is nil);
  - `TestStandardLoggerOutputIsUntouchedAfterApply` (root): set `log.SetOutput(buf)`, apply, then `log.Print("x")` and assert `buf` contains it;
  - the same for validate and destroy, as separate tests.
- New `static_output_test.go` (root, `package xcl_test`): walk the module's non-test `.go` files excluding `internal/xcl/`, `internal/cty/`, `example/` and `logger/pretty_printer.go`, parse them with `go/parser`, and assert there is no import of stdlib `"log"` and no selector `log.SetOutput`. It is extended in 2.1.

**Complexity:** High · **Token estimate:** ~70k · **Agent strategy:** Parallel analysis, sequential integration. One agent converts emission sites plus tests in `internal/parser`, a second updates root and `plugins/example` tests after the first lands, and the orchestrator runs `go test -race ./...`.

### Phase 1.4: One operation runner with graceful receiver-panic shutdown

**File changes**
- **`options.go`:** add `WithEventBufferSize(size int)` and `const DefaultEventBufferSize = 1024` (documented). Update the `WithEventHandler` doc: delivery is one event at a time and in order, emission never waits for the receiver, and everything is delivered before the call returns.
- **`config.go`:**
  - Add the field `eventBufferSize int`.
  - `NewConfig` (`:104-120`): the default registry is `registry.NewPluginRegistry(nil)` until 2.2 changes the signature.
  - **New `run(operation string, work func(ctx context.Context, emit events.Emit) error) error`:**
    - If `c.eventHandler == nil`, return `work(context.Background(), nil)`.
    - Otherwise:
      - `stream := eventstream.New(size)` and `ctx, cancel := context.WithCancel(context.Background())`, with `defer cancel()`.
      - `done := make(chan struct{})`.
      - Emit the operation start event.
      - `go func(){ defer close(done); err = work(ctx, stream.Emit); emit operation success/error (Error: err, Duration) }()`.
      - Set `drained := false` and `defer func(){ if !drained { cancel(); stream.Discard(); <-done } }()`, with **no recover** and a comment explaining that the receiver's panic must continue untouched with its original stack.
      - Call `stream.Drain(c.eventHandler, done)`, set `drained = true` and return `err`.
  - A panic *inside the worker* (a provider panic) is not caught. That behaviour is the same as today and out of scope, and a comment says so.
  - `Validate` (`:199-218`), `Apply` (`:232-263`) and `Destroy` (`:277-310`) move their bodies into `work` closures that use `ParserOptions{Emit: emit}` and call `p.Validate(ctx, ...)`, `p.Apply(ctx, ...)` and `p.Destroy(ctx, ...)`.
  - Apply's state save stays inside the closure (`:256-260`), so a cancelled apply still saves before `done` closes.
  - Destroy's state `Load` (`:285`) moves inside the closure.
  - A state save failure emits `Operation: apply|destroy, Phase: error` with the save error. This is covered by the operation error event, because the joined error is returned.
  - Remove the temporary `emitFor` from 1.3.
- No new `errors` entry is needed for cancellation (the parser's `errNotReached` is unexported).

**Tests** (root, `package xcl`, in new `config_delivery_test.go` and `config_panic_test.go`). Each is its own function, using `parser.TestPlugin`, whose recorded calls come from `internal/parser/test_plugin.go`:
- `TestApplyDeliversEveryEventBeforeReturning`
- `TestApplyDeliversErrorEventBeforeReturningError`
- `TestValidateDeliversEveryEventBeforeReturning`
- `TestDestroyDeliversEveryEventBeforeReturning`
- `TestEveryEventTimeFallsWithinTheCall`
- `TestEveryCoreEventNamesCoreAsSource`
- `TestApplyDoesNotWaitForSlowReceiver`: a 1s receiver, a buffer larger than N, and a timestamp recorded by the provider at its last call (assert < N s), plus apply's return after all N are handled. It uses a TestPlugin provider hook or a timestamp on the create-success event's emit time.
- `TestApplyWithBufferOfOneDeliversEveryEvent`: compare with a fast receiver.
- `TestApplyWithBufferOfOneAnnouncesBlockingOnce`
- `TestReceiverIsNeverCalledConcurrently`
- `TestResourceEventsArriveInStartSuccessOrder`
- `TestFailedCreateStopsDependantsWithReceiver` and `...WithoutReceiver`
- `TestFailedCreateEmitsErrorEventWithReturnedError`
- `TestSlowReceiverDoesNotChangeApplyResult`
- `TestIgnoringReceiverDoesNotChangeApplyResult`
- `TestReceiverPanicIsRaisedFromApplyWithSameValue`: `defer recover()` in the test and compare the value.
- `TestReceiverPanicStackContainsReceiver`: `debug.Stack()` in the test's deferred recover contains the receiver function's name.
- `TestReceiverPanicRecordsOnlyCreatedResourcesInState`: several independent resources, a file state store, and a TestPlugin create that sleeps briefly so creates overlap. Assert state IDs ⊆ created IDs and ⊇ resources whose create returned.
- `TestReceiverPanicStartsNoProviderCallAfterPanic`: TestPlugin records call start times, and the receiver records the panic time.
- `TestWithEventBufferSizeSetsLimit`
- `TestDefaultEventBufferSizeIsUsedWhenUnset`
- `TestApplyWithoutReceiverWritesNothingToStdoutOrStderr`: redirect `os.Stdout`/`os.Stderr` to `os.Pipe` for the call.

All run under `-race`.

**Complexity:** High · **Token estimate:** ~60k · **Agent strategy:** Parallel analysis, sequential integration. A single implementing agent owns `config.go`, and a second writes the panic/delivery tests against the agreed runner contract.

### Phase 2.1: Plugin authors log from the call's context

**File changes**
- **`logger/`:**
  - Delete `stdout_logger.go`, `event.go`, `event_test.go`, `test_logger.go` and `test_logger_test.go`.
  - `tagged.go`: remove the text-formatting `taggedLogger`. `WithTag` only works on the event logger and returns `l` unchanged for other implementations.
  - Rewrite `tagged_test.go` to cover event-logger tags.
  - Leave `pretty_printer.go` untouched (out of scope).
- **`plugins/interfaces.go:6`:** keep `type Logger = logger.Logger`.
- **New `plugins/context.go`:** `Logger(ctx) Logger` returns the bound logger or `logger.Nop()`. `WithLogger(ctx, l)` uses an unexported key type.
- **`plugins/plugin.go`:**
  - `PluginEntityProvider` (`:44-53`) adds `ctx` to Validate, Create, Destroy, Update and Changed.
  - `Plugin` (`:74-79`) removes `SetLogger`. `PluginBase` removes `SetLogger` (`:88-99`) and the `logger` field (`:82`).
  - `PluginBase` methods (`:145-206`) pass `ctx` through to the adapters.
  - The `RegisterResourceProvider` doc says the `logger` is plugin-scoped, for messages outside a call.
- **`plugins/adapter.go`:** delete `debugCall` (`:101-112`) and its calls at `:141`, `:167`, `:190`, `:215` and `:245`. `Init` (`:86-95`) no longer tags `provider=` as text. It uses `logger.WithTag(log, "provider", a.name)`, which becomes a Meta detail.
- **`plugins/direct_plugin_host.go`:**
  - `NewDirectPluginHost(emit events.Emit, state, plugin)` (`:16`) builds the plugin-scoped logger with `logger.New(emit, events.Event{Source: pluginTypeName(plugin), Operation: events.OperationLoad})`.
  - Replace the `"plugin loaded"` debug (`:32`) with a `load` success event: `Source: core`, `Meta{"plugin": name, "block_types": ...}`.
  - Each method (`:60-91`) re-stamps the ctx logger's source with the plugin name before delegating. Add `logger.WithSource(l, name)`, which returns a copy with `base.Source` set.
- **`plugins/grpc_plugin_host.go`:**
  - `NewGRPCPluginHost(emit events.Emit, state)` (`:27`). For this phase, keep the plugin-scoped logger for the callback server and hclog (`:38-54`) built from `emit` with `Source: pluginBinaryName(path)`.
  - Replace `"plugin loaded"` (`:74`) with a load event.
  - Wrapper methods (`:117-225`) take `ctx`. The call-ID work comes in 2.3.
- **`plugins/grpc_resource_adapter.go`:** forward `ctx` in every method.
- **`plugins/grpc_server.go`:**
  - Replace the per-RPC `SetLogger` (`:43-168`) with `ctx = WithLogger(ctx, l)`, using the cached host logger until 2.3.
  - Call `s.plugin.X(ctx, ...)` for Validate, Destroy and Changed.
  - Guard `getLogger` (`:177-191`) with a `sync.Mutex`.
- **`plugins/hclog_adapter.go`:** keep the adapter, now over the event logger. Keep the level mapping, drop the text `"event","go-plugin"` pair, and put `Meta{"component": "go-plugin"}`.
- **`internal/parser/lifecycle.go`** (`callProvider` at `:321`) and **`callbacks.go`** (destroy at `:257-259`): build `ctx := plugins.WithLogger(context.Background(), logger.New(options.Emit, lifecycleEvent(meta, operation, events.PhaseLog, ...)))` and pass it to the adapter. The call closures take `ctx` in place of `context.Background()` at `lifecycle.go:121,174,200,215,246` and `callbacks.go:259`.
- **`internal/parser/test_plugin.go:245-343`:** `Init` no longer resets calls on re-init (it is only called once now). Providers log via `plugins.Logger(ctx)` and gain an optional hook, `LogOnCreate`/`LogOnRead` fields, used by tests to log a message with details.
- **`plugins/registry/plugin_registry.go:310,330`:** call the hosts with `nil` emit for now, because the registry keeps its logger param until 2.2. Discovery still uses its `logger.Logger` param, and callers pass `logger.Nop()`.
- **`plugins/example/`** (person plugin: `main.go:21`, `pkg/person/provider.go:20-108`): log through `plugins.Logger(ctx)` without `"resource"`/`"event"` args.
- **`plugins/testing/helpers.go:29-73`:** replace the `WithLogger` variants with `WithEmit(t, ..., emit events.Emit)`. The default is a nil emit.
- Replace every `logger.NewTestLogger(t)` argument with `nil`/`logger.Nop()` at the call sites listed in research (root tests, `internal/parser` tests, `plugins` tests, `state` tests, examples). `NewPluginRegistry(...)` keeps a `logger.Logger` param until 2.2, so pass `logger.Nop()`.
- **Examples** (`example/*/main.go`, `example/plugin/internal/plugin.go`, `example/plugin/external/main.go`): minimal compile fixes only. `main` passes `logger.Nop()` where a logger is still required. Full rework is in 3.1.
- Run `make mocks` to regenerate `plugins/mocks/mock_provider_adapter.go`, and any mock whose interface changed.

**Tests**
- New `plugins/context_test.go`: `Logger` without a bound logger is Nop, and `Logger` returns the bound logger.
- `plugins/adapter_test.go`: the adapter emits no log events of its own.
- New root `config_plugin_logging_test.go`, with the in-process TestPlugin and separate tests:
  - `TestPluginLogBecomesEventWithSeverityMessageAndDetail` (`remote_id=213`)
  - `TestPluginLogCarriesResourceTypeFileAndCreateStep`
  - `TestPluginLogCarriesReadStep`
  - `TestPluginLogNamesPluginAsSource`
  - `TestPluginLogSitsBetweenCreateStartAndSuccess`
  - `TestApplyingOneResourceGivesOneCreateStartAndNoRestatingLog`
  - `TestPluginLogsAtEverySeverityReachReceiver`
- Rewrite `plugins/example/e2e_test.go:513-660` to assert on events: in-process source `PersonPlugin`, the resource bound, no "calling provider", and a load event.
- `plugins/hclog_adapter_test.go`: assert emitted events.
- Extend `static_output_test.go`:
  - no `fmt.Print*`, `os.Stdout` or `os.Stderr` in library packages;
  - no type outside `logger/` implements `logger.Logger` (checked by scanning for method sets `Debug`, `Info`, `Warn` and `Error` on non-test types outside `logger/` and `plugins/grpc_clients.go`, where the gRPC logger is allowed because it forwards to the stream). The allowed list is explicit in the test.

**Complexity:** High · **Token estimate:** ~90k · **Agent strategy:** Parallel analysis, sequential integration.
1. Agent A handles `logger/` + `plugins/` interfaces/hosts/adapter + mocks.
2. After A compiles, agent B handles `internal/parser` binding + TestPlugin + parser tests, and agent C handles the `plugins/example` + `plugins/testing` + root/state/example compile fixes, in parallel.
3. The orchestrator runs `go build ./... && go test -race ./...`.

### Phase 2.2: The plugin registry records plugins and loads them on first use

**File changes**
- **`plugins/registry/plugin_registry.go`:**
  - The struct (`:18-26`) gains:
    - `mu sync.RWMutex`
    - `pending []plugins.Plugin`
    - `pendingPaths []string`
    - `discoveryDirs []string; discoveryPattern string`
    - `loadOnce sync.Mutex; loaded bool; loadErr error`
    - `active atomic.Pointer[events.Emit]`
    It drops `logger`.
  - `NewPluginRegistry()` (`:29`).
  - `RegisterPlugin` (`:308-325`) and `RegisterPluginWithPath` (`:328-349`) append to pending under `mu`, with no validation of the path.
  - Replace `DiscoverAndLoadPlugins` (`:352-404`) with `DiscoverPlugins(dirs, pattern)`, which records them.
  - **New `Load(emit events.Emit) error`:** holds `loadOnce`. If `loaded` is set, it returns `loadErr`. Otherwise it:
    1. emits `discover` start;
    2. scans the dirs with `PluginDiscovery` (now emit-based);
    3. emits `discover` success with `Meta{"count": n, "dirs": ...}`;
    4. for each in-process plugin, runs `load` start → `NewDirectPluginHost(r.pluginEmit, nil, p)` → `checkHostTypes` → `load` success/error, where the error is a `*xclerrors.PluginLoadError{Plugin: name, Err}` and is fatal;
    5. for each explicit path, does the same with `NewGRPCPluginHost(...).Start(path)`, which is fatal on failure;
    6. for each discovered path, a failure or clash gives a `load` error event with `Meta{"rejected": true}` and the path is skipped. The run fails only if every discovered plugin failed and none loaded, as today at `:399-401`. Clashes are always fatal, as today at `:393-396`.
    7. It appends hosts under `mu` (write lock) and caches `loaded = true` and `loadErr`.
  - `r.pluginEmit` is a stable `events.Emit` closure that forwards to `*r.active.Load()` if set, otherwise drops.
  - **New `Activate(emit) func()`:** stores the emit and returns a function that clears it only if it is still the current one (compare and swap).
  - `Type`, `Types`, `CreateResource`, `GetProviderForResource`, `IsRegisteredType`, `TypePath` and `GetPluginHosts` take `mu.RLock`. `RegisterType`/`RegisterBareType` take `mu.Lock`, and `checkTypeName` (`:185`) checks only `typeInfo` plus already-loaded hosts.
- **`plugins/registry/plugin_discovery.go:13-166`:** `NewPluginDiscovery(dirs, pattern, emit events.Emit)`. Each `logger.X` call (`:44`, `:57`, `:68`, `:80`, `:96`, `:106`) becomes a `logger.New(emit, events.Event{Operation: discover})` log call with the same message and details, minus `"event"`.
- **`errors/`:** new `plugin_load_error.go` with `ErrPluginLoad` plus `PluginLoadError{Plugin string; Err error}` (pointer receiver, `Unwrap`, `Is`), following `plugins/errors.go`. Re-export them from `config.go` next to the query errors (`:30-73`).
- **`config.go`:** `run()` calls `c.pluginRegistry.Load(emit)` first (inside the work closure, so load events flow), then `defer c.pluginRegistry.Activate(emit)()`. With no receiver, `emit` is nil, and Load still runs.
  - `addressParser()` (`:88-99`) does not cache while the registry has not loaded. Add a `Loaded() bool` accessor.
  - `NewConfig` defaults to `registry.NewPluginRegistry()`.
- **`state/file_state_store.go`:** no change. It is covered by the runner loading before `Load`.
- Update every `NewPluginRegistry(x)` call site to `NewPluginRegistry()`:
  - root tests: `config_destroy_test.go:48,269,446`, `config_events_test.go:57,137`, `config_validate_test.go:38`, `config_options_test.go:20,36,50`, `query_setup_test.go:33`, `query_test.go:34`, `query_by_type_test.go:31`;
  - `internal/parser/parser.go:119`, and the parser tests listed in research;
  - `state/*_test.go`;
  - `plugins/example/apply_test.go:114`;
  - `example/*/main.go` and `main_test.go`.
- Tests and helpers that relied on eager registration errors or `GetPluginHosts()` right after `RegisterPlugin` now call `r.Load(nil)` first.

**Tests**
- Rework `plugins/registry/plugin_registry_test.go` (`:103-494`) and `plugin_discovery_test.go` (`:15-602`) for the record → Load flow, one behaviour per test.
- New:
  - `TestNewPluginRegistryNeedsNoLogger`
  - `TestRegisterPluginWithMissingPathReturnsNoError`
  - `TestFirstValidateFailsNamingMissingPlugin` (root; `errors.Is(err, xcl.ErrPluginLoad)` and the name is in the message)
  - `TestNoPluginProcessRunsBeforeFirstOperation`: check `GetPluginHosts()` is empty. Also check the process list: the test builds the example plugin and asserts no child process with that binary path exists, using `/proc` scan on Linux or `pgrep` skipped elsewhere.
  - `TestSharedRegistryStartsExternalPluginOnce`: two Configs; assert one host, and one `load` success event across both receivers.
  - `TestRegisterTypeClashingWithBuiltinFailsImmediately`
  - `TestRegisterTypeClashingWithRegisteredTypeFailsImmediately`
  - `TestRegisterTypeMatchingPluginTypeSucceedsAtRegistration`
  - `TestFirstValidateFailsWithClashForPluginType`
  - `TestDiscoveryReportsDiscoverLoadAndRejectEvents` (the valid plugin via `buildExamplePlugin`, the invalid via `createNonPlugin`)
  - `TestConcurrentLoadIsSafe` (`-race`)

**Complexity:** High · **Token estimate:** ~70k · **Agent strategy:** Parallel analysis, sequential integration. Agent A handles the registry, discovery and errors plus registry tests. Agent B then handles root runner integration and the call-site sweep.

### Phase 2.3: The same logging across the plugin process boundary

**File changes**
- **`plugins/plugin.proto`** (`LogRequest` at `:111-117`): add `string call_id = 3;`. Regenerate `plugins/proto/plugin.pb.go` and `plugin_grpc.pb.go` with `make protos`. Delete the stale invalid `plugins/protos/plugin.proto`.
- **`plugins/grpc_plugin_host.go`:**
  - `GRPCPluginHost` gains `calls sync.Map // string → logger.Logger`.
  - Each wrapper method (`:117-225`):
    - takes the logger from `ctx` (`plugins.Logger(ctx)`), re-sourced with the plugin name;
    - generates an ID (`strconv.FormatUint(atomic counter)` plus a host nonce), then `calls.Store(id, l)` and `defer calls.Delete(id)`;
    - sets `ctx = metadata.AppendToOutgoingContext(ctx, "xcl-call-id", id)`;
    - calls the gRPC client with that `ctx` instead of `context.Background()`.
  - `Start` (`:37-78`) builds the plugin-scoped logger from the registry's forwarding emit.
- **`plugins/grpc_plugin.go:37-62`:** `SetupHostCallbackService` passes the host's `calls` map and the plugin-scoped logger to `NewGRPCHostCallbackServer`.
- **`plugins/grpc_host_callback.go:28-54`:** each handler resolves `req.CallId`. If it is found in `calls`, it uses that logger. Otherwise it uses the plugin-scoped logger. It calls the level method with `req.Message` and `stringArgsToInterfaces(req.Args)`. Keep the existing re-typing at `:100-115` (values-as-text is a non-goal). Update the doc to say values may arrive as text.
- **`plugins/grpc_server.go:42-191`:**
  - Each RPC reads `metadata.FromIncomingContext(ctx)["xcl-call-id"]` and builds `&GRPCLogger{client, callID}`, where the client is the cached dial under the mutex from 2.1.
  - It sets `ctx = WithLogger(ctx, l)` and passes `ctx` to the plugin.
  - The plugin-scoped logger used for `Init` providers is a `GRPCLogger` with an empty call ID that dials lazily. `grpc_plugin.go:15-30` passes it to `Impl.Init`, replacing `Init(nil, nil)`.
- **`plugins/grpc_clients.go:14-47`:** `GRPCLogger` gains a `callID` and sets `LogRequest.CallId`. It keeps `interfaceArgsToStrings` (`:96`). Errors stay ignored, and a comment says why: a failed callback must not affect processing.
- **`plugins/hclog_adapter.go`:** built over the plugin-scoped logger, whose emit is the registry forwarder, so go-plugin/stderr lines go to the active operation.
- **`example/plugin/external/main.go:27-34`:** update the comment, because `Init` now gets a working plugin-scoped logger.

**Tests**
- New root `config_plugin_boundary_test.go`, which builds the `plugins/example` person plugin with `plugintesting.BuildPlugin` and runs it both in-process and external. Separate tests:
  - `TestExternalPluginLogCarriesResourceAndStep`
  - `TestInProcessAndExternalPluginLogsMatchExceptSource`: compare level, message, detail names, `fmt.Sprint` of the values, ResourceID/Type/File and Operation.
  - `TestExternalPluginLogNamesPluginAsSource`
  - `TestEachConfigReceivesOnlyItsOwnPluginLogs`: two Configs, one shared registry, separate receivers, and resources with distinct IDs.
  - `TestNoReceiverWritesNothingWithBothPluginKinds`: validate, apply and destroy with stdout/stderr pipes, and assert no new files in the working dir or `$HOME` (use `t.Setenv("HOME", t.TempDir())`).
  - `TestEveryExternalPluginLogReachesReceiver`: the plugin counts its log calls and the receiver's count matches.
- `plugins/example/e2e_test.go`: the external variants assert events with source `example`.
- `plugins/grpc_plugin_host_test.go`: the wrapper sets the call-ID metadata (fake client captures the ctx), and the callback resolves a known call ID to the bound logger. An unknown ID falls back to the plugin logger.

**Complexity:** High · **Token estimate:** ~60k · **Agent strategy:** Parallel analysis, sequential integration. A single agent does proto + host + server, then a second writes the boundary tests.

### Phase 3.1: A styled example receiver used by every example

**File changes**
- Delete `example/eventlog/eventlog.go`.
- **New `example/prettylog/prettylog.go`:**
  - The package doc explains that this is an example, not library API, and why charm is used.
  - `Handler(w io.Writer, level slog.Level) xcl.EventHandler` returns `events.SlogHandler(slog.New(charmlog.NewWithOptions(w, charmlog.Options{Level: toCharm(level), ReportTimestamp: true, TimeFormat: time.Kitchen})))`.
  - It sets charm styles so the `source`, `resource` and `operation` keys are coloured.
  - Level comes from the `XCL_LOG_LEVEL` env var, default `info`, via the helper `LevelFromEnv()`.
- **`example/plugin/main.go`, `example/configonly/main.go`, `example/appconfig/main.go`:**
  - `run(out io.Writer, handler xcl.EventHandler, dir, ..., statePath)`, replacing the `log logger.Logger` parameter.
  - `main` calls `run(os.Stdout, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv()), ...)`, the single line.
  - `registry.NewPluginRegistry()`, and the `logger` import is removed.
  - Plugin example: `r.RegisterPluginWithPath(externalPlugin)` no longer errors for a missing binary. Keep the "build it with make build" hint by wrapping the `ErrPluginLoad` from `Apply` in `main`.
  - Keep the deferred host `Stop()`.
- **`example/plugin/internal/plugin.go:29-165`:** the `Init` log is kept using the Init logger (plugin-scoped). Providers drop the stored logger and use `plugins.Logger(ctx).Info("created", "connection_string", ...)` etc., at info so the default example level shows them, without `"resource"`/`"event"` args.
- **`example/plugin/external/main.go:66-168`:** the same change.
- **`example/*/Makefile`:** no change beyond removing `BUILD_DIR` in configonly (unused). Optional, and done because the file is touched anyway.
- **`example/*/main_test.go`:**
  - Replace the `recordingLogger` copies (`plugin/main_test.go:71-123`, `configonly/main_test.go:299-338`) with an event recorder.
  - Rewrite the log tests (plugin `:272-512`, configonly `:342-481`) as event assertions:
    - provider log events carry the resource ID/type/file/step;
    - load events for `ExamplePlugin` and `external`;
    - parse events with the file;
    - no error events on the happy path;
    - destroy phases;
    - per-severity presence.
  - New per-example test: `TestRunWithoutReceiverWritesNothingToStdoutOrStderr`, which uses fd pipes and passes `nil` as the handler.
- **New root `static_examples_test.go`:** for each `example/*/main.go`, assert:
  - exactly one `WithEventHandler(` whose argument is the `handler` param;
  - `main` calls `prettylog.Handler(` exactly once;
  - no import of `github.com/jumppad-labs/xcl/logger`;
  - `NewPluginRegistry()` is called with zero args.
  For the example plugin sources, assert that no call to `.Debug/.Info/.Warn/.Error` passes a `"resource"` string literal key.
- **New root `static_dependencies_test.go`:** run `go list -deps` for all packages excluding `./example/...` (via `exec.Command("go", "list", "-deps", ...)`, skipped if `go` is not on PATH) and assert that no `github.com/charmbracelet/` path appears.
- **`go.mod`:** charm stays a direct requirement (used by `example/prettylog`). Run `go mod tidy`, which may move `fatih/color`/`go-wordwrap` if unaffected. No version changes.

**Complexity:** Medium · **Token estimate:** ~50k · **Agent strategy:** Medium: 2–3 parallel agents. One does `prettylog` plus the static tests, one the plugin example (main, plugins, tests), and one configonly + appconfig. Integrate and run `go test ./example/... ./...`.

### Phase 3.2: Library documentation describes the event stream

**File changes**
- **`README.md`:**
  - `:138-144` and `:206-210`: rewrite the example logging descriptions for the pretty receiver and events.
  - `:248`: use `registry.NewPluginRegistry()`.
  - `:410-431`: replace "Lifecycle events" with an "Events and logging" section covering the Event fields (Time, Source, Operation, Phase, resource fields, Duration, Error, Data, Meta), the reserved `level`/`message` keys and constants, one-at-a-time ordered delivery, `WithEventBufferSize` and the default, blocking and the `blocked` event, delivery before return, receiver panic behaviour, and `events.SlogHandler` in one line.
- **`docs/README.md:38-46`:** the package list gains `events` and `internal/eventstream`. `logger/` is redescribed as "log calls become events".
- **`docs/parser-lifecycle.md:208-290`:** replace the "Instrumentation: ParserEvent" section with `events.Event` via `Emit`. Add the operation context (cancellation check before each provider call, non-cancelling provider context) and the operation-level events.
- **`docs/plugins.md`:**
  - `:32`, `:68` and `:143`: the Init logger is plugin-scoped.
  - `:160-161`: providers are tagged via the `provider` detail.
  - `:179-180` and `:270-283`: lazy registration, `DiscoverPlugins`, `Load` timing, and clash timing.
  - Replace "Plugin logging" (`:204-260`) with logging via `plugins.Logger(ctx)`, sources, resource/step binding, the external boundary (values arrive as text), and go-plugin messages.
- **`docs/plugin-developer-guide.md`:**
  - `:21` and `:32`: the Init logger.
  - `:308-316`: the configured-value warning is now a warn log event.
  - `:392-397`: Events.
  - Add a "Logging from a provider" snippet.
- **`plugins/README_test_helpers.md`:** the `WithEmit` helpers replace the `WithLogger` ones.
- **`CHANGELOG.md`:** a new top entry `20260922061954-event-based-logging` listing the breaking changes:
  - `NewPluginRegistry()`, `DiscoverPlugins`, lazy loading, `ErrPluginLoad`;
  - `Plugin.SetLogger` removed, ctx added to `PluginEntityProvider`, `plugins.Logger(ctx)`;
  - logger implementations removed;
  - `Event` fields added, `WithEventBufferSize`, `events.SlogHandler`;
  - xcl no longer writes output or touches `log.SetOutput`.

**Tests:** none automated. A text search check is done at phase end: `grep -rn "NewPluginRegistry(log\|NewStdOutLogger\|calling provider\|DiscoverAndLoadPlugins" --include=*.md .` returns nothing.

**Complexity:** Low · **Token estimate:** ~30k · **Agent strategy:** Single agent, sequential.

### Phase 3.3: The documentation site covers events, logging and plugin loading

**File changes** (all in the `xcl-website` repo)
- **New `xcl-website:src/pages/events.mdx`:**
  - Frontmatter follows the pattern in `src/pages/examples/plugins.mdx` (`layout: ../layouts/Shell.astro`, `title`, `description`), with `Hero` + `Prose` + `CtaBanner`.
  - Contents:
    - the event stream and each Event field;
    - operations and phases;
    - sources;
    - delivery guarantees (ordered, one at a time, bounded buffer, blocked event, delivered before return);
    - errors and receiver panics;
    - connecting to slog in one line;
    - the pretty receiver example.
  - Code blocks are ```go title="..."``` and output is ```text```.
- **New `xcl-website:src/pages/plugin-logging.mdx`:**
  - logging from a provider with `plugins.Logger(ctx)`;
  - automatic resource/step context;
  - in-process vs external, with values as text;
  - the plugin-scoped Init logger;
  - registration vs lazy loading, `DiscoverPlugins`, load-once per registry, and clash timing.
- **`xcl-website:src/components/Nav.astro:8-18`:** add a "Guides" dropdown (or items) linking `/events/` and `/plugin-logging/`.
- **`xcl-website:README.md`:** add rows to the Pages table.
- **`xcl-website:src/pages/index.mdx`:**
  - `:90` becomes `registry.NewPluginRegistry()`;
  - `:173-177` becomes a feature card "Events and logging" linking `/events/`.
- **`xcl-website:src/pages/examples/plugins.mdx`:**
  - `:137-223`: provider snippets use `plugins.Logger(ctx)`;
  - `:245-287`: the `run` body copy is synced to the new `example/plugin/main.go` (`NewPluginRegistry()`, `WithEventHandler(handler)`, the one-line `prettylog` in `main`);
  - `:245-247`: the registration prose says lazy loading;
  - `:310-339`: the output blocks are replaced with pretty-receiver sample output captured from `make run`;
  - `:366-367`: rewritten for source/resource/step.
- **`xcl-website:src/pages/examples/configuration-only.mdx`:** `:222`, `:252` and `:292` are synced with `example/configonly/main.go`.
- **`xcl-website:src/pages/examples/application-config.mdx`:** `:278` and `:294` are synced with `example/appconfig/main.go`.

**Tests:** `make check` (`npx astro check`) and `npm run build` pass, and `grep -rn "NewPluginRegistry(log\|NewStdOutLogger\|eventlog" src/` returns nothing. The rendered pages are reviewed by hand (manual test plan).

**Complexity:** Medium · **Token estimate:** ~35k · **Agent strategy:** Single agent, sequential. It needs the finished example code from 3.1 to quote.

## Testing Strategy

The plan-level strategy is in plan.md § Testing Approach. Per phase:

- **1.1:** unit tests for the slog adapter's level mapping and filtering, and for the event logger (reserved keys win, details keep their names, a resource tag fills the ID).
- **1.2:** `internal/eventstream` unit tests under `-race`: no wait with room, no drop at limit 1, one blocked event per stretch, order, no concurrent entry, drain completeness, discard release.
- **1.3:** existing parser event tests move to recording `events.Event`. New tests cover error events at silent sites, cancellation behaviour, and the global-logger regression. The static scan starts with a check for stdlib `log`.
- **1.4:** root delivery and panic tests. These cover delivered before return (success and error), timestamps inside the call, fire-and-forget timing, limit-1 completeness, blocked-once, the concurrency guard, per-resource order, errors stopping dependants with and without a receiver, a slow or ignoring receiver not changing the outcome, panic value and stack, state after a panic, no provider call after a panic, buffer option and default, and silence with no receiver.
- **2.1:** root plugin-logging tests with the in-process TestPlugin cover severity, message, detail, resource, file, step, source, ordering within a step, no duplicate, and all four severities. The e2e and hclog tests are rewritten against events. The static scan is extended to stdout, stderr and fmt.Print, and to logger implementations.
- **2.2:** registry and discovery tests reworked for record → Load, one behaviour per test. Root tests cover lazy failure naming the plugin, no process before the first call, load once across Configs, clash timing, discovery/load/reject events, and concurrent Load safety.
- **2.3:** boundary tests with the built person plugin. They cover external resource and step context, in-process and external parity, source naming, per-Config routing, no output with both plugin kinds and no receiver, and every external log reaching the receiver. Wrapper and callback unit tests cover the call-ID metadata and routing.
- **3.1:** example tests rewritten to use event recorders, plus a no-output-without-receiver test per example. Static tests cover the example sources (one receiver, no logger, registry built with no arguments, no `resource` detail in provider logs) and the `go list -deps` check that library packages never import charmbracelet.
- **3.2 and 3.3:** a text search for removed APIs, `astro check` and `astro build`, and a manual review of the rendered pages and of styled example output. These are captured in the implementation test plan.

**Success metric → test mapping.**
- 0 direct log calls: static scan (1.3, extended in 2.1).
- 0 bytes with no receiver: pipe tests (1.4, 2.3, 3.1).
- One-line receiver and no registry logger in the examples: static examples test (3.1).
- 0 lost events: the limit-1 test (1.2, 1.4).
- Example plugins log context without passing it: example tests plus the static check (3.1).

## Project References

- Spec: `20260922061954-event-based-logging` (read via `spektacular spec file read 20260922061954-event-based-logging.md`).
- Repos: `xclconfig` at `/home/nicj/code/github.com/jumppad-labs/xcl` (library, examples, library docs); `xcl-website` at `/home/nicj/code/github.com/jumppad-labs/xcl-website` (xcl.dev docs site, Phase 3.3 only).
- Knowledge: conventions in store `xclconfig` (code style, dependencies, shared errors package, testing & mocking, test state from real apply, never modify dependencies, patterns & architecture).
- Design documents: none referenced by the spec.
- Research log: `research.md` in this plan.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Phases 1.3, 1.4, 2.1, 2.2 and 2.3 are High. Run each in a fresh implementation context, and commit after each phase, so a later phase can rehydrate from git and this document. Every sub-agent prompt must include the repo root `/home/nicj/code/github.com/jumppad-labs/xcl` (and `/home/nicj/code/github.com/jumppad-labs/xcl-website` for 3.3), and must say that store files under `.spektacular/` are reached only through the `spektacular` CLI.

## Migration Notes

Breaking API changes. xcl is unreleased, so no deprecation shims are added. All in-repo callers are updated, and CHANGELOG.md records every change:
- `registry.NewPluginRegistry(logger)` becomes `NewPluginRegistry()`. `DiscoverAndLoadPlugins(logger, dirs, pattern)` becomes `DiscoverPlugins(dirs, pattern)`. Plugins load on the first `Validate`/`Apply`/`Destroy`, and load errors surface there as `ErrPluginLoad`.
- `plugins.Plugin.SetLogger` and `PluginBase.SetLogger` are removed. `PluginEntityProvider` methods take `ctx`. Providers log through `plugins.Logger(ctx)`. Host constructors take an `events.Emit` instead of a logger.
- `logger.NewStdOutLogger`, `NewStdOutLoggerWithOptions` and `NewTestLogger` are removed, along with the text `event=` line format.
- `parser.ParserOptions.Logger` and `OnParserEvent` are replaced by `Emit` (internal), and the parser's `Validate`/`Apply`/`Destroy` take a `ctx` (internal).
- The `xcl.Event` fields are extended, and existing fields keep their names. `EventHandler` is now called one event at a time.
- `plugins/plugin.proto` `LogRequest` gains `call_id`. Plugin binaries must be rebuilt against the new `plugins` package. The handshake version is unchanged, because older binaries would only lose context, not break. A plugin built before this change still sends its logs, but they route as out-of-call logs.
No state file format changes.

## Performance Considerations

- **No receiver:** the work runs inline with a nil emit. There is no goroutine and no queue, and emit sites cost a nil check plus building an event struct. Emit helpers should check for a nil emit before building `Meta` maps.
- **With a receiver:** there is one extra goroutine per operation, and each event costs one channel send. Emitters only wait once the buffer (default 1024) is full, and a slow receiver can then slow provider calls by design, because the spec requires blocking over dropping.
- **gRPC:** every provider call adds one `sync.Map` store and delete plus one metadata entry. Log callbacks are unary RPCs, as they are today.
- **Registry:** read locks are taken on type lookups during the walk. Contention is negligible next to provider calls.
