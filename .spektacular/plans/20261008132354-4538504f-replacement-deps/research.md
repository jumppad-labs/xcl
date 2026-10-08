---
created_date: "2026-10-08"
document_status: draft
---

# Research: 20261008132354-4538504f-replacement-deps

## Alternatives considered and rejected

- **Keep apply as one interleaved walk and add a `refreshReplace` outcome that routes to `rebuild`** — `rebuild` (`internal/parser/lifecycle.go:494-528`) destroys and recreates inside the resource's own DAG callback, i.e. in *create* order: a replaced network would be destroyed while its still-attached container exists, and a replaced container would only be destroyed after its network was. Violates the design's "destroy every replaced and removed resource, dependents first" and the spec's "nothing changes until every decision is made". Rejected.
- **Separate decision pass that re-implements read/changed outside the walk** — the existing diff walk (`walkDiff`, `diffResource` `lifecycle.go:159-246`, `refresh` `:355-438`, `decodeForDiff` `diff_decode.go:23`, `diffRecorder` `diff_recorder.go`) already is a decide-only pass sharing DAG, decode and refresh with apply. A second path would let plan and apply disagree (spec Technical Approach: "Plans and applies share one decision pass"). Rejected.
- **Infer replacement from struct tags (`xcl:",replace"`, ForceNew-style) or "any reference cascades" rules** — rejected by the user during spec work; spec Constraint "The plugin decides".
- **Reuse `diff.Action` as the provider's answer** — `diff.Action` (`diff/diff.go:17-36`) is a JSON string vocabulary with create/delete and no "none"; `plugins` does not import `diff` (go list). Coupling the provider API to the renderer's vocabulary is wrong; keep `plugins.Change` and map in the lifecycle. Rejected.
- **Pass placeholder values (from `decodeForDiff`) to Read/Changed for a resource whose inputs are unknown** — placeholders are type-zero values, so a provider would see bogus configuration and DefaultChanged would always report a change for meaningless reasons. Rejected in favour of substituting the saved value at unknown paths and flooring the outcome at Update (see assumptions).
- **Render the dependency reason as a trailing comment on the header line (design sketch)** — the user chose to keep the existing layout: the reason goes in the comment line above the header (`# docker.container.web will be replaced because docker.network.app is replaced`). Rejected by the user.
- **Tell a resource only about provider-backed resources it references literally** — a reference through a module output, variable or registered config-only type would hide a replaced resource behind the module boundary. The user chose to look through provider-less entities to the provider-backed resources behind them. Rejected by the user.
- **Keep the proto's `bool changed = 1` alongside the new enum** — breaking the protocol is allowed (spec Constraint); keeping both invites skew. Replace it with a `Change change` field and reserve 1.
- **Create-before-destroy, Replace from a failed Update** — explicit Non-Goals / design "Open: not proposed".

## Chosen approach — evidence

