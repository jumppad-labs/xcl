Tests follow the project's conventions throughout:
- testify `require`, one behaviour per test function, no table-driven tests;
- positive and negative cases in separate functions, with tests next to the code they test;
- saved state produced by real applies with the recording `TestPlugin` (a failing create gives `failed`, a failing destroy gives `destroy_failed`), never hand-written state files;
- ordering asserted on graph parents, with call order compared only between linked resources;
- no test that reads documentation, source or CI files.

The existing lifecycle, diff, removal and destroy suites are the regression guard. Each phase lands with them passing. Tests whose expectations the design deliberately changes are updated in the same task, with the reason given in the task: event interleaving across resources, a resource with unknown inputs now being read, and a decide-pass Read failure no longer marking the resource failed.

**Unit tests — contract layers (`plugins`).**
- `Change.String`.
- `DefaultChanged` answers `Update` or `NoChange` for each existing comparison case, and ignores dependencies.
- The typed adapter decodes both copies and passes dependencies through.
- The direct host passes dependencies and the answer through.
- The gRPC wrapper sends the dependency list and maps each proto `Change` back to the Go value, tested with a fake service client that captures the request.
- The gRPC server maps the plugin's answer and error into the response.

**Unit tests — providers.** Each in-repo provider with replace rules gets one test per rule, beside the provider:
- network `subnet` → Replace;
- container `image`, `command`, `environment` and `network` blocks → Replace each, and a replaced dependency → Replace;
- container with only an updated dependency → its own default answer;
- template `destination` → Replace, while `source` and `variables` → Update;
- person `first_name` and `last_name` → Replace, and other fields → Update.

There are also negative cases: an identical configuration gives NoChange, and a dependency that is only updated does not force a replace. These carry the "every setting it cannot change in place answers replace" success metric.

**Unit tests — decision record and dependency resolver (`internal/parser`).**
- The dependency list contains only Update and Replace dependencies.
- It lists direct references only, and looks through outputs, variables, modules and config-only types to the provider-backed resources behind them.
- The destroy target list is replaced plus removed.
- The diff result carries reason and replaced dependencies, sorted.

**Integration tests — parser apply and diff with `TestPlugin`.** These are the densest coverage, because the decide/act split carries the design. One test per acceptance criterion:
- A provider answering Replace is planned as replace and applied as destroy then create.
- An Update answer is planned and applied as an in-place update.
- A resource referencing one replaced and one updated dependency is told exactly those two with their outcomes, and is not told about an unchanged one.
- A dependent answering NoChange to a replaced dependency is neither updated nor replaced.
- With a linked network and container both replaced, the calls run container destroy, network destroy, network create, container create.
- A `Changed` error on one resource fails the apply with no create, update or destroy call, and the state is unchanged.
- A failing create of a replaced resource saves it as failed, and the next diff lists it as replace again.
- A failing destroy of a replaced resource saves `destroy_failed` and stops the apply.
- Every read and changed event precedes every destroy, create and update event.
- A resource with unknown inputs is read with saved values, and is planned and applied at least as an update.
- Failed-status replacement still works through the new destroy phase.

**Unit and integration tests — rendering (`diff`).** Each reason renders its phrase on the comment line with the `-/+` marker. The dependency reason names the dependency. The JSON form includes `reason` and `replaced_dependencies` only for replacements. Existing render tests keep their output except the replace phrase.

**Root-package tests.** `Config.Diff` and `Config.Apply` report and perform a provider-decided replacement end to end through public options, including the lifecycle event order.

**End-to-end tests (`e2e`, public packages only).**
- The plan-versus-apply helper becomes an exact comparison for the replacement scenarios.
- External and in-process providers are exercised with the same configuration change, and must produce the same diff and the same apply outcome. The person plugin from the SDK example runs it both ways, and the e2e fixtures cover the mixed graph.

**Example tests (`example/plugin`, Docker-gated as today).**
- `./config` applies alone and the existing tests pass.
- Applying `./config` then `./config-subnet` leaves Docker with the network on the new subnet, a new container ID attached to it, and the template rendered with the new container's address.
- The plan for the subnet configuration renders the network `-/+` and the container `-/+` "because docker.network.app is replaced", with the template updated.
- A plan after the apply reports no changes.

Tests that need no Docker (rendering, provider rules with the mocked client) run everywhere.

Deliberate gaps:
- There is no new test of the gRPC server in isolation beyond the mapping test, because the external-versus-in-process e2e test exercises the full protocol.
- The website and guides are not tested by code (convention).

**Success metrics**
- *After the subnet configuration is applied, the Docker network's address range matches the configuration and the next plan reports no changes* — **Behavioural test**: a Docker-gated example test applies `./config` then `./config-subnet`, inspects the real network's IPAM subnet and requires it to equal the configured value, then runs a plan of `./config-subnet` and requires "no changes". The test skips without Docker, so a run on a machine with Docker is also part of the manual walkthrough below.
- *For each plugin, every setting it cannot change in place has a test showing it answers "replace"* — **Behavioural test**: the per-provider unit tests above, one per setting: Docker network subnet; container image, command, environment, networks and replaced dependency; template destination; person first and last name. The e2e fixture providers have no in-place limits, which is recorded as a deliberate decision.
- *The plan shown before an apply lists exactly the resources the apply then destroys, creates or updates* — **Behavioural test**: the parser integration tests and the e2e plan-versus-apply comparison require, for create, update, replace, delete, dependent-replace and failed-replace scenarios, that the set of addresses per action in the diff equals the set derived from the apply's lifecycle events (destroy+create = replace), with nothing extra and nothing missing.

**Manual reviews**
- **Manual — captured in the implementation test plan**: run the plugin example's documented walkthrough against real Docker (`apply ./config`, `plan ./config-subnet`, `apply ./config-subnet`, `plan ./config-subnet`), and check that the output shown in the docs matches what the program prints, including the `-/+` lines and reason.
- **Manual — captured in the implementation test plan**: review the new website guide on unchanged/update/replace and the updated diff and plugin-example pages for accuracy against the shipped behaviour, check the page is reachable from the Guides menu, and check the site builds and `astro check` passes.
- **Manual — captured in the implementation test plan**: review the core guides (plugin developer guide, plugins, parser lifecycle, state), README and CHANGELOG entry for the new `Changed` signature, the breaking-change notes, and the decide-then-act description.
