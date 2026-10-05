---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Module boundary and output entities

## What was built

**Module boundary (xclconfig).** A module's outputs are now the only way to reach inside it from configuration, at every level of nesting. Validation stage 2 checks every reference an entity holds, including user-written `depends_on` entries, against the reference as written. The reference's module path must be empty, or name a single direct child module with an `output` as the target. A reference to a module's resources, variables or nested modules, or to anything in a grandchild module, is reported once as `resource '<id>' refers to '<ref>', which is inside module '<module>'; only a module's outputs can be referenced from outside it`. It is not also reported as undefined, and both `Validate` and `Apply` refuse it before anything is created. `depends_on = ["module.<name>"]` stays valid.

**Re-export (xclconfig).** The reference resolver now composes module-scoped keys with `FQRN.AppendParentModule`, the same way the DAG and the evaluation context do. A module can therefore re-export its child module's output (`output "from_b" { value = module.b.output.value }`), and a deeply nested value is reachable when each module in between re-exports it. An end-to-end three-level test applies such a configuration and reads the grandchild's value from the root.

**Outputs as entities (xclconfig).** The output type moved from an internal package to the public `types.Output`, with identical fields and serialised form. The query layer dropped its two output special cases. `Find[types.Output]` returns the entity with the published value on `.Value`. `FindByType[types.Output](c, "output")`, `All[types.Output]` and `Decode` into `[]*types.Output` return every declared output at any depth, and `Outputs()` still returns every value keyed by address. The plugin example reads its outputs as entities and prints the same lines as before.

**Documentation.** In xclconfig, the README (published-values section and a new "The module boundary" subsection), `docs/modules.md`, the CHANGELOG with both breaking changes, and content tests guarding them. In xcl-website, the home page Modules and Variables-and-outputs cards and the plugin example page. The `architecture/ux-flow.md` knowledge entry now shows `Find[types.Output]` and `.Value`, with a note on `Outputs()`.

## Why it matters

Module authors now control exactly what a module exposes, so a module's internals can change without breaking the configurations that use it. Application code handles outputs the same way as every other entity, which lets it list and decode them, not just read single values.

## Breaking changes

- `Find[string]`, or any type other than `types.Output`, on an output address now fails with `ErrTypeMismatch`. Use `Find[types.Output](...).Value` or `Outputs()`.
- Configurations that reference a module's resources, variables or nested modules, or a grandchild's outputs, now fail validation. Re-export the value through outputs instead.

## Deviations from the plan

- The resolver fix also makes a `module.b` reference written inside module `a` resolve, to `module.a.b`. A test was added for it.
- Extra documentation tests were added: a separate negative README test, kept apart per convention, and a modules-guide content test.
- The open question about whether a nested re-export evaluates at walk time was answered yes, with no change to the evaluation context.
- Otherwise none.