- Decide pass = today's diff walk, generalised: `Parser.Diff` (`internal/parser/parser.go:365-414`) already records deletes from `removedResources` (`parser.go:483`), walks with `walkWith(..., mode: walkDiff, recorder)` (`parser.go:1389-1442`), and `diffResource` chooses create / replace-by-status / unknown-cascade update / refresh. It never calls Create/Update/Destroy.
- `refresh` (`lifecycle.go:355-438`) is the single Read + Changed site (`adapter.Changed(ctx, copies.old, copies.read)` at `:416-421`); it returns `refreshOutcome` (`:317-329`) and the read copy bytes (`refreshed{old, configured, read}` `:332`), which the act pass can reuse.
- Pending/unknown machinery already exists: `diffRecorder.markPending` makes all computed fields unknown to dependents (`diff_unknown.go:25-42`); updated resources are also pending (`TestDiffPropagatesUnknownOfUpdatedResourceToItsDependents`, `diff_update_unknown_test.go`). This satisfies "values a replaced or updated dependency will only know after the apply are shown as unknown".
- Destroy machinery for the act pass's first phase: `destroyer.destroy(targets)` (`internal/parser/destroy.go:56-103`) builds `buildDestroyDAG` from saved links restricted to the targets and walks with `Reverse: true`, saving state after each destroy (`destroyed`/`failedToDestroy` `:106-133`). Apply already uses it for removed resources before the walk (`parser.go:280-315`). Adding replaced resources to its targets gives "dependents first" for free.
- Graph edges come from `Meta.Links` via `getResourceDependencies` (`internal/parser/util.go:513`) — the same source for computing each resource's direct dependencies (knowledge `learnings/depends-on-mirrors-links.md`).
- Walker semantics (`internal/dag/walk.go`): independent vertices run concurrently; a vertex whose dependency errored is skipped ("upstream dependencies failed"). Decide pass therefore guarantees parents decided before children.
- Events: rebuild already emits `destroy start/success` then `create start/success` for one ID (`TestRebuildEventsUseDestroyThenCreate`, `lifecycle_test.go:866`); there is no `replace` operation. `TestPlugin.Calls` is globally ordered under its mutex — usable for the linked network/container order assertion (convention `assert-ordering-on-graph-parents`: allowed because they are linked).
- Status semantics: create failure → `StatusFailed` (`lifecycle.go:300`); destroy failure → `StatusDestroyFailed`; both are replaced on the next apply (`run` `:104-144`, `diffResource` `:211`). Reusing these satisfies "a failed replacement is recorded honestly".
- Provider surface: `plugins` imports only `events, internal/schema, internal/wire, logger, plugins/proto, types` — new `plugins.Change`/`DependencyChange` create no cycle; no package-level `Update`/`Replace`/`NoChange` identifiers exist in `plugins` today.
- All in-repo providers except `internal/parser/test_plugin.go` embed `plugins.DefaultChanged[T]`, so changing `DefaultChanged` migrates them; only `TestResourceProvider.Changed` (`test_plugin.go:644-671`) and the generated `plugins/mocks/mock_provider_adapter.go` need hand edits, plus direct callers (`plugins/testing/helpers.go:153,202`, `plugins/example/e2e_test.go:230,296,486-568`, `plugins/changed_test.go`, `plugins/changed_sensitive_test.go`).
- Person plugin runs both in-process and external in `plugins/example/e2e_test.go` (`TestInProcessPlugin*`, `TestExternalPlugin*`) — the fixture for "external and built-in plugins behave the same".
- Render: `renderer.resource` writes `# <address> <actionPhrase>` then the marker+header (`diff/render.go:136`); `actionPhrase` (`:222-238`) hardcodes the failed-apply phrase for replace; `headerMarker` returns `-/+` for replace (`:242`). Adding a reason to `diff.Resource` and branching `actionPhrase` on it is the whole render change.
- e2e `requireDiffPredictsApply` (`e2e/diff_test.go:288`) and `applySets` (`:232`, created+destroyed ⇒ replace) already compare plan to apply — extend for the "plan lists exactly what apply does" success metric.

## Files examined

