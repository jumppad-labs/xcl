---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Test plan: 20261023074627-cc23fc08-encode-saved-entity-during-operation

The spec's success metric, "a Python program can print each created resource's configuration beneath its success event during an apply, with no buffering until the apply ends", is covered by an automated behavioural test: `tests/integration/test_sync.py::test_encode_saved_entity_from_handler_during_apply` in xcl-python, with the asyncio twin `tests/integration/test_aio.py::test_async_encode_saved_entity_from_handler_during_apply`. The procedures below are the manual checks and reviews the plan lists. They include the two tasks descoped to follow-up work: releasing xcl, and re-running the Python suites against the released engine.

## 1. xcl is released (descoped task, done by a person after the merge)

- **What**: an xcl release tag that contains the core-service change, cut in the right order.
- **How**:
  1. Merge the xcl pull request for branch `spek/20261023074627-cc23fc08-encode-saved-entity-during-operation` with a release-size label (`minor` recommended; the latest tag at planning time was `v0.11.1`).
  2. Auto-tag cuts the next `v*` tag. `release.yml` builds `xcl-core` and dispatches `xcl-release` to xcl-python.
  3. Check that the new tag contains the change: `git tag --contains <merge commit>`. Check also that `coreservice/lookups.go` at the tag has `encodeSavedEntity` calling `requireEntity` and not `lookupAllowed`.
  4. In xcl-python, confirm that the `engine/<tag>` pull request opened by `engine-bump.yml` updates `xcl-ref.txt` to the tag and has merged.
- **Ordering constraint (decided during epic planning)**: cut this release **before** `20261023075136-42c51a7d-printing-tokens-over-the-engine` merges to xcl main. That change reserves the protocol's `highlight` field, and xcl-python's CI regenerates its stubs from the pinned xcl, so an automatic engine bump to a release containing it fails until the Python printing work lands with it. If the ordering is missed, do not force the automatic bump. This plan's Python tests and docs then ship with the release of `20261023075302-bad71237-printing-and-highlighting-python` instead.
- **Pass**: the tag exists and contains the change, and the engine pin bump in xcl-python has merged.
- **Who / when**: an xcl maintainer, after the spec's branches are merged.

## 2. Python suites against the released engine (descoped task, done by a person after the release)

- **What**: the new rendering tests pass against an engine built from the released tag, not the development branch.
- **How**: on xcl-python `main`, which carries this spec's tests and the bumped pin, do the following:
  1. Confirm that `cat xcl-ref.txt` names the released tag from check 1.
  2. Build the engine and plugins from that tag, using a checkout or `git archive` of xcl at the tag: `make protos engine plugins XCL_SRC=<xcl at tag> PYTHON=.venv/bin/python`.
  3. Run `.venv/bin/python -m pytest -q tests/integration -k "handler_during_apply or unchanged_by_rendering"`. Optionally run it again with `XCL_TRANSPORT=inprocess` after `make libxcl`.
- **Pass**: all selected tests pass (9 at the time of implementation: 6 synchronous, 3 asyncio). Note that against the old `v0.11.1` engine these tests fail, because the engine refuses the render. Until the pin is bumped, xcl-python's CI is expected to fail on them.
- **Who / when**: an xcl-python maintainer, once the engine bump has merged.

## 3. Installed package has it

- **What**: the released Python package renders saved records from an event handler with no local engine build.
- **How**:
  1. In a fresh environment with no xcl checkout on the path, run `python -m venv /tmp/xcl-check && /tmp/xcl-check/bin/pip install xcl` (the released version that bundles the tag from check 1).
  2. Check `/tmp/xcl-check/bin/python -c "import xcl; print(xcl.engine_version())"`.
  3. Run a program with a config-only type and an `on_event` handler that, on `create`/`success`, prints `cfg.encode_saved_entity(e.data, include_computed=True)`. Create the `xcl.Config` with `event_data=xcl.EventDataLevel.PROCESSED` and a Python `state_store` (for example the in-memory store in `tests/integration/sync_entities.py`), and apply a configuration that creates one resource. Then render the same record again after the apply.
- **Pass**: `xcl.engine_version()` reports the released xcl version from check 1. The handler prints configuration text with no exception. The apply succeeds. The text equals the post-apply render.
- **Who / when**: an xcl-python maintainer, after the package release.

## 4. Review the xcl changelog entry

- **Where**: xcl `CHANGELOG.md`, section `## 20261023074627-cc23fc08-encode-saved-entity-during-operation` at the top.
- **Look for**:
  - It says `EncodeSavedEntity` is accepted during Validate, Apply, Diff, Destroy and Load, including from event handling, with the same text as after.
  - It says `EncodeEntity`, `Entities`, `Find`, `FindByType`, `FindOne` and `Outputs` stay refused, and why.
  - It says bad records keep `invalid_argument`, `invalid_saved_data` and `unregistered_type`.
  - It says Go and the protocol messages are unchanged.
  - It ends with `**Breaking:**` and `None.`
- **Pass**: all present and accurate.

## 5. Review the revised `libxcl.md` design

- **Where**: `spektacular design read --data '{"source":"design","path":"libxcl.md"}'`. Check the error table row for `operation_in_progress`, the Threading rules, the CoreService block's lookups and the encode paragraph after the `EncodeEntity` messages.
- **Look for**:
  - The saved-record render is allowed during an operation, including from event handling.
  - `EncodeEntity` and the other lookups are refused.
  - No sentence still says every render is refused during an operation.
- **Pass**: all three hold.

## 6. Review the Python docstrings and README

- **Where**: xcl-python `xcl/config.py` (`Config.encode_saved_entity`, `Config.encode_entity`), `xcl/aio.py` (their asyncio twins), and the README section "Encoding entities as configuration".
- **Look for**:
  - Saved records render at any time, including from an `on_event` handler during an operation.
  - `encode_entity` raises `xcl.OperationInProgressError` during an operation and points at `encode_saved_entity(event.data)`.
  - The README example prints each created resource on its create success event.
- **Pass**: all three present and correct.

## 7. Review the xcl.dev pages and check the site builds

- **Where**: xcl-website `src/pages/configuration-text.mdx` ("Converting saved data", plus the `event.entity()` example after it) and `src/pages/events.mdx` (the paragraph on `Event.Entity()` and masking).
- **Look for**:
  - Saved records render during an operation and from an event handler.
  - Looked-up entities (`cfg.encode_entity`) are refused during an operation.
  - The events page no longer suggests `cfg.encode_entity(event.entity())` from inside the handler.
- **How**: run `npm ci && npm run build` in xcl-website.
- **Pass**: both statements are present, and the build completes with no errors.
