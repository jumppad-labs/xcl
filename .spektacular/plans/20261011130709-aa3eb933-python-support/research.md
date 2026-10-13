---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Research: 20261011130709-aa3eb933-python-support

## Alternatives considered and rejected

- **Python functions offered through a synthetic in-process plugin** (coreservice registers a `registry.Plugin` whose host answers `GetFunctions`/`CallFunction` by enqueuing a FunctionTask). Rejected: language-surface fixes that clashes involving a plugin fail the *first operation*, while an application's own function clashes fail `NewConfig`. A Python program's functions are application functions, and the spec's constraint says they must behave exactly as Go application functions do. The plugin route also names a plugin in `FunctionNameClashError`, and the catalog calls plugin functions with `context.Background()` (language-surface context.md, task "Plugin functions are called from configuration over gRPC"), so it gives no cancellation either.
- **Exposing `internal/functions` or the cty function type to coreservice so it registers a cty function directly.** Rejected: the libxcl design binds `coreservice` to import only the public packages of the root module ("Where coreservice needs something internal, that something is made public in the root module first", design § Layers). The public way to add an application function is `registry.Local.RegisterFunction(name, fn any)` (language-surface rule), which takes a Go func.
- **Two Python implementations (sync and asyncio) each with its own loop.** Rejected by the spec's Technical Approach ("two faces over the single poll loop").
- **Building provider tasks, `ForeignProviderAdapter`, `DefaultChangedJSON`, `Log`/`StateGet`/`StateFind` RPCs, plugin protocol v2, `xcl.Plugin.serve()`.** Rejected: spec Non-Goals and interview decision (Go-compiled plugins only; no in-process Python providers).
- **Moving `PropertyChange`/`DependencyChange` to a shared `proto/xcl/types/v1/types.proto` now.** Deferred: only provider tasks need the change lists in protobuf form; diffs cross as `diff_json` (design § Operations, `OperationFinished.diff_json`). Moving them now would touch `plugins/plugin.proto` for nothing this spec uses.
- **`DescribeTypes` and `python -m xcl generate`.** Not required by any requirement or acceptance criterion; the spec asks that unregistered types be "readable generically" (`xcl.Entity`). Left for a later spec; field/method numbers are not reserved because adding a method is protobuf-compatible.
- **Generating Python RPC stubs for CoreService.** Rejected by the design (§ Generating code: message classes only, typed client from the descriptor).
- **Downloading libxcl at first use instead of bundling it in wheels.** Rejected by the design (§ Bundled platform packages) and by the spec ("without Go or a C compiler", offline-friendly).
- **cgo in the root module behind a build tag.** Rejected by the design (§ Why the shared library is a nested module); the `CGO_ENABLED=0` build in `.github/workflows/go.yml:24-28` is the guard.
- **Mapping a declared `number` parameter to `int64` in the reflected Go func.** Rejected: cty `number` holds floats too; `float64` is what JSON numbers decode to and what language-surface's Go-func conversion accepts as a numeric kind. Integer precision beyond 2^53 is a recorded limitation.

## Chosen approach — evidence

