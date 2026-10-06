---
tags: [examples, go-modules, imports]
---

# Example modules cannot import xcl's internal packages

Each example under `example/` is its own Go module, pointed at the local xcl with
`replace github.com/jumppad-labs/xcl => ../..`. Go only lets code inside xcl's own
module import `internal/`, so an example can't use `internal/testutil`,
`internal/test_fixtures` or any other internal package, even though it sits in the
same repository. Examples use only public packages and keep their own small helpers.

The root `go test ./...` and `go list ./...` also stop at module boundaries, so they no
longer reach the examples. The e2e suite runs each example's tests explicitly.
