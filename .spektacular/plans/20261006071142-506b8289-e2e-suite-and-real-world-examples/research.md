---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Research: 20261006071142-506b8289-e2e-suite-and-real-world-examples

## Alternatives considered and rejected

- **e2e as its own Go module (`e2e/go.mod` with a `replace` to `..`).** Rejected: the root CI step `go test ./... -race` (`.github/workflows/go.yml`) would no longer reach it, so it would need its own CI step, and the spec calls it "xcl's whole-library black-box tests", i.e. part of xcl. Living in the root module it runs with every `go test ./...`.
- **Keep the examples inside the root module and have e2e import their packages.** Rejected: contradicts the spec constraint "Each example is its own Go module" and keeps example-only dependencies (`github.com/charmbracelet/*`, `github.com/kr/pretty`, `github.com/muesli/termenv`) in xcl's `go.mod` (current `go.mod:19-22`, working-tree diff adds `termenv` as a direct requirement).
- **A `go.work` file tying the example modules to the root.** Rejected: the spec's Technical Approach fixes a `replace` per example module; a workspace would also change how the root module builds for every contributor and would make "prettylog builds alone" harder to see.
- **One e2e test that globs `example/*/go.mod` and loops over the examples as subtests.** Rejected: that is a table-driven test, forbidden by `conventions/testing-and-mocking.md`. Chosen instead: one named test per example.
- **Keep the library-behaviour assertions in the examples and only add e2e alongside.** Rejected by the spec's "No coverage lost" requirement, which removes them from the examples.
- **Point the e2e suite at the existing `plugins/example` person plugin instead of its own plugin fixtures.** Rejected: the spec says the suite "lives with its own fixtures"; `plugins/example` is the library's plugin-boundary fixture with its own tests (`config_plugin_boundary_test.go:21-35`), and the behaviours being carried over (connection strings, computed URLs, published values, module outputs, in-process and external logs) come from `example/plugin`'s plugins, not the person plugin.
- **Keep `example/*/smoke_test.go` on `internal/testutil.BuildProgram`/`RunProgram`.** Impossible once each example is its own module: Go refuses imports of another module's `internal/` packages. The spec constraint "Examples use only xcl's public packages" also forbids it. Smoke tests switch to standard-library `os/exec` (the sibling specs' Technical Approach says the same).
- **A test that lists the repository's dependencies to enforce that the library does not depend on the charmbracelet libraries.** Rejected: `conventions/testing-and-mocking.md` forbids tests that walk, parse or `go list` the repository to enforce a rule. Once the examples are their own modules, a root `go mod tidy` drops the example-only dependencies, and review of `go.mod` confirms it.

## Chosen approach — evidence