- `config.go:337-516` — Validate/Apply/Destroy/Load all go through `c.run` (`config.go:547-610`), which today creates `context.Background()` (`:551`) or `context.WithCancel(context.Background())` (`:566`). A context-taking form only needs `run` to take the caller's ctx; cancellation already stops new provider calls and saves progress (`config.go:395-399` comment, `:586-596`).
- `config_diff.go:34` — `Diff(paths, options...)` is the fifth operation; needs `DiffContext`.
- `query.go:212-250` — `entitiesOf(want reflect.Type, path...)` holds the FindByType rules (`typeable`, `NotTypeableError`, `UnknownTypeError`) but is private and requires a Go type; `query.go:273-288` `oneOf` gives `NotUniqueError`. coreservice needs an untyped, public form with the same errors.
- `config.go:262-280` — `Entities()` and `FindResource(path)` are already untyped and public; `config.go:309-330` `Outputs()` returns `map[string]any`.
- `encode.go:169` — `Config.EncodeSavedEntity(data, options...)` is public, maps directly to the design's `EncodeSavedEntity`.
- `options.go:22-178` — every `CreateConfigRequest` field has a matching option: `WithRegistry`, `WithStateStore`, `WithStatePath`, `WithStateMask`, `WithEventHandler`, `WithEventBufferSize`, `WithVariables`, `WithEventData`, `WithEventMask`, `WithNoEventMask`.
- `state/state_store.go:9-38` — `StateStore` exchanges `[]any`; `Load` may return `json.RawMessage`; so a `ForeignStateStore` needs no library types.
- `events/events.go:72-125` — `Event` fields map 1:1 to the design's `Event` message; `Error` is an `error`, `Meta` is `map[string]any`.
- `registry/registry.go:26-52` — `Registry{Name, Types, Plugins}` + language-surface's `Functions()`; `Type{Type, Subtype, Prototype}` is what a schema-built type becomes.
- `registry/local.go:60-155` — `NewLocal`, `RegisterType`, `RegisterExternalPlugin`, `RegisterPluginDirectory`, `PluginPattern` map the design's `LocalRegistry` message.
- `internal/schema/deserialize.go:14` `CreateInstanceFromSchema` + `known_types.go:22` `KnownTypes` — the legacy reader and the sensitive instantiations a v1 reader is limited to.
- `internal/catalog` + language-surface `catalog.DeclareFunction` — converts the registry's Go func with `CreateCtyFunctionFromGoFunc`, which supports string, bool, numeric kinds, slices and string-keyed maps. `reflect.FuncOf` + `reflect.MakeFunc` can build a Go func of exactly those shapes at run time from a `FunctionSignature`, so coreservice registers Python functions through the public `Functions()` path with no root-module change.
- `.github/workflows/go.yml` — only build/test jobs; there is no release workflow in xcl today (tags `v0.1.0`…`v0.2.1` were pushed by hand). The plugin template's release (`make dist`, gpg, gh) is the in-house precedent for signed release assets (CHANGELOG entry `20261009092551-82db0140-plugin-template`).
- `registry/github_assets.go`, `registry/github_verify.go` — the release asset contract (`<name>_<version>_<os>_<arch>.tar.gz`, `_checksums.txt`, `.sig`) libxcl's GitHub assets reuse.
- `Makefile` `protos` target — protoc today; design moves generation to `buf`.
- Knowledge `gotchas/plugin-types-rebuilt-with-structof.md` — the v1 reader must use the known-type map for sensitive generics; an unknown named struct silently becomes `struct{}`.
- Knowledge `gotchas/registered-types-convert-only-to-themselves.md` — schema-built (anonymous) types convert by JSON round trip, which is exactly how coreservice hands entities to Python.
- Knowledge `architecture/config-is-the-public-query-surface.md` — lookups go through `Config`, which is why the untyped lookups are added to `Config`, not to `state`.

## Files examined

- `xcl:config.go:26-170` — sentinel and detail-type re-exports the error-code table maps.
- `xcl:config.go:173-258` — `Config` fields and `NewConfig`: types declared per registry, catalog built, event mask default.
- `xcl:config.go:337-516` — Validate/Apply/Destroy/Load bodies; parser options built inline (language-surface replaces with `c.parserOptions`).
- `xcl:config.go:547-640` — `run` and `withPlugins`; the only place a context is created.
- `xcl:config_diff.go:34` — Diff.
- `xcl:options.go:13-190` — every ConfigOption.
- `xcl:query.go:29-400` — Find/FindByType/FindOne/All generic lookups, `entitiesOf`, `typeable`, `oneOf`, `typePath`.
- `xcl:query_methods_go127.go:23-45` — method forms (Go 1.27 generic methods).
- `xcl:encode.go:141-196` — `EncodeEntity`, `EncodeSavedEntity`.
- `xcl:events/events.go:60-171` — Event shape, Handler, Emit, DataLevel.
- `xcl:state/state_store.go:1-38` — StateStore contract.
- `xcl:registry/registry.go:1-80` — Registry, Type, Plugin, InProcess.
- `xcl:registry/local.go:45-216` — local registry API.
- `xcl:internal/schema/*.go` — legacy serializer/deserializer, `KnownTypes`.
- `xcl:internal/savedentity/savedentity.go:55,129` — decoding saved records (used by Load and event decoding).
- `xcl:internal/wire/wire.go:74-104` — the JSON encoder carrying real sensitive values.
- `xcl:.github/workflows/go.yml` — CGO_ENABLED=0 cross builds, race tests, Go 1.25 minimum job.
- `xcl:Makefile` — protos, mocks, examples list (nested modules are listed in `EXAMPLES`).
- `xcl:CHANGELOG.md:1-40` — entry format: `## <spec name>`, prose, `**Breaking:**` list or "None.".
- `xcl:e2e/fixtures/recorder/recorder.go` — a recording plugin fixture to model the cancellation test plugin on.
- `xcl:go.mod:1-5` — module `github.com/jumppad-labs/xcl`, `go 1.25.0`.

## External references

