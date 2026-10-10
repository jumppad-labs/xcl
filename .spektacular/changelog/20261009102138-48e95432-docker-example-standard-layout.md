---
created_date: "2026-10-10"
document_status: final
closed_date: "2026-10-10"
---

# 20261009102138-48e95432-docker-example-standard-layout

## What was built

The plugin example's Docker plugin was rebuilt to the standard xcl plugin layout (design `plugin-layout.md`).

- **Repo xcl.** `example/plugin/plugins/docker` is now its own Go module, `github.com/jumppad-labs/xcl/example/plugin/plugins/docker`.
  - **Plugin type.** The importable `Plugin` type is in the root package `docker`, and `cmd/docker` serves it as a separate program.
  - **Block types.** These live in `entities`, which imports only xcl's `types`.
  - **Providers.** These live in `providers` and import no Docker library.
  - **Client layer.** The Docker libraries are imported only under `client/`:
    - `client/docker` is the narrow SDK interface, with `New` and `Ping`.
    - `client/containers` is a new container task layer, with its own types and a `containers.ErrNotFound` sentinel. It makes exactly the Docker calls the providers used to make, and keeps their error text.
  - **Plugin build and tests.** Each client package has a Mockery double beside it. The plugin also has its own `Makefile`, `README.md` and `.mockery.yml`, a sample (`examples/basic`), and end-to-end tests. The tests apply the sample, check that a second plan reports zero changes, and destroy it.
  - **Application.** The example application imports only `entities`. Its pre-flight engine check is a standard-library `GET /_ping` of `DOCKER_HOST` (`engine.go`), so its build has no Docker library.
  - **File order.** Every file in the plugin example lists its public surface first, and providers follow `Init, Create, Read, Changed, Update, Destroy, Functions`. The network's `Changed` takes the explicit form, with `networkReplaceSettings` (`subnet`).
  - **Wiring.** CI's example loop and the root e2e runner now build, vet and test the new module, and `.gitignore` knows its build output.
  - **Docs.** The README, `docs/plugin-developer-guide.md`, `docs/plugins.md` and `CHANGELOG.md` are updated.
- **Repo xcl-website.** The plugin example, replacement and plugin-logging pages quote the new file locations and code, and the site builds.

All suites pass against a real Docker engine:

- the provider unit tests, which keep all their names
- the new task-layer tests
- the application's scenario, unit and smoke tests, with unchanged assertions
- the plugin's e2e tests
- the root e2e runner

## Why it matters

The example used to put entity types and providers in one package. Its application pulled in the Docker providers and SDK just to read state. Now the example teaches the same shape as the plugin template, and an application that reads a plugin's state stays light.

## Deviations from the plan

- **Not-found errors.** These are marked with a multi-unwrap error that keeps Docker's message, not with a `"%w: %w"` prefix. That prefix would have changed every provider error message.
- **App wiring added early.** The application's `go.mod` got the plugin require and replace in the first task, so the application kept building while the code was being moved. Its `go mod tidy` raised a few indirect versions through MVS (otelhttp v0.70.0, httpsnoop v1.1.0).
- **File order written early.** New files were written in the standard file order from the start.
- **Name clash.** The network's replace list is named `networkReplaceSettings`, because the container already declares `replaceSettings`.
- **Sample syntax.** The sample uses the real `docker "network" "<name>"` block syntax.
- **Extra engine checks.** `pingDocker` also fails on a non-200 answer, and it has an extra tcp test.
- **Provider tests.** Tests whose point was the shape of the SDK call now assert the spec the provider passes. The SDK shape itself is pinned in the `client/containers` tests.
- **Site changes.** The site gained a "host imports only the entities" note, and an old prose slip in `replacement.mdx` was fixed.
- **Not run.** The go1.25.0 minimum-toolchain build was not run locally.
