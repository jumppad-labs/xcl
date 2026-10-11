---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Plan: 20261011095738-fad05e41-language-surface

<!-- Metadata -->
<!-- Created: 2026-10-11T14:33:45Z -->
<!-- Commit: 021b4ec4aaabdce2b0b6af19356647fdd7317d37 -->
<!-- Branch: f-libxcl -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

XCL becomes a language with a settled shape. Configuration can be overridden from the environment with `XCL_VAR_`. Named child blocks are collected into maps and can be referenced by name. References inside `for` expressions resolve. Optional fields work inside object values. Template directives are rejected in favour of functions, and applications and plugins can add their own functions. The library also stops crashing on Go types it can't decode. This lets the language reference describe real behaviour, and it benefits configuration authors, developers embedding XCL, and plugin authors. The work spans the xcl library, its website, the VS Code extension, the plugin template and the example Docker plugin.

## Conventions

- **Never modify dependency packages; `internal/xcl` is edited in place under MPL rules** — template-directive rejection, optional labels, map blocks and panic removal change `internal/xcl/hclsyntax` and `internal/xcl/gohcl`. Every changed file keeps its HashiCorp header, adds `// Modifications Copyright (c) Jumppad Labs`, and is logged in `internal/xcl/UPSTREAM.md`. The `internal/cty` fork changes go in `internal/cty/UPSTREAM.md`.
- **Shared error types live in the `errors` package** — `FunctionNameClashError` goes in `errors/` beside `TypeNameClashError`, with pointer receivers, and is re-exported from `config.go`.
- **Testing & mocking (testify `require`, no table-driven tests, positive and negative cases in separate functions, tests beside the code)** — this applies to every new parser, gohcl, catalog, registry, plugins and root-package test. New tests added to the fork's upstream table-test files use the non-table style of `gohcl/check_test.go`.
- **Never write tests that inspect repository files** — the website, README, docs and CHANGELOG corrections are checked by human review, not by tests.
- **Shared test helpers live in `internal/testutil`** — the helper that builds a fixture plugin offering functions, used by both the root and the catalog tests, goes there. A helper used by one package stays local to it.
- **Generate test state with a real apply** — the "plugin receives the result" test applies through a recording plugin and asserts on what its Create received. It does not hand-write state.
- **Code style / project structure (`any` over `interface{}`, public packages at module top level)** — `registry.Function`, `plugins.NewFunction` and the error re-exports are public; their implementation helpers stay in `internal/`.
- **Patterns & architecture (dependency injection, small interfaces)** — `PluginHost` gains only the two function methods, and the catalog takes functions from registries rather than from globals.
- **Glossary: entity** — new docs and error text say "entity" for anything a configuration declares.

