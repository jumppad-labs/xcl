---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Context: 20261006112023-f7a185dc-docker-plugin-example

## Current State Analysis

- `example/plugin` (in repo `xclconfig`, `/home/nicj/code/github.com/jumppad-labs/xcl`) holds:
  - an in-process `ExamplePlugin` providing `resource "postgres"` and `resource "redis"` (`example/plugin/internal/plugin.go:33-56`);
  - an external plugin providing `resource "app"` and `resource "ingress"` (`example/plugin/external/main.go:34-60,162-172`);
  - shared types (`example/plugin/resources/resources.go:1-89`);
  - a config with a module (`example/plugin/config/main.xcl`, `config/modules/db/db.xcl`);
  - a `run` function with test-shaped parameters (`example/plugin/main.go:117`).

  None of it talks to a real system.
- The e2e plan `20261006071142-506b8289` (final, not yet implemented) will:
  - make `example/plugin` its own module;
  - swap its smoke test to `os/exec`;
  - strip library tests from `example/plugin/main_test.go`;
  - add `TestPluginExampleTestsPass` to `e2e/examples_test.go`.

  This plan starts from that state. If `example/plugin/go.mod` does not exist when implementation starts, STOP.
- Plugins can already register a type with an empty subtype, and the registry, parser and FQRN handle it:
  - `plugins/registry/plugin_registry.go:162-190`
  - `internal/parser/parser.go:800-846`
  - `internal/resources/fqrn.go:293`

  The gaps:
  - The adapter is named after the subtype (`plugins/plugin.go:70`), so the log tag is lost.
  - Error messages join with `.` (`plugins/plugin.go:147-197`, `plugins/grpc_server.go:81,111,132`).
  - No test covers a no-subtype plugin type, in-process or over gRPC.
- xcl already depends on `github.com/infinytum/raymond/v2 v2.0.5` (`go.mod:20`, `internal/functions/functions.go:286`).
- Docs quoting the example:
  - `README.md:137-160` and the module link at `:1357`
  - `docs/plugins.md:180-182,289,408-410`
  - in `xcl-website` (`/home/nicj/code/github.com/jumppad-labs/xcl-website`): `src/pages/examples/plugins.mdx` (whole page), `plugin-logging.mdx`, `events.mdx`, `index.mdx:213-216,228`. The `state-masking.mdx` call to action also claims the plugin example, but the configuration-example spec owns that whole call to action, so this plan leaves it alone.
- The local engine is Podman behind `/var/run/docker.sock`. CI runners have Docker.

Requirement to repo and task:
- *Stands alone*: xclconfig, `example/plugin/go.mod`, Makefile, `.mockery.yml`. Tasks: Add the Docker client interface…, Rewrite the plugin example application….
- *Reads as real code*: xclconfig, `example/plugin/main.go`. Task: Rewrite the plugin example application….
- *Smoke test*: xclconfig, `example/plugin/smoke_test.go`. Task: Add the wiring and smoke tests.
- *Docker plugin*: xclconfig, `example/plugin/docker/`, `example/plugin/cmd/docker-plugin/`. Tasks: the network, container and serve tasks.
- *Template plugin*: xclconfig, `example/plugin/template/`. Task: Build the template plugin.
- *Provider unit tests without Docker*: xclconfig, `docker/*_test.go`, `template/*_test.go`. Tasks: the provider tasks.
- *Real-Docker tests skip*: xclconfig, `docker/docker_test.go`, `main_test.go`, `smoke_test.go`. Tasks: Serve the Docker plugin…, Add the wiring and smoke tests.
- *Wiring tested*: xclconfig, `example/plugin/main_test.go`. Task: Add the wiring and smoke tests.
- *Library change backwards compatible*: xclconfig, `plugins/`. Task: Support plugin block types with no subtype.
- *Website matches*:
  - xcl-website, `src/pages/**`. Tasks: Rewrite the website's plugin example page, and Update the website pages that quote the plugin example.
  - xclconfig, `README.md`, `docs/plugins.md`. Task: Update xcl's docs for the new plugin example.

## Per-Task Technical Notes

### Task: Support plugin block types with no subtype

