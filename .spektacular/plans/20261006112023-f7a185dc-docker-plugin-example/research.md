---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Research: 20261006112023-f7a185dc-docker-plugin-example

## Alternatives considered and rejected

### Option A: Register `template` under `resource` (`resource "template" "x"`)

Register `template` under `resource` (`resource "template" "x"`).

**Rejected**: rejected: the spec fixes the form as `template "name" {}` with no `resource` keyword and no subtype. The registry already decides label count from the first registration of a keyword (`plugins/registry/plugin_registry.go:162-190`), so a no-subtype registration is the direct route.

### Option B: Wrap the full Docker SDK client, or Jumppad's two-layer `Docker` + `ContainerTasks` design

Wrap the full Docker SDK client, or Jumppad's two-layer `Docker` + `ContainerTasks` design.

**Rejected**: rejected: Jumppad's `pkg/clients/container/docker.go:21-72` interface has ~40 methods and a second `ContainerTasks` layer (`container_tasks.go:18`). The spec asks for "a thin interface mirroring the SDK methods the plugin uses"; a ~10-method interface keeps the example readable while keeping Jumppad's shape (real `*client.Client` satisfies it, providers hold it as a field, Mockery mock in tests).

### Option C: Jumppad-style container network attach (create with empty endpoints, `NetworkConnect` each, `NetworkDisconnect` defaults; `docker_tasks.go:207,375,393`)

Jumppad-style container network attach (create with empty endpoints, `NetworkConnect` each, `NetworkDisconnect` defaults; `docker_tasks.go:207,375,393`).

**Rejected**: rejected for the first network: attaching the first network through `NetworkingConfig.EndpointsConfig` at create is simpler and avoids the default-bridge dance. Additional networks use `NetworkConnect` as Jumppad does.

### Option D: Skipping Docker tests with build tags or `-short`

Skipping Docker tests with build tags or `-short` (Jumppad uses `testing.Short()`, `docker_tasks_execute_command_test.go:52`).

**Rejected**: rejected by the spec's technical approach: skip on engine reachability.

### Option E: A shared `requireDocker` helper copied into each test package

A shared `requireDocker` helper copied into each test package.

**Rejected**: rejected in favour of an exported `docker.Ping(ctx) error` the program itself calls before applying (so it is not a test-only seam), and the tests call to decide whether to skip. Examples cannot import `internal/testutil` (separate module).

### Option F: A `run(out, handler, r, dir, plugin, stateDir, stateKey)` function shaped for tests

A `run(out, handler, r, dir, plugin, stateDir, stateKey)` function shaped for tests (`example/plugin/main.go:117`).

**Rejected**: rejected: the spec forbids test-only seams. The program is split into functions `main` itself calls (`apply`, `report`), each parameter of which main passes.

### Option G: Keeping a module and state encryption in the plugin example

Keeping a module and state encryption in the plugin example.

**Rejected**: rejected: neither is a Docker/template concern; modules are covered by the configuration example and docs, state masking by its own docs. Keeping the example focused on plugins matches the spec's "just enough".

### Option H: Mocking with hand-written fakes

Mocking with hand-written fakes.

**Rejected**: rejected: conventions mandate Mockery.

## Chosen approach — evidence

