# Parser & Resource Lifecycle

This is the engine that turns parsed HCL into a dependency-ordered sequence
of provider calls. It lives in `internal/parser/`.

## `Parser.Apply` — the single entry point

```go
func (p *Parser) Apply(ctx context.Context, paths ...string) (*State, error)
```

([`internal/parser/parser.go`](../internal/parser/parser.go)) does,
in order:

1. `parseAndValidate` — load previous state, parse all HCL files under
   `paths` into Go resource instances (via `PluginRegistry.CreateResource`
   for typing), then **validate the configuration as a whole**. Nothing is
   acted upon unless validation passes, and validation itself decodes no
   bodies and reaches no provider.
2. Reject a configuration that declares no blocks with
   [`ErrEmptyConfiguration`](../internal/parser/errors.go) ("the configuration
   declares no blocks, use Destroy to remove everything"). Applying it would
   remove everything, so nothing is destroyed, created, changed or saved.
3. Destroy the resources that are in the previous state but no longer in the
   configuration ([`removedResources`](../internal/parser/parser.go#L332)),
   children first, before anything is created or changed. This uses the same
   destroyer as `Destroy`, see [Destroy](#destroy).
4. Build a DAG from resource dependencies (`internal/parser/dag.go`) —
   explicit `depends_on`, plus implicit edges from cross-resource
   references discovered during parsing (`Meta.Links`). The parents each
   resource ends up with are recorded in `Meta.Parents`.
5. Walk the DAG in dependency order.
6. Decode each resource body (`gohcl.DecodeBody`) once its dependencies'
   values are available.
7. Look up the resource in the previous state and call
   `Create`, or `Read`+`Changed`+`Update`, or `Destroy`+`Create`, on the
   resource's provider.

If a removal in step 3 fails, `Apply` stops there; if a provider call fails
in the walk, `Apply` returns the state the walk reached. Either way the error
comes back with a state to save, see
[State saved after a failed apply](#state-saved-after-a-failed-apply). An
apply that removes nothing skips step 3 entirely. `ctx` is the operation's
context; once it is cancelled no new provider call starts, see
[The operation context](#the-operation-context).

```go
func (p *Parser) Validate(ctx context.Context, paths ...string) error
```

is the checking half on its own: it runs `parseAndValidate` and stops there,
so it never reaches steps 2-7. This is what `Config.Validate` calls. It
reaches no provider, so it takes `ctx` only for symmetry with `Apply` and
`Destroy`.

### The validation gate

Validation sits between parsing and walking, and runs three stages in order —
**structure**, then **references**, then **properties**
([`internal/parser/validate.go`](../internal/parser/validate.go)):

- **structure** — resources too malformed to check further. This is also
  where computed fields are checked: a computed field that is not optional,
  and a computed field set in configuration, are both reported here (see the
  [Plugin Developer Guide](plugin-developer-guide.md#computed-fields)).
- **references** — everything a resource refers to must be defined somewhere
  in the configuration ([`references.go`](../internal/parser/references.go)).
  A reference is resolved against its referring resource's module scope first
  and the whole configuration second.
- **properties** — a reference's trailing property path must name properties
  the target's type actually has
  ([`properties.go`](../internal/parser/properties.go)). Selecting a member of
  a collection continues the walk against the member's type; the walk stops
  and accepts the remainder only where the type itself stops being knowable.

Each stage gathers every problem it finds before returning, and a later stage
is skipped when an earlier one found anything.

`Parser` is stateless across calls — `Config` constructs a new one for
every `Apply`/`Validate`/`Destroy` (see [Overview](overview.md)).

## `walkCallback` and `resourceLifecycle`

The DAG walker (`internal/dag`, a copy of `github.com/silas/dag`) invokes one callback per vertex.
[`walkCallback`](../internal/parser/callbacks.go#L37) is that callback: it
decodes the resource's HCL body, handles module-specific evaluation-context
setup, and then hands the resource to `resourceLifecycle.apply`
([`internal/parser/lifecycle.go`](../internal/parser/lifecycle.go)).

`resourceLifecycle` picks the provider calls from the resource's entry in the
previous state, the state saved by the last apply:

```
not in previous state
    -> Create                                  status created

saved as created or updated
    -> carry computed values from the saved copy onto the configured copy
    -> Read(saved, configured)
         ErrNotFound -> reset to the configured copy, Create   status created
         other error -> status failed, the apply fails
    -> Changed(saved, read result)
         true  -> Update(read result)          status updated
         false -> keep the read result and the previous status

saved as failed, destroy_failed, or anything else (rebuild)
    -> Destroy(saved copy)
         error -> keep the saved copy          status destroy_failed
    -> Create(configured copy)                 status created
```

A rebuild happens whether or not the resource's configuration changed. When
the rebuild's `Destroy` fails, `Create` is not called; the saved copy is kept
because it holds the identity needed to try the destroy again on the next
apply.

After `Create`, `Read` and `Update`, the lifecycle compares the resource it
sent with what the provider returned and emits a warn log event for every
configured (non-computed) value the provider changed
([`configured_check.go`](../internal/parser/configured_check.go)): message
`provider changed a configured value`, the resource and step on the event and
the field path in `Meta["field"]`. The warning never fails the apply. The
[Plugin Developer Guide](plugin-developer-guide.md) describes what providers
may change in each call.

Builtin types with no provider (`variable`, `output`, `module`, the DAG
root) are skipped early — see [Builtin types](#builtin-types) under Events
below for a subtlety here.

### Destroy

```go
func (p *Parser) Destroy(ctx context.Context, saved []any) (*State, error)
```

([`internal/parser/parser.go`](../internal/parser/parser.go#L301)) destroys
every resource in `saved` and needs no configuration. `Config.Destroy`
([`config.go`](../config.go#L155)) passes it the state loaded from the
`StateStore` (or the in-memory state when there is no store); when nothing
has been saved, or the state is empty, it returns nil without calling the
parser and writes nothing. A load failure is returned as
`failed to load state: ...`.

The work is done by the
[`destroyer`](../internal/parser/destroy.go#L22), which the removal phase of
`Apply` uses too:

- [`buildDestroyDAG`](../internal/parser/dag.go#L124) builds a graph with the
  same shape as the create graph, from each resource's recorded
  `Meta.Parents`: an edge from each parent in the set to the resource, and
  resources with no parent in the set hang off a root. Parents that are not
  being destroyed are ignored.
- The graph is walked with `dag.Walker{Reverse: true}`, so every child is
  destroyed before its parents and unrelated resources are destroyed in
  parallel. A parent is never visited once one of its children has failed.
- [`destroyWalkCallback`](../internal/parser/callbacks.go#L188) handles one
  resource. `variable`, `output`, `module`, registered (config-only) types and
  disabled blocks never reach a provider: they fire a `destroy` success event
  and are removed. Every other resource is passed to its provider's `Destroy`
  as the saved copy, with `force` always false.
- After each resource the working state is updated and saved through the
  `StateStore`: a destroyed resource is removed, a failed one is kept and
  marked `destroy_failed`. The saved state is correct at every step, so an
  interrupted destroy resumes from it. A failed save fails the step, so that
  resource's parents are not visited either.

When a destroy fails, the failed resource and everything it depends on (its
parents, transitively) stay in the state, while unrelated resources are still
destroyed. The error names every failed resource (`destroy failed for <id>:
...`), and calling `Destroy` again retries what is left. `Parser.Destroy`
returns the remaining state, never nil, and `Config.Destroy` adopts it as its
current state.

Resources saved before `Meta.Parents` was recorded have no parents, so they
are destroyed with no ordering guarantee between them.

## Resolving the provider: `ProviderResolver`

Both callbacks need to turn a resource into a `plugins.ProviderAdapter`.
Rather than depending on the concrete `*registry.PluginRegistry`, they
depend on a narrow interface defined in this package:

```go
// internal/parser/callbacks.go
type ProviderResolver interface {
    GetProviderForResource(resource any) plugins.ProviderAdapter
}
```

`*registry.PluginRegistry` satisfies this structurally (Go interfaces are
implicit), so production code is unaffected — `ParserOptions.PluginRegistry`
is still what gets passed in practice. But `walkCallback`,
`destroyWalkCallback`, and `resourceLifecycle` only ever call this one
method, so tests can substitute
[`internal/parser/mocks.MockProviderResolver`](../internal/parser/mocks/mock_provider_resolver.go)
instead of standing up a real registry + real (or fake) plugin.

`ParserOptions` exposes this as its own field, defaulting to
`PluginRegistry` when unset:

```go
// internal/parser/parser.go
ProviderResolver ProviderResolver // overrides provider lookup; defaults to PluginRegistry
```

This is what `TestParserProcessesResourcesInCorrectOrder` uses to verify
DAG-walk ordering with a single mock adapter/resolver pair instead of the
hand-written `TestPlugin` fake ([`internal/parser/test_plugin.go`](../internal/parser/test_plugin.go)).
Note `PluginRegistry.CreateResource` (used in step 1 of `Apply`, to
instantiate resources from HCL) is a *different* method not covered by
`ProviderResolver` — a real registry is still needed for that part even in
tests that mock the lifecycle-call path.

## Events: `ParserOptions.Emit`

The parser reports everything it does as `events.Event` values
([`events/events.go`](../events/events.go)), the one flat event shape xcl and
its plugins share. Set `ParserOptions.Emit` (an `events.Emit`) and the parser
calls it for every event; a nil `Emit` is silent. The helpers in
[`internal/parser/events.go`](../internal/parser/events.go) (`emit`,
`emitLifecycle`, `emitParse`, `emitOperationError`) are nil-safe and skip
building an event nobody receives. `Emit` may be called from several
goroutines at once, since unrelated resources are walked in parallel; the
runner in `Config` orders them for the receiver (see
[Operation events and the runner](#operation-events-and-the-runner)).

```go
type Event struct {
    Time         time.Time      // stamped when emitted if zero
    Source       string         // events.SourceCore ("core") for the parser
    Operation    string         // "parse", "validate", "apply", "create", "read", "changed", "update", "destroy", ...
    Phase        string         // "start", "success", "error", "log", "blocked"
    ResourceType string         // "<type>.<name>", e.g. "container.base"
    ResourceID   string         // full resource ID, e.g. "resource.container.base"
    File         string         // the file the resource was declared in
    Duration     time.Duration  // time spent in the provider call; 0 for "start"
    Error        error          // set for "error"
    Data         []byte         // the resource serialized to JSON; nil for builtin types
    Meta         map[string]any // details, e.g. a log event's level and message
}
```

Operations and phases are constants in the `events` package
(`events.OperationCreate`, `events.PhaseStart`, ...), not string literals.

### Operation and phase

For lifecycle events, `Operation` names the provider method being called.
`Phase` says where in that call the event was fired:

| Phase | Fired | `Duration` | `Error` |
|---|---|---|---|
| `start` | immediately before the provider method is called | 0 | nil |
| `log` | for each message the provider logs during the call, see below | 0 | nil |
| `success` | after the method returns without error | time the call took | nil |
| `error` | after the method returns an error | time the call took | the provider's error |

Every provider call is bracketed: one `start`, any number of `log` events,
then exactly one of `success` or `error`.

The parser also emits:

- **`parse`** — a `success` or `error` for every block as it is read, with
  the `File` it came from. A file that is not valid syntax gives an error
  with only `File` set.
- **`validate`** — an `error` for every problem the
  [validation gate](#the-validation-gate) finds, naming the resource declared
  at the problem's position when there is one, otherwise only the file.
- **`apply`** / **`destroy`** errors with no resource — a failure of the
  operation as a whole, such as a dependency graph that can't be built.
- **`apply`** errors for a resource that failed in the walk before any
  provider call, such as a body that does not decode. A resource whose type
  has no provider gets an error on the step it would have started with
  (`create`, `read` or `destroy`).

Every failure the parser returns is also emitted as an error event.

### Event sequences per resource

Which operations fire depends on the resource's entry in the previous state
([`lifecycle.go`](../internal/parser/lifecycle.go)):

- **New resource** — `create` start, then `create` success or error.
- **Existing resource** (saved as `created` or `updated`) — `read`
  start/success-or-error, then `changed` start/success-or-error, then, only
  if `Changed` reported a change, `update` start/success-or-error. When
  `Read` returns `ErrNotFound`, the `read` error event is followed by
  `create` start/success-or-error instead.
- **Rebuilt resource** (saved as `failed` or `destroy_failed`) — `destroy`
  start/success-or-error, then, if the destroy succeeded, `create`
  start/success-or-error.
- **Removed resource** (in the previous state, no longer configured) —
  `destroy` start, then `destroy` success or error, before any other
  resource is processed.
- **Destroyed resource** (`Config.Destroy`) — `destroy` start, then
  `destroy` success or error.

### Builtin types

`variable`, `output` and `module` resources have no provider. They fire a
single `create` success event on apply, or `destroy` success when destroyed,
with zero duration, no error and no data, and no `start` event. Registered
(config-only) types and disabled blocks do the same on destroy. This keeps
every visited resource in the stream, which matters if you consume events to
reconstruct processing order.

### Errors and control flow

Events never affect control flow. Every provider error is handled the same
way, whichever operation it came from:

- An error from `create`, `read`, `changed`, `update` or `destroy` marks the
  resource `failed` (`destroy_failed` for any `destroy`), stops the
  walk from reaching the resources that depend on it (on a destroy walk, the
  resources it depends on), and is returned from `Apply` or `Destroy`.
  Resources that don't depend on it still complete.
- The one exception is `plugins.ErrNotFound` from `read`: the `error` event
  fires, but the lifecycle creates the resource again instead of failing.

## The operation context

`Validate`, `Apply` and `Destroy` take the operation's `context.Context`.
`Config` cancels it only when the application's event receiver panics (see
below), but the parser treats any cancellation the same way:

- **A cancellation check before each provider call.** `callProvider`
  ([`lifecycle.go`](../internal/parser/lifecycle.go)) and the destroy
  callback ([`callbacks.go`](../internal/parser/callbacks.go)) check
  `ctx.Err()` before starting a call. Once it is cancelled no new call
  starts and no event is fired for it; the resource is treated as not
  reached, so it keeps its previous entry in the state. `Apply` returns the
  partial state with `apply stopped before every resource was reached`
  wrapping `ctx`'s error, and `Destroy` returns what is left with
  `destroy stopped before every resource was reached`.
- **A provider context that is never cancelled.** Each call is made with
  `providerContext(...)` ([`events.go`](../internal/parser/events.go)),
  built on `context.WithoutCancel(ctx)`, so a call already running always
  finishes and its result is recorded.
- **A per-call bound logger.** `providerContext` also puts a logger in the
  call's context with `plugins.WithLogger`. It is `logger.New(Emit, base)`,
  where `base` carries the resource's `ResourceID`, `ResourceType` and `File`
  and the step as `Operation`, so every message a provider writes through
  `plugins.Logger(ctx)` becomes a `log` event between the call's `start` and
  its `success` or `error`, with no resource details passed by the provider.
  The plugin's host sets the event's `Source` to the plugin's name (see
  [Plugin logging](plugins.md#plugin-logging)).

## Operation events and the runner

`Parser` lives under `internal/`; applications receive its events through
`Config`, which wraps every `Validate`, `Apply` and `Destroy` in `run`
([`config.go`](../config.go)) and delivers to the handler set with
`xcl.WithEventHandler`:

- The operation emits its own `validate`, `apply` or `destroy` `start` event
  first, and a `success` or `error` event, carrying the error the call
  returns, last. Everything else, plugin `discover`/`load` events, `parse`
  events and the resource lifecycle, comes in between.
- Before the work, `withPlugins` calls the registry's `Activate(emit)`, so
  messages plugins write outside a provider call reach this operation, then
  `Load(emit)`, which loads the plugins on the first operation only.
- With no handler, the work runs directly with a nil `Emit` and nothing is
  started. With one, the work runs on a worker goroutine and emits into an
  [`internal/eventstream`](../internal/eventstream/stream.go) `Stream`, a
  bounded queue of `WithEventBufferSize` events (default
  `DefaultEventBufferSize`, 1024). The calling goroutine runs `Drain`, which
  calls the handler one event at a time in emit order.
- Emitting waits only when the queue is full and never drops an event. Each
  stretch of waiting is announced once, afterwards, as an `events` /
  `blocked` event whose `Duration` is how long emitters waited; it is held
  aside and delivered ahead of the queue, so it never takes a queue slot.
- `run` returns once the work has finished and every event it emitted has
  been delivered.
- A panic in the handler happens on the calling goroutine and is not
  recovered. The deferred shutdown cancels the operation context, so no new
  provider call starts, calls `Discard` so emitters blocked on the full queue
  are released and later events dropped, and waits for the worker, which lets
  calls in progress finish and saves state. The panic then continues with
  the handler's own value and stack.

Tests inside this module set `ParserOptions.Emit` directly to assert on
provider calls and DAG-walk order.

## State saved after a failed apply

When destroying a removed resource fails, nothing is created or changed.
`Parser.Apply` returns the previous state minus the resources that were
destroyed, with the failed ones (and, as with `Destroy`, everything they
depend on) still in it, the failures marked `destroy_failed`. The removal has
already saved this after each resource, and `Config.Apply` saves it again
before returning the error. The next apply retries the removal first.

When the walk fails, `Parser.Apply` still returns a state, built by
`applyProgress.buildState`
([`internal/parser/progress.go`](../internal/parser/progress.go)), together
with the error:

- resources the walk reached are included as they are now, with their new
  values and status;
- the failing resource is included as `failed`, or `destroy_failed` if the
  rebuild's destroy failed;
- resources that existed in the previous state but were not reached keep
  their previous entry;
- new resources that were not reached are left out.

A resource whose error came before any provider call (a decode error, or no
provider for its type) is treated as not reached.

`Config.Apply` ([`config.go`](../config.go#L110)) saves this state and then
returns the error, so the next apply picks up where this one stopped. When
parsing or validation fails, the configuration declares no blocks, or the
dependency graph can't be built, `Parser.Apply` returns a nil state and
nothing is saved.
