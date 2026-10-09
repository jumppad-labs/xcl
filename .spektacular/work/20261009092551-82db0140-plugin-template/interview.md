# Interview: plugin-template

## From the source (GitHub issue #8)
- Scaffold a new xcl plugin, most likely as a GitHub template repository, so authors start from a working, idiomatic layout instead of copying the examples by hand.
- One resource type and its provider, buildable and runnable both in-process and as an external plugin.
- An explicit `Changed` on the provider, rather than relying on an embedded `DefaultChanged` alone, so the change rules are visible and easy to edit. The issue's sample predates the `changes` parameter, so the template must use the current contract.
- Tests in the project's style (testify `require`, no table-driven tests), including the `Changed` rules.
- A Makefile and README showing how to build, test and register the plugin.
- Delivered as a GitHub template (`gh repo create --template` / "Use this template").
- It tracks the current plugin contract, including the changed properties passed to `Update`.

## From the user (this conversation)
- **Motivation:**
  - The example layout should separate entities from providers.
  - A "standard plugin approach" should be defined that forms the basis for the template.
  - Style: public methods kept together at the top of a file.
- **Delivery:**
  - A new GitHub template repository (e.g. `jumppad-labs/xcl-plugin-template`) with placeholder names, used through "Use this template" / `gh repo create --template`.
  - It's a new repo, so it must be created and registered with the project.
- **One template** covers both in-process and external plugins: the same layout, with `main.go` serving it externally and the README showing in-process registration.
- **The Docker plugin example is rebuilt to the standard layout in this spec**, so the example and the template stay in step:
  - entities split from providers;
  - file ordering applied;
  - the host application imports only the entity types, not the providers or the Docker SDK through them.
- **Entity package name:** `entities`.
- **Sample resource:** runs with no external service. It sits behind a narrow client interface with a simple local implementation, so the template builds, tests and applies out of the box while still showing the client-interface-plus-mock pattern.
- **Tests the template carries:**
  - strict-mock provider unit tests, including the `Changed` rules and `Update` working only from what it is told;
  - an apply end-to-end test (apply, a plan with no changes, destroy) through the sample configuration;
  - a CI workflow (GitHub Actions) that builds, vets and tests.
  - Not the in-process vs external parity test.
- **Website:** a page on starting a plugin from the template and the standard layout, and the plugin example page updated to the new layout and paths.

## The standard layout (discussed; detail belongs in Technical Approach, possibly a design doc)
- `main.go` (external serving), `plugin.go` (`Plugin.Init` registers providers)
- `entities/<resource>.go`: structs and tags only, no SDK imports
- `providers/<resource>.go` plus tests; shared unexported helpers in their own file
- `client/` (narrow interface over the backend, plus Mockery mocks), `.mockery.yml`, `Makefile`, `go.mod`, `README.md`
- Provider file order: type, interface assertion and constructor, then the exported methods in lifecycle order (Init, Create, Read, Changed, Update, Destroy, Functions), then unexported helpers.
- Explicit `Changed`: replace for settings that can't change in place (matched with `change.Within`) or a replaced dependency it is built on; update otherwise; then `DefaultChanged`.
- `Update` works only from the `changes` and `dependencies` it is told.

## Repos affected
- **xclconfig:** the Docker example rebuild; the docs and README that quote example paths (plugin developer guide, README plugin example section); docs pointing authors to the template.
- **xcl-website:** a new template/standard-layout page and the updated plugin example page.
- **New repo, the template:** must be created and registered with Spektacular.
- **xcl-vscode:** none.

## Ruled out
- Two separate templates (in-process vs external).
- A generator (`gonew`/`go run`) in place of a template repo.
- The parity e2e test in the template.
- Changes to the plugin contract itself.

## Added after the interview
- The template repo is registered with the Spektacular project, so later changes to the plugin contract are planned and made across it too ("we need to keep this template up to date so we should add the repo to the spektacular setup").
