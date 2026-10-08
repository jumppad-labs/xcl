---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Context: 20261008071608-eb05cae0-config-and-plugin-registries

## Current State Analysis

- `plugins/registry/plugin_registry.go` (`PluginRegistry`) is public and does two jobs. It is the type catalog: builtins, `RegisterType` Go types and plugin types, plus `CreateEntity`, `TypePath` and provider lookup. It is also the plugin loader: `RegisterPlugin`, `RegisterPluginWithPath` and `DiscoverPlugins` record plugins, and the once-only `Load` and per-operation `Use` start and stop them. Registration returns errors, except `RegisterPluginWithPath`, which always returns nil. A clash with an already-loaded plugin is reported at `RegisterType` (`:305`), and a clash with a not-yet-loaded plugin is reported at load.
- Discovered plugins that fail are rejected and skipped (`:687-727`, `rejected:true` event), and the load fails only when every discovered plugin fails. **The user decided during planning that any plugin failure fails the load.**
- `config.go:186-210` `NewConfig` applies options in order and defaults to `registry.NewPluginRegistry()`. With no state store, nothing is persisted (`config.go:353`). `config.go:498` `run` is the single wrapper for every operation's events.
- `options.go:17` `WithPluginRegistry` is how every program and test supplies types and plugins today.
- `encode.go:168` `EncodeSavedEntity(registry, data, ...)`; `example/prettylog/prettylog.go:64` `Handler(w, level, reg)` needs the registry only for that.
- `events/events.go:67` `Event` has no JSON tags and no custom marshaller.
- `errors/plugin_load_error.go:20` `PluginLoadError{Plugin, Err}`. The clash and form errors live in `plugins/registry/errors.go`.
- Importers of `plugins/registry`:
  - 3 production files in root, 2 internal (`internal/parser`, `internal/savedentity`) and 3 example programs.
  - Tests: about 115 `NewPluginRegistry` calls, 70 `RegisterType`, 34 `RegisterPlugin`, 11 `RegisterPluginWithPath`, 82 `WithPluginRegistry` and 30 `EncodeSavedEntity`.
- `example/configonly` is committed broken at `465566f` (user WIP: `routes.go`/`routes_test.go` deleted, `main.go` half-rewritten). The user will fix it by hand.
- A tracked 24 MB ELF `configonly` sits at the repo root (51f1c0b). `.gitignore` does not cover it.
- xcl-website shows the old API on `index.mdx`, `events.mdx`, `plugin-logging.mdx`, `configuration-text.mdx`, `examples/configuration-only.mdx` and `examples/plugins.mdx`. Its Go snippets are hand-copied and never compiled.

**Requirement → repo and files**
- Configuration without saved state, Declare types, Registration never returns an error, Programmer mistakes panic → xclconfig: `options.go`, `config.go`, `internal/catalog/`, `registry/local.go`.
- Environment problems at load, Registries supply plugins, Local registry, Load order, Discovered plugins (any failure is an error), Duplicate type always an error, Custom registries → xclconfig: `registry/`, `internal/catalog/catalog.go`, `errors/`.
- Event entity, Events serialise cleanly → xclconfig: `events/events.go`, `config.go` (`run`).
- Pretty logging needs no catalog → xclconfig: `example/prettylog/`.
- Encoding saved data through the Config → xclconfig: `encode.go`.
- Catalog no longer public → xclconfig: delete `plugins/registry/`, add `internal/catalog/`.
- Examples use the new API → xclconfig: `example/configonly/`, `example/plugin/`.
- Documentation uses the new API → xcl-website: `src/pages/*.mdx`, `src/pages/registries.mdx`, `src/components/Nav.astro`; xclconfig: `README.md`, `docs/*.md`, `e2e/COVERAGE.md`.
- No committed build outputs → xclconfig: `configonly`, `.gitignore`.

## Per-Task Technical Notes

### Task: Event entity accessor

**Requirement → repo:** "Event handlers can read an event's entity without a catalog" and "Events still serialise cleanly" → xclconfig.

**File changes**:
- `events/events.go:67-112`: add an unexported field `decode EntityDecoder` to `Event`. Add `type EntityDecoder func(data []byte) (any, error)`, `func (e Event) WithEntityDecoder(d EntityDecoder) Event` (returns a copy with the decoder set), and `func (e Event) Entity() (any, error)`. `Entity` returns `nil, nil` when `len(e.Data) == 0`. It returns an error ("event carries data but was not delivered by a Config") when `Data` is set but `decode` is nil. Otherwise it returns `e.decode(e.Data)`. Doc comments follow the design text. No import of the root package.
- `events.go:1-40`: document `Entity` in the root alias comment. Update the `EventDataProcessed` comment, which now says "the form EncodeSavedEntity reads", to point at `Event.Entity` and `(*Config).EncodeSavedEntity`.
- `config.go:498-554` (`run`): when a handler is set, build `emit := func(e Event) { stream.Emit(e.WithEntityDecoder(c.decodeEventEntity)) }`. Use it for `work(ctx, emit)` and for the start and finish events. Add `func (c *Config) decodeEventEntity(data []byte) (any, error)`, which calls `savedentity.Decode(c.pluginRegistry, data, savedentity.ReadOptions{ForDisplay: true})` (it becomes `c.catalog` in the catalog task).
- `config_event_entity_test.go` (new, root package): separate tests:
  - `TestEventEntityReturnsRegisteredGoType` checks processed data, a registered type and the configured values.
  - `TestEventEntityShowsSensitiveValuesMasked` uses a `types.Sensitive` field and expects `types.SensitiveMarker`.
  - `TestEventEntityWithoutDataReturnsNil` uses the operation start event.
  - `TestEventEntityFromRawDataReturnsRegisteredGoType` uses `EventDataRaw`.