- `internal/parser/lifecycle.go:25-52` — `resourceLifecycle` fields (mode, recorder, progress); one per walk.
- `internal/parser/lifecycle.go:74-144` — `apply`/`run`: create (no state) / read (created|updated) / rebuild (other status).
- `internal/parser/lifecycle.go:159-246` — `diff`/`diffResource`: status-replace, unknown cascade → Update without Read, refresh → create/update/unchanged.
- `internal/parser/lifecycle.go:254-313` — `recordPending`, `changes`, `create` (status failed on error).
- `internal/parser/lifecycle.go:317-438` — `refreshOutcome`, `refreshed`, `refresh` (carry computed, Read, Changed at :416-421).
- `internal/parser/lifecycle.go:443-528` — `read` (Update) and `rebuild` (destroy saved copy then create; destroy error → destroy_failed).
- `internal/parser/lifecycle.go:606-637` — `callProvider` (start/error events, wrapping `"<op> failed for <id>"`).
- `internal/parser/parser.go:264-345` — `Apply`: removed destroyed first via destroyer, then walk, `progress.buildState`.
- `internal/parser/parser.go:365-414` — `Diff`: deletes + diff walk + `recorder.result()`.
- `internal/parser/parser.go:1389-1442` — `walk`/`walkWith`: DAG build, reduce, validate, walker.
- `internal/parser/callbacks.go:25-27,39-223` — `ProviderResolver`; `walkCallback` (context with unknown hook, diff decode, dispatch diff/apply).
- `internal/parser/callbacks.go:232-344` — `destroyWalkCallback` (no `callProvider`; own event emission).
- `internal/parser/destroy.go:23-136` — destroyer, reverse walk, per-resource state save.
- `internal/parser/dag.go:31-130` — `DoYouLikeDags`, `buildCreateDAG`, `buildDestroyDAG`, `buildDependencyGraph` (edges only among `nodes`).
- `internal/parser/util.go:513` — `getResourceDependencies` resolves Meta.Links (module refs expand to module resources).
- `internal/parser/diff_recorder.go` — `walkMode`, `diffRecorder` (pending, unknown, resources, unchanged, result sort+summary).
- `internal/parser/diff_unknown.go:25-42` — `contextValue`: pending ⇒ computed fields unknown; recorded unknown paths.
- `internal/parser/diff_decode.go:23` — `decodeForDiff` placeholders + unknown paths.
- `internal/parser/diff_changes.go:39,84,286` — `resourceChanges`, comparator, `unknownChange`.
- `internal/parser/progress.go` — `applyProgress`, `outcome`, `buildState`.
- `internal/parser/test_plugin.go:44-98,309,354,476-671` — `TestPlugin` recorders, `ChangedResults map[string]bool`, `SetChangedResult`, `TestResourceProvider.Changed`.
- `internal/parser/lifecycle_test.go:24-260,412-490,695-933,1256-1453` — harness, change-detection tests, statuses, rebuild tests, refresh unit tests.
- `internal/parser/diff_test.go`, `diff_update_unknown_test.go`, `diff_unknown_test.go`, `removal_test.go` — diff replace/unknown tests; removal ordering (`requireBefore`).
- `internal/test_fixtures/config/lifecycle/dependent/dependent.xcl` — first ← second (meta.name) ← third; independent.
- `internal/test_fixtures/config/lifecycle/computed_ref/computed_ref.xcl` — container references network.provider_id (computed).
- `internal/test_fixtures/config/diff/{replace_base,replace_mixed,update_ref}` — existing replace/update fixtures.
- `internal/test_fixtures/plugin/structs/network.go:9` — `Network{Subnet; ProviderID computed; Observed computed}`.
- `internal/dag/walk.go` — concurrency, `Reverse`, upstream-failed skipping.
- `types/status.go` — created/updated/failed/destroyed/destroy_failed.
- `events/events.go:52-67` — operations; no replace operation.
- `config.go:353-410` — `Config.Apply` saves once after the walk; `config_diff.go:34` — `Config.Diff`.
- `diff/diff.go:17-84` — `Action`, `Diff`, `Summary`, `Resource` (no reason field), `Change`.
- `diff/render.go:14,99,136,166-255` — unknown placeholder, `Render`, resource/comment/header, `actionPhrase`, `headerMarker`; `diff/render_test.go:50-56,259-260,380-386` pin replace text.
- `plugins/provider.go:76-99` — `ResourceProvider[T]` (Update doc "after Changed() returns true").
- `plugins/changed.go:14-61` — `DefaultChanged` JSON DeepEqual ignoring meta/depends_on/disabled.
- `plugins/adapter.go:32,207-223` — `ProviderAdapter.Changed`, `TypedProviderAdapter.Changed`.
- `plugins/plugin.go:57,202-210`, `plugins/plugin_host.go:28` — Plugin/PluginHost Changed.
- `plugins/direct_plugin_host.go:133-135,167-170` — sourcedAdapter / DirectPluginHost.
- `plugins/grpc_resource_adapter.go:52-54`, `plugins/grpc_plugin_host.go:292-312,393-399`, `plugins/grpc_server.go:144-155` — gRPC path.
- `plugins/plugin.proto:13,99-109` — `ChangedRequest{1..4}`, `ChangedResponse{bool changed=1; string error=2}`; no enums yet.
- `plugins/proto/plugin.pb.go` (protoc-gen-go v1.36.11, protoc v3.21.12), `plugins/proto/plugin_grpc.pb.go` (protoc-gen-go-grpc v1.5.1).
- `Makefile` — `protos` target (protoc command), `mocks` (mockery; installer installs v2 though config is v3).
- `.mockery.yml` — v3 config: plugins.State, plugins.ProviderAdapter, state.StateStore, parser.ProviderResolver.
- `plugins/testing/helpers.go:139-157,200-203` — TestChanged/TestCRUDOperations (asserting bool).
- `plugins/changed_test.go`, `plugins/changed_sensitive_test.go`, `plugins/adapter_test.go`, `plugins/grpc_plugin_host_test.go` (fake client embedding `proto.PluginServiceClient`) — test style templates.
- `plugins/example/pkg/person/{resource,provider}.go` — Person: first/last name (identity → person_id), optional fields, token; embeds DefaultChanged.
- `plugins/example/e2e_test.go:157-568` — in-process and external host tests, Changed asserts bool.
- `e2e/fixtures/{externalplugin/main.go:61-155,inprocess/plugin.go:70-200}` — App/Ingress external, PostgreSQL/Redis in-process; embed DefaultChanged.
- `e2e/diff_test.go:133,232,285-295,480` — diffScenario, applySets, requireDiffPredictsApply, failed replica.
- `example/plugin/main.go:60-292` — apply/plan/status/inspect/destroy CLI; `newConfig` registers template in-process and docker external.
- `example/plugin/plan.go:18-40` — `c.Diff` + `diff.Render`.
- `example/plugin/config/main.xcl` — network app (subnet 10.42.0.0/24), container web (network name = network meta.name), template welcome (reads container ip_address).
- `example/plugin/config/alt.xcl` — identical but subnet 10.42.0.0/23; read alongside main.xcl ⇒ duplicate declarations, breaks Docker-gated tests.
- `example/plugin/plugins/docker/resources/network.go:18-104` — Network{Subnet, DockerID computed}; Update no-op.
- `example/plugin/plugins/docker/resources/container.go:24-246` — Container{Image, Command, Environment, Networks[]{Name, Aliases}, DockerID, IPAddress}; Update no-op.
- `example/plugin/plugins/docker/client/client.go:29-84` — Docker interface (NetworkInspect gives IPAM subnet; no NetworkDisconnect); mock in `client/mocks`.
- `example/plugin/plugins/docker/resources/{network,container,docker}_test.go` — mock-based unit tests; real-Docker tests skip without engine.
- `example/plugin/plugins/template/template.go:27-141` — Template{Source, Destination, Variables}; Update re-renders.
- `example/plugin/{main,plan,smoke}_test.go` — Docker-gated `applyExample` on `./config`; plan tests with `writeChangedConfig`.
- `example/plugin/Makefile` — build/run/test/generate (mockery v3.8.0 via go run).
- `example/prettylog/fixtures_test.go:115,149` — embeds DefaultChanged.
- `docs/plugin-developer-guide.md:13-43,120-269,316-383,424-503` — contract, lifecycle, Changed, Update, statuses.
- `docs/plugins.md:25-148,149-198,473-486`; `docs/parser-lifecycle.md:6-129,282-322`; `docs/state.md:59-76`; `docs/README.md:16,21`; `plugins/example/README.md:131-132`.
- `README.md:225-300` — Plugins / Running them (plan description, make run).
- `CHANGELOG.md:3` — newest entry first, headed `## <spec-id>`.
- `xcl-website:src/components/Nav.astro:8-29` — hardcoded nav, Guides menu.
- `xcl-website:src/pages/examples/plugins.mdx:20,46-100,240,780-836,987-1016` — example page; plan walkthrough with image bump shown as update.
- `xcl-website:src/pages/diff.mdx:67-86,117-172` — action meanings (replace = last apply failed), rendered sample, marker table.
- `xcl-website:package.json`, `Makefile` — `npm run build`, `make check` (`astro check`).

