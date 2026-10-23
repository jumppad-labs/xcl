---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Research: 20261023074627-cc23fc08-encode-saved-entity-during-operation

## Alternatives considered and rejected

- **Buffer renders in the Python package until the operation ends.** The package would queue `encode_saved_entity` calls made from a handler and run them after the finished task. Rejected: the spec's success metric demands "no buffering until the apply ends", and the constraint forbids the package waiting inside a handler. The engine already refuses the call; the package cannot work around it (xcl-python:xcl/config.py:259-288, xcl-python:xcl/aio.py:279-296).
- **Change Go's `Config.EncodeSavedEntity` (add a lock, or a new method).** Rejected: the spec's Non-Goals exclude changing Go's encoding API, and its Technical Approach prefers the service guard. Go already documents calls during an operation as safe (xcl:encode.go:154-185: "External plugin processes are stopped again once the record is typed, unless an operation is still using them"), and Go handlers already decode event data concurrently with the operation through the same `savedentity.Decode` (xcl:config.go:743-745).
- **Relax the guard for every encode call, or for all lookups.** Rejected: `EncodeEntity` calls `Config.EncodeEntityRecord`, which reads the live entity slice through `c.FindResource` (xcl:encode_record.go:66-87). That slice is replaced when Apply, Destroy and Load end (xcl:config.go:442, 515, 571), which is the guard's stated reason (xcl:coreservice/operations.go:82-90). The spec's constraint also says the saved-record render is the only call newly allowed.
- **A per-method policy flag in the service's dispatch table** (`unary(allowedDuringOperation, ...)` at xcl:coreservice/service.go:225). Rejected as more machinery than one call needs. Every lookup handler calls its guard explicitly, so the absence of the guard at one call site, with a comment giving the reason, is the smallest and most readable change.
- **Hold each operation in tests through a blocking registry during plugin load.** Rejected: encode's own `catalog.Use` takes `usersMu`, which the operation holds while it loads plugins (xcl:internal/catalog/catalog.go:720-746). The render would wait for the load and the test would hang. Tests hold operations at a host state-store task instead (see Chosen approach).

## Chosen approach — evidence

- The only refusal is service-level: `encodeSavedEntity` calls `encodeAllowed`, which calls `conf.lookupAllowed` (xcl:coreservice/lookups.go:121-169). `lookupAllowed` checks `c.running()` (xcl:coreservice/operations.go:77-90). Removing the guard from `encodeSavedEntity` alone, and keeping the empty-record check, is the change.
- `Config.EncodeSavedEntity` never reads `c.entities`. It uses `c.catalog.Use(nil)`, `savedentity.Decode(c.catalog, ...)` and the pure `encodeEntity` (xcl:encode.go:154-185, 189-260). Catalog reads take `c.mu.RLock` (xcl:internal/catalog/catalog.go:450-453, 642-646).
- `catalog.Use` counts users under `usersMu`. A render during an operation goes from N to N+1 users and back, so it never stops plugins the operation uses. A render while no operation holds the catalog (before `withPlugins` runs, or after it returns) starts and stops the external plugins, exactly as a render outside an operation does today (xcl:internal/catalog/catalog.go:710-758, xcl:config.go:700-717). Counting is covered by xcl:config_plugin_lifecycle_test.go:74-130.
- Events cross the service fire-and-forget: `handleEvent` posts without a completion (xcl:coreservice/events.go:10-17, xcl:coreservice/queue.go:45-55). So the operation does not wait on the host while the host renders, and a render cannot deadlock the operation. A state task does wait for `Complete` (xcl:coreservice/queue.go:57-80).
- Every operation reads the host state store before it ends. Validate checks `Exists` in `parseAndValidate` (xcl:internal/parser/parser.go:676-712). Apply and Diff parse the same way. Destroy and Load call `Exists` first (xcl:config.go:493-520, 545-575). A host store therefore holds any operation at its first state task. The test helpers `hostRequest`, `nextStateTask`, `fakeStore` and `drive(..., onEvent)` already exist (xcl:coreservice/operations_test.go:39-47, 217-235; xcl:coreservice/service_test.go:132-215).
- During an apply with a host store, the create success event is posted before the SAVE task that follows it. The host handles the event before it answers SAVE, so the operation is provably still running when a handler renders. This holds in the Go test client (`drive` calls onEvent before the next poll) and in Python's sync driver (xcl-python:xcl/_poll.py:178-200).
- Python already exposes `encode_saved_entity` on both clients and needs no code change to call it. The asyncio client runs lookups on its single-worker executor (xcl-python:xcl/aio.py:66-67, 114-116, 216-218), so a handler `await`s the render without blocking the loop.
- The existing refusal tests to invert or narrow: xcl:coreservice/operations_test.go:769-787 (`TestEncodeSavedEntityDuringOperationRefused`). Tests to keep: `TestEncodeEntityDuringOperationRefused` (749-767); xcl-python:tests/integration/test_sync.py:1485-1514 and xcl-python:tests/integration/test_aio.py:836-861 (encode_entity refused).

