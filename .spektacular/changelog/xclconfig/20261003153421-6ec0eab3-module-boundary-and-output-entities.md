---
created_date: "2026-10-05"
document_status: draft
project: xclconfig
spec: 20261003153421-6ec0eab3-module-boundary-and-output-entities
plan: 20261003153421-6ec0eab3-module-boundary-and-output-entities
---

# Module boundary and output entities

Configurations can now reach into a module only through its outputs. A reference to anything else inside a module, or into a module nested inside it, fails validation with an error naming the reference. A module can re-export a nested module's value as its own output, so deep values stay reachable on purpose. In Go, outputs are ordinary entities: `xcl.Find[types.Output]` returns one with its value on `.Value`, outputs can be listed and decoded like any other type, and `Outputs()` still returns every value at once.

> Derived from project xclconfig, spec/plan 20261003153421-6ec0eab3-module-boundary-and-output-entities. See the project-level record for the full feature.

## What changed in this repo

- `types/output.go`: new public `types.Output`, moved from `internal/resources`, which keeps only the `TypeOutput` constant. All internal users were retargeted.
- `query.go`, `config.go`, `decode.go`, `errors/query_errors.go`: the output special cases in `Find` and `typeable` were removed, and the doc comments updated.
- `internal/parser/references.go`: module-scoped keys are composed with `AppendParentModule`, and a new `crossesModuleBoundary` predicate was added.
- `internal/parser/validate.go`: stage 2 reports boundary crossings, including in `depends_on`, before the undefined check.
- `example/plugin/main.go`: reads outputs with `Find[types.Output]` and `.Value`.
- `README.md`, `docs/modules.md`, `CHANGELOG.md` and `readme_test.go`: documentation of the boundary, re-export and output entities, with content tests.
- New fixtures `output_entities`, `module_reexport` and `module_boundary/*`, plus parser, query, decode and Apply tests.
- Knowledge entry `architecture/ux-flow.md` was updated for output entities.

**Breaking:** `Find[string]` on an output address now fails with `ErrTypeMismatch`, and configurations that reach into a module other than through its outputs now fail validation.

## Why

The library owns the boundary rule and the query API, so both changes land here. Module internals become private to the module, and outputs become first-class entities for applications.
