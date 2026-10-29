---
tags: [ci, releases, versioning]
---

# xcl releases were tagged by hand (superseded)

Until October 2026, xcl tags were pushed by hand and nothing built release artifacts. That is no longer true: releases are now automated from pull request labels across xcl, xcl-python and xcl-website. See `architecture/release-chain.md` in this store for how they work, and `conventions/release-plan-after-implementation.md` for writing a release plan once released work is merged.

What still holds: before 1.0, breaking changes bump the minor version, and the next number comes from the latest tag (`git tag --sort=-v:refname`), not from assumption.
