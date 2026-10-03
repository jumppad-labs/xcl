---
created_date: "2026-10-03"
document_status: final
closed_date: "2026-10-03"
---

# Research: 20261003081552-e1e07cbe-config-decode

## Alternatives considered and rejected

- **Call the generic `all[T]` per field via reflection.** Go cannot instantiate a generic function from a `reflect.Type` at run time, so `Decode`, which only learns `T` from a field's type, cannot call `all[T]` (`query.go:312`). Rejected as impossible, not merely awkward.
- **Re-decode the configuration files into the target with the HCL decoder.** This re-reads files the configuration already processed, would produce copies rather than the configuration's own instances (breaking "same instances", spec requirement 4), and would bypass reference resolution done during the DAG walk (`internal/parser/parser.go`). The spec's technical approach also prefers the already-parsed blocks. Rejected.
- **JSON round-trip of each entity into the field type.** This is what `As` falls back to (`query.go:85`, `schema.UnmarshalUntyped`). Used for every field, it yields copies, so a change through the filled structure is not visible through `Find`. Rejected for registered types. It survives only as the shared fallback inside the conversion helper, unchanged from `As`.
- **Duplicate the scan in `Decode`, with its own loop over `c.Entities()` matching `meta.Type` / `meta.Subtype`.** This is the drift `all[T]` was written to avoid (`query.go:324-326`: "deliberately goes through the kind lookup rather than scanning again, so the two cannot disagree"). Rejected: spec requirement 3 demands the two "can never disagree".
- **Struct tags to select blocks (e.g. `xcl:"server"`).** Ruled out by a spec constraint ("No struct tags or other annotations are read").
- **Generic method `c.Decode[T]`.** Unnecessary, since the target is `any`. A non-generic method compiles on Go 1.25 (`go.mod:3`), so it needs no `query_methods_go127.go`-style build constraint (`query_methods_go127.go:1`).

## Chosen approach — evidence

- The type → address path resolution already exists for a `reflect.Type`: `PluginRegistry.TypePath(t reflect.Type)` (`plugins/registry/plugin_registry.go:737`). It strips pointers, matches `info.Prototype`'s type, and reports false for plugin-provided types (nil Prototype). Using it for each field's element type gives exactly the path `all[T]` uses (`query.go:320`).
- `findByType[T]` (`query.go:179-217`) is the single scan: `addressable`, `typeable`, a loop over `c.Entities()` filtering by `meta.Type`/`meta.Subtype`, then `As[T]`. Its generic parts are only the `addressable[T]` check and the `As[T]` conversion. Both can be expressed over a `reflect.Type`. That makes a non-generic core (`matching path → []any`, plus a conversion over `reflect.Type`) shareable by `findByType[T]` and `Decode`, so both go through one implementation.
- `As[T]` (`query.go:68-95`) returns the stored `*T` as-is when the entity already is one. That is how "same instances" holds for registered types (verified: `TestAllReturnsWhatTheEquivalentKindLookupReturns` uses `require.Same`, `query_all_test.go:23-38`).
- `findOne[T]` (`query.go:219-235`) builds `&xclerrors.NotUniqueError{Segments: path, Count: n}`. `Decode` returns the same error type for a `*T` field with more than one match. For zero matches, `Decode` leaves the field nil instead of returning `NotFoundError` (spec).
- `addressable[T]` (`query.go:285-296`) uses `types.GetMeta(&zero)`. The reflect equivalent is `types.GetMeta(reflect.New(t).Interface())`. A nested-only type such as `registered.Timeouts` also fails `TypePath` (never registered), so it is left untouched.
- The `NewConfig` default registry is always non-nil (`config.go:162-164`), so `Decode` before `Apply` sees an empty `c.entities` (`config.go:150-153`) and fills collection fields with empty slices and single fields with nil. This matches `All` on an unapplied config.
- Entity order: `Apply` adopts `newState.GetResources()` (`config.go:304`), built by `AppendResource` in parse order (`internal/parser/parser.go:531`, `internal/parser/entities.go:39`). Spike (below) confirmed that `server.vault_current` then `server.vault_arc` come back in declaration order, which is not alphabetical.
- **Spike (throwaway test, deleted, 2026-10-03):** registered `server` (bare) and `mount` (bare) types. A nested `classic { server = server.vault_current }` was decoded into a `*spikeServer` field and `arc { server = server.vault_arc }` into a `spikeServer` value field. Both arrived with `location` and `type` filled in. **The spec's known risk does not reproduce. Coverage is needed, but a fix is not expected.**
- Sentinel-and-detail error convention: `errors/query_errors.go:23-56,150-159`, re-exported in `config.go:33-63,91-104`. The knowledge entry `conventions/shared-errors-package.md` requires new errors in `errors/`, pointer receivers, `Unwrap` to the sentinel, and a root re-export.
- Knowledge entry `architecture/config-is-the-public-query-surface.md`: `Config` is the only public query surface, so `Decode` belongs on `*Config` in package `xcl`, not in `state`.

