---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
designs:
    - source: design
      path: plugin-layout.md
epic: 20261009092551-82db0140-plugin-template
---

# Feature: 20261009102138-48e95432-docker-example-standard-layout

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

The Docker plugin example predates the standard plugin layout: its entity types and providers share a package, and the example application pulls in the Docker providers and libraries just to read entity types from state. This spec rebuilds the example to the standard layout and file order, so the example teaches the same shape as the template and applications that read its state stay light.

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

- [x] **Entity types stand alone**
  The plugin example's application reads the example's resources from state using only the plugin's entity types, without depending on the plugin's providers or on the Docker libraries.
- [x] **Consistent file order**
  In the plugin example, every file follows the file order the standard layout defines.
- [x] **The plugin example follows the standard**
  The Docker plugin example is rebuilt so each part a plugin holds is where the standard plugin layout puts it.
- [x] **Documentation reflects the example's layout**
  The project's guides, README and the documentation site's plugin example page show the example's new file locations and code.

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

- **Public members first in every file.** In every source file in the Docker plugin example, exported types and methods are kept together at the top, ahead of unexported helpers.
- **The plugin example keeps its behaviour.** The rebuild changes only layout and ordering.
- **Tests follow the repository rules.** Any test the rebuild moves or changes uses testify `require` and Mockery for test doubles, with no table-driven tests and positive and negative cases in separate test functions.
- **Entity types live in a package named `entities`.**
- **The plugin contract doesn't change.** The template and the example's rebuild use the contract as it is today.
- **Built to the standard plugin layout.** The design `plugin-layout.md` (source `design`) settles the plugin layout, what each part holds, file order, the explicit change decision, updates from what they are told, and the test shape. The template, the plugin example and the docs build to it.

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

- [x] **Reading state needs only the entity types**
  The plugin example's application reads the example's resources from state without its build including the Docker providers or the Docker libraries they use.
- [x] **Files list public members first**
  In the plugin example, every source file shows its exported types and methods before any unexported helper, and provider methods appear in the lifecycle order the standard layout defines.
- [x] **The plugin example matches the standard layout**
  Every location the standard layout names for a plugin is where the plugin example keeps that part.
- [x] **The plugin example behaves as before**
  After the rebuild, the plugin example's scenario tests (network swap, subnet rebuild, init-script rebuild and content edit, network removal, dangling reference) pass unchanged, as do its unit tests.
- [x] **The documentation shows the new layout**
  The guides, README and the documentation site's plugin example page quote only file locations that exist in the rebuilt example, and the site builds.

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

- The example application and its status and inspect commands import only the plugin's entity types; the Docker client and providers stay inside the plugin.
- Risk: the template and the plugin example drift apart; the layout design is the shared reference.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- **One shape everywhere.** The plugin example and the documentation never disagree about the layout: checked against the layout design, neither has a part in a place the design doesn't name.
- **Example applications stay light.** The plugin example's application builds without the Docker libraries being pulled in only to read entity types.

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

- **Restructuring the other examples.** Only the Docker plugin example is rebuilt. The person example plugin and the end-to-end test fixtures keep their current shape.
- **Porting jumppad's resources.** Porting jumppad's resources to xcl plugins is later work.
