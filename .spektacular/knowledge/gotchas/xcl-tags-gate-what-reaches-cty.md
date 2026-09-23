---
tags: [hcl, gohcl, cty, encoding, struct-tags, meta]
---

# The `xcl` tag decides what reaches cty, not the Go type

`gocty.ImpliedType` builds a cty type from a struct's **`xcl`-tagged fields
only**. `impliedStructType` (`internal/cty/gocty/type_implied.go:76-123`) gets
its field set from `structTagIndices`, so a field carrying only a `json` tag is
invisible to encoding — however unrepresentable its Go type would be.

The consequence is counter-intuitive: a struct can hold a field cty cannot
represent and still encode cleanly.

`types.Meta` is the case that bites. It holds `Properties map[string]any`, plus
`Links`, `Parents` and `Status`, all with `json` tags and no `xcl` tag. Reading
the Go struct, you would expect encoding a `meta` attribute to fail on that
`map[string]any` — cty has no type for a map of untyped values, and the encoder
does return an error for one in an `xcl`-tagged field. It does not fail, because
those four fields never reach cty at all. `meta` implies a clean object of the
strings and ints that *are* tagged.

**When judging whether a type can be encoded as configuration, read its `xcl`
tags, not its Go shape.** Reasoning from the Go struct leads you to expect
failures that never happen, and to add defensive special-cases for them. During
the hcl-encoding-helpers work this nearly put a `meta`-skipping branch inside
`internal/xcl/gohcl`, which would have moved xcl-specific shaping into the
MPL-licensed fork — where, per `conventions/never-modify-dependencies.md`, it
does not belong. Trimming `meta` is the root wrapper's job.

Found on 2026-09-23 implementing Phase 1.1 of the hcl-encoding-helpers plan.
