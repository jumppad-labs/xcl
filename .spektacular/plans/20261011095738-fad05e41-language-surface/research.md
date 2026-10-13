---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Research: 20261011095738-fad05e41-language-surface

## Alternatives considered and rejected

### Option A: Reject template directives with a lexer pre-scan in internal/parser

Run `hclsyntax.LexConfig` after `ParseXCLFile` in `parseResourcesInFile` (`internal/parser/parser.go:739-753`) and report every `TokenTemplateControl` token.

**Rejected**: The spec's Technical Approach asks for language changes to be made in XCL's own handling of the language, not bolted on around the parser. A pre-scan is a second pass around the parser; the rule belongs in the parser XCL owns (`internal/xcl/hclsyntax/parser_template.go:477`).

### Option B: Namespaced plugin functions (`docker::fn`)

hclsyntax already parses `ns::fn` (`internal/xcl/hclsyntax/parser.go:1155-1188`).

**Rejected**: The spec's Technical Approach says function names stay bare, like the built-ins.

### Option C: A second plugin function mechanism on plugins.Plugin

Add a new interface unrelated to providers.

**Rejected**: The spec requires building on `ProviderFunction`, `ProviderFunctions` and `ResourceProvider.Functions()` (`plugins/functions.go:8-28`, `plugins/provider.go:125`). The plugin-layout design also puts `Functions` last among each provider's methods.

### Option D: Expose cty function.Function publicly for application functions

**Rejected**: cty is a fork under `internal/cty`, so applications cannot import it. Applications pass a plain Go func instead, converted by `functions.CreateCtyFunctionFromGoFunc` (`internal/functions/functions.go:17`).

### Option E: Keep ParserOptions.CustomFunctions, or add an xcl.WithFunctions Config option

**Rejected**: Config never sets `CustomFunctions` (`internal/parser/parser.go:129`, `config.go:344`), and custom functions silently override built-ins (`parser.go:1537-1549`). The spec asks for registration that looks like type registration (`registry/local.go:90`).

### Option F: Add a ForExpr case to the hand-written processExpr switch

**Rejected**: The switch (`internal/parser/exp.go:28-149`) also misses `TemplateJoinExpr`, `IndexExpr`, `RelativeTraversalExpr` and `ParenthesesExpr`. `hclsyntax.Variables` (`internal/xcl/hclsyntax/variables.go:14-66`) already walks every node and masks `for` key and value names, and `sensitive_check.go:289-298` uses it.

### Option G: Imply cty.ObjectWithOptionalAttrs everywhere in gocty.ImpliedType

**Rejected**: A type with optional attributes is only valid as a conversion constraint. `ImpliedType` also builds values (`internal/convert/convert.go`, `gohcl/encode.go:380-385`), so the constraint type is used only in `gohcl.DecodeExpression`.

### Option H: Load plugins eagerly in NewConfig to report plugin function clashes at creation

**Rejected**: By the user's decision (option A, via the orchestrator), clashes involving a plugin are reported when plugins load on the first operation. That is where plugin type clashes are reported today (`config.go:622-637`, `internal/catalog/catalog.go:632-700`).

## Chosen approach — evidence

