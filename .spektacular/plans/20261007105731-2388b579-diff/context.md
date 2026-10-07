---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
---

# Context: 20261007105731-2388b579-diff

## Current State Analysis

- xcl has `Validate`, `Apply`, `Destroy` and `Load` on `Config` (`config.go:288-465`), each run through `Config.run` (`config.go:497-551`), which emits operation start/finish events, activates plugins (`config.go:559-574`) and delivers events to the handler. There is no way to ask what an apply would do without doing it.
- `Parser.Apply` (`internal/parser/parser.go:263-363`) parses and validates (`parseAndValidate`, `parser.go:452-576`, loading previous state from the store or an empty state), rejects an empty configuration, destroys removed resources through a `destroyer` that saves state after each one (`parser.go:282-308`, `internal/parser/destroy.go:56-159`), then walks the create DAG (`parser.go:1320-1367`).
- `walkCallback` (`internal/parser/callbacks.go:38-199`) builds each entity's eval context from its links (`internal/parser/context.go:15-226`), evaluates `disabled`, applies defaults, decodes the body straight into the Go struct with `gohcl.DecodeBody` (`callbacks.go:124`), evaluates module variables, then runs `resourceLifecycle.apply` (`internal/parser/lifecycle.go:55-82`).
- `resourceLifecycle.run` (`lifecycle.go:85-125`) chooses create (no saved entry), read (saved created/updated) or rebuild (any other status). `read` (`lifecycle.go:167-273`) carries computed values, calls `Read(saved, configured)`, handles `plugins.ErrNotFound` by re-creating, calls `Changed(saved, read)`, and calls `Update` when changed. Provider-less entities (`handledWithoutProvider`, `lifecycle.go:453-463`) emit a create success event and never reach a provider.
- Computed fields are owned by providers and marked in the `xcl` tag (`internal/parser/computed.go:13-23`); helpers walk struct fields by xcl name (`computed.go:68-170`). The configured-value check (`internal/parser/configured_check.go`) is the nearest existing saved-vs-configured walk.
- Sensitive values are `types.Sensitive[T]` leaves (`types/sensitive.go`), detected by type (`computed.go:31`); validation forbids a sensitive value reaching a plain field (`internal/parser/sensitive_check.go`).
- Tests use `parser.TestPlugin` (`internal/parser/test_plugin.go`) as a recording provider with error and behaviour injection; root tests build Configs with a file state store and event recorder (`config_destroy_test.go:41-80`); the e2e suite uses public packages only (`e2e/COVERAGE.md`).

## Per-Task Technical Notes

### Task: Add the diff result types package

Requirements carried: Result is inspectable by code, Summary counts, Unknown values are marked (type shape), Sensitive values stay hidden (type shape). Repo: xclconfig, root `/home/nicj/code/github.com/jumppad-labs/xcl`.

