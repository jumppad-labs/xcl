---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Plan: 20261008071608-eb05cae0-config-and-plugin-registries

<!-- Metadata -->
<!-- Created: 2026-10-08T07:54:00Z -->
<!-- Commit: 465566f -->
<!-- Branch: f-diff -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

This plan makes xcl's setup short and hard to get wrong. Programs declare their own Go types as options when they create a configuration. Running without saved state becomes the documented configuration-only mode. Plugins come only from registries the application adds explicitly, starting with a local registry, and the contracts leave room for a remote registry later. Registration never returns an error: code mistakes panic straight away, and environment problems are reported when plugins load. That includes any plugin that fails to start, discovered ones too, which is a decision the user made during planning. Go developers embedding xcl, with or without plugins, get a few-line setup, and event handlers read typed entities without holding a catalog.

## Conventions

- **Shared error types live in the `errors` package, re-exported from `xcl`, pointer receivers, always wrapping** — `PluginLoadError` gains `Registry`, and `TypeNameClashError`/`TypeFormError` move out of the soon-internal registry so `errors.As` keeps working for users.
- **Public library packages live at module top level; private code under `/internal`** — decides `github.com/jumppad-labs/xcl/registry` (public) and `internal/catalog` (private).
- **Testing & mocking: testify `require`, Mockery, no table-driven tests, positive and negative cases in separate functions, tests live beside their code, every example keeps its `smoke_test.go`, no tests that read docs/CHANGELOG/source** — the ~23 registration error tests become one panic test each; discovery tests move with the code into `registry/`; docs are checked by review, not tests.
- **Shared test helpers live in `internal/testutil`, imported only from `_test.go`, never importing root `xcl`** — any helper shared by root/e2e tests for building local registries goes there, not copied per package.
- **Example modules cannot import `internal/`** (gotcha `example-modules-cannot-import-internal`) — examples and `prettylog` must use only `xcl.WithType`, `xcl.WithRegistry` and `registry.NewLocal`.
- **Assert ordering on graph parents, not provider call order** — not applicable to dependency order here, but load-order tests assert on the emitted load event sequence, which is sequential by construction (registries and plugins are started one at a time).
- **Go code style (gofmt, `any`, descriptive names) and structured logging via events** — discovery keeps reporting through `logger.New(emit, …)` events; library code writes no output.
- **Never modify dependency packages** — the rename/migration is scoped to this repo's files only.
- Not applied: database/external-services, test-state-from-real-apply (no new state fixtures).

## Architecture & Design Decisions

The work is built to the design `design/plugin-registries.md`. Today's `plugins/registry.PluginRegistry` has two jobs: it resolves block types and it loads plugins. It is split along that line. The type catalog and loader move, behaviour intact, to a new internal package `internal/catalog` (xclconfig: `plugins/registry/plugin_registry.go` → `internal/catalog/catalog.go`, plus `errors.go` and the tests). A `Catalog` is now private to one `Config`: `NewConfig` builds it after every option has been collected. Where plugins come from moves into a new public package `github.com/jumppad-labs/xcl/registry` (xclconfig: `registry/registry.go`, `registry/local.go`, `registry/discovery.go`). That package holds the design's `Registry` interface (`Name`, `Plugins(ctx, emit)`), the `Plugin` interface (`Name`, `Start(emit)`), `InProcess(p)` wrapping `plugins.NewDirectPluginHost` (`plugins/direct_plugin_host.go:29`), `Executable(path)` wrapping `plugins.NewGRPCPluginHost` (`plugins/grpc_plugin_host.go:38`), and `NewLocal(PluginPattern(...))` with `RegisterPlugin`, `RegisterExternalPlugin` and `RegisterPluginDirectory`. The directory search code (`plugins/registry/plugin_discovery.go`) moves into the local registry. The local registry runs that search inside `Plugins`, once per load, and emits the existing `discover` events. It returns one ordered list in which every directory's matches sit at the point where that directory was registered. The public package is placed at module top level per the project-structure convention, and given a name that no longer collides with the old one, as the design's "Open" section asks. `plugins/registry` is deleted, and nothing for the remote registry ships.

Registration only records. `WithType(prototype, name...)` checks the shape of its own arguments straight away: an empty name, more than one subtype, an empty subtype, or a value that is not a pointer to a struct embedding `types.ResourceBase`. Any of these panics, naming the type, so the stack trace points at the call. `NewConfig` then registers every declared type on the catalog and panics on a duplicate, a type keyword used in both forms, or a builtin name. Those checks need every option, and options may be given in any order. `registry.Local.RegisterPlugin(nil)` and `xcl.WithRegistry(nil)` panic. Everything that depends on the environment goes to the catalog's existing once-per-Config load (`plugin_registry.go:587`, called through `Use` from `Config.run`). The catalog calls each registry's `Plugins` in the order `WithRegistry` was given, then starts each plugin in order. **Any plugin that fails to start fails the load, including one found by directory discovery.** This is the user's decision during planning ("Any failure is an error"). It replaces the spec's "Discovered plugins that fail are skipped" and the design's "skipped … as today": the old rejected-and-skipped path (`plugin_registry.go:687`) and its `rejected:true` event are removed. A failure is a `*PluginLoadError`, which gains a `Registry` field, and the load error event names the plugin and its registry. A block type provided twice is checked at load whatever its source: within one registry, across registries, or against a `WithType`. It fails with a `*TypeNameClashError`, moved to the shared `errors` package and re-exported from `xcl`, that names both providers and both registries. Because every `WithType` is fixed before the first load, the clash is reported identically whatever order the code was written in. `WithPluginRegistry`, `CastResourceTo` and the package-level `EncodeSavedEntity` are removed. `(*Config).EncodeSavedEntity(data, ...)` replaces the last of these, loading plugins through the same `Use` path.

Events read their entity without a catalog. `events.Event` gains an unexported decoder field. `encoding/json` skips unexported fields, so JSON output is unchanged. The event also gains `Entity() (any, error)`, which decodes `Data` on every call, and a method that sets the decoder. `Config.run` (`config.go:498`) is the single place every event passes through, and it attaches `c.decodeEventEntity` there. That function is `savedentity.Decode(c.catalog, data, ReadOptions{ForDisplay: true})`, so sensitive values show as the mask marker and a buffered handler always gets a fresh copy. `events` still imports nothing from the root package (spec constraint), and `example/prettylog.Handler(w, level)` encodes `e.Entity()` with `xcl.EncodeEntity`.

The rest is mechanical migration across both repos. In xclconfig, about 115 test call sites move to the internal catalog where a test needs only a catalog (`internal/parser`, `internal/savedentity`, `state`). Tests that build a `Config` (root package, `e2e`, `plugins/example`) and the three example modules, which cannot import `internal/`, move to `WithType`/`WithRegistry`. The tests that asserted `RegisterType` errors become panic tests. `README.md` and `docs/*.md` are rewritten to the new API, and the tracked root `configonly` binary is removed and ignored. In xcl-website, the pages `index.mdx`, `events.mdx`, `plugin-logging.mdx`, `configuration-text.mdx`, `examples/configuration-only.mdx` and `examples/plugins.mdx` are updated, a new `registries.mdx` guide is added with a configuration-only-without-state section, and `src/components/Nav.astro` gets the new entry. This direction is preferred to keeping a public catalog beside the new options, to putting the new contracts in `plugins/registry`, and to having registries start their own hosts. See `research.md#alternatives-considered-and-rejected` for the evidence.

