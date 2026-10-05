---
tags: [testing, fixtures, sensitive]
---

# registered.Secret is the fixture for sensitive-record tests

`internal/test_fixtures/registered/types.go` defines `Secret` (block `resource "secret"`), with `Username string` and `Password types.Sensitive[string]`.

Use it for tests of sensitive values in state, events, masking and configuration text rather than adding a new fixture type.