## Files examined

- `xclconfig:query.go:29-36` — `Find`/`find`: package function delegating to an unexported impl, the pattern to copy for `Decode`/`decode`.
- `xclconfig:query.go:68-95` — `As[T]`: identity for `*T`, `convertible` check, then JSON copy.
- `xclconfig:query.go:103-127` — `convertible[T]`: named-struct mismatch refusal.
- `xclconfig:query.go:165-235` — `FindByType`/`FindOne`/`findByType`/`findOne`: the scan and the not-unique error.
- `xclconfig:query.go:243-283` — `typeable`: path validation against the registry.
- `xclconfig:query.go:285-296` — `addressable[T]`: nested-block refusal via `types.GetMeta`.
- `xclconfig:query.go:308-327` — `All`/`all`: `TypePath`, then delegates to `findByType`.
- `xclconfig:query_methods_go127.go:1-47` — method forms of the generic lookups, `//go:build go1.27`.
- `xclconfig:config.go:16-104` — sentinel and alias re-exports.
- `xclconfig:config.go:109-120` — `Config` struct (`entities`, `pluginRegistry`).
- `xclconfig:config.go:149-168` — `NewConfig` always sets a registry.
- `xclconfig:config.go:278-317` — `Apply` adopts parser state order.
- `xclconfig:plugins/registry/plugin_registry.go:90-130` — `RegisterType` requires a pointer to a struct embedding `ResourceBase`.
- `xclconfig:plugins/registry/plugin_registry.go:729-765` — `TypePath(reflect.Type)`.
- `xclconfig:errors/query_errors.go:1-160` — sentinels and detail types for lookups.
- `xclconfig:types/resource.go:5-86` — `Meta`, `ResourceBase` (`Disabled`, `Meta.ID`).
- `xclconfig:internal/parser/entities.go:33-44` — state keeps entities in append order.
- `xclconfig:internal/test_fixtures/registered/types.go` — `Database` (nested `*Timeouts`), `App`, `Consumer`, `Cache` (bare), `Server` (`server "big"`), `Timeouts` (nested only).
- `xclconfig:internal/test_fixtures/config/registered/{basic,bare,disabled,subtyped}` — existing fixtures reused by the Decode tests. `basic` declares two databases (one inside module `shared`), one app and one consumer; `disabled` has one disabled database; `bare` has `cache.main` and a database.
- `xclconfig:internal/test_fixtures/plugin/structs/container.go:19` — `NetworkObj Network` is the only existing whole-block reference field, but it is a plugin `resource` type with value form only, so it is unsuitable for the reference case.
- `xclconfig:query_all_test.go:1-120` — test style: prose comment blocks, one behaviour per test, `require.Same`.
- `xclconfig:query_test.go:28-66` — `setupFindConfig`: registers database/app/consumer/cache and applies `registered/basic`.
- `xclconfig:query_by_type_test.go:25-55` — `setupBareTypeConfig`.
- `xclconfig:query_setup_test.go:20-56` — `setupQueryConfig` (plugin types).
- `xclconfig:query_equivalence_go127_test.go` — tests asserting the method and function forms agree.
- `xclconfig:readme_test.go:1-80` — README/CHANGELOG content guards via `require.Contains`.
- `xclconfig:README.md:68-119` — "Configuration only" example section (HCL only, no Go lookup).
- `xclconfig:README.md:309-347` — "Querying a configuration": code block of lookups.
- `xclconfig:README.md:399-416` — "Two spellings, one implementation" (Go 1.27 note).
- `xclconfig:CHANGELOG.md:1-30` — one `## <spec-name>` section per feature, newest first.
- `xclconfig:example/configonly/main.go:61-139` — registers 4 resource types; `printDeployments` (`FindByType`, :141) and `printRouting` (`Find` Service/Ingress, :193-201). ConfigMap is never read back.
- `xclconfig:example/configonly/main_test.go:541-597` — `UsesPortableLookupForm` guard: lookups must be `xcl.`-qualified, and at least one must exist.
- `xclconfig:example/appconfig/main.go:68,92` — single registered type, one `Find`. Not a several-types assembly.
- `xclconfig:example/plugin/main.go:133-178` — several types, but all plugin-provided. `TypePath` cannot reach them, so `Decode` cannot fill them.
- `xclconfig:static_examples_test.go:17-22` — globs `example/*/main.go` for cross-example conventions.
- `xclconfig:.github/workflows/go.yml` — tests on Go 1.27.0; `build-minimum-go` builds and vets only, on go1.25.0.
- `xcl-website:src/pages/examples/configuration-only.mdx:1-372` — Hero/Prose/CtaBanner. The program section is at :216-288 (lookup snippet :266-288); fenced code blocks carry `title="<repo path>"`. The Go snippets at :224-256 are stale (old `run` signature, old `RegisterType` argument order, no `WithEventData`).
- `xcl-website:src/pages/index.mdx:176-179` — "Typed queries" feature card.
- `xcl-website:Makefile` — `build`, `check` (`npx astro check`). There are no content tests.

