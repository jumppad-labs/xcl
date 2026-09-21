---
tags: [meta, parsing, types, subtype]
---

# `Meta.Type` holds a different axis depending on the stanza

`Meta.Type` does not mean one thing. `blockResource`
(`internal/parser/parser.go:565-575`) sets it from the stanza keyword, then
**overwrites it with the first label** when the stanza is `resource`:

- `resource "postgres" "main"` → `Meta.Type == "postgres"`, the subtype
- `output "api_url"` → `Meta.Type == "output"`, the stanza keyword

So the word `resource` never appears in `Meta.Type`, and matching
`meta.Type == "resource"` finds **nothing, silently**. It reads back as "you
have none of those" rather than "that is not what this field holds".

The second half of the trap is the conversion. `schema.UnmarshalUntyped`
(`internal/schema/unmarshal.go:7`) round-trips through JSON and does **not**
fail on mismatched structs: unknown fields are dropped, absent fields zeroed,
and `ResourceBase` populates from the shared embed. Converting an ingress into
a `Container` therefore yields a half-empty value and a nil error.

Match on the subtype you actually mean, and verify the type before converting
rather than trusting the round-trip to fail.

Splitting these into separate `Type` and `Subtype` fields is the fix, and it
is designed in `design/querier-api-v2.md`. Adding the field will also trip
`gotchas/meta-field-golden-schema.md`.
