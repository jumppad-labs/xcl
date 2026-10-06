---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Plan: 20261006112023-aadf3c10-configuration-example

<!-- Metadata -->
<!-- Created: 2026-10-06T11:48:08Z -->
<!-- Commit: 82718166fe8ef1ff89d1988d2df915b5213523ce -->
<!-- Branch: main -->
<!-- Repository: github.com/jumppad-labs/xcl (xclconfig), github.com/jumppad-labs/xcl-website (xcl-website) -->

## Overview

The configuration example is rewritten as a real application: it loads its Kubernetes-like configuration into its own types, follows the links between ingress, service and deployment, and reports where each ingress path sends traffic. It is tested the way a developer would test their own configuration-driven code, against a test configuration of its own plus a smoke test of the built program, and the separate application config example that repeated it is removed. Application developers get an example they can copy and run as their own project, and the repository README and documentation website quote code that actually exists.

## Conventions

- **Testing & Mocking (testify `require`, no table-driven tests, positive and negative apart, tests live with the code they test, each example has a `smoke_test.go` that builds and runs the real binary)** — every new configonly test follows it: one route test against the test configuration, one test per error path, and the smoke test stays in the example.
- **Code style (gofmt, go vet, import grouping, descriptive names, `any`)** — applies to the rewritten `main.go`, the new routes code and tests.
- **Dependencies (prefer standard library, pin versions in go.mod)** — the rewrite drops `kr/pretty` from the example module; the smoke test uses `os/exec` only; tidying the example's module keeps its versions pinned.
- **Patterns & Architecture (dependency injection for testability)** — `loadConfig` takes the registry and config options from its caller, which is what lets `main` add state, masking and the event handler while tests load the same way without them.
- **Never modify dependency packages** — the example keeps pointing at xcl with a `replace` to the working tree.

## Architecture & Design Decisions

The work lands in two repos. In `xclconfig` (`/home/nicj/code/github.com/jumppad-labs/xcl`) the configuration example under `example/configonly` is rewritten as an application, the application config example is confirmed gone, and the README and `docs/README.md` are brought in line. In `xcl-website` (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) the configuration-only page is rewritten from the new source, the application-config page is deleted, and every page that quoted or linked to the application config example is repointed at the configuration example. This plan starts from the layout the e2e plan (`20261006071142-506b8289-e2e-suite-and-real-world-examples`) sets up: configonly is already its own Go module with `replace` directives to `../..` and `../prettylog`, its library-behaviour and source-inspection tests have moved to `e2e/` or been deleted, its smoke test uses `os/exec`, and `e2e/examples_test.go` runs its tests through `TestConfigOnlyExampleTestsPass`.

**The program: load, derive, print.** The program is split into three pieces that `main` calls in order, so every function and parameter is one the program itself uses. `loadConfig(dir, registry, options...)` registers the five block types on the registry it is given, builds the config with `xcl.WithPluginRegistry` plus the options it is passed, applies `dir`, and returns the application's own `appConfig` filled by one `Decode` call. `appConfig` holds only what the program reads, as slices — `Deployments []*resources.Deployment`, `Services []*resources.Service`, `Ingresses []*resources.Ingress` — because `Decode` fails a pointer field when more than one block of its type is declared (`decode.go:20-23`) and a real configuration, like the test one, can declare several. `ingressRoutes(cfg)` is the one piece of business logic: for each rule of each ingress it follows the rule's service id to the service, the service's deployment id to the deployment, and the service's target port to the container and port that expose it, returning a `Route` value per ingress path; a rule whose service, deployment or target port cannot be found is an error naming the ingress and path. `main` builds the registry and the prettylog handler, generates the per-run state key and temporary state directory as today, passes `WithStatePath`, `WithStateMask`, `WithEventHandler` and `WithEventData` as options, then prints one line per route to standard output and exits non-zero with `error: …` on standard error on any failure. `kr/pretty` and the returned `[]any` go: they existed to dump entities for tests and debugging (`example/configonly/main.go:84,176`). The configuration files are not touched.

