---
created_date: "2026-10-14"
document_status: final
closed_date: "2026-10-14"
---

# Plan: 20261014052450-0b131707-ci-release-automation

<!-- Metadata -->
<!-- Created: 2026-10-14T05:54:18+00:00 -->
<!-- Commit: 63c4af6acfbb7255b24e5d35da6b23ea6c1245d6 -->
<!-- Branch: specs/ci-release-and-python-dataclasses -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Automates releases for xcl and its language bindings (today xcl-python). Every PR declares its release size with a label and merging tags and releases automatically; xcl PRs must pass the Python suite against their own code unless a maintainer adds `breaks-bindings`; and Python gets its own version line, picking up each engine release through an automatically verified, auto-merging pin-update PR. Maintainers get a hands-off, documented process, and language fixes no longer force releases of everything else.

## Conventions

- **Never write tests that inspect repository files (no test reads CI workflow files, README or CHANGELOG)** — this feature is mostly workflow YAML and README text; they are verified by running the workflows and by review, never by tests that parse them.
- **Testing & Mocking: unit tests for business logic, no table-driven tests, never mix positive and negative cases in one test** — applies to the xcl-python engine-pin check (its tests in `tests/unit/test_client.py` move to the pinned engine version and stay one case per test).
- **Releases were tagged by hand; before 1.0, breaking changes bump the minor** (learning `releases-tagged-by-hand.md`) — the auto-tag replaces the hand step; the `minor` label is what a pre-1.0 breaking change uses, and `major` deliberately means 1.0.0.

## Architecture & Design Decisions

**Shape.** The process is built from four small GitHub Actions pieces, shared through reusable workflows that live in xcl so every language repo gets the same behaviour from one line:

1. **Release-label check** (reusable, `xcl:.github/workflows/release-label-check.yml`, called from a thin `release-label.yml` in each repo). Runs on every PR open, label change and push, and fails unless exactly one of `major`, `minor`, `patch`, `no-release` is set. It is a required check in both repos.
2. **Auto-tag on merge** (reusable, `xcl:.github/workflows/auto-tag.yml`, called from `tag.yml` in each repo on push to `main`). Finds the PR that produced the merge commit, reads its release label, and for anything but `no-release` computes the next version from the latest `v*` tag (`major` from 0.x gives 1.0.0) and pushes the tag **with a GitHub App installation token**. Because the App, not `GITHUB_TOKEN`, pushed it, each repo's existing tag-triggered `release.yml` starts as it does today. Keeping `release.yml` as a top-level tag-triggered workflow leaves PyPI trusted publishing, which is bound to that workflow file, untouched.
3. **Cross-language PR check** (xcl `bindings.yml`). Calls `jumppad-labs/xcl-python/.github/workflows/test.yml@main` with the xcl PR's head SHA. A final `bindings` gate job, which always runs and is never skipped, passes when the suite passed or the PR carries the override label `breaks-bindings`; that gate is the required check. When the override label is set the suite is not run at all. The workflow listens to label events so adding or removing the override re-evaluates; any other label change re-runs the suite, which is correct but slower. This is accepted over a gate that could be skipped, because a skipped job reports success.
4. **Engine propagation** (xcl-python `engine-bump.yml`). xcl's `release.yml` keeps its repository_dispatch to xcl-python, now carrying the tag and the bump size. xcl-python, using the App token, writes the new tag into `xcl-ref.txt` on a branch, opens a PR labelled with the mirrored bump size, and enables auto-merge. The PR's own required checks gate the merge, and the merge triggers that repo's `tag.yml`, so an engine patch ends as a Python patch release with no hands. A red bump PR simply stays open.

**Independent Python versions.** xcl-python's `release.yml` changes trigger from the xcl dispatch to its own `v*` tag. The package version comes from the xcl-python tag; the engine comes from `xcl-ref.txt`, which is now the single pin for CI, the release download and the runtime check. The build stamps that pin into the package, and the client compares the engine's reported version with the pin, not with the package version (`xcl-python:xcl/_client.py:76-91`). The GitHub release for each Python tag is created with generated notes plus a line naming the bundled engine. xcl-python is seeded with a `v0.7.0` tag on the commit that shipped 0.7.0 to PyPI, so the first label-driven release is at least 0.7.1, above everything already on PyPI.

**xcl-python's own CI becomes the reusable `test.yml`.** Its existing jobs (`xcl-python:.github/workflows/ci.yml:36-180`) move into a `workflow_call` workflow taking `python-ref` and `xcl-ref`, with a final `result` gate job. Every checkout names its repository explicitly, since a called workflow's default checkout is the caller's repo. `ci.yml` then calls it for its own PRs, with its own commit and the pinned xcl ref. The same suite runs in both places, so a binding check on xcl and the binding's own CI cannot drift.

