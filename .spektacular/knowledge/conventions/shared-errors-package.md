---
tags: [errors, sentinel, package-layout, import-cycle]
---

# Shared error types live in the `errors` package

Put a new shared error type or sentinel in `github.com/jumppad-labs/xcl/errors`, not in whichever
package happens to raise it. That package already holds `ConfigError` and `ParserError`.

**Check it exists before concluding it does not.** This rule was missed once by assuming a shared
error package would have to be created.

## Why it has to be that package

`errors` imports only `internal/xcl` and `go-wordwrap` — not `state`, not `internal/parser`, not the
root `xcl` package. That thin import list is the whole point: any package can depend on it without a
cycle, so it is the only available neutral ground between packages that cannot import each other.

The case that forces it: the root package imports `xcl/state` (`config.go:10`), so `state` cannot
import back. A sentinel both raise must live somewhere both can reach.

**Keep that import list thin.** Adding a dependency on `state`, `internal/parser` or the root package
destroys the property the package exists for.

## Re-export a public sentinel from the root package

Where consumers match a sentinel, re-export it from `xcl`, following `config.go:16`
(`var ErrEmptyConfiguration = parser.ErrEmptyConfiguration`). A re-export is the same value, so
identity is preserved. It also spares consumers alias-importing a package named `errors` just to keep
using `errors.Is` from the standard library.

## Follow the sentinel-and-detail convention

Declare a package-level `var Err… = errors.New(…)` with a doc comment naming who returns it and that
it is matched with `errors.Is`, then wrap it in a struct carrying the detail for `errors.As`.
`plugins/errors.go` is the archetype and the only place currently implementing it end to end.

Two existing mistakes not to copy: `types/register.go:38` formats its typed error with `%s` instead of
wrapping it, so the type is unrecoverable and is matched nowhere; and receivers are inconsistent
across packages (value in `state/`, pointer in `plugins/registry/`). New error types use pointer
receivers and always wrap.
