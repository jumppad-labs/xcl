---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Plan: 20261006071142-506b8289-e2e-suite-and-real-world-examples

<!-- Metadata -->
<!-- Created: 2026-10-06T11:39:42Z -->
<!-- Commit: 82718166fe8ef1ff89d1988d2df915b5213523ce -->
<!-- Branch: main -->
<!-- Repository: github.com/jumppad-labs/xcl (xclconfig) -->

## Overview

xcl gets a dedicated end-to-end test suite that exercises the library only through its public packages, against fixtures it owns, and carries over every library behaviour the examples test today, recorded in a coverage map. The examples then test only what they do: their library-behaviour and source-inspection tests are removed, each becomes its own Go module pointed at the local xcl, and the suite runs each example's tests. Maintainers get end-to-end coverage that no longer depends on how the examples are written, and the sibling specs can rewrite the examples as real-world projects without losing coverage.

## Conventions

- **Testing & Mocking (testify `require`, Mockery, no table-driven tests, positive and negative apart, tests live with the code they test, each example carries its own tests and a `smoke_test.go` that builds and runs the real binary)** — every carried-over e2e test and every rewritten example test follows it; it is why each example gets one named e2e test instead of a loop, why the e2e suite only delegates to examples' own tests, and why configonly and plugin keep smoke tests while prettylog (not a program) has none.
- **Shared test helpers live in `internal/testutil`** — the e2e suite reuses the event recorder and stream capture from it rather than copying them; examples, being separate modules, cannot import it and keep a small local helper instead.
- **Generate test state with a real apply** — the carried-over state, masking and destroy tests produce their state by applying fixtures, never by hand-writing state files.
- **Assert ordering on graph parents, not provider call order** — any carried-over test about event or destroy ordering of unlinked resources asserts on the graph, not on call order.
- **Dependencies (prefer stdlib, pin versions in go.mod)** — example `go.mod` files pin the versions the root currently uses; smoke-test helpers use `os/exec` only; the root `go.mod` drops example-only dependencies.
- **Code style (gofmt, go vet, import grouping, `any`)** — applies to every new Go file in `e2e/` and the examples.
- **Never modify dependency packages** — the example modules point at xcl with a `replace` to the working tree, never by editing the module cache.

## Architecture & Design Decisions

All work lands in the `xclconfig` repo (`/home/nicj/code/github.com/jumppad-labs/xcl`); no other registered repo changes. The solution has three parts: a top-level `e2e/` test package inside xcl's root module, the three examples (`example/configonly`, `example/plugin`, `example/prettylog`) turned into Go modules of their own, and the removal of every test that inspects an example's source.

**The e2e suite.** `e2e/` is an ordinary directory of the root module holding only `_test.go` files in package `e2e_test`, so the existing CI step `go test ./... -race` runs it with no workflow change. It drives xcl the way an application does — `registry.NewPluginRegistry`, `xcl.NewConfig` and its options, `Apply`, `Entities`, `Decode`, `Destroy`, events, state masking — and imports no package under `internal/` except the shared test helpers in `internal/testutil`, which are test infrastructure rather than library surface (`conventions/shared-test-helpers.md` forbids copying them). Its fixtures live with it: `.xcl` configurations under `e2e/testdata/` (ignored by `./...`), and the Go fixtures that must be compiled — the block types, an in-process plugin and an external plugin binary modelled on today's `example/plugin` — under `e2e/fixtures/`, written only against public packages. A `TestMain` builds the external plugin once into a temp directory, as `example/plugin/main_test.go:45-66` does today. Each library-behaviour test removed from an example is reproduced as a named e2e test (or, where the library's own tests already fail when that behaviour breaks, mapped to that test), and `e2e/COVERAGE.md` lists every removed test beside the test that now covers it. The carried-over tests are written to the project's testing conventions: testify `require`, one behaviour per function, no tables, positive and negative cases apart, state produced by a real apply (`conventions/test-state-from-real-apply.md`), and ordering asserted on graph parents rather than provider call order where ordering matters (`conventions/assert-ordering-on-graph-parents.md`).

**Running the examples.** Each example gets its own `go.mod` (module paths unchanged, `go 1.25.0`) with `replace github.com/jumppad-labs/xcl => ../..`; configonly and plugin also `replace github.com/jumppad-labs/xcl/example/prettylog => ../prettylog`, so no import path changes and example-only dependencies (`charmbracelet/*`, `muesli/termenv`, `kr/pretty`) leave xcl's `go.mod` after a root `go mod tidy`. Because the root `./...` no longer reaches nested modules, the e2e suite runs them: one named test per example (`TestConfigOnlyExampleTestsPass`, `TestPluginExampleTestsPass`, `TestPrettylogExampleTestsPass`) runs `go test ./...` in that example's directory through a shared helper and fails with the example's name and the command output. This delegates to the tests that live with each example rather than testing example code from outside, which keeps faith with `conventions/testing-and-mocking.md`; it uses one function per example rather than a loop over discovered directories, since that would be a table-driven test. Example tests then use only public packages: the smoke tests swap `internal/testutil.BuildProgram/RunProgram` for a small local `os/exec` helper, and prettylog's tests replace `internal/parser.TestPlugin`, `internal/test_fixtures/registered` and the `../../internal/test_fixtures/...` config with their own types, in-process plugin and `testdata` config, so prettylog builds alone against any published xcl version. Its tests remain its own behaviour; it has no smoke test because it is not a program.

**Removing source inspection.** The four configonly and two plugin tests that parse `main.go` with `go/ast` (and the root `static_examples_test.go`, already staged for deletion) are deleted outright, not carried over: they check how examples are written. The guarantee one of them stood for — that what a reader copies compiles on the minimum supported Go — becomes behavioural: the `build-minimum-go` CI job additionally builds and vets each example module under Go 1.25.0. That xcl's own module sheds the example-only dependencies is shown by a root `go mod tidy` and checked in review; no test lists or scans the repository's dependencies. Example tests that check what an example prints or does stay with the example (the sibling configuration and Docker-plugin specs rewrite them later); only tests asserting what xcl did move.

