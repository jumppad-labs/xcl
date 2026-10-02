---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Feature: 20260922132517-hcl-encoding-helpers

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Developers embedding xcl will be able to turn any single resource back into configuration text, in the same syntax people write by hand, and print it or save it to a file. They can start from a resource they already hold, or from the saved data xcl keeps between runs, so a resource that was created, including every value filled in along the way, can be shown to a person or written back out as configuration. Today the only way to see a resource after the fact is as raw machine-oriented data. This makes it readable, and lets the example program show each resource's full configuration as it is created.

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

- [x] **Resource to configuration text**
  Developers can pass a single resource they hold, such as one obtained after applying a configuration, and receive it as configuration text in xcl's own syntax.
- [x] **Saved data to configuration text**
  Developers can pass a single resource's saved data, in the form xcl stores it between runs (which is also the form lifecycle events carry), and receive the same configuration text they would get from the resource itself.
- [x] **Written as a person would write it**
  The output uses the block type, labels and field names a person would use in a configuration file. Nested structures appear as nested blocks, and repeated blocks appear once per entry.
- [ ] **Nothing left out**
  The output includes every value the resource holds, including values filled in by providers after creation. Nothing is filtered or hidden.
- [ ] **Readable by xcl again**
  The output is valid xcl configuration that xcl can read back, including when it contains values filled in by providers.
- [x] **Printable or writable**
  The output can be printed as it is or written to a file without further processing, and is formatted consistently.
- [x] **Every known resource type**
  Every resource type the configuration knows can be converted: types provided by plugins, in-process or external, and types registered directly, in either declaration form.
- [x] **Clear failure for data that can't be converted**
  When saved data names a type the configuration doesn't know, or can't be read, the developer gets an error that says why. No partial output is produced.
- [x] **One call per resource**
  Each call converts exactly one resource. Converting several means calling once for each.
- [x] **Example shows created resources**
  The example programs' terminal output shows each resource's full configuration as configuration text when it has been created.
- [x] **Documented**
  The library documentation and the documentation site describe how to convert a resource, and a resource's saved data, into configuration text, with an example of each.

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

- The saved-data entry point must accept resource data exactly as xcl already stores it between runs and carries it on lifecycle events. The stored format is an existing contract and must not change to support this feature.
- xcl's internal HCL-writing machinery must not become public API; only the purpose-built conversion interface is exposed.

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

- [x] **Resource to configuration text**
  After applying a configuration containing `resource "postgres" "main"` with `port = 5432` and a nested `timeouts` block, converting that resource after the apply returns text beginning `resource "postgres" "main" {`, containing `port = 5432` and a `timeouts` block.
- [x] **Saved data to configuration text**
  Converting the saved data for the same resource, as read from the state file, returns text identical to converting the resource itself.
- [x] **Written as a person would write it**
  For a resource whose Go field names differ from its configuration names, the output uses the configuration names. A field holding two repeated blocks appears as two blocks with the same name, and a bare-form type's block starts with `<type> "<name>" {`, without the `resource` keyword.
- [ ] **Nothing left out**
  After an apply in which a provider fills in `connection_string`, converting the resource, or its saved data, returns text containing that `connection_string` value.
- [ ] **Readable by xcl again**
  Writing the output to a file and validating a configuration made of that file succeeds, and parsing it gives a resource of the same type and name with the same configured values.
- [x] **Printable or writable**
  Converting the same resource twice returns byte-identical text, and running the output through xcl's formatter leaves it unchanged.
- [x] **Every known resource type**
  One resource of each kind converts successfully: a type from an in-process plugin, a type from an external plugin, a registered type in resource-keyword form, and a registered type in bare form.
- [x] **Clear failure for data that can't be converted**
  Converting saved data that names an unregistered type returns an error naming that type and no text. Converting data that isn't valid saved-resource data returns an error and no text.
- [x] **One call per resource**
  Converting three resources one at a time gives three separate texts, each with exactly one top-level block.
- [x] **Example shows created resources**
  Running any example at its default output level shows, for each resource the example creates, the resource's configuration as configuration text alongside its create success line.
- [x] **Documented**
  The library README and the documentation site each contain a section on converting a resource to configuration text, with one example starting from a resource and one starting from saved data.

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

- Design the public conversion interface around xcl's own concepts, a resource and its saved data, rather than around HCL-writing primitives.
- Prefer driving the output from the same field tags xcl uses to read configuration, since that's what makes the output use configuration names and nested blocks. The vendored tag-driven encoder that already exists internally is a natural base.
- Converting saved data means first turning it back into a typed resource, and only the plugin registry knows the types, including those plugins provide from a schema. So the saved-data entry point should resolve types through the registry rather than guessing from the data.
- The example receiver needs the same type knowledge to show a created resource's configuration, so expect the example set-up to give it access to the registry.
- Known risk: resources built from a plugin's schema may not carry the same field tags as hand-written Go types, which could affect how faithfully they encode. The plan should check this early.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- Round trip: 100% of resources in the example programs convert to configuration text that xcl reads back without errors.
- Agreement: for every resource in the example programs, converting the resource and converting its saved data give identical text (0 mismatches).
- Visible in the examples: every resource created by any example program appears as configuration text in that example's default output.
- Easy to use: a developer can convert a resource, or a resource's saved data, following the docs alone.

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

- **Sensitive data:** masking or omitting secrets such as passwords is out of scope. Output shows every value as it is. Tracked in https://github.com/jumppad-labs/xcl/issues/1.
- **Builtin blocks:** converting `variable`, `output` and `module` blocks is out of scope. Only resource types, plugin-provided and registered, are covered.
- **Other input formats:** reading configuration from formats other than xcl's own syntax is out of scope.
