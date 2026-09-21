---
created_date: "2026-09-21"
document_status: final
closed_date: "2026-09-21"
---

# Research: 20260921093100-query-api-v2

Research spans two registered repos. Entries are prefixed with the repo name; `xclconfig` is
rooted at `/home/nicj/code/github.com/jumppad-labs/xcl` and `xcl-website` at
`/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Alternatives considered and rejected

### Keeping `Querier[T]` and adding methods alongside it

Rejected. The spec requires the superseded helper be *removed, not deprecated*, and the design
states it outright: `Querier[T]` exists only to bind `T` at construction, which a generic method
now does per call. The evidence that binding buys nothing is that **every one of the 15 call
sites constructs a querier and makes exactly one call on it**, and all 8 example sites are
single chained expressions that never store the querier
(`xclconfig:example/plugin/main.go:124,134,144,153`, `xclconfig:example/configonly/main.go:127,173,181`,
`xclconfig:example/appconfig/main.go:88`). Keeping both would also leave the silent-empty
`FindResourcesByType` path alive (`xclconfig:querier.go:62`), which is the defect the spec exists
to close.

### Raising `go.mod` to 1.27 so generic methods can be the only spelling

Rejected — forbidden by a spec constraint ("minimum supported Go version must remain 1.25.0")
and by the design. Confirmed as a real constraint, not a guess: `go.mod` declares `go 1.25.0`
with no `toolchain` directive (`xclconfig:go.mod:3`), and the design records the compiler error
proving generic methods need 1.27 (`-lang was set to go1.25`). The build-tag split is therefore
the only way to satisfy "capability must not be gated on toolchain version".

### Adding a `Kind` field and keeping `resource` as the umbrella term

Rejected in the design, and the knowledge base independently mandates against it. The glossary
entry `xclconfig:.spektacular/knowledge/glossary/entity.md` states `entity` is the umbrella term
and that `resource` names one stanza, citing `xclconfig:internal/resources/fqrn.go:60` where
`resource` is a sibling of `output|local|variable` inside one regex alternation group, not their
parent. The taxonomy also already exists on the plugin wire as `Type`/`SubType`
(`xclconfig:plugins/plugin.go:32-42`, proto `entity_sub_type` at
`xclconfig:plugins/proto/plugin.pb.go:168,272,384,488,618,730`), so `Kind` would be a third
vocabulary.

### Deriving `All[T]` addressing by reflecting over plugin types too

Rejected — impossible, not merely undesirable. Plugin types have no host-side Go type to reflect
against: they are built from JSON schema at `xclconfig:plugins/registry/plugin_registry.go:150-152`
via `schema.CreateInstanceFromSchema`, whereas builtin and `RegisterType` types are stored as
concrete pointer prototypes in `types.RegisteredTypes` (`xclconfig:types/register.go:20`,
instantiated by `reflect.New` at `:26`). This is why `All[T]` must return `ErrNotRegistered`
naming `FindByType` as the working form.

### Making `Outputs()` a case of `FindByType("output")`

Rejected by the design and corroborated by the code. `FindByType` is typed only when the
segments pin exactly one Go type; `"output"` cannot, for the same reason `"resource"` cannot.
Additionally an output's *value* is not its declaration: `Output.Value any` carries a `json` tag
but no `xcl` tag, and `CtyValue` carries an `xcl` tag but no `json` tag
(`xclconfig:internal/resources/output.go:11-17`), so only `Value` survives the JSON copy path
that `asType` falls back to (`xclconfig:internal/schema/unmarshal.go:5-20`).

### Matching `ErrNotFound` by making `ResourceNotFoundError` comparable rather than adding `Is`

Rejected. `ResourceNotFoundError` is a value type with one field and no `Is`/`Unwrap`
(`xclconfig:state/errors.go:8-15`), and its `Resource` field holds **three different kinds of
string** depending on which of seven construction sites produced it — an FQRN
(`xclconfig:state/state.go:230`), a bare type name (`xclconfig:state/state.go:128`,
`xclconfig:querier.go:73`), a module name (`xclconfig:state/state.go:174`), or empty
(`xclconfig:state/state.go:50,71`). Equality matching therefore cannot work;
`errors.Is(err, ResourceNotFoundError{})` succeeds only when the field is empty. An `Is` method
is the only route to one not-found concept.

### Docs-site samples: building a compile-proof mechanism at all

**Rejected by user decision on 2026-09-21 — the compile-proofing requirement is dropped.** Three
mechanisms were costed before the decision, and all three were real work rather than
configuration:

- *Real `.go` files in the site repo* — a Go module under `samples/`, each sample compilable,
  rendered into MDX through a `?raw` import into Expressive Code's `<Code>` component so page and
  compiler read the same bytes. Strongest guarantee; cost is a Go module and toolchain inside a
  Node site, plus a site dependency on a released `xcl` version that does not exist yet.
- *Samples owned by `xclconfig`, synced into the site* — keeps all Go in the Go repo and needs no
  new toolchain in the site; cost is cross-repo coupling and a sync step that can lag or fail.
- *Extraction with synthesised scaffolding* — pages stay as they are; cost is a bespoke MDX
  parser, and the scaffolding is invisible to the author, so a sample can compile for reasons the
  reader cannot see.

The decision leaves the site's existing manual policy in force, stated at
`xcl-website:README.md:39-40`: "The example pages quote the code in the xcl repository's
`example/` directory. When that code changes, update the pages to match."

**Consequence to carry into the plan, not to bury:** the spec's acceptance criterion "No
documentation-site page shows the superseded helper" is only *partly* satisfiable now. Its first
clause (a search for the superseded helper returns no matches) and its second (the landing page's
headline example and feature description both show the new surface) remain fully in scope and
testable. Its third clause — "every code sample on the site compiles under the pinned minimum Go
version" — is **dropped**, as is the same requirement under "Site examples use the form that
compiles on the minimum supported Go version" insofar as it demands verification. Samples will
still be *written* in the portable function form; that is simply not machine-checked.

### Docs-site samples: a regex/grep-only check that the superseded name is absent

Retained in reduced form, having been the fallback once compile-proofing was dropped. On its own
it satisfies the success metric "a search for the superseded query helper across the library and
the documentation site returns zero matches", which is now the criterion the site is held to. Its
known weakness is recorded so nobody mistakes it for more than it is: it would have passed on the
*existing* landing-page sample, which is already non-compilable (`err :=` at
`xcl-website:src/pages/index.mdx:94` is never checked and `db` at `:96` is never used). The search
must also be case-insensitive, because two lowercase "querier" mentions
(`xcl-website:src/pages/examples/plugins.mdx:289,294`) are invisible to a capital-Q grep.

## Chosen approach — evidence

### The surface, its spellings, and where they live

- Everything lands in package `xcl`: `Config` has all five fields unexported
  (`xclconfig:config.go:21-27`) and its complete method set is `GetResources`/`FindResource`/
  `ResourceCount`/`Validate`/`Apply`/`Destroy` (`xclconfig:config.go:51,59,67,82,115,160`) — no
  `Config` methods exist outside `config.go`, so the new surface has one home and needs no
  accessor for `pluginRegistry`.
- `querier.go` lifts out whole: it is 90 lines holding exactly 5 symbols
  (`NewQuerier` `:12`, `Querier[T]` `:16-18`, `FindResource` `:26`, `FindResourcesByType` `:51`,
  `asType` `:82`), and `asType` has no caller outside that file.
- The build-tag split is viable here and locally provable. The installed toolchain is **go1.27.0**,
  `GOTOOLCHAIN=auto`, and go1.25.0/1.25.3/1.25.5/1.25.6/1.26.0 are already cached in
  `~/go/pkg/mod/golang.org/`, so `GOTOOLCHAIN=go1.25.0 go build ./...` runs with no network fetch.
  Because `go.mod` says 1.25.0 and 1.27.0 satisfies it, a plain `go build` does **not** exercise
  the floor — the explicit `GOTOOLCHAIN` is load-bearing.
- Build-tag precedent in first-party code exists but is thin: only the GOOS pair
  `xclconfig:consts_win.go:1` (`//go:build windows`) and `xclconfig:consts_not_win.go:1`
  (`//go:build !windows`), both in package `xcl`, modern single-line form, no legacy `// +build`.
  The four `//go:build go1.18` files are all vendored HCL. So a Go-version tag is a new pattern
  for first-party code, though the mechanism is already in the package.

