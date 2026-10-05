---
tags: [tooling, imports, hcl]
---

# goimports aliases the internal/xcl import in internal/parser

Running `goimports` on files in `internal/parser` rewrites the `internal/xcl` import as `hcl "…/internal/xcl"`.

The alias is unwanted churn. Format these files with `gofmt` instead, or revert the import line after running `goimports`.
