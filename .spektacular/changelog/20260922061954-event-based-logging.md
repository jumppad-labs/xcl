---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Event-based logging: one silent, ordered stream for everything xcl reports

## What was built

xcl no longer writes log output of its own. Everything it and its plugins report goes to the embedding application as one ordered stream of events, through the existing `WithEventHandler`. That covers every step of a resource's lifecycle, plugin discovery and loading, warnings, errors, and the log messages plugin authors write. With no receiver set, xcl is silent and leaves the application's own logging settings alone.

**One event shape.** A new leaf package, `events`, holds the single flat `Event`: the existing lifecycle fields plus a time, a source (`core`, or the plugin's name) and a generic details map. A log message is an event whose phase is `log`, with its severity and text under the reserved `level` and `message` keys, which caller details can never overwrite. `xcl.Event` and `xcl.EventHandler` are aliases, so existing receivers keep compiling. A ready-made `events.SlogHandler` connects the stream to `log/slog` in one line, and the slog handler's level does the severity filtering.

**Delivery.** Every `Validate`, `Apply` and `Destroy` now runs through one runner. The runner emits the operation's own start and success or error events, runs the work on a worker goroutine, and delivers events to the receiver on the caller's goroutine, one at a time and in order. Emitting never waits for the receiver, only for room in a bounded buffer (`WithEventBufferSize`, default 1024). A full buffer makes emitters wait rather than drop anything, and each stretch of waiting is announced by a single `blocked` event that never takes a buffer slot. Every event has been delivered before the call returns. If the receiver panics, xcl stops starting provider calls through a new operation context, lets calls already in progress finish, saves state, and lets the receiver's panic continue with its original value and stack.

**Errors are events.** Every failure the parser returns now has a matching error event. That includes the ones that were silent before: a missing provider, serialisation and decode problems, validation problems and graph-build failures. The configured-value warning is a warn log event bound to its resource and step.

**Plugin logging with context.** Providers log through `plugins.Logger(ctx)`. Before every provider call xcl binds a logger to the resource, its type, the file it was declared in and the lifecycle step, and the plugin's host names the plugin as the source, so a plugin author passes none of it. External plugins get the same behaviour. Each call carries an ID over gRPC metadata, the plugin returns it on every log message (`LogRequest.call_id`), and the host routes each message to the right resource, step and configuration. Messages written outside a call, including go-plugin's own, go to the operation currently using the registry. The duplicate "calling provider" message and the per-call re-initialisation of external plugins are gone.

**Lazy plugin registry.** The registry takes no logger. Registering a plugin, a plugin path or a discovery directory only records it. Plugins are discovered, started and clash-checked once per registry, however many configurations share it, on the first `Validate`, `Apply` or `Destroy`. Discovery, loading and rejection are reported as `discover` and `load` events. A plugin that fails to load fails that operation with `ErrPluginLoad`, whose detail names the plugin. Registering a type that clashes with a builtin or registered type still fails immediately.

**Examples and docs.** A styled terminal receiver, built on the slog adapter with a charmbracelet handler, replaces the old example event logger. Every example sets it up in one line, and the example plugins log without passing any resource context. Only the examples import charm; a test checks that no library package depends on it. The library README and docs and the xcl.dev site (two new guide pages plus updated example pages) describe the event stream, connecting it to a logger, plugin logging, and lazy loading.

## Why it matters

Before this change, some of xcl's output went straight to standard output, where the application could not turn it off or redirect it, and some of it could not be tied back to the resource or plugin it came from. Sending events could also slow processing down. Application authors now get a single filterable view of what xcl is doing, with every message labelled by its source, resource and step. Plugin authors keep writing ordinary log messages, and xcl stays silent unless the application asks to hear from it. Lazy loading also prepares for plugins to be declared in configuration instead of code.

## Deviations from the plan

- **Two error helpers in the parser.** An unexported `reportedError` wrapper marks errors that already have an event, so failures outside a provider call (serialisation, state lookup) get an `apply` error event exactly once. Graph-build failures emit operation-level error events.
- **Cancelled operations report the context's error.** A cancelled apply returns its partial state plus an error wrapping the context's error, and a cancelled destroy returns the wrapped error. The walk's lifecycle error now keeps the provider's error as its cause, so `errors.Is` matches what `Apply` returned.
- **The `plugins.Logger` type alias was removed.** It could not coexist with the planned `plugins.Logger(ctx)` function; code uses `logger.Logger`.
- **In-process adapters are wrapped.** The registry hands the parser each in-process plugin's own adapters, bypassing the host's methods, so the host wraps those adapters to stamp the plugin name as the source.
- **The registry owns the load events.** Hosts don't emit them, so a clash found after a plugin starts isn't reported as success followed by error. While `Load` runs, plugins' own messages go to the loading operation. The parser also calls the cached `Load`, so a parser used on its own keeps working. `TestPlugin.Init` no longer resets the test configuration.
- **Asynchronous `Init` logger in the plugin process.** go-plugin initialises the plugin before the host has connected, so a synchronous logger would stall start-up.
- **Accepted exception to "silent by default"** (the plan's open question, decided by the user). go-plugin v1.6.3 writes `[ERR] plugin: plugin acceptAndServe error: broker closed` through the standard library's global logger when an external plugin is stopped within milliseconds of starting. It can't be routed without patching go-plugin or changing the global logger. It is documented, and the `Config`-level no-output tests all pass.
- **Example plugins moved early.** They switched to `plugins.Logger(ctx)` during Milestone 2 rather than in Phase 3.1, because the external one panicked on its nil `Init` logger once per-call re-initialisation was gone.
- **Other small changes.** The plugin example's "build it with `make build`" hint now wraps the `ErrPluginLoad` returned by `Apply`. `lipgloss` became a direct requirement at the same pinned version. The `provider=<block type>` detail on `Init` loggers applies to in-process plugins only.
- **Tooling.** The Spektacular binary was upgraded to 0.20.0 partway through implementation, and the project was migrated before the workflow continued.