- `events/events_test.go` (new or existing): `TestEventMarshalsToJSONWithoutDecoder` marshals an event with a decoder attached and asserts the JSON keys equal those of the same event without one. `TestEventEntityWithDataButNoDecoderFails` covers the no-decoder error.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Encode saved data through the Config

**Requirement → repo:** "Encoding saved data through the configuration" → xclconfig.

**File changes**:
- `encode.go:142-197`: replace `func EncodeSavedEntity(registry *registry.PluginRegistry, data []byte, options ...EncodeOption)` with `func (c *Config) EncodeSavedEntity(data []byte, options ...EncodeOption) ([]byte, error)`. Load plugins through `c.pluginRegistry.Use(nil)`, calling `done()` afterwards, so external processes are stopped again. Then call `savedentity.Decode(c.pluginRegistry, data, ReadOptions{ForDisplay: true})` followed by `encodeEntity`. Drop the nil-registry `NotEncodableError` branch. Keep the doc comment's error list.
- `encode_test.go`, `encode_highlight_test.go`, `encode_references_test.go`, `encode_sensitive_test.go`, `sensitive_leak_test.go`, `config_test.go`, `config_event_mask_test.go`, `config_event_sensitive_test.go`, `config_event_data_test.go`: migrate the 24 calls to `c.EncodeSavedEntity(data, ...)`, using the Config each test already builds. A test that only had a registry builds `xcl.NewConfig(xcl.WithPluginRegistry(reg))` here; that becomes `WithType` in the Config options task.
- `e2e/encode_test.go`, `e2e/plugin_encode_test.go`, `e2e/plugin_state_test.go`, `e2e/sensitive_test.go`: one call each, migrated.
- `example/prettylog/prettylog.go:112` and `example/prettylog/prettylog_test.go:470`: these call sites are rewritten by the pretty log task. To keep this task compiling on its own, change line 112 to build a Config with `xcl.NewConfig(xcl.WithPluginRegistry(reg))` and call the method. The pretty log task then deletes it.
- `encode_test.go`: add `TestConfigEncodeSavedEntityMatchesEncodeEntity`, which compares the method's output on processed event data with `xcl.EncodeEntity` on the applied entity. `TestConfigEncodeSavedEntityLoadsPluginsFirst` uses an in-process plugin type on a fresh Config.

**Complexity**: Low
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Pretty log handler without a catalog

**Requirement → repo:** "Pretty event logging needs no catalog" → xclconfig (`example/prettylog` module).

**File changes**:
- `example/prettylog/prettylog.go:56-64`: change the signature to `func Handler(w io.Writer, level slog.Level) xcl.EventHandler` and update the doc comment and example line 63. Remove the `plugins/registry` import at line 30.
- `example/prettylog/prettylog.go:107-130`: `writeConfiguration(w, logger, options, e)` calls `entity, err := e.Entity()`. A nil entity means nothing is written. Otherwise it calls `xcl.EncodeEntity(entity, options...)`, and an error is logged as today.
- `example/prettylog/prettylog_test.go`: update 13 `Handler` calls (lines 126, 142, 158, 169, 184, 243, 327, 346, 363, 398, 420, 459, 502) to the two-argument form. Tests that used `pr` for configuration output must now drive a real Config with a handler, so the event carries a decoder. Hand-built events no longer decode. Line 470 compares output with `c.EncodeSavedEntity` or `xcl.EncodeEntity`. Add `TestHandlerWritesConfigurationWithoutRegistry`.
- `example/plugin/main.go:218-219`: `eventHandler(out io.Writer) xcl.EventHandler` returns `prettylog.Handler(out, prettylog.LevelFromEnv())`. Update the callers at 157 and 208, which still pass `r` to `newConfig` until the examples task.
- `example/configonly/main.go`: currently broken work in progress (`465566f`), repaired by the user. If it calls `prettylog.Handler` with three arguments when this task runs, change it to two. If the module does not compile for reasons unrelated to this change, STOP and ask the user.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Public registry package and local registry

**Requirement → repo:** "Registries supply plugins", "Local registry", "Custom registries" (contracts), "Registration never returns an error" (local calls) → xclconfig.

