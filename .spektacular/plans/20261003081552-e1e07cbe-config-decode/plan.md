---
created_date: "2026-10-03"
document_status: final
closed_date: "2026-10-03"
---

# Plan: 20261003081552-e1e07cbe-config-decode

<!-- Metadata -->
<!-- Created: 2026-10-03T08:38:20Z -->
<!-- Commit: 8978499 -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Applications that use xcl for configuration-only work can declare a struct of their own and fill it from a processed configuration in one call, `c.Decode(&cfg)` or `xcl.Decode(c, &cfg)`. A `[]*T` field receives every block of registered type `T`, and a `*T` field receives the one block of that type. Today they must call one lookup per block type and assemble the result by hand. Adding a new block type then becomes a matter of adding a field. The call reuses the same lookup core as `All`, so the two can never disagree. The config-only example, the README and the docs site show it in use.

## Conventions

- **Shared error types live in the `errors` package, re-exported from `xcl`, sentinel-and-detail with pointer receivers** — `Decode` introduces `ErrInvalidDecodeTarget` / `*InvalidDecodeTargetError` in `errors/query_errors.go`, re-exported from `config.go`, and reuses `*NotUniqueError` unchanged.
- **Testing & Mocking: testify `require`, NEVER table-driven, NEVER mix positive and negative cases, favour verbosity** — every acceptance criterion becomes its own named test. Unusable-target cases (by value, nil pointer, non-struct) are three separate negative tests, not a table.
- **Generate test state with a real apply, not a hand-written state file** — every `Decode` test builds its configuration by `Apply`-ing an `.xcl` fixture, as `setupFindConfig` does. No state is hand-written.
- **Go code style: standard-library idiom, `any` over `interface{}`, descriptive names, explicit error handling** — `Decode(target any) error`, `reflect`-based field walk modelled on standard-library decoders (`encoding/json`'s non-nil-pointer target rule).
- **Dependencies: prefer the standard library** — the feature is `reflect` and `errors` only. No new module is added to `go.mod`.
- **Glossary: entity** — doc comments and README text call what `Decode` collects "entities" / "blocks a configuration declares", not "resources", because a registered type may be declared by its own keyword (`server "x" {}`).

## Architecture & Design Decisions

`Decode` fills an application's own struct from a processed configuration by walking the struct's fields with `reflect`. For each settable field shaped `[]*T` or `*T`, it asks the registry which address `T` is registered under (`PluginRegistry.TypePath`, `plugins/registry/plugin_registry.go:737`). It then collects that type's entities through the **same scan and the same conversion** that `All[T]` uses. A field whose element type the registry cannot reach is left exactly as it was. That covers a plain value, an unregistered struct, a nested-only block type and a plugin-provided type. All of the work is in the `xclconfig` repo, in package `xcl` (new `decode.go`, refactored `query.go`), with a new error in `errors/query_errors.go` re-exported from `config.go`. `Config` is the only public query surface (`architecture/config-is-the-public-query-surface.md`), so `Decode` lives there and nowhere else. It is offered as `c.Decode(&cfg)` and `xcl.Decode(c, &cfg)`. Both delegate to one unexported `decode`, following `Find`/`find` (`query.go:29-36`). The documentation lands in `xclconfig:README.md`, `xclconfig:CHANGELOG.md`, `xclconfig:example/configonly` and `xcl-website:src/pages/examples/configuration-only.mdx`.

**One implementation, not two that agree.** Go cannot instantiate `all[T]` from a `reflect.Type` at run time, so `Decode` cannot simply call it. Rather than give `Decode` its own loop over `c.Entities()`, the plan first splits the generic lookups into non-generic cores that take a `reflect.Type`: the type → path derivation `all[T]` performs, the `addressable` check, the type/subtype scan in `findByType[T]`, and `As[T]`'s identity-or-copy conversion. The generic `findByType[T]`, `all[T]` and `As[T]` then become thin typed wrappers over those cores, and `Decode` calls the cores directly. This is the same reasoning `all[T]` already records at `query.go:324-326`: going through the kind lookup means "the two cannot disagree". Spec requirement 3 turns that from a property of the tests into a property of the code. Because the conversion is shared, a registered entity stored as `*T` comes back as that very pointer, so the configuration's own instances land in the struct (requirement 4). This is the behaviour `require.Same` already pins for `All` (`query_all_test.go:23-38`). The refactor is landed first, behind the existing lookup tests, before `Decode` exists.

**Resolve everything, then assign.** `decode` runs in two phases. First it validates the target: a non-nil pointer to a struct, otherwise a new `ErrInvalidDecodeTarget` wrapped by `*InvalidDecodeTargetError`, following the sentinel-and-detail convention (`conventions/shared-errors-package.md`). Then it resolves every eligible field into a pending value. Only when no field failed does it assign them all. So a target is never left half-filled: not for an unusable target, and not when one single-block field matches several entities. In that case the returned error is the same `*NotUniqueError{Segments, Count}` that `findOne` builds (`query.go:233`), returned unwrapped so `errors.Is(err, ErrNotUnique)` and `errors.As` behave exactly as they do for `FindOne`. A `*T` field with no match is set to nil rather than returning `ErrNotFound`. A `[]*T` field with none gets an empty non-nil slice, mirroring `All`. Fields are matched by their type alone. No tags are read, and names play no part. Unexported fields, and embedded or nested structs, are never entered (spec non-goal).

**Not generic, so not version-gated.** `Decode` takes `any`, so its method form is an ordinary method that compiles on Go 1.25 (`go.mod:3`). Unlike the generic lookups, it needs no `//go:build go1.27` file (`query_methods_go127.go:1`), and the README's "Two spellings" section gains a sentence saying so. The rejected directions are a separate `Decode`-only scan kept in line by tests, a per-type decoder captured at registration (which needs a generic `RegisterType` and an API change), and re-decoding the source files (which yields copies). They are written up with evidence in `research.md#alternatives-considered-and-rejected`.

**The flagged reference risk is closed with coverage, not a fix.** A spike during discovery showed a nested block decoding a whole-block reference to a registered non-`resource` type into both a `T` and a `*T` field, with values filled in. The plan adds tests for exactly that, using test-local types that reference the existing registered `Cache`, and only plans code if those tests fail.

## Component Breakdown

- **Decode entry points (new, `xclconfig`).** These are the public `Config.Decode(target)` method and the package-level `Decode(c, target)` function. Neither holds any logic. Both delegate to the decoder, in the same way `Find`, `FindByType`, `FindOne` and `All` delegate to their unexported implementations. They are not generic, so both spellings are available on every supported Go version.

- **Decoder (new, `xclconfig`).** This is the unexported implementation behind both entry points, and it owns three jobs:
  - It validates the target: it must be a non-nil pointer to a struct.
  - It walks the target's direct fields, picking out exported, settable fields shaped `[]*T` or `*T`.
  - It applies the two-phase resolve-then-assign rule.

  For each eligible field it asks the type-path derivation which address the element type is registered under. A type the derivation cannot reach is not a decode field, and it is left alone. For a reachable type, it asks the kind scan for the matching entities. A slice field takes all of them. A single-pointer field takes exactly one, takes nothing when there are none, and fails with the not-unique error when there are several. The decoder never scans the configuration or converts an entity itself.

- **Type-path derivation (changed, `xclconfig`).** This is the step `All` performs today: refuse a type with no address of its own, then ask the registry which type and subtype a Go type is registered under. It now takes a `reflect.Type` and is shared. `All` reaches it through its generic wrapper, and the decoder calls it directly, so both resolve a Go type to an address identically.

- **Kind scan (changed, `xclconfig`).** This is the loop behind `FindByType`. It checks that the segments are typeable, keeps the entities whose type and subtype match, and converts each one to the wanted Go type. It now works over a `reflect.Type` and returns untyped results. `FindByType`, `FindOne` and `All` become thin typed wrappers over it, and the decoder uses it as is. This shared scan is what guarantees that `Decode` and `All` cannot disagree on which entities match or in what order.

- **Entity conversion (changed, `xclconfig`).** This is `As`'s rule: an entity that already is the wanted pointer type is returned as that same instance, a named type that cannot be the wanted type is refused, and anything else is copied. The rule moves into a non-generic form over a `reflect.Type`. `As` wraps it with its generic signature unchanged, and the kind scan calls it. Because the rule is shared, the decoder hands back the configuration's own instances exactly when the typed lookups do.

- **Plugin registry (unchanged, `xclconfig`).** It already answers which address a `reflect.Type` is registered under, and reports that it cannot reach plugin-provided types. That is the boundary between fields `Decode` fills and fields it leaves alone.

- **Lookup error vocabulary (changed, `xclconfig`).** This is the shared errors package plus its root re-exports. It gains one sentinel, an invalid decode target, together with a detail type that names what was passed. That type is matched with `errors.Is`/`errors.As` and re-exported from `xcl` like the other lookup errors. The not-unique sentinel and its detail are reused unchanged.

- **Test fixtures (reused, `xclconfig`).** The existing registered test types and fixtures supply almost every `Decode` case:
  - two databases, one app and one consumer, with the cache type registered but not declared
  - the bare cache fixture
  - the disabled fixture

  Only two gaps get anything new, and none of it goes into the shared fixture types. Two tiny fixture files declare the same pair of cache blocks in opposite orders. A small test-local type, with nested blocks that reference a registered cache as a whole, covers both the pointer and the value form.
- **Config-only example program (changed, `xclconfig`).** The example that registers several block types gathers them into one application struct with a single `Decode` call. Its per-type assembly lookups are removed, and its printed output stays the same. It keeps a portable lookup where one is genuinely a single-address question, so its portable-lookup guard still holds.

- **User documentation (changed, `xclconfig` and `xcl-website`).** The README and the changelog document `Decode`, including that it is not version-gated, and the README content tests are extended to guard the new section. On the docs site, the configuration-only example page shows a configuration and the `Decode` call that fills a struct from it.

## Data Structures & Interfaces

**Public surface (new).** There are two spellings with one contract, and neither is generic:

```go
// Decode fills target's []*T and *T fields, T a registered type, from the
// entities the configuration declares. target must be a non-nil pointer to a struct.
func (c *Config) Decode(target any) error
func Decode(c *Config, target any) error
```

| Field shape (exported, settable) | T reachable by the registry | Result |
|---|---|---|
| `[]*T` (or a named slice type with that shape) | yes | every entity of T, in declaration order; an empty non-nil slice when none |
| `*T` | yes, exactly one declared | that entity, the configuration's own instance |
| `*T` | yes, none declared | `nil` |
| `*T` | yes, several declared | call fails with `*NotUniqueError`; nothing assigned |
| any other shape, or T not reachable | — | left untouched |

**Error vocabulary (new).** These follow the existing sentinel-and-detail pattern. They live in the shared errors package and are re-exported from `xcl`:

```go
// ErrInvalidDecodeTarget is returned by Decode when target is not a non-nil
// pointer to a struct. Match with errors.Is.
var ErrInvalidDecodeTarget = errors.New("decode target must be a non-nil pointer to a struct")

// InvalidDecodeTargetError names what was passed instead.
type InvalidDecodeTargetError struct {
    Type reflect.Type // nil when target itself was nil
    Nil  bool         // a typed nil pointer was passed
}
func (e *InvalidDecodeTargetError) Error() string
func (e *InvalidDecodeTargetError) Unwrap() error // ErrInvalidDecodeTarget
```

`*NotUniqueError{Segments, Count}` and `ErrNotUnique` are reused unchanged. The error returned is exactly the one `FindOne` returns for the same type.

**Internal contracts (changed, unexported).** The generic lookups are split into non-generic cores over `reflect.Type`. Every generic signature that is exported today stays byte-for-byte the same:

```go
// typePath: addressable check + registry lookup, what all[T] does today.
func (c *Config) typePath(want reflect.Type) ([]string, error)

// entitiesOf: the kind scan behind findByType[T]; each result is a pointer to want.
func (c *Config) entitiesOf(want reflect.Type, path ...string) ([]any, error)

// asType: As[T]'s identity / refusal / copy rule; returns a pointer to want.
func asType(entity any, want reflect.Type) (any, error)
```

`findByType[T]`, `findOne[T]`, `all[T]` and `As[T]` become typed wrappers that call these and cast their `any` results to `*T`. There is no serialization boundary: `Decode` assigns in-memory pointers and writes nothing to state or to disk.

**Test fixture types.** None are added to the shared fixture package. The `Decode` tests reuse the existing registered `Database`, `App`, `Consumer` and `Cache`. The only new type is local to a test file. It covers the whole-block reference case that no existing type exercises:

```go
type cacheClient struct { types.ResourceBase; Primary *cacheLink; Mirror *cacheCopy } // cache_client "x" {}
type cacheLink struct { Cache *registered.Cache } // primary { cache = cache.east }
type cacheCopy struct { Cache registered.Cache  } // mirror  { cache = cache.west }
```

## Implementation Detail

**Generic wrappers over reflective cores (new pattern for the query layer).** Today each lookup is generic from top to bottom: `T` flows through `findByType[T]` into `As[T]`. After this change, the logic lives in three non-generic cores that take a `reflect.Type`. The generic lookups shrink to two or three lines each: call the core with `reflect.TypeFor[T]()`, then cast each `any` result back to `*T`. Someone reading the query code finds every rule once (what is addressable, how a type maps to an address, which entities match, when an entity is returned as-is and when it is copied), and finds the typed API as a visibly thin skin over it. This extends the existing "two spellings, one implementation" principle down a level. The method and function forms already share one unexported function. Now the typed and reflective callers share one core too. The refactor is behaviour-preserving and lands on its own, so the whole existing lookup suite (address, kind, Go-type, error and equivalence tests) proves it before `Decode` is written on top.

**The decoder follows the standard library's decoder idiom.** It refuses anything but a non-nil pointer to a struct up front, as `encoding/json.Unmarshal` does. It walks only the direct fields of that struct, with no recursion, no tags and no names. It classifies each field purely by the shape of its type. Resolution and assignment are separate passes. The first pass builds a list of pending (field, value) pairs and stops at the first error. The second pass sets them. This keeps the "nothing changes on failure" guarantee structural rather than relying on cleanup. Field order inside the struct does not affect the result, because every field is resolved independently against the same configuration.

**Error behaviour stays within the existing vocabulary.** The decoder adds exactly one new condition, the invalid target, and expresses it in the established sentinel-and-detail form beside the other lookup errors. Every other failure is passed through from the shared cores untouched. That includes the not-unique error, so a caller who handles `FindOne`'s errors already handles `Decode`'s. A field whose type the registry cannot reach is a deliberate no-op rather than a `NotRegistered` error. That is what lets an application struct carry its own settings, plugin-typed pointers or nested-only block types alongside its configuration.

**Shape of the developer experience.** On the consumer side, the configuration-only example shows the intended use: one application struct declared beside the registered types, one `Decode` call after `Apply`, and the rest of the program working from that struct's fields. The method form is presented as the primary spelling everywhere, including in runnable examples. This is the one place the "examples use the function form" rule does not apply, because `Decode` is not generic and so works on the oldest supported toolchain. The README states that exception explicitly so the rule stays easy to follow.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references (`design ref list` reports zero refs, zero unresolved).
- **`reflect` (Go standard library)** — powers the type → address derivation, the field walk and field assignment. No change; no new module in `go.mod`.
- **`errors` (Go standard library)** — sentinel and `errors.Is`/`errors.As` matching for the new invalid-target error. No change.
- **Plugin registry (`plugins/registry`, internal to `xclconfig`)** — answers which address a `reflect.Type` is registered under, and reports plugin-provided types as unreachable. Used as is, no change.
- **Shared errors package (`errors`, internal to `xclconfig`)** — home of the lookup sentinels and detail types. It gains the invalid-decode-target sentinel and detail. Its import list is not widened (still only `internal/xcl` and `go-wordwrap`).
- **Entity metadata (`types`)** — `GetMeta` decides whether a Go type is addressable, and supplies each entity's type and subtype for the scan. Used as is, no change.
- **Parser state ordering (`internal/parser`)** — entities are held in the order they were parsed, which is what gives `Decode` (and `All`) declaration order. Relied on, not changed.
- **Prior plan `20260921093100-query-api-v2` (landed)** — built `Find`/`FindByType`/`FindOne`/`All`, `TypePath(reflect.Type)` and the one-implementation-two-spellings rule this plan extends. It is already in the codebase, so nothing has to land first.
- **Prior spec `20260919120639-config-only-types-and-examples` (landed)** — introduced `RegisterType` and the `example/configonly` program that becomes the `Decode` showcase. Already in the codebase.
- **`xcl-website` repo** — hosts the configuration-only example page that is updated. It has no code dependency on `xclconfig`: its snippets are copied text, so the site change can land independently of a library release. There are no content tests; the site's own build and type-check are its only gate.
- **CI (`.github/workflows/go.yml`)** — the Go 1.27 job runs every test, and the `build-minimum-go` job builds and vets on go1.25.0. `Decode` must compile there, which the non-generic signature guarantees. No workflow change.

## Testing Approach

**Test types.** Nearly all coverage is unit-level, in package `xcl`, run against real applied configurations. Each test registers fixture types on a fresh registry, `Apply`s an `.xcl` fixture against a temporary file state store, and then calls `Decode`. This is the same harness shape the existing lookup tests use, and it follows the convention of producing state with a real apply. Above that sit three other layers:
- Regression coverage: the existing lookup suite, unchanged, guards the core refactor.
- Static and content guards: the example's AST guard and the README/CHANGELOG content tests.
- The example program's own output tests, which act as a small end-to-end check.

Every test uses testify `require`, there are no table-driven tests, and every positive case is a separate test function from every negative case.

**Where coverage concentrates.**
- **The shared cores (regression first).** The refactor of the type-path derivation, kind scan and conversion lands before `Decode`. The whole existing lookup suite must stay green without edits: address, kind, Go-type, error-vocabulary, migration and Go 1.27 method/function equivalence tests. That suite is the proof that the typed API's behaviour did not move. No new tests are needed for the cores themselves, because they are exercised entirely through the existing typed wrappers and through `Decode`.
- **The decoder (most new tests).** One test per acceptance criterion, plus one per success metric. The load-bearing assertions, in plain language:
  - Collection fields receive every entity of their type with its configured values.
  - Collection fields come back in declaration order, and reversing the declarations reverses the result.
  - Each collection field is element-for-element identical to `All` for the same type, including a disabled block, and the elements are the same pointers.
  - A decoded entity is the very instance `Find` returns by address, so a change through one is visible through the other.
  - A single field is set when exactly one entity exists, and left nil when none exists.
  - A single field fails with an error that `errors.Is` matches to `ErrNotUnique`, whose detail reports a count of 2. The error equals what `FindOne` returns, and the target is left unchanged.
  - Renaming a field changes nothing.
  - A plain value field and an unregistered-struct field survive untouched.
  - A by-value struct, a nil pointer and a pointer to a non-struct are each rejected with `ErrInvalidDecodeTarget`, leaving both the target and the configuration's entities unchanged. These are three separate negative tests.
  - Before any `Apply`, the call succeeds with empty collections and nil singles.
  - The method and function forms return equal structs and equal errors.
  - A nested block that references a registered non-`resource` entity as a whole arrives filled, through both a pointer field and a value field.
- **Documentation.** The README content tests are extended to require the `Decode` section and call. The CHANGELOG content test requires the new entry, including `ErrInvalidDecodeTarget`. The docs-site page has no automated content tests; it is checked by the site's own build/type-check, and its correctness is reviewed by eye.

**Success metrics → verification.**
- *Filling an application structure from a configuration takes one call, however many registered block types the structure holds.* **Behavioural test.** A struct with fields for three registered types, some collections and some singles, is filled by exactly one `Decode` call. Each field is asserted against the corresponding `All`/`FindOne` result.
- *Adding a new registered block type needs only a new field on its structure, with no new lookup code.* **Behavioural test.** Against a configuration that declares two registered types, a struct holding a field for only one of them and a second struct that adds a field for the other are each filled. The same single `Decode` call, with no other code change, fills the new field.
- *Every shipped example program that assembles configuration from several block types does so with the new call.* **Behavioural test (static guard).** The config-only example gains AST guards that require exactly one `Decode` call and forbid any `Find`/`FindByType`/`FindOne`/`All` call in the program. Its service and ingress become single-pointer fields, so no lookup is left. The existing portable-lookup guard keeps rejecting method-form lookups, and it drops its "at least one lookup" assertion. The example's existing output tests prove the printed result is unchanged. The plugin example is excluded deliberately: its types are plugin-provided and cannot be decoded (see the assumptions log). The application-config example assembles only one type.

**Deliberate gaps.**
- No mocks are introduced. `Decode` has no interfaces of its own to mock, and real applies are both cheap and the convention.
- No Go 1.25 runtime test is added. The minimum-version CI job builds and vets, and the non-generic signature is what makes `Decode` compile there; no build-tagged test file is needed.
- No fuzzing of arbitrary struct shapes. The two accepted shapes and the leave-alone rule are covered by explicit cases.

## Milestones & Tasks

### Milestone 1: The typed lookups run on one shared core, and whole-block references are proven

**What changes**: This is mostly an internal cleanup with no change in what any existing lookup returns. Today the rules for which entities match a Go type, and when an entity comes back as the configuration's own instance, exist only in generic code that is tied to a compile-time type. Those rules move into one shared core that works from a runtime type. `Find`, `FindByType`, `FindOne`, `All` and `As` become thin typed layers over it. This deserves its own milestone because it is what lets `Decode` reuse the lookups instead of duplicating them, which is the only way the spec's "can never disagree with `All`" can hold by construction. Landing it alone means the full existing lookup suite proves the refactor before anything is built on it.

The milestone also adds the missing coverage the spec flagged as a known risk. A nested block that references another registered, non-`resource` block as a whole now has tests showing that it arrives with that block's values, whether the receiving field holds the type directly or as a pointer.

**Validation point**: The entire existing test suite passes unchanged, including the Go 1.27 method/function equivalence tests. The new whole-block reference tests pass. The minimum-version build and vet on Go 1.25 succeed.

### Milestone 2: Applications fill their own configuration struct with one `Decode` call

**What changes**: A developer can declare a struct of their own, with a `[]*T` field for each registered block type they want every instance of and a `*T` field for each type they expect exactly one of, and fill it with a single `c.Decode(&cfg)` or `xcl.Decode(c, &cfg)` after `Apply`. Each collection field receives exactly what `All` would return for its type, in declaration order and as the configuration's own instances. Each single field is set when its type is declared once, left nil when it is not declared, and fails with the familiar "not unique" error when it is declared more than once. Fields of any other type keep their values. Passing something that is not a non-nil pointer to a struct fails with a new, matchable invalid-target error. A failed call never leaves the struct half-filled. The call works on every supported Go version.

**Validation point**: Every spec acceptance criterion except the documentation one has a passing test, as do the first two success metrics. `errors.Is(err, xcl.ErrInvalidDecodeTarget)` and `errors.Is(err, xcl.ErrNotUnique)` match the documented failures. The Go 1.25 build and vet job still succeeds.

### Milestone 3: The README, changelog, config-only example and docs site show `Decode`

**What changes**: A developer reading the project's documentation finds `Decode` alongside the other lookups. The README explains its contract and why, unlike the generic lookups, its method form works on Go 1.25. The changelog records the new call and its error. The config-only example program gathers all its registered blocks into one application struct with `Decode` instead of fetching each type separately, while printing exactly what it printed before. The docs site's configuration-only example page walks through the same configuration and the `Decode` call that fills a struct from it.

**Validation point**:
- The README and changelog content tests pass.
- The config-only example's output tests pass unchanged, and its extended guard confirms that it assembles its configuration with `Decode` and no per-type collection lookups.
- Running the example prints the same output as before.
- The docs site builds and type-checks cleanly.

### Milestone 1 tasks

#### - [x] Task: Move the typed lookups onto a shared reflective core
**Id:** 509ce57b-fa4b-4dd2-b032-2ff1fe369683
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Today the rules behind `Find`, `FindByType`, `FindOne`, `All` and `As` are written only in generic code, so nothing can use them with a type that is known only at run time. This task moves four rules into non-generic helpers that take a runtime type:
- which types have an address of their own
- which address a registered Go type is reached by
- which entities match an address
- when an entity is returned as the configuration's own instance

The existing generic functions become thin typed wrappers around those helpers. Nothing a caller can observe changes. The work exists so that `Decode` can reuse these rules rather than copy them.

*Technical detail:* [context.md#task-move-the-typed-lookups-onto-a-shared-reflective-core](./context.md#task-move-the-typed-lookups-onto-a-shared-reflective-core)

**Acceptance criteria**:
- [x] Every existing lookup returns exactly what it returned before, for the same inputs, including the same errors.
- [x] Each rule (addressable type, type to address, matching scan, identity-or-copy conversion) is written once, and the generic lookups only delegate to it.
- [x] The library still builds and vets on the oldest supported Go version.

#### - [x] Task: Cover whole-block references to registered blocks
**Id:** 18e3ece9-2ae6-423e-9b4a-b647d071f08b
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

The spec flagged a known risk: no test covers a nested block that references a registered block declared by its own keyword, rather than as a `resource`, as a whole. Nor is there coverage for that reference landing in a pointer field. This task adds a small type, local to the test file, whose nested blocks reference the existing registered cache type as a whole, along with one fixture that declares it. Tests show that the referenced values arrive filled in for both field forms. Nothing is added to the shared fixture types. A spike during planning showed this already works, so this task adds coverage. Code changes are needed only if the tests fail.

*Technical detail:* [context.md#task-cover-whole-block-references-to-registered-blocks](./context.md#task-cover-whole-block-references-to-registered-blocks)

**Acceptance criteria**:
- [x] A nested block that references a registered cache through a pointer field holds that cache's configured values.
- [x] A nested block that references a registered cache through a value field holds that cache's configured values.
- [x] The shared registered fixture types are unchanged.

### Milestone 2 tasks

#### - [x] Task: Add the invalid decode target error
**Id:** 443df64d-03a7-483a-b8f6-c18601576709
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

`Decode` must reject a target it cannot fill: a struct passed by value, a nil pointer, or a pointer to something that is not a struct. The rejection must be an error callers can match. This task adds that error to the shared errors package, using the project's sentinel-and-detail form with the detail naming what was passed, and re-exports it from `xcl` beside the other lookup errors.

*Technical detail:* [context.md#task-add-the-invalid-decode-target-error](./context.md#task-add-the-invalid-decode-target-error)

**Acceptance criteria**:
- [x] `errors.Is(err, xcl.ErrInvalidDecodeTarget)` matches the new error, and `errors.As` recovers a detail that names the type that was passed, or says that nothing was passed.
- [x] The error's message tells the caller what was expected and what they passed.
- [x] The shared errors package's import list is unchanged.

#### - [x] Task: Implement Decode with its tests
**Id:** 270a8e82-b0e4-41ad-bcb1-6def5890547e
**Repo:** xclconfig
**Depends on:**
- 509ce57b-fa4b-4dd2-b032-2ff1fe369683 — Move the typed lookups onto a shared reflective core
- 18e3ece9-2ae6-423e-9b4a-b647d071f08b — Cover whole-block references to registered blocks
- 443df64d-03a7-483a-b8f6-c18601576709 — Add the invalid decode target error
**Execution:** agent

This task adds the method `c.Decode(&cfg)` and the function `xcl.Decode(c, &cfg)`, which both use one implementation. That implementation checks the target, works out which fields hold a registered type, and resolves every such field through the shared lookup core before assigning any of them:
- A collection field gets what `All` returns for its type.
- A single field gets the one entity, or nil when there is none, or fails with the not-unique error when there are several.

Every other field is left alone. Tests cover each acceptance criterion in the spec and the first two success metrics. They reuse the existing registered test types and fixtures, and add only two small fixture files that declare the same blocks in opposite orders.

*Technical detail:* [context.md#task-implement-decode-with-its-tests](./context.md#task-implement-decode-with-its-tests)

**Acceptance criteria**:
- [x] Collection fields hold every block of their type, with configured values, in declaration order, element-for-element the same instances `All` returns, including a disabled block.
- [x] A block taken from the filled struct is the same instance a lookup by its address returns.
- [x] Single fields work as follows:
  - A single field is set when its type is declared once.
  - It is left unset when the type is never declared.
  - When the type is declared twice, the call fails with the same "not unique" error `FindOne` gives, reporting a count of 2, and the struct is left unchanged.
- [x] Renaming a field changes nothing, and plain or unregistered fields keep their earlier values.
- [x] A struct passed by value, a nil pointer and a pointer to a non-struct are each rejected with the invalid-target error, and neither the target nor the configuration changes.
- [x] Before any `Apply`, the call succeeds with empty collections and unset single fields.
- [x] The method and function forms give equal structs and equal errors.
- [x] Whole-block references inside a decoded block arrive filled in, through both pointer and value fields.
- [x] One call fills a struct holding three registered types, and adding a field for a fourth needs no other code.

### Milestone 3 tasks

#### - [x] Task: Assemble the config-only example with Decode
**Id:** e02f6d0e-b4d2-46d3-abe2-5db07c90d8c0
**Repo:** xclconfig
**Depends on:**
- 270a8e82-b0e4-41ad-bcb1-6def5890547e — Implement Decode with its tests
**Execution:** agent

The config-only example currently fetches each type it prints with a separate lookup. This task gives it one application struct, holding every config map and deployment plus the service and the ingress, filled by a single `Decode` call after `Apply`. The printing code then works from that struct. The example's static guard is updated to require that assembly and to forbid per-type collection lookups, so it keeps guarding the success metric.

*Technical detail:* [context.md#task-assemble-the-config-only-example-with-decode](./context.md#task-assemble-the-config-only-example-with-decode)

**Acceptance criteria**:
- [x] The example fills one application struct with a single `Decode` call and makes no per-type lookups to assemble it.
- [x] The example prints exactly what it printed before, and all its existing tests pass unchanged.
- [x] The example's guard fails if a future edit goes back to per-type lookups, or uses a lookup's Go 1.27-only method form.

#### - [x] Task: Document Decode in the README and changelog
**Id:** fa6f988e-727f-4de4-a932-0fe42bbb612c
**Repo:** xclconfig
**Depends on:**
- 270a8e82-b0e4-41ad-bcb1-6def5890547e — Implement Decode with its tests
**Execution:** agent

This task adds a `Decode` section to the README's querying documentation. The section uses a small configuration and struct as its worked example and states the contract: which field shapes are filled, the empty, single and not-unique outcomes, the invalid-target error, and that other fields are untouched. It also explains that, because `Decode` is not generic, its method form works on Go 1.25. The configuration-only section points to it, and the changelog gets an entry for the feature. The README content tests are extended so the documentation cannot silently drift away.

*Technical detail:* [context.md#task-document-decode-in-the-readme-and-changelog](./context.md#task-document-decode-in-the-readme-and-changelog)

**Acceptance criteria**:
- [x] The README shows a configuration and the one call that fills a struct from it, and explains every outcome a caller can get.
- [x] The README says why `Decode`, unlike the generic lookups, can be called as a method on every supported Go version.
- [x] The changelog records `Decode` and its new error.
- [x] Content tests fail if either the README section or the changelog entry is removed.

#### - [x] Task: Show Decode on the docs site configuration-only page
**Id:** 31ed1d1e-810d-4d74-a2f7-e100ceb21eb5
**Repo:** xcl-website
**Depends on:**
- e02f6d0e-b4d2-46d3-abe2-5db07c90d8c0 — Assemble the config-only example with Decode
**Execution:** agent

The docs site's configuration-only example page walks through the example program. This task updates the page's program section to show the application struct and the `Decode` call from the rewritten example. In that section, the Go snippets that have drifted from the repository are brought back in line, so the page shows code that compiles. The page's prose about lookups is updated to introduce `Decode` alongside `Find`.

*Technical detail:* [context.md#task-show-decode-on-the-docs-site-configuration-only-page](./context.md#task-show-decode-on-the-docs-site-configuration-only-page)

**Acceptance criteria**:
- [x] The configuration-only page shows a configuration and the `Decode` call that fills a struct from it.
- [x] The Go snippets in the page's program section match the current example program.
- [x] The site builds and type-checks without errors.

## Open Questions

- **Does any existing test depend on error precedence or on result nil-ness in a way the core refactor subtly shifts?** This depends on the refactor exposing a hidden assumption in an untested path. The plan keeps the check order and the non-nil empty results exactly as they are, but only running the full suite against the refactored code proves it. **If any existing lookup test fails after the refactor, STOP and ask the user.** Do not edit the test to fit; the refactor is meant to be behaviour-preserving.
- **Does a registered type ever reach `c.Entities()` as something other than a pointer to its prototype type?** This depends on parser paths that only real applies exercise, for example entities restored from saved state after a removed-resource destroy. If a `Decode` or `All` identity test (`require.Same`) fails because an entity arrived as a copy, then "the configuration's own blocks, not copies" cannot hold through the shared conversion alone. **STOP and ask the user** before changing the conversion rule.

No other questions remain open. Everything else was settled during planning: the API shape, the error vocabulary, the field-shape rules, the example rewrite, the website scope, and the reference-risk spike.

## Out of Scope

- **Recursing into nested aggregate structs.** A field that is itself a struct to be filled is left alone, not decoded into (spec Non-Goal).
- **Selecting blocks by subtype or by module.** Each field receives every entity of its type, including those declared inside modules, exactly as `All` does (spec Non-Goal). Callers who want a subset keep using `FindByType`.
- **Writing a filled struct back out as configuration text, or saving it to state** (spec Non-Goal). The existing `EncodeEntity` remains the way to produce text from an entity.
- **Plugin-provided types.** A `[]*T`/`*T` field whose `T` is a plugin type is left untouched, because the registry holds no Go type for it. As a result the plugin example program is not converted to `Decode`. Extending `Decode` to plugin types would need a way to bind a Go type to a plugin schema, which no spec currently covers.
- **Struct tags, field-name matching or other annotations** (spec constraint).
- **Fields of any shape other than `[]*T` and `*T`**, such as `[]T`, `T`, maps or interfaces (spec constraint).
- **A site-wide refresh of stale docs-site snippets.** Only the configuration-only page's program section is brought in line. Stale snippets on `index.mdx` (old `RegisterType` argument order) and elsewhere are not touched.
- **Documenting `Decode` on other docs-site pages** (home page feature card, application-config and plugins pages). The spec names only the README and the configuration-only page.
- **Cleaning up the `state` package's vestigial query methods** recorded in the `config-is-the-public-query-surface` knowledge entry. This plan does not touch it.

## Changelog

### 2026-10-03 — Task: Move the typed lookups onto a shared reflective core

**What was done**: The lookup rules in `query.go` now live in non-generic helpers over `reflect.Type`: `asType` (identity, refusal or copy), `convertibleTo`, `addressableType`, `Config.entitiesOf` (the kind scan), `oneOf` (the single-result pick and `NotUniqueError`) and `Config.typePath` (addressable check plus registry lookup). `As`, `findByType`, `findOne` and `all` are thin typed wrappers that cast the `any` results back to `*T`. No test was edited, and the full suite plus the Go 1.25 build and vet pass.

**Deviations**: None. The task added no new tests, as the plan specified, so the test step wrote nothing.

**Files changed**:
- `xclconfig: query.go`

**Discoveries**: The identity check in `asType` compares `reflect.TypeOf(entity) == reflect.PointerTo(want)`. For a concrete `*T` that is exactly what the old `entity.(*T)` assertion matched. `find[T]` still calls `As[T]` directly, including for published output values, so it needs no reflective form.

### 2026-10-03 — Task: Cover whole-block references to registered blocks

**What was done**: A new test file covers a nested block that references a registered bare-keyword cache as a whole. It declares test-local `cacheClient`, `cacheLink` (pointer field) and `cacheCopy` (value field) types and a `setupCacheClientConfig` helper that applies a new `cache_client` fixture. Both the pointer form and the value form arrive with the referenced cache's `location` and `Meta.ID` filled in.

**Deviations**: None. No code fix was needed, as the planning spike predicted.

**Files changed**:
- `xclconfig: query_references_test.go`
- `xclconfig: internal/test_fixtures/config/registered/cache_client/main.xcl`

**Discoveries**: The value form (`registered.Cache`, not a pointer) also has `Meta.ID` populated by the reference. `cacheClient` and `setupCacheClientConfig` are package-scope test helpers that the Decode tests reuse.

### 2026-10-03 — Task: Add the invalid decode target error

**What was done**: Added the `ErrInvalidDecodeTarget` sentinel and the `*InvalidDecodeTargetError{Type, Nil}` detail to the shared errors package. The detail's message reads "decode target must be a non-nil pointer to a struct, got <what>", and the detail unwraps to the sentinel. Both are re-exported from `xcl` beside the other lookup errors, and the comments that count the sentinels were updated in both places. Tests cover the unwrap, recovery through a wrap and all three message forms. The root package's re-export tests gained one line each.

**Deviations**: None. The plan's struct-only sketch `{Type}` was superseded within the plan itself by `{Type, Nil}`, which is what was built.

**Files changed**:
- `xclconfig: errors/query_errors.go`
- `xclconfig: errors/query_errors_test.go`
- `xclconfig: config.go`
- `xclconfig: query_errors_test.go`

**Discoveries**: The root `query_errors_test.go` enumerates every re-exported sentinel and detail type, so any future lookup error must be added there as well.

### 2026-10-03 — Task: Implement Decode with its tests

**What was done**: Added `decode.go` with `xcl.Decode(c, &cfg)` and `c.Decode(&cfg)`. Both delegate to one two-phase `decode`. It validates the target, resolves every exported `[]*T`/`*T` field through the shared `typePath`, `entitiesOf` and `oneOf` cores, and only then assigns. `decode_test.go` covers every acceptance criterion and success metrics 1 and 2 in 28 tests. Two ordering fixtures (`registered/ordered/forward` and `registered/ordered/reversed`) were added.

**Deviations**: Declaration order did not hold, and the plan had assumed it did. The parser's working storage holds entities in a map keyed by address, and `parseAndValidate` copied them into the state by ranging over that map. As a result `c.Entities()`, and with it `All`, `FindByType` and `Decode`, came back in random order, and the two order tests failed about 1 run in 10. The planning spike had passed by chance. With the user's agreement ("ordering is probably useful, I feel we might need that when we do diff"), `internal/parser/parser.go` now records first-seen declaration order in `parsed.order`, through a new `parsed.store` helper, and builds the state from that order. This is outside the task's planned file list.

**Files changed**:
- `xclconfig: decode.go`
- `xclconfig: decode_test.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/test_fixtures/config/registered/ordered/forward/main.xcl`
- `xclconfig: internal/test_fixtures/config/registered/ordered/reversed/main.xcl`

**Discoveries**: Entity order across every lookup and in the saved state is now declaration order by construction, through `parsed.order`. Before this change it was random. Any code that adds to `parsedResources` must go through `parsed.store`, or the entity is silently dropped from the state. The research note claiming that the spike confirmed declaration order was wrong. Identity holds: every registered entity arrives as the configuration's own `*T`.

### 2026-10-03 — Task: Assemble the config-only example with Decode

**What was done**: The config-only example now declares an `appConfig` struct with fields `ConfigMaps`, `Deployments`, `Service` and `Ingress`. It fills the struct with a single `c.Decode(&cfg)` after `Apply`. `printDeployments` and `printRouting` take the decoded fields rather than looking anything up, and the program's output is byte-identical to before. Two new AST guards were added: one requires exactly one `Decode` call, the other forbids any `Find`/`FindByType`/`FindOne`/`All` call. The portable-lookup guard lost its "at least one lookup" assertion.

**Deviations**: `printRouting` now returns an error naming the missing block when the service or ingress is nil. Previously the error came from `Find`'s not-found error. No output for the shipped configuration changed.

**Files changed**:
- `xclconfig: example/configonly/main.go`
- `xclconfig: example/configonly/main_test.go`

**Discoveries**: `static_examples_test.go` makes no lookup-related assertions, so it needed no change.

### 2026-10-03 — Task: Document Decode in the README and changelog

**What was done**: The README gained a `#### Filling a struct of your own` subsection under "Querying a configuration". It works through the server/mount example, showing the HCL, an application struct and `c.Decode(&cfg)`, and states the full contract. The error vocabulary now lists `ErrInvalidDecodeTarget`. "Two spellings" explains that `Decode` is not generic, so its method form works on Go 1.25. The "Configuration only" section shows the example's `appConfig` and its `Decode` call. CHANGELOG.md has a new top entry. `readme_test.go` has three new content tests.

**Deviations**: The CHANGELOG entry also records the declaration-order change made in the parser during the Decode task, because it is user-visible: lookups, `c.Entities()` and saved state now follow declaration order. The README's worked example leaves out a `*Server` field, because the two declared servers would make it fail. The not-unique outcome is described in prose instead.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: CHANGELOG.md`
- `xclconfig: readme_test.go`

**Discoveries**: None.

### 2026-10-03 — Task: Show Decode on the docs site configuration-only page

**What was done**: The program section of the configuration-only page is back in line with `example/configonly/main.go`:
- the `main` call, which builds the registry and passes it to `run`
- the current `run` signature
- `RegisterType(&T{}, "resource", "<name>")` argument order
- `WithStatePath(stateDir)` and `WithEventData`

The old `FindByType` snippet is replaced by the `appConfig` struct and its `c.Decode(&cfg)` call. A "What to notice" bullet now introduces gathering the configuration with `Decode`. `npx astro check` reports 0 errors and `npm run build` succeeds.

**Deviations**: The page's sample `## Resources` output was reordered to the program's real output, with variables first in declaration order. The parser change in the Decode task made that order deterministic. The output block is outside the program section the plan scoped. Site dependencies were installed with `npm ci` from the lockfile in order to run the check; they are gitignored.

**Files changed**:
- `xcl-website: src/pages/examples/configuration-only.mdx`

**Discoveries**: The website repo had no `node_modules`, so `make check` stops at an interactive `npx` install prompt (it exits 0 without checking anything). `npm ci` must be run first.
