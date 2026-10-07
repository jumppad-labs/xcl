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
