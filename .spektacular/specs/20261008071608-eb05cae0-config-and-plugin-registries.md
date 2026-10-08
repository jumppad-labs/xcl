---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
designs:
    - source: design
      path: plugin-registries.md
amendments:
    - at: "2026-10-08T11:42:42Z"
      sections: [Requirements, Acceptance Criteria]
      design: {source: design, path: plugin-registries.md}
      hash: sha256:4c8d87359c1aa6ecc189fc38f7ef069dce1b1c33631d98fdc41df301523e9d29
---

# Feature: 20261008071608-eb05cae0-config-and-plugin-registries

## Overview

Setting up xcl today takes too much ceremony. Even a program that only reads configuration into its own types has to build a plugin catalog by hand, check an error after every type it registers, and set up state it then throws away. This change makes those programs a few lines long: types are declared when the configuration is created, and running with no saved state is a supported mode. Plugins come from one or more registries that the application names explicitly, starting with a local one, and the API is shaped so a remote registry can be added later without changing it. Go developers embedding xcl, whether only for configuration or with plugins, get a shorter setup that's easier to read and harder to get wrong.

## Requirements

- [x] **Configuration without saved state**
  Users can load and apply configuration without any state being persisted, simply by not choosing a place to keep state, and the documentation describes this as the supported mode for configuration-only use.
- [x] **Declare types on a registry**
  Users can declare each of their own Go types, with its block type name and optional subtype, on a registry, without building or holding a separate catalog object.
- [x] **Registration never returns an error**
  Every way of registering a type, a plugin, a plugin binary or a plugin directory returns no error value, so registering needs no error check at the call site.
- [x] **Programmer mistakes in registration stop the program immediately**
  The system must panic, naming the offending type or plugin, when a single registration call is wrong in a way that would fail on every run: an empty type name, more than one subtype, an empty subtype, a type that is not a valid entity type, or a missing in-process plugin or registry.
- [x] **Type clashes are reported when the configuration is created**
  Creating a configuration must return an error, naming both types and their registries, when the same block type is declared twice in one registry or across two, when a type keyword is declared both with and without a subtype, or when a declared type uses a builtin name.
- [x] **Environment problems are reported when plugins load**
  The system must report a plugin binary that is missing or fails to start as an error from the first operation that loads plugins, naming the plugin and the registry it came from.
- [x] **Registries supply plugins**
  Users can obtain plugins only through registries, and can add any number of registries to a configuration; plugins from every added registry are available to the configuration.
- [x] **Local registry**
  Users can create a local registry and register on it an in-process plugin, a plugin binary by path, and a directory whose plugin binaries matching a search pattern are discovered; the search pattern is set once when the registry is created and defaults to the existing plugin naming pattern.
- [x] **Load order follows registration order**
  The system must load registries in the order they were added to the configuration, and plugins within a registry in the order they were registered.
- [x] **Any plugin that fails to start fails the load**
  A plugin that fails to start, whether registered explicitly or found by directory discovery, fails the load with an error naming the plugin and its registry.
- [x] **A block type provided twice is always an error**
  The system must fail the load with an error naming both providers and their registries whenever the same block type and subtype is provided twice: within one registry, across two registries, or by a plugin and a declared Go type.
- [x] **Custom registries**
  Users can write their own registry, and their own way of starting a plugin, against the public registry and plugin contracts.
- [x] **Event handlers can read an event's entity without a catalog**
  Users can ask an event that carries resource data for the entity it holds, as the user's registered Go type with sensitive values shown masked, without holding any catalog or registry; an event without data yields nothing.
- [x] **Events still serialise cleanly**
  Events must still serialise to JSON, with no part of the entity-reading capability appearing in the output.
- [x] **Pretty event logging needs no catalog**
  The example pretty log handler can be created from only an output and a log level, and still writes each created entity's configuration beneath its event.
- [x] **Encoding saved data through the configuration**
  Users can turn a saved entity record back into configuration text through the configuration object, without a separate catalog.