- golang/go#18976 — two Go runtimes in one process unsupported; why libxcl is bundled only by the `xcl` package.
- golang/go#31860 — no fork after the Go runtime starts; why lazy load and `ForkedProcessError`.
- cffi ABI mode docs — `ffi.dlopen`, GIL released around foreign calls; why one `py3-none-<platform>` wheel per platform serves every CPython.
- PEP 600 / PEP 656 — `manylinux_2_28` and `musllinux_1_2` platform tags.
- PyPI Trusted Publishing docs — OIDC publishing from a GitHub workflow, no API token.
- buf docs (`buf generate`, `buf breaking`, `buf lint`) — the design's contract-stability tooling.
- Temporal sdk-core ARCHITECTURE.md — the poll/complete inversion the design follows.
- https://docs.pypi.org/project-management/storage-limits/ — per-file 100 MB, project 10 GB.

## Prior plans / specs consulted

- `20261011095738-fad05e41-language-surface` (plan, final) — decides `registry.Local.RegisterFunction`, `Registry.Functions()`, `FunctionSignature` JSON with cty JSON type strings, supported arg types, clash timing (app clash at NewConfig), `FunctionNameClashError`/`InvalidFunctionError`/`InvalidTypeError` in `errors/`, named block maps `map[string]T` with `xcl:"name,block"` plus optional name label in lists, `XCL_VAR_`, CHANGELOG format, and that its release is v0.3.0. It explicitly leaves "schema v1 must describe named-block maps" to this spec.
- Design `libxcl.md` (`design` source, binding) — the whole architecture: coreservice, C ABI, CoreService messages, poll loop, schema v1, Python SDK layout, packaging and shipping.

## Open assumptions

- The PyPI project name `xcl` is available to jumppad-labs (or already owned). If not, the implement workflow must STOP and ask: the spec fixes `pip install xcl`.
- The jumppad-labs plugin signing key (used by the plugin template and docker plugin releases) can also sign libxcl release assets, and is available to xcl's release workflow as a repository secret.
- `reflect.FuncOf`/`reflect.MakeFunc` funcs are accepted by language-surface's `CreateCtyFunctionFromGoFunc` exactly like hand-written funcs (they are ordinary func values to reflection). If the conversion inspects anything a made func lacks, STOP and ask.
- Language-surface's Go-func conversion supports variadic funcs (its `FunctionSignature` has `Variadic`). If it does not, Python variadic functions are refused with `InvalidFunctionError` rather than the root module being changed.
- GitHub provides `ubuntu-24.04-arm`, `windows-11-arm`, an Intel macOS runner and `macos-latest` (arm64) for the install checks; musl is checked in Alpine containers on the two Linux runners.
- The Docker example plugin's published release (jumppad-labs/xcl-plugin-docker) is signed and built for linux, darwin and windows, so the Python registry test can install it.
- The xcl release that ships this spec follows v0.3.0 (language-surface) and is a minor, non-breaking release (v0.4.0).

## Drafting assumptions

### Chosen direction (architecture)
- **Decision**: Option 1. Python functions are application functions that coreservice builds as reflected Go funcs from a declared signature. A single Python poll engine has a sync driver and an asyncio driver. libxcl is built, signed and published once by an xcl tag workflow, which then dispatches the xcl-python release that wraps the verified archives into eight wheels.
- **Key design decisions**:
  - Only the design's surface this spec needs is built, and the provider parts are reserved by field number.
  - The root module gains context-taking operations, untyped lookups and `registry.TypeFromSchema`, all public and additive.
  - Schema v1 is extended for map blocks and labels.
  - Cancellation is carried by the operation context that every task waits under.
- **Rationale**: this is the only option that keeps Python functions identical to Go application functions (spec constraint, language-surface clash timing) while using public API only (design § Layers).
- **Rejected**: option 2, a synthetic function plugin (wrong clash timing, no cancellation); option 3, per-SDK libxcl builds (unsigned duplicates, and no assets on the xcl release).

### Conventions selected (architecture)
- **Decision**: applied testing-and-mocking (extended to pytest by analogy), no-repo-file tests, shared test helpers, real-apply state, shared errors package, project structure, dependencies, code style, patterns, development standards, graph-parent ordering, and the entity glossary. Dropped never-modify-dependencies, database, and provider Changed/Update.
- **Rationale**: these are the surfaces the work touches. The Go test conventions are carried into Python by analogy, because the project has no Python conventions.
- **Rejected**: inventing separate Python conventions in the knowledge base during planning (that is a knowledge write the user must confirm).

