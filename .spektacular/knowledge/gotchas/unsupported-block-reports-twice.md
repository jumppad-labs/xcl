---
tags: [events, errors, parsing, diagnostics]
---

# An unsupported block's report arrives twice

Applying a configuration with an unsupported block produces a `parse error` event carrying that block's single diagnostic, and then the closing `<operation> error` event carrying the whole `ConfigError`, which includes the same problem. An event receiver that prints reports shows the same report twice unless it de-duplicates, for example by printing on the closing event only the problems not already shown.
