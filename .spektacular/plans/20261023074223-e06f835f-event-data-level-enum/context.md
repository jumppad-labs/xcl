---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Context: 20261023074223-e06f835f-event-data-level-enum

## Current State Analysis

- `xcl-python:xcl/config.py:20-24` holds `EVENT_DATA_LEVELS`, a public dict from the bare words `"none"`, `"raw"` and `"processed"` to the protocol's `core_pb2.EVENT_DATA_*` values.
- `xcl-python:xcl/config.py:56,75-76` and `xcl-python:xcl/aio.py:53,73-74` type `event_data` as `str` (default `"none"`) and refuse an unknown word with `ValueError`. The check runs after the client, class map and functions are set up, and for `aio.Config` after its `ThreadPoolExecutor` starts (`aio.py:67`).
- `xcl-python:xcl/config.py:89-117` builds `CreateConfigRequest`; `aio.Config` reuses it (`aio.py:76-85`) and creates the engine configuration lazily (`aio.py:89-92`). Sync `Config` calls `CreateConfig` at the end of `__init__` (`config.py:87`).
- The package has no `py.typed` marker. mypy reports `import-untyped` for `xcl` in a user's program (verified during planning) and so cannot check `event_data`.
- `xcl-python:pyproject.toml` requires Python >=3.11 (so `enum.StrEnum` is available); the `test` extra is only `pytest>=8`.
- The protocol already has its own `EventDataLevel` enum (`xcl-python:xcl/_core/core_pb2.pyi:10-17`); Go exposes `xcl.EventDataLevel` with `EventDataNone/Raw/Processed` (`xcl:events.go:21-41`) and `WithEventData` (`xcl:options.go:144`).
- Bare-string usages: `xcl-python:README.md:230`, `xcl-python:PARITY.md:66`, `xcl-python:tests/integration/test_sync.py:323,630,654,655,1040`, `xcl-python:tests/integration/test_aio.py:283,631`, `xcl-python:tests/integration/test_test_plugin.py:246`, `xcl-website:src/pages/events.mdx:191,203`, `xcl-website:src/pages/configuration-text.mdx:103`, `xcl-website:src/pages/examples/configuration-only.mdx:652`, `xcl-website:src/pages/examples/plugins.mdx:1136`.
- No existing test covers the raw level, the none level or the option left out against the engine.
- xcl-python's GitHub release notes are generated from PR titles plus an engine line (`xcl-python:.github/workflows/release.yml:388-407`). The latest tag at planning time was `v0.13.0`.

## Per-Task Technical Notes

### Requirement-to-repo resolution

| Spec requirement | Repo | Files |
|---|---|---|
| A named set of event data levels in the main package | xcl-python | `xcl/events.py`, `xcl/__init__.py` |
| Both configuration flavours take the named levels | xcl-python | `xcl/config.py`, `xcl/aio.py` |
| Leaving the option out means none | xcl-python | `xcl/config.py`, `xcl/aio.py` |
| Bare words are refused | xcl-python | `xcl/config.py` (shared check), `xcl/aio.py` |
| Anything else is refused | xcl-python | `xcl/config.py`, `xcl/aio.py` |
| Type checkers understand the option | xcl-python | `xcl/py.typed`, `pyproject.toml`, `tests/unit/test_typing.py` |
| Documentation shows the named levels | xcl-python, xcl-website | `README.md`; `src/pages/events.mdx`, `src/pages/configuration-text.mdx`, `src/pages/examples/configuration-only.mdx`, `src/pages/examples/plugins.mdx` |
| The parity record pairs the levels | xcl-python | `PARITY.md` |
| Release notes explain the break | xcl-python (GitHub release, by a person) | none in the tree |

xcl (engine, protocol, Go API) is unchanged.

### Task: Add the EventDataLevel enum and accept only its members