- Empty subtype is already mostly supported: `types.TypeKey` returns `type` alone for empty sub (`types/register.go:93`); parser takes one label when `TakesSubtype` is false (`internal/parser/parser.go:800-846`); `ResourceTypeNames` handles `SubType == ""` (`plugins/direct_plugin_host.go:56`); FQRN prints `type.name` (`internal/resources/fqrn.go:293`); `createEntityFromPlugins` sets `meta.Subtype = ""` (`plugins/registry/plugin_registry.go:337`).
- Gaps: `RegisterResourceProvider` names the adapter with `subTypeName` (`plugins/plugin.go:70`), so a no-subtype provider loses its `provider=` log tag (`plugins/adapter.go:86`); error messages concatenate `type + "." + subtype` (`plugins/plugin.go:144` and siblings, `plugins/grpc_server.go:36`) leaving a trailing dot; no test registers a plugin type with an empty subtype, in-process or over gRPC.
- Typed blocks without `resource` already work for registered Go types (`example/configonly/config/*.xcl`, `example/configonly/main.go:114-130`) and references use `type.name.attr` / `type.subtype.name.attr` (`isReferenceRoot`, `internal/parser/parser.go:690`).
- Nested blocks in plugin schemas are supported (`internal/schema/functional_test.go:36`, `example/plugin/resources/resources.go` `Timeouts`) but `reflect.StructOf` rebuild only knows `types.Meta`, `types.ResourceBase`, `cty.Value` as named types (knowledge `gotchas/plugin-types-rebuilt-with-structof.md`).
- Jumppad pins `github.com/docker/docker v28.5.2+incompatible`, `github.com/docker/go-connections v0.6.0`, `github.com/infinytum/raymond/v2 v2.0.5` (jumppad `go.mod:18,19,32`); real client `client.NewClientWithOpts(client.WithHostFromEnv(), client.WithAPIVersionNegotiation())` (jumppad `pkg/clients/container/docker.go:75-82`); xcl already depends on `infinytum/raymond/v2 v2.0.5` (`go.mod:20`, used in `internal/functions/functions.go:286` with `quote`/`trim` helpers).
- Jumppad template provider: render with raymond only when variables set, `MkdirAll` parent, write; Destroy `os.RemoveAll(Destination)` (jumppad `pkg/config/resources/template/provider.go:38-139`); tests use `t.TempDir()` (`provider_test.go:146-161`).
- Jumppad network provider labels and options (`pkg/config/resources/network/provider.go:168-191`), mock tests inject `client: md` into the provider struct (`provider_test.go:25-35`).
- External plugin served with `plugin.Serve(&plugin.ServeConfig{HandshakeConfig: plugins.HandshakeConfig, Plugins: {"plugin": &plugins.GRPCPlugin{Impl: ...}}, GRPCServer: plugin.DefaultGRPCServer})` (`example/plugin/external/main.go:162-172`).
- Mockery v3 config shape (`.mockery.yml`: `template: testify`, `structname: Mock{{.InterfaceName}}`, `dir: '{{.InterfaceDir}}/mocks'`).

## Files examined

- `example/plugin/main.go:1-252` — current program; `run` takes test-shaped params; registers `ExamplePlugin` + external path; prints and destroys.
- `example/plugin/internal/plugin.go:1-204` — in-process plugin, `RegisterResourceProvider(..., "resource", "postgres", ...)`, providers embed `plugins.DefaultChanged`.
- `example/plugin/external/main.go:1-172` — external plugin serving pattern.
- `example/plugin/resources/resources.go:1-89` — types embed `types.ResourceBase` with `xcl:",remain"`, computed fields `optional,computed`.
- `example/plugin/config/main.xcl`, `config/modules/db/db.xcl` — `resource` forms; module referenced from README `:1357`.
- `example/plugin/main_test.go:44-66` — `TestMain` builds the external plugin; `smoke_test.go` uses `internal/testutil` (to be replaced with `os/exec` by the e2e plan).
- `example/plugin/Makefile` — `build`/`run`/`test`/`clean`.
- `plugins/plugin.go:36-140` — `RegisteredType`, `RegisterResourceProvider`, `PluginBase.RegisterType`, `getRegisteredType`.
- `plugins/provider.go:17-80` — `ResourceProvider[T]` method set.
- `plugins/direct_plugin_host.go:48-66` — `ResourceTypeNames` handles empty subtype.
- `plugins/registry/plugin_registry.go:89-104,142,160-190,273,337,391` — registration, `TakesSubtype`, `checkType`, entity creation.
- `plugins/grpc_server.go:36`, `plugins/grpc_plugin_host.go:135,298,302`, `plugins/grpc_resource_adapter.go:18` — subtype passed through gRPC.
- `internal/parser/parser.go:607-846` — top-level block handling and label counts.
- `internal/resources/fqrn.go:31,266-293` — address formatting.
- `plugins/grpc_calls_test.go:81-160` — gRPC wrapper test style.
- `config_plugin_loading_test.go:80-92` — root tests build `./plugins/example`, not `example/plugin` (unaffected).
- `internal/parser/test_plugin.go:394-457` — internal test plugin registering subtyped types.
- `entity_subtype_test.go:30` — Go-registered no-subtype type test (not a plugin).
- `.mockery.yml` — root mockery v3 config.
- `README.md:137-160` (Plugins section), `:226-234` (Running them, owned by e2e plan), `:1357` (link to `example/plugin/config/modules/db/db.xcl`).
- `docs/plugins.md:180-182,289,408-410` — prose naming the plugin example's types.
- `xcl-website:src/pages/examples/plugins.mdx:1-563` — whole page quotes the old example.
- `xcl-website:src/pages/plugin-logging.mdx:31-44,46-55,69-73,78-101,118-137,169-174,187-201,219` — quotes postgres/app providers and log lines.
- `xcl-website:src/pages/events.mdx:165-183,258-271` — plugin example log output and `run` line.
- `xcl-website:src/pages/index.mdx:213-216,228` — prose about the plugin example.
- `xcl-website:src/pages/state-masking.mdx:145` — CTA claims the plugin example encrypts state; the configuration-example spec owns that whole call to action, so this plan does not edit it.
- `xcl-website:src/components/Nav.astro:13` — nav label "Plugins and the lifecycle".
- `xcl-website:package.json`, `Makefile`, `.github/workflows/deploy.yml` — `npm run build`, `npx astro check`; no snippet test.

