---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Context: 20261011095738-fad05e41-language-surface

## Current State Analysis

- **Variables:** `DefaultOptions` sets `HCL_VAR_` (`internal/parser/parser.go:140-160`), but `Config` builds `ParserOptions` without a prefix (`config.go:344,378,461`, `config_diff.go:42`). `setVariables` (`internal/parser/util.go:167-184`) therefore turns every environment variable into a variable.
- **Functions:**
  - `getFunctions` (`internal/parser/parser.go:1537-1549`) merges 57 built-ins (`internal/functions/functions.go:429-486`) with an always-empty `customFunctions`, silently overriding names. Nothing reaches it from `Config`.
  - `ProviderFunction`/`ProviderFunctions` (`plugins/functions.go`) and `ResourceProvider.Functions()` (`plugins/provider.go:125`) exist, but nothing calls them, and `plugin.proto` has no function RPC.
- **Type clashes:** reported at `NewConfig` for declared types (`internal/catalog/catalog.go:142-171`), and at plugin load for plugin types (`catalog.go:369-406`), as `TypeNameClashError` (`errors/type_errors.go:11-49`).
- **Blocks:**
  - A `map[string]T` block field panics (`internal/xcl/gohcl/schema.go:87-91`).
  - Label counts are all-or-nothing (`internal/xcl/hclsyntax/structure.go:178-218`).
  - A list of blocks becomes a cty list, which `GetAttr` cannot index by name (`internal/xcl/ops.go:324-350`).
  - Several other gohcl panic sites are reachable from user Go types (`schema.go:153,173,179`; `decode.go:51,128,271,297`; `check.go:117`).
- **Optional fields** in object and map attribute values are required, because `impliedStructType` (`internal/cty/gocty/type_implied.go:81-129`) never marks attributes optional.
- **`for` references:** `processExpr` (`internal/parser/exp.go:28-149`) has no `ForExpr` case, so the referenced entity never enters the eval context.
- **Template directives** parse and evaluate (`internal/xcl/hclsyntax/parser_template.go:477-656`).
- **Variable defaults** already link references (`internal/parser/parser.go:924-957,1045`), but no test covers it.
- **Other repos:**
  - xcl-website claims `HCL_VAR_` and "around sixty" functions.
  - xcl-vscode has no `%{` rule.
  - The template and docker plugins pin xcl `v0.1.1`, and their `Functions()` return nil.

## Per-Task Technical Notes

### Requirement → repo and files

| Requirement | Repo | Main files |
|---|---|---|
| Environment overrides use the XCL prefix / no accidental overrides | xcl | `config.go`, `config_diff.go`, `internal/parser/parser.go`, `internal/parser/util.go`, `example/plugin/*_test.go` |
| Applications can register their own functions | xcl | `registry/registry.go`, `registry/local.go`, `registry/github.go`, `internal/catalog/catalog.go`, `internal/functions/functions.go`, `internal/parser/parser.go`, `errors/`, `config.go` |
| Plugins can offer functions; functions evaluated while config is read | xcl | `plugins/functions.go`, `plugins/plugin.go`, `plugins/plugin_host.go`, `plugins/direct_plugin_host.go`, `plugins/grpc_*.go`, `plugins/plugin.proto`, `plugins/proto/`, `internal/catalog/catalog.go` |
| Function name clashes are reported | xcl | `errors/function_errors.go` (new), `internal/catalog/catalog.go`, `config.go` |
| Named blocks into a map; unnamed into map fails; duplicates; lists accept both | xcl | `internal/xcl/schema.go`, `internal/xcl/hclsyntax/structure.go`, `internal/xcl/gohcl/{schema,decode,check,encode}.go` |
| Unsupported field types are errors | xcl | `internal/xcl/gohcl/*.go`, `internal/catalog/catalog.go` |
| Named blocks referenced by name | xcl | `internal/parser/parser.go`, `internal/parser/diff_decode.go`, `internal/parser/context.go` (unchanged, verified) |
| Optional fields in map/object values | xcl | `internal/xcl/gohcl/decode.go`, `internal/cty/gocty/out.go` |
| References inside `for` resolve | xcl | `internal/parser/parser.go`, `internal/parser/exp.go` |
| Template directives rejected | xcl | `internal/xcl/hclsyntax/parser_template.go` |
| Variables built from other values | xcl | test only: `internal/parser/` and the root package |
| Editor highlighting matches | xcl-vscode | `test/tokenize.test.js`, `test/fixtures/sample.xcl`, `CHANGELOG.md` |
| Site matches the language | xcl-website | `src/pages/index.mdx`, `src/pages/examples/configuration-only.mdx`, `src/pages/examples/plugins.mdx`, `src/pages/plugin-template.mdx` |
| Plugin template supports functions; docker builds | xcl-plugin-template, xcl-plugin-docker | `providers/note.go`, `examples/basic/main.xcl`, `e2e/`, `README.md`, `go.mod` |

### Task: Environment overrides use the XCL_VAR_ prefix

- `internal/parser/parser.go:71-131,140-160`:
  - add the exported `const DefaultVariableEnvPrefix = "XCL_VAR_"`;
  - `DefaultOptions` uses it;
  - update the doc comment, which currently says `HCL_VAR_`.
- `internal/parser/util.go:167-184`, `setVariables`:
  - return without reading the environment when `variableEnvPrefix == ""`;
  - replace `strings.Split(e, "=")` with `strings.Cut(e, "=")`, so values containing `=` are kept;
  - strip the prefix with `strings.TrimPrefix`, not `strings.Replace`.
