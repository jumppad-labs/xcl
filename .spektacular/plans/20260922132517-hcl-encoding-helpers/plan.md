---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-23"
---

# Plan: 20260922132517-hcl-encoding-helpers

<!-- Metadata -->
<!-- Created: 2026-09-22T16:41:10Z -->
<!-- Commit: ada7cdf -->
<!-- Branch: event-based-logging -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

This plan lets developers embedding xcl turn a single entity back into configuration text in xcl's own syntax, ready to print or write to a `.xcl` file. They can start from an entity they hold after an apply, or from that entity's saved data exactly as xcl stores it in state. Two public functions and an opt-in computed-values option sit on top of xcl's internal tag-driven encoder, repaired so it writes what the decoder reads. Saved data is typed through the plugin registry. Two decisions made during planning change the spec's behaviour. Provider-filled (computed) fields are left out by default, so the text always reads back, and an option adds them for display. Lifecycle events stop carrying resource data unless the application opts in to a raw or processed level. The example programs opt in and show each created entity's configuration beneath its create-success line. The library README and xcl.dev document how to convert an entity and its saved data.

## Conventions

- **Shared error types live in the `errors` package: sentinel plus pointer detail with `Unwrap`, re-exported from `xcl`** — `ErrUnregisteredType`, `ErrInvalidSavedData` and `ErrNotEncodable` are raised in the root package and by the shared saved-data decode that `state` also calls, so they need the neutral home.
- **NEVER modify dependency packages; the HCL fork in `internal/xcl` is ours but MPL-2.0** — the encoder fixes go in `internal/xcl/gohcl/encode.go` with the HashiCorp header, "Modifications Copyright (c) Jumppad Labs", and a `UPSTREAM.md` entry. The public wrapper only calls in and lives in Apache-licensed root files.
- **Generate test state with a real apply, not a hand-written state file** — the saved-data tests and the "identical to the resource" agreement test read the state file written by a real `Apply`.
- **Use testify `require`; NEVER table-driven tests; NEVER mix positive and negative tests; favour verbose tests** — every acceptance criterion becomes its own named test, with the error paths (unregistered type, invalid data, builtin) in separate functions.
- **Go code style: gofmt/vet, `any` over `interface{}`, small focused interfaces, explicit error handling** — the new options and functions take `any` and return errors rather than panicking (the encoder's panics are converted).
- **Prefer the standard library; pin dependency versions** — no new dependency. Encoding reuses the in-repo hclwrite/gohcl, and the example continues to use the charm log already pinned.
- **Use dependency injection for testability** — the registry is passed explicitly to `EncodeSavedEntity` and `prettylog.Handler`, not reached through a global or a Config getter.
- Dropped as not applicable: database/prepared statements, context.Context threading (conversion is synchronous and does no I/O), graceful shutdown, structured logging (no new log output; the example writes text through the existing prettylog writer), and the `/cmd`/`/api`/`/pkg` layout (public API follows the existing root-package layout).

## Architecture & Design Decisions

**Shape.** The work lands in two repos: the library (`xclconfig`, root `/home/nicj/code/github.com/jumppad-labs/xcl`) and the docs site (`xcl-website`, root `/home/nicj/code/github.com/jumppad-labs/xcl-website`). The library gains a small public conversion surface in the root `xcl` package, built on xcl's own concepts rather than on HCL-writing primitives, as the spec's Technical Approach steers. It uses the glossary's word for a declared thing, **entity**:

- `xcl.EncodeEntity(entity any, options ...EncodeOption) ([]byte, error)` converts one entity the caller holds.
- `xcl.EncodeSavedEntity(registry *registry.PluginRegistry, data []byte, options ...EncodeOption) ([]byte, error)` converts one entity's saved data, in the form state stores it.
- `xcl.IncludeComputed()` is an opt-in option.

Both return a formatted, single-block `[]byte` ready to print or write to a `.xcl` file. The block header comes from the entity's `Meta`: `resource "<subtype>" "<name>"` when it has a subtype, otherwise `<type> "<name>"` for bare types. The body comes from the same `xcl` field tags the parser decodes with. Builtins (`variable`, `output`, `module`) are refused with an error, since the spec leaves them out. Underneath, the vendored tag-driven encoder in `internal/xcl/gohcl/encode.go` is repaired and extended in place. It stays internal, as the spec's constraint requires, and every change carries the MPL header and a `UPSTREAM.md` record, per the never-modify-dependencies convention. The changes:

- recurse into remain-embedded bases such as `types.ResourceBase` and plugin `*Base` structs;
- encode interface-held values by their dynamic type, because schema-built plugin types hold named scalars as `interface{}` (`internal/schema/deserialize.go:301-316`);
- skip nil values;
- skip computed fields unless asked;
- return errors instead of panicking.

The root wrapper then removes `meta` and drops `disabled = false` and empty `depends_on`, which are ResourceBase bookkeeping rather than configuration.

**Saved data and the registry.** Only the plugin registry can type saved data, including plugin types it builds from a schema, so `EncodeSavedEntity` resolves the type through the registry and never guesses from the data. It reads `meta.type`/`meta.subtype`/`meta.name`, calls `CreateResource`, unmarshals into the typed value, then encodes it exactly as `EncodeEntity` does. That is what makes the two entry points agree byte for byte. The per-record decode already exists inline in `FileStateStore.Load` (`state/file_state_store.go:67-138`). It is extracted into one internal function that both `state` and the root package call, so the stored format has one reader and stays an unchanged contract. `EncodeSavedEntity` calls `registry.Load(nil)` first, because plugin types are resolvable only after loading, and loading is once-only and cached. Failures follow the shared-errors-package convention:

- new sentinels in `errors/`, re-exported from `xcl`, each wrapped by a pointer detail type:
  - `ErrUnregisteredType` (detail names the type);
  - `ErrInvalidSavedData` (detail wraps the decode cause);
  - `ErrNotEncodable` (a builtin, a non-entity, or a value the encoder cannot represent);
- a failure returns no text.

**Two decisions that change the spec's behaviour.** First, the user decided that **computed fields are left out by default**. `IncludeComputed()` adds them for display. Validation is deliberately not relaxed, because computed values would be overwritten anyway. Default output is therefore always readable by xcl, and output with computed values is for reading only. This replaces the spec's "Nothing left out" requirement as written. Second, lifecycle events today carry the pre-call resource on provider types, so provider-filled values never appear, and carry nothing for registered types (`internal/parser/lifecycle.go:90-94,132,400`). The user wants resource data off events unless asked for, on security grounds. A new `xcl.WithEventData(level)` option takes one of three levels:

| Level | What events carry |
|---|---|
| `EventDataNone` (default) | No `Data` on any event. |
| `EventDataRaw` | Every lifecycle event carries the resource before the provider call. |
| `EventDataProcessed` | Success events carry the resource after the provider call, marshalled at the point state records it (status included). Other phases carry the pre-call resource. |

Raw and processed apply to every resource type, including registered config-only types. Processed data is byte-for-byte the state entry, so it feeds `EncodeSavedEntity` directly.

**Examples and docs.** Each example's `main` creates the registry and passes it both to `run` and to `prettylog.Handler`, whose new registry parameter lets it type event data. Each example also opts in to `WithEventData(xcl.EventDataProcessed)`. On an entity's create success, prettylog writes the success line and then the entity's text from `EncodeSavedEntity(registry, e.Data, xcl.IncludeComputed())`. The examples show values like `connection_string` because they are for people to read. The README and `docs/` gain a conversion section and the event-data option, and the stale Serialization/Deserialization sections are replaced. The docs site gains a guide page linked from the Guides menu, and `events.mdx` documents the event-data levels.

**Why this beats the alternatives.**
- **Methods on `Config`** would hide the registry, but an event receiver is built before any `Config` exists, and state tooling may have no `Config` at all.
- **A fresh reflection walker in the root package** would duplicate the decoder's tag rules and risk drifting from them. Code adapted from the MPL encoder would also have to live in `internal/xcl` anyway.
- **Encoding the JSON directly** would use `json` names and flatten blocks into attributes.

Evidence and citations are in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Tag-driven encoder (`internal/xcl/gohcl` encode, changed, library)** — Turns a Go value carrying `xcl` tags into an HCL body, using the same tag rules the decoder uses. It is repaired and extended:
  - it walks into remain-embedded base structs;
  - it encodes values held in interface fields by their dynamic type;
  - it skips nil pointers, slices and maps;
  - it leaves computed fields out unless told to include them;
  - it reports unrepresentable values as errors rather than panicking.

  It stays internal and MPL-licensed, with changes recorded in the fork's upstream log. It is used only by the entity encoder.

- **Entity encoder (root `xcl` package, new, library)** — The public conversion surface: `EncodeEntity`, `EncodeSavedEntity` and the `IncludeComputed` option.
  - It checks the value is an encodable entity: it has `Meta`, and is a resource-kind or bare registered type, not a builtin.
  - It builds the block header from `Meta`, then has the tag-driven encoder fill the body.
  - It removes ResourceBase bookkeeping: `meta` always, `disabled` when false, `depends_on` when empty.
  - It returns the formatted text for exactly one block.
  - For saved data, it first asks the saved-entity decoder for the typed value, so both paths share one encode step and agree byte for byte.

  It depends on the tag-driven encoder, the saved-entity decoder, the plugin registry and the shared errors.

- **Saved-entity decoder (new internal package, library)** — Owns turning one saved entity record back into its typed Go value. It reads the record's meta type, subtype and name, has the plugin registry create the matching typed value (including schema-built plugin types), and unmarshals the record into it. It reports an unregistered type or unreadable data through the shared errors. It is extracted from the state store's load loop so the stored format has a single reader. Used by the entity encoder and the file state store.

- **File state store (`state`, changed, library)** — Its load loop calls the saved-entity decoder per record instead of doing the decode inline. It still collects every unknown type into its existing aggregate error, so its behaviour and format are unchanged.

- **Plugin registry (`plugins/registry`, unchanged API)** — Remains the only authority on types. The saved-entity decoder uses its create-resource and load operations. `EncodeSavedEntity` triggers its once-only load, so plugin types resolve on a registry that no Config has loaded yet.

- **Shared errors (`errors` package, changed, library)** — Adds three sentinels, each with a pointer detail type, re-exported from `xcl`:
  - an unregistered type (names the type);
  - invalid saved data (wraps the decode cause);
  - not encodable (names what and why: a builtin, a non-entity, or an unrepresentable value).

- **Event data level (`events`, parser lifecycle and Config options, changed, library)**
  - The `events` package gains the data-level type and its three constants: none, raw, processed.
  - `Config` gains the `WithEventData` option, defaulting to none, and passes the level to the parser.
  - The parser's lifecycle emission sets `Data` by level:
    - none: nothing;
    - raw: the pre-call resource on every lifecycle event;
    - processed: success events carry the resource marshalled after the provider result and status are applied, which is identical to its state entry.
  - Registered config-only types, which today get no `Data`, now carry it at raw and processed.
  - The `Event.Data` documentation changes to match.

- **Pretty example receiver (`example/prettylog`, changed, examples)** — Its constructor takes the plugin registry. It still renders every event through the slog adapter. For an entity's create success that carries data, it also writes the entity's configuration text from `EncodeSavedEntity`, with computed values included, beneath the success line. A conversion failure is written as a single line rather than stopping the example.

- **Example programs (`example/plugin`, `example/configonly`, `example/appconfig`, changed, examples)** — Each `main` creates the registry, passes it to both prettylog and `run`, and turns on processed event data. Between them the examples exercise in-process plugin, external plugin and keyword-form registered types. Their tests assert that the text appears and reads back. The static example tests are updated for the new constructor and `run` shape.

- **Test fixtures (`internal/test_fixtures`, changed, tests)** — Provide root-importable coverage for the bare form (the existing bare `Cache` type). They also add a registered-type fixture whose Go field names differ from their `xcl` names and which has a repeated block, if the existing fixtures don't already have one.

- **Library docs (README, `docs/`, CHANGELOG; changed)** — A section on converting an entity and an entity's saved data to configuration text, with one example of each. The event-data option and levels, with the `Data` row corrected. The stale Serialization/Deserialization sections are replaced.

- **Docs site (`xcl-website`, changed)** — A new guide page on converting to configuration text, linked from the Guides menu and the README pages table. The events page documents the event-data option. Example pages are updated where they quote changed example code.

## Data Structures & Interfaces

**`xcl` public conversion surface (root package, new).** These are the only new public functions. Both return the text of exactly one formatted block, or no text and an error.

```go
// EncodeEntity returns entity as configuration text in xcl's own syntax.
// entity is a pointer to a resource-kind or bare registered type, such as one
// returned by Find, FindByType, All or Entities after Apply.
func EncodeEntity(entity any, options ...EncodeOption) ([]byte, error)

// EncodeSavedEntity returns the configuration text for one entity's saved
// data, exactly as state stores it and as events carry it at
// EventDataProcessed. registry resolves the type, including plugin types.
func EncodeSavedEntity(registry *registry.PluginRegistry, data []byte, options ...EncodeOption) ([]byte, error)

type EncodeOption func(*encodeOptions) // opaque; configured only through the functions below

func IncludeComputed() EncodeOption // also write provider-filled (computed) fields; off by default
```

The block header is `resource "<subtype>" "<name>"` for a resource-kind entity and `<type> "<name>"` for a bare one. Output with `IncludeComputed()` is for reading. xcl refuses configured computed fields, so it does not validate.

**Event data level (`events` package, new; `xcl` option, new).**

```go
package events

type DataLevel int

const (
    DataNone      DataLevel = iota // default: no Data on any event
    DataRaw                        // every lifecycle event: the resource before the provider call
    DataProcessed                  // success events: the resource as state records it; other phases as DataRaw
)
```

```go
package xcl

type EventDataLevel = events.DataLevel
const (
    EventDataNone      = events.DataNone
    EventDataRaw       = events.DataRaw
    EventDataProcessed = events.DataProcessed
)

func WithEventData(level EventDataLevel) ConfigOption
```

`Event` itself is unchanged. Only what fills `Data`, and its doc comment, change. `parser.ParserOptions` gains an `EventData events.DataLevel` field that `Config` sets.

**Shared errors (`errors` package, new; re-exported from `xcl`).** Each follows the sentinel plus pointer detail convention.

```go
var (
    ErrUnregisteredType  = errors.New("type is not known to the registry")
    ErrInvalidSavedData  = errors.New("data is not a saved entity")
    ErrNotEncodable      = errors.New("entity cannot be encoded as configuration")
)

type UnregisteredTypeError struct{ Type string }              // Unwrap() error → ErrUnregisteredType
type InvalidSavedDataError struct{ ID string; Err error }     // Unwrap() []error → {ErrInvalidSavedData, Err}; ID when readable
type NotEncodableError  struct{ What, Reason string; Err error } // Unwrap() []error → {ErrNotEncodable, Err}
```

**Saved-entity decoder (new internal package).** This is the single reader of one saved record. It is used by the root package and by `FileStateStore.Load`.

```go
package savedentity // internal

// Decode returns the typed entity for one saved record. It fails with
// *UnregisteredTypeError or *InvalidSavedDataError.
func Decode(registry *registry.PluginRegistry, data []byte) (any, error)
```

**Tag-driven encoder (`internal/xcl/gohcl`, changed, internal).** Encoding gains an options struct and an error return. The existing panicking entry points are kept for the upstream example test.

```go
type EncodeOptions struct {
    IncludeComputed bool
}

func EncodeBody(val any, dst *hclwrite.Body, options EncodeOptions) error
```

**`prettylog.Handler` (example package, changed signature).**

```go
func Handler(w io.Writer, level slog.Level, registry *registry.PluginRegistry) xcl.EventHandler
```

**Serialization boundaries.** Two contracts are unchanged:
- **The state file format.** Records are decoded by `savedentity.Decode` exactly as before.
- **The `Event` struct.**

At `EventDataProcessed`, a success event's `Data` is byte-identical to that entity's state record.

## Implementation Detail

**Existing pattern followed: the decoder's tag rules drive the encoder.** The encoder and the decoder already share one tag parser in the HCL fork. The repairs keep it that way. Every rule about names, blocks, repeated blocks, remain bases and computed fields comes from the same parsed tags. A field that decodes under a name therefore encodes under that name. Nothing in the root package inspects struct tags itself. The root encoder only chooses the block header and trims ResourceBase bookkeeping from the encoded body. Anyone reading the change finds encoding logic in the fork, next to decoding, and xcl-specific shaping in one short root file.

**New pattern: panics become errors at the fork boundary.** The upstream encoder panics on values it cannot represent: interface maps, `time.Time`, unknown or capsule cty values, and non-struct inputs. The new options-taking entry point returns an error naming the field path and the reason for each of these instead. Where the panic comes from deep in the cty/hclwrite code, a single deferred recover at the entry point converts it into an error. That recover is limited to that one function, with a comment saying why. The public functions never panic on a well-formed entity.

**New pattern: interface-held values encode by their dynamic type.** Schema-built plugin types hold named scalars as `interface{}`. The encoder resolves such a field's dynamic value before implying its cty type, so a plugin field typed as a named string still appears. Interface fields that hold nil are skipped like any other nil.

**Code-shape change: one reader of saved records.** The per-record logic in the state store's load loop moves into the new internal decoder. State keeps its aggregate unknown-types error by collecting the decoder's typed errors, so its observable behaviour is unchanged. The root package reaches the same reader through `EncodeSavedEntity`. There is now exactly one place that knows how a saved record names its type.

**Code-shape change: event data is chosen in one helper.** Today each lifecycle emission site passes whatever bytes it happens to hold. Instead, the parser's lifecycle-event helper decides `Data` from the configured level and the phase. It takes the pre-call bytes, plus, for a success, the resource after the provider result and status were applied. Emission sites pass the resource rather than choosing bytes. At level none nothing is marshalled, so the default costs no extra work. Registered config-only types go through the same helper, which is how they start carrying data. The processed marshal happens after status is set, at the same point the resource is appended to state, so event and state cannot disagree.

**Examples: the registry moves up to `main`.** Each example's `main` now creates the registry and hands it to both the pretty receiver and `run`. This is the dependency-injection shape the conventions ask for, and the static example tests are updated to expect it. The pretty receiver stays a thin wrapper over the slog adapter. It adds exactly one behaviour: an entity's configuration text beneath its create-success line. The text is indented and written through the same writer, so example output stays in one stream.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **Event-based logging (spec and plan `20260922061954-event-based-logging`)** — Provides `events.Event`, the operation runner that loads the registry before delivery, and `example/prettylog`. It is implemented on the `event-based-logging` branch and **must be merged, or this plan built on that branch, before starting**.
- **HCL fork (`internal/xcl/gohcl`, `internal/xcl/hclwrite`, `internal/xcl/tags`)** — Provides the tag parser, the encoder and formatting. The encoder is changed in place under the MPL rules. hclwrite and tags are used unchanged.
- **Plugin registry (`plugins/registry`)** — Provides `CreateResource`, `Type` and the once-only `Load`. No API change.
- **Schema reconstruction (`internal/schema`)** — Rebuilds plugin types from their schema, keeping the plugin's `xcl` tags. No change. The encoder adapts to its `interface{}` fields.
- **State store (`state`)** — Its load loop is refactored onto the shared decoder. Its format and behaviour are unchanged.
- **Parser lifecycle (`internal/parser`)** — Its event emission changes to honour the data level.
- **Shared errors (`errors`)** — Gains three sentinels with detail types.
- **go-cty fork (`github.com/zclconf/go-cty` replaced by `jumppad-labs/go-cty`) and `gocty`** — Used for type implication. No change.
- **charmbracelet/log** — Already pinned, used only by `example/prettylog`. No change.
- **Docs site repo (`xcl-website`)** — Receives a new guide page and edits. No build-tooling change.
- No new third-party dependencies.

## Testing Approach

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

- **Round trip (100% of example resources read back):** Descoped with the read-back requirement. The text is for display, not reprocessing, so it is not validated as a configuration. Replaced by the check that every entity converts without error.
- **Agreement (0 mismatches between entity and saved data):** Behavioural test. For every entity each example creates, the example tests convert the entity found in `Config` and its state record, and assert the two texts are byte-identical.
- **Visible in the examples:** Behavioural test. Each example's tests run with a buffer-backed pretty receiver and processed event data. They assert that, for every created entity, its header line appears after its create-success line. A manual look at real terminal output is also captured in the implementation test plan ("Manual — captured in the implementation test plan").
- **Easy to use (a developer can convert following the docs alone):** Manual — captured in the implementation test plan.

## Milestones & Phases

### Milestone 1: Developers can turn an entity, or its saved data, into configuration text

**What changes**: A developer embedding xcl can take any single resource-kind or bare registered entity and get it back as configuration text in xcl's own syntax, ready to print or write to a `.xcl` file. The entity can be one they hold after `Apply`, or one entity's saved data exactly as xcl stores it in state. The text uses the block type, labels and field names a person would write, with nested and repeated blocks as blocks. Provider-filled (computed) values are left out by default, so the text is valid xcl that reads back to the same configured values. An opt-in option adds computed values for display. Both starting points give identical text for the same entity. Plugin types (in-process and external) and registered types (keyword and bare form) all work. Unregistered types, unreadable data and builtins such as variables fail with a clear error and no text. The state store's loading behaviour is unchanged. It now shares the same reader of saved records.

**Validation point**: The root-package conversion tests pass for every acceptance criterion: header, names, repeated blocks, read-back, agreement, determinism, formatter-stable, computed off/on, every kind, errors, and one block per call. The encoder unit tests and the unchanged state-store tests pass. The full test suite passes.

#### - [x] Phase 1.1: The internal encoder writes what the decoder reads
**Repo:** xclconfig

This phase repairs the tag-driven encoder inside xcl's HCL fork so it faithfully writes any value the decoder can read. It walks into embedded base structs, and it writes values that plugin types hold in untyped fields. It skips empty (nil) values and leaves computed fields out unless asked. Values it cannot represent become errors rather than crashes. The encoder stays internal. The existing upstream entry points keep working.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-the-internal-encoder-writes-what-the-decoder-reads)

**Acceptance criteria**:
- [x] Fields of an embedded base struct are written alongside the outer struct's fields.
- [x] A plugin-style field held as an untyped value, such as a named string, is written with its value.
- [x] Nil pointers, slices and maps are not written. Zero numbers, false and empty strings that are present are written.
- [x] Computed fields are absent by default and present when inclusion is requested.
- [x] A value the encoder cannot represent, such as a map of untyped values or a timestamp, returns an error naming the field. Nothing panics.
- [x] The fork's change log records the modifications, and each changed file carries the fork's copyright notice.

#### - [x] Phase 1.2: One reader for saved entity records, with clear errors
**Repo:** xclconfig

This phase moves the logic that turns one saved state record back into a typed entity out of the file state store into a shared internal reader. The state store and the new public conversion then both use it. It adds the shared error types for an unregistered type, unreadable saved data and an entity that cannot be encoded, each re-exported from the root package. Loading state behaves exactly as before.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-one-reader-for-saved-entity-records-with-clear-errors)

