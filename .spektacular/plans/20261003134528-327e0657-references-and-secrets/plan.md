---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Plan: 20261003134528-327e0657-references-and-secrets

<!-- Metadata -->
<!-- Created: 2026-10-05T10:02:12Z -->
<!-- Commit: d554c1d -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Passwords and other secrets in an xcl configuration currently appear in plain text wherever xcl or the application prints, logs or reports them. This plan adds `types.Sensitive[T]`, a field type that type authors use to declare a value sensitive, for application and plugin types alike. Sensitivity is carried through references, interpolation, functions, module inputs and outputs as a cty mark, and assigning a sensitive value to a plain field is rejected at validation and at Go conversion. Logs, events, errors, configuration text and printed resources show only `(sensitive)`. State and plugins keep the real value, and application code reaches it only with an explicit `Reveal()`. Application developers, plugin authors and their users stop leaking secrets by accident. The examples, the library docs and the documentation site are brought into line.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, never mix positive and negative cases in one test, favour verbosity** — every redaction path, conversion guard and validation rule gets its own named accepted test and its own rejected test.
- **Generate test state with a real apply, not a hand-written state file** — the state round-trip, reload and output tests run a real `Apply` and load the state it wrote.
- **Code style: standard Go conventions, `any` over `interface{}`, descriptive names** — applies to `types/sensitive.go`, `internal/wire` and the new validation stage.
- **Shared errors live in the `errors` package; sentinel-and-detail convention** — the field-level type error extends the existing `TypeMismatchError` there, keeping `ErrTypeMismatch` as the sentinel, rather than adding a new error type elsewhere.
- **NEVER modify dependency packages; `internal/xcl` (MPL) and `internal/cty` are in-repo copies whose changes are recorded in `UPSTREAM.md`** — the `gocty` wrapper hook and the `gohcl` encoder option are generic library changes with licence headers kept and changes recorded, and no xcl-specific code enters the MPL fork.
- **Project structure: `/internal` is private** — the revealing encoder is `internal/wire`, so no public API exposes real values except `Reveal()`.
- **Dependencies: prefer the standard library** — redaction uses only `fmt`, `log/slog`, `encoding` and `encoding/json` interfaces; no new module dependency.
- **Development standards: structured logs** — the redaction is proven through `log/slog` (`LogValuer`), the event stream and the slog bridge.

## Architecture & Design Decisions

All Go work lands in the `xclconfig` repo. The documentation site page and its updated example output land in `xcl-website`.

**A sensitive value is a self-protecting Go type, carried through configuration as a cty mark.** `types.Sensitive[T]` holds its real value in an unexported field. Every display interface Go offers writes the fixed marker `(sensitive)`: `String`, `GoString`, `Format` for every verb, `slog.LogValuer`, `MarshalText` and `MarshalJSON`. So loggers, error formatting and JSON that xcl does not own show only the marker. Only `Reveal()` returns the real value. The type lives in `types`, beside `ResourceBase` and `types.Output`, following `architecture/shared-public-types-live-in-types.md`. Inside configuration, sensitivity is a cty mark. When `gocty` turns an entity into a cty value for the evaluation context (`internal/parser/context.go:93`), each `Sensitive[T]` field becomes `T`'s cty value carrying the mark. When `gocty` decodes a value back into Go, a marked value goes into a `Sensitive[T]` field unwrapped and rewrapped, and into any other field fails with an error naming the field. cty already carries marks through references, templates, conditionals and functions with known arguments. Output `CtyValue`s and module inputs keep them too. So one mechanism covers references, interpolation, functions and module outputs, as the spec's technical approach directs. The hook is generic: `types` registers a wrapper recogniser with `gocty`, so neither copied library imports xcl code and no xcl-specific shaping enters the MPL-licensed `internal/xcl` (`conventions/never-modify-dependencies.md`, `gotchas/xcl-tags-gate-what-reaches-cty.md`). Each change is recorded in the library's `UPSTREAM.md`.

**Validation predicts sensitivity statically, and decoding enforces it again at walk time.** cty drops marks when a function gets an unknown argument (`gotchas/cty-unknown-args-drop-marks.md`), so validation does not evaluate anything. A new stage after reference resolution walks each entity's stored body alongside its Go field types, the way `validateStructure` already does (`internal/parser/validate.go:221-346`). For each attribute it collects every traversal with `hclsyntax.Variables` and resolves each one. It then asks whether the reference reaches a sensitive source, where a source is one of the following:
- a `Sensitive[T]` field, found by walking the target's Go type along the attribute path;
- an output whose own value expression is sensitive, judged recursively, including a child module's output;
- a module input fed a sensitive value.

A bare reference keeps the exact sensitive paths of what it reaches. Any expression that combines values, such as a template, a function call or an operator, is wholly sensitive if any input is. Object and tuple constructors are checked element by element against the field's type. A sensitive part landing in a field that is not `Sensitive[T]`, `cty.Value` or `any` (outputs and variables hold dynamic values) is a positioned `ParserError` naming the entity and the field. It is collected into `ConfigError` like every other validation problem, so `Validate` and `Apply` both refuse it before anything is created. The `gocty` guard is the safety net: if a marked value ever reaches a plain field at walk time, it fails with an error naming the field instead of panicking, as it does today.

**State and plugins see real values through one internal encoder; everything else sees the marker.** Every internal xcl hop serialises with `encoding/json` (`gotchas/custom-marshaljson-changes-internal-hops.md`). A new internal package, `internal/wire`, walks a value, writes each `Sensitive[T]` as its real value, and otherwise follows `encoding/json`'s tag rules and delegates to it for anything holding no sensitive value. The hops that must keep the real value switch to it:
- provider calls in both directions;
- change detection and the configured-value check;
- the host state callback;
- query conversion.

`Sensitive[T].UnmarshalJSON` accepts the real `T`, so reading needs no special decoder. The two state save sites (`config.go:315`, `internal/parser/destroy.go:130`) encode each entity before handing it to the store as `json.RawMessage`. A custom `StateStore` therefore stores real values without access to the internal encoder. These are also the points the later masking spec hooks into (`learnings/state-save-and-load-points.md`). Events take the opposite path. Event data is always re-encoded from the typed entity with plain `encoding/json`, so it shows the marker, and the provider-call bytes are never reused. Making event masking pluggable belongs to the masking spec. Plugin types are rebuilt on the host with `reflect.StructOf`, so the host's type map gains a fixed set of `Sensitive` instantiations, and an unsupported one fails plugin loading by name (`gotchas/plugin-types-rebuilt-with-structof.md`).

**Application code gets wrapped values, and xcl's own output redacts unless asked.** Lookups, listing and `Decode` hand back entities with their `Sensitive[T]` fields intact. Converting to a Go type whose matching field is plain fails before any copy, with `ErrTypeMismatch` naming the field (`query.go:80-107`). Converting to a type whose field is sensitive succeeds through the revealing encoder. There is no option to unwrap automatically. An output's Go value wraps exactly its sensitive parts, such as a `Sensitive[string]` leaf inside a map, or a whole `Sensitive[...]` value. It records those paths in state so a reloaded output wraps them again. The configuration-text encoder and the resource printers recognise `types.SensitiveValue` explicitly rather than walking into it, as the spec's technical approach directs. Both show the marker by default and the real value only through `xcl.RevealSensitive()` and `logger.WithRevealSensitive(true)`. xcl's function wrappers replace any sensitive argument's text in a function's error message with the marker. The examples declare their passwords sensitive and unwrap them where they use them, and the docs and site describe all of this.