**File changes**:
- `xcl-python:xcl/events.py:1-14` — add `import enum` and declare `class EventDataLevel(enum.StrEnum)` above `Event`, with members `NONE = "none"`, `RAW = "raw"` and `PROCESSED = "processed"`. The class docstring says it is the Python form of Go's `xcl.EventDataLevel`, set with `event_data=` on `xcl.Config` and `xcl.aio.Config`, and describes each member in the words of `xcl:events.go:26-40`. `NONE` carries no resource data on any event and is the default. `RAW` carries the resource as it was before the provider was called, on every lifecycle event. `PROCESSED` carries, on a success event, the resource as xcl records it in state, including provider-filled values, read with `Event.entity()` or turned into text with `encode_saved_entity`, and masked by the event mask.
- `xcl-python:xcl/__init__.py:57` — import `EventDataLevel` alongside `Event` (`from xcl.events import Event, EventDataLevel`). Add `"EventDataLevel"` to `__all__` after `"Event"` (`__init__.py:66`).
- `xcl-python:xcl/config.py:17,20-24` — import `EventDataLevel` from `xcl.events`. Replace `EVENT_DATA_LEVELS` with a private `_EVENT_DATA_WIRE: dict[EventDataLevel, int]` mapping members to `core_pb2.EVENT_DATA_NONE/RAW/PROCESSED`. Add `def _event_data_level(value: object) -> int`. It raises `TypeError(f"event_data must be an xcl.EventDataLevel, such as xcl.EventDataLevel.PROCESSED, not {value!r} ({type(value).__name__})")` unless `isinstance(value, EventDataLevel)`, and otherwise returns `_EVENT_DATA_WIRE[value]`. The check must be `isinstance`, never `in` or a dict lookup alone: a StrEnum member hashes and compares equal to its string.
- `xcl-python:xcl/config.py:36` — docstring: ``event_data`` is an :class:`xcl.EventDataLevel`, ``EventDataLevel.NONE`` (the default), ``RAW`` or ``PROCESSED``.
- `xcl-python:xcl/config.py:56` — `event_data: EventDataLevel = EventDataLevel.NONE`.
- `xcl-python:xcl/config.py:61-62` — make `level = _event_data_level(event_data)` the first statement of `__init__`, before `self._client = ...`.
- `xcl-python:xcl/config.py:75-76` — remove the old `ValueError` check.
- `xcl-python:xcl/config.py:82,95,104` — `_request` takes the encoded `event_data: int = core_pb2.EVENT_DATA_NONE` and sets `event_data=event_data` on the request directly. The caller passes `event_data=level`.
- `xcl-python:xcl/aio.py:29` — import `_event_data_level` instead of `EVENT_DATA_LEVELS` (`from xcl.config import _encode_options, _event_data_level`). Import `EventDataLevel` from `xcl.events` (`aio.py:32`).
- `xcl-python:xcl/aio.py:53` — `event_data: EventDataLevel = EventDataLevel.NONE`.
- `xcl-python:xcl/aio.py:58-59` — make `level = _event_data_level(event_data)` the first statement of `__init__`, before the client, the class map and the `ThreadPoolExecutor` (`aio.py:67`).
- `xcl-python:xcl/aio.py:73-74,81` — remove the old `ValueError` check and pass `event_data=level` to `_SyncConfig._request`.
- `xcl-python:tests/integration/test_sync.py:323,630,654,655,1040` — `event_data="processed"` → `event_data=xcl.EventDataLevel.PROCESSED`.
- `xcl-python:tests/integration/test_aio.py:283,631` — same change.
- `xcl-python:tests/integration/test_test_plugin.py:246` — same change.
- `xcl-python:tests/unit/test_events.py` — add `test_event_data_level_imports_from_top_level`, asserting `xcl.EventDataLevel` is in `xcl.__all__`. Add `test_event_data_level_has_three_members`, asserting `[m.name for m in xcl.EventDataLevel] == ["NONE", "RAW", "PROCESSED"]` and the values `["none", "raw", "processed"]`.
- `xcl-python:tests/unit/test_config.py` (after `test_context_manager_closes`, line ~50) — add separate functions following `test_config_creates_and_keeps_handle` (`test_config.py:17-28`). Decode `transport.calls[-1][2]` with `core_pb2.CreateConfigRequest.FromString`.
  - Wire value, sync: `test_config_sends_event_data_none`, `..._raw`, `..._processed` and `test_config_event_data_defaults_to_none`.
  - Wire value, aio: `test_aio_config_sends_event_data_none`, `..._raw`, `..._processed` and `test_aio_config_event_data_defaults_to_none`. Each uses `asyncio.run` with `await config.open()` then `await config.close()`, as in `test_config.py:490-510`.
  - Refusal, sync: `test_config_refuses_event_data_string_processed`, `..._string_verbose`, `..._integer` (`1`) and `..._none`. Each uses `pytest.raises(TypeError, match=r"xcl\.EventDataLevel")` and asserts `"CreateConfig" not in transport.methods_called()`.
  - Refusal, aio: the same four as `test_aio_config_refuses_event_data_...`, constructing `xcl.aio.Config(..., client=Client(transport))` outside an event loop and asserting no calls were made.
  - Keep every one separate: no parametrize, and positive and negative cases never share a function.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution. Run `python -m pytest tests/unit` (no engine needed), then the integration suite where an engine is built (`make engine plugins`).

### Task: Prove each level against the engine

