# Phase 1.1 — consolidated edit list

Repo: `xclconfig` @ `/home/nicj/code/github.com/jumppad-labs/xcl`.
Built from four parallel research agents (registry/dispatch, lifecycle/callbacks, state, logging/tests),
every claim re-verified against source by the driving agent. **Zero line-number drift** across all
plan-named sites.

Both axes are `string`: the build is NOT a safety net. Work this list exhaustively.

## Decisions taken (deviations from `context.md` are marked ▲)

1. **New field**: `Subtype string \`xcl:"subtype,optional" json:"subtype,omitempty"\`` on `types.Meta`,
   mirroring `Module`'s style (internal, string, legitimately empty). `omitempty` is load-bearing —
   dropping it breaks `plugins/adapter_test.go:114,182` (`require.JSONEq`, exact match).
   `Type`'s doc comment at `types/resource.go:15-16` must be rewritten; it currently claims to be
   "the text representation of the golang type".

2. ▲ **New helper `func (m *Meta) AddressType() string`** in `types` — returns `Subtype` when
   non-empty, else `Type`. This is the address segment an entity is reached by, and it is what the
   old conflated `Type` field used to hold. Needed because four packages (`state`, `internal/parser`,
   `logger`, and indirectly `plugins/registry`) need the same fallback; duplicating it unexported in
   each is worse. NOT in the plan.

3. ▲ **`handledWithoutProvider` signature changes to take `*types.Meta`**, not a string. This is the
   ONLY edit in the whole phase the compiler will enforce — it breaks both call sites and forces a
   human read. Two string parameters would compile even transposed. NOT in the plan.

4. ▲ **`types/register.go:32` keeps `Type = resourceType, Subtype = ""`** (correct for builtins, DAG
   roots and `createBuiltinResource`), and the registered-Go-type case is corrected at the
   `plugins/registry` layer instead. The plan says this line "serves builtins"; it does not — it also
   serves `registeredTypes`, which are resource varieties. `types` cannot import `internal/resources`
   (cycle), so it cannot tell the two apart; the fix must live where both are visible.

5. ▲ **The `"dummy"` builtin probe is replaced, not field-swapped.** Authorised without approval by
   plan Open Question #3.

6. ▲ **Three state sites are a substitution, not an addition.** `context.md` says of `state.go:59,119,
   195,224`: "each needs the variety added to the comparison, not substituted for the kind." True only
   for `:59`. `FQRN.Type` carries the identical conflation `Meta` does (`fqrn.go:78` parses the variety
   into it; `fqrn.go:202` renders it in the variety slot) and does not gain its own axis until Phase
   1.3, so at every Meta↔FQRN boundary the comparison must use `AddressType()`.

## Edits

### types
- `types/resource.go:17` — add `Subtype` after `Type`; rewrite `Type`'s doc comment.
- `types/resource.go` — add `AddressType()` method + doc comment.
- `types/register.go:32` — unchanged behaviour; add `meta.Subtype = ""` explicitly for clarity.

### plugins/registry
- `plugin_registry.go:79` — after `registeredTypes.CreateResource` succeeds, set
  `meta.Type = types.TypeResource; meta.Subtype = resourceType`. (The builtin branch at `:74` is
  already correct.)
- `plugin_registry.go:163` (`createResourceFromPlugins`) — set BOTH from the matched `RegisteredType`:
  `meta.Type = t.Type; meta.Subtype = t.SubType`. Both values are in scope from the `:149` loop;
  verified the wire populates them separately (`plugins/plugin.go:118-123`), so no plumbing needed.
- `plugin_registry.go:182` (`GetProvider`) and `:332` (`GetProviderForResource`) — read `meta.Subtype`
  (both match against `t.SubType`). Near-duplicates; do NOT unify — explicitly out of scope.
- `plugin_registry.go:184-187` — replace the create-and-see probe with
  `if meta.Type != types.TypeResource { return nil }`. Left on `Type` it returns nil for every
  plugin-backed resource (silent total loss of provider dispatch); moved to `Subtype` it becomes dead
  code that would mis-fire for any plugin registering an empty SubType.
- The six hard-coded `t.Type == "resource"` comparisons (`:100,117,150,195,339`, plus
  `plugins/direct_plugin_host.go:47`) all read `RegisteredType`, not `Meta` — **no change**.

### internal/parser
- `parser.go:565-575` (`blockResource`) — set both axes: `Type = b.Type` always;
  `Subtype = b.Labels[0]` for the two-label resource case; `Subtype = ""` for the one-label case.
- `parser.go:610` — `CreateResource(b.Labels[0], name)`: variety must reach `Meta.Subtype`,
  `Meta.Type` becomes `types.TypeResource`. (Handled by the registry fix above; verify.)