**File changes**:
- `plugins/plugin.go:65-79`: in `RegisterResourceProvider`, set `adapter.name = typeName` when `subTypeName == ""`, otherwise `subTypeName`, so that `plugins/adapter.go:86` adds `provider=<type>`. Update the doc comment.
- `plugins/plugin.go:147,157,167,177,187,197`: build the "no registered type found for …" messages with `types.TypeKey(entityType, entitySubType)` (`types/register.go:93`). Check first that importing `types` from `plugins` creates no cycle (today only `plugins/*_test.go` import it). If it does, add an unexported helper with the same logic in `plugins`.
- `plugins/grpc_server.go:81,111,132` (and any sibling Validate/Destroy/Changed messages): same message fix.
- `plugins/plugin_test.go` (or a new `plugins/plugin_subtypeless_test.go`), new tests, one behaviour each:
  - `TestRegisterResourceProviderWithoutSubtypeTagsInitLogsWithType`: use the existing test logger or recorder pattern in `plugins/*_test.go`.
  - `TestPluginBaseCreatesTypeRegisteredWithoutSubtype`.
  - `TestPluginBaseReportsUnknownTypeWithoutTrailingSeparator`: negative, its own function.
- `config_plugin_subtypeless_test.go` (new, root package `xcl`), in-process tests with a test-local plugin (`plugins.PluginBase` + `RegisterResourceProvider(..., "widget", "", &widget{}, &widgetProvider{})`). The type has a repeated nested block (`part { name = ... }`), and the provider records what it receives:
  - `TestApplyCreatesInProcessPluginTypeWithoutSubtype`: the entity is found at `widget.main`.
  - `TestDestroyRemovesInProcessPluginTypeWithoutSubtype`.
  - `TestInProcessPluginTypeWithoutSubtypeRejectsSubtypeLabel`: config `widget "a" "b" {}` fails Apply. Negative, its own function.
  - Use `internal/testutil` recorders if events are needed (allowed for root tests).
- `config_plugin_subtypeless_test.go`, external tests:
  - `TestApplyCreatesExternalPluginTypeWithoutSubtype`: builds `internal/test_fixtures/plugins/subtypeless` with `go build -o` (pattern: `config_plugin_loading_test.go:80-92`), registers it with `RegisterPluginWithPath`, applies, and asserts the entity `widget.main` with its nested `part` values filled from the provider's computed echo field.
  - `TestDestroyRemovesExternalPluginTypeWithoutSubtype`.
- `internal/test_fixtures/plugins/subtypeless/main.go` (new): external plugin `package main` serving one no-subtype type with a repeated nested block and a computed field that echoes the nested values (proves they crossed gRPC). Serve as `example/plugin/external/main.go:162-172` does.
- `internal/test_fixtures/config/subtypeless/main.xcl`, `internal/test_fixtures/config/subtypeless_with_subtype/main.xcl` (new): fixtures for the positive and negative tests.
- If the nested block does not survive the `reflect.StructOf` rebuild (`gotchas/plugin-types-rebuilt-with-structof.md`), fix it in `internal/schema` if the fix is backwards compatible. Otherwise STOP and report: the container's `network` block depends on it.
- Run `go build ./... && go vet ./... && go test ./...` at the root. Every existing test must pass unchanged.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential. The in-process tests come first, then the external fixture.

### Task: Add the Docker client interface and its mock

**File changes**:
- `example/plugin/docker/client.go` (new), package `docker`:
  - The `Client` interface exactly as in the plan's Data Structures. Copy each signature from `$(go env GOMODCACHE)/github.com/docker/docker@v28.5.2+incompatible/client/*.go`, and adjust `NetworkInspect`'s return type to what v28.5.2 declares.
  - `var _ Client = (*client.Client)(nil)`.
  - `NewClient() (Client, error)` using `client.NewClientWithOpts(client.WithHostFromEnv(), client.WithAPIVersionNegotiation())` (model: jumppad `pkg/clients/container/docker.go:75-82`).
  - `Ping(ctx context.Context) error`, which calls NewClient, then `Ping` with a short timeout derived from ctx, closes the client, and wraps the error as `no Docker engine reachable: %w`.
  - Package doc explains why the interface is narrow.