- `example/plugin/main_test.go:45-66` — `TestMain` builds the external plugin once into a temp dir with `go build`; the e2e suite uses the same pattern for its own external plugin fixture.
- `example/plugin/internal/plugin.go:1-60` — an in-process plugin written only against public packages (`plugins`, `logger`); proves e2e fixtures and prettylog's test plugin can be built without `internal/`.
- `example/plugin/external/main.go` — external plugin binary using only `plugins`, `logger` and `hashicorp/go-plugin`; template for `e2e/fixtures/externalplugin`.
- `example/configonly/main.go:109-175` (working tree) — the application flow e2e tests reproduce through the public API: `registry.NewPluginRegistry` + `RegisterType`, `xcl.NewConfig` with `WithPluginRegistry`, `WithStatePath`, `WithStateMask(mask.EncryptAES256GCM(key))`, `WithEventHandler`, `WithEventData(xcl.EventDataProcessed)`, then `Apply`, `Entities`, `Decode`, `Destroy`.
- `internal/testutil/{events,streams,entities,programs}.go` — the shared recorder, stream capture and entity helpers the moved assertions use; e2e (same module) may import them as test infrastructure.
- `.github/workflows/go.yml` — `go test ./... -race` in the root reaches `e2e/`; `build-minimum-go` builds only the root module, so once examples leave it the minimum-Go guarantee for examples needs its own build step.
- `README.md:221-233` — "The tests for all three run as part of `go test ./...`" becomes untrue as written once the examples are modules; text must say they run through the e2e suite.
- `README.md:500-513` — the claim "every runnable example uses the function form" was guarded by the source-inspection tests `TestConfigOnlyExampleUsesPortableLookupForm` / `TestPluginExampleUsesPortableLookupForm`; building every example module on Go 1.25.0 in CI is the behavioural replacement.
- `CHANGELOG.md:1-20` — past specs each added one `## <spec name>` section, newest on top; that convention has changed, and this spec does not edit it (the epic's changelog entry is written once after all its specs are implemented).

## Files examined

- `xclconfig:example/configonly/main_test.go:1-920` — 46 tests; mix of library behaviour, example output and 4 source-inspection tests (`ImportsNoPluginCode:284`, `UsesPortableLookupForm:539`, `AssemblesItsConfigurationWithDecode:585`, `MakesNoPerTypeLookups:617`); imports `go/ast`, `go/parser`, `internal/testutil`, `example/prettylog`. Calls `run(out, …)` with a signature the working-tree `main.go` no longer has — the file is mid-edit.
- `xclconfig:example/configonly/main.go:109` — working-tree `run(handler, registry, dir, stateDir, stateKey)`; uses `github.com/kr/pretty`.
- `xclconfig:example/configonly/smoke_test.go:1-28` — smoke tests via `internal/testutil.BuildProgram/RunProgram` (untracked file).
- `xclconfig:example/configonly/config/{deployment,ingress,secret}.xcl` — untracked; `config/main.xcl` deleted in the working tree.
- `xclconfig:example/plugin/main_test.go:1-1244` — 48 tests; `TestMain:45` builds the external plugin; source-inspection tests `DefinesNoTypesOrConfig:248` and `UsesPortableLookupForm:853`.
- `xclconfig:example/plugin/main.go:120` — `run(out, handler, registry, dir, externalPlugin, stateDir, stateKey)`.
- `xclconfig:example/plugin/smoke_test.go` — smoke tests via `internal/testutil` (untracked file).
- `xclconfig:example/plugin/{internal/plugin.go,external/main.go,resources/resources.go,config/main.xcl,config/modules/db/db.xcl}` — plugins, types and config the library-behaviour assertions depend on.
- `xclconfig:example/prettylog/prettylog_test.go:222-260` — imports `internal/parser` (`parser.TestPlugin`) and `internal/test_fixtures/registered`, and reads `../../internal/test_fixtures/config/encode/main.xcl`; all three must be replaced for prettylog to stand alone.
- `xclconfig:example/prettylog/{prettylog.go,highlight.go,highlight_test.go}` — public imports only plus charmbracelet; `highlight*` untracked (encoder-highlighting spec later removes it).
- `xclconfig:static_examples_test.go` — staged for deletion; root-level source inspection of `example/*/main.go` (`HEAD:static_examples_test.go`).
- `xclconfig:internal/testutil/programs.go:22,36` — `BuildProgram`/`RunProgram`, used only by the example smoke tests and `config_delivery_test.go`.
- `xclconfig:go.mod` — single module, `go 1.25.0`; working tree needs `go mod tidy` (`go vet ./example/...` reports "updates to go.mod needed").
- `xclconfig:.gitignore` — already ignores `example/*/build/`.
- `xclconfig:README.md:65-233,500-513` — examples section and the portable-lookup claim.
- `xclconfig:plugins/example/e2e_test.go` — the library's own person-plugin end-to-end tests; candidate existing coverage for plugin-boundary behaviours in the coverage map.

## External references

- Go modules reference, "replace directives" and "internal packages" — a `replace github.com/jumppad-labs/xcl => ../..` makes the example build against the working tree; another module cannot import `github.com/jumppad-labs/xcl/internal/...`.
- `go help packages` — directories named `testdata` are ignored by `./...`, so `.xcl` fixtures go under `e2e/testdata/`, while Go fixture packages that must be built go under `e2e/fixtures/`. Nested modules are excluded from the parent's `./...`, which is why the root `go test ./...` no longer reaches example packages directly.

## Prior plans / specs consulted

- `20261003153421-9fa72edd-masking` (plan) — established the former per-spec CHANGELOG convention (top section per spec, breaking list; since replaced by one epic-level entry written after all specs are implemented) and that the examples gained state encryption and plaintext-warning tests, which are among the behaviours being carried over.
- Sibling specs in epic `20261006071139-7b266535-examples-and-output`: `…-configuration-example` and `…-docker-plugin-example` depend on this spec and rewrite configonly and plugin as real applications (own modules, `replace`, stdlib smoke tests, no test-only seams); `…-encoder-syntax-highlighting` removes prettylog's highlighter and requires the library to stay free of the charmbracelet libraries.

## Open assumptions

- The uncommitted working tree (new `internal/testutil`, untracked smoke tests, configonly mid-rewrite, deleted `static_examples_test.go`/`testutils_test.go`, deleted `example/appconfig`) is the starting point and will be committed or settled before implementation starts. If configonly's `main_test.go` still does not compile at that point, the implementer must STOP and ask rather than guess the intended `run` signature.
- The library-behaviour tests to carry over are those in the example test files at implementation start **plus** any present at `HEAD` (8271816) that the working tree has already dropped; if the two differ materially the implementer reconciles both into the coverage map.
- The Go module cache (or network) can resolve the example modules' third-party dependencies when `go mod tidy` runs inside each example.
- `go` is on `PATH` wherever `go test ./...` runs (it must be, to run the tests at all).

## Drafting assumptions

### Chosen direction: e2e package in the root module, examples as replace-pointed modules (architecture)
- **Decision**: `e2e/` is a test-only package of xcl's root module with its own fixtures (`e2e/testdata/` configs, `e2e/fixtures/` Go types and plugins); each example becomes its own module with `replace` to `../..` (and to `../prettylog`); e2e runs each example's `go test ./...` from one named test per example; source-inspection tests are deleted; minimum-Go CI builds each example module.
- **Rationale**: Runs under the existing `go test ./... -race`; satisfies the one-module-per-example and public-packages-only constraints; keeps import paths unchanged so the sibling rewrites start from a working layout.
- **Rejected**: separate e2e module (outside root CI), `go.work` (contradicts the `replace` steer), keeping examples in the root module (violates constraint).

### No glob guard over example directories (architecture)
- **Decision**: The e2e suite names each example explicitly and does not glob `example/*` to detect unlisted examples.
- **Rationale**: Globbing another directory from a test is what `conventions/testing-and-mocking.md` forbids; a new example adds its own named e2e test as part of its spec.
- **Rejected**: a guard comparing discovered `example/*/go.mod` to the named tests.

### Conventions selected (architecture)
- **Decision**: Applied testing-and-mocking, shared-test-helpers, test-state-from-real-apply, assert-ordering-on-graph-parents, dependencies, code-style, never-modify-dependencies. Dropped database-and-external-services, patterns-and-architecture, development-standards (logging), project-structure, shared-errors-package and the entity glossary as not bearing on test and module layout.
- **Rationale**: Only the selected ones constrain test layout, fixtures, module files or dependency handling.
- **Rejected**: listing every convention.

### e2e may import internal/testutil (discovery)
- **Decision**: The e2e suite imports no xcl package under `internal/` except `internal/testutil`, which is test infrastructure rather than xcl's library surface.
- **Rationale**: `conventions/shared-test-helpers.md` forbids copying shared helpers (event recorder, stream capture) into each package; the acceptance criterion "uses xcl only through its public interface" is about the library under test.
- **Rejected**: copying the helpers into e2e (breaks the convention); importing other internal packages (breaks black-box testing).

### Example smoke tests use local standard-library helpers (discovery)
- **Decision**: Each example's smoke test builds and runs its binary with `os/exec` directly, inside that example; no helper is shared between examples.
- **Rationale**: A separate module cannot import `internal/testutil`; the spec constraint says examples use only public packages, and sibling specs say smoke tests use standard library process handling.
- **Rejected**: publishing a test helper package from xcl just for this (scope creep; sibling specs may later decide whether one is needed).

### No dependency-listing test for the library's charm-free boundary (discovery)
- **Decision**: No test lists or scans the repository's dependencies. That the library does not depend on the charmbracelet libraries follows from the examples becoming their own modules; a root `go mod tidy` drops `charmbracelet/*`, `muesli/termenv` and `kr/pretty`, and review of `go.mod` confirms it.
- **Rationale**: `conventions/testing-and-mocking.md` forbids tests that inspect repository files or `go list` the repository's own source to enforce a rule; such rules are checked by review and behaviour tests.
- **Rejected**: a root test asserting the dependency listing (with or without a sanity anchor on `github.com/hashicorp/go-plugin`); running `go list` inside `example/prettylog` from a root test.

### Baseline for coverage carry-over (discovery)
- **Decision**: The coverage map covers every library-behaviour test in the example test files at implementation start, plus any present at HEAD 8271816 that the uncommitted working tree has already dropped.
- **Rationale**: The working tree is mid-edit; mapping only the current files could silently lose coverage already removed.
- **Rejected**: mapping only HEAD (misses tests added in the working tree).

### Coverage map is a Markdown file in e2e/ (components)
- **Decision**: The coverage map is `e2e/COVERAGE.md`, one row per removed test: source example, removed test name, covering test (package and name).
- **Rationale**: The acceptance criterion asks for a map "kept with the end-to-end suite"; Markdown is reviewable and needs no tooling.
- **Rejected**: a Go test that parses the map (would be a test inspecting files rather than behaviour, and adds nothing the review does not).

### Runner tested against fixture modules (testing_approach)
- **Decision**: The example runner is split into a function returning an error and a per-example wrapper; it is tested against two tiny fixture modules under `e2e/testdata/` (one passing, one failing) so "a broken example fails the suite and names it" is automated.
- **Rationale**: Automates an acceptance criterion without breaking a real example; `testdata` keeps the fixture modules out of `./...`.
- **Rejected**: leaving it as a manual check only.

## Rehydration cues

- `spektacular spec file read 20261006071142-506b8289-e2e-suite-and-real-world-examples`
- `spektacular knowledge always-applied --tier repo --filter xclconfig` (testing-and-mocking, shared-test-helpers, test-state-from-real-apply)
- `grep -rn '^func Test' example/` and `git show HEAD:example/configonly/main_test.go`, `git show HEAD:example/plugin/main_test.go`
- Read `example/plugin/main_test.go:30-160`, `example/prettylog/prettylog_test.go:215-260`, `go.mod`, `.github/workflows/go.yml`, `README.md:215-233`.
