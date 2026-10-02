---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-23"
---

# Research: 20260922132517-hcl-encoding-helpers

## Alternatives considered and rejected

- **JSON → cty → hclwrite (encode the saved JSON directly).** Rejected. The output would use `json` tag names, which can differ from the `xcl` names, and nested blocks would come out as object attributes. Only the typed Go struct carries the `xcl` tags the parser reads. See `internal/xcl/gohcl/schema.go:129-179` (getFieldTags reads the `xcl` key, `internal/xcl/tags/tags.go:18`).
- **Make hclwrite/gohcl public and let callers encode.** Rejected by a spec constraint (the internal HCL-writing machinery stays internal) and by the user during the spec interview.
- **Use `gohcl.EncodeIntoBody` / `EncodeAsBlock` unchanged.** Rejected. Probes (scratchpad `go test -overlay`, then deleted) showed:
  - remain-embedded bases (`types.ResourceBase \`xcl:",remain"\``, which every resource has; `ContainerBase`) are dropped entirely (`encode.go:89-195` handles only Attributes and Blocks);
  - `any`/interface fields are silently skipped (`encode.go:121`, `exprType.AssignableTo(fieldTy)` is true for every interface). Schema-built plugin types turn named scalars (`type Mode string`, `time.Duration`, `int8/int16/uint8`) into `interface{}` (`internal/schema/deserialize.go:301-316`), so those plugin fields vanish;
  - computed fields are emitted (`tags.Field.Computed`, `tags.go:73`, is ignored);
  - nil slices and maps are written as `= null`, and zero optionals as `= 0` / `= ""`;
  - struct-valued attributes that embed ResourceBase dump `depends_on`, `disabled` and a full `meta = {...}`;
  - it panics on `map[string]any`, `time.Time`, unknown cty values, capsules, and non-struct values (`encode.go:47,137,144`; `hclwrite/generate.go:303`).
