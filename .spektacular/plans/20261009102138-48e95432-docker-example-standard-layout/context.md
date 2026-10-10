---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Context: 20261009102138-48e95432-docker-example-standard-layout

## Current State Analysis

The Docker plugin lives at `example/plugin/plugins/docker` inside the `example/plugin` Go module (`example/plugin/go.mod:1`). It is a `main` package (`example/plugin/plugins/docker/plugin.go:1`, `main.go:15-17`), so its plugin type is not importable. Its block types and providers share the `resources` package (`resources/network.go:19-151`, `resources/container.go:24-436`). The providers import the Docker SDK directly (`resources/network.go:8-9`, `resources/container.go:11-15`) alongside a narrow `client.Docker` interface (`client/client.go:29-49`) with a Mockery double (`client/mocks/mock_docker.go`, generated from `example/plugin/.mockery.yml`). The application imports `resources` to read state (`status.go:13,54,63`) and `client` for its pre-flight `Ping` (`main.go:44,200,218,262`), so its build compiles the Docker SDK. Provider methods are out of lifecycle order (`resources/network.go:84-124`: `Destroy` before `Read`), and helpers are interleaved with exported methods (`resources/container.go:129-229`). The template plugin's `Changed` comes first (`plugins/template/template.go:65`). The network's `Changed` compares `old.Subnet != new.Subnet` (`resources/network.go:126-132`). CI builds and vets `example/configonly example/plugin example/prettylog` (`.github/workflows/go.yml`), and the root e2e runs each example's tests (`e2e/examples_test.go:29-55`). The README (`README.md:227-243`), the developer guide (`docs/plugin-developer-guide.md:414,469,520,599`) and four site pages (`xcl-website:src/pages/examples/plugins.mdx`, `replacement.mdx`, `plugin-logging.mdx`, `events.mdx`) quote these paths.

## Per-Task Technical Notes

Requirement → repo and files:
- **Entity types stand alone** → xcl: `example/plugin/plugins/docker/entities/*`, `example/plugin/status.go`, `example/plugin/main.go`, `example/plugin/go.mod`.
- **Consistent file order** → xcl: every `.go` file under `example/plugin/` (Docker plugin, `plugins/template/`, application).
- **The plugin example follows the standard** → xcl: `example/plugin/plugins/docker/**`, `.github/workflows/go.yml`, `e2e/examples_test.go`, `.gitignore`.
- **Documentation reflects the example's layout** → xcl: `README.md`, `docs/plugin-developer-guide.md`, `docs/plugins.md`; xcl-website: `src/pages/examples/plugins.mdx`, `src/pages/replacement.mdx`, `src/pages/plugin-logging.mdx`.

Paths below are relative to the xcl root `/home/nicj/code/github.com/jumppad-labs/xcl` unless prefixed `xcl-website:` (root `/home/nicj/code/github.com/jumppad-labs/xcl-website`). `PD` = `example/plugin/plugins/docker`.

### Task: Create the plugin module with its entity types and Docker SDK client

