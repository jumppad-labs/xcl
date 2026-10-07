# Working context — spec 20261007105731-2388b579-diff

## Problem / motivation
User wants to "do a diff between state and config". The diff "should show the resources that have changed and what has changed in them", and they want to "show this graphically, kind of like a git diff". Later: "It should work like terraform plan but I think it has to run the provider to call refresh and things. I kind of just want to return the changed types I really like the terraform plan output".

## Decisions so far
- Behaves like a plan: parse config, load state, walk DAG in dependency order, call provider `Read` (refresh) and `Changed` for existing resources. Never calls Create/Update/Destroy, never saves state.
- Result is the changed resources with an action (create / update / destroy / rebuild for failed) and, for updates, per-field old -> new values.
- Rendering in a Terraform-plan-like style (+ / ~ / - markers, summary line), coloured via existing `highlight` renderer; sensitive values masked unless `RevealSensitive()`.
- Values depending on a resource that would be created are unknown at diff time: show as "(known after apply)" and do not call the provider for the dependent (user: "yes").
- Name it "diff", not "plan" — user: "I don't directly want to copy terraform".
- Epic: user said "Anything is fine" — created standalone (no epic).
- Uncommitted changes in xcl-website and xcl-vscode were committed first at user's request.

- Existing design `design/config-diff.md` (2026-09-21) found at constraints step. User liked it and asked to add an example of rendered output. Revised via spek-design (design author, --spec) to add: provider Read/Changed refresh, events, sensitive masking (`sensitive: true`), `Render` + `Highlight` renderer with worked example, signature `Diff(paths []string, options ...diff.Option)`. Referenced from the spec (design ref add) and recorded as a constraint.
- Spec terminology aligned to design action names: create / update / replace / delete (was rebuild / destroy).
- User said "just keep churning to the end" after acceptance criteria: constraints, technical approach, success metrics, non-goals drafted without per-section confirmation; to be reviewed together at verification.
- Docs in xcl-website are in scope; example programs are out of scope.

## Codebase learnings
- Change detection today: provider `Changed(old, new) (bool, error)` only (plugins/changed.go, plugins/adapter.go:32) — no field-level detail.
- Apply update path: internal/parser/lifecycle.go — carryComputedValues, Read (ErrNotFound -> create), Changed, Update.
- Encoding/highlighting: encode.go `EncodeEntity`, `Highlight(renderer)`, `RevealSensitive()`; highlight package.
- Library only today (Config.Validate/Apply/Destroy/Load in config.go); no CLI in repo.
- Other repos: xcl-website (docs), xcl-vscode (syntax highlighting).

# Orchestrator — spek-plan-epic 20261007105731-2388b579-diff (2026-10-07)
- Epic chosen: 20261007105731-2388b579-diff (only epic with unplanned specs).
- Specs: 20261007105731-2388b579-diff (ready), 20261007111826-cf3b66d8-diff-rendering-and-docs (blocked on the first).
- Started child for 20261007105731-2388b579-diff.
- Q (diff child): finished step auto-commits (auto_commit: workflow). User answered: finish and commit. Sent to child.
- diff child FAILED at walkthrough -> finished: auto_commit_failed (git pathspec on untracked empty dir .spektacular/tmp/20261007105731-2388b579-diff; .spektacular/tmp/ not git-ignored). Child's attempt to edit .git/info/exclude was denied by permissions. Run stopped; epic summary not yet written; rendering-and-docs not started. Plan docs are staged (A) in git.
- User switched auto_commit off (config.yaml set to 'off'). Resuming diff child to finish without commit.
- diff plan DONE (no commit). Starting child for 20261007111826-cf3b66d8-diff-rendering-and-docs.
- Both plans DONE; epic summary written (2 spec sections, no decisions); epic order added none. Next: end-of-planning review.

# Orchestrator — spek-implement-epic 20261007105731-2388b579-diff (2026-10-07)
- Epic: 20261007105731-2388b579-diff. Order: diff -> diff-rendering-and-docs.
- Dirty check: user chose "commit first"; tree was already clean (user committed cc065ec on branch f-diff).
- Created worktree for 20261007105731-2388b579-diff at .spektacular/worktrees/20261007105731-2388b579-diff/xclconfig; started child.
- diff child QUESTION at Task 8 verify: redis port edit -> apply also updates app.web (computed connection_string recomputed on Update), diff reports only redis. Conflicts design (unknown only for create/replace) vs success metric (diff == apply for all e2e). Options: 1 keep design + document limitation in test/metric (default), 2 treat updated resources' computed fields unknown, 3 drop redis scenario. Tasks 1-7 done. Put to user.
- User decision (diff Task 8 question): conservative default — every computed field of a resource reported as update is unknown; dependents show update with (known after apply). Contract: diff never under-reports; apply may skip dependents (lifecycle re-reads + Changed). Override = optional provider method ComputedChanges(ctx, old, new T) ([]string, error) + gRPC RPC; example: docker network DNS add keeps docker_id. Override goes in a NEW separate spec in this epic (with example + docs); user accepted.
- config-diff.md revised via spek-design (user approved). Written into the diff spec's WORKTREE (so it merges with the branch); main checkout's copy restored to HEAD content.
- Resumed diff child with the decision.
- User: create the ComputedChanges override spec AFTER this run (then add to this epic, plan, implement). Reminder for final report.
- Merge refused (uncommitted worktree). User authorised committing each finished spec's worktree on its spek/ branch for THIS run (both specs).
- diff spec DONE. Merge needed: (a) commit (user-authorised), (b) store files must NOT be on the branch (epic_merge_touches_spektacular) — copied spec/plan/context/test-plan/changelogs/design into the project via CLI (verified byte-identical), dropped them from the branch, committed code only (1e247a9). Merged into f-diff (316bb5f); worktree removed. Project store records left uncommitted in main checkout.
- Started rendering-and-docs (worktree created). Same merge procedure applies when it finishes.
- rendering child QUESTION: implement new refused dependencies_unmet (diff spec's records uncommitted in project, worktree store stale). User chose: override_dependencies. repo list fails in worktree (no xcl-vscode worktree) — child uses given roots.
- rendering-and-docs DONE; records copied to project via CLI (verified identical), code committed (xclconfig 9a4a22f, xcl-website 30c69e9), merged. Epic implement complete. Next (user): create ComputedChanges override spec in this epic.