Not applied: database and external services (nothing here touches a database); assert ordering on graph parents (no ordering assertions are added, and the `for` reference test asserts on the entity's links); testing provider `Changed`/`Update` (no provider decision logic changes).

## Architecture & Design Decisions

**Options weighed.**

1. **Registry-level functions, provider-hosted plugin functions over additive RPCs, and language rules in the `internal/xcl` fork** (chosen). An application registers a Go func on `registry.Local` next to its types. Plugins offer functions through the `ResourceProvider.Functions()` method that already exists, and the host reaches them through two new `PluginService` RPCs. Template directives, named-block maps, optional labels and optional object attributes are implemented in the forked `hclsyntax`, `gohcl` and `gocty` code XCL owns. *Pros:* it mirrors type registration (`registry/local.go:90`), reuses `CreateCtyFunctionFromGoFunc` (`internal/functions/functions.go:17`) and the unused `ProviderFunction` interfaces (`plugins/functions.go:8-28`), and puts clash checks where type clashes already live (`internal/catalog/catalog.go:142-171,369-406`). Each language rule sits in one place in the parser. *Cons:* it changes the `registry.Registry` interface, the `PluginHost` interface and `plugin.proto`, and fork changes must be logged in `internal/xcl/UPSTREAM.md`. *Effort:* High.
2. **A Config option (`xcl.WithFunctions`) and a separate plugin function interface.** *Pros:* no change to `registry.Registry`. *Cons:* it doesn't look like type registration, which the spec asks for. It also adds a second mechanism next to `ProviderFunction`, which the spec forbids, and it breaks the plugin-layout design's provider file order, which ends with `Functions`. *Effort:* Medium.
3. **Wrappers around the parser.** Template directives would be caught by a lexer pre-scan in `internal/parser`. For `for` expressions, `processExpr` would get a `ForExpr` case. Map blocks would be rewritten into lists before decoding. *Pros:* it changes the fork least. *Cons:* this is the "bolted on around it" approach the spec rejects. The `processExpr` switch (`internal/parser/exp.go:28-149`) would still miss `IndexExpr`, `ParenthesesExpr`, `RelativeTraversalExpr` and `TemplateJoinExpr`. *Effort:* Medium.

**Shape of the chosen solution.**

*Functions.*
- `registry.Registry` gains `Functions() []Function`, and `registry.Local` gains `RegisterFunction(name string, fn any)`; the GitHub registry delegates to its local registry. `NewConfig` calls a new `catalog.DeclareFunction` for each function, which converts the func and checks its name. A malformed func, or a name already used by a built-in or another application function, fails `NewConfig`.
- A plugin offers functions from its providers. `plugins.NewFunction(goFunc)` builds a `ProviderFunction` whose `Signature()` describes parameter and return types in cty JSON type form, and whose `Call(ctx, args)` takes a JSON array and returns JSON. `RegisterResourceProvider` collects each provider's `Functions().List()` into `PluginBase`.
- `PluginEntityProvider` and `PluginHost` gain `GetFunctions()` and `CallFunction()`. These map to the new additive `PluginService` RPCs `GetFunctions` and `CallFunction`. The handshake stays at protocol 1 and the RPCs only add to it, so libxcl's protocol v2 can carry them forward. A plugin built before this change answers `Unimplemented`, which the host reads as "no functions".
- When a plugin loads, `catalog.addHost` checks its function names against built-ins, application functions, functions already loaded and the plugin's own list. A clash fails the first operation with a `FunctionNameClashError` (errors package, re-exported from `xcl`) that names the function and both sources. This is the user's decision, option A: an application-function clash fails `NewConfig`, and a clash involving a plugin fails at plugin load.
- For each plugin function, the catalog builds a host-side cty function from the signature. Its `Impl` marshals the arguments with `internal/cty/json`, calls the host, and unmarshals the result.
- `Parser.getFunctions` merges the built-ins with `catalog.Functions()` and keeps the redaction wrapper. Every call is therefore evaluated during the walk, before any lifecycle call, and the plugin only ever sees the resulting value.

*Language.*
- **Template directives.** The fork's template parser (`internal/xcl/hclsyntax/parser_template.go:477`) emits an error diagnostic at the `%{` saying templating isn't part of XCL and pointing to functions. `${ }` stays.
- **Named blocks.**
  - The fork's `hcl.BlockHeaderSchema` gains a `LabelsOptional` flag, which `hclsyntax` honours in `structure.go:178-218`.
  - `gohcl` schema building treats a `map[string]T` block field as one optional name label. It treats a slice-of-struct block field with no label field or one label field the same way.
  - Decoding collects map blocks by name. It reports a block with no name ("a name is required") and a duplicate name (naming it, at the second block) as diagnostics. The duplicate-name check also applies to lists. A list keeps a block's name when the element has a label field.
  - Every `panic` in `gohcl` schema, decode and check that a Go type can trigger becomes an error. A new `gohcl.ValidateSchemaType` is run from `catalog.DeclareType` and `checkHostTypes`, so a bad field type is reported by `NewConfig` or at plugin load and never panics. `RegisterType` keeps its existing panics for malformed declarations only.
  - A map of blocks becomes a cty map (`internal/cty/gocty/type_implied.go:61-70`), so `app.web.listener.https.port` resolves through `GetAttr` on the map (`internal/xcl/ops.go:299-321`).
  - The parser paths that index blocks by position (`internal/parser/parser.go:1346,1430`, `diff_decode.go:70-85`) gain a map case keyed by name. `gohcl` encoding gains a map case so `xcl.Encode` can write named blocks.
- **Optional object attributes.** `gohcl.DecodeExpression` converts against a constraint type built with `cty.ObjectWithOptionalAttrs` from the `optional` tag kind. `gocty.fromCtyObject` leaves a null optional field at its zero value.
- **References in `for`.** Dependency extraction (`internal/parser/parser.go:1317,1412`) is rebuilt on `hclsyntax.Variables`, which walks every expression node and masks a `for` expression's key and value names.
- **Variables.** `DefaultVariableEnvPrefix = "XCL_VAR_"`. `Config` builds every operation's `ParserOptions` through one helper that sets the prefix. `setVariables` reads nothing when the prefix is empty and splits each entry on the first `=`. A variable `default` that refers to other values already resolves in dependency order; this work adds a test for it.

*Other repos.*
- **xcl-vscode:** a regression test fixes that `%{ … }` stays plain string scope while `${ … }` keeps interpolation scopes.
- **xcl-website:** the `HCL_VAR_` and "Around sixty" claims are corrected, along with the description of `Functions` on the plugin-template page.
- **xcl-plugin-template:** gains an example function, its tests, an example config that calls it, and a README section on offering a function.
- **xcl-plugin-docker:** builds against the new interface. Both plugin repos move to the xcl release that contains this work, which a person cuts first.

**Why this direction.** It is the only option that meets three of the spec's Technical Approach points at once: registration that looks like type registration, functions built on the existing interfaces, and language changes in XCL's own handling of the language. It also follows the binding plugin-layout design, where `Functions` stays a provider method. Clash reporting follows the user's option A answer. Applying the forked code follows the knowledge entries `conventions/never-modify-dependencies.md` and `decisions/own-hcl-fork-with-xcl-tags.md`: keep the MPL header, add the modification notice, record each change in `UPSTREAM.md`, and keep xcl-specific shaping such as `meta` trimming out of `gohcl`. The new error goes in the `errors` package and is re-exported from the root, following `conventions/shared-errors-package.md`. The evidence for each rejected option is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

**xcl library**

- **Forked template parser** (changed, `internal/xcl` hclsyntax)
  - Owns the grammar of string literals and heredocs.
  - Rejects every template directive (`%{ if }`, `%{ for }`, `%{ else }`, `%{ end… }`) with an error at the directive. The error says templating isn't part of XCL and points to functions.
  - Interpolation `${ }` and the `%%{` escape are unchanged.
  - Every XCL configuration, `.vars` file and module passes through it, so no other component repeats the rule.
- **Block structure handling** (changed, `internal/xcl` hcl schema and hclsyntax body)
  - Owns the label count check for each block type.
  - Gains an "optional label" form, so a block type may appear with or without one name.
- **Go-struct decoder** (changed, `internal/xcl` gohcl)
  - Owns how Go types map to bodies: schema building, checking, decoding and encoding.
  - New behaviour:
    - Collects named blocks into a `map[string]T` field.
    - Accepts named and unnamed blocks in a list, keeping the name when the element has a label field.
    - Reports a missing name for a map field, and duplicate names in maps and lists, at the offending block.
    - Converts object and map attribute values against a constraint type that honours `optional`.
  - Exposes `ValidateSchemaType`, so callers can reject an unsupported field type before decoding.
  - Returns an error wherever it used to panic on a user Go type.
  - The entity layer and the validate stage are its consumers.
- **Go↔cty value conversion** (changed, `internal/cty` gocty)
  - Owns turning cty values into Go values.
  - Leaves an optional field that arrives null at its zero value.
  - Map-of-block fields become cty maps, which is what makes reference by name work.
- **Reference extraction** (changed, parser dependency links)
  - Owns turning attribute expressions into entity links and the references recorded as written.
  - Rebuilt on the forked parser's variable walker, so traversals inside `for` expressions, index expressions, parentheses and relative traversals are all found, and `for` key/value names are ignored.
  - Written-reference paths and known-body lookups index map blocks by name instead of position.
  - Feeds the DAG and the eval context builder unchanged.
- **Variable resolution** (changed, parser context and Config)
  - Owns how variable values are overridden.
  - The environment prefix is the `XCL_VAR_` constant. Config sets it through one shared builder of parser options for every operation, replacing the four hand-built copies.
  - An empty prefix reads no environment variables.
  - A value is split on its first `=` only.
- **Function set** (changed, `internal/functions` and the parser)
  - Owns the functions available to an expression.
  - Built-ins stay as they are. Their names are exposed so the catalog can check clashes.
  - The parser merges built-ins with the catalog's functions instead of with the unused custom-function option. Every function keeps the sensitive-value redaction wrapper.
  - Go-func conversion gains bool parameters and slice/map types, shared with the plugin function helper.
- **Catalog** (changed, `internal/catalog`)
  - Already owns block types and loaded plugins. Now also owns functions:
    - declares application functions at `NewConfig`;
    - adds each plugin's functions when the plugin loads;
    - builds a host-side callable for each plugin function;
    - reports name clashes against built-ins, application functions and other plugins.
  - Checks each declared or plugin type with `ValidateSchemaType`.
- **Registries** (changed, `registry`)
  - The `Registry` interface gains `Functions()`. The local registry gains `RegisterFunction(name, fn)` beside `RegisterType`, and the GitHub registry delegates to its local registry.
  - Third-party registries must add the method, a listed breaking change.
- **Plugin function surface** (changed, `plugins`)
  - `ProviderFunction` is reshaped around a signature and JSON arguments and results.
  - `NewFunction(goFunc)` is the author's helper. A small `ProviderFunctions` implementation backs a provider's `Functions()` return.
  - `PluginBase` collects provider functions as providers register.
  - `PluginEntityProvider` and `PluginHost` gain the function listing and call methods, served by both the direct (in-process) host and the gRPC host.
- **Plugin protocol** (changed, `plugins/plugin.proto` and the generated code)
  - Gains the `GetFunctions` and `CallFunction` RPCs and their messages. The protocol is otherwise unchanged and the handshake stays at version 1.
  - The gRPC host treats an `Unimplemented` reply as a plugin with no functions.
- **Errors** (changed, `errors` and the root re-exports)
  - New `FunctionNameClashError`, naming the function and both sources with their registries.
  - New diagnostics wording for directives, missing names, duplicate names and unsupported field types.
- **Library docs and changelog** (changed)
  - Plugin developer docs describe `Functions` as functions offered to configuration.
  - Example tests use `XCL_VAR_`.
  - The CHANGELOG entry lists every breaking change.

**Other repos**

- **xcl-vscode grammar tests** (changed): fixture lines and scope assertions pin that directives are plain string text while interpolation keeps its scopes. The extension's changelog notes it.
- **xcl-website pages** (changed): the landing page, example pages and plugin template page state the `XCL_VAR_` prefix, the real built-in count, and that functions can come from the application or a plugin.
- **xcl-plugin-template** (changed): its provider offers one example function through `NewFunction`, with unit tests, an example config that calls it, end-to-end coverage, and a README section on adding a function. The xcl dependency moves to the new release.
- **xcl-plugin-docker** (changed): the xcl dependency moves to the new release. It builds and passes its tests with providers whose `Functions()` still return none.

## Data Structures & Interfaces

**Application functions (registry package)**

```go
// Function is a Go func a registry offers to configuration under Name.
type Function struct {
    Name string
    Func any // func(args...) (result, error)
}

type Registry interface {
    Name() string
    Types() []Type
    Functions() []Function // new; nil when the registry offers none
    Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error)
}

func (l *Local) RegisterFunction(name string, fn any) // new, beside RegisterType
```

`Functions` is read once, by `NewConfig`, just as `Types` is.

**Plugin functions (plugins package)**

```go
// ProviderFunction is reshaped: a typed signature plus a JSON call.
type ProviderFunction interface {
    Signature() FunctionSignature
    Call(ctx context.Context, args []byte) ([]byte, error) // args: JSON array; result: JSON value
}

type ProviderFunctions interface { // unchanged shape
    Get(name string) (ProviderFunction, error)
    Find(pattern string) (map[string]ProviderFunction, error)
    List() (map[string]ProviderFunction, error)
}

type FunctionSignature struct {
    Params   []FunctionParam
    Variadic *FunctionParam   // nil when there is none
    Return   json.RawMessage  // cty JSON type, e.g. "string", ["list","number"]
}
type FunctionParam struct { Name string; Type json.RawMessage }

func NewFunction(fn any) (ProviderFunction, error)          // author helper, built from a Go func
func NewFunctions(fns map[string]ProviderFunction) ProviderFunctions

type RegisteredFunction struct { Name string; Signature FunctionSignature }

// PluginEntityProvider and PluginHost each gain:
GetFunctions() []RegisteredFunction
CallFunction(ctx context.Context, name string, args []byte) ([]byte, error)
```

`FunctionSignature` is the language-neutral contract a future non-Go plugin produces under protocol v2. On the host it is turned into a cty function.

**Plugin protocol (plugin.proto), additive**

```proto
rpc GetFunctions(GetFunctionsRequest) returns (GetFunctionsResponse);
rpc CallFunction(CallFunctionRequest) returns (CallFunctionResponse);

message RegisteredFunction   { string name = 1; bytes signature = 2; } // FunctionSignature as JSON
message GetFunctionsResponse { repeated RegisteredFunction functions = 1; }
message CallFunctionRequest  { string name = 1; bytes args = 2; }       // JSON array
message CallFunctionResponse { bytes result = 1; string error = 2; }
```

No existing message or field is renumbered or retyped, and the go-plugin handshake stays at protocol version 1.

**Errors (errors package, re-exported from xcl)**

```go
// FunctionNameClashError is returned when a function name is offered twice.
// NewConfig returns it for an application function that clashes with a
// built-in or another application function. The first operation returns it
// when plugins load, for any clash involving a plugin.
type FunctionNameClashError struct {
    Name             string
    Provider         string // plugin name, or "function <Go type>" for an application function
    Registry         string
    Existing         string // "builtin", a plugin name, or "function <Go type>"
    ExistingRegistry string
}
```

`InvalidFunctionError` covers a func that cannot be converted: wrong return arity or an unsupported parameter type. `NewConfig` returns it, naming the function and its registry.

**Decoder contracts (internal/xcl)**

```go
type BlockHeaderSchema struct {
    Type           string
    LabelNames     []string
    LabelsOptional bool // new: a block may carry zero labels or all of them
}

func ValidateSchemaType(t reflect.Type) error // new: rejects field types gohcl cannot decode
```

**Go types authors write.** These are tags, not new types:

```go
type App struct {
    types.ResourceBase `xcl:",remain"`
    Listeners map[string]Listener `xcl:"listener,block"` // named blocks, keyed by name
    // or: Listeners []Listener `xcl:"listener,block"` (named and unnamed)
}
type Listener struct {
    Name string `xcl:"name,label"` // optional in a list; filled from the key in a map
    Port int    `xcl:"port"`
    TLS  bool   `xcl:"tls,optional"`
}
```

**Serialization boundaries**

- In references, a map-of-blocks field is a cty map of objects. In entity JSON (state, plugin calls, events) it is a JSON object keyed by name. The legacy plugin type schema already carries `map[string]struct` fields.
- Function arguments and results cross the plugin boundary as plain JSON, encoded and decoded against the signature's cty types. Sensitive arguments arrive unmarked, as real values, and the result is re-marked on the host.

**Parser options**

- `ParserOptions.CustomFunctions` is removed.
- `DefaultVariableEnvPrefix = "XCL_VAR_"` is exported from the parser package. Config always sets the prefix.

## Implementation Detail

**Functions follow the type-registration pattern.**
- A function moves from the registry, to the catalog at `NewConfig`, to the parser at walk time, the same route a declared type already takes.
  - The catalog is the one place that knows every function: built-ins (by name), application functions and plugin functions.
  - It reports clashes with the same error shape and the same timing as type clashes.
- The parser stops owning a function list. For each file it builds the built-ins, which still close over the file path for `file`, `dir` and `template_file`, and adds the catalog's functions.
- Every function is wrapped in the existing sensitive-value redaction. A developer reading the code finds functions handled next to types throughout, not in a parallel path.

**One Go-func reflection path.**
- Two jobs share one reflection helper in the functions package:
  - turning a Go func into a callable for configuration (application side);
  - turning a Go func into a signature plus a JSON call (plugin side).
- It gains bool parameters and slice and string-keyed map types. The application and plugin sides therefore accept the same func shapes, and an author's function behaves the same in-process and over gRPC.

**Plugin functions ride the existing host abstraction.**
- The direct host and the gRPC host already mirror each other for lifecycle calls. Function listing and calling are added to both in the same style:
  - the gRPC host caches the function list as it caches types;
  - calls carry the per-call id metadata, so a function's logs are routed as a provider call's are;
  - an `Unimplemented` reply from an older plugin is read as an empty list.
- On the plugin side, `PluginBase` collects functions from providers at registration, just as it collects types.
- Template and docker providers keep a `Functions()` method in the last position the plugin-layout design gives it. It returns `NewFunctions(...)`, or nil when a provider offers none.

**Language rules move into the forked parser.**
- This is new for XCL. Until now the fork was edited mainly for tags and encoding. With this change it also carries language policy:
  - the template parser refuses directives;
  - block headers can mark labels optional;
  - gohcl treats map block fields as first-class;
  - gohcl reports unsupported Go types as errors.
- Each change keeps the MPL header, gains the modification notice and gets an entry in `UPSTREAM.md`.
- Upstream tests for directives are deleted or rewritten to assert the new error. New fork tests use the non-table style already used for fork-local tests.
- xcl-specific shaping, such as `meta`, stays out of gohcl.

**Panics become positioned errors.**
- The decoder's panic sites become diagnostics with a subject range where one exists. Callers already collect diagnostics, so no recovery is introduced.
- A type-level check runs early, at declaration and at plugin load, through the catalog. A bad Go type is therefore reported once, before any configuration is read.

**Reference extraction by walking variables.**
- The hand-written expression switch used for dependency links is replaced by the parser's own variable walker, and each traversal is reduced to an entity address as before.
- The sensitive-value check already uses this pattern, so the codebase ends up with one way of finding references.
- Map-of-block paths, as written and in known-body lookups, use the block name instead of a position index, matching what the computed-value carry-over already does for maps.

**Config builds parser options once.**
- Validate, Apply, Destroy and Diff share one options builder on `Config`. Settings such as the environment prefix, and any later additions, cannot drift between operations.

**Cross-repo sequencing.**
- The xcl work lands and is released first.
- The plugin template and docker repos then move their dependency to that release, the same weekly path their `current` workflow already follows.
- The website and extension changes are independent of the release and only describe or test behaviour.

## Dependencies

- **`plugin-layout.md` from the `design` source** is the settled standard plugin layout. The template and docker changes keep to it, and `Functions` stays the last provider method in file order. The layout itself needs no change.
- **`libxcl.md` from the `design` source** is consulted for compatibility only. It is not referenced by this spec. The function RPCs must stay additive so its protocol v2 (`VersionedPlugins`) carries them forward. Its schema v1 must describe named-block maps, but that is the dependent python-support spec's work, not this plan's.
- **The downstream spec `20261011130709-aa3eb933-python-support`** builds on this plan's function registration and named-block maps, so this plan must land first. Nothing here waits on it.
- **The `internal/xcl` fork (HCL v2.21.0, MPL-2.0)** is changed: template parser, block header schema, label checks, and gohcl schema/decode/check/encode. Every change is recorded in `internal/xcl/UPSTREAM.md`.
- **The `internal/cty` fork (go-cty v1.15.0)** is changed: gocty leaves null optional fields at their zero value. The change is recorded in `internal/cty/UPSTREAM.md`. `cty/json` and `ObjectWithOptionalAttrs` are used as they are.
- **`github.com/hashicorp/go-plugin` v1.6.3, `google.golang.org/grpc` v1.73.0, `google.golang.org/protobuf` v1.36.6** carry the new RPCs. No version change is needed.
- **protoc with `protoc-gen-go` and `protoc-gen-go-grpc`** regenerate `plugins/proto` through the existing protobuf generation target in the Makefile. All three are installed on the development machine.
- **Built-in function set (`internal/functions`, raymond for `template_file`)** is unchanged. Its names are exposed for the clash check.
- **`github.com/jumppad-labs/xcl` release (new tag)** must exist before the xcl-plugin-template and xcl-plugin-docker dependency bumps. Both repos pin `v0.1.1` with no replace. Cutting the release is a person's task between milestones.
- **xcl-vscode's `vscode-textmate` and `vscode-oniguruma`** run the existing tokenize test. No change is needed.
- **xcl-website (Astro 5, MDX)** needs page text edits only. No page is removed, so no redirects are needed (`gotchas/no-redirects-on-page-removal`).
- **Prior plans** consulted as history only: `20261008071608-eb05cae0-config-and-plugin-registries` (registry and catalog shape), `20261009092551-82db0140-plugin-template` and `20261009102138-48e95432-docker-example-standard-layout` (repos this plan edits). None of them needs to change.

## Testing Approach

**Kinds of test**

- **Unit tests in the forks.** gohcl and hclsyntax get these for:
  - map block decoding, including missing name and duplicate name;
  - optional labels in lists;
  - every former panic site, which now returns an error;
  - `ValidateSchemaType`;
  - optional attributes inside object and map values;
  - directive rejection inside quoted strings and heredocs, with interpolation and the `%%{` escape still working.

  gocty gets a unit test for a null optional field decoding to its zero value. Directive-related upstream table cases that now fail are rewritten to assert the new error. New fork tests use the non-table style.
- **Parser tests** apply fixture configurations. They follow the existing `setupParser` / fixture-directory pattern, with registered fixture types for an `app` entity with a `listener` map and a `listener` list. They cover:
  - reference by name through a map of blocks;
  - a `for` expression over an entity, both as a dependency link and as the evaluated value;
  - links found inside index, parenthesis and relative-traversal expressions;
  - a variable default built from another entity;
  - the `XCL_VAR_` override;
  - unprefixed and old-prefix variables being ignored;
  - `variable.PATH` reported as unknown.
- **Catalog and registry tests** cover:
  - declaring an application function;
  - an app function clashing with a built-in or another app function, failing at declaration;
  - plugin function clashes against built-ins, app functions and another plugin, failing at load;
  - an unconvertible func returning `InvalidFunctionError`;
  - a bad block field type returning an error from declaration and from plugin load.
- **Plugin package tests** cover:
  - `NewFunction` signatures and calls;
  - `PluginBase` collecting provider functions;
  - the direct host and the gRPC host listing and calling functions, using the existing loopback-server style so both paths are proven equal;
  - an old plugin's `Unimplemented` answer giving no functions.
- **Root-package integration tests** run through `NewConfig` and `Apply`. Each spec acceptance criterion gets its own test, with the positive and negative cases in separate functions:
  - prefixed override applies; unprefixed and `HCL_VAR_` are ignored;
  - application function `slugify` is callable;
  - a plugin function is callable in-process and from a compiled fixture plugin binary;
  - the plugin receives the result, not the call, asserted on what a recording plugin's Create received;
  - each of the three function clash cases;
  - named blocks fill a map; an unnamed block into a map fails; a duplicate name fails; a list takes both kinds;
  - no panic on a bad field type;
  - reference by name works;
  - an optional field can be omitted in a map value;
  - `for` over an entity resolves;
  - a template directive is rejected while `${}` applies;
  - a variable default can refer to an entity.

  Per the user's decision, the plugin clash tests assert on the first operation's error, not on `NewConfig`.
- **Example module tests** (`example/plugin`) switch to `XCL_VAR_output_dir`. Their existing smoke tests then prove the override reaches a real binary.
- **xcl-vscode** gets regression scope assertions in its tokenize test, showing that directives stay plain string text and that `${}` keeps its scopes.
- **xcl-plugin-template** gets three kinds of test for its example function:
  - a unit test of the function itself;
  - an in-process e2e test applying an example config that calls the function;
  - an external e2e test doing the same through the built binary.
- **xcl-plugin-docker**: its existing unit, real-engine and e2e tests run unchanged against the new xcl release.

**Most coverage** goes to the gohcl decoder and the function path. They carry the new behaviour, the crash-safety guarantee and the cross-process contract.

**What the tests guarantee**
- XCL never panics on a Go type or configuration shape while reading blocks.
- Every function kind is evaluated before any lifecycle call.
- Function name clashes always name both sources.
- Named blocks are addressable by name.
- Environment variables reach configuration only through `XCL_VAR_`.

**Deliberate gaps**
- No tests inspect the website, README, docs or CHANGELOG. That is per convention; these are reviewed by a person.
- No fuzzing of gohcl. The panic sites are enumerated and each gets a test.
- The highlight package's directive colouring is not retested, because it is unchanged.

**Success metrics**

- *The language reference can document env overrides, application functions, named blocks into maps and references inside `for` as working behaviour, with no caveats.* Covered by a **behavioural test**: each of the four features has root-package integration tests driven by the spec's acceptance criteria.
- *Every claim on xcl.dev and in the README about the environment prefix, functions and templating matches what the latest release does when tried.* **Manual — captured in the implementation test plan.**
- *No configuration or Go type can make the library panic while reading block definitions; every problem is an error pointing at a position.* Covered by a **behavioural test**: one test per former panic site and per unsupported field type, asserting an error comes back, with a position where configuration is involved, and no panic.
- *A plugin author can add a working function to a plugin built from the template by following the template alone.* The template's example function e2e tests cover this as a **behavioural test**. Following the README from a fresh copy of the template is **Manual — captured in the implementation test plan**.

**Manual reviews**

- **Manual — captured in the implementation test plan:** review the xcl.dev page edits and the README. Check that no `HCL_VAR_` or "around sixty" text remains, and that the function count and the functions wording are right.
- **Manual — captured in the implementation test plan:** open a `.xcl` file with `%{ if … }` and `${ … }` in VS Code with the extension and check that only the interpolation is coloured as template syntax.
- **Manual — captured in the implementation test plan:** review the CHANGELOG entry's **Breaking:** list against the changed public surface: the `Registry` interface, `ProviderFunction`, `PluginHost`/`PluginEntityProvider`, the removal of `ParserOptions.CustomFunctions`, the env prefix, directive rejection, block label rules and the protocol additions.

## Milestones & Tasks

### Milestone 1: Configuration is read the way the XCL language says

**What changes**
- Environment overrides use `XCL_VAR_<name>`. Process environment variables no longer leak in as variables.
- A `for` expression over an entity resolves, and other references hidden inside index or parenthesised expressions are found as dependencies.
- A variable's default can be built from other values in dependency order. This is now tested.
- Object and map values written as attributes can leave out fields marked optional.
- A string that uses a `%{ … }` template directive is rejected with an error at the directive that points to functions, while `${ … }` keeps working.

These are breaking changes for anyone relying on `HCL_VAR_` or on directives.

**Validation point**: The root-package and parser tests for the prefixed override, unprefixed and old-prefix variables, `variable.PATH`, `for` over an entity, the variable default built from an entity, the optional field in a map value, and directive rejection all pass. The example plugin smoke tests pass with `XCL_VAR_output_dir`.

#### - [ ] Task: Environment overrides use the XCL_VAR_ prefix
**Id:** c3fd56b3-d059-479a-ba85-3641eb99b7a0
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Config builds its parser options in four places and never sets the environment prefix, so every process environment variable becomes a variable. This task moves the prefix to `XCL_VAR_` and gives Config one options builder that every operation uses. Reading the environment is changed so that an empty prefix reads nothing and values containing `=` survive. The example plugin's tests move to the new prefix.

*Technical detail:* [context.md#task-environment-overrides-use-the-xcl_var_-prefix](./context.md#task-environment-overrides-use-the-xcl_var_-prefix)

**Acceptance criteria**:
- [ ] With `XCL_VAR_region=us` set, an entity using `variable.region` (default `eu`) is applied with `us`.
- [ ] With only `region=us` or `HCL_VAR_region=us` set, the entity is applied with `eu`.
- [ ] Configuration that refers to `variable.PATH` is reported as an unknown variable.
- [ ] Validate, Apply, Destroy and Diff all read variables the same way.
- [ ] The example plugin's tests pass with the override given as `XCL_VAR_output_dir`.

#### - [ ] Task: References inside every kind of expression are found
**Id:** 849cfc81-42c0-4900-8143-64a81d6f6890
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Dependency links are found by a hand-written walk that skips `for` expressions, index expressions, parentheses and relative traversals. Without a link, the referenced entity is never added to the evaluation context, so `for name, l in app.web.listener` fails. This task rebuilds link extraction on the parser's own variable walker, which also ignores a `for` expression's own key and value names. It also adds a test showing that a variable default built from another entity resolves in dependency order.

*Technical detail:* [context.md#task-references-inside-every-kind-of-expression-are-found](./context.md#task-references-inside-every-kind-of-expression-are-found)

**Acceptance criteria**:
- [ ] An entity whose attribute is a `for` expression over another entity depends on that entity and is applied with the computed value.
- [ ] References inside index, parenthesised and relative-traversal expressions are recorded as dependencies.
- [ ] The key and value names of a `for` expression are never treated as references.
- [ ] `variable "url" { default = "http://${app.web.host}" }` used by another entity applies with the URL built from the applied host.
- [ ] References recorded as written are unchanged for configurations that worked before.

#### - [ ] Task: Optional fields can be left out of object and map values
**Id:** ec116aa9-82bd-4ce1-b57a-8fae05c8ba18
**Repo:** xcl
**Depends on:** none
**Execution:** agent

A struct field tagged `optional` can be omitted in a block, but not inside an object or map value written as an attribute: the converter treats every field as required. This task builds a conversion constraint that marks optional fields, used only when decoding an attribute expression. It also lets a field that arrives null keep its zero value. Both forks record the change.

*Technical detail:* [context.md#task-optional-fields-can-be-left-out-of-object-and-map-values](./context.md#task-optional-fields-can-be-left-out-of-object-and-map-values)

**Acceptance criteria**:
- [ ] `listeners = { http = { port = 80 } }`, where `tls` is optional, applies with `tls` at its zero value.
- [ ] An object attribute whose struct type marks a field optional can omit that field.
- [ ] Leaving out a required field in an object value is still reported as an error.
- [ ] Values encoded from Go types (state, events, references) are unaffected.

#### - [ ] Task: Template directives are rejected by the parser
**Id:** 5ee8ac01-91dc-4aca-bdb2-535e960d51e7
**Repo:** xcl
**Depends on:** none
**Execution:** agent

XCL drops string templating from the core. This task makes XCL's forked template parser report every `%{ … }` directive as an error at the directive. The error explains that templating isn't part of XCL and points to functions such as `format`, `join` and `template_file`. Interpolation with `${ … }` and the `%%{` escape keep working. Upstream tests that relied on directives are rewritten to expect the error.

*Technical detail:* [context.md#task-template-directives-are-rejected-by-the-parser](./context.md#task-template-directives-are-rejected-by-the-parser)

**Acceptance criteria**:
- [ ] A string containing `%{ if … }` or `%{ for … }`, quoted or in a heredoc, fails validation with an error at the directive's position that mentions functions.
- [ ] A string containing `${app.web.image}` still applies with the value filled in.
- [ ] A string containing `%%{` is read as the literal text `%{`.
- [ ] The fork's change is recorded in its upstream log.

### Milestone 2: Named child blocks are collected by name, and bad Go types never crash

**What changes**
- Child blocks with names can be read into a map field keyed by name, and configuration can refer to them by name, as in `app.web.listener.https.port`.
- Lists accept named and unnamed blocks, and keep the name when the Go type has a field for it.
- A missing name where a map needs one, and a duplicate name, are reported at the block.
- A Go type with a child-block field that cannot hold blocks is reported as an error when the config is created, or when its plugin loads, instead of crashing the application.

**Validation point**: The acceptance tests for named blocks filling a map, an unnamed block into a map, a duplicate name, a list taking both kinds, no crash on a bad field type, and reference by name all pass. Every former gohcl panic site has a test showing it now returns an error.

#### - [ ] Task: Named child blocks decode into maps and lists
**Id:** 2ca619f2-9b3d-4422-90b2-3f8ecc0aa643
**Repo:** xcl
**Depends on:** none
**Execution:** agent

A map field tagged as a block crashes the decoder, and a list must have all blocks named or none. This task teaches the forked block schema an optional name label. In the decoder:
- named blocks are collected into a `map[string]T` keyed by name;
- a list accepts named and unnamed blocks and keeps the name when the element has a label field;
- a missing name where a map is expected, and a duplicate name in a map or list, are reported at the block.

Encoding writes map blocks back with their names.

*Technical detail:* [context.md#task-named-child-blocks-decode-into-maps-and-lists](./context.md#task-named-child-blocks-decode-into-maps-and-lists)

**Acceptance criteria**:
- [ ] `listener "http" { port = 80 }` and `listener "https" { port = 443 }` read into a map field give keys `http` and `https` with ports 80 and 443.
- [ ] `listener { port = 80 }` read into a map field fails with an error at that block saying a name is required.
- [ ] Two `listener "http"` blocks fail with an error naming `http` at the second block.
- [ ] `listener "http" { port = 80 }` and `listener { port = 8080 }` read into a list whose element keeps the name give one item named `http` and one with an empty name.
- [ ] Blocks with two or more labels, and single (non-list, non-map) blocks, keep their current label rules.
- [ ] An entity holding a map of blocks is written back as configuration with each block's name.

#### - [ ] Task: Unsupported Go field types return errors instead of panics
**Id:** 0a9af403-6c29-4819-b2b8-6de19282c8ce
**Repo:** xcl
**Depends on:**
- 2ca619f2-9b3d-4422-90b2-3f8ecc0aa643 — Named child blocks decode into maps and lists
**Execution:** agent

The forked decoder panics on Go field types it cannot handle, which crashes the embedding application. This task turns every such panic into an error and adds a type check that runs when a Go type is declared or a plugin's type loads. A bad type is therefore reported by creating the config, or by the first operation for a plugin type, before any configuration is read.

*Technical detail:* [context.md#task-unsupported-go-field-types-return-errors-instead-of-panics](./context.md#task-unsupported-go-field-types-return-errors-instead-of-panics)

**Acceptance criteria**:
- [ ] Registering a type whose child-block field is a string returns an error from `NewConfig`, and the application does not panic.
- [ ] A plugin type with an unsupported block field fails plugin loading with an error naming the type and field.
- [ ] Each former panic case returns an error instead: a block tag on a non-struct field, an invalid tag, two `remain` fields, a non-string label field, and an attribute type with no configuration form.
- [ ] Problems found while reading configuration point at a position in the file.

#### - [ ] Task: Entities with named block maps work end to end
**Id:** 35103d66-1b9d-4867-8473-2c04efa600e1
**Repo:** xcl
**Depends on:**
- 2ca619f2-9b3d-4422-90b2-3f8ecc0aa643 — Named child blocks decode into maps and lists
- 849cfc81-42c0-4900-8143-64a81d6f6890 — References inside every kind of expression are found
**Execution:** agent

Several parts of the parser address child blocks by position:
- references recorded as written;
- the known-body lookups used when deciding changes;
- the computed-value carry-over.

This task makes them address map blocks by name, so named blocks behave correctly through apply, diff and state. It adds the root-package acceptance tests that use the spec's `app "web"` entity.

*Technical detail:* [context.md#task-entities-with-named-block-maps-work-end-to-end](./context.md#task-entities-with-named-block-maps-work-end-to-end)

**Acceptance criteria**:
- [ ] Another entity's `port = app.web.listener.https.port` applies and holds 443.
- [ ] `ports = { for name, l in app.web.listener : name => l.port }` applies and holds `{ http = 80, https = 443 }`.
- [ ] References to a named block are recorded with the block's name.
- [ ] A second apply of the same configuration with named blocks reports no changes.
- [ ] Changing one named block's attribute is reported as a change to that block only.

### Milestone 3: Applications and plugins can add functions to configuration

**What changes**
- An application registers its own Go function on its registry next to its types, and configuration can call it.
- A plugin's providers can offer functions, and configuration calls them across the plugin boundary, in process or over gRPC.
- Every function call is evaluated while configuration is read, so a plugin's lifecycle only ever sees the result.
- Two sources offering the same function name are reported with both named:
  - when the config is created, for application functions;
  - when plugins load on the first operation, for anything involving a plugin.

The registry interface and the plugin function interfaces change, which is listed as breaking.

**Validation point**: The acceptance tests for an application function, a plugin function (in process and from a compiled fixture plugin), the plugin receiving the result rather than the call, and the three function clash cases all pass. The full xcl test suite passes with the regenerated protocol code.

#### - [ ] Task: Applications register their own functions
**Id:** 864d2374-e851-445e-831c-4c654afaea3c
**Repo:** xcl
**Depends on:** none
**Execution:** agent

An application registers a Go function on its local registry, next to its types, and configuration can call it by its bare name. This task:
- adds `Functions()` to the registry interface and `RegisterFunction` to the local and GitHub registries;
- makes the catalog own the function set and check names against the built-ins and other application functions when the config is created;
- makes the parser take its functions from the catalog, still wrapped for sensitive values.

It also adds the function name clash and invalid function errors.

*Technical detail:* [context.md#task-applications-register-their-own-functions](./context.md#task-applications-register-their-own-functions)

**Acceptance criteria**:
- [ ] An application registers `slugify` with no plugin, and `name = slugify("My App")` applies with the function's result.
- [ ] Registering an application function named `upper` makes `NewConfig` fail with an error naming `upper`, the application function and "builtin".
- [ ] Two application functions with the same name, in one registry or two, make `NewConfig` fail, naming both and their registries.
- [ ] A function whose signature cannot be used, such as a wrong return count or an unsupported parameter type, makes `NewConfig` fail with an error naming it.
- [ ] Sensitive arguments to an application function are still kept out of its errors, and its result is still marked sensitive.

#### - [ ] Task: Plugin providers offer functions
**Id:** 337c348e-bb0b-4de0-ac24-ad7d593d7419
**Repo:** xcl
**Depends on:**
- 864d2374-e851-445e-831c-4c654afaea3c — Applications register their own functions
**Execution:** agent

This builds the plugin side of functions on the provider interfaces that already exist but were never used:
- the provider function contract gains a typed signature and JSON arguments and results;
- `NewFunction` builds one from a plain Go func;
- the plugin base collects every provider's functions as providers register;
- plugins and plugin hosts gain function listing and calling, and the in-process host serves them directly.

*Technical detail:* [context.md#task-plugin-providers-offer-functions](./context.md#task-plugin-providers-offer-functions)

**Acceptance criteria**:
- [ ] A provider whose `Functions()` returns a function built with `NewFunction` has that function listed by its in-process plugin host, with its parameter and return types.
- [ ] Calling the function through the in-process host returns the Go function's result.
- [ ] A function error is returned to the caller with its message.
- [ ] Two providers of one plugin offering the same function name is reported as a clash within that plugin.
- [ ] Providers whose `Functions()` return nil still register and work unchanged.

#### - [ ] Task: Plugin functions are called from configuration over gRPC
**Id:** 8d8d4e04-5f2c-4a22-876d-d59a04e72782
**Repo:** xcl
**Depends on:**
- 337c348e-bb0b-4de0-ac24-ad7d593d7419 — Plugin providers offer functions
**Execution:** agent

This connects plugin functions to configuration.
- The plugin protocol gains two additive calls, list functions and call a function, and the gRPC host and server implement them. A plugin built before this change still loads and is treated as offering none.
- When a plugin loads, the catalog adds its functions, building a configuration function from each signature. It reports a clash against built-ins, application functions and other plugins, as the first operation's error.
- Calls are evaluated while configuration is read, so a plugin's lifecycle receives only results.

*Technical detail:* [context.md#task-plugin-functions-are-called-from-configuration-over-grpc](./context.md#task-plugin-functions-are-called-from-configuration-over-grpc)

**Acceptance criteria**:
- [ ] Configuration that loads a compiled fixture plugin and calls its function applies, and the entity holds the result. The same holds for the plugin registered in process.
- [ ] When an entity handled by a plugin has an attribute set by a function call, the plugin's create step receives the function's result.
- [ ] An application function with the same name as a loaded plugin's function makes the first operation fail with an error naming the function, the application function and the plugin.
- [ ] Two loaded plugins offering the same function name make the first operation fail, naming both plugins and their registries.
- [ ] A plugin function named like a built-in fails plugin loading, naming "builtin".
- [ ] A plugin binary built against the previous protocol still loads and runs its lifecycle.

### Milestone 4: The website, editor extension and plugin repos match the language

**What changes**
- xcl.dev states the `XCL_VAR_` prefix and the real number of built-in functions, and describes functions offered by applications and plugins. A plugin's `Functions` method is described as offering functions to configuration.
- The editor extension has a test pinning that template directives are not highlighted as template syntax.
- The xcl changelog lists every breaking change.
- After a person cuts an xcl release containing milestones 1–3:
  - the plugin template shows a working example function, with tests and a README section an author can follow;
  - the example Docker plugin builds and passes its tests against the new release.

**Validation point**: The xcl-vscode tokenize test passes. The template's unit and end-to-end tests, including its example function, pass against the new xcl release. The Docker plugin's tests pass against that release. The manual reviews of the site, the extension and the changelog listed in the Testing Approach are carried into the implementation test plan.

#### - [ ] Task: Document the changed library surface and its breaking changes
**Id:** 07200883-19d9-4082-8722-cb058114d5f6
**Repo:** xcl
**Depends on:**
- c3fd56b3-d059-479a-ba85-3641eb99b7a0 — Environment overrides use the XCL_VAR_ prefix
- 849cfc81-42c0-4900-8143-64a81d6f6890 — References inside every kind of expression are found
- ec116aa9-82bd-4ce1-b57a-8fae05c8ba18 — Optional fields can be left out of object and map values
- 5ee8ac01-91dc-4aca-bdb2-535e960d51e7 — Template directives are rejected by the parser
- 0a9af403-6c29-4819-b2b8-6de19282c8ce — Unsupported Go field types return errors instead of panics
- 35103d66-1b9d-4867-8473-2c04efa600e1 — Entities with named block maps work end to end
- 8d8d4e04-5f2c-4a22-876d-d59a04e72782 — Plugin functions are called from configuration over gRPC
**Execution:** agent

The xcl repository's own docs describe a provider's `Functions` as something other providers call, and nothing describes application functions. This task updates the plugin docs to say plugins offer functions to configuration, and documents registering an application function. It adds the changelog entry for this spec, with every breaking change listed under **Breaking:**.

*Technical detail:* [context.md#task-document-the-changed-library-surface-and-its-breaking-changes](./context.md#task-document-the-changed-library-surface-and-its-breaking-changes)

**Acceptance criteria**:
- [ ] The plugin developer docs explain how a provider offers a function to configuration, with a short example.
- [ ] The docs show how an application registers a function next to its types.
- [ ] The xcl changelog has an entry for this spec whose **Breaking:** list covers each item:
  - the environment prefix;
  - directive rejection;
  - block label rules;
  - the registry interface;
  - the provider function interface;
  - the plugin and host interfaces;
  - the removed parser option;
  - the protocol additions.
- [ ] Nothing in the xcl README or docs mentions `HCL_VAR_`.

#### - [ ] Task: Release xcl with the language surface changes
**Id:** a99fef3e-e316-4f4a-96a7-ab9c53c975de
**Repo:** xcl
**Depends on:**
- 07200883-19d9-4082-8722-cb058114d5f6 — Document the changed library surface and its breaking changes
**Execution:** human — publishing a release tag of xcl is an action outside the repo's code

The plugin template and the Docker plugin depend on a published xcl version. A person merges the work and tags a new xcl release, so those repos can move to it.

*Technical detail:* [context.md#task-release-xcl-with-the-language-surface-changes](./context.md#task-release-xcl-with-the-language-surface-changes)

**Acceptance criteria**:
- [ ] A tagged xcl release containing milestones 1 to 3 is published and can be fetched as a Go module dependency.

#### - [ ] Task: The plugin template offers an example function
**Id:** f1a9335f-9a09-4a6b-ae52-6b45a4f9885d
**Repo:** xcl-plugin-template
**Depends on:**
- a99fef3e-e316-4f4a-96a7-ab9c53c975de — Release xcl with the language surface changes
**Execution:** agent

The template moves to the new xcl release, and its note provider offers one example function through `NewFunction`, keeping the standard plugin layout. The task adds:
- a unit test for the function;
- an example configuration that calls the function;
- end-to-end coverage, in process and as a separate program;
- a README section that walks an author through adding their own function.

*Technical detail:* [context.md#task-the-plugin-template-offers-an-example-function](./context.md#task-the-plugin-template-offers-an-example-function)

**Acceptance criteria**:
- [ ] The template builds against the new xcl release.
- [ ] Its example configuration calls the example function, and the applied note holds the function's result, in process and as a separate program.
- [ ] The README explains, step by step, how to add a function to a provider.
- [ ] The provider file keeps `Functions` last, as the standard layout requires.

#### - [ ] Task: The Docker plugin builds against the new plugin interface
**Id:** 7e0829b9-29a8-464c-a319-0a34c9da9772
**Repo:** xcl-plugin-docker
**Depends on:**
- a99fef3e-e316-4f4a-96a7-ab9c53c975de — Release xcl with the language surface changes
**Execution:** agent

The example Docker plugin moves to the new xcl release and is adjusted wherever the changed interfaces require it. Its providers still offer no functions.

*Technical detail:* [context.md#task-the-docker-plugin-builds-against-the-new-plugin-interface](./context.md#task-the-docker-plugin-builds-against-the-new-plugin-interface)

**Acceptance criteria**:
- [ ] The Docker plugin builds against the new xcl release.
- [ ] Its unit, real-engine and end-to-end tests pass. Real-engine tests skip when no engine is reachable, as today.

#### - [ ] Task: The website states what the language does
**Id:** dd7d5d1c-8d45-4e9b-99e1-702959482f91
**Repo:** xcl-website
**Depends on:**
- a99fef3e-e316-4f4a-96a7-ab9c53c975de — Release xcl with the language surface changes
**Execution:** agent

xcl.dev says variables are overridden with `HCL_VAR_` and that there are around sixty functions, and it describes a provider's `Functions` vaguely. This task corrects:
- the environment prefix on the landing page and the example pages;
- the function card, so it gives the real built-in count and mentions application and plugin functions;
- the plugin template page's description of `Functions`.

*Technical detail:* [context.md#task-the-website-states-what-the-language-does](./context.md#task-the-website-states-what-the-language-does)

**Acceptance criteria**:
- [ ] No page on the site mentions `HCL_VAR_` or "around sixty". Every environment override example uses `XCL_VAR_`.
- [ ] The function card's count matches the number of built-in functions, and it says applications and plugins can add functions.
- [ ] The plugin template page says a provider's `Functions` offers functions to configuration.
- [ ] No page is removed or renamed.

#### - [ ] Task: The editor extension does not highlight template directives
**Id:** 99c29411-9080-4863-82ef-237b85d1b9ba
**Repo:** xcl-vscode
**Depends on:** none
**Execution:** agent

The extension's grammar has no rule for `%{ … }`, so directives already show as plain string text. This task pins that with fixture lines and scope assertions for quoted strings and heredocs, keeps the `${ … }` interpolation assertions, and notes the change in the extension's changelog.

*Technical detail:* [context.md#task-the-editor-extension-does-not-highlight-template-directives](./context.md#task-the-editor-extension-does-not-highlight-template-directives)

**Acceptance criteria**:
- [ ] In the tokenize test, `%{ if … }` and `%{ for … }` inside a string and a heredoc are string text, with no interpolation or template scopes.
- [ ] `${ … }` inside a string is still scoped as interpolation.
- [ ] The extension's changelog records that template directives are not part of XCL highlighting.

## Open Questions

- **Can the template parser recover cleanly after rejecting a directive?** This depends on how `parseTemplateParts` unwinds nested `if`/`for`/`end` tokens once it stops building directive nodes, which only shows when the change is made. The implementer should keep the error at the directive and accept follow-on diagnostics if recovery is imperfect. Stop and ask only if rejection would also break `${ }` parsing.
- **Does `Validate` report an unknown `variable.PATH`, or only `Apply`?** This depends on whether validation evaluates variable references against the context. Validate does not decode today, and `validateReferences` checks reference roots. The implementer writes the acceptance test against whichever operation reports it. If neither does, stop and ask, because the spec's acceptance criterion would fail.

No other questions remain open. Every design choice is settled in the architecture and recorded in the assumption log.

## Out of Scope

- No `for_each`, dynamic blocks or any other way of generating blocks from a value (spec non-goal; may be reconsidered if the community asks).
- No `local` block (spec non-goal). The editor grammar's `local` keyword is also left alone, because the extension's only change here concerns directives.
- No templating function or plugin. The existing `template_file` built-in is unchanged.
- No removal of, or warning on, map and object attributes. Documentation simply leads with named blocks.
- No other expression-language changes. Operators, conditionals, splat, indexing, `for` and the built-ins behave as today, apart from the reference-extraction fix.
- The language reference and the "Using XCL with Go" guide are not written here. Their own specs in the `20261010160411-19b965db-language-reference` epic cover them. This plan only corrects existing site claims.
- No API renames, for example `types.ResourceBase` to entity naming.
- No formatting support in the editor extension.
- No plugin protocol v2, version negotiation or schema v1. Those belong to the libxcl design and the `20261011130709-aa3eb933-python-support` spec, which will carry the additive function RPCs forward and describe named-block maps in schema v1.
- No functions written in Python or Node, whether host-language or in non-Go plugins. They are deferred to the libxcl work; plugins and their functions are Go only (spec constraint).
- No namespacing of function names. Names stay bare (spec Technical Approach).
- The `Init(state, functions, logger)` provider parameter keeps receiving nil. Letting one provider call another's functions is a separate feature.
- The `highlight` package's directive colouring is not changed.
- No change to the module boundary. Configuration reaches into a module only through its outputs.
