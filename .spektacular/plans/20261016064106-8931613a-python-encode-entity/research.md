---
created_date: "2026-10-16"
document_status: final
closed_date: "2026-10-16"
---

# Research: 20261016064106-8931613a-python-encode-entity

## Alternatives considered and rejected

- **Encode the lookup JSON as it stands, with no fill from the engine's copy.** The unmerged branch
  `origin/spek/20261015072147-b41720ce-python-encoder` (xcl `9c5dc1f`, "Render entity records with Go's
  encode choices") did this: `encodeHeld` encoded the lookup-JSON copy, so reveal on a looked-up
  entity kept the marker. Rejected because the spec requires the real value for a looked-up entity
  with reveal ("as Go shows it"). Go shows it because Go encodes the engine's own stored pointer
  (`config.go:194`, `Entities()` :300, `FindResource` :305-317). Lookup JSON carries the marker,
  because `coreservice/lookups.go:24,50,72,93` use `json.Marshal` and `Sensitive.MarshalJSON`
  writes the marker (`types/sensitive.go:90`). The branch is also based on the old `efdb6a9`, which
  predates the singleton, plain-type and nullable work. Its proto shape (`EncodeOptions`, fields 1-4)
  and its 16-combination parity naming are reusable; its reveal semantics are not.
- **Python builds the text itself, or re-implements the options.** Rejected by the spec constraint
  that the shared engine does the rendering. `encode.go:181-249` (`encodeEntity`) is the one path.
- **Python sends field values plus a separate meta message, rather than one JSON record.** Rejected.
  The design says entity data crosses as JSON bytes (libxcl.md "The CoreService"), and
  `savedentity.Decode` (`internal/savedentity/savedentity.go:55-111`) already reads exactly the
  record shape hydration reads (`xcl-python:xcl/_entity.py:40-55`, `_schema.py:466-531`).
- **Fill redacted values at the JSON level** (`wire.Marshal` of the stored entity, then replace each
  `"(sensitive)"` string at the same JSON path in the host record). Rejected. `wire` is
  internal and allow-listed (`internal/wire/wire.go:1-31`), and a string match cannot tell a
  sensitive field's marker from an ordinary string `"(sensitive)"`. A typed walk that touches only
  `types.SensitiveValue` fields where `types.IsRedacted` is true (`types/sensitive.go:142-160`) is
  exact.
- **A new `EncodeOption` (e.g. `FillFrom(entity)`) on Go's public encoder.** Rejected. It would add a
  choice to Go's encoder, and changing Go's encoder choices is a spec non-goal.
- **Fill by address inside `coreservice`.** Rejected. `coreservice` reaches the engine only through
  public packages (`coreservice/service.go:8-13`; design "Layers"), and record decoding
  (`internal/savedentity`) is internal. The design's rule is that what `coreservice` needs is made
  public in the root module first.
- **Python raises `NotRegisteredError` for an object that isn't an entity** (what
  `ClassMap.names_of` raises today, `xcl-python:xcl/_classes.py:51-61`). Rejected for non-entities
  such as `None`, a dict or an unregistered non-dataclass. Go reports a non-entity as `ErrNotEncodable`
  "it is not an entity" (`encode.go:192-198`). `NotRegisteredError` is kept for an instance of a
  dataclass the configuration did not register, which the spec lists as its own case.

## Chosen approach — evidence

- `xcl:encode.go:31-36,41,49,67,79,99`: `encodeOptions` has four bools, set by `IncludeComputed`,
  `IncludeEmpty`, `RevealSensitive` and `ShowReferences`. All default to false.
- `xcl:encode.go:154-177`: `Config.EncodeSavedEntity` runs `catalog.Use(nil)`, then
  `savedentity.Decode(..., ReadOptions{ForDisplay: true})`, then `encodeEntity`. The new entry point
  follows the same three steps, with one extra step between decode and encode: fill redacted values
  from the stored entity.
