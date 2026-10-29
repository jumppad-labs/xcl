---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Context: 20261023074627-cc23fc08-encode-saved-entity-during-operation

## Current State Analysis

- `coreservice/lookups.go:121-169` — `encodeSavedEntity` and `encodeEntity` both call `encodeAllowed`, which runs `conf.lookupAllowed` (refuses with `operation_in_progress` while an operation runs) and then rejects an empty record with `invalid_argument`.
- `coreservice/operations.go:77-90` — `running()` and `lookupAllowed`. The guard exists because "the entity slice is replaced when an operation ends".
- `encode.go:154-185` — `Config.EncodeSavedEntity` reads none of `c.entities`. It calls `c.catalog.Use(nil)`, then `savedentity.Decode(c.catalog, ...)`, then the shared `encodeEntity`. Its godoc already says calls during an operation are safe.
- `encode_record.go:48-87` — `Config.EncodeEntityRecord` fills redacted values from `c.FindResource(meta.ID)`, which reads the live entities. It must stay refused.
- `internal/catalog/catalog.go:710-758` — `Use` and `release` count users under `usersMu`. External plugins are restarted at 0→1 and stopped at 1→0.
- `config.go:628-760` — `run` emits the start event, runs `withPlugins(work)` (which calls `catalog.Use` inside the work) on a goroutine, and emits the finished event after `done()`. The handler drains on the calling goroutine.
- `coreservice/events.go:10-17`, `coreservice/queue.go:45-80` — events are posted with no completion. State and function tasks wait for `Complete`.
- `coreservice/operations_test.go:769-787` — `TestEncodeSavedEntityDuringOperationRefused` asserts the refusal this plan removes.
- `design:libxcl.md` (~187, ~198-199, ~491) states that every encode is refused during an operation.
- xcl-python: both clients expose `encode_saved_entity` and `encode_entity` (`xcl/config.py:213-288`, `xcl/aio.py:258-296`). The README (290-306) says encoding while an operation runs raises `OperationInProgressError`. The engine is pinned at `v0.11.1` (`xcl-ref.txt`).
- xcl-website: `src/pages/configuration-text.mdx:82-142` documents saved-data rendering with no timing rule. `src/pages/events.mdx:210-215` suggests `cfg.encode_entity(event.entity())`.

## Per-Task Technical Notes

### Requirement-to-repo resolution

| Requirement | Repo | Where |
|---|---|---|
| Saved records render while an operation runs | xcl | `coreservice/lookups.go` (guard), `proto/xcl/core/v1/core.proto` (comment) |
| Same text during and after | xcl, xcl-python | `coreservice/operations_test.go`; `xcl-python:tests/integration/test_sync.py`, `test_aio.py` |
| The operation is unaffected | xcl, xcl-python | `coreservice/operations_test.go`, `config_plugin_lifecycle_test.go`; Python integration suites |
| Looked-up/edited entity still refused | xcl, xcl-python | `coreservice/lookups.go` (guard kept on `encodeEntity`); tests in both repos |
| Errors stay specific | xcl, xcl-python | `coreservice/operations_test.go`; Python integration suites |
| Python ships the capability | xcl-python (+ xcl release) | `xcl-python:xcl-ref.txt` via engine bump; no package code change |
| Documentation says when rendering is allowed | xcl-python, xcl-website, xcl (design) | `xcl-python:README.md`, `xcl/config.py`, `xcl/aio.py`; `xcl-website:src/pages/configuration-text.mdx`, `events.mdx`; `design:libxcl.md` |

### Task: Allow the saved-record render while an operation runs

