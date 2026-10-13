---
tags: [ci, releases, versioning]
---

# xcl releases were tagged by hand

As of 2026-10-11, xcl has no release workflow. Tags `v0.1.0` to `v0.2.1` were pushed by hand, and nothing builds release artifacts. The plugin repos pin xcl by tag (`v0.1.1` at the time).

The python-support plan adds the first release workflow (`libxcl-release.yml` on `v*` tags), and the epic's release sequence is `v0.3.0` (language-surface), then `v0.4.0` (python-support). Before 1.0, breaking changes bump the minor version.

**How to apply:** when planning work that needs a release, check the latest tag with `git tag` rather than assuming the next number. Treat cutting a tag as a task for a person, unless a release workflow does it.