## External references

- Jumppad `pkg/clients/container/docker.go` — the thin interface model, SDK method signatures for docker v28.
- Jumppad `pkg/config/resources/network/provider.go`, `pkg/clients/container/docker_tasks.go` — create/destroy flows, labels, image pull, attach.
- Jumppad `pkg/config/resources/template/{resource,provider,provider_test}.go` — template resource fields and tests.
- Docker Engine SDK v28 (`github.com/docker/docker/client`) — `client.IsErrNotFound` / `errdefs.IsNotFound` for "gone" checks; `Ping` for reachability.
- Mockery v3 (`github.com/vektra/mockery/v3`) — config format already used at the root.

## Prior plans / specs consulted

- Plan `20261006071142-506b8289-e2e-suite-and-real-world-examples` (final) — sets each example as its own module (`go 1.25.0`, `replace` to `../..` and `../prettylog`), stdlib smoke tests, runner test `TestPluginExampleTestsPass` in `e2e/examples_test.go`, strips library tests from `example/plugin/main_test.go`, owns README "Running them" paragraph. The e2e fixtures are copies, so rewriting `example/plugin` does not affect the e2e suite.
- Sibling spec `20261006112023-aadf3c10-configuration-example` (planned in parallel) — owns configonly and its README/website prose.

## Open assumptions

- The e2e plan has landed before implementation: `example/plugin/go.mod` exists and smoke tests use `os/exec`. If not, STOP.
- Docker SDK v28.5.2 and its dependencies build with the module's `go 1.25.0` directive; if `go mod tidy` raises it, STOP and ask (it would contradict the e2e plan's per-example rule).
- Repeated nested `network` blocks on an external-plugin type round-trip through schema generation and `reflect.StructOf`. If they do not, fall back to a list attribute and record it.
- The local engine on the developer machine is Podman behind `/var/run/docker.sock`; the Docker API calls used are supported by Podman's compat API. GitHub runners have Docker.
- An external-plugin type with an empty subtype round-trips over gRPC (untested today).

## Drafting assumptions

### Chosen direction (architecture)
- **Decision**:
  - Verify and harden no-subtype plugin types in `plugins/` first.
  - Rewrite `example/plugin` with:
    - a `docker/` package: a thin `Client` interface, `NewClient`, `Ping`, the types, the providers, the plugin and Mockery mocks;
    - `cmd/docker-plugin/`, the external binary;
    - a `template/` package, the in-process plugin;
    - `main.go`, split into `apply` and `report`.
  - Real-Docker tests skip when `docker.Ping` fails.
  - Update the README, `docs/plugins.md` and the website pages.
- **Rationale**:
  - The spec's Jumppad-modelled steer at example size.
  - No test-only seams.
  - The library risk is retired before the example depends on it.
- **Rejected**:
  - Jumppad's full two-layer client.
  - Test-shaped `run` signature.
  - `-short`/build-tag skips.
  - Registering `template` under `resource`.

### No module and no state encryption in the rewritten plugin example (discovery)
- **Decision**: The rewritten example applies a Docker network, a container and a template; it drops the `modules/db` module and the state-encryption key.
- **Rationale**: The spec scopes the example to "just enough" Docker and template; modules and masking are documented elsewhere.
- **Rejected**: Keeping both — adds unrelated surface to a plugin example.

### `docker.Ping` is the Docker availability check (discovery)
- **Decision**: The docker package exports `Ping(ctx) error`; the program calls it before applying (clear error when no engine), and tests use it to skip.
- **Rationale**: Avoids a test-only seam and avoids copying a helper into two test packages (examples cannot use `internal/testutil`).
- **Rejected**: Local `requireDocker` helpers in each package's tests.

### README module link and docs prose updated by this plan (discovery)
- **Decision**: README `:1357` link to `example/plugin/config/modules/db/db.xcl`, the README Plugins section and `docs/plugins.md:180-182,289,408-410` are updated here, since the rewrite breaks them.
- **Rationale**: This drift is caused by the example; the spec's non-goal only excludes drift not caused by the examples.
- **Rejected**: Leaving them for issue #4.