**File changes**:
- `example/plugin/plugins/docker/go.mod` (new) — `module github.com/jumppad-labs/xcl/example/plugin/plugins/docker`, `go 1.25.0`, require `github.com/docker/docker v28.5.2+incompatible`, `github.com/opencontainers/image-spec v1.1.1`, `github.com/stretchr/testify v1.12.1`, `github.com/jumppad-labs/xcl v0.0.0-00010101000000-000000000000`; `replace github.com/jumppad-labs/xcl => ../../../..`. Generate `go.sum` with `go mod tidy` (copy versions from `example/plugin/go.mod:5-84`; do not bump).
- `example/plugin/plugins/docker/entities/network.go` (new) — move `Network` from `example/plugin/plugins/docker/resources/network.go:19-30` verbatim; package doc comment for `entities` (block types only, an application reading state imports only this package, e.g. `xcl.FindByType[entities.Container]`), replacing the `resources` package doc at `example/plugin/plugins/docker/resources/labels.go:1-4`.
- `example/plugin/plugins/docker/entities/container.go` (new) — move `Container` and `NetworkAttachment` from `example/plugin/plugins/docker/resources/container.go:24-61` verbatim. Imports only `github.com/jumppad-labs/xcl/types`.
- `example/plugin/plugins/docker/client/docker/docker.go` (new) — move `example/plugin/plugins/docker/client/client.go:1-85` unchanged except `package docker` (import the SDK client as `dockerclient` as today); keep `Docker`, `New`, `Ping`, `newSDKClient`, `pingTimeout`.
- `example/plugin/plugins/docker/client/docker/docker_test.go` (new) — move `example/plugin/plugins/docker/client/client_test.go:1-27`.
- `example/plugin/plugins/docker/client/docker/mocks/mock_docker.go` (generated) — regenerate; replaces `example/plugin/plugins/docker/client/mocks/mock_docker.go`.
- `example/plugin/plugins/docker/.mockery.yml` (new) — copy `example/plugin/.mockery.yml:1-14`, package key `github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/docker: interfaces: Docker: {}` (the containers entry is added by the next task).
- `example/plugin/plugins/docker/Makefile` (new) — targets `build` (`go build -o build/docker-plugin ./cmd/docker`, added once the entry point exists; until then `go build ./...`), `test` (`go test -v ./...`), `generate` (`$(MOCKERY)` with `MOCKERY := go run github.com/vektra/mockery/v3@v3.8.0`, as `example/plugin/Makefile:19`), `clean`.
- The old `PD/resources/*` and `PD/client/*` stay in place until the plugin-type task removes them. While both exist, the old packages belong to the new module. Keep them compiling by repointing their imports (`.../plugins/docker/client` → `.../plugins/docker/client/docker`) if needed, or do this task and the next three in one branch without intermediate builds of the old packages.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Add the container task layer over the Docker SDK client

**File changes**:
- `example/plugin/plugins/docker/client/containers/containers.go` (new) — package `containers`: exported `ErrNotFound`, `Attachment`, `NetworkSpec`, `ContainerSpec`, `Tasks` (the 11 methods in plan § Data Structures), `New(docker.Docker) Tasks`; unexported `tasks` struct holding `docker.Docker`. Move the SDK-shaped code out of the providers, keeping the exact calls:
  - `CreateNetwork` ← `PD/resources/network.go:59-75` (`Driver: "bridge"`, `Attachable: true`, `Labels`, IPAM only when `Subnet != ""`); returns `resp.ID`.
  - `NetworkContainers` ← `PD/resources/network.go:87-101` (`NetworkInspect` with empty options, collect `inspect.Containers` keys, `sort.Strings`).
  - `RemoveNetwork` ← `NetworkRemove`; `DisconnectNetwork` ← `NetworkDisconnect(..., true)`; `ConnectNetwork` ← `NetworkConnect(..., &network.EndpointSettings{Aliases: aliases})` (`PD/resources/container.go:176,412`).
  - `EnsureImage` ← `pullImage` `PD/resources/container.go:153-180`, keeping its messages "unable to list images: %w" and "unable to pull image %s: %w" and the progress drain.
  - `CreateContainer` ← `PD/resources/container.go:96-117`: `container.Config{Image, Cmd, Env: environment(...), Labels}`, networking from `spec.Network`, `HostConfig` with `Binds: []string{InitScript + ":" + initScriptPath + ":ro"}` when `InitScript != ""` else empty `HostConfig{}`, platform nil, name; move `initScriptPath` (`container.go:132`) and `environment` (`container.go:221-229`) here.
  - `StartContainer` ← `ContainerStart(..., container.StartOptions{})`; `StopContainer` ← `ContainerStop(..., container.StopOptions{})`; `RemoveContainer` ← `ContainerRemove(..., container.RemoveOptions{Force: true})`.
  - `ContainerAddresses` ← `ContainerInspect`, return `map[name]IPAddress` for non-nil endpoints, empty map when `NetworkSettings == nil` (`container.go:192-219`).
  - Every method returns Docker's error as is, except a `dockerclient.IsErrNotFound(err)` error, returned as `fmt.Errorf("%w: %w", ErrNotFound, err)` so `errors.Is` matches the sentinel and the message keeps Docker's text. `EnsureImage` keeps its wrapped messages.
