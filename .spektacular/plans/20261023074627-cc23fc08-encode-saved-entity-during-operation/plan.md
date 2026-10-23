---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Plan: 20261023074627-cc23fc08-encode-saved-entity-during-operation

<!-- Metadata -->
<!-- Created: 2026-10-23T08:22:18Z -->
<!-- Commit: ae3b7b6 -->
<!-- Branch: f-python-prettylogger -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Programs written in languages other than Go, starting with Python, cannot render a resource's saved record as configuration text while an operation runs, because xcl's engine refuses every encode call until the operation ends. This plan lets the engine render saved records at any time, including from an event handler during an apply, validate, diff, destroy or load, while still refusing renders of looked-up or edited entities that read state the operation is changing. Python programs get it by bundling the xcl release that contains it, with no package code change, so tools such as the planned pretty logger can show each resource as it is created rather than all at the end. The Python docs, xcl.dev and the other-language design all say when rendering is allowed.

## Conventions

- **Testing & Mocking (testify `require`, never table-driven, never mix positive and negative cases in one test, verbose over abstract, tests live beside the code they test, no tests that read repository files)** — every new core-service and root-package Go test follows it: each operation kind gets its own render test, each refusal and each error case its own function, and the docs, changelog and design updates are checked by human review only. The xcl-python integration tests follow the same rules.
- **Shared test helpers live in `internal/testutil`** — the hold-at-first-state-task helper is used only by `coreservice` tests, so it stays in that package's `_test.go` files and is not promoted to `internal/testutil`.
- **Generate test state with a real apply, not a hand-written state file** — the saved record rendered during Validate, Diff, Destroy and Load comes from an earlier apply's SAVE through the host `fakeStore` (or the state file it wrote), never from a hand-written state document. The invalid-JSON and unknown-type records are deliberately hand-built, because those tests are about bad input.
- **Go Code Style & Standards (gofmt, go vet, descriptive names, explicit error handling, stdlib style)** — applies to the small `coreservice/lookups.go` refactor (splitting the record-presence check out of `encodeAllowed`) and to the new tests.
- **Shared error types live in the `errors` package** — deliberately not triggered: no new error type or sentinel is added. The existing `operation_in_progress`, `invalid_saved_data` and `unregistered_type` codes are reused unchanged.

## Architecture & Design Decisions

**Shape.** The engine side is a one-call change in the xcl repo's core service. Today `encodeSavedEntity` and `encodeEntity` share `encodeAllowed`, which runs the per-configuration lookup guard `lookupAllowed` and then checks that a record was sent (`coreservice/lookups.go:121-169`). The guard's stated reason is that "the entity slice is replaced when an operation ends" (`coreservice/operations.go:82-90`). The plan splits the record-presence check out of `encodeAllowed`. `encodeSavedEntity` then runs only that check. `encodeEntity` keeps the guard followed by the same check, so it refuses exactly as before. A comment at `encodeSavedEntity` records why it needs no guard: Go's `Config.EncodeSavedEntity` never reads the live entities. It types the record through the read-locked plugin catalog and renders it through the shared encoder (`encode.go:154-185`). `EncodeEntityRecord` reads the live entity by `meta.id` (`encode_record.go:66-87`), so it stays refused. Go's library, its encode API and the protocol messages are unchanged. Only the protocol's comment on `EncodeSavedEntity` is added, saying it is allowed during an operation, and the Go stubs are regenerated so the comment carries into them. The Python package needs no code change, because both of its clients already expose `encode_saved_entity` (xcl-python `xcl/config.py:259-288`, `xcl/aio.py:279-296`). It gains tests, and its docs change, in xcl-python and on xcl-website. The engine design `design:libxcl.md` is revised through the design CLI.

