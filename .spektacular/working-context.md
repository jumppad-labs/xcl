# Working context: ci-release-automation

Spec: `20261014052450-0b131707-ci-release-automation`. Started 2026-10-14 after parking
`20261014043756-e853a081-python-plain-dataclasses` (see its work folder, PARKED.md). The user wants
to build and deploy this CI/release automation first, then use the parked Python spec as the first
real test of it.

Repos involved: xcl (`/home/nicj/code/github.com/jumppad-labs/xcl`, engine + Go library) and
xcl-python (`/home/nicj/code/github.com/jumppad-labs/xcl-python`), with more language bindings
expected ("we are going to have many languages in the future").
`spektacular repo list` currently errors (xcl-vscode location missing on disk).

## Problem

- Today: package version == xcl tag; Python checks the engine reports exactly its own version
  (`xcl-python/xcl/_client.py:76`). Tagging xcl dispatches xcl-python's release, which builds
  whatever is on xcl-python `main`. A Python-only change can only ship via an xcl tag, which (with
  many languages) re-releases every language. User: "if we have to make changes to python. This
  causes a release to every single language."
- xcl's CI never runs the Python suite; an xcl change that breaks Python is found only when
  `xcl-ref.txt` is bumped or at release time, after the xcl tag exists.
- Tagging is manual.

## Decisions (user confirmed)

- **PR checks**: xcl PRs run every dependent language's build/test suite against the PR's head
  SHA; "only if everything is green can we merge" (required checks). Direction: each binding
  publishes a reusable workflow taking the xcl ref; xcl's PR workflow calls it.
- **Auto-tagging**: every PR gets one label `major` / `minor` / `patch`; on merge the next semver
  tag is pushed automatically and everything downstream is automatic. `major` = 1.0.0. An override
  label for maintenance/docs PRs that does not release.
- **Independent binding versions** (replaces strict lockstep): each binding repo has its own semver
  and the same label-driven auto-tagging. `xcl-ref.txt` pins the engine the binding tests against
  and bundles; the engine check compares against the pin, not the package version.
- **Engine release propagation**: an xcl tag opens a bump PR in each binding moving the pin to the
  new tag; it runs the binding's CI and auto-merges when green, with a label mirroring the engine
  bump (engine patch -> binding patch, engine minor -> binding minor); that merge auto-tags and
  releases the binding.
- User's worked example (confirmed, numbers corrected): Python v0.1.6 on engine v0.1.2. Python-only
  fix with `patch` -> Python v0.1.7 on engine v0.1.2, nothing else releases. Engine fix on xcl with
  `patch`: PR runs xcl tests + Python suite against PR SHA; merge -> xcl v0.1.3 -> bump PR in
  xcl-python -> green, auto-merge -> Python v0.1.8 bundling engine v0.1.3.
- **Documentation**: "we need this process documented in the README.md too".

## Interview answers

- Coordinated breaking changes: **manual override label** on the xcl PR (not same-branch-name).
- Skip label: **`no-release`**.
- Docs: **full process in xcl README + short pointer section in each binding README**.
- Other repos (plugins, website, vscode): **none for now**; only xcl and xcl-python.

## Gotchas to carry into technical approach

- Tags pushed with GITHUB_TOKEN don't trigger other workflows: use a GitHub App token or call the
  release workflow directly.
- Release notes from changelog entries since the last tag.
- Pre-1.0: `major` label means 1.0.0.