**Tests a developer would write.** The example's tests are the ones an application author writes for configuration-driven code: `ingressRoutes` is tested against a test configuration under `example/configonly/testdata/`, separate from `./config`, loaded through `loadConfig` with no options (no state, no handler), so the test exercises the same load path the program uses. The test configuration is the example's shape with different values — two services and deployments, an ingress with two paths — so the expected routes cannot be confused with the example's own. Error paths (unknown service, unknown deployment, no container exposing the target port) each get their own test with a small configuration of their own, following `conventions/testing-and-mocking.md` (testify `require`, no tables, positive and negative apart). The smoke test keeps building and running the real binary with `os/exec`, now asserting the route line for the default configuration and the failure on a missing directory. Because the example is a standalone module that imports only public packages, copying it out and replacing the `replace` with a published xcl version is enough to build and test it.

**Docs follow the code.** The README's configuration-only section is rewritten around the routes the program reports, quoting the current configuration and `appConfig`, and its "Application configuration" section is deleted; the "Running them" paragraph stays with the e2e plan. On the website, the configuration-only page quotes the current types, configuration files, `loadConfig`, `ingressRoutes` and real output; the application-config page and its nav entry are removed; the sensitive-values page quotes configonly's `Secret` type and `secret.xcl` instead of the application config example; the state-masking page's call to action is rewritten to name the configuration example as the example that encrypts its state; and the index and plugins-page calls to action point at the configuration example. Only the plugins page's application-config button changes here; the rest of that page belongs to the docker-plugin spec. Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.

This beats keeping `run` and deriving routes from its `[]any` result (a test-only return value and a type switch no application would write), and beats dropping state from the program (it would leave no example of encrypted state for the pages that described the application config example, since the docker-plugin spec removes state encryption from the plugin example). The evidence is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Configuration loader (changed, configuration example).** Replaces the example's `run`. Owns registering the example's block types, building and applying the configuration with whatever options its caller supplies, and handing back the application's own configuration struct, filled by one `Decode`. Used by the program entry point (with state, masking and the pretty event receiver) and by the example's tests (with none).
- **Application configuration struct (changed, configuration example).** The application's view of its configuration: every deployment, service and ingress declared, as slices of the example's own types. Filled by the loader, read by the route deriver. Holds nothing the program does not read.
- **Route deriver (new, configuration example).** The example's business logic: turns the application configuration into one route per ingress path — host, path, service, deployment, container and port — by following the ids and the target port the configuration links together, and reports an error when a link cannot be followed. Depends only on the application configuration struct and the example's block types.
- **Program entry point (changed, configuration example).** Chooses the configuration directory, sets up the pretty event receiver, a per-run state key and a temporary state directory, calls the loader and the route deriver, prints the routes to standard output, and reports failures on standard error with a non-zero exit. No longer dumps the applied entities.
- **Block types and configuration (unchanged, configuration example).** The Go types and the three configuration files stay as they are; only doc comments that describe the program change if they no longer match.
- **Example tests (replaced, configuration example).** Route tests against a test configuration the example owns, one test per route error, and the smoke test that builds and runs the real binary and checks it prints the default configuration's route. They are run by xcl's test run through the e2e suite's existing configuration-example runner test.
- **Test configurations (new, configuration example).** A configuration in the example's own shape with different values, used by the route test, plus small configurations for each error path. Kept out of the program's default configuration directory.
- **Application config example (removed).** Already deleted in the working tree; the plan confirms nothing in either repo still refers to it.
- **Repository documentation (changed, xcl).** The README's configuration-only section describes and quotes the rewritten example and its route output; its application-config section and the layout table's mention of it are removed.
- **Website configuration-only page (changed, xcl-website).** Rewritten from the current types, configuration files, loader, route deriver and real output.
- **Website application-config page and navigation (removed / changed, xcl-website).** The page is deleted and its navigation entry removed.
- **Website pages that referred to the application config example (changed, xcl-website).** The sensitive-values page quotes the configuration example's secret type and configuration instead; the state-masking page's call to action names the configuration example as the example that encrypts its state; the home and plugins pages' calls to action point at the configuration example. The rest of the plugins page belongs to the docker-plugin spec.

## Data Structures & Interfaces

