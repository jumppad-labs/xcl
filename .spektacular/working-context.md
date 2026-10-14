# Working context: python-plain-dataclasses (resumed 2026-10-14)

New spec id `20261014112837-57d1afde-python-plain-dataclasses` (spec.id_method is timestamp, so the
parked `20261014043756-e853a081-...` could not be reused; its store entry is an empty draft). Drafts
copied from the parked work folder; full park-time context in
`.spektacular/work/20261014043756-e853a081-python-plain-dataclasses/working-context-at-park.md`.

Target repo: xcl-python (`/home/nicj/code/github.com/jumppad-labs/xcl-python`); also xcl (parity
fixtures) and xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).

## Confirmed before park

- overview, requirements, acceptance_criteria, constraints confirmed. technical_approach was a draft,
  never confirmed. Remaining: technical_approach, success_metrics, non_goals, verification.
- Interview: remove `xcl.Resource` and `@xcl.entity` outright (breaking); website docs in scope;
  extend xcl parity fixtures (Python plain dataclasses vs Go plain structs).

## Changed since park: CI/release automation landed

- Python now has its own version line (0.8.1 on PyPI, engine pin v0.8.0 in `xcl-ref.txt`), released
  by merging a labelled PR; xcl releases open auto-merging "Use xcl engine <tag>" PRs.
- So constraint 3 ("ships with the next xcl release ... through the existing linked release") and
  the technical-approach ship order are stale and must be re-confirmed with the user.
- This spec is meant to be the first real use of the process; the engine-bump PR path is still
  unobserved (CI plan tasks: auto-tag concurrency criterion, engine update PRs).
- CI implement workflow `20261014052450-0b131707-ci-release-automation` still at step implement,
  18/20 tasks; its notes are in its plan changelog.

## Re-confirmed after resume (2026-10-14)

- Constraint 3 rewritten for the independent release process (own version line, `minor` as a
  pre-1.0 breaking change, only the fixture-carrying xcl release needed). User: "yes this looks good".
- Technical approach confirmed, including the new ship order (xcl fixtures `patch` -> v0.8.1 ->
  auto bump -> Python 0.8.2; then breaking PR `minor` -> 0.9.0; then website) and release notes via
  PR title + README migration section link.

## Corrections after the parity check (2026-10-14)

- No xcl change: the parity harness's Go side registers `types.json` via `registry.TypeFromSchema`
  (`bindings/libxcl/cmd/parity/main.go:112`), never Go structs; Python's parity classes live in
  xcl-python `tests/parity/types.py`. The interview's "extend xcl parity fixtures" was based on a
  wrong picture. "Same schema as Go" requirement/AC reworded accordingly; constraint 3 drops the xcl
  release clause. Whole change = one xcl-python `minor` PR.
- Ship order removed from the technical approach: user wants sequencing left to planning, backed by
  a knowledge entry on how cross-repo changes ship (being written via spek-knowledge).
- Non-goal to propose: a Go test that plain structs produce `types.json` schemas (outside this spec).
- Success metrics confirmed (three; release-process metric dropped).

## Knowledge (2026-10-14)

- Written: xcl `architecture/cross-repo-release-flow.md` (labels, bindings check, pin, bump PRs,
  breaking-change flow, fixing languages in any order, manual PR stacking recipe, sharp edges).
  `learnings/releases-tagged-by-hand.md` rewritten as superseded, pointing at it.
- Offered (no answer yet): a spec for per-language override labels, companion binding PR in the
  bindings check, a pin-must-be-a-tag check, and engine-bump not clobbering human pushes.
- `spek-knowledge` skill is not installed; `spektacular knowledge write --data ... --file <path>`.
- Non-goals draft presented, awaiting confirmation.
- Non-goals confirmed.
- Verification: 10 reviewer fixes applied (user confirmed); 10 dropped as template format or confirmed wording.
- Split check: not offered (one weak signal; single atomic change plus docs). Spec committed to store.
