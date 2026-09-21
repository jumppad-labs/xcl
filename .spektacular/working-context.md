# Working context — 20260921093100-query-api-v2

## Origin
User opened the spec with: "This change should relate to the querier-api-v2 design
document, it relates to the updates to the UX when accessing config items."

So the spec's subject is the **developer UX of reaching into a parsed configuration**,
and the worked design already exists at design source `design`, path `querier-api-v2.md`
(status: draft, created 2026-09-20). The design cites
[jumppad-labs/hclconfig#61](https://github.com/jumppad-labs/hclconfig/issues/61) as its source.

## Decisions so far
- Spec name chosen by the user from three offered: **`query-api-v2`** (over
  `config-lookup-surface` and `entity-query-api`). Minted as
  `20260921093100-query-api-v2`.
- The design document is **referenced, not copied**. The spec carries requirements and
  acceptance criteria; the worked API surface stays in `design/querier-api-v2.md`.
  A `design ref add` must be recorded at the technical-approach step.

## Project shape (from `repo list` / `config.yaml`)
- Two registered repos:
  - `xclconfig` — root `/home/nicj/code/github.com/jumppad-labs/xcl` (the Go library).
  - `xcl-website` — root `/home/nicj/code/github.com/jumppad-labs/xcl-website`, role
    `documentation`, Astro 5 + MDX + Tailwind v4, served at xcl.dev.
- `spec.id_method: timestamp`, so IDs are minted by the CLI; never pass `id`.
- Design source: `design` → `.spektacular/design`.
- Because this is a public API change, whether `xcl-website` needs doc updates is an
  open question to put to the user during the interview.

## Problem the design states (summary, for framing the interview)
Parsing a configuration hands back a flat, untyped collection; reaching into it is more
work than unmarshalling YAML into a struct. Specific pains named:
- `Querier[T]` binds `T` at construction, but all 15 in-repo call sites construct a
  querier and make exactly one call, usually of a different type each time.
- `Meta.Type` conflates two axes (stanza keyword vs. resource subtype), so
  `FindByType("resource")` silently returns empty, and mismatched structs pass through
  `schema.UnmarshalUntyped` returning half-empty values with no error.
- Outputs are not reachable by address through the same lookup as everything else.
- Nested blocks (Container, Port, Volume) are not entities, but querying one silently
  returns empty rather than saying so.

## Open questions for the interview
- Scope confirmation: is the spec the whole of the design, or a subset?
- Breaking-change appetite (design frames it as v2, unreleased).
- Does `xcl-website` need documentation updates as part of this?
- The Go 1.27 generic-methods split (methods + portable functions behind a build tag) —
  is that settled or still open?

## Interview answers (2026-09-21)
User answered four questions; all open questions above are now closed:
1. **Scope** — "The whole design, one change." Not phased, not a subset.
2. **Consumers** — "Unreleased v2, no external migration." No deprecation window, no
   migration guide. In-repo callers + README + docs site are the only things to update.
3. **Docs site** — "Yes, all four pages, in this spec." User chose the narrower option
   over "plus new outputs/errors docs", so migrating existing pages is the requirement;
   new site pages for outputs/error semantics are **not** in scope. README does gain a
   reading-outputs section (that is the library, not the site).
4. **Success** — all four measures selected: no silent-empty results, fewer lines at the
   call site, outputs reachable by address, no half-populated structs.

## Grounding done
Grepped xcl-website for the v1 API before asking, so the docs question was specific
rather than generic. Four pages hit, exact lines recorded in
`.spektacular/work/20260921093100-query-api-v2/interview.md`. The homepage hero snippet
(index.mdx:96) is one of them, which is why the docs question was worth asking rather
than assuming.

## Learnings to carry forward
- The design doc is unusually complete — it already names files to change, breaking
  changes, and a "Not in scope" list. Section drafts should lean on it rather than
  re-deriving, and the spec must stay at requirements altitude, pointing at the design
  for the worked API surface.
- The Go 1.27 two-spelling split is **settled**, not an open option: verified in the
  design against a real toolchain both ways. Do not present it as a decision to make.
- Interview file written to `.spektacular/work/20260921093100-query-api-v2/interview.md`;
  every later section drafts from it.

## Overview step (2026-09-21)
- **User rejected the first overview draft.** Correction: "we have the querier but the UX
  on this does not take advantage of go 1.27 and even the stuff that is there does not
  provide the best developer experience." The framing must be **improving an existing
  query capability**, not introducing one where none exists. Do not describe the current
  state as "no way to query" anywhere in the spec.
- Two distinct complaints to keep separate throughout: (a) the surface does not exploit
  Go 1.27 generic methods, and (b) even the existing surface has poor ergonomics
  (construct-then-call, silent empties, blank structs). Both must show up in requirements.
- User chose to **name Go 1.27 explicitly** in the overview, knowingly overriding the
  step's "no version/framework names" guidance for concreteness. Keep it named.

## Requirements step (2026-09-21)
- User approved all 27 requirements as drafted, and asked where the design doc gets
  referenced.
- **Decision on referencing the design**: three-way split, stated to the user and agreed:
  1. `spektacular design ref add` — **done at the requirements step**, recording
     `design/querier-api-v2.md` against spec `20260921093100-query-api-v2`. This is the
     mechanically meaningful one; the plan workflow consumes it as an input.
  2. Prose mention in **Technical Approach** — points at the worked API surface, error
     sentinels and two-spelling split rather than restating them.
  3. **Not** referenced inside requirement text. Rationale given to the user: each
     requirement must be independently verifiable, "matches the design doc" is not
     testable, and if the two ever disagree the requirements are what acceptance is
     judged against. Keep this separation in later sections.
- Requirements are grouped under six `###` sub-headings (asking for what you want /
  published values / honest answers / data model / older toolchains / migration). The
  assembled spec will carry those sub-headings.
- Vocabulary substitutions used to keep requirements implementation-free — carry them
  into acceptance criteria for consistency: "kind" = stanza type axis, "variety" = subtype
  axis, "published values" = outputs, "declared item" = entity, "superseded query helper"
  = Querier[T]/NewQuerier.

## Acceptance criteria step (2026-09-21)
- User approved all 31 criteria as drafted, including the one requiring documentation-site
  code samples to compile under the pinned minimum Go version (they were offered the
  chance to drop it as unverifiable and kept it — so a mechanism to check MDX snippets is
  in scope for the plan to solve, not an oversight).

## CORRECTION to the design-reference decision (2026-09-21)
User: "a design doc, it should be a constraint not a technical requirement. We have
already covered the approach."

**Supersedes point 2 of the requirements-step decision above.** The prose reference to
`design/querier-api-v2.md` belongs in **Constraints**, not Technical Approach, because the
design is *settled and binding* rather than non-binding direction. This matches the
requirements step's own rule: a hard "must honour X" is a constraint; only non-binding
direction is Technical Approach.

Still true and unchanged:
- The `design ref add` metadata is recorded (done at the requirements step).
- The design is **not** referenced inside requirement or acceptance-criterion text.

Consequence for the technical_approach step: do **not** restate the design reference
there. The approach is already covered by the design document itself; that section should
carry only whatever direction is genuinely not in the design.

## Constraints step (2026-09-21)
Seven constraints saved. Decisions taken with the user:
- **Design document constraint is the first bullet**, per the user's correction. Its
  precedence clause was drafted and accepted: the design governs *how*, the spec's
  requirements and acceptance criteria govern *what must be true*.
- **Error convention kept as a constraint** (user confirmed binding, not direction) —
  sentinels matched by identity + detail-carrying wrapper types, matching the existing
  codebase convention; a second convention alongside it is not acceptable.
- **Docs-site consistency bullet dropped** (user's call) — the migration requirement and
  its acceptance criterion already mandate it; restating it as a constraint was
  duplication. Do not reintroduce it.
- User confirmed no further binding constraints. Saved-state compatibility was explicitly
  offered as a possible constraint and declined, so **regeneration of v1-written state is
  acceptable** and belongs in Non-Goals, not Constraints.

## Technical approach step (2026-09-21)
- Kept deliberately thin, per the user's correction that the design document is a
  constraint and "we have already covered the approach". **No design-capture offer was
  made** at this step because the design already exists and is already referenced.
- Two bullets, both accepted: (1) nothing further decided, detail left to the plan;
  (2) a flagged uncertainty — no existing mechanism compile-checks documentation-site code
  samples, which the accepted acceptance criteria nevertheless require. The plan must
  propose one. This is the one known gap between what the criteria demand and what the
  project can do today.

## Success metrics step (2026-09-21)
- Six metrics accepted as drafted, derived from the four success measures the user selected
  in the interview plus two migration-completion measures (zero references remaining, CI
  job continuously green).
- User was offered the chance to cut metrics that restate acceptance criteria and declined
  — the overlap is accepted deliberately, so do not prune it at verification.

## Non-goals step (2026-09-21) — SCOPE WIDENED

**The user brought the bare stanza form into scope.** This is the one place where the spec
now deliberately goes beyond the design document.

Background: the user asked what "bare stanza stays unimplemented" meant. Explaining it
surfaced a genuine inconsistency — the already-accepted acceptance criterion "The two
stanza forms address differently" cannot be tested if the parser rejects the bare form,
because no configuration can declare one. Offered three resolutions (soften the criterion /
widen scope to include the parser / drop the criterion); user chose **widen scope**.

Changes applied as a result:
- `requirements.md` — "The two stanza forms stay distinct" trimmed of its "even where only
  one is accepted today" clause; **new requirement added**, "Both stanza forms can be
  declared" (authors can declare a top-level item with the variety as leading keyword and a
  single label). Requirements now total 28.
- `acceptance_criteria.md` — **new criterion added**, "The bare stanza form is accepted";
  "The two stanza forms address differently" reworded to describe a real parsed
  configuration rather than two hypothetical declarations. Criteria now total 33.
- `non_goals.md` — the bare-form exclusion is **removed**; the language-boundary non-goal
  reworded to "beyond accepting the additional stanza form".

**Outstanding consequence: `design/querier-api-v2.md` is now wrong.** Its "Not in scope"
section still lists "implementing the bare `container \"nics\"` stanza form", and its
"Files to change" table has no parser entry for accepting it. The design must be revised
via `spek-design`, or the two documents will disagree — and the constraints section says the
design governs *how*, so the disagreement matters. Offer this to the user.

Other three scope cuts confirmed out by the user: no state migration (regeneration fine),
no new docs-site pages, stale library docs not corrected here. Remaining five non-goals
confirmed in the same exchange.

### Design-document revision: DECLINED (2026-09-21) — do not re-offer
User: "The design is related to the api change, changing the parser is just a requirement
to facilitate this."

`design/querier-api-v2.md` stays exactly as written. Its "Not in scope" line about the bare
stanza form describes **the design's own scope** (the API change), not the spec's. The
parser change is a facilitating requirement carried solely by this spec.

This supersedes the "design is now wrong / offer a revision" note above — that offer was
made and declined, and per the design-trigger rules a decline is final for this detail.
Do not raise it again.

To stop a planner reading the design's exclusion as overriding the spec, the requirement
"Both stanza forms can be declared" now carries a closing clause stating it is a
facilitating change rather than part of the referenced API design. That clause is the
agreed resolution of the contradiction — leave it in at verification.

## Verification step (2026-09-21)
- Spec assembled from the seven section working files and staged to
  `.spektacular/tmp/spec_template.md` (423 lines; 28 requirements, 34 acceptance criteria).
- Fresh-eyes reviewer subagent launched against the staged file only, with explicit
  instructions not to read the working files, working context, design docs, the spec store,
  source code or git history, and not to write anything.
- Triage rule for its findings: drop anything the user already deliberately decided —
  specifically (a) naming Go 1.27 in the Overview, (b) the deliberate overlap between
  success metrics and acceptance criteria, (c) the design-document constraint's placement,
  (d) the facilitating-change clause on the bare-form requirement. Do not re-litigate these.

## Verification outcome (2026-09-21) — SPEC COMMITTED
Reviewer returned 12 findings. Triage:
- **Dropped 2** as decisions the user had already made deliberately: naming Go 1.27 in the
  Overview (flagged as a leak — user chose it over version-neutral phrasing), and adding a
  "docs site is a separate repository" constraint (user dropped that bullet at the
  constraints step as duplication).
- **Applied 10**, all user-confirmed in one exchange. Most consequential: the reviewer
  caught a **genuine contradiction between two accepted acceptance criteria** — "a kind
  lookup naming the first variety returns three" versus "a variety name alone fails". The
  first now names kind *and* variety. Worth remembering that the fresh-eyes pass earned
  its keep on a criterion both the drafter and the user had already signed off.
- **Applied 1 more the reviewer missed**: a second "sentinel" mechanism reference in the
  not-found criterion, the same class as an approved fix. "sentinel" now appears only in
  Constraints, which is where the mechanism belongs.
- Spec written via `spektacular spec file write` and verified by read-back; frontmatter
  carries `designs: [{source: design, path: querier-api-v2.md}]` as intended, plus
  created_date 2026-09-21 and document_status draft. Working directory removed.

---

# PLAN workflow — 20260921093100-query-api-v2 (started 2026-09-21)

Spec above is now `final`/closed. The plan workflow is now driving; working files for
each section live in `.spektacular/work/20260921093100-query-api-v2/`.

## Carried forward into planning
- Scope is **the whole design in one change** — not phased, not a subset.
- **Unreleased v2**: no deprecation window, no compat shim, no external migration guide.
  In-repo callers, the library README, and the docs site are the only consumers.
- Docs site (`xcl-website`) **is in scope**: migrate every page naming the superseded
  helper, including the landing page headline example + feature blurb. No *new* site
  pages for outputs/error semantics.
- Binding design document: source `design`, path `querier-api-v2.md`. Architecture is
  **built on** it, not re-derived.

## Repo roots (verified present on disk, 2026-09-21)
- `xclconfig` → `/home/nicj/code/github.com/jumppad-labs/xcl` (Go library)
- `xcl-website` → `/home/nicj/code/github.com/jumppad-labs/xcl-website` (Astro 5 + MDX + Tailwind v4)

## Known-open question the spec itself flags for the plan
- There is **no existing mechanism** for verifying that docs-site code samples compile.
  Acceptance criteria demand it, so the plan must propose one (extract samples to
  buildable files, or equivalent). This is the single largest unforced design decision
  left to the plan.

## Plan step log
- `overview` — spec read in full. Advancing to `discovery`.

## Plan step log (cont.)
- `discovery` — complete. Four parallel research agents across both repos; findings distilled
  to `.spektacular/work/20260921093100-query-api-v2/research.md` (the decision log) and
  judgement calls to `assumptions.md` alongside it.

### User decisions taken during discovery (2026-09-21)
Asked as three blocking questions; all three answered. These change what the plan delivers:

1. **Compile-proofing of docs-site code samples is DROPPED.** Site pages are still migrated off
   `NewQuerier` to the new surface, but samples are not machine-verified. The site's existing
   manual policy (`xcl-website:README.md:39-40`) stands. This partly descopes one acceptance
   criterion — the "search returns zero matches" and "landing page shows the new surface"
   clauses survive; the "every sample compiles under the pinned minimum Go version" clause does
   not. Must be stated plainly in the plan so spec and plan do not silently disagree.
   - Context for the decision: the site has no Go module, no `.go` file and no Go CI step, and
     all 16 Go samples are non-compilable fragments, so this was a from-scratch build covering
     all 16 blocks, not just the 4 that change.
2. **`example/configonly` IS fixed in this plan.** Already red on HEAD (no `c.Destroy()` call,
   no `## Destroyed` output, three tests asserting both). In hand because the plan rewrites that
   file anyway.
3. **`plugins/example`'s pre-built-binary failure is LEFT ALONE**, tracked separately. Means a
   full-suite run is not a clean signal; the floor CI job should be `go build`-scoped and no
   "suite is green" claim can be made without naming this.
4. **`architecture/ux-flow.md` IS updated as a plan deliverable**, via `spek-knowledge`
   propose-then-confirm. It currently documents `Querier` as current architecture.

### Cross-cutting learnings worth carrying
- **`Meta.Type` is the highest-risk change in this plan.** One key drives HCL expression
  namespacing, state-file identity, provider dispatch, the resource ID, and every event string
  (~30 production read sites). Sharpest edge: `state/file_state_store.go:69-73` *silently skips*
  a resource whose `meta.type` it cannot resolve rather than erroring.
- **The error convention is being established, not copied.** `plugins.ErrNotFound` is the only
  error in the repo implementing it end to end; nothing in first-party code implements
  `Is`/`As`/`Unwrap`, and receivers are inconsistent (value in `state/`, pointer in `registry/`).
  `types.ErrTypeNotRegistered` is the cautionary case — never matched, formatted with `%s`.
- **`IsRegisteredType` is narrower than its name suggests** — `RegisterType` Go types only, not
  builtins and not plugin types. The parser's keyword `default` arm has no single "do you know
  this name?" query to consult for the bare stanza form.
- **Design vs spec conflict, resolved in the spec's favour**: the design lists the bare
  `container "nics"` stanza form under "Not in scope"; the spec requires it and calls it a
  facilitating change. Spec governs on *what must be true*. Without it, the acceptance criterion
  "The two stanza forms address differently" is unprovable.
- Verified locally: installed toolchain is **go1.27.0**, and `GOTOOLCHAIN=go1.25.0` resolves from
  cache with no network fetch. A plain `go build` does **not** exercise the floor.
- Existing CI installs **Go 1.22**, below the `go.mod` floor of 1.25.0 — needs correcting as part
  of adding the floor job.

- `architecture` — complete. Direction chosen and recorded in `assumptions.md`; drafted to
  `architecture.md` and `conventions.md` in the work dir.

### Architecture direction (locked)
**Split the data model first, then build the surface.** Four movements: (1) kind/variety split
across `Meta`/`FQRN`/parser + ~30 read sites, old querier minimally adapted to stay green;
(2) new surface + error taxonomy, `querier.go` deleted; (3) consumer migration — examples,
tests, README, `docs/state.md`, site; (4) floor CI job + knowledge entry update.
Rejected: surface-first (kind axis has no correct implementation against conflated data) and
big-bang (no intermediate green state, regression not bisectable).

Non-obvious decisions taken (full rationale in `assumptions.md`):
- HCL expression namespacing (`internal/parser/context.go:126,136`) moves to `Meta.Subtype` for
  resource-kind entities — its **own** acceptance-tested step, because missing it silently breaks
  every cross-resource reference and no public-API test would catch it.
- The silent skip in `state/file_state_store.go:69-73` becomes a reported error. No state
  migration is written (non-goal), but stale state must not vanish silently.
- `RegisterType` keeps its current kind-led meaning; the bare stanza form gets a **sibling entry
  point**, so `TypePath` can tell the two forms apart and "registered under one form only" holds.
- `checkTypeName`'s existing three-source scan is inverted into one registry query for the parser.
  Do **not** widen `IsRegisteredType` — `internal/parser/lifecycle.go:356-365` depends on its
  current narrow meaning.
- New error detail structs use **pointer** receivers (matching `TypeNameClashError`, the one
  production-matched typed error). `state.ResourceNotFoundError` keeps its value receiver and
  merely gains `Is`.
- `components` — complete, drafted to `components.md`. 21 components: the split lookup
  implementation + its two spellings, error taxonomy, conversion, and the changed existing
  components (metadata, addressing, expression namespacing, block parsing, registry, state
  lookup/persistence, published-value projection, examples, docs, site, CI, knowledge entry).
- `data_structures` — complete, drafted to `data_structures.md`. Names provisionally fixed for the
  two registry additions the design is silent on: `KnownType(name) bool` and
  `RegisterBareType(name, resource)`. `TypePath` follows the design's given signature.
  Flagged in the draft: `Meta` is a **serialization boundary** (its JSON tags are the on-disk
  state format), so the kind/variety split is a state-format change, not just in-memory.
- `implementation_detail` — complete, drafted to `implementation_detail.md`.
  Key risk articulated there for the implementer: changing the conflated metadata field's meaning
  produces **zero compiler errors** (both axes are strings), so all ~30 readers must be walked
  deliberately rather than chased via the build.
- `dependencies` — complete, drafted to `dependencies.md`. Design document named per instruction
  (`querier-api-v2.md` from the `design` source), with the one design/spec divergence stated on
  its own bullet. No new third-party dependency; predecessor plan already landed, nothing must
  land first.
- `testing_approach` — complete, drafted to `testing_approach.md`. All six spec success metrics
  carried through and classified; three are **split** behavioural/manual (call-site migration,
  published-value retrieval, floor CI job) — the manual halves use the exact phrase
  "manual — captured in the implementation test plan" as required.
  Deliberate gaps recorded: docs-site samples not compile-checked (descoped clause named
  explicitly), plugin example's pre-existing red test stays, no perf testing, no new mocks.
- `milestones` — complete, drafted to `milestones.md`. Four milestones:
  M1 two-axis split + addressing + bare stanza form + stale-state honesty (declared as
  groundwork, justified per instruction); M2 the whole lookup surface + errors + published
  values, old helper removed; M3 bundled examples/tests on the portable form + configonly
  repair + floor CI job; M4 README + docs/ + site (4 pages) + knowledge entry.
- `phases` — complete, drafted to `phases_plan.md` and `phases_context.md`. 15 phases:
  M1 1.1-1.5, M2 2.1-2.5, M3 3.1-3.3, M4 4.1-4.3. All but 4.2 are repo `xclconfig`;
  4.2 is `xcl-website`.
  Sequencing decisions worth keeping: reference resolution (1.2) is its own phase because its
  failure is silent and total; the example repair (3.1) precedes the migration (3.2) so the
  migration lands on green; the floor CI job (3.3) is **build-only**, because the full suite
  cannot pass on a fresh checkout while the plugin-example binary issue is left open.
  Phase 4.1 explicitly excludes the README's 232-line stale tail (a spec non-goal).
- `open_questions` — complete, drafted to `open_questions.md`. Three kept, each with an explicit
  "fix it yourself" vs "STOP and ask" boundary.
  **Two apparent open questions were resolved during this step instead of parked:**
  1. Import cycle — `config.go:10` imports `xcl/state`, so `state` CANNOT import `xcl`. The
     not-found sentinel is therefore declared in `state` and re-exported from `xcl`, following
     the existing precedent at `config.go:16`. Phase 2.1 context updated.
  2. Enumeration coverage — verified that `internal/parser/parser.go:463-467` appends every
     parsed resource to state with no kind filter, so variables/outputs/modules are genuinely
     present and the "enumeration covers every kind" criterion is satisfiable. Phase 2.4
     context updated.
- `out_of_scope` — complete, drafted to `out_of_scope.md`. Grouped by excluding authority: user
  decision (sample compile-proofing, plugin-example test), spec non-goals (8 items), chosen design
  (4 items, incl. not unifying the duplicate provider lookups and not touching the `local`
  address oddity). Notes the one apparent contradiction explicitly: no compat shim for external
  consumers, but the renamed enumeration/count operations DO keep deprecated aliases in-repo.
- `assemble` — complete. Three documents staged to `.spektacular/tmp/`:
  `plan_template.md` (993 lines, 4 milestones, **16** phases — note: 5/5/3/3, not 15),
  `context_template.md` (507 lines, 16 per-phase sections), `research_template.md` (526 lines).
  Metadata: 2026-09-21T12:32:52Z, commit 13e0311, branch main, git@github.com:jumppad-labs/xcl.git.
  Assembly gotchas hit and fixed: milestones and phases must be **interleaved** (milestone N then
  its phases), not concatenated; and the feature-slug skill's `NNNN-` scheme does not apply to this
  project, which names plans by timestamp — plan name used as the namespace, no question asked.
- `verification` — passed. Checks run: all 11 plan.md sections, all 7 context.md sections, all 8
  research.md sections present, in order, and filled; 16/16 phases each carry `**Repo:**`,
  a `*Technical detail:*` link and outcome-based acceptance criteria; all 16 anchors resolve
  against context.md headings with slugs matching exactly; phase titles identical between the two
  documents; no shell commands in plan.md phase content; no placeholders; no thin sections.
  **One real omission caught and fixed:** `## Project References` was missing from context.md —
  the assemble scaffold does not show it but the verification list requires it. Written to a new
  working file `project_references.md` (so it is durable, not just staged) and inserted between
  Testing Strategy and Token Management Strategy.
  Staged sizes: plan 993 lines, context 562, research 531.
- `write_plan` / `write_context` / `write_research` — all three documents committed to the plan
  store via `spektacular plan file write`. Scratch files and the per-section working directory
  `.spektacular/work/20260921093100-query-api-v2/` removed. Next: `walkthrough` — the plan is NOT
  approved until the user signs off there and the `finished` step runs.
- `walkthrough` — complete. User raised one change request and approved the plan.

### Walkthrough outcome (2026-09-21)
**Change applied and committed to all three documents:** the seven query sentinels now live in the
project's **existing `errors` package** (`github.com/jumppad-labs/xcl/errors`), not split between
`state` and `xcl` as first drafted. User's steer; my original rejection rationale ("an unnecessary
third package for one value") was simply wrong — the package already existed and I had not checked.
Verified cycle-free: `errors` imports only `internal/xcl` and `go-wordwrap`, so both `state -> errors`
and `xcl -> errors` are safe. Added on top: re-export the sentinels from `xcl` (precedent
`config.go:16`) so consumers are not forced to alias-import a package named `errors` to keep using
stdlib `errors.Is`. The drafting-assumption entry in research.md was rewritten to record the
correction rather than the original reasoning.

**Knowledge entry written** (user confirmed): `repo`/`xclconfig`,
`conventions/shared-errors-package.md`, tags `[errors, sentinel, package-layout, import-cycle]`.
Captures that `xcl/errors` is the home for shared error types, that its thin import list is what
makes it usable as neutral ground, the re-export precedent, and the two existing mistakes not to
copy (`%s` instead of `%w` at `types/register.go:38`; inconsistent receivers).

**Plan approved by the user.** No other assumptions challenged.