- `config.go:337-489`: add `func (c *Config) parserOptions(emit events.Emit) *parser.ParserOptions`. It returns `EventData`, `StateStore`, `StateMask`, `EventMask`, `Catalog`, `Variables` (`convertVariablesToStringMap`), `Emit` and `VariableEnvPrefix: parser.DefaultVariableEnvPrefix`. Use it at `config.go:344` (Validate), `:378` (Apply) and `:461` (Destroy).
- `config_diff.go:41-49`: use `c.parserOptions(emit)`.
- `internal/parser/parse_test.go:234-252`: change `HCL_VAR_cpu_resources` to `XCL_VAR_cpu_resources`, and use `t.Setenv`.
- `example/plugin/main_test.go:80,87,244` and `example/plugin/smoke_test.go:57,99`: change `HCL_VAR_output_dir` to `XCL_VAR_output_dir`.
- New `config_variables_env_test.go` (root package). Each test is its own function, using `t.Setenv` and an inline config written with `t.TempDir()` in the style of `config_delivery_test.go:70`:
  - `TestApplyOverridesVariableFromXCLVarPrefixedEnvironment`
  - `TestApplyIgnoresUnprefixedEnvironmentVariable`
  - `TestApplyIgnoresHCLVarPrefixedEnvironmentVariable`
  - `TestValidateReportsEnvironmentVariableWithoutPrefixAsUnknownVariable` (`variable.PATH`; if Validate does not evaluate, assert on the Apply error instead)
  - `TestDiffReadsPrefixedEnvironmentVariable`
- New `internal/parser/util_test.go` cases: an empty prefix reads nothing, and a value containing `=` is kept whole.
- Complexity: Low. Token estimate: ~25k. Agent strategy: single agent, sequential.

### Task: References inside every kind of expression are found

- `internal/parser/parser.go:1311-1400` (`getDependentResources`) and `:1407-1440` (`getDependentResourcesFromBlock`): replace `processExpr(a.Expr, p.isReferenceRoot)` at :1317 and :1412 with a new `referencesIn(expr hclsyntax.Expression, isRoot func(string) bool) ([]string, error)`. It iterates `hclsyntax.Variables(expr)` (`internal/xcl/hclsyntax/variables.go:14`) and reduces each traversal with `processScopeTraversal(&hclsyntax.ScopeTraversalExpr{Traversal: t, SrcRange: t.SourceRange()}, isRoot)`, following `internal/parser/sensitive_check.go:289-298`. Keep the output order deterministic, sorted by source position as `processExpr` produced it.
- `internal/parser/exp.go:28-149`: delete `processExpr` once it has no callers. Keep `processScopeTraversal` (:151-184), which `sensitive_check.go:309` uses.
- Check that the references recorded as written (`internal/parser/references_written_test.go`) and `Meta.Links` are unchanged for the existing fixtures. Links are attribute paths, not entity addresses (`architecture/links-are-attribute-paths`).
- New fixtures:
  - `internal/test_fixtures/config/for_references/main.xcl`, using registered types in `internal/test_fixtures/registered/types.go`. Add an `App` with `Listeners []Listener` if none is suitable; the map form comes in task 35103d66.
  - `internal/test_fixtures/config/variable_from_entity/main.xcl`.
- New tests:
  - `internal/parser/dependencies_test.go` (style at :22): `TestForExpressionOverEntityIsADependency`, `TestIndexExpressionReferenceIsADependency`, `TestParenthesisedReferenceIsADependency`, `TestForExpressionKeyAndValueNamesAreNotReferences`.
  - `internal/parser/parse_test.go`: `TestApplyEvaluatesForExpressionOverEntity` and `TestApplyResolvesVariableDefaultBuiltFromEntity`.
  - Root package `config_variables_test.go`: `TestApplyBuildsVariableDefaultFromEntity`, for the spec's `variable "url"` acceptance criterion.
- Complexity: Medium. Token estimate: ~40k. Agent strategy: single agent; write the tests first, then switch the extraction.

### Task: Optional fields can be left out of object and map values

- `internal/xcl/gohcl/decode.go:292-324` (`DecodeExpression`): build the conversion type with a new `constraintType(reflect.Type) (cty.Type, error)` in `internal/xcl/gohcl/types.go`.
  - It walks struct, slice, map and pointer types like `gocty.ImpliedType`.
  - For structs it returns `cty.ObjectWithOptionalAttrs(atys, optionalNames)`, where a field is optional when its `xcl` tag kind is `optional` (`tags.Parse` in `internal/xcl/tags/tags.go:56-87`).
  - Use it only for `convert.Convert(srcVal, constraint)` at :300. Values keep `ImpliedType`.
  - Add the MPL modification notice and an `internal/xcl/UPSTREAM.md` entry.
- `internal/cty/gocty/out.go:458-521` (`fromCtyObject`): when an attribute value is null and the target field is a non-pointer field tagged optional, leave the field at its zero value instead of calling `fromCtyValue`, which errors at :76-84. Also make the anonymous-embed recursion at :494-500 return its error. Record both in `internal/cty/UPSTREAM.md`.
- New tests:
  - `internal/xcl/gohcl/decode_optional_test.go` (MPL header, non-table style): `TestDecodeExpressionAllowsOmittedOptionalFieldInObject`, `TestDecodeExpressionAllowsOmittedOptionalFieldInMapOfObjects`, `TestDecodeExpressionRejectsOmittedRequiredFieldInObject`.
  - `internal/cty/gocty/out_test.go`: `TestFromCtyValueLeavesNullOptionalFieldAtZero`.
  - Root package: `TestApplyAllowsOmittedOptionalFieldInMapValue`, for `listeners = { http = { port = 80 } }` with `tls` optional.
