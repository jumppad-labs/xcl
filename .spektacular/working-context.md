# Working context — plan 20261008071608-eb05cae0-config-and-plugin-registries

## Spec workflow carry-over (decisions already made with the user)
- Nicest syntax over compatibility; breaking public API OK, no shims, no migration guide (CHANGELOG entry only).
- Design doc `design/plugin-registries.md` is binding (option names, Registry/Plugin contracts, local registry calls, failure rules, internals).
- Registration never errors: programmer errors panic; environment errors at plugin load as *PluginLoadError naming plugin + registry.
- Clashes always an error (no precedence), incl. plugin vs WithType, regardless of order.
- Remote registry: designed only, ship nothing.
- xcl-website docs updated in this change; remove committed `configonly` binary + gitignore example builds.
- Single spec, not an epic.

## Codebase learnings (from spec phase — re-verify during discovery)
- plugins/registry/plugin_registry.go: RegisterType, RegisterPlugin, RegisterPluginWithPath, DiscoverPlugins, Load, checkType/checkHostTypes.
- config.go NewConfig (nil store = no state); xcl.Event = events.Event alias.
- example/prettylog uses registry only for xcl.EncodeSavedEntity.

## Plan workflow
- Plan started 2026-10-08. User picked this spec.

## Discovery (done)
- Target repos: xclconfig (/home/nicj/code/github.com/jumppad-labs/xcl) and xcl-website (/home/nicj/code/github.com/jumppad-labs/xcl-website). Not xcl-vscode.
- User asked to commit the broken configonly WIP: committed 465566f on f-diff ("I will fix this myself just commit it broken"). Plan targets the spec's final example shape; STOP if ingressRoutes missing at implement time.
- Public package: github.com/jumppad-labs/xcl/registry; catalog moves to internal (main-module tests can still import it; example modules cannot).
- Website verify: npm ci, npm run build, npx astro check; Go snippets are hand-copied, nav in src/components/Nav.astro.
- Open for architecture: all-discovered-fail rule; how catalog tells discovered (skippable) plugins from explicit ones.

## Architecture (done)
- USER DECISION (planning, 2026-10-08): any plugin failure is a load error, INCLUDING plugins found by RegisterPluginDirectory ("Any failure is an error"). Overrides spec req "Discovered plugins that fail are skipped", its acceptance criterion, and design "skipped as today". Rejected/skip path and rejected:true event removed. Raise at walkthrough: offer to update spec + design doc (via spek-design) to match.
- Packages: public registry/ (registry.go, local.go, discovery.go); internal/catalog (moved PluginRegistry); plugins/registry deleted.
- Event: unexported decoder + (Event).WithEntityDecoder; attached in Config.run.
- Components drafted (components.md).
- Data structures drafted.
- Implementation detail drafted.
- Dependencies drafted.
- Testing approach drafted (failing-discovered acceptance criterion inverted per user).
- Milestones: M1 events/prettylog/EncodeSavedEntity method; M2 registries+WithType+migration; M3 docs+hygiene.
- Tasks drafted: 12 tasks, ids in tasks_plan.md. Catalog task retypes WithPluginRegistry temporarily.
- Open questions + out of scope drafted.
- Assembled + staged to .spektacular/tmp/<plan>/ (plan/context/research _template.md).
- Verification passed (12 tasks, anchors ok, no shell cmds in plan.md).
- All three docs written to plan store; working dir removed. Now in walkthrough.
- Walkthrough: user signed off ('no looks good'); no assumptions challenged. Spec/design not updated for the any-failure decision (not requested).