- `example/plugin/plugins/docker/client/containers/containers_test.go` (new) — tests against `client/docker/mocks.MockDocker`, one behaviour per function, positive and negative separate, no tables. Port the SDK-shape assertions out of `PD/resources/network_test.go:29-130` (labelled bridge, subnet IPAM, ID, errors), `PD/resources/network_test.go:151-370` (inspect/disconnect/remove calls, sorted order), `PD/resources/container_test.go:85-465` (pull missing / skip present, name+image+labels, first network aliases, read-only bind by path, empty host config, start, inspect address, errors), plus not-found translation for each method that can see it.
- `example/plugin/plugins/docker/client/containers/mocks/mock_tasks.go` (generated).
- `example/plugin/plugins/docker/.mockery.yml` — add `github.com/jumppad-labs/xcl/example/plugin/plugins/docker/client/containers: interfaces: Tasks: {}`.

**Complexity**: Medium
**Token estimate**: ~60k tokens
**Agent strategy**: Write `containers.go` first. Then use 2 parallel agents for the tests, one for network and image methods and one for container methods. Generate mocks last.

### Task: Move the providers onto the task layer

**File changes**:
- `example/plugin/plugins/docker/providers/network.go` (new) — from `PD/resources/network.go:32-151`; field `tasks containers.Tasks`; `NewNetworkProvider(tasks containers.Tasks) plugins.ResourceProvider[*entities.Network]`. `Create` calls `tasks.CreateNetwork(ctx, n.Meta.Name, containers.NetworkSpec{Subnet, Labels: labels(n.Meta)})`. `Destroy` uses `NetworkContainers` and translates `errors.Is(err, containers.ErrNotFound)` to the "network already removed" path, then `DisconnectNetwork`/`RemoveNetwork` with the same not-found tolerance. Keep every message and log line from `network.go:74,88-117`.
- `example/plugin/plugins/docker/providers/container.go` (new) — from `PD/resources/container.go:63-436`. `Create` calls `tasks.EnsureImage`, computes the absolute init script path (keeping "unable to mount the init script of container %s: %w"), `tasks.CreateContainer` with `Network` = first attachment, then the connect/start/address sequence (`startContainer`, using `ConnectNetwork`, `StartContainer`, `ContainerAddresses` + `firstAddress` over the map), and `RemoveContainer` on failure with `errors.Join`. `Destroy`, `Update` and `connect` use the task calls with `ErrNotFound` replacing `dockerclient.IsErrNotFound`. `Changed`, `replaceSettings`, `equalStrings` are unchanged.
- `example/plugin/plugins/docker/providers/attachments.go` (new) — move `PD/resources/attachments.go:1-148` (types now `entities.NetworkAttachment`).
- `example/plugin/plugins/docker/providers/labels.go` (new) — move `PD/resources/labels.go:6-27` (exported constants kept).
- `example/plugin/plugins/docker/providers/network_test.go`, `container_test.go` (new) — move `PD/resources/network_test.go`, `container_test.go`, switching `mocks.MockDocker` to `containers/mocks.MockTasks`. Keep every test name and intent. Expectations become task calls (for example `TestContainerUpdateMovesTheContainerToTheRenamedNetwork` expects `DisconnectNetwork(old)`, `ConnectNetwork(new, aliases)`, `ContainerAddresses`). `TestContainerUpdateWithNothingToldMakesNoDockerCalls` asserts no task calls. Helpers `expectImagePresent`, `expectContainerCreated` and the rest at `container_test.go:42-83,481-507` are rewritten for the task double.
- `example/plugin/plugins/docker/providers/attachments_test.go`, `labels_test.go` (new) — move unchanged except package and entity type names.
- `example/plugin/plugins/docker/providers/docker_test.go` (new) — move `PD/resources/docker_test.go:1-243`. Build providers with `containers.New(realDocker)`. Keep `requireDocker` (via `client/docker.Ping`), and keep the cleanup helpers using the `client/docker` SDK interface (test files may import the SDK, since the "only client/ imports backend libraries" rule is read as applying to non-test source).

**Complexity**: High
**Token estimate**: ~90k tokens
**Agent strategy**: Parallel analysis. One agent ports `network.go` and its tests, another ports `container.go` and its tests, while `attachments`/`labels`/`docker_test` move sequentially. Integrate and run the module's tests.

### Task: Make the plugin type importable and serve it from its own entry point

