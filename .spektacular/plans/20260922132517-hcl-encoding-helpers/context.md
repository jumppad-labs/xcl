---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-23"
---

# Context: 20260922132517-hcl-encoding-helpers

## Current State Analysis

- **Encoder.** `internal/xcl/gohcl/encode.go:40-195` has an xcl-tag-driven encoder that nothing outside the package uses. It drops remain-embedded bases (every resource embeds `types.ResourceBase \`xcl:",remain"\``), silently skips interface fields (`:121`), writes computed fields and `null` for nil slices and maps, and panics on unrepresentable values. `hclwrite.File.Bytes()` already formats (`internal/xcl/hclwrite/ast.go:39-52`), and `hclwrite.Format` is at `public.go:41`.
- **Tags.** The decoder and encoder share `getFieldTags` (`internal/xcl/gohcl/schema.go:129-179`), which reads the `xcl` key (`internal/xcl/tags/tags.go:18`). Computed fields are marked `computed` (`tags.go:73`), and validation rejects them when configured (`internal/parser/validate.go:219`).
- **Plugin types.** Schema-built plugin types keep the plugin's full tags (`internal/schema/serialize.go:53`, `deserialize.go:144-207`) but hold named scalars as `interface{}` (`deserialize.go:301-316`).
- **Saved data.** The only JSON-to-typed path is inline in `FileStateStore.Load` (`state/file_state_store.go:67-138`). The state file is a `MarshalIndent`ed JSON array (`:162-187`). `registry.CreateResource` (`plugins/registry/plugin_registry.go:195-226`) types values, including plugin types after `Load` (`:442`). Its unknown-type miss is an unwrapped `fmt.Errorf` (`:318`).
- **Events.** Provider lifecycle events carry the pre-call resource (`internal/parser/lifecycle.go:132-140,400`). Registered and builtin types' create success carries no Data (`lifecycle.go:90-94`). Status is set after `callProvider` returns (`:152`). Destroy events carry the held resource (`internal/parser/callbacks.go:289-310`). `events/slog.go:19` never writes Data.
- **Examples.** Each `main` builds `prettylog.Handler(os.Stderr, level)` before `run` creates the registry (`example/plugin/main.go:59,75`, `example/configonly/main.go:428,443`, `example/appconfig/main.go:247,261`). The static tests in `static_examples_test.go:102-186` constrain that shape.
- **Public API.** Root re-exports shared errors (`config.go:33-80`), and `Config.pluginRegistry` is unexported (`config.go:87`).
- **Docs.** The README's Serialization/Deserialization sections (`README.md:1419-1440`) describe APIs that don't exist. The docs site's pages are MDX under `xcl-website:src/pages`, with navigation in `xcl-website:src/components/Nav.astro:8-25`.

## Per-Phase Technical Notes

### Phase 1.1: The internal encoder writes what the decoder reads

**Requirements covered:** Written as a person would write it; Nothing left out (as amended); Every known resource type (encoder side). **Repo:** xclconfig (`/home/nicj/code/github.com/jumppad-labs/xcl`).

**File changes**
- `internal/xcl/gohcl/encode.go`
  - Add `type EncodeOptions struct{ IncludeComputed bool }` and `func EncodeBody(val any, dst *hclwrite.Body, options EncodeOptions) error`.
  - Refactor `populateBody` (:89-195) into an error-returning `encodeBody(val reflect.Value, dst *hclwrite.Body, options EncodeOptions, path string) error`.
  - Keep `EncodeIntoBody` (:40-53) and `EncodeAsBlock` (:64-87) as wrappers that panic on error, preserving upstream behaviour for `encode_test.go`.
