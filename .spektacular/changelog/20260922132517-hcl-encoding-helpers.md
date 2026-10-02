---
created_date: "2026-09-23"
document_status: final
closed_date: "2026-09-23"
---

# Converting entities back to configuration text

## What was built

An application embedding xcl can now take a single entity, or the data xcl
saved for it, and get it back as configuration text in xcl's own syntax,
formatted and ready to print or write to a `.xcl` file.

Two functions do it. `xcl.EncodeEntity(entity, options...)` converts an entity
the caller holds, such as one returned by `Find` after an apply.
`xcl.EncodeSavedEntity(registry, data, options...)` converts one entity's saved
data, in the same form the state file holds and an event carries. Both write
exactly one block and produce identical text for the same entity, so it makes
no difference which you start from. The registry is passed explicitly because
it is the only thing that can type a saved record, including the types a plugin
provides, and it is loaded on demand so a registry no configuration has used
still works.

The block is written the way a person writes it: `resource "<variety>" "<name>"`
for a resource-kind entity and `<type> "<name>"` for one declared by its own
keyword, using configuration names rather than Go field names, with nested and
repeated blocks as blocks. The text shows what a person wrote — xcl's own
bookkeeping is left out at every depth, including inside an attribute whose
value is a whole object, and `depends_on` is not written at all, because by the
time a configuration is parsed it holds the references xcl resolved alongside
anything the author wrote and the two cannot be told apart. Values a provider
filled in are left out too; `xcl.IncludeComputed()` writes them and marks each
one with a comment saying the provider set it.

Three errors say why a conversion failed, each matched with `errors.Is` and
carrying a detail recovered with `errors.As`: `ErrUnregisteredType`,
`ErrInvalidSavedData` and `ErrNotEncodable`, the last being what a `variable`,
`output` or `module` returns, since those are never written as configuration. A
failure returns no text.

Underneath, the tag-driven encoder in xcl's vendored HCL fork was repaired so it
writes everything the decoder can read: it walks into embedded base structs,
writes values held in untyped fields (which is how plugin types rebuilt from a
schema hold their named scalars), skips values that hold nothing rather than
writing `null`, honours the `computed` tag option the parser already understood
but discarded, and returns errors naming the field instead of panicking. The
per-record decode that turns saved data back into a typed value was lifted out
of the file state store into one shared reader, so the stored format has a
single reader and the state store's behaviour is unchanged.

Separately, lifecycle events no longer carry resource data unless asked. A new
option, `xcl.WithEventData(level)`, takes none (the default), raw (on every
lifecycle event, the resource before the provider call) or processed (on a
success event, the resource as state records it, provider values and final
status included). Processed data is byte for byte what state stores, so it can
be handed straight to `EncodeSavedEntity`. The example programs turn it on and
print each entity's configuration beneath the line announcing it was created.

The library README, `docs/state.md` and the xcl.dev site document all of it.

## Why it matters

An application embedding xcl could inspect what a configuration declared, but
had no way to show it back to a person in the language they wrote it in. It
could reach individual values through the query API and print them by hand, but
not reproduce the declaration. This closes that gap: a tool can now show a user
exactly what was created, including the values a provider filled in, in the
syntax the user already knows.

The event-data change is the security counterpart. A resource's configuration
and state used to travel through every event receiver by default. Now nothing
does unless an application asks for it, and asking is what makes the examples
able to render each resource as it appears.

## Deviations from the plan

Four, all agreed with the user during implementation.

**Read-back was descoped.** The spec required the output be "readable by xcl
again", and an acceptance criterion required it validate as a configuration.
Once the encoder met a real container the user decided the text is for display
rather than reprocessing, and read-back is no longer a guarantee. Validation
was deliberately not relaxed to accept provider-filled values, because they
would be overwritten anyway. The requirement and the matching Round-trip
success metric are recorded as descoped in the plan.

**A phase was added.** Phase 1.3 grew beyond its scope once it became clear the
rules about bookkeeping and provider values had to reach inside an attribute
whose value is a whole object, and that marking provider values needed an API
the HCL fork did not have. That work became a new Phase 1.4.

**Two items moved to a separate spec, still to be written.** Writing a
reference such as `networkobj = resource.network.onprem` in place of the value
it resolved to, which the parser cannot support today because it discards which
field each reference fed. And separating hand-written `depends_on` from
reference-derived links, which needs the same provenance tracking; the user
identified xcl overwriting the public `DependsOn` with privately-computed links
as a design mistake worth fixing on its own.

**Provider-filled values inside an object attribute carry no comment.** Such an
object is rendered from a value rather than from tokens, so there is nowhere to
attach one. The acceptance criterion was narrowed to say so rather than left
unmet. Comments work everywhere else, including inside nested blocks.

## Two upstream HCL bugs fixed along the way

Both are defects in hashicorp/hcl, not introduced here, and both are recorded
in the fork's `UPSTREAM.md` with regression tests.

`SetAttributeRaw`, `SetAttributeValue` and `SetAttributeTraversal` each declared
a new variable inside their else branch, shadowing the one they return, so
creating an attribute returned nil despite each being documented as returning
the attribute created.

`node.ReplaceWith` rewired a replaced node's neighbours but never updated the
owning list's own first and last pointers. Replacing the first node of a list
left `first` on a node that had just been detached, so walking the list yielded
that node alone and **silently dropped everything after it** — no error, the
content simply vanished. It surfaced as setting a comment on an attribute
deleting the whole attribute.
