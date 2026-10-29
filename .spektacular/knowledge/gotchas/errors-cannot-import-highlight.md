---
tags: [errors, highlight, import-cycle, package-layout]
---

# errors cannot import highlight

`highlight` imports `errors` (for `InvalidThemeError`), so `errors` can never import `highlight`; the cycle is permanent while shared error types live in `errors` (see `conventions/shared-errors-package.md`). Anything in `errors` that needs report layout or lexing must go through a token-free internal package (`internal/report`), which the root package then turns into tokens. Type aliases or an `init()` hook to break the cycle were considered and rejected.
