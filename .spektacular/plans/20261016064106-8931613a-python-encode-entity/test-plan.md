---
created_date: "2026-10-16"
document_status: final
closed_date: "2026-10-16"
---

# Test plan: 20261016064106-8931613a-python-encode-entity

## Success metrics

The spec's one success metric, byte-identical text from Python and Go for every parity fixture under every combination of the four encoding choices, for both a looked-up entity and its saved record, is covered by an automated behavioural test. xcl-python's parity suite (`tests/parity/test_parity.py`, one `test_parity_<fixture>` per fixture, including `test_parity_encode`) compares the `encode` section that xcl's `bindings/libxcl/cmd/parity` program writes with the one `tests/parity/run_python.py` writes. It fails on the first differing text. No manual measurement is needed.

## Manual reviews

### 1. xcl-python README, PARITY.md and docstrings

- **Who / when:** a maintainer, reviewing the xcl-python `python-encode-entity` pull request before it merges.
- **Where:**
  - `xcl-python/README.md`: the "Encoding entities as configuration" section under Lookups, the lookups list, and the note on looked-up `Sensitive` values in "Entity types".
  - `xcl-python/PARITY.md`: the Lookups table, the paragraph after it, and "Not applicable".
  - Docstrings: `Config.encode_entity` and `Config.encode_saved_entity` in `xcl-python/xcl/config.py`, and their twins in `xcl-python/xcl/aio.py`. Run `.venv/bin/python -c "import xcl; help(xcl.Config.encode_entity)"` to read them as a user does.
- **What to look for:**
  - The README shows `encode_entity` with an example.
  - It names `include_computed`, `include_empty`, `reveal_sensitive`, `show_references` and `highlight`, all off by default.
  - It says reveal shows a looked-up entity's configured value, and that a kept event entity shows the engine's current value, unlike Go.
  - It says `encode_saved_entity` takes the same keywords, and lists the errors and the asyncio form.
  - PARITY.md lists `EncodeEntity` and `EncodeSavedEntity` with their Python forms, sync and asyncio, maps each Go option to its keyword, and no longer lists `EncodeEntity` under "Not applicable".
  - The docstrings name every choice and the errors.
- **Pass:** every point above holds, and nothing claims Python cannot encode a live entity.

### 2. xcl.dev configuration-text guide and events page

- **Who / when:** a maintainer, before the xcl-website `python-encode-entity` branch is merged, after the xcl-python release that ships `encode_entity`.
- **Where:** run `make dev` in `xcl-website`, then open `/configuration-text/` and `/events/`. Read each Python tab: Converting an entity, Converting saved data, the event entity, Provider-filled values, Empty attributes, Sensitive values, Showing references as written, and both terminal highlighting tabs. On `/events/`, read the paragraph on masked event data.
- **What to look for:**
  - Each Python tab shows the working `cfg.encode_entity` call with the right keyword: `include_computed=True`, `include_empty=True`, `reveal_sensitive=True`, `show_references=True` and `highlight=True`.
  - `cfg.encode_saved_entity` is shown taking the same choices.
  - The pages say that an event entity kept and encoded after its operation reveals the engine's current value, unlike Go.
  - No "not yet available from Python" note remains for converting an entity, provider-filled values or references.
  - The editor-theme and renderer notes still say Python offers `highlight=True` only.
  - The events page no longer says Python must go through the saved-record call.
- **Pass:** every point above holds, and `make build` and `make check` succeed.

### 3. Lookups section of the shared-library design

- **Who / when:** a maintainer, once, after this spec closes.
- **Where:** `spektacular design read --data '{"source":"design","path":"libxcl.md"}'`, in the Threading list, the `service CoreService` block and the Lookups section.
- **What to look for:**
  - `EncodeEntity` is among the lookups refused during an operation.
  - The service block lists `rpc EncodeEntity(EncodeEntityRequest) returns (EncodeEntityResponse);`.
  - The Lookups protobuf shows `EncodeOptions`, `options = 3` on `EncodeSavedEntityRequest`, and `EncodeEntityRequest` and `EncodeEntityResponse`.
  - The paragraph after it explains that `EncodeEntity` fills marker values and empty lists or maps from the engine's copy found by `meta.id`, writes a host's real values as they are, and colours after rendering.
  - The header carries no `spec:` field.
- **Pass:** every point above holds.

### 4. xcl CHANGELOG entry and the godoc of the new method

- **Who / when:** a maintainer. xcl v0.11.0 is already released, so this is a post-release check.
- **Where:**
  - The `## 20261016064106-8931613a-python-encode-entity` entry at the top of `xcl/CHANGELOG.md`.
  - The godoc of `Config.EncodeEntityRecord`: run `go doc github.com/jumppad-labs/xcl Config.EncodeEntityRecord` in `xcl`.
- **What to look for:**
  - The entry describes `EncodeEntity`, `EncodeOptions`, the saved-record choices with "no options renders as before", `Config.EncodeEntityRecord`, the fill from the engine's copy (sensitive values and empty lists or maps), the event-entity difference, and ends with `**Breaking:**` and `None.`
  - The godoc states the record shape, what is filled and when, that a missing or unknown `meta.id` is not filled, and the errors.
- **Pass:** every point above holds.
