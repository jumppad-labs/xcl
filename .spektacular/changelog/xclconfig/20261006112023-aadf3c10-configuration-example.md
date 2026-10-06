---
created_date: "2026-10-06"
document_status: draft
project: xclconfig
spec: 20261006112023-aadf3c10-configuration-example
plan: 20261006112023-aadf3c10-configuration-example
---

# Configuration example rewritten as a real application

The configuration example now reads its configuration into its own Go types and reports, for every ingress path, the host, path, service, deployment, container and port the traffic reaches. It shows how to test configuration-driven code against a test configuration of its own, and can be copied out as the start of a new project. The separate application config example has been removed.

> Derived from project xcl (xclconfig), spec/plan 20261006112023-aadf3c10-configuration-example. See the project-level record for the full feature.

## What changed in this repo

- `example/configonly/main.go`: `loadConfig(dir, registry, options...)` registers the block types, applies and decodes into `appConfig` (slices of deployments, services, ingresses); `main` wires encrypted temporary state and the prettylog handler in as options and prints one route per line. The old section printing and `Destroy` are gone.
- `example/configonly/routes.go` (new): `Route`, `Route.String` and `ingressRoutes`, which follow ingress rule → service → deployment → container port and fail with an error naming the ingress and path.
- `example/configonly/routes_test.go` and `testdata/` (new): route tests against a separate test configuration and one test per broken link.
- `example/configonly/smoke_test.go`: asserts the default route line; `main_test.go` (old output tests) removed. Makefile comment updated.
- `README.md`: examples section describes two examples and the rewritten configuration example; the application configuration section is removed. `docs/README.md`: layout table lists two examples.

## Why

The example is meant to be the program a developer would write for their own tool, tested the way they would test it, and the repository's docs must quote code that exists.
