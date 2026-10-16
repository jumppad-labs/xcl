# Implement: 20261016064106-8931613a-python-encode-entity

## Worktree roots (work only here; run spektacular from the project root)
- xcl: /home/nicj/code/github.com/jumppad-labs/xcl/.spektacular/worktrees/20261016064106-8931613a-python-encode-entity/xcl
- xcl-python: /home/nicj/code/github.com/jumppad-labs/xcl/.spektacular/worktrees/20261016064106-8931613a-python-encode-entity/xcl-python
- xcl-website: /home/nicj/code/github.com/jumppad-labs/xcl/.spektacular/worktrees/20261016064106-8931613a-python-encode-entity/xcl-website
- Project root (spektacular CLI): /home/nicj/code/github.com/jumppad-labs/xcl
- Branch in every worktree: spek/20261016064106-8931613a-python-encode-entity

## User answers
- Spec chosen: python-encode-entity.
- Mode: "run without asking" — loop through tasks without the continue prompt; still STOP on failures,
  mismatches, spec conflicts and the human release task.

## Helpers (scratchpad, recreate if missing)
- tick.py <in> <out> "<task title>" ticks heading + acceptance criteria; changelog.py <in> <entry> <out> appends entry.

## Decisions and learnings
- read_plan: structure valid, drift check clean, spec fully covered, changelog absent (first-task run).
- `lookupAllowed` is a method on `*configuration` (`conf.lookupAllowed("EncodeEntity")`), not on Service.
- xcl-python's `xcl-ref.txt` (currently v0.10.0) should point at the xcl feature branch
  `spek/20261016064106-8931613a-python-encode-entity` while in review; locally use `XCL_SRC` = the xcl worktree.
- Task "Add the encode messages": proto edited, `make core-protos buf-lint buf-breaking` pass. No tests per plan.
  KNOWN RED until task "Serve entity encoding...": `coreservice` TestEveryServiceMethodHasAHandler fails
  (EncodeEntity has no handler) — the plan registers it in that task.
- Workflow FSM: from update_changelog loop to analyze, then goto implement, test, verify, update_plan, update_changelog in order
  (skipping implement → refusal).
- Task "Render an entity record": `xcl:encode_record.go` written (EncodeEntityRecord, storedFor, fillRedacted/fillValue;
  skips fields of type types.ResourceBase and types.Meta; interface and map elements filled through a copy).
  Done; plan's open question resolved (decoded and stored types match, incl. plugin types).
- Task "Serve entity encoding": coreservice handler + helpers encodeAllowed/encodeOptions/colour in lookups.go;
  descriptor tests green again. Tests being written by sub-agent.
  Done (29 tests). EncodeSavedEntity on a lookup record + reveal keeps the marker; only EncodeEntity fills.
- Task "Describe...": CHANGELOG entry added; design updated via `design author` (write refused). Passing `--spec`
  stamped a sticky `spec:` field into libxcl.md frontmatter — flag to user at the end.
- Task "Regenerate protos": done; xcl-ref.txt = spek/<spec>. xcl-python worktree has .venv (uv) and
  `make engine plugins XCL_SRC=../xcl PYTHON=.venv/bin/python` built xcl/_bin/xcl-core + build/plugins.
- Task "Turn a Python entity into the record": Meta.to_record + _record_keys in _entity.py; record_of/record_bytes
  in _schema.py (value-driven mirror of _hydrate_value). Done (295 unit tests).
  LIMITATION: a class without a `meta` field loses meta.id/references on hydration, so reveal/show_references
  can't use the engine copy for it — document in README (task "Document entity encoding").
- Task "Add encode_entity": config.py encode_entity + encode_saved_entity keywords + module `_encode_options`;
  aio.py twins (imports record_bytes, _encode_options at top). Tests written: unit 312 pass; integration 2 FAIL.
  STOPPED at verify (workflow step = test→verify not yet advanced past test). Divergences from Go for looked-up entities:
  (1) hydrated [] / {} defaults sent back → engine writes `tags = []`; (2) lookup JSON writes marker for unset
  Sensitive → text shows marker where Go omits; (3) Python dataclass defaults (nested `interval=10`) written.
  Proposed: A) engine fills from stored copy whenever meta.id matches, not only with reveal; B) hydration keeps
  the source record per instance and record_of writes unedited fields from it. User APPROVED A and B ("ok")
  after a plain-language explanation. Both implemented; all suites green (Go root+coreservice, py unit 320,
  integration+parity). CHANGELOG + design doc corrected for always-fill. Task done.
- Docs task, Go parity, Python parity, website: all done. Parity passes for EVERY fixture (18 tests), byte-identical
  under 16 combos for looked_up/saved/hand_built. Extra engine fill: empty list/map left out of lookup JSON is
  taken from the stored copy (needed for configured `tags = []`).
- PAUSED at update_changelog: remaining tasks are "Release xcl" (HUMAN) and "Pin xcl-python" (needs the release tag).
  After the release: loop analyze → implement the pin task (xcl-ref.txt → tag, `make protos` from tag), then the
  wrap-up steps (test_plan, update_feature_changelog, reconcile_spec, finished, `implement merge`).
- Open items to raise with the user: sticky `spec:` stamp in libxcl.md frontmatter; spec AC "empty list omitted by
  default" holds only for an unset list (Go writes a configured `tags = []`); nothing committed (auto_commit off).
- FSM note: from `verify` you cannot goto `implement`; fixes were made while in verify, then re-verified.
- User prefers plain-language explanations with a concrete example over abstract descriptions.
- Before the workflow, the user had plain branches `python-encode-entity` created in xcl, xcl-python, xcl-website
  (from main); the worktrees branch from those HEADs.

## After the merge (2026-10-16)
- User approved committing; worktrees committed and merged with `spektacular implement merge` into the
  `python-encode-entity` branch of xcl, xcl-python and xcl-website (note: `--dry-run` was ignored and merged).
  Worktrees and spek/ branches are gone: further work happens in the main checkouts on `python-encode-entity`.
- xcl-ref.txt now names `python-encode-entity` (the branch to push for the xcl release PR).
- Still open: "Release xcl" (human: push xcl `python-encode-entity`, PR with `minor` label, merge), then
  "Pin xcl-python" to the tag, then wrap-up steps (test_plan, update_feature_changelog, reconcile_spec, finished).