- [x] **The catalog is no longer public**
  The system must not expose the internal type and plugin catalog in its public API, and the option for supplying one to a configuration no longer exists.
- [x] **Examples use the new API**
  The configuration-only example declares its types on a local registry and keeps no state, and has no registration error checks; the plugin example uses one local registry; neither passes a catalog to the log handler.
- [x] **Documentation uses the new API**
  Every documentation-site page that shows type or plugin registration, catalogs, or the pretty log handler shows the new API, and the site explains configuration-only use without state, registries, and how registration problems are reported.
- [x] **No committed build outputs**
  The repository must not contain the committed configuration-only example binary, and example build outputs must be ignored by version control.

## Constraints

- **Built to the design.** The API must follow the design document `plugin-registries.md` in the `design` source: its option names, registry and plugin contracts, local registry calls, failure rules and what becomes internal. A disagreement with the design is raised with the user, not designed around.
- **Breaking the public API is allowed.** The user chose the nicest syntax over compatibility; no deprecated aliases or compatibility shims are required.
- **No precedence between registries.** A block type provided twice is always an error; the user explicitly rejected first-registry-wins.
- **Remote plugin registration names only a plugin and a version.** The plugin defines the types it provides, so no registration call (now or when a remote registry is built) may name types on a plugin's behalf.
- **Room for remote plugins.** The public registry and plugin contracts must allow a remote registry, and a plugin reached over the network, to be added later without changing those contracts.
- **Tests follow the repository rules.** testify `require`, no table-driven tests, positive and negative cases in separate test functions, Mockery for mocks.
- **No dependency cycle for events.** The events package must not depend on the root package, so reading an event's entity cannot itself produce configuration text.

## Acceptance Criteria

- [x] **Applying without state writes nothing**
  A program that creates a configuration with no state location, declares its types and applies a configuration directory succeeds, its entities are readable afterwards, and no state file or directory is created anywhere.
- [x] **Types declared on a registry decode configuration**
  A configuration given a registry with declared types, and no catalog built by the program, applies a configuration that uses those block types and decodes them into the program's own Go values with the configured values.
- [x] **Registration calls have no error result**
  A program that registers types, an in-process plugin, a plugin binary path and a plugin directory compiles with each registration written as a single statement with no error assigned or checked.
- [x] **Each registration mistake panics with its name**
  Each of these panics with a message naming the type or plugin: an empty type name, more than one subtype, an empty subtype, a type that is not a valid entity type, and a missing in-process plugin or registry.
- [x] **Declared type clashes are returned when creating the configuration**
  Declaring the same block type twice in one registry, the same block type in two registries, one keyword in both forms, or a builtin name makes creating the configuration return an error naming both types and their registries.
- [x] **Missing plugin binary fails at load, not at registration**
  Registering a plugin binary path that does not exist does not fail; the first apply returns a plugin load error that names the path and the registry.
- [x] **Plugins from several registries are all usable**
  A configuration given two local registries, each providing a different plugin, applies a configuration using block types from both plugins successfully.
- [x] **Directory discovery honours the pattern**
  A local registry created with a custom search pattern and given a directory loads the plugin binaries matching that pattern and none of the files that do not match.
- [x] **Load order is observable in events**
  With two registries, the plugin load events appear in the order the registries were added, and within a registry in the order its plugins were registered.
- [x] **A failing discovered plugin fails the load**
  When a discovered plugin binary fails to start, the first apply returns an error naming it and the local registry.
- [x] **A failing registered plugin fails the load**
  When an explicitly registered plugin binary fails to start, the first apply returns an error naming it and the registry it came from.
- [x] **Duplicate type across registries fails the load**
  Two registries providing the same block type and subtype make the first apply fail with an error that names both providers and both registries.
- [x] **Duplicate type within a registry fails the load**
  One registry providing the same block type and subtype from two plugins makes the first apply fail with an error that names both providers.
- [x] **Plugin type clashing with a declared type fails the load**
  A plugin providing a block type that was also declared as a Go type makes the first apply fail with an error naming the plugin and the type, whether the registry declaring the type was added before or after the plugin's registry.