### New error codes for language-surface errors (architecture)
- **Decision**: add the codes `function_name_clash`, `invalid_function` and `invalid_type` to the design's error-code table. Python raises `FunctionNameClashError`, `InvalidFunctionError` and `InvalidTypeError` for them.
- **Rationale**: the language-surface rule requires mapping these errors to Python exception classes, and the design lets codes be added but never renamed.
- **Rejected**: folding them into `internal` (they would not be catchable by kind).

### CoreService carries only what this spec builds (discovery)
- **Decision**: `core.proto` defines the design's methods and messages except provider work: no `ProviderTask`, `ProviderResult`, `Log`, `StateGet`, `StateFind`, `DescribeTypes`, and no `DeclaredType.provided`/`implements`. Their design field numbers are `reserved` so the later Python-providers spec adds them with the design's numbers. `FunctionTask`/`FunctionResult` and `LocalRegistry.functions` are added.
- **Rationale**: spec Technical Approach ("provider registration and provider tasks aren't needed") and the interview decision; the design's own test holds every service method to a handler, so the service lists only what is implemented.
- **Rejected**: declaring the full design surface and answering unbuilt methods with an error (dead contract surface, untestable).

### Shared types.proto move deferred (discovery)
- **Decision**: `PropertyChange` etc. stay in `plugins/plugin.proto`; no `proto/xcl/types/v1/types.proto` yet.
- **Rationale**: only provider tasks need change lists; diffs cross as JSON.
- **Rejected**: moving them now (churn in plugin.proto with no consumer).

### No DescribeTypes or code generation (discovery)
- **Decision**: `DescribeTypes` and `python -m xcl generate` are not built; plugin entities are read as `xcl.Entity`.
- **Rationale**: no requirement or acceptance criterion asks for typed classes of plugin types; "readable generically" is the requirement.
- **Rejected**: building codegen now (scope growth without a criterion).

### Python functions reach the catalog as reflected Go funcs (discovery)
- **Decision**: coreservice builds a Go func per declared Python function with `reflect.FuncOf`/`reflect.MakeFunc` from its `FunctionSignature` and offers it through its registry's `Functions()`; the func body encodes args as JSON, enqueues a `FunctionTask` under the running operation's context and decodes the result. cty `number` maps to `float64`.
- **Rationale**: keeps Python functions application functions (NewConfig-time clash checks, same conversion and redaction as Go functions) using only the public registry path; spec says "the core builds the function from a signature the Python side declares".
- **Rejected**: a synthetic in-process plugin (wrong clash timing, no cancellation); importing internal function packages (design forbids).