### Why the silent-empty and half-populated defects exist

- `FindResourcesByType` matches `meta.Type == typeName` (`xclconfig:querier.go:62`), and
  `meta.Type` holds the *subtype* for a `resource` stanza — `blockResource` seeds `fqrn.Type`
  from the block keyword then overwrites it with `Labels[0]` for `resource`
  (`xclconfig:internal/parser/parser.go:565,568-570`), and `types.RegisteredTypes.CreateResource`
  writes `meta.Type = resourceType` (`xclconfig:types/register.go:32`). The word `resource`
  never lands in `Meta.Type`, so `FindByType("resource")` returns empty today.
- The half-populated path is `encoding/json` round-tripping with no `DisallowUnknownFields`
  (`xclconfig:internal/schema/unmarshal.go:9,15`): unknown fields drop silently, absent fields
  zero, and on a `*json.UnmarshalTypeError` the other fields are still populated — and `asType`
  returns that partially-filled value *alongside* the error (`xclconfig:querier.go:87-89`).
- The check that refuses nested blocks already exists: `findResourceBase` returns
  `ResourceBase field not found in resource type %s` (`xclconfig:types/resource_helpers.go:19`).
  Note `st.Name()` is **empty for an anonymous struct**, which is the common shape for
  reflection-built plugin resources — so the message needs the type supplied by the caller, not
  taken from reflection alone.
- Both current panic sites are confirmed (`xclconfig:querier.go:32,59`), both from
  `types.GetMeta`. `state.State.FindResourcesByType` handles the same error with `continue`
  (`xclconfig:state/state.go:115-118`), which is the behaviour to adopt.

### Addressing and the data model

- `FQRN` has four fields and no subtype slot (`xclconfig:internal/resources/fqrn.go:12-22`), and
  the `resource.` segment is a **string literal in the formatters**
  (`xclconfig:internal/resources/fqrn.go:202,223`) — so round-tripping a variety segment is a
  change to both `ParseFQRN` (`:53-142`) and `String`/`StringWithoutAttribute` (`:179-224`).
- `ParseFQRN` already tolerates the three address forms the acceptance criteria demand: a
  trailing attribute path is preserved verbatim via `strings.Join(parts[2:], ".")`
  (`xclconfig:internal/resources/fqrn.go:78`), module-relative addresses resolve through
  `AppendParentModule` (`:150-164`), and lookup ignores `Attribute` entirely — `findResource`
  matches only `Module`/`Type`/`Name` (`xclconfig:state/state.go:223-225`).
- A caution for the variety segment: `case "variable"` requires **exactly one** segment and
  errors otherwise (`xclconfig:internal/resources/fqrn.go:108-116`), and `local` falls through to
  the `output` arm (`:83-106`) while having no `TypeLocal` constant and no `Local` type anywhere
  — so a parsed `local.x` currently round-trips out as `resource.local.x`. Existing asymmetry to
  not make worse.
- The bare stanza form is a small parser change by construction: the keyword whitelist is one
  `switch` with an erroring `default` (`xclconfig:internal/parser/parser.go:521-531,539-548`,
  message listing the four keywords at `:544`), and `blockResource` **already** handles the
  generic one-label case at `:571-572`. The gap is that `PluginRegistry.IsRegisteredType` covers
  only `RegisterType` Go types (`xclconfig:plugins/registry/plugin_registry.go:64-67`) — not
  builtins and not plugin types — so the `default` arm has no single "do you know this name"
  query to consult today.
- `Meta.Type`'s blast radius is the largest risk in this plan. It is the single key driving: HCL
  expression namespacing (`xclconfig:internal/parser/context.go:126,136`), state-file identity
  and round-trip (`xclconfig:state/file_state_store.go:63-73`, `state/state.go:195,224`),
  provider dispatch against `RegisteredType.SubType`
  (`xclconfig:plugins/registry/plugin_registry.go:182,332`), provider-less short-circuits
  (`xclconfig:internal/parser/lifecycle.go:75,356-365`), the resource `ID`
  (`xclconfig:state/state.go:199`, `internal/parser/parser.go:706-707`), and every event string
  (`xclconfig:internal/parser/lifecycle.go:368-370`). Roughly 30 production read sites plus
  ~15 test sites are enumerated in the agent findings; state deserialisation is the sharpest,
  because `metaMap["type"]` is read back from JSON and a missing or non-string value **silently
  skips the resource** (`xclconfig:state/file_state_store.go:69-73`).

### The error convention to follow

`plugins.ErrNotFound` is the archetype and its doc comment states the convention
(`xclconfig:plugins/errors.go:5-10`). Practised form: package-level `var Err… = errors.New(…)`,
a doc comment naming who returns it and that it is matched with `errors.Is`, producers free to
wrap with `%w` (both honoured — `xclconfig:plugins/adapter_test.go:137,151`), and identity carried
explicitly across the plugin wire rather than in the message
(`xclconfig:plugins/grpc_server.go:133`, `plugins/grpc_plugin_host.go:179-183`).

Important caveat for planning: **`ErrNotFound` is the only error in the repo that implements this
end to end.** The existing typed errors are plain structs with an `Error()` method and nothing
else, and receivers are inconsistent — value receivers in `state/` (`state/errors.go:13,21,35`),
pointer receivers in `plugins/registry/errors.go:17` and `types/register.go:8-18`. Nothing in
first-party code implements `Is`, `As` or `Unwrap` today. So the seven sentinels plus detail
structs are establishing the convention's *first* full instance beyond `ErrNotFound`, not copying
a settled in-repo pattern. `types.ErrTypeNotRegistered` is a cautionary example: it is never
matched anywhere, and its single production use formats it with `%s` not `%w`
(`xclconfig:types/register.go:38`), so the type is lost.

### Migration surface, measured