No library types or public interfaces change; xcl's API is untouched. The new contracts are inside the configuration example's `main` package, between the loader, the route deriver and the program entry point.

- **`appConfig`** — the application's view of its configuration, filled by one `Decode`. Each field is a slice so any number of blocks can be declared:

  ```go
  type appConfig struct {
      Deployments []*resources.Deployment
      Services    []*resources.Service
      Ingresses   []*resources.Ingress
  }
  ```

- **`Route`** — one ingress path and where its traffic ends up. The deriver produces one per rule of every ingress, in declaration order; `String` gives the line the program prints.

  ```go
  type Route struct {
      Host        string // ingress host, e.g. "api.example.com"
      Path        string // rule path, e.g. "/"
      Service     string // service id, e.g. "service.api"
      ServicePort int    // port the rule sends to on the service, e.g. 80
      Deployment  string // deployment id the service selects, e.g. "deployment.api"
      Container   string // container exposing the service's target port, e.g. "api"
      PortName    string // that container port's name, e.g. "http"
      Port        int    // the container port, e.g. 8080
  }

  // api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)
  func (r Route) String() string
  ```

- **`loadConfig`** — registers the example's block types on the registry given, applies `dir` with `xcl.WithPluginRegistry(r)` followed by `options`, and decodes into an `appConfig`. `main` passes state, masking, the event handler and event data; tests pass nothing.

  ```go
  func loadConfig(dir string, r *registry.PluginRegistry, options ...xcl.ConfigOption) (*appConfig, error)
  ```

- **`ingressRoutes`** — the business logic. Returns the routes, or an error naming the ingress and path whose service, deployment or target port cannot be resolved.

  ```go
  func ingressRoutes(cfg *appConfig) ([]Route, error)
  ```

- **Program output contract** — standard output holds exactly one `Route.String()` line per route; events go to standard error through prettylog; any failure prints `error: <message>` to standard error and exits 1.

## Implementation Detail

**From demo to application.** Today the configuration example is a harness: a `run` function returns every applied entity so tests can inspect them, and `main` dumps that list. After this plan it reads like the program a developer would write for their own tool. A reader opening it sees three steps in `main` — load the configuration into the application's own struct, work out the routes, print them — and each step is a named function with one job. The configuration-loading pattern the README already recommends (register types, apply, one `Decode` into a struct of your own) becomes the example's actual code rather than a snippet beside it. Nothing exists for tests alone: tests call the same loader and deriver `main` calls, and the extra behaviour `main` wants (state, masking, the event receiver) arrives as ordinary config options rather than special parameters.

**Business logic over configuration types.** The route deriver is a new kind of code for the examples: plain Go over the decoded types, following the ids and ports the configuration links together, with no xcl calls. It shows what references buy an application — the ingress names the service and the service names the deployment, so the program can follow them without matching labels — and it is the part a developer would unit test. Errors name the ingress and path that could not be resolved, wrapped once with context, following the standard library's style.

**Testing configuration-driven code.** The tests model the approach the spec asks to teach: keep a test configuration beside the code, separate from the one the program ships with, load it through the real loader, and assert the derived result as values. The test configuration varies the example's shape (more than one service, deployment and path) so a passing test proves the logic follows links rather than happening to match the defaults. Each failure mode has its own small configuration and its own test. The smoke test stays the only test that runs the binary.

