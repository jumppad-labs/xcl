# Working context: event-based-logging spec

## Problem / motivation
- User: "I don't think that the code should log in the traditional sense. What I think it should do is broadcast events which could be captured by a logger."
- Today there are two parallel systems: lifecycle events (`xcl.Event`, `WithEventHandler`, `events.go`) and a `logger.Logger` interface called directly by library code (plugin discovery/load in `plugins/registry`, configured-value warning `internal/parser/configured_check.go:23`, adapter "calling provider" debug `plugins/adapter.go:107`, direct host "plugin loaded", gRPC-forwarded provider logs `plugins/grpc_host_callback.go`).
- Log calls already smuggle `"event", "<name>"` keys; `logger/event.go` has `splitEvent`/`leadWithEvent` to parse them back out — logger used as an ad-hoc event bus.
- Library writes to stdout by default: `config.go:116` and `internal/parser/parser.go:119/168` hard-code `logger.NewStdOutLogger()`.
- `log.SetOutput(io.Discard)` in `internal/parser/parser.go:1269` and `internal/parser/destroy.go:69` mutates the process-wide stdlib logger (likely silencing the dag walker) — must go.

## Decisions (user-confirmed)
- User: "A log anywhere in the code base or plugin should be translated to a log event which has all the details on it like level. This way we keep conceptual logging but logs are written to the event stream which can be output to a standard logger."
- User: "internal logging libraries just becomes a convenience wrapper for a standard event of operation log."
- Keep the flat `Event` struct (no typed event interface). Add `Time time.Time`, `Source string`, `Meta map[string]any`.
- User: "Meta can be generic ... message should go into meta for log events, so should level." No Level/Message fields; `level` and `message` are reserved Meta keys (exported constants; logger's values win on collision). Level is a string ("debug","info","warn","error").
- Logs carry the resource/step: user: "it should be bound, ideally also per workflow step, i.e if Refresh is being called on a plugin then that should be automatically set." Adapter binds a logger to resource ID + operation before each provider call (create/read/update/changed/destroy — there is no Refresh; read is the equivalent). Host fills ResourceType and File (and Line) from `types.Meta`.
- Proposed shape for log events: `Operation` = the step in progress, `Phase: "log"`. Stream reads start → log → log → success. Logs outside provider calls use core operation names like "discover"/"load".
- User: "we need source on an event which would be plugin or the part of the core. This will enable filtering later." `Source` = "core" or plugin name; proposed "emitter" semantics (lifecycle events are "core"). User did not object.
- Library never writes output; no handler => silent. Ship an slog adapter (formatting + level filtering in the adapter, not the library).
- `WithTag` values go into Meta; a resource tag fills ResourceID.
- User: "PluginRegistry should not have a logger, it should emit events that the main application can tap into." Extend the existing `xcl.WithEventHandler(eventlog.Handler(log))` pattern.
- Lazy plugin loading chosen: registry records plugins/paths/discovery dirs; load happens on first Validate/Apply/Destroy with Config's handler. Load once (sync.Once/mutex) since external plugins spawn processes; handler supplied per call, not captured at load. User: "there is a benefit to failing fast with plugins ... but I think eventually I would like plugin registration to move to configuration rather than code. In this instance lazy loading would be preferred."
- Keep immediate clash checks for builtin/registered types in RegisterType; plugin clashes surface at load time.
- `NewPluginRegistry()` and `DiscoverAndLoadPlugins(dirs, pattern)` lose their logger params.
- Event/EventHandler move to a leaf package (e.g. `events`) to avoid import cycles; `xcl` keeps type aliases. Replaces duplicate `parser.ParserEvent`.

## Alternatives rejected
- Typed event interface (ResourceEvent/PluginEvent/...) — user preferred flat Event + generic Meta.
- Level/Message as dedicated fields — moved into Meta.
- Positional resource ID in log call — ambiguous with key/value pairs; binding instead.
- Registry-owned handler (`registry.WithEventHandler`) / buffering / passing handler twice — replaced by lazy loading.

## Open / out of scope
- Plugin registration in HCL configuration — future direction, out of scope.
- gRPC log args are strings today (`stringArgsToInterfaces`); Meta values from external plugins will be strings unless proto changes.

## Interview answers (spec workflow)
- Docs site `xcl-website` is in scope.
- Breaking changes OK: "we have not yet released."
- No duplicate messages: remove adapter "calling provider" debug log.
- Delivery: fire-and-forget emit ("we should not have to wait for the receiver to ack an event"); bounded queue ("I have no idea what people are going to do with this library"); block when full; raise a "queue blocked" event that must not itself be the reason for blocking (out-of-band, never takes a queue slot); single delivery goroutine; drain before Validate/Apply/Destroy return; configurable size.
- Errors: "we should still add an error to the event stream as the receiver would want to log them but it should cause termination as current."
- Interview complete; synthesis in .spektacular/work/20260922061954-event-based-logging/interview.md
- Overview confirmed by user.
- Requirements confirmed by user.
- Receiver panics: user: "We def should not [absorb], it is their responsibility... If we can detect a panic and shut down gracefully. That would be very cool." Agreed: stop new provider calls, let in-flight finish, save state, stop delivery, re-raise original panic (value + receiver stack) from Validate/Apply/Destroy on caller's goroutine. Added two requirements; acceptance criteria confirmed.
- Constraints confirmed (3 bullets). Backwards-compat freedom and slog preference go to Technical Approach.
- Technical Approach confirmed. Design doc declined by user ("more concerned about concept than actual implementation") — do not re-offer.
- Success metrics confirmed.
- Non-goals confirmed (incl. value types across plugin boundary may be strings).
- Charm logger: user wants a nice-looking charmbracelet/log example receiver; recorded as an example (not library API), built on the slog adapter with charm as slog handler.
- Fresh-eyes review applied (13 fixes). Spec committed to store as 20260922061954-event-based-logging.md. Next: spek-plan.

# Plan workflow: 20260922061954-event-based-logging (started 2026-09-22)
- User chose spec 20260922061954-event-based-logging to plan against.
- Repos: xclconfig root /home/nicj/code/github.com/jumppad-labs/xcl (library); xcl-website root /home/nicj/code/github.com/jumppad-labs/xcl-website (docs, in scope).
- Spec has 5 success metrics that must be carried into Testing Approach.
- Discovery done (research.md + assumptions.md in .spektacular/work/<plan>/). Key learnings:
  - No cancellation exists in walks today; independent branches keep calling providers after a failure. Need a halt flag checked before each provider call.
  - Handler is currently called concurrently from dag walker goroutines; no recover anywhere.
  - log.SetOutput(io.Discard) is dead: internal/dag never uses stdlib log.
  - Latent nil-deref: NewParser defaults Logger to zero StdOutLogger.
  - Provider logger stored at Init races under concurrent calls -> per-call logger via ctx; gRPC routing via call ID in metadata + LogRequest.call_id.
  - Destroy loads state (needs plugin types) before parser -> lazy load at the very start of each op.
  - Design choice: caller goroutine drains events, work on worker goroutine, so a receiver panic keeps original value+stack (deviates from spec wording "catch on delivery goroutine").
- Architecture drafted (Option A chosen: caller-drained queue + worker goroutine, ctx-bound loggers w/ gRPC call ID, lazy registry Load(emit) once). Default buffer 1024; DiscoverPlugins(dirs, pattern) records only; TestLogger/StdOutLogger deleted.
- Components drafted (events, slog adapter, eventstream, runner, logger, parser, plugin runtime, registry, errors, prettylog example, docs, website).
- Data structures drafted (events.Event w/ Time/Source/Meta; Emit/Handler; logger.New/WithTag/Nop; plugins.Logger(ctx); registry Load/Activate; LogRequest.call_id + xcl-call-id metadata; eventstream.Stream).
- Implementation detail drafted (single runner; ctx logger; registry record/load split; halt check in walk callbacks; error sites emit).
- Dependencies drafted; no design docs; protoc/mockery installed locally.
- Testing approach drafted; all 5 success metrics mapped (4 static/behavioural tests + stdout-pipe test); pretty styling & website manual.
- Milestones: M1 core stream+runner+panic; M2 plugin logging ctx/gRPC + lazy registry; M3 examples+docs+website.
- Phases drafted: 1.1 events+slog, 1.2 eventstream, 1.3 parser emit/halt, 1.4 runner/panic; 2.1 ctx logger+logger cleanup, 2.2 lazy registry, 2.3 gRPC call-id; 3.1 prettylog+examples+static tests, 3.2 lib docs, 3.3 website.
- Open questions: go-plugin host stderr leakage (STOP+ask if found); panic test determinism (use TestPlugin hook).
- Out of scope drafted.
- Assembled & verified staged docs in .spektacular/tmp/ (plan/context/research templates); removed shell commands from plan.md; context sections reordered.
- All three plan docs committed to store; work dir removed. Now in walkthrough.
- Walkthrough: user asked to use context cancellation instead of halt flag -> applied (op ctx cancelled on receiver panic; walk callbacks check ctx.Err(); providers get context.WithoutCancel; errNotReached unexported; no ErrHalted). Walkthrough beat 1 done; next: phases.
- Walkthrough complete: user signed off ('great commit it') on 2026-09-22.

## Implement workflow (2026-09-22)
- Plan `20260922061954-event-based-logging` chosen by the user for implementation on branch `event-based-logging`.
- read_plan: structure valid; no code drift (no non-.spektacular changes since plan commit 51f1c0b); every spec requirement/AC covered; no `## Changelog` in plan → first-phase mode, starting at Phase 1.1.
- Repo roots: xclconfig = /home/nicj/code/github.com/jumppad-labs/xcl; xcl-website = /home/nicj/code/github.com/jumppad-labs/xcl-website (Phase 3.3 only).
- Phase 1.1 analysis: `logger.WithTag(l, key, value string)` has 3 callers (adapter, direct host, grpc host), all strings — widening value to `any` is safe. Old taggedLogger stays until 2.1.
- Phase 1.1 implemented: `events/events.go`, `events/slog.go`, root `events.go` aliases (+ adapted `parserEventHandler` stamping Time/Source), `logger/emit.go` (`New`, `Nop` = New(nil,…), eventLogger.withTag), `WithTag(l, key string, value any)` dispatches to eventLogger. Nop is an eventLogger with nil emit so WithTag works on it.
- Phase 1.1 tests: events/slog_test.go, events/imports_test.go, logger/emit_test.go, config_event_fields_test.go.
- Phase 1.1 verify: all green (build, vet, gofmt, race tests, full suite, events deps stdlib-only).
- Phase 1.1 complete. User chose "Continue without commit": do not commit, and ask again after each phase.
- Phase 1.2 analysis: new self-contained package, no touchpoints. Discard needs no drain goroutine because Emit's fast path is a non-blocking send and the slow path selects on discardCh.
- Phase 1.2 implemented at internal/eventstream/stream.go. Deviation: Discard starts no drain goroutine, because the fast path never blocks.
- User (after 1.2 implement): "just keep going until you are done". Autonomous mode: loop through phases without asking. Still no commits.
- Phase 1.2 done and verified. Helper scripts for ticking phases and appending the changelog are in the session scratchpad (tick.sh).
- Phase 1.3 implemented. Decisions:
  - The cancellation check is in `callProvider` (it returns `errNotReached`) and at the top of `walkCallback`/`destroyWalkCallback`. A cancelled walk with no other errors makes `Parser.Apply` return the partial state plus a wrapped `ctx.Err()`, and `destroyer.destroy` returns a wrapped `ctx.Err()`.
  - `reportedError` (in internal/parser/errors.go) marks errors already emitted. `lifecycle.apply` emits `apply`/`error` for any unreported failure; graph-build failures emit operation errors.
  - The interim default registry is `NewPluginRegistry(logger.Nop())`, not `nil`, because hosts call methods on the logger and nil would panic. Phase 2.2 removes the param.
  - `config.emitFor()` is an interim emitter that stamps Time/Source and calls the handler directly. 1.4 replaces it.
  - The `warnChangedConfiguredValues` signature was kept (its `id` param is now unused, as the plan says).
- Phase 1.3 done and verified (race suite green).
- Phase 1.4 implemented: Config.run in config.go and WithEventBufferSize/DefaultEventBufferSize in options.go. Existing root event tests need updating because validate/apply/destroy start+success events are new.
- Phase 1.4 done and verified. Milestone 1 complete.
- Phase 2.1 implemented. Decisions:
  - The `plugins.Logger` type alias (plugins/interfaces.go) is deleted because it clashes with the planned `plugins.Logger(ctx)` func. Code inside plugins uses `logger.Logger`.
  - The parser hands in-process providers the plugin's own adapters (via host.GetTypes), bypassing DirectPluginHost methods. So DirectPluginHost wraps each adapter in a `sourcedAdapter` that re-sources the ctx logger with the plugin name. `logger.WithSource` was added.
  - `parser.providerContext()` (internal/parser/events.go) builds each provider call's ctx: WithoutCancel plus a bound logger.
  - TestPlugin gained `LogMessage`, `SetLogOnCreate` and `SetLogOnRead`.
  - Load events: `{Source core, Operation load, Phase success, Meta{plugin, block_types}}` via `plugins.loadedEvent`.
  - The registry still passes a nil emit to the hosts (until 2.2). Examples use `logger.Nop()` (until 3.1).
- Phase 2.1 done and verified. 6 skipped example/plugin tests must be restored in 3.1 (grep 'restored as an event assertion').
- Phase 2.2 implemented. Decisions:
  - The registry owns the load events (start/success/error, Meta{plugin, block_types | path, rejected}). Hosts no longer emit them, so a clash found after start isn't reported as success then error.
  - `registry.pluginEmit` forwards to the *loading* emitter during Load, otherwise to the *active* one.
  - `Config.withPlugins` does Activate(emit) (a no-op for nil), then Load(emit), then the work. `Parser.parseAndValidate` and `Parser.Destroy` also call the cached `Load`, so a standalone parser works.
  - `plugins.PluginName`, `PluginBinaryName` and `ResourceTypeNames` are now exported for the registry. `errors.PluginLoadError.Unwrap` returns `[]error{ErrPluginLoad, Err}`.
  - `Config.addressParser` isn't cached until the registry has loaded.
- Mid-session the Spektacular binary was upgraded to 0.20.0. The user ran migrate; the workflow resumed from 2.2's update_plan.
- Phase 2.3 implemented. Decisions:
  - `plugins/grpc_calls.go` holds `callLoggers` (a sync.Map of call ID → logger, IDs are nonce-counter), `callIDKey = "xcl-call-id"` and `callIDFromContext`.
  - The wrapper's `callContext` registers the ctx logger re-sourced to the binary name and sends the ID as outgoing metadata. It's nil-safe when a test builds the wrapper without a registry.
  - The callback server's `loggerFor(req)` resolves the call ID and falls back to the plugin-scoped logger.
  - Plugin side: `GRPCServer.withLogger` builds a `GRPCLogger{client, callID}`, and `hostClient()` dials once under a mutex.
  - `Impl.Init` now receives `server.pluginLogger()`, an `asyncLogger` that sends from goroutines. Init runs before the host has connected (go-plugin calls GRPCServer before the handshake), so a synchronous dial would block startup until the broker times out.
  - The stale `plugins/protos/plugin.proto` was deleted, and the protos regenerated with the same generator versions.
- User decision (Phase 2.3 open question): go-plugin v1.6.3 prints "[ERR] plugin: plugin acceptAndServe error: broker closed" through stdlib log.Printf (grpc_broker.go:382) when an external plugin is stopped within milliseconds of starting. The user said: "Go-plugin is not within our control, it is an external tool. I think this exception is acceptable." It is documented as an exception (in 3.2/3.3 docs) and the host-level test asserting silence was removed. The Config-path no-output tests remain.
- Phase 2.3 done. Milestone 2 complete.
- Phase 3.1 implemented:
  - `example/prettylog` (Handler(w, level), LevelFromEnv, XCL_LOG_LEVEL) replaces `example/eventlog`.
  - The example mains take `run(out, handler xcl.EventHandler, ...)`, and `main` makes the single `prettylog.Handler(os.Stderr, prettylog.LevelFromEnv())` call.
  - The plugin example's "make build" hint now wraps `ErrPluginLoad` from Apply, inside `run` rather than `main` so its test sees it.
  - The example providers log at info via plugins.Logger(ctx) ("created database" etc.) and no longer store a logger. Plugin and provider Init log at debug through the Init logger.
  - lipgloss is now a direct go.mod requirement at the same v1.1.0.
- 3.2 started in parallel with the 3.1 test step: README.md done by me; docs/, plugins/README_test_helpers.md and CHANGELOG.md delegated to an agent.
- Phase 3.1 done and verified.
- Phase 3.2 done.
- Phase 3.3 done. All phases complete.
- The test plan was written with 2 manual procedures (styled receiver, docs site).
- Changelog records written: project, xclconfig, xcl-website.
- Spec reconciled: all 64 requirement and acceptance-criteria boxes are ticked (the go-plugin line is a user-accepted exception, not descoped).