## External references

- protobuf proto3 language guide (enums need a zero value; `reserved` for removed fields) — shapes the `Change` enum and replacing `bool changed = 1`.
- Terraform plan output (`-/+ … # forces replacement`) — prior art for the `-/+` marker and per-resource reason; we keep xcl's comment-line-above layout instead.

## Prior plans / specs consulted

- Plan `20261007105731-2388b579-diff` — established the diff walk as a mode of the apply walk, `refresh` as the shared read/changed step, the diff recorder, pending/unknown hooks, and "diff and apply cannot disagree". This plan promotes that diff walk into the decide pass of apply.
- Spec `20261008132354-4538504f-replacement-deps` and design `replacement-and-dependency-changes.md` (source `design`) — binding scope.

## Open assumptions

- A resource whose configured values contain unknowns (because a dependency is created, updated or replaced) is still Read and asked `Changed` in the decide pass, with the saved value substituted at each unknown path; its outcome is floored at Update (a provider may raise it to Replace). If this turns out to contradict expected plugin behaviour, STOP and ask.
- Existing diff tests asserting that a resource with unknown inputs is *never Read* will change to assert it is read with saved values substituted. If a test encodes a deliberate guarantee beyond the old implementation, STOP and ask.
- A decide-pass failure (Read or Changed error) fails the apply with previous state left unchanged — the resource is *not* marked failed (nothing was attempted). Today a failed Read marks the resource failed.
- In the act pass, an Update receives the freshly decoded configuration with computed values carried from the decide pass's Read copy (not the decide pass's read copy itself).
- A resource decided NoChange keeps its saved status and its decide-pass read copy is saved.
- Replaced resources whose destroy succeeds are removed from working state before the create phase; if the create phase never reaches them (upstream failure) they are simply absent from state and the next apply creates them.
- Regenerating protobuf with locally installed protoc (6.36.1) and pinned plugins (`protoc-gen-go@v1.36.11`, `protoc-gen-go-grpc@v1.5.1` via `go run`) is acceptable; header churn is fine.
- Mocks are regenerated with `go run github.com/vektra/mockery/v3@v3.8.0` (mockery not on PATH).
- Docker network replacement while a container that the plugin answered Update/NoChange for is attached would fail in Docker — acceptable, it is the plugin's decision; the example container answers Replace when its network is replaced.

