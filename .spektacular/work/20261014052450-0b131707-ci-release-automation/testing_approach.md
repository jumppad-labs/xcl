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
