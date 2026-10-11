---
tags: [struct-tags, gohcl, schema]
---

# `optional` is a kind in the xcl tag, not a modifier

In an `xcl:"…"` struct tag, `optional` is one of the field *kinds* (`attr`, `optional`, `block`, `label`, `remain`, …), not a flag that combines with them. `xcl:"name,optional"` is an optional attribute. `xcl:"name,block,optional"` is a tag error, not an optional block.

**How to apply:** to make a nested block optional, use a pointer field (`*T`) or a slice/map field with `xcl:"name,block"`. Don't add `optional` after `block`.
