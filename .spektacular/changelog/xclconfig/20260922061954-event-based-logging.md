---
created_date: "2026-09-22"
document_status: draft
project: xclconfig
spec: 20260922061954-event-based-logging
plan: 20260922061954-event-based-logging
---

# Event-based logging

xcl no longer prints anything itself. Everything it and its plugins have to say, lifecycle steps, plugin loading, warnings, errors and plugin log messages, now reaches your application as one ordered stream of events through `WithEventHandler`, which you can send to `log/slog` in one line. Plugin authors write ordinary log messages from the call's context, and those messages arrive labelled with the resource, step and plugin, whether the plugin runs in-process or as a separate process. Plugins now load on the first validate, apply or destroy instead of when they are registered.

> Derived from project xcl (xclconfig), spec/plan 20260922061954-event-based-logging. See the project-level record for the full feature.

## What changed in this repo

- **Events.** The new `events` package holds the flat `Event`, which adds `Time`, `Source` and `Meta` fields; the `Handler` and `Emit` types; the reserved `level`/`message` keys; the level, phase, operation and source constants; and `events.SlogHandler`. `xcl.Event` and `xcl.EventHandler` are now aliases.
- **Delivery.** The new `internal/eventstream` package is a bounded per-operation queue. It never drops events and announces blocking once per stretch. `Config` now runs every operation through one runner. The runner emits operation start and success/error events, delivers every event before returning, and on a receiver panic stops new provider calls, saves state and lets the panic continue unchanged. Adds `WithEventBufferSize` and `DefaultEventBufferSize` (1024).
- **Parser.** `ParserOptions.Emit` replaces `OnParserEvent` and `Logger`, and the parser's `Validate`/`Apply`/`Destroy` take an operation context. The parser checks that context before every provider call and gives providers a copy that is never cancelled, carrying a logger bound to the resource and step. Every returned failure has an error event. The configured-value warning is a warn log event. The `log.SetOutput(io.Discard)` calls are gone.
- **Logger.** The `logger` package only builds log events (`New`, `Nop`, `WithTag`, `WithSource`). `NewStdOutLogger`, `NewStdOutLoggerWithOptions`, `NewTestLogger` and the `event=` text format are removed.
- **Plugins.**
  - `plugins.Logger(ctx)` and `plugins.WithLogger` are added. `ctx` is threaded through `PluginEntityProvider`, `PluginHost`, `PluginBase` and the gRPC wrapper, adapter and server.
  - `Plugin.SetLogger` and the `plugins.Logger` type alias are removed. Host constructors take an `events.Emit`, and hosts name the plugin as the source.
  - `LogRequest` gains `call_id`, and the host sends `xcl-call-id` metadata so external plugin logs keep their call's context. Plugin binaries must be rebuilt.
  - go-plugin's messages become events tagged `component=go-plugin`. The "calling provider" log is gone.
- **Registry.** `NewPluginRegistry()` takes no arguments. `RegisterPlugin`, `RegisterPluginWithPath` and `DiscoverPlugins` (which replaces `DiscoverAndLoadPlugins`) only record. `Load` runs once per registry and emits `discover`/`load` events. `Activate` routes out-of-call plugin messages. A plugin that fails to load produces `ErrPluginLoad` / `PluginLoadError`, now in the shared `errors` package and re-exported from `xcl`.
- **Examples.** `example/prettylog` replaces `example/eventlog`. Every example takes an `EventHandler` and sets up the styled receiver in one line, and the example plugins log through `plugins.Logger(ctx)`.
- **Tests and docs.** New static tests keep library code free of direct output and of charmbracelet dependencies, and keep the examples to one receiver. The README, `docs/`, the plugin test-helper notes and `CHANGELOG.md` are updated.

## Why

Applications couldn't silence or redirect xcl's own output, and plugin messages couldn't always be tied to the resource or plugin they came from. This repo holds the library, the examples and their docs, so the whole behaviour change lives here. The one known exception is a line go-plugin writes to standard error when an external plugin is stopped within milliseconds of starting, which is outside xcl's control.
