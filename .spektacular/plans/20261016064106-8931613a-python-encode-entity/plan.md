---
created_date: "2026-10-16"
document_status: final
closed_date: "2026-10-16"
---

# Plan: 20261016064106-8931613a-python-encode-entity

<!-- Metadata -->
<!-- Created: 2026-10-16T08:02:32Z -->
<!-- Commit: 41774ed -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git (also xcl-python, xcl-website) -->

## Overview

Python programs that embed xcl gain `encode_entity`, the counterpart of Go's `EncodeEntity`. It turns any entity they hold into xcl configuration text: one from a lookup or an event, a dataclass instance they built themselves, or the generic entity of a plugin type. It takes the same four choices Go offers (include computed fields, include empty fields, reveal sensitive values, show references as written) plus terminal highlighting, and `encode_saved_entity` gains the same choices. The text is rendered by xcl's shared engine through a new core-service call. A looked-up entity's secrets are filled from the engine's own copy when revealing, so Python and Go produce byte-identical text, and a parity suite proves it across every fixture and every combination of choices. The Python package's README and parity record, xcl.dev's configuration-text guide and the shared-library design document are updated to match.

## Conventions

- **Testing & Mocking (testify `require`, no table-driven tests, never mix positive and negative cases in one test, no tests that read docs, README, changelog or CI files)** — every Go test added to `xcl` (root encode-record tests, `coreservice` handler tests, parity program tests) follows it. The xcl-python tests follow the same rules. Docs, PARITY.md and the website are checked by human review only.
- **Locate tests with the source they test** — `encode_record_test.go` sits beside `encode_record.go`, handler tests sit in `coreservice/`, and parity program tests sit in `bindings/libxcl/cmd/parity/`.
- **Generate test state with a real apply, not a hand-written state file** — saved-record tests and the parity `saved` section read records written by a real apply, never hand-written records.
- **registered.Secret is the fixture for sensitive-record tests** (learning) — root-package reveal and fill tests use `registered.Secret` rather than a new fixture type.
- **Shared error types live in the `errors` package** — no new error type is needed. The new method returns the existing `ErrNotEncodable`, `ErrUnregisteredType` and `ErrInvalidSavedData`. Any new error would have to go there.
- **Project structure: public packages at the module's top level** — the new public method sits in the root `xcl` package, the only place `coreservice` may reach.
- **Go code style / standard library idiom** — the new reflection walk and method follow the existing style of `encode.go`.
- **NEVER modify dependencies; `internal/xcl` is MPL** — the fill is written in the Apache-licensed root package. `internal/xcl/gohcl` is not changed.
- **Dependencies: prefer the standard library** — no new Go or Python dependency is added.
- Deliberately not applied: assert-ordering-on-graph-parents, config-parser-options-builder, find-references-with-hclsyntax-variables, database-and-external-services, development-standards (logging), and testing-provider-changed-and-update. This feature touches no graph ordering, parser options, reference extraction, database, logging or provider lifecycle.

## Architecture & Design Decisions

The feature is built to the settled shared-library design (`design:libxcl.md`). Encoding is a core-service message, the entity crosses as JSON bytes, and the call is a lookup under the lookup rules. The work lands in three repos, in this order.

- **xcl** (`/home/nicj/code/github.com/jumppad-labs/xcl`) carries the engine side:
  - `proto/xcl/core/v1/core.proto` gains:
    - a shared `EncodeOptions` message (`include_computed`, `include_empty`, `reveal_sensitive`, `show_references`);
    - an `options` field on `EncodeSavedEntityRequest`;
    - a new `rpc EncodeEntity(EncodeEntityRequest) returns (EncodeEntityResponse)`, whose request carries the entity record, `highlight` and `options`.
  - `coreservice/lookups.go` gains an `encodeEntity` handler beside `encodeSavedEntity`. The two handlers share:
    - the options translation into `[]xcl.EncodeOption`;
    - the highlight step, `highlight.ANSI.Render(highlight.Pieces(text))`, so both colour identically;
    - the `lookupAllowed` refusal, so both are refused while an operation runs.
  - The root package gains one public method, `(*Config).EncodeEntityRecord(data []byte, options ...EncodeOption)`, in a new `encode_record.go`. It mirrors `EncodeSavedEntity` (`encode.go:154-177`):
    1. load plugins with `catalog.Use`;
    2. decode the record with `savedentity.Decode` and `ForDisplay`;
    3. fill redacted values from the configuration's own copy of the entity, but only when `RevealSensitive` is given;
    4. call the shared `encodeEntity`.

  Both Python calls therefore reach Go's single rendering path, and identical text comes for free. The bindings' parity program (`bindings/libxcl/cmd/parity`) and its fixtures gain an `encode` stage.
- **xcl-python** (`/home/nicj/code/github.com/jumppad-labs/xcl-python`):
  - regenerates its protos;
  - adds `encode_entity` beside `encode_saved_entity` on both `xcl.Config` and `xcl.aio.Config`. Its keyword choices `include_computed`, `include_empty`, `reveal_sensitive`, `show_references` and `highlight` are all off by default. They are the snake_case of Go's option names, in the keyword-only style of `diff(*paths, reveal_sensitive=False)`;
  - gives `encode_saved_entity` the same four keywords.
- **xcl-website** (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) updates the Python tabs of `configuration-text.mdx` and the mentions in `events.mdx`. The user decided these are xcl.dev's Python guide. No API reference page exists, and none is created.

**Filling from the engine's copy, and where it lives.** Lookup JSON carries the marker, because `coreservice` marshals with plain `encoding/json` and `Sensitive.MarshalJSON` writes `(sensitive)`. Go shows the real value only because it encodes the stored pointer. With reveal, `EncodeEntityRecord` therefore:
1. looks up `meta.id` with the public `FindResource` (`config.go:305-317`);
2. walks the decoded and stored values together by reflection: struct fields, pointers, slices by index and maps by key;
3. copies the stored `Sensitive[T]` wherever the decoded one is redacted (`types.IsRedacted`, `types/sensitive.go:153`) and the stored one is not.

These rules follow:
- An edited sensitive field arrives holding a real value and is left alone, so it shows the edit.
- An unedited one arrives as the marker and is filled from the configured value.
- A hand-built record has no id, or an id that matches nothing, so nothing is filled and nothing fails. Its real values show because `Sensitive.UnmarshalJSON` keeps them.
- An entity taken from an event and kept until after its operation is filled like any other record. The user decided this. It differs from Go only for masked event data, and the docs say so.

The fill sits in the root package, not in `coreservice`. Record decoding is internal, and `coreservice` may only use public packages; the design's rule is to make what it needs public first. It is a new method rather than a new `EncodeOption`, because adding a choice to Go's encoder is a spec non-goal. The fill copies typed values rather than matching strings in JSON, so an ordinary string that happens to read `(sensitive)` is never touched. The **custom-marshaljson** gotcha also applies: `Sensitive`'s JSON form is left exactly as it is.

**Python's own code is the instance-to-record step, nothing more.** A new `record_of(instance, classes)` in `xcl/_schema.py`, beside `hydrate`, is the mirror image of `_hydrate_into` and `Meta.from_entity`. It builds the record as follows:
- **Type and subtype** come from the configuration's own `ClassMap.names_of(type(instance))`. A dataclass class this configuration did not register raises `NotRegisteredError`.
- **The name** comes from the label fields, or from `meta.name` when the class has no label field. A singleton has none.
- **Field keys** are `options.name or f.name`.
- **`meta`** is written from the instance's `Meta`: the known keys plus `extra`. `disabled` and `depends_on` go beside it.
- **Nested blocks** recurse, and their label fields are written as keys.
- **`Sensitive`** is written as its real value, or as the marker when it is redacted.
- **`None`** is omitted, and lists and dicts recurse.

An untyped `xcl.Entity` sends `to_dict()` unchanged. Anything else raises `xcl.NotEncodableError`, mirroring Go's "it is not an entity" (`encode.go:192-198`). That covers `None`, a dict, a non-dataclass object, or a value `json.dumps` cannot write. So does a named entity with no name, which would otherwise surface as a confusing invalid-saved-data error. Variables, outputs, modules and values with no configuration form are refused by the engine with `not_encodable`. Python raises that through the existing error map.

