---
created_date: "2026-09-23"
document_status: draft
project: xclconfig
spec: 20260922132517-hcl-encoding-helpers
plan: 20260922132517-hcl-encoding-helpers
---

# Turn an entity back into configuration text

You can now take a single entity, or the data xcl saved for it, and get it back
as configuration in xcl's own syntax — formatted, and ready to print or write
to a `.xcl` file. It is how an application shows someone what was actually
created, in the language they wrote it in.

> Derived from project xcl (file), spec/plan 20260922132517-hcl-encoding-helpers.
> See the project-level record for the full feature.

## What changed in this repo

**Two new functions.** `xcl.EncodeEntity(entity, options...)` converts an entity
you hold, such as one returned by `Find` after an apply.
`xcl.EncodeSavedEntity(registry, data, options...)` converts one entity's saved
data, in the same form the state file holds and an event carries. Both write
exactly one block, and both produce identical text for the same entity, so it
makes no difference which you start from. Convert several entities by calling
once for each.

The registry is passed in because it is the only thing that can type a saved
record, including types a plugin provides. It is loaded if it has not been
loaded already, so a registry no configuration has used yet still works.

**The text shows what a person wrote.** The block is written the way you write
it — `resource "<variety>" "<name>"`, or `<type> "<name>"` for a type declared
by its own keyword — with configuration names rather than Go field names, and
nested and repeated blocks as blocks. xcl's own bookkeeping is left out at every
depth, and `depends_on` is not written at all, because once a configuration is
parsed it holds the references xcl resolved as well as anything you wrote and
the two cannot be told apart.

**Values a provider filled in are off by default.** `xcl.IncludeComputed()`
writes them, and marks each one so a reader can tell it from something that was
configured:

```hcl
connection_string = "postgres://admin@localhost:5432/main" # set by the provider
```

**This text is for reading, not for feeding back in.** References come out as
the literal values they resolved to, the original comments and layout are not
kept, and output including provider-filled values does not validate, because
xcl refuses a configuration that sets them. Values are shown as they are held,
so a password or anything else secret is shown too.

**Three errors say why a conversion failed**, each matched with `errors.Is` and
carrying a detail recovered with `errors.As`: `xcl.ErrUnregisteredType`,
`xcl.ErrInvalidSavedData` and `xcl.ErrNotEncodable`, which is what a `variable`,
`output` or `module` returns. A failure returns no text.

**Events no longer carry resource data unless you ask.**
`xcl.WithEventData(level)` takes `xcl.EventDataNone` (the default, nothing on
any event), `xcl.EventDataRaw` (on every lifecycle event, the resource as it was
before the provider was called) or `xcl.EventDataProcessed` (on a success event,
the resource as state records it, including the values the provider filled in
and the status it ended with). Processed data is what state stores, so it goes
straight to `EncodeSavedEntity`.

**The examples show it.** Every example program now prints each entity's
configuration beneath the line announcing it was created, provider-filled values
included.

### Breaking

- **Lifecycle events carry no `Data` by default.** Code reading `Event.Data`
  must add `xcl.WithEventData(xcl.EventDataRaw)` to keep the pre-call resource,
  or `xcl.EventDataProcessed` for the resource as state records it.
- **`example/prettylog.Handler(w, level)` takes the plugin registry as a third
  argument**, `Handler(w, level, registry)`. Pass `nil` to leave configuration
  text out. This is an example package, so only the bundled examples are
  affected.

### Also worth knowing

The README previously documented `Config.ToJSON` and `Parser.UnmarshalJSON`,
neither of which exists in this codebase. Those sections are gone, and a test
now fails if either name reappears.

Two defects were fixed in the vendored HCL fork, both present upstream in
hashicorp/hcl. `SetAttributeRaw`, `SetAttributeValue` and
`SetAttributeTraversal` returned nil when they created an attribute, rather
than the attribute they created. And `node.ReplaceWith` left the owning list's
first and last pointers on the node it had just detached, so replacing the
first node of a list silently dropped every node after it — no error, the
content simply disappeared.

## Why

An application embedding xcl could inspect what a configuration declared, but
had no way to show it back in the language it was written in. It could reach
values through the query API and print them by hand, but not reproduce the
declaration. Now it can, including the values a provider filled in.

The event-data change is the security counterpart: a resource's configuration
and state used to travel through every event receiver by default, and now
nothing does unless you ask for it.
