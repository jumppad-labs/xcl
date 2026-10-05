---
tags: [modules, fqrn, addressing, references]
---

# Nested module keys: build them with FQRN.AppendParentModule

Working-set keys for entities inside nested modules use one dot-joined module path, `module.a.b.output.x`, not a repeated `module.` prefix (`module.a.module.b...`).

The trap: building a scoped key by joining strings looks right for one level and silently gets nested references wrong. `resolveReference` in `internal/parser` did exactly this, so a module that re-exports its child's output fails validation.

Build scoped keys with `FQRN.AppendParentModule`, which the evaluation context (`AppendParentModule` in context.go) already uses.

The same shape applies to the module entities themselves: a module `b` declared inside module `a` is keyed `module.a.b`, and a `module.b` reference written inside `a` resolves through `AppendParentModule` to that same key.
