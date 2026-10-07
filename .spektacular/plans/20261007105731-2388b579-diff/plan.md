---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Plan: 20261007105731-2388b579-diff

<!-- Metadata -->
<!-- Created: 2026-10-07T13:25:43Z -->
<!-- Commit: 3ffcf638b2dc89800370e9a88ed5489418cb512c -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

This plan adds `Config.Diff`, which reports what an apply of a configuration would do — which provider-backed resources would be created, updated, replaced or deleted and exactly which values would change — without creating, changing, destroying or saving anything. It reuses the apply walk and the providers' read and change checks so a diff and the apply that follows cannot disagree, and returns a plain result in a new public `diff` package that code can inspect directly or encode as JSON. Applications and tools built on xcl benefit: they can show or act on pending changes, including drift in real infrastructure, before running an apply, with sensitive values hidden unless explicitly revealed.

## Conventions

- **Go code style (gofmt, vet, `any`, descriptive names)** — all new code in `diff/` and `internal/parser`; `diff.Change.Before`/`After` are `any`.
- **Public library packages live at the module's top level** — the result types go in `github.com/jumppad-labs/xcl/diff`, not under `/pkg` or the root package.
- **Public types shared with the parser live outside the root package (`architecture/shared-public-types-live-in-types`)** — the parser builds `diff.Diff`, so the types cannot live in package `xcl`; the new `diff` package must import neither the root package nor `internal/parser`.
- **Testing & mocking: testify `require`, no table-driven tests, positive and negative cases in separate functions, tests next to the code they test** — every task's tests: `diff/*_test.go` for types, `internal/parser/*diff*_test.go` for the walk and comparator, `config_diff*_test.go` for `Config.Diff`, `e2e/diff_test.go` for the suite.
- **Generate test state with a real apply** — every diff test that needs saved state (created, updated, failed, destroy_failed, removed) produces it by applying with `parser.TestPlugin` (using `SetCreateError` / `SetDestroyError` for failed states), never a hand-written state file.
- **Shared test helpers live in `internal/testutil`** — a helper needed by both root and e2e tests (e.g. reading a state file's bytes) goes there; helpers used by one package stay local.
- **Assert ordering on graph parents, not provider call order** — diff tests assert on the sorted result and on the set of recorded provider calls, never on call order between unlinked resources.
- **Never write tests that inspect repository files** — no test reads docs or source to check the diff feature; behaviour only.
- **Never modify dependency packages; `internal/xcl` and `internal/cty` are in-repo forks under MPL** — the plan uses `hclsyntax.LiteralValueExpr` and cty APIs as they are; if a change there becomes necessary, keep the MPL header, add the Jumppad modifications line and record it in `UPSTREAM.md`.
- **Include proper logging with structured logs** — diff emits its completion summary as a structured debug log through the core logger, with counts as key/value pairs.
- **Entity is the shared vocabulary (glossary)** — new internal code and doc comments speak of entities; only provider-backed entities appear in a diff.

## Architecture & Design Decisions

`Config.Diff(paths []string, options ...diff.Option) (*diff.Diff, error)` is a diff mode of the existing apply walk, not a second comparison path. It runs through `Config.run` as the new operation `events.OperationDiff` ("diff"), so it gets the same start/finish events, plugin activation, logger and panic handling as `Validate`, `Apply` and `Destroy` (`config.go:497-574`). It builds a parser exactly as `Apply` does and calls a new `Parser.Diff(ctx, settings, paths...)`, which reuses `parseAndValidate` (same failures, same previous state: the store's, or empty without one), the `ErrEmptyConfiguration` check, `removedResources` for deletes, and `Parser.walk` for everything still configured. `Config.Diff` never saves state and never assigns `c.entities`; `Parser.Diff` never builds a `destroyer`, so nothing can reach the store or a provider's Create, Update or Destroy. The result types live in a new top-level public package `github.com/jumppad-labs/xcl/diff` (project-structure convention), which imports nothing from the root package or `internal/parser`; the parser imports it to build results, the root package re-exposes nothing beyond the method. Every public name says "diff", never "plan".

The walk gains a mode carried on `resourceLifecycle`. In apply mode nothing changes. In diff mode `walkCallback` calls `lifecycle.diff(r)` instead of `lifecycle.apply(r)`, provider-less entities (builtins, registered config-only types) emit no create event and are not recorded, and walk-level error events carry `OperationDiff`. To keep apply and diff from disagreeing, `resourceLifecycle.read` (`lifecycle.go:167-273`) is split at the point where it decides to update: a shared `refresh(r, old, adapter)` carries the saved computed values, calls `Read(saved, configured)` and `Changed(saved, read)` with their usual lifecycle events, and reports not-found / changed / unchanged. Apply's `read` becomes refresh followed by create (not found) or Update (changed), byte-for-byte the same behaviour; diff records create (not found), update (changed) or unchanged. Diff's action choice mirrors `run` (`lifecycle.go:85-125`): no saved entry → create; saved `created`/`updated` → refresh; any other saved status → replace, with no provider call. Deletes are `removedResources` filtered to entities the destroyer would hand to a provider (not disabled, not `handledWithoutProvider`). A provider error from Read or Changed stops the walk; `Parser.Diff` returns the walk's `ConfigError`, whose message names the resource (`"read failed for <address>: …"`) and which still unwraps to the provider's error; there is never a partial result. A cancelled context returns its error the same way.

Unknown values are solved in two places. First, a resource the diff will create or replace (including one whose Read reported not found) is marked *pending* in a mutex-guarded diff recorder before its dependents run — the DAG guarantees dependents are visited after their parents finish. `buildContextForResource` takes an optional unknown-value hook (nil in apply mode, so apply is unchanged): when it converts a linked entity to cty it replaces that entity's computed fields (at any depth, via `computedFields`/`isComputed`) with `cty.UnknownVal` of their type if the entity is pending, and replaces any path already recorded as unknown on that entity, so unknowns propagate through outputs, module variables and registered types. Second, because gohcl cannot decode an unknown into a Go field, diff mode decodes a provider-backed or registered entity's body through `decodeForDiff`: it walks the body's attributes and nested blocks, evaluates each attribute against the context, records the `diff.Path` of every unknown inside the value (splitting until each unknown stands alone), substitutes a type-appropriate placeholder for each unknown, and decodes a shallow copy of the body in which those attributes are `LiteralValueExpr`s — the parsed body is never mutated. Builtin outputs and module variables hold `cty.Value` and keep unknowns natively; `convertOutputValue` is skipped for a value that is not wholly known. An entity in saved state that has any unknown path gets no provider call and is reported as `update`. Unknown detection is by evaluation, never by marks, so the cty gotcha that unknown function arguments drop marks does not affect sensitivity, which is decided by Go type (`types.Sensitive`).

Changes are computed in `internal/parser` (it owns the field helpers in `computed.go`) by a new comparator that walks the saved copy and the configured copy together in field declaration order, skipping computed fields: leaves by value (nil and empty collections equal), nested blocks by descending, lists strictly by index, maps by key, an added element or key as one entry with only `after`, a removed one with only `before`. The configured copy is snapshotted before the provider Read touches it, so the comparison is saved-vs-configured as the design requires and drift shows as an update with no changes. `create` lists each configured top-level field with only `after`; `delete` lists nothing. Values are converted to plain Go values keyed by xcl field names (`map[string]any`, `[]any`, scalars), so `json.Marshal` produces the design's JSON. A `types.Sensitive` leaf, or any value inside one, becomes an entry with `Sensitive: true` and no values unless `diff.RevealSensitive()` was given, in which case both are filled from `RevealAny()`; a whole added/removed/created value that contains a sensitive leaf is split into its children so the sensitive leaf stands alone, so no revealed or wrapped secret can reach the result. Unknown paths override comparison: the entry gets `Unknown: true`, no `after`, and the saved value as `before` on update/replace. The diff recorder sorts resources by address and fills `Summary` (including `Unchanged` for provider-backed resources that would not change); `Diff.Changed()` sums the four action counts. A debug log line through the core logger reports the counts when the diff completes. See `research.md#alternatives-considered-and-rejected` for why a dry-run provider adapter, a standalone comparator, and decoding unknowns directly were rejected.

## Component Breakdown

- **`diff` public package (new)** — owns the result types (`Diff`, `Summary`, `Resource`, `Change`, `Action` and its four constants, `Path`, `Step`, `StepKind`), `Path.String` and its JSON marshalling, `Diff.Changed()`, and the options (`Option`, `Options`, `RevealSensitive`, `NewOptions`). It is plain data with no dependency on the root package or the parser, so applications import it to inspect a result and the parser imports it to build one. The sibling rendering spec adds `Render` to this package later.
- **`Config.Diff` (new, root package)** — the public entry point. Validates paths, resolves the options, builds a parser with the same options `Apply` uses, and runs `Parser.Diff` inside `Config.run` as the `diff` operation. It never saves state and never changes what `Config.Entities()` returns.
- **`events.OperationDiff` (new constant)** — the operation name on the diff's start, finish, walk-error and core log events.
- **`Parser.Diff` (new, parser)** — the diff counterpart of `Parser.Apply`: parse and validate, empty-configuration check, collect deletes from the removed set, walk the remaining configuration in diff mode, and hand the recorder's sorted result back. Builds no destroyer and writes nothing.
- **`Parser.walk` (changed)** — takes the operation name and the lifecycle mode, so the same DAG build, reduction, validation and walker serve both apply and diff, and walk-level error events carry the right operation.
- **`walkCallback` (changed)** — in diff mode: decodes provider-backed and registered entities through the diff decoder, skips the provider-less create event, skips output conversion for values not wholly known, and calls the lifecycle's diff step instead of its apply step. Apply mode is unchanged.
- **`resourceLifecycle` (changed)** — gains a mode and a reference to the diff recorder. Its `read` path is split into a shared refresh (carry computed values, `Read`, `Changed`, with their events) used by both apply's read-then-update and the new diff step; the diff step chooses create / refresh / replace from the saved status exactly as `run` does and records the outcome, never calling Create, Update or Destroy.
- **Diff recorder (new, parser)** — concurrency-safe store of the walk's diff outcome: which entities are pending (to be created or replaced, so their computed values are unknown), the unknown paths recorded for each entity, each changed resource with its action and changes, and the unchanged count. Produces the final `diff.Diff`, sorted by address, with the summary.
- **Unknown-aware context building (changed `buildContextForResource`)** — accepts an optional unknown-value hook from the recorder; when present, a linked entity's cty value has its computed fields replaced by unknowns if the entity is pending, and its recorded unknown paths replaced by unknowns. Nil in apply mode.
- **Diff decoder (new, parser)** — evaluates an entity's body against its context, records every unknown path (split until each unknown stands alone), substitutes placeholders into a copy of the body, and decodes the copy into the entity, leaving the parsed body untouched.
- **Change comparator (new, parser)** — computes an entity's `[]diff.Change` for create, update and replace from the saved and configured copies and the unknown paths, applying the design's rules for leaves, blocks, lists, maps, added and removed elements, computed fields, unknowns and sensitive values, and converting values to plain Go values keyed by xcl names.
- **Test fixtures (changed)** — new configuration fixtures under the parser's test fixtures for the diff scenarios (create, update, delete, replace, unknown reference, sensitive change, summary mix); `parser.TestPlugin` is reused as the recording test provider without change unless a scenario needs a new knob.

## Data Structures & Interfaces

**Public, package `github.com/jumppad-labs/xcl/diff`** — exactly the shapes the referenced design fixes, plus the option plumbing:

```go
type Action string // ActionCreate "create", ActionUpdate "update", ActionReplace "replace", ActionDelete "delete"

type Diff struct {
	Summary   Summary    `json:"summary"`
	Resources []Resource `json:"resources"` // sorted by address; unchanged omitted
}
func (d *Diff) Changed() int // Create + Update + Replace + Delete

type Summary struct{ Create, Update, Replace, Delete, Unchanged int } // json: create, update, replace, delete, unchanged

type Resource struct {
	Address string   `json:"address"`
	Action  Action   `json:"action"`
	Changes []Change `json:"changes,omitempty"`
}

type Change struct {
	Path      Path `json:"path"`
	Before    any  `json:"before,omitempty"` // absent: added, or sensitive
	After     any  `json:"after,omitempty"`  // absent: removed, unknown, or sensitive
	Unknown   bool `json:"unknown,omitempty"`
	Sensitive bool `json:"sensitive,omitempty"`
}

type Path []Step
func (p Path) String() string               // image, ports[0].host, env["LOG_LEVEL"]
func (p Path) MarshalJSON() ([]byte, error) // the string form

type StepKind int // StepAttribute, StepIndex, StepKey
type Step struct {
	Kind      StepKind
	Attribute string
	Index     int
	Key       string
}

type Options struct{ RevealSensitive bool }
type Option func(*Options)
func RevealSensitive() Option
func NewOptions(options ...Option) Options
```

`Diff` is the contract between xcl and its callers: its Go fields are the programmatic interface, its `json.Marshal` output is the machine-readable format. `Before`/`After` hold plain Go values only — strings, numbers, bools, `[]any`, `map[string]any` keyed by xcl field names — never a `types.Sensitive` wrapper or a `cty.Value`.

**Public, root package**

```go
func (c *Config) Diff(paths []string, options ...diff.Option) (*diff.Diff, error)
```

**Public, package `events`**: `const OperationDiff = "diff"`.

**Internal, `internal/parser`** — contracts between the walk components (names indicative):

```go
func (p *Parser) Diff(ctx context.Context, options diff.Options, paths ...string) (*diff.Diff, error)

type walkMode int // walkApply, walkDiff

// diffRecorder: concurrency-safe, one per diff walk
type diffRecorder struct { /* mutex, pending set, unknown paths per entity, resources, unchanged */ }
func (r *diffRecorder) markPending(id string)
func (r *diffRecorder) recordUnknown(id string, paths []diff.Path)
func (r *diffRecorder) record(resource diff.Resource)
func (r *diffRecorder) recordUnchanged()
func (r *diffRecorder) result() *diff.Diff // sorted, with summary

// unknownValues is the optional hook buildContextForResource consults; nil in apply
type unknownValues interface {
	contextValue(entity any, value cty.Value) cty.Value
}

// refreshOutcome is what the shared refresh step reports to apply and diff
type refreshOutcome int // refreshNotFound, refreshChanged, refreshUnchanged

func decodeForDiff(body *hclsyntax.Body, ctx *hcl.EvalContext, entity any) ([]diff.Path, hcl.Diagnostics)
func resourceChanges(action diff.Action, saved, configured any, body *hclsyntax.Body, unknown []diff.Path, reveal bool) []diff.Change
```

No serialization boundary changes: saved state, the plugin wire format and event payloads are untouched.

## Implementation Detail

**One walk, two modes.** The plan introduces a mode on the existing walk rather than a parallel pipeline. A reader of the parser sees `Apply` and `Diff` side by side as two thin entry points over the same parse-validate-walk sequence; inside the walk the only branches on mode are where the design requires different behaviour — which lifecycle step runs, how a body is decoded, and which events fire. Everything else (DAG build, reduction, validation, disabled handling, defaults, module variables, context building) is shared code, so a fix to how apply resolves a reference automatically applies to diff.

**Refresh as the shared heart of the read path.** Today the read path is one long function that reads, compares and updates. It is split so that the reading and comparing half is a named step with a small outcome (not found, changed, unchanged) and its own events. Apply's read path reads as "refresh, then act on the outcome"; diff's reads as "refresh, then record the outcome". This refactor is behaviour-preserving for apply and is landed first, protected by the existing lifecycle tests, before any diff behaviour is added.

**A recorder instead of results threaded through callbacks.** The walk's callbacks already run concurrently and report through a shared progress object; the diff follows the same pattern with its own mutex-guarded recorder. It carries the two pieces of cross-resource knowledge diff needs — which entities are pending (so their computed values are unknown) and which paths on each entity are unknown — and accumulates the per-resource result. DAG order guarantees a parent's entry is recorded before any child consults it, the same guarantee apply relies on for computed values.

**Unknowns as a context concern plus a decode concern.** Unknowns enter only through the evaluation context (an optional hook, absent in apply) and leave only through the diff decoder, which turns them into recorded paths and placeholders before handing a body copy to the ordinary decoder. Nothing downstream — providers, the comparator's value conversion, state — ever sees an unknown cty value. The parsed bodies are never mutated, so a diff followed by an apply on the same parser state is not a concern, and each operation builds its own parser anyway.

**A comparator modelled on the configured-value check.** The change comparator follows the structure of the existing configured-value check — walk struct fields by xcl name in declaration order, skip computed fields, treat sensitive values as leaves, descend into blocks — but with the design's rules: strictly positional lists, keyed maps, structured paths, before/after values, unknown and sensitive overrides. It is a pure function of two typed values plus the unknown paths and the reveal flag, so it is unit-tested directly without a walk.

**Public package UX.** Application code reads a diff with no xcl internals: `d.Summary.Create`, `for _, r := range d.Resources { r.Address; r.Action; for _, c := range r.Changes { c.Path.String(); c.Before; c.After } }`, or `json.Marshal(d)`. Option handling follows the functional-options pattern already used by `NewConfig`. Doc comments on every exported name describe behaviour in "diff" terms; nothing in the public surface mentions "plan".

**Patterns followed.** Operation wrapper (`Config.run`), core structured logging through the event logger, error wrapping that names the resource and preserves `errors.Is`, test state produced by real applies with the recording test plugin, tests located next to the code they test, e2e tests that use only public packages.

## Dependencies

- **Design `config-diff.md` from the `design` source** — the settled entry point, actions, change rules, unknown and sensitive rules, result types and JSON format this plan implements. Binding; no changes.
- **`internal/parser` walk, lifecycle and context building** — the apply machinery diff reuses; changes: walk takes an operation and mode, the read path is split into a shared refresh, context building takes an optional unknown-value hook, the walk callback branches for diff mode.
- **`internal/xcl/hclsyntax` and `internal/xcl/gohcl` (in-repo HCL fork)** — expression evaluation, `LiteralValueExpr` for substituted bodies, body decoding. Used as is; no change expected.
- **`internal/cty` (in-repo go-cty fork)** — `UnknownVal`, `IsWhollyKnown`, value walking and transforming for unknown detection and placeholder substitution. Used as is.
- **`internal/convert`** — Go-to-cty conversion of linked entities in context building. Used as is.
- **`types` package** — `Meta`, statuses, `SensitiveValue` / `RevealAny` for sensitive detection and reveal. No change.
- **`plugins` package** — `ProviderAdapter.Read` / `Changed`, `ErrNotFound`. No change.
- **`events` and `logger` packages** — gains `OperationDiff`; core debug log through `logger.New`. Otherwise unchanged.
- **`parser.TestPlugin` and test fixtures** — the recording provider used by every diff test; reused, new configuration fixtures added.
- **External libraries** — none added; standard library `encoding/json`, `sort`, `strconv`, `sync` only.
- **Upstream specs / plans** — none must land first. The sibling spec 20261007111826-cf3b66d8-diff-rendering-and-docs depends on this plan (it adds rendering to the `diff` package and documents the feature), not the other way round.

Design documents this plan was built on:
- `config-diff.md` from the `design` source — the settled shape of `Config.Diff`, the `diff` package types and their JSON encoding.

## Testing Approach

Tests follow the project's conventions throughout: testify `require`, one behaviour per test function, no table-driven tests, positive and negative cases in separate functions, tests next to the code they test, and saved state produced by a real apply with the recording `parser.TestPlugin` (a failing create for `failed`, a failing destroy for `destroy_failed`) rather than hand-written state files.

**Unit tests — `diff` package.** `Path.String` and its JSON form for attribute, index, key and mixed paths (including quoting of keys); `Diff.Changed`; option resolution; and a golden-shape test that `json.Marshal` of the design's example `Diff` produces exactly the design's JSON (omitted `before`/`after`/`changes`, `unknown` and `sensitive` flags).

**Unit tests — change comparator.** The comparator is a pure function and gets the densest coverage, since it carries most of the design's rules: one changed leaf yields exactly one entry with before and after; computed fields never appear; nested blocks extend the path; lists compare by index (removing the first element changes every later one); maps compare by key; an added element or key has only `after`, a removed one only `before`; nil and empty collections are equal; create lists configured top-level fields with only `after`; unknown paths override comparison and split until each unknown stands alone; a sensitive leaf yields `sensitive: true` with no values, a whole value containing a sensitive leaf is split, and with reveal both values appear with `sensitive` still set.

**Integration tests — parser diff walk.** Using the recording test plugin against parser fixtures: the refresh split leaves every existing apply lifecycle test passing (regression guard landed before diff behaviour); diff makes no Create/Update/Destroy call in any scenario; Read is called for every saved, still-configured entity that is not replaced and has no unknown dependency, and for no other; not-found becomes create; failed and destroy_failed become replace with no provider call; a reference to a new resource's computed field is reported unknown and the referencing resource gets no Read; a provider Read or Changed error fails the diff with an error naming the resource and matching the provider's error with `errors.Is`; builtins, registered types and disabled entities are neither listed nor counted.

**Integration tests — `Config.Diff`.** Each acceptance criterion gets its own test through the public method: identical config gives an empty result with zero counts; state file bytes are identical before and after a diff that reports changes; drift (provider reports changed for an unchanged config) is an update; add / change one attribute / remove produce create (with values), update (exactly that attribute), delete; the mixed scenario reports 2/1/1/1; one changed and two unchanged lists one and counts two; a caller enumerates addresses, actions, paths and values from the Go types; the event handler sees `diff` start and finish events and read/changed lifecycle events, and no create, update or destroy events; a log event from the configured logger is received; `Config.Entities()` is unchanged; the same failures as Apply (no paths, invalid config, empty config, unreadable state) are returned.

**Sensitive leak coverage.** The existing sensitive-leak suite gains diff cases: the diff's JSON, `%v`/`%+v` formatting, and every event emitted during the diff contain no secret when reveal is not requested; with reveal, the result contains both values. Leak checks target distinctive secrets or the specific field, per the known gotcha about temp paths in event data.

**End-to-end tests.** The e2e suite gains diff tests through public packages only, against its plugin (in-process and external providers) and kube (registered types) fixtures.

Success metrics:
- *Diff matches the following apply for every e2e configuration* — **Behavioural test**: for each e2e configuration and each scenario (first apply, unchanged re-apply, edited configuration, removed block, failed resource), the set of addresses the diff reports per action equals the set the following apply actually creates, updates, replaces and deletes, derived from the apply's lifecycle events.
- *Diff leaves saved state and real resources unchanged in every e2e case* — **Behavioural test**: for the same scenarios, state file bytes are identical before and after the diff, and the diff emits no create, update or destroy lifecycle event (the providers' only side-effecting calls).
- *No sensitive value appears in a diff result without an explicit reveal* — **Behavioural test**: the e2e plugin fixture's database passwords and the parser fixtures' credential secrets never appear in the JSON or formatted output of any unrevealed diff, including when the sensitive value changed.

Manual reviews:
- **Manual — captured in the implementation test plan**: review that every new public name, doc comment and error message uses "diff" terminology and never "plan" (a code rule, checked by review rather than by a source-inspecting test, per the testing convention).
- **Manual — captured in the implementation test plan**: review the `diff` package's godoc for the result types against the referenced design (actions, path forms, JSON field presence) so the sibling rendering spec can build on it.

Deliberate gaps: no rendering tests (sibling spec); no tests of the `example/` programs (non-goal); no performance tests (no metric asks for one).

## Milestones & Tasks

### Milestone 1: Code built on xcl can work with diff results

**What changes**: A new public `diff` package gives applications the types a diff returns — the summary counts, each changed resource with its action, each changed value with its structured path and before and after values — together with the option to reveal sensitive values. Nothing produces a diff yet, but the result shape is fixed and its JSON encoding matches the referenced design exactly, so tools and the sibling rendering work can be written against it.

**Validation point**: The `diff` package's tests pass: path strings and JSON for every step kind, the changed-count method, option resolution, and the design's example result marshals to the design's JSON byte for byte.

#### - [x] Task: Add the diff result types package
**Id:** 620e6cd3-2fe8-4861-997d-0d2a9054d1bf
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Creates the public `diff` package holding the result a diff returns: the summary, each changed resource with its action, each changed value with its structured path, and the option to reveal sensitive values. The shapes and JSON encoding are exactly those fixed by the referenced design, so every later task and the sibling rendering spec build on one settled contract.

*Technical detail:* [context.md#task-add-the-diff-result-types-package](./context.md#task-add-the-diff-result-types-package)

**Acceptance criteria**:
- [x] Applications can import the `diff` package and read a result's summary, resources, actions and changes without importing any xcl internals.
- [x] A path prints in the design's forms (`image`, `ports[0].host`, `env["LOG_LEVEL"]`) and encodes to JSON as that string.
- [x] Encoding the design's example result to JSON produces the design's JSON, with absent before, after and changes omitted.
- [x] The number of changed resources is offered as a method that always equals the sum of the four action counts.
- [x] The only option is the one that reveals sensitive values, and it can be resolved by code outside the package.
- [x] The package and its documentation use the name "diff" and never "plan".

### Milestone 2: A diff reports which resources an apply would create, update, replace or delete

**What changes**: Applications can call `Config.Diff` with the same paths they pass to `Apply` and get back which provider-backed resources would be created, updated, replaced or deleted, and how many would stay unchanged, without anything being created, changed, destroyed or saved. Existing resources are checked against the real system through their providers, so drift shows as an update and a vanished resource as a create; failed resources are reported as replaced. Provider errors fail the diff naming the resource, and the diff is observable through `diff` operation events, the provider read and change events, and the configured logger. The apply read path is reorganised internally so apply and diff share the same read-and-compare step; apply's behaviour is unchanged. Resources are listed without their individual value changes at this stage.

**Validation point**: Every existing test still passes, and new parser and `Config.Diff` tests show the right action for each scenario, no Create/Update/Destroy calls, reads for exactly the expected resources, byte-identical state after a diff, the summary counts, the error naming the resource, and the events and log.

#### - [x] Task: Share the read-and-compare step between apply and diff
**Id:** 717823ca-0a37-4d65-9f51-3582ea79e6ee
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Reorganises apply's handling of an existing resource so that reading it through its provider and asking whether it changed is one step, reporting whether the resource was not found, changed or unchanged, and apply then acts on that outcome. This is an internal change with no effect on apply; it exists so the diff can run exactly the same read-and-compare step and can never disagree with apply about what changed.

*Technical detail:* [context.md#task-share-the-read-and-compare-step-between-apply-and-diff](./context.md#task-share-the-read-and-compare-step-between-apply-and-diff)

**Acceptance criteria**:
- [x] Apply makes the same provider calls, in the same order per resource, with the same events and the same saved state as before the change.
- [x] Every existing lifecycle, event and state test passes without modification.
- [x] The read-and-compare step can be run on its own and reports not found, changed or unchanged without ever creating or updating anything.

#### - [x] Task: Add the diff walk to the parser
**Id:** 3ea446ce-e0a9-4399-8ee4-69e225c8dae7
**Repo:** xclconfig
**Depends on:**
- 620e6cd3-2fe8-4861-997d-0d2a9054d1bf — Add the diff result types package
- 717823ca-0a37-4d65-9f51-3582ea79e6ee — Share the read-and-compare step between apply and diff
**Execution:** agent

Adds a diff operation to the parser that parses and validates exactly as apply does, then walks the configuration in a diff mode: new resources are recorded as create, existing ones are read and compared through their providers and recorded as update or unchanged, a vanished one as create, a failed one as replace, and removed ones as delete. Nothing is created, updated, destroyed or saved, and a provider error stops the diff with an error naming the resource. Resources are recorded without value changes at this stage.

*Technical detail:* [context.md#task-add-the-diff-walk-to-the-parser](./context.md#task-add-the-diff-walk-to-the-parser)

**Acceptance criteria**:
- [x] A diff never calls a provider's create, update or destroy, and never writes to the state store.
- [x] Every resource in state that is still configured and not failed is read and compared through its provider, and no other resource is read.
- [x] New resources are reported as create, removed ones as delete, failed and failed-to-destroy ones as replace, changed or drifted ones as update, and vanished ones as create.
- [x] Variables, outputs, modules, disabled blocks and registered config-only types are neither listed nor counted; other unchanged resources are counted but not listed.
- [x] A provider error during a read or compare fails the diff with an error that names the resource and still matches the provider's error.
- [x] A diff emits read and changed lifecycle events, and never a create, update or destroy event.
- [x] Apply's behaviour is unchanged.

#### - [x] Task: Add Config.Diff as a public operation
**Id:** 610d5a71-a42e-4264-918f-14086db077ba
**Repo:** xclconfig
**Depends on:**
- 3ea446ce-e0a9-4399-8ee4-69e225c8dae7 — Add the diff walk to the parser
**Execution:** agent

Adds `Config.Diff`, which takes the same paths as `Apply` plus diff options and returns the diff result. It runs as its own `diff` operation with the same start and finish events, plugin handling and logger as the other operations, and writes a summary log line when it completes. It never saves state and leaves what the Config holds untouched.

*Technical detail:* [context.md#task-add-configdiff-as-a-public-operation](./context.md#task-add-configdiff-as-a-public-operation)

**Acceptance criteria**:
- [x] A diff of a configuration identical to the one applied, with unchanged real resources, returns no changed resources and zero to create, update, replace and delete.
- [x] The saved state is byte-for-byte identical before and after a diff that reports changes, and the Config's entities are unchanged.
- [x] A mixed configuration reports the right number to create, update, replace, delete and leave unchanged.
- [x] A resource altered outside xcl is reported as updated against an unchanged configuration.
- [x] A subscribed caller receives start and finish events for the `diff` operation and the per-resource read and changed events, and log entries arrive through the configured logger.
- [x] A diff fails in the same cases as apply: no paths, a configuration that does not parse or validate, an empty configuration, or a state that cannot be loaded.

### Milestone 3: A diff shows exactly which values would change, with secrets hidden

**What changes**: Each created, updated or replaced resource in a diff now lists its changed values, one entry per changed field with its path and before and after values, following the design's rules for nested blocks, lists, maps and added or removed elements, and never showing provider-computed fields. Sensitive values are reported as changed without either value unless the caller asks to reveal them.

**Validation point**: Comparator unit tests cover every change rule; `Config.Diff` tests show a single changed attribute listed alone, a created resource's values listed, and a changed sensitive value hidden in the result and its JSON, then revealed with the option; the sensitive-leak suite's diff cases pass.

#### - [x] Task: Compute field-level changes between saved and configured resources
**Id:** bc6cdfea-0b40-4bd1-aba6-67103977585f
**Repo:** xclconfig
**Depends on:**
- 620e6cd3-2fe8-4861-997d-0d2a9054d1bf — Add the diff result types package
**Execution:** agent

Adds the comparator that turns a resource's saved copy and configured copy into its list of changed values, following the design's rules: leaves by value, nested blocks by path, lists by position, maps by key, added and removed elements as single entries, computed fields never compared, a created resource's configured fields listed with only their new value. Sensitive values are reported as changed without either value unless revealing was asked for.

*Technical detail:* [context.md#task-compute-field-level-changes-between-saved-and-configured-resources](./context.md#task-compute-field-level-changes-between-saved-and-configured-resources)

**Acceptance criteria**:
- [x] Changing one attribute yields exactly one change for that attribute, with its old and new values.
- [x] Values a provider computes never appear as changes.
- [x] Lists are compared by position, maps by key, and an added or removed element appears once with only its new or old value.
- [x] A created resource lists each configured top-level field with only its new value; a deleted resource lists nothing.
- [x] A changed sensitive value is listed as changed with neither value present; with revealing asked for, both values are present and it is still marked sensitive.
- [x] No sensitive value is reachable from a change unless revealing was asked for, including inside an added or created whole value.
- [x] Values in changes are plain values keyed by configuration names, so their JSON reads like the configuration.

#### - [x] Task: Report value changes in diff results
**Id:** 541a9fbb-3a8e-4eea-835f-099b6fcb338f
**Repo:** xclconfig
**Depends on:**
- 610d5a71-a42e-4264-918f-14086db077ba — Add Config.Diff as a public operation
- bc6cdfea-0b40-4bd1-aba6-67103977585f — Compute field-level changes between saved and configured resources
**Execution:** agent

Connects the comparator to the diff walk so every created, updated and replaced resource in a diff carries its changed values, comparing the saved copy with the configuration as written rather than with what the provider read back. Extends the sensitive-leak checks to diffs, so no secret can escape through a diff result, its JSON, its formatting or its events.

*Technical detail:* [context.md#task-report-value-changes-in-diff-results](./context.md#task-report-value-changes-in-diff-results)

**Acceptance criteria**:
- [x] Adding a resource reports it as created with its configured values listed.
- [x] Changing one attribute of an applied resource reports that resource as updated, listing exactly that attribute with its old and new values.
- [x] A resource that drifted with no configuration change is reported as updated with no changes listed.
- [x] A changed sensitive value appears in the result and its JSON as changed, with neither value, unless the diff asked to reveal sensitive values, in which case both appear.
- [x] No secret appears in a diff result, its JSON, its formatted output or any event a diff emits, unless revealing was asked for.
- [x] Callers can enumerate every changed resource's address and action and every change's path, before and after values from the Go result alone.

### Milestone 4: A diff marks values known only after apply, and matches what apply does

**What changes**: A value that depends on something a resource being created or replaced will compute is reported as known only after apply instead of as a guessed value, and a resource already in state that depends on such a value is reported as updated without asking its provider. The end-to-end suite now runs a diff before each apply and confirms the diff predicted exactly what the apply then did, left state and resources untouched, and never exposed a secret.

**Validation point**: The unknown-reference tests pass (the referencing value is marked unknown, the referencing resource gets no read, dependents through outputs and modules also see the unknown), and the end-to-end diff tests pass for both fixtures in every scenario.

#### - [x] Task: Mark values known only after apply
**Id:** 335e1ee4-6896-4057-8cc9-6bf5e67aa2a0
**Repo:** xclconfig
**Depends on:**
- 541a9fbb-3a8e-4eea-835f-099b6fcb338f — Report value changes in diff results
**Execution:** agent

Makes the diff treat values that a resource being created or replaced will compute as unknown: any configured value depending on one is reported as known only after apply rather than as a guessed value, and the unknown is carried through to resources that depend on it via outputs, module variables and other resources. A resource already in state that depends on such a value is reported as updated without asking its provider, since asking about a half-resolved resource would be meaningless.

*Technical detail:* [context.md#task-mark-values-known-only-after-apply](./context.md#task-mark-values-known-only-after-apply)

**Acceptance criteria**:
- [x] When an existing resource is changed to reference a new resource's computed value, the diff reports the referencing value as known only after apply and the provider records no read for the referencing resource.
- [x] A created resource whose value depends on another created resource's computed value lists that value as known only after apply, with every other value concrete.
- [x] An unknown inside a larger value is reported on its own, with the rest of the value concrete.
- [x] Unknowns pass through outputs and module variables to the resources that use them.
- [x] Apply's behaviour is unchanged, and no unknown value ever reaches a provider or the result's values.

#### - [x] Task: Treat computed values of updated resources as unknown
**Id:** 25298ac3-b8c3-4cc7-a632-671bf831d97a
**Repo:** xclconfig
**Depends on:**
- 335e1ee4-6896-4057-8cc9-6bf5e67aa2a0 — Mark values known only after apply
**Execution:** agent

Applies the revised design's conservative default: when a resource is reported as update, every computed field on it is unknown, so every resource that references one is reported as update with that value known only after apply and gets no provider call, exactly as dependents of created and replaced resources already are. The diff never under-reports and may over-report. Added after the end-to-end suite found an apply updating a resource the diff had not listed (the user chose to change the design).

*Technical detail:* [context.md#task-treat-computed-values-of-updated-resources-as-unknown](./context.md#task-treat-computed-values-of-updated-resources-as-unknown)

**Acceptance criteria**:
- [x] A resource reported as update, whether its provider reported a change or it depends on an unknown value, has every computed field unknown to the resources that reference it.
- [x] A saved resource that references a computed field of an updated resource is reported as update with that value unknown, and its provider records no read for it.
- [x] A resource that references only configured fields of an updated resource is unaffected.
- [x] Apply's behaviour is unchanged.

#### - [x] Task: Prove diff matches apply end to end
**Id:** 56175414-e2c4-43ba-9e91-cea244dad16a
**Repo:** xclconfig
**Depends on:**
- 25298ac3-b8c3-4cc7-a632-671bf831d97a — Treat computed values of updated resources as unknown
**Execution:** agent

Adds end-to-end tests that run a diff before each apply against the suite's plugin and registered-type configurations, across a first apply, an unchanged re-apply, an edited configuration, a removed block and a failed resource. They confirm the diff predicted exactly what the following apply did, left state and resources untouched, and never exposed a secret, using only xcl's public packages.

*Technical detail:* [context.md#task-prove-diff-matches-apply-end-to-end](./context.md#task-prove-diff-matches-apply-end-to-end)

**Acceptance criteria**:
- [x] For every end-to-end configuration and scenario, an apply never changes a resource the diff did not list (every resource the following apply creates, updates, replaces or deletes is in the diff with that action), and every listed resource the apply leaves alone was listed with at least one unknown value.
- [x] For every end-to-end configuration and scenario, saved state is byte-for-byte unchanged by the diff and no resource is created, updated or destroyed during it.
- [x] No database password from the plugin configuration appears in any diff result, its JSON or the events it emits without an explicit reveal.
- [x] The end-to-end tests use only xcl's public packages and the shared test helpers.

## Open Questions

- **Does gohcl decode every placeholder the diff decoder substitutes for an unknown?** It depends on how the in-repo gohcl/gocty fork converts literal placeholders (empty string, zero, false, empty collection, or null for a dynamic type) into each Go field kind, which is only discoverable by exercising it against the fixture types. If a field kind rejects its placeholder, the implementer adjusts the placeholder for that kind; if no placeholder can satisfy a field without changing the HCL fork, STOP and ask the user before modifying `internal/xcl`.
- **Does the e2e plugin fixture allow producing a failed resource through public packages?** It depends on whether the in-process or external provider can be made to fail a create from a test without changing the fixture's behaviour for other e2e tests. If it cannot, the replace scenario is covered by the root-package tests only and the e2e test says so in a comment; if covering it would require changing what existing e2e tests observe, STOP and ask the user.

## Out of Scope

- Rendering a diff as text (`diff.Render`, `diff.Highlight`, the git-diff-style output and colour) — sibling spec 20261007111826-cf3b66d8-diff-rendering-and-docs, which builds on this plan's `diff` package.
- Documentation on the xcl website explaining diffs — same sibling spec.
- Adding diff output to the `example/` programs — left for later (spec non-goal).
- A command-line interface — xcl remains a library.
- Saving a diff and later applying exactly that diff.
- Matching list elements by identity rather than by position; lists are compared strictly by index.
- Showing the saved values of a resource that would be deleted; a delete lists no changes.
- Changes to the VS Code extension.
- The provider override for computed values of updated resources (an optional `ComputedChanges(ctx, old, new T) ([]string, error)` provider method, its gRPC RPC, and the docker network DNS example) — a separate spec in this epic. This spec implements only the conservative default: every computed field of a resource reported as update is unknown to its dependents.
- Reading a diff's JSON back into Go types (`Path` marshals only).

## Changelog

### 2026-10-07 — Task: Add the diff result types package

**What was done**: Added the public `diff` package with the result types (`Diff`, `Summary`, `Resource`, `Change`, `Action` and its constants, `Path`, `Step`, `StepKind`), `Path.String`/`MarshalJSON`, nil-safe `Diff.Changed()`, the `Path.Attribute`/`Index`/`Key` copy-on-extend constructors, and the `Options`/`Option`/`RevealSensitive`/`NewOptions` option plumbing, with external-package tests including the design's JSON example.

**Deviations**: None.

**Files changed**:
- `diff/diff.go`
- `diff/path.go`
- `diff/options.go`
- `diff/diff_test.go`
- `diff/path_test.go`
- `diff/options_test.go`

**Discoveries**: A path whose first step is an index prints without a leading dot (`[2].host`); an empty path prints as the empty string.

### 2026-10-07 — Task: Share the read-and-compare step between apply and diff

**What was done**: Split `resourceLifecycle.read` into a shared `refresh` step (carry saved computed values, provider Read with its events, restore configured values on not found, provider Changed with its events) that reports `refreshNotFound`, `refreshChanged` or `refreshUnchanged` along with the serialized saved, configured (pre-read) and read copies; `read` now refreshes and then creates, keeps the previous status, or updates. Added focused tests of `refresh` on its own.

**Deviations**: None.

**Files changed**:
- `internal/parser/lifecycle.go`
- `internal/parser/lifecycle_test.go`

**Discoveries**: Saved entities loaded from state are schema-generated types, not `*structs.Network`; tests convert through `findResource` to inspect fixture fields. The `example/plugin` tests (and the e2e test that runs them) fail on this host because a Docker network named `app` already exists — environmental, unrelated to this change.

### 2026-10-07 — Task: Add the diff walk to the parser

**What was done**: Added `events.OperationDiff`, a concurrency-safe `diffRecorder` (pending set, unknown paths, resources, unchanged count, sorted result with summary), a `walkMode` and `diff` step on `resourceLifecycle` that chooses create / refresh / replace as `run` does without calling Create, Update or Destroy, and `Parser.Diff`, which parses and validates like Apply, records provider-backed removed resources as delete and walks in diff mode via a new `walkWith` that takes a prepared lifecycle. Walk-level error events now carry the walk's operation (apply or diff).

**Deviations**: `walk` keeps its signature for Apply and delegates to a new `walkWith(ctx, current, previous, functions, lifecycle)`; the lifecycle carries the mode, recorder and diff options rather than `walk` taking extra arguments. Behaviourally equivalent to the plan.

**Files changed**:
- `events/events.go`
- `internal/parser/diff_recorder.go`
- `internal/parser/lifecycle.go`
- `internal/parser/callbacks.go`
- `internal/parser/parser.go`
- `internal/parser/diff_test.go`
- `internal/test_fixtures/config/diff/base/main.xcl`
- `internal/test_fixtures/config/diff/changed_attribute/main.xcl`
- `internal/test_fixtures/config/diff/added/main.xcl`
- `internal/test_fixtures/config/diff/removed/main.xcl`
- `internal/test_fixtures/config/diff/mixed/main.xcl`
- `internal/test_fixtures/config/diff/with_builtins/main.xcl`
- `internal/test_fixtures/config/diff/with_builtins/module/outputs.xcl`

**Discoveries**: Removed resources are recorded before the walk starts, so the recorder's sort is what puts deletes in address order. Value-level changes are not yet filled (next milestone).

### 2026-10-07 — Task: Add Config.Diff as a public operation

**What was done**: Added `Config.Diff(paths, options...)`, which runs `Parser.Diff` as the `diff` operation through `Config.run` with the same parser options as Apply, emits a core debug log "diff complete" with the action counts, and never saves state or touches the Config's entities. Mentioned Diff in the `Config` and `run` doc comments. Added `Config.Diff` tests and two fixtures for the mixed create/update/replace/delete scenario.

**Deviations**: The "state cannot be loaded" negative test uses a corrupt state file written over a real apply's state rather than an `UnknownTypesError`; it exercises the same load failure path.

**Files changed**:
- `config_diff.go`
- `config.go`
- `config_diff_test.go`
- `internal/test_fixtures/config/diff/replace_base/main.xcl`
- `internal/test_fixtures/config/diff/replace_mixed/main.xcl`

**Discoveries**: A drifted parent (network) reported as update does not make its dependent containers updates: they are read and compared on their own and count as unchanged.

### 2026-10-07 — Task: Compute field-level changes between saved and configured resources

**What was done**: Added `resourceChanges` in the parser: a pure comparator that walks the saved and configured copies by xcl field name in declaration order, skips computed fields, compares leaves by value, descends into blocks, compares lists by index and maps by sorted key, reports added/removed elements whole, lists a create's configured top-level fields (set in the body or non-zero), masks sensitive values unless revealed, honours unknown paths, and splits added/created whole values holding a sensitive or unknown value until it stands alone. Values are converted to plain Go values keyed by xcl names; `cty.Value` fields are leaves converted through their JSON form.

**Deviations**: None.

**Files changed**:
- `internal/parser/diff_changes.go`
- `internal/parser/diff_changes_test.go`

**Discoveries**: A whole added/removed element that is not split carries all its non-computed fields, including zero values (e.g. `{"id":0,"name":"app","ip_address":"","aliases":null}`); only split values leave zero parts out. Unknown entries on update/replace carry the saved value as `before`.

### 2026-10-07 — Task: Report value changes in diff results

**What was done**: The diff step now fills each resource's `Changes` through `resourceChanges`: create from the decoded (or, when the provider no longer finds it, restored) configured resource, replace from the saved copy against the configured resource, and update from the saved copy against the configured snapshot taken before the provider Read, so drift alone is an update with no changes. Reveal comes from the diff options. Tests cover create values, a single changed attribute, drift without changes, enumerating the Go result, sensitive masking and reveal, and diff cases in the sensitive-leak suite (JSON, `%v`, `%+v`, events).

**Deviations**: The credential fixture's password is sensitive because it flows into a `types.Sensitive` field (the repo has no `sensitive = true` variable syntax). Four earlier diff-walk tests that compared whole `diff.Resource` values were narrowed to address and action now that changes are filled.

**Files changed**:
- `internal/parser/lifecycle.go`
- `internal/parser/diff_test.go`
- `config_diff_test.go`
- `sensitive_leak_test.go`
- `internal/test_fixtures/config/diff/credential/before/main.xcl`
- `internal/test_fixtures/config/diff/credential/after/main.xcl`

**Discoveries**: A created resource lists defaults applied by `defaults.Set` (e.g. container `default = "hello world"`) because the field is non-zero after decoding, as the plan intended.

### 2026-10-07 — Task: Mark values known only after apply

**What was done**: Added the unknown-value hook: `buildContextForResource` takes an optional `unknownValues` (the diff recorder; nil in apply) that makes a pending entity's computed fields, and every recorded unknown path of an entity, unknown in its dependents' context. Added `decodeForDiff`, which evaluates each attribute, records the path of every unknown (split until each stands alone), and decodes a copied body with type-appropriate placeholders, never mutating the parsed body. The walk callback decodes provider-backed and registered entities through it in diff mode, outputs skip Go conversion when not wholly known, and a saved resource with unknown values is reported as update without a provider call, keeping its saved computed values for dependents.

**Deviations**: Output unknown paths are not recorded separately: outputs and module variables hold `cty.Value`s that carry unknowns natively, which is enough for dependents (verified through module outputs and module variables). A saved resource reported as update because of unknowns has its saved computed values carried onto it so its dependents see them.

**Files changed**:
- `internal/parser/diff_unknown.go`
- `internal/parser/diff_decode.go`
- `internal/parser/context.go`
- `internal/parser/callbacks.go`
- `internal/parser/lifecycle.go`
- `internal/parser/diff_unknown_test.go`
- `internal/parser/diff_decode_test.go`
- `config_diff_test.go`
- `internal/test_fixtures/config/diff/unknown_ref/before/main.xcl`
- `internal/test_fixtures/config/diff/unknown_ref/after/main.xcl`
- `internal/test_fixtures/config/diff/unknown_chain/main.xcl`
- `internal/test_fixtures/config/diff/unknown_collection/main.xcl`
- `internal/test_fixtures/config/diff/unknown_output/main.xcl`
- `internal/test_fixtures/config/diff/unknown_output/net/main.xcl`
- `internal/test_fixtures/config/diff/unknown_output/app/main.xcl`

**Discoveries**: The placeholder open question resolved without touching the HCL fork: empty string, zero, false and empty collections decode into every field kind exercised. A root-level output cannot be referenced as `output.x`; outputs reach other blocks only as `module.<name>.output.<x>`. In a create, a block split because it holds an unknown omits its zero-valued siblings, while a fully known block is listed whole with its zero fields.

### 2026-10-07 — Plan change: computed values of updated resources are unknown

**What was done**: The end-to-end diff test for an edited redis port found an apply updating `resource.app.web` that the diff had not listed: redis's Update recomputes `connection_string`, which `app.web` references, and the plan took an updated resource's computed values from its provider's read. The user chose to change the design (`config-diff.md`, new section "Computed values of updated resources"). The plan now applies its conservative default: every computed field of a resource reported as update is unknown, so its dependents are reported as update with that value unknown and get no provider call. A new task "Treat computed values of updated resources as unknown" was added before the end-to-end task. The Out of Scope line that kept updated resources' computed values known was replaced by an entry putting the provider override (`ComputedChanges`, its gRPC RPC, the docker network DNS example) in a separate spec in this epic.

**Deviations**: The end-to-end task's parity criterion is restated as the new contract: an apply never changes a resource the diff did not list, and any listed resource the apply leaves alone was listed with at least one unknown value. This supersedes the spec's success metric "diff matches apply for every scenario"; the spec itself is not edited.

**Files changed**:
- none (plan and context only)

**Discoveries**: Apply needs no change for the new contract: it re-reads a dependent after its parent updates and skips Update when Changed reports nothing.

### 2026-10-07 — Task: Treat computed values of updated resources as unknown

**What was done**: A resource the diff reports as update — because its provider's refresh reported a change, or because it depends on unknown values — is now marked pending like a create or replace, so every computed field on it is unknown to its dependents. A saved dependent that references one is reported as update with that value unknown and gets no provider call; dependents that reference only configured fields or `meta` are read normally. The earlier carry of saved computed values onto an unknown-dependent update was removed, since those values are now unknown to dependents anyway. Tests cover drift and config-edit updates, configured and meta references, propagation through a chain, the unchanged regression, and that the following apply leaves an over-reported dependent alone.

**Deviations**: None from the revised plan. Verification: `internal/parser`, root and `diff` packages pass. The e2e diff tests `TestDiffOfEditedRedisPortPredictsApply` and `TestDiffOfChangedPasswordPredictsApply` now see the expected over-reporting (`app.web`/`ingress.web` listed with unknowns) and are updated to the new contract in the next task. `TestPluginExampleTestsPass` / `example/plugin` still fail on this host because a Docker network named `app` already exists — environmental, left alone.

**Files changed**:
- `internal/parser/lifecycle.go`
- `internal/parser/diff_recorder.go`
- `internal/parser/diff_unknown.go`
- `internal/parser/diff_update_unknown_test.go`
- `internal/test_fixtures/config/diff/update_ref/before/main.xcl`
- `internal/test_fixtures/config/diff/update_ref/edited/main.xcl`

**Discoveries**: A drift-only update lists the drifted resource with no changes; its dependents that use its computed values are listed as updates with unknown values even though the following apply leaves them alone (Changed reports nothing) — this is the accepted over-report.

### 2026-10-07 — Task: Prove diff matches apply end to end

**What was done**: Added `e2e/diff_test.go`, which drives `Config.Diff` and `Config.Apply` through public packages only against copies of the plugin and kube fixtures. Scenarios: first apply (all creates; app and ingress computed values reported unknown), unchanged re-apply (nothing listed), edited redis port, edited replica location, removed ingress, changed database password (sensitive change), and a failed replica that is replaced. For each one the tests check that the state file bytes are unchanged and that no create, update or destroy event fires during the diff. They then derive the apply's per-action sets from its lifecycle events and assert the parity contract with `requireDiffPredictsApply`. The kube (registered types) diff reports nothing. Leak tests check the JSON, `%v`, `%+v` and event channels before and after a password change. `e2e/COVERAGE.md` gains a diff section.

**Deviations**: The parity criterion is the new contract from the user's design decision, not exact equality. The contract: every resource the apply changes is in the diff with the same action, and any listed resource the apply leaves alone has at least one unknown value. In the redis-port and changed-password scenarios the diff lists `app.web` and `ingress.web` as updates with unknown values: `cache_connection_string` or `connection_string`, and `app_url`. This is the accepted over-report. The spec's success metric "diff matches apply for every scenario" is superseded by this contract; the spec is not edited. Verification: every package passes except `TestPluginExampleTestsPass` (e2e) and the `example/plugin` tests. They fail because a Docker network named `app` already exists on this host, which is environmental and unrelated, and the network was left alone.

**Files changed**:
- `e2e/diff_test.go`
- `e2e/COVERAGE.md`

**Discoveries**: A failed resource can be produced end to end without changing fixtures: setting the postgres replica location to `"bad host"` in a temp copy makes the in-process plugin's `connect` fail.