- Changes inside `encodeBody`:
  - **Remain recursion.** `getFieldTags` (`schema.go:129-179`) records `tags.Remain`. When the remain field is an anonymous embedded struct (or pointer to one), recurse into it in field order before the outer fields. Otherwise, e.g. an `hcl.Body` remain, skip it. This covers `types.ResourceBase \`xcl:",remain"\`` and `ContainerBase \`xcl:",remain"\`` (`internal/test_fixtures/plugin/structs/container.go:102,108`).
  - **Computed.** Skip attribute and block fields whose parsed tag has `Computed` (`internal/xcl/tags/tags.go:73`) unless `options.IncludeComputed`. Extend `fieldTags` to carry a computed set (or re-parse with `tags.Parse`), mirroring `internal/parser/computed.go:40`.
  - **Interfaces.** Replace the skip at `:121` (`exprType.AssignableTo(fieldTy)`). Skip only when the field's static type is exactly `hcl.Expression` or `*hcl.Attribute`. For other interface fields, take `fieldVal.Elem()`. Skip nil, otherwise encode by its dynamic type.
  - **Nil.** Skip nil pointers (already done), nil slices and nil maps, and nil interfaces, for attributes and blocks alike.
  - **Errors.** Replace the panics at `:47-48`, `:137`, `:144` and in EncodeAsBlock for non-struct block elements with `fmt.Errorf("%s: ...", path, ...)`. In `EncodeBody`, a single `defer recover()` converts panics from gocty and hclwrite into errors, e.g. `hclwrite/generate.go:303` for capsules and unknown values. Add a comment explaining it is a boundary for third-party-derived code.
  - Nested blocks recurse through the same `encodeBody` with the block path (`network[1]`) so options propagate. Today `EncodeAsBlock` is called at `:175-190`, which would lose the options.
- `internal/xcl/gohcl/encode_body_test.go` (new, package `gohcl`, testify `require`, one behaviour per test, modelled on `check_test.go`). Fixture structs at the top with `xcl` tags:
  - `TestEncodeBodyWritesEmbeddedRemainFields`
  - `TestEncodeBodyWritesInterfaceHeldValue`
  - `TestEncodeBodySkipsNilValues`
  - `TestEncodeBodyWritesZeroScalars`
  - `TestEncodeBodyOmitsComputedByDefault`
  - `TestEncodeBodyIncludesComputedWhenAsked`
  - `TestEncodeBodyWritesRepeatedBlocks`
  - `TestEncodeBodyReturnsErrorForUntypedMap`
  - `TestEncodeBodyReturnsErrorForTime`
  - `TestEncodeBodyReturnsErrorForNonStruct`
- `internal/xcl/UPSTREAM.md`: record the encoder modifications. Keep the MPL header, and add `// Modifications Copyright (c) Jumppad Labs` if it is not already on encode.go (it is today). The new test file carries the MPL header too, because it lives in the fork.

**Complexity:** Medium. **Token estimate:** ~40k. **Agent strategy:** Single agent, sequential. The encoder is one file and the tests depend on its shape.

### Phase 1.2: One reader for saved entity records, with clear errors

**Requirements covered:** Saved data to configuration text (decode half); Clear failure for data that can't be converted. Constraint: the stored format is unchanged. **Repo:** xclconfig.

**File changes**
- `errors/encode_errors.go` (new): sentinels `ErrUnregisteredType`, `ErrInvalidSavedData` and `ErrNotEncodable`, with doc comments naming who returns them and that they are matched with `errors.Is`. Detail types with pointer receivers:
  - `UnregisteredTypeError{Type string}`: `Unwrap() error`.
  - `InvalidSavedDataError{ID string; Err error}`: `Unwrap() []error{ErrInvalidSavedData, Err}`. ID is the record's `meta.id` when readable.
  - `NotEncodableError{What, Reason string; Err error}`: `Unwrap() []error`.
  - Follow `errors/query_errors.go:58-159` and `errors/plugin_load_error.go`. The `errors` import list stays thin (`conventions/shared-errors-package.md`).
- `config.go:33-80`: re-export the three sentinels as vars and the detail types as aliases, following `config.go:66,71-80`.
- `internal/savedentity/savedentity.go` (new): `func Decode(registry *registry.PluginRegistry, data []byte) (any, error)`.
  - Lift the loop body of `state/file_state_store.go:74-135` verbatim in behaviour:
    1. unmarshal into a map;
    2. read `meta`, `id`, `type`, `subtype` (overrides the type when non-empty) and `name`;
    3. `registry.CreateResource`;
    4. re-marshal and unmarshal.
  - A missing or ill-typed meta, type or name, or a JSON error, gives `*InvalidSavedDataError`. A `CreateResource` failure gives `*UnregisteredTypeError{Type: resourceType}`.
  - Builtins (variable/output/module/root) decode as today, because state needs them.
