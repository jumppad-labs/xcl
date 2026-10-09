---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Plugins are told what changed

## What was built

When xcl asks a provider whether a resource changed, or tells it to update a resource in place, it now passes the provider the list of settings that changed. Each setting comes with its location, its previous value and its new value. `Update` is also given the list of dependencies that the same apply updates or replaces, the same list `Changed` was given while deciding.

- **The vocabulary.** The public `entity` package gained `PropertyChange{Path, Before, After, Unknown, Sensitive}`. The structured `Path` type (with `Step` and `StepKind`) moved there from `diff`, which now aliases it. `Path` gained `Equal` and `Within`, and `PropertyChange` gained `At` and `Within`, so a provider matches a change with `change.Within(entity.Path{}.Attribute("network"))` rather than parsing text. Values are plain JSON values.
- **Sensitive values.** A sensitive change holds its real values, but `String`, `Format`, `LogValue` and `MarshalJSON` show `(sensitive)`.
- **The contract (breaking).**
  - `ResourceProvider.Changed(ctx, old, new, changes, dependencies)` and `Update(ctx, resource, changes, dependencies)`.
  - The same lists pass through every adapter, host and gRPC layer.
  - The protocol gained `StepKind`, `PathStep`, `PropertyChange`, `ChangedRequest.changes` and `UpdateRequest.changes`/`dependencies`. Values travel as raw JSON bytes, so external plugins are told exactly what built-in ones are; an end-to-end parity test proves it.
- **One comparison.**
  - **When deciding,** the core runs the plan's own comparison of the saved copy against the configured copy before `Changed`. It shows the result two ways: the plan's view, with sensitive values hidden unless revealed, and the plugin's view, with real, JSON-normalised values. The plan and the plugin therefore always list the same settings. A value only known once the apply runs is marked `Unknown`.
  - **When acting,** the comparison runs again with real values, including values produced earlier in the same apply. Settings that were unknown when deciding stay listed with their real values.
  - Sensitive values reach plugins but never plans, events or logs.
- **The Docker plugin example now updates in place.**
  - The container's `Update` hot swaps its networks using only what it is told. It rebuilds the previous attachments from each change's previous value, disconnects and connects the difference, reconnects networks that were rebuilt, and reads its new address.
  - The network's `Destroy` force-detaches attached containers. A subnet change therefore replaces the network and keeps the container.
  - The container gained an `init_script`, rendered by a second template with a new `mode` setting. Moving that template rebuilds the container; editing its content leaves the container alone.
  - New config variants and Makefile walkthroughs cover a network swap, an init-script rebuild, a content edit, a network removal and a dangling reference, each backed by real-Docker scenario tests.
- **Documentation.** The guides, README, CHANGELOG and documentation site describe the new contract with the hot swap and the init-script rebuild as worked examples.

## Why it matters

Before this change, a provider asked to update a resource in place could not tell what had changed or what the values used to be. A plugin that wanted to move a container to a different network had to make extra calls to rediscover state that xcl already had, so it often fell back to rebuilding. Plugin authors can now write precise in-place updates and choose to rebuild only when it is really needed. People applying configuration get fewer needless rebuilds: in the example, network changes never rebuild the container.

## Deviations from the plan

- **Plans name the dependencies behind an update (user-approved).**
  - With the container updated only because its network was replaced, the plan rendered the misleading `changed outside xcl and will be updated`.
  - `diff.Resource` gained an additive `Dependencies` field (JSON `dependencies`).
  - An update with no changes of its own now renders `will be updated because docker.network.app is replaced`.
  - This adds a field to the plan's JSON, which the plan had said would stay unchanged.
- **Providers can clear computed values (user-approved core fix).**
  - A provider's result was decoded onto a resource that still held the computed values carried over from the last apply, so a value a provider cleared (an `omitempty` address) was kept. The removal scenario left a stale address in state and in the rendered template.
  - The core now clears computed fields before decoding any provider result.
- **The act-time comparison keeps unknown paths.** It runs with the decide-time unknown paths, and `resolveUnknown` fills them with real values. This replaces the planned "append missing unknown paths" step, and keeps the plan's order with no re-sorting.
- **Container `Changed` refined.** A non-network dependency that is only updated leaves the container alone, which meets the criterion that a content edit leaves the container alone.
- **Recorder fixture naming.** It is registered as `resource "recorder"`, following the codebase convention, rather than type `recorder`/subtype `item`.
- **Test updates moved earlier.** The example's subnet and count-dependent test updates, planned for the scenarios task, were made earlier to keep the tree green.
- **Website scope.** `diff.mdx` and `index.mdx` were updated as well as the two planned pages.
- **One acceptance criterion left for manual check.** "The container runs the init script at start" is confirmed only up to the executable mount, because the example's Docker client cannot observe the container running it. It is in the manual test plan.
- **Known issue (pre-existing, not fixed).** `HCL_VAR_` variables are ignored through `xcl.Config`, because the `ParserOptions` built there never set `VariableEnvPrefix`.