- Env prefix: `DefaultOptions` sets `HCL_VAR_` (`internal/parser/parser.go:140-160`); Config builds `ParserOptions` without it at `config.go:344`, `:378`, `:461`, `config_diff.go:42`; `setVariables` (`internal/parser/util.go:167-184`) treats an empty prefix as matching everything and splits on every `=`.
- Function plumbing: `getFunctions` (`internal/parser/parser.go:1537-1549`) merges built-ins and `customFunctions`, silently overriding, then wraps with `redactingFunctions` (`internal/parser/function_redaction.go:14-30`). EvalContext functions come from `buildContextForResource` (`internal/parser/context.go:24-25`) and module subcontexts (`callbacks.go:180-181`). Functions are evaluated only during the walk (Apply/Diff), before lifecycle calls.
- Type clash reporting: `NewConfig` → `catalog.DeclareType` (`config.go:242-247`, `internal/catalog/catalog.go:142-171`, `checkDeclared` :329); plugin clashes at load → `addHost` → `checkHostTypes` (`catalog.go:369-406`). Error type `TypeNameClashError` (`errors/type_errors.go:11-49`) is the model for a `FunctionNameClashError`.
- Registry surface: `registry.Registry{Name, Types, Plugins}` (`registry/registry.go:26-38`), `Local.RegisterType` (`registry/local.go:90`), GitHub registry delegates (`registry/github.go:157-163`).
- Plugin boundary: `PluginEntityProvider` (`plugins/plugin.go:47-59`), `PluginHost` (`plugins/plugin_host.go:9-37`), direct host (`plugins/direct_plugin_host.go:30`), gRPC host (`plugins/grpc_plugin_host.go`), server (`plugins/grpc_server.go`), proto (`plugins/plugin.proto:6-27`), Makefile `protos` target (protoc, no buf). Handshake fixed at `ProtocolVersion: 1` (`plugins/grpc_plugin.go:87-91`). Entity data is JSON; ctyjson (`internal/cty/json/value.go:31,62`, `type.go:11,19`) is the natural codec for typed args/results.
- Named block maps: panic at `internal/xcl/gohcl/schema.go:87-91`; decode block loop `gohcl/decode.go:136-233`; label set `decode.go:264-276`; `CheckBody` `gohcl/check.go:83-117`; label count enforcement `internal/xcl/hclsyntax/structure.go:178-218`; encode `gohcl/encode.go:585-627`; position-indexed paths `internal/parser/parser.go:1346,1430`, `diff_decode.go:70-85`; maps already handled in `computed.go:110-120,210-264`, `properties.go:185-234`, `configured_check.go:124-141`; schema serialize/deserialize already round-trip `map[K]struct` (`internal/schema/serialize.go:71-73`, `deserialize.go:105-124`).
- References by name: `ImpliedType` maps `map[string]X` to `cty.Map` (`internal/cty/gocty/type_implied.go:61-70`); `GetAttr` on a map indexes by key (`internal/xcl/ops.go:299-321`); on a list it fails (`ops.go:324-350`).
- Optional in object values: `impliedStructType` always `cty.Object` (`type_implied.go:81-129`); `DecodeExpression` (`gohcl/decode.go:292-324`); converter supports optional attrs (`internal/cty/convert/conversion_object.go:21-101`); `fromCtyObject` rejects null for non-pointer (`gocty/out.go:76-84,458-521`).
- Template directives: lexer `TokenTemplateControl` (`hclsyntax/scan_tokens.rl:181-189`), parser branch `parser_template.go:477-656`; highlight package colours directives (`highlight/tokenize.go:69,226,329`).
- Variables: references in `default` are already linked and decoded with a full context (`parser.go:924-957,1045`, `callbacks.go:86,141`); no test covers a default referring to an entity.
- Panics a user Go type can trigger: `gohcl/schema.go:88,153,173,179`, `decode.go:51,128,271,297`, `check.go:117`; `declaration.Validate` (`internal/declaration/declaration.go:18-49`) does not check tags.

## Files examined