- `state/file_state_store.go:67-138`: the loop becomes: for each raw message, call `savedentity.Decode`. On `*UnregisteredTypeError`, record its Type. On `*InvalidSavedDataError`, record ID if non-empty, else `entry %d`. The aggregate `UnknownTypesError` (`state/errors.go:39-56`) and its sorting are unchanged.
- `internal/savedentity/savedentity_test.go` (new):
  - `TestDecodeReturnsTypedRegisteredEntity`
  - `TestDecodeReturnsTypedPluginEntity` (uses `parser.TestPlugin`, calls `registry.Load(nil)`)
  - `TestDecodeReturnsTypedBuiltin`
  - `TestDecodeFailsForUnregisteredType`
  - `TestDecodeFailsForInvalidJSON`
  - `TestDecodeFailsForRecordWithoutMeta`

  Records come from a real apply's state file (`conventions/test-state-from-real-apply.md`), reusing the pattern of `state/file_state_store_apply_test.go:42-110`. The invalid-data tests use literal non-record bytes, which the convention's exception allows because the test is about the format.
- The existing state tests (`state/file_state_store_test.go`, `state/file_state_store_apply_test.go`) must pass unchanged. That is the regression proof.

**Complexity:** Low–Medium. **Token estimate:** ~25k. **Agent strategy:** Single agent, sequential.

### Phase 1.3: Public conversion of an entity and of its saved data

**Requirements covered:** Resource to configuration text; Saved data to configuration text; Written as a person would write it; Nothing left out (amended: computed opt-in); Readable by xcl again; Printable or writable; Every known resource type (in-process, keyword, bare; the external plugin is in 3.2); Clear failure; One call per resource. **Repo:** xclconfig.

**File changes**
- `encode.go` (new, root, Apache):
  - `EncodeOption`, `encodeOptions{includeComputed bool}`, `IncludeComputed()`.
  - `EncodeEntity(entity any, options ...EncodeOption) ([]byte, error)`:
    1. `types.GetMeta(entity)` (`types/resource_helpers.go:28`). On error, `*NotEncodableError{What: "value", Reason: "not an entity"}`.
    2. Refuse `meta.Type` in `resources.TypeVariable/TypeOutput/TypeModule/TypeRoot` with a NotEncodableError naming the kind.
    3. Header: if `meta.Subtype != ""`, use `hclwrite.NewBlock("resource", []string{meta.Subtype, meta.Name})`, otherwise `hclwrite.NewBlock(meta.Type, []string{meta.Name})` (`internal/xcl/hclwrite/ast_block.go:30`).
    4. `gohcl.EncodeBody(entity, block.Body(), gohcl.EncodeOptions{IncludeComputed: ...})`. Wrap an error in NotEncodableError with Err set.
    5. Trim bookkeeping: `body.RemoveAttribute("meta")`. Remove `disabled` when the entity's `ResourceBase.Disabled` is false, and `depends_on` when it is empty. Read these through reflection on the embedded `types.ResourceBase` field, or a small `types` helper if one exists.
    6. `f := hclwrite.NewEmptyFile(); f.Body().AppendBlock(block); return f.Bytes()` (`ast.go:20-52`, already formatted).
  - `EncodeSavedEntity(registry *registry.PluginRegistry, data []byte, options ...EncodeOption) ([]byte, error)`: `registry.Load(nil)` (`plugins/registry/plugin_registry.go:442`, once-only and cached; wrap its error), then `savedentity.Decode`, then `EncodeEntity`. On any error, return nil bytes.
- Test fixtures:
  - `internal/test_fixtures/config/encode/main.xcl` (new): a `resource "database" "main"` with `port = 5432` and `timeouts { connect = 30 read = 60 }`, a bare `cache "main" { location = "us-east" }`, and an in-process `resource "container" "web"` with two `network` blocks. Check the TestPlugin container's required fields in `internal/test_fixtures/plugin/structs/container.go` and copy an existing working container from `internal/test_fixtures/config/simple/container.xcl`, including the network resources it references.
  - Registered via `RegisterType("database", &registered.Database{})`, `RegisterBareType("cache", &registered.Cache{})` and `RegisterPlugin(&parser.TestPlugin{})`, as in `config_events_test.go:46` (`applyQueryFixtureWithEvents`).
  - The provider-filled computed value is the network's `provider_id` (`internal/test_fixtures/plugin/structs/network.go:15`, `xcl:"provider_id,optional,computed"`). TestPlugin sets it to `id-<name>` on Create when `CreateSetsID` is on (`internal/parser/test_plugin.go:95,508-510`), and Init enables it. The fixture therefore declares a `resource "network" "main"` that the container's `network` blocks reference. The container's `assigned_address` (`container.go:58`) is also filled on attach (`test_plugin.go:512+`).
