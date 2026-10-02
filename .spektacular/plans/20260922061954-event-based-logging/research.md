---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Research: 20260922061954-event-based-logging

## Alternatives considered and rejected

### Option A: Deliver on a dedicated background goroutine, recover the receiver's panic there, re-panic on the caller (the spec's Technical Approach wording)

**Rejected**: a value recovered on one goroutine and re-raised on another loses the receiver's stack, so the acceptance criterion "a stack trace containing the receiver's code" can only be met by wrapping the value, which breaks "the same panic value". Chosen instead: the *caller's* goroutine is the single delivery goroutine and the work (parse/validate/walk/load) runs on a worker goroutine. A receiver panic then happens on the caller's goroutine; a deferred shutdown runs *without* `recover()`, so the panic continues with its original value and stack. Evidence: the caller already blocks in `w.Wait()` (`internal/dag/walk.go:112-131`) and does nothing else during a walk; Go runs deferred calls on top of the panicking stack.

### Option B: Give the provider a per-call logger by re-running `Init`/`SetLogger` before each call (today's external-plugin mechanism, `plugins/grpc_server.go:43-168` → `PluginBase.SetLogger` `plugins/plugin.go:88-99`)

**Rejected**: independent resources are walked concurrently (`internal/dag/walk.go:293-300`), so the same provider instance receives concurrent calls; a logger stored on the provider races and would attach the wrong resource. Also `internal/parser/test_plugin.go:245-326` resets recorded calls on every `Init`.

### Option C: Carry resource/step context in every `PluginService` request message and echo it in `LogRequest`

**Rejected**: Rejected as the routing mechanism: the host still has to find *which Config's* receiver a callback log belongs to (a registry is shared across Configs, `options.go:13`; one host-side callback server per plugin, `plugins/grpc_plugin.go:37-62`). A call ID that the host maps to a bound logger solves context and routing together.

### Option D: Synchronous handler serialised behind a mutex

**Rejected**: Delivers in order without a queue, but every emitter waits for the receiver, which violates "fire and forget". A slow receiver would slow every provider call, and a receiver panic would happen on a walker goroutine (`internal/dag/walk.go:377`), where it cannot be raised from the calling method.

### Option E: Level and message as dedicated `Event` fields / typed event interface

**Rejected**: Rejected by the spec (Constraints: one flat shape, kind-specific data in `Meta`).

### Option F: Registry-owned event handler (`registry.WithEventHandler`)

**Rejected**: Rejected by the spec (registry must not hold or accept a logger; lazy loading with per-call receiver instead).

### Option G: Keep `log.SetOutput(io.Discard)` to silence the dag walker

**Rejected**: `internal/dag` never uses the stdlib `log` package (`internal/dag/walk.go` imports only `errors`, `sync`; `internal/dag/UPSTREAM.md`), so the calls at `internal/parser/parser.go:1269` and `internal/parser/destroy.go:69` only mutate the host application's global logger.

### Option H: Moving examples to their own Go module so charmbracelet stays out of `go.mod`

**Rejected**: examples are packages of the root module (no nested `go.mod`), and module-graph pruning means a consumer that imports only library packages never compiles charm. "Out of the library's dependencies" is met by no library package importing it, verified with `go list -deps`.

### Option I: Deleting `logger/pretty_printer.go`

**Rejected**: Out of scope: it is a resource printer, not logging, has no callers, and writes only when explicitly called.

## Chosen approach — evidence

