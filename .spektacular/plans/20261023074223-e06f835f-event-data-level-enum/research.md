---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Research: 20261023074223-e06f835f-event-data-level-enum

## Alternatives considered and rejected

### Option A: `enum.Enum` with the protocol integers as values

An enum whose members carry the protocol numbers, for example `EventDataLevel.PROCESSED = core_pb2.EVENT_DATA_PROCESSED`.

**Rejected**: the spec's Technical Approach prefers string values that are today's wire words, so reprs and logs stay readable. Integer values would also leak the protocol's numbering into the public API. `xcl-python:xcl/_core/core_pb2.pyi:10-17` shows the protocol already has its own enum, so the mapping can stay internal.

### Option B: Re-export the generated protobuf enum

Expose `core_pb2.EventDataLevel` as `xcl.EventDataLevel`.

**Rejected**: the generated class is an `int` wrapper with `EVENT_DATA_*` names, not a Python `enum`. It fails the "an enum exported from the top level" constraint, and its members do not read like Go's (`xcl:events.go:23-41`).

### Option C: Accept both the enum and the bare word for a deprecation period

Keep accepting strings with a warning while users move to the enum.

**Rejected**: the spec's Constraints refuse plain strings with no fallback and no deprecation period.

### Option D: Refuse with `ValueError`

Keep today's exception type (`xcl-python:xcl/config.py:75-76`, `xcl-python:xcl/aio.py:73-74`).

**Rejected**: the spec's Technical Approach prefers `TypeError`, the Python convention for an argument of the wrong type. Every refused value (a str, an int, None) is the wrong type, not a bad value of the right type.

### Option E: Refuse with an `xcl.Error` subclass that is also a `TypeError`

A new error type built like `SchemaError(Error, TypeError)` (`xcl-python:xcl/_schema.py:33`).

**Rejected**: `xcl.Error` is documented as "an error reported by the xcl engine" with a stable `code` (`xcl-python:xcl/_errors.py:25-31`). The configuration's other argument checks raise plain built-ins (`xcl-python:xcl/config.py:73-74`). A plain `TypeError` matches them.

### Option F: Check membership with `in` or a dict lookup keyed by the enum

Validate with `value in mapping`.

**Rejected**: a `StrEnum` member hashes and compares equal to its string, so `"processed" in {EventDataLevel.PROCESSED: ...}` is True and a bare word would slip through. The check must be `isinstance(value, EventDataLevel)`.

### Option G: Prove the type-checker criterion by hand only

Leave the mypy/pyright criterion entirely to manual review.

**Rejected**: the criterion is binary and can be automated by running mypy over snippets in a subprocess. That tests the package's typing surface as a user meets it, rather than reading repository files. Only pyright stays manual, because it would need Node as a test dependency.

### Option H: Generate the release-note line in `release.yml`

Make the release workflow append a notes file to `gh release create`.

**Rejected**: it changes the shared release flow for one spec. xcl-python's notes are generated from PR titles (`xcl-python:.github/workflows/release.yml:388-407`), and a person adding the breaking line to the published release is the established way to add prose.

## Chosen approach — evidence

- `xcl:events.go:21-41` — Go's type is `xcl.EventDataLevel` with `EventDataNone` (default), `EventDataRaw`, `EventDataProcessed`; Python mirrors it as `xcl.EventDataLevel.NONE/RAW/PROCESSED`, and the member docstrings can follow Go's comments.
- `xcl:options.go:144` — `WithEventData(level EventDataLevel)`, the parity row's Go side.
- `xcl-python:xcl/config.py:20-24` — `EVENT_DATA_LEVELS` maps the three bare words to `core_pb2.EVENT_DATA_*`; it becomes an internal enum-to-wire mapping.
- `xcl-python:xcl/config.py:56,75-76,95,104` and `xcl-python:xcl/aio.py:29,53,73-74` — the only places the option is typed, validated and encoded; `aio.Config` reuses `config.Config._request` (`aio.py:76-85`), so one shared validator covers both.
- `xcl-python:xcl/aio.py:59-69` — `aio.Config.__init__` creates a `ThreadPoolExecutor` and resolves the client before validating; validating first means a refused value leaks nothing and builds no client. `aio.Config` creates its engine configuration lazily (`aio.py:89-92`), so a refusal in `__init__` creates none.
- `xcl-python:xcl/config.py:62-87` — sync `Config` calls `CreateConfig` at the end of `__init__`; validating first guarantees no configuration is created.
- `xcl-python:pyproject.toml:11` — `requires-python = ">=3.11"`, so `enum.StrEnum` (3.11+) is available.
- `xcl-python:xcl/` has no `py.typed` marker, so mypy treats the installed package as untyped and would not flag a bare string; adding `xcl/py.typed` is required for the type-checker requirement. `pyproject.toml` `[tool.hatch.build.targets.wheel] packages = ["xcl"]` and the sdist's `only-include = ["xcl", ...]` both pick the marker up with no build change.
- `xcl-python:.github/workflows/test.yml` "Install" step — CI installs `.[test]`, so adding `mypy` to the `test` extra makes the typing test run everywhere the suite runs.
- `xcl-python:tests/unit/test_config.py:17-28` — FakeTransport pattern: `transport.methods_called()` and `transport.calls[-1]` give the serialised `CreateConfigRequest`; the model for wire-mapping and "no configuration created" tests. `test_config.py:490-510` is the aio pattern (`asyncio.run`, `await config.close()`).
- `xcl-python:tests/integration/test_sync.py:627-640` (`test_event_entity_hydrates`) and `xcl-python:tests/integration/test_aio.py:274-292` (`test_async_event_entity_is_configs_class`) — existing processed-level tests asserting the create event's entity is the created resource; they switch to the enum and carry the "processed, sync and async" criterion.
- `xcl-python:tests/unit/test_docstrings.py` — walks `xcl.__all__`; the new class needs a docstring.
- `xcl-python:xcl/_schema.py:33` / `tests/unit/test_registry.py:135-145` — precedent for an error that is a `TypeError`.