**Proving it matches Go.** The guard is the parity suite. The Go parity program and `tests/parity/run_python.py` each write an `encode` section into the record `test_parity.py` compares. For every encodable entity of every fixture they write:
- the looked-up entity's text;
- its saved state record's text.

Each is written under all 16 combinations of the four choices. On the Go side the looked-up text comes from `EncodeEntity` on the stored entity and the saved text from `EncodeSavedEntity`. On the Python side they come from `encode_entity(find(...))` and `encode_saved_entity(record)`. A new `encode` fixture covers:
- a type with a subtype and one without;
- a sensitive field, a reference, and a sensitive bare reference;
- an unset optional string, a nullable attribute left out, an empty list and an unset map.

It also lists hand-built entities. Python builds them with literal values in `tests/parity/types.py`, and Go writes its stored entity's text for the same addresses. Behaviour the parity record cannot express is covered by Go `coreservice` tests and Python integration tests: highlight, edits, errors and refusal during an operation.

**Why this direction beats the alternatives.** Each alternative breaks something the spec needs:
- Encoding the lookup copy as is (the unmerged `spek/…-python-encoder` branch) loses reveal.
- Sending only an address loses the user's edits.
- Filling in `coreservice` breaks the public-packages rule.
- Matching strings in JSON can't tell a marker from an ordinary string.
- A Python serialiser with its own rules risks drifting from the engine's record shape. Mirroring hydration avoids that.

The evidence for each is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Core-service protocol (xcl, changed).** This is the `CoreService` definition. It gains:
  - an `EncodeOptions` message holding the four encoding choices;
  - an `options` field on the saved-record request;
  - a new `EncodeEntity` method whose request carries an entity record, `highlight` and `options`, and whose response carries the text.

  It is the single API contract the engine handlers and the Python client are generated from. Fields are only appended, never renumbered.
- **Entity-record encoder (xcl root package, new).** This is the public `Config` method that turns an entity's JSON record, as a lookup returns it and possibly edited, into configuration text. It owns:
  - decoding the record through the existing saved-record reader, with display semantics;
  - filling redacted sensitive values from the configuration's own copy of that entity, but only when reveal is asked for.

  It hands the typed value to the existing shared rendering path. It is new because the core service may only reach the engine through public API. It is not a new encode option, so Go's encoder and its choices stay exactly as they are.
- **Redacted-value fill (xcl root package, new, internal to the encoder).** This walks a decoded entity and the stored entity of the same type side by side: struct fields, pointers, list elements by position and map entries by key. Wherever the decoded value is redacted and the stored one is not, it copies the stored sensitive value. When the types differ or there is no stored copy, it leaves everything alone.
- **Shared rendering path (xcl root package, unchanged).** The one `encodeEntity` path that Go's `EncodeEntity`, `EncodeSavedEntity` and the new record encoder all call. This is what makes Python's and Go's text identical. It refuses builtins and non-entities.
- **Core-service lookup handlers (xcl `coreservice`, changed).** The existing saved-record handler and a new entity handler. They share:
  - the lookup refusal while an operation runs;
  - the empty-record check;
  - translating `EncodeOptions` into Go encode options;
  - applying the existing terminal highlighting after rendering.

  The saved-record handler calls `EncodeSavedEntity` and the new one calls the record encoder. Both are registered in the handler table that is held to the protocol's service descriptor.
- **Parity program and fixtures (xcl `bindings/libxcl`, changed).** The Go parity program gains an `encode` stage. For every encodable entity of each fixture, it records the looked-up text and the saved-record text under all 16 combinations of the four choices. For the addresses a fixture lists as hand-built, it records the hand-built text. A new `encode` fixture exercises a type with and without a subtype, sensitive values, references, and empty and nullable attributes.
- **Generated protocol messages (xcl-python, regenerated).** The Python message classes, regenerated from the changed protocol.
- **Instance-to-record builder (xcl-python, new).** The mirror image of hydration, beside it in the schema module. It turns a registered dataclass instance into the JSON record the engine reads:
  - type and subtype come from the configuration's class registrations;
  - the name comes from label fields or `meta`;
  - fields are written under their schema names, with nested blocks, lists and maps recursed;
  - sensitive values are written as the real value or the marker;
  - `meta`, `disabled` and `depends_on` are written back where hydration read them.

  An untyped entity is passed through as its own record. It raises the existing exceptions for non-entities, unregistered classes, unnamed entities and values JSON cannot write.
- **Configuration classes (xcl-python `Config` and `aio.Config`, changed).** Each gains `encode_entity(entity, *, include_computed, include_empty, reveal_sensitive, show_references, highlight)`, and `encode_saved_entity` gains the same four choices. Both build their request through one shared helper for the options and call the client. The sync class calls directly; the asyncio class goes through its existing lookup executor. Their public surfaces stay identical.
- **Python parity runner and types (xcl-python `tests/parity`, changed).** The Python half of the parity record. It writes the same `encode` section from `encode_entity` on looked-up entities, from `encode_saved_entity` on state records, and from `encode_entity` on hand-built instances defined per fixture.
- **Documentation (xcl, xcl-python, xcl-website, and the design document, changed):**
  - xcl's CHANGELOG and godoc;
  - xcl-python's README, PARITY.md and docstrings;
  - the Python tabs of xcl.dev's configuration-text guide and its events page;
  - the Lookups section of the shared-library design document, updated through the design CLI.

## Data Structures & Interfaces

**Core-service messages (`xcl.core.v1`).** One new message and one new method; the saved-record request gains one field. Field numbers are only appended.

```protobuf
// The four choices Go's encoder takes, all off by default.
message EncodeOptions {
  bool include_computed = 1;   // xcl.IncludeComputed()
  bool include_empty    = 2;   // xcl.IncludeEmpty()
  bool reveal_sensitive = 3;   // xcl.RevealSensitive()
  bool show_references  = 4;   // xcl.ShowReferences()
}

message EncodeSavedEntityRequest  { bytes entity = 1; bool highlight = 2; EncodeOptions options = 3; }

rpc EncodeEntity(EncodeEntityRequest) returns (EncodeEntityResponse);   // a lookup
message EncodeEntityRequest  { bytes entity = 1; bool highlight = 2; EncodeOptions options = 3; }
message EncodeEntityResponse { bytes text = 1; }
```

`EncodeEntityRequest.entity` is the entity's JSON record in exactly the shape a lookup returns: `meta` (with `type`, `subtype`, `name`, and `id` when known), `disabled` and `depends_on` beside it, and fields under their schema names. Sensitive values are either real values or the marker. An unset `options` means every choice is off, so a saved-record call that sends none renders exactly as it does today. Failures use the existing codes: `not_encodable`, `unregistered_type`, `invalid_saved_data`, `operation_in_progress`, `invalid_argument` (empty record) and `plugin_load`.

**Go public API (root package `xcl`).** One additive method. The `EncodeOption` type and its four constructors are unchanged.

```go
// EncodeEntityRecord renders an entity's JSON record, as a lookup returns it
// and possibly edited, as configuration text. With RevealSensitive, a
// sensitive value still holding the marker is filled from this
// configuration's own copy of the entity named by meta.id.
func (c *Config) EncodeEntityRecord(data []byte, options ...EncodeOption) ([]byte, error)
```

It returns the same errors as `EncodeSavedEntity`. On failure it returns no text.

**Python API (`xcl.Config`; `xcl.aio.Config` is the awaitable twin).**

```python
def encode_entity(self, entity: object, *, include_computed: bool = False,
                  include_empty: bool = False, reveal_sensitive: bool = False,
                  show_references: bool = False, highlight: bool = False) -> str
def encode_saved_entity(self, data: bytes, *, include_computed: bool = False,
                        include_empty: bool = False, reveal_sensitive: bool = False,
                        show_references: bool = False, highlight: bool = False) -> str
```

`entity` can be one of two things. The first is an instance of a dataclass registered on this configuration, whether looked up, taken from an event or built by hand. The second is an `xcl.Entity`. It raises `NotEncodableError`, `NotRegisteredError`, `UnregisteredTypeError`, `InvalidSavedDataError`, `OperationInProgressError` and `PluginLoadError`, as listed under Errors.

