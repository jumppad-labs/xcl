---
created_date: "2026-10-31"
document_status: draft
designs:
    - source: design
      path: typescript-sdk.md
    - source: design
      path: typescript-entity-types.md
    - source: design
      path: libxcl.md
epic: 20261031080131-8c2a58ec-typescript-sdk
sources:
    - uri: spec:20261011130709-aa3eb933-python-support
      retrieved_date: "2026-10-31"
    - uri: spec:20261011211634-a8895873-python-out-of-process-engine
      retrieved_date: "2026-10-31"
    - uri: spec:20261011232013-da18c586-python-release-hardening
      retrieved_date: "2026-10-31"
    - uri: spec:20261016064106-8931613a-python-encode-entity
      retrieved_date: "2026-10-31"
    - uri: https://github.com/jumppad-labs/xcl-python/blob/main/README.md
      retrieved_date: "2026-10-31"
---

# Feature: 20261031080136-264e0eab-typescript-core-sdk

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

TypeScript and JavaScript developers can't use xcl today: it runs only from Go and Python. This spec brings xcl to Node.js the way Python got it, by running the same bundled engine rather than rewriting it. A Node program then reads, checks and applies configuration exactly as a Go program would, uses the same Go plugins, and can add its own functions to configuration. Developers describe their configuration's types as TypeScript classes, much as they would describe ORM entities, and long-running work never stalls the rest of their program. It serves people building command-line tools, Jumppad-style tools and services in TypeScript. It is the first part of bringing xcl to TypeScript; printing, the pretty logger and the SDK documentation on xcl.dev follow separately.

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

- [ ] **Install from npm, no toolchain**
  Users can install xcl for TypeScript from npm on every supported platform without installing Go or any other toolchain, and without anything downloaded at install time or on first use.
- [ ] **A typed ES module**
  The package is imported as an ES module on every supported Node version and is fully typed for TypeScript.
- [ ] **Entity types defined in TypeScript**
  Developers can describe an entity type in TypeScript and register it with a type name and an optional subtype, which become the block's labels and address, as in Go.
- [ ] **Type definitions map to configuration**
  Optional fields, defaults, sensitive and computed fields, nested blocks, lists of blocks, named blocks collected by name, label fields, the block's metadata and singletons are all expressed in the type definition, and configuration is checked against it as it is for Go types.
- [ ] **TypeScript names, configuration names**
  Properties are camelCase in TypeScript and snake_case in configuration, converted by one rule in both directions.
- [ ] **TypeScript functions callable from configuration**
  Developers can register a TypeScript function, synchronous or asynchronous, and configuration can call it. The result is filled in exactly like a built-in function's. Registering a function under a name a built-in or another function already uses is refused.
- [ ] **Go plugins usable from TypeScript**
  A Node program can use Go-compiled plugins from a local path, a plugin directory or the GitHub registry, with release signatures verified as they are for Go programs.
- [ ] **Every operation available**
  A Node program can validate, diff, apply, load and destroy configuration, with the same results a Go program gets for the same configuration, plugins and state.
- [ ] **Variables passed in**
  A Node program can supply variable values when it creates a configuration.
- [ ] **Entities read back as typed values**
  A Node program can find entities by address, by type, exactly one of a type, all of a registered type, and every entity, and gets values of their registered type. Entities of types it hasn't registered, such as a Go plugin's types, are readable generically.
- [ ] **Outputs and diffs readable**
  A Node program can read a configuration's outputs, and a diff's changes, as JavaScript values.
- [ ] **Entities encoded as configuration**
  A Node program can turn a looked-up entity, a hand-built entity, a generic entity and a saved record into xcl configuration text, with the same four choices Go offers (include computed, include empty, reveal sensitive, show references), and gets the same text Go writes.
- [ ] **Encoding during an operation**
  Encoding a looked-up entity is refused while an operation is running on its configuration; a saved record can be encoded at any time, including from an event handler during an operation.
- [ ] **State location and custom stores**
  A Node program can keep state in a directory, or in its own store written in TypeScript whose methods may be asynchronous, and can have sensitive values encrypted in state.
- [ ] **Events delivered to TypeScript**
  A Node program can receive the engine's events through a handler, which may be asynchronous, with the same event data levels and masking a Go program has.
