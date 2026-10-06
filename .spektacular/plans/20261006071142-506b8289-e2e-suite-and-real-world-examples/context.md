---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Context: 20261006071142-506b8289-e2e-suite-and-real-world-examples

## Current State Analysis

- xcl is a single Go module (`go.mod`, `go 1.25.0`); the three examples (`example/configonly`, `example/plugin`, `example/prettylog`) are packages of it, so the root `go test ./...` runs their tests directly and xcl's module carries example-only dependencies (`charmbracelet/lipgloss`, `charmbracelet/log`, `muesli/termenv`, `kr/pretty`).
- The examples double as xcl's end-to-end tests: `example/configonly/main_test.go` (46 tests) and `example/plugin/main_test.go` (48 tests) mostly assert library behaviour (decoding, events, plugin logging, destroy, masking), with six tests that parse example source via `go/ast` and a handful that check what the example prints.
- `static_examples_test.go` (root source inspection of `example/*/main.go`) is already staged for deletion in the working tree.
- Example smoke tests (`example/*/smoke_test.go`, untracked) and `example/plugin/main_test.go` use `internal/testutil`; prettylog's tests use `internal/parser.TestPlugin`, `internal/test_fixtures/registered` and `../../internal/test_fixtures/config/encode/main.xcl`.
- CI (`.github/workflows/go.yml`) runs `go test -v ./... -race` on Go 1.27.0 and builds/vets the root on Go 1.25.0.
- The working tree is mid-edit (configonly program rewritten, its test file not yet matching; `go.mod` needs tidying; appconfig example deleted). See Open Questions in plan.md.

## Per-Task Technical Notes

All tasks are in repo `xclconfig`, root `/home/nicj/code/github.com/jumppad-labs/xcl`. Requirement-to-repo-and-files resolution:

- *xcl has its own end-to-end suite* → `e2e/` (new): tasks "Build the e2e fixtures", "Carry the configuration-only behaviours…", "Carry the plugin behaviours…".
- *No coverage lost* → `e2e/COVERAGE.md`, `example/configonly/main_test.go`, `example/plugin/main_test.go`: "Write the coverage map", both "Strip…" tasks.
- *The suite runs the examples* → `e2e/examples_test.go`, `e2e/testdata/{passingexample,failingexample}/`: "Run the example tests from the e2e suite".
- *No tests of how examples are written* → `example/configonly/main_test.go`, `example/plugin/main_test.go`, `static_examples_test.go`, `go.mod`, `.github/workflows/go.yml`: both "Strip…" tasks, "Make each example its own module", "Build the examples on the minimum supported Go in CI".
- *prettylog is self-contained* → `example/prettylog/`: "Give prettylog fixtures of its own", "Make each example its own module".

Baseline note for every task touching example tests: the working tree is mid-edit. `example/configonly/main_test.go` calls `run(out, …)` but working-tree `example/configonly/main.go:109` is `run(handler, r, dir, stateDir, stateKey)`. If the file still does not compile when implementation starts, STOP and ask. Read the HEAD versions with `git show 8271816:example/configonly/main_test.go` and `git show 8271816:example/plugin/main_test.go` alongside the current files.

### Task: Build the e2e fixtures