**File changes**:
- `xcl-python:tests/integration/test_sync.py` (near `test_event_entity_hydrates`, `test_sync.py:627`) — add these tests. Each applies `WEB` (`test_sync.py:44-48`) with `registry()` and `on_event=events.append`.
  - `test_raw_event_data_carries_the_resource`: with `event_data=xcl.EventDataLevel.RAW`, select the `sync_app.web` events with `operation == "create"` and assert there is at least one. Assert each carries non-empty `data` whose JSON has `meta.id == "sync_app.web"` and `image == "nginx"`. Assert on parsed fields, not substrings: see knowledge `xcl:gotchas/event-data-includes-temp-paths.md`.
  - `test_event_data_left_out_carries_nothing`: with no `event_data`, assert `all(e.data == b"" for e in events)` and that `events` is non-empty.
  - `test_none_event_data_carries_nothing`: the same with `event_data=xcl.EventDataLevel.NONE`.
- `xcl-python:tests/integration/test_sync.py:627-640` (`test_event_entity_hydrates`) and `xcl-python:tests/integration/test_aio.py:274-292` (`test_async_event_entity_is_configs_class`) — already switched to the member by the previous task. They carry "processed, sync and async: the create success event's entity is the created resource". Check that they still assert `entity.meta.id` and the class, and change nothing else.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution. Needs `make engine plugins` against the `xcl-ref.txt` pin (or `XCL_SRC`).

### Task: Declare the package typed and check the option with mypy

**File changes**:
- `xcl-python:xcl/py.typed` — new, empty file (PEP 561). `pyproject.toml` already packages it: the wheel uses `packages = ["xcl"]` and the sdist `only-include = ["xcl", ...]`. Confirm with `python -m build` or by listing the wheel's contents.
- `xcl-python:pyproject.toml:39-41` — add `"mypy>=1.11"` to the `test` extra, with a comment: mypy is used only by `tests/unit/test_typing.py`, to check that a user's type checker sees the package's annotations, such as `event_data` taking `xcl.EventDataLevel`.
- `xcl-python:tests/unit/test_typing.py` — new; the module docstring says it runs mypy, as a user would, over programs written to a temporary directory against the installed package. Model the subprocess on `tests/unit/test_import.py:24-33`, using `[sys.executable, "-m", "mypy", "--no-incremental", "--no-error-summary", "--hide-error-context", str(path)]` with `cwd=tmp_path` and no `MYPYPATH`, so `xcl` resolves as the installed (editable) package, as it does for users.
  - `test_mypy_flags_string_event_data`: the snippet has `xcl.Config(event_data="processed")` on one line and `xcl.aio.Config(event_data="processed")` on another. Assert `returncode == 1` and that the output has an `error:` line for each of those line numbers mentioning `EventDataLevel`.
  - `test_mypy_accepts_event_data_member`: the same snippet with `xcl.EventDataLevel.PROCESSED`. Assert `returncode == 0` and print the output on failure.
  - The snippets only construct the configurations inside a function that is never called, so mypy needs no engine.
  - If mypy cannot resolve `xcl` from the editable install, STOP and ask (research.md Open assumptions). Do not switch to `MYPYPATH`.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution. Install `.[test]` in the venv first so mypy is present.

### Task: Show the enum in the package README and parity list

**File changes**:
- `xcl-python:README.md:230` — `event_data=xcl.EventDataLevel.PROCESSED,  # carry the entity on events, masked`.
- `xcl-python:README.md:236-240` — where the prose says events have "the same data levels ... a Go program has", name `xcl.EventDataLevel` (`NONE`, the default, `RAW`, `PROCESSED`).
- `xcl-python:README.md:347` — before or after "Migrating from xcl.Resource and @xcl.entity", add a short section "Migrating from string event data levels". It says `event_data` takes only an `xcl.EventDataLevel` member and that a string, even a correctly spelled one, raises `TypeError`, and that code which passed a word passes the member of the same name, upper-cased (`xcl.EventDataLevel.PROCESSED`). It must not contain a bare-string `event_data` literal: the spec's docs criterion is a search of the README that must find none. The literal before/after example belongs only in the release notes.
- `xcl-python:PARITY.md:66` — replace the row with `` | `WithEventData(level)`, `level` an `xcl.EventDataLevel` (`xcl.EventDataNone`, `xcl.EventDataRaw`, `xcl.EventDataProcessed`) | `event_data=xcl.EventDataLevel.NONE`, `.RAW` or `.PROCESSED` (an `xcl.EventDataLevel`; a string is refused) | ``.
- No test reads these files (project convention); they are reviewed by a person.

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Show the enum on the xcl website pages