- `encode_test.go` (new, package `xcl`, testify `require`, one criterion per test). A helper applies the fixture with a `state.NewFileStateStore` in a temp dir and returns config, registry and state path, plus a helper that reads one state record by `meta.id` as `json.RawMessage`. Tests:
  - `TestEncodeEntityWritesResourceHeaderAndValues`
  - `TestEncodeSavedEntityMatchesEncodeEntity`
  - `TestEncodeEntityUsesConfigurationNames`
  - `TestEncodeEntityWritesRepeatedBlocks`
  - `TestEncodeEntityWritesBareHeader`
  - `TestEncodeEntityOmitsComputedByDefault`
  - `TestEncodeEntityIncludesComputedWhenAsked`
  - `TestEncodeEntityOutputValidates` (write each default text to a temp dir, then `NewConfig(WithPluginRegistry(fresh)).Validate(dir)`)
  - `TestEncodeEntityOutputParsesToSameValues` (Apply the written text with a fresh registry, find by address, compare configured fields)
  - `TestEncodeEntityIsDeterministic`
  - `TestEncodeEntityIsFormatterStable` (`hclwrite.Format(out)` equals out)
  - `TestEncodeEntityConvertsInProcessPluginType`
  - `TestEncodeEntityConvertsKeywordRegisteredType`
  - `TestEncodeEntityConvertsBareRegisteredType`
  - `TestEncodeSavedEntityFailsForUnregisteredType` (error names the type, `errors.Is(err, ErrUnregisteredType)`, nil text)
  - `TestEncodeSavedEntityFailsForInvalidData`
  - `TestEncodeEntityRefusesBuiltin`
  - `TestEncodeEntityWritesOneBlockPerCall` (three texts, each parsed with `hclsyntax.ParseConfig`, each with one block)
  - `TestEncodeSavedEntityLoadsUnloadedRegistry` (a fresh registry with TestPlugin registered and never loaded)
- `static_dependencies_test.go` has no rule on root imports of `internal/xcl/*` (checked), so it needs no change.

**Complexity:** Medium–High. **Token estimate:** ~60k. **Agent strategy:** Parallel analysis, sequential integration. One agent writes `encode.go` and fixtures. A second drafts tests against the agreed signatures. The main agent integrates and runs the suite.

### Phase 1.4: Output shows only what the user wrote, and marks what the provider filled in

**Requirements covered:** Written as a person would write it (display half). Replaces the descoped read-back requirement with a cleaner rule: the text shows what a person set, nothing xcl or a provider recorded. **Repo:** xclconfig.

**Background.** A struct-valued attribute (not a block) is handed whole to `gocty.ImpliedType`/`ToCtyValue`, which builds a cty object from every `xcl`-tagged field. The encoder's computed skip and the root wrapper's `meta` trim both work on fields they walk directly, so neither reaches inside that object. `ContainerBase.NetworkObj Network` (`internal/test_fixtures/plugin/structs/container.go:20`) is the case in the repo: it is a value type, never nil, and encodes as an object carrying `meta`, `depends_on = null`, `disabled` and the computed `observed`/`provider_id`.

**File changes**
- `internal/xcl/hclwrite/ast_attribute.go`: add a public way to set an attribute's comments. `leadComments` and `lineComments` nodes already exist (`:13,16`) and `newComments` is unexported (`ast.go:61`), so add methods on `*Attribute`, e.g. `SetLineComment(Tokens)`, alongside a helper that builds comment tokens. MPL header plus a `UPSTREAM.md` entry, per the fork rules.
- `internal/xcl/gohcl/encode.go`: stop delegating a struct-valued attribute wholesale to gocty. When an attribute's resolved value is a struct carrying `xcl` tags, build its object from the fields the encoder itself walks, so the computed skip and the nil skip apply at every depth. Nested structs recurse the same way. Non-struct values keep going through gocty unchanged.
- `encode.go` (root): extend `trimBookkeeping` into the nested case, or rely on the encoder skipping `meta`/`depends_on`/`disabled` at depth. `depends_on` is now removed unconditionally, not only when empty.
- Computed marking: when `IncludeComputed` is set, each computed attribute gets a line comment. The encoder knows which fields are computed from `fieldTags.Computed`, so the comment is attached where the attribute is written.

**Complexity:** Medium. **Token estimate:** ~40k. **Agent strategy:** Single agent for the fork changes, then tests.

### Phase 2.1: Event data is off by default, with raw and processed levels

**Requirements covered:** User decision (event data opt-in, raw/processed). Enables "Example shows created resources". **Repo:** xclconfig.

**File changes**
- `events/events.go`:
  - Add `type DataLevel int` with `DataNone`, `DataRaw` and `DataProcessed`, and doc comments.
  - Update the `Data` field comment (:181-183) to: "serialized resource, set only when the Config's event data level asks for it; see DataLevel".
