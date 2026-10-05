---
tags: [dependencies, depends_on, dag, meta, links, destroy]
---

# Meta.Links orders; DependsOn is as written

As of 2026-10-05, `ResourceBase.DependsOn` holds exactly the strings the user wrote in `depends_on`, in the order written, and nothing else. The parser sets it at parse time (`getUniqueResourceLinks` in `internal/parser/parser.go`), so disabled entities, which are never decoded, keep it too; the walk's decode writes the same strings again for enabled entities.

`Meta.Links` holds every dependency: each reference found in attributes and nested blocks (as full attribute paths, e.g. `resource.network.c.subnet`) plus each `depends_on` entry in its canonical address form, module-relative. It is what the dependency graph reads for both create and destroy, and what validation (including the module boundary check) and the evaluation context read.

Create and destroy use one builder, `buildDependencyGraph` in `internal/parser/dag.go`. Destroy builds it from the links saved in state, resolved against the whole working state, keeping only edges between entities being destroyed, and walks it with `Reverse`. There is no `Meta.Parents`; saved state carries no `meta.parents`.

`types.AppendUniqueLink` appends to `Meta.Links` only. It replaced `AppendUniqueDependency`, which also copied into `DependsOn`. Code that needs every dependency reads `Meta.Links`, never `DependsOn`.
