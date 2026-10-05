---
created_date: "2026-10-05"
document_status: draft
project: xclconfig
spec: 20261003153421-bf87d907-references-as-written
plan: 20261003153421-bf87d907-references-as-written
---

# References as written in configuration text

Configuration text can now show where values came from. Passing `xcl.ShowReferences()` to `EncodeEntity` or `EncodeSavedEntity` writes each field that referred to another entity exactly as the user wrote it, such as `x = resource.b.one.y` or a template, instead of its resolved value. Without the option nothing changes. A sensitive field still shows `(sensitive)` unless it was written as a single bare reference, or real values are also asked for.

> Derived from project xcl (xclconfig), spec/plan 20261003153421-bf87d907-references-as-written. See the project-level record for the full feature.

## What changed in this repo

- `types.Meta` gained `References`, saved in state as the optional `meta.references` key. State saved earlier still loads and shows resolved values until it is applied again.
- The parser records the text written after `=` for every reference-holding attribute, keyed by path such as `network[1].name`.
- `encode.go` adds the `ShowReferences()` option and the replacement step that runs after bookkeeping is trimmed. Live and saved text are byte-identical, and the option combines with `IncludeComputed()` and `RevealSensitive()`.
- The README, CHANGELOG and `docs/state.md` document the option, and new parser, encoder, state and content tests cover it. The plugin schema snapshot fixture gained the new `Meta` field.
- There are no breaking changes.

## Why

Before this change, configuration text turned every reference into a literal, so a reader could not see which entity a field pointed at. The bookkeeping and the encoder both live in this library, so this repo carries the whole behaviour.