**Acceptance criteria**:
- [x] A saved record of a registered type, a plugin type or a builtin is read back into its typed value with the same field values as before.
- [x] A record naming a type the registry doesn't know fails with an error that names the type and matches the unregistered-type sentinel.
- [x] A record that isn't valid saved-entity data fails with an error matching the invalid-saved-data sentinel.
- [x] Loading a state file gives the same entities, and the same aggregate error listing unknown types, as before the change.
- [x] Applications can match all three new errors through the root package.

#### - [x] Phase 1.3: Public conversion of an entity and of its saved data
**Repo:** xclconfig

This phase adds the public functions that convert one entity, or one entity's saved data, into formatted configuration text. It also adds the option that includes computed values. Each function builds the block header the way a person writes it, removes xcl's internal bookkeeping, and refuses builtins. Saved data is typed through the registry, and the registry loads its plugins when needed, so the two paths give identical text. Tests cover every acceptance criterion in the spec, with the example-specific ones carried in Milestone 3.

*Technical detail:* [context.md#phase-13](./context.md#phase-13-public-conversion-of-an-entity-and-of-its-saved-data)

**Acceptance criteria**:
- [x] After an apply, converting a registered database entity returns text beginning `resource "database" "main" {` with `port = 5432` and a `timeouts` block. Converting its state record returns identical text.
- [x] The output uses configuration names rather than Go field names. An entity with two repeated blocks shows both blocks under the same name. A bare-form type starts with `cache "<name>" {` and has no `resource` keyword.
- [x] Default output contains no computed fields. With the computed option, a computed value the provider filled in appears.
- [x] Converting the same entity twice gives byte-identical text, and running the formatter over it changes nothing.
- [x] An in-process plugin type, a keyword-form registered type and a bare-form registered type each convert successfully.
- [x] Saved data naming an unregistered type returns an error naming the type and no text. Data that is not a saved entity returns an error and no text. A variable, output or module returns a not-encodable error and no text.
- [x] Converting three entities one at a time gives three texts, each with exactly one top-level block.
- [x] Converting saved data works with a registry that no configuration has loaded yet.

#### - [x] Phase 1.4: Output shows only what the user wrote, and marks what the provider filled in
**Repo:** xclconfig

This phase makes the text show only what a person put in the configuration. Anything xcl or a provider recorded is left out, at every depth, including inside an attribute whose value is a whole object. When provider-filled values are asked for, each one is marked with a comment so a reader can tell it apart from something that was configured. Writing a comment needs a small addition to the HCL fork, which has no public way to set one.

*Technical detail:* [context.md#phase-14](./context.md#phase-14-output-shows-only-what-the-user-wrote-and-marks-what-the-provider-filled-in)

**Acceptance criteria**:
- [x] An attribute whose value is a whole object shows only the fields a person set. The object carries no `meta`, no `depends_on`, no `disabled` and no provider-filled fields.
- [x] `depends_on` is not written, at any depth.
- [x] With provider-filled values asked for, each one written into a body is followed by a comment saying the provider set it, including inside a nested block. A provider-filled field inside an attribute whose value is an object is shown but carries no comment: such an object is rendered from a value rather than from tokens, so there is nowhere to hang one.
- [x] Default output for every entity of the container fixture, which has an attribute holding a whole object, converts without error and contains none of the bookkeeping above.
- [x] The fork's change log records the comment API, and each changed file carries the fork's copyright notice.

**Descoped requirements**:
- Readable by xcl again — descoped: the user decided the text is for display rather than reprocessing, so read-back is no longer a guarantee and validation is not relaxed to make it one.

### Milestone 2: Applications choose whether events carry resource data

**What changes**: Lifecycle events no longer carry resource data unless the application asks for it. This keeps configuration and state from travelling through event receivers by default. A new configuration option selects one of three levels:
- **None**, the default.
- **Raw**: the resource before its provider call, on every lifecycle event.
- **Processed**: success events carry the resource as xcl records it in state, including values providers filled in.

The processed level works for registered config-only types as well as plugin types, which previously carried no data. Processed data can be passed straight to the saved-data conversion from Milestone 1. This is a behaviour change: applications that read `Data` today must now opt in to raw to keep what they had.

**Validation point**: Event tests pass for each level:
- **None**: no data on any event.
- **Raw**: pre-call data on start and success.
- **Processed**: a success event's data equals that entity's state record, and it includes provider-filled values.

Registered types carry data at raw and processed. Existing event tests are updated to opt in. The full test suite passes.

#### - [x] Phase 2.1: Event data is off by default, with raw and processed levels
**Repo:** xclconfig

This phase adds a configuration option that decides what resource data lifecycle events carry. With none, the default, events carry nothing. Raw gives every lifecycle event the resource as it was before its provider call. Processed gives success events the resource as xcl records it in state, including values the provider filled in. Registered config-only types, which previously carried nothing, now carry data at both levels. Existing tests that relied on data being present opt in to raw.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-event-data-is-off-by-default-with-raw-and-processed-levels)

**Acceptance criteria**:
- [x] With no option set, no lifecycle event of any resource carries data.
- [x] At the raw level, a plugin resource's create start and create success both carry the resource as it was before the provider call.
- [x] At the processed level, a plugin resource's create success carries the resource including the provider-filled value. It has the same content as that resource's entry in the saved state, and its status is `created`.
- [x] At raw and processed, a registered config-only type's create success carries its data.
- [x] Processed data from a create success converts with the saved-data function to the same text as the entity itself.

### Milestone 3: The examples show each created entity's configuration

**What changes**: Running any example program shows each entity it creates as configuration text, directly beneath that entity's create-success line. Provider-filled values such as connection strings are included, so a reader sees what was actually created. Each example turns on processed event data and hands its plugin registry to the pretty receiver.

**Validation point**: Each example's tests find every created entity's header after its create-success line in the receiver output. Default-option text for every created entity validates as a configuration, and matches the text from its state record. The static example tests pass with the new `main` shape.

#### - [x] Phase 3.1: The pretty receiver prints a created entity's configuration
**Repo:** xclconfig

This phase gives the example receiver access to the plugin registry. Whenever an entity's create succeeds with data attached, the receiver writes the entity's configuration text, computed values included, beneath the success line. If the text cannot be produced, it writes a single explanatory line instead and carries on.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-the-pretty-receiver-prints-a-created-entitys-configuration)

