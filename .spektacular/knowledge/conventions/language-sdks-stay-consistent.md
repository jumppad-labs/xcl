---
tags: [sdk, consistency, releases, versioning, python, typescript, public-api]
---

# The language SDKs stay consistent with each other

The language SDKs (xcl-python, xcl-typescript and any that follow) behave the same wherever their languages allow. Where one SDK settles a question, the others follow it. A difference needs a reason the language forces, and that reason is written into the SDK's design and parity record.

This covers the public API and the way the SDKs ship:

- **Construction and naming.** A configuration is constructed as a type, `xcl.Config(...)` in Python and `new Config({...})` in TypeScript. Go's `xcl.New` is the Go idiom for the same thing. Where a language cannot do the work in a constructor, as with TypeScript's async engine start, the SDK creates lazily and offers an optional early step, matching Python's asyncio API.
- **Releases.** Every SDK follows xcl's shared release process: its own version line, one release label per pull request (`major`, `minor`, `patch`, `no-release`) checked by xcl's shared `release-label-check.yml`, a tag on merge from xcl's shared `auto-tag.yml`, and an engine bump pull request labelled with the engine release's size that merges itself once checks pass. A fix to one SDK releases it alone, with no xcl release. The bundled engine is the xcl tag in `xcl-ref.txt`, reported by `engine_version()` / `engineVersion()`.

When a design or plan for one SDK copies another SDK's behaviour, check the other SDK's current code, not its old design: the TypeScript design once copied Python's earlier rule that the package version equals the engine tag, after Python had already moved to its own version line.