**File changes**:
- `coreservice/lookups.go:121-135` — In `encodeSavedEntity`, replace `encodeAllowed(conf, "EncodeSavedEntity", req.GetEntity())` with a call to the new record-presence helper only. Add a comment saying it does not call `lookupAllowed`. The comment should say: `Config.EncodeSavedEntity` reads none of the configuration's entities (it types the record through the catalog and renders it), so the reason lookups are refused, that the entity slice is replaced when an operation ends, does not apply, and the call is allowed during an operation, including from a host's event handling.
- `coreservice/lookups.go:137-152` — In `encodeEntity`, call `conf.lookupAllowed("EncodeEntity")` and then the record-presence helper. Keep the existing order, so it still answers `operation_in_progress` before checking the record. Note in the comment that it reads the live entity by `meta.id` through `EncodeEntityRecord`, so it stays refused.
- `coreservice/lookups.go:154-169` — Replace `encodeAllowed` with `requireEntity(entity []byte) error`, which returns `invalidArgument("an entity is required", "argument", "entity")` when the record is empty. Rewrite the doc comment to match. Both handlers call it, so they still reject an empty record identically.
- `coreservice/operations.go:82-90` — Leave `lookupAllowed` unchanged. Optionally extend its comment with one line: "a call that reads none of the entities, such as EncodeSavedEntity, does not need it".
- `proto/xcl/core/v1/core.proto:46` — Add a comment above `rpc EncodeSavedEntity`, worded as in plan.md § Data Structures. Leave the `EncodeEntity` comment (lines 48-50) as it is.
- `coreservice/proto/core.pb.go` — Regenerate with `make core-protos` (buf generate). Only comment text changes. Run `make buf-lint` and `make buf-breaking`; both must pass, because the change is comment-only.
- `coreservice/operations_test.go:769-787` — Delete `TestEncodeSavedEntityDuringOperationRefused`. Keep `TestEncodeEntityDuringOperationRefused` (749-767) and `TestLookupDuringOperationRefused` (729-747) unchanged.
- `coreservice/operations_test.go` (new tests, near the deleted one) — Add a package-local helper, e.g. `holdAtFirstStateTask(t, c, handle, method, req) (opID uint64, task *corepb.StateTask)`. It starts the operation on a `hostRequest` configuration and returns its first state task via `nextStateTask`, which is defined at lines 217-235. Then add one test per operation:
  - `TestEncodeSavedEntityDuringValidate`
  - `TestEncodeSavedEntityDuringApply`
  - `TestEncodeSavedEntityDuringDiff`
  - `TestEncodeSavedEntityDuringDestroy`
  - `TestEncodeSavedEntityDuringLoad`

  Each test does the following:
  1. Apply `appConfig` first with a `fakeStore`, via `c.drive` (service_test.go:132), so the store holds real records. Take the `app.web` record from `store.saved` (see `recordFor`, operations_test.go:171) and use it as the record to render.
  2. Start the operation and wait for its first state task.
  3. Call `EncodeSavedEntity` with the record, and require text that contains `app "web"`.
  4. Complete the held task with `store.answer(task)` and `drive` to the end, requiring no error.

  For Validate and Diff use `appConfig`. For Destroy and Load, no paths are needed.
- `coreservice/operations_test.go` (error tests, during an operation, each held as above during an apply):
  - `TestEncodeSavedEntityInvalidJSONDuringOperation` — the record `this is not json` gives `CodeInvalidSavedData`.
  - `TestEncodeSavedEntityUnregisteredTypeDuringOperation` — the record `{"meta":{"id":"bogus.x","name":"x","type":"bogus"},"image":"nginx"}` gives `CodeUnregisteredType` with `details["type"] == "bogus"`, matching `TestEncodeSavedEntityUnregisteredType` at lookups_test.go:506.
  - `TestEncodeSavedEntityEmptyDuringOperation` — an empty request gives `CodeInvalidArgument` with `argument=entity`.

  Each also requires that the operation then finishes without error.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution. Make the handler change and regenerate first, then the tests. Run `go test -race ./coreservice/...` and `go vet ./...`.

### Task: Prove rendering from an event handler leaves the apply unchanged