## Component Breakdown

- **Public `registry` package (new, xclconfig).** It defines the contracts every source of plugins is written against. `Registry` names itself and returns its plugins once per load. `Plugin` names itself and starts its own host. Two plugin starters ship with it: `InProcess`, which wraps the existing direct host, and `Executable`, which wraps the existing process-over-gRPC host. Third-party registries and plugin starters are written against the same contracts. The package depends only on the existing plugin hosts and events, never on the root package or the catalog.
- **Local registry (new, in the public `registry` package, xclconfig).** It holds an ordered list of what the application registered: in-process plugins, plugin binary paths and plugin directories. The search pattern is fixed when the registry is created. When `Plugins` is called it searches each directory (expanding `~` and environment variables), reports the search as discover events, and returns everything as one list in registration order. A nil plugin panics. Paths that do not exist are not checked here, because that is the loader's job. It reuses the existing discovery code, moved here from the old registry package.
- **Type catalog and plugin loader (moved and changed, xclconfig `internal/catalog`).** This is the former `PluginRegistry`, now private and owned by exactly one `Config`. It keeps its type-resolution role: builtins, declared Go types, plugin types, entity creation, address paths and provider lookup for the parser, saved-entity decoding and queries. It also keeps its lifecycle role: one cached load, starting and stopping external processes around each operation, and routing plugin log messages. Its loader changes in three ways. It now goes through the registries it was given, in order. Any plugin failing to start fails the load. It records which plugin and which registry provided each type, so that load errors and clash errors can name both. Registering a type now panics on programmer errors instead of returning them.
- **Config and its options (changed, xclconfig root package).** `WithType` records a declared Go type after checking its arguments' shape. `WithRegistry` records a registry. `NewConfig` gathers both, builds the Config's own catalog, registers the declared types (panicking on duplicates or form clashes), and adds the registries in the order they were given. `WithPluginRegistry` is gone. Not choosing a place for state remains the no-persistence mode, now documented as the supported configuration-only setup. `Config.run` keeps its job as the one wrapper around every operation, and now also attaches the entity decoder to each event it delivers. `EncodeSavedEntity` becomes a Config method that decodes through the Config's own catalog.
- **Event (changed, xclconfig `events` package).** It gains a private, non-serialised decoder, a way for an emitter to attach one, and `Entity()`, which decodes the event's data into the registered Go type with sensitive values masked. It still imports nothing from the root package.
- **Shared errors (changed, xclconfig `errors` package, re-exported from root).** `PluginLoadError` gains the registry name. The type-clash error moves here from the old registry package, re-exported from root, and now names both providers and their registries. The type-form error also moves here.
- **Parser and saved-entity decoding (changed in signature only, xclconfig internal).** They take the internal catalog wherever they took the public registry. Their behaviour is unchanged.
- **Pretty log handler (changed, xclconfig `example/prettylog` module).** It is built from only an output and a level. It writes each created entity's configuration by encoding `Event.Entity()`.
- **Examples (changed, xclconfig `example/configonly`, `example/plugin`).** The configuration-only example declares its types with `WithType`, keeps no state, uses no registry and checks no registration errors. The plugin example builds one local registry, adds it with `WithRegistry`, and passes no catalog anywhere.
- **Test suites (changed, xclconfig).** Root, e2e and plugin-example tests build Configs with the new options. Internal parser, saved-entity and state tests use the internal catalog directly. Registration-error tests become panic tests, and the clash and failed-plugin tests become load-time tests.
- **Documentation (changed, xcl-website and xclconfig markdown).** Every page that shows registration, catalogs or the pretty log handler is rewritten to the new API. A new Registries guide covers local registries, load order, failure reporting and clashes. A configuration-only section explains running without state. The in-repo `README.md` and `docs/` guides are rewritten the same way.
- **Repository hygiene (changed, xclconfig).** The tracked configuration-only binary is removed, and the ignore rules cover example build outputs.

## Data Structures & Interfaces

**Public registry contracts** (new package `github.com/jumppad-labs/xcl/registry`, exactly as the design fixes them). These are what any registry, including a future remote one or a third party's, is written against.

```go
type Registry interface {
    Name() string                                                  // "local", or i.e. "registry.xcl.dev"
    Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error) // called once per Config, at load
}

type Plugin interface {
    Name() string                                       // used in events and errors
    Start(emit events.Emit) (plugins.PluginHost, error) // runs the plugin, returns its host
}

func InProcess(p plugins.Plugin) Plugin // direct host; panics on nil
func Executable(path string) Plugin     // gRPC process host; a missing path fails at Start
```

**Local registry** (new). An ordered recording of registrations. It returns no errors from registration.

```go
func NewLocal(options ...LocalOption) *Local // Name() == "local"
func PluginPattern(pattern string) LocalOption // default "xcl-plugin-*"

func (l *Local) RegisterPlugin(p plugins.Plugin)      // panics on nil
func (l *Local) RegisterExternalPlugin(path string)
func (l *Local) RegisterPluginDirectory(dir string)
func (l *Local) Name() string
func (l *Local) Plugins(ctx context.Context, emit events.Emit) ([]Plugin, error)
```

**Config options** (root package). `ConfigOption` keeps its shape.

```go
func WithType(prototype any, name ...string) ConfigOption // panics on a malformed declaration
func WithRegistry(r registry.Registry) ConfigOption        // panics on nil; may repeat
// removed: WithPluginRegistry
func (c *Config) EncodeSavedEntity(data []byte, options ...EncodeOption) ([]byte, error)
// removed: package-level EncodeSavedEntity(reg, data, ...)
```

**Event** (events package). It gains a non-serialised decoder and an accessor. Its JSON form is unchanged, because the decoder is an unexported field.

```go
type EntityDecoder func(data []byte) (any, error)

func (e Event) WithEntityDecoder(decode EntityDecoder) Event // set by Config.run
func (e Event) Entity() (any, error) // nil, nil when Data is empty
```

**Errors** (`errors` package, each re-exported from root as an alias).
- `PluginLoadError{Plugin, Registry string; Err error}`. It still unwraps to `ErrPluginLoad` and to the cause. `Plugin` is empty when the registry itself failed, for example when a directory could not be read.
- `TypeNameClashError{Name, Provider, Registry, Existing, ExistingRegistry string}`. It is returned from load for any block type provided twice. It names both providers, using the plugin name, `"builtin"` or `"type <GoType>"`, and their registries, which are empty for a builtin or a declared type. It moves here from the old registry package.
- `TypeFormError{Type string; TakesSubtype bool}`. It moves here unchanged and is returned from load when a plugin's type uses a keyword in the other form.

**Internal catalog** (`internal/catalog`, not public). It has the same query surface the parser, saved-entity decoding and queries use today: type lookup, subtype form, entity creation, provider lookup, registered-type check, address paths, type list, activation, and the `Use` and `Load` cycle. Three things change. Construction is `New()`. `RegisterType(prototype, name...)` now panics instead of returning an error. `AddRegistry(registry.Registry)` replaces the three plugin-recording methods. Each loaded host is held with its plugin and registry names.

