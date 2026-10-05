---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Context: 20261003134528-327e0657-references-and-secrets

## Current State Analysis

- **Values today**: secrets are plain `string` fields (`example/appconfig/resources/resources.go:95`, `example/plugin/resources/resources.go:24`). They show in state, events, logs, printed JSON and configuration text (`README.md:714-718`, `CHANGELOG.md:19`).
- **cty marks**: the copied go-cty (`internal/cty`, v1.15.0) and HCL (`internal/xcl`, v2.21.0) fully support marks. No xcl code outside them uses marks. `gocty` panics on marked values (`internal/cty/gocty/out.go:242-252` through `internal/cty/marks.go:139-143`).
- **Decode path**: bodies are stored at parse time (`internal/parser/parser.go:901`) and decoded at walk time with `gohcl.DecodeBody` (`internal/parser/callbacks.go:124`). Referenced entities enter the evaluation context through `convert.GoToCtyValue` (`internal/parser/context.go:93`).
- **Validation**: three stages (`internal/parser/validate.go:29-51`). There is no expression-versus-field type check today.
- **Serialisation**: every internal hop uses `encoding/json`. The list of sites is in `research.md`. Event data reuses provider-call bytes (`internal/parser/events.go:37-83`).
- **Plugins**: types are rebuilt on the host with a three-entry type map (`plugins/registry/plugin_registry.go:334-338`).
- **Queries**: one conversion function, `asType` (`query.go:80-107`).
- **Output**: one printer, `logger/pretty_printer.go`, which is unused by the library. One configuration-text encoder, `encode.go` plus `internal/xcl/gohcl/encode.go`.
- **Predecessor**: plan `20261003153421-6ec0eab3` moves `Output` to `types.Output` and makes outputs entities. This plan assumes that has landed.

## Per-Task Technical Notes

Requirement-to-repo resolution: every requirement except the site page lands in `xclconfig` (root `/home/nicj/code/github.com/jumppad-labs/xcl`). The "Sensitive values are documented" requirement's site half and the matching example-output refresh land in `xcl-website` (root `/home/nicj/code/github.com/jumppad-labs/xcl-website`). Line numbers are as of commit `d554c1d`. The predecessor plan moves `internal/resources/output.go` to `types/output.go`, so use whichever exists.

### Task: Add the sensitive value type

- `types/sensitive.go` (new):
  - `SensitiveMarker` and `Sensitive[T any] struct{ value T }`.
  - `NewSensitive`, `Reveal`, `String`, `GoString`, `Format` (writes the marker for every verb, honouring width only), `LogValue`, `MarshalText`, `MarshalJSON` (quoted marker).
  - `UnmarshalJSON`: if the data is exactly the quoted marker, set an unexported `redacted` flag and leave the value zero. Otherwise `json.Unmarshal` into `value`.
  - Sealed `SensitiveValue` interface (`RevealAny() any`, unexported `sensitive()`), implemented on the value receiver.
  - Unexported pointer-receiver `setAny(any) error`, used by the cty hook registration in the next task.
  - `SensitiveMark`: an unexported struct type exposed as one exported value.
  - `RevealAny` of a redacted value returns the zero value. The encoder and printer check an unexported `isRedacted()`, reached through a small exported helper `IsRedacted(SensitiveValue) bool`, so reveal options still print the marker for a redacted value.
- `types/sensitive_test.go` (new): one test per formatting path (`%v`, `%+v`, `%#v`, `%s`, `%q`, `%d`, `%x`, `String`, `slog` text handler, `slog` JSON handler, `MarshalText`, `json.Marshal` of a struct holding one), reveal round-trip, unmarshal of real JSON, unmarshal of the marker. Each is a separate function using `require`.
- **Complexity**: Low
- **Token estimate**: ~25k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Carry sensitive values through configuration evaluation

