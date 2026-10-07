---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Research: 20261007105731-2388b579-diff

## Alternatives considered and rejected

- **Separate diff comparator (no walk)**: compare parsed config against saved state directly, then call Read/Changed in a loop. Rejected: spec Technical Approach requires reusing the apply walk and provider read/change checks so diff and apply cannot disagree; the walk is also the only place bodies are decoded in dependency order with references resolved (`internal/parser/callbacks.go:38-199`, `internal/parser/parser.go:1320-1367`).
- **Dry-run provider adapter**: wrap each `plugins.ProviderAdapter` so Create/Update/Destroy are recorded but not executed, and run `Parser.Apply` unchanged. Rejected: apply's removal phase and rebuild path still emit destroy/create/update lifecycle events (`internal/parser/lifecycle.go:139-157, 254-270, 292-311`; `callbacks.go:289-310`), which the design forbids ("No create, update or destroy events fire"); Create returns no computed values so dependents would decode zero values instead of unknowns; the destroyer saves state through the store (`internal/parser/destroy.go:136`), violating "Diff changes nothing".
- **Decode with unknown values directly**: put `cty.UnknownVal` in the eval context and let `gohcl.DecodeBody` decode. Rejected: gocty cannot write an unknown into a Go field, decode fails — the design calls this out explicitly ("the apply walk decodes a body straight into a Go struct, which cannot hold an unknown value").
- **Drop unknown attributes from the body before decode**: rejected, required attributes would then fail decode with "missing required argument".
- **Compare saved vs configured with `changedConfiguredValues`** (`internal/parser/configured_check.go:30`): rejected as-is; it pairs list elements by key fields (`pairElements`, `computed.go:233`), reports whole lists on length change, returns string paths without values, and skips reference-set fields. The design wants strict by-index comparison, structured paths, before/after values. Its walking shape (structFields / blockElement / isComputed) is reused.

## Chosen approach — evidence

- Diff mode of the existing walk: `Parser.walk` builds the create DAG and runs `walkCallback` per vertex (`parser.go:1320-1367`); `resourceLifecycle.run` already dispatches create / read / rebuild on saved status (`lifecycle.go:85-125`); `read` performs exactly the two calls the design requires, `Read(saved, configured-with-carried-computed)` then `Changed(saved, read)` (`lifecycle.go:167-246`), with `plugins.ErrNotFound` handled at `lifecycle.go:204`. Splitting `read` at line 248 (before Update) gives a shared refresh used by apply and diff.
- Removal set: `removedResources(current, previous)` (`parser.go:413-433`) is the rule Apply's removal phase uses; destroyer skips disabled and provider-less entities (`callbacks.go:239`), so delete is filtered the same way.
- Provider-less entities: `handledWithoutProvider` (`lifecycle.go:453`) covers builtins and registered config-only types; disabled ones are skipped in `walkCallback` (`callbacks.go:62-118`).
- Parse/validate/state load identical to Apply: `parseAndValidate` (`parser.go:452-576`), `ErrEmptyConfiguration` check (`parser.go:269`).
- Context building: `buildContextForResource` (`context.go:15-226`) converts each linked entity with `convert.GoToCtyValue`; this is where computed fields of pending (create/replace) resources become `cty.UnknownVal`.
- Computed fields at any depth: `computedFields`, `isComputed`, `structFields`, `blockElement` (`computed.go:47-170`).
- Sensitive leaves: `isSensitiveType` (`computed.go:31`), `types.SensitiveValue.RevealAny` (`types/sensitive.go`).
- Operation wrapper: `Config.run(operation, work)` (`config.go:497-551`) gives start/finish events, plugin activation (`withPlugins`, `config.go:559`), logger; `events.Operation*` constants (`events/events.go:48-61`) gain `OperationDiff = "diff"`.
- Core log emission pattern: `logger.New(emit, events.Event{Source: events.SourceCore, Operation: ...})` (`config.go:362`).
- Provider error wrapping: `callProvider` returns `reportedError{"%s failed for %s: %w"}` naming the resource ID (`lifecycle.go:411`); `walkCallback` wraps in `ParserError` with `Cause` (`callbacks.go:177-185`); `ConfigError.Unwrap() []error` (`errors/config_error.go:28`) keeps `errors.Is` working.
- Test provider: `parser.TestPlugin` records Read/Create/Update/Destroy/Changed calls and supports `SetReadNotFound`, `SetReadError`, `SetChangedError`, `SetReadObserved` (drift via computed `observed`), `SetChangedResult`, `SetCreateError`, `SetDestroyError`, `SetLogOnRead` (`internal/parser/test_plugin.go:104-343`); types container, sidecar, network, credential (sensitive `password`, `pin`), template.
- Unknown fixture: `internal/test_fixtures/config/lifecycle/computed_ref/computed_ref.xcl` (container network name = `resource.network.one.provider_id`, computed).

