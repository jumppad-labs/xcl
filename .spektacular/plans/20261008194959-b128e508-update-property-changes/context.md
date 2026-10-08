---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Context: 20261008194959-b128e508-update-property-changes

## Current State Analysis

xclconfig (`/home/nicj/code/github.com/jumppad-labs/xcl`, branch `f-diff`) already has the decide-then-act apply from spec `20261008132354-4538504f-replacement-deps`.

**Decide and act.**
- `Parser.Apply` (`internal/parser/parser.go:267`) runs `decide` (:281), reparses (:290), destroys `record.toDestroy` (:300), and walks the act pass (:338).
- While deciding, `refresh` (`internal/parser/lifecycle.go:514-598`) marshals the saved copy (`copies.old`), the configured copy (`copies.configured`, :528) and the post-Read copy (`copies.read`, :567). It then calls `adapter.Changed(ctx, copies.old, copies.read, dependencies)` (:577).
- The dependency list comes from `dependencyChanges` (`internal/parser/dependencies.go:30`) and is stored in `decision.dependencies` (`internal/parser/diff_recorder.go:39`).
- While acting, `update` (`lifecycle.go:158-192`) passes only the marshalled entity, with computed values carried from the decide-pass read, to `adapter.Update(ctx, data)` (:174).

**Plan changes.**
- These are computed by `resourceChanges` (`internal/parser/diff_changes.go:39`), which compares the saved copy against the configured copy by reflection. It is called only after `Changed`, for pending actions (`lifecycle.go:408-424`).
- Sensitive values are compared for real, then blanked unless revealed (`compareSensitive` :174, `plainValue` :419).
- Unknown values are flagged from the decide-time unknown paths (`unknownChange` :286).

**Public types.**
- `diff.Change{Path, Before, After, Unknown, Sensitive}` (`diff/diff.go:110-128`).
- `diff.Path` (`diff/path.go:9-33`, stdlib-only, no matching helpers).
- `entity` (`entity/change.go`) holds `Change` and `DependencyChange` and imports only the standard library.

**Plugin contract.**
- `ResourceProvider[T].Update(ctx, resource T)` and `Changed(ctx, old, new, dependencies)` (`plugins/provider.go:87,109`), carried through the adapters, hosts, gRPC wrapper, server and `plugins/plugin.proto` (`UpdateRequest` fields 1-3; `ChangedRequest` fields 1-5).

**Docker example (`example/plugin`).**
- The container's `Changed` replaces on any network change or replaced dependency, and its `Update` is a no-op.
- The network's `Destroy` doesn't detach containers.
- The client interface lacks `NetworkDisconnect`.
- The template has no file mode.

**Website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).**
- `src/pages/replacement.mdx` and `src/pages/examples/plugins.mdx` document the current signatures, and state that the Docker `Update` does nothing.

## Per-Task Technical Notes

### Requirement → repo and files

| Spec requirement | Repo | Main files |
|---|---|---|
| Updates are told what changed | xclconfig | `internal/parser/lifecycle.go`, `internal/parser/diff_changes.go`, `plugins/*` |
| Updates are told about changing dependencies | xclconfig | `internal/parser/lifecycle.go` (`update`), `plugins/*`, `plugins/plugin.proto` |
| Change decisions see what changed | xclconfig | `internal/parser/lifecycle.go` (`refresh`, `decideResource`) |
| Values not yet known are marked when deciding | xclconfig | `internal/parser/diff_changes.go` (`unknownChange`), plugin projection |
| Changes hold real values at update time | xclconfig | `internal/parser/lifecycle.go` (`update`), `internal/parser/callbacks.go:141` |
| Sensitive settings reported with real values | xclconfig | `internal/parser/diff_changes.go`, `entity/property_change.go`, `plugins/change_proto.go` |
| Plugins can easily check where a change is | xclconfig | `entity/path.go`, `entity/property_change.go` |
| External plugins receive the same information | xclconfig | `plugins/plugin.proto`, `plugins/change_proto.go`, `plugins/grpc_*.go`, `e2e/` |
| Every plugin in the repository uses the new information | xclconfig | every provider listed under the contract task |
| Example hot swaps networks / keeps containers / init script / lose a network | xclconfig | `example/plugin/plugins/docker/**`, `example/plugin/plugins/template/**`, `example/plugin/config/**`, `example/plugin/*_test.go`, `example/plugin/Makefile` |
| Documentation shows the new information | xclconfig, xcl-website | `docs/*.md`, `README.md`, `CHANGELOG.md`, `plugins/example/README.md`; `xcl-website:src/pages/replacement.mdx`, `xcl-website:src/pages/examples/plugins.mdx` |

### Task: Changed setting type and structured path