- `internal/cty/gocty/wrapper.go` (new, MIT header kept): `Wrapper` struct, `RegisterWrapper`, and a lookup `wrapperFor(reflect.Type)`. Registration happens once at init, so it is guarded by a `sync.RWMutex` or written only from `init`.
- `internal/cty/gocty/type_implied.go:30-74`: in `impliedType`, before the Kind switch, a registered wrapper returns the inner type's implied type. This avoids the "no xcl field tags" error at `:83-86`.
- `internal/cty/gocty/in.go:35-70`: in `toCtyValue`, a registered wrapper converts `w.Unwrap(val)` to the target type and returns it `.Mark(w.Mark)`. This covers struct fields reached through `toCtyObject` at `:409`.
- `internal/cty/gocty/out.go:46-107`: in `fromCtyValue`, a registered wrapper target does `val, _ = val.Unmark()` (or `UnmarkDeep`), decodes into `reflect.New(inner)` and calls `w.Wrap(target, inner)`. For any other non-`cty.Value` target, a `val.ContainsMarked()` returns `path.NewErrorf("value is sensitive and cannot be assigned to a field not declared sensitive")` before the primitive conversions that panic (`:242-252`).
- `internal/cty/UPSTREAM.md`: record the wrapper hook and the marked-value error.
- `types/sensitive.go`: an `init()` that calls `gocty.RegisterWrapper` with:
  - `Inner`: `reflect.PointerTo(t).Implements(setterIface)` and `t.Field(0).Type`;
  - `Unwrap`: `RevealAny`;
  - `Wrap`: `setAny`;
  - `Mark`: `SensitiveMark`.
- `internal/xcl/gohcl/decode.go:292-324`: no logic change. A `FromCtyValue` path error already becomes a diagnostic on the attribute's range. Confirm that the diagnostic names the attribute. If it does not, add the attribute name to the summary there, record that in `internal/xcl/UPSTREAM.md` and add the modifications line.
- `internal/parser/context.go:93-97`: the conversion now succeeds for entities with sensitive fields. Add a test that a referenced sensitive field is marked. Watch `AsValueMap` at `:146,164,183,238` and `internal/parser/util.go:212-216`. Only leaves are marked, so containers stay unmarked. Add a test with a module input object holding a sensitive attribute.
- `internal/parser/computed.go:24-25,106-118`: `blockElement` and the leaf set treat any type implementing `types.SensitiveValue` as a leaf. `copyComputed` (`:169+`) copies it whole.
- `internal/parser/configured_check.go:34-63`: compare sensitive leaves with `reflect.DeepEqual` on the whole value. Unexported fields compare correctly. The warning names only the field path, never the value.
- Tests:
  - `internal/cty/gocty/wrapper_test.go` (new): uses a test wrapper type, not `types`, to keep the library test xcl-free;
  - `internal/parser` tests for decode from a literal, a variable and a reference, the marked reference, and the panic-free error;
  - fixtures under `internal/test_fixtures/config/sensitive/`.
- **Complexity**: High
- **Token estimate**: ~70k tokens
- **Agent strategy**: Parallel analysis of `gocty` and parser call sites, then sequential integration: hook first, registration second, parser tests last.

### Task: Keep real sensitive values on every internal hop

- `internal/wire/wire.go` (new): `Marshal` and `MarshalIndent`. The walk is reflect-based:
  - a type that can contain no `SensitiveValue` (decided per type and cached, where any interface kind counts as "may contain") delegates to `json.Marshal`;
  - structs follow json tag rules: name, `omitempty`, `-`, `string`, and embedded struct flattening with Go's dominance rules;
  - a type implementing `json.Marshaler` that is not sensitive delegates to it;
  - a `SensitiveValue` writes `json.Marshal(RevealAny())`.

  The package comment lists the allowed callers.
