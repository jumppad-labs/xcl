---
tags: [json, encoding, state, plugins, serialization]
---

# A custom MarshalJSON changes what state and plugins see

Every internal xcl step serialises with plain `encoding/json`: saving state, provider calls in both directions, saved-entity decoding, query conversion and change detection. Event `Data` reuses the provider-call bytes.

The trap: giving a type a custom `MarshalJSON` (for example to hide a value when displayed) changes what plugins receive, what state stores and what change detection compares, not just what is printed. A redacting `MarshalJSON` silently loses the real value on every one of those steps.

Instead, route the internal steps through an encoder that bypasses the type's display form, and keep the custom form for output meant for people.
