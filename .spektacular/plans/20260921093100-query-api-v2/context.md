---
created_date: "2026-09-21"
document_status: final
closed_date: "2026-09-21"
---

# Context: 20260921093100-query-api-v2

## Current State Analysis

**Where the code lives.** Two registered repos. `xclconfig` at `/home/nicj/code/github.com/jumppad-labs/xcl`
is the Go library and carries all but one phase. `xcl-website` at
`/home/nicj/code/github.com/jumppad-labs/xcl-website` is the Astro 5 + MDX + Tailwind v4
documentation site served at xcl.dev, and carries Phase 4.2 only. Planning was done against
commit `13e0311` on branch `main`.

**The surface being replaced.** `querier.go` is 90 lines holding exactly five symbols —
`NewQuerier` (`:12`), `Querier[T]` (`:16-18`), `FindResource` (`:26`), `FindResourcesByType`
(`:51`) and `asType` (`:82`). `asType` has no caller outside that file, so the file lifts out
whole. Three defects in it drive this work:

- `querier.go:35` matches an address by plain string equality with no normalisation and no module handling.
- `querier.go:32,59` **panic** when metadata cannot be read, where `state/state.go:115-118` handles the identical error with `continue`.
- `querier.go:87-89` returns the partially-filled value *alongside* its error, because `internal/schema/unmarshal.go:5-20` is plain `encoding/json` with no `DisallowUnknownFields` — unknown fields drop silently and absent fields zero.

**Why the kind axis returns silently empty.** `querier.go:62` matches `meta.Type == typeName`, but
`Meta.Type` holds the *variety* for a `resource` stanza and the *stanza keyword* for everything
else. `internal/parser/parser.go:565` seeds the type from the block keyword and `:568-570`
overwrites it with `b.Labels[0]` for `resource`; `types/register.go:32` and
`plugins/registry/plugin_registry.go:163` both write `meta.Type = resourceType`. The word
`resource` therefore never appears in `Meta.Type` at all.

**The blast radius of splitting that field.** `Meta.Type` (`types/resource.go:17`) is a single key
driving all of: HCL expression namespacing (`internal/parser/context.go:126,136` — this is how
`resource.postgres.main` resolves in a configuration), state-file identity and JSON round-trip
(`state/file_state_store.go:63-73`, `state/state.go:195,224`), provider dispatch against
`RegisteredType.SubType` (`plugins/registry/plugin_registry.go:182,332`), the provider-less
short-circuit (`internal/parser/lifecycle.go:75,356-365`), the resource `ID`
(`state/state.go:199`, `internal/parser/parser.go:706-707`), and every parser event string
(`internal/parser/lifecycle.go:368-370`). Roughly thirty production read sites plus around fifteen
test sites. **Both axes are strings, so none of these produces a compiler error when left on the
wrong one.**

**What the plugin boundary already does right.** `plugins/plugin.go:32-42` defines
`RegisteredType{Type, SubType, Schema, Adapter}`, and the proto carries `entity_type` /
`entity_sub_type` (`plugins/proto/plugin.pb.go:168,272,384,488,618,730`). The type-and-subtype
taxonomy this plan adopts is already the wire format's vocabulary; only `Meta` conflates it. Every
caller passes `"resource"` as the top-level type, and six registry and host sites hard-code
`t.Type == "resource"`.

**Addressing.** `internal/resources/fqrn.go:12-22` has four fields and no variety slot.
`internal/resources/fqrn.go:60` is the regex where `resource` appears as one alternative among
`resource|output|local|variable` — a sibling, not a parent. The literal `resource.` prefix is
hard-coded in both formatters (`:202`, `:223`). Lookup already ignores the attribute suffix —
`state/state.go:223-225` matches only `Module`/`Type`/`Name` — so attribute-suffixed addresses
already resolve. Two pre-existing oddities to leave alone: `variable` addresses may not carry a
trailing attribute (`:108-116`), and `local` has no constant and no Go type anywhere, so a parsed
`local.x` currently formats back out as `resource.local.x`.

**Registration and type sources.** `PluginRegistry` (`plugins/registry/plugin_registry.go:17-23`)
draws from three sources: builtins, `RegisterType` Go types held as concrete pointer prototypes in
`types.RegisteredTypes` (`types/register.go:20`, instantiated by `reflect.New` at `:26`), and
plugin-provided types that exist host-side **only as JSON schema**
(`plugin_registry.go:150-152`) — which is why deriving addressing by reflecting a Go type cannot
serve them. `IsRegisteredType` (`:64-67`) covers only the middle source, despite its name.
`checkTypeName` (`:89-104`) already scans all three, to reject clashes.

**The error convention.** `plugins/errors.go:5-10` is the archetype and its doc comment states the
convention; identity survives the plugin wire through a dedicated field
(`plugins/grpc_server.go:133`, `plugins/grpc_plugin_host.go:179-183`). It is, however, the **only**
error in the repo implementing the pattern end to end. Nothing in first-party code implements `Is`,
`As` or `Unwrap`; receivers are inconsistent (value in `state/errors.go:13,21,35`, pointer in
`plugins/registry/errors.go:17` and `types/register.go:8-18`); and `types.ErrTypeNotRegistered` is
formatted with `%s` rather than wrapped (`types/register.go:38`) and is matched nowhere.
`state.ResourceNotFoundError` (`state/errors.go:8-15`) has no `Is`, and its one field carries three
different kinds of string across its seven construction sites.

**Build, toolchain and CI.** `go.mod:1,3` — `module github.com/jumppad-labs/xcl`, `go 1.25.0`, no
`toolchain` directive, no `replace`, one module covering `example/*`. Installed toolchain here is
**go1.27.0** with `GOTOOLCHAIN=auto`; 1.25.0, 1.25.3, 1.25.5, 1.25.6 and 1.26.0 are cached in
`~/go/pkg/mod/golang.org/`, so `GOTOOLCHAIN=go1.25.0` resolves with no network fetch. A plain
`go build` uses 1.27.0 and proves nothing about the floor. `.github/workflows/go.yml` is the only
workflow: one `build` job, **pinned to Go 1.22** (`:22`) which is *below* the module's own floor, no
vet, no lint, no matrix, and no precedent for a second job. There is no linter config anywhere, and
the root `Makefile` has no build or test target — it is codegen only.

**Two pre-existing failures, both confirmed on HEAD.** `example/configonly/main.go:61-120` never
calls `c.Destroy()` and never prints `## Destroyed`, though its doc comment at `:12-13` says it does
and three of its tests assert it (`main_test.go:447-465,467-474,479-493`) — **this plan fixes it**
(Phase 3.1). Separately, `plugins/example/e2e_test.go` needs a pre-built binary at `./build/example`
with no `TestMain` build step and no skip, while that directory is gitignored — so `go test ./...`
fails on a clean clone. **This plan does not fix that**, by decision; it is why the floor CI job is
scoped to building rather than testing.

**Migration surface, measured.** 15 call sites of the superseded helper, exactly as the design
claims: 8 in examples (`example/appconfig/main.go:88`, `example/configonly/main.go:127,173,181`,
`example/plugin/main.go:124,134,144,153`) and 7 in `querier_test.go:69,94,116,132,147,167,184`. The
README's `### Querying resources` is `README.md:267-284`, and the README covers reading published
values **nowhere** — every `output` mention is authoring-side, and `Output` lives in
`internal/resources` so a consumer cannot even name the type. On the site: 6 lines across 4 `.mdx`
files, which is every page; `GetResources` and `ResourceCount` appear zero times; two mentions are
lowercase prose invisible to a capital-Q search.


## Per-Phase Technical Notes

### Phase 1.1: Record kind and variety separately on every declaration

**Repo:** xclconfig (`/home/nicj/code/github.com/jumppad-labs/xcl`)