## Files examined

- xcl:coreservice/operations.go:28-90 — `start`, `running`, `lookupAllowed` (the guard and its reason).
- xcl:coreservice/lookups.go:121-169 — `encodeSavedEntity`, `encodeEntity`, the shared `encodeAllowed`.
- xcl:coreservice/errors.go:41,61-64 — `CodeOperationInProgress` and its message "<what> cannot run while an operation is in progress".
- xcl:coreservice/events.go:10-17 — events posted with no completion.
- xcl:coreservice/queue.go:14-80 — 256-task bound; `post` vs `run`.
- xcl:coreservice/service.go:58-90,225-226 — dispatch has no per-configuration serialisation.
- xcl:coreservice/operations_test.go:39-135,187-235,709-787 — fixtures, the hold helpers, the refusal tests.
- xcl:coreservice/service_test.go:132-215,256-275 — `drive`, `fakeStore`, `appRequest`.
- xcl:coreservice/lookups_test.go:156-190,500-525 — encode test style and the unregistered-type and invalid-data assertions.
- xcl:encode.go:136-185 — `EncodeSavedEntity` godoc and body.
- xcl:encode_record.go:20-87 — `EncodeEntityRecord` reads live entities through `storedFor`.
- xcl:config.go:425-575,628-760 — operations replace `c.entities`; `run`/`withPlugins` call `catalog.Use` inside the work, after the start event and before the finished event.
- xcl:internal/catalog/catalog.go:640-800 — `Use`, `release`, `Load`, the lock order.
- xcl:config_plugin_lifecycle_test.go:1-135 — external-plugin lifecycle tests and fixtures (`externalWidgetConfig`, `externalHost`, `buildSubtypelessPlugin`).
- xcl:options.go:106-124 — `WithEventHandler`, `WithEventBufferSize`.
- xcl:proto/xcl/core/v1/core.proto:44-51 — `EncodeEntity` comment says it is refused like other lookups; `EncodeSavedEntity` has no comment.
- xcl:CHANGELOG.md:1-12 — one section per spec, headed by the spec name, ending with a `**Breaking:**` list.
- design:libxcl.md:187,198-199,491 — the error table row, the Threading rule and the encode paragraph that say every encode is refused during an operation.
- xcl-python:xcl/config.py:213-288 — sync `encode_entity` and `encode_saved_entity` docstrings.
- xcl-python:xcl/aio.py:66-218,258-296 — async run loop, single-worker executor, encode methods.
- xcl-python:xcl/_poll.py:91-200 — sync driver calls the handler between polls.
- xcl-python:xcl/_errors.py:253-256 — `OperationInProgressError`.
- xcl-python:README.md:241-306 — the encode section ("encoding while an operation runs raises `xcl.OperationInProgressError`").
- xcl-python:PARITY.md:30-55 — the encode rows; no refusal wording.
- xcl-python:tests/integration/test_sync.py:1485-1514, test_aio.py:836-861 — refusal tests using `slow_plugin`.
- xcl-python:tests/integration/conftest.py:1-60 — fixture plugins built from `XCL_SRC`.
- xcl-python:xcl-ref.txt — engine pinned at `v0.11.1`.
- xcl-python:.github/workflows/engine-bump.yml:1-30, tag.yml:1-10, release.yml — xcl release dispatches the pin bump, which auto-merges and tags the package.
- xcl-website:src/pages/configuration-text.mdx:82-142 — "Converting saved data" section and the Python `encode_entity(event.entity())` example.
- xcl-website:src/pages/events.mdx:205-215 — `cfg.encode_entity(event.entity())` sentence.

