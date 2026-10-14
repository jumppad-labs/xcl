---
tags: [ci, releases, versioning]
---

# xcl releases were tagged by hand (superseded)

Until 2026-10-14 xcl tags were pushed by hand (`v0.1.0` to `v0.7.0`), and xcl-python's version was
locked to the xcl tag it bundled.

Since 2026-10-14 releases are automatic: every PR carries a release label, merging tags the next
version, and each language binding has its own version line with an engine pin. The first
automatic tags were xcl `v0.8.0` and xcl-python `v0.8.1`.

**How to apply:** don't plan a hand-cut tag. Choose the PR's release label (before 1.0, a breaking
change is `minor`), and follow `architecture/cross-repo-release-flow.md` for the order work across
xcl and the bindings lands in.