- `lifecycle.go:352-365` — `handledWithoutProvider(typeRegistry, meta *types.Meta)`; builtin arms stay
  on `meta.Type`, `IsRegisteredType` fallback moves to `meta.Subtype` (`registeredTypes` is keyed by
  variety — confirmed at `plugin_registry.go:57` / `checkTypeName:99`).
  Both wrong answers compile and are silent: all-`Type` breaks every registered type; all-`Subtype`
  breaks every builtin and both DAG roots.
- `lifecycle.go:75`, `callbacks.go:212` — pass `meta` / `rMeta`.
- `lifecycle.go:367-369` (`resourceType`) — use `meta.AddressType()`. Single choke point, three callers
  (`lifecycle.go:76,328`, `callbacks.go:203`); `:328` is NOT in the plan but inherits the fix.
  The fallback is required, not cosmetic — builtins and DAG roots pass through here and would
  otherwise emit `".name"`.
- `lifecycle.go:82`, `callbacks.go:229` — `"no provider found for resource type %s"` → `AddressType()`;
  the lookup that just failed was variety-keyed, so printing `resource` is useless.
- `callbacks.go:48,93,135,169,198` — all **stay on `Type`** and become strictly more correct.
  `:169`'s switch guards the unguarded `r.(*resources.Output)` assertion at `:171`; moving it to
  `Subtype` silently never fires (Output's Subtype is empty) and `out.Value` is never populated.
  Same argument for `:135`/`:136` and `*resources.Module`.
- `util.go:275-297` — stays on `Type`; both cases are kinds guarding unguarded assertions, and the
  `default` arm is exactly where varieties must land.
- `util.go:124-127`, `configured_check.go:199`, `dag.go:50,132` — **unchanged**, verified.
  `util.go:529` is `FQRN.Type`, not `Meta.Type` — Phase 1.3's problem.
- `parser.go:1035-1037` (cycle detection, `rMeta.Type == fqrn.Type`) — use `AddressType()`.

### state
- `state.go:59` (`RemoveResource`) — add `rfMeta.Subtype == rMeta.Subtype` as an AND. Plan rule correct.
- `state.go:119` (`FindResourcesByType`) — ▲ disjunction, not AND: `meta.Type == t || (meta.Subtype != "" && meta.Subtype == t)`.
  `t` is one string and callers pass either axis. No non-test caller exists, so blast radius is tests.
- `state.go:195` (`addResource`) — ▲ `Type: meta.AddressType()`, else `meta.ID` becomes
  `resource.resource.main` and poisons what is written to disk.
- `state.go:224` (`findResource`) — ▲ `meta.AddressType() == fqdn.Type`. Left as an AND, **every**
  resource lookup returns not-found, silently breaking `lifecycle.go:85`, `parser.go:344`,
  `progress.go:65`, `util.go:555,565`, `config.go:63`. Highest-severity site in the phase.
- `file_state_store.go:62-73` — read `metaMap["subtype"]` alongside `metaMap["type"]`; feed the variety
  to `CreateResource` for resource-kind entries. Without this, a state file written after the split
  holds `"type":"resource"`, which is not a creatable type, and `Load` fails wholesale — breaking this
  phase's own "re-apply behaves as before" criterion. Leave the *reporting* of unresolvable records to
  Phase 1.5; do not pre-empt it.

### logger (display only)
- `pretty_printer.go:567,833` — `getResourceEmoji(meta.AddressType())`.
  ▲ `getResourceEmoji` (`:750-769`) switches over BOTH axes: `container|network|volume|template` are
  varieties, `variable|output|module` are kinds. A single-field call breaks half the table, untested
  (there is no `pretty_printer_test.go`).
- `pretty_printer.go:834` — `strings.Title(meta.AddressType())`; otherwise every card header reads
  `🐳 Resource: consul`.
- `pretty_printer.go:295,582` — the `{"Type", meta.Type}` rows: show both axes.

## Tests

Plan-named:
- `plugins/registry/plugin_registry_test.go:124,148,299` — expectation moves to `Subtype`; also assert
  `Type == types.TypeResource` so the new axis is covered at the site where the data was being discarded.
- `internal/parser/parser_plugin_test.go:40` — `"person"` moves to `Subtype`; assert `Type` too.
- `internal/resources/default_test.go:32` — stays on `Type`; add `require.Empty(..., Meta.Subtype)`.
- `types/resource_helpers_test.go:52,63,70,81,92,99,191` — synthetic sentinels, no axis change; extend
  to exercise the new field.
- ▲ `config_test.go:364,400,433` — **DEAD CODE.** `/*` at line 33, matching `*/` at 456 (the `/*` at
  347 is inert — Go block comments do not nest). Not compiled, not run. Do not chase them.

NOT in the plan — found by the completeness sweep:
- ▲ `internal/parser/types_test.go:186` — **hard CI failure.** Asserts the exact sorted slice
  `{"column","file","id","line","module","name","type"}`; adding the `xcl:"subtype"` tag adds an eighth
  entry. Must become `{..., "subtype", "type"}`. Highest-probability "went red unexpectedly" item.
- ▲ `types/resource_helpers_test.go:198` — `require.Equal(t, "new-type", meta2.Type)`. The plan's grep
  `meta\.Type` structurally cannot see the receiver `meta2`; editing `:191` without this splits the
  test across two axes.
- ▲ `state/file_state_store_test.go:103,106,109,120` — varieties sitting in `"type"` in the only
  round-trip fixtures covering the JSON reconstruction this phase depends on.
- ▲ `plugins/example/e2e_test.go` — 10 JSON literals say `sub_type`, which matches no field (inert,
  green by accident). The tag is `subtype`. Also `:65-69` is a half-split `Meta` literal with the
  variety stuffed into `Name`.
- ▲ `Meta` literals carrying a variety in `Type`: `internal/parser/registered_types_test.go:439-442`,
  `internal/parser/progress_test.go:17-20`, `internal/schema/debug_test.go:21-24`.
- ▲ `internal/parser/lifecycle_test.go:667` — reads `meta["type"]` from decoded state JSON; stays
  correct post-split. Consciously skip; "fixing" it to `subtype` would break it.
- `plugins/adapter_test.go:114,182` — tripwire for the `omitempty` decision, no edit.
- `internal/resources/fqrn_test.go` — 12 assertions on `FQRN.Type`, all on builtin kinds, all survive.
  Phase 1.3 should add variety-bearing coverage; this file has none today.

House style: testify `require` everywhere; `assert` appears nowhere in first-party code.
`internal/test_fixtures/` needs no edits (no `.xcl` fixture references `meta.type`), and there are no
golden/snapshot files — every JSON expectation is an inline Go literal.

## Baseline (before any edit)

- `go build ./...` and `GOTOOLCHAIN=go1.25.0 go build ./...` both clean.
- `go test ./...`: exactly 3 failures, all `example/configonly` (`...DestroysEverythingItApplied`,
  `...PrintsNoResourcesRemaining`, `...LogsDestroySuccessWithoutStartAtDebug`) — pre-existing, fixed in
  Phase 3.1. Everything else green. `plugins/example` passes only because `build/example` exists
  locally; it is gitignored, so the plan's "fails on a clean clone" still holds.

---

## Implement step — outcome

Production code complete. `go build ./...` and `go vet ./...` clean.

**Phase 1.2 was folded in by user decision** (see working-context): `internal/parser/context.go`
namespace keys moved to `AddressType()`. Without it Phase 1.1 cannot meet its own criterion
"applying/re-applying/destroying behave exactly as before" — ~40 tests failed with
`This object does not have an attribute named "application"`, the expression language keying on
`"resource"` instead of the variety. Folding it in took the suite from ~40 failures to 10.

**Extra edit not in the plan's Phase 1.1 list**: `internal/resources/fqrn.go:175`
(`FQRNFromResource`) copies `meta.Type` into `FQRN.Type`, feeding `meta.ID` at `parser.go:706`.
Same Meta↔FQRN boundary class as `state.go:195`; left alone it makes every ID
`resource.resource.<name>`. The plan files it under Phase 1.3, but it breaks immediately.

### Remaining failures, all test-side (for the `test` step)

| Test | Fix |
|---|---|
| `example/configonly` ×3 | **Pre-existing baseline**, Phase 3.1. Not ours. |
| `internal/parser/types_test.go` `TestPropertyNamesForMeta` | add `"subtype"` to the expected sorted slice |
| `internal/schema/serialize_test.go:191` `TestSerializeEmbedded` | ▲ **NOT found by any agent** — a second golden property-list for `Meta`; add the `Subtype` entry |
| `plugins/registry` ×3 (`:124,148,299`) | expectation moves to `Subtype`; also assert `Type == types.TypeResource` |
| `internal/parser` `TestPluginRegistration` (`parser_plugin_test.go:40`) | `"person"` moves to `Subtype` |
| `internal/parser` `TestDestroyWalkSkipsProviderForRegisteredType` | `registered_types_test.go:439-442` hand-builds `Meta{Type: registered.TypeDatabase}` — the pre-split shape. Must become `Type: types.TypeResource, Subtype: registered.TypeDatabase`, else `IsRegisteredType("")` is false and it wrongly reaches a provider |

Still to do in the test step (found by sweep, not yet failing):
`types/resource_helpers_test.go:198` (`meta2.Type`), `state/file_state_store_test.go:103,106,109,120`,
`plugins/example/e2e_test.go` `sub_type`→`subtype` ×10 and the `:65-69` literal,
`internal/parser/progress_test.go:17-20`, `internal/schema/debug_test.go:21-24`.
