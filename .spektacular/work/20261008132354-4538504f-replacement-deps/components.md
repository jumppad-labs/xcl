- **Change vocabulary (new, public `plugins` package).** Owns `Change` (`NoChange`, `Update`, `Replace`, with a `String` method) and `DependencyChange`. It is the shared language of the provider contract, the adapters, the gRPC protocol and the core's decision record. It has no dependencies beyond the standard library.
- **Provider contract (changed, `plugins`).** `ResourceProvider[T].Changed` takes the dependency list and returns a `Change`. The doc comments on `Changed` and `Update` describe what the core does with each answer.
- **`DefaultChanged` (changed, `plugins`).** Keeps its comparison of configured values (ignoring meta, `depends_on` and `disabled`) and answers `Update` or `NoChange`. It ignores dependencies, so a provider that has to react to a replaced dependency overrides `Changed` itself. Every in-repo provider that embeds it migrates with it.
- **Adapter and host chain (changed, `plugins`).** `ProviderAdapter`, `TypedProviderAdapter`, the `Plugin` and `PluginHost` interfaces, `PluginBase`, the direct host and its sourced adapter, and the gRPC resource adapter pass `dependencies` in and the `Change` out, unchanged. The plugin-testing helpers and the generated adapter mock follow suit.
- **gRPC protocol (changed, `plugins/plugin.proto` and the generated `plugins/proto`, plus `grpcPluginWrapper` and `GRPCServer`).** Carries the `Change` enum and the repeated dependency list across the process boundary. The wrapper and server convert between the proto and Go types, so external and in-process plugins see identical values.
- **Decision record (new, `internal/parser`, grown from the diff recorder).** A concurrency-safe store of each entity's decision for one operation:
  - the outcome: create, update, replace, delete or unchanged;
  - the dependency changes the entity was told;
  - the reason for a replacement (last apply failed, provider, or replaced dependencies);
  - the decide-pass read copy;
  - the pending and unknown-path information the diff recorder already keeps.

  It answers "what are this entity's changing dependencies", produces the `diff.Diff` for a plan, and gives the act pass the lists of entities to destroy and how to treat each one.
- **Dependency resolver (new, `internal/parser`).** For one entity, resolves its links to the provider-backed resources it reaches, looking through outputs, variables, modules and registered config-only types. Using the decision record, it returns the `[]plugins.DependencyChange` for those that will update or replace.
- **Decide pass (changed: today's diff walk and the lifecycle's diff step).** Runs in dependency order for both plan and apply and never acts. For each entity it:
  - assigns create or replace from saved status as today;
  - otherwise calls Read, then `Changed` with the resolved dependencies, through the shared refresh;
  - for an entity with unknown inputs, substitutes the saved values before Read and floors the outcome at `Update`;
  - marks entities that will be created, updated or replaced as pending, so their computed values are unknown to dependents.

  Any provider error stops the pass.
- **Refresh (changed, `internal/parser`).** The single Read + `Changed` step. It takes the dependency list, returns the provider's `Change`, and keeps the read copy for the act pass.
- **Act pass (changed: `Parser.Apply` and the lifecycle's apply step).** Runs only after a successful decide pass:
  1. hands every replaced and removed entity to the destroyer;
  2. runs the apply walk in dependency order, where the lifecycle follows each entity's decision (create, `Update` with fresh configuration plus carried computed values, or keep the decide copy) instead of reading and comparing.

  The in-walk rebuild is removed.
- **Destroyer (reused, `internal/parser`).** Unchanged mechanics: a reverse graph over the saved links of its targets, a destroy per entity, and a state save after each one. Its targets now include replaced entities as well as removed ones.
- **Diff result and renderer (changed, public `diff` package).** `Resource` gains the replacement reason and the replaced dependency addresses. `Render`'s comment line names the cause ("its last apply failed", "it cannot be updated in place", or "because <address> is replaced"). The JSON form carries the same fields.
- **Recording test plugin (changed, `internal/parser` `TestPlugin`).** Can be set to answer any `Change` per entity, and records the dependency list each `Changed` call received. It is used by every core replacement test.
- **In-repo providers (changed).**
  - The Docker network and container providers and the template provider in the plugin example, and the person provider in the plugin SDK example, override `Changed` with their replace rules.
  - The e2e fixtures, prettylog fixtures, subtypeless fixture and test fakes migrate through `DefaultChanged`.
  - The no-op Docker `Update`s are kept only for changes that can be made in place.
- **Plugin example configurations (changed, `example/plugin`).** The main configuration stands alone again. A new subnet-change configuration in its own directory drives the example's replacement tests and the documentation walkthrough.
- **Documentation (changed: xclconfig guides, README and changelog; xcl-website pages and nav).**
  - A plugin-author explanation of unchanged, update and replace, with the dependency list, as a new website guide page linked from the Guides menu.
  - The diff guide's replace semantics and sample output.
  - The plugin example page's subnet walkthrough.
  - The core guides that show the `Changed` signature.