**File changes**:
- `e2e/doc_test.go` (new) — package `e2e_test` with a package comment: black-box, whole-library tests that use xcl only through public packages and their own fixtures; the only internal import allowed is `internal/testutil`.
- `e2e/main_test.go` (new) — `TestMain` building `./fixtures/externalplugin` into `os.MkdirTemp` with `go build -o`, storing the path in a package var, removing the dir after `m.Run()`. Model: `example/plugin/main_test.go:45-66`.
- `e2e/fixtures/resources/resources.go` (new) — Go block types reproducing `example/configonly/resources/resources.go:1-166` (config_map, secret, deployment with nested/repeated blocks, service, ingress) and `example/plugin/resources/resources.go:1-89` (postgres, redis, app, ingress). Embed `types.ResourceBase`; `xcl` tags only. Split into `fixtures/kube` and `fixtures/plugintypes` packages if the two `ingress` types clash.
- `e2e/fixtures/inprocess/plugin.go` (new) — in-process plugin providing postgres and redis, modelled on `example/plugin/internal/plugin.go:1-204` (same Init debug message "registering block types", provider Init/Create/Destroy logs via `plugins.Logger(ctx)`, connection-string fill, published values).
- `e2e/fixtures/externalplugin/main.go` (new) — external plugin binary providing app and ingress, modelled on `example/plugin/external/main.go:1-172` (go-plugin serve, computed URL, provider logs).
- `e2e/testdata/kube/*.xcl` (new) — copy of the configonly config shape (`example/configonly/config/{deployment,ingress,secret}.xcl`, or `HEAD:example/configonly/config/main.xcl` if those are not final) with variables `image_tag`, `replicas`, output `api_url`, and a secret read with `env("DB_PASSWORD")`.
- `e2e/testdata/plugin/main.xcl`, `e2e/testdata/plugin/modules/db/db.xcl` (new) — copy of `example/plugin/config/main.xcl:1-76` and `example/plugin/config/modules/db/db.xcl:1-16`.
- `e2e/fixtures_test.go` (new) — two smoke tests of the fixtures: applying the kube config with types registered succeeds; applying the plugin config with the in-process plugin and the built external plugin succeeds.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: 2 parallel agents (kube fixtures; plugin fixtures + TestMain), then one integrates `fixtures_test.go`. Imports allowed in `e2e/fixtures/**`: root `xcl`, `types`, `plugins`, `plugins/registry`, `logger`, `hashicorp/go-plugin` — never `internal/`.

### Task: Carry the configuration-only behaviours into the e2e suite