- **Emit computed values and relax the validator so it accepts them.** Rejected by the user: "We should not change the validation to allow the import of computed as they would be overwritten anyway." Validation rejects configured computed fields at `internal/parser/validate.go:219` (`configuredComputedFields`, `computedFieldProblem`).
- **Always put Data on events (make success carry the post-call resource unconditionally).** Rejected by the user for security reasons. Config and state should not travel in events unless asked for, so Data becomes opt-in, off by default, with raw and processed levels.
- **Examples render from `Config` after Apply instead of from events.** Rejected. The acceptance criterion wants the text alongside the create-success line.
- **A `Config` method for saved data (use the Config's private registry).** Weighed in architecture. `Config.pluginRegistry` is unexported (`config.go:87`) and the example handler is built before Config exists, so a registry-taking entry point is needed either way.

## Chosen approach — evidence

- A tag-driven encoder is the right base: gohcl encode shares `getFieldTags` with the decoder (`internal/xcl/gohcl/schema.go:129`). Lists of blocks, pointer blocks and key fields already encode correctly (probe). It compiles and passes against the fork (`go test ./internal/xcl/gohcl ./internal/xcl/hclwrite`). It needs fork-local changes (MPL: changes stay in `internal/xcl`, recorded in `UPSTREAM.md`):
  - recurse into remain-embedded anonymous structs, skipping `meta`;
  - encode interface-held values by their dynamic type;
  - skip computed fields unless asked;
  - skip nil values;
  - return errors instead of panicking.
- Block header from Meta: `Subtype != ""` gives `resource "<Subtype>" "<Name>"`, otherwise `<Type> "<Name>"` (`plugins/registry/plugin_registry.go:214-221`, `types/resource.go:69-75` AddressType). Build the block by hand with `hclwrite.NewBlock`, then encode the body. Resources carry no `label` fields.
- `hclwrite.File.Bytes()` already runs the formatter (`internal/xcl/hclwrite/ast.go:39-52`). `hclwrite.Format` is at `public.go:41` and is "xcl's formatter" for the idempotence criterion.
- Saved data → typed resource: `FileStateStore.Load` has the per-record logic (`state/file_state_store.go:67-138`):
  1. unmarshal into a map;
  2. read `meta.type`, and `meta.subtype`, which overrides the type when non-empty;
  3. read `meta.name`;
  4. `registry.CreateResource(key, name)` (`plugin_registry.go:195`);
  5. re-marshal and unmarshal into the typed pointer.

  Extract it into a single-record function shared by state and the new API.
- Plugin types are only resolvable after `registry.Load(emit)` (`plugin_registry.go:442`), which runs once and caches its result. Config calls it in `withPlugins` (`config.go:427-437`). The saved-data entry point must call `Load(nil)` itself so it works on a fresh registry.
- Schema-built plugin types keep the plugin's full struct tags (`internal/schema/serialize.go:53`, `deserialize.go:144-207`). So `xcl` names, blocks and computed flags survive. Only the named-scalar → `interface{}` loss needs the dynamic-type encode fix. This resolves the spec's "known risk".
- Event Data today:
  - Create success carries pre-call JSON (`internal/parser/lifecycle.go:132-140`, success at `:400`; read/changed/update at `:195,:224,:244`).
  - Registered and builtin types' create success has nil Data (`lifecycle.go:90-94`, `handledWithoutProvider` `:433-444`; asserted in `config_events_test.go:98-109`).
  - Destroy events carry the old resource JSON (`internal/parser/callbacks.go:289-310`).
  - `Event.Data` is documented at `events/events.go:181-183`.
- Error convention: sentinel plus pointer detail with `Unwrap` in `errors/` (`errors/query_errors.go:23-159`; `plugin_load_error.go` for a detail that also wraps a cause), re-exported from root (`config.go:33-80`). The registry's unknown-type miss is a plain `fmt.Errorf` (`plugin_registry.go:318`) with no sentinel. `state.UnknownTypesError` (`state/errors.go:39-56`) is a value type without Unwrap.
- Example wiring: `prettylog.Handler(w, level)` is built in each `main` (`example/plugin/main.go:59`, `example/configonly/main.go:428`, `example/appconfig/main.go:247`), but the registry is created later in `run` (plugin `:75`, configonly `:443`, appconfig `:261`). Delivery runs after `registry.Load` (`config.go:367-438`), so a handler can call `CreateResource` safely.
- Acceptance-test model: `state/file_state_store_apply_test.go` (`testRegistry` :42, `testApplyToStateFile` :64, `testLoadSavedState` :87, `entityByID` :104) runs a real apply and reads the state file, as the test-state-from-real-apply convention requires.

## Files examined

- `internal/xcl/gohcl/encode.go:40-195` — EncodeIntoBody/EncodeAsBlock/populateBody; the gaps listed above; used nowhere outside the package; only test is `ExampleEncodeIntoBody` (`encode_test.go:14-68`)
- `internal/xcl/gohcl/schema.go:129-179` — getFieldTags reads the `xcl` key; kinds attr/block/label/remain/body/optional; computed/key parsed; an unknown option panics
- `internal/xcl/gohcl/decode.go:85-100` — the decoder recurses into the remain field
- `internal/xcl/gohcl/check_test.go` — fork-authored test style: package gohcl, testify require, one behaviour per test
- `internal/xcl/tags/tags.go:18,56-87` — tag key `xcl`, Parse, Field.Computed
- `internal/xcl/hclwrite/{ast.go:20-52, ast_block.go:30, ast_body.go:148-236, public.go:41, generate.go:27,186-306}` — file/block/body API; Bytes formats; TokensForValue panics on unknown and capsule values
- `internal/xcl/UPSTREAM.md` — MPL rules; modifications must be recorded here
- `internal/cty/json.go:16` — only cty.Type has MarshalJSON, so cty.Value is lost in JSON (builtins out of scope anyway)
- `types/resource.go:5-87` — Meta json/xcl tags; ResourceBase depends_on/disabled/meta
- `types/register.go:8-76` — TypeInfo{Bare, Builtin, Prototype}; ErrTypeNotRegistered unused by the registry
- `plugins/registry/plugin_registry.go:26,80,115,122,195-226,278-318,357,374,442` — registry API, CreateResource, plugin-schema types, Load
- `internal/schema/serialize.go:53`, `deserialize.go:66-123,144-207,301-349` — tags copied verbatim; named scalars become interface{}; nested structs become anonymous
- `state/file_state_store.go:34-187` — Load per-record decode and Save format (JSON array)
- `state/errors.go:39-56` — UnknownTypesError
- `internal/parser/lifecycle.go:90-153,370-402,433-449` — event emission points and Data contents
- `internal/parser/events.go:30-51` — lifecycleEvent/emitLifecycle
- `internal/parser/callbacks.go:289-310` — destroy event Data
- `internal/parser/validate.go:190-275` — computed-field rejection
- `internal/parser/parser.go:521-815` — keyword vs bare block parsing
- `events/events.go:66-193`, `events/slog.go:19,40-114` — Event struct; SlogHandler never writes Data
- `config.go:87,134-136,367-438`, `options.go:13-66` — options, private registry, runner
- `errors/query_errors.go`, `errors/plugin_load_error.go` — sentinel conventions
- `internal/test_fixtures/registered/types.go:21-62` — Database (timeouts, computed connection_string, never set); Cache (bare)
- `internal/test_fixtures/plugin/structs/{container.go,template.go}` — remain bases, block lists, computed/key fields
- `internal/parser/test_plugin.go:44-` — in-process TestPlugin types
- `config_events_test.go:17-109` — eventRecorder; Data assertions that will change
- `example/plugin/{main.go,resources/resources.go,internal/plugin.go,external/main.go,config/main.xcl,main_test.go}` — PostgreSQL with timeouts and computed connection_string filled by the provider
- `example/configonly/main.go`, `example/appconfig/main.go`, and their main_test.go files — registered-type examples
- `example/prettylog/prettylog.go:27-80`, `prettylog_test.go` — handler, LevelFromEnv, test helpers
- `static_examples_test.go:17,102,118,129,144,186` — static constraints on example main/run shape
- `README.md:203-218,436-512,1419-1440` — Running the examples; Events (Data row 468); stale Serialization/Deserialization sections
- `xcl-website:src/pages/{events.mdx,plugin-logging.mdx,index.mdx,examples/*.mdx}`, `src/components/Nav.astro:8-25`, `README.md` Pages table, `ec.config.mjs`, `Makefile` — docs structure and conventions

## External references

- None required. The HCL native syntax is already vendored in `internal/xcl` (`spec.md`).

## Prior plans / specs consulted

- `20260922061954-event-based-logging` (plan): defined `events.Event`, `Data` as the serialized resource, the prettylog example over slog/charm, and the operation runner that loads the registry before delivery. Explains why `WithEventHandler` exists and why the library never prints.
- `20260922132517-hcl-encoding-helpers` (spec): the target.
- Knowledge: `decisions/own-hcl-fork-with-xcl-tags.md` (the fork is XCL-owned; `computed` lives in the xcl tag), `gotchas/meta-type-holds-two-axes.md` (Type/Subtype axes; UnmarshalUntyped does not fail on mismatched structs), `architecture/config-is-the-public-query-surface.md`, and the conventions `shared-errors-package`, `test-state-from-real-apply` and `never-modify-dependencies` (the HCL fork is ours to edit, with an MPL header and a UPSTREAM.md record).

## Open assumptions

- Changing a field's JSON-derived value into the Go struct and back through the fixed encoder reproduces configured values closely enough that re-parsing gives equal configured values for every example resource. Durations or named scalars held as `interface{}` in schema-built types may encode as a different literal (e.g. a number). If a round-trip mismatch appears, STOP and ask.
- `registry.Load(nil)` is safe to call from the saved-data entry point both before and after Config has loaded, since it is once-only and cached.
- Values in the typed resource are resolved values. References in the original config (e.g. `resource.postgres.main.port`) come out as literals. This is assumed acceptable ("configured values" equal, not expressions).
- Turning event Data off by default is acceptable as a behaviour change. Existing tests and docs asserting Data on plugin events will be updated.

## Drafting assumptions

### Event Data levels interpretation (discovery)
- **Decision**: The user asked for "two levels. One, the raw resource, two the processed resource ... off by default". Interpreted as a Config option with three settings: off (default, no Data on any event), raw (resource as decoded from config, before the provider call, which is today's content), processed (resource after the provider call, the same form state saves). Raw and processed apply to every resource type, including registered config-only types. Examples opt in to processed.
- **Rationale**: This matches the user's words and security motivation, and lets all three examples show created config.
- **Rejected**: Always-on post-call Data (the user raised security); providers-only Data (configonly/appconfig examples would show nothing).

### Nil values omitted, zero values kept (discovery)
- **Decision**: The encoder skips nil pointers, nil slices and nil maps (they hold nothing) but writes zero scalars that are present (`port = 0`).
- **Rationale**: "Nothing left out" covers every value held. A nil holds no value, and writing `= null` is noise.
- **Rejected**: Writing `null` (noisy); dropping zero values (would hide a real configured 0/false/"").

### Chosen direction: root-package entity functions over the repaired internal encoder (architecture)
- **Decision**: Option A. `xcl.EncodeEntity`, `xcl.EncodeSavedEntity(registry, data)` and `xcl.IncludeComputed()` in the root package. The vendored `gohcl` encoder is fixed in place (remain recursion, dynamic interface values, skip nil, skip computed unless asked, errors not panics). The per-record saved-data decode is extracted from `FileStateStore.Load` into a shared internal function. `WithEventData(EventDataNone|Raw|Processed)` has none as the default. prettylog takes the registry.
- **Key trade-offs**: "entity" naming per the glossary (bare types are not `resource` stanzas). The registry is passed explicitly, so it works before or without a Config. ResourceBase bookkeeping is trimmed in the wrapper: `meta` is removed, `disabled` only when true, `depends_on` only when non-empty.
- **Rejected**: B, Config methods (the handler is built before Config; state tooling needs no Config), low-medium effort. C, a new reflection walker in root (duplicates the decoder's tag rules; MPL-adapted code must live in internal/xcl anyway), medium-high effort.

### Examples include computed values (architecture)
- **Decision**: prettylog renders with `IncludeComputed()` so people see provider-filled values like connection_string.
- **Rationale**: The example output is for reading. The spec's example requirement says "full configuration".
- **Rejected**: Default output in the examples (hides the provider values the spec's motivation was about).

### Builtins refused with ErrNotEncodable (architecture)
- **Decision**: `variable`/`output`/`module`/`root` entities return `ErrNotEncodable` rather than best-effort text.
- **Rationale**: This is a spec non-goal, and cty.Value fields don't survive JSON (`internal/cty/json.go:16`), so saved builtins can't be rebuilt faithfully.
- **Rejected**: Best-effort encoding (silently wrong defaults and values).

### Processed data marshalled where state records it (architecture)
- **Decision**: With `EventDataProcessed`, success events carry the resource marshalled after the provider result and status are applied, and start/error events carry pre-call data. Destroy events keep the held resource.
- **Rationale**: The spec's agreement metric needs event data identical to the state entry.
- **Rejected**: Processed data on every phase (no post-call value exists at start).

### Conventions selection (architecture)
- **Decision**: Kept shared-errors, never-modify-dependencies/MPL fork rules, test-state-from-real-apply, testing-and-mocking, code-style, dependencies and DI. Dropped database, context threading, graceful shutdown, structured logging and project layout, as not applicable.
- **Rationale**: Only these bear on a synchronous in-process encoder plus an event option.
- **Rejected**: n/a

### New gohcl EncodeBody with options, keep old entry points (data_structures)
- **Decision**: Add `gohcl.EncodeBody(val, dst, EncodeOptions) error` alongside the existing `EncodeIntoBody`/`EncodeAsBlock`. The old functions become thin wrappers that keep their panic behaviour for the upstream example.
- **Rationale**: This keeps the upstream API shape recognisable and gives the new code error returns and options.
- **Rejected**: Changing the existing signatures (churns the upstream example for no gain).

### Internal package name `savedentity` (data_structures)
- **Decision**: The per-record decoder lives in a new internal package `internal/savedentity`.
- **Rationale**: Both `state` and root import it, and it needs registry plus errors only, so there is no cycle. Putting it on the public registry would add public API the spec didn't ask for.
- **Rejected**: A `registry.DecodeResource` method (widens the public surface); leaving it in `state` (root would need a new public state function).

### Processed data compared to state as JSON, not bytes (phases)
- **Decision**: "Processed success Data equals the state record" is asserted as JSON-equal. The state file is written with MarshalIndent (`state/file_state_store.go:162-187`), so its bytes differ in whitespace. Text from EncodeSavedEntity is still byte-identical.
- **Rationale**: Changing the state format is forbidden by a spec constraint.
- **Rejected**: Byte equality (would require changing the state writer).

### Success emitted after status is set (phases)
- **Decision**: `callProvider` stops emitting success. Callers emit it after assigning the status, so processed data carries `created`/`updated`.
- **Rationale**: This matches the state record.
- **Rejected**: Marshalling inside callProvider (status not yet set).

### InvalidSavedDataError carries the record ID (phases)
- **Decision**: Add `ID string` to InvalidSavedDataError.
- **Rationale**: The state aggregate error must keep reporting a record's ID or `entry N` exactly as before.
- **Rejected**: Having state peek at the record separately (duplicates the reader).

### prettylog prints config only at info or lower (phases)
- **Decision**: Configuration text is written when the handler level is info or more verbose (the default level).
- **Rationale**: The spec says "at its default output level". Warn/error runs stay terse.
- **Rejected**: Always printing it.

### Postgres acceptance criteria verified in the plugin example (phases)
- **Decision**: The spec's postgres/connection_string/external-plugin criteria are tested in `example/plugin` tests. The root tests use the registered Database, bare Cache and TestPlugin container fixtures.
- **Rationale**: The PostgreSQL type and its provider live in an internal package that root tests cannot import.
- **Rejected**: Duplicating a postgres plugin into test_fixtures (redundant).

### Stray configonly binary left out (out_of_scope)
- **Decision**: The 24MB `configonly` ELF committed at the repo root is not removed by this plan. It is listed in Out of Scope.
- **Rationale**: It is unrelated to this feature, and removing it is a separate cleanup the user should choose.
- **Rejected**: Deleting it in Phase 3.2 (scope creep).

## Rehydration cues

- `spektacular spec file read 20260922132517-hcl-encoding-helpers.md`
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`
- Re-read `internal/xcl/gohcl/encode.go`, `state/file_state_store.go:34-146`, `internal/parser/lifecycle.go:80-160,370-402`, `plugins/registry/plugin_registry.go:195-320`, `example/prettylog/prettylog.go`, `static_examples_test.go`
- `.spektacular/work/20260922132517-hcl-encoding-helpers/assumptions.md` and `.spektacular/working-context.md`