- `internal/wire/wire_test.go` (new): byte-identical output against `encoding/json` for existing entity shapes (`types.ResourceBase`, `Meta` with `Properties`, the test fixtures' resource structs), plus revealed output for top-level, nested, embedded, slice, map and `any`-held sensitive values.
- Switch to `wire.Marshal`:
  - `internal/parser/lifecycle.go:133,178,183,195,223,286,369`;
  - `internal/parser/callbacks.go:270`;
  - `plugins/adapter.go:127,174,198`;
  - `plugins/changed.go:42`;
  - `plugins/grpc_host_callback.go:90,111`;
  - `internal/schema/unmarshal.go:9`;
  - `plugins/testing/helpers.go:112,131,150,179,190,222,313`;
  - `internal/parser/test_plugin.go:558,563`.
- Unmarshal sites stay `encoding/json`: `internal/parser/lifecycle.go:328,347,414`, `internal/parser/configured_check.go:34,39`, `plugins/adapter.go:100-216`, `internal/savedentity/savedentity.go:75`.
- State save:
  - `config.go:315` and `internal/parser/destroy.go:130` map each entity through `wire.Marshal` into `json.RawMessage` before `Save`, in a shared helper in `internal/parser` that both call.
  - `state/file_state_store.go:92` keeps `json.MarshalIndent`, which re-indents raw messages.
  - Update the contract comment in `state/state_store.go:9-27`.
  - `state/custom_store_test.go:35` is adjusted to expect raw messages.
- `internal/parser/entities.go:89` (`State.Bytes`, test-only): switch to `wire.MarshalIndent` so tests see what state holds.
- `static_output_test.go`: add a check that `internal/wire` is imported only from the allowed packages.
- **Complexity**: High
- **Token estimate**: ~60k tokens
- **Agent strategy**: Single agent writes `internal/wire` and its tests first. Then 2 parallel agents switch call sites, one for parser and state, one for plugins and schema. Run the full suite sequentially at the end.

### Task: Show only the marker in event data

- `internal/parser/events.go:37-83`: `eventData` never returns `pre` unchanged.
  - When `r` is non-nil, it unmarshals `pre` into `reflect.New(reflect.TypeOf(r).Elem())` and re-marshals with `encoding/json`, so the pre-call snapshot is kept but redacted. A success uses `json.Marshal(r)`.
  - The fallback on a marshal error returns nil, not `pre`.
- Update the comments at `events/events.go:30-33` and `encode.go:69-70`: processed event data is what state stores, except that sensitive values show the marker.
- Tests in `internal/parser/error_events_test.go` or a new `internal/parser/event_data_sensitive_test.go`: raw, processed-success and processed-error events each contain the marker and not the value, each in its own function. A plain entity's event data is unchanged. `EncodeSavedEntity` on processed data with a sensitive field succeeds (root `encode_test.go`).
- **Complexity**: Low
- **Token estimate**: ~20k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Support sensitive fields on plugin types

- `internal/schema/types.go` or a new `internal/schema/known_types.go`: `KnownTypes() map[string]reflect.Type` holding:
  - the three existing entries from `plugins/registry/plugin_registry.go:334-338`;
  - `types.Sensitive[string]`, `[int]`, `[int64]`, `[float64]`, `[bool]`, `[[]string]` and `[map[string]string]`, keyed by `reflect.Type.String()`.
- `plugins/registry/plugin_registry.go:332-372`: use `schema.KnownTypes()`.
- `plugins/testing/helpers.go:248` and `plugins/example/e2e_test.go:171,271`: pass `schema.KnownTypes()` instead of nil. Fix the ignored `UnmarshalUntyped` error at `plugins/testing/helpers.go:279`.
- `internal/schema/deserialize.go:133-180`: `parseType` must handle nested brackets. Replace the greedy regex with a bracket-depth scanner for map key and value and generic arguments.
- `internal/schema/deserialize.go:190-271`: a type name starting `types.Sensitive[` that is not in the map returns an error naming the type. Surface it from `createEntityFromPlugins` as a load error naming the plugin type and field.
- `internal/schema/serialize.go:22-84`: write a `Sensitive` field's type name without descending into its unexported `value` property.
- Tests:
  - `internal/schema/deserialzie_test.go` for each supported instantiation and for nested-bracket parsing;
  - `plugins/registry/plugin_registry_test.go` for an unsupported instantiation's load error;
  - a test-plugin type in `internal/parser/test_plugin.go` with a sensitive field.
- **Complexity**: Medium
- **Token estimate**: ~40k tokens
- **Agent strategy**: 2 parallel agents, one on schema parse and serialize, one on registry and helpers. Integrate sequentially.

### Task: Prove sensitive fields round-trip through apply and state

- `config_sensitive_test.go` (new, root): fixtures in `internal/test_fixtures/config/sensitive/`:
  - an application type registered with `RegisterType` holding `Password types.Sensitive[string]`;
  - apply, `Find` and `Reveal`;
  - a second `Config` with the same state path, apply, and `Reveal` again;
  - events are captured with `WithEventData(EventDataProcessed)` and `EventDataRaw`, and asserted to hold no value.
- In-process plugin: reuse the test plugin type from the previous task with `plugins/testing` `InProcessPluginSetupWithEmit`.
- External plugin: `plugins/example` gains a sensitive field on its person resource (`plugins/example/pkg/person/resource.go:10-19`), for example `Token types.Sensitive[string]`. `plugins/example/e2e_test.go` asserts that the round trip reveals it, and that the provider received the real value.
- Each scenario has its own test function, and state comes from a real apply.
- **Complexity**: Medium
- **Token estimate**: ~35k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Reject sensitive values assigned to plain fields during validation

- `internal/parser/sensitive_check.go` (new): `validateSensitive()` runs as a new stage in `internal/parser/validate.go:29-51`, after `validateReferences` (`:102-131`) and before `validateProperties` (`:60-92`).
  - It walks `p.parsedResources.bodies[id]` attributes and nested blocks with the entity's Go type, in the style of `configuredComputedFields` / `configuredComputedValues` (`validate.go:221-346`) and `structFields` / `blockElement` (`computed.go:61-118`).
  - Skip `ResourceBase` / `meta`, and skip targets that are `cty.Value`, `any` or `SensitiveValue`. An output's `value` and a variable's `default` are dynamic and accept anything.
  - A module's `variables` attribute is checked by mapping each object key to the child module's `variable` blocks.
- Expression sensitivity:
  - `hclsyntax.Variables(expr)` (`internal/xcl/hclsyntax/variables.go:14`) gives traversals. Each is rendered as `processScopeTraversal` does (`internal/parser/exp.go:149-182`) and resolved with `p.resolveReference(link, meta.Module)` (`internal/parser/references.go:37-69`).
  - A resolved entity's Go type is walked along the remaining attribute path, in the style of `checkPropertyPath` (`internal/parser/properties.go:33-110`). The walk yields "sensitive", "contains sensitive at paths", or "plain".
  - An output target is judged recursively on its own `value` expression from its stored body, memoised per output ID with an in-progress set to break cycles.
  - A `variable` inside a module is judged by the parent module block's `variables` expression for that key.
- Combination rules:
  - an `*hclsyntax.ScopeTraversalExpr` keeps exact paths;
  - `ObjectConsExpr` and `TupleConsExpr` recurse per item against the field's struct, map or slice element type;
  - anything else (template, function call, operator, conditional, for or splat) is wholly sensitive if any traversal in it reaches any sensitive part.
- Problem: an `errors.ParserError` at the attribute's `Expr.Range()`, with message `field "<field path>" of <entity address> is not declared sensitive and cannot be assigned a sensitive value`. It is collected like `computedFieldProblem` (`validate.go:364-371`).
- Tests (`internal/parser/sensitive_check_test.go`, new), one function each:
  - rejected: direct, interpolated, function, inside an object, inside a list, a module output used by its caller, a module input used inside the module;
  - accepted: each same shape into a sensitive field, a plain value into a sensitive field, and a sensitive value into an output's `value`.
  - The `Apply` refusal test sits in root `config_validate_test.go`.
- Fixtures go in `internal/test_fixtures/config/sensitive_check/`, with a module subdirectory.
- **Complexity**: High
- **Token estimate**: ~80k tokens
- **Agent strategy**: Parallel analysis of the expression kinds and the module-input mapping, then sequential integration. One agent writes the judgement function and its unit tests first, then wires the stage and the fixture tests.

### Task: Keep sensitivity in output values

- `internal/parser/util.go:582-613`:
  - `convertCtyToGo` first does `val.UnmarkDeepWithPaths()`, converts the unmarked value as today, then wraps each path whose marks include `types.SensitiveMark`:
    - a string leaf becomes `Sensitive[string]`;
    - a number leaf becomes `Sensitive[float64]`;
    - a bool leaf becomes `Sensitive[bool]`;
    - an object or map becomes `Sensitive[map[string]any]`;
    - a list, tuple or set becomes `Sensitive[[]any]`.
  - It returns the Go value and the sensitive paths as `[][]string`, with indices as decimal strings and `[]` for the root.
- `internal/parser/callbacks.go:189-195`: set `out.Value` and `out.SensitivePaths`.
- `types/output.go` (from the predecessor plan; `internal/resources/output.go:11-17` before it): add `SensitivePaths [][]string \`json:"sensitive_paths,omitempty"\``. Add `UnmarshalJSON` on `*Output`, which decodes with an alias type and then re-wraps `Value` at each recorded path. The `wire` encoder writes the real values. Plain `encoding/json` writes the marker at those paths.
- `internal/parser/context.go:85-87`: `CtyValue` already keeps marks for references to outputs, including `module.<m>.output.<x>`. Add tests.
- Root `config_outputs_sensitive_test.go` (new):
  - an output of an entity with one sensitive and one plain field;
  - a wholly sensitive output;
  - each after reload;
  - a module output into the caller's sensitive field;
  - an interpolated and a function-derived value into a sensitive field revealing the combined string.

  Fixtures go in `internal/test_fixtures/config/sensitive_outputs/`.
- **Complexity**: Medium
- **Token estimate**: ~40k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Refuse converting a sensitive field into a plain Go field

- `query.go:80-107` (`asType`): between `convertibleTo` (`:89`) and `schema.UnmarshalUntyped` (`:94`), add `sensitiveFieldBlocked(from, to reflect.Type) (field string, blocked bool)`. It walks both types in parallel by JSON field name, flattening embedded structs as `internal/parser/computed.go:61` does and recursing into nested structs, slices and maps. It reports the first field where `from` implements `types.SensitiveValue` and `to` does not, unless `to` is `any` or `cty.Value`.
  - On a hit, return `&errors.TypeMismatchError{Address, Want, Got, Field: field}`.
  - `internal/schema/unmarshal.go` now uses `wire.Marshal` (from the hop task), so sensitive-to-sensitive keeps the real value.
- `errors/query_errors.go:122-134`: add `Field string`. `Error()` reads `entity %q field %q is sensitive, %s declares it as a plain value` when `Field` is set. `Is` still matches `ErrTypeMismatch`.
- Tests:
  - `query_sensitive_test.go` (new): `Find`, `FindByType`, `All`, `As` and `Decode` each into a plain-field type (rejected, `errors.Is` plus `errors.As` with `Field`) and into a sensitive-field type (accepted, reveals). Use a plugin entity, whose anonymous struct passes `convertibleTo`.
  - `errors/query_errors_test.go`: the message with and without `Field`.
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Redact sensitive arguments in function errors

- `internal/parser/parser.go:1333-1343` (`getFunctions`): wrap every `function.Function` in the map, built-in and `options.CustomFunctions`, with `redactingFunction(name, fn)` in a new `internal/parser/function_redaction.go`.
  - The wrapper's spec copies the parameters with `AllowMarked: true` and keeps `AllowUnknown`, `AllowNull` and `AllowDynamicType`.
  - Its `Impl` unmarks the arguments deep, collects marks, and calls the inner `fn.Call(unmarkedArgs)`.
  - On error, if any argument carried `types.SensitiveMark`, it replaces each sensitive argument's string form (`ctystrings`, or the `fmt` of `AsString`, `AsBigFloat` or `True`) in the error text with the marker. It returns a `function.NewArgError` or plain error keeping the argument index.
  - On success it returns the result `.WithMarks(collected)`.
  - The return type func delegates to the inner spec with unmarked argument types (`internal/cty/function/function.go:154-166`).
- `internal/functions/functions.go:281-302`: template-file errors embed template text. They are covered by the wrapper and need no change.
- Tests (`internal/parser/function_redaction_test.go`, new): a failing `file()` on a sensitive path gives the marker and not the value; a failing `file()` on a plain path gives an unchanged message; a successful `trim()` on a sensitive value gives a marked result. Each is its own function.
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Redact sensitive values in configuration text

- `internal/xcl/gohcl/encode.go:17-30`: add `ReplaceMarked func(cty.Value) cty.Value` to `EncodeOptions`.
  - In `attributeValue` (`:335-365`), after `gocty.ToCtyValue`, if `val.ContainsMarked()`:
    - with `ReplaceMarked` nil, unmark deep;
    - otherwise apply `cty.Transform` over the value, replacing each marked node with `ReplaceMarked(node)`, then unmark deep.
  - `EncodeIntoBody` (`:103`) and `EncodeAsBlock` (`:118`) pass the option through.
  - Keep the MPL header, add `// Modifications Copyright (c) Jumppad Labs`, and record the change in `internal/xcl/UPSTREAM.md`.
- `encode.go:21-40`: add `revealSensitive bool` to `encodeOptions` and `RevealSensitive() EncodeOption`. At `:140-143`, pass `ReplaceMarked` returning `cty.StringVal(types.SensitiveMarker)` unless revealing. When revealing, pass nil, except that a redacted value (from event data) still yields the marker: the wrapper's `Unwrap` returns the zero value, so check `types.IsRedacted` in a `ReplaceMarked` that is set in both modes.
- `EncodeSavedEntity` (`encode.go:82-102`) reads real JSON from state, or the marker from event data, through `savedentity.Decode`. It needs no other change.
- Tests in `encode_test.go`: `TestEncodeEntityShowsSensitiveAsMarkerByDefault`, `TestEncodeEntityRevealsSensitiveWhenAsked`, `TestEncodeSavedEntityShowsSensitiveAsMarkerByDefault`, `TestEncodeSavedEntityFromEventDataShowsMarkerEvenWhenRevealing`. Gohcl unit tests go in `internal/xcl/gohcl/encode_body_test.go`.
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Redact sensitive values in the resource printer

- `logger/pretty_printer.go:29-74`: add `RevealSensitive bool` to `PrinterOptions` and `WithRevealSensitive(reveal bool) PrinterOption`.
- `logger/pretty_printer.go:176-219` (`formatValue`): before the kind switch, a `types.SensitiveValue` (also reached through slice and map elements at `:195,205`) prints `types.SensitiveMarker`, or `fmt.Sprint(RevealAny())` when revealing and the value is not redacted.
- `logger/pretty_printer.go:400-500` (`addResourceFields`, `addEmbeddedFields`) and `:629-700` (tree): make sure a sensitive field is not recursed into. `getFieldEmoji` (`:772`) treats it as a scalar.
- `logger/pretty_printer.go:516-518` (`printJSON`): `json.MarshalIndent` by default, and `wire.MarshalIndent` when revealing.
- Tests in `logger/pretty_printer_test.go` (new or existing): marker by default for table, tree, card and json, and the real value with reveal. Each format and mode is its own function.
- **Complexity**: Low
- **Token estimate**: ~20k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Prove no secret leaks from any output path

- `sensitive_leak_test.go` (new, root): const `knownSecret = "s3cr3t-leak-check-7f1d"`. Fixture `internal/test_fixtures/config/sensitive_leak/` holds:
  - a registered type with a sensitive field;
  - a test-plugin type with a sensitive field;
  - a sensitive output;
  - a resource that interpolates the secret into another sensitive field.

  Each capture below is its own test function, asserting `NotContains(knownSecret)` and `Contains(types.SensitiveMarker)`:
  - all events at `EventDataRaw` and at `EventDataProcessed`, through `events.SlogHandler` with a text and a JSON `slog` handler;
  - a custom `slog.Handler` that formats attributes with `%v`;
  - a provider that logs the entity as a detail with `plugins.Logger(ctx)` (in-process and external, the latter through the example plugin);
  - `fmt.Sprintf("%v", entity)` and `%+v`;
  - an error from a failing function on the secret;
  - an error from a validation failure of a sensitive-to-plain assignment;
  - `EncodeEntity` and `EncodeSavedEntity`;
  - each printer format.
- **Complexity**: Medium
- **Token estimate**: ~35k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Declare the examples' secrets sensitive

- `example/appconfig/resources/resources.go:95`: `Password types.Sensitive[string] \`xcl:"password" json:"password"\``.
- `example/appconfig/main.go:108-186`: the JSON section now shows the marker. Where the example builds a connection or DSN, it uses `Password.Reveal()` without printing it. Add one such use, for example building a DSN passed to a stub, with a comment saying `Reveal()` gives the real value and must not be printed.
- `example/appconfig/main_test.go`:
  - add `TestAppConfigExamplePrintsNoSecret`, which sets `DB_PASSWORD` to a known value and checks stdout and stderr;
  - add `TestAppConfigExampleWritesTheMarkerForThePassword` on the JSON section;
  - keep `TestRunWithoutReceiverWritesNothingToStdoutOrStderr` (`:394`) passing.
- `example/plugin/resources/resources.go:24`: `Password types.Sensitive[string]`. The plugin type crosses the external boundary, so it relies on the plugin-type task's type map.
- `example/plugin/internal/plugin.go:81-154`: where the provider "connects", it uses `db.Password.Reveal()` in the connection it builds. The connection string it logs stays password-free (`:114`).
- `example/plugin/external/main.go:82-83`: unchanged, no password.
- `example/plugin/config/main.xcl:9-34` and `config/modules/db/db.xcl:11`: no change needed, because plain values into sensitive fields are allowed. Keep `variable.db_password`.
- `example/plugin/main_test.go`: add `TestPluginExamplePrintsNoSecret`, checking `password` and `analytics` against stdout and stderr. Update any expectations that change.
- `example/prettylog/prettylog.go:84-107`: no change. `EncodeSavedEntity` on event data now shows the marker.
- `static_examples_test.go`: add `TestExampleSecretFieldsAreSensitive`, an AST check that every struct field in `example/*/resources/*.go` whose name contains `Password` or `Secret` (case-insensitive) has type `types.Sensitive[...]`. Add `TestExamplesUsingSecretsCallReveal`, which checks that each example declaring one calls `.Reveal()`.
- **Complexity**: Medium
- **Token estimate**: ~40k tokens
- **Agent strategy**: 2 parallel agents, one on appconfig, one on plugin. The static tests come after.

### Task: Document sensitive values in the library

- `README.md`:
  - add a `## Sensitive values` section after "Converting to configuration text" (`:653`), with these subsections:
    - `### Declaring a sensitive field`
    - `### Reading the real value`
    - `### When a sensitive value meets a plain field`
    - `### Your own JSON and templates`
    - `### Showing real values`

    It shows `types.Sensitive[string]`, `types.NewSensitive(`, `.Reveal()`, the validation error, `ErrTypeMismatch` with the field, the marker `(sensitive)` in `json.Marshal` and templates, `xcl.RevealSensitive()` and `logger.WithRevealSensitive(true)`.
  - Replace the "Values are shown as they are held, so anything secret is shown too" text (`:714-718`).
  - Change the Modules example `db_password = "topsecret"` (`:1093`) to show it feeding a sensitive field.
- `docs/plugin-developer-guide.md`: add a `## Sensitive fields` section after "Kinds of field" (`:55`). It covers the supported instantiations, the load error for others, using `Reveal()` only where needed, and the sentence "Once you call `Reveal()`, the value is an ordinary value: it is no longer protected and must not be logged or otherwise emitted." Cross-link from "Logging from a provider" (`:400-428`).
- `docs/plugins.md:304` ("Across the process boundary"): note that sensitive log details cross as the marker.
- `docs/state.md:120-183`: say that state stores real sensitive values, that `StateStore.Save` receives raw JSON, and that `EncodeSavedEntity` shows the marker.
- `docs/modules.md`: note that sensitive outputs stay sensitive for the caller.
- `CHANGELOG.md`: add a top entry `## 20261003134528-327e0657-references-and-secrets`, with prose covering the feature and `**Breaking:**`:
  - `StateStore.Save` receives each entity as `json.RawMessage`;
  - event data, at both levels, shows sensitive values as the marker, and processed event data is no longer byte-identical to state for those entities.
- `readme_test.go`, one `require.Contains` per test:
  - `TestReadmeDocumentsSensitiveValues` checks `"## Sensitive values"`;
  - `TestReadmeDocumentsRevealingASensitiveValue` checks `".Reveal()"`;
  - `TestReadmeDocumentsSensitiveToPlainErrors`;
  - `TestReadmeDocumentsTheRevealSensitiveOption` checks `"xcl.RevealSensitive()"`;
  - `TestReadmeNoLongerSaysSecretsAreShown` checks `NotContains` on "anything secret is shown too";
  - `TestPluginGuideWarnsThatRevealedValuesAreUnprotected` reads `docs/plugin-developer-guide.md`;
  - `TestChangelogRecordsSensitiveValues` checks the heading;
  - `TestChangelogListsTheStateStoreBreakingChange`.
- **Complexity**: Medium
- **Token estimate**: ~35k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Document sensitive values on the site

- `xcl-website:src/pages/sensitive-values.mdx` (new): layout `../layouts/Shell.astro`, `Hero` and markdown sections mirroring the README's Sensitive values section: declaring, `Reveal()`, both errors, the marker in the application's own JSON and templates, and the reveal options. Code blocks use expressive-code titles.
- `xcl-website:src/components/Nav.astro:8-25`: add "Sensitive values" to the Guides dropdown.
- `xcl-website:README.md`: add the page to the Pages table.
- `xcl-website:src/pages/index.mdx:40-80`: the PostgreSQL struct shows `Password types.Sensitive[string]`. Optionally link the new guide from the lists at `:185,201-217`.
- `xcl-website:src/pages/examples/application-config.mdx:111,171-173,331+,423`: update the struct, the "Run it" output (the JSON shows `"password": "(sensitive)"` if quoted) and "What to notice".
- `xcl-website:src/pages/examples/plugins.mdx:55-80,375-450`: update the resource struct and the quoted output to match what the plugin example now prints.
- Gate: `npm ci`, then the build and `astro check` in `xcl-website`.
- **Complexity**: Low
- **Token estimate**: ~25k tokens
- **Agent strategy**: Single agent, sequential execution.

## Testing Strategy

Every test uses testify `require`. There are no table-driven tests, and positive and negative cases go in separate functions. State comes from a real `Apply`. The overall strategy is in plan.md's Testing Approach. Per task:

- **Add the sensitive value type**: unit tests in `types/sensitive_test.go`, one per formatting path, plus reveal and unmarshal.
- **Carry sensitive values through configuration evaluation**: library-level wrapper tests in `internal/cty/gocty` with a test wrapper type. Parser tests cover decode from a literal, a variable and a reference, the marked reference, and the panic-free error.
- **Keep real sensitive values on every internal hop**: byte-identical and reveal tests for `internal/wire`, the state contract test, the provider receives the real value, and a sensitive-only change is detected. The full existing suite is the regression gate.
- **Show only the marker in event data**: raw, processed-success and processed-error data each hold the marker. Plain entities are unchanged. Processed data still encodes.
- **Support sensitive fields on plugin types**: schema parse tests per instantiation, nested brackets, and the unsupported-instantiation load error.
- **Prove sensitive fields round-trip through apply and state**: end-to-end tests for an application type, an in-process plugin and an external plugin.
- **Reject sensitive values assigned to plain fields during validation**: one rejected and one accepted test per expression shape, the module output and module input cases, and the `Apply` refusal.
- **Keep sensitivity in output values**: partly sensitive, wholly sensitive, after reload, a module output into a sensitive field, and interpolated or function-derived values.
- **Refuse converting a sensitive field into a plain Go field**: each lookup form rejected and accepted, plus the error message tests.
- **Redact sensitive arguments in function errors**: a sensitive failing call, a plain failing call, and a sensitive successful call.
- **Redact sensitive values in configuration text**: default, reveal and saved-data cases, plus event data with reveal.
- **Redact sensitive values in the resource printer**: each format by default and with reveal.
- **Prove no secret leaks from any output path**: the leak suite, one function per capture.
- **Declare the examples' secrets sensitive**: a no-secret test per example and the static checks.
- **Document sensitive values in the library**: README, plugin guide and CHANGELOG content tests.
- **Document sensitive values on the site**: build and `astro check`, plus the manual browser review in the implementation test plan.

## Project References

- Spec: `20261003134528-327e0657-references-and-secrets` (epic `20261003134528-327e0657-references-and-secrets`).
- Upstream plan: `20261003153421-6ec0eab3-module-boundary-and-output-entities`.
- Downstream specs: `20261003153421-bf87d907-references-as-written`, `20261003153421-c283547c-user-depends-on`, `20261003153421-9fa72edd-masking`.
- Design documents: none.
- Knowledge (`xclconfig` store):
  - `gotchas/cty-unknown-args-drop-marks.md`
  - `gotchas/custom-marshaljson-changes-internal-hops.md`
  - `gotchas/plugin-types-rebuilt-with-structof.md`
  - `gotchas/xcl-tags-gate-what-reaches-cty.md`
  - `learnings/state-save-and-load-points.md`
  - `architecture/shared-public-types-live-in-types.md`
  - `conventions/never-modify-dependencies.md`
  - `conventions/shared-errors-package.md`
  - `conventions/testing-and-mocking.md`
  - `conventions/test-state-from-real-apply.md`
- Repo roots: `xclconfig` is at `/home/nicj/code/github.com/jumppad-labs/xcl`, and `xcl-website` is at `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The High tasks (configuration evaluation, internal hops, validation stage) are each 60-80k tokens. Run them as parallel analysis followed by sequential integration, and run the full test suite after each.

## Migration Notes

- **State files**: no migration. Existing state holds plain values. A field changed from `string` to `Sensitive[string]` reads the same JSON string, because `UnmarshalJSON` accepts the real `T`.
- **Custom `StateStore` implementations**: `Save` now receives `json.RawMessage` elements. A store that persisted with `json.Marshal` keeps working unchanged. A store that inspected typed entities must decode the raw JSON instead. This is listed under **Breaking** in the changelog.
- **Event consumers**: `Event.Data` for entities with sensitive fields carries the marker. `EncodeSavedEntity` still reads it.
- **External plugins**: a plugin that adds a `Sensitive` field must be rebuilt against this version so that its schema type names match the host's map. Plugins without sensitive fields are unaffected.

## Performance Considerations

- `internal/wire` delegates to `encoding/json` for any type that cannot contain a sensitive value, decided per type and cached, so entities with no sensitive or `any` fields pay one cached type check per marshal. Entities holding `any` (`Meta.Properties`, `Output.Value`) take the reflective walk, which is a little slower than `encoding/json`. These entities are small, and marshalling is not on a hot path compared with provider calls.
- Event data now costs one extra unmarshal and marshal for the pre-call snapshot at `EventDataRaw` and `EventDataProcessed`. At the default `EventDataNone`, nothing is encoded.
- The validation stage memoises output sensitivity per output, so validation stays linear in the number of attributes and references.
