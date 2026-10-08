The change is built on the design `replacement-and-dependency-changes.md` (source `design`), which settles the shape of the provider answer, the dependency list and the order of work. Spread across the work's two repositories, it looks like this.

**The contract (xclconfig, `plugins/`).** The public `plugins` package gains:
- `type Change int` with `NoChange`, `Update` and `Replace`, and a `String` method;
- `DependencyChange{Address, Change}`;
- the new `ResourceProvider[T].Changed(ctx, old, new T, dependencies []DependencyChange) (Change, error)`.

Every layer that carries the call passes `dependencies` in and `Change` out: `ProviderAdapter`, `TypedProviderAdapter`, the `Plugin` and `PluginHost` interfaces, `PluginBase`, the direct host and `sourcedAdapter`, the gRPC wrapper, server and resource adapter, and `plugins/plugin.proto`. The proto gains a `Change` enum, a `DependencyChange` message and `repeated DependencyChange dependencies = 5` on `ChangedRequest`. `ChangedResponse`'s `bool changed = 1` becomes `Change change = 3`, with field 1 reserved.

`DefaultChanged` keeps its comparison and now returns `Update` or `NoChange`, ignoring dependencies. Because nearly every provider in the repository embeds it, the signature change moves them all at once. The types live in `plugins` rather than reusing `diff.Action`, because the provider vocabulary has no create or delete and `plugins` must not depend on the renderer (see `research.md#alternatives-considered-and-rejected`). Breaking the interface and the protocol is allowed by the spec, so there is no compatibility shim.

**Apply becomes decide-then-act, and the decide pass is the existing diff walk (xclconfig, `internal/parser`).**
- **Decide.** `Parser.Diff` already walks the graph without acting. It reads and runs `Changed` through the shared `refresh`, and marks pending entities so their computed values become unknown to dependents. It is promoted into the single decide pass that both `Parser.Diff` and `Parser.Apply` run. This is what keeps a plan from disagreeing with the apply that follows.
- **The decision record.** The diff recorder grows into a per-entity decision record holding the outcome, the dependency changes it was told, the reason, and the decide-pass read copy. Because the DAG walk finishes every parent before its children, a resource's `dependencies` is built from its parents' recorded decisions. It contains only `Update` and `Replace` entries.
- **Which resources count as dependencies.** They are resolved from `Meta.Links`, looking through outputs, variables, modules and registered config-only types to the provider-backed resources behind them, as the user decided.
- **Outcomes without a provider call.** A resource not in state is a create. One saved `failed` or `destroy_failed` is a replace, as today.
- **Unknown inputs.** A resource whose configuration holds unknowns (its dependency will be created, updated or replaced) is still read and asked `Changed`, with each unknown replaced by its saved value. Its outcome is never lower than `Update`, because its inputs will change; the provider may still raise it to `Replace`. Without this floor, a template whose input address will change could answer "unchanged" and never re-render.
- **Decide failures.** If any Read or `Changed` fails, the apply fails before anything is destroyed, created or updated, and the previous state is returned untouched.
- **Act, step 1: destroy.** The existing `destroyer` handles everything being replaced together with everything removed, building its reverse graph from the saved links of those targets only. That gives dependents-first destroys and per-resource state saves. A destroy failure marks the resource `destroy_failed` and stops the apply, just as a failed removal does today.
- **Act, step 2: create and update.** The ordinary apply walk then runs in dependency order, with real values. The lifecycle no longer reads or compares; it follows the decision:
  - create for new and replaced resources;
  - `Update` for updated ones, with the fresh configuration and the computed values from the decide pass's Read;
  - for unchanged ones, the decide-pass copy is kept with its status.
- **What goes.** The in-walk `rebuild` disappears. Failed resources follow the same destroy-then-create road as provider-decided replacements, which is what the spec's "reuse the existing replacement path" asks for. A create failure saves `failed`, so the next apply replaces the resource again.

**Plans show the reason (xclconfig, `diff/`).** `diff.Resource` gains a reason for a replacement:
- the last apply failed;
- the provider decided;
- a dependency is replaced, with the replaced dependency addresses listed.

`Render`'s comment line above the header names the cause, for example `# docker.container.web will be replaced because docker.network.app is replaced`. This keeps the existing layout rather than the trailing comment in the design sketch; the user chose it. Unknown values keep rendering as `(known after apply)` through the existing pending/unknown hooks, which already cover updated as well as replaced dependencies.

**Plugins, example and docs.**
- **Example plugin (xclconfig, `example/plugin`).**
  - The Docker network answers `Replace` when `subnet` changes.
  - The container answers `Replace` for changes to image, command, environment or networks, and whenever a dependency is replaced.
  - The template answers `Update` for source and variables, and `Replace` for destination.
  - `alt.xcl` moves to `example/plugin/config-subnet/` so `./config` applies on its own again.
- **Person plugin.** It answers `Replace` for `first_name`/`last_name`, which its ID derives from. It is the fixture run both in-process and external to prove the two behave the same.
- **Test plugin.** `TestPlugin` records the dependencies it is told and can be set to answer any `Change`.
- **Documentation (xclconfig `docs/`, `README.md`, `CHANGELOG.md`; xcl-website `src/pages/`).** It explains unchanged, update and replace for plugin authors, on a new website guide page linked from the Guides nav. The plugin example and diff pages show a `-/+` replacement with its reason.

**Delivery order.** The work is sequenced so the repository builds at every step:
1. Change the contract everywhere, with the core mapping `Replace` onto the current rebuild and passing no dependencies yet.
2. Introduce the decide/act split and dependency lists.
3. Add the reason and rendering.
4. Migrate the example plugins.
5. Write the documentation.

Rejected directions are recorded in `research.md#alternatives-considered-and-rejected`:
- extending the in-walk `rebuild`, which destroys in create order;
- a second decision path, which could disagree with the plan;
- an acting phase that runs as serial lists instead of the DAG walker;
- tag-driven replacement;
- reusing `diff.Action`.

**Conventions that drive specific choices.**
- Tests use testify `require`, one behaviour per function, no table-driven tests, and sit next to the code they test.
- Saved state for tests comes from real applies with `TestPlugin`.
- Ordering is asserted on graph parents, or on call order only for linked resources (the network and its container).
- Mocks are regenerated with Mockery.
- The examples use only public packages.
