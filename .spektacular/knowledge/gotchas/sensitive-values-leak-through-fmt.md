---
tags: [sensitive, marks, cty, reflection, public-api]
---

# A public type holding a sensitive cty.Value leaks through fmt

`fmt` prints a struct by reflection, field by field, and reflection reaches the real value inside a sensitive-marked `cty.Value`. Marks do not stop it.

Any public type that holds a `cty.Value` which may be sensitive must implement `fmt.Formatter` (a `Format` method) that prints the redacted form. `types.Output` (`types/output.go`) is the only such type today; the leak suite caught it before it had one.

When adding a public type that carries a `cty.Value`, give it a `Format` method and add a leak-suite case for it.