**Python internal contract (`xcl._schema`).**

```python
def record_of(entity: object, classes: ClassMap) -> dict[str, Any]
```

This is the mirror of `hydrate`. It returns the JSON-ready record above for a registered dataclass instance, or `entity.to_dict()` for an `xcl.Entity`. It raises before any engine call for a non-entity, an unregistered class, a named entity with no name, or a value JSON cannot write.

**Parity record (shared by `cmd/parity` and `tests/parity/run_python.py`).** The record gains an `encode` section, compared as decoded JSON like every other section.

```json
"encode": {
  "<entity address>": {
    "looked_up":  { "none": "<text>", "computed": "...", "computed+empty": "...", ..., "computed+empty+reveal+references": "..." },
    "saved":      { ...the same 16 keys... },
    "hand_built": { ...the same 16 keys, only for addresses the fixture lists... }
  }
}
```

Each combination is named by the choices it turns on, joined with `+` in the fixed order `computed`, `empty`, `reveal`, `references`. The combination with no choices is named `none`. Builtins (variables, outputs, modules) are left out of the section. A fixture lists its hand-built addresses in an optional `hand_built.json` (a JSON array of addresses). The Python side builds those instances in `tests/parity/types.py`.

## Implementation Detail

**Engine side: one more entry point in front of an unchanged path.** The record encoder follows the shape of `EncodeSavedEntity` exactly: load the plugins, decode with display semantics, render. The only addition is a fill step between decode and render. Someone reading the root package sees three entry points that converge on the one rendering function, which already carries the comment that sharing it is what makes the texts identical. The fill is the one new pattern in the engine: a small recursive reflection walk over two values of the same type. It recognises sensitive values through the public `SensitiveValue` interface and the `IsRedacted` helper, and copies whole `Sensitive[T]` values with reflection, so the `types` package needs no new API. The walk is conservative:
- it does nothing when the types differ or the stored copy is missing;
- it never creates elements that the decoded value lacks;
- it never overwrites a value that is not redacted.

**Core service: two thin handlers over shared helpers.** The new handler is a near copy of the saved-record handler. The steps both handlers share are factored into small helpers in the lookups file, so the two cannot drift: the lookup refusal, the empty-record check, translating options into Go encode options, and colouring. This follows the existing pattern: one `unary` handler per method, each registered in the handler table, with the descriptor tests failing if the table and the protocol disagree. Protocol changes are made in the `.proto` and regenerated with the existing `buf` target, never hand-edited.

**Python: a mirror function beside the function it mirrors.** `record_of` sits next to `hydrate` in the schema module and walks fields the way `_hydrate_into` and `_hydrate_value` do, so a reader can check the two against each other line by line:
- key names come from the same `options.name or f.name` rule;
- labels go to `meta` the way `_labels_of` reads them;
- `Annotated` and optional wrappers are unwrapped the same way;
- `Sensitive` becomes either the value or the marker.

The name-to-meta step is the mirror of `Meta.from_entity`. It lives with `Meta`, so the known-key list is maintained in one place. Python does no rendering and applies no option semantics: it builds a record, puts the keyword choices into an `EncodeOptions` message, and hands both to the client. Sync and asyncio share the request-building helper, as the asyncio class already reuses the sync helpers for lookups. Their public surfaces stay identical, which the existing surface test enforces.

**Parity: one new stage written twice, compared once.** The Go program and the Python runner each gain an `encode` stage that runs after apply. It iterates the fixture's encodable entities in address order and writes the 16 combinations under the fixed names. The two writers mirror each other, as their existing stages do. A new fixture covers the cases the existing fixtures do not, and an optional per-fixture list of hand-built addresses lets Python construct instances literally. Go needs nothing extra to answer for them: its hand-built text is its stored entity's text. All existing fixtures gain encode coverage for free, which is what the success metric asks for.

**Delivery across repositories.** The xcl change lands first, because the Python package regenerates its protocol from an xcl ref and runs against the engine built from it. xcl-python is developed against the xcl branch, through `xcl-ref.txt` or an `XCL_SRC` checkout, then pinned to the released tag. The website follows the library docs' wording. Breaking the protocol is allowed, since xcl is unreleased, but the plan only appends fields, so nothing breaks anyway.

## Dependencies

