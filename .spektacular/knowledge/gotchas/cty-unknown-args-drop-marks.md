---
tags: [cty, marks, validation]
---

# cty drops marks when a function gets unknown arguments

When any argument to a cty function call is unknown, the call returns an unmarked unknown before marks are collected.

The trap: checking marks (such as a "sensitive" mark) by evaluating expressions with unknown placeholder values silently loses those marks wherever a function or operator is involved, so the check passes when it should fail.

Instead, predict marks statically from the expression and the Go types. Don't evaluate with zero values either: that runs user functions.
