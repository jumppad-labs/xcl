---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Context: 20261006112023-aadf3c10-configuration-example

## Current State Analysis

- xclconfig (`/home/nicj/code/github.com/jumppad-labs/xcl`): `example/configonly/main.go` (working tree) generates a random state key, keeps state in a temp dir, builds the prettylog handler, and calls `run(handler, r, dir, stateDir, stateKey) ([]any, error)` (`main.go:109`), which registers `config_map`, `secret`, `deployment`, `service`, `ingress`, applies with `WithStatePath`, `WithStateMask`, `WithEventHandler`, `WithEventData(EventDataProcessed)` and returns `c.Entities()`; `main` dumps them with `kr/pretty` (`main.go:84`). `appConfig` (`main.go:98-103`) is declared but unused; `Service`/`Ingress` are pointer fields.
- `example/configonly/config/` holds `deployment.xcl` (variables, `config_map "api"`, `deployment "api"` with containers `api` [ports http/8080, metrics/9090] and `proxy` [proxy/8081]), `ingress.xcl` (`service "api"` port 80 target_port from `deployment.api.container[0].port[0].container_port`, `ingress "api"` host `api.example.com` rule `/` → `service.api` port 80, `output "api_url"`), `secret.xcl` (`secret "db"` with `env("DB_PASSWORD")`). Ids resolve to `<type>.<name>` (`example/configonly/main_test.go:207,224`).
- `example/configonly/resources/resources.go` — block types, unchanged by this plan.
- `example/appconfig/` — deleted in the working tree (unstaged).
- The e2e plan (`20261006071142-506b8289`) is not implemented yet at planning time; this plan assumes it has landed (module per example, stdlib smoke test, runner test, library tests moved to `e2e/`).
- `README.md:63-200` examples section quotes stale `resource.`-prefixed syntax and has an "Application configuration" section; `docs/README.md:48` lists appconfig.
- xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`, Astro 5 MDX): `src/pages/examples/configuration-only.mdx` quotes the old single `main.xcl`, old `run(out, …)` and old output; `src/pages/examples/application-config.mdx` exists; `Nav.astro:15`, `index.mdx:22,217-229`, `sensitive-values.mdx:29-75,113-120,149-154`, `state-masking.mdx:143-148`, `examples/plugins.mdx:562` refer to the application config example. No redirects (`astro.config.mjs`).

## Per-Task Technical Notes

Requirement-to-repo-and-files resolution:

- *Stands alone*, *reads as real code*, *uses its configuration*, *config unchanged* → xclconfig `example/configonly/{main.go,routes.go,go.mod,go.sum,Makefile}`: "Rewrite the configuration example as load, derive and print".
- *Has a smoke test* → xclconfig `example/configonly/smoke_test.go`: "Rewrite the configuration example…".
- *Shows testing config-driven code* → xclconfig `example/configonly/{main_test.go,routes_test.go,testdata/}`: "Test the route logic against a test configuration".
- *Application config example removed* → xclconfig `example/appconfig/`, `README.md`, `docs/README.md`: "Remove the application config example…"; xcl-website `src/pages/examples/application-config.mdx`, `src/components/Nav.astro`: "Remove the application config page…".
- *Website matches* → xcl-website `src/pages/examples/configuration-only.mdx`, `src/pages/sensitive-values.mdx`, `src/pages/state-masking.mdx`, `src/pages/index.mdx`, `src/pages/examples/plugins.mdx`: the three website tasks.

Baseline note: every xclconfig task assumes the e2e plan (`20261006071142-506b8289`) has landed — `example/configonly/go.mod` exists with `replace github.com/jumppad-labs/xcl => ../..` and `replace …/example/prettylog => ../prettylog`; `example/configonly/main_test.go` holds only output/exit tests; `example/configonly/smoke_test.go` uses `os/exec`; `e2e/examples_test.go` has `TestConfigOnlyExampleTestsPass`. If any of these is missing, STOP and ask. Line numbers below are from the working tree at planning time (commit 8271816 plus uncommitted edits) and will have shifted; re-read before editing.

### Task: Rewrite the configuration example as load, derive and print

**File changes**:
- `example/configonly/main.go:1-22` — rewrite the package doc: the program loads `./config` (or the directory given) into its own types and prints, per ingress path, host, path, service, deployment, container and port; mention encrypted state in a temporary directory and `make run`. Drop the "printed then destroyed" sentence.
- `example/configonly/main.go:24-35` — imports: drop `github.com/kr/pretty`; keep `crypto/rand`, `fmt`, `os`, `xcl`, `resources`, `prettylog`, `mask`, `registry`.
- `example/configonly/main.go:37-49` — keep `newStateKey` (used by main).
- `example/configonly/main.go:51-91` — `main`: resolve `dir`; `stateKey := newStateKey()`; `stateDir, err := os.MkdirTemp("", "xcl-example")` removed with `os.RemoveAll` before every exit, since `os.Exit` skips deferred calls; build `masker, err := mask.EncryptAES256GCM(stateKey)`; `r := registry.NewPluginRegistry()`; `cfg, err := loadConfig(dir, r, xcl.WithStatePath(stateDir), xcl.WithStateMask(masker), xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r)), xcl.WithEventData(xcl.EventDataProcessed))`; `routes, err := ingressRoutes(cfg)`; `for _, route := range routes { fmt.Println(route) }`; on any error `fmt.Fprintf(os.Stderr, "error: %s\n", err)` and exit 1. Keep the explanatory comments on each option from the current `run` (`main.go:142-169`).
- `example/configonly/main.go:94-104` — `appConfig` becomes `Deployments []*resources.Deployment`, `Services []*resources.Service`, `Ingresses []*resources.Ingress`; doc comment says slices receive every block in declaration order and the struct holds only what the program reads.
- `example/configonly/main.go:106-178` — replace `run` with `loadConfig(dir string, r *registry.PluginRegistry, options ...xcl.ConfigOption) (*appConfig, error)`: the five `r.RegisterType` calls (`config_map`, `secret`, `deployment`, `service`, `ingress`; secret and config_map must stay registered or the configuration fails to parse), `xcl.NewConfig(append([]xcl.ConfigOption{xcl.WithPluginRegistry(r)}, options...)...)`, `c.Apply(dir)`, `var cfg appConfig; c.Decode(&cfg)`, return `&cfg`. Wrap errors with context (`fmt.Errorf("loading configuration from %s: %w", dir, err)`).
- `example/configonly/routes.go` (new) — `type Route struct { Host, Path, Service string; ServicePort int; Deployment, Container, PortName string; Port int }`; `func (r Route) String() string` returning `fmt.Sprintf("%s%s -> %s:%d -> %s container %s port %s (%d)", …)`; `func ingressRoutes(cfg *appConfig) ([]Route, error)`: index services and deployments by `Meta.ID` (`types.ResourceBase.Meta.ID`, values like `service.api`; confirm the field path in `types/resource.go:6-9`), loop ingresses then rules in order; missing service → `fmt.Errorf("ingress %s path %s: service %q not found", ingress.Meta.ID, rule.Path, rule.Service)`; missing deployment → `"…: deployment %q of service %s not found"`; first `container.Ports[i]` with `ContainerPort == service.TargetPort` across `deployment.Containers` in order, none → `"…: no container in %s exposes port %d"`. Doc comments explain that references let the program follow links without label matching.
- `example/configonly/smoke_test.go` — success test: assert `stdout` contains `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)` and stderr has no `error:`; drop `## Resources`/`## Deployments` assertions. Missing-config test unchanged. The run must set `DB_PASSWORD` (`cmd.Env = append(os.Environ(), "DB_PASSWORD=example-password")`) since `secret.xcl` reads it, matching the Makefile; check what the e2e plan's smoke test already does.
- `example/configonly/main_test.go` — delete tests asserting the old output (`## Resources`, `## Deployments`, entity lists) that no longer compile against `loadConfig`; route tests arrive in the next task. If the file is left empty, delete it.
- `example/configonly/Makefile:1-2` — update the header comment ("Builds and runs the configuration example, which reports where each ingress path sends traffic"); targets unchanged.
- `example/configonly/go.mod`, `go.sum` — `go mod tidy` in the example drops `github.com/kr/pretty` (and `kr/text`, `rogpeppe/go-internal` if only it needed them).
- `example/configonly/resources/resources.go`, `example/configonly/config/*.xcl` — no change (config must stay byte-identical). Adjust a doc comment in resources.go only if it describes printing that no longer happens (none expected).

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential.