- `example/plugin/docker/labels.go` (new): constants `LabelCreatedBy = "created_by"`, `CreatedByValue = "xcl-example-plugin"`, `LabelXCLID = "xcl_id"`, and `labels(meta types.Meta) map[string]string`.
- `example/plugin/.mockery.yml` (new): same shape as the root `.mockery.yml` (`template: testify`, `structname: Mock{{.InterfaceName}}`, `dir: '{{.InterfaceDir}}/mocks'`, `pkgname: mocks`), package `github.com/jumppad-labs/xcl/example/plugin/docker`, interface `Client`.
- `example/plugin/docker/mocks/mock_client.go` (generated).
- `example/plugin/Makefile`: add `generate` (`go run github.com/vektra/mockery/v3@<pinned v3> `) and keep `build`/`run`/`test`/`clean`. `build` builds `./cmd/docker-plugin` into `build/docker-plugin` (the cmd is added later; adjust the target in that task if needed).
- `example/plugin/go.mod`, `go.sum`: require `github.com/docker/docker v28.5.2+incompatible`, `github.com/docker/go-connections v0.6.0`, `github.com/opencontainers/image-spec` (whatever version tidy picks), then `go mod tidy`. Keep `go 1.25.0`. If tidy raises it, STOP and ask, because the e2e plan fixes `go 1.25.0` per example.
- `example/plugin/docker/client_test.go` (new): `TestPingFailsWhenNoEngineIsReachable` sets `t.Setenv("DOCKER_HOST", "unix://"+filepath.Join(t.TempDir(), "none.sock"))` and requires an error.
- Root `go.mod` must not gain Docker. Check with `grep docker go.mod` at the root.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Build the Docker network provider

**File changes**:
- `example/plugin/docker/network.go` (new):
  - `Network` type (plan Data Structures).
  - `networkProvider struct { plugins.DefaultChanged[*Network]; client Client }`.
  - `Init` stores nothing extra and logs `provider ready` at debug.
  - `Create`: `client.NetworkCreate(ctx, n.Meta.Name, network.CreateOptions{Driver: "bridge", Attachable: true, Labels: labels(n.Meta), IPAM: <only when Subnet set: &network.IPAM{Driver: "default", Config: []network.IPAMConfig{{Subnet: n.Subnet}}}>})`, then sets `n.DockerID = resp.ID` and logs `created network` with `name`/`id` via `plugins.Logger(ctx)`. Model: jumppad `pkg/config/resources/network/provider.go:168-191`.
  - `Destroy`: `client.NetworkRemove(ctx, n.DockerID)`. Treat `errdefs.IsNotFound(err)` (or `client.IsErrNotFound`) as success. Log `destroyed network`.
  - `Read` returns `new`. `Update` returns the resource unchanged (change detection is a non-goal). `Functions` returns nil.
- `example/plugin/docker/network_test.go` (new), providers built as `&networkProvider{client: mockClient}` with `mocks.NewMockClient(t)`. Tests:
  - `TestNetworkCreateAsksDockerForALabelledBridgeNetwork`
  - `TestNetworkCreateSetsTheSubnetWhenGiven`
  - `TestNetworkCreateFillsTheDockerID`
  - `TestNetworkCreateReturnsDockerErrors`
  - `TestNetworkDestroyRemovesTheNetworkByID`
  - `TestNetworkDestroySucceedsWhenTheNetworkIsGone`
  - `TestNetworkDestroyReturnsDockerErrors`

  Use `context.Background()` with a logger. Check how `plugins.Logger(ctx)` behaves with no bound logger. If it needs one, use the public helper from `plugins` (e.g. `plugins/context.go:21`) or a no-op logger from `logger`.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Build the Docker container provider

