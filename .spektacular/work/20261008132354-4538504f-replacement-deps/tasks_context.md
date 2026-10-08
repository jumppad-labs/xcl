### Task: Change contract through every plugin layer

**Requirements covered:** Plugins can say a change needs a rebuild; External plugins can decide replacements; Every plugin in the repository uses the new decisions (mechanical part). Repo: xclconfig.

**File changes**
- `plugins/change.go` (new) — `type Change int`, `NoChange`/`Update`/`Replace`, `String()`; `type DependencyChange struct{ Address string; Change Change }`. Doc comments follow the design text.
- `plugins/provider.go:76-99` — `Changed(ctx, old, new T, dependencies []DependencyChange) (Change, error)`. Reword Update's doc ("called when Changed answers Update") and Changed's doc (what each answer causes; dependencies only list direct Update/Replace deps).
- `plugins/changed.go:23-39` — `DefaultChanged[T].Changed(ctx, old, new T, dependencies []DependencyChange) (Change, error)`: `Update` if comparable JSON differs, else `NoChange`. Ignore dependencies, and document that.
- `plugins/adapter.go:32` — `ProviderAdapter.Changed(ctx, old, new []byte, dependencies []DependencyChange) (Change, error)`.
- `plugins/adapter.go:207-223` — `TypedProviderAdapter.Changed` passes dependencies through.
- `plugins/plugin.go:57` and `:202-210` — `PluginEntityProvider.Changed` / `PluginBase.Changed` gain dependencies and return `Change`.
- `plugins/plugin_host.go:28` — `PluginHost.Changed` same.
- `plugins/direct_plugin_host.go:133-135`, `:167-170` — pass-through.
- `plugins/grpc_resource_adapter.go:52-54` — pass-through.
- `plugins/plugin.proto:99-109` — add `enum Change { CHANGE_NO_CHANGE=0; CHANGE_UPDATE=1; CHANGE_REPLACE=2; }` and `message DependencyChange { string address=1; Change change=2; }`. `ChangedRequest` gains `repeated DependencyChange dependencies = 5`. `ChangedResponse`: `reserved 1; string error = 2; Change change = 3;`.
- `plugins/proto/plugin.pb.go`, `plugins/proto/plugin_grpc.pb.go` — regenerate. From the repo root, run the protoc command in `Makefile` (`protos` target) with `--plugin=protoc-gen-go=$(go run ... )`. Simplest is `GOBIN=$(mktemp -d) go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1`, then put that dir first on PATH. Never hand-edit the generated files.
- `plugins/grpc_plugin_host.go:292-312` — `grpcPluginWrapper.Changed` converts `[]DependencyChange` to `[]*proto.DependencyChange`, returns `fromProtoChange(resp.Change)`.
- `plugins/grpc_plugin_host.go:393-399` — `GRPCPluginHost.Changed` same signature.
- `plugins/grpc_server.go:144-155` — `GRPCServer.Changed` converts `req.Dependencies` to Go and the answer to proto.
- `plugins/change_proto.go` (new, unexported helpers) — `toProtoChange`, `fromProtoChange`, `toProtoDependencies`, `fromProtoDependencies`. An unknown enum value maps to an error, not to NoChange.
- `plugins/testing/helpers.go:139-157`, `:200-203` — call with nil dependencies and `require.Equal(t, plugins.NoChange, change)`.
- `plugins/mocks/mock_provider_adapter.go` — regenerate with `go run github.com/vektra/mockery/v3@v3.8.0` from the repo root (`.mockery.yml`). Also fix the `Makefile` `install-mockery` target to v3 if trivial.
- `internal/parser/test_plugin.go:93,309,379` — `ChangedResults map[string]plugins.Change`, `SetChangedResult(id string, change plugins.Change)`.
- `internal/parser/test_plugin.go:644-671` — new signature, returning the configured `Change` or the DefaultChanged answer.
- `internal/parser/lifecycle.go:317-329` — add `refreshReplace` outcome.
- `internal/parser/lifecycle.go:416-436` — call `adapter.Changed(ctx, copies.old, copies.read, nil)` and map `Update`→`refreshChanged`, `Replace`→`refreshReplace`, `NoChange`→`refreshUnchanged`.
- `internal/parser/lifecycle.go:443-487` (`read`) — on `refreshReplace`, call `l.rebuild(r, old, adapter)` (temporary; removed by the act-pass task).
- `internal/parser/lifecycle.go:228-243` (`diffResource`) — `refreshReplace` → `recordPending(meta, diff.ActionReplace, old, r)`.
- `internal/parser/lifecycle_test.go:412-490` — `SetChangedResult(..., true)` → `plugins.Update`, `false` → `plugins.NoChange`. Add `TestOverriddenChangeDetectionCanReportAReplace` (destroy then create calls).
- `internal/parser/diff_test.go`, `diff_update_unknown_test.go` — same knob migration.
- `plugins/changed_test.go:60-117`, `plugins/changed_sensitive_test.go:26,35,58` — assert `plugins.Update`/`plugins.NoChange`. Add `TestDefaultChangedIgnoresDependencies`.
- `plugins/example/e2e_test.go:230,296,486-568` — assert `Change` values; pass nil dependencies.
- `plugins/grpc_plugin_host_test.go` — add `fakeChangedServiceClient` (embeds `proto.PluginServiceClient`, captures `*proto.ChangedRequest`). New tests:
  - `TestGRPCPluginWrapperChangedSendsDependencies`
  - `TestGRPCPluginWrapperChangedReturnsReplace`
  - `TestGRPCPluginWrapperChangedReturnsError`
  - `TestGRPCPluginWrapperChangedRejectsUnknownChange`
