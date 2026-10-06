---
created_date: "2026-10-06"
---

# Planning summary: 20261006071139-7b266535-examples-and-output

## Decisions
- **State-masking call to action (xcl-website `state-masking.mdx`).** Plans: configuration example (aadf3c10) and docker plugin example (f7a185dc). The configuration plan expected the plugin example to keep `XCL_STATE_KEY`, while the docker plan removes state encryption and edited the same call to action. **Outcome:** the configuration example plan owns the whole call to action and points it only at the configuration example, the one example that still encrypts its state; the docker plan does not edit `state-masking.mdx`.
- **Tests that inspect repository files.** Plans: e2e suite (506b8289), configuration example (aadf3c10) and docker plugin example (f7a185dc) each added a changelog "content test" in `readme_test.go`; encoder highlighting (17623cda) added none, and the e2e and highlighting plans relied on `static_dependencies_test.go`. **Outcome (user):** no test ever checks documentation, CI configuration, `go.mod` or the repository's own source. All such tests were removed from the plans, and from the code: `readme_test.go`, `ci_workflow_test.go`, `static_output_test.go`, `static_dependencies_test.go`, `state/dependencies_test.go`, `state/public_surface_test.go`, `events/imports_test.go` and the source scan in `query_migration_test.go` are deleted. Documentation, CI configuration and the dependency boundary are checked by human review. Recorded in the always-applied convention `conventions/testing-and-mocking.md`.
- **Changelog.** Plans: all four each added a `## <spec>` section at the top of xcl's `CHANGELOG.md`, which made `epic order` serialise every pair of specs. **Outcome (user):** no spec edits `CHANGELOG.md`. The changelog is written once at the end of the epic, after every spec is implemented, as a single entry built from each plan's `Changelog input` notes. The changelog-only dependency of encoder highlighting on the configuration example was removed.

## Order added for shared files
- Added: `20261006112023-f7a185dc-docker-plugin-example` now depends on `20261006112023-aadf3c10-configuration-example`. Both change `xclconfig:./config`, `xclconfig:main.go`, `xclconfig:go.sum`, `xclconfig:e2e/examples_test.go`, `xclconfig:go.mod`, `xclconfig:README.md`, `xclconfig:docs/plugins.md`, `xclconfig:CHANGELOG.md`, `xcl-website:src/components/Nav.astro`, `xcl-website:src/pages/index.mdx`, `xcl-website:src/pages/examples/plugins.mdx`, `xcl-website:src/pages/sensitive-values.mdx`.
- Added: `20261006112108-17623cda-encoder-syntax-highlighting` now depends on `20261006071142-506b8289-e2e-suite-and-real-world-examples`. Both change `xclconfig:internal/testutil`, `xclconfig:example/prettylog/prettylog_test.go`, `xclconfig:prettylog_test.go`, `xclconfig:internal/test_fixtures/config/encode/main.xcl`, `xclconfig:example/prettylog/highlight_test.go`, `xclconfig:CHANGELOG.md`.
- Added: `20261006112108-17623cda-encoder-syntax-highlighting` now depends on `20261006112023-aadf3c10-configuration-example`. Both change `xclconfig:CHANGELOG.md`.
- Added: `20261006112108-17623cda-encoder-syntax-highlighting` now depends on `20261006112023-f7a185dc-docker-plugin-example`. Both change `xclconfig:internal/testutil`, `xclconfig:CHANGELOG.md`.
- Removed at review: `20261006112108-17623cda-encoder-syntax-highlighting` no longer depends on `20261006112023-aadf3c10-configuration-example`; they may be implemented side by side.

## 20261006071142-506b8289-e2e-suite-and-real-world-examples
### Approach

All work is in xclconfig. A new top-level `e2e/` package inside the root module (so `go test ./... -race` in CI runs it) uses xcl only through public packages plus `internal/testutil`, with its own fixtures (`e2e/testdata/` configs; Go block types, an in-process plugin and an external plugin under `e2e/fixtures/`, the external one built once in `TestMain`). Every library-behaviour test in the examples is rebuilt as a named e2e test or mapped to an existing library test, recorded in `e2e/COVERAGE.md`. configonly, plugin and prettylog each become their own Go module with `replace` pointing at `../..` (configonly and plugin also at `../prettylog`); import paths unchanged. The e2e suite runs each example's `go test ./...` from one named test per example, whose failure names the example. Source-reading tests are deleted; the "examples compile on the oldest supported Go" guarantee moves to a CI job building and vetting each example on Go 1.25.0.