**File changes**:
- `example/plugin/docker/container.go` (new):
  - `Container` and `NetworkAttachment` types (plan Data Structures).
  - `containerProvider struct { plugins.DefaultChanged[*Container]; client Client }`.
  - `Create`:
    1. `ImageList(ctx, image.ListOptions{Filters: filters.NewArgs(filters.Arg("reference", c.Image))})`. If the list is empty, call `ImagePull(ctx, c.Image, image.PullOptions{})`, drain the reader with `io.Copy(io.Discard, rc)`, then close it. Model: jumppad `pkg/clients/container/docker_tasks.go:477-524`.
    2. `ContainerCreate(ctx, &container.Config{Image, Cmd: c.Command, Env: KEY=VALUE sorted, Labels: labels(c.Meta)}, &container.HostConfig{}, &network.NetworkingConfig{EndpointsConfig: {first.Name: {Aliases: first.Aliases}}}, nil, c.Meta.Name)`.
    3. For each further network, `NetworkConnect(ctx, n.Name, resp.ID, &network.EndpointSettings{Aliases: n.Aliases})`.
    4. `ContainerStart`.
    5. `ContainerInspect`, then set `c.DockerID = resp.ID` and `c.IPAddress = inspect.NetworkSettings.Networks[first.Name].IPAddress`.
    6. On a failure after the create call, remove the container (`ContainerRemove` with `Force: true`) before returning the error. Model: rollback at jumppad `docker_tasks.go:375`.
  - Log `created container` with `name`, `image`, `ip_address`.
  - `Destroy`: `ContainerStop(ctx, id, container.StopOptions{})`, then `ContainerRemove(ctx, id, container.RemoveOptions{Force: true})`. Not-found counts as success. Model: jumppad `docker_tasks.go:598-620`.
  - With no `network` block, the container goes on Docker's default network and the IP address is read from `NetworkSettings.IPAddress`.
- `example/plugin/docker/container_test.go` (new), mock-backed, one behaviour each:
  - `TestContainerCreatePullsAMissingImage`
  - `TestContainerCreateSkipsThePullForAPresentImage`
  - `TestContainerCreateUsesTheBlocksNameImageAndLabels`
  - `TestContainerCreateAttachesTheFirstNetworkWithAliases`
  - `TestContainerCreateConnectsFurtherNetworks`
  - `TestContainerCreateStartsTheContainer`
  - `TestContainerCreateFillsTheDockerIDAndAddress`
  - `TestContainerCreateReturnsPullErrors`
  - `TestContainerCreateReturnsCreateErrors`
  - `TestContainerCreateRemovesTheContainerWhenStartFails`
  - `TestContainerDestroyStopsAndRemovesTheContainer`
  - `TestContainerDestroySucceedsWhenTheContainerIsGone`
  - `TestContainerDestroyReturnsDockerErrors`

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential: the create path first, then destroy.

### Task: Serve the Docker plugin and test it against real Docker

**File changes**:
- `example/plugin/docker/plugin.go` (new):
  - `type Plugin struct { plugins.PluginBase }` with `var _ plugins.Plugin = (*Plugin)(nil)`.
  - `Init(logger, state)`: calls `NewClient()` and returns its error. Logs `registering block types` with `block_types="docker.network, docker.container"`. Then calls `plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "docker", "network", &Network{}, &networkProvider{client: c})` and the same for `"container"`. Model: `example/plugin/internal/plugin.go:33-56`.
- `example/plugin/cmd/docker-plugin/main.go` (new): `plugin.Serve(&plugin.ServeConfig{HandshakeConfig: plugins.HandshakeConfig, Plugins: map[string]plugin.Plugin{"plugin": &plugins.GRPCPlugin{Impl: &docker.Plugin{}}}, GRPCServer: plugin.DefaultGRPCServer})`, modelled on `example/plugin/external/main.go:162-172`.
- `example/plugin/Makefile`: `build` builds `./cmd/docker-plugin` into `build/docker-plugin`.
- `example/plugin/docker/docker_test.go` (new), real Docker tests:
  - Each starts with `if err := Ping(context.Background()); err != nil { t.Skipf("no Docker engine reachable: %s", err) }`. Each uses unique names (`xcl-example-test-<t.Name() hash or random suffix>`) and real providers `&networkProvider{client: c}` / `&containerProvider{client: c}` with `c, _ := NewClient()`.
  - Each registers `t.Cleanup` that removes by name, ignoring not-found.
  - Tests:
    - `TestNetworkCreateMakesANetworkVisibleInDocker`: `NetworkInspect` finds it with label `created_by=xcl-example-plugin`.
    - `TestNetworkDestroyRemovesItFromDocker`: inspect then returns not-found.
    - `TestContainerCreateRunsAContainerAttachedToTheNetwork`: `ContainerInspect` shows `State.Running` and the network in `NetworkSettings.Networks`.
    - `TestContainerDestroyRemovesItFromDocker`.
  - Image `nginx:1.27-alpine`.
