---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Research: 20261003134528-327e0657-references-and-secrets

## Alternatives considered and rejected

### Option: A `sensitive` struct-tag option instead of a type

Rejected by the spec's constraints: sensitivity must be `types.Sensitive[T]` so the value protects itself in loggers xcl does not own. The tag parser would also need changing (`internal/xcl/tags/tags.go:77-78` rejects unknown options).

### Option: A redacting `MarshalJSON` used by every hop

Rejected. Every internal hop serialises with plain `encoding/json`: state save (`state/file_state_store.go:92`), provider calls (`internal/parser/lifecycle.go:133,178,183,195,223,286,369`, `internal/parser/callbacks.go:270`, `plugins/adapter.go:127,174,198`), change detection (`plugins/changed.go:42`), host state callback (`plugins/grpc_host_callback.go:90,111`) and query conversion (`internal/schema/unmarshal.go:9`, called from `query.go:94`). A redacting marshal would lose the real value on each (knowledge `gotchas/custom-marshaljson-changes-internal-hops.md`).

### Option: A process-wide "reveal" switch read by `MarshalJSON`

Rejected. It is global state shared by concurrent applies and loggers, and the sibling masking spec's technical approach says to choose the masking per serialisation call, never through global state.

### Option: json/v2 per-call marshalers (`json.WithMarshalers`)

Rejected. The module targets Go 1.25 (`go.mod:3`), where json/v2 needs a GOEXPERIMENT.

### Option: A parallel `reflect.StructOf` type that swaps each `Sensitive[T]` for `T`

Rejected. `StructOf` does not generate wrapper methods for embedded fields and cannot rebuild every embedded shape the entities use (`types.ResourceBase` is embedded with `xcl:",remain"`), and `any` fields such as `Output.Value` hold sensitive values only known at run time.

### Option: Patch the redacted JSON afterwards from a reflect walk of sensitive paths

Rejected. It needs the same json-tag path logic as a direct walk, plus a second parse.

### Option: A cty capsule type for sensitive values

Rejected. A capsule hides `T`'s cty type, so `resource.db.main.password` would no longer type as a string in references and functions (`internal/cty/capsule.go:79,96`).

### Option: Detect marks by evaluating expressions with unknown placeholders at validation

Rejected. cty drops marks when a function gets an unknown argument (`internal/cty/function/function.go:254-260,288-289,313-314`; knowledge `gotchas/cty-unknown-args-drop-marks.md`). Evaluating with zero values would run user functions.

### Option: Use `Meta.Links` for the static sensitive check

Rejected. Links are a de-duplicated per-entity list with no field association (`types/resource.go:49`, `internal/parser/parser.go:1130-1233`).

### Option: Teach `gocty` or `gohcl` about `types.Sensitive` directly

Rejected. `internal/xcl` is the MPL-licensed fork, where xcl-specific shaping does not belong (knowledge `gotchas/xcl-tags-gate-what-reaches-cty.md`, `conventions/never-modify-dependencies.md`). `internal/cty` cannot import `types` without coupling the copied library to xcl. A small generic wrapper hook in `gocty` and a generic marked-value option in `gohcl`'s encoder keep both libraries xcl-agnostic.

### Option: Keep `StateStore.Save` receiving typed entities and ask custom stores to use a special encoder

Rejected. A custom store outside the module would call `json.Marshal` and silently store the marker (`state/state_store.go:9-27`). Pre-encoding to `json.RawMessage` at the two save sites is what `DecodeAll` already accepts, and it is where the masking spec hooks encryption (knowledge `learnings/state-save-and-load-points.md`).

## Chosen approach — evidence