Rejected directions: a global reveal switch, json/v2 marshalers, a `StructOf` parallel type, a capsule type, evaluating with unknown placeholders, and teaching the copied libraries about `types.Sensitive` directly. The evidence for each is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Sensitive value type (new, public `types`)**: `Sensitive[T]`, its constructor and the fixed marker. It owns the guarantee that every Go formatting and marshalling path shows only the marker, and that `Reveal()` is the only way to the real value. A sealed interface, `SensitiveValue`, lets xcl's own encoders and printers recognise any instantiation and reveal it generically. Application code cannot implement it. At package initialisation, the type registers itself with the cty conversion hook.
- **cty conversion wrapper hook (changed, copied `gocty` library)**: a generic extension point for Go types that wrap one inner value and carry a mark. It works in three places. Type implication uses the inner type. Conversion to cty converts the inner value and applies the mark. Conversion from cty removes the mark and wraps the result. Separately, a marked value that meets an ordinary Go target becomes a path error rather than a panic. It knows nothing about xcl. The sensitive value type is its only registered user.
- **Evaluation context and walk-time decode (changed, parser)**: these are unchanged in structure. They gain sensitivity through the hook: referenced entities carry marks into expressions, and decoding a body fills `Sensitive[T]` fields from marked or plain values. A marked value reaching a plain field fails the step with an error naming the field.
- **Sensitive-assignment validation stage (new, parser validation)**: it runs after references resolve and before properties are checked. It walks each entity's stored body alongside its Go field types. For each expression it predicts, without evaluating anything, which parts are sensitive. The sources it follows are sensitive fields, outputs (recursively, across a child module's outputs) and module inputs. It reports any sensitive part that lands in a plain field as a positioned validation problem naming the entity and the field.
- **Output value conversion (changed, parser)**: turns an output's evaluated value into its Go `Value`. Exactly the marked parts become sensitive values: a sensitive scalar, list or map, or sensitive leaves inside a map or list. The paths of those parts are recorded on the output so state can wrap them again on reload.
- **Function error redaction (changed, parser function set)**: wraps each function the parser offers. When a call that received a sensitive argument fails, the error text has that argument's value replaced by the marker. Results keep their marks.
- **Revealing wire encoder (new, `internal/wire`)**: the one place that serialises an entity with real sensitive values. It follows `encoding/json`'s field rules and writes each sensitive value revealed. The callers are provider calls in both directions, change detection, the configured-value check, the host state callback, query conversion and the two state save sites.
- **State save (changed)**: after Apply and during Destroy, each entity is encoded by the wire encoder and handed to the store as raw JSON. Every store, built-in or custom, holds real values. Loading is unchanged, because the sensitive type reads its real value back from JSON.
- **Event data (changed, parser events)**: event data is always produced from the typed entity through standard JSON, so sensitive fields show the marker at both data levels. Provider-call bytes are no longer reused for events. Processed event data can still be read by `EncodeSavedEntity`, which then shows the marker.
- **Plugin type rebuild (changed, registry and schema)**: the host's type map lists the supported `Sensitive` instantiations, shared with the plugin test helpers. The schema type parser reads nested brackets correctly. A plugin type using an unsupported instantiation fails loading with an error naming the type and field.
- **Typed query conversion (changed, root package)**: before copying an entity into a different Go type, it compares the two types field by field. A sensitive source field meeting a plain target field fails with `ErrTypeMismatch` naming the field. Allowed conversions copy through the wire encoder, so sensitive-to-sensitive keeps the real value.
- **Configuration-text encoder (changed, root `encode` plus copied `gohcl` encoder)**: the `gohcl` encoder gains a generic option for marked values, write them revealed or replace each one with a literal the caller supplies. The root encoder passes the marker by default, and the real value under `RevealSensitive()`.
- **Resource printer (changed, `logger`)**: the table, tree and card formats recognise sensitive values and print the marker. The JSON format redacts through standard JSON. `WithRevealSensitive(true)` shows real values in all four, the JSON one through the wire encoder.
- **Configured-value and computed walks (changed, parser)**: they treat a sensitive value as a leaf, never as a nested block. So a changed sensitive value is compared, and it is copied as a whole.
- **Bundled examples (changed)**: the application-config example and the plugin example declare their passwords as `Sensitive[string]`. Where the password is actually used, it is unwrapped with `Reveal()`, never logged or printed. Their tests assert that no secret appears in any output.
- **Library documentation (changed)**: covers the README, the plugin developer guide, the state guide, the changelog and the README/CHANGELOG content tests.
- **Documentation site (changed, `xcl-website`)**: a new Sensitive values guide page linked from the navigation. The example pages' quoted code and output are refreshed to match what the examples now produce.

## Data Structures & Interfaces

**`types.Sensitive[T]` (new, public).** This is a field type a type author uses to declare a value sensitive. The real value is unexported. Every display path yields `SensitiveMarker`.

```go
package types

const SensitiveMarker = "(sensitive)"

type Sensitive[T any] struct { /* value T, unexported */ }

func NewSensitive[T any](value T) Sensitive[T]

func (s Sensitive[T]) Reveal() T                        // the only way to the real value
func (s Sensitive[T]) String() string                   // SensitiveMarker
func (s Sensitive[T]) GoString() string                 // SensitiveMarker, for %#v
func (s Sensitive[T]) Format(f fmt.State, verb rune)    // SensitiveMarker for every verb
func (s Sensitive[T]) LogValue() slog.Value             // slog.StringValue(SensitiveMarker)
func (s Sensitive[T]) MarshalText() ([]byte, error)     // SensitiveMarker
func (s Sensitive[T]) MarshalJSON() ([]byte, error)     // "\"(sensitive)\""
func (s *Sensitive[T]) UnmarshalJSON(data []byte) error // real T; the bare marker string gives a redacted value
```

A type author writes `Password types.Sensitive[string] \`xcl:"password" json:"password"\``. The field is used as a value, never a pointer.

**`types.SensitiveValue` (new, public, sealed).** Every `Sensitive[T]` implements it. xcl's encoder, printer and wire encoder use it to recognise a sensitive value of any `T` and to reveal it. An unexported method seals it, so nothing outside `types` can implement it.

```go
type SensitiveValue interface {
    RevealAny() any // the real value as any; explicit, greppable, like Reveal
    sensitive()
}
```

`types.IsRedacted(SensitiveValue) bool` reports a value that was read back from the marker, such as from event data, and so has no real value. Reveal options still print the marker for it.

**`types.SensitiveMark` (new, public value).** The cty mark that sensitivity travels as through configuration. It is an unexported, comparable Go type exposed as a single value, as cty recommends for mark values. The parser reads it when converting outputs and when guarding decodes.

**`gocty` wrapper hook (new, copied library, generic).** This is how a Go type that wraps one inner value takes part in cty conversion while carrying a mark. `types` registers one wrapper at package initialisation.

```go
package gocty

type Wrapper struct {
    Inner  func(t reflect.Type) (inner reflect.Type, ok bool) // recognise a wrapper type and give its inner type
    Unwrap func(wrapper reflect.Value) reflect.Value          // the inner value
    Wrap   func(target reflect.Value, inner reflect.Value)    // store an inner value into an addressable wrapper
    Mark   any                                                // the mark applied on the way into cty
}

func RegisterWrapper(w Wrapper)
```

`ImpliedType` gives the inner type's cty type. `ToCtyValue` returns the inner value converted and marked. `FromCtyValue` into a wrapper removes the mark and wraps the result, whether or not the value was marked. `FromCtyValue` of a marked value into anything else, apart from a `cty.Value` target, returns a `cty.PathError` saying the value is sensitive.

**`gohcl.EncodeOptions.ReplaceMarked` (new field, copied library, generic).** It is called with each marked value before writing. It returns the value to write. Leaving it nil writes the value unmarked.

```go
type EncodeOptions struct {
    IncludeComputed bool
    ComputedComment string
    ReplaceMarked   func(cty.Value) cty.Value
}
```

**`xcl.RevealSensitive() EncodeOption` (new, public).** Configuration text shows real values. Without it, each sensitive value is written as the string `"(sensitive)"`.

**`logger.WithRevealSensitive(reveal bool) PrinterOption` (new, public).** The resource printer shows real values in every format. The default is `false`.

**`errors.TypeMismatchError.Field` (new member).** It is the dotted path of the field that blocked the conversion. When it is set, the message reads `entity "<address>" field "<field>" is sensitive, <want> declares it as a plain value`. It still matches `ErrTypeMismatch`.

**`types.Output.SensitivePaths` (new field).** It holds the paths, inside `Value`, of the parts that are sensitive. `[]` means the whole value is sensitive. Map keys and list indices are written as strings. It is serialised as `sensitive_paths` and omitted when empty. When an output is read back from JSON, those parts are wrapped again.

```go
type Output struct {
    ResourceBase   `xcl:",remain"`
    CtyValue       cty.Value  // keeps marks
    Value          any        `json:"value"` // sensitive parts held as Sensitive[string|float64|bool|map[string]any|[]any]
    SensitivePaths [][]string `json:"sensitive_paths,omitempty"`
    Description    string     `xcl:"description,optional" json:"description,omitempty"`
}
```

**`internal/wire` (new, internal).** It serialises with real sensitive values. Every other rule follows `encoding/json`: tag names, `omitempty`, `-`, embedded struct flattening and existing `Marshaler`s.

```go
func Marshal(v any) ([]byte, error)
func MarshalIndent(v any, prefix, indent string) ([]byte, error)
```

**`StateStore.Save` input (changed contract).** Each element of the `[]any` passed to `Save` is now a `json.RawMessage` holding the entity's revealed JSON, not the typed entity. `Load` may return the same raw messages, as it already may.

**Schema type map (changed, shared).** The host's map from schema type names to Go types moves to one function that the registry and the plugin test helpers both use. It adds `types.Sensitive[string]`, `[int]`, `[int64]`, `[float64]`, `[bool]`, `[[]string]` and `[map[string]string]`.

## Implementation Detail

**New pattern: two encodings of one entity, chosen by the caller.** Until now, one `json.Marshal` served every purpose. After this change, a reader of the code will see two deliberate kinds of call. A call that must keep the truth, meaning state, a provider call, change detection or a conversion, uses the internal wire encoder. A call that is shown to someone, meaning events, the printer's JSON or an application's own `json.Marshal`, uses plain `encoding/json` and gets the marker. The choice is made at each call site, never through global state. That matches what the later masking spec needs, because it will swap the encoder at exactly the same sites. A short package comment on the wire encoder lists who may use it. A static test keeps the list honest: it fails if a library file outside the allowed callers imports the wire encoder.

**New pattern: a generic wrapper hook in the cty conversion library.** The copied `gocty` library gains one extension point: a registered wrapper is a Go type that converts as its inner type and carries a mark. The sensitive type registers itself once. The library stays free of any knowledge of xcl, so its code reads like upstream with one clearly fenced addition, recorded in its `UPSTREAM.md`. The same principle applies to the copied `gohcl` encoder, which gains a generic "replace marked values" callback rather than any reference to sensitivity. Turning a decode-time panic on marked values into a path error is a behaviour fix inside the same fenced addition.