**File changes**
- `entity/path.go` (new): move `StepKind` (with `StepAttribute`, `StepIndex`, `StepKey`), `Step`, `Path`, the builders `Attribute`/`Index`/`Key` (copy-on-append `with`), `String` and `MarshalJSON` verbatim from `diff/path.go:9-100`. Add `Equal(other Path) bool` (same length and steps) and `Within(prefix Path) bool` (`len(p) >= len(prefix)` and `p[:len(prefix)].Equal(prefix)`; a path is within itself).
- `entity/path_test.go` (new): move the tests from `diff/path_test.go`, and add separate positive and negative tests for `Equal` and `Within`.
- `diff/path.go:1-100`: replace with aliases: `type StepKind = entity.StepKind`, `type Step = entity.Step`, `type Path = entity.Path`, plus `const (StepAttribute = entity.StepAttribute; StepIndex = entity.StepIndex; StepKey = entity.StepKey)`. Keep the package doc comment. `diff` now imports `entity` (stdlib-only, no cycle).
- `diff/path_test.go`: keep one test proving `diff.Path{}.Attribute("x").String()` and JSON still work through the alias. Remove the moved duplicates.
- `entity/property_change.go` (new): `PropertyChange{Path Path; Before, After any; Unknown, Sensitive bool}`, with doc comments saying values are plain JSON values and `Unknown` appears only when deciding.
  - `At(path Path) bool` and `Within(path Path) bool`.
  - Masking methods, used when `Sensitive`:
    - `String()` → e.g. `network[0].name: "app" -> "backend"`, or `password: (sensitive) -> (sensitive)`;
    - `Format(f fmt.State, verb rune)` writes `String()` for every verb, so `%v`, `%+v` and `%#v` are all safe;
    - `LogValue() slog.Value` is a group of path, before, after, unknown and sensitive, with the values replaced by `"(sensitive)"`;
    - `MarshalJSON` emits `{"path":…, "before":…, "after":…, "unknown":…, "sensitive":…}`, with the values replaced by `"(sensitive)"`.
  - Define the marker as a package constant equal to `types.SensitiveMarker` (`types/sensitive.go:15`, `"(sensitive)"`). `entity` must not import `types`.
- `entity/property_change_test.go` (new): separate tests for:
  - `At`/`Within` positive (`network[0].name` is at that path and within `network`);
  - `Within` negative (not within `image`);
  - `String`/`%v`/`LogValue`/`MarshalJSON` mask a sensitive change;
  - non-sensitive values print;
  - the fields keep real values.
- `entity/doc.go:1-11`: extend the package doc to cover `PropertyChange` and `Path`; keep the stdlib-only note.
- `internal/parser/diff_changes.go:305-335`: `unknownAt`/`unknownUnder`/`pathEqual` delegate to `Path.Equal`/`Path.Within`. Delete `pathEqual` and call `Path.Equal` directly.
- All callers of `diff.Path`/`diff.Step*` keep compiling through the aliases. No mass rename.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential: move the path, add the aliases, run the diff tests, then add `PropertyChange` and its tests.

### Task: Change contract through every plugin layer

**File changes**
- `plugins/provider.go:78-109`: `Update(ctx, resource T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (T, error)` and `Changed(ctx, old, new T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)`. Rewrite the doc comments:
  - at `Changed`, `changes` lists settings that differ from the last apply, and a new value only known after the apply has `Unknown` set;
  - at `Update`, every value is real, and `dependencies` are as told to `Changed`;
  - sensitive values are real.
- `plugins/changed.go:24-53`: `DefaultChanged.Changed` gains `changes` (ignored). Its doc comment says so.
- `plugins/adapter.go:26-33`: in `ProviderAdapter`, add `changes []entity.PropertyChange, dependencies []entity.DependencyChange` to `Update`, and `changes` before `dependencies` on `Changed`.
- `plugins/adapter.go:184-225`: `TypedProviderAdapter.Update`/`Changed` pass the lists through to the provider.
- `plugins/plugin.go:47-58,194-211`: `PluginEntityProvider` and `PluginBase.Update`/`Changed` gain the lists and forward them to `rt.Adapter`.
- `plugins/plugin_host.go:29-33`: in the `PluginHost` interface, `Update`/`Changed` gain the lists.
- `plugins/direct_plugin_host.go:130-135,164-171`: `sourcedAdapter` and `DirectPluginHost` forward the lists.
- `plugins/grpc_resource_adapter.go:49-54`: forward the lists.
- `plugins/grpc_plugin_host.go:273-311,393-406`:
  - `grpcPluginWrapper.Update` builds `UpdateRequest{…, Changes: toProtoPropertyChanges(changes), Dependencies: toProtoDependencies(dependencies)}`;
  - `Changed` adds `Changes: toProtoPropertyChanges(changes)`;
  - converter errors are returned before the call.
- `plugins/grpc_server.go:124-165`:
  - `GRPCServer.Update` decodes `fromProtoPropertyChanges(req.Changes)` and `fromProtoDependencies(req.Dependencies)` and passes them to `rt.Adapter.Update`;
  - `Changed` decodes `req.Changes`;
  - a decode error goes into the `Error` string, as today.
- `plugins/plugin.proto:88-125`: add `enum StepKind`, `message PathStep`, `message PropertyChange`, `ChangedRequest.changes = 6`, `UpdateRequest.changes = 4` and `UpdateRequest.dependencies = 5`. No reserved fields are needed, since none are removed.
- `plugins/proto/plugin.pb.go`, `plugins/proto/plugin_grpc.pb.go`: regenerate with `make protos` (protoc-gen-go v1.36.11, -grpc v1.5.1). Never hand-edit.
- `plugins/change_proto.go:11-80`: add `toProtoPropertyChanges`/`fromProtoPropertyChanges`, `toProtoPath`/`fromProtoPath` and `toProtoStepKind`/`fromProtoStepKind` (an unknown kind is an error). For values:
  - encode with `json.Marshal(value)` (the values are already plain JSON values; nil gives empty bytes);
  - decode with `json.Unmarshal` into `any` (empty bytes give nil);
  - never marshal the `PropertyChange` itself;
  - an empty list gives nil, matching `toProtoDependencies`.