## External references

- None needed. The behaviour is internal to xcl, its core service and the Python package.

## Prior plans / specs consulted

- Plan `20261016064106-8931613a-python-encode-entity` — the plan that added `EncodeEntity` and kept both encodes refused during an operation. Learned: changelog and `design:libxcl.md` were updated through a dedicated task; delivery order is xcl first, xcl-python developed against the xcl branch (`make protos engine plugins XCL_SRC=../xcl`), then pinned to the release, with the website last; merging the xcl PR with its release label was a `human` task.
- Epic `20261023074219-4357d314-printing-and-pretty-output` — sibling `20261023075136-42c51a7d-printing-tokens-over-the-engine` also edits `coreservice/lookups.go` (labelled text on both encode calls, `highlight` reinterpreted). Sibling `20261023075328-741138ee-python-prettylog` is the first consumer and depends on this spec. Sibling `20261023074223-e06f835f-event-data-level-enum` changes how Python's `event_data` is passed.
- Knowledge `learnings/releases-tagged-by-hand.md` — check `git tag` for the next version (latest is `v0.11.1`); cutting a release is a task for a person unless a workflow does it. Auto-tag now does it from the PR's release label.
- Knowledge `learnings/config-operations-not-cancellable.md` — background on context-taking operations; no impact.

## Open assumptions

- `savedentity.Decode` and `encodeEntity` touch no shared mutable state beyond the read-locked catalog. Go handlers already decode concurrently with operations. The race-enabled coreservice tests added by this plan confirm it. If the race detector reports a race, STOP and ask.
- A render requested while the operation is loading plugins waits on `usersMu` until loading ends. It does not deadlock, because plugin loading never waits on the host. It could deadlock only if more than 256 tasks were posted while the host is stuck in the render. This is unreachable with today's load events, and it is not tested.
- A render made while no operation holds the catalog (from the start event before `withPlugins` runs, or after work returns) starts and stops external plugin processes, as a render outside an operation does today. The cost is accepted and the result is correct.
- The next xcl release is a minor bump (v0.12.0 at the time of writing). The person merging picks the release label.

## Drafting assumptions

### Chosen direction: drop the lookup guard from the core service's saved-record encode only (architecture)
- **Decision**: Split the record-presence check out of `encodeAllowed` in `coreservice/lookups.go`. `encodeSavedEntity` runs only that check, and `encodeEntity` keeps the `lookupAllowed` guard plus the check. Add a proto comment on `rpc EncodeSavedEntity` and regenerate the Go stubs. No Go library change and no Python code change. Python gains tests and docs; the website and `design:libxcl.md` are updated.
- **Key design decisions**: guard at the call site with a comment that applies the guard's own test, not a dispatch-table flag; no new locking, relying on fire-and-forget events and the catalog's user counting; the spec's catalog risk is verified by a deterministic Go test (external plugin, event buffer of one, plugin still running after the render); error codes unchanged because the error paths are untouched.
- **Rationale**: This is the spec's preferred direction (Technical Approach). The guard's stated reason does not apply to a call that never reads the live entities. It is the smallest change, and it keeps `EncodeEntity` refused as the spec requires.
- **Rejected**: Python-side buffering (breaks the no-buffering metric and the no-wait constraint); a Go encoder change (Non-Goal, unnecessary); relaxing every encode or every lookup (exposes live-slice reads; the constraint allows one call only); a dispatch-table policy flag (machinery for one call).

### Add a proto comment and regenerate Go stubs (architecture)
- **Decision**: Add a comment to `rpc EncodeSavedEntity` in `proto/xcl/core/v1/core.proto` saying it is allowed while an operation runs, and run `make core-protos` (buf generate).
- **Rationale**: The sibling `EncodeEntity` rpc already documents its refusal in the proto. Documenting the difference where protocol readers look is cheap, and it is comment-only, so it is not a breaking change.
- **Rejected**: Leaving the proto silent (readers of the protocol would assume every lookup is refused).