**File changes**
- `types/resource.go:17` — add `Subtype string` beside `Type`, with `xcl:"subtype,optional" json:"subtype,omitempty"` following the tag style of the surrounding fields. `Type` keeps its tag; its doc comment at `:15-17` must be rewritten, since it currently claims to be "the text representation of the golang type".
- `internal/parser/parser.go:565-575` (`blockResource`) — `:565` seeds `fqrn.Type` from the block keyword and `:569` overwrites it with `b.Labels[0]` for `resource`. Change so both axes are set: `Type = b.Type` always, `Subtype = b.Labels[0]` for the two-label resource case, `Subtype = ""` for the one-label case at `:571-572`.
- `internal/parser/parser.go:610` — `p.pluginRegistry.CreateResource(b.Labels[0], name)` is the only place a non-builtin type name is resolved; the variety must reach `Meta.Subtype` while `Meta.Type` becomes `types.TypeResource`.
- `types/register.go:30-33` — `CreateResource` writes `meta.Type = resourceType`. Must set both axes; this serves builtins (`variable`/`output`/`module`/`root`), which take an empty variety.
- `plugins/registry/plugin_registry.go:163` — `createResourceFromPlugins` writes `meta.Type = resourceType` for schema-built plugin resources. The plugin wire already carries the split as `RegisteredType{Type, SubType}` (`plugins/plugin.go:32-42`), so both values are in hand here — this is the one site where the correct data already exists and is being discarded.
- `plugins/registry/plugin_registry.go:182,195` (`GetProvider`) and `:332,339` (`GetProviderForResource`) — both read `meta.Type` and match it against `t.SubType`. Both must read `meta.Subtype`. These are near-duplicates of each other; **do not** unify them in this phase, it widens the diff on the riskiest change.
- `plugins/registry/plugin_registry.go:185-188` — `GetProvider` detects builtins by attempting a create with the name `"dummy"`; verify this still behaves once `Type` is uniform.
- `internal/parser/lifecycle.go:75,356-365` — `handledWithoutProvider(l.types, meta.Type)` compares against `TypeVariable|TypeOutput|TypeModule|TypeRoot` then falls back to `IsRegisteredType`. The builtin comparisons stay on `Type`; the registered-type fallback must move to `Subtype`. Getting this backwards routes registered types to a provider that does not exist.
- `internal/parser/lifecycle.go:82` (`"no provider found for resource type %s"`), `:368-370` (`resourceType(meta)` → `"%s.%s"` for every event string) — decide which axis each message shows; events should keep naming the variety, since that is what an operator recognises.
- `internal/parser/callbacks.go:48,93,135,169-175,198,212,229` — kind comparisons against `TypeRoot`/`TypeModule`/`TypeOutput`. All stay on `Type` and become *more* correct, since `Type` now genuinely holds the stanza. `:171`'s unguarded `r.(*resources.Output)` assertion is safe only while `:169`'s switch is on `Type` — keep it that way.
- `internal/parser/parser.go:723` (eager `description` read for outputs), `internal/parser/parser.go:1035-1037` (cycle detection matching `rMeta.Type == fqrn.Type`), `internal/parser/util.go:275-297` (link resolution switching on `lMeta.Type`) — all stay on `Type`; the cycle-detection comparison must gain the variety alongside it.
- `state/state.go:59` (`RemoveResource` identity), `:119` (`FindResourcesByType`), `:195` (FQRN built from meta), `:224` (`findResource` match) — each needs the variety added to the comparison, not substituted for the kind.
- `logger/pretty_printer.go:295,567,582,833-834` — display only; `getResourceEmoji` and the card header should use the variety, since that is what distinguishes one resource from another to a reader.
- `querier.go:62` — the superseded helper's `meta.Type == typeName` match must be pointed at `Subtype` purely to keep it and its tests green until Phase 2.5 deletes it. Deliberately throwaway.
- Tests to update: `config_test.go:364,400,433`; `plugins/registry/plugin_registry_test.go:124,148,299`; `internal/resources/default_test.go:32`; `internal/parser/parser_plugin_test.go:40`; `types/resource_helpers_test.go:52,63,70,81,92,99,191`.

**Critical note.** Both axes are `string`, so every site above keeps compiling whether or not it is
changed. The build is not a safety net here. Work the list exhaustively rather than chasing compiler
errors, and treat `grep -rn 'meta\.Type\|Meta\.Type' --include='*.go'` (excluding `internal/xcl` and
`internal/cty`) as the checklist.

**Complexity**: High
**Token estimate**: ~55k tokens
**Agent strategy**: Parallel analysis, sequential integration. Fan out read-only agents over the
independent reader groups (registry/provider dispatch; lifecycle and callbacks; state; logging and
tests) to produce exact edit lists, then apply centrally in one pass so the intermediate states are
never committed half-migrated.

### Phase 1.2: Keep references between items resolving

**Repo:** xclconfig

**File changes**
- `internal/parser/context.go:126,136` — `resourceVars[resourceMeta.Type]` is the flat namespace key that makes `resource.<variety>.<name>` resolve in an expression. Must become `resourceMeta.Subtype` for resource-kind items.
- `internal/parser/context.go:108,118` — the module-scoped equivalent of the same keying; same change.
- `internal/parser/context.go:71` — skips `TypeModule`/`TypeRoot` when building the context; stays on `Type`.
- `internal/parser/context.go:77-83` — `switch resourceMeta.Type` with concrete assertions for `TypeOutput` (`CtyValue`) and `TypeVariable` (`Default`); stays on `Type` and becomes more correct.
- `internal/parser/context.go:27,175` — `ctx.Variables["resource"]` hardcodes the root namespace name; unchanged, but confirm it still lines up now that `Type` literally holds `"resource"`.
- `internal/parser/context.go:62` — `fqdn.Type == TypeVariable && fqdn.Module == ""`; unchanged.
- `internal/parser/exp.go:152` — the expression-root whitelist (`"resource"|"module"|"variable"|"output"`); unchanged, and now consistent with `Meta.Type`'s values.
- `internal/parser/configured_check.go:199` — `case "resource", "module":`; unchanged.
- Fixtures: exercise against an existing multi-reference fixture rather than a new minimal one, so the test covers resource→resource, resource→variable, resource→module-output and module-output→resource in one apply.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential. The edit is small and highly coupled; the cost here is
verification, not volume, and splitting it risks two agents disagreeing about which axis a namespace
key should use.

### Phase 1.3: Carry the variety as its own address segment

**Repo:** xclconfig