**File changes**:
- `example/plugin/plugins/docker/plugin.go` — rewrite as `package docker` (from `PD/plugin.go:1-56`). `Init` calls `docker.New()` from `client/docker` (alias the import, e.g. `dockerclient`, to avoid clashing with the package name), wraps it once with `containers.New`, and passes the same `Tasks` to `providers.NewNetworkProvider` and `providers.NewContainerProvider`. It registers `&entities.Network{}` and `&entities.Container{}` under the same type names, with the same debug log line.
- `example/plugin/plugins/docker/cmd/docker/main.go` (new) — from `PD/main.go:1-17`. Calls `plugins.Serve(&docker.Plugin{})`. The doc comment says build with `make build` in the plugin directory or the example.
- `example/plugin/plugins/docker/main.go` — delete.
- `example/plugin/plugins/docker/resources/` — delete the whole directory.
- `example/plugin/plugins/docker/client/client.go`, `client_test.go`, `client/mocks/` — delete.
- `example/plugin/plugins/docker/Makefile` — `build` target becomes `go build -o build/docker-plugin ./cmd/docker`.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Point the example application at the entity types only

**File changes**:
- `example/plugin/go.mod` — add `require github.com/jumppad-labs/xcl/example/plugin/plugins/docker v0.0.0-00010101000000-000000000000` and `replace github.com/jumppad-labs/xcl/example/plugin/plugins/docker => ./plugins/docker`. Run `go mod tidy`. `github.com/docker/docker` stays only because the tests need it (`main_test.go:14-16`, `scenarios_test.go:10-11`).
- `example/plugin/status.go:13` — import `.../plugins/docker/entities`. `resources.Network`/`resources.Container` at `status.go:54,63` become `entities.*`.
- `example/plugin/main.go:44,200,218,262` — drop the `client` import. Call `pingDocker(context.Background())` instead of `client.Ping`. Update the package doc (`main.go:1-34`) for the plugin's new location (`./plugins/docker`, served by `./plugins/docker/cmd/docker`).
- `example/plugin/engine.go` (new) — `pingDocker(ctx)`: read `DOCKER_HOST` (default `unix:///var/run/docker.sock`). For `unix://`, use an `http.Client` with a `DialContext` to the socket path. For `tcp://`, use `http://host:port`. Use a 5 s timeout, `GET /_ping`, and require a 200. Errors return `fmt.Errorf("no Docker engine reachable: %w", err)`. Other schemes return nil. The doc comment explains why the application does not use the Docker SDK.
- `example/plugin/engine_test.go` (new) — `TestPingDockerSucceedsWhenAnEngineAnswers` (an `httptest`-style server on `net.Listen("unix", tempdir/sock)` answering `/_ping` with OK, `t.Setenv("DOCKER_HOST", "unix://...")`), `TestPingDockerFailsWhenNoEngineIsReachable` (missing socket, message contains "no Docker engine reachable"), `TestPingDockerSkipsAnAddressItCannotDial` (`ssh://host` returns nil).
- `example/plugin/main_test.go:20-22,38,58,137-145` — imports become `.../plugins/docker/entities` and `.../plugins/docker/client/docker`. `requireDocker` uses `pingDocker`. `TestMain`'s build runs `go build -o <abs dockerPlugin> ./cmd/docker` with `build.Dir = "plugins/docker"`. `resources.CreatedByValue`/`LabelCreatedBy` (`main_test.go:159`) come from `.../plugins/docker/providers`. `resources.X` → `entities.X` everywhere else. No assertion changes.
- `example/plugin/scenarios_test.go:15` — import `entities`. `resources.` → `entities.`. Nothing else.
- `example/plugin/smoke_test.go:26` — the plugin build runs from `plugins/docker` at `./cmd/docker` with an absolute `-o`.
- `example/plugin/Makefile:20-27,81-83` — `build` builds the plugin with `go -C plugins/docker build -o ../../$(DOCKER_PLUGIN) ./cmd/docker`. `generate` delegates to `$(MAKE) -C plugins/docker generate`. `test` also runs `$(MAKE) -C plugins/docker test`. Update the header comment (`Makefile:1-5`).
- `example/plugin/.mockery.yml` — delete (no interfaces left in the app to mock).

**Complexity**: Medium
**Token estimate**: ~45k tokens
**Agent strategy**: Two parallel agents. One does the engine check and its tests. The other does the import/build rewiring of the app and its tests. Then run the app's suite with and without Docker.

### Task: Build and test the plugin module in CI and the end-to-end runner

