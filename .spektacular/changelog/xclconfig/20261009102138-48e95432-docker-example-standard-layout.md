---
created_date: "2026-10-10"
document_status: draft
project: xclconfig
spec: 20261009102138-48e95432-docker-example-standard-layout
plan: 20261009102138-48e95432-docker-example-standard-layout
---

# Docker plugin example rebuilt to the standard plugin layout

The plugin example's Docker plugin now follows the standard xcl plugin layout, the same shape as the plugin template. It is a module of its own. Its parts sit where the layout puts them: block types, providers, a client layer over Docker, an entry point, a sample and end-to-end tests. The example application reads what it applied through the plugin's block types alone, so building it no longer pulls in the Docker libraries. The example behaves exactly as before.

> Derived from project xcl (spektacular), spec/plan 20261009102138-48e95432-docker-example-standard-layout. See the project-level record for the full feature.

## What changed in this repo

- **New module.** `example/plugin/plugins/docker` is its own module.
  - **Plugin type.** `plugin.go` is in package `docker`, and `cmd/docker/main.go` serves it.
  - **Entities.** `entities/network.go` and `entities/container.go` hold the block types.
  - **Providers.** `providers/{network,container,attachments,labels}.go` hold the providers and their unit and real-engine tests.
  - **Client layer.** `client/docker` is the SDK interface, with its mock. `client/containers` is the task layer, with its tests and mock.
  - **Sample and e2e.** `examples/basic/main.xcl` is the sample, and `e2e/e2e_test.go` holds the end-to-end tests.
  - **Module files.** `.mockery.yml`, `Makefile`, `README.md` and `go.mod`.
- **Removed.** The old `resources/`, `client/client.go`, `client/mocks/` and `main.go` are gone.
- **Application.** `example/plugin` requires the plugin module through a local `replace`.
  - `status.go` imports only `entities`.
  - The new `engine.go` (`pingDocker`) replaces the plugin client's `Ping`.
  - The tests build the plugin from `plugins/docker` at `./cmd/docker`.
  - The `Makefile` delegates `test` and `generate` to the plugin.
  - `.mockery.yml` is removed.
- **File order.** Every file in the plugin example lists its public surface first, in lifecycle order. This includes the template plugin and the application's files. The network's `Changed` is explicit.
- **CI and e2e.** `.github/workflows/go.yml`, `e2e/examples_test.go` (`TestDockerPluginExampleTestsPass`) and `.gitignore` now cover the new module.
- **Docs.** `README.md`, `docs/plugin-developer-guide.md` and `docs/plugins.md` are updated, and there is a new top `CHANGELOG.md` entry.

**Breaking:**

- The plugin's import path and layout changed.
- `example/plugin/plugins/docker/resources`, `.../client` and `.../main.go` are removed.
- `go build ./plugins/docker` from `example/plugin` no longer works. Use `make build`.

## Why

The example is what plugin authors copy, so it has to teach the standard layout. An application that only reads a plugin's state should not compile the plugin's backend.
