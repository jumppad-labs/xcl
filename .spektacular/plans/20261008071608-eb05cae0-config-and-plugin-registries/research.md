---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Research: 20261008071608-eb05cae0-config-and-plugin-registries

## Alternatives considered and rejected

- **Keep `plugins/registry.PluginRegistry` public, add `WithType`/`WithRegistry` beside it.** Rejected: the spec requires the catalog to leave the public API and `WithPluginRegistry` to be removed (spec "The catalog is no longer public"; design "What becomes internal").
- **Rename `plugins/registry` to a public `catalog` package.** Rejected for the same reason: no public catalog type may remain.
- **Put the new public `Registry`/`Plugin` contracts in `plugins/registry` (reuse the name).** Rejected: design "Open" says the old package moves to `internal/` or is renamed "so the two don't share a name"; the convention `conventions/project-structure.md` puts public packages at module top level → `github.com/jumppad-labs/xcl/registry`.
- **Registries start their own hosts (Registry returns `[]plugins.PluginHost`).** Rejected: design fixes `Plugins(ctx, emit) ([]Plugin, error)` + `Plugin.Start(emit)`, so the catalog decides start order, failure policy and clash checks in one place.
- **Report a plugin-vs-type clash at registration when the plugin already loaded (today's `RegisterType` → `checkType` against `pluginHosts`, `plugin_registry.go:305`).** Rejected: spec/design require every clash to surface from load, independent of order. `WithType` is collected by `NewConfig` before any load, so the clash path is load-only.
- **First-registry-wins precedence for duplicate block types.** Rejected by the user (spec Constraints "No precedence between registries").
- **`Event.Configuration()` returning HCL text.** Rejected in the spec: `events` cannot import the root package (import cycle), so `Event.Entity()` returns the Go value and callers encode it with `xcl.EncodeEntity`.
- **Exported `Decoder` field on `events.Event`.** Rejected in favour of an unexported field (ignored by `encoding/json` automatically) set through one exported method; see assumptions.
- **Share one loaded catalog across Configs (today's "Load once per registry, however many Configs share it", `plugin_registry.go:576`).** Rejected: the catalog becomes private to a Config and is built by `NewConfig`; a `registry.Registry` value may be given to several Configs but each Config starts its own plugin hosts.

## Chosen approach — evidence

- Catalog internals to move as-is: `plugins/registry/plugin_registry.go:28-62` (state), `:104` RegisterType validation, `:288` checkType, `:325` checkHostTypes, `:509` Use, `:587` Load (once, cached), `:607` load order (in-process, paths, discovered), `:687` loadDiscovered (reject + `rejected:true` event, `:777`), `:730` startExternal, `:64` `restartable` (Restart/Path) used by Use/stopPlugins.
- Discovery to move into the local registry: `plugins/registry/plugin_discovery.go:23` (default pattern `xcl-plugin-*`, `:25`), `:117` isPluginBinary, `:150` ExpandPluginDirectories (`~` and env expansion).
- Plugin hosts to wrap: `plugins/direct_plugin_host.go:29` `NewDirectPluginHost(emit, state, plugin)`, `:71` `PluginName`; `plugins/grpc_plugin_host.go:38` `NewGRPCPluginHost(emit, state)`, `:51` Start(path), `:61` Restart, `:75` Path, `:132` `PluginBinaryName`.
- Config wiring: `config.go:143-156` (field `pluginRegistry`), `:186-210` NewConfig (options applied in order, default registry), `:498` `run` (single place every operation's events pass through; `stream.Emit`), `:556` withPlugins (Activate + Use). `options.go:13` `ConfigOption func(*Config) error`, `:17` WithPluginRegistry.
- No-state already works: `config.go:353` Apply skips save when `stateStore == nil`; `config.go` Load returns nil without a store.
- Saved-data decode: `internal/savedentity/savedentity.go:55` `Decode(registry, data, ReadOptions{ForDisplay:true})` is what `encode.go:168` EncodeSavedEntity uses; same call powers `Event.Entity()`.
- Events: `events/events.go:67-112` Event struct has no JSON tags/MarshalJSON; an unexported field is skipped by encoding/json. Event data is built at `internal/parser/events.go:132`.
- Errors: `errors/plugin_load_error.go:20` `PluginLoadError{Plugin, Err}` (needs `Registry`); clash types `plugins/registry/errors.go:10` TypeNameClashError, `:28` TypeFormError (move to `errors` per `conventions/shared-errors-package.md`).
- Parser/savedentity dependency on the concrete type: `internal/parser/parser.go:92` `ParserOptions.PluginRegistry`, `:156` default, `:182,755,916,1518`; `internal/parser/callbacks.go:26,33` ProviderResolver/TypeRegistry interfaces; `internal/savedentity/savedentity.go:55,129`. Query uses `TakesSubtype/KnownType/TypePath` at `query.go:303,318,378`.
- Panic precedent: `gob.Register`, `http.ServeMux.Handle` (design "Registration never returns an error").

## Files examined

- `plugins/registry/plugin_registry.go:1-858` — catalog + loader; all behaviour to preserve.
- `plugins/registry/plugin_discovery.go:1-168` — directory search, pattern default, `~` expansion.
- `plugins/registry/errors.go:1-41` — TypeNameClashError, TypeFormError (value moves to `errors`).
- `plugins/registry/plugin_registry_test.go:146-613,970,984,1266` — RegisterType error tests that become panic tests; `:813` DiscoverPlugins test.
- `plugins/registry/plugin_discovery_test.go` (~40 calls) — discovery tests move with discovery into the local registry.
- `config.go:143-210,498-575` — Config fields, NewConfig, run, withPlugins.
- `options.go:1-173` — option shape; WithPluginRegistry to remove.
- `encode.go:142-197` — EncodeEntity / EncodeSavedEntity (becomes `(*Config).EncodeSavedEntity`).
- `events/events.go:67-112`, `events.go:1-40` — Event struct, root alias.
- `errors/plugin_load_error.go:8-31` — PluginLoadError.
- `internal/parser/parser.go:80-160` — ParserOptions.PluginRegistry and default.
- `internal/savedentity/savedentity.go:43-160` — Decode/DecodeAll.
- `example/configonly/main.go` — committed mid-rewrite and not compiling at `465566f` (user will fix it by hand); HEAD~1 version shows the full old setup (random AES key, temp dir, 5× RegisterType error checks).
- `example/plugin/main.go:152-241` — registry per command, `eventHandler(out, r)`, `newConfig(r, ...)` with RegisterPlugin/RegisterPluginWithPath error checks; `main_test.go:81,102,189,333`.
- `example/prettylog/prettylog.go:56-130` — Handler(w, level, reg); writeConfiguration uses EncodeSavedEntity; `prettylog_test.go` 13 Handler calls (6 nil, 7 registry).
- `e2e/helpers_test.go:35-89` — shared registerPlugins/newConfig helpers (GetPluginHosts only for cleanup Stop); `e2e/plugin_errors_test.go:17` missing-binary test.
- `.gitignore` — no rule covers `/configonly` or `example/*/<pkg>` binaries; root `configonly` is a tracked 24 MB ELF (commit 51f1c0b).
- Call-site counts outside the package: NewPluginRegistry ≈115 (5 production), RegisterType 70, RegisterPlugin 34, RegisterPluginWithPath 11, WithPluginRegistry 82, EncodeSavedEntity 30, prettylog.Handler 14, DiscoverPlugins 0, CastResourceTo 0. GetPluginHosts in tests at `config_plugin_loading_test.go:204,228`, `config_plugin_boundary_test.go:120,645`, `config_plugin_lifecycle_test.go:22`, `internal/parser/parser_plugin_test.go:34`, `plugins/example/e2e_test.go:767`; `Use(nil)` at `config_plugin_lifecycle_test.go:70-135`; `TypeNameClashError` via errors.As at `config_plugin_boundary_test.go:684`, `config_plugin_loading_test.go:268`.
- Repo markdown mentioning the APIs: `README.md` (17), `CHANGELOG.md` (12, historical — not rewritten), `docs/plugins.md` (11), `e2e/COVERAGE.md` (6), `docs/state.md` (5), `docs/overview.md` (3), `docs/README.md` (1).
- xcl-website:`src/pages/index.mdx:92-97,170-174` — hero snippet (old API, wrong arg order) and config-only card.
- xcl-website:`src/pages/events.mdx:31-33,80-81,130,146,209-211,241-267` — WithPluginRegistry, DiscoverPlugins in op table, prettylog Handler signature.
- xcl-website:`src/pages/plugin-logging.mdx:190-263` — Registering plugins / Loading / Failures / Type name clashes (current registry content).
- xcl-website:`src/pages/configuration-text.mdx:20,58-71,170` — EncodeSavedEntity(registry, ...).
- xcl-website:`src/pages/examples/configuration-only.mdx:22-27,324-399,491,632` — RegisterType + state setup snippets.
- xcl-website:`src/pages/examples/plugins.mdx:527-692` — newConfig(r, ...), RegisterPluginWithPath, eventHandler(stderr, r).
- xcl-website:`src/components/Nav.astro:8-28` — hard-coded nav (Guides children); `Makefile` `check` = `npx astro check`; `package.json` `build`.

## External references

- Go `encoding/gob.Register` and `net/http.ServeMux.Handle` — panic-on-programmer-error precedent cited by the design.
- `encoding/json` docs — unexported struct fields are never marshalled; a `func` field that *is* exported makes Marshal fail, which is why the decoder field must be unexported.

## Prior plans / specs consulted

- Design `design/plugin-registries.md` (binding) — option names, Registry/Plugin contracts, local registry calls, failure rules, what becomes internal; remote registry designed but not built.
- Plan `20260919120639-config-only-types-and-examples` — why PluginRegistry is the single type source; resolution order builtin → registered → plugin; examples share a `run(...)` with tests.
- Plan `20260922061954-event-based-logging` — lazy Load once, discover/load events, `rejected:true`, PluginLoadError + re-export, plugin log routing (loading vs active emitter), "an error returned is an error emitted".
- Plan `20261006071142-506b8289-e2e-suite-and-real-world-examples` — examples are separate modules; e2e runs each example's tests by name; smoke tests build with `-o` to a temp path (root `configonly` binary gotcha).
- Plans `20261006112023-aadf3c10-configuration-example`, `20261006112023-f7a185dc-docker-plugin-example` — website verification: `npm ci`, `npm run build`, `npx astro check` = 0 errors; snippets copied from example source byte-for-byte and checked by a script; leftover-reference grep as a manual check.
- Plan `20261007111826-cf3b66d8-diff-rendering-and-docs` — CHANGELOG left to the implement workflow's changelog step.
- Knowledge: `conventions/shared-errors-package.md`, `conventions/shared-test-helpers.md`, `conventions/testing-and-mocking.md`, `conventions/project-structure.md`, `gotchas/example-modules-cannot-import-internal.md`, xcl-website `gotchas/no-redirects-on-page-removal.md`.

## Open assumptions

- Every test in the main module (root, internal, state, e2e, plugins/example) can import an `internal/` catalog package; only `example/*` modules cannot.
- `savedentity.Decode(..., ForDisplay:true)` decodes both raw and processed event data into the registered Go type (both are JSON of the entity with `meta`).
- e2e `GetPluginHosts` cleanup is redundant because `Use`'s `done` stops external processes after every operation.
- The user will repair `example/configonly` (routes.go/routes_test.go deleted in `465566f`) before or during implementation; if `ingressRoutes` is still missing when the example task runs, STOP and ask.
- No external consumer outside this repo and its examples depends on `plugins/registry` (breaking is allowed by the spec).

## Drafting assumptions

### Chosen direction (architecture)
- **Decision**: Split PluginRegistry into an internal per-Config `internal/catalog.Catalog` (type resolution + once-per-Config load) and a new public `github.com/jumppad-labs/xcl/registry` package (Registry/Plugin contracts, InProcess, Executable, NewLocal with PluginPattern). WithType checks argument shape eagerly (panic at the call site); NewConfig registers types and panics on duplicates/form/builtin clashes; all environment failures and every block-type clash are reported from the catalog's load as typed errors naming plugin and registry. Event.Entity decodes on demand via an unexported decoder attached in Config.run.
- **Key design decisions**: catalog no longer shared between Configs (each Config starts its own hosts); discovered plugins that fail now fail the load (user decision); TypeNameClashError/TypeFormError move to `errors` and gain provider/registry fields; plugins/registry is deleted, not kept as a shim.
- **Rejected**: public catalog beside new options; contracts inside plugins/registry; registries starting hosts themselves (see research.md alternatives).

### Event decoder attachment API (architecture)
- **Decision**: unexported field on events.Event plus an exported `(Event).WithEntityDecoder(func([]byte) (any, error)) Event`; `Entity()` returns (nil, nil) when Data is empty and an error when Data is present but no decoder was attached.
- **Rationale**: events cannot import root and root cannot set an unexported field otherwise; unexported fields are skipped by encoding/json so serialisation is untouched.
- **Rejected**: exported `func` field (breaks json.Marshal and pollutes the struct); silent nil when undecodable (hides misuse).

### Registry.Plugins failure (architecture)
- **Decision**: an error returned by a registry's Plugins (e.g. unreadable plugin directory) fails the load as a *PluginLoadError with Registry set and Plugin empty; a plugin directory that does not exist stays a non-error (as today, plugin_discovery.go:81).
- **Rationale**: keeps every environment failure in one error type naming its registry.
- **Rejected**: a separate RegistryError type (more public surface for no caller need).

### Panic message format (architecture)
- **Decision**: panics are `fmt.Sprintf("xcl: ...")` strings naming the type key (e.g. `xcl: type "resource.postgres" registered twice`) or the registry/plugin.
- **Rationale**: matches gob.Register-style messages; tests assert with require.PanicsWithValue / require.Panics plus message contains.
- **Rejected**: panicking with typed errors (no caller recovers them).

### Event equality risk (architecture)
- **Decision**: accept that events carrying a decoder are no longer reflect.DeepEqual to literals; tests that compare whole events compare fields instead.
- **Rationale**: grep shows no test compares whole emitted Event values with require.Equal; recorder tests filter by fields.
- **Rejected**: stripping the decoder in recorders.

### Repo markdown docs updated too (discovery)
- **Decision**: `README.md` and `docs/*.md` in the xcl repo are updated to the new API alongside the website; `CHANGELOG.md` history is not rewritten.
- **Rationale**: the spec names the website, but leaving in-repo docs showing removed calls would contradict "Documentation uses the new API" in spirit; prior plans never rewrite historical changelog entries.
- **Rejected**: website only (leaves stale in-repo docs).

### configonly example baseline (discovery)
- **Decision**: the plan targets the example's final shape required by the spec; the user's broken WIP was committed as `465566f` at their request and they will fix it by hand.
- **Rationale**: user answer "I will fix this myself just commit it broken".
- **Rejected**: restoring routes.go from HEAD~1 in the plan.

### InProcess(nil) and Executable("") (data_structures)
- **Decision**: `registry.InProcess(nil)` panics; `Executable(path)` never panics, a bad path fails at Start (load).
- **Rationale**: spec lists a missing in-process plugin as a programmer error and a missing binary as an environment error.
- **Rejected**: panicking on an empty path (not in the spec's list).

### Load event meta gains "registry" (implementation_detail)
- **Decision**: load start/success/error events carry `registry` in Meta beside `plugin`; discover events carry `registry` too.
- **Rationale**: spec requires load-order to be observable per registry and failures to name the registry; events are how that is observed.
- **Rejected**: inferring registry from order alone (not observable for clashes).

### Milestone ordering (milestones)
- **Decision**: M1 Event.Entity + prettylog(w, level) + Config.EncodeSavedEntity on the old registry; M2 registries/WithType/internal catalog + all Go call sites incl. examples; M3 docs + repo hygiene.
- **Rationale**: M1 is independently shippable on today's API and removes prettylog's registry need before the catalog goes internal; M2 cannot be split further without leaving examples (separate modules) uncompilable.
- **Rejected**: registries before WithType (would need an interim public catalog); docs inside M2 (cross-repo, separate verification).

### Interim WithPluginRegistry(*catalog.Catalog) (tasks)
- **Decision**: the catalog task temporarily retypes WithPluginRegistry to the internal catalog so the root package compiles between tasks; the Config options task deletes it.
- **Rationale**: keeps every task independently buildable without a big-bang commit.
- **Rejected**: merging the catalog and options tasks (too large for one agent context).

### CHANGELOG entry left to the implement workflow (tasks)
- **Decision**: no task writes CHANGELOG.md; the implement workflow's changelog step writes the single entry with a Breaking list.
- **Rationale**: prior plans (diff-rendering-and-docs) left it to that step; spec says the changelog entry replaces a migration guide.
- **Rejected**: a dedicated changelog task (duplicates the workflow step).

### Registries guide as a new website page (tasks)
- **Decision**: new `/registries/` guide holds registries + configuration-only-without-state; plugin-logging.mdx links to it.
- **Rationale**: spec asks the site to explain config-only use without state and registries; plugin-logging already overloaded.
- **Rejected**: expanding plugin-logging.mdx only (buries config-only content under plugins).

## Rehydration cues

- `spektacular spec file read 20261008071608-eb05cae0-config-and-plugin-registries`
- `spektacular design read --data '{"source":"design","path":"plugin-registries.md"}'`
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`
- Re-read `plugins/registry/plugin_registry.go`, `plugins/registry/plugin_discovery.go`, `config.go:140-210,490-575`, `options.go`, `encode.go:140-200`, `events/events.go`, `example/prettylog/prettylog.go`, `example/plugin/main.go:140-245`.
- `grep -rn 'NewPluginRegistry\|WithPluginRegistry\|RegisterPluginWithPath\|EncodeSavedEntity' --include='*.go' .` for the migration surface.