**File changes**:
- `coreservice/operations_test.go` (new tests) — These use a `hostRequest` configuration (processed event data, host store) and `c.drive(handle, opID, store, onEvent)` (service_test.go:132). `onEvent` runs before the next poll, and the SAVE task that follows the create success event cannot complete until the host polls it, so the operation is provably running inside `onEvent`.
  - `TestEncodeSavedEntityDuringApplyMatchesAfter` — In `onEvent`, for `create/success` of `app.web`, render `e.GetData()` with `Options: &corepb.EncodeOptions{IncludeComputed: true}` and keep the text. After `drive` returns with no error, render the same bytes with the same options, then require equal text. Repeat the comparison with no options, in the same test (one positive behaviour).
  - `TestApplyUnchangedByRenderingInHandler` — Run two fresh configurations, each with its own `fakeStore`, and apply `appConfig` to each. One `onEvent` renders every `create/success` record and requires no error. The other only records. For each run, collect `eventName(e)` (operations_test.go:187) in order, the finished error (`nil`) and the store's saved records. Require equal event-name sequences and equal saved records, after decoding them with `decodeEntities` (operations_test.go:337), so the comparison is semantic. If the two stores' time-dependent fields differ, compare as `TestApplyMatchesGoApply` (operations_test.go:296) does.
  - `TestEncodeEntityFromHandlerDuringApplyRefused` — In `onEvent` on `create/success` of `app.web`, call `EncodeEntity` with the event's data treated as an entity record, and require `CodeOperationInProgress`. Then require that `drive` finishes with no error.
  - `TestEncodeEditedEntityFromHandlerDuringApplyRefused` — The same set-up. Decode the event data into a map, set `image` to `"edited"`, re-marshal it, call `EncodeEntity`, and require `CodeOperationInProgress`.
- Use `c.fail` and `c.ok`, as the existing tests do. The `testClient` helpers are safe to call from inside `onEvent`, since `drive` is single-goroutine.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution, after the first task. Run `go test -race ./coreservice/...`.

### Task: Verify plugins keep running when a handler renders during an apply

**File changes**:
- `config_plugin_lifecycle_test.go` (new test after line 53) — `TestRenderFromHandlerDuringApplyKeepsTheExternalPluginRunning`:
  1. Build the subtypeless external plugin with `buildSubtypelessPlugin(t)` (config_plugin_subtypeless_test.go:128) and call `isolateHome(t)`.
  2. Build the config with `newPersonConfig(t, externalWidgetRegistry(binary), subtypelessFixture(t, "subtypeless"), t.TempDir(), WithEventData(EventDataProcessed), WithEventBufferSize(1), WithEventHandler(handler))` (config_plugin_boundary_test.go:141). Check how `newPersonConfig` threads `opts`, and the fixture's resource ids.
  3. The handler, on the first resource's create success event, calls `c.EncodeSavedEntity(e.Data)`. It records the error and the text, and records `externalHost(t, c).Running()` immediately after the call returns. A buffer of one means the work goroutine cannot finish, and so cannot release its catalog use, while the handler is busy with an event that more events follow. Choose the fixture's first created resource so further events follow it. If the fixture creates only one resource, the operation's own later events, such as state save logs and other resources, must still follow it. Confirm this with a debug run before relying on it, and if needed use a fixture file with two widgets.
  4. After `Apply` returns, require: no apply error; no render error; non-empty text; running observed as `true`; and `externalHost(t, c).Running()` now `false`.

  Because the handler needs the config, capture `c` through a variable set after construction, as other tests in the package do.
- No library change is expected. If running is observed `false`, or the apply fails, STOP: the spec's risk is real, and the fix needs the user's direction.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution. Run `go test -race -run TestRenderFromHandlerDuringApply ./`.

### Task: Describe the change in the xcl changelog and the other-language design

**File changes**:
- `CHANGELOG.md:3` — Insert a new section `## 20261023074627-cc23fc08-encode-saved-entity-during-operation` above the current first section. Describe the following, and end with `**Breaking:**` and `None.`:
  - the core service's `EncodeSavedEntity` is now accepted while an operation runs on the handle, including from a host's event handling, so a host can render each resource's saved record as its create success event arrives;
  - the text is the same as after the operation;
  - `EncodeEntity`, `Entities`, `Find`, `FindByType`, `FindOne` and `Outputs` are still refused with `operation_in_progress`, because they read entities an operation replaces when it ends;
  - an empty, invalid or unknown record reports the same error during an operation as outside one;
  - Go programs are unaffected, and the protocol's messages are unchanged.
