---
tags: [packaging, pypi, python, versioning, plugins]
---

# Each language SDK is one package

**Decision:** xcl publishes exactly one package for each language: `xcl` on
PyPI, and one npm package for TypeScript. That package holds everything for the
language: the application API, the bundled `xcl-core` engine, and the plugin
SDK (`xcl.plugin` in Python). New features go into it. They don't get a package
of their own.

**Why:** the project lead's concern is the cost of maintaining several
packages in each of several languages: separate versions, releases and
compatibility to keep straight, multiplied by every language.

**Considered and rejected:** a separate pure-Python `xcl-plugin` package for
the plugin SDK, which `xcl` would depend on. It would have kept the 20 MB
engine binary out of plugin images and frozen executables, and let baked-in
plugins pin a package that changes only when the protocol does. Both were
judged smaller than the cost of a second package in every language.

**Consequences:**

- A plugin carries the engine binary it never runs. That is accepted as a size
  cost.
- A plugin declares a loose minimum version of the language package (the first
  release whose protocol it speaks), never an exact pin. A baked-in plugin
  shares its application's environment, and a loose minimum is what keeps that
  from clashing.
- Platform engine packages do not count as separate packages. `xcl-lang` on npm
  ships `xcl-core` in eight `xcl-lang-<os>-<arch>[-<libc>]` packages listed in
  its `optionalDependencies`, the npm form of Python's per-platform wheels. A
  user installs only `xcl-lang`, and the platform packages share its version
  and release. The project lead approved this on 2026-10-31.
- No package exists only to carry entity classes, neither one per plugin nor
  one an application installs to import a plugin's classes. An application
  that wants typed access to a plugin's types generates classes from its schema
  v1 into its own code (`python -m xcl generate`), or reads `xcl.Entity`.

Decided 2026-10-31 while designing Python plugins (`python-plugins.md`).