**File changes**:
- `.github/workflows/go.yml` (build-minimum-go job, "Build and vet the examples" step) — the loop list becomes `example/configonly example/plugin example/plugin/plugins/docker example/prettylog`.
- `e2e/examples_test.go:44-48` — add `TestDockerPluginExampleTestsPass` calling `runExampleTests(t, filepath.Join("plugin", "plugins", "docker"))` with `t.Parallel()`. The failure message names `filepath.Base(dir)`, i.e. `docker`.
- `.gitignore:9-19` — add `example/plugin/plugins/docker/build/` and the stray binary `example/plugin/plugins/docker/docker`.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Put every plugin example file in the standard order

**File changes**:
- `example/plugin/plugins/docker/providers/network.go` — order: `networkProvider` type, assertion, `NewNetworkProvider`, then `Init, Create, Read, Changed, Update, Destroy, Functions` (today `Destroy` sits before `Read`, `network.go:84-124`). Unexported `replaceSettings` list (`entity.Path{}.Attribute("subnet")`) at the end. `Changed` replaces when any change is `Within` a replace setting, answers `entity.Update` when `len(changes) > 0`, and otherwise returns `DefaultChanged.Changed`. Doc comment updated.
- `example/plugin/plugins/docker/providers/container.go` — type, assertion, constructor, lifecycle methods in order (`Destroy` moves after `Update`). Then the unexported helpers `startContainer`, `firstAddress`, `connect`, `equalStrings`, then unexported `replaceSettings` (today interleaved at `container.go:129-229,262-273`).
- `example/plugin/plugins/docker/providers/attachments.go` — helpers first, then the unexported vars `networksSetting` and `removed` (today at the top, `attachments.go:10-14`).
- `example/plugin/plugins/docker/providers/labels.go` — exported consts, then `labels`.
- `example/plugin/plugins/docker/client/containers/containers.go`, `client/docker/docker.go` — exported types/vars/consts, exported funcs/methods, then unexported, then unexported consts (`pingTimeout`, `initScriptPath`).
- `example/plugin/plugins/template/template.go:50-191` — `Template` type and the exported consts first. Then `provider`, its assertion, and methods `Init, Create, Read, Changed, Update, Destroy, Functions` (today `Changed` is first, at `template.go:65`). Then `render`, `fileMode`, then `defaultMode` (`template.go:50`).
- `example/plugin/plugins/template/plugin.go` — check, already in order.
- `example/plugin/main.go`, `status.go`, `inspect.go`, `plan.go`, `engine.go` — package `main` has no exported members beyond `main`. Order is: exported types (`statusNode` etc. are unexported, so they stay with the helpers), functions, then unexported consts and vars at the end (`main.go:52-75` `defaultStateDir`, `dockerPluginName`, `usage` move after the functions; `status.go:19` `shortIDLength` likewise). Treat `main` as the file's public entry and keep it first.
- `example/plugin/plugins/docker/entities/*.go`, `plugin.go`, `cmd/docker/main.go` — confirm order.
- `example/plugin/plugins/docker/providers/network_test.go` (`TestNetworkChangedReplacesOnSubnetChange`, ported from `PD/resources/network_test.go:386-397`) — today it passes `nil` changes and relies on comparing `old.Subnet`/`new.Subnet`. With the design's `change.Within` form it must pass the `PropertyChange` xcl would report (`Path: entity.Path{}.Attribute("subnet")`, `Before`/`After` the two ranges). The assertion (`entity.Replace`) is unchanged. Add `TestNetworkChangedUpdatesWhenANonReplaceSettingChanges` (a change at `depends_on`, a `types.ResourceBase` setting, answers `entity.Update`). `TestNetworkChangedReportsNoChangeForIdenticalNetwork` stays as is.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Two parallel agents. One takes the Docker plugin files, the other the template plugin and app files. Then run every suite.

### Task: Add a sample configuration and end-to-end test to the plugin