This beats a separate e2e module (it would fall outside the root CI step), a `go.work` (it changes how the root builds for everyone, against the spec's `replace` steer), and keeping examples in the root module (it breaks the one-module-per-example constraint and keeps example dependencies in xcl). The evidence for each is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **End-to-end suite (new).** A test-only package at the top of xcl's root module that exercises the library as an application does, through its public packages only (plus the shared test helpers). It owns the carried-over library-behaviour tests — parsing, decoding, references, variables, events and their phases, plugin loading and logging (in-process and external), apply and destroy, state and event masking, the plaintext-state warning, and silence without an event handler — and the per-example runner tests. It depends on the e2e fixtures and on the shared test helpers; it runs under the root `go test ./...`.
- **E2E fixtures (new).** Owned by the e2e suite and used by nothing else. Configuration fixtures reproduce the shapes the examples' library tests relied on (Kubernetes-like nested and repeated blocks with references, variables and a secret read from the environment; a plugin-driven configuration with a module, outputs and published values). Go fixtures provide the registered block types, an in-process plugin and an external plugin program, each written only against xcl's public plugin and logger packages. The external plugin is built once per suite run.
- **Coverage map (new).** A document kept beside the e2e suite listing every library-behaviour test removed from the examples, the example it came from, and the test that now covers it (an e2e test, or an existing library test that fails when that behaviour breaks). It is the reviewable record behind "no coverage lost".
- **Example runner (new, part of the e2e suite).** One named test per example that runs that example module's own tests and fails naming the example, with the command's output. It is the only link from xcl's test run to the examples.
- **Example modules (changed).** configonly, plugin and prettylog each become a standalone Go module pointed at the local xcl (and configonly and plugin at the local prettylog) by `replace`. Their tests keep only what the example itself does — its output, its failure on a missing config, its smoke test — and use public packages only. Their library-behaviour tests move to the e2e suite, and their source-inspection tests are deleted.
- **Example smoke tests (changed).** configonly's and plugin's smoke tests build and run the real binary with standard-library process handling instead of the internal program helpers.
- **prettylog tests (changed).** Keep testing prettylog's own behaviour, but against fixtures prettylog owns — its own block types, a small in-process plugin and a configuration file — so the package builds and tests outside the repository.
- **Root module definition (changed).** Loses the dependencies only the examples used once they are their own modules.
- **CI workflow (changed).** The minimum-Go job also builds and vets each example module on the minimum supported Go, the behavioural replacement for the deleted portable-lookup source checks. The main test job is unchanged; it reaches the examples through the e2e suite.
- **Documentation (changed).** The README's "running them" passage says each example is its own module and how its tests are run; the coverage map's preamble says how the suite runs and that a new example adds its own named runner test.
- **Shared test helpers (unchanged, reused).** The event recorder, stream capture and entity helpers are reused by the e2e suite; the program build-and-run helpers lose their example callers but remain for the library's own delivery test.

## Data Structures & Interfaces

No library types or public interfaces are added or changed: xcl's API is untouched. The contracts this plan introduces are between test code, module files and documents.

- **Example module definition.** Each example's `go.mod` keeps its current import path as the module path and points at the local tree:

  ```
  module github.com/jumppad-labs/xcl/example/<name>
  go 1.25.0
  require github.com/jumppad-labs/xcl v0.0.0-00010101000000-000000000000
  replace github.com/jumppad-labs/xcl => ../..
  // configonly and plugin only:
  require github.com/jumppad-labs/xcl/example/prettylog v0.0.0-00010101000000-000000000000
  replace github.com/jumppad-labs/xcl/example/prettylog => ../prettylog
  ```

  Removing the `replace` and requiring a published xcl version is all it takes to build an example outside the repository.

- **Example runner helper (e2e, test-only).** `runExampleTests(t testing.TB, example string)` runs `go test ./...` in `example/<example>` relative to the e2e package and fails with `example <name>: tests failed` plus the combined output. Each per-example test calls it once with a literal name.

- **External plugin fixture build (e2e, test-only).** A package-level path to the external plugin binary, set by `TestMain` after building `e2e/fixtures/externalplugin` into a temp directory and removed when the run ends — the same shape as today's plugin example.

- **Coverage map rows (`e2e/COVERAGE.md`).** One table per source example with the columns *removed test*, *behaviour*, *now covered by* (package-qualified test name). Every removed library-behaviour test appears exactly once; deleted source-inspection tests are listed separately as removed without replacement, with the reason.

## Implementation Detail

**New module boundary.** The repository goes from one Go module to four: xcl itself and one per example. This is the main code-shape change. A developer opening an example sees a self-describing project — its own module file, its own dependencies, a `replace` pointing at the checkout — and can copy the directory out, drop the `replace`, require a published xcl and build. Inside xcl, `go build ./...`, `go vet ./...` and `go test ./...` no longer descend into `example/`; the only route from xcl's test run into the examples is the e2e runner, which shells out to `go test` in each example's directory. Import paths do not change, so the sibling specs that rewrite configonly and plugin start from compiling code.

**New test layer.** The e2e suite is a new kind of test in this codebase: black-box, whole-library, application-shaped. It follows the existing in-package test style (`require`, one behaviour per function, descriptive `Test<Subject><Behaviour>` names, small local query helpers over the shared event recorder) and the existing pattern of building a plugin binary once in `TestMain`. What is new is that it never reaches below the public packages, and that its fixtures are self-owned rather than borrowed from `internal/test_fixtures` or from an example. Each carried-over test keeps the behaviour and assertions of the example test it replaces but is renamed for what it checks about xcl rather than which example it lived in (for example "configonly example reports parse event with file" becomes a test about parse events carrying their file).

**Splitting example tests by what they assert.** Every test in an example falls into one of three buckets, and the split is mechanical once the rule is stated: a test that asserts what xcl did (entities, decoded values, links, events, state, errors, writes to the standard streams) is library behaviour and moves to e2e; a test that asserts what the example printed or how it exits stays; a test that parses the example's source is deleted. Example test files shrink accordingly and lose their `go/ast` and `internal/testutil` imports; helpers left with no caller go too.

**prettylog stands on its own.** Its tests keep their subject and assertions but swap the library's internal fixtures for ones it owns: a couple of registered block types declared in its test files, a minimal in-process plugin written against the public plugin package (as the plugin example's in-process plugin already is), and a configuration under its own `testdata`. Nothing in prettylog refers to a path outside its directory.

