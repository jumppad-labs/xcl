---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Test plan: 20261008132354-4538504f-replacement-deps

Every success metric has an automated behavioural test:
- **No drift after the subnet apply:** the Docker-gated `example/plugin` tests `TestSubnetChangeReplacesTheNetworkWithTheNewRange` and `TestPlanAfterSubnetChangeReportsNoChanges`.
- **A replace answer for every setting that can't change in place:** the provider `Changed` unit tests.
- **Plan equals apply:** the exact comparisons in `e2e/diff_test.go` and the parser replacement tests.

The Docker-gated tests skip on a machine without a Docker engine, so the walkthrough in M1 covers the first metric by hand. The plan also lists two further manual reviews (M2 and M3). All four are below.

## M1. Plugin example walkthrough against real Docker (no-drift metric and the documented output)

- **What to check:** applying the address-range configuration rebuilds the network on the new range with a new container and a re-rendered template. The next plan reports no changes, and the output the program prints matches the output shown in the README and on the website.
- **Who / when:** the release owner, before merging this spec, on a machine with a Docker (or Podman, via `DOCKER_HOST`) engine.
- **Setup:** first make sure no Docker network named `app` and no container named `web` already exist. Check with `docker network ls` and `docker ps -a`; remove leftovers only if they are yours. Then:
  ```
  cd example/plugin
  make build
  ```
- **Steps:**
  1. `build/xcl-docker apply ./config`. It succeeds and creates `docker.network.app`, `docker.container.web` and `template.welcome`. `./config` applies on its own.
  2. `docker network inspect app --format '{{(index .IPAM.Config 0).Subnet}}'` prints `10.42.0.0/24`. Note the container ID from `docker inspect web --format '{{.Id}}'`.
  3. `build/xcl-docker plan ./config-subnet` prints exactly:
     ```
       # docker.container.web will be replaced because docker.network.app is replaced
     -/+ docker "container" "web" {}

       # docker.network.app will be replaced, it cannot be updated in place
     -/+ docker "network" "app" {
           ~ subnet = "10.42.0.0/24" -> "10.42.0.0/23"
         }

       # template.welcome will be updated
       ~ template "welcome" {
           ~ variables["address"] = "10.42.0.2" -> (known after apply)
         }

     Diff: 0 to create, 1 to update, 2 to replace, 0 to delete, 0 unchanged.
     ```
     The IP address may differ.
  4. `build/xcl-docker apply ./config-subnet`. In the log, every `read`/`changed` line comes before the first `destroy`. The order after that is: `destroy docker.container.web`, `destroy docker.network.app`, `create docker.network.app`, `create docker.container.web`, `update template.welcome`.
  5. `docker network inspect app --format '{{(index .IPAM.Config 0).Subnet}}'` prints `10.42.0.0/23`. `docker inspect web --format '{{.Id}}'` differs from step 2, and the container is attached to `app`.
  6. `cat build/rendered/welcome.txt` contains the new container's address, the `ip_address` shown by `build/xcl-docker status`.
  7. `build/xcl-docker plan ./config-subnet` prints `Diff: no changes, 3 unchanged.`
  8. Compare steps 3 and 4 with the README's plugin example section and with the website's plugin example page (`/examples/plugins/`). The `-/+` lines, reasons and summary must match, and so must the event order.
  9. Clean up with `build/xcl-docker destroy` and `make clean`. Afterwards no `app` network or `web` container remains.

  `make replace` runs steps 1, 3, 4, a status and the destroy in one go.
- **Pass:** steps 1 to 7 all hold as stated (subnet `10.42.0.0/23`, a new container ID, the template holding the new address, "no changes"), and step 8 finds no mismatch between the docs and the real output.

## M2. Website review: guide, diff page and plugin example page

- **What to check:**
  - The new guide "Unchanged, update or replace" (`src/pages/replacement.mdx`) and the updated `src/pages/diff.mdx` and `src/pages/examples/plugins.mdx` are accurate against the shipped behaviour.
  - The guide is reachable from the Guides menu.
  - The site builds and passes `astro check`.
- **Who / when:** a maintainer, before merging the xcl-website changes.
- **How:**
  ```
  cd <xcl-website worktree>
  npm ci            # only if node_modules is missing
  npm run build     # expect "11 page(s) built", including /replacement/
  make check        # expect 0 errors, 0 warnings
  npm run dev       # open http://localhost:4321/
  ```
  - Open the Guides menu and follow "Unchanged, update or replace" to `/replacement/`.
  - Check the three answers and what the apply does with each.
  - Check the dependency rules: direct only; only updated or replaced dependencies, never unchanged or newly created ones; outputs, variables, modules and config-only types looked through.
  - Check decide-then-act, and that destroys run before creates, dependents first.
  - Check that the Docker container override shown matches `example/plugin/plugins/docker/resources/container.go`.
  - On `/diff/`, check the three replacement reasons, the rendered sample naming the dependency, and the JSON `reason` / `replaced_dependencies`.
  - On `/examples/plugins/`, check the `config-subnet` walkthrough, and that no page shows a container image change as an in-place update.
- **Pass:** the build and check succeed, the guide is reachable from the Guides menu, and every statement and code excerpt matches the code and the M1 output.

## M3. Core guides, README and CHANGELOG review

- **What to check:** the project's own documentation describes the new contract and behaviour correctly.
- **Who / when:** a maintainer, before merging this spec.
- **Where:** in the xclconfig worktree:
  - `docs/plugin-developer-guide.md`: the `Changed` contract, the dependency list, decide then act, the destroy phase, statuses of failed replacements, and event order.
  - `docs/plugins.md`: the `ResourceProvider`, `ProviderAdapter` and `PluginHost` signatures and the proto messages.
  - `docs/parser-lifecycle.md`: the apply steps, decision tree and event sequence.
  - `docs/state.md`: status meanings, and that a failed decision saves nothing.
  - `docs/README.md` and `docs/overview.md`.
  - `README.md`: the plugin example, `config-subnet` and `make replace`.
  - `CHANGELOG.md`: the top entry `## 20261008132354-4538504f-replacement-deps`.
- **What to look for:**
  - Every signature matches the code: `entity.Change` and `[]entity.DependencyChange`, with no `bool` result left.
  - The CHANGELOG's **Breaking:** list names every breaking change: the Go `Changed` and adapter/host signatures, the protocol change (external plugins must be rebuilt), no saved-state compatibility, decide failures saving nothing and not marking resources failed, dependents of computed values now updated, and `config/alt.xcl` moving to `config-subnet/main.xcl`.
  - The decide-then-act description matches the behaviour seen in M1.
  - The existing `#L<line>` source links in `docs/parser-lifecycle.md` and `docs/state.md` still point at the right lines, or are corrected.
- **Pass:** no statement contradicts the code or the M1 output, and the breaking-changes list is complete.