**File changes**:
- `example/plugin/plugins/docker/examples/basic/main.xcl` (new) — `resource "docker" "network" "xcl_plugin_basic" { subnet = "10.73.0.0/24" }` and `resource "docker" "container" "xcl_plugin_basic" { image = "nginx:1.27-alpine" network { name = resource.docker.network.xcl_plugin_basic.meta.name } }`. Match the block syntax used in `example/plugin/config/main.xcl`, and pick a subnet unused by `example/plugin/config*/main.xcl`.
- `example/plugin/plugins/docker/e2e/e2e_test.go` (new) — package `e2e`. `TestMain` builds `../cmd/docker` into a temp dir (as `example/plugin/main_test.go:29-50`). `requireDocker` via `client/docker.Ping`. Add `TestBasicExampleAppliesThenPlansNoChanges` (`xcl.NewConfig(xcl.WithRegistry(local), xcl.WithStatePath(t.TempDir()))`, with `local := registry.NewLocal(); local.RegisterExternalPlugin(binary)`; `Apply("../examples/basic")`; destroy registered in `t.Cleanup`; a second Config on the same state calls `Diff([]string{"../examples/basic"})`, requiring `Changed() == 0`) and `TestBasicExampleDestroyLeavesNothingInDocker` (after `Destroy`, inspecting the container and network IDs from `xcl.Find[entities.*]` returns not found through `client/docker`). Use the registry and Config API exactly as `example/plugin/main.go:271-293` uses it.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Write a README for the plugin

**File changes**:
- `example/plugin/plugins/docker/README.md` (new) — sections: what it provides (`docker "network"`, `docker "container"`); layout (each directory and what it holds, matching the design `plugin-layout.md`); commands (`make build`, `make test`, `make generate`, `make clean`); registering in-process (`local.RegisterPlugin(&docker.Plugin{})`) and as a separate program (`local.RegisterExternalPlugin("<path>/docker-plugin")`, as `example/plugin/main.go:280-281`); reading state with only `entities`; how the e2e and real-engine tests skip without Docker.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Update the README and guides to the new layout

**File changes**:
- `README.md:227-243` — Docker plugin bullet: plugin type in `plugin/plugins/docker`, served by `plugins/docker/cmd/docker`; block types in `plugins/docker/entities`, providers in `plugins/docker/providers`, the SDK interface in `plugins/docker/client/docker`, and the task layer in `plugins/docker/client/containers`, with Mockery doubles beside each; its own module, README, sample and e2e; the application imports only `entities`. Fix links accordingly. Lines `README.md:315-375` link only config dirs (unchanged), so check them.
- `docs/plugin-developer-guide.md:414,469,520,599` — links become `../example/plugin/plugins/docker/providers/container.go`, `.../providers/network.go`, `.../providers/attachments.go`. Re-quote any code block around these lines from the rebuilt source (task calls instead of SDK calls, network `Changed` explicit form).
- `docs/plugins.md:289,580` — directory links to `example/plugin` stay. Check the surrounding prose does not name `resources`.
- `e2e/COVERAGE.md` — no change (mentions only `example/plugin/main_test.go`).
- `CHANGELOG.md` — new top entry `## 20261009102138-48e95432-docker-example-standard-layout`, above the current top entry, in the same shape: prose saying the Docker plugin example is rebuilt to the standard plugin layout as its own module (`github.com/jumppad-labs/xcl/example/plugin/plugins/docker`), with entities, providers, a `client/docker` SDK interface and a `client/containers` task layer, its own README, sample and e2e, and that the example application imports only `entities` and checks the engine with the standard library; behaviour is unchanged. Then **Breaking:** the plugin's import path and package layout changed; `example/plugin/plugins/docker/resources`, `.../client` and `.../main.go` are removed.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Update the documentation site pages to the new layout

