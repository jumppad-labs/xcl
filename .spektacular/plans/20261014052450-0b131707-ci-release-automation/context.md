---
created_date: "2026-10-14"
document_status: final
closed_date: "2026-10-14"
---

# Context: 20261014052450-0b131707-ci-release-automation

## Current State Analysis

- xcl releases are tag-triggered (`xcl:.github/workflows/release.yml:31-34`), build 8 signed `xcl-core` archives, create the GitHub release with `--generate-notes`, then repository_dispatch `xcl-release` to xcl-python (`release.yml:200-218`, secret `XCL_PYTHON_DISPATCH_TOKEN`). Tags were pushed by hand (latest `v0.7.0`).
- xcl PR checks: `go.yml` (build, test, buf, minimum Go, libxcl, Windows) and `examples.yml`. No binding is tested on xcl PRs.
- xcl-python CI (`xcl-python:.github/workflows/ci.yml`) tests against xcl at `xcl-ref.txt` (`v0.7.0`) on 20 OS/Python combinations, Alpine and the in-process route.
- xcl-python release (`xcl-python:.github/workflows/release.yml`) runs only on the xcl dispatch (or manual), builds from xcl-python's default branch, sets the package version to the xcl tag, and checks the engine reports that tag; the client enforces engine == package version (`xcl-python:xcl/_client.py:76-91`).
- Neither repo has release labels, branch protection, rulesets or auto-merge enabled. xcl-python has no tags; PyPI has 0.5.3, 0.6.0, 0.7.0.

## Per-Task Technical Notes

### Task: Release decision command

- `cmd/xcl-release/main.go` (new) — tiny CLI with three subcommands, printing plain output for workflows:
  - `labels <label>...`: exits 0 and prints the chosen size when exactly one of `major|minor|patch|no-release` is present; exits 1 with a message naming none/several otherwise.
  - `next --size <size> <tag>...`: prints the next tag (`vX.Y.Z`); prints nothing and exits 0 for `no-release`; exits 1 with "no version tags; push a seed tag" when no `vMAJOR.MINOR.PATCH` tag is given.
  - `bump <from-tag> <to-tag>`: prints `major|minor|patch`.
- `internal/release/labels.go` (new) — `ReleaseSize` type and `SizeFromLabels([]string) (ReleaseSize, error)`; errors are a sentinel `ErrNoReleaseLabel`/`ErrSeveralReleaseLabels`, wrapped with the labels found (sentinel-and-detail style).
- `internal/release/version.go` (new) — parse `vMAJOR.MINOR.PATCH` only (ignore pre-release and other tags); `Latest(tags)`, `Next(latest, size)` with `major` at 0.x → `1.0.0`; `Bump(from, to)`.
- `internal/release/labels_test.go`, `internal/release/version_test.go` (new) — testify `require`, one behaviour per test function, positive and negative cases separate, no tables: one label each size; none; two; only unrelated labels; patch/minor/major from v0.7.0; major above 1.0 (v1.2.3 → v2.0.0); ignores `v0.8.0-rc1` and `docs-1`; no tags → error; bump major/minor/patch.
- `go.yml` already runs `go test ./...` so the tests run in CI with no workflow change.
- Standard library only (no semver dependency).

**Complexity:** Low  **Token estimate:** ~15k  **Agent strategy:** single agent, sequential.

### Task: Shared release-label check for xcl

- `.github/workflows/release-label-check.yml` (new, `on: workflow_call`) — one job `release-label`: checks out `jumppad-labs/xcl` at `main` (explicit `repository:`), sets up Go 1.27.0, runs `go run ./cmd/xcl-release labels` with the PR's label names from `github.event.pull_request.labels.*.name` (the caller's event context, as `toJSON` piped through `jq -r '.[]'`).
- `.github/workflows/release-label.yml` (new) — `on: pull_request: types: [opened, reopened, synchronize, labeled, unlabeled]` on `main`; one job `uses: ./.github/workflows/release-label-check.yml`. Check name seen by protection: `release-label / release-label`.
- Note: the reusable workflow must reference xcl at `main` so other repos get one implementation.

