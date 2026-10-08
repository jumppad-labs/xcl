---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Research: 20261008194959-b128e508-update-property-changes

## Alternatives considered and rejected

### Option A: Hand plugins `diff.Change` directly

`plugins` would import `diff`, which pulls `internal/cty`, `internal/xcl/hclsyntax`, `hclwrite` and `highlight` into every plugin (diff imports them from `diff/render_value.go`). The spec's constraints also put the change type in `entity`, not in the plan-rendering code. Rejected.

### Option B: Put the change type in `entity` and have `entity` import `diff` for `Path`

There's no cycle today, but it breaks `entity/doc.go`'s stdlib-only rule and drags the same dependencies into plugins. Rejected. Instead, `Path`/`Step`/`StepKind` move to `entity` and `diff` re-exports them as type aliases. `diff/path.go` needs only `encoding/json`, `strconv` and `strings`.

### Option C: A text path (`"network[0].name"`) on the change

The spec asks for no text parsing. `diff.Path` is already structured (`diff/path.go:9-33`). Rejected.

### Option D: Use the Changed bytes (saved against post-Read) as the comparison

`refresh` calls `adapter.Changed(ctx, copies.old, copies.read, deps)` (`internal/parser/lifecycle.go:577`), and `copies.read` includes carried computed values and drift. The plan compares saved against *configured* (`lifecycle.go:418` → `resourceChanges`). The success metric requires the plugin's change set to equal the plan's. Rejected: changes are computed saved against configured, exactly as for the plan.

### Option E: Compute the change list once at decide time and reuse it at Update

Decide-time values have placeholders or saved values at unknown paths (`withSavedValues`, `diff_decode.go:253`), so they would carry stale or unknown values into Update. The spec requires real values at update time. Rejected: the list is recomputed in the act pass from the real decode (`callbacks.go:141`).

### Option F: Reveal sensitive values with `plainValue(..., reveal=true)` and pass the raw values

It unwraps `types.Sensitive` (`diff_changes.go:426-431`), so any `%v`, log or event of the change would print the secret. Rejected as the only safeguard. Values are real but the change type masks itself in `String`/`Format`/`LogValue`/`MarshalJSON` when `Sensitive` is set, and core never logs or events the plugin-facing list.

### Option G: Carry values across gRPC as `google.protobuf.Value`/structpb

There's no precedent in `plugins/plugin.proto` (no imports, entity data is JSON bytes). Rejected for JSON bytes per value, matching `entity_data`.

### Option H: Docker container: reconcile in Update by inspecting the container

The spec success metric says Update works only from what it is told. Rejected, apart from reading the new IP address after reconnecting (an output, not previous state).

### Option I: Init script rebuild via the container replacing on any template *update*

It would rebuild the container on every template source edit, against "rebuilds only what genuinely needs it". Rejected. The container replaces when the init-script producer is *replaced* (`template.Changed` replaces on `destination`, `example/plugin/plugins/template/template.go:56-62`).

### Option J: Cascade-delete the container when its network is removed

Forbidden by the spec's constraints.

### Option K: Two independent computations, one masked for the plan and one revealed for plugins

Run `resourceChanges` twice with different `reveal` flags (`internal/parser/diff_changes.go:39`). It is simpler to wire. Rejected: the two can drift apart, breaking the success metric that the plugin's settings equal the plan's.

### Option L: Compute at decide time and patch real values into the stored list at act time

Rejected: it needs per-path value resolution that duplicates the act-time decode (`callbacks.go:141`), and it is still wrong for values from dependencies applied earlier in the apply.

## Chosen approach — evidence

