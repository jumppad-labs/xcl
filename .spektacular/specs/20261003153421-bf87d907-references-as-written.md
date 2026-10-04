---
created_date: "2026-10-03"
document_status: final
closed_date: "2026-10-03"
epic: 20261003134528-327e0657-references-and-secrets
---

# Feature: 20261003153421-bf87d907-references-as-written

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Configuration text produced by xcl shows every reference as the literal value it resolved to, so a reader cannot see which other entity a field pointed at. This lets developers who embed xcl ask for references to be shown exactly as the user wrote them, from a live entity or from its saved data with identical results, while resolved values remain the default.

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

- [ ] **References can be shown as written**
  Developers can ask for an entity's configuration text to show each reference as the address the user wrote, instead of the value it resolved to.
- [ ] **Resolved values remain the default**
  Without that request, configuration text continues to show references as the values they resolved to.
- [ ] **References survive saving**
  Text produced from an entity's saved data shows the same references, and is identical to the text produced from the live entity, whichever form is requested.
- [ ] **References output is documented**
  The library's documentation and the documentation site's configuration-text guide describe how to request references and what the text then shows.

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

- [ ] **References written as addresses on request**
  Given an entity with a field written as a reference to another entity (e.g. `x = resource.b.one.y`), when its configuration text is requested with references shown, the text contains that field as the reference exactly as written, not the resolved value.
- [ ] **Resolved values by default**
  Given the same entity, when its configuration text is requested without asking for references, the field shows the resolved value, as before this change.
- [ ] **Live and saved text match**
  For an entity containing references, after applying, the text produced from the live entity and the text produced from its saved data are byte-identical, both with references shown and without.
- [ ] **References docs updated**
  The library's documentation and the site's configuration-text guide each describe requesting references, with an example showing a reference written as the user wrote it.

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

- Record which field each reference came from at parse time and keep it in saved state, so both the live and the saved encoding paths can write references; the stored format may change to carry it.

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

- Backwards compatibility with existing state files: state written by earlier versions does not have to load.
- Making configuration text reprocessable: it remains for display and reading, as settled in spec `20260922132517-hcl-encoding-helpers`.