- `plugins/adapter_test.go` — add `TestTypedProviderAdapterChangedPassesDependenciesToProvider` and `TestDirectPluginHostChangedReturnsProviderAnswer` (recording fake provider).
- `plugins/change_test.go` (new) — `Change.String` tests, one per value.
- Every DefaultChanged embedder compiles unchanged:
  - `example/plugin/plugins/docker/resources/{network,container}.go`
  - `example/plugin/plugins/template/template.go`
  - `example/prettylog/fixtures_test.go`
  - `plugins/example/pkg/person/provider.go`
  - `e2e/fixtures/{externalplugin,inprocess}`
  - `internal/test_fixtures/plugins/subtypeless/main.go`
  - test fakes listed in research.md

  Confirm with `go build ./...` and `go vet ./...` in the root, `example/plugin`, `example/prettylog` and `example/configonly`.

**Complexity:** High
**Token estimate:** ~90k tokens
**Agent strategy:** Parallel analysis, sequential integration.
1. One agent does `plugins/` (types, layers, proto regen, mock regen, tests).
2. Then a second agent does the core mapping and test-knob migration.
3. Finally, one integration pass builds and tests every module.

The build must be green at the end of the task, not in between.

### Task: Decision record and dependency resolver

**Requirements covered:** Resources learn what will happen to what they depend on (data side); dependency look-through (user decision). Repo: xclconfig.

**File changes**
- `internal/parser/diff_recorder.go` — rename the type to `decisions` (or keep `diffRecorder` and add the fields; either is fine as long as it is one type). Add:
  - `decision{action diff.Action; reason diff.ReplaceReason (string until the diff task lands — use a local unexported type and swap later, or add the diff fields here first); replacedDeps []string; dependencies []plugins.DependencyChange; read []byte}`
  - `map[string]decision`, plus `decide`, `lookup`, `toDestroy(previous *State) []any` (saved entities whose decision is replace or delete) and `result()`, which is unchanged in output.

  Keep `markPending`/`unknownPaths`/`contextValue` as they are.

  Note: if the diff task has not landed, keep the reason as an unexported `replaceReason` in the parser and map it when the diff task adds `diff.ReplaceReason`. Adding the diff fields first is also fine; the dependency chain allows either.