**Key decisions and trade-offs.** (1) *The guard goes at the call site, not in a policy table.* Every lookup handler names its guard explicitly. Leaving it off one handler, with a comment that applies the guard's own test ("does this call read the live entities?"), is the smallest change and the easiest to review. A dispatch-table flag would add machinery for one call. (2) *No new locking.* Events cross the service with no completion (`coreservice/events.go:10-17`), so the operation never waits on a host that is rendering. The catalog counts its users under `usersMu`, so a render during an operation moves the count from N to N+1 and back, and never stops the external plugins the operation is using (`internal/catalog/catalog.go:710-758`). A render made while no operation holds the catalog, at the very start or end of an operation, briefly starts and stops the external plugins, exactly as a render outside an operation does today. That cost is accepted. A render made while the first operation is loading plugins waits for the load to finish, and cannot deadlock, because loading never waits on the host. (3) *The spec's risk is verified with a deterministic Go test.* An apply against the external fixture plugin, with an event buffer of one, renders from its handler. With a buffer of one, the work cannot finish while the handler is busy. The test asserts that the external plugin is still running after the render returns, and that the apply completes. (4) *Errors keep their identity.* With the guard gone, an empty record during an operation reports `invalid_argument`, invalid JSON reports `invalid_saved_data`, and an unknown type reports `unregistered_type`. These are the same codes the calls give outside an operation, all produced by unchanged code.

**Testing shape.** Core-service tests in package `coreservice` hold each of Validate, Apply, Diff, Destroy and Load at its first host state-store task. All five read the store's `Exists` before they end, and a state task waits for `Complete`. While the operation waits there, the tests render a saved record and assert text. Apply-time tests render from the create success event inside `drive`'s `onEvent`. That event is posted before the SAVE task that follows it, so the operation is provably still running. Python integration tests in xcl-python do the same through a Python `state_store`, from the synchronous and asyncio clients. Each test also shows that a looked-up or edited entity is still refused at that moment, which proves the render really happened during the operation. Tests follow the project's testing conventions: testify `require`, no table-driven tests, positive and negative cases in separate functions, and no test that reads docs or the changelog.

**Why this beats the alternatives.** Buffering in Python breaks the spec's "no buffering" success metric and its "no waiting in the handler" constraint. Changing Go's encoder is a Non-Goal and unnecessary, since Go already documents the call as safe during an operation. Relaxing the guard for `EncodeEntity` or for all lookups would expose reads of an entity slice that is about to be replaced, which the spec forbids. See `research.md#alternatives-considered-and-rejected` for the evidence behind each rejection.

**Delivery order across repos.** xcl lands first and is released through its PR's release label. That triggers the automated engine-bump PR in xcl-python. xcl-python's tests and docs are developed against an engine built from the xcl branch and merged once the pin names the release. The website changes go out after the Python release they describe.

## Component Breakdown

- **Core service encode handlers (xcl, changed).** These own the decision of which encode calls may run while an operation is in progress on a configuration. The saved-record call stops consulting the configuration's lookup guard and keeps its own check that a record was sent. The entity-record call still consults the guard before that same check. The lookup guard itself and the operation tracking that backs it are unchanged, and so is every other lookup. The handlers call Go's `Config` encoders exactly as before.
- **Go `Config` saved-record encoder and plugin catalog (xcl, unchanged, verified).** These type the record through the catalog and render it through the shared encoder. The catalog counts its users, so a render during an operation never stops the external plugins the operation uses. No code changes. A new root-package test pins down that behaviour under a concurrent apply, which is the spec's flagged risk.
- **Core service protocol (xcl, comment only).** The protocol definition gains a comment on the saved-record call saying it is allowed while an operation runs, matching the existing comment on the entity-record call. The generated Go code is regenerated so it carries the comment. No message, field or method changes, so the Python generated code needs no update.
- **Core service tests (xcl, changed).** These own proof of the new rule at the service boundary. They render during each of the five operations, held at a host state-store task. They check that the text is the same during and after an apply, that an apply's result, events and saved state are unchanged by rendering, that looked-up and edited records are still refused, and that bad records keep their specific errors. The existing test asserting the saved-record refusal is replaced.
- **xcl changelog and engine design (xcl, docs).** The changelog gets this spec's section. The other-language design, `design:libxcl.md`, has its Threading rule, error table row and encode paragraph revised through the design CLI, so it no longer says every render is refused.
- **Python clients (xcl-python, unchanged code; docs and tests changed).** The synchronous and asyncio `encode_saved_entity` already reach the engine call. Their docstrings, and the docstring of the entity encoder, now say when each may run. The README's encoding section is updated to match. New integration tests drive the real engine from both clients' event handlers, with a Python state store so the operation is provably still running. The package ships the capability by bundling the engine release that contains it, through the existing automated engine pin bump.
- **xcl.dev pages (xcl-website, docs).** The configuration-text page's saved-data section, and the event-entity example that encodes a looked-up entity, state that a saved record renders at any time, including from an event handler during an operation, and that a looked-up entity does not. The events page's sentence about encoding an event's entity is aligned.