- `plugins/mocks/mock_provider_adapter.go`: regenerate with `mockery` (root `.mockery.yml`, v3.8.0).
- `plugins/testing/helpers.go:141-203`: `TestChanged`/`TestCRUDOperations` pass `nil, nil`. There are no `Update` helper calls to change.
- `internal/parser/lifecycle.go:174`: `adapter.Update(ctx, data, nil, nil)`. At `:577`: `adapter.Changed(ctx, copies.old, copies.read, nil, dependencies)`. These are temporary; the later tasks fill them in.
- `internal/parser/test_plugin.go:639-696`: `TestResourceProvider.Update`/`Changed` take the new parameters (recording comes later).
- Providers to migrate (signatures only, behaviour unchanged):
  - `plugins/example/pkg/person/provider.go:40,107`;
  - `e2e/fixtures/externalplugin/main.go:92,133,155`;
  - `e2e/fixtures/inprocess/plugin.go:90,120,202`;
  - `internal/test_fixtures/plugins/subtypeless/main.go:65`;
  - `example/plugin/plugins/docker/resources/container.go:248,268`;
  - `example/plugin/plugins/docker/resources/network.go:104,115`;
  - `example/plugin/plugins/template/template.go:56,103`.
- Test fakes to migrate:
  - `config_plugin_subtypeless_test.go:77`;
  - `example/prettylog/fixtures_test.go:134,170`;
  - `internal/parser/validate_test.go:655`;
  - `internal/catalog/sensitive_types_test.go:48,76`;
  - `internal/catalog/catalog_test.go:74,152`;
  - `registry/local_test.go:47`;
  - `plugins/adapter_test.go:49,246,250`;
  - `plugins/direct_plugin_host_test.go:38`;
  - `plugins/changed_test.go:45,61-130`;
  - `plugins/changed_sensitive_test.go:27,36,94`;
  - `plugins/grpc_plugin_host_test.go:79` (`fakeChangedServiceClient`, which must capture `UpdateRequest` too);
  - direct `Changed`/`Update` callers in `plugins/example/e2e_test.go:231,297,464,487-621`, `plugins/example/pkg/person/provider_test.go:29-61` and `example/plugin/plugins/**/*_test.go`.
- New tests:
  - `plugins/change_proto_test.go`: round-trip tests (plain, nested path with index and key, sensitive real values, unknown with an absent after, nil before), plus an invalid step kind as a separate test.
  - `plugins/grpc_plugin_host_test.go`: the wrapper sends changes and dependencies on `Update` and `Changed`.
  - `plugins/grpc_server_test.go:17-78`: the server delivers decoded lists to the provider for `Update` and `Changed`.
  - `plugins/adapter_test.go`: the typed adapter passes the lists to the provider.

**Complexity**: High
**Token estimate**: ~80k tokens
**Agent strategy**: Parallel analysis, sequential integration. One agent changes the `plugins` package, the proto, the regeneration and the converter tests. Once those compile, two agents in parallel migrate (a) root-module providers and fakes (`e2e`, `internal`, `registry`, `plugins/example`, root tests) and (b) the separate example modules (`example/plugin`, `example/prettylog`). Finally, one agent builds and tests every module.

### Task: One comparison feeds the plan and the plugins

**File changes**
- `internal/parser/diff_changes.go:39`: callers of `resourceChanges(action, saved, configured, body, unknown, reveal)` pass `reveal=true` to get the real-valued result. The function itself is unchanged; `compareSensitive` (:174) already sets `Sensitive` regardless of reveal.
- `internal/parser/change_views.go` (new; `change_views_test.go` beside it):
  - `planChanges(revealed []diff.Change, reveal bool) []diff.Change` copies each change. When `!reveal`, it nils `Before`/`After` on every change with `Sensitive` set, giving exactly today's masked output. Check that `whole()` (:220) and `containsSensitive` (:376) already split sensitive leaves into their own changes, so blanking per change matches today's output for nested sensitive values. If it doesn't, STOP and report.
  - `pluginChanges(revealed []diff.Change, unknownAtDecide []diff.Path) []entity.PropertyChange` normalises each `Before`/`After` by `json.Marshal` then `json.Unmarshal` into `any`, and copies `Path`, `Unknown` and `Sensitive`. When `unknownAtDecide` is given (act time), for each decide-time unknown path with no change at or under it, it appends `PropertyChange{Path, Before, After}`. The real values come from the same saved and configured structs, through a small `valueAt(saved/configured, path)` helper that reuses the struct walker `plainValue` (:419) with reveal on. It keeps the result sorted in the same order `resourceChanges` uses (field order, then index or key).
  - Also check `plainCtyValue` (:480): with reveal it must return real values for cty marks.
- `internal/parser/diff_recorder.go:25-44`: `decision` gains `changes []diff.Change` (the plan projection).
- `internal/parser/lifecycle.go:408-424`: `recordPendingResource` / `changes` take a precomputed plan list when one is available, and only compute (with `planChanges(resourceChanges(..., true), l.diffOptions.RevealSensitive)`) for paths that don't go through `refresh` (create, failed-replace). The plan output is identical either way.
- Tests in `internal/parser/change_views_test.go`, each a separate function:
  - the plan view masks a sensitive change without reveal;
  - the plan view keeps values with reveal;
  - the plugin view keeps real sensitive values;
  - the plugin view normalises an `int` to `float64` and a struct to `map[string]any`;
  - an unknown change has a nil `After` in both views;
  - a decide-time unknown path resolved to the saved value is still listed at act time;
  - the plugin view's paths equal the plan view's.
- `internal/parser/diff_changes_test.go:332-439`: existing sensitive tests must still pass unchanged.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential.

### Task: Change decisions are told the changed settings

**File changes**
- `internal/parser/lifecycle.go:514-598` (`refresh`):
  - after `copies.configured = wire.Marshal(r)` (:528), decode the configured copy (`decodeCopy`, as at :336/:355);
  - compute `revealed := resourceChanges(diff.ActionUpdate, old, configured, l.bodies[meta.ID], l.recorder.unknownPaths(meta.ID), true)`;
  - pass `pluginChanges(revealed, nil)` to `adapter.Changed(ctx, copies.old, copies.read, changes, dependencies)` (:577);
  - return `revealed` in `refreshed` (:490-501) as a new field.