- cty marks exist and propagate through traversal, `GetAttr`/`Index` (`internal/cty/value_ops.go:792-795,839-843`), templates even when unknown (`internal/xcl/hclsyntax/expression_template.go:28-119`), conditionals, for and splat expressions (`internal/xcl/hclsyntax/expression.go:792-879,1394-1712,1762-1924`), function calls with known arguments (`internal/cty/function/function.go:292-346`) and `cty/convert` (`internal/cty/convert/conversion.go:21-29`). Templates mark the result whenever any part is marked, so interpolation is covered at walk time.
- The single decode choke point is `gohcl.DecodeExpression` (`internal/xcl/gohcl/decode.go:292-324`), which calls `gocty.FromCtyValue`. Nested object fields decode through `fromCtyObject` (`internal/cty/gocty/out.go:447-510`), so the wrapper hook belongs in `gocty`, not `gohcl`.
- Marked values reaching a plain Go field panic today: `AsString` and friends call `assertUnmarked` (`internal/cty/marks.go:139-143`, `internal/cty/gocty/out.go:242-252`), and nothing recovers in `internal/parser`. The hook turns that into an error with the path.
- A struct with no `xcl` tags fails `ImpliedType` (`internal/cty/gocty/type_implied.go:76-87`), so `Sensitive[T]` needs the hook before it can be referenced (`internal/parser/context.go:93-97` silently skips a resource that fails to convert) or encoded (`internal/xcl/gohcl/encode.go:335-365`).
- The evaluation context converts every referenced resource with `convert.GoToCtyValue` (`internal/parser/context.go:93`), outputs pass their `CtyValue` through with marks intact (`context.go:85-87`, `gocty/out.go:54-57`), and module inputs keep marks in `SubContext` (`internal/parser/callbacks.go:148-167`). One mark applied in `gocty.ToCtyValue` therefore covers resources, module outputs and module inputs.
- Each attribute's expression is available during validation from `p.parsedResources.bodies` (`internal/parser/parser.go:901`), and `validateStructure` already walks bodies alongside Go field types (`internal/parser/validate.go:221-346`, `computed.go:61-118`). `hclsyntax.Variables` (`internal/xcl/hclsyntax/variables.go:14`) extracts every traversal, including from for and index expressions that `processExpr` skips (`internal/parser/exp.go:26-147`). References resolve with `resolveReference` (`internal/parser/references.go:37-69`).
- Outputs reduce their value to plain Go in `convertCtyToGo` (`internal/parser/util.go:582-613`, called from `callbacks.go:189-195`), which panics on marks today.
- Query conversion is one function, `asType` (`query.go:80-107`), used by `Find`, `FindByType`, `FindOne`, `All`, `As` and `Decode` (`decode.go` reaches it through `entitiesOf`, `query.go:236`). `TypeMismatchError` (`errors/query_errors.go:122-134`) has no field member yet.
- Event data reuses the provider-call bytes `pre` (`internal/parser/events.go:37-83`), so redacting events needs a re-encode of `pre`, not only of `r`.
- Plugin log arguments cross the process boundary through `fmt.Sprintf("%v")` (`plugins/grpc_clients.go:140-157`), so `String`/`Format` redacts them. `events.SlogHandler` passes `Meta` values with `slog.Any` (`events/slog.go:109`), so `LogValue` and `String` both matter.
- Plugin types are rebuilt with a hard-coded type map (`plugins/registry/plugin_registry.go:334-338`); `internal/schema/deserialize.go:133-180` misparses nested brackets such as `types.Sensitive[map[string]string]`; `plugins/testing/helpers.go:248` passes a nil type map (knowledge `gotchas/plugin-types-rebuilt-with-structof.md`).
- Encoder options follow `IncludeComputed()` (`encode.go:21-40`); printer options follow `WithColor(bool)` (`logger/pretty_printer.go:43-74`).

## Files examined