- `internal/parser/dependencies.go` (new) — `dependencyChanges(entity any, state *State, record *decisions) []plugins.DependencyChange`:
  - Start from `types.Meta.Links` (via `getResourceDependencies`, `internal/parser/util.go:513`, which expands module refs).
  - For each linked entity: if provider-backed (not `handledWithoutProvider` `lifecycle.go:667` and not a builtin output/variable/module/local), look up its decision. Otherwise recurse into that entity's own links (look-through), keeping a visited set.
  - Emit `{Address: meta.ID, Change: Update}` for a dependency decided update, and `{Address, Change: Replace}` for one decided replace. A dependency decided create is not listed: the design's vocabulary is Update/Replace only. A newly referenced resource already changes the dependent's configuration (DefaultChanged sees it), and its computed values are unknown, so the Update floor applies. Explain this in the code comment.
  - Deduplicate by address and sort by address.
- `internal/parser/dependencies_test.go` (new), using parser fixtures and a record populated by hand:
  - `TestDependencyChangesListsUpdatedAndReplacedDependencies`
  - `TestDependencyChangesOmitsUnchangedDependencies`
  - `TestDependencyChangesListsOnlyDirectDependencies` (chain first ← second ← third; third is not told about first)
  - `TestDependencyChangesLooksThroughModuleOutputs` (fixture `internal/test_fixtures/config/lifecycle/module_reference/`)
  - `TestDependencyChangesLooksThroughVariables`
- `internal/parser/diff_recorder_test.go` (new or existing) — `TestDecisionsToDestroyListsReplacedAndRemoved` and `TestDecisionsToDestroyOmitsUpdatedAndUnchanged`.
- Fixtures: add `internal/test_fixtures/config/lifecycle/two_dependencies/main.xcl` (network `replaced`, network `updated`, network `same`, container `user` whose `network` blocks reference all three by `meta.name`).

**Complexity:** Medium
**Token estimate:** ~45k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Decide pass tells each resource about its dependencies

**Requirements covered:** Resources learn what will happen to what they depend on; Each resource decides its own outcome; Decisions follow dependency order; Nothing changes until every decision is made (decide side). Repo: xclconfig.

**File changes**
- `internal/parser/diff_recorder.go` — rename `walkDiff` → `walkDecide` (keep `walkApply`).
- `internal/parser/lifecycle.go:55` — `operation()` must still return `OperationDiff` for a plan and `OperationApply` for an apply's decide pass. Carry the operation name on the lifecycle rather than deriving it from the mode.
- `internal/parser/lifecycle.go:159-246` (`diff`/`diffResource` → `decide`/`decideResource`):
  - not in state → `create`, as today;
  - failed status → `replace`, reason failed;
  - **remove the early return for unknown paths at `:218`**. Instead:
    1. Build the provider copy with saved values at the unknown paths: use `decodeCopy` with the saved copy, and set each unknown path from the saved value (helper in `diff_decode.go`).
    2. Call `refresh(r, old, adapter, deps)` with `deps := dependencyChanges(r, l.previous, l.recorder)`.
    3. Floor `refreshUnchanged` to update when the resource has unknown paths.
  - map `refreshReplace` → replace (reason dependency when any dep is `Replace`, with `replacedDeps` = those addresses; otherwise reason provider);
  - map `refreshChanged` → update, and `refreshUnchanged` → unchanged.

  Every outcome calls `record.decide(id, decision{..., dependencies: deps, read: copies.read})`. `markPending` stays for create, update and replace.