## Drafting assumptions

### Unknown inputs: read with saved values, floor at Update (discovery)
- **Decision**: In the decide pass a resource whose configured values contain unknowns is still Read and asked Changed, with the saved value at each unknown path; core floors its outcome at Update, and the provider may raise it to Replace.
- **Rationale**: The design says every resource gets Read then Changed with its dependencies' decisions. Placeholders would mislead providers. Without a floor, a template whose input IP will change could answer NoChange and never re-render.
- **Rejected**: Skipping Read/Changed for unknown inputs (today's diff behaviour) stops the provider deciding Replace. Passing placeholders feeds providers bogus values.

### Decide failure leaves state untouched (discovery)
- **Decision**: A Read or Changed error in the decide pass fails the apply and leaves the previous state as it was. No resource is marked failed.
- **Rationale**: The spec says "a failing decision changes nothing". Nothing was attempted, so marking the resource failed would force a needless replace next time.
- **Rejected**: Keeping today's behaviour of marking a resource failed when its Read fails.

### Replace reason in the comment line (discovery, user decision)
- **Decision**: The rendered reason goes in the existing comment line above the header: `# <addr> will be replaced because <dep> is replaced`.
- **Rationale**: The user chose it, for consistency with the other actions.
- **Rejected**: A trailing comment on the header line, as in the design sketch.

### Look through provider-less entities for dependencies (discovery, user decision)
- **Decision**: A resource's dependencies are the provider-backed resources it reaches directly, or through outputs, variables, modules and registered config-only types.
- **Rationale**: The user chose it, so that module boundaries don't hide a replacement.
- **Rejected**: Listing only literal direct references to provider-backed resources.

### Tooling for generated code (discovery)
- **Decision**: Regenerate the proto with the local protoc and pinned `protoc-gen-go@v1.36.11` / `protoc-gen-go-grpc@v1.5.1` run through `go run`. Regenerate mocks with `go run mockery/v3@v3.8.0`.
- **Rationale**: These match the generator versions already in the repo, and neither mockery nor matching plugins are on PATH.
- **Rejected**: Using the installed protoc-gen-go-grpc 1.6.1 (needless churn), or editing the generated code by hand.
### Chosen direction: decide pass = promoted diff walk, act = destroyer then apply walk (architecture)
- **Decision**: Promote the existing diff walk into a shared decide pass with a per-entity decision record. Act runs the destroyer (reverse graph over replaced and removed resources), then the ordinary apply walk following the recorded decisions. The in-walk rebuild is removed. Delivery order: contract change, then decide/act, then reason and render, then example plugins, then docs.
- **Rationale**: This reuses the code that already decides without acting (refresh, the pending/unknown hooks), so plan and apply share one decision. The destroyer already gives dependents-first destroys and per-resource saves. The repository keeps building between phases.
- **Rejected**: Extending the in-walk rebuild (wrong destroy order). A separate decide path (plan/apply could disagree). Serial topological lists for the act phase (loses concurrency and duplicates context building). Effort: Medium-High.

### Unchanged resources keep the decide-pass copy (architecture)
- **Decision**: An entity decided NoChange is saved as its decide-pass read copy, with its status unchanged. An Update gets the freshly decoded configuration with computed values carried from the decide-pass Read.
- **Rationale**: This matches what read() saves and sends today, without calling Read twice.
- **Rejected**: Calling Read again in the act pass (doubles provider calls and could disagree with the plan).

### Plugin replace settings (architecture)
- **Decision**: Docker network: subnet means Replace. Docker container: image, command, environment and network blocks mean Replace, and a replaced dependency means Replace. Template: destination means Replace; source and variables mean Update. Person: first_name or last_name means Replace. The e2e fixtures keep DefaultChanged semantics (Update) because their fake providers can change every field in place.
- **Rationale**: These are the settings each provider really cannot change in place: Docker cannot change them without recreating, the template's Update cannot remove the old file, and the person ID derives from the name.
- **Rejected**: Leaving container fields as no-op updates, which is the drift the spec exists to remove.

### Conventions selected (architecture)
- **Decision**: Applied: code style, top-level public packages, testing and mocking, real-apply state, graph-parent ordering, shared test helpers, never modifying dependencies, the shared errors package, structured logging, the example-modules gotcha, the EncodeForState gotcha, and the entity glossary. Dropped: database, and HTTP/graceful shutdown.
- **Rationale**: These are the surfaces the work touches.
- **Rejected**: Listing every convention.

### Diff reason shape (data_structures)
- **Decision**: Add `diff.ReplaceReason` (failed, provider, dependency) and `ReplacedDeps []string` to `diff.Resource`, as JSON `reason` and `replaced_dependencies`, both omitted when empty.
- **Rationale**: The renderer needs three distinct phrases, and JSON consumers can then tell why a resource is replaced. The fields are additive to the diff design's Resource shape.
- **Rejected**: A single free-text reason string (not machine-readable). Deriving the reason at render time (the renderer has no state).

### Proto field numbering (data_structures)
- **Decision**: Make `ChangedResponse.change` field 3 and reserve 1, and add `ChangedRequest.dependencies` as field 5.
- **Rationale**: A stale plugin then fails loudly: it gets an unset change (NoChange) rather than having its bool misread. Breaking the protocol is allowed.
- **Rejected**: Reusing field 1 with a new type (wire-incompatible in confusing ways).

### Existing tests whose expectations change (testing_approach)
- **Decision**: Update the tests the design deliberately breaks (cross-resource event interleaving, never-read-with-unknown-inputs, a failed Read marking the resource failed) in the same task, and give the reason in that task.
- **Rationale**: These behaviours follow directly from decide-then-act and the unknown-input rule.
- **Rejected**: Keeping the old expectations through special cases.

### Docker success metric is behavioural but gated (testing_approach)
- **Decision**: Classify the no-drift metric as a behavioural Docker-gated test, plus a manual walkthrough on a machine with Docker.
- **Rationale**: The test skips without Docker, as every existing example test does.
- **Rejected**: Manual only (loses automation where Docker exists).

### e2e fixtures gain small replace rules (tasks)
- **Decision**: The in-process e2e PostgreSQL `location` and the external e2e Ingress `hostname` answer Replace. Every other field keeps the DefaultChanged answer.
- **Rationale**: The plan-versus-apply e2e scenarios need provider-decided replacements on both plugin kinds, and those two fields are identity-like.
- **Rejected**: Leaving all e2e fixtures on DefaultChanged (no e2e coverage of provider replace on the mixed graph).

### Newly created dependencies are not listed (tasks)
- **Decision**: A dependency decided create is not included in a dependent's list.
- **Rationale**: The design lists only Update and Replace. The Update floor for unknown inputs covers the dependent.
- **Rejected**: Reporting create as Replace (misleading to providers).

### Website page placement (tasks)
- **Decision**: Add a new page `src/pages/replacement.mdx`, linked from the Guides menu.
- **Rationale**: The site has no plugin-authoring page. The spec requires a section reachable from the guides.
- **Rejected**: Burying the explanation inside the plugin example page (not reachable as a guide).

## Rehydration cues

- `spektacular spec file read 20261008132354-4538504f-replacement-deps`
- `spektacular design read --data '{"source":"design","path":"replacement-and-dependency-changes.md"}'`
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`
- `spektacular knowledge read` for `learnings/depends-on-mirrors-links.md`, `learnings/failed-resource-in-e2e-with-bad-host.md`, `gotchas/example-modules-cannot-import-internal.md`, `learnings/state-save-and-load-points.md`, `gotchas/save-state-through-encodeforstate.md`.
- `spektacular plan file read 20261007105731-2388b579-diff plan` — diff walk design.
- Re-read: `internal/parser/lifecycle.go`, `internal/parser/parser.go:264-414,1389-1442`, `internal/parser/destroy.go`, `internal/parser/diff_recorder.go`, `internal/parser/diff_unknown.go`, `internal/parser/test_plugin.go:44-98,476-671`, `plugins/{provider,changed,adapter,plugin,grpc_plugin_host,grpc_server}.go`, `plugins/plugin.proto`, `diff/{diff,render}.go`, `example/plugin/plugins/docker/resources/{network,container}.go`, `example/plugin/config/*.xcl`.