**File changes**:
- `registry/registry.go` (new): package doc. The `Registry` and `Plugin` interfaces, verbatim from design § Registries. `InProcess(p plugins.Plugin) Plugin` panics with `xcl: registry: in-process plugin must not be nil`. Its `Name()` is `plugins.PluginName(p)` (`plugins/direct_plugin_host.go:71`) and its `Start(emit)` is `plugins.NewDirectPluginHost(emit, nil, p)` (`:29`). `Executable(path string) Plugin`: its `Name()` is `plugins.PluginBinaryName(path)` (`plugins/grpc_plugin_host.go:132`), and its `Start(emit)` calls `h := plugins.NewGRPCPluginHost(emit, nil); err := h.Start(path)` (`:38,:51`) and returns `h`, which keeps `Restart`/`Path` for the catalog's restart logic.
- `registry/local.go` (new): `type Local struct { mu sync.Mutex; pattern string; entries []localEntry }`. A `localEntry` is either a plugin or a directory. `NewLocal(options ...LocalOption)`. `PluginPattern(p string) LocalOption`, where an empty value keeps the default `xcl-plugin-*`. `RegisterPlugin(p)` appends `InProcess(p)` and panics on nil with a message naming registry `local`. `RegisterExternalPlugin(path)` appends `Executable(path)`. `RegisterPluginDirectory(dir)` appends a directory entry. `Name()` returns `"local"`. `Plugins(ctx, emit)` walks the entries in order: a plugin is appended, and a directory is expanded (from `ExpandPluginDirectories`) and searched. Discover start/success/error events are emitted around the directory searches, carrying `Meta{"dirs", "registry": "local"}`, as `plugin_registry.go:665-682` does today. Each match is appended as `Executable(path)`.
- `registry/discovery.go` (new): move `plugins/registry/plugin_discovery.go:14-168` unchanged in behaviour, unexported (`pluginDiscovery`, `newPluginDiscovery`, `expandPluginDirectories`).
- `registry/discovery_test.go`: move `plugins/registry/plugin_discovery_test.go`, `testutils_discovery_test.go` and the needed event helpers from `testutils_events_test.go`, adapted to the unexported names.
- `registry/local_test.go` (new): separate tests:
  - `TestLocalPluginsKeepRegistrationOrder` mixes an in-process plugin, a path and a directory.
  - `TestLocalCustomPatternFindsOnlyMatchingExecutables`.
  - `TestLocalDefaultPatternIsXclPlugin`.
  - `TestLocalRegisterNilPluginPanics`.
  - `TestLocalRegisterMissingPathDoesNotFail`.
  - `TestLocalExpandsHomeInPluginDirectory`.
  - `TestExecutableStartFailsForMissingPath`.
  - `TestInProcessStartReturnsHostWithPluginTypes`.

**Complexity**: Medium
**Token estimate**: ~45k tokens
**Agent strategy**: 2 parallel agents. One moves discovery and its tests, the other writes `registry.go`, `local.go` and the new tests. Then integrate.

### Task: Plugin load and type clash errors name registries

**Requirement → repo:** "Environment problems are reported when plugins load" and "A block type provided twice is always an error" (error shapes) → xclconfig.

**File changes**:
- `errors/plugin_load_error.go:18-31`: add `Registry string`. `Error()` is `plugin %s from registry %s failed to load: %s`. When `Plugin == ""` it is `registry %s failed to load its plugins: %s`. Keep `Unwrap() []error`.
- `errors/type_errors.go` (new): move `TypeNameClashError` from `plugins/registry/errors.go:10-21`, now with fields `Name, Provider, Registry, Existing, ExistingRegistry string` and pointer receivers. Its message is `type %q is provided by both %s and %s`, where each side renders as `<provider>` or `<provider> (registry <name>)` when the registry is non-empty. Move `TypeFormError` (`:28-41`) unchanged.
- `errors/type_errors_test.go` and `errors/plugin_load_error_test.go` (new): message and `errors.Is`/`errors.As` tests, one per behaviour.
- `config.go:118-138`: add `TypeNameClashError = xclerrors.TypeNameClashError` and `TypeFormError = xclerrors.TypeFormError` aliases, with doc comments in the existing block.
- `plugins/registry/errors.go`: make it type aliases of the moved errors, so the old package keeps compiling until the catalog task deletes it.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Internal type catalog loading from registries

**Requirement → repo:** "Programmer mistakes in registration stop the program immediately", "Load order follows registration order", "Discovered plugins that fail" (inverted by the user: any failure is an error), "A block type provided twice is always an error", "The catalog is no longer public" → xclconfig.

