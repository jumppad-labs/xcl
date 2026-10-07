---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Test plan: 20261007105731-2388b579-diff

All three success metrics have automated behavioural tests. The parity metric is checked against the revised contract (see the plan changelog). The tests are:
- `e2e/diff_test.go` `requireDiffPredictsApply`, run in every diff-vs-apply scenario.
- The state-bytes and no-create/update/destroy-event checks in every e2e diff.
- The leak tests in `e2e/diff_test.go` and `sensitive_leak_test.go`.

The plan lists two manual reviews. They follow.

## Review 1: diff terminology, never "plan"

- **Who / when**: a maintainer, before this branch merges.
- **What to look at**: every new public name, every doc comment and every error message this feature added:
  - `diff/diff.go`, `diff/path.go` and `diff/options.go`;
  - `config_diff.go`;
  - the `OperationDiff` comment in `events/events.go`;
  - `Parser.Diff` in `internal/parser/parser.go`;
  - in `internal/parser/lifecycle.go`: `diff`, `diffResource`, `recordPending`, `changes` and `decodeCopy`;
  - `internal/parser/diff_recorder.go`, `diff_changes.go`, `diff_unknown.go` and `diff_decode.go`.
- **How**:
  - Run `go doc -all ./diff` and `go doc github.com/jumppad-labs/xcl Config.Diff`, and read the output.
  - Run `git diff main -- diff config_diff.go events internal/parser/*.go ':!*_test.go' | grep -in '\bplan'`.
- **Pass**: no new identifier, comment or message uses "plan" to mean a diff. Every one says "diff". Pre-existing uses of "plan" that have nothing to do with this feature do not count.

## Review 2: godoc of the `diff` package against the design

- **Who / when**: a maintainer, before the sibling rendering spec (20261007111826-cf3b66d8-diff-rendering-and-docs) starts.
- **What to look at**: the output of `go doc -all ./diff`, side by side with the design `config-diff.md` (`spektacular design read --data '{"source":"design","path":"config-diff.md"}'`). Read the Actions, Changes, Unknown values, Computed values of updated resources, Sensitive values, Types and JSON format sections.
- **Check**:
  - The four actions and when each applies. `ActionUpdate`'s comment should cover a provider-reported change and a dependency on an unknown value. Dependency on an unknown value includes a computed value of an updated resource.
  - The path forms in `Path.String`: `image`, `ports[0].host`, `env["LOG_LEVEL"]`.
  - When `Change` fields are present or absent:
    - `before` is absent for an added value or an unrevealed sensitive one;
    - `after` is absent for a removed, unknown or unrevealed sensitive value;
    - `changes` is omitted when there are none.
  - That `Changed()` is documented as create + update + replace + delete.
  - That `RevealSensitive` is the only option.
- **Pass**: every point in the godoc agrees with the revised design, and nothing the rendering spec needs is missing or contradicted. Note any gap as a follow-up for the rendering spec, not as a blocker.

## Environment note

`TestPluginExampleTestsPass` (e2e) and the `example/plugin` tests fail on the implementation host. A Docker network named `app` already exists there. This is unrelated to this feature. Run them on a host without that network, or remove it first, to confirm they pass.
