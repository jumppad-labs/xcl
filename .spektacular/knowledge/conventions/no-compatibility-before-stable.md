---
tags: [compatibility, releases, api, coreservice, plugins]
---

# Breaking changes are allowed until xcl is stable

xcl has no stable release, so nothing is kept only for compatibility. Public Go APIs, the language SDKs, the CoreService protocol, the plugin handshake and the plugin release layout may change wherever that gives a simpler result. No compatibility shims, deprecation periods or old-and-new code paths side by side.

**Why:** the user, 2026-10-31: "I don't care about breaking changes, we are not released."

**How to apply:**

- When planning or designing, don't use "keeps existing callers working" as a reason to choose an option. Choose on simplicity. Say so if an option is kept for another reason.
- Breaking changes still follow the release mechanics: a breaking change before 1.0 is labelled `minor` (see [[release-chain]]), and plugins and SDKs that depend on the change are rebuilt or regenerated.
- CI's `buf breaking` check still runs on `core.proto`, so a removed field is retired by reserving its number and its name ([[buf-field-no-delete]]). That's a CI rule, not a compatibility promise.
- Rules about formats read by software already in the field, such as registries ignoring unknown package kinds in a release index, are design decisions for after the first stable release, not compatibility shims, and they stay.