- `xcl:encode.go:254-266`: `sensitiveReplacement`. A redacted value (`RedactedMark`) stays the
  marker even with reveal. A hand-built record carrying a real value therefore shows that value, and
  one carrying the marker shows the marker unless it is filled first.
- `xcl:types/sensitive.go:96-114`: `UnmarshalJSON` reads a real JSON value as real and the bare
  marker as redacted. ForDisplay turns mask envelopes into the marker
  (`savedentity.go:244-247`). The edited-sensitive rule therefore falls out: an edited field
  arrives real and is not filled, while an unedited one arrives as the marker and is filled.
- `xcl:types/sensitive.go:142-160`: `SensitiveValue` and `types.IsRedacted` are public, so the root
  package can find redacted fields by reflection and copy the stored `Sensitive[T]` value with
  `reflect.Value.Set`. Both values have the same Go type: both come from catalog prototypes
  (`internal/catalog/catalog.go:450`, `CreateEntity`).
- `xcl:config.go:305-317`: `FindResource(id)` returns the stored pointer, so finding the engine's
  copy by `meta.id` is a public, cheap call.
- `xcl:coreservice/lookups.go:120-141`: the `encodeSavedEntity` handler (`lookupAllowed`, empty
  check, highlight via `highlight.ANSI.Render(highlight.Pieces(text))`) is the template for the new
  handler. The options are added here.
- `xcl:coreservice/operations.go:82-90`: `lookupAllowed` gives the operation-in-progress refusal
  both calls need. `coreservice/errors.go:274-277` maps `not_encodable` (details `what`,
  `reason`).
- `xcl:coreservice/service.go:205-226`: the handler table; `service_test.go:299,308` hold it to
  the descriptor. A new `EncodeEntity` RPC must be added to both.
- `xcl:proto/xcl/core/v1/core.proto:46,292-299`: the existing messages. Fields are append-only
  (:7-10) and `buf-breaking` runs against main.
- `xcl-python:xcl/config.py:213-216`, `aio.py:257-260`: `encode_saved_entity` builds the request and
  calls the client. The new `encode_entity` sits beside it and the aio one goes through
  `self._lookup`.
- `xcl-python:xcl/_schema.py:466-578`: `hydrate`, `_hydrate_into`, `_labels_of`, `_hydrate_value`
  and `_sensitive_of` are the code the new record builder mirrors, field for field: key
  `options.name or f.name`, top-level `meta` from `Meta`, labels into meta, nested labels as keys,
  Sensitive real or marker, and lists and dicts recursed.
- `xcl-python:xcl/_entity.py:40-55`: `Meta.from_entity`. Its mirror writes the known keys plus
  `extra` into `meta`, and `disabled` and `depends_on` beside it.
- `xcl-python:xcl/_entity.py:97-99`: `Entity.to_dict()` is already the record for an untyped
  entity.
- `xcl-python:xcl/_classes.py:47-61`: `ClassMap.names_of(cls)` gives type and subtype from the
  configuration's own registrations, as the spec asks.
- `xcl:bindings/libxcl/cmd/parity/main.go:190-245` and `xcl-python:tests/parity/run_python.py:125-168`:
  the two record writers, compared by `tests/parity/test_parity.py`. An `encode` section is added
  to both.

## Files examined

