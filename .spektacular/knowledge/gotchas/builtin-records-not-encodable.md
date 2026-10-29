---
tags: [events, encoding, coreservice]
---

# Variables and outputs get create success events but cannot be rendered

An apply emits `create`/`success` for every entity it creates, including builtins: `variable.<name>` and `output.<name>` (modules too). With processed event data each carries a saved record. But `EncodeSavedEntity` (Go `Config.EncodeSavedEntity`, the core service call, Python `encode_saved_entity`) refuses builtins with `not_encodable` / `xcl.ErrNotEncodable` / `xcl.NotEncodableError`, because a variable or output is not written as configuration. A handler that renders "every created resource", such as a pretty logger, must skip builtins by resource id or tolerate that error. Do not treat it as a failure.
