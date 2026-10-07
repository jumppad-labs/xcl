---
tags: [testing, fixtures, e2e]
---

# Producing a failed resource in the e2e plugin suite

To get a resource saved as `failed` in an e2e test without changing the shared
fixtures, copy the configuration to a temp dir and set a postgres block's
`location` to `"bad host"`. The in-process provider's `connect` cannot reach it, so
the create fails and the resource is saved as failed. The next apply, and any
diff, then treats it as `replace`.

`e2e/diff_test.go` holds the value as `diffFailingLocation`.