- Verify: `go test ./...` in `example/plugin` passes with Docker. Without Docker (`DOCKER_HOST` pointing at a missing socket), the four tests report SKIP.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential.

### Task: Build the template plugin

**File changes**:
- `example/plugin/template/template.go` (new):
  - `Template` type (plan Data Structures).
  - `provider struct { plugins.DefaultChanged[*Template] }`.
  - `Create`:
    1. `raymond.Parse(t.Source)`, then register the helpers `quote` and `trim` as in `internal/functions/functions.go:291-298`.
    2. `Exec(t.Variables)`.
    3. `os.MkdirAll(filepath.Dir(t.Destination), 0755)`.
    4. `os.WriteFile(t.Destination, out, 0644)`.
    5. Log `rendered template` with `destination`.
  - `Destroy`: `os.Remove(t.Destination)`, ignoring `errors.Is(err, fs.ErrNotExist)`.
  - `Read` returns `new`. `Update` re-renders, since a template is cheap. Model: jumppad `pkg/config/resources/template/provider.go:38-139`.
- `example/plugin/template/plugin.go` (new): `type Plugin struct { plugins.PluginBase }`. `Init` logs `registering block types` with `block_types="template"` and calls `plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "template", "", &Template{}, &provider{})`.
- `example/plugin/template/template_test.go` (new), using `t.TempDir()` (model: jumppad `provider_test.go:146-161`). Tests:
  - `TestTemplateCreateWritesTheRenderedFile`
  - `TestTemplateCreateSubstitutesVariables`
  - `TestTemplateCreateCreatesParentDirectories`
  - `TestTemplateCreateSupportsTheQuoteHelper`
  - `TestTemplateCreateFailsForAnInvalidTemplate`
  - `TestTemplateDestroyRemovesTheFile`
  - `TestTemplateDestroySucceedsWhenTheFileIsGone`
- `example/plugin/template/plugin_test.go` (new): `TestPluginRegistersTemplateWithoutASubtype` asserts `GetTypes()` holds `Type == "template"` and `SubType == ""`.
- `example/plugin/go.mod`: require `github.com/infinytum/raymond/v2 v2.0.5`, then tidy.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential.

### Task: Rewrite the plugin example application and configuration

