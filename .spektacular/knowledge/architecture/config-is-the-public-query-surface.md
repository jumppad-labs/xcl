---
tags: [state, config, public-api, query]
---

# `Config` is the only public query surface

The public query surface of this library is `Config`, and only `Config`.

The `state` package is public, but its purpose is limited to **storing and
retrieving raw resources**: `StateStore` should traffic in raw `[]any` —
`Load() ([]any, error)`, `Save([]any) error`, plus `Exists` and `Clear` — and
the `State` struct is not a public consumable. How the internal system
*searches* resources is not the state package's concern, and the struct that
provides data access needs no dependency on `FQRN` or on address parsing at
all. Guarding against invalid resources belongs at the public level, on
`Config`.

**The code does not yet match this.** `state.State` still exports
`FindResource`, `FindRelativeResource`, `FindResourcesByType` and
`FindModuleResources`, and imports `internal/resources` to parse FQRNs.

Those methods are a fossil. Before Oct 2025 `Config` itself held the whole
container-and-query surface — `FindResource`, `findResource`,
`FindRelativeResource`, `FindResourcesByType`, `FindModuleResources`,
`AppendResource`, `RemoveResource`, `addResource`. Commit `ef30ca3` ("Add state
container and filestore") created `state/state.go` as a new 231-line file
**without touching `config.go`**, copying the query methods across along with
the storage. The parser interface those methods satisfy is still literally
named `ConfigProvider` (`internal/parser/dag.go`) — named for `Config`,
implemented by `State`. `Querier` and `Config` delegation later took the public
query role back, leaving `State` holding methods it never needed to own.

They are demonstrably vestigial:

- nothing outside this repo calls them
- the bundled examples never touch them — their `FindResource` calls are
  `Querier`'s method of the same name, and `c.GetResources()` is `Config`'s
- `FindResourcesByType` has zero callers
- `FindRelativeResource` has exactly one

A `StateStore` implementer never needs them either: building a state in
`Load()` needs only construction and append, and `Save()` needs only the
resources or their bytes.

The cleanup is cheapest after Milestone 2 of the query-api-v2 plan, which
builds the replacement lookup surface (`find`, `findByType`, `findOne`, `all`)
in package `xcl` scanning `Config.Entities()`.
