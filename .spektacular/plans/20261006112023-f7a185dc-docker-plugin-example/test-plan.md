---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Test plan: 20261006112023-f7a185dc-docker-plugin-example

The spec defines no success metrics beyond its acceptance criteria. Every acceptance criterion that can be automated is covered by tests in `example/plugin` (provider unit tests, Docker integration tests, wiring and smoke tests) and by xcl's own `go test ./...`, which runs them through `e2e`'s `TestPluginExampleTestsPass`. The plan lists five checks a person makes, below.

## 1. The plugin example stands alone

**What**: copied out of the repository and pointed at a published xcl, the example builds and its tests pass, or skip where Docker is absent.

**How**:
1. `cp -r example/plugin /tmp/plugin-standalone && cp -r example/prettylog /tmp/prettylog && cd /tmp/plugin-standalone`
2. Remove both `replace` lines from `go.mod`. Then `go get github.com/jumppad-labs/xcl@<published version that includes no-subtype plugin types>` and point prettylog at a published version or at `/tmp/prettylog` with a `replace` (prettylog is not published separately).
3. `go mod tidy && go get go@1.25.0` (tidy can raise the go directive through the Docker SDK's dependencies), then `go build ./... && go vet ./... && go test ./...`.

**Expected**: build and vet succeed, and `go test ./...` reports `ok` for `.`, `./docker` and `./template`.

**Who / when**: the maintainer, once an xcl release containing the no-subtype library change is published, before announcing the example.

## 2. Tests pass without a Docker engine

**What**: Docker-dependent tests report as skipped, not failed, and the run passes.

**How**: in `example/plugin`, `DOCKER_HOST=unix:///nonexistent.sock go test -count=1 -v ./... 2>&1 | grep -E '^(--- |ok|FAIL)'`. Ideally also run once on a machine with no engine at all.

**Expected**:
- The following report `--- SKIP` with "no Docker engine reachable": `TestNetworkCreateMakesANetworkVisibleInDocker`, `TestNetworkDestroyRemovesItFromDocker`, `TestContainerCreateRunsAContainerAttachedToTheNetwork`, `TestContainerDestroyRemovesItFromDocker`, the `TestApply*` and `TestDestroy*` wiring tests (except `TestApplyFailsForAMissingDockerPlugin`), `TestReportPrintsTheResourcesOfBothPlugins`, `TestPluginExampleSmokeRunsWithDefaultArguments` and `TestPluginExampleSmokeFailsForMissingPlugin`.
- Every provider unit test (`TestNetwork*` and `TestContainer*` against the mock, `TestTemplate*`) passes.
- No `FAIL` line appears.

**Who / when**: the reviewer of this change, before merge.

## 3. Real Docker resources appear during a run and are gone afterwards

**What**: `make run` creates a network and an attached container that are visible in Docker, and destroys them.

**How**: in `example/plugin`, with Docker or Podman running:
1. In a second terminal: `watch -n 0.2 'docker network ls --filter label=created_by=xcl-example-plugin; docker ps --filter label=created_by=xcl-example-plugin'`. The run is short; nginx stops in under a second.
2. `make run`.
3. Afterwards: `docker network ls --filter label=created_by=xcl-example-plugin -q | wc -l` and `docker ps -a --filter label=created_by=xcl-example-plugin -q | wc -l`.

**Expected**:
- During the run, network `app` and container `web` (`nginx:1.27-alpine`) appear.
- Standard output shows `## Networks`, `## Containers`, `## Templates` with `Welcome to the app network.` and `http://10.42.0.x/`, then `## Destroyed` and `0 resources remaining`.
- Both counts afterwards are `0`, and `build/rendered/welcome.txt` no longer exists.
- Run it on real Docker as well as Podman.

**Who / when**: the reviewer, before merge.

## 4. No test-only seams in the program

**What**: `example/plugin/main.go` exposes no function, parameter or return value that only its tests use.

**How**: read `example/plugin/main.go`. For each of `apply`, `report`, `destroy` and `exit`, confirm that `main` calls it, and that `main` supplies every parameter with a value it uses itself: `prettylog.Handler(...)` as `handler`, a real temporary `stateDir`, `os.Stdout` as `out`. Confirm that the return values (`*xcl.Config` and `error`) are used by `main`.

**Expected**: every function, parameter and return value is used by `main`. The tests pass `nil` as the handler, which `main` never does, but the parameter is still one `main` passes.

**Who / when**: the reviewer, before merge.

## 5. Website, README and guide snippets match the source

**What**: every snippet taken from the plugin example matches the final source, and the shown output is from a real run.

**How**:
1. In `xcl-website`, check `src/pages/examples/plugins.mdx`, `plugin-logging.mdx`, `events.mdx` and `index.mdx`. For each fence titled `example/plugin/<path>`, compare it with that file in xcl, for example with a script that checks the fence body is a substring of the file.
2. Run `make run` and `XCL_LOG_LEVEL=debug make run` in `example/plugin`, and compare the `text` output blocks line by line. Timestamps, IDs, durations and the pid differ per run; the shape and fields must match.
3. In xcl, read `README.md` "### Plugins" and "### Running them", the `docs/plugins.md` passages naming `example/plugin`, and the `docs/README.md` layout row. Confirm every quoted line (`plugins.Logger(ctx).Info("created network", ...)`, `template "welcome"`, make targets `build`/`generate`/`clean`) exists in `example/plugin`.
4. Build the site with `npm run build && npx astro check` in `xcl-website`.

**Expected**: no snippet differs from its source, every output line has the shape of a real run, and the site builds with 0 errors. Known, out of scope: the `example/prettylog/prettylog.go` `Handler` snippet on `events.mdx` is stale (owned by the prettylog spec).

**Who / when**: the reviewer, before merge. Repeat for any later change to `example/plugin`.