- `options.go` (:44-66 area): `func WithEventData(level EventDataLevel) ConfigOption` sets `c.eventData`. Root `events.go`: aliases `EventDataLevel = events.DataLevel`, plus consts `EventDataNone/Raw/Processed`.
- `config.go`: add the `eventData events.DataLevel` field to `Config` (near :87). Pass `EventData: c.eventData` in the three `parser.ParserOptions{...}` literals (:225, :256, :322).
- `internal/parser/parser.go:51-92`: `ParserOptions` gains `EventData events.DataLevel`.
- `internal/parser/events.go:30-51`:
  - `lifecycleEvent`/`emitLifecycle` stop taking raw `data`. They take the pre-call bytes and, for success, the resource.
  - Add `eventData(options, phase, pre []byte, r any) []byte`:
    - `DataNone` returns nil;
    - `DataRaw` returns pre;
    - `DataProcessed` with `PhaseSuccess` returns `json.Marshal(r)`, and otherwise pre.
  - Do not marshal at all under DataNone.
- `internal/parser/lifecycle.go`:
  - `callProvider` (:370-402) no longer emits success. It returns `(duration, error)`.
  - Callers emit success after the status assignment:
    - create: after `meta.Status = types.StatusCreated` (:152);
    - read: after :36 area;
    - changed: immediately, no status change;
    - update: after `meta.Status = types.StatusUpdated` (≈:258).
  - So processed data carries the final status, the same value `AppendResource` then records (`internal/parser/parser.go:512`).
  - `handledWithoutProvider` path (:91-94): pass `r` so registered and builtin types get data at raw (pre = `json.Marshal(r)`) and processed.
  - Destroy events (`internal/parser/callbacks.go:289-310`) carry the held resource at raw and processed, and nothing at none.
- Tests that assume Data is present opt in with `WithEventData(EventDataRaw)` or set `ParserOptions.EventData`:
  - `config_events_test.go:83-109` (and the registered-nil assertion at :98-109, which now moves to a level-none test);
  - `internal/parser/lifecycle_test.go:814`;
  - `example/*/main_test.go` Data assertions (grep `\.Data`).
