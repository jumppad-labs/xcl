---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Research: 20261009102138-48e95432-docker-example-standard-layout

## Alternatives considered and rejected

### Option A: Keep the Docker plugin inside the `example/plugin` Go module

No `go.mod` of its own. The app binary would already skip the Docker libraries if it imported only `entities`.

**Rejected**: The design's tree names `go.mod`, `Makefile`, `README.md`, `.mockery.yml`, `examples/` and `e2e/` inside `<plugin>/`, and says "the plugin type lives in the module's root package" (`design/plugin-layout.md`, Directory tree). Today the plugin is `package main` (`example/plugin/plugins/docker/plugin.go:1`). This option fails the acceptance criterion "every location the standard layout names".

### Option B: Providers keep calling Docker SDK types through today's `client.Docker`

Only move files.

**Rejected**: Providers import `github.com/docker/docker/api/types/*` and `dockerclient.IsErrNotFound` (`plugins/docker/resources/network.go:8-9`, `container.go:11-15`). The design makes `client/` "the only place backend libraries are imported" and names "container tasks over a Docker interface" as its model.

### Option C: One client package that is both SDK wrapper and task layer

**Rejected**: Testing its real implementation against a double of the SDK needs an inner interface anyway, which is the two-package shape hidden in one package. Two packages, `client/docker` and `client/containers`, match the design's own example.

### Option D: The app keeps its engine check by importing the plugin's `client.Ping`

**Rejected**: `main.go:44,200,218,262` would keep the Docker SDK in the app binary, against the spec's Technical Approach ("import only the plugin's entity types; the Docker client ... stay[s] inside the plugin").

### Option E: Drop the app's pre-apply engine check

**Rejected**: `smoke_test.go:94-112` asserts `apply` fails with "no Docker engine reachable" before any plugin loads. Dropping the check changes behaviour the spec keeps.

### Option F: Ping Docker from the plugin's `Init`

**Rejected**: The design says `plugin.go` builds clients "without contacting the backend". `status`/`inspect` load plugins but must work with no engine (`main.go:226-230`).

## Chosen approach — evidence

- `design/plugin-layout.md` — binding layout: `plugin.go` in module root, `cmd/<plugin>/main.go`, `entities/`, `providers/`, `client/<backend>/<backend>.go` + `mocks/`, `examples/<name>/main.xcl`, `e2e/`, `.mockery.yml`, `Makefile`, `README.md`, `go.mod`; file order (exported first; provider methods `Init, Create, Read, Changed, Update, Destroy, Functions`; then unexported helpers, then unexported vars/consts); `Changed` explicit with `change.Within`, replace settings listed once; `Update` from what it is told.
- `example/plugin/plugins/docker/resources/network.go:19-30`, `container.go:24-61` — block types `Network`, `Container`, `NetworkAttachment` mixed with providers; they import only `types` themselves, so they can move to `entities/` with no new dependency.
- `example/plugin/plugins/docker/resources/labels.go:13-17` — exported `LabelCreatedBy`, `CreatedByValue`, `LabelXCLID` used by app tests (`main_test.go:159`); they are provider behaviour, so they stay in `providers` and app tests import `providers` (test-only) or the constants are referenced from there.
- `example/plugin/status.go:13`, `main_test.go:21`, `scenarios_test.go:15` — the only consumers of `resources`; switch to `entities`.
- `example/plugin/main.go:44` — the app imports `plugins/docker/client` only for `Ping`; replaced by a stdlib ping in the app.
- `example/plugin/plugins/docker/client/client.go:29-49` — narrow SDK interface already exists; moves to `client/docker/docker.go` unchanged.
- `network.go` method order is `Init, Create, Destroy, Read, Changed, Update, Functions`; `container.go` interleaves helpers (`initScriptMount`, `pullImage`, `startContainer`, `firstAddress`, `environment`) between exported methods — both violate the design's file order.
- `network.go:126-132` — `Changed` compares `old.Subnet != new.Subnet`, then `DefaultChanged`; the design asks for `change.Within(...)` against a single replace list, then explicit update when any setting changed (`plugins/changed.go:33-49` shows `DefaultChanged` returns `Update` when values differ, so explicit update on `len(changes) > 0` keeps behaviour).
- `container.go:268-273` — `replaceSettings` already the single list.
- `e2e/examples_test.go:29-55` — the root e2e suite runs each example module's tests by directory; a nested module needs its own entry.
- `.github/workflows/go.yml` (build-minimum-go job) — loops over `example/configonly example/plugin example/prettylog`; nested module needs adding.
- `smoke_test.go:16-30`, `main_test.go:29-50` — build the plugin with `go build ./plugins/docker` from the app module; must build from the nested module (`cmd.Dir`) at `./cmd/docker`.
- `config_diff.go:34`, `diff/diff.go:67` — `Config.Diff` returns `*diff.Diff` with `Changed() int`, usable by the plugin's `e2e/` "plan reports no changes" check.