**Complexity:** Low  **Token estimate:** ~8k  **Agent strategy:** single agent.

### Task: Release-label check on xcl-python

- `xcl-python:.github/workflows/release-label.yml` (new) — same triggers as xcl's; job `release-label` with `uses: jumppad-labs/xcl/.github/workflows/release-label-check.yml@main`.

**Complexity:** Low  **Token estimate:** ~3k  **Agent strategy:** single agent.

### Task: Labels and label rule on xcl

Follow step 1 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does:
- Create labels on jumppad-labs/xcl: `major`, `minor`, `patch`, `no-release` (e.g. `gh label create major --repo jumppad-labs/xcl --description "Release: breaking (1.0.0 from 0.x)"`, likewise for the others).
- Branch protection (or a ruleset) on `main`: require PRs, require status checks `release-label / release-label`, `build`, `Build on minimum supported Go`, `Build and smoke-test libxcl`, `Test xcl-core on Windows` and the examples matrix checks currently passing; require branches to be up to date is optional.
Check: open a test PR with no label → blocked; add `no-release` → mergeable.

### Task: Labels and label rule on xcl-python

Follow step 1 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does: same four labels on jumppad-labs/xcl-python; protect `main` requiring `release-label / release-label` and `Check the package`.
Check: test PR blocked without a label.

### Task: Reusable Python test suite

