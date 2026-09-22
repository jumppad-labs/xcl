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