## External references

- Go `reflect` package (`reflect.PointerTo`, `Value.Set`, `Value.CanSet`, `StructField.IsExported`). The whole feature is standard-library reflection, which is consistent with `conventions/dependencies.md`.
- Go spec, "Method declarations": a package-level function and a method may share the name `Decode`, as `Find` already does here.

## Prior plans / specs consulted

- `20260921093100-query-api-v2` (plan, historical). It established the `Find`/`FindByType`/`FindOne`/`All` surface, `TypePath(reflect.Type)`, the delegate-to-one-implementation rule for the two spellings, the Go 1.27 build constraint for generic methods, and standard-library-only reflection. `Decode` follows the same pattern.
- `20260919120639-config-only-types-and-examples` (spec list only). It is the origin of `RegisterType` and the configonly example that `Decode` will demonstrate.

## Open assumptions

- Registered-type entities are always stored as `*T` of the registered prototype's type, so the identity branch of the conversion always fires for them. The spike and `require.Same` tests support this. If a registered type is ever found stored differently, the shared conversion's JSON fallback would produce a copy, and "same instances" would fail. **STOP and ask.**
- Declaration order of `c.Entities()` is parse order across a single file. Across several files it follows the parser's file order. That order is not documented as a guarantee; the spec's ordering criterion is tested within one file.
- `Decode` with `WithVariables` / `env()` behaves exactly as `All` does. No separate handling is needed.
- The plugin example (`example/plugin`) is outside the "every shipped example program" success metric, because its types are plugin-provided and cannot be decoded. Only `example/configonly` assembles several *registered* types.

## Drafting assumptions

