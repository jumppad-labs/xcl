# Interview: python-plain-dataclasses

## What and why

Bring the Python binding (`xcl-python`, package `xcl-lang`, imported as `xcl`) in line with the Go
library after PR #13 (plain-structs): a block type is a plain data type with no framework base
class and no framework decorator. In Go a plain struct registers with
`registry.Register[Database](local, "database", "postgres")`; the type and subtype are given at
registration, `Meta types.Meta` is an optional field filled with the block's metadata, and
`disabled` / `depends_on` fields optionally mirror the values every block has.

Today Python requires two pieces of xcl markup on every entity class:
- subclassing `xcl.Resource` (in practice it is not structurally needed: the schema skips
  `meta`/`depends_on`/`disabled` and hydration only fills `meta` if declared — verified by running
  a plain dataclass end to end against a locally built engine);
- decorating with `@xcl.entity(type, subtype)`, which `register_type` requires
  (`registration_of` rejects undecorated classes) and which also feeds a process-wide class
  registry used by untyped lookups (`entities()`, `find(None, …)`, `event.entity()`).

Users: Python application authors defining config-only block types (and declaring types on
GitHub registries) for XCL configuration.

## Decisions

- **Remove `xcl.Resource` and `@xcl.entity` entirely** (user's choice over keeping as sugar or
  deprecating). Breaking change. Plain dataclasses + names at registration are the only way.
- Registration takes the names: `local.register_type(Database, "database", "postgres")`;
  `GitHubRegistry.register_type` likewise. Mirrors Go's `Register[T](r, name...)`.
- Optional fields on a plain dataclass, mirroring Go plain structs:
  `meta: xcl.Meta | None` filled with the block's metadata, never decoded;
  `disabled` / `depends_on` mirror the values every block has. All optional.
- `.address` (only on `Resource`) goes away with it; address is `obj.meta.id` when `meta` is
  declared. `xcl.Entity` (untyped plugin entities) keeps `.meta` and `.address`.
- Untyped lookups resolve classes from the Config's own registries, not a process-wide registry.
  This also removes the "last decorator wins" problem across configs in one process.
  `event.entity()` must work without a Config in scope (event carries the config's class map).
- Registering something that is not a dataclass, or looking up with an unregistered class / `None`
  where a class is required, raises a proper `xcl` error (today `registration_of(None)` raises an
  unrelated `AttributeError`).
- Field shapes stay annotation-driven and markup-free by default (user: "it is nice that it is
  optional"): `xcl.field` only for `name=` (config name differs, e.g. attribute `images`, config
  `image`), `block=False`, `label=True`. `xcl.Sensitive[T]` / `xcl.Computed[T]` stay.

## Cross-repo scope

- **xcl-python**: API change, README, tests (unit + integration), changelog/breaking note.
- **xcl-website**: Python tabs across the docs, the language reference and "Using XCL with Python"
  switch to plain dataclasses and names at registration (user: yes, include).
- **xcl** parity harness (`bindings/libxcl/testdata/parity`): Python parity side uses plain
  dataclasses, Go side plain structs, and the schemas must match (user: yes, extend).

## Non-goals (as discussed)

- No change to the engine, the core service protocol, or Go's public API.
- No change to how field shapes are inferred from annotations, beyond dropping base-class fields.
- Plugins remain Go-only.