- `types/resource.go:1-86` — `Meta`, `ResourceBase`; `types` imports only the standard library today (it gains `internal/cty` from the predecessor plan).
- `types/register.go` — `RegisteredTypes`, `TypeInfo`.
- `internal/cty/marks.go:27-392` — marks API (`Mark`, `UnmarkDeep`, `UnmarkDeepWithPaths`, `MarkWithPaths`, `ContainsMarked`); recommends an unexported Go type as the mark value (`:184-188`).
- `internal/cty/gocty/type_implied.go:30-124` — `ImpliedType`; untagged struct is an error.
- `internal/cty/gocty/in.go:35-70,350-418` — `ToCtyValue`; per-field call at `:409`.
- `internal/cty/gocty/out.go:46-107,242-252,447-510` — `FromCtyValue`; `cty.Value` targets keep marks.
- `internal/cty/UPSTREAM.md` — changes to the cty copy are recorded here.
- `internal/convert/convert.go:8-24` — `GoToCtyValue`, `CtyToGo`.
- `internal/xcl/gohcl/decode.go:55-236,292-324` — body and expression decode.
- `internal/xcl/gohcl/encode.go:17-30,54-63,335-365` — encoder options and attribute values.
- `internal/xcl/hclsyntax/expression_template.go:28-253` — templates keep marks.
- `internal/cty/function/function.go:154-350` — mark handling in calls; unknown args drop marks.
- `internal/functions/functions.go:17-490` — custom functions; none set `AllowMarked`; errors embed argument values (`:281-302`).
- `internal/parser/context.go:15-246` — evaluation context; `AsValueMap` on containers at `:146,164,183,238`.
- `internal/parser/callbacks.go:74-195,270` — walk decode, outputs, destroy marshal.
- `internal/parser/validate.go:29-371` — validation stages and body walkers.
- `internal/parser/properties.go:33-182` — property path walk over Go types.
- `internal/parser/computed.go:24-25,61-118,169+` — field walks; `blockElement` would treat `Sensitive` as a block.
- `internal/parser/configured_check.go:34-63` — configured-value warning via JSON and `DeepEqual`.
- `internal/parser/lifecycle.go:91-416` — every provider-call marshal and unmarshal.
- `internal/parser/events.go:37-112` — `eventData`, reuse of `pre`.
- `internal/parser/util.go:199-228,582-613` — context variable helpers, `convertCtyToGo`.
- `internal/parser/destroy.go:97-130` — state save during destroy.
- `internal/parser/references.go:37-69` — `resolveReference`.
- `internal/resources/output.go:11-17`, `variable.go:11-15`, `module.go:13-29` — builtin shapes.
- `internal/savedentity/savedentity.go:33-164` — `Decode`, `DecodeAll`, accepts `json.RawMessage`.
- `config.go:225-243,315` — `Outputs()`, state save after apply.
- `state/state_store.go:9-27`, `state/file_state_store.go:58-92` — store contract and file store.
- `query.go:80-135,236` — `asType`, `convertibleTo`, `entitiesOf`.
- `decode.go:57-136` — `Decode` reaches `entitiesOf`.
- `errors/query_errors.go:47,122-134` — `ErrTypeMismatch`, `TypeMismatchError`.
- `encode.go:21-185` — `EncodeEntity`, `EncodeSavedEntity`, `IncludeComputed`.
- `logger/pretty_printer.go:21-934` — the only resource printer (table, tree, card, json); unused, excluded at `static_output_test.go:40-44`.
- `logger/emit.go:63-114`, `events/slog.go:17-113`, `plugins/hclog_adapter.go:57-164`, `plugins/grpc_clients.go:29-157`, `plugins/grpc_host_callback.go:50-137` — log paths.
- `plugins/adapter.go:100-216`, `plugins/changed.go:36-48`, `plugins/plugin.go:65-200` — plugin-side JSON and schema.
- `internal/schema/serialize.go:13-84`, `deserialize.go:15-271`, `unmarshal.go:9-15` — plugin schema and untyped conversion.
- `plugins/registry/plugin_registry.go:58-68,245-268,332-372,737-762` — builtins, entity creation, type map, `TypePath`.
- `plugins/testing/helpers.go:112-313` — test helpers; nil type map at `:248`, ignored error at `:279`.
- `example/appconfig/config/app.xcl:63-65`, `resources/resources.go:95`, `main.go:80,108-186`, `main_test.go` — `env("DB_PASSWORD")`, JSON print of the password.
- `example/plugin/config/main.xcl:9-34`, `config/modules/db/db.xcl:11`, `resources/resources.go:24`, `internal/plugin.go:81-154`, `main.go`, `main_test.go` — `db_password`, module password.
- `example/prettylog/prettylog.go:57-107` — prints `EncodeSavedEntity(..., IncludeComputed())` under each create.
- `static_examples_test.go`, `static_output_test.go` — AST checks over examples and library.
- `README.md:714-718,1093` — "secret is shown too", `db_password = "topsecret"`.
- `CHANGELOG.md:1-60` — entry format; `:19` mentions secrets shown.
- `readme_test.go` — one `require.Contains` per test.
- `docs/plugin-developer-guide.md:55,333,400-437`, `docs/plugins.md:251-357`, `docs/state.md:120-183` — docs to extend.
- `xcl-website:src/pages/index.mdx:40-80,185-217`, `examples/application-config.mdx:111,171-173,331+,423`, `examples/plugins.mdx:55-80,375-450`, `events.mdx:187-229`, `src/components/Nav.astro:8-25`, `README.md` — pages, nav, quoted output.