**File changes**:
- `xcl-website:src/pages/examples/plugins.mdx:150,192,204-256,299,375,429,454,486,512,570` — titles `example/plugin/plugins/docker/resources/container.go` become `.../entities/container.go` (type blocks) or `.../providers/container.go` (provider blocks). Likewise network. `plugin.go` code re-quoted (`entities.Network{}`, `containers.New`). The client section (`:251-256`) becomes `client/docker/docker.go` plus a short `client/containers/containers.go` excerpt. Re-quote every provider snippet from the rebuilt source.
- `xcl-website:src/pages/examples/plugins.mdx:670-690` — the external-plugin prose: the plugin is its own module, `plugin.go` in its root package, `cmd/docker/main.go` serves it, providers in `providers`, Docker client layer in `client/`, host imports only `entities`. Retitle the code block to `example/plugin/plugins/docker/cmd/docker/main.go`.
- `xcl-website:src/pages/examples/plugins.mdx:1179-1256` — test snippet titles go to `providers/network_test.go` and `providers/docker_test.go`. The mock location (`:1203`) becomes `client/containers/mocks` (provider tests) and `client/docker/mocks` (task-layer tests), with `make generate` in `plugins/docker`.
- `xcl-website:src/pages/examples/plugins.mdx:896-1166` — `main.go` snippets re-quoted where they changed (no `client.Ping`; `pingDocker`). `status.go` snippet uses `entities`.
- `xcl-website:src/pages/replacement.mdx:241,266,324` — titles go to `providers/network.go` / `providers/container.go`. Re-quote `Changed` (network now explicit).
- `xcl-website:src/pages/plugin-logging.mdx:31` — title goes to `providers/network.go`. Re-quote if lines moved.
- `xcl-website:src/pages/events.mdx:266` — `example/plugin/main.go` is unchanged in path. Re-quote only if the snippet's code changed.
- Verify with `npm run build` in xcl-website.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Two parallel agents, one for `plugins.mdx` and one for `replacement.mdx`, `plugin-logging.mdx` and `events.mdx`. Then a single site build.

## Testing Strategy

Per task:
- **Create the plugin module with its entity types and Docker SDK client**: the moved `client/docker` tests (`New`, `Ping`) pass in the new module. The entities have no behaviour to test.
- **Add the container task layer over the Docker SDK client**: new `client/containers` unit tests against the strict `MockDocker`, one behaviour per function, positive and negative split. Every SDK-shape assertion removed from the provider tests reappears here.
- **Move the providers onto the task layer**: the ported provider unit tests against the strict `MockTasks` keep their names and asserted outcomes. The `Update` tests prove that only the required task calls are made. The real-engine tests skip without Docker.
- **Make the plugin type importable and serve it from its own entry point**: covered by the application's suite, which builds `cmd/docker` and applies through it.
- **Point the example application at the entity types only**: new `engine_test.go` (succeed, fail, unsupported scheme). The scenario, unit and smoke tests pass with unchanged assertions, with and without Docker. Manual: `go list -deps .` in `example/plugin` lists no `github.com/docker/` package.
- **Build and test the plugin module in CI and the end-to-end runner**: the root `e2e` suite runs `TestDockerPluginExampleTestsPass`. The CI minimum-Go job builds and vets the module.
- **Put every plugin example file in the standard order**: every existing suite passes. The network `Changed` tests, including the new update case, pass. Manual file-order review.
- **Add a sample configuration and end-to-end test to the plugin**: the e2e applies, plans zero changes and destroys against Docker, and skips without it.
- **Write a README for the plugin**: manual, every command shown works.
- **Update the README and guides to the new layout** and **Update the documentation site pages to the new layout**: manual path review, and the xcl-website build succeeds.

Success metrics and manual reviews (all **Manual — captured in the implementation test plan**): the layout and docs agree with the design; the application build has no Docker libraries; file order; the site builds and shows only existing paths; the example's `make` targets behave as before.

## Project References

- Spec `20261009102138-48e95432-docker-example-standard-layout` (this plan's spec), in epic `20261009092551-82db0140-plugin-template`.
- Design `plugin-layout.md` from the `design` design source, binding.
- Sibling spec `20261009092551-82db0140-plugin-template`, which owns the layout write-up in the guides and the template.
- Knowledge: `conventions/testing-and-mocking.md`, `conventions/code-style.md`, `conventions/dependencies.md`, `conventions/never-modify-dependencies.md`, `conventions/shared-errors-package.md`, `gotchas/example-modules-cannot-import-internal.md` (xcl store).
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Total estimate is about 370k tokens across 11 tasks. The provider move (High) is the largest. Run the plugin's tests after each Milestone 1 task to keep failures local.

## Migration Notes

No state or contract migration: block types, tags and registration names are unchanged, so state saved before the rebuild loads after it. Developers with a checkout run `make build` again. The plugin binary is now built from the plugin module (`example/plugin/plugins/docker`), and the old `go build ./plugins/docker` from `example/plugin` no longer works. Mocks are regenerated with `make generate` in the plugin directory.

## Performance Considerations

None. The task layer adds one function call per Docker operation. The application binary gets smaller because the Docker SDK is no longer compiled into it.