**Removed public surface.** `plugins/registry` is gone entirely: `PluginRegistry`, `NewPluginRegistry`, `RegisterPluginWithPath`, `DiscoverPlugins`, `PluginDiscovery`, `ExpandPluginDirectories` and `CastResourceTo`. In `example/prettylog`, `Handler(w, level, reg)` becomes `Handler(w, level)`.

**Serialisation boundaries.** Event JSON is unchanged. State and the plugin wire protocol are unchanged.

## Implementation Detail

**Module boundary: one package becomes two.** The old registry package splits along the line between *what types exist* and *where plugins come from*. The catalog moves to `internal/` without a rewrite: type resolution, the `Use`/`Load` lifecycle, plugin log routing and external-process restarts keep their code and comments. The public `registry` package is new and small: two interfaces, two plugin starters and the local registry. The local registry reuses the discovery code. A developer reading `config.go` sees `Config` holding its own catalog, and no longer a borrowed public object.

**Loader shape.** Today the catalog keeps three separate lists: in-process plugins, binary paths and discovery directories. The new loader holds one list of registries and runs a single, simpler loop:

```
for each registry (in WithRegistry order):
    plugins, err := registry.Plugins(ctx, emit)         // local: discover events here
    if err: fail load with PluginLoadError{Registry}
    for each plugin (in registration order):
        emit load start {plugin, registry}
        host, err := plugin.Start(catalog.pluginEmit)
        if err: emit load error; fail load with PluginLoadError{Plugin, Registry}
        if clash := checkHost(host, plugin, registry): stop host; emit load error; fail load
        record host with its plugin + registry names; emit load success {plugin, registry, block_types}
```

Because every plugin is now treated the same way (user decision), the separate discovered-plugin pass, its skip-and-reject branch and the "all plugin loads failed" rule are deleted. The catalog no longer needs to know how a plugin was found. Load events gain a `registry` meta key next to `plugin`. A load that fails partway still stops whatever it had started, through the existing `Use` failure path.

**Panic-at-registration pattern (new to xcl).** Validation that used to return errors from `RegisterType` now panics, following `gob.Register`. It runs in two places. `WithType` checks its own arguments so that the panic's stack points at the user's line. `NewConfig` panics on checks that need every declaration: duplicates, type keyword form, and builtin names. Every panic message starts with `xcl:` and names the type key or plugin. The checks themselves (`checkType`) stay in the catalog, with one change: a clash against a plugin no longer happens at registration, because no plugin can have loaded when `NewConfig` runs.

**Event decoding.** `Config.run` already wraps every operation's work in one place. Its emitter now attaches the Config's decoder to every event before buffering it. This follows the existing pattern of doing cross-cutting work in `run`, so no emitter in the parser, the hosts or the logger changes. Without an event handler nothing is attached, because no event is delivered.

**Call-site migration pattern.** Tests and examples move along three paths:
- Tests and examples that build a `Config` use `xcl.WithType(...)` and `xcl.WithRegistry(local)`. The common shape, a local registry holding the test plugin, moves into the shared test-helper package where more than one package needs it.
- Tests of internals (parser, saved entities, state) build `catalog.New()` and register on it directly.
- Tests that reached into a registry's hosts either drop that access, because cleanup is already handled when each operation finishes, or assert through events and behaviour instead.

None of these paths adds a compatibility layer: the old names are deleted and the compiler finds every caller.

**Documentation.** The in-repo `README.md` and `docs/` guides and the website pages are rewritten in the same terms. "Registration" now means `WithType` or a registry's `Register*` calls. "Problems are reported" means a panic for code mistakes and a `PluginLoadError` or type-clash error from the first `Validate`/`Apply`/`Destroy`/`Diff`/`Load` for everything else. Website snippets copied from the examples are refreshed from the example sources so they match exactly.

## Dependencies

- **Design `plugin-registries.md` from the `design` source** is the settled API this plan implements: option names, the `Registry` and `Plugin` contracts, the local registry calls, the failure rules and what becomes internal. The plan departs from it in one place, by the user's decision during planning: a discovered plugin that fails to start now fails the load instead of being skipped. The design document and the spec should be updated to match, and that is offered at review.
- **Existing plugin hosts (`plugins` package: direct host, gRPC host, `PluginName`, `PluginBinaryName`)** are what `InProcess` and `Executable` wrap. They are unchanged.
- **`internal/savedentity`** turns event data into an entity for `Event.Entity()` and for `(*Config).EncodeSavedEntity`. Only its parameter type changes, from the public registry to the internal catalog.
- **`internal/parser`** consumes the catalog for type lookup, entity creation and provider lookup. Its option field changes type and its behaviour is unchanged.
- **`errors` package** holds `PluginLoadError`, which gains a registry field, and receives the type-clash and type-form errors.
- **`events` package** gains the entity accessor. It must keep not importing the root package.
- **`internal/testutil`** receives any helper that more than one package needs for building local registries in tests.
- **Example modules (`example/configonly`, `example/plugin`, `example/prettylog`)** are separate Go modules with `replace` to the repo root, so they pick up the new API without version bumps. `example/configonly` is currently committed in a broken, half-rewritten state (`465566f`), and the user will repair it by hand. Its rewrite in this plan assumes the `ingressRoutes` logic exists by then.
- **xcl-website (Astro 5 + MDX)**: the documentation pages, the nav component, and its build and type-check verification. No new packages are needed.
- **No new third-party Go dependencies.**
- **Prior plans** whose behaviour this keeps: event-based-logging (lazy once-only load, load/discover events, `PluginLoadError`, plugin log routing), config-only-types-and-examples (type resolution order, registered types skip providers), e2e-suite-and-real-world-examples (examples as modules, smoke tests). Nothing has to land first.

## Testing Approach

**Kinds of tests.** The plan uses three levels, all following the repo's rules: testify `require`, no table-driven tests, positive and negative cases in separate functions, and tests beside their code.
- **Unit tests** cover the public `registry` package (local registry ordering, pattern and directory discovery, `InProcess`/`Executable` start), the internal catalog (registration panics, loader order, clash and failure errors), and `Event.Entity()`.
- **Integration tests through `Config`** in the root package exercise the spec's acceptance criteria end to end with the existing test plugins and the in-repo test plugin binaries.
- **Example tests and the e2e suite** confirm that the real programs work on the new API. That covers each example's own tests and smoke test, and the e2e tests that run them.

**Most coverage goes to the loader and the registration rules,** because they carry the behaviour change. The tests guarantee the following:
- Each registration mistake panics with a message naming the type or plugin. The mistakes covered are an empty name, more than one subtype, an empty subtype, a type that is not a valid entity, a duplicate, the same keyword in both forms, and a nil in-process plugin. There is one test per case.
- Registering a missing binary does not fail. The first operation then fails with a `PluginLoadError` naming the path and the registry.
- Two registries each providing a different plugin are both usable.
- Load events appear in registry order, then in plugin registration order within each registry. That holds because loading is sequential, so the assertion is on the event sequence.
- A custom pattern discovers only matching executables.
- **Any failing plugin fails the load,** whether explicitly registered or discovered, with an error naming the plugin and its registry. This is the user's planning decision, and it replaces the spec's "a failing discovered plugin is skipped" acceptance criterion with its opposite.
- A duplicate block type fails the load and names both providers and registries. The cases covered are within one registry, across two registries, and against a declared type, with the declared type written both before and after the registry.
- A registry and plugin starter written outside xcl, as a test type in an external test package, has its plugin started and its types applied.
- With processed event data, `Entity()` returns the user's Go type with configured values and sensitive values masked. An event without data returns nil with no error.
- Marshalling an event to JSON succeeds and contains no decoder field.
- Applying with no state location writes no state anywhere. The test checks the working directory and a temp HOME stay empty, and that entities are readable afterwards.
- Encoding saved data through the Config equals encoding the entity directly.
- The pretty log handler, built from only a writer and a level, writes configuration beneath the success line.

