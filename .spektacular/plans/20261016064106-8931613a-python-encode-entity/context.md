---
created_date: "2026-10-16"
document_status: final
closed_date: "2026-10-16"
---

# Context: 20261016064106-8931613a-python-encode-entity

## Current State Analysis

- **Go encoder** (`xcl:encode.go`): `EncodeEntity(entity, options...)` :126 and `(*Config).EncodeSavedEntity(data, options...)` :154 share `encodeEntity` :181. The options are `IncludeComputed` :49, `IncludeEmpty` :67, `RevealSensitive` :79 and `ShowReferences` :99. Highlighting is a separate step (`highlight.ANSI.Render(highlight.Pieces(text))`). Builtins are refused with `ErrNotEncodable` :204. A redacted sensitive value stays the marker even with reveal (`sensitiveReplacement` :254-266).
- **Why Go reveals real values:** Go callers encode the stored entity pointer (`xcl:config.go:194,300-317`). `Find[T]` copies, and a plain-type copy is refused by the encoder (`xcl:encode.go:183-190`).
- **Core service** (`xcl:coreservice/lookups.go`): lookups marshal entities with plain `encoding/json`, so sensitive values reach the host as `"(sensitive)"` (:10-12, :24, :50, :72, :93). The `encodeSavedEntity` handler :120-141 calls `EncodeSavedEntity` with **no options** and colours on `highlight`. Lookups are refused while an operation runs (`operations.go:82-90`). The handler table is `service.go:205-226`, held to the descriptor by `service_test.go:299,308`.
- **Protocol** (`xcl:proto/xcl/core/v1/core.proto`): `EncodeSavedEntityRequest {entity=1; highlight=2}` :292-295 and `EncodeSavedEntityResponse {text=1}` :297-299. There is no `EncodeEntity` method. Fields are append-only, and `buf breaking` runs against main.
- **Python** (`xcl-python`): `encode_saved_entity(data, *, highlight=False)` (`xcl/config.py:213-216`, `xcl/aio.py:257-260`). There is **no instance-to-record serialiser** anywhere, because Python has no providers. Hydration (`xcl/_schema.py:466-578`) and `Meta.from_entity` (`xcl/_entity.py:40-55`) define the record shape to mirror. Type and subtype registrations are per configuration (`xcl/_classes.py:16-61`). `xcl.Entity.to_dict()` (`xcl/_entity.py:97-99`) is already a record. A looked-up `Sensitive` is redacted (`reveal()` returns None).
- **Parity** (`xcl:bindings/libxcl/cmd/parity/main.go:190-245`, `xcl-python:tests/parity/run_python.py:125-168`): the two sides write a JSON record compared by `test_parity.py`. There is no encode section today. Fixtures are under `xcl:bindings/libxcl/testdata/parity/`.
- **Docs:**
  - `xcl-python:PARITY.md:34` maps `EncodeSavedEntity` to `encode_saved_entity(data, highlight=False)`, and `:80` lists `EncodeEntity` under "Not applicable".
  - The xcl.dev page `xcl-website:src/pages/configuration-text.mdx` says "not yet available from Python" in its Python tabs (:52, :131, :152, :231).
  - There is no separate Python guide or API reference page. The user decided the configuration-text Python tabs and `events.mdx` count as the Python guide.
- **Prior attempt:** the unmerged branch `origin/spek/20261015072147-b41720ce-python-encoder` (xcl `9c5dc1f`) added `EncodeOptions` and a parity encode stage. It encoded the lookup copy, so reveal kept the marker, and it predates the plain-type and nullable work. It is reference only.

## Per-Task Technical Notes

### Requirement → repo and files

