# Working context: 20261009092551-82db0140-plugin-template

Source: GitHub issue #8, "Scaffold for new plugins (GitHub template) with an explicit Changed" (https://github.com/jumppad-labs/xcl/issues/8). The issue predates the update-property-changes spec: its `Changed` sample lacks the `changes` parameter, and the template must track the current contract.

## Problem / motivation
- Arose while reviewing the Docker plugin example (`example/plugin`) after the update-property-changes spec merged.
- The user wanted a better layout. In their words: "entities to store each entity and providers should be separate, however maybe we just have file names rather than packages".
- The user then widened the goal: "we should look at defining a standard plugin approach that would form the basis for a git template".
- Style preference, user's words: "I like to keep public methods together at the top of a file."

## Findings that shaped the direction
- The host app (`xcl-docker`) imports `plugins/docker/resources` only for the `Network`/`Container` types (`status.go`, `xcl.FindByType`). That links the Docker providers and SDK into the host binary (22 Docker packages), even though the Docker plugin runs as a separate process.
- A separate entities package (structs only, no SDK) makes that boundary real, and is the pattern a template should teach.
- `main.go` also imports `plugins/docker/client`, so the host binary keeps some Docker dependency regardless. Raised but not decided.
- Current provider files interleave helpers between exported methods (e.g. `container.go`), and the template provider puts `Changed` before `Init`.

## Draft standard layout (proposed by Claude, not yet confirmed)
- `main.go` (external only), `plugin.go`
- `entities/<resource>.go`: structs and tags, no SDK imports
- `providers/<resource>.go` and tests, plus shared helpers
- `client/` (narrow SDK interface plus Mockery mocks), `.mockery.yml`, `Makefile`, `go.mod`
- Provider file order: type, interface assertion and constructor; exported methods in lifecycle order (Init, Create, Read, Changed, Update, Destroy, Functions); then unexported helpers.
- Explicit `Changed` (replace via `change.Within`, update otherwise, then `DefaultChanged`). `Update` works only from the `changes`/`dependencies` it is told.
- Strict-mock unit tests plus real-backend tests that skip when no engine is available. In-process vs external chosen at the top level.
- Package-name worry: `entities` vs xcl's own `entity` package; `blocks` was offered as an alternative. Undecided.
- For a single-resource in-process plugin (template), files may be enough, without packages.

## Open questions raised (unanswered)
1. Real template repo with placeholder names, or a generator (`go run`/`gonew`)?
2. One template for both in-process and external plugins, or two?
3. Rebuild the Docker example to exactly this layout, so the example and the template can't drift apart?
4. Does the template carry the e2e in-process/external parity pattern, or only unit tests?
5. `entities` or `blocks`?
6. Update the docs (README, plugin developer guide, website plugin example page) that quote `plugins/docker/resources/...` paths?

## Decisions so far
- Spec name: plugin-template, with no epic.
- Knowledge offer pending (not yet answered): save the "public methods at top of file" convention via spek-knowledge.

## Interview answers (user)
- Delivery: a new GitHub template repo (it must be created and registered). One template covers in-process and external.
- The Docker example is rebuilt to the standard layout in this spec.
- Website: a template/layout page plus the plugin example page updated.
- Package name: `entities`. The sample resource needs no external service but keeps the client interface plus mock.
- Template tests: strict-mock unit tests, apply e2e, and CI workflow. No parity test.
- Overview confirmed.
- USER: "we need to keep this template up to date so we should add the repo to the spektacular setup". The template repo must be registered with the Spektacular project, so future specs that change the plugin contract also update it. Captured as a requirement.
- Requirements confirmed (13).