**File changes**:
- `diff/diff.go` (new) — package doc (diff terminology only), `Action` + four constants, `Diff`, `Summary`, `Resource`, `Change` with the exact JSON tags from the design, `func (d *Diff) Changed() int` (nil-safe: nil receiver returns 0).
- `diff/path.go` (new) — `StepKind` (`StepAttribute`, `StepIndex`, `StepKey`), `Step`, `Path`; `Path.String()`: attributes joined by `.`, index as `[n]` with no leading dot, key as `[` + `strconv.Quote(key)` + `]`; `Path.MarshalJSON()` returns `json.Marshal(p.String())`. Small constructors used by the parser are fine (`Path.Attribute(name) Path`, `Path.Index(i) Path`, `Path.Key(k) Path`, each returning a copy, never aliasing the receiver's backing array).
- `diff/options.go` (new) — `Options{RevealSensitive bool}`, `type Option func(*Options)`, `RevealSensitive() Option`, `NewOptions(options ...Option) Options` (nil options ignored).
- `diff/diff_test.go`, `diff/path_test.go`, `diff/options_test.go` (new) — one behaviour per test, `require`: each path form; key quoting (`env["A\"B"]`); JSON of a Path is its string; `Changed()` sums; `NewOptions()` default false; `NewOptions(RevealSensitive())` true; design example `Diff` marshals to the design JSON (compare with `require.JSONEq` against the literal from `config-diff.md` § JSON format).
- Package must import only the standard library (no `xcl`, no `internal/parser`).

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Share the read-and-compare step between apply and diff

Requirements carried: Existing resources are refreshed (shared with apply, per spec Technical Approach). Repo: xclconfig.

**File changes**:
- `internal/parser/lifecycle.go:162-273` — extract lines 178-252 into `func (l *resourceLifecycle) refresh(r, old any, adapter plugins.ProviderAdapter) (refreshOutcome, refreshed, error)` where the returned data carries `oldData`, `configuredData` (snapshot before `carryComputedValues`, line 184) and `readData`. It does: marshal old/configured, `carryComputedValues`, `callProvider(OperationRead…)`, on `plugins.ErrNotFound` restore `configuredData` via `replaceValues` and return `refreshNotFound`; emit read success; `warnChangedConfiguredValues(OperationRead…)`; `callProvider(OperationChanged…)`; emit changed success; return `refreshChanged`/`refreshUnchanged`. Error paths keep setting `meta.Status = types.StatusFailed` exactly as today (lines 217-220, 241-244) and propagate `errNotReached` unchanged.
- `internal/parser/lifecycle.go:167` — `read` becomes: `refresh`; not found → `l.create(r, adapter)`; unchanged → `meta.Status = oldMeta.Status`; changed → the existing Update block (lines 254-272) unchanged.
- `internal/parser/lifecycle.go` — add `type refreshOutcome int` with `refreshNotFound`, `refreshChanged`, `refreshUnchanged`.
- `internal/parser/lifecycle_test.go` — add focused tests of `refresh` on its own using the existing test harness there (not-found → outcome and configured values restored; drift via `SetReadObserved` → changed; unchanged; no Create/Update calls recorded by `TestPlugin` in any case). Existing tests must pass unmodified.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential: refactor, run the whole parser package tests, then add the refresh tests.

### Task: Add the diff walk to the parser

Requirements carried: Diff compares configuration with state, Diff changes nothing, Existing resources are refreshed, New/Changed/Removed/Failed/Missing resources reported, Unchanged only counted, Summary counts, Provider errors reported, Diff is observable (lifecycle events). Repo: xclconfig.

**File changes**:
- `events/events.go:48-61` — add `OperationDiff = "diff"` with a doc comment.
- `internal/parser/diff_recorder.go` (new) — `diffRecorder` (mutex; `pending map[string]bool`; `unknown map[string][]diff.Path`; `resources []diff.Resource`; `unchanged int`), methods `markPending`, `isPending`, `recordUnknown`, `unknownPaths`, `record`, `recordUnchanged`, `result()` which sorts `resources` by `Address` (`sort.Slice`), counts actions into `Summary`, and returns `*diff.Diff` with a non-nil empty `Resources` slice when nothing changes.
- `internal/parser/lifecycle.go:24-42` — add `mode walkMode` (`walkApply` zero value, `walkDiff`) and `recorder *diffRecorder` to `resourceLifecycle`.
- `internal/parser/lifecycle.go` — add `func (l *resourceLifecycle) diff(r any) error`: provider-less (`handledWithoutProvider`, line 453) → return nil, no event, no record; adapter nil → same error as `run` (lines 97-102); no saved entry (`findByID` not found, line 104) → `markPending`, record `create`; saved status created/updated → `refresh`: not found → `markPending`, record `create`; changed → record `update`; unchanged → `recordUnchanged`; any other status → `markPending`, record `replace` with no provider call. Errors from refresh are returned as is (they already name the resource via `callProvider`, line 411).
- `internal/parser/callbacks.go:38-199` — `walkCallback` uses `lifecycle.mode`: in diff mode call `lifecycle.diff(r)` instead of `lifecycle.apply(r)` (line 172); keep the ParserError wrapping with `Cause` (lines 177-185). `emitWalkError` (line 325) takes the operation from the lifecycle (apply or diff).
- `internal/parser/parser.go:1320-1367` — `walk` gains an `operation string` and a `*resourceLifecycle`-mode argument (or takes a prepared lifecycle): pass `events.OperationApply` from Apply (line 315) and `events.OperationDiff` from Diff; `emitOperationError` calls at lines 1324 and 1335 use the operation.
- `internal/parser/parser.go` (after Apply, ~line 364) — `func (p *Parser) Diff(ctx context.Context, options diff.Options, paths ...string) (*diff.Diff, error)`: `parseAndValidate` (line 452); `ErrEmptyConfiguration` when `currentState.ResourceCount() == 0` (as line 269); build recorder; for each of `removedResources(currentState, previousState)` (line 413) whose meta is not disabled and not `handledWithoutProvider(p.typeRegistry, meta)`, record `delete` (no changes); walk in diff mode with the recorder; if errors → return `nil, ce` (ConfigError of all errors, as Apply does at lines 321-336, never a partial result); if `ctx.Err() != nil` → `nil`, error wrapping `ctx.Err()`; else `recorder.result()`. Store `options` on the lifecycle for later tasks (reveal flag). Never construct a `destroyer`.
- `internal/test_fixtures/config/diff/...` (new) — fixtures: `base/main.xcl` (network + two containers), `changed_attribute/main.xcl`, `added/main.xcl`, `removed/main.xcl`, `mixed/main.xcl` (adds two, changes one, deletes one; replace comes from a failed resource produced by `SetCreateError`), `with_builtins/main.xcl` (variable, output, module, disabled block, registered type alongside one provider resource).
- `internal/parser/diff_test.go` (new) — tests per behaviour, state produced by real `Parser.Apply` with `TestPlugin` and a `state.FileStateStore` in `t.TempDir()` saved via `EncodeForState` as Config does (or by driving through a small local helper): no Create/Update/Destroy calls (`GetCreatedResources`, `GetUpdatedResources`, `GetDestroyedResources` empty after `ResetCalls`); `GetReadResources` equals the expected set (compare sorted, not ordered); action per scenario; failed (via `SetCreateError`) and destroy_failed (via `SetDestroyError` on a removed-then-re-added resource or failed rebuild) → replace with no read; `SetReadNotFound` → create; `SetReadError` / `SetChangedError` → error whose message contains the resource address and `errors.Is` matches the injected error; builtins/registered/disabled neither listed nor counted; state file bytes unchanged.

**Complexity**: High
**Token estimate**: ~60k tokens
**Agent strategy**: Parallel analysis, sequential integration: one agent writes fixtures and tests while another implements recorder and lifecycle step; integrate the walk changes sequentially, then run the full parser test suite.

### Task: Add Config.Diff as a public operation

Requirements carried: Diff compares configuration with state, Diff changes nothing, Summary counts, Result is inspectable by code, Diff is observable (operation events and logs). Repo: xclconfig.

**File changes**:
- `config_diff.go` (new, package `xcl`) — `func (c *Config) Diff(paths []string, options ...diff.Option) (*diff.Diff, error)` with a doc comment in diff terms (what it compares, that it changes nothing, refreshes through providers, the same failure cases as Apply). Body: `var result *diff.Diff`; `err := c.run(events.OperationDiff, func(ctx, emit) error { if len(paths)==0 {…same message as Apply, config.go:324…}; p := parser.NewParser(&parser.ParserOptions{…same fields as Apply, config.go:329-337…}); d, err := p.Diff(ctx, diff.NewOptions(options...), paths...); if err != nil { return err }; logger.New(emit, events.Event{Source: events.SourceCore, Operation: events.OperationDiff}).Debug("diff complete", "create", …, "update", …, "replace", …, "delete", …, "unchanged", …); result = d; return nil })`; return `nil, err` on error. Never touch `c.entities` or `c.stateStore.Save`.
- `config.go:140-143` — mention Diff in the Config doc comment's list of operations; `config.go:477` run doc comment lists Diff.
- `config_diff_test.go` (new) — fixture helper modelled on `setupDestroyConfig` (`config_destroy_test.go:41-80`): registry + `parser.TestPlugin` + `state.NewFileStateStore(t.TempDir())` + event recorder + `WithEventData(EventDataRaw)` + config copied to a temp dir. Tests (one behaviour each): identical config → zero counts, empty `Resources`; state file bytes (`os.ReadFile(store.Path()…)`) identical before/after a diff reporting changes; `Entities()` unchanged after diff; drift via `SetReadObserved` → update; mixed → 2/1/1/1 and unchanged count; one changed + two unchanged → one listed, `Unchanged == 2`; events: first is `diff` start, last is `diff` success, read and changed lifecycle events present, no create/update/destroy lifecycle events; log: a `PhaseLog` event from the core with message "diff complete"; provider read log via `SetLogOnRead` arrives with operation read; negative tests each in their own function: no paths, invalid config (`internal/test_fixtures/config/invalid/no_name.xcl`), empty config (`ErrEmptyConfiguration`), state that cannot be loaded.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Compute field-level changes between saved and configured resources

Requirements carried: Field-level changes are reported, Sensitive values stay hidden in the result. Repo: xclconfig.

**File changes**:
- `internal/parser/diff_changes.go` (new) — `func resourceChanges(action diff.Action, saved, configured any, body *hclsyntax.Body, unknown []diff.Path, reveal bool) []diff.Change`. Walk `structFields` (`internal/parser/computed.go:68`) of the dereferenced type in declaration order, skip `isComputed` (line 47) and `ResourceBase`. Leaf (`blockElement(field.Type) == nil` and not a slice/map) → compare with `reflect.DeepEqual` after normalising nil vs empty slices/maps; `isSensitiveType` (line 31) leaves are compared whole. Struct / pointer-to-struct block → descend, `Path.Attribute(name)`; nil vs non-nil pointer block → one added or removed entry. Slices (of blocks or scalars) → by index only (do NOT use `pairElements`, line 233): common indices descend/compare, extra configured indices → added entry (`After` only), extra saved → removed entry (`Before` only). Maps → by sorted key, `Path.Key(k)`, added/removed likewise. `cty.Value` fields → leaf, compared with `Equals`, value via ctyjson→`any`. `create`: for each non-computed top-level field set in `body` (attribute or block named by the xcl name) or non-zero after decode, one entry with `After` only; nested blocks whole. `delete`: nil. Value conversion `plainValue(reflect.Value, reveal) any`: structs → `map[string]any` keyed by xcl names (skipping computed fields and ResourceBase), slices → `[]any`, maps → `map[string]any`, pointers dereferenced (nil → nil), `types.SensitiveValue` → `RevealAny()` converted (only reached when reveal is true). Sensitive rule: a changed sensitive leaf → `Change{Path, Sensitive: true}` plus values only when `reveal`; a whole added/removed/created value containing a sensitive leaf (detect by type via `isSensitiveType` recursively) is split into child entries until each sensitive leaf stands alone. Unknown rule: for each path in `unknown`, emit `Change{Path, Unknown: true}` (with `Before` from saved on update/replace) and skip comparing anything at or under that path; a created whole value containing an unknown path is split until the unknown stands alone.
- `internal/parser/diff_changes_test.go` (new) — unit tests against `internal/test_fixtures/plugin/structs` types (`Container`, `Network`, `Credential`) built directly in Go, one behaviour per test: single leaf change; computed field (`AssignedAddress`, `ImageID`, `ProviderID`) never listed; nested block path `run_as.user`; list by index (removing `port[0]` changes every later port); added list element; removed list element; map key change `env["LOG_LEVEL"]`; added and removed map key; nil equals empty; create lists configured fields only; delete empty; sensitive change masked; sensitive reveal; created credential masks password and pin; unknown path override; unknown inside added block split.

**Complexity**: High
**Token estimate**: ~45k tokens
**Agent strategy**: Parallel analysis, sequential integration: write the test cases first from the design's rules, then implement the comparator to them.

### Task: Report value changes in diff results

Requirements carried: Changed resources are reported (with values), New resources are reported (with values), Field-level changes, Sensitive values stay hidden, Result is inspectable by code. Repo: xclconfig.

**File changes**:
- `internal/parser/lifecycle.go` (diff step from the parser-walk task) — call `resourceChanges` when recording: create → `(ActionCreate, nil, r, body, unknown, reveal)` with `r` as decoded; update → `(ActionUpdate, old, configured, …)` where `configured` is the pre-read snapshot (`configuredData` from `refresh`) unmarshalled into a fresh value of `r`'s type; replace → `(ActionReplace, old, r, …)`; not-found create uses the restored configured values. `body` from `l.bodies[meta.ID]` (line 38); reveal from the stored `diff.Options`.
- `config_diff_test.go` — add: added resource → create with its configured values; changed attribute → update listing exactly that attribute with before/after; drift only → update with no changes; caller enumerates address, action, path string, before, after (no string parsing); sensitive change using the `credential` type (a fixture `internal/test_fixtures/config/diff/credential/main.xcl` with `password = variable.db_password`, variable sensitive) → change has `Sensitive: true`, nil Before/After, JSON lacks both secrets; with `diff.RevealSensitive()` both values present and `Sensitive` still true.
- `sensitive_leak_test.go:202-330` — add diff cases following the existing leak pattern (`knownSecret`, `leakFixture`): unrevealed diff JSON, `fmt.Sprintf("%v")` and `"%+v"` of the result, and every event recorded during the diff hold no `knownSecret`. Target the field or use the distinctive secret, per the temp-path gotcha.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Mark values known only after apply

Requirements carried: Unknown values are marked; Existing resources are refreshed (except those depending on unknowns). Repo: xclconfig.

**File changes**:
- `internal/parser/context.go:15` — `buildContextForResource` gains a trailing `unknown unknownValues` parameter (nil from apply). At line 93, after `convert.GoToCtyValue(resource)`, and for outputs (line 87) and variables (line 90), if `unknown != nil` set `ctyRes = unknown.contextValue(resource, ctyRes)`.
- `internal/parser/diff_unknown.go` (new) — `diffRecorder` implements `unknownValues`: if the entity is pending, replace every computed field (walk `computedFields`, `computed.go:138`, matching object attributes by xcl name through lists/maps of blocks) with `cty.UnknownVal(attr type)`; then replace each recorded unknown path for that entity. Use `cty.Transform`/rebuild of object/tuple/list/map values; never mutate the Go entity.
- `internal/parser/diff_decode.go` (new) — `decodeForDiff(body, ctx, entity) ([]diff.Path, hcl.Diagnostics)`: copy the body shallowly (new `Attributes` map, new `Blocks` slice with copied blocks whose bodies are processed recursively; nested block occurrence index gives `Path.Index` for list-of-block fields, matching how `nestedBlockBody` counts, `configured_check.go:165`); for each attribute evaluate `attr.Expr.Value(ctx)`; if not `IsWhollyKnown()`, collect unknown sub-paths with `cty.Walk` (object attr → `Attribute`, list/tuple index → `Index`, map key → `Key`), replace each unknown with a placeholder of its type (`""`, `cty.Zero`, `cty.False`, empty collection; null for `DynamicPseudoType`) and set the copied attribute's `Expr` to `&hclsyntax.LiteralValueExpr{Val: v, SrcRange: attr.Expr.Range()}`; then `gohcl.DecodeBody(copy, ctx, entity)`. The parsed body in `parsed.bodies` is never modified.
- `internal/parser/callbacks.go:80` — pass `lifecycle.recorder` (nil in apply) to `buildContextForResource`.
- `internal/parser/callbacks.go:124` — in diff mode, for non-builtin entities (`resources.TypeVariable`, `TypeOutput`, `TypeModule` keep `gohcl.DecodeBody` because their `cty.Value` fields hold unknowns natively) use `decodeForDiff`, then `recorder.recordUnknown(meta.ID, paths)`.
- `internal/parser/callbacks.go:145-168` — module `Variables.Value(ctx)` keeps unknowns, no change needed beyond the context hook; verify.
- `internal/parser/callbacks.go:189-195` — skip `convertOutputValue` when `!out.CtyValue.IsWhollyKnown()`; for outputs, record the unknown paths of `CtyValue` so dependents via `output.x` see them.
- `internal/parser/lifecycle.go` (diff step) — before refresh: if `recorder.unknownPaths(meta.ID)` is non-empty and the saved status is created/updated, record `update` with no provider call (and do not mark pending); for create and replace pass the unknown paths to `resourceChanges`.
- `internal/test_fixtures/config/diff/unknown_ref/{before,after}/main.xcl` (new) — before: network `one` and container `user` with a literal network name; after: adds network `two` and changes container `user`'s network name to `resource.network.two.provider_id`. Also `unknown_create/main.xcl` reusing the shape of `internal/test_fixtures/config/lifecycle/computed_ref/computed_ref.xcl`, and `unknown_output/main.xcl` routing a computed value through an output and a module variable.
- `internal/parser/diff_unknown_test.go`, `internal/parser/diff_decode_test.go` (new) — referencing value unknown with no read recorded for `resource.container.user`; create with unknown network name and concrete other values; unknown inside an added block split to `network[0].name`; propagation through output and module; decode leaves the parsed body unchanged; apply mode unaffected (existing computed_ref apply test still passes).
- `config_diff_test.go` — the acceptance-criterion test "Unknown values are marked and not read" through `Config.Diff`.

**Complexity**: High
**Token estimate**: ~60k tokens
**Agent strategy**: Parallel analysis, sequential integration: one agent on the context hook and propagation, one on the decoder; integrate into the walk callback sequentially, then run the full test suite.

### Task: Prove diff matches apply end to end

Requirements carried: success metrics (diff matches apply; diff changes nothing; no sensitive value without reveal). Repo: xclconfig.

**File changes**:
- `e2e/diff_test.go` (new, package `e2e_test`) — public packages only. Copy `testdata/plugin` (`e2e/helpers_test.go:21`) and `testdata/kube` into `t.TempDir()` so scenarios can edit files; build configs with `newPluginConfig` (`e2e/helpers_test.go:72`) and a `testutil.EventRecorder`. Scenarios, each its own test: first diff then apply (all create; the app's reference to the redis connection string is unknown); unchanged re-apply (zero changes); edited attribute (update); removed block (delete); failed resource (replace) where the fixture supports producing one, otherwise omitted for the plugin fixture with the reason in a comment. For each: snapshot state file bytes, run `Diff`, assert bytes unchanged and no create/update/destroy lifecycle events during the diff, then `Apply` and derive per-action address sets from the apply's lifecycle success events (create without a destroy of the same resource → create; destroy then create → replace; update → update; destroy without create → delete) and `require.Equal` them with the diff's sets. Kube fixture (registered types only): diff reports nothing, apply hands nothing to a provider.
- `e2e/diff_test.go` — leak test: plugin fixture secrets (the database password variable value used by `newPluginConfig`) absent from unrevealed diff JSON, formatted output and diff events, including after changing the password.
- `e2e/COVERAGE.md` — add a short section listing the new diff tests (documentation of the suite, not a test).
- `internal/testutil` — only if a helper is needed by both root and e2e tests (e.g. copying a fixture directory); otherwise keep helpers local to `e2e`.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Single agent, sequential execution.

## Testing Strategy

Per task:
- **Add the diff result types package** — unit tests in `diff/` for every path form, JSON of a path, `Changed()`, option resolution, and the design's example JSON.
- **Share the read-and-compare step between apply and diff** — the existing `internal/parser` lifecycle, event and state tests are the regression guard and must pass unmodified; new focused tests exercise `refresh` alone (not found, changed via drift, unchanged, no create/update calls).
- **Add the diff walk to the parser** — `internal/parser/diff_test.go` integration tests with `TestPlugin` and real applies for state: no create/update/destroy calls, the exact read set, every action, provider-less entities excluded, provider errors naming the resource and matching with `errors.Is`, state bytes unchanged.
- **Add Config.Diff as a public operation** — `config_diff_test.go`: one test per acceptance criterion reachable without value changes (empty result, state bytes, entities untouched, drift, mixed counts, unchanged count, operation and lifecycle events, logs) and separate negative tests for each Apply-equivalent failure.
- **Compute field-level changes between saved and configured resources** — `internal/parser/diff_changes_test.go`: pure unit tests of every comparison rule, sensitive masking and reveal, unknown overrides and splitting.
- **Report value changes in diff results** — `config_diff_test.go` additions for create values, single-attribute update, drift without changes, programmatic enumeration, sensitive masked and revealed; `sensitive_leak_test.go` diff cases.
- **Mark values known only after apply** — `internal/parser/diff_unknown_test.go` and `diff_decode_test.go`: unknown marking without a read, unknown in a create, splitting inside an added block, propagation through outputs and modules, parsed body untouched, apply unaffected; one `config_diff_test.go` acceptance test.
- **Prove diff matches apply end to end** — `e2e/diff_test.go`: diff-vs-apply parity per scenario and fixture, state and resources untouched, no secret without reveal.

Overall strategy (from the plan's Testing Approach):

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

## Project References

- Spec: `20261007105731-2388b579-diff` (read with `spektacular spec file read`).
- Design: `config-diff.md` from the `design` source (binding; read with `spektacular design read --data '{"source":"design","path":"config-diff.md"}'`).
- Sibling spec building on this plan: `20261007111826-cf3b66d8-diff-rendering-and-docs`.
- Knowledge entries (repo `xclconfig`): conventions/code-style, conventions/project-structure, conventions/testing-and-mocking, conventions/test-state-from-real-apply, conventions/shared-test-helpers, conventions/assert-ordering-on-graph-parents, conventions/never-modify-dependencies, conventions/development-standards, architecture/shared-public-types-live-in-types, gotchas/cty-unknown-args-drop-marks, gotchas/event-data-includes-temp-paths, gotchas/sensitive-values-leak-through-fmt, glossary/entity.
- Repo root: `xclconfig` at `/home/nicj/code/github.com/jumppad-labs/xcl` (all tasks).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Tasks in dependency order: types (Low) and refresh split (Medium) can run in parallel; parser walk (High) then `Config.Diff` (Medium); comparator (High) can run in parallel with the walk once the types land; value changes (Medium); unknowns (High); e2e (Medium). Re-read the design before the comparator and unknown tasks.

## Migration Notes

None. `Config.Diff`, the `diff` package and `events.OperationDiff` are additions; the read-path split and the extra parameters on internal parser functions do not change apply's behaviour or any public API. Saved state, the plugin wire format and event payloads are unchanged.

## Performance Considerations

A diff costs one parse and one Read plus one Changed call per existing, still-configured provider resource — the same provider work as the read half of an apply, with no Create, Update or Destroy. The comparator walks each changed resource's fields once; the diff decoder evaluates each attribute once more than a plain decode, only in diff mode. Apply's performance is unchanged.