## Files examined

- `xcl-python:xcl/config.py:20-24` — `EVENT_DATA_LEVELS` bare-word map.
- `xcl-python:xcl/config.py:27-45` — `Config` docstring describes `event_data` as `"none"`, `"raw"` or `"processed"`.
- `xcl-python:xcl/config.py:47-117` — `__init__` signature, validation, `_request` builder.
- `xcl-python:xcl/aio.py:29` — imports `EVENT_DATA_LEVELS` from `xcl.config`.
- `xcl-python:xcl/aio.py:36-85` — `aio.Config` signature and validation; reuses `_SyncConfig._request`.
- `xcl-python:xcl/__init__.py:12-115` — top-level imports and `__all__`; the enum is added here.
- `xcl-python:xcl/__init__.py:125-149` — `_REMOVED` mechanism for removed names (not needed: a value change, not a removed name).
- `xcl-python:xcl/events.py:1-40` — `Event`; natural home for the enum, mirroring Go's `events.DataLevel`.
- `xcl-python:xcl/_core/core_pb2.pyi:10-17,66,70` — protocol enum `EventDataLevel`, `CreateConfigRequest.event_data`.
- `xcl-python:xcl/_errors.py:25-31,259-262` — `xcl.Error` is engine errors; not used for argument checks.
- `xcl-python:xcl/_schema.py:33-38` — `SchemaError(Error, TypeError)`.
- `xcl-python:pyproject.toml` — Python >=3.11, `test` extra only `pytest>=8`, wheel/sdist include rules.
- `xcl-python:Makefile` — `test`, `test-unit` targets.
- `xcl-python:.github/workflows/test.yml` — installs `.[test]`, runs `pytest -q -rs` strict on 3.11–3.x, five OSes, Alpine.
- `xcl-python:.github/workflows/release.yml:18-19,388-407` — GitHub release notes = engine line + `--generate-notes` PR titles since previous tag.
- `xcl-python:.github/workflows/tag.yml`, `release-label.yml` — release label (major/minor/patch/no-release) on every PR drives the tag.
- `xcl-python:xcl/_version.py` — `0.5.0.dev0`; tags go to `v0.13.0` (latest), so the shipping version is expected to be `v0.14.0` with a `minor` label.
- `xcl-python:README.md:221-240` — "Variables, state and events" example uses `event_data="processed"`.
- `xcl-python:README.md` "Migrating from xcl.Resource and @xcl.entity" — precedent for a migration section.
- `xcl-python:PARITY.md:58-71` — "Configuration options" table; `WithEventData(level)` row pairs with bare strings.
- `xcl-python:tests/unit/test_config.py` — FakeTransport-based unit tests, no parametrize.
- `xcl-python:tests/unit/test_docstrings.py` — docstring coverage over `xcl.__all__`.
- `xcl-python:tests/unit/test_import.py` — subprocess-from-`ROOT` pattern for a fresh interpreter.
- `xcl-python:tests/integration/test_sync.py:319-330,627-640,650-660,1040` — processed-level usages.
- `xcl-python:tests/integration/test_aio.py:283,631` — processed-level usages.
- `xcl-python:tests/integration/test_test_plugin.py:246` — processed-level usage.
- `xcl-python:tests/integration/sync_entities.py:43-101` — `SyncApp`, `registry()`.
- `xcl:events.go:21-41` — Go `EventDataLevel` and its constants' doc comments.
- `xcl:options.go:144` — `WithEventData`.
- `xcl:CHANGELOG.md` — per-spec entries for specs with xcl changes; xcl-python has no changelog file.
- `xcl-website:src/pages/events.mdx:185-204` — Python tab `event_data="processed"` and prose listing the bare words.
- `xcl-website:src/pages/configuration-text.mdx:103` — prose `event_data="processed"`.
- `xcl-website:src/pages/examples/configuration-only.mdx:640-655` — Python example `event_data="processed"`.
- `xcl-website:src/pages/examples/plugins.mdx:1125-1137` — Python example `event_data="processed"`.

