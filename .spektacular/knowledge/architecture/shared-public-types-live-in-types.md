---
tags: [types, public-api, import-cycle, package-layout]
---

# Public types shared with the parser live in `types`

The root `xcl` package imports `internal/parser`. So a public type that both `internal/parser` and applications need, such as an entity type that applications look up, must live in the `types` package, never in the root package. Putting it in the root package creates an import cycle.