- Existing subscription to extend: `events.go:11-47` (`Event`, `EventHandler`), `options.go:32-36` (`WithEventHandler`), conversion shim `events.go:48-67`, duplicate `internal/parser/events.go:6-55` (`ParserEvent`, `fireParserEvent`, `fireParseEvent`), callback `ParserOptions.OnParserEvent` `internal/parser/parser.go:91-93`.
- All provider-call events go through one wrapper, `callProvider` (`internal/parser/lifecycle.go:321-350`), plus destroy walk emission `internal/parser/callbacks.go:214,257,263,278`: the natural place to bind a per-call logger into ctx and to check the operation context before starting a call. Every call currently passes `context.Background()` (`lifecycle.go:121,174,200,215,246`; `callbacks.go:259`).
- `types.Meta` carries ID, Type, Subtype, File, Line (`types/resource.go:5-62`); `resourceType(meta)` at `lifecycle.go:372-375` yields "type.name".
- Provider interface already takes `ctx` on every lifecycle method (`plugins/provider.go:16-103`), so `plugins.Logger(ctx)`-style retrieval needs no provider signature change; `PluginEntityProvider` (`plugins/plugin.go:44-53`), `PluginBase` methods (`plugin.go:145-206`), `GRPCResourceProviderAdapter` (`plugins/grpc_resource_adapter.go`) and `grpcPluginWrapper` (`plugins/grpc_plugin_host.go:98-225`) drop ctx except for Read and must thread it.
- gRPC protocol: `plugins/plugin.proto` (generated via `Makefile` `protos` target into `plugins/proto/`); `HostCallbackService` has `Info/Debug/Warn/Error(LogRequest)`, `LogRequest{message=1, args=2}`. gRPC metadata on the outgoing ctx can carry a call ID without changing request messages; `LogRequest` gains a `call_id` field.
- No cancellation exists today: failures only skip dependants via `waitDeps` (`internal/dag/walk.go:402-440`); `CancelCh` (`walk.go:80-84`) only used by `Update`. An operation `context.Context`, cancelled on receiver panic and checked in `callProvider` and the destroy callback before each provider call, is needed; providers get `context.WithoutCancel` so in-flight calls finish for "starts no new provider calls".
- State after failure: Apply builds partial state with `applyProgress.buildState` (`internal/parser/progress.go:44-76`) and `Config.Apply` saves once (`config.go:256-260`); Destroy saves after each resource under `destroyer.mu` (`internal/parser/destroy.go:86-131`). A cancelled walk that returns normally reuses these paths unchanged.
- Registry eagerly loads today: `RegisterPlugin` → `NewDirectPluginHost` → `plugin.Init` (`plugins/registry/plugin_registry.go:308-325`, `plugins/direct_plugin_host.go:16-32`); `RegisterPluginWithPath` spawns the process (`plugin_registry.go:328-349`, `plugins/grpc_plugin_host.go:37-78`). Clash checks `checkTypeName` (`:185`) and `checkHostTypes` (`:209`) exist to reuse at load time. No mutex anywhere in the registry.
- Types needed before parse: `Config.Destroy` → `stateStore.Load()` (`config.go:285`) → `FileStateStore.Load` → `registry.CreateResource` (`state/file_state_store.go:111`); `Parser.parseAndValidate` loads state first. So loading must happen at the very start of each operation.
- `Config.addressParser()` caches from `pluginRegistry.Types()` forever (`config.go:88-99`) — must not be cached before plugins load.
- slog: `go.mod` is `go 1.25.0`; `log/slog` available and unused today. `charmbracelet/log v0.4.2` implements `slog.Handler` (`$GOMODCACHE/github.com/charmbracelet/log@v0.4.2/logger_121.go:27-66`).
- go-plugin logging paths: host `ClientConfig.Logger: newHCLogAdapter(pluginLogger)` (`plugins/grpc_plugin_host.go:54`) receives go-plugin's own client logs and the plugin process's stderr (go-plugin `client.go logStderr`); plugin stdout/stderr after `Serve` go to `SyncStdout/SyncStderr`, unset → discarded. So silence holds as long as the hclog adapter emits events instead of writing.
- Latent bug removed by this work: `NewParser` defaults `Logger` to a zero `&logger.StdOutLogger{}` (`internal/parser/parser.go:166-169`) whose inner charm logger is nil; `warnChangedConfiguredValues` (`configured_check.go:17-25`, reached from `lifecycle.go:315`) would panic through `Config.Apply`.

## Files examined