## External references

- Terraform `sensitive` (cty marks with a `sensitive` mark, redaction in plan output) — the model the spec points to for carrying sensitivity with marks.
- Go `fmt.Formatter`, `fmt.Stringer`, `fmt.GoStringer`, `log/slog.LogValuer`, `encoding.TextMarshaler`, `encoding/json.Marshaler` — the standard interfaces `Sensitive[T]` implements so every formatting path redacts.
- `reflect.StructOf` documentation — no wrapper methods for embedded fields; why the parallel-type encoder was rejected.

## Prior plans / specs consulted

- Plan `20261003153421-6ec0eab3-module-boundary-and-output-entities` — `types.Output` moves to `types` with `CtyValue cty.Value` and `Value any`; outputs are entities; only a direct child's outputs are reachable; module keys use `FQRN.AppendParentModule`; changelog, readme_test and website conventions.
- Spec `20261003153421-9fa72edd-masking` (later in the epic) — owns pluggable maskers, state encryption and `WithNoEventMask`; this plan leaves events always redacted and state in plain text, with save sites ready for masking.
- Specs `20261003153421-bf87d907-references-as-written` and `20261003153421-c283547c-user-depends-on` (later) — change configuration text; not touched here.
- Plan `20260922132517-hcl-encoding-helpers` (via CHANGELOG) — origin of `EncodeEntity`, `IncludeComputed`, event data levels and the promise that processed event data is what state stores.

## Open assumptions

- The gocty wrapper hook can be generic (a recogniser registered by `types`), with no import of `types` from `internal/cty`. If `gocty` cannot reach the inner value without `types` exporting more than planned, STOP and ask before widening the public API.
- `hclwrite` cannot write marked values directly; the gohcl encoder option unmarks before writing.
- A fixed set of plugin instantiations is enough: `Sensitive[string]`, `Sensitive[int]`, `Sensitive[int64]`, `Sensitive[float64]`, `Sensitive[bool]`, `Sensitive[[]string]`, `Sensitive[map[string]string]`, plus the shapes outputs use.
- HCL's own diagnostics do not render evaluated values; only xcl's function errors embed argument values. If a diagnostic is found rendering a value, it gets the same redaction.
- Validation at walk time always sees known values, so the runtime guard in `gocty` never misses marks; the static check is the primary enforcement.

## Drafting assumptions