- `internal/parser/lifecycle.go:355-438` (`refresh`) — add the `deps` parameter and pass it to `adapter.Changed`. On a Read/Changed error, **do not** set `meta.Status = StatusFailed` in decide mode. The error propagates, and the whole operation fails.
- `internal/parser/callbacks.go:136-139,190-194` — walk dispatch uses `walkDecide`. Diff decode is used for decide in both plan and apply.
- `internal/parser/parser.go:365-414` (`Diff`) — calls a new `p.decide(ctx, operation, current, previous) (*decisions, error)` that builds the removed deletes and runs `walkWith(... walkDecide)`. `Diff` returns `record.result()`.
- `internal/parser/test_plugin.go` — add `ChangedDependencies map[string][]plugins.DependencyChange` (set in `TestResourceProvider.Changed`, `:644`), `GetChangedDependencies(id string) []plugins.DependencyChange`, and include it in `ResetCalls`.
- Tests (new `internal/parser/decide_test.go`, harness `setupLifecycle` from `lifecycle_test.go:69`, state via `applyAndSave`):
  - `TestDecideTellsResourceAboutReplacedAndUpdatedDependencies` (fixture `two_dependencies`; `SetChangedResult(replaced, Replace)`, `SetChangedResult(updated, Update)`; `GetChangedDependencies(user)` equals exactly those two)
  - `TestDecideDoesNotTellResourceAboutUnchangedDependency`
  - `TestDiffReportsProviderReplaceAsReplace`
  - `TestDiffReportsProviderUpdateAsUpdate`
  - `TestDiffKeepsDependentUnchangedWhenItsPluginSaysSo` (network Replace, container NoChange → container not in the result, counted unchanged)
  - `TestDiffFloorsResourceWithUnknownInputsAtUpdate` (fixture `lifecycle/computed_ref`: network replaced; container answers NoChange → planned update)
  - `TestDiffReadsResourceWithUnknownInputsUsingSavedValues` (ReadCalls carry the saved provider_id)
  - `TestDiffFailsWhenChangedFails` and `TestDiffFailsWhenReadFails` (error names the resource, `errors.Is` the provider error)
- Update `internal/parser/diff_update_unknown_test.go` tests that assert "never reads": they now assert that the read used saved values. Give the reason in the test comment ("decide pass reads every saved resource; unknowns carry saved values").

**Complexity:** High
**Token estimate:** ~70k tokens
**Agent strategy:** Single agent for the lifecycle change (tightly coupled). A second agent may write the new decide tests in parallel against the agreed `TestPlugin` API.

### Task: Act pass destroys first, then creates and updates

**Requirements covered:** Applying replaces resources safely; Nothing changes until every decision is made; A failed replacement is recorded honestly. Repo: xclconfig.

**File changes**
- `internal/parser/parser.go:264-345` (`Apply`):
  1. `record, err := p.decide(ctx, events.OperationApply, current, previous)`. On error, return `previous` unchanged and the error, before any destroyer is built.
  2. `targets := record.toDestroy(previousState)` (removed plus replaced saved entities).
  3. If any, build a `destroyer` over a working copy as today (`:283-315`) and `d.destroy(targets)`. On failure, return working state, as today.
  4. `p.walkWith(..., &resourceLifecycle{mode: walkApply, recorder: record, previous: working})`.
- `internal/parser/lifecycle.go:74-144` (`apply`/`run`) — `run` looks up `record.lookup(id)`:
  - create or replace → `create(r, adapter)`;
  - update → `carryComputedValues(r, decision.read)`, then `Update` via `callProvider` (code moved from `read` `:459-486`), with status updated or failed;
  - unchanged → `replaceValues(r, decision.read)` and keep the saved status.

  Provider-less entities are unchanged.