**File changes**:
- `internal/catalog/catalog.go` (new, moved from `plugins/registry/plugin_registry.go`):
  - Rename `PluginRegistry`→`Catalog` and `NewPluginRegistry`→`New`.
  - Replace the fields `pending`, `pendingPaths`, `discoveryDirs` and `discoveryPattern` (`:39-43`) with `registries []registry.Registry`. Replace `pluginHosts []plugins.PluginHost` (`:37`) with `hosts []loadedHost{host plugins.PluginHost; plugin, registry string}`, keeping `GetPluginHosts` returning the hosts.
  - `RegisterType` (`:104-144`) returns nothing and panics with `fmt.Sprintf("xcl: %s", err)` on each former error. `checkType` no longer runs against plugin hosts at registration, because none have loaded.
  - Add `AddRegistry(r registry.Registry)`, which panics on nil.
  - Delete `RegisterPlugin`, `RegisterPluginWithPath`, `DiscoverPlugins`, `CastResourceTo` (`:795-808`), `discover`, `loadDiscovered` and `emitRejected` (`:663-727`, `:776-784`).
  - Rewrite `load` (`:607-661`) as the loop in plan § Implementation Detail. Call `r.Plugins(ctx, emit)` with `context.Background()`, since Load takes no ctx today. A Plugins error becomes `&xclerrors.PluginLoadError{Registry: r.Name(), Err: err}`. For each plugin, call `emitLoad(emit, name, registry, phase, ...)` with `"registry"` added to Meta (`:767-774`), then `p.Start(r.pluginEmit)`. A start error becomes `PluginLoadError{Plugin: p.Name(), Registry: r.Name()}`, emitted and returned. Then `addHost(host, p.Name(), r.Name())`; on a clash, `host.Stop()`, emit and return it.
  - `checkHostTypes` and `checkType` (`:288-348`) build `*xclerrors.TypeNameClashError{Name, Provider: plugin, Registry: registry, Existing: <"builtin" | "type <Go type>" | other plugin>, ExistingRegistry}`. The Go type name comes from `reflect.TypeOf(info.Prototype)`. They build `*xclerrors.TypeFormError` for form clashes.
  - `restartPlugins` (`:551-564`) sets `Registry` on its `PluginLoadError` from `loadedHost`.
  - Keep `Use`, `release`, `Activate`, `pluginEmit`, `Load`, `Loaded`, `Type`, `TakesSubtype`, `Types`, `KnownType`, `IsRegisteredType`, `CreateEntity`, `GetProvider`, `GetProviderForResource` and `TypePath` as they are.
- `internal/catalog/catalog_test.go` (moved from `plugins/registry/plugin_registry_test.go`, `sensitive_types_test.go`, `type_path_test.go`, `testutils_events_test.go`):
  - Convert the error tests at `:146,171,185,199,213,338,348,358,369,380,521,538,553,568,579,590,598,606,613,970,984` into `require.PanicsWithValue` or `require.Panics` tests, one per case, keeping their names but with `Panics` in place of `Rejects`.
  - Turn `:227` and `:1266`, which tested registration against a loaded plugin, into load-time tests: register the type, add a registry with the plugin, and expect a `TypeNameClashError` or `TypeFormError` from `Load`.
  - Turn the discovery-through-registry test at `:813` into `TestLoadDiscoveredPluginThatFailsFailsTheLoad`.
  - Add:
    - `TestLoadOrderFollowsRegistriesThenRegistration`, which asserts on the load start event sequence `[a1, a2, b1]` with registry meta.
    - `TestLoadFailsForDuplicateTypeAcrossRegistries` and `TestLoadFailsForDuplicateTypeWithinRegistry`, which assert both providers and registries.
    - `TestLoadFailsForPluginTypeClashingWithDeclaredType`.
    - `TestLoadFailsWhenRegistryPluginsFails`.
    - `TestLoadFailsNamingRegistryForMissingBinary`.
  - Test plugins: use `plugins/testing/helpers.go` and the existing in-package test plugins.
- `plugins/registry/` (delete the whole directory).
- `internal/parser/parser.go:26,89-101,143-160,167,182,755,916,1518`: change `ParserOptions.PluginRegistry *registry.PluginRegistry` to `Catalog *catalog.Catalog`. The default is `catalog.New()`. Update the references.
- `internal/parser/callbacks.go:23-33`: update the comments, which say "Satisfied by *catalog.Catalog".
- `internal/parser/{computed,lifecycle,parser_plugin,parse,registered_types,validate,diff}_test.go`: use `catalog.New()`, `RegisterType` with no error, and `AddRegistry(local)`, where `local := registry.NewLocal(); local.RegisterPlugin(p)`. `parse_test.go:83-85` drops its panic-on-error. `Load` calls are unchanged.
- `internal/savedentity/savedentity.go:55,129` and the tests `savedentity_test.go`, `decode_all_test.go`: change the parameter type to `*catalog.Catalog`. The tests use `catalog.New()`.
- `state/file_state_store_test.go:34,134-137` and `state/file_state_store_apply_test.go:51-57`: use the catalog directly where only `CreateEntity` is needed. Calls through Config are migrated in the Config options task. Until then this file uses `xcl.WithPluginRegistry`, which the next task replaces. Keep the two tasks consistent by having this task temporarily make the root package's `WithPluginRegistry` take `*catalog.Catalog`.
- `config.go:17,143-145,170-176,199-201,300,334,417,456,556-575`, `config_diff.go:47`, `encode.go`, `query.go:303,318,378` and `options.go:8,17`: switch the imports and field types to `internal/catalog`. Temporarily keep `WithPluginRegistry(*catalog.Catalog)` so the root tests keep compiling; it is removed in the next task.
- Root tests that call `RegisterPlugin`, `RegisterPluginWithPath` or `GetPluginHosts` on the registry are rewritten in the next task. This task makes them compile by mechanically replacing `registry.NewPluginRegistry()` with `catalog.New()`, dropping `require.NoError` around `RegisterType`, and replacing plugin registration with `local := registry.NewLocal(); local.RegisterPlugin(...); cat.AddRegistry(local)`.