- `internal/parser/lifecycle.go:267-369` (`decideResource`): for the Changed and Replace outcomes (:336-361), call `recordPending` / `recordReplace` with `planChanges(copies.revealed, l.diffOptions.RevealSensitive)` instead of recomputing, and store the same plan list in `decision.changes`. Outcomes, the unknown floor (:325), the dependency list (:306) and `withSavedValues` (:315) are unchanged.
- `internal/parser/test_plugin.go:45-103,137-166,242-251,665-696`: add `ChangedChanges map[string][]entity.PropertyChange`, recorded in `TestResourceProvider.Changed` under the plugin mutex, and `GetChangedChanges(id)` returning a copy. Reset it in `ResetCalls`.
- Tests in `internal/parser/decide_test.go` (model: `applyComputedRef` :41, `runDecide` :54), each its own function:
  - an edited attribute: `GetChangedChanges` holds one change with the path, before and after;
  - an unchanged resource: an empty list;
  - a value from a dependency set to `entity.Replace`: the change at that path has `Unknown` set and a nil `After` (fixture `internal/test_fixtures/config/diff/update_ref/{before,edited}`, as used in `diff_update_unknown_test.go:14-40`);
  - a sensitive `Credential.Password` edit: real before and after with `Sensitive` set.
- `internal/parser/diff_test.go`, `diff_update_unknown_test.go`: must pass unchanged, proving plans are the same.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential.

### Task: Updates are told the changed settings and dependencies

**File changes**
- `internal/parser/lifecycle.go:158-192` (`update`):
  - before `carryComputedValues(r, dec.read)` (:164), find `saved := findByID(l.previous.GetResources(), meta.ID)`, as `keep` does (:202);
  - compute `revealed := resourceChanges(diff.ActionUpdate, saved, r, l.bodies[meta.ID], nil, true)` and `changes := pluginChanges(revealed, l.recorder.unknownPaths(meta.ID))`. Pass the saved and configured values for the decide-time-unknown append;
  - call `adapter.Update(ctx, data, changes, dec.dependencies)` (:174);
  - a missing saved copy is an internal error. It can't happen for an Update decision, because updated resources are not destroyed (`parser.go:300-330`).
- `internal/parser/test_plugin.go`: add `UpdateChanges` and `UpdateDependencies` maps, recorded in `TestResourceProvider.Update` (:639), plus `GetUpdateChanges(id)` and `GetUpdateDependencies(id)`. Reset them in `ResetCalls` (:242).
- `internal/parser/events.go:47-129` and `internal/parser/parser.go:527-554`: no change. Confirm in review that neither list is added to `Event.Data` or to `logDecisions`.
- Tests in `internal/parser/update_changes_test.go` (new, beside `replace_test.go`; uses `setupLifecycle`/`applyAndSave`/`ResetCalls`/`SetChangedResult` from `lifecycle_test.go:57-177`), each its own function:
  - an edited attribute with an update answer: `GetUpdateChanges` is exactly that change, with no unchanged setting;
  - a replaced dependency with an update answer and no own changes: empty changes, and `GetUpdateDependencies` equals `[{dep, Replace}]`, which equals `GetChangedDependencies`;
  - a value from a dependency replaced in the same apply (update_ref fixture): `Unknown` is false and `After` is the dependency's new real value (model: `diff_update_unknown_test.go:248`);
  - a sensitive `Credential.Password` edit: the update gets real values. Run the same edit with `newParserWithEventData` (`lifecycle_test.go:103`) and a captured logger, and assert the event data and logs contain no real secret (model: `config_event_sensitive_test.go`, `events_sensitive_test.go`); and the plan (diff) shows the change as sensitive with nil values;
  - the same edit run through `Diff` and then `Apply`: the paths of `diff.Resource.Changes`, `GetChangedChanges` and `GetUpdateChanges` are equal (one test per variant: plain, sensitive, not-yet-known).

**Complexity**: Medium
**Token estimate**: ~45k tokens
**Agent strategy**: Single agent, sequential.

### Task: Built-in and external plugins are told the same

**File changes**
- `e2e/fixtures/recorder/recorder.go` (new package in the root module):
  - `Recorder` resource type: `types.ResourceBase`, `Value string`, `Secret types.Sensitive[string]`, `Input string`, `ReplaceKey string`, `Output string` (computed, set by Create to `<id>-<replace_key>`) and `Log string` (`xcl:"log"`, the file path to append to);
  - a provider embedding `DefaultChanged`. Its `Changed` and `Update` append one JSON line per call to `Log`: `{"call":"changed"|"update","id":…,"changes":[…],"dependencies":[…]}`;
  - the changes are written by a local struct that copies the real fields (path string, before, after, unknown, sensitive), not by marshalling `PropertyChange`, so the record holds real values for the test to compare;
  - `Changed` answers `Update` when there are changes or dependencies.
- `e2e/fixtures/externalplugin/main.go:33-60`: register type `"recorder"`, subtype `"item"`, with the recorder provider.
- `e2e/plugin_changes_test.go` (new):
  - an in-process wrapper plugin registering the same provider under the same type, modelled on `personInProcessPlugin` (`e2e/plugin_replace_test.go:45-100`);
  - scenarios: first apply, then an edit of `value`, `secret` and an `input` taken from a dependency that the same apply replaces. The dependency is a second recorder resource that answers `Replace` when its `replace_key` attribute changes, and exposes a computed `output` that the first recorder's `input` references;
  - separate tests assert that the in-process and external records are equal for `changed` lines and for `update` lines;
  - the reader helper stays local to the test file, since it's used by one package.