- Complexity: Medium. Token estimate: ~35k. Agent strategy: single agent, sequential.

### Task: Template directives are rejected by the parser

- `internal/xcl/hclsyntax/parser_template.go:477-656` (`case TokenTemplateControl:` in `parseTemplateParts`): on the first directive token, append an error diagnostic.
  - Summary: `Template directives are not supported`.
  - Detail: `XCL has no string templating. Use a function such as format, join or template_file, or a conditional expression inside ${ }, to build the string.`
  - Subject: the `%{ … }` range.
  - Then recover by skipping to the matching close so later parsing continues. Keep the `templateIfToken`/`templateForToken` production only if needed for recovery; the result must never evaluate.
  - Add the modification notice. Add an `internal/xcl/UPSTREAM.md` entry ("template directives rejected").
- The lexer (`scan_tokens.rl:181-189`) is unchanged, so `%%{` stays a literal.
- Update the upstream tests that use directives: `internal/xcl/hclsyntax/expression_template_test.go`, `parser_test.go` and `scan_string_lit_test.go`. Rewrite the directive cases to assert the error diagnostic, and keep lexer-only token cases.
- `internal/xcl/hclwrite` and `encode.go:406`: confirm that encoding a string value containing `%{` writes it escaped as `%%{`, and add a test in `encode_test.go` if one is missing.
- `highlight/tokenize.go:69,226,329` stays unchanged (assumption: highlight package left as is).
- New tests:
  - `internal/xcl/hclsyntax/template_directive_test.go` (MPL header, non-table): `TestTemplateIfDirectiveIsAnError`, `TestTemplateForDirectiveIsAnError`, `TestHeredocDirectiveIsAnError`, `TestInterpolationStillParses`, `TestEscapedDirectiveIsLiteral`.
  - Root package: `TestValidateRejectsTemplateDirectiveAtItsPosition` and `TestApplyFillsInterpolatedReference` (`${app.web.image}`).
- Complexity: Medium. Token estimate: ~35k. Agent strategy: single agent; fork change first, then the upstream test rewrite.

### Task: Named child blocks decode into maps and lists

- `internal/xcl/schema.go:8-11` (`BlockHeaderSchema`): add `LabelsOptional bool`.
- `internal/xcl/hclsyntax/structure.go:178-218`: when `blockS.LabelsOptional` and the block has zero labels, accept it. Fewer-than-all but more than zero labels, or too many, keeps today's diagnostics. The same applies in `internal/xcl/merged.go` and in the json body if they enforce label counts; check and mirror.
- `internal/xcl/gohcl/schema.go:77-105` (block loop in `ImpliedBodySchema`):
  - Accept `reflect.Map` with a string key and a struct (or pointer-to-struct) element. This gives `LabelNames: ["name"]` and `LabelsOptional: true`.
  - For a slice of structs whose element has zero or one `label` field, use `LabelNames: [<label name or "name">]` and `LabelsOptional: true`.
  - Single struct and pointer fields, and elements with two or more labels, keep today's rules.
  - A non-struct element becomes an error once the error-returning variant from task 0a9af403 lands. Here, only add the map case.