- `xcl:encode.go:31-334` — options, `EncodeEntity` :126, `EncodeSavedEntity` :154, `encodeEntity` :181, `trimBookkeeping` :277, `showReferences` :302, sensitive rule :319, `holdsSensitiveMarker` :436, `isBareReference` :444
- `xcl:internal/savedentity/savedentity.go:55-111,244-247` — the record reader; needs meta, meta.type, and meta.name unless singleton; ForDisplay turns envelopes into the marker
- `xcl:internal/catalog/catalog.go:450,610,720-745` — `CreateEntity`, plugin fallback, `Use`
- `xcl:types/sensitive.go:31-160` — Sensitive marshal/unmarshal, `SensitiveValue`, `IsRedacted`
- `xcl:types/resource.go:20-73` — Meta JSON tags; Disabled/DependsOn `json:"-"`, saved beside meta
- `xcl:config.go:98-118,181-184,194,300-317` — error re-exports, stored entities, `FindResource`
- `xcl:query.go:39-151` — `Find[T]` copies; a plain copy is refused by the encoder (`plain_types_test.go:117`)
- `xcl:internal/xcl/gohcl/encode.go:36-41,712-754` — IncludeEmpty, `emptyValue`, nullable pointers
- `xcl:internal/schema/v1.go:69-73,280-289,301,342,360-392,650-656` — nullable pointer fields, JSON tags, block shapes
- `xcl:errors/encode_errors.go:23-81` — ErrUnregisteredType, ErrInvalidSavedData, ErrNotEncodable and detail types
- `xcl:coreservice/lookups.go:10-141` — lookup marshalling and the EncodeSavedEntity handler
- `xcl:coreservice/service.go:8-13,72-107,180-226` — public-only rule, dispatch, `unary`, handlers
- `xcl:coreservice/operations.go:32-35,77-90` — operation guard and lookup refusal
- `xcl:coreservice/errors.go:16-44,84-331` — code constants and mapping
- `xcl:coreservice/lookups_test.go:24,121,154,165,175` — `appRecord`, lookup and encode tests to model on
- `xcl:coreservice/service_test.go:19-28,256-308` — `appSchema` (sensitive password), helpers, descriptor tests
- `xcl:coreservice/operations_test.go:728-750` — `TestLookupDuringOperationRefused`, `holdCreate`
- `xcl:encode_test.go:36-506`, `encode_sensitive_test.go:17-172`, `encode_references_test.go:66-331`, `encode_highlight_test.go:44-146`, `encode_errors_test.go:18-81` — Go encode tests and fixtures
- `xcl:internal/test_fixtures/registered/*.go:89-110` — `Secret`, `SecretConsumer`, `SecretShape`
- `xcl:proto/xcl/core/v1/core.proto:7-10,17-47,278,292-299` — service and messages
- `xcl:Makefile` (core-protos, buf-lint, buf-breaking), `buf.gen.yaml` — generation into `coreservice/proto/core.pb.go`
- `xcl:bindings/libxcl/cmd/parity/main.go:190-245`, `bindings/libxcl/testdata/parity/README.md` and fixtures — parity program and fixture layout
- `xcl:docs/state.md:125-161`, `README.md:220`, `CHANGELOG.md:1-10,189-236` — library docs and changelog format
- `xcl-python:xcl/config.py:64,138-141,175-235` — lookups, `_hydrate`, `encode_saved_entity`, local errors
- `xcl-python:xcl/aio.py:75,125,215-260,298-306` — `_lookup`, reuse of sync helpers
- `xcl-python:xcl/_schema.py:58-219,466-578` — schema derivation (nullable :209-212) and hydration
- `xcl-python:xcl/_entity.py:15-99` — Meta and Entity
- `xcl-python:xcl/_types.py:10-137` — Sensitive (`redacted`, `reveal`, `is_redacted`), Computed, FieldOptions
- `xcl-python:xcl/_classes.py:16-61` — per-config ClassMap
- `xcl-python:xcl/_errors.py:78-165,253-262,419-464` — exception classes and code map
- `xcl-python:xcl/_client.py:26,57-64` — any descriptor method is callable; status 1 becomes an exception
- `xcl-python:Makefile` (protos, engine, plugins), `xcl-ref.txt` (v0.10.0), `.github/workflows/test.yml:68-101`
- `xcl-python:tests/fakes.py`, `tests/unit/test_hydrate.py:30-41,79-86,267`, `tests/unit/test_config.py:238`, `tests/unit/test_docstrings.py`
- `xcl-python:tests/integration/test_sync.py:775-807`, `sync_entities.py`, `test_aio.py:63-78,483-525,586-593`, `test_singleton.py:115-131`
- `xcl-python:tests/parity/{conftest.py,run_python.py,test_parity.py,types.py,test_types.py}` — parity harness
- `xcl-python:PARITY.md:15,34,36-41,74,80`, `README.md:108-192,239-245,335-353,405-425`
- `xcl-website:src/pages/configuration-text.mdx:20-427` — the only page on encoding; Python tabs at 52, 90-94, 125-131, 152, 231, 298-302, 326-330, 363, 410
- `xcl-website:src/pages/events.mdx:201-213,401` — encode mentions
- `xcl-website:src/components/CodeTabs.astro`, `src/config/site.ts` — two slots (go, python); `pythonParityURL`
- `xcl-website:src/pages/index.mdx:22` — version badge