**Acceptance criteria**:
- [x] A create success carrying a registered entity's data is followed in the output by that entity's configuration text.
- [x] Events other than a create success, and a create success without data, produce exactly the lines they did before.
- [x] A create success whose data names an unknown type produces one line saying it could not be shown, and later events are still written.

#### - [x] Phase 3.2: Every example shows its created entities
**Repo:** xclconfig

This phase moves registry creation into each example's `main`, hands the registry to the receiver and to `run`, and turns on processed event data. It then adds example tests covering the spec's postgres-specific criteria and the success metrics: every created entity is visible, reads back, and agrees with its state record. The external plugin's types are among those covered.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-every-example-shows-its-created-entities)

**Acceptance criteria**:
- [x] Running the plugin example at the default output level shows `resource "postgres" "main" {` with `port = 5432`, a `timeouts` block and the filled-in `connection_string`, after its create success line. The configonly and appconfig examples do the same for each entity they create.
- [x] For every entity each example creates, default-option text converts without error. It is not validated as a configuration: the user descoped read-back, the text is for display.
- [x] For every entity each example creates, the text from the entity and the text from its state record are identical.
- [x] External plugin types (app, ingress) convert successfully.
- [x] The static checks on example programs pass with the registry created in `main`.

### Milestone 4: Converting to configuration text is documented

