## Alternatives considered and rejected

- **Dispatch the binding suite to xcl-python and post a commit status back** (repository_dispatch from the xcl PR, xcl-python posts a status to the xcl SHA). Rejected: needs a cross-repo token with `statuses: write` on xcl, results live in another repo's Actions tab, and a status a bot posts is easy to forge or forget. A reusable workflow called from xcl's PR runs as a normal job in xcl's own run. Ref: GitHub docs, "Reusing workflows" (public repos can call each other's reusable workflows).
- **Same-branch-name coordination for breaking engine PRs.** Rejected by the user in the spec (Non-Goals); override label instead.
- **Push the auto-tag with the default `GITHUB_TOKEN` and let `on: push: tags` start release.yml.** Rejected: events created by `GITHUB_TOKEN` do not start new workflow runs (GitHub docs, "Triggering a workflow from a workflow"), so the release would never run. Chosen instead: the tagging workflow calls `release.yml` as a reusable workflow in the same run (`workflow_call`), which needs no extra token for xcl itself.
- **Create the xcl-python engine-bump PR with `GITHUB_TOKEN`.** Rejected: a PR opened by `GITHUB_TOKEN` does not trigger `pull_request` workflows, so its CI would never run; auto-merge enabled by `GITHUB_TOKEN` produces a merge push that does not trigger the tagging workflow either. A GitHub App installation token (or a PAT) is required for the bump PR and its auto-merge.
- **Use GitHub ruleset bypass actors as the "override".** Rejected: the spec requires an override *label* visible on the PR; a bypass merges past every check silently and is not recorded on the PR.
- **Gate job that is skipped on unrelated label events.** Rejected: a skipped job reports success to required status checks, so adding e.g. `patch` after a red binding run would turn the gate green. Any gate design must never be skipped; on events that do not re-run the suite it must reproduce the suite's last result or re-run it.
- **Strict lockstep (binding version = xcl tag).** Rejected by the spec (independent binding versions).
- **Release notes from the changelog store.** Not chosen as the primary source: `gh release create --generate-notes` (already used, `xcl:.github/workflows/release.yml` "Attach the assets") lists PRs merged since the previous tag, which is exactly the requirement.

## Chosen approach — evidence