## External references

- None needed. All behaviour is defined by the xcl engine and the existing SDK.

## Prior plans / specs consulted

- `20260922132517-hcl-encoding-helpers`: origin of `EncodeEntity`/`EncodeSavedEntity` and the one
  shared `encodeEntity`; builtins refused; `savedentity.Decode` is the single reader.
- `20261003153421-bf87d907-references-as-written`: `Meta.References` and the sensitive bare-reference
  rule; live and saved text are byte-identical.
- `20261003134528-327e0657-references-and-secrets`: `Sensitive`, the marker, `RevealSensitive`; text
  from event data always shows the marker.
- `20261003153421-9fa72edd-masking`: display reads turn envelopes into the marker; opening masked
  data inside `EncodeSavedEntity` is out of scope.
- `20261006112108-17623cda-encoder-syntax-highlighting`: `highlight` package; the `Highlight` encode
  option was later removed; the core service colours after encoding.
- `20261011130709-aa3eb933-python-support`: core service, `EncodeSavedEntity` RPC, keyword-only
  `highlight`, the parity harness compares decoded JSON records.
- `20261011211634-a8895873-python-out-of-process-engine`: `xcl-core` over stdio; xcl tasks first, then
  xcl-python built against an xcl ref.
- `20261014043756-e853a081-python-plain-dataclasses` (draft, empty template). The plain-dataclass work
  shipped under another id (xcl-python #13, #14, #16, #17): `register_type(cls, type, subtype)`, an
  optional `meta: xcl.Meta | None` field, label fields, singletons inferred, and `T | None` nullable.

## Open assumptions

- The decoded record and the stored entity have the same Go type for every declared and plugin
  type, so a field-by-field reflective walk lines them up. Plugin types are rebuilt by
  `reflect.StructOf`, which Go caches for identical field lists. If they ever differ, the fill must
  skip that entity rather than panic.
- Lists of blocks are matched by index and maps by key. A host that reorders blocks on a looked-up
  instance before encoding with reveal could see a stored secret written at the reordered position.
  That position is the same field of the same entity, and the user asked for reveal, so it is
  accepted.
- A hand-built instance's `meta.id`, when the user copied a meta in, is trusted to find the engine's
  copy. An id that matches nothing means no fill (the marker stays) and no error.
- Python's record builder is the only new Python logic. No Python provider serialiser exists
  (xcl-python has no providers), so it is written new as the mirror of hydration.
- xcl-website has no separate Python guide or API reference page. The Python docs live in the Python
  tabs of `configuration-text.mdx` (and `events.mdx`). See the open question raised with the user.

## Drafting assumptions

### Chosen direction (architecture)
- **Decision**: New `EncodeEntity` RPC plus an `EncodeOptions` message shared with `EncodeSavedEntity`. A new public `(*Config).EncodeEntityRecord(data, options...)` in the xcl root package decodes the record, fills redacted sensitive values from the stored entity (found by `meta.id`) when `RevealSensitive` is given, and calls the shared `encodeEntity`. Python adds `encode_entity` on both Config classes, plus a `record_of` mirror of hydration in `_schema.py`.
- **Key design decisions**: the fill is typed (reflection over `types.SensitiveValue`), not a JSON string match; an id that matches nothing means no fill and no error; slices are matched by index and maps by key; the keyword names are the snake_case of Go's option names; highlight is applied in coreservice for both calls; parity gains an `encode` section with 16 combinations for looked-up and saved records, and hand-built entities listed per fixture.
- **Rejected**: an address-only request (loses edits); the fill in coreservice (no internal access); a new Go `EncodeOption` (changes Go's encoder choices); encoding the lookup copy as is (loses reveal).

### Old unmerged branch is reference only (discovery)
- **Decision**: Do not build on `origin/spek/20261015072147-b41720ce-python-encoder`. Reuse its proto shape (`EncodeOptions` fields 1-4) and its 16-combination parity naming as a model only.
- **Rationale**: It predates the singleton, plain-type and nullable work, and it encodes the lookup copy, so reveal shows the marker. That contradicts the spec.
- **Rejected**: Rebasing the branch, which would carry the wrong reveal semantics.

### Fill from the engine's copy happens in the root package (discovery)
- **Decision**: Add the fill step in the root `xcl` package, behind a new public `Config` method that `coreservice` calls. It matches redacted `types.SensitiveValue` fields by reflection and finds the stored entity with `FindResource(meta.id)`.
- **Rationale**: `coreservice` may use only public packages, and record decoding is internal. The design says to make what `coreservice` needs public in the root module.
- **Rejected**: A fill inside `coreservice` (has no internal access); a JSON-level fill (cannot tell the marker from an ordinary string); a new `EncodeOption` (changes Go's encoder choices, a non-goal).

### Non-entity input raises NotEncodableError (discovery)
- **Decision**: Python raises `xcl.NotEncodableError` (what="a value of type X", reason="it is not an entity") for `None`, dicts and other non-dataclass objects. It raises `xcl.NotRegisteredError` (from `ClassMap.names_of`) for a dataclass instance whose class this configuration did not register.
- **Rationale**: Mirrors Go's `NotEncodableError{Reason: "it is not an entity"}`, and reuses the existing per-config registration error.
- **Rejected**: `NotAnEntityError`, which describes a lookup that reached a nested block, not an encode.

### Python keyword names (architecture)
- **Decision**: `include_computed`, `include_empty`, `reveal_sensitive`, `show_references`, `highlight`, all keyword-only and False by default, on both `encode_entity` and `encode_saved_entity`.
- **Rationale**: snake_case of Go's `IncludeComputed`, `IncludeEmpty`, `RevealSensitive` and `ShowReferences`; `reveal_sensitive` already matches `diff(..., reveal_sensitive=False)`.
- **Rejected**: the shorter `computed`, `empty` and `references`, which drift from Go's names and from PARITY.md's mapping.

### Name of a hand-built entity (architecture)
- **Decision**: label fields first, then `meta.name` when the class has no label field. The type and subtype always come from the registration. A named entity with no name raises `NotEncodableError` in Python before any engine call.
- **Rationale**: the mirror of `_labels_of`; the registration is authoritative for type and subtype (two label fields only exist when a subtype is registered).
- **Rejected**: letting the engine report "record has no name" as `InvalidSavedDataError`, which is confusing for a live object.

### Name of the public Go method (architecture)
- **Decision**: `(*Config).EncodeEntityRecord(data []byte, options ...EncodeOption) ([]byte, error)`, documented as encoding an entity record as a lookup returns it, possibly edited.
- **Rationale**: additive, sits beside `EncodeSavedEntity`, and is what coreservice needs made public.
- **Rejected**: an unexported method (coreservice cannot reach it).

### Conventions selected (architecture)
- **Decision**: apply testing-and-mocking, test location, test-state-from-real-apply, secret fixture, shared-errors-package, project-structure, code-style, never-modify-dependencies and dependencies; drop the graph-ordering, parser-options, reference-extraction, database, logging and provider-changed conventions.
- **Rationale**: only the kept ones touch the surfaces this feature changes.
- **Rejected**: listing every convention.

### Parity encode section shape (data_structures)
- **Decision**: `encode.<address>.{looked_up,saved,hand_built}.<combo>`, where combos are named by the choices turned on, joined with `+` in the order computed, empty, reveal, references (`none` when no choice is on). Hand-built addresses are listed in an optional per-fixture `hand_built.json`. Builtins are omitted.
- **Rationale**: compared as decoded JSON like the rest of the record, with a readable first difference; naming follows the earlier branch.
- **Rejected**: a separate text-file golden per combination (the harness has no golden files; Go output is produced live).

### Separate EncodeEntityResponse message (data_structures)
- **Decision**: a new `EncodeEntityResponse { bytes text = 1; }`, rather than reusing `EncodeSavedEntityResponse`.
- **Rationale**: follows the `FooRequest`/`FooResponse` naming the design asks buf lint to keep.
- **Rejected**: reusing the saved response (buf lint permits it, but the name would mislead).

### Parity is its own milestone (milestones)
- **Decision**: the parity `encode` stage (Go program, Python runner, new fixture) is Milestone 3, after the Python API.
- **Rationale**: it needs both sides to exist, and it is the release gate for the success metric. Separating it keeps Milestones 1 and 2 independently verifiable with ordinary tests.
- **Rejected**: folding parity into Milestone 1 (Python half missing) or Milestone 2 (mixes a cross-repo harness change into the API milestone).

### Highlight theme and renderer notes stay (milestones)
- **Decision**: the website's "With an editor theme" and "Writing a renderer" Python notes stay as they are; only the terminal highlight is offered, as today.
- **Rationale**: the spec's highlight requirement is "the same colouring the existing saved-record call uses" (terminal ANSI).
- **Rejected**: adding theme or renderer support to Python (out of scope).

### Release as a human task, re-pin as an agent task (tasks)
- **Decision**: "Release xcl with the encode call" is a human task (merging with a release label publishes a release). "Pin xcl-python to the released engine" is an agent task that depends on it.
- **Rationale**: the release rules in xcl-python README:405-425 require a released tag on main. Releasing is an action outside the repo.
- **Rejected**: leaving the pin on a branch (forbidden on main); one mixed task (it must be split by executor).

### Website gains a short "Empty attributes" section (tasks)
- **Decision**: add a short IncludeEmpty / include_empty section to configuration-text.mdx.
- **Rationale**: the spec requires the Python guide to name all four choices, and IncludeEmpty is documented nowhere on the site today.
- **Rejected**: naming it only in prose elsewhere (easy to miss).

### Feature slug is the spec name (assemble)
- **Decision**: the plan is addressed by its spec name `20261016064106-8931613a-python-encode-entity`, as the CLI's `plan_path` already fixes, not by a new `NNNN-` slug.
- **Rationale**: every existing plan in the store is named after its spec, and the workflow returned this plan_path.
- **Rejected**: inventing a numbered slug, which would not match the store's naming.

## Rehydration cues

- `spektacular spec file read 20261016064106-8931613a-python-encode-entity`
- `spektacular design read --data '{"source":"design","path":"libxcl.md"}'` (Lookups section, lines ~460-478)
- `spektacular knowledge always-applied --tier repo --filter xcl --filter xcl-python --filter xcl-website`
- Re-read `xcl:encode.go:150-270`, `xcl:coreservice/lookups.go:110-141`, `xcl-python:xcl/_schema.py:460-580`, `xcl-python:xcl/_entity.py`
- `git -C /home/nicj/code/github.com/jumppad-labs/xcl show 9c5dc1f --stat` for the earlier attempt's proto and parity shape

## User decisions recorded during discovery

- Website: the Python tabs of `xcl-website:src/pages/configuration-text.mdx` and the encode mentions in
  `events.mdx` are "xcl.dev's Python guide". There is no Python API reference page, and none is
  created.
- Event entities: the engine fills redacted values by `meta.id` for every record. An event entity
  kept and encoded after its operation shows the engine's current real value with reveal. This
  differs from Go for masked event data, and the docs say so.