## Data Structures & Interfaces

No new types, messages, fields or public functions are introduced. The plan changes when one existing call is accepted, not its shape.

- **`EncodeSavedEntity` (core service protocol, unchanged shape).** The request is still `{ entity bytes, highlight bool, options EncodeOptions }` and the response is still `{ text bytes }`. The contract change is that the call is now accepted while an operation is in progress on the handle. Only a comment is added to the protocol definition.

  ```proto
  // EncodeSavedEntity is Config.EncodeSavedEntity: it renders a saved record,
  // as state stores it and events carry it, as configuration text. It reads
  // none of the configuration's entities, so unlike the other lookups it is
  // allowed while an operation runs, including from an event handler.
  rpc EncodeSavedEntity(EncodeSavedEntityRequest) returns (EncodeSavedEntityResponse);
  ```

- **`EncodeEntity` and every other lookup (unchanged).** These are still refused with `operation_in_progress` ("<call> cannot run while an operation is in progress") while an operation runs.
- **Error contract of `EncodeSavedEntity` (unchanged codes, now reachable during an operation).** An empty record gives `invalid_argument` with `argument=entity`. Data that is not one saved record gives `invalid_saved_data`. A type the configuration does not know gives `unregistered_type` with `type=<name>`. A builtin gives `not_encodable`, and a plugin load failure gives `plugin_load`. The code for a given record is the same whether or not an operation is running.
- **Core service internal helper (package-private).** The shared `encodeAllowed(conf, what, entity)` is split. A record-presence check (`entity` non-empty, else `invalid_argument`) is used by both encode handlers, and the existing `conf.lookupAllowed(what)` stays in front of it in `encodeEntity` only. Neither is exported.
- **Python API (unchanged signatures).** `xcl.Config.encode_saved_entity(data, *, include_computed, include_empty, reveal_sensitive, show_references, highlight) -> str` and its `await`able `xcl.aio.Config` twin keep their signatures. Their documented contract widens to "callable at any time, including from an `on_event` handler during an operation". `encode_entity` keeps raising `xcl.OperationInProgressError` during an operation.

## Implementation Detail

**No new pattern; one existing pattern made explicit.** Every core-service lookup handler opens by asking the configuration whether a lookup is allowed. After this change, the saved-record handler is the one lookup that deliberately does not ask. The code says why at the call site, using the guard's own reason as the test: the call reads none of the configuration's entities, so a running operation that will replace them cannot affect it. Someone adding the next call has a worked example of how to decide whether it needs the guard. The two encode handlers keep sharing their record-presence check, so they still reject an empty record identically, now with the same `invalid_argument` during an operation as outside one.

**Code shape.** The shared "may this encode run?" helper becomes a smaller "was a record sent?" helper. The entity-record handler calls the guard and then that helper, and the saved-record handler calls only the helper. Nothing else in the service changes shape: operation tracking, the task queue, event posting and the other lookups are untouched. The Go library's encoders and plugin catalog are not edited, and their existing godoc already describes calls during an operation as safe.

**Tests follow the service's existing style.** New service tests use the in-package test client, the host fake store and the poll-driven `drive` loop already in use, written one behaviour per function. A small package-local helper starts an operation against a host store and waits for its first state task, so a test can render while the operation is provably in progress, then answer the task and drive to the end. That helper is the one new piece of test scaffolding. It stays in the core service's test files, because no other package needs it. The root package's plugin-lifecycle tests gain one test in their existing style, using the external fixture plugin and a one-slot event buffer, for the concurrent-catalog risk.

