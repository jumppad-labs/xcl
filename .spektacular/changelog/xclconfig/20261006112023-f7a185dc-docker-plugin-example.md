---
created_date: "2026-10-06"
document_status: draft
project: xclconfig
spec: 20261006112023-f7a185dc-docker-plugin-example
plan: 20261006112023-f7a185dc-docker-plugin-example
---

# Docker plugin example

The plugin example is now a real plugin project. An external Docker plugin creates and removes real Docker networks and containers, and an in-process template plugin renders a file from the container's address. Each plugin comes with the tests a plugin author would write, and those tests pass on machines without Docker by skipping the ones that need an engine. Plugins can now provide block types with no subtype, such as `template "welcome" {}`, and such providers' logs and errors read cleanly.

> Derived from project xcl (xclconfig), spec/plan 20261006112023-f7a185dc-docker-plugin-example. See the project-level record for the full feature.

## What changed in this repo

- **Library.**
  - `plugins.RegisterResourceProvider` names a provider registered without a subtype after its type, so its Init logs carry `provider=<type>`.
  - "no registered type found" errors, in-process and from the gRPC server, use `types.TypeKey` and no longer end in a dot.
  - New tests cover a no-subtype type in-process and through an external fixture plugin (`internal/test_fixtures/plugins/subtypeless`), including a repeated nested block, addressing and rejection of a subtype label.
- **Example (`example/plugin`).** Rewritten:
  - `docker/`: the `Client` interface, `NewClient`, `Ping`, labels, the network and container types and providers, the plugin, and the Mockery mock.
  - `cmd/docker-plugin/`: the binary that serves the Docker plugin.
  - `template/`: `TemplatePlugin`, type `template` with no subtype.
  - `main.go` (`apply`/`report`/`destroy`) and `config/main.xcl`.
  - Unit, integration, wiring and smoke tests.
  - `.mockery.yml` and a `make generate` target (Mockery v3.8.0).
  - The Docker SDK pinned in the example's own module, not in xcl's.
  - The old postgres/redis/app/ingress plugins, types and module are removed.
- **Docs.** The README "Plugins" and "Running them" passages, the README modules link, `docs/plugins.md` and `docs/README.md` now describe the Docker and template plugins.

## Why

The old example simulated everything and could not be lifted out as a starting point. Plugin authors needed a runnable, tested project that talks to a real system. The `template` block needed the library to support plugin types with no subtype, which it now does without changing anything for subtyped types.