## Files examined

- config.go:310-375 — Apply: parser built from Config options, state saved; Diff must skip save and leave `c.entities`.
- config.go:497-574 — run/withPlugins wrapper; diff runs through it with OperationDiff.
- internal/parser/parser.go:263-363 — Parser.Apply: parse, empty check, removal phase, walk, partial state.
- internal/parser/parser.go:413-433 — removedResources rule.
- internal/parser/parser.go:452-576 — parseAndValidate loads previous state (empty when no store).
- internal/parser/parser.go:1320-1367 — walk: DAG build, lifecycle construction, operation errors emitted as OperationApply.
- internal/parser/callbacks.go:38-199 — walkCallback: context, disabled, defaults.Set, gohcl.DecodeBody, module variables, lifecycle.apply, output conversion.
- internal/parser/lifecycle.go:55-314 — apply/run/create/read/rebuild; read = carry computed + Read + Changed + Update.
- internal/parser/lifecycle.go:392-423 — callProvider emits start/error lifecycle events and decodes result into r.
- internal/parser/context.go:15-226 — eval context from links; GoToCtyValue per linked entity.
- internal/parser/computed.go:31-301 — field helpers (structFields, isComputed, blockElement, copyComputed, pairElements).
- internal/parser/configured_check.go:1-206 — structure of a saved/configured field walk.
- internal/parser/events.go:18-212 — emit, emitLifecycle, providerContext.
- internal/parser/progress.go:22-44 — applyProgress with mutex; callbacks run concurrently.
- internal/parser/test_plugin.go — TestPlugin call recording and error injection.
- internal/test_fixtures/plugin/structs/{container,network,credential}.go — fixture types (computed fields, nested blocks, maps, sensitive).
- plugins/changed.go — DefaultChanged compares whole JSON minus meta, so computed drift reports changed.
- types/sensitive.go — Sensitive[T] leaf, RevealAny, MarshalJSON writes marker.
- events/events.go:48-61 — operation constants.
- errors/config_error.go, errors/parser_error.go — Unwrap chain.
- e2e/fixtures/inprocess/plugin.go, e2e/plugin_helpers_test.go, e2e/testdata/plugin — e2e plugin fixture and apply helper; e2e/testdata/kube uses registered types only.
- config_destroy_test.go:1-80 — root-package fixture pattern: registry + TestPlugin + FileStateStore + event recorder.

## External references

- go-cty `Value.IsWhollyKnown`, `cty.Walk`, `cty.Transform` (forked in internal/cty) — used to find and replace unknowns inside evaluated attribute values.
- HCL `hclsyntax.LiteralValueExpr` (internal/xcl/hclsyntax) — substitute an evaluated, placeholder-filled value into a copied body so gohcl can decode.

## Prior plans / specs consulted

- Plan list only (`spektacular plan file list`): 20260918165700-provider-lifecycle-read and 20261003153421-9fa72edd-masking are the origins of the read/changed path and masking; current code was used as the source of truth, not the plans.
- Spec 20261007111826-cf3b66d8-diff-rendering-and-docs (sibling, same epic): owns Render, Highlight, website docs. Not planned here.
- Design `design:config-diff.md` (resolved, binding): entry point, actions, change rules, unknown and sensitive rules, types, JSON.