- `internal/parser/lifecycle.go:443-528` — delete `read` and `rebuild`; their behaviour now lives in decide plus the destroy phase. Remove the temporary `refreshReplace`→`rebuild` from the contract task.
- `internal/parser/destroy.go:56-103` — no mechanical change. Confirm that the destroy graph built from `targets` only links targets (`dag.go:62-130`), so a replaced network with a non-replaced container is destroyed alone. The doc comment on `Parser.Apply` describes the order.
- `internal/parser/callbacks.go:232-344` — destroy events in apply carry the apply operation as today.
- Tests (new `internal/parser/replace_test.go`, real applies via `applyAndSave`):
  - `TestApplyReplacesResourceWhenProviderAnswersReplace` (destroy then create for that ID; saved status created)
  - `TestApplyUpdatesResourceInPlaceWhenProviderAnswersUpdate` (no destroy)
  - `TestApplyDestroysDependentsBeforeDependenciesWhenReplacing` (fixture `lifecycle/dependent` trimmed to network.first ← container.second, or a new fixture `lifecycle/replace_pair/main.xcl`; both Replace; `GetCalls()` filtered to destroy/create equals `destroy container`, `destroy network`, `create network`, `create container`. This is allowed because they are linked, per the convention)
  - `TestApplyDecidesEverythingBeforeActing` (event collector: index of the last read/changed < index of the first destroy/create/update)
  - `TestApplyMakesNoProviderChangeWhenADecisionFails` (`SetChangedError` on one; no create/update/destroy calls; state file bytes unchanged)
  - `TestApplySavesFailedWhenCreatingAReplacementFails` then `TestDiffAfterFailedReplacementListsReplaceAgain` (separate functions; `SetCreateError`)
  - `TestApplySavesDestroyFailedWhenDestroyingAReplacementFails` (`SetDestroyError`; apply returns an error; status destroy_failed)
  - `TestApplyDoesNotReplaceDependentWhenItsPluginAnswersUnchanged`