## External references

- Python `enum.StrEnum` (docs.python.org/3/library/enum.html#enum.StrEnum, new in 3.11) — members are `str` instances that compare and hash equal to their values; the reason the refusal must check type, not equality.
- PEP 561 (peps.python.org/pep-0561) — a package must ship `py.typed` for type checkers to use its inline annotations; the reason `xcl/py.typed` is added.
- mypy docs, "Running mypy and managing imports" — mypy finds installed packages through site-packages (including path-based `.pth` editable installs) and skips untyped ones.

## Prior plans / specs consulted

- `20261016064106-8931613a-python-encode-entity` (plan) — xcl-python + website delivery pattern; docs, PARITY.md and the website are checked by human review, never by tests; release is done by merging with a release label.
- `20261011232013-da18c586-python-release-hardening` and `20261011211634-a8895873-python-out-of-process-engine` (plans) — "one CHANGELOG entry per spec in xcl's CHANGELOG.md" applied to specs that change xcl; no test reads the changelog.
- Knowledge `xcl:learnings/releases-tagged-by-hand.md` — check the latest tag rather than assuming a number; pre-1.0 breaking changes bump the minor version; cutting a release is a task for a person unless a workflow does it (here the tag workflow does it on merge with a label).
- Knowledge `xcl:conventions/testing-and-mocking.md` — no table-driven tests, no mixing positive and negative, no tests that read README/CHANGELOG/website.

## Open assumptions

- Verified during planning (2026-10-23): mypy, run from a temporary directory, resolves the project venv's path-based editable install (`_editable_impl_xcl_lang.pth` holds the repo root). Without `py.typed` it reports `import-untyped`. With `py.typed` and the enum annotation, in a scratch copy, it reports `arg-type ... expected "EventDataLevel"` on each bad line for `xcl.Config` and `xcl.aio.Config`, and "Success" for the member snippet, even with protobuf absent. If CI's install behaves differently, the implement workflow must STOP and ask rather than point mypy at the source tree with `MYPYPATH`.
- The next xcl-python release after this merges is a minor bump (expected `v0.14.0`); the person editing the release notes checks the actual tag.
- No other code path (parity runner `tests/parity/run_python.py`, `.github/scripts/install_check.py`) passes `event_data`; grep found none.

## Drafting assumptions

### Chosen direction: StrEnum in xcl/events.py with one shared type-checking validator (architecture)
- **Decision**: `xcl.EventDataLevel(enum.StrEnum)` with NONE/RAW/PROCESSED = "none"/"raw"/"processed", declared in `xcl/events.py` and exported from `xcl/__init__.py`. A private helper in `xcl/config.py` refuses non-members with an `isinstance` check and maps members to the protocol enum, and both `Config.__init__` and `aio.Config.__init__` call it first. `event_data: EventDataLevel = EventDataLevel.NONE`. `xcl/py.typed` is added.
- **Rationale**: follows the spec's Technical Approach (Go's name, string values, TypeError). Placing the enum beside `Event` mirrors Go's `events.DataLevel`. Validating first means a refusal creates no configuration and starts no executor or client.
- **Rejected**: a plain Enum with protocol integers (unreadable reprs, leaks wire numbering); re-exporting the protobuf enum (not a Python enum); a Literal-typed str (the spec refuses strings); placing the enum in a new module (Go keeps it with events).

### Conventions applied (architecture)
- **Decision**: apply xcl's Testing & Mocking rules (no table tests, no mixed positive/negative tests, verbosity, no repository-file tests, tests located with source) and the Dependencies rule (stdlib first, document third-party). Drop the Go-only conventions (code style, project structure, errors package, testutil, DAG, state fixtures, parser builder, database, logging, patterns).
- **Rationale**: the xcl-python and xcl-website stores have no conventions of their own. Prior Python plans applied xcl's testing rules to the Python suite. The dropped entries govern Go code, which this plan does not touch.
- **Rejected**: listing every convention (most would not apply here).