- New tests in `config_event_data_test.go`:
  - `TestEventsCarryNoDataByDefault`
  - `TestRawEventDataIsPreCallResource`
  - `TestProcessedCreateSuccessCarriesProviderFilledValue` (TestPlugin network `provider_id = "id-<name>"`)
  - `TestProcessedCreateSuccessMatchesStateRecord` (compare with `json.Compact`/`JSONEq` against the state record, because state is MarshalIndent'd)
  - `TestProcessedCreateSuccessHasCreatedStatus`
  - `TestRegisteredTypeCarriesDataAtRaw`
  - `TestRegisteredTypeCarriesDataAtProcessed`
  - `TestProcessedDataConvertsLikeTheEntity` (EncodeSavedEntity(e.Data) == EncodeEntity(found))
- `internal/test_fixtures`/`internal/parser` golden or event-field tests (`config_event_fields_test.go`) are updated if they assert Data.

**Complexity:** Medium. **Token estimate:** ~45k. **Agent strategy:** Single agent for the parser changes, then a second agent in parallel for test migration across packages once the helper signature is fixed.

### Phase 3.1: The pretty receiver prints a created entity's configuration

**Requirements covered:** Example shows created resources (receiver). **Repo:** xclconfig.

**File changes**
- `example/prettylog/prettylog.go:39-49`:
  - `Handler(w io.Writer, level slog.Level, registry *registry.PluginRegistry) xcl.EventHandler` wraps the slog handler.
  - After delegating each event, when `e.Operation == events.OperationCreate && e.Phase == events.PhaseSuccess && len(e.Data) > 0 && registry != nil`, call `xcl.EncodeSavedEntity(registry, e.Data, xcl.IncludeComputed())`.
  - On success, write the text to `w`, indented two spaces per line, followed by a blank line.
  - On error, log one warn line through the same charm logger: "unable to show configuration" with resource and error attrs.
  - Only at or below info level (`level <= slog.LevelInfo`), so a warn-level run stays terse.
  - Update the doc comment example (:30-38).
- `example/prettylog/prettylog_test.go`: update existing calls for the new parameter (pass nil where irrelevant). Add:
  - `TestHandlerWritesConfigurationAfterCreateSuccess` (a registry with `registered.Database` or a local test type; Data built from a real apply at `EventDataProcessed`, or a `json.Marshal` of a typed value with meta set via `CreateResource`)
  - `TestHandlerWritesNothingExtraForOtherEvents`
  - `TestHandlerWritesNothingExtraWithoutData`
  - `TestHandlerReportsUnconvertibleData`

**Complexity:** Low. **Token estimate:** ~15k. **Agent strategy:** Single agent.

### Phase 3.2: Every example shows its created entities

**Requirements covered:** Example shows created resources; Every known resource type (external plugin); Success metrics round-trip, agreement and visibility. **Repo:** xclconfig.

**File changes**
- `example/plugin/main.go`:
  - In `main` (:41-66), create `r := registry.NewPluginRegistry()` and build `prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), r)` (:59).
  - Change `run(out, handler, r, dir, externalPlugin, statePath)`. Remove the registry creation at :75.
  - Add `xcl.WithEventData(xcl.EventDataProcessed)` to `NewConfig` (:102-106).
- `example/configonly/main.go` (:415-474) and `example/appconfig/main.go` (:234-278): the same restructuring.
- `static_examples_test.go` (:102-186): keep `TestExamplesMainSetsUpPrettyLogOnce` (still one call in main) and `TestExamplesCreatePluginRegistryWithoutArguments` (the call moves to main; confirm the AST walk covers `main`). Add a check that each example passes `xcl.WithEventData`.
- `example/plugin/main_test.go`: update the `run` calls (:140 `runRecordingEvents` etc.) to create and pass a registry. Add:
  - `TestPluginExampleShowsPostgresConfigurationAfterCreate` (buffer-backed `prettylog.Handler`; assert order: the create success line for `resource.postgres.main`, then `resource "postgres" "main" {`, `port = 5432`, `timeouts {` and `connection_string =`)
  - `TestPluginExampleShowsEveryCreatedEntity` (IDs from :57-70, excluding variables and outputs)
  - `TestPluginExampleEntitiesReadBack` (for each created resource entity from `Config.Entities()`, `EncodeEntity` default, write to a temp dir, then `Validate` with a registry that has the same plugins registered)
  - `TestPluginExampleEntityAndStateAgree` (read the state file at `statePath` and compare `EncodeSavedEntity` with `EncodeEntity` per ID)
  - `TestPluginExampleConvertsExternalPluginTypes` (app.web, ingress.web)
- `example/configonly/main_test.go` and `example/appconfig/main_test.go`: the same visibility, read-back and agreement tests. Update `run` call sites. `TestRunWithoutReceiverWritesNothingToStdoutOrStderr` stays green.
- Module entities (e.g. `module.analytics` postgres) have IDs with a module prefix. Read-back validates the text as a standalone root config, which is acceptable because values are resolved literals.

**Complexity:** Medium. **Token estimate:** ~45k. **Agent strategy:** 2–3 parallel agents, one per example, after one agent lands the shared `main`/`run` pattern on the plugin example first.

### Phase 4.1: Library documentation

**Requirements covered:** Documented (library). **Repo:** xclconfig.

**File changes**
- `README.md`:
  - Add a `### Converting to configuration text` section after "Events and logging" (before `## Struct Tags`, :515). It shows `xcl.EncodeEntity` on an entity from `xcl.Find`, and `xcl.EncodeSavedEntity(registry, data)` on a state record or processed event data. It covers `IncludeComputed()` and that text including computed values is for reading and does not validate. It covers the errors.
  - In "Events and logging" (:436-512), document `WithEventData` and the levels. Fix the Data row (:468).
  - In "Running them" (:203-218), mention that output now includes created configuration.
  - Replace the stale `## Serialization`/`## Deserialization` (:1419-1440) with a pointer to the new section.
- `docs/state.md` and `docs/plugins.md` (events section): a short note on the conversion functions and the data level. `docs/README.md` index, if a new doc is added (none planned).
- `CHANGELOG.md`: entries for `EncodeEntity`, `EncodeSavedEntity`, `IncludeComputed`, `WithEventData` and the new errors. **Breaking:** event Data is off by default.

**Complexity:** Low. **Token estimate:** ~15k. **Agent strategy:** Single agent.

### Phase 4.2: Documentation site guide

**Requirements covered:** Documented (site). **Repo:** xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).