- `xclconfig:events.go:11-67` — Event struct (8 fields), EventHandler doc says concurrent calls; parserEventHandler converter.
- `xclconfig:options.go:9-39` — ConfigOption, WithPluginRegistry, WithStateStore, WithEventHandler, WithVariables; no logger option.
- `xclconfig:config.go:75-120` — Config struct; NewConfig defaults registry with NewStdOutLogger (`:116`).
- `xclconfig:config.go:199-310` — Validate/Apply/Destroy build a fresh parser each; Apply saves once at `:256-260`; Destroy loads state at `:285` before parser.
- `xclconfig:config.go:88-99` — addressParser caches Types() forever.
- `xclconfig:internal/parser/events.go:6-55` — ParserEvent + fire helpers; string literals, no constants.
- `xclconfig:internal/parser/parser.go:51-197` — ParserOptions (Logger `:66`, OnParserEvent `:93`), DefaultOptions uses StdOutLogger (`:119`), NewParser zero StdOutLogger (`:166-169`).
- `xclconfig:internal/parser/parser.go:231-303` — Apply: parseAndValidate, removal destroyer, walk, partial state on error.
- `xclconfig:internal/parser/parser.go:511,538,562,581,595,607,881,1031` — parse event emission sites (caller goroutine).
- `xclconfig:internal/parser/parser.go:1235-1280` — walk; `log.SetOutput(io.Discard)` at `:1269`.
- `xclconfig:internal/parser/destroy.go:35-131` — destroyer, mutex, save per resource; `log.SetOutput` at `:69`.
- `xclconfig:internal/parser/lifecycle.go:76-375` — builtin create success `:76`, provider lookup `:80`, callProvider `:321-350`, warnChangedConfiguredValues `:304-316`.
- `xclconfig:internal/parser/callbacks.go:23-32,37-287` — ProviderResolver/TypeRegistry interfaces; walkCallback; destroyWalkCallback emission; missing-provider/marshal errors fire no event (`:225-254`); panics at `:57,:69`.
- `xclconfig:internal/parser/configured_check.go:17-25` — the parser's only log call (Warn configured_value_changed).
- `xclconfig:internal/parser/progress.go:23,44-76` — applyProgress, buildState.
- `xclconfig:internal/parser/test_plugin.go:245-343` — TestPlugin Init resets calls; providers store logger.
- `xclconfig:internal/parser/recording_logger_test.go` — Warn recorder used by configured_check_test, lifecycle_test (`:972-1008`), registered_types_test (`:388`).
- `xclconfig:internal/dag/walk.go:80-131,293-300,360-440` — goroutine per vertex; no logging; waitDeps cascade; no recover.
- `xclconfig:logger/logger.go:4-9` — Logger interface (Info/Debug/Warn/Error msg, args...).
- `xclconfig:logger/tagged.go`, `logger/event.go` — WithTag and event-first text formatting (to be replaced).
- `xclconfig:logger/stdout_logger.go:18-26` — only library use of charmbracelet/log; writes os.Stdout.
- `xclconfig:logger/test_logger.go:27` — NewTestLogger, widely used in tests.
- `xclconfig:logger/pretty_printer.go` — unused resource printer; out of scope.
- `xclconfig:plugins/adapter.go:25-247` — ProviderAdapter; TypedProviderAdapter Init tags provider (`:86-95`); debugCall "calling provider" (`:101-112`) to remove.
- `xclconfig:plugins/plugin.go:33-206` — RegisteredType, PluginEntityProvider, Plugin(Init/SetLogger/SetState), RegisterResourceProvider (`:57`), PluginBase methods use context.Background.
- `xclconfig:plugins/provider.go:16-103` — ResourceProvider[T]; every lifecycle method takes ctx.
- `xclconfig:plugins/direct_plugin_host.go:16-107` — in-process host; tags plugin=; Init at construction; "plugin loaded" log `:32`.
- `xclconfig:plugins/grpc_plugin_host.go:27-265` — Start spawns process with hclog adapter (`:54`); wrapper drops ctx; GetTypes cache without lock.
- `xclconfig:plugins/grpc_plugin.go:15-88` — GRPCServer Init(nil,nil); host callback server with fixed logger; HandshakeConfig.
- `xclconfig:plugins/grpc_server.go:31-191` — every RPC: getLogger + SetLogger; getLogger caches without lock.
- `xclconfig:plugins/grpc_clients.go:14-96` — GRPCLogger sends LogRequest, args stringified.
- `xclconfig:plugins/grpc_host_callback.go:28-115` — host forwards to fixed logger; stringArgsToInterfaces re-types.
- `xclconfig:plugins/grpc_resource_adapter.go:19-55` — Init no-op; drops ctx and force.
- `xclconfig:plugins/hclog_adapter.go:24-162` — hclog → Logger with event=go-plugin; drops Trace and EOF noise.
- `xclconfig:plugins/plugin.proto` — canonical proto; `plugins/protos/plugin.proto` is a stale invalid draft.
- `xclconfig:plugins/interfaces.go:6` — `type Logger = logger.Logger`.
- `xclconfig:plugins/registry/plugin_registry.go:18-484` — registry struct (no mutex), API, eager load, DiscoverAndLoadPlugins(logger,...) `:352-404`.
- `xclconfig:plugins/registry/plugin_discovery.go:13-166` — PluginDiscovery with logger; event=discover logs.
- `xclconfig:plugins/registry/errors.go` — TypeNameClashError only.
- `xclconfig:plugins/registry/*_test.go` — ~35 registry tests + discovery/load tests rely on eager errors and GetPluginHosts; buildExamplePlugin helper.
- `xclconfig:plugins/testing/helpers.go:29-73,320-329` — InProcess/ExternalPluginSetup(WithLogger), BuildPlugin.
- `xclconfig:plugins/example/*` — person plugin (in-process + external), e2e_test asserts tagged log strings (`:513-660`).
- `xclconfig:state/file_state_store.go:17,111` — store holds registry; Load calls CreateResource.
- `xclconfig:config_events_test.go` — eventRecorder with mutex; applyQueryFixtureWithEvents; test style to follow.
- `xclconfig:example/{plugin,configonly,appconfig}/main.go` — run(out, log, ...) pattern; NewPluginRegistry(log); WithEventHandler(eventlog.Handler(log)).
- `xclconfig:example/eventlog/eventlog.go` — current example handler, to be replaced.
- `xclconfig:example/*/main_test.go` — recordingLogger assertions on tagged strings; to be rewritten against events.
- `xclconfig:example/plugin/internal/plugin.go`, `example/plugin/external/main.go` — providers store Init logger and pass resource ID manually.
- `xclconfig:README.md:138-144,206-210,248,410-431`, `docs/plugins.md:32-283`, `docs/plugin-developer-guide.md:21-397`, `docs/parser-lifecycle.md:208-290`, `docs/README.md:38-46`, `plugins/README_test_helpers.md`, `CHANGELOG.md` — library docs to update.
- `xclconfig:.mockery.yml` — mocks for plugins.State, plugins.ProviderAdapter, state.StateStore, parser.ProviderResolver; regenerate with `make mocks` after signature changes.
- `xcl-website:src/pages/index.mdx:90,173-177` — NewStdOutLogger registry snippet; lifecycle events feature card.
- `xcl-website:src/pages/examples/plugins.mdx:137-367` — plugin/provider logger usage, run() copy, log output blocks.
- `xcl-website:src/pages/examples/configuration-only.mdx:222,252,292`, `application-config.mdx:278,294` — NewPluginRegistry(log), eventlog handler.
- `xcl-website:src/components/Nav.astro:8-18`, `README.md` Pages table — navigation to extend for new pages.
- `xcl-website:package.json`, `Makefile` — `npm run build`, `make check` (`npx astro check`).

