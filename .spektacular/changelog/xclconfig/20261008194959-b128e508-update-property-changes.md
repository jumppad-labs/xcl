---
created_date: "2026-10-09"
document_status: draft
project: xclconfig
spec: 20261008194959-b128e508-update-property-changes
plan: 20261008194959-b128e508-update-property-changes
---

# Plugins are told what changed

Providers are now told exactly which settings of a resource changed, with each one's previous and new values, both when xcl asks whether the resource changed and when it updates the resource in place. An update is also told which of its dependencies the same apply updates or replaces. Sensitive values reach the provider for real but stay hidden in plans, events and logs, and plugins that run as separate programs are told exactly the same. The Docker plugin example uses this to move a running container between networks in place, rather than rebuilding it.

> Derived from project xcl (xclconfig), spec/plan 20261008194959-b128e508-update-property-changes. See the project-level record for the full feature.

## What changed in this repo

- **Contract (breaking):**
  - `ResourceProvider[T].Changed(ctx, old, new, changes []entity.PropertyChange, dependencies []entity.DependencyChange)` and `Update(ctx, resource, changes, dependencies)`.
  - The same lists pass through `ProviderAdapter`, `PluginHost`, `PluginEntityProvider`/`PluginBase`, the direct host and the gRPC wrapper and server.
  - `DefaultChanged` ignores both lists.
- **`entity`:**
  - New `PropertyChange`, with `At`/`Within` and self-masking `String`, `Format`, `LogValue` and `MarshalJSON`.
  - `Path`, `Step` and `StepKind` moved here from `diff`, with new `Equal`/`Within`. The `diff` types are now aliases.
- **Protocol (breaking):** `StepKind`, `PathStep` and `PropertyChange` messages; `ChangedRequest.changes = 6`; `UpdateRequest.changes = 4`, `dependencies = 5`. Values are carried as raw JSON bytes by the converters in `plugins/change_proto.go`. External plugins must be rebuilt.
- **Core (`internal/parser`):**
  - One real-valued comparison per pass, shown two ways (`change_views.go`: `planChanges`, `pluginChanges`, `resolveUnknown`).
  - `refresh` passes the changes to `Changed`.
  - The act pass recomputes them for `Update` and passes the recorded dependency list.
  - The decision record keeps the plan's view.
- **Plan:** `diff.Resource.Dependencies` (JSON `dependencies`). `Render` names them for an update with no changes of its own (`will be updated because … is replaced`).
- **Computed values:** a provider's result is now authoritative. Computed fields are cleared before a Read/Create/Update result is decoded (`clearComputed`), so a provider can clear a computed value.
- **Docker plugin example:**
  - `NetworkDisconnect` on the client.
  - The network `Destroy` force-detaches containers.
  - The container `Changed` replaces only for image, command, environment or init-script changes, or a replaced non-network dependency.
  - The container `Update` hot swaps networks from what it is told (`attachments.go`).
  - New `init_script` mount and template `mode`.
  - The configs gain `template "init"`; new variants are `config-swap`, `config-init`, `config-init-content`, `config-remove` and `config-dangling`.
  - Makefile targets `swap`, `rebuild-init` and `remove-network`.
  - Real-Docker `scenarios_test.go`.
- **Tests:**
  - Core tests of what `Changed` and `Update` are told.
  - A recording e2e fixture (`e2e/fixtures/recorder`) and an in-process vs external parity test.
  - Converter and gRPC tests.
- **Docs:** the plugin developer guide, `docs/plugins.md`, `docs/parser-lifecycle.md`, `docs/README.md`, the README, the person example README and the CHANGELOG.

## Why

Plugin authors needed to know what changed, and what it used to be, to write precise in-place updates without rediscovering state. Both the plugin contract and the core that computes the changes live in this repo, so the change lands here. The Docker example lives here too and proves the behaviour end to end.

## Deviations

- `diff.Resource.Dependencies` and the "will be updated because" plan line, added with the user's approval.
- Computed values are cleared before decoding provider results, a user-approved core fix.
- `resolveUnknown` replaces the planned append of unknown paths.
- Container `Changed` ignores dependencies that are only updated and are not networks.
- The recorder fixture is named `resource "recorder"`.
- Pre-existing and not fixed: `HCL_VAR_` variables are ignored through `xcl.Config`.