- `e2e/main_test.go:22-50`: no change; the external fixture binary is already built there.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential.

### Task: Network rebuild detaches its containers

**File changes**
- `example/plugin/plugins/docker/client/client.go:29-45`: add `NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error` to `Docker`. The real client satisfies it (`:48` assertion).
- `example/plugin/plugins/docker/client/mocks/mock_docker.go`: regenerate with `make generate` (mockery v3.8.0, `example/plugin/.mockery.yml`).
- `example/plugin/plugins/docker/resources/network.go:82-91` (`Destroy`):
  - `NetworkInspect(ctx, n.DockerID, network.InspectOptions{})`; NotFound means already gone, so return nil;
  - for each container ID in `inspect.Containers`, call `NetworkDisconnect(ctx, n.DockerID, id, true)`, ignoring NotFound;
  - then `NetworkRemove`, as today.
- `example/plugin/plugins/docker/resources/network_test.go:147`: replace the "active endpoints" expectation with separate tests:
  - destroy with two attached containers: inspect, then two force disconnects, then remove;
  - destroy with none: inspect, then remove;
  - destroy of a missing network: inspect NotFound gives success, and remove is not called.
- `example/plugin/plugins/docker/docker_test.go`: one real-Docker test that creates a network, attaches a container with `containerProvider`, and destroys the network successfully, leaving the container running.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential.

### Task: Container hot swaps its networks

**File changes**
- `example/plugin/plugins/docker/resources/container.go:248-263` (`Changed`):
  - Answer `entity.Replace` if any change is within `image`, `command` or `environment`. The `init_script` rule is added with the field in the next task.
  - Answer `entity.Replace` if any dependency with `Change == entity.Replace` is not a Docker network. Decide this with a helper that splits the address on "." and checks that the last three segments are `docker`, `network`, `<name>`, so module prefixes still work.
  - Otherwise answer `entity.Update` when there are changes or dependencies.
  - Otherwise defer to `DefaultChanged`.
  - Remove `equalStrings`/`equalEnvironment`/`equalAttachments` (:279-324) if they become unused.
- `example/plugin/plugins/docker/resources/container.go:268-270` (`Update`), new behaviour:
  1. `previous := previousAttachments(c.Networks, changes)`: start from a copy of the current attachments. For every change within `network`, apply its `Before` at its path: a whole-entry change `network[i]` with `Before` nil removes entry i; one with `After` nil restores a `map[string]any` as a `NetworkAttachment`; a `network[i].name` / `network[i].aliases` change sets that field. Apply from the highest index down.
  2. For each previous attachment whose name is absent from the current list, or whose aliases differ, call `NetworkDisconnect(ctx, name, c.DockerID, true)`, ignoring NotFound.
  3. For each current attachment that is absent from previous, or whose aliases differ, call `NetworkConnect(ctx, name, c.DockerID, &network.EndpointSettings{Aliases})`.
  4. For each dependency with `Change == entity.Replace` that is a Docker network, whose last segment equals a current attachment name not already connected in step 3, call `NetworkConnect` for it.
  5. If anything was connected or disconnected, `ContainerInspect` and set `IPAddress` via `firstAddress` (:177-198). This reads an output, not previous state. Then return.
  - The doc comment states that Update uses only what it is told.
- `example/plugin/plugins/docker/resources/container_test.go:465-507`: replace the replace-on-network tests with separate tests, using a strict `mocks.NewMockDocker(t)` so any unexpected call fails:
  - a network name change answers Update;
  - an alias change answers Update;
  - a replaced network dependency answers Update;
  - a replaced non-network dependency (`template.init`) answers Replace;
  - an image change answers Replace;
  - a command change answers Replace;
  - an environment change answers Replace;
  - no changes defers to the default;
  - an Update with `network[0].name "app"→"backend"` disconnects app, connects backend, and inspects once;
  - an Update with a replaced `docker.network.app` and no changes connects app and inspects;
  - an Update removing `network[0]` disconnects app, connects nothing, and the IP is empty;
  - an Update ignores NotFound on disconnect.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Single agent, sequential.

### Task: Container rebuilds when its init script is rebuilt

**File changes**
- `example/plugin/plugins/template/template.go:28-41`: add `Mode string \`xcl:"mode,optional"\``, parsed with `strconv.ParseUint(mode, 8, 32)`; empty means `0644`. An invalid mode is an error from Create/Update.
- `example/plugin/plugins/template/template.go:121-155` (`render`): `os.WriteFile(dest, out, mode)`, then `os.Chmod(dest, mode)`, so an existing file's mode is updated too.
- `example/plugin/plugins/template/template_test.go`: separate tests for: mode `"0755"` gives an executable file; no mode gives `0644`; an invalid mode errors.
- `example/plugin/plugins/docker/resources/container.go:25-47`: add `InitScript string \`xcl:"init_script,optional"\``.
- `example/plugin/plugins/docker/resources/container.go:85-122` (`Create`): when `InitScript` is set, resolve it with `filepath.Abs`, and set `HostConfig{Binds: []string{abs + ":/docker-entrypoint.d/90-xcl-init.sh:ro"}}`.
- `example/plugin/plugins/docker/resources/container.go` (`Changed`): add `init_script` to the replace-on-change list.
- `example/plugin/config/main.xcl`:
  - add `template "init"` with `destination = "${variable.output_dir}/init.sh"` and `mode = "0755"`; its source is a short `#!/bin/sh` script that writes `/usr/share/nginx/html/init.txt` naming the network;
  - add `init_script = template.init.destination` to `docker "container" "web"`;
  - update the header comments.