**Compile-time guarantees replace some tests.** Under the repo rule, no test greps the source. The guarantee that registration has no error result is a compile-time property: the migrated tests and examples call each registration as a bare statement, and the build would fail if an error result were left unhandled. The same holds for the removal of the public catalog and `WithPluginRegistry`: deleting `plugins/registry` and the option means any use fails to compile.

**Migration of existing tests.** About 23 tests that asserted `RegisterType` errors become panic tests, and the existing missing-plugin, clash and discovery tests move to the new loader and registry. The rest of the suite, including e2e, parser, saved-entity, state and the plugin example, is migrated mechanically and must stay green. That is the regression net for unchanged behaviour.

**Deliberate gaps.**
- No test reads documentation, the README, the changelog or source text, per the repo's testing convention.
- No remote registry tests, because nothing for the remote registry is built.

**Success metrics.**
- *The config-only example's setup has no error checks other than creating and applying the config, and no state, key or catalog code.* **Manual — captured in the implementation test plan.** This is code review of the example source; behaviour is covered by the example's tests and its smoke test printing the expected route.
- *Every registration failure case that has a test today still has one, as a panic test or a load-time error test.* **Manual — captured in the implementation test plan.** A reviewer maps the old registration-error tests to their new panic or load-time tests.
- *The full test suite, the example tests and the documentation site build all pass.* This is a behavioural test, in the sense that the root and e2e suites, each example module's tests and the website build plus type-check are run as the verification gate.

**Manual reviews.**
- **Manual — captured in the implementation test plan:** review every website page and in-repo doc that shows registration, catalogs or the pretty log handler, to confirm it shows only the new API. Also check that the new Registries guide and the configuration-only-without-state section read correctly and are linked from the nav.
- **Manual — captured in the implementation test plan:** confirm that website snippets copied from example sources match those sources verbatim.
- **Manual — captured in the implementation test plan:** confirm the root `configonly` binary is untracked, and that building an example leaves no new untracked files.

## Milestones & Tasks

### Milestone 1: Event handlers read entities without a catalog

**What changes**: An event handler can ask any event that carries resource data for the entity it holds, as the program's own Go type with sensitive values masked, without holding anything from xcl. The example pretty log handler is created from only an output and a level, and it still writes each created entity's configuration beneath its event. A program that reads saved data turns it back into configuration text through its `Config`, so it no longer needs a separate registry. Event JSON output is unchanged. This milestone lands on the existing registration API, so everything else keeps working while event consumers lose their dependency on the catalog.

**Validation point**: The new entity-reading tests pass: processed data gives the registered Go type with masked secrets, an event without data gives nothing, and JSON has no extra field. The pretty log handler's tests pass with the two-argument constructor. Encoding saved data through the Config gives the same text as encoding the entity directly. The full root suite, the e2e suite and every example's tests are green.

#### - [x] Task: Event entity accessor
**Id:** 6a92ace9-acc0-4e5f-99d2-de05b7b3c5ff
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Give every event a way to return the entity its data holds, decoded on demand into the program's registered Go type with sensitive values shown as the mask marker. The configuration attaches the decoder in the one wrapper every operation's events already pass through. Event JSON output must not change, and the events package must keep not depending on the root package.