### Conventions selected (architecture)
- **Decision**: Kept testing-and-mocking, code-style, dependencies, patterns-and-architecture, development-standards, shared-errors-package, never-modify-dependencies, project-structure (`/cmd`). Dropped database-and-external-services (no database), shared-test-helpers (examples are separate modules and cannot import `internal/testutil`; the library tests use it as normal), test-state-from-real-apply and assert-ordering-on-graph-parents (no state fixtures or ordering assertions planned beyond real applies).
- **Rationale**: Only conventions that drive a concrete choice in this plan.
- **Rejected**: Listing every convention.

### First network attached at create (architecture)
- **Decision**: A container's first `network` block is attached through `NetworkingConfig` at `ContainerCreate`. Further blocks use `NetworkConnect` before start.
- **Rationale**: Simpler than Jumppad's connect-then-disconnect-default flow, and still real.
- **Rejected**: Jumppad's exact flow, which carries more code for no example value.

### Docker object names, labels and computed fields (data_structures)
- **Decision**:
  - Docker objects are named after the block name.
  - They are labelled `created_by=xcl-example-plugin` and `xcl_id=<Meta.ID>`.
  - The computed fields are `docker_id` on both types and `ip_address` (on the first network) on the container.
  - Container networks are a repeated `network { name, aliases }` block.
- **Rationale**:
  - Mirrors Jumppad's labels and network block, and stays readable.
  - `docker_id` avoids clashing with `meta.id`.
- **Rejected**:
  - Prefixing names. The block name is what a reader expects to see in `docker ps`.
  - A list-of-strings `networks` attribute. Kept only as the fallback if nested blocks fail across gRPC.

### Template source is inline Handlebars text, variables are map[string]string (data_structures)
- **Decision**: `source` holds the template text (a reader can use `file()`), and `variables` is `map[string]string`.
- **Rationale**: Matches Jumppad's behaviour (it renders `Source` as text). A string map crosses the plugin schema without `cty.Value`.
- **Rejected**: A file-path source, which Jumppad documents but does not implement. `map[string]cty.Value`, which adds plugin-type complexity.

### Container image nginx:1.27-alpine (dependencies)
- **Decision**: The example container runs `nginx:1.27-alpine` with no command override.
- **Rationale**: It is small, it is a long-running server so the container stays up, and the tag is pinned.
- **Rejected**: `alpine` plus `sleep infinity`, which reads as less real. An unpinned `nginx:latest`.

### Mockery run through go run (dependencies)
- **Decision**: The Makefile's `generate` target runs `go run github.com/vektra/mockery/v3@<pinned v3 version>`. The implementer picks the newest v3 release and records it.
- **Rationale**: Mockery is not installed locally, and the root mocks use the v3 config format.
- **Rejected**: A `tools.go` dependency, which pulls mockery into the module graph.

### Website snippet match checked by hand (testing_approach)
- **Decision**: Checking that the website matches the source is a manual review, not an automated test.
- **Rationale**: The website has no test harness, and its snippets are hand-pasted MDX. Building a cross-repo snippet checker is out of scope.
- **Rejected**: A snippet-extraction test spanning both repos.

### Library no-subtype test uses its own external fixture (tasks)
- **Decision**: An external test plugin under `internal/test_fixtures/plugins/subtypeless` proves the no-subtype type and its nested block over gRPC.
- **Rationale**: The library's own tests must not depend on an example module, and `plugins/example` is a separate public example.
- **Rejected**: Testing through `example/plugin`, which is a separate module invisible to the root tests. Changing `plugins/example`, whose own documented shape would change.

### state-masking CTA left to the configuration-example spec (tasks)
- **Decision**: This plan does not edit `state-masking.mdx`. The configuration-example spec owns the whole call to action and points it only at the configuration example, which also removes the plugin claim.
- **Rationale**: One spec owning the whole call to action avoids two plans editing the same passage.
- **Rejected**: Removing only "and plugin" here, which would split ownership of one passage across two specs.

## Rehydration cues

- `spektacular spec file read 20261006112023-f7a185dc-docker-plugin-example`
- `spektacular plan file read 20261006071142-506b8289-e2e-suite-and-real-world-examples plan` (and `context`)
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"gotchas/plugin-types-rebuilt-with-structof.md"}'`
- Re-read `plugins/plugin.go:60-150`, `plugins/registry/plugin_registry.go:160-190`, `internal/parser/parser.go:800-846`.
- Jumppad checkout: `/home/nicj/code/github.com/jumppad-labs/jumppad` (`pkg/clients/container/docker.go`, `pkg/config/resources/template/`).