### Conventions selected (architecture)
- **Decision**: Apply Testing & Mocking, Shared test helpers, Test state from a real apply, and Go Code Style. Record Shared errors as deliberately not triggered. Drop the rest (parser options builder, hclsyntax references, graph-parent ordering, provider Changed/Update, DB, dependencies, never-modify-deps, project structure, logging).
- **Rationale**: Only these bear on a core-service guard change and its tests. The others concern the parser, the DAG, plugins or databases, which this work does not touch.
- **Rejected**: Listing every convention (noise).

### Hold operations at a host state-store task in tests (discovery)
- **Decision**: Core-service tests hold each of Validate, Apply, Diff, Destroy and Load at their first host state-store task, and render while the operation waits there.
- **Rationale**: Every operation reads the store's `Exists` before it ends, and a state task waits for `Complete`. The existing helpers (`hostRequest`, `nextStateTask`, `fakeStore`) support this, and it gives one hold mechanism for all five.
- **Rejected**: A registry that blocks during plugin load (the render would wait on the catalog's `usersMu` and hang the test). `holdCreate` works only for Apply.

### Next xcl release is a minor bump (discovery)
- **Decision**: The xcl PR is labelled `minor`.
- **Rationale**: The change adds a capability without breaking anything. Before 1.0, xcl bumps the minor version for capabilities and breaking changes alike.
- **Rejected**: `patch`. It undersells a newly allowed call. The person merging still chooses the label.

### Align the website's events page too (components)
- **Decision**: Besides the configuration-text page the spec names, adjust the sentence on xcl-website's events page that suggests `cfg.encode_entity(event.entity())` from a handler.
- **Rationale**: That sentence would mislead readers once the configuration-text page says looked-up entities are refused during an operation. The fix is one sentence.
- **Rejected**: Leaving the events page untouched (inconsistent docs).

### Five-operation matrix tested at the service boundary only (testing_approach)
- **Decision**: "Every operation allows it" is proven by five core-service tests. Python tests cover apply only.
- **Rationale**: The rule lives in the engine. Python reaches it through the same call, and repeating the matrix in Python adds runtime without adding assurance.
- **Rejected**: A Python test per operation (slow, redundant).

### Python tests use a Python state store to guarantee in-progress rendering (testing_approach)
- **Decision**: Python acceptance tests pass a small in-memory `state_store`. In the asyncio test, the store's `save` may wait for the handler to finish rendering.
- **Rationale**: Events cross fire-and-forget, so with a file store a fast one-resource apply could finish before the handler runs, and the test would pass without the change. A host store makes the engine wait on Python after the event.
- **Rejected**: Relying on the slow plugin's delay (timing-based, flaky).

### The release is a human task; pinning is automated with an agent follow-up (tasks)
- **Decision**: One `human` task covers merging the xcl PR with its release label. The engine pin bump is left to xcl-python's existing automation. An agent task then brings the xcl-python branch onto the released pin and reruns the tests.
- **Rationale**: Publishing a release is an action outside the repo. The engine-bump workflow already writes `xcl-ref.txt` and releases the package, so no person needs to edit the pin by hand.
- **Rejected**: A human task to bump the pin (duplicates automation); no follow-up task (the feature branch could merge still pinned to a development branch).

### Catalog-risk test needs no code change and has no dependency (tasks)
- **Decision**: The external-plugin concurrency test is its own task, depends on nothing, and touches only a root-package test.
- **Rationale**: Go already allows `EncodeSavedEntity` during an operation, so the test verifies existing behaviour. If it fails, the implementer stops and asks, because a fix would change the Go library, which the spec steers away from.
- **Rejected**: Folding it into the core-service task (a different package, and it can run in parallel).

## Rehydration cues

- `spektacular spec file read 20261023074627-cc23fc08-encode-saved-entity-during-operation`
- Re-read xcl:coreservice/lookups.go:121-169 and xcl:coreservice/operations.go:77-90 (the guard).
- Re-read xcl:encode.go:154-185 and xcl:internal/catalog/catalog.go:710-758 (catalog use counting).
- Re-read xcl:coreservice/operations_test.go:709-787 and xcl:coreservice/service_test.go:132-215 (test client, hold helpers).
- `spektacular design read --data '{"source":"design","path":"libxcl.md"}'` (Threading section, error table, encode paragraph).
- xcl-python:README.md:290-306, xcl-python:xcl/config.py:213-288, xcl-website:src/pages/configuration-text.mdx:82-142.