**Docs quote the code that exists.** Every code snippet on the website and in the README that names a configuration-example file is copied from the current source, so the stale `resource.`-prefixed configuration, the single `main.xcl`, the old `run` signature and the old printed output all go. Pages that leaned on the application config example to illustrate sensitive values and state encryption now lean on the configuration example's secret and its encrypted state. The application config example's page disappears and the site's navigation and calls to action stop pointing at it.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **Plan `20261006071142-506b8289-e2e-suite-and-real-world-examples` (must land first).** Provides the starting layout: configonly as its own Go module (`go 1.25.0`, `replace` to `../..` and `../prettylog`), its library-behaviour and source-inspection tests moved to `e2e/` or deleted, a stdlib `os/exec` smoke test, the `TestConfigOnlyExampleTestsPass` runner test, and ownership of the README "Running them" paragraph. If it has not landed when implementation starts, implementation stops and asks.
- **Sibling spec `20261006112023-f7a185dc-docker-plugin-example` (parallel).** Owns the website's plugins page and the events and plugin-logging pages, and the README's plugin section; it removes state encryption and `XCL_STATE_KEY` from the plugin example and does not edit the state-masking page. This plan touches only the plugins page's application-config button, and owns the whole state-masking call to action. Whichever lands second rebases on the other's changes to that page and the README examples section.
- **xcl public packages (no change).** The root `xcl` package (`NewConfig`, options, `Apply`, `Decode`), `mask`, `types` and `plugins/registry` are what the example uses.
- **`example/prettylog` (no change).** The example's event receiver, reached through the existing `replace`.
- **Third-party libraries.** `stretchr/testify` stays for the tests; `kr/pretty` is dropped from the example's module once nothing prints with it. No new dependency.
- **Go toolchain.** Module tidying in the example; the e2e runner and CI minimum-Go job set up by the e2e plan cover building and testing it.
- **xcl-website repo (changed).** Astro 5 with MDX; building and type-checking the site verify the edited pages compile and no link is left dangling. Needs `node_modules` installed.
- **Docs issue #4 (overlap).** This plan fixes the drift caused by the configuration and application-config examples; the rest of that issue, such as the README's state-masking snippet, stays with it.

## Testing Approach

**Kinds of tests.** The configuration example gets unit tests of its route deriver, run against configurations loaded through the program's real loader (so they are integration tests of load-then-derive in practice), and keeps its smoke test, which builds and runs the real binary. These are the tests an application developer would write for their own configuration-driven code. xcl's own test run reaches them through the e2e suite's existing configuration-example runner test; no new e2e test is needed. No test checks documentation: the README and website pages are checked by human review. The website has no test suite: its pages are checked by building the site and by a manual snippet review.

**Where coverage concentrates.** On the route deriver, because it is the only business logic and the thing the spec says the example must teach to test. The positive test loads a test configuration that differs from the example's own — more than one service and deployment, an ingress with more than one path, a target port that is not on the first container — and asserts the full list of routes as values. Each way a route can fail to resolve (a rule naming an unknown service, a service naming an unknown deployment, a target port no container exposes) has its own test and its own small configuration, asserting an error that names the ingress and path.

**Load-bearing assertions.**
- For every ingress path, the program reports host, path, service, deployment, container and port, and they are the ones the configuration links together (route test against the test configuration).
- A configuration whose links cannot be followed fails with an error naming the ingress and path, never a silent omission (error-path tests).
- The built program, run with its default arguments, succeeds and prints the default configuration's route `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)`; run against a missing directory it fails with `error:` on standard error (smoke tests).

**Fit with existing conventions.** testify `require`, `Test<Subject><Behaviour>` names, one behaviour per function, no tables, positive and negative cases apart, tests beside the code they test, the smoke test in its own `smoke_test.go` using standard-library process handling. No Mockery mocks: nothing crosses an interface worth mocking.

**Deliberate gaps.** No tests of xcl's library behaviour in the example: the e2e suite owns those. No test inspects the example's source; "no test-only seams" is checked by review. No automated check that the configuration files are unchanged or that website snippets match source; both are manual reviews below. No tests for the website.

**Success metrics.** The spec defines none beyond its acceptance criteria; there are none to verify separately.