- `example/plugin/config-subnet/main.xcl`: mirror the init template and `init_script`. Update its header comment: the network is replaced, the container updated and reattached, and the template updated.
- `example/plugin/plugins/docker/resources/container_test.go`: separate tests for: Create with `InitScript` binds the absolute path read-only; an `init_script` change answers Replace.
- `example/plugin/status.go`: no code change expected (templates already listed). `main_test.go:258` "3 lines" becomes 4, and is updated in the scenarios task.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential.

### Task: Example scenarios prove hot swap and rebuilds

**File changes**
- `example/plugin/main_test.go:408-474` (subnet scenario):
  - `TestSubnetChangeAttachesANewContainer` (:432) becomes `TestSubnetChangeKeepsTheContainerAttached`: `require.Equal(old.DockerID, web.DockerID)`, running, and `Networks["app"].NetworkID == app.DockerID`;
  - keep `…ReplacesTheNetworkWithTheNewRange` (:408) and `…RemovesTheOldNetwork` (:421);
  - `TestPlanAfterSubnetChangeReportsNoChanges` (:474) expects the new unchanged count, with four resources.
- `example/plugin/plan_test.go:88-109`: `TestPlanOfSubnetChangeReplacesNetworkAndContainer` becomes `TestPlanOfSubnetChangeReplacesNetworkAndUpdatesContainer`. It expects:
  - `-/+ docker "network" "app"`;
  - `~ docker "container" "web"`;
  - no "container … will be replaced" line;
  - a summary of 1 to replace and 2 to update (container and welcome), with init unchanged. Verify the counts by running.
  - Also update the other plan tests' unchanged counts for the extra template.
- `example/plugin/main_test.go:258`: the status tree expects 4 lines.
- `example/plugin/scenarios_test.go` (new; real Docker; `requireDocker` and `applyExampleDir` from `main_test.go`; variants from the `config-swap/`, `config-init/`, `config-init-content/`, `config-remove/` and `config-dangling/` dirs described below). Each test is its own function:
  - **Network swap:** the config variant adds `docker "network" "backend" {}` and changes the container's `name = docker.network.backend.meta.name`. After the second apply:
    - the same container ID;
    - `Networks` has `backend` and not `app`;
    - the IP matches;
    - the next plan reports no changes.
  - **Init-script rebuild:** the config variant changes `template "init"`'s destination (to `init-v2.sh`) and the container's `init_script` follows the reference.
    - The plan (via `plan`) contains `# docker.container.web will be replaced because template.init is replaced` and `-/+ docker "container" "web"`.
    - After apply, there is a new container ID, and the container is running.
    - The next plan reports no changes.
  - **Init-script content edit:** a source-only change updates `template.init` and leaves the container's ID unchanged.
  - **Network removal:** the config variant deletes `docker "network" "app"`, the container's `network` block, and `template "welcome"`'s `network` variable. After apply:
    - NetworkInspect of the old ID is NotFound;
    - the same container is running with no `NetworkSettings.Networks` entry for `app`;
    - the next plan reports no changes.
  - **Dangling reference:** the config variant deletes only `docker "network" "app"`. Apply returns a validation error, and Docker shows the original network and container IDs unchanged.
  - **Init script is mounted:** after the first apply, `ContainerInspect` shows a bind mount of the rendered script at `/docker-entrypoint.d/90-xcl-init.sh`, and the rendered file is executable. This works on every host, unlike reaching the container network.
- `example/plugin/Makefile`: add targets `swap`, `rebuild-init` and `remove-network` that build, apply `./config`, plan the variant, apply it, show status and destroy, modelled on `replace`. The variants live in new dirs `config-swap/`, `config-init/` and `config-remove/` (used by the Makefile and the tests), plus `config-init-content/` and `config-dangling/` (tests only). The Makefile can't use test-time rewriting, so the walkthrough and the tests share one copy of each variant. Each dir has a header comment saying what the change demonstrates.
- `example/plugin/smoke_test.go`: no change expected. If its apply-status output assertions count resources, update them for four.

**Complexity**: Medium
**Token estimate**: ~45k tokens
**Agent strategy**: Single agent, sequential (real Docker, one engine).

### Task: Guides, README and changelog describe the changes

**File changes**
- `docs/plugin-developer-guide.md:15-28`: interface block with the new signatures.
- `docs/plugin-developer-guide.md:135-203`: decide/act pseudocode shows `Changed(old, result, changes, dependencies)` and `Update(new, changes, dependencies)`.
- `docs/plugin-developer-guide.md:265-400`:
  - new "#### The changed settings" subsection: the `PropertyChange` fields, `Unknown` (decide only), `Sensitive` (real values, self-masking), plain JSON values, and `At`/`Within` with `entity.Path{}.Attribute("network")`;
  - the container `Changed` example replaced with the new rules;
  - `### Update(ctx, resource, changes, dependencies)` rewritten, with the hot swap code (previous attachments from `Before`, disconnect/connect) and the init-script replace rule.
