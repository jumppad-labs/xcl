---
tags: [dependencies, depends_on, dag, meta]
---

# DependsOn mirrors Meta.Links

As of 2026-10-04, `ResourceBase.DependsOn` is not just the list the user wrote. `types.AppendUniqueDependency` appends to `Meta.Links` and copies every link into `DependsOn`, and the create graph (`getResourceDependencies` in `internal/parser/util.go`) reads `DependsOn` back.

During the walk, decode overwrites `DependsOn` only when a `depends_on` attribute is present. It never runs for disabled entities.

The user-depends-on spec in the references-and-secrets epic changes this: ordering moves to `Meta.Links`, and `DependsOn` keeps only what the user wrote. Re-check this entry once that lands.
