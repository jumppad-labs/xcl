---
tags: [state, sensitive, encoding, persistence]
---

# Code that saves state itself must use parser.EncodeForState

Typed entities holding `types.Sensitive[T]` marshal to the marker `(sensitive)` under plain `json.Marshal`. That is right for display, wrong for state.

The trap: code (including tests) that hands typed entities straight to a store's `json.Marshal` saves `(sensitive)` instead of the real value. Nothing fails at save time; the next apply sees the saved value differ from the configuration and reports a spurious change.

Encode records for state with `parser.EncodeForState(entities, stateMask)` (`internal/parser/state_encode.go`), which keeps real values and applies the state masker when one is set.
