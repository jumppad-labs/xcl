---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Configuration example rewritten as a real application

## What was built

The configuration example (`example/configonly`) is now a small application rather than a test harness. It loads its Kubernetes-like configuration (a config map, a secret, a deployment, a service and an ingress across three files, unchanged) into its own `appConfig` struct with one `Decode` call, follows the links from every ingress path to the service, deployment, container and port the traffic reaches, and prints one line per route:

```
api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)
```

A link that cannot be followed is an error naming the ingress and path. The program still keeps encrypted state in a temporary directory and sends events to the prettylog receiver, passed to its loader as ordinary config options.

Its tests are the ones an application author writes: route tests against a separate test configuration under `testdata/` (two services, two deployments, two paths), one test per broken link (unknown service, unknown deployment, unreachable target port), a `Route.String` test, and the smoke test that builds and runs the binary and checks the default route line. The old tests of the previous printed output were removed.

The application config example is gone. In xcl, the README's examples section describes two examples and quotes the rewritten configuration example, and `docs/README.md`'s layout table lists two. On the website, the configuration-only page is rewritten from the current source (types, three config files, loader, route logic, tests and real output), the application-config page and its nav entry are deleted, the home, state-masking and plugins pages point at the configuration example, and the sensitive-values page illustrates sensitive data with the configuration example's `Secret` type.

## Why it matters

Application developers get an example they can copy and run as the start of their own project, showing how to read configuration into their own types and how to test configuration-driven code. The README and website now quote code that actually exists.

## Deviations from the plan

- The starting program differed from the plan's description (it printed sections and destroyed state; `kr/pretty` was already gone), so there was no dependency to drop.
- A small `run(dir string) error` is kept, called only by `main`, so the temporary state directory is removed with `defer` before `os.Exit`.
- The plugins page's call-to-action body was reworded as well as its button removed, because it counted the removed example.
- In the README's "Running them" paragraph only "prints the resources it parsed" became "prints what it read".