**New pattern: static mark prediction during validation.** A new validation stage sits beside the existing body-walking stages and is built the same way. It walks attributes and nested blocks in step with the Go field types. It reports positioned problems collected into the configuration error, and it stops later stages when it fires. What is new is a small "sensitivity of an expression" judgement. It is a pure function of the expression's traversals, how they resolve, and the Go types or stored output bodies they reach. Combining operations taint the whole value, a bare traversal keeps exact paths, and constructors recurse. It is cached per output and guarded against cycles. It never evaluates, so it never runs a user function and never meets the unknown-argument mark loss.

**Existing patterns followed.**
- Options follow the shape of the API they extend: `RevealSensitive()` sits beside `IncludeComputed()`, and `WithRevealSensitive(bool)` beside `WithColor(bool)`.
- The field-level conversion failure extends the existing `TypeMismatchError` and keeps `ErrTypeMismatch` as its sentinel, per the shared-errors convention.
- The new public types live in `types`, beside `ResourceBase` and `Output`.
- Plugin type support extends the existing host type map rather than adding a second rebuild path.

**Code-shape changes.**
- The two state save sites now encode before saving, so a store receives raw JSON.
- The event data helper stops reusing provider-call bytes and always encodes the typed entity.
- The function set the parser hands to HCL is wrapped once, in one place, rather than each function redacting its own errors.
- The configured-value and computed-field walks gain a single "sensitive is a leaf" rule.

**Public surface UX.** A type author changes `Password string` to `Password types.Sensitive[string]` and nothing else. Configuration authors write exactly what they wrote before. Assigning a secret to a plain field is now a validation error that names the field. Application code reads `cfg.Password.Reveal()` where it needs the secret, and every `fmt`, `slog`, `json` or template output of the entity shows `(sensitive)`. Tests construct one with `types.NewSensitive("x")`. A developer who wants real values in printed configuration passes `xcl.RevealSensitive()`, and in printer output `logger.WithRevealSensitive(true)`. A plugin author declares the field the same way and calls `Reveal()` only where the value is used, guided by the plugin developer guide.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **Upstream plan `20261003153421-6ec0eab3-module-boundary-and-output-entities` (must land first).** It provides the public `types.Output` with `CtyValue` and `Value`, outputs as entities, the output-only module boundary, and module keys composed with `FQRN.AppendParentModule`. This plan adds `SensitivePaths` to `types.Output`, and relies on the boundary so a child's sensitive output is reached only through `module.<name>.output.<x>`.
- **`types` package (changed).** It receives `Sensitive[T]`, `SensitiveValue`, `SensitiveMarker` and `SensitiveMark`, and registers the cty wrapper. It may import `internal/cty`, per `architecture/shared-public-types-live-in-types.md`.
- **`internal/cty` (copied go-cty v1.15.0, MIT; changed).** Its marks API is used as is. `gocty` gains the generic wrapper hook and the marked-value error. The changes are recorded in `internal/cty/UPSTREAM.md`.
- **`internal/xcl` (copied HCL v2.21.0, MPL-2.0; changed).** Templates, function calls and expressions already carry marks. `gohcl`'s encoder gains the generic `ReplaceMarked` option. The MPL headers stay, the modified-by line is added, and the change is recorded in `internal/xcl/UPSTREAM.md`.
- **`internal/parser` (changed).** It gains the new validation stage, the output value conversion, the function wrapper, the event data change, the leaf rule in the configured and computed walks, and wire-encoder call sites.
- **`internal/schema`, `plugins`, `plugins/registry`, `plugins/testing` (changed).** These take the shared type map, the nested-bracket fix, and the wire encoder at plugin-side marshal sites and in change detection.
- **`internal/savedentity`, `state` (changed contract, little code).** Loading is unchanged. `StateStore.Save` receives raw JSON elements.
- **`errors` package (changed).** It gains `TypeMismatchError.Field`. Its thin import list is kept.
- **`logger` package (changed).** The resource printer gains a sensitive case and `WithRevealSensitive`.
- **Standard library only.** The plan uses `fmt`, `log/slog`, `encoding`, `encoding/json` and `reflect`, and adds no new module dependency. Tests use testify `require`, which is already a dependency.
- **`xcl-website` repo.** It gains the new guide page, the navigation entry and refreshed example pages. Its snippets are copied text with no code dependency, so it lands after the library docs. Its gate is its own build and type-check after `npm ci`.
- **Knowledge entries relied on.** `gotchas/cty-unknown-args-drop-marks.md`, `gotchas/custom-marshaljson-changes-internal-hops.md`, `gotchas/plugin-types-rebuilt-with-structof.md`, `learnings/state-save-and-load-points.md`, `gotchas/xcl-tags-gate-what-reaches-cty.md`, `architecture/shared-public-types-live-in-types.md` and `conventions/never-modify-dependencies.md`. None are changed.
- **Downstream specs in the epic.** `20261003153421-bf87d907-references-as-written`, `20261003153421-c283547c-user-depends-on` and `20261003153421-9fa72edd-masking` all build on this one. Masking replaces the fixed event redaction with pluggable maskers and adds state encryption at the two save sites this plan establishes. References-as-written must keep writing sensitive values as the marker in configuration text, with one exception settled as an epic decision: under `xcl.ShowReferences()`, a sensitive attribute written as a single bare reference shows that reference, because an address does not reveal the secret.

## Testing Approach

All tests follow the project's conventions:
- They use testify `require`.
- There are no table-driven tests, and every accepted case and every rejected case has its own named test function.
- Test state comes from a real `Apply`, and a test that reloads state builds a second `Config` on the state the first apply wrote.

One known secret value is used throughout the new fixtures, so each leak assertion is a plain "output does not contain this string".

**Unit tests: the sensitive type.** `NewSensitive` and `Reveal` round-trip a value. These each produce exactly the marker, one test per path:
- `%v`, `%+v`, `%#v`, `%s`, `%q`, `%d` and `%x`;
- `String`;
- `slog` through both a text handler and a JSON handler;
- `json.Marshal` and `MarshalText`.

Unmarshalling real JSON gives the real value. Unmarshalling the bare marker gives a redacted value that still prints as the marker. These guarantee the spec's "protected form everywhere but unwrap" at the source.

**Unit tests: the wrapper hook and the wire encoder.** On the `gocty` side:
- a `Sensitive` field implies its inner cty type;
- it converts to a marked value;
- it decodes back from both a marked and a plain value.

A marked value decoded into a plain field returns an error naming the path instead of panicking. The wire encoder writes real values for top-level, nested, embedded, slice, map and `any`-held sensitive values. It writes byte-identical output to `encoding/json` for an entity holding no sensitive value, so every existing hop is unchanged for existing types.

**Unit and fixture tests: validation (the heaviest coverage).** This is the rule with the most shapes, and it is where silent gaps would hide. Each of these is rejected, with a problem naming the entity and the field:
- a sensitive field referenced directly into a plain field;
- the same value interpolated into a template;
- the same value passed through a function;
- the same value inside an object or tuple constructor;
- a sensitive child-module output used by its caller;
- a sensitive value passed into a module input and used in a plain field inside the module.

The same shapes into a sensitive field each validate, and so does a plain value into a sensitive field. `Apply` refuses a rejected configuration before anything is created.

**End-to-end tests: application and plugin types.** These cover the spec's first two criteria and its state criterion. An application type with a sensitive field, and a plugin type with one (in-process and external), are each configured, applied and reloaded from state, and `Reveal` returns the real value. Interpolated and function-derived values arrive in a sensitive field as a sensitive value holding the combined result. A module output whose value is sensitive can be assigned by the caller to a sensitive field. An output whose value is an entity with one sensitive and one plain field reads back in Go as a map holding a sensitive leaf and a plain leaf. A wholly sensitive output reads back as a sensitive value. Both survive a state reload.

**Conversion tests (root package).** `Find`, `FindByType`, `All`, `As` and `Decode` into a Go type whose matching field is plain each fail with `ErrTypeMismatch`, and the detail names the field. The same calls into a type whose field is sensitive succeed, and they keep the real value.

**Leak tests: one end-to-end leak suite.** One fixture holds a known secret in a sensitive field. It is applied with event data at both levels, the slog bridge, a custom slog handler, a plugin that logs the entity as a detail, and plain `fmt` formatting of the entity. Every captured byte is checked for the secret, and the marker must be present. In the same suite:
- an error built from a failing function call on a sensitive argument contains the marker and not the secret;
- configuration text and each printer format (table, tree, card, json) show the marker by default and the secret only with the reveal option, the default and the revealing case in separate tests.

**Regression.** The existing parser, state, plugin, encode and query tests must pass unchanged. That proves the wire encoder and the new validation stage alter nothing for types with no sensitive field.

**Examples and documentation.** Each example's tests set the example's secrets to known values, including `DB_PASSWORD` in the environment. They run the example and assert that neither stdout nor stderr contains any of them. The existing output assertions are updated where a password line now shows the marker. The README and CHANGELOG content tests are extended for the new section, the plugin-author guidance and the changelog entry, each in its own test. The site has no content tests. Its build and type-check are the automated gate.

**Success metrics.**
- *Zero occurrences of a known test secret in logs, errors, or printed and encoded output across the full test suite and every bundled example, under default settings*:
  - **Behavioural test.** The leak suite and the per-example no-secret tests assert it for every output path and every bundled example.
  - **Manual — captured in the implementation test plan.** Run the full suite verbosely and search all of its output for the known secret.
