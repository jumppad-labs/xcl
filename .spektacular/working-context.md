# Working context: 20261006112108-17623cda-encoder-syntax-highlighting

Epic: `20261006071139-7b266535-examples-and-output` (siblings: e2e suite, configuration-example,
docker-plugin-example). Source: https://github.com/jumppad-labs/xcl/issues/6 (no comments).
User instruction: "keep going until the epic is done" — finish specifying this last spec with
minimal pauses.

## Background

- `example/prettylog/highlight.go` colours encoder output per token category matching the
  xcl-vscode TextMate grammar (block type bold magenta, type label cyan, name label green,
  attribute blue, strings yellow, numbers/constants bright magenta, reference root cyan,
  comments dim italic). It uses lipgloss with a renderer bound to the writer, so non-terminals
  get plain text. Regex line-based, relies on encoder formatting.
- User: "I think we should add this as a real feature to the encoder but not now" and later
  "Once we add colorization to encode saved entities then we can remove most of that example code."
- Library must not depend on charmbracelet (`TestLibraryDoesNotDependOnCharm`).
- Related: issue #5 (xcl fmt) could share a tokeniser.

## Decisions after review (2026-10-06)
- Invalid/unreadable theme -> error at renderer creation; unmatched token -> terminal default
  colour; theme font styles applied. Added constraint: existing encode output unchanged.
- Spec written to store.

# Plan-epic run (2026-10-06)

Orchestrating `spek-plan-epic` for epic `20261006071139-7b266535-examples-and-output`, root
`/home/nicj/code/github.com/jumppad-labs/xcl`.
- Started: 506b8289-e2e-suite-and-real-world-examples, 17623cda-encoder-syntax-highlighting.
- Blocked on e2e suite: aadf3c10-configuration-example, f7a185dc-docker-plugin-example.
- DONE: 506b8289 e2e suite (summary kept in orchestrator scratchpad e2e-done.md).
- Started: aadf3c10-configuration-example, f7a185dc-docker-plugin-example.
- DONE: 17623cda encoder highlighting (summary in scratchpad highlight-done.md).
- DONE: aadf3c10 configuration example (summary in scratchpad configonly-done.md).
- DONE: f7a185dc docker plugin example (summary in scratchpad docker-done.md). All 4 planned.
- Step 6: 2 cross-plan disagreements put to user (state-masking CTA ownership; highlighting changelog content test).
- Decisions settled: (1) config plan owns state-masking CTA, docker drops its edit — applied to plans. (2) NO tests that inspect repo files (docs, CI yaml, go.mod, source scans) — user. Deleted readme_test.go, ci_workflow_test.go, static_output_test.go, scan test in query_migration_test.go. Plans e2e/config/docker stripped of changelog content tests. Convention added to conventions/testing-and-mocking.md.
- Removed also state/dependencies_test, state/public_surface_test, events/imports_test, static_dependencies_test (user). Plans updated. Summary written (4 sections + decisions). epic order added 4 deps. Now: end-of-planning review with user.
- User: CHANGELOG.md written once at end of epic (not saved as convention — epic decision only). All 4 plans updated with Changelog input notes; summary rewritten; highlight->configonly unordered.
- Review done: order kept; project-structure convention updated (top-level public pkgs); gotchas saved (example modules/internal, website no redirects). Run complete.
- Questions/answers: see above
