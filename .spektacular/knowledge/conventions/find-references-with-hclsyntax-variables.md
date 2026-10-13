---
tags: [references, dag, parsing, expressions]
---

# Find references with hclsyntax.Variables, not a hand-written walker

Dependency extraction must find the references in an expression with `hclsyntax.Variables` (as the sensitive-value check already does), never with a hand-written switch over expression types.

**Why:** the hand-written `processExpr` in `internal/parser/exp.go` didn't handle `ForExpr`, `IndexExpr`, `ParenthesesExpr`, `RelativeTraversalExpr` or `TemplateJoinExpr`. A reference written inside any of them was never recorded as a dependency, so the referenced entity wasn't resolved first. For example, `{ for l in app.web.listener : l.name => l.port }` failed with `There is no variable named "app"`. A walker silently misses every expression kind it doesn't list, including new ones; `hclsyntax.Variables` covers them all.

**How to apply:** any code that needs "what does this expression refer to" calls `hclsyntax.Variables` (or a helper built on it) and converts the traversals, rather than switching on expression types.
