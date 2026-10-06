---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# E2E suite and real-world examples

## What was built

xcl now has its own end-to-end test suite in `e2e/`. It drives the library the way an application does, through its public packages only (plus the shared `internal/testutil` helpers), against fixtures it owns:
- a Kubernetes-like configuration and its Go types;
- a plugin-driven configuration with a module, outputs and published values;
- an in-process plugin;
- an external plugin binary, which the suite builds once per run.

Every library behaviour the examples used to test was carried over as a named e2e test with the same assertion: 29 tests from configonly and 38 from plugin. They cover:
- decoding of nested, repeated and omitted blocks;
- references, variables and environment values;
- parse, create, destroy and plugin-load events;
- in-process and external plugin logging;
- published values and module outputs;
- entity encoding and agreement with state;
- masking of state and events, and the plain-state warning;
- silence without an event handler;
- the missing-external-plugin error.

`e2e/COVERAGE.md` maps every removed example test to the test that now covers it. It also lists the 14 source-inspection tests deleted without replacement, with the reason for each.

The examples now test only what they do. The configonly and plugin examples lost their library-behaviour and source-inspection tests and keep their output, exit and smoke tests. prettylog's tests use block types, an in-process plugin and a configuration of their own instead of xcl's internal fixtures.

Each example is now a standalone Go module pointed at the local xcl by a `replace`. Smoke tests build and run the real binary with the standard library. The e2e suite runs each example's tests through one named runner test per example, and the runner is tested against a passing and a failing fixture module. The minimum-Go CI job now builds and vets every example on Go 1.25.0. xcl's own module no longer requires the charmbracelet libraries or `muesli/termenv`. The README explains the new layout.

## Why it matters

The examples used to double as xcl's end-to-end tests, so a change to how an example was written could fail the build, and rewriting an example risked losing coverage. Coverage is now independent of how the examples are written, and the examples can be rewritten as real-world projects (by the sibling configuration and Docker-plugin specs) without losing any. Because each example is self-contained, a reader can copy it out of the repository and build it against a published xcl.

## Deviations from the plan

- **configonly baseline.** At the start, configonly's committed `main.go` did not match its tests. The user chose to restore `main.go` from 8271816. It needed small adaptations to match the current tests, configuration and types:
  - a trailing `stateKey` argument on `run`;
  - registering types under a single name;
  - registering the `secret` type;
  - in the configuration, `secret_key_ref` now reads `secret.db.meta.name`.

  `kr/pretty` was then unused and left the root module.
- **Fixture packages.** The e2e fixture types are split into `kube` and `services` packages, because both sets define an `Ingress`.
- **Shared e2e config helper.** It always requests processed event data, as the examples do.
- **Split tests.** Six example tests checked both library behaviour and printed output, so they were split. The library half moved to e2e, and the printing or rendering half stayed with the example or prettylog. `TestPluginExampleFailsWithoutExternalPlugin` stays in the example, trimmed to the example's own "build it" hint.
- **Event sources.** Expected sources follow the fixture names: `Plugin` and `externalplugin`.
- **Ordering assertions.** None are made between different resources. Only per-resource phase sequences are checked.
- **No library tests cited.** No existing library test was cited in place of a new e2e test.
- **Inline verification.** For a few small tasks, verification ran inline rather than in a separate sub-agent.

## Manual checks

The test plan for this spec lists the manual checks: a stylistic-rewrite run, deliberate regression checks, prettylog built against a published xcl, coverage-map review, a root `go.mod` review, a fresh-checkout run, and reviews of the CI workflow and README.