- 15 call sites, matching the design's claim exactly: 8 in examples, 7 in `querier_test.go`
  (`:69,94,116,132,147,167,184`). `grep -rn NewQuerier --include=*.go` yields 17 hits, 2 of which
  are `querier.go`'s own doc comment and declaration.
- Library docs: README is 1273 lines; `### Querying resources` is `xclconfig:README.md:267-284`.
  The README **does not cover reading outputs anywhere** — every `output` mention is
  HCL-authoring-side (`xclconfig:README.md:710-785`), and the `Output` type is not even nameable
  by a consumer because it lives in `internal/resources`. `docs/state.md:30-34` is the passage
  stating lookups are linear scans.
- Docs site: 6 matching lines across **4 distinct files, which is all 4 pages on the site** —
  `xcl-website:src/pages/index.mdx:96,162`, `examples/configuration-only.mdx:260,264`,
  `examples/plugins.mdx:295`, `examples/application-config.mdx:303`. `GetResources` and
  `ResourceCount` appear **zero** times site-wide, and bare `Querier` never appears — every hit
  is the constructor. Two lowercase "querier" mentions would be missed by a capital-Q grep
  (`xcl-website:src/pages/examples/plugins.mdx:289,294`).
- The landing page's headline example is `xcl-website:src/pages/index.mdx:89-97` (third of a
  three-block narrative, introduced at `:87`) and its accompanying blurb is the
  `🔎 Typed queries` `<FeatureCard>` at `:161-164`, whose body names `` `NewQuerier[T]` ``
  explicitly. Both are named in the acceptance criteria.

### Why compile-proofing site samples was a from-scratch build (evidence behind dropping it)

Definitive, by absence: the site has **no Go module, no `.go` file, no `go.sum`** anywhere; its
single workflow `xcl-website:.github/workflows/deploy.yml` installs Node 22 only and has **no
`pull_request` trigger** (`:3-6`), so nothing runs on PRs at all today; `package.json:7-12` has no
`test`/`lint` script; `Makefile:15-16`'s `check` is `npx astro check`, which does not inspect
fenced-block contents; and there is no snippet-extraction tooling in the dependency list. Samples
are embedded as hand-written fenced blocks with no include mechanism — verified by zero hits for
`?raw`, `readFileSync`, `import.meta.glob`, `getEntry`, `getCollection` across `src/`. The
`title="example/configonly/main.go"` meta strings look like paths but nothing reads them; they are
decorative labels. The only stated correctness process is prose: "When that code changes, update
the pages to match" (`xcl-website:README.md:39-40`).

Inventory for scoping: 29 fenced blocks total, **16 of them `go`**, of which 4 contain the
superseded API. A compile-proof mechanism would have had to cover all 16, not just the 4 that
change — which is the cost that made dropping it the reasonable call. **With it dropped, the site
work is confined to those 4 code blocks plus 2 prose mentions across 4 pages**, hand-edited, in
the same style the pages already use.

## Files examined

- `xclconfig:querier.go:1-90` — the entire superseded surface; 5 symbols; panics at `:32,59`; exact-string match at `:35`; `asType` returns a partially-filled value alongside its error at `:87-89`.
- `xclconfig:querier_test.go:1-189` — 7 tests + `setupQueryConfig` helper `:24-61`; testify `require` only, no table-driven tests, one scenario per function; fixture `internal/test_fixtures/config/query/main.xcl` (3 databases, 2 networks).
- `xclconfig:config.go:21-27,51,59,67,82,115,160` — `Config` and its complete method set; `FindResource` returns `any`; nil-state guards return empty rather than erroring.
- `xclconfig:types/resource.go:3,5-54,56-66` — `TypeResource` is a `var` not a `const`; `Meta`'s 11 fields and their `xcl`/`json` tags; `Meta` is a *named* field of `ResourceBase`, not embedded.
- `xclconfig:types/resource_helpers.go:10-25,19,28-53` — `findResourceBase`; the exact not-an-entity message; `st.Name()` is empty for anonymous structs; `GetMeta` returns a pointer *into* the resource.
- `xclconfig:types/register.go:8-18,20,22-39,32,38` — `RegisteredTypes` is `map[string]any` of pointer prototypes; `CreateResource` writes `meta.Type`; `ErrTypeNotRegistered` is never matched and is formatted with `%s`.
- `xclconfig:internal/resources/fqrn.go:12-22,53-142,60,78,108-116,150-164,166-177,179-224` — four fields, no subtype; the sibling regex; attribute preservation; the variable single-segment rule; `resource.` hardcoded in both formatters.
- `xclconfig:internal/parser/parser.go:521-531,539-548,544,559-578,565,568-572,610,706-707,723` — the keyword whitelist and its error message; `blockResource` in full; the single `CreateResource` call site; where `meta.ID` is assigned.
- `xclconfig:internal/parser/callbacks.go:168-175` — `Output.Value` is populated only during apply, only for `TypeOutput`, only when `CtyValue` is non-null; `r.(*resources.Output)` is an unguarded assertion.
- `xclconfig:internal/parser/lifecycle.go:75,82,87-90,356-365,368-370` — provider-less short-circuit keyed on `Meta.Type`; the existing `errors.As` idiom for `ResourceNotFoundError`; event strings built from `Meta.Type`.
- `xclconfig:internal/parser/context.go:62,71,77-83,108,118,126,136` — `Meta.Type` is the HCL namespace key, so changing its meaning reaches the expression language.
- `xclconfig:state/state.go:50,59,71,81-86,119,128,174,195,199,212-231,223-225,230` — `findResource` matches on three Meta fields and ignores `Attribute`; seven `ResourceNotFoundError` construction sites carrying three different kinds of string.
- `xclconfig:state/errors.go:8-15,17-24,26-40` — `ResourceNotFoundError` value receiver, no `Is`; `ResourceExistsError`; `UnknownTypesError`.
- `xclconfig:state/file_state_store.go:63-73,82-91,94,100,112` — state deserialisation reads `metaMap["type"]` and **silently skips** a resource whose type is missing or non-string; the JSON re-unmarshal overwrites what `CreateResource` set.
- `xclconfig:plugins/errors.go:1-10` — the whole file; the convention's doc comment.
- `xclconfig:plugins/plugin.go:32-42,57-70,108-126` — `RegisteredType{Type, SubType, Schema, Adapter}`; the type/subtype split already exists here.
- `xclconfig:plugins/registry/plugin_registry.go:17-23,43,57,64-67,72-85,89-104,136-172,150-152,163,177-205,314,326-340` — prototypes as concrete pointers vs schema-only plugin types; `IsRegisteredType` covers registered types only; two near-duplicate provider lookups; a panic at `:158-160`.
- `xclconfig:internal/schema/unmarshal.go:5-20` — the whole function; plain `encoding/json`, no `DisallowUnknownFields`.
- `xclconfig:internal/resources/output.go:8,11-17` — `Value any` has `json` but no `xcl` tag; `CtyValue` the reverse; the type is in `internal/`, so consumers cannot name it.
- `xclconfig:go.mod:1,3` — `go 1.25.0`, no `toolchain`, no `replace`, single module covering `example/*`.
- `xclconfig:.github/workflows/go.yml:1-31` — the only workflow; **installs Go 1.22, below the go.mod floor**; one `build` job; no vet, no lint, no matrix; no precedent for a second job.
- `xclconfig:Makefile:1-21` — codegen only; **no build/test/lint target**.
- `xclconfig:example/{appconfig,configonly,plugin}/` — three examples, per-example Makefiles, `main.go` + `main_test.go`, each exposing a shared `run(...)`; `example/plugin/main_test.go:29-51` builds the external plugin in `TestMain` via `exec.Command("go","build",...)`, which inherits `GOTOOLCHAIN` from the environment.
- `xclconfig:README.md:267-284,552-566,710-785,1042-1273` — the section to rewrite; the untyped `FindResource` + type-assertion example; outputs covered only HCL-side; the 232-line stale tail documenting `Process`/`ToJSON`/`ParseCallback`, all verified absent from the code.
- `xclconfig:docs/state.md:30-34` — the linear-scan passage the design says to annotate.
- `xclconfig:consts_win.go:1`, `consts_not_win.go:1` — the only first-party build-tag pair.
- `xcl-website:astro.config.mjs:1-14`, `ec.config.mjs:1-35`, `package.json:7-26`, `Makefile:1-19` — Astro 5 + MDX + Expressive Code (ordered before `mdx()` deliberately); no test/lint script; `check` is `astro check` only.
- `xcl-website:src/pages/index.mdx:87,89-97,96,161-164` — the headline example and the feature blurb, both named by acceptance criteria; the sample is already non-compilable.
- `xcl-website:src/pages/examples/{configuration-only,plugins,application-config}.mdx` — the other three pages; 6 superseded-API lines total site-wide, 2 of them lowercase "querier".
- `xcl-website:.github/workflows/deploy.yml:3-6,25-41` — Node 22 only, push-to-main only, no `pull_request` trigger.
- `xcl-website:README.md:33-40` — the page table (already stale, missing `application-config`) and the manual "update the pages to match" policy.