- [x] **A custom registry and plugin starter work**
  A registry written outside xcl against the public contracts, returning a plugin with its own start behaviour, has its plugin started and its block types applied.
- [x] **Event entity is the registered Go type**
  With processed event data enabled, the success event for a created resource returns, when asked for its entity, a value of the program's registered Go type holding the configured values, with any sensitive value shown as the mask marker; an event with no data returns nothing and no error.
- [x] **Events serialise to JSON without the entity-reading capability**
  Marshalling such an event to JSON succeeds and the output contains no field for the entity-reading capability.
- [x] **Pretty log writes configuration with no catalog**
  The example log handler, created with only an output and a level, writes a created resource's configuration text beneath its success line.
- [x] **Saved data encodes through the configuration**
  Encoding a saved entity record through the configuration returns the same configuration text as encoding the entity directly.
- [x] **No public catalog**
  The public packages expose no catalog type and no configuration option that accepts one; a program using the removed option or catalog type fails to compile.
- [x] **Configuration-only example is minimal**
  The configuration-only example's source contains no catalog construction, no state option, no state key or mask, no registration error check, and running the example prints the expected ingress route and its tests pass.
- [x] **Plugin example uses a local registry**
  The plugin example builds one local registry, adds it to the configuration, has no registration error checks, and its tests pass.
- [x] **Documentation shows only the new API**
  No page on the documentation site mentions the removed registration calls, the catalog option or the old log handler signature; the site has a section on configuration-only use without state and on registries; and the site builds and type-checks.
- [x] **Build outputs are gone and ignored**
  The repository no longer tracks the configuration-only example binary, and building an example leaves no new untracked file reported by version control.

## Technical Approach

- **Split the catalog from where plugins come from.** The existing plugin catalog keeps its two jobs, resolving block types and loading plugins, as an internal component behind the new public options and registries.
- **Reuse the existing plugin hosts.** The in-process and binary plugin starters wrap today's direct and process-over-gRPC hosts rather than introducing new transports.
- **Defer all registration failures to one place.** Environment problems, including a clash with a plugin that has already loaded, are reported from the existing once-per-configuration plugin load, so the result no longer depends on whether registration happened before or after loading.
- **Decode event entities on demand.** The configuration attaches the decoding capability to events in the single place it already wraps every operation, and decoding happens from the event's serialised data so a buffered handler gets its own copy.
- **Mechanical call-site migration.** Most of the work is rewriting the many existing tests, examples and docs that register types and plugins; the changes are mechanical (drop error checks, rename, move registration onto a registry).
- **Risk: panic tests replace error tests.** Existing tests that assert registration errors become panic tests, and the plugin clash tests move to the load.

## Success Metrics

- The configuration-only example's setup, from creating the configuration to having its decoded values, has no error checks other than the ones on creating and applying the configuration, and no state, key or catalog code.
- Every registration failure case that has a test today still has one after the change, as a panic test or a load-time error test.
- The full test suite, the example tests and the documentation site build all pass.

## Non-Goals

- **Remote registry.** Its API is settled in the design, but nothing for it ships in this change: no remote registry constructor, no stub.
- **Connecting to plugins running elsewhere.** Not built in this change.
- **Plugin download verification, version ranges and fetching only the plugins a configuration uses.** These belong to the remote registry and are deferred with it.
- **A migration guide.** The changelog entry is enough; no old-to-new upgrade table is written.
- **The editor extension.** Syntax highlighting is unaffected and needs no change.

## Amendments

- **2026-10-08: Requirements, Acceptance Criteria; design design:plugin-registries.md** (post-implement revision)
  User clarified types belong on registries, not Config: WithType is removed and types are declared with Local.RegisterType (Registry gains Types()). Declared-type clashes (duplicate within/across registries, both forms, builtin name) are errors returned by NewConfig, not panics. Any failing plugin, including a discovered one, fails the load (replaces the descoped 'skipped' items). Design plugin-registries.md revised to match.
