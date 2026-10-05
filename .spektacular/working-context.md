# Working context: 20261003134528-327e0657-references-and-secrets

## How this workflow started

- Resumes the earlier `20260923095659-references-and-secrets` spec, whose workflow state was
  dropped when the config-decode spec was started with `spec new --force`. The old store doc was
  an empty template; user chose (2026-10-03) to start a new workflow, reuse the saved interview,
  and delete the old draft (done via `spec file delete`).
- The earlier interview synthesis was moved to
  `.spektacular/work/20261003134528-327e0657-references-and-secrets/interview.md` — it is the
  source of truth for the interview; do not re-ask what it settles.
- Source recorded: https://github.com/jumppad-labs/xcl/issues/1 (secrets part).
- No epics exist in the project.

## Code facts re-checked 2026-10-03 (after Decode landed)

- `EncodeEntity` (encode.go:59) and `EncodeSavedEntity` (encode.go:82) still exist.
- DAG: `buildCreateDAG` (internal/parser/dag.go:63) loops `Meta.Links` through
  `types.AppendUniqueDependency`, which appends to Links AND mirrors into `DependsOn`
  ("for backwards compatibility", types/resource_helpers.go:91). `getResourceDependencies`
  (internal/parser/util.go:507) then reads `DependsOn` back. Interview's description holds.
- Additional `DependsOn` reader not in the interview: `logger/pretty_printer.go:302,588` uses
  `types.GetDependencies` — affected by part 2.
- Tag options live in internal/xcl/tags/tags.go (`OptionComputed = "computed"`); no
  `sensitive` option exists yet.

## Interview decisions (2026-10-03) — detail lives in work/<spec>/interview.md

- Scope widened by user to all of issue #1 (state, events, logs, sensitive), reversing the first
  interview's exclusions. References + depends_on kept in this spec; split decided at split step.
- Names chosen by user: `types.Sensitive[T]`; interface `Masker` (Mask/Unmask/Name); options
  `WithStateMask`, `WithEventMask`, `WithNoEventMask`; built-ins `mask.EncryptAES256GCM`,
  `mask.HashHMACSHA256`, `mask.Omit`, `mask.Redact`. User asked for AES406/MD5; corrected to
  AES-256-GCM / HMAC-SHA256 and accepted.
- Defaults: events Redact; state plaintext + warning without a masker; errors always Redact.
- Go API returns real values (wrapped in Sensitive). Propagation via cty marks; sensitive
  outputs stay sensitive. Diagnostics: no change (user: they only print what's in the source).
- User style this session: thinks out loud and redirects mid-question; when a question dialog is
  rejected, ask what to clarify and show concrete code/types (they asked to see the Output type).
- Outputs (Part 4, user-confirmed): `output` stays builtin; only outputs referenceable from
  outside a module (validation); outputs are public entities returned by Find/All/Decode with the
  value as a field; Outputs() stays; sensitive values never unwrap into plain Go types.
- User: "keep adding this to this spec, I think we make it part of an epic in a while" — expect
  an epic split at the split step (references / depends_on / sensitive+masking+state / outputs).
- Module boundary applies at every nesting level (user: "a parent should only be able to reach a childs outputs"). Inbound already variables-only (context.go AppendParentModule) — no non-goal needed.
- Whole-file state encryption: non-goal, "not right now". Design doc for sensitive API offered at technical approach; user's reply ("ok") ambiguous, not written.
- Split done (2026-10-03): epic 20261003134528-327e0657-references-and-secrets with specs
  references-as-written, user-depends-on, module-boundary-and-output-entities,
  the original spec (now sensitive values; state keeps real values), masking (depends on it).
  User plans to ship all at once.

## Plan-epic run (2026-10-04) — orchestrator notes

- Epic: 20261003134528-327e0657-references-and-secrets. Project root: /home/nicj/code/github.com/jumppad-labs/xcl.
- Order: references-as-written, user-depends-on, module-boundary-and-output-entities (ready) →
  references-and-secrets (sensitive values) → masking.
- An earlier run's notes claimed all 5 DONE, but on re-run status showed no plans in the store,
  no epic summary and no lanes. Only unsaved working files for module-boundary remained in
  work/. Restarted the loop from the CLI's status.
- Run 2 started children: references-as-written, user-depends-on, module-boundary-and-output-entities.
- DONE: user-depends-on (verified in store). DONE: module-boundary-and-output-entities (verified). Summary sections written. Started child: references-and-secrets. DONE: references-as-written (verified), summary written. DONE: references-and-secrets (verified), summary written. Started child: masking. DONE: masking (verified), summary written. All 5 planned. epic order added 5 deps (chain: module-boundary → refs-and-secrets → refs-as-written → user-depends-on → masking). 3 decisions written (changelog test anchor, site page review, event-data shape). User accepted all 3 decisions (applied to plans and summary), kept the order, and approved 8 knowledge entries (written to xclconfig). Review done.
- QUESTION (module-boundary, at discovery): spec "outputs found as entities" contradicts knowledge entry architecture/ux-flow.md (Find[string](c,"output.x") returns value). Answer (user): A, wording approved now — plan a task to rewrite ux-flow.md via spek-knowledge with Find[types.Output] + .Value example and Outputs() note. Relayed; child resumed at architecture.

## Plan-epic run 3 (2026-10-05)

- Store again showed no plans / no summary / no lanes despite run 2 notes saying all DONE
  (plans/ epics/ config.yaml touched 2026-10-05 10:32; cause unknown). User: "ignore all that
  and just run". Re-planning under the existing epic-order chain (serial):
  module-boundary → references-and-secrets → references-as-written → user-depends-on → masking.
- The 3 decisions from run 2 (changelog test anchor, site page review, event-data shape) and the
  module-boundary ux-flow.md answer (A: rewrite ux-flow.md via spek-knowledge with
  Find[types.Output] + .Value, Outputs() note) were already settled by the user — reuse, don't re-ask.
- DONE: module-boundary (verified in store; summary kept for step 6). DONE: references-and-secrets (verified). DONE: references-as-written (verified). DONE: user-depends-on (verified). Started child: masking.
- Run 3: all 5 DONE (verified). Decisions settled by user: (1) ShowReferences bare ref shown for sensitive field (A); (2) envelope always incl. Redact; (3) CHANGELOG Breaking only vs last release (masking dropped 2 items). Applied to plans; summary + decisions written. epic order added nothing (chain already present). Next: end-of-planning review.
- Review closed: user approved summary; declined all knowledge saves this run.
- Review change: user chose destroy graph from Meta.Links (create builder, reverse walk), Meta.Parents removed. user-depends-on plan revised (new tasks b3e94632, 5078beb1); summary section + decisions updated.