### Untyped lookups added to Config (discovery)
- **Decision**: add public `(*Config).FindEntitiesByType(path ...string) ([]any, error)` and `(*Config).FindOneEntity(path ...string) (any, error)` sharing `entitiesOf`/`typeable`/`oneOf` rules without a Go type.
- **Rationale**: coreservice has no Go `T`; the design says needed internals are made public in the root module first; `Config` is the public query surface (knowledge `architecture/config-is-the-public-query-surface`).
- **Rejected**: re-implementing the type/subtype rules in coreservice (would drift from Go's errors).

### Schema v1 reader exposed through the registry package (discovery)
- **Decision**: `registry.TypeFromSchema(schema []byte) (registry.Type, error)` is the public entry to the v1 reader in `internal/schema`; `CreateInstanceFromSchema` sniffs `schema_version: 1` so plugins may send v1 too. `GenerateSchemaFromInstance` keeps emitting the legacy form under protocol 1.
- **Rationale**: design § How the core builds a Go type; plugins under protocol 1 must keep sending the legacy schema (design § Where schema v1 travels).
- **Rejected**: switching the producer to v1 now (older hosts could not read new plugins).

### Schema v1 describes named block maps and the optional name label (discovery)
- **Decision**: a block field whose `type` is `{"map": {"object": [...]}}` builds `map[string]struct` tagged `name,block`; a block field (repeated or map) may carry `"label": "<field>"`, which adds a string label field of that name to the element struct.
- **Rationale**: language-surface rule "libxcl schema v1 must describe them (and the optional name label on list-of-block fields)"; the design pre-dates named blocks and leaves the format open for it, so this is an extension, not a disagreement.
- **Rejected**: a new `kind` value (would duplicate `block` semantics).

### Engine version stamped at link time (discovery)
- **Decision**: `coreservice.Version` is a package var set with `-ldflags -X` by the libxcl build; dev builds stamp the version from the Python package being developed.
- **Rationale**: xcl has no version constant today; `GetVersion` must report the release tag.
- **Rejected**: `debug.ReadBuildInfo` (reports `(devel)` under the nested module's `replace`).

### Parity list scope and home (components)
- **Decision**: `PARITY.md` in xcl-python lists each Go operation (Validate, Apply, Diff, Destroy, Load, and their Context forms) and each lookup (Entities, EntityCount, FindResource/Find, FindByType, FindOne, All, Outputs, EncodeSavedEntity) with its sync and async equivalent. A person reviews it at each release. The struct-filling `Decode`, the deprecated `GetResources`/`ResourceCount`, `As` and `EncodeEntity` (a Go-value encoder) are listed as "not applicable" with the reason.
- **Rationale**: the success metric says to check against "a parity list at release". The listed exclusions are Go-type conveniences, not operations or lookups.
- **Rejected**: implementing a Python `decode` (no requirement asks for it); keeping the list in the README (it would crowd out getting started).

### Sync API runs tasks serially; no max_workers option (components)
- **Decision**: `xcl.Config` runs function, store and event tasks on the polling thread, one at a time. The design's optional `max_workers` is not built.
- **Rationale**: the design proposes `max_workers` for thread-safe *providers*, which this spec doesn't build. Functions are evaluated during the walk, and serial execution is correct.
- **Rejected**: a thread pool option (adds a concurrency surface with no requirement behind it).

### Python lookups take an optional class (data_structures)
- **Decision**: `find`, `find_by_type` and `find_one` take the class first, as the design writes them (`cfg.find(Postgres, "database.postgres.main")`). Passing `None` returns the registered class if there is one, or `xcl.Entity` otherwise. `all` requires a registered class.
- **Rationale**: this follows the design's API sketch and still lets unregistered plugin types be read generically.
- **Rejected**: separate untyped methods (they would double the surface).

### Cancellation exception (data_structures)
- **Decision**: a cancelled operation raises `xcl.CancelledError` (code `cancelled`). Under `xcl.aio` the awaiting task sees `asyncio.CancelledError` re-raised after the engine stops, because that is what asyncio requires of a cancelled task.
- **Rationale**: the spec says it "end[s] promptly with a cancellation error", and swallowing `asyncio.CancelledError` breaks asyncio semantics.
- **Rejected**: returning normally after a cancel.

### Musl archives named with an _musl suffix (tasks)
- **Decision**: libxcl release archives are named `libxcl_<version>_<os>_<arch>.tar.gz`, with `_musl` appended to the arch for musl builds (for example `libxcl_v0.4.0_linux_amd64_musl.tar.gz`). Checksums and the signature follow the plugin asset contract.
- **Rationale**: the plugin contract has no libc dimension, and the design asks for both glibc and musl builds.
- **Rejected**: a separate `linux-musl` os value (the registry code parses `os` against `GOOS` names).

### The xcl release after language-surface is v0.4.0 (tasks)
- **Decision**: the human release task expects `v0.4.0`, a minor release, because every change here is additive.
- **Rationale**: language-surface ships v0.3.0 as a breaking release. This spec adds only new API.
- **Rejected**: a patch release (it adds features).

### Python test conventions mirror the Go ones (tasks)
- **Decision**: pytest tests use one function per case, no `parametrize` tables, and positive and negative cases apart. Integration tests that need Go fixture plugins build them from the xcl checkout in a session fixture and skip when Go is absent.
- **Rationale**: the project's testing convention carried over by analogy.
- **Rejected**: table-driven pytest (it breaks the convention's intent).

### The Python CI builds libxcl from xcl source; the release uses the signed archives (tasks)
- **Decision**: CI checks out xcl at `xcl-ref.txt` and builds the engine with Go and a C compiler on the runner. The release workflow uses only the signed archives from the xcl release.
- **Rationale**: development needs unreleased engine changes, and releases must ship the one signed build per platform.
- **Rejected**: CI downloading archives (no archives exist for unreleased commits).

### xcl-python registered before the repository exists (tasks)
- **Decision**: following the orchestrator's answer (the user's option A), xcl-python was registered with `spektacular repo add` at `/home/nicj/code/github.com/jumppad-labs/xcl-python` during planning. The human task creates the GitHub repository and connects that directory to it.
- **Rationale**: `plan file write` needs every task's repo to be registered.
- **Rejected**: attributing the Python tasks to xcl.

## Rehydration cues

- `spektacular spec file read 20261011130709-aa3eb933-python-support`
- `spektacular design read --data '{"source":"design","path":"libxcl.md"}'`
- `spektacular plan file read 20261011095738-fad05e41-language-surface plan` and `... context`
- `spektacular knowledge always-applied --tier repo --filter xcl`
- Re-read `config.go` (`run`, operations), `query.go` (`entitiesOf`, `typeable`), `options.go`, `registry/registry.go`, `registry/local.go`, `internal/schema/deserialize.go`, `.github/workflows/go.yml`.
