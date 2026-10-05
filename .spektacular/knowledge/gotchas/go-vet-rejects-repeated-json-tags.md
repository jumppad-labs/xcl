---
tags: [testing, json, struct-tags, ci]
---

# go vet rejects test structs that repeat a json tag

CI runs `go vet ./...`, and its structtag check fails a struct where two fields carry the same `json` tag name.

Tests that exercise JSON field dominance (which of two clashing fields wins) cannot write the clash with explicit tags. Build the clash from untagged fields instead, for example two embedded structs each with a field of the same Go name.