## Files examined

- xcl:example/plugin/plugins/docker/plugin.go:1-56 — `package main`, `Plugin` registers network and container with `client.New()`.
- xcl:example/plugin/plugins/docker/main.go:1-17 — `main` serves the plugin.
- xcl:example/plugin/plugins/docker/client/client.go:1-85 — `Docker` SDK interface, `New`, `Ping`.
- xcl:example/plugin/plugins/docker/client/client_test.go — Ping/New tests.
- xcl:example/plugin/plugins/docker/client/mocks/mock_docker.go — Mockery output.
- xcl:example/plugin/plugins/docker/resources/network.go:1-151 — Network type + provider; methods out of lifecycle order.
- xcl:example/plugin/plugins/docker/resources/container.go:1-436 — Container + NetworkAttachment + provider; helpers interleaved.
- xcl:example/plugin/plugins/docker/resources/attachments.go:1-148 — `previousAttachments`, `putAt`, `dropRemoved`, `findAttachment`, `isNetworkAddress`, `networkName`; unexported var first (order issue).
- xcl:example/plugin/plugins/docker/resources/labels.go — labels constants and helper.
- xcl:example/plugin/plugins/docker/resources/*_test.go — unit tests on mocks (network 410 lines, container 938) and real-engine tests (`docker_test.go`, skip without engine).
- xcl:example/plugin/main.go:1-294 — app; imports plugin client for Ping.
- xcl:example/plugin/status.go:1-289 — reads `resources.Network`/`resources.Container`.
- xcl:example/plugin/inspect.go, plan.go — no plugin imports.
- xcl:example/plugin/main_test.go, scenarios_test.go, smoke_test.go, status_test.go, plan_test.go — app tests; use Docker SDK to inspect real objects.
- xcl:example/plugin/plugins/template/plugin.go, template.go — in-process template plugin; provider methods out of lifecycle order (`Changed` before `Init`).
- xcl:example/plugin/.mockery.yml, Makefile, go.mod — build, generate, module wiring.
- xcl:.github/workflows/go.yml — example build loop.
- xcl:.gitignore — `example/*/build/`, stray binaries.
- xcl:e2e/examples_test.go — runs example modules' tests.
- xcl:README.md:227-375 — links into `plugins/docker`, `plugins/docker/resources`, `plugins/docker/client`.
- xcl:docs/plugin-developer-guide.md:414,469,520,599 — links to `resources/container.go`, `network.go`, `attachments.go`.
- xcl:docs/plugins.md:289,580 — links to `example/plugin` (directory only).
- xcl:plugins/changed.go:33-49 — `DefaultChanged` semantics.
- xcl-website:src/pages/examples/plugins.mdx — titles and prose quoting `plugins/docker/resources/*.go`, `plugins/docker/client/client.go`, `plugins/docker/plugin.go`, `plugins/docker/main.go`, mocks location, host imports.
- xcl-website:src/pages/replacement.mdx:241,266,324 — quotes `resources/network.go`, `resources/container.go`.
- xcl-website:src/pages/plugin-logging.mdx:31 — quotes `resources/network.go`.
- xcl-website:src/pages/events.mdx:266 — quotes `example/plugin/main.go` (unchanged path).

## External references

- Go modules reference, nested modules and `replace` directives — a directory with its own `go.mod` is excluded from the parent module; `go -C <dir>` runs the go command in another module.
- Docker Engine API `GET /_ping` — the endpoint the SDK `Ping` uses; answering it over the `DOCKER_HOST` socket is what the app's stdlib check reproduces.
- Mockery v3 configuration (`packages:` keyed by import path) — `.mockery.yml` must name the moved interface packages.

## Prior plans / specs consulted

- Spec `20261009092551-82db0140-plugin-template` (sibling in the epic) — same layout design; it owns writing the standard layout into the guides. This plan only updates docs to the example's new file locations.
- Epic `20261009092551-82db0140-plugin-template` — this spec has no dependencies; the template spec depends on the registry spec only.
- Knowledge `gotchas/example-modules-cannot-import-internal.md` — each example is its own module with `replace ... => ../..`; root `go test ./...` stops at module boundaries; the e2e suite runs example tests explicitly.

## Open assumptions

- Interpreting "changes only layout and ordering" as "no observable behaviour change": moving Docker SDK calls behind a `client/containers` task layer with its own types counts as layout. If the user reads it more strictly, the client restructure must STOP for a decision.
- The app's stdlib engine check (unix/tcp `DOCKER_HOST`, default `/var/run/docker.sock`) reproduces the SDK `Ping` closely enough; unsupported schemes skip the pre-check.
- "Scenario tests pass unchanged" allows their import paths (`resources` → `entities`) and the plugin build command in `TestMain` to change; assertions and test bodies do not.
- App tests may keep importing the Docker SDK and the plugin's `client/docker` package to inspect real objects; the success metric is about the application build, not its tests.
- Only the Docker plugin is restructured into the layout; the in-process template plugin keeps its package but gets file-order fixes.

## Drafting assumptions

### Chosen direction: in-place rebuild as a nested module with a container task layer (architecture)
- **Decision**: rebuild `example/plugin/plugins/docker` in place as its own Go module in the design's layout; add `client/containers` as the only SDK-facing layer the providers use; the app imports only `entities` and checks the engine with a stdlib ping; file order applied to every source file in `example/plugin`; network `Changed` made explicit with an equivalent outcome.
- **Key design decisions**: the SDK translation moves layer with identical calls and messages (behaviour kept); provider unit tests double `containers.Tasks`, SDK-level assertions move to `client/containers` tests; the "no Docker libraries in the app build" check is manual, not a test.
- **Rationale**: the only direction that satisfies every location the design names while keeping behaviour and the spec's Technical Approach.
- **Rejected**: see research.md § Alternatives considered and rejected.

### Docker plugin becomes its own Go module (discovery)
- **Decision**: `example/plugin/plugins/docker` gets its own `go.mod` (module `github.com/jumppad-labs/xcl/example/plugin/plugins/docker`), with the plugin type in the root package `docker`; the app requires it through a `replace => ./plugins/docker`.
- **Rationale**: the design's tree puts `go.mod`, `Makefile`, `README.md`, `.mockery.yml` in `<plugin>/` and the plugin type in "the module's root package"; the acceptance criterion requires every named location.
- **Rejected**: keeping one module for app and plugin (fails the layout criterion); moving the plugin out of `example/plugin` (needless churn of every doc path).

### Two client packages: `client/docker` and `client/containers` (discovery)
- **Decision**: `client/docker/docker.go` keeps today's narrow SDK interface `Docker` with `New` and `Ping`; a new `client/containers/containers.go` is the task layer (`Tasks` interface, own types, `ErrNotFound` sentinel) built on it. Providers depend only on `containers.Tasks`.
- **Rationale**: the design makes `client/` the only place backend libraries are imported and names "container tasks over a Docker interface" as its example. Read "changes only layout and ordering" as "no observable behaviour change": the SDK calls move layer, they do not change.
- **Rejected**: providers keep importing SDK types (breaks the design); one combined package (needs a hidden inner interface to test).

### The app checks for an engine without the Docker SDK (discovery)
- **Decision**: the app keeps its pre-apply/plan/destroy engine check, reimplemented with the standard library (`GET /_ping` over `DOCKER_HOST` unix or tcp, default `unix:///var/run/docker.sock`, 5 s timeout, same "no Docker engine reachable" message). A scheme it cannot dial (ssh, npipe) skips the check.
- **Rationale**: Technical Approach says the app imports only the entity types; the smoke test pins the message; behaviour must stay.
- **Rejected**: importing the plugin's client (pulls the SDK in); dropping the check (behaviour change); pinging in plugin Init (design forbids contacting the backend there, and status must work with no engine).

### Scenario tests keep their bodies, not their import lines (discovery)
- **Decision**: "pass unchanged" means the scenario and unit test assertions are untouched; import paths (`resources` → `entities`) and the plugin build command in `TestMain` change.
- **Rationale**: the package the tests import is the thing being moved.
- **Rejected**: none viable.

### Template plugin gets file order only (discovery)
- **Decision**: the in-process template plugin (`example/plugin/plugins/template`) stays in its package; its provider methods and helpers are reordered to the design's file order. App source files are checked for order too.
- **Rationale**: the acceptance criterion says every source file "in the plugin example"; the layout rebuild is scoped to the Docker plugin, and non-goals keep other examples' shape.
- **Rejected**: restructuring the template plugin into entities/providers (beyond "the Docker plugin example is rebuilt").

### App tests may still import the Docker SDK (discovery)
- **Decision**: `main_test.go` and `scenarios_test.go` keep using the Docker SDK and the plugin's `client/docker` to inspect real objects, so the app's `go.mod` still lists `github.com/docker/docker` as a test dependency.
- **Rationale**: the acceptance criterion and success metric concern the application's build; test-only code is not compiled into it, and changing the scenario tests is ruled out.
- **Rejected**: rewriting the app tests to avoid the SDK.

### Conventions selected (architecture)
- **Decision**: apply testing & mocking, no repo-inspecting tests, code style, dependencies, never-modify-dependencies, and the example-module gotcha; drop the shared-errors convention (it governs errors shared between xcl's own packages, not a plugin-private sentinel), database, logging-only development standards, project-structure, shared-test-helpers (`internal/testutil` is unreachable from example modules), graph-ordering and test-state conventions.
- **Rationale**: the work is a Go module restructure with test moves and docs edits; the dropped ones govern surfaces it does not touch.
- **Rejected**: listing every convention.

### Plugin-private sentinel stays in the plugin's client package (architecture)
- **Decision**: `containers.ErrNotFound` lives in `client/containers`, not `xcl/errors`.
- **Rationale**: the shared-errors entry governs errors shared between xcl's own packages; this sentinel belongs to an example plugin's backend layer and is matched only by that plugin's providers.
- **Rejected**: adding an example-specific sentinel to the public `xcl/errors` package.

### Sample configuration name and Docker object names (architecture)
- **Decision**: one sample, `examples/basic/main.xcl`, with a network and a container whose block names are unique to it (e.g. `xcl_plugin_basic`), so the plugin's e2e can run in parallel with the app's tests without Docker name clashes.
- **Rationale**: the root e2e runner runs example modules' tests in parallel and the app's configs use `app`/`web`.
- **Rejected**: reusing the app's `config/` (it needs the template plugin).

### Task layer shape keeps provider messages (data_structures)
- **Decision**: `containers.Tasks` returns raw Docker errors (wrapped with `ErrNotFound` when Docker says not found); providers keep every current error message and log line; the provider computes the init script's absolute path (stdlib) so its "unable to mount the init script" error stays in the provider; the first-network address is chosen by the provider from `ContainerAddresses`.
- **Rationale**: keeps behaviour, including error text, identical while removing SDK imports from providers.
- **Rejected**: moving messages into the task layer (changes wrapping text); a task per provider method (would hide decisions in the client, against the design's "decision lives in `Changed`").

### Unit tests keep their assertions, not necessarily their inputs (tasks)
- **Decision**: "unit tests pass unchanged" is met when each moved unit test keeps its name and asserted outcome. Provider tests switch to the task-layer double. The network subnet-replace test passes the `subnet` property change xcl really reports, because the design's `change.Within` form decides from changes rather than by comparing old and new.
- **Rationale**: the spec's own constraint expects tests the rebuild "moves or changes", and the design binds the explicit `Changed` form; real applies always report the subnet change, so the behaviour is the same.
- **Rejected**: keeping `old.Subnet != new.Subnet` (departs from the design's matching rule); keeping provider tests on the SDK double (providers no longer hold the SDK interface).

### Scope of the docs update (tasks)
- **Decision**: update the README, `docs/plugin-developer-guide.md` and `docs/plugins.md` links, plus the site's plugin example page and the replacement, plugin-logging and events pages wherever they quote Docker plugin files. Do not add a "standard layout" write-up: the plugin template spec owns that.
- **Rationale**: the success metric "the example and the docs never disagree" covers every page that quotes the plugin, not just the example page; the template spec's requirement already covers describing the layout.
- **Rejected**: updating only `examples/plugins.mdx` (would leave stale paths on three pages); writing the layout guide here (duplicates the sibling spec).

### Old packages removed in a later task than the moves (tasks)
- **Decision**: the moves happen first, across three tasks, and the plugin-type task deletes `resources/`, the old `client/` and the old `main.go`. Intermediate states may need import repointing to compile.
- **Rationale**: keeps each task reviewable; the milestone, not each task, is the deliverable unit.
- **Rejected**: one large "move everything" task.

### Read keeps copying saved IDs (out_of_scope)
- **Decision**: the providers' `Read` stays as it is (copies `DockerID`/`IPAddress` from the saved resource) rather than querying Docker.
- **Rationale**: the design says `Read` "reports the real resource". The spec, though, says the rebuild keeps behaviour. Today's `Read` already fills every computed value a caller needs, so it does not contradict the design's intent that `Read` fills computed values.
- **Rejected**: making `Read` inspect Docker (a behaviour change the spec rules out).

## Rehydration cues

- `spektacular spec file read 20261009102138-48e95432-docker-example-standard-layout`
- `spektacular design read --data '{"source":"design","path":"plugin-layout.md"}'`
- `spektacular knowledge always-applied --tier repo --filter xcl --filter xcl-website`
- Re-read `example/plugin/plugins/docker/` (all files), `example/plugin/main.go`, `status.go`, `main_test.go:1-140`, `smoke_test.go:1-60`, `e2e/examples_test.go`, `.github/workflows/go.yml`.
- `grep -rn "plugins/docker" README.md docs ../xcl-website/src/pages` for every doc reference.
