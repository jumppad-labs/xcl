---
tags: [releases, workflow, spektacular, testing]
---

# Write a release plan once released work is merged

When a spec or an epic whose work needs a release has been implemented and merged, write its release plan before anything is pushed, with the local `release-plan` skill (`.claude/skills/release-plan/SKILL.md` in the xcl project). Store it once, with the work it releases:

- one spec: in the spec's plan documents as `release-plan`, beside `plan`, `context`, `research` and `test-plan` (`spektacular plan file write <spec> release-plan --from <staged file>`);
- an epic: with the epic, never with its specs, which get no `release-plan` document at all. The epic store holds no extra documents, so the plan is the last section of the epic body, `## Release plan`, written with `spektacular epic write <epic> --from <staged body>` (no `--data`, so the specs list is kept). A regenerated plan replaces that section rather than adding another. Read it with `spektacular epic read <epic>`.

The plan orders every release step across the repos the work touches, names what each step's automation does by itself, and carries the testing: automated suites before pushing, every manual check from each spec's `test-plan`, the CI checks to see pass at each step, and post-release verification of what was published. It also gives the release-notes lines (from each changelog's Breaking items and any descoped release-notes requirement) and what to do when a step fails.

The plan is for a person. Writing it never pushes, labels, merges, tags or publishes anything. It needs no review before it is stored, only to be technically correct: every command, workflow, label and version checked against the repos.

**Why:** implement runs descope release tasks to "done by a person after the merge" because the workflow cannot finish a human task, and the order between repos (engine release, then the package's pin bump, then the website deploy) is easy to get wrong. Without a written plan, the descoped steps and manual checks are scattered across several specs' plans and test plans.

**How to apply:** at the end of an implement run (or an epic's), if any spec has a descoped release requirement or touches a repo that releases, run the `release-plan` skill and store the plan without asking; when the user asks how to release implemented work, point them to the stored plan, or run the skill if there is none.
