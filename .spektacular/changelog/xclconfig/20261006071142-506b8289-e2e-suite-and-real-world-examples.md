---
created_date: "2026-10-06"
document_status: draft
project: xclconfig
spec: 20261006071142-506b8289-e2e-suite-and-real-world-examples
plan: 20261006071142-506b8289-e2e-suite-and-real-world-examples
---

# E2E suite and standalone examples

xcl's behaviour is now covered by a dedicated end-to-end test suite that uses the library exactly as an application would, independent of the examples. The examples (configonly, plugin and prettylog) are now standalone Go modules that test only what they do. Each can be copied out of the repository and built against a published xcl. Applications importing xcl no longer pick up the examples' terminal-styling dependencies.

> Derived from project xclconfig, spec/plan 20261006071142-506b8289-e2e-suite-and-real-world-examples. See the project-level record for the full feature.

## What changed in this repo

- **New `e2e/` suite.** It uses public packages plus `internal/testutil`, with its own fixtures: `e2e/fixtures/{kube,services,inprocess,externalplugin}` and `e2e/testdata/{kube,plugin}`. Its 67 carried-over tests cover every library behaviour the examples used to check. `e2e/COVERAGE.md` maps each removed example test to its covering test and lists the deleted source-inspection tests.
- **Example runner.** `e2e/examples_test.go` runs each example module's tests: `TestConfigOnlyExampleTestsPass`, `TestPluginExampleTestsPass` and `TestPrettylogExampleTestsPass`. The runner is tested against a passing and a failing fixture module under `e2e/testdata`.
- **Example modules.** `example/configonly`, `example/plugin` and `example/prettylog` each have their own `go.mod` and `go.sum`, with a `replace` to the local xcl (and to the local prettylog for configonly and plugin). Their smoke tests use `os/exec`. Their library-behaviour and source-inspection tests are removed. prettylog's tests use their own fixtures (`fixtures_test.go`, `testdata/encode/main.xcl`).
- **configonly program restored.** `example/configonly/main.go` is the 8271816 program, adapted to the current tests, configuration and types. In `config/deployment.xcl`, `secret_key_ref` now reads `secret.db.meta.name`.
- **Root module.** A root `go mod tidy` removed `charmbracelet/*`, `muesli/termenv`, `kr/pretty` and their indirect dependencies from `go.mod` and `go.sum`.
- **CI.** The `build-minimum-go` job in `.github/workflows/go.yml` builds and vets each example module on Go 1.25.0.
- **README.** "Running them" describes the standalone modules and how their tests are run.

## Why

The examples used to double as xcl's end-to-end tests. Moving that coverage into a suite xcl owns means rewriting an example cannot lose coverage, and a stylistic change to an example cannot fail the build. Making each example its own module keeps its dependencies out of xcl and lets readers lift an example out of the repository unchanged.