**Why this direction.** It reuses everything that already builds, signs, tests and publishes (`xcl:.github/workflows/release.yml`, `xcl-python:.github/workflows/release.yml`) and changes only what starts a release and which version it gets. Results show as ordinary checks on the PR. Adding a language means one `uses:` line in `bindings.yml`, one dispatch target, and the same three thin workflows in the new repo. Rejected options are dispatch-and-post-status checks, `GITHUB_TOKEN` tag pushes, release as a called workflow (it breaks the PyPI trusted publisher binding), ruleset bypass as the override, and skippable gates; see research.md#alternatives-considered-and-rejected. Repository settings (labels, branch protection with required checks, auto-merge enabled, App installation and secrets, the seed tag) are one-off maintainer steps, done in order so that each required check exists before it is required. Per the testing convention, the workflow files are verified by running them, not by tests that read them.

## Component Breakdown

- **Release-label check (new, shared, owned by xcl).** Decides whether a PR carries exactly one release label. Owned once in xcl as a reusable workflow; each repo calls it from a thin per-repo workflow so the result is a required check in that repo. Feeds nothing else at runtime; the auto-tag relies on the guarantee it enforces.
- **Auto-tag (new, shared, owned by xcl).** Turns a merged PR into a version tag: finds the PR behind the merge commit, reads its release label, computes the next semantic version from the latest tag, and pushes the tag with the release App's token. Owned once in xcl as a reusable workflow, called on push to `main` by each repo. Its tag starts that repo's Release.
- **xcl Release (changed).** Still builds, signs and publishes the engine on a tag, and still dispatches xcl-python. The dispatch now also carries the bump size, so the binding can mirror it. Its GitHub release notes are generated from the PRs merged since the previous tag, as today.
- **Bindings check (new, xcl).** Runs every language binding's reusable test suite against the xcl PR's head commit, and owns the single required gate that honours the `breaks-bindings` override label. Depends on each binding publishing a reusable test suite.
- **Python test suite (changed, xcl-python).** The existing CI jobs become one reusable suite, parameterised by which xcl-python and which xcl to test, ending in one result gate. Called by xcl-python's own CI (its commit, pinned engine) and by xcl's Bindings check (`main`, the xcl PR's commit).
- **Engine bump (new, xcl-python).** Receives xcl's release dispatch, moves the engine pin to the new tag on a branch, opens a PR labelled with the mirrored bump size using the release App's token, and enables auto-merge. The PR's required checks decide whether it merges; its merge feeds Auto-tag.
- **Python Release (changed, xcl-python).** Starts from xcl-python's own tag instead of xcl's dispatch. Takes the package version from that tag and the engine from the pin, stamps the pin into the package, publishes to PyPI as today, and creates a GitHub release whose notes list merged PRs and name the bundled engine.
- **Engine pin and runtime check (changed, xcl-python).** The pin file becomes the single source of which engine CI tests, the release bundles, and the package runs against. The client's startup check compares the engine's reported version with the stamped pin instead of the package's own version.
- **Release App and repository settings (new, maintainer-owned).** A GitHub App installed on xcl and xcl-python, whose credentials are secrets in both, plus the release labels, branch protection with the required checks, auto-merge enabled, and the xcl-python seed tag. These are one-off settings, not code, applied in order so each check exists before it is required.
- **Release process documentation (changed).** xcl's README gains the full process. xcl-python's README gains a short section on its labels and engine pin, linking to xcl's.

## Data Structures & Interfaces

No program types are added. The contracts are the interfaces between workflows, one dispatch payload, the package's version stamp, and repository settings.

**Release labels.** A fixed set of four PR labels, identical in every participating repo: `major`, `minor`, `patch`, `no-release`. Exactly one per PR. The override label `breaks-bindings` exists only in xcl.

**Release-label check (reusable workflow).** No inputs; reads the calling PR's labels. Its only output is pass/fail.

**Auto-tag (reusable workflow).**
```
inputs:  none (reads the merge commit of the calling push)
secrets: RELEASE_APP_ID, RELEASE_APP_PRIVATE_KEY
outputs: tag    — e.g. "v0.7.1", empty for no-release
         bump   — "major" | "minor" | "patch" | "none"
```
Next version: semver from the highest `v*` tag; `major` while below 1.0 yields `1.0.0`.

**Binding test suite (reusable workflow, one per language).**
```
inputs: binding-ref — the binding repo's ref to test (default: main)
        xcl-ref     — the xcl ref to build the engine from (default: the binding's pin)
result: one final job whose conclusion is the suite's result
```
This is the contract a new language must implement to join the Bindings check.

**Release dispatch payload (xcl → each binding, event type `xcl-release`).**
```
client_payload: { tag: "v0.8.0", bump: "major" | "minor" | "patch" }
```
`bump` is new; `tag` is unchanged.

**Engine pin.** `xcl-ref.txt` in a binding repo holds one xcl tag (or, transiently in a bump PR, the new tag). It is the only source of the engine version for CI, the release download, and the runtime check.

**Package version stamp (xcl-python).** The version module carries two values:
```
__version__         = package version, from the xcl-python tag at release
__engine_version__  = the pinned engine tag at release; a fixed development value otherwise
```
The development engine build is stamped with the same development value, so local and CI runs pass the check. The client compares the engine's reported version with `__engine_version__`; `EngineVersionError` keeps its shape, with the expected side now the pin.

**Repository settings (per repo).** Labels as above; branch protection on `main` requiring the release-label check and the repo's test gates (in xcl also the `bindings` gate); auto-merge allowed; secrets `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY`.

## Implementation Detail

**New pattern: release machinery as reusable workflows owned by the engine repo.** Until now each repo's workflows were self-contained. This plan introduces shared workflows in xcl (release-label check, auto-tag) that every participating repo calls by reference, plus a per-binding reusable test suite that xcl calls. The shape a developer sees in each binding repo is three thin caller workflows (label check, tag on merge, engine bump) and one substantive reusable suite. Adding a language means copying those thin callers and implementing the suite contract, not designing CI again.

**New pattern: never-skipped gate jobs as the required checks.** Every check a branch rule requires is a final job that always runs and computes its own conclusion from the jobs before it (and, for the bindings gate, from the override label). Matrix jobs are never required individually, so their names can change without touching branch protection, and a skipped job can never satisfy a rule by accident.

**Existing pattern kept: tag-triggered releases.** Both `release.yml` files stay top-level workflows started by a `v*` tag push, so their build, sign, verify and publish steps, the existing manual dry-run mode, and PyPI trusted publishing are untouched. The auto-tag pushes the tag with the App token precisely so this pattern still works.

**Code-shape change in xcl-python CI.** The current `ci.yml` jobs move bodily into the reusable suite. `ci.yml` shrinks to a caller. Inside the suite, every checkout names its repository and ref explicitly, and the xcl ref comes from an input that falls back to the pin file, replacing the hard-coded read of the pin.

**Code-shape change in the Python version check.** The version module gains the pinned engine value. The client's check, the release's "engine reports the expected version" step, the PyPI install check, and the development engine and shared-library builds all switch from the package version to that value. The error type and message shape stay; only the expected side changes. Tests that built fake engines reporting the package version now report the pin.

**Bot-authored changes.** The engine-bump PR is the first change in these repos authored by automation. It uses a predictable branch name per engine tag (so a re-run updates the same PR instead of opening another), a label mirroring the engine bump, and auto-merge, so a maintainer only sees it when it is red.

**Documentation.** xcl's README gains a "Releases" section covering labels, the bindings check and override, auto-tagging, and engine propagation with the worked example from the spec. xcl-python's README gains a short "Releases and the engine pin" section that links to it.

## Dependencies

- **Design documents this plan was built on:** none. The spec carries no design references.
- **GitHub Actions and branch protection** (external platform, mandated by the spec's constraint). Provide reusable workflows, required status checks and auto-merge. Needs settings changes on both repos: labels, protection, `allow_auto_merge`.
- **A GitHub App for release automation** (external, new; **must exist before the auto-tag and engine-bump workflows can work**). Installed on xcl and xcl-python with contents, pull-requests and workflows write access. Its ID and private key are stored as secrets in both repos. Minted in workflows with `actions/create-github-app-token`.
- **`XCL_PYTHON_DISPATCH_TOKEN`** (existing secret on xcl). Still used for the release dispatch to xcl-python; no change. It may later be replaced by the App token.
- **xcl Release workflow** (existing). Builds, signs and publishes `xcl-core`. Its dispatch gains the bump size; otherwise unchanged.
- **xcl-python Release workflow and PyPI trusted publishing** (existing). The trigger changes to xcl-python's own tag. It must remain the top-level workflow named in the PyPI trusted publisher, so no PyPI-side change is needed.
- **jumppad-labs release GPG key** (existing secrets `XCL_RELEASE_GPG_KEY` and `XCL_RELEASE_GPG_PASSPHRASE`, and the public key in xcl-python's tests). Unchanged.
- **GitHub CLI and `--generate-notes`** (existing, used by the xcl release). Provides the PR list for release notes; xcl-python adopts it as well.
- **Prior plans** `python-out-of-process-engine` and `python-release-hardening` (landed). They produced the current release and dispatch pipeline this plan modifies; nothing further needs to land first.
- **Downstream:** the parked spec `python-plain-dataclasses` depends on this plan landing first. It is the first change intended to ship through the new process.

## Testing Approach

**The decisions are unit-tested in Go; the workflows are verified by running them.** The two pieces of real logic are deciding a PR's release size from its labels, and computing the next version from the existing tags and that size. They live in a small Go command in xcl that the shared workflows run. They get ordinary unit tests in xcl's style: testify `require`, one behaviour per test, positive and negative cases in separate functions, and no table-driven tests. The load-bearing assertions:
- exactly one release label is accepted, and none, two, or only unrelated labels are rejected with a message naming the problem;
- `patch`, `minor` and `major` advance the correct component;
- `major` below 1.0 gives `1.0.0`;
- non-semver and pre-release tags are ignored when finding the latest version;
- `no-release` yields no tag;
- a repo with no tags is reported as needing a seed tag, not silently started at 0.0.1.

The workflow YAML itself is not tested by any test that reads it (project convention). It is proved by exercising it.

**Python version check:** unit tests in xcl-python's existing style. The client accepts an engine reporting the pinned version, and rejects one reporting the package version when the two differ, with `EngineVersionError` naming both. Positive and negative cases are in separate tests. Existing tests and fakes that assumed engine version = package version are moved to the pin, and the integration and parity suites continue to run unchanged against a development engine stamped with the development pin.

**End-to-end verification on the real repos.** Rollout is staged so each behaviour is observed before the next depends on it: label check, then bindings check, then auto-tag, then engine bump.

**Success metrics:**
- *Proven on real work (the parked Python plain-dataclasses spec ships through this process)*: **Manual — captured in the implementation test plan.** It is observed when that spec is implemented, after this plan lands.
- *No manual tags after rollout*: **Manual — captured in the implementation test plan.** Check the tag history against merged, labelled PRs.
- *No surprise breakage at release*: **Manual — captured in the implementation test plan.** For each xcl release, check that the bindings gate on its PR was green or carried the override.
- *Language fixes stay local*: **Manual — captured in the implementation test plan.** A `patch` PR on xcl-python publishes to PyPI with no xcl tag.
- *Engine fixes reach Python without hands*: **Manual — captured in the implementation test plan.** An xcl `patch` release produces a merged bump PR and a published Python patch with no maintainer action.

**Acceptance criteria that are observed live, not unit-tested** (each **Manual — captured in the implementation test plan**):
- the label check blocks a PR with no label or two labels;
- `patch`, `minor` and `major` merges create the right tags and run the release;
- `no-release` creates nothing;
- the Python check appears on an xcl PR, blocks a breaking PR, and is overridden by `breaks-bindings` only for users who can label;
- an engine release opens a bump PR, which merges and publishes when green and stays open when red;
- release notes list merged PRs and name the bundled engine;
- engine checksums still verify, and the package still installs from PyPI on every supported platform.

**Manual reviews:**
- Review of the new README sections against the spec: **Manual — captured in the implementation test plan.**
- Review of each repo's branch protection and App permissions once applied: **Manual — captured in the implementation test plan.**

## Milestones & Tasks

### Milestone 1: Every pull request declares its release size

**What changes:** Pull requests on xcl and xcl-python have to carry exactly one release label, `major`, `minor`, `patch` or `no-release`, before they can merge. A missing or doubled label shows as a failing required check that names the problem. Nothing is released automatically yet; this milestone only establishes the labels and the rule, so every later piece can rely on them.

**Validation point:** On each repo, a PR with no label is blocked, adding one label unblocks it, and adding a second blocks it again. The label-decision unit tests pass in xcl's CI.

#### - [ ] Task: Release decision command
**Id:** 699b935b-aab4-49f8-a96c-8f0cdd8b1341
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds a small command to xcl that holds the two release decisions: whether a PR's labels name exactly one release size, and what the next version is given the existing tags and that size. It also reports the bump size between two tags, which the engine release uses to tell bindings how big a change it was. Keeping this in tested Go code means the shared workflows only call it, and the rules are unit-tested.

*Technical detail:* [context.md#task-release-decision-command](./context.md#task-release-decision-command)

**Acceptance criteria**:
- [ ] A PR with exactly one of `major`, `minor`, `patch` or `no-release` is accepted, and one with none, two, or only unrelated labels is rejected with a message saying which.
- [ ] From `v0.7.0`, `patch` gives `v0.7.1`, `minor` gives `v0.8.0` and `major` gives `v1.0.0`; above 1.0, `major` advances the major number.
- [ ] `no-release` gives no version, and a repository with no version tags is reported as needing a seed tag.
- [ ] Tags that are not plain versions are ignored when finding the latest one.
- [ ] The bump size between two versions is reported as major, minor or patch.

#### - [ ] Task: Shared release-label check for xcl
**Id:** 828f193f-0ff6-46cb-bfbb-4618dde9043c
**Repo:** xcl
**Depends on:**
- 699b935b-aab4-49f8-a96c-8f0cdd8b1341 — Release decision command
**Execution:** agent

Adds the shared label check that any participating repository can call. It runs the release decision command against the PR's labels and fails with its message. xcl also calls it for its own pull requests, so the check appears on every xcl PR and is re-evaluated whenever labels change.

*Technical detail:* [context.md#task-shared-release-label-check-for-xcl](./context.md#task-shared-release-label-check-for-xcl)

**Acceptance criteria**:
- [ ] Every xcl PR shows a "release label" check that is red without a release label and green with exactly one.
- [ ] Adding or removing a label re-runs the check without a new push.
- [ ] Another repository can use the same check by referring to it, with no copy of its logic.

#### - [ ] Task: Release-label check on xcl-python
**Id:** 8d5b6c9c-55e8-4cac-91a3-1075f28a8f09
**Repo:** xcl-python
**Depends on:**
- 828f193f-0ff6-46cb-bfbb-4618dde9043c — Shared release-label check for xcl
**Execution:** agent

Makes xcl-python call xcl's shared label check for its pull requests, so both repos apply the same rule.

*Technical detail:* [context.md#task-release-label-check-on-xcl-python](./context.md#task-release-label-check-on-xcl-python)

**Acceptance criteria**:
- [ ] Every xcl-python PR shows the same "release label" check, behaving as it does on xcl.

#### - [ ] Task: Labels and label rule on xcl
**Id:** 8828f594-b3c0-470a-9c3d-9e194d85f0e9
**Repo:** xcl
**Depends on:**
- 828f193f-0ff6-46cb-bfbb-4618dde9043c — Shared release-label check for xcl
**Execution:** human — needs repository admin access to create labels and branch protection

Creates the four release labels on xcl and protects `main` so the release-label check, plus the existing Go and examples checks, must pass before merging.

*Technical detail:* [context.md#task-labels-and-label-rule-on-xcl](./context.md#task-labels-and-label-rule-on-xcl)

**Acceptance criteria**:
- [ ] xcl has the labels `major`, `minor`, `patch` and `no-release`.
- [ ] An xcl PR can't be merged while its release-label check is red.

#### - [ ] Task: Labels and label rule on xcl-python
**Id:** 31fbb5e2-6d78-4147-93c7-bbe2eb1af0ae
**Repo:** xcl-python
**Depends on:**
- 8d5b6c9c-55e8-4cac-91a3-1075f28a8f09 — Release-label check on xcl-python
**Execution:** human — needs repository admin access to create labels and branch protection

Creates the four release labels on xcl-python and protects `main` so the release-label check must pass before merging.

*Technical detail:* [context.md#task-labels-and-label-rule-on-xcl-python](./context.md#task-labels-and-label-rule-on-xcl-python)

**Acceptance criteria**:
- [ ] xcl-python has the labels `major`, `minor`, `patch` and `no-release`.
- [ ] An xcl-python PR can't be merged while its release-label check is red.

### Milestone 2: Engine pull requests are checked against Python

**What changes:** Every xcl pull request runs the full Python test suite against its own code and shows the result as a check on the PR. A PR that breaks Python can't merge unless a maintainer adds the `breaks-bindings` label. xcl-python's own CI runs the same suite, so the two can't drift.

**Validation point:** A harmless xcl PR shows a green Python check. A deliberately breaking draft PR shows red and can't merge, and turns mergeable once `breaks-bindings` is added. xcl-python PRs still run their full suite.

#### - [ ] Task: Reusable Python test suite
**Id:** b5ff6015-12a9-4963-b38a-3a4b4a5b701a
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Turns xcl-python's existing CI jobs into one reusable suite that can test any xcl-python commit against any xcl commit, ending in a single result job. xcl-python's own CI then calls it with its own commit and the pinned engine, so the suite xcl runs and the suite xcl-python runs are the same.

*Technical detail:* [context.md#task-reusable-python-test-suite](./context.md#task-reusable-python-test-suite)

**Acceptance criteria**:
- [ ] xcl-python PRs still run every platform, Python version, Alpine and in-process job they run today.
- [ ] One "result" check summarises the suite, and is red if any job failed or was cancelled.
- [ ] The suite can be started from another repository with a chosen xcl commit, and then tests xcl-python `main` against that commit.

#### - [ ] Task: Bindings check on xcl pull requests
**Id:** 7eb07133-4391-4146-bec2-7305bd75098e
**Repo:** xcl
**Depends on:**
- b5ff6015-12a9-4963-b38a-3a4b4a5b701a — Reusable Python test suite
**Execution:** agent

Runs the Python suite against every xcl PR's own commit and adds a single "bindings" gate. The gate passes when the suite passes, or when the PR carries `breaks-bindings`, in which case the suite isn't run. The gate always runs, so it can never pass by being skipped.

*Technical detail:* [context.md#task-bindings-check-on-xcl-pull-requests](./context.md#task-bindings-check-on-xcl-pull-requests)

**Acceptance criteria**:
- [ ] An xcl PR shows the Python suite's results and a "bindings" gate.
- [ ] A PR that breaks Python has a red gate, and adding `breaks-bindings` turns it green without running the suite.
- [ ] Removing `breaks-bindings` runs the suite again and the gate follows its result.
- [ ] Changing any other label never leaves the gate green when the suite is red.

#### - [ ] Task: Require the bindings gate on xcl
**Id:** e0e8b774-7843-4f0b-a63f-2d12aaa37810
**Repo:** xcl
**Depends on:**
- 7eb07133-4391-4146-bec2-7305bd75098e — Bindings check on xcl pull requests
- 8828f594-b3c0-470a-9c3d-9e194d85f0e9 — Labels and label rule on xcl
**Execution:** human — needs repository admin access to change labels and branch protection

Creates the `breaks-bindings` label on xcl and adds the "bindings" gate to `main`'s required checks.

*Technical detail:* [context.md#task-require-the-bindings-gate-on-xcl](./context.md#task-require-the-bindings-gate-on-xcl)

**Acceptance criteria**:
- [ ] An xcl PR with a red bindings gate can't be merged, and only people who can label PRs can apply `breaks-bindings`.

#### - [ ] Task: Require the test result on xcl-python
**Id:** 70865473-b724-471e-b760-c98819de1160
**Repo:** xcl-python
**Depends on:**
- b5ff6015-12a9-4963-b38a-3a4b4a5b701a — Reusable Python test suite
- 31fbb5e2-6d78-4147-93c7-bbe2eb1af0ae — Labels and label rule on xcl-python
**Execution:** human — needs repository admin access to change branch protection

Adds the suite's single result check to xcl-python's required checks, so nothing merges into xcl-python, including the automated engine updates, while its tests are red.

*Technical detail:* [context.md#task-require-the-test-result-on-xcl-python](./context.md#task-require-the-test-result-on-xcl-python)

**Acceptance criteria**:
- [ ] An xcl-python PR with a failing suite can't be merged.

### Milestone 3: Merging releases automatically, and Python versions independently

**What changes:** Merging a labelled PR tags the next version and releases it, with no manual step, in both repos. `no-release` merges do nothing. Python gets its own version line, starting above everything already on PyPI. Each Python release bundles the engine named by its pin, checks at startup that it is running on that engine, and publishes release notes listing the merged PRs and the bundled engine. A Python-only fix now releases only Python.

**Validation point:** A `patch` PR merged on xcl-python publishes the next Python patch to PyPI, with no xcl tag, and the installed package reports the pinned engine. A `no-release` merge creates no tag. An xcl `patch` merge produces the next xcl tag, a signed release, and generated notes.

#### - [ ] Task: Release App on xcl
**Id:** 7577a52a-6f60-4f51-8d45-55a5b14bbe76
**Repo:** xcl
**Depends on:** none
**Execution:** human — needs organisation admin access to create and install a GitHub App and to set secrets

Creates the jumppad-labs release GitHub App, installs it on xcl and xcl-python, and stores its ID and private key as secrets on xcl. The automation uses it wherever an action must start further workflows, which the built-in Actions token can't do.

*Technical detail:* [context.md#task-release-app-on-xcl](./context.md#task-release-app-on-xcl)

**Acceptance criteria**:
- [ ] The App is installed on xcl and xcl-python with the permissions listed in the technical detail.
- [ ] xcl has the secrets `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY`.

#### - [ ] Task: Release App secrets on xcl-python
**Id:** 5715f137-a3e9-436f-92cc-7c3eb5e3fbf2
**Repo:** xcl-python
**Depends on:**
- 7577a52a-6f60-4f51-8d45-55a5b14bbe76 — Release App on xcl
**Execution:** human — needs repository admin access to set secrets

Stores the release App's ID and private key as secrets on xcl-python.

*Technical detail:* [context.md#task-release-app-secrets-on-xcl-python](./context.md#task-release-app-secrets-on-xcl-python)

**Acceptance criteria**:
- [ ] xcl-python has the secrets `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY`.

#### - [ ] Task: Auto-tag on merge
**Id:** 5ea16ccb-c247-4391-9f8d-0e6dee7ad720
**Repo:** xcl
**Depends on:**
- 699b935b-aab4-49f8-a96c-8f0cdd8b1341 — Release decision command
- 7577a52a-6f60-4f51-8d45-55a5b14bbe76 — Release App on xcl
**Execution:** agent

Adds the shared auto-tag workflow and calls it from xcl on every push to `main`. It finds the merged PR, asks the release decision command for the next version, and pushes that tag with the App's token, which starts xcl's existing release. `no-release` merges and pushes with no PR do nothing.

*Technical detail:* [context.md#task-auto-tag-on-merge](./context.md#task-auto-tag-on-merge)

**Acceptance criteria**:
- [ ] Merging a `patch`, `minor` or `major` PR on xcl creates the next tag, and the signed engine release runs for it with no manual step.
- [ ] Merging a `no-release` PR creates no tag.
- [ ] Two merges in quick succession get two consecutive versions, never the same one.

#### - [ ] Task: Python checks against its pinned engine
**Id:** 8017cbb4-a82a-4327-a7a2-b352892858c2
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Separates the engine version from the package version. The package records the engine it was built for, from the pin, and its startup check, development builds, tests and install check all compare against that instead of the package's own version.

*Technical detail:* [context.md#task-python-checks-against-its-pinned-engine](./context.md#task-python-checks-against-its-pinned-engine)

**Acceptance criteria**:
- [ ] A package whose version differs from its engine's runs when the engine matches the pin.
- [ ] It refuses an engine that doesn't match the pin, naming the expected and actual versions.
- [ ] `xcl.engine_version()` still reports the running engine, and development builds and the full test suite still pass.

#### - [ ] Task: Seed tag for xcl-python
**Id:** 69655b3f-f8d2-411b-a194-17f0070210f5
**Repo:** xcl-python
**Depends on:** none
**Execution:** human — needs push access to create a tag on the release commit, a release action outside the code

Tags the xcl-python commit that produced the 0.7.0 package on PyPI as `v0.7.0`, so automatic versions continue above everything already published.

*Technical detail:* [context.md#task-seed-tag-for-xcl-python](./context.md#task-seed-tag-for-xcl-python)

**Acceptance criteria**:
- [ ] xcl-python has a `v0.7.0` tag on the commit that shipped 0.7.0, and no other version tags.

#### - [ ] Task: Python releases from its own tags
**Id:** 81b6a536-fbd2-4b76-9645-9c1a7d8f65b4
**Repo:** xcl-python
**Depends on:**
- 5ea16ccb-c247-4391-9f8d-0e6dee7ad720 — Auto-tag on merge
- 8017cbb4-a82a-4327-a7a2-b352892858c2 — Python checks against its pinned engine
- 69655b3f-f8d2-411b-a194-17f0070210f5 — Seed tag for xcl-python
- 5715f137-a3e9-436f-92cc-7c3eb5e3fbf2 — Release App secrets on xcl-python
**Execution:** agent

Moves the Python release from xcl's dispatch to xcl-python's own tags, and adds auto-tag on merge to xcl-python. Each release takes its version from its tag and its engine from the pin, verifies and bundles that signed engine, publishes to PyPI as today, and creates a GitHub release whose notes list the merged PRs and name the bundled engine.

*Technical detail:* [context.md#task-python-releases-from-its-own-tags](./context.md#task-python-releases-from-its-own-tags)

**Acceptance criteria**:
- [ ] Merging a `patch` PR on xcl-python publishes the next patch to PyPI with no xcl tag or release.
- [ ] The installed package reports and checks the pinned engine, not its own version.
- [ ] The xcl-python GitHub release lists the merged PRs and names the bundled engine.
- [ ] The xcl release dispatch no longer publishes Python by itself.

### Milestone 4: Engine releases reach Python by themselves, and the process is documented

**What changes:** Each xcl release opens a pull request in xcl-python that moves the engine pin to the new release, labelled to mirror the engine's bump. When its checks pass it merges and releases itself; when they fail it waits for a maintainer. xcl's README describes the whole process, and xcl-python's README explains its labels and engine pin.

**Validation point:** An xcl `patch` release is followed, with no maintainer action, by a merged pin-update PR and a published Python patch bundling the new engine. A pin-update PR forced red stays open and unpublished. Both README sections are reviewed.

#### - [ ] Task: Engine release tells bindings its size
**Id:** eb9aaca4-4d9d-467b-a20d-aa6db201cc17
**Repo:** xcl
**Depends on:**
- 699b935b-aab4-49f8-a96c-8f0cdd8b1341 — Release decision command
**Execution:** agent

Makes the xcl release include the bump size, worked out from the previous tag, in what it sends to xcl-python, so the binding can mirror it. The build, signing and release notes are unchanged.

*Technical detail:* [context.md#task-engine-release-tells-bindings-its-size](./context.md#task-engine-release-tells-bindings-its-size)

**Acceptance criteria**:
- [ ] An xcl release tells xcl-python both the new tag and whether it was a major, minor or patch release.
- [ ] The engine archives, signature and generated release notes are produced exactly as before.

#### - [ ] Task: Engine update pull requests in xcl-python
**Id:** 96df2316-c5ca-4ffb-994f-9cfa219cb3be
**Repo:** xcl-python
**Depends on:**
- eb9aaca4-4d9d-467b-a20d-aa6db201cc17 — Engine release tells bindings its size
- 81b6a536-fbd2-4b76-9645-9c1a7d8f65b4 — Python releases from its own tags
**Execution:** agent

When xcl releases, xcl-python opens a pull request that moves its engine pin to the new tag, labels it with the mirrored size, and turns on auto-merge, all with the App's token so its checks run and its merge releases. A re-run for the same tag updates the same pull request.

*Technical detail:* [context.md#task-engine-update-pull-requests-in-xcl-python](./context.md#task-engine-update-pull-requests-in-xcl-python)

**Acceptance criteria**:
- [ ] An xcl release produces one open PR in xcl-python moving the pin to the new tag, labelled to match the engine's bump.
- [ ] When its checks pass it merges by itself and a Python release follows.
- [ ] When its checks fail it stays open and nothing is published.

#### - [ ] Task: Allow auto-merge on xcl-python
**Id:** 6b084007-03f2-4d30-b38d-df65a8b51697
**Repo:** xcl-python
**Depends on:**
- 70865473-b724-471e-b760-c98819de1160 — Require the test result on xcl-python
**Execution:** human — needs repository admin access to change repository settings

Turns on "Allow auto-merge" for xcl-python, so the engine update PRs can merge themselves once their required checks pass.

*Technical detail:* [context.md#task-allow-auto-merge-on-xcl-python](./context.md#task-allow-auto-merge-on-xcl-python)

**Acceptance criteria**:
- [ ] xcl-python allows auto-merge, and auto-merge waits for the required checks.

#### - [ ] Task: Document the release process in xcl
**Id:** b40284cd-3208-44dd-94aa-3ed44163b66e
**Repo:** xcl
**Depends on:**
- 5ea16ccb-c247-4391-9f8d-0e6dee7ad720 — Auto-tag on merge
- 7eb07133-4391-4146-bec2-7305bd75098e — Bindings check on xcl pull requests
**Execution:** agent

Adds a "Releases" section to xcl's README covering the release labels, the bindings check and the `breaks-bindings` override, automatic tagging and releases, independent language versions, and how engine releases reach each language, with the spec's worked example. It also explains what a new language must provide to join.

*Technical detail:* [context.md#task-document-the-release-process-in-xcl](./context.md#task-document-the-release-process-in-xcl)

**Acceptance criteria**:
- [ ] A contributor can learn from xcl's README which label to use, what happens when they merge, how to override a binding failure, and how an engine fix reaches Python.

#### - [ ] Task: Document releases and the engine pin in xcl-python
**Id:** 334cfd14-110e-46b4-b4b9-34786b1c8fa7
**Repo:** xcl-python
**Depends on:**
- 81b6a536-fbd2-4b76-9645-9c1a7d8f65b4 — Python releases from its own tags
**Execution:** agent

Adds a short section to xcl-python's README on its release labels, its own version line, the engine pin and the automated engine updates, linking to xcl's full description. It also corrects text that says the package version equals the xcl release it bundles.

*Technical detail:* [context.md#task-document-releases-and-the-engine-pin-in-xcl-python](./context.md#task-document-releases-and-the-engine-pin-in-xcl-python)

**Acceptance criteria**:
- [ ] xcl-python's README explains its labels and engine pin and links to xcl's release section.
- [ ] Nothing in xcl-python's README still says the package version equals the engine version.

## Open Questions

- **Does a fork PR's head commit check out in the called Python suite?** This depends on GitHub letting `jumppad-labs/xcl` be checked out at a fork PR's SHA from a workflow called by xcl. It can only be seen on the first fork PR. If it fails, STOP and ask the user whether fork PRs need maintainer approval runs or a different ref.
- **Does an App-created tag ref start `release.yml` in both repos as expected?** This is documented GitHub behaviour, but only the first real merge proves it with this App's permissions. If no release run starts, STOP and ask; don't fall back to manual tagging silently.
- **Does PyPI trusted publishing accept the first release started by xcl-python's own tag?** The workflow file and environment are unchanged, but the trigger is new. If publishing fails with an OIDC claim mismatch, STOP and ask the user to check the trusted publisher settings on PyPI.

## Out of Scope

- **Plugin repos, xcl-website and xcl-vscode.** They adopt neither the labels nor the cross-repo checks. A later spec can bring them in using the same shared workflows.
- **New languages.** The design is built to take them through the reusable suite contract and the thin per-repo workflows, but none are added here.
- **Engine version ranges.** Each language package bundles and runs exactly its pinned engine.
- **How artefacts are built.** Engine compilation, signing, the wheel platform matrix and the in-process shared library build are unchanged.
- **The Python API.** The plain-dataclasses change is the parked spec `20261014043756-e853a081-python-plain-dataclasses`, which ships after this plan as the first real use of the process.
- **Same-branch-name coordination.** Coordinated breaking changes use the `breaks-bindings` override, not automatic branch matching.
- **Replacing `XCL_PYTHON_DISPATCH_TOKEN` with the App token.** It still works and is left as is; swapping it is an optional later tidy-up.
- **Avoiding suite re-runs on unrelated label changes.** Accepted for correctness; a gate that replays the previous result is a possible later optimisation.
