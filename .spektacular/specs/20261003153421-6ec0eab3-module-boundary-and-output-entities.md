---
created_date: "2026-10-03"
document_status: final
closed_date: "2026-10-03"
epic: 20261003134528-327e0657-references-and-secrets
---

# Feature: 20261003153421-6ec0eab3-module-boundary-and-output-entities

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Today a configuration can reach directly into a module's internals, so a module's outputs are a convention rather than its interface, and looking up an output returns a bare value instead of an entity. This makes a module's outputs the only way to reach inside it, at every level of nesting, and treats outputs as entities in the Go API like everything else, so module authors control what they expose and application code works with outputs consistently.

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

- [ ] **Only outputs are reachable from outside a module**
  A reference from outside a module to anything inside it other than one of its outputs is rejected when the configuration is validated, with an error naming the reference.
- [ ] **The boundary holds at every level of nesting**
  A parent can reach only its direct child modules' outputs; anything deeper is reachable only if each module in between re-exports it as one of its own outputs.
- [ ] **Outputs are found as entities**
  Looking up, listing or decoding outputs returns the output entities themselves, with the published value available on the entity, rather than the bare value.
- [ ] **All published values remain available together**
  Developers can still get every published value in one call, keyed by address.
- [ ] **Bundled examples follow the module boundary**
  Every bundled example that uses modules or outputs reaches into modules only through their outputs and reads outputs as entities.
- [ ] **Module boundary and outputs are documented**
  The library's documentation and the documentation site describe that only a module's outputs can be referenced from its parent, how to re-export a nested module's value, and how to read outputs as entities.

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

- `output` must remain a builtin entity type; the module boundary is enforced by validating references.

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

- [ ] **Reference to a module internal is rejected**
  Given a configuration that references an entity inside a module other than one of the module's outputs, validation fails with an error naming that reference; the same configuration referencing the module's output instead validates.
- [ ] **Nested modules expose only through outputs**
  Given a root configuration using module A, which uses module B: a root reference to anything inside B, including B's outputs, fails validation; a root reference to an output of A that re-exports a value from B's output validates and resolves to that value.
- [ ] **Looking up an output returns the entity**
  Looking up an output by its address returns the output entity, whose value field holds the published value; listing outputs by type returns every declared output as entities.
- [ ] **All published values in one call**
  Requesting all published values returns one entry per declared output, keyed by address, each holding that output's value.
- [ ] **Examples run under the module boundary**
  Every bundled example that uses modules or outputs validates and runs successfully.
- [ ] **Module and output docs updated**
  The library's documentation and the site each describe the output-only module boundary with a re-export example, and show reading an output as an entity.

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

- Move the output entity type out of the internal package so applications can name it, and drop the lookup special case that returns an output's value instead of the entity.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

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

- Jumppad's own `output` entity: Jumppad, a downstream tool built on xcl, uses `output` to mean an environment-variable export and implements it as its own type.