- *Every bundled example that uses a secret declares it sensitive and reads it with an explicit unwrap, with no workaround code*:
  - **Behavioural test.** A static check over the example sources asserts that every example resource field named for a password or secret is a `types.Sensitive` type, and that each example using one calls `Reveal()`.
  - **Manual — captured in the implementation test plan.** Review the example code for any workaround that keeps a secret out of output by other means.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: read the new Sensitive values page and the refreshed example pages on the `xcl-website` site in a browser. Check that they cover declaring a sensitive field, `Reveal()`, the two sensitive-to-plain errors, and the marker in the application's own JSON and templates. Check also that every quoted example output matches what the examples now print.
- **Manual — captured in the implementation test plan**: read the plugin developer guide's sensitive-fields section. Check that it says an unwrapped value is no longer protected and must not be logged or emitted.

**Deliberate gaps.**
- No test asserts behaviour for plugin `Sensitive` instantiations outside the supported set beyond the load-time error.
- Diagnostics that show the user's own source text are not redacted, per the spec's non-goals, so no test covers them.
- Pluggable event masking and state encryption are left to the masking spec.

## Milestones & Tasks

### Milestone 1: Type authors can declare a field sensitive, and it keeps its real value only where it must

**What changes**: A type author, for an application type or a plugin type, can declare a field `types.Sensitive[T]`, and application code can build one with `types.NewSensitive`. Configuration sets that field exactly as before. After `Apply`, the entity holds a sensitive value, and only `Reveal()` gives the real one. Printing it with `fmt`, logging it with `slog`, or marshalling it with `encoding/json` or as text shows `(sensitive)`. State keeps the real value, so a new configuration that loads it reads the same value back. Plugins receive and return the real value, and changes to it are still detected. Event data shows the marker at every data level. Custom state stores now receive each entity as raw JSON, which is the breaking part of this milestone.

**Validation point**: The full existing suite passes unchanged. New tests prove every formatting and marshalling path shows only the marker. They prove an application type and a plugin type, in-process and external, round-trip the real value through apply, state and reload. They also prove event data at both levels contains no real value.

#### - [x] Task: Add the sensitive value type
**Id:** 0f8904f5-98be-4b5b-a16d-dfc606dcc015
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

The public `types` package gains `Sensitive[T]`, a field type that holds a real value which only `Reveal()` returns. Printing, logging, text marshalling and JSON marshalling all yield the fixed marker `(sensitive)`. Application code can build one with `NewSensitive`, for example in tests. Reading real JSON gives the real value back, and reading the bare marker gives a redacted value. A sealed interface lets xcl's own code recognise any sensitive value without knowing its inner type.