- [ ] **Errors as error classes**
  Failures are thrown as JavaScript errors, one class for each kind of error, each distinguishable by its class, carrying the message, the details and any configuration positions.
- [ ] **The event loop keeps running**
  The event loop keeps serving other work while an operation runs.
- [ ] **Cancellation stops the work**
  Aborting an operation through the platform's standard abort signal stops it in the engine, rather than leaving it running in the background, and keeps the state saved so far.
- [ ] **Ctrl-C cancels when asked**
  Users can opt in, per operation, to Ctrl-C cancelling that operation while keeping the state saved so far.
- [ ] **Ctrl-C without opting in**
  Without opting in, Ctrl-C ends the program as Node does by default, and the state of the operation in progress is still saved.
- [ ] **One engine per process**
  Every configuration in a process shares one engine, started on first use.
- [ ] **No engine left behind**
  No engine is left running after the last configuration is closed, after the process exits, or after the process is killed.
- [ ] **An idle engine never holds the process**
  A Node program that has finished its work exits, even if it never closed its configurations.
- [ ] **Engine failures are reported, not hidden**
  If the engine exits unexpectedly, every call waiting on it fails with an error carrying the exit status; the next use starts a fresh engine and the host process survives. The engine's own diagnostics reach the user's standard error.
- [ ] **Clear error when the engine is missing**
  If the engine package for the platform is not installed, the first use fails with an error naming the package expected for this platform and how to install it.
- [ ] **Engine version checked**
  The package refuses to run against an engine whose version doesn't match its own, with an error naming both versions.
- [ ] **Worker threads and cluster workers**
  Configurations work from worker threads and from cluster or child-process workers.
- [ ] **Releases kept in step**
  Every xcl release produces a TypeScript package of the same version, carrying the engine built from that release, without anyone publishing it by hand.
- [ ] **Parity breaks caught before release**
  A change to xcl, or to the package, that makes the package's results differ from Go's for the same configuration is caught before it is released.
- [ ] **Package documentation**
  The package ships a README covering install and getting started, a parity record against the Go API, and a doc comment on every public export.

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

- The work is built to three design documents in the project's `design` design source: `typescript-sdk.md`, which fixes the TypeScript API, the package layout and the platform packages; `typescript-entity-types.md`, which fixes how entity types are defined; and `libxcl.md`, which fixes the core service, the framing, the engine's process lifecycle and the release assets. The plan designs within them and raises any disagreement rather than designing around them.
- The package runs the bundled `xcl-core` engine as a subprocess and reimplements none of the engine's behaviour; it uses no native addon and no FFI.
- Entity types are defined as `typescript-entity-types.md` settles. Its proposal, decorated entity classes, is under review, and this spec is a draft until it is decided.
- The package supports Node 22 and later.
- The package is published to npm as `xcl-lang`, with npm provenance, and with one engine package per platform named `xcl-lang-<os>-<arch>[-<libc>]`.
- The eight supported platforms are Linux x86-64 and arm64 on glibc and on musl, macOS x86-64 and arm64, and Windows x86-64 and arm64. musl support is not negotiable.
- The code lives in the `jumppad-labs/xcl-typescript` repository.
- The API is asynchronous only; there is no synchronous variant.
- The package never listens for interrupts unless the program asks it to.
- The Go library's and the Python package's behaviour must not change; in the xcl repository only the release and CI workflows change.
- End-to-end and registry tests use the xcl-plugin-test plugin, not Docker.

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

- [ ] **Clean install on every platform**
  On a fresh machine for each of the eight platforms, with no Go or C compiler, `npm install xcl-lang` succeeds without network access beyond the npm registry, and, with the network then disconnected, a script that imports the package as an ES module and creates and closes a configuration runs without error on Node 22, 24 and 26. The README's getting-started example type-checks in a strict TypeScript project, and a lookup's result has its registered type.
- [ ] **Alpine install and apply**
  In a fresh `node:22-alpine` image on x86-64 and on arm64, installing the package and running a program that registers a type, applies a configuration, reads the entity back and closes succeeds.
- [ ] **Registered type decodes config**
  With a type registered as `app`, `app "web" { image = "nginx" }` validates and applies, and finding `app.web` returns a value of that type with `image === "nginx"`. A type registered as `database` with subtype `postgres` decodes `database "postgres" "main" { … }` and is found at `database.postgres.main`.
