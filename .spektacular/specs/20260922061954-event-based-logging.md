---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Feature: 20260922061954-event-based-logging

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

xcl will stop writing log output of its own. Instead, everything it or its plugins have to report (progress through each resource's lifecycle, plugin discovery and loading, warnings, errors, and messages written by plugin authors) goes out as one stream of events that the embedding application receives and can send to whatever logger it uses. Today some of this goes straight to standard output, which the application can't turn off or redirect. Some of it can't be tied back to the resource or plugin it came from. Sending events can also slow down processing. With this change, application authors get a single, filterable view of what xcl is doing, with every message labelled by its source, resource and step. Plugin authors keep writing ordinary log messages without passing that context themselves, and xcl stays silent unless the application asks to hear from it.

<!--
  REQUIREMENTS
  Specific, testable behaviours the feature must deliver.
  Format: bold title on the checkbox line, detail indented below.
  Rules:
    - Use active voice: "Users can...", "The system must..."
    - Each requirement should be independently verifiable
    - Focus on WHAT, not HOW — avoid prescribing implementation
    - Keep each item atomic — one behaviour per line
-->
## Requirements

**Silence and output**

- [x] **Silent by default**
  xcl and its plugins must produce no output of their own (standard output, standard error, or files) when the application has not asked to receive events.
- [x] **No global side effects**
  xcl must not change process-wide logging settings that belong to the embedding application.
- [x] **One stream for everything**
  Everything xcl or a plugin reports must reach the application through the single event stream it subscribes to. No messages are delivered through any other channel.
- [x] **Ready-made logger adapter**
  Application authors can send the event stream to a logger from Go's standard library with one line of setup, including filtering by severity.
- [x] **Pretty logger example**
  The repository includes an example receiver that renders the event stream as readable, styled terminal output, and every example program uses it.

**Log messages as events**

- [x] **Log calls become events**
  Code in xcl and in plugins can write a log message with a severity, a message and arbitrary key/value details, and the application receives it as an event carrying all three.
- [x] **Severity is carried, not acted on**
  Every log event states its severity (debug, info, warn or error). xcl must not filter or suppress events by severity; that is the receiver's choice.
- [x] **Arbitrary detail travels with the event**
  Any key/value details given with a log message, or attached to any other event, reach the receiver unchanged in name. The reserved names for severity and message cannot be overwritten by caller-supplied details.
- [x] **Every event is timestamped**
  Every event records the time it happened.
- [x] **Every event names its source**
  Every event identifies whether it came from xcl itself or from a plugin, and if from a plugin, which one.
- [x] **No duplicate messages**
  A single occurrence must produce a single event. For example, starting a provider call produces one event, not a lifecycle event plus a log message saying the same thing.

**Context is attached automatically**

- [x] **Resource and step are bound for plugin authors**
  A log message written by a plugin while xcl is calling it for a resource carries that resource's identity, its type, the file it was declared in, and the lifecycle step being performed, without the plugin author supplying any of them.
- [x] **Works across the plugin boundary**
  Automatic context and log events behave the same for plugins running inside the application's process and for plugins running as separate processes.
- [x] **Logs sit within their step**
  Log events written during a lifecycle step can be identified as belonging to that step, alongside that step's start, success and error events.

**Plugin registry**

- [x] **Registry needs no logging setup**
  Plugins can be registered and discovered without the application setting up any logging.
- [x] **Plugin discovery and loading are reported as events**
  Discovering, loading, and rejecting plugins, and their outcomes, are reported on the same event stream as everything else.
- [x] **Plugins load when first needed**
  Plugins are discovered and loaded when the configuration is first validated, applied or destroyed, not when they are registered. A plugin that fails to load causes that operation to fail with an error.
- [x] **Plugins load once per registry**
  A registry shared by several configurations loads each plugin only once.
- [x] **Events go to the configuration that produced them**
  Each configuration's events, including plugin log messages during its operations, go to that configuration's receiver and to no other.
- [x] **Immediate checks where possible**
  Registering a type whose name clashes with a builtin or already-registered type still fails immediately. Clashes with plugin-provided types are reported when plugins load.

**Delivery**

- [x] **Fire and forget**
  Emitting an event must not wait for the receiver to process or acknowledge it.
- [x] **Bounded buffering**
  The number of undelivered events xcl holds is limited; the application can configure the limit, and a default applies.
- [x] **Block, don't drop, when full**
  When the limit is reached, emitting waits for space. No events are lost.
- [x] **Blocking is announced**
  When emitting has been held up because the limit was reached, the receiver is told with an event saying so and how long it lasted. This announcement must not itself count towards the limit or add to the blocking, and a single blocked stretch produces a single announcement.
- [x] **Delivered in order, one at a time**
  The receiver gets events in the order they were emitted and is never called concurrently.
- [x] **Everything delivered before returning**
  When a validate, apply or destroy call returns, every event produced by that call has been delivered to the receiver.

**Errors**

- [x] **Errors still stop processing**
  A failure stops processing and is returned to the caller exactly as it is today, regardless of the event stream or the receiver.
- [x] **Errors are also events**
  Every such failure is also emitted as an event so the receiver can log it.
- [x] **Receiver cannot affect processing**
  Apart from a panic, which ends the run as described below, nothing the receiver does (including being slow, beyond making emitters wait when the limit is reached) changes the outcome of a validate, apply or destroy call.
- [x] **Receiver panics are not absorbed**
  A panic in the receiver is never swallowed. It is raised again to the application, with the same panic value and the receiver's original stack trace, from the validate, apply or destroy call that was running.
- [x] **Receiver panics shut down gracefully**
  Before the panic is raised again, xcl starts no new provider calls, lets calls already in progress finish, and records xcl's record of created resources (state) as it does after a failure, so that the state afterwards matches the resources that actually exist.

**Documentation**

- [x] **Docs site updated**
  The documentation site describes the event stream, how to connect it to a logger, how plugin authors write log messages, and the change to plugin registration and loading.

<!--
  CONSTRAINTS
  Hard boundaries the solution must operate within. These are non-negotiable.
  Format: one bullet point per constraint.
  Examples:
    - Must integrate with the existing authentication system
    - Cannot introduce breaking changes to the public API
    - Must support the current minimum supported runtime versions
  Leave blank if there are no constraints.
-->
## Constraints

- The event stream must extend the existing mechanism applications already use to subscribe to xcl's events. It must not be a second, parallel subscription.
- All events, log messages included, must share one flat event shape. Anything specific to one kind of event, including a log message's severity and text, goes in the event's generic key/value details, not in extra dedicated fields or separate event types.
- The plugin registry must not hold or accept a logger; plugin activity is reported only through the event stream.
- Plugins running as separate processes must stay supported, communicating with the host over the existing plugin protocol. Their log messages must reach the event stream across that boundary.

<!--
  ACCEPTANCE CRITERIA
  The specific, binary conditions that define "done".
  Format: bold title on the checkbox line, verifiable detail indented below.
  Each criterion must be:
    - Independently verifiable (pass/fail, not subjective)
    - Traceable back to a requirement above
    - Testable by someone who didn't write the code
-->
## Acceptance Criteria

- [x] **Silent by default**
  An application that validates, applies and destroys a configuration using an in-process plugin and an external plugin, with no event receiver set, writes nothing to standard output or standard error, and xcl creates no log files.
- [x] **No global side effects**
  An application that writes a message with the standard library logger after validating, applying or destroying a configuration sees that message appear at its original destination.
- [x] **One stream for everything**
  With a receiver set, running validate, apply and destroy using both plugin kinds produces no output outside the receiver, and every message a plugin logs appears in the receiver.
- [x] **Ready-made logger adapter**
  Connecting the shipped adapter to a standard-library logger at "info" writes info, warn and error log events plus lifecycle events to that logger, and writes no debug log events.
- [x] **Pretty logger example**
  Running any example program shows its lifecycle events, plugin log messages and errors through the styled example receiver, with each line showing severity, source, and resource and step where relevant.
- [x] **Log calls become events**
  A plugin that logs a message with severity "info", text "something happened" and the detail `remote_id=213` produces one event the receiver sees with that severity, that text and that detail.
- [x] **Severity is carried, not acted on**
  Log messages written at debug, info, warn and error each reach the receiver with that severity. None are missing when no filtering adapter is used.
- [x] **Arbitrary detail travels with the event**
  A detail given with a name that is not reserved reaches the receiver under that exact name. A detail named like a reserved name does not change the event's severity or message.
- [x] **Every event is timestamped**
  Every event the receiver gets has a time between the start and end of the call that produced it.
- [x] **Every event names its source**
  Events produced by xcl itself name xcl as their source. Events produced by a plugin name that plugin, for both an in-process and an external plugin.
- [x] **No duplicate messages**
  Applying a single resource backed by a provider gives the receiver exactly one event marking the start of the provider's create call, and no log event restating it.
- [x] **Resource and step are bound for plugin authors**
  A provider that logs from inside its create and read calls, without passing any resource information, produces log events carrying the resource's ID, type, source file and the step (create or read respectively).
- [x] **Works across the plugin boundary**
  The same provider logic run as an in-process plugin and as an external plugin produces log events with the same severity, message, detail names, detail values when rendered as text, resource context and step. Only the source name may differ.
- [x] **Logs sit within their step**
  For a resource whose provider logs during create, the receiver sees that resource's create start, then the log event marked as belonging to create, then create success.
- [x] **Registry needs no logging setup**
  A plugin registry can be created, have plugins registered on it, and have plugin directories added for discovery without supplying a logger.
- [x] **Plugin discovery and loading are reported as events**
  Validating a configuration with a discovery directory holding one valid and one invalid plugin gives the receiver events reporting the discovery, the successful load and the rejection.
- [x] **Plugins load when first needed**
  Registering an external plugin path that does not exist returns no error at registration. The first validate, apply or destroy fails with an error naming the plugin. No external plugin process runs before that first call.
- [x] **Plugins load once per registry**
  Two configurations sharing one registry, each validated, start each external plugin process only once.
- [x] **Events go to the configuration that produced them**
  With two configurations sharing one registry, each with its own receiver, each receiver gets the plugin log events for its own configuration's operations and none from the other.
- [x] **Immediate checks where possible**
  Registering a type with the same name as a builtin type, or as an already-registered type, returns a clash error at registration. Registering a type whose name matches a plugin's type returns no error at registration, and the first validate fails with a clash error.
- [x] **Fire and forget**
  With a receiver that takes one second per event, a limit larger than the number of events produced, and N events, the last provider call completes well before N seconds have passed, and apply returns only after the receiver has handled every event.
- [x] **Bounded buffering**
  An application can set the limit on undelivered events. Leaving it unset uses a documented default.
- [x] **Block, don't drop, when full**
  With a limit of 1 and a slow receiver, applying a configuration that produces many events delivers every event. The count received equals the count produced with a fast receiver.
- [x] **Blocking is announced**
  In the scenario above, the receiver gets a "blocked" event reporting how long emitting was held up. One blocked stretch produces exactly one such event. The run still completes and delivers every other event.
- [x] **Delivered in order, one at a time**
  A receiver that records whether it is ever entered while already running never detects concurrent entry, and each resource's events arrive in start, log, success/error order.
- [x] **Everything delivered before returning**
  Immediately after validate, apply or destroy returns, including when it returns an error, the receiver has already received every event for that call, including the error event.
- [x] **Errors still stop processing**
  A configuration where one provider fails on create returns an error from apply, and resources that depend on the failed one are not created. This is the same with and without a receiver.
- [x] **Errors are also events**
  In the scenario above, the receiver gets an error event for the failed resource carrying the same failure that apply returned.
- [x] **Receiver cannot affect processing**
  A receiver that is slow, or that ignores every event, produces the same apply result and the same created resources as no receiver.
- [x] **Receiver panics are not absorbed**
  With a receiver that panics on the third event, the apply call itself panics with the receiver's panic value. An application that recovers around apply gets that value and a stack trace containing the receiver's code.
- [x] **Receiver panics shut down gracefully**
  In the scenario above, with several independent resources, every resource whose create finished is recorded in state, no resource is recorded that wasn't created, and no provider call starts after the panic.
- [x] **Docs site updated**
  The documentation site has pages covering the event stream and its contents, connecting it to a logger, logging from a plugin, and plugin registration and lazy loading. It has no remaining references to passing a logger to the plugin registry.

<!--
  TECHNICAL APPROACH
  High-level technical direction to guide the planning agent. Include:
    - Key architectural decisions already made
    - Preferred patterns or technologies if known
    - Integration points with existing systems
    - Known risks or areas of uncertainty
  Format: one bullet point per direction/steer.
  Leave blank if you want the planner to propose the approach.
-->
## Technical Approach

- Add the time and the source (`core` for xcl itself, or the plugin's name) to the existing event, alongside the generic details map from Constraints. Suggested reserved keys are `level` and `message`, published as constants.
- A log event carries the lifecycle step in progress as its operation and is marked with a `log` phase, so a resource's stream reads start → log… → success/error. Logs outside provider calls use core operation names such as discover and load.
- The logging API that core code and plugins call stays. It becomes a thin wrapper that builds a log event and emits it. Tags go into the details map, except a resource tag, which fills in the resource identity.
- Before each provider call, the adapter gives the provider a logger bound to the resource and the step. The host fills in the resource type and declaring file from the resource's own metadata. External plugins get the same binding through the per-call setup that already happens across the plugin protocol.
- Remove the provider adapter's "calling provider" log line. The lifecycle start event replaces it.
- Move the event types into a small package of their own that the parser, plugin and registry code can all import without cycles. It replaces the parser's duplicate event type, and the top-level package re-exports it.
- Deliver events through a bounded queue drained by a single goroutine, sized by a config option. Send the "blocked" notification outside the queue, and flush the queue before validate, apply and destroy return.
- When the receiver panics, catch the panic on the delivery goroutine and trigger the same graceful stop the error path uses. Then raise the panic again from the calling method on the caller's goroutine, keeping the original stack trace.
- The plugin registry records plugins, paths and discovery directories. Load once on first use, with the receiver supplied per call rather than captured at load time.
- Remove the hard-coded standard-output logger defaults and the process-wide changes to the standard library logger.
- Ship an adapter from the event stream to Go's standard `log/slog`, in line with the project's preference for the standard library. It replaces the current example-only event logger.
- Build the pretty logger example on the `slog` adapter with charmbracelet/log as the `slog` handler, replacing the current example event logger. Keep it an example, not library API, so charm stays out of the library's dependencies.
- Backwards compatibility isn't required, because xcl hasn't been released. Existing signatures (registry constructor, discovery, logger package) can change freely.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- Direct log calls in xcl's own code, outside the logging wrapper itself: 0. Every diagnostic goes through the event stream.
- Output from xcl when no receiver is set: 0 bytes to standard output or standard error, across the full example suite.
- Every example in the repository runs its whole output through one receiver set up with a single line, and none of them builds or passes a logger for the plugin registry.
- Events lost in normal operation or when the buffer fills: 0.
- Example plugins that log with resource context: all of them do so without passing the context themselves.

<!--
  NON-GOALS
  Explicitly state what this spec does NOT cover. This is as important as
  the requirements — it prevents scope creep and sets clear expectations.
  Format: one bullet point per exclusion.
  Examples:
    - "Mobile support is out of scope (tracked in #456)"
    - "Internationalisation will be addressed in a follow-up spec"
  Leave blank if there are no explicit exclusions to call out.
-->
## Non-Goals

- **Declaring plugins in configuration files.** Registering plugins through configuration rather than code is the intended next step, and lazy loading prepares for it, but it's a follow-up spec.
- **Supported adapters for other loggers in the library.** The charmbracelet example shows how to use one, but isn't library API. Applications using other loggers write their own receiver.
- **More than one receiver per configuration.** An application that wants events in several places fans them out from its own receiver. xcl doesn't manage a list of subscribers.
- **Keeping value types across the plugin process boundary.** Details from plugins running as separate processes arrive with their names intact, but their values may arrive as text rather than the original types.
- **Tracing and metrics.** Integrations like OpenTelemetry spans or counters derived from events aren't part of this, although the stream is meant to make them possible later.
- **Storing or replaying events.** xcl delivers events live only. It keeps no history.