### Task: Test the route logic against a test configuration

**File changes**:
- `example/configonly/testdata/routes/` (new) — test configuration in the example's shape with different values: two `deployment` blocks (`web` with containers `web` port `http`/3000 and `sidecar` port `admin`/9000; `api` with containers `proxy` port `proxy`/8081 first and `api` port `grpc`/7000 second, so the target port is not on the first container), two `service` blocks (`web` → `deployment.web`, `port = 80`, `target_port = deployment.web.container[0].port[0].container_port`; `api` → `deployment.api`, `port = 9090`, `target_port = deployment.api.container[1].port[0].container_port`), one `ingress` (`host = "shop.test"`, rules `/` → `service.web` and `/api` → `service.api`). A `config_map` and `secret` are optional; if a secret is included use a literal, not `env()`, so tests need no environment.
- `example/configonly/testdata/unknown_service/`, `testdata/unknown_deployment/`, `testdata/unknown_target_port/` (new) — minimal configs. Because references are resolved at parse time, write the broken link as a literal id string (e.g. `service = "service.missing"`, `deployment = "deployment.missing"`, `target_port = 1234`), not a reference, so `loadConfig` succeeds and `ingressRoutes` reports the error.
- `example/configonly/routes_test.go` (new) — `TestIngressRoutesReportsEveryIngressPath`: `cfg, err := loadConfig(filepath.Join("testdata", "routes"), registry.NewPluginRegistry())`, `routes, err := ingressRoutes(cfg)`, `require.Equal` against the full `[]Route{…}` literal in declaration order. `TestIngressRoutesFailsForUnknownService`, `TestIngressRoutesFailsForUnknownDeployment`, `TestIngressRoutesFailsForUnreachableTargetPort`: each loads its directory, `require.Error`, `require.ErrorContains` the ingress id and path. Optionally `TestRouteStringNamesEveryHop` asserting the `String` format for one route literal (main uses `String`).
- `example/configonly/main_test.go` — remove any remaining test of the old output; keep nothing that duplicates the smoke test. Delete the file if empty.
- No change to `e2e/examples_test.go`: `TestConfigOnlyExampleTestsPass` already runs `go test ./...` in the example.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

