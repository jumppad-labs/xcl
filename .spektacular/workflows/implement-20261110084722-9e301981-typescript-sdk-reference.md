# Working context: 20261110084722-9e301981-typescript-sdk-reference (orchestrated)

## Decisions and substitutions
- read_plan: structure, drift and spec coverage pass. Plan paths all exist in the worktrees.
- Differences from the plan (trust code): `new Config({...})` not `createConfig`; public `engineVersion()`;
  platform packages `@jumppad-labs/xcl-lang-<platform>`; release.yml settings outputs tag/version/engine/engine-version.
- xcl-website already has `reference-check.yml` (strict build, `XCL_REFERENCE_STRICT=1`) from the Python reference;
  PyRef is lenient like the planned TsRef.
- Python pin `xcl-python-ref.txt` is the placeholder v0.14.1 with no asset: website builds need
  `XCL_PYTHON_REFERENCE=<local json>` (generate with xcl-python's `.github/scripts/api_reference.py --out`). Never change it.
- Python counterparts to mirror: xcl-python `.github/scripts/api_reference.py`, release.yml `reference`/`github-release`/
  `dispatch-website` jobs, test.yml `check-docs` job, Makefile `check-docs`.
- Use a short TMPDIR (e.g. /tmp/x) for anything starting real plugins. Docker is Podman.
- Staging under `.spektacular/tmp/20261110084722-9e301981-typescript-sdk-reference/` only.

## Progress and learnings
- Task 1 (API reference tool) done. Tool: xcl-typescript `.github/scripts/api-reference.mjs` (--check/--out/--source),
  tests `.github/scripts/api-reference.test.mjs` (node --test). Package already has every doc comment; task 2 adds
  @param/@returns/@throws {@link X} tags only.
- Knowledge to write at the end (xcl-typescript gotcha typedoc-throws-and-signature-text): merged value+type names share
  one TS symbol, pick the declaration by kind; Application.bootstrap default readers read a typedoc options file from cwd.
- Helper scripts in .spektacular/tmp/<spec>/: stage.sh (goal-check staging), ticktask.sh <id> <criteria>, addlog.sh (entry.md).