- Update `internal/parser/lifecycle_test.go:775-933` rebuild tests: they still pass, with the event sequence destroy then create. Update `TestDependentsOfFailedRebuildAreNotProcessed` (the destroy now happens in the destroy phase; dependents' decide still happens). Give the reason in the test comment. Update `removal_test.go` only if the cross-resource order changes (removals are still destroyed before creates).
- `config.go:353-410` — no change expected. Verify that partial state on a destroy-phase failure is saved as before.

**Complexity:** High
**Token estimate:** ~75k tokens
**Agent strategy:** Single agent, sequential (one cohesive refactor), then one review agent that runs the full parser, root and e2e suites.

### Task: Person plugin replaces on a name change

**Requirements covered:** Every plugin in the repository uses the new decisions; External plugins can decide replacements (fixture). Repo: xclconfig.

**File changes**
- `plugins/example/pkg/person/provider.go:16` — add `Changed(ctx, old, new *Person, deps []plugins.DependencyChange) (plugins.Change, error)`: `Replace` if `FirstName` or `LastName` differ, otherwise `p.DefaultChanged.Changed(...)`.
- `plugins/example/pkg/person/provider_test.go` (new):
  - `TestPersonChangedReplacesOnFirstNameChange`
  - `TestPersonChangedReplacesOnLastNameChange`
  - `TestPersonChangedUpdatesOnEmailChange`
  - `TestPersonChangedReportsNoChangeForIdenticalPerson`
- `plugins/example/e2e_test.go:478-568` — add `TestInProcessPluginChangedReplacesOnNameChange` and `TestExternalPluginChangedReplacesOnNameChange`.
- `plugins/example/README.md:131-132` — mention the override.

**Complexity:** Low
**Token estimate:** ~15k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Plans agree with applies for built-in and external plugins

**Requirements covered:** External plugins can decide replacements; success metric "plan lists exactly what apply does". Repo: xclconfig.

**File changes**
- `e2e/diff_test.go:285-295` — `requireDiffPredictsApply`: add `requireDiffEqualsApply` (exact set equality per action, with destroy+create = replace via `applySets` `:232`) and use it in the new scenarios. Keep the subset helper where unknown-driven updates are legitimately skipped. Since the floor now makes those real updates, switch to exact where it now holds and note the change.
- `e2e/diff_test.go` new tests:
  - `TestDiffOfProviderReplacePredictsApply`
  - `TestDiffOfDependentReplacePredictsApply`
  - `TestDiffOfRemovalAndReplacePredictsApply`

  They use the e2e plugin fixtures with two small replace rules: in the in-process fixture, PostgreSQL `location` → Replace (connection identity, `e2e/fixtures/inprocess/plugin.go:70`); in the external fixture, Ingress `hostname` → Replace (`e2e/fixtures/externalplugin/main.go:117`). Each is a `Changed` override that otherwise defers to `DefaultChanged`, and is documented in a fixture comment. Each rule gets a positive and a negative test beside the fixture's existing e2e tests. Check `failed-resource-in-e2e-with-bad-host`: a `location` edit to "bad host" now plans as Replace, which is still a replace, so `TestDiffOfFailedReplicaPredictsReplaceByApply` keeps its meaning.
- `e2e/plugin_replace_test.go` (new) — the person plugin from `plugins/example`, in-process vs external (copy the helper pattern from `plugins/example/e2e_test.go`, or build the external binary as `e2e/main_test.go` does):
  - `TestPersonNameChangePlansTheSameInProcessAndExternal` (compare the `diff.Diff` JSON)
  - `TestPersonNameChangeAppliesTheSameInProcessAndExternal` (compare the lifecycle event sequences per ID and the final state entity values minus IDs/times)

  If `plugins/example` cannot be imported from e2e (it is in the root module, so it can be), use `e2e/fixtures` instead.
- `config_diff_test.go`, `config_test.go` (root) — `TestDiffReportsProviderDecidedReplacement` and `TestApplyPerformsProviderDecidedReplacement` through `NewConfig` with `TestPlugin`-like registration (follow `setupDiffConfig` `config_diff_test.go:22-75`).
- `internal/testutil/events.go` — if the e2e and root tests both need a "classify lifecycle events into create/update/replace/delete sets" helper, move it here (shared-helpers convention).

**Complexity:** Medium
**Token estimate:** ~50k tokens
**Agent strategy:** 2 parallel agents: one for the e2e diff scenarios, one for in-process vs external equivalence and the root tests.

### Task: Diff carries and renders the replacement reason

**Requirements covered:** Plans show replacements and why. Repo: xclconfig.

**File changes**
- `diff/diff.go:17-84` — add `type ReplaceReason string` with `ReplaceFailed`, `ReplaceProvider`, `ReplaceDependency`. Add `Reason ReplaceReason \`json:"reason,omitempty"\`` and `ReplacedDeps []string \`json:"replaced_dependencies,omitempty"\`` to `Resource`. Widen the `ActionReplace` doc (`:29-31`).
- `diff/render.go:222-238` (`actionPhrase`) — replace phrases by reason:
  - failed → "will be replaced, its last apply failed"
  - provider → "will be replaced, it cannot be updated in place"
  - dependency → "will be replaced because " + `strings.Join(deps, ", ")` + " is replaced" (" are replaced" when more than one)

  Update the doc comment at `:75-98`.
- `internal/parser/lifecycle.go` decide step and `diff_recorder.go` — set `Reason`/`ReplacedDeps` on the recorded `diff.Resource`. The status-based replace sets `ReplaceFailed`.
- `diff/render_test.go:50-56,259-260,380-386` — keep the failed phrase. Add:
  - `TestRenderReplaceBecauseDependencyNamesTheDependency`
  - `TestRenderReplaceBecauseOfSeveralDependenciesNamesThemAll`
  - `TestRenderReplaceByProviderSaysItCannotBeUpdatedInPlace`
- `diff/diff_test.go` — `TestReplaceResourceJSONIncludesReasonAndDependencies` and `TestUpdateResourceJSONOmitsReason`.
- `internal/parser/decide_test.go` — `TestDiffNamesReplacedDependencyAsReason` and `TestDiffMarksFailedResourceReplaceAsFailed`.
- `internal/parser/diff_unknown_test.go` — `TestDiffShowsComputedValuesOfReplacedDependencyAsUnknown` (fixture `computed_ref`, network Replace → the container's `network.name` change is unknown).

**Complexity:** Medium
**Token estimate:** ~30k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Docker and template providers decide replacements

**Requirements covered:** The plugin example replaces its network and container correctly (provider rules); success metric "every setting it cannot change in place answers replace". Repo: xclconfig.

**File changes**
- `example/plugin/plugins/docker/resources/network.go:33-37,100-104` — add `Changed`: `Replace` if `old.Subnet != new.Subnet`, else defer to `DefaultChanged`. The `Update` doc says it is only reached for changes that need no Docker call (labels/meta); keep the no-op.
- `example/plugin/plugins/docker/resources/container.go:60-64,242-246` — add `Changed`: `Replace` if any dependency has `Change == plugins.Replace`; `Replace` if `Image`, `Command`, `Environment` or `Networks` differ (`reflect.DeepEqual` on the slices/maps, treating nil and empty as equal); else `DefaultChanged`. Fix the `Update` doc to match.
- `example/plugin/plugins/template/template.go:44-46` — add `Changed`: `Replace` if `Destination` differs, else `DefaultChanged` (Update for source/variables).
- `example/plugin/plugins/docker/resources/network_test.go` — `TestNetworkChangedReplacesOnSubnetChange` and `TestNetworkChangedReportsNoChangeForIdenticalNetwork`.
- `example/plugin/plugins/docker/resources/container_test.go`:
  - `TestContainerChangedReplacesOnImageChange`, `...OnCommandChange`, `...OnEnvironmentChange`, `...OnNetworkChange`
  - `TestContainerChangedReplacesWhenNetworkIsReplaced`
  - `TestContainerChangedReportsNoChangeWhenNetworkIsOnlyUpdated`
  - `TestContainerChangedReportsNoChangeForIdenticalContainer`
- `example/plugin/plugins/template/template_test.go` — `TestTemplateChangedReplacesOnDestinationChange`, `TestTemplateChangedUpdatesOnSourceChange`, `TestTemplateChangedUpdatesOnVariablesChange`.
- No Docker client calls in `Changed`, so no mock expectations (the existing `mocks.NewMockDocker(t)` is unused for these tests).

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Plugin example ships its address-range change configuration

**Requirements covered:** The plugin example replaces its network and container correctly; The example ships a configuration that shows it; success metric "no drift after subnet apply". Repo: xclconfig.

**File changes**
- `example/plugin/config/alt.xcl` → `git mv` to `example/plugin/config-subnet/main.xcl`. Rewrite the header comment to say it is the main configuration with the network's address range changed to `10.42.0.0/23`, used to show a replacement. The rest is unchanged.
- `example/plugin/Makefile:10,26-29` — add `SUBNET_DIR := ./config-subnet` and a `replace` (or `run-subnet`) target: build, `apply ./config`, `plan ./config-subnet`, `apply ./config-subnet`, `status`, `destroy`.
- `example/plugin/main_test.go:65-94` — add `applyExampleDir(t, dir, statePath)` (generalising `applyExampleWithState`). Add, Docker-gated:
  - `TestSubnetChangeReplacesTheNetworkWithTheNewRange` (`client.NetworkInspect` → IPAM Config[0].Subnet == "10.42.0.0/23")
  - `TestSubnetChangeAttachesANewContainer` (the container DockerID differs from the first apply and is attached to the network)
  - `TestSubnetChangeRendersTemplateWithNewAddress`
  - `TestPlanAfterSubnetChangeReportsNoChanges`
- `example/plugin/plan_test.go` — `TestPlanOfSubnetChangeReplacesNetworkAndContainer`: rendered output contains `# docker.network.app will be replaced, it cannot be updated in place`, `-/+ docker "network" "app"`, `# docker.container.web will be replaced because docker.network.app is replaced`, `# template.welcome will be updated`, and the summary line `0 to create, 1 to update, 2 to replace, 0 to delete, 0 unchanged.`
- `example/plugin/smoke_test.go:52-90` — unchanged (it uses `./config`). Confirm it passes now that alt.xcl is gone.
- `example/plugin/plugins/docker/resources/docker_test.go` — optional real-Docker test of network replace via the provider; skip unless cheap.

**Complexity:** Medium
**Token estimate:** ~35k tokens
**Agent strategy:** Single agent, sequential execution. It must run with Docker available to see the gated tests execute. If Docker is not available, report that the gated tests skipped.

### Task: Core guides, README and changelog explain replacement

**Requirements covered:** Documentation explains replacement (core repo). Repo: xclconfig.

**File changes**
- `docs/plugin-developer-guide.md`:
  - `:13-43` — new `Changed` signature in the contract.
  - `:120-184` — lifecycle rewritten as decide then act; the destroy phase replaces "rebuild".
  - `:227-259` — rewrite the `Changed` section: the three answers, the dependency list (direct, Update/Replace only, look-through), DefaultChanged's answers, and the Docker container override as the example.
  - `:261-269` — `Update` is called only for an Update answer.
  - `:424-438` — statuses for a failed replacement.
  - `:485-490` — events: all read/changed before destroy/create/update.
- `docs/plugins.md:25-148` — signatures for `ResourceProvider`, `ProviderAdapter` and `PluginHost`, plus the proto messages.
- `docs/parser-lifecycle.md`:
  - `:6-56` — apply steps: decide, destroy, create/update.
  - `:83-129` — the decision tree.
  - `:282-300` — the event sequence.
- `docs/state.md:59-76` — status meanings: replacement destroy and create failures.
- `docs/README.md:16,21` — mention the Change answer.
- `README.md:225-300` — add the subnet plan walkthrough (`plan ./config-subnet` output with `-/+` and reason), and note that `./config` stands alone.
- `CHANGELOG.md:3` — new top entry `## 20261008132354-4538504f-replacement-deps`. It covers the new `Changed` signature and `Change`/`DependencyChange` types (breaking), the protocol change (breaking: external plugins must be rebuilt), saved state compatibility (not guaranteed), decide-then-act apply, the replacement reasons in diff/JSON, and the plugin example's subnet configuration. Follow the style of existing entries.

**Complexity:** Low
**Token estimate:** ~25k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Website guide on unchanged, update or replace

**Requirements covered:** Documentation explains replacement (site, plugin authors). Repo: xcl-website.

**File changes**
- `xcl-website:src/pages/replacement.mdx` (new) — the guide "Unchanged, update or replace". Sections: what `Changed` answers; what dependencies a resource is told about (direct only, update/replace only, look-through of outputs/modules); decide-then-act and destroy-first order; worked example with the Docker network (subnet → Replace) and container (replaced network → Replace) code taken from `example/plugin`; how the plan shows it (link to /diff/). Follow the frontmatter/layout pattern of `xcl-website:src/pages/registries.mdx`.
- `xcl-website:src/components/Nav.astro:8-29` — add "Replacement" (or "Unchanged, update or replace") to the Guides menu, linking `/replacement/`.
- `xcl-website:src/pages/index.mdx:166-168` — the FeatureCard mentioning Changed: point at the new guide.
- Verify with `npm run build` (and `make check`) in `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

**Complexity:** Low
**Token estimate:** ~20k tokens
**Agent strategy:** Single agent, sequential execution. Code samples are copied from the xclconfig sources after the example task lands.

### Task: Website diff and plugin example pages show replacements

**Requirements covered:** Documentation explains replacement (plans display replacements and reasons). Repo: xcl-website.

**File changes**
- `xcl-website:src/pages/diff.mdx`:
  - `:67-86` — the replace action now has three reasons.
  - `:117-172` — the rendered sample gains a dependency-replace resource (`# docker.container.web will be replaced because docker.network.app is replaced` above `-/+ docker "container" "web" {`), and the marker table text is updated.
  - `:208+` — the JSON sample shows `reason` and `replaced_dependencies`.
- `xcl-website:src/pages/examples/plugins.mdx`:
  - `:20` and `:240` — lifecycle list and Changed text: the providers now override `Changed`, with code excerpts of the network/container rules.
  - `:780-836` — Testing: mention the subnet tests.
  - `:987-1016` — replace the image-bump "update" walkthrough with the `./config-subnet` plan (network and container `-/+` with reasons, template `~`, summary `0 to create, 1 to update, 2 to replace`), then the apply events showing destroy container, destroy network, create network, create container.
  - Also correct any text that shows an image change as an update.
- Verify with `npm run build` in `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

**Complexity:** Low
**Token estimate:** ~25k tokens
**Agent strategy:** Single agent, sequential execution. Take the real output from running the example (or its plan test expectations) so the docs match.