### Chosen direction: shared non-generic lookup core (architecture)
- **Decision**: Split `findByType[T]`, `all[T]`, `addressable[T]` and `As[T]` into non-generic cores over `reflect.Type`. The generic functions become thin wrappers, and `Decode` calls the cores per field. `Decode` is two-phase (resolve all fields, then assign). It has a new `ErrInvalidDecodeTarget`, returns `*NotUniqueError` unwrapped, and has a non-generic method that is not version-gated.
- **Rationale**: Spec requirement 3 says Decode and All "can never disagree". Sharing the code guarantees that, and `all[T]` already reasons the same way (`query.go:324-326`). Two-phase assignment gives "target unchanged" on every error, not just unusable targets.
- **Rejected**: Decode-only reflection scan kept equal by tests (Low effort, but agreement holds only by testing). Per-type decoder closure captured at registration (High effort, needs a generic RegisterType API change). Re-decoding files with HCL (produces copies and re-reads files).

### Plugin example excluded from the "every example" success metric (discovery)
- **Decision**: Only `example/configonly` is converted to use `Decode`. `example/plugin` is not.
- **Rationale**: `example/plugin` assembles plugin-provided types, which `TypePath` cannot reach, so `Decode` leaves those fields untouched. The metric can only apply to registered types. `example/appconfig` uses a single type.
- **Rejected**: Converting the plugin example. It is not possible without extending `Decode` to plugin types, which is out of the spec's scope.

### Known reference risk treated as coverage-only (discovery)
- **Decision**: Plan tests for whole-block references into `T` and `*T` nested fields of a registered non-resource type, with no code fix.
- **Rationale**: A throwaway spike (2026-10-03) showed both forms already decode correctly.
- **Rejected**: Planning a speculative fix.

### NotUniqueError returned unwrapped, no field context (architecture)
- **Decision**: For a `*T` field with several matches, return the exact `*NotUniqueError` `findOne` builds, without wrapping it in field-name context.
- **Rationale**: The spec requires "the same not-unique error the existing single-block lookup returns", and that the method and function forms return equal errors. The `Segments` already name the type.
- **Rejected**: Wrapping with `fmt.Errorf("field %s: %w")`. `errors.Is` still matches, but the error is no longer equal to FindOne's.

### All-or-nothing assignment (architecture)
- **Decision**: No field is assigned unless every field resolved.
- **Rationale**: The spec requires an unchanged target for unusable targets. Extending that to not-unique failures avoids half-filled structs at no cost.
- **Rejected**: Assigning as fields are visited, which leaves partial state on error.

### Embedded and named field shapes (architecture)
- **Decision**: Any exported, settable field whose type has the shape `[]*T` (including a named slice type) or `*T` is eligible, embedded or not. Unexported fields are skipped.
- **Rationale**: The spec says fields are matched by type alone. Reflection cannot set unexported fields.
- **Rejected**: Special-casing embedded fields.