**Behaviour replaces inspection.** Where a deleted source check stood for a real guarantee — examples compile on the minimum supported Go — the guarantee is kept by building each example module on that Go in CI. That the library no longer carries the examples' dependencies is settled by `go mod tidy` and review, not by a test. No test anywhere reads an example's source or the repository's module files.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **Uncommitted working tree (must settle first).** The tree holds in-flight work this plan builds on: the new shared test helpers, untracked example smoke tests and configonly config files, a configonly program mid-rewrite whose test file no longer compiles against it, the staged deletion of the root example-inspection test and of the application-config example, and a root module file that needs tidying. It must be committed or otherwise settled before implementation starts; if configonly's tests still do not compile then, implementation stops and asks.
- **xcl public packages (no change).** The root package, `events`, `mask`, `state`, `types`, `plugins`, `plugins/registry` and `logger` are what the e2e fixtures, e2e tests and example tests use. No library change is planned; if carrying a behaviour over turns out to need an internal package, implementation stops and asks rather than widening the public API.
- **Shared test helpers (`internal/testutil`, no change).** The event recorder, stream capture and entity helpers are reused by the e2e suite. The program build-and-run helpers keep their remaining library caller.
- **Go toolchain and module tooling.** Nested modules with `replace` directives; `go mod tidy` in each example and in the root needs the module cache or network for the examples' third-party dependencies.
- **Third-party libraries (versions unchanged, ownership moves).** `charmbracelet/lipgloss`, `charmbracelet/log` and `muesli/termenv` move from xcl's module to prettylog's (and transitively configonly's and plugin's); `kr/pretty` moves to configonly's; `hashicorp/go-plugin` and `stretchr/testify` are required by the example modules that use them. Versions are pinned to what the root uses today.
- **GitHub Actions workflow (changed).** The minimum-Go job gains a build and vet of each example module; the main job is unchanged.
- **Prior plan `20261003153421-9fa72edd-masking` (landed).** Source of the state-encryption, event-masking and plaintext-warning tests in the examples that this plan carries over. Its changelog-entry-per-spec convention no longer applies: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.
- **Downstream specs in epic `20261006071139-7b266535-examples-and-output`.** `…-configuration-example` and `…-docker-plugin-example` depend on this plan landing first: they rewrite configonly and plugin on top of the module layout, runner tests and stdlib smoke tests this plan sets up. `…-encoder-syntax-highlighting` is independent but shares prettylog; whichever lands second rebases onto the other's prettylog changes.

## Testing Approach

**Kinds of tests.** This plan is mostly test code, and almost all of it is end-to-end: black-box tests that drive xcl through its public packages exactly as an application would, against fixtures the suite owns. Alongside them sit the runner tests that execute each example module's own tests, small unit tests of the runner itself, and the examples' remaining tests (output checks and smoke tests that build and run the real binary). prettylog's tests remain unit and integration tests of the handler, re-pointed at fixtures prettylog owns. No new library unit tests are expected; where a removed example test is already covered by a library test that fails when the behaviour breaks, the coverage map cites that test instead of duplicating it.

**Where coverage concentrates.** The carried-over library behaviours get the most attention because losing one is the main risk of the spec: decoding of nested, repeated and omitted blocks; values read from other blocks, variables and the environment; links between resources; parse, create, destroy and loading events with their sources, operations, phases and files; in-process and external plugin logging at the right levels and between the right phases; plugins and block types reported as loaded; published values and module outputs; entities converting from saved records and agreeing with state; no secret reaching state or event data; the plaintext-state warning present without a key and absent with one; nothing written to the standard streams without an event handler; and a parse error and a missing external plugin reported as errors. Each keeps the assertion of the example test it replaces, one behaviour per test, positive and negative cases in separate functions, state produced by a real apply, ordering asserted on graph parents where the resources involved are not linked.

**Load-bearing assertions.**
- Every library-behaviour test removed from an example has a named covering test in the coverage map, and that test fails when the behaviour it covers breaks.
- Each example's tests are run by xcl's test run; a failing example fails it with the example's name. The runner is itself tested against two tiny fixture modules under the suite's test data — one whose test passes and one whose test fails — asserting success for the first and an error naming the second.
- No test in the repository parses or inspects an example's source; the examples' and root's remaining tests import no `go/ast` or `go/parser` for that purpose.
- Examples build and vet on the minimum supported Go (CI).

**Fit with existing conventions.** Tests use testify `require`, descriptive `Test<Subject><Behaviour>` names, no tables, and the shared recorder from the test helpers package with small package-local queries over it — the same shape as the root package's and the plugin package's existing end-to-end tests. Examples cannot use the shared helpers, so their smoke tests use standard-library process handling directly.

**Deliberate gaps.** No tests are written for example output that the sibling specs will rewrite; the examples' existing output tests are kept as they are. No Mockery mocks are needed: nothing here crosses an interface that warrants one. Nothing tests the coverage map's file contents; it is a reviewed document, not an executable contract.

**Success metrics.**
- *A change to how an example is written, without changing what it does, never fails a test.* Partly behavioural: deleting every source-inspection test and keeping only output, exit and smoke tests in the examples removes every test that could fail this way, and the load-bearing assertion above covers the deletion. Confirming that a purely stylistic rewrite of an example leaves the suite green is **Manual — captured in the implementation test plan**.
- *A regression in xcl's behaviour is caught by the e2e suite or the library's own tests, never only by an example's tests.* **Manual — captured in the implementation test plan**: break behaviours named in the coverage map and confirm a covering e2e or library test fails, not only an example test.

**Acceptance checks a person makes.**
- **Manual — captured in the implementation test plan**: copy prettylog out of the repository, point it at a published xcl version, and confirm it builds and its tests pass.
- **Manual — captured in the implementation test plan**: review the coverage map against the removed tests (current files and `HEAD` 8271816) — every library-behaviour test listed once, each with a covering test that exists.
- **Manual — captured in the implementation test plan**: after a root `go mod tidy`, review xcl's `go.mod` and confirm it no longer lists `charmbracelet/*`, `muesli/termenv` or `kr/pretty`.
- **Manual — captured in the implementation test plan**: from a fresh checkout, run xcl's full test run and confirm the e2e suite passes and imports no internal package other than the shared test helpers.