**Python and docs follow existing shapes.** The Python package keeps its two clients exactly as they are. New integration tests sit beside the existing encode and refusal tests in the sync and asyncio suites, reuse the slow fixture plugin, and add a small in-memory Python state store. The store guarantees each render happens while the engine is still waiting on the host. The existing "encode_entity refused during an operation" tests stay. Docs keep their current structure: one sentence or short paragraph changes in each place that today says or implies that every encode is refused. The design document is revised through the design CLI, and the changelog gets this spec's section in its usual form.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references. The other-language design `libxcl.md`, from the `design` design source, is read here as current documentation and is **revised** by this plan, as the spec's documentation requirement demands. It is not a design this plan builds on.
- **xcl `Config.EncodeSavedEntity` and the plugin catalog's use counting** — provide the render and the guarantee that a render during an operation never stops plugins the operation uses. No change needed; the plan only adds a test that verifies the guarantee.
- **xcl core service (operation tracking, task queue, event posting)** — provides the fire-and-forget event delivery and host state-store tasks the tests rely on. No change apart from the encode handler guard.
- **buf (protocol code generation)** — regenerates the Go stubs after the comment-only proto change. This is an existing tool and target; nothing new.
- **xcl release automation (PR release label, auto-tag, release workflow)** — publishes the xcl release containing the change. **Must land before** xcl-python can ship it, because the package only bundles an engine built from a released xcl tag.
- **xcl-python engine bump workflow** — on the xcl release it opens and auto-merges the pin bump, and the package's tag-and-release workflow then publishes a package bundling the new engine. No change needed. Its success is a manual check.
- **xcl-python fixture plugins (`slow`, built from xcl's `bindings/libxcl` testdata)** — used by the new Python integration tests. Unchanged.
- **Sibling spec `20261023075136-42c51a7d-printing-tokens-over-the-engine` (same epic)** — also edits the core service's encode handlers (labelled text, highlight handling). There is no ordering dependency, but whichever lands second rebases onto the other's handler changes. This plan touches only the guard and the record-presence helper.
- **Sibling spec `20261023074223-e06f835f-event-data-level-enum` (same epic)** — changes how a Python program asks for processed event data. There is no ordering dependency. The Python tests pass the event data level in whatever form the package accepts at the time they are written.
- **Downstream: `20261023075328-741138ee-python-prettylog` (same epic)** — depends on this plan. It is the first consumer and needs no further engine change.
- **Prior plan `20261016064106-8931613a-python-encode-entity`** — introduced `EncodeEntity`, encode options and both refusals. This plan narrows one of those refusals. Nothing from it needs to change first.

## Testing Approach

**Kinds of tests.**
- *Service-boundary integration tests in xcl's core service package.* These carry most of the coverage, because the rule being changed lives there. They drive real operations through the in-package test client, as a host would, with the host fake state store.
- *One root-package concurrency test.* It pins down the spec's flagged catalog risk with the external fixture plugin.
- *End-to-end integration tests in xcl-python.* These run against the real engine built from the xcl branch, from both the synchronous and the asyncio clients, and prove the acceptance criteria as a Python user experiences them.

There are no new unit tests, since there is no new business logic to isolate. The existing core-service test asserting the saved-record refusal is replaced. The existing tests asserting the entity-record refusal stay, in Go and in both Python suites.

**Load-bearing assertions (each in its own test function, positives and negatives separate).**
- *Renders during every operation.* A saved record renders to text, not `operation_in_progress`, while each of Validate, Apply, Diff, Destroy and Load is held at its first host state-store task. Each operation then finishes without error. These are five separate tests.
- *Same text during and after.* For one saved record, the text rendered from the create success handler during an apply is byte-identical to the text rendered after the apply, both with computed values included and with default options.
- *Operation unaffected.* Two applies of the same configuration are compared: one whose handler renders every create success record, and one whose handler renders nothing. They have the same finished result, the same sequence of event operation/phase pairs and the same saved records.
- *Still refused.* From the same handler, during the apply, rendering the event's entity as a looked-up record fails with `operation_in_progress`. So does rendering an edited record. The apply still completes.
- *Errors stay specific.* During an operation, invalid JSON gives `invalid_saved_data`, a well-formed record of an unknown type gives the same `unregistered_type` it gives after the operation, and an empty record gives `invalid_argument`, never `operation_in_progress`.
- *Catalog under a concurrent apply.* With the external fixture plugin and a one-slot event buffer, a render from the handler returns text, and the plugin process is still running when the render returns. The apply completes, and the plugin is stopped afterwards, as today.
- *Python, both clients.* With `processed` event data and a Python state store, a handler renders the create success record with computed values included. The render returns text, the apply succeeds, and the text equals the post-apply render of the same record. A handler using `encode_entity` on the event's entity, or on an edited entity, gets `xcl.OperationInProgressError`. Invalid and unrecognised records get `InvalidSavedDataError` and the same error class the post-apply render gives. An apply whose handler renders every record has the same events and saved state as one whose handler renders nothing.

Rendering during the operation is made provable rather than lucky. The host answers the state task that follows the event only after the handler returns, so the engine is guaranteed to still be running when the render happens. The refusal assertions made at the same moment confirm it.

**Conventions.** The tests follow the project's testing rules: testify `require`, no table-driven tests, positive and negative cases in separate functions, verbose over abstract, tests beside the code they test, saved state produced by a real apply, and the race detector on for Go tests. No test reads the README, PARITY.md, the changelog, the design document or website pages.

**Success metric mapping.**
- *"A Python program can print each created resource's configuration beneath its success event during an apply, with no buffering until the apply ends."* — **Behavioural test.** This is the synchronous Python integration test. Its handler renders the create success record while the engine is provably still running, and receives the text in the handler, not after the apply. The asyncio twin covers the other client.

**Manual checks — each captured in the implementation test plan.**
- **Manual — captured in the implementation test plan.** *Installed package has it:* after the xcl release and the automated engine bump, install the released package from PyPI into a fresh environment with no local engine build. Run the synchronous render-from-handler program, and confirm that `xcl.engine_version()` reports the released xcl version containing the change.
- **Manual — captured in the implementation test plan.** Review the xcl changelog entry for this spec.
- **Manual — captured in the implementation test plan.** Review the revised `libxcl.md` design: its Threading rule, error table and encode paragraph no longer say every render is refused, and they say the saved-record render is allowed during an operation and the entity-record render is not.
- **Manual — captured in the implementation test plan.** Review the Python docstrings and README encoding section: saved records render at any time, including from an event handler, and looked-up entities are refused during an operation.
- **Manual — captured in the implementation test plan.** Review the xcl.dev configuration-text page and the events page sentence for the same two statements, and check that the site builds.

**Deliberate gaps.**
- No test exercises a render issued while the first operation is still loading plugins. It waits for the load and cannot deadlock, because loading never waits on the host. Constructing it would need a blocking registry, which adds test complexity for an unreachable failure.
- No Python test repeats the five-operation matrix. That rule is enforced in the engine and is fully covered at the service boundary.

## Milestones & Tasks

### Milestone 1: The engine renders saved records while an operation runs

**What changes**: A program talking to xcl's engine from another language can turn a resource's saved record into configuration text while an apply, validate, diff, destroy or load is still running, including from its event handler. The text is exactly what it gets after the operation. The operation itself runs exactly as before, with the same result, events and saved state. Rendering a looked-up or edited entity is still refused during an operation, with the same error as today, and a bad record reports the same specific error during an operation as outside one. Go programs see no change, since they could already do this. The protocol, xcl's changelog and the other-language design say when each render is allowed.

**Validation point**: xcl's core-service and root-package test suites pass with the race detector on. That includes the five per-operation render tests, the during/after text equality test, the unchanged-apply comparison, both refusal tests, the three error tests and the external-plugin concurrency test. The design and changelog read correctly.

#### - [ ] Task: Allow the saved-record render while an operation runs
**Id:** 64338e68-32e3-4675-95cf-c4c7b34bb30f
**Repo:** xcl
**Depends on:** none
**Execution:** agent

The engine's saved-record render stops checking whether an operation is running, because it reads none of the configuration's entities. The entity-record render and every other lookup keep the check. The protocol gains a comment saying the saved-record call is allowed during an operation. The existing test that expected a refusal is replaced by tests showing the render returns text while each of the five operations is running, and that bad or empty records keep their specific errors.

*Technical detail:* [context.md#task-allow-the-saved-record-render-while-an-operation-runs](./context.md#task-allow-the-saved-record-render-while-an-operation-runs)

**Acceptance criteria**:
- [ ] A saved record renders to configuration text while each of apply, validate, diff, destroy and load is running, and each operation still finishes without error.
- [ ] Rendering an entity record (a looked-up or edited entity) while an operation runs is still refused with the operation-in-progress error, as are the other lookups.
- [ ] During an operation, a record that is not valid JSON fails with the invalid saved data error, a record of an unknown type fails with the same unregistered-type error it gives after the operation, and an empty request fails as an invalid argument. None of them fails with the operation-in-progress error.
- [ ] The protocol definition documents that the saved-record call is allowed during an operation, and the generated Go code carries that comment.

#### - [ ] Task: Prove rendering from an event handler leaves the apply unchanged
**Id:** 61e00152-2c93-4d99-918c-d9ceec78662c
**Repo:** xcl
**Depends on:**
- 64338e68-32e3-4675-95cf-c4c7b34bb30f — Allow the saved-record render while an operation runs
**Execution:** agent

Adds core-service tests that render from a host's handling of the create success event during an apply, at a moment when the engine is provably still running. They show that the text equals the text rendered after the apply, that an apply which renders every created resource has the same result, events and saved state as one that renders nothing, and that looked-up and edited records are still refused from that same handler.

*Technical detail:* [context.md#task-prove-rendering-from-an-event-handler-leaves-the-apply-unchanged](./context.md#task-prove-rendering-from-an-event-handler-leaves-the-apply-unchanged)

**Acceptance criteria**:
- [ ] A record rendered from the create success event during an apply, with computed values included, is identical to the same render after the apply. The same holds with default options.
- [ ] An apply whose event handling renders every create success record ends with the same result, the same sequence of event operations and phases, and the same saved state as an apply that renders nothing.
- [ ] From the same event handling, rendering the event's entity as a looked-up record fails with the operation-in-progress error, and the apply still completes.
- [ ] From the same event handling, rendering an edited entity record fails with the operation-in-progress error.

#### - [ ] Task: Verify plugins keep running when a handler renders during an apply
**Id:** 705b4073-6a29-4ba3-a6a2-122d91d664ed
**Repo:** xcl
**Depends on:** none
**Execution:** agent

The spec flags a risk: the saved-record render briefly uses the plugin catalog, which could stop external plugin processes an apply still needs. This task adds a deterministic Go test against the external fixture plugin, showing that a render from an event handler during an apply leaves the plugin running and the apply succeeds. The plugin still stops once the apply is done, as today. No library code changes are expected. If the test fails, the implementer stops and reports.

*Technical detail:* [context.md#task-verify-plugins-keep-running-when-a-handler-renders-during-an-apply](./context.md#task-verify-plugins-keep-running-when-a-handler-renders-during-an-apply)

**Acceptance criteria**:
- [ ] A saved record rendered from an event handler during an apply that uses an external plugin returns text, and the external plugin process is still running when the render returns.
- [ ] That apply completes successfully, and the external plugin process is stopped after it, exactly as without the render.

#### - [ ] Task: Describe the change in the xcl changelog and the other-language design
**Id:** c045b2c9-b98e-49a5-a062-9240033f243d
**Repo:** xcl
**Depends on:**
- 64338e68-32e3-4675-95cf-c4c7b34bb30f — Allow the saved-record render while an operation runs
**Execution:** agent

Adds this spec's section to xcl's changelog: saved records can now be rendered through the engine while an operation runs, entity-record rendering and other lookups are still refused, and nothing breaks. It also revises the other-language design document through the design CLI, so its threading rule, error table and encoding paragraph no longer say every render is refused during an operation.

*Technical detail:* [context.md#task-describe-the-change-in-the-xcl-changelog-and-the-other-language-design](./context.md#task-describe-the-change-in-the-xcl-changelog-and-the-other-language-design)

**Acceptance criteria**:
- [ ] The changelog has a section for this spec describing the newly allowed render, what stays refused and why, and a Breaking list saying none.
- [ ] The other-language design says the saved-record render is allowed while an operation runs, including from an event handler, and that the entity-record render and the other lookups are refused. No sentence in it still says every render is refused during an operation.

### Milestone 2: Python programs show each resource as it is created

**What changes**: A Python program using either the synchronous or the asyncio client can render each created resource's saved record from its event handler while the apply is still running, and print it beneath the success event without waiting for the apply to finish. The package's docstrings and README say that saved records render at any time, including from an event handler, while looked-up entities are refused during an operation. The released package ships this by bundling the xcl release that contains Milestone 1, so users need no local engine build.

**Validation point**: xcl-python's sync and asyncio integration suites pass, first against an engine built from the Milestone 1 xcl branch and then against the released xcl tag the package pins. These cover rendering from the create success handler, during/after equality, the unchanged apply, both refusals and both error cases. The installed-package check in the implementation test plan passes after the package release.

#### - [ ] Task: Test rendering from Python event handlers during an apply
**Id:** aa6a42dc-b5bc-4f41-83d5-693dd89e37d3
**Repo:** xcl-python
**Depends on:**
- 64338e68-32e3-4675-95cf-c4c7b34bb30f — Allow the saved-record render while an operation runs
**Execution:** agent

Adds integration tests to the synchronous and asyncio suites that apply a configuration creating one resource, with a Python state store, and render the create success record from the event handler while the engine is still running. They check that the text matches the render after the apply, that the apply is unchanged by rendering, that looked-up and edited entities are still refused, and that bad records raise their specific errors. The tests run against an engine built from the Milestone 1 xcl branch until xcl is released.

*Technical detail:* [context.md#task-test-rendering-from-python-event-handlers-during-an-apply](./context.md#task-test-rendering-from-python-event-handlers-during-an-apply)

**Acceptance criteria**:
- [ ] Using the synchronous client, an event handler renders the create success record with computed values included. The render returns text, the apply succeeds, and the text equals the same render after the apply.
- [ ] Using the asyncio client, an awaiting event handler gets the same result, and the apply completes.
- [ ] An apply whose handler renders every create success record produces the same events and saved state as one whose handler renders nothing.
- [ ] From the handler, encoding the event's entity, or an entity the handler edited, raises the operation-in-progress error, and the apply still completes.
- [ ] From the handler, a record that is not valid JSON raises the invalid saved data error, and a record of an unknown type raises the same error class the render gives after the apply.

#### - [ ] Task: Document when Python programs can render configuration text
**Id:** 27c95072-5647-41dc-8ca8-d9808f0dba5a
**Repo:** xcl-python
**Depends on:**
- 64338e68-32e3-4675-95cf-c4c7b34bb30f — Allow the saved-record render while an operation runs
**Execution:** agent

Updates the package's docstrings for both encoders on both clients, and the README's encoding section. They now say that a saved record can be rendered at any time, including from an event handler while an operation runs, and that encoding a looked-up or edited entity raises the operation-in-progress error during an operation. A short example shows printing a created resource beneath its success event.

*Technical detail:* [context.md#task-document-when-python-programs-can-render-configuration-text](./context.md#task-document-when-python-programs-can-render-configuration-text)

**Acceptance criteria**:
- [ ] The saved-record encoder's documentation, on both clients, says it works at any time, including from an event handler during an operation.
- [ ] The entity encoder's documentation still says it is refused while an operation runs, and points readers at the saved-record encoder for rendering from an event handler.
- [ ] The README's encoding section states both rules, with an example that renders each created resource from the event handler.

#### - [ ] Task: Release xcl with the saved-record render allowed during operations
**Id:** 486a7a20-71de-43d4-bf09-dc56033240fd
**Repo:** xcl
**Depends on:**
- 64338e68-32e3-4675-95cf-c4c7b34bb30f — Allow the saved-record render while an operation runs
- 61e00152-2c93-4d99-918c-d9ceec78662c — Prove rendering from an event handler leaves the apply unchanged
- 705b4073-6a29-4ba3-a6a2-122d91d664ed — Verify plugins keep running when a handler renders during an apply
- c045b2c9-b98e-49a5-a062-9240033f243d — Describe the change in the xcl changelog and the other-language design
- aa6a42dc-b5bc-4f41-83d5-693dd89e37d3 — Test rendering from Python event handlers during an apply
**Execution:** human — merging the xcl pull request with its release label publishes a release, an action outside the repo

Once the Python tests have proven the xcl branch, a maintainer merges the xcl pull request with a release-size label. Auto-tag then cuts the release, and the release workflow dispatches the engine pin bump to xcl-python, which merges and publishes a package bundling the new engine.

This xcl release must be cut before `20261023075136-42c51a7d-printing-tokens-over-the-engine` merges to xcl main (decided during epic planning). That change reserves the protocol's `highlight` field, and xcl-python's CI regenerates its stubs from the pinned xcl, so an automatic engine bump to a release containing it fails until the Python printing work lands with it. If the ordering is missed, do not force the automatic bump: this plan's Python tests and docs ship with the release of `20261023075302-bad71237-printing-and-highlighting-python` instead.

*Technical detail:* [context.md#task-release-xcl-with-the-saved-record-render-allowed-during-operations](./context.md#task-release-xcl-with-the-saved-record-render-allowed-during-operations)

**Acceptance criteria**:
- [ ] An xcl release tag exists that contains the saved-record render change.
- [ ] That release tag was cut before the printing-tokens-over-the-engine change merged to xcl main, or the Python work is recorded as shipping with the Python printing release.
- [ ] xcl-python's engine pin bump for that tag has merged.

#### - [ ] Task: Run the Python suites against the released engine
**Id:** 58b80ef0-b046-4937-ad8e-8e0be6d5742a
**Repo:** xcl-python
**Depends on:**
- 486a7a20-71de-43d4-bf09-dc56033240fd — Release xcl with the saved-record render allowed during operations
- aa6a42dc-b5bc-4f41-83d5-693dd89e37d3 — Test rendering from Python event handlers during an apply
- 27c95072-5647-41dc-8ca8-d9808f0dba5a — Document when Python programs can render configuration text
**Execution:** agent

Brings the xcl-python branch onto the main line, where the engine pin now names the released xcl tag. It drops any development pin that pointed at the xcl branch, and runs the new tests against an engine built from the released tag, so the package's main line only ever builds against a released engine.

*Technical detail:* [context.md#task-run-the-python-suites-against-the-released-engine](./context.md#task-run-the-python-suites-against-the-released-engine)

**Acceptance criteria**:
- [ ] The xcl-python branch's engine pin names the released xcl tag that contains the change, not a development branch.
- [ ] The synchronous and asyncio rendering tests pass against an engine built from that tag.

### Milestone 3: xcl.dev says when rendering is allowed

**What changes**: The xcl website's configuration-text page states that a saved record can be rendered at any time, including from an event handler while an operation runs, and that rendering a looked-up entity cannot. Its event-entity example, and the related sentence on the events page, no longer suggest encoding an event's looked-up entity from inside the handler. Readers of the site get the same rule the Python package documents.

**Validation point**: The site builds, and a review of both pages finds the two statements.

#### - [ ] Task: Say on the xcl website when configuration text can be rendered
**Id:** ec6c7a60-9480-4041-bdcb-0ec3ea68870a
**Repo:** xcl-website
**Depends on:**
- 27c95072-5647-41dc-8ca8-d9808f0dba5a — Document when Python programs can render configuration text
**Execution:** agent

Updates the configuration-text page's saved-data section to say that a saved record can be rendered at any time, including from an event handler while an operation runs, with a Python example that prints each created resource beneath its success event. It also says that rendering a looked-up entity is refused during an operation. The page's event-entity example, and the related sentence on the events page, are reworded so they no longer suggest encoding an event's looked-up entity from inside the handler.

*Technical detail:* [context.md#task-say-on-the-xcl-website-when-configuration-text-can-be-rendered](./context.md#task-say-on-the-xcl-website-when-configuration-text-can-be-rendered)

**Acceptance criteria**:
- [ ] The configuration-text page states that saved records render during an operation and from an event handler, and that looked-up entities do not.
- [ ] The events page no longer implies that a looked-up event entity can be encoded from inside the handler during an operation.
- [ ] The site builds without errors.

## Open Questions

- **Does the race detector find shared mutable state on the saved-record render path when it runs concurrently with an operation?** Reading the code, the render touches only the read-locked plugin catalog and the stateless encoder, and Go handlers already decode event data the same way during operations. But only running the new core-service tests under `-race` can confirm it. *Depends on*: the first two Milestone 1 tasks. *If a race is reported*: STOP and ask the user. A fix would mean locking in the Go library, which the spec steers away from.
- **Does an external plugin stay running when a handler renders during an apply?** The catalog's use counting says it does, and this is the spec's flagged risk. *Depends on*: the plugin-lifecycle verification task. *If the plugin is observed stopped, or the apply fails*: STOP and ask the user before changing any library code.

## Out of Scope

- **Changing Go's encoding API.** Go programs already render in process, and `Config.EncodeSavedEntity` is not edited (spec Non-Goal).
- **Events carrying rendered configuration text themselves.** A host renders the record it receives; events are unchanged (spec Non-Goal).
- **A TypeScript counterpart.** There is no TypeScript package yet (spec Non-Goal).
- **Allowing any other call during an operation.** `EncodeEntity`, `Entities`, `Find`, `FindByType`, `FindOne` and `Outputs` stay refused. The spec's constraint makes the saved-record render the only newly allowed call.
- **The pretty Python logger that prints each created resource.** It is the first consumer, tracked as `20261023075328-741138ee-python-prettylog` in the same epic, and needs no further engine change.
- **Labelled or token output from the encode calls, and changes to how `highlight` behaves.** These are tracked as `20261023075136-42c51a7d-printing-tokens-over-the-engine` in the same epic.
- **Making the render avoid briefly starting and stopping external plugins** when it runs while no operation holds the catalog, at the very start or end of an operation. That is today's behaviour outside an operation too, and it is left as is.
- **Bumping the version badge on xcl.dev's home page.** It follows the release that ships this, as other site releases do.
