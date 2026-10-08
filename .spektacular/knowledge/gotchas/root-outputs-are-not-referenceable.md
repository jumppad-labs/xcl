---
tags: [references, modules, addressing]
---

# A root-level output can not be referenced as output.<name>

Outputs are not in the evaluation context of other blocks. An expression such as
`value = output.network_name` in the root module does not resolve, the way
`resource.<type>.<name>` or `variable.<name>` would.

An output is reachable only from outside its module, as
`module.<name>.output.<output>` (see `internal/parser/context.go`). To share a value
between blocks of the same module, reference the resource or variable the output
reads from, not the output itself.