### Chosen direction: cty marks plus a self-redacting type and a revealing internal encoder (architecture)
- **Decision**: `types.Sensitive[T]` redacts itself through every standard formatting and marshalling interface; sensitivity crosses HCL evaluation as a cty mark applied and removed by a generic `gocty` wrapper hook; validation predicts sensitivity statically from expressions and Go types; internal hops use `internal/wire`; events, encoder and printers redact unless revealing is requested.
- **Rationale**: it follows the spec's technical approach exactly, uses machinery that already propagates marks (templates, functions, conditionals, outputs), and respects the knowledge gotchas on marks, JSON hops and plugin type rebuilding.
- **Rejected**: (B) static analysis only, with Go-side wrapping at decode: derived values such as interpolations could not arrive as sensitive without marks. (C) a redacting JSON form with a separate state envelope: every internal hop would still need changing, and plugins would see the marker. Effort for the chosen direction is High; B was Medium but fails requirements.

### Internal hops use a revealing encoder; `MarshalJSON` redacts (discovery)
- **Decision**: `Sensitive[T].MarshalJSON` writes the fixed marker. A new internal encoder (`internal/wire`) walks values and writes each sensitive value's real value, and every internal hop (state, provider calls both ways, change detection, host state callback, query conversion) uses it. Unmarshalling stays `encoding/json`, with `Sensitive[T].UnmarshalJSON` accepting the real `T`.
- **Rationale**: the spec requires every serialisation except state to show the protected form, and the knowledge gotcha says internal hops must bypass the display form.
- **Rejected**: global reveal switch (global state), json/v2 (Go 1.25), parallel `StructOf` types (embedded fields, `any` fields).

### State stores receive pre-encoded entities (discovery)
- **Decision**: the two save sites encode each entity with the revealing encoder and pass `json.RawMessage` values to `StateStore.Save`.
- **Rationale**: a custom store outside the module cannot reach the internal encoder and would store the marker; `DecodeAll` already accepts `json.RawMessage`; the masking spec hooks in at the same sites.
- **Rejected**: leaving typed entities and documenting a special encoder for custom stores.

### Fixed marker text is `(sensitive)` (discovery)
- **Decision**: the marker is the string `(sensitive)`, exported as `types.SensitiveMarker`. In configuration text it is written as the string literal `"(sensitive)"`.
- **Rationale**: matches the Terraform convention the spec points to, and keeps configuration text parseable.
- **Rejected**: `***` (reads as a real value), an unquoted token (text no longer parses).

### Generic hooks in the copied libraries, not xcl-specific code (discovery)
- **Decision**: `internal/cty/gocty` gains a generic wrapper-type hook that `types` registers for `Sensitive[T]`; `internal/xcl/gohcl`'s encoder gains a generic option for how marked values are written. Changes are recorded in each library's `UPSTREAM.md`.
- **Rationale**: keeps xcl-specific shaping out of the MPL fork and avoids `internal/cty` importing `types`.
- **Rejected**: importing `types` from gocty or gohcl; a capsule type.

### Plugin types support a fixed set of instantiations (discovery)
- **Decision**: plugin types may use `Sensitive[string]`, `Sensitive[int]`, `Sensitive[int64]`, `Sensitive[float64]`, `Sensitive[bool]`, `Sensitive[[]string]` and `Sensitive[map[string]string]`. Any other instantiation on a plugin type fails plugin loading with an error naming the field.
- **Rationale**: Go cannot instantiate generics at run time, so the host's type map must list each one; failing loudly beats the silent empty struct the gotcha describes.
- **Rejected**: supporting arbitrary `T` for plugins (impossible), silently dropping unknown ones (data loss).

### Variables cannot be declared sensitive (discovery)
- **Decision**: no `sensitive` attribute on `variable`. A variable holds whatever it is given; a sensitive value passed into a module input stays sensitive through the mark.
- **Rationale**: the spec declares sensitivity only on Go fields and never mentions variables.
- **Rejected**: adding `sensitive = true` to variables (scope creep).

### A redacted value read back holds no real value (discovery)
- **Decision**: `UnmarshalJSON` of the bare marker string yields a redacted `Sensitive[T]` whose `Reveal()` returns the zero value, and which still formats as the marker, even when revealing is requested.
- **Rationale**: event data now carries the marker, and `EncodeSavedEntity` must still read it for every `T`, including non-string ones.
- **Rejected**: failing to decode event data (breaks `EncodeSavedEntity` on events), reading the marker as the real value of a `Sensitive[string]` (a reveal would print it as if real).

