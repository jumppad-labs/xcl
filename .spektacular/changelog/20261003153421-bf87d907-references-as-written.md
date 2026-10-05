---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# References as written in configuration text

## What was built

Configuration text from `xcl.EncodeEntity` and `xcl.EncodeSavedEntity` can now show each reference exactly as the user wrote it. Pass `xcl.ShowReferences()` and the text shows `x = resource.b.one.y`, a template such as `"${variable.region}-a"`, or a reference inside a nested block, where it used to show the resolved value. Without the option the text is unchanged.

- **Recording (xclconfig)**: while the parser finds links between entities, it records the text after `=` for every attribute holding a reference. It uses the same rule that decides links. The text goes into a new `types.Meta.References` map, keyed by attribute path (`location`, `network[1].name`) and stored as JSON only, so it never reaches cty or configuration. Because it lives in `Meta`, it is saved in state as `meta.references`, carried to plugins and events, and decoded from saved records. None of those paths needed new code. State written earlier still loads.
- **Encoding (xclconfig)**: under `ShowReferences()`, after the resolved text is written and bookkeeping trimmed, the encoder walks the recorded paths in sorted order and swaps each attribute's tokens for the re-lexed recorded text. An attribute the text does not hold is never added. Live and saved text stay byte-identical with and without the option. A sensitive attribute is only replaced by a single bare reference, unless `RevealSensitive()` is also given.
- **Documentation**: the README documents the option with an example and qualifies the "for reading" note. The CHANGELOG has an entry for this change ("There are no breaking changes."), `docs/state.md` mentions `meta.references`, and content tests guard all of these. The documentation site (`xcl-website`) gains a configuration-text guide in the Guides menu, covering conversion, saved data, provider-filled values, sensitive values and showing references.

## Why it matters

A reader of configuration text produced by xcl could not tell which entity a field pointed at, because every reference had become a literal. Developers embedding xcl can now show their users where values came from, from a live entity or from saved data, while resolved values stay the default and secrets stay masked.

## Deviations from the plan

- Adding a field to `Meta` changed the plugin schema snapshot in `internal/schema/test_fixtures/embedded.go`. That existing test expectation was updated.
- The plan left open whether the sensitive marker always shows in an attribute's tokens. This was settled by checking for the quoted marker anywhere in the attribute's tokens, which covers markers nested in objects and lists. The fallback to the Go field was not needed.
- More tests were added than the plan listed: nested-block defaults, formatter stability of a template, the saved-entity sensitive template, and extra README, state-guide and changelog content checks.
