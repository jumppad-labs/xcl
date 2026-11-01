# Run notes (full run, started 2026-11-01)
- xcl v0.13.0 is released (merge 27da7d5): it carries the registry, ReplaceOn, the nested-block fix, the XCL_PLUGIN cookie and the command form. Plugin tasks bump to v0.13.0.
- xcl-python engine bump is PR #25, branch engine/v0.13.0, failing as expected: 244 tests fail on `external_plugins` (Registry._to_proto always sends it), and 15 error on the old-layout test plugin v0.2.0. The Python tasks land on that branch.
- PR #40 merged with breaks-bindings, because xcl's bindings check runs xcl-python main against the PR.

## Knowledge written

- repo tier, `xcl` store, `gotchas/example-remoteplugin-binary-is-tracked.md`: building `example/remoteplugin` overwrites a committed binary, which then leaks into commits and goal-check diffs.