### Conventions selected (architecture)
- **Decision**: Applied: shared-errors-package, testing-and-mocking, test-state-from-real-apply, code-style, dependencies, glossary/entity. Dropped: database (no DB), development-standards logging (Decode is a pure in-memory lookup that emits no events, like `All`), patterns-and-architecture (no services or handlers), project-structure (the library's public surface is the root package by existing design), never-modify-dependencies (nothing under `internal/xcl` or the module cache is touched).
- **Rationale**: Only conventions that drive a concrete choice in this feature are kept.
- **Rejected**: Listing every convention.

### Website scope (architecture)
- **Decision**: Update `examples/configuration-only.mdx` only. Where the Decode snippet replaces or sits within the stale program snippets (:224-256), bring those snippets in line with the current `example/configonly/main.go`. Leave `index.mdx` alone.
- **Rationale**: The spec names the README and the configuration-only page. Showing Decode next to a snippet that no longer compiles would mislead.
- **Rejected**: A site-wide refresh of stale snippets, which is out of scope.

### New error type InvalidDecodeTargetError (data_structures)
- **Decision**: Add a new sentinel `ErrInvalidDecodeTarget` with an `*InvalidDecodeTargetError{Type reflect.Type}` detail, rather than reusing an existing error.
- **Rationale**: None of the existing lookup sentinels describes an unusable target. The convention requires a sentinel plus a detail type, and `encoding/json`'s `InvalidUnmarshalError` is the standard-library precedent.
- **Rejected**: A plain `fmt.Errorf`, which callers cannot match. Reusing `ErrTypeMismatch`, which means a different thing.

### Reuse existing test fixtures for Decode (walkthrough, user decision)
- **Decision**: The `Decode` tests reuse the existing registered types (`Database`, `App`, `Consumer`, `Cache`) and fixtures (`registered/basic` via `setupFindConfig`, `registered/bare` via `setupBareTypeConfig`, and `registered/disabled`). The only additions are two ordering fixture files with cache blocks in opposite orders, plus one test-local `cacheClient` type with a `cache_client` fixture for the whole-block reference case.
- **Rationale**: The user pointed out at walkthrough that existing resources cover the tests. Checking confirmed this for every criterion except two. Declaration order needs a same-type pair that is not split across the root and a module. Whole-block references to a registered non-`resource` type need a shape no existing type has.
- **Rejected**: New shared fixture types `Endpoint`, `Mount`, `MountTarget` and `MountMirror` (the original draft). Reusing the plugin `structs.Container.NetworkObj` was also rejected: it is a plugin `resource` type with only the value form.

### Example uses the method form c.Decode (implementation_detail)
- **Decision**: `example/configonly` and the docs call `c.Decode(&cfg)` (the method), not `xcl.Decode(c, &cfg)`.
- **Rationale**: The "examples use the function form" rule exists because generic methods need Go 1.27. `Decode` is not generic, so the method form compiles on Go 1.25, and it is the idiomatic spelling the user asked for.
- **Rejected**: The function form in examples, which would follow a rule whose reason does not apply here.

### Unreachable field types are a no-op, not NotRegistered (implementation_detail)
- **Decision**: A `[]*T`/`*T` field whose T is not reachable via `TypePath` (unregistered, plugin-provided or nested-only) is skipped silently.
- **Rationale**: The spec requires "Other fields are left alone", and a field of unregistered type U must keep its value.
- **Rejected**: Surfacing `ErrNotRegistered` the way `All` does.

### Config-only example drops all lookups (tasks)
- **Decision**: The example's Service and Ingress become `*T` fields of the decoded struct, so `main.go` makes no `Find` calls. The portable-lookup guard loses its `NotZero` assertion, and new guards require one `Decode` call and zero lookups.
- **Rationale**: The configuration declares exactly one service and one ingress, so single-pointer fields express them directly. That meets success metric 3 in full.
- **Rejected**: Keeping `Find` for the service and ingress just to satisfy the existing guard's `NotZero`.

### Decode does not special-case a nil *Config (tasks)
- **Decision**: `xcl.Decode(nil, &cfg)` is documented as invalid but not checked, the same as `xcl.Find(nil, ...)`.
- **Rationale**: This is consistent with every existing lookup. The spec defines only target validity.
- **Rejected**: A new error case that the spec does not ask for.

## Rehydration cues

- `spektacular spec file read 20261003081552-e1e07cbe-config-decode`
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"conventions/shared-errors-package.md"}'`
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"architecture/config-is-the-public-query-surface.md"}'`
- Re-read `query.go` (whole file, 329 lines), `query_methods_go127.go`, `config.go:16-120`, `plugins/registry/plugin_registry.go:729-765`, `errors/query_errors.go`.
- Re-read `query_all_test.go`, `query_test.go:28-66` for test style and setup helpers.
- Re-read `example/configonly/main.go`, `example/configonly/main_test.go:541-597`.
- Re-read `xcl-website/src/pages/examples/configuration-only.mdx:216-290`.