**File changes**:
- `xcl-website:src/pages/events.mdx:191` — `event_data=xcl.EventDataLevel.PROCESSED,`.
- `xcl-website:src/pages/events.mdx:202-204` — the prose becomes: In Python the options are `event_data=` with an `xcl.EventDataLevel` (`NONE`, `RAW` or `PROCESSED`), `event_mask=` with one of the `xcl.mask` built-ins, and `no_event_mask=True`.
- `xcl-website:src/pages/configuration-text.mdx:103` — "...when the Config was created with `event_data=xcl.EventDataLevel.PROCESSED`".
- `xcl-website:src/pages/examples/configuration-only.mdx:652` — `event_data=xcl.EventDataLevel.PROCESSED,`. The comment above it stays.
- `xcl-website:src/pages/examples/plugins.mdx:1136` — `event_data=xcl.EventDataLevel.PROCESSED,`.
- Re-run `grep -rn 'event_data' src` before finishing, to catch any usage added since planning.
- Build the site (`npm run build` or the repo's `make` target) to confirm the pages render.

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Release xcl-python with the breaking change in its notes

**What the person does**:
1. Open or confirm the xcl-python pull request carrying the Milestone 1 and README/parity tasks, and give it the `minor` release label (`.github/workflows/release-label.yml` requires exactly one). Pre-1.0, a breaking change bumps the minor version (knowledge `xcl:learnings/releases-tagged-by-hand.md`). The PR title should say string event data levels are refused, because generated notes list PR titles.
2. Merge it. `tag.yml` pushes the next minor tag, and `release.yml` publishes to PyPI and creates the GitHub release with generated notes (`release.yml:388-407`). Check the actual tag with `git tag`: the latest at planning time was `v0.13.0`, so `v0.14.0` is expected.
3. Edit that GitHub release (`gh release edit <tag> --notes-file ...` or the web UI). Add, above the generated list, a line such as: "**Breaking:** `event_data` no longer accepts a string; pass an `xcl.EventDataLevel` member. `event_data="processed"` becomes `event_data=xcl.EventDataLevel.PROCESSED`."

**How it is checked**: the release page for the new tag shows the breaking line with the example, and PyPI lists the new version.

## Testing Strategy

- **Add the EventDataLevel enum and accept only its members**: unit tests against `FakeTransport` in `tests/unit/test_config.py` and `tests/unit/test_events.py`. They cover the members and values, the wire value per member and the default for both flavours, and the four refused values for both flavours (`TypeError` naming `xcl.EventDataLevel`, no `CreateConfig` call). The existing processed-level integration tests are switched to the member and must still pass. `tests/unit/test_docstrings.py` must still pass with the new public name.
- **Prove each level against the engine**: integration tests in `tests/integration/test_sync.py` for raw (create events carry the resource's data, checked on parsed JSON fields), left out and none (no event carries data). Processed for sync and aio is carried by `test_event_entity_hydrates` and `test_async_event_entity_is_configs_class`.
- **Declare the package typed and check the option with mypy**: `tests/unit/test_typing.py` runs mypy in a subprocess from a temporary directory against the installed package. The string snippet is flagged on both lines; the member snippet is clean.
- **Show the enum in the package README and parity list** and **Show the enum on the xcl website pages**: no automated tests (project convention forbids tests that read repository files). The website build must succeed.
- **Release xcl-python with the breaking change in its notes**: checked by a person on the release page.
- Manual checks for the implementation test plan: pyright over the two snippets; review of the README, website pages and migration note; review of the parity row; the release-notes line on the published release.
- No success metrics are defined by the spec.

## Project References

- Spec: `20261023074223-e06f835f-event-data-level-enum` (epic `20261023074219-4357d314-printing-and-pretty-output`).
- Design documents: none referenced by the spec.
- Knowledge: `xcl:conventions/testing-and-mocking.md`, `xcl:conventions/dependencies.md`, `xcl:learnings/releases-tagged-by-hand.md`, `xcl:gotchas/event-data-includes-temp-paths.md`.
- Repo roots: xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`, xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`, xcl `/home/nicj/code/github.com/jumppad-labs/xcl` (reference only, unchanged).
- Downstream spec: `20261023075328-741138ee-python-prettylog` uses `xcl.EventDataLevel.PROCESSED`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Every agent task here is Low complexity. The first task must land before the others. After it, the engine-level tests, the typing task and the two documentation tasks are independent and can run in parallel.

## Migration Notes

This is a breaking change for Python users, with no deprecation period. Code passing `event_data="processed"` (or `"raw"`, `"none"`) must pass `xcl.EventDataLevel.PROCESSED` (or `.RAW`, `.NONE`). Code that left the option out is unaffected. The release is a minor bump (pre-1.0). The README carries a migration note, and the release notes carry the breaking line with the example. `xcl.config.EVENT_DATA_LEVELS` is removed; it was never part of the documented API.

## Performance Considerations

None. The check is one `isinstance` and a dict lookup at configuration creation. The new mypy tests add several seconds to the unit suite.