*Technical detail:* [context.md#task-event-entity-accessor](./context.md#task-event-entity-accessor)

**Acceptance criteria**:
- [x] With processed event data on, the success event for a created resource returns, when asked for its entity, a value of the program's registered Go type holding the configured values, with each sensitive value shown as the mask marker
- [x] An event with no data returns nothing and no error
- [x] Marshalling such an event to JSON succeeds, and the output contains no field for the decoder
- [x] The events package still imports nothing from the root package

#### - [x] Task: Encode saved data through the Config
**Id:** b4faf219-3921-4fe2-895a-330d09f3a868
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Replace the package-level function that encodes saved entity data and takes a registry with a method on the configuration. The method resolves the record's type through the configuration's own types and plugins, and loads plugins first when needed. Every existing caller moves to the method.

*Technical detail:* [context.md#task-encode-saved-data-through-the-config](./context.md#task-encode-saved-data-through-the-config)

**Acceptance criteria**:
- [x] Encoding a saved entity record through the configuration returns the same configuration text as encoding the entity directly
- [x] A saved record of a plugin-provided type encodes through a configuration whose plugins have not yet loaded
- [x] The package-level function that took a registry no longer exists, and all existing encoding tests pass using the method

#### - [x] Task: Pretty log handler without a catalog
**Id:** 177a89e6-e983-45d2-ba13-44a85383a62f
**Repo:** xclconfig
**Depends on:**
- 6a92ace9-acc0-4e5f-99d2-de05b7b3c5ff — Event entity accessor
**Execution:** agent

Change the example pretty log handler so it is created from only an output and a log level. It writes each created entity's configuration by encoding the entity the event itself returns. Update the plugin example's use of the handler.

*Technical detail:* [context.md#task-pretty-log-handler-without-a-catalog](./context.md#task-pretty-log-handler-without-a-catalog)

**Acceptance criteria**:
- [x] The handler, created with only an output and a level, writes a created resource's configuration text beneath its success line
- [x] Events without data still produce their log line with no configuration block
- [x] The pretty log and plugin example tests pass

### Milestone 2: Declare types at creation and get plugins from registries

**What changes**: Programs declare their own Go types as options when they create a configuration, and get plugins only from registries they add explicitly, starting with a local registry. A local registry holds in-process plugins, plugin binaries and plugin directories searched with a pattern fixed at creation. No registration call returns an error. Code mistakes panic straight away and name the culprit. A missing or failing plugin, whether registered directly or found in a directory, is reported from the first operation that loads plugins, naming the plugin and its registry. A block type provided twice anywhere is always a load error naming both providers. Registries load in the order given, and their plugins load in registration order. Anyone can write their own registry or plugin starter against the public contracts. The old public catalog and the option that took one are gone. Both examples and every test are rewritten to the new API: the configuration-only example declares its types and keeps no state, and the plugin example uses one local registry.

**Validation point**: Each acceptance criterion has a passing test: the panic per mistake, the load-time missing binary, two registries, the custom pattern, load-order events, failing plugins, duplicates within and across registries and against declared types, a custom registry, and applying without state writing nothing. The root suite, the e2e suite and every example module's tests and smoke tests are green, and the old registry package no longer exists.

#### - [x] Task: Public registry package and local registry
**Id:** 8645bb13-3d3a-4fda-a0e1-04b2c089f352
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Add the public registry package, with the registry and plugin contracts exactly as the design fixes them, plus two plugin starters: one for a plugin compiled into the program and one for a plugin binary. Add the local registry. It records in-process plugins, binary paths and plugin directories in order, with a search pattern fixed at creation, and returns them all as one ordered list when asked. The plugin directory search code moves here from the old registry package, with its tests.

*Technical detail:* [context.md#task-public-registry-package-and-local-registry](./context.md#task-public-registry-package-and-local-registry)

**Acceptance criteria**:
- [x] A local registry returns its plugins in the order they were registered, with each directory's matches at the position the directory was registered
- [x] A local registry created with a custom search pattern finds the executables in a directory that match it, and none that do not
- [x] Registering a nil in-process plugin panics with a message naming the local registry
- [x] Registering a plugin binary path that does not exist does not fail
- [x] Every registration call is a statement with no result

#### - [x] Task: Plugin load and type clash errors name registries
**Id:** 90e9cd1e-425f-47cb-a01c-39a9c648311a
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Extend the plugin load error so it also names the registry a plugin came from. Move the type-name clash and type-form errors into the shared errors package and re-export them from the root package. Extend the clash error so it names both providers and their registries.

*Technical detail:* [context.md#task-plugin-load-and-type-clash-errors-name-registries](./context.md#task-plugin-load-and-type-clash-errors-name-registries)

**Acceptance criteria**:
- [x] A plugin load error's message names the plugin and its registry, and the error still matches the plugin load sentinel and its cause
- [x] A clash error's message names the block type, both providers and both registries
- [x] Both errors can be recovered from the root package with the standard error-matching functions

#### - [x] Task: Internal type catalog loading from registries
**Id:** 649da1e7-48df-4719-a5ef-0a01f849fe6b
**Repo:** xclconfig
**Depends on:**
- 8645bb13-3d3a-4fda-a0e1-04b2c089f352 — Public registry package and local registry
- 90e9cd1e-425f-47cb-a01c-39a9c648311a — Plugin load and type clash errors name registries
**Execution:** agent

Move the type catalog and plugin loader into an internal package and delete the old public registry package. The loader takes registries and loads them in order, starting each plugin in turn. Any plugin that fails to start fails the load, including one found in a directory; this is the user's planning decision. Every block type provided twice fails the load with an error naming both providers and their registries. Registering a malformed, duplicate or wrongly-formed type now panics. The parser, saved-entity decoding and their tests move to the internal catalog.

*Technical detail:* [context.md#task-internal-type-catalog-loading-from-registries](./context.md#task-internal-type-catalog-loading-from-registries)

**Acceptance criteria**:
- [x] Every registration mistake that used to return an error panics with a message naming the type, with one panic test per case
- [x] Plugins load registry by registry in the order the registries were added, and within a registry in registration order, as seen in the load events
- [x] A plugin that fails to start, whether registered explicitly or found in a directory, fails the load with an error naming the plugin and its registry
- [x] The same block type provided twice, within one registry or across two, fails the load with an error naming both providers and registries
- [x] The old public registry package no longer exists, and the parser, saved-entity and state tests pass against the internal catalog

#### - [x] Task: Config options declare types and add registries
**Id:** 29c14d2c-ba97-41f1-8aa7-91fac3893526
**Repo:** xclconfig
**Depends on:**
- 649da1e7-48df-4719-a5ef-0a01f849fe6b — Internal type catalog loading from registries
- b4faf219-3921-4fe2-895a-330d09f3a868 — Encode saved data through the Config
**Execution:** agent

Add the options that declare a Go type and add a registry, and remove the option that took a catalog. Creating a configuration builds its own catalog from the declared types and registries, in any order the options were given. Migrate every root-package test to the new options. Add the end-to-end acceptance tests through the configuration: applying without state, declared types decoding, the registration panics, failing and missing plugins, multiple registries, load order, every kind of duplicate, and a custom registry with its own plugin starter.

*Technical detail:* [context.md#task-config-options-declare-types-and-add-registries](./context.md#task-config-options-declare-types-and-add-registries)

**Acceptance criteria**:
- [x] A configuration created with declared types and no state location applies a configuration directory, its entities are readable afterwards and decode into the program's own Go values, and no state file or directory is created
- [x] Registering a missing plugin binary does not fail, and the first apply returns a plugin load error naming the path and the registry
- [x] A configuration given two local registries, each providing a different plugin, applies configuration using block types from both
- [x] A plugin that provides a block type also declared as a Go type fails the first apply with an error naming the plugin and the type, whether the type was declared before or after the registry was added
- [x] A registry and plugin starter written outside xcl against the public contracts has its plugin started and its block types applied
- [x] The option that took a catalog no longer exists, and the full root test suite passes

#### - [x] Task: Migrate e2e and plugin SDK example tests
**Id:** b69161ae-dc5f-4981-a15e-ed3d27d8c0fe
**Repo:** xclconfig
**Depends on:**
- 29c14d2c-ba97-41f1-8aa7-91fac3893526 — Config options declare types and add registries
**Execution:** agent

Move the end-to-end suite and the plugin SDK example tests to declared types and local registries. Drop the test cleanup that reached into the registry's plugin hosts, since every operation already stops its external processes when it finishes.

*Technical detail:* [context.md#task-migrate-e2e-and-plugin-sdk-example-tests](./context.md#task-migrate-e2e-and-plugin-sdk-example-tests)

**Acceptance criteria**:
- [x] The e2e suite and the plugin SDK example tests pass on the new API
- [x] The missing external plugin end-to-end test still fails the apply with a plugin load error, which now names the local registry

#### - [x] Task: Rewrite the configuration-only and plugin examples
**Id:** d05fac3c-fd76-4d05-a077-06f5e078b00a
**Repo:** xclconfig
**Depends on:**
- 29c14d2c-ba97-41f1-8aa7-91fac3893526 — Config options declare types and add registries
- 177a89e6-e983-45d2-ba13-44a85383a62f — Pretty log handler without a catalog
**Execution:** agent

Rewrite the configuration-only example so it declares its types when creating the configuration and keeps no state. It uses no registry and has no registration error checks. Rewrite the plugin example so it builds one local registry and adds it to the configuration. Neither passes a catalog to the log handler. Update the pretty log tests to the new API.

*Technical detail:* [context.md#task-rewrite-the-configuration-only-and-plugin-examples](./context.md#task-rewrite-the-configuration-only-and-plugin-examples)

**Acceptance criteria**:
- [x] The configuration-only example has no catalog construction, state option, state key, mask or registration error check, and running it prints the expected ingress route
- [x] The plugin example builds one local registry, adds it to the configuration, and has no registration error checks
- [x] Every example module's tests and smoke tests pass

### Milestone 3: Documentation and repository show only the new API

**What changes**: Everyone reading the documentation site or the repository's own guides sees only the new way of working. Types are declared at creation, plugins come from registries, configuration-only programs need no state, and registration problems are reported either as an immediate panic or as a load error. The site gains a Registries guide and a configuration-only-without-state section, both reachable from the navigation. The repository stops tracking the stray configuration-only binary, and example build outputs are ignored, so building an example leaves the working tree clean.

**Validation point**: The website builds and type-checks with no errors. A search of both repositories finds no remaining mention of the removed calls, the catalog option or the old log handler signature, outside historical changelog entries and spec/plan records. Copied snippets match the example sources. Building each example leaves the working tree clean.

#### - [x] Task: Untrack the stray binary and ignore example builds
**Id:** ed3a4a2d-e6b3-466e-81d5-cab13997c88b
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Stop tracking the configuration-only binary committed at the repository root. Add ignore rules so that building any example, including a plain build inside an example directory, leaves no untracked file.

*Technical detail:* [context.md#task-untrack-the-stray-binary-and-ignore-example-builds](./context.md#task-untrack-the-stray-binary-and-ignore-example-builds)

**Acceptance criteria**:
- [x] The repository no longer tracks the configuration-only binary
- [x] Building each example in its own directory leaves no new untracked file

#### - [x] Task: Update in-repo guides to the new API
**Id:** fc3de07d-c1b9-49ba-bfa4-cab73fc3fc57
**Repo:** xclconfig
**Depends on:**
- d05fac3c-fd76-4d05-a077-06f5e078b00a — Rewrite the configuration-only and plugin examples
- b69161ae-dc5f-4981-a15e-ed3d27d8c0fe — Migrate e2e and plugin SDK example tests
**Execution:** agent

Rewrite the README, the docs guides and the e2e coverage notes so they show declared types, local registries and the configuration-only mode without state. They also explain that registration mistakes panic and that plugin problems are reported when plugins load. Historical changelog entries are left alone.

*Technical detail:* [context.md#task-update-in-repo-guides-to-the-new-api](./context.md#task-update-in-repo-guides-to-the-new-api)

**Acceptance criteria**:
- [x] No in-repo guide outside historical changelog entries mentions the removed registration calls, the catalog option or the old log handler signature
- [x] The README shows the short configuration-only setup and a plugin setup with a local registry

#### - [x] Task: Update the documentation site to the new API
**Id:** 2a57a5d3-8db4-46e4-b340-38f95a9c66af
**Repo:** xcl-website
**Depends on:**
- d05fac3c-fd76-4d05-a077-06f5e078b00a — Rewrite the configuration-only and plugin examples
**Execution:** agent

Rewrite every site page that shows type or plugin registration, catalogs or the pretty log handler, so it shows the new API. Refresh the example snippets from the rewritten example sources. Add a Registries guide covering local registries, load order, how registration problems are reported and clashes. Add a configuration-only section covering use without state. Link both from the navigation.

*Technical detail:* [context.md#task-update-the-documentation-site-to-the-new-api](./context.md#task-update-the-documentation-site-to-the-new-api)

**Acceptance criteria**:
- [x] No page mentions the removed registration calls, the catalog option or the old log handler signature
- [x] The site has a Registries guide and a section on configuration-only use without state, both reachable from the navigation
- [x] The site builds and type-checks with no errors

**Descoped requirements**:
- Discovered plugins that fail are skipped — descoped: replaced by the user's planning decision that any plugin failure, discovered or registered, fails the load
- A failing discovered plugin is skipped — descoped: replaced by the user's planning decision that any plugin failure, discovered or registered, fails the load

## Open Questions

- **Is the configuration-only example repaired when its tasks run?** The user committed `example/configonly` in a broken state (`465566f`) and said they will fix it by hand. The answer depends on whether that repair has happened before the pretty log task and the examples rewrite task, and on what shape it takes: whether `ingressRoutes` and its tests exist. If the module does not compile for reasons unrelated to this change, or the route logic is missing, STOP and ask the user rather than rebuilding it.
- **Do raw-level event data decode into the registered Go type?** The plan assumes `savedentity.Decode` reads raw event data, meaning the entity before the provider ran, as well as processed data. That only shows once `Event.Entity()` is exercised with `EventDataRaw`. If raw data does not decode, STOP and ask whether `Entity()` should support only processed data, which is the spec's stated case.

## Out of Scope

- **The remote registry.** Its API is settled in the design, but nothing for it ships: no `NewRemote`, no stub, no remote `RegisterPlugin`. It is left to a future spec built on design `plugin-registries.md`.
- **Connecting to plugins running elsewhere (`registry.Connect`).** The `Plugin.Start` contract leaves room for it, but it is not built. It is left to a future spec.
- **Remote-only concerns.** Plugin download verification, version ranges, a download cache and fetching only the plugins a configuration uses are all deferred with the remote registry.
- **A migration guide or old-to-new upgrade table.** The implement workflow's changelog entry, with its Breaking list, is the only migration note.
- **Compatibility shims or deprecated aliases** for `PluginRegistry`, `WithPluginRegistry`, `RegisterPluginWithPath`, `DiscoverPlugins`, package-level `EncodeSavedEntity` or `CastResourceTo`. Breaking the API is allowed.
- **Sharing one set of started plugins between several Configs.** Each Config now starts its own hosts, even when given the same registry value.
- **The editor extension (xcl-vscode).** Syntax highlighting is unaffected.
- **Rewriting historical CHANGELOG entries**, and historical spec or plan records that mention the old API.
- **Updating the spec and the design document to reflect the user's "any plugin failure is an error" decision.** That is not done by this plan. It is offered at plan review, through the spec and design workflows.
- **Repairing the configuration-only example's current broken work in progress.** The user will do this by hand; this plan only moves it to the new API.

## Changelog

### 2026-10-08 — Task: Event entity accessor

**What was done**: `events.Event` gained an unexported decoder, `WithEntityDecoder` and `Entity()`, which decodes the event's data into the registered Go type on every call, with sensitive values masked. `Config.run` attaches `c.decodeEventEntity` (`savedentity.Decode` with `ForDisplay`) to every event it delivers. Event JSON output is unchanged.

**Deviations**: The planned test asserted `Password.Reveal() == types.SensitiveMarker`. A masked `types.Sensitive` *shows* the marker (`String`/`Format`/`MarshalJSON`), but `Reveal()` returns the zero value by design, so the tests assert `fmt.Sprint(secret.Password)` and add a separate test that `Reveal()` is empty. `example/configonly` (committed broken at 465566f) was restored from `b0c43e2`, as the user approved, so the e2e suite could pass during verification. Stale Docker resources (`app` network, `web` container) were removed, with the user's approval, so the plugin example tests could run.

**Files changed**:
- `xclconfig: events/events.go`
- `xclconfig: events/events_test.go`
- `xclconfig: events.go`
- `xclconfig: config.go`
- `xclconfig: config_event_entity_test.go`
- `xclconfig: example/configonly/main.go`
- `xclconfig: example/configonly/routes.go`
- `xclconfig: example/configonly/routes_test.go`

**Discoveries**: Raw (`EventDataRaw`) event data decodes into the registered type as well as processed data, which settles the plan's open question. `example/plugin` tests leave or collide with a Docker network named `app` and a container named `web`, and a leftover from an earlier run makes every apply test fail with "network already exists app".

### 2026-10-08 — Task: Encode saved data through the Config

**What was done**: The package-level `EncodeSavedEntity(registry, data, ...)` is replaced by `(*Config).EncodeSavedEntity(data, ...)`, which loads the Config's plugins through `Use(nil)` (stopping external processes again afterwards), decodes with `ForDisplay` and encodes. Every caller moved to the method, and the example pretty log handler, for now, builds a Config from its registry to call it.

**Deviations**: No new `TestConfigEncodeSavedEntityMatchesEncodeEntity` test was added, because the migrated `TestEncodeSavedEntityMatchesEncodeEntity` and `TestProcessedDataConvertsLikeTheEntity` already assert that equality. `TestEncodeSavedEntityLoadsUnloadedRegistry` was renamed `TestConfigEncodeSavedEntityLoadsPluginsFirst`. The test helpers `applySensitiveFixtureWithEventData` and `applySensitiveFixtureWithEventOptions` now return the Config instead of the registry.

**Files changed**:
- `xclconfig: encode.go`
- `xclconfig: example/prettylog/prettylog.go`
- `xclconfig: example/prettylog/prettylog_test.go`
- `xclconfig: config_event_data_test.go`
- `xclconfig: config_event_sensitive_test.go`
- `xclconfig: config_event_mask_test.go`
- `xclconfig: config_test.go`
- `xclconfig: encode_test.go`
- `xclconfig: encode_references_test.go`
- `xclconfig: encode_highlight_test.go`
- `xclconfig: encode_sensitive_test.go`
- `xclconfig: sensitive_leak_test.go`
- `xclconfig: e2e/encode_test.go`
- `xclconfig: e2e/plugin_encode_test.go`
- `xclconfig: e2e/plugin_state_test.go`
- `xclconfig: e2e/sensitive_test.go`

**Discoveries**: `Use` holds its mutex only while it runs, so calling `EncodeSavedEntity` from an event handler during an operation only increments the user count and does not deadlock.

### 2026-10-08 — Task: Pretty log handler without a catalog

**What was done**: `prettylog.Handler` now takes only a writer and a level. It writes a created entity's configuration by encoding `Event.Entity()` with `xcl.EncodeEntity`, and the interim Config built from a registry in the previous task is gone. The plugin and configuration-only examples call the two-argument form.

**Deviations**: Hand-built events no longer decode, so the tests that rendered configuration now drive a real Config through a new one-block fixture, `example/prettylog/testdata/cache/main.xcl`. The old "type nothing registered" warning test became `TestHandlerReportsDataItCannotDecode`: a hand-built event with data and no decoder logs the warning. The unknown-type case can't happen through a real Config. `TestHandlerWritesNothingWhenRegistryIsNil` became `TestHandlerWritesNothingExtraWhenConfigSendsNoData`. The unused `savedWidgetRecord` helper was removed.

**Files changed**:
- `xclconfig: example/prettylog/prettylog.go`
- `xclconfig: example/prettylog/prettylog_test.go`
- `xclconfig: example/prettylog/testdata/cache/main.xcl`
- `xclconfig: example/plugin/main.go`
- `xclconfig: example/configonly/main.go`

**Discoveries**: An event carrying data but no decoder (built by hand) makes `Entity()` fail. The handler logs that as a warning rather than dropping it silently.

### 2026-10-08 — Task: Public registry package and local registry

**What was done**: Added the public `github.com/jumppad-labs/xcl/registry` package. It holds the design's `Registry` and `Plugin` interfaces, plus `InProcess`, which wraps the direct host and panics on nil, and `Executable`, which wraps the gRPC host; a missing path fails at `Start` with the path in the error. It also adds `NewLocal` with `PluginPattern`, `RegisterPlugin`, `RegisterExternalPlugin` and `RegisterPluginDirectory`, all returning nothing. `Local.Plugins` returns entries in registration order, with each directory's matches in its place, and emits `discover` events carrying `registry: local`. Directory discovery was copied, unexported, into `registry/discovery.go` with its tests.

**Deviations**: The old `plugins/registry/plugin_discovery.go` and its tests stay in place until the catalog task deletes that package. Each registered directory is searched, and reported as its own discover start/success pair, at its registration position; there is no longer one search over all directories. Added constants `LocalName` and `DefaultPluginPattern`. The discovery tests that drove the old `PluginRegistry` load were not copied; they belong to the catalog task.

**Files changed**:
- `xclconfig: registry/registry.go`
- `xclconfig: registry/local.go`
- `xclconfig: registry/discovery.go`
- `xclconfig: registry/discovery_test.go`
- `xclconfig: registry/local_test.go`

**Discoveries**: `plugins/example` is `package main`, so there is no importable in-process test plugin. `registry/local_test.go` defines its own `thingPlugin`, copied from the old registry tests.

### 2026-10-08 — Task: Plugin load and type clash errors name registries

**What was done**: `PluginLoadError` gained `Registry`, and its message names the plugin and the registry, or only the registry when the registry itself failed. `TypeNameClashError` (now with `Provider`, `Registry`, `Existing`, `ExistingRegistry`) and `TypeFormError` moved to the shared `errors` package. They are re-exported from root as `xcl.TypeNameClashError` and `xcl.TypeFormError`. `plugins/registry/errors.go` now holds type aliases, so the old package compiles until it is deleted.

**Deviations**: Both messages keep a fallback form while the old registry still builds errors without the new fields: `plugin %s failed to load` when `Registry` is empty, and `type %q is already provided by %s` when `Provider` is empty.

**Files changed**:
- `xclconfig: errors/plugin_load_error.go`
- `xclconfig: errors/plugin_load_error_test.go`
- `xclconfig: errors/type_errors.go`
- `xclconfig: errors/type_errors_test.go`
- `xclconfig: plugins/registry/errors.go`
- `xclconfig: config.go`
- `xclconfig: errors_reexport_test.go`

**Discoveries**: None.

### 2026-10-08 — Task: Internal type catalog loading from registries

**What was done**: The type catalog and plugin loader moved from `plugins/registry` to a private `internal/catalog` (`Catalog`, `New()`), and `plugins/registry` was deleted. The loader goes through the registries given with `AddRegistry` in order, starting each plugin in turn. Any plugin that fails to start fails the load, discovered or not, with a `PluginLoadError` naming the plugin and registry. Every duplicate block type fails the load with a `TypeNameClashError` naming both providers and registries. `RegisterType` panics on mistakes, and load events carry `registry` in Meta. The parser (`ParserOptions.Catalog`), savedentity and their tests use the catalog.

**Deviations**: Done together with the Config options task. Deleting `plugins/registry` breaks every caller at once, so root tests were migrated once, straight to `WithType`/`WithRegistry`; there was no interim `WithPluginRegistry(*catalog.Catalog)`. The shape checks were pulled into an exported `catalog.ValidateDeclaration`, shared with `WithType`. For a binary, `PluginLoadError.Plugin` is now the file name; the path is still in the message. A type registered after load is no longer checked against loaded plugin types, since `NewConfig` registers every type before any load.

**Files changed**:
- `xclconfig: internal/catalog/catalog.go`
- `xclconfig: internal/catalog/catalog_test.go`
- `xclconfig: internal/catalog/sensitive_types_test.go`
- `xclconfig: internal/catalog/type_path_test.go`
- `xclconfig: internal/catalog/testutils_events_test.go`
- `xclconfig: internal/catalog/testutils_plugins_test.go`
- `xclconfig: plugins/registry/ (deleted)`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/callbacks.go`
- `xclconfig: internal/parser/*_test.go`
- `xclconfig: internal/savedentity/savedentity.go`
- `xclconfig: internal/savedentity/*_test.go`
- `xclconfig: state/file_state_store_test.go`
- `xclconfig: state/file_state_store_apply_test.go`
- `xclconfig: state/custom_store_test.go`
- `xclconfig: config_diff.go`
- `xclconfig: query.go`
- `xclconfig: encode.go`

**Discoveries**: Each Config now starts its own plugin hosts. Root tests that shared one registry across Configs, such as `TestSharedRegistryStartsExternalPluginForEachConfig` and the per-Config plugin log routing tests, now run on separate processes.

### 2026-10-08 — Task: Config options declare types and add registries

**What was done**: Added `WithType(prototype, name...)`, which checks its arguments' shape at the call and panics naming the type, and `WithRegistry(r)`, which panics on nil and may repeat. `WithPluginRegistry` was removed. `NewConfig` collects every option, then builds the Config's own catalog: it registers declared types (panicking on duplicates, form clashes and builtins) and adds registries in the order given. Every root test was migrated, and new acceptance tests cover the spec's criteria: no state, declared types, panics, missing and failing plugins, two registries, load order, every kind of duplicate, a custom pattern, and a custom registry with its own plugin starter.

**Deviations**: Done together with the catalog task (see above). `TestRegisterPluginWithPathAcceptsMissingBinary` was removed, because `registry` covers it. `TestCustomPatternLoadsOnlyMatchingPlugins` uses the subtypeless widget plugin. The new fixtures are `internal/test_fixtures/config/registries/{two_registries,custom}/main.xcl`.

**Files changed**:
- `xclconfig: options.go`
- `xclconfig: config.go`
- `xclconfig: config_no_state_test.go`
- `xclconfig: config_with_type_test.go`
- `xclconfig: config_registries_test.go`
- `xclconfig: config_custom_registry_test.go`
- `xclconfig: internal/test_fixtures/config/registries/two_registries/main.xcl`
- `xclconfig: internal/test_fixtures/config/registries/custom/main.xcl`
- `xclconfig: config_*_test.go, decode_test.go, encode*_test.go, entity_subtype_test.go, query*_test.go, sensitive_leak_test.go (migrated)`

**Discoveries**: Root tests can reach `c.catalog` directly (package `xcl`), so host-level assertions use `c.catalog.GetPluginHosts()` and `c.catalog.Use(nil)` without any exported hook.

### 2026-10-08 — Task: Migrate e2e and plugin SDK example tests

**What was done**: The e2e helpers now build `kubeTypes()` as `WithType` options and `newLocalRegistry()` as a local registry. The `GetPluginHosts` cleanups are gone. The missing-plugin e2e test asserts the error names `from registry local`. In `plugins/example` tests, a catalog with a local registry is used where `internal/parser` runs directly, and the load-event tests go through a real Config and expect `registry: local` in Meta.

**Deviations**: `plugins/example/apply_test.go` and `sensitive_test.go` drive the parser directly, so they use `internal/catalog` with `AddRegistry`, not `WithRegistry`. No `rejected` event assertions existed to remove.

**Files changed**:
- `xclconfig: e2e/helpers_test.go`
- `xclconfig: e2e/plugin_errors_test.go`
- `xclconfig: e2e/plugin_events_test.go`
- `xclconfig: e2e/{destroy,diff,encode,events,fixtures,kube_helpers,plugin_encode,plugin_helpers,plugin_silence,plugin_state,sensitive,silence}_test.go`
- `xclconfig: plugins/example/apply_test.go`
- `xclconfig: plugins/example/sensitive_test.go`
- `xclconfig: plugins/example/e2e_test.go`

**Discoveries**: None.

### 2026-10-08 — Task: Rewrite the configuration-only and plugin examples

**What was done**: `example/configonly` declares its five types with `WithType` in `loadConfig(dir, options...)` and keeps no state: there is no key, mask, temp dir or registration error check. `DB_PASSWORD=x go run . ./config` prints `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)`. `example/plugin`'s `newConfig(handler, dockerPlugin, stateDir)` builds one local registry and adds it with `WithRegistry`, and its missing-plugin test asserts `from registry local`. The `example/prettylog` tests use `WithType`/`WithRegistry`.

**Deviations**: The starting point was the configonly example restored from `b0c43e2`, as the user approved, not a hand-repaired version. `go mod tidy` changed nothing.

**Files changed**:
- `xclconfig: example/configonly/main.go`
- `xclconfig: example/configonly/routes_test.go`
- `xclconfig: example/plugin/main.go`
- `xclconfig: example/plugin/main_test.go`
- `xclconfig: example/prettylog/prettylog_test.go`

**Discoveries**: None.

### 2026-10-08 — Task: Untrack the stray binary and ignore example builds

**What was done**: The 24 MB root `configonly` binary was removed from the index and deleted. `.gitignore` now covers `/configonly`, `example/configonly/configonly`, `example/plugin/plugin` and `example/prettylog/prettylog`. A plain `go build .` in each example leaves `git status` unchanged.

**Deviations**: None.

**Files changed**:
- `xclconfig: configonly (deleted)`
- `xclconfig: .gitignore`

**Discoveries**: None.

### 2026-10-08 — Task: Update in-repo guides to the new API

**What was done**: `README.md` gained a Quick start with the configuration-only `WithType` setup, a no-state note, a plugin setup with `registry.NewLocal` + `WithRegistry`, and a registration-problems note: panics for code mistakes, `PluginLoadError`/`TypeNameClashError` from the first operation. Its type and plugin sections were rewritten. `docs/overview.md`, `docs/plugins.md`, `docs/state.md`, `docs/parser-lifecycle.md` and `docs/README.md` now describe the internal catalog, the public `registry` package, `c.EncodeSavedEntity` and `Event.Entity()`. Two behaviour notes in `e2e/COVERAGE.md` were updated. `CHANGELOG.md` history is untouched.

**Deviations**: `e2e/COVERAGE.md` keeps the historical name `TestExamplesCreatePluginRegistryWithoutArguments` in its table of removed tests.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: docs/overview.md`
- `xclconfig: docs/plugins.md`
- `xclconfig: docs/state.md`
- `xclconfig: docs/parser-lifecycle.md`
- `xclconfig: docs/README.md`
- `xclconfig: e2e/COVERAGE.md`

**Discoveries**: The README's Custom Functions section (`p := NewParser(...); p.RegisterType(...)`, around line 1824) was already out of date before this change, and was left alone. It needs a separate fix.

### 2026-10-08 — Task: Update the documentation site to the new API

**What was done**: The pages `index.mdx`, `events.mdx`, `plugin-logging.mdx`, `configuration-text.mdx`, `examples/configuration-only.mdx` and `examples/plugins.mdx` now show `WithType`, local registries, `Event.Entity()`, `c.EncodeSavedEntity` and the two-argument `prettylog.Handler`. Example snippets were refreshed byte for byte from the example sources (37 blocks checked, 0 mismatches). A new `registries.mdx` guide covers declaring types, configuration only without state, local registries, load order, how problems are reported, clashes and writing your own registry. It is linked from the Guides nav and the README pages table. `npm run build` and `npx astro check` both report 0 errors.

**Deviations**: `plugin-logging.mdx`'s `rejected=true` paragraph was removed, because that behaviour is gone. The "Writing your own registry" section includes an illustrative `Remote` registry that is not in the source; it needs review against the spec's out-of-scope note on the remote registry. The sample `load` error output in `events.mdx` was derived from the source, not captured from a run.

**Files changed**:
- `xcl-website: src/pages/index.mdx`
- `xcl-website: src/pages/events.mdx`
- `xcl-website: src/pages/plugin-logging.mdx`
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/pages/examples/configuration-only.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`
- `xcl-website: src/pages/registries.mdx`
- `xcl-website: src/components/Nav.astro`
- `xcl-website: README.md`

**Discoveries**: None.