*Technical detail:* [context.md#task-add-the-sensitive-value-type](./context.md#task-add-the-sensitive-value-type)

**Acceptance criteria**:
- [x] A sensitive value built from a plain value returns that value from `Reveal()`.
- [x] Every `fmt` verb, `String`, `slog` text and JSON output, `MarshalText` and `json.Marshal` produce the marker and never the real value.
- [x] Unmarshalling the real value's JSON gives a sensitive value that reveals it, and unmarshalling the marker gives one that still prints as the marker.
- [x] Code outside the `types` package cannot implement the sensitive-value interface.

#### - [x] Task: Carry sensitive values through configuration evaluation
**Id:** cb6e4c6d-ddb8-4c2f-badf-150627067251
**Repo:** xclconfig
**Depends on:**
- 0f8904f5-98be-4b5b-a16d-dfc606dcc015 — Add the sensitive value type
**Execution:** agent

The copied cty conversion library gains a generic hook for wrapper types, and the sensitive type registers itself with it. A configuration can then set a sensitive field from a plain or a sensitive value. A referenced entity's sensitive field enters expressions as a marked value of its inner type. A marked value that reaches a plain field fails with an error naming the field, rather than crashing. The configured-value and computed-field checks treat a sensitive value as a single leaf, so a changed secret is still noticed.

*Technical detail:* [context.md#task-carry-sensitive-values-through-configuration-evaluation](./context.md#task-carry-sensitive-values-through-configuration-evaluation)

**Acceptance criteria**:
- [x] A configuration sets a sensitive field from a literal, a variable and a reference, and after apply each reveals the expected value.
- [x] A reference to another entity's sensitive field evaluates to a value of the inner type that carries the sensitive mark.
- [x] A marked value decoded into a plain field produces an error naming the field instead of a panic.
- [x] The cty conversion library's documentation of local changes records the new hook.

#### - [x] Task: Keep real sensitive values on every internal hop
**Id:** 574ca9e3-9c3d-4e09-a743-4711e529ef18
**Repo:** xclconfig
**Depends on:**
- 0f8904f5-98be-4b5b-a16d-dfc606dcc015 — Add the sensitive value type
**Execution:** agent

A new internal encoder writes entities with their real sensitive values and otherwise matches standard JSON. Every hop that must carry the truth switches to it:
- provider calls in both directions;
- change detection and the configured-value check;
- the host state callback;
- query conversion.

State is saved by encoding each entity first and handing the store raw JSON, so built-in and custom stores both keep real values. Loading is unchanged.

*Technical detail:* [context.md#task-keep-real-sensitive-values-on-every-internal-hop](./context.md#task-keep-real-sensitive-values-on-every-internal-hop)

**Acceptance criteria**:
- [x] An entity with no sensitive field encodes to exactly the same bytes as before.
- [x] Sensitive values at any depth, including inside `any` values, maps and slices, are written as their real values.
- [x] State written after apply and during destroy holds real sensitive values, and a custom state store receives each entity as raw JSON.
- [x] A provider receives the real value, and changing only a sensitive value is detected as a change.

#### - [x] Task: Show only the marker in event data
**Id:** 6bb6948f-d66a-4aa8-b388-d248fe7744cc
**Repo:** xclconfig
**Depends on:**
- 574ca9e3-9c3d-4e09-a743-4711e529ef18 — Keep real sensitive values on every internal hop
**Execution:** agent

Event data no longer reuses the bytes sent to a provider, which now hold real values. At both data levels, event data is encoded from the typed entity with standard JSON, so sensitive fields show the marker. Processed event data can still be turned into configuration text.

*Technical detail:* [context.md#task-show-only-the-marker-in-event-data](./context.md#task-show-only-the-marker-in-event-data)

**Acceptance criteria**:
- [x] Raw and processed event data for an entity with a sensitive field contain the marker and not the real value, at every phase.
- [x] Event data for entities without sensitive fields is unchanged.
- [x] Processed event data with a sensitive field can still be converted to configuration text.

#### - [x] Task: Support sensitive fields on plugin types
**Id:** 5df9c560-db27-4028-a9ac-229ddea691e9
**Repo:** xclconfig
**Depends on:**
- 0f8904f5-98be-4b5b-a16d-dfc606dcc015 — Add the sensitive value type
- 574ca9e3-9c3d-4e09-a743-4711e529ef18 — Keep real sensitive values on every internal hop
**Execution:** agent

Plugin types are rebuilt on the host from a schema, so the host learns the supported sensitive instantiations, and the schema reader handles nested brackets. The plugin test helpers use the same type list. A plugin type using an unsupported instantiation fails to load with an error naming the type and field, instead of silently losing the data.

*Technical detail:* [context.md#task-support-sensitive-fields-on-plugin-types](./context.md#task-support-sensitive-fields-on-plugin-types)

**Acceptance criteria**:
- [x] A plugin type with a field of each supported sensitive instantiation is rebuilt on the host with that field typed as the sensitive type.
- [x] A plugin type using an unsupported instantiation fails plugin loading with an error naming the type and the field.
- [x] The plugin test helpers rebuild plugin types the same way the host does.

#### - [x] Task: Prove sensitive fields round-trip through apply and state
**Id:** 74c057c9-b1e7-4082-af0e-f4e850d5d013
**Repo:** xclconfig
**Depends on:**
- cb6e4c6d-ddb8-4c2f-badf-150627067251 — Carry sensitive values through configuration evaluation
- 574ca9e3-9c3d-4e09-a743-4711e529ef18 — Keep real sensitive values on every internal hop
- 6bb6948f-d66a-4aa8-b388-d248fe7744cc — Show only the marker in event data
- 5df9c560-db27-4028-a9ac-229ddea691e9 — Support sensitive fields on plugin types
**Execution:** agent

End-to-end tests configure and apply an application type and a plugin type, in-process and external, each with a sensitive field. They then load the saved state into a new configuration and reveal the real value. The same runs check that events for those entities show only the marker.

*Technical detail:* [context.md#task-prove-sensitive-fields-round-trip-through-apply-and-state](./context.md#task-prove-sensitive-fields-round-trip-through-apply-and-state)

**Acceptance criteria**:
- [x] An application type's sensitive field reveals the configured value after apply and after reloading state.
- [x] A plugin type's sensitive field does the same, for an in-process and for an external plugin.
- [x] Events for those entities contain the marker and not the value.

### Milestone 2: Sensitivity follows a value through the configuration and cannot reach a plain field

**What changes**: Sensitivity travels with a value. A value referenced, interpolated or passed through a function is still sensitive. So is a value carried through a module input or out of a module's output. Assigning a sensitive value, or one derived from one, to a field that is not declared sensitive fails validation with an error naming the field, before anything is created. In Go, an output's value holds its sensitive parts as sensitive values and its plain parts as plain values, and keeps them that way after reloading. Converting or looking up an entity into a Go type whose matching field is plain fails with a type error naming the field. The same conversion into a type that declares the field sensitive succeeds.

**Validation point**: Validation tests show each sensitive-to-plain shape rejected with the field named, and each sensitive-to-sensitive shape accepted. End-to-end tests show interpolated, function-derived and module-output values arriving as sensitive. Partly and wholly sensitive outputs read correctly before and after a reload. Conversion tests show the plain-field type error and the sensitive-field success.

#### - [x] Task: Reject sensitive values assigned to plain fields during validation
**Id:** cd312385-6c76-43e1-a41c-baba9cf9858e
**Repo:** xclconfig
**Depends on:**
- cb6e4c6d-ddb8-4c2f-badf-150627067251 — Carry sensitive values through configuration evaluation
**Execution:** agent

A new validation stage predicts, without evaluating anything, which parts of each attribute's value are sensitive. A sensitive part can come from:
- a sensitive field, referenced directly, interpolated or passed through a function;
- a child module's sensitive output;
- a module input fed a sensitive value.

Any sensitive part landing in a field that is not declared sensitive is reported as a validation problem naming the entity and the field. So both `Validate` and `Apply` refuse the configuration before anything is created. Assignments into sensitive fields, and plain values into sensitive fields, are accepted.

*Technical detail:* [context.md#task-reject-sensitive-values-assigned-to-plain-fields-during-validation](./context.md#task-reject-sensitive-values-assigned-to-plain-fields-during-validation)

**Acceptance criteria**:
- [x] Assigning a sensitive value to a plain field directly, by interpolation, through a function, or inside an object or list fails validation, and each error names the field.
- [x] Assigning a child module's sensitive output to a plain field in the caller fails validation, and assigning it to a sensitive field validates.
- [x] A sensitive value passed into a module input and used in a plain field inside the module fails validation.
- [x] The same values assigned to sensitive fields validate, and `Apply` creates nothing when validation fails.

#### - [x] Task: Keep sensitivity in output values
**Id:** a87feb82-b463-4e59-8f00-2d94d7c37138
**Repo:** xclconfig
**Depends on:**
- cb6e4c6d-ddb8-4c2f-badf-150627067251 — Carry sensitive values through configuration evaluation
- 574ca9e3-9c3d-4e09-a743-4711e529ef18 — Keep real sensitive values on every internal hop
**Execution:** agent

An output's Go value now holds exactly its sensitive parts as sensitive values, and keeps everything else plain:
- a whole sensitive value becomes a sensitive value;
- an entity with one sensitive field becomes a map whose sensitive leaf is wrapped.

The output records which paths are sensitive, so reading it back from state wraps them again. A module's sensitive output reaches its caller still marked.

*Technical detail:* [context.md#task-keep-sensitivity-in-output-values](./context.md#task-keep-sensitivity-in-output-values)

**Acceptance criteria**:
- [x] An output whose value is an entity with one sensitive and one plain field holds the sensitive field as a sensitive value and the plain field as a plain value.
- [x] An output whose whole value is sensitive holds a sensitive value.
- [x] Both read the same after state is saved and reloaded.
- [x] A module output whose value is sensitive arrives in a caller's sensitive field as a sensitive value revealing the real value.
- [x] A sensitive value interpolated into a string or passed through a function arrives in a sensitive field holding the combined result.

#### - [x] Task: Refuse converting a sensitive field into a plain Go field
**Id:** 2d4c69fd-e26b-4160-8512-39ba48baf4fb
**Repo:** xclconfig
**Depends on:**
- 574ca9e3-9c3d-4e09-a743-4711e529ef18 — Keep real sensitive values on every internal hop
**Execution:** agent

Before an entity is converted into a different Go type, the two types are compared field by field. Where a sensitive field would land in a plain field, the lookup fails with the existing type-mismatch error, now naming the field. Conversions into a type that declares the field sensitive succeed and keep the real value. There is no way to unwrap automatically.

*Technical detail:* [context.md#task-refuse-converting-a-sensitive-field-into-a-plain-go-field](./context.md#task-refuse-converting-a-sensitive-field-into-a-plain-go-field)

**Acceptance criteria**:
- [x] Finding, listing, `All`, `As` and `Decode` into a type whose matching field is plain each fail with a type-mismatch error naming the field.
- [x] The same calls into a type whose matching field is sensitive succeed and reveal the real value.

### Milestone 3: Errors, configuration text and printed resources never show a secret unless asked

**What changes**: Errors xcl produces show sensitive values only as the marker, including a failing function called with a sensitive argument. Configuration text from `EncodeEntity` and `EncodeSavedEntity` writes each sensitive value as `"(sensitive)"`, and writes the real value only when `xcl.RevealSensitive()` is passed. The resource printer's table, tree, card and JSON formats show the marker by default and real values with `logger.WithRevealSensitive(true)`. One end-to-end leak suite proves that a known secret never appears in any log, event, error, configuration text or printer output under default settings.

**Validation point**: The leak suite passes. The error, encoder and printer tests cover the default and the revealing case, each in its own test.

#### - [x] Task: Redact sensitive arguments in function errors
**Id:** 2814aae7-c890-46f4-a372-0009484d02bf
**Repo:** xclconfig
**Depends on:**
- cb6e4c6d-ddb8-4c2f-badf-150627067251 — Carry sensitive values through configuration evaluation
**Execution:** agent

Functions build their error messages from their arguments. So every function the parser offers, built-in or custom, is wrapped once. When a call that received a sensitive argument fails, the error shows the marker in place of that argument's value. Successful results keep their sensitivity.

*Technical detail:* [context.md#task-redact-sensitive-arguments-in-function-errors](./context.md#task-redact-sensitive-arguments-in-function-errors)

**Acceptance criteria**:
- [x] A failing function called with a sensitive argument produces an error containing the marker and not the value.
- [x] A failing function called with plain arguments produces the same error as before.
- [x] A successful call on a sensitive argument still returns a sensitive result.

#### - [x] Task: Redact sensitive values in configuration text
**Id:** b36f9bb6-6670-4478-8fc1-21f00b713e89
**Repo:** xclconfig
**Depends on:**
- cb6e4c6d-ddb8-4c2f-badf-150627067251 — Carry sensitive values through configuration evaluation
**Execution:** agent

Converting an entity, or its saved data, to configuration text writes each sensitive value as `"(sensitive)"` by default. The new `xcl.RevealSensitive()` option writes the real value instead. The copied HCL encoder gains only a generic option for how marked values are written.

*Technical detail:* [context.md#task-redact-sensitive-values-in-configuration-text](./context.md#task-redact-sensitive-values-in-configuration-text)

**Acceptance criteria**:
- [x] Configuration text for an entity with a sensitive field shows the marker by default, from a live entity and from saved data alike.
- [x] With `RevealSensitive()`, the text shows the real value.
- [x] Text for entities without sensitive fields is unchanged.

#### - [x] Task: Redact sensitive values in the resource printer
**Id:** a04e28f4-4fac-4c22-a3df-29e3d30a81d4
**Repo:** xclconfig
**Depends on:**
- 0f8904f5-98be-4b5b-a16d-dfc606dcc015 — Add the sensitive value type
- 574ca9e3-9c3d-4e09-a743-4711e529ef18 — Keep real sensitive values on every internal hop
**Execution:** agent

The resource printer's table, tree, card and JSON formats recognise a sensitive value and print the marker, instead of walking into it or printing its type name. `WithRevealSensitive(true)` prints the real value in each format.

*Technical detail:* [context.md#task-redact-sensitive-values-in-the-resource-printer](./context.md#task-redact-sensitive-values-in-the-resource-printer)

**Acceptance criteria**:
- [x] Each printer format shows the marker for a sensitive field by default.
- [x] Each printer format shows the real value when revealing is requested.

#### - [x] Task: Prove no secret leaks from any output path
**Id:** afefb347-064b-48e9-afef-86b7306f7c64
**Repo:** xclconfig
**Depends on:**
- 6bb6948f-d66a-4aa8-b388-d248fe7744cc — Show only the marker in event data
- a87feb82-b463-4e59-8f00-2d94d7c37138 — Keep sensitivity in output values
- 2814aae7-c890-46f4-a372-0009484d02bf — Redact sensitive arguments in function errors
- b36f9bb6-6670-4478-8fc1-21f00b713e89 — Redact sensitive values in configuration text
- a04e28f4-4fac-4c22-a3df-29e3d30a81d4 — Redact sensitive values in the resource printer
**Execution:** agent

One end-to-end leak suite applies a configuration holding a known secret in a sensitive field and a sensitive output. It captures everything xcl can emit:
- events at both data levels;
- the slog bridge and a custom slog handler;
- a plugin's log details;
- plain formatting of the entity;
- errors;
- configuration text;
- each printer format.

Each capture is checked for the secret and for the marker.

*Technical detail:* [context.md#task-prove-no-secret-leaks-from-any-output-path](./context.md#task-prove-no-secret-leaks-from-any-output-path)

**Acceptance criteria**:
- [x] No captured log, event, error, configuration text or printer output contains the known secret under default settings.
- [x] Each of those outputs contains the marker where the secret would have been.

### Milestone 4: The examples stop printing secrets and the documentation explains sensitive values

**What changes**: The application-config and plugin examples declare their passwords sensitive and unwrap them only where they use them. Running any example prints no password or secret. The README and plugin developer guide cover:
- declaring a sensitive field and unwrapping it with `Reveal()`;
- the validation error and the Go type error raised when a sensitive value meets a plain field;
- that the application's own JSON and templates show the marker unless the value is unwrapped;
- what a plugin gives up by unwrapping.

The state guide says state keeps real values. The changelog records the feature and its breaking change. The documentation site gains a Sensitive values page, and its example pages match what the examples now print.

**Validation point**: Every example's no-secret test passes. So does the static check that example secrets are sensitive and read with `Reveal()`. The README and changelog content tests pass and fail if the new text is removed. The site builds and type-checks.

#### - [x] Task: Declare the examples' secrets sensitive
**Id:** a827d510-e864-46fd-aa95-8fc27261ad9b
**Repo:** xclconfig
**Depends on:**
- 5df9c560-db27-4028-a9ac-229ddea691e9 — Support sensitive fields on plugin types
- cd312385-6c76-43e1-a41c-baba9cf9858e — Reject sensitive values assigned to plain fields during validation
- b36f9bb6-6670-4478-8fc1-21f00b713e89 — Redact sensitive values in configuration text
- afefb347-064b-48e9-afef-86b7306f7c64 — Prove no secret leaks from any output path
**Execution:** agent

The application-config example's database password and the plugin example's database password become sensitive fields. Each example reads the password with `Reveal()` only where it is actually used, never to print or log it. Each example's tests set its secrets to known values and assert that nothing the example prints contains them. A static check keeps example secrets declared sensitive and read with an explicit unwrap.

*Technical detail:* [context.md#task-declare-the-examples-secrets-sensitive](./context.md#task-declare-the-examples-secrets-sensitive)

**Acceptance criteria**:
- [x] Running each bundled example prints no password or secret from its configuration, on standard output or standard error.
- [x] Every example field holding a password or secret is declared sensitive and read with `Reveal()`, with no other workaround keeping it out of output.
- [x] The examples' existing output tests pass, with any password line now showing the marker.

#### - [x] Task: Document sensitive values in the library
**Id:** 16a3ab8d-b234-4bb0-bb2a-6b8e4128259a
**Repo:** xclconfig
**Depends on:**
- 2d4c69fd-e26b-4160-8512-39ba48baf4fb — Refuse converting a sensitive field into a plain Go field
- a827d510-e864-46fd-aa95-8fc27261ad9b — Declare the examples' secrets sensitive
**Execution:** agent

The README gains a Sensitive values section covering:
- declaring a field, building one in tests and unwrapping with `Reveal()`;
- the validation error and the Go type error;
- that the application's own JSON and templates show the marker unless unwrapped;
- the reveal options for configuration text and the printer.

Statements that secrets are shown are removed. The plugin developer guide explains sensitive fields on plugin types, the supported instantiations, and that an unwrapped value is no longer protected and must not be logged or emitted. The state guide says state keeps real values. The changelog gains an entry for this spec with its breaking change, and the content tests guard all of it.

*Technical detail:* [context.md#task-document-sensitive-values-in-the-library](./context.md#task-document-sensitive-values-in-the-library)

**Acceptance criteria**:
- [x] The README describes declaring sensitive fields, unwrapping, both sensitive-to-plain errors, the marker in the application's own serialisation and templates, and the reveal options.
- [x] The README and changelog no longer say secrets are shown in configuration text.
- [x] The plugin developer guide states that an explicitly unwrapped value is no longer protected and must not be logged or otherwise emitted.
- [x] The changelog has an entry for this spec that lists the breaking change.
- [x] Content tests fail if any of these sections or the changelog entry is removed.

#### - [x] Task: Document sensitive values on the site
**Id:** e4fdf6fa-045c-4646-84df-9bd6ed537cc4
**Repo:** xcl-website
**Depends on:**
- 16a3ab8d-b234-4bb0-bb2a-6b8e4128259a — Document sensitive values in the library
**Execution:** agent

The documentation site gains a Sensitive values guide page, linked from the navigation. It covers:
- declaring sensitive fields and unwrapping them in application code;
- the errors raised when a sensitive value meets a plain field;
- that the application's own JSON and templates show the marker unless unwrapped.

The home page and example pages that quote the password fields, configuration or printed output are refreshed to match what the examples now produce. The wording follows the library documentation.

*Technical detail:* [context.md#task-document-sensitive-values-on-the-site](./context.md#task-document-sensitive-values-on-the-site)

**Acceptance criteria**:
- [x] The site has a Sensitive values page, reachable from the navigation, covering declaring, unwrapping, both errors and the marker in the application's own output.
- [x] Every example code and output quoted on the site matches what the examples now contain and print.
- [x] The site builds and type-checks.

## Open Questions

- **Does `hclwrite` or any HCL diagnostic render a marked value in a way the plan has not anticipated?** This depends on code paths that only run once marked values reach them: `hclwrite` value tokens, and HCL's own expression diagnostics that quote values. The research found none that render evaluated values, but only running them proves it. If one appears, redact it the same way, by unmarking with the marker, inside the generic `gohcl` option or the parser's diagnostic handling. Record any change to `internal/xcl` in its `UPSTREAM.md`. STOP and ask the user only if the fix would mean putting xcl-specific code into the MPL-licensed fork.
- **Does a sensitive value inside a module input object leave the container unmarked in every path?** This depends on how the walker evaluates `variables = { ... }` expressions. A whole-object mark would panic at `AsValueMap`. The context-building tests in the evaluation task will show it. If a container comes back marked, unmark the container while keeping per-attribute marks with `UnmarkDeepWithPaths` and `MarkWithPaths`. The behaviour stays as planned, so no question to the user is needed unless that cannot preserve per-attribute sensitivity.

## Out of Scope

- **Protection against an attacker with access to the process.** The protection is against accidental disclosure only (spec Non-Goals).
- **Detecting secrets in fields not declared sensitive**, for example by field name. A plain field gets no protection and no warning (spec Non-Goals). The static check over the bundled examples applies only to the examples' own sources.
- **Preventing a leak after an explicit `Reveal()`.** Plugin or application code that unwraps and then logs is covered by guidance in the plugin developer guide only (spec Non-Goals).
- **Redacting the user's own source text in parse and validation diagnostics.** A literal secret written in a file can appear in the source excerpt a diagnostic shows (spec Non-Goals).
- **Pluggable masking, state encryption, the plaintext-state warning and turning event masking off.** Events always show the marker and state holds real values. `WithStateMask`, `WithEventMask`, `WithNoEventMask` and the `mask` package belong to the masking spec, `20261003153421-9fa72edd-masking`. That spec builds on the save sites and the event encoding this plan establishes.
- **Declaring a `variable` sensitive.** There is no `sensitive` attribute on variables. A sensitive value passed into a module input stays sensitive through its mark, and a plain variable feeding a sensitive field is wrapped on assignment.
- **Arbitrary `Sensitive[T]` instantiations on plugin types.** Plugins support a fixed set, because Go cannot build generic types at run time. Application-registered types may use any `T` that cty can represent.
- **Showing references as written and keeping user-written `depends_on`.** These belong to `20261003153421-bf87d907-references-as-written` and `20261003153421-c283547c-user-depends-on`.
- **Cleaning up stale content on the site beyond what this feature changes**, such as the old `prettylog.Handler` form quoted on the events page.

## Changelog

### 2026-10-05 — Task: Add the sensitive value type

**What was done**: Added `types.Sensitive[T]` with `NewSensitive`, `Reveal`, and redacting `String`/`GoString`/`Format`/`LogValue`/`MarshalText`/`MarshalJSON`; `UnmarshalJSON` reads the real value or, for the bare marker, a redacted value. Added the sealed `SensitiveValue` interface, `IsRedacted`, `SensitiveMarker` and `SensitiveMark`.

**Deviations**: None. The external sealed-interface test lives in its own `types_test` file.

**Files changed**:
- `xclconfig: types/sensitive.go`
- `xclconfig: types/sensitive_test.go`
- `xclconfig: types/sensitive_sealed_test.go`

**Discoveries**: `SensitiveMark` is declared as `any` holding an unexported struct value so it is comparable and unforgeable as a cty mark.

### 2026-10-05 — Task: Carry sensitive values through configuration evaluation

**What was done**: Added a generic wrapper hook to the copied `gocty` (`RegisterWrapper`): a wrapper implies and converts as its inner type, is marked on the way into cty and unmarked and wrapped on the way out. A marked value meeting any other Go target now returns a path error instead of panicking. `types` registers `Sensitive` with `SensitiveMark`. The parser's computed and configured-value walks treat a sensitive value as one leaf.

**Deviations**: `gohcl` attribute-decode diagnostics did not name the attribute, so `decodeBodyToStruct` now prefixes their detail with `Attribute "<name>":` (generic, recorded in `internal/xcl/UPSTREAM.md`). Test fixture types `Secret` and `SecretConsumer` were added to `internal/test_fixtures/registered`.

**Files changed**:
- `xclconfig: internal/cty/gocty/wrapper.go`
- `xclconfig: internal/cty/gocty/type_implied.go`
- `xclconfig: internal/cty/gocty/in.go`
- `xclconfig: internal/cty/gocty/out.go`
- `xclconfig: internal/cty/gocty/wrapper_test.go`
- `xclconfig: internal/cty/UPSTREAM.md`
- `xclconfig: internal/xcl/gohcl/decode.go`
- `xclconfig: internal/xcl/UPSTREAM.md`
- `xclconfig: types/sensitive.go`
- `xclconfig: internal/parser/computed.go`
- `xclconfig: internal/parser/configured_check_test.go`
- `xclconfig: internal/convert/sensitive_test.go`
- `xclconfig: internal/test_fixtures/registered/types.go`
- `xclconfig: internal/test_fixtures/config/sensitive/`
- `xclconfig: config_sensitive_eval_test.go`

**Discoveries**: A sensitive value inside a module `variables` object leaves the container unmarked (only leaves are marked), so `AsValueMap` does not panic; the plan's open question on this resolves without extra unmarking. Outputs holding a marked value still panic in `convertCtyToGo` until the output-values task.

### 2026-10-05 — Task: Keep real sensitive values on every internal hop

**What was done**: Added `internal/wire`, a reflect-based encoder that follows `encoding/json`'s field rules but writes each `types.Sensitive` as its real value, and delegates whole to `encoding/json` for any type that cannot hold one. Provider calls, change detection, the host state callback, query conversion and the plugin test helpers now use it. Both state save sites encode each entity through the new `parser.EncodeForState` and hand the store `json.RawMessage` elements.

**Deviations**: `types/resource_helpers_test.go`'s schema test moved to an external test package (`types/resource_helpers_schema_test.go`), because `internal/schema` now imports `internal/wire`, which imports `types`, and the internal test would form an import cycle. The custom-store and provider-failure tests were adjusted to decode the raw records with `savedentity.DecodeAll`.

**Files changed**:
- `xclconfig: internal/wire/wire.go`
- `xclconfig: internal/wire/wire_test.go`
- `xclconfig: internal/parser/state_encode.go`
- `xclconfig: internal/parser/lifecycle.go`
- `xclconfig: internal/parser/callbacks.go`
- `xclconfig: internal/parser/test_plugin.go`
- `xclconfig: internal/parser/destroy.go`
- `xclconfig: internal/parser/entities.go`
- `xclconfig: internal/schema/unmarshal.go`
- `xclconfig: plugins/adapter.go`
- `xclconfig: plugins/changed.go`
- `xclconfig: plugins/grpc_host_callback.go`
- `xclconfig: plugins/testing/helpers.go`
- `xclconfig: plugins/changed_sensitive_test.go`
- `xclconfig: config.go`
- `xclconfig: state/state_store.go`
- `xclconfig: state/custom_store_test.go`
- `xclconfig: config_validate_test.go`
- `xclconfig: config_sensitive_state_test.go`
- `xclconfig: static_output_test.go`
- `xclconfig: types/resource_helpers_test.go`
- `xclconfig: types/resource_helpers_schema_test.go`

**Discoveries**: `go vet` (run in CI) rejects test structs that repeat a json tag, so json-dominance fixtures must use untagged clashes. `Meta.Properties` is `map[string]any`, so every entity "may hold" a sensitive value and takes wire's reflective walk.

### 2026-10-05 — Task: Show only the marker in event data

**What was done**: `eventData` no longer passes provider-call bytes on as they are when they hold a sensitive value: the pre-call snapshot is read back into the entity's type and re-encoded with `encoding/json`, so sensitive fields show the marker; a snapshot whose redacted and revealed encodings match is returned byte-identical, so plain entities' event data is unchanged. A marshal failure now yields no data rather than the revealing bytes. Doc comments on `events.DataProcessed` and `EncodeSavedEntity` say event data shows the marker.

**Deviations**: Converting processed event data to configuration text needed the configuration-text redaction, so the `gohcl` `ReplaceMarked` option, `xcl.RevealSensitive()` and a `types.RedactedMark` (a second cty mark carried by a value read back from the marker, applied through a new generic `gocty.Wrapper.ExtraMarks`) were implemented here, ahead of the "Redact sensitive values in configuration text" task. `types.RedactedMark` is a public addition not in the plan.

**Files changed**:
- `xclconfig: internal/parser/events.go`
- `xclconfig: internal/parser/events_sensitive_test.go`
- `xclconfig: events/events.go`
- `xclconfig: encode.go`
- `xclconfig: internal/xcl/gohcl/encode.go`
- `xclconfig: internal/xcl/UPSTREAM.md`
- `xclconfig: internal/cty/gocty/wrapper.go`
- `xclconfig: internal/cty/UPSTREAM.md`
- `xclconfig: types/sensitive.go`
- `xclconfig: config_event_sensitive_test.go`

**Discoveries**: Registered types (no provider) emit no start-phase lifecycle event, so start-phase redaction is only reachable end to end through a plugin type; it is covered by direct `eventData` unit tests.

### 2026-10-05 — Task: Support sensitive fields on plugin types

**What was done**: Added `schema.KnownTypes()`, the host's type map with the seven supported `types.Sensitive` instantiations, used by the registry and by the plugin test helpers (which passed nil before, and now also check the `UnmarshalUntyped` error). The schema writer describes a sensitive field by type name only; the reader parses types with a bracket-depth scanner instead of the greedy regex, and an unknown `types.Sensitive[...]` fails naming the field. The registry checks every plugin type can be rebuilt when its host is added, so an unsupported instantiation fails plugin loading naming the type and field. The in-process test plugin gained a `credential` type with sensitive fields.

**Deviations**: The load-time check sits in `checkHostTypes`, beside the existing name-clash checks, so all problems are joined into one load error. `config_plugin_loading_test.go`'s expected block-type list now includes `credential`.

**Files changed**:
- `xclconfig: internal/schema/known_types.go`
- `xclconfig: internal/schema/serialize.go`
- `xclconfig: internal/schema/deserialize.go`
- `xclconfig: internal/schema/sensitive_test.go`
- `xclconfig: plugins/registry/plugin_registry.go`
- `xclconfig: plugins/registry/sensitive_types_test.go`
- `xclconfig: plugins/testing/helpers.go`
- `xclconfig: plugins/testing/sensitive_test.go`
- `xclconfig: plugins/testing/testdata/credential.xcl`
- `xclconfig: plugins/example/e2e_test.go`
- `xclconfig: internal/parser/test_plugin.go`
- `xclconfig: internal/test_fixtures/plugin/structs/credential.go`
- `xclconfig: config_plugin_loading_test.go`
- `xclconfig: config_credential_sensitive_test.go`

**Discoveries**: `parseAttribute` builds map key types with `reflect.TypeOf(t.MapKey)`, so every map key is `string` regardless of the declared key; pre-existing and left alone.

### 2026-10-05 — Task: Prove sensitive fields round-trip through apply and state

**What was done**: End-to-end tests apply a registered type, the in-process test plugin's `credential` type and the example plugin's person (now with a sensitive `Token`), then reload state through a second configuration or `Load`/`DecodeAll` and reveal the real value. They also check the state file holds real values, the provider receives real values (including the saved copy after reload), and events at both levels show only the marker. The external plugin binary is exercised through Create and Read.

**Deviations**: Tests that saved typed entities straight to a `FileStateStore` (parser lifecycle and registered-type harnesses, the example plugin's `applyPeople`) now encode with `parser.EncodeForState` first, the way `Config` does; without it the store wrote the marker and the next apply saw a change. There is no existing pattern for a full apply through the external binary, so the external plugin is covered with host-level Create and Read calls.

**Files changed**:
- `xclconfig: plugins/example/pkg/person/resource.go`
- `xclconfig: plugins/example/apply_test.go`
- `xclconfig: plugins/example/sensitive_test.go`
- `xclconfig: plugins/example/testdata/people_token.xcl`
- `xclconfig: internal/parser/lifecycle_test.go`
- `xclconfig: internal/parser/registered_types_test.go`
- `xclconfig: config_sensitive_roundtrip_test.go`

**Discoveries**: Any code that saves state itself must go through `parser.EncodeForState`; handing typed entities to a store's `json.Marshal` silently stores the marker, which then shows up as a spurious change on the next apply.

### 2026-10-05 — Task: Reject sensitive values assigned to plain fields during validation

**What was done**: Added validation stage 3, `validateSensitive` (`internal/parser/sensitive_check.go`), between references and properties. Without evaluating, it predicts which parts of each attribute expression are sensitive, following sensitive Go fields, outputs (judged recursively from their own value expressions, memoised and cycle-guarded) and module inputs (judged from the parent module block's `variables` item), and reports each sensitive part landing in a field that is not `Sensitive`, `any` or `cty.Value` as a positioned problem naming the field and entity.

**Deviations**: Module blocks themselves are not checked, since their inputs are judged where the module uses them. `config_sensitive_eval_test.go`'s plain-field test now expects the validation message, because validation catches the case before decoding; the decode-time guard stays covered at the `gocty` level.

**Files changed**:
- `xclconfig: internal/parser/sensitive_check.go`
- `xclconfig: internal/parser/sensitive_check_test.go`
- `xclconfig: internal/parser/validate.go`
- `xclconfig: internal/test_fixtures/registered/types.go`
- `xclconfig: internal/test_fixtures/config/sensitive_check/`
- `xclconfig: config_sensitive_validation_test.go`
- `xclconfig: config_sensitive_eval_test.go`

**Discoveries**: A module variable's entity has `Meta.Module` set to the module path, and its module block is keyed `module.<that path>`, which is how a module input is traced back to the expression its parent passes in.

### 2026-10-05 — Task: Keep sensitivity in output values

**What was done**: An output's evaluated value is converted by `convertOutputValue`, which unmarks it with paths and wraps exactly the outermost sensitive parts as `types.Sensitive[string|float64|bool|map[string]any|[]any]`, recording them in the new `types.Output.SensitivePaths`. `types.Output.UnmarshalJSON` wraps those paths again on reload, so a reloaded output reads the same; a marker read back from event data gives a redacted value. Module outputs keep their marks for the caller.

**Deviations**: The wrapping logic lives in `types` and is exposed as `(*types.Output).WrapSensitivePaths()`, a small public method not in the plan, so the parser and `UnmarshalJSON` share one implementation.

**Files changed**:
- `xclconfig: types/output.go`
- `xclconfig: types/output_test.go`
- `xclconfig: internal/parser/util.go`
- `xclconfig: internal/parser/callbacks.go`
- `xclconfig: internal/test_fixtures/config/sensitive_outputs/`
- `xclconfig: config_outputs_sensitive_test.go`

**Discoveries**: None beyond the plan.

### 2026-10-05 — Task: Refuse converting a sensitive field into a plain Go field

**What was done**: `asType` now compares the entity's type with the requested one field by field, by JSON name, before copying (`sensitiveFieldBlocked` in `query_sensitive.go`). A sensitive field meeting a plain field fails with `ErrTypeMismatch`, and `errors.TypeMismatchError` gained a `Field` member whose message reads `entity "<id>" field "<field>" is sensitive, <type> declares it as a plain value`. Allowed conversions copy through the revealing encoder, so sensitive-to-sensitive keeps the real value.

**Deviations**: `All` and `Decode` cannot reach a plain-field type: they resolve the requested Go type through the registry, a registered type's entities are always that exact type, and a plugin type cannot also be registered. They share `asType` with `Find`, `FindByType` and `As`, which are tested in both directions; `All` and `Decode` have no dedicated test.

**Files changed**:
- `xclconfig: query.go`
- `xclconfig: query_sensitive.go`
- `xclconfig: query_sensitive_test.go`
- `xclconfig: errors/query_errors.go`
- `xclconfig: errors/query_errors_test.go`

**Discoveries**: Only plugin entities (anonymous structs rebuilt from a schema) can be converted into a different named Go type, so the sensitive-field guard matters for plugin types.

### 2026-10-05 — Task: Redact sensitive arguments in function errors

**What was done**: Every function the parser offers, built-in or custom, is wrapped once in `getFunctions` by `redactingFunction` (`internal/parser/function_redaction.go`). The wrapper accepts marked arguments, calls the function with them unmarked, marks the result with every argument mark, and on a type-check or call error replaces the text of each sensitive argument value with the marker, keeping a `function.ArgError`'s index. A plain-argument error is returned unchanged.

**Deviations**: None.

**Files changed**:
- `xclconfig: internal/parser/function_redaction.go`
- `xclconfig: internal/parser/function_redaction_test.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/test_fixtures/config/sensitive/function_error/main.xcl`
- `xclconfig: config_function_redaction_test.go`

**Discoveries**: Secrets are replaced longest first, so a secret that contains another is redacted whole.

### 2026-10-05 — Task: Redact sensitive values in configuration text

**What was done**: The code landed ahead, in the event-data task: `gohcl.EncodeOptions.ReplaceMarked` replaces each marked part before `hclwrite` writes it (or unmarks when nil), and `EncodeEntity`/`EncodeSavedEntity` pass a replacement that writes `"(sensitive)"` unless `xcl.RevealSensitive()` is given; a value read back from the marker carries `types.RedactedMark` and stays the marker. This task added tests for live entities, saved state, a sensitive number, and unchanged text for plain entities.

**Deviations**: Code landed with the event-data task (see that entry). `EncodeIntoBody`/`EncodeAsBlock` keep their signatures and write marked values unmarked.

**Files changed**:
- `xclconfig: encode_sensitive_test.go`
- `xclconfig: internal/xcl/gohcl/encode_body_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Redact sensitive values in the resource printer

**What was done**: The printer's `formatValue` recognises `types.SensitiveValue`, including as slice and map elements, and prints the marker, or the real value with the new `logger.WithRevealSensitive(true)` unless the value is redacted. The JSON format uses `encoding/json` by default and the revealing encoder when revealing.

**Deviations**: None.

**Files changed**:
- `xclconfig: logger/pretty_printer.go`
- `xclconfig: logger/pretty_printer_sensitive_test.go`

**Discoveries**: The card format prints only "key" fields whose names contain command, image, networks, ports or volumes, so a sensitive field shows there only under such a name.

### 2026-10-05 — Task: Prove no secret leaks from any output path

**What was done**: Added the end-to-end leak suite `sensitive_leak_test.go` with `knownSecret = "s3cr3t-leak-check-7f1d"` held by a registered type, a test-plugin type, an interpolated consumer and two outputs. One test per capture checks the secret is absent and the marker present: events at both data levels, the slog bridge with text and JSON handlers, a custom `%v` slog handler, in-process plugin log details, `%v`/`%+v`/`%#v` of every entity, a function error, a validation error, `EncodeEntity`/`EncodeSavedEntity`, every printer format, `c.Outputs()` and the output entities' JSON. The suite found a real leak: `fmt` of a `*types.Output` reached the real value through `CtyValue` by reflection. `types.Output` now has a `Format` method that shows a marked `CtyValue` as the marker.

**Deviations**: `types.Output.Format` was added to fix the leak (not in the plan). The validation-error capture asserts the field name and no secret but not the marker, since no value is printed. The card printer shows no field values for these types, so only absence of the secret is asserted. The in-process test plugin's log hooks take fixed arguments, so it logs a sensitive value and a struct holding one rather than the applied entity; the external plugin log path is covered by `%v` formatting, which is how details cross the process boundary.

**Files changed**:
- `xclconfig: types/output.go`
- `xclconfig: sensitive_leak_test.go`
- `xclconfig: internal/test_fixtures/config/sensitive_leak/`

**Discoveries**: Any public type that holds a `cty.Value` carrying a sensitive mark leaks through `fmt`'s reflection unless it formats itself; `types.Output` is the only such public type today.

### 2026-10-05 — Task: Declare the examples' secrets sensitive

**What was done**: The application-config example's `Database.Password` and the plugin example's `PostgreSQL.Password` are `types.Sensitive[string]`. The application-config example builds a database URL with `Reveal()` and checks it without printing it; the plugin example's provider calls `Reveal()` only in `connect`, which never logs the address. Tests set known secrets and assert nothing either example prints contains them, that the JSON shows `"password": "(sensitive)"`, and static AST checks require every example field named for a password or secret to be `types.Sensitive` and each such example to call `.Reveal()`.

**Deviations**: The plugin example's configured passwords collided with ordinary words in its output (`password`, `analytics`), so `example/plugin/config/main.xcl`'s `db_password` default is now `"pg-s3cret-example"` and `config/modules/db/db.xcl`'s literal password is `"pg-an4lytics-example"`.

**Files changed**:
- `xclconfig: example/appconfig/resources/resources.go`
- `xclconfig: example/appconfig/main.go`
- `xclconfig: example/appconfig/main_test.go`
- `xclconfig: example/plugin/resources/resources.go`
- `xclconfig: example/plugin/internal/plugin.go`
- `xclconfig: example/plugin/config/main.xcl`
- `xclconfig: example/plugin/config/modules/db/db.xcl`
- `xclconfig: example/plugin/main_test.go`
- `xclconfig: static_examples_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Document sensitive values in the library

**What was done**: The README gained a `## Sensitive values` section (declaring, `NewSensitive`, `Reveal()`, sensitivity following the value, both sensitive-to-plain errors, the marker in the application's own JSON and templates, and the reveal options), and its statements that secrets are shown were replaced; the Modules example now feeds the password from `env`. The plugin developer guide gained `## Sensitive fields` with the supported instantiations, the load error and the warning that a revealed value is no longer protected, cross-linked from its logging section. `docs/plugins.md`, `docs/state.md` and `docs/modules.md` describe log details, state's real values and raw-JSON `Save`, and sensitive module outputs. The changelog has an entry with its breaking changes, and the earlier entry's "secret is shown too" sentence was updated. Content tests guard each part.

**Deviations**: None.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: CHANGELOG.md`
- `xclconfig: docs/plugin-developer-guide.md`
- `xclconfig: docs/plugins.md`
- `xclconfig: docs/state.md`
- `xclconfig: docs/modules.md`
- `xclconfig: readme_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Document sensitive values on the site

**What was done**: Added `src/pages/sensitive-values.mdx`, linked from the Guides navigation, the home page features and the site README, covering declaring, unwrapping, sensitivity following the value, both errors, the marker in the application's own JSON and templates, and what xcl shows and keeps. The home page struct, the application-config page (JSON output now showing the marker, a new "what to notice" point) and the plugins and plugin-logging pages (sensitive password field, `connect` with `Reveal()`, the new `db_password` default) match the examples. `npm ci`, `npm run build` and `astro check` pass.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/pages/sensitive-values.mdx`
- `xcl-website: src/components/Nav.astro`
- `xcl-website: README.md`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: src/pages/examples/application-config.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`
- `xcl-website: src/pages/plugin-logging.mdx`

**Discoveries**: None.