### Task: Remove the application config example and update the repository docs

**File changes**:
- `example/appconfig/` — confirm absent (deleted in the working tree: `Makefile`, `config/app.xcl`, `main.go`, `main_test.go`, `resources/resources.go`); `git rm` if still tracked. No `go.mod` exists for it.
- `README.md:63-66` — "three self-contained programs" → two (configuration and plugin), each its own module.
- `README.md:68-136` — rewrite "Configuration only": describe the Kubernetes-like configuration (now three files: `deployment.xcl`, `ingress.xcl`, `secret.xcl`), nested/repeated/linked blocks with the current unprefixed reference syntax (`deployment.api.container[0].port[0].container_port`, `config_map.api.data.db_host`), quote a trimmed `deployment "api"` block copied from `example/configonly/config/deployment.xcl`, quote the current `appConfig` and the `loadConfig` Decode lines, describe `ingressRoutes`, and show the route output line. Keep the cross-link to "Filling a struct of your own".
- `README.md:162-200` — delete the "Application configuration" section.
- `README.md:220-233` — leave the "Running them" paragraph to the e2e plan; only fix a sentence there if it names appconfig (e.g. "Every example … prints the resources it parsed" — adjust to "prints what it read" only if the e2e plan's text still says so; coordinate, do not rewrite the paragraph).
- `docs/README.md:48` — layout row: two examples (`configonly`: Kubernetes-like configuration decoded into registered types and reported as ingress routes, no plugin; `plugin`), sharing `prettylog/`; drop `appconfig`.
- `docs/plugins.md:406-409` — still accurate; leave.
- Run `grep -rn -i -e appconfig -e "application config" -e application-config` excluding `.spektacular/`, `.git/`, and `CHANGELOG.md` (historical entries, not edited by this spec); fix any other current reference.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential.

### Task: Rewrite the website's configuration example page

**File changes**:
- `xcl-website:src/pages/examples/configuration-only.mdx:1-15` — front matter description and Hero `sub`: a Kubernetes-like configuration loaded into Go types and reported as ingress routes.
- `xcl-website:src/pages/examples/configuration-only.mdx:17-36` — intro: the program reads its configuration and acts on it; link to `example/configonly`.
- `xcl-website:src/pages/examples/configuration-only.mdx:38-108` — "The Go types": re-copy `ConfigMap`, `Secret`, `Deployment`, `Container`, `Service`, `Ingress`/`Rule` from `example/configonly/resources/resources.go` verbatim (comments now use unprefixed references).
- `xcl-website:src/pages/examples/configuration-only.mdx:110-222` — "The configuration": replace the single `main.xcl` block with three titled blocks copied verbatim from `example/configonly/config/deployment.xcl`, `ingress.xcl`, `secret.xcl` (trim comments only if the trimmed text still matches contiguous source lines; prefer whole files).
- `xcl-website:src/pages/examples/configuration-only.mdx:224-320` — "The program": quote `appConfig`, `loadConfig`, the option list in `main`, and `ingressRoutes`/`Route` from `example/configonly/main.go` and `routes.go` verbatim.
- New section "Testing it" — quote the test configuration's ingress and `TestIngressRoutesReportsEveryIngressPath` from `example/configonly/routes_test.go`, explaining the test configuration is separate from `./config`.
- `xcl-website:src/pages/examples/configuration-only.mdx:322-380` — "Run it": real stderr event lines (resources now `config_map.api` etc., files `config/deployment.xcl`…) and the stdout route line, captured from `make run` in the example.
- `xcl-website:src/pages/examples/configuration-only.mdx:382-410` — "What to notice": routes follow ids; keep the declared-once, linked-not-matched, nesting, Decode points.
- `xcl-website:src/pages/examples/configuration-only.mdx:412-420` — CtaBanner: "One more example", single button to `/examples/plugins/`; remove the application-config button.
- Verify with `make build` (or `npm run build`) and `make check` in xcl-website.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential.

### Task: Remove the application config page and repoint its links

**File changes**:
- `xcl-website:src/pages/examples/application-config.mdx` — delete.
- `xcl-website:src/components/Nav.astro:15` — remove the "Application config file" entry.
- `xcl-website:src/pages/index.mdx:22` — hero "See an example" button → `/examples/configuration-only/`.
- `xcl-website:src/pages/index.mdx:207-221` — "Three examples" → "Two examples"; drop the application-config bullet; reword the configuration-only bullet to mention route reporting.
- `xcl-website:src/pages/index.mdx:224-230` — CtaBanner primary button → "Configuration only →" `/examples/configuration-only/`.
- `xcl-website:src/pages/state-masking.mdx:143-148` — body, rewritten in full by this plan: "The configuration example encrypts its state with a fresh key each run." It names only the configuration example, the one example that still encrypts its state; the docker-plugin spec removes state encryption from the plugin example and does not edit this page; button → `/examples/configuration-only/` "Configuration example →".
- `xcl-website:src/pages/examples/plugins.mdx:562` — remove only the application-config `<Button>`; leave every other line to the docker-plugin spec. If that spec has already rewritten the CTA without it, nothing to do.
- `grep -rn -e application-config -e "application config" -e appconfig src` in xcl-website must return nothing; `make build` must succeed.

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential.

### Task: Illustrate sensitive values with the configuration example

**File changes**:
- `xcl-website:src/pages/sensitive-values.mdx:26-46` — replace the `example/appconfig/resources/resources.go` `Database` snippet with the `Secret` type copied verbatim from `example/configonly/resources/resources.go` (`types.Sensitive[map[string]string]`), and the `app.xcl` line with the `secret "db"` block from `example/configonly/config/secret.xcl`. Adjust the surrounding sentence: the field type is `types.Sensitive[T]`, here a map.
- `xcl-website:src/pages/sensitive-values.mdx:58-75` — keep the Reveal explanation; drop the `title="example/appconfig/main.go"` attribute so the `databaseURL` illustration is no longer attributed to an example file, or rewrite it as a short untitled snippet reading one key from `secret.Data.Reveal()`.
- `xcl-website:src/pages/sensitive-values.mdx:111-120` — replace "The application config example prints its configuration as JSON…" with a neutral sentence ("Marshalling a struct holding one with `json.Marshal` writes the marker:") and keep the JSON snippet untitled.
- `xcl-website:src/pages/sensitive-values.mdx:149-154` — CtaBanner body: "The configuration example declares its database password sensitive, encrypts it in state and prints no secret."; button → `/examples/configuration-only/` "Configuration example →".
- `make build` in xcl-website succeeds.

**Complexity**: Low
**Token estimate**: ~6k tokens
**Agent strategy**: Single agent, sequential.

## Testing Strategy

- *Rewrite the configuration example as load, derive and print*: `example/configonly/smoke_test.go` success test asserts the default route line `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)` on stdout and no `error:` on stderr; missing-config test asserts failure with `error:`. Old output tests removed so the package compiles.
- *Test the route logic against a test configuration*: `example/configonly/routes_test.go` — one positive test asserting the full `[]Route` for `testdata/routes`; one test each for unknown service, unknown deployment and unreachable target port, each with its own `testdata/` directory and asserting the error names the ingress and path. testify `require`, no tables, positive and negative apart. Run via `go test ./...` in the example and via `e2e.TestConfigOnlyExampleTestsPass`.
- *Remove the application config example and update the repository docs*: no automated tests; tests never check documentation. The README is checked by human review (README snippets against the source in the manual snippet review).
- Website tasks: `make build` and `make check` in xcl-website; no automated tests.
- Manual checks (captured in the implementation test plan): copy-out build against a published xcl; no test-only seams review; config files byte-identical; `make run` output; website snippets match source; no remaining appconfig references and no broken links.

## Project References

- Spec: `20261006112023-aadf3c10-configuration-example` (epic `20261006071139-7b266535-examples-and-output`).
- Prior plan built on: `20261006071142-506b8289-e2e-suite-and-real-world-examples` (final).
- Sibling spec: `20261006112023-f7a185dc-docker-plugin-example`.
- Design documents: none.
- Knowledge: `conventions/testing-and-mocking.md`, `conventions/code-style.md`, `conventions/dependencies.md`, `conventions/patterns-and-architecture.md`, `conventions/never-modify-dependencies.md` (xclconfig repo tier).
- Repo roots: xclconfig `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

All tasks are Low or Medium and run as a single agent each. The xclconfig tasks run in order; the three website tasks follow the route tests (the configuration page needs the real code and output), and the last two can run in parallel after the configuration page.

## Migration Notes

- The website URL `/examples/application-config/` stops existing; no redirect is added.
- No library or state migration.

## Performance Considerations

None: the example's route derivation is a linear walk over a handful of blocks.