- xcl release already creates the GitHub release with `--generate-notes` and dispatches xcl-python with `XCL_PYTHON_DISPATCH_TOKEN`: `xcl:.github/workflows/release.yml:206-218`. Only its trigger (add `workflow_call`) and the dispatch payload's meaning change.
- xcl-python CI already parameterises the xcl ref from `xcl-ref.txt` and builds the engine from source: `xcl-python:.github/workflows/ci.yml:47-74`. Making the ref a `workflow_call` input (default: the file) turns it into the cross-repo check with no change to what it tests.
- xcl-python release already verifies signed engine archives and builds the matrix: `xcl-python:.github/workflows/release.yml:57-185`; only where `TAG`/`VERSION` come from changes (package version from xcl-python's own tag; engine tag from `xcl-ref.txt`).
- Runtime check compares engine version to the package version: `xcl-python:xcl/_client.py:76-91` (`normalise_version(response.version) != normalise_version(__version__)`). Change to compare against a pinned engine version stamped at build time.
- `xcl.engine_version()` already exists: `xcl-python:xcl/__init__.py:118`.
- Version source of the package: `xcl-python:pyproject.toml:53-54` (hatch reads `xcl/_version.py`).
- Repos today: no labels beyond GitHub defaults, no branch protection, no rulesets, `allow_auto_merge: false` on both xcl and xcl-python (checked with `gh api`, 2026-10-14).
- Tags: xcl latest `v0.7.0`; xcl-python has **no tags**; PyPI `xcl-lang` has 0.5.3, 0.6.0, 0.7.0. Seeding xcl-python with tag `v0.7.0` on the commit that shipped 0.7.0 lets the first label-driven release be ≥ 0.7.1, satisfying the "above every existing PyPI version" constraint.

## Files examined

- `xcl:.github/workflows/release.yml:1-218` — tag-triggered build of 8 `xcl-core` archives, GPG-signed checksums, `gh release create --generate-notes`, then repository_dispatch `xcl-release` to xcl-python with `XCL_PYTHON_DISPATCH_TOKEN`. Has a `workflow_dispatch` dry-run mode (publish=false).
- `xcl:.github/workflows/go.yml:1-160` — PR/push checks: build (3 OSes), `go test -race`, buf lint/breaking, minimum-Go build, libxcl smoke test, xcl-core on Windows.
- `xcl:.github/workflows/examples.yml` — PR/push matrix over examples.
- `xcl:.github/workflows/libxcl-build.yml` — manual libxcl build matrix; unaffected.
- `xcl-python:.github/workflows/ci.yml:1-180` — 5 OS x 4 Python matrix + in-process route on ubuntu + 2 Alpine jobs + package check; xcl checked out at `xcl-ref.txt`; strict mode; uses `github.token` only.
- `xcl-python:.github/workflows/release.yml:1-270` — triggered by `repository_dispatch: xcl-release` or manual tag input; checks out xcl-python default branch (no ref) and xcl at the tag; sets `_version.py` from the tag; asserts `engine_version() == TAG`; parity tests on each wheel; sdist; PyPI trusted publishing (`environment: pypi`); install check from PyPI.
- `xcl-python:.github/scripts/install_check.py:1-60` — asserts engine reports `TAG`; uses `xcl.Resource`/`@xcl.entity` (touched later by the parked dataclasses spec, not here).
- `xcl-python:xcl/_client.py:28-91` — `normalise_version`, `_ensure_version` exact-match check raising `EngineVersionError`.
- `xcl-python:xcl/_version.py:1-8` — `__version__ = "0.5.0.dev0"`, set from tag at release.
- `xcl-python:tests/unit/test_client.py:15-111`, `tests/fakes.py:30`, `tests/integration/test_engine_process.py:707` — tests that assume engine version == package version; must move to the pinned engine version.
- `xcl-python:xcl-ref.txt` — `v0.7.0`.
- `xcl-python:README.md` "Development" section — documents `XCL_SRC`, CI strict mode; release process section to add.
- `xcl:README.md` "Other languages"/"Contributing" — where the release process section goes.

## External references

- GitHub Docs, "Reusing workflows" — `workflow_call`, cross-repo `uses: owner/repo/.github/workflows/x.yml@ref`; in a called workflow `actions/checkout` with no `repository` checks out the **caller** repo, so the binding's workflow must name its own repository explicitly.
- GitHub Docs, "Triggering a workflow from a workflow" — `GITHUB_TOKEN` events (tag push, PR open, auto-merge push) don't start workflows; use a GitHub App token.
- GitHub Docs, "About protected branches / rulesets" — required status checks; a skipped job counts as passing.
- `actions/create-github-app-token` — mint an installation token for the App in a workflow.
- GitHub Docs, "Automatically merging a pull request" — repo setting `allow_auto_merge` must be on; needs at least one required check to wait for.

## Prior plans / specs consulted

- Knowledge `xcl:learnings/releases-tagged-by-hand.md` — tags were pushed by hand; "before 1.0, breaking changes bump the minor"; "treat cutting a tag as a task for a person, unless a release workflow does it" — this plan is that workflow.
- Plans `20261011211634-a8895873-python-out-of-process-engine`, `20261011232013-da18c586-python-release-hardening` (names only, via `plan file list`) — origin of the current xcl-core release and dispatch; not re-read, the shipped workflows are authoritative.
- Parked spec `20261014043756-e853a081-python-plain-dataclasses` — the first real change to ship through this process (success metric).

## Open assumptions

- The maintainer can create a GitHub App (or PAT) for jumppad-labs and add its credentials as secrets on xcl and xcl-python. If not possible, STOP: auto-merging bump PRs cannot trigger CI or tagging otherwise.
- `XCL_PYTHON_DISPATCH_TOKEN` stays valid for repository_dispatch from xcl to xcl-python.
- PyPI trusted publishing for xcl-lang is bound to the xcl-python `release.yml` workflow file and `pypi` environment; if the release is invoked as a reusable workflow from a differently named workflow, the trusted publisher may need updating (PyPI matches the top-level workflow filename). If publishing fails with an OIDC claim mismatch, STOP and ask.
- Python's matrix (≈27 jobs) is acceptable to run on every xcl PR.
- Fork PRs to xcl: a called workflow can check out the PR head SHA from `jumppad-labs/xcl` (refs/pull/*/head are fetchable). Not verified.

## Rehydration cues

- `spektacular spec file read 20261014052450-0b131707-ci-release-automation`
- Read `xcl:.github/workflows/release.yml`, `xcl-python:.github/workflows/ci.yml`, `xcl-python:.github/workflows/release.yml`, `xcl-python:xcl/_client.py:28-91`.
- `gh api repos/jumppad-labs/{xcl,xcl-python} --jq '{allow_auto_merge}'`; `gh label list --repo jumppad-labs/xcl`; `git tag` in both repos; `curl -s https://pypi.org/pypi/xcl-lang/json`.
- Knowledge: `spektacular knowledge read --data '{"tier":"repo","name":"xcl","path":"learnings/releases-tagged-by-hand.md"}'`; convention `testing-and-mocking.md` forbids tests that read CI workflow files.