- `docs/plugin-developer-guide.md:636-644`: the person example signature.
- `docs/plugins.md:31-39,78-85,112-120`: signatures.
- `docs/plugins.md:155-190`: the proto excerpt gains `StepKind`, `PathStep`, `PropertyChange`, `ChangedRequest.changes = 6` and `UpdateRequest.changes = 4, dependencies = 5`.
- `docs/parser-lifecycle.md:115-140`: decide computes the changes before `Changed` with the plan's comparison; act recomputes them for `Update` with real values.
- `docs/README.md:23,37`: mention `PropertyChange` in the `entity/` row.
- `README.md:227-300`: the plugin example section describes the hot swap, the subnet rebuild keeping the container, the init script, and the new make targets.
- `plugins/example/README.md:125-133`: the Update and Changed bullets mention the lists.
- `CHANGELOG.md:1-3`: new top entry `## 20261008194959-b128e508-update-property-changes`, written as prose paragraphs in the existing style:
  - what Changed and Update are told;
  - unknown and real values;
  - sensitive values;
  - the path helpers;
  - the protocol;
  - the Docker example's new behaviour.

  It ends with a `**Breaking:**` list: the `ResourceProvider.Changed`/`Update` signatures, the `ProviderAdapter`/`PluginHost`/`PluginEntityProvider` signatures, the protocol fields (external plugins must be rebuilt), and `diff.Path` now aliasing `entity.Path`, which is source compatible.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: 2 parallel agents: (a) `docs/plugin-developer-guide.md` and `plugins/example/README.md`; (b) `docs/plugins.md`, `docs/parser-lifecycle.md`, `docs/README.md`, `README.md` and `CHANGELOG.md`. Both copy code excerpts from the finished example code.

### Task: Website shows the changed settings and the hot swap

**File changes**
- `xcl-website:src/pages/replacement.mdx:23-29`: the signature block (`plugins/provider.go`) shows `Changed` with `changes` and `Update`.
- `xcl-website:src/pages/replacement.mdx:81-120`: after the dependency section, a new "## What a resource is told about its settings" section: the `PropertyChange` struct block (`entity/property_change.go`), not-yet-known values when deciding, real values when updating, sensitive values, and matching with `Within`.
- `xcl-website:src/pages/replacement.mdx:145-197`: the Docker worked example uses the new container `Changed` (network changes update; a replaced non-network dependency replaces). Replace :195-197 ("Both providers' `Update` returns the resource unchanged…") with the hot swap `Update` excerpt and the init-script rule.
- `xcl-website:src/pages/replacement.mdx:267-307`: the external plugins proto excerpt gains the new messages and fields.
- `xcl-website:src/pages/examples/plugins.mdx:20-25,210-253`: the intro and providers describe the in-place network update and the init script.
- `xcl-website:src/pages/examples/plugins.mdx:382-435`: "Update or replace" shows the new container `Changed` and `Update` excerpts, and the network `Destroy` detaching. Remove the "both providers' `Update` methods return the resource unchanged" claim (:433-435).
- `xcl-website:src/pages/examples/plugins.mdx:457-520`: the template section adds `mode` and the `template "init"` block.
- `xcl-website:src/pages/examples/plugins.mdx:937-1166`: "Run it" adds the `swap`, `rebuild-init` and `remove-network` walkthroughs with their plan output (copied from a real run), and "What to notice" says network changes never rebuild the container.
- `xcl-website:src/components/Nav.astro:8-30`: no change (the pages already exist and keep their URLs, per the no-redirects gotcha).

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential; it reads the finished example code for excerpts.

## Testing Strategy

**Kinds of tests.**
- **Unit tests** cover:
  - the new `entity` vocabulary: the path helpers, `At`/`Within`, and the masking forms;
  - the two projections of the shared comparison;
  - the proto converters;
  - each provider that overrides `Changed`.
- **Core integration tests** drive real applies through the parser with the recording `TestPlugin`. They assert what `Changed` and `Update` were told, getting saved state only from earlier real applies.
- **Contract tests** check the two hosts against each other: the same edit is applied through the in-process and external e2e fixtures with the recording provider, and the told lists are compared.
- **End-to-end example tests** run the Docker example against a real engine. Each scenario is a config variant applied over a first apply's state, then checked through the Docker API and a follow-up plan.
- **Regression tests:** the full existing suite, every example's tests and the external test plugins must build and pass after the signature break. The tests that encode the old replace-on-network behaviour are rewritten, not deleted.

**Where coverage is heaviest, and why.** The core's change plumbing gets the most tests, because it carries the spec's correctness promises:
- the right settings with the right previous and new values;
- unknown values flagged only when deciding;
- real values at update time, including those from dependencies applied earlier;
- sensitive values real for plugins and hidden everywhere else.

The Docker example gets scenario tests, because its behaviour is the spec's user-visible proof.

**Load-bearing assertions.**
- **Edited setting.** When a setting is edited and the plugin answers update, `Update` is told exactly that setting, its location, its previous value and its new value, and nothing unchanged.
- **Dependency only.** When only a dependency is replaced and the plugin answers update, `Update` is told no setting changes and is told the replaced dependency, exactly as `Changed` was.
- **Decide.** `Changed` is told the edited setting with its previous and new values.
- **Unknown values.** A value taken from a dependency the same apply replaces is flagged not-yet-known when deciding. At update time, a value taken from a dependency created or changed earlier in the apply holds its real value.
- **Sensitive values.** A sensitive setting's change reaches the plugin with real values. The plan, the event stream and the captured logs show it hidden, and printing or logging the change hides it.
- **Matching.** A change at the first network's name matches that path and the `network` setting, and does not match `image`. These are separate positive and negative tests.
- **Parity.** In-process and external plugins are told identical change and dependency lists, when deciding and when updating.
- **Docker scenarios.**
  - A network swap keeps the container's ID and moves it to the new network.
  - A subnet rebuild keeps the container's ID and reattaches it.
  - An init-script producer replacement gives a new container ID, and the plan names the reason.
  - Removing a network and every reference leaves the container with no network.
  - A dangling reference fails validation and changes nothing.
  - Every scenario ends with a plan that reports no changes.

