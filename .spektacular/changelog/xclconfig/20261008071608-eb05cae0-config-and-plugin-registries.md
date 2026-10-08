---
created_date: "2026-10-08"
document_status: draft
project: xclconfig
spec: 20261008071608-eb05cae0-config-and-plugin-registries
plan: 20261008071608-eb05cae0-config-and-plugin-registries
---

# Config and plugin registries

Programs now declare their own Go types and their plugins on registries they add explicitly with `xcl.WithRegistry`, starting with a local registry (`RegisterType`, `RegisterPlugin`, `RegisterExternalPlugin`, `RegisterPluginDirectory`). They can run without saved state as a supported configuration-only mode. Registration never returns an error: code mistakes panic straight away, and missing, failing or clashing plugins are reported by the first operation that loads plugins, naming the plugin and its registry. Event handlers can read an event's entity with `Event.Entity()` without holding anything from xcl.

> Derived from project xcl (xclconfig), spec/plan 20261008071608-eb05cae0-config-and-plugin-registries. See the project-level record for the full feature.

## What changed in this repo

- **New public package `registry`**: the `Registry`/`Plugin` interfaces, `InProcess`, `Executable`, and `NewLocal` with `PluginPattern`, `RegisterPlugin`, `RegisterExternalPlugin` and `RegisterPluginDirectory`. Directory discovery moved here.
- **New private package `internal/catalog`**: the former `plugins/registry.PluginRegistry`, which is now owned by one Config and loads plugins registry by registry. Any plugin failure fails the load, and every duplicate block type is a `TypeNameClashError` naming both providers and registries. `plugins/registry` was deleted.
- **Root package**:
  - `WithRegistry` was added, and `WithPluginRegistry` was removed. `NewConfig` builds the catalog from every registry's declared types and returns an error on a clash.
  - `(*Config).EncodeSavedEntity` replaces the package-level function.
  - `Config.run` attaches an entity decoder to every event.
  - `TypeNameClashError` and `TypeFormError` are re-exported.
- **`events`**: `Event.Entity()`, `WithEntityDecoder` and `EntityDecoder` were added. The decoder is unexported, so it is never serialised.
- **`errors`**:
  - `PluginLoadError` gained `Registry`.
  - `TypeNameClashError` gained `Provider`, `Registry` and `ExistingRegistry`, and moved here along with `TypeFormError`.
- **`internal/parser` and `internal/savedentity`**: they take `*catalog.Catalog`, and the parser option is now `ParserOptions.Catalog`.
- **Examples**:
  - `example/configonly` declares its types on a local registry and keeps no state.
  - `example/plugin` uses one local registry.
  - `example/prettylog.Handler(w, level)` encodes `Event.Entity()`.
- **Tests**:
  - Every suite was migrated: root, internal, state, e2e, plugins/example and the examples.
  - Registration error tests became panic tests, and clash and failing-plugin tests became load-time tests.
  - New acceptance tests cover no state, declared types, every panic, missing and failing plugins, two registries, load order, custom patterns, duplicates and a custom registry.
- **Docs**: `README.md`, `docs/*.md` and `e2e/COVERAGE.md` were updated to the new API.
- **Hygiene**: the tracked root `configonly` binary was removed, and `.gitignore` covers example build outputs.

### Breaking

- `plugins/registry`, `WithPluginRegistry` (types are declared with `registry.Local.RegisterType`), `RegisterPluginWithPath`, `DiscoverPlugins`, `CastResourceTo` and the package-level `EncodeSavedEntity` were removed.
- `prettylog.Handler` takes two arguments.
- Type registration mistakes panic.
- A failing discovered plugin fails the load.
- `PluginLoadError.Plugin` is the binary's file name for an external plugin.
- Each Config starts its own plugin hosts.

## Why

Embedding xcl took too much ceremony: a hand-built catalog, an error check per registration, and throwaway state. This repo holds the library, so it carries the new options, the registry contracts, the private catalog and the examples that demonstrate the short setup.

## Revision: types declared on registries

After the first implementation, types moved from `Config` onto registries. `xcl.WithType` is removed: plain Go types are declared with `registry.Local.RegisterType`, and `registry.Registry` gains `Types()`. Creating a configuration now returns an error, rather than panicking, when declared types clash: a duplicate in one registry or across two, a keyword in both forms, or a builtin name.