**What changes**: The library README and docs, and the xcl.dev site, explain how to convert an entity and an entity's saved data to configuration text, with one example of each. They cover the computed-values option and the event-data levels. The event documentation reflects that data is off by default. The stale serialization sections in the README are replaced.

**Validation point**: The README section and the new site guide each contain both examples. The site builds and type-checks cleanly. The guide is reachable from the Guides menu.

#### - [x] Phase 4.1: Library documentation
**Repo:** xclconfig

This phase documents the conversion functions and the computed option in the README, with one example starting from an entity and one starting from saved data. It documents the event-data option and its levels, corrects the description of event data, and replaces the stale serialization sections. The architecture docs and the changelog are updated to match.

*Technical detail:* [context.md#phase-41](./context.md#phase-41-library-documentation)

**Acceptance criteria**:
- [x] The README has a section on converting to configuration text, with an entity example and a saved-data example that match the real API.
- [x] The README's events section says data is off by default and explains the three levels.
- [x] No README section refers to serialization functions that don't exist.
- [x] The changelog lists the new functions, the new option and the change in event-data behaviour.

#### - [x] Phase 4.2: Documentation site guide
**Repo:** xcl-website

This phase adds a guide page to xcl.dev on converting an entity and its saved data to configuration text, with one example of each and the computed option. The page is linked from the Guides menu and the site README. The events guide gains the event-data option, and any example page quoting changed example code is brought in step.

*Technical detail:* [context.md#phase-42](./context.md#phase-42-documentation-site-guide)

**Acceptance criteria**:
- [x] The site has a guide on converting to configuration text, with an entity example and a saved-data example, reachable from the Guides menu.
- [x] The events guide explains that event data is off by default and describes the three levels.
- [x] Example pages match the example code they quote.
- [x] The site builds and type-checks without errors.

## Open Questions

- **Do schema-built plugin fields held as `interface{}` round-trip to the same configured value?** Named scalars in plugin types (for example `time.Duration`, or `type Mode string`) become `interface{}` when the host rebuilds the type from the plugin's schema, and after a JSON decode they hold a `float64` or `string`. Whether the text written from that value re-parses into the plugin's real type with an equal value can only be seen once the encoder change is exercised against a real plugin type. It depends on Phase 1.1's interface handling and on the example plugins' field types. **When hit:** if any example or fixture entity fails read-back or agreement because of such a field, STOP and ask the user whether to special-case the type (for example, write durations as strings) or accept and document the limitation.
- **Do attributes whose value is a whole entity object survive read-back?** A struct-valued attribute that embeds `ResourceBase` encodes as an object that includes `meta`, `depends_on` and `disabled`. Whether any example or fixture uses such an attribute, and whether the parser accepts it back, only shows when Phase 3.2's read-back tests run over every example entity. **When hit:** STOP and ask whether to strip bookkeeping inside nested objects too, or to exclude such fields.

## Out of Scope

- **Masking or omitting sensitive values** such as passwords. Output shows every value as held. Tracked in https://github.com/jumppad-labs/xcl/issues/1. The opt-in, off-by-default event data level limits where values travel, but it is not masking.
- **Converting builtin blocks** (`variable`, `output`, `module`). They are refused with a not-encodable error. Saved builtins also can't be rebuilt faithfully, because `cty.Value` fields don't survive JSON.
- **Other input formats.** Only xcl's own syntax is written. Nothing is read from other formats.
- **Converting several entities in one call.** One call converts one entity, and callers loop.
- **Making text with computed values read back.** By the user's decision, validation is not relaxed to accept configured computed fields. Output made with the computed option is for reading only.
- **Preserving original expressions.** The text holds resolved values, so a reference such as `resource.postgres.main.port` comes out as its literal value, and comments and layout from the source file are not kept.
- **A public HCL writer.** The hclwrite and gohcl packages stay internal (a spec constraint).
- **Writing a reference in place of a resolved value.** An attribute that a person wrote as `networkobj = resource.network.onprem` comes out as the object it resolved to. Writing the reference back needs the parser to record which field each reference fed, which it discards today (`internal/parser/parser.go:1119-1132`). Tracked for a separate spec.
- **Separating hand-written `depends_on` from reference-derived links.** `depends_on` is not written at all, because after parsing it holds both and they cannot be told apart. Tracked for the same separate spec as the reference work above, which needs the same provenance tracking. See the Phase 1.2 changelog entry for why `Meta.Links` cannot simply replace it today.
- **Removing the stray `configonly` binary committed at the repository root.** It is unrelated to this feature and is worth a separate cleanup.

## Changelog

### 2026-09-23 — Phase 1.1: The internal encoder writes what the decoder reads

**What was done**: Repaired the tag-driven encoder in xcl's vendored HCL fork so it writes every value the decoder can read. A new `gohcl.EncodeBody(val, dst, EncodeOptions)` walks attributes, blocks and the `remain` field in one field-order pass, so fields of an embedded base such as `types.ResourceBase` are written alongside the embedding struct's own. It writes a value held in an interface field by the value's dynamic type, leaves out a field holding nothing (nil pointer, interface, slice or map) while keeping present zero scalars, omits `computed` fields unless `IncludeComputed` asks for them, and returns an error naming the field instead of panicking. `EncodeIntoBody` and `EncodeAsBlock` keep their signatures and their panic-on-failure behaviour.

**Deviations**: None from the phase as planned. One correction to a research note: `research.md` suggested the encoder should skip `meta` during remain recursion, and it does not. `gocty.impliedStructType` builds its object from `xcl` tags only, so `Meta.Properties map[string]any` is already excluded and `meta` encodes cleanly as an object. Trimming it stays in Phase 1.3's root wrapper, which keeps xcl-specific shaping out of the MPL-licensed fork as the plan intends.

**Files changed**:
- `xclconfig: internal/xcl/gohcl/encode.go`
- `xclconfig: internal/xcl/gohcl/schema.go`
- `xclconfig: internal/xcl/gohcl/encode_body_test.go`
- `xclconfig: internal/xcl/UPSTREAM.md`

**Discoveries**:
- `getFieldTags` was parsing the `computed` option and discarding it — `tags.Field.Computed` was read but never reached `fieldTags`. The encoder could not have honoured computed without adding the `Computed` set to `fieldTags` first.
- Upstream's block layout depends on resetting its "previous was a block" flag at the start of **each block field**, not per block. That reset is what puts a blank line before the first block of each group; dropping it silently broke `ExampleEncodeIntoBody`'s spacing while every assertion-based test still passed. It is now explicit and commented in `encodeBlock`.
- `gocty.impliedStructType` (`internal/cty/gocty/type_implied.go:76-123`) derives its object type from `xcl` struct tags alone, so json-only fields such as `Meta.Properties`, `Links`, `Parents` and `Status` never reach cty. This is why encoding `meta` does not fail on its `map[string]any`.
- The blank-line layout state has to be shared between a remain base and the struct embedding it, because both write into one body. That is why the encoder is a `bodyEncoder` struct bound to a destination body rather than a free function.
- `reflect` permits `Interface()` on exported fields promoted through an **unexported** embedded struct, so test fixtures may use unexported base types and still exercise the real promotion path.

### 2026-09-23 — Phase 1.2: One reader for saved entity records, with clear errors

**What was done**: Moved the per-record decode out of `FileStateStore.Load` into a new internal package, `savedentity`, so there is exactly one place that knows how a saved record names its type. Added the three shared sentinels with their pointer detail types and re-exported them from the root package. The state store's load loop now calls the shared reader and maps its typed errors back onto the aggregate it already raised, so its behaviour and error format are unchanged.

**Deviations**: One deliberate departure from the house error pattern. `InvalidSavedDataError.Unwrap()` and `NotEncodableError.Unwrap()` omit the cause from their `[]error` slice when it is nil, rather than returning `[]error{Sentinel, nil}` as `PluginLoadError` does. `NotEncodableError`'s cause is genuinely optional, since a builtin or a non-entity has no underlying error. `errors.Is` behaves identically either way, so this is tidiness rather than a behaviour fix.

**Files changed**:
- `xclconfig: errors/encode_errors.go`
- `xclconfig: errors/encode_errors_test.go`
- `xclconfig: internal/savedentity/savedentity.go`
- `xclconfig: internal/savedentity/savedentity_test.go`
- `xclconfig: state/file_state_store.go`
- `xclconfig: config.go`
- `xclconfig: encode_errors_test.go`

**Discoveries**:
- `state.UnknownTypesError` does not wrap the reader's errors, so `ErrUnregisteredType` and `ErrInvalidSavedData` are **not** matchable through a `FileStateStore.Load` failure. This is pre-existing behaviour, not a regression — the value-typed aggregate never wrapped anything — and the plan requires the state store's behaviour and format stay unchanged, so it was preserved deliberately. If a caller ever needs to match a sentinel through `Load`, that is a separate, deliberate change to the aggregate.
- Nothing raises `ErrNotEncodable` yet; its raiser arrives with the public encoder in Phase 1.3. The root-package criterion is met by the re-export and `errors.As` alias tests. **Phase 1.3 should add a library-raised match test for it**, the way 1.2 did for the other two.
- `savedentity`'s own tests must live in an external `savedentity_test` package: `parser` → `state` → `savedentity` would otherwise cycle, and the tests need `parser.TestPlugin` to exercise the plugin-type path.
- Plugin types are schema-generated, so a decoded plugin entity has no Go type to assert against. Its fields are read back through JSON and its identity through `types.GetMeta`.

### 2026-09-23 — Phase 1.3: Public conversion of an entity and of its saved data

**What was done**: Added the public conversion surface in the root package: `EncodeEntity`, `EncodeSavedEntity` and the `IncludeComputed` option. Both entry points share one encode path, which is what makes an entity and its saved record produce byte-identical text. The header is built from `Meta` the way a person writes it, builtins are refused, and `EncodeSavedEntity` loads the registry itself so plugin types resolve on a registry no configuration has used. Added the `internal/test_fixtures/config/encode` fixture, which exercises every shape in one apply.

**Deviations**: Substantial, all agreed with the user during this phase. The plan's second open question fired immediately, and the answers reshaped the phase:
- **Read-back was descoped.** The user decided the text is for display rather than reprocessing, so the two read-back tests the plan listed were not written and the requirement is recorded under `**Descoped requirements**:`.
- **The phase was split.** Recursive stripping, the hclwrite comment API and computed-marking moved to the new Phase 1.4, so 1.3 stayed the size it was scoped at.
- **Two items moved to a separate spec**: writing a reference in place of a resolved value, and separating hand-written `depends_on` from reference-derived links. Both are in Out of Scope.

**Files changed**:
- `xclconfig: encode.go`
- `xclconfig: encode_test.go`
- `xclconfig: internal/test_fixtures/config/encode/main.xcl`

**Discoveries**:
- **The encoder resolves every reference to a literal, so it exposes every computed field the original configuration was hiding behind a reference.** `configuredComputedValues` (`internal/parser/validate.go:318-321`) only runs its computed check when the expression is an `*hclsyntax.ObjectConsExpr`; a traversal such as `resource.network.onprem` returns unchecked. The validator inspects syntax, never resolved values. This is why a configuration can validate while its encoded form does not, and it is general, not specific to any one field.
- A struct-valued **attribute** (not a block) is handed whole to `gocty`, which builds its object from every `xcl`-tagged field. Neither the encoder's computed skip nor the root wrapper's `meta` trim reaches inside it. `ContainerBase.NetworkObj Network` is the case in the repo: a value type, never nil, so it always encodes. This is Phase 1.4's work.
- **`DependsOn` is not what the user wrote.** `types.AppendUniqueDependency` mirrors the privately-computed `Meta.Links` into the public `DependsOn` under a "backwards compatibility" comment, but the mirror is load-bearing: `dag.go:62-74` copies `Links` into `DependsOn` and reads `DependsOn` back to build the graph. Everything else load-bearing already reads `Links`. The contained fix, for the separate spec, is to point `getResourceDependencies` (`internal/parser/util.go:506`) at `Meta.Links` and drop both the copy loop and the mirror write.
- A test asserting formatter output must not bake in alignment padding. `port       = 5432` was written with today's spacing, which is set by the longest name in the block; Phase 1.4 removing `depends_on` would have broken it for an unrelated reason. It now matches `port\s+= 5432`.

### 2026-09-23 — Phase 1.4: Output shows only what the user wrote, and marks what the provider filled in

**What was done**: The text now shows only what a person put in the configuration. An attribute whose value is a whole object is built by the encoder rather than handed to `gocty`, so the rules about provider-filled and empty values reach every depth, and the object leaves out the embedded base that carries `meta`, `depends_on` and `disabled`. `depends_on` is no longer written at all. When provider-filled values are asked for, each is followed by `# set by the provider`. Writing a comment needed a public API on the fork's attribute type, which did not exist.

**Deviations**: One acceptance criterion was narrowed rather than ticked as written. A provider-filled field inside an object-valued attribute is shown but carries no comment: the object is rendered from a `cty.Value` through `TokensForValue`, which has no place to hang one. Comments work for every field written into a body, including inside nested blocks. The criterion in this plan now says exactly that.

**Files changed**:
- `xclconfig: internal/xcl/gohcl/encode.go`
- `xclconfig: internal/xcl/gohcl/encode_body_test.go`
- `xclconfig: internal/xcl/hclwrite/ast_attribute.go`
- `xclconfig: internal/xcl/hclwrite/ast_attribute_test.go`
- `xclconfig: internal/xcl/hclwrite/ast_body.go`
- `xclconfig: internal/xcl/hclwrite/node.go`
- `xclconfig: internal/xcl/UPSTREAM.md`
- `xclconfig: encode.go`
- `xclconfig: encode_test.go`

**Discoveries**:
- **Two upstream HCL bugs, both in hashicorp/hcl and neither introduced here.** First, `SetAttributeRaw`, `SetAttributeValue` and `SetAttributeTraversal` each declared a new `attr` inside their else branch, shadowing the one they return, so creating an attribute returned nil despite the documented contract. Second, `node.ReplaceWith` rewired a replaced node's neighbours but never updated the owning list's `first`/`last`, so replacing the first node of a list left `first` on a detached node and silently dropped everything after it. `Detach` in the same file does update both pointers, which is what shows the omission was an oversight rather than a design.
- The second bug is worth remembering for how it presents: it does not error, it **silently deletes content**. Setting a lead comment on an attribute removed the entire attribute from the output, because `leadComments` is the first of an attribute's child nodes. `SetLineComment` escaped only because `lineComments` is neither first nor last.
- Skipping a struct's `remain` base when building an object-valued attribute removes `meta`, `depends_on` and `disabled` in one stroke, and does it **without the fork needing to know any xcl-specific field name**. An explicit skip-list option was written first and deleted once this fell out. It keeps the layering that `gotchas/xcl-tags-gate-what-reaches-cty.md` argues for.

### 2026-09-23 — Phase 2.1: Event data is off by default, with raw and processed levels

**What was done**: Lifecycle events carry no resource data unless the application asks. `WithEventData` takes one of three levels, and a single helper in the parser decides what each event carries from the level and the phase, so an emission site passes what it holds and never chooses. At the raw level every lifecycle event carries the resource as it was before the provider call. At processed, a success event carries the resource as state is about to record it, provider-filled values and final status included. Registered config-only types now carry data at both levels, where before they carried none.

**Deviations**: None from the phase as planned.

**Files changed**:
- `xclconfig: events/events.go`
- `xclconfig: options.go`
- `xclconfig: events.go`
- `xclconfig: config.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/events.go`
- `xclconfig: internal/parser/lifecycle.go`
- `xclconfig: internal/parser/callbacks.go`
- `xclconfig: internal/parser/parse_test.go`
- `xclconfig: internal/parser/lifecycle_test.go`
- `xclconfig: config_event_data_test.go`
- `xclconfig: config_events_test.go`
- `xclconfig: config_destroy_test.go`

**Discoveries**:
- Moving the success event out of `callProvider` is what makes processed data trustworthy: the status is set by the caller *after* the provider returns, so a success emitted from inside `callProvider` could never carry it. The cost is that **every** caller must now emit its own success. Read, changed and the rebuild path's destroy each needed theirs restored by hand after the move; losing one would have been a silent regression no test was watching for.
- Two existing tests changed meaning rather than merely needing an opt-in, and both were edited deliberately. `TestApplyCallsEventHandlerWhenRegisteredTypeIsApplied` asserted a registered type's `Data` is nil, which used to mean "registered types never carry data" and now means "the default carries nothing"; its fixture keeps the default level and its comment says so. `TestConfigDestroyReportsOnlySuccessForVariable` asserted a variable's destroy data is nil, which now contradicts raw carrying every resource type; the assertion was incidental to that test's stated subject and was removed rather than weakened.
- Builtins carry data at raw and processed too, not only registered types. `handledWithoutProvider` covers both, and the plan's "every resource type" reading is the consistent one. `EncodeSavedEntity` still refuses to encode a builtin, so nothing downstream is affected.

### 2026-09-23 — Phase 3.1: The pretty receiver prints a created entity's configuration

**What was done**: The example receiver takes the plugin registry and, when an entity's create succeeds with data attached, writes that entity's configuration beneath the success line, computed values included and indented so it reads as belonging to the line above. A nil registry, or a run asking only for warnings and errors, leaves it out entirely.

**Deviations**: One behaviour added beyond the phase as written. A builtin is skipped **silently** rather than reported. The first run warned "unable to show configuration" for every variable, output and module, because a builtin is deliberately not encodable. That is a documented non-goal rather than a failure, and warning on each one in every example run would train a reader to ignore the warning. The receiver now treats `ErrNotEncodable` as nothing to show, and still reports a real failure such as an unregistered type, which is what the phase's third criterion is about.

**Files changed**:
- `xclconfig: example/prettylog/prettylog.go`
- `xclconfig: example/prettylog/prettylog_test.go`
- `xclconfig: example/plugin/main.go`
- `xclconfig: example/appconfig/main.go`
- `xclconfig: example/configonly/main.go`

**Discoveries**:
- Changing `Handler`'s signature forces the three example `main`s to change with it, which is Phase 3.2's work. They pass a nil registry for now, which is a documented argument rather than a placeholder, so the build stays green and the phases stay separable. Phase 3.2 replaces it with the real wiring.
- The receiver only ever sees bytes, never the entity, so it cannot know a record is a builtin before trying to encode it. Distinguishing "nothing to show" from "something went wrong" therefore has to be done on the returned error, which is what makes the three sentinels worth having as separate values rather than one.

### 2026-09-23 — Phase 3.2: Every example shows its created entities

**What was done**: Each example's `main` now builds the plugin registry and hands it to both the event receiver and `run`, and asks for processed event data. Running any example prints each entity's configuration beneath the line announcing it was created, provider-filled values included. The plugin example covers in-process and external plugin types, configonly and appconfig cover registered types.

**Deviations**: One acceptance criterion was rewritten to match the descope rather than left unmet. It asked that every entity's text "validates as a configuration without errors", which is read-back; it now asks that every entity converts without error. The plan's Round trip success metric is marked descoped for the same reason.

**Files changed**:
- `xclconfig: example/plugin/main.go`, `example/plugin/main_test.go`
- `xclconfig: example/configonly/main.go`, `example/configonly/main_test.go`
- `xclconfig: example/appconfig/main.go`, `example/appconfig/main_test.go`

**Discoveries**:
- **The state file is empty by the time `run` returns** for the plugin and configonly examples, because `run` ends with `Destroy`. An agreement test that reads state after `run` compares nothing and passes vacuously. Those tests read the real state file at the `apply`/`success` event instead, which is the one moment it holds records, and every agreement test carries a `require.NotZero(compared)` guard so it can never pass on an empty set. appconfig does not destroy, so it reads the file normally.
- Threading the registry through `run` touched 32 existing call sites across the three example test files, far more than the production change. A signature change in an example's `run` is much more expensive than it looks.
- Example output now shows secrets verbatim, such as a postgres `password`. That is the documented Out of Scope item (jumppad-labs/xcl#1), not a regression, but it is more prominent than before: it is in terminal output by default rather than only reachable through the API.

### 2026-09-23 — Phase 4.1: Library documentation

**What was done**: The README gained a section on converting to configuration text, with an example starting from an entity and one starting from saved data, the computed option and the errors, plus a subsection on the event data levels and a corrected `Data` row. The stale `## Serialization` and `## Deserialization` sections, which described `Config.ToJSON` and `Parser.UnmarshalJSON` that do not exist, were replaced with a pointer to the state docs and the new section. `docs/state.md` gained a section on reading a saved record back. The changelog records the new functions, the option, the errors and both breaking changes.

**Deviations**: None.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: CHANGELOG.md`
- `xclconfig: docs/state.md`
- `xclconfig: readme_test.go`

**Discoveries**:
- The README documented two functions that have never existed in this codebase, `Config.ToJSON` and `Parser.UnmarshalJSON`. `readme_test.go` now asserts neither name reappears, alongside asserting the conversion section and both examples are present, so documentation drift of this kind fails the suite rather than waiting to be noticed.
- The docs have to state plainly that the text is for reading, not reprocessing, because every other property of the feature invites the opposite assumption. It is valid xcl syntax, it is formatted, it round-trips for simple types. Saying so once in the README, once in the changelog and once in `docs/state.md` is deliberate.

### 2026-09-23 — Phase 4.2: Documentation site guide

**What was done**: xcl.dev gained a guide on converting to configuration text, with an example starting from an entity and one starting from saved data, the computed option and its comment marking, what the text is and is not for, and the errors. It is linked from the Guides menu and the site README's page table. The events guide gained a section on the data levels and a corrected `Data` row, and the three example pages were brought in step with the example code, which now builds the registry in `main` and asks for processed event data.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/pages/events.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`
- `xcl-website: src/pages/examples/configuration-only.mdx`
- `xcl-website: src/pages/examples/application-config.mdx`
- `xcl-website: src/components/Nav.astro`
- `xcl-website: README.md`

**Discoveries**:
- The example pages quote the library's example code, so a signature change in `example/*/main.go` silently makes them wrong. The site README already warns about this. Three pages needed editing for one parameter added to `prettylog.Handler` and one to `run`, which is worth remembering before changing an example's shape again.
- `npx astro check` reports one pre-existing hint about a deprecated `execCommand` in the copy-to-clipboard script. It is unrelated to this work and was left alone.