Rejected: a separate e2e module (root CI step would not reach it); a `go.work` file (contradicts the spec's `replace` steer); keeping examples in the root module (breaks one-module-per-example).

### Milestones and tasks

**M1 — xcl has its own end-to-end suite** (coverage exists before anything is removed)
- Build the e2e fixtures
- Carry the configuration-only behaviours into the e2e suite
- Carry the plugin behaviours into the e2e suite
- Write the coverage map

**M2 — examples test only what they do**
- Strip library and source tests from the configuration-only example (confirms `static_examples_test.go` stays deleted)
- Strip library and source tests from the plugin example

**M3 — each example is a standalone project the suite runs**
- Give prettylog fixtures of its own (replaces `internal/parser.TestPlugin`, `internal/test_fixtures/registered` and the `../../internal` config)
- Make each example its own module (smoke tests use `os/exec`; root `go mod tidy` drops charmbracelet, termenv and kr/pretty)
- Run the example tests from the e2e suite (passing and failing fixture modules under `e2e/testdata` test the runner)
- Build the examples on the minimum supported Go in CI
- Document the new layout (README "Running them" paragraph)

### Tasks a person must do

None — all 11 tasks are agent tasks.

### Out of scope

- Encoder highlighting (spec 17623cda).
- Rewriting configonly and plugin as real applications, removing test-only seams, README/website prose — the two sibling specs.
- The appconfig removal (already in the working tree).
- Publishing a public test-helper package.
- Any library API change.
- A glob guard catching examples without a runner test.

### Drafting assumptions

- e2e is a package in the root module; examples are modules with `replace`; one named runner test per example; source-inspection tests deleted; CI builds each example on the minimum Go.
- No glob guard over `example/*` (globbing another directory from a test is what the testing convention forbids).
- Conventions applied: testing-and-mocking, shared-test-helpers, test-state-from-real-apply, assert-ordering-on-graph-parents, dependencies, code-style, never-modify-dependencies.
- e2e may import `internal/testutil` and no other internal package.
- Example smoke tests call `os/exec` directly; nothing shared between examples.
- Tests to carry over: those in the current example test files plus any at HEAD 8271816 the uncommitted tree already dropped.
- The coverage map is Markdown (`e2e/COVERAGE.md`) with no test parsing it.
- The runner is tested against passing and failing fixture modules.
- Open at implementation: the working tree is mid-edit (`example/configonly/main_test.go` calls a `run(out, …)` signature `main.go` no longer has) — ask which baseline if it still does not compile; ask before importing any internal package a carried-over behaviour turns out to need.

### Project-wide rules

- Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.
- Example module layout: module path `github.com/jumppad-labs/xcl/example/<name>`, `go 1.25.0`, `replace github.com/jumppad-labs/xcl => ../..`; configonly and plugin also `replace .../example/prettylog => ../prettylog`.
- Examples import no `internal/` package; smoke tests use `os/exec` directly.
- A new example adds its own named runner test in `e2e/examples_test.go` (`Test<Name>ExampleTestsPass`).
- e2e tests import only public packages plus `internal/testutil`; fixtures in `e2e/fixtures` (Go) and `e2e/testdata` (configs).
- The `build-minimum-go` CI job builds and vets every example module on Go 1.25.0.
- Root `go.mod` no longer requires charmbracelet, termenv or kr/pretty.
- README "Running them" paragraph rewritten here; rest of the examples section left to the sibling specs.

### Manual checks

- Copy prettylog out of the repo, point it at a published xcl version, confirm it builds and its tests pass.
- Review the coverage map against the removed tests (current files and HEAD 8271816): each library-behaviour test appears once, with a covering test that exists.
- Break behaviours named in the map and confirm an e2e or library test fails, not only an example test (success metric 2).
- Make a purely stylistic rewrite of an example and confirm no test fails (success metric 1).
- From a fresh checkout, run the full suite and confirm e2e passes and imports no internal package other than `internal/testutil`.
- After a root `go mod tidy`, review that `go.mod` no longer lists `charmbracelet/*`, `muesli/termenv` or `kr/pretty`.
- Review the `build-minimum-go` CI workflow change by hand (CI proves it on push).

### Changelog input

- New end-to-end suite (`e2e/`) exercising xcl through its public packages only, with a coverage map recording where every library-behaviour test removed from the examples is now covered.
- The configonly, plugin and prettylog examples are now standalone Go modules pointed at the local xcl; xcl's `go test ./...` runs their tests through the e2e suite, and CI builds each example on the minimum supported Go.
- xcl's module no longer requires the charmbracelet libraries, `muesli/termenv` or `kr/pretty`; source-inspection tests in the examples were removed.
- Breaking changes: none.

## 20261006112023-aadf3c10-configuration-example
### Approach

Rewrite `example/configonly` as a three-step program:
- `loadConfig(dir, registry, options ...xcl.ConfigOption) (*appConfig, error)` registers the 5 block types, applies the config and fills `appConfig` (slices `[]*Deployment`, `[]*Service`, `[]*Ingress`) with one `Decode`.
- `ingressRoutes(cfg) ([]Route, error)` follows each ingress rule to its service, then the deployment, then the container and port matching the service's `target_port`; an unfollowable link is an error naming the ingress and path.
- `main` passes state, encryption, the prettylog handler and the event data level as ordinary config options and prints one `Route.String()` line per route, e.g. `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)`.

`kr/pretty` and the `[]any` return value are dropped; config files untouched; no test-only seams. Rejected: keeping `run` returning `[]any` (exists only for tests/debugging); dropping state from the program (with appconfig gone, the encrypted-state and sensitive-values pages would have no non-plugin example).

### Milestones and tasks

**M1 — The configuration example is a real application** (xclconfig)
- Rewrite the configuration example as load, derive and print (updates the smoke test's expected output)
- Test the route logic against a test configuration (`testdata/routes`; one config and test per failure: unknown service, unknown deployment, target port no container exposes)

**M2 — The repository has one configuration example and documents it** (xclconfig)
- Remove the application config example and update the repository docs (README configuration-only section rewritten, application-config section deleted, `docs/README.md` layout row fixed)

**M3 — The website matches the configuration example** (xcl-website)
- Rewrite the website's configuration example page
- Remove the application config page and repoint its links (nav, home page hero/example list/call to action, state-masking call to action, only the application-config button on the plugins page)
- Illustrate sensitive values with the configuration example

### Tasks a person must do

None — all tasks are agent tasks.

### Out of scope

- Generating Kubernetes manifests.
- Docs drift from issue #4 that the examples did not cause (e.g. the README's state-masking snippet).
- Changing the config files or block types.
- The plugin example and the plugins, events and plugin-logging pages (docker-plugin spec).
- The README "Running them" paragraph, module files, the e2e runner and CI (e2e plan).
- Rewriting old changelog sections.
- Website redirects for the removed URL.
- Any library or API change.

### Drafting assumptions

- The program keeps a temporary state store, encrypted with a random key each run.
- The sensitive-values page quotes configonly's `Secret` type and `secret.xcl`; its Reveal snippet is no longer credited to an example file, and the passage about printing JSON names no example.
- Only the application-config button on the plugins page changes here.
- Route resolution picks the first container port (declaration order) equal to the service's `target_port`; an unresolved link is an error, never skipped.
- Route line format as in the approach.
- Conventions applied: testing-and-mocking, code-style, dependencies, patterns-and-architecture, never-modify-dependencies.
- Old changelog sections mentioning appconfig are left as written.
- "Config unchanged", "no test-only seams", "stands alone" and "website matches" are checked by hand.
- Error-path test configs write missing ids and the bad port as literal values, not references (a missing reference would fail in xcl's parser).
- Website work is split into 3 tasks; the configuration-only page task owns that page's call to action.
- Open at implementation: stop and ask if the e2e plan has not landed in the expected shape; the plugins page and README intro are shared with the docker-plugin spec — adapt to what landed, ask only on direct contradiction.

### Project-wide rules

- Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.
- configonly stays its own module (`go 1.25.0`, `replace` to `../..` and `../prettylog`); `go mod tidy` there drops `kr/pretty`.
- The smoke test builds and runs the binary with `os/exec`; the example imports no `internal/` package.
- The e2e plan's `TestConfigOnlyExampleTestsPass` runs the example's tests; no new e2e test.
- README ownership: e2e plan owns "Running them"; this plan owns the examples intro count ("two programs"), the configuration-only section, and deleting the Application configuration section; the plugin section belongs to the docker-plugin spec.
- Website ownership: this plan owns configuration-only.mdx, deleting application-config.mdx, the Nav entry, index.mdx example links and calls to action, the state-masking call to action, and sensitive-values.mdx; on plugins.mdx only the application-config button.
- This plan owns the whole state-masking call to action and points it only at the configuration example, the one example that still encrypts its state (settled decision).
- Old changelog sections are never rewritten.

### Manual checks

- Copy configonly out of the repo, point it at a published xcl (and prettylog), confirm it builds and its tests pass.
- Review the program for test-only seams.
- Confirm the files under `config/` are byte-identical to before.
- Run the example: one route line per ingress path with host, path, service, deployment, container and port.
- Compare every website snippet credited to a configuration-example file, and the quoted output, against current source and a real run.
- Search both repos: no application config example directory, page, link or mention remains (excluding `.spektacular`, `node_modules`, `dist`, old changelog sections), and the site builds with no broken link.

### Changelog input

- The configuration example (`example/configonly`) is now a real application: it loads its configuration into its own types and prints one line per ingress route (host, path, service, deployment, container and port).
- The example shows how to test configuration-driven code: unit tests of the route logic against a separate test configuration, plus a smoke test that runs the built program.
- The application config example (`example/appconfig`) and its website page are removed; the configuration example covers what it showed, and website links now point there.
- Breaking changes: none.

## 20261006112023-f7a185dc-docker-plugin-example
### Approach

Spans xclconfig and xcl-website.

**Library risk first.** Plugin block types with no subtype mostly work already (registry picks the label count, parser takes one label, address is `type.name`). Milestone 1 confirms this with tests and fixes the gaps, all backwards compatible: a provider registered without a subtype loses its `provider=` log tag (`plugins/plugin.go:70`); "no registered type" errors print a trailing dot; nothing tests the path in-process or over gRPC.

**The example becomes a plugin project modelled on Jumppad.**
- `docker/`: a thin `Client` interface (~12 Docker SDK methods), `NewClient`, `Ping`, `Network` and `Container` types, providers holding the client as a field, a Mockery mock, and a plugin registering `docker "network"` and `docker "container"`.
- `cmd/docker-plugin/`: the external plugin binary.
- `template/`: the in-process plugin, registering `template` with no subtype and rendering with infinytum/raymond v2.0.5.
- `main.go`: split into `apply` and `report`, both called by `main` — no test-only seams.
- Config: a network, a container on it, and a template whose variables read the container's `ip_address` and the network's name.

Real-Docker tests skip when `docker.Ping` fails (not `-short` or build tags).

Rejected: Jumppad's ~40-method client plus `ContainerTasks` (too big); the current test-shaped `run(...)` signature (spec forbids); copying a `requireDocker` helper into each test package (exported `Ping` serves both); registering `template` under `resource` (contradicts the spec's block forms).

### Milestones and tasks

**M1 — No-subtype plugin types**
- Support plugin block types with no subtype (in-process tests plus an external fixture binary proving the type and a repeated nested block cross gRPC)

**M2 — Docker and template plugins**
- Add the Docker client interface and its mock
- Build the Docker network provider
- Build the Docker container provider
- Serve the Docker plugin and test it against real Docker
- Build the template plugin

**M3 — The example as a real project**
- Rewrite the plugin example application and configuration (deletes the old postgres/redis/app/ingress code and the module)
- Add the wiring and smoke tests

**M4 — Docs match**
- Update xcl's docs for the new plugin example (README Plugins section, README module link at :1357, `docs/plugins.md`)
- Rewrite the website's plugin example page (xcl-website)
- Update the website pages that quote the plugin example: plugin-logging, events, index (xcl-website)

### Tasks a person must do

None — all tasks are agent tasks.

### Out of scope

- Jumppad-level Docker features: volumes, health checks, builds, ports, sidecars, registry auth, refresh and change detection.
- Docs drift the examples did not cause (issue #4).
- Modules and state encryption in the plugin example.
- The configuration-only example and prettylog (sibling specs).
- The e2e suite, the example module layout and the README "Running them" paragraph (e2e plan).
- `plugins/example` (PersonPlugin).
- An automated check that website snippets match the source.

### Drafting assumptions

- The rewritten example drops the `modules/db` module and the state-encryption key.
- `docker.Ping(ctx)` is exported and used by the program (clear error with no engine) and the tests (to skip).
- The README module link, the README Plugins section and `docs/plugins.md` are updated here, because this rewrite breaks them.
- A container's first network is attached at create through `NetworkingConfig`; further networks use `NetworkConnect`.
- Conventions kept: testing-and-mocking, code-style, dependencies, patterns-and-architecture, development-standards, shared-errors-package, never-modify-dependencies, project-structure (`/cmd`). Dropped: database, shared-test-helpers (examples cannot import it), test-state-from-real-apply, assert-ordering-on-graph-parents.
- Docker objects are named after the block name and labelled `created_by=xcl-example-plugin` and `xcl_id=<Meta.ID>`; computed fields `docker_id` and `ip_address`; container networks are a repeated `network { name, aliases }` block (list attribute as fallback).
- The template `source` is inline Handlebars text; `variables` is a `map[string]string`.
- The container image is `nginx:1.27-alpine`.
- Mockery runs through `go run github.com/vektra/mockery/v3@<pinned>` from a Makefile `generate` target.
- Website snippets are checked by hand (the site has no test harness).
- The library's no-subtype gRPC test uses its own fixture at `internal/test_fixtures/plugins/subtypeless`.

### Project-wide rules

- Each example is its own module: `example/plugin/go.mod` stays at `go 1.25.0` with `replace` to `../..` and `../prettylog`; if `go mod tidy` with the Docker SDK raises the go directive, the implementer stops and asks.
- Example-only dependencies stay in the example's module: Docker SDK v28.5.2+incompatible and go-connections v0.6.0 (Jumppad's pins), raymond v2.0.5 (xcl's pin). xcl's root `go.mod` never gains Docker.
- Examples import no `internal/` package; the smoke test uses `os/exec`; Docker availability comes from the example's own `docker.Ping`.
- Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.
- README: this plan owns the "### Plugins" section and the module link at :1357; "Running them" stays with the e2e plan.
- Website: this plan owns `examples/plugins.mdx` (except the application-config button) and the plugin-example passages in `plugin-logging.mdx`, `events.mdx` and `index.mdx:213-216,228`. It does not touch `state-masking.mdx` (settled decision).
- The e2e runner `TestPluginExampleTestsPass` is reused unchanged; Docker tests skip without Docker so the runner passes either way.
- Mockery config for an example lives in the example (`example/plugin/.mockery.yml`), same v3 shape as the root.
- Library: `plugins.RegisterResourceProvider` with an empty subtype becomes a supported, tested contract — written `<type> "<name>"`, addressed `<type>.<name>`, logged `provider=<type>`, named via `types.TypeKey` in errors.

### Manual checks

- Copy `example/plugin` out of the repo, point it at a published xcl and prettylog, confirm it builds and its tests pass, or skip where there is no Docker.
- On a machine with no Docker engine, the Docker-dependent tests report as skipped and the run passes.
- With Docker, running the example creates the labelled network and container, visible during the run and gone afterwards.
- Review that the example's program has no test-only function, parameter or return value.
- Every website snippet taken from the plugin example, and the README and `docs/plugins.md` passages, match the final source.

### Changelog input

- Library: plugins can provide block types with no subtype (`template "name" {}`), tested in-process and across the external-plugin boundary.
- Library fix: providers registered without a subtype are tagged `provider=<type>` in logs, and "no registered type" errors no longer end in a dangling separator.
- Example: `example/plugin` is rewritten as a self-contained module with an external Docker plugin (`docker.network`, `docker.container`) and an in-process template plugin, with docs and website pages updated to match.
- Breaking changes: none.

## 20261006112108-17623cda-encoder-syntax-highlighting
### Approach

A new top-level public `highlight` package runs after the encoder has formatted its text and lexes that final text with the HCL scanner already in xcl. Each token is labelled with the TextMate scope the xcl-vscode grammar gives it (`.xcl` suffix kept), and every byte goes to a one-method `Renderer` (`Render(scope, text string) string`); unlabelled text arrives with scope `""`. Text between tokens passes through untouched, so highlighting cannot change the text. `xcl.Highlight(renderer)` joins the existing encode options; without it encoding is unchanged.

The built-in terminal renderer is `highlight.NewANSIRenderer(WithTheme(io.Reader) | WithThemeFile(path))`: with no theme it uses only the 16 standard terminal colours; with a VS Code theme (JSON or JSONC) it writes 24-bit colours plus bold, italic, underline and strikethrough. Matching follows TextMate (prefix selectors, most specific wins, colour and font style resolved separately). A bad theme fails the constructor with `xcl.ErrInvalidTheme`.

Rejected: a regex highlighter (cannot handle multi-line or nested tokens); a port of the TextMate engine (Go regex has no lookbehind — needs cgo or a new dependency); taking tokens before formatting (formatting moves byte positions).

### Milestones and tasks

**M1 — configuration text can be labelled and rendered in any format**
- Add the highlight package with the tokeniser and renderer interface

**M2 — terminal colour, by default or from an editor theme**
- Add the invalid-theme error
- Read VS Code colour themes and match scopes against them
- Add the ANSI terminal renderer

**M3 — the encoder highlights on request, and the logging example uses it**
- Add the Highlight encode option
- Switch the logging example to library highlighting
- Document highlighting on the website encoding page (xcl-website)

### Tasks a person must do

None — all tasks are agent tasks.

### Out of scope

- A built-in HTML renderer.
- Sharing a tokeniser with `xcl fmt` (#5).
- Terminal detection.
- 256-colour output or reducing a theme's colours for limited terminals.
- Full scope-stack matching.
- Theme `include` chains and `.tmTheme` files.
- Bundled named themes.
- Changes to the xcl-vscode grammar.
- Highlighting in examples rewritten by the sibling specs (they are expected to use `xcl.Highlight`).

### Drafting assumptions

- The package lives at the top level (`highlight/`), not under `/pkg`, following `mask`.
- Text preservation is tested inside the package (parity fixture plus fuzz test); checking every `.xcl` file in examples and fixtures is a manual check, since the testing convention forbids globbing other directories.
- The reference scope set is the xcl-vscode grammar in its working tree (which has uncommitted edits).
- A theme selector with spaces is matched on its last element; exclusion selectors are ignored.
- Theme files may contain comments and trailing commas; a theme using `include` or pointing `tokenColors` at a file is rejected as invalid.
- Theme entries with no scope, and editor colours, are ignored, so unmatched tokens keep the terminal default colour.
- A multi-line token is styled one line at a time, newlines outside the escape codes, keeping prettylog's indenting safe.
- The default theme carries over prettylog's current palette.
- `ErrInvalidTheme` and `InvalidThemeError` go in the `errors` package and are re-exported from `xcl`.
- A shared `testutil.StripANSI` helper is added to `internal/testutil`; prettylog keeps its own copy as a separate module.
- prettylog keeps deciding colour through lipgloss; its tests force colour with `CLICOLOR_FORCE=1`.
- The website task depends on the API tasks.
- Open at implementation: which module prettylog is in when its task runs (stop and ask if it pins a published xcl without `highlight`); stop and ask if a grammar construct cannot be labelled exactly from scanner tokens.

### Project-wide rules

- Changelog: this spec does not edit `CHANGELOG.md`; the epic's changelog entry is written once after all its specs are implemented.
- Errors: shared sentinels and detail types live in the `errors` package, use pointer receivers, and are re-exported from `xcl` (`config.go`).
- Public package placement: new public packages go at the top level, as `mask` did.
- Shared test helpers go in `internal/testutil`; self-contained examples keep local copies.
- Dependency boundary: the library gains no charmbracelet or other module, enforced by review (no test inspects the module graph).
- This plan edits `example/prettylog/` (deletes `highlight.go` and `highlight_test.go`, changes `prettylog.go` and its tests), which the e2e suite spec also edits.
- Website: adds a Highlighting section to `xcl-website:src/pages/configuration-text.mdx`.

### Manual checks

- Highlight every `.xcl` file in the examples and test fixtures; stripping the colour gives back the plain text byte for byte.
- In VS Code, "Inspect Editor Tokens and Scopes" on the parity fixture: scopes match the labels `highlight` gives.
- Run the logging example in a real terminal with the default theme and with a dark editor theme: configuration coloured as in the editor, no colour leaks past it.
- Review the new Highlighting section on the website's encoding page in a local build.
- Success metric: no example prints configuration with its own highlighter, including examples rewritten by the sibling specs.
- Review the change and confirm the library gains no charmbracelet or other new module dependency.

### Changelog input

- New public `highlight` package: `Renderer`, `RendererFunc`, `Text` and `Scope*` constants matching the xcl-vscode grammar scopes.
- New `xcl.Highlight(renderer)` encode option for `EncodeEntity` and `EncodeSavedEntity`; without it, encode output is unchanged.
- New `highlight.NewANSIRenderer` with `WithTheme` and `WithThemeFile`, plus `xcl.ErrInvalidTheme` / `InvalidThemeError`.
- The prettylog example now uses the library's highlighting instead of its own regex highlighter.
- Breaking changes: none.
