---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Test plan: 20261023074223-e06f835f-event-data-level-enum

The spec defines no post-delivery success metric. Every behavioural criterion is covered by the xcl-python test suite: the unit tests in `tests/unit/test_config.py`, `tests/unit/test_events.py` and `tests/unit/test_typing.py` (mypy), and the integration tests in `tests/integration/test_sync.py` and `tests/integration/test_aio.py`. What follows are the manual checks the plan's Testing Approach lists. Each needs a person.

## 1. pyright flags a string level and accepts the member

**What to check**: pyright, like mypy, flags `event_data="processed"` on both `xcl.Config` and `xcl.aio.Config`, and accepts `xcl.EventDataLevel.PROCESSED`. mypy is automated in `tests/unit/test_typing.py`; pyright is not a test dependency.

**How**:
1. In an xcl-python checkout that holds this change, create a venv and install the package: `uv venv .venv && uv pip install -p .venv/bin/python -e .`.
2. In an empty temporary directory, write `bad.py`:
   ```python
   import xcl
   import xcl.aio


   def build() -> None:
       xcl.Config(event_data="processed")
       xcl.aio.Config(event_data="processed")
   ```
   and `good.py`, the same program with `xcl.EventDataLevel.PROCESSED` in place of both strings.
3. From that directory run `npx -y pyright --pythonpath <checkout>/.venv/bin/python bad.py good.py`.

**Expected result**: exactly two errors, both in `bad.py`, on lines 6 and 7. Each says `"Literal['processed']"` is not assignable to `"EventDataLevel"` (`reportArgumentType`). `good.py` has no errors.

**Who / when**: the person preparing the xcl-python release, before merging its pull request.

## 2. Documentation shows only the enum

**What to check**: every documented example and description of the event data option uses `xcl.EventDataLevel` members. The README's migration note says strings are refused and names the member to pass, with no bare-string code example.

**How**:
1. In xcl-python, run `grep -n 'event_data' README.md` and read each hit. Open the "Variables, state and events" section (the example passes `event_data=xcl.EventDataLevel.PROCESSED`, and the prose names `NONE`, `RAW` and `PROCESSED`). Then open the "Migrating from string event data levels" section.
2. In xcl-website, run `grep -rn 'event_data' src` and read each hit in context: `src/pages/events.mdx` (Python tab and the "In Python the options are" sentence), `src/pages/configuration-text.mdx`, `src/pages/examples/configuration-only.mdx`, `src/pages/examples/plugins.mdx`. Optionally, `npm ci && npm run dev` and view `/events/`, `/configuration-text/`, `/examples/configuration-only/` and `/examples/plugins/`.
3. In both repos, run `grep -rn '"processed"\|"raw"\|"none"' README.md src` to catch any bare-string level outside an `event_data` line.

**Expected result**: no hit passes or describes a bare string level. The migration note states that a string, even a correctly spelled one, raises `TypeError`, and that code passes the upper-cased member of the same name. It has no code block. Go tabs still use `xcl.EventDataProcessed`, which is correct for Go.

**Who / when**: the reviewer of the xcl-python and xcl-website pull requests, before merge.

## 3. Parity row pairs Go's type with the Python enum

**What to check**: the configuration-options row for event data in `PARITY.md` names Go's `WithEventData` and `xcl.EventDataLevel`, and the Python `xcl.EventDataLevel` with its three members.

**How**: in xcl-python, open `PARITY.md` at the "Configuration options" table and read the `WithEventData(level)` row.

**Expected result**: the Go column reads `WithEventData(level)`, `level` an `xcl.EventDataLevel` (`xcl.EventDataNone`, `xcl.EventDataRaw`, `xcl.EventDataProcessed`). The Python column reads `event_data=xcl.EventDataLevel.NONE`, `.RAW` or `.PROCESSED`, and says a string is refused.

**Who / when**: the reviewer of the xcl-python pull request, before merge.

## 4. Release notes carry the breaking change

**What to check**: the GitHub release of the xcl-python version that ships this change says string event data levels are refused, with the migration example. This was descoped from the implement run as work a person does after the merge (see the plan's `**Descoped requirements**:`).

**How**:
1. Give the xcl-python pull request that carries this change the `minor` release label (`.github/workflows/release-label.yml` requires exactly one). Title it so the generated notes name the break, for example "Refuse string event data levels; use xcl.EventDataLevel".
2. Merge it. `tag.yml` pushes the next minor tag and `release.yml` publishes to PyPI and creates the GitHub release with generated notes. Check the real tag with `git fetch --tags && git tag --sort=version:refname | tail -1`. `v0.14.0` is expected if the latest was `v0.13.0`, but confirm.
3. Edit that release (`gh release edit <tag> --notes-file <file>` or the web UI). Add above the generated list: "**Breaking:** `event_data` no longer accepts a string; pass an `xcl.EventDataLevel` member. `event_data="processed"` becomes `event_data=xcl.EventDataLevel.PROCESSED`."

**Expected result**: the release page for the new tag shows that line with the example, and PyPI lists the new minor version of `xcl-lang`.

**Who / when**: a maintainer with release rights, right after the xcl-python pull request merges.
