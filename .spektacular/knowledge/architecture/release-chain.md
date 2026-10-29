---
tags: [releases, ci, versioning, python, website, engine]
---

# How xcl, xcl-python and xcl-website release

Releases are automated by GitHub Actions, driven by pull request labels. A person pushes a branch, opens the PR, labels it and merges it; everything after the merge runs by itself.

**xcl**
- Every PR to `main` carries exactly one of `major`, `minor`, `patch`, `no-release` (`release-label.yml`, rules in `internal/release`, run through `cmd/xcl-release`). Before 1.0 a breaking change is `minor`; `major` releases `v1.0.0`.
- A merge to `main` pushes the next version tag (`tag.yml` calling the shared `auto-tag.yml`, using the release bot GitHub App so the tag starts workflows).
- The tag starts `release.yml`: it builds `xcl-core` for eight platforms with cgo disabled, signs the checksums with the jumppad-labs key, creates the GitHub release with generated notes, then dispatches xcl-python (`repository_dispatch` event `xcl-release` with the tag and the release size).

**xcl-python**
- The dispatch starts `engine-bump.yml`: it writes the tag to `xcl-ref.txt` on branch `engine/<tag>`, opens a PR labelled with the engine release's size, and turns on auto-merge. A bump whose checks fail stays open for a maintainer to fix on the same branch.
- PRs need the same release labels (shared check). A merge to `main` is tagged by xcl's shared `auto-tag.yml`, and the tag starts `release.yml`: wheels for seven platforms bundling the engine pinned in `xcl-ref.txt`, verified against the signed checksums, the parity suite against each wheel, publish to PyPI by Trusted Publishing, an install check on every platform, and a GitHub release whose notes list the merged PRs and name the bundled engine. Notes for breaking changes are added to that release by hand (xcl-python keeps no changelog file).

**xcl-website**
- No labels and no versions: every push to `main` deploys to GitHub Pages (`deploy.yml`). Merge docs for unreleased behaviour only after the release they describe is published.

**Order:** xcl release, then the xcl-python pin bump (which releases the package), then the website. See `gotchas/ci-regenerates-stubs.md` in the xcl-python store for when the automatic bump fails and the Python work has to land on the bump's branch.

**How to apply:** read the workflows again when planning a release rather than relying on this summary; the `release-plan` skill does so.