| Spec requirement | Repo | Main files |
|---|---|---|
| Encode a looked-up entity | xcl, xcl-python | `xcl:encode_record.go`, `xcl:coreservice/lookups.go`, `xcl-python:xcl/config.py`, `xcl-python:xcl/aio.py`, `xcl-python:xcl/_schema.py` |
| Encode a hand-built entity | xcl-python | `xcl-python:xcl/_schema.py`, `xcl-python:xcl/_entity.py` |
| Encode an untyped entity | xcl-python | `xcl-python:xcl/_schema.py` (`Entity.to_dict()`) |
| Same text as Go | xcl, xcl-python | `xcl:bindings/libxcl/cmd/parity/main.go`, `xcl-python:tests/parity/run_python.py` |
| Include computed / empty, highlight | xcl | `xcl:coreservice/lookups.go`, `xcl:proto/xcl/core/v1/core.proto` |
| Reveal sensitive (fill from the engine's copy) | xcl | `xcl:encode_record.go` |
| Show references | xcl | unchanged `xcl:encode.go:302-334`, reached through the options |
| Changes to a looked-up entity are kept | xcl, xcl-python | `xcl:encode_record.go` (fill skips real values), `xcl-python:xcl/_schema.py` |
| Saved records get the same choices | xcl, xcl-python | `xcl:coreservice/lookups.go:120-141`, `xcl-python:xcl/config.py:213-216`, `xcl-python:xcl/aio.py:257-260` |
| Failures raise the existing errors | xcl, xcl-python | `xcl:coreservice/errors.go`, `xcl-python:xcl/_schema.py`, `xcl-python:xcl/_errors.py` (unchanged) |
| Parity record, docs and design updated | xcl, xcl-python, xcl-website | `xcl-python:PARITY.md`, `xcl-python:README.md`, `xcl-website:src/pages/configuration-text.mdx`, `xcl-website:src/pages/events.mdx`, `design:libxcl.md` |

### Task: Add the encode messages to the core-service protocol

**File changes**:
- `xcl:proto/xcl/core/v1/core.proto:46` — add `rpc EncodeEntity(EncodeEntityRequest) returns (EncodeEntityResponse);` after `EncodeSavedEntity` in the Lookups group of `service CoreService` (:17-47). Its comment says it is refused during an operation, like the other lookups.
- `xcl:proto/xcl/core/v1/core.proto:292-299` — add `EncodeOptions options = 3;` to `EncodeSavedEntityRequest`. Add `message EncodeOptions { bool include_computed = 1; bool include_empty = 2; bool reveal_sensitive = 3; bool show_references = 4; }` with a comment naming the Go option each field maps to (`xcl.IncludeComputed()` and so on). Add `message EncodeEntityRequest { bytes entity = 1; bool highlight = 2; EncodeOptions options = 3; }`, documented as an entity's JSON record in the shape a lookup returns, possibly edited, with real values or the marker for sensitive fields. Add `message EncodeEntityResponse { bytes text = 1; }`. Keep the header rule (:7-10): append only.
- `xcl:coreservice/proto/core.pb.go` — regenerate with the Makefile `core-protos` target (`buf generate`, `buf.gen.yaml`, protoc-gen-go v1.36.12). Never hand-edit it.
- Run `make buf-lint` and `make buf-breaking`. Breaking against main must pass because only fields and messages are appended.
- Model: the unmerged branch `origin/spek/20261015072147-b41720ce-python-encoder` (`git show 9c5dc1f -- proto/`) used the same `EncodeOptions` field numbers.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Render an entity record with secrets filled from the engine

**File changes**:
- `xcl:encode_record.go` (new) — add `func (c *Config) EncodeEntityRecord(data []byte, options ...EncodeOption) ([]byte, error)` with godoc in the style of `encode.go:135-154`. The godoc covers: an entity's JSON record as a lookup returns it, possibly edited; fills redacted sensitive values from this configuration's copy (`meta.id`) only with `RevealSensitive`; errors as `EncodeSavedEntity`; no text on failure. The body mirrors `encode.go:154-177`:
  1. fold the options;
  2. `done, err := c.catalog.Use(nil)` / `defer done()`;
  3. `entity, err := savedentity.Decode(c.catalog, data, savedentity.ReadOptions{ForDisplay: true})`;
  4. if `opts.revealSensitive`, call `fillRedacted(entity, c.storedFor(entity))`;
  5. `return encodeEntity(entity, opts)`.
- `xcl:encode_record.go` — `storedFor(entity any) any`. It reads `types.GetMeta(entity)`. It returns nil when the meta has no `ID`, or when `c.FindResource(meta.ID)` (`config.go:305-317`) fails. A not-found result is not an error. A builtin's id is irrelevant, because `encodeEntity` refuses builtins anyway (`encode.go:204`).
- `xcl:encode_record.go` — `fillRedacted(decoded, stored any)`, which calls a recursive `fillValue(dst, src reflect.Value)`:
  - return when either value is invalid, or when `dst.Type() != src.Type()`;
  - if the type implements `types.SensitiveValue` (`types/sensitive.go:142-148`), and `types.IsRedacted(dst)` holds while `!types.IsRedacted(src)`, then `dst.Set(src)` when `dst.CanSet()`;
  - pointers: recurse into `Elem()` when both are non-nil;
  - structs: recurse field by field over exported fields, skipping the embedded `types.ResourceBase` / `Meta` bookkeeping;
  - slices and arrays: recurse by index up to the shorter length;
  - maps: for each key in `dst` that is also in `src`, recurse on a settable copy of the element (`reflect.New(elemType).Elem()`, set, recurse, then `SetMapIndex`);
  - interfaces: recurse into `Elem()`.

  It never creates elements and never touches a value that is not redacted. The decoded entity and the stored entity are both pointers, so start from `reflect.ValueOf(x).Elem()`.
- `xcl:encode_record_test.go` (new). Model it on `encode_sensitive_test.go:17-172` (`applyEncodeSensitiveConfig`, `registered.Secret` per the knowledge entry) and `encode_test.go:36-123` (`applyEncodeFixture`, `encodeEntityByID`). Records come from `json.Marshal(stored)`, the lookup form that `coreservice/lookups.go:24` produces, and from edits to that map. Separate tests (no tables; testify `require`; positive and negative split):
  - `TestEncodeEntityRecordMatchesEncodeEntity` (no options), and one test each for `IncludeComputed`, `IncludeEmpty` and `ShowReferences` against `EncodeEntity` on the stored entity;
  - `TestEncodeEntityRecordRevealFillsFromStoredEntity`;
  - `TestEncodeEntityRecordWithoutRevealShowsMarker`;
  - `TestEncodeEntityRecordRevealKeepsEditedSensitiveValue`;
  - `TestEncodeEntityRecordKeepsEditedValue`;
  - `TestEncodeEntityRecordRevealWithoutIDShowsMarker` and `TestEncodeEntityRecordRevealWithUnknownIDShowsMarker`;
  - `TestEncodeEntityRecordRevealFillsNestedSensitiveValues`, using `registered.SecretShape` (`internal/test_fixtures/registered/*.go:110`, a map of Sensitive and a pointer block);
  - `TestEncodeEntityRecordRefusesVariable` (and output and module, one test each);
  - `TestEncodeEntityRecordRecordWithoutMetaIsInvalid`;
  - `TestEncodeEntityRecordUnregisteredType`.

  Use `testutil.EntityByID` (`internal/testutil`) to get the stored entity.
- Do not change `encode.go`, `types/sensitive.go` or `internal/xcl/gohcl` (MPL; never-modify-dependencies convention).

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Medium. One agent writes `encode_record.go`; a second writes the tests in parallel from this spec. Integrate and run `go test ./...` and `go vet ./...` sequentially.

### Task: Serve entity encoding and saved-record choices from the core service

**File changes**:
- `xcl:coreservice/lookups.go:120-141` — factor two helpers out of `encodeSavedEntity`:
  - `encodeOptions(o *corepb.EncodeOptions) []xcl.EncodeOption`: nil gives none; each true field appends `xcl.IncludeComputed()`, `xcl.IncludeEmpty()`, `xcl.RevealSensitive()` or `xcl.ShowReferences()`;
  - `colour(text []byte, on bool) []byte`: `highlight.ANSI.Render(highlight.Pieces(text))` when on.

  Make `encodeSavedEntity` pass `encodeOptions(req.GetOptions())...` to `conf.cfg.EncodeSavedEntity`. Add `func (s *Service) encodeEntity(conf *configuration, req *corepb.EncodeEntityRequest) (*corepb.EncodeEntityResponse, error)`, which does the following in order:
  1. `lookupAllowed("EncodeEntity")` (`operations.go:84-90`);
  2. an empty entity gives `invalidArgument("an entity is required", "argument", "entity")`;
  3. `conf.cfg.EncodeEntityRecord(req.GetEntity(), encodeOptions(req.GetOptions())...)`;
  4. `colour`.
- `xcl:coreservice/service.go:205-226` — register `"EncodeEntity": unary(true, (*Service).encodeEntity)` beside `"EncodeSavedEntity"` (:225). The descriptor tests `service_test.go:299,308` then pass.
- `xcl:coreservice/lookups_test.go` — model on `TestEncodeSavedEntityRendersText` :154, `…HighlightAddsColour` :165, `…EmptyIsInvalidArgument` :175, the `appRecord` helper :24, and `appSchema`/`appConfig` (`service_test.go:19-28`, sensitive `password = "s3cret"`). New separate tests:
  - `TestEncodeEntityRendersText`;
  - `TestEncodeEntityRevealShowsConfiguredSecret` (looked-up record carries the marker; reveal yields `"s3cret"`);
  - `TestEncodeEntityWithoutRevealShowsMarker`;
  - `TestEncodeEntityHighlightAddsColour` (strip ANSI and compare to plain);
  - `TestEncodeEntityHighlightMatchesSavedRecord`;
  - `TestEncodeEntityEmptyIsInvalidArgument`;
  - `TestEncodeEntityVariableIsNotEncodable`;
  - `TestEncodeEntityUnregisteredType`;
  - `TestEncodeSavedEntityIncludeComputed`, `…IncludeEmpty`, `…RevealSensitive` and `…ShowReferences`, one each, compared to `EncodeEntity` with the same option;
  - `TestEncodeSavedEntityWithoutOptionsUnchanged` (equal to the text produced with an unset `options`).
- `xcl:coreservice/operations_test.go:728-750` — add `TestEncodeEntityDuringOperationRefused`, a copy of `TestLookupDuringOperationRefused` using `newPluginConfig` and `holdCreate`. If the saved-record case is not already covered there, add `TestEncodeSavedEntityDuringOperationRefused`.
- `xcl:coreservice/errors.go:266-277` — no change. Mapping for `not_encodable`, `unregistered_type` and `invalid_saved_data` already exists.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential execution. The handler is small; the tests depend on it.

### Task: Describe the encode call in the xcl changelog and the shared-library design

**File changes**:
- `xcl:CHANGELOG.md:1-3` — add `## 20261016064106-8931613a-python-encode-entity` at the top, in the prose style of :3-9. It covers:
  - the new `EncodeEntity` core-service call and `EncodeOptions`;
  - the saved-record call's options, with no options meaning today's text;
  - `Config.EncodeEntityRecord` and its reveal fill by `meta.id`, including that a kept event entity reveals the engine's current value.

  End with `**Breaking:**` and `None.`, because fields were only appended.
- `design:libxcl.md` (Lookups section, around lines 460-478 of the read document; Threading list :199; service block :243) — make the edits through the CLI only:
  1. `spektacular design read --data '{"source":"design","path":"libxcl.md"}'` into `.spektacular/tmp/libxcl.md`;
  2. edit the staged copy:
     - add `rpc EncodeEntity(EncodeEntityRequest) returns (EncodeEntityResponse);` to the service block;
     - add `EncodeEntity` to the lookups refused while an operation runs (Threading);
     - in the Lookups protobuf block, add `EncodeOptions`, `options = 3` on `EncodeSavedEntityRequest`, and `EncodeEntityRequest`/`EncodeEntityResponse`;
     - add a paragraph: encoding is a lookup; the record is lookup-shaped JSON; with `reveal_sensitive` the engine fills marker values from its own copy found by `meta.id`; a hand-built record holds real values; highlight is applied after rendering;
  3. write it back with `spektacular design write --data '{"source":"design","path":"libxcl.md"}' --from .spektacular/tmp/libxcl.md`, then remove the staged file.

  Never Write or Edit the store file directly. The working tree already holds an uncommitted edit to this document (the spec reference added to its frontmatter). Read the current content through the CLI and keep it.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Regenerate the xcl-python protocol messages

**File changes**:
- `xcl-python:xcl/_core/core_pb2.py`, `xcl-python:xcl/_core/core_pb2.pyi` — regenerate with `make protos` (`protoc -I $(XCL_SRC)/proto --python_out --pyi_out`, with `XCL_SRC=../xcl` checked out on the xcl feature branch). Never hand-edit.
- `xcl-python:xcl-ref.txt` — set it to the xcl feature branch name while in review (CI `.github/workflows/test.yml:68-101` checks out that ref and runs `make protos engine plugins`). The final pin to a tag is task "Pin xcl-python to the released engine".

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Turn a Python entity into the record the engine reads

**File changes**:
- `xcl-python:xcl/_entity.py:40-55` — add `Meta.to_record(self) -> dict[str, Any]`, the mirror of `from_entity`. It writes the known keys that are set (`id`, `name`, `type`, `subtype`, `module`, `file`, `line`, `column`, `status`, `links`, `references`), skipping empty values so the engine's omitempty shape is matched, then merges `extra`. It does not include `disabled` or `depends_on`, because the caller writes those beside meta. Keep the known-key list in one place, shared with `from_entity`.
- `xcl-python:xcl/_schema.py` (beside `hydrate`, :466-578) — add `record_of(entity: object, classes: ClassMap) -> dict[str, Any]`:
  - `isinstance(entity, Entity)` returns `entity.to_dict()` (`_entity.py:97-99`).
  - A non-dataclass instance (including a dataclass class object, `None` and dicts) raises `NotEncodableError(f"a value of type {type(entity).__name__} is not an entity", details={"what": ..., "reason": "it is not an entity"})`. Mirror the hand-built error pattern at `config.py:224-228`.
  - `type_, subtype = classes.names_of(type(entity))` (`_classes.py:51-61`) raises `NotRegisteredError` for an unregistered class.
  - Build meta: start from `entity.meta.to_record()` if the class has a top-level `meta` field set, else `{}`. Set `type` and `subtype` (omit an empty subtype) from the registration. The name comes from the label field. With two label fields (`label_fields_of`, :159-161) the second label field holds the name and the first is ignored in favour of the registered subtype. Otherwise the name is `meta.name`. A non-singleton (`is_singleton`, :102-109) with an empty name raises `NotEncodableError(... reason "it has no name; set its label field or meta.name")`.
  - `disabled` (only when true) and `depends_on` (only when non-empty) come from `entity.meta`, written beside `meta` as `_entity.py:53-54` reads them.
  - Fields: `_record_into(cls, instance, top_level=True)`, mirroring `_hydrate_into` (:493-515). For each dataclass field: skip a top-level `meta` field; skip top-level label fields (they went into meta); key `options.name or f.name`; omit `None`; else `_record_value(annotation, value)`.
  - `_record_value`, mirroring `_hydrate_value` (:534-565): unwrap `Annotated`, then `T | None`. `Sensitive` gives the marker `SENSITIVE_MARKER` when `is_redacted()`, else `reveal()`. A dataclass instance gives `_record_into(..., top_level=False)`, which writes nested label fields as keys. Lists and dicts recurse. Anything else is passed through.
  - Finally `json.dumps(record)` inside `record_of` callers (not here). The callers catch `TypeError`/`ValueError` and raise `NotEncodableError(reason="it holds a value with no configuration form")`. Alternatively expose `record_bytes(entity, classes) -> bytes` in `_schema.py` doing both; prefer the latter, so sync and aio share it.
- `xcl-python:tests/unit/test_record.py` (new). Use the hydration matrix classes from `tests/unit/test_hydrate.py:30-41,79-86` and a `ClassMap` built from a `LocalRegistry`. Separate tests:
  - `test_record_of_hydrated_instance_round_trips` (record → `hydrate` → `record_of` gives the record back, with keys omitted where hydration had none);
  - `test_record_of_writes_labels_into_meta`;
  - `test_record_of_two_labels_takes_registered_subtype`;
  - `test_record_of_singleton_has_no_name`;
  - `test_record_of_hand_built_sensitive_is_real_value`;
  - `test_record_of_redacted_sensitive_is_marker`;
  - `test_record_of_renamed_field_uses_schema_name`;
  - `test_record_of_nested_block_labels_are_keys`;
  - `test_record_of_disabled_and_depends_on_beside_meta`;
  - `test_record_of_untyped_entity_is_its_dict`.

  Negative tests, one per case:
  - `test_record_of_none_is_not_encodable`;
  - `test_record_of_dict_is_not_encodable`;
  - `test_record_of_unregistered_class_raises_not_registered`;
  - `test_record_of_unnamed_entity_is_not_encodable`;
  - `test_record_bytes_unserialisable_value_is_not_encodable`.
- `xcl-python:tests/unit/test_entity.py` — `test_meta_to_record_round_trips_from_entity`.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Medium. One agent writes `Meta.to_record` and `record_of`/`record_bytes`; a second writes the unit tests from this spec. Integrate with `pytest tests/unit`.

### Task: Add encode_entity and the saved-record choices to both configuration classes

**File changes**:
- `xcl-python:xcl/config.py:213-216` — add a module-level helper `_encode_options(include_computed, include_empty, reveal_sensitive, show_references) -> core_pb2.EncodeOptions`. Change `encode_saved_entity(self, data, *, include_computed=False, include_empty=False, reveal_sensitive=False, show_references=False, highlight=False)` to send `options=_encode_options(...)`. Add `encode_entity(self, entity, *, include_computed=False, include_empty=False, reveal_sensitive=False, show_references=False, highlight=False) -> str`: `record_bytes(entity, self._classes)`, then `self._client.call(self._open(), "EncodeEntity", core_pb2.EncodeEntityRequest(entity=..., highlight=highlight, options=...))`, then `.text.decode()`. Docstrings name each choice (`tests/unit/test_docstrings.py` requires them).
- `xcl-python:xcl/aio.py:257-260` — the same two methods as `async def`, via `await self._lookup("EncodeEntity", ...)` (:215-217), reusing `config._encode_options` and `record_bytes`, following how aio reuses sync helpers (:75, :234). The record is built before `_lookup`, so Python-side errors raise without an engine call.
- `xcl-python:tests/integration/sync_entities.py` — add a class with a sensitive field, a reference and the empty-field forms if `SyncApp` lacks them, plus a subtype class (`SyncDatabase` exists).
- `xcl-python:tests/integration/test_sync.py:775-807` — beside the existing encode tests, add separate tests:
  - `test_encode_entity_from_find`, `…_from_find_by_type`, `…_from_find_one`, `…_from_all`, `…_from_entities` and `…_from_kept_event_entity` (header `<type> "<subtype>" "<name>"` and `<type> "<name>"`);
  - `test_encode_entity_hand_built_matches_loaded`;
  - `test_encode_entity_untyped_plugin_entity` (fixture plugin type with no class);
  - `test_encode_entity_computed_off_by_default` and `test_encode_entity_include_computed`;
  - `test_encode_entity_empty_off_by_default` and `test_encode_entity_include_empty` (`""`, `[]`, `{}`, nullable zero);
  - `test_encode_entity_sensitive_masked_by_default`, `test_encode_entity_reveal_shows_configured_value` and `test_encode_entity_hand_built_reveal_shows_held_value`;
  - `test_encode_entity_show_references` and `test_encode_entity_sensitive_bare_reference_shown`;
  - `test_encode_entity_sensitive_template_marker_unless_reveal`;
  - `test_encode_entity_hand_built_references_unchanged`;
  - `test_encode_entity_highlight_strips_to_plain`;
  - `test_encode_entity_highlight_matches_saved_record`;
  - `test_encode_entity_shows_edited_value` and `test_encode_entity_reveal_shows_edited_secret`;
  - `test_encode_saved_entity_with_each_choice_matches_encode_entity` (one test per choice);
  - `test_encode_saved_entity_without_choices_unchanged`.

  Separate error tests, one per case:
  - `test_encode_entity_not_an_entity`;
  - `test_encode_entity_variable_not_encodable`;
  - `test_encode_entity_output_not_encodable`;
  - `test_encode_entity_module_not_encodable`;
  - `test_encode_entity_value_with_no_configuration_form`;
  - `test_encode_entity_unregistered_class`.

  Add `test_encode_entity_during_operation_refused`, raised from inside an `on_event` handler. Use `MemoryStore.record(address)` for saved records.
- `xcl-python:tests/integration/test_aio.py:63-78` — add `encode_entity` to `OPERATIONS_AND_LOOKUPS`. Add aio twins of the lookup-kind, reveal, highlight, error and refusal tests near `test_lookups_are_awaitable` (:483-525). `test_sync_and_async_expose_same_surface` (:586-593) must keep passing.
- `xcl-python:tests/unit/test_config.py` — with `tests/fakes.py` `FakeTransport`: `test_encode_entity_sends_options` and `test_encode_saved_entity_sends_options` (each keyword lands in `EncodeOptions`), `test_encode_saved_entity_default_options_unset`.

**Complexity**: High
**Token estimate**: ~60k tokens
**Agent strategy**: High. Analyse in parallel: one agent for the API methods plus unit tests, one for the sync integration tests, one for the aio integration tests. Integrate sequentially and run the full suite against an engine built from the xcl branch (`make protos engine plugins` with `XCL_SRC`).

### Task: Document entity encoding in the xcl-python README and parity record

**File changes**:
- `xcl-python:PARITY.md:34` — change the row to `| EncodeSavedEntity(data, options...) | encode_saved_entity(data, include_computed=False, include_empty=False, reveal_sensitive=False, show_references=False, highlight=False) | await encode_saved_entity(...) |`. Add a row `| EncodeEntity(value, options...) | encode_entity(entity, include_computed=..., ..., highlight=False) | await encode_entity(...) |`. Add a note mapping `IncludeComputed()`→`include_computed` and so on. Highlight is the terminal colouring Go applies as `highlight.ANSI.Render(highlight.Pieces(text))`.
- `xcl-python:PARITY.md:80` — remove the `EncodeEntity` row from "Not applicable" (:74).
- `xcl-python:README.md:239-245` — add `encode_entity(entity)` to the lookups list. Add an "Encoding entities as configuration" subsection after the lookups paragraph (before Asyncio, :250). It covers:
  - the call on a looked-up, hand-built or untyped entity;
  - the four keywords and `highlight`;
  - reveal showing a looked-up entity's configured secret, and edits kept;
  - a kept event entity revealing the engine's current value, which differs from Go for masked event data;
  - `encode_saved_entity` taking the same keywords;
  - the errors (`NotEncodableError`, `NotRegisteredError`, `OperationInProgressError`);
  - the asyncio form in one sentence.

  Update `README.md:189-192` (looked-up `Sensitive.reveal()` returns None) to point at `encode_entity(..., reveal_sensitive=True)` for display.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Record encoded text in the Go parity program

**File changes**:
- `xcl:bindings/libxcl/cmd/parity/main.go:190-245` — after `record["state"] = n.state(stateDir)` (:222), add `record["encode"] = n.encode(c, stateDir, fixtureDir)`. The new `encode` method:
  - iterates `c.Entities()` sorted by `meta.ID`, skipping variable, output, module and root;
  - for each of the 16 combinations of `[]string{"computed","empty","reveal","references"}`, keyed by the `+`-joined names that are on (or `none`), writes:
    - `looked_up[key] = xcl.EncodeEntity(entity, opts...)`;
    - `saved[key]` = `c.EncodeSavedEntity(record, opts...)`, where `record` is the entity's state record read from `stateDir`, using the same reader `n.state` uses;
  - for addresses in an optional `hand_built.json` (JSON array) in the fixture directory, writes `hand_built[key]` = the same text as `looked_up`.

  An encode error is recorded as `n.outcome(err)` so both sides compare it. Text goes through `n.text`/path normalisation only if it holds absolute paths (it should not).
- `xcl:bindings/libxcl/cmd/parity/main_test.go` — add separate tests:
  - `TestRunRecordsEncodeSection` (keys present, 16 combinations);
  - `TestRunEncodeLeavesOutBuiltins`;
  - `TestRunEncodeHandBuiltOnlyForListedAddresses`.
- `xcl:bindings/libxcl/testdata/parity/encode/types.json`, `xcl:bindings/libxcl/testdata/parity/encode/main.xcl` and `xcl:bindings/libxcl/testdata/parity/encode/hand_built.json` (new). The types:
  - a `store` type with subtype `postgres` and a sensitive `password`, an optional string, a nullable attribute, an optional list and an optional map;
  - a `service` type with no subtype, whose `location` refers to the store, whose `secret` is a sensitive bare reference to the store's password, and whose `token` is a sensitive template `"${...}-x"`.

  Schemas use schema v1 with `"nullable": true` where Python would mark `T | None`. `hand_built.json` lists the store's address (no references). The model is the old branch's `testdata/parity/encode/` (`git show 9c5dc1f`).
- `xcl:bindings/libxcl/testdata/parity/README.md` — document `hand_built.json` and the `encode` section.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Medium. One agent for the program stage and its tests; one for the fixture and README. Integrate and run `go test ./...` in `bindings/libxcl`.

### Task: Record encoded text in the Python parity runner

**File changes**:
- `xcl-python:tests/parity/run_python.py:125-168` — after `record["state"]` (:150), add `record["encode"] = encode_section(config, state_dir, fixture)`, the same shape as Go:
  - entities: `config.entities()`, sorted by `meta.id` like `_sorted_by_id` (:363), skipping builtins;
  - `looked_up[key]` = `config.encode_entity(config.find(None, address), **choices)`;
  - `saved[key]` = `config.encode_saved_entity(record_bytes, **choices)`, with state records read the way `n.state` (:278) reads them;
  - `hand_built[key]` = `config.encode_entity(HAND_BUILT[fixture][address](), **choices)` for the addresses in `hand_built.json`.

  An error goes through `n.outcome`. Combination keys are built with the same order and `+` join as Go.
- `xcl-python:tests/parity/types.py:125-137` — add the `encode` fixture's dataclasses (`EncodeStore` with label field `name`, `meta: xcl.Meta | None`, `password: xcl.Sensitive[str] | None`, and the optional, nullable, list and map fields; and `EncodeService`) to `FIXTURES`. Add `HAND_BUILT: dict[str, dict[str, Callable[[], object]]]`, building the store with the literal values from `main.xcl`, including `xcl.Sensitive("<password>")`.
- `xcl-python:tests/parity/test_types.py` — covered automatically through `FIXTURES` (schema equals `types.json`).
- `xcl-python:tests/parity/test_parity.py` — no change. The `first_difference` comparison covers the new section.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution. Run `pytest tests/parity` with the Go parity program built by `make plugins` from the xcl branch.

### Task: Release xcl with the encode call

**What the person does**: Opens or confirms the xcl PR carrying Milestones 1 and 3 with exactly one release label (`minor`), as `xcl/docs/release-setup.md` and `xcl/README.md:246-320` describe. Confirms the xcl-python PR, pinned to the xcl branch, is green, which proves the pair. Then merges the xcl PR. Label it `breaks-bindings` only if the required `bindings` check fails on Python's `main`; it should not, since fields are only appended.

**How the result is checked**: A new xcl release tag exists on GitHub. The automated "Use xcl engine <tag>" PR opens in xcl-python (`xcl-python:.github/workflows/engine-bump.yml`).

### Task: Pin xcl-python to the released engine

**File changes**:
- `xcl-python:xcl-ref.txt` — replace the feature-branch ref with the released tag from "Release xcl with the encode call", following `xcl-python:README.md:405-425`. Alternatively, rebase the work onto the `engine/vX.Y.Z` branch the bump workflow opened.
- `xcl-python:xcl/_core/core_pb2.py`, `xcl-python:xcl/_core/core_pb2.pyi` — regenerate from the tag with `make protos`. They should be unchanged from the branch.

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Show encode_entity in the website configuration-text guide

**File changes**:
- `xcl-website:src/pages/configuration-text.mdx:20-24` — the intro names Python's `cfg.encode_entity` and `cfg.encode_saved_entity` beside Go's.
- `xcl-website:src/pages/configuration-text.mdx:52` — replace the "not yet available from Python" Python tab with `text = cfg.encode_entity(db)` (db from `cfg.find(Postgres, "...")`), plus one sentence: "With `xcl.aio.Config` the call is `await cfg.encode_entity(db)`." Add a sentence on a hand-built instance and on `xcl.Entity`.
- `xcl-website:src/pages/configuration-text.mdx:90-103` — show `cfg.encode_saved_entity(event.data, include_computed=True)` taking the same choices.
- `xcl-website:src/pages/configuration-text.mdx:125-131` — replace "Python has no `EncodeEntity`…" with `cfg.encode_entity(event.entity())`. Say that an event entity kept and encoded after its operation shows the engine's current value with `reveal_sensitive=True`, unlike Go, where event data always shows the marker.
- `xcl-website:src/pages/configuration-text.mdx:152` — Python tab `cfg.encode_entity(db, include_computed=True)`.
- `xcl-website:src/pages/configuration-text.mdx:165-171` — add `reveal_sensitive=True` for Python. A looked-up entity shows its configured value.
- `xcl-website:src/pages/configuration-text.mdx` (new short section after "Provider-filled values", in the same prose-then-CodeTabs shape) — "Empty attributes": Go `xcl.IncludeEmpty()`, Python `include_empty=True`, with wording from the godoc `xcl:encode.go:55-71`.
- `xcl-website:src/pages/configuration-text.mdx:231` — Python tab `cfg.encode_entity(app, show_references=True)`.
- `xcl-website:src/pages/configuration-text.mdx:298-330` — show `highlight=True` on `encode_entity` as well. Keep the theme note (:363) and the renderer note (:410).
- `xcl-website:src/pages/events.mdx:211-213` — mention `cfg.encode_entity(event.entity())` beside `cfg.encode_saved_entity`.

Follow the CodeTabs conventions (`src/components/CodeTabs.astro`: `go` and `python` slots, with blank lines inside the divs). The wording follows the xcl-python README section. Check with `make build` and `make check` (`npm run build`, `npx astro check`). Leave the home page badge (`src/pages/index.mdx:22`) for the release that ships it.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution

## Testing Strategy

- **Render an entity record with secrets filled from the engine** (xcl root): Go unit tests on `EncodeEntityRecord` against real applies, using `registered.Secret` and `registered.SecretShape`. Covered: equality with `EncodeEntity` per option; the reveal fill and the edited-value rule; no-id and unknown-id records; nested, list and map fill; and errors for builtins, missing meta and unregistered types. One behaviour per test, no tables.
- **Serve entity encoding and saved-record choices from the core service** (xcl coreservice): `Service.Call` tests for both calls. Covered: text; each option; reveal with a looked-up record; highlight stripping to plain and matching across calls; an empty record; `not_encodable` and `unregistered_type`; unchanged no-option saved text; and refusal during an operation through `holdCreate`.
- **Add the encode messages to the core-service protocol**: no new tests. `buf lint`, `buf breaking` and the descriptor tests are the gate.
- **Record encoded text in the Go parity program** (xcl bindings): program tests for the presence of the encode section, the 16 combination keys, the exclusion of builtins and hand-built filtering.
- **Turn a Python entity into the record the engine reads** (xcl-python unit): the round trip `record_of(hydrate(record)) == record` for every annotation form, plus label, name, sensitive and nested-label handling, and one test per error.
- **Add encode_entity and the saved-record choices to both configuration classes** (xcl-python unit and integration): fake-transport tests that the keywords land in `EncodeOptions`. Real-engine sync and aio tests cover each lookup kind, hand-built and untyped entities, each choice default-off and opt-in, reveal, edits, highlight, every error case and refusal during an operation.
- **Record encoded text in the Python parity runner** (xcl-python parity): the existing comparison over the new section is the behavioural test for the success metric, byte-identical text for every fixture, entity and combination, for looked-up and saved records.
- **Docs tasks** (changelog, design, README, PARITY.md, website): no tests read docs (project convention). Reviewed by a person, as listed under the plan's Testing Approach manual items. The website's gate is `npm run build` and `npx astro check`.

## Project References

- Spec: `20261016064106-8931613a-python-encode-entity` (final).
- Design: `libxcl.md` from the `design` source (Lookups section and Threading list to update).
- Knowledge entries (xcl repo):
  - conventions: testing-and-mocking, shared-test-helpers, test-state-from-real-apply, shared-errors-package, project-structure, code-style, never-modify-dependencies and dependencies;
  - gotchas: custom-marshaljson-changes-internal-hops, save-state-through-encodeforstate and xcl-tags-gate-what-reaches-cty;
  - learnings: secret-fixture-for-sensitive-tests;
  - architecture: links-are-attribute-paths;
  - glossary: entity.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.
- Release process: `xcl:docs/release-setup.md`, `xcl:README.md:246-320`, `xcl-python:README.md:405-425`, `xcl-python:.github/workflows/engine-bump.yml`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Build xcl first (Milestone 1 and the Go parity task), with `XCL_SRC` pointing xcl-python at that checkout. The Python tasks then build the engine and parity program from it with `make protos engine plugins`.

## Migration Notes

None for users:
- the protocol only gains fields and a method;
- `encode_saved_entity` keeps its signature, with new keywords defaulting off;
- Go's API only gains a method.

During review, `xcl-python:xcl-ref.txt` points at the xcl feature branch. It must be re-pinned to the released tag before xcl-python merges to main (task "Pin xcl-python to the released engine").

## Performance Considerations

- An encode is one extra core-service round trip, the same as `encode_saved_entity`.
- The reveal fill costs one `FindResource` (a linear match over the configuration's entities) plus one reflective walk of a single entity, and runs only when reveal is asked for.
- The parity `encode` stage adds 16 to 48 encodes per entity per fixture. That is small beside apply, but it lengthens the parity job slightly.