**File changes**
- `xcl-website:src/pages/configuration-text.mdx` (new): frontmatter `layout: ../layouts/Shell.astro`, title and description, as in `events.mdx:1-5`. `Hero`, `Prose` and `CtaBanner` imports and structure (`events.mdx:6-16,222-247`). Sections:
  - converting an entity (Go block titled `main.go` plus the resulting `hcl` block titled `.xcl`);
  - converting saved data (from a state record, and from processed event data);
  - including computed values;
  - errors.

  Style: second person, no em-dashes, `hcl` fences for xcl.
- `xcl-website:src/components/Nav.astro:18-24`: add a Guides child, "Configuration text" at `/configuration-text/`.
- `xcl-website:README.md`: a Pages table row.
- `xcl-website:src/pages/events.mdx`: a section on event data levels (off by default, raw, processed) and why it is off by default.
- `xcl-website:src/pages/examples/{plugins,configuration-only,application-config}.mdx`: update quoted `main`/`run` code (registry in main, `WithEventData`, `prettylog.Handler` third argument) and show the sample output with configuration text.
- `xcl-website:src/pages/index.mdx:166-182`: optionally link the "State, apply and destroy" card to the new guide.
- A changelog record in the website repo following the pattern of `.spektacular/changelog/xclconfig/20260922061954-event-based-logging.md`. Write it through `spektacular changelog file write`, never with file tools.
- Verify with `npm run build` and `npx astro check` (`Makefile`: `make build`, `make check`).

**Complexity:** Low. **Token estimate:** ~20k. **Agent strategy:** Single agent.

## Testing Strategy

- **Phase 1.1:** encoder unit tests in the fork (package `gohcl`, testify, one behaviour each), covering remain, interface, nil, zero, computed off/on, repeated blocks, and errors instead of panics.
- **Phase 1.2:** `savedentity` decode tests from a real apply's state file, plus failure tests. The existing state tests are the regression proof.
- **Phase 1.3:** root `encode_test.go` carries every spec acceptance criterion except the postgres, external-plugin and example ones, using the registered Database, bare Cache and TestPlugin container/network fixture from a real apply.
- **Phase 2.1:** `config_event_data_test.go` tests one level per function, plus processed-equals-state (JSON-equal) and processed-converts-like-entity. Existing Data-asserting tests opt in to raw.
- **Phase 3.1:** prettylog tests for config text after create success, no extra output otherwise, and a failure line.
- **Phase 3.2:** per-example tests for visibility order, read-back (Validate), agreement (entity vs state record) and external plugin types. These carry the Round trip, Agreement and Visible success metrics as behavioural tests.
- **Phase 4.x:** a site build and `astro check`. Manual — captured in the implementation test plan: "Easy to use" (follow the docs alone) and a visual check of real example terminal output.

**Test kinds.**

- **Encoder unit tests (the HCL fork).** They cover each repaired behaviour on small fixture structs, in the fork's own test style:
  - remain bases are walked;
  - interface-held values are written;
  - nil is skipped;
  - computed is skipped by default and included on request;
  - unrepresentable values return errors instead of panicking.
- **Public API integration tests (root package).** These run a real `Apply` and convert the resulting entities and their state records.
- **Saved-decoder unit tests, plus a state regression.** The state tests prove the load behaviour and the aggregate unknown-types error are unchanged.
- **Event-level tests.** One for each of none, raw and processed, and for registered types.
- **Example tests.** They check that the output contains each created entity's text and that the text reads back.
- **Updated static example tests.**

Everything follows the project conventions: testify `require`, no table-driven tests, positive and negative cases in separate functions, and test state produced by a real apply.

**Where coverage concentrates.** The root API tests carry the acceptance criteria and get the most coverage. Encode fidelity is the feature, and it is what can drift silently. The load-bearing guarantees are these:

- **Header.** The output starts with the right header: `resource "postgres" "main" {`, or `<type> "<name>" {` for a bare type.
- **Names and blocks.** The output uses `xcl` names, not Go names. Nested blocks are blocks, and a repeated block appears once per entry.
- **Read-back.** Default output, written to a file, passes `Validate`. Parsing it gives the same type, name and configured values.
- **Agreement.** Converting the entity and converting its state record give byte-identical text.
- **Deterministic and formatted.** Converting twice gives identical bytes, and the formatter leaves the output unchanged.
- **Computed values.** Computed values are absent by default. With `IncludeComputed()`, the provider-filled `connection_string` appears.
- **Every kind.** In-process plugin, external plugin, keyword-registered and bare-registered types all convert.
- **Errors.** An unregistered type returns an error that names it and wraps `ErrUnregisteredType`, with nil text. Invalid data wraps `ErrInvalidSavedData`, with nil text. A builtin wraps `ErrNotEncodable`.
- **One block per call.** Three conversions give three texts, each with exactly one top-level block.
- **Event data.** At level none there is no `Data` on any event. At processed, a success event's `Data` equals the state record.