**Complexity**: High
**Token estimate**: ~120k tokens
**Agent strategy**: Parallel analysis, sequential integration. One agent builds `internal/catalog` and its tests. Once the package compiles, a second agent migrates `internal/parser`, `internal/savedentity` and `state`, and a third the root package's compile-only edits. Integration is run sequentially, with `go build ./... && go test ./internal/... ./state/...`.

### Task: Config options declare types and add registries

**Requirement → repo:** "Configuration without saved state", "Declare types when creating the configuration", "Registration never returns an error", "Registries supply plugins", "Custom registries", "The catalog is no longer public" → xclconfig.

**File changes**:
- `options.go:13-22`: delete `WithPluginRegistry`. Add `WithType(prototype any, name ...string) ConfigOption`. It validates eagerly, at the `WithType` call and outside the returned closure: an empty name, more than one name past the type, an empty subtype, and a value that is not a non-nil pointer to a struct for which `types.GetMeta` succeeds. Each failure panics with `xcl: type %q ...`. The closure appends `typeDecl{prototype, name}` to `c.types`. Add `WithRegistry(r registry.Registry) ConfigOption`, which panics on nil and appends to `c.registries`. Update the `WithStateStore`/`WithStatePath` comments: "Without either, nothing is persisted; this is the supported mode for configuration-only use".
- `config.go:143-156`: replace the `pluginRegistry` field with `catalog *catalog.Catalog`, `types []typeDecl` and `registries []registry.Registry`.
- `config.go:182-210` (`NewConfig`): after the options, `c.catalog = catalog.New()`. For each declaration call `c.catalog.RegisterType(d.prototype, d.name...)`, which panics on a duplicate, a form clash or a builtin name. For each registry call `c.catalog.AddRegistry(r)`. Update the doc comment to say that options may be given in any order.
- `config.go`, `config_diff.go`, `encode.go`, `query.go`: rename `c.pluginRegistry` to `c.catalog`.
- Root tests, which make up the bulk: migrate every `catalog.New()` and `WithPluginRegistry` site left by the catalog task to `xcl.NewConfig(xcl.WithType(...), xcl.WithRegistry(local), ...)`. The files are `config_*_test.go`, `decode_test.go`, `encode*_test.go`, `entity_subtype_test.go`, `query*_test.go` and `sensitive_leak_test.go` (69 `WithPluginRegistry`, 45 `RegisterType`, 20 `RegisterPlugin` and 7 `RegisterPluginWithPath` sites). Root tests are in package `xcl`, so tests that need hosts (`config_plugin_loading_test.go:204,228`, `config_plugin_boundary_test.go:120,645`, `config_plugin_lifecycle_test.go:22,70-135`) use `c.catalog.GetPluginHosts()` and `c.catalog.Use(nil)`. `config_plugin_loading_test.go:268` and `config_plugin_boundary_test.go:684` use `errors.As(err, &clash)` with `*xcl.TypeNameClashError`. `state/custom_store_test.go:71,99,140,149` and `state/file_state_store_apply_test.go` use the options too.
- Any helper needed by more than one package for building a local registry with the shared test plugins goes in `internal/testutil` (it must not import root `xcl`).
- New acceptance tests (root package; positive and negative cases in separate functions):
  - `config_no_state_test.go`:
    - `TestApplyWithoutStateWritesNothing` uses `t.Chdir(t.TempDir())` and `t.Setenv("HOME", t.TempDir())`, applies with `WithType` only, asserts `Entities()` is non-empty, and asserts both directories are still empty afterwards.
    - `TestDeclaredTypesDecodeIntoGoValues`.
  - `config_with_type_test.go`, one panic test per case through `WithType` and `NewConfig`:
    - `TestWithTypePanicsOnEmptyName`
    - `TestWithTypePanicsOnMoreThanOneSubtype`
    - `TestWithTypePanicsOnEmptySubtype`
    - `TestWithTypePanicsOnNonEntityType`
    - `TestNewConfigPanicsOnDuplicateType`
    - `TestNewConfigPanicsOnTypeInBothForms`
    - `TestNewConfigPanicsOnBuiltinTypeName`
    - `TestWithRegistryPanicsOnNil`
    
    Each asserts that the panic message contains the type key.
  - `config_registries_test.go`:
    - `TestMissingPluginBinaryFailsFirstApplyNamingRegistry`
    - `TestPluginsFromTwoRegistriesAreUsable`
    - `TestPluginLoadEventsFollowRegistryOrder`
    - `TestFailingDiscoveredPluginFailsTheLoad`
    - `TestFailingRegisteredPluginFailsTheLoad`
    - `TestDuplicateTypeAcrossRegistriesFailsLoad`
    - `TestDuplicateTypeWithinRegistryFailsLoad`
    - `TestPluginTypeClashingWithDeclaredTypeFailsLoad`, together with `TestPluginTypeClashingWithTypeDeclaredAfterRegistryFailsLoad`
    
    Directory tests use a temp dir holding the test plugin binary, which `config_plugin_loading_test.go` already builds, and a non-executable or corrupt `xcl-plugin-bad`.
  - `config_custom_registry_test.go` (package `xcl_test`): `TestCustomRegistryAndPluginStarterAreUsed` defines its own `Registry` and `Plugin` types that wrap `plugins.NewDirectPluginHost`, records that `Start` was called, and applies a block type of the plugin.

