### Milestone 1: Every pull request declares its release size

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