The postgres-specific acceptance criteria live in the plugin example's tests, since the exact type and its provider live in that example's internal package. The root tests use the in-repo fixtures.

**Deliberate gaps.** Builtins (`variable`, `output`, `module`) are only tested for refusal, since they are a non-goal. Output with `IncludeComputed()` is not tested for read-back, because by the user's decision it is for reading only. Docs are checked by building the site and reviewing them, not by automated content tests. There is one exception: a test asserts that the README has the conversion section and both examples.

**Success metrics.**

- **Round trip (100% of example resources read back):** Behavioural test. For every entity each example creates, the example tests convert it with default options, write the texts to a temporary configuration and `Validate` it without errors.
- **Agreement (0 mismatches between entity and saved data):** Behavioural test. For every entity each example creates, the example tests convert the entity found in `Config` and its state record, and assert the two texts are byte-identical.
- **Visible in the examples:** Behavioural test. Each example's tests run with a buffer-backed pretty receiver and processed event data. They assert that, for every created entity, its header line appears after its create-success line. A manual look at real terminal output is also captured in the implementation test plan ("Manual — captured in the implementation test plan").
- **Easy to use (a developer can convert following the docs alone):** Manual — captured in the implementation test plan.

## Project References

- Repos: `xclconfig`, root `/home/nicj/code/github.com/jumppad-labs/xcl` (library, examples, library docs); `xcl-website`, root `/home/nicj/code/github.com/jumppad-labs/xcl-website` (xcl.dev docs site).
- Knowledge: `conventions/shared-errors-package.md`, `conventions/never-modify-dependencies.md`, `conventions/test-state-from-real-apply.md`, `conventions/testing-and-mocking.md`, `decisions/own-hcl-fork-with-xcl-tags.md`, `gotchas/meta-type-holds-two-axes.md`, `glossary/entity.md`.
- Prior plan: `20260922061954-event-based-logging` (events, runner, prettylog).

**Requirement → repo and files**

| Requirement | Repo | Main files |
|---|---|---|
| Resource to configuration text | xclconfig | `encode.go` (new), `internal/xcl/gohcl/encode.go` |
| Saved data to configuration text | xclconfig | `encode.go`, `internal/savedentity/` (new), `state/file_state_store.go` |
| Written as a person would write it | xclconfig | `internal/xcl/gohcl/encode.go`, `encode.go` |
| Nothing left out (amended: computed opt-in) | xclconfig | `internal/xcl/gohcl/encode.go`, `encode.go` |
| Readable by xcl again | xclconfig | `encode.go`, `encode_test.go` |
| Printable or writable | xclconfig | `encode.go` |
| Every known resource type | xclconfig | `encode_test.go`, `example/plugin/main_test.go` |
| Clear failure | xclconfig | `errors/encode_errors.go` (new), `internal/savedentity/`, `config.go` |
| One call per resource | xclconfig | `encode.go` |
| Example shows created resources (+ event data levels) | xclconfig | `events/events.go`, `options.go`, `config.go`, `internal/parser/{events.go,lifecycle.go,callbacks.go}`, `example/prettylog/`, `example/*/main.go` |
| Documented | xclconfig, xcl-website | `README.md`, `docs/`, `CHANGELOG.md`; `xcl-website:src/pages/configuration-text.mdx`, `Nav.astro`, `events.mdx`, `examples/*.mdx` |

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Per-phase estimates: 1.1 ~40k, 1.2 ~25k, 1.3 ~60k, 2.1 ~45k, 3.1 ~15k, 3.2 ~45k, 4.1 ~15k, 4.2 ~20k.

## Migration Notes

- **Breaking:** lifecycle events no longer carry `Data` by default. Applications reading `Event.Data` add `xcl.WithEventData(xcl.EventDataRaw)` to keep the pre-call resource, or `EventDataProcessed` for the post-call resource. This goes in the CHANGELOG.
- `prettylog.Handler` gains a registry parameter. It is an example package, so only the bundled examples change.
- The state file format is unchanged, and no state migration is needed.

## Performance Considerations

- At the default `EventDataNone`, no resource is marshalled for events, which is less work than today.
- `EventDataProcessed` adds one `json.Marshal` per success event.
- Encoding is reflection over one entity per call. It is not on any hot path, and the examples call it once per created entity.