- xcl:config.go:148-259,330-637 — option handling, NewConfig, ParserOptions built per operation, lazy plugin load
- xcl:options.go:13-178 — ConfigOption list; no function option
- xcl:registry/registry.go:26-116, registry/local.go:60-140, registry/github.go:140-163 — registry surface
- xcl:internal/catalog/catalog.go:111-760 — type declaration, clash checks, plugin load
- xcl:errors/type_errors.go:5-69 — TypeNameClashError shape and message
- xcl:internal/functions/functions.go:17-487 — CreateCtyFunctionFromGoFunc, 57 built-ins
- xcl:internal/parser/parser.go:71-221,739-813,924-957,1250-1440,1532-1549 — options, file parse, links, getFunctions
- xcl:internal/parser/exp.go:28-184 — processExpr misses ForExpr etc.
- xcl:internal/parser/context.go:18-233 — eval context, variables, env override
- xcl:internal/parser/util.go:146-184,233-327 — vars files, setVariables, dead setContextVariablesFromList
- xcl:internal/parser/function_redaction.go:14-133 — sensitive wrapping of all functions
- xcl:plugins/functions.go, provider.go:19-126, plugin.go:36-142, plugin_host.go, direct_plugin_host.go, grpc_plugin_host.go, grpc_plugin.go:34-91, grpc_server.go, serve.go, plugin.proto — plugin boundary
- xcl:internal/xcl/gohcl/schema.go, decode.go, check.go, encode.go — block decoding and panics
- xcl:internal/xcl/hclsyntax/structure.go:178-218, parser_template.go, variables.go, scan_tokens.rl — labels, templates, variable walk
- xcl:internal/cty/gocty/type_implied.go, out.go, internal/cty/convert/conversion_object.go — implied types and optional attrs
- xcl:internal/xcl/tags/tags.go:56-87 — tag kinds (`optional` is a kind)
- xcl:internal/parser/parse_test.go:31-55,234-252, references_test.go, dependencies_test.go, registered_types_test.go, functions_test.go — test models
- xcl:internal/test_fixtures/registered/types.go, internal/test_fixtures/config/* — fixtures
- xcl:example/plugin/main_test.go:80,244, smoke_test.go:57,99 — set HCL_VAR_output_dir
- xcl:docs/plugin-developer-guide.md:24-44, docs/plugins.md:32,97-104 — describe Functions as provider-to-provider
- xcl:CHANGELOG.md:1-64 — `## <spec-id>` headings, prose, `**Breaking:**` list
- xcl-website:src/pages/index.mdx:153-154,168-172 — HCL_VAR_, "Around sixty"
- xcl-website:src/pages/examples/configuration-only.mdx:160-161, examples/plugins.mdx:74-75 — HCL_VAR_ comments
- xcl-website:src/pages/plugin-template.mdx:66-68, plugin-logging.mdx:149 — Functions in lifecycle list
- xcl-vscode:syntaxes/xcl.tmLanguage.json:136-187 — no `%{` rule; `${}` interpolation; heredoc `#{{ }}`
- xcl-vscode:test/tokenize.test.js, test/fixtures/sample.xcl, package.json, CHANGELOG.md — test style
- xcl-plugin-template:providers/note.go:186-189, plugin.go:24-34, README.md:86,194-221, go.mod (xcl v0.1.1, no replace), Makefile — template
- xcl-plugin-docker:providers/container.go:241-244, network.go:125-128, go.mod (xcl v0.1.1), Makefile, e2e/e2e_test.go — example plugin

## External references

- HashiCorp go-plugin non-Go guide / `VersionedPlugins` — protocol negotiation later used by libxcl protocol v2; new RPCs must be additive so they carry forward.
- cty `ObjectWithOptionalAttrs` and convert semantics (fork of go-cty v1.15.0) — how optional attributes are filled with null on conversion.

## Prior plans / specs consulted

- Design `design/plugin-layout.md` (binding, referenced by the spec) — provider file order ends with `Functions`; template/docker keep the standard layout.
- Design `design/libxcl.md` (referenced by the dependent python-support spec) — protocol v2 via `VersionedPlugins`; schema v1 describes blocks as `repeated` only, so named-block maps are its concern; host-language functions deferred and would build cty functions from a declared signature.
- Knowledge: decisions/own-hcl-fork-with-xcl-tags, conventions/never-modify-dependencies (internal/xcl is MPL, keep headers, record UPSTREAM.md), gotchas/xcl-tags-gate-what-reaches-cty, gotchas/plugin-types-rebuilt-with-structof, architecture/links-are-attribute-paths, conventions/shared-errors-package, conventions/testing-and-mocking, xcl-website gotchas/no-redirects-on-page-removal.

## Open assumptions

- (Resolved by the user, option A) Plugin function clashes are reported when plugins load on the first operation; app-function clashes fail NewConfig.
- Sensitive arguments reach plugin functions unmarked (the redaction wrapper unmarks before calling); the result is re-marked. Plugins already receive real sensitive values in lifecycle calls.
- A plugin built against today's xcl (no function RPCs) answers Unimplemented; the host treats that as "no functions".
- xcl-plugin-template and xcl-plugin-docker require a published xcl version; they can only be bumped after an xcl release containing this work.
- The website function count is 57 (`GetDefaultFunctions`).

## Drafting assumptions

### Chosen direction: registry functions, provider-hosted plugin functions over additive RPCs, language rules in the fork (architecture)
- **Decision**: Option 1. App functions are registered on `registry.Local.RegisterFunction` and read through `Registry.Functions()`. Plugin functions come from `ResourceProvider.Functions()` through `plugins.NewFunction`, and reach the host over new `GetFunctions`/`CallFunction` RPCs on protocol 1. Clashes raise `FunctionNameClashError` (option A timing). Directives, optional labels, map blocks, panic removal and optional object attributes are done in `internal/xcl`/`internal/cty`. `for` references move to `hclsyntax.Variables`, and `Config` sets the `XCL_VAR_` prefix through one helper.
- **Key design decisions**: Additive RPCs with no handshake bump, so libxcl protocol v2 can carry them forward. A plugin with no function RPC means no functions. Signatures use cty JSON type strings, and args/results travel as JSON. Map blocks become cty maps. `ValidateSchemaType` runs at declaration and at plugin load, so a bad field never panics. `RegisterType` keeps its malformed-declaration panics.
- **Rationale**: This is the only option that meets the Technical Approach points together with the plugin-layout design.
- **Rejected**: Option 2 (a Config option plus a separate plugin interface) and option 3 (wrappers around the parser). See research.md.

### Language rules live in the internal/xcl fork (discovery)
- **Decision**: Template-directive rejection is made in the forked template parser (`internal/xcl/hclsyntax/parser_template.go`); named-block maps and optional labels in the forked `gohcl` / `hclsyntax` structure handling, recorded in `internal/xcl/UPSTREAM.md` with MPL headers kept.
- **Rationale**: The spec asks for language changes in XCL's own handling of the language rather than bolted on around the parser; `internal/xcl` is XCL's own copy and the knowledge base allows editing it there.
- **Rejected**: A lexer pre-scan in `internal/parser` (a second pass around the parser); patching outside the fork with wrappers.

### Application functions are plain Go funcs registered on the local registry (discovery)
- **Decision**: `registry.Local.RegisterFunction(name string, fn any)`, with `registry.Registry` gaining `Functions() []Function`; converted with `CreateCtyFunctionFromGoFunc`, problems returned by `NewConfig`.
- **Rationale**: Mirrors `RegisterType`; cty is internal so a Go func is the only importable form.
- **Rejected**: Exposing cty types; a `WithFunction` Config option (does not look like type registration).

### Plugin functions build on ProviderFunction and a pair of additive RPCs (discovery)
- **Decision**: Reshape `ProviderFunction` (signature + JSON-array args/result), collect each provider's `Functions()` into the plugin, add `GetFunctions` and `CallFunction` RPCs to `PluginService`; an older plugin's Unimplemented answer means no functions.
- **Rationale**: Spec requires building on the existing interfaces; additive RPCs carry forward into libxcl's protocol v2 negotiation.
- **Rejected**: A second mechanism on `plugins.Plugin`; bumping the go-plugin protocol version now (protocol v2 is libxcl's work).

### Plugin repos are bumped after an xcl release (discovery)
- **Decision**: A human task cuts an xcl release; template and docker then `go get` it.
- **Rationale**: Both require `github.com/jumppad-labs/xcl v0.1.1` with no replace directive and verify against releases weekly.
- **Rejected**: Committing a `replace` to a local path (breaks CI everywhere else).

### Function clash timing (discovery — decided by the user via orchestrator, option A)
- **Decision**: App function vs built-in or another app function fails `NewConfig`. Any clash involving a plugin fails the first operation when plugins load, with a `FunctionNameClashError` naming the function and both sources, exactly as plugin type clashes work. Plugin-case tests assert on the first operation's error. The AC wording "makes creating the config fail" is read as "creating or first using the config".
- **Rationale**: Plugins load lazily; matches the Technical Approach ("same place type clashes are reported today").
- **Rejected**: Eager plugin loading in NewConfig (B); an opt-in preload option (C).

### Optional labels for list and map block fields (architecture)
- **Decision**: A `map[string]T` block field, and a slice block field whose element has zero or one `label` field, takes one optional name label. Single (struct or pointer) block fields, and elements with two or more labels, keep today's strict label count. In a map, a missing name and a duplicate name are gohcl diagnostics. The duplicate-name check also covers lists.
- **Rationale**: This meets the "lists accept named and unnamed blocks" and "duplicate names" requirements without changing single-block semantics the spec doesn't mention.
- **Rejected**: Making every block's labels optional, which would weaken label validation for multi-label blocks.

### Bad block field types are validated at declaration, not in RegisterType (architecture)
- **Decision**: `gohcl.ValidateSchemaType` runs in `catalog.DeclareType` (returned by NewConfig) and in `checkHostTypes` (at plugin load). gohcl's own panics become diagnostics as a backstop.
- **Rationale**: The AC requires an error from creating or validating the config with no panic, and `RegisterType` panics on a declaration error.
- **Rejected**: Adding the check to `declaration.Validate`, because it would panic in `RegisterType`.

### Signature format for plugin functions (architecture)
- **Decision**: The signature is JSON `{"params":[{"name","type"}],"variadic"?:..., "return": <cty JSON type>}`, with types written in `internal/cty/json` type form. Args are a JSON array and the result is plain JSON. Supported Go types are string, bool, the int/uint/float kinds, and slices and string-keyed maps of those.
- **Rationale**: This is language-neutral, so a future Python/Node plugin under protocol v2 can produce it, and it reuses the ctyjson codec.
- **Rejected**: msgpack (not vendored), and the Go type strings the legacy entity schema uses (Go-only).

### Highlight package left as is (architecture)
- **Decision**: `highlight/tokenize.go` keeps its directive colouring.
- **Rationale**: Configuration with directives no longer validates, so the highlighter never sees them in valid output. The spec lists only the editor extension.
- **Rejected**: Removing the directive frames, which changes a public package for no user-visible effect.

### Editor extension change is a regression test (architecture)
- **Decision**: The grammar has no `%{` rule today (`syntaxes/xcl.tmLanguage.json`), so xcl-vscode gets fixture lines and `assertScope` checks that `%{ if … }` and `%{ for … }` stay `string.quoted` scope and do not start `meta.interpolation`. `${ }` keeps its scopes. CHANGELOG.md gets an entry.
- **Rationale**: The requirement is already met by the grammar. A test pins it.
- **Rejected**: Adding an `invalid.illegal` scope for directives, which would be a highlighting feature the spec doesn't ask for.

### Website function count wording (architecture)
- **Decision**: The Functions feature card names the exact built-in count (57) and adds application and plugin functions.
- **Rationale**: The AC allows a count that matches the built-ins. 57 is `len(GetDefaultFunctions)`.
- **Rejected**: Dropping the count entirely. This was acceptable too, and is equally fine if the reviewer prefers it.

### Remove ParserOptions.CustomFunctions (data_structures)
- **Decision**: Functions reach the parser only through the catalog. Tests that set `CustomFunctions` register on a catalog instead.
- **Rationale**: This leaves one path for functions, and every clash goes through the catalog's check.
- **Rejected**: Keeping both paths, because a custom function could then silently override a built-in.

### InvalidFunctionError for unconvertible funcs (data_structures)
- **Decision**: `NewConfig` returns a typed error for a func that cannot be converted. A plugin function that cannot be converted fails plugin load.
- **Rationale**: This matches how declaration problems surface as errors, and keeps panics for nil inputs only.
- **Rejected**: Panicking in `RegisterFunction`, which would crash the embedding application over a signature mistake.

### Example template function and release tag (tasks)
- **Decision**: The template's example function is `note_filename(title)`. The next xcl tag is expected to be `v0.3.0`: the latest tag is `v0.2.1`, and breaking changes are a minor bump before 1.0; the person cutting the release chooses the exact tag.
- **Rationale**: The function fits the notes domain and has a clear test. A breaking change on a 0.x line bumps the minor version.
- **Rejected**: A generic `slugify` in the template, which could clash with the application function used in the xcl acceptance tests.

### Website waits for the release (tasks)
- **Decision**: The website task depends on the xcl release task.
- **Rationale**: The success metric requires site claims to match the latest release.
- **Rejected**: Editing the site before the release, which would briefly describe unreleased behaviour.

### New InvalidTypeError for bad block field types (tasks)
- **Decision**: `NewConfig` and plugin load report a bad field type with a new `InvalidTypeError` in the errors package, re-exported from the root package.
- **Rationale**: This follows the shared-errors convention, and callers can match it with `errors.As`.
- **Rejected**: A plain `fmt` error, which the convention discourages.

## Rehydration cues

- `spektacular spec file read 20261011095738-fad05e41-language-surface`
- `spektacular design read --data '{"source":"design","path":"plugin-layout.md"}'`
- Re-read: `internal/parser/parser.go:1532-1549`, `internal/catalog/catalog.go:369-406`, `internal/xcl/gohcl/schema.go:77-105`, `internal/xcl/gohcl/decode.go:136-276`, `internal/xcl/hclsyntax/structure.go:178-218`, `internal/parser/exp.go`, `plugins/plugin.proto`.
