**Code this plan builds on**
- **`internal/parser` diff walk, recorder, refresh and decoder.** These become the decide pass and the decision record: the diff recorder is extended, refresh gains dependencies and a replace outcome, and the walk's modes change meaning. They are the core of the change.
- **`internal/parser` destroyer and destroy graph builder.** Reused as the act pass's destroy phase with a wider target list. No change to their mechanics.
- **`internal/parser` apply walk, lifecycle and progress.** The lifecycle's apply step follows decisions instead of reading and comparing, and the in-walk rebuild is removed. Progress and partial-state building are unchanged.
- **`internal/dag` walker.** Its dependency-order guarantee, concurrency and upstream-failure skipping are relied on as they are. No change.
- **`plugins` package: provider contract, `DefaultChanged`, adapters, direct host and gRPC host/server.** Changed to carry `Change` and the dependency list. This change must land first, in one step, so the repository builds.
- **`plugins/plugin.proto` and the generated `plugins/proto`.** Gain an enum, a message and new fields, and are regenerated with the generator versions already recorded in the generated files (protoc-gen-go v1.36.11, protoc-gen-go-grpc v1.5.1).
- **`diff` package.** `Resource` gains the reason fields, and `Render` phrases the reason. Everything else is unchanged.
- **`types` (statuses, `Meta.Links`), `events`, `logger`, `errors`, `state`.** Used as they are. No new statuses or event operations.
- **`internal/parser` `TestPlugin`.** Its `Changed` knobs change type, and it gains dependency recording.
- **`.mockery.yml` / the generated `plugins/mocks`.** The adapter mock is regenerated with Mockery v3.8.0, the version the plugin example already pins.
- **Example modules (`example/plugin`, `example/prettylog`, `example/configonly`) and the plugin SDK example (`plugins/example`).** They depend on the local xcl through `replace` and migrate in the same change as the contract. `example/plugin`'s Docker client and its mock are unchanged.

**External libraries**
- No new libraries. The existing pinned `google.golang.org/protobuf` and `google.golang.org/grpc` runtime versions are unchanged; only the generated code is regenerated.

**Repositories**
- **xclconfig** (`/home/nicj/code/github.com/jumppad-labs/xcl`) — all code, tests, examples, core docs and the changelog.
- **xcl-website** (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) — the new plugin-author guide page, the Guides nav entry, and updates to the diff and plugin-example pages. It is built with its existing Astro build.

**Upstream specs and plans**
- Nothing has to land first. This plan builds on the shipped diff work (plan `20261007105731-2388b579-diff`) and diff rendering (`20261007111826-cf3b66d8-diff-rendering-and-docs`), and on the registries work on the current `f-diff` branch, all of which are already in the tree.
- `example/plugin/config/alt.xcl` (commit 8f6f931) is the user's subnet configuration. This plan moves it rather than recreating it.

**Design documents this plan was built on**
- `replacement-and-dependency-changes.md` from the `design` source — the settled shape: the `Change`/`DependencyChange` contract, what a resource is told about its dependencies, decide-then-act, destroy-then-create order, and `-/+` plans showing the reason. It is binding. The one presentation detail that differs from the sketch (reason on the comment line above the header) was the user's decision during planning.