**File changes**
- `internal/resources/fqrn.go:12-22` — add `Subtype string` to the four existing fields.
- `internal/resources/fqrn.go:53-142` (`ParseFQRN`) — the `case "resource"` arm at `:72-81` currently takes `parts[0]` as type and `parts[1]` as name, requiring ≥2 segments at `:74-76`. It must now read variety and name, with the trailing attribute still joined from `parts[2:]` at `:78`. The regex at `:60` keeps `resource|output|local|variable` as the leading alternation.
- `internal/resources/fqrn.go:83-106` — the `local`/`output` arm; leave as is, including its index-selector handling at `:94-106`. Note `local` has no `TypeLocal` constant and no Go type anywhere, and currently formats back out as `resource.local.x`; **do not** try to fix that here, it predates this work and widening the change invites regressions.
- `internal/resources/fqrn.go:108-116` — the `variable` arm requires exactly one segment and errors otherwise; unchanged, and it means a variable address still cannot carry a trailing attribute.
- `internal/resources/fqrn.go:166-177` (`FQRNFromResource`) — copies `meta.Type` into `FQRN.Type` at `:175`; must copy both axes. It returns `nil` silently on error at `:172` — worth noting, not worth changing here.
- `internal/resources/fqrn.go:179-203` (`String`) and `:205-224` (`StringWithoutAttribute`) — `:202` and `:223` hardcode the literal `resource.` prefix and interpolate `Type` where the variety now belongs. Both must emit `<mod>resource.<Subtype>.<Resource><attr>`. The `TypeOutput`/`TypeVariable` branch at `:190-192` and the `TypeModule` branch at `:194-200` are unchanged.
- `internal/resources/fqrn.go:150-164` (`AppendParentModule`) — copies `Resource`/`Type`/`Attribute`; must carry `Subtype` too, or module-relative resolution loses the variety.
- `state/state.go:186-209` (`addResource`) — builds the FQRN from meta at `:192-196` and assigns `meta.ID = fqdn.String()` at `:199`; must include the variety so IDs carry it.
- `internal/parser/parser.go:706-707` — the other site assigning `meta.ID` from `FQRNFromResource(rt).String()`; same requirement.
- `state/state.go:93-104` (`FindRelativeResource`), `:136-141` (`FindModuleResources`) — both parse then re-render; confirm round-trip with the extra segment.
- Tests: `internal/resources/fqrn_test.go` — existing cases at `:61,75` (module), `:94-227` (output), `:258-285` (variable-from-resource) must keep passing; add round-trip coverage for the variety segment.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential. Parsing and formatting must move together — a split
between two agents produces exactly the asymmetry the round-trip criterion exists to catch.

### Phase 1.4: Accept a declaration led by its variety

**Repo:** xclconfig