- [ ] **Type rules enforced**
  For a type with a required field, an optional field with a default, a sensitive field, a computed field, a single nested block, a list of `volume` blocks and named `listener` blocks:
  - leaving out the required field fails validation at the block's position;
  - leaving out the optional field gives the default;
  - the sensitive value is masked in events and state;
  - the computed field can't be set in config;
  - the nested block is decoded;
  - two `volume` blocks give an array of two items in order;
  - two `listener "…"` blocks give a record keyed by their names.
- [ ] **Labels, metadata and singletons**
  A type with a label field receives the block's label in it; a type with a metadata field receives the block's address in it; a type with neither is a singleton, written without labels and found by its type alone.
- [ ] **Name conversion**
  A property `connectionString` is written `connection_string` in configuration, and the value read back is on `connectionString`.
- [ ] **Function called**
  With `slugify` registered, `name = slugify("My App")` applies and the entity holds `"my-app"`; with an async function registered, its resolved value is filled in. Registering a name a builtin uses fails with the name-clash error.
- [ ] **Go plugin from path, directory and registry**
  A Node program applies config whose entities are handled by the test plugin, loaded from a local path, from a plugin directory and from the GitHub registry. A tampered release asset is refused with a signature error, and an asset signed by an untrusted key is refused naming the key.
- [ ] **Same results as Go**
  For the shared fixture configurations, the TypeScript and Go programs produce the same validation errors, the same diff, the same entities and outputs after apply, and the same state file contents. A load in a new process reads back the same entities in both, and a destroy removes every entity and empties the state in both.
- [ ] **Variables supplied**
  Passing `region: "us"` when creating the configuration makes an entity that uses `variable.region` hold `"us"`.
- [ ] **Lookups return typed entities**
  For a configuration with two `database "postgres"` blocks, one `database "mysql"` block and one `app` block: finding by type `database`, `postgres` returns the two postgres entities; finding exactly one of type `app` returns the app; finding all of the registered postgres type returns the two postgres entities; and finding every entity returns all four plus any variables and outputs, each of its registered type.
- [ ] **Unregistered types readable**
  Entities of a Go plugin's type, with no type registered, are returned with their address and attributes readable by name.
- [ ] **Outputs and diff**
  After apply, outputs are returned as a plain object of JavaScript values. A diff after changing one attribute reports that attribute's old and new values.
- [ ] **Encoding matches Go**
  For each entity the parity fixtures cover (looked-up, hand-built, generic and saved record) and each combination of the four encoding choices, the TypeScript text is byte-for-byte equal to Go's encoder output. Encoding a non-entity, a variable, an output, a module, an unencodable value or a value of an unregistered type throws the matching error; encoding a looked-up entity while an operation runs throws the operation-in-progress error, and encoding a saved record from an event handler during the operation succeeds.
- [ ] **State directory and custom store**
  With a state directory set, an apply writes state there and a load in a new process reads it back. An object implementing the store methods, with async methods, receives the state on apply, and a later load in a new process reads it back. With state encryption enabled, sensitive values in what the store receives are encrypted.
- [ ] **Events received**
  An event handler receives an event for each entity's start and finish during an apply; with event data on, sensitive values in event data are redacted, and the event's entity can be read as a typed value. An async handler is awaited.
- [ ] **Typed errors**
  A missing address throws the not-found error, asking for exactly one entity of a type when several exist throws the not-unique error, and invalid config throws the config error, which lists each problem with its file and line. Each is an `instanceof` its own class and of the base xcl error class.
- [ ] **Event loop not blocked**
  While an apply runs a slow plugin operation, a timer on the same event loop keeps firing with a delay under 50 ms, measured by Node's event-loop delay monitor.
- [ ] **Cancellation**
  Aborting an apply's `AbortSignal` mid-way makes it reject with the cancellation error within two seconds of the abort, the entities completed before it are in state, and the test plugin records no operation started after the abort.
- [ ] **Ctrl-C with the interrupt signal**
  Sending SIGINT to a program mid-apply that passed the package's interrupt signal makes the apply reject with the cancellation error, the entities completed before it are in state, the program's own code after the rejection runs, and once the apply has ended the program has no SIGINT listener left from the package.
