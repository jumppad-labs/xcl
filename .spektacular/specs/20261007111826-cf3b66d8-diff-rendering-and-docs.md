---
created_date: "2026-10-07"
document_status: final
closed_date: "2026-10-07"
designs:
    - source: design
      path: config-diff.md
epic: 20261007105731-2388b579-diff
---

# Feature: 20261007111826-cf3b66d8-diff-rendering-and-docs

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

People reviewing a change want to see what an apply would do at a glance, not read raw data. This turns a diff result into a readable summary in the familiar style of a git diff, marking each resource and value as added, changed, replaced or removed, optionally in colour, with sensitive values kept hidden. The xcl documentation explains how to run a diff and how to read this output, so users can review changes with confidence before applying them.

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

- [x] **Readable rendering**
  Users can render a diff result as text in a git-diff-like style, where each resource and each changed value is marked by whether it is added, changed, replaced or removed, values known only after apply are shown as such, and a summary line ends the output giving the number of resources to create, update, replace and delete, and the number unchanged.
- [x] **Optional colour**
  Users can choose to render the diff with colour; without that choice the rendering is plain text.
- [x] **Sensitive values stay hidden in the rendering**
  The rendering shows that a sensitive value changed without showing its values, and shows the values only when the diff was run with sensitive values revealed.
- [x] **Documentation**
  The xcl documentation site explains how to run a diff, what each kind of change means, and how to read the rendered output.

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

- Must be built to the design settled in the `design` source at `config-diff.md`, which fixes the entry point, the result types and paths, the JSON format, the action names (create, update, replace, delete) and the rendered output format.
- The public API, types and documentation must use the name "diff" and must not use "plan" terminology.

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

- [x] **Rendering marks changes**
  Rendered output marks created resources and values as added, deleted ones as removed, replaced ones as replaced, and updated ones as changed with the old and new values shown together, and ends with the summary line.
- [x] **Unknown values are shown**
  Rendering a result that holds a value known only after apply shows that value as known after apply rather than as a concrete value.
- [x] **Rendered summary matches the result**
  For a result reporting 2 to create, 1 to update, 1 to replace, 1 to delete and 3 unchanged, the rendered summary line shows all five numbers; for a result with no changes, the summary line is the only output.
- [x] **Plain by default, colour on request**
  Rendered output contains no colour codes unless colour is requested; when it is requested, the output contains colour codes.
- [x] **Sensitive values are masked in the rendering**
  When a sensitive value changes, the rendered output contains neither the old nor the new value and shows that a sensitive value changed; when the diff was run with sensitive values revealed, both values appear.
- [x] **Documentation published**
  The xcl documentation site has a page that explains running a diff, the meaning of create, update, replace, delete and known-after-apply, and shows an example of the rendered output.

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

- Reuse the existing highlight renderer and themes for coloured output rather than introducing a new colouring mechanism.
- Beyond the referenced design, the detailed design is left for the plan workflow to propose.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- The rendered example on the documentation page matches, line for line, what the renderer produces for the same diff result.
- No sensitive value appears in any rendering produced without an explicit request to reveal it.

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

- Adding diff output to the `example/` programs; this is left for later.
- A command-line interface; xcl remains a library.
- Showing the saved values of a resource that would be deleted.
- Changes to the VS Code extension.
