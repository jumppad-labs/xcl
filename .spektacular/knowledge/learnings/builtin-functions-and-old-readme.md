---
tags: [functions, docs, readme]
---

# Built-in function count and the old README's language sections

- As of v0.2.1 there are exactly **57** built-in functions, registered in `internal/functions/functions.go` (`GetDefaultFunctions`). Count them there rather than quoting the home page's "around sixty".
- The README's language and Go API sections were removed in commit `bdba483` ("Clean up docs"). Recover them with `git show bdba483^:README.md` (the language sections are around lines 1317–2060; the Go API querying and state sections around lines 600–842).
- That old text is raw material, not a source of truth. It contains stale claims:
  - `HCL_VAR_` overrides;
  - `local` blocks;
  - GitHub module sources (only local directories resolve);
  - custom functions through parser options;
  - "Lifecycle Callbacks" (`Processable`, `ParseCallback`, `Config.Process`), none of which exist.
