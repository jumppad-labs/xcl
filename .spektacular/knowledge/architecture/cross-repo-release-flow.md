---
tags: [releases, ci, cross-repo, bindings, xcl-python, planning, versioning]
---

# How changes ship across xcl and its language bindings

Releases are driven by pull request labels; nobody tags by hand. This holds for xcl and every
language binding (today xcl-python). Use it when planning the order work lands in.

## The moving parts

- **Release label.** Every PR carries exactly one of `patch`, `minor`, `major` (below 1.0 this
  releases 1.0.0) or `no-release`; a required check enforces it. Before 1.0 a breaking change is
  `minor`.
- **Merge tags.** On merge to `main`, `tag.yml` (calling xcl's shared `auto-tag.yml`) pushes the
  next `v*` tag, which starts that repo's `release.yml`. `no-release` tags nothing.
- **Bindings check (xcl only).** Every xcl PR runs each binding's full suite against the PR's own
  commit, but always with the binding's `main`. The required `bindings` gate fails if any binding
  fails. The `breaks-bindings` label skips every binding suite and passes the gate.
- **Engine pin.** Each binding has its own version line and pins the xcl release it tests,
  builds and bundles in `xcl-ref.txt`. The binding's CI builds the engine from xcl source at that
  ref, so the pin can name any xcl ref while testing, but a release downloads the signed engine
  from the xcl GitHub release, so on `main` it must be a released tag.
- **Engine bump PR.** Each xcl release dispatches every binding. The binding's `engine-bump.yml`
  opens "Use xcl engine vX" on branch `engine/vX`, labelled with the engine's bump size, with
  auto-merge on. Green, it merges itself and releases the binding; red, it stays open and nothing
  is published.

## What a change needs, by where it lives

- **Binding-only change** (API, docs, tests reading only what the pinned xcl already has): one PR
  in the binding; its label releases the binding alone. No xcl work.
- **Binding change that needs something new in xcl** (an engine feature, a new parity fixture,
  a proto change): the binding's tests read xcl at the pin, so xcl must release first. Land and
  release the xcl change, let the bump PR move the pin, then land the binding change.
- **Additive xcl feature**: the bindings check stays green, the bump PR merges itself, and each
  binding ships the new engine without exposing the feature. Each binding adds the API later in
  its own PR.
- **Breaking xcl change**: see the next section.

Note that the parity fixtures' `types.json` is the shared contract: Go registers it with
`registry.TypeFromSchema`, and Python's own dataclasses (in xcl-python `tests/parity/types.py`)
must derive it. Changing a Python parity class needs no xcl change unless `types.json` changes.

## A breaking xcl change

1. The xcl PR's `bindings` gate is red; it can't merge.
2. Optionally prove the fix first (stacking, below).
3. A maintainer adds `breaks-bindings` and merges with the right size (`minor` before 1.0).
4. xcl releases; each binding gets a bump PR, which goes red.
5. Each binding is fixed on its own bump PR's branch (`engine/vX`). When green it auto-merges
   and releases. Until then the binding stays on its old pin and can keep shipping its own fixes.

**Fixing some languages before others.** Downstream, every binding's bump PR is independent, so
languages can be fixed and released in any order, and a language whose suite still passes
releases at once. Upstream the override is all or nothing: `breaks-bindings` skips every
binding's suite, including ones the change doesn't break, so a break in one language hides any
breakage in the others on that PR. Check the last red run's per-language results before applying
it.

## Stacking an engine PR with a binding PR

To prove an xcl change and its binding fix work together before either merges:

1. Open the xcl PR (branch `feature-x`). Its bindings gate is red, since it tests binding `main`.
2. Open the binding PR with the fix and `xcl-ref.txt` set to `feature-x` (or the xcl PR's head
   SHA). Its CI builds the engine from that ref, so a green run proves the pair. Don't merge it
   in this state: on `main` the pin must be a released tag, and nothing enforces that yet.
3. Merge the xcl PR with `breaks-bindings`. xcl releases vX and the bump PR opens red.
4. Move the fix onto the bump PR: push the binding PR's commits to `engine/vX` (keeping
   `xcl-ref.txt` at `vX`) and close the binding PR, or set the binding PR's pin to `vX`, merge
   it, and close the bump PR. Either way, check the label: it should reflect the binding's own
   change, not just mirror the engine.

## Sharp edges

- Re-running `engine-bump.yml` for the same tag resets `engine/vX` from `main` and force-pushes,
  discarding anything a person pushed there.
- A second xcl release while a bump PR is still red opens a new bump PR from `main`, without the
  fix. Fix the newest one and close the older.
- The bump PR's label mirrors the engine. If the binding fix changes the binding's own API more
  than that, change the label before it merges.
- A PR opened with a label already set starts two Bindings runs; the first is cancelled and its
  jobs show as 1-second failures. The later run's `bindings` gate is the one that counts.

The one-off repository setup (labels, rules, the release bot App, auto-merge) is in xcl's
`docs/release-setup.md`; the contributor view is the Releases section of xcl's README.