**Fit with existing conventions.**
- Tests use testify `require`, one behaviour per function, positive and negative cases apart, no table-driven tests, and live next to the code they test.
- Mocks are regenerated with Mockery.
- Shared helpers needed by more than one package go in `internal/testutil`.
- The example's tests use only public packages.

**Deliberate gaps.**
- No test reads the documentation or changelog; documentation is checked by review.
- No test of the generated proto code itself; the converters and the parity test cover the wire.
- The Docker scenarios don't run without an engine and skip as today. The core and contract tests carry the guarantees there.

**Success metrics.**
- **The container updates in place using only what it is told** — **behavioural test**. The container's unit tests drive `Update` against a strict Docker client mock that permits only disconnect, connect and the post-reconnect inspect for the new address; any other call, such as an inspect to discover previous state, fails the test. The real-Docker scenarios confirm the same container ID survives a network change.
- **Applies rebuild only what genuinely needs it** — **behavioural test**. The example scenarios assert that:
  - an address-range change replaces only the network and updates the container (same ID);
  - a network swap updates the container (same ID) and replaces nothing;
  - an init-script producer replacement replaces the container (new ID);
  - each scenario's plan names the replace and update counts.
- **Real Docker state matches the configuration, and the next plan reports no changes** — **behavioural test**. Each example scenario checks the network, container and attachments through the Docker API, then runs a plan and asserts "no changes".
- **The settings a plugin is told match the plan's** — **behavioural test**. Core tests run a diff and an apply on the same edit and assert that the paths in the plan equal the paths told to `Changed` and to `Update`. They cover a plain edit, a sensitive edit (values hidden in the plan, real for the plugin) and a not-yet-known value (unknown in the plan and at decide time, real at update time).

**Manual reviews.**
- **Manual — captured in the implementation test plan:** run the example's Makefile walkthroughs (network swap, subnet rebuild, init-script rebuild, network removal) against a local Docker engine, and confirm the plan and status output read correctly to a person.
- **Manual — captured in the implementation test plan:** review the updated guides and the two website pages for accuracy against the new signatures and the example code, and confirm the website builds and passes its check.

**Per-task test placement.**
- **Changed setting type:** `entity/path_test.go` and `entity/property_change_test.go` (unit), plus an alias check in `diff/path_test.go`.
- **Contract:** `plugins/change_proto_test.go` (converter round trips), `plugins/grpc_plugin_host_test.go`, `plugins/grpc_server_test.go` and `plugins/adapter_test.go`. Migrated fakes keep the whole suite green.
- **One comparison:** `internal/parser/change_views_test.go`. The existing `diff_changes_test.go` sensitive tests are unchanged.
- **Decisions told:** `internal/parser/decide_test.go`. The diff tests prove plans are unchanged.
- **Updates told:** `internal/parser/update_changes_test.go`, including the event, log and plan masking checks and the plan-equals-told path checks.
- **Parity:** `e2e/plugin_changes_test.go` with the `e2e/fixtures/recorder` provider.
- **Network detach:** `example/plugin/plugins/docker/resources/network_test.go` (mock) and `docker_test.go` (real Docker).
- **Hot swap:** `example/plugin/plugins/docker/resources/container_test.go` (strict mock).
- **Init script:** `example/plugin/plugins/template/template_test.go` and `container_test.go`.
- **Scenarios:** `example/plugin/scenarios_test.go`, plus the updated `main_test.go` and `plan_test.go` (real Docker).
- **Docs:** reviewed by hand; the website runs `npm run build` and `make check`.

## Project References

- Spec `20261008194959-b128e508-update-property-changes` (`spektacular spec file read …`): the source of truth.
- Design `replacement-and-dependency-changes.md` from the `design` source: governs unchanged/update/replace, the dependency list and destroy-then-create. It is binding and unchanged by this plan.
- Prior plan `20261008132354-4538504f-replacement-deps`: historical. Its sequencing and the dependency-list threading are the model here.
- Knowledge (xclconfig):
  - conventions: code-style, project-structure, testing-and-mocking, test-state-from-real-apply, shared-test-helpers, assert-ordering-on-graph-parents, never-modify-dependencies, development-standards;
  - gotchas: example-modules-cannot-import-internal, custom-marshaljson-changes-internal-hops, plugin-types-rebuilt-with-structof;
  - learnings: secret-fixture-for-sensitive-tests;
  - glossary: entity.
- Knowledge (xcl-website): gotchas/no-redirects-on-page-removal (the pages keep their URLs).
- Repo roots: xclconfig `/home/nicj/code/github.com/jumppad-labs/xcl`, xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`. Examples are separate Go modules (`example/plugin`, `example/prettylog`, `example/configonly`). `e2e/` and `plugins/example/` are in the root module.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The contract task is the only High task. It is split as its notes describe, so no single agent holds the whole provider migration.

## Migration Notes

- This is a breaking plugin contract and protocol change. External plugins must be rebuilt against the new `plugins` package. There is no shim.
- `diff.Path`, `diff.Step` and `diff.StepKind` become aliases of the `entity` types. This is source compatible for callers, and the plan's JSON is unchanged.
- Saved state is unchanged. No state migration is needed.
- The Docker example's subnet change now updates the container rather than replacing it. Its `make replace` walkthrough text changes accordingly.

## Performance Considerations

- The comparison now runs once per saved resource in the decide pass, instead of only for pending ones, plus once per updated resource in the act pass. It is a reflection walk over one resource's settings, small next to the provider calls it accompanies.
- Each change adds one JSON encode/decode per value for normalisation, and for external plugins one more each way on the wire. These are negligible for configuration-sized values.
