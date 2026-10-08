---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Config and plugin registries

## What was built

Setting up xcl is now a few lines, and plugins come only from registries the application adds explicitly.

- **Types declared at creation.** `xcl.WithType(&MyType{}, "type"[, "subtype"])` declares a plain Go type as a block type. No program builds or holds a catalog object any more.
- **Configuration without state.** Leaving out `WithStatePath`/`WithStateStore` persists nothing. This is now documented as the supported configuration-only mode.
- **Registries.** A new public package, `github.com/jumppad-labs/xcl/registry`, defines the `Registry` and `Plugin` interfaces, plus two plugin starters: `InProcess` (direct host) and `Executable` (process over gRPC).
  - `registry.NewLocal(registry.PluginPattern(...))` holds in-process plugins (`RegisterPlugin`), plugin binaries (`RegisterExternalPlugin`) and plugin directories (`RegisterPluginDirectory`), with the search pattern fixed at creation.
  - `xcl.WithRegistry(r)` adds a registry and may repeat. Registries load in the order given, and plugins load in registration order.
  - Third parties can write their own registries and plugin starters against the same interfaces.
- **Registration never returns an error.**
  - Code mistakes panic immediately, naming the type or plugin: an empty name, more than one subtype, an empty subtype, a non-entity type, a duplicate declared type, a keyword used in both forms, a builtin name, or a nil plugin or registry.
  - Environment problems are returned by the first `Validate`/`Apply`/`Destroy`/`Diff`/`Load`:
    - A missing or failing plugin is a `*xcl.PluginLoadError` naming the plugin and its registry. **Any** failing plugin fails the load, including one found by directory discovery.
    - A block type provided twice anywhere is a `*xcl.TypeNameClashError` naming both providers and both registries. There is no precedence between registries.
- **The catalog is private.** The old `plugins/registry` package, with `PluginRegistry`, `NewPluginRegistry`, `RegisterPluginWithPath`, `DiscoverPlugins` and `CastResourceTo`, is gone, and so is `WithPluginRegistry`. The catalog lives in `internal/catalog`, and each `Config` owns its own catalog and starts its own plugin hosts.
- **Events read their entity without a catalog.**
  - `Event.Entity()` returns the entity an event's data holds as the registered Go type, with sensitive values masked. Event JSON is unchanged.
  - `prettylog.Handler(w, level)` no longer takes a registry.
  - `(*Config).EncodeSavedEntity(data, ...)` replaces the package-level function that took a registry.
- **Examples, tests and docs.**
  - The configuration-only example declares its types and keeps no state. The plugin example uses one local registry.
  - Every test suite moved to the new API, with new acceptance tests for each rule above.
  - The README, `docs/` and the documentation site (including a new Registries guide) show only the new API.
  - The stray committed `configonly` binary is gone, and example build outputs are ignored.

### Breaking changes

- `plugins/registry` was removed. `RegisterPluginWithPath` is now `RegisterExternalPlugin`, and `DiscoverPlugins` is now `RegisterPluginDirectory` plus `PluginPattern`. `CastResourceTo` is gone.
- `WithPluginRegistry` was removed and replaced by `WithType` and `WithRegistry`.
- `EncodeSavedEntity` is now a method on `Config`.
- `prettylog.Handler` takes two arguments.
- Type registration mistakes now panic.
- A discovered plugin that fails now fails the load.
- `PluginLoadError` gained `Registry`. For a binary, `Plugin` is now the file name; the path stays in the message.
- `TypeNameClashError` and `TypeFormError` moved to `xcl`/`errors`, and the clash error gained the `Provider`, `Registry` and `ExistingRegistry` fields.
- Each `Config` now starts its own plugin hosts, even when given the same registry value.

## Why it matters

Before this change, even a program that only reads configuration into its own types had to build a plugin catalog by hand, check an error after every type it registered, and set up state it then threw away. Programs are now short and hard to get wrong. It is always explicit where a plugin comes from. The registry interfaces leave room for a remote registry, or for plugins reached over the network, without changing them.

## Deviations from the plan

- **Discovered plugins.** By the user's decision during planning, a discovered plugin that fails now fails the load, rather than being skipped. The spec's two "skipped" items were recorded as descoped.
- **`example/configonly`.** The example was committed broken, so it was restored from `b0c43e2` (with the user's approval) and migrated from there.
- **Combined tasks.** The internal catalog and Config options tasks were done together: deleting `plugins/registry` breaks every caller at once, so tests were migrated once, straight to the final API, with no interim `WithPluginRegistry(*catalog.Catalog)`. The shape checks were shared through `catalog.ValidateDeclaration`.
- **Masked values.** A masked `types.Sensitive` read through `Event.Entity()` *shows* the mask marker, but `Reveal()` returns the zero value, as the type documents. The tests assert the shown value.
- **Pretty log tests.** Configuration rendering is now tested by driving a real Config through a new one-block fixture, because hand-built events carry no decoder.
- **Directory discovery.** Each registered directory is searched, and reported with its own discover events, at its registration position.
- **Website.** The website's "Writing your own registry" section includes an illustrative `Remote` registry that xcl does not ship; it is flagged for review in the test plan.
- **README.** The README's Custom Functions section was already out of date before this change and was left alone.

## Revision: types declared on registries (2026-10-08)

After the first implementation, the user clarified that types belong on registries, not on `Config`. The design and spec were amended to match, and the change was made on the same branch:

- `xcl.WithType` is removed. Plain Go types are declared with `registry.Local.RegisterType(prototype, name...)`, which returns nothing and panics only on a malformed declaration. `xcl.WithRegistry` is the one way in.
- `registry.Registry` gains `Types() []registry.Type` (`Type`, `Subtype`, `Prototype`). A registry with no Go types returns nil.
- `NewConfig` reads every registry's types in order. It **returns an error**, rather than panicking, when declared types clash: a duplicate in one registry or across two, a keyword in both forms, or a builtin name (`*TypeNameClashError` naming both Go types and registries, or `*TypeFormError`).
- A plugin clashing with a declared type is still a load-time error. `ExistingRegistry` now names the registry that declared the type.
- Shape validation moved to a new `internal/declaration` package, shared by `registry` and `internal/catalog`. The catalog's type registration returns errors instead of panicking.
- Every `WithType` call site (about 90, across root, e2e, savedentity, state and the examples), the README and `docs/`, and the website pages (including the Registries guide) were migrated. The guide's hypothetical `Remote` registry was replaced by a custom `Toolkit` registry, labelled as not part of xcl.