- [ ] **Ctrl-C without it**
  Importing the package and running an apply adds no SIGINT listener to the process. Sending SIGINT to a program mid-apply that did not pass the interrupt signal ends the program as Node does by default, the entities completed before it are in the state directory, and no engine process is left running.
- [ ] **One engine per process, cleaned up**
  Creating three configurations in one process starts one engine process; closing the last one leaves no engine process running; a script that never calls `close()` still exits when its work is done, leaving no engine running; killing the host process leaves no engine process running within five seconds. Each of these holds on all eight platforms.
- [ ] **Engine death is surfaced**
  Killing the engine process while an apply is in progress makes the apply reject with the engine-exited error carrying the signal or exit status; the host process continues, and a new configuration created afterwards works.
- [ ] **Engine diagnostics shown**
  Output the engine writes to its standard error appears on the host program's standard error.
- [ ] **Missing engine named**
  With the platform engine package removed, the first use throws an error naming that package and how to install it.
- [ ] **Version mismatch refused**
  Running the package against an engine of a different version throws an error naming both versions.
- [ ] **Workers**
  A configuration created inside a worker thread, one created inside a cluster worker and one created inside a child-process worker each apply successfully.
- [ ] **Release publishes matching package**
  An xcl release tag leads, with no manual publishing step, to `xcl-lang` and its eight platform packages being published to npm at the same version with provenance, and that version's engine reports the tag.
- [ ] **Parity breaks caught**
  A change to xcl runs the TypeScript package's tests, including the Go parity fixtures, against the engine built from that change, and a change that makes the TypeScript results differ from Go's fails that run.
- [ ] **Docs present**
  The npm page shows the README with install and a getting started that applies a configuration; the parity record lists every Go operation and lookup with its TypeScript equivalent; every public export has a doc comment.

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

- Follow the three layers `libxcl.md` gives every SDK: a byte transport that starts the engine and frames calls, a typed client over messages generated from xcl's protos at the bundled tag, and a hand-written public API. The SDK's tests can then run against a fake transport with no engine.
- Read engine frames on the event loop, so nothing blocks it and polling needs no special handling; let go of the engine's hold on the event loop whenever no call is outstanding, so an idle engine never keeps the process alive.
- Carry functions, events and state-store calls through the poll loop as tasks the host completes, as Python does; run tasks concurrently on the event loop.
- Offer Ctrl-C cancellation as a ready-made abort signal from the package, passed as the operation's signal in one argument, whose interrupt listener lasts only as long as that operation.
- Derive each registered type's schema v1 at registration, so a definition the engine would refuse fails there rather than on the first operation.
- Keep the package's runtime dependencies to the protobuf runtime, as the Python package keeps to protobuf.
- Mirror xcl-python's release flow: xcl's release dispatches an engine bump to xcl-typescript, a merged bump tags and publishes, and publishing uses npm trusted publishing. The bindings CI in xcl gains a job running the TypeScript suite against the engine built from each change.
- Reuse the parity approach and fixtures xcl-python uses to compare with Go, rather than inventing a second one.
- Risks to watch: the engine outliving a host on Windows, where Node has no job object without a native addon, so the engine's exit on end of input is the safeguard; the optional-dependency install being skipped by some package managers or flags; and keeping the schema derived from a TypeScript type identical to what the same Go type gives.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- A TypeScript developer goes from installing the package to a working apply against a Go plugin by following the package README alone.
- Every operation and lookup in the Go API has a TypeScript equivalent, checked against the parity record at each release.
- The Go parity fixtures show zero differences on every release, on all eight platforms.
- A web service using the package keeps serving other requests during a long apply, with event-loop delay staying under 50 ms at the 99th percentile.
- The first configuration in a process is created within one second on the CI runners.
- No open issue reports an orphaned engine process after a Node program exits.

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

- Printing, highlighting and the pretty logger for TypeScript: the next spec in this epic.
- The TypeScript SDK reference and guide on xcl.dev, and TypeScript tabs on the site: a later spec in this epic.
- Providers written in TypeScript, TypeScript plugins and a plugin SDK.
- Generating TypeScript types for a plugin's entity types.
- Support guarantees for Bun and Deno.
- A separate CommonJS build.