## Open assumptions

- Registered config-only types and builtins are decoded in diff mode the same way as in apply; their unknown paths are tracked so dependents still see unknowns.
- `gohcl` decodes a `LiteralValueExpr` holding a type-appropriate placeholder (empty string, zero, false, empty collection) without error; a dynamic-typed unknown is substituted with null. If this proves false for some field kinds, STOP and ask.
- Output and module `variables` values are `cty.Value` and can hold unknowns directly; `convertOutputValue` must be skipped for values that are not wholly known.
- A `disabled` expression that evaluates to unknown fails the diff with the existing decode error (not handled specially).
- Saved state never holds redacted Sensitive values (WithStateMask requires a reversible masker), so saved-vs-configured comparison of Sensitive leaves is meaningful.

## Drafting assumptions

### Chosen direction: diff mode of the apply walk (architecture)
- **Decision**: Add `Parser.Diff` reusing `parseAndValidate`, `removedResources` and `Parser.walk`, with a lifecycle mode; split `resourceLifecycle.read` into a shared `refresh` used by apply and diff; track pending resources and unknown paths in a mutex-guarded recorder; decode through `decodeForDiff` that substitutes placeholders for unknowns in a copied body.
- **Rationale**: Spec Technical Approach requires reusing the walk and provider checks; the design fixes the provider calls, actions and unknown handling; a shared refresh makes apply and diff decide the same way.
- **Rejected**: dry-run provider adapter (emits create/update/destroy events, saves via destroyer, no unknowns); standalone comparator (separate path, violates technical approach); decoding unknowns directly (gocty cannot hold them).

### Diff options shape (architecture)
- **Decision**: `type Option func(*Options)`, exported `Options struct{ RevealSensitive bool }`, `func RevealSensitive() Option`, and `func NewOptions(options ...Option) Options` for the root package to resolve them.
- **Rationale**: The design names only `diff.RevealSensitive()`; the root package must read the setting across the package boundary, and an exported struct is the simplest standard-library-style shape.
- **Rejected**: unexported settings with accessor functions (more API for one flag).

### Changed-count method name (architecture)
- **Decision**: `func (d *Diff) Changed() int` returns `Create + Update + Replace + Delete`.
- **Rationale**: The design asks for a method on `Diff`, unnamed.
- **Rejected**: a stored field (design forbids), a method on `Summary` (design says on `Diff`).

### Which fields a create lists (architecture)
- **Decision**: a create lists each non-computed top-level field that is set in the configuration body or holds a non-zero value after decoding (so defaults show), in declaration order.
- **Rationale**: "each configured top-level field" — listing every zero-valued optional field would be noise; defaults are part of what is created.
- **Rejected**: every field (noise); only attributes present in the body (hides defaults).

### Unknown entries keep the saved value on update and replace (architecture)
- **Decision**: an unknown entry on `update`/`replace` carries `Before` (the saved value) and `Unknown: true`, no `After`; on `create` it has neither.
- **Rationale**: The design only says "no after"; the before value is known and useful.
- **Rejected**: dropping before (loses information).

### Sensitive leaves inside whole values (architecture)
- **Decision**: an added, removed or created whole value that contains a sensitive leaf is split into its children until the sensitive leaf stands alone, mirroring the unknown rule.
- **Rationale**: The success metric forbids any sensitive value in an unrevealed result; masking a single leaf keeps the rest concrete.
- **Rejected**: marking the whole element sensitive (hides non-secret values); writing the marker string inside the value (a value would no longer be the real value).

### Nil and empty collections compare equal (architecture)
- **Decision**: the comparator treats a nil and an empty list or map as equal.
- **Rationale**: saved state round-trips through JSON with `omitempty`, so `[]` configured vs nil saved would otherwise be a spurious change.
- **Rejected**: strict DeepEqual.