**Complexity**: High
**Token estimate**: ~130k tokens
**Agent strategy**: Parallel analysis, sequential integration. One agent writes the options and `NewConfig` and gets the build green. Then 2-3 agents migrate disjoint sets of root test files in parallel, while one writes the new acceptance tests. Integration runs the full root suite sequentially.

### Task: Migrate e2e and plugin SDK example tests

**Requirement → repo:** regression coverage on the new API → xclconfig.

**File changes**:
- `e2e/helpers_test.go:35-89`:
  - `registerPlugins` becomes `newLocalRegistry() *registry.Local`, which registers `&inprocess.Plugin{}` and `externalPlugin`. Drop the `t.Cleanup` that ranged over `GetPluginHosts`.
  - The kube types at `:35-39` become `xcl.WithType` options.
  - `newConfig(t, r, ...)` at `:89` takes options instead of a registry.
- `e2e/plugin_errors_test.go:17-35`: build `local.RegisterExternalPlugin(missing)` and assert `ErrPluginLoad`, the path and `local` in the message.
- `e2e/{destroy,diff,encode,events,fixtures,kube_helpers,plugin_encode,plugin_events,plugin_helpers,plugin_silence,plugin_state,sensitive,silence}_test.go`: mechanical migration. Any `rejected` event assertion in `e2e/plugin_events_test.go` is removed or changed to expect a failed load.
- `e2e/COVERAGE.md`: the row text changes in the in-repo guides task.
- `plugins/example/apply_test.go:111`, `plugins/example/sensitive_test.go:59` and `plugins/example/e2e_test.go:579-600,744-775`:
  - Use `registry.NewLocal()` and `xcl.WithRegistry`.
  - The two load-event tests (`:588`, `:757`) build a catalog through a Config with an event recorder, or use `internal/catalog` directly, since this package is in the main module.
  - Drop the `GetPluginHosts` cleanup at `:767`.

**Complexity**: Medium
**Token estimate**: ~50k tokens
**Agent strategy**: 2 parallel agents, one for `e2e/` and one for `plugins/example/`.

### Task: Rewrite the configuration-only and plugin examples

**Requirement → repo:** "Examples use the new API" → xclconfig (`example/*` modules).

**File changes**:
- `example/configonly/main.go`: the starting point is the user's repaired version; `465566f` left it broken. `loadConfig(dir string, options ...xcl.ConfigOption)` builds `xcl.NewConfig` with these options:
  - `xcl.WithType(&resources.ConfigMap{}, "config_map")`
  - `xcl.WithType(&resources.Secret{}, "secret")`
  - `xcl.WithType(&resources.Deployment{}, "deployment")`
  - `xcl.WithType(&resources.Service{}, "service")`
  - `xcl.WithType(&resources.Ingress{}, "ingress")`
  - the caller's options
  
  Then it calls `Apply` and `Decode`. `run` drops `newStateKey`, `mask`, `os.MkdirTemp`, `WithStatePath` and `WithStateMask`, and passes `xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv()))` and `xcl.WithEventData(xcl.EventDataProcessed)`. Update the package doc comment, which says state is kept encrypted in a temporary directory, to say the example keeps no state. Remove the `crypto/rand`, `mask` and `plugins/registry` imports. If `ingressRoutes` (formerly `routes.go`) is missing, STOP and ask the user.