- **Design documents this plan was built on:** `libxcl.md` from the `design` design source. It is the settled shared-library design: the core-service message API, entity data crossing as JSON bytes, and the lookup rules, including refusal while an operation runs. This plan implements the new call within that shape and updates the document's Lookups section (through the design CLI) to list `EncodeEntity` and the saved-record call's new options.
- **xcl root encoder (`encode.go`, `EncodeOption` and its four constructors, the shared `encodeEntity`).** Provides all rendering and option semantics. It is reused unchanged; the plan only adds a new method beside it.
- **xcl internal saved-record reader (`internal/savedentity`).** Turns a JSON record into a typed entity through the configuration's catalog, with display semantics. It is reused unchanged by the new method.
- **xcl `types` package (`Sensitive`, `SensitiveValue`, `IsRedacted`).** Provides the redaction state the fill relies on. No change is needed.
- **xcl `highlight` package.** Provides the terminal colouring the core service already applies to saved records. It is reused unchanged for both calls.
- **xcl `coreservice` and `proto/xcl/core/v1/core.proto`, with the `buf` generation setup.** These change. They must land, or at least exist on a branch, before the Python work can regenerate its messages and run against the engine.
- **xcl bindings parity program and fixtures (`bindings/libxcl`).** These change. The Python parity runner depends on the Go side writing the same `encode` section.
- **xcl-python's schema module, class map and error map.** Hydration, registrations and exception classes are reused. The new record builder mirrors hydration, and no new exception class is added.
- **Python `protobuf` runtime and `protoc` (used by xcl-python's protocol generation).** Already in use. No version change.
- **No new third-party dependencies** in any repository.
- **Prior work this builds on, all landed:**
  - hcl-encoding-helpers (Go encoder);
  - references-and-secrets and references-as-written (`Sensitive`, `RevealSensitive`, `ShowReferences`);
  - masking (display reads show the marker);
  - encoder-syntax-highlighting (the `highlight` package);
  - python-support and python-out-of-process-engine (core service, `EncodeSavedEntity` RPC, parity harness);
  - the Python plain-dataclasses work (register_type, `meta` field, labels, nullable). It shipped in xcl-python #13 to #17, and its draft spec `20261014043756-e853a081-python-plain-dataclasses` is empty.
- **Release ordering (flagged).**
  1. The xcl change must merge and be released (the PR-label-driven release) before xcl-python's `xcl-ref.txt` can be pinned to a tag on `main`.
  2. Until then, xcl-python is developed and its CI proven against the xcl branch.
  3. The website changes go out after the xcl-python release they describe.
- **Unmerged earlier attempt `origin/spek/20261015072147-b41720ce-python-encoder`.** Not a dependency. It is reference only and is neither merged nor rebased.

## Testing Approach

Testing happens at four layers. Each layer guards what the one above can't see.

1. **Go unit tests in the root package (most coverage on the fill).** The record encoder and its fill step are the only new engine logic, so they get the most direct tests. They run against real applied configurations, using `registered.Secret` and the existing encode fixtures, with state produced by real applies. They guarantee:
   - a lookup-shaped record with no options renders byte-identically to `EncodeEntity` on the stored entity;
   - with reveal, a field still holding the marker is filled from the stored entity, while an edited field (holding a real value) keeps its edit;
   - a record with no `meta.id`, or an id that matches nothing, renders with the marker and does not fail;
   - the fill reaches into nested blocks, lists of blocks and maps of sensitive values;
   - a record whose stored entity has a different type is left alone;
   - builtins, a record with no meta, and an unregistered type each fail with the existing error and no text.

   Positive and negative cases are separate tests, with no table-driven tests and testify `require` throughout.
2. **Go `coreservice` handler tests.** These go through `Service.Call`, as the existing lookup tests do. They guarantee:
   - `EncodeEntity` renders text;
   - both calls pass each of the four options through;
   - a saved-record call with no options returns exactly the text it does today;
   - highlight adds colour codes, and stripping them gives the plain text, the same for both calls;
   - an empty record is `invalid_argument`;
   - each call is refused with `operation_in_progress` while an operation runs, using the existing `holdCreate` pattern;
   - `not_encodable` and `unregistered_type` are carried as codes.

   The existing descriptor tests guarantee the new method has a handler.
3. **Parity, the cross-language contract.** The Go parity program and the Python runner write the new `encode` section, and the existing comparison asserts the two records are equal as decoded JSON. That makes the text byte-for-byte equal for every encodable entity of every fixture, for both the looked-up entity and its saved record, under all 16 combinations of the four choices, plus the fixture-listed hand-built entities. The new `encode` fixture supplies the cases the acceptance criteria name:
   - a type with a subtype and one without;
   - a provider-filled field (from the plugin fixture);
   - a sensitive field;
   - a reference, a sensitive bare reference and a sensitive template;
   - an unset optional string, a nullable attribute left out, an empty list and an unset map.

   The Go parity program's own tests cover the new stage's shape.
4. **Python unit and integration tests.**
   - Unit tests on `record_of`, with no engine and against the fake transport, guarantee it is the mirror of hydration. `record_of(hydrate(record))` gives back the same record for every annotation form in the existing hydration test classes. They also cover label and name handling, `Sensitive` real versus redacted, and the error raised for each bad input.
   - Unit tests on the configuration classes check that the keyword choices land in `EncodeOptions` and that sync and asyncio expose the same surface.
   - Integration tests against the real engine, in both the sync and asyncio suites, cover:
     - encoding from each kind of lookup (address, type, single of type, all of class, every entity, a kept event entity);
     - a hand-built instance equal to the loaded one;
     - an untyped plugin entity;
     - each option's default-off and opt-in behaviour;
     - reveal on a looked-up instance showing the configured value;
     - an edited non-sensitive and an edited sensitive field;
     - references on a hand-built instance not failing;
     - highlight matching the saved-record call;
     - each error case raising its existing exception;
     - refusal while an operation runs.

Deliberate gaps:
- No Go test drives the Python record builder; the parity suite is the bridge.
- No test reads README, PARITY.md, CHANGELOG, the design document or website pages; the project forbids it.
- The website has no content tests; its build and type check are the automated gate.

**Success metrics.**
- *100% of the parity fixtures produce byte-identical text from Python and Go for every combination of the four encoding choices, for both a looked-up entity and its saved record.* **Behavioural test.** The parity suite's `encode` section covers every fixture, every encodable entity and all 16 combinations, for the `looked_up` and `saved` texts. The existing record comparison fails on the first differing text.

**Manual reviews.**
- **Manual — captured in the implementation test plan.** Review xcl-python's README, PARITY.md (entity encoding listed as supported, no longer under "Not applicable") and the docstrings for the new call, its four choices and highlight.
- **Manual — captured in the implementation test plan.** Review the Python tabs of xcl.dev's configuration-text guide and the events page. They must show `encode_entity`, name the four choices and highlight, mention `encode_saved_entity`'s new choices, and state the event-entity reveal difference from Go.
- **Manual — captured in the implementation test plan.** Review the Lookups section of `design:libxcl.md`: it must list `EncodeEntity` and the saved-record call's new options.
- **Manual — captured in the implementation test plan.** Review xcl's CHANGELOG entry and the godoc of the new method.

## Milestones & Tasks

### Milestone 1: The engine renders any entity record with Go's encoding choices

**What changes**: xcl's core service gains a call that turns an entity's JSON record, as a lookup returns it and possibly edited, into configuration text. The call takes the four encoding choices Go offers (include computed fields, include empty fields, reveal sensitive values, show references as written) and can colour the text for a terminal. The existing saved-record call takes the same four choices. With reveal asked for, a looked-up entity shows its real configured secrets, even though the lookup only ever handed out the marker. Go programs get the same capability as a new `Config` method. Go's encoder, its choices and its defaults are unchanged, and a saved-record call that passes no choices returns exactly the text it does today. The shared-library design document and xcl's changelog describe the new call.

**Validation point**: The Go tests pass in xcl's root package and `coreservice`, and so do the descriptor tests and the protocol's lint and breaking-change checks. The new tests cover: a lookup record rendering identically to `EncodeEntity`, the reveal fill and its edge cases, each option through both calls, highlight, the errors, and refusal during an operation.

#### - [x] Task: Add the encode messages to the core-service protocol
**Id:** 6990ade2-2ebb-4787-a651-6ed2516d82dc
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds an `EncodeOptions` message carrying the four encoding choices. Gives the saved-record request an `options` field. Adds the `EncodeEntity` method with its request and response. The Go message code is regenerated from the protocol so the engine can use the new messages. Fields are only appended, so existing messages keep their numbers.

*Technical detail:* [context.md#task-add-the-encode-messages-to-the-core-service-protocol](./context.md#task-add-the-encode-messages-to-the-core-service-protocol)

**Acceptance criteria**:
- [x] The protocol defines `EncodeOptions` with include computed, include empty, reveal sensitive and show references, all off when unset.
- [x] The saved-record request carries an optional `EncodeOptions`, and a new `EncodeEntity` method takes an entity record, `highlight` and `EncodeOptions` and returns text.
- [x] The regenerated Go messages build, and the protocol passes the project's lint and breaking-change checks.

#### - [x] Task: Render an entity record with secrets filled from the engine
**Id:** e50a42b8-bc85-408d-b11e-6925f463b74f
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds a public `Config` method that renders an entity's JSON record, as a lookup returns it and possibly edited, through the same path as `EncodeEntity` and `EncodeSavedEntity`. When reveal is asked for, every sensitive value that still holds the marker is filled from the configuration's own copy of that entity, found by its address. A value the caller changed keeps the change. Go's existing encoder and its choices are not touched.

*Technical detail:* [context.md#task-render-an-entity-record-with-secrets-filled-from-the-engine](./context.md#task-render-an-entity-record-with-secrets-filled-from-the-engine)

**Acceptance criteria**:
- [x] Rendering a looked-up entity's record gives byte-identical text to `EncodeEntity` on the configuration's own entity, for each of the four choices.
- [x] With reveal, an unedited sensitive field shows the configured value, an edited one shows the edit, and without reveal both show the marker.
- [x] Sensitive values inside nested blocks, lists of blocks and maps are filled the same way.
- [x] A record with no address, or with an address that matches nothing, renders with the marker and does not fail.
- [x] Variables, outputs, modules, records with no meta and records of an unregistered type fail with the existing encode errors and return no text.

#### - [x] Task: Serve entity encoding and saved-record choices from the core service
**Id:** c8ebc79a-ce7e-46b2-92e2-24662930ad68
**Repo:** xcl
**Depends on:**
- 6990ade2-2ebb-4787-a651-6ed2516d82dc — Add the encode messages to the core-service protocol
- e50a42b8-bc85-408d-b11e-6925f463b74f — Render an entity record with secrets filled from the engine
**Execution:** agent

Adds the `EncodeEntity` handler beside the saved-record handler and passes the four choices through both. The two handlers share the lookup refusal while an operation runs, the empty-record check, the options translation and the terminal highlighting, so they behave identically. A saved-record call with no choices keeps returning exactly today's text.

*Technical detail:* [context.md#task-serve-entity-encoding-and-saved-record-choices-from-the-core-service](./context.md#task-serve-entity-encoding-and-saved-record-choices-from-the-core-service)

**Acceptance criteria**:
- [x] `EncodeEntity` returns the record's text, and both calls honour each of the four choices.
- [x] With highlight, both calls return colour codes that strip back to the plain text, and the same record gives the same coloured text from either call.
- [x] Both calls are refused with `operation_in_progress` while an operation runs, and an empty record is an invalid argument.
- [x] Encode failures reach the host as `not_encodable`, `unregistered_type` or `invalid_saved_data`.

#### - [x] Task: Describe the encode call in the xcl changelog and the shared-library design
**Id:** 4c8c1bfd-1489-4b31-83bb-ffee00e3a8bd
**Repo:** xcl
**Depends on:**
- c8ebc79a-ce7e-46b2-92e2-24662930ad68 — Serve entity encoding and saved-record choices from the core service
**Execution:** agent

Adds a changelog entry for the new core-service call, the saved-record call's choices and the new `Config` method. Updates the Lookups section of the shared-library design document, through the design CLI, to list `EncodeEntity` and the saved-record request's options.

*Technical detail:* [context.md#task-describe-the-encode-call-in-the-xcl-changelog-and-the-shared-library-design](./context.md#task-describe-the-encode-call-in-the-xcl-changelog-and-the-shared-library-design)

**Acceptance criteria**:
- [x] The changelog has an entry for this spec describing the new call, the saved-record choices, the reveal fill and the new `Config` method, with its Breaking list.
- [x] The design document's Lookups section lists `EncodeEntity` among the lookups refused during an operation, shows its messages, and shows `EncodeOptions` on the saved-record request.

### Milestone 2: Python programs can encode the entities they hold

**What changes**: `xcl.Config` and `xcl.aio.Config` gain `encode_entity`, which works on any entity the program holds:
- one returned by a lookup or carried by an event;
- an instance of a registered dataclass the program built itself;
- the generic `xcl.Entity` for a plugin type with no class.

`encode_entity` takes `include_computed`, `include_empty`, `reveal_sensitive`, `show_references` and `highlight`, all off by default. `encode_saved_entity` gains the same four choices, and calls that pass none behave as before. Field edits on a looked-up entity show in the text. Bad input raises xcl-python's existing exceptions: a non-entity, an unregistered class, a variable, output or module, or a value with no configuration form. Encoding while an operation runs is refused. xcl-python's README and PARITY.md list entity encoding as supported.

**Validation point**: xcl-python's unit tests and its sync and asyncio integration suites pass against an engine built from the Milestone 1 xcl branch. These cover each lookup kind, hand-built and untyped entities, each choice, edits, highlight, every error case and refusal during an operation.

#### - [x] Task: Regenerate the xcl-python protocol messages
**Id:** 90f1ff60-c805-448b-929f-3dde73d8342a
**Repo:** xcl-python
**Depends on:**
- 6990ade2-2ebb-4787-a651-6ed2516d82dc — Add the encode messages to the core-service protocol
**Execution:** agent

Regenerates the Python message classes and type stubs from the changed xcl protocol. Points the package's engine pin at the xcl branch carrying the change, so CI builds the matching engine while the work is in review.

*Technical detail:* [context.md#task-regenerate-the-xcl-python-protocol-messages](./context.md#task-regenerate-the-xcl-python-protocol-messages)

**Acceptance criteria**:
- [x] The Python message classes include `EncodeOptions`, `EncodeEntityRequest` and `EncodeEntityResponse`, and the saved-record request has `options`.
- [x] The engine pin names the xcl branch carrying the change.

#### - [x] Task: Turn a Python entity into the record the engine reads
**Id:** 23576734-7a60-4f8f-86ac-6cda987e4a36
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Adds the mirror image of hydration. It turns a registered dataclass instance, whether looked up, from an event or built by hand, into the JSON record the engine reads. Type and subtype come from the configuration's registrations, the name from label fields or `meta`, and sensitive values become their real value or the marker. An untyped `xcl.Entity` passes through as its own record. Bad input raises the existing exceptions before anything reaches the engine.

*Technical detail:* [context.md#task-turn-a-python-entity-into-the-record-the-engine-reads](./context.md#task-turn-a-python-entity-into-the-record-the-engine-reads)

**Acceptance criteria**:
- [x] Building a record from a hydrated instance gives back the record it was hydrated from, for every field form hydration supports: labels, nested blocks, lists and maps of blocks, sensitive, computed, nullable and renamed fields, `meta`, `disabled` and `depends_on`.
- [x] A hand-built instance's record names its registered type and subtype and its name, and carries real sensitive values.
- [x] A non-entity raises `NotEncodableError`, and so does an unnamed named entity or a value JSON cannot write. An instance of a class the configuration did not register raises `NotRegisteredError`.

#### - [x] Task: Add encode_entity and the saved-record choices to both configuration classes
**Id:** b6d32cc3-7aa8-47eb-895e-04a87aeda684
**Repo:** xcl-python
**Depends on:**
- 90f1ff60-c805-448b-929f-3dde73d8342a — Regenerate the xcl-python protocol messages
- 23576734-7a60-4f8f-86ac-6cda987e4a36 — Turn a Python entity into the record the engine reads
- c8ebc79a-ce7e-46b2-92e2-24662930ad68 — Serve entity encoding and saved-record choices from the core service
**Execution:** agent

Adds `encode_entity` to `xcl.Config` and `xcl.aio.Config`, with `include_computed`, `include_empty`, `reveal_sensitive`, `show_references` and `highlight`, all off by default. Gives `encode_saved_entity` the same four choices. Both build the record and the options and leave the rendering to the engine. The tests run against the real engine, in both the sync and asyncio suites.

*Technical detail:* [context.md#task-add-encode_entity-and-the-saved-record-choices-to-both-configuration-classes](./context.md#task-add-encode_entity-and-the-saved-record-choices-to-both-configuration-classes)

**Acceptance criteria**:
- [x] An entity from each kind of lookup encodes to one block headed `<type> "<subtype>" "<name>"` or `<type> "<name>"`, from both the sync and asyncio APIs. The lookups are by address, by type, single of type, all of class, every entity, and an event's entity.
- [x] A hand-built instance encodes to the same text as the same entity loaded from a file, and a generic plugin entity encodes to a block of its type.
- [x] Each choice is off by default and takes effect when asked for. Reveal shows a looked-up entity's configured secret, and edits show in the text.
- [x] With highlight the text is coloured, strips back to the plain text, and matches the saved-record call's coloured text.
- [x] Each error case raises its existing exception and returns no text, and encoding during an operation raises `OperationInProgressError`.
- [x] `encode_saved_entity` with each choice matches `encode_entity` on the live entity, and with no choices returns exactly today's text.

#### - [x] Task: Document entity encoding in the xcl-python README and parity record
**Id:** b2c4fb8e-b6c6-4f5b-8eca-53e0befabffe
**Repo:** xcl-python
**Depends on:**
- b6d32cc3-7aa8-47eb-895e-04a87aeda684 — Add encode_entity and the saved-record choices to both configuration classes
**Execution:** agent

Moves entity encoding from "Not applicable" to the supported lookups in PARITY.md, mapping Go's options to the Python keywords. Adds an encoding section to the README covering the new call, the four choices, highlight, reveal on looked-up and event entities, and the saved-record call's new choices.

*Technical detail:* [context.md#task-document-entity-encoding-in-the-xcl-python-readme-and-parity-record](./context.md#task-document-entity-encoding-in-the-xcl-python-readme-and-parity-record)

**Acceptance criteria**:
- [x] PARITY.md lists `EncodeEntity` and the saved-record call with their Python forms, sync and asyncio, and no longer lists `EncodeEntity` under "Not applicable".
- [x] The README shows `encode_entity`, names the four choices and highlight, and mentions `encode_saved_entity`'s new choices.

### Milestone 3: Python's text is proven identical to Go's

**What changes**: The cross-language parity suite now compares encoded text as well as the other records. Every encodable entity of every fixture is covered, under all 16 combinations of the four choices, both for the looked-up entity and for its saved record. A new fixture adds the cases the earlier fixtures lack, and lists entities that Python rebuilds by hand to compare against Go's loaded copy. This makes the "same text as Go" promise a release gate rather than a hope. Its user-visible effect is confidence: a divergence fails CI before a release.

**Validation point**: The parity suite passes, with the Go program from the xcl branch and the Python runner from the xcl-python branch. The `encode` section is present and equal for every fixture.

#### - [x] Task: Record encoded text in the Go parity program
**Id:** 051b8d4d-334b-4a4e-a875-926f379bacfe
**Repo:** xcl
**Depends on:**
- c8ebc79a-ce7e-46b2-92e2-24662930ad68 — Serve entity encoding and saved-record choices from the core service
**Execution:** agent

Adds an `encode` stage to the bindings' parity program. For every encodable entity of a fixture, it records Go's text for the entity and for its saved state record under all 16 combinations of the four choices, plus a hand-built section for the addresses the fixture lists. Adds a new `encode` fixture with the cases the acceptance criteria name, and documents the stage and the hand-built list in the fixtures' README.

*Technical detail:* [context.md#task-record-encoded-text-in-the-go-parity-program](./context.md#task-record-encoded-text-in-the-go-parity-program)

**Acceptance criteria**:
- [x] Every fixture's parity record has an `encode` section with `looked_up` and `saved` texts under the 16 named combinations for each encodable entity, and leaves out variables, outputs and modules.
- [x] A fixture that lists hand-built addresses gets a `hand_built` section for them.
- [x] The new fixture covers a type with and without a subtype, a sensitive field, a reference, a sensitive bare reference and template, an unset optional string, a nullable attribute left out, an empty list and an unset map.

#### - [x] Task: Record encoded text in the Python parity runner
**Id:** 364da9f7-0c57-4d8d-9fe4-990279f6a157
**Repo:** xcl-python
**Depends on:**
- b6d32cc3-7aa8-47eb-895e-04a87aeda684 — Add encode_entity and the saved-record choices to both configuration classes
- 051b8d4d-334b-4a4e-a875-926f379bacfe — Record encoded text in the Go parity program
**Execution:** agent

Adds the same `encode` stage to the Python parity runner. Looked-up texts come from `encode_entity` on what `find` returns, saved texts from `encode_saved_entity` on the state records, and hand-built texts from instances the parity types build with literal values. Adds the classes and hand-built instances for the new fixture, so the existing comparison fails on any differing text.

*Technical detail:* [context.md#task-record-encoded-text-in-the-python-parity-runner](./context.md#task-record-encoded-text-in-the-python-parity-runner)

**Acceptance criteria**:
- [x] The parity suite passes for every fixture with the `encode` section present on both sides, so the Python text equals Go's byte for byte for every entity and combination.
- [x] The new fixture's types match their `types.json`, and its hand-built instances encode to Go's text for the same addresses.

#### - [ ] Task: Release xcl with the encode call
**Id:** bfa93d2c-8301-46d9-a68e-6f5bf7314f25
**Repo:** xcl
**Depends on:**
- 4c8c1bfd-1489-4b31-83bb-ffee00e3a8bd — Describe the encode call in the xcl changelog and the shared-library design
- 051b8d4d-334b-4a4e-a875-926f379bacfe — Record encoded text in the Go parity program
**Execution:** human — merging the xcl PR with its release label publishes a release, an action outside the repo

Merges the xcl pull request with its release-size label once the xcl-python pull request, pinned to the xcl branch, has proven the pair. The release produces the tag xcl-python must pin to.

*Technical detail:* [context.md#task-release-xcl-with-the-encode-call](./context.md#task-release-xcl-with-the-encode-call)

**Acceptance criteria**:
- [ ] An xcl release tag exists that carries the `EncodeEntity` call and the saved-record choices.

#### - [ ] Task: Pin xcl-python to the released engine
**Id:** 8c06dbe7-b8c4-48f5-90e7-99ed20303661
**Repo:** xcl-python
**Depends on:**
- bfa93d2c-8301-46d9-a68e-6f5bf7314f25 — Release xcl with the encode call
- 364da9f7-0c57-4d8d-9fe4-990279f6a157 — Record encoded text in the Python parity runner
- b2c4fb8e-b6c6-4f5b-8eca-53e0befabffe — Document entity encoding in the xcl-python README and parity record
**Execution:** agent

Points xcl-python's engine pin at the released xcl tag in place of the development branch, and regenerates the messages from that tag, so `main` builds against a released engine as the release rules require.

*Technical detail:* [context.md#task-pin-xcl-python-to-the-released-engine](./context.md#task-pin-xcl-python-to-the-released-engine)

**Acceptance criteria**:
- [ ] The engine pin names the released xcl tag, and the generated messages match that tag.

### Milestone 4: xcl.dev documents encoding from Python

**What changes**: xcl.dev's configuration-text guide stops saying that converting a live entity, provider-filled values and references are "not yet available from Python". Its Python tabs show `encode_entity` beside Go's `EncodeEntity`, name the four choices and highlight, and show `encode_saved_entity` taking the same choices. The page and the events page explain that an event entity kept and encoded after its operation reveals the engine's current value. The editor-theme and custom-renderer highlight sections keep their existing Python notes.

**Validation point**: The site builds and passes its type check, and a person has reviewed the changed Python tabs.

#### - [x] Task: Show encode_entity in the website configuration-text guide
**Id:** 51e9f153-923a-40ed-8b0c-3b2a3e9a6a1b
**Repo:** xcl-website
**Depends on:**
- b2c4fb8e-b6c6-4f5b-8eca-53e0befabffe — Document entity encoding in the xcl-python README and parity record
**Execution:** agent

Replaces the "not yet available from Python" notes in the configuration-text guide's Python tabs with `encode_entity` examples for converting an entity, provider-filled values and references as written. Shows the saved-record call taking the same choices, and describes including empty fields and revealing secrets for Python. Updates the events page's mentions to match. The wording follows the xcl-python README.

*Technical detail:* [context.md#task-show-encode_entity-in-the-website-configuration-text-guide](./context.md#task-show-encode_entity-in-the-website-configuration-text-guide)

**Acceptance criteria**:
- [x] Each Python tab on the guide that said encoding a live entity, computed fields or references was unavailable now shows the working `encode_entity` call.
- [x] The guide names the four choices and highlight for Python, shows `encode_saved_entity` with the new choices, and says that a kept event entity reveals the engine's current value.
- [x] The events page no longer says Python must go through the saved-record call to turn an entity into text.
- [x] The site builds and passes its type check.

## Open Questions

- **Do a decoded plugin-type record and the stored entity have the same Go type?** Declared types come from catalog prototypes, so both sides share one type. A plugin's type is rebuilt from its schema with `reflect.StructOf`, which Go caches for identical field lists. That should give the same type, but only exercising it proves it. *Depends on:* how the catalog rebuilds plugin types on decode compared with apply. *When hit:* the reveal fill skips any entity whose types differ, so a plugin entity's parity `reveal` texts would show the marker where Go shows the value. If the parity or `coreservice` tests show that, STOP and ask the user before changing how the fill matches values (for example, matching by field name instead of by type).

No other open questions. Every other uncertainty was resolved during planning and is recorded in the assumption log.

## Out of Scope

- **Compatibility of the core-service messages with older clients.** xcl is unreleased, so no effort goes into keeping older clients working against the new engine. The plan only appends fields anyway.
- **Entity encoding through any binding other than Python.** There is no Node or other SDK work.
- **Any change to Go's encoder.** What it writes, its choices and its defaults stay as they are. Python matches Go as it is. The only Go addition is the record-encoding `Config` method the core service needs.
- **Encoding several entities in one call.** Each entity is encoded separately, as in Go.
- **Reading encoded text back into a Python instance.** Turning xcl text into entities stays with the existing load.
- **Editor-theme and custom-renderer highlighting from Python.** Python offers only the terminal colouring the saved-record call already uses. The website's existing Python notes for those two sections stay.
- **New Python guide or Python API reference pages on xcl.dev.** By the user's decision, the Python tabs of the configuration-text guide and the events page are the Python documentation updated here.
- **Matching Go's marker-only reveal for masked event data.** By the user's decision, an event entity kept and encoded after its operation reveals the engine's current value. The difference is documented, not engineered away.
- **The unmerged branch `origin/spek/20261015072147-b41720ce-python-encoder`.** It is neither merged, rebased nor deleted by this plan.
- **Bumping the version badge on the xcl.dev home page.** It follows the release that ships this feature, as other site releases do, and is not part of this plan.

## Changelog

### 2026-10-16 — Task: Add the encode messages to the core-service protocol

**What was done**: The core-service protocol gained an `EncodeOptions` message (include computed, include empty, reveal sensitive, show references), an `options = 3` field on `EncodeSavedEntityRequest`, and a new `EncodeEntity` lookup method with `EncodeEntityRequest {entity, highlight, options}` and `EncodeEntityResponse {text}`. The Go messages were regenerated with `make core-protos`.

**Deviations**: None.

**Files changed**:
- `xcl: proto/xcl/core/v1/core.proto`
- `xcl: coreservice/proto/core.pb.go`

**Discoveries**: Adding a method to the service makes `coreservice`'s `TestEveryServiceMethodHasAHandler` fail until a handler is registered in `coreservice/service.go`; it stays red until the task "Serve entity encoding and saved-record choices from the core service".

### 2026-10-16 — Task: Render an entity record with secrets filled from the engine

**What was done**: Added the public `(*Config).EncodeEntityRecord(data, options...)`, which decodes a lookup-shaped entity record with display semantics and, only with `RevealSensitive`, fills every redacted `types.Sensitive[T]` from the Config's own entity named by `meta.id` before calling the shared `encodeEntity`. The fill is a reflective side-by-side walk over struct fields, pointers, interfaces, slices by index and maps by key; it skips xcl's bookkeeping (`types.ResourceBase`, `types.Meta`), never adds elements and never overwrites a value that is not redacted.

**Deviations**: Tests go beyond the plan's list: a reveal-equals-`EncodeEntity` test, list-of-blocks fill tests (with a test-local `encodeRecordVault` type, since no shared fixture has a sensitive value in a list of blocks), and a plugin-type fill test.

**Files changed**:
- `xcl: encode_record.go`
- `xcl: encode_record_test.go`

**Discoveries**: The plan's open question is resolved: a decoded record and the stored entity have the same Go type for declared types and for plugin types (`Credential` from the test plugin), so the fill works for both. `registered.SecretShape.Login` is an object attribute (pointer struct), not a block. A test module source is resolved relative to the config file, so module tests write a small module beside `main.xcl` in a temp dir.

### 2026-10-16 — Task: Serve entity encoding and saved-record choices from the core service

**What was done**: Added the `EncodeEntity` handler, which renders a lookup-shaped record through `Config.EncodeEntityRecord`, and made `EncodeSavedEntity` pass its `EncodeOptions` through. The two handlers share `encodeAllowed` (lookup refusal during an operation and the empty-record check), `encodeOptions` (protocol choices into Go encode options) and `colour` (terminal highlighting after rendering). `EncodeEntity` is registered in the handler table, which brings the descriptor tests back to green.

**Deviations**: Tests go beyond the plan's list: `EncodeEntity` tests for each choice on and off, the three encode failures for the saved-record call as well, a highlight strip test for the saved-record call, and `TestEncodeSavedEntityDuringOperationRefused`, which was not covered before.

**Files changed**:
- `xcl: coreservice/lookups.go`
- `xcl: coreservice/service.go`
- `xcl: coreservice/lookups_test.go`
- `xcl: coreservice/operations_test.go`

**Discoveries**: `EncodeSavedEntity` fed a lookup record with reveal still shows the marker, because only `EncodeEntity` fills from the Config; a real saved record holds the real value. So the reveal comparisons between the two calls use a saved record on one side and a `Find` record on the other. This matters for the Python parity and integration tests.

### 2026-10-16 — Task: Describe the encode call in the xcl changelog and the shared-library design

**What was done**: Added the spec's entry to xcl's CHANGELOG covering `EncodeEntity`, `EncodeOptions`, the saved-record call's choices, `Config.EncodeEntityRecord`, its reveal fill by `meta.id` and the event-entity difference from Go, with `**Breaking:** None.` Updated `design:libxcl.md` through the design CLI. Its Threading list now includes `EncodeEntity` among the lookups refused during an operation, its service block lists the new method, and its Lookups section shows `EncodeOptions`, `options = 3` on the saved-record request, the new request and response, and a paragraph on how encoding works.

**Deviations**: The design was written with `spektacular design author`, not `design write`, because the CLI refuses a verbatim write over a Spektacular-authored design. A first attempt passed `--spec`, which stamped `spec: 20261016064106-8931613a-python-encode-entity` into the design's frontmatter. Re-authoring without `--spec` does not remove it, so the field stays and needs a person's decision.

**Files changed**:
- `xcl: CHANGELOG.md`
- `xcl: .spektacular/design/libxcl.md` (through the design CLI, in the project checkout)

**Discoveries**: `design author` takes the body without frontmatter and keeps `created_date` and `specs`. Its `--spec` flag records the design's originating spec and is sticky, so don't pass it when revising someone else's design.

### 2026-10-16 — Task: Regenerate the xcl-python protocol messages

**What was done**: Regenerated xcl-python's `core_pb2.py` and `core_pb2.pyi` from the xcl feature branch's protocol with `make protos XCL_SRC=../xcl` (protoc 36.1, the same version as the checked-in code). This adds `EncodeOptions`, `EncodeEntityRequest`, `EncodeEntityResponse` and `EncodeSavedEntityRequest.options`; `transport_pb2` is unchanged. `xcl-ref.txt` now names the xcl branch `spek/20261016064106-8931613a-python-encode-entity` for review.

**Deviations**: None.

**Files changed**:
- `xcl-python: xcl/_core/core_pb2.py`
- `xcl-python: xcl/_core/core_pb2.pyi`
- `xcl-python: xcl-ref.txt`

**Discoveries**: In the worktree, `XCL_SRC=../xcl` resolves to the sibling xcl worktree, so `make protos engine plugins XCL_SRC=../xcl PYTHON=.venv/bin/python` builds everything from the feature branch. A local `.venv` (git-ignored) was created with `uv venv` and `uv pip install -e '.[test]'`. CI can only check out the `xcl-ref.txt` branch once it is pushed.

### 2026-10-16 — Task: Turn a Python entity into the record the engine reads

**What was done**: Added `Meta.to_record()`, the mirror of `Meta.from_entity`. It writes the set known keys plus `extra`, and the two now share the `_record_keys()` list. Added `record_of(entity, classes)` and `record_bytes(entity, classes)` in `xcl/_schema.py`, the mirror of hydration:
- type and subtype come from the configuration's registration;
- the name comes from the label field (the second of two) or from `meta.name`;
- `disabled` and `depends_on` are written beside `meta`;
- fields go under their schema names, with nested label fields as keys;
- a `Sensitive` is written as its value or the marker, and `None` is omitted.

An `xcl.Entity` passes through as `to_dict()`. A non-entity, a named entity with no name, or a value JSON can't write raises `NotEncodableError`, worded like the engine's. An unregistered class raises `NotRegisteredError`.

**Deviations**: `_record_value` dispatches on the value hydration produced (`Sensitive`, dataclass instance, list, dict) rather than re-reading annotations; the result is the same and simpler. `record_bytes` serialises the record, and the callers share it, as the plan preferred.

**Files changed**:
- `xcl-python: xcl/_entity.py`
- `xcl-python: xcl/_schema.py`
- `xcl-python: tests/unit/test_record.py`
- `xcl-python: tests/unit/test_entity.py`

**Discoveries**: A registered dataclass with no `meta` field does not keep `meta.id` or `meta.references` through hydration. Its record therefore has no id to fill reveal from and no references to show, so for such a class reveal keeps the marker and show-references writes the values. Classes meant to be encoded with those choices need a `meta: xcl.Meta | None` field, and the README should say so. An unlabelled list element hydrates with its label field `""` and is written back with `"name": ""`.

### 2026-10-16 — Task: Add encode_entity and the saved-record choices to both configuration classes

**What was done**: `xcl.Config` and `xcl.aio.Config` gained `encode_entity(entity, *, include_computed, include_empty, reveal_sensitive, show_references, highlight)`, and `encode_saved_entity` gained the same four keywords, all off by default. Both build `EncodeOptions` through the shared `_encode_options` helper. `encode_entity` builds its record with `record_bytes` before any engine call. Unit tests check that each keyword lands in `EncodeOptions` and that bad input makes no engine call. Real-engine sync and asyncio tests cover every lookup kind, kept event entities, hand-built and untyped entities, each choice, edits, highlight, every error and refusal during an operation.

**Deviations**: Verification found that a looked-up entity's text differed from Go's in three cases: hydrated `[]` and `{}` defaults were sent back and written, an unset sensitive field showed the marker, and Python-only dataclass defaults (such as a nested `interval = 10`) were written. With the user's approval, two fixes were made:
- **A (engine):** `Config.EncodeEntityRecord` now fills marker values from its own copy whenever `meta.id` names an entity, not only with `RevealSensitive`. The encoder still writes the marker unless reveal is asked for, and an unset value is left out as Go leaves it out.
- **B (Python):** `hydrate` remembers the JSON each top-level instance came from, keyed by `id()` with a `weakref.finalize` cleanup, so a slotted class without `__weakref__` keeps none. `record_of` writes every field still equal to what its source hydrates to exactly as the source holds it, and leaves out a field the source lacked whose value is still the hydration default. It recurses into nested blocks, lists and maps, and uses the source's `meta`, `disabled` and `depends_on` for a class with no `meta` field. Edited fields are written from the instance.

The xcl CHANGELOG entry, `EncodeEntityRecord`'s godoc and `design:libxcl.md` were updated to match.

**Amendments**: None to the spec. The plan's decision to fill only with `RevealSensitive` was changed by the user's approval of fix A.

**Files changed**:
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/aio.py`
- `xcl-python: xcl/_schema.py`
- `xcl-python: tests/unit/test_config.py`
- `xcl-python: tests/unit/test_record.py`
- `xcl-python: tests/integration/sync_entities.py`
- `xcl-python: tests/integration/test_sync.py`
- `xcl-python: tests/integration/test_aio.py`
- `xcl: encode_record.go`
- `xcl: encode_record_test.go`
- `xcl: CHANGELOG.md`

**Discoveries**: The engine's lookup JSON writes the marker for every `Sensitive` field, set or not, so only the engine's copy can tell the two apart. Top-level schema defaults (such as `replicas = 2`) are held by the engine as well, so Go writes them too; nested dataclass defaults are Python-only. A copy of a looked-up instance (`dataclasses.replace`) has no source and encodes from its values, defaults included. Encoding inside `on_event` is refused only while the operation still runs, so the refusal tests need the slow plugin.

### 2026-10-16 — Task: Document entity encoding in the xcl-python README and parity record

**What was done**: PARITY.md lists `EncodeEntity` as `encode_entity(entity, ...)` and `EncodeSavedEntity` with the four keywords, sync and asyncio, and maps each Go option to its keyword. It notes that highlight is Go's ANSI rendering and that a kept event entity reveals the engine's current value, and it no longer lists `EncodeEntity` under "Not applicable". The README gains an "Encoding entities as configuration" section covering:
- the call on looked-up, event, hand-built and untyped entities, with an example;
- the four keywords and `highlight`;
- reveal on looked-up and event entities;
- `encode_saved_entity`'s new keywords;
- that only changed fields differ from Go, and that a copy encodes from its values;
- the errors and the asyncio form.

The looked-up `Sensitive` note now points at `encode_entity(..., reveal_sensitive=True)`, and the lookups list names `encode_entity`.

**Deviations**: None.

**Files changed**:
- `xcl-python: PARITY.md`
- `xcl-python: README.md`

**Discoveries**: None.

### 2026-10-16 — Task: Record encoded text in the Go parity program

**What was done**: The parity program gains an `encode` stage after `state`. For every entity that is not a variable, output, module or root, keyed by address, it records `looked_up` (`xcl.EncodeEntity` on the stored entity) and `saved` (`Config.EncodeSavedEntity` on its state record) under all 16 combinations, named `none` or the turned-on choices joined with `+` in the order computed, empty, reveal, references. Addresses listed in an optional `hand_built.json` also get `hand_built`, Go's own text for that entity. Failures are recorded as outcomes. A new `encode` fixture covers:
- a `store "postgres"` with a sensitive password, an optional string with a default, a nullable int, a configured empty list and an unset nullable map;
- a `service` without subtype with a reference, a sensitive bare reference and a sensitive template.

Its `types.json` is the schema xcl-python derives from the classes the next task adds. The fixtures README documents `hand_built.json` and the section.

**Deviations**: The fixture's unset map is declared `nullable`, because a hand-built Python instance can only spell "unset" for a map as `None`; a `default_factory=dict` map would send `{}`. The configured empty list stays `tags = []`. Go writes it even with no choices, so "an empty list" in the spec's empty-fields criterion only holds for an unset list. Every fixture's record now has an `encode` section, so the Python parity comparison fails until the Python runner writes it in the next task.

**Files changed**:
- `xcl: bindings/libxcl/cmd/parity/main.go`
- `xcl: bindings/libxcl/cmd/parity/main_test.go`
- `xcl: bindings/libxcl/testdata/parity/encode/types.json`
- `xcl: bindings/libxcl/testdata/parity/encode/main.xcl`
- `xcl: bindings/libxcl/testdata/parity/encode/hand_built.json`
- `xcl: bindings/libxcl/testdata/parity/README.md`

**Discoveries**: Go's own saved-record text differs from its live text for a configured empty list, because state omits empty lists (`omitempty`). Parity compares Python with Go section by section, so this is not a parity failure, but it qualifies the spec's "saved records give the same text as the live entity" for that case.

### 2026-10-16 — Task: Record encoded text in the Python parity runner

**What was done**: `tests/parity/run_python.py` writes the same `encode` section as the Go program:
- `looked_up` from `encode_entity(find(None, address))`;
- `saved` from `encode_saved_entity` on the state records;
- `hand_built` from the instances `types.HAND_BUILT` builds for the addresses in `hand_built.json`.

It uses the same 16 combination names, and failures are recorded through `Normaliser.outcome`. `tests/parity/types.py` gains `EncodeStore` and `EncodeService` for the `encode` fixture, plus `HAND_BUILT`. `test_types.py` checks the fixture's schema, and `test_parity.py` gains `test_parity_encode`. The parity suite passes for every fixture: Python's text equals Go's byte for byte under all 16 combinations, for looked-up, saved and hand-built entities, which meets the success metric.

**Deviations**: The first run found one more divergence: a configured `tags = []` disappears from a lookup's JSON because Go leaves empty lists out, so Python could not send it back. In the spirit of the approved fix A, `EncodeEntityRecord`'s fill now also takes an empty list or map from the engine's copy where the record has none. It still never replaces a list or map the record holds. Go tests cover it, and the CHANGELOG, godoc and `design:libxcl.md` mention it.

**Files changed**:
- `xcl-python: tests/parity/run_python.py`
- `xcl-python: tests/parity/types.py`
- `xcl-python: tests/parity/test_types.py`
- `xcl-python: tests/parity/test_parity.py`
- `xcl: encode_record.go`
- `xcl: encode_record_test.go`
- `xcl: CHANGELOG.md`

**Discoveries**: The parity program in `build/plugins` must be rebuilt (`make plugins XCL_SRC=../xcl`) after any Go parity change, or the Go side shows `<missing>` sections. `bindings/libxcl` is a nested Go module, so its tests run from inside it.

### 2026-10-16 — Task: Show encode_entity in the website configuration-text guide

**What was done**: The Python tabs of xcl.dev's configuration-text guide now show the working calls in place of every "not yet available from Python" note for encoding:
- the intro names `cfg.encode_entity` and `cfg.encode_saved_entity`, and says the engine renders identical text;
- converting an entity shows `cfg.encode_entity(db)` (and its asyncio form), on looked-up, hand-built and untyped entities, with edits shown;
- saved data shows `encode_saved_entity` taking the same keywords;
- an event's entity shows `cfg.encode_entity(entity)`, and that a kept event entity reveals the engine's current value;
- each choice has its Python call: `include_computed=True`, a new "Empty attributes" section with `xcl.IncludeEmpty()` and `include_empty=True`, a new Go/Python tab for `RevealSensitive` with `reveal_sensitive=True`, `show_references=True`, and `highlight=True` on `encode_entity`.

The editor-theme and renderer notes keep saying Python offers `highlight=True` only, now naming both calls. The events page mentions `cfg.encode_entity(event.entity())` and its reveal behaviour.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/pages/events.mdx`

**Discoveries**: The site's only automated checks are `npm run build` and `npx astro check`. Both pass: 0 errors, 0 warnings, and 1 hint that was already there. The worktree needed `npm ci` because `node_modules` is not tracked.
