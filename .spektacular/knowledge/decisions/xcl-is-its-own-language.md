---
tags: [vision, language, positioning, hcl, docs, declarative, templates]
---

# XCL is its own language, built on HCL's readability

**Decision:** XCL is a configuration language in its own right. It takes the
readable syntax of HCL (blocks, attributes and expressions) and builds on it
for modern processing: references between blocks, graph-based processing in
dependency order, plugins, and state so configuration can be planned,
applied, changed and destroyed.

- **No HCL compatibility goal.** XCL does not aim to be backward compatible
  with Terraform configuration or any other HCL dialect. Where XCL can be
  clearer or do more, it diverges from HCL.
- **Many implementations.** The Go library, `github.com/jumppad-labs/xcl`, is
  the first implementation. Implementations for Python, JavaScript and other
  languages are intended.

**Why:** HCL is a pleasant syntax to read and write, but the HCL library
stops at parsing and decoding. Everything that makes configuration useful
(resolving references, ordering by dependency, plugins, state) lives in
Terraform rather than in HCL. XCL exists to provide that as a language of its
own, so it is not bound by HCL's or Terraform's choices.

**How to apply:**

- Docs, the README and the website present the language first and the Go
  library second.
- Never call XCL a "superset of HCL" or say "any HCL file is valid XCL". Say
  it builds on HCL's readability or starts from HCL's syntax.
- Avoid HCL and Terraform jargon such as "interpolation". Use plain words:
  "references", "resolved", "filled in".
- Write XCL for the language and `xcl` only for the Go module and package.
- A block in XCL is an **entity**, never a "resource", which is Terraform's
  word. Every entity type is registered with a type and an optional subtype,
  which become the block's labels and its address. Examples never use
  `resource` as a block type; the canonical pair is `database "postgres"
  "main"` (address `database.postgres.main`) and `app "web"` (address
  `app.web`). Go identifiers such as `types.ResourceBase` keep their names
  until the API itself is renamed.
- Design work may change syntax or semantics away from HCL when that serves
  XCL; compatibility with existing HCL files is not a reason to reject it.

## What the language includes (decided 2026-10-11)

"Just because we inherit these things, we don't have to expose them." The HCL-derived parser is a stopgap, and XCL diverges from it where that makes sense.

- **Declarative.** No `for_each`, dynamic blocks or any other block generation. Moving towards imperative constructs takes XCL away from what a configuration language should be. This may be revisited if the community asks for it.
- **No templating in the core.** String template directives (`%{ if }`, `%{ for }`) are rejected. `${ }` references in strings stay. Templating belongs in functions or plugins, so people can use Go templates, Mustache or anything else.
- **Kept:** operators, conditionals, indexing, splat, `for` expressions (they reshape a value, they don't generate structure) and functions.
- **No `local` block.** Variables are resolved through the graph, so a variable's default can be built from other values.
- **Named blocks over map attributes.** Named child blocks (`listener "http" { … }`) collect into a map keyed by name. Map and object attributes stay supported, because they are the only route for structured data through variables and modules, but they are not the encouraged style.
- **Modules are encapsulated.** Configuration reaches into a module only through its outputs.
- **Functions** come from three places: built-ins, functions an application registers (like types), and functions plugins host. All are evaluated while configuration is read.
- **Environment overrides** use the `XCL_VAR_` prefix.