- `example/configonly/*_test.go` and `smoke_test.go`: update the `loadConfig` call sites to the new signature.
- `example/plugin/main.go:145-245`:
  - `newConfig(handler xcl.EventHandler, dockerPlugin, stateDir string)` builds `local := registry.NewLocal(); local.RegisterPlugin(&template.TemplatePlugin{}); local.RegisterExternalPlugin(dockerPlugin)` and calls `xcl.NewConfig(xcl.WithRegistry(local), xcl.WithStatePath(stateDir), xcl.WithEventHandler(handler), xcl.WithEventData(xcl.EventDataProcessed))`.
  - Remove the registry from `applyCommand` (`:152-157`), `statusCommand` (`:170`), `inspectCommand` (`:186`) and `destroyCommand` (`:206-208`), and update their comments.
  - Change the import from `plugins/registry` to `github.com/jumppad-labs/xcl/registry`.
- `example/plugin/main_test.go:22,81,102,189,333`: change to `newConfig(nil, dockerPlugin, stateDir)`. Line 189's missing-plugin test also asserts that the error names `local`.
- `example/prettylog/prettylog_test.go:219-242`: `RegisterType`, `RegisterPlugin` and `WithPluginRegistry` become `WithType` and `WithRegistry`.
- `go mod tidy` in each example module if imports change.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: 2 parallel agents, one for `configonly` and one for `plugin` + `prettylog` tests.

### Task: Untrack the stray binary and ignore example builds

**Requirement → repo:** "No committed build outputs" → xclconfig.

**File changes**:
- `configonly` (repo root, the 24 MB ELF from 51f1c0b): `git rm --cached configonly` and delete the file.
- `.gitignore`: add `/configonly` and per-example package binaries `example/configonly/configonly`, `example/plugin/plugin` and `example/prettylog/prettylog`, next to the existing `/example/example` and `/example/main` stray-build block. Keep the existing `example/*/build/`.

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Update in-repo guides to the new API

**Requirement → repo:** "Documentation uses the new API", the in-repo portion and an assumption → xclconfig.

**File changes**:
- `README.md`: there are 17 mentions. Rewrite the quick start to the configuration-only `WithType` form (design § What the result looks like) and the plugin form with `registry.NewLocal` + `WithRegistry`. Add a short "configuration only, no state" note and a "registration problems" note: panics for code mistakes, `PluginLoadError` or `TypeNameClashError` from the first operation.
- `docs/overview.md:23,38,155`: rename `PluginRegistry (plugins/registry)` to the internal catalog plus public `registry`, and change `WithPluginRegistry` to `WithRegistry`/`WithType`.
- `docs/plugins.md:64,197,201,212,310,372-412`: rewrite to `WithType`, the registries, the load-time clash rules and "any failed plugin fails the load". Update the `plugin_registry.go` links to `internal/catalog/catalog.go`.
- `docs/state.md:10,38,122-148,200,241-259`: change to `c.EncodeSavedEntity(record)` and catalog wording.
- `docs/parser-lifecycle.md:183-193`: change `*registry.PluginRegistry` to `*catalog.Catalog`.
- `docs/README.md:37`: change the package table row to `internal/catalog/` and `registry/`.
- `e2e/COVERAGE.md`: 6 mentions, updated to the renamed tests and helpers.
- `CHANGELOG.md`: historical entries are left untouched. The new entry is written by the implement workflow's changelog step.

**Complexity**: Low
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Update the documentation site to the new API

**Requirement → repo:** "Documentation uses the new API" → xcl-website (root `/home/nicj/code/github.com/jumppad-labs/xcl-website`).

**File changes**:
- `xcl-website:src/pages/index.mdx:92-97`: replace the hero `main.go` snippet with `local := registry.NewLocal()`, `local.RegisterPlugin(...)` and `xcl.NewConfig(xcl.WithRegistry(local), xcl.WithEventHandler(...))`. This also fixes the old argument order.
- `xcl-website:src/pages/index.mdx:170-174`: the configuration-only card says "`WithType` declares a plain Go type…, no state needed".
- `xcl-website:src/pages/events.mdx:31-33,80-81,209-211`: drop `WithPluginRegistry(r)`.
- `xcl-website:src/pages/events.mdx:130,256-258`: change to `Event.Entity()` / `c.EncodeSavedEntity`.
- `xcl-website:src/pages/events.mdx:146`: the `discover` row reads "searching the directories given to a local registry's `RegisterPluginDirectory`".
- `xcl-website:src/pages/events.mdx:241-252,266-267`: refresh the prettylog snippet and its call from the source.
- `xcl-website:src/pages/plugin-logging.mdx:190-263`: replace "Registering plugins" with a short summary that links to the new Registries page. "When a plugin fails to load" names the registry and says that any failing plugin, including a discovered one, fails the load. "Type name clashes" says clashes are reported at load and a duplicate `WithType` panics.
- `xcl-website:src/pages/configuration-text.mdx:20,58-71,170`: change to `c.EncodeSavedEntity(event.Data)` / `Event.Entity()`. The "needs the registry" wording becomes "resolved through the Config".
- `xcl-website:src/pages/examples/configuration-only.mdx:22-27,324-399,491,632`: refresh `loadConfig`/`run` from `example/configonly/main.go` byte for byte. Remove the state, key and registry prose, and say that no state is kept.
- `xcl-website:src/pages/examples/plugins.mdx:527-692`: refresh `newConfig` and the command snippets from `example/plugin/main.go` byte for byte. The prose covers the local registry.
- `xcl-website:src/pages/registries.mdx` (new, `layout: ../layouts/Shell.astro`, Hero/Prose components as on the sibling guides), with these sections:
  - Declaring types (`WithType`)
  - Configuration only, without state
  - Local registries (`RegisterPlugin` / `RegisterExternalPlugin` / `RegisterPluginDirectory` / `PluginPattern`)
  - Several registries and load order
  - How problems are reported (panic vs `PluginLoadError`)
  - Clashes
  - Writing your own registry