**File changes**
- `plugins/registry/plugin_registry.go:89-104` (`checkTypeName`) — already scans all three sources (builtins `:90-92`, registered `:94-96`, every plugin host's types `:98-104`) to reject a clash. Invert into a new exported `KnownType(name string) bool` sharing that scan; `checkTypeName` then expresses itself in terms of it.
- `plugins/registry/plugin_registry.go:64-67` — leave `IsRegisteredType` exactly as it is. `internal/parser/lifecycle.go:356-365` depends on its current narrow meaning (registered Go types only); widening it silently changes provider routing.
- `plugins/registry/plugin_registry.go:43-57` — `RegisterType` keeps its kind-led meaning. Add `RegisterBareType(name string, resource any) error` alongside, sharing the same validation (`:44-47` pointer-to-struct, `:49-51` meta probe, `:53-55` clash check) and recording the declaration form so `TypePath` can report it in Phase 2.4. Storage today is a flat `types.RegisteredTypes` map at `:20`/`:57`, which has nowhere to record the form — it needs a parallel record or a small struct value.
- `internal/parser/parser.go:521-548` — the keyword `switch`. `TypeModule`/`TypeVariable`/`TypeOutput`/`types.TypeResource` arms stay. The `default` arm at `:539-548`, whose message at `:544` lists the four valid stanzas, becomes: ask `KnownType(b.Type)`; if known, treat as a bare-form declaration requiring exactly one label; otherwise error naming the keyword as not a known type.
- `internal/parser/parser.go:580-752` (`parseResource`) — needs a bare-form arm alongside `case types.TypeResource` at `:585-619`. It requires one label rather than two, validates the name via `:599-607`, and calls `CreateResource(b.Type, name)`, setting `Meta.Type = b.Type` and `Meta.Subtype = ""`.
- `internal/parser/parser.go:559-578` (`blockResource`) — the one-label arm at `:571-572` already handles this shape generically; confirm it produces the right address for a bare-form block rather than falling through to `default` at `:573-574`.
- `internal/parser/util.go:124-127` — rejects `resource|module|output|variable` as resource *names*; unchanged.
- `internal/xcl/` — expected to need **no** change; block-header parsing is generic. If it does, the fork's licence rules bind: keep the HashiCorp MPL header, add `// Modifications Copyright (c) Jumppad Labs`, record in `internal/xcl/UPSTREAM.md`.
- Test fixtures: new `.xcl` fixtures declaring the same variety in both forms. **Gotcha** (`knowledge:gotchas/hcl-halts-on-malformed-block-header.md`): the HCL parser halts at a malformed block header, so a negative fixture with two bad headers yields one diagnostic, not two — assert on diagnostic *content*, never on counts.

**Complexity**: High
**Token estimate**: ~40k tokens
**Agent strategy**: 2 parallel agents — one on the registry (`KnownType`, `RegisterBareType`, form
recording), one on the parser arms and fixtures — then sequential integration, since the parser
change cannot be verified until the registry query exists.

### Phase 1.5: Report saved state that can no longer be understood

**Repo:** xclconfig

**File changes**
- `state/file_state_store.go:63-73` — reads `metadata["meta"].(map[string]any)` at `:63` then `metaMap["type"].(string)` at `:69`; the `:70-73` branch **silently skips** the resource when the assertion fails. Replace the skip with collection into the existing failure path. Also read the variety here.
- `state/file_state_store.go:82-91` — `registry.CreateResource(resourceType, resourceName)` failures already accumulate into `unknownTypes` at `:87-89`; route the missing/invalid-kind case into the same accumulator so one error type covers both.
- `state/file_state_store.go:112` — `UnknownTypesError` is already raised from that accumulator; reuse it rather than adding a new error type. Its shape is `state/errors.go:26-40` (`Types []string`, value receiver).
- `state/file_state_store.go:94,100` — the re-marshal/`json.Unmarshal` round-trip overwrites what `CreateResource` set with what the file held, so both axes must be present in the written JSON for a load to reconstruct correctly. This is why Phase 1.1's tag choice on `Meta.Subtype` matters here.
- Tests: per `knowledge:conventions/test-state-from-real-apply.md`, produce the "current version" fixtures by a **real apply** with a file state store. The stale-file test is the documented exception — it is about the on-disk format itself, so hand-write a state file carrying the old single-axis shape.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential. Small, self-contained, and entirely within one file plus
its tests.

### Phase 2.1: One vocabulary for every way a lookup can fail

**Repo:** xclconfig

**File changes**
- `errors/query_errors.go` (new, package `errors` — the project's existing error package, alongside `config_error.go` and `parser_error.go`) — seven sentinels as package-level `var … = errors.New(…)`, each with a doc comment naming who returns it and that it is matched with `errors.Is`, following `plugins/errors.go:5-10` exactly. Seven detail structs with **pointer** receivers, each `Unwrap`ing to its sentinel: address/segments queried, the Go type involved, and the **count found** for the not-unique case (an explicit acceptance criterion).
- **Import cycle, resolved — do not wire this the obvious way.** `config.go:10` imports `xcl/state`, so `state` **cannot** import the root `xcl` package, and `ResourceNotFoundError.Is` therefore cannot reference a sentinel declared in `xcl`. Resolution: declare **all seven** sentinels in the project's existing `errors` package (`github.com/jumppad-labs/xcl/errors`), which already holds `ConfigError` (`errors/config_error.go:5-33`) and `ParserError` (`errors/parser_error.go:13-79`). Verified safe: that package imports only `internal/xcl` and `github.com/mitchellh/go-wordwrap` — not `state`, not `internal/parser`, not the root package — so `state -> errors` and `xcl -> errors` both introduce no cycle.
- **Re-export the sentinels from `xcl`**, following the precedent at `config.go:16` (`var ErrEmptyConfiguration = parser.ErrEmptyConfiguration`). This is ergonomics, not indirection: the package is *named* `errors`, so without the re-export every consumer matching one would have to alias-import it to keep the standard library's `errors.Is` usable. With it, a caller writes `errors.Is(err, xcl.ErrNotFound)` using stdlib `errors` and nothing else. Identity is preserved because a re-export is the same value, so a sentinel raised inside `state` matches the name a consumer holds from `xcl`.
- `state/errors.go:8-15` — add `func (r ResourceNotFoundError) Is(target error) bool` matching the sentinel declared in the same package. Keep the **value** receiver: `internal/parser/lifecycle.go:87-90` uses `errors.As` against the value type. **Inventory corrected during implementation**: there are eight construction sites, not seven (`state/state.go:50,71,128,174,230`, `config.go:61`, `querier.go:41,73`), and the `Resource` field carries four kinds of string across them — empty (×2), a bare type name (×2), a module name (×1) and an FQRN (×3) — which is why equality matching cannot work and an `Is` method is the only route to one not-found concept. Three of those sites disappear before this phase lands: `querier.go:41,73` go with the superseded helper in Phase 2.5, and `state/state.go:230` goes with `findResource` in Phase 5.1. The `Is` method still earns its place, because `state` keeps raising the error from `RemoveResource` (`:50,71`) and `FindModuleResources` (`:174`).
- `plugins/errors.go:5-10` — extend the doc comment to state that this condition means the **real infrastructure** is gone and is distinct from the configuration-lookup not-found, per the spec's "stays distinct" requirement.
- Avoid the trap at `types/register.go:38`, where `ErrTypeNotRegistered` is formatted with `%s` rather than wrapped and is consequently matched nowhere. Every new error wraps.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential. One new file plus two small edits; the design fixes the
taxonomy, so there is little to explore. Resolve the `state`→`xcl` import direction first, as it
constrains where the sentinels live.

### Phase 2.2: Look up one item by address, published values included

**Repo:** xclconfig

**File changes**
- **Shared address matcher (new, `internal/resources`)** — created here, because this is the first phase that needs it. A function taking a plain `[]any` and an address and returning the item it names. `internal/resources` is the right home: it already owns `FQRN`, imports only `types` plus stdlib, is already imported by `internal/parser`, and the root package can import it with no cycle (verified). It cannot live anywhere that would require handing the parser a `*Config` — `config.go:7` imports `internal/parser`, so the reverse is a cycle. The parser adopts this matcher in Phase 5.1; writing it here means the logic exists once rather than being duplicated between `query.go` and `state`.
- `query.go` (new, package `xcl`) — unexported `find[T any](c *Config, id string) (*T, error)` plus exported `Find`. Resolution order: the shared matcher over `Config.Entities()` (`config.go:51`), then the published-value branch, then conversion. **Do not delegate to `Config.FindResource` → `state.findResource`**: the storage layer is being taken out of the query path (see Architecture & Design Decisions), and resolving here makes `find` consistent with `findByType` and `all`, which already scan `Entities()`.
- Address tolerance is now this phase's own responsibility rather than inherited: match on `Module`/`Type`/`Subtype`/`Name` and ignore `Attribute`, so a trailing attribute suffix resolves; module-relative goes through `AppendParentModule` (`internal/resources/fqrn.go`). The existing `state/state.go` `findResource` is the reference implementation to lift. Assert all three forms rather than assuming.
- Published-value branch — `internal/resources/output.go:11-17`: return `Output.Value` (`:15`, populated at `internal/parser/callbacks.go:173` during apply, only when `CtyValue` is non-null), **not** the `Output` entity. Note `Value` has a `json` tag but no `xcl` tag and `CtyValue` the reverse, so only `Value` survives a JSON copy. `Output` lives in `internal/resources`, so the type is not nameable by a consumer — the conversion must target the caller's `T`, not the entity type.
- Require a full address: a bare name must fail. `ParseFQRN`'s catch-all `onlymodules` alternation (`internal/resources/fqrn.go:60`, `:118-133`) is where a bare name would otherwise land — confirm it errors rather than resolving.
- `As[T any](entity any) (*T, error)` — lift `asType` from `querier.go:82-89` (its only caller is that file, so it moves cleanly). Two changes: the `*T` fast path at `:83-85` stays; the fallback at `:87-89` must **not** return the partially-filled value alongside an error. Add the verification gate in front: where `T` is a registered type, compare its registered variety against `Meta.Subtype` and return the type-mismatch error before reaching `internal/schema/unmarshal.go:5-20`.
- Nested-block refusal reuses `types/resource_helpers.go:10-25`; note `st.Name()` at `:19` is **empty for an anonymous struct**, the common shape for reflection-built plugin resources, so the error must carry the type from the caller rather than from reflection alone.
- Replace the panics: `querier.go:32,59` panic on `types.GetMeta` failure. Follow `state/state.go:115-118`, which `continue`s instead.

**Complexity**: High
**Token estimate**: ~45k tokens
**Agent strategy**: Parallel analysis, sequential integration. The published-value projection and the
conversion gate are independent enough to analyse concurrently, but both land in one new file and
share the error taxonomy, so integrate in one pass.

### Phase 2.3: Look up every item of a kind, and the one item expected

**Repo:** xclconfig

**File changes**
- `query.go` — `findByType[T any](c *Config, path ...string) ([]*T, error)` and `findOne[T any](...)`, with their exported wrappers. Segments match positionally from the root: segment one the kind, segment two the variety. At least one segment required; the no-segment form is `All` (Phase 2.4).
- Scan source: `Config.Entities()` (`config.go:51`). Per-item axes come from `types.GetMeta` (`types/resource_helpers.go:28-53`). Note this is a second linear scan over a materialised `[]any` on top of state's own — acceptable per the explicit non-goal on indexing.
- Typeability rule: a set is typed only when the segments pin exactly one Go type. `("resource")` alone and `("output")` both fail with the not-typeable error — the published-values case must name the dedicated call that returns them. `("nosuchkind")` fails as unknown-type. This is the criterion that closes today's silent empty at `querier.go:62`.
- A variety with no kind before it must match nothing and fail, not match positionally in the wrong place — the design's "nothing is typed `container`" rule.
- `findOne` — exactly one match returns it; none returns the not-found error; two or more returns the not-unique error **carrying the count**.
- Nested-block refusal: a kind lookup naming a block that exists only nested inside another declaration returns the not-an-entity error naming the containing declaration. Reference shapes at `example/configonly/resources/resources.go:47-51` (`Container`, `Port`, `Volume`, `VolumeMount` — none embeds `ResourceBase`).
- Test fixture: `internal/test_fixtures/config/query/main.xcl` already declares 3 `database` and 2 `network` resources, which matches the acceptance criterion's "three of one variety and two of another" exactly. Reuse it; add a nested-block type and a bare-form declaration.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: 2 parallel agents — one on the two lookups, one on the negative-path tests (seven
conditions, each its own named function per the no-table-driven and no-mixed-polarity conventions) —
then sequential integration.

### Phase 2.4: Look up by Go type, and enumerate everything declared

**Repo:** xclconfig

**File changes**
- `plugins/registry/plugin_registry.go` — `TypePath(t reflect.Type) ([]string, bool)` returning `{"resource","<variety>"}` for kind-led registrations and `{"<name>"}` for bare-form ones, using the form recorded by Phase 1.4. Prototypes are concrete pointers in `types.RegisteredTypes` (`types/register.go:20`, instantiated via `reflect.New` at `:26`), so matching is `reflect.TypeOf(proto).Elem()` against `T`.
- Plugin-provided types have **no host-side Go type** — they are built from JSON schema at `plugins/registry/plugin_registry.go:150-152`. So `TypePath` reports not-found for them and `All[T]` returns the not-registered error naming the kind-lookup form. This is a hard constraint, not a gap.
- `query.go` — `all[T any](c *Config) ([]*T, error)`: derive segments via `TypePath`, then delegate to the kind-lookup path so the two agree by construction (an acceptance criterion).
- `config.go:51` — rename `GetResources` to `Entities`, keeping `GetResources` as a deprecated alias delegating to it. `config.go:67` — same for `ResourceCount` → `EntityCount`. Both keep the nil-state guards at `:52-54` and `:68-70`. Mandated by `knowledge:glossary/entity.md`, not only by the design.
- `Outputs() map[string]any` on `Config` — every published value keyed by address, projecting `Output.Value` the same way Phase 2.2 does. One entry per published declaration.
- `Entities()` genuinely covers every kind — **verified during planning, not assumed**: `internal/parser/parser.go:463-467` appends every parsed resource into state with no filtering by kind, and `state/state.go:29` returns that slice whole. So variables, published values and modules are already present and the count criterion is satisfiable as written. Root entities (`internal/parser/dag.go:50,132`, type `root`) are synthesised for the DAG rather than parsed, so they do not enter by this path — confirm they do not appear.
- Internal callers of the old names to repoint: check `state/`, `internal/parser/`, examples, tests.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: 2 parallel agents — one on `TypePath` and the registry, one on the rename plus
enumeration and published-value collection — then sequential integration.

### Phase 2.5: Ship both spellings and remove the superseded helper

**Repo:** xclconfig

**File changes**
- `query_methods_go127.go` (new) — first line `//go:build go1.27`, modern single-line form with **no** legacy `// +build` companion, matching `consts_win.go:1`/`consts_not_win.go:1` (the only first-party build-tag precedent; the `go1.18`-tagged files are all vendored HCL and use the old dual form). Methods on `*Config` delegating to the unexported implementations; no logic.
- `query.go` — the exported package-level functions, likewise pure pass-throughs. `As` is a function in both worlds; it takes no `Config`.
- **Delete** `querier.go` (all 90 lines, 5 symbols) and `querier_test.go` (189 lines, 7 tests + `setupQueryConfig` helper). `asType` has no caller outside `querier.go`, so the file lifts out whole. Port the helper's fixture setup (`querier_test.go:24-61`) into the new tests rather than losing it — it registers a type, registers `parser.TestPlugin`, mocks the state store and applies the shared fixture.
- Equivalence tests — for every lookup, assert the method form and the function form return equal values and equal errors for identical inputs. These only compile on 1.27+, so they belong in a file carrying the same build tag.
- `go.mod:3` stays `go 1.25.0`. No `toolchain` directive is added — adding one would override the floor job's `GOTOOLCHAIN`.
- Verify locally both ways: `go build ./...` (native 1.27.0) and `GOTOOLCHAIN=go1.25.0 go build ./...` (resolves from the module cache, no network).

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential. The two spellings must be written and verified together;
splitting them across agents is how they drift, which is the one thing this phase exists to prevent.

### Phase 3.1: Repair the configuration-only example's teardown

**Repo:** xclconfig

**File changes**
- `example/configonly/main.go:61-120` (`run`) — never calls `c.Destroy()` and never prints `## Destroyed`. Add both, following the working shape at `example/plugin/main.go:165-170` (`c.Destroy()` then `"## Destroyed\n  %d resources remaining"`).
- `example/configonly/main.go:12-13` — the doc comment already claims the teardown behaviour, so it needs no change; it is the code that is wrong.
- Tests are **already written and already failing** — do not modify them to match the code: `example/configonly/main_test.go:447-465` (`TestConfigOnlyExampleDestroysEverythingItApplied`), `:467-474` (`TestConfigOnlyExamplePrintsNoResourcesRemaining`), `:479-493` (`TestConfigOnlyExampleLogsDestroySuccessWithoutStartAtDebug`).
- Confirmed failing on HEAD `13e0311` before any of this plan's work.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential. Small and well-specified by the failing tests.

### Phase 3.2: Move every bundled example and test to the new surface

**Repo:** xclconfig

**File changes** — all 8 example call sites, each a single chained expression today:
- `example/appconfig/main.go:88` — `Find[resources.Application]` by address.
- `example/configonly/main.go:127` — list-by-type → kind lookup for `deployment`; `:173` `service`, `:181` `ingress` by address.
- `example/plugin/main.go:124` `postgres`, `:134` `redis` (both list-by-type); `:144` `app`, `:153` `ingress` (both by address).
- All rewritten in the **portable function form** (`xcl.Find[T](c, id)`), never the method form, so they compile on the floor toolchain.
- Published-value example — `example/plugin/config/modules/db/db.xcl` already declares a module with outputs, so `example/plugin` is the natural host for the address-based published-value retrieval, with an assertion in `example/plugin/main_test.go`.
- Example tests to update alongside: `example/appconfig/main_test.go` (10 funcs), `example/configonly/main_test.go` (31 funcs/helpers), `example/plugin/main_test.go` (31 funcs/helpers, plus `TestMain` at `:29-51` which shells out to `go build` and inherits `GOTOOLCHAIN` from the environment).
- Leave the static-analysis guards intact: `example/configonly/main_test.go:271-295` (asserts no plugin imports) and `example/plugin/main_test.go:225` (asserts the example defines no types of its own).

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: 3 parallel agents, one per example directory — they are independent programs with
independent tests — then a single sequential pass to confirm no `NewQuerier` reference survives
anywhere.

### Phase 3.3: Prove the build on the oldest supported toolchain

**Repo:** xclconfig

**File changes**
- `.github/workflows/go.yml:22` — currently `go-version: 1.22`, **below** the `go.mod` floor of `1.25.0` (`go.mod:3`). Correct it; with `setup-go@v3` and `GOTOOLCHAIN=auto` a 1.22 toolchain would try to download 1.25.0 to satisfy the directive. Also consider raising `actions/checkout@v3` and `actions/setup-go@v3`, both old majors.
- `.github/workflows/go.yml` — add a second job alongside `build:` (`:14`) running `GOTOOLCHAIN=go1.25.0 go build ./...`. There is **no existing precedent for a second job** in this repo, so follow the established step shape: `runs-on: ubuntu-latest`, `actions/checkout`, `Set up Go`, then a named run step. Triggers at `:6-10` (push and PR to `main`) already satisfy "on every change".
- Scope the job to `go build`, **not** `go test ./...`: `plugins/example/e2e_test.go` requires a pre-built binary at `./build/example` (`plugins/testing/helpers.go:62-63`, no `t.Skip`) while `plugins/example/build/` is gitignored, so a fresh checkout fails the full suite for reasons unrelated to this work. Left tracked separately by decision.
- No `toolchain` directive in `go.mod` — it would override the job's `GOTOOLCHAIN`.
- The existing `build` job cross-compiles for darwin/linux/windows at `:26-28` with `CGO_ENABLED=0`; the floor job needs only the host platform.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential. One file; the care needed is in the toolchain semantics,
not the volume.

### Phase 4.1: Rewrite the library documentation for the new surface

**Repo:** xclconfig

**File changes**
- `README.md:267-284` — the `### Querying resources` section: prose at `:269-270` naming `NewQuerier`, the code block at `:273-279`, and the registered-vs-plugin note at `:283-284`. Replace wholesale with the new surface.
- `README.md` — add a reading-published-values subsection. The README covers `output` only from the authoring side today (`:710-785` under `## Modules`); **no Go example anywhere reads one back**. Retrieval by address is the worked example to add.
- `README.md:552-566` — under `## References to other resources`, the untyped `c.FindResource(...)` plus `r.(*Config)` type assertion. Still compiles (`config.go:59` returns `any`), but it is now the worse way; update to the typed lookup.
- `README.md:269` and `CHANGELOG.md:21` — both name the superseded helper and must be updated for the zero-matches search to pass.
- `docs/state.md:30-34` — the linear-scan passage. Carry it forward and extend it to the new surface, which adds a second scan over a materialised slice.
- `docs/README.md:46` — says `example/` holds "**Two**" runnable examples; there are three. Already stale, cheap to correct while here.
- **Out of scope, deliberately**: `README.md:1042-1273` (232 lines documenting `Process`, `ToJSON`, `ParseCallback`, `Processable` — all verified absent from the code) and the related prose at `:39-42,58-60`. The spec's non-goals name this cleanup as separate work. Do not absorb it.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential. Prose work with a clear inventory; the main risk is
scope creep into the stale tail, which the exclusion above guards against.

### Phase 4.2: Move every documentation site page to the new surface

**Repo:** xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`)

**File changes** — 6 lines across 4 files, which is all 4 pages on the site:
- `xcl-website:src/pages/index.mdx:96` — the headline Go example (block spans `:89-97`, introduced at `:87`). The first code a visitor reads. Note it is *already* non-compilable: unchecked `err` at `:94`, unused `db` at `:96` — fix while rewriting.
- `xcl-website:src/pages/index.mdx:161-164` — the `🔎 Typed queries` `<FeatureCard>`, whose body names `` `NewQuerier[T]` `` explicitly. Both the body and possibly the title need rewriting. Named in its own acceptance criterion.
- `xcl-website:src/pages/examples/configuration-only.mdx:260-261` (prose naming `` `NewQuerier` ``) and `:264` (list-by-type; block spans `:263-282`, the rest of which is API-neutral).
- `xcl-website:src/pages/examples/plugins.mdx:295` (list-by-type, block `:292-296`), plus **lowercase** "querier" at `:289` (prose) and `:294` (in-sample comment).
- `xcl-website:src/pages/examples/application-config.mdx:303` (by address, block `:277-307`), plus adjacent prose at `:274` ("One block type, one Go type, one query.").
- All rewritten in the **portable function form**.
- The search guard must be **case-insensitive** — `:289` and `:294` are invisible to a capital-Q grep. `GetResources` and `ResourceCount` appear **zero** times site-wide; bare `Querier` never appears, only the constructor.
- Samples stay hand-written fenced blocks; there is no include mechanism (no `?raw`, `readFileSync`, `import.meta.glob`, `getEntry` or `getCollection` anywhere in `src/`) and none is being added. `title="example/configonly/main.go"` meta strings look like paths but nothing reads them.
- `xcl-website:README.md:33-40` — the page table is already stale (missing `/examples/application-config/`); cheap to correct. Lines `:39-40` state the manual "update the pages to match" policy, which remains the governing policy.
- Verify with `npm run build` (`package.json:7-12`); there is no test or lint script, and `Makefile:15-16`'s `check` is `npx astro check`, which does not inspect fenced-block contents.
- **Not doing**: any Go module, Go CI job or snippet-extraction tooling. Compile-proofing was dropped by decision; `.github/workflows/deploy.yml` installs Node 22 only and has no `pull_request` trigger, and that stays as it is.

**Complexity**: Low
**Token estimate**: ~18k tokens
**Agent strategy**: Single agent, sequential. Six lines across four files in one repo; parallelising
risks inconsistent phrasing across pages that a reader sees side by side.

### Phase 4.3: Bring the recorded architecture knowledge in line

**Repo:** xclconfig (knowledge store, `repo` tier, source `xclconfig`)

**File changes**
- `knowledge:architecture/ux-flow.md` — revise three parts: the "### 3. Query Resources" section showing `xcl.NewQuerier[MyResourceType](config)` with `FindResource`/`FindResourcesByType`; the public-API layer diagram, which lists `Querier[T] - Type-safe resource queries`; and "### 2. Querier Requires Config" under Key Design Decisions. Add retrieval of published values, which the entry does not cover.
- Its three "Open Questions" — whether the querier should accept an interface, how tests query without circular dependencies, and the minimal querier interface — are **settled** by this work (the surface is methods on `Config` plus portable functions, in package `xcl`). Resolve or remove them rather than leaving them open against a construct that no longer exists.
- Access is through `spektacular knowledge` only — never `Write`/`Edit` on a store path. Propose tier, store name, path and exact content, and **wait for explicit confirmation** before `spektacular knowledge write`.
- `knowledge:glossary/entity.md` — check only. It states the `entity` target and notes the code has not caught up; once Phase 2.4 lands, that note may need softening, but the entry's substance stands.

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential. Must not be delegated blindly — a sub-agent inherits the
store-access rules but not the propose-then-confirm obligation, so the confirmation step stays with
the driving agent.


### Phase 5.1: Take lookups out of storage

**Repo:** xclconfig (`/home/nicj/code/github.com/jumppad-labs/xcl`)

**File changes**
- `state/state.go` — delete `FindResource` (`:82`), `FindRelativeResource` (`:90`), `FindResourcesByType` (`:110`), `FindModuleResources` (`:134`) and the unexported `findResource`. Keep `GetResources`, `AppendResource`, `RemoveResource`, `ResourceCount`, `Bytes` and `addResource`. The `resources` import goes with them — but note `addResource` builds the FQRN that becomes `meta.ID`, so either it keeps that one use or ID assignment moves to the parser, which already does the same thing at `internal/parser/parser.go:707,821` via `FQRNFromResource`. Prefer moving it: it is the last thing tying storage to the address type.
- `internal/parser/dag.go:13-24` — `ConfigProvider` is named for `Config` but satisfied by `*state.State`. Rename it for what it provides. `ResourceProvider` embeds it and adds `GetResources()`.
- `internal/parser/util.go:254` (`FindRelativeResource`), `:539` and `internal/parser/callbacks.go:97` (`FindModuleResources`), `internal/parser/util.go:555,565`, `internal/parser/lifecycle.go:85`, `internal/parser/parser.go:344`, `internal/parser/progress.go:65` (`FindResource`) — move each onto the shared matcher from Phase 2.2, applied over `GetResources()`.
- `config.go:59-63` — `Config.FindResource` currently delegates to `currentState.FindResource`. Repoint it at the shared matcher over `Entities()`, which is what `find[T]` already does after Phase 2.2.
- **Do not** change `StateStore` here; that is Phase 5.2. This phase must leave the suite green on its own.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential. The edit is mechanical but the call sites are load-bearing — `lifecycle.go:85` and `progress.go:65` drive re-apply and failed-apply state — so verification matters more than volume.

### Phase 5.2: Narrow the persistence contract to raw items

**Repo:** xclconfig

**File changes**
- `state/state_store.go:6-21` — `Load() ([]any, error)` and `Save(resources []any) error`; `Exists()` and `Clear()` unchanged.
- `state/file_state_store.go:33` (`Load`) — return the built slice instead of assembling a `*State`; `:143` (`Save`) — take `[]any` and marshal it directly, replacing the `state.Bytes()` call at `:144`. The `UnknownTypesError` accumulation (`:81-91`, `:110-113`) and the two-axis read added in Phase 1.1 are unchanged.
- `state/mocks/mock_state_store.go` — regenerate with Mockery for the new signatures.
- `config.go:22,34` — `currentState` becomes the raw slice, or `State` moves behind `internal/`. Note `NewConfig` builds the state **before** the options loop runs, so whichever shape is chosen must be valid with no store configured.
- `internal/parser/parser.go:231,303,306,383,387`, `internal/parser/progress.go:49` — every `state.NewState()` site follows whatever shape is chosen.
- `docs/parser-lifecycle.md:9,127` show `Apply`/`Destroy` returning `*state.State`; those signatures change, and Phase 5.3 covers the prose.
- **Breaking**: sanctioned by the spec's non-goal on compatibility shims for an unreleased version. No example implements `StateStore` (verified), and `NewFileStateStore` and `WithStateStore` both survive, so `README.md:287-297` barely moves.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential. One contract change rippling through a known set of call sites; splitting it risks two agents choosing different shapes for `currentState`.

### Phase 5.3: Document the state contract the library now has

**Repo:** xclconfig

**File changes**
- `docs/state.md:3-53` — `## State — the in-memory resource registry` presents the container as a public type with a query surface. Replace with the storage contract. `:114` (`## StateStore — the persistence contract`) and `:145` (`## FileStateStore`) are updated for the new signatures. The linear-scan passage at `:30-34` was already carried forward in Phase 4.1 — do not undo that.
- `docs/parser-lifecycle.md:9,127,133,156` — signatures and prose naming `*state.State`.
- `docs/overview.md:13,31,49,58,61-62,100` — `owns: … in-memory current State`, and the `Config{currentState: state.NewState()}` walkthrough.
- `docs/README.md:26,43` — the component table row `state/ | State, StateStore interface, FileStateStore`.
- `README.md:287-297` — check only; `NewFileStateStore` and `WithStateStore` survive, so this likely needs no change.
- Add the statement that the configuration object is the only supported way to reach a declared item, so a reader does not go looking for a second one.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential. Prose with a fixed inventory.


## Testing Strategy

Recast at per-phase granularity. The plan-level strategy, the load-bearing assertions and the
success-metric mapping live in plan.md § Testing Approach; this is where each phase's coverage
actually lands.

House conventions bind every entry below without exception: testify `require`; one behaviour per
named test function; positive and negative cases in **separate** functions; **no table-driven
tests**; verbosity preferred over shared abstraction; and state needed by a re-apply or removal test
produced by a real first apply rather than hand-written, except where the test is about the on-disk
shape itself.

- **Phase 1.1** — Regression-dominated. The load-bearing proof is that existing behaviour is unchanged: an existing configuration applies, re-applies and destroys as before, and every resource still reaches the right provider. Add direct assertions that a resource-kind declaration reports both axes and that a variable, published value and module each report an empty variety. Update the existing metadata expectations at `config_test.go:364,400,433`, `plugins/registry/plugin_registry_test.go:124,148,299`, `internal/resources/default_test.go:32`, `internal/parser/parser_plugin_test.go:40`, `types/resource_helpers_test.go:52,63,70,81,92,99,191`.
- **Phase 1.2** — Integration only; a unit test cannot prove this. Apply a fixture exercising resource→resource, resource→variable, resource→module-output and module-output→resource in one pass, and assert the *values* landed, not merely that apply returned nil. Add a negative test that an undefined reference is still reported naming the item that made it, in its own function.
- **Phase 1.3** — Unit tests in `internal/resources/fqrn_test.go`. Round-trip is the assertion that matters: parse→format→identical string, and an address produced for a declaration accepted by a lookup for it. Keep the existing cases at `:61,75`, `:94-227`, `:258-285` passing unchanged; they are the guard that variable, published-value and module address forms did not shift.
- **Phase 1.4** — Parser-level integration with new `.xcl` fixtures declaring the same variety in both forms, asserting two declarations with different addresses. Negative: an unknown leading keyword errors naming that keyword. **Gotcha** (`knowledge:gotchas/hcl-halts-on-malformed-block-header.md`): a fixture with two malformed block headers yields **one** diagnostic, not two — assert on diagnostic content, never on counts.
- **Phase 1.5** — The one documented exception to the real-apply rule: the stale-file test is *about* the on-disk format, so hand-write a state file in the old single-axis shape and assert the load fails naming what it could not resolve. The positive round-trip fixture is produced by a real apply with a file state store.
- **Phase 2.1** — Unit tests, one named function per sentinel, asserting each is distinguishable from every other and its detail recoverable — including the **count** on the not-unique detail. Separately assert that a state-originated not-found and a lookup-originated not-found both match the same sentinel, while the plugin-boundary not-found matches neither.
- **Phase 2.2** — Positive: address lookup returns a populated typed value from one call, for module-relative, non-normalised and attribute-suffixed forms, all three resolving to the same item. Published values: the resolved value is returned rather than the declaration, a module's resolves by full address, and a bare name fails. Negative, each its own function: conversion to a non-corresponding type errors and returns no value; repeat across every registered type so a single leaked path fails the suite.
- **Phase 2.3** — The densest negative coverage in the plan. One named function each for: only-a-variety, a kind spanning several Go types, published values (asserting the message names the dedicated call), an unknown kind, and a nested block (asserting the message names access through the containing declaration). Positive: three of one variety and two of another returned exactly, and an empty result with a nil error for a variety with nothing declared. Reuse `internal/test_fixtures/config/query/main.xcl`, which already declares exactly 3 `database` and 2 `network`.
- **Phase 2.4** — Assert the Go-type lookup returns what the equivalent kind lookup returns for the same configuration, which is the criterion tying the two paths together. Negative: a plugin-provided type reports not-registered and names the working form; a nested block's type gives the not-addressable error. Enumeration: the count matches the declarations present, entries cover resource-kind items, variables, published values and modules, and synthesised DAG root entities do **not** appear. Assert the deprecated aliases return results identical to the new names.
- **Phase 2.5** — Equivalence tests for every lookup, asserting equal values *and* equal errors between the two spellings, in a file carrying the same build constraint as the methods, since they only compile where both exist. Port the fixture setup from `querier_test.go:24-61` before deleting it. Verify both ways locally: native build, and with the toolchain pinned to the floor.
- **Phase 3.1** — No new tests. The three already-failing tests are the specification; they must pass **without being modified to match the code**.
- **Phase 3.2** — The example tests are the coverage and already run in the normal test run, so a broken example fails an ordinary run. Add the published-value assertion to `example/plugin/main_test.go`, whose fixture already declares a module with outputs. Leave the static-analysis guards intact (`example/configonly/main_test.go:271-295`, `example/plugin/main_test.go:225`).
- **Phase 3.3** — Verified by the job existing, running on every change, and passing. Deliberately build-scoped, not suite-scoped, because the full suite cannot pass on a fresh checkout while the plugin-example binary issue stays open.
- **Phase 4.1 / 4.2** — Prose. Verified by a case-insensitive search returning no matches and, for the site, by the build succeeding and the affected pages rendering. Site samples are **not** compile-checked, by decision.
- **Phase 4.3** — Verified by review of the proposed content before it is written, not by a test.

**Deliberate gaps.** No compile-checking of documentation-site samples (dropped by decision — this
descopes one clause of the spec's site acceptance criterion). No fix for the plugin example's
pre-built-binary failure, so a full-suite run is not a clean signal and no phase may claim otherwise
without naming it. No performance or scale testing, since lookups remain linear scans by design and
there is no latency requirement. No new mocks — the surface is exercised against real applied
configurations, and the existing state-store mock continues to serve the tests that already use it.


## Project References

**Repos.** Resolved from `spektacular repo list`; both roots verified present on disk during
discovery. Never infer either from the working directory.

| Repo | Root | Role |
|---|---|---|
| `xclconfig` | `/home/nicj/code/github.com/jumppad-labs/xcl` | The Go library. Phases 1.1–1.5, 2.1–2.5, 3.1–3.3, 4.1, 4.3. |
| `xcl-website` | `/home/nicj/code/github.com/jumppad-labs/xcl-website` | Documentation site, Astro 5 + MDX + Tailwind v4, served at xcl.dev. Phase 4.2 only. |

**Binding documents.** Reached through the CLI, never with file tools.

- **Spec** — `spektacular spec file read 20260921093100-query-api-v2.md`. Final, closed 2026-09-21. Source of requirements, acceptance criteria and success metrics. Governs *what must be true*.
- **Design** — `spektacular design read --data '{"source":"design","path":"querier-api-v2.md"}'`. The settled shape, referenced by the spec and resolved during discovery (`unresolved: 0`). Governs *how*. Its "Files to change" table is the canonical work inventory. Cites [jumppad-labs/hclconfig#61](https://github.com/jumppad-labs/hclconfig/issues/61) as its origin.
- **One divergence between them**: the design places the bare declaration form under "Not in scope"; the spec requires it. The spec's own precedence rule resolves this in the spec's favour, so Phase 1.4 exists.

**Knowledge — binding, and outranking the code it describes.**

- `knowledge:glossary/entity.md` — mandates the `entity` umbrella term and the renamed enumeration and count operations (Phase 2.4). States the code has not caught up; this plan is the catching up.
- `knowledge:conventions/testing-and-mocking.md` — testify `require`, **never** table-driven, never mix positive and negative cases in one function, favour verbosity. Binds every test in every phase.
- `knowledge:conventions/test-state-from-real-apply.md` — state fixtures come from a real apply, except when the test is about the on-disk format itself. Governs Phase 1.5.
- `knowledge:conventions/never-modify-dependencies.md` — HCL lives in-tree at `internal/xcl` under MPL-2.0. Relevant to Phase 1.4 if it reaches the fork: keep the upstream header, add the modification notice, record in `internal/xcl/UPSTREAM.md`.
- `knowledge:conventions/code-style.md` — explicit error handling, small focused interfaces, `any` over `interface{}`.
- `knowledge:gotchas/hcl-halts-on-malformed-block-header.md` — two malformed block headers yield **one** diagnostic, not two. Assert on diagnostic content, never counts. Governs Phase 1.4's negative fixtures.
- `knowledge:architecture/ux-flow.md` — **revised by this plan** (Phase 4.3). Currently documents the superseded helper as current architecture, including in its layer diagram, and carries three open questions about that helper which this work settles.

Load with `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website`;
search with `spektacular knowledge search <surface>`; read a hit with
`spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"<path>"}'`.

**Prior plans consulted** (historical intent, not current behaviour — read with
`spektacular plan file read <name>/plan.md`):

- `20260919120639-config-only-types-and-examples` — the immediate predecessor, already landed. Introduced `RegisterType`, the type-name clash gate whose three-source scan Phase 1.4 inverts into a query, and the three example programs with the shared `run(out, dir)` shape Phase 3.2 migrates. It last changed the superseded helper's list-by-type method; its recorded reasoning — that a zero `T` has no metadata and plugin look-alikes cannot be matched back to `T` — is exactly what `TypePath` now answers for registered types, and exactly why `All[T]` cannot serve plugin types.
- `20260918165700-provider-lifecycle-read`, `20260919152915-destroy-cycle` — establish the lifecycle and destroy paths that Phase 1.1's metadata change reaches through the provider-less short-circuit. Consulted for blast radius only.

**Key source files, by concern** (full detail per phase above):

- Superseded surface: `querier.go` (90 lines, 5 symbols), `querier_test.go` (189 lines, 7 tests).
- Public entry point: `config.go:21-27` (the `Config` struct), `:51,59,67` (the three methods that change).
- Metadata: `types/resource.go:5-54`, helpers at `types/resource_helpers.go:10-25,28-53`.
- Addressing: `internal/resources/fqrn.go:12-22,53-142,179-224`.
- Parsing: `internal/parser/parser.go:521-548,559-578,580-752`; expression context `internal/parser/context.go:108,118,126,136`.
- Registry: `plugins/registry/plugin_registry.go:17-23,43-57,64-67,89-104,136-172`.
- State: `state/state.go:212-231`, `state/errors.go:8-15`, `state/file_state_store.go:63-73,82-112`.
- Error convention archetype: `plugins/errors.go` (10 lines, the whole file). New sentinels land in `errors/` alongside `errors/config_error.go:5-33` and `errors/parser_error.go:13-79`; that package imports only `internal/xcl` plus `go-wordwrap`, which is what makes it cycle-free for both `state` and the root package.
- Examples: `example/{appconfig,configonly,plugin}/`; docs: `README.md:267-284`, `docs/state.md:30-34`.
- CI: `.github/workflows/go.yml` (31 lines, one job).

**Verification commands** (for the implementer, not for plan.md's acceptance criteria):

- Exercise the floor: `GOTOOLCHAIN=go1.25.0 go build ./...` — a plain `go build` uses the installed 1.27.0 and proves nothing about the minimum.
- Re-derive the metadata blast radius: `grep -rn 'meta\.Type\|Meta\.Type' --include='*.go'` from the `xclconfig` root, excluding `internal/xcl` and `internal/cty`.
- Re-derive the migration surface: `grep -rn 'NewQuerier' --include='*.go'` (17 hits; 2 are the declaration and its doc comment) and, in `xcl-website`, `grep -rni 'querier' src/` — **case-insensitive**, since two site mentions are lowercase prose.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Per-phase estimates and strategies are recorded on each phase above. Rough totals: Milestone 1
~165k, Milestone 2 ~165k, Milestone 3 ~57k, Milestone 4 ~50k.

Two phases warrant care beyond their tier. **Phase 1.1** is the largest and the one where parallel
agents are most tempting and most dangerous: its edit sites are independent to *analyse* but must be
applied in one pass, because a half-migrated intermediate state compiles cleanly and behaves wrongly.
**Phase 2.5** must not be split across agents at all — the two spellings drifting apart is the exact
failure the phase exists to prevent, and two agents are how that happens.


## Migration Notes

**No state migration is written**, and this is deliberate: regenerating configuration state written
by the previous version is explicitly acceptable per the spec's non-goals.

What changes instead is honesty about it. `Meta` gains a second axis, and because `Meta`'s JSON tags
*are* the on-disk state format (`state/state.go:178-183` marshals the resource slice directly), a
state file written before this change carries only the old single axis. The loader currently drops an
unresolvable item **silently** (`state/file_state_store.go:69-73`), so such a file would quietly yield
a smaller configuration. Phase 1.5 turns that into a reported failure reusing the existing
`UnknownTypesError` (`state/errors.go:26-40`), so a stale file announces itself instead of
half-loading.

Note the ordering hazard in the loader: `CreateResource` sets the metadata, then the JSON
re-unmarshal at `state/file_state_store.go:94,100` overwrites it with what the file held. Both axes
must therefore be present in the written JSON for a load to reconstruct correctly — which is why the
tag choice on the new field in Phase 1.1 matters to Phase 1.5.

**For consumers**, there is no migration path and none is required: the version carrying these
changes is unreleased and has no external consumers. In-repo, the two renamed enumeration and count
operations keep their previous names as deprecated aliases, so internal callers continue to compile.
Addresses gain a segment, so a stored address string from a previous version is not guaranteed to
resolve — regenerating state covers this.


## Performance Considerations

**Nothing here is optimised, and that is a decision rather than an omission.** Indexing and
lookup-performance work are explicit non-goals in the spec, and `docs/state.md:30-34` already records
that every lookup is a linear scan comparing metadata fields against a parsed address, with no index,
which is adequate at one configuration's scale.

The new surface adds a second scan on top of the existing one: the lookups iterate `Config.Entities()`
(`config.go:51`), which returns the materialised slice from `state/state.go:29`, while state's own
`findResource` (`state/state.go:212-231`) scans independently. So an address lookup is O(n) over a
full materialised slice, and a kind lookup likewise. At the scale this library is used — one
configuration's worth of declarations — that is immaterial, and the documentation says so explicitly
in Phase 4.1 so the next reader does not assume an index exists.

Two small costs are worth naming so they are not discovered as surprises. Deriving addressing from a
Go type uses reflection against the registry's stored prototypes, which happens once per call rather
than once per item scanned. And conversion still routes non-matching types through a JSON round-trip
(`internal/schema/unmarshal.go:5-20`) — the new verification gate in front of it actually *reduces*
that cost, because a mismatched type is now refused before the round-trip runs rather than after.
