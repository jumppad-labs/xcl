---
created_date: "2026-10-10"
document_status: final
closed_date: "2026-10-10"
---

# Test plan: 20261009102138-48e95432-docker-example-standard-layout

Manual procedures for the success metrics and reviews the plan's Testing Approach marks as manual. Everything else is covered by the automated suites (`make test` in `example/plugin` and `example/plugin/plugins/docker`, and `TestPluginExampleTestsPass` / `TestDockerPluginExampleTestsPass` in the root `e2e` suite).

Paths are relative to the xcl repository root unless prefixed `xcl-website:`.

## Success metric: One shape everywhere

- **What to measure**: the Docker plugin example and the documentation agree with the layout design `plugin-layout.md` (design source `design`); neither has a part in a place the design does not name. Threshold: zero mismatches.
- **How**:
  1. Open the design (`spektacular design read --data '{"source":"design","path":"plugin-layout.md"}'`) beside `find example/plugin/plugins/docker -type f -not -path '*/build/*' | sort`.
  2. Walk the design's directory tree: `plugin.go`, `cmd/docker/main.go`, `entities/{network,container}.go`, `providers/{network,container,attachments,labels}.go` and their `_test.go` files, `client/docker/docker.go` + `client/docker/mocks/`, `client/containers/containers.go` + `client/containers/mocks/`, `examples/basic/main.xcl`, `e2e/`, `.mockery.yml`, `Makefile`, `README.md`, `go.mod`. Confirm each exists and that no other file or directory exists.
  3. Check "what each part holds": `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` from `example/plugin/plugins/docker` — `entities` imports only `github.com/jumppad-labs/xcl/types`; no `providers` package imports `github.com/docker/...`; only `client/docker` and `client/containers` import the Docker SDK.
  4. For every Docker-plugin path quoted in `README.md`, `docs/plugin-developer-guide.md`, `docs/plugins.md`, `example/plugin/plugins/docker/README.md`, `xcl-website:src/pages/examples/plugins.mdx`, `xcl-website:src/pages/replacement.mdx` and `xcl-website:src/pages/plugin-logging.mdx`, confirm the path exists (e.g. `grep -o 'example/plugin/plugins/docker[^)" ]*' <file>` then `test -e`).
- **Expected result**: every design location present, no extra locations, import rules hold, and zero quoted paths missing.
- **Who / when**: the reviewer of this change, before merging; again whenever the layout design or the plugin template changes.

## Success metric: Example applications stay light

- **What to measure**: the plugin example application's build includes no Docker library. Threshold: zero `github.com/docker/` packages.
- **How**: `cd example/plugin && go list -deps . | grep -c 'github.com/docker/'` (the application only, not its tests). Also confirm `grep -n 'plugins/docker' *.go | grep -v _test.go` lists only `.../plugins/docker/entities` imports.
- **Expected result**: the count is `0`, and the application's non-test files import only `entities` from the plugin.
- **Who / when**: the reviewer of this change, before merging, and whenever the application's imports change.

## Manual review: file order

- **What to look at**: every non-generated `.go` file under `example/plugin/` (the Docker plugin, `plugins/template`, and the application's `main.go`, `status.go`, `inspect.go`, `plan.go`, `engine.go`).
- **What to look for**: exported types with their exported vars and consts first; in provider files (`providers/network.go`, `providers/container.go`, `plugins/template/template.go`) the provider type, its interface assertion and constructor, then `Init, Create, Read, Changed, Update, Destroy, Functions` in that order; then unexported helpers; unexported vars and consts last (e.g. `networkReplaceSettings`, `replaceSettings`, `networksSetting`/`removed`, `initScriptPath`, `pingTimeout`, `defaultMode`, `defaultStateDir`/`dockerPluginName`/`usage`, `shortIDLength`). A quick listing: `grep -n '^func\|^type\|^var\|^const' <file>`.
- **Passing**: every file follows the order; no exported method appears after an unexported helper.
- **Who / when**: code reviewer, before merging.

## Manual review: documentation site

- **What to look at**: `xcl-website` — run `npm ci && npm run build` (and `npm run dev` to browse) and open the plugin example page, the replacement page and the plugin-logging page.
- **What to look for**: the build completes without errors; every code block title naming a Docker plugin file names one that exists in the rebuilt example (`entities/…`, `providers/…`, `client/docker/docker.go`, `client/containers/containers.go`, `cmd/docker/main.go`), and the quoted code matches the source (task calls, `entities` types, explicit network `Changed`, `pingDocker`).
- **Passing**: the build succeeds and no stale `resources/`, `client/client.go` or `plugins/docker/main.go` reference remains (`grep -rn 'docker/resources\|client/client.go\|client\.Ping' src` is empty).
- **Who / when**: docs reviewer, before the site is published.

## Manual review: example behaviour with a Docker engine

- **What to look at**: with a running Docker engine, from `example/plugin`: `make run`, `make swap`, `make replace`, `make rebuild-init`, `make remove-network`; then from `example/plugin/plugins/docker`: `make build`, `make test`, `make generate`, `make clean`.
- **What to look for**: each example target applies, plans, prints status and destroys exactly as before (subnet change shows `Diff: 0 to create, 2 to update, 1 to replace, 0 to delete, 1 unchanged.`; swap moves the container in place keeping its ID; rebuild-init replaces the template and container; remove-network leaves the container running with no network). In the plugin directory, `make build` produces `build/docker-plugin`, `make test` passes (e2e included), `make generate` leaves `git status` unchanged, and `make clean` removes `build/`. Afterwards `docker ps -a --filter label=created_by=xcl-example-plugin` and `docker network ls --filter label=created_by=xcl-example-plugin` are empty.
- **Passing**: all targets succeed with the outputs above and nothing is left behind in Docker.
- **Who / when**: the reviewer of this change, before merging.