## External references

- Go `log/slog` (Go 1.21+) — standard structured logging target for the shipped adapter; `slog.Handler` level filtering gives "filter by severity" for free.
- `github.com/charmbracelet/log` v0.4.2 `logger_121.go` — `*log.Logger` implements `slog.Handler`, so the pretty example is `slog.New(charmlog.New(os.Stderr))` behind the slog adapter.
- `github.com/hashicorp/go-plugin` v1.6.3 `server.go:275-278,347-355`, `client.go:407-411,1162-1237` — plugin stderr/hclog output reaches the host's `ClientConfig.Logger`; plugin stdout/stderr streams are discarded when `SyncStdout/SyncStderr` are unset.
- gRPC-Go `metadata` package — per-call metadata on the outgoing context carries the call ID from host to plugin without message changes.
- Go spec / runtime: deferred functions run on the panicking goroutine before unwinding, and a deferred function that does not call `recover` lets the panic continue with its original value and stack.

## Prior plans / specs consulted

- Spec `20260922061954-event-based-logging` — source of truth for this plan.
- `.spektacular/working-context.md` spec-interview notes (2026-09-22) — user decisions: flat Event + Meta, reserved `level`/`message`, `Source` = `core` or plugin name, lazy loading, block-don't-drop queue, panic re-raise with graceful stop.
- `CHANGELOG.md` entry `20260919152915-destroy-cycle` (not the plan itself) — introduced the event-first log line format (`event=<name>`, `calling provider`, `plugin loaded` → `event=load`, `event=go-plugin`, `event=discover`, `event=configured_value_changed`); these names map onto event operations here.
- Knowledge `architecture/ux-flow.md`, `architecture/config-is-the-public-query-surface.md` — Config is the public entry point; no logging guidance. No design references on the spec (`design ref list` → 0).