- `xcl-python:.github/workflows/test.yml` (new, `on: workflow_call` with inputs `python-ref` (string, default `main`) and `xcl-ref` (string, default empty = read `xcl-ref.txt`)) — move the jobs `test`, `test-musl`, `check-package` from `xcl-python:.github/workflows/ci.yml:36-180` unchanged in substance. Every `actions/checkout` of xcl-python gets `repository: jumppad-labs/xcl-python` and `ref: ${{ inputs.python-ref }}` (a called workflow otherwise checks out the caller). The "Read the xcl ref" steps (`ci.yml:49-51`, `ci.yml:132-135`) use `inputs.xcl-ref` when non-empty, else the file.
- Same file — final job `result`: `needs: [test, test-musl, check-package]`, `if: always()`, fails unless every needed job's result is `success` (`contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled') || contains(needs.*.result, 'skipped')` → exit 1).
- `xcl-python:.github/workflows/ci.yml` — replace the jobs with one job `test` that `uses: ./.github/workflows/test.yml` with `python-ref: ${{ github.event.pull_request.head.sha || github.sha }}`. Keep triggers. Check name: `test / result`.
- `GITHUB_TOKEN` usage (`github.token` for plugin release downloads) works in called workflows; no secrets needed.

**Complexity:** Medium  **Token estimate:** ~20k  **Agent strategy:** single agent; verify by pushing a branch and watching all jobs.

### Task: Bindings check on xcl pull requests

- `.github/workflows/bindings.yml` (new) — `on: pull_request: branches: [main], types: [opened, reopened, synchronize, labeled, unlabeled]`; `concurrency: group: bindings-${{ github.event.pull_request.number }}, cancel-in-progress: true`.
  - job `python`: `if: ${{ !contains(github.event.pull_request.labels.*.name, 'breaks-bindings') }}`, `uses: jumppad-labs/xcl-python/.github/workflows/test.yml@main` with `python-ref: main`, `xcl-ref: ${{ github.event.pull_request.head.sha }}`.
  - job `bindings` (the gate): `needs: [python]`, `if: always()`; passes when `needs.python.result == 'success'`, or when the PR has `breaks-bindings` (then `python` is skipped by design); fails otherwise. Never skipped.
- Adding a language later = one more called job and one more entry in the gate's `needs`.
- Assumption to verify on the first fork PR: `actions/checkout` of `jumppad-labs/xcl` at a fork PR head SHA works (refs/pull are fetchable).

**Complexity:** Medium  **Token estimate:** ~10k  **Agent strategy:** single agent; verify with a harmless PR and a deliberately breaking draft PR.

### Task: Require the bindings gate on xcl

Follow step 2 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does: create label `breaks-bindings` on xcl ("Merge although a language binding fails; fix it in the binding's engine update"); add required check `bindings` to xcl `main`. Labels can only be applied by users with triage or higher, which is the "maintainer" boundary.
Check: breaking draft PR blocked; mergeable after adding the label.

### Task: Require the test result on xcl-python

Follow step 2 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does: add required check `test / result` to xcl-python `main`.
Check: a PR with a failing test can't merge.

### Task: Release App on xcl

Follow step 3 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does:
- Create the GitHub App `xcl release bot` owned by jumppad-labs, no webhook. Repository permissions: Contents read/write, Pull requests read/write, Workflows read/write (lets it push branches touching workflow-adjacent files), Metadata read.
- Install it on jumppad-labs/xcl and jumppad-labs/xcl-python only.
- Generate a private key; add secrets on xcl: `RELEASE_APP_ID`, `RELEASE_APP_PRIVATE_KEY`.
- If branch protection restricts tag creation, allow the App.
Check: `gh api repos/jumppad-labs/xcl/actions/secrets --jq '.secrets[].name'` lists both secrets; the App shows both repos under its installation.

### Task: Release App secrets on xcl-python

Follow step 3 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does: add `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` secrets on jumppad-labs/xcl-python (same values).
Check: secrets listed on the repo.

### Task: Auto-tag on merge

- `.github/workflows/auto-tag.yml` (new, `on: workflow_call`, `secrets: RELEASE_APP_ID, RELEASE_APP_PRIVATE_KEY`, outputs `tag`, `size`):
  - mint a token with `actions/create-github-app-token@v1` (`app-id`, `private-key`, `owner: jumppad-labs`, `repositories: ${{ github.event.repository.name }}`);
  - find the merged PR: `gh api repos/$GITHUB_REPOSITORY/commits/$GITHUB_SHA/pulls --jq '.[0]'`; no PR (direct push) → exit 0 with no tag;
  - checkout xcl at `main` (for the command) into a subfolder, and the caller repo with `fetch-depth: 0` and `fetch-tags: true`;
  - `size=$(go run ./cmd/xcl-release labels <pr labels>)`; `no-release` → exit 0;
  - `tag=$(go run ./cmd/xcl-release next --size $size $(git tag -l 'v*'))`;
  - push the tag with the App token: `gh api repos/$GITHUB_REPOSITORY/git/refs -f ref=refs/tags/$tag -f sha=$GITHUB_SHA` using `GH_TOKEN=<app token>` (an App-created ref fires `push: tags` workflows).
- `.github/workflows/tag.yml` (new) — `on: push: branches: [main]`; `concurrency: group: auto-tag, cancel-in-progress: false` (serialises merges so versions never collide); job `uses: ./.github/workflows/auto-tag.yml`, `secrets: inherit`.
- `.github/workflows/release.yml:31-34` — unchanged trigger `push: tags: ['v*']`; the App-created tag starts it.

**Complexity:** Medium  **Token estimate:** ~15k  **Agent strategy:** single agent; first validate with a `no-release` merge, then a `patch` merge.

### Task: Python checks against its pinned engine

- `xcl-python:xcl/_version.py:1-8` — add `__engine_version__ = "v0.0.0-dev"` (development value) with a docstring line: set from `xcl-ref.txt` at release.
- `xcl-python:xcl/_client.py:76-91` — compare `normalise_version(response.version)` with `normalise_version(__engine_version__)`; message: "the xcl engine reports version X, but this xcl-lang package was built for engine Y"; `EngineVersionError(package=__version__, engine=response.version, expected=__engine_version__)` — keep existing fields, add `expected`.
- `xcl-python:xcl/_errors.py:299` — `EngineVersionError` accepts and exposes `expected` in `details`.
- `xcl-python:Makefile:8-10,29,37` — stamp `make engine` and `make libxcl` with `ENGINE_VERSION` read from `__engine_version__` instead of `v$(VERSION)`.
- `xcl-python:tests/fakes.py:30` — fake engine reports `xcl._version.__engine_version__`.
- `xcl-python:tests/unit/test_client.py:15-111` — update: accepted when engine == pin; separate test: rejected when engine == package version but != pin (error names expected and actual); keep the `check_version=False` test.
- `xcl-python:tests/integration/test_engine_process.py:707` — assert on the engine pin in the message.
- `xcl-python:.github/scripts/install_check.py:1-60` — compare `xcl.engine_version()` with env `ENGINE_TAG` instead of `TAG`.

**Complexity:** Low  **Token estimate:** ~12k  **Agent strategy:** single agent; run `make engine plugins test` locally.

### Task: Seed tag for xcl-python

Follow step 4 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does: identify the xcl-python commit the 0.7.0 release built (the run of `release.yml` dispatched for xcl `v0.7.0`; its checkout SHA is in the run log; expected `a7f4993` or `d39b4de`) and push `v0.7.0` there: `git tag v0.7.0 <sha> && git push origin v0.7.0`. This must happen **before** xcl-python's release workflow triggers on its own tags, or it would immediately rebuild 0.7.0 — so push it while the release workflow still listens only to the dispatch.
Check: `git ls-remote --tags origin` shows exactly `v0.7.0`; no new release run started.

### Task: Python releases from its own tags

- `xcl-python:.github/workflows/release.yml:19-27` — triggers: `push: tags: ['v*']` and `workflow_dispatch` (tag input). Remove `repository_dispatch: xcl-release` (moves to the engine-bump workflow).
- `xcl-python:.github/workflows/release.yml:34-53` (settings job) — `tag`/`version` from `github.ref_name` (or input); new output `engine` = contents of `xcl-ref.txt` at that tag (checkout at the tag, `tr -d '[:space:]'`), with `engine_version=${engine#v}`.
- `release.yml:57-185` (wheels, wheels-musl, sdist) — checkout xcl-python at `ref: ${{ needs.settings.outputs.tag }}`; xcl-src checkout and engine download at `engine` tag, not the package tag (`gh release download "$ENGINE"` with `xcl-core_${ENGINE_VERSION}_*` names); "Set the version" also writes `__engine_version__ = "$ENGINE"`; "Check the engine reports" compares with `$ENGINE`.
- `xcl-python:.github/scripts/musl-wheel.sh` — same engine/package split (pass `ENGINE`, `ENGINE_VERSION`).
- `release.yml` install-check job — pass `ENGINE_TAG`.
- `release.yml` new final job `github-release` (needs publish): `gh release create "$TAG" --generate-notes --notes "Bundles the xcl engine $ENGINE (https://github.com/jumppad-labs/xcl/releases/tag/$ENGINE)."` (`contents: write`).
- `xcl-python:.github/workflows/tag.yml` (new) — `on: push: branches: [main]`, concurrency `auto-tag`, `uses: jumppad-labs/xcl/.github/workflows/auto-tag.yml@main`, `secrets: inherit`.
- `xcl:.github/workflows/release.yml:200-218` — keep the dispatch (consumed by the engine-bump workflow from the next task on); until then nothing listens to it, which is the intended "xcl release no longer publishes Python" state.
- PyPI trusted publisher stays bound to `release.yml` + environment `pypi` (top-level workflow unchanged in name).

**Complexity:** Medium  **Token estimate:** ~20k  **Agent strategy:** single agent; dry-run with `workflow_dispatch` on a test tag is not possible without publishing, so validate the first real `patch` merge and watch the run.

### Task: Engine release tells bindings its size

- `xcl:.github/workflows/release.yml:200-218` (dispatch job) — check out with `fetch-depth: 0`, compute `previous` = highest `v*` tag below `$TAG`, `size=$(go run ./cmd/xcl-release bump $previous $TAG)`, send `-F "client_payload[bump]=$size"` alongside `client_payload[tag]`. If there is no previous tag, send `minor`.
- No change to build, sign, attach or `--generate-notes` steps (`release.yml:73-196`).

**Complexity:** Low  **Token estimate:** ~6k  **Agent strategy:** single agent.

### Task: Engine update pull requests in xcl-python

- `xcl-python:.github/workflows/engine-bump.yml` (new) — `on: repository_dispatch: types: [xcl-release]` and `workflow_dispatch` (inputs `tag`, `bump`).
  - validate `tag` starts with `v` and `bump` ∈ major|minor|patch;
  - App token via `actions/create-github-app-token@v1` (repo `xcl-python`);
  - checkout with the App token; branch `engine/$TAG` (force-reset from `main` so re-runs update the same branch); write `$TAG` to `xcl-ref.txt`; commit "Use xcl engine $TAG" as the App's bot identity; `git push --force-with-lease origin engine/$TAG`;
  - if no open PR for the branch: `gh pr create --base main --head engine/$TAG --title "Use xcl engine $TAG" --body "<link to xcl release; generated>" --label "$BUMP"`; else ensure the label;
  - `gh pr merge --auto --squash engine/$TAG`.
- A red PR simply stays open; the maintainer fixes it on the same branch (as the override flow intends).

**Complexity:** Medium  **Token estimate:** ~12k  **Agent strategy:** single agent; validate with `workflow_dispatch` against an existing tag on a throwaway run, then the first real xcl patch.

### Task: Allow auto-merge on xcl-python

Follow step 5 of `docs/release-setup.md` (xcl), which holds the exact commands and checks.

What the person does: Settings → General → Pull Requests → "Allow auto-merge" on jumppad-labs/xcl-python (`gh api -X PATCH repos/jumppad-labs/xcl-python -F allow_auto_merge=true`).
Check: `gh api repos/jumppad-labs/xcl-python --jq .allow_auto_merge` is `true`.

### Task: Document the release process in xcl

- `README.md:221-229` (after "Other languages", before "Contributing") — new `## Releases` section: the four labels and when to use each (pre-1.0 breaking → `minor`; `major` = 1.0.0); merging tags and releases automatically; the bindings check and `breaks-bindings`; independent language versions and engine pins; engine releases opening auto-merging update PRs, with the worked example (Python v0.1.6 on engine v0.1.2 …); what a new language provides (reusable test suite, three thin workflows, a dispatch target).
- `README.md:221-229` "Other languages" — replace "same version" wording if present with a pointer to Releases.

**Complexity:** Low  **Token estimate:** ~8k  **Agent strategy:** single agent.

### Task: Document releases and the engine pin in xcl-python

- `xcl-python:README.md:16-40` (Install) — replace "The package version is the xcl release it bundles…" with the pin description and `xcl.engine_version()`.
- `xcl-python:README.md:263-304` (Development) — new `### Releases and the engine pin` subsection: labels, own version line, `xcl-ref.txt` as the pin, automated engine update PRs, link to xcl's README Releases section; note `__engine_version__` development value.

**Complexity:** Low  **Token estimate:** ~6k  **Agent strategy:** single agent.

## Testing Strategy

- Release decision command: Go unit tests beside the code in `internal/release`, run by the existing `go.yml` `go test ./...` (testify `require`, one behaviour per test, no tables).
- Python engine-pin check: xcl-python unit tests in `tests/unit/test_client.py`, fakes in `tests/fakes.py`; the integration and parity suites run unchanged.
- Workflows: never tested by reading their files (project convention). Each task's acceptance is observed live on the repos, in rollout order: label check → bindings gate → auto-tag → Python release → engine bump. These and the spec's success metrics are manual items for the implementation test plan.

## Project References

- Spec: `20261014052450-0b131707-ci-release-automation`.
- Downstream spec (parked): `20261014043756-e853a081-python-plain-dataclasses`.
- Knowledge: `xcl:learnings/releases-tagged-by-hand.md`; conventions `testing-and-mocking.md`.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`.
- Design documents: none.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

- Order matters: the seed tag `v0.7.0` on xcl-python must exist before its release workflow listens to its own tags; each check must have run once before it is made required; the App and secrets must exist before auto-tag and engine-bump merge.
- From the Python-release task onward, xcl's dispatch has no listener until the engine-bump task lands; an xcl release in that window does not update Python (intended, briefly).

## Performance Considerations

- Every xcl PR runs the full Python matrix (~27 jobs); label changes other than `breaks-bindings` re-run it. Concurrency cancels superseded runs per PR.