- The plan's change computation is reusable and pure: `resourceChanges(action, saved, configured any, body *hclsyntax.Body, unknown []diff.Path, reveal bool) []diff.Change` (`internal/parser/diff_changes.go:39`). It compares structs by reflection, skips computed fields (`computed.go:47`), and marks unknowns via `unknownAt`/`unknownUnder` (`diff_changes.go:305/316`).
- Masking happens after comparison: `compareSensitive` (`diff_changes.go:174`) compares real values (`RevealAny`) and only then drops Before/After unless `reveal`. A single real-valued computation can produce both the plugin list and the plan list (nil the values where `Sensitive && !RevealSensitive`), which guarantees the same set of paths.
- Decide inputs are already in hand before `Changed`. `old` is the saved copy; `decodeCopy(r, copies.configured)` (`lifecycle.go:336/355`) is the configured copy with real sensitive values from `wire.Marshal` (`lifecycle.go:528`); unknowns come from `l.recorder.unknownPaths(meta.ID)` (`lifecycle.go:313`). `copies.configured` is produced inside `refresh` before Read and Changed (`lifecycle.go:528` < `:577`).
- Act inputs: `update` (`lifecycle.go:158-192`) has the real decoded `r` (before `carryComputedValues` at :164). The saved copy comes from `findByID(l.previous.GetResources(), meta.ID)` (pattern at `lifecycle.go:202`, `keep`). Updated resources are not destroyed, so `l.previous` still holds them (only replaced or removed ones leave `previousState` at `parser.go:330`). `dec.dependencies` is already recorded (`diff_recorder.go:39`) but not passed.
- Act-time decode uses real values: `gohcl.DecodeBody(bdy, ctx, r)` with `unknown=nil` (`callbacks.go:141`). Dependencies applied earlier have their results unmarshalled into their entity (`lifecycle.go:698-703`).
- The DependencyChange end-to-end addition (commit `ce714f4`) is the template for every layer: `entity` → `plugins/provider.go:109` → `changed.go:33` → `adapter.go:33,208-225` → `plugin.go:58,205` → `plugin_host.go:33` → `direct_plugin_host.go:134,170` → `grpc_resource_adapter.go:53` → `grpc_plugin_host.go:293,402` → `grpc_server.go:144` → `change_proto.go` → `plugin.proto` → regenerated `plugins/proto` → regenerated `plugins/mocks/mock_provider_adapter.go`.
- Docker SDK v28 has `NetworkDisconnect(ctx, networkID, containerID string, force bool) error`. The example's `client.Docker` interface (`example/plugin/plugins/docker/client/client.go:29-45`) lacks it, and the mock is regenerated with `make generate` (mockery v3.8.0).
- The nginx official image runs executable `*.sh` files in `/docker-entrypoint.d/` at start, a natural home for an `init_script` bind mount.
- `writeChangedConfig` (`example/plugin/plan_test.go:16-30`) builds config variants at test time. `applySubnetChange` (`main_test.go:111-118`) is the two-apply scenario model.

## Files examined