### Conventions applied (architecture)
- **Decision**: apply testing, real-apply state, code style, shared errors, never-modify-dependencies, project structure, dependencies and structured logging. Drop the database and patterns/architecture conventions (no database, no services or handlers here).
- **Rationale**: these are the conventions the touched surfaces engage.
- **Rejected**: listing every convention.

### Reveal option names (architecture)
- **Decision**: `xcl.RevealSensitive()` for configuration text, following `IncludeComputed()`. `logger.WithRevealSensitive(bool)` for the printer, following `WithColor(bool)`.
- **Rationale**: each follows the option style of the API it extends.
- **Rejected**: a single shared option type, since the two APIs already use different option types.

### Field-level type error extends `TypeMismatchError` (architecture)
- **Decision**: `errors.TypeMismatchError` gains a `Field` member. When it is set, the message names the field and says the value is sensitive. The sentinel stays `ErrTypeMismatch`.
- **Rationale**: the spec calls it a type error, and callers already match `ErrTypeMismatch`.
- **Rejected**: a new `ErrSensitiveField` sentinel. It is a second thing to match for the same failure.

### Outputs record their sensitive paths (data_structures)
- **Decision**: `types.Output` gains `SensitivePaths`, serialised as `sensitive_paths`. Reading an output back from JSON wraps those parts again.
- **Rationale**: an output's `Value` is `any`, so once it is reloaded from state nothing else says which parts were sensitive, and they would come back plain.
- **Rejected**: no record, which leaks after a reload; making `Value` a `cty.Value`, which breaks the predecessor plan's public shape.

### The change to `StateStore.Save` input is listed as breaking (data_structures)
- **Decision**: the changelog lists, under **Breaking**, that `StateStore.Save` now receives `json.RawMessage` elements.
- **Rationale**: a custom store that inspected the typed entities changes behaviour.
- **Rejected**: calling it non-breaking.

### Shared schema type map lives in `internal/schema` (data_structures)
- **Decision**: the registry and `plugins/testing` both read the map from one `internal/schema` function.
- **Rationale**: the test helpers pass a nil map today, so a plugin with a `Sensitive` field would fail differently in tests than in real use.
- **Rejected**: duplicating the map.

### Supported plugin instantiations and the leak-suite secret are fixed in the plan (tasks)
- **Decision**: the plan names the supported plugin instantiations and a single known secret constant for the leak suite.
- **Rationale**: these are implementation constants with no user-facing trade-off.
- **Rejected**: deferring them to implementation.

### Example secrets get a real use through `Reveal()` (tasks)
- **Decision**: each example that holds a password uses `Reveal()` where the value is genuinely needed, for example to build a connection, without printing it.
- **Rationale**: the spec's success metric requires each example that uses a secret to read it with an explicit unwrap.
- **Rejected**: declaring the field sensitive without ever reading it, which would leave the metric unmet.

## Rehydration cues

- `spektacular spec file read 20261003134528-327e0657-references-and-secrets`
- `spektacular plan file read 20261003153421-6ec0eab3-module-boundary-and-output-entities plan`
- `spektacular knowledge read` for `gotchas/cty-unknown-args-drop-marks.md`, `gotchas/custom-marshaljson-changes-internal-hops.md`, `gotchas/plugin-types-rebuilt-with-structof.md`, `learnings/state-save-and-load-points.md`, `gotchas/xcl-tags-gate-what-reaches-cty.md`, `architecture/shared-public-types-live-in-types.md`.
- Re-read `internal/cty/gocty/{in,out,type_implied}.go`, `internal/xcl/gohcl/{decode,encode}.go`, `internal/parser/{context,validate,callbacks,lifecycle,events}.go`, `query.go:80-135`.