### cty.Value fields are leaves (architecture)
- **Decision**: a `cty.Value` field on a provider-backed entity is compared as a leaf (`Equals`) and reported via its JSON form converted to plain Go values.
- **Rationale**: such fields are rare on provider types; descending into them is out of scope.
- **Rejected**: structured descent into cty values.

### Path JSON is marshal-only (architecture)
- **Decision**: `Path` implements `MarshalJSON` (string form) but not `UnmarshalJSON`.
- **Rationale**: the design specifies marshalling; reading JSON back is not a requirement, and parsing path strings adds surface.
- **Rejected**: round-trip parsing.

### Unknown disabled expressions (architecture)
- **Decision**: a `disabled` expression that evaluates to unknown fails the diff with the existing "unable to decode disabled expression" error naming the resource.
- **Rationale**: whether the resource exists after apply cannot be known; rare case, explicit failure beats a wrong answer.
- **Rejected**: treating it as enabled or disabled silently.

### Completion log line (architecture)
- **Decision**: on success the diff emits one core debug log, "diff complete", with create/update/replace/delete/unchanged counts.
- **Rationale**: gives the "diff writes logs" criterion a core-owned log independent of provider behaviour; provider logs during Read also flow as they do in apply.
- **Rejected**: relying only on provider logs.

### Conventions selected (architecture)
- **Decision**: apply code style, project structure, shared-public-types placement, testing & mocking, state from real apply, shared test helpers, ordering assertions, no repo-inspecting tests, never-modify-dependencies (in-repo forks), structured logging, entity glossary. Dropped: database & external services (no database), patterns & architecture beyond context passing (no services/handlers), dependencies (no new dependency), shared errors package (no new error type or sentinel is introduced).
- **Rationale**: these are the conventions the touched surfaces (public package, parser walk, tests) bear on.
- **Rejected**: listing all conventions.

### Diff recorder is a new parser component (components)
- **Decision**: a dedicated diff recorder rather than extending `applyProgress`.
- **Rationale**: `applyProgress` builds partial state for apply resumption; a diff has no partial result and records different things (pending set, unknown paths, changes).
- **Rejected**: overloading `applyProgress` with diff fields (mixes two responsibilities).

### Milestone order puts unknowns last (milestones)
- **Decision**: actions (M2) and value changes (M3) land before unknown handling (M4); until M4 a value that references a computed field of a resource being created is decoded from the empty pre-create value.
- **Rationale**: unknown handling is the riskiest part (context hook, decoder, propagation) and benefits from the comparator and recorder already existing; each milestone remains a coherent, tested increment.
- **Rejected**: unknowns first (needs the recorder and comparator anyway); one large milestone (no intermediate validation).

### Task split and executors (tasks)
- **Decision**: eight agent tasks, all in the colocated `xclconfig` repo; no human tasks (nothing needs secrets, access or action outside the repo). Reviews are Testing Approach manual items.
- **Rationale**: every change is Go code and tests in this repository; website docs belong to the sibling spec.
- **Rejected**: a task in `xcl-website` (sibling spec's scope); a CHANGELOG task (left to the implement workflow's changelog step).

### Fixture placement (tasks)
- **Decision**: diff configuration fixtures live under `internal/test_fixtures/config/diff/`; test state comes from real applies with `parser.TestPlugin`.
- **Rationale**: matches the existing fixture layout and the state-from-real-apply convention.
- **Rejected**: hand-written state files (convention forbids).

## Rehydration cues

- `spektacular design read --data '{"source":"design","path":"config-diff.md"}'`
- `spektacular spec file read 20261007105731-2388b579-diff`
- Re-read internal/parser/lifecycle.go (read path), internal/parser/callbacks.go (walkCallback), internal/parser/context.go, internal/parser/parser.go Apply/walk, config.go Apply/run.
- `spektacular knowledge always-applied --tier repo --filter xclconfig`