- `xclconfig:entity/change.go` — `Change`, `DependencyChange`; stdlib only (`entity/doc.go`).
- `xclconfig:diff/diff.go:86-128` — `diff.Resource`, `diff.Change{Path, Before, After, Unknown, Sensitive}`.
- `xclconfig:diff/path.go:9-33,88` — `StepKind`, `Step`, `Path`, builders, `String`, `MarshalJSON` (no Unmarshal, no match helpers).
- `xclconfig:diff/render.go:14,183,190` — `(known after apply)` and the sensitive marker are rendering-only.
- `xclconfig:internal/parser/parser.go:267-458` — Apply = decide → reparse → destroy (`toDestroy`) → act walk; `previousState = working` at :330; Diff = decide only; `logDecisions` :527 logs no changes.
- `xclconfig:internal/parser/callbacks.go:83-88,136-142,190-194` — the unknown eval context in decide; `decodeForDiff` vs real decode; dispatch.
- `xclconfig:internal/parser/lifecycle.go:118-192` — act `run`/`update` (Update gets the real config plus decide-read computed values; deps not passed).
- `xclconfig:internal/parser/lifecycle.go:267-369` — `decideResource`: deps :306, `withSavedValues` :315, `refresh` :318, unknown floor :325, recording per outcome.
- `xclconfig:internal/parser/lifecycle.go:408-424` — `recordPendingResource`/`changes` → `resourceChanges` (plan changes only for pending actions).
- `xclconfig:internal/parser/lifecycle.go:490-598` — `refreshed{old,configured,read}`; Read then `Changed(old, read, deps)`.
- `xclconfig:internal/parser/diff_recorder.go:25-44,83-199` — `decision` struct (no changes field); recorder methods.
- `xclconfig:internal/parser/diff_changes.go:39-480` — `resourceChanges`, `compare`, `compareSensitive`, `unknownChange`, `pathEqual` :327, `plainValue` :419, `plainCtyValue` :480.
- `xclconfig:internal/parser/diff_decode.go:23,113,253` — `decodeForDiff`, `unknownPaths`, `withSavedValues`.
- `xclconfig:internal/parser/dependencies.go:30-102` — `dependencyChanges`, sorted, Update/Replace only.
- `xclconfig:internal/parser/test_plugin.go:45-103,137-166,330,639-696` — TestPlugin records Changed deps; Update records only the ID.
- `xclconfig:internal/parser/decide_test.go:28-117`, `replace_test.go:19-257`, `diff_update_unknown_test.go:14-248` — test patterns (`runDecide`, `applyAndSave`, `SetChangedResult`, unknown assertions).
- `xclconfig:internal/test_fixtures/plugin/structs/credential.go:12-18` — sensitive `Password`/`Pin` fixture in TestPlugin.
- `xclconfig:internal/parser/events.go:47-129` — event Data is re-encoded with `EventMask`; holds no changes.
- `xclconfig:types/sensitive.go:15,90-144` — `Sensitive` marshals to `"(sensitive)"`; `RevealAny`.
- `xclconfig:internal/wire` — `wire.Marshal` writes real sensitive values (provider calls).
- `xclconfig:plugins/provider.go:19,78-109` — `ResourceProvider[T]` `Update`/`Changed` and their doc comments.
- `xclconfig:plugins/changed.go:15-53` — `DefaultChanged` JSON comparison minus meta/depends_on/disabled.
- `xclconfig:plugins/adapter.go:26-33,184-225` — `ProviderAdapter`, `TypedProviderAdapter` Update/Changed.
- `xclconfig:plugins/plugin.go:47-58,194-211`, `plugin_host.go:12-33`, `direct_plugin_host.go:97-171` — byte-level layers.
- `xclconfig:plugins/grpc_plugin_host.go:273-311,393-406`, `grpc_resource_adapter.go:49-58`, `grpc_server.go:124-165` — gRPC Update/Changed (server Update goes straight to the adapter).
- `xclconfig:plugins/change_proto.go:11-63` — enum and dependency converters; new converters go here.
- `xclconfig:plugins/plugin.proto:88-125` — `UpdateRequest` (fields 1-3), `ChangedRequest` (fields 1-5).
- `xclconfig:Makefile:1-3` (`protos`), `.mockery.yml` — generation (protoc-gen-go v1.36.11, -grpc v1.5.1; mockery v3.8.0).
- `xclconfig:plugins/testing/helpers.go:141-203` — `TestChanged`/`TestCRUDOperations` pass nil deps.
- Providers to migrate: `plugins/example/pkg/person/provider.go:18,40,107`; `e2e/fixtures/externalplugin/main.go:63,92,120,133,155`; `e2e/fixtures/inprocess/plugin.go:73,90,120,171,202`; `internal/test_fixtures/plugins/subtypeless/main.go:36,65`; `internal/parser/test_plugin.go:498,639,665`; `example/plugin/plugins/docker/resources/{container,network}.go`; `example/plugin/plugins/template/template.go`; test fakes in `config_plugin_subtypeless_test.go:44,77`, `example/prettylog/fixtures_test.go:115-170`, `internal/parser/validate_test.go:636-655`, `internal/catalog/sensitive_types_test.go:29-76`, `internal/catalog/catalog_test.go:53-152`, `registry/local_test.go:26-47`, `plugins/adapter_test.go:19-250`, `plugins/direct_plugin_host_test.go:17-38`, `plugins/changed_test.go:26-130`, `plugins/changed_sensitive_test.go:27-94`, `plugins/grpc_plugin_host_test.go:79`.
- In-process vs external comparisons: `e2e/plugin_replace_test.go:170-209` (person scenarios), `plugins/example/e2e_test.go:576-611`.
- `xclconfig:example/plugin/plugins/docker/resources/container.go:25-324` — fields, Create attaches the first network plus `NetworkConnect` for the rest, Read copies computed values only, Update no-op, Changed replaces on deps Replace or image/command/env/networks.
- `xclconfig:example/plugin/plugins/docker/resources/network.go:18-116` — Destroy = `NetworkRemove` only (`force` unused), Changed replaces on Subnet.
- `xclconfig:example/plugin/plugins/template/template.go:28-155` — Source/Destination/Variables, renders 0644, Changed replaces on Destination.
- `xclconfig:example/plugin/plugins/docker/client/client.go:29-48` — `Docker` interface; no `NetworkDisconnect`.
- `xclconfig:example/plugin/{main_test.go,plan_test.go,smoke_test.go,status_test.go,Makefile}` — scenario helpers. Tests that encode replace-on-network semantics: `container_test.go:465-507`, `main_test.go:432-454`, `plan_test.go:88-109`; count-dependent: `main_test.go:258`, "3 unchanged".
- `xclconfig:example/plugin/config/main.xcl`, `config-subnet/main.xcl` — network app, container web, template welcome.
- `xclconfig:docs/plugin-developer-guide.md:15-28,135-203,265-400,636`, `docs/plugins.md:31-190`, `docs/parser-lifecycle.md:115-140`, `docs/README.md:23,37`, `README.md:227-300`, `plugins/example/README.md:125-133`, `CHANGELOG.md:1-…` — docs carrying the signatures and the changelog style (prose paragraphs plus a `**Breaking:**` list).
- `xcl-website:src/pages/replacement.mdx:23-307` — Changed signature, DefaultChanged, deps, the Docker worked example; :195-197 says Docker `Update` is a no-op (now wrong).
- `xcl-website:src/pages/examples/plugins.mdx:20-1166` — plugin example page; :433-435 says Update is a no-op (now wrong); template section :457-520.
- `xcl-website:src/pages/diff.mdx:67-91` — action and replace-reason tables.
- `xcl-website:src/components/Nav.astro:8-30` — hard-coded nav; `package.json` build = `astro build`; `make check` = `astro check`.

