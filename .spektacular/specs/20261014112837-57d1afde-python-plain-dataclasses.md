---
created_date: "2026-10-14"
document_status: final
closed_date: "2026-10-14"
---

# Feature: 20261014112837-57d1afde-python-plain-dataclasses

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Python programs that read XCL configuration currently have to build every configuration type on top of an XCL-specific base class and mark it with an XCL-specific label. Go programs no longer need either. This change lets a Python program use ordinary data classes for its configuration types, naming each type when it hands it to XCL, exactly as a Go program does. The result is that Python types carry no XCL markup unless a field needs it, and both languages describe configuration the same way. The old base class and label are removed, so existing Python programs need a small update. The Python documentation, the website examples and the Go/Python comparison tests are updated to match.

<!--
  REQUIREMENTS
  Specific, testable behaviours the feature must deliver.
  Format: bold title on the checkbox line, detail indented below.
  Rules:
    - Use active voice: "Users can...", "The system must..."
    - Each requirement should be independently verifiable
    - Focus on WHAT, not HOW — avoid prescribing implementation
    - Keep each item atomic — one behaviour per line
-->
## Requirements

- [ ] **Plain data classes as configuration types**
  Users can register an ordinary data class as a block type, with no XCL base class and no XCL label on the class.
- [ ] **Type named at registration**
  Users give the block's type, and optionally its subtype, when they register the class on a local registry or a GitHub registry. The class itself carries no type name.
- [ ] **Old base class and label removed**
  The XCL base class and the class label are no longer part of the Python package. Code that uses them fails with an error that tells the user how to register a plain data class instead.
- [ ] **Block metadata on request**
  A class that declares a metadata field receives the block's metadata (its address, name, type, file and line) on every lookup. A class that doesn't declare one works the same in every other respect.
- [ ] **Disabled and dependencies on request**
  A class can declare fields for a block's disabled flag and its explicit dependencies, and they hold the values set in configuration. Configuration can set both on any block whether or not the class declares them.
- [ ] **Untyped lookups use the configuration's own types**
  A lookup that doesn't name a class returns an instance of the class registered on that configuration for the entity's type. An entity whose type has no registered class is returned as the generic entity.
- [ ] **Configurations are independent**
  Two configurations in one process can register different classes for the same type name, and each configuration's lookups return its own class.
- [ ] **Events hydrate with the configuration's types**
  An entity carried on an event comes back as the class registered on the configuration that emitted it.
- [ ] **Clear errors for bad registrations and lookups**
  Registering something that isn't a data class, registering without a type name, or looking up with a class that isn't registered raises an XCL error naming the problem, not an unrelated Python error.
- [ ] **Field markup stays optional**
  A field's configuration name, and whether it's an attribute, an optional attribute, a nested block, a repeated block or a map of named blocks, all still come from the field's name, type and default. Users need field-level markup only to rename a field, to treat a data class field as an object value instead of a block, or to mark a block's name label.
- [ ] **Same schema as Go**
  For each shared comparison fixture, the Python plain data classes derive exactly the schema documents the Go side registers.
- [ ] **Documentation shows plain data classes**
  The Python package's documentation and every Python example on the website use plain data classes with type names given at registration, and none use the removed base class or label outside the migration guide.
- [ ] **Breaking change recorded**
  The release notes record the removal as a breaking change and point to a migration guide showing how to update an existing class.

<!--
  CONSTRAINTS
  Hard boundaries the solution must operate within. These are non-negotiable.
  Format: one bullet point per constraint.
  Examples:
    - Must integrate with the existing authentication system
    - Cannot introduce breaking changes to the public API
    - Must support the current minimum supported runtime versions
  Leave blank if there are no constraints.
-->
## Constraints

- The Python model of a block type must follow the Go plain-struct model: no base type, type names given at registration, and metadata, disabled and dependencies available only when the type declares them.
- The removal is a clean break: the old base class and class label are not kept as deprecated aliases or optional sugar.
- The change must not require any change to the xcl engine or its protocol.
- The change must not change Go's public API.
- The Python package must ship on its own version line through the automated release process.
- Being a breaking change before 1.0, it must ship as a minor release.

<!--
  ACCEPTANCE CRITERIA
  The specific, binary conditions that define "done".
  Format: bold title on the checkbox line, verifiable detail indented below.
  Each criterion must be:
    - Independently verifiable (pass/fail, not subjective)
    - Traceable back to a requirement above
    - Testable by someone who didn't write the code
-->
## Acceptance Criteria

- [ ] **Plain data class applies end to end**
  A program registers a data class that has no XCL base class and no class label, then applies, validates, looks up, diffs and destroys a configuration that uses it. Each operation succeeds, and the lookup returns an instance of that class with the configured values.