- `xcl-website:src/components/Nav.astro:8-28`: add `{ label: 'Registries', href: '/registries/' }` to the Guides children.
- `xcl-website:README.md`: add the page to its pages table, if it has one.
- Verify with `npm ci`, `npm run build` and `npx astro check`, with 0 errors.

**Complexity**: Medium
**Token estimate**: ~60k tokens
**Agent strategy**: 2 parallel agents. One writes the new Registries page and nav; the other does the existing-page edits and snippet refresh. Then run the build and check.

## Testing Strategy

Per task:
- **Event entity accessor**: root-package Config tests for typed entity, masked sensitive values, no-data nil and raw data; `events` tests for JSON without the decoder and for data without a decoder failing.
- **Encode saved data through the Config**: equality with `EncodeEntity`, a plugin type on a fresh Config; the 30 migrated callers act as regression tests.
- **Pretty log handler without a catalog**: the handler with two arguments writes configuration beneath the success line when driven by a real Config; the existing nil-registry tests become the "no configuration block" cases.
- **Public registry package and local registry**: order, custom pattern, default pattern, `~` expansion, nil panic, missing path not failing, plus the moved discovery tests.
- **Plugin load and type clash errors name registries**: message and `errors.Is`/`errors.As` tests for each error type.
- **Internal type catalog loading from registries**: one panic test per former `RegisterType` error test (21), load-time clash and form tests, load order by events, and a discovered plugin failing the load; the parser, saved-entity and state suites act as regression.
- **Config options declare types and add registries**: the acceptance tests listed in its technical notes (no state, panics, missing binary, two registries, order, failing plugins, every duplicate case, custom registry); the whole root suite acts as regression.
- **Migrate e2e and plugin SDK example tests**: the existing suites green; the missing-plugin test also asserts the registry name.
- **Rewrite the configuration-only and plugin examples**: each example module's tests and smoke tests; the plugin example's missing-plugin test asserts `local`.
- **Untrack the stray binary**, **Update in-repo guides** and **Update the documentation site**: no tests by convention. These are manual items in the implementation test plan, plus the website `npm run build` and `npx astro check`.

The success metrics and manual reviews are mapped in plan.md § Testing Approach. Two metrics and every manual review are flagged "Manual — captured in the implementation test plan".

## Project References

- Spec `20261008071608-eb05cae0-config-and-plugin-registries` (via `spektacular spec file read`).
- Design `plugin-registries.md` from the `design` source (via `spektacular design read`). It is binding, except for the user's planning decision that any plugin failure fails the load.
- Knowledge (xclconfig):
  - `conventions/shared-errors-package.md`
  - `conventions/project-structure.md`
  - `conventions/testing-and-mocking.md`
  - `conventions/shared-test-helpers.md`
  - `gotchas/example-modules-cannot-import-internal.md`
- Knowledge (xcl-website): `gotchas/no-redirects-on-page-removal.md`.
- Repo roots:
  - xclconfig `/home/nicj/code/github.com/jumppad-labs/xcl`
  - xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The two High tasks, the internal catalog and the Config options, carry most of the mechanical test migration. Split them by file sets across parallel agents, with a single integrator running the build and tests.

## Migration Notes

This is a breaking public API change with no shims. The implement workflow's changelog entry lists the breaking changes:
- `plugins/registry` is removed: `PluginRegistry`, `NewPluginRegistry`, `RegisterPluginWithPath` (now `RegisterExternalPlugin`), `DiscoverPlugins` (now `RegisterPluginDirectory` plus `PluginPattern`) and `CastResourceTo`.
- `WithPluginRegistry` is removed, replaced by `WithType` and `WithRegistry`.
- `EncodeSavedEntity` is now a `Config` method.
- `prettylog.Handler` takes two arguments.
- `RegisterType` mistakes now panic.
- A discovered plugin that fails now fails the load.
- `PluginLoadError` gains `Registry`.
- `TypeNameClashError` and `TypeFormError` move to `xcl`/`errors`, and the clash error gains fields.

## Performance Considerations

- `Event.Entity()` decodes on each call, and only when called, so handlers that never call it pay nothing.
- Each Config now starts its own plugin hosts instead of sharing them through a shared registry. That is a minor cost, and only for programs that build several Configs.