## Milestones & Tasks

### Milestone 1: xcl has its own end-to-end suite

**What changes**: xcl gains a dedicated end-to-end test suite that uses the library only as an application would, with configurations, block types and plugins of its own. Every library behaviour the examples check today is reproduced there (or mapped to an existing library test that guards it), and a coverage map beside the suite records, for each example test, what now covers it. Nothing is removed yet, so for one milestone the behaviours are checked twice; that is deliberate, so coverage is in place before anything is taken away.

**Validation point**: The e2e suite passes as part of xcl's normal test run, imports no internal package except the shared test helpers, and the coverage map lists every library-behaviour test in the examples (current files and `HEAD` 8271816) beside an existing covering test.

#### - [x] Task: Build the e2e fixtures
**Id:** eba0ed7e-2060-4a6c-995d-491b4240b481
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Create the end-to-end suite's home and the fixtures it owns: configurations reproducing the shapes the examples' library tests rely on, the block types they decode into, an in-process plugin and an external plugin program. Everything is written against xcl's public packages only, and the suite builds the external plugin once per run. This gives the carried-over tests something to run against that no example rewrite can disturb.

*Technical detail:* [context.md#task-build-the-e2e-fixtures](./context.md#task-build-the-e2e-fixtures)

**Acceptance criteria**:
- [x] The suite has its own configurations, block types, in-process plugin and external plugin, none borrowed from an example or from the library's internal fixtures.
- [x] The fixtures use no xcl package under `internal/`.
- [x] The external plugin is built automatically when the suite runs, and the build is cleaned up afterwards.
- [x] Applying each fixture configuration through the public API succeeds.

#### - [x] Task: Carry the configuration-only behaviours into the e2e suite
**Id:** a2c04319-7e73-4baf-9994-5f5b558d593e
**Repo:** xclconfig
**Depends on:**
- eba0ed7e-2060-4a6c-995d-491b4240b481 — Build the e2e fixtures
**Execution:** agent

Reproduce in the e2e suite every library behaviour the configuration-only example's tests check today: decoding, references, variables, environment values, events, destroy, state masking and the plaintext warning, and silence without an event handler. Each becomes a test named for what it checks about xcl, with the same assertion, written to the project's testing conventions. Where an existing library test already fails when the behaviour breaks, it is noted for the coverage map instead of duplicated.

*Technical detail:* [context.md#task-carry-the-configuration-only-behaviours-into-the-e2e-suite](./context.md#task-carry-the-configuration-only-behaviours-into-the-e2e-suite)

**Acceptance criteria**:
- [x] Every library-behaviour test in the configuration-only example (current file and `HEAD` 8271816) has a covering test in the e2e suite or an identified existing library test.
- [x] Each new test checks one behaviour, positive and negative cases are in separate tests, and no test is table-driven.
- [x] The new tests pass, and each fails if the behaviour it covers is broken.

#### - [x] Task: Carry the plugin behaviours into the e2e suite
**Id:** b77459e0-d663-45ec-9f56-78939930fd8b
**Repo:** xclconfig
**Depends on:**
- eba0ed7e-2060-4a6c-995d-491b4240b481 — Build the e2e fixtures
**Execution:** agent

Reproduce in the e2e suite every library behaviour the plugin example's tests check today: values filled by providers and passed between blocks, plugin loading and logging from in-process and external plugins, event phases for create and destroy, published values and module outputs, entity conversion and agreement with state, masking and the plaintext warning, and the error for a missing external plugin. Each becomes a test named for what it checks about xcl, written to the project's testing conventions.

*Technical detail:* [context.md#task-carry-the-plugin-behaviours-into-the-e2e-suite](./context.md#task-carry-the-plugin-behaviours-into-the-e2e-suite)

**Acceptance criteria**:
- [x] Every library-behaviour test in the plugin example (current file and `HEAD` 8271816) has a covering test in the e2e suite or an identified existing library test.
- [x] Ordering assertions between unlinked resources are made on the dependency graph, not on provider call order.
- [x] The new tests pass, and each fails if the behaviour it covers is broken.

#### - [x] Task: Write the coverage map
**Id:** 8169f01b-09b3-4431-99d2-105774a51a58
**Repo:** xclconfig
**Depends on:**
- a2c04319-7e73-4baf-9994-5f5b558d593e — Carry the configuration-only behaviours into the e2e suite
- b77459e0-d663-45ec-9f56-78939930fd8b — Carry the plugin behaviours into the e2e suite
**Execution:** agent

Write the coverage map beside the e2e suite. It lists every library-behaviour test that will be removed from the examples, the example it came from, the behaviour it checked and the test that now covers it, and separately lists the source-inspection tests that are deleted without replacement and why. This is the record a reviewer uses to confirm no coverage is lost.

*Technical detail:* [context.md#task-write-the-coverage-map](./context.md#task-write-the-coverage-map)

**Acceptance criteria**:
- [x] Every library-behaviour test in the configuration-only and plugin examples appears exactly once, beside a covering test that exists.
- [x] Every source-inspection test being deleted is listed with the reason it has no replacement.
- [x] The map explains how the suite runs the examples and that a new example adds its own runner test.

### Milestone 2: Examples test only what they do

**What changes**: The examples stop testing xcl. Their library-behaviour tests are removed now that the e2e suite covers them, and every test that reads an example's source code — in the examples and at the repository root — is deleted. What remains in each example tests that example's own output and exit behaviour, plus its smoke test. Changing how an example is written, without changing what it does, can no longer fail a test.

**Validation point**: xcl's full test run passes; no test in the repository parses or inspects an example's source; every test removed in this milestone appears in the coverage map.

#### - [x] Task: Strip library and source tests from the configuration-only example
**Id:** 1a368904-38d9-4516-8788-59179654b98b
**Repo:** xclconfig
**Depends on:**
- 8169f01b-09b3-4431-99d2-105774a51a58 — Write the coverage map
**Execution:** agent

Remove from the configuration-only example every test the coverage map says is now covered elsewhere, and delete its tests that parse the example's source. Also make sure the root-level test that inspected example sources stays deleted. What remains tests only what the example prints and how it exits, plus its smoke test.

*Technical detail:* [context.md#task-strip-library-and-source-tests-from-the-configuration-only-example](./context.md#task-strip-library-and-source-tests-from-the-configuration-only-example)

**Acceptance criteria**:
- [x] The configuration-only example's tests check only its own output, exit behaviour and smoke run.
- [x] No test in the example or at the repository root parses or inspects an example's source.
- [x] Every removed test is listed in the coverage map, and the example's remaining tests pass.

#### - [x] Task: Strip library and source tests from the plugin example
**Id:** 613cdd65-9037-4936-bc39-e60c2b1dad9e
**Repo:** xclconfig
**Depends on:**
- 8169f01b-09b3-4431-99d2-105774a51a58 — Write the coverage map
**Execution:** agent

Remove from the plugin example every test the coverage map says is now covered elsewhere, and delete its tests that parse the example's source. What remains tests only what the example prints and how it exits, plus its smoke test.

*Technical detail:* [context.md#task-strip-library-and-source-tests-from-the-plugin-example](./context.md#task-strip-library-and-source-tests-from-the-plugin-example)

**Acceptance criteria**:
- [x] The plugin example's tests check only its own output, exit behaviour and smoke run.
- [x] No test in the plugin example parses or inspects its source.
- [x] Every removed test is listed in the coverage map, and the example's remaining tests pass.

### Milestone 3: Each example is a standalone project the suite runs

**What changes**: configonly, plugin and prettylog each become a Go module of their own, pointed at the local xcl, so example-only dependencies leave xcl's module and an example can be copied out and built on its own. Their tests use only xcl's public packages: smoke tests build and run the real binary with the standard library, and prettylog tests against fixtures it owns. The e2e suite runs each example's tests and fails, naming the example, if any fail; CI also builds every example on the minimum supported Go. The README describes the new layout.

**Validation point**: xcl's full test run passes and runs every example's tests; a deliberately failing fixture module makes the runner fail with its name; a root `go mod tidy` leaves xcl's module without the charmbracelet libraries (review); each example builds on Go 1.25.0; prettylog builds and tests outside the repository against a published xcl (manual).

#### - [x] Task: Give prettylog fixtures of its own
**Id:** 53083707-d518-4a1a-b592-29fb29bdfb39
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

prettylog's tests currently borrow the library's internal test plugin, internal fixture types and an internal configuration file. Replace them with block types, a small in-process plugin and a configuration that prettylog owns, written against public packages, so the tests keep checking the same handler behaviour without reaching outside the directory.

*Technical detail:* [context.md#task-give-prettylog-fixtures-of-its-own](./context.md#task-give-prettylog-fixtures-of-its-own)

**Acceptance criteria**:
- [x] prettylog's tests import no xcl package under `internal/` and refer to no file outside prettylog's directory.
- [x] Every prettylog test that existed before still exists and checks the same behaviour.
- [x] prettylog's tests pass.

#### - [x] Task: Make each example its own module
**Id:** 8c17e6e6-7531-4b52-8751-2e93ea32dd80
**Repo:** xclconfig
**Depends on:**
- 1a368904-38d9-4516-8788-59179654b98b — Strip library and source tests from the configuration-only example
- 613cdd65-9037-4936-bc39-e60c2b1dad9e — Strip library and source tests from the plugin example
- 53083707-d518-4a1a-b592-29fb29bdfb39 — Give prettylog fixtures of its own
**Execution:** agent

Give configonly, plugin and prettylog each a module definition pointed at the local xcl (and configonly and plugin at the local prettylog), rewrite the two smoke tests to build and run the real binary with the standard library, and tidy xcl's own module so example-only dependencies leave it.

*Technical detail:* [context.md#task-make-each-example-its-own-module](./context.md#task-make-each-example-its-own-module)

**Acceptance criteria**:
- [x] Each example builds and its tests pass when run from its own directory.
- [x] No example imports an xcl package under `internal/`.
- [x] After a root `go mod tidy`, xcl's module no longer lists the dependencies only the examples use (the charmbracelet libraries, `muesli/termenv`, `kr/pretty`), confirmed in review.

#### - [x] Task: Run the example tests from the e2e suite
**Id:** 892c2dd1-5469-4d48-8ba3-fda73b13a6d1
**Repo:** xclconfig
**Depends on:**
- 8c17e6e6-7531-4b52-8751-2e93ea32dd80 — Make each example its own module
**Execution:** agent

Add the example runner to the e2e suite: one named test per example that runs that example's tests and fails, naming the example, if they fail. The runner itself is tested against a tiny passing module and a tiny failing module kept in the suite's test data, so a broken example is proven to fail the suite.

*Technical detail:* [context.md#task-run-the-example-tests-from-the-e2e-suite](./context.md#task-run-the-example-tests-from-the-e2e-suite)

**Acceptance criteria**:
- [x] xcl's normal test run runs the configuration-only, plugin and prettylog examples' tests, smoke tests included.
- [x] A failing example makes the suite fail with a message naming that example.
- [x] The runner's own tests pass for the passing fixture module and report the failing one by name.

#### - [x] Task: Build the examples on the minimum supported Go in CI
**Id:** bc7c14a9-94c1-42e0-bce2-321798ffab49
**Repo:** xclconfig
**Depends on:**
- 8c17e6e6-7531-4b52-8751-2e93ea32dd80 — Make each example its own module
**Execution:** agent

The deleted portable-lookup source checks stood for one real guarantee: what a reader copies from an example compiles on the oldest supported Go. Now that examples are outside xcl's module, the minimum-Go CI job builds and vets each example module as well, keeping that guarantee as behaviour rather than source inspection.

*Technical detail:* [context.md#task-build-the-examples-on-the-minimum-supported-go-in-ci](./context.md#task-build-the-examples-on-the-minimum-supported-go-in-ci)

**Acceptance criteria**:
- [x] The minimum-Go CI job builds and vets every example module on Go 1.25.0; the workflow change is reviewed by hand.

#### - [x] Task: Document the new layout
**Id:** 0ed63f13-d9c0-4b13-8684-006388cd401a
**Repo:** xclconfig
**Depends on:**
- 892c2dd1-5469-4d48-8ba3-fda73b13a6d1 — Run the example tests from the e2e suite
**Execution:** agent

Update the README's passage on running the examples to say each is its own module pointed at the local xcl and that xcl's test run runs their tests through the e2e suite. The README edit is reviewed by hand.

*Technical detail:* [context.md#task-document-the-new-layout](./context.md#task-document-the-new-layout)

**Acceptance criteria**:
- [x] The README describes the examples as standalone modules and says how their tests are run.

### Changelog input

Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented. Notes for that entry:

- New end-to-end suite (`e2e/`) exercising xcl through its public packages only, with a coverage map recording where every library-behaviour test removed from the examples is now covered.
- The configonly, plugin and prettylog examples are now standalone Go modules pointed at the local xcl; xcl's `go test ./...` runs their tests through the e2e suite, and CI builds each example on the minimum supported Go.
- xcl's module no longer requires the charmbracelet libraries, `muesli/termenv` or `kr/pretty` (example-only dependencies); source-inspection tests in the examples were removed.
- Breaking changes: none (no library API change).

## Open Questions

- **Will the in-flight working tree be settled when implementation starts?** It depends on the uncommitted configonly rewrite being finished or committed: at planning time configonly's test file calls a `run` signature its program no longer has. If it still does not compile when implementation begins, STOP and ask the user which version of the configonly example is the baseline; do not guess the intended signature.
- **Does any carried-over behaviour need something only an internal package exposes?** Only discoverable while rewriting each test against the public API. If one does, STOP and ask the user whether to map it to an existing library test, extend the public API, or drop it — never import the internal package into the e2e suite.

No other open questions remain.

## Out of Scope

- **Encoder syntax highlighting.** prettylog keeps its own highlighter; replacing it with library highlighting is spec `20261006112108-17623cda-encoder-syntax-highlighting` (issue #6).
- **Rewriting the configuration and plugin examples as real applications**, including removing their test-only seams, changing what they print, and updating their README and website prose. Those are specs `20261006112023-aadf3c10-configuration-example` and `20261006112023-f7a185dc-docker-plugin-example`, which build on this plan's module layout and runner.
- **Removing the application-config example.** Its deletion is already in the working tree and belongs to the configuration-example spec; this plan neither restores nor documents it.
- **Publishing a public test-helper package** for application and plugin authors. Examples use standard-library process handling in their smoke tests instead; whether xcl should publish helpers is left to the sibling specs if they need one.
- **New library behaviour or public API changes.** The e2e suite covers existing behaviour only.
- **A guard that detects examples without a runner test.** Each new example adds its own named runner test as part of its spec.

## Changelog

### 2026-10-06 — Baseline decision: configuration-only example restored

**What was done**: The working tree's configonly test file did not compile against its rewritten `main.go` (plan Open Question 1). The user chose to restore `example/configonly/main.go` from 8271816, the version the tests were written for, and follow the plan as written. The 8271816 program needed small adaptations to match the committed tests, config and types: `run` gained a trailing `stateKey []byte` (nil means no state mask, so xcl warns about plain state), `main` generates the key with `newStateKey`, types are registered under a single block name (`config_map`, not `resource`/`config_map`), and the `secret` type is registered. In `config/deployment.xcl`, the `secret_key_ref` name now reads `secret.db.meta.name`, which the test expects, instead of `meta.id`. Nothing used `kr/pretty` any more, so a root `go mod tidy` removed it.

**Deviations**: The restored `main.go` is the 8271816 program with the adaptations above, not a byte-for-byte copy, because that copy did not compile against the current tests.

**Files changed**:
- `example/configonly/main.go`
- `example/configonly/config/deployment.xcl`
- `go.mod`
- `go.sum`

**Discoveries**: A tracked ELF binary named `configonly` sits at the repository root. `go build ./example/configonly/` run from the root overwrites it. Build to a temporary path with `-o`, or restore it with `git checkout -- configonly`.

### 2026-10-06 — Task: Build the e2e fixtures

**What was done**: Added the e2e suite's home and its own fixtures. There are block types in `e2e/fixtures/kube` (Kubernetes-like) and `e2e/fixtures/services` (postgres, redis, app, ingress), an in-process plugin in `e2e/fixtures/inprocess`, and an external plugin binary in `e2e/fixtures/externalplugin`. All of them are written against public packages only. The configurations are in `e2e/testdata/kube` and `e2e/testdata/plugin`. `TestMain` builds the external plugin once into a temporary directory and removes it afterwards. Shared helpers (`registerKubeTypes`, `registerPlugins`, `newKubeConfig`, `newPluginConfig`, `newConfig`) and two smoke tests confirm each configuration applies.

**Deviations**: The types are split across the `kube` and `services` packages rather than a single `resources` package, because both sets define an `Ingress` type, which the context allowed for. `newConfig` adds the event handler and state mask only when they are non-nil, and does not set `WithEventData`. Tests that need entity data on events add that option themselves.

**Files changed**:
- `e2e/doc_test.go`
- `e2e/main_test.go`
- `e2e/helpers_test.go`
- `e2e/fixtures_test.go`
- `e2e/fixtures/kube/kube.go`
- `e2e/fixtures/services/services.go`
- `e2e/fixtures/inprocess/plugin.go`
- `e2e/fixtures/externalplugin/main.go`
- `e2e/testdata/kube/deployment.xcl`
- `e2e/testdata/kube/ingress.xcl`
- `e2e/testdata/kube/secret.xcl`
- `e2e/testdata/plugin/main.xcl`
- `e2e/testdata/plugin/modules/db/db.xcl`

**Discoveries**: The kube configuration uses unlabelled block types (ids such as `config_map.api`). The plugin configuration uses `resource "postgres" "main"` (ids such as `resource.postgres.main`). `e2e` holds only external test files, so its imports are listed under `go list`'s `.XTestImports`, not `.TestImports`.

### 2026-10-06 — Task: Carry the configuration-only behaviours into the e2e suite

**What was done**: Each library-behaviour test of the configuration-only example (28 of them, all present in the current file, which is a superset of the one at 8271816) now has a named e2e test. Each runs against the suite's kube fixtures through the public API. The tests cover decoding, references and variables, parse, create and destroy events, destroy emptying the saved state, silence without an event handler, sensitive values (the env secret, masked state and events, the plain-state warning with and without a key), and encoding of saved entities. Old→new pairs are staged in `.spektacular/tmp/coverage-configonly-{a,b}.md` for the coverage map.

**Deviations**:
- `newConfig` in `e2e/helpers_test.go` now always sets `WithEventData(xcl.EventDataProcessed)`, mirroring the examples.
- Two example tests were split along library and rendering lines. `ShowsCreatedEntities` has a library half (`TestEncodeSavedEntityWritesCreatedConfiguration`) and a prettylog rendering half that stays with prettylog. `PrintsNoSecret` has library halves (`TestStandardStreamsHoldNoSecret`, `TestEncodeSavedEntityMasksSecret`), while its report and rendering checks stay with the example.
- The silence test drops the example's `## Resources` output assertion.
- State tests read the state file directly after `Apply` instead of the example's event-handler trick.
- No existing library test was cited in place of a new e2e test.

**Files changed**:
- `e2e/helpers_test.go`
- `e2e/kube_helpers_test.go`
- `e2e/decode_test.go`
- `e2e/events_test.go`
- `e2e/destroy_test.go`
- `e2e/silence_test.go`
- `e2e/sensitive_test.go`
- `e2e/encode_test.go`

**Discoveries**: Apply of the kube configuration succeeds without `DB_PASSWORD` set, because `env()` yields an empty value. The e2e tests set it anyway so the secret checks are meaningful. `state.NewFileStateStore(dir).Load()` errors on a missing file, which lets the destroy test tell "emptied" from "deleted".

### 2026-10-06 — Task: Carry the plugin behaviours into the e2e suite

**What was done**: Each library-behaviour test of the plugin example now has a named e2e test, 41 in all. They run against the suite's in-process and external plugin fixtures through the public API. They cover values filled by providers and passed between blocks and plugins, generated types, published values and module outputs, in-process and external plugin logging (levels, sources, between start and success), create, destroy and plugin-load events, destroy emptying state, entity encoding and agreement with state, masking, the plain-state warning, silence without a handler, and the missing-external-plugin error. Old→new pairs are staged in `.spektacular/tmp/coverage-plugin-{a,b}.md`.

**Deviations**:
- Expected event sources follow the fixtures' names: `Plugin` and `externalplugin`, rather than the example's `ExamplePlugin` and `external`.
- Example-only halves stay with the example or prettylog: the `make build` hint in the missing-plugin error, the `## Resources` report, and rendered-text checks.
- No ordering is asserted between different resources. Only per-resource phase sequences are checked, and destroy was seen to interleave across resources.
- No existing library test was cited in place of a new e2e test.

**Files changed**:
- `e2e/plugin_helpers_test.go`
- `e2e/plugin_values_test.go`
- `e2e/plugin_logging_test.go`
- `e2e/plugin_errors_test.go`
- `e2e/plugin_events_test.go`
- `e2e/plugin_state_test.go`
- `e2e/plugin_encode_test.go`
- `e2e/plugin_silence_test.go`

**Discoveries**: xcl names an in-process plugin's event source after its Go type name (`Plugin`) and an external plugin's after its binary's base name (`externalplugin`). The plugin tests were stable across repeated `-race -count=3` runs.

### 2026-10-06 — Task: Write the coverage map

**What was done**: Wrote `e2e/COVERAGE.md`. Its preamble covers what the suite is, how it runs each example module through one named runner test, and that a new example adds its own runner test. It has one table per example (29 configonly rows, 38 plugin rows), each pairing a removed test with its behaviour and the covering e2e test. It lists the tests and test halves that stay with the example or prettylog, and gives a table of the 14 source-inspection tests (4 configonly, 2 plugin, 8 from the root `static_examples_test.go` at 8271816) deleted without replacement, each with its reason.

**Deviations**: Six example tests are split. Their library half is mapped in the tables, and their printing or rendering half is named under "Stayed with the example or prettylog". `TestPluginExamplePrintsPublishedTotal` stays with the example (it checks a printed line); the count behind it is also covered by `TestOutputsHoldsModuleOutputs`. The preamble names `TestPluginExampleTestsPass`, a runner test that the "Run the example tests from the e2e suite" task adds later. The staged `.spektacular/tmp/coverage-*.md` scratch files were folded in and removed.

**Files changed**:
- `e2e/COVERAGE.md`

**Discoveries**: None.

### 2026-10-06 — Task: Strip library and source tests from the configuration-only example

**What was done**: Removed from `example/configonly/main_test.go` the 28 library-behaviour tests the coverage map now covers elsewhere, the four source-inspection tests and `TestConfigOnlyExampleShowsCreatedEntities`, whose rendering half is prettylog's. Also removed the helpers left with no caller. The file no longer imports `go/ast`, `go/parser`, `go/token`, `internal/testutil`, `events` or `state`. What remains is six tests of the example's own report and failure behaviour, plus its two smoke tests. The root `static_examples_test.go` stays deleted.

**Deviations**: `TestConfigOnlyExamplePrintsNoSecret` keeps its checks on the example's report and rendered events. Its standard-stream capture, which needed `internal/testutil`, was dropped, because `e2e.TestStandardStreamsHoldNoSecret` covers that half.

**Files changed**:
- `example/configonly/main_test.go`

**Discoveries**: None.

### 2026-10-06 — Task: Strip library and source tests from the plugin example

**What was done**: Removed from `example/plugin/main_test.go` the 36 library-behaviour tests the coverage map covers elsewhere and the two source-inspection tests, along with the helpers left with no caller. The file no longer imports `go/ast`, `go/parser`, `go/token`, `internal/testutil`, `events`, `state` or `encoding/json`. What remains is `TestMain`, which builds the external plugin, and six tests of the example's own report and failure behaviour. The two smoke tests are unchanged.

**Deviations**: `TestPluginExampleFailsWithoutExternalPlugin` was kept but trimmed to the example's own "build it with `make build`" hint. Its library half (`ErrPluginLoad`, the path in the error) is covered by `e2e.TestMissingExternalPluginFailsApply`. `TestPluginExamplePrintsNoSecret` lost its standard-stream capture, which `e2e.TestPluginStandardStreamsHoldNoSecret` covers. The "Stayed with the example" list in `e2e/COVERAGE.md` was corrected to name the hint half of `TestPluginExampleFailsWithoutExternalPlugin`.

**Files changed**:
- `example/plugin/main_test.go`
- `e2e/COVERAGE.md`

**Discoveries**: None.

### 2026-10-06 — Task: Give prettylog fixtures of its own

**What was done**: prettylog's tests now use fixtures prettylog owns. `example/prettylog/fixtures_test.go` holds `Database`, `Cache`, `Network` and `Container` block types and a small in-process plugin (`fixturePlugin`) whose providers fill the computed `provider_id` and `assigned_address`, all written against the public `types`, `plugins` and `logger` packages. The configuration is a copy at `example/prettylog/testdata/encode/main.xcl`. `prettylog_test.go` no longer imports `internal/parser` or `internal/test_fixtures/registered`, and no longer reads a path outside its directory. `HOME` is set with `t.Setenv`. All 30 prettylog tests remain, with unchanged assertions.

**Deviations**: The local `NetworkAttachment` leaves out the internal fixture's optional `id` key field, which the configuration never sets. This task's verification was run inline (prettylog tests with `-race`, a grep for internal or out-of-directory paths, the test-name list compared against HEAD, and `go vet ./...`) rather than in a separate sub-agent, because the test-writing sub-agent had already run the same checks.

**Files changed**:
- `example/prettylog/prettylog_test.go`
- `example/prettylog/fixtures_test.go`
- `example/prettylog/testdata/encode/main.xcl`

**Discoveries**: None.

### 2026-10-06 — Task: Make each example its own module

**What was done**: configonly, plugin and prettylog each now have a `go.mod` (module path unchanged, `go 1.25.0`) and a `go.sum`, with `replace github.com/jumppad-labs/xcl => ../..`. configonly and plugin also replace prettylog with `../prettylog`. The example modules were seeded with the root's requirement versions and then tidied offline, so their third-party versions match what the root used. The configonly and plugin smoke tests now build and run the real binary with `os/exec` through a small local helper (`buildExample`, `runExample`) in place of `internal/testutil`, keeping the same assertions. A root `go mod tidy` removed `charmbracelet/*`, `muesli/termenv` and their indirect dependencies from xcl's module; `kr/pretty` had already gone.

**Deviations**: The first verification sub-agent ran its checks in the main checkout rather than the worktree, so it reported missing `go.mod` files. The checks were re-run directly in the worktree: each example vets, tests and is tidy; no example imports `internal/`; the root is tidy and lists no charmbracelet, termenv or kr/pretty dependency; `go list ./...` from the root reaches no example package (only `plugins/example`, which is library code).

**Files changed**:
- `example/prettylog/go.mod`
- `example/prettylog/go.sum`
- `example/configonly/go.mod`
- `example/configonly/go.sum`
- `example/configonly/smoke_test.go`
- `example/plugin/go.mod`
- `example/plugin/go.sum`
- `example/plugin/smoke_test.go`
- `go.mod`
- `go.sum`

**Discoveries**: `go mod tidy` in an example module succeeds even while its tests import another module's `internal/` package; only build or vet catches that. Running tidy with `GOPROXY=off` works from the module cache once the examples are seeded with the root's requirements.

### 2026-10-06 — Task: Run the example tests from the e2e suite

**What was done**: Added `e2e/examples_test.go`. `exampleTestsError(dir)` runs `go test ./...` in a module directory and returns `example <name>: tests failed: …` with the combined output. `runExampleTests(t, name)` checks that `go` is on `PATH` (a missing `go` fails the test rather than skipping it) and runs `example/<name>`. There are three named runner tests, `TestConfigOnlyExampleTestsPass`, `TestPluginExampleTestsPass` and `TestPrettylogExampleTestsPass`, each run with `t.Parallel()`. The runner is itself tested against two stdlib-only fixture modules under `e2e/testdata`, one passing and one failing: `TestExampleRunnerPassesForAPassingModule` and `TestExampleRunnerNamesAFailingModule`.

**Deviations**: The tests were written and verified inline rather than in a separate sub-agent, given how small the task is. Verification was gofmt, `go vet ./...`, and `go test -race -count=1 ./...` from the root, all green.

**Files changed**:
- `e2e/examples_test.go`
- `e2e/testdata/passingexample/go.mod`
- `e2e/testdata/passingexample/pass_test.go`
- `e2e/testdata/failingexample/go.mod`
- `e2e/testdata/failingexample/fail_test.go`

**Discoveries**: None.

### 2026-10-06 — Task: Build the examples on the minimum supported Go in CI

**What was done**: The `build-minimum-go` job in `.github/workflows/go.yml` has a new final step, "Build and vet the examples". It runs `go build -v ./...` and `go vet ./...` in each of `example/configonly`, `example/plugin` and `example/prettylog` under the job's pinned `GOTOOLCHAIN=go1.25.0`, and fails on the first error. The job's comment now explains that the examples are built there because they are separate modules, and that what a reader copies out of an example must compile on the minimum Go.

**Deviations**: None. As planned, no test reads the workflow file. The change was checked by parsing the YAML and building each example locally on the current toolchain. The Go 1.25.0 build itself is proved when CI runs.

**Files changed**:
- `.github/workflows/go.yml`

**Discoveries**: None.

### 2026-10-06 — Task: Document the new layout

**What was done**: In the README's "Running them" passage, the sentence saying the examples' tests run as part of `go test ./...` is replaced. The passage now says each example is its own Go module pointed at the checkout with a `replace`, and that dropping the `replace` and requiring a published xcl lets it build on its own. It says how to run an example's tests and that xcl's `go test ./...` runs them through the `e2e/` suite, which also holds the library's end-to-end tests and links to `e2e/COVERAGE.md`. The rest of the examples section is left to the sibling specs.

**Deviations**: The README says to run an example's tests with `go test ./...` (or `make test`), because prettylog has no Makefile. The minimum-Go guarantee from the previous task was also confirmed locally: all three examples build and vet on the cached go1.25.0 toolchain.

**Files changed**:
- `README.md`

**Discoveries**: None.
