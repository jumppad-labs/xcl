---
created_date: "2026-10-11"
document_status: draft
project: xclconfig
spec: 20261011095738-fad05e41-language-surface
plan: 20261011095738-fad05e41-language-surface
---

# XCL v0.3.0: the language surface settles

Variables are overridden from the environment with `XCL_VAR_<name>` and nothing else in the environment leaks in. Named child blocks are collected into a map by name and can be referenced as `app.web.listener.https.port` or iterated with `for`, references inside `for`, index and parenthesised expressions resolve, optional fields can be left out of object and map values, and string template directives (`%{ … }`) are rejected in favour of functions. Applications register their own functions beside their types, plugins offer functions from their providers, every function is evaluated while configuration is read, and a Go type the library cannot decode is an error rather than a crash.

> Derived from project xclconfig (file), spec/plan 20261011095738-fad05e41-language-surface. See the project-level record for the full feature.

## What changed in this repo

- **Environment overrides.** `parser.DefaultVariableEnvPrefix` is `XCL_VAR_`; `Config` builds every operation's parser options through one `parserOptions` helper that sets it; `setVariables` reads nothing for an empty prefix and splits each entry on its first `=`. The example and e2e fixtures use `XCL_VAR_`.
- **References.** `referencesIn` (`internal/parser/exp.go`) replaces the hand-written `processExpr` switch with `hclsyntax.Variables`, so `for`, index, parenthesised and relative-traversal references become links, with `for` key/value names masked. Fixture types `Service`, `Listener`, `Gateway` added.
- **Optional object attributes.** `gocty.ImpliedConstraintType` marks `optional` fields as optional attributes; `gohcl.DecodeExpression` converts against it; `fromCtyObject` leaves a null optional field at zero and returns its embedded-struct recursion error. Logged in both fork upstream logs. Fixture types `Proxy`, `Endpoint` added.
- **Template directives.** The forked template parser reports every `%{ … }` as "Template directives are not supported", skips to its closing brace and restores the recovery flag so later directives are reported too. Upstream directive table cases removed; `template_directive_test.go` added.
- **Named blocks.** `hcl.BlockHeaderSchema.LabelsOptional`, honoured by `hclsyntax`; `gohcl` accepts `map[string]T` block fields, gives maps and single-label lists an optional name label (`blockLabels`), collects map blocks by name (`decodeBlocksToMap`), reports a missing or repeated name at the block in both the decoder and `CheckBody` (`checkBlockNames`), and encodes a map of blocks in key order with the key as label. The JSON body treats an optional-label type's array as unnamed blocks and its object as named ones. The parser addresses map blocks by name in links (`nestedBlockPath`, `mapBlockName`), known-body diff paths (`Path.Key`), the configured check (`nestedBlockBodyByName`), validate and sensitive paths, and `encode.go`'s `findReferenceBody`. Fixture type `WebApp` added.
- **Panics to errors.** `gohcl.ValidateSchemaType`, error-returning `impliedBodySchema` and `fieldTagsOf`, and "Unsupported decode target" diagnostics in the decoder and checker. `catalog.DeclareType` and `checkHostTypes` validate every type, returning the new `InvalidTypeError` (errors package, re-exported from the root).
- **Application functions.** `registry.Function`, `Registry.Functions()`, `Local.RegisterFunction`, GitHub delegation; `catalog.DeclareFunction`/`Functions` with `checkFunctionName`; `functions.GoFuncSignature`, `GoFunc` (`Call`, `CallJSON`), `CtyFunctionFromSignature`, `BuiltinNames`; the parser's `getFunctions` merges catalog functions with builtins; `ParserOptions.CustomFunctions` removed. New `FunctionNameClashError` and `InvalidFunctionError`, re-exported.
- **Plugin functions.** `plugins.ProviderFunction` reshaped (`Signature`, JSON `Call`), `FunctionSignature`, `FunctionParam`, `RegisteredFunction`, `NewFunction`, `NewFunctions`; `PluginBase` collects provider functions and refuses an in-plugin duplicate; `PluginEntityProvider` and `PluginHost` gained `GetFunctions`/`CallFunction`; the direct host serves them with the plugin-sourced logger. `plugin.proto` gained `GetFunctions` and `CallFunction` with `GetFunctionsRequest`, `RegisteredFunction`, `GetFunctionsResponse`, `CallFunctionRequest`, `CallFunctionResponse` (regenerated); the gRPC server and host implement them, reading `Unimplemented` as no functions and caching the list; `catalog.addHost` runs `checkHostFunctions` and builds a configuration function per plugin function (`pluginFunction`). Fixture plugins `functions` and `functions_clash` with `testutil.BuildFunctionFixturePlugin`/`BuildFunctionClashFixturePlugin`.
- **Docs.** `docs/plugin-developer-guide.md` ("Offering a function"), `docs/plugins.md` ("Application functions", `RegisterFunction`), `docs/overview.md`, `docs/README.md`, `README.md`, and the `CHANGELOG.md` entry naming v0.3.0 with the breaking list.
- **Tests.** Root-package acceptance tests for every spec criterion (`config_variables_env_test.go`, `config_variables_test.go`, `config_optional_values_test.go`, `config_template_directives_test.go`, `config_named_blocks_test.go`, `config_bad_type_test.go`, `config_functions_test.go`, `config_plugin_functions_test.go`), plus fork, parser, catalog, functions, registry, errors and plugins unit tests.

## Why

The library did not do what its documentation said about environment overrides, functions and named blocks, and the HCL-derived parser exposed templating XCL does not want. Fixing the library to match a settled language lets the language reference describe real behaviour, and gives embedding developers and plugin authors one way to add functions.

## Breaking

See the **Breaking:** list under `## 20261011095738-fad05e41-language-surface` in `CHANGELOG.md`: the `XCL_VAR_` prefix, directive rejection, block label rules, `Registry.Functions`, the reshaped `ProviderFunction`, the new `PluginEntityProvider`/`PluginHost` methods, the removed `ParserOptions.CustomFunctions`, the protocol additions, builtin-named application functions, and `InvalidTypeError` for undecodable Go types.