## External references

- [jumppad-labs/hclconfig#61](https://github.com/jumppad-labs/hclconfig/issues/61) — the issue the design cites as its source. Establishes that the awkward-lookup complaint predates this spec.
- Go 1.27 generic methods — the language-level fact the two-spellings split exists for. Verified locally rather than taken on trust: the installed toolchain is go1.27.0 and `GOTOOLCHAIN=go1.25.0` resolves from cache.
- Go `//go:build go1.N` semantics — a version tag raises the language version for that file alone, which is what lets the method file compile on 1.27 and drop out on 1.25. Precedent in-tree is vendored-only (`xclconfig:internal/xcl/diagnostic_typeparams.go:4`).
- `encoding/json` unknown/absent field behaviour — why `UnmarshalUntyped` cannot report a type mismatch on its own and why `As` must verify *before* converting.
- Astro + Expressive Code `<Code>` component and Vite's `?raw` import suffix — the mechanism that would let MDX render a real on-disk `.go` file rather than a hand-typed fragment. Relevant to the sample-compilation decision; not currently used anywhere in the site.

## Prior plans / specs consulted

- `20260919120639-config-only-types-and-examples` (plan + spec) — the immediate predecessor and the reason the current code looks as it does. It introduced `RegisterType`, the type-name clash gate, and **changed `Querier[T].FindResourcesByType` to take an explicit type name**, with the recorded reasoning that "a zero `T` has no `Meta.Type` and plugin look-alikes can't be matched back to `T`". That argument is exactly what `TypePath(reflect.Type)` now answers for registered types, and it is also why `All[T]` cannot serve plugin types. It also built the three examples and their shared `run(out, dir)` shape that this plan migrates.
- `20260918165700-provider-lifecycle-read`, `20260919152915-destroy-cycle` — establish the lifecycle and destroy paths that `Meta.Type` changes reach through `handledWithoutProvider`. Consulted for blast radius only.
- `xclconfig` knowledge, `architecture/ux-flow.md` — **documents the superseded `Querier` API as current architecture**, including `q := xcl.NewQuerier[T](config)` in its layer diagram and three "Open Questions" about Querier's shape. This work makes the entry wrong, so it is a deliverable, not just collateral. Raised with the user rather than silently overruled.
- `xclconfig` knowledge, `glossary/entity.md` — binding. Mandates the `entity` umbrella term and states the code has not caught up. Confirms `Entities()`/`EntityCount()` are required by the knowledge base independently of the design.
- `xclconfig` knowledge, `gotchas/hcl-halts-on-malformed-block-header.md` — relevant to testing the new bare stanza form: a fixture with two malformed block headers yields **one** diagnostic, so negative-parse tests must assert on content, not counts.
- `xclconfig` knowledge, `conventions/never-modify-dependencies.md` — HCL lives in-tree at `internal/xcl` under MPL-2.0. If the bare-stanza work touches it, the HashiCorp header stays, `// Modifications Copyright (c) Jumppad Labs` is added, and `internal/xcl/UPSTREAM.md` records it.
- `xcl-website` knowledge — all six category files are unmodified 9-line stubs. Nothing is recorded about either the old or the new API.

## Open assumptions

1. **`Meta.Type`'s new meaning does not break the HCL expression language.** `Meta.Type` is the namespace key at `xclconfig:internal/parser/context.go:126,136`, so `resource.<Meta.Type>.<name>` in an expression resolves through it today. If `Meta.Type` becomes `"resource"` for every resource stanza, that keying must move to `Meta.Subtype` or references stop resolving. **Not verified by execution.** If wrong, the implement workflow must STOP — this would silently break every configuration that references another resource.
2. **State written by v1 is discarded, not migrated.** The spec's non-goals allow regeneration. But `xclconfig:state/file_state_store.go:69-73` *silently skips* a resource whose `meta.type` it cannot resolve, so a stale state file degrades quietly rather than erroring. Assumed acceptable; worth an explicit check.
3. **The 1.25.0 floor job can actually pass on CI.** Locally it can. On CI it currently cannot be trusted, because the existing workflow installs Go 1.22 (`xclconfig:.github/workflows/go.yml:22`), below the `go.mod` floor. Assumed the workflow's Go version is corrected as part of this work.
4. **`plugins/example` stays red on a fresh checkout, by decision.** `plugins/example/e2e_test.go` requires a pre-built binary at `./build/example`, has no `TestMain` build step and no `t.Skip`, and `plugins/example/build/` is gitignored — so `go test ./...` fails on a clean clone. Pre-existing and unrelated to this feature. **User decided 2026-09-21 to leave it and track it separately** (contrast `example/plugin/main_test.go:29-51`, which builds its own binary in `TestMain` and is the obvious model whenever it is fixed). Consequence to carry: until it is fixed, a full-suite run is not a clean signal, so the new floor job must be scoped to `go build`, and any "the suite passes" claim in this plan has to name this known failure rather than assert green. The sibling `example/configonly` failure is **not** an open assumption — it is in scope and fixed by this plan (see below).
5. **`TypePath` can distinguish the two stanza forms from registration alone.** `RegisterType(name, resource)` takes a single name (`xclconfig:plugins/registry/plugin_registry.go:43`) with nothing recording whether that name is a kind-led variety or a bare-form type. Assumed the registration API grows a way to express this; if it does not, "a Go type is registered under one form only" cannot be enforced.
6. **No consumer outside these two repos depends on the superseded surface.** Stated by the user during the spec interview (unreleased v2). Not independently verified.

## Scope decisions taken during discovery (user, 2026-09-21)

Recorded here because each narrows or widens what the plan must deliver, and a cold reader will
otherwise re-derive the rejected options.

- **Compile-proofing of docs-site samples is dropped.** Pages still migrate to the new surface; they are not machine-verified. One acceptance criterion is partly descoped as a result — see the alternatives section above for exactly which clauses survive.
- **`example/configonly` is repaired in this plan.** It is already broken on HEAD: `main.go` never calls `c.Destroy()` and never prints `## Destroyed`, while `main_test.go:447-465,467-474,479-493` assert all three behaviours. This plan rewrites that file's query call sites anyway, so the fix is in hand rather than a scope widening. Note its doc comment at `main.go:12-13` already *claims* the destroy behaviour, so the code is what is wrong, not the tests.
- **`plugins/example`'s pre-built-binary failure is left alone**, tracked separately (assumption 4).
- **`architecture/ux-flow.md` is updated as a plan deliverable.** The entry documents `Querier` as current architecture, including in its layer diagram and its three "Open Questions" about Querier's shape. Wording goes through `spek-knowledge` propose-then-confirm; nothing is written without explicit confirmation. This is the one case where an entry is superseded by an approved spec rather than merely disagreeing with the code, which is why it was put to the user rather than decided here.

## Drafting assumptions

# Judgement calls

### Both registered repos are in scope (discovery)
- **Decision**: Plan across `xclconfig` and `xcl-website`, and load always-applied knowledge for both.
- **Rationale**: The spec's final requirement group names the documentation site explicitly, including the landing page's headline example and its feature description. The site is a separate registered repo with its own root.
- **Rejected**: Planning `xclconfig` alone and treating the site as follow-up work — the spec's acceptance criteria make site migration a condition of done, and the user confirmed during the spec interview that all site pages are in this spec.

### Research fanned out per concern, not per repo (discovery)
- **Decision**: Four parallel agents — query surface + call sites, data model + error convention, docs site, CI/toolchain/README — with each given its repo root explicitly.
- **Rationale**: The `Meta.Type` blast radius and the docs-site sample inventory are independent questions with little overlap, so they parallelise cleanly. Repo roots were passed explicitly because a sub-agent inherits the working directory, not the registry.
- **Rejected**: One agent per repo — the `xclconfig` side is far too large for a single pass, and the `Meta.Type` blast radius alone needed a dedicated sweep.

### Design vs. spec conflict on the bare stanza form resolved in the spec's favour (discovery)
- **Decision**: Treat "Both stanza forms can be declared" as in scope and plan for the parser change.
- **Rationale**: The design lists implementing the bare `container "nics"` form under "Not in scope", but the spec requires it and labels it "a facilitating change rather than part of the referenced API design". The spec's own constraint says the design governs *how* and the spec governs *what must be true* — this is a what.
- **Rejected**: Deferring it to match the design's scope note. That would leave the acceptance criterion "The two stanza forms address differently" unprovable, since only one form could be declared.

### `architecture/ux-flow.md` treated as work to do, not as stale (discovery)
- **Decision**: Record the entry's conflict with the new surface and put the question to the user rather than planning around it or silently ignoring it.
- **Rationale**: The standing rule is that a knowledge entry outranks the code and a difference is work to do. Here the *spec* supersedes the entry, which is the one case the rule does not cover, so it is the user's call.
- **Rejected**: Silently overruling the entry, and equally, planning to the entry's description of `Querier` — both would contradict an approved spec.

### Pre-existing CI failures recorded as open assumptions, not absorbed into scope (discovery)
- **Decision**: Document the two red paths (configonly destroy tests; `plugins/example` needing a pre-built binary) as open assumptions and surface them for a scope decision rather than assuming either answer.
- **Rationale**: Both are independent of the query API, but both block acceptance criteria this spec does own ("every example runs to completion", "the floor job passes on every change from merge onward"). Scaling scope up or down is the user's call, not mine.
- **Rejected**: Quietly folding the fixes in (widens scope beyond the ask) or quietly ignoring them (leaves acceptance criteria unmeetable).

### Chosen direction: split the data model first, then build the surface (architecture)
- **Decision**: Sequence the work as (1) kind/variety split across `Meta`, `FQRN`, the parser and every read site, with the superseded querier minimally adapted to stay green; (2) the new surface and error taxonomy, deleting `querier.go`; (3) consumer migration — examples, tests, README, `docs/state.md`, the site; (4) the floor CI job and the knowledge entry.
- **Rationale**: `FindByType[T]("resource","container")` cannot be implemented correctly while `Meta.Type` holds the variety for resource stanzas and the kind for everything else, so the surface depends on the split. The split is also the riskiest edit (~30 production read sites, reaching the expression language and the state format), and isolating it keeps a regression bisectable.
- **Rejected**: *Surface-first* — the kind axis has no correct implementation against conflated data, so it would be built then immediately rewritten. *Big-bang single change* — no intermediate green state, so a broken expression reference would be indistinguishable from a broken lookup.

### HCL expression namespacing moves to `Meta.Subtype` for resource-kind entities (architecture)
- **Decision**: Treat the namespace key at `internal/parser/context.go:126,136` as its own acceptance-tested step: it becomes `Meta.Subtype` for resource-kind entities and stays `Meta.Type` for `variable`, `output` and `module`.
- **Rationale**: It is the one place where the split reaches the configuration language itself rather than the Go API. Missing it breaks every cross-resource reference in every configuration, silently and totally, and it is invisible from the public surface so no API test would catch it.
- **Rejected**: Folding it into a general "update the `Meta.Type` read sites" task — it would be one line among thirty and is far too consequential to be reviewed that way.

### Stale-state resources are reported, not silently skipped (architecture)
- **Decision**: Turn the silent skip at `state/file_state_store.go:69-73` into a reported error, while writing no state migration.
- **Rationale**: The spec's non-goals permit regenerating state, so migration is genuinely out of scope. But a resource vanishing from state without a word is precisely the "unanswerable question answered as none" failure the spec exists to remove, and the split makes it newly likely by changing what `meta.type` holds.
- **Rejected**: Leaving the skip as-is (turns a stale state file into silent infrastructure loss); writing a v1→v2 state migration (explicitly a non-goal).

### `RegisterType` keeps its meaning; the bare form gets a sibling entry point (architecture)
- **Decision**: Leave `RegisterType(name, resource)` as the kind-led registration it is today, and add a separate entry point for the bare stanza form, so `TypePath` can derive `{"resource","container"}` or `{"container"}` from the registration.
- **Rationale**: Every existing caller passes a variety intending the kind-led form. Changing the meaning would silently re-address existing registrations; a second entry point makes the choice explicit at the call site and enforces the design's "registered under one form, never both" without a flag argument.
- **Rejected**: Encoding the form in the name string (e.g. `"resource.container"`) — stringly-typed and easy to get subtly wrong; an options/variadic parameter — heavier than one well-named function for a binary choice.

### One registry query for "is this a known type name" (architecture)
- **Decision**: Invert the existing three-source scan in `checkTypeName` (`plugins/registry/plugin_registry.go:89-104`) into a single query the parser can ask, instead of widening `IsRegisteredType` or adding a fourth enumeration of the type sources.
- **Rationale**: `IsRegisteredType` covers `RegisterType` types only — not builtins, not plugin-provided types — so the parser's keyword `default` arm has nothing correct to consult today. The registry already knows how to scan all three; reusing that keeps one source of truth and keeps the interface small.
- **Rejected**: Widening `IsRegisteredType`'s meaning (would silently change the provider-less short-circuit at `internal/parser/lifecycle.go:356-365`, which relies on its current narrow meaning); enumerating the three sources again in the parser (a fourth copy to drift).

### New error detail structs use pointer receivers (architecture)
- **Decision**: Give the new detail structs pointer receivers, and add `Is` to `state.ResourceNotFoundError` on its existing value receiver rather than changing it.
- **Rationale**: Receivers are inconsistent across the repo today, so some choice has to be made. Pointer receivers match `plugins/registry.TypeNameClashError`, the only typed error matched with `errors.As` in production code. `ResourceNotFoundError` is left a value type because seven construction sites and existing `errors.As` call sites depend on that, and changing it is gratuitous churn outside this feature's scope.
- **Rejected**: Value receivers throughout (would mean changing `TypeNameClashError`, unrelated to this work); normalising every existing error's receiver (a repo-wide refactor the spec does not ask for).

### Conventions selected, and four deliberately dropped (architecture)
- **Decision**: Apply the entity glossary, the testing conventions, code style, the error convention, real-apply test state, the dependency/HCL-fork rules and stdlib preference. Explicitly drop database/external-services, the handler-and-lifecycle clauses of patterns-and-architecture, structured logging, and the `/cmd`-`/pkg` project structure — each with a stated reason.
- **Rationale**: The instruction is to record relevance rather than ask. Listing the dropped ones with reasons makes it visible that the knowledge base was read, and prevents a reviewer re-raising them.
- **Rejected**: Listing every loaded convention (an undifferentiated list signals the knowledge base was not actually consulted); listing only the kept ones (leaves the omissions looking accidental — notably the project-structure entry, which this plan knowingly does not follow because the repo does not).

### Expression namespacing and nested-block refusal listed as components in their own right (components)
- **Decision**: Break "expression namespacing" and "nested-block refusal" out as named components rather than treating them as incidental edits inside parsing and type helpers.
- **Rationale**: Each owns a distinct responsibility that the plan is held to by a specific acceptance criterion, and each is a place where the old behaviour was silent — one would break the expression language invisibly, the other currently answers a legitimate question with a panic or an empty result. A reviewer scanning the component list should see both.
- **Rejected**: Folding them into "block parsing" and "type registry" — they would disappear into a general edit and the load-bearing risk would not be visible at review.

### Both spellings listed as separate components despite sharing one implementation (components)
- **Decision**: List the portable functions and the generic methods as two components alongside the shared implementation, rather than one "lookup surface" component.
- **Rationale**: They differ in exactly the way the spec cares about — one is excluded by build constraint below the required toolchain, the other is what examples and documentation use — and the requirement that they cannot drift is only meaningful if they are visibly two things delegating to one.
- **Rejected**: A single combined component (hides the delivery mechanism the design fixes and makes the no-drift requirement read as trivially true).

### Component list covers changed existing components, not only new ones (components)
- **Decision**: Include state persistence, the lifecycle-adjacent readers, examples, documentation, CI and the knowledge entry as components, even though several are changed only modestly.
- **Rationale**: The instruction asks for meaningfully-changed existing components, and in this feature the widest risk sits in existing ones rather than new code — the metadata record alone is read by persistence, dispatch, lifecycle, events and expressions. Omitting them would make the plan look smaller than it is.
- **Rejected**: Listing only new components (understates the blast radius, which is the main thing a reviewer needs to see here).

### `KnownType` and `RegisterBareType` named provisionally (data_structures)
- **Decision**: Give the two new registry entry points the working names `KnownType` and `RegisterBareType`, and state their contracts precisely rather than leaving them unnamed.
- **Rationale**: The design fixes `TypePath`'s name and signature but is silent on both of these, because it treats the bare stanza form as out of scope while the spec requires it. A plan that describes a method without naming it is not implementable; the names follow the repo's existing style (`IsRegisteredType`, `RegisterType`, `RegisterPlugin`).
- **Rejected**: Leaving them unnamed (not actionable); reusing `IsRegisteredType` with widened meaning (the lifecycle depends on its current narrow meaning — recorded separately in the architecture calls).

### `Meta` shown as a partial struct rather than in full (data_structures)
- **Decision**: Show only `Type` and `Subtype` in the metadata block, with the nine unchanged fields named in a comment.
- **Rationale**: The instruction is to capture contracts, not internal representation, and keep code blocks concise. The two axes are the contract; reproducing all eleven fields with their tags would bury the one change that matters.
- **Rejected**: Reproducing the full struct (noise, and it would date the plan against unrelated field changes); omitting the struct entirely (the empty-variety rule for single-label stanzas is a real contract that needs stating).

### Detail-type fields described rather than declared (data_structures)
- **Decision**: State what each error detail type must carry — queried address or segments, Go type, and the count for the not-unique case — without writing out seven struct declarations.
- **Rationale**: Per-field detail is explicitly directed to `context.md`. What plan.md must pin down is that each failure reason is independently recognisable and its detail recoverable, which the spec requires; the exact field names are implementation detail.
- **Rejected**: Seven full struct declarations (crosses into source, and the instruction says this is a plan not source); naming only the sentinels (would lose the "count found" requirement, which is an explicit acceptance criterion).

### The axis split is worked through deliberately, not compiler-driven (implementation_detail)
- **Decision**: State explicitly that changing the conflated field's meaning produces **no compiler errors** at its read sites, because both axes are strings, and that every reader must therefore be walked deliberately.
- **Rationale**: This is the single most likely way the implementation goes wrong: a normal Go refactor leans on the build to find call sites, and here the build stays green while behaviour changes underneath. A plan that does not say so invites exactly that mistake.
- **Rejected**: Describing it as an ordinary field addition (understates it); proposing a temporary type change to force compiler errors (a larger, riskier change than the split itself, and it would churn the serialization boundary twice).

### Documented form and demonstrated form deliberately differ (implementation_detail)
- **Decision**: Call out that prose presents the method spelling as the destination while every runnable example and sample uses the portable function spelling, and require the documentation to explain why.
- **Rationale**: The design fixes both halves — methods are "the destination and the documented form", examples "use the function form so they build on the floor version" — but the combination reads as an inconsistency to anyone encountering it cold, and an unexplained inconsistency gets tidied away by a well-meaning contributor.
- **Rejected**: Showing methods in the documentation samples (they would not compile for a reader on the floor toolchain, defeating the point); saying nothing about the asymmetry (leaves it looking like an oversight).

### Linear-scan performance restated as an explicit non-change (implementation_detail)
- **Decision**: State in the plan that no index or cache is introduced and that this is recorded in the documentation.
- **Rationale**: The new surface reads as though it might be doing something cleverer, and the spec lists indexing as a non-goal while the design notes lookups remain linear scans. Saying so prevents a reviewer reading the absence as an oversight and prevents an implementer adding one opportunistically.
- **Rejected**: Leaving it unstated (the section is meant to let a reviewer spot design gaps; an unstated non-goal reads as a gap).

### Design/spec divergence surfaced in Dependencies rather than left in research only (dependencies)
- **Decision**: State the bare-declaration-form divergence in the Dependencies section, on the design document's own bullet, with the resolution and the rule that produced it.
- **Rationale**: The section must name every design the plan was built on, and a reader checking the plan against the design will hit this difference immediately. Naming it beside the design, with the spec's own precedence rule quoted, prevents it reading as the plan having ignored a binding document.
- **Rejected**: Recording it only in `research.md` (the reader most likely to be confused is reading plan.md); treating the design as governing and dropping the bare form (would leave an acceptance criterion unprovable — already recorded as a discovery-step call).

### An "explicitly not depended on" group added to the section (dependencies)
- **Decision**: Close the section with the three things this plan deliberately does *not* depend on — a sample-compilation mechanism, a state migration, and a fix for the plugin example's failing test.
- **Rationale**: All three were live options during planning and two were settled by user decision. A dependency section that lists only what is depended on cannot distinguish "not needed" from "not considered", and these particular absences each have a visible consequence downstream.
- **Rejected**: Omitting them (the descoped acceptance criterion and the known-red test would surface as surprises during implementation).

### Three success metrics split rather than classified whole (testing_approach)
- **Decision**: Split metrics 1, 4 and 6 into a behavioural half and a manual half rather than forcing each into one classification.
- **Rationale**: Each genuinely contains two claims of different kinds. Metric 1 pairs "every call site migrated" (assertable — the code will not compile otherwise) with "no more code than before" (a diff property). Metric 4 pairs a bundled example (runs in the test suite) with a documentation sample (no longer compile-checked). Metric 6 pairs "the job exists and passes" (CI configuration plus a green run) with "on every change from merge onward, never absent or skipped" (a claim about future repository history). Classifying each whole would either over-claim automation or discard verification that is genuinely available.
- **Rejected**: Marking each entirely manual (loses real automated coverage and makes the plan look weaker than it is); marking each entirely behavioural (over-claims — no test can assert a property of the repo's future history).

### The descoped acceptance-criterion clause is restated in the testing approach (testing_approach)
- **Decision**: Name the dropped compile-proofing clause explicitly under deliberate gaps, spelling out which clauses of that acceptance criterion survive and which does not.
- **Rationale**: The implement workflow reads the plan, not the spec. A criterion silently unmet downstream looks like an implementation failure rather than a recorded scope decision, and the person hitting it would have no way to tell the difference.
- **Rejected**: Recording it only in the dependencies section or in `research.md` (the testing approach is where an implementer looks to see what "done" means).

### Coverage concentrated on invisible failures rather than spread evenly (testing_approach)
- **Decision**: Direct the heaviest coverage at the error taxonomy, the two-axis split's non-obvious readers, and conversion mismatches, rather than distributing it evenly across the seven lookup operations.
- **Rationale**: The lookup operations are thin pass-throughs over one implementation and largely fail loudly. The failures worth spending tests on are the ones that are silent today: a question answered "none", a struct returned half-filled, and a reference that stops resolving because a reader was left on the wrong axis.
- **Rejected**: Even coverage per operation (would produce seven near-identical happy-path tests and under-test the three genuinely risky areas).

### Four milestones, split data-model / surface / consumers / documentation (milestones)
- **Decision**: M1 the two-axis split, addressing and the bare stanza form; M2 the whole lookup surface including published values and the error taxonomy; M3 bundled consumers plus the floor CI job; M4 all documentation including the site and the knowledge entry.
- **Rationale**: Follows the architecture's chosen ordering and keeps each milestone independently deliverable. M1 leaves a working library on the old helper; M2 delivers the entire user-facing change at once, which matches the user's "whole design, one change" scope decision; M3 and M4 are migration, and separating them keeps the compile-verified work (examples, tests, CI) apart from the prose-only work (README, site, knowledge), which have different validation methods.
- **Rejected**: Splitting published values into their own milestone — they are part of one lookup surface and separating them would deliver a surface that visibly cannot answer a whole class of address. Folding documentation into M3 — it would mix work proven by the compiler with work proven only by review, blurring what "done" means.

### Milestone 1 declared as groundwork, with its justification stated in the paragraph (milestones)
- **Decision**: Keep the data-model split as its own milestone and say plainly in its "What changes" that much of it is groundwork, explaining why it earns a milestone.
- **Rationale**: The instruction requires a largely-internal milestone to say so explicitly and justify itself. The justification is real: the field it changes reaches the expression language, state identity and provider routing, and landing it separately is what makes the following milestone reviewable as an API change rather than an API change tangled with a data migration.
- **Rejected**: Merging it into M2 (produces one enormous milestone whose failure modes cannot be told apart); presenting it as user-facing only (would overstate the visible benefit and hide that its value is mostly enabling).

### The stale-state reporting change placed in Milestone 1 (milestones)
- **Decision**: Put "a stored item whose recorded kind cannot be resolved is reported rather than dropped" in M1 rather than with the error taxonomy in M2.
- **Rationale**: It is a direct consequence of the axis split — the split is what makes previously-written state unresolvable — so it belongs with the change that causes it. Deferring it would leave a window in which the split silently shrinks a loaded configuration.
- **Rejected**: Grouping it with the error work in M2 (thematically neat, but leaves M1 shipping a known silent-loss path).

### Reference resolution given its own phase (phases)
- **Decision**: Split "keep references between items resolving" out as Phase 1.2 rather than folding it into the axis split in 1.1.
- **Rationale**: It is the only reader of the changed field whose failure is total and silent from outside the library — nothing about the public surface would look wrong while every configuration that refers to anything stopped working. A phase boundary forces it to be validated on its own terms.
- **Rejected**: Folding into 1.1 (one line among thirty, reviewed as part of a large mechanical sweep).

### The example repair sequenced before the migration (phases)
- **Decision**: Repair the configuration-only example's teardown (3.1) before migrating examples to the new surface (3.2), rather than after or as part of it.
- **Rationale**: The repair is against currently-failing tests, so doing it first gives the migration a green baseline to land on. Reversed, a failure after migration would be ambiguous between the pre-existing defect and the new surface.
- **Rejected**: Repairing during the migration (mixes a bug fix into an API migration in one diff); repairing after (leaves the migration unverifiable against a red suite).

### The floor CI job scoped to building, not testing (phases)
- **Decision**: The minimum-version job runs a build only, not the full test suite.
- **Rationale**: The full suite cannot pass on a fresh checkout because of the plugin example's pre-built-binary requirement, which the user decided to leave tracked separately. A job that is known-red from the day it lands teaches contributors to ignore it, which defeats its purpose. The spec's wording asks for a job that "builds the module against the pinned minimum Go version", so this satisfies it as written.
- **Rejected**: Running the full suite (lands permanently red); fixing the plugin example to enable it (explicitly out of scope by user decision).

### Sixteen phases across four milestones (phases)
- **Decision**: 5 phases in M1, 5 in M2, 3 in M3, 3 in M4.
- **Rationale**: Phase size is set by what can be validated as one outcome. M1 and M2 are large because the axis split and the lookup surface each contain several independently-checkable behaviours; M3 and M4 are migration with naturally smaller units. Every phase has acceptance criteria that can be read without running anything.
- **Rejected**: Fewer, larger phases (the axis split in particular would become unreviewable); finer phases per operation (would produce near-duplicate phases for lookups that share one implementation).

### The README's stale tail explicitly excluded in the phase detail (phases)
- **Decision**: Name the 232-line block documenting methods that no longer exist as out of scope in Phase 4.1's technical detail, rather than leaving it unmentioned.
- **Rationale**: The spec lists this cleanup as a non-goal, but anyone rewriting the querying section will be looking straight at it and will reasonably assume it is in scope. An explicit exclusion is cheaper than the scope creep.
- **Rejected**: Silence (invites the creep); including it (contradicts a stated non-goal).

### Sentinels live in the project's existing errors package (open_questions, revised at walkthrough)
- **Decision**: Declare all seven sentinels in the project's existing `errors` package and re-export them from `xcl`, rather than declaring the not-found sentinel in `state`.
- **Rationale**: `config.go:10` imports `xcl/state`, so `state` cannot import `xcl` back — a neutral home both can depend on is required for one not-found concept. The `errors` package already exists (`config_error.go`, `parser_error.go`) and imports only `internal/xcl` and one third-party package, so it creates no cycle and is not a new package at all. Keeping all seven sentinels together also avoids the split home the earlier draft would have produced. The re-export follows `config.go:16` and exists purely so consumers are not forced to alias-import a package named `errors` in order to keep using stdlib `errors.Is`.
- **Rejected**: Declaring the not-found sentinel in `state` and the other six in `xcl` — this was the original draft, **corrected by the user during the walkthrough**. It split one vocabulary across two packages and gave the state layer ownership of a concept belonging to the lookup surface. The original rationale for rejecting a shared package ("an unnecessary third package for one value") was simply mistaken: the package already existed and the codebase was not checked before the claim was made. Also rejected: comparing by message text (fragile, defeats identity matching).


### Enumeration coverage verified rather than assumed (open_questions)
- **Decision**: Check whether state holds variables, published values and modules rather than leaving it as a "confirm during implementation" note.
- **Rationale**: An acceptance criterion depends on it — if state held only resources, the enumeration requirement would need a different design, which is a planning-time problem, not an implementation-time one. `internal/parser/parser.go:463-467` appends every parsed resource with no kind filter, so it holds.
- **Rejected**: Leaving the "confirm" note in the phase detail (would have shifted a design risk into implementation).

### Three open questions kept, each with an explicit stop condition (open_questions)
- **Decision**: Keep exactly three, and for each distinguish what the implementer should simply fix from what must stop and ask.
- **Rationale**: All three are genuinely only answerable by exercising code, and all three have a foreseeable benign outcome and a foreseeable outcome that would change the design. Without that distinction an open question either halts work unnecessarily or gets absorbed silently when it should not be.
- **Rejected**: An empty section (would be dishonest — the axis split's silent-failure mode is a real residual risk); listing more (everything else was answerable now and was answered).

### Out-of-scope items grouped by the authority that excluded them (out_of_scope)
- **Decision**: Split the section into three groups — excluded by user decision during planning, excluded by the spec's non-goals, and excluded by the chosen design — rather than one flat list.
- **Rationale**: The groups carry different weight and different reversibility. A user decision can be revisited by asking; a spec non-goal cannot be reopened inside this plan; a design exclusion is the planner's call and is the one a reviewer might legitimately push back on. Flattening them would hide which is which.
- **Rejected**: A single undifferentiated list (obscures who decided what, and invites a reviewer to re-litigate spec non-goals).

### The deprecated-alias exception stated inside the no-compatibility-shim exclusion (out_of_scope)
- **Decision**: Note, within the bullet excluding migration shims for outside consumers, that the renamed enumeration and count operations *do* keep deprecated aliases.
- **Rationale**: The two read as contradictory otherwise — the spec both forbids compatibility shims for external consumers and requires the old enumeration names to survive as deprecated aliases. Stating the distinction (external consumers versus in-repo callers) prevents an implementer resolving the apparent conflict the wrong way.
- **Rejected**: Leaving them in separate bullets (the contradiction stands unexplained at exactly the point someone acts on it).

### Feature slug taken from the plan name, not minted separately (assemble)
- **Decision**: Use the CLI-minted plan name `20260921093100-query-api-v2` as the document namespace, rather than proposing a zero-padded `NNNN-description` slug and asking the user to confirm it.
- **Rationale**: The slug skill describes an `NNNN-` numbering scheme this project does not use. Every existing plan in the store is timestamp-prefixed (`20260714080036-…` through `20260919152915-…`), with no numeric prefix anywhere, and the project is configured to mint IDs by timestamp. Asking the user to confirm a slug in a format the repo has never used would be a confusing question with one sensible answer.
- **Rejected**: Introducing an `NNNN-` prefix for this plan alone (inconsistent with all five existing plans and with the configured ID method); asking anyway (a question whose premise does not hold).

## Rehydration cues

- `spektacular design read --data '{"source":"design","path":"querier-api-v2.md"}'` — the binding design. Read this first; architecture is built on it, not re-derived. Its "Files to change" table is the canonical work inventory.
- `spektacular spec file read 20260921093100-query-api-v2.md` — requirements and acceptance criteria. Where spec and design disagree on *what must be true*, the spec governs; on *how*, the design governs.
- `spektacular knowledge always-applied --tier repo --filter xclconfig --filter xcl-website` — conventions + glossary. The `entity` glossary entry is binding on naming.
- `spektacular repo list` — the two roots. Never assume the working directory holds either repo's code.
- Re-read cold: `xclconfig:querier.go` (90 lines, the whole superseded surface), `xclconfig:internal/resources/fqrn.go:53-224` (parse + format, both need the variety segment), `xclconfig:internal/parser/parser.go:519-578` (keyword whitelist + `blockResource`), `xclconfig:plugins/errors.go` (10 lines, the error convention), `xclconfig:state/errors.go:8-15` (the `Is` method's home).
- To re-derive the `Meta.Type` blast radius: `grep -rn 'meta\.Type\|Meta\.Type' --include='*.go'` from the `xclconfig` root, excluding `internal/xcl` and `internal/cty`.
- To re-derive the migration surface: `grep -rn 'NewQuerier' --include='*.go'` in `xclconfig` (17 hits, 2 are declarations) and `grep -rni 'querier' src/` in `xcl-website` (catches the two lowercase mentions a capital-Q search misses).
- To exercise the floor: `GOTOOLCHAIN=go1.25.0 go build ./...` — a plain `go build` uses 1.27.0 and proves nothing.