**Acceptance checks a person makes.**
- **Manual — captured in the implementation test plan**: copy the configuration example out of the repository, replace the `replace` with a published xcl version (and prettylog's), and confirm it builds and its tests pass.
- **Manual — captured in the implementation test plan**: review the program for test-only seams — every function, parameter and return value in the example is used by the program itself.
- **Manual — captured in the implementation test plan**: confirm the configuration files under the example's `config/` are byte-identical to before this work (config map, secret, deployment, service and ingress unchanged).
- **Manual — captured in the implementation test plan**: run the example from its directory and confirm it prints one route line per ingress path with host, path, service, deployment, container and port.
- **Manual — captured in the implementation test plan**: compare every website code snippet titled with a configuration-example file, and the quoted program output, against the current source and a real run.
- **Manual — captured in the implementation test plan**: search both repositories (excluding `.spektacular`, `node_modules`, `dist` and historical changelog entries) and confirm no application config example directory, page, link or mention remains, and the website builds with no broken link to `/examples/application-config/`.

## Milestones & Tasks

### Milestone 1: The configuration example is a real application

**What changes**: The configuration example stops being a test harness and becomes a program a developer could copy as the start of their own: it loads its configuration into its own types and reports, for each ingress path, the host, path, service, deployment, container and port the traffic reaches. Its tests are the ones an application author writes — the route logic checked against a test configuration of its own, each broken link checked to fail clearly, and a smoke test that builds and runs the binary. The configuration files themselves do not change.

**Validation point**: The example's tests pass from its own directory and through xcl's test run; running the example prints one route line per ingress path; the example's configuration files are unchanged; nothing in the program exists only for its tests.

#### - [x] Task: Rewrite the configuration example as load, derive and print
**Id:** 0ecb8e2f-60ba-4282-8fc0-16345eef7cdd
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Replace the example's test-facing `run` with the shape a real application has: a loader that registers the block types, applies the configuration with the options its caller passes and decodes it into the application's own struct; a route deriver that follows each ingress rule to its service, deployment, container and port; and a `main` that wires state, encryption and the event receiver in as options and prints one line per route. The entity dump and the pretty-printing dependency go, and the smoke test is updated to expect the route line. The configuration files are not touched.

*Technical detail:* [context.md#task-rewrite-the-configuration-example-as-load-derive-and-print](./context.md#task-rewrite-the-configuration-example-as-load-derive-and-print)

**Acceptance criteria**:
- [x] Running the example prints, for each ingress path, the host, path, service, deployment, container and port the traffic reaches.
- [x] A configuration whose ingress, service or target port cannot be followed makes the program fail with an error naming the ingress and path.
- [x] Every function, parameter and return value in the program is used by the program itself.
- [x] The example's configuration files are unchanged and the example no longer depends on the pretty-printing library.
- [x] The smoke test builds and runs the program with its default arguments and passes.

#### - [x] Task: Test the route logic against a test configuration
**Id:** 7fd38e4b-7f34-480a-9336-88843b4dcc5d
**Repo:** xclconfig
**Depends on:**
- 0ecb8e2f-60ba-4282-8fc0-16345eef7cdd — Rewrite the configuration example as load, derive and print
**Execution:** agent

Give the example the tests an application developer would write: a test configuration of its own, in the example's shape but with different values and more than one service, deployment and path, loaded through the program's loader and checked against the exact routes expected; and one small configuration and test per way a route can fail to resolve. Any remaining tests that asserted the old printed output are replaced.

*Technical detail:* [context.md#task-test-the-route-logic-against-a-test-configuration](./context.md#task-test-the-route-logic-against-a-test-configuration)

**Acceptance criteria**:
- [x] The example's tests load a test configuration separate from the example's own and check every reported route.
- [x] An unknown service, an unknown deployment and a target port no container exposes each have their own test that expects a clear error.
- [x] No test is table-driven and no test mixes a success case with a failure case.
- [x] The example's tests pass from its own directory and through xcl's normal test run.

### Milestone 2: The repository has one configuration example and documents it

**What changes**: The separate application config example, which repeated the configuration example, is gone for good, and the repository's own documentation says so: the README's configuration-only section describes the rewritten example and quotes its current code and output, and the application-config section and the layout table's mention of it are removed.

**Validation point**: No application config example remains in the repository outside historical changelog entries and planning records; the README's configuration-example snippets match the source (manual review).

#### - [x] Task: Remove the application config example and update the repository docs
**Id:** cf5c5e2f-4a80-4094-9bc3-24d39ecf4d68
**Repo:** xclconfig
**Depends on:**
- 0ecb8e2f-60ba-4282-8fc0-16345eef7cdd — Rewrite the configuration example as load, derive and print
**Execution:** agent

Make sure the application config example's directory is gone, then bring the repository's documentation in line: the README's examples introduction counts two programs, its configuration-only section quotes the current configuration and code and shows the route output, its application-config section is deleted, and the repository layout table no longer mentions the removed example. The "Running them" paragraph is left to the e2e plan.

*Technical detail:* [context.md#task-remove-the-application-config-example-and-update-the-repository-docs](./context.md#task-remove-the-application-config-example-and-update-the-repository-docs)

**Acceptance criteria**:
- [x] The repository contains no application config example.
- [x] The README describes the configuration example as it now is, and every snippet it quotes from the example matches the source.
- [x] No current documentation in the repository refers to the application config example.

### Milestone 3: The website matches the configuration example

**What changes**: Readers of the documentation website see the configuration example as it now is: its page quotes the current types, configuration files, program and real output. The application config example's page is removed, the navigation and every call to action that pointed at it lead to the configuration example instead, and the sensitive-values page illustrates sensitive data with the configuration example's secret rather than the removed example.

**Validation point**: The site builds and type-checks; no page refers to the application config example or links to its page; every snippet titled with a configuration-example file matches the current source (manual review).

#### - [x] Task: Rewrite the website's configuration example page
**Id:** 47233c2a-8e54-4529-983f-2c8360994f65
**Repo:** xcl-website
**Depends on:**
- 7fd38e4b-7f34-480a-9336-88843b4dcc5d — Test the route logic against a test configuration
**Execution:** agent

Rewrite the configuration-only page from the current example: its Go types including the secret, its three configuration files, the loader, the route deriver and how it is tested against a test configuration, and the real output of a run. The page's closing call to action stops offering the application config example.

*Technical detail:* [context.md#task-rewrite-the-websites-configuration-example-page](./context.md#task-rewrite-the-websites-configuration-example-page)

**Acceptance criteria**:
- [x] Every code snippet on the page titled with a configuration-example file matches that file's current source.
- [x] The output shown is what the example prints when run.
- [x] The page explains the route report and how the example's tests check it.
- [x] The page does not refer to the application config example.

#### - [x] Task: Remove the application config page and repoint its links
**Id:** 911b4433-9869-4be4-9d6d-14b967cd3a4f
**Repo:** xcl-website
**Depends on:**
- 47233c2a-8e54-4529-983f-2c8360994f65 — Rewrite the website's configuration example page
**Execution:** agent

Delete the application config example's page and its navigation entry, and point every call to action and example list that led to it — on the home page, the state-masking page and the plugins page — at the configuration example instead. The state-masking page's call to action is rewritten to name only the configuration example, the one example that still encrypts its state. On the plugins page only that one button changes; the rest of the page belongs to the docker-plugin spec.

*Technical detail:* [context.md#task-remove-the-application-config-page-and-repoint-its-links](./context.md#task-remove-the-application-config-page-and-repoint-its-links)

**Acceptance criteria**:
- [x] The site has no application config example page and no navigation entry for it.
- [x] No page links to the removed page, and the home page lists the examples that exist.
- [x] The state-masking page's call to action names the configuration example as the example that encrypts its state.
- [x] The site builds without errors.

#### - [x] Task: Illustrate sensitive values with the configuration example
**Id:** 6a85a693-bd97-435b-ad7f-84fe152337f9
**Repo:** xcl-website
**Depends on:**
- 47233c2a-8e54-4529-983f-2c8360994f65 — Rewrite the website's configuration example page
**Execution:** agent

The sensitive-values page quotes the removed application config example's database type, configuration and connection-string code. Replace the declaration example with the configuration example's secret type and secret configuration, keep the explanation of revealing a value with a snippet that is not attributed to any example file, rewrite the passage about the example printing JSON so it no longer cites a removed example, and point the call to action at the configuration example.

*Technical detail:* [context.md#task-illustrate-sensitive-values-with-the-configuration-example](./context.md#task-illustrate-sensitive-values-with-the-configuration-example)

**Acceptance criteria**:
- [x] Every snippet on the page titled with an example file matches that file's current source.
- [x] The page still explains declaring, revealing and printing a sensitive value.
- [x] The page does not refer to the application config example.
- [x] The site builds without errors.

## Open Questions

- **Has the e2e plan landed in the shape this plan assumes?** It depends on `20261006071142-506b8289-e2e-suite-and-real-world-examples` being implemented first: configonly as its own module, its library and source-inspection tests gone, a standard-library smoke test, and the e2e runner test for it. If any of that is missing or different when implementation starts, STOP and ask the user whether to wait for it or adapt; do not reimplement the e2e plan's work here.
- **What does the sibling docker-plugin spec leave on the shared pages?** The plugins page's call to action and the README examples introduction may already have been changed by that spec when these tasks run. Only discoverable at implementation time; adapt to what has landed (keep its changes, apply only this plan's), and STOP and ask only if the two directly contradict each other.

No other open questions remain.

## Out of Scope

- **Generating Kubernetes manifests** from the configuration example. xcl's example types do not cover all of Kubernetes (spec Non-Goal).
- **Docs drift not caused by the examples.** The rest of docs issue #4, such as the README's state-masking snippet, stays with that issue (spec Non-Goal).
- **Changing the configuration example's configuration or block types.** The config map, secret, deployment, service and ingress stay exactly as they are.
- **The plugin example, the plugins website page, and the events and plugin-logging pages.** They belong to spec `20261006112023-f7a185dc-docker-plugin-example`; this plan only removes the application-config button from the plugins page.
- **The README's "Running them" paragraph, example module files, the e2e runner and CI.** Owned by plan `20261006071142-506b8289-e2e-suite-and-real-world-examples`.
- **Rewriting historical changelog sections** that mention the application config example; they record what shipped at the time.
- **Website redirects** for the removed application-config URL; the site has no redirect mechanism configured and the page is simply removed.
- **Library behaviour or public API changes.** None are needed.

### Changelog input

Notes for the epic's single changelog entry, written once after all its specs are implemented (this spec does not edit `CHANGELOG.md`):

- The configuration example (`example/configonly`) is now a real application: it loads its configuration into its own types and prints one line per ingress route (host, path, service, deployment, container and port).
- The example shows how to test configuration-driven code: unit tests of the route logic against a separate test configuration, plus a smoke test that runs the built program.
- The application config example (`example/appconfig`) and its website page are removed; the configuration example covers what it showed, and website links now point there.
- Breaking changes: none (no library or public API change).

## Changelog


### 2026-10-06 — Task: Rewrite the configuration example as load, derive and print

**What was done**: `example/configonly` now loads its configuration with `loadConfig(dir, registry, options...)` into an `appConfig` of slices (one `Decode`), derives one `Route` per ingress path with `ingressRoutes` (new `routes.go`), and `main` prints one route line per path. The old harness `run(out, handler, …)` with its `## Resources`/`## Deployments` printing and `Destroy` is gone, `main_test.go` (old-output tests) is deleted, and the smoke test asserts the route line `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)` with `DB_PASSWORD` set as the Makefile does.

**Deviations**: Starting point differed from the plan's description: the program was the 8271816 restore (`run(out, handler, r, dir, stateDir, stateKey)` printing sections and destroying), and `kr/pretty` was already absent from main.go and go.mod, so there was nothing to drop. Kept a small `run(dir string) error` called only by `main`, so the temporary state directory is removed with `defer` before `os.Exit`; the plan had `main` remove it before every exit. Added a `findContainerPort` helper inside routes.go. `go mod tidy` changed nothing. `configDir` moved from main_test.go into smoke_test.go.

**Files changed**:
- `xclconfig: example/configonly/main.go`
- `xclconfig: example/configonly/routes.go`
- `xclconfig: example/configonly/main_test.go` (deleted)
- `xclconfig: example/configonly/smoke_test.go`
- `xclconfig: example/configonly/Makefile`

**Discoveries**: An unreadable configuration directory surfaces from `Apply` as an "error finding .vars files" error; the smoke test only asserts `error:` on stderr. The `secret.xcl` `env("DB_PASSWORD")` parses even when the variable is unset.

### 2026-10-06 — Task: Test the route logic against a test configuration

**What was done**: Added `routes_test.go` with a positive test that loads `testdata/routes` (two deployments, two services, one ingress with two paths, the api target port on the second container) through `loadConfig` with no options and asserts the full `[]Route`; one test each for an unknown service, an unknown deployment and an unreachable target port, each with its own small `testdata/` configuration; and a `Route.String` format test.

**Deviations**: None. The broken-link configurations write the missing ids and port as literals, as planned. main_test.go had already been deleted in the previous task.

**Files changed**:
- `xclconfig: example/configonly/routes_test.go`
- `xclconfig: example/configonly/testdata/routes/deployment.xcl`
- `xclconfig: example/configonly/testdata/routes/ingress.xcl`
- `xclconfig: example/configonly/testdata/unknown_service/main.xcl`
- `xclconfig: example/configonly/testdata/unknown_deployment/main.xcl`
- `xclconfig: example/configonly/testdata/unknown_target_port/main.xcl`

**Discoveries**: None.

### 2026-10-06 — Task: Remove the application config example and update the repository docs

**What was done**: Confirmed `example/appconfig` is gone (not on disk, not tracked). The README's examples introduction now counts two programs, each its own module; the "Configuration only" section is rewritten around the three config files, unprefixed reference syntax, a trimmed `deployment "api"` snippet copied from `deployment.xcl`, the current `appConfig` and `Decode` lines from `main.go`, `ingressRoutes` and the route output line, and the test approach; the "Application configuration" section is deleted. `docs/README.md`'s layout row lists two examples.

**Deviations**: In the e2e-owned "Running them" paragraph, only "prints the resources it parsed" became "prints what it read", as the plan's context allowed. No tests: documentation is checked by human review.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: docs/README.md`

**Discoveries**: The README's "Plugins" section still quotes a `resource.postgres.main` resource id; that section belongs to the docker-plugin spec.

### 2026-10-06 — Task: Rewrite the website's configuration example page

**What was done**: Rewrote `configuration-only.mdx` from the current example: the Go types (ConfigMap, Secret, Deployment, Container, then Service, Ingress, Rule) and the three configuration files are quoted whole, `appConfig`/`loadConfig`, the `loadConfig` call with its options, `ingressRoutes`/`findContainerPort` and the printing loop are quoted from `main.go` and `routes.go`, a new "Testing it" section quotes `testdata/routes/ingress.xcl` and two tests from `routes_test.go`, and "Run it" shows trimmed real event lines plus the real route line. The closing call to action offers only the plugins example.

**Deviations**: Snippets were generated by a script that copies contiguous source ranges, so they match byte for byte; the two snippets from inside `run` are dedented by one tab. `npm ci` was run in the website worktree to build (node_modules is gitignored). No tests: the site is checked by `npm run build` (9 pages) and `astro check` (0 errors).

**Files changed**:
- `xcl-website: src/pages/examples/configuration-only.mdx`

**Discoveries**: None.

### 2026-10-06 — Task: Remove the application config page and repoint its links

**What was done**: Deleted `src/pages/examples/application-config.mdx` and its nav entry. The home page hero button and closing call to action now lead to the configuration example, and its examples list is "Two examples" with the configuration-only bullet mentioning route reporting. The state-masking call to action now reads "The configuration example encrypts its state with a fresh key each run." and links to the configuration example. The plugins page loses its application-config button.

**Deviations**: On the plugins page the call to action's body also changed, from "The other two examples parse configuration…" to "The configuration example reads its configuration into Go types with no plugin at all.", because it counted the removed example; the rest of that page is untouched and stays with the docker-plugin spec.

**Files changed**:
- `xcl-website: src/pages/examples/application-config.mdx` (deleted)
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: src/pages/state-masking.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`

**Discoveries**: None.

### 2026-10-06 — Task: Illustrate sensitive values with the configuration example

**What was done**: The sensitive-values page now declares a sensitive field with the configuration example's `Secret` type (`types.Sensitive[map[string]string]`) and the `secret "db"` block, both copied verbatim from source. The Reveal illustration is an untitled snippet that builds a connection string from `secret.Data.Reveal()["password"]`, the JSON passage no longer cites a removed example (marshalling the secret writes `"data": "(sensitive)"`, as `Sensitive.MarshalJSON` does), and the call to action names and links the configuration example.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/pages/sensitive-values.mdx`

**Discoveries**: None.
