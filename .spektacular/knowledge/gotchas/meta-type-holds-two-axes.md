---
tags: [meta, parsing, types, subtype, fqrn, addressing]
---

# `Meta.Type` and `FQRN.Type` each held two axes

**Status: split landed 2026-09-21** (query-api-v2 plan, Phases 1.1–1.3).
`types.Meta` now carries `Type` (the stanza kind) and `Subtype` (the variety,
empty for single-label stanzas), and `internal/resources.FQRN` carries the same
pair. This entry stays because the *shape* of the trap recurs.

Both structs held one `Type` field meaning the variety for a `resource` stanza
and the stanza keyword for everything else:

- `resource "postgres" "main"` → the variety, `"postgres"`
- `output "api_url"` → the stanza keyword, `"output"`

So the word `resource` never appeared in either field, and matching
`Type == "resource"` found **nothing, silently** — it read back as "you have
none of those" rather than "that is not what this field holds".

The part that makes this expensive: `Meta` and `FQRN` are separate structs in
separate packages with **no compile-time link**, and both fields are plain
`string`. A reader left on the wrong axis keeps building while behaviour
changes underneath.

This is not hypothetical. The query-api-v2 plan was written accounting for
`Meta` but not `FQRN`, and got three instructions wrong as a result: it stated
that four `state` sites "each need the variety added to the comparison, not
substituted for the kind", which held for only one of them. Left as written,
`state.findResource` would have compared `"resource"` against `"postgres"` and
made **every** resource lookup return not-found, silently.

When changing either struct's type axes, walk the other's boundaries
deliberately rather than relying on the build. The boundaries are wherever a
`Meta` is turned into an `FQRN` or compared against one: `FQRNFromResource`,
`state` `addResource`/`findResource`, and the parser's cycle detection.

The conversion half of the original trap is unchanged. `schema.UnmarshalUntyped`
(`internal/schema/unmarshal.go`) round-trips through JSON and does **not** fail
on mismatched structs: unknown fields are dropped, absent fields zeroed, and
`ResourceBase` populates from the shared embed. Converting an ingress into a
`Container` therefore yields a half-empty value and a nil error. Verify the type
before converting rather than trusting the round-trip to fail.

Adding a field to `Meta` also trips `gotchas/meta-field-golden-schema.md`.
