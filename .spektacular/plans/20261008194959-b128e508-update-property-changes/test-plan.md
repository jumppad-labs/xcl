---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Test plan: 20261008194959-b128e508-update-property-changes

Every success metric in the plan has an automated behavioural test:
- the container updates in place using only what it is told (strict-mock unit tests in `example/plugin/plugins/docker/resources/container_test.go`);
- applies rebuild only what needs it, and real Docker state matches with a clean next plan (`example/plugin/scenarios_test.go`, `main_test.go`, `plan_test.go`);
- the plugin's settings match the plan's (`internal/parser/update_changes_test.go`, `e2e/plugin_changes_test.go`).

This plan covers the two manual reviews the plan lists, and one check that could not be automated.

## 1. Walk through the plugin example against a local Docker engine

**What:** the Makefile walkthroughs behave as documented, and the plan and status output reads correctly to a person.

**Setup:**
- A running Docker engine (or podman with the Docker socket).
- No leftover `app` or `backend` network and no `web` container, checked with `docker network ls` and `docker ps -a --filter label=created_by=xcl-example-plugin`.
- Run from `example/plugin` in the xclconfig repo.

**How:** run each target in turn. Each one builds, applies `./config`, plans and applies the variant, prints status, then destroys everything. Before each apply of the variant, note the container ID with `docker inspect -f '{{.Id}}' web`, by pausing between steps or running the steps by hand from the Makefile.

| Target | Plan must show | Summary line | After apply |
|---|---|---|---|
| `make replace` | `# docker.container.web will be updated because docker.network.app is replaced`, `~ docker "container" "web" {}`, and `-/+ docker "network" "app"` with `~ subnet = "10.42.0.0/24" -> "10.42.0.0/23"` | `Diff: 0 to create, 2 to update, 1 to replace, 0 to delete, 1 unchanged.` | Same container ID. `docker inspect web` shows it on `app` with an address in 10.42.0.0/23. |
| `make swap` | `+ docker "network" "backend"`, `~ docker "container" "web"` with `~ network[0].name = "app" -> "backend"`, and no `-/+` for web | `Diff: 1 to create, 2 to update, 0 to replace, 0 to delete, 2 unchanged.` | Same container ID, on `backend`, not on `app`. |
| `make rebuild-init` | `# docker.container.web will be replaced because template.init is replaced` and `-/+ docker "container" "web"` | `Diff: 0 to create, 1 to update, 2 to replace, 0 to delete, 1 unchanged.` | New container ID. The mount source ends in `init-v2.sh`. |
| `make remove-network` | `- docker "network" "app" {}`, and `~ docker "container" "web"` with `- network[0] = …` | `Diff: 0 to create, 3 to update, 0 to replace, 1 to delete, 0 unchanged.` | Same container ID, running, with no networks. `status` shows web with no address, and `build/rendered/welcome.txt` shows no stale `10.42.x` address. |

**Expected result:** every row matches, and `status` lists the four resources (five for swap) as a readable tree. The comment line above each changed resource explains why it changes in plain words; none says "changed outside xcl". After each target, `docker ps -a --filter label=created_by=xcl-example-plugin` is empty and no `app` or `backend` network remains.

**Who / when:** a maintainer, before merging the spec branch.

## 2. The init script runs when the container starts

**What:** the rendered init script is not only mounted but run by nginx's entrypoint. The example's Docker client has no exec or logs call, so the automated tests only check that the script is executable and mounted at `/docker-entrypoint.d/90-xcl-init.sh`.

**How:** from `example/plugin`, run `make build && ./build/xcl-docker apply ./config`, then:
- `docker logs web 2>&1 | grep 90-xcl-init.sh`
- `docker exec web cat /usr/share/nginx/html/init.txt`

Finish with `./build/xcl-docker destroy`.

**Expected result:** the logs show the entrypoint launching `/docker-entrypoint.d/90-xcl-init.sh`, not "Ignoring … not executable", and `init.txt` contains `web joined the app network`.

**Who / when:** a maintainer, before merging the spec branch, together with review 1.

## 3. Documentation review and website build

**What:** the guides and website pages are accurate to the new signatures and the example code.

**Where (xclconfig):**
- `docs/plugin-developer-guide.md`: the contract block, "The changed settings", "Overriding `Changed`" and the `Update` section.
- `docs/plugins.md`: signatures and the proto excerpt.
- `docs/parser-lifecycle.md`: the decide and act trees.
- `docs/README.md`.
- `README.md`: the plugin example section.
- `plugins/example/README.md`.
- `CHANGELOG.md`: the top entry.

**Where (xcl-website):**
- `src/pages/replacement.mdx`
- `src/pages/examples/plugins.mdx`
- `src/pages/diff.mdx`
- `src/pages/index.mdx`

**Look for:**
- Every `Changed`/`Update` signature matches `plugins/provider.go`.
- The `PropertyChange` struct matches `entity/property_change.go`.
- The proto excerpts match `plugins/plugin.proto`.
- The container `Changed`/`Update`, the network `Destroy`, the template `mode` and the `template "init"` excerpts match `example/plugin`.
- No page says the Docker example's `Update` does nothing, or that the container is replaced on a network change.
- `diff.mdx` describes the `dependencies` field and the "will be updated because … is replaced" line.
- The CHANGELOG's Breaking list covers the signatures, the protocol, the `diff.Path` aliases, computed values now being cleared, and the example's behaviour change.

**How (build):** in the xcl-website repo, run `npm install && npm run build && make check`.

**Expected result:** no mismatches are found, the build completes, and `astro check` reports 0 errors and 0 warnings.

**Who / when:** a maintainer, before merging; the website check is repeated before the site is deployed.
