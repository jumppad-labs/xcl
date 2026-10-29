---
tags: [logging, errors, diagnostics, events]
---

# slog's text handler and zap print the full report of an xcl error

`*errors.Diagnostic` and `*errors.ConfigError` implement `fmt.Formatter`, with
`%+v` giving the full plain report (source lines included). Loggers pick that
up without any adapter, but not uniformly:

- `log/slog`'s **text** handler formats an error attribute with `%+v`, so
  `slog.Any("error", err)` writes the whole report as one quoted value with its
  line breaks escaped. slog's JSON handler, and `events.SlogHandler` (which
  logs the message), write the one line `Error()` gives.
- zap's `zap.Error(err)` puts the report in `errorVerbose`, but only for the
  unwrapped xcl error: an error wrapped with `fmt.Errorf("...: %w", err)` is
  formatted by `*fmt.wrapError`, so `errorVerbose` holds just the one line.
- `%v` on a `ConfigError` is one line per problem, not one line in total.

For a person, `xcl.PrintError` is the dependable route: it finds every
diagnostic however the error was wrapped. In an event handler, print reports
for `*errors.Diagnostic` events only; the event that ends the call carries the
whole `*ConfigError` and printing it too repeats every report.
