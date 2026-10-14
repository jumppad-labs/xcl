### Chosen direction: shared reusable workflows + App-token tag push (architecture)
- **Decision**: four pieces — reusable release-label check and auto-tag in xcl, xcl `bindings.yml` calling xcl-python's reusable `test.yml` with an always-run gate honouring the `breaks-bindings` override label, and an App-token engine-bump PR with auto-merge in xcl-python. Tags are pushed with a GitHub App token so the existing tag-triggered `release.yml` files run unchanged as top-level workflows.
- **Rationale**: reuses the existing build/sign/publish pipelines; keeps PyPI trusted publishing bound to `release.yml`; results are ordinary PR checks; one line per new language.
- **Rejected**: calling release.yml via workflow_call (PyPI OIDC claim names the caller workflow); dispatch + posted status; skippable gate job.

### Override label named `breaks-bindings` (architecture)
- **Decision**: the override label is `breaks-bindings`.
- **Rationale**: the spec left the name to the plan; it says what the PR does, not what the maintainer does.
- **Rejected**: `override`, `skip-bindings` (less specific).

### Label changes re-run the binding suite (architecture)
- **Decision**: the bindings workflow listens to label events; non-override label changes re-run the suite.
- **Rationale**: the gate must never be skipped (a skipped job passes required checks); re-running is correct and labels are usually set once.
- **Rejected**: a gate that replays the last suite result via the API (more moving parts).

### Conventions selected (architecture)
- **Decision**: apply no-repo-file-tests, testing-and-mocking, releases-tagged-by-hand; drop Go code style, DAG, parser, errors-package, testutil and state conventions.
- **Rationale**: the work is workflow YAML, README text and a small Python change; the dropped ones govern Go library code this plan doesn't touch.
- **Rejected**: n/a.

### Seed xcl-python with a v0.7.0 tag (discovery)
- **Decision**: tag the xcl-python commit that shipped 0.7.0 as `v0.7.0` so label-driven bumps start from there (first release 0.7.1 or 0.8.0).
- **Rationale**: xcl-python has no tags; PyPI already holds 0.7.0; continuing the same number line satisfies the "higher than every PyPI version" constraint with the smallest jump.
- **Rejected**: restarting at 1.0.0 (implies stability not claimed); 0.8.0 seed (skips numbers for no reason).

### GitHub App token for cross-repo automation (discovery)
- **Decision**: one GitHub App installed on xcl and xcl-python, used for the bump PR, its auto-merge, and anything that must trigger further workflows.
- **Rationale**: `GITHUB_TOKEN` events don't trigger workflows; an App token is scoped, not tied to a person, and rotates automatically.
- **Rejected**: personal access token (tied to a user, long-lived).

### Engine pin stamped as a second version value (data_structures)
- **Decision**: the version module gains `__engine_version__`, set from `xcl-ref.txt` at release and a fixed development value otherwise; `make engine` stamps dev engines with the same value.
- **Rationale**: keeps the runtime check exact while decoupling it from the package version; dev and CI runs still pass.
- **Rejected**: comparing major.minor only (loses the exact pin the spec requires); reading `xcl-ref.txt` at runtime (not shipped in the wheel).

### Label and version logic as a small tested Go command in xcl (testing_approach)
- **Decision**: the "exactly one release label" decision and next-version computation live in a small Go command in xcl (under `cmd/`, logic in `internal/`), unit-tested, and run by the shared workflows.
- **Rationale**: the testing convention requires unit tests for business logic and forbids tests that read workflow files; inline shell in YAML could not be tested either way.
- **Rejected**: inline shell/`gh` scripts (untestable); a third-party semver/tag action (adds an unpinned-by-default dependency and still leaves the label rule untested).

### Settings changes are human tasks (tasks)
- **Decision**: labels, branch protection, auto-merge setting, App creation/secrets and the seed tag are `human` tasks, one per repo.
- **Rationale**: they need repository/organisation admin access and act outside the code.
- **Rejected**: scripting them as agent tasks via `gh api` (needs admin tokens an agent shouldn't hold).
