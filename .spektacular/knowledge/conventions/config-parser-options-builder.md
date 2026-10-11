---
tags: [config, parsing, variables]
---

# Config operations build parser options through one builder

Every `Config` operation (Validate, Apply, Diff, Destroy, Load) must build its `parser.ParserOptions` through a single shared builder on `Config`, never by filling in a `ParserOptions` literal by hand.

**Why:** `Config` used to build `ParserOptions` by hand in four places. Defaults set in `parser.DefaultOptions()` never reached them. That's how `VariableEnvPrefix` came to be empty in every operation, so every process environment variable leaked into configuration as `variable.<NAME>` and the documented prefix did nothing. The language-surface plan introduces the shared builder and the `XCL_VAR_` prefix.

**How to apply:** when adding a parser option, set it in the shared builder and check it reaches every operation. Never add another hand-built `ParserOptions` in `config.go`.
