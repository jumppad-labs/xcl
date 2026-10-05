---
tags: [state, json, testing]
---

# The file state store writes indented JSON

The file state store saves state with `MarshalIndent`, so a key and value are separated by `": "` with a space, for example `"xcl_masked": "aes-256-gcm"`.

A string assertion against a state file on disk must include that space. A recording or in-memory store sees the compact form (`"xcl_masked":"aes-256-gcm"`), so the same assertion cannot be shared between the two; prefer decoding the JSON and asserting on fields.