**File changes**:
- `example/plugin/main.go`: full rewrite.
  - The package doc describes the two plugins, `make run`, and the arguments `[config dir] [docker plugin binary]`, which default to `./config` and `./build/docker-plugin`.
  - `main()`:
    1. `docker.Ping(ctx)`. On failure, print `error: %s` to stderr and exit 1.
    2. `os.MkdirTemp` for state, removed at the end.
    3. `r := registry.NewPluginRegistry()`, with `defer` stopping `r.GetPluginHosts()`.
    4. `c, err := apply(r, prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r), dir, plugin, stateDir)`.
    5. `report(os.Stdout, c)`, then `c.Destroy()`, then print `## Destroyed` with `c.EntityCount()`.
  - `apply` registers `&template.Plugin{}` and `RegisterPluginWithPath(dockerPlugin)`, builds `xcl.NewConfig(xcl.WithPluginRegistry(r), xcl.WithStatePath(stateDir), xcl.WithEventHandler(handler), xcl.WithEventData(xcl.EventDataProcessed))`, applies, and returns the config. On `xcl.ErrPluginLoad` it adds the hint "build it with `make build` in example/plugin", as in today's `main.go:160-166`.
  - `report` prints `## Networks` (`xcl.FindByType[docker.Network](c, "docker", "network")`: id, docker_id), `## Containers` (`FindByType[docker.Container](c, "docker", "container")`: id, image, ip_address), and `## Templates` (`FindByType[template.Template](c, "template")`, the one-segment form for a type with no subtype (`query.go:165-180`): id, destination, then the rendered file's contents).
  - No state key and no masking.
- `example/plugin/config/main.xcl`: full rewrite.
  - `variable "output_dir" { default = "./build/rendered" }`.
  - `docker "network" "app" { subnet = "10.42.0.0/24" }`.
  - `docker "container" "web" { image = "nginx:1.27-alpine"; network { name = docker.network.app.meta.name; aliases = ["web"] } }`. `meta.name` is addressable, as in `internal/test_fixtures/config/computed_set/nested.xcl:8`.
  - `template "welcome" { source = <<-EOF ... {{network}} ... {{address}} EOF; destination = "${variable.output_dir}/welcome.txt"; variables = { network = docker.network.app.meta.name, address = docker.container.web.ip_address } }`.
  - Comments explain each block, as today's config does.
- Delete:
  - `example/plugin/config/modules/db/db.xcl`
  - `example/plugin/internal/plugin.go`
  - `example/plugin/external/main.go`
  - `example/plugin/resources/resources.go`
  - every remaining test in `example/plugin/main_test.go` that asserts old output (after the e2e plan these are `PrintsEveryResource`, `FailsForMissingConfig`, `PrintsNoResourcesRemaining`, `PrintsPublishedTotal`, `PrintsNoSecret` and the prettylog-render halves of `Shows…`). The new tests are added in the next task.
- `example/plugin/Makefile`: header comment describes the Docker plugin binary and the in-process template plugin. `run: build` runs `go run . $(CONFIG_DIR) $(DOCKER_PLUGIN)`. `clean` removes `build/`.
- `example/plugin/go.mod`: tidy. `kr/pretty` etc. are dropped if unused.
- `.gitignore`: confirm `example/*/build/` still covers `build/rendered`.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential.

### Task: Add the wiring and smoke tests

**File changes**:
- `example/plugin/main_test.go` (rewrite):
  - `TestMain` builds `./cmd/docker-plugin` into `os.MkdirTemp` and sets package var `dockerPlugin` (model: `example/plugin/main_test.go:44-66` as it is today). `requireDocker(t)` is a small local func calling `docker.Ping`, then `t.Skipf`.
  - Each test sets `t.Setenv("HCL_VAR_output_dir", t.TempDir())`, builds `registry.NewPluginRegistry()`, and registers `t.Cleanup` to stop the hosts. It calls `apply(r, nil, "./config", dockerPlugin, t.TempDir())`; check that a nil handler is accepted, as today's tests do.
  - Tests:
    - `TestApplyCreatesTheDockerNetwork`
    - `TestApplyCreatesTheDockerContainerOnTheNetwork`
    - `TestApplyRendersTheTemplateWithTheContainerAddress`: the file contains the container's `IPAddress` and the network name.
    - `TestDestroyRemovesTheRenderedTemplate`
    - `TestDestroyRemovesTheDockerResources`: verify with `docker.NewClient()` inspect, which returns not-found.
  - Tests that do not destroy register `t.Cleanup(func(){ c.Destroy() })`.
  - Run sequentially: names are fixed by the config, so no `t.Parallel`.
- `example/plugin/smoke_test.go` (rewrite on the e2e plan's `os/exec` version): `TestPluginExampleSmokeRunsWithDefaultArguments`.
  1. `requireDocker(t)`.
  2. `go build -o build/docker-plugin ./cmd/docker-plugin`, with cmd.Dir as the package dir.
  3. `go build -o <tempdir>/plugin .`.
  4. Run the program with no args, `cmd.Dir` as the package dir and env `HCL_VAR_output_dir=<tempdir>`.
  5. Require a nil error, stdout containing `## Networks`, `## Containers`, `## Templates`, `docker.network.app`, `docker.container.web`, `template.welcome` and `## Destroyed`, and stderr without `error:`.
  - Keep a negative `TestPluginExampleSmokeFailsForMissingPlugin` as its own function. It also needs Docker (the program pings first): pass a missing plugin path and require an error.
- `e2e/examples_test.go`: no change. `TestPluginExampleTestsPass` runs this module's tests. Check that it passes with and without Docker.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential.

### Task: Update xcl's docs for the new plugin example

**File changes**:
- `README.md:137-160` ("### Plugins"): rewrite. `example/plugin` has two plugins:
  - the external Docker plugin (`docker/`, served by `cmd/docker-plugin`), providing `docker "network"` and `docker "container"`;
  - the in-process template plugin (`template/`), providing `template` with no subtype.

  Then describe the computed `ip_address` crossing from the external plugin into the template's variables. Quote the provider log example from the new source (e.g. `plugins.Logger(ctx).Info("created network", ...)` and a matching log line from a real run). Note that Docker-dependent tests skip without an engine.
- `README.md:226-234` ("Running them"): leave to the e2e plan. Only replace a plugin-specific sentence if it names the old `external` binary or `build/external`, and change nothing else.
- `README.md:1355-1357`: replace the link to `./example/plugin/config/modules/db/db.xcl` with a description of the module shown in the following snippet, so no link points at a deleted file.
- `docs/plugins.md:180-182,289,408-410`: replace the `postgres`/`redis`/`app`/`ingress`/`ExamplePlugin` mentions with the Docker plugin (`docker.network`, `docker.container`) and the template plugin (`template`, no subtype). At `:289`, name the in-process plugin's name as it will be logged (`Plugin`, from `plugins.PluginName`). Consider whether the template plugin's Go type should be named `TemplatePlugin` so its log source reads clearly. If you rename it, apply the rename back in the template task's files and record it.
- Grep the repo (excluding `.spektacular/`) for `example/plugin/internal`, `example/plugin/external`, `example/plugin/resources`, `example/plugin/config/modules` and `ExamplePlugin`, and fix any remaining reference outside `plugins/example` (a different example).

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

### Task: Rewrite the website's plugin example page

**File changes**:
- `xcl-website:src/pages/examples/plugins.mdx:1-563`: rewrite end to end.
  - Hero/intro (`:1-36`): the Docker and template plugins, external vs in-process.
  - Computed-value prose (`:38-44`): `ip_address` into the template.
  - Config snippet (`:46-123`): the new `config/main.xcl`.
  - Type snippets (`:128-144`): `docker.Container` with its computed fields and the `network` block.
  - Plugin `Init` (`:146-190`): `docker.Plugin.Init`.
  - Provider walkthrough (`:192-290`): the `Client` interface, `networkProvider.Create`/`Destroy` and `containerProvider.Create`, replacing the postgres provider and `connect`/`Reveal`.
  - External binary (`:292-328`): `cmd/docker-plugin/main.go`.
  - Wiring (`:330-422`): `main.go` `apply`/`report`.
  - Template plugin: a new section on `template.Plugin.Init` with no subtype and the provider.
  - Testing: a new short section on mock-backed provider tests and skipping without Docker.
  - Output (`:428-519`): paste a real `make run` (info and `XCL_LOG_LEVEL=debug`) captured from the final example.
  - "What to notice" (`:521-551`): rewrite and drop the module snippet.
  - CTA (`:553-563`).
  - Every fence keeps `title="example/plugin/<path>"`.
- `xcl-website:src/components/Nav.astro:13`: keep the label "Plugins and the lifecycle" unless it no longer fits. Record the decision.
- Verify with `npm run build` and `npx astro check` in `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential. It reads the final example source first and captures a real run.

### Task: Update the website pages that quote the plugin example

**File changes**:
- `xcl-website:src/pages/plugin-logging.mdx`:
  - `:31-44`: replace `postgresProvider.Create` with `networkProvider.Create` or `containerProvider.Create`.
  - `:46-55`: rewrite the password/`connect`/`Reveal()` prose around the Docker client. Keep any general statement about sensitive values only if it is still true.
  - `:69-73`: create log lines for `docker.network.app` from a real run.
  - `:78-101`: in-process vs external now means template vs Docker. Quote `template` provider `Create` and its log line.
  - `:118-137`: the plugin `Init` debug message and provider `Init` with `provider=network`/`provider=container`/`provider=template`.
  - `:169-174`: load events with the new plugin names and `block_types`.
  - `:187-201`: the `apply` `ErrPluginLoad` hint and its real error output.
  - `:219`: CTA text.
- `xcl-website:src/pages/events.mdx`:
  - `:165-172`: prose ("creating a network") and log lines from a real run.
  - `:177-183`: source names.
  - `:258-260`: the new `main.go` prettylog handler line.
  - `:262-271`: the load error output with the new plugin names.
- `xcl-website:src/pages/index.mdx:213-216,228`: prose naming a Docker plugin (external) and a template plugin (in-process).
- `xcl-website:src/pages/sensitive-values.mdx:76`: leave as is. It is generic, not quoted from the example.
- Leave `state-masking.mdx` untouched: the configuration-example spec owns its whole call to action.
- Leave `configuration-text.mdx`, `index.mdx:43-110` and the generic `main.go` blocks in `events.mdx`/`plugin-logging.mdx` unchanged.
- Verify with `npm run build` and `npx astro check`.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential, using one real captured run shared with the plugin example page task.

## Testing Strategy

- **Support plugin block types with no subtype.** Library tests:
  - in-process apply, destroy and addressing;
  - an external fixture binary proving the type and its nested block cross gRPC;
  - the log tag;
  - a separate negative test for a subtype label and for the error message.

  The whole existing suite is the backwards-compatibility guard.
- **Add the Docker client interface and its mock.** A ping-failure test, and a regenerated mock that compiles.
- **Build the Docker network provider / Build the Docker container provider.** Mock-backed unit tests: one behaviour per function, positive and negative cases separate, no Docker.
- **Serve the Docker plugin and test it against real Docker.** Four real-engine tests that skip when `docker.Ping` fails and clean up through `t.Cleanup`.
- **Build the template plugin.** `t.TempDir()` tests for rendering, the helpers, directory creation and removal, with the invalid template in its own test, plus a registration test for the empty subtype.
- **Rewrite the plugin example application and configuration.** Covered by the next task's tests and by `make run`.
- **Add the wiring and smoke tests.** Wiring tests through `apply`, a destroy test, and the smoke test with default arguments, all skipping without Docker. The e2e runner `TestPluginExampleTestsPass` still passes.
- **Update xcl's docs for the new plugin example.** No automated test. The README and guide edits are checked in the manual snippet review below.
- **Website tasks.** `npm run build` and `npx astro check`. Snippet fidelity is reviewed by hand.
- Manual checks, all **Manual — captured in the implementation test plan**:
  - the copied-out module builds against a published xcl;
  - a no-Docker run skips cleanly;
  - `make run` resources are visible in Docker, then gone;
  - no test-only seams;
  - website, README and guide snippets match the source.

## Project References

- Spec: `20261006112023-f7a185dc-docker-plugin-example` (epic `20261006071139-7b266535-examples-and-output`).
- Depends on plan `20261006071142-506b8289-e2e-suite-and-real-world-examples`. Sibling: `20261006112023-aadf3c10-configuration-example`. Independent sibling: `20261006112108-17623cda-encoder-syntax-highlighting`.
- Design documents: none.
- Knowledge (repo `xclconfig`):
  - `conventions/testing-and-mocking.md`
  - `conventions/code-style.md`
  - `conventions/dependencies.md`
  - `conventions/patterns-and-architecture.md`
  - `conventions/development-standards.md`
  - `conventions/shared-errors-package.md`
  - `conventions/never-modify-dependencies.md`
  - `conventions/project-structure.md`
  - `gotchas/plugin-types-rebuilt-with-structof.md`
  - `gotchas/meta-type-holds-two-axes.md`
- Repo roots:
  - `xclconfig` is `/home/nicj/code/github.com/jumppad-labs/xcl`.
  - `xcl-website` is `/home/nicj/code/github.com/jumppad-labs/xcl-website`.
- Reference implementation: Jumppad at `/home/nicj/code/github.com/jumppad-labs/jumppad`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The network, container and template tasks can run in parallel once the client interface exists, and the template task as soon as the library task lands. The two website tasks share one captured `make run` output.

## Migration Notes

- Readers who ran the old plugin example now need a Docker engine to run it. Its tests still pass without one, with the Docker tests skipped.
- The external plugin binary moves from `build/external` to `build/docker-plugin`, and `go run . <config> <plugin>` keeps the same argument order.
- There is no library migration: subtyped plugin types are unchanged.

## Performance Considerations

- The Docker integration, wiring and smoke tests start real containers and may pull `nginx:1.27-alpine` on first run. This adds tens of seconds to the example's tests, and through the e2e runner to xcl's test run in CI.
- The tests run sequentially because the configuration fixes the Docker object names.
