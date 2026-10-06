---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Research: 20261006112023-aadf3c10-configuration-example

## Alternatives considered and rejected

- **Keep `run(handler, r, dir, stateDir, stateKey) ([]any, error)` and add a routes function over `[]any`.** Rejected: `run` returns the applied entity list only so tests and `pretty.Print` can poke at it (`example/configonly/main.go:109-178`); a type switch over `[]any` is not how an application reads configuration, and `Decode` already fills `appConfig` (`decode.go:9-35`). The spec steers to "decode then derive routes".
- **Drop state and state masking from the program** (no `WithStatePath`, so `stateStore` is nil, `config.go:139`, `config_options_test.go:19`). Rejected: the working tree's latest direction (commit 8271816 "Add example to show state encryption", `example/configonly/main.go:37-49,135-160`) is that configonly demonstrates encrypted state, and once appconfig is gone, and with the docker-plugin spec removing state encryption from the plugin example, configonly is the only example of encrypted state the website's state-masking and sensitive-values pages can point at.
- **Test `main` output only through the smoke test.** Rejected for route logic: the spec requires the consuming code be tested against a separate test config, so route derivation is a function tested directly; the smoke test only checks the binary prints the default route.
- **Keep the application-config website page as a redirect or stub.** Rejected: astro has no redirects configured (`xcl-website/astro.config.mjs`) and the spec allows "removed or merged"; the page is deleted and every link to it repointed.
- **Make `appConfig.Service`/`Ingress` pointers as today.** Rejected: `Decode` fails a `*T` field when more than one block of T is declared (`decode.go:20-23`); a test config with two ingresses/services, and real configs, need `[]*T`.

## Chosen approach — evidence

- `decode.go:9-35` — `[]*T` receives every entity of T in declaration order; unused types can simply be left out of the struct.
- `options.go:27-45` — `WithStatePath`/`WithStateStore` are options; passing options variadically lets `main` add state, mask, handler and event data while tests pass none.
- `example/configonly/main_test.go:200-226` — ids resolve to `"service.api"`, `"deployment.api"`; `service.TargetPort == 8080` resolved from `deployment.api.container[0].port[0].container_port`; ingress rule `{Path "/", Service "service.api", Port 80}`.
- `example/configonly/config/{deployment,ingress,secret}.xcl` — the config to keep unchanged: config_map, secret, deployment (two containers, ports http/metrics/proxy), service, ingress, output, two variables.
- E2E plan `20261006071142-506b8289-e2e-suite-and-real-world-examples` — configonly becomes its own module with `replace` directives, stdlib `os/exec` smoke test, runner test `TestConfigOnlyExampleTestsPass` in `e2e/examples_test.go`; its library-behaviour tests have moved to `e2e/` and source-inspection tests are deleted. This plan starts from that layout.

## Files examined