## External references

- Docker Engine Go SDK v28 `client.NetworkDisconnect(ctx, networkID, containerID string, force bool)` and `NetworkInspect` (`Containers` map): the hot swap and force-detach calls.
- The nginx official image's `/docker-entrypoint.sh` runs executable `/docker-entrypoint.d/*.sh` before starting nginx (non-executable ones are skipped with a notice). This is how the example's `init_script` runs.
- `log/slog.LogValuer` and `fmt.Formatter` (stdlib): let `entity.PropertyChange` mask sensitive values when logged or printed.

## Prior plans / specs consulted

- Plan `20261008132354-4538504f-replacement-deps` (historical): the decide-then-act structure, the decision record grown from the diff recorder, the `entity` package rationale (stdlib-only, not reusing `diff.Action`), and the delivery order that keeps the repo building (contract first, then core, then examples, then docs). This plan follows the same sequencing.
- Design `replacement-and-dependency-changes.md` (source `design`): the unchanged/update/replace answer, direct-only dependency list, destroy-then-create order. It still governs; this plan only extends what `Changed`/`Update` are told.
- Spec `20261008194959-b128e508-update-property-changes`: the source of truth.

## Open assumptions

- `diff.Path`'s methods can move to `entity` unchanged and `diff` can alias the types (`type Path = entity.Path`) without breaking `diff`'s JSON output (`Path.MarshalJSON` keeps the string form). If an alias breaks the public diff API in a way callers notice, STOP and ask.
- Recomputing changes at act time from (saved, real configured) gives the same paths as the plan, apart from paths that were unknown at decide time and turned out equal to the saved value. Those are kept in the Update list with their real values so the set still matches the plan (see the assumptions log).
- JSON-normalised values (string, float64, bool, nil, `[]any`, `map[string]any`) are acceptable to plugin authors as `Before`/`After`, on both in-process and external paths.
- The nginx image used by the example executes `/docker-entrypoint.d/*.sh` only when executable. The template provider gains an optional file mode so the init script can be written executable.
- The template's `destination` change is the way to make the init-script producer "replaced" in the example scenario (it's the template's only replace rule).
- `NetworkDisconnect` on a network already removed (force-detached by the network's Destroy) returns a not-found error the container's Update can ignore.

## Drafting assumptions

### Chosen direction: one shared comparison per pass, contract-first sequencing (architecture)
- **Decision**: Option A. One real-valued run of `resourceChanges` per resource per pass produces both the plan list (masked) and the plugin list (JSON-normalised, real values). It runs in `refresh` before `Changed` at decide time and again in `update` at act time. The contract change (entity types, signatures, proto) lands first with empty lists, then the core fills them, then the examples and docs follow.
- **Key design decisions**: Changes are compared saved vs configured, as the plan does. The act-time list is recomputed from the real decode, plus decide-time unknown paths. `Update` gets `decision.dependencies` as recorded. The plan's `diff.Change` values keep their current Go shape; only the plugin copy is normalised. The changes are never added to events or logs.
- **Rejected**: Option B, two independent computations (one masked for the plan, one revealed for plugins): simpler to wire, but the two can drift apart, breaking the "same set as the plan" success metric. Option C, compute at decide and patch real values into the stored list at act time: needs per-path value resolution that duplicates the decode, and is still wrong for values from dependencies applied earlier.
- **Effort**: A Medium; B Low-Medium; C High.

### Parameter order: changes before dependencies (architecture)
- **Decision**: `Changed(ctx, old, new, changes, dependencies)` and `Update(ctx, resource, changes, dependencies)`.
- **Rationale**: Both calls read the same way. A resource's own changes come before what is happening around it.
- **Rejected**: a single `entity.Changes{Properties, Dependencies}` struct argument (one more type to learn, and it doesn't match today's dependency parameter); dependencies first (Update would read oddly).

### Container Changed rules in the example (architecture)
- **Decision**: Replace on a change within image, command, environment or init_script, or on any replaced dependency that isn't a `docker.network`. Update on any other change or changing dependency. Otherwise defer to DefaultChanged.
- **Rationale**: Implements the spec's hot swap, keep-through-rebuild and init-script-rebuild requirements with rules expressed through `PropertyChange.Within`.
- **Rejected**: replacing only when `template.*` is replaced (hard-codes the template type; the rule "anything that isn't a network" generalises).

### Conventions selected (architecture)
- **Decision**: Apply code style, top-level packages, testing and mocking, real-apply state, shared test helpers, never modifying dependencies, the example-modules gotcha, the custom-MarshalJSON gotcha, the sensitive fixture learning, structured logging and the entity glossary. Ordering-on-graph-parents applies only to the linked network and container. Drop the shared errors package, database, and HTTP/graceful-shutdown conventions.
- **Rationale**: These are the surfaces the change touches. No new error sentinel or service is introduced.
- **Rejected**: listing every convention (noise).

### Path type moves to entity, diff aliases it (discovery)
- **Decision**: `Path`, `Step`, `StepKind` and their builders move from `diff` to `entity`; `diff` keeps `type Path = entity.Path` (etc.) aliases.
- **Rationale**: The spec puts the change type in `entity`, which must stay stdlib-only. `diff/path.go` is stdlib-only, and aliases keep `diff`'s API and JSON identical.
- **Rejected**: `entity` importing `diff` (heavy dependencies into every plugin); a second, duplicate path type in `entity` (two types for one concept, with conversion code).

### Changes are compared saved vs configured, as the plan does (discovery)
- **Decision**: The changes passed to `Changed` and `Update` come from `resourceChanges(saved, configured)`, not from saved vs the post-Read bytes `Changed` already receives.
- **Rationale**: The success metric requires the plugin's set to equal the plan's set; the plan compares saved against configured.
- **Rejected**: saved vs read (includes drift and carried computed values, so it would disagree with the plan).

### Sensitive values are real but self-masking (discovery)
- **Decision**: `entity.PropertyChange` carries real `Before`/`After` values with a `Sensitive` flag, and implements `String`, `Format`, `LogValue` and `MarshalJSON` so a sensitive change prints, logs and marshals with the values hidden.
- **Rationale**: The spec requires real values for plugins and hidden values in plans, events and logs. Self-masking guards against an accidental `%v` or log in core or plugin code.
- **Rejected**: wrapping values in `types.Sensitive` (`entity` can't import `types`, and it breaks the plain-value shape); raw values with no guard (easy to leak).

### Values are JSON-normalised on every path (discovery)
- **Decision**: `Before`/`After` are always plain JSON-decoded values (string, float64, bool, nil, `[]any`, `map[string]any`), produced by a JSON round trip of the computed value for in-process plugins too.
- **Rationale**: The acceptance criteria require built-in and external plugins to be told identical changes; external plugins necessarily get JSON-decoded values.
- **Rejected**: passing Go values in-process (ints vs float64 would differ from external).

### Previously-unknown paths stay in the Update list (discovery)
- **Decision**: At act time the change list is the real-valued comparison plus any path that was unknown at decide time (with its real before/after), even if it resolved to the saved value.
- **Rationale**: Keeps the set of paths equal to the plan's, per the success metric, with only values differing.
- **Rejected**: plain recompute (the set could shrink and disagree with the plan).

### Init script runs through nginx's entrypoint directory (discovery)
- **Decision**: The container bind-mounts `init_script` read-only at `/docker-entrypoint.d/90-xcl-init.sh`; the template gains an optional `mode` (for example `"0755"`) so the rendered script is executable.
- **Rationale**: The example's nginx image already runs executable scripts from that directory, so no command or entrypoint override is needed.
- **Rejected**: overriding the container command (changes nginx start-up); mounting without running it (not really an init script).

### Container maps a replaced network dependency to its attachment by name (discovery)
- **Decision**: In the container's Update, a dependency `docker.network.<name>` that is replaced is matched to the `network` attachment whose `name` equals the address's last segment, and that attachment is reconnected.
- **Rationale**: The example's Docker network name is its block name (`network.go` Create uses `n.Meta.Name`), so the address names it without any extra call.
- **Rejected**: inspecting the container to find missing networks (an extra call the success metric rules out).

### A dedicated recording provider proves in-process/external parity (components)
- **Decision**: Add a small recording provider to both the in-process and external e2e fixtures. It writes what `Changed` and `Update` were told to a file named in its configuration.
- **Rationale**: The acceptance criterion needs the same edit applied through both hosts, with identical told values. No existing fixture can report its call arguments across a process boundary.
- **Rejected**: relying only on converter round-trip unit tests (they don't exercise a real apply through both hosts); reusing the person provider (it would need test-only recording behaviour added to a public example).

### Proto values as JSON bytes, empty meaning absent (data_structures)
- **Decision**: `PropertyChange.before`/`after` are JSON bytes; empty bytes mean absent (nil). Path steps are a structured `PathStep` message.
- **Rationale**: Matches `entity_data`. Needs no well-known-type imports. nil and absent are the same thing in `PropertyChange`, so no extra presence flag is needed.
- **Rejected**: `google.protobuf.Value` (new import, no precedent); the string path form (the spec asks for structure, and `Path` has no UnmarshalJSON).

### Template mode attribute (data_structures)
- **Decision**: The template gains optional `mode` (octal string, default `"0644"`).
- **Rationale**: The nginx entrypoint runs only executable scripts. A string keeps the configuration readable (`mode = "0755"`).
- **Rejected**: making every `.sh` destination executable (hidden magic); a number attribute (octal is unreadable as a decimal number).

### Example scenario variants live in config directories (tasks)
- **Decision**: The new scenarios use committed variant dirs (`config-swap/`, `config-init/`, `config-remove/`, `config-init-content/`, `config-dangling/`), shared by the Makefile walkthroughs and the tests, following the existing `config-subnet/`.
- **Rationale**: The Makefile walkthroughs can't rewrite configs at run time. One copy per variant keeps the docs, the Makefile and the tests in step.
- **Rejected**: `writeChangedConfig` string rewrites (tests only, so the walkthroughs would need duplicates).

### Init script checked by mount, not by fetching from the container (tasks)
- **Decision**: The scenario test asserts that the bind mount and the executable script exist, rather than fetching a file the script wrote over HTTP.
- **Rationale**: The container network is not reachable from every test host (for example Docker Desktop on macOS).
- **Rejected**: an HTTP fetch (flaky across hosts); adding exec to the client interface (extra surface just for a test).

### The recorder provider is external-only in the fixtures, with an in-test in-process wrapper (tasks)
- **Decision**: The recorder is registered in the external e2e fixture binary, and wrapped in-process by the test, as the person scenarios do.
- **Rationale**: Avoids a type clash in the mixed e2e config that loads both fixtures, and follows the existing parity pattern.
- **Rejected**: registering it in both fixture plugins (the same type registered twice in one registry).

## Rehydration cues

- `spektacular spec file read 20261008194959-b128e508-update-property-changes`
- `spektacular design read --data '{"source":"design","path":"replacement-and-dependency-changes.md"}'`
- `spektacular plan file read 20261008132354-4538504f-replacement-deps plan`
- Re-read: `internal/parser/lifecycle.go:118-192,267-369,408-424,514-598`, `internal/parser/diff_changes.go:39-200,400-500`, `internal/parser/diff_recorder.go`, `diff/path.go`, `diff/diff.go:86-128`, `entity/change.go`, `plugins/adapter.go`, `plugins/grpc_plugin_host.go:273-311`, `plugins/grpc_server.go:124-165`, `plugins/change_proto.go`, `plugins/plugin.proto:88-125`, `example/plugin/plugins/docker/resources/container.go`, `network.go`, `example/plugin/plugins/template/template.go`, `example/plugin/main_test.go`, `plan_test.go`.
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`