### Refusal is a plain TypeError (discovery)
- **Decision**: a refused `event_data` raises a plain built-in `TypeError`, not an `xcl.Error` subclass.
- **Rationale**: the spec prefers `TypeError`; `xcl.Error` is documented as an engine error with a stable code, and the configuration's other argument checks raise plain built-ins.
- **Rejected**: a `SchemaError`-style `xcl.Error` + `TypeError` hybrid (would add a new public error type for one argument check).

### Add py.typed so type checkers read the package (discovery)
- **Decision**: ship an empty `xcl/py.typed` marker.
- **Rationale**: without it mypy treats the installed package as untyped and cannot flag a bare string, so the "type checkers understand the option" requirement cannot hold for users.
- **Rejected**: relying on pyright's library inference only (mypy would still not flag it; the criterion names mypy).

### mypy joins the test extra (discovery)
- **Decision**: add `mypy` to the `test` optional dependency and test the typing criterion by running `python -m mypy` over snippets in a subprocess.
- **Rationale**: the criterion is binary and automatable; CI already installs `.[test]`. The test exercises the package's typing surface, not repository files.
- **Rejected**: a manual-only check (left only for pyright); pyright as a dependency (needs Node).

### Release notes: README migration note plus a person editing the GitHub release (discovery)
- **Decision**: the README gains a short migration note, worded without a bare-string literal so the docs criterion's search stays clean; a person adds the breaking line and the one-line migration example to the generated GitHub release notes of the xcl-python release that ships the change, and the PR carries the `minor` label. No xcl CHANGELOG entry, because nothing in xcl changes.
- **Rationale**: xcl-python has no changelog file and its release notes are generated from PR titles; editing a published release is outside the repo.
- **Rejected**: changing `release.yml` to append a notes file (changes the shared release flow for one spec); an xcl CHANGELOG entry for a Python-only change.

### Rename the internal mapping (discovery)
- **Decision**: `xcl.config.EVENT_DATA_LEVELS` becomes a private enum-to-wire mapping with a shared validator; the old name is not kept.
- **Rationale**: `xcl.config` is not part of the documented API (`xcl.__all__` exports `Config` only); keeping a bare-word map public would invite the forms the spec refuses.
- **Rejected**: keeping `EVENT_DATA_LEVELS` keyed by the enum (a StrEnum key still matches its string on lookup).

### Six tasks in two milestones (tasks)
- **Decision**: three xcl-python code tasks (enum and refusal with unit tests, engine-backed level tests, typed marker with mypy test), one xcl-python docs task, one xcl-website docs task, and one human release task.
- **Rationale**: the enum change and its existing-test updates must land together (bare strings break), while engine tests, typing and docs are independent once it exists.
- **Rejected**: one large xcl-python task (harder to review); an xcl CHANGELOG task (nothing in xcl changes).

### Raw-level parity with the old string is proved through the wire value (tasks)
- **Decision**: "matches what the string 'raw' delivered before" is covered by a unit test that `RAW` sends `EVENT_DATA_RAW`, the value the string mapped to, plus an integration test that raw delivers resource data on create events.
- **Rationale**: the old string form can no longer be run after the change, so a before/after comparison in one suite is impossible; the wire value is the full contract with the engine.
- **Rejected**: a recorded fixture of the old raw payload (brittle, depends on temp paths in `meta.file`).

### Other annotations exposed by py.typed are not audited (out_of_scope)
- **Decision**: shipping `py.typed` is in scope; auditing every other public annotation is not.
- **Rationale**: the spec asks only that the event data option is understood by type checkers; a full typing audit is a separate piece of work.
- **Rejected**: a repo-wide mypy pass as part of this plan (unbounded scope).

## Rehydration cues

- `spektacular spec file read 20261023074223-e06f835f-event-data-level-enum`
- `spektacular repo list` (xcl-python root `/home/nicj/code/github.com/jumppad-labs/xcl-python`, xcl-website root `/home/nicj/code/github.com/jumppad-labs/xcl-website`, xcl root `/home/nicj/code/github.com/jumppad-labs/xcl`)
- `spektacular knowledge always-applied --tier repo --filter xcl --filter xcl-python --filter xcl-website`
- Re-read `xcl-python:xcl/config.py:1-117`, `xcl-python:xcl/aio.py:1-92`, `xcl-python:xcl/__init__.py`, `xcl:events.go:21-41`.
- `grep -rn 'event_data' xcl-python xcl-website/src` to find every usage.
