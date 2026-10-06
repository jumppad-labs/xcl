---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Docker plugin example

## What was built

xcl's plugin example (`example/plugin`) has been rewritten as a real plugin project modelled on Jumppad. It has two plugins:

- **An external Docker plugin.** The `docker` package is served by its own `cmd/docker-plugin` binary over gRPC. It provides `docker "network"` and `docker "container"` and creates real Docker networks and containers. Its providers hold a narrow `Client` interface over the Docker SDK v28.5.2. The real SDK client satisfies the interface, and a Mockery-generated mock stands in for it in unit tests.
- **An in-process template plugin.** The `template` package provides `template`, a block type with no subtype written `template "welcome" {}`. It renders Handlebars to a file and removes the file on destroy.

The program checks Docker is reachable, applies a configuration with a network, a container on it and a template, prints them, then destroys everything. The template reads the container's computed IP address, so a value crosses from the external plugin to the in-process one. The program's steps (`apply`, `report`, `destroy`) are plain functions `main` calls; nothing exists only for tests.

The example ships with the tests a plugin author would write:
- mock-backed provider unit tests that need no Docker;
- integration tests against a real engine;
- wiring tests through the program's own `apply`;
- smoke tests that build and run the real binary.

Every test that needs Docker skips when `docker.Ping` finds no engine.

The xcl library now supports and tests plugin block types with no subtype, both in-process and across the external-plugin boundary, including a repeated nested block. Such a provider's Init logs are tagged `provider=<type>`, and "no registered type" errors no longer end in a dangling dot.

The README's plugins passage, `docs/plugins.md`, `docs/README.md` and the documentation website's plugin example, plugin-logging, events and home pages were updated to match. Every snippet was re-copied from the final source and every output block from a real run.

## Why it matters

Plugin developers get an example they can run for real and copy as their own project. It shows the narrow-interface-plus-mock pattern for testing a provider that talks to an external system, how to keep tests green on machines without Docker, and how a type with no subtype is registered.

## Deviations from the plan

- **Example module dependencies.** The example's indirect dependencies are not all at Jumppad's versions. `go mod tidy` raises the go directive through the Docker SDK's unconstrained test dependencies, so it is followed by `go get go@1.25.0`, which keeps the module at `go 1.25.0` (otel v1.46.0, grpc v1.83.2, testify v1.12.1).
- **Template plugin name.** The template plugin's Go type is `TemplatePlugin`, so its log source reads clearly.
- **Template helpers.** The `quote` and `trim` helpers return `raymond.SafeString`, so Handlebars does not HTML-escape them.
- **Provider `Read`.** Providers' `Read` methods carry computed fields over from saved state.
- **Program flow.** `apply` returns the Config even on failure, so `main` can destroy anything created part way. `main` destroys only when the Config holds entities.
- **Extra tests.** Added tests for connect-before-start ordering, the report output, a missing plugin and running with no engine.
- **Extra doc fix.** Also fixed the `docs/README.md` layout row describing the examples.

Known and left out of scope: `docs/state.md` links to a nonexistent `state/state.go`, the website's `prettylog.go` `Handler` snippet on the events page is stale (owned by the prettylog spec), and xcl's own `template` function likely HTML-escapes its `quote` helper output.
