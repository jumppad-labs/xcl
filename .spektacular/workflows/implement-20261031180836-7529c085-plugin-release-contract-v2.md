# Run notes (full run, started 2026-11-01)
- xcl v0.13.0 is released (merge 27da7d5): it carries the registry, ReplaceOn, the nested-block fix, the XCL_PLUGIN cookie and the command form. Plugin tasks bump to v0.13.0.
- xcl-python engine bump is PR #25, branch engine/v0.13.0, failing as expected: 244 tests fail on `external_plugins` (Registry._to_proto always sends it), and 15 error on the old-layout test plugin v0.2.0. The Python tasks land on that branch.
- PR #40 merged with breaks-bindings, because xcl's bindings check runs xcl-python main against the PR.
- 2026-11-02: user approved pushing the Python work onto xcl-python PR #26 (branch engine/v0.13.1, supersedes #25). The xcl-python worktree's spec branch was fast-forwarded to origin/engine/v0.13.1. Both Python tasks are done there, then pushed to engine/v0.13.1.
- Released pins: test plugin v0.3.1 and Docker v0.4.1 (v0.3.0/v0.4.0 are old-layout, tagged from a stale main). Assets use `-` names.
- Python suite setup in the worktree: .venv inside the worktree; engine, protos and plugins built from `git archive v0.13.1` in the scratchpad (`make protos engine plugins XCL_SRC=<archive> PYTHON=.venv/bin/python`). Run it with GITHUB_TOKEN=$(gh auth token) XCL_TEST_STRICT=1 XCL_SRC=<archive>.
- xcl v0.13.1 already has external_plugin_commands, so the Python command task can merge with the engine bump.

## Knowledge written

- repo tier, `xcl` store, `gotchas/example-remoteplugin-binary-is-tracked.md`: building `example/remoteplugin` overwrites a committed binary, which then leaks into commits and goal-check diffs.
