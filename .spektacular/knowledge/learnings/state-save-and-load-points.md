---
tags: [state, persistence, architecture]
---

# Where state is saved and loaded

As of 2026-10-04 there are exactly two places state is saved:
- `config.go`, after Apply;
- `internal/parser/destroy.go`, during Destroy.

Every load is typed through `savedentity.DecodeAll` (called from `internal/parser/parser.go`). The public `EncodeSavedEntity` goes through `savedentity.Decode`.

Any transformation of what is stored, such as masking or encryption, has to hook in at those save sites and at `savedentity.Decode`/`DecodeAll`. Missing one leaves state written or read untransformed.
