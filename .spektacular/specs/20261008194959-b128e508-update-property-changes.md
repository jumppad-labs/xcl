---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Feature: 20261008194959-b128e508-update-property-changes

## Overview

When a plugin decides a resource can be changed in place, xcl today hands it only the new settings, so the plugin cannot tell what actually changed or what the values used to be. A plugin author who wants to hot swap a container's network, rather than rebuild the container, has to make extra calls to rediscover information xcl already has. This change tells plugins exactly which settings changed, with their old and new values, and which of the resources they depend on are being updated or replaced. Plugin authors can then write precise in-place updates and decide when a rebuild is really needed, and people applying configuration get fewer needless rebuilds.

## Requirements

- [x] **Updates are told what changed**
  When a resource is updated in place, its plugin is told every setting that changed, with each one's location in the resource, its previous value and its new value.
- [x] **Updates are told about changing dependencies**
  When a resource is updated in place, its plugin is also told which of the resources it depends on the same apply is updating or replacing, and which of the two each one is, exactly as it was told when deciding.
- [x] **Change decisions see what changed**
  When asked whether a resource changed, a plugin is also told which settings differ from the last apply, with their previous and new values, so it can decide between update and replace without comparing settings itself.
- [x] **Values not yet known are marked when deciding**
  When a changed setting's new value is only known once the apply runs, the plugin is told the setting is changing and that its new value is not yet known, rather than being given a made-up value.
- [x] **Changes hold real values at update time**
  The settings a plugin is told about when updating hold their real values, never a value still to be worked out, including values that come from resources created or changed earlier in the same apply.
- [x] **Sensitive settings are reported with their real values**
  A plugin is told the real previous and new values of a changed sensitive setting, while plans, events and logs keep hiding them.
- [x] **Plugins can easily check where a change is**
  Plugin authors can check whether a reported change is at a given setting, or anywhere inside it, without parsing text.
- [x] **External plugins receive the same information**
  Plugins that run as separate programs are told exactly the same changes and dependencies as plugins built into the program.
- [x] **Every plugin in the repository uses the new information**
  Every plugin shipped in the repository, including the examples and the test plugins, is given the changed settings and dependencies when deciding and updating.
- [x] **The plugin example hot swaps networks**
  In the plugin example, changing the networks a container is attached to updates the container in place, detaching the old network and attaching the new one, rather than rebuilding the container.
- [x] **The plugin example keeps containers through a network rebuild**
  When the example's network is rebuilt, for example because its address range changed, the container attached to it is updated in place and reattached, not rebuilt. Rebuilding the network does not fail because a container is still attached.
- [x] **The plugin example rebuilds a container when its init script is rebuilt**
  The example's container gains an init script, produced by a separate resource in the example that renders it. When the resource that produces the init script is replaced, the container is replaced too.
- [x] **A container can lose a network**
  When a network and every reference to it are removed from the example's configuration, the network is removed and the container stays, without that network.
- [x] **Documentation shows the new information**
  The project's guides and the documentation site show how plugin authors use the changed settings and dependencies when deciding and updating, using the network hot swap and the init script rebuild as examples.

## Constraints

- **Breaking the plugin interface is allowed.** The plugin contract and the protocol for plugins that run as separate programs may change incompatibly. No compatibility layer for plugins built against the current contract is required. The user decided this ahead of release.
- **The change type sits with the other change types.** The type that describes a changed setting is public and lives with the existing description of a resource's outcome and its changing dependencies, not with the plan-rendering code.
- **No cascade delete.** Removing a resource never deletes resources that are still in the configuration. A configured resource that references a removed one stays a validation error.
- **Decide, then act stays as it is.** Every decision is still made before anything is created, changed or destroyed, and destroys still come before creates and updates. This change must not alter that ordering.
- **The plugin decides.** Whether a resource is updated or replaced remains the plugin's decision. The new information informs that decision; the core does not act on it by itself.
- **Built to the existing design.** The design document `replacement-and-dependency-changes.md` in the `design` source still governs the unchanged/update/replace answer, what a resource is told about its dependencies, and the destroy-then-create order. This spec extends what plugins are told; it does not change those rules.
- **Tests follow the repository rules.** testify `require`, no table-driven tests, positive and negative cases in separate test functions, and Mockery for mocks. Tests that need saved state must produce it with real applies, never with hand-written state files.
- **Sensitive values stay hidden outside plugins.** Real sensitive values given to plugins must never reach plans, events or logs.
- **No configuration syntax change.** This change must not alter the configuration language.

## Acceptance Criteria

- [x] **An update names the changed setting and its old value**
  When a resource's setting is edited and its plugin answers update, a test plugin that records what it is told sees exactly that setting, its location, its previous value and its new value, and no unchanged setting.
