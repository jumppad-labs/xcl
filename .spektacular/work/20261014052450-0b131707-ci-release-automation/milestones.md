### Milestone 1: Every pull request declares its release size

**What changes:** Pull requests on xcl and xcl-python have to carry exactly one release label, `major`, `minor`, `patch` or `no-release`, before they can merge. A missing or doubled label shows as a failing required check that names the problem. Nothing is released automatically yet; this milestone only establishes the labels and the rule, so every later piece can rely on them.

**Validation point:** On each repo, a PR with no label is blocked, adding one label unblocks it, and adding a second blocks it again. The label-decision unit tests pass in xcl's CI.

### Milestone 2: Engine pull requests are checked against Python

**What changes:** Every xcl pull request runs the full Python test suite against its own code and shows the result as a check on the PR. A PR that breaks Python can't merge unless a maintainer adds the `breaks-bindings` label. xcl-python's own CI runs the same suite, so the two can't drift.

**Validation point:** A harmless xcl PR shows a green Python check. A deliberately breaking draft PR shows red and can't merge, and turns mergeable once `breaks-bindings` is added. xcl-python PRs still run their full suite.

### Milestone 3: Merging releases automatically, and Python versions independently

**What changes:** Merging a labelled PR tags the next version and releases it, with no manual step, in both repos. `no-release` merges do nothing. Python gets its own version line, starting above everything already on PyPI. Each Python release bundles the engine named by its pin, checks at startup that it is running on that engine, and publishes release notes listing the merged PRs and the bundled engine. A Python-only fix now releases only Python.

**Validation point:** A `patch` PR merged on xcl-python publishes the next Python patch to PyPI, with no xcl tag, and the installed package reports the pinned engine. A `no-release` merge creates no tag. An xcl `patch` merge produces the next xcl tag, a signed release, and generated notes.

### Milestone 4: Engine releases reach Python by themselves, and the process is documented

**What changes:** Each xcl release opens a pull request in xcl-python that moves the engine pin to the new release, labelled to mirror the engine's bump. When its checks pass it merges and releases itself; when they fail it waits for a maintainer. xcl's README describes the whole process, and xcl-python's README explains its labels and engine pin.

**Validation point:** An xcl `patch` release is followed, with no maintainer action, by a merged pin-update PR and a published Python patch bundling the new engine. A pin-update PR forced red stays open and unpublished. Both README sections are reviewed.