**File changes**:
- `e2e/decode_test.go` (new) — from `example/configonly/main_test.go`: `FindsDeclaredResources:95`, `ReturnsRegisteredGoTypes:107`, `DecodesRepeatedBlocks:126`, `DecodesNestedBlocks:145`, `LeavesOmittedBlockNil:165`, `ReadsValuesFromConfigMap:175`, `ReadsVariables:190`, `LinksServiceToDeployment:200`, `LinksIngressToService:214`.
- `e2e/events_test.go` (new) — `ReportsParseEventWithFile:345`, `ReportsCreateSuccessWithoutStart:370`, `ReportsNoLogEvents:385`, `ReportsNoErrors:395`, `ReportsEveryEventFromCore:406`, `ReportsOperationAndPhaseOnEveryEvent:420`, `ReportsParseErrorWithFile:434` (needs a broken config in `e2e/testdata/broken/`), `ReportsDestroySuccessWithoutStart:493`, `ReportsDestroyOperationStartAndSuccess:514`.
- `e2e/destroy_test.go` (new) — `DestroysEverythingItApplied:461`.
- `e2e/silence_test.go` (new) — `TestRunWithoutReceiverWritesNothingToStdoutOrStderr:657` using `testutil.CaptureStandardStreams` (`internal/testutil/streams.go:23`).
- `e2e/sensitive_test.go` (new) — `EntityAndStateAgree:740`, `DoesNotWarnAboutPlainState:782`, `ReadsTheSecretFromTheEnvironment:799`, `ReferencesTheSecretByNameAndKey:814`, `StateHoldsNoSecret:852`, `EventDataHoldsNoSecret:875`, `WithoutKeyWarnsAboutPlainState:901`. Use `t.Setenv("DB_PASSWORD", …)`; state key fixed 32 bytes as in `main_test.go:35`.
- `e2e/encode_test.go` (new) — `ShowsCreatedEntities:690` is prettylog-rendered; carry over its library half (`xcl.EncodeSavedEntity` of each created entity's event data contains `config_map "api" {` etc.) and leave the prettylog half to prettylog's `TestHandlerWritesConfigurationAfterCreateSuccess`.
- Line numbers are the working-tree file; reconcile with `HEAD` (test names differ by `TestConfigOnlyExample` prefix). Each e2e test drives `registry.NewPluginRegistry` → `RegisterType` → `xcl.NewConfig(WithPluginRegistry, WithStatePath(t.TempDir()), WithStateMask/none, WithEventHandler(recorder.Record), WithEventData(xcl.EventDataProcessed))` → `Apply("testdata/kube")` → `Entities`/`Decode`/`Destroy`, mirroring `example/configonly/main.go:109-175`. Local queries over `testutil.EventRecorder` (`internal/testutil/events.go:19-36`) stay in the e2e package.
- Name tests for the xcl behaviour, e.g. `TestDecodeFillsNestedBlocks`, `TestParseEventCarriesFile`; record old→new names for the coverage map.

**Complexity**: Medium
**Token estimate**: ~50k tokens
**Agent strategy**: 2-3 parallel agents split by file (decode+destroy; events+silence; sensitive+encode), each writing its old→new name pairs to `.spektacular/tmp/coverage-configonly.md` for the map task.

### Task: Carry the plugin behaviours into the e2e suite

**File changes**:
- `e2e/plugin_values_test.go` (new) — from `example/plugin/main_test.go`: `FindsDeclaredResources:161`, `FillsConnectionString:181`, `PassesConnectionStringToReferencingBlock:193`, `PassesComputedURLToIngress:205`, `HoldsGeneratedTypes:217`, `RetrievesPublishedValues:820`.
- `e2e/plugin_logging_test.go` (new) — `ReportsInProcessPluginInitAtDebug:297`, `ReportsInProcessProviderInitAtDebug:319`, `ReportsInProcessProviderCreateLogsAsCreateEvents:339`, `ReportsExternalProviderCreateLogsFromThePluginBinary:390`, `ReportsExternalProviderCreateLogsBetweenStartAndSuccess:498`, `ReportsInProcessProviderCreateLogsBetweenStartAndSuccess:509`, `ReportsPluginLoadingLogsAtDebug:596`, `ReportsProviderCallLogsAtInfo:612`, `ReportsExternalProviderDestroyLogsBetweenStartAndSuccess:795`, `ReportsInProcessProviderDestroyLogsBetweenStartAndSuccess:806`.
- `e2e/plugin_events_test.go` (new) — `ReportsCreateEventPhases:476`, `ReportsPluginsLoaded:544`, `ReportsBlockTypesOfLoadedPlugins:560`, `ReportsNoErrors:573`, `ReportsNoWarnings:585`, `ReportsParseEventWithFile:632`, `ReportsDestroyEventPhases:747`, `ReportsDestroyOperationStartAndSuccess:769`, `InProcessProvidersReportDestroyForEveryResource:690`, `ExternalProvidersReportDestroyForEveryResource:725`, `DestroysEverythingItApplied:666`.
- `e2e/plugin_errors_test.go` (new) — `FailsWithoutExternalPlugin:424` (negative, own file/function).
- `e2e/plugin_state_test.go` (new) — `ConvertsExternalPluginTypes:971`, `EntityAndStateAgree:1025`, `EveryEntityConverts:1062`, `StateHoldsNoSecret:1162`, `EventDataHoldsNoSecret:1185`, `WithoutKeyWarnsAboutPlainState:1215`, and `RunWithoutReceiverWritesNothingToStdoutOrStderr:902` (may share `e2e/silence_test.go` under a distinct name).
- `ShowsPostgresConfigurationAfterCreate:936` and `ShowsEveryCreatedEntity:957`: carry the library half (`EncodeSavedEntity` on event data) into `e2e/encode_test.go`; the prettylog half stays with prettylog.
- Each test applies `testdata/plugin` with the in-process plugin registered via `registry.RegisterPlugin` and the external plugin path from `TestMain`, mirroring `example/plugin/main.go:120-252`. Ordering between unlinked resources asserted on graph parents per `conventions/assert-ordering-on-graph-parents.md`; between-start-and-success checks stay as event-index comparisons within one resource. Candidate existing coverage to cite instead of duplicating: `plugins/example/e2e_test.go`, `config_plugin_logging_test.go`, `config_plugin_loading_test.go`, `config_state_mask_test.go` — cite only if the test demonstrably fails when the behaviour breaks.

**Complexity**: High
**Token estimate**: ~70k tokens
**Agent strategy**: Parallel analysis by file group (values+errors; logging; events; state), sequential integration into the e2e package; each agent appends old→new pairs to `.spektacular/tmp/coverage-plugin.md`.

### Task: Write the coverage map

**File changes**:
- `e2e/COVERAGE.md` (new) — preamble (what the suite is, that it runs each example's tests through named runner tests, that a new example adds its own runner test); one table per example (`configonly`, `plugin`): removed test | behaviour | now covered by (`e2e.TestX` or `xcl.TestY` / `plugins/example.TestZ`); a final table of deleted source-inspection tests with reason: `TestConfigOnlyExampleImportsNoPluginCode` (`example/configonly/main_test.go:284`), `TestConfigOnlyExampleUsesPortableLookupForm:539`, `TestConfigOnlyExampleAssemblesItsConfigurationWithDecode:585`, `TestConfigOnlyExampleMakesNoPerTypeLookups:617`, `TestPluginExampleDefinesNoTypesOrConfig` (`example/plugin/main_test.go:248`), `TestPluginExampleUsesPortableLookupForm:853`, and every test in `HEAD:static_examples_test.go`. Note that the portable-lookup guarantee is now held by the CI minimum-Go build of each example.
- Build from `.spektacular/tmp/coverage-*.md`; delete those scratch files afterwards.
- Cross-check: `grep -n '^func Test' example/configonly/main_test.go example/plugin/main_test.go` plus HEAD versions — every name is in the map either as library behaviour, source inspection, or (in a third short list) "stays with the example" (output/exit tests: configonly `PrintsEveryResource:228`, `PrintsNestedBlocks:241`, `PrintsLinkedResources:261`, `FailsForMissingConfig:274`, `PrintsNoResourcesRemaining:481`, `PrintsNoSecret:829`; plugin `PrintsEveryResource:170`, `FailsForMissingConfig:238`, `PrintsNoResourcesRemaining:678`, `PrintsPublishedTotal:834`, `PrintsNoSecret:1092`, and the prettylog-render halves of the `Shows…` tests).

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Strip library and source tests from the configuration-only example

**File changes**:
- `example/configonly/main_test.go` — delete every test the map lists under configonly as library behaviour or source inspection; delete now-unused helpers (`eventRecorder`, `runRecordingEvents`, `deployment`, `findResource`, `resourceIDs` if unused, `methodFormLookups`) and imports (`go/ast`, `go/parser`, `go/token`, `internal/testutil`, `events`, `state`, `encoding/json`…). For each `Shows…` test, keep only the prettylog-render assertions if the example still prints them; otherwise remove and list in the map.
- `static_examples_test.go` — confirm deleted (already staged `D` in the working tree); `testutils_test.go` likewise.
- Tests left must compile against the working-tree `run` signature.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Strip library and source tests from the plugin example

**File changes**:
- `example/plugin/main_test.go` — delete every test the map lists under plugin as library behaviour or source inspection; delete now-unused helpers (`eventRecorder`, `logEvents`, `logRecord`, `logRecords`, `runRecordingEvents`, `declaredResourceIDs` if unused) and imports (`go/ast`, `go/parser`, `go/token`, `internal/testutil`, `state`, `events`, `encoding/json`). Keep `TestMain:45-66` (remaining output tests still need the external plugin).

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Give prettylog fixtures of its own

**File changes**:
- `example/prettylog/prettylog_test.go:215-260` — replace `registered.Database`/`registered.Cache` (`internal/test_fixtures/registered`) with test-local types embedding `types.ResourceBase`; replace `parser.TestPlugin` (`internal/parser`) with a test-local in-process plugin using `plugins.PluginBase` + `plugins.RegisterResourceProvider` providing the network and container types and filling the value `TestHandlerWritesConfigurationAfterCreateSuccess:316` asserts; replace `filepath.Abs("../../internal/test_fixtures/config/encode/main.xcl")` with `testdata/encode/main.xcl`.
- `example/prettylog/fixtures_test.go` (new) — the local types and plugin, if they make `prettylog_test.go` too long.
- `example/prettylog/testdata/encode/main.xcl` (new) — copy of `internal/test_fixtures/config/encode/main.xcl` trimmed to the types above.
- Replace the manual `os.Setenv("HOME", …)` at `prettylog_test.go:240-245` with `t.Setenv`.
- `example/prettylog/highlight_test.go` — no change (public deps only).

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential.

### Task: Make each example its own module

**File changes**:
- `example/prettylog/go.mod`, `go.sum` (new) — `module github.com/jumppad-labs/xcl/example/prettylog`, `go 1.25.0`, require xcl, `replace github.com/jumppad-labs/xcl => ../..`; `go mod tidy`.
- `example/configonly/go.mod`, `go.sum` (new) — as above plus require/replace `github.com/jumppad-labs/xcl/example/prettylog => ../prettylog`; brings `github.com/kr/pretty`.
- `example/plugin/go.mod`, `go.sum` (new) — as configonly; brings `hashicorp/go-plugin`.
- `example/configonly/smoke_test.go:1-28` — replace `testutil.BuildProgram`/`RunProgram` with `exec.Command("go", "build", "-o", filepath.Join(t.TempDir(), "configonly"), ".")` and `exec.Command(binary, args...)` capturing stdout/stderr into `bytes.Buffer`; same assertions.
- `example/plugin/smoke_test.go:1-30` — same, plus build the external plugin (reuse `TestMain`'s `externalPlugin`).
- `go.mod`, `go.sum` — root `go mod tidy`; expect `charmbracelet/lipgloss`, `charmbracelet/log`, `muesli/termenv`, `kr/pretty` and their indirects to drop.
- `.gitignore` — no change needed (`example/*/build/` already ignored).
- Verify: root `go build ./... && go vet ./... && go test ./...` and in each example `go test ./...`; review the tidied root `go.mod` for the dropped dependencies. Add no test that lists or scans the repository's dependencies.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential (module order: prettylog, then configonly and plugin).

### Task: Run the example tests from the e2e suite

**File changes**:
- `e2e/examples_test.go` (new) — `exampleTestsError(dir string) error` running `go test ./...` with `cmd.Dir = dir`, returning an error `"example %s: tests failed: %w\n%s"` with combined output; `runExampleTests(t testing.TB, name string)` calling it on `filepath.Join("..", "example", name)` and `require.NoError`. Tests: `TestConfigOnlyExampleTestsPass`, `TestPluginExampleTestsPass`, `TestPrettylogExampleTestsPass`; `TestExampleRunnerPassesForAPassingModule` (dir `testdata/passingexample`) and `TestExampleRunnerNamesAFailingModule` (dir `testdata/failingexample`, asserts error contains `failingexample`). Fail (not skip) if `exec.LookPath("go")` errors.
- `e2e/testdata/passingexample/{go.mod,pass_test.go}` (new) — stdlib-only module, one passing test.
- `e2e/testdata/failingexample/{go.mod,fail_test.go}` (new) — stdlib-only module, one test that calls `t.Fatal`.
- Consider `t.Parallel()` on the three example tests to keep the run time down; the plugin example builds its own external plugin.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Build the examples on the minimum supported Go in CI

**File changes**:
- `.github/workflows/go.yml:57-65` — in `build-minimum-go`, after the root build and vet, add a step looping over `example/configonly example/plugin example/prettylog` running `go build ./...` and `go vet ./...` in each (`working-directory` per step or a shell loop). Update the job comment to say examples are built here because they are separate modules.
- No test reads the workflow file; the change is reviewed by hand.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential.

### Task: Document the new layout

**File changes**:
- `README.md:229-233` — replace "The tests for all three run as part of `go test ./...`…" with: each example is its own Go module pointed at this checkout with a `replace`; run its tests with `make test` in its directory; xcl's `go test ./...` runs them through the e2e suite (`e2e/`), which also holds xcl's own end-to-end tests. Leave the rest of the examples section (appconfig, configonly and plugin prose) to the sibling specs.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential.

## Testing Strategy

- **Build the e2e fixtures** — two fixture smoke tests (each configuration applies through the public API); the external plugin build in `TestMain` fails the run loudly if it cannot build.
- **Carry the configuration-only behaviours / Carry the plugin behaviours** — one e2e test per carried-over behaviour, same assertion as the example test it replaces, testify `require`, no tables, negative cases (parse error, missing external plugin, plaintext warning without a key) in their own functions, state from a real apply, graph-parent ordering for unlinked resources. Each implementer breaks the behaviour locally (or reasons from the assertion) to confirm the test would fail.
- **Write the coverage map** — no executable test; reviewed manually (Manual — captured in the implementation test plan).
- **Strip… tasks** — the remaining example tests and the whole root test run pass; no `go/ast`/`go/parser` import remains in example tests.
- **Give prettylog fixtures of its own** — every existing prettylog test still passes with prettylog-owned fixtures.
- **Make each example its own module** — `go test ./...` passes in each example directory and in the root; after the root `go mod tidy`, review confirms `go.mod` no longer lists `charmbracelet/*`, `muesli/termenv` or `kr/pretty`.
- **Run the example tests from the e2e suite** — three per-example runner tests, plus `TestExampleRunnerPassesForAPassingModule` and `TestExampleRunnerNamesAFailingModule` against `e2e/testdata` fixture modules.
- **Build the examples on the minimum supported Go in CI** — no executable test; the workflow change is reviewed by hand and the CI job proves it on push.
- **Document the new layout** — no executable test; the README passage is reviewed by hand.
- Manual checks (all **Manual — captured in the implementation test plan**): prettylog builds alone against a published xcl; coverage-map review; breaking mapped behaviours fails a covering e2e/library test; a stylistic rewrite of an example leaves the run green; fresh-checkout run passes and e2e imports no internal package but `internal/testutil`.

## Project References

- Spec: `20261006071142-506b8289-e2e-suite-and-real-world-examples` (epic `20261006071139-7b266535-examples-and-output`).
- Sibling specs: `20261006112023-aadf3c10-configuration-example`, `20261006112023-f7a185dc-docker-plugin-example` (depend on this), `20261006112108-17623cda-encoder-syntax-highlighting` (independent; shares prettylog).
- Design documents: none.
- Knowledge (repo `xclconfig`): `conventions/testing-and-mocking.md`, `conventions/shared-test-helpers.md`, `conventions/test-state-from-real-apply.md`, `conventions/assert-ordering-on-graph-parents.md`, `conventions/dependencies.md`, `conventions/code-style.md`, `conventions/never-modify-dependencies.md`.
- Repo root: `xclconfig` → `/home/nicj/code/github.com/jumppad-labs/xcl`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The two carry-over tasks are the bulk of the work (~120k tokens together); split them by test file as noted per task, and keep old→new test-name pairs in `.spektacular/tmp/` for the coverage-map task.

## Migration Notes

- Contributors who ran an example's tests with `go test ./example/...` from the root now run `go test ./...` (or `make test`) inside the example, or rely on the root `go test ./...` which runs them through the e2e suite.
- After the root `go mod tidy`, xcl no longer requires the charmbracelet libraries, `muesli/termenv` or `kr/pretty`; applications importing xcl lose those indirect requirements.

## Performance Considerations

- The root test run now shells out to `go test` in three example modules; the plugin example builds its external plugin again inside its own run. Running the three runner tests with `t.Parallel()` keeps the added wall time close to the slowest example. The e2e suite builds its own external plugin once in `TestMain`.