- [x] **An update with no setting changes still learns its dependencies**
  When a resource's dependency is replaced, its own settings are unchanged, and its plugin answers update, the update is told nothing changed in its own settings and is told the dependency is replaced.
- [x] **A change decision sees the changed settings**
  When a resource's setting is edited, a test plugin that records what it is told when deciding sees that setting with its previous and new values.
- [x] **A not-yet-known value is marked when deciding**
  When a resource takes a value from a dependency the same apply replaces, the change it is told about when deciding marks the new value as not yet known.
- [x] **No value is still to be worked out at update time**
  When a resource takes a value from a dependency that is created or changed earlier in the same apply, the change its update is told about holds the real new value.
- [x] **Sensitive changes are real for plugins and hidden elsewhere**
  When a sensitive setting changes, the plugin is told its real previous and new values, while the plan, the event stream and the logs show it as hidden.
- [x] **A change can be matched to a setting**
  A plugin can tell, without parsing text, that a change to a container's first network name is at that network name and inside the networks setting, and that it is not inside the image setting.
- [x] **External and built-in plugins are told the same**
  The same edit applied through a plugin built into the program and through the same plugin run as a separate program results in identical changes and dependencies told to the plugin, when deciding and when updating.
- [x] **All repository plugins are given the new information**
  Each plugin shipped in the repository, when deciding and updating a resource whose setting was edited, receives that setting's change and its dependency list, and the full test suite, every example's tests and the external test plugins build and pass.
- [x] **Changing a container's network swaps it in place**
  In the plugin example, changing the container's network from one network to another updates the container without rebuilding it. Docker shows the same container, attached to the new network and no longer to the old one.
- [x] **Rebuilding the network keeps the container**
  In the plugin example, applying the address-range change rebuilds the network and updates the container in place. Docker shows the network on the new range, the same container attached to it, and the next plan reports no changes.
- [x] **Rebuilding the init script rebuilds the container**
  In the plugin example, changing the resource that produces the init script so that it is replaced causes the plan to show the container replaced because that resource is replaced. After the apply, Docker shows a new container.
- [x] **Removing a network and its references leaves the container**
  In the plugin example, removing the network and every reference to it deletes the network. The container stays, with no network attached, and the next plan reports no changes.
- [x] **A dangling reference is still rejected**
  Removing a network while the container still references it fails validation, and nothing is created, changed or destroyed.
- [x] **The documentation shows the new information**
  The project's guides and the documentation site show an update that uses the changed settings and dependencies, with the network hot swap and the init script rebuild as examples. The documentation site builds.

## Technical Approach

- **Reuse the plan's own change computation.** The core already works out which settings changed, with previous and new values, to render plans. Prefer giving plugins that same comparison, so the plan and what a plugin is told agree. The core then runs it again when the apply acts, so updates see real values.
- **A structured location with a matching helper.** Placement of the type is set by the constraints. Prefer it to carry a structured location, not a text path, with a small helper for matching a change against a setting.
- **Reuse the recorded dependency list.** Prefer handing an update the dependency list already recorded when the resource was decided, rather than working it out again.
- **External plugins carry the lists on the wire.** The protocol for plugins that run as separate programs gains the setting changes and the dependency list on both the decide and update calls, as with the dependency list today.
- **Detach in the provider.** In the plugin example, prefer the network's destroy force-detaching attached containers, with the container's update reattaching from its dependency and setting changes.
- **Risk: every plugin changes at once again.** As with the previous change, the contract change touches every in-repo plugin and the protocol. Prefer sequencing it so the repository keeps building.
- **Risk: sensitive values.** Plugins get real sensitive values (see Constraints); take care the shared change computation keeps its masking for plans, events and logs.

## Success Metrics

- The plugin example's container updates in place without any extra calls to discover its previous state. Its in-place update works only from what it is told: the changed settings and the changing dependencies.
- Across the example's scenarios, an apply rebuilds only what genuinely needs it: the network on an address-range change, and the container only when its init script's producer is rebuilt. Network changes never rebuild the container.
- After each example scenario, the real Docker state matches the configuration, and the next plan reports no changes.
- The set of settings a plugin is told changed matches the settings the plan showed for that resource, with nothing extra and nothing missing; only the values may differ, for sensitive settings and for values not known until the apply runs.

## Non-Goals

- **A plugin scaffold.** Scaffolding a new plugin with an explicit change decision is tracked separately in GitHub issue #8, probably as a GitHub template.
- **A shared default that replaces on replaced dependencies.** No new built-in change decision is added; scaffolding an explicit one is issue #8.
- **Reconciling against the real resource.** Plugins are not expected to look up previous values from the real resource or from saved state; the change list replaces that need.
- **The editor extension.** It needs no change.