## Open assumptions

- The in-process `DirectPluginHost` and external host can both carry a per-call logger in `context.Context`; plugin authors retrieve it with a `plugins` package function from the ctx their provider method receives. If the user requires the Init-time logger itself to be call-bound, STOP and ask.
- A plugin logging outside a provider call (e.g. `Plugin.Init`, go-plugin's own stderr/hclog lines) is routed to the receiver of the operation currently using the registry (last started wins, silent when none). If this proves insufficient for "events go to the configuration that produced them", STOP and ask.
- Plugin load failure is cached per registry: once a load has failed, every later operation on that registry returns the same error without retrying.
- Lifecycle (non-log) events map to slog Info in the shipped adapter (error phase → Error, blocked → Warn), matching the acceptance criterion that an "info" logger shows lifecycle events.
- `go list -deps` of every non-example package excluding charmbracelet is an acceptable proof that charm "stays out of the library's dependencies" while it remains in the root `go.mod` for the example.
- Validate-stage (reference/property) problems currently fire no event; emitting one `validate`/`error` event per problem plus one operation-level error event is not a "duplicate" because they are distinct occurrences.

## Drafting assumptions

### Chosen direction: caller-drained event stream, ctx-bound loggers, lazily loaded registry (architecture)
- **Decision**: Option A. (1) `events` leaf package holds the flat Event + slog adapter; `logger` becomes a wrapper that emits log events. (2) Each operation runs its work on a worker goroutine while the caller's goroutine drains a bounded queue into the receiver; a cancellable operation context + deferred non-recovering shutdown gives graceful stop with the original panic. (3) Per-call loggers bound to resource+step travel in ctx; across gRPC via a call ID in metadata and `LogRequest.call_id`. (4) Registry records plugins and loads once, on first operation, emitting to that operation.
- **Key design decisions**: caller goroutine as the delivery goroutine (keeps panic value + stack); ctx-carried logger instead of Init/SetLogger (concurrency-safe); plugin-name Source added by hosts, resource/step bound by the parser; out-of-call plugin logs go to the registry's active emitter; blocked announcement via a one-slot side channel; load result (including failure) cached per registry.
- **Rejected**: Option B — dedicated delivery goroutine with recover/re-panic plus per-call SetLogger re-Init (loses stack or changes panic value; races on shared providers). Option C — synchronous handler behind a mutex (violates fire-and-forget; slow receiver slows provider calls). Effort: A High, B Medium-High, C Low.

### Caller's goroutine delivers events; work runs on a worker goroutine (discovery)
- **Decision**: Validate/Apply/Destroy run their work on a worker goroutine while the calling goroutine drains the event queue into the receiver. A receiver panic therefore happens on the caller's goroutine, and a deferred shutdown (cancel the operation context, wait for in-flight calls, save state) runs without recover so the panic continues unchanged.
- **Rationale**: This is the only way to re-raise with the *same* panic value *and* the receiver's original stack; the spec's suggested "recover on delivery goroutine, re-panic on caller" cannot keep the stack without wrapping the value. It still satisfies fire-and-forget, one-at-a-time delivery and flush-before-return.
- **Rejected**: Dedicated delivery goroutine + recover + re-panic (loses stack or changes value); wrapping the value in a `ReceiverPanic` type (changes the value).

### Per-call provider logger travels in context.Context (discovery)
- **Decision**: Before each provider call the host puts a logger bound to resource + step into ctx; providers get it with a `plugins` package function (e.g. `plugins.Logger(ctx)`). Across gRPC, a call ID in gRPC metadata lets the plugin build a logger whose `LogRequest`s carry the call ID; the host maps the call ID to the bound logger.
- **Rationale**: Providers are called concurrently for different resources, so any logger stored on the provider (Init/SetLogger) races. ctx is already a parameter of every provider method.
- **Rejected**: Re-running Init/SetLogger per call (racy); resource context in every request message (doesn't solve routing to the right Config).

### Plugin logs outside a provider call use the registry's active receiver (discovery)
- **Decision**: Logs a plugin writes outside a provider call (Plugin.Init, go-plugin/hclog lines, plugin stderr) go to the receiver of the operation currently using the registry — last started wins, silent when none.
- **Rationale**: Such logs have no call to attribute them to; the spec's "events go to the configuration that produced them" is framed around operations' plugin log messages, which are call-scoped.
- **Rejected**: Dropping them (loses diagnostics); broadcasting to all receivers (violates routing).

### Charm stays in root go.mod, out of library packages (discovery)
- **Decision**: The pretty example lives in the root module; no library package imports charmbracelet, verified with `go list -deps`.
- **Rationale**: Examples have no go.mod of their own; splitting modules adds build complexity for no consumer benefit given module-graph pruning.
- **Rejected**: A separate example module.

### Lifecycle events log at Info in the slog adapter (discovery)
- **Decision**: Log events use their own level; lifecycle/load/discover events map to Info, error-phase events to Error, blocked announcements to Warn.
- **Rationale**: Acceptance says an "info" slog logger shows lifecycle events and no debug log events.
- **Rejected**: Lifecycle at Debug (fails acceptance).

### Registry discovery API renamed to DiscoverPlugins and only records (architecture)
- **Decision**: Replace `DiscoverAndLoadPlugins(logger, dirs, pattern)` with `DiscoverPlugins(dirs []string, pattern string)` that records directories; scanning and loading happen in `Load`.
- **Rationale**: Spec requires lazy loading and no logger; keeping the "AndLoad" name would misdescribe it. Backwards compatibility is not required.
- **Rejected**: Keeping the old name with changed semantics.

### Discovered plugins that fail to start are rejected, not fatal (architecture)
- **Decision**: An explicitly registered plugin (RegisterPlugin / RegisterPluginWithPath) that fails to load, or any plugin type clash, fails the operation. A discovered binary that fails to start is reported with a `load` error-phase event (rejected) and skipped, unless no discovered plugin loads at all (today's rule).
- **Rationale**: Discovery is a best-effort scan (today it only logs partial failures, `plugin_registry.go:393-401`); the acceptance scenario with one valid and one invalid discovered plugin expects the events, not a failure.
- **Rejected**: Failing on any discovered plugin (turns a stray file in a plugin directory into a hard failure).

### Operation-level events and filled-in error events (architecture)
- **Decision**: Each Validate/Apply/Destroy emits `validate|apply|destroy` start and success/error events (the error carrying the returned error). Error sites that emit nothing today (missing provider, marshal/decode/context errors, validate-stage problems, plugin load, state save) gain an error event.
- **Rationale**: "Every failure is also emitted as an event"; the operation-level error is a distinct occurrence (the call failed), so it is not a duplicate.
- **Rejected**: Only a final operation error (loses per-resource attribution); only per-site events (easy to miss a site).

### Removing logger.TestLogger and StdOutLogger (architecture)
- **Decision**: Delete `logger/stdout_logger.go`, `logger/event.go`, the text-formatting `taggedLogger`, and `logger/test_logger.go`. Tests assert on recorded events instead.
- **Rationale**: A stdout logger in the library contradicts "silent by default" and pulls charm into library imports; TestLogger exists only to feed registries/parsers a logger, which they no longer take.
- **Rejected**: Keeping them as optional helpers (dead weight; charm stays in library import graph).

### Default event buffer size 1024 (architecture)
- **Decision**: `WithEventBufferSize(n)`; default 1024; n < 1 treated as 1.
- **Rationale**: Large enough that typical configurations never block, small enough to bound memory; spec leaves the value to the planner.
- **Rejected**: Unbounded (violates spec), very small default (needless blocking).

### PluginEntityProvider gains ctx only; Destroy's missing force stays out of scope (data_structures)
- **Decision**: Add `ctx` as the first parameter of Validate/Create/Destroy/Update/Changed on `PluginEntityProvider`; do not add `force` to the plugin-level Destroy or the gRPC DestroyRequest.
- **Rationale**: ctx is needed to carry the bound logger; the dropped `force` flag is an unrelated pre-existing gap.
- **Rejected**: Fixing force in the same change (scope creep).

### Event has no Line field (data_structures)
- **Decision**: Resource context on events is ResourceID, ResourceType and File; no line number.
- **Rationale**: The spec asks for identity, type and declaring file; adding Line is harmless but unrequested.
- **Rejected**: Adding Line/Column.

### Static source-scan tests enforce the "no direct output" metrics (testing_approach)
- **Decision**: Enforce "0 direct log calls" and "examples use one receiver" with Go tests that parse the repository's own sources (go/parser), with an explicit exclusion list (internal/xcl, internal/cty, logger/pretty_printer.go).
- **Rationale**: Makes the success metrics fail `go test` on regression instead of relying on review.
- **Rejected**: Manual review only; a lint tool dependency.

### Three milestones: core stream, plugins + lazy registry, examples + docs (milestones)
- **Decision**: M1 delivers the stream/runner/panic shutdown for core events with plugins still on the old logger (default registry built with a nil logger so xcl is silent); M2 converts the logger package and plugin runtime/registry; M3 examples and docs.
- **Rationale**: Each milestone compiles and is testable alone; the riskiest concurrency work lands first and is validated before the wide plugin signature change.
- **Rejected**: One big milestone (unreviewable); plugins first (would need the stream anyway).

### Resources skipped by cancellation are treated as not reached (phases)
- **Decision**: (Revised at walkthrough on the user's suggestion: an operation context replaces the halt flag.) A provider step skipped because the operation context is cancelled returns an unexported `errNotReached`; apply treats the resource as not reached (previous state entry kept, new resource omitted); destroy leaves it in state untouched.
- **Rationale**: Matches "state afterwards matches the resources that actually exist" and reuses the existing partial-state builder unchanged.
- **Rejected**: A hand-rolled `atomic.Bool` halt flag (user preferred context cancellation — idiomatic and gives the walk a real context); passing the cancellable context straight to providers (would abort in-flight calls, violating the spec); marking skipped resources failed (would make the next apply destroy-then-recreate resources that were never touched).

### Pretty example defaults to info, configurable by XCL_LOG_LEVEL (phases)
- **Decision**: `prettylog.Handler(w, level)` with `LevelFromEnv()` reading `XCL_LOG_LEVEL` (default info); example providers log at info so they show by default; output goes to stderr, program report to stdout.
- **Rationale**: Readable default output while still demonstrating severity filtering.
- **Rejected**: Debug by default (noisy); hard-coded level (hides the filtering feature).

### Provider panics on the worker goroutine stay out of scope (phases)
- **Decision**: The runner does not catch panics raised by providers on walker goroutines; behaviour matches today (process crash).
- **Rationale**: The spec only covers receiver panics.
- **Rejected**: Recovering provider panics (new behaviour not asked for).

### Stale plugins/protos/plugin.proto is deleted (phases)
- **Decision**: Delete the invalid draft proto while editing the canonical one.
- **Rationale**: It is not generated from, is invalid, and would confuse whoever edits the log contract.
- **Rejected**: Leaving it.

### Plan slug is the CLI-assigned spec name (assemble)
- **Decision**: Use `20260922061954-event-based-logging`, the name `plan new` assigned, rather than an `NNNN-` slug.
- **Rationale**: Every existing plan in the store uses the spec's timestamped name, and the CLI already fixed the plan path.
- **Rejected**: Proposing a `0007-...` slug (inconsistent with the store).

## Rehydration cues

- `spektacular spec file read 20260922061954-event-based-logging.md` — spec.
- Read `.spektacular/working-context.md` — interview decisions and plan-workflow notes.
- Re-read `events.go`, `options.go`, `config.go`, `internal/parser/events.go`, `internal/parser/lifecycle.go:300-375`, `internal/parser/callbacks.go:37-287`, `internal/parser/destroy.go`, `internal/dag/walk.go:280-440`.
- Re-read `plugins/plugin.go`, `plugins/adapter.go`, `plugins/direct_plugin_host.go`, `plugins/grpc_*.go`, `plugins/hclog_adapter.go`, `plugins/plugin.proto`, `plugins/registry/plugin_registry.go`, `plugins/registry/plugin_discovery.go`.
- `grep -rn "logger\." --include=*.go . | grep -v internal/xcl | grep -v internal/cty` — remaining logger call sites.
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website` — conventions (no table tests, require, mockery, shared errors package).