- `internal/xcl/gohcl/decode.go:136-233` (block loop):
  - Add a map branch. It builds `reflect.MakeMap`. For each block: a missing label gives the diagnostic "A name is required" (detail: "<type> blocks are collected by name, so each one needs a name, e.g. <type> \"http\" { … }", subject: the block's `DefRange`); a duplicate key gives "Duplicate <type> name" naming the label, with subject the second block's `LabelRanges[0]` and a detail pointing to the first. Otherwise the block is decoded into a new element and stored under the label; the element's label field, if any, is set to the key.
  - In the slice branch (:183-216), collect a label only when present, and report duplicate names the same way.
  - `decodeBlockToValue` (:264-276): tolerate fewer labels than label fields when `LabelsOptional`, leaving the field empty.
- `internal/xcl/gohcl/check.go:83-117`: mirror the map element handling, so `CheckBody` (used by `internal/parser/validate.go:224`) validates map blocks. Don't take the `JustAttributes` path for a map of structs.
- `internal/xcl/gohcl/encode.go:585-627` (`encodeBlock`): for a map field, iterate sorted keys and emit one block per entry with the key as its label.
- `internal/xcl/UPSTREAM.md`: add an entry for optional labels and map blocks.
- New tests, in non-table style with MPL headers:
  - `internal/xcl/gohcl/decode_blocks_map_test.go`: `TestDecodeNamedBlocksIntoMap`, `TestDecodeUnnamedBlockIntoMapFails`, `TestDecodeDuplicateBlockNameInMapFails`, `TestDecodeDuplicateBlockNameInListFails`, `TestDecodeNamedAndUnnamedBlocksIntoList`, `TestDecodeMultiLabelBlocksKeepStrictLabelCount`.
  - `internal/xcl/gohcl/check_test.go`: map cases.
  - `internal/xcl/gohcl/encode_body_test.go`: `TestEncodeMapOfBlocksWritesNames`.
  - `internal/xcl/hclsyntax/structure_test.go`: optional label acceptance.
- Complexity: High. Token estimate: ~70k. Agent strategy: parallel analysis of hclsyntax/schema and gohcl decode/check/encode, then sequential integration (schema → structure → decode → check → encode).

### Task: Unsupported Go field types return errors instead of panics

- `internal/xcl/gohcl/schema.go`:
  - Add `func ValidateSchemaType(t reflect.Type) error`. It walks the struct, as `ImpliedBodySchema` and `getFieldTags` do, recursing into block element types, and returns one error per problem, naming the Go type and field.
  - Add the error-returning internal variants `impliedBodySchema(...) (hcl.BodySchema, bool, error)` and `getFieldTags(...) (*fieldTags, error)`. The existing exported `ImpliedBodySchema` keeps its signature but is only called on types already validated.
  - Panic sites to convert: :34 (non-struct), :87-91 (block tag on a non-struct, non-map, non-slice-of-struct field), :153 (invalid tag), :173 and :179 (second `remain`/`body`).
- `internal/xcl/gohcl/decode.go`:
  - :51 (`remain`/`body` of an unsupported type): return a diagnostic.
  - :128-129 (unexported attribute field): skip it, or report it, without `Addr()` on an unexported field.
  - :264-276 (label field not a string, or unexported): diagnostic.
  - :297 (`ImpliedType` failure): diagnostic with the attribute's subject range.
- `internal/xcl/gohcl/check.go:29,117`: return diagnostics instead of panicking.
- `internal/catalog/catalog.go:142-171` (`DeclareType`): call `gohcl.ValidateSchemaType(reflect.TypeOf(prototype).Elem())` after `declaration.Validate`, and return a new `*xclerrors.InvalidTypeError{Type, Registry, Err}` (`errors/type_errors.go`) so that `NewConfig` returns it.
- `internal/catalog/catalog.go:369-406` (`checkHostTypes`): run the same check on the instance rebuilt by `schema.CreateInstanceFromSchema`, and append the problem.
- `internal/resources` builtins: add a test that every builtin type passes `ValidateSchemaType`.
- `config.go:148-157`: re-export `InvalidTypeError`.
- `internal/xcl/UPSTREAM.md`: add an entry ("panics replaced by diagnostics; ValidateSchemaType").
- New tests:
  - `internal/xcl/gohcl/schema_validate_test.go`: one function per former panic case.
  - `internal/catalog/catalog_test.go`: `TestDeclareTypeRejectsStringBlockField`.
  - Root package `config_bad_type_test.go`: `TestNewConfigReturnsErrorForStringBlockField`, which asserts no panic with `require.NotPanics` and `errors.As` to `InvalidTypeError`.
  - Root package: `TestFirstOperationReturnsErrorForPluginTypeWithBadBlockField`, using a small in-process plugin fixture.
- Complexity: Medium. Token estimate: ~50k. Agent strategy: 2 parallel agents (gohcl conversion; catalog and errors wiring), then integrate.

### Task: Entities with named block maps work end to end

- `internal/parser/parser.go:1346` and `:1430`: build written-reference prefixes for map fields as `%s["%s"].` (by name). Keep `%s[%d].` for lists. The field kind comes from the entity's Go type, so pass the resolved field type or look it up with `reflect` on the resource.
- `internal/parser/diff_decode.go:70-85` (`knownBody`): add a `reflect.Map` branch indexing by the block's label.
- `internal/parser/configured_check.go:124-141`: pass the matching block body for map elements by label, instead of `nil`.
- `internal/parser/computed.go:110-120,210-264`: verify that map pairing by key covers map blocks, and add a test.
- `internal/parser/properties.go:64-73,185-234`: verify that `listener.https.port` validates against a map field.
- `internal/parser/context.go:96` (`convert.GoToCtyValue`): no change. A map of blocks implies `cty.Map(Object)` (`internal/cty/gocty/type_implied.go:61-70`) and `GetAttr` indexes maps (`internal/xcl/ops.go:299-321`).
- `internal/test_fixtures/registered/types.go`: add `App` with `Image`, `Host`, `Listeners map[string]Listener \`xcl:"listener,block"\``, and `Listener{Name label, Port, TLS optional}`. Also add a list-form variant for the list criterion. Register them in `setupRegisteredTypes` (`internal/parser/registered_types_test.go:54`) and in the root-package helpers.
- New root-package `config_named_blocks_test.go`, one function per criterion:
  - `TestApplyCollectsNamedBlocksIntoMap`
  - `TestValidateRejectsUnnamedBlockIntoMap`
  - `TestValidateRejectsDuplicateBlockName`
  - `TestApplyCollectsNamedAndUnnamedBlocksIntoList`
  - `TestApplyResolvesReferenceToNamedBlock`
  - `TestApplyEvaluatesForOverNamedBlocks`
  - `TestSecondApplyWithNamedBlocksReportsNoChanges`
  - `TestDiffReportsChangeToOneNamedBlock`

  Use a real apply for state (convention: test state from a real apply).
- Complexity: Medium. Token estimate: ~50k. Agent strategy: single agent; fixtures and tests first, then the path fixes.

### Task: Applications register their own functions

- `errors/function_errors.go` (new): `FunctionNameClashError{Name, Provider, Registry, Existing, ExistingRegistry}` with `Error()`: `function %q is provided by both %s and %s` (`describeProvider` from `errors/type_errors.go:43`). Also `InvalidFunctionError{Name, Registry, Err}` with `Unwrap`. Use pointer receivers (`conventions/shared-errors-package`), and add tests in `errors/function_errors_test.go`.
- `config.go:136-157`: add `FunctionNameClashError` and `InvalidFunctionError` aliases, and document when each is returned. Update `errors_reexport_test.go`.
- `registry/registry.go:26-52`: add `Functions() []Function` to `Registry`, and `type Function struct{ Name string; Func any }`. Update the package doc.
- `registry/local.go:90-120`: add `functions []Function` and `RegisterFunction(name string, fn any)`, which panics only on an empty name or a nil `fn`, as `RegisterType` does for programmer errors. Add `Functions()`.
- `registry/github.go:140-163`: add delegating `RegisterFunction` and `Functions`.
- Every other `Registry` implementation must gain `Functions()`. Find them with `grep -rn "Plugins(ctx context.Context" --include=*.go`, including test fakes.
- `internal/functions/functions.go`:
  - `:17-197`: refactor `CreateCtyFunctionFromGoFunc` onto a shared `goFuncSignature(fn any) (params []paramSpec, ret cty.Type, err error)`. Add bool parameters, `[]T` and `map[string]T` of primitives; name parameters `arg0..n`.
  - `:208`: add `BuiltinNames() []string`, the keys of `GetDefaultFunctions("")`.
- `internal/catalog/catalog.go`:
  - add `functions map[string]catalogFunction`, where `catalogFunction` is `{fn function.Function; provider, registry string}`;
  - add `DeclareFunction(f registry.Function, registryName string) error`, which checks against `functions.BuiltinNames()` and the declared functions, and converts with `CreateCtyFunctionFromGoFunc`, returning `InvalidFunctionError`;
  - add `Functions() map[string]function.Function`, read under `mu`.
- `config.go:241-248`: after the types loop, loop over `r.Functions()` and call `c.catalog.DeclareFunction`, returning the first error.
- `internal/parser/parser.go:129,165,201,220-221,1537-1549`:
  - remove `CustomFunctions` and `customFunctions`;
  - `getFunctions` merges `functions.GetDefaultFunctions(file)` with `p.catalog.Functions()` when the catalog is non-nil. It must not overwrite: names cannot clash after the catalog checks, so a duplicate is a programming error, which a test asserts.
  - keep `redactingFunctions`.
- `internal/parser/functions_test.go:17,88`: switch `constantNumberOptions` to registering on a catalog.
- `internal/parser/function_redaction_test.go:93`: the same.
- New tests:
  - `registry/local_test.go`: `TestRegisterFunctionRecordsFunction`.
  - `internal/catalog/catalog_test.go`: `TestDeclareFunctionClashesWithBuiltin`, `TestDeclareFunctionClashesWithDeclaredFunction`, `TestDeclareFunctionRejectsUnsupportedSignature`.
  - Root package `config_functions_test.go`: `TestApplyCallsApplicationFunction` (`slugify("My App")`), `TestNewConfigReportsApplicationFunctionClashWithBuiltin` (`upper`), `TestNewConfigReportsApplicationFunctionClashAcrossRegistries`, `TestNewConfigReportsInvalidApplicationFunction`, `TestApplicationFunctionErrorHidesSensitiveArgument`.
- Complexity: Medium. Token estimate: ~55k. Agent strategy: 2 parallel agents (errors, registry and catalog; functions refactor and parser), then integrate and run the full suite.

### Task: Plugin providers offer functions

- `plugins/functions.go`:
  - Reshape `ProviderFunction` to `Signature() FunctionSignature` and `Call(ctx, args []byte) ([]byte, error)`. Add `FunctionSignature` and `FunctionParam`.
  - Add `NewFunction(fn any) (ProviderFunction, error)`. It uses the shared reflection from `internal/functions`: a new `functions.GoFuncSignature` and `functions.CallGoFuncJSON(fn, args []byte) ([]byte, error)`. Types are written with `internal/cty/json.MarshalType`.
  - Add `NewFunctions(map[string]ProviderFunction) ProviderFunctions`, implementing `Get`, `Find` (regexp) and `List`.
  - Update the doc comments: functions are offered to configuration.
- `plugins/provider.go:123-125` and `plugins/datasource.go:33`: update the `Functions()` doc.
- `plugins/plugin.go`:
  - `:36-59`: add `RegisteredFunction{Name; Signature}`, and add `GetFunctions() []RegisteredFunction` and `CallFunction(ctx, name string, args []byte) ([]byte, error)` to `PluginEntityProvider`.
  - `:71`: `RegisterResourceProvider` reads `provider.Functions()` and, when non-nil, calls `List()`. It registers each function on `PluginBase` and returns an error for a name already registered in the same plugin.
  - `:100-215`: `PluginBase` gains a `functions` map, `GetFunctions` (sorted by name) and `CallFunction`.
- `plugins/plugin_host.go:9-37`: add `GetFunctions() []RegisteredFunction` and `CallFunction(ctx, name, args)` to `PluginHost`.
- `plugins/direct_plugin_host.go:30-120`: implement both by delegating to the plugin, with the call logger like the lifecycle methods.
- Test doubles: `plugins/mocks` (regenerate with mockery), `internal/parser/test_plugin.go:45,557,771`, `internal/test_fixtures/plugins/subtypeless/main.go:70`, `plugins/example/pkg/person/provider.go:130`, `example/plugin/plugins/{template,docker}`, and `e2e/fixtures/*`. Update anything that implements `PluginHost` or `PluginEntityProvider` directly; most embed `PluginBase`.
- New tests:
  - `plugins/functions_test.go`: `TestNewFunctionDescribesSignature`, `TestNewFunctionCallsGoFunc`, `TestNewFunctionReturnsGoFuncError`, `TestNewFunctionRejectsUnsupportedParameter`.
  - `plugins/plugin_test.go`: `TestRegisterResourceProviderCollectsFunctions`, `TestRegisterResourceProviderRejectsDuplicateFunctionInPlugin`, `TestRegisterResourceProviderAcceptsNilFunctions`.
  - `plugins/direct_plugin_host_test.go`: `TestDirectHostListsAndCallsFunctions`.
- Complexity: Medium. Token estimate: ~50k. Agent strategy: single agent, sequential (interfaces, then helpers, then hosts, then doubles).

### Task: Plugin functions are called from configuration over gRPC

- `plugins/plugin.proto:6-14`: add `rpc GetFunctions(GetFunctionsRequest) returns (GetFunctionsResponse)` and `rpc CallFunction(CallFunctionRequest) returns (CallFunctionResponse)`, plus the messages from Data Structures. Use new field numbers only. Regenerate `plugins/proto/plugin.pb.go` and `plugin_grpc.pb.go` with `make protos`.
- `plugins/grpc_server.go:47-184`: add `GetFunctions` (marshal each `FunctionSignature` to JSON) and `CallFunction`, which wraps `withLogger` (:186) and puts errors in the response `Error` as `errorToString` (:223) does.
- `plugins/grpc_plugin_host.go:145-340`:
  - `grpcPluginWrapper.GetFunctions`: on `codes.Unimplemented`, return nil and no error; on any other error, return it.
  - `grpcPluginWrapper.CallFunction`: uses `callContext` (:158).
  - `GRPCPluginHost.GetFunctions`: cached alongside types (:342-376).
  - `GRPCPluginHost.CallFunction`.
- `plugins/grpc_plugin.go:87-91`: the handshake is unchanged. Add a comment that the function RPCs are additive.
- `internal/catalog/catalog.go:369-406,699-733`:
  - `checkHostTypes` (or a sibling `checkHostFunctions`) checks each `host.GetFunctions()` name against `functions.BuiltinNames()` (`Existing: "builtin"`), declared application functions and loaded hosts' functions, returning `FunctionNameClashError` with `Provider`/`Registry` set to the plugin.
  - It builds a `function.Function` from the signature with `ctyjson.UnmarshalType`. Its `Impl` marshals args with `ctyjson.Marshal(arg, paramType)` into a JSON array, calls `host.CallFunction(ctx, name, args)` with `context.Background()` (the eval context carries no ctx), and unmarshals the result with `ctyjson.Unmarshal(result, returnType)`.
  - `addHost` stores the functions in `c.functions` under `mu`.
  - A failed check stops the host and fails `Load`, cached in `loadErr` as type clashes are.
- `internal/parser/parser.go:1537`: no change beyond the earlier task. Plugin functions are present because `parseAndValidate` loads plugins first (`parser.go:1593`), and the walk runs after.
- `internal/testutil/plugins.go:32`: add `BuildFunctionFixturePlugin(t, moduleRoot)`, which builds a new `internal/test_fixtures/plugins/functions/main.go` offering `greet(name string) (string, error)` on a `widget`-style provider. Also add a second fixture, `functions_clash`, offering the same name.
- `e2e/fixtures/recorder`: reuse it to record what Create receives, or add a recording provider to the functions fixture.
- New tests:
  - `plugins/grpc_server_test.go` (loopback style at :249-280): `TestGRPCHostListsFunctionsLikeDirectHost`, `TestGRPCHostCallsFunction`, `TestGRPCHostTreatsUnimplementedAsNoFunctions`.
  - `internal/catalog/catalog_test.go`: `TestLoadReportsPluginFunctionClashWithBuiltin`, `TestLoadReportsPluginFunctionClashWithApplicationFunction`, `TestLoadReportsFunctionClashBetweenPlugins`.
  - Root package `config_plugin_functions_test.go`: `TestApplyCallsInProcessPluginFunction`, `TestApplyCallsExternalPluginFunction`, `TestPluginCreateReceivesFunctionResult`, `TestFirstOperationReportsApplicationAndPluginFunctionClash`, `TestFirstOperationReportsFunctionClashBetweenPlugins`, `TestFirstOperationReportsPluginFunctionClashWithBuiltin`, `TestPluginWithoutFunctionRPCStillLoads` (a stub gRPC server that leaves the RPCs unimplemented).
- Complexity: High. Token estimate: ~75k. Agent strategy: parallel analysis (proto, server and host; catalog and fixtures), then sequential integration and the root-package tests.

### Task: Document the changed library surface and its breaking changes

- `docs/plugin-developer-guide.md:24-44`: rewrite the `Functions` description. Add an "Offering a function" section with `plugins.NewFunction` and `plugins.NewFunctions`, the supported argument types, and a note that calls are evaluated while configuration is read.
- `docs/plugins.md:32,97-104`: update the interface listing and the wording.
- `docs/README.md:55`: describe `internal/functions` as the built-in functions.
- Add a section on `registry.Local.RegisterFunction` to whichever doc describes `RegisterType`; find it with `grep -rn RegisterType docs`.
- `README.md`: check that there is no `HCL_VAR_`. Optionally mention application and plugin functions where line 15 lists features.
- `CHANGELOG.md`: add `## 20261011095738-fad05e41-language-surface` at the top, in the existing format (prose paragraphs, then `**Breaking:**` and `- ` bullets, as at lines 3-11 and 33-37). The bullets are:
  - `HCL_VAR_` replaced by `XCL_VAR_`, and unprefixed env vars no longer read;
  - template directives rejected;
  - list blocks accept optional names, and duplicate names are errors;
  - `registry.Registry` gains `Functions()`;
  - `plugins.ProviderFunction` reshaped;
  - `PluginEntityProvider` and `PluginHost` gain `GetFunctions` and `CallFunction`;
  - `ParserOptions.CustomFunctions` removed;
  - application functions named like built-ins are errors;
  - plugins must be rebuilt to offer functions (old plugins still load);
  - unsupported block field types are reported by `NewConfig`.
- No tests (convention: never test repository files).
- Complexity: Low. Token estimate: ~20k. Agent strategy: single agent.

### Task: Release xcl with the language surface changes

- A person merges the xcl branch and pushes a semver tag, following the repo's existing release practice (the latest xcl tag is `v0.2.1`, so the next tag would be `v0.3.0`: the changes are breaking, which is a minor bump before 1.0; the plugin repos currently pin `v0.1.1`).
- Check: `go list -m github.com/jumppad-labs/xcl@<tag>` resolves from a clean module cache.
- Complexity: Low. Token estimate: n/a (human).

### Task: The plugin template offers an example function

- `xcl-plugin-template:go.mod`: run `go get github.com/jumppad-labs/xcl@<new tag>` and `go mod tidy`.
- `xcl-plugin-template:providers/note.go:186-189`: `Functions()` returns `plugins.NewFunctions(map[string]plugins.ProviderFunction{"note_filename": ...})`, built once in `NewNoteProvider` with `plugins.NewFunction(noteFilename)`. `noteFilename(title string) (string, error)` turns a title into a safe file name and lives in a private helper below the exported methods, per the file order of `plugin-layout.md`. Keep `Functions` last.
- `xcl-plugin-template:providers/note_test.go`: add `TestNoteFilenameFunctionTurnsTitleIntoFileName` and `TestNoteFilenameFunctionRejectsEmptyTitle` (separate functions).
- `xcl-plugin-template:examples/basic/main.xcl`: make one note use `note_filename("My Greeting")` for an attribute.
- `xcl-plugin-template:e2e/inprocess_test.go` and `e2e/external_test.go`: assert the applied note holds the function's result.
- `xcl-plugin-template:README.md:86,194-221`: add an "Add a function" section after "Add your own resource". It covers writing the Go func, wrapping it with `NewFunction`, returning it from `Functions()`, unit testing it, calling it from configuration, the bare and global naming with clash errors, and the supported types.
- Complexity: Low. Token estimate: ~30k. Agent strategy: single agent.

### Task: The Docker plugin builds against the new plugin interface

- `xcl-plugin-docker:go.mod`: run `go get github.com/jumppad-labs/xcl@<new tag>` and `go mod tidy`.
- `xcl-plugin-docker:providers/container.go:241-244` and `providers/network.go:125-128`: `Functions()` keeps returning nil. Adjust only if the signature changes, which it doesn't.
- `xcl-plugin-docker:plugin.go:27,38`: compile check. `PluginBase` provides the new methods.
- Run `make test` and `make e2e`.
- Complexity: Low. Token estimate: ~10k. Agent strategy: single agent.

### Task: The website states what the language does

- `xcl-website:src/pages/index.mdx:153-154`: `HCL_VAR_<name>` becomes `XCL_VAR_<name>`.
- `xcl-website:src/pages/index.mdx:168-172` (Functions FeatureCard): change "Around sixty built in" to "57 built in", keeping the examples, and add "and your own: register Go functions with the application or offer them from a plugin".
- `xcl-website:src/pages/examples/configuration-only.mdx:160-161`: change `HCL_VAR_` and `HCL_VAR_image_tag` to `XCL_VAR_` and `XCL_VAR_image_tag`.
- `xcl-website:src/pages/examples/plugins.mdx:74-75`: change `HCL_VAR_` and `HCL_VAR_output_dir` to `XCL_VAR_` and `XCL_VAR_output_dir`.
- `xcl-website:src/pages/plugin-template.mdx:66-68`: describe `Functions` as returning the functions the provider offers to configuration.
- No pages are removed (`gotchas/no-redirects-on-page-removal`). Build the site to check that the MDX compiles.
- Complexity: Low. Token estimate: ~15k. Agent strategy: single agent.

### Task: The editor extension does not highlight template directives

- `xcl-vscode:test/fixtures/sample.xcl`: add lines such as `msg = "%{ if true }yes%{ endif }"`, `items = "%{ for x in list }${x}%{ endfor }"`, and a heredoc containing `%{ if … }`.
- `xcl-vscode:test/tokenize.test.js` (style at :96-97): add `assertScope` checks that the `%{` tokens carry `string.quoted.double.xcl` (or the heredoc's string scope) and no `meta.interpolation` scope. Keep the existing `${` checks, and add one for the `${x}` inside the new line.
- `xcl-vscode:syntaxes/xcl.tmLanguage.json:136-187`: no change expected. If a check shows `%{` picks up a scope, adjust the `string` and `heredoc` patterns.
- `xcl-vscode:CHANGELOG.md`: add an `## [Unreleased]` entry: "Template directives (`%{ … }`) are not part of XCL and are shown as plain string text."
- Complexity: Low. Token estimate: ~10k. Agent strategy: single agent.

## Testing Strategy

Per-task test names and files are listed under each task above. Across the plan:

- **Environment overrides task:** root-package tests drive `NewConfig`/`Apply` with `t.Setenv`. The parser utility tests cover an empty prefix and values containing `=`. The example plugin smoke tests prove a real binary sees `XCL_VAR_`.
- **References task:** parser dependency tests assert links for `for`, index, parenthesised and relative-traversal references. Apply tests assert the evaluated values. A root-package test covers a variable default built from an entity.
- **Optional fields task:** fork unit tests for constraint-type conversion and null-to-zero decoding, plus one root-package acceptance test.
- **Template directives task:** fork tests for `if`/`for`/heredoc rejection, interpolation and the `%%{` escape. Rewritten upstream directive cases. Root-package validate/apply tests.
- **Named blocks decode task:** gohcl decode, check and encode tests for maps, lists, missing names, duplicate names and multi-label strictness, plus hclsyntax optional-label tests.
- **Panic removal task:** one gohcl test per former panic site, catalog declaration tests, and root-package `NewConfig` and plugin-load tests using `require.NotPanics`.
- **Named block maps end-to-end task:** root-package acceptance tests for maps, lists, reference by name, `for` over named blocks, no-change second apply and a single-block diff. State comes from real applies.
- **Application functions task:** errors, registry and catalog unit tests. Root-package tests for `slugify`, the `upper` clash, the cross-registry clash, an invalid func and sensitive redaction.
- **Plugin providers task:** plugins package tests for `NewFunction`, `PluginBase` collection and the direct host.
- **gRPC functions task:**
  - loopback gRPC tests showing the direct and gRPC hosts behave the same, and that `Unimplemented` means no functions;
  - catalog load clash tests;
  - root-package in-process and compiled-binary function tests;
  - the recorded Create input (result, not call);
  - the three clash cases, asserted on the first operation's error.
- **Docs task:** no tests, per convention. Human review is in the test plan.
- **Template task:** a unit test for `note_filename`, and in-process and external e2e tests asserting the applied note holds the result.
- **Docker task:** the existing suite against the new release.
- **Website task:** none automated. The MDX build must succeed, and a human reviews the text.
- **Extension task:** tokenize-test scope assertions.

Manual items (**Manual — captured in the implementation test plan**):
- website and README claims;
- the extension's highlighting in VS Code;
- the CHANGELOG **Breaking:** list;
- following the template README to add a function from a fresh copy.

## Project References

- Spec: `20261011095738-fad05e41-language-surface` (`spektacular spec file read 20261011095738-fad05e41-language-surface`).
- Design `plugin-layout.md` from the `design` source (binding); design `libxcl.md` from the `design` source (compatibility only).
- Knowledge (xcl store):
  - `conventions/never-modify-dependencies.md`
  - `conventions/shared-errors-package.md`
  - `conventions/testing-and-mocking.md`
  - `conventions/shared-test-helpers.md`
  - `conventions/test-state-from-real-apply.md`
  - `decisions/own-hcl-fork-with-xcl-tags.md`
  - `gotchas/xcl-tags-gate-what-reaches-cty.md`
  - `gotchas/plugin-types-rebuilt-with-structof.md`
  - `architecture/links-are-attribute-paths.md`
- Knowledge (xcl-website store): `gotchas/no-redirects-on-page-removal.md`.
- Repo roots:
  - xcl `/home/nicj/code/github.com/jumppad-labs/xcl`
  - xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`
  - xcl-vscode `/home/nicj/code/github.com/jumppad-labs/xcl-vscode`
  - xcl-plugin-template `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-template`
  - xcl-plugin-docker `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-docker`

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Milestones 1 and 2 touch disjoint areas, except that the named-block-maps task depends on the references task. They can run in parallel agents: the parser references task, the gohcl/hclsyntax tasks and the cty task. Milestone 3 is sequential: application functions, then plugin surface, then gRPC. The plugin-repo tasks wait for the human release task. The extension task can run at any time.

## Migration Notes

- Users replace `HCL_VAR_<name>` with `XCL_VAR_<name>`. Code that relied on raw environment variables appearing as `variable.<NAME>` must pass them with `WithVariables`.
- Configuration using `%{ … }` directives is rewritten with functions such as `format`, `join`, `template_file` or `${ cond ? a : b }`.
- Third-party `registry.Registry` implementations add `Functions() []registry.Function`, returning nil when they offer none.
- `ProviderFunction` implementations, if any exist outside xcl, move to `Signature()`/`Call(ctx, args)`. Most providers return nil from `Functions()` and need no change.
- Code implementing `PluginHost` or `PluginEntityProvider` directly, rather than embedding `PluginBase`, adds `GetFunctions` and `CallFunction`.
- Code passing `ParserOptions.CustomFunctions` registers functions on a registry instead.
- Plugins are rebuilt against the new release to offer functions. Older plugin binaries keep working without functions.

## Performance Considerations

- A plugin function call is one gRPC round trip per evaluation, made during the walk. Configurations that call a plugin function inside large `for` expressions pay that per element, which is acceptable for configuration-time evaluation.
- The function list is cached per host, as types are.
- The variable walker replaces a hand-written switch with a full expression walk. The cost is negligible against parsing.
