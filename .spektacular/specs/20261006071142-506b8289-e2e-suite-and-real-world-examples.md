---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
epic: 20261006071139-7b266535-examples-and-output
---

# Feature: 20261006071142-506b8289-e2e-suite-and-real-world-examples

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

xcl's examples currently double as its end-to-end test suite, so they are written to suit the tests rather than the reader. This spec gives xcl a dedicated end-to-end test suite that covers its own behaviour, carrying over everything the examples test today, so the examples can then be rewritten as real-world projects without losing coverage. It also removes the tests that inspect how examples are written, and makes the logging example self-contained. Maintainers get end-to-end coverage that no longer depends on how the examples are written.

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

- [ ] **xcl has its own end-to-end suite**
  xcl's behaviour is covered by an end-to-end test suite that exercises the library only as an application would, through its public interface.
- [ ] **No coverage lost**
  This spec removes the library-behaviour tests from the examples. Every library behaviour they check today is covered by the end-to-end suite or by the library's own tests before they are removed.
- [ ] **The suite runs the examples**
  The end-to-end suite runs each example's own tests, including any smoke test, and fails if any of them fail.
- [ ] **No tests of how examples are written**
  No test anywhere checks how an example's source is written. Tests only check what the examples do.
- [ ] **prettylog is self-contained**
  prettylog builds and tests on its own as if it were a separate project. It is a reusable handler rather than a runnable application, so it has no smoke test.

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

- **Each example is its own Go module.** It has its own module definition, so example-only dependencies never become dependencies of xcl itself.
- **Examples use only xcl's public packages.** They can't import xcl's internal packages, so a shared helper an example needs has to be one xcl publishes for application and plugin authors.
- **prettylog is not promoted to a library package.** It stays an example.
- **The end-to-end suite is xcl's whole-library black-box tests.** It tests the library as a whole from the outside and lives with its own fixtures, which is consistent with tests living with the code they test.
- **The project's testing conventions apply to every new and moved test.** testify `require`; Mockery for mocks; no table-driven tests; never positive and negative cases in the same test function; tests live with the code they test.

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

- [ ] **The e2e suite runs and passes**
  Running the end-to-end suite from a fresh checkout passes, and it uses xcl only through its public interface.
- [ ] **Coverage carried over**
  A coverage map kept with the end-to-end suite lists every library-behaviour test removed from the examples alongside the test that now covers it, and each listed test fails if that behaviour breaks.
- [ ] **A broken example fails the suite**
  If an example's tests are made to fail, the end-to-end suite fails and names that example.
- [ ] **No source-inspection tests**
  No test in the repository parses or inspects an example's source code.
- [ ] **prettylog builds alone**
  Copying prettylog's directory out of the repository, and pointing it at a published xcl version, leaves it buildable and its tests passing.

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

- **Put the e2e suite in a top-level `e2e/` directory** with its own fixtures.
- **Run example tests from the suite.** Since each example is its own module, the suite runs each example module's tests rather than repeating them.
- **Point example modules at the local xcl** with a `replace`, so they build against the working tree in the repo.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- A change to how an example is written, without changing what it does, never fails a test.
- A regression in xcl's behaviour is caught by the e2e suite or the library's own tests, never only by an example's tests.

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

- **Encoder syntax highlighting.** prettylog keeps its own highlighter for now. That's a separate spec to be added to this epic (issue #6).
- **Relocating the other misplaced root tests** (`static_output_test.go`, `static_dependencies_test.go`, `query_migration_test.go`). Those are a separate cleanup.
- **Rewriting the configuration and plugin examples.** Those are their own specs in this epic.
