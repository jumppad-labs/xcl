---
tags: [parsing, hcl, references, meta]
---

# Expression source text is lost during parsing

Parsing keeps only the flattened addresses an expression refers to, in `Meta.Links`, with no record of which field they came from. The text the user wrote, such as `"${resource.b.one.y}-x"`, is gone.

The trap: anything that needs the expression as written can't recover it later from `Meta` or from state.

Instead, capture it during parsing. The file bytes (`f.Bytes`) are only available in `parseResourcesInFile`, so they have to be passed down from there.