- [ ] **Type and subtype come from registration**
  A data class registered as type `database` with subtype `postgres` matches a `database "postgres" "main" {}` block and is found at `database.postgres.main`. Registered as type `app` with no subtype, it matches `app "web" {}` and is found at `app.web`. The same holds when the class is registered on a GitHub registry.
- [ ] **Removed API fails with guidance**
  Importing the old base class or the old class label from the package raises an error. The error message names the replacement: a plain data class with the type name given at registration.
- [ ] **Metadata only when declared**
  When a class declares a metadata field, a looked-up instance has it set to the block's address, name, type, file and line. When a class doesn't declare one, the same configuration applies and looks up without error, and the instance has no metadata attribute.
- [ ] **Disabled and dependencies mirror configuration**
  For a block with `disabled = true` and `depends_on = ["x.y"]`, a class that declares those fields reads them back as true and `["x.y"]`. A class that doesn't declare them accepts the same block without a validation error.
- [ ] **Untyped lookup returns the registered class**
  Listing all entities, or finding one without naming a class, returns instances of the class registered on that configuration for each type. A plugin type with no registered class comes back as the generic entity.
- [ ] **Two configurations, two classes**
  Two configurations in one process register different classes for type `app`. An untyped lookup of `app.web` on each returns an instance of that configuration's own class.
- [ ] **Event entity uses the configuration's class**
  An event handler on a configuration that carries entity data on its events gets back, from the event's entity, an instance of the class registered on that configuration.
- [ ] **Errors are XCL errors**
  Registering a class that isn't a data class, registering with no type name, and looking up with a class that isn't registered each raise an exception under the package's base error class. The message names the class or the missing type name.
- [ ] **Unmarked fields infer their shape**
  A data class with no field markup, holding a required string, a defaulted integer, an optional string, a nested data class, a list of data classes and a dict of data classes, validates configuration as a required attribute, an optional attribute with a default, an optional attribute, a nested block, a repeated block and a map of named blocks respectively. A field marked with a configuration name of `image` on the attribute `images` reads `image = "…"` from configuration.
- [ ] **Python and Go schemas match**
  For every shared comparison fixture, the schema the Python plain data classes produce is identical to the one the Go side registers.
- [ ] **No removed API in the docs**
  Searching the Python package's README and the website's Python examples finds no use of the removed base class or class label outside the migration guide, and each Python example registers its types with a type name.
- [ ] **Release notes cover the migration**
  The release notes name the removed base class and label as a breaking change and link to a migration guide with a before-and-after example of migrating one class.

<!--
  TECHNICAL APPROACH
  High-level technical direction to guide the planning agent. Include:
    - Key architectural decisions already made
    - Preferred patterns or technologies if known
    - Integration points with existing systems
    - Known risks or areas of uncertainty
  Format: one bullet point per direction/steer.
  Leave blank if you want the planner to propose the approach.
-->
## Technical Approach

- Registration takes the block's type and optional subtype alongside the class, in the same shape as Go's `Register[T](registry, type, subtype)`.
- Each configuration builds its own type-to-class map from its registries; untyped lookups use it, and the process-wide class registry goes away.
- An event carries what it needs to hydrate its entity with the emitting configuration's classes.
- The metadata, disabled and dependency fields are recognised by name on a plain data class, following the Go plain-struct rule, and are excluded from the declared schema.
- Field shapes stay inferred from annotations; the field-options helper remains only for renaming, treating a data class as an object value, and marking a block's name label.
- The removed names stay importable just long enough to raise a clear migration error.
- Release notes are generated from PR titles: the breaking PR's title names the removal of `xcl.Resource` and `@xcl.entity`, and its description links to a migration section added to the README.
- Risk: the breaking change touches every existing test, the README and the website's Python tabs; the plan should treat that migration as real work.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- In the release that ships this change, no example in the README, the tests or the website subclasses an XCL base class or uses the class decorator; `xcl.Resource` and `@xcl.entity` appear only in the migration guide and the migration error.
- Migrating an existing class takes two edits: drop the base class and decorator, and pass the type and subtype to `register_type`; the README's migration section shows the before and after.

<!--
  NON-GOALS
  Explicitly state what this spec does NOT cover. This is as important as
  the requirements — it prevents scope creep and sets clear expectations.
  Format: one bullet point per exclusion.
  Examples:
    - "Mobile support is out of scope (tracked in #456)"
    - "Internationalisation will be addressed in a follow-up spec"
  Leave blank if there are no explicit exclusions to call out.
-->
## Non-Goals

- Plugins stay Go-only; this does not make it possible to write a plugin in Python.
- No Go-side check that a Go plain struct produces the same schema as the shared comparison fixtures; if wanted, that is a separate test in xcl.
