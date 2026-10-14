# Implement: ci-release-automation

- Worktrees: xcl `.spektacular/worktrees/20261014052450-0b131707-ci-release-automation/xcl`,
  xcl-python `.spektacular/worktrees/20261014052450-0b131707-ci-release-automation/xcl-python`
  (branch `spek/20261014052450-0b131707-ci-release-automation` in both).
- read_plan: structure ok (20 tasks), no drift, full spec coverage, changelog absent (first task).
- Git identity not configured: commit with `-c user.name="Nicholas Jackson" -c user.email="jackson.nic@gmail.com"`.
- Human tasks follow `docs/release-setup.md` in xcl.

## State at pause (2026-10-14, user rebooting)

Done (merged + recorded in plan): release decision command; shared label check (xcl #19) and
xcl-python caller (#6); labels + rulesets on both repos (xcl ruleset "Main", xcl-python "main");
reusable Python suite (xcl-python #7, required check `test / result`); bindings gate (xcl #20,
required `bindings`, label `breaks-bindings`); release bot App + secrets on both repos; engine pin
check (xcl-python #8); release dispatch sends bump size (xcl #21); auto-tag (xcl #22).

In flight:
- xcl #24 (branch spek/<name>-tag-permissions): tag.yml grants contents+pull-requests read. First
  Tag run (38531529299) was startup_failure because the default token is read-only. On last look
  #24 showed ~24 failing python jobs that failed in ~1s in run 38531673548 — NOT yet diagnosed
  (likely the cancelled first run from opening with a label; check the second Bindings run).
- Auto-tag task (5ea16ccb) NOT yet ticked in the plan: tick after #24 merges and its Tag run
  finishes with no tag.
- Knowledge saved: xcl gotchas/reusable-workflow-caller-permissions.md.
- Issue raised: xcl #23 (go-plugin data race flake in TestFailingDiscoveredPluginFailsTheLoad).

Next: user does step 4 (seed tag v0.7.0 on xcl-python, before "Python releases from its own tags");
then agent tasks: Python releases from its own tags (81b6a536), engine-update PRs (96df2316),
then human step 5 (allow auto-merge), then README docs tasks (b40284cd, 334cfd14).

Helper scripts used (scratchpad, may be gone after reboot): tick.py / tickhead.py / tickcrit.py /
changelog.py — read plan via CLI, edit, write via `spektacular plan file write ... --from`.
Worktree branches: xcl uses separate branches per PR off origin/main (spek/<name>-auto-tag,
-tag-permissions); xcl-python on spek/<name>.
