---
created_date: "2026-10-26"
document_status: draft
project: xclconfig
spec: 20261023075302-bad71237-printing-and-highlighting-python
plan: 20261023075302-bad71237-printing-and-highlighting-python
---

# Parity program records printing and logging

The Go parity program that language bindings are compared against now also records what a person is shown: the report printed for a failed validate and apply, the configuration text printed for every entity, the change summaries printed before the apply and for a changed configuration, and the event log written through the slog bridge. A new fixture holds a configuration with an unsupported block on line 7. A binding whose printing or logging differs from Go's now fails its parity suite.

> Derived from project xcl, spec/plan 20261023075302-bad71237-printing-and-highlighting-python. See the project-level record for the full feature.

## What changed

- `bindings/libxcl/cmd/parity/main.go`: new record keys `printed_validate`, `printed_apply` (`xcl.PrintError`, plain, width 80, or `""`), `printed_diff_before` and `printed_diff_changed` (`xcl.PrintDiff`, or the outcome), `printed_entities` (`xcl.PrintEntity` with `IncludeComputed()`, by address) and `log` (every event up to the load through `events.SlogHandler` into a recording slog handler, as level, message, resource and operation, sorted), with run paths replaced.
- New fixture `bindings/libxcl/testdata/parity/unsupported_block` and README sections describing the new keys.
- Tests (`cmd/parity/main_test.go`): the new keys, the exact line-7 report, printed entities and change summaries, log entries and order, path normalisation and determinism.

## Why

The Python package now prints reports, resources and change summaries and bridges events to logging; the parity harness proves its output equals Go's. These stages must be in the xcl release xcl-python pins, because its CI builds the parity program from that checkout.