- `design:libxcl.md` (from the `design` design source) — Revise through the CLI and never edit the file directly. Read it with `spektacular design read --data '{"source":"design","path":"libxcl.md"}'`, stage the edited copy under `.spektacular/tmp/`, then write it back with `spektacular design author --data '{"source":"design","path":"libxcl.md"}' --from <staged file>` (the spek-design skill's revise flow), and remove the staged file. Make these edits:
  - line ~187, error table row: `operation_in_progress` becomes "a lookup other than `EncodeSavedEntity` was made while an operation was running on the handle".
  - line ~199, Threading rule: split it in two. Lookups (`Find*`, `Entities`, `Outputs`, `EncodeEntity`) stay refused, for the existing reason. `EncodeSavedEntity` is allowed while an operation runs, including from a host's event handling, because it reads none of the configuration's entities: it types the record through the catalog, whose use counting keeps the operation's plugins running.
  - line ~491, encode paragraph: replace "Encoding an entity as configuration text is a lookup too, refused while an operation runs." with the same distinction.
  - the service block comment near line ~243, if it lists refusals.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Test rendering from Python event handlers during an apply

**File changes**:
- `xcl-python:xcl-ref.txt` — During development, point it at the xcl feature branch, so CI builds the engine from it, as the prior encode plan did. Locally, build with `make protos engine plugins XCL_SRC=<xcl worktree> PYTHON=.venv/bin/python`. Protocol messages are unchanged, so `make protos` produces no diff.
- `xcl-python:tests/integration/test_sync.py` (new tests after `test_encode_entity_during_operation_refused`, line 1485) — Use `plugin_registry(slow_plugin)` and a config of one resource, `resource "slow" "one" { delay_ms = 0 }`, so its record has the computed `done` field. Use `event_data="processed"` (or the enum, if the event-data-level spec has landed) and `state_store=MemoryStore()` (sync_entities.py:138). The host store guarantees the engine waits on Python after the create success event: the next task is SAVE, which the sync driver only polls after the handler returns (xcl-python:xcl/_poll.py:178-200). Hold the config in a list for the handler, as the existing refusal test does. Add these tests:
  - `test_encode_saved_entity_from_handler_during_apply` — On `event.operation == "create" and event.phase == "success"`, call `configs[0].encode_saved_entity(event.data, include_computed=True)` and keep the text and the data. After `c.apply`, render the kept data again, then assert the texts are equal, non-empty and contain `slow "one"` and `done`.
  - `test_apply_unchanged_by_rendering_in_handler` — Run two applies in fresh directories and stores, one rendering every create success record and one not. Compare the lists of `(event.operation, event.phase)` and the stores' saved records, decoded as JSON.
  - `test_encode_entity_from_handler_during_apply_refused` — `encode_entity(event.entity())` raises `xcl.OperationInProgressError`, and the apply completes.
  - `test_encode_edited_entity_from_handler_during_apply_refused` — Take `event.entity()`, change a field (`delay_ms`), call `encode_entity`, and it raises `xcl.OperationInProgressError`.
  - `test_encode_saved_invalid_record_from_handler_during_apply` — `encode_saved_entity(b"not json")` raises `xcl.InvalidSavedDataError`.
  - `test_encode_saved_unregistered_record_from_handler_during_apply` — For a well-formed `bogus` record, catch the error class during the apply, render the same record after the apply, and assert both raise `xcl.UnregisteredTypeError`.

  Collect exceptions in lists inside handlers, as the existing refusal test does, so a failure inside the handler is reported, not swallowed by the operation.
- `xcl-python:tests/integration/test_aio.py` (new tests after `test_async_encode_entity_during_operation_refused`, line 836) — These are async twins of the sync tests, with `async def on_event` awaiting `configs[0].encode_saved_entity(...)`. Use a store derived from `AsyncMemoryStore` (test_aio.py:129) whose `save` awaits an `asyncio.Event` that the handler sets after rendering (with a timeout), so the render deterministically happens before the engine's SAVE completes. At minimum, add `test_async_encode_saved_entity_from_handler_during_apply` (the during/after equality) and `test_async_encode_entity_from_handler_during_apply_refused`. Add the async unchanged-apply and error twins only if cheap. The sync suite carries the full matrix.
- Do not change `xcl-python:xcl/*` code. Both clients already call `EncodeSavedEntity` (xcl/config.py:259-288, xcl/aio.py:279-296).

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: 2 parallel agents, one for the sync suite and one for the asyncio suite, after one agent builds the engine and plugins from the xcl branch. Run `pytest tests/integration -k "handler_during_apply or unchanged_by_rendering"` with `XCL_TRANSPORT` unset (subprocess engine), then once with `XCL_TRANSPORT=inprocess` if `make libxcl` is available.

### Task: Document when Python programs can render configuration text

**File changes**:
- `xcl-python:xcl/config.py:259-288` — Extend the `encode_saved_entity` docstring: it can be called at any time, including from an `on_event` handler while an operation runs, so a program can print each created resource as its success event arrives. Name the errors it raises (`InvalidSavedDataError`, `UnregisteredTypeError`, `NotEncodableError`, `PluginLoadError`).
- `xcl-python:xcl/config.py:243-250` — In the `encode_entity` docstring, keep "raises `xcl.OperationInProgressError` while an operation runs", and add "to render from an event handler, use `encode_saved_entity(event.data)`".
- `xcl-python:xcl/aio.py:268,289` — The one-line docstrings point to the sync methods. Add "allowed during an operation, including from an event handler" to the `encode_saved_entity` line.
- `xcl-python:README.md:290-306` — Rewrite the end of the "Encoding entities as configuration" section. `cfg.encode_saved_entity(record)` works at any time, including from `on_event` during an operation. Show a short example:

  ```python
  def on_event(e):
      if e.operation == "create" and e.phase == "success":
          print(cfg.encode_saved_entity(e.data))
  ```

  with `event_data="processed"`. State that `encode_entity` raises `xcl.OperationInProgressError` while an operation runs.
- `xcl-python:PARITY.md` — Optional. No change is needed unless a sentence there implies the refusal, and none does at lines 30-55.
- `tests/unit/test_docstrings.py` already checks that docstrings exist. Do not add tests that read docs.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Release xcl with the saved-record render allowed during operations

**What the person does**: After the xcl-python pull request, pinned to the xcl branch, shows the new tests passing, merge the xcl pull request with a release-size label (`minor` recommended; see the assumptions). Auto-tag (`.github/workflows/auto-tag.yml`, called from xcl's tag workflow) creates the next `v*` tag. Check it with `git tag`; the latest at planning time was `v0.11.1`. `release.yml` then builds and attaches `xcl-core` and dispatches `xcl-release` to xcl-python. There, `engine-bump.yml` opens `engine/<tag>`, updates `xcl-ref.txt`, and auto-merges once checks pass, and `tag.yml` releases the package.

**Ordering (decided during epic planning)**: cut this release before `20261023075136-42c51a7d-printing-tokens-over-the-engine` merges to xcl main. That change reserves the protocol's `highlight` field, and xcl-python's CI regenerates its stubs from the pinned xcl, so an engine bump to a release containing it fails until the Python printing work (`20261023075302-bad71237-printing-and-highlighting-python`) lands with it. If the ordering is missed, do not force the bump: this plan's Python tests and docs ship with the Python printing release instead.

**How it is checked**: The tag exists and contains the core-service change. The xcl-python engine bump pull request for that tag has merged. If its checks fail, a maintainer fixes it on the same branch.

### Task: Run the Python suites against the released engine

**File changes**:
- `xcl-python:xcl-ref.txt` — Rebase or merge the feature branch onto xcl-python `main`, which now pins the released tag through the engine bump. Make sure the file names that tag (e.g. `v0.12.0`) and not the xcl development branch. Resolve any conflict in favour of the released tag.
- No other file changes. Build the engine from the tag with `make engine plugins` (default `XCL_SRC`, checked out at the tag) and run the new sync and asyncio tests.

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Say on the xcl website when configuration text can be rendered

**File changes**:
- `xcl-website:src/pages/configuration-text.mdx:82-110` — In "Converting saved data", add a paragraph saying `c.EncodeSavedEntity` / `cfg.encode_saved_entity` can be called at any time, including from an event handler while an operation runs, which is how a program shows each resource as it is created. Extend the Python tab (lines 97-104) with a handler example rendering `event.data` on create success. State that rendering a looked-up entity (`cfg.encode_entity`) is refused with `xcl.OperationInProgressError` while an operation runs.
- `xcl-website:src/pages/configuration-text.mdx:112-142` — In the `event.entity()` example's Python tab, add that `cfg.encode_entity(entity)` works once the operation has finished, and that from inside the handler, `cfg.encode_saved_entity(event.data)` gives the same text.
- `xcl-website:src/pages/events.mdx:210-215` — Reword "In Python, `cfg.encode_entity(event.entity())` turns the entity an event carries into configuration text directly" to say this works after the operation, and that during it, from the handler, `cfg.encode_saved_entity(event.data)` does.
- Run the site build (`npm run build`, or the Makefile target) to confirm it builds. Follow the website's gotchas (no redirects needed; no pages removed).

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution.

## Testing Strategy

Per task:
- *Allow the saved-record render while an operation runs* — five `coreservice` tests (one per operation), each held at its first host state-store task; three error tests during an operation (invalid JSON, unknown type, empty); the old refusal test deleted; `go test -race ./coreservice/...`.
- *Prove rendering from an event handler leaves the apply unchanged* — four `coreservice` tests rendering inside `drive`'s `onEvent` on create success: during/after equality, the unchanged-apply comparison, and the looked-up and edited refusals.
- *Verify plugins keep running when a handler renders during an apply* — one root-package test with the external subtypeless plugin and `WithEventBufferSize(1)`.
- *Test rendering from Python event handlers during an apply* — six sync and two or more asyncio integration tests, with a Python state store, using the slow fixture plugin. Run against the engine built from the xcl branch, and with `XCL_TRANSPORT=inprocess` where available.
- *Run the Python suites against the released engine* — rerun the new Python tests against the engine built from the released tag.
- Docs, changelog and design tasks: human review only (see plan.md § Testing Approach, manual items). No test reads docs.

## Project References

- Spec: `20261023074627-cc23fc08-encode-saved-entity-during-operation` (epic `20261023074219-4357d314-printing-and-pretty-output`).
- Design revised (not built on): `libxcl.md` from the `design` design source.
- Knowledge (xcl repo store): `conventions/testing-and-mocking.md`, `conventions/shared-test-helpers.md`, `conventions/test-state-from-real-apply.md`, `conventions/code-style.md`, `conventions/shared-errors-package.md`, `learnings/releases-tagged-by-hand.md`.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.
- Prior plan: `20261016064106-8931613a-python-encode-entity`.
- Sibling specs: `20261023075136-42c51a7d-printing-tokens-over-the-engine` (also edits the encode handlers), `20261023074223-e06f835f-event-data-level-enum` (the Python `event_data` form), `20261023075328-741138ee-python-prettylog` (consumer).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Every task here is Low, except the Python test task (Medium, with the sync and asyncio suites written in parallel). Total is about 145k tokens.

## Migration Notes

None. No data, state or protocol message changes. Hosts that relied on `EncodeSavedEntity` being refused during an operation would now get text. No known host does; the Python package documented the refusal only as an error to expect.

## Performance Considerations

- A render during an operation adds one catalog use count increment and decrement, and no plugin restarts.
- A render at the very start (before the operation's catalog use) or end (after it) of an operation, or after it, briefly starts and stops external plugin processes, as any render outside an operation does today.
- A render issued while the first operation is still loading plugins waits for the load to finish.
- The Python asyncio client serialises engine calls on its single-worker executor, so a handler's render runs between polls. It does not block the event loop.