- `xclconfig:example/configonly/main.go:1-178` — working-tree program: random state key, temp state dir, prettylog handler, `run` registers five types, applies with state mask + event data, returns `c.Entities()`; `appConfig` declared but unused; `pretty.Print(entities)` in main.
- `xclconfig:example/configonly/resources/resources.go` — ConfigMap, Secret (Sensitive map), Deployment/Container/Port/EnvVar/…, Service{Deployment, Port, TargetPort}, Ingress{Host, Rules[]Rule{Path, Service, Port}}.
- `xclconfig:example/configonly/config/*.xcl` — three files, unchanged by this plan.
- `xclconfig:example/configonly/main_test.go` — 920 lines, mostly library behaviour; after the e2e plan only output/exit tests remain; this plan replaces the file.
- `xclconfig:example/configonly/smoke_test.go` — asserts `## Resources`/`## Deployments` output (will be rewritten to stdlib by e2e plan; this plan changes its assertions to routes).
- `xclconfig:example/configonly/Makefile` — `run` exports `DB_PASSWORD`; `go run . ./config`.
- `xclconfig:git show HEAD:example/appconfig/*` — appconfig already deleted in working tree (unstaged `D`).
- `xclconfig:README.md:63-235` — examples section: "three self-contained programs", configonly section quotes stale `resource.`-prefixed syntax and the `appConfig` snippet, an "Application configuration" section (162-200), "Running them" (owned by e2e plan).
- `xclconfig:docs/README.md:48` — repo layout table names `appconfig`.
- `xclconfig:docs/plugins.md:406-409` — mentions configonly (still accurate).
- `xclconfig:example/plugin/config/main.xcl:1`, `example/plugin/main_test.go:68` — mention configonly (plugin spec's territory).
- `xclconfig:CHANGELOG.md:1-15` — masking entry mentions "application-config and plugin examples" (historical entry, left as is; this spec does not edit the changelog).
- `xcl-website:src/pages/examples/configuration-only.mdx` — quotes `main.xcl`, `resource.`-prefixed config, old `run(out, …)` program and `printDeployments` output; all stale.
- `xcl-website:src/pages/examples/application-config.mdx` — to delete.
- `xcl-website:src/components/Nav.astro:14-15` — nav entries for both pages.
- `xcl-website:src/pages/index.mdx:22,217-229` — hero and CTA buttons and "Three examples" list link to application-config.
- `xcl-website:src/pages/sensitive-values.mdx:29-75,113-120,149-154` — quotes appconfig `Database` type, `app.xcl`, `databaseURL`, JSON print; CTA to application-config.
- `xcl-website:src/pages/state-masking.mdx:143-148` — CTA names application config example; this plan owns the whole CTA and rewrites it to name only the configuration example (the docker-plugin spec removes the plugin example's state encryption and does not edit this page).
- `xcl-website:src/pages/configuration-text.mdx:190-195` — CTA to configuration-only (fine).
- `xcl-website:src/pages/examples/plugins.mdx:47,560-563` — CTA button to application-config; rest of page owned by docker-plugin spec.
- `xcl-website:astro.config.mjs` — no redirects; `Makefile` has `build` and `check` targets.

## External references

- None needed; Go `os/exec` and Astro static pages behave as documented.

## Prior plans / specs consulted

- `20261006071142-506b8289-e2e-suite-and-real-world-examples` (plan, final) — module layout, `replace` directives, runner test names, smoke test via `os/exec`, README "Running them" ownership, coverage map listing configonly tests.
- `20261006112023-f7a185dc-docker-plugin-example` (spec) — sibling owns plugins page, events and plugin-logging pages; overlap is only the plugins page CTA linking to application-config.

## Open assumptions

- The e2e plan has landed before implementation: `example/configonly/go.mod` exists, its main_test.go holds only output/exit tests, smoke test uses `os/exec`, `e2e/examples_test.go` has `TestConfigOnlyExampleTestsPass`. If not, STOP and ask.
- `meta.id` of a block resolves to `<type>.<name>` (e.g. `service.api`) — evidenced by current tests.
- The website repo is clean and on main; its build (`npm run build`) works locally with `node_modules` present.

## Drafting assumptions

### Keep encrypted state in the configuration example (discovery)
- **Decision**: The rewritten program keeps a temporary file state store encrypted with a fresh random key, as the working tree does today.
- **Rationale**: It is the user's latest direction (commit 8271816) and, with appconfig gone, configonly is the example the state-masking and sensitive-values pages point to.
- **Rejected**: Dropping state entirely (simpler, but loses the only remaining demonstration of encrypted state).

### Website pages that quoted appconfig move to configonly (discovery)
- **Decision**: sensitive-values quotes configonly's `Secret` type and `secret.xcl`; its Reveal snippet becomes an untitled illustrative snippet; index, nav and plugins-page CTAs drop application-config links, pointing at configuration-only; the state-masking CTA is rewritten by this plan to name only the configuration example as the example that encrypts its state.
- **Rationale**: Acceptance requires no page refer to the application config example and every titled snippet match its source.
- **Rejected**: Keeping appconfig snippets untitled (still describes an example that no longer exists).

### Plugins page: only the application-config button changes here (discovery)
- **Decision**: This plan removes only the application-config CTA button from `examples/plugins.mdx`; the sibling docker-plugin plan owns the rest of that page.
- **Rationale**: Our acceptance criterion forbids references to appconfig anywhere; the sibling owns the page body.
- **Rejected**: Leaving it for the sibling (would leave our criterion unmet if the sibling does not touch the CTA).

### Chosen direction: load, derive routes, print (architecture)
- **Decision**: Split the program into `loadConfig(dir, registry, options...) (*appConfig, error)` (register, apply, Decode), `ingressRoutes(cfg) ([]Route, error)` and printing in `main`; `appConfig` holds `[]*Deployment`, `[]*Service`, `[]*Ingress`; tests load `testdata/` through `loadConfig` with no options.
- **Rationale**: Follows the spec's "decode then derive routes" steer; every parameter is used by `main`, so there is no test-only seam; slices let real and test configs declare several services/ingresses.
- **Rejected**: Keeping `run` returning `[]any` (test-only return value); pointer fields (Decode fails on more than one block).

### Route resolution rules (architecture)
- **Decision**: A route's container and port are the first container port (in declaration order) whose `container_port` equals the service's `target_port`; an unresolved service, deployment or target port is an error naming the ingress and path.
- **Rationale**: Mirrors how Kubernetes resolves a numeric targetPort; failing loudly is what a real tool would do.
- **Rejected**: Skipping unresolved rules silently (hides config mistakes).

### Changelog entry for this spec (architecture)
- **Decision**: Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented. The plan carries a short "Changelog input" note for that entry.
- **Rationale**: Specs in the same epic that touch different code no longer collide on `CHANGELOG.md` and are not forced to run in sequence (user decision).
- **Rejected**: A `## <spec name>` section per spec (causes collisions between sibling specs).

### Conventions selected (architecture)
- **Decision**: Apply testing-and-mocking, code-style, dependencies, patterns-and-architecture and never-modify-dependencies; drop database, development-standards (structured logging handled by prettylog/events already), project-structure, shared-errors, shared-test-helpers (example cannot import internal/testutil), graph-ordering and test-state conventions (no state or ordering tests here).
- **Rationale**: Only these bear on a rewritten example program and its tests.
- **Rejected**: Listing all conventions.

### Route line format (data_structures)
- **Decision**: Print `<host><path> -> <service>:<service port> -> <deployment> container <container> port <port name> (<port>)`, one line per route, via `Route.String`.
- **Rationale**: Carries all six facts the spec requires on one readable line; easy to assert in the smoke test.
- **Rejected**: A table or JSON output (heavier for a teaching example).

### Historical changelog entries keep their appconfig mention (testing_approach)
- **Decision**: Earlier `CHANGELOG.md` sections that mention the application-config example are left as written; "no page refers to the application config example" applies to the website and current docs.
- **Rationale**: Changelog sections record what shipped at the time; rewriting history is not what the acceptance criterion is about.
- **Rejected**: Editing old changelog entries.

### Unchanged config and snippet match checked by review (testing_approach)
- **Decision**: "Config unchanged", "no test-only seams", "stands alone" and "website matches" are manual checks, not automated tests.
- **Rationale**: Automating them would mean tests that inspect source or other repos, which the testing conventions forbid.
- **Rejected**: A golden-file test of the config directory.

### Error-path test configs use literal ids (tasks)
- **Decision**: The broken-link test configurations write the missing service/deployment as literal id strings and the unreachable target port as a literal number.
- **Rationale**: A reference to a missing block fails at parse time inside xcl, which would test xcl rather than the example's route logic.
- **Rejected**: References to missing blocks.

### Website tasks split by page group (tasks)
- **Decision**: Three website tasks: configuration-only page; delete application-config page and repoint links (nav, index, state-masking, plugins CTA); sensitive-values page.
- **Rationale**: Each is independently reviewable; the configuration-only page owns its own CTA so the tasks do not edit the same lines.
- **Rejected**: One large website task.

## Rehydration cues

- `spektacular spec file read 20261006112023-aadf3c10-configuration-example`
- `spektacular plan file read 20261006071142-506b8289-e2e-suite-and-real-world-examples plan` (and `context`)
- Read `example/configonly/{main.go,resources/resources.go,config/*.xcl,smoke_test.go}`.
- `grep -rn -i -e appconfig -e application-config -e "application config"` in both repos (excluding `.spektacular`, `node_modules`, `dist`).
